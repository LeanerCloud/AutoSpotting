package autospotting

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

func Test_getRegions(t *testing.T) {

	tests := []struct {
		name    string
		ec2conn mockEC2
		want    []string
		wantErr error
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
		wantErr: nil,
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
			wantErr: fmt.Errorf("fooErr"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			got, err := getRegions(tt.ec2conn)
			CheckErrors(t, err, tt.wantErr)

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getRegions() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_spotEnabledIsAddedByDefault(t *testing.T) {

	tests := []struct {
		name   string
		config Config
		want   Tag
	}{{
		name:   "Default No ASG Tags",
		config: Config{},
		want:   Tag{Key: "spot-enabled", Value: "true"},
	},
		{
			name: "Specified ASG Tags",
			config: Config{
				FilterByTags: []Tag{
					{Key: "environment", Value: "dev"},
				},
			},
			want: Tag{
				Key:   "environment",
				Value: "dev",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			addDefaultFilter(&tt.config)

			if len(tt.config.FilterByTags) != 1 {
				t.Errorf("addDefaultFilter() = %v, want %v", tt.config.FilterByTags, tt.want)
			}

			if tt.config.FilterByTags[0].Key != tt.want.Key || tt.config.FilterByTags[0].Value != tt.want.Value {
				t.Errorf("addDefaultFilter() = %v, want %v", tt.config.FilterByTags, tt.want)
			}
		})
	}
}
