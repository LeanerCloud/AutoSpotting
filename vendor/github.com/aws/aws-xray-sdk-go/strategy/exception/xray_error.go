// Copyright 2017-2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not use this file except in compliance with the License. A copy of the License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package exception

// XRayError records error type, message,
// and a slice of stack frame pointers.
type XRayError struct {
	Type    string
	Message string
	Stack   []uintptr
}

// Error returns the value of error message.
func (e *XRayError) Error() string {
	return e.Message
}

// StackTrace returns a slice of integer pointers.
func (e *XRayError) StackTrace() []uintptr {
	return e.Stack
}
