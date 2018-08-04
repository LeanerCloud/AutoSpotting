// Copyright 2017-2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not use this file except in compliance with the License. A copy of the License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package header

import (
	"bytes"
	"strings"
)

const (
	// RootPrefix is the prefix for
	// Root attribute in X-Amzn-Trace-Id.
	RootPrefix = "Root="

	// ParentPrefix is the prefix for
	// Parent attribute in X-Amzn-Trace-Id.
	ParentPrefix = "Parent="

	// SampledPrefix is the prefix for
	// Sampled attribute in X-Amzn-Trace-Id.
	SampledPrefix = "Sampled="

	// SelfPrefix is the prefix for
	// Self attribute in X-Amzn-Trace-Id.
	SelfPrefix = "Self="
)

// SamplingDecision is a string representation of
// whether or not the current segment has been sampled.
type SamplingDecision string

const (
	// Sampled indicates the current segment has been
	// sampled and will be sent to the X-Ray daemon.
	Sampled SamplingDecision = "Sampled=1"

	// NotSampled indicates the current segment has
	// not been sampled.
	NotSampled SamplingDecision = "Sampled=0"

	// Requested indicates sampling decision will be
	// made by the downstream service and propagated
	// back upstream in the response.
	Requested SamplingDecision = "Sampled=?"

	// Unknown indicates no sampling decision will be made.
	Unknown SamplingDecision = ""
)

func samplingDecision(s string) SamplingDecision {
	if s == string(Sampled) {
		return Sampled
	} else if s == string(NotSampled) {
		return NotSampled
	} else if s == string(Requested) {
		return Requested
	}
	return Unknown
}

// Header is the value of X-Amzn-Trace-Id.
type Header struct {
	TraceID          string
	ParentID         string
	SamplingDecision SamplingDecision

	AdditionalData map[string]string
}

// FromString gets individual value for each item in Header struct.
func FromString(s string) *Header {
	ret := &Header{
		SamplingDecision: Unknown,
		AdditionalData:   make(map[string]string),
	}
	parts := strings.Split(s, ";")
	for i := range parts {
		p := strings.TrimSpace(parts[i])
		value, valid := valueFromKeyValuePair(p)
		if valid {
			if strings.HasPrefix(p, RootPrefix) {
				ret.TraceID = value
			} else if strings.HasPrefix(p, ParentPrefix) {
				ret.ParentID = value
			} else if strings.HasPrefix(p, SampledPrefix) {
				ret.SamplingDecision = samplingDecision(p)
			} else if !strings.HasPrefix(p, SelfPrefix) {
				key, valid := keyFromKeyValuePair(p)
				if valid {
					ret.AdditionalData[key] = value
				}
			}
		}
	}
	return ret
}

// String returns a string representation for header.
func (h Header) String() string {
	var p [][]byte
	if h.TraceID != "" {
		p = append(p, []byte(RootPrefix+h.TraceID))
	}
	if h.ParentID != "" {
		p = append(p, []byte(ParentPrefix+h.ParentID))
	}
	p = append(p, []byte(h.SamplingDecision))
	for key := range h.AdditionalData {
		p = append(p, []byte(key+"="+h.AdditionalData[key]))
	}
	return string(bytes.Join(p, []byte(";")))
}

func keyFromKeyValuePair(s string) (string, bool) {
	e := strings.Index(s, "=")
	if -1 != e {
		return s[:e], true
	}
	return "", false
}

func valueFromKeyValuePair(s string) (string, bool) {
	e := strings.Index(s, "=")
	if -1 != e {
		return s[e+1:], true
	}
	return "", false
}
