// Copyright 2017-2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not use this file except in compliance with the License. A copy of the License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package sampling

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var maxRate uint64 = 1e8

// Reservoir allows a specified amount of `Take()`s per second.
// Support for atomic operations on uint64 is required.
// More information: https://golang.org/pkg/sync/atomic/#pkg-note-BUG
type Reservoir struct {
	maskedCounter uint64
	perSecond     uint64
	mutex         sync.Mutex // don't use embedded struct to ensure 64 bit alignment for maskedCounter.
}

// NewReservoir creates a new reservoir with a specified perSecond
// sampling capacity. The maximum supported sampling capacity per
// second is currently 100,000,000. An error is returned if the
// desired capacity is greater than this maximum value.
func NewReservoir(perSecond uint64) (*Reservoir, error) {
	if perSecond >= maxRate {
		return nil, fmt.Errorf("desired sampling capacity of %d is greater than maximum supported rate %d", perSecond, maxRate)
	}
	return &Reservoir{
		maskedCounter: 0,
		perSecond:     perSecond,
	}, nil
}

// Take returns true when the reservoir has remaining sampling
// capacity for the current epoch. Take returns false when the
// reservoir has no remaining sampling capacity for the current
// epoch. The sampling capacity decrements by one each time
// Take returns true.
func (r *Reservoir) Take() bool {
	now := uint64(time.Now().Unix())
	counterNewVal := atomic.AddUint64(&r.maskedCounter, 1)
	previousTimestamp := extractTime(counterNewVal)

	if previousTimestamp != now {
		r.mutex.Lock()
		beforeUpdate := atomic.LoadUint64(&r.maskedCounter)
		timestampBeforeUpdate := extractTime(beforeUpdate)

		if timestampBeforeUpdate != now {
			valueToSet := timestampToCounter(now)
			atomic.StoreUint64(&r.maskedCounter, valueToSet)
		}

		counterNewVal = atomic.AddUint64(&r.maskedCounter, 1)
		r.mutex.Unlock()
	}

	newCounterValue := extractCounter(counterNewVal)
	return newCounterValue <= r.perSecond
}

func extractTime(maskedCounter uint64) uint64 {
	return maskedCounter / maxRate
}

func extractCounter(maskedCounter uint64) uint64 {
	return maskedCounter % maxRate
}

func timestampToCounter(timestamp uint64) uint64 {
	return timestamp * maxRate
}
