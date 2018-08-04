// Copyright 2017-2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not use this file except in compliance with the License. A copy of the License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package ctxmissing

import (
	log "github.com/cihub/seelog"
)

// RuntimeErrorStrategy provides the AWS_XRAY_CONTEXT_MISSING
// environment variable value for enabling the runtime error
// context missing strategy (panic).
var RuntimeErrorStrategy = "RUNTIME_ERROR"

// LogErrorStrategy provides the AWS_XRAY_CONTEXT_MISSING
// environment variable value for enabling the log error
// context missing strategy.
var LogErrorStrategy = "LOG_ERROR"

// DefaultRuntimeErrorStrategy implements the
// runtime error context missing strategy.
type DefaultRuntimeErrorStrategy struct{}

// DefaultLogErrorStrategy implements the
// log error context missing strategy.
type DefaultLogErrorStrategy struct{}

// NewDefaultRuntimeErrorStrategy initializes
// an instance of DefaultRuntimeErrorStrategy.
func NewDefaultRuntimeErrorStrategy() *DefaultRuntimeErrorStrategy {
	return &DefaultRuntimeErrorStrategy{}
}

// NewDefaultLogErrorStrategy initializes
// an instance of DefaultLogErrorStrategy.
func NewDefaultLogErrorStrategy() *DefaultLogErrorStrategy {
	return &DefaultLogErrorStrategy{}
}

// ContextMissing panics when the segment context is missing.
func (dr *DefaultRuntimeErrorStrategy) ContextMissing(v interface{}) {
	panic(v)
}

// ContextMissing logs an error message when the
// segment context is missing.
func (dl *DefaultLogErrorStrategy) ContextMissing(v interface{}) {
	log.Errorf("Suppressing AWS X-Ray context missing panic: %v", v)
}
