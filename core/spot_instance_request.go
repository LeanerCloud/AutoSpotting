package autospotting

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

// Set the default timeout for tagging instances
const (
	defaultTimeout = 60
)

type spotInstanceRequest struct {
	*ec2.SpotInstanceRequest
	region *region
	asg    *autoScalingGroup
}

// This function returns an Instance ID
func (s *spotInstanceRequest) waitForSpotInstance() error {
	logger.Println(s.asg.name, "Waiting for spot instance for spot instance request",
		*s.SpotInstanceRequestId)

	ec2Client := s.region.services.ec2

	params := ec2.DescribeSpotInstanceRequestsInput{
		SpotInstanceRequestIds: []*string{s.SpotInstanceRequestId},
	}

	err := ec2Client.WaitUntilSpotInstanceRequestFulfilled(&params)
	if err != nil {
		logger.Println(s.asg.name, "Error waiting for instance:", err.Error())
		return err
	}
	return nil
}

func (s *spotInstanceRequest) tag(asgName string) error {
	svc := s.region.services.ec2

	_, err := svc.CreateTags(&ec2.CreateTagsInput{
		Resources: []*string{s.SpotInstanceRequestId},
		Tags: []*ec2.Tag{
			{
				Key:   aws.String("launched-for-asg"),
				Value: aws.String(asgName),
			},
		},
	})

	if err != nil {
		// Print the error, cast err to awserr.Error to get the Code and
		// Message from an error.
		logger.Println(asgName,
			"Failed to create tags for the spot instance request",
			err.Error())
		return err
	}

	logger.Println(asgName, "successfully tagged spot instance request",
		*s.SpotInstanceRequestId)

	return nil
}
