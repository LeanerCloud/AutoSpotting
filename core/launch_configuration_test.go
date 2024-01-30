// Copyright (c) 2016-2022 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"testing"

	autoscalingTypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go/aws"
)

func Test_countLaunchConfigEphemeralVolumes(t *testing.T) {
	tests := []struct {
		name  string
		lc    *launchConfiguration
		count int
	}{
		{
			name: "empty launchConfiguration",
			lc: &launchConfiguration{
				LaunchConfiguration: &autoscalingTypes.LaunchConfiguration{
					BlockDeviceMappings: nil,
				},
			},
			count: 0,
		},
		{
			name: "empty BlockDeviceMappings",
			lc: &launchConfiguration{
				LaunchConfiguration: &autoscalingTypes.LaunchConfiguration{
					BlockDeviceMappings: []autoscalingTypes.BlockDeviceMapping{
						{},
					},
				},
			},
			count: 0,
		},
		{
			name: "mix of valid and invalid configuration",
			lc: &launchConfiguration{
				LaunchConfiguration: &autoscalingTypes.LaunchConfiguration{
					BlockDeviceMappings: []autoscalingTypes.BlockDeviceMapping{
						{VirtualName: aws.String("ephemeral")},
						{},
					},
				},
			},
			count: 1,
		},
		{
			name: "valid configuration",
			lc: &launchConfiguration{
				LaunchConfiguration: &autoscalingTypes.LaunchConfiguration{
					BlockDeviceMappings: []autoscalingTypes.BlockDeviceMapping{
						{VirtualName: aws.String("ephemeral")},
						{VirtualName: aws.String("ephemeral")},
					},
				},
			},
			count: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			count := tc.lc.countLaunchConfigEphemeralVolumes()
			if count != tc.count {
				t.Errorf("count expected: %d, actual: %d", tc.count, count)
			}
		})
	}
}
