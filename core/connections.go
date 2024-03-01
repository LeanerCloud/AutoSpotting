package autospotting

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/codedeploy"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancing"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// AutoScalingAPI defines the methods from autoscaling.Client that we use.
type AutoScalingAPI interface {
	AttachInstances(ctx context.Context, params *autoscaling.AttachInstancesInput, optFns ...func(*autoscaling.Options)) (*autoscaling.AttachInstancesOutput, error)
	CompleteLifecycleAction(ctx context.Context, params *autoscaling.CompleteLifecycleActionInput, optFns ...func(*autoscaling.Options)) (*autoscaling.CompleteLifecycleActionOutput, error)
	CreateOrUpdateTags(ctx context.Context, params *autoscaling.CreateOrUpdateTagsInput, optFns ...func(*autoscaling.Options)) (*autoscaling.CreateOrUpdateTagsOutput, error)
	DescribeAutoScalingGroups(ctx context.Context, params *autoscaling.DescribeAutoScalingGroupsInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error)
	DescribeAutoScalingInstances(ctx context.Context, params *autoscaling.DescribeAutoScalingInstancesInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingInstancesOutput, error)
	DescribeInstanceRefreshes(ctx context.Context, params *autoscaling.DescribeInstanceRefreshesInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeInstanceRefreshesOutput, error)
	DescribeLaunchConfigurations(ctx context.Context, params *autoscaling.DescribeLaunchConfigurationsInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeLaunchConfigurationsOutput, error)
	DescribeLifecycleHooks(ctx context.Context, params *autoscaling.DescribeLifecycleHooksInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeLifecycleHooksOutput, error)
	DescribeScalingActivities(ctx context.Context, params *autoscaling.DescribeScalingActivitiesInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeScalingActivitiesOutput, error)
	ResumeProcesses(ctx context.Context, params *autoscaling.ResumeProcessesInput, optFns ...func(*autoscaling.Options)) (*autoscaling.ResumeProcessesOutput, error)
	SetInstanceProtection(ctx context.Context, params *autoscaling.SetInstanceProtectionInput, optFns ...func(*autoscaling.Options)) (*autoscaling.SetInstanceProtectionOutput, error)
	SuspendProcesses(ctx context.Context, params *autoscaling.SuspendProcessesInput, optFns ...func(*autoscaling.Options)) (*autoscaling.SuspendProcessesOutput, error)
	TerminateInstanceInAutoScalingGroup(ctx context.Context, params *autoscaling.TerminateInstanceInAutoScalingGroupInput, optFns ...func(*autoscaling.Options)) (*autoscaling.TerminateInstanceInAutoScalingGroupOutput, error)
	UpdateAutoScalingGroup(ctx context.Context, params *autoscaling.UpdateAutoScalingGroupInput, optFns ...func(*autoscaling.Options)) (*autoscaling.UpdateAutoScalingGroupOutput, error)
}

// EC2API defines the methods from ec2.Client that we use.
type EC2API interface {
	AssociateAddress(ctx context.Context, params *ec2.AssociateAddressInput, optFns ...func(*ec2.Options)) (*ec2.AssociateAddressOutput, error)
	AttachVolume(ctx context.Context, params *ec2.AttachVolumeInput, optFns ...func(*ec2.Options)) (*ec2.AttachVolumeOutput, error)
	CreateFleet(ctx context.Context, params *ec2.CreateFleetInput, optFns ...func(*ec2.Options)) (*ec2.CreateFleetOutput, error)
	CreateLaunchTemplate(ctx context.Context, params *ec2.CreateLaunchTemplateInput, optFns ...func(*ec2.Options)) (*ec2.CreateLaunchTemplateOutput, error)
	CreateTags(ctx context.Context, params *ec2.CreateTagsInput, optFns ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error)
	DescribeAddresses(ctx context.Context, params *ec2.DescribeAddressesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error)
	DescribeImages(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error)
	DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	DescribeLaunchTemplateVersions(ctx context.Context, params *ec2.DescribeLaunchTemplateVersionsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeLaunchTemplateVersionsOutput, error)
	DescribeRegions(ctx context.Context, params *ec2.DescribeRegionsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeRegionsOutput, error)
	DescribeSecurityGroups(ct context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error)
	DescribeSpotPriceHistory(ctx context.Context, params *ec2.DescribeSpotPriceHistoryInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSpotPriceHistoryOutput, error)
	DescribeVolumes(ctx context.Context, params *ec2.DescribeVolumesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error)
	DetachVolume(ctx context.Context, params *ec2.DetachVolumeInput, optFns ...func(*ec2.Options)) (*ec2.DetachVolumeOutput, error)
	DisassociateAddress(ctx context.Context, params *ec2.DisassociateAddressInput, optFns ...func(*ec2.Options)) (*ec2.DisassociateAddressOutput, error)
	TerminateInstances(ctx context.Context, params *ec2.TerminateInstancesInput, optFns ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error)
}

// SQSAPI defines the methods from sqs.Client that we use.
type SQSAPI interface {
	DeleteMessage(ctx context.Context, params *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
}

// ElasticLoadBalancingV2API defines the methods from elasticloadbalancingv2.Client that we use.
type ElasticLoadBalancingV2API interface {
	DeregisterTargets(ctx context.Context, params *elasticloadbalancingv2.DeregisterTargetsInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DeregisterTargetsOutput, error)
	DescribeTargetGroups(ctx context.Context, params *elasticloadbalancingv2.DescribeTargetGroupsInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTargetGroupsOutput, error)
	DescribeTargetHealth(ctx context.Context, params *elasticloadbalancingv2.DescribeTargetHealthInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTargetHealthOutput, error)
}

// ElasticLoadBalancingAPI defines the methods from elasticloadbalancing.Client that we use.
type ElasticLoadBalancingAPI interface {
	DeregisterInstancesFromLoadBalancer(ctx context.Context, params *elasticloadbalancing.DeregisterInstancesFromLoadBalancerInput, optFns ...func(*elasticloadbalancing.Options)) (*elasticloadbalancing.DeregisterInstancesFromLoadBalancerOutput, error)
	DescribeLoadBalancers(ctx context.Context, params *elasticloadbalancing.DescribeLoadBalancersInput, optFns ...func(*elasticloadbalancing.Options)) (*elasticloadbalancing.DescribeLoadBalancersOutput, error)
}

// CloudFormationAPI defines the methods from cloudformation.Client that we use.
type CloudFormationAPI interface {
	DescribeStacks(ctx context.Context, params *cloudformation.DescribeStacksInput, optFns ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error)
}

// CodeDeployAPI defines the methods from codedeploy.Client that we use.
type CodeDeployAPI interface {
	CreateDeployment(ctx context.Context, params *codedeploy.CreateDeploymentInput, optFns ...func(*codedeploy.Options)) (*codedeploy.CreateDeploymentOutput, error)
	GetDeploymentGroup(ctx context.Context, params *codedeploy.GetDeploymentGroupInput, optFns ...func(*codedeploy.Options)) (*codedeploy.GetDeploymentGroupOutput, error)
	ListApplications(ctx context.Context, params *codedeploy.ListApplicationsInput, optFns ...func(*codedeploy.Options)) (*codedeploy.ListApplicationsOutput, error)
	ListDeploymentGroups(ctx context.Context, params *codedeploy.ListDeploymentGroupsInput, optFns ...func(*codedeploy.Options)) (*codedeploy.ListDeploymentGroupsOutput, error)
}

// SSMAPI defines the methods from ssm.Client that we use.
type SSMAPI interface {
	GetParameter(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
	GetParameterHistory(ctx context.Context, params *ssm.GetParameterHistoryInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterHistoryOutput, error)
	PutParameter(ctx context.Context, params *ssm.PutParameterInput, optFns ...func(*ssm.Options)) (*ssm.PutParameterOutput, error)
}

// SNSAPI defines the methods from sns.Client that we use.
type SNSAPI interface {
	Publish(ctx context.Context, params *sns.PublishInput, optFns ...func(*sns.Options)) (*sns.PublishOutput, error)
}

// ECSAPI defines the methods from ecs.Client that we use.
type ECSAPI interface {
	DescribeContainerInstances(ctx context.Context, params *ecs.DescribeContainerInstancesInput, optFns ...func(*ecs.Options)) (*ecs.DescribeContainerInstancesOutput, error)
	ListClusters(ctx context.Context, params *ecs.ListClustersInput, optFns ...func(*ecs.Options)) (*ecs.ListClustersOutput, error)
	ListContainerInstances(ctx context.Context, params *ecs.ListContainerInstancesInput, optFns ...func(*ecs.Options)) (*ecs.ListContainerInstancesOutput, error)
	UpdateContainerInstancesState(ctx context.Context, params *ecs.UpdateContainerInstancesStateInput, optFns ...func(*ecs.Options)) (*ecs.UpdateContainerInstancesStateOutput, error)
}

type connections struct {
	cfg            aws.Config
	autoScaling    AutoScalingAPI
	ec2            EC2API
	elbv2          ElasticLoadBalancingV2API
	elb            ElasticLoadBalancingAPI
	cloudFormation CloudFormationAPI
	sqs            SQSAPI
	codedeploy     CodeDeployAPI
	ssm            SSMAPI
	sns            SNSAPI
	ecs            ECSAPI
	region         string
}

func (c *connections) setConfig(ctx context.Context, region string) error {
	var err error
	c.cfg, err = config.LoadDefaultConfig(ctx, config.WithRegion(region))
	return err
}

func (c *connections) connect(region, mainRegion string) error {
	debug.Println("Creating service connections in", region)

	ctx := context.TODO()

	if c.cfg.Region == "" {
		err := c.setConfig(ctx, region)
		if err != nil {
			return err
		}
	}

	c.autoScaling = autoscaling.NewFromConfig(c.cfg)
	c.ec2 = ec2.NewFromConfig(c.cfg)
	c.cloudFormation = cloudformation.NewFromConfig(c.cfg)
	c.codedeploy = codedeploy.NewFromConfig(c.cfg)
	c.ecs = ecs.NewFromConfig(c.cfg)
	c.elbv2 = elasticloadbalancingv2.NewFromConfig(c.cfg)
	c.elb = elasticloadbalancing.NewFromConfig(c.cfg)

	// For SQS, SSM, and SNS, using mainRegion if specified
	if mainRegion != "" {
		mainCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(mainRegion))
		if err != nil {
			return err
		}
		c.sqs = sqs.NewFromConfig(mainCfg)
		c.ssm = ssm.NewFromConfig(mainCfg)
		c.sns = sns.NewFromConfig(mainCfg)
	} else {
		c.sqs = sqs.NewFromConfig(c.cfg)
		c.ssm = ssm.NewFromConfig(c.cfg)
		c.sns = sns.NewFromConfig(c.cfg)
	}

	c.region = region

	debug.Println("Created service connections in", region)
	return nil
}
