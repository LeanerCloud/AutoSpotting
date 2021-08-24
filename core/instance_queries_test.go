// Copyright (c) 2016-2021 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0
package autospotting

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
)

func TestIsSpot(t *testing.T) {

	tests := []struct {
		name      string
		lifeCycle *string
		expected  bool
	}{
		{name: "LifeCycle is nil",
			lifeCycle: nil,
			expected:  false,
		},
		{name: "LifeCycle is not nil but not spot",
			lifeCycle: aws.String("something"),
			expected:  false,
		},
		{name: "LifeCycle is not nil and is spot",
			lifeCycle: aws.String(Spot),
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &instance{Instance: &ec2.Instance{}}
			i.InstanceLifecycle = tt.lifeCycle
			retValue := i.isSpot()
			if retValue != tt.expected {
				if tt.lifeCycle != nil {
					t.Errorf("Value received for '%v': %t expected %t", *tt.lifeCycle, retValue, tt.expected)
				} else {
					t.Errorf("Value received for '%v': %t expected %t", tt.lifeCycle, retValue, tt.expected)
				}
			}
		})
	}
}

func TestIsEBSCompatible(t *testing.T) {
	tests := []struct {
		name         string
		spotInfo     instanceTypeInformation
		instanceInfo instance
		expected     bool
	}{
		{name: "EBS not Optimized Spot not Optimized",
			spotInfo: instanceTypeInformation{
				EBSThroughput: 0,
			},
			instanceInfo: instance{
				typeInfo: instanceTypeInformation{
					EBSThroughput: 0,
				},
			},
			expected: true,
		},
		{name: "EBS Optimized Spot Optimized with same throughput",
			spotInfo: instanceTypeInformation{
				EBSThroughput: 100,
			},
			instanceInfo: instance{
				typeInfo: instanceTypeInformation{
					EBSThroughput: 100,
				},
			},
			expected: true,
		},
		{name: "EBS Optimized Spot Optimized with more throughput",
			spotInfo: instanceTypeInformation{
				EBSThroughput: 200,
			},
			instanceInfo: instance{
				typeInfo: instanceTypeInformation{
					EBSThroughput: 100,
				},
			},
			expected: true,
		},
		{name: "EBS Optimized Spot not Optimized",
			spotInfo: instanceTypeInformation{
				EBSThroughput: 0,
			},
			instanceInfo: instance{
				typeInfo: instanceTypeInformation{
					EBSThroughput: 100,
				},
			},
			expected: false,
		},
		{name: "EBS not Optimized Spot Optimized",
			spotInfo: instanceTypeInformation{
				EBSThroughput: 100,
			},
			instanceInfo: instance{
				typeInfo: instanceTypeInformation{
					EBSThroughput: 0,
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &tt.instanceInfo
			retValue := i.isEBSCompatible(tt.spotInfo)
			if retValue != tt.expected {
				t.Errorf("Value received: %t expected %t", retValue, tt.expected)
			}
		})
	}
}

func TestIsPriceCompatible(t *testing.T) {
	tests := []struct {
		name             string
		spotPrices       prices
		availabilityZone *string
		instancePrice    float64
		bestPrice        float64
		expected         bool
	}{
		{name: "No spot price for such availability zone",
			spotPrices: prices{
				spot: map[string]float64{
					"eu-central-1": 0.5,
					"eu-west-1":    1.0,
					"eu-west-2":    2.0,
				},
			},
			availabilityZone: aws.String("eu-west-42"),
			instancePrice:    5.0,
			bestPrice:        0.7,
			expected:         false,
		},
		{name: "Spot price is 0.0",
			spotPrices: prices{
				spot: map[string]float64{
					"eu-central-1": 0.0,
					"eu-west-1":    0.0,
					"eu-west-2":    0.0,
				},
			},
			availabilityZone: aws.String("eu-west-1"),
			instancePrice:    5.0,
			bestPrice:        1.4,
			expected:         false,
		},
		{name: "Spot price is higher than instance price",
			spotPrices: prices{
				spot: map[string]float64{
					"eu-central-1": 0.5,
					"eu-west-1":    1.0,
					"eu-west-2":    2.0,
				},
			},
			availabilityZone: aws.String("eu-west-1"),
			instancePrice:    0.7,
			bestPrice:        0.7,
			expected:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &instance{Instance: &ec2.Instance{
				Placement: &ec2.Placement{
					AvailabilityZone: tt.availabilityZone,
				}},
				price: tt.instancePrice,
			}
			candidate := instanceTypeInformation{pricing: prices{}}
			candidate.pricing = tt.spotPrices
			spotPrice := i.calculatePrice(candidate)
			retValue := i.isPriceCompatible(spotPrice)
			if retValue != tt.expected {
				t.Errorf("Value received: %t expected %t", retValue, tt.expected)
			}
		})
	}
}

func TestIsClassCompatible(t *testing.T) {
	tests := []struct {
		name           string
		spotInfo       instanceTypeInformation
		instanceCPU    int
		instanceMemory float32
		instanceGPU    int
		expected       bool
	}{
		{name: "Spot is higher in both CPU & memory",
			spotInfo: instanceTypeInformation{
				vCPU:              10,
				memory:            2.5,
				PhysicalProcessor: "Intel",
			},
			instanceCPU:    5,
			instanceMemory: 1.0,
			expected:       true,
		},
		{name: "Spot is lower in CPU but higher in memory",
			spotInfo: instanceTypeInformation{
				vCPU:              10,
				memory:            2.5,
				PhysicalProcessor: "Intel",
			},
			instanceCPU:    15,
			instanceMemory: 1.0,
			expected:       false,
		},
		{name: "Spot is lower in memory but higher in CPU",
			spotInfo: instanceTypeInformation{
				vCPU:              10,
				memory:            2.5,
				PhysicalProcessor: "Intel",
			},
			instanceCPU:    5,
			instanceMemory: 10.0,
			expected:       false,
		},
		{name: "Spot is lower in both CPU & memory",
			spotInfo: instanceTypeInformation{
				vCPU:              10,
				memory:            2.5,
				PhysicalProcessor: "Intel",
			},
			instanceCPU:    15,
			instanceMemory: 5.0,
			expected:       false,
		},
		{name: "Spot is lower in CPU, memory and GPU ",
			spotInfo: instanceTypeInformation{
				vCPU:              10,
				memory:            2.5,
				GPU:               0,
				PhysicalProcessor: "Intel",
			},
			instanceCPU:    15,
			instanceMemory: 5.0,
			instanceGPU:    1,
			expected:       false,
		},

		{name: "Spot is higher in CPU, memory and GPU ",
			spotInfo: instanceTypeInformation{
				vCPU:              10,
				memory:            20,
				GPU:               4,
				PhysicalProcessor: "Intel",
			},
			instanceCPU:    8,
			instanceMemory: 4,
			instanceGPU:    2,
			expected:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &instance{typeInfo: instanceTypeInformation{
				vCPU:              tt.instanceCPU,
				memory:            tt.instanceMemory,
				PhysicalProcessor: "Intel",
			},
			}
			retValue := i.isClassCompatible(tt.spotInfo)
			if retValue != tt.expected {
				t.Errorf("Value received: %t expected %t", retValue, tt.expected)
			}
		})
	}
}

func TestIsStorageCompatible(t *testing.T) {
	tests := []struct {
		name            string
		spotInfo        instanceTypeInformation
		instanceInfo    instanceTypeInformation
		attachedVolumes int
		expected        bool
	}{
		{name: "Instance has no attached volumes",
			spotInfo: instanceTypeInformation{
				instanceStoreDeviceCount: 0.0,
				instanceStoreDeviceSize:  0.0,
				instanceStoreIsSSD:       false,
			},
			instanceInfo: instanceTypeInformation{
				instanceStoreDeviceSize: 0.0,
				instanceStoreIsSSD:      false,
			},
			attachedVolumes: 0,
			expected:        true,
		},
		{name: "Spot's storage is identical to instance",
			spotInfo: instanceTypeInformation{
				instanceStoreDeviceCount: 1,
				instanceStoreDeviceSize:  50.0,
				instanceStoreIsSSD:       false,
			},
			instanceInfo: instanceTypeInformation{
				instanceStoreDeviceSize: 50.0,
				instanceStoreIsSSD:      false,
			},
			attachedVolumes: 1,
			expected:        true,
		},
		{name: "Spot's storage is lower than the instance's one",
			spotInfo: instanceTypeInformation{
				instanceStoreDeviceCount: 1,
				instanceStoreDeviceSize:  25.0,
				instanceStoreIsSSD:       false,
			},
			instanceInfo: instanceTypeInformation{
				instanceStoreDeviceSize: 50.0,
				instanceStoreIsSSD:      false,
			},
			attachedVolumes: 1,
			expected:        false,
		},
		{name: "Spot's storage is bigger than the instance's one",
			spotInfo: instanceTypeInformation{
				instanceStoreDeviceCount: 1,
				instanceStoreDeviceSize:  150.0,
				instanceStoreIsSSD:       false,
			},
			instanceInfo: instanceTypeInformation{
				instanceStoreDeviceSize: 50.0,
				instanceStoreIsSSD:      false,
			},
			attachedVolumes: 1,
			expected:        true,
		},
		{name: "Spot's storage is bigger and has less storage attached",
			spotInfo: instanceTypeInformation{
				instanceStoreDeviceCount: 1,
				instanceStoreDeviceSize:  150.0,
				instanceStoreIsSSD:       false,
			},
			instanceInfo: instanceTypeInformation{
				instanceStoreDeviceSize: 50.0,
				instanceStoreIsSSD:      false,
			},
			attachedVolumes: 2,
			expected:        false,
		},
		{name: "Spot's storage is bigger and has more storage attached",
			spotInfo: instanceTypeInformation{
				instanceStoreDeviceCount: 4,
				instanceStoreDeviceSize:  150.0,
				instanceStoreIsSSD:       false,
			},
			instanceInfo: instanceTypeInformation{
				instanceStoreDeviceSize: 50.0,
				instanceStoreIsSSD:      false,
			},
			attachedVolumes: 1,
			expected:        true,
		},
		{name: "Spot's storage is bigger and has more storage attached but isn't SSD (unlike the instance)",
			spotInfo: instanceTypeInformation{
				instanceStoreDeviceCount: 4,
				instanceStoreDeviceSize:  150.0,
				instanceStoreIsSSD:       false,
			},
			instanceInfo: instanceTypeInformation{
				instanceStoreDeviceSize: 50.0,
				instanceStoreIsSSD:      true,
			},
			attachedVolumes: 1,
			expected:        false,
		},
		{name: "Spot's storage is bigger, has more storage attached, is SSD (like the instance)",
			spotInfo: instanceTypeInformation{
				instanceStoreDeviceCount: 4,
				instanceStoreDeviceSize:  150.0,
				instanceStoreIsSSD:       true,
			},
			instanceInfo: instanceTypeInformation{
				instanceStoreDeviceSize: 50.0,
				instanceStoreIsSSD:      true,
			},
			attachedVolumes: 1,
			expected:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &instance{typeInfo: tt.instanceInfo}
			retValue := i.isStorageCompatible(tt.spotInfo, tt.attachedVolumes)
			if retValue != tt.expected {
				t.Errorf("Value received: %t expected %t", retValue, tt.expected)
			}
		})
	}
}

func TestIsVirtualizationCompatible(t *testing.T) {
	tests := []struct {
		name                       string
		spotVirtualizationTypes    []string
		instanceVirtualizationType *string
		expected                   bool
	}{
		{name: "Spot's virtualization includes instance's one (pv case)",
			spotVirtualizationTypes:    []string{"PV", "HVM"},
			instanceVirtualizationType: aws.String("paravirtual"),
			expected:                   true,
		},
		{name: "Spot's virtualization includes instance's one (hvm case)",
			spotVirtualizationTypes:    []string{"PV", "HVM"},
			instanceVirtualizationType: aws.String("hvm"),
			expected:                   true,
		},
		{name: "Spot's virtualization does not include any type",
			spotVirtualizationTypes:    []string{},
			instanceVirtualizationType: aws.String("hvm"),
			expected:                   true,
		},
		{name: "Spot's virtualization doesn't include instance's one (pv case)",
			spotVirtualizationTypes:    []string{"HVM"},
			instanceVirtualizationType: aws.String("paravirtual"),
			expected:                   false,
		},
		{name: "Spot's virtualization doesn't include instance's one (hvm case)",
			spotVirtualizationTypes:    []string{"PV"},
			instanceVirtualizationType: aws.String("hvm"),
			expected:                   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &instance{Instance: &ec2.Instance{
				VirtualizationType: tt.instanceVirtualizationType,
				InstanceType:       aws.String("dummy"),
			}}
			retValue := i.isVirtualizationCompatible(tt.spotVirtualizationTypes)
			if retValue != tt.expected {
				t.Errorf("Value received: %t expected %t", retValue, tt.expected)
			}
		})
	}
}

func TestGetCheapestCompatibleSpotInstanceType(t *testing.T) {
	tests := []struct {
		name                  string
		spotInfos             map[string]instanceTypeInformation
		instanceInfo          *instance
		asg                   *autoScalingGroup
		expectedCandidateList []string
		expectedError         error
		allowedList           []string
		disallowedList        []string
	}{
		{name: "better/cheaper spot instance found",
			spotInfos: map[string]instanceTypeInformation{
				"1": {
					instanceType: "type1", // cheapest, cheaper than ondemand
					pricing: prices{
						spot: map[string]float64{
							"eu-central-1": 0.5,
							"eu-west-1":    1.0,
							"eu-west-2":    2.0,
						},
					},
					vCPU:                     10,
					PhysicalProcessor:        "Intel",
					memory:                   2.5,
					instanceStoreDeviceCount: 1,
					instanceStoreDeviceSize:  50.0,
					instanceStoreIsSSD:       false,
					virtualizationTypes:      []string{"PV", "else"},
				},
				"2": {
					instanceType: "type2", // less cheap, but cheaper than ondemand
					pricing: prices{
						spot: map[string]float64{
							"eu-central-1": 0.7,
							"eu-west-1":    1.0,
							"eu-west-2":    2.0,
						},
					},
					vCPU:                     10,
					PhysicalProcessor:        "Intel",
					memory:                   2.5,
					instanceStoreDeviceCount: 1,
					instanceStoreDeviceSize:  50.0,
					instanceStoreIsSSD:       false,
					virtualizationTypes:      []string{"PV", "else"},
				},
				"3": {
					instanceType: "type3", // more expensive than ondemand
					pricing: prices{
						spot: map[string]float64{
							"eu-central-1": 0.8,
							"eu-west-1":    1.0,
							"eu-west-2":    2.0,
						},
					},
					vCPU:                     10,
					PhysicalProcessor:        "Intel",
					memory:                   2.5,
					instanceStoreDeviceCount: 1,
					instanceStoreDeviceSize:  50.0,
					instanceStoreIsSSD:       false,
					virtualizationTypes:      []string{"PV", "else"},
				},
			},
			instanceInfo: &instance{
				Instance: &ec2.Instance{
					InstanceId:         aws.String("i-dummy"),
					VirtualizationType: aws.String("paravirtual"),
					Placement: &ec2.Placement{
						AvailabilityZone: aws.String("eu-central-1"),
					},
				},
				typeInfo: instanceTypeInformation{
					instanceType:             "typeX",
					PhysicalProcessor:        "Intel",
					vCPU:                     10,
					memory:                   2.5,
					instanceStoreDeviceCount: 1,
					instanceStoreDeviceSize:  50.0,
					instanceStoreIsSSD:       false,
				},
				price:  0.75,
				region: &region{},
			},
			asg: &autoScalingGroup{
				name: "test-asg",
				instances: makeInstancesWithCatalog(
					instanceMap{
						"id-1": {
							Instance: &ec2.Instance{
								InstanceId:        aws.String("id-1"),
								InstanceType:      aws.String("typeX"),
								Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1")},
								InstanceLifecycle: aws.String(Spot),
							},
						},
					},
				),
				Group: &autoscaling.Group{
					DesiredCapacity: aws.Int64(4),
				},
			},
			expectedCandidateList: []string{"type1", "type2"},
			expectedError:         nil,
		},
		{name: "better/cheaper spot instance found but marked as disallowed",
			spotInfos: map[string]instanceTypeInformation{
				"1": {
					instanceType: "type1",
					pricing: prices{
						spot: map[string]float64{
							"eu-central-1": 0.5,
							"eu-west-1":    1.0,
							"eu-west-2":    2.0,
						},
					},
					vCPU:                     10,
					PhysicalProcessor:        "Intel",
					memory:                   2.5,
					instanceStoreDeviceCount: 1,
					instanceStoreDeviceSize:  50.0,
					instanceStoreIsSSD:       false,
					virtualizationTypes:      []string{"PV", "else"},
				},
				"2": {
					instanceType: "type2",
					pricing: prices{
						spot: map[string]float64{
							"eu-central-1": 0.8,
							"eu-west-1":    1.0,
							"eu-west-2":    2.0,
						},
					},
					vCPU:                     10,
					PhysicalProcessor:        "Intel",
					memory:                   2.5,
					instanceStoreDeviceCount: 1,
					instanceStoreDeviceSize:  50.0,
					instanceStoreIsSSD:       false,
					virtualizationTypes:      []string{"PV", "else"},
				},
			},
			instanceInfo: &instance{
				Instance: &ec2.Instance{
					InstanceId:         aws.String("i-dummy"),
					VirtualizationType: aws.String("paravirtual"),
					Placement: &ec2.Placement{
						AvailabilityZone: aws.String("eu-central-1"),
					},
				},
				typeInfo: instanceTypeInformation{
					instanceType:             "typeX",
					vCPU:                     10,
					PhysicalProcessor:        "Intel",
					memory:                   2.5,
					instanceStoreDeviceCount: 1,
					instanceStoreDeviceSize:  50.0,
					instanceStoreIsSSD:       false,
				},
				price:  0.75,
				region: &region{},
			},
			asg: &autoScalingGroup{
				name: "test-asg",
				instances: makeInstancesWithCatalog(
					instanceMap{
						"id-1": {
							typeInfo: instanceTypeInformation{
								PhysicalProcessor: "Intel",
							},
							Instance: &ec2.Instance{
								InstanceId:        aws.String("id-1"),
								InstanceType:      aws.String("typeX"),
								Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1")},
								InstanceLifecycle: aws.String(Spot),
							},
						},
					},
				),
				Group: &autoscaling.Group{
					DesiredCapacity: aws.Int64(4),
				},
			},
			disallowedList:        []string{"type*"},
			expectedCandidateList: nil,
			expectedError:         errors.New("no cheaper spot instance types could be found"),
		},
		{name: "better/cheaper spot instance found but not marked as allowed",
			spotInfos: map[string]instanceTypeInformation{
				"1": {
					instanceType: "type1",
					pricing: prices{
						spot: map[string]float64{
							"eu-central-1": 0.5,
							"eu-west-1":    1.0,
							"eu-west-2":    2.0,
						},
					},
					vCPU:                     10,
					PhysicalProcessor:        "Intel",
					memory:                   2.5,
					instanceStoreDeviceCount: 1,
					instanceStoreDeviceSize:  50.0,
					instanceStoreIsSSD:       false,
					virtualizationTypes:      []string{"PV", "else"},
				},
				"2": {
					instanceType: "type2",
					pricing: prices{
						spot: map[string]float64{
							"eu-central-1": 0.8,
							"eu-west-1":    1.0,
							"eu-west-2":    2.0,
						},
					},
					vCPU:                     10,
					PhysicalProcessor:        "Intel",
					memory:                   2.5,
					instanceStoreDeviceCount: 1,
					instanceStoreDeviceSize:  50.0,
					instanceStoreIsSSD:       false,
					virtualizationTypes:      []string{"PV", "else"},
				},
			},
			instanceInfo: &instance{
				Instance: &ec2.Instance{
					InstanceId:         aws.String("i-dummy"),
					VirtualizationType: aws.String("paravirtual"),
					Placement: &ec2.Placement{
						AvailabilityZone: aws.String("eu-central-1"),
					},
				},
				typeInfo: instanceTypeInformation{
					instanceType:             "typeX",
					PhysicalProcessor:        "Intel",
					vCPU:                     10,
					memory:                   2.5,
					instanceStoreDeviceCount: 1,
					instanceStoreDeviceSize:  50.0,
					instanceStoreIsSSD:       false,
				},
				price:  0.75,
				region: &region{},
			},
			asg: &autoScalingGroup{
				name: "test-asg",
				instances: makeInstancesWithCatalog(
					instanceMap{
						"id-1": {
							Instance: &ec2.Instance{
								InstanceId:        aws.String("id-1"),
								InstanceType:      aws.String("typeX"),
								Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1")},
								InstanceLifecycle: aws.String(Spot),
							},
						},
					},
				),
				Group: &autoscaling.Group{
					DesiredCapacity: aws.Int64(4),
				},
			},
			allowedList:           []string{"asdf*"},
			expectedCandidateList: nil,
			expectedError:         errors.New("no cheaper spot instance types could be found"),
		},
		{name: "better/cheaper spot instance found and marked as allowed",
			spotInfos: map[string]instanceTypeInformation{
				"1": {
					instanceType: "type1",
					pricing: prices{
						spot: map[string]float64{
							"eu-central-1": 0.5,
							"eu-west-1":    1.0,
							"eu-west-2":    2.0,
						},
					},
					vCPU:                     10,
					PhysicalProcessor:        "Intel",
					memory:                   2.5,
					instanceStoreDeviceCount: 1,
					instanceStoreDeviceSize:  50.0,
					instanceStoreIsSSD:       false,
					virtualizationTypes:      []string{"PV", "else"},
				},
				"2": {
					instanceType: "type2",
					pricing: prices{
						spot: map[string]float64{
							"eu-central-1": 0.8,
							"eu-west-1":    1.0,
							"eu-west-2":    2.0,
						},
					},
					vCPU:                     10,
					PhysicalProcessor:        "Intel",
					memory:                   2.5,
					instanceStoreDeviceCount: 1,
					instanceStoreDeviceSize:  50.0,
					instanceStoreIsSSD:       false,
					virtualizationTypes:      []string{"PV", "else"},
				},
			},
			instanceInfo: &instance{
				Instance: &ec2.Instance{
					InstanceId:         aws.String("i-dummy"),
					VirtualizationType: aws.String("paravirtual"),
					Placement: &ec2.Placement{
						AvailabilityZone: aws.String("eu-central-1"),
					},
				},
				typeInfo: instanceTypeInformation{
					instanceType:             "typeX",
					vCPU:                     10,
					PhysicalProcessor:        "Intel",
					memory:                   2.5,
					instanceStoreDeviceCount: 1,
					instanceStoreDeviceSize:  50.0,
					instanceStoreIsSSD:       false,
				},
				price:  0.75,
				region: &region{},
			},
			asg: &autoScalingGroup{
				name: "test-asg",
				instances: makeInstancesWithCatalog(
					instanceMap{
						"id-1": {
							Instance: &ec2.Instance{
								InstanceId:        aws.String("id-1"),
								InstanceType:      aws.String("typeX"),
								Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1")},
								InstanceLifecycle: aws.String(Spot),
							},
						},
					},
				),
				Group: &autoscaling.Group{
					DesiredCapacity: aws.Int64(4),
				},
			},

			allowedList:           []string{"ty*"},
			expectedCandidateList: []string{"type1"},
			expectedError:         nil,
		},
		{name: "better/cheaper spot instance not found",
			spotInfos: map[string]instanceTypeInformation{
				"1": {
					instanceType: "type1",
					pricing: prices{
						spot: map[string]float64{
							"eu-central-1": 0.5,
							"eu-west-1":    1.0,
							"eu-west-2":    2.0,
						},
					},
					vCPU:                     10,
					PhysicalProcessor:        "Intel",
					memory:                   2.5,
					instanceStoreDeviceCount: 1,
					instanceStoreDeviceSize:  50.0,
					instanceStoreIsSSD:       false,
					virtualizationTypes:      []string{"PV", "else"},
				},
				"2": {
					instanceType: "type2",
					pricing: prices{
						spot: map[string]float64{
							"eu-central-1": 0.8,
							"eu-west-1":    1.0,
							"eu-west-2":    2.0,
						},
					},
					vCPU:                     10,
					PhysicalProcessor:        "Intel",
					memory:                   2.5,
					instanceStoreDeviceCount: 1,
					instanceStoreDeviceSize:  50.0,
					instanceStoreIsSSD:       false,
					virtualizationTypes:      []string{"PV", "else"},
				},
			},
			instanceInfo: &instance{
				Instance: &ec2.Instance{
					InstanceId:         aws.String("i-dummy"),
					VirtualizationType: aws.String("paravirtual"),
					Placement: &ec2.Placement{
						AvailabilityZone: aws.String("eu-central-1"),
					},
				},
				typeInfo: instanceTypeInformation{
					instanceType:             "typeX",
					vCPU:                     10,
					PhysicalProcessor:        "Intel",
					memory:                   2.5,
					instanceStoreDeviceCount: 1,
					instanceStoreDeviceSize:  50.0,
					instanceStoreIsSSD:       false,
				},
				price:  0.45,
				region: &region{},
			},
			asg: &autoScalingGroup{
				name: "test-asg",
				instances: makeInstancesWithCatalog(
					instanceMap{
						"id-1": {
							Instance: &ec2.Instance{
								InstanceId:        aws.String("id-1"),
								InstanceType:      aws.String("typeX"),
								Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1")},
								InstanceLifecycle: aws.String(Spot),
							},
						},
					},
				),
				Group: &autoscaling.Group{
					DesiredCapacity: aws.Int64(4),
				},
			},
			expectedCandidateList: nil,
			expectedError:         errors.New("no cheaper spot instance types could be found"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := tt.instanceInfo
			i.region.instanceTypeInformation = tt.spotInfos
			i.asg = tt.asg
			allowedList := tt.allowedList
			disallowedList := tt.disallowedList
			retValue, err := i.getCompatibleSpotInstanceTypesListSortedAscendingByPrice(allowedList, disallowedList)
			var retInstTypes []string
			for _, retval := range retValue {
				retInstTypes = append(retInstTypes, retval.instanceType)
			}
			if err == nil && tt.expectedError != err {
				t.Errorf("1 Error received: %v expected %v", err, tt.expectedError.Error())
			} else if err != nil && tt.expectedError == nil {
				t.Errorf("2 Error received: %s expected %s", err.Error(), tt.expectedError)
			} else if err != nil && tt.expectedError != nil && err.Error() != tt.expectedError.Error() {
				t.Errorf("3 Error received: %s expected %s", err.Error(), tt.expectedError.Error())
			} else if !reflect.DeepEqual(retInstTypes, tt.expectedCandidateList) {
				t.Errorf("4 Value received: %s expected %s", retInstTypes, tt.expectedCandidateList)
			}
		})
	}
}

func TestGetPriceToBid(t *testing.T) {
	tests := []struct {
		spotPercentage       float64
		currentSpotPrice     float64
		currentOnDemandPrice float64
		spotPremium          float64
		policy               string
		want                 float64
	}{
		{
			spotPercentage:       50.0,
			currentSpotPrice:     0.0216,
			currentOnDemandPrice: 0.0464,
			spotPremium:          0.0,
			policy:               "aggressive",
			want:                 0.0324,
		},
		{
			spotPercentage:       79.0,
			currentSpotPrice:     0.0216,
			currentOnDemandPrice: 0.0464,
			spotPremium:          0.0,
			policy:               "aggressive",
			want:                 0.038664,
		},
		{
			spotPercentage:       79.0,
			currentSpotPrice:     0.0216,
			currentOnDemandPrice: 0.0464,
			spotPremium:          0.0,
			policy:               "normal",
			want:                 0.0464,
		},
		{
			spotPercentage:       200.0,
			currentSpotPrice:     0.0216,
			currentOnDemandPrice: 0.0464,
			spotPremium:          0.0,
			policy:               "aggressive",
			want:                 0.0464,
		},
		{
			spotPercentage:       0.0,
			currentSpotPrice:     0.0216,
			currentOnDemandPrice: 0.0464,
			spotPremium:          0.0,
			policy:               "aggressive",
			want:                 0.0216,
		},
		{
			spotPercentage:       50.0,
			currentSpotPrice:     0.0816,
			currentOnDemandPrice: 0.1064,
			spotPremium:          0.06,
			policy:               "aggressive",
			want:                 0.0924,
		},
	}
	for _, tt := range tests {
		cfg := &Config{
			AutoScalingConfig: AutoScalingConfig{
				SpotPriceBufferPercentage: tt.spotPercentage,
				BiddingPolicy:             tt.policy,
			}}
		i := &instance{
			region: &region{
				name: "us-east-1",
				conf: cfg,
			},
			Instance: &ec2.Instance{
				InstanceId: aws.String("i-0000000"),
			},
		}

		currentSpotPrice := tt.currentSpotPrice
		currentOnDemandPrice := tt.currentOnDemandPrice
		currentSpotPremium := tt.spotPremium
		actualPrice := i.getPriceToBid(currentOnDemandPrice, currentSpotPrice, currentSpotPremium)
		if math.Abs(actualPrice-tt.want) > 0.000001 {
			t.Errorf("percentage = %.2f, policy = %s, expected price = %.5f, want %.5f, currentSpotPrice = %.5f",
				tt.spotPercentage, tt.policy, actualPrice, tt.want, currentSpotPrice)
		}
	}
}

func Test_instance_isSameArch(t *testing.T) {

	tests := []struct {
		name          string
		Instance      instance
		spotCandidate instanceTypeInformation
		want          bool
	}{
		{
			name: "Same architecture: both Intel",
			Instance: instance{
				typeInfo: instanceTypeInformation{
					PhysicalProcessor: "Intel",
				},
			},
			spotCandidate: instanceTypeInformation{
				PhysicalProcessor: "Intel",
			},
			want: true,
		},

		{
			name: "Same architecture: both AMD",
			Instance: instance{
				typeInfo: instanceTypeInformation{
					PhysicalProcessor: "AMD",
				},
			},
			spotCandidate: instanceTypeInformation{
				PhysicalProcessor: "AMD",
			},
			want: true,
		},

		{
			name: "Same architecture: Intel and AMD",
			Instance: instance{
				typeInfo: instanceTypeInformation{
					PhysicalProcessor: "Intel",
				},
			},
			spotCandidate: instanceTypeInformation{
				PhysicalProcessor: "AMD",
			},
			want: true,
		},

		{
			name: "Same architecture: AMD and Intel",
			Instance: instance{
				typeInfo: instanceTypeInformation{
					PhysicalProcessor: "AMD",
				},
			},
			spotCandidate: instanceTypeInformation{
				PhysicalProcessor: "Intel",
			},
			want: true,
		},

		{
			name: "Same architecture: Intel and Variable",
			Instance: instance{
				typeInfo: instanceTypeInformation{
					PhysicalProcessor: "Intel",
				},
			},
			spotCandidate: instanceTypeInformation{
				PhysicalProcessor: "Variable",
			},
			want: true,
		},

		{
			name: "Same architecture: Variable and Intel",
			Instance: instance{
				typeInfo: instanceTypeInformation{
					PhysicalProcessor: "Variable",
				},
			},
			spotCandidate: instanceTypeInformation{
				PhysicalProcessor: "Intel",
			},
			want: true,
		},

		{
			name: "Same architecture, both ARM-based",
			Instance: instance{
				typeInfo: instanceTypeInformation{
					PhysicalProcessor: "AWS",
				},
			},
			spotCandidate: instanceTypeInformation{
				PhysicalProcessor: "AWS",
			},
			want: true,
		},

		{
			name: "Different architecture, Intel and ARM",
			Instance: instance{
				typeInfo: instanceTypeInformation{
					PhysicalProcessor: "Intel",
				},
			},
			spotCandidate: instanceTypeInformation{
				PhysicalProcessor: "AWS",
			},
			want: false,
		},

		{
			name: "Different architecture, AMD and ARM",
			Instance: instance{
				typeInfo: instanceTypeInformation{
					PhysicalProcessor: "Intel",
				},
			},
			spotCandidate: instanceTypeInformation{
				PhysicalProcessor: "AWS",
			},
			want: false,
		},

		{
			name: "Different architecture, ARM and Intel",
			Instance: instance{
				typeInfo: instanceTypeInformation{
					PhysicalProcessor: "AWS",
				},
			},
			spotCandidate: instanceTypeInformation{
				PhysicalProcessor: "Intel",
			},
			want: false,
		},

		{
			name: "Different architecture, ARM and AMD",
			Instance: instance{
				typeInfo: instanceTypeInformation{
					PhysicalProcessor: "AWS",
				},
			},
			spotCandidate: instanceTypeInformation{
				PhysicalProcessor: "AMD",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.Instance.isSameArch(tt.spotCandidate); got != tt.want {
				t.Errorf("Instance.isSameArch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_instance_isUnattachedSpotInstanceLaunchedForAnEnabledASG(t *testing.T) {

	tests := []struct {
		name           string
		i              *instance
		wantASG        *autoScalingGroup
		wantUnattached bool
	}{
		// {
		// 	name: "on-demand instance",
		// 	i: &instance{
		// 		Instance: &ec2.Instance{
		// 			InstanceLifecycle: nil,
		// 		},
		// 	},
		// },

		// {
		// 	name: "no instances launched for this ASG",
		// 	asg: autoScalingGroup{
		// 		name: "mygroup",
		// 		region: &region{
		// 			instances: makeInstancesWithCatalog(
		// 				instanceMap{
		// 					"id-1": {
		// 						Instance: &ec2.Instance{
		// 							InstanceId: aws.String("id-1"),
		// 							Tags:       []*ec2.Tag{},
		// 						},
		// 					},
		// 				},
		// 			),
		// 		},
		// 	},
		// 	want: nil,
		// },
		// {
		// 	name: "instance launched for another ASG",
		// 	asg: autoScalingGroup{
		// 		name: "mygroup",
		// 		region: &region{
		// 			instances: makeInstancesWithCatalog(
		// 				instanceMap{
		// 					"id-1": {
		// 						Instance: &ec2.Instance{
		// 							InstanceId: aws.String("id-1"),
		// 							Tags:       []*ec2.Tag{},
		// 						},
		// 					},
		// 					"id-2": {
		// 						Instance: &ec2.Instance{
		// 							InstanceId: aws.String("id-2"),
		// 							Tags: []*ec2.Tag{
		// 								{
		// 									Key:   aws.String("launched-for-asg"),
		// 									Value: aws.String("another-asg"),
		// 								},
		// 								{
		// 									Key:   aws.String("another-key"),
		// 									Value: aws.String("another-value"),
		// 								},
		// 							},
		// 						},
		// 					},
		// 				},
		// 			),
		// 		},
		// 	},
		// 	want: nil,
		// }, {
		// 	name: "instance launched for current ASG",
		// 	asg: autoScalingGroup{
		// 		name: "mygroup",
		// 		Group: &autoscaling.Group{
		// 			Instances: []*autoscaling.Instance{
		// 				{InstanceId: aws.String("foo")},
		// 				{InstanceId: aws.String("bar")},
		// 				{InstanceId: aws.String("baz")},
		// 			},
		// 		},

		// 		region: &region{
		// 			instances: makeInstancesWithCatalog(
		// 				instanceMap{
		// 					"id-1": {
		// 						Instance: &ec2.Instance{
		// 							InstanceId: aws.String("id-1"),
		// 							Tags:       []*ec2.Tag{},
		// 						},
		// 					},
		// 					"id-2": {
		// 						Instance: &ec2.Instance{
		// 							InstanceId: aws.String("id-2"),
		// 							Tags: []*ec2.Tag{
		// 								{
		// 									Key:   aws.String("launched-for-asg"),
		// 									Value: aws.String("mygroup"),
		// 								},
		// 								{
		// 									Key:   aws.String("another-key"),
		// 									Value: aws.String("another-value"),
		// 								},
		// 							},
		// 						},
		// 					},
		// 				},
		// 			),
		// 		},
		// 	},
		// 	want: &instance{
		// 		Instance: &ec2.Instance{
		// 			InstanceId: aws.String("id-2"),
		// 			Tags: []*ec2.Tag{
		// 				{
		// 					Key:   aws.String("launched-for-asg"),
		// 					Value: aws.String("mygroup"),
		// 				},
		// 				{
		// 					Key:   aws.String("another-key"),
		// 					Value: aws.String("another-value"),
		// 				},
		// 			},
		// 		},
		// 	},
		// },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			gotUnattached := tt.i.isUnattachedSpotInstanceLaunchedForAnEnabledASG()
			if !reflect.DeepEqual(gotUnattached, tt.wantUnattached) {
				t.Errorf("instance.isUnattachedSpotInstanceLaunchedForAnEnabledASG() = %v, want %v", gotUnattached, tt.wantUnattached)
			}
		})

	}
}

func TestMin(t *testing.T) {
	tests := []struct {
		name     string
		x        int
		y        int
		expected int
	}{
		{name: "Testing min between 0 and 0",
			x:        0,
			y:        0,
			expected: 0,
		},
		{name: "Testing min between 0 and 10",
			x:        0,
			y:        10,
			expected: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retValue := min(tt.x, tt.y)
			if retValue != tt.expected {
				t.Errorf("Value received: %d expected %d", retValue, tt.expected)
			}
		})
	}
}
