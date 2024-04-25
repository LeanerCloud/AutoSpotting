// Copyright (c) 2016-2022 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/aws/aws-sdk-go-v2/service/ecs"

	elb "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancing"
	elbTypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancing/types"

	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2Types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/aws/aws-sdk-go/aws"
)

type elasticIPDetails struct {
	AllocationID  *string
	AssociationID *string
}

// instance_actions.go contains functions that act on instances, altering their state.

func (i *instance) handleInstanceStates() (bool, error) {
	log.Printf("%s Found instance %s in state %s",
		i.region.name, *i.InstanceId, i.State.Name)

	if i.State.Name != "running" {
		log.Printf("%s Instance %s is not in the running state",
			i.region.name, *i.InstanceId)
		return true, errors.New("instance not in running state")
	}

	unattached := i.isUnattachedSpotInstanceLaunchedForAnEnabledASG()
	if !unattached {
		log.Printf("%s Instance %s is already attached to an ASG, skipping it",
			i.region.name, *i.InstanceId)
		return true, nil
	}
	return false, nil
}

func (i *instance) launchSpotReplacement() (*string, error) {
	return i.launchReplacement("spot")
}

func (i *instance) launchOnDemandReplacement() (*string, error) {
	return i.launchReplacement("")
}

// reuse the Launch Template of the ASG if the ASG is using a launch template or
// clone Launch Configuration into a new temporary Launch Template for this ASG
func (i *instance) createOrReuseLaunchTemplate(replacementLifecycle string) (*string, *string, *ec2Types.ResponseLaunchTemplateData, error) {

	if i.asg.LaunchTemplate != nil {
		log.Printf("Reusing the Launch template from the ASG %s", i.asg.name)
		ltData, err := i.asg.getLaunchTemplateData(*i.asg.LaunchTemplate.LaunchTemplateName, *i.asg.LaunchTemplate.Version)
		if err != nil {
			log.Printf("Failed to describe launch template data for ASG %s: %v", i.asg.name, err)
			return nil, nil, nil, err
		}
		return i.asg.LaunchTemplate.LaunchTemplateName, i.asg.LaunchTemplate.Version, ltData, nil
	}

	if i.asg.MixedInstancesPolicy != nil && i.asg.MixedInstancesPolicy.LaunchTemplate != nil {
		lt := i.asg.MixedInstancesPolicy.LaunchTemplate.LaunchTemplateSpecification
		log.Printf("Reusing the Launch template from the Mixed Instances Policy of ASG %s", i.asg.name)
		ltData, err := i.asg.getLaunchTemplateData(*lt.LaunchTemplateName, *lt.Version)
		if err != nil {
			log.Printf("Failed to describe launch template data for ASG %s: %v", i.asg.name, err)
			return nil, nil, nil, err
		}
		return lt.LaunchTemplateName, lt.Version, ltData, nil
	}

	log.Printf("Couldn't determine an existing launch template, creating a new Launch Template for the ASG %s", i.asg.name)
	debug.Printf("ASG data %v", i.asg)

	ltData, err := i.createLaunchTemplateData(replacementLifecycle)

	if err != nil {
		log.Println("failed to create LaunchTemplate data,", err.Error())
		return nil, nil, nil, err
	}
	debug.Printf("Launch template data: %+#v", ltData)

	ltName, ltVersion, err := i.createFleetLaunchTemplate(ltData)

	if err != nil {
		log.Println(i.region, i.asg.name, "createFleetLaunchTemplate() failure:", err.Error())
		return nil, nil, nil, err
	}
	log.Printf("Using Launch Template: %s with version %s", *ltName, *ltVersion)
	return ltName, ltVersion, nil, nil
}

func (i *instance) launchReplacement(replacementLifecycle string) (*string, error) {
	log.Printf("Launching replacement instance for %s", *i.InstanceId)
	instanceTypes, err := i.getCompatibleSpotInstanceTypesList(
		i.asg.config.PrioritizedInstanceTypesBias,
		i.asg.config.AllowedInstanceTypes,
		i.asg.config.DisallowedInstanceTypes)

	if err != nil {
		log.Println("Couldn't determine the list of compatible spot instance types")
		return nil, err
	}

	ltName, ltVersion, ltData, err := i.createOrReuseLaunchTemplate(replacementLifecycle)
	if err != nil {
		log.Println("Couldn't create or determine the Launch Template")
		return nil, err
	}

	log.Printf("Using the following list of %s instance types: %s", replacementLifecycle, strings.Join(instanceTypes, ", "))

	cfi := i.createFleetInput(ltName, ltVersion, ltData, instanceTypes, replacementLifecycle)

	debug.Printf("Fleet Input: %+#v", cfi)

	resp, err := i.region.services.ec2.CreateFleet(context.TODO(), cfi)

	if err != nil {
		log.Println(i.region, i.asg.name, "CreateFleet() failure:", err.Error())
		return nil, err
	}

	debug.Printf("CreateFleet() response: %#v", resp)

	if resp != nil && len(resp.Instances) > 0 && len(resp.Instances[0].InstanceIds) > 0 {

		inst := resp.Instances[0]
		id := inst.InstanceIds[0]

		lifecycle := "spot"
		if inst.Lifecycle != "spot" {
			lifecycle = "on-demand"
		}

		log.Printf("%s %s CreateFleet() successfully launched %s instance: %s", i.region.name, i.asg.name, lifecycle, id)
		return &id, nil
	}

	if resp.Errors != nil {
		errors := getInstanceTypesAndErrors(resp.Errors)
		return nil, fmt.Errorf("couldn't launch spot instance, encountered errors: %#v", errors)
	}

	return nil, fmt.Errorf("couldn't launch spot instance because of unknown errors (this code should never be reached)")
}

func getInstanceTypesAndErrors(errors []ec2Types.CreateFleetError) map[string]string {
	instanceTypesAndErrorMessages := make(map[string]string)
	for _, e := range errors {
		instanceTypesAndErrorMessages[string(e.LaunchTemplateAndOverrides.Overrides.InstanceType)] = *e.ErrorMessage
	}
	return instanceTypesAndErrorMessages
}

func validateInputs(i *instance, asg *autoScalingGroup) error {
	if i == nil {
		log.Printf("Error: Instance is nil")
		return fmt.Errorf("instance is nil")
	}

	if i.InstanceId == nil {
		log.Printf("Error: Instance ID is nil")
		return fmt.Errorf("instance ID is nil")
	}

	if asg == nil {
		log.Printf("Error: AutoScalingGroup is nil")
		return fmt.Errorf("autoScalingGroup is nil")
	}

	return nil
}

func (i *instance) getReplacementInstance() (*instance, error) {

	newLifecycle := i.getLifecycleValue()

	log.Printf("Finding existing swap candidate for %s instance %s", newLifecycle, *i.InstanceId)
	swapCandidate, err := i.getExistingSwapCandidate()
	if err != nil {
		log.Printf("Couldn't find suitable swap candidate for instance %s, terminating instance. Error: %s",
			*i.InstanceId, err.Error())
		err := i.terminate()
		if err != nil {
			log.Printf("Couldn't terminate instance %s: %v", *i.InstanceId, err)
		}
		return nil, err
	}
	return swapCandidate, nil
}

func checkASGValues(asg *autoScalingGroup) error {
	if asg.DesiredCapacity == nil || asg.MaxSize == nil || asg.MinSize == nil {
		log.Printf("Error: autoScalingGroup has nil values")
		return fmt.Errorf("autoScalingGroup has nil values")
	}
	return nil
}

func handleASGSizeAdjustments(asg *autoScalingGroup) {
	desiredCapacity, maxSize, minSize := *asg.DesiredCapacity, *asg.MaxSize, *asg.MinSize
	log.Printf("ASG status - DesiredCapacity: %d, MaxSize: %d, MinSize: %d", desiredCapacity, maxSize, minSize)

	if minSize == maxSize {
		log.Printf("%s: Temporarily increasing MaxSize", asg.name)
		if err := asg.setAutoScalingMaxSize(maxSize + 1); err != nil {
			log.Printf("Couldn't set ASG max size for AutoScalingGroup %s, error: %s", asg.name, err.Error())
		}
		defer asg.setAutoScalingMaxSize(maxSize)
	}
}

func (i *instance) getLifecycleValue() string {
	defaultValue := "on-demand"
	if i != nil && i.InstanceLifecycle != "" {
		return string(i.InstanceLifecycle)
	}
	return defaultValue
}

func (i *instance) replaceInstanceInASG(asg *autoScalingGroup, existingInstance *instance) (*instance, error) {
	desiredCapacity, minSize, maxSize := *asg.DesiredCapacity, *asg.MinSize, *asg.MaxSize
	log.Printf("Starting instance replacement. ASG desired capacity: %d, min size: %d", desiredCapacity, minSize)

	newInstanceLifecycle := i.getLifecycleValue()
	existingInstanceLifecycle := existingInstance.getLifecycleValue()
	log.Printf("New instance lifecycle: %s, Existing instance lifecycle: %s", newInstanceLifecycle, existingInstanceLifecycle)

	attachedEBSVolumes := existingInstance.getAttachedEBSVolumes()
	log.Printf("Found %d attached EBS volumes on existing instance %s", len(attachedEBSVolumes), *existingInstance.InstanceId)

	attachedElasticEIPs, err := existingInstance.getAttachedElasticIPs()
	if err != nil {
		log.Printf("Failed to get list of attached Elastic IPs for %s instance %s: %v", newInstanceLifecycle, *i.InstanceId, err)
		return nil, err
	}
	log.Printf("Found %d attached Elastic IPs on existing instance %s", len(attachedElasticEIPs), *existingInstance.InstanceId)

	log.Printf("Migrating stateful resources from existing instance %s", *existingInstance.InstanceId)
	if err := i.migrateResources(existingInstance, attachedEBSVolumes, attachedElasticEIPs); err != nil {
		log.Printf("Migrating stateful resources from existing instance %s failed, encounted errors: %s",
			*existingInstance.InstanceId, err.Error())
	}

	asg.suspendProcesses([]string{"Terminate", "AZRebalance"})
	defer asg.resumeProcesses([]string{"Terminate", "AZRebalance"}, 20*time.Second)

	// attach the spot instance first and then terminate the on demand instance,
	// to avoid decreasing the capacity of the group below the mimimum size
	if desiredCapacity < maxSize {

		log.Printf("Attaching %s instance %s to the group %s", newInstanceLifecycle, *i.InstanceId, asg.name)
		if err := asg.attachNewInstance(*i.InstanceId, newInstanceLifecycle, true); err != nil {
			log.Printf("Couldn't attach %s instance %s (error: %s), terminating it.",
				newInstanceLifecycle, *i.InstanceId, err.Error())
			if err := i.terminate(); err != nil {
				log.Printf("Couldn't terminate %s instance %s: %v",
					newInstanceLifecycle, *i.InstanceId, err)
			}
			return nil, fmt.Errorf("couldn't attach %s instance %s: %w",
				newInstanceLifecycle, *i.InstanceId, err)
		}

		gracePeriod := *asg.HealthCheckGracePeriod
		if gracePeriod > 600 {
			log.Printf("Capping ASG %s health check grace period %d to 600 seconds",
				asg.name, *asg.HealthCheckGracePeriod)
			gracePeriod = 600
		}

		log.Printf("Attached %s instance %s to the group %s, sleeping for %d seconds",
			newInstanceLifecycle, *i.InstanceId, asg.name, gracePeriod)
		time.Sleep(i.region.conf.SleepMultiplier * time.Duration(gracePeriod) * time.Second)
		log.Printf("Attached %s instance %s to the group %s and sleep finished, terminating existing %s instance %s",
			newInstanceLifecycle, *i.InstanceId,
			asg.name, existingInstanceLifecycle, *existingInstance.InstanceId)

		// refresh the instance information to see if the new instance is still alive
		if err := i.region.scanInstance(*i.InstanceId); err == nil {
			if inst := i.region.instances.get(*i.InstanceId); inst != nil &&
				inst.State.Name != ec2Types.InstanceStateNameRunning {
				log.Printf("new replacement %s instance %s is not running, skipping termination of the old instance %s",
					newInstanceLifecycle, *i.InstanceId, *existingInstance.InstanceId)
				return nil, fmt.Errorf("new replacement %s instance %s is not running, skipping termination of the old instance %s",
					newInstanceLifecycle, *i.InstanceId, *existingInstance.InstanceId)
			}
		}

		if err := existingInstance.terminateInstanceInAutoScalingGroup(asg); err != nil {
			log.Printf("Failed to terminate existing %s instance: %v", existingInstanceLifecycle, err)
			return nil, fmt.Errorf("couldn't terminate %s instance: %w", existingInstanceLifecycle, err)
		}

		log.Printf("%s: Successfully terminated existing %s instance %s",
			asg.name, existingInstanceLifecycle, *existingInstance.InstanceId)
		return existingInstance, nil
	} else if desiredCapacity == maxSize && desiredCapacity >= minSize {

		log.Printf("ASG %s at maximum capacity, need to terminate existing %s instance %s before attempting to attach new %s instance %s",
			asg.name, existingInstanceLifecycle, *existingInstance.InstanceId,
			newInstanceLifecycle, *i.InstanceId)

		if err := existingInstance.terminateInstanceInAutoScalingGroup(asg); err != nil {
			log.Printf("Failed to terminate existing %s instance %s: %v", existingInstanceLifecycle, *existingInstance.InstanceId, err)
			return nil, fmt.Errorf("couldn't terminate instance %s: %w", *i.InstanceId, err)
		}

		log.Printf("%s: Terminated existing %s instance %s, proceegint to attach new %s instance %s",
			asg.name, existingInstanceLifecycle, *existingInstance.InstanceId, newInstanceLifecycle, *i.InstanceId)

		if err := asg.attachNewInstance(*i.InstanceId, newInstanceLifecycle, true); err != nil {
			if err := i.terminate(); err != nil {
				log.Printf("Couldn't terminate instance %s: %v", *i.InstanceId, err)
			}
			log.Printf("Failed to attach new %s instance: %v", newInstanceLifecycle, err)
			return nil, fmt.Errorf("couldn't attach new %s instance: %w", newInstanceLifecycle, err)
		}

		log.Printf("%s: Attached new %s instance %s", asg.name, newInstanceLifecycle, *i.InstanceId)
		return existingInstance, nil
	}

	log.Printf("replaceInstanceInASG error: unexpected condition")
	return nil, fmt.Errorf("unexpected condition in replaceInstanceInASG")
}

func (i *instance) swapWithGroupMember(asg *autoScalingGroup) (*instance, error) {
	if err := validateInputs(i, asg); err != nil {
		return nil, err
	}

	replacementInstance, err := i.getReplacementInstance()
	if err != nil || replacementInstance == nil {
		log.Printf("Couldn't find replacement instance for %s", *i.InstanceId)
		return nil, err
	}

	if err := checkASGValues(asg); err != nil {
		return nil, err
	}

	// temporarily increase AutoScaling group in case the desired capacity reaches the max size,
	// otherwise attachSpotInstance might fail
	handleASGSizeAdjustments(asg)

	return i.replaceInstanceInASG(asg, replacementInstance)
}

func (i *instance) terminateInstanceInAutoScalingGroup(asg *autoScalingGroup) error {
	lifecycle := i.getLifecycleValue()
	log.Printf("Terminating %s instance %s from the group %s",
		lifecycle, *i.InstanceId, asg.name)

	if err := asg.terminateInstanceInAutoScalingGroup(i, false, false, true); err != nil {
		log.Printf("%s: %s instance %s couldn't be terminated", lifecycle, asg.name, *i.InstanceId)
		return fmt.Errorf("couldn't terminate %s instance %s: %w", lifecycle, *i.InstanceId, err)
	}
	return nil
}

func (i *instance) getExistingSwapCandidate() (*instance, error) {
	instanceToReplaceID := i.getReplacementTargetInstanceID()
	if instanceToReplaceID == nil {
		log.Println("Couldn't find target instance of", *i.InstanceId)
		return nil, fmt.Errorf("couldn't find target instance for %s", *i.InstanceId)
	}

	if err := i.region.scanInstance(*instanceToReplaceID); err != nil {
		log.Printf("Couldn't describe the target instance %s", *instanceToReplaceID)
		return nil, fmt.Errorf("target instance %s couldn't be described", *instanceToReplaceID)
	}

	instanceToReplace := i.region.instances.get(*instanceToReplaceID)
	if instanceToReplace == nil {
		log.Printf("Target instance %s couldn't be found", *instanceToReplaceID)
		return nil, fmt.Errorf("target instance %s is missing", *instanceToReplaceID)
	}

	if !instanceToReplace.shouldBeReplacedWithNewSpotInstance() {
		log.Printf("Target instance %s shouldn't be replaced", *instanceToReplaceID)
		if err := i.terminate(); err != nil {
			log.Printf("Couldn't terminate instance %s: %v", *i.InstanceId, err)
		}
		return nil, fmt.Errorf("target instance %s should not be replaced with a new Spot instance",
			*instanceToReplaceID)
	}
	return instanceToReplace, nil
}

func (i *instance) terminate() error {
	var err error
	debug.Printf("Instance: %v\n", i)

	lifecycle := i.getLifecycleValue()

	log.Printf("Terminating %s instance %v", lifecycle, *i.InstanceId)
	svc := i.region.services.ec2

	if !i.canTerminate() {
		log.Printf("Can't terminate %s instance %v, current state: %s",
			lifecycle, *i.InstanceId, i.State.Name)
		return fmt.Errorf("can't terminate %s instance %s",
			lifecycle, *i.InstanceId)
	}

	_, err = svc.TerminateInstances(context.TODO(),
		&ec2.TerminateInstancesInput{
			InstanceIds: []string{*i.InstanceId},
		})

	if err != nil {
		log.Printf("Issue while terminating %s instance %v: %v", lifecycle, *i.InstanceId, err.Error())
	}

	return err
}

// DeRegisterInstanceFromLoadBalancer de-registers an EC2 instance from a classic Elastic Load Balancer
// named `lbName`. The instance is identified by its `instanceID`.
func (i *instance) deregisterInstanceFromClasicLoadBalancer(lbName string) error {
	// Get the Load Balancer description
	output, err := i.region.services.elb.DescribeLoadBalancers(
		context.TODO(),
		&elb.DescribeLoadBalancersInput{
			LoadBalancerNames: []string{lbName},
		})
	if err != nil {
		return fmt.Errorf("failed to describe Load Balancer %s: %v", lbName, err)
	}
	if len(output.LoadBalancerDescriptions) == 0 {
		return fmt.Errorf("load Balancer %s not found", lbName)
	}
	lb := output.LoadBalancerDescriptions[0]

	// Check if the instance is already registered with the Load Balancer
	var registered bool
	for _, instance := range lb.Instances {
		if *instance.InstanceId == *i.InstanceId {
			registered = true
			break
		}
	}
	if !registered {
		log.Printf("Instance %s is not registered with Load Balancer %s", *i.InstanceId, lbName)
		return nil
	}

	// Deregister the instance from the Load Balancer
	_, err = i.region.services.elb.DeregisterInstancesFromLoadBalancer(
		context.TODO(),
		&elb.DeregisterInstancesFromLoadBalancerInput{
			LoadBalancerName: &lbName,
			Instances:        []elbTypes.Instance{{InstanceId: i.InstanceId}},
		})
	if err != nil {
		return fmt.Errorf("failed to deregister instance %s from Load Balancer %s: %v", *i.InstanceId, lbName, err)
	}

	log.Printf("Triggered deregistration of instance %s from Load Balancer %s", *i.InstanceId, lbName)
	return nil
}
func (i *instance) deregisterInstanceFromTargetGroup(tgARN string) error {
	// Get the targets of the Target Group, which includes both the instance ID and the port numbers
	targets, err := i.getTargetGroupTargets(tgARN)
	if err != nil {
		return fmt.Errorf("failed to get instance %s target information for Target Group %s: %v", *i.InstanceId, tgARN, err)
	}

	// Deregister the instance from the Target Group on all ports it may be connected to.
	var targetIDs []elbv2Types.TargetDescription
	for _, target := range targets {
		if *target.Id == *i.InstanceId {
			targetIDs = append(targetIDs, elbv2Types.TargetDescription{
				Id:   target.Id,
				Port: target.Port,
			})
		}
	}
	if len(targetIDs) > 0 {
		_, err = i.region.services.elbv2.DeregisterTargets(
			context.TODO(),
			&elbv2.DeregisterTargetsInput{
				TargetGroupArn: &tgARN,
				Targets:        targetIDs,
			})
		if err != nil {
			return fmt.Errorf("failed to deregister instance %s from Target Group %s: %v", *i.InstanceId, tgARN, err)
		}
		log.Printf("Triggered deregistration of instance %s from Target Group %s on all ports", *i.InstanceId, tgARN)
	} else {
		log.Printf("Instance %s is not registered with Target Group %s", *i.InstanceId, tgARN)
	}

	return nil
}

func (i *instance) getTargetGroupTargets(tgARN string) ([]*elbv2Types.TargetDescription, error) {
	output, err := i.region.services.elbv2.DescribeTargetHealth(
		context.TODO(),
		&elbv2.DescribeTargetHealthInput{
			TargetGroupArn: &tgARN,
		})
	if err != nil {
		return nil, fmt.Errorf("failed to describe instance %s health for Target Group %s: %v", *i.InstanceId, tgARN, err)
	}
	targets := make([]*elbv2Types.TargetDescription, 0)
	for _, target := range output.TargetHealthDescriptions {
		if *target.Target.Id == *i.InstanceId {
			targets = append(targets, target.Target)
		}
	}
	return targets, nil
}

func (i *instance) deregisterInstanceFromAllTargetGroups() error {

	input := &elbv2.DescribeTargetGroupsInput{}
	paginator := elbv2.NewDescribeTargetGroupsPaginator(i.region.services.elbv2, input)

	var wg sync.WaitGroup

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			return fmt.Errorf("failed to describe Target Groups: %v", err)
		}

		for _, tg := range page.TargetGroups {
			if tg.TargetType == "instance" {
				wg.Add(1)
				go func(tgARN string) {
					defer wg.Done()
					err := i.deregisterInstanceFromTargetGroup(tgARN)
					if err != nil {
						log.Printf("failed to deregister instance from Target Group %s: %v", tgARN, err)
					}
				}(*tg.TargetGroupArn)
			}
		}
	}

	wg.Wait()
	return nil
}

func (i *instance) getContainerInstanceArn(clusterName *string) (*string, error) {

	// List the container instances on the cluster
	listInput := &ecs.ListContainerInstancesInput{
		Cluster: clusterName,
	}
	listOutput, err := i.region.services.ecs.ListContainerInstances(context.TODO(), listInput)
	if err != nil {
		return nil, fmt.Errorf("error listing container instances on cluster %s: %v", *clusterName, err)
	}

	if len(listOutput.ContainerInstanceArns) == 0 {
		return nil, errors.New("no container instances found on cluster")
	}

	describeInput := &ecs.DescribeContainerInstancesInput{
		Cluster:            clusterName,
		ContainerInstances: listOutput.ContainerInstanceArns,
	}
	describeOutput, err := i.region.services.ecs.DescribeContainerInstances(context.TODO(), describeInput)
	if err != nil {
		return nil, fmt.Errorf("error describing container instances: %v", err)
	}

	// Iterate over the list of container instances
	for _, instance := range describeOutput.ContainerInstances {
		// Check if the instance ID matches the instance ID of the current instance
		if *instance.Ec2InstanceId == *i.InstanceId {
			return instance.ContainerInstanceArn, nil
		}
	}

	// Return an error if the instance is not found
	return nil, errors.New("instance not found in the list of container instances")
}

func (i *instance) setContainerInstanceToDraining(clusterARN *string, setDrainingGroup *sync.WaitGroup) {
	defer setDrainingGroup.Done()

	// Extract the cluster name from the ARN
	clusterName, err := i.region.extractECSClusterName(aws.StringValue(clusterARN))
	if err != nil {
		log.Printf("Error extracting cluster name from ARN %s: %v\n", aws.StringValue(clusterARN), err)
		return
	}

	containerInstanceArn, err := i.getContainerInstanceArn(clusterARN)
	if err != nil {
		log.Printf("Couldn't find container instance ARN for instance %s in ECS cluster %s: %v\n", aws.StringValue(i.InstanceId), clusterName, err)
		return
	}

	// Set the container instance status to DRAINING
	updateInput := &ecs.UpdateContainerInstancesStateInput{
		Cluster:            aws.String(clusterName),
		ContainerInstances: []string{*containerInstanceArn},
		Status:             "DRAINING",
	}
	_, err = i.region.services.ecs.UpdateContainerInstancesState(context.TODO(), updateInput)
	if err != nil {
		log.Printf("Error setting instance %s to DRAINING in cluster %s: %v\n", aws.StringValue(i.InstanceId), clusterName, err)
	} else {
		log.Printf("Set instance %s to DRAINING in cluster %s\n", aws.StringValue(i.InstanceId), clusterName)
	}
}

func (i *instance) setInstanceToDraining(clusterARNs []string) error {
	// Create a workgroup to perform parallel status update
	setDrainingGroup := new(sync.WaitGroup)
	setDrainingGroup.Add(len(clusterARNs))

	// Set the instance status to DRAINING for each cluster
	for _, clusterARN := range clusterARNs {
		arn := clusterARN
		go i.setContainerInstanceToDraining(&arn, setDrainingGroup)
	}

	setDrainingGroup.Wait()

	return nil
}

func (i *instance) setToDrainingInAllECSClusters() error {

	ecsClient := i.region.services.ecs

	input := &ecs.ListClustersInput{}
	paginator := ecs.NewListClustersPaginator(ecsClient, input)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			return fmt.Errorf("could't list ECS clusters: %s", err)
		}

		if err := i.setInstanceToDraining(page.ClusterArns); err != nil {
			log.Printf("Failed to set instance to draining in clusters %v: %s", page.ClusterArns, err)
		}
	}

	return nil
}
