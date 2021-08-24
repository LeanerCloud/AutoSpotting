// Copyright (c) 2016-2021 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0
package autospotting

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

func TestTerminate(t *testing.T) {
	tests := []struct {
		name     string
		tags     []*ec2.Tag
		inst     *instance
		expected error
	}{
		{
			name: "no issue with terminate",
			tags: []*ec2.Tag{},
			inst: &instance{
				Instance: &ec2.Instance{
					InstanceId: aws.String("id1"),
					State: &ec2.InstanceState{
						Name: aws.String(ec2.InstanceStateNameRunning),
					},
				},
				region: &region{
					services: connections{
						ec2: mockEC2{
							tierr: nil,
						},
					},
				},
			},
			expected: nil,
		},
		{
			name: "issue with terminate",
			tags: []*ec2.Tag{},
			inst: &instance{
				Instance: &ec2.Instance{
					InstanceId: aws.String("id1"),
					State: &ec2.InstanceState{
						Name: aws.String(ec2.InstanceStateNameRunning),
					},
				},
				region: &region{
					services: connections{
						ec2: mockEC2{
							tierr: errors.New(""),
						},
					},
				},
			},
			expected: errors.New(""),
		},
	}
	for _, tt := range tests {
		ret := tt.inst.terminate()
		if ret != nil && ret.Error() != tt.expected.Error() {
			t.Errorf("error actual: %s, expected: %s", ret.Error(), tt.expected.Error())
		}
	}
}
