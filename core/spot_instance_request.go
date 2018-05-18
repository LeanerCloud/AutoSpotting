package autospotting

import (
	"errors"
	"time"

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

func (s *spotInstanceRequest) tag(asgName string) error {
	svc := s.region.services.ec2
	params := ec2.CreateTagsInput{
		Resources: []*string{s.SpotInstanceRequestId},
		Tags: []*ec2.Tag{
			{
				Key:   aws.String("launched-for-asg"),
				Value: aws.String(asgName),
			},
			{
				Key:   aws.String(DefaultSIRRequestCompleteTagName),
				Value: aws.String("false"),
			},
		},
	}

	for count := 1; count < 11; count++ {
		_, err := svc.CreateTags(&params)

		if err != nil {
			if count == 10 {
				logger.Println("Failed to mark the spot instance request as complete after 10 tries:",
					"cancelling the spot instance request, error: ", err.Error())

				s.reload()

				if s.InstanceId != nil {
					svc.TerminateInstances(&ec2.TerminateInstancesInput{
						InstanceIds: []*string{s.InstanceId},
					})
				}

				svc.CancelSpotInstanceRequests(
					&ec2.CancelSpotInstanceRequestsInput{
						SpotInstanceRequestIds: []*string{s.SpotInstanceRequestId},
					})
				return err
			}
			logger.Println(asgName,
				"Failed to create tags for the spot instance request",
				*s.SpotInstanceRequestId, "retrying in 5 seconds...")
			time.Sleep(5 * time.Second * s.region.conf.SleepMultiplier)
			continue
		}
		break
	}

	logger.Println(asgName, "successfully tagged spot instance request",
		*s.SpotInstanceRequestId)

	return nil
}

func (s *spotInstanceRequest) reload() error {

	resp, err := s.region.services.ec2.DescribeSpotInstanceRequests(
		&ec2.DescribeSpotInstanceRequestsInput{
			SpotInstanceRequestIds: []*string{s.SpotInstanceRequestId},
		})

	if err != nil {
		return err
	}

	if resp != nil && len(resp.SpotInstanceRequests) == 1 {
		s.SpotInstanceRequest = resp.SpotInstanceRequests[0]
		return nil
	}
	return errors.New("spot instance request not found")

}
