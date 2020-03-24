// Copyright (c) 2016-2019 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"errors"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/davecgh/go-spew/spew"
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
							InstanceLifecycle: aws.String("spot"),
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
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
					"id-2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String("on-demand"),
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
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"id-2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String("spot"),
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
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
					"id-2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String("on-demand"),
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
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
					"id-2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String("on-demand"),
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
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"id-2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String("spot"),
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
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"id-2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String("spot"),
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
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"id-2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String("on-demand"),
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
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"id-2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String("on-demand"),
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
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"id-2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameStopped)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String("on-demand"),
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

func TestNeedReplaceOnDemandInstances(t *testing.T) {
	tests := []struct {
		name            string
		asgInstances    instances
		minOnDemand     int64
		desiredCapacity *int64
		expectedRun     bool
		regionASG       *region
	}{
		{name: "ASG has no instance at all - 1 on-demand required",
			asgInstances:    makeInstances(),
			minOnDemand:     1,
			desiredCapacity: aws.Int64(0),
			expectedRun:     false,
		},
		{name: "ASG has no instance at all - 0 on-demand required",
			asgInstances:    makeInstances(),
			minOnDemand:     0,
			desiredCapacity: aws.Int64(0),
			expectedRun:     false,
		},
		{name: "ASG has no instance running - 1 on-demand required",
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
					"id-1": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameShuttingDown)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"id-2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameShuttingDown)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
				},
			),
			minOnDemand:     1,
			desiredCapacity: aws.Int64(0),
			expectedRun:     false,
		},
		{name: "ASG has no instance running - 0 on-demand required",
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
					"id-1": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameShuttingDown)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"id-2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameShuttingDown)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
				},
			),
			minOnDemand:     0,
			desiredCapacity: aws.Int64(0),
			expectedRun:     false,
		},
		{name: "ASG has not the required on-demand running",
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
					"id-1": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("id-1"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("spot"),
						},
						region: &region{
							name: "test-region",
							services: connections{
								ec2: &mockEC2{},
							},
						},
					},
					"id-2": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("id-2"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
				},
			),
			minOnDemand:     2,
			desiredCapacity: aws.Int64(0),
			expectedRun:     false,
			regionASG: &region{
				name: "regionTest",
				services: connections{
					autoScaling: mockASG{},
				},
				conf: &Config{},
			},
		},
		{name: "ASG has just enough on-demand instances running",
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
					"id-1": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"id-2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
				},
			),
			minOnDemand:     1,
			desiredCapacity: aws.Int64(0),
			expectedRun:     false,
		},
		{name: "ASG has only one remaining instance, less than enough on-demand",
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
					"id-1": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
				},
			),
			minOnDemand:     1,
			desiredCapacity: aws.Int64(1),
			expectedRun:     false,
		},
		{name: "ASG has more than enough on-demand instances running but not desired capacity",
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
					"id-1": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
					"id-2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
				},
			),
			minOnDemand:     1,
			desiredCapacity: aws.Int64(1),
			expectedRun:     true,
		},
		{name: "ASG has more than enough on-demand instances running and desired capacity",
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
					"id-1": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
					"id-2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
				},
			),
			minOnDemand:     1,
			desiredCapacity: aws.Int64(4),
			expectedRun:     true,
		},
		{name: "ASG has on-demand instances equal to the min on-demand number",
			asgInstances: makeInstancesWithCatalog(
				map[string]*instance{
					"id-1": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
					"id-2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
				},
			),
			minOnDemand:     1,
			desiredCapacity: aws.Int64(2),
			expectedRun:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := autoScalingGroup{Group: &autoscaling.Group{}}
			a.name = "asg-test"
			a.DesiredCapacity = tt.desiredCapacity
			a.instances = tt.asgInstances
			a.minOnDemand = tt.minOnDemand
			a.region = tt.regionASG
			shouldRun := a.needReplaceOnDemandInstances()
			if tt.expectedRun != shouldRun {
				t.Errorf("needReplaceOnDemandInstances returned: %t expected %t",
					shouldRun, tt.expectedRun)
			}
		})
	}
}

func TestDetachAndTerminateOnDemandInstance(t *testing.T) {
	tests := []struct {
		name         string
		instancesASG instances
		regionASG    *region
		instanceID   *string
		expected     error
	}{
		{name: "no err during detach nor terminate",
			instancesASG: makeInstancesWithCatalog(
				instanceMap{
					"1": {
						Instance: &ec2.Instance{
							InstanceId: aws.String("1"),
							State: &ec2.InstanceState{
								Name: aws.String(ec2.InstanceStateNameRunning),
							},
						},
						region: &region{
							services: connections{
								ec2: mockEC2{tierr: nil},
							},
						},
					},
				},
			),
			regionASG: &region{
				name: "regionTest",
				services: connections{
					autoScaling: mockASG{dierr: nil},
				},
				conf: &Config{},
			},
			instanceID: aws.String("1"),
			expected:   nil,
		},
		{name: "err during detach not during terminate",
			instancesASG: makeInstancesWithCatalog(
				instanceMap{
					"1": {
						Instance: &ec2.Instance{
							InstanceId: aws.String("1"),
							State: &ec2.InstanceState{
								Name: aws.String(ec2.InstanceStateNameRunning),
							},
						},
						region: &region{
							services: connections{
								ec2: mockEC2{tierr: nil},
							},
						},
					},
				},
			),
			regionASG: &region{
				name: "regionTest",
				services: connections{
					autoScaling: mockASG{dierr: errors.New("detach")},
				},
				conf: &Config{},
			},
			instanceID: aws.String("1"),
			expected:   errors.New("detach"),
		},
		{name: "no err during detach but error during terminate",
			instancesASG: makeInstancesWithCatalog(
				instanceMap{
					"1": {
						Instance: &ec2.Instance{
							InstanceId: aws.String("1"),
							State: &ec2.InstanceState{
								Name: aws.String(ec2.InstanceStateNameRunning),
							},
						},
						region: &region{
							services: connections{
								ec2: mockEC2{tierr: errors.New("terminate")},
							},
						},
					},
				},
			),
			regionASG: &region{
				name: "regionTest",
				services: connections{
					autoScaling: mockASG{dierr: nil},
				},
				conf: &Config{},
			},
			instanceID: aws.String("1"),
			expected:   errors.New("terminate"),
		},
		{name: "errors during detach and terminate",
			instancesASG: makeInstancesWithCatalog(
				instanceMap{
					"1": {
						Instance: &ec2.Instance{
							InstanceId: aws.String("1"),
							State: &ec2.InstanceState{
								Name: aws.String(ec2.InstanceStateNameRunning),
							},
						},
						region: &region{
							services: connections{
								ec2: mockEC2{tierr: errors.New("terminate")},
							},
						},
					},
				},
			),
			regionASG: &region{
				name: "regionTest",
				services: connections{
					autoScaling: mockASG{dierr: errors.New("detach")},
				},
				conf: &Config{},
			},
			instanceID: aws.String("1"),
			expected:   errors.New("detach"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := autoScalingGroup{
				name:      "testASG",
				region:    tt.regionASG,
				instances: tt.instancesASG,
			}
			err := a.detachAndTerminateOnDemandInstance(tt.instanceID)
			CheckErrors(t, err, tt.expected)
		})
	}
}

func TestTerminateInstanceInAutoScalingGroup(t *testing.T) {
	tests := []struct {
		name         string
		instancesASG instances
		regionASG    *region
		instanceID   *string
		expected     error
	}{
		{name: "no err during terminate",
			instancesASG: makeInstancesWithCatalog(
				map[string]*instance{
					"1": {
						Instance: &ec2.Instance{
							InstanceId: aws.String("1"),
							State: &ec2.InstanceState{
								Name: aws.String(ec2.InstanceStateNameRunning),
							},
						},
						region: &region{
							services: connections{
								ec2: mockEC2{},
							},
						},
					},
				},
			),
			regionASG: &region{
				name: "regionTest",
				services: connections{
					autoScaling: mockASG{tiiasgerr: nil},
				},
				conf: &Config{},
			},
			instanceID: aws.String("1"),
			expected:   nil,
		},
		{name: "errors during terminate",
			instancesASG: makeInstancesWithCatalog(
				map[string]*instance{
					"1": {
						Instance: &ec2.Instance{
							InstanceId: aws.String("1"),
							State: &ec2.InstanceState{
								Name: aws.String(ec2.InstanceStateNameRunning),
							},
						},
						region: &region{
							services: connections{
								ec2: mockEC2{},
							},
						},
					},
				},
			),
			regionASG: &region{
				name: "regionTest",
				services: connections{
					autoScaling: mockASG{tiiasgerr: errors.New("terminate-asg")},
				},
				conf: &Config{},
			},
			instanceID: aws.String("1"),
			expected:   errors.New("terminate-asg"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := autoScalingGroup{
				name:      "testASG",
				region:    tt.regionASG,
				instances: tt.instancesASG,
			}
			err := a.terminateInstanceInAutoScalingGroup(tt.instanceID)
			CheckErrors(t, err, tt.expected)
		})
	}
}

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
					autoScaling: mockASG{aierr: nil},
				},
			},
			instanceID: "1",
			expected:   nil,
		},
		{name: "err during attach",
			regionASG: &region{
				name: "regionTest",
				services: connections{
					autoScaling: mockASG{aierr: errors.New("attach")},
				},
			},
			instanceID: "1",
			expected:   errors.New("attach"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := autoScalingGroup{
				name:   "testASG",
				region: tt.regionASG,
			}
			err := a.attachSpotInstance(tt.instanceID)
			CheckErrors(t, err, tt.expected)
		})
	}
}

func TestLoadLaunchConfiguration(t *testing.T) {
	tests := []struct {
		name        string
		nameLC      *string
		regionASG   *region
		expectedLC  *launchConfiguration
		expectedErr error
	}{
		{name: "nil launch configuration name",
			nameLC: nil,
			regionASG: &region{
				services: connections{
					autoScaling: mockASG{
						dlcerr: nil},
				},
			},
			expectedErr: nil,
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

				region: tt.regionASG,
				Group: &autoscaling.Group{
					LaunchConfigurationName: tt.nameLC,
				},
			}
			lc, err := a.loadLaunchConfiguration()

			if !reflect.DeepEqual(tt.expectedErr, err) {
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
				instances: makeInstancesWithCatalog(
					instanceMap{
						"1": {
							Instance: &ec2.Instance{
								InstanceId: aws.String("1"),
								Placement: &ec2.Placement{
									AvailabilityZone: aws.String("az-1"),
								},
								InstanceLifecycle: aws.String("spot"),
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
					{InstanceId: aws.String("1")},
					{
						InstanceId:           aws.String("2"),
						ProtectedFromScaleIn: aws.Bool(true),
						AvailabilityZone:     aws.String("1a"),
					},
					{InstanceId: aws.String("3")},
				},
			},
			expectedInstances: instanceMap{
				"1": {
					Instance: &ec2.Instance{
						InstanceId: aws.String("1"),
						Placement: &ec2.Placement{
							AvailabilityZone: aws.String("az-1"),
						},
						InstanceLifecycle: aws.String("spot"),
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

func TestGetOnDemandInstanceInAZ(t *testing.T) {
	tests := []struct {
		name         string
		asgInstances instances
		az           *string
		expected     *instance
	}{
		{
			name: "ASG has no 'running' instance in the current AZ but only in other AZs",
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
					"spot-stopped": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("spot-stopped"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameStopped)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"spot-running": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("spot-running"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
							InstanceLifecycle: aws.String("spot"),
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
			az:       aws.String("1c"),
			expected: nil,
		},
		{
			name: "ASG has 'running' instance in AZ",
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
					"spot-stopped": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("spot-stopped"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameStopped)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"spot-running": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("spot-running"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
							InstanceLifecycle: aws.String("spot"),
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
			az: aws.String("1b"),
			expected: &instance{
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

		{
			name: "ASG has 'running' instance in AZ ad we we get error when trying to determine termination protection",
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
					"spot-stopped": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("spot-stopped"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameStopped)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"spot-running": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("spot-running"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
							InstanceLifecycle: aws.String("spot"),
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
									diaerr: errors.New("error when determining instance termination protection"),
								},
							},
						},
					},
				},
			),
			az:       aws.String("1b"),
			expected: nil,
		},

		{
			name: "ASG has 'running' but protected from termination instance in AZ",
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
					"spot-stopped": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("spot-stopped"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameStopped)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"spot-running": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("spot-running"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
							InstanceLifecycle: aws.String("spot"),
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
											Value: aws.Bool(true),
										},
									},
								},
							},
						},
					},
				},
			),
			az:       aws.String("1b"),
			expected: nil,
		},
		{
			name: "ASG has 'running' but protected from ASG scale-in instance in AZ",
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
					"spot-stopped": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("spot-stopped"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameStopped)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"spot-running": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("spot-running"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
							InstanceLifecycle: aws.String("spot"),
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
						asg: &autoScalingGroup{
							Group: &autoscaling.Group{
								Instances: []*autoscaling.Instance{
									{
										InstanceId:           aws.String("ondemand-running"),
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
			az:       aws.String("1b"),
			expected: nil,
		},

		{
			name: "ASG has no instance in AZ",
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
					"spot-stopped": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("spot-stopped"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameStopped)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"spot-running": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("spot-running"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
							InstanceLifecycle: aws.String("spot"),
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
			az:       aws.String("2a"),
			expected: nil,
		},
		{
			name:         "ASG has no instance at all",
			asgInstances: makeInstances(),
			az:           aws.String("1a"),
			expected:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &autoScalingGroup{
				instances: tt.asgInstances,
			}
			returnedInstance := a.getUnprotectedOnDemandInstanceInAZ(tt.az)
			if !reflect.DeepEqual(returnedInstance, tt.expected) {
				t.Errorf("%s: getOnDemandInstanceInAZ \nreceived: %+v,\n  expected: %+v",
					tt.name,
					spew.Sdump(returnedInstance),
					spew.Sdump(tt.expected))
			}
		})
	}
}

func TestGetAnyOnDemandInstance(t *testing.T) {
	tests := []struct {
		name         string
		asgInstances instances
		expected     []*instance
	}{
		{name: "ASG has no 'running' OnDemand instance",
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
					"spot-stopped": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("spot-stopped"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameStopped)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"spot-running": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("spot-running"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
							InstanceLifecycle: aws.String("spot"),
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
				},
			),
			expected: []*instance{},
		},
		{name: "ASG has one 'running' OnDemand instance",
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
					"spot-stopped": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("spot-stopped"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameStopped)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"spot-running": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("spot-running"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
							InstanceLifecycle: aws.String("spot"),
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
					},
				},
			),
			expected: []*instance{{
				Instance: &ec2.Instance{
					InstanceId:        aws.String("ondemand-running"),
					State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
					Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
					InstanceLifecycle: aws.String(""),
				}},
			},
		},
		{name: "ASG has multiple 'running' OnDemand instances",
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
					"spot-stopped": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("spot-stopped"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameStopped)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"spot-running": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("spot-running"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"ondemand-running1": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("ondemand-running1"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1c")},
							InstanceLifecycle: aws.String(""),
						},
					},
					"ondemand-running2": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("ondemand-running2"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
							InstanceLifecycle: aws.String(""),
						},
					},
				},
			),
			expected: []*instance{
				{
					Instance: &ec2.Instance{
						InstanceId:        aws.String("ondemand-running2"),
						State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
						Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
						InstanceLifecycle: aws.String(""),
					},
				},
				{
					Instance: &ec2.Instance{
						InstanceId:        aws.String("ondemand-running1"),
						State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
						Placement:         &ec2.Placement{AvailabilityZone: aws.String("1c")},
						InstanceLifecycle: aws.String(""),
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
			returnedInstance := a.getAnyOnDemandInstance()
			if len(tt.expected) == 0 && returnedInstance != nil {
				t.Errorf("getAnyOnDemandInstance received: %+v, expected: nil",
					returnedInstance)
			} else if len(tt.expected) != 0 {
				for _, i := range tt.expected {
					if reflect.DeepEqual(returnedInstance, i) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("getAnyOnDemandInstance received: %+v, expected to be in: %+v",
						returnedInstance,
						tt.expected)
				}
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
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"ondemand-stopped": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("onemand-stopped"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameStopped)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
							InstanceLifecycle: aws.String(""),
						},
					},
					"ondemand-running": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("onemand-running"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1c")},
							InstanceLifecycle: aws.String(""),
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
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"spot-running": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("spot-running"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
							InstanceLifecycle: aws.String("spot"),
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
					},
				},
			),
			expected: []*instance{{
				Instance: &ec2.Instance{
					InstanceId:        aws.String("spot-running"),
					State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
					Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
					InstanceLifecycle: aws.String("spot"),
				}},
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
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"spot-running2": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("spot-running2"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
							InstanceLifecycle: aws.String("spot"),
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
					},
				},
			),
			expected: []*instance{
				{
					Instance: &ec2.Instance{
						InstanceId:        aws.String("spot-running1"),
						State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
						Placement:         &ec2.Placement{AvailabilityZone: aws.String("1a")},
						InstanceLifecycle: aws.String("spot"),
					},
				},
				{
					Instance: &ec2.Instance{
						InstanceId:        aws.String("spot-running2"),
						State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
						Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
						InstanceLifecycle: aws.String("spot"),
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
						ec2: &mockEC2{
							tio:   nil,
							tierr: nil,
						},
					},
					instances: makeInstancesWithCatalog(
						instanceMap{
							"spot-running": {
								Instance: &ec2.Instance{
									InstanceId:        aws.String("spot-running"),
									State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameStopped)},
									Placement:         &ec2.Placement{AvailabilityZone: aws.String("1a")},
									InstanceLifecycle: aws.String("spot"),
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
						ec2: &mockEC2{
							tio:   nil,
							tierr: nil,
						},
					},
					instances: makeInstancesWithCatalog(
						instanceMap{
							"spot-running": {
								Instance: &ec2.Instance{
									InstanceId:        aws.String("spot-running"),
									State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameStopped)},
									Placement:         &ec2.Placement{AvailabilityZone: aws.String("1a")},
									InstanceLifecycle: aws.String("spot"),
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
									InstanceLifecycle: aws.String("spot"),
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
			expected: errors.New("couldn't find ondemand instance to replace"),
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
									Placement:         &ec2.Placement{AvailabilityZone: aws.String("1z")},
									InstanceLifecycle: aws.String("spot"),
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			returned := tt.asg.replaceOnDemandInstanceWithSpot(tt.spotID)
			CheckErrors(t, returned, tt.expected)
		})
		t.Run(tt.name+"-detach-method", func(t *testing.T) {
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
							AllowedInstanceTypes: "",
						}},
				},
				Group: &autoscaling.Group{
					DesiredCapacity: aws.Int64(4),
				},
			},
			asgtags: []*autoscaling.TagDescription{
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
			asgtags: []*autoscaling.TagDescription{},
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
							AllowedInstanceTypes: "c2.xlarge",
						}},
				},
				Group: &autoscaling.Group{
					DesiredCapacity: aws.Int64(4),
				},
			},
			asgtags: []*autoscaling.TagDescription{
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
			asgtags: []*autoscaling.TagDescription{
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
							AllowedInstanceTypes: "c2.xlarge t2.medium c3.small",
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
							AllowedInstanceTypes: ",,c2.xlarge,,,t2.medium,c3.small,,",
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
							AllowedInstanceTypes: "   c2.xlarge    t2.medium  c3.small  ",
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

func Test_autoScalingGroup_findUnattachedInstanceLaunchedForThisASG(t *testing.T) {

	tests := []struct {
		name string
		asg  autoScalingGroup
		want *instance
	}{
		{
			name: "no instances launched for this ASG",
			asg: autoScalingGroup{
				name: "mygroup",
				region: &region{
					instances: makeInstancesWithCatalog(
						instanceMap{
							"id-1": {
								Instance: &ec2.Instance{
									InstanceId: aws.String("id-1"),
									Tags:       []*ec2.Tag{},
								},
							},
						},
					),
				},
			},
			want: nil,
		},
		{
			name: "instance launched for another ASG",
			asg: autoScalingGroup{
				name: "mygroup",
				region: &region{
					instances: makeInstancesWithCatalog(
						instanceMap{
							"id-1": {
								Instance: &ec2.Instance{
									InstanceId: aws.String("id-1"),
									Tags:       []*ec2.Tag{},
								},
							},
							"id-2": {
								Instance: &ec2.Instance{
									InstanceId: aws.String("id-2"),
									Tags: []*ec2.Tag{
										{
											Key:   aws.String("launched-for-asg"),
											Value: aws.String("another-asg"),
										},
										{
											Key:   aws.String("another-key"),
											Value: aws.String("another-value"),
										},
									},
								},
							},
						},
					),
				},
			},
			want: nil,
		}, {
			name: "instance launched for current ASG",
			asg: autoScalingGroup{
				name: "mygroup",
				Group: &autoscaling.Group{
					Instances: []*autoscaling.Instance{
						{InstanceId: aws.String("foo")},
						{InstanceId: aws.String("bar")},
						{InstanceId: aws.String("baz")},
					},
				},

				region: &region{
					instances: makeInstancesWithCatalog(
						instanceMap{
							"id-1": {
								Instance: &ec2.Instance{
									InstanceId: aws.String("id-1"),
									Tags:       []*ec2.Tag{},
								},
							},
							"id-2": {
								Instance: &ec2.Instance{
									InstanceId: aws.String("id-2"),
									Tags: []*ec2.Tag{
										{
											Key:   aws.String("launched-for-asg"),
											Value: aws.String("mygroup"),
										},
										{
											Key:   aws.String("another-key"),
											Value: aws.String("another-value"),
										},
									},
								},
							},
						},
					),
				},
			},
			want: &instance{
				Instance: &ec2.Instance{
					InstanceId: aws.String("id-2"),
					Tags: []*ec2.Tag{
						{
							Key:   aws.String("launched-for-asg"),
							Value: aws.String("mygroup"),
						},
						{
							Key:   aws.String("another-key"),
							Value: aws.String("another-value"),
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := tt.asg

			if got := a.findUnattachedInstanceLaunchedForThisASG(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("autoScalingGroup.findUnattachedInstanceLaunchedForThisASG() = %v, want %v", got, tt.want)
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
					"ondemand-protected-scalein": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("ondemand-protected-scalein"),
							State:             &ec2.InstanceState{Name: aws.String(ec2.InstanceStateNameRunning)},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
							InstanceLifecycle: aws.String(""),
						},
						asg: &autoScalingGroup{
							Group: &autoscaling.Group{
								Instances: []*autoscaling.Instance{
									{
										InstanceId:           aws.String("ondemand-protected-scalein"),
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

func Test_autoScalingGroup_licensedToRun(t *testing.T) {

	tests := []struct {
		name    string
		asg     autoScalingGroup
		want    bool
		wantErr bool
	}{
		{
			name: "On-demand-only",
			asg: autoScalingGroup{
				instances: makeInstancesWithCatalog(
					instanceMap{
						"ondemand-1": {
							price: 1.0,
							typeInfo: instanceTypeInformation{
								pricing: prices{
									onDemand: 1.0,
								},
							},
						},
						"ondemand-2": {
							price: 1.0,
							typeInfo: instanceTypeInformation{
								pricing: prices{
									onDemand: 1.0,
								},
							},
						},
					}),
			},
			want:    true,
			wantErr: false,
		},

		{
			name: "spot-less-than-1000-monthly",
			asg: autoScalingGroup{
				instances: makeInstancesWithCatalog(
					instanceMap{
						"spot-1": {
							price: 0.9,
							typeInfo: instanceTypeInformation{
								pricing: prices{
									onDemand: 1.0,
								},
							},
						},
						"spot-2": {
							price: 0.9,
							typeInfo: instanceTypeInformation{
								pricing: prices{
									onDemand: 1.0,
								},
							},
						},
					}),
			},
			want:    true,
			wantErr: false,
		},

		{
			name: "spot-more-than-1000-monthly-evaluation",
			asg: autoScalingGroup{
				region: &region{
					conf: &Config{
						LicenseType: "evaluation",
						Version:     "nightly",
					},
				},
				instances: makeInstancesWithCatalog(
					instanceMap{
						"spot-1": {
							price: 0.1,
							typeInfo: instanceTypeInformation{
								pricing: prices{
									onDemand: 10.1,
								},
							},
						},
						"spot-2": {
							price: 0.1,
							typeInfo: instanceTypeInformation{
								pricing: prices{
									onDemand: 10.1,
								},
							},
						},
					}),
			},
			want:    false,
			wantErr: true,
		},

		{
			name: "spot-more-than-1000-monthly-custom",
			asg: autoScalingGroup{
				region: &region{
					conf: &Config{
						LicenseType: "evaluation",
						Version:     "custom",
					},
				},
				instances: makeInstancesWithCatalog(
					instanceMap{
						"spot-1": {
							price: 0.1,
							typeInfo: instanceTypeInformation{
								pricing: prices{
									onDemand: 10.1,
								},
							},
						},
						"spot-2": {
							price: 0.1,
							typeInfo: instanceTypeInformation{
								pricing: prices{
									onDemand: 10.1,
								},
							},
						},
					}),
			},
			want:    true,
			wantErr: false,
		},

		{
			name: "spot-more-than-1000-monthly-patron",
			asg: autoScalingGroup{
				region: &region{
					conf: &Config{
						LicenseType: "I_am_supporting_it_on_Patreon",
						Version:     "nightly",
					},
				},
				instances: makeInstancesWithCatalog(
					instanceMap{
						"spot-1": {
							price: 0.1,
							typeInfo: instanceTypeInformation{
								pricing: prices{
									onDemand: 10.1,
								},
							},
						},
						"spot-2": {
							price: 0.1,
							typeInfo: instanceTypeInformation{
								pricing: prices{
									onDemand: 10.1,
								},
							},
						},
					}),
			},
			want:    true,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := tt.asg

			got, err := a.licensedToRun()
			if (err != nil) != tt.wantErr {
				t.Errorf("autoScalingGroup.licensedToRun() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("autoScalingGroup.licensedToRun() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_autoScalingGroup_calculateHourlySavings(t *testing.T) {
	tests := []struct {
		name      string
		instances instances
		want      float64
	}{
		{
			name: "On-demand-only",
			instances: makeInstancesWithCatalog(
				instanceMap{
					"ondemand-1": {
						price: 1.0,
						typeInfo: instanceTypeInformation{
							pricing: prices{
								onDemand: 1.0,
							},
						},
					},
					"ondemand-2": {
						price: 1.0,
						typeInfo: instanceTypeInformation{
							pricing: prices{
								onDemand: 1.0,
							},
						},
					},
				}),
			want: 0,
		},

		{
			name: "spot-only",
			instances: makeInstancesWithCatalog(
				instanceMap{
					"spot-1": {
						price: 0.1,
						typeInfo: instanceTypeInformation{
							pricing: prices{
								onDemand: 1.0,
							},
						},
					},
					"spot-2": {
						price: 0.1,
						typeInfo: instanceTypeInformation{
							pricing: prices{
								onDemand: 1.0,
							},
						},
					},
				}),
			want: 1.8,
		},

		{
			name: "spot-and-on-demand",
			instances: makeInstancesWithCatalog(
				instanceMap{
					"ondemand-1": {
						price: 1.0,
						typeInfo: instanceTypeInformation{
							pricing: prices{
								onDemand: 1.0,
							},
						},
					},
					"spot-1": {
						price: 0.1,
						typeInfo: instanceTypeInformation{
							pricing: prices{
								onDemand: 1.0,
							},
						},
					},
				}),
			want: 0.9,
		},
		{
			name: "premium-instance",
			instances: makeInstancesWithCatalog(
				instanceMap{
					"ondemand-1": {
						price: 1.6,
						typeInfo: instanceTypeInformation{
							pricing: prices{
								onDemand: 1.0,
								premium:  0.6,
							},
						},
					},
					"spot-1": {
						price: 0.1,
						typeInfo: instanceTypeInformation{
							pricing: prices{
								onDemand: 1.0,
								premium:  0.6,
							},
						},
					},
				}),
			want: 1.5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &autoScalingGroup{
				instances: tt.instances,
			}
			if got := a.calculateHourlySavings(); got != tt.want {
				t.Errorf("autoScalingGroup.calculateHourlySavings() = %v, want %v", got, tt.want)
			}
		})
	}
}
