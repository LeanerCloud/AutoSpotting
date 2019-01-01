package autospotting

import (
	"encoding/json"
	"errors"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
)

//SpotTermination is used to detach an instance, used when a spot instance is due for termination
type SpotTermination struct {
	asSvc autoscalingiface.AutoScalingAPI
}

//InstanceData represents JSON structure of the Detail property of CloudWatch event when a spot instance is terminated
//Reference = https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/spot-interruptions.html#spot-instance-termination-notices
type instanceData struct {
	InstanceID     string `json:"instance-id"`
	InstanceAction string `json:"instance-action"`
}

//NewSpotTermination is a constructor for creating an instance of spotTermination to call DetachInstance
func NewSpotTermination(region string) SpotTermination {

	log.Println("Connection to region ", region)

	session := session.Must(
		session.NewSession(&aws.Config{Region: aws.String(region)}))

	return SpotTermination{

		asSvc: autoscaling.New(session),
	}
}

//GetInstanceIDDueForTermination checks if the given CloudWatch event data is triggered from a spot termination
//If it is a termination event for a spot instance, it returns the instance id present in the event data
func GetInstanceIDDueForTermination(event events.CloudWatchEvent) (*string, error) {

	var detailData instanceData
	if err := json.Unmarshal(event.Detail, &detailData); err != nil {
		log.Println(err.Error())
		return nil, err
	}

	if detailData.InstanceAction != "" {
		return &detailData.InstanceID, nil
	}

	return nil, nil
}

//DetachInstance detaches the instance from autoscaling group without decrementing the desired capacity
//This makes sure that the autoscaling group spawns a new instance as soon as this instance is detached
func (s *SpotTermination) DetachInstance(instanceID *string) error {

	if s.asSvc == nil {
		return errors.New("AutoScaling service not defined. Please use NewSpotTermination()")
	}

	asgName, err := s.getAsgName(instanceID)

	if err != nil {
		log.Printf("Failed to detach instance %s with err: %s\n", err.Error(), *instanceID)
		return err
	}

	detachParams := autoscaling.DetachInstancesInput{
		AutoScalingGroupName: aws.String(asgName),
		InstanceIds: []*string{
			instanceID,
		},
		ShouldDecrementDesiredCapacity: aws.Bool(false),
	}
	if _, detachErr := s.asSvc.DetachInstances(&detachParams); detachErr != nil {
		log.Println(detachErr.Error())
		return detachErr
	}

	log.Printf("Detached instance %s successfully", *instanceID)
	return nil
}

func (s *SpotTermination) getAsgName(instanceID *string) (string, error) {

	asParams := autoscaling.DescribeAutoScalingInstancesInput{
		InstanceIds: []*string{instanceID},
	}

	result, err := s.asSvc.DescribeAutoScalingInstances(&asParams)

	var asgName = ""
	if err == nil {
		asgName = *result.AutoScalingInstances[0].AutoScalingGroupName
	}

	return asgName, err
}
