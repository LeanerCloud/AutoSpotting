//
// Copyright 2017 Alsanium, SAS. or its affiliates. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package runtime

// CognitoIdentity provides information about the Amazon Cognito identity
// provider when Lambda function invoked through the AWS Mobile SDK.
//
// For more information about the exact values for a specific mobile platform,
// see Identity Context in the AWS Mobile SDK for iOS Developer Guide, and
// Identity Context in the AWS Mobile SDK for Android Developer Guide.
//
// http://aws.amazon.com/cognito/
// http://docs.aws.amazon.com/mobile/sdkforios/developerguide/lambda.html#identitycontext
// http://docs.aws.amazon.com/mobile/sdkforandroid/developerguide/lambda.html#identity-context
type CognitoIdentity struct {
	// Amazon Cognito identity ID
	IdentityID string `json:"cognito_identity_id"`

	// Amazon Cognito identity pool ID
	IdentityPoolID string `json:"cognito_identity_pool_id"`
}

// Client provides information about the client application when Lambda function
// invoked through the AWS Mobile SDK.
//
// For more information about the exact values for a specific mobile platform,
// see Client Context in the AWS Mobile SDK for iOS Developer Guide, and Client
// Context in the AWS Mobile SDK for Android Developer Guide.
//
// http://docs.aws.amazon.com/mobile/sdkforios/developerguide/lambda.html#clientcontext
// http://docs.aws.amazon.com/mobile/sdkforandroid/developerguide/lambda.html#client-context
type Client struct {
	InstallationID string `json:"installation_id"`
	AppTitle       string `json:"app_title"`
	AppVersionName string `json:"app_version_name"`
	AppVersionCode string `json:"app_version_code"`
	AppPackageName string `json:"app_package_name"`
}

// ClientContext provides information about the client application and device
// when Lambda function invoked through the AWS Mobile SDK.
//
// For more information about the exact values for a specific mobile platform,
// see Client Context in the AWS Mobile SDK for iOS Developer Guide, and Client
// Context in the AWS Mobile SDK for Android Developer Guide.
//
// http://docs.aws.amazon.com/mobile/sdkforios/developerguide/lambda.html#clientcontext
// http://docs.aws.amazon.com/mobile/sdkforandroid/developerguide/lambda.html#client-context
type ClientContext struct {
	// Client information provided by AWS Mobile SDK.
	Client *Client `json:"client,omitempty"`

	// Custom values set by mobile client application.
	Custom map[string]string `json:"custom"`

	// Environment information provided by AWS Mobile SDK.
	Environment map[string]string `json:"env"`
}

// Context provides information about Lambda execution environment.
//
// For example, you can use the Context to determine the
// CloudWatch log stream associated with the function or use the ClientContext
// property of the Context to learn more about the application calling the
// Lambda function (when invoked through the AWS Mobile SDK).
type Context struct {
	// The name of the Lambda function that is executing.
	FunctionName string `json:"function_name"`

	// The version of the Lambda function that is executing.
	// If an alias is used to invoke the function, then FunctionVersion will be
	// the version the alias points to.
	FunctionVersion string `json:"function_version"`

	// The ARN used to invoke the Lambda function.
	// It can be function ARN or alias ARN. An unqualified ARN executes the
	// $LATEST version and aliases execute the function version they are pointing
	// to.
	InvokedFunctionARN string `json:"invoked_function_arn"`

	// The Memory limit, in MB, configured for the Lambda function.
	MemoryLimitInMB int `json:"memory_limit_in_mb,string"`

	// The AWS request ID associated with the request.
	// This is the ID returned to the client invoked the function. The request ID
	// can be used for any follow up enquiry with AWS support. Note that if Lambda
	// retries the function (for example, in a situation where the Lambda function
	// processing Amazon Kinesis records throw an exception), the request ID
	// remains the same.
	AWSRequestID string `json:"aws_request_id"`

	// The CloudWatch log group name of the Lambda function that is executing.
	// It can be empty if the IAM user provided does not have permission for
	// CloudWatch actions.
	LogGroupName string `json:"log_group_name"`

	// The CloudWatch log stream name of the Lambda function that is executing.
	// It can be empty if the IAM user provided does not have permission for
	// CloudWatch actions.
	LogStreamName string `json:"log_stream_name"`

	// The information about the Amazon Cognito identity provider when the Lambda
	// function invoked through the AWS Mobile SDK.
	// It can be nil.
	Identity *CognitoIdentity `json:"identity,omitempty"`

	// The information about the client application and device when the Lambda
	// function invoked through the AWS Mobile SDK.
	// It can be nil.
	ClientContext *ClientContext `json:"client_context,omitempty"`

	// RemainingTimeInMillis returns remaining execution time till the function
	// will be terminated, in milliseconds.
	//
	// The maximum time limit at which Lambda will terminate the function
	// execution is set at the time the Lambda function is created. Information
	// about the remaining time of function execution can be used to specify
	// function behavior when nearing the timeout.
	RemainingTimeInMillis func() int64 `json:"-"`
}
