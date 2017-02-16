package autospotting

import (
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
)

type mock struct {
	ec2iface.EC2API
	er      error
	dsiro   *ec2.DescribeSpotInstanceRequestsOutput
	dsiroer error
	dio     *ec2.DescribeInstancesOutput
	dioer   error
}

func (m mock) CreateTags(in *ec2.CreateTagsInput) (*ec2.CreateTagsOutput, error) {
	return nil, m.er
}

func (m mock) WaitUntilSpotInstanceRequestFulfilled(in *ec2.DescribeSpotInstanceRequestsInput) error {
	return m.er
}

func (m mock) DescribeSpotInstanceRequests(in *ec2.DescribeSpotInstanceRequestsInput) (*ec2.DescribeSpotInstanceRequestsOutput, error) {
	return m.dsiro, m.dsiroer
}

func (m mock) DescribeInstances(in *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	return m.dio, m.dioer
}

func (m mock) TerminateInstances(*ec2.TerminateInstancesInput) (*ec2.TerminateInstancesOutput, error) {
	return nil, m.er
}
