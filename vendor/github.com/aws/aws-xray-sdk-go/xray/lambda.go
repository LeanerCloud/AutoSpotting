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
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-xray-sdk-go/header"
	log "github.com/cihub/seelog"
)

// LambdaTraceHeaderKey is key to get trace header from context.
const LambdaTraceHeaderKey string = "x-amzn-trace-id"

// LambdaTaskRootKey is the key to get Lambda Task Root from environment variable.
const LambdaTaskRootKey string = "LAMBDA_TASK_ROOT"

// SDKInitializedFileFolder records the location of SDK initialized file.
const SDKInitializedFileFolder string = "/tmp/.aws-xray"

// SDKInitializedFileName records the SDK initialized file name.
const SDKInitializedFileName string = "initialized"

func getTraceHeaderFromContext(ctx context.Context) *header.Header {
	var traceHeader string

	if traceHeaderValue := ctx.Value(LambdaTraceHeaderKey); traceHeaderValue != nil {
		traceHeader = traceHeaderValue.(string)
		return header.FromString(traceHeader)
	}
	return nil
}

func newFacadeSegment(ctx context.Context) (context.Context, *Segment) {
	traceHeader := getTraceHeaderFromContext(ctx)
	return BeginFacadeSegment(ctx, "facade", traceHeader)
}

func getLambdaTaskRoot() string {
	return os.Getenv(LambdaTaskRootKey)
}

func initLambda() {
	if getLambdaTaskRoot() != "" {
		now := time.Now()
		filePath, err := createFile(SDKInitializedFileFolder, SDKInitializedFileName)
		if err != nil {
			log.Tracef("unable to create file at %s. failed to signal SDK initialization with error: %v", filePath, err)
		} else {
			e := os.Chtimes(filePath, now, now)
			if e != nil {
				log.Tracef("unable to write to %s. failed to signal SDK initialization with error: %v", filePath, e)
			}
		}
	}
}

func createFile(dir string, name string) (string, error) {
	fileDir := filepath.FromSlash(dir)
	filePath := fileDir + string(os.PathSeparator) + name

	// detect if file exists
	var _, err = os.Stat(filePath)

	// create file if not exists
	if os.IsNotExist(err) {
		e := os.MkdirAll(dir, os.ModePerm)
		if e != nil {
			return filePath, e
		} else {
			var file, err = os.Create(filePath)
			if err != nil {
				return filePath, err
			}
			file.Close()
			return filePath, nil
		}
	} else if err != nil {
		return filePath, err
	}

	return filePath, nil
}
