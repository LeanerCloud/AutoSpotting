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
	"crypto/rand"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/aws/aws-xray-sdk-go/header"
	"github.com/aws/aws-xray-sdk-go/internal/plugins"
	log "github.com/cihub/seelog"
)

// NewTraceID generates a string format of random trace ID.
func NewTraceID() string {
	var r [12]byte
	_, err := rand.Read(r[:])
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("1-%08x-%02x", time.Now().Unix(), r)
}

// NewSegmentID generates a string format of segment ID.
func NewSegmentID() string {
	var r [8]byte
	_, err := rand.Read(r[:])
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%02x", r)
}

// BeginFacadeSegment creates a Segment for a given name and context.
func BeginFacadeSegment(ctx context.Context, name string, h *header.Header) (context.Context, *Segment) {
	seg := basicSegment(name, h)

	cfg := GetRecorder(ctx)
	seg.assignConfiguration(cfg)

	return context.WithValue(ctx, ContextKey, seg), seg
}

// BeginSegment creates a Segment for a given name and context.
func BeginSegment(ctx context.Context, name string) (context.Context, *Segment) {
	seg := basicSegment(name, nil)

	cfg := GetRecorder(ctx)
	seg.assignConfiguration(cfg)

	seg.Lock()
	defer seg.Unlock()

	seg.addPlugin(plugins.InstancePluginMetadata)
	seg.addSDKAndServiceInformation()
	if seg.ParentSegment.GetConfiguration().ServiceVersion != "" {
		seg.GetService().Version = seg.ParentSegment.GetConfiguration().ServiceVersion
	}

	go func() {
		select {
		case <-ctx.Done():
			seg.handleContextDone()
		}
	}()

	return context.WithValue(ctx, ContextKey, seg), seg
}

func basicSegment(name string, h *header.Header) *Segment {
	if len(name) > 200 {
		name = name[:200]
	}
	seg := &Segment{parent: nil}
	log.Tracef("Beginning segment named %s", name)
	seg.ParentSegment = seg

	seg.Lock()
	defer seg.Unlock()

	seg.Name = name
	seg.StartTime = float64(time.Now().UnixNano()) / float64(time.Second)
	seg.InProgress = true

	if h == nil {
		seg.TraceID = NewTraceID()
		seg.ID = NewSegmentID()
		seg.Sampled = true
	} else {
		seg.Facade = true
		seg.ID = h.ParentID
		seg.TraceID = h.TraceID
		seg.Sampled = h.SamplingDecision == header.Sampled
	}

	return seg
}

// assignConfiguration assigns value to seg.Configuration
func (seg *Segment) assignConfiguration(cfg *Config) {
	seg.Lock()
	if cfg == nil {
		seg.GetConfiguration().ContextMissingStrategy = globalCfg.contextMissingStrategy
		seg.GetConfiguration().ExceptionFormattingStrategy = globalCfg.exceptionFormattingStrategy
		seg.GetConfiguration().SamplingStrategy = globalCfg.samplingStrategy
		seg.GetConfiguration().StreamingStrategy = globalCfg.streamingStrategy
		seg.GetConfiguration().ServiceVersion = globalCfg.serviceVersion
	} else {
		if cfg.ContextMissingStrategy != nil {
			seg.GetConfiguration().ContextMissingStrategy = cfg.ContextMissingStrategy
		} else {
			seg.GetConfiguration().ContextMissingStrategy = globalCfg.contextMissingStrategy
		}

		if cfg.ExceptionFormattingStrategy != nil {
			seg.GetConfiguration().ExceptionFormattingStrategy = cfg.ExceptionFormattingStrategy
		} else {
			seg.GetConfiguration().ExceptionFormattingStrategy = globalCfg.exceptionFormattingStrategy
		}

		if cfg.SamplingStrategy != nil {
			seg.GetConfiguration().SamplingStrategy = cfg.SamplingStrategy
		} else {
			seg.GetConfiguration().SamplingStrategy = globalCfg.samplingStrategy
		}

		if cfg.StreamingStrategy != nil {
			seg.GetConfiguration().StreamingStrategy = cfg.StreamingStrategy
		} else {
			seg.GetConfiguration().StreamingStrategy = globalCfg.streamingStrategy
		}

		if cfg.ServiceVersion != "" {
			seg.GetConfiguration().ServiceVersion = cfg.ServiceVersion
		} else {
			seg.GetConfiguration().ServiceVersion = globalCfg.serviceVersion
		}
	}
	seg.Unlock()
}

// BeginSubsegment creates a subsegment for a given name and context.
func BeginSubsegment(ctx context.Context, name string) (context.Context, *Segment) {
	if len(name) > 200 {
		name = name[:200]
	}

	parent := &Segment{}
	// first time to create facade segment
	if getTraceHeaderFromContext(ctx) != nil && GetSegment(ctx) == nil {
		_, parent = newFacadeSegment(ctx)
	} else {
		parent = GetSegment(ctx)
		if parent == nil {
			cfg := GetRecorder(ctx)
			failedMessage := fmt.Sprintf("failed to begin subsegment named '%v': segment cannot be found.", name)
			if cfg != nil && cfg.ContextMissingStrategy != nil {
				cfg.ContextMissingStrategy.ContextMissing(failedMessage)
			} else {
				globalCfg.ContextMissingStrategy().ContextMissing(failedMessage)
			}
			return ctx, nil
		}
	}

	seg := &Segment{parent: parent}
	log.Tracef("Beginning subsegment named %s", name)

	seg.Lock()
	defer seg.Unlock()

	parent.Lock()
	seg.ParentSegment = parent.ParentSegment
	seg.ParentSegment.totalSubSegments++
	parent.rawSubsegments = append(parent.rawSubsegments, seg)
	parent.openSegments++
	parent.Unlock()

	seg.ID = NewSegmentID()
	seg.Name = name
	seg.StartTime = float64(time.Now().UnixNano()) / float64(time.Second)
	seg.InProgress = true

	return context.WithValue(ctx, ContextKey, seg), seg
}

// NewSegmentFromHeader creates a segment for downstream call and add information to the segment that gets from HTTP header.
func NewSegmentFromHeader(ctx context.Context, name string, h *header.Header) (context.Context, *Segment) {
	con, seg := BeginSegment(ctx, name)

	if h.TraceID != "" {
		seg.TraceID = h.TraceID
	}
	if h.ParentID != "" {
		seg.ParentID = h.ParentID
	}

	seg.Sampled = h.SamplingDecision == header.Sampled
	switch h.SamplingDecision {
	case header.Sampled:
		log.Trace("Incoming header decided: Sampled=true")
	case header.NotSampled:
		log.Trace("Incoming header decided: Sampled=false")
	}

	seg.IncomingHeader = h
	seg.RequestWasTraced = true

	return con, seg
}

// Close a segment.
func (seg *Segment) Close(err error) {
	seg.Lock()
	defer seg.Unlock()
	if seg.parent != nil {
		log.Tracef("Closing subsegment named %s", seg.Name)
	} else {
		log.Tracef("Closing segment named %s", seg.Name)
	}
	seg.EndTime = float64(time.Now().UnixNano()) / float64(time.Second)
	seg.InProgress = false

	if err != nil {
		seg.addError(err)
	}

	seg.flush()
}

// CloseAndStream closes a subsegment and sends it.
func (subseg *Segment) CloseAndStream(err error) {
	subseg.Lock()

	if subseg.parent != nil {
		log.Tracef("Ending subsegment named: %s", subseg.Name)
		subseg.EndTime = float64(time.Now().UnixNano()) / float64(time.Second)
		subseg.InProgress = false
		subseg.Emitted = true
		if subseg.parent.RemoveSubsegment(subseg) {
			log.Tracef("Removing subsegment named: %s", subseg.Name)
		}
	}

	if err != nil {
		subseg.addError(err)
	}

	subseg.beforeEmitSubsegment(subseg.parent)
	subseg.Unlock()

	Emit(subseg)
}

// RemoveSubsegment removes a subsegment child from a segment or subsegment.
func (seg *Segment) RemoveSubsegment(remove *Segment) bool {
	seg.Lock()
	defer seg.Unlock()

	for i, v := range seg.rawSubsegments {
		if v == remove {
			seg.rawSubsegments[i] = seg.rawSubsegments[len(seg.rawSubsegments)-1]
			seg.rawSubsegments[len(seg.rawSubsegments)-1] = nil
			seg.rawSubsegments = seg.rawSubsegments[:len(seg.rawSubsegments)-1]

			seg.totalSubSegments--
			seg.openSegments--
			return true
		}
	}
	return false
}

func (seg *Segment) handleContextDone() {
	seg.Lock()
	defer seg.Unlock()

	seg.ContextDone = true
	if !seg.InProgress && !seg.Emitted {
		seg.flush()
	}
}

func (seg *Segment) flush() {
	if (seg.openSegments == 0 && seg.EndTime > 0) || seg.ContextDone {
		if seg.parent == nil {
			seg.Emitted = true
			Emit(seg)
		} else if seg.parent != nil && seg.parent.Facade {
			seg.Emitted = true
			seg.beforeEmitSubsegment(seg.parent)
			log.Tracef("emit lambda subsegment named: %v", seg.Name)
			Emit(seg)
		} else {
			seg.parent.safeFlush()
		}
	}
}

func (seg *Segment) safeFlush() {
	seg.Lock()
	defer seg.Unlock()
	seg.openSegments--
	seg.flush()
}

func (seg *Segment) root() *Segment {
	if seg.parent == nil {
		return seg
	}
	return seg.parent.root()
}

func (seg *Segment) addPlugin(metadata *plugins.PluginMetadata) {
	// Only called within a seg locked code block
	if metadata == nil {
		return
	}

	if metadata.EC2Metadata != nil {
		seg.GetAWS()[plugins.EC2ServiceName] = metadata.EC2Metadata
	}

	if metadata.ECSMetadata != nil {
		seg.GetAWS()[plugins.ECSServiceName] = metadata.ECSMetadata
	}

	if metadata.BeanstalkMetadata != nil {
		seg.GetAWS()[plugins.EBServiceName] = metadata.BeanstalkMetadata
	}

	if metadata.Origin != "" {
		seg.Origin = metadata.Origin
	}
}

func (seg *Segment) addSDKAndServiceInformation() {
	seg.GetAWS()["xray"] = SDK{Version: SDKVersion, Type: SDKType}

	seg.GetService().Compiler = runtime.Compiler
	seg.GetService().CompilerVersion = runtime.Version()
}

func (sub *Segment) beforeEmitSubsegment(seg *Segment) {
	// Only called within a subsegment locked code block
	sub.TraceID = seg.root().TraceID
	sub.ParentID = seg.ID
	sub.Type = "subsegment"
	sub.RequestWasTraced = seg.RequestWasTraced
	sub.parent = nil
}

// AddAnnotation allows adding an annotation to the segment.
func (seg *Segment) AddAnnotation(key string, value interface{}) error {
	switch value.(type) {
	case bool, int, uint, float32, float64, string:
	default:
		return fmt.Errorf("failed to add annotation key: %q value: %q to subsegment %q. value must be of type string, number or boolean", key, value, seg.Name)
	}

	seg.Lock()
	defer seg.Unlock()

	if seg.Annotations == nil {
		seg.Annotations = map[string]interface{}{}
	}
	seg.Annotations[key] = value
	return nil
}

// AddMetadata allows adding metadata to the segment.
func (seg *Segment) AddMetadata(key string, value interface{}) error {
	seg.Lock()
	defer seg.Unlock()

	if seg.Metadata == nil {
		seg.Metadata = map[string]map[string]interface{}{}
	}
	if seg.Metadata["default"] == nil {
		seg.Metadata["default"] = map[string]interface{}{}
	}
	seg.Metadata["default"][key] = value
	return nil
}

// AddMetadataToNamespace allows adding a namespace into metadata for the segment.
func (seg *Segment) AddMetadataToNamespace(namespace string, key string, value interface{}) error {
	seg.Lock()
	defer seg.Unlock()

	if seg.Metadata == nil {
		seg.Metadata = map[string]map[string]interface{}{}
	}
	if seg.Metadata[namespace] == nil {
		seg.Metadata[namespace] = map[string]interface{}{}
	}
	seg.Metadata[namespace][key] = value
	return nil
}

// AddError allows adding an error to the segment.
func (seg *Segment) AddError(err error) error {
	seg.Lock()
	defer seg.Unlock()

	seg.addError(err)

	return nil
}

func (seg *Segment) addError(err error) error {
	seg.Fault = true
	seg.GetCause().WorkingDirectory, _ = os.Getwd()
	seg.GetCause().Exceptions = append(seg.GetCause().Exceptions, seg.ParentSegment.GetConfiguration().ExceptionFormattingStrategy.ExceptionFromError(err))

	return nil
}
