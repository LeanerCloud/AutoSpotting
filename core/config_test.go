// Copyright (c) 2016-2019 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"os"
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
