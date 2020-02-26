// Copyright (c) 2016-2019 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"gotest.tools/v3/assert"
)

func TestMain(m *testing.M) {
	var logOutput io.Writer

	if os.Getenv("AUTOSPOTTING_DEBUG") == "true" {
		logOutput = os.Stdout
	} else {
		logOutput = ioutil.Discard
	}

	logger = log.New(logOutput, "", 0)
	debug = log.New(logOutput, "", 0)

	os.Exit(m.Run())
}

func Test_processRegions(t *testing.T) {
	tests := []struct {
		name    string
		regions []string
		config  *Config
	}{
		{
			name:    "does nothing if no available regions",
			regions: []string{},
			config: &Config{
				Regions: "us-east-1",
			},
		},
		{
			name:    "does nothing if not enabled for any matching regions",
			regions: []string{"us-east-1"},
			config: &Config{
				Regions: "us-east-2",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processRegions(tt.regions, tt.config)
		})
	}
}

func Test_getRegions(t *testing.T) {

	tests := []struct {
		name    string
		ec2conn mockEC2
		want    []string
		wantErr error
	}{{
		name: "return some regions",
		ec2conn: mockEC2{
			dro: &ec2.DescribeRegionsOutput{
				Regions: []*ec2.Region{
					{RegionName: aws.String("foo")},
					{RegionName: aws.String("bar")},
				},
			},
			drerr: nil,
		},
		want:    []string{"foo", "bar"},
		wantErr: nil,
	},
		{
			name: "return an error",
			ec2conn: mockEC2{
				dro: &ec2.DescribeRegionsOutput{
					Regions: []*ec2.Region{
						{RegionName: aws.String("foo")},
						{RegionName: aws.String("bar")},
					},
				},
				drerr: fmt.Errorf("fooErr"),
			},
			want:    nil,
			wantErr: fmt.Errorf("fooErr"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			got, err := getRegions(tt.ec2conn)
			CheckErrors(t, err, tt.wantErr)

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getRegions() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_handler(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		rawEvent    json.RawMessage
		expectedErr string
	}{
		{
			name:        "returns error if event is not JSON",
			config:      &Config{},
			rawEvent:    json.RawMessage(`not JSON`),
			expectedErr: "invalid",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handler(tt.config)(context.TODO(), tt.rawEvent)
			if err != nil || tt.expectedErr != "" {
				assert.ErrorContains(t, err, tt.expectedErr)
			}
		})
	}
}
