/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	awsv1alpha1 "github.com/jittakal/lambda-con-scaler-operator/api/v1alpha1"
)

var (
	lambdaConcurrencyConfig = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "lambda_concurrency_config",
			Help: "Current Lambda concurrency configuration",
		},
		[]string{"namespace", "controller", "lambda_name", "sqs_name"},
	)
)

func init() {
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(lambdaConcurrencyConfig)
}

const (
	lambdaNotExistsError              = "%s lambda does not exists"
	sqsTriggerForLambdaNotExistsError = "%s SQS is not in innput trigger list for %s lambda"
)

// LambdaConcurrencyScalerReconciler reconciles a LambdaConcurrencyScaler object
type LambdaConcurrencyScalerReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	AwsSvcClient *AwsSvcClient
}

//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups=aws.operators.jittakal.io,resources=lambdaconcurrencyscalers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=aws.operators.jittakal.io,resources=lambdaconcurrencyscalers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=aws.operators.jittakal.io,resources=lambdaconcurrencyscalers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the LambdaConcurrencyScaler object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *LambdaConcurrencyScalerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("aws lambda function concurrency auto scaling reconcile", "status", "started")

	lambdaConcurrencyScaler := &awsv1alpha1.LambdaConcurrencyScaler{}
	if err := r.Get(ctx, req.NamespacedName, lambdaConcurrencyScaler); err != nil {
		log.Error(err, "unable to get resources")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if lambdaConcurrencyScaler.Status.State == "" {
		lambdaConcurrencyScaler.Status.State = awsv1alpha1.PENDING_STATE
		r.Status().Update(ctx, lambdaConcurrencyScaler)
	}

	// Adjust Lambda Function Reserved Concurrency Execution Configuration
	if adjustConcurrencyAllowed(lambdaConcurrencyScaler.Status.State, lambdaConcurrencyScaler.Status.AdjustedTimestamp) {
		// Check if AWS Lambda functions  is exists
		if !r.AwsSvcClient.LambdaExists(ctx, lambdaConcurrencyScaler.Spec.AWSLambdaName) {
			lambdaNotExistsErr := fmt.Errorf(lambdaNotExistsError, lambdaConcurrencyScaler.Spec.AWSLambdaName)
			log.Error(lambdaNotExistsErr, "aws lambda does not exists")

			lambdaConcurrencyScaler.Status.State = awsv1alpha1.ERROR_STATE
			r.Status().Update(ctx, lambdaConcurrencyScaler)

			return ctrl.Result{}, nil // do not requeue
		}

		// Check if SQS is input trigger of AWS Lambda function
		if !r.AwsSvcClient.SQSTriggerForLambdaExists(ctx, lambdaConcurrencyScaler.Spec.AWSLambdaName,
			lambdaConcurrencyScaler.Spec.AWSSQSName) {
			sqsTriggerForLambdaNotExistsErr := fmt.Errorf(sqsTriggerForLambdaNotExistsError, lambdaConcurrencyScaler.Spec.AWSSQSName,
				lambdaConcurrencyScaler.Spec.AWSLambdaName)
			log.Error(sqsTriggerForLambdaNotExistsErr, "aws sqs is not in list of input triggers for lambda function")

			lambdaConcurrencyScaler.Status.State = awsv1alpha1.ERROR_STATE
			r.Status().Update(ctx, lambdaConcurrencyScaler)

			return ctrl.Result{}, nil // do not requeue
		}

		lambdaConcurrencyScaler.Status.State = awsv1alpha1.ADJUSTING_STATE
		r.Status().Update(ctx, lambdaConcurrencyScaler)

		// Adjust lambda function reserved concurrency execution configurations
		newConcurrency, err := r.AwsSvcClient.AdjustLambdaConcurrency(ctx, lambdaConcurrencyScaler.Spec.AWSLambdaName,
			lambdaConcurrencyScaler.Spec.AWSSQSName, lambdaConcurrencyScaler.Spec.AWSSQSMsgCountThreshold,
			lambdaConcurrencyScaler.Spec.AWSLambdaMinConcurrency, lambdaConcurrencyScaler.Spec.AWSLambdaMaxConcurrency,
			lambdaConcurrencyScaler.Spec.AWSLambdaStepConcurrency)

		if err != nil {
			log.Error(err, "aws sqs is not in list of input triggers for lambda function")

			lambdaConcurrencyScaler.Status.State = awsv1alpha1.ERROR_STATE
			r.Status().Update(ctx, lambdaConcurrencyScaler)

			return ctrl.Result{}, nil // do not requeue
		}

		lambdaConcurrencyScaler.Status.State = awsv1alpha1.ADJUSTED_STATE
		lambdaConcurrencyScaler.Status.AdjustedTimestamp = metav1.Now()
		lambdaConcurrencyScaler.Status.Concurrency = newConcurrency

		lambdaConcurrencyConfig.WithLabelValues(req.NamespacedName.Name, lambdaConcurrencyScaler.Name,
			lambdaConcurrencyScaler.Spec.AWSLambdaName, lambdaConcurrencyScaler.Spec.AWSSQSName).Set(float64(newConcurrency))

		r.Status().Update(ctx, lambdaConcurrencyScaler)
	}

	time.Sleep(time.Second * 5)
	log.Info("aws lambda function concurrency auto scaling reconcile", "status", "end")
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil // retry after 5 minutes
}

func adjustConcurrencyAllowed(state string, adjustedTimestamp metav1.Time) bool {

	if state != awsv1alpha1.ERROR_STATE && adjustedTimestamp.IsZero() {
		return true
	}

	if state != awsv1alpha1.ERROR_STATE && !adjustedTimestamp.IsZero() {
		// Calculate the time difference
		timeDifference := time.Since(adjustedTimestamp.Time)
		// Check if the time difference is greater than 5 minutes
		if timeDifference.Minutes() >= 5 {
			return true
		}
	}

	return false
}

// SetupWithManager sets up the controller with the Manager.
func (r *LambdaConcurrencyScalerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&awsv1alpha1.LambdaConcurrencyScaler{}).
		Complete(r)
}
