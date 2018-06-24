// Copyright 2017-2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not use this file except in compliance with the License. A copy of the License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package xray

import (
	"bytes"
	"encoding/json"
	"net"
	"sync"

	log "github.com/cihub/seelog"
)

// Header is added before sending segments to daemon.
var Header = []byte(`{"format": "json", "version": 1}` + "\n")

type emitter struct {
	sync.Mutex
	conn *net.UDPConn
}

var e = &emitter{}

func init() {
	initLambda()
	refreshEmitter()
}

func refreshEmitter() {
	e.Lock()
	e.conn, _ = net.DialUDP("udp", nil, globalCfg.DaemonAddr())
	e.Unlock()
}

func refreshEmitterWithAddress(raddr *net.UDPAddr) {
	e.Lock()
	e.conn, _ = net.DialUDP("udp", nil, raddr)
	e.Unlock()
}

// Emit segment or subsegment if root segment is sampled.
func Emit(seg *Segment) {
	if seg == nil || !seg.ParentSegment.Sampled {
		return
	}

	// TODO: going to change our logging framework in next cr.
	var logLevel string
	if seg.Configuration != nil && seg.Configuration.LogLevel == "trace" {
		logLevel = "trace"
	} else if globalCfg.logLevel <= log.TraceLvl {
		logLevel = "trace"
	}

	for _, p := range packSegments(seg, nil) {
		if logLevel == "trace" {
			b := &bytes.Buffer{}
			json.Indent(b, p, "", " ")
			log.Trace(b.String())
		}
		e.Lock()
		_, err := e.conn.Write(append(Header, p...))
		if err != nil {
			log.Error(err)
		}
		e.Unlock()
	}
}

func packSegments(seg *Segment, outSegments [][]byte) [][]byte {
	trimSubsegment := func(s *Segment) []byte {
		ss := globalCfg.StreamingStrategy()
		if seg.ParentSegment.Configuration != nil && seg.ParentSegment.Configuration.StreamingStrategy != nil {
			ss = seg.ParentSegment.Configuration.StreamingStrategy
		}
		for ss.RequiresStreaming(s) {
			if len(s.rawSubsegments) == 0 {
				break
			}
			cb := ss.StreamCompletedSubsegments(s)
			outSegments = append(outSegments, cb...)
		}
		b, _ := json.Marshal(s)
		return b
	}

	for _, s := range seg.rawSubsegments {
		outSegments = packSegments(s, outSegments)
		if b := trimSubsegment(s); b != nil {
			seg.Subsegments = append(seg.Subsegments, b)
		}
	}
	if seg.parent == nil {
		if b := trimSubsegment(seg); b != nil {
			outSegments = append(outSegments, b)
		}
	}
	return outSegments
}
