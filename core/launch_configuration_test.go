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
			&autoscaling.BlockDeviceMapping{
				DeviceName:  aws.String("/dev/ephemeral0"),
				Ebs:         nil,
				NoDevice:    aws.Bool(false),
				VirtualName: aws.String("foo"),
			},
			&autoscaling.BlockDeviceMapping{
				DeviceName:  aws.String("/dev/ephemeral1"),
				Ebs:         nil,
				NoDevice:    aws.Bool(false),
				VirtualName: aws.String("bar"),
			},
		},
		want: []*ec2.BlockDeviceMapping{
			&ec2.BlockDeviceMapping{
				DeviceName:  aws.String("/dev/ephemeral0"),
				Ebs:         nil,
				NoDevice:    aws.String("false"),
				VirtualName: aws.String("foo"),
			},
			&ec2.BlockDeviceMapping{
				DeviceName:  aws.String("/dev/ephemeral1"),
				Ebs:         nil,
				NoDevice:    aws.String("false"),
				VirtualName: aws.String("bar"),
			},
		},
	},
		{name: "instance-store and EBS",
			asbdm: []*autoscaling.BlockDeviceMapping{
				&autoscaling.BlockDeviceMapping{
					DeviceName:  aws.String("/dev/ephemeral0"),
					Ebs:         nil,
					NoDevice:    aws.Bool(false),
					VirtualName: aws.String("foo"),
				},
				&autoscaling.BlockDeviceMapping{
					DeviceName: aws.String("/dev/xvda"),
					Ebs: &autoscaling.Ebs{
						DeleteOnTermination: aws.Bool(false),
						VolumeSize:          aws.Int64(10),
					},
					VirtualName: aws.String("bar"),
				},
				&autoscaling.BlockDeviceMapping{
					DeviceName: aws.String("/dev/xvdb"),
					Ebs: &autoscaling.Ebs{
						DeleteOnTermination: aws.Bool(true),
						VolumeSize:          aws.Int64(20),
					},
					VirtualName: aws.String("baz"),
				},
			},
			want: []*ec2.BlockDeviceMapping{
				&ec2.BlockDeviceMapping{
					DeviceName:  aws.String("/dev/ephemeral0"),
					Ebs:         nil,
					NoDevice:    aws.String("false"),
					VirtualName: aws.String("foo"),
				},
				&ec2.BlockDeviceMapping{
					DeviceName: aws.String("/dev/xvda"),
					Ebs: &ec2.EbsBlockDevice{
						DeleteOnTermination: aws.Bool(false),
						VolumeSize:          aws.Int64(10),
					},
					VirtualName: aws.String("bar"),
				},
				&ec2.BlockDeviceMapping{
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
