package autospotting

import (
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

func Test_region_enabled(t *testing.T) {
	type fields struct {
		name string
		conf Config
	}
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

func TestRequestSpotInstanceTypes(t *testing.T) {
	tests := []struct {
		name    string
		want    []string
		tregion *region
	}{
		{
			name: "Test with single instance",
			want: []string{"m3.large"},
			tregion: &region{
				instances: makeInstances(),
				conf:      &Config{},
				services: connections{
					ec2: mockEC2{
						dspho: &ec2.DescribeSpotPriceHistoryOutput{
							SpotPriceHistory: []*ec2.SpotPrice{
								{
									InstanceType: aws.String("m3.large"),
									SpotPrice:    aws.String("1"),
								},
							},
						},
						dspherr: nil,
					},
				},
			},
		},
		{
			name: "Test empty instance",
			want: []string{""},
			tregion: &region{
				instances: makeInstances(),
				conf:      &Config{},
				services: connections{
					ec2: mockEC2{
						dspho: &ec2.DescribeSpotPriceHistoryOutput{
							SpotPriceHistory: []*ec2.SpotPrice{
								{
									InstanceType: aws.String(""),
									SpotPrice:    aws.String("1"),
								},
							},
						},
						dspherr: nil,
					},
				},
			},
		},
		{
			name: "Test multiple instances returned",
			want: []string{"m3.large", "m3.xlarge"},
			tregion: &region{
				instances: makeInstances(),
				conf:      &Config{},
				services: connections{
					ec2: mockEC2{
						dspho: &ec2.DescribeSpotPriceHistoryOutput{
							SpotPriceHistory: []*ec2.SpotPrice{
								{
									InstanceType: aws.String("m3.large"),
									SpotPrice:    aws.String("1"),
								},
								{
									InstanceType: aws.String("m3.xlarge"),
									SpotPrice:    aws.String("2"),
								},
							},
						},
						dspherr: nil,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.tregion
			instanceTypes, _ := r.requestSpotInstanceTypes()
			if !reflect.DeepEqual(tt.want, instanceTypes) {
				t.Errorf("region.requestSpotInstanceTypes() = %v, want %v", instanceTypes, tt.want)
			}
		})
	}
}
