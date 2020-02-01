// Copyright (c) 2016-2019 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0
package autospotting

import (
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
)

func CheckErrors(t *testing.T, err error, expected error) {
	if err != nil && expected != nil && !reflect.DeepEqual(err, expected) {
		t.Errorf("Error received: '%v' expected '%v'",
			err.Error(), expected.Error())
	}
}

// All fields are composed of the abbreviation of their method
// This is useful when methods are doing multiple calls to AWS API
type mockEC2 struct {
	ec2iface.EC2API

	// DescribeSpotPriceHistoryPages output
	dsphpo []*ec2.DescribeSpotPriceHistoryOutput

	// DescribeSpotPriceHistoryPages error
	dsphperr error

	// DescribeInstancesOutput
	dio *ec2.DescribeInstancesOutput

	// DescribeInstancesPages error
	diperr error

	// DescribeInstanceAttribute
	diao   *ec2.DescribeInstanceAttributeOutput
	diaerr error

	// Terminate Instance
	tio   *ec2.TerminateInstancesOutput
	tierr error

	// Describe Regions
	dro   *ec2.DescribeRegionsOutput
	drerr error

	// Delete Tags
	dto   *ec2.DeleteTagsOutput
	dterr error

	// DescribeLaunchTemplateVersionsOutput
	dltvo   *ec2.DescribeLaunchTemplateVersionsOutput
	dltverr error
}

func (m mockEC2) DescribeSpotPriceHistoryPages(in *ec2.DescribeSpotPriceHistoryInput, f func(*ec2.DescribeSpotPriceHistoryOutput, bool) bool) error {
	for i, page := range m.dsphpo {
		f(page, i == len(m.dsphpo)-1)
	}
	return m.dsphperr
}

func (m mockEC2) DescribeInstancesPages(in *ec2.DescribeInstancesInput, f func(*ec2.DescribeInstancesOutput, bool) bool) error {
	f(m.dio, true)
	return m.diperr
}

func (m mockEC2) DescribeInstanceAttribute(in *ec2.DescribeInstanceAttributeInput) (*ec2.DescribeInstanceAttributeOutput, error) {
	return m.diao, m.diaerr
}

func (m mockEC2) TerminateInstances(*ec2.TerminateInstancesInput) (*ec2.TerminateInstancesOutput, error) {
	return m.tio, m.tierr
}

func (m mockEC2) DescribeRegions(*ec2.DescribeRegionsInput) (*ec2.DescribeRegionsOutput, error) {
	return m.dro, m.drerr
}

func (m mockEC2) DeleteTags(*ec2.DeleteTagsInput) (*ec2.DeleteTagsOutput, error) {
	return m.dto, m.dterr
}

func (m mockEC2) DescribeLaunchTemplateVersions(*ec2.DescribeLaunchTemplateVersionsInput) (*ec2.DescribeLaunchTemplateVersionsOutput, error) {
	return m.dltvo, m.dltverr
}

// All fields are composed of the abbreviation of their method
// This is useful when methods are doing multiple calls to AWS API
type mockASG struct {
	autoscalingiface.AutoScalingAPI
	// Detach Instances
	dio   *autoscaling.DetachInstancesOutput
	dierr error
	// Terminate Instances
	tiiasgo   *autoscaling.TerminateInstanceInAutoScalingGroupOutput
	tiiasgerr error
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
	dto *autoscaling.DescribeTagsOutput

	// Describe AutoScaling Group
	dasgo   *autoscaling.DescribeAutoScalingGroupsOutput
	dasgerr error

	// Describe AutoScalingInstances
	dasio   *autoscaling.DescribeAutoScalingInstancesOutput
	dasierr error

	// DescribeLifecycleHooks
	dlho   *autoscaling.DescribeLifecycleHooksOutput
	dlherr error
}

func (m mockASG) DetachInstances(*autoscaling.DetachInstancesInput) (*autoscaling.DetachInstancesOutput, error) {
	return m.dio, m.dierr
}

func (m mockASG) TerminateInstanceInAutoScalingGroup(*autoscaling.TerminateInstanceInAutoScalingGroupInput) (*autoscaling.TerminateInstanceInAutoScalingGroupOutput, error) {
	return m.tiiasgo, m.tiiasgerr
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

func (m mockASG) DescribeAutoScalingGroups(input *autoscaling.DescribeAutoScalingGroupsInput) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	return m.dasgo, m.dasgerr
}

func (m mockASG) DescribeAutoScalingGroupsPages(input *autoscaling.DescribeAutoScalingGroupsInput, function func(*autoscaling.DescribeAutoScalingGroupsOutput, bool) bool) error {
	function(m.dasgo, true)
	return nil
}

func (m mockASG) DescribeAutoScalingInstances(inout *autoscaling.DescribeAutoScalingInstancesInput) (*autoscaling.DescribeAutoScalingInstancesOutput, error) {
	return m.dasio, m.dasierr
}

func (m mockASG) DescribeLifecycleHooks(*autoscaling.DescribeLifecycleHooksInput) (*autoscaling.DescribeLifecycleHooksOutput, error) {
	return m.dlho, m.dlherr
}

// All fields are composed of the abbreviation of their method
// This is useful when methods are doing multiple calls to AWS API
type mockCloudFormation struct {
	cloudformationiface.CloudFormationAPI
	// DescribeStacks
	dso   *cloudformation.DescribeStacksOutput
	dserr error
}

func (m mockCloudFormation) DescribeStacks(*cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error) {
	return m.dso, m.dserr
}
