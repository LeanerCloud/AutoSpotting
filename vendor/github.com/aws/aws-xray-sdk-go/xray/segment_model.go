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
	"sync"

	"github.com/aws/aws-xray-sdk-go/header"
	"github.com/aws/aws-xray-sdk-go/strategy/exception"
)

// Segment provides the resource's name, details about the request, and details about the work done.
type Segment struct {
	sync.Mutex
	parent           *Segment
	openSegments     int
	totalSubSegments int
	Sampled          bool           `json:"-"`
	RequestWasTraced bool           `json:"-"` // Used by xray.RequestWasTraced
	ContextDone      bool           `json:"-"`
	Emitted          bool           `json:"-"`
	IncomingHeader   *header.Header `json:"-"`
	ParentSegment    *Segment       `json:"-"` // The root of the Segment tree, the parent Segment (not Subsegment).

	// Required
	TraceID   string  `json:"trace_id,omitempty"`
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	StartTime float64 `json:"start_time"`
	EndTime   float64 `json:"end_time,omitempty"`

	// Optional
	InProgress  bool       `json:"in_progress,omitempty"`
	ParentID    string     `json:"parent_id,omitempty"`
	Fault       bool       `json:"fault,omitempty"`
	Error       bool       `json:"error,omitempty"`
	Throttle    bool       `json:"throttle,omitempty"`
	Cause       *CauseData `json:"cause,omitempty"`
	ResourceARN string     `json:"resource_arn,omitempty"`
	Origin      string     `json:"origin,omitempty"`

	Type         string   `json:"type,omitempty"`
	Namespace    string   `json:"namespace,omitempty"`
	User         string   `json:"user,omitempty"`
	PrecursorIDs []string `json:"precursor_ids,omitempty"`

	HTTP *HTTPData              `json:"http,omitempty"`
	AWS  map[string]interface{} `json:"aws,omitempty"`

	Service *ServiceData `json:"service,omitempty"`

	// SQL
	SQL *SQLData `json:"sql,omitempty"`

	// Metadata
	Annotations map[string]interface{}            `json:"annotations,omitempty"`
	Metadata    map[string]map[string]interface{} `json:"metadata,omitempty"`

	// Children
	Subsegments    []json.RawMessage `json:"subsegments,omitempty"`
	rawSubsegments []*Segment

	// Configuration
	Configuration *Config `json:"-"`

	// Lambda
	Facade bool `json:"-"`
}

// CauseData provides the shape for unmarshalling data that records exception.
type CauseData struct {
	WorkingDirectory string                `json:"working_directory,omitempty"`
	Paths            []string              `json:"paths,omitempty"`
	Exceptions       []exception.Exception `json:"exceptions,omitempty"`
}

// HTTPData provides the shape for unmarshalling request and response data.
type HTTPData struct {
	Request  *RequestData  `json:"request,omitempty"`
	Response *ResponseData `json:"response,omitempty"`
}

// RequestData provides the shape for unmarshalling request data.
type RequestData struct {
	Method        string `json:"method,omitempty"`
	URL           string `json:"url,omitempty"` // http(s)://host/path
	ClientIP      string `json:"client_ip,omitempty"`
	UserAgent     string `json:"user_agent,omitempty"`
	XForwardedFor bool   `json:"x_forwarded_for,omitempty"`
	Traced        bool   `json:"traced,omitempty"`
}

// ResponseData provides the shape for unmarshalling response data.
type ResponseData struct {
	Status        int `json:"status,omitempty"`
	ContentLength int `json:"content_length,omitempty"`
}

// ServiceData provides the shape for unmarshalling service version.
type ServiceData struct {
	Version         string `json:"version,omitempty"`
	CompilerVersion string `json:"compiler_version,omitempty"`
	Compiler        string `json:"compiler,omitempty"`
}

// SQLData provides the shape for unmarshalling sql data.
type SQLData struct {
	ConnectionString string `json:"connection_string,omitempty"`
	URL              string `json:"url,omitempty"` // host:port/database
	DatabaseType     string `json:"database_type,omitempty"`
	DatabaseVersion  string `json:"database_version,omitempty"`
	DriverVersion    string `json:"driver_version,omitempty"`
	User             string `json:"user,omitempty"`
	Preparation      string `json:"preparation,omitempty"` // "statement" / "call"
	SanitizedQuery   string `json:"sanitized_query,omitempty"`
}

// DownstreamHeader returns a header for passing to downstream calls.
func (s *Segment) DownstreamHeader() *header.Header {
	r := s.ParentSegment.IncomingHeader
	if r == nil {
		r = &header.Header{
			TraceID: s.ParentSegment.TraceID,
		}
	}
	if r.TraceID == "" {
		r.TraceID = s.ParentSegment.TraceID
	}
	if s.ParentSegment.Sampled {
		r.SamplingDecision = header.Sampled
	} else {
		r.SamplingDecision = header.NotSampled
	}
	r.ParentID = s.ID
	return r
}

// GetCause returns value of Cause.
func (s *Segment) GetCause() *CauseData {
	if s.Cause == nil {
		s.Cause = &CauseData{}
	}
	return s.Cause
}

// GetHTTP returns value of HTTP.
func (s *Segment) GetHTTP() *HTTPData {
	if s.HTTP == nil {
		s.HTTP = &HTTPData{}
	}
	return s.HTTP
}

// GetAWS returns value of AWS.
func (s *Segment) GetAWS() map[string]interface{} {
	if s.AWS == nil {
		s.AWS = make(map[string]interface{})
	}
	return s.AWS
}

// GetService returns value of Service.
func (s *Segment) GetService() *ServiceData {
	if s.Service == nil {
		s.Service = &ServiceData{}
	}
	return s.Service
}

// GetSQL returns value of SQL.
func (s *Segment) GetSQL() *SQLData {
	if s.SQL == nil {
		s.SQL = &SQLData{}
	}
	return s.SQL
}

// GetRequest returns value of RequestData.
func (d *HTTPData) GetRequest() *RequestData {
	if d.Request == nil {
		d.Request = &RequestData{}
	}
	return d.Request
}

// GetResponse returns value of ResponseData.
func (d *HTTPData) GetResponse() *ResponseData {
	if d.Response == nil {
		d.Response = &ResponseData{}
	}
	return d.Response
}

// GetConfiguration returns a value of Config.
func (s *Segment) GetConfiguration() *Config {
	if s.Configuration == nil {
		s.Configuration = &Config{}
	}
	return s.Configuration
}
