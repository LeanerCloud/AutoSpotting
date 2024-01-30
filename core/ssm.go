package autospotting

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmTypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/aws/aws-sdk-go/aws"
)

type parameterStore struct {
	svc SSMAPI
}

func (ps *parameterStore) putSSMParameter(key, value string) {

	_, err := ps.svc.PutParameter(
		context.TODO(),
		&ssm.PutParameterInput{
			Name:      aws.String(key),
			Overwrite: aws.Bool(true),
			Type:      "String",
			Value:     aws.String(value),
		})
	if err != nil {
		log.Printf("Error persisting marketplace metering status(%s) to SSM: %s", value, err.Error())
	}
}

func (ps *parameterStore) getSSMParameter(key string) (*string, *time.Time, error) {

	res, err := ps.svc.GetParameter(
		context.TODO(),
		&ssm.GetParameterInput{
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

func (ps *parameterStore) getSSMParameterHistory(key string) ([]*ssmTypes.ParameterHistory, error) {
	input := &ssm.GetParameterHistoryInput{
		Name:       aws.String(key),
		MaxResults: aws.Int32(50),
	}

	var history []*ssmTypes.ParameterHistory
	paginator := ssm.NewGetParameterHistoryPaginator(ps.svc, input)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			log.Printf("Error reading parameter history for SSM parameter '%s': %v", key, err)
			var notFoundErr *ssmTypes.ParameterNotFound
			if errors.As(err, &notFoundErr) {
				log.Printf("Parameter not found: %s: %v", key, err)
				return nil, err
			}
			log.Printf("Encountered error: %v", err)
			return nil, err
		}

		for _, value := range page.Parameters {
			history = append(history, &value)
		}
	}

	return history, nil
}
