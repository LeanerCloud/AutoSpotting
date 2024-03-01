// Copyright (c) 2016-2022 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"errors"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	autoscalingTypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/davecgh/go-spew/spew"
)

func TestGenerateTagList(t *testing.T) {
	tests := []struct {
		name                     string
		ASGName                  string
		ASGLCName                string
		instanceTags             []ec2Types.Tag
		instanceID               string
		expectedTagSpecification []ec2Types.TagSpecification
	}{
		{name: "no tags on original instance",
			ASGLCName: "testLC0",
			ASGName:   "myASG", instanceID: "foo",
			instanceTags: []ec2Types.Tag{},
			expectedTagSpecification: []ec2Types.TagSpecification{
				{
					ResourceType: "instance",
					Tags: []ec2Types.Tag{
						{
							Key:   aws.String("LaunchConfigurationName"),
							Value: aws.String("testLC0"),
						},
						{
							Key:   aws.String("launched-by-autospotting"),
							Value: aws.String("true"),
						},
						{
							Key:   aws.String("launched-for-asg"),
							Value: aws.String("myASG"),
						}, {
							Key:   aws.String("launched-for-replacing-instance"),
							Value: aws.String("foo"),
						},
						{
							Key:   aws.String("odPrice"),
							Value: aws.String("0"),
						},
					},
				},
			},
		},
		{name: "Multiple tags on original instance",
			ASGLCName:  "testLC0",
			ASGName:    "myASG",
			instanceID: "bar",
			instanceTags: []ec2Types.Tag{
				{
					Key:   aws.String("foo"),
					Value: aws.String("bar"),
				},
				{
					Key:   aws.String("baz"),
					Value: aws.String("bazinga"),
				},
			},
			expectedTagSpecification: []ec2Types.TagSpecification{
				{
					ResourceType: "instance",
					Tags: []ec2Types.Tag{
						{
							Key:   aws.String("LaunchConfigurationName"),
							Value: aws.String("testLC0"),
						},
						{
							Key:   aws.String("launched-by-autospotting"),
							Value: aws.String("true"),
						},
						{
							Key:   aws.String("launched-for-replacing-instance"),
							Value: aws.String("bar"),
						},
						{
							Key:   aws.String("odPrice"),
							Value: aws.String("0"),
						},
						{
							Key:   aws.String("launched-for-asg"),
							Value: aws.String("myASG"),
						},
						{
							Key:   aws.String("foo"),
							Value: aws.String("bar"),
						},
						{
							Key:   aws.String("baz"),
							Value: aws.String("bazinga"),
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			i := instance{
				Instance: &ec2Types.Instance{
					Tags:       tt.instanceTags,
					InstanceId: aws.String(tt.instanceID),
				},
				asg: &autoScalingGroup{
					name: tt.ASGName,
					AutoScalingGroup: &autoscalingTypes.AutoScalingGroup{
						LaunchConfigurationName: aws.String(tt.ASGLCName),
					},
				},
			}

			tags := i.generateTagsList()

			// make sure the lists of tags are sorted, otherwise the comparison fails
			sort.Slice(tags[0].Tags, func(i, j int) bool {
				return *tags[0].Tags[i].Key < *tags[0].Tags[j].Key
			})
			sort.Slice(tt.expectedTagSpecification[0].Tags, func(i, j int) bool {
				return *tt.expectedTagSpecification[0].Tags[i].Key < *tt.expectedTagSpecification[0].Tags[j].Key
			})

			if !reflect.DeepEqual(tags[0].Tags, tt.expectedTagSpecification[0].Tags) {
				t.Errorf("propagatedInstanceTags received: %+v, expected: %+v",
					tags, tt.expectedTagSpecification)
			}
		})
	}
}

func Test_instance_convertLaunchConfigurationBlockDeviceMappings(t *testing.T) {

	tests := []struct {
		name string
		BDMs []autoscalingTypes.BlockDeviceMapping
		i    *instance
		want []ec2Types.LaunchTemplateBlockDeviceMappingRequest
	}{
		{
			name: "nil block device mapping",
			BDMs: nil,
			i:    &instance{},
			want: nil,
		},
		{
			name: "instance-store only, skipping one of the volumes from the BDMs",
			BDMs: []autoscalingTypes.BlockDeviceMapping{
				{
					DeviceName:  aws.String("/dev/ephemeral0"),
					Ebs:         nil,
					NoDevice:    aws.Bool(true),
					VirtualName: aws.String("foo"),
				},
				{
					DeviceName:  aws.String("/dev/ephemeral1"),
					Ebs:         nil,
					VirtualName: aws.String("bar"),
				},
			},
			i: &instance{},
			want: []ec2Types.LaunchTemplateBlockDeviceMappingRequest{
				{
					DeviceName:  aws.String("/dev/ephemeral1"),
					Ebs:         nil,
					VirtualName: aws.String("bar"),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := tt.i
			if got := i.convertLaunchConfigurationBlockDeviceMappings(tt.BDMs); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("instance.convertLaunchConfigurationBlockDeviceMappings() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func Test_instance_convertSecurityGroups(t *testing.T) {

	tests := []struct {
		name    string
		inst    instance
		want    []string
		wantErr bool // Indicate whether an error is expected
	}{
		{
			name: "missing SGs",
			inst: instance{
				Instance: &ec2Types.Instance{},
			},
			want:    []string{},
			wantErr: false,
		},
		{
			name: "single SG",
			inst: instance{
				Instance: &ec2Types.Instance{
					SecurityGroups: []ec2Types.GroupIdentifier{
						{
							GroupId:   aws.String("sg-123"),
							GroupName: aws.String("foo"),
						},
					},
				},
			},
			want:    []string{"sg-123"},
			wantErr: false,
		},
		{
			name: "multiple SGs",
			inst: instance{
				Instance: &ec2Types.Instance{
					SecurityGroups: []ec2Types.GroupIdentifier{
						{
							GroupId:   aws.String("sg-123"),
							GroupName: aws.String("foo"),
						},
						{
							GroupId:   aws.String("sg-456"),
							GroupName: aws.String("bar"),
						},
					},
				},
			},
			want:    []string{"sg-123", "sg-456"},
			wantErr: false,
		},
		// Add test cases for scenarios expected to return an error
		{
			name: "error scenario",
			inst: instance{
				Instance: &ec2Types.Instance{
					SecurityGroups: []ec2Types.GroupIdentifier{
						{
							GroupName: aws.String("dummy-name"),
						},
					},
				},
				region: &region{
					services: &connections{
						ec2: &mockEC2Client{
							dsgo:   nil,
							dsgerr: errors.New("dummy error"),
						},
					},
				},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.inst.convertSecurityGroups()
			if (err != nil) != tt.wantErr {
				t.Errorf("instance.convertSecurityGroups() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("instance.convertSecurityGroups() = %v, want %v", spew.Sdump(got), spew.Sdump(tt.want))
			}
		})
	}
}

func Test_instance_createLaunchTemplateData(t *testing.T) {

	tests := []struct {
		name string
		inst instance
		want *ec2Types.RequestLaunchTemplateData
	}{

		{
			name: "createLaunchTemplateData() with simple LC",
			inst: instance{
				spotProduct: aws.String(LinuxSpotProduct),
				typeInfo: &instanceTypeInformation{
					pricing: prices{
						LinuxSpotProduct: {
							onDemand: 1.5,
						},
					},
				},
				region: &region{
					services: &connections{
						ec2: mockEC2Client{
							damio: &ec2.DescribeImagesOutput{
								Images: []ec2Types.Image{},
							},
						},
					},
				},
				asg: &autoScalingGroup{
					name:        "mygroup",
					spotProduct: aws.String(LinuxSpotProduct),
					config: AutoScalingConfig{
						OnDemandPriceMultiplier: 1,
					},
					AutoScalingGroup: &autoscalingTypes.AutoScalingGroup{
						LaunchConfigurationName: aws.String("myLC"),
					},
					launchConfiguration: &launchConfiguration{
						LaunchConfiguration: &autoscalingTypes.LaunchConfiguration{
							AssociatePublicIpAddress: nil,
							BlockDeviceMappings:      nil,
							ImageId:                  aws.String("ami-12345"),
							KeyName:                  aws.String("mykey"),
							InstanceMonitoring:       nil,
							UserData:                 aws.String("userdata"),
							IamInstanceProfile:       aws.String("profile"),
						},
					},
				},
				Instance: &ec2Types.Instance{
					EbsOptimized: aws.Bool(true),

					IamInstanceProfile: &ec2Types.IamInstanceProfile{
						Arn: aws.String("profile-arn"),
					},
					ImageId:      aws.String("ami-123"),
					InstanceId:   aws.String("i-foo"),
					InstanceType: "t2.medium",

					Placement: &ec2Types.Placement{
						Affinity: aws.String("foo"),
					},

					SecurityGroups: []ec2Types.GroupIdentifier{
						{
							GroupName: aws.String("foo"),
							GroupId:   aws.String("sg-123"),
						},
						{
							GroupName: aws.String("bar"),
							GroupId:   aws.String("sg-456"),
						},
					},

					SubnetId: nil,
				},
			},
			want: &ec2Types.RequestLaunchTemplateData{

				EbsOptimized: aws.Bool(true),

				IamInstanceProfile: &ec2Types.LaunchTemplateIamInstanceProfileSpecificationRequest{
					Name: aws.String("profile"),
				},

				ImageId: aws.String("ami-12345"),

				InstanceMarketOptions: &ec2Types.LaunchTemplateInstanceMarketOptionsRequest{
					MarketType: Spot,
					SpotOptions: &ec2Types.LaunchTemplateSpotMarketOptionsRequest{
						MaxPrice: aws.String("1.5"),
					},
				},

				KeyName: aws.String("mykey"),

				Placement: &ec2Types.LaunchTemplatePlacementRequest{
					Affinity: aws.String("foo"),
				},

				SecurityGroupIds: []string{
					"sg-123",
					"sg-456",
				},
				UserData: aws.String("userdata"),
			},
		},

		{
			name: "createLaunchTemplateData() with full launch configuration",
			inst: instance{
				spotProduct: aws.String(LinuxSpotProduct),
				typeInfo: &instanceTypeInformation{
					pricing: prices{
						LinuxSpotProduct: {
							onDemand: 1.5,
						},
					},
				},
				region: &region{
					services: &connections{
						ec2: mockEC2Client{
							damio: &ec2.DescribeImagesOutput{
								Images: []ec2Types.Image{},
							},
						},
					},
				},
				asg: &autoScalingGroup{
					name:        "mygroup",
					spotProduct: aws.String(LinuxSpotProduct),
					config: AutoScalingConfig{
						OnDemandPriceMultiplier: 1,
					},
					AutoScalingGroup: &autoscalingTypes.AutoScalingGroup{
						LaunchConfigurationName: aws.String("myLC"),
					},
					launchConfiguration: &launchConfiguration{
						LaunchConfiguration: &autoscalingTypes.LaunchConfiguration{
							IamInstanceProfile: aws.String("profile-name"),
							ImageId:            aws.String("ami-12345"),
							InstanceMonitoring: &autoscalingTypes.InstanceMonitoring{
								Enabled: aws.Bool(true),
							},
							KeyName: aws.String("current-key"),
							BlockDeviceMappings: []autoscalingTypes.BlockDeviceMapping{
								{
									DeviceName: aws.String("foo"),
								},
							},
							AssociatePublicIpAddress: aws.Bool(true),
							UserData:                 aws.String("userdata"),
							SecurityGroups: []string{
								"sg-123",
								"sg-456",
							},
						},
					},
				},
				Instance: &ec2Types.Instance{
					EbsOptimized: aws.Bool(true),

					IamInstanceProfile: &ec2Types.IamInstanceProfile{
						Arn: aws.String("profile-arn"),
					},
					ImageId:      aws.String("ami-123"),
					InstanceId:   aws.String("i-foo"),
					InstanceType: "t2.medium",
					KeyName:      aws.String("older-key"),

					Placement: &ec2Types.Placement{
						Affinity: aws.String("foo"),
					},

					SecurityGroups: []ec2Types.GroupIdentifier{
						{
							GroupName: aws.String("foo"),
							GroupId:   aws.String("sg-123"),
						},
						{
							GroupName: aws.String("bar"),
							GroupId:   aws.String("sg-456"),
						},
					},

					SubnetId: aws.String("subnet-123"),
				},
			},

			want: &ec2Types.RequestLaunchTemplateData{
				BlockDeviceMappings: []ec2Types.LaunchTemplateBlockDeviceMappingRequest{
					{
						DeviceName: aws.String("foo"),
					},
				},

				EbsOptimized: aws.Bool(true),

				IamInstanceProfile: &ec2Types.LaunchTemplateIamInstanceProfileSpecificationRequest{
					Name: aws.String("profile-name"),
				},

				ImageId: aws.String("ami-12345"),

				InstanceMarketOptions: &ec2Types.LaunchTemplateInstanceMarketOptionsRequest{
					MarketType: Spot,
					SpotOptions: &ec2Types.LaunchTemplateSpotMarketOptionsRequest{
						MaxPrice: aws.String("1.5"),
					},
				},

				KeyName: aws.String("current-key"),

				Monitoring: &ec2Types.LaunchTemplatesMonitoringRequest{
					Enabled: aws.Bool(true),
				},

				Placement: &ec2Types.LaunchTemplatePlacementRequest{
					Affinity: aws.String("foo"),
				},

				NetworkInterfaces: []ec2Types.LaunchTemplateInstanceNetworkInterfaceSpecificationRequest{
					{
						AssociatePublicIpAddress: aws.Bool(true),
						DeviceIndex:              aws.Int32(0),
						SubnetId:                 aws.String("subnet-123"),
						Groups: []string{
							"sg-123",
							"sg-456",
						},
					},
				},

				UserData: aws.String("userdata"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			got, _ := tt.inst.createLaunchTemplateData("spot")

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Instance.createLaunchTemplateData() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func Test_instance_convertImageBlockDeviceMappings(t *testing.T) {
	tests := []struct {
		name string
		BDMs []ec2Types.BlockDeviceMapping
		i    *instance
		want []ec2Types.LaunchTemplateBlockDeviceMappingRequest
	}{
		{
			name: "nil block device mapping",
			BDMs: nil,
			i:    &instance{},
			want: nil,
		},
		{
			name: "instance-store only, skipping one of the volumes from the BDMs",
			BDMs: []ec2Types.BlockDeviceMapping{
				{
					DeviceName:  aws.String("/dev/ephemeral0"),
					Ebs:         nil,
					NoDevice:    aws.String("true"),
					VirtualName: aws.String("foo"),
				},
				{
					DeviceName:  aws.String("/dev/ephemeral1"),
					Ebs:         nil,
					VirtualName: aws.String("bar"),
				},
			},
			i: &instance{},
			want: []ec2Types.LaunchTemplateBlockDeviceMappingRequest{
				{
					DeviceName:  aws.String("/dev/ephemeral1"),
					Ebs:         nil,
					VirtualName: aws.String("bar"),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.i.convertImageBlockDeviceMappings(tt.BDMs); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("instance.convertImageBlockDeviceMappings() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_instance_createFleetInput(t *testing.T) {

	tests := []struct {
		name          string
		i             *instance
		ltName        *string
		ltVersion     *string
		instanceTypes []string
		want          *ec2.CreateFleetInput
	}{
		{
			name:      "test generating list of overrides with capacity-optimized-prioritized",
			ltName:    aws.String("testLT"),
			ltVersion: aws.String("$Latest"),
			instanceTypes: []string{
				"instance-type1",
				"instance-type2",
			},
			i: &instance{
				spotProduct:  aws.String(LinuxSpotProduct),
				initialPrice: 2,
				Instance: &ec2Types.Instance{
					SubnetId: aws.String("subnet-id"),
				},
				asg: &autoScalingGroup{
					config: AutoScalingConfig{
						SpotAllocationStrategy: "capacity-optimized-prioritized",
					},

					AutoScalingGroup: &autoscalingTypes.AutoScalingGroup{
						LaunchConfigurationName: aws.String("myLC"),
					},
					launchConfiguration: &launchConfiguration{
						LaunchConfiguration: &autoscalingTypes.LaunchConfiguration{
							IamInstanceProfile: aws.String("profile-name"),
							ImageId:            aws.String("ami-12345"),
							InstanceMonitoring: &autoscalingTypes.InstanceMonitoring{
								Enabled: aws.Bool(true),
							},
							KeyName: aws.String("current-key"),
							BlockDeviceMappings: []autoscalingTypes.BlockDeviceMapping{
								{
									DeviceName: aws.String("foo"),
								},
							},
							AssociatePublicIpAddress: aws.Bool(true),
							UserData:                 aws.String("userdata"),
						},
					},
				},
				region: &region{},
				typeInfo: &instanceTypeInformation{
					pricing: prices{
						LinuxSpotProduct: {
							onDemand: 1,
						},
					},
				},
			},
			want: &ec2.CreateFleetInput{
				LaunchTemplateConfigs: []ec2Types.FleetLaunchTemplateConfigRequest{
					{
						LaunchTemplateSpecification: &ec2Types.FleetLaunchTemplateSpecificationRequest{
							LaunchTemplateName: aws.String("testLT"),
							Version:            aws.String("$Latest"),
						},
						Overrides: []ec2Types.FleetLaunchTemplateOverridesRequest{
							{
								InstanceType: "instance-type1",
								Priority:     aws.Float64(0),
								SubnetId:     aws.String("subnet-id"),
							},
							{
								InstanceType: "instance-type2",
								Priority:     aws.Float64(1),
								SubnetId:     aws.String("subnet-id"),
							},
						},
					},
				},
				OnDemandOptions: &ec2Types.OnDemandOptionsRequest{
					AllocationStrategy: "prioritized",
				},
				SpotOptions: &ec2Types.SpotOptionsRequest{
					AllocationStrategy: "capacity-optimized-prioritized",
					MaxTotalPrice:      aws.String("2"),
				},
				TagSpecifications: []ec2Types.TagSpecification{
					{
						ResourceType: "instance",
						Tags: []ec2Types.Tag{
							{
								Key:   aws.String("launched-by-autospotting"),
								Value: aws.String("true"),
							},
							{
								Key:   aws.String("launched-for-asg"),
								Value: aws.String(""),
							},
							{
								Key: aws.String("launched-for-replacing-instance"),
							},
							{
								Key:   aws.String("odPrice"),
								Value: aws.String("2"),
							},
							{
								Key:   aws.String("LaunchConfigurationName"),
								Value: aws.String("myLC"),
							},
						},
					},
				},
				TargetCapacitySpecification: &ec2Types.TargetCapacitySpecificationRequest{
					DefaultTargetCapacityType: "spot",
					SpotTargetCapacity:        aws.Int32(1),
					TotalTargetCapacity:       aws.Int32(1),
				},
				Type: "instant",
			},
		},
		{
			name:      "test generating list of overrides with capacity-optimized",
			ltName:    aws.String("testLT"),
			ltVersion: aws.String("$Latest"),
			instanceTypes: []string{
				"instance-type1",
				"instance-type2",
			},
			i: &instance{
				spotProduct:  aws.String(LinuxSpotProduct),
				initialPrice: 2,
				Instance: &ec2Types.Instance{
					SubnetId: aws.String("subnet-id"),
				},
				asg: &autoScalingGroup{
					config: AutoScalingConfig{
						SpotAllocationStrategy: "capacity-optimized",
					},
					AutoScalingGroup: &autoscalingTypes.AutoScalingGroup{
						LaunchConfigurationName: aws.String("myLC"),
					},
					launchConfiguration: &launchConfiguration{
						LaunchConfiguration: &autoscalingTypes.LaunchConfiguration{
							IamInstanceProfile: aws.String("profile-name"),
							ImageId:            aws.String("ami-12345"),
							InstanceMonitoring: &autoscalingTypes.InstanceMonitoring{
								Enabled: aws.Bool(true),
							},
							KeyName: aws.String("current-key"),
							BlockDeviceMappings: []autoscalingTypes.BlockDeviceMapping{
								{
									DeviceName: aws.String("foo"),
								},
							},
							AssociatePublicIpAddress: aws.Bool(true),
							UserData:                 aws.String("userdata"),
						},
					},
				},

				region: &region{},
				typeInfo: &instanceTypeInformation{
					pricing: prices{
						LinuxSpotProduct: {
							onDemand: 1,
						},
					},
				},
			},
			want: &ec2.CreateFleetInput{
				LaunchTemplateConfigs: []ec2Types.FleetLaunchTemplateConfigRequest{
					{
						LaunchTemplateSpecification: &ec2Types.FleetLaunchTemplateSpecificationRequest{
							LaunchTemplateName: aws.String("testLT"),
							Version:            aws.String("$Latest"),
						},
						Overrides: []ec2Types.FleetLaunchTemplateOverridesRequest{
							{
								InstanceType: "instance-type1",
								SubnetId:     aws.String("subnet-id"),
							},
							{
								InstanceType: "instance-type2",
								SubnetId:     aws.String("subnet-id"),
							},
						},
					},
				},
				OnDemandOptions: &ec2Types.OnDemandOptionsRequest{
					AllocationStrategy: "prioritized",
				},
				SpotOptions: &ec2Types.SpotOptionsRequest{
					AllocationStrategy: "capacity-optimized",
					MaxTotalPrice:      aws.String("2"),
				},
				TargetCapacitySpecification: &ec2Types.TargetCapacitySpecificationRequest{
					DefaultTargetCapacityType: "spot",
					SpotTargetCapacity:        aws.Int32(1),
					TotalTargetCapacity:       aws.Int32(1),
				},
				Type: "instant",
				TagSpecifications: []ec2Types.TagSpecification{
					{
						ResourceType: "instance",
						Tags: []ec2Types.Tag{
							{
								Key:   aws.String("launched-by-autospotting"),
								Value: aws.String("true"),
							},
							{
								Key:   aws.String("launched-for-asg"),
								Value: aws.String(""),
							},
							{
								Key: aws.String("launched-for-replacing-instance"),
							},
							{
								Key:   aws.String("odPrice"),
								Value: aws.String("2"),
							},
							{
								Key:   aws.String("LaunchConfigurationName"),
								Value: aws.String("myLC"),
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			got := tt.i.createFleetInput(tt.ltName, tt.ltVersion, tt.instanceTypes, "spot")

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("instance.createFleetInput() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_instance_createFleetLaunchTemplate(t *testing.T) {
	tests := []struct {
		name          string
		ltData        *ec2Types.RequestLaunchTemplateData
		asgName       string
		cltErr        error
		wantLTName    *string
		wantLTVersion *string
		wantErr       bool
	}{
		{
			name:       "dummy asg, no errors",
			asgName:    "dummy-asg",
			wantLTName: aws.String("AutoSpotting-Converted-LaunchTemplate-for-dummy-asg-" + time.Now().Format("20060102-150405")),
		},
		{
			name:       "dummy asg, AlreadyExistsException error",
			asgName:    "dummy-asg",
			cltErr:     errors.New("AlreadyExistsException"),
			wantLTName: aws.String("AutoSpotting-Converted-LaunchTemplate-for-dummy-asg-" + time.Now().Format("20060102-150405")),
		},
		{
			name:       "dummy asg, another error",
			asgName:    "dummy-asg",
			cltErr:     errors.New("randomError"),
			wantLTName: nil,
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &instance{
				asg: &autoScalingGroup{
					name: tt.asgName,
				},
				region: &region{
					services: &connections{
						ec2: mockEC2Client{
							clterr: tt.cltErr,
						},
					},
				},
			}

			name, version, err := i.createFleetLaunchTemplate(tt.ltData)
			if (err != nil) != tt.wantErr {
				t.Errorf("instance.createFleetLaunchTemplate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if name == nil && tt.wantLTName != nil {
				t.Errorf("instance.createFleetLaunchTemplate() = nil, want LaunchTemplate Name %v", tt.wantLTName)
			}

			if name != nil && tt.wantLTName != nil && *name != *tt.wantLTName {
				t.Errorf("instance.createFleetLaunchTemplate() = %v, want LaunchTemplate Name %v", *name, *tt.wantLTName)
			}

			if version != nil && tt.wantLTVersion != nil && *version != *tt.wantLTVersion {
				t.Errorf("instance.createFleetLaunchTemplate() = %v, want LaunchTemplate Version %v", *name, *tt.wantLTVersion)
			}

		})
	}
}

func TestDeduplicateTags(t *testing.T) {
	inputTags := []ec2Types.Tag{
		{Key: aws.String("Name"), Value: aws.String("Server1")},
		{Key: aws.String("Environment"), Value: aws.String("Production")},
		{Key: aws.String("Name"), Value: aws.String("Server2")},
		{Key: aws.String("Environment"), Value: aws.String("Staging")},
		{Key: aws.String("Owner"), Value: aws.String("Alice")},
		{Key: aws.String("Owner"), Value: aws.String("Bob")},
	}

	expectedTags := []ec2Types.Tag{
		{Key: aws.String("Name"), Value: aws.String("Server1")},
		{Key: aws.String("Environment"), Value: aws.String("Production")},
		{Key: aws.String("Owner"), Value: aws.String("Alice")},
	}

	deduplicatedTags := deduplicateTags(inputTags)

	if !reflect.DeepEqual(deduplicatedTags, expectedTags) {
		t.Errorf("deduplicateTags() = %v, want %v", deduplicatedTags, expectedTags)
	}
}
