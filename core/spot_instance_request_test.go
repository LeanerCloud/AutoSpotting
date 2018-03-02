package autospotting

import (
	"errors"
	"testing"
	"time"

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
					conf: &Config{},
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
					conf: &Config{},
					services: connections{
						ec2: mockEC2{
							dsiro: &ec2.DescribeSpotInstanceRequestsOutput{
								SpotInstanceRequests: []*ec2.SpotInstanceRequest{
									{InstanceId: aws.String("")},
								},
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
					conf: &Config{},
					services: connections{
						ec2: mockEC2{
							dsiro: &ec2.DescribeSpotInstanceRequestsOutput{
								SpotInstanceRequests: []*ec2.SpotInstanceRequest{
									{InstanceId: aws.String("")},
								},
							},
							dsirerr: errors.New(""),
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

func Test_isHoldingRequest(t *testing.T) {

	statuses := []string{"capacity-not-available",
		"capacity-oversubscribed",
		"price-too-low",
		"not-scheduled-yet",
		"launch-group-constraint",
		"az-group-constraint",
		"placement-group-constraint",
		"constraint-not-fulfillable",
	}

	for _, status := range statuses {
		if !hasHoldingRequestStatus(status) {
			t.Errorf("" + status + " should be a holding request")
		}
	}

	statuses = []string{"pending-evaluation", "bad-parameters", "schedule-expired"}

	for _, status := range statuses {
		if hasHoldingRequestStatus(status) {
			t.Errorf("" + status + " should not be a holding request")
		}
	}
}

func Test_isSpotRequestAHoldingRequest(t *testing.T) {

	tests := []struct {
		name     string
		req      spotInstanceRequest
		expected bool
	}{
		{
			req: spotInstanceRequest{
				SpotInstanceRequest: &ec2.SpotInstanceRequest{
					SpotInstanceRequestId: aws.String("aaa"),
					State: aws.String("open"),
					Status: &ec2.SpotInstanceStatus{
						Code: aws.String("capacity-not-available"),
					},
				},
			},
			expected: true,
			name:     "Is Holding Request With Capacity Not Available",
		},
		{
			req: spotInstanceRequest{
				SpotInstanceRequest: &ec2.SpotInstanceRequest{
					SpotInstanceRequestId: aws.String("aaa"),
					State: aws.String("open"),
				},
			},
			expected: false,
			name:     "Is Holding Request With No Status Information",
		},
	}

	for _, test := range tests {
		if test.req.isHoldingRequest() != test.expected {
			if test.expected {
				t.Errorf(test.name + " should be a holding request")
			} else {
				t.Errorf(test.name + " should not be a holding request")
			}
		}
	}

}

func Test_processHoldingRequest(t *testing.T) {
	tests := []struct {
		name             string
		req              spotInstanceRequest
		er               error
		cancelled        bool
		isHoldingRequest bool
		maxTimeInHolding int64
		sleepTestFor     time.Duration
	}{
		{
			name: "with holding request no Creation Time",
			req: spotInstanceRequest{
				SpotInstanceRequest: &ec2.SpotInstanceRequest{
					SpotInstanceRequestId: aws.String("aaa"),
					State: aws.String("open"),
					Status: &ec2.SpotInstanceStatus{
						Code: aws.String("capacity-not-available"),
					},
				},
				region: &region{
					conf: &Config{},
					services: connections{
						ec2: mockEC2{
							csiro: &ec2.CancelSpotInstanceRequestsOutput{
								CancelledSpotInstanceRequests: []*ec2.CancelledSpotInstanceRequest{
									{SpotInstanceRequestId: aws.String("aaa")},
								},
							},
						},
					},
				},
				asg: &autoScalingGroup{
					name: ""},
			},
			er:               nil,
			cancelled:        false,
			isHoldingRequest: true,
		},
		{
			name: "with holding request that has a Creation Time",
			req: spotInstanceRequest{
				SpotInstanceRequest: &ec2.SpotInstanceRequest{
					SpotInstanceRequestId: aws.String("aaa"),
					State: aws.String("open"),
					Status: &ec2.SpotInstanceStatus{
						Code: aws.String("capacity-not-available"),
					},
				},
				region: &region{
					conf: &Config{},
					services: connections{
						ec2: mockEC2{
							csiro: &ec2.CancelSpotInstanceRequestsOutput{
								CancelledSpotInstanceRequests: []*ec2.CancelledSpotInstanceRequest{
									{SpotInstanceRequestId: aws.String("aaa")},
								},
							},
						},
					},
				},
				asg: &autoScalingGroup{
					name: ""},
			},
			er:               nil,
			cancelled:        false,
			isHoldingRequest: true,
		},
	}

	for _, tc := range tests {
		if tc.sleepTestFor > 0 {
			time.Sleep(tc.sleepTestFor)
		}

		isHoldingRequest := tc.req.isHoldingRequest()

		if isHoldingRequest != tc.isHoldingRequest {
			t.Errorf("test isHoldingRequest for \"%v\", actual: %v, expected: %v", tc.name, isHoldingRequest, tc.isHoldingRequest)
		}

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
					conf: &Config{},
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
					conf: &Config{},
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
