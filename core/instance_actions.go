// Copyright (c) 2016-2021 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/davecgh/go-spew/spew"
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
	i.price = i.typeInfo.pricing.onDemand / i.region.conf.OnDemandPriceMultiplier * i.asg.config.OnDemandPriceMultiplier
	instanceTypes, err := i.getCompatibleSpotInstanceTypesListSortedAscendingByPrice(
		i.asg.getAllowedInstanceTypes(i),
		i.asg.getDisallowedInstanceTypes(i))

	if err != nil {
		log.Println("Couldn't determine the cheapest compatible spot instance type")
		return nil, err
	}

	//Go through all compatible instances until one type launches or we are out of options.
	for _, instanceType := range instanceTypes {
		az := *i.Placement.AvailabilityZone
		bidPrice := i.getPriceToBid(i.price,
			instanceType.pricing.spot[az], instanceType.pricing.premium)

		runInstancesInput, err := i.createRunInstancesInput(instanceType.instanceType, bidPrice)
		if err != nil {
			log.Println(az, i.asg.name, "Failed to generate run instances input, ", err.Error(), "skipping instance type ", instanceType.instanceType)
			continue
		}

		log.Println(az, i.asg.name, "Launching spot instance of type", instanceType.instanceType, "with bid price", bidPrice)
		log.Println(az, i.asg.name)
		resp, err := i.region.services.ec2.RunInstances(runInstancesInput)

		if err != nil {
			if strings.Contains(err.Error(), "InsufficientInstanceCapacity") {
				log.Println("Couldn't launch spot instance due to lack of capacity, trying next instance type:", err.Error())
			} else {
				log.Println("Couldn't launch spot instance:", err.Error(), "trying next instance type")
				debug.Println(runInstancesInput)
			}
		} else {
			spotInst := resp.Instances[0]
			log.Println(i.asg.name, "Successfully launched spot instance", *spotInst.InstanceId,
				"of type", *spotInst.InstanceType,
				"with bid price", bidPrice,
				"current spot price", instanceType.pricing.spot[az])

			debug.Println("RunInstances response:", spew.Sdump(resp))
			// add to FinalRecap
			recapText := fmt.Sprintf("%s Launched spot instance %s", i.asg.name, *spotInst.InstanceId)
			i.region.conf.FinalRecap[i.region.name] = append(i.region.conf.FinalRecap[i.region.name], recapText)
			return spotInst.InstanceId, nil
		}
	}

	log.Println(i.asg.name, "Exhausted all compatible instance types without launch success. Aborting.")
	return nil, errors.New("exhausted all compatible instance types")

}

func (i *instance) swapWithGroupMember(asg *autoScalingGroup) (*instance, error) {
	odInstanceID := i.getReplacementTargetInstanceID()
	if odInstanceID == nil {
		log.Println("Couldn't find target on-demand instance of", *i.InstanceId)
		return nil, fmt.Errorf("couldn't find target instance for %s", *i.InstanceId)
	}

	if err := i.region.scanInstance(odInstanceID); err != nil {
		log.Printf("Couldn't describe the target on-demand instance %s", *odInstanceID)
		return nil, fmt.Errorf("target instance %s couldn't be described", *odInstanceID)
	}

	odInstance := i.region.instances.get(*odInstanceID)
	if odInstance == nil {
		log.Printf("Target on-demand instance %s couldn't be found", *odInstanceID)
		return nil, fmt.Errorf("target instance %s is missing", *odInstanceID)
	}

	if !odInstance.shouldBeReplacedWithSpot() {
		log.Printf("Target on-demand instance %s shouldn't be replaced", *odInstanceID)
		i.terminate()
		return nil, fmt.Errorf("target instance %s should not be replaced with spot",
			*odInstanceID)
	}

	asg.suspendProcesses()
	defer asg.resumeProcesses()

	desiredCapacity, maxSize := *asg.DesiredCapacity, *asg.MaxSize

	// temporarily increase AutoScaling group in case the desired capacity reaches the max size,
	// otherwise attachSpotInstance might fail
	if desiredCapacity == maxSize {
		log.Println(asg.name, "Temporarily increasing MaxSize")
		asg.setAutoScalingMaxSize(maxSize + 1)
		defer asg.setAutoScalingMaxSize(maxSize)
	}

	log.Printf("Attaching spot instance %s to the group %s",
		*i.InstanceId, asg.name)
	err := asg.attachSpotInstance(*i.InstanceId, true)

	if err != nil {
		log.Printf("Spot instance %s couldn't be attached to the group %s, terminating it...",
			*i.InstanceId, asg.name)
		i.terminate()
		return nil, fmt.Errorf("couldn't attach spot instance %s ", *i.InstanceId)
	}

	log.Printf("Terminating on-demand instance %s from the group %s",
		*odInstanceID, asg.name)
	if err := asg.terminateInstanceInAutoScalingGroup(odInstanceID, true, true); err != nil {
		log.Printf("On-demand instance %s couldn't be terminated, re-trying...",
			*odInstanceID)
		return nil, fmt.Errorf("couldn't terminate on-demand instance %s",
			*odInstanceID)
	}

	return odInstance, nil
}

func (i *instance) terminate() error {
	var err error
	log.Printf("Instance: %v\n", i)

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
