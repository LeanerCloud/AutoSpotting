// Copyright (c) 2016-2022 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	autoscalingTypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func TestAlreadyRunningInstanceCount(t *testing.T) {
	tests := []struct {
		name             string
		asgName          string
		asgInstances     instances
		spot             bool
		availabilityZone *string
		expectedCount    int64
		expectedTotal    int64
	}{
		{name: "ASG has no instance at all",
			asgName:          "test-asg",
			asgInstances:     makeInstances(),
			spot:             true,
			availabilityZone: nil,
			expectedCount:    0,
			expectedTotal:    0,
		},
		{name: "ASG has no 'running' instance but has some",
			asgName: "test-asg",
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
					"id-1": {
						Instance: &ec2Types.Instance{
							State:             &ec2Types.InstanceState{Name: ec2Types.InstanceStateNameStopped},
							Placement:         &ec2Types.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: Spot,
						},
					},
				},
			),
			spot:             true,
			availabilityZone: nil,
			expectedCount:    0,
			expectedTotal:    0,
		},
		{name: "ASG has no 'running' spot instances but has some",
			asgName: "test-asg",
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
					"id-1": {
						Instance: &ec2Types.Instance{
							State:             &ec2Types.InstanceState{Name: ec2Types.InstanceStateNameRunning},
							Placement:         &ec2Types.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: OnDemand,
						},
					},
					"id-2": {
						Instance: &ec2Types.Instance{
							State:             &ec2Types.InstanceState{Name: ec2Types.InstanceStateNameRunning},
							Placement:         &ec2Types.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: OnDemand,
						},
					},
				},
			),
			spot:             true,
			availabilityZone: nil,
			expectedCount:    0,
			expectedTotal:    2,
		},
		{name: "ASG has no 'running' on-demand instances but has some",
			asgName: "test-asg",
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
					"id-1": {
						Instance: &ec2Types.Instance{
							State:             &ec2Types.InstanceState{Name: ec2Types.InstanceStateNameRunning},
							Placement:         &ec2Types.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: Spot,
						},
					},
					"id-2": {
						Instance: &ec2Types.Instance{
							State:             &ec2Types.InstanceState{Name: ec2Types.InstanceStateNameRunning},
							Placement:         &ec2Types.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: Spot,
						},
					},
				},
			),
			spot:             false,
			availabilityZone: nil,
			expectedCount:    0,
			expectedTotal:    2,
		},
		{name: "ASG has no 'running' on-demand instances in the AZ",
			asgName: "test-asg",
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
					"id-1": {
						Instance: &ec2Types.Instance{
							State:             &ec2Types.InstanceState{Name: ec2Types.InstanceStateNameRunning},
							Placement:         &ec2Types.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: OnDemand,
						},
					},
					"id-2": {
						Instance: &ec2Types.Instance{
							State:             &ec2Types.InstanceState{Name: ec2Types.InstanceStateNameRunning},
							Placement:         &ec2Types.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: OnDemand,
						},
					},
				},
			),
			spot:             false,
			availabilityZone: aws.String("eu-west-1c"),
			expectedCount:    0,
			expectedTotal:    2,
		},
		{name: "ASG has some 'running' on-demand instances in the AZ",
			asgName: "test-asg",
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
					"id-1": {
						Instance: &ec2Types.Instance{
							State:             &ec2Types.InstanceState{Name: ec2Types.InstanceStateNameRunning},
							Placement:         &ec2Types.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: OnDemand,
						},
					},
					"id-2": {
						Instance: &ec2Types.Instance{
							State:             &ec2Types.InstanceState{Name: ec2Types.InstanceStateNameRunning},
							Placement:         &ec2Types.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: OnDemand,
						},
					},
				},
			),
			spot:             false,
			availabilityZone: aws.String("eu-west-1b"),
			expectedCount:    1,
			expectedTotal:    2,
		},
		{name: "ASG has no 'running' spot instances in the AZ",
			asgName: "test-asg",
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
					"id-1": {
						Instance: &ec2Types.Instance{
							State:             &ec2Types.InstanceState{Name: ec2Types.InstanceStateNameRunning},
							Placement:         &ec2Types.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: Spot,
						},
					},
					"id-2": {
						Instance: &ec2Types.Instance{
							State:             &ec2Types.InstanceState{Name: ec2Types.InstanceStateNameRunning},
							Placement:         &ec2Types.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: Spot,
						},
					},
				},
			),
			spot:             true,
			availabilityZone: aws.String("eu-west-1c"),
			expectedCount:    0,
			expectedTotal:    2,
		},
		{name: "ASG has some 'running' spot instances in any AZ",
			asgName: "test-asg",
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
					"id-1": {
						Instance: &ec2Types.Instance{
							State:             &ec2Types.InstanceState{Name: ec2Types.InstanceStateNameRunning},
							Placement:         &ec2Types.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: Spot,
						},
					},
					"id-2": {
						Instance: &ec2Types.Instance{
							State:             &ec2Types.InstanceState{Name: ec2Types.InstanceStateNameRunning},
							Placement:         &ec2Types.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: Spot,
						},
					},
				},
			),
			spot:             true,
			availabilityZone: nil,
			expectedCount:    2,
			expectedTotal:    2,
		},
		{name: "ASG has no 'running' spot instances in any AZ",
			asgName: "test-asg",
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
					"id-1": {
						Instance: &ec2Types.Instance{
							State:             &ec2Types.InstanceState{Name: ec2Types.InstanceStateNameShuttingDown},
							Placement:         &ec2Types.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: Spot,
						},
					},
					"id-2": {
						Instance: &ec2Types.Instance{
							State:             &ec2Types.InstanceState{Name: ec2Types.InstanceStateNameRunning},
							Placement:         &ec2Types.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: OnDemand,
						},
					},
				},
			),
			spot:             true,
			availabilityZone: nil,
			expectedCount:    0,
			expectedTotal:    1,
		},
		{name: "ASG has some 'running' on-demand instances in any AZ",
			asgName: "test-asg",
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
					"id-1": {
						Instance: &ec2Types.Instance{
							State:             &ec2Types.InstanceState{Name: ec2Types.InstanceStateNameRunning},
							Placement:         &ec2Types.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: Spot,
						},
					},
					"id-2": {
						Instance: &ec2Types.Instance{
							State:             &ec2Types.InstanceState{Name: ec2Types.InstanceStateNameRunning},
							Placement:         &ec2Types.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: OnDemand,
						},
					},
				},
			),
			spot:             false,
			availabilityZone: nil,
			expectedCount:    1,
			expectedTotal:    2,
		},
		{name: "ASG has no 'running' on-demand instances in any AZ",
			asgName: "test-asg",
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
					"id-1": {
						Instance: &ec2Types.Instance{
							State:             &ec2Types.InstanceState{Name: ec2Types.InstanceStateNameRunning},
							Placement:         &ec2Types.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: Spot,
						},
					},
					"id-2": {
						Instance: &ec2Types.Instance{
							State:             &ec2Types.InstanceState{Name: ec2Types.InstanceStateNameStopped},
							Placement:         &ec2Types.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: OnDemand,
						},
					},
				},
			),
			spot:             false,
			availabilityZone: nil,
			expectedCount:    0,
			expectedTotal:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := autoScalingGroup{AutoScalingGroup: &autoscalingTypes.AutoScalingGroup{}}
			a.name = tt.asgName
			a.instances = tt.asgInstances
			count, total := a.alreadyRunningInstanceCount(tt.spot, tt.availabilityZone)
			if tt.expectedCount != count {
				t.Errorf("alreadyRunningInstanceCount returned count: %d expected %d",
					count, tt.expectedCount)
			} else if tt.expectedTotal != total {
				t.Errorf("alreadyRunningInstanceCount returned total: %d expected %d",
					total, tt.expectedTotal)
			}
		})
	}
}

// func TestNeedReplaceOnDemandInstances(t *testing.T) {
// 	tests := []struct {
// 		name            string
// 		asgInstances    instances
// 		minOnDemand     int64
// 		desiredCapacity *int64
// 		expectedRun     bool
// 		regionASG       *region
// 	}{
// 		{name: "ASG has no instance at all - 1 on-demand required",
// 			asgInstances:    makeInstances(),
// 			minOnDemand:     1,
// 			desiredCapacity: aws.Int32(0),
// 			expectedRun:     true,
// 		},
// 		{name: "ASG has no instance at all - 0 on-demand required",
// 			asgInstances:    makeInstances(),
// 			minOnDemand:     0,
// 			desiredCapacity: aws.Int32(0),
// 			expectedRun:     true,
// 		},
// 		{name: "ASG has no instance running - 1 on-demand required",
// 			asgInstances: makeInstancesWithCatalog(
// 				instanceMap{
// 					"id-1": {
// 						Instance: &ec2Types.Instance{
// 							State:             &ec2Types.InstanceState{Name: InstanceStateNameShuttingDown},
// 							Placement:         &ec2Types.Placement{AvailabilityZone: aws.String("eu-west-1a")},
// 							InstanceLifecycle: Spot,
// 						},
// 					},
// 					"id-2": {
// 						Instance: &ec2Types.Instance{
// 							State:             &ec2Types.InstanceState{Name: InstanceStateNameShuttingDown},
// 							Placement:         &ec2Types.Placement{AvailabilityZone: aws.String("eu-west-1a")},
// 							InstanceLifecycle: OnDemand,
// 						},
// 					},
// 				},
// 			),
// 			minOnDemand:     1,
// 			desiredCapacity: aws.Int32(0),
// 			expectedRun:     true,
// 		},
// 		{name: "ASG has no instance running - 0 on-demand required",
// 			asgInstances: makeInstancesWithCatalog(
// 				instanceMap{
// 					"id-1": {
// 						Instance: &ec2Types.Instance{
// 							State:             &ec2Types.InstanceState{Name: InstanceStateNameShuttingDown},
// 							Placement:         &ec2Types.Placement{AvailabilityZone: aws.String("eu-west-1a")},
// 							InstanceLifecycle: Spot,
// 						},
// 					},
// 					"id-2": {
// 						Instance: &ec2Types.Instance{
// 							State:             &ec2Types.InstanceState{Name: InstanceStateNameShuttingDown},
// 							Placement:         &ec2Types.Placement{AvailabilityZone: aws.String("eu-west-1a")},
// 							InstanceLifecycle: OnDemand,
// 						},
// 					},
// 				},
// 			),
// 			minOnDemand:     0,
// 			desiredCapacity: aws.Int32(0),
// 			expectedRun:     true,
// 		},
// 		{name: "ASG has not the required on-demand running",
// 			asgInstances: makeInstancesWithCatalog(
// 				instanceMap{
// 					"id-1": {
// 						Instance: &ec2Types.Instance{
// 							InstanceId:        aws.String("id-1"),
// 							State:             &ec2Types.InstanceState{Name: ec2Types.InstanceStateNameRunning},
// 							Placement:         &ec2Types.Placement{AvailabilityZone: aws.String("eu-west-1a")},
// 							InstanceLifecycle: Spot,
// 						},
// 						region: &region{
// 							name: "test-region",
// 							services: &connections{
// 								ec2: &mockEC2{},
// 							},
// 						},
// 					},
// 					"id-2": {
// 						Instance: &ec2Types.Instance{
// 							InstanceId:        aws.String("id-2"),
// 							State:             &ec2Types.InstanceState{Name: ec2Types.InstanceStateNameRunning},
// 							Placement:         &ec2Types.Placement{AvailabilityZone: aws.String("eu-west-1a")},
// 							InstanceLifecycle: OnDemand,
// 						},
// 					},
// 				},
// 			),
// 			minOnDemand:     2,
// 			desiredCapacity: aws.Int32(0),
// 			expectedRun:     false,
// 			regionASG: &region{
// 				name: "regionTest",
// 				services: &connections{
// 					autoScaling: mockASG{},
// 				},
// 				conf: &Config{},
// 			},
// 		},
// 		{name: "ASG has just enough on-demand instances running",
// 			asgInstances: makeInstancesWithCatalog(
// 				instanceMap{
// 					"id-1": {
// 						Instance: &ec2Types.Instance{
// 							State:             &ec2Types.InstanceState{Name: ec2Types.InstanceStateNameRunning},
// 							Placement:         &ec2Types.Placement{AvailabilityZone: aws.String("eu-west-1a")},
// 							InstanceLifecycle: Spot,
// 						},
// 					},
// 					"id-2": {
// 						Instance: &ec2Types.Instance{
// 							State:             &ec2Types.InstanceState{Name: ec2Types.InstanceStateNameRunning},
// 							Placement:         &ec2Types.Placement{AvailabilityZone: aws.String("eu-west-1b")},
// 							InstanceLifecycle: OnDemand,
// 						},
// 					},
// 				},
// 			),
// 			minOnDemand:     1,
// 			desiredCapacity: aws.Int32(0),
// 			expectedRun:     false,
// 		},
// 		{name: "ASG has only one remaining instance, less than enough on-demand",
// 			asgInstances: makeInstancesWithCatalog(
// 				instanceMap{
// 					"id-1": {
// 						Instance: &ec2Types.Instance{
// 							State:             &ec2Types.InstanceState{Name: ec2Types.InstanceStateNameRunning},
// 							Placement:         &ec2Types.Placement{AvailabilityZone: aws.String("eu-west-1a")},
// 							InstanceLifecycle: Spot,
// 						},
// 					},
// 				},
// 			),
// 			minOnDemand:     1,
// 			desiredCapacity: aws.Int32(1),
// 			expectedRun:     false,
// 		},
// 		{name: "ASG has more than enough on-demand instances running but not desired capacity",
// 			asgInstances: makeInstancesWithCatalog(
// 				instanceMap{
// 					"id-1": {
// 						Instance: &ec2Types.Instance{
// 							State:             &ec2Types.InstanceState{Name: ec2Types.InstanceStateNameRunning},
// 							Placement:         &ec2Types.Placement{AvailabilityZone: aws.String("eu-west-1a")},
// 							InstanceLifecycle: OnDemand,
// 						},
// 					},
// 					"id-2": {
// 						Instance: &ec2Types.Instance{
// 							State:             &ec2Types.InstanceState{Name: ec2Types.InstanceStateNameRunning},
// 							Placement:         &ec2Types.Placement{AvailabilityZone: aws.String("eu-west-1b")},
// 							InstanceLifecycle: OnDemand,
// 						},
// 					},
// 				},
// 			),
// 			minOnDemand:     1,
// 			desiredCapacity: aws.Int32(1),
// 			expectedRun:     true,
// 		},
// 		{name: "ASG has more than enough on-demand instances running and desired capacity",
// 			asgInstances: makeInstancesWithCatalog(
// 				instanceMap{
// 					"id-1": {
// 						Instance: &ec2Types.Instance{
// 							State:             &ec2Types.InstanceState{Name: ec2Types.InstanceStateNameRunning},
// 							Placement:         &ec2Types.Placement{AvailabilityZone: aws.String("eu-west-1a")},
// 							InstanceLifecycle: OnDemand,
// 						},
// 					},
// 					"id-2": {
// 						Instance: &ec2Types.Instance{
// 							State:             &ec2Types.InstanceState{Name: ec2Types.InstanceStateNameRunning},
// 							Placement:         &ec2Types.Placement{AvailabilityZone: aws.String("eu-west-1b")},
// 							InstanceLifecycle: OnDemand,
// 						},
// 					},
// 				},
// 			),
// 			minOnDemand:     1,
// 			desiredCapacity: aws.Int32(4),
// 			expectedRun:     true,
// 		},
// 		{name: "ASG has on-demand instances equal to the min on-demand number",
// 			asgInstances: makeInstancesWithCatalog(
// 				map[string]*instance{
// 					"id-1": {
// 						Instance: &ec2Types.Instance{
// 							State:             &ec2Types.InstanceState{Name: ec2Types.InstanceStateNameRunning},
// 							Placement:         &ec2Types.Placement{AvailabilityZone: aws.String("eu-west-1a")},
// 							InstanceLifecycle: OnDemand,
// 						},
// 					},
// 					"id-2": {
// 						Instance: &ec2Types.Instance{
// 							State:             &ec2Types.InstanceState{Name: ec2Types.InstanceStateNameRunning},
// 							Placement:         &ec2Types.Placement{AvailabilityZone: aws.String("eu-west-1b")},
// 							InstanceLifecycle: Spot,
// 						},
// 					},
// 				},
// 			),
// 			minOnDemand:     1,
// 			desiredCapacity: aws.Int32(2),
// 			expectedRun:     false,
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			a := autoScalingGroup{AutoScalingGroup: &autoscalingTypes.AutoScalingGroup{}}
// 			a.name = "asg-test"
// 			a.DesiredCapacity = tt.desiredCapacity
// 			a.instances = tt.asgInstances
// 			a.minOnDemand = tt.minOnDemand
// 			a.region = tt.regionASG
// 			shouldRun := a.needReplaceOnDemandInstances(false)
// 			if tt.expectedRun != shouldRun {
// 				t.Errorf("needReplaceOnDemandInstances returned: %t expected %t",
// 					shouldRun, tt.expectedRun)
// 			}
// 		})
// 	}
//}

// func TestTerminateInstanceInAutoScalingGroup(t *testing.T) {
// 	tests := []struct {
// 		name         string
// 		instancesASG instances
// 		regionASG    *region
// 		instanceID   *string
// 		expected     error
// 	}{
// 		{name: "no err during terminate",
// 			instancesASG: makeInstancesWithCatalog(
// 				map[string]*instance{
// 					"1": {
// 						Instance: &ec2Types.Instance{
// 							InstanceId: aws.String("1"),
// 							State: &ec2Types.InstanceState{
// 								Name: ec2Types.InstanceStateNameRunning,
// 							},
// 						},
// 						region: &region{
// 							services: &connections{
// 								ec2: mockEC2{},
// 							},
// 						},
// 					},
// 				},
// 			),
// 			regionASG: &region{
// 				name: "regionTest",
// 				services: &connections{
// 					autoScaling: mockASG{tiiasgerr: nil},
// 				},
// 				conf: &Config{},
// 			},
// 			instanceID: aws.String("1"),
// 			expected:   nil,
// 		},
// 		{name: "errors during terminate",
// 			instancesASG: makeInstancesWithCatalog(
// 				map[string]*instance{
// 					"1": {
// 						Instance: &ec2Types.Instance{
// 							InstanceId: aws.String("1"),
// 							State: &ec2Types.InstanceState{
// 								Name: ec2Types.InstanceStateNameRunning,
// 							},
// 						},
// 						region: &region{
// 							services: &connections{
// 								ec2: mockEC2{},
// 							},
// 						},
// 					},
// 				},
// 			),
// 			regionASG: &region{
// 				name: "regionTest",
// 				services: &connections{
// 					autoScaling: mockASG{tiiasgerr: errors.New("terminate-asg")},
// 				},
// 				conf: &Config{},
// 			},
// 			instanceID: aws.String("1"),
// 			expected:   errors.New("terminate-asg"),
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			a := autoScalingGroup{
// 				name:      "testASG",
// 				region:    tt.regionASG,
// 				instances: tt.instancesASG,
// 			}
// 			//			err := a.terminateInstanceInAutoScalingGroup(tt.instanceID, false, false)
// 			CheckErrors(t, err, tt.expected)
// 		})
// 	}
// }

func TestAttachSpotInstance(t *testing.T) {
	tests := []struct {
		name       string
		regionASG  *region
		instanceID string
		expected   error
	}{
		{name: "no err during attach",
			regionASG: &region{
				name: "regionTest",
				services: &connections{
					autoScaling: mockASG{
						aierr: nil,
						dasio: &autoscaling.DescribeAutoScalingInstancesOutput{
							AutoScalingInstances: []autoscalingTypes.AutoScalingInstanceDetails{
								{
									LifecycleState: aws.String("InService"),
								},
							},
						},
					},
					ec2: mockEC2Client{
						dio: &ec2.DescribeInstancesOutput{
							Reservations: []ec2Types.Reservation{
								{
									Instances: []ec2Types.Instance{
										{
											State: &ec2Types.InstanceState{
												Name: "running",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			instanceID: "1",
			expected:   nil,
		},
		{name: "err during attach",
			regionASG: &region{
				name: "regionTest",
				services: &connections{
					autoScaling: mockASG{
						aierr: errors.New("ValidationError"),
						// aierr: errors.New("attach"),
					},
					ec2: mockEC2Client{
						dio: &ec2.DescribeInstancesOutput{
							Reservations: []ec2Types.Reservation{
								{
									Instances: []ec2Types.Instance{
										{
											State: &ec2Types.InstanceState{
												Name: "running",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			instanceID: "1",
			expected:   errors.New("ValidationError"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 	a := autoScalingGroup{
			// 		name:   "testASG",
			// 		region: tt.regionASG,
			// 		AutoScalingGroup: &autoscalingTypes.AutoScalingGroup{
			// 			MaxSize:         aws.Int64(4),
			// 			MinSize:         aws.Int64(2),
			// 			DesiredCapacity: aws.Int32(3),
			// 		},
			// 	}
			// 	err := a.attachSpotInstance(tt.instanceID, false)
			// 	CheckErrors(t, err, tt.expected)
		})
	}
}

func TestLoadLaunchConfiguration(t *testing.T) {
	tests := []struct {
		name        string
		nameLC      *string
		regionASG   *region
		groupLC     *launchConfiguration
		expectedLC  *launchConfiguration
		expectedErr error
	}{
		{name: "launch configuration already exists",
			groupLC: &launchConfiguration{
				LaunchConfiguration: &autoscalingTypes.LaunchConfiguration{
					LaunchConfigurationName: aws.String("foo"),
				},
			},
			// nameLC: aws.String("foo"),
			// regionASG: &region{
			// 	services: &connections{
			// 		autoScaling: mockASG{
			// 			dlco: &autoscaling.DescribeLaunchConfigurationsOutput{
			// 				LaunchConfigurations: []*autoscalingTypes.LaunchConfiguration{
			// 					{
			// 						LaunchConfigurationName: aws.String("foo"),
			// 					},
			// 				},
			// 			},
			// 		},
			// 	},
			// },
			expectedErr: nil,
			expectedLC: &launchConfiguration{
				LaunchConfiguration: &autoscalingTypes.LaunchConfiguration{
					LaunchConfigurationName: aws.String("foo"),
				},
			},
		},
		{name: "nil launch configuration name",
			nameLC: nil,
			regionASG: &region{
				services: &connections{
					autoScaling: mockASG{
						dlcerr: nil},
				},
			},
			expectedErr: errors.New("missing launch configuration"),
			expectedLC:  nil,
		},
		{name: "no err during get launch configuration",
			nameLC: aws.String("testLC"),
			regionASG: &region{
				services: &connections{
					autoScaling: mockASG{
						dlcerr: nil,
						dlco: &autoscaling.DescribeLaunchConfigurationsOutput{
							LaunchConfigurations: []autoscalingTypes.LaunchConfiguration{
								{
									LaunchConfigurationName: aws.String("testLC"),
								},
							},
						},
					},
				},
			},
			expectedErr: nil,
			expectedLC: &launchConfiguration{
				LaunchConfiguration: &autoscalingTypes.LaunchConfiguration{
					LaunchConfigurationName: aws.String("testLC"),
				},
			},
		},
		{name: "err during get launch configuration",
			nameLC: aws.String("testLC"),
			regionASG: &region{
				services: &connections{
					autoScaling: mockASG{
						dlcerr: errors.New("describe"),
						dlco:   nil,
					},
				},
			},
			expectedErr: errors.New("describe"),
			expectedLC:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := autoScalingGroup{
				launchConfiguration: tt.groupLC,
				region:              tt.regionASG,
				AutoScalingGroup: &autoscalingTypes.AutoScalingGroup{
					LaunchConfigurationName: tt.nameLC,
				},
			}
			lc, err := a.loadLaunchConfiguration()

			if !errorMatches(err, tt.expectedErr) {
				t.Errorf("loadLaunchConfiguration received error status: %+v expected %+v",
					err, tt.expectedErr)
			}

			if !reflect.DeepEqual(tt.expectedLC, lc) {
				t.Errorf("loadLaunchConfiguration received: %+v expected %+v",
					lc, tt.expectedLC)
			}

			if lc != a.launchConfiguration {
				t.Errorf("loadLaunchConfiguration returned %+v but set member field launchConfiguration to %+v",
					lc, a.launchConfiguration)
			}
		})
	}
}

func TestLoadLaunchTemplate(t *testing.T) {
	tests := []struct {
		name        string
		ltID        *string
		ltVer       *int64
		regionASG   *region
		groupLT     *launchTemplate
		expectedLT  *launchTemplate
		expectedErr error
	}{
		{name: "launch template already exists",
			groupLT: &launchTemplate{
				LaunchTemplateVersion: &ec2Types.LaunchTemplateVersion{
					LaunchTemplateId: aws.String("foo"),
					VersionNumber:    aws.Int64(1),
				},
				Image: &ec2Types.Image{
					ImageId: aws.String("bar"),
				},
			},
			expectedErr: nil,
			expectedLT: &launchTemplate{
				LaunchTemplateVersion: &ec2Types.LaunchTemplateVersion{
					LaunchTemplateId: aws.String("foo"),
					VersionNumber:    aws.Int64(1),
				},
				Image: &ec2Types.Image{
					ImageId: aws.String("bar"),
				},
			},
		},
		{name: "nil launch template name",
			ltID:  nil,
			ltVer: nil,
			regionASG: &region{
				services: &connections{
					ec2: mockEC2Client{
						dltverr: nil},
				},
			},
			expectedErr: errors.New("missing launch template"),
			expectedLT:  nil,
		},
		{name: "no err during get launch template",
			ltID:  aws.String("foo"),
			ltVer: aws.Int64(1),
			regionASG: &region{
				services: &connections{
					ec2: mockEC2Client{
						dltverr: nil,
						dltvo: &ec2.DescribeLaunchTemplateVersionsOutput{
							LaunchTemplateVersions: []ec2Types.LaunchTemplateVersion{
								{
									LaunchTemplateId: aws.String("foo"),
									VersionNumber:    aws.Int64(1),
									LaunchTemplateData: &ec2Types.ResponseLaunchTemplateData{
										ImageId: aws.String("bar"),
									},
								},
							},
						},
						damierr: nil,
						damio: &ec2.DescribeImagesOutput{
							Images: []ec2Types.Image{
								{
									ImageId: aws.String("bar"),
								},
							},
						},
					},
				},
			},
			expectedErr: nil,
			expectedLT: &launchTemplate{
				LaunchTemplateVersion: &ec2Types.LaunchTemplateVersion{
					LaunchTemplateId: aws.String("foo"),
					VersionNumber:    aws.Int64(1),
					LaunchTemplateData: &ec2Types.ResponseLaunchTemplateData{
						ImageId: aws.String("bar"),
					},
				},
				Image: &ec2Types.Image{
					ImageId: aws.String("bar"),
				},
			},
		},
		{name: "err during get launch template",
			ltID:  aws.String("foo"),
			ltVer: aws.Int64(1),
			regionASG: &region{
				services: &connections{
					ec2: mockEC2Client{
						dltverr: errors.New("describe"),
						dltvo:   nil,
					},
				},
			},
			expectedErr: errors.New("describe"),
			expectedLT:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := autoScalingGroup{
				launchTemplate: tt.groupLT,
				region:         tt.regionASG,
				AutoScalingGroup: &autoscalingTypes.AutoScalingGroup{
					LaunchTemplate: &autoscalingTypes.LaunchTemplateSpecification{
						LaunchTemplateId: tt.ltID,
						Version:          aws.String("1"),
					},
				},
			}
			lt, err := a.loadLaunchTemplate()
			if !errorMatches(err, tt.expectedErr) {
				t.Errorf("loadLaunchConfiguration received error status: %+v expected %+v",
					err, tt.expectedErr)
			}

			if !reflect.DeepEqual(tt.expectedLT, lt) {
				t.Errorf("loadLaunchConfiguration received: %+v expected %+v",
					lt, tt.expectedLT)
			}

			if lt != a.launchTemplate {
				t.Errorf("loadLaunchConfiguration returned %+v but set member field launchConfiguration to %+v",
					lt, a.launchConfiguration)
			}
		})
	}
}

func TestSetAutoScalingMaxSize(t *testing.T) {
	tests := []struct {
		name      string
		maxSize   int32
		regionASG *region
		expected  error
	}{
		{name: "err during set autoscaling max size",
			maxSize: 4,
			regionASG: &region{
				services: &connections{
					autoScaling: mockASG{
						uasgerr: errors.New("update"),
					},
				},
			},
			expected: errors.New("update"),
		},
		{name: "no err during set autoscaling max size",
			maxSize: 4,
			regionASG: &region{
				services: &connections{
					autoScaling: mockASG{
						uasgerr: nil,
					},
				},
			},
			expected: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := autoScalingGroup{
				name:   "testASG",
				region: tt.regionASG,
			}
			err := a.setAutoScalingMaxSize(tt.maxSize)
			CheckErrors(t, err, tt.expected)
		})
	}
}

func TestScanInstances(t *testing.T) {
	tests := []struct {
		name              string
		ec2ASG            *autoscalingTypes.AutoScalingGroup
		regionInstances   *region
		expectedInstances instanceMap
	}{
		{name: "multiple instances to scan",
			regionInstances: &region{
				services: &connections{
					ec2: &mockEC2Client{
						dio: &ec2.DescribeInstancesOutput{},
						damio: &ec2.DescribeImagesOutput{
							Images: []ec2Types.Image{
								{
									PlatformDetails: aws.String(LinuxSpotProduct),
									ImageId:         aws.String("ami-12345"),
								},
							},
						},
					},
				},
				instances: makeInstancesWithCatalog(
					instanceMap{
						"1": {
							Instance: &ec2Types.Instance{
								InstanceId: aws.String("1"),
								Placement: &ec2Types.Placement{
									AvailabilityZone: aws.String("az-1"),
								},
								InstanceLifecycle: Spot,
							},
							typeInfo: &instanceTypeInformation{
								pricing: prices{
									LinuxSpotProduct: {
										onDemand: 0.5,
										spot: map[string]float64{
											"az-1": 0.1,
											"az-2": 0.2,
											"az-3": 0.3,
										},
									},
								},
							},
						},
						"2": {
							Instance: &ec2Types.Instance{
								InstanceId: aws.String("2"),
								Placement: &ec2Types.Placement{
									AvailabilityZone: aws.String("az-2"),
								},
							},
							typeInfo: &instanceTypeInformation{
								pricing: prices{
									LinuxSpotProduct: {
										onDemand: 0.8,
										spot: map[string]float64{
											"az-1": 0.4,
											"az-2": 0.5,
											"az-3": 0.6,
										},
									},
								},
							},
						},
					},
				),
			},
			ec2ASG: &autoscalingTypes.AutoScalingGroup{
				AutoScalingGroupName: aws.String("ASGName"),
				Instances: []autoscalingTypes.Instance{
					{
						InstanceId:     aws.String("1"),
						LifecycleState: "running",
					},
					{
						InstanceId:           aws.String("2"),
						ProtectedFromScaleIn: aws.Bool(true),
						AvailabilityZone:     aws.String("1a"),
						LifecycleState:       "running",
					},
					{
						InstanceId:     aws.String("3"),
						LifecycleState: "running",
					},
				},
			},
			expectedInstances: instanceMap{
				"1": {
					Instance: &ec2Types.Instance{
						InstanceId: aws.String("1"),
						Placement: &ec2Types.Placement{
							AvailabilityZone: aws.String("az-1"),
						},
						InstanceLifecycle: Spot,
					},
					typeInfo: &instanceTypeInformation{
						pricing: prices{
							LinuxSpotProduct: {
								onDemand: 0.5,
								spot: map[string]float64{
									"az-1": 0.1,
									"az-2": 0.2,
									"az-3": 0.3,
								},
							},
						},
					},
					initialPrice: 0.1,
					protected:    false,
				},
				"2": {
					Instance: &ec2Types.Instance{
						InstanceId: aws.String("2"),
						Placement: &ec2Types.Placement{
							AvailabilityZone: aws.String("az-2"),
						},
					},
					typeInfo: &instanceTypeInformation{
						pricing: prices{
							LinuxSpotProduct: {
								onDemand: 0.8,
								spot: map[string]float64{
									"az-1": 0.4,
									"az-2": 0.5,
									"az-3": 0.6,
								},
							},
						},
					},
					initialPrice: 0.8,
					protected:    true,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &autoScalingGroup{
				name:             "testASG",
				AutoScalingGroup: tt.ec2ASG,
				region:           tt.regionInstances,
				spotProduct:      aws.String(LinuxSpotProduct),
			}
			loadedInstances := a.scanInstances()
			for _, v := range tt.expectedInstances {
				v.asg, v.region = a, a.region
			}
			asgInstanceManager, receivedOk := loadedInstances.(*instanceManager)
			if !receivedOk {
				t.Errorf("instances of asg aren't valid - not of type *instanceManager")
			}
			if !reflect.DeepEqual(asgInstanceManager.catalog, tt.expectedInstances) {
				t.Errorf("scanInstances: catalog does not match, received: %+v, expected: %+v",
					asgInstanceManager.catalog,
					tt.expectedInstances)
			}
		})
	}
}

func Test_autoScalingGroup_hasMemberInstance(t *testing.T) {

	tests := []struct {
		name             string
		AutoScalingGroup *autoscalingTypes.AutoScalingGroup
		inst             *instance
		want             bool
	}{
		{
			name: "has member",
			AutoScalingGroup: &autoscalingTypes.AutoScalingGroup{
				Instances: []autoscalingTypes.Instance{
					{InstanceId: aws.String("foo")},
					{InstanceId: aws.String("bar")},
					{InstanceId: aws.String("baz")},
				},
			},
			inst: &instance{
				asg:      &autoScalingGroup{},
				Instance: &ec2Types.Instance{InstanceId: aws.String("bar")},
			},
			want: true,
		},
		{
			name: "doesn't have member",
			AutoScalingGroup: &autoscalingTypes.AutoScalingGroup{
				Instances: []autoscalingTypes.Instance{
					{InstanceId: aws.String("foo")},
					{InstanceId: aws.String("bar")},
					{InstanceId: aws.String("baz")},
				},
			},
			inst: &instance{
				asg:      &autoScalingGroup{},
				Instance: &ec2Types.Instance{InstanceId: aws.String("bazinga")},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &autoScalingGroup{
				AutoScalingGroup: tt.AutoScalingGroup,
			}
			if got := a.hasMemberInstance(tt.inst); got != tt.want {
				t.Errorf("autoScalingGroup.hasMemberInstance() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_autoScalingGroup_needReplaceOnDemandInstances(t *testing.T) {

	tests := []struct {
		name        string
		instances   instances
		minOnDemand int64
		wantNeed    bool
		wantTotal   int64
	}{
		{
			name:      "empty autoscalinggroup",
			instances: makeInstancesWithCatalog(instanceMap{}),
			wantNeed:  true,
			wantTotal: 0,
		},
		{
			name:        "autoscalinggroup with more on-demand instances than enough",
			minOnDemand: 0,
			instances: makeInstancesWithCatalog(instanceMap{
				"i-1": &instance{
					Instance: &ec2Types.Instance{
						State: &ec2Types.InstanceState{
							Name: "running",
						},
					},
				},
			}),
			wantNeed:  true,
			wantTotal: 1,
		},
		{
			name:        "autoscalinggroup with as much on-demand instances as desired",
			minOnDemand: 1,
			instances: makeInstancesWithCatalog(instanceMap{
				"i-1": &instance{
					Instance: &ec2Types.Instance{
						State: &ec2Types.InstanceState{
							Name: "running",
						},
					},
				},
			}),
			wantNeed:  false,
			wantTotal: 1,
		},
		{
			name:        "autoscalinggroup with fewer on-demand instances than desired",
			minOnDemand: 2,
			instances: makeInstancesWithCatalog(instanceMap{
				"i-1": &instance{
					Instance: &ec2Types.Instance{
						State: &ec2Types.InstanceState{
							Name: "running",
						},
					},
				},
			}),
			wantNeed:  false,
			wantTotal: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &autoScalingGroup{
				instances: tt.instances,
				config: AutoScalingConfig{
					MinOnDemand: tt.minOnDemand,
				},
			}
			gotNeed, gotTotal := a.needReplaceOnDemandInstances()
			if gotNeed != tt.wantNeed {
				t.Errorf("autoScalingGroup.needReplaceOnDemandInstances() got = %v, want %v", gotNeed, tt.wantNeed)

			}
			if gotTotal != tt.wantTotal {
				t.Errorf("autoScalingGroup.needReplaceOnDemandInstances() got1 = %v, want %v", gotTotal, tt.wantTotal)
			}
		})
	}
}

func Test_autoScalingGroup_cronEventAction(t *testing.T) {
	asgNoInstances := autoScalingGroup{
		spotProduct: aws.String(LinuxSpotProduct),
		AutoScalingGroup: &autoscalingTypes.AutoScalingGroup{
			DesiredCapacity:      aws.Int32(0),
			AutoScalingGroupName: aws.String("ASGName1"),
		},
		region: &region{
			conf: &Config{
				AutoScalingConfig: AutoScalingConfig{
					CronSchedule:      DefaultCronSchedule,
					CronScheduleState: "on",
				},
				Version: "custom",
			},
			instances: makeInstancesWithCatalog(
				instanceMap{}),
		},
	}

	asgCantTerminateOnDemandInstance := autoScalingGroup{
		spotProduct: aws.String(LinuxSpotProduct),
		AutoScalingGroup: &autoscalingTypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String("ASGName2"),
			DesiredCapacity:      aws.Int32(1),
			Instances: []autoscalingTypes.Instance{
				{
					InstanceId:           aws.String("i-ondemand"),
					InstanceType:         aws.String("m5.large"),
					ProtectedFromScaleIn: aws.Bool(false),
					LifecycleState:       "running",
				},
			},
		},
		region: &region{
			instanceTypeInformation: map[string]*instanceTypeInformation{
				"m5.large": {
					pricing: prices{
						LinuxSpotProduct: {
							onDemand: 30.3,
							spot: spotPriceMap{
								"us-east-1a": 0.2,
							},
						},
					},
				},
			},
			conf: &Config{
				AutoScalingConfig: AutoScalingConfig{
					CronSchedule:      DefaultCronSchedule,
					CronScheduleState: "on",
					MinOnDemandNumber: 1,
				},
				Version: "nightly",
			},
			instances: makeInstancesWithCatalog(
				instanceMap{
					"i-ondemand": {
						Instance: &ec2Types.Instance{
							InstanceId:   aws.String("i-ondemand"),
							InstanceType: "m5.large",
							ImageId:      aws.String("ami-12345678"),

							Placement: &ec2Types.Placement{
								AvailabilityZone: aws.String("us-east-1a"),
							},
							State: &ec2Types.InstanceState{
								Name: ec2Types.InstanceStateNameRunning,
							},
						},
						initialPrice: 30.3,
						typeInfo: &instanceTypeInformation{
							pricing: prices{
								LinuxSpotProduct: {
									onDemand: 30.3,
									spot: spotPriceMap{
										"us-east-1a": 0.2,
									},
								},
							},
						},
					},
				}),
			services: &connections{
				ec2: mockEC2Client{
					diao: &ec2.DescribeInstanceAttributeOutput{},

					damio: &ec2.DescribeImagesOutput{
						Images: []ec2Types.Image{
							{
								PlatformDetails: aws.String(LinuxSpotProduct),
								ImageId:         aws.String("ami-12345678"),
							},
						},
					},
				},
			},
		},
	}

	asgTerminateOnDemandInstance := autoScalingGroup{

		AutoScalingGroup: &autoscalingTypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String("ASGName"),
			DesiredCapacity:      aws.Int32(1),
			Instances: []autoscalingTypes.Instance{
				{
					InstanceId:           aws.String("i-ondemand"),
					InstanceType:         aws.String("m5.large"),
					ProtectedFromScaleIn: aws.Bool(false),
					LifecycleState:       "running",
				},
			},
		},
		region: &region{
			conf: &Config{
				AutoScalingConfig: AutoScalingConfig{
					CronSchedule:      DefaultCronSchedule,
					CronScheduleState: "on",
					MinOnDemandNumber: 0,
				},
				Version:    "nightly",
				FinalRecap: make(map[string][]string),
			},
			instanceTypeInformation: map[string]*instanceTypeInformation{
				"m5.large": {
					pricing: prices{
						LinuxSpotProduct: {
							onDemand: 30.3,
							spot: spotPriceMap{
								"us-east-1a": 0.2,
							},
						},
					},
				},
			},
			instances: makeInstancesWithCatalog(
				instanceMap{
					"i-ondemand": {
						Instance: &ec2Types.Instance{
							InstanceId:   aws.String("i-ondemand"),
							InstanceType: "m5.large",
							ImageId:      aws.String("ami-12345678"),
							Placement: &ec2Types.Placement{
								AvailabilityZone: aws.String("us-east-1a"),
							},
							State: &ec2Types.InstanceState{
								Name: ec2Types.InstanceStateNameRunning,
							},
						},
						initialPrice: 30.3,
						typeInfo: &instanceTypeInformation{
							pricing: prices{
								LinuxSpotProduct: {
									onDemand: 30.3,
									spot: spotPriceMap{
										"us-east-1a": 0.2,
									},
								},
							},
						},
					},
				}),
			services: &connections{
				ec2: mockEC2Client{
					diao: &ec2.DescribeInstanceAttributeOutput{},
					damio: &ec2.DescribeImagesOutput{
						Images: []ec2Types.Image{
							{
								PlatformDetails: aws.String(LinuxSpotProduct),
								ImageId:         aws.String("ami-12345678"),
							},
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name string
		asg  *autoScalingGroup
		want runer
	}{
		{name: "should not run",

			asg: &autoScalingGroup{
				spotProduct: aws.String(LinuxSpotProduct),
				AutoScalingGroup: &autoscalingTypes.AutoScalingGroup{
					AutoScalingGroupName: aws.String("ASGName"),
				},
				region: &region{
					conf: &Config{
						AutoScalingConfig: AutoScalingConfig{
							CronSchedule:      DefaultCronSchedule,
							CronScheduleState: "off",
						},
					},
					instances: makeInstancesWithCatalog(instanceMap{}),
				},
			},
			want: skipRun{reason: "outside-cron-schedule"},
		},

		{name: "no instances",
			asg:  &asgNoInstances,
			want: skipRun{reason: "no-instances-to-replace"},
		},

		{name: "not allowed to terminate on demand instance",
			asg:  &asgCantTerminateOnDemandInstance,
			want: skipRun{reason: "not-allowed-to-replace-more-instances"},
		},

		{name: "terminate on demand instance",
			asg: &asgTerminateOnDemandInstance,
			want: replaceAndTerminateInstance{
				target{
					autospotting:     nil,
					onDemandInstance: asgTerminateOnDemandInstance.region.instances.get("i-ondemand"),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if got := tt.asg.cronEventAction(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("autoScalingGroup.cronEventAction() \ngot %#v, \nwant %#v", got, tt.want)
			}
		})
	}
}

func TestHaveTooManyInstancesOfInstanceType(t *testing.T) {
	// Table tests
	var tests = []struct {
		name           string
		asg            autoScalingGroup
		instanceType   string
		az             string
		expectedResult bool
	}{
		{
			name: "Case 1: No instances, should return false",
			asg: autoScalingGroup{
				AutoScalingGroup: &autoscalingTypes.AutoScalingGroup{
					Instances: []autoscalingTypes.Instance{},
				},
				config: AutoScalingConfig{
					NumberOfInstanceTypesPerAZ: 3,
				},
			},
			instanceType:   "t2.micro",
			az:             "us-east-1a",
			expectedResult: false,
		},
		{
			name: "Case 2: Instances present but different AZ, should return false",
			asg: autoScalingGroup{
				AutoScalingGroup: &autoscalingTypes.AutoScalingGroup{
					Instances: []autoscalingTypes.Instance{
						{
							InstanceId:       aws.String("i-spot1"),
							InstanceType:     aws.String("t2.micro"),
							AvailabilityZone: aws.String("us-east-1b"),
						},
						{
							InstanceId:       aws.String("i-spot2"),
							InstanceType:     aws.String("t3.micro"),
							AvailabilityZone: aws.String("us-east-1b"),
						},
					},
				},
				config: AutoScalingConfig{
					NumberOfInstanceTypesPerAZ: 3,
				},
				region: &region{
					instances: makeInstancesWithCatalog(
						instanceMap{
							"i-spot1": {
								Instance: &ec2Types.Instance{
									InstanceId:        aws.String("i-spot1"),
									InstanceType:      "t2.micro",
									InstanceLifecycle: "spot",
									ImageId:           aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1b"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
								},
								initialPrice: 1.0,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 1.0,
											spot: spotPriceMap{
												"us-east-1b": 0.2,
											},
										},
									},
								},
							},
							"i-spot2": {
								Instance: &ec2Types.Instance{
									InstanceId:        aws.String("i-spot2"),
									InstanceType:      "t3.micro",
									InstanceLifecycle: "spot",
									ImageId:           aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1b"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
								},
								initialPrice: 1.0,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 1.0,
											spot: spotPriceMap{
												"us-east-1b": 0.2,
											},
										},
									},
								},
							},
						}),
				},
			},
			instanceType:   "t2.micro",
			az:             "us-east-1a",
			expectedResult: false,
		},

		{
			name: "Case 3: Instances present in same AZ, but different types and below limit, should return false",
			asg: autoScalingGroup{
				AutoScalingGroup: &autoscalingTypes.AutoScalingGroup{
					Instances: []autoscalingTypes.Instance{
						{
							InstanceId:       aws.String("i-spot1"),
							InstanceType:     aws.String("t2.micro"),
							AvailabilityZone: aws.String("us-east-1a"),
						},
						{
							InstanceId:       aws.String("i-spot2"),
							InstanceType:     aws.String("t3.micro"),
							AvailabilityZone: aws.String("us-east-1a"),
						},
					},
				},
				config: AutoScalingConfig{
					NumberOfInstanceTypesPerAZ: 3,
				},
				region: &region{
					instances: makeInstancesWithCatalog(
						instanceMap{
							"i-spot1": {
								Instance: &ec2Types.Instance{
									InstanceId:        aws.String("i-spot1"),
									InstanceType:      "t2.micro",
									InstanceLifecycle: "spot",
									ImageId:           aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1a"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
								},
								initialPrice: 1.0,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 1.0,
											spot: spotPriceMap{
												"us-east-1a": 0.2,
											},
										},
									},
								},
							},
							"i-spot2": {
								Instance: &ec2Types.Instance{
									InstanceId:        aws.String("i-spot2"),
									InstanceType:      "t3.micro",
									InstanceLifecycle: "spot",
									ImageId:           aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1a"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
								},
								initialPrice: 1.0,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 1.1,
											spot: spotPriceMap{
												"us-east-1a": 0.25,
											},
										},
									},
								},
							},
						}),
				},
			},
			instanceType:   "m5.large",
			az:             "us-east-1a",
			expectedResult: false,
		},
		{
			name: "Case 4: Instances present in same AZ, and would be too many of the same type, should return true",
			asg: autoScalingGroup{
				AutoScalingGroup: &autoscalingTypes.AutoScalingGroup{
					Instances: []autoscalingTypes.Instance{
						{
							InstanceId:       aws.String("i-spot1"),
							InstanceType:     aws.String("t2.micro"),
							AvailabilityZone: aws.String("us-east-1a"),
						},
						{
							InstanceId:       aws.String("i-spot2"),
							InstanceType:     aws.String("t3.micro"),
							AvailabilityZone: aws.String("us-east-1a"),
						},
					},
				},
				config: AutoScalingConfig{
					NumberOfInstanceTypesPerAZ: 3,
				},
				region: &region{
					instances: makeInstancesWithCatalog(
						instanceMap{
							"i-spot1": {
								Instance: &ec2Types.Instance{
									InstanceId:        aws.String("i-spot1"),
									InstanceType:      "t2.micro",
									InstanceLifecycle: "spot",
									ImageId:           aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1a"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
								},
								initialPrice: 1.0,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 1.0,
											spot: spotPriceMap{
												"us-east-1a": 0.2,
											},
										},
									},
								},
							},
							"i-spot2": {
								Instance: &ec2Types.Instance{
									InstanceId:        aws.String("i-spot2"),
									InstanceType:      "t3.micro",
									InstanceLifecycle: "spot",
									ImageId:           aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1a"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
								},
								initialPrice: 1.0,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 1.1,
											spot: spotPriceMap{
												"us-east-1a": 0.25,
											},
										},
									},
								},
							},
						}),
				},
			},
			instanceType:   "t2.micro",
			az:             "us-east-1a",
			expectedResult: true,
		},
		{
			name: "Case 5: Instances present in same AZ, different type, but exceeds limit, should return false",
			asg: autoScalingGroup{
				AutoScalingGroup: &autoscalingTypes.AutoScalingGroup{
					Instances: []autoscalingTypes.Instance{

						{
							InstanceId:       aws.String("i-spot1"),
							InstanceType:     aws.String("t2.micro"),
							AvailabilityZone: aws.String("us-east-1a"),
						},
						{
							InstanceId:       aws.String("i-spot2"),
							InstanceType:     aws.String("t3.micro"),
							AvailabilityZone: aws.String("us-east-1a"),
						},
						{
							InstanceId:       aws.String("i-spot3"),
							InstanceType:     aws.String("m5.large"),
							AvailabilityZone: aws.String("us-east-1a"),
						},
					},
				},
				config: AutoScalingConfig{
					NumberOfInstanceTypesPerAZ: 3,
				},
				region: &region{
					instances: makeInstancesWithCatalog(
						instanceMap{
							"i-spot1": {
								Instance: &ec2Types.Instance{
									InstanceId:        aws.String("i-spot1"),
									InstanceType:      "t2.micro",
									InstanceLifecycle: "spot",
									ImageId:           aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1a"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
								},
								initialPrice: 1.0,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 1.0,
											spot: spotPriceMap{
												"us-east-1a": 0.2,
											},
										},
									},
								},
							},
							"i-spot2": {
								Instance: &ec2Types.Instance{
									InstanceId:        aws.String("i-spot2"),
									InstanceType:      "t3.micro",
									InstanceLifecycle: "spot",
									ImageId:           aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1a"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
								},
								initialPrice: 1.0,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 1.1,
											spot: spotPriceMap{
												"us-east-1a": 0.25,
											},
										},
									},
								},
							},
							"i-spot3": {
								Instance: &ec2Types.Instance{
									InstanceId:        aws.String("i-spot3"),
									InstanceType:      "m3.large",
									InstanceLifecycle: "spot",
									ImageId:           aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1a"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
								},
								initialPrice: 1.0,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 2.1,
											spot: spotPriceMap{
												"us-east-1a": 0.45,
											},
										},
									},
								},
							},
						}),
				},
			},
			instanceType:   "m4.large",
			az:             "us-east-1a",
			expectedResult: false,
		},

		{
			name: "Case 6: Instances present in same AZ, same type, exceeds limit but wouldn't imbalance the autoscalinggroup, should return false",
			asg: autoScalingGroup{
				AutoScalingGroup: &autoscalingTypes.AutoScalingGroup{
					Instances: []autoscalingTypes.Instance{
						{
							InstanceId:       aws.String("i-spot4"),
							InstanceType:     aws.String("t2.micro"),
							AvailabilityZone: aws.String("us-east-1a"),
						},
						{
							InstanceId:       aws.String("i-spot5"),
							InstanceType:     aws.String("t3.micro"),
							AvailabilityZone: aws.String("us-east-1a"),
						},
						{
							InstanceId:       aws.String("i-spot6"),
							InstanceType:     aws.String("m5.large"),
							AvailabilityZone: aws.String("us-east-1a"),
						},
					},
				},
				config: AutoScalingConfig{
					NumberOfInstanceTypesPerAZ: 3,
				},
				region: &region{
					instances: makeInstancesWithCatalog(
						instanceMap{
							"i-spot4": {
								Instance: &ec2Types.Instance{
									InstanceId:        aws.String("i-spot4"),
									InstanceType:      "t2.micro",
									InstanceLifecycle: "spot",
									ImageId:           aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1a"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
								},
								initialPrice: 1.0,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 1.0,
											spot: spotPriceMap{
												"us-east-1a": 0.2,
											},
										},
									},
								},
							},
							"i-spot5": {
								Instance: &ec2Types.Instance{
									InstanceId:        aws.String("i-spot5"),
									InstanceType:      "t3.micro",
									InstanceLifecycle: "spot",
									ImageId:           aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1a"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
								},
								initialPrice: 1.0,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 1.1,
											spot: spotPriceMap{
												"us-east-1a": 0.25,
											},
										},
									},
								},
							},
							"i-spot6": {
								Instance: &ec2Types.Instance{
									InstanceId:        aws.String("i-spot6"),
									InstanceType:      "m5.large",
									InstanceLifecycle: "spot",
									ImageId:           aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1a"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
								},
								initialPrice: 1.0,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 2.0,
											spot: spotPriceMap{
												"us-east-1a": 0.45,
											},
										},
									},
								},
							},
						}),
				},
			},
			instanceType:   "m5.large",
			az:             "us-east-1a",
			expectedResult: false,
		},

		{
			name: "Case 7: Instances present in same AZ, same type, exceeds limit and would imbalance the autoscalinggroup, should return true",
			asg: autoScalingGroup{
				AutoScalingGroup: &autoscalingTypes.AutoScalingGroup{
					Instances: []autoscalingTypes.Instance{
						{
							InstanceId:       aws.String("i-spot4"),
							InstanceType:     aws.String("t2.micro"),
							AvailabilityZone: aws.String("us-east-1a"),
						},
						{
							InstanceId:       aws.String("i-spot5"),
							InstanceType:     aws.String("t3.micro"),
							AvailabilityZone: aws.String("us-east-1a"),
						},
						{
							InstanceId:       aws.String("i-spot6"),
							InstanceType:     aws.String("m5.large"),
							AvailabilityZone: aws.String("us-east-1a"),
						},
						{
							InstanceId:       aws.String("i-spot7"),
							InstanceType:     aws.String("m5.large"),
							AvailabilityZone: aws.String("us-east-1a"),
						},
					},
				},
				config: AutoScalingConfig{
					NumberOfInstanceTypesPerAZ: 3,
				},
				region: &region{
					instances: makeInstancesWithCatalog(
						instanceMap{
							"i-spot4": {
								Instance: &ec2Types.Instance{
									InstanceId:        aws.String("i-spot4"),
									InstanceType:      "t2.micro",
									InstanceLifecycle: "spot",
									ImageId:           aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1a"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
								},
								initialPrice: 1.0,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 1.0,
											spot: spotPriceMap{
												"us-east-1a": 0.2,
											},
										},
									},
								},
							},
							"i-spot5": {
								Instance: &ec2Types.Instance{
									InstanceId:        aws.String("i-spot5"),
									InstanceType:      "t3.micro",
									InstanceLifecycle: "spot",
									ImageId:           aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1a"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
								},
								initialPrice: 1.0,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 1.1,
											spot: spotPriceMap{
												"us-east-1a": 0.25,
											},
										},
									},
								},
							},
							"i-spot6": {
								Instance: &ec2Types.Instance{
									InstanceId:        aws.String("i-spot6"),
									InstanceType:      "m5.large",
									InstanceLifecycle: "spot",
									ImageId:           aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1a"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
								},
								initialPrice: 1.0,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 2.1,
											spot: spotPriceMap{
												"us-east-1a": 0.45,
											},
										},
									},
								},
							},
							"i-spot7": {
								Instance: &ec2Types.Instance{
									InstanceId:        aws.String("i-spot7"),
									InstanceType:      "m5.large",
									InstanceLifecycle: "spot",
									ImageId:           aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1a"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
								},
								initialPrice: 1.0,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 2.1,
											spot: spotPriceMap{
												"us-east-1a": 0.45,
											},
										},
									},
								},
							},
						}),
				},
			},
			instanceType:   "m5.large",
			az:             "us-east-1a",
			expectedResult: true,
		},

		{
			name: "Case 8: Instances present in same AZ, same type, exceeds limit and would not imbalance the autoscalinggroup, should return false",
			asg: autoScalingGroup{
				AutoScalingGroup: &autoscalingTypes.AutoScalingGroup{
					Instances: []autoscalingTypes.Instance{
						{
							InstanceId:       aws.String("i-spot4"),
							InstanceType:     aws.String("t2.micro"),
							AvailabilityZone: aws.String("us-east-1a"),
						},
						{
							InstanceId:       aws.String("i-spot5"),
							InstanceType:     aws.String("t3.micro"),
							AvailabilityZone: aws.String("us-east-1a"),
						},
						{
							InstanceId:       aws.String("i-spot6"),
							InstanceType:     aws.String("m5.large"),
							AvailabilityZone: aws.String("us-east-1a"),
						},
						{
							InstanceId:       aws.String("i-spot7"),
							InstanceType:     aws.String("c5.large"),
							AvailabilityZone: aws.String("us-east-1a"),
						},
					},
				},
				config: AutoScalingConfig{
					NumberOfInstanceTypesPerAZ: 3,
				},
				region: &region{
					instances: makeInstancesWithCatalog(
						instanceMap{
							"i-spot4": {
								Instance: &ec2Types.Instance{
									InstanceId:        aws.String("i-spot4"),
									InstanceType:      "t2.micro",
									InstanceLifecycle: "spot",
									ImageId:           aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1a"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
								},
								initialPrice: 1.0,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 1.0,
											spot: spotPriceMap{
												"us-east-1a": 0.2,
											},
										},
									},
								},
							},
							"i-spot5": {
								Instance: &ec2Types.Instance{
									InstanceId:        aws.String("i-spot5"),
									InstanceType:      "t3.micro",
									InstanceLifecycle: "spot",
									ImageId:           aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1a"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
								},
								initialPrice: 1.0,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 1.1,
											spot: spotPriceMap{
												"us-east-1a": 0.25,
											},
										},
									},
								},
							},
							"i-spot6": {
								Instance: &ec2Types.Instance{
									InstanceId:        aws.String("i-spot6"),
									InstanceType:      "m5.large",
									InstanceLifecycle: "spot",
									ImageId:           aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1a"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
								},
								initialPrice: 2.0,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 2.0,
											spot: spotPriceMap{
												"us-east-1a": 0.4,
											},
										},
									},
								},
							},
							"i-spot7": {
								Instance: &ec2Types.Instance{
									InstanceId:        aws.String("i-spot7"),
									InstanceType:      "c5.large",
									InstanceLifecycle: "spot",
									ImageId:           aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1a"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
								},
								initialPrice: 2.5,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 2.5,
											spot: spotPriceMap{
												"us-east-1a": 0.5,
											},
										},
									},
								},
							},
						}),
				},
			},
			instanceType:   "m5.large",
			az:             "us-east-1a",
			expectedResult: false,
		},

		{
			name: "Case 9: Multiple instances in same AZ and of same type, below limit, should return true",
			asg: autoScalingGroup{
				AutoScalingGroup: &autoscalingTypes.AutoScalingGroup{
					Instances: []autoscalingTypes.Instance{
						{InstanceId: aws.String("i-spot4"), InstanceType: aws.String("t2.micro"), AvailabilityZone: aws.String("us-east-1a")},
						{InstanceId: aws.String("i-spot5"), InstanceType: aws.String("t2.micro"), AvailabilityZone: aws.String("us-east-1a")},
					},
				},
				config: AutoScalingConfig{
					NumberOfInstanceTypesPerAZ: 2,
				},
				region: &region{
					instances: makeInstancesWithCatalog(
						instanceMap{
							"i-spot4": {
								Instance: &ec2Types.Instance{
									InstanceId:        aws.String("i-spot4"),
									InstanceType:      "t2.micro",
									InstanceLifecycle: "spot",
									ImageId:           aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1a"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
								},
								initialPrice: 1.0,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 1.0,
											spot: spotPriceMap{
												"us-east-1a": 0.2,
											},
										},
									},
								},
							},
							"i-spot5": {
								Instance: &ec2Types.Instance{
									InstanceId:        aws.String("i-spot5"),
									InstanceType:      "t2.micro",
									InstanceLifecycle: "spot",
									ImageId:           aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1a"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
								},
								initialPrice: 1.0,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 1.0,
											spot: spotPriceMap{
												"us-east-1a": 0.2,
											},
										},
									},
								},
							},
						}),
				},
			},
			instanceType:   "t2.micro",
			az:             "us-east-1a",
			expectedResult: true,
		},

		{
			name: "Case 10: Multiple instances of different types in same AZ, just hitting the limit, should return false",
			asg: autoScalingGroup{
				AutoScalingGroup: &autoscalingTypes.AutoScalingGroup{
					Instances: []autoscalingTypes.Instance{
						{
							InstanceId:       aws.String("i-spot4"),
							InstanceType:     aws.String("t2.micro"),
							AvailabilityZone: aws.String("us-east-1a"),
						},
						{
							InstanceId:       aws.String("i-spot5"),
							InstanceType:     aws.String("t3.micro"),
							AvailabilityZone: aws.String("us-east-1a"),
						},
						{
							InstanceId:       aws.String("i-spot6"),
							InstanceType:     aws.String("m5.large"),
							AvailabilityZone: aws.String("us-east-1a"),
						},
					},
				},
				config: AutoScalingConfig{
					NumberOfInstanceTypesPerAZ: 3,
				},
				region: &region{
					instances: makeInstancesWithCatalog(
						instanceMap{
							"i-spot4": {
								Instance: &ec2Types.Instance{
									InstanceId:        aws.String("i-spot4"),
									InstanceType:      "t2.micro",
									InstanceLifecycle: "spot",
									ImageId:           aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1a"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
								},
								initialPrice: 1.0,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 1.0,
											spot: spotPriceMap{
												"us-east-1a": 0.2,
											},
										},
									},
								},
							},
							"i-spot5": {
								Instance: &ec2Types.Instance{
									InstanceId:        aws.String("i-spot5"),
									InstanceType:      "t3.micro",
									InstanceLifecycle: "spot",
									ImageId:           aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1a"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
								},
								initialPrice: 1.0,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 1.1,
											spot: spotPriceMap{
												"us-east-1a": 0.25,
											},
										},
									},
								},
							},
							"i-spot6": {
								Instance: &ec2Types.Instance{
									InstanceId:        aws.String("i-spot6"),
									InstanceType:      "m5.large",
									InstanceLifecycle: "spot",
									ImageId:           aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1a"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
								},
								initialPrice: 1.0,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 2.1,
											spot: spotPriceMap{
												"us-east-1a": 0.45,
											},
										},
									},
								},
							},
						}),
				},
			},
			instanceType:   "c5.large",
			az:             "us-east-1a",
			expectedResult: false,
		},

		{
			name: "Case 11: Multiple instances in different AZs of the same type, should return true",
			asg: autoScalingGroup{
				AutoScalingGroup: &autoscalingTypes.AutoScalingGroup{
					Instances: []autoscalingTypes.Instance{
						{
							InstanceId:       aws.String("i-spot4"),
							InstanceType:     aws.String("t2.micro"),
							AvailabilityZone: aws.String("us-east-1a"),
						},
						{
							InstanceId:       aws.String("i-spot5"),
							InstanceType:     aws.String("t2.micro"),
							AvailabilityZone: aws.String("us-east-1b"),
						},
					},
				},
				config: AutoScalingConfig{
					NumberOfInstanceTypesPerAZ: 3,
				},
				region: &region{
					instances: makeInstancesWithCatalog(
						instanceMap{
							"i-spot4": {
								Instance: &ec2Types.Instance{
									InstanceId:        aws.String("i-spot4"),
									InstanceType:      "t2.micro",
									InstanceLifecycle: "spot",
									ImageId:           aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1a"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
								},
								initialPrice: 1.0,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 1.0,
											spot: spotPriceMap{
												"us-east-1a": 0.2,
											},
										},
									},
								},
							},
							"i-spot5": {
								Instance: &ec2Types.Instance{
									InstanceId:        aws.String("i-spot5"),
									InstanceType:      "t2.micro",
									InstanceLifecycle: "spot",
									ImageId:           aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1b"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
								},
								initialPrice: 1.0,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 1.0,
											spot: spotPriceMap{
												"us-east-1b": 0.2,
											},
										},
									},
								},
							},
						}),
				},
			},
			instanceType:   "t2.micro",
			az:             "us-east-1a",
			expectedResult: true,
		},

		{
			name: "Case 12: Instances exceed limit in one AZ but not the other, should return true",
			asg: autoScalingGroup{
				AutoScalingGroup: &autoscalingTypes.AutoScalingGroup{
					Instances: []autoscalingTypes.Instance{
						{
							InstanceId:       aws.String("i-spot1"),
							InstanceType:     aws.String("t2.micro"),
							AvailabilityZone: aws.String("us-east-1a"),
						},
						{
							InstanceId:       aws.String("i-spot2"),
							InstanceType:     aws.String("t2.micro"),
							AvailabilityZone: aws.String("us-east-1a"),
						},
						{
							InstanceId:       aws.String("i-spot3"),
							InstanceType:     aws.String("t3.micro"),
							AvailabilityZone: aws.String("us-east-1a"),
						},
						{
							InstanceId:       aws.String("i-spot4"),
							InstanceType:     aws.String("m5.large"),
							AvailabilityZone: aws.String("us-east-1a"),
						},
						{
							InstanceId:       aws.String("i-spot5"),
							InstanceType:     aws.String("t2.micro"),
							AvailabilityZone: aws.String("us-east-1b"),
						},
						{
							InstanceId:       aws.String("i-spot6"),
							InstanceType:     aws.String("t3.micro"),
							AvailabilityZone: aws.String("us-east-1b"),
						},
					},
				},
				config: AutoScalingConfig{
					NumberOfInstanceTypesPerAZ: 3,
				},
				region: &region{
					instances: makeInstancesWithCatalog(
						instanceMap{
							"i-spot1": {
								Instance: &ec2Types.Instance{
									InstanceId:   aws.String("i-spot1"),
									InstanceType: "t2.micro",
									ImageId:      aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1a"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
									InstanceLifecycle: "spot",
								},
								initialPrice: 1.0,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 1.0,
											spot: spotPriceMap{
												"us-east-1a": 0.2,
											},
										},
									},
								},
							},
							"i-spot2": {
								Instance: &ec2Types.Instance{
									InstanceId:   aws.String("i-spot2"),
									InstanceType: "t2.micro",
									ImageId:      aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1a"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
									InstanceLifecycle: "spot",
								},
								initialPrice: 1.0,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 1.1,
											spot: spotPriceMap{
												"us-east-1a": 0.25,
											},
										},
									},
								},
							},
							"i-spot3": {
								Instance: &ec2Types.Instance{
									InstanceId:   aws.String("i-spot3"),
									InstanceType: "t3.micro",
									ImageId:      aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1a"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
									InstanceLifecycle: "spot",
								},
								initialPrice: 1.0,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 2.1,
											spot: spotPriceMap{
												"us-east-1a": 0.45,
											},
										},
									},
								},
							},
							"i-spot4": {
								Instance: &ec2Types.Instance{
									InstanceId:   aws.String("i-spot4"),
									InstanceType: "m5.large",
									ImageId:      aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1a"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
									InstanceLifecycle: "spot",
								},
								initialPrice: 1.0,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 1.0,
											spot: spotPriceMap{
												"us-east-1b": 0.2,
											},
										},
									},
								},
							},
							"i-spot5": {
								Instance: &ec2Types.Instance{
									InstanceId:   aws.String("i-spot5"),
									InstanceType: "t2.micro",
									ImageId:      aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1b"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
									InstanceLifecycle: "spot",
								},
								initialPrice: 1.0,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 1.1,
											spot: spotPriceMap{
												"us-east-1b": 0.25,
											},
										},
									},
								},
							},
							"i-spot6": {
								Instance: &ec2Types.Instance{
									InstanceId:   aws.String("i-spot6"),
									InstanceType: "t3.micro",
									ImageId:      aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1b"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
									InstanceLifecycle: "spot",
								},
								initialPrice: 1.0,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 1.1,
											spot: spotPriceMap{
												"us-east-1b": 0.25,
											},
										},
									},
								},
							},
						}),
				},
			},
			instanceType:   "t2.micro",
			az:             "us-east-1a",
			expectedResult: true,
		},

		// Then the same case as above, but check for "us-east-1b"
		{
			name: "Case 13: Instances exceed limit in one AZ but not the other, should return true",
			asg: autoScalingGroup{
				AutoScalingGroup: &autoscalingTypes.AutoScalingGroup{
					Instances: []autoscalingTypes.Instance{
						{
							InstanceId:       aws.String("i-spot6"),
							InstanceType:     aws.String("t2.micro"),
							AvailabilityZone: aws.String("us-east-1a"),
						},
						{
							InstanceId:       aws.String("i-spot7"),
							InstanceType:     aws.String("t2.micro"),
							AvailabilityZone: aws.String("us-east-1a"),
						},
						{
							InstanceId:       aws.String("i-spot8"),
							InstanceType:     aws.String("t3.micro"),
							AvailabilityZone: aws.String("us-east-1a"),
						},
						{
							InstanceId:       aws.String("i-spot9"),
							InstanceType:     aws.String("m5.large"),
							AvailabilityZone: aws.String("us-east-1a"),
						},
						{
							InstanceId:       aws.String("i-spot10"),
							InstanceType:     aws.String("t2.micro"),
							AvailabilityZone: aws.String("us-east-1b"),
						},
						{
							InstanceId:       aws.String("i-spot11"),
							InstanceType:     aws.String("t3.micro"),
							AvailabilityZone: aws.String("us-east-1b"),
						},
					},
				},
				config: AutoScalingConfig{
					NumberOfInstanceTypesPerAZ: 3,
				},
				region: &region{
					instances: makeInstancesWithCatalog(
						instanceMap{
							"i-spot6": {
								Instance: &ec2Types.Instance{
									InstanceId:        aws.String("i-spot6"),
									InstanceType:      "t2.micro",
									InstanceLifecycle: "spot",
									ImageId:           aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1a"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
								},
								initialPrice: 1.0,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 1.0,
											spot: spotPriceMap{
												"us-east-1a": 0.2,
											},
										},
									},
								},
							},
							"i-spot7": {
								Instance: &ec2Types.Instance{
									InstanceId:        aws.String("i-spot7"),
									InstanceType:      "t2.micro",
									InstanceLifecycle: "spot",
									ImageId:           aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1a"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
								},
								initialPrice: 1.0,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 1.0,
											spot: spotPriceMap{
												"us-east-1a": 0.2,
											},
										},
									},
								},
							},
							"i-spot8": {
								Instance: &ec2Types.Instance{
									InstanceId:        aws.String("i-spot8"),
									InstanceType:      "t3.micro",
									InstanceLifecycle: "spot",
									ImageId:           aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1a"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
								},
								initialPrice: 1.0,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 1.0,
											spot: spotPriceMap{
												"us-east-1a": 0.2,
											},
										},
									},
								},
							},
							"i-spot9": {
								Instance: &ec2Types.Instance{
									InstanceId:        aws.String("i-spot9"),
									InstanceType:      "m5.large",
									InstanceLifecycle: "spot",
									ImageId:           aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1a"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
								},
								initialPrice: 1.0,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 1.0,
											spot: spotPriceMap{
												"us-east-1a": 0.2,
											},
										},
									},
								},
							},
							"i-spot10": {
								Instance: &ec2Types.Instance{
									InstanceId:        aws.String("i-spot10"),
									InstanceType:      "t2.micro",
									InstanceLifecycle: "spot",
									ImageId:           aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1b"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
								},
								initialPrice: 1.0,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 1.0,
											spot: spotPriceMap{
												"us-east-1b": 0.2,
											},
										},
									},
								},
							},
							"i-spot11": {
								Instance: &ec2Types.Instance{
									InstanceId:        aws.String("i-spot11"),
									InstanceType:      "t3.micro",
									InstanceLifecycle: "spot",
									ImageId:           aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1b"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
								},
								initialPrice: 1.0,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 1.0,
											spot: spotPriceMap{
												"us-east-1b": 0.2,
											},
										},
									},
								},
							},
						}),
				},
			},
			instanceType:   "t2.micro",
			az:             "us-east-1b",
			expectedResult: true,
		},

		{
			name: "Case 14: No instances in the desired AZ, but instances of the desired type in another AZ, should return false",
			asg: autoScalingGroup{
				AutoScalingGroup: &autoscalingTypes.AutoScalingGroup{
					Instances: []autoscalingTypes.Instance{
						{
							InstanceId:       aws.String("i-spot11"),
							InstanceType:     aws.String("t2.micro"),
							AvailabilityZone: aws.String("us-east-1b"),
						},
					},
				},
				config: AutoScalingConfig{
					NumberOfInstanceTypesPerAZ: 3,
				},
				region: &region{
					instances: makeInstancesWithCatalog(
						instanceMap{
							"i-spot11": {
								Instance: &ec2Types.Instance{
									InstanceId:        aws.String("i-spot11"),
									InstanceType:      "t2.micro",
									InstanceLifecycle: "spot",
									ImageId:           aws.String("ami-12345678"),
									Placement: &ec2Types.Placement{
										AvailabilityZone: aws.String("us-east-1b"),
									},
									State: &ec2Types.InstanceState{
										Name: ec2Types.InstanceStateNameRunning,
									},
								},
								initialPrice: 1.0,
								typeInfo: &instanceTypeInformation{
									pricing: prices{
										LinuxSpotProduct: {
											onDemand: 1.0,
											spot: spotPriceMap{
												"us-east-1b": 0.2,
											},
										},
									},
								},
							},
						}),
				},
			},
			instanceType:   "t2.micro",
			az:             "us-east-1a",
			expectedResult: false,
		},
	}

	for _, test := range tests {
		fmt.Println(test.name)
		result, _ := test.asg.willCauseInstanceTypeImbalance(test.instanceType, test.az)
		if result != test.expectedResult {
			t.Errorf("%s failed, expected %v, got %v", test.name, test.expectedResult, result)
		}
	}
}
