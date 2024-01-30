// Copyright (c) 2016-2022 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0
package autospotting

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	autoscalingTypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func TestTerminate(t *testing.T) {
	tests := []struct {
		name     string
		tags     []*ec2Types.Tag
		inst     *instance
		expected error
	}{
		{
			name: "no issue with terminate",
			tags: []*ec2Types.Tag{},
			inst: &instance{
				Instance: &ec2Types.Instance{
					InstanceId: aws.String("id1"),
					State: &ec2Types.InstanceState{
						Name: ec2Types.InstanceStateNameRunning,
					},
				},
				region: &region{
					services: connections{
						ec2: mockEC2Client{
							tierr: nil,
						},
					},
				},
			},
			expected: nil,
		},
		{
			name: "issue with terminate",
			tags: []*ec2Types.Tag{},
			inst: &instance{
				Instance: &ec2Types.Instance{
					InstanceId: aws.String("id1"),
					State: &ec2Types.InstanceState{
						Name: ec2Types.InstanceStateNameRunning,
					},
				},
				region: &region{
					services: connections{
						ec2: mockEC2Client{
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

func Test_instance_handleInstanceStates(t *testing.T) {

	tests := []struct {
		name     string
		instance instance
		want     bool
		wantErr  bool
	}{
		{
			name: "not running instance",
			instance: instance{
				Instance: &ec2Types.Instance{
					InstanceId: aws.String("i-dummy"),
					State: &ec2Types.InstanceState{
						Name: ec2Types.InstanceStateNameStopped,
					},
				},
				region: &region{
					name: "dummy",
				},
			},
			want:    true,
			wantErr: true,
		},
		{
			name: "running instance",
			instance: instance{
				Instance: &ec2Types.Instance{
					InstanceId: aws.String("i-dummy"),
					State: &ec2Types.InstanceState{
						Name: ec2Types.InstanceStateNameRunning,
					},
				},
				region: &region{
					name: "dummy",
				},
			},
			want:    true,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &instance{
				Instance:  tt.instance.Instance,
				typeInfo:  tt.instance.typeInfo,
				odPrice:   tt.instance.odPrice,
				region:    tt.instance.region,
				protected: tt.instance.protected,
				asg:       tt.instance.asg,
			}
			got, err := i.handleInstanceStates()
			if (err != nil) != tt.wantErr {
				t.Errorf("instance.handleInstanceStates() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("instance.handleInstanceStates() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_instance_launchSpotReplacement(t *testing.T) {
	tests := []struct {
		name     string
		instance instance
		want     *string
		wantErr  bool
	}{
		{
			name: "happy-path-no-errors",
			instance: instance{
				Instance: &ec2Types.Instance{
					InstanceId:         aws.String("i-dummy"),
					InstanceType:       ec2Types.InstanceTypeT2Micro,
					VirtualizationType: ec2Types.VirtualizationTypeParavirtual,
					Placement: &ec2Types.Placement{
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
					pricing: prices{
						LinuxSpotProduct: {
							onDemand: 1.2,
						},
					},
				},
				odPrice: 0.75,
				asg: &autoScalingGroup{
					spotProduct: aws.String(LinuxSpotProduct),
					AutoScalingGroup: &autoscalingTypes.AutoScalingGroup{
						DesiredCapacity: aws.Int32(4),
						Instances: []autoscalingTypes.Instance{
							{
								InstanceId:       aws.String("id-1"),
								InstanceType:     aws.String("typeX"),
								AvailabilityZone: aws.String("eu-west-1"),
							},
						},
					},
					instances: makeInstancesWithCatalog(
						instanceMap{
							"id-1": {
								Instance: &ec2Types.Instance{
									InstanceId:        aws.String("id-1"),
									InstanceType:      "typeX",
									Placement:         &ec2Types.Placement{AvailabilityZone: aws.String("eu-west-1")},
									InstanceLifecycle: Spot,
								},
							},
						},
					),
					config: AutoScalingConfig{
						OnDemandPriceMultiplier:    1.0,
						NumberOfInstanceTypesPerAZ: 3,
					},
					region: &region{
						conf: &Config{
							AutoScalingConfig: AutoScalingConfig{
								AllowedInstanceTypes: "",
							},
						},
					},
				},
				region: &region{
					instanceTypeInformation: map[string]instanceTypeInformation{
						"1": {
							instanceType: "type1", // cheapest, cheaper than ondemand
							pricing: prices{
								LinuxSpotProduct: {
									spot: map[string]float64{
										"eu-central-1": 0.5,
										"eu-west-1":    1.0,
										"eu-west-2":    2.0,
									},
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
								LinuxSpotProduct: {
									spot: map[string]float64{
										"eu-central-1": 0.7,
										"eu-west-1":    1.0,
										"eu-west-2":    2.0,
									},
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
								LinuxSpotProduct: {
									spot: map[string]float64{
										"eu-central-1": 0.8,
										"eu-west-1":    1.0,
										"eu-west-2":    2.0,
									},
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
					services: connections{
						ec2: mockEC2Client{
							cferr: nil,
							cfo: &ec2.CreateFleetOutput{
								Instances: []ec2Types.CreateFleetInstance{
									{
										InstanceIds: []string{
											"i-dummy-spot-instance-id",
										},
									},
								},
							},
							damierr: nil,
							damio:   &ec2.DescribeImagesOutput{},
						},
					},
				},
			},
			want: aws.String("i-dummy-spot-instance-id"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &instance{
				Instance:  tt.instance.Instance,
				typeInfo:  tt.instance.typeInfo,
				odPrice:   tt.instance.odPrice,
				region:    tt.instance.region,
				protected: tt.instance.protected,
				asg:       tt.instance.asg,
			}
			got, err := i.launchSpotReplacement()
			if (err != nil) != tt.wantErr {
				t.Errorf("instance.launchSpotReplacement() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != nil && tt.want != nil && *got != *tt.want {
				t.Errorf("instance.launchSpotReplacement() = %v, want %v", *got, *tt.want)
			}
		})
	}
}
