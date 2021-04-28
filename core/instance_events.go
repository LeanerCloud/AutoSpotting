// Copyright (c) 2016-2019 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"encoding/json"
	"errors"

	"github.com/aws/aws-lambda-go/events"
)

const (
	// InstanceStateChangeNotificationMessage store detail-type of the CloudWatch Event for
	// the Amazon EC2 State Change Events
	InstanceStateChangeNotificationMessage = "EC2 Instance State-change Notification"

	// InstanceStateChangeNotificationCode store the 3 letter code used to idenify
	// the Amazon EC2 State Change Events
	InstanceStateChangeNotificationCode = "ISC"

	// SpotInstanceInterruptionWarningMessage store detail-type of the CloudWatch Event for
	// Amazon EC2 Spot Instance Interruption Events
	SpotInstanceInterruptionWarningMessage = "EC2 Spot Instance Interruption Warning"

	// SpotInstanceInterruptionWarningCode store the 3 letter code used to idenify
	// Amazon EC2 Spot Instance Interruption Events
	SpotInstanceInterruptionWarningCode = "SII"

	// InstanceRebalanceRecommendationMessage store detail-type of the CloudWatch Event for
	// Amazon EC2 Instance Rebalance Recommendation Events
	InstanceRebalanceRecommendationMessage = "EC2 Instance Rebalance Recommendation"

	// InstanceRebalanceRecommendationCode store the 3 letter code used to idenify
	// Amazon EC2 Instance Rebalance Recommendation Events
	InstanceRebalanceRecommendationCode = "IRR"

	// AWSAPICallCloudTrailMessage store detail-type of the CloudWatch Event for
	// Events Delivered Via CloudTrail
	AWSAPICallCloudTrailMessage = "AWS API Call via CloudTrail"

	// AWSAPICallCloudTrailCode store the 3 letter code used to idenify
	// Events Delivered Via CloudTrail
	AWSAPICallCloudTrailCode = "ACC"

	// ScheduledEventMessage store detail-type of the CloudWatch Event for
	// Amazon CloudWatch Events Scheduled Events
	ScheduledEventMessage = "Scheduled Event"

	// ScheduledEventCode store the 3 letter code used to idenify
	// Amazon CloudWatch Events Scheduled Events
	ScheduledEventCode = "SCE"
)

//InstanceData represents JSON structure of the Detail property of CloudWatch event when a spot instance is terminated
//Reference = https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/spot-interruptions.html#spot-instance-termination-notices
type instanceData struct {
	InstanceID     *string `json:"instance-id"`
	InstanceAction *string `json:"instance-action"`
	State          *string `json:"state"`
}

// returns the InstanceID, State or an error
func parseEventData(event events.CloudWatchEvent) (string, *string, *string, error) {
	var detailData instanceData
	var eventTypeCode string
	var instanceID *string
	var instanceState *string
	var result error

	if err := json.Unmarshal(event.Detail, &detailData); err != nil {
		logger.Println(err.Error())
		return "", nil, nil, err
	}
	eventType := event.DetailType

	// Amazon EC2 State Change Events
	if eventType == InstanceStateChangeNotificationMessage &&
		detailData.InstanceID != nil &&
		detailData.State != nil {
		eventTypeCode = InstanceStateChangeNotificationCode
		instanceID = detailData.InstanceID
		instanceState = detailData.State
	}

	// Amazon EC2 Spot Instance Interruption Events
	if eventType == SpotInstanceInterruptionWarningMessage &&
		detailData.InstanceAction != nil &&
		*detailData.InstanceAction != "" {
		eventTypeCode = SpotInstanceInterruptionWarningCode
		instanceID = detailData.InstanceID
	}

	// Amazon EC2 Instance Rebalance Recommendation Events
	if eventType == InstanceRebalanceRecommendationMessage &&
		detailData.InstanceID != nil &&
		*detailData.InstanceID != "" {
		eventTypeCode = InstanceRebalanceRecommendationCode
		instanceID = detailData.InstanceID
	}

	// Events Delivered Via CloudTrail
	if eventType == AWSAPICallCloudTrailMessage {
		eventTypeCode = AWSAPICallCloudTrailCode
	}

	// Amazon CloudWatch Events Scheduled Events
	if eventType == ScheduledEventMessage {
		eventTypeCode = ScheduledEventCode
	}

	// This code shouldn't be reachable
	if len(eventTypeCode) == 0 {
		logger.Println("This code shouldn't be reachable")
		result = errors.New("this code shoudn't be reached")
	}

	return eventTypeCode, instanceID, instanceState, result
}
