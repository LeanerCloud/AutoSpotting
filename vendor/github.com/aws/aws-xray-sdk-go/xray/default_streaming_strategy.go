// Copyright 2017-2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not use this file except in compliance with the License. A copy of the License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package xray

import (
	"encoding/json"
	"errors"

	log "github.com/cihub/seelog"
)

var defaultMaxSubsegmentCount = 20

// DefaultStreamingStrategy provides a default value of 20
// for the maximum number of subsegments that can be emitted
// in a single UDP packet.
type DefaultStreamingStrategy struct {
	MaxSubsegmentCount int
}

// NewDefaultStreamingStrategy initializes and returns a
// pointer to an instance of DefaultStreamingStrategy.
func NewDefaultStreamingStrategy() (*DefaultStreamingStrategy, error) {
	return &DefaultStreamingStrategy{MaxSubsegmentCount: defaultMaxSubsegmentCount}, nil
}

// NewDefaultStreamingStrategyWithMaxSubsegmentCount initializes
// and returns a pointer to an instance of DefaultStreamingStrategy
// with a custom maximum number of subsegments per UDP packet.
func NewDefaultStreamingStrategyWithMaxSubsegmentCount(maxSubsegmentCount int) (*DefaultStreamingStrategy, error) {
	if maxSubsegmentCount <= 0 {
		return nil, errors.New("maxSubsegmentCount must be a non-negative integer")
	}
	return &DefaultStreamingStrategy{MaxSubsegmentCount: maxSubsegmentCount}, nil
}

// RequiresStreaming returns true when the number of subsegment
// children for a given segment is larger than MaxSubsegmentCount.
func (dSS *DefaultStreamingStrategy) RequiresStreaming(seg *Segment) bool {
	if seg.ParentSegment.Sampled {
		return seg.ParentSegment.totalSubSegments > dSS.MaxSubsegmentCount
	}
	return false
}

// StreamCompletedSubsegments separates subsegments from the provided
// segment tree and sends them to daemon as streamed subsegment UDP packets.
func (dSS *DefaultStreamingStrategy) StreamCompletedSubsegments(seg *Segment) [][]byte {
	log.Trace("Beginning to stream subsegments.")
	var outSegments [][]byte
	for i := 0; i < len(seg.rawSubsegments); i++ {
		child := seg.rawSubsegments[i]
		seg.rawSubsegments[i] = seg.rawSubsegments[len(seg.rawSubsegments)-1]
		seg.rawSubsegments[len(seg.rawSubsegments)-1] = nil
		seg.rawSubsegments = seg.rawSubsegments[:len(seg.rawSubsegments)-1]

		seg.Subsegments[i] = seg.Subsegments[len(seg.Subsegments)-1]
		seg.Subsegments[len(seg.Subsegments)-1] = nil
		seg.Subsegments = seg.Subsegments[:len(seg.Subsegments)-1]

		seg.ParentSegment.totalSubSegments--

		// Add extra information into child subsegment
		child.Lock()
		child.beforeEmitSubsegment(seg)
		cb, _ := json.Marshal(child)
		outSegments = append(outSegments, cb)
		log.Tracef("Streaming subsegment named '%s' from segment tree.", child.Name)
		child.Unlock()

		break
	}
	log.Trace("Finished streaming subsegments.")
	return outSegments
}
