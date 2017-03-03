package autospotting

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
)

func Test_waitForAndTagSpotInstance(t *testing.T) {
	tests := []struct {
		name string
		req  spotInstanceRequest
		er   error
	}{
		{
			name: "with WaitUntilSpotInstanceRequestFulfilled error",
			req: spotInstanceRequest{
				SpotInstanceRequest: &ec2.SpotInstanceRequest{
					SpotInstanceRequestId: aws.String(""),
				},
				region: &region{
					services: connections{
						ec2: mockEC2{
							wusirferr: errors.New(""),
						},
					},
				},
				asg: &autoScalingGroup{
					name: ""},
			},
			er: errors.New(""),
		},
		{
			name: "without WaitUntilSpotInstanceRequestFulfilled error",
			req: spotInstanceRequest{
				SpotInstanceRequest: &ec2.SpotInstanceRequest{
					SpotInstanceRequestId: aws.String(""),
				},
				region: &region{
					services: connections{
						ec2: mockEC2{
							dsiro: &ec2.DescribeSpotInstanceRequestsOutput{
								SpotInstanceRequests: []*ec2.SpotInstanceRequest{
									{InstanceId: aws.String("")},
								},
							},
							dio: &ec2.DescribeInstancesOutput{
								Reservations: []*ec2.Reservation{{}},
							},
						},
					},
				},
				asg: &autoScalingGroup{
					Group: &autoscaling.Group{
						Tags: []*autoscaling.TagDescription{},
					},
					name: "",
				},
			},
			er: errors.New(""),
		},
		{
			name: "with DescribeSpotInstanceRequestsOutput error",
			req: spotInstanceRequest{
				SpotInstanceRequest: &ec2.SpotInstanceRequest{
					SpotInstanceRequestId: aws.String(""),
				},
				region: &region{
					services: connections{
						ec2: mockEC2{
							dsiro: &ec2.DescribeSpotInstanceRequestsOutput{
								SpotInstanceRequests: []*ec2.SpotInstanceRequest{
									{InstanceId: aws.String("")},
								},
							},
							dsirerr: errors.New(""),
							dio: &ec2.DescribeInstancesOutput{
								Reservations: []*ec2.Reservation{{}},
							},
						},
					},
				},
				asg: &autoScalingGroup{
					Group: &autoscaling.Group{
						Tags: []*autoscaling.TagDescription{},
					},
					name: "",
				},
			},
			er: errors.New(""),
		},
	}

	for _, tc := range tests {
		tc.req.waitForAndTagSpotInstance()
	}
}

func Test_tag(t *testing.T) {
	tests := []struct {
		name string
		tag  string
		req  spotInstanceRequest
		er   error
	}{
		{
			name: "with error",
			tag:  "tag",
			req: spotInstanceRequest{
				SpotInstanceRequest: &ec2.SpotInstanceRequest{
					SpotInstanceRequestId: aws.String(""),
				},
				region: &region{
					services: connections{
						ec2: mockEC2{
							cterr: errors.New(""),
						},
					},
				},
			},
			er: errors.New(""),
		},
		{
			name: "without error",
			tag:  "tag",
			req: spotInstanceRequest{
				SpotInstanceRequest: &ec2.SpotInstanceRequest{
					SpotInstanceRequestId: aws.String(""),
				},
				region: &region{
					services: connections{
						ec2: mockEC2{
							cterr: nil,
						},
					},
				},
			},
			er: nil,
		},
	}

	for _, tc := range tests {
		er := tc.req.tag(tc.tag)
		if er != nil && er.Error() != tc.er.Error() {
			t.Errorf("error actual: %s, expected: %s", er.Error(), tc.er.Error())
		}
	}
}
