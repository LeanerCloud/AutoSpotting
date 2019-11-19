package autospotting

import (
	"encoding/json"
	"errors"

	"github.com/aws/aws-lambda-go/events"
)

// returns the InstanceID, State or an error
func parseEventData(event events.CloudWatchEvent) (*string, *string, error) {
	var detailData instanceData
	if err := json.Unmarshal(event.Detail, &detailData); err != nil {
		logger.Println(err.Error())
		return nil, nil, err
	}

	if detailData.InstanceID != nil && detailData.State != nil {
		return detailData.InstanceID, detailData.State, nil
	}

	logger.Println("This code shouldn't be reachable")
	return nil, nil, errors.New("this code shoudn't be reached")
}
