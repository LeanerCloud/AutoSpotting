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

func (s *spotInstanceRequest) cancelRequest() (bool, error) {
	canCancel := false
	if s.State != nil {
		switch *s.State {
		case "capacity-not-available":
			canCancel = true
			break
		default:
			canCancel = false
		}
	}

	if canCancel {
		ec2Client := s.region.services.ec2
		params := ec2.CancelSpotInstanceRequestsInput{
			SpotInstanceRequestIds: []*string{s.SpotInstanceRequestId},
		}
		_, err := ec2Client.CancelSpotInstanceRequests(&params)
		return true, err
	} else {
		return false, nil
	}
}

// This function returns an Instance ID
func (s *spotInstanceRequest) waitForAndTagSpotInstance() error {
	logger.Println(s.asg.name, "Waiting for spot instance for spot instance request",
		*s.SpotInstanceRequestId)

	ec2Client := s.region.services.ec2

	cancelled, cancelError := s.cancelRequest()

	if cancelError != nil {
		logger.Println(s.asg.name, "Error attempting to cancel Spot request:", cancelError.Error())
		return cancelError
	}

	if cancelled {
		logger.Println(s.asg.name, "Spot request cancelled")
		return nil
	}

	params := ec2.DescribeSpotInstanceRequestsInput{
		SpotInstanceRequestIds: []*string{s.SpotInstanceRequestId},
	}

	err := ec2Client.WaitUntilSpotInstanceRequestFulfilled(&params)
	if err != nil {
		logger.Println(s.asg.name, "Error waiting for instance:", err.Error())
		return err
	}

	logger.Println(s.asg.name, "Done waiting for an instance.")

	// Now we try to get the InstanceID of the instance we got
	requestDetails, err := ec2Client.DescribeSpotInstanceRequests(&params)
	if err != nil {
		logger.Println(s.asg.name, "Failed to describe spot instance requests")
		return err
	}

	// due to the waiter we can now safely assume all this data is available
	spotInstanceID := requestDetails.SpotInstanceRequests[0].InstanceId

	logger.Println(s.asg.name, "Found new spot instance", *spotInstanceID)
	logger.Println("Tagging it to match the other instances from the group")

	// we need to re-scan in order to have the information a
	err = s.region.scanInstances()
	if err != nil {
		logger.Printf("Failed to scan instances: %s for %s\n", err, s.asg.name)
	}

	tags := s.asg.propagatedInstanceTags()

	i := s.region.instances.get(*spotInstanceID)

	if i != nil {
		i.tag(tags, defaultTimeout)
	} else {
		logger.Println(s.asg.name, "new spot instance", *spotInstanceID, "has disappeared")
	}
	return nil
}

func (s *spotInstanceRequest) tag(asgName string) error {
	svc := s.region.services.ec2
	tags := []*ec2.Tag{
		{
			Key:   aws.String("launched-for-asg"),
			Value: aws.String(asgName),
		},
	}

	for count, err := 0, errors.New("dummy"); err != nil; _, err = svc.CreateTags(&ec2.CreateTagsInput{
		Resources: []*string{s.SpotInstanceRequestId},
		Tags:      tags,
	}) {

		// after failing to tag it for 10 retries, terminate its instance and cancel
		// the spot instance request in order to avoid any orphaned instances
		// created by spot requests which failed to be tagged.
		if err != nil {
			if count > 10 {
				logger.Println(asgName,
					"Failed to create tags for the spot instance request after 10 retries",
					"cancelling the spot instance request, error: ", err.Error())
				s.reload()

				svc.TerminateInstances(&ec2.TerminateInstancesInput{
					InstanceIds: []*string{s.InstanceId},
				})

				svc.CancelSpotInstanceRequests(
					&ec2.CancelSpotInstanceRequestsInput{
						SpotInstanceRequestIds: []*string{s.SpotInstanceRequestId},
					})
				return err

			}
			logger.Println(asgName,
				"Failed to create tags for the spot instance request",
				*s.SpotInstanceRequestId, "retrying in 5 seconds...")
			count = count + 1
			time.Sleep(5 * time.Second * s.region.conf.SleepMultiplier)

		}
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
