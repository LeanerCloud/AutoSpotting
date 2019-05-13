package autospotting

import (
	"encoding/json"
	"errors"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
)

const (
	// DefaultTerminationNotificationAction is the default value for the termination notification
	// action configuration option
	DefaultTerminationNotificationAction = AutoTerminationNotificationAction
)

//SpotTermination is used to detach an instance, used when a spot instance is due for termination
type SpotTermination struct {
	asSvc  autoscalingiface.AutoScalingAPI
	ec2Svc ec2iface.EC2API
}

//InstanceData represents JSON structure of the Detail property of CloudWatch event when a spot instance is terminated
//Reference = https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/spot-interruptions.html#spot-instance-termination-notices
type instanceData struct {
	InstanceID     string `json:"instance-id"`
	InstanceAction string `json:"instance-action"`
}

//NewSpotTermination is a constructor for creating an instance of spotTermination to call DetachInstance
func NewSpotTermination(region string) SpotTermination {

	logger.Println("Connection to region ", region)

	session := session.Must(
		session.NewSession(&aws.Config{Region: aws.String(region)}))

	return SpotTermination{

		asSvc:  autoscaling.New(session),
		ec2Svc: ec2.New(session),
	}
}

//GetInstanceIDDueForTermination checks if the given CloudWatch event data is triggered from a spot termination
//If it is a termination event for a spot instance, it returns the instance id present in the event data
func GetInstanceIDDueForTermination(event events.CloudWatchEvent) (*string, error) {

	var detailData instanceData
	if err := json.Unmarshal(event.Detail, &detailData); err != nil {
		logger.Println(err.Error())
		return nil, err
	}

	if detailData.InstanceAction != "" {
		return &detailData.InstanceID, nil
	}

	return nil, nil
}

//DetachInstance detaches the instance from autoscaling group without decrementing the desired capacity
//This makes sure that the autoscaling group spawns a new instance as soon as this instance is detached
func (s *SpotTermination) detachInstance(instanceID *string, asgName string) error {

	logger.Println(asgName,
		"Detaching instance:",
		*instanceID)

	detachParams := autoscaling.DetachInstancesInput{
		AutoScalingGroupName: aws.String(asgName),
		InstanceIds: []*string{
			instanceID,
		},
		ShouldDecrementDesiredCapacity: aws.Bool(false),
	}
	if _, detachErr := s.asSvc.DetachInstances(&detachParams); detachErr != nil {
		logger.Println(detachErr.Error())
		return detachErr
	}

	logger.Printf("Detached instance %s successfully", *instanceID)

	s.deleteTagInstanceLaunchedForAsg(instanceID)

	return nil
}

//TerminateInstance terminate the instance from autoscaling group without decrementing the desired capacity
//This makes sure that any LifeCycle Hook configured is triggered and the autoscaling group spawns a new instance
// as soon as this instance begin terminating.
func (s *SpotTermination) terminateInstance(instanceID *string, asgName string) error {

	logger.Println(asgName,
		"Terminating instance:",
		*instanceID)
	// terminate the spot instance
	terminateParams := autoscaling.TerminateInstanceInAutoScalingGroupInput{
		InstanceId:                     instanceID,
		ShouldDecrementDesiredCapacity: aws.Bool(false),
	}

	if _, err := s.asSvc.TerminateInstanceInAutoScalingGroup(&terminateParams); err != nil {
		logger.Println(err.Error())
		return err
	}
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

// ExecuteAction execute the proper termination action (terminate|detach) based on the value of
// terminationNotificationAction and the presence of a LifecycleHook on ASG.
func (s *SpotTermination) ExecuteAction(instanceID *string, terminationNotificationAction string) error {
	if s.asSvc == nil {
		return errors.New("AutoScaling service not defined. Please use NewSpotTermination()")
	}

	asgName, err := s.getAsgName(instanceID)

	if err != nil {
		logger.Printf("Failed get ASG name for %s with err: %s\n", *instanceID, err.Error())
		return err
	}

	switch terminationNotificationAction {
	case "detach":
		s.detachInstance(instanceID, asgName)
	case "terminate":
		s.terminateInstance(instanceID, asgName)
	default:
		if s.asgHasTerminationLifecycleHook(&asgName) {
			s.terminateInstance(instanceID, asgName)
		} else {
			s.detachInstance(instanceID, asgName)
		}
	}

	return nil
}

func (s *SpotTermination) deleteTagInstanceLaunchedForAsg(instanceID *string) error {
	ec2Params := ec2.DeleteTagsInput{
		Resources: []*string{
			aws.String(*instanceID),
		},
		Tags: []*ec2.Tag{
			{
				Key: aws.String("launched-for-asg"),
			},
		},
	}
	_, err := s.ec2Svc.DeleteTags(&ec2Params)

	if err != nil {
		logger.Printf("Failed to delete Tag 'launched-for-asg' from spot instance %s with err: %s\n", *instanceID, err.Error())
		return err
	}

	logger.Printf("Tag 'launched-for-asg' deleted from spot instance %s", *instanceID)

	return nil
}

func (s *SpotTermination) asgHasTerminationLifecycleHook(autoScalingGroupName *string) bool {
	asParams := autoscaling.DescribeLifecycleHooksInput{
		AutoScalingGroupName: autoScalingGroupName,
	}

	result, err := s.asSvc.DescribeLifecycleHooks(&asParams)

	if err != nil {
		logger.Println(err.Error())
		return false
	}

	var hasHook = false
	for _, lfh := range result.LifecycleHooks {
		if *lfh.LifecycleTransition == "autoscaling:EC2_INSTANCE_TERMINATING" {
			hasHook = true
			logger.Println("Found Hook", *lfh.LifecycleHookName)
			break
		}
	}

	return hasHook
}
