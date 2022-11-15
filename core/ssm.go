package autospotting

import (
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
)

type parameterStore struct {
	svc ssmiface.SSMAPI
}

func (ps *parameterStore) putSSMParameter(key, value string) {

	_, err := ps.svc.PutParameter(&ssm.PutParameterInput{
		Name:      aws.String(key),
		Overwrite: aws.Bool(true),
		Type:      aws.String("String"),
		Value:     aws.String(value),
	})
	if err != nil {
		log.Printf("Error persisting marketplace metering status(%s) to SSM: %s", value, err.Error())
	}
}

func (ps *parameterStore) getSSMParameter(key string) (*string, *time.Time, error) {

	res, err := ps.svc.GetParameter(&ssm.GetParameterInput{
		Name: aws.String(key),
	})

	if err != nil {
		debug.Printf("Error reading SSM Parameter '%s' from SSM, error: %s", key, err.Error())
		return nil, nil, err
	}

	value := res.Parameter.Value
	timestamp := res.Parameter.LastModifiedDate
	debug.Printf("Retrieved SSM parameter '%s' = '%s'", key, *value)
	return value, timestamp, nil
}

func (ps *parameterStore) getSSMParameterHistory(key string) ([]*ssm.ParameterHistory, error) {
	input := &ssm.GetParameterHistoryInput{
		Name:       aws.String(key),
		MaxResults: aws.Int64(50),
	}
	var history []*ssm.ParameterHistory

	err := ps.svc.GetParameterHistoryPages(input,
		func(page *ssm.GetParameterHistoryOutput, lastPage bool) bool {
			for _, value := range page.Parameters {
				history = append(history, value)
			}
			return true
		})

	if err != nil {
		debug.Printf("Error reading parameter history for SSM parameter '%s'", key)
		if _, ok := err.(*ssm.ParameterNotFound); ok {
			debug.Printf("Parameter not found: %s: %v", key, err.Error())
			return nil, err
		}
		debug.Printf("Encountered error: %v", err.Error())
		return nil, err
	}
	return history, nil
}
