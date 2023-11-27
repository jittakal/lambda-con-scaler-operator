package controller

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const visibilityMetricName = "ApproximateNumberOfMessagesVisible"

type AwsSvcClient struct {
	LambdaClient *lambda.Client
	SQSClient    *sqs.Client
	CWClient     *cloudwatch.Client
}

func NewAwsSvcClient() (*AwsSvcClient, error) {
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}

	lambdaClient := lambda.NewFromConfig(cfg)
	sqsClient := sqs.NewFromConfig(cfg)
	cwClient := cloudwatch.NewFromConfig(cfg)

	return &AwsSvcClient{
		LambdaClient: lambdaClient,
		SQSClient:    sqsClient,
		CWClient:     cwClient,
	}, nil
}

func (c *AwsSvcClient) LambdaExists(ctx context.Context, lambdaName string) bool {
	log := log.FromContext(ctx)

	resp, err := c.LambdaClient.GetFunction(ctx, &lambda.GetFunctionInput{
		FunctionName: &lambdaName,
	})
	if err != nil {
		log.Error(err, "unable to get aws lambda function details")
		return false
	}

	return resp.Configuration != nil
}

func (c *AwsSvcClient) LambdaConcurrency(ctx context.Context, lambdaName string) (int32, error) {
	log := log.FromContext(ctx)

	resp, err := c.LambdaClient.GetFunction(ctx, &lambda.GetFunctionInput{
		FunctionName: &lambdaName,
	})
	if err != nil {
		log.Error(err, "unable to get aws lambda function details")
		return -1, err
	}

	return *resp.Concurrency.ReservedConcurrentExecutions, nil
}

func (c *AwsSvcClient) SQSTriggerForLambdaExists(ctx context.Context, lambdaName, sqsName string) bool {
	log := log.FromContext(ctx)

	resp, err := c.LambdaClient.ListEventSourceMappings(ctx, &lambda.ListEventSourceMappingsInput{
		FunctionName: &lambdaName,
	})
	if err != nil {
		log.Error(err, "unable to get aws lambda function and input trigger sqs details")
		return false
	}

	for _, mapping := range resp.EventSourceMappings {
		if mapping.EventSourceArn != nil {
			arnParts := strings.Split(*mapping.EventSourceArn, ":")
			if len(arnParts) > 0 && arnParts[len(arnParts)-1] == sqsName {
				return true
			}
		}
	}

	return false
}

func (c *AwsSvcClient) getVisibilityMetrics(ctx context.Context, sqsName string) (float64, error) {
	log := log.FromContext(ctx)

	// Set the time range for the metric query (last 5 minutes)
	startTime := time.Now().Add(-5 * time.Minute)
	endTime := time.Now()
	metricID := "m1"

	// Define the input for the GetMetricData API
	params := &cloudwatch.GetMetricDataInput{
		EndTime:   &endTime,
		StartTime: &startTime,
		MetricDataQueries: []types.MetricDataQuery{
			{
				Id: &metricID,
				MetricStat: &types.MetricStat{
					Metric: &types.Metric{
						Namespace:  aws.String("AWS/SQS"),
						MetricName: aws.String(visibilityMetricName),
						Dimensions: []types.Dimension{
							{
								Name:  aws.String("QueueName"),
								Value: &sqsName,
							},
						},
					},
					Period: aws.Int32(60), // 1 minute granularity
					Stat:   aws.String("Average"),
				},
			},
		},
	}

	// Get the metric data
	resp, err := c.CWClient.GetMetricData(ctx, params)
	if err != nil {
		return 0.0, fmt.Errorf("error fetching metric data: %v", err)
	}

	// Calculate the average visibility metric over the last 5 minutes
	totalVisibility := 0.0
	for _, metricDataResult := range resp.MetricDataResults {
		for _, value := range metricDataResult.Values {
			totalVisibility += aws.ToFloat64(&value)
		}
	}

	avgVisibility := totalVisibility / float64(len(resp.MetricDataResults))

	log.Info("sqs average visibility message count", "avg", avgVisibility)
	return avgVisibility, nil
}

func (c *AwsSvcClient) AdjustLambdaConcurrency(ctx context.Context, lambdaName, sqsName string, threshold, minConcurrency, maxConcurrency, step int32) (int32, error) {
	log := log.FromContext(ctx)

	// Get the SQS visibility metrics
	avgVisibility, err := c.getVisibilityMetrics(ctx, sqsName)
	if err != nil {
		log.Error(err, "reteriving sqs message vistibility detail failed")
		return -1, fmt.Errorf("error getting visibility metrics: %v", err)
	}

	currentConcurrency, err := c.LambdaConcurrency(ctx, lambdaName)
	if err != nil {
		log.Error(err, "error while reteriving lambda function concurrency")
		return -1, fmt.Errorf("error while reteriving lambda function concurrency: %v", err)
	}

	// Determine the new Lambda concurrency based on the visibility metrics
	newConcurrency := calculateNewConcurrency(currentConcurrency, avgVisibility, threshold, minConcurrency, maxConcurrency, step)

	// Update the Lambda concurrency
	err = c.updateLambdaConcurrency(ctx, lambdaName, newConcurrency)
	if err != nil {
		log.Error(err, "error on updating concurrency of lambda function")
		return currentConcurrency, fmt.Errorf("error updating lambda concurrency: %v", err)
	}

	log.Info("lambda function concurrency adjusted", "OldConcurrency", currentConcurrency, "NewConcurrency", newConcurrency)
	return newConcurrency, nil
}

func calculateNewConcurrency(currentConcurrency int32, avgVisibility float64, threshold, minConcurrency, maxConcurrency, step int32) int32 {
	// Calculate the new Lambda concurrency based on the visibility metrics
	if avgVisibility > float64(threshold) {
		newConcurrency := currentConcurrency + int32(step)
		return int32(math.Min(float64(newConcurrency), float64(maxConcurrency)))
	} else {
		newConcurrency := currentConcurrency - int32(step)
		return int32(math.Max(float64(newConcurrency), float64(minConcurrency)))
	}
}

func (c *AwsSvcClient) updateLambdaConcurrency(ctx context.Context, lambdaName string, newConcurrency int32) error {
	// Update the Lambda concurrency
	_, err := c.LambdaClient.PutFunctionConcurrency(ctx, &lambda.PutFunctionConcurrencyInput{
		FunctionName:                 &lambdaName,
		ReservedConcurrentExecutions: aws.Int32(newConcurrency),
	})
	return err
}
