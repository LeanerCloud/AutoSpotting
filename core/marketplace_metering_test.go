package autospotting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetAccountID(t *testing.T) {

	tests := []struct {
		name        string
		snsTopicARN string
		sqsQueueURL string
		expectedID  string
		expectError bool
	}{
		{"Valid SNS Topic ARN", "arn:aws:sns:us-east-1:123456789012:MyTopic", "", "123456789012", false},
		{"Invalid SNS Topic ARN", "arn:aws:sns:us-east-1", "", "", true},
		{"Valid SQS Queue URL", "", "https://sqs.us-east-1.amazonaws.com/123456789012/MyQueue", "123456789012", false},
		{"Invalid SQS Queue URL", "", "https://sqs.us-east-1.amazonaws.com", "", true},
		{"Neither Set", "", "", "", true},
		{"Both Set, SNS Valid", "arn:aws:sns:us-east-1:123456789012:MyTopic", "https://sqs.us-east-1.amazonaws.com/123456789012/MyQueue", "123456789012", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			id, err := getAccountID(tc.snsTopicARN, tc.sqsQueueURL)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedID, *id)
			}
		})
	}
}
