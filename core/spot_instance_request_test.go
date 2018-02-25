package autospotting

import (
	"errors"
	"fmt"
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

func Test_isSpotRequestOld(t *testing.T) {

	tests := []struct {
		name          string
		secondsToTest time.Duration
		expected      bool
	}{
		{
			name:          "OnFiveMinuteThreshold",
			secondsToTest: 298 * time.Second,
			expected:      false,
		},
		{
			name:          "TenMinutesOld",
			secondsToTest: 600 * time.Second,
			expected:      true,
		},
		{
			name:          "FiveMinutesOneSecondOld",
			secondsToTest: 301 * time.Second,
			expected:      true,
		},
	}

	for _, test := range tests {
		var testTime time.Time
		testTime = time.Now().Add(-1 * test.secondsToTest)

		fmt.Println(testTime)
		isOlderThan5Minutes := hasRequestBeenOpenForLongerThanXSeconds(&testTime, 300)
		if isOlderThan5Minutes != test.expected {
			if test.expected {
				t.Errorf(test.name + " expected spot request time to be older.")
			} else {
				t.Errorf(test.name + " expected spot request time to be younger.")
			}
		}

	}

}

func Test_processHoldingRequest(t *testing.T) {
	creationTime := time.Now()
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
			name: "with holding request that has a Creation Time, and there is no max time to wait",
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
			name: "with holding request that has a Creation Time, and has not exceeded default max Waiting time",
			req: spotInstanceRequest{
				maxTimeInHolding: 300,
				SpotInstanceRequest: &ec2.SpotInstanceRequest{
					SpotInstanceRequestId: aws.String("aaa"),
					State: aws.String("open"),
					Status: &ec2.SpotInstanceStatus{
						Code: aws.String("capacity-not-available"),
					},
					CreateTime: &creationTime,
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
			name: "with holding request that has a Creation Time, that has exceeded the default max Waiting time",
			req: spotInstanceRequest{
				SpotInstanceRequest: &ec2.SpotInstanceRequest{
					SpotInstanceRequestId: aws.String("aaa"),
					State: aws.String("open"),
					Status: &ec2.SpotInstanceStatus{
						Code: aws.String("capacity-not-available"),
					},
					CreateTime: &creationTime,
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
			cancelled:        true,
			isHoldingRequest: true,
			maxTimeInHolding: int64(2),
			sleepTestFor:     5 * time.Second,
		},
		{
			name: "with a holding request that has a Creation Time, but has not exceeded the given max Waiting time",
			req: spotInstanceRequest{
				SpotInstanceRequest: &ec2.SpotInstanceRequest{
					SpotInstanceRequestId: aws.String("aaa"),
					State: aws.String("open"),
					Status: &ec2.SpotInstanceStatus{
						Code: aws.String("capacity-not-available"),
					},
					CreateTime: &creationTime,
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
			cancelled:        true,
			isHoldingRequest: true,
			maxTimeInHolding: int64(10),
			sleepTestFor:     5 * time.Second,
		},
		{
			name: "not with holding request that has a Creation Time, that has exceeded max Waiting time",
			req: spotInstanceRequest{
				SpotInstanceRequest: &ec2.SpotInstanceRequest{
					SpotInstanceRequestId: aws.String("aaa"),
					State: aws.String("open"),
					Status: &ec2.SpotInstanceStatus{
						Code: aws.String("pending-evaluation"),
					},
					CreateTime: &creationTime,
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
			isHoldingRequest: false,
			maxTimeInHolding: int64(2),
			sleepTestFor:     5 * time.Second,
		},
	}

	for _, tc := range tests {
		if tc.sleepTestFor > 0 {
			time.Sleep(tc.sleepTestFor)
		}

		isHoldingRequest, cancelled := tc.req.processHoldingRequest(tc.maxTimeInHolding)
		if cancelled != tc.cancelled {
			t.Errorf("test cancelled for \"%v\", actual: %v, expected: %v", tc.name, cancelled, tc.cancelled)
		}

		if isHoldingRequest != tc.isHoldingRequest {
			t.Errorf("test isHoldingRequest for \"%v\", actual: %v, expected: %v", tc.name, cancelled, tc.cancelled)
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
