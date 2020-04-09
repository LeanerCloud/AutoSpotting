// Copyright (c) 2016-2019 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
)

func TestLoadSpotPriceBufferPercentage(t *testing.T) {
	tests := []struct {
		tagValue        *string
		loadingExpected bool
		valueExpected   float64
	}{
		{
			tagValue:        aws.String("5.0"),
			valueExpected:   5.0,
			loadingExpected: true,
		},
		{
			tagValue:        aws.String("TEST"),
			valueExpected:   10.0,
			loadingExpected: false,
		},
		{
			tagValue:        aws.String("-10.0"),
			valueExpected:   10.0,
			loadingExpected: false,
		},
		{
			tagValue:        aws.String("0"),
			valueExpected:   0.0,
			loadingExpected: true,
		},
	}
	for _, tt := range tests {
		a := autoScalingGroup{Group: &autoscaling.Group{}}
		value, loading := a.loadSpotPriceBufferPercentage(tt.tagValue)

		if value != tt.valueExpected || loading != tt.loadingExpected {
			t.Errorf("LoadBiddingPolicy returned: %f, expected: %f", value, tt.valueExpected)
		}

	}
}

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
				t.Errorf("getTagValue received for %s: %s expected %s", tt.tagKey, *retValue, *tt.expected)
			} else if tt.expected != nil && *retValue != *tt.expected {
				t.Errorf("getTagValue received for %s: %s expected %s", tt.tagKey, *retValue, *tt.expected)
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
					Key:   aws.String(OnDemandPercentageTag),
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
					Key:   aws.String(OnDemandPercentageTag),
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
					Key:   aws.String(OnDemandPercentageTag),
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
					Key:   aws.String(OnDemandPercentageTag),
					Value: aws.String("0"),
				},
			},
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
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
					Key:   aws.String(OnDemandPercentageTag),
					Value: aws.String("33.0"),
				},
			},
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
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
					Key:   aws.String(OnDemandPercentageTag),
					Value: aws.String("75.0"),
				},
			},
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
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
					Key:   aws.String(OnDemandPercentageTag),
					Value: aws.String("100.0"),
				},
			},
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
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
				instanceMap{
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
				instanceMap{
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
					Key:   aws.String(OnDemandPercentageTag),
					Value: aws.String("75"),
				},
				{
					Key:   aws.String(OnDemandNumberLong),
					Value: aws.String("2"),
				},
			},
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
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
					Key:   aws.String(OnDemandPercentageTag),
					Value: aws.String("75"),
				},
				{
					Key:   aws.String(OnDemandNumberLong),
					Value: aws.String("-2"),
				},
			},
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
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
					Key:   aws.String(OnDemandPercentageTag),
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

func TestLoadBiddingPolicy(t *testing.T) {
	tests := []struct {
		name          string
		tagValue      *string
		valueExpected string
	}{
		{name: "Loading a false tag",
			tagValue:      aws.String("aggressive"),
			valueExpected: "aggressive",
		},
		{name: "Loading a true tag",
			tagValue:      aws.String("normal"),
			valueExpected: "normal",
		},
		{name: "Loading a fake tag",
			tagValue:      aws.String("autospotting"),
			valueExpected: "normal",
		},
	}
	for _, tt := range tests {
		a := autoScalingGroup{Group: &autoscaling.Group{}}
		value, _ := a.loadBiddingPolicy(tt.tagValue)

		if value != tt.valueExpected {
			t.Errorf("LoadBiddingPolicy returned: %s, expected: %s", value, tt.valueExpected)
		}

	}
}

func TestLoadConfSpot(t *testing.T) {
	tests := []struct {
		name            string
		asgTags         []*autoscaling.TagDescription
		loadingExpected bool
		valueExpected   string
	}{
		{name: "Loading a fake tag",
			asgTags: []*autoscaling.TagDescription{
				{
					Key:   aws.String("Name"),
					Value: aws.String("asg-test"),
				},
			},
			loadingExpected: false,
			valueExpected:   "normal",
		},
		{name: "Loading a false tag",
			asgTags: []*autoscaling.TagDescription{
				{
					Key:   aws.String("Name"),
					Value: aws.String("asg-test"),
				},
				{
					Key:   aws.String(BiddingPolicyTag),
					Value: aws.String("aggressive"),
				},
			},
			loadingExpected: true,
			valueExpected:   "aggressive",
		},
		{name: "Loading a true tag",
			asgTags: []*autoscaling.TagDescription{
				{
					Key:   aws.String("Name"),
					Value: aws.String("asg-test"),
				},
				{
					Key:   aws.String(BiddingPolicyTag),
					Value: aws.String("normal"),
				},
			},
			loadingExpected: false,
			valueExpected:   "normal",
		},
	}
	for _, tt := range tests {
		cfg := &Config{
			AutoScalingConfig: AutoScalingConfig{
				BiddingPolicy: "normal",
			}}
		a := autoScalingGroup{Group: &autoscaling.Group{},
			region: &region{
				name: "us-east-1",
				conf: cfg,
			},
		}
		a.Tags = tt.asgTags
		done := a.loadConfSpot()
		if tt.loadingExpected != done {
			t.Errorf("LoadSpotConf retured: %t expected %t", done, tt.loadingExpected)
		} else if tt.valueExpected != a.region.conf.BiddingPolicy {
			t.Errorf("LoadSpotConf loaded: %s expected %s", a.region.conf.BiddingPolicy, tt.valueExpected)
		}

	}

}

func TestLoadConfSpotPrice(t *testing.T) {
	tests := []struct {
		name            string
		asgTags         []*autoscaling.TagDescription
		loadingExpected bool
		valueExpected   float64
	}{
		{name: "Loading a fake tag",
			asgTags: []*autoscaling.TagDescription{
				{
					Key:   aws.String("Name"),
					Value: aws.String("asg-test"),
				},
			},
			loadingExpected: false,
			valueExpected:   10.0,
		},
		{name: "Loading the right tag",
			asgTags: []*autoscaling.TagDescription{
				{
					Key:   aws.String("Name"),
					Value: aws.String("asg-test"),
				},
				{
					Key:   aws.String(SpotPriceBufferPercentageTag),
					Value: aws.String("15.0"),
				},
			},
			loadingExpected: true,
			valueExpected:   15.0,
		},
		{name: "Loading a false tag",
			asgTags: []*autoscaling.TagDescription{
				{
					Key:   aws.String("Name"),
					Value: aws.String("asg-test"),
				},
				{
					Key:   aws.String(SpotPriceBufferPercentageTag),
					Value: aws.String("-50.0"),
				},
			},
			loadingExpected: false,
			valueExpected:   10.0,
		},
	}
	for _, tt := range tests {
		cfg := &Config{
			AutoScalingConfig: AutoScalingConfig{
				SpotPriceBufferPercentage: 10.0,
			}}
		a := autoScalingGroup{Group: &autoscaling.Group{},
			region: &region{
				name: "us-east-1",
				conf: cfg,
			},
		}
		a.Tags = tt.asgTags
		done := a.loadConfSpotPrice()
		if tt.loadingExpected != done {
			t.Errorf("LoadSpotConf retured: %t expected %t", done, tt.loadingExpected)
		} else if tt.valueExpected != a.region.conf.SpotPriceBufferPercentage {
			t.Errorf("LoadSpotConf loaded: %f expected %f", a.region.conf.SpotPriceBufferPercentage, tt.valueExpected)
		}

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
					Key:   aws.String(OnDemandPercentageTag),
					Value: aws.String("text"),
				},
				{
					Key:   aws.String(BiddingPolicyTag),
					Value: aws.String("Autospotting"),
				},
				{
					Key:   aws.String(SpotPriceBufferPercentageTag),
					Value: aws.String("-15.0"),
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
					Key:   aws.String(OnDemandPercentageTag),
					Value: aws.String("75"),
				},
				{
					Key:   aws.String(OnDemandNumberLong),
					Value: aws.String("-2"),
				},
				{
					Key:   aws.String(BiddingPolicyTag),
					Value: aws.String("normal"),
				},
				{
					Key:   aws.String(SpotPriceBufferPercentageTag),
					Value: aws.String("15.0"),
				},
			},
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
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
			cfg := &Config{
				AutoScalingConfig: AutoScalingConfig{
					BiddingPolicy:             "normal",
					SpotPriceBufferPercentage: 10.0,
				}}
			a := autoScalingGroup{Group: &autoscaling.Group{},
				region: &region{
					name: "us-east-1",
					conf: cfg,
				},
			}
			a.Tags = tt.asgTags
			a.instances = tt.asgInstances
			a.MaxSize = tt.maxSize

			done := a.loadConfigFromTags()
			if tt.loadingExpected != done {
				t.Errorf("loadConfigFromTags returned: %t expected %t", done, tt.loadingExpected)
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
				conf: &Config{
					AutoScalingConfig: AutoScalingConfig{
						MinOnDemandNumber:     0,
						MinOnDemandPercentage: 0.0,
					}},
			},
			asgInstances:    makeInstances(),
			maxSize:         aws.Int64(10),
			numberExpected:  DefaultMinOnDemandValue,
			loadingExpected: false,
		},
		{name: "Percentage value out of range (0-100)",
			region: &region{
				conf: &Config{
					AutoScalingConfig: AutoScalingConfig{
						MinOnDemandNumber:     0,
						MinOnDemandPercentage: 142.2,
					}},
			},
			asgInstances:    makeInstances(),
			maxSize:         aws.Int64(10),
			numberExpected:  DefaultMinOnDemandValue,
			loadingExpected: false,
		},
		{name: "Percentage value out of range - negative (0-100)",
			region: &region{
				conf: &Config{
					AutoScalingConfig: AutoScalingConfig{
						MinOnDemandNumber:     0,
						MinOnDemandPercentage: -22.2,
					}},
			},
			asgInstances:    makeInstances(),
			maxSize:         aws.Int64(10),
			numberExpected:  DefaultMinOnDemandValue,
			loadingExpected: false,
		},
		{name: "Percentage equals 33.0%",
			region: &region{
				conf: &Config{
					AutoScalingConfig: AutoScalingConfig{
						MinOnDemandNumber:     0,
						MinOnDemandPercentage: 33.0,
					}},
			},
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
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
				conf: &Config{
					AutoScalingConfig: AutoScalingConfig{
						MinOnDemandNumber:     0,
						MinOnDemandPercentage: 75.0,
					}},
			},
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
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
				conf: &Config{
					AutoScalingConfig: AutoScalingConfig{
						MinOnDemandNumber:     0,
						MinOnDemandPercentage: 100,
					}},
			},
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
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
				conf: &Config{
					AutoScalingConfig: AutoScalingConfig{
						MinOnDemandNumber:     -4,
						MinOnDemandPercentage: 0,
					}},
			},
			asgInstances:    makeInstances(),
			maxSize:         aws.Int64(10),
			numberExpected:  DefaultMinOnDemandValue,
			loadingExpected: false,
		},
		{name: "Number superior to ASG size",
			region: &region{
				conf: &Config{
					AutoScalingConfig: AutoScalingConfig{
						MinOnDemandNumber:     50,
						MinOnDemandPercentage: 0,
					}},
			},
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
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
				conf: &Config{
					AutoScalingConfig: AutoScalingConfig{
						MinOnDemandNumber:     1,
						MinOnDemandPercentage: 0,
					}},
			},
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
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
				conf: &Config{
					AutoScalingConfig: AutoScalingConfig{
						MinOnDemandNumber:     2,
						MinOnDemandPercentage: 75,
					}},
			},
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
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
				conf: &Config{
					AutoScalingConfig: AutoScalingConfig{
						MinOnDemandNumber:     -20,
						MinOnDemandPercentage: 75.0,
					}},
			},
			asgInstances: makeInstancesWithCatalog(
				instanceMap{
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
				conf: &Config{
					AutoScalingConfig: AutoScalingConfig{
						MinOnDemandNumber:     -10,
						MinOnDemandPercentage: 142.2,
					}},
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
				t.Errorf("loadDefaultConfig returned: %t expected %t", done, tt.loadingExpected)
			} else if tt.numberExpected != a.minOnDemand {
				t.Errorf("loadDefaultConfig, minOnDemand value received %d, expected %d",
					a.minOnDemand, tt.numberExpected)
			}
		})
	}
}

func Test_autoScalingGroup_LoadCronSchedule(t *testing.T) {

	tests := []struct {
		name    string
		Group   *autoscaling.Group
		asgName string
		region  *region
		config  AutoScalingConfig
		want    string
	}{
		{
			name:  "No tag set on the group",
			Group: &autoscaling.Group{},
			region: &region{
				conf: &Config{
					AutoScalingConfig: AutoScalingConfig{
						CronSchedule: "1 2",
					},
				},
			},
			want: "1 2",
		},
		{
			name: "Tag set on the group",
			Group: &autoscaling.Group{
				Tags: []*autoscaling.TagDescription{
					{
						Key:   aws.String(ScheduleTag),
						Value: aws.String("3 4"),
					},
				},
			},
			region: &region{
				conf: &Config{
					AutoScalingConfig: AutoScalingConfig{
						CronSchedule: "1 2",
					},
				},
			},
			want: "3 4",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &autoScalingGroup{
				Group:  tt.Group,
				name:   tt.asgName,
				region: tt.region,
				config: tt.config,
			}
			a.LoadCronSchedule()
			got := a.config.CronSchedule
			if got != tt.want {
				t.Errorf("LoadCronSchedule got %v, expected %v", got, tt.want)
			}
		})
	}
}

func Test_autoScalingGroup_LoadCronTimezone(t *testing.T) {

	tests := []struct {
		name    string
		Group   *autoscaling.Group
		asgName string
		region  *region
		config  AutoScalingConfig
		want    string
	}{
		{
			name:  "No tag set on the group",
			Group: &autoscaling.Group{},
			region: &region{
				conf: &Config{
					AutoScalingConfig: AutoScalingConfig{
						CronTimezone: "UTC",
					},
				},
			},
			want: "UTC",
		},
		{
			name: "Tag set on the group",
			Group: &autoscaling.Group{
				Tags: []*autoscaling.TagDescription{
					{
						Key:   aws.String(TimezoneTag),
						Value: aws.String("Europe/London"),
					},
				},
			},
			region: &region{
				conf: &Config{
					AutoScalingConfig: AutoScalingConfig{
						CronTimezone: "UTC",
					},
				},
			},
			want: "Europe/London",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &autoScalingGroup{
				Group:  tt.Group,
				name:   tt.asgName,
				region: tt.region,
				config: tt.config,
			}
			a.LoadCronTimezone()
			got := a.config.CronTimezone
			if got != tt.want {
				t.Errorf("LoadCronTimezone got %v, expected %v", got, tt.want)
			}
		})
	}
}

func Test_autoScalingGroup_LoadCronScheduleState(t *testing.T) {

	tests := []struct {
		name   string
		Group  *autoscaling.Group
		region *region
		config AutoScalingConfig
		want   string
	}{
		{
			name:  "No tag set on the group",
			Group: &autoscaling.Group{},
			region: &region{
				conf: &Config{
					AutoScalingConfig: AutoScalingConfig{
						CronScheduleState: "off",
					},
				},
			},
			want: "off",
		},
		{
			name: "Tag set on the group",
			Group: &autoscaling.Group{
				Tags: []*autoscaling.TagDescription{
					{
						Key:   aws.String(CronScheduleStateTag),
						Value: aws.String("off"),
					},
				},
			},
			region: &region{
				conf: &Config{
					AutoScalingConfig: AutoScalingConfig{
						CronSchedule: "on",
					},
				},
			},
			want: "off",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &autoScalingGroup{
				Group:  tt.Group,
				region: tt.region,
				config: tt.config,
			}
			a.LoadCronScheduleState()
			got := a.config.CronScheduleState
			if got != tt.want {
				t.Errorf("LoadCronScheduleState got %v, expected %v", got, tt.want)
			}
		})
	}
}

func Test_autoScalingGroup_LoadPatchBeanstalkUserdata(t *testing.T) {
	tests := []struct {
		name    string
		Group   *autoscaling.Group
		asgName string
		config  AutoScalingConfig
		region  *region
		want    string
	}{
		{
			name:  "No tag set on the group, use region config (no value)",
			Group: &autoscaling.Group{},
			region: &region{
				conf: &Config{
					PatchBeanstalkUserdata: "",
				},
			},
			want: "",
		},
		{
			name:  "No tag set on the group, use region config (true)",
			Group: &autoscaling.Group{},
			region: &region{
				conf: &Config{
					PatchBeanstalkUserdata: "true",
				},
			},
			want: "true",
		},
		{
			name: "Tag set on the group",
			Group: &autoscaling.Group{
				Tags: []*autoscaling.TagDescription{
					{
						Key:   aws.String(PatchBeanstalkUserdataTag),
						Value: aws.String("false"),
					},
				},
			},
			region: &region{
				conf: &Config{
					PatchBeanstalkUserdata: "true",
				},
			},
			want: "false",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &autoScalingGroup{
				Group:  tt.Group,
				name:   tt.asgName,
				config: tt.config,
				region: tt.region,
			}
			a.loadPatchBeanstalkUserdata()
			got := a.config.PatchBeanstalkUserdata
			if got != tt.want {
				t.Errorf("LoadPatchBeanstalkUserdata got %v, expected %v", got, tt.want)
			}
		})
	}
}
