// Copyright (c) 2016-2021 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"encoding/base64"
	"errors"
	"io/ioutil"
	"reflect"
	"sort"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/davecgh/go-spew/spew"
)

func TestGenerateTagList(t *testing.T) {
	tests := []struct {
		name                     string
		ASGName                  string
		ASGLCName                string
		instanceTags             []*ec2.Tag
		instanceID               string
		expectedTagSpecification []*ec2.TagSpecification
	}{
		{name: "no tags on original instance",
			ASGLCName: "testLC0",
			ASGName:   "myASG", instanceID: "foo",
			instanceTags: []*ec2.Tag{},
			expectedTagSpecification: []*ec2.TagSpecification{
				{
					ResourceType: aws.String("instance"),
					Tags: []*ec2.Tag{
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
					},
				},
			},
		},
		{name: "Multiple tags on original instance",
			ASGLCName:  "testLC0",
			ASGName:    "myASG",
			instanceID: "bar",
			instanceTags: []*ec2.Tag{
				{
					Key:   aws.String("foo"),
					Value: aws.String("bar"),
				},
				{
					Key:   aws.String("baz"),
					Value: aws.String("bazinga"),
				},
			},
			expectedTagSpecification: []*ec2.TagSpecification{
				{
					ResourceType: aws.String("instance"),
					Tags: []*ec2.Tag{
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
				Instance: &ec2.Instance{
					Tags:       tt.instanceTags,
					InstanceId: aws.String(tt.instanceID),
				},
				asg: &autoScalingGroup{
					name: tt.ASGName,
					Group: &autoscaling.Group{
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
		BDMs []*autoscaling.BlockDeviceMapping
		i    *instance
		want []*ec2.LaunchTemplateBlockDeviceMappingRequest
	}{
		{
			name: "nil block device mapping",
			BDMs: nil,
			i:    &instance{},
			want: nil,
		},
		{
			name: "instance-store only, skipping one of the volumes from the BDMs",
			BDMs: []*autoscaling.BlockDeviceMapping{
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
			want: []*ec2.LaunchTemplateBlockDeviceMappingRequest{
				{
					DeviceName:  aws.String("/dev/ephemeral1"),
					Ebs:         nil,
					VirtualName: aws.String("bar"),
				},
			},
		},

		{
			name: "GP2 EBS to be converted to GP3 when size it below the configured threshold",
			BDMs: []*autoscaling.BlockDeviceMapping{
				{
					DeviceName:  aws.String("/dev/ephemeral0"),
					Ebs:         nil,
					VirtualName: aws.String("foo"),
				},
				{
					DeviceName: aws.String("/dev/xvda"),
					Ebs: &autoscaling.Ebs{
						DeleteOnTermination: aws.Bool(false),
						VolumeSize:          aws.Int64(10),
						VolumeType:          aws.String("gp2"),
					},
					VirtualName: aws.String("bar"),
				},
			},
			i: &instance{
				asg: &autoScalingGroup{
					name: "asg-with",
					region: &region{
						name: "not-blacklisted",
					},
					config: AutoScalingConfig{
						GP2ConversionThreshold: 100,
					},
				},
			},
			want: []*ec2.LaunchTemplateBlockDeviceMappingRequest{
				{
					DeviceName:  aws.String("/dev/ephemeral0"),
					Ebs:         nil,
					VirtualName: aws.String("foo"),
				},
				{
					DeviceName: aws.String("/dev/xvda"),
					Ebs: &ec2.LaunchTemplateEbsBlockDeviceRequest{
						DeleteOnTermination: aws.Bool(false),
						VolumeSize:          aws.Int64(10),
						VolumeType:          aws.String("gp3"),
					},
					VirtualName: aws.String("bar"),
				},
			},
		},
		{
			name: "GP2 EBS to be kept as it is when size it above the configured threshold",
			BDMs: []*autoscaling.BlockDeviceMapping{
				{
					DeviceName:  aws.String("/dev/ephemeral0"),
					Ebs:         nil,
					VirtualName: aws.String("foo"),
				},
				{
					DeviceName: aws.String("/dev/xvda"),
					Ebs: &autoscaling.Ebs{
						DeleteOnTermination: aws.Bool(false),
						VolumeSize:          aws.Int64(150),
						VolumeType:          aws.String("gp2"),
					},
					VirtualName: aws.String("bar"),
				},
			},
			i: &instance{
				asg: &autoScalingGroup{
					name: "asg-with",
					region: &region{
						name: "not-blacklisted",
					},
					config: AutoScalingConfig{
						GP2ConversionThreshold: 100,
					},
				},
			},
			want: []*ec2.LaunchTemplateBlockDeviceMappingRequest{
				{
					DeviceName:  aws.String("/dev/ephemeral0"),
					Ebs:         nil,
					VirtualName: aws.String("foo"),
				},
				{
					DeviceName: aws.String("/dev/xvda"),
					Ebs: &ec2.LaunchTemplateEbsBlockDeviceRequest{
						DeleteOnTermination: aws.Bool(false),
						VolumeSize:          aws.Int64(150),
						VolumeType:          aws.String("gp2"),
					},
					VirtualName: aws.String("bar"),
				},
			},
		},
		{
			name: "Provision IO2 EBS volume instead of IO1 in a supported region",
			BDMs: []*autoscaling.BlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/xvdb"),
					Ebs: &autoscaling.Ebs{
						DeleteOnTermination: aws.Bool(true),
						VolumeSize:          aws.Int64(20),
						VolumeType:          aws.String("io1"),
					},
					VirtualName: aws.String("baz"),
				},
			},
			i: &instance{
				asg: &autoScalingGroup{
					name: "asg-with",
					region: &region{
						name: "supported",
					},
				},
			},
			want: []*ec2.LaunchTemplateBlockDeviceMappingRequest{
				{
					DeviceName: aws.String("/dev/xvdb"),
					Ebs: &ec2.LaunchTemplateEbsBlockDeviceRequest{
						DeleteOnTermination: aws.Bool(true),
						VolumeSize:          aws.Int64(20),
						VolumeType:          aws.String("io2"),
					},
					VirtualName: aws.String("baz"),
				},
			},
		},
		{
			name: "Provision IO1 EBS volume instead of replacing to IO2 in an unsupported region",
			BDMs: []*autoscaling.BlockDeviceMapping{

				{
					DeviceName: aws.String("/dev/xvdb"),
					Ebs: &autoscaling.Ebs{
						DeleteOnTermination: aws.Bool(true),
						VolumeSize:          aws.Int64(20),
						VolumeType:          aws.String("io1"),
					},
					VirtualName: aws.String("baz"),
				},
			},
			i: &instance{
				asg: &autoScalingGroup{
					name: "asg-with",
					region: &region{
						name: "us-gov-west-1",
					},
				},
			},
			want: []*ec2.LaunchTemplateBlockDeviceMappingRequest{
				{
					DeviceName: aws.String("/dev/xvdb"),
					Ebs: &ec2.LaunchTemplateEbsBlockDeviceRequest{
						DeleteOnTermination: aws.Bool(true),
						VolumeSize:          aws.Int64(20),
						VolumeType:          aws.String("io1"),
					},
					VirtualName: aws.String("baz"),
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
		name string
		inst instance
		want []*string
	}{
		{
			name: "missing SGs",
			inst: instance{
				Instance: &ec2.Instance{
					SecurityGroups: []*ec2.GroupIdentifier{},
				},
			},
			want: []*string{},
		},
		{
			name: "single SG",
			inst: instance{
				Instance: &ec2.Instance{
					SecurityGroups: []*ec2.GroupIdentifier{{
						GroupId:   aws.String("sg-123"),
						GroupName: aws.String("foo"),
					}},
				},
			},
			want: []*string{aws.String("sg-123")},
		},
		{
			name: "multiple SGs",
			inst: instance{
				Instance: &ec2.Instance{
					SecurityGroups: []*ec2.GroupIdentifier{{
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
			want: []*string{aws.String("sg-123"), aws.String("sg-456")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.inst.convertSecurityGroups(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("instance.convertSecurityGroups() = %v, want %v",
					spew.Sdump(got), spew.Sdump(tt.want))
			}
		})
	}
}

func Test_instance_createLaunchTemplateData(t *testing.T) {
	beanstalkUserDataExample, err := ioutil.ReadFile("../test_data/beanstalk_userdata_example.txt")
	if err != nil {
		t.Errorf("Unable to read Beanstalk UserData example")
	}

	beanstalkUserDataWrappedExample, err := ioutil.ReadFile("../test_data/beanstalk_userdata_wrapped_example.txt")
	if err != nil {
		t.Errorf("Unable to read Beanstalk UserData wrapped example")
	}

	tests := []struct {
		name string
		inst instance
		want *ec2.RequestLaunchTemplateData
	}{
		{
			name: "createLaunchTemplateData() with basic launch template",
			inst: instance{
				typeInfo: instanceTypeInformation{
					pricing: prices{
						onDemand: 1.5,
					},
				},
				region: &region{
					services: connections{
						ec2: mockEC2{
							dltverr: nil,
							damio: &ec2.DescribeImagesOutput{
								Images: []*ec2.Image{},
							},
							dltvo: &ec2.DescribeLaunchTemplateVersionsOutput{
								LaunchTemplateVersions: []*ec2.LaunchTemplateVersion{
									{
										LaunchTemplateData: &ec2.ResponseLaunchTemplateData{},
									},
								},
							},
						},
					},
				},
				asg: &autoScalingGroup{
					name: "mygroup",
					Group: &autoscaling.Group{
						LaunchTemplate: &autoscaling.LaunchTemplateSpecification{
							LaunchTemplateId: aws.String("lt-id"),
							Version:          aws.String("v1"),
						},
					},
					config: AutoScalingConfig{
						OnDemandPriceMultiplier: 1,
					},
				},
				Instance: &ec2.Instance{
					EbsOptimized: aws.Bool(true),

					IamInstanceProfile: &ec2.IamInstanceProfile{
						Arn: aws.String("profile-arn"),
					}, InstanceId: aws.String("i-foo"),

					InstanceType: aws.String("t2.medium"),

					Placement: &ec2.Placement{
						Affinity: aws.String("foo"),
					},

					SecurityGroups: []*ec2.GroupIdentifier{
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
			want: &ec2.RequestLaunchTemplateData{
				EbsOptimized: aws.Bool(true),
				InstanceMarketOptions: &ec2.LaunchTemplateInstanceMarketOptionsRequest{
					MarketType: aws.String(Spot),
					SpotOptions: &ec2.LaunchTemplateSpotMarketOptionsRequest{
						MaxPrice: aws.String("1.5"),
					},
				},

				Placement: &ec2.LaunchTemplatePlacementRequest{
					Affinity: aws.String("foo"),
				},

				SecurityGroupIds: []*string{
					aws.String("sg-123"),
					aws.String("sg-456"),
				},

				TagSpecifications: []*ec2.LaunchTemplateTagSpecificationRequest{{
					ResourceType: aws.String("instance"),
					Tags: []*ec2.Tag{
						{
							Key:   aws.String("LaunchTemplateID"),
							Value: aws.String("lt-id"),
						},
						{
							Key:   aws.String("LaunchTemplateVersion"),
							Value: aws.String("v1"),
						},
						{
							Key:   aws.String("launched-by-autospotting"),
							Value: aws.String("true"),
						},
						{
							Key:   aws.String("launched-for-asg"),
							Value: aws.String("mygroup"),
						},
						{
							Key:   aws.String("launched-for-replacing-instance"),
							Value: aws.String("i-foo"),
						},
					},
				},
				},
			},
		},
		{
			name: "createLaunchTemplateData() with launch template containing advanced network configuration",
			inst: instance{
				typeInfo: instanceTypeInformation{
					pricing: prices{
						onDemand: 1.5,
					},
				},
				region: &region{
					services: connections{
						ec2: mockEC2{
							damio: &ec2.DescribeImagesOutput{
								Images: []*ec2.Image{},
							},
							dltverr: nil,
							dltvo: &ec2.DescribeLaunchTemplateVersionsOutput{
								LaunchTemplateVersions: []*ec2.LaunchTemplateVersion{
									{
										LaunchTemplateData: &ec2.ResponseLaunchTemplateData{
											KeyName: aws.String("mykey"),
											NetworkInterfaces: []*ec2.LaunchTemplateInstanceNetworkInterfaceSpecification{
												{
													Description: aws.String("dummy network interface definition"),
												},
											},
											UserData: aws.String("dummy"),
										},
									},
								},
							},
						},
					},
				},
				asg: &autoScalingGroup{
					name: "mygroup",
					config: AutoScalingConfig{
						OnDemandPriceMultiplier: 1,
					},
					Group: &autoscaling.Group{
						LaunchTemplate: &autoscaling.LaunchTemplateSpecification{
							LaunchTemplateId: aws.String("lt-id"),
							Version:          aws.String("v1"),
						},
					},
				},
				Instance: &ec2.Instance{
					EbsOptimized: aws.Bool(true),

					IamInstanceProfile: &ec2.IamInstanceProfile{
						Arn: aws.String("profile-arn"),
					},
					InstanceId:   aws.String("i-foo"),
					InstanceType: aws.String("t2.medium"),
					KeyName:      aws.String("mykey"),

					Placement: &ec2.Placement{
						Affinity: aws.String("foo"),
					},

					SecurityGroups: []*ec2.GroupIdentifier{
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
			want: &ec2.RequestLaunchTemplateData{
				EbsOptimized: aws.Bool(true),

				InstanceMarketOptions: &ec2.LaunchTemplateInstanceMarketOptionsRequest{
					MarketType: aws.String(Spot),
					SpotOptions: &ec2.LaunchTemplateSpotMarketOptionsRequest{
						MaxPrice: aws.String("1.5"),
					},
				},
				KeyName: aws.String("mykey"),
				NetworkInterfaces: []*ec2.LaunchTemplateInstanceNetworkInterfaceSpecificationRequest{
					{
						Groups:   []*string{aws.String("sg-123"), aws.String("sg-456")},
						SubnetId: aws.String("subnet-123"),
					},
				},

				Placement: &ec2.LaunchTemplatePlacementRequest{
					Affinity: aws.String("foo"),
				},

				TagSpecifications: []*ec2.LaunchTemplateTagSpecificationRequest{{
					ResourceType: aws.String("instance"),
					Tags: []*ec2.Tag{
						{
							Key:   aws.String("LaunchTemplateID"),
							Value: aws.String("lt-id"),
						},
						{
							Key:   aws.String("LaunchTemplateVersion"),
							Value: aws.String("v1"),
						},
						{
							Key:   aws.String("launched-by-autospotting"),
							Value: aws.String("true"),
						},
						{
							Key:   aws.String("launched-for-asg"),
							Value: aws.String("mygroup"),
						},
						{
							Key:   aws.String("launched-for-replacing-instance"),
							Value: aws.String("i-foo"),
						},
					},
				},
				},
				UserData: aws.String("dummy"),
			},
		},
		{
			name: "createLaunchTemplateData() with simple LC",
			inst: instance{
				typeInfo: instanceTypeInformation{
					pricing: prices{
						onDemand: 1.5,
					},
				},
				region: &region{
					services: connections{
						ec2: mockEC2{
							damio: &ec2.DescribeImagesOutput{
								Images: []*ec2.Image{},
							},
						},
					},
				},
				asg: &autoScalingGroup{
					name: "mygroup",
					config: AutoScalingConfig{
						OnDemandPriceMultiplier: 1,
					},
					Group: &autoscaling.Group{
						LaunchConfigurationName: aws.String("myLC"),
					},
					launchConfiguration: &launchConfiguration{
						LaunchConfiguration: &autoscaling.LaunchConfiguration{
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
				Instance: &ec2.Instance{
					EbsOptimized: aws.Bool(true),

					IamInstanceProfile: &ec2.IamInstanceProfile{
						Arn: aws.String("profile-arn"),
					},
					ImageId:      aws.String("ami-123"),
					InstanceId:   aws.String("i-foo"),
					InstanceType: aws.String("t2.medium"),

					Placement: &ec2.Placement{
						Affinity: aws.String("foo"),
					},

					SecurityGroups: []*ec2.GroupIdentifier{
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
			want: &ec2.RequestLaunchTemplateData{

				EbsOptimized: aws.Bool(true),

				IamInstanceProfile: &ec2.LaunchTemplateIamInstanceProfileSpecificationRequest{
					Name: aws.String("profile"),
				},

				ImageId: aws.String("ami-12345"),

				InstanceMarketOptions: &ec2.LaunchTemplateInstanceMarketOptionsRequest{
					MarketType: aws.String(Spot),
					SpotOptions: &ec2.LaunchTemplateSpotMarketOptionsRequest{
						MaxPrice: aws.String("1.5"),
					},
				},

				KeyName: aws.String("mykey"),

				Placement: &ec2.LaunchTemplatePlacementRequest{
					Affinity: aws.String("foo"),
				},

				SecurityGroupIds: []*string{
					aws.String("sg-123"),
					aws.String("sg-456"),
				},

				TagSpecifications: []*ec2.LaunchTemplateTagSpecificationRequest{{
					ResourceType: aws.String("instance"),
					Tags: []*ec2.Tag{
						{
							Key:   aws.String("LaunchConfigurationName"),
							Value: aws.String("myLC"),
						},
						{
							Key:   aws.String("launched-by-autospotting"),
							Value: aws.String("true"),
						},
						{
							Key:   aws.String("launched-for-asg"),
							Value: aws.String("mygroup"),
						}, {
							Key:   aws.String("launched-for-replacing-instance"),
							Value: aws.String("i-foo"),
						},
					},
				},
				},
				UserData: aws.String("userdata"),
			},
		},

		{
			name: "createLaunchTemplateData() with full launch configuration",
			inst: instance{
				typeInfo: instanceTypeInformation{
					pricing: prices{
						onDemand: 1.5,
					},
				},
				region: &region{
					services: connections{
						ec2: mockEC2{
							damio: &ec2.DescribeImagesOutput{
								Images: []*ec2.Image{},
							},
						},
					},
				},
				asg: &autoScalingGroup{
					name: "mygroup",
					config: AutoScalingConfig{
						OnDemandPriceMultiplier: 1,
					},
					Group: &autoscaling.Group{
						LaunchConfigurationName: aws.String("myLC"),
					},
					launchConfiguration: &launchConfiguration{
						LaunchConfiguration: &autoscaling.LaunchConfiguration{
							IamInstanceProfile: aws.String("profile-name"),
							ImageId:            aws.String("ami-12345"),
							InstanceMonitoring: &autoscaling.InstanceMonitoring{
								Enabled: aws.Bool(true),
							},
							KeyName: aws.String("current-key"),
							BlockDeviceMappings: []*autoscaling.BlockDeviceMapping{
								{
									DeviceName: aws.String("foo"),
								},
							},
							AssociatePublicIpAddress: aws.Bool(true),
							UserData:                 aws.String("userdata"),
						},
					},
				},
				Instance: &ec2.Instance{
					EbsOptimized: aws.Bool(true),

					IamInstanceProfile: &ec2.IamInstanceProfile{
						Arn: aws.String("profile-arn"),
					},
					ImageId:      aws.String("ami-123"),
					InstanceId:   aws.String("i-foo"),
					InstanceType: aws.String("t2.medium"),
					KeyName:      aws.String("older-key"),

					Placement: &ec2.Placement{
						Affinity: aws.String("foo"),
					},

					SecurityGroups: []*ec2.GroupIdentifier{
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

			want: &ec2.RequestLaunchTemplateData{
				BlockDeviceMappings: []*ec2.LaunchTemplateBlockDeviceMappingRequest{
					{
						DeviceName: aws.String("foo"),
					},
				},

				EbsOptimized: aws.Bool(true),

				IamInstanceProfile: &ec2.LaunchTemplateIamInstanceProfileSpecificationRequest{
					Name: aws.String("profile-name"),
				},

				ImageId: aws.String("ami-12345"),

				InstanceMarketOptions: &ec2.LaunchTemplateInstanceMarketOptionsRequest{
					MarketType: aws.String(Spot),
					SpotOptions: &ec2.LaunchTemplateSpotMarketOptionsRequest{
						MaxPrice: aws.String("1.5"),
					},
				},

				KeyName: aws.String("current-key"),

				Monitoring: &ec2.LaunchTemplatesMonitoringRequest{
					Enabled: aws.Bool(true),
				},

				Placement: &ec2.LaunchTemplatePlacementRequest{
					Affinity: aws.String("foo"),
				},

				NetworkInterfaces: []*ec2.LaunchTemplateInstanceNetworkInterfaceSpecificationRequest{
					{
						AssociatePublicIpAddress: aws.Bool(true),
						DeviceIndex:              aws.Int64(0),
						SubnetId:                 aws.String("subnet-123"),
						Groups: []*string{
							aws.String("sg-123"),
							aws.String("sg-456"),
						},
					},
				},

				TagSpecifications: []*ec2.LaunchTemplateTagSpecificationRequest{{
					ResourceType: aws.String("instance"),
					Tags: []*ec2.Tag{
						{
							Key:   aws.String("LaunchConfigurationName"),
							Value: aws.String("myLC"),
						},
						{
							Key:   aws.String("launched-by-autospotting"),
							Value: aws.String("true"),
						},
						{
							Key:   aws.String("launched-for-asg"),
							Value: aws.String("mygroup"),
						},
						{
							Key:   aws.String("launched-for-replacing-instance"),
							Value: aws.String("i-foo"),
						},
					},
				},
				},
				UserData: aws.String("userdata"),
			},
		},
		{
			name: "createLaunchTemplateData() with customized UserData for Beanstalk",
			inst: instance{
				typeInfo: instanceTypeInformation{
					pricing: prices{
						onDemand: 1.5,
					},
				},
				region: &region{
					services: connections{
						ec2: mockEC2{
							damio: &ec2.DescribeImagesOutput{
								Images: []*ec2.Image{},
							},
						},
					},
				},
				asg: &autoScalingGroup{
					name: "mygroup",
					Group: &autoscaling.Group{
						LaunchConfigurationName: aws.String("myLC"),
					},
					launchConfiguration: &launchConfiguration{
						LaunchConfiguration: &autoscaling.LaunchConfiguration{
							IamInstanceProfile: aws.String("profile-name"),
							ImageId:            aws.String("ami-12345"),
							InstanceMonitoring: &autoscaling.InstanceMonitoring{
								Enabled: aws.Bool(true),
							},
							KeyName: aws.String("current-key"),
							BlockDeviceMappings: []*autoscaling.BlockDeviceMapping{
								{
									DeviceName: aws.String("foo"),
								},
							},
							AssociatePublicIpAddress: aws.Bool(true),
							UserData:                 aws.String(string(beanstalkUserDataExample)),
						},
					},
					config: AutoScalingConfig{
						PatchBeanstalkUserdata:  true,
						OnDemandPriceMultiplier: 1,
					},
				},
				Instance: &ec2.Instance{
					EbsOptimized: aws.Bool(true),

					IamInstanceProfile: &ec2.IamInstanceProfile{
						Arn: aws.String("profile-arn"),
					},
					ImageId:      aws.String("ami-123"),
					InstanceType: aws.String("t2.medium"),
					KeyName:      aws.String("older-key"),

					Placement: &ec2.Placement{
						Affinity: aws.String("foo"),
					},

					SecurityGroups: []*ec2.GroupIdentifier{
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
			want: &ec2.RequestLaunchTemplateData{
				BlockDeviceMappings: []*ec2.LaunchTemplateBlockDeviceMappingRequest{
					{
						DeviceName: aws.String("foo"),
					},
				},

				EbsOptimized: aws.Bool(true),

				IamInstanceProfile: &ec2.LaunchTemplateIamInstanceProfileSpecificationRequest{
					Name: aws.String("profile-name"),
				},

				ImageId: aws.String("ami-12345"),

				InstanceMarketOptions: &ec2.LaunchTemplateInstanceMarketOptionsRequest{
					MarketType: aws.String(Spot),
					SpotOptions: &ec2.LaunchTemplateSpotMarketOptionsRequest{
						MaxPrice: aws.String("1.5"),
					},
				},

				KeyName: aws.String("current-key"),

				Monitoring: &ec2.LaunchTemplatesMonitoringRequest{
					Enabled: aws.Bool(true),
				},

				Placement: &ec2.LaunchTemplatePlacementRequest{
					Affinity: aws.String("foo"),
				},

				NetworkInterfaces: []*ec2.LaunchTemplateInstanceNetworkInterfaceSpecificationRequest{
					{
						AssociatePublicIpAddress: aws.Bool(true),
						DeviceIndex:              aws.Int64(0),
						SubnetId:                 aws.String("subnet-123"),
						Groups: []*string{
							aws.String("sg-123"),
							aws.String("sg-456"),
						},
					},
				},

				TagSpecifications: []*ec2.LaunchTemplateTagSpecificationRequest{{
					ResourceType: aws.String("instance"),
					Tags: []*ec2.Tag{
						{
							Key:   aws.String("LaunchConfigurationName"),
							Value: aws.String("myLC"),
						},
						{
							Key:   aws.String("launched-by-autospotting"),
							Value: aws.String("true"),
						},
						{
							Key:   aws.String("launched-for-asg"),
							Value: aws.String("mygroup"),
						},
						{
							Key: aws.String("launched-for-replacing-instance"),
						},
					},
				},
				},
				UserData: aws.String(base64.StdEncoding.EncodeToString(beanstalkUserDataWrappedExample)),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			got, _ := tt.inst.createLaunchTemplateData()

			// make sure the lists of tags are sorted, otherwise the comparison fails
			sort.Slice(got.TagSpecifications[0].Tags, func(i, j int) bool {
				return *got.TagSpecifications[0].Tags[i].Key < *got.TagSpecifications[0].Tags[j].Key
			})
			sort.Slice(tt.want.TagSpecifications[0].Tags, func(i, j int) bool {
				return *tt.want.TagSpecifications[0].Tags[i].Key < *tt.want.TagSpecifications[0].Tags[j].Key
			})

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Instance.createLaunchTemplateData() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_instance_convertLaunchTemplateBlockDeviceMappings(t *testing.T) {
	tests := []struct {
		name string
		BDMs []*ec2.LaunchTemplateBlockDeviceMapping
		i    *instance
		want []*ec2.LaunchTemplateBlockDeviceMappingRequest
	}{
		{
			name: "nil block device mapping",
			BDMs: nil,
			i:    &instance{},
			want: nil,
		},
		{
			name: "instance-store only, skipping one of the volumes from the BDMs",
			BDMs: []*ec2.LaunchTemplateBlockDeviceMapping{
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
			want: []*ec2.LaunchTemplateBlockDeviceMappingRequest{
				{
					DeviceName:  aws.String("/dev/ephemeral1"),
					Ebs:         nil,
					VirtualName: aws.String("bar"),
				},
			},
		},

		{
			name: "GP2 EBS to be converted to GP3 when size it below the configured threshold",
			BDMs: []*ec2.LaunchTemplateBlockDeviceMapping{
				{
					DeviceName:  aws.String("/dev/ephemeral0"),
					Ebs:         nil,
					VirtualName: aws.String("foo"),
				},
				{
					DeviceName: aws.String("/dev/xvda"),
					Ebs: &ec2.LaunchTemplateEbsBlockDevice{
						DeleteOnTermination: aws.Bool(false),
						VolumeSize:          aws.Int64(10),
						VolumeType:          aws.String("gp2"),
					},
					VirtualName: aws.String("bar"),
				},
			},
			i: &instance{
				asg: &autoScalingGroup{
					name: "asg-with",
					region: &region{
						name: "not-blacklisted",
					},
					config: AutoScalingConfig{
						GP2ConversionThreshold: 100,
					},
				},
			},
			want: []*ec2.LaunchTemplateBlockDeviceMappingRequest{
				{
					DeviceName:  aws.String("/dev/ephemeral0"),
					Ebs:         nil,
					VirtualName: aws.String("foo"),
				},
				{
					DeviceName: aws.String("/dev/xvda"),
					Ebs: &ec2.LaunchTemplateEbsBlockDeviceRequest{
						DeleteOnTermination: aws.Bool(false),
						VolumeSize:          aws.Int64(10),
						VolumeType:          aws.String("gp3"),
					},
					VirtualName: aws.String("bar"),
				},
			},
		},
		{
			name: "GP2 EBS to be kept as it is when size it above the configured threshold",
			BDMs: []*ec2.LaunchTemplateBlockDeviceMapping{
				{
					DeviceName:  aws.String("/dev/ephemeral0"),
					Ebs:         nil,
					VirtualName: aws.String("foo"),
				},
				{
					DeviceName: aws.String("/dev/xvda"),
					Ebs: &ec2.LaunchTemplateEbsBlockDevice{
						DeleteOnTermination: aws.Bool(false),
						VolumeSize:          aws.Int64(150),
						VolumeType:          aws.String("gp2"),
					},
					VirtualName: aws.String("bar"),
				},
			},
			i: &instance{
				asg: &autoScalingGroup{
					name: "asg-with",
					region: &region{
						name: "not-blacklisted",
					},
					config: AutoScalingConfig{
						GP2ConversionThreshold: 100,
					},
				},
			},
			want: []*ec2.LaunchTemplateBlockDeviceMappingRequest{
				{
					DeviceName:  aws.String("/dev/ephemeral0"),
					Ebs:         nil,
					VirtualName: aws.String("foo"),
				},
				{
					DeviceName: aws.String("/dev/xvda"),
					Ebs: &ec2.LaunchTemplateEbsBlockDeviceRequest{
						DeleteOnTermination: aws.Bool(false),
						VolumeSize:          aws.Int64(150),
						VolumeType:          aws.String("gp2"),
					},
					VirtualName: aws.String("bar"),
				},
			},
		},
		{
			name: "Provision IO2 EBS volume instead of IO1 in a supported region",
			BDMs: []*ec2.LaunchTemplateBlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/xvdb"),
					Ebs: &ec2.LaunchTemplateEbsBlockDevice{
						DeleteOnTermination: aws.Bool(true),
						VolumeSize:          aws.Int64(20),
						VolumeType:          aws.String("io1"),
					},
					VirtualName: aws.String("baz"),
				},
			},
			i: &instance{
				asg: &autoScalingGroup{
					name: "asg-with",
					region: &region{
						name: "supported",
					},
				},
			},
			want: []*ec2.LaunchTemplateBlockDeviceMappingRequest{
				{
					DeviceName: aws.String("/dev/xvdb"),
					Ebs: &ec2.LaunchTemplateEbsBlockDeviceRequest{
						DeleteOnTermination: aws.Bool(true),
						VolumeSize:          aws.Int64(20),
						VolumeType:          aws.String("io2"),
					},
					VirtualName: aws.String("baz"),
				},
			},
		},
		{
			name: "Provision IO1 EBS volume instead of replacing to IO2 in an unsupported region",
			BDMs: []*ec2.LaunchTemplateBlockDeviceMapping{

				{
					DeviceName: aws.String("/dev/xvdb"),
					Ebs: &ec2.LaunchTemplateEbsBlockDevice{
						DeleteOnTermination: aws.Bool(true),
						VolumeSize:          aws.Int64(20),
						VolumeType:          aws.String("io1"),
					},
					VirtualName: aws.String("baz"),
				},
			},
			i: &instance{
				asg: &autoScalingGroup{
					name: "asg-with",
					region: &region{
						name: "us-gov-west-1",
					},
				},
			},
			want: []*ec2.LaunchTemplateBlockDeviceMappingRequest{
				{
					DeviceName: aws.String("/dev/xvdb"),
					Ebs: &ec2.LaunchTemplateEbsBlockDeviceRequest{
						DeleteOnTermination: aws.Bool(true),
						VolumeSize:          aws.Int64(20),
						VolumeType:          aws.String("io1"),
					},
					VirtualName: aws.String("baz"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.i.convertLaunchTemplateBlockDeviceMappings(tt.BDMs); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("instance.convertLaunchTemplateBlockDeviceMappings() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_instance_convertImageBlockDeviceMappings(t *testing.T) {
	tests := []struct {
		name string
		BDMs []*ec2.BlockDeviceMapping
		i    *instance
		want []*ec2.LaunchTemplateBlockDeviceMappingRequest
	}{
		{
			name: "nil block device mapping",
			BDMs: nil,
			i:    &instance{},
			want: nil,
		},
		{
			name: "instance-store only, skipping one of the volumes from the BDMs",
			BDMs: []*ec2.BlockDeviceMapping{
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
			want: []*ec2.LaunchTemplateBlockDeviceMappingRequest{
				{
					DeviceName:  aws.String("/dev/ephemeral1"),
					Ebs:         nil,
					VirtualName: aws.String("bar"),
				},
			},
		},

		{
			name: "GP2 EBS to be converted to GP3 when size it below the configured threshold",
			BDMs: []*ec2.BlockDeviceMapping{
				{
					DeviceName:  aws.String("/dev/ephemeral0"),
					Ebs:         nil,
					VirtualName: aws.String("foo"),
				},
				{
					DeviceName: aws.String("/dev/xvda"),
					Ebs: &ec2.EbsBlockDevice{
						DeleteOnTermination: aws.Bool(false),
						VolumeSize:          aws.Int64(10),
						VolumeType:          aws.String("gp2"),
					},
					VirtualName: aws.String("bar"),
				},
			},
			i: &instance{
				asg: &autoScalingGroup{
					name: "asg-with",
					region: &region{
						name: "not-blacklisted",
					},
					config: AutoScalingConfig{
						GP2ConversionThreshold: 100,
					},
				},
			},
			want: []*ec2.LaunchTemplateBlockDeviceMappingRequest{
				{
					DeviceName:  aws.String("/dev/ephemeral0"),
					Ebs:         nil,
					VirtualName: aws.String("foo"),
				},
				{
					DeviceName: aws.String("/dev/xvda"),
					Ebs: &ec2.LaunchTemplateEbsBlockDeviceRequest{
						DeleteOnTermination: aws.Bool(false),
						VolumeSize:          aws.Int64(10),
						VolumeType:          aws.String("gp3"),
					},
					VirtualName: aws.String("bar"),
				},
			},
		},
		{
			name: "GP2 EBS to be kept as it is when size it above the configured threshold",
			BDMs: []*ec2.BlockDeviceMapping{
				{
					DeviceName:  aws.String("/dev/ephemeral0"),
					Ebs:         nil,
					VirtualName: aws.String("foo"),
				},
				{
					DeviceName: aws.String("/dev/xvda"),
					Ebs: &ec2.EbsBlockDevice{
						DeleteOnTermination: aws.Bool(false),
						VolumeSize:          aws.Int64(150),
						VolumeType:          aws.String("gp2"),
					},
					VirtualName: aws.String("bar"),
				},
			},
			i: &instance{
				asg: &autoScalingGroup{
					name: "asg-with",
					region: &region{
						name: "not-blacklisted",
					},
					config: AutoScalingConfig{
						GP2ConversionThreshold: 100,
					},
				},
			},
			want: []*ec2.LaunchTemplateBlockDeviceMappingRequest{
				{
					DeviceName:  aws.String("/dev/ephemeral0"),
					Ebs:         nil,
					VirtualName: aws.String("foo"),
				},
				{
					DeviceName: aws.String("/dev/xvda"),
					Ebs: &ec2.LaunchTemplateEbsBlockDeviceRequest{
						DeleteOnTermination: aws.Bool(false),
						VolumeSize:          aws.Int64(150),
						VolumeType:          aws.String("gp2"),
					},
					VirtualName: aws.String("bar"),
				},
			},
		},
		{
			name: "Provision IO2 EBS volume instead of IO1 in a supported region",
			BDMs: []*ec2.BlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/xvdb"),
					Ebs: &ec2.EbsBlockDevice{
						DeleteOnTermination: aws.Bool(true),
						VolumeSize:          aws.Int64(20),
						VolumeType:          aws.String("io1"),
					},
					VirtualName: aws.String("baz"),
				},
			},
			i: &instance{
				asg: &autoScalingGroup{
					name: "asg-with",
					region: &region{
						name: "supported",
					},
				},
			},
			want: []*ec2.LaunchTemplateBlockDeviceMappingRequest{
				{
					DeviceName: aws.String("/dev/xvdb"),
					Ebs: &ec2.LaunchTemplateEbsBlockDeviceRequest{
						DeleteOnTermination: aws.Bool(true),
						VolumeSize:          aws.Int64(20),
						VolumeType:          aws.String("io2"),
					},
					VirtualName: aws.String("baz"),
				},
			},
		},
		{
			name: "Provision IO1 EBS volume instead of replacing to IO2 in an unsupported region",
			BDMs: []*ec2.BlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/xvdb"),
					Ebs: &ec2.EbsBlockDevice{
						DeleteOnTermination: aws.Bool(true),
						VolumeSize:          aws.Int64(20),
						VolumeType:          aws.String("io1"),
					},
					VirtualName: aws.String("baz"),
				},
			},
			i: &instance{
				asg: &autoScalingGroup{
					name: "asg-with",
					region: &region{
						name: "us-gov-west-1",
					},
				},
			},
			want: []*ec2.LaunchTemplateBlockDeviceMappingRequest{
				{
					DeviceName: aws.String("/dev/xvdb"),
					Ebs: &ec2.LaunchTemplateEbsBlockDeviceRequest{
						DeleteOnTermination: aws.Bool(true),
						VolumeSize:          aws.Int64(20),
						VolumeType:          aws.String("io1"),
					},
					VirtualName: aws.String("baz"),
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

func Test_convertLaunchConfigurationEBSVolumeType(t *testing.T) {
	tests := []struct {
		name string
		ebs  *autoscaling.Ebs
		a    *autoScalingGroup
		want *string
	}{
		{
			name: "nil volume type",
			a: &autoScalingGroup{
				region: &region{
					name: "us-east-1",
				},
			},
			ebs: &autoscaling.Ebs{
				VolumeType: nil,
			},
			want: nil,
		},

		{
			name: "IO1 in region supported by IO2",
			a: &autoScalingGroup{
				region: &region{
					name: "us-east-1",
				},
			},
			ebs: &autoscaling.Ebs{
				VolumeType: aws.String("io1"),
			},
			want: aws.String("io2"),
		},
		{
			name: "IO1 in region not supported by IO2",
			a: &autoScalingGroup{
				region: &region{
					name: "cn-northwest-1",
				},
			},
			ebs: &autoscaling.Ebs{
				VolumeType: aws.String("io1"),
			},
			want: aws.String("io1"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := convertLaunchConfigurationEBSVolumeType(tt.ebs, tt.a); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("convertLaunchConfigurationEBSVolumeType() = %v, want %v", spew.Sdump(got), spew.Sdump(tt.want))
			}
		})
	}
}

func Test_instance_createFleetInput(t *testing.T) {

	tests := []struct {
		name          string
		i             *instance
		ltName        *string
		instanceTypes []*string
		want          *ec2.CreateFleetInput
	}{
		{
			name:   "test generating list of overrides with capacity-optimized-prioritized",
			ltName: aws.String("testLT"),
			instanceTypes: []*string{
				aws.String("instance-type1"),
				aws.String("instance-type2"),
			},
			i: &instance{
				Instance: &ec2.Instance{
					SubnetId: aws.String("subnet-id"),
				},
				asg: &autoScalingGroup{
					config: AutoScalingConfig{
						SpotAllocationStrategy: "capacity-optimized-prioritized",
					},
				},
				region: &region{},
				typeInfo: instanceTypeInformation{
					pricing: prices{
						onDemand: 1,
					},
				},
			},
			want: &ec2.CreateFleetInput{
				LaunchTemplateConfigs: []*ec2.FleetLaunchTemplateConfigRequest{
					{
						LaunchTemplateSpecification: &ec2.FleetLaunchTemplateSpecificationRequest{
							LaunchTemplateName: aws.String("testLT"),
							Version:            aws.String("$Latest"),
						},
						Overrides: []*ec2.FleetLaunchTemplateOverridesRequest{
							{
								InstanceType: aws.String("instance-type1"),
								Priority:     aws.Float64(0),
								SubnetId:     aws.String("subnet-id"),
							},
							{
								InstanceType: aws.String("instance-type2"),
								Priority:     aws.Float64(1),
								SubnetId:     aws.String("subnet-id"),
							},
						},
					},
				},
				SpotOptions: &ec2.SpotOptionsRequest{
					AllocationStrategy: aws.String("capacity-optimized-prioritized"),
				},
				TargetCapacitySpecification: &ec2.TargetCapacitySpecificationRequest{
					DefaultTargetCapacityType: aws.String("spot"),
					SpotTargetCapacity:        aws.Int64(1),
					TotalTargetCapacity:       aws.Int64(1),
				},
				Type: aws.String("instant"),
			},
		},
		{
			name:   "test generating list of overrides with capacity-optimized",
			ltName: aws.String("testLT"),
			instanceTypes: []*string{
				aws.String("instance-type1"),
				aws.String("instance-type2"),
			},
			i: &instance{
				Instance: &ec2.Instance{
					SubnetId: aws.String("subnet-id"),
				},
				asg: &autoScalingGroup{
					config: AutoScalingConfig{
						SpotAllocationStrategy: "capacity-optimized",
					},
				},
				region: &region{},
				typeInfo: instanceTypeInformation{
					pricing: prices{
						onDemand: 1,
					},
				},
			},
			want: &ec2.CreateFleetInput{
				LaunchTemplateConfigs: []*ec2.FleetLaunchTemplateConfigRequest{
					{
						LaunchTemplateSpecification: &ec2.FleetLaunchTemplateSpecificationRequest{
							LaunchTemplateName: aws.String("testLT"),
							Version:            aws.String("$Latest"),
						},
						Overrides: []*ec2.FleetLaunchTemplateOverridesRequest{
							{
								InstanceType: aws.String("instance-type1"),
								SubnetId:     aws.String("subnet-id"),
							},
							{
								InstanceType: aws.String("instance-type2"),
								SubnetId:     aws.String("subnet-id"),
							},
						},
					},
				},
				SpotOptions: &ec2.SpotOptionsRequest{
					AllocationStrategy: aws.String("capacity-optimized"),
				},
				TargetCapacitySpecification: &ec2.TargetCapacitySpecificationRequest{
					DefaultTargetCapacityType: aws.String("spot"),
					SpotTargetCapacity:        aws.Int64(1),
					TotalTargetCapacity:       aws.Int64(1),
				},
				Type: aws.String("instant"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			got := tt.i.createFleetInput(tt.ltName, tt.instanceTypes)

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("instance.createFleetInput() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_instance_createFleetLaunchTemplate(t *testing.T) {
	tests := []struct {
		name       string
		ltData     *ec2.RequestLaunchTemplateData
		instanceID string
		cltErr     error
		want       *string
		wantErr    bool
	}{
		{
			name:       "dummy instance, no errors",
			instanceID: "i-dummy",
			want:       aws.String("AutoSpotting-Temporary-LaunchTemplate-for-i-dummy"),
		},
		{
			name:       "dummy instance, AlreadyExistsException error",
			instanceID: "i-dummy",
			cltErr:     errors.New("AlreadyExistsException"),
			want:       aws.String("AutoSpotting-Temporary-LaunchTemplate-for-i-dummy"),
		},
		{
			name:       "dummy instance, another error",
			instanceID: "i-dummy",
			cltErr:     errors.New("randomError"),
			want:       nil,
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &instance{
				Instance: &ec2.Instance{
					InstanceId: aws.String(tt.instanceID),
				},
				region: &region{
					services: connections{
						ec2: mockEC2{
							clterr: tt.cltErr,
						},
					},
				},
			}

			got, err := i.createFleetLaunchTemplate(tt.ltData)
			if (err != nil) != tt.wantErr {
				t.Errorf("instance.createFleetLaunchTemplate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got == nil && tt.want != nil {
				t.Errorf("instance.createFleetLaunchTemplate() = nil, want %v", tt.want)
			}

			if got != nil && tt.want != nil && *got != *tt.want {
				t.Errorf("instance.createFleetLaunchTemplate() = %v, want %v", *got, tt.want)
			}
		})
	}
}
