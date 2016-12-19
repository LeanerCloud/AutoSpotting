package autospotting

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
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
				t.Errorf("Value received for %s: %s expected %s", tt.tagKey, retValue, tt.expected)
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
			asgInstances:    instances{},
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
			asgInstances:    instances{},
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
			asgInstances:    instances{},
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
			asgInstances:    instances{},
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
			asgInstances: instances{
				catalog: map[string]*instance{
					"id-1": &instance{},
				},
			},
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
			asgInstances: instances{
				catalog: map[string]*instance{
					"id-1": &instance{},
					"id-2": &instance{},
					"id-3": &instance{},
				},
			},
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
			asgInstances: instances{
				catalog: map[string]*instance{
					"id-1": &instance{},
					"id-2": &instance{},
					"id-3": &instance{},
				},
			},
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
			asgInstances: instances{
				catalog: map[string]*instance{
					"id-1": &instance{},
					"id-2": &instance{},
					"id-3": &instance{},
				},
			},
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
			asgInstances:    instances{},
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
			asgInstances:    instances{},
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
			asgInstances:    instances{},
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
			asgInstances: instances{
				catalog: map[string]*instance{
					"id-1": &instance{},
					"id-2": &instance{},
					"id-3": &instance{},
				},
			},
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
			asgInstances: instances{
				catalog: map[string]*instance{
					"id-1": &instance{},
					"id-2": &instance{},
					"id-3": &instance{},
				},
			},
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
			asgInstances: instances{
				catalog: map[string]*instance{
					"id-1": &instance{},
					"id-2": &instance{},
					"id-3": &instance{},
					"id-4": &instance{},
				},
			},
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
			asgInstances: instances{
				catalog: map[string]*instance{
					"id-1": &instance{},
					"id-2": &instance{},
					"id-3": &instance{},
					"id-4": &instance{},
				},
			},
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
			asgInstances:    instances{},
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
				t.Errorf("loadConfOnDemand returned: %b expected %b", done, tt.loadingExpected)
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
			asgInstances:    instances{},
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
			asgInstances:    instances{},
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
			asgInstances:    instances{},
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
			asgInstances: instances{
				catalog: map[string]*instance{
					"id-1": &instance{},
					"id-2": &instance{},
					"id-3": &instance{},
				},
			},
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
			asgInstances: instances{
				catalog: map[string]*instance{
					"id-1": &instance{},
					"id-2": &instance{},
					"id-3": &instance{},
				},
			},
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
			asgInstances: instances{
				catalog: map[string]*instance{
					"id-1": &instance{},
					"id-2": &instance{},
					"id-3": &instance{},
				},
			},
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
			asgInstances:    instances{},
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
			asgInstances: instances{
				catalog: map[string]*instance{
					"id-1": &instance{},
					"id-2": &instance{},
					"id-3": &instance{},
				},
			},
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
			asgInstances: instances{
				catalog: map[string]*instance{
					"id-1": &instance{},
					"id-2": &instance{},
					"id-3": &instance{},
				},
			},
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
			asgInstances: instances{
				catalog: map[string]*instance{
					"id-1": &instance{},
					"id-2": &instance{},
					"id-3": &instance{},
					"id-4": &instance{},
				},
			},
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
			asgInstances: instances{
				catalog: map[string]*instance{
					"id-1": &instance{},
					"id-2": &instance{},
					"id-3": &instance{},
					"id-4": &instance{},
				},
			},
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
			asgInstances:    instances{},
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
				t.Errorf("loadConfOnDemand returned: %b expected %b", done, tt.loadingExpected)
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
			asgInstances:    instances{},
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
			asgInstances: instances{
				catalog: map[string]*instance{
					"id-1": &instance{},
					"id-2": &instance{},
					"id-3": &instance{},
					"id-4": &instance{},
				},
			},
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
				t.Errorf("loadConfOnDemand returned: %b expected %b", done, tt.loadingExpected)
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
			asgInstances:     instances{},
			spot:             true,
			availabilityZone: "",
			expectedCount:    0,
			expectedTotal:    0,
		},
		{name: "ASG has no 'running' instance but has some",
			asgName: "test-asg",
			asgInstances: instances{
				catalog: map[string]*instance{
					"id-1": &instance{
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("stopped")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
				},
			},
			spot:             true,
			availabilityZone: "",
			expectedCount:    0,
			expectedTotal:    0,
		},
		{name: "ASG has no 'running' spot instances but has some",
			asgName: "test-asg",
			asgInstances: instances{
				catalog: map[string]*instance{
					"id-1": &instance{
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
					"id-2": &instance{
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
				},
			},
			spot:             true,
			availabilityZone: "",
			expectedCount:    0,
			expectedTotal:    2,
		},
		{name: "ASG has no 'running' on-demand instances but has some",
			asgName: "test-asg",
			asgInstances: instances{
				catalog: map[string]*instance{
					"id-1": &instance{
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"id-2": &instance{
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
				},
			},
			spot:             false,
			availabilityZone: "",
			expectedCount:    0,
			expectedTotal:    2,
		},
		{name: "ASG has no 'running' on-demand instances in the AZ",
			asgName: "test-asg",
			asgInstances: instances{
				catalog: map[string]*instance{
					"id-1": &instance{
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
					"id-2": &instance{
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
				},
			},
			spot:             false,
			availabilityZone: "eu-west-1c",
			expectedCount:    0,
			expectedTotal:    2,
		},
		{name: "ASG has some 'running' on-demand instances in the AZ",
			asgName: "test-asg",
			asgInstances: instances{
				catalog: map[string]*instance{
					"id-1": &instance{
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
					"id-2": &instance{
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
				},
			},
			spot:             false,
			availabilityZone: "eu-west-1b",
			expectedCount:    1,
			expectedTotal:    2,
		},
		{name: "ASG has no 'running' spot instances in the AZ",
			asgName: "test-asg",
			asgInstances: instances{
				catalog: map[string]*instance{
					"id-1": &instance{
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"id-2": &instance{
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
				},
			},
			spot:             true,
			availabilityZone: "eu-west-1c",
			expectedCount:    0,
			expectedTotal:    2,
		},
		{name: "ASG has some 'running' spot instances in any AZ",
			asgName: "test-asg",
			asgInstances: instances{
				catalog: map[string]*instance{
					"id-1": &instance{
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"id-2": &instance{
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
				},
			},
			spot:             true,
			availabilityZone: "",
			expectedCount:    2,
			expectedTotal:    2,
		},
		{name: "ASG has no 'running' spot instances in any AZ",
			asgName: "test-asg",
			asgInstances: instances{
				catalog: map[string]*instance{
					"id-1": &instance{
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("shutting-down")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"id-2": &instance{
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
				},
			},
			spot:             true,
			availabilityZone: "",
			expectedCount:    0,
			expectedTotal:    1,
		},
		{name: "ASG has some 'running' on-demand instances in any AZ",
			asgName: "test-asg",
			asgInstances: instances{
				catalog: map[string]*instance{
					"id-1": &instance{
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"id-2": &instance{
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
				},
			},
			spot:             false,
			availabilityZone: "",
			expectedCount:    1,
			expectedTotal:    2,
		},
		{name: "ASG has no 'running' on-demand instances in any AZ",
			asgName: "test-asg",
			asgInstances: instances{
				catalog: map[string]*instance{
					"id-1": &instance{
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"id-2": &instance{
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("stopped")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
				},
			},
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
				t.Errorf("alreadyRunningInstanceCount returned count: %b expected %b",
					count, tt.expectedCount)
			} else if tt.expectedTotal != total {
				t.Errorf("alreadyRunningInstanceCount returned total: %b expected %b",
					total, tt.expectedTotal)
			}
		})
	}
}

type mockEC2Client struct {
	ec2iface.EC2API
}

func (m *mockEC2Client) TerminateInstances(*ec2.TerminateInstancesInput) (*ec2.TerminateInstancesOutput, error) {
	return nil, nil
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
			asgInstances:    instances{},
			minOnDemand:     1,
			desiredCapacity: aws.Int64(0),
			expectedRun:     false,
		},
		{name: "ASG has no instance at all - 0 on-demand required",
			asgInstances:    instances{},
			minOnDemand:     0,
			desiredCapacity: aws.Int64(0),
			expectedRun:     false,
		},
		{name: "ASG has no instance running - 1 on-demand required",
			asgInstances: instances{
				catalog: map[string]*instance{
					"id-1": &instance{
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("shutting-down")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"id-2": &instance{
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("shutting-down")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
				},
			},
			minOnDemand:     1,
			desiredCapacity: aws.Int64(0),
			expectedRun:     false,
		},
		{name: "ASG has no instance running - 0 on-demand required",
			asgInstances: instances{
				catalog: map[string]*instance{
					"id-1": &instance{
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("shutting-down")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"id-2": &instance{
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("shutting-down")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
				},
			},
			minOnDemand:     0,
			desiredCapacity: aws.Int64(0),
			expectedRun:     false,
		},
		{name: "ASG has not the required on-demand running",
			asgInstances: instances{
				catalog: map[string]*instance{
					"id-1": &instance{
						Instance: &ec2.Instance{
							InstanceId:        aws.String("id-1"),
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("spot"),
						},
						region: &region{
							name: "test-region",
							services: connections{
								ec2: &mockEC2Client{},
							},
						},
					},
					"id-2": &instance{
						Instance: &ec2.Instance{
							InstanceId:        aws.String("id-2"),
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
				},
			},
			minOnDemand:     2,
			desiredCapacity: aws.Int64(0),
			expectedRun:     false,
		},
		{name: "ASG has just enough on-demand instances running",
			asgInstances: instances{
				catalog: map[string]*instance{
					"id-1": &instance{
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
					"id-2": &instance{
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
				},
			},
			minOnDemand:     1,
			desiredCapacity: aws.Int64(0),
			expectedRun:     false,
		},
		{name: "ASG has more than enough on-demand instances running but not desired capacity",
			asgInstances: instances{
				catalog: map[string]*instance{
					"id-1": &instance{
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
					"id-2": &instance{
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
				},
			},
			minOnDemand:     1,
			desiredCapacity: aws.Int64(1),
			expectedRun:     true,
		},
		{name: "ASG has more than enough on-demand instances running and desired capacity",
			asgInstances: instances{
				catalog: map[string]*instance{
					"id-1": &instance{
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
					"id-2": &instance{
						Instance: &ec2.Instance{
							State:             &ec2.InstanceState{Name: aws.String("running")},
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1b")},
							InstanceLifecycle: aws.String("on-demand"),
						},
					},
				},
			},
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
				t.Errorf("needReplaceOnDemandInstances returned: %b expected %b",
					shouldRun, tt.expectedRun)
			}
		})
	}
}