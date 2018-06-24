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
	"errors"
)

// ContextKeytype defines integer to be type of ContextKey.
type ContextKeytype int

// ContextKey returns a pointer to a newly allocated zero value of ContextKeytype.
var ContextKey = new(ContextKeytype)

// ErrRetrieveSegment happens when a segment cannot be retrieved
var ErrRetrieveSegment = errors.New("unable to retrieve segment")

// RecorderContextKey records the key for Config value.
type RecorderContextKey struct{}

// GetRecorder returns a pointer to the config struct provided
// in ctx, or nil if no config is set.
func GetRecorder(ctx context.Context) *Config {
	if r, ok := ctx.Value(RecorderContextKey{}).(*Config); ok {
		return r
	}
	return nil
}

// GetSegment returns a pointer to the segment or subsegment provided
// in ctx, or nil if no segment or subsegment is found.
func GetSegment(ctx context.Context) *Segment {
	if seg, ok := ctx.Value(ContextKey).(*Segment); ok {
		return seg
	}
	return nil
}

// TraceID returns the canonical ID of the cross-service trace from the
// given segment in ctx. The value can be used in X-Ray's UI to uniquely
// identify the code paths executed. If no segment is provided in ctx,
// an empty string is returned.
func TraceID(ctx context.Context) string {
	if seg, ok := ctx.Value(ContextKey).(*Segment); ok {
		return seg.TraceID
	}
	return ""
}

// RequestWasTraced returns true if the context contains an X-Ray segment
// that was created from an HTTP request that contained a trace header.
// This is useful to ensure that a service is only called from X-Ray traced
// services.
func RequestWasTraced(ctx context.Context) bool {
	for seg := GetSegment(ctx); seg != nil; seg = seg.parent {
		if seg.RequestWasTraced {
			return true
		}
	}
	return false
}

// DetachContext returns a new context with the existing segment.
// This is useful for creating background tasks which won't be cancelled
// when a request completes.
func DetachContext(ctx context.Context) context.Context {
	return context.WithValue(context.Background(), ContextKey, GetSegment(ctx))
}

// AddAnnotation adds an annotation to the provided segment or subsegment in ctx.
func AddAnnotation(ctx context.Context, key string, value interface{}) error {
	if seg := GetSegment(ctx); seg != nil {
		return seg.AddAnnotation(key, value)
	}
	return ErrRetrieveSegment
}

// AddMetadata adds a metadata to the provided segment or subsegment in ctx.
func AddMetadata(ctx context.Context, key string, value interface{}) error {
	if seg := GetSegment(ctx); seg != nil {
		return seg.AddMetadata(key, value)
	}
	return ErrRetrieveSegment
}

// AddMetadataToNamespace adds a namespace to the provided segment's or subsegment's metadata in ctx.
func AddMetadataToNamespace(ctx context.Context, namespace string, key string, value interface{}) error {
	if seg := GetSegment(ctx); seg != nil {
		return seg.AddMetadataToNamespace(namespace, key, value)
	}
	return ErrRetrieveSegment
}

// AddError adds an error to the provided segment or subsegment in ctx.
func AddError(ctx context.Context, err error) error {
	if seg := GetSegment(ctx); seg != nil {
		return seg.AddError(err)
	}
	return ErrRetrieveSegment
}
