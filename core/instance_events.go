// Copyright (c) 2016-2019 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"encoding/json"
	"errors"

	"github.com/aws/aws-lambda-go/events"
)

// returns the InstanceID, State or an error
func parseEventData(event events.CloudWatchEvent) (string, *string, *string, error) {
	var detailData instanceData
	if err := json.Unmarshal(event.Detail, &detailData); err != nil {
		logger.Println(err.Error())
		return "", nil, nil, err
	}
	eventType := event.DetailType

	// Amazon EC2 State Change Events
	if eventType == "EC2 Instance State-change Notification" &&
		detailData.InstanceID != nil &&
		detailData.State != nil {
		return "IS", detailData.InstanceID, detailData.State, nil
	}

	// Amazon EC2 Spot Instance Interruption Events
	if eventType == "EC2 Spot Instance Interruption Warning" &&
		detailData.InstanceAction != nil &&
		*detailData.InstanceAction != "" {
		return "II", detailData.InstanceID, nil, nil
	}

	// Amazon EC2 Instance Rebalance Recommendation Events
	if eventType == "EC2 Instance Rebalance Recommendation" &&
		detailData.InstanceID != nil &&
		*detailData.InstanceID != "" {
		return "IR", detailData.InstanceID, nil, nil
	}

	// Events Delivered Via CloudTrail
	if eventType == "AWS API Call via CloudTrail" {
		return "CT", nil, nil, nil
	}

	// Amazon CloudWatch Events Scheduled Events
	if eventType == "Scheduled Event" {
		return "SC", nil, nil, nil
	}

	logger.Println("This code shouldn't be reachable")
	return "", nil, nil, errors.New("this code shoudn't be reached")
}
