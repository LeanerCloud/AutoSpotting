// Copyright (c) 2016-2022 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/elasticbeanstalk"
)

// instance_actions.go contains functions that act on instances, altering their state.

func (i *instance) handleInstanceStates() (bool, error) {
	log.Printf("%s Found instance %s in state %s",
		i.region.name, *i.InstanceId, *i.State.Name)

	if *i.State.Name != "running" {
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

func (i *instance) launchReplacement(replacementLifecycle string) (*string, error) {

	ltData, err := i.createLaunchTemplateData(replacementLifecycle)

	debug.Printf("Launch template data: %+#v", ltData)

	if err != nil {
		log.Println("failed to create LaunchTemplate data,", err.Error())
		return nil, err
	}

	lt, err := i.createFleetLaunchTemplate(ltData)

	debug.Printf("Fleet Launch Template: %+#v", lt)

	if err != nil {
		log.Println(i.region, i.asg.name, "createFleetLaunchTemplate() failure:", err.Error())
		return nil, err
	}

	defer i.deleteLaunchTemplate(lt)
	instanceTypes, err := i.getCompatibleSpotInstanceTypesList(
		i.asg.config.PrioritizedInstanceTypesBias,
		i.asg.config.AllowedInstanceTypes,
		i.asg.config.DisallowedInstanceTypes)

	if err != nil {
		log.Println("Couldn't determine the list of compatible spot instance types")
		return nil, err
	}

	cfi := i.createFleetInput(lt, instanceTypes, replacementLifecycle)

	debug.Printf("Fleet Input: %+#v", cfi)

	resp, err := i.region.services.ec2.CreateFleet(cfi)

	if err != nil {
		log.Println(i.region, i.asg.name, "CreateFleet() failure:", err.Error())
		return nil, err
	}

	log.Printf("CreateFleet() response: %#v", resp)

	if resp != nil && len(resp.Instances) > 0 && resp.Instances[0] != nil && len(resp.Instances[0].InstanceIds) > 0 {

		return resp.Instances[0].InstanceIds[0], nil
	}

	if resp.Errors != nil {
		errors := getInstanceTypesAndErrorCodes(resp.Errors)
		return nil, fmt.Errorf("couldn't launch spot instance, encountered errors: %#v", errors)
	}

	return nil, fmt.Errorf("couldn't launch spot instance because of unknown errors (this code should never be reached)")
}

func getInstanceTypesAndErrorCodes(errors []*ec2.CreateFleetError) map[string]string {
	instanceTypesAndErrorCodes := make(map[string]string)
	for _, e := range errors {
		instanceTypesAndErrorCodes[*e.LaunchTemplateAndOverrides.Overrides.InstanceType] = *e.ErrorCode
	}
	return instanceTypesAndErrorCodes
}

func (i *instance) swapWithGroupMember(asg *autoScalingGroup, instancetoReplace *instance) (*instance, error) {
	var odInstance *instance
	var err error

	if instancetoReplace != nil {
		odInstance = instancetoReplace
	} else {
		odInstance, err = i.getSwapCandidate()
		if err != nil {
			log.Printf("Couldn't find suitable OnDemand swap candidate, terminating Spot instance: %s", *i.InstanceId)
			i.terminate()
			return nil, fmt.Errorf("couldn't find suitable OnDemand swap candidate, error: %w", err)
		}
	}
	asg.suspendProcesses()
	defer asg.resumeProcesses(2 * time.Minute)

	desiredCapacity, maxSize, minSize := *asg.DesiredCapacity, *asg.MaxSize, *asg.MinSize

	// temporarily increase AutoScaling group in case the desired capacity reaches the max size,
	// otherwise attachSpotInstance might fail
	if minSize == maxSize {
		log.Printf("%s: Temporarily increasing MaxSize", asg.name)
		asg.setAutoScalingMaxSize(maxSize + 1)
		defer asg.setAutoScalingMaxSize(maxSize)
	}

	// attach the spot instance first and then terminate the on demand instance,
	//to avoid decreasing the capacity of the group below the mimimum size
	if desiredCapacity == minSize {
		log.Printf("Attaching %s instance %s to the group %s", *i.InstanceLifecycle,
			*i.InstanceId, asg.name)
		if err := attachSpotInstance(i, asg); err != nil {
			return nil, fmt.Errorf("couldn't attach spot instance: %w", err)
		}

		if err := i.terminateInstanceInAutoScalingGroup(asg); err != nil {
			return nil, fmt.Errorf("couldn't terminate on-demand instance: %w", err)
		}

		log.Printf("%s: Terminated on-demand instance %s", asg.name, *odInstance.InstanceId)
		return odInstance, nil

	} else if desiredCapacity > minSize {
		// we can terminate on-demand instance first and then attach the spot instance.
		if err := i.terminateInstanceInAutoScalingGroup(asg); err != nil {
			return nil, fmt.Errorf("couldn't terminate on-demand instance: %w", err)
		}

		log.Printf("%s: Terminated on-demand instance %s", asg.name, *odInstance.InstanceId)

		if err := attachSpotInstance(i, asg); err != nil {
			return nil, fmt.Errorf("couldn't attach spot instance: %w", err)
		}

		log.Printf("%s: Attached spot instance %s", asg.name, *i.InstanceId)
		return odInstance, nil
	}

	return nil, fmt.Errorf("%s: desiredCapacity is less than minSize, this shouldn't happen", asg.name)
}

func attachSpotInstance(i *instance, asg *autoScalingGroup) error {
	log.Printf("%s: Attaching spot instance %s", asg.name, *i.InstanceId)
	err := asg.attachSpotInstance(*i.InstanceId, true)
	if err != nil {
		log.Printf("%s: Spot instance %s couldn't be attached to the group, terminating it...", asg.name, *i.InstanceId)
		i.terminate()
		return fmt.Errorf("couldn't attach spot instance %s", *i.InstanceId)
	}
	return nil
}

func (i *instance) terminateInstanceInAutoScalingGroup(asg *autoScalingGroup) error {
	log.Printf("Terminating instance %s from the group %s",
		*i.InstanceId, asg.name)
	if err := asg.terminateInstanceInAutoScalingGroup(i, true, true); err != nil {
		log.Printf("%s: On-demand instance %s couldn't be terminated, re-trying...", asg.name, *i.InstanceId)
		return fmt.Errorf("couldn't terminate on-demand instance %s: %w", *i.InstanceId, err)
	}
	return nil
}

func (i *instance) getSwapCandidate() (*instance, error) {
	instanceToReplaceID := i.getReplacementTargetInstanceID()
	if instanceToReplaceID == nil {
		log.Println("Couldn't find target instance of", *i.InstanceId)
		return nil, fmt.Errorf("couldn't find target instance for %s", *i.InstanceId)
	}

	if err := i.region.scanInstance(instanceToReplaceID); err != nil {
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
		i.terminate()
		return nil, fmt.Errorf("target instance %s should not be replaced with a new Spot instance",
			*instanceToReplaceID)
	}
	return instanceToReplace, nil
}

func (i *instance) terminate() error {
	var err error
	debug.Printf("Instance: %v\n", i)

	log.Printf("Terminating %v", *i.InstanceId)
	svc := i.region.services.ec2

	if !i.canTerminate() {
		log.Printf("Can't terminate %v, current state: %s",
			*i.InstanceId, *i.State.Name)
		return fmt.Errorf("can't terminate %s", *i.InstanceId)
	}

	_, err = svc.TerminateInstances(&ec2.TerminateInstancesInput{
		InstanceIds: []*string{i.InstanceId},
	})

	if err != nil {
		log.Printf("Issue while terminating %v: %v", *i.InstanceId, err.Error())
	}

	return err
}

func (i *instance) deleteLaunchTemplate(ltName *string) {
	_, err := i.region.services.ec2.DeleteLaunchTemplate(&ec2.DeleteLaunchTemplateInput{
		LaunchTemplateName: ltName,
	})

	if err != nil {
		log.Printf("Issue while deleting launch template %v, error: %v", *ltName, err.Error())
	}
}

func (i *instance) waitForBeanstalkDeployments() {
	envName, envID := i.getBesnstalkDeploymentInformation()
	if envName == nil || envID == nil {
		log.Printf("Instance %s is not provisioned by ElasticBeanstalk, skipping deployment wait logic", *i.InstanceId)
		return
	}

	log.Printf("Instance %s is provisioned by Beanstalk for the environment %s with ID %s, waiting for any ongoing deployments to complete",
		*i.InstanceId, *envName, *envID)
	i.region.services.beanstalk.WaitUntilEnvironmentUpdated(&elasticbeanstalk.DescribeEnvironmentsInput{
		EnvironmentNames: []*string{envName},
		EnvironmentIds:   []*string{envID},
	})
	log.Printf("Instance %s provisioned by Beanstalk for the environment %s with ID %s, wait done",
		*i.InstanceId, *envName, *envID)
}
