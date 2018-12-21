package autospotting

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/service/autoscaling"
)

func TestNewSpotTermination(t *testing.T) {

	region := "foo"
	spotTermination := NewSpotTermination(region)

	if spotTermination.asSvc == nil {
		t.Errorf("Unable to connect to region %s", region)
	}
}

func TestGetInstanceIDDueForTermination(t *testing.T) {

	expectedInstanceID := "i-123456"
	dummyInstanceData := instanceData{
		InstanceID:     expectedInstanceID,
		InstanceAction: "terminate",
	}

	tests := []struct {
		name            string
		cloudWatchEvent events.CloudWatchEvent
		expected        *string
		expectedError   error
	}{
		{
			name: "Invalid Detail in CloudWatch event",
			cloudWatchEvent: events.CloudWatchEvent{
				Detail: []byte(""),
			},
		},
		{
			name: "Detail in event is empty",
			cloudWatchEvent: events.CloudWatchEvent{
				Detail: []byte("{}"),
			},
			expected:      nil,
			expectedError: nil,
		},
		{
			name: "Detail has spot termination event data",
			cloudWatchEvent: events.CloudWatchEvent{
				Detail: func() json.RawMessage {
					data, _ := json.Marshal(dummyInstanceData)
					return data
				}(),
			},
			expected:      &expectedInstanceID,
			expectedError: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			instanceID, _ := GetInstanceIDDueForTermination(tc.cloudWatchEvent)

			if tc.expected == nil && instanceID != nil {
				t.Errorf("Expected nil instanceID, actual: %s", *instanceID)
			}

			if tc.expected != nil && *tc.expected != *instanceID {
				t.Errorf("InstanceID expected: %v\nactual: %v", tc.expected, instanceID)
			}
		})
	}
}

func TestDetachInstance(t *testing.T) {

	asgName := "dummyASGName"
	instanceID := "dummyInstanceID"

	tests := []struct {
		name            string
		spotTermination *SpotTermination
		expectedError   error
	}{
		{
			name:            "When AutoScaling service is nil",
			spotTermination: &SpotTermination{},
			expectedError:   errors.New("AutoScaling service not defined. Please use NewSpotTermination()"),
		},
		{
			name: "When AutoScaling service returns error",
			spotTermination: &SpotTermination{
				asSvc: mockASG{dasierr: errors.New("")},
			},
			expectedError: errors.New(""),
		},
		{
			name: "When AutoScaling service returns asgName",
			spotTermination: &SpotTermination{
				asSvc: mockASG{dasio: &autoscaling.DescribeAutoScalingInstancesOutput{
					AutoScalingInstances: []*autoscaling.InstanceDetails{
						{
							AutoScalingGroupName: &asgName,
						},
					},
				}},
			},
			expectedError: nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			err := tc.spotTermination.DetachInstance(&instanceID)

			if err != nil && err.Error() != tc.expectedError.Error() {
				t.Errorf("Error in DetachInstance: expected %s actual %s", tc.expectedError.Error(), err.Error())
			}

		})
	}
}
