// Copyright (c) 2016-2019 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
)

func TestParseEventData(t *testing.T) {

	expectedInstanceID := "i-123456"
	expectedNotMatchedError := errors.New("this code shoudn't be reached")
	instanceAction := aws.String(TerminateTerminationNotificationAction)
	instanceState := "running"

	tests := []struct {
		name                  string
		eventType             string
		cloudWatchEvent       events.CloudWatchEvent
		expectedInstanceID    *string
		expectedInstanceState *string
		expectedError         error
	}{
		{
			name: "Invalid Detail in CloudWatch event",
			cloudWatchEvent: events.CloudWatchEvent{
				Detail: []byte(""),
			},
		},
		{
			name: "DetailType is not matched",
			cloudWatchEvent: events.CloudWatchEvent{
				DetailType: "not matching",
				Detail:     []byte("{}"),
			},
			expectedError: expectedNotMatchedError,
		},
		{
			name: "Detail is Amazon EC2 State Change Events with no instanceID and no State",
			cloudWatchEvent: events.CloudWatchEvent{
				DetailType: InstanceStateChangeNotificationMessage,
				Detail: func() json.RawMessage {
					data, _ := json.Marshal(instanceData{})
					return data
				}(),
			},
			expectedInstanceID:    nil,
			expectedInstanceState: nil,
			expectedError:         expectedNotMatchedError,
		},
		{
			name: "Detail is Amazon EC2 State Change Events with instanceID and instanceState",
			cloudWatchEvent: events.CloudWatchEvent{
				DetailType: InstanceStateChangeNotificationMessage,
				Detail: func() json.RawMessage {
					data, _ := json.Marshal(instanceData{
						InstanceID: aws.String(expectedInstanceID),
						State:      &instanceState,
					})
					return data
				}(),
			},
			expectedInstanceID:    &expectedInstanceID,
			expectedInstanceState: aws.String("running"),
			expectedError:         nil,
		},
		{
			name: "Detail is Amazon EC2 Spot Instance Interruption Events with no InstanceAction",
			cloudWatchEvent: events.CloudWatchEvent{
				DetailType: SpotInstanceInterruptionWarningMessage,
				Detail: func() json.RawMessage {
					data, _ := json.Marshal(instanceData{
						InstanceID: aws.String(expectedInstanceID),
					})
					return data
				}(),
			},
			expectedInstanceID:    nil,
			expectedInstanceState: nil,
			expectedError:         expectedNotMatchedError,
		},
		{
			name: "Detail is Amazon EC2 Spot Instance Interruption Events with InstanceID and InstanceAction",
			cloudWatchEvent: events.CloudWatchEvent{
				DetailType: SpotInstanceInterruptionWarningMessage,
				Detail: func() json.RawMessage {
					data, _ := json.Marshal(instanceData{
						InstanceID:     aws.String(expectedInstanceID),
						InstanceAction: instanceAction,
					})
					return data
				}(),
			},
			expectedInstanceID:    &expectedInstanceID,
			expectedInstanceState: nil,
			expectedError:         nil,
		},
		{
			name: "Detail is Amazon EC2 Instance Rebalance Recommendation Events with no InstanceID",
			cloudWatchEvent: events.CloudWatchEvent{
				DetailType: InstanceRebalanceRecommendationMessage,
				Detail: func() json.RawMessage {
					data, _ := json.Marshal(instanceData{})
					return data
				}(),
			},
			expectedInstanceID:    nil,
			expectedInstanceState: nil,
			expectedError:         expectedNotMatchedError,
		},
		{
			name: "Detail is Amazon EC2 Spot Instance Interruption Events with InstanceID",
			cloudWatchEvent: events.CloudWatchEvent{
				DetailType: InstanceRebalanceRecommendationMessage,
				Detail: func() json.RawMessage {
					data, _ := json.Marshal(instanceData{
						InstanceID: aws.String(expectedInstanceID),
					})
					return data
				}(),
			},
			expectedInstanceID:    &expectedInstanceID,
			expectedInstanceState: nil,
			expectedError:         nil,
		},
		{
			name: "Detail is Events Delivered Via CloudTrail",
			cloudWatchEvent: events.CloudWatchEvent{
				DetailType: AWSAPICallCloudTrailMessage,
				Detail: func() json.RawMessage {
					data, _ := json.Marshal(instanceData{})
					return data
				}(),
			},
			expectedInstanceID:    nil,
			expectedInstanceState: nil,
			expectedError:         nil,
		},
		{
			name: "Detail is Amazon CloudWatch Events Scheduled Events",
			cloudWatchEvent: events.CloudWatchEvent{
				DetailType: ScheduledEventMessage,
				Detail: func() json.RawMessage {
					data, _ := json.Marshal(instanceData{})
					return data
				}(),
			},
			expectedInstanceID:    nil,
			expectedInstanceState: nil,
			expectedError:         nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			eventTypeCode, instanceID, instanceState, _ := parseEventData(tc.cloudWatchEvent)

			if eventTypeCode == "" && (instanceID != nil ||
				instanceState != nil) {
				t.Errorf("Expected nil instanceID and InstanceState, actual: %s, %s", *instanceID, *instanceState)
			}

			if eventTypeCode == InstanceStateChangeNotificationCode && (*tc.expectedInstanceID != *instanceID || *tc.expectedInstanceState != *instanceState) {
				t.Errorf("InstanceID expected: %v\nactual: %v", tc.expectedInstanceID, instanceID)
				t.Errorf("InstanceState expected: %v\nactual: %v", tc.expectedInstanceState, instanceID)
			}
			if (eventTypeCode == SpotInstanceInterruptionWarningCode ||
				eventTypeCode == InstanceRebalanceRecommendationCode) && *tc.expectedInstanceID != *instanceID {
				t.Errorf("InstanceID expected: %v\nactual: %v", tc.expectedInstanceID, instanceID)
			}
			if (eventTypeCode == AWSAPICallCloudTrailCode ||
				eventTypeCode == ScheduledEventCode) && tc.expectedInstanceID != instanceID {
				t.Errorf("InstanceID expected: %v\nactual: %v", tc.expectedInstanceID, instanceID)
			}
		})
	}
}
