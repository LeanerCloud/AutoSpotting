// Copyright 2017-2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not use this file except in compliance with the License. A copy of the License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package exception

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"runtime"
	"strings"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/pkg/errors"
)

// StackTracer is an interface for implementing StackTrace method.
type StackTracer interface {
	StackTrace() []uintptr
}

// Exception provides the shape for unmarshalling an exception.
type Exception struct {
	ID      string  `json:"id,omitempty"`
	Type    string  `json:"type,omitempty"`
	Message string  `json:"message,omitempty"`
	Stack   []Stack `json:"stack,omitempty"`
	Remote  bool    `json:"remote,omitempty"`
}

// Stack provides the shape for unmarshalling an stack.
type Stack struct {
	Path  string `json:"path,omitempty"`
	Line  int    `json:"line,omitempty"`
	Label string `json:"label,omitempty"`
}

// MultiError is a type for a slice of error.
type MultiError []error

// Error returns a string format of concatenating multiple errors.
func (e MultiError) Error() string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%d errors occurred:\n", len(e))
	for _, err := range e {
		buf.WriteString("* ")
		buf.WriteString(err.Error())
		buf.WriteByte('\n')
	}
	return buf.String()
}

var defaultErrorFrameCount = 32

// DefaultFormattingStrategy is the default implementation of
// the ExceptionFormattingStrategy and has a configurable frame count.
type DefaultFormattingStrategy struct {
	FrameCount int
}

// NewDefaultFormattingStrategy initializes DefaultFormattingStrategy
// with default value of frame count.
func NewDefaultFormattingStrategy() (*DefaultFormattingStrategy, error) {
	return &DefaultFormattingStrategy{FrameCount: defaultErrorFrameCount}, nil
}

// NewDefaultFormattingStrategyWithDefinedErrorFrameCount initializes
// DefaultFormattingStrategy with customer defined frame count.
func NewDefaultFormattingStrategyWithDefinedErrorFrameCount(frameCount int) (*DefaultFormattingStrategy, error) {
	if frameCount > 32 || frameCount < 0 {
		return nil, errors.New("frameCount must be a non-negative integer and less than 32")
	}
	return &DefaultFormattingStrategy{FrameCount: frameCount}, nil
}

// Error returns the value of XRayError by given error message.
func (dEFS *DefaultFormattingStrategy) Error(message string) *XRayError {
	s := make([]uintptr, dEFS.FrameCount)
	n := runtime.Callers(2, s)
	s = s[:n]

	return &XRayError{
		Type:    "error",
		Message: message,
		Stack:   s,
	}
}

// Errorf formats according to a format specifier and returns value of XRayError.
func (dEFS *DefaultFormattingStrategy) Errorf(formatString string, args ...interface{}) *XRayError {
	e := dEFS.Error(fmt.Sprintf(formatString, args...))
	e.Stack = e.Stack[1:]
	return e
}

// Panic records error type as panic in segment and returns value of XRayError.
func (dEFS *DefaultFormattingStrategy) Panic(message string) *XRayError {
	e := dEFS.Error(message)
	e.Type = "panic"
	e.Stack = e.Stack[4:]
	return e
}

// Panicf formats according to a format specifier and returns value of XRayError.
func (dEFS *DefaultFormattingStrategy) Panicf(formatString string, args ...interface{}) *XRayError {
	e := dEFS.Panic(fmt.Sprintf(formatString, args...))
	e.Stack = e.Stack[1:]
	return e
}

// ExceptionFromError takes an error and returns value of Exception
func (dEFS *DefaultFormattingStrategy) ExceptionFromError(err error) Exception {
	var isRemote bool
	if reqErr, ok := err.(awserr.RequestFailure); ok {
		// A service error occurs
		if reqErr.RequestID() != "" {
			isRemote = true
		}
	}
	e := Exception{
		ID:      newExceptionID(),
		Type:    "error",
		Message: err.Error(),
		Remote:  isRemote,
	}

	if err, ok := err.(*XRayError); ok {
		e.Type = err.Type
	}

	var s []uintptr

	// This is our publicly supported interface for passing along stack traces
	if err, ok := err.(StackTracer); ok {
		s = err.StackTrace()
	}

	// We also accept github.com/pkg/errors style stack traces for ease of use
	if err, ok := err.(interface {
		StackTrace() errors.StackTrace
	}); ok {
		for _, frame := range err.StackTrace() {
			s = append(s, uintptr(frame))
		}
	}

	if s == nil {
		s = make([]uintptr, dEFS.FrameCount)
		n := runtime.Callers(5, s)
		s = s[:n]
	}

	e.Stack = convertStack(s)
	return e
}

func newExceptionID() string {
	var r [8]byte
	_, err := rand.Read(r[:])
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%02x", r)
}

func convertStack(s []uintptr) []Stack {
	var r []Stack
	frames := runtime.CallersFrames(s)

	d := true
	for frame, more := frames.Next(); d; frame, more = frames.Next() {
		f := &Stack{}
		f.Path, f.Line, f.Label = parseFrame(frame)
		r = append(r, *f)
		d = more
	}
	return r
}

func parseFrame(frame runtime.Frame) (string, int, string) {
	path, line, label := frame.File, frame.Line, frame.Function

	// Strip GOPATH from path by counting the number of seperators in label & path
	// For example:
	//   GOPATH = /home/user
	//   path   = /home/user/src/pkg/sub/file.go
	//   label  = pkg/sub.Type.Method
	// We want to set path to:
	//    pkg/sub/file.go
	i := len(path)
	for n, g := 0, strings.Count(label, "/")+2; n < g; n++ {
		i = strings.LastIndex(path[:i], "/")
		if i == -1 {
			// Something went wrong and path has less seperators than we expected
			// Abort and leave i as -1 to counteract the +1 below
			break
		}
	}
	path = path[i+1:] // Trim the initial /

	// Strip the path from the function name as it's already in the path
	label = label[strings.LastIndex(label, "/")+1:]
	// Likewise strip the package name
	label = label[strings.Index(label, ".")+1:]

	return path, line, label
}
