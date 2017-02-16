package autospotting

import (
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
)

func Test_copyBlockDeviceMappings(t *testing.T) {

	tests := []struct {
		name  string
		asbdm []*autoscaling.BlockDeviceMapping
		want  []*ec2.BlockDeviceMapping
	}{{name: "instance-store only",
		asbdm: []*autoscaling.BlockDeviceMapping{
			{
				DeviceName:  aws.String("/dev/ephemeral0"),
				Ebs:         nil,
				NoDevice:    aws.Bool(false),
				VirtualName: aws.String("foo"),
			},
			{
				DeviceName:  aws.String("/dev/ephemeral1"),
				Ebs:         nil,
				NoDevice:    aws.Bool(false),
				VirtualName: aws.String("bar"),
			},
		},
		want: []*ec2.BlockDeviceMapping{
			{
				DeviceName:  aws.String("/dev/ephemeral0"),
				Ebs:         nil,
				NoDevice:    aws.String("false"),
				VirtualName: aws.String("foo"),
			},
			{
				DeviceName:  aws.String("/dev/ephemeral1"),
				Ebs:         nil,
				NoDevice:    aws.String("false"),
				VirtualName: aws.String("bar"),
			},
		},
	},
		{name: "instance-store and EBS",
			asbdm: []*autoscaling.BlockDeviceMapping{
				{
					DeviceName:  aws.String("/dev/ephemeral0"),
					Ebs:         nil,
					NoDevice:    aws.Bool(false),
					VirtualName: aws.String("foo"),
				},
				{
					DeviceName: aws.String("/dev/xvda"),
					Ebs: &autoscaling.Ebs{
						DeleteOnTermination: aws.Bool(false),
						VolumeSize:          aws.Int64(10),
					},
					VirtualName: aws.String("bar"),
				},
				{
					DeviceName: aws.String("/dev/xvdb"),
					Ebs: &autoscaling.Ebs{
						DeleteOnTermination: aws.Bool(true),
						VolumeSize:          aws.Int64(20),
					},
					VirtualName: aws.String("baz"),
				},
			},
			want: []*ec2.BlockDeviceMapping{
				{
					DeviceName:  aws.String("/dev/ephemeral0"),
					Ebs:         nil,
					NoDevice:    aws.String("false"),
					VirtualName: aws.String("foo"),
				},
				{
					DeviceName: aws.String("/dev/xvda"),
					Ebs: &ec2.EbsBlockDevice{
						DeleteOnTermination: aws.Bool(false),
						VolumeSize:          aws.Int64(10),
					},
					VirtualName: aws.String("bar"),
				},
				{
					DeviceName: aws.String("/dev/xvdb"),
					Ebs: &ec2.EbsBlockDevice{
						DeleteOnTermination: aws.Bool(true),
						VolumeSize:          aws.Int64(20),
					},
					VirtualName: aws.String("baz"),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := copyBlockDeviceMappings(tt.asbdm); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("copyBlockDeviceMappings() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_countLaunchConfigEphemeralVolumes(t *testing.T) {
	tests := []struct {
		name  string
		lc    *launchConfiguration
		count int
	}{
		{
			name: "empty launchConfiguration",
			lc: &launchConfiguration{
				&autoscaling.LaunchConfiguration{
					BlockDeviceMappings: nil,
				},
			},
			count: 0,
		},
		{
			name: "empty BlockDeviceMappings",
			lc: &launchConfiguration{
				&autoscaling.LaunchConfiguration{
					BlockDeviceMappings: []*autoscaling.BlockDeviceMapping{
						{},
					},
				},
			},
			count: 0,
		},
		{
			name: "mix of valid and invalid configuration",
			lc: &launchConfiguration{
				&autoscaling.LaunchConfiguration{
					BlockDeviceMappings: []*autoscaling.BlockDeviceMapping{
						{VirtualName: aws.String("ephemeral")},
						{},
					},
				},
			},
			count: 1,
		},
		{
			name: "valid configuration",
			lc: &launchConfiguration{
				&autoscaling.LaunchConfiguration{
					BlockDeviceMappings: []*autoscaling.BlockDeviceMapping{
						{VirtualName: aws.String("ephemeral")},
						{VirtualName: aws.String("ephemeral")},
					},
				},
			},
			count: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			count := tc.lc.countLaunchConfigEphemeralVolumes()
			if count != tc.count {
				t.Errorf("count expected: %d, actual: %d", tc.count, count)
			}
		})
	}
}

func Test_convertLaunchConfigurationToSpotSpecification(t *testing.T) {
	tests := []struct {
		name         string
		lc           *launchConfiguration
		instance     *instance
		instanceType string
		az           string
		spotRequest  *ec2.RequestSpotLaunchSpecification
	}{
		{
			name: "empty everything",
			lc: &launchConfiguration{
				&autoscaling.LaunchConfiguration{},
			},
			instance: &instance{
				Instance: &ec2.Instance{},
			},
			spotRequest: &ec2.RequestSpotLaunchSpecification{
				InstanceType: aws.String(""),
				Placement: &ec2.SpotPlacement{
					AvailabilityZone: aws.String(""),
				},
			},
		},
		{
			name: "empty structs, but with az and instanceType",
			lc: &launchConfiguration{
				&autoscaling.LaunchConfiguration{},
			},
			instance: &instance{
				Instance: &ec2.Instance{},
			},
			spotRequest: &ec2.RequestSpotLaunchSpecification{
				InstanceType: aws.String("instance"),
				Placement: &ec2.SpotPlacement{
					AvailabilityZone: aws.String("zone"),
				},
			},
			az:           "zone",
			instanceType: "instance",
		},
		{
			name: "ESB optimized",
			lc: &launchConfiguration{
				&autoscaling.LaunchConfiguration{
					EbsOptimized: aws.Bool(true),
				},
			},
			instance: &instance{
				Instance: &ec2.Instance{},
			},
			spotRequest: &ec2.RequestSpotLaunchSpecification{
				EbsOptimized: aws.Bool(true),
				InstanceType: aws.String(""),
				Placement: &ec2.SpotPlacement{
					AvailabilityZone: aws.String(""),
				},
			},
		},
		{
			name: "IAM instance profile ARN",
			lc: &launchConfiguration{
				&autoscaling.LaunchConfiguration{
					IamInstanceProfile: aws.String("arn:aws:something"),
				},
			},
			instance: &instance{
				Instance: &ec2.Instance{},
			},
			spotRequest: &ec2.RequestSpotLaunchSpecification{
				IamInstanceProfile: &ec2.IamInstanceProfileSpecification{
					Arn: aws.String("arn:aws:something"),
				},
				InstanceType: aws.String(""),
				Placement: &ec2.SpotPlacement{
					AvailabilityZone: aws.String(""),
				},
			},
		},
		{
			name: "IAM instance profile name",
			lc: &launchConfiguration{
				&autoscaling.LaunchConfiguration{
					IamInstanceProfile: aws.String("bla bla bla something"),
				},
			},
			instance: &instance{
				Instance: &ec2.Instance{},
			},
			spotRequest: &ec2.RequestSpotLaunchSpecification{
				IamInstanceProfile: &ec2.IamInstanceProfileSpecification{
					Name: aws.String("bla bla bla something"),
				},
				InstanceType: aws.String(""),
				Placement: &ec2.SpotPlacement{
					AvailabilityZone: aws.String(""),
				},
			},
		},
		{
			name: "IAM instance profile key",
			lc: &launchConfiguration{
				&autoscaling.LaunchConfiguration{
					KeyName: aws.String("key xyz"),
				},
			},
			instance: &instance{
				Instance: &ec2.Instance{},
			},
			spotRequest: &ec2.RequestSpotLaunchSpecification{
				KeyName:      aws.String("key xyz"),
				InstanceType: aws.String(""),
				Placement: &ec2.SpotPlacement{
					AvailabilityZone: aws.String(""),
				},
			},
		},
		{
			name: "instance monitoring",
			lc: &launchConfiguration{
				&autoscaling.LaunchConfiguration{
					InstanceMonitoring: &autoscaling.InstanceMonitoring{
						Enabled: aws.Bool(false),
					},
				},
			},
			instance: &instance{
				Instance: &ec2.Instance{},
			},
			spotRequest: &ec2.RequestSpotLaunchSpecification{
				Monitoring: &ec2.RunInstancesMonitoringEnabled{
					Enabled: aws.Bool(false),
				},
				InstanceType: aws.String(""),
				Placement: &ec2.SpotPlacement{
					AvailabilityZone: aws.String(""),
				},
			},
		},
		{
			name: "user data",
			lc: &launchConfiguration{
				&autoscaling.LaunchConfiguration{
					UserData: aws.String("user data"),
				},
			},
			instance: &instance{
				Instance: &ec2.Instance{},
			},
			spotRequest: &ec2.RequestSpotLaunchSpecification{
				UserData:     aws.String("user data"),
				InstanceType: aws.String(""),
				Placement: &ec2.SpotPlacement{
					AvailabilityZone: aws.String(""),
				},
			},
		},
		{
			name: "networking",
			lc: &launchConfiguration{
				&autoscaling.LaunchConfiguration{
					AssociatePublicIpAddress: aws.Bool(true),
				},
			},
			instance: &instance{
				Instance: &ec2.Instance{},
			},
			spotRequest: &ec2.RequestSpotLaunchSpecification{
				NetworkInterfaces: []*ec2.InstanceNetworkInterfaceSpecification{
					{
						AssociatePublicIpAddress: aws.Bool(true),
						DeviceIndex:              aws.Int64(0),
					},
				},
				InstanceType: aws.String(""),
				Placement: &ec2.SpotPlacement{
					AvailabilityZone: aws.String(""),
				},
			},
		},
		{
			name: "full configuration",
			lc: &launchConfiguration{
				&autoscaling.LaunchConfiguration{
					AssociatePublicIpAddress: aws.Bool(true),
					UserData:                 aws.String("user data"),
					InstanceMonitoring: &autoscaling.InstanceMonitoring{
						Enabled: aws.Bool(false),
					},
					KeyName:      aws.String("key xyz"),
					EbsOptimized: aws.Bool(true),
				},
			},
			instance: &instance{
				Instance: &ec2.Instance{},
			},
			spotRequest: &ec2.RequestSpotLaunchSpecification{
				EbsOptimized: aws.Bool(true),
				UserData:     aws.String("user data"),
				NetworkInterfaces: []*ec2.InstanceNetworkInterfaceSpecification{
					{
						AssociatePublicIpAddress: aws.Bool(true),
						DeviceIndex:              aws.Int64(0),
					},
				},
				KeyName: aws.String("key xyz"),
				Monitoring: &ec2.RunInstancesMonitoringEnabled{
					Enabled: aws.Bool(false),
				},
				InstanceType: aws.String(""),
				Placement: &ec2.SpotPlacement{
					AvailabilityZone: aws.String(""),
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			spot := tc.lc.convertLaunchConfigurationToSpotSpecification(tc.instance, tc.instanceType, tc.az)
			if !reflect.DeepEqual(spot, tc.spotRequest) {
				t.Errorf("expected: %+v\nactual: %+v", tc.spotRequest, spot)
			}
		})
	}
}
