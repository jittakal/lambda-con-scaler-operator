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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	PENDING_STATE   = "PENDING"
	ADJUSTED_STATE  = "ADJUSTED"
	ADJUSTING_STATE = "ADJUSTING"
	ERROR_STATE     = "ERROR"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// LambdaConcurrencyScalerSpec defines the desired state of LambdaConcurrencyScaler
type LambdaConcurrencyScalerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Name of AWS SQS
	AWSSQSName string `json:"awsSQSName"`

	// AWS SQS Message Visible Count Threshold
	AWSSQSMsgCountThreshold int32 `json:"awsSQSMsgCountThreshold"`

	// Name of AWS Lambda
	AWSLambdaName string `json:"awsLambdaName"`

	// Minimum Reserved Concurrency Configuration of AWS Lambda
	AWSLambdaMinConcurrency int32 `json:"awsLambdaMinConcurrency"`

	// Minimum Reserved Concurrency Configuration of AWS Lambda
	AWSLambdaMaxConcurrency int32 `json:"awsLamndaMaxConcurrency"`

	// Steps to Increase or Decrease Reserved Concurrency Configuration of AWS Lambda
	AWSLambdaStepConcurrency int32 `json:"awsLambdaStepConcurrency"`
}

// LambdaConcurrencyScalerStatus defines the observed state of LambdaConcurrencyScaler
type LambdaConcurrencyScalerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// State of Concurrency Adjustment
	State string `json:"state"`

	// Desired Concurrency
	Concurrency int32 `json:"concurrency"`

	// Time Since last concurrency adjusted
	AdjustedTimestamp metav1.Time `json:"adjustedTimestamp,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Concurrency",type=integer,JSONPath=`.status.concurrency`
//+kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
//+kubebuilder:printcolumn:name="AdjustedTimestamp",type=date,JSONPath=`.status.adjustedTimestamp`

// LambdaConcurrencyScaler is the Schema for the lambdaconcurrencyscalers API
type LambdaConcurrencyScaler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LambdaConcurrencyScalerSpec   `json:"spec,omitempty"`
	Status LambdaConcurrencyScalerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// LambdaConcurrencyScalerList contains a list of LambdaConcurrencyScaler
type LambdaConcurrencyScalerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LambdaConcurrencyScaler `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LambdaConcurrencyScaler{}, &LambdaConcurrencyScalerList{})
}
