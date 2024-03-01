// Copyright (c) 2016-2022 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

func CheckErrors(t *testing.T, err error, expected error) {
	if err != nil && expected != nil && !strings.Contains(err.Error(), expected.Error()) {
		t.Errorf("Error received: '%v' expected '%v'",
			err.Error(), expected.Error())
	}
}

// All fields are composed of the abbreviation of their method
// This is useful when methods are doing multiple calls to AWS API
type mockEC2Client struct {
	// CreateLaunchTemplate Output and error
	clto   *ec2.CreateLaunchTemplateOutput
	clterr error

	// CreateFleet input/error
	cfo   *ec2.CreateFleetOutput
	cferr error

	// DescribeSpotPriceHistoryPages output
	dsphpo   *ec2.DescribeSpotPriceHistoryOutput
	dsphperr error

	// DescribeInstancesOutput
	dio *ec2.DescribeInstancesOutput

	// DescribeInstancesPages error
	diperr error

	// DescribeInstanceAttribute
	diao   *ec2.DescribeInstanceAttributeOutput
	diaerr error

	// DescribeImagesOutput
	damio   *ec2.DescribeImagesOutput
	damierr error

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

	// WaitUntilInstanceRunning error
	wuirerr error

	// CreateTags
	cto   *ec2.CreateTagsOutput
	cterr error

	// DeleteLaunchTemplate
	dlto   *ec2.DeleteLaunchTemplateOutput
	dlterr error

	// DescribeAddresses
	dao   *ec2.DescribeAddressesOutput
	daerr error

	// DescribeLaunchTemplates
	dslto   *ec2.DescribeLaunchTemplatesOutput
	dslterr error

	// DescribeSecurityGroups
	dsgo   *ec2.DescribeSecurityGroupsOutput
	dsgerr error

	// DescribeVolumes
	dvolo   *ec2.DescribeVolumesOutput
	dvolerr error

	// AttachVolume
	avo   *ec2.AttachVolumeOutput
	averr error

	// DetachVolume
	dvo   *ec2.DetachVolumeOutput
	dverr error

	// AssociateAddress
	aao   *ec2.AssociateAddressOutput
	aaerr error

	// DisassociateAddress
	ddao  *ec2.DisassociateAddressOutput
	dderr error
}

// Implement AssociateAddress
func (m mockEC2Client) AssociateAddress(ctx context.Context, params *ec2.AssociateAddressInput, optFns ...func(*ec2.Options)) (*ec2.AssociateAddressOutput, error) {
	return m.aao, m.aaerr
}

// Implement AttachVolume
func (m mockEC2Client) AttachVolume(ctx context.Context, params *ec2.AttachVolumeInput, optFns ...func(*ec2.Options)) (*ec2.AttachVolumeOutput, error) {
	return m.avo, m.averr
}

// Implement CreateFleet
func (m mockEC2Client) CreateFleet(ctx context.Context, params *ec2.CreateFleetInput, optFns ...func(*ec2.Options)) (*ec2.CreateFleetOutput, error) {
	return m.cfo, m.cferr
}

// Implement CreateLaunchTemplate
func (m mockEC2Client) CreateLaunchTemplate(ctx context.Context, params *ec2.CreateLaunchTemplateInput, optFns ...func(*ec2.Options)) (*ec2.CreateLaunchTemplateOutput, error) {
	return m.clto, m.clterr
}

// Implement CreateTags
func (m mockEC2Client) CreateTags(ctx context.Context, params *ec2.CreateTagsInput, optFns ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error) {
	return m.cto, m.cterr
}

func (m mockEC2Client) DeleteLaunchTemplate(ctx context.Context, params *ec2.DeleteLaunchTemplateInput, optFns ...func(*ec2.Options)) (*ec2.DeleteLaunchTemplateOutput, error) {
	return m.dlto, m.dlterr
}

// Implement DescribeAddresses
func (m mockEC2Client) DescribeAddresses(ctx context.Context, params *ec2.DescribeAddressesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error) {
	return m.dao, m.daerr
}

func (m mockEC2Client) DescribeLaunchTemplates(ctx context.Context, params *ec2.DescribeLaunchTemplatesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeLaunchTemplatesOutput, error) {
	return m.dslto, m.dslterr
}

// Implement DescribeImages
func (m mockEC2Client) DescribeImages(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	return m.damio, m.damierr
}

// Implement DescribeInstances
func (m mockEC2Client) DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	return m.dio, m.diperr
}

// Implement DescribeLaunchTemplateVersions
func (m mockEC2Client) DescribeLaunchTemplateVersions(ctx context.Context, params *ec2.DescribeLaunchTemplateVersionsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeLaunchTemplateVersionsOutput, error) {
	return m.dltvo, m.dltverr
}

// Implement DescribeSpotPriceHistory
func (m mockEC2Client) DescribeSpotPriceHistory(ctx context.Context, params *ec2.DescribeSpotPriceHistoryInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSpotPriceHistoryOutput, error) {
	return m.dsphpo, m.dsphperr
}

// Implement DescribeVolumes
func (m mockEC2Client) DescribeVolumes(ctx context.Context, params *ec2.DescribeVolumesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
	return m.dvolo, m.dvolerr
}

func (m mockEC2Client) DescribeSecurityGroups(ct context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
	return m.dsgo, m.dsgerr
}

// Implement DetachVolume
func (m mockEC2Client) DetachVolume(ctx context.Context, params *ec2.DetachVolumeInput, optFns ...func(*ec2.Options)) (*ec2.DetachVolumeOutput, error) {
	return m.dvo, m.dverr
}

// Implement DisassociateAddress
func (m mockEC2Client) DisassociateAddress(ctx context.Context, params *ec2.DisassociateAddressInput, optFns ...func(*ec2.Options)) (*ec2.DisassociateAddressOutput, error) {
	return m.ddao, m.daerr
}

// Implement TerminateInstances
func (m mockEC2Client) TerminateInstances(ctx context.Context, params *ec2.TerminateInstancesInput, optFns ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error) {
	return m.tio, m.tierr
}

// Add other method implementations as needed...

// All fields are composed of the abbreviation of their method
// This is useful when methods are doing multiple calls to AWS API
type mockASG struct {
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
	// CreateOrUpdateTags
	couto   *autoscaling.CreateOrUpdateTagsOutput
	couterr error
}

func (m mockASG) AttachInstances(ctx context.Context, params *autoscaling.AttachInstancesInput, optFns ...func(*autoscaling.Options)) (*autoscaling.AttachInstancesOutput, error) {
	return m.aio, m.aierr
}

func (m mockASG) CompleteLifecycleAction(ctx context.Context, params *autoscaling.CompleteLifecycleActionInput, optFns ...func(*autoscaling.Options)) (*autoscaling.CompleteLifecycleActionOutput, error) {
	// Implement the CompleteLifecycleAction method here
	return nil, nil
}

func (m mockASG) CreateOrUpdateTags(ctx context.Context, params *autoscaling.CreateOrUpdateTagsInput, optFns ...func(*autoscaling.Options)) (*autoscaling.CreateOrUpdateTagsOutput, error) {
	return m.couto, m.couterr
}

func (m mockASG) DescribeAutoScalingGroups(ctx context.Context, params *autoscaling.DescribeAutoScalingGroupsInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	return m.dasgo, m.dasgerr
}

func (m mockASG) DescribeAutoScalingInstances(ctx context.Context, params *autoscaling.DescribeAutoScalingInstancesInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingInstancesOutput, error) {
	return m.dasio, m.dasierr
}

func (m mockASG) DescribeInstanceRefreshes(ctx context.Context, params *autoscaling.DescribeInstanceRefreshesInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeInstanceRefreshesOutput, error) {
	// Implement the DescribeInstanceRefreshes method here
	return nil, nil
}

func (m mockASG) DescribeLaunchConfigurations(ctx context.Context, params *autoscaling.DescribeLaunchConfigurationsInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeLaunchConfigurationsOutput, error) {
	return m.dlco, m.dlcerr
}

func (m mockASG) DescribeLifecycleHooks(ctx context.Context, params *autoscaling.DescribeLifecycleHooksInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeLifecycleHooksOutput, error) {
	return m.dlho, m.dlherr
}

func (m mockEC2Client) DescribeRegions(ctx context.Context, in *ec2.DescribeRegionsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeRegionsOutput, error) {
	// Implement the behavior of DescribeRegions as needed for your tests
	// You can return a mock output and error here
	return m.dro, m.drerr
}

func (m mockASG) DescribeScalingActivities(ctx context.Context, params *autoscaling.DescribeScalingActivitiesInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeScalingActivitiesOutput, error) {
	// Implement the DescribeScalingActivities method here
	return nil, nil
}

func (m mockASG) ResumeProcesses(ctx context.Context, params *autoscaling.ResumeProcessesInput, optFns ...func(*autoscaling.Options)) (*autoscaling.ResumeProcessesOutput, error) {
	// Implement the ResumeProcesses method here
	return nil, nil
}

func (m mockASG) SetInstanceProtection(ctx context.Context, params *autoscaling.SetInstanceProtectionInput, optFns ...func(*autoscaling.Options)) (*autoscaling.SetInstanceProtectionOutput, error) {
	// Implement the SetInstanceProtection method here
	return nil, nil
}

func (m mockASG) SuspendProcesses(ctx context.Context, params *autoscaling.SuspendProcessesInput, optFns ...func(*autoscaling.Options)) (*autoscaling.SuspendProcessesOutput, error) {
	// Implement the SuspendProcesses method here
	return nil, nil
}

func (m mockASG) TerminateInstanceInAutoScalingGroup(ctx context.Context, params *autoscaling.TerminateInstanceInAutoScalingGroupInput, optFns ...func(*autoscaling.Options)) (*autoscaling.TerminateInstanceInAutoScalingGroupOutput, error) {
	return m.tiiasgo, m.tiiasgerr
}

func (m mockASG) UpdateAutoScalingGroup(ctx context.Context, params *autoscaling.UpdateAutoScalingGroupInput, optFns ...func(*autoscaling.Options)) (*autoscaling.UpdateAutoScalingGroupOutput, error) {
	return m.uasgo, m.uasgerr
}

// All fields are composed of the abbreviation of their method
// This is useful when methods are doing multiple calls to AWS API
type mockCloudFormation struct {
	cloudformation.Client
	// DescribeStacks
	dso   *cloudformation.DescribeStacksOutput
	dserr error
}

func (m mockCloudFormation) DescribeStacks(ctx context.Context, in *cloudformation.DescribeStacksInput, cfOpts ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
	return m.dso, m.dserr
}

// All fields are composed of the abbreviation of their method
// This is useful when methods are doing multiple calls to AWS API
type mockSQS struct {

	// SendMessage
	smo   *sqs.SendMessageOutput
	smerr error

	//DeleteMessage
	dmo   *sqs.DeleteMessageOutput
	dmerr error
}

func (m mockSQS) SendMessage(ctx context.Context, in *sqs.SendMessageInput) (*sqs.SendMessageOutput, error) {
	return m.smo, m.smerr
}

func (m mockSQS) DeleteMessage(ctx context.Context, in *sqs.DeleteMessageInput) (*sqs.DeleteMessageOutput, error) {
	return m.dmo, m.dmerr
}

// utility function for checking if error messages are matching
func errorMatches(got error, wanted error) bool {
	if got == nil {
		return wanted == nil
	}
	if wanted == nil {
		return false
	}
	return strings.Contains(got.Error(), wanted.Error())
}

type mockSSM struct {
	gpo    *ssm.GetParameterOutput
	gpoerr error

	gpho    *ssm.GetParameterHistoryOutput
	gphoerr error

	// unused so far
	//poo *ssm.PutParameterOutput

}

func (m mockSSM) GetParameter(ctx context.Context, in *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
	return m.gpo, m.gpoerr
}
func (m mockSSM) GetParameterHistory(ctx context.Context, in *ssm.GetParameterHistoryInput) (*ssm.GetParameterHistoryOutput, error) {
	return m.gpho, m.gphoerr
}
