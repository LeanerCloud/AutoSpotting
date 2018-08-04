// Copyright 2017-2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not use this file except in compliance with the License. A copy of the License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

// Package ctxmissing provides control over
// the behavior of the X-Ray SDK when subsegments
// are created without a provided parent segment.
package ctxmissing

// Strategy provides an interface for
// implementing context missing strategies.
type Strategy interface {
	ContextMissing(v interface{})
}
