package autospotting

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/service/ec2"
)

func Test_getRegions(t *testing.T) {

	tests := []struct {
		name    string
		ec2conn mockEC2
		want    []string
		wantErr bool
	}{{
		name: "return some regions",
		ec2conn: mockEC2{
			dro: &ec2.DescribeRegionsOutput{
				Regions: []*ec2.Region{
					{RegionName: aws.String("foo")},
					{RegionName: aws.String("bar")},
				},
			},
			drerr: nil,
		},
		want:    []string{"foo", "bar"},
		wantErr: false,
	},
		{
			name: "return an error",
			ec2conn: mockEC2{
				dro: &ec2.DescribeRegionsOutput{
					Regions: []*ec2.Region{
						{RegionName: aws.String("foo")},
						{RegionName: aws.String("bar")},
					},
				},
				drerr: fmt.Errorf("fooErr"),
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getRegions(tt.ec2conn)
			if (err != nil) != tt.wantErr {
				t.Errorf("getRegions() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getRegions() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_connectEC2(t *testing.T) {
	tests := []struct {
		name   string
		region string
		want   ec2.EC2
	}{
		{
			name:   "connect to a region",
			region: "us-east-1",
			want: ec2.EC2{
				Client: &client.Client{
					Config: aws.Config{
						Region: aws.String("us-east-1"),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := connectEC2(tt.region)
			if !reflect.DeepEqual(
				got.Client.Config.Region,
				tt.want.Client.Config.Region) {
				t.Errorf("connectEC2() = %v, want %v",
					got.Client.Config.Region,
					tt.want.Client.Config.Region)
			}
		})
	}
}
