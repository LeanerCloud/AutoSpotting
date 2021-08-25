// Copyright (c) 2016-2019 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
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
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameStopped)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String(Spot),
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
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String(OnDemand),
						},
					},
					"id-2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String(OnDemand),
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
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String(Spot),
						},
					},
					"id-2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String(Spot),
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
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String(OnDemand),
						},
					},
					"id-2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String(OnDemand),
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
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String(OnDemand),
						},
					},
					"id-2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String(OnDemand),
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
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String(Spot),
						},
					},
					"id-2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String(Spot),
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
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String(Spot),
						},
					},
					"id-2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String(Spot),
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
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameShuttingDown)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String(Spot),
						},
					},
					"id-2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String(OnDemand),
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
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String(Spot),
						},
					},
					"id-2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String(OnDemand),
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
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String(Spot),
						},
					},
					"id-2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameStopped)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String(OnDemand),
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
			a := autoScalingGroup{Group: &autoscaling.Group{}}
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
// 			desiredCapacity: aws.Int64(0),
// 			expectedRun:     true,
// 		},
// 		{name: "ASG has no instance at all - 0 on-demand required",
// 			asgInstances:    makeInstances(),
// 			minOnDemand:     0,
// 			desiredCapacity: aws.Int64(0),
// 			expectedRun:     true,
// 		},
// 		{name: "ASG has no instance running - 1 on-demand required",
// 			asgInstances: makeInstancesWithCatalog(
// 				instanceMap{
// 					"id-1": {
// 						Instance: &ec2.Instance{
// 							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameShuttingDown)},
// 							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
// 							InstanceLifecycle: aws.String(Spot),
// 						},
// 					},
// 					"id-2": {
// 						Instance: &ec2.Instance{
// 							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameShuttingDown)},
// 							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
// 							InstanceLifecycle: aws.String(OnDemand),
// 						},
// 					},
// 				},
// 			),
// 			minOnDemand:     1,
// 			desiredCapacity: aws.Int64(0),
// 			expectedRun:     true,
// 		},
// 		{name: "ASG has no instance running - 0 on-demand required",
// 			asgInstances: makeInstancesWithCatalog(
// 				instanceMap{
// 					"id-1": {
// 						Instance: &ec2.Instance{
// 							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameShuttingDown)},
// 							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
// 							InstanceLifecycle: aws.String(Spot),
// 						},
// 					},
// 					"id-2": {
// 						Instance: &ec2.Instance{
// 							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameShuttingDown)},
// 							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
// 							InstanceLifecycle: aws.String(OnDemand),
// 						},
// 					},
// 				},
// 			),
// 			minOnDemand:     0,
// 			desiredCapacity: aws.Int64(0),
// 			expectedRun:     true,
// 		},
// 		{name: "ASG has not the required on-demand running",
// 			asgInstances: makeInstancesWithCatalog(
// 				instanceMap{
// 					"id-1": {
// 						Instance: &ec2.Instance{
// 							InstanceId:        aws.String("id-1"),
// 							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
// 							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
// 							InstanceLifecycle: aws.String(Spot),
// 						},
// 						region: &region{
// 							name: "test-region",
// 							services: connections{
// 								ec2: &mockEC2{},
// 							},
// 						},
// 					},
// 					"id-2": {
// 						Instance: &ec2.Instance{
// 							InstanceId:        aws.String("id-2"),
// 							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
// 							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
// 							InstanceLifecycle: aws.String(OnDemand),
// 						},
// 					},
// 				},
// 			),
// 			minOnDemand:     2,
// 			desiredCapacity: aws.Int64(0),
// 			expectedRun:     false,
// 			regionASG: &region{
// 				name: "regionTest",
// 				services: connections{
// 					autoScaling: mockASG{},
// 				},
// 				conf: &Config{},
// 			},
// 		},
// 		{name: "ASG has just enough on-demand instances running",
// 			asgInstances: makeInstancesWithCatalog(
// 				instanceMap{
// 					"id-1": {
// 						Instance: &ec2.Instance{
// 							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
// 							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
// 							InstanceLifecycle: aws.String(Spot),
// 						},
// 					},
// 					"id-2": {
// 						Instance: &ec2.Instance{
// 							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
// 							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
// 							InstanceLifecycle: aws.String(OnDemand),
// 						},
// 					},
// 				},
// 			),
// 			minOnDemand:     1,
// 			desiredCapacity: aws.Int64(0),
// 			expectedRun:     false,
// 		},
// 		{name: "ASG has only one remaining instance, less than enough on-demand",
// 			asgInstances: makeInstancesWithCatalog(
// 				instanceMap{
// 					"id-1": {
// 						Instance: &ec2.Instance{
// 							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
// 							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
// 							InstanceLifecycle: aws.String(Spot),
// 						},
// 					},
// 				},
// 			),
// 			minOnDemand:     1,
// 			desiredCapacity: aws.Int64(1),
// 			expectedRun:     false,
// 		},
// 		{name: "ASG has more than enough on-demand instances running but not desired capacity",
// 			asgInstances: makeInstancesWithCatalog(
// 				instanceMap{
// 					"id-1": {
// 						Instance: &ec2.Instance{
// 							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
// 							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
// 							InstanceLifecycle: aws.String(OnDemand),
// 						},
// 					},
// 					"id-2": {
// 						Instance: &ec2.Instance{
// 							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
// 							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
// 							InstanceLifecycle: aws.String(OnDemand),
// 						},
// 					},
// 				},
// 			),
// 			minOnDemand:     1,
// 			desiredCapacity: aws.Int64(1),
// 			expectedRun:     true,
// 		},
// 		{name: "ASG has more than enough on-demand instances running and desired capacity",
// 			asgInstances: makeInstancesWithCatalog(
// 				instanceMap{
// 					"id-1": {
// 						Instance: &ec2.Instance{
// 							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
// 							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
// 							InstanceLifecycle: aws.String(OnDemand),
// 						},
// 					},
// 					"id-2": {
// 						Instance: &ec2.Instance{
// 							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
// 							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
// 							InstanceLifecycle: aws.String(OnDemand),
// 						},
// 					},
// 				},
// 			),
// 			minOnDemand:     1,
// 			desiredCapacity: aws.Int64(4),
// 			expectedRun:     true,
// 		},
// 		{name: "ASG has on-demand instances equal to the min on-demand number",
// 			asgInstances: makeInstancesWithCatalog(
// 				map[string]*instance{
// 					"id-1": {
// 						Instance: &ec2.Instance{
// 							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
// 							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
// 							InstanceLifecycle: aws.String(OnDemand),
// 						},
// 					},
// 					"id-2": {
// 						Instance: &ec2.Instance{
// 							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
// 							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
// 							InstanceLifecycle: aws.String(Spot),
// 						},
// 					},
// 				},
// 			),
// 			minOnDemand:     1,
// 			desiredCapacity: aws.Int64(2),
// 			expectedRun:     false,
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			a := autoScalingGroup{Group: &autoscaling.Group{}}
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

func TestDetachAndTerminateOnDemandInstance(t *testing.T) {
	tests := []struct {
		name         string
		instancesASG instances
		regionASG    *region
		instanceID   *string
		expected     error
	}{
		// {name: "no err during detach nor terminate",
		// 	instancesASG: makeInstancesWithCatalog(
		// 		instanceMap{
		// 			"1": {
		// 				Instance: &ec2.Instance{
		// 					InstanceId: aws.String("1"),
		// 					State: &ec2.InstanceState{
		// 						Name: aws.String(ec2.InstanceStateNameRunning),
		// 					},
		// 				},
		// 				region: &region{
		// 					services: connections{
		// 						ec2: mockEC2{
		// 							tierr: nil,
		// 							dio:   &ec2.DescribeInstancesOutput{},
		// 						},
		// 					},
		// 				},
		// 			},
		// 		},
		// 	),
		// 	regionASG: &region{
		// 		name: "regionTest",
		// 		instances: makeInstancesWithCatalog(
		// 			instanceMap{
		// 				"1": {
		// 					Instance: &ec2.Instance{
		// 						InstanceId: aws.String("1"),
		// 						State: &ec2.InstanceState{
		// 							Name: aws.String(ec2.InstanceStateNameRunning),
		// 						},
		// 					},
		// 				},
		// 			}),
		// 		services: connections{
		// 			autoScaling: mockASG{dierr: nil},
		// 			ec2:         mockEC2{},
		// 		},
		// 		conf: &Config{},
		// 	},
		// 	instanceID: aws.String("1"),
		// 	expected:   nil,
		// },
		// {name: "err during detach not during terminate",
		// 	instancesASG: makeInstancesWithCatalog(
		// 		instanceMap{
		// 			"1": {
		// 				Instance: &ec2.Instance{
		// 					InstanceId: aws.String("1"),
		// 					State: &ec2.InstanceState{
		// 						Name: aws.String(ec2.InstanceStateNameRunning),
		// 					},
		// 				},
		// 				region: &region{
		// 					instances: makeInstancesWithCatalog(
		// 						instanceMap{
		// 							"1": {
		// 								Instance: &ec2.Instance{
		// 									InstanceId: aws.String("1"),
		// 									State: &ec2.InstanceState{
		// 										Name: aws.String(ec2.InstanceStateNameRunning),
		// 									},
		// 								},
		// 							},
		// 						}),
		// 					services: connections{
		// 						ec2: mockEC2{
		// 							tierr: nil,
		// 							dio:   &ec2.DescribeInstancesOutput{},
		// 						},
		// 					},
		// 				},
		// 			},
		// 		},
		// 	),
		// 	regionASG: &region{
		// 		name: "regionTest",
		// 		services: connections{
		// 			autoScaling: mockASG{dierr: errors.New("detach")},
		// 		},
		// 		conf: &Config{},
		// 	},
		// 	instanceID: aws.String("1"),
		// 	expected:   errors.New("detach"),
		// },
		// {name: "no err during detach but error during terminate",
		// 	instancesASG: makeInstancesWithCatalog(
		// 		instanceMap{
		// 			"1": {
		// 				Instance: &ec2.Instance{
		// 					InstanceId: aws.String("1"),
		// 					State: &ec2.InstanceState{
		// 						Name: aws.String(ec2.InstanceStateNameRunning),
		// 					},
		// 				},
		// 				region: &region{
		// 					instances: makeInstancesWithCatalog(
		// 						instanceMap{
		// 							"1": {
		// 								Instance: &ec2.Instance{
		// 									InstanceId: aws.String("1"),
		// 									State: &ec2.InstanceState{
		// 										Name: aws.String(ec2.InstanceStateNameRunning),
		// 									},
		// 								},
		// 							},
		// 						}),
		// 					services: connections{
		// 						ec2: mockEC2{
		// 							tierr: errors.New("terminate"),
		// 							dio:   &ec2.DescribeInstancesOutput{},
		// 						},
		// 					},
		// 				},
		// 			},
		// 		},
		// 	),
		// 	regionASG: &region{
		// 		name: "regionTest",
		// 		instances: makeInstancesWithCatalog(
		// 			instanceMap{
		// 				"1": {
		// 					Instance: &ec2.Instance{
		// 						InstanceId: aws.String("1"),
		// 						State: &ec2.InstanceState{
		// 							Name: aws.String(ec2.InstanceStateNameRunning),
		// 						},
		// 					},
		// 				},
		// 			}),
		// 		services: connections{
		// 			autoScaling: mockASG{dierr: nil},
		// 		},
		// 		conf: &Config{},
		// 	},
		// 	instanceID: aws.String("1"),
		// 	expected:   errors.New("terminate"),
		// },
		// {name: "errors during detach and terminate",
		// 	instancesASG: makeInstancesWithCatalog(
		// 		instanceMap{
		// 			"1": {
		// 				Instance: &ec2.Instance{
		// 					InstanceId: aws.String("1"),
		// 					State: &ec2.InstanceState{
		// 						Name: aws.String(ec2.InstanceStateNameRunning),
		// 					},
		// 				},
		// 				region: &region{
		// 					services: connections{
		// 						ec2: mockEC2{
		// 							tierr: errors.New("terminate"),
		// 							dio:   &ec2.DescribeInstancesOutput{},
		// 						},
		// 					},
		// 				},
		// 			},
		// 		},
		// 	),
		// 	regionASG: &region{
		// 		name: "regionTest",
		// 		instances: makeInstancesWithCatalog(
		// 			instanceMap{
		// 				"1": {
		// 					Instance: &ec2.Instance{
		// 						InstanceId: aws.String("1"),
		// 						State: &ec2.InstanceState{
		// 							Name: aws.String(ec2.InstanceStateNameRunning),
		// 						},
		// 					},
		// 				},
		// 			}),
		// 		services: connections{
		// 			autoScaling: mockASG{dierr: errors.New("detach")},
		// 		},
		// 		conf: &Config{},
		// 	},
		// 	instanceID: aws.String("1"),
		// 	expected:   errors.New("detach"),
		// },
	}

	for _, tt := range tests {
		fmt.Println(tt.name)
		t.Run(tt.name, func(t *testing.T) {
			a := autoScalingGroup{
				name:      "testASG",
				region:    tt.regionASG,
				instances: tt.instancesASG,
			}
			err := a.detachAndTerminateOnDemandInstance(tt.instanceID, false)
			CheckErrors(t, err, tt.expected)
		})
	}
}

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
// 						Instance: &ec2.Instance{
// 							InstanceId: aws.String("1"),
// 							State: &ec2.InstanceState{
// 								Name: aws.String(ec2.InstanceStateNameRunning),
// 							},
// 						},
// 						region: &region{
// 							services: connections{
// 								ec2: mockEC2{},
// 							},
// 						},
// 					},
// 				},
// 			),
// 			regionASG: &region{
// 				name: "regionTest",
// 				services: connections{
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
// 						Instance: &ec2.Instance{
// 							InstanceId: aws.String("1"),
// 							State: &ec2.InstanceState{
// 								Name: aws.String(ec2.InstanceStateNameRunning),
// 							},
// 						},
// 						region: &region{
// 							services: connections{
// 								ec2: mockEC2{},
// 							},
// 						},
// 					},
// 				},
// 			),
// 			regionASG: &region{
// 				name: "regionTest",
// 				services: connections{
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
				services: connections{
					autoScaling: mockASG{
						aierr: nil,
						dasio: &autoscaling.DescribeAutoScalingInstancesOutput{
							AutoScalingInstances: []*autoscaling.InstanceDetails{
								{
									LifecycleState: aws.String("InService"),
								},
							},
						},
					},
					ec2: mockEC2{
						dio: &ec2.DescribeInstancesOutput{
							Reservations: []*ec2.Reservation{
								{
									Instances: []*ec2.Instance{
										{
											State: &ec2.InstanceState{
												Name: aws.String("running"),
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
				services: connections{
					autoScaling: mockASG{
						aierr: error(awserr.New("ValidationError", "Error", errors.New("attach"))),
						// aierr: errors.New("attach"),
					},
					ec2: mockEC2{
						dio: &ec2.DescribeInstancesOutput{
							Reservations: []*ec2.Reservation{
								{
									Instances: []*ec2.Instance{
										{
											State: &ec2.InstanceState{
												Name: aws.String("running"),
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
			expected:   error(awserr.New("ValidationError", "Error", errors.New("attach"))),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := autoScalingGroup{
				name:   "testASG",
				region: tt.regionASG,
				Group: &autoscaling.Group{
					MaxSize:         aws.Int64(4),
					MinSize:         aws.Int64(2),
					DesiredCapacity: aws.Int64(3),
				},
			}
			err := a.attachSpotInstance(tt.instanceID, false)
			CheckErrors(t, err, tt.expected)
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
				LaunchConfiguration: &autoscaling.LaunchConfiguration{
					LaunchConfigurationName: aws.String("foo"),
				},
			},
			// nameLC: aws.String("foo"),
			// regionASG: &region{
			// 	services: connections{
			// 		autoScaling: mockASG{
			// 			dlco: &autoscaling.DescribeLaunchConfigurationsOutput{
			// 				LaunchConfigurations: []*autoscaling.LaunchConfiguration{
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
				LaunchConfiguration: &autoscaling.LaunchConfiguration{
					LaunchConfigurationName: aws.String("foo"),
				},
			},
		},
		{name: "nil launch configuration name",
			nameLC: nil,
			regionASG: &region{
				services: connections{
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
				services: connections{
					autoScaling: mockASG{
						dlcerr: nil,
						dlco: &autoscaling.DescribeLaunchConfigurationsOutput{
							LaunchConfigurations: []*autoscaling.LaunchConfiguration{
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
				LaunchConfiguration: &autoscaling.LaunchConfiguration{
					LaunchConfigurationName: aws.String("testLC"),
				},
			},
		},
		{name: "err during get launch configuration",
			nameLC: aws.String("testLC"),
			regionASG: &region{
				services: connections{
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
				Group: &autoscaling.Group{
					LaunchConfigurationName: tt.nameLC,
				},
			}
			lc, err := a.loadLaunchConfiguration()

			if !strings.Contains(err.Error(), tt.expectedErr.Error()) {
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
				LaunchTemplateVersion: &ec2.LaunchTemplateVersion{
					LaunchTemplateId: aws.String("foo"),
					VersionNumber:    aws.Int64(1),
				},
				Image: &ec2.Image{
					ImageId: aws.String("bar"),
				},
			},
			expectedErr: nil,
			expectedLT: &launchTemplate{
				LaunchTemplateVersion: &ec2.LaunchTemplateVersion{
					LaunchTemplateId: aws.String("foo"),
					VersionNumber:    aws.Int64(1),
				},
				Image: &ec2.Image{
					ImageId: aws.String("bar"),
				},
			},
		},
		{name: "nil launch template name",
			ltID:  nil,
			ltVer: nil,
			regionASG: &region{
				services: connections{
					ec2: mockEC2{
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
				services: connections{
					ec2: mockEC2{
						dltverr: nil,
						dltvo: &ec2.DescribeLaunchTemplateVersionsOutput{
							LaunchTemplateVersions: []*ec2.LaunchTemplateVersion{
								{
									LaunchTemplateId: aws.String("foo"),
									VersionNumber:    aws.Int64(1),
									LaunchTemplateData: &ec2.ResponseLaunchTemplateData{
										ImageId: aws.String("bar"),
									},
								},
							},
						},
						damierr: nil,
						damio: &ec2.DescribeImagesOutput{
							Images: []*ec2.Image{
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
				LaunchTemplateVersion: &ec2.LaunchTemplateVersion{
					LaunchTemplateId: aws.String("foo"),
					VersionNumber:    aws.Int64(1),
					LaunchTemplateData: &ec2.ResponseLaunchTemplateData{
						ImageId: aws.String("bar"),
					},
				},
				Image: &ec2.Image{
					ImageId: aws.String("bar"),
				},
			},
		},
		{name: "err during get launch template",
			ltID:  aws.String("foo"),
			ltVer: aws.Int64(1),
			regionASG: &region{
				services: connections{
					ec2: mockEC2{
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
				Group: &autoscaling.Group{
					LaunchTemplate: &autoscaling.LaunchTemplateSpecification{
						LaunchTemplateId: tt.ltID,
						Version:          aws.String("1"),
					},
				},
			}
			lt, err := a.loadLaunchTemplate()

			if !strings.Contains(err.Error(), tt.expectedErr.Error()) {
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
		maxSize   int64
		regionASG *region
		expected  error
	}{
		{name: "err during set autoscaling max size",
			maxSize: 4,
			regionASG: &region{
				services: connections{
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
				services: connections{
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
		ec2ASG            *autoscaling.Group
		regionInstances   *region
		expectedInstances instanceMap
	}{
		{name: "multiple instances to scan",
			regionInstances: &region{
				services: connections{
					ec2: &mockEC2{
						dio: &ec2.DescribeInstancesOutput{},
					},
				},
				instances: makeInstancesWithCatalog(
					instanceMap{
						"1": {
							Instance: &ec2.Instance{
								InstanceId: aws.String("1"),
								Placement: &ec2.Placement{
									AvailabilityZone: aws.String("az-1"),
								},
								InstanceLifecycle: aws.String(Spot),
							},
							typeInfo: instanceTypeInformation{
								pricing: prices{
									onDemand: 0.5,
									premium:  0.0,
									spot: map[string]float64{
										"az-1": 0.1,
										"az-2": 0.2,
										"az-3": 0.3,
									},
								},
							},
						},
						"2": {
							Instance: &ec2.Instance{
								InstanceId: aws.String("2"),
								Placement: &ec2.Placement{
									AvailabilityZone: aws.String("az-2"),
								},
							},
							typeInfo: instanceTypeInformation{
								pricing: prices{
									onDemand: 0.8,
									premium:  0.0,
									spot: map[string]float64{
										"az-1": 0.4,
										"az-2": 0.5,
										"az-3": 0.6,
									},
								},
							},
						},
					},
				),
			},
			ec2ASG: &autoscaling.Group{
				Instances: []*autoscaling.Instance{
					{
						InstanceId:     aws.String("1"),
						LifecycleState: aws.String("Running"),
					},
					{
						InstanceId:           aws.String("2"),
						ProtectedFromScaleIn: aws.Bool(true),
						AvailabilityZone:     aws.String("1a"),
						LifecycleState:       aws.String("Running"),
					},
					{
						InstanceId:     aws.String("3"),
						LifecycleState: aws.String("Running"),
					},
				},
			},
			expectedInstances: instanceMap{
				"1": {
					Instance: &ec2.Instance{
						InstanceId: aws.String("1"),
						Placement: &ec2.Placement{
							AvailabilityZone: aws.String("az-1"),
						},
						InstanceLifecycle: aws.String(Spot),
					},
					typeInfo: instanceTypeInformation{
						pricing: prices{
							onDemand: 0.5,
							spot: map[string]float64{
								"az-1": 0.1,
								"az-2": 0.2,
								"az-3": 0.3,
							},
						},
					},
					price:     0.1,
					protected: false,
				},
				"2": {
					Instance: &ec2.Instance{
						InstanceId: aws.String("2"),
						Placement: &ec2.Placement{
							AvailabilityZone: aws.String("az-2"),
						},
					},
					typeInfo: instanceTypeInformation{
						pricing: prices{
							onDemand: 0.8,
							spot: map[string]float64{
								"az-1": 0.4,
								"az-2": 0.5,
								"az-3": 0.6,
							},
						},
					},
					price:     0.8,
					protected: true,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &autoScalingGroup{
				name:   "testASG",
				Group:  tt.ec2ASG,
				region: tt.regionInstances,
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

func TestGetAnySpotInstance(t *testing.T) {
	tests := []struct {
		name         string
		asgInstances instances
		expected     []*instance
	}{
		{name: "ASG has no 'running' Spot instance",
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
					"spot-stopped": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("spot-stopped"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameStopped)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1a")},
							InstanceLifecycle: aws.String(Spot),
						},
					},
					"ondemand-stopped": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("ondemand-stopped"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameStopped)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
							InstanceLifecycle: aws.String(""),
						},
					},
					"ondemand-running": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("ondemand-running"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1c")},
							InstanceLifecycle: aws.String(""),
						},
						region: &region{
							services: connections{
								ec2: mockEC2{
									diao: &ec2.DescribeInstanceAttributeOutput{
										DisableApiTermination: &ec2.AttributeBooleanValue{
											Value: aws.Bool(false),
										},
									},
								},
							},
						},
					},
				},
			),
			expected: []*instance{},
		},
		{name: "ASG has one 'running' Spot instance",
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
					"spot-stopped": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("spot-stopped"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameStopped)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1a")},
							InstanceLifecycle: aws.String(Spot),
						},
					},
					"spot-running": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("spot-running"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
							InstanceLifecycle: aws.String(Spot),
						},
						region: &region{
							services: connections{
								ec2: mockEC2{
									diao: &ec2.DescribeInstanceAttributeOutput{
										DisableApiTermination: &ec2.AttributeBooleanValue{
											Value: aws.Bool(false),
										},
									},
								},
							},
						},
					},
					"ondemand-stopped": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("ondemand-stopped"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameStopped)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1c")},
							InstanceLifecycle: aws.String(""),
						},
					},
					"ondemand-running": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("ondemand-running"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
							InstanceLifecycle: aws.String(""),
						},
						region: &region{
							services: connections{
								ec2: mockEC2{
									diao: &ec2.DescribeInstanceAttributeOutput{
										DisableApiTermination: &ec2.AttributeBooleanValue{
											Value: aws.Bool(false),
										},
									},
								},
							},
						},
					},
				},
			),
			expected: []*instance{{
				Instance: &ec2.Instance{
					InstanceId:        aws.String("spot-running"),
					State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
					Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
					InstanceLifecycle: aws.String(Spot),
				},
				region: &region{
					services: connections{
						ec2: mockEC2{
							diao: &ec2.DescribeInstanceAttributeOutput{
								DisableApiTermination: &ec2.AttributeBooleanValue{
									Value: aws.Bool(false),
								},
							},
						},
					},
				},
			},
			},
		},
		{name: "ASG has multiple 'running' Spot instances",
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
					"spot-running1": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("spot-running1"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1a")},
							InstanceLifecycle: aws.String(Spot),
						}, region: &region{
							services: connections{
								ec2: mockEC2{
									diao: &ec2.DescribeInstanceAttributeOutput{
										DisableApiTermination: &ec2.AttributeBooleanValue{
											Value: aws.Bool(false),
										},
									},
								},
							},
						},
					},
					"spot-running2": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("spot-running2"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
							InstanceLifecycle: aws.String(Spot),
						},
						region: &region{
							services: connections{
								ec2: mockEC2{
									diao: &ec2.DescribeInstanceAttributeOutput{
										DisableApiTermination: &ec2.AttributeBooleanValue{
											Value: aws.Bool(false),
										},
									},
								},
							},
						},
					},
					"ondemand-stopped": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("ondemand-stopped"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameStopped)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1c")},
							InstanceLifecycle: aws.String(""),
						},
					},
					"ondemand-running": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("ondemand-running"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
							InstanceLifecycle: aws.String(""),
						}, region: &region{
							services: connections{
								ec2: mockEC2{
									diao: &ec2.DescribeInstanceAttributeOutput{
										DisableApiTermination: &ec2.AttributeBooleanValue{
											Value: aws.Bool(false),
										},
									},
								},
							},
						},
					},
				},
			),
			expected: []*instance{
				{
					Instance: &ec2.Instance{
						InstanceId:        aws.String("spot-running1"),
						State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
						Placement:         &ec2.Placement{AvailabilityZone: aws.String("1a")},
						InstanceLifecycle: aws.String(Spot),
					}, region: &region{
						services: connections{
							ec2: mockEC2{
								diao: &ec2.DescribeInstanceAttributeOutput{
									DisableApiTermination: &ec2.AttributeBooleanValue{
										Value: aws.Bool(false),
									},
								},
							},
						},
					},
				},
				{
					Instance: &ec2.Instance{
						InstanceId:        aws.String("spot-running2"),
						State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
						Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
						InstanceLifecycle: aws.String(Spot),
					},
					region: &region{
						services: connections{
							ec2: mockEC2{
								diao: &ec2.DescribeInstanceAttributeOutput{
									DisableApiTermination: &ec2.AttributeBooleanValue{
										Value: aws.Bool(false),
									},
								},
							},
						},
					},
				},
			},
		},
		{name: "ASG has no instance at all",
			asgInstances: makeInstancesWithCatalog(instanceMap{}),
			expected:     []*instance{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var found = false

			a := &autoScalingGroup{
				instances: tt.asgInstances,
			}
			returnedInstance := a.getAnySpotInstance()
			if len(tt.expected) == 0 && returnedInstance != nil {
				t.Errorf("getAnySpotInstance received: %+v, expected: nil",
					returnedInstance)
			} else if len(tt.expected) != 0 {
				for _, i := range tt.expected {
					if reflect.DeepEqual(returnedInstance, i) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("getAnySpotInstance received: %+v, expected to be in: %+v",
						returnedInstance,
						tt.expected)
				}
			}
		})
	}
}

func TestReplaceOnDemandInstanceWithSpot(t *testing.T) {
	tests := []struct {
		name     string
		asg      *autoScalingGroup
		spotID   string
		expected error
	}{
		{name: "OnDemand is replaced by spot instance - min/max/des identical",
			spotID:   "spot-running",
			expected: nil,
			asg: &autoScalingGroup{
				name: "test-asg",
				Group: &autoscaling.Group{
					MaxSize:         aws.Int64(2),
					MinSize:         aws.Int64(2),
					DesiredCapacity: aws.Int64(2),
				},
				instances: makeInstancesWithCatalog(
					instanceMap{
						"ondemand-stopped": {
							Instance: &ec2.Instance{
								InstanceId:        aws.String("ondemand-stopped"),
								State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameStopped)},
								Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
								InstanceLifecycle: aws.String(""),
							},
							region: &region{
								services: connections{
									ec2: &mockEC2{
										tio:   nil,
										tierr: nil,
									},
									autoScaling: &mockASG{
										aio:       nil,
										aierr:     nil,
										tiiasgo:   nil,
										tiiasgerr: nil,
									},
								},
							},
						},
						"ondemand-running": {
							Instance: &ec2.Instance{
								InstanceId:        aws.String("ondemand-running"),
								State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
								Placement:         &ec2.Placement{AvailabilityZone: aws.String("1a")},
								InstanceLifecycle: aws.String(""),
							},
							region: &region{
								services: connections{
									ec2: &mockEC2{
										tio:   nil,
										tierr: nil,
										diao: &ec2.DescribeInstanceAttributeOutput{
											DisableApiTermination: &ec2.AttributeBooleanValue{
												Value: aws.Bool(true),
											},
										},
										diaerr: nil,
									},
								},
							},
						},
					},
				),
				region: &region{
					name: "test-region",
					conf: &Config{
						FinalRecap: make(map[string][]string),
					},
					services: connections{
						autoScaling: &mockASG{
							uasgo:     nil,
							uasgerr:   nil,
							dio:       nil,
							dierr:     nil,
							tiiasgo:   nil,
							tiiasgerr: nil,
						},
						ec2: &mockEC2{
							tio:   nil,
							tierr: nil,
						},
						sqs: &mockSQS{
							smo:   nil,
							smerr: nil,
						},
					},
					instances: makeInstancesWithCatalog(
						instanceMap{
							"spot-running": {
								Instance: &ec2.Instance{
									InstanceId:        aws.String("spot-running"),
									State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameStopped)},
									Placement:         &ec2.Placement{AvailabilityZone: aws.String("1a")},
									InstanceLifecycle: aws.String(Spot),
								},
								region: &region{
									services: connections{
										ec2: &mockEC2{
											tio:   nil,
											tierr: nil,
											diao: &ec2.DescribeInstanceAttributeOutput{
												DisableApiTermination: &ec2.AttributeBooleanValue{
													Value: aws.Bool(true),
												},
											},
											diaerr: nil,
										},
									},
								},
							},
							"ondemand-stopped": {
								Instance: &ec2.Instance{
									InstanceId:        aws.String("ondemand-stopped"),
									State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameStopped)},
									Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
									InstanceLifecycle: aws.String(""),
								},
								region: &region{
									services: connections{
										ec2: &mockEC2{
											tio:   nil,
											tierr: nil,
										},
									},
								},
							},
							"ondemand-running": {
								Instance: &ec2.Instance{
									InstanceId:        aws.String("ondemand-running"),
									State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
									Placement:         &ec2.Placement{AvailabilityZone: aws.String("1a")},
									InstanceLifecycle: aws.String(""),
								},
								region: &region{
									services: connections{
										ec2: &mockEC2{
											tio:   nil,
											tierr: nil,
										},
									},
								},
							},
						},
					),
				},
			},
		},
		{name: "OnDemand is replaced by spot instance - min/max/des different",
			spotID:   "spot-running",
			expected: nil,
			asg: &autoScalingGroup{
				name: "test-asg",
				Group: &autoscaling.Group{
					MaxSize:         aws.Int64(4),
					MinSize:         aws.Int64(1),
					DesiredCapacity: aws.Int64(2),
				},
				instances: makeInstancesWithCatalog(
					instanceMap{
						"ondemand-running": {
							Instance: &ec2.Instance{
								InstanceId:        aws.String("ondemand-running"),
								State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
								Placement:         &ec2.Placement{AvailabilityZone: aws.String("1a")},
								InstanceLifecycle: aws.String(""),
							},
							region: &region{
								services: connections{
									ec2: &mockEC2{
										tio:   nil,
										tierr: nil,
										diao: &ec2.DescribeInstanceAttributeOutput{
											DisableApiTermination: &ec2.AttributeBooleanValue{
												Value: aws.Bool(true),
											},
										},
										diaerr: nil,
									},
								},
							},
						},
					},
				),
				region: &region{
					name: "test-region",
					conf: &Config{
						FinalRecap: make(map[string][]string),
					},
					services: connections{
						autoScaling: &mockASG{
							uasgo:     nil,
							uasgerr:   nil,
							dio:       nil,
							dierr:     nil,
							tiiasgo:   nil,
							tiiasgerr: nil,
						},
						ec2: &mockEC2{
							tio:   nil,
							tierr: nil,
						},
						sqs: &mockSQS{
							smo:   nil,
							smerr: nil,
						},
					},
					instances: makeInstancesWithCatalog(
						instanceMap{
							"spot-running": {
								Instance: &ec2.Instance{
									InstanceId:        aws.String("spot-running"),
									State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameStopped)},
									Placement:         &ec2.Placement{AvailabilityZone: aws.String("1a")},
									InstanceLifecycle: aws.String(Spot),
								},
								region: &region{
									services: connections{
										ec2: &mockEC2{
											tio:   nil,
											tierr: nil,
										},
									},
								},
							},
						},
					),
				},
			},
		},
		{name: "no spot instances found in region",
			spotID:   "spot-not-found",
			expected: errors.New("couldn't find spot instance to use"),
			asg: &autoScalingGroup{
				name: "test-asg",
				Group: &autoscaling.Group{
					MaxSize:         aws.Int64(4),
					MinSize:         aws.Int64(2),
					DesiredCapacity: aws.Int64(2),
				},
				region: &region{
					name: "test-region",
					conf: &Config{},
					services: connections{
						autoScaling: &mockASG{
							uasgo:     nil,
							uasgerr:   nil,
							dio:       nil,
							dierr:     nil,
							tiiasgo:   nil,
							tiiasgerr: nil,
						},
					},
					instances: makeInstancesWithCatalog(
						instanceMap{
							"spot-running": {
								Instance: &ec2.Instance{
									InstanceId:        aws.String("spot-running"),
									State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameStopped)},
									Placement:         &ec2.Placement{AvailabilityZone: aws.String("1a")},
									InstanceLifecycle: aws.String(Spot),
								},
							},
							"ondemand-stopped": {
								Instance: &ec2.Instance{
									InstanceId:        aws.String("ondemand-stopped"),
									State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameStopped)},
									Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
									InstanceLifecycle: aws.String(""),
								},
							},
							"ondemand-running": {
								Instance: &ec2.Instance{
									InstanceId:        aws.String("ondemand-running"),
									State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
									Placement:         &ec2.Placement{AvailabilityZone: aws.String("1a")},
									InstanceLifecycle: aws.String(""),
								},
							},
						},
					),
				},
			},
		},
		{name: "no OnDemand instances found in asg",
			spotID:   "spot-running",
			expected: errors.New("couldn't find target instance for spot-running"),
			asg: &autoScalingGroup{
				name: "test-asg",
				Group: &autoscaling.Group{
					MaxSize:         aws.Int64(4),
					MinSize:         aws.Int64(1),
					DesiredCapacity: aws.Int64(2),
				},
				instances: makeInstances(),
				region: &region{
					name: "test-region",
					conf: &Config{
						FinalRecap: make(map[string][]string),
					},
					services: connections{
						autoScaling: &mockASG{
							uasgo:     nil,
							uasgerr:   nil,
							dio:       nil,
							dierr:     nil,
							tiiasgo:   nil,
							tiiasgerr: nil,
						},
						sqs: &mockSQS{
							smo:   nil,
							smerr: nil,
						},
					},
					instances: makeInstancesWithCatalog(
						instanceMap{
							"spot-running": {
								Instance: &ec2.Instance{
									InstanceId:        aws.String("spot-running"),
									State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
									Placement:         &ec2.Placement{AvailabilityZone: aws.String("1z")},
									InstanceLifecycle: aws.String(Spot),
								},
								region: &region{
									services: connections{
										ec2: &mockEC2{
											tio:   nil,
											tierr: nil,
										},
									},
								},
							},
						},
					),
				},
			},
		},

		{name: "found OnDemand instance in asg, without lifecycle hooks",
			spotID:   "spot-running",
			expected: errors.New("couldn't find target instance for spot-running"),
			asg: &autoScalingGroup{
				name: "test-asg",
				Group: &autoscaling.Group{
					MaxSize:         aws.Int64(4),
					MinSize:         aws.Int64(1),
					DesiredCapacity: aws.Int64(2),
				},
				instances: makeInstancesWithCatalog(instanceMap{
					"on-demand-running": {
						Instance: &ec2.Instance{
							InstanceId: aws.String("on-demand-running"),
							State:      &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:  &ec2.Placement{AvailabilityZone: aws.String("1z")},
						},
						region: &region{
							services: connections{
								ec2: &mockEC2{
									tio:   nil,
									tierr: nil,
								},
							},
						},
					},
				}),
				region: &region{
					name: "test-region",
					conf: &Config{
						FinalRecap: make(map[string][]string),
					},
					services: connections{
						autoScaling: &mockASG{
							uasgo:     nil,
							uasgerr:   nil,
							dio:       nil,
							dierr:     nil,
							tiiasgo:   nil,
							tiiasgerr: nil,
							dlho: &autoscaling.DescribeLifecycleHooksOutput{
								LifecycleHooks: []*autoscaling.LifecycleHook{},
							},
							dasio: &autoscaling.DescribeAutoScalingInstancesOutput{
								AutoScalingInstances: []*autoscaling.InstanceDetails{
									{
										LifecycleState: aws.String("InService"),
									},
								},
							},
						},
						ec2: &mockEC2{},
						sqs: &mockSQS{
							smo:   nil,
							smerr: nil,
						},
					},
					instances: makeInstancesWithCatalog(
						instanceMap{
							"on-demand-running": {
								Instance: &ec2.Instance{
									InstanceId: aws.String("on-demand-running"),
									State:      &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
									Placement:  &ec2.Placement{AvailabilityZone: aws.String("1z")},
								},
								region: &region{
									services: connections{
										ec2: &mockEC2{
											tio:   nil,
											tierr: nil,
										},
									},
								},
							},

							"spot-running": {
								Instance: &ec2.Instance{
									InstanceId:        aws.String("spot-running"),
									State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
									Placement:         &ec2.Placement{AvailabilityZone: aws.String("1z")},
									InstanceLifecycle: aws.String(Spot),
								},
								region: &region{
									services: connections{
										ec2: &mockEC2{
											tio:    nil,
											tierr:  nil,
											diaerr: nil,
											diao: &ec2.DescribeInstanceAttributeOutput{
												DisableApiTermination: &ec2.AttributeBooleanValue{
													Value: aws.Bool(false),
												},
											},
										},
									},
								},
							},
						},
					),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fmt.Println(tt.name)
			returned := tt.asg.replaceOnDemandInstanceWithSpot(tt.spotID)
			CheckErrors(t, returned, tt.expected)
		})
		t.Run(tt.name+"-detach-method", func(t *testing.T) {
			fmt.Println(tt.name)
			tt.asg.config.TerminationMethod = "detach"
			returned := tt.asg.replaceOnDemandInstanceWithSpot(tt.spotID)
			CheckErrors(t, returned, tt.expected)
		})
	}
}

func TestGetAllowedInstanceTypes(t *testing.T) {
	tests := []struct {
		name         string
		expected     []string
		instanceInfo *instance
		asg          *autoScalingGroup
		asgTags      []*autoscaling.TagDescription
	}{
		{name: "Single Type Tag c2.xlarge",
			expected: []string{"c2.xlarge"},
			instanceInfo: &instance{
				typeInfo: instanceTypeInformation{
					instanceType: "typeX",
				},
				region: &region{},
			},
			asg: &autoScalingGroup{
				name: "TestASG",
				region: &region{
					conf: &Config{
						AutoScalingConfig: AutoScalingConfig{
							AllowedInstanceTypes: "",
						}},
				},
				Group: &autoscaling.Group{
					DesiredCapacity: aws.Int64(4),
				},
			},
			asgTags: []*autoscaling.TagDescription{
				{
					Key:   aws.String("autospotting_allowed_instance_types"),
					Value: aws.String("c2.xlarge"),
				},
			},
		},
		{name: "Single Type Cmd Line c2.xlarge",
			expected: []string{"c2.xlarge"},
			instanceInfo: &instance{
				typeInfo: instanceTypeInformation{
					instanceType: "typeX",
				},
				region: &region{},
			},
			asg: &autoScalingGroup{
				name: "TestASG",
				region: &region{
					conf: &Config{
						AutoScalingConfig: AutoScalingConfig{
							AllowedInstanceTypes: "c2.xlarge",
						}},
				},
				Group: &autoscaling.Group{
					DesiredCapacity: aws.Int64(4),
				},
			},
			asgTags: []*autoscaling.TagDescription{},
		},
		{name: "Single Type from Base c2.xlarge",
			expected: []string{"c2.xlarge"},
			instanceInfo: &instance{
				typeInfo: instanceTypeInformation{
					instanceType: "c2.xlarge",
				},
				region: &region{},
			},
			asg: &autoScalingGroup{
				name: "TestASG",
				region: &region{
					conf: &Config{
						AutoScalingConfig: AutoScalingConfig{
							AllowedInstanceTypes: "current",
						}},
				},
				Group: &autoscaling.Group{
					DesiredCapacity: aws.Int64(4),
				},
			},
			asgTags: []*autoscaling.TagDescription{},
		},
		{name: "ASG precedence on command line",
			expected: []string{"c4.4xlarge"},
			instanceInfo: &instance{
				typeInfo: instanceTypeInformation{
					instanceType: "typeX",
				},
				region: &region{},
			},
			asg: &autoScalingGroup{
				name: "TestASG",
				region: &region{
					conf: &Config{
						AutoScalingConfig: AutoScalingConfig{
							AllowedInstanceTypes: "c2.xlarge",
						}},
				},
				Group: &autoscaling.Group{
					DesiredCapacity: aws.Int64(4),
				},
			},
			asgTags: []*autoscaling.TagDescription{
				{
					Key:   aws.String("autospotting_allowed_instance_types"),
					Value: aws.String("c4.4xlarge"),
				},
			},
		},
		{name: "ASG 'current' precedence on command line",
			expected: []string{"c2.xlarge"},
			instanceInfo: &instance{
				typeInfo: instanceTypeInformation{
					instanceType: "c2.xlarge",
				},
				region: &region{},
			},
			asg: &autoScalingGroup{
				name: "TestASG",
				region: &region{
					conf: &Config{
						AutoScalingConfig: AutoScalingConfig{
							AllowedInstanceTypes: "c4.xlarge",
						}},
				},
				Group: &autoscaling.Group{
					DesiredCapacity: aws.Int64(4),
				},
			},
			asgTags: []*autoscaling.TagDescription{
				{
					Key:   aws.String("autospotting_allowed_instance_types"),
					Value: aws.String("current"),
				},
			},
		},
		{name: "Comma separated list",
			expected: []string{"c2.xlarge", "t2.medium", "c3.small"},
			instanceInfo: &instance{
				typeInfo: instanceTypeInformation{
					instanceType: "typeX",
				},
				region: &region{},
			},
			asg: &autoScalingGroup{
				name: "TestASG",
				region: &region{
					conf: &Config{
						AutoScalingConfig: AutoScalingConfig{
							AllowedInstanceTypes: "c2.xlarge,t2.medium,c3.small",
						}},
				},
				Group: &autoscaling.Group{
					DesiredCapacity: aws.Int64(4),
				},
			},
			asgTags: []*autoscaling.TagDescription{},
		},
		{name: "Space separated list",
			expected: []string{"c2.xlarge", "t2.medium", "c3.small"},
			instanceInfo: &instance{
				typeInfo: instanceTypeInformation{
					instanceType: "typeX",
				},
				region: &region{},
			},
			asg: &autoScalingGroup{
				name: "TestASG",
				region: &region{
					conf: &Config{
						AutoScalingConfig: AutoScalingConfig{
							AllowedInstanceTypes: "c2.xlarge t2.medium c3.small",
						}},
				},
				Group: &autoscaling.Group{
					DesiredCapacity: aws.Int64(4),
				},
			},
			asgTags: []*autoscaling.TagDescription{},
		},
		{name: "No empty elements in comma separated list",
			expected: []string{"c2.xlarge", "t2.medium", "c3.small"},
			instanceInfo: &instance{
				typeInfo: instanceTypeInformation{
					instanceType: "typeX",
				},
				region: &region{},
			},
			asg: &autoScalingGroup{
				name: "TestASG",
				region: &region{
					conf: &Config{
						AutoScalingConfig: AutoScalingConfig{
							AllowedInstanceTypes: ",,c2.xlarge,,,t2.medium,c3.small,,",
						}},
				},
				Group: &autoscaling.Group{
					DesiredCapacity: aws.Int64(4),
				},
			},
			asgTags: []*autoscaling.TagDescription{},
		},
		{name: "No empty elements in space separated list",
			expected: []string{"c2.xlarge", "t2.medium", "c3.small"},
			instanceInfo: &instance{
				typeInfo: instanceTypeInformation{
					instanceType: "typeX",
				},
				region: &region{},
			},
			asg: &autoScalingGroup{
				name: "TestASG",
				region: &region{
					conf: &Config{
						AutoScalingConfig: AutoScalingConfig{
							AllowedInstanceTypes: "   c2.xlarge    t2.medium  c3.small  ",
						}},
				},
				Group: &autoscaling.Group{
					DesiredCapacity: aws.Int64(4),
				},
			},
			asgTags: []*autoscaling.TagDescription{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := tt.asg
			a.Tags = tt.asgTags
			baseInstance := tt.instanceInfo
			allowed := a.getAllowedInstanceTypes(baseInstance)
			if !reflect.DeepEqual(allowed, tt.expected) {
				t.Errorf("Allowed Instance Types does not match, received: %+v, expected: %+v",
					allowed, tt.expected)
			}
		})
	}
}

func TestGetDisallowedInstanceTypes(t *testing.T) {
	tests := []struct {
		name         string
		expected     []string
		instanceInfo *instance
		asg          *autoScalingGroup
		asgtags      []*autoscaling.TagDescription
	}{
		{name: "Single Type Tag c2.xlarge",
			expected: []string{"c2.xlarge"},
			instanceInfo: &instance{
				typeInfo: instanceTypeInformation{
					instanceType: "typeX",
				},
				region: &region{},
			},
			asg: &autoScalingGroup{
				name: "TestASG",
				region: &region{
					conf: &Config{
						AutoScalingConfig: AutoScalingConfig{
							DisallowedInstanceTypes: "",
						}},
				},
				Group: &autoscaling.Group{
					DesiredCapacity: aws.Int64(4),
				},
			},
			asgtags: []*autoscaling.TagDescription{
				{
					Key:   aws.String("autospotting_disallowed_instance_types"),
					Value: aws.String("c2.xlarge"),
				},
			},
		},
		{name: "Single Type Cmd Line c2.xlarge",
			expected: []string{"c2.xlarge"},
			instanceInfo: &instance{
				typeInfo: instanceTypeInformation{
					instanceType: "typeX",
				},
				region: &region{},
			},
			asg: &autoScalingGroup{
				name: "TestASG",
				region: &region{
					conf: &Config{
						AutoScalingConfig: AutoScalingConfig{
							DisallowedInstanceTypes: "c2.xlarge",
						}},
				},
				Group: &autoscaling.Group{
					DesiredCapacity: aws.Int64(4),
				},
			},
			asgtags: []*autoscaling.TagDescription{},
		},
		{name: "ASG precedence on command line",
			expected: []string{"c4.4xlarge"},
			instanceInfo: &instance{
				typeInfo: instanceTypeInformation{
					instanceType: "typeX",
				},
				region: &region{},
			},
			asg: &autoScalingGroup{
				name: "TestASG",
				region: &region{
					conf: &Config{
						AutoScalingConfig: AutoScalingConfig{
							DisallowedInstanceTypes: "c2.xlarge",
						}},
				},
				Group: &autoscaling.Group{
					DesiredCapacity: aws.Int64(4),
				},
			},
			asgtags: []*autoscaling.TagDescription{
				{
					Key:   aws.String("autospotting_disallowed_instance_types"),
					Value: aws.String("c4.4xlarge"),
				},
			},
		},
		{name: "Comma separated list",
			expected: []string{"c2.xlarge", "t2.medium", "c3.small"},
			instanceInfo: &instance{
				typeInfo: instanceTypeInformation{
					instanceType: "typeX",
				},
				region: &region{},
			},
			asg: &autoScalingGroup{
				name: "TestASG",
				region: &region{
					conf: &Config{
						AutoScalingConfig: AutoScalingConfig{
							DisallowedInstanceTypes: "c2.xlarge,t2.medium,c3.small",
						}},
				},
				Group: &autoscaling.Group{
					DesiredCapacity: aws.Int64(4),
				},
			},
			asgtags: []*autoscaling.TagDescription{},
		},
		{name: "Space separated list",
			expected: []string{"c2.xlarge", "t2.medium", "c3.small"},
			instanceInfo: &instance{
				typeInfo: instanceTypeInformation{
					instanceType: "typeX",
				},
				region: &region{},
			},
			asg: &autoScalingGroup{
				name: "TestASG",
				region: &region{
					conf: &Config{
						AutoScalingConfig: AutoScalingConfig{
							DisallowedInstanceTypes: "c2.xlarge t2.medium c3.small",
						}},
				},
				Group: &autoscaling.Group{
					DesiredCapacity: aws.Int64(4),
				},
			},
			asgtags: []*autoscaling.TagDescription{},
		},
		{name: "No empty elements in comma separated list",
			expected: []string{"c2.xlarge", "t2.medium", "c3.small"},
			instanceInfo: &instance{
				typeInfo: instanceTypeInformation{
					instanceType: "typeX",
				},
				region: &region{},
			},
			asg: &autoScalingGroup{
				name: "TestASG",
				region: &region{
					conf: &Config{
						AutoScalingConfig: AutoScalingConfig{
							DisallowedInstanceTypes: ",,c2.xlarge,,,t2.medium,c3.small,,",
						}},
				},
				Group: &autoscaling.Group{
					DesiredCapacity: aws.Int64(4),
				},
			},
			asgtags: []*autoscaling.TagDescription{},
		},
		{name: "No empty elements in space separated list",
			expected: []string{"c2.xlarge", "t2.medium", "c3.small"},
			instanceInfo: &instance{
				typeInfo: instanceTypeInformation{
					instanceType: "typeX",
				},
				region: &region{},
			},
			asg: &autoScalingGroup{
				name: "TestASG",
				region: &region{
					conf: &Config{
						AutoScalingConfig: AutoScalingConfig{
							DisallowedInstanceTypes: "   c2.xlarge    t2.medium  c3.small  ",
						}},
				},
				Group: &autoscaling.Group{
					DesiredCapacity: aws.Int64(4),
				},
			},
			asgtags: []*autoscaling.TagDescription{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := tt.asg
			a.Tags = tt.asgtags
			baseInstance := tt.instanceInfo
			allowed := a.getDisallowedInstanceTypes(baseInstance)
			if !reflect.DeepEqual(allowed, tt.expected) {
				t.Errorf("Disallowed Instance Types does not match, received: %+v, expected: %+v",
					allowed, tt.expected)
			}
		})
	}
}

func Test_autoScalingGroup_hasMemberInstance(t *testing.T) {

	tests := []struct {
		name  string
		Group *autoscaling.Group
		inst  *instance
		want  bool
	}{
		{
			name: "has member",
			Group: &autoscaling.Group{
				Instances: []*autoscaling.Instance{
					{InstanceId: aws.String("foo")},
					{InstanceId: aws.String("bar")},
					{InstanceId: aws.String("baz")},
				},
			},
			inst: &instance{
				asg:      &autoScalingGroup{},
				Instance: &ec2.Instance{InstanceId: aws.String("bar")},
			},
			want: true,
		},
		{
			name: "doesn't have member",
			Group: &autoscaling.Group{
				Instances: []*autoscaling.Instance{
					{InstanceId: aws.String("foo")},
					{InstanceId: aws.String("bar")},
					{InstanceId: aws.String("baz")},
				},
			},
			inst: &instance{
				asg:      &autoScalingGroup{},
				Instance: &ec2.Instance{InstanceId: aws.String("bazinga")},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &autoScalingGroup{
				Group: tt.Group,
			}
			if got := a.hasMemberInstance(tt.inst); got != tt.want {
				t.Errorf("autoScalingGroup.hasMemberInstance() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_autoScalingGroup_getAnyUnprotectedOnDemandInstance(t *testing.T) {
	tests := []struct {
		name         string
		asgInstances instances

		want *instance
	}{
		{
			name: "ASG has unprotected and protected from scale-in instance",
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
					"ondemand-unprotected": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("ondemand-unprotected"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1c")},
							InstanceLifecycle: aws.String(""),
						},
						asg: &autoScalingGroup{
							Group: &autoscaling.Group{
								Instances: []*autoscaling.Instance{
									{
										InstanceId:           aws.String("ondemand-unprotected"),
										ProtectedFromScaleIn: aws.Bool(false),
									},
								},
							},
						},
						region: &region{
							services: connections{
								ec2: mockEC2{
									diao: &ec2.DescribeInstanceAttributeOutput{
										// not protected from termination
										DisableApiTermination: &ec2.AttributeBooleanValue{
											Value: aws.Bool(false),
										},
									},
								},
							},
						},
					},
					"ondemand-protected-scale-in": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("ondemand-protected-scale-in"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
							InstanceLifecycle: aws.String(""),
						},
						asg: &autoScalingGroup{
							Group: &autoscaling.Group{
								Instances: []*autoscaling.Instance{
									{
										InstanceId:           aws.String("ondemand-protected-scale-in"),
										ProtectedFromScaleIn: aws.Bool(true),
										AvailabilityZone:     aws.String("1b"),
									},
								},
							},
						},
						region: &region{
							services: connections{
								ec2: mockEC2{
									diao: &ec2.DescribeInstanceAttributeOutput{
										// not protected from termination
										DisableApiTermination: &ec2.AttributeBooleanValue{
											Value: aws.Bool(false),
										},
									},
								},
							},
						},
					},
				},
			),
			want: &instance{
				Instance: &ec2.Instance{
					InstanceId:        aws.String("ondemand-unprotected"),
					State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
					Placement:         &ec2.Placement{AvailabilityZone: aws.String("1c")},
					InstanceLifecycle: aws.String(""),
				},
				asg: &autoScalingGroup{
					Group: &autoscaling.Group{
						Instances: []*autoscaling.Instance{
							{
								InstanceId:           aws.String("ondemand-unprotected"),
								ProtectedFromScaleIn: aws.Bool(false),
							},
						},
					},
				},
				region: &region{
					services: connections{
						ec2: mockEC2{
							diao: &ec2.DescribeInstanceAttributeOutput{
								// not protected from termination
								DisableApiTermination: &ec2.AttributeBooleanValue{
									Value: aws.Bool(false),
								},
							},
						},
					},
				},
			},
		},

		{
			name: "ASG has unprotected and protected from termination instance",
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
					"ondemand-unprotected": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("ondemand-unprotected"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1c")},
							InstanceLifecycle: aws.String(""),
						},
						asg: &autoScalingGroup{
							Group: &autoscaling.Group{
								Instances: []*autoscaling.Instance{
									{
										InstanceId:           aws.String("ondemand-unprotected"),
										ProtectedFromScaleIn: aws.Bool(false),
									},
								},
							},
						},
						region: &region{
							services: connections{
								ec2: mockEC2{
									diao: &ec2.DescribeInstanceAttributeOutput{
										// not protected from termination
										DisableApiTermination: &ec2.AttributeBooleanValue{
											Value: aws.Bool(false),
										},
									},
								},
							},
						},
					},
					"ondemand-protected-termination": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("ondemand-protected-termination"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
							InstanceLifecycle: aws.String(""),
						},
						asg: &autoScalingGroup{
							Group: &autoscaling.Group{
								Instances: []*autoscaling.Instance{
									{
										InstanceId:           aws.String("ondemand-protected-termination"),
										ProtectedFromScaleIn: aws.Bool(false),
									},
								},
							},
						},
						region: &region{
							services: connections{
								ec2: mockEC2{
									diao: &ec2.DescribeInstanceAttributeOutput{
										// not protected from termination
										DisableApiTermination: &ec2.AttributeBooleanValue{
											Value: aws.Bool(true),
										},
									},
								},
							},
						},
					},
				},
			),
			want: &instance{
				Instance: &ec2.Instance{
					InstanceId:        aws.String("ondemand-unprotected"),
					State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
					Placement:         &ec2.Placement{AvailabilityZone: aws.String("1c")},
					InstanceLifecycle: aws.String(""),
				},
				asg: &autoScalingGroup{
					Group: &autoscaling.Group{
						Instances: []*autoscaling.Instance{
							{
								InstanceId:           aws.String("ondemand-unprotected"),
								ProtectedFromScaleIn: aws.Bool(false),
							},
						},
					},
				},
				region: &region{
					services: connections{
						ec2: mockEC2{
							diao: &ec2.DescribeInstanceAttributeOutput{
								// not protected from termination
								DisableApiTermination: &ec2.AttributeBooleanValue{
									Value: aws.Bool(false),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "ASG has no unprotected instances in AZ",
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
					"ondemand-protected-scale-in": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("ondemand-protected-scale-in"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1c")},
							InstanceLifecycle: aws.String(""),
						},
						asg: &autoScalingGroup{
							Group: &autoscaling.Group{
								Instances: []*autoscaling.Instance{
									{
										InstanceId:           aws.String("ondemand-protected-scale-in"),
										ProtectedFromScaleIn: aws.Bool(true),
										AvailabilityZone:     aws.String("1c"),
									},
								},
							},
						},
						region: &region{
							services: connections{
								ec2: mockEC2{
									diao: &ec2.DescribeInstanceAttributeOutput{
										DisableApiTermination: &ec2.AttributeBooleanValue{
											Value: aws.Bool(false),
										},
									},
								},
							},
						},
					},
					"ondemand-protected-termination": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("ondemand-protected-termination"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
							InstanceLifecycle: aws.String(""),
						},
						asg: &autoScalingGroup{
							Group: &autoscaling.Group{
								Instances: []*autoscaling.Instance{
									{
										InstanceId:           aws.String("ondemand-protected-termination"),
										ProtectedFromScaleIn: aws.Bool(false),
									},
								},
							},
						},
						region: &region{
							services: connections{
								ec2: mockEC2{
									diao: &ec2.DescribeInstanceAttributeOutput{
										DisableApiTermination: &ec2.AttributeBooleanValue{
											Value: aws.Bool(true),
										},
									},
								},
							},
						},
					},
				},
			),
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &autoScalingGroup{
				name:      tt.name,
				instances: tt.asgInstances,
			}
			if got := a.getAnyUnprotectedOnDemandInstance(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("autoScalingGroup.getAnyUnprotectedOnDemandInstance() = %v, want %v", got, tt.want)
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
			name:      "empty group",
			instances: makeInstancesWithCatalog(instanceMap{}),
			wantNeed:  true,
			wantTotal: 0,
		},
		{
			name:        "group with more on-demand instances than enough",
			minOnDemand: 0,
			instances: makeInstancesWithCatalog(instanceMap{
				"i-1": &instance{
					Instance: &ec2.Instance{
						State: &ec2.InstanceState{
							Name: aws.String("running"),
						},
					},
				},
			}),
			wantNeed:  true,
			wantTotal: 1,
		},
		{
			name:        "group with as much on-demand instances as desired",
			minOnDemand: 1,
			instances: makeInstancesWithCatalog(instanceMap{
				"i-1": &instance{
					Instance: &ec2.Instance{
						State: &ec2.InstanceState{
							Name: aws.String("running"),
						},
					},
				},
			}),
			wantNeed:  false,
			wantTotal: 1,
		},
		{
			name:        "group with fewer on-demand instances than desired",
			minOnDemand: 2,
			instances: makeInstancesWithCatalog(instanceMap{
				"i-1": &instance{
					Instance: &ec2.Instance{
						State: &ec2.InstanceState{
							Name: aws.String("running"),
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
				instances:   tt.instances,
				minOnDemand: tt.minOnDemand,
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

func Test_autoScalingGroup_terminateRandomSpotInstanceIfHavingEnough(t *testing.T) {

	tests := []struct {
		name                string
		group               *autoscaling.Group
		asgName             string
		region              *region
		launchConfiguration *launchConfiguration
		instances           instances
		minOnDemand         int64
		config              AutoScalingConfig
		totalRunning        int64
		wait                bool
		wantErr             bool
	}{
		{
			name:         "last running instance",
			totalRunning: 1,
			wantErr:      false,
		},

		{name: "not enough running capacity in the group",
			group: &autoscaling.Group{
				DesiredCapacity: aws.Int64(2),
			},
			instances: makeInstancesWithCatalog(instanceMap{
				"i-foo": &instance{
					Instance: &ec2.Instance{
						InstanceId: aws.String("i-foo"),
						State: &ec2.InstanceState{
							Name: aws.String("running"),
						},
					},
				},
			}),
			wantErr: false,
		},

		{name: "no spot capacity in the group",
			group: &autoscaling.Group{
				DesiredCapacity: aws.Int64(1),
			},
			instances: makeInstancesWithCatalog(instanceMap{
				"i-f00": &instance{
					Instance: &ec2.Instance{
						InstanceId: aws.String("i-foo"),
						State: &ec2.InstanceState{
							Name: aws.String("running"),
						},
					},
				},
			}),
			wantErr: false,
		},

		{name: "spot capacity is correct, skip termination",
			group: &autoscaling.Group{
				DesiredCapacity: aws.Int64(2),
			},
			minOnDemand: 1,
			instances: makeInstancesWithCatalog(instanceMap{
				"i-f00": &instance{
					Instance: &ec2.Instance{
						InstanceId:        aws.String("i-foo0"),
						InstanceLifecycle: aws.String(Spot),
						State: &ec2.InstanceState{
							Name: aws.String("running"),
						},
					},
				},
				"i-f01": &instance{
					Instance: &ec2.Instance{
						InstanceId: aws.String("i-foo1"),
						State: &ec2.InstanceState{
							Name: aws.String("running"),
						},
					},
				},
			}),
			wantErr: false,
		},

		{name: "spot capacity exists in the group, terminating using the default termination method",
			group: &autoscaling.Group{
				DesiredCapacity: aws.Int64(1),
			},
			instances: makeInstancesWithCatalog(instanceMap{
				"i-foo": &instance{
					Instance: &ec2.Instance{
						InstanceId:        aws.String("i-foo"),
						InstanceLifecycle: aws.String(Spot),
						State: &ec2.InstanceState{
							Name: aws.String("running"),
						},
					},
					region: &region{
						services: connections{
							ec2: mockEC2{
								diao: &ec2.DescribeInstanceAttributeOutput{},
							},
						},
					},
				},
			}),
			region: &region{
				services: connections{
					ec2: mockEC2{
						tierr: errors.New("dummy error"),
					},
					autoScaling: mockASG{
						dlho: &autoscaling.DescribeLifecycleHooksOutput{},
					},
				},
			},
			wantErr: false,
		},

		{name: "spot capacity exists in the group, terminating using the non-default termination method",
			group: &autoscaling.Group{
				DesiredCapacity: aws.Int64(1),
			},
			instances: makeInstancesWithCatalog(instanceMap{
				"i-f00": &instance{
					Instance: &ec2.Instance{
						InstanceId:        aws.String("i-foo"),
						InstanceLifecycle: aws.String(Spot),
						State: &ec2.InstanceState{
							Name: aws.String("running"),
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
			}),
			region: &region{
				services: connections{
					autoScaling: mockASG{
						dlho: &autoscaling.DescribeLifecycleHooksOutput{},
					},
				},
			},
			config: AutoScalingConfig{
				TerminationMethod: DetachTerminationMethod,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &autoScalingGroup{
				Group:               tt.group,
				name:                tt.asgName,
				region:              tt.region,
				launchConfiguration: tt.launchConfiguration,
				instances:           tt.instances,
				minOnDemand:         tt.minOnDemand,
				config:              tt.config,
			}
			if err := a.terminateRandomSpotInstanceIfHavingEnough(tt.totalRunning, tt.wait); (err != nil) != tt.wantErr {
				t.Errorf("autoScalingGroup.terminateRandomSpotInstanceIfHavingEnough() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_autoScalingGroup_cronEventAction(t *testing.T) {
	asgNoInstances := autoScalingGroup{
		Group: &autoscaling.Group{},
		region: &region{
			conf: &Config{
				AutoScalingConfig: AutoScalingConfig{
					CronSchedule:      DefaultCronSchedule,
					CronScheduleState: "on",
				},
				LicenseType: "evaluation",
				Version:     "custom",
			},
			instances: makeInstancesWithCatalog(
				instanceMap{}),
		},
	}

	asgTerminateSpotInstance := autoScalingGroup{

		Group: &autoscaling.Group{
			Instances: []*autoscaling.Instance{
				{
					InstanceId:           aws.String("i-ondemand"),
					ProtectedFromScaleIn: aws.Bool(false),
					LifecycleState:       aws.String("Running"),
				},
			},
		},
		region: &region{
			conf: &Config{
				AutoScalingConfig: AutoScalingConfig{
					CronSchedule:      DefaultCronSchedule,
					CronScheduleState: "on",
					MinOnDemandNumber: 1,
				},
				LicenseType: "custom",
				Version:     "nightly",
			},
			instances: makeInstancesWithCatalog(
				instanceMap{
					"i-ondemand": {
						Instance: &ec2.Instance{
							InstanceId: aws.String("i-ondemand"),

							Placement: &ec2.Placement{
								AvailabilityZone: aws.String("us-east-1a"),
							},
							State: &ec2.InstanceState{
								Name: aws.String(ec2.InstanceStateNameRunning),
							},
						},
						price: 30.3,
						typeInfo: instanceTypeInformation{
							pricing: prices{
								onDemand: 30.3,
								spot: spotPriceMap{
									"us-east-1a": 0.2,
								},
							},
						},
					},
				}),
			services: connections{
				ec2: mockEC2{
					diao: &ec2.DescribeInstanceAttributeOutput{},
				},
			},
		},
	}

	onDemandInstance := instance{
		Instance: &ec2.Instance{
			InstanceId: aws.String("i-ondemand"),

			Placement: &ec2.Placement{
				AvailabilityZone: aws.String("us-east-1a"),
			},
			State: &ec2.InstanceState{
				Name: aws.String(ec2.InstanceStateNameRunning),
			},
		},
		price: 30.3,
		typeInfo: instanceTypeInformation{
			pricing: prices{
				onDemand: 30.3,
				spot: spotPriceMap{
					"us-east-1a": 0.2,
				},
			},
		},
	}

	asgLaunchSpotReplacement := autoScalingGroup{
		instances: makeInstancesWithCatalog(instanceMap{}),
		Group: &autoscaling.Group{
			Instances: []*autoscaling.Instance{
				{
					InstanceId:           aws.String("i-ondemand"),
					ProtectedFromScaleIn: aws.Bool(false),
					LifecycleState:       aws.String("Running"),
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
				LicenseType: "custom",
				Version:     "nightly",
			},
			instances: makeInstancesWithCatalog(
				instanceMap{
					"i-ondemand": &onDemandInstance,
				}),
			services: connections{
				ec2: mockEC2{
					diao: &ec2.DescribeInstanceAttributeOutput{},
				},
			},
		},
	}

	spotInstance := instance{
		Instance: &ec2.Instance{
			InstanceId:        aws.String("i-spot"),
			InstanceLifecycle: aws.String(Spot),
			LaunchTime:        aws.Time(time.Now().Add(-1 * time.Hour)),

			Placement: &ec2.Placement{
				AvailabilityZone: aws.String("us-east-1a"),
			},
			State: &ec2.InstanceState{
				Name: aws.String(ec2.InstanceStateNameRunning),
			},
			Tags: []*ec2.Tag{
				{Key: aws.String("launched-for-asg"),
					Value: aws.String("asg-foo"),
				},
			},
		},
		price: 30.3,
		typeInfo: instanceTypeInformation{
			pricing: prices{
				onDemand: 30.3,
				spot: spotPriceMap{
					"us-east-1a": 0.2,
				},
			},
		},
	}

	asgExistingSpotReplacementButUnneeded := autoScalingGroup{
		name:      "asg-foo",
		instances: makeInstancesWithCatalog(instanceMap{}),
		Group: &autoscaling.Group{
			Instances: []*autoscaling.Instance{
				{
					InstanceId:           aws.String("i-ondemand"),
					ProtectedFromScaleIn: aws.Bool(false),
					LifecycleState:       aws.String("Running"),
				},
			},
			HealthCheckGracePeriod: aws.Int64(60),
		},
		region: &region{
			conf: &Config{
				AutoScalingConfig: AutoScalingConfig{
					CronSchedule:      DefaultCronSchedule,
					CronScheduleState: "on",

					MinOnDemandNumber: 1,
				},
				FinalRecap:  make(map[string][]string),
				LicenseType: "custom",
				Version:     "nightly",
			},
			instances: makeInstancesWithCatalog(
				instanceMap{
					"i-ondemand": &onDemandInstance,
					"i-spot":     &spotInstance,
				}),
			services: connections{
				ec2: mockEC2{
					diao: &ec2.DescribeInstanceAttributeOutput{},
				},
			},
		},
	}

	asgExistingSpotReplacementButNotReady := autoScalingGroup{
		name:      "asg-foo",
		instances: makeInstancesWithCatalog(instanceMap{}),
		Group: &autoscaling.Group{
			Instances: []*autoscaling.Instance{
				{
					InstanceId:           aws.String("i-ondemand"),
					ProtectedFromScaleIn: aws.Bool(false),
					LifecycleState:       aws.String("Running"),
				},
			},
			HealthCheckGracePeriod: aws.Int64(7200),
		},
		region: &region{
			conf: &Config{
				AutoScalingConfig: AutoScalingConfig{
					CronSchedule:      DefaultCronSchedule,
					CronScheduleState: "on",
					MinOnDemandNumber: 0,
				},
				LicenseType: "custom",
				Version:     "nightly",
			},
			instances: makeInstancesWithCatalog(
				instanceMap{
					"i-ondemand": &onDemandInstance,
					"i-spot":     &spotInstance,
				}),
			services: connections{
				ec2: mockEC2{
					diao: &ec2.DescribeInstanceAttributeOutput{},
				},
			},
		},
	}

	asgExistingSpotReplacementAndReady := autoScalingGroup{
		name:      "asg-foo",
		instances: makeInstancesWithCatalog(instanceMap{}),
		Group: &autoscaling.Group{
			Instances: []*autoscaling.Instance{
				{
					InstanceId:           aws.String("i-ondemand"),
					ProtectedFromScaleIn: aws.Bool(false),
					LifecycleState:       aws.String("Running"),
				},
			},
			HealthCheckGracePeriod: aws.Int64(60),
		},
		region: &region{
			conf: &Config{
				AutoScalingConfig: AutoScalingConfig{
					CronSchedule:      DefaultCronSchedule,
					CronScheduleState: "on",
					MinOnDemandNumber: 0,
				},
				LicenseType: "custom",
				Version:     "nightly",
			},
			instances: makeInstancesWithCatalog(
				instanceMap{
					"i-ondemand": &onDemandInstance,
					"i-spot":     &spotInstance,
				}),
			services: connections{
				ec2: mockEC2{
					diao: &ec2.DescribeInstanceAttributeOutput{},
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
				Group: &autoscaling.Group{},
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

		{name: "not allowed to replace instances, terminate-spot-instance",
			asg:  &asgTerminateSpotInstance,
			want: terminateSpotInstance{target{asg: &asgTerminateSpotInstance, totalInstances: 1}},
		},

		{name: "allowed to replace instance, launch spot instance replacement",
			asg: &asgLaunchSpotReplacement,
			want: launchSpotReplacement{target{
				onDemandInstance: &onDemandInstance},
			},
		},

		{name: "allowed to replace instance, spot instance replacement exists but not needed",
			asg: &asgExistingSpotReplacementButUnneeded,
			want: terminateUnneededSpotInstance{
				target{
					asg:            &asgExistingSpotReplacementButUnneeded,
					spotInstance:   &spotInstance,
					totalInstances: 1,
				},
			},
		},

		{name: "allowed to replace instance, spot instance replacement exists but not ready",
			asg:  &asgExistingSpotReplacementButNotReady,
			want: skipRun{reason: "spot instance replacement exists but not ready"},
		},

		{name: "allowed to replace instance, spot instance replacement exists and ready",
			asg: &asgExistingSpotReplacementAndReady,
			want: swapSpotInstance{target{
				asg:          &asgExistingSpotReplacementAndReady,
				spotInstance: &spotInstance,
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
