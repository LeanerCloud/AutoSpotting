// Copyright (c) 2016-2019 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name        string
		environment map[string]string
	}{
		{
			name: "default settings",
		},
		{
			name: "with AWS_REGION set",
			environment: map[string]string{
				"AWS_REGION": "us-west-2",
			},
		},
		{
			name: "with LICENSE set",
			environment: map[string]string{
				"LICENSE": "I_built_it_from_source_code",
			},
		},
	}

	// save copy of environment before we run any tests
	envVars := make(map[string]string)
	for _, item := range os.Environ() {
		e := strings.SplitN(item, "=", 2)
		envVars[e[0]] = e[1]
	}

	for _, tt := range tests {
		if tt.environment != nil {
			for key, value := range tt.environment {
				os.Setenv(key, value)
			}
		}

		t.Run(tt.name, func(t *testing.T) {
			config := Config{}
			ParseConfig(&config)

			if tt.environment != nil {
				if tt.environment["AWS_REGION"] != "" {
					assert.Equal(t, config.MainRegion, tt.environment["AWS_REGION"])
				} else {
					assert.Equal(t, config.MainRegion, "us-east-1", "MainRegion should default to us-east-1")
				}

				if tt.environment["LICENSE"] != "" {
					assert.Equal(t, config.LicenseType, tt.environment["LICENSE"])
				}
			}

			assert.Equal(t, config.LogFile, os.Stdout)
			assert.Equal(t, config.SleepMultiplier, time.Duration(1))
			assert.Assert(t, config.InstanceData != nil, "expected InstanceData to be initialized")
		})

		// reset environment variables
		if tt.environment != nil {
			for name := range tt.environment {
				os.Setenv(name, envVars[name])
			}
		}
	}
}

func Test_addDefaultFilter(t *testing.T) {

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

func Test_addDefaultFilteringMode(t *testing.T) {
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
