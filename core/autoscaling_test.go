package autospotting

import (
	"errors"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"reflect"
	"testing"
)

func TestGetTagValue(t *testing.T) {

	tests := []struct {
		name     string
		asgTags  []*autoscaling.TagDescription
		tagKey   string
		expected *string
	}{
		{name: "Tag can't be found in ASG (no tags)",
			asgTags:  []*autoscaling.TagDescription{},
			tagKey:   "spot-enabled",
			expected: nil,
		},
		{name: "Tag can't be found in ASG (many tags)",
			asgTags: []*autoscaling.TagDescription{
				{
					Key:   aws.String("Name"),
					Value: aws.String("asg-test"),
				},
				{
					Key:   aws.String("env"),
					Value: aws.String("prod"),
				},
			},
			tagKey:   "spot-enabled",
			expected: nil,
		},
		{name: "Tag can be found in ASG (many tags)",
			asgTags: []*autoscaling.TagDescription{
				{
					Key:   aws.String("Name"),
					Value: aws.String("asg-test"),
				},
				{
					Key:   aws.String("env"),
					Value: aws.String("prod"),
				},
				{
					Key:   aws.String("spot-enabled"),
					Value: aws.String("true"),
				},
			},
			tagKey:   "spot-enabled",
			expected: aws.String("true"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := autoScalingGroup{Group: &autoscaling.Group{}}
			a.Tags = tt.asgTags
			retValue := a.getTagValue(tt.tagKey)
			if tt.expected == nil && retValue != tt.expected {
				t.Errorf("Value received for %s: %s expected %s", tt.tagKey, *retValue, *tt.expected)
			} else if tt.expected != nil && *retValue != *tt.expected {
				t.Errorf("Value received for %s: %s expected %s", tt.tagKey, *retValue, *tt.expected)
			}
		})
	}
}

func TestLoadConfOnDemand(t *testing.T) {
	tests := []struct {
		name            string
		asgTags         []*autoscaling.TagDescription
		asgInstances    instances
		maxSize         *int64
		numberExpected  int64
		loadingExpected bool
	}{
		{name: "ASG does not have any conf tags",
			asgTags:         []*autoscaling.TagDescription{},
			asgInstances:    makeInstances(),
			maxSize:         aws.Int64(10),
			numberExpected:  DefaultMinOnDemandValue,
			loadingExpected: false,
		},
		{name: "Percentage value not a number",
			asgTags: []*autoscaling.TagDescription{
				{
					Key:   aws.String("Name"),
					Value: aws.String("asg-test"),
				},
				{
					Key:   aws.String(OnDemandPercentageLong),
					Value: aws.String("text"),
				},
			},
			asgInstances:    makeInstances(),
			maxSize:         aws.Int64(10),
			numberExpected:  DefaultMinOnDemandValue,
			loadingExpected: false,
		},
		{name: "Percentage value out of range (0-100)",
			asgTags: []*autoscaling.TagDescription{
				{
					Key:   aws.String("Name"),
					Value: aws.String("asg-test"),
				},
				{
					Key:   aws.String(OnDemandPercentageLong),
					Value: aws.String("142.2"),
				},
			},
			asgInstances:    makeInstances(),
			maxSize:         aws.Int64(10),
			numberExpected:  DefaultMinOnDemandValue,
			loadingExpected: false,
		},
		{name: "Percentage value out of range - negative (0-100)",
			asgTags: []*autoscaling.TagDescription{
				{
					Key:   aws.String("Name"),
					Value: aws.String("asg-test"),
				},
				{
					Key:   aws.String(OnDemandPercentageLong),
					Value: aws.String("-22"),
				},
			},
			asgInstances:    makeInstances(),
			maxSize:         aws.Int64(10),
			numberExpected:  DefaultMinOnDemandValue,
			loadingExpected: false,
		},
		{name: "Percentage equals 0.00%",
			asgTags: []*autoscaling.TagDescription{
				{
					Key:   aws.String("Name"),
					Value: aws.String("asg-test"),
				},
				{
					Key:   aws.String(OnDemandPercentageLong),
					Value: aws.String("0"),
				},
			},
			asgInstances: makeInstancesWithCatalog(
				map[string]*instance{
					"id-1": {}},
			),
			maxSize:         aws.Int64(10),
			numberExpected:  0,
			loadingExpected: true,
		},
		{name: "Percentage equals 33.0%",
			asgTags: []*autoscaling.TagDescription{
				{
					Key:   aws.String("Name"),
					Value: aws.String("asg-test"),
				},
				{
					Key:   aws.String(OnDemandPercentageLong),
					Value: aws.String("33.0"),
				},
			},
			asgInstances: makeInstancesWithCatalog(
				map[string]*instance{
					"id-1": {},
					"id-2": {},
					"id-3": {},
				},
			),
			maxSize:         aws.Int64(10),
			numberExpected:  1,
			loadingExpected: true,
		},
		{name: "Percentage equals 75.0%",
			asgTags: []*autoscaling.TagDescription{
				{
					Key:   aws.String("Name"),
					Value: aws.String("asg-test"),
				},
				{
					Key:   aws.String(OnDemandPercentageLong),
					Value: aws.String("75.0"),
				},
			},
			asgInstances: makeInstancesWithCatalog(
				map[string]*instance{
					"id-1": {},
					"id-2": {},
					"id-3": {},
				},
			),
			maxSize:         aws.Int64(10),
			numberExpected:  2,
			loadingExpected: true,
		},
		{name: "Percentage equals 100.0%",
			asgTags: []*autoscaling.TagDescription{
				{
					Key:   aws.String("Name"),
					Value: aws.String("asg-test"),
				},
				{
					Key:   aws.String(OnDemandPercentageLong),
					Value: aws.String("100.0"),
				},
			},
			asgInstances: makeInstancesWithCatalog(
				map[string]*instance{
					"id-1": {},
					"id-2": {},
					"id-3": {},
				},
			),
			maxSize:         aws.Int64(10),
			numberExpected:  3,
			loadingExpected: true,
		},
		{name: "Number passed is text",
			asgTags: []*autoscaling.TagDescription{
				{
					Key:   aws.String("Name"),
					Value: aws.String("asg-test"),
				},
				{
					Key:   aws.String(OnDemandNumberLong),
					Value: aws.String("text"),
				},
			},
			asgInstances:    makeInstances(),
			maxSize:         aws.Int64(10),
			numberExpected:  DefaultMinOnDemandValue,
			loadingExpected: false,
		},
		{name: "Number passed is an invalid integer",
			asgTags: []*autoscaling.TagDescription{
				{
					Key:   aws.String("Name"),
					Value: aws.String("asg-test"),
				},
				{
					Key:   aws.String(OnDemandNumberLong),
					Value: aws.String("2.5"),
				},
			},
			asgInstances:    makeInstances(),
			maxSize:         aws.Int64(10),
			numberExpected:  DefaultMinOnDemandValue,
			loadingExpected: false,
		},
		{name: "Number passed out of range (negative)",
			asgTags: []*autoscaling.TagDescription{
				{
					Key:   aws.String("Name"),
					Value: aws.String("asg-test"),
				},
				{
					Key:   aws.String(OnDemandNumberLong),
					Value: aws.String("-7"),
				},
			},
			asgInstances:    makeInstances(),
			maxSize:         aws.Int64(10),
			numberExpected:  DefaultMinOnDemandValue,
			loadingExpected: false,
		},
		{name: "Number superior to ASG size",
			asgTags: []*autoscaling.TagDescription{
				{
					Key:   aws.String("Name"),
					Value: aws.String("asg-test"),
				},
				{
					Key:   aws.String(OnDemandNumberLong),
					Value: aws.String("50"),
				},
			},
			asgInstances: makeInstancesWithCatalog(
				map[string]*instance{
					"id-1": {},
					"id-2": {},
					"id-3": {},
				},
			),
			maxSize:         aws.Int64(10),
			numberExpected:  DefaultMinOnDemandValue,
			loadingExpected: false,
		},
		{name: "Number is valid 1",
			asgTags: []*autoscaling.TagDescription{
				{
					Key:   aws.String("Name"),
					Value: aws.String("asg-test"),
				},
				{
					Key:   aws.String(OnDemandNumberLong),
					Value: aws.String("1"),
				},
			},
			asgInstances: makeInstancesWithCatalog(
				map[string]*instance{
					"id-1": {},
					"id-2": {},
					"id-3": {},
				},
			),
			maxSize:         aws.Int64(10),
			numberExpected:  1,
			loadingExpected: true,
		},
		{name: "Number has priority on percentage value",
			asgTags: []*autoscaling.TagDescription{
				{
					Key:   aws.String("Name"),
					Value: aws.String("asg-test"),
				},
				{
					Key:   aws.String(OnDemandPercentageLong),
					Value: aws.String("75"),
				},
				{
					Key:   aws.String(OnDemandNumberLong),
					Value: aws.String("2"),
				},
			},
			asgInstances: makeInstancesWithCatalog(
				map[string]*instance{
					"id-1": {},
					"id-2": {},
					"id-3": {},
					"id-4": {},
				},
			),
			maxSize:         aws.Int64(10),
			numberExpected:  2,
			loadingExpected: true,
		},
		{name: "Number is invalid so percentage value is used",
			asgTags: []*autoscaling.TagDescription{
				{
					Key:   aws.String("Name"),
					Value: aws.String("asg-test"),
				},
				{
					Key:   aws.String(OnDemandPercentageLong),
					Value: aws.String("75"),
				},
				{
					Key:   aws.String(OnDemandNumberLong),
					Value: aws.String("-2"),
				},
			},
			asgInstances: makeInstancesWithCatalog(
				map[string]*instance{
					"id-1": {},
					"id-2": {},
					"id-3": {},
					"id-4": {},
				},
			),
			maxSize:         aws.Int64(10),
			numberExpected:  3,
			loadingExpected: true,
		},
		{name: "Both number and percentage are invalid",
			asgTags: []*autoscaling.TagDescription{
				{
					Key:   aws.String("Name"),
					Value: aws.String("asg-test"),
				},
				{
					Key:   aws.String(OnDemandPercentageLong),
					Value: aws.String("-75"),
				},
				{
					Key:   aws.String(OnDemandNumberLong),
					Value: aws.String("200"),
				},
			},
			maxSize:         aws.Int64(10),
			asgInstances:    makeInstances(),
			numberExpected:  DefaultMinOnDemandValue,
			loadingExpected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := autoScalingGroup{Group: &autoscaling.Group{}}
			a.Tags = tt.asgTags
			a.instances = tt.asgInstances
			a.MaxSize = tt.maxSize
			done := a.loadConfOnDemand()
			if tt.loadingExpected != done {
				t.Errorf("loadConfOnDemand returned: %t expected %t", done, tt.loadingExpected)
			} else if tt.numberExpected != a.minOnDemand {
				t.Errorf("loadConfOnDemand, minOnDemand value received %d, expected %d",
					a.minOnDemand, tt.numberExpected)
			}
		})
	}
}

func TestLoadDefaultConf(t *testing.T) {
	tests := []struct {
		name            string
		asgInstances    instances
		region          *region
		maxSize         *int64
		numberExpected  int64
		loadingExpected bool
	}{
		{name: "No configuration given",
			region: &region{
				conf: Config{
					MinOnDemandNumber:     0,
					MinOnDemandPercentage: 0.0,
				},
			},
			asgInstances:    makeInstances(),
			maxSize:         aws.Int64(10),
			numberExpected:  DefaultMinOnDemandValue,
			loadingExpected: false,
		},
		{name: "Percentage value out of range (0-100)",
			region: &region{
				conf: Config{
					MinOnDemandNumber:     0,
					MinOnDemandPercentage: 142.2,
				},
			},
			asgInstances:    makeInstances(),
			maxSize:         aws.Int64(10),
			numberExpected:  DefaultMinOnDemandValue,
			loadingExpected: false,
		},
		{name: "Percentage value out of range - negative (0-100)",
			region: &region{
				conf: Config{
					MinOnDemandNumber:     0,
					MinOnDemandPercentage: -22.2,
				},
			},
			asgInstances:    makeInstances(),
			maxSize:         aws.Int64(10),
			numberExpected:  DefaultMinOnDemandValue,
			loadingExpected: false,
		},
		{name: "Percentage equals 33.0%",
			region: &region{
				conf: Config{
					MinOnDemandNumber:     0,
					MinOnDemandPercentage: 33.0,
				},
			},
			asgInstances: makeInstancesWithCatalog(
				map[string]*instance{
					"id-1": {},
					"id-2": {},
					"id-3": {},
				},
			),
			maxSize:         aws.Int64(10),
			numberExpected:  1,
			loadingExpected: true,
		},
		{name: "Percentage equals 75.0%",
			region: &region{
				conf: Config{
					MinOnDemandNumber:     0,
					MinOnDemandPercentage: 75.0,
				},
			},
			asgInstances: makeInstancesWithCatalog(
				map[string]*instance{
					"id-1": {},
					"id-2": {},
					"id-3": {},
				},
			),
			maxSize:         aws.Int64(10),
			numberExpected:  2,
			loadingExpected: true,
		},
		{name: "Percentage equals 100.0%",
			region: &region{
				conf: Config{
					MinOnDemandNumber:     0,
					MinOnDemandPercentage: 100,
				},
			},
			asgInstances: makeInstancesWithCatalog(
				map[string]*instance{
					"id-1": {},
					"id-2": {},
					"id-3": {},
				},
			),
			maxSize:         aws.Int64(10),
			numberExpected:  3,
			loadingExpected: true,
		},
		{name: "Number passed out of range (negative)",
			region: &region{
				conf: Config{
					MinOnDemandNumber:     -4,
					MinOnDemandPercentage: 0,
				},
			},
			asgInstances:    makeInstances(),
			maxSize:         aws.Int64(10),
			numberExpected:  DefaultMinOnDemandValue,
			loadingExpected: false,
		},
		{name: "Number superior to ASG size",
			region: &region{
				conf: Config{
					MinOnDemandNumber:     50,
					MinOnDemandPercentage: 0,
				},
			},
			asgInstances: makeInstancesWithCatalog(
				map[string]*instance{
					"id-1": {},
					"id-2": {},
					"id-3": {},
				},
			),
			maxSize:         aws.Int64(10),
			numberExpected:  DefaultMinOnDemandValue,
			loadingExpected: false,
		},
		{name: "Number is valid 1",
			region: &region{
				conf: Config{
					MinOnDemandNumber:     1,
					MinOnDemandPercentage: 0,
				},
			},
			asgInstances: makeInstancesWithCatalog(
				map[string]*instance{
					"id-1": {},
					"id-2": {},
					"id-3": {},
				},
			),
			maxSize:         aws.Int64(10),
			numberExpected:  1,
			loadingExpected: true,
		},
		{name: "Number has priority on percentage value",
			region: &region{
				conf: Config{
					MinOnDemandNumber:     2,
					MinOnDemandPercentage: 75,
				},
			},
			asgInstances: makeInstancesWithCatalog(
				map[string]*instance{
					"id-1": {},
					"id-2": {},
					"id-3": {},
					"id-4": {},
				},
			),
			maxSize:         aws.Int64(10),
			numberExpected:  2,
			loadingExpected: true,
		},
		{name: "Number is invalid so percentage value is used",
			region: &region{
				conf: Config{
					MinOnDemandNumber:     -20,
					MinOnDemandPercentage: 75.0,
				},
			},
			asgInstances: makeInstancesWithCatalog(
				map[string]*instance{
					"id-1": {},
					"id-2": {},
					"id-3": {},
					"id-4": {},
				},
			),
			maxSize:         aws.Int64(10),
			numberExpected:  3,
			loadingExpected: true,
		},
		{name: "Both number and percentage are invalid",
			region: &region{
				conf: Config{
					MinOnDemandNumber:     -10,
					MinOnDemandPercentage: 142.2,
				},
			},
			maxSize:         aws.Int64(10),
			asgInstances:    makeInstances(),
			numberExpected:  DefaultMinOnDemandValue,
			loadingExpected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := autoScalingGroup{Group: &autoscaling.Group{}}
			a.instances = tt.asgInstances
			a.MaxSize = tt.maxSize
			a.region = tt.region
			done := a.loadDefaultConfig()
			if tt.loadingExpected != done {
				t.Errorf("loadConfOnDemand returned: %t expected %t", done, tt.loadingExpected)
			} else if tt.numberExpected != a.minOnDemand {
				t.Errorf("loadConfOnDemand, minOnDemand value received %d, expected %d",
					a.minOnDemand, tt.numberExpected)
			}
		})
	}
}

func TestLoadConfigFromTags(t *testing.T) {
	tests := []struct {
		name            string
		asgTags         []*autoscaling.TagDescription
		asgInstances    instances
		maxSize         *int64
		loadingExpected bool
	}{
		{name: "Percentage value not a number",
			asgTags: []*autoscaling.TagDescription{
				{
					Key:   aws.String("Name"),
					Value: aws.String("asg-test"),
				},
				{
					Key:   aws.String(OnDemandPercentageLong),
					Value: aws.String("text"),
				},
			},
			asgInstances:    makeInstances(),
			maxSize:         aws.Int64(10),
			loadingExpected: false,
		},
		{name: "Number is invalid so percentage value is used",
			asgTags: []*autoscaling.TagDescription{
				{
					Key:   aws.String("Name"),
					Value: aws.String("asg-test"),
				},
				{
					Key:   aws.String(OnDemandPercentageLong),
					Value: aws.String("75"),
				},
				{
					Key:   aws.String(OnDemandNumberLong),
					Value: aws.String("-2"),
				},
			},
			asgInstances: makeInstancesWithCatalog(
				map[string]*instance{
					"id-1": {},
					"id-2": {},
					"id-3": {},
					"id-4": {},
				},
			),
			maxSize:         aws.Int64(10),
			loadingExpected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := autoScalingGroup{Group: &autoscaling.Group{}}
			a.Tags = tt.asgTags
			a.instances = tt.asgInstances
			a.MaxSize = tt.maxSize
			done := a.loadConfigFromTags()
			if tt.loadingExpected != done {
				t.Errorf("loadConfOnDemand returned: %t expected %t", done, tt.loadingExpected)
			}
		})
	}
}

func TestAlreadyRunningInstanceCount(t *testing.T) {
	tests := []struct {
		name             string
		asgName          string
		asgInstances     instances
		spot             bool
		availabilityZone string
		expectedCount    int64
		expectedTotal    int64
	}{
		{name: "ASG has no instance at all",
			asgName:          "test-asg",
			asgInstances:     makeInstances(),
			spot:             true,
			availabilityZone: "",
			expectedCount:    0,
			expectedTotal:    0,
		},
		{name: "ASG has no 'running' instance but has some",
			asgName: "test-asg",
			asgInstances: makeInstancesWithCatalog(
				map[string]*instance{
					"id-1": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("stopped")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
				},
			),
			spot:             true,
			availabilityZone: "",
			expectedCount:    0,
			expectedTotal:    0,
		},
		{name: "ASG has no 'running' spot instances but has some",
			asgName: "test-asg",
			asgInstances: makeInstancesWithCatalog(
				map[string]*instance{
					"id-1": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
					"id-2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
				},
			),
			spot:             true,
			availabilityZone: "",
			expectedCount:    0,
			expectedTotal:    2,
		},
		{name: "ASG has no 'running' on-demand instances but has some",
			asgName: "test-asg",
			asgInstances: makeInstancesWithCatalog(
				map[string]*instance{
					"id-1": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"id-2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
				},
			),
			spot:             false,
			availabilityZone: "",
			expectedCount:    0,
			expectedTotal:    2,
		},
		{name: "ASG has no 'running' on-demand instances in the AZ",
			asgName: "test-asg",
			asgInstances: makeInstancesWithCatalog(
				map[string]*instance{
					"id-1": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
					"id-2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
				},
			),
			spot:             false,
			availabilityZone: "eu-west-1c",
			expectedCount:    0,
			expectedTotal:    2,
		},
		{name: "ASG has some 'running' on-demand instances in the AZ",
			asgName: "test-asg",
			asgInstances: makeInstancesWithCatalog(
				map[string]*instance{
					"id-1": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
					"id-2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
				},
			),
			spot:             false,
			availabilityZone: "eu-west-1b",
			expectedCount:    1,
			expectedTotal:    2,
		},
		{name: "ASG has no 'running' spot instances in the AZ",
			asgName: "test-asg",
			asgInstances: makeInstancesWithCatalog(
				map[string]*instance{
					"id-1": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"id-2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
				},
			),
			spot:             true,
			availabilityZone: "eu-west-1c",
			expectedCount:    0,
			expectedTotal:    2,
		},
		{name: "ASG has some 'running' spot instances in any AZ",
			asgName: "test-asg",
			asgInstances: makeInstancesWithCatalog(
				map[string]*instance{
					"id-1": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"id-2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
				},
			),
			spot:             true,
			availabilityZone: "",
			expectedCount:    2,
			expectedTotal:    2,
		},
		{name: "ASG has no 'running' spot instances in any AZ",
			asgName: "test-asg",
			asgInstances: makeInstancesWithCatalog(
				map[string]*instance{
					"id-1": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("shutting-down")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"id-2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
				},
			),
			spot:             true,
			availabilityZone: "",
			expectedCount:    0,
			expectedTotal:    1,
		},
		{name: "ASG has some 'running' on-demand instances in any AZ",
			asgName: "test-asg",
			asgInstances: makeInstancesWithCatalog(
				map[string]*instance{
					"id-1": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"id-2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
				},
			),
			spot:             false,
			availabilityZone: "",
			expectedCount:    1,
			expectedTotal:    2,
		},
		{name: "ASG has no 'running' on-demand instances in any AZ",
			asgName: "test-asg",
			asgInstances: makeInstancesWithCatalog(
				map[string]*instance{
					"id-1": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"id-2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("stopped")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
				},
			),
			spot:             false,
			availabilityZone: "",
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
				map[string]*instance{
					"id-1": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("shutting-down")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"id-2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("shutting-down")},
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
				map[string]*instance{
					"id-1": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("shutting-down")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"id-2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("shutting-down")},
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
				map[string]*instance{
					"id-1": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("id-1"),
							State:             &ec2.InstanceState{Name: aws.String("running")},
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
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
				},
			),
			minOnDemand:     2,
			desiredCapacity: aws.Int64(0),
			expectedRun:     false,
		},
		{name: "ASG has just enough on-demand instances running",
			asgInstances: makeInstancesWithCatalog(
				map[string]*instance{
					"id-1": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"id-2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
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
		{name: "ASG has more than enough on-demand instances running but not desired capacity",
			asgInstances: makeInstancesWithCatalog(
				map[string]*instance{
					"id-1": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
					"id-2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
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
				map[string]*instance{
					"id-1": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
					"id-2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := autoScalingGroup{Group: &autoscaling.Group{}}
			a.name = "asg-test"
			a.DesiredCapacity = tt.desiredCapacity
			a.instances = tt.asgInstances
			a.minOnDemand = tt.minOnDemand
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
				map[string]*instance{
					"1": &instance{
						Instance: &ec2.Instance{
							InstanceId: aws.String("1"),
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
			},
			instanceID: aws.String("1"),
			expected:   nil,
		},
		{name: "err during detach not during terminate",
			instancesASG: makeInstancesWithCatalog(
				map[string]*instance{
					"1": &instance{
						Instance: &ec2.Instance{
							InstanceId: aws.String("1"),
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
			},
			instanceID: aws.String("1"),
			expected:   errors.New("detach"),
		},
		{name: "no err during detach but error during terminate",
			instancesASG: makeInstancesWithCatalog(
				map[string]*instance{
					"1": &instance{
						Instance: &ec2.Instance{
							InstanceId: aws.String("1"),
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
			},
			instanceID: aws.String("1"),
			expected:   errors.New("terminate"),
		},
		{name: "errors during detach and terminate",
			instancesASG: makeInstancesWithCatalog(
				map[string]*instance{
					"1": &instance{
						Instance: &ec2.Instance{
							InstanceId: aws.String("1"),
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

func TestAttachSpotInstance(t *testing.T) {
	tests := []struct {
		name       string
		regionASG  *region
		instanceID *string
		expected   error
	}{
		{name: "no err during attach",
			regionASG: &region{
				name: "regionTest",
				services: connections{
					autoScaling: mockASG{aierr: nil},
				},
			},
			instanceID: aws.String("1"),
			expected:   nil,
		},
		{name: "err during attach",
			regionASG: &region{
				name: "regionTest",
				services: connections{
					autoScaling: mockASG{aierr: errors.New("attach")},
				},
			},
			instanceID: aws.String("1"),
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

func TestGetLaunchConfiguration(t *testing.T) {
	tests := []struct {
		name       string
		nameLC     *string
		regionASG  *region
		expectedLC *launchConfiguration
	}{
		{name: "get nil launch configuration",
			nameLC: nil,
			regionASG: &region{
				services: connections{
					autoScaling: mockASG{dlcerr: nil},
				},
			},
			expectedLC: nil,
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
			expectedLC: nil,
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
			lc := a.getLaunchConfiguration()
			if !reflect.DeepEqual(tt.expectedLC, lc) {
				t.Errorf("LaunchConfig received: %+v expected %+v", lc, tt.expectedLC)
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

// Those various calls are bein mocked in this metdho:
//
// rsiX:   request spot instance
// ctX:    create tag
// wusirX: wait until spot instance request fulfilled
// dsirX:  describe spot instance request
// diX:    describe instance
//
func TestBidForSpotInstance(t *testing.T) {
	tests := []struct {
		name      string
		rsls      *ec2.RequestSpotLaunchSpecification
		regionASG *region
		expected  error
	}{
		{name: "no err during bid for spot instance",
			rsls: &ec2.RequestSpotLaunchSpecification{},
			regionASG: &region{
				instances: makeInstances(),
				services: connections{
					ec2: mockEC2{
						rsierr: nil,
						rsio: &ec2.RequestSpotInstancesOutput{
							SpotInstanceRequests: []*ec2.SpotInstanceRequest{
								{
									SpotInstanceRequestId: aws.String("bidTestId"),
								},
							},
						},
						cto:       nil,
						cterr:     nil,
						wusirferr: nil,
						dsirerr:   nil,
						dsiro: &ec2.DescribeSpotInstanceRequestsOutput{
							SpotInstanceRequests: []*ec2.SpotInstanceRequest{
								{InstanceId: aws.String("1")},
							},
						},
						dio: &ec2.DescribeInstancesOutput{
							Reservations: []*ec2.Reservation{{}},
						},
					},
				},
			},
			expected: nil,
		},
		{name: "err during request spot instances",
			rsls: &ec2.RequestSpotLaunchSpecification{},
			regionASG: &region{
				instances: makeInstances(),
				services: connections{
					ec2: mockEC2{
						rsierr: errors.New("requestSpot"),
						rsio: &ec2.RequestSpotInstancesOutput{
							SpotInstanceRequests: []*ec2.SpotInstanceRequest{
								{
									SpotInstanceRequestId: aws.String("bidTestId"),
								},
							},
						},
						cto:       nil,
						cterr:     nil,
						wusirferr: nil,
						dsirerr:   nil,
						dsiro: &ec2.DescribeSpotInstanceRequestsOutput{
							SpotInstanceRequests: []*ec2.SpotInstanceRequest{
								{InstanceId: aws.String("1")},
							},
						},
						dio: &ec2.DescribeInstancesOutput{
							Reservations: []*ec2.Reservation{{}},
						},
					},
				},
			},
			expected: errors.New("requestSpot"),
		},
		{name: "err during create tags",
			rsls: &ec2.RequestSpotLaunchSpecification{},
			regionASG: &region{
				instances: makeInstances(),
				services: connections{
					ec2: mockEC2{
						rsierr: nil,
						rsio: &ec2.RequestSpotInstancesOutput{
							SpotInstanceRequests: []*ec2.SpotInstanceRequest{
								{
									SpotInstanceRequestId: aws.String("bidTestId"),
								},
							},
						},
						cto:       nil,
						cterr:     errors.New("create-tags"),
						wusirferr: nil,
						dsirerr:   nil,
						dsiro: &ec2.DescribeSpotInstanceRequestsOutput{
							SpotInstanceRequests: []*ec2.SpotInstanceRequest{
								{InstanceId: aws.String("1")},
							},
						},
						dio: &ec2.DescribeInstancesOutput{
							Reservations: []*ec2.Reservation{{}},
						},
					},
				},
			},
			expected: errors.New("create-tags"),
		},
		{name: "err during wait until spot instance request fulfilled",
			rsls: &ec2.RequestSpotLaunchSpecification{},
			regionASG: &region{
				instances: makeInstances(),
				services: connections{
					ec2: mockEC2{
						rsierr: nil,
						rsio: &ec2.RequestSpotInstancesOutput{
							SpotInstanceRequests: []*ec2.SpotInstanceRequest{
								{
									SpotInstanceRequestId: aws.String("bidTestId"),
								},
							},
						},
						cto:       nil,
						cterr:     nil,
						wusirferr: errors.New("wait-fulfilled"),
						dsirerr:   nil,
						dsiro: &ec2.DescribeSpotInstanceRequestsOutput{
							SpotInstanceRequests: []*ec2.SpotInstanceRequest{
								{InstanceId: aws.String("1")},
							},
						},
						dio: &ec2.DescribeInstancesOutput{
							Reservations: []*ec2.Reservation{{}},
						},
					},
				},
			},
			expected: errors.New("wait-fulfilled"),
		},
		{name: "err during describe spot instance request",
			rsls: &ec2.RequestSpotLaunchSpecification{},
			regionASG: &region{
				instances: makeInstances(),
				services: connections{
					ec2: mockEC2{
						rsierr: nil,
						rsio: &ec2.RequestSpotInstancesOutput{
							SpotInstanceRequests: []*ec2.SpotInstanceRequest{
								{
									SpotInstanceRequestId: aws.String("bidTestId"),
								},
							},
						},
						cto:       nil,
						cterr:     nil,
						wusirferr: nil,
						dsirerr:   errors.New("describe"),
						dsiro: &ec2.DescribeSpotInstanceRequestsOutput{
							SpotInstanceRequests: []*ec2.SpotInstanceRequest{
								{InstanceId: aws.String("1")},
							},
						},
						dio: &ec2.DescribeInstancesOutput{
							Reservations: []*ec2.Reservation{{}},
						},
					},
				},
			},
			expected: errors.New("describe"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := autoScalingGroup{
				name:   "testASG",
				region: tt.regionASG,
				Group: &autoscaling.Group{
					Tags: []*autoscaling.TagDescription{},
				},
			}
			err := a.bidForSpotInstance(tt.rsls, 0.24)
			CheckErrors(t, err, tt.expected)
		})
	}
}

func TestLoadSpotInstanceRequest(t *testing.T) {
	tests := []struct {
		name     string
		req      *ec2.SpotInstanceRequest
		region   *region
		expected *spotInstanceRequest
	}{
		{name: "using region name 1",
			region: &region{name: "1"},
			req:    &ec2.SpotInstanceRequest{},
			expected: &spotInstanceRequest{
				SpotInstanceRequest: &ec2.SpotInstanceRequest{},
				region:              &region{name: "1"},
				asg: &autoScalingGroup{
					region: &region{name: "1"},
				},
			},
		},
		{name: "using region name 2",
			region: &region{name: "2"},
			req:    &ec2.SpotInstanceRequest{},
			expected: &spotInstanceRequest{
				SpotInstanceRequest: &ec2.SpotInstanceRequest{},
				region:              &region{name: "2"},
				asg: &autoScalingGroup{
					region: &region{name: "2"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &autoScalingGroup{
				region: tt.region,
			}
			sir := a.loadSpotInstanceRequest(tt.req)
			if !reflect.DeepEqual(tt.expected, sir) {
				t.Errorf("Request received: %+v expected %+v", sir, tt.expected)
			}
		})
	}

}

func TestFindSpotInstanceRequests(t *testing.T) {
	tests := []struct {
		name     string
		region   *region
		expected error
	}{
		{name: "multiple spot instance requests found",
			region: &region{
				services: connections{
					ec2: mockEC2{
						dsiro: &ec2.DescribeSpotInstanceRequestsOutput{
							SpotInstanceRequests: []*ec2.SpotInstanceRequest{
								{InstanceId: aws.String("1")},
								{InstanceId: aws.String("2")},
								{InstanceId: aws.String("3")},
							},
						},
						dsirerr: nil,
					},
				},
			},
			expected: nil,
		},
		{name: "no spot instance requests found",
			region: &region{
				services: connections{
					ec2: mockEC2{
						dsiro: &ec2.DescribeSpotInstanceRequestsOutput{
							SpotInstanceRequests: []*ec2.SpotInstanceRequest{},
						},
						dsirerr: nil,
					},
				},
			},
			expected: nil,
		},
		{name: "error during describing spot instance requests",
			region: &region{
				services: connections{
					ec2: mockEC2{
						dsiro: &ec2.DescribeSpotInstanceRequestsOutput{
							SpotInstanceRequests: []*ec2.SpotInstanceRequest{},
						},
						dsirerr: errors.New("describe"),
					},
				},
			},
			expected: errors.New("describe"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &autoScalingGroup{
				Group:  &autoscaling.Group{AutoScalingGroupName: aws.String("testASG")},
				name:   "testASG",
				region: tt.region,
			}
			err := a.findSpotInstanceRequests()
			CheckErrors(t, err, tt.expected)
		})
	}
}

func TestScanInstances(t *testing.T) {
	tests := []struct {
		name              string
		ec2ASG            *autoscaling.Group
		regionInstances   *region
		expectedInstances map[string]*instance
	}{
		{name: "multiple instances to scan",
			regionInstances: &region{
				instances: makeInstancesWithCatalog(
					map[string]*instance{
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
						},
					},
				),
			},
			ec2ASG: &autoscaling.Group{
				Instances: []*autoscaling.Instance{
					{InstanceId: aws.String("1")},
					{InstanceId: aws.String("2")},
					{InstanceId: aws.String("3")},
				},
			},
			expectedInstances: map[string]*instance{
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
					price: 0.1,
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
					price: 0.8,
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
				t.Errorf("catalog does not match, received: %+v, expected: %+v",
					asgInstanceManager.catalog,
					tt.expectedInstances)
			}
		})
	}
}

func TestPropagatedInstance(t *testing.T) {
	tests := []struct {
		name         string
		tagsASG      []*autoscaling.TagDescription
		expectedTags []*ec2.Tag
	}{
		{name: "no tags on asg",
			tagsASG: []*autoscaling.TagDescription{},
		},
		{name: "multiple tags but none to propagate",
			tagsASG: []*autoscaling.TagDescription{
				{
					Key:               aws.String("k1"),
					Value:             aws.String("v1"),
					PropagateAtLaunch: aws.Bool(false),
				},
				{
					Key:               aws.String("k2"),
					Value:             aws.String("v2"),
					PropagateAtLaunch: aws.Bool(false),
				},
				{
					Key:               aws.String("k3"),
					Value:             aws.String("v3"),
					PropagateAtLaunch: aws.Bool(false),
				},
			},
		},
		{name: "multiple tags but none to propagate",
			tagsASG: []*autoscaling.TagDescription{
				{
					Key:               aws.String("aws:k1"),
					Value:             aws.String("v1"),
					PropagateAtLaunch: aws.Bool(true),
				},
				{
					Key:               aws.String("k2"),
					Value:             aws.String("v2"),
					PropagateAtLaunch: aws.Bool(false),
				},
				{
					Key:               aws.String("k3"),
					Value:             aws.String("v3"),
					PropagateAtLaunch: aws.Bool(false),
				},
			},
		},
		{name: "multiple tags on asg - only one to propagate",
			tagsASG: []*autoscaling.TagDescription{
				{
					Key:               aws.String("k1"),
					Value:             aws.String("v1"),
					PropagateAtLaunch: aws.Bool(false),
				},
				{
					Key:               aws.String("k2"),
					Value:             aws.String("v2"),
					PropagateAtLaunch: aws.Bool(true),
				},
				{
					Key:               aws.String("aws:k3"),
					Value:             aws.String("v3"),
					PropagateAtLaunch: aws.Bool(true),
				},
			},
			expectedTags: []*ec2.Tag{
				{
					Key:   aws.String("k2"),
					Value: aws.String("v2"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &autoScalingGroup{
				Group: &autoscaling.Group{
					Tags: tt.tagsASG,
				},
			}
			tags := a.propagatedInstanceTags()
			if !reflect.DeepEqual(tags, tt.expectedTags) {
				t.Errorf("Tags received: %+v, expected: %+v", tags, tt.expectedTags)
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
		{name: "ASG has no 'running' instance in AZ",
			asgInstances: makeInstancesWithCatalog(
				map[string]*instance{
					"spot-stopped": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("stopped")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"spot-running": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"ondemand-stopped": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("stopped")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1c")},
							InstanceLifecycle: aws.String(""),
						},
					},
					"ondemand-running": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
							InstanceLifecycle: aws.String(""),
						},
					},
				},
			),
			az: aws.String("1c"),
		},
		{name: "ASG has 'running' instance in AZ",
			asgInstances: makeInstancesWithCatalog(
				map[string]*instance{
					"spot-stopped": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("stopped")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"spot-running": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"ondemand-stopped": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("stopped")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1c")},
							InstanceLifecycle: aws.String(""),
						},
					},
					"ondemand-running": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
							InstanceLifecycle: aws.String(""),
						},
					},
				},
			),
			az: aws.String("1b"),
			expected: &instance{
				Instance: &ec2.Instance{
					State:             &ec2.InstanceState{Name: aws.String("running")},
					Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
					InstanceLifecycle: aws.String(""),
				},
			},
		},
		{name: "ASG has no instance in AZ",
			asgInstances: makeInstancesWithCatalog(
				map[string]*instance{
					"spot-stopped": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("stopped")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"spot-running": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"ondemand-stopped": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("stopped")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1c")},
							InstanceLifecycle: aws.String(""),
						},
					},
					"ondemand-running": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
							InstanceLifecycle: aws.String(""),
						},
					},
				},
			),
			az: aws.String("2a"),
		},
		{name: "ASG has no instance at all",
			asgInstances: makeInstances(),
			az:           aws.String("1a"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &autoScalingGroup{
				instances: tt.asgInstances,
			}
			returnedInstance := a.getOnDemandInstanceInAZ(tt.az)
			if !reflect.DeepEqual(returnedInstance, tt.expected) {
				t.Errorf("instance does not match, received: %+v, expected: %+v",
					returnedInstance,
					tt.expected)
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
				map[string]*instance{
					"spot-stopped": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("stopped")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"spot-running": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"ondemand-stopped": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("stopped")},
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
				map[string]*instance{
					"spot-stopped": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("stopped")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"spot-running": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"ondemand-stopped": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("stopped")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1c")},
							InstanceLifecycle: aws.String(""),
						},
					},
					"ondemand-running": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
							InstanceLifecycle: aws.String(""),
						},
					},
				},
			),
			expected: []*instance{{
				Instance: &ec2.Instance{
					State:             &ec2.InstanceState{Name: aws.String("running")},
					Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
					InstanceLifecycle: aws.String(""),
				}},
			},
		},
		{name: "ASG has multiple 'running' OnDemand instances",
			asgInstances: makeInstancesWithCatalog(
				map[string]*instance{
					"spot-stopped": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("stopped")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"spot-running": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"ondemand-running1": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1c")},
							InstanceLifecycle: aws.String(""),
						},
					},
					"ondemand-running2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
							InstanceLifecycle: aws.String(""),
						},
					},
				},
			),
			expected: []*instance{
				{
					Instance: &ec2.Instance{
						State:             &ec2.InstanceState{Name: aws.String("running")},
						Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
						InstanceLifecycle: aws.String(""),
					},
				},
				{
					Instance: &ec2.Instance{
						State:             &ec2.InstanceState{Name: aws.String("running")},
						Placement:         &ec2.Placement{AvailabilityZone: aws.String("1c")},
						InstanceLifecycle: aws.String(""),
					},
				},
			},
		},
		{name: "ASG has no instance at all",
			asgInstances: makeInstancesWithCatalog(map[string]*instance{}),
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
				t.Errorf("instance does not match, received: %+v, expected: nil",
					returnedInstance)
			} else if len(tt.expected) != 0 {
				for _, i := range tt.expected {
					if reflect.DeepEqual(returnedInstance, i) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("instance does not match, received: %+v, expected to be in: %+v",
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
				map[string]*instance{
					"spot-stopped": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("stopped")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"ondemand-stopped": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("stopped")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
							InstanceLifecycle: aws.String(""),
						},
					},
					"ondemand-running": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
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
				map[string]*instance{
					"spot-stopped": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("stopped")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"spot-running": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"ondemand-stopped": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("stopped")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1c")},
							InstanceLifecycle: aws.String(""),
						},
					},
					"ondemand-running": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
							InstanceLifecycle: aws.String(""),
						},
					},
				},
			),
			expected: []*instance{{
				Instance: &ec2.Instance{
					State:             &ec2.InstanceState{Name: aws.String("running")},
					Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
					InstanceLifecycle: aws.String("spot"),
				}},
			},
		},
		{name: "ASG has multiple 'running' Spot instances",
			asgInstances: makeInstancesWithCatalog(
				map[string]*instance{
					"spot-running1": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"spot-running2": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"ondemand-stopped": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("stopped")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1c")},
							InstanceLifecycle: aws.String(""),
						},
					},
					"ondemand-running": {
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
							InstanceLifecycle: aws.String(""),
						},
					},
				},
			),
			expected: []*instance{
				{
					Instance: &ec2.Instance{
						State:             &ec2.InstanceState{Name: aws.String("running")},
						Placement:         &ec2.Placement{AvailabilityZone: aws.String("1a")},
						InstanceLifecycle: aws.String("spot"),
					},
				},
				{
					Instance: &ec2.Instance{
						State:             &ec2.InstanceState{Name: aws.String("running")},
						Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
						InstanceLifecycle: aws.String("spot"),
					},
				},
			},
		},
		{name: "ASG has no instance at all",
			asgInstances: makeInstancesWithCatalog(map[string]*instance{}),
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
				t.Errorf("instance does not match, received: %+v, expected: nil",
					returnedInstance)
			} else if len(tt.expected) != 0 {
				for _, i := range tt.expected {
					if reflect.DeepEqual(returnedInstance, i) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("instance does not match, received: %+v, expected to be in: %+v",
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
		spotId   *string
		expected error
	}{
		{name: "ondemand is replaced by spot instance - min/max/des identical",
			spotId:   aws.String("spot-running"),
			expected: nil,
			asg: &autoScalingGroup{
				name: "test-asg",
				Group: &autoscaling.Group{
					MaxSize:         aws.Int64(2),
					MinSize:         aws.Int64(2),
					DesiredCapacity: aws.Int64(2),
				},
				instances: makeInstancesWithCatalog(
					map[string]*instance{
						"ondemand-stopped": {
							Instance: &ec2.Instance{
								InstanceId:        aws.String("ondemand-stopped"),
								State:             &ec2.InstanceState{Name: aws.String("stopped")},
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
										aio:   nil,
										aierr: nil,
									},
								},
							},
						},
						"ondemand-running": {
							Instance: &ec2.Instance{
								InstanceId:        aws.String("ondemand-running"),
								State:             &ec2.InstanceState{Name: aws.String("running")},
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
				region: &region{
					name: "test-region",
					services: connections{
						autoScaling: &mockASG{
							uasgo:   nil,
							uasgerr: nil,
							dio:     nil,
							dierr:   nil,
						},
						ec2: &mockEC2{
							tio:   nil,
							tierr: nil,
						},
					},
					instances: makeInstancesWithCatalog(
						map[string]*instance{
							"spot-running": {
								Instance: &ec2.Instance{
									InstanceId:        aws.String("spot-running"),
									State:             &ec2.InstanceState{Name: aws.String("stopped")},
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
							"ondemand-stopped": {
								Instance: &ec2.Instance{
									InstanceId:        aws.String("ondemand-stopped"),
									State:             &ec2.InstanceState{Name: aws.String("stopped")},
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
									State:             &ec2.InstanceState{Name: aws.String("running")},
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
		{name: "ondemand is replaced by spot instance - min/max/des different",
			spotId:   aws.String("spot-running"),
			expected: nil,
			asg: &autoScalingGroup{
				name: "test-asg",
				Group: &autoscaling.Group{
					MaxSize:         aws.Int64(4),
					MinSize:         aws.Int64(1),
					DesiredCapacity: aws.Int64(2),
				},
				instances: makeInstancesWithCatalog(
					map[string]*instance{
						"ondemand-running": {
							Instance: &ec2.Instance{
								InstanceId:        aws.String("ondemand-running"),
								State:             &ec2.InstanceState{Name: aws.String("running")},
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
				region: &region{
					name: "test-region",
					services: connections{
						autoScaling: &mockASG{
							uasgo:   nil,
							uasgerr: nil,
							dio:     nil,
							dierr:   nil,
						},
						ec2: &mockEC2{
							tio:   nil,
							tierr: nil,
						},
					},
					instances: makeInstancesWithCatalog(
						map[string]*instance{
							"spot-running": {
								Instance: &ec2.Instance{
									InstanceId:        aws.String("spot-running"),
									State:             &ec2.InstanceState{Name: aws.String("stopped")},
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
			spotId:   aws.String("spot-not-found"),
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
					services: connections{
						autoScaling: &mockASG{
							uasgo:   nil,
							uasgerr: nil,
							dio:     nil,
							dierr:   nil,
						},
					},
					instances: makeInstancesWithCatalog(
						map[string]*instance{
							"spot-running": {
								Instance: &ec2.Instance{
									InstanceId:        aws.String("spot-running"),
									State:             &ec2.InstanceState{Name: aws.String("stopped")},
									Placement:         &ec2.Placement{AvailabilityZone: aws.String("1a")},
									InstanceLifecycle: aws.String("spot"),
								},
							},
							"ondemand-stopped": {
								Instance: &ec2.Instance{
									InstanceId:        aws.String("ondemand-stopped"),
									State:             &ec2.InstanceState{Name: aws.String("stopped")},
									Placement:         &ec2.Placement{AvailabilityZone: aws.String("1b")},
									InstanceLifecycle: aws.String(""),
								},
							},
							"ondemand-running": {
								Instance: &ec2.Instance{
									InstanceId:        aws.String("ondemand-running"),
									State:             &ec2.InstanceState{Name: aws.String("running")},
									Placement:         &ec2.Placement{AvailabilityZone: aws.String("1a")},
									InstanceLifecycle: aws.String(""),
								},
							},
						},
					),
				},
			},
		},
		{name: "no ondemand instances found in asg",
			spotId:   aws.String("spot-running"),
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
					services: connections{
						autoScaling: &mockASG{
							uasgo:   nil,
							uasgerr: nil,
							dio:     nil,
							dierr:   nil,
						},
					},
					instances: makeInstancesWithCatalog(
						map[string]*instance{
							"spot-running": {
								Instance: &ec2.Instance{
									InstanceId:        aws.String("spot-running"),
									State:             &ec2.InstanceState{Name: aws.String("stopped")},
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
			returned := tt.asg.replaceOnDemandInstanceWithSpot(tt.spotId)
			CheckErrors(t, returned, tt.expected)
		})
	}
}
