package autospotting

import (
	"math"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/cristim/ec2-instances-info"
)

func Test_region_enabled(t *testing.T) {

	tests := []struct {
		name    string
		region  string
		allowed string
		want    bool
	}{
		{
			name:    "No regions given in the filter",
			region:  "us-east-1",
			allowed: "",
			want:    true,
		},
		{
			name:    "Running in a different region than one allowed one",
			region:  "us-east-1",
			allowed: "eu-west-1",
			want:    false,
		},

		{
			name:    "Running in a different region than a list of allowed ones",
			region:  "us-east-1",
			allowed: "eu-west-1 ca-central-1",
			want:    false,
		},
		{
			name:    "Running in a region from the allowed ones",
			region:  "us-east-1",
			allowed: "us-east-1 eu-west-1",
			want:    true,
		},
		{
			name:    "Comma-separated allowed regions",
			region:  "us-east-1",
			allowed: "us-east-1,eu-west-1",
			want:    true,
		},
		{
			name:    "Comma and whitespace-separated allowed regions",
			region:  "us-east-1",
			allowed: "us-east-1, eu-west-1",
			want:    true,
		},
		{
			name:    "Whitespace-and-comma-separated allowed regions",
			region:  "us-east-1",
			allowed: "us-east-1, eu-west-1",
			want:    true,
		},
		{
			name:    "Region globs matching",
			region:  "us-east-1",
			allowed: "us-*, eu-*",
			want:    true,
		},
		{
			name:    "Region globs not matching",
			region:  "us-east-1",
			allowed: "ap-*, eu-*",
			want:    false,
		},
		{
			name:    "Region globs without dash matching",
			region:  "us-east-1",
			allowed: "us*, eu*",
			want:    true,
		},
		{
			name:    "Region globs without dash not matching",
			region:  "us-east-1",
			allowed: "ap*, eu*",
			want:    false,
		},
		{
			name:    "Non-separated allowed regions",
			region:  "us-east-1",
			allowed: "us-east-1eu-west-1",
			want:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &region{
				name: tt.region,
				conf: &Config{
					Regions: tt.allowed,
				},
			}
			if got := r.enabled(); got != tt.want {
				t.Errorf("region.enabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAsgFiltersSetupOnRegion(t *testing.T) {
	tests := []struct {
		name    string
		want    []Tag
		tregion *region
	}{
		{
			name: "No tags specified",
			want: []Tag{{Key: "spot-enabled", Value: "true"}},
			tregion: &region{
				conf: &Config{},
			},
		},
		{
			name: "No tags specified",
			want: []Tag{{Key: "spot-enabled", Value: "true"}, {Key: "environment", Value: "dev"}},
			tregion: &region{
				conf: &Config{
					FilterByTags: "spot-enabled=true, environment=dev",
				},
			},
		},
		{
			name: "No tags specified",
			want: []Tag{{Key: "spot-enabled", Value: "true"}, {Key: "environment", Value: "dev"}, {Key: "team", Value: "interactive"}},
			tregion: &region{
				conf: &Config{
					FilterByTags: "spot-enabled=true, environment=dev,team=interactive",
				},
			},
		},
	}
	for _, tt := range tests {

		tt.tregion.setupAsgFilters()
		if !reflect.DeepEqual(tt.want, tt.tregion.tagsToFilterASGsBy) {
			t.Errorf("region.setupAsgFilters() = %v, want %v", tt.tregion.tagsToFilterASGsBy, tt.want)

		}

	}
}

func TestOnDemandPriceMultiplier(t *testing.T) {
	tests := []struct {
		multiplier float64
		want       float64
	}{
		{
			multiplier: 1.0,
			want:       0.044,
		},
		{
			multiplier: 2.0,
			want:       0.088,
		},
		{
			multiplier: 0.99,
			want:       0.04356,
		},
	}
	for _, tt := range tests {
		cfg := &Config{
			InstanceData: &ec2instancesinfo.InstanceData{
				0: {
					InstanceType: "m1.small",
					Pricing: map[string]ec2instancesinfo.RegionPrices{
						"us-east-1": {
							Linux: ec2instancesinfo.LinuxPricing{
								OnDemand: 0.044,
							},
						},
					},
				},
			},
			OnDemandPriceMultiplier: tt.multiplier,
		}
		r := region{
			name: "us-east-1",
			conf: cfg,
			services: connections{
				ec2: mockEC2{
					dspho: &ec2.DescribeSpotPriceHistoryOutput{
						SpotPriceHistory: []*ec2.SpotPrice{},
					},
					dspherr: nil,
				},
			}}
		r.determineInstanceTypeInformation(cfg)

		actualPrice := r.instanceTypeInformation["m1.small"].pricing.onDemand
		if math.Abs(actualPrice-tt.want) > 0.000001 {
			t.Errorf("multiplier = %.2f, pricing.onDemand = %.5f, want %.5f",
				tt.multiplier, actualPrice, tt.want)
		}
	}
}

func TestDefaultASGFiltering(t *testing.T) {
	tests := []struct {
		tregion  *region
		expected []Tag
	}{
		{
			expected: []Tag{{Key: "spot-enabled", Value: "true"}},
			tregion: &region{
				conf: &Config{
					FilterByTags: "bob",
				},
			},
		},
		{
			expected: []Tag{{Key: "bob", Value: "value"}},
			tregion: &region{
				conf: &Config{
					FilterByTags: "bob=value",
				},
			},
		},
		{
			expected: []Tag{{Key: "spot-enabled", Value: "true"}, {Key: "team", Value: "interactive"}},
			tregion: &region{
				conf: &Config{
					FilterByTags: "spot-enabled=true,team=interactive",
				},
			},
		},
	}
	for _, tt := range tests {
		tt.tregion.setupAsgFilters()
		for _, tag := range tt.expected {
			matchingTag := false
			for _, setTag := range tt.tregion.tagsToFilterASGsBy {
				if tag.Key == setTag.Key && tag.Value == setTag.Value {
					matchingTag = true
				}
			}

			if !matchingTag {
				t.Errorf("tags not correctly filtered = %v, want %v", tt.tregion.tagsToFilterASGsBy, tt.expected)

			}
		}
	}
}

func TestFilterAsgs(t *testing.T) {
	tests := []struct {
		name    string
		want    []string
		tregion *region
	}{
		{
			name: "Test with single filter",
			want: []string{"asg1", "asg2", "asg3", "asg4"},
			tregion: &region{
				tagsToFilterASGsBy: []Tag{{Key: "spot-enabled", Value: "true"}},
				conf:               &Config{},
				services: connections{
					autoScaling: mockASG{
						dasgo: &autoscaling.DescribeAutoScalingGroupsOutput{
							AutoScalingGroups: []*autoscaling.Group{
								{
									Tags: []*autoscaling.TagDescription{
										{Key: aws.String("environment"), Value: aws.String("dev"), ResourceId: aws.String("asg1")},
										{Key: aws.String("spot-enabled"), Value: aws.String("true"), ResourceId: aws.String("asg1")},
									},
									AutoScalingGroupName: aws.String("asg1"),
								},
								{
									Tags: []*autoscaling.TagDescription{
										{Key: aws.String("environment"), Value: aws.String("dev"), ResourceId: aws.String("asg2")},
										{Key: aws.String("spot-enabled"), Value: aws.String("true"), ResourceId: aws.String("asg2")},
									},
									AutoScalingGroupName: aws.String("asg2"),
								},
								{
									Tags: []*autoscaling.TagDescription{
										{Key: aws.String("environment"), Value: aws.String("qa"), ResourceId: aws.String("asg3")},
										{Key: aws.String("spot-enabled"), Value: aws.String("true"), ResourceId: aws.String("asg3")},
									},
									AutoScalingGroupName: aws.String("asg3"),
								},
								{
									Tags: []*autoscaling.TagDescription{
										{Key: aws.String("environment"), Value: aws.String("qa"), ResourceId: aws.String("asg4")},
										{Key: aws.String("spot-enabled"), Value: aws.String("true"), ResourceId: aws.String("asg4")},
									},
									AutoScalingGroupName: aws.String("asg4"),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Test opt-out mode",
			// Run on all groups except for those tagged with spot-enabled=false
			want: []string{"asg2", "asg3"},
			tregion: &region{
				tagsToFilterASGsBy: []Tag{{Key: "spot-enabled", Value: "false"}},
				conf:               &Config{TagFilteringMode: "opt-out"},
				services: connections{
					autoScaling: mockASG{
						dasgo: &autoscaling.DescribeAutoScalingGroupsOutput{
							AutoScalingGroups: []*autoscaling.Group{
								{
									Tags: []*autoscaling.TagDescription{
										{Key: aws.String("environment"), Value: aws.String("dev"), ResourceId: aws.String("asg1")},
										{Key: aws.String("spot-enabled"), Value: aws.String("false"), ResourceId: aws.String("asg1")},
									},
									AutoScalingGroupName: aws.String("asg1"),
								},
								{
									Tags: []*autoscaling.TagDescription{
										{Key: aws.String("environment"), Value: aws.String("dev"), ResourceId: aws.String("asg2")},
										{Key: aws.String("spot-enabled"), Value: aws.String("true"), ResourceId: aws.String("asg2")},
									},
									AutoScalingGroupName: aws.String("asg2"),
								},
								{
									Tags: []*autoscaling.TagDescription{
										{Key: aws.String("environment"), Value: aws.String("qa"), ResourceId: aws.String("asg3")},
									},
									AutoScalingGroupName: aws.String("asg3"),
								},
								{
									Tags: []*autoscaling.TagDescription{
										{Key: aws.String("environment"), Value: aws.String("qa"), ResourceId: aws.String("asg4")},
										{Key: aws.String("spot-enabled"), Value: aws.String("false"), ResourceId: aws.String("asg4")},
									},
									AutoScalingGroupName: aws.String("asg4"),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Test opt-out mode with multiple tag filters",
			// Run on all groups except for those tagged with spot-enabled=false and
			// environment=dev, regardless of other tags that may be set
			want: []string{"asg2", "asg3", "asg4"},
			tregion: &region{
				tagsToFilterASGsBy: []Tag{
					{Key: "spot-enabled", Value: "false"},
					{Key: "environment", Value: "dev"},
				},
				conf: &Config{TagFilteringMode: "opt-out"},
				services: connections{
					autoScaling: mockASG{
						dasgo: &autoscaling.DescribeAutoScalingGroupsOutput{
							AutoScalingGroups: []*autoscaling.Group{
								{
									Tags: []*autoscaling.TagDescription{
										{Key: aws.String("spot-enabled"), Value: aws.String("false"), ResourceId: aws.String("asg1")},
										{Key: aws.String("environment"), Value: aws.String("dev"), ResourceId: aws.String("asg1")},
										{Key: aws.String("team"), Value: aws.String("awesome"), ResourceId: aws.String("asg1")},
									},
									AutoScalingGroupName: aws.String("asg1"),
								},
								{
									Tags: []*autoscaling.TagDescription{
										{Key: aws.String("environment"), Value: aws.String("dev"), ResourceId: aws.String("asg2")},
										{Key: aws.String("spot-enabled"), Value: aws.String("true"), ResourceId: aws.String("asg2")},
										{Key: aws.String("team"), Value: aws.String("awesome"), ResourceId: aws.String("asg2")},
									},
									AutoScalingGroupName: aws.String("asg2"),
								},
								{
									Tags: []*autoscaling.TagDescription{
										{Key: aws.String("spot-enabled"), Value: aws.String("false"), ResourceId: aws.String("asg3")},
										{Key: aws.String("environment"), Value: aws.String("qa"), ResourceId: aws.String("asg3")},
										{Key: aws.String("team"), Value: aws.String("awesome"), ResourceId: aws.String("asg3")},
									},
									AutoScalingGroupName: aws.String("asg3"),
								},
								{
									Tags: []*autoscaling.TagDescription{
										{Key: aws.String("environment"), Value: aws.String("qa"), ResourceId: aws.String("asg4")},
										{Key: aws.String("spot-enabled"), Value: aws.String("true"), ResourceId: aws.String("asg4")},
										{Key: aws.String("team"), Value: aws.String("awesome"), ResourceId: aws.String("asg4")},
									},
									AutoScalingGroupName: aws.String("asg4"),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Test with two filters",
			want: []string{"asg3", "asg4"},
			tregion: &region{
				tagsToFilterASGsBy: []Tag{{Key: "spot-enabled", Value: "true"}, {Key: "environment", Value: "qa"}},
				conf:               &Config{},
				services: connections{
					autoScaling: mockASG{
						dasgo: &autoscaling.DescribeAutoScalingGroupsOutput{
							AutoScalingGroups: []*autoscaling.Group{
								{
									Tags: []*autoscaling.TagDescription{
										{Key: aws.String("environment"), Value: aws.String("dev"), ResourceId: aws.String("asg1")},
										{Key: aws.String("spot-enabled"), Value: aws.String("true"), ResourceId: aws.String("asg1")},
									},
									AutoScalingGroupName: aws.String("asg1"),
								},
								{
									Tags: []*autoscaling.TagDescription{
										{Key: aws.String("environment"), Value: aws.String("dev"), ResourceId: aws.String("asg2")},
										{Key: aws.String("spot-enabled"), Value: aws.String("true"), ResourceId: aws.String("asg2")},
									},
									AutoScalingGroupName: aws.String("asg2"),
								},
								{
									Tags: []*autoscaling.TagDescription{
										{Key: aws.String("environment"), Value: aws.String("qa"), ResourceId: aws.String("asg3")},
										{Key: aws.String("spot-enabled"), Value: aws.String("true"), ResourceId: aws.String("asg3")},
									},
									AutoScalingGroupName: aws.String("asg3"),
								},
								{
									Tags: []*autoscaling.TagDescription{
										{Key: aws.String("environment"), Value: aws.String("qa"), ResourceId: aws.String("asg4")},
										{Key: aws.String("spot-enabled"), Value: aws.String("true"), ResourceId: aws.String("asg4")},
									},
									AutoScalingGroupName: aws.String("asg4"),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Test with multiple secondary filter",
			want: []string{"asg4"},
			tregion: &region{
				tagsToFilterASGsBy: []Tag{
					{Key: "spot-enabled", Value: "true"},
					{Key: "environment", Value: "qa"},
					{Key: "team", Value: "interactive"},
				},
				conf: &Config{},
				services: connections{
					autoScaling: mockASG{
						dasgo: &autoscaling.DescribeAutoScalingGroupsOutput{
							AutoScalingGroups: []*autoscaling.Group{
								{
									Tags: []*autoscaling.TagDescription{
										{Key: aws.String("environment"), Value: aws.String("dev"), ResourceId: aws.String("asg1")},
										{Key: aws.String("spot-enabled"), Value: aws.String("true"), ResourceId: aws.String("asg1")},
									},
									AutoScalingGroupName: aws.String("asg1"),
								},
								{
									Tags: []*autoscaling.TagDescription{
										{Key: aws.String("environment"), Value: aws.String("dev"), ResourceId: aws.String("asg2")},
										{Key: aws.String("spot-enabled"), Value: aws.String("true"), ResourceId: aws.String("asg2")},
									},
									AutoScalingGroupName: aws.String("asg2"),
								},
								{
									Tags: []*autoscaling.TagDescription{
										{Key: aws.String("environment"), Value: aws.String("qa"), ResourceId: aws.String("asg3")},
										{Key: aws.String("spot-enabled"), Value: aws.String("true"), ResourceId: aws.String("asg3")},
									},
									AutoScalingGroupName: aws.String("asg3"),
								},
								{
									Tags: []*autoscaling.TagDescription{
										{Key: aws.String("environment"), Value: aws.String("qa"), ResourceId: aws.String("asg4")},
										{Key: aws.String("spot-enabled"), Value: aws.String("true"), ResourceId: aws.String("asg4")},
										{Key: aws.String("team"), Value: aws.String("interactive"), ResourceId: aws.String("asg4")},
									},
									AutoScalingGroupName: aws.String("asg4"),
								},
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.tregion
			r.scanForEnabledAutoScalingGroups()
			var asgNames []string
			for _, name := range r.enabledASGs {
				asgNames = append(asgNames, name.name)
			}
			if !reflect.DeepEqual(tt.want, asgNames) {
				t.Errorf("region.scanForEnabledAutoScalingGroups() = %v, want %v", asgNames, tt.want)
			}
		})
	}
}
