// Copyright (c) 2016-2019 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
)

func TestNewSpotTermination(t *testing.T) {

	region := "foo"
	spotTermination := NewSpotTermination(region)

	if spotTermination.asSvc == nil || spotTermination.ec2Svc == nil {
		t.Errorf("Unable to connect to region %s", region)
	}
}

func TestGetInstanceIDDueForTermination(t *testing.T) {

	expectedInstanceID := "i-123456"
	dummyInstanceData := instanceData{
		InstanceID:     expectedInstanceID,
		InstanceAction: "terminate",
	}

	tests := []struct {
		name            string
		cloudWatchEvent events.CloudWatchEvent
		expected        *string
		expectedError   error
	}{
		{
			name: "Invalid Detail in CloudWatch event",
			cloudWatchEvent: events.CloudWatchEvent{
				Detail: []byte(""),
			},
		},
		{
			name: "Detail in event is empty",
			cloudWatchEvent: events.CloudWatchEvent{
				Detail: []byte("{}"),
			},
			expected:      nil,
			expectedError: nil,
		},
		{
			name: "Detail has spot termination event data",
			cloudWatchEvent: events.CloudWatchEvent{
				Detail: func() json.RawMessage {
					data, _ := json.Marshal(dummyInstanceData)
					return data
				}(),
			},
			expected:      &expectedInstanceID,
			expectedError: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			instanceID, _ := GetInstanceIDDueForTermination(tc.cloudWatchEvent)

			if tc.expected == nil && instanceID != nil {
				t.Errorf("Expected nil instanceID, actual: %s", *instanceID)
			}

			if tc.expected != nil && *tc.expected != *instanceID {
				t.Errorf("InstanceID expected: %v\nactual: %v", tc.expected, instanceID)
			}
		})
	}
}

func TestDetachInstance(t *testing.T) {

	asgName := "dummyASGName"
	instanceID := "dummyInstanceID"
	description := "Detaching EC2 instance: " + instanceID
	statusCode := "InProgress"

	tests := []struct {
		name            string
		spotTermination *SpotTermination
		expectedError   error
	}{
		{
			name: "When DetachInstances returns error",
			spotTermination: &SpotTermination{
				asSvc: mockASG{dierr: errors.New("")},
			},
			expectedError: errors.New(""),
		},
		{
			name: "When DetachInstances execute successfully",
			spotTermination: &SpotTermination{
				asSvc: mockASG{dio: &autoscaling.DetachInstancesOutput{
					Activities: []*autoscaling.Activity{
						{
							AutoScalingGroupName: &asgName,
							Description:          &description,
							StatusCode:           &statusCode,
						},
					},
				}},
				ec2Svc: mockEC2{dto: &ec2.DeleteTagsOutput{}},
			},
			expectedError: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.spotTermination.detachInstance(&instanceID, asgName)
			if err != nil && err.Error() != tc.expectedError.Error() {
				t.Errorf("Error in DetachInstance: expected %s actual %s", tc.expectedError.Error(), err.Error())
			}

		})
	}
}

func TestTerminateInstance(t *testing.T) {

	asgName := "dummyASGName"
	instanceID := "dummyInstanceID"
	statusCode := "InProgress"

	tests := []struct {
		name            string
		spotTermination *SpotTermination
		expectedError   error
	}{
		{
			name: "When TerminateInstance returns error",
			spotTermination: &SpotTermination{
				asSvc: mockASG{tiiasgerr: errors.New("")},
			},
			expectedError: errors.New(""),
		},
		{
			name: "When TerminateInstance execute successfully",
			spotTermination: &SpotTermination{
				asSvc: mockASG{tiiasgo: &autoscaling.TerminateInstanceInAutoScalingGroupOutput{
					Activity: &autoscaling.Activity{
						AutoScalingGroupName: &asgName,
						StatusCode:           &statusCode,
					},
				}},
			},
			expectedError: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.spotTermination.terminateInstance(&instanceID, asgName)
			if err != nil && err.Error() != tc.expectedError.Error() {
				t.Errorf("Error in TerminateInstance: expected %s actual %s", tc.expectedError.Error(), err.Error())
			}

		})
	}
}

func TestGetAsgName(t *testing.T) {
	asgName := "dummyASGName"
	instanceID := "dummyInstanceID"

	tests := []struct {
		name            string
		spotTermination *SpotTermination
		expectedError   error
		expectedName    string
	}{
		{
			name: "When DescribeAutoScalingInstances return error",
			spotTermination: &SpotTermination{
				asSvc: mockASG{dasierr: errors.New("")},
			},
			expectedError: errors.New(""),
			expectedName:  "",
		},
		{
			name: "When DescribeAutoScalingInstances returns no instances",
			spotTermination: &SpotTermination{
				asSvc: mockASG{dasio: &autoscaling.DescribeAutoScalingInstancesOutput{
					AutoScalingInstances: []*autoscaling.InstanceDetails{},
				}},
			},
			expectedError: nil,
			expectedName:  "",
		},
		{
			name: "When DescribeAutoScalingInstances returns asgName",
			spotTermination: &SpotTermination{
				asSvc: mockASG{dasio: &autoscaling.DescribeAutoScalingInstancesOutput{
					AutoScalingInstances: []*autoscaling.InstanceDetails{
						{
							AutoScalingGroupName: &asgName,
						},
					},
				}},
			},
			expectedError: nil,
			expectedName:  asgName,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			name, err := tc.spotTermination.getAsgName(&instanceID)

			if err != nil && err.Error() != tc.expectedError.Error() {
				t.Errorf("Error in getAsgName: expected error %s actual %s", tc.expectedError.Error(), err.Error())
			} else if name != tc.expectedName {
				t.Errorf("Error in getAsgName: expected name %s actual %s", tc.expectedName, name)
			}

		})
	}

}

func TestDeleteTagInstanceLaunchedForAsg(t *testing.T) {
	instanceID := "dummyInstanceID"

	tests := []struct {
		name            string
		spotTermination *SpotTermination
		expectedError   error
	}{
		{
			name: "When DeleteTags return error",
			spotTermination: &SpotTermination{
				ec2Svc: mockEC2{dterr: errors.New("")},
			},
			expectedError: errors.New(""),
		},
		{
			name: "When DeleteTags execute successfully",
			spotTermination: &SpotTermination{
				ec2Svc: mockEC2{dto: &ec2.DeleteTagsOutput{}},
			},
			expectedError: nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			err := tc.spotTermination.deleteTagInstanceLaunchedForAsg(&instanceID)

			if err != nil && err.Error() != tc.expectedError.Error() {
				t.Errorf("Error in deleteTagInstanceLaunchedForAsg: expected %s actual %s", tc.expectedError.Error(), err.Error())

			}
		})
	}
}

func TestExecuteAction(t *testing.T) {

	instanceID := "dummyInstanceID"
	asgName := "dummyASGName"
	lfhName := "dummyLFHName"
	lfhTransition := "autoscaling:EC2_INSTANCE_TERMINATING"

	tests := []struct {
		name                          string
		spotTermination               *SpotTermination
		expectedError                 error
		terminationNotificationAction string
	}{
		{
			name:            "When AutoScaling service is nil",
			spotTermination: &SpotTermination{},
			expectedError:   errors.New("AutoScaling service not defined. Please use NewSpotTermination()"),
		},
		{
			name: "When AutoScaling service returns error",
			spotTermination: &SpotTermination{
				asSvc: mockASG{dasierr: errors.New("")},
			},
			expectedError: errors.New(""),
		},
		{
			name: "When AutoScaling service returns no instances",
			spotTermination: &SpotTermination{
				asSvc: mockASG{dasio: &autoscaling.DescribeAutoScalingInstancesOutput{
					AutoScalingInstances: []*autoscaling.InstanceDetails{},
				}},
			},
			expectedError: nil,
		},
		{
			name: "When AutoScaling service returns asgName and action is auto",
			spotTermination: &SpotTermination{
				asSvc: mockASG{
					dasio: &autoscaling.DescribeAutoScalingInstancesOutput{
						AutoScalingInstances: []*autoscaling.InstanceDetails{
							{
								AutoScalingGroupName: &asgName,
							},
						},
					},
					dlho: &autoscaling.DescribeLifecycleHooksOutput{
						LifecycleHooks: []*autoscaling.LifecycleHook{
							{
								AutoScalingGroupName: &asgName,
								LifecycleHookName:    &lfhName,
								LifecycleTransition:  &lfhTransition,
							},
						},
					},
				},
			},
			expectedError:                 nil,
			terminationNotificationAction: "auto",
		},
		{
			name: "When AutoScaling service returns asgName and action is terminate",
			spotTermination: &SpotTermination{
				asSvc: mockASG{
					dasio: &autoscaling.DescribeAutoScalingInstancesOutput{
						AutoScalingInstances: []*autoscaling.InstanceDetails{
							{
								AutoScalingGroupName: &asgName,
							},
						},
					},
				},
			},
			expectedError:                 nil,
			terminationNotificationAction: "terminate",
		},
		{
			name: "When AutoScaling service returns asgName and action is detach",
			spotTermination: &SpotTermination{
				asSvc: mockASG{
					dasio: &autoscaling.DescribeAutoScalingInstancesOutput{
						AutoScalingInstances: []*autoscaling.InstanceDetails{
							{
								AutoScalingGroupName: &asgName,
							},
						},
					},
				},
				ec2Svc: mockEC2{
					dto: &ec2.DeleteTagsOutput{},
				},
			},
			expectedError:                 nil,
			terminationNotificationAction: "detach",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			err := tc.spotTermination.ExecuteAction(&instanceID, tc.terminationNotificationAction)

			if err != nil && err.Error() != tc.expectedError.Error() {
				t.Errorf("Error in ExecuteAction: expected %s actual %s", tc.expectedError.Error(), err.Error())
			}

		})
	}
}

func TestIsInAutoSpottingASG(t *testing.T) {
	instanceID := "dummyInstanceID"

	tests := []struct {
		name             string
		spotTermination  *SpotTermination
		tagFilteringMode string
		filterByTags     string
		expected         bool
	}{
		{
			name: "When instance is not in an ASG",
			spotTermination: &SpotTermination{
				asSvc: mockASG{
					dasio: &autoscaling.DescribeAutoScalingInstancesOutput{
						AutoScalingInstances: []*autoscaling.InstanceDetails{},
					},
				},
			},
			tagFilteringMode: "opt-in",
			filterByTags:     "spot-enabled=true",
			expected:         false,
		},
		{
			name: "When instance is in ASG with matching tags",
			spotTermination: &SpotTermination{
				asSvc: mockASG{
					dasgo: &autoscaling.DescribeAutoScalingGroupsOutput{
						AutoScalingGroups: []*autoscaling.Group{
							{
								AutoScalingGroupName: aws.String("asg1"),
								Tags: []*autoscaling.TagDescription{
									{
										Key:   aws.String("spot-enabled"),
										Value: aws.String("true"),
									},
								},
							},
						},
					},
					dasio: &autoscaling.DescribeAutoScalingInstancesOutput{
						AutoScalingInstances: []*autoscaling.InstanceDetails{
							{
								AutoScalingGroupName: aws.String("asg1"),
							},
						},
					},
				},
			},
			tagFilteringMode: "opt-in",
			filterByTags:     "spot-enabled=true",
			expected:         true,
		},
		{
			name: "When instance is in ASG without matching tag value",
			spotTermination: &SpotTermination{
				asSvc: mockASG{
					dasgo: &autoscaling.DescribeAutoScalingGroupsOutput{
						AutoScalingGroups: []*autoscaling.Group{
							{
								AutoScalingGroupName: aws.String("asg1"),
								Tags: []*autoscaling.TagDescription{
									{
										Key:   aws.String("spot-enabled"),
										Value: aws.String("false"),
									},
								},
							},
						},
					},
					dasio: &autoscaling.DescribeAutoScalingInstancesOutput{
						AutoScalingInstances: []*autoscaling.InstanceDetails{
							{
								AutoScalingGroupName: aws.String("asg1"),
							},
						},
					},
				},
			},
			tagFilteringMode: "opt-in",
			filterByTags:     "spot-enabled=true",
			expected:         false,
		},
		{
			name: "When instance is in ASG with no tags",
			spotTermination: &SpotTermination{
				asSvc: mockASG{
					dasgo: &autoscaling.DescribeAutoScalingGroupsOutput{
						AutoScalingGroups: []*autoscaling.Group{
							{
								AutoScalingGroupName: aws.String("asg1"),
								Tags:                 []*autoscaling.TagDescription{},
							},
						},
					},
					dasio: &autoscaling.DescribeAutoScalingInstancesOutput{
						AutoScalingInstances: []*autoscaling.InstanceDetails{
							{
								AutoScalingGroupName: aws.String("asg1"),
							},
						},
					},
				},
			},
			tagFilteringMode: "opt-in",
			filterByTags:     "spot-enabled=true",
			expected:         false,
		},
		{
			name: "When instance is in ASG that has opted out",
			spotTermination: &SpotTermination{
				asSvc: mockASG{
					dasgo: &autoscaling.DescribeAutoScalingGroupsOutput{
						AutoScalingGroups: []*autoscaling.Group{
							{
								AutoScalingGroupName: aws.String("asg1"),
								Tags: []*autoscaling.TagDescription{
									{
										Key:   aws.String("spot-enabled"),
										Value: aws.String("false"),
									},
								},
							},
						},
					},
					dasio: &autoscaling.DescribeAutoScalingInstancesOutput{
						AutoScalingInstances: []*autoscaling.InstanceDetails{
							{
								AutoScalingGroupName: aws.String("asg1"),
							},
						},
					},
				},
			},
			tagFilteringMode: "opt-out",
			filterByTags:     "spot-enabled=false",
			expected:         false,
		},
		{
			name: "When instance is in ASG that has not opted out",
			spotTermination: &SpotTermination{
				asSvc: mockASG{
					dasgo: &autoscaling.DescribeAutoScalingGroupsOutput{
						AutoScalingGroups: []*autoscaling.Group{
							{
								AutoScalingGroupName: aws.String("asg1"),
							},
						},
					},
					dasio: &autoscaling.DescribeAutoScalingInstancesOutput{
						AutoScalingInstances: []*autoscaling.InstanceDetails{
							{
								AutoScalingGroupName: aws.String("asg1"),
							},
						},
					},
				},
			},
			tagFilteringMode: "opt-out",
			filterByTags:     "spot-enabled=false",
			expected:         true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			actual := tc.spotTermination.IsInAutoSpottingASG(&instanceID, tc.tagFilteringMode, tc.filterByTags)

			if tc.expected != actual {
				t.Errorf("isInAutoSpottingASG received for %s: %v expected %v", tc.name, actual, tc.expected)
			}
		})
	}
}
