package autospotting

import (
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"testing"
)

func CheckErrors(t *testing.T, err error, expected error) {
	if err == nil && expected != err {
		t.Errorf("Error received: %v expected %v", err, expected.Error())
	} else if err != nil && expected == nil {
		t.Errorf("Error received: %s expected %s", err.Error(), expected)
	} else if err != nil && expected != nil && err.Error() != expected.Error() {
		t.Errorf("Error received: %s expected %s", err.Error(), expected.Error())
	}
}

// All fields are composed of the abreviation of their method
// This is useful when methods are doing multiple calls to AWS API
type mockEC2 struct {
	ec2iface.EC2API
	// Create tags
	cto   *ec2.CreateTagsOutput
	cterr error
	// Wait Until Spot Instance Request Fullfilled
	wusirferr error
	// Describe Instance Request
	dsiro   *ec2.DescribeSpotInstanceRequestsOutput
	dsirerr error
	// Describe Spot Price History
	dspho   *ec2.DescribeSpotPriceHistoryOutput
	dspherr error
	// Describe Instance
	dio   *ec2.DescribeInstancesOutput
	dierr error
	// Terminate Instance
	tio   *ec2.TerminateInstancesOutput
	tierr error
	// Request Spot Instance
	rsio   *ec2.RequestSpotInstancesOutput
	rsierr error
	// Describe Spot Instance Requests
	dspiro   *ec2.DescribeSpotInstanceRequestsOutput
	dspirerr error
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

func (m mockEC2) DescribeInstances(in *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	return m.dio, m.dierr
}

func (m mockEC2) TerminateInstances(*ec2.TerminateInstancesInput) (*ec2.TerminateInstancesOutput, error) {
	return m.tio, m.tierr
}

func (m mockEC2) RequestSpotInstances(*ec2.RequestSpotInstancesInput) (*ec2.RequestSpotInstancesOutput, error) {
	return m.rsio, m.rsierr
}

// All fields are composed of the abreviation of their method
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
