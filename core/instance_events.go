// Copyright (c) 2016-2019 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"encoding/json"
	"errors"

	"github.com/aws/aws-lambda-go/events"
)

const (
	InstanceStateChangeNotificationMessage = "EC2 Instance State-change Notification"
	InstanceStateChangeNotificationCode    = "ISC"

	SpotInstanceInterruptionWarningMessage = "EC2 Spot Instance Interruption Warning"
	SpotInstanceInterruptionWarningCode    = "SII"

	InstanceRebalanceRecommendationMessage = "EC2 Instance Rebalance Recommendation"
	InstanceRebalanceRecommendationCode    = "IRR"

	AWSAPICallCloudTrailMessage = "AWS API Call via CloudTrail"
	AWSAPICallCloudTrailCode    = "ACC"

	ScheduledEventMessage = "Scheduled Event"
	ScheduledEventCode    = "SCE"
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
	var eventTypeCode string = ""
	var instanceID *string = nil
	var instanceState *string = nil
	var result error = nil

	if err := json.Unmarshal(event.Detail, &detailData); err != nil {
		logger.Println(err.Error())
		return "", nil, nil, err
	}
	eventType := event.DetailType

	if eventType == InstanceStateChangeNotificationMessage &&
		detailData.InstanceID != nil &&
		detailData.State != nil {
		// Amazon EC2 State Change Events
		eventTypeCode = InstanceStateChangeNotificationCode
		instanceID = detailData.InstanceID
		instanceState = detailData.State
	} else if eventType == SpotInstanceInterruptionWarningMessage &&
		detailData.InstanceAction != nil &&
		*detailData.InstanceAction != "" {
		// Amazon EC2 Spot Instance Interruption Events
		eventTypeCode = SpotInstanceInterruptionWarningCode
		instanceID = detailData.InstanceID
	} else if eventType == InstanceRebalanceRecommendationMessage &&
		detailData.InstanceID != nil &&
		*detailData.InstanceID != "" {
		// Amazon EC2 Instance Rebalance Recommendation Events
		eventTypeCode = InstanceRebalanceRecommendationCode
		instanceID = detailData.InstanceID
	} else if eventType == AWSAPICallCloudTrailMessage {
		// Events Delivered Via CloudTrail
		eventTypeCode = AWSAPICallCloudTrailCode
	} else if eventType == ScheduledEventMessage {
		// Amazon CloudWatch Events Scheduled Events
		eventTypeCode = ScheduledEventCode
	} else {
		logger.Println("This code shouldn't be reachable")
		result = errors.New("this code shoudn't be reached")
	}
	return eventTypeCode, instanceID, instanceState, result
}
