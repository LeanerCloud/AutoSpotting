package autospotting

import (
	"encoding/json"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/service/autoscaling"
)

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
			instanceID, err := GetInstanceIDDueForTermination(tc.cloudWatchEvent)

			if err != nil {
				t.Errorf("Found error: %s", err.Error())
			}

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

	region := "us-east-1"
	asgName := "dummyASGName"
	instanceID := "dummyInstanceID"
	spotTermination := &SpotTermination{
		asSvc: mockASG{dasio: &autoscaling.DescribeAutoScalingInstancesOutput{
			AutoScalingInstances: []*autoscaling.InstanceDetails{
				{
					AutoScalingGroupName: &asgName,
				},
			},
		}},
	}

	if err := spotTermination.DetachInstance(&instanceID, region); err != nil {
		t.Errorf("Error in DetachInstance: %s", err.Error())
	}
}
