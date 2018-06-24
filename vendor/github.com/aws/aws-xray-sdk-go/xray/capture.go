// Copyright 2017-2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not use this file except in compliance with the License. A copy of the License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package xray

import (
	"context"
	"fmt"
)

// Capture traces the provided synchronous function by
// beginning and closing a subsegment around its execution.
func Capture(ctx context.Context, name string, fn func(context.Context) error) (err error) {
	c, seg := BeginSubsegment(ctx, name)

	defer func() {
		if seg != nil {
			seg.Close(err)
		} else {
			cfg := GetRecorder(ctx)
			failedMessage := fmt.Sprintf("failed to end subsegment: subsegment '%v' cannot be found.", name)
			if cfg != nil && cfg.ContextMissingStrategy != nil {
				cfg.ContextMissingStrategy.ContextMissing(failedMessage)
			} else {
				globalCfg.ContextMissingStrategy().ContextMissing(failedMessage)
			}
		}
	}()

	defer func() {
		if p := recover(); p != nil {
			err = seg.ParentSegment.GetConfiguration().ExceptionFormattingStrategy.Panicf("%v", p)
			panic(p)
		}
	}()

	if c == nil && seg == nil {
		err = fn(ctx)
	} else {
		err = fn(c)
	}

	return err
}

// CaptureAsync traces an arbitrary code segment within a goroutine.
// Use CaptureAsync instead of manually calling Capture within a goroutine
// to ensure the segment is flushed properly.
func CaptureAsync(ctx context.Context, name string, fn func(context.Context) error) {
	started := make(chan struct{})
	go Capture(ctx, name, func(ctx context.Context) error {
		close(started)
		return fn(ctx)
	})
	<-started
}
