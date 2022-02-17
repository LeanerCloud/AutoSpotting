// Copyright (c) 2016-2022 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

func Test_countLaunchTemplateEphemeralVolumes(t *testing.T) {
	tests := []struct {
		name  string
		lt    *launchTemplate
		count int
	}{
		{
			name:  "empty LaunchTemplate",
			lt:    &launchTemplate{},
			count: 0,
		},
		{
			name: "empty BlockDeviceMappings",
			lt: &launchTemplate{
				LaunchTemplateVersion: &ec2.LaunchTemplateVersion{},
				Image: &ec2.Image{
					BlockDeviceMappings: []*ec2.BlockDeviceMapping{
						{},
					},
				},
			},
			count: 0,
		},
		{
			name: "mix of valid and invalid configuration",
			lt: &launchTemplate{
				LaunchTemplateVersion: &ec2.LaunchTemplateVersion{},
				Image: &ec2.Image{
					BlockDeviceMappings: []*ec2.BlockDeviceMapping{
						{VirtualName: aws.String("ephemeral")},
						{},
					},
				},
			},
			count: 1,
		},
		{
			name: "valid configuration",
			lt: &launchTemplate{
				LaunchTemplateVersion: &ec2.LaunchTemplateVersion{},
				Image: &ec2.Image{
					BlockDeviceMappings: []*ec2.BlockDeviceMapping{
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
			count := tc.lt.countLaunchTemplateEphemeralVolumes()
			if count != tc.count {
				t.Errorf("count expected: %d, actual: %d", tc.count, count)
			}
		})
	}
}
