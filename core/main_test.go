// Copyright (c) 2016-2019 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
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

func Test_spotEnabledIsAddedByDefault(t *testing.T) {

	tests := []struct {
		name   string
		config Config
		want   string
	}{
		{
			name:   "Default No ASG Tags",
			config: Config{},
			want:   "spot-enabled=true",
		},
		{
			name: "Specified ASG Tags",
			config: Config{
				FilterByTags: "environment=dev",
			},
			want: "environment=dev",
		},
		{
			name: "Specified ASG that is just whitespace",
			config: Config{
				FilterByTags: "         ",
			},
			want: "spot-enabled=true",
		},
		{
			name:   "Default No ASG Tags",
			config: Config{TagFilteringMode: "opt-out"},
			want:   "spot-enabled=false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			addDefaultFilter(&tt.config)

			if !reflect.DeepEqual(tt.config.FilterByTags, tt.want) {
				t.Errorf("addDefaultFilter() = %v, want %v", tt.config.FilterByTags, tt.want)
			}
		})
	}
}

func Test_addDefaultFilterMode(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{
			name: "Missing FilterMode",
			cfg:  Config{TagFilteringMode: ""},
			want: "opt-in",
		},
		{
			name: "Opt-in FilterMode",
			cfg: Config{
				TagFilteringMode: "opt-in",
			},
			want: "opt-in",
		},
		{
			name: "Opt-out FilterMode",
			cfg: Config{
				TagFilteringMode: "opt-out",
			},
			want: "opt-out",
		},
		{
			name: "Anything else gives the opt-in FilterMode",
			cfg: Config{
				TagFilteringMode: "whatever",
			},
			want: "opt-in",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addDefaultFilteringMode(&tt.cfg)
			if !reflect.DeepEqual(tt.cfg.TagFilteringMode, tt.want) {
				t.Errorf("addDefaultFilteringMode() = %v, want %v",
					tt.cfg.TagFilteringMode, tt.want)
			}
		})
	}
}
