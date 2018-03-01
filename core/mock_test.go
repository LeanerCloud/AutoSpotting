package autospotting

import (
	"reflect"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
)

func CheckErrors(t *testing.T, err error, expected error) {
	if err != nil && !reflect.DeepEqual(err, expected) {
		t.Errorf("Error received: '%v' expected '%v'",
			err.Error(), expected.Error())
	}
}

// All fields are composed of the abbreviation of their method
// This is useful when methods are doing multiple calls to AWS API
type mockEC2 struct {
	ec2iface.EC2API

	// Create tags
	cto   *ec2.CreateTagsOutput
	cterr error

	// Wait Until Spot Instance Request Fulfilled
	wusirferr error

	// Describe Instance Request
	dsiro   *ec2.DescribeSpotInstanceRequestsOutput
	dsirerr error

	// Describe Spot Price History
	dspho   *ec2.DescribeSpotPriceHistoryOutput
	dspherr error

	// Error in DescribeInstancesPages
	diperr error

	// Terminate Instance
	tio   *ec2.TerminateInstancesOutput
	tierr error

	// Request Spot Instance
	rsio   *ec2.RequestSpotInstancesOutput
	rsierr error

	// Describe Spot Instance Requests
	dspiro   *ec2.DescribeSpotInstanceRequestsOutput
	dspirerr error

	// Describe Regions
	dro   *ec2.DescribeRegionsOutput
	drerr error

	// Cancel Spot instance request
	csiro   *ec2.CancelSpotInstanceRequestsOutput
	csirerr error
}

func (m mockEC2) CreateTags(in *ec2.CreateTagsInput) (*ec2.CreateTagsOutput, error) {
	return m.cto, m.cterr
}

func (m mockEC2) WaitUntilSpotInstanceRequestFulfilled(in *ec2.DescribeSpotInstanceRequestsInput) error {
	return m.wusirferr
}

func (m mockEC2) DescribeSpotInstanceRequests(in *ec2.DescribeSpotInstanceRequestsInput) (*ec2.DescribeSpotInstanceRequestsOutput, error) {
	return m.dsiro, m.dsirerr
}

func (m mockEC2) DescribeSpotPriceHistory(in *ec2.DescribeSpotPriceHistoryInput) (*ec2.DescribeSpotPriceHistoryOutput, error) {
	return m.dspho, m.dspherr
}

func (m mockEC2) DescribeInstancesPages(in *ec2.DescribeInstancesInput, fn func(*ec2.DescribeInstancesOutput, bool) bool) error {
	return m.diperr
}

func (m mockEC2) TerminateInstances(*ec2.TerminateInstancesInput) (*ec2.TerminateInstancesOutput, error) {
	return m.tio, m.tierr
}

func (m mockEC2) RequestSpotInstances(*ec2.RequestSpotInstancesInput) (*ec2.RequestSpotInstancesOutput, error) {
	return m.rsio, m.rsierr
}

func (m mockEC2) CancelSpotInstanceRequests(*ec2.CancelSpotInstanceRequestsInput) (*ec2.CancelSpotInstanceRequestsOutput, error) {
	return m.csiro, m.csirerr
}

func (m mockEC2) DescribeRegions(*ec2.DescribeRegionsInput) (*ec2.DescribeRegionsOutput, error) {
	return m.dro, m.drerr
}

// For testing we "convert" the SecurityGroupIDs/SecurityGroupNames by
// prefixing the original name/id with "sg-" if not present already.
func (m mockEC2) DescribeSecurityGroups(input *ec2.DescribeSecurityGroupsInput) (*ec2.DescribeSecurityGroupsOutput, error) {
	var groups []*ec2.SecurityGroup

	for _, groupName := range input.GroupNames {
		newgroup := *groupName

		if !strings.HasPrefix(*groupName, "sg-") {
			newgroup = "sg-" + *groupName
		}

		groups = append(groups, &ec2.SecurityGroup{GroupId: &newgroup})
	}

	for _, groupID := range input.GroupIds {
		groups = append(groups, &ec2.SecurityGroup{GroupId: aws.String(*groupID)})
	}

	return &ec2.DescribeSecurityGroupsOutput{SecurityGroups: groups}, nil
}

// All fields are composed of the abbreviation of their method
// This is useful when methods are doing multiple calls to AWS API
type mockASG struct {
	autoscalingiface.AutoScalingAPI
	// Detach Instances
	dio   *autoscaling.DetachInstancesOutput
	dierr error
	// Attach Instances
	aio   *autoscaling.AttachInstancesOutput
	aierr error
	// Describe Launch Config
	dlco   *autoscaling.DescribeLaunchConfigurationsOutput
	dlcerr error
	// Update AutoScaling Group
	uasgo   *autoscaling.UpdateAutoScalingGroupOutput
	uasgerr error
	// Describe Tags
	dto   *autoscaling.DescribeTagsOutput
	dterr error
	// Describe AutoScaling Group
	dasgo   *autoscaling.DescribeAutoScalingGroupsOutput
	dasgerr error
}

func (m mockASG) DetachInstances(*autoscaling.DetachInstancesInput) (*autoscaling.DetachInstancesOutput, error) {
	return m.dio, m.dierr
}

func (m mockASG) AttachInstances(*autoscaling.AttachInstancesInput) (*autoscaling.AttachInstancesOutput, error) {
	return m.aio, m.aierr
}

func (m mockASG) DescribeLaunchConfigurations(*autoscaling.DescribeLaunchConfigurationsInput) (*autoscaling.DescribeLaunchConfigurationsOutput, error) {
	return m.dlco, m.dlcerr
}

func (m mockASG) UpdateAutoScalingGroup(*autoscaling.UpdateAutoScalingGroupInput) (*autoscaling.UpdateAutoScalingGroupOutput, error) {
	return m.uasgo, m.uasgerr
}

func (m mockASG) DescribeTagsPages(input *autoscaling.DescribeTagsInput, function func(*autoscaling.DescribeTagsOutput, bool) bool) error {
	function(m.dto, true)
	return nil
}

func (m mockASG) DescribeAutoScalingGroupsPages(input *autoscaling.DescribeAutoScalingGroupsInput, function func(*autoscaling.DescribeAutoScalingGroupsOutput, bool) bool) error {
	function(m.dasgo, true)
	return nil
}
