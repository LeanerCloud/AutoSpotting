// Copyright 2017-2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not use this file except in compliance with the License. A copy of the License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package sampling

import (
	"math/rand"

	"github.com/aws/aws-xray-sdk-go/pattern"
)

// Rule represents a single entry in a sampling ruleset.
type Rule struct {
	ServiceName string  `json:"service_name"`
	HTTPMethod  string  `json:"http_method"`
	URLPath     string  `json:"url_path"`
	FixedTarget uint64  `json:"fixed_target"`
	Rate        float64 `json:"rate"`
	reservoir   *Reservoir
}

// AppliesTo returns true when the rule applies to the given parameters
func (sr *Rule) AppliesTo(serviceName string, path string, method string) bool {
	return pattern.WildcardMatchCaseInsensitive(sr.ServiceName, serviceName) && pattern.WildcardMatchCaseInsensitive(sr.URLPath, path) && pattern.WildcardMatchCaseInsensitive(sr.HTTPMethod, method)
}

// Sample returns true when the rule's reservoir is not full or
// when a randomly generated float is less than the rule's rate
func (sr *Rule) Sample() bool {
	if sr.reservoir.Take() {
		return true
	}
	return rand.Float64() < sr.Rate
}
