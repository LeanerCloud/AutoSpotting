// Copyright (c) 2016-2019 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
)

type autoScalingGroup struct {
	*autoscaling.Group

	name                string
	region              *region
	launchConfiguration *launchConfiguration
	instances           instances
	minOnDemand         int64
	config              AutoScalingConfig
}

func (a *autoScalingGroup) loadLaunchConfiguration() (*launchConfiguration, error) {
	//already done
	if a.launchConfiguration != nil {
		return a.launchConfiguration, nil
	}

	lcName := a.LaunchConfigurationName

	if lcName == nil {
		// ASG does not have launch configuration
		return nil, nil
	}

	svc := a.region.services.autoScaling

	params := &autoscaling.DescribeLaunchConfigurationsInput{
		LaunchConfigurationNames: []*string{lcName},
	}
	resp, err := svc.DescribeLaunchConfigurations(params)

	if err != nil {
		logger.Println(err.Error())
		return nil, err
	}

	a.launchConfiguration = &launchConfiguration{
		LaunchConfiguration: resp.LaunchConfigurations[0],
	}
	return a.launchConfiguration, nil
}

func (a *autoScalingGroup) needReplaceOnDemandInstances() bool {
	onDemandRunning, totalRunning := a.alreadyRunningInstanceCount(false, nil)
	if onDemandRunning > a.minOnDemand {
		logger.Println("Currently more than enough OnDemand instances running")
		return true
	}
	if onDemandRunning == a.minOnDemand {
		logger.Println("Currently OnDemand running equals to the required number, skipping run")
		return false
	}
	logger.Println("Currently fewer OnDemand instances than required !")
	if a.allInstancesRunning() && a.instances.count64() >= *a.DesiredCapacity {
		logger.Println("All instances are running and desired capacity is satisfied")
		if randomSpot := a.getAnySpotInstance(); randomSpot != nil {
			if totalRunning == 1 {
				logger.Println("Warning: blocking replacement of very last instance - consider raising ASG to >= 2")
			} else {
				logger.Println("Terminating a random spot instance",
					*randomSpot.Instance.InstanceId)
				switch a.config.TerminationMethod {
				case DetachTerminationMethod:
					randomSpot.terminate()
				default:
					a.terminateInstanceInAutoScalingGroup(randomSpot.Instance.InstanceId)
				}
			}
		}
	}
	return false
}

func (a *autoScalingGroup) allInstancesRunning() bool {
	_, totalRunning := a.alreadyRunningInstanceCount(false, nil)
	return totalRunning == a.instances.count64()
}

func (a *autoScalingGroup) calculateHourlySavings() float64 {
	var savings float64
	for i := range a.instances.instances() {
		savings += (i.typeInfo.pricing.onDemand + i.typeInfo.pricing.premium) - i.price
	}
	return savings
}

func (a *autoScalingGroup) licensedToRun() (bool, error) {
	defer savingsMutex.Unlock()
	savingsMutex.Lock()

	savings := a.calculateHourlySavings()
	hourlySavings += savings

	monthlySavings := hourlySavings * 24 * 30
	if (monthlySavings > 1000) &&
		strings.Contains(a.region.conf.Version, "nightly") &&
		a.region.conf.LicenseType == "evaluation" {
		return false, fmt.Errorf(
			"would reach estimated monthly savings of $%.2f when processing this group, above the $1000 evaluation limit",
			monthlySavings)
	}
	return true, nil
}

func (a *autoScalingGroup) process() {
	var spotInstanceID string
	a.scanInstances()
	a.loadDefaultConfig()
	a.loadConfigFromTags()

	logger.Println("Finding spot instances created for", a.name)

	spotInstance := a.findUnattachedInstanceLaunchedForThisASG()
	debug.Println("Candidate Spot instance", spotInstance)

	shouldRun := cronRunAction(time.Now(), a.config.CronSchedule, a.config.CronTimezone, a.config.CronScheduleState)
	debug.Println(a.region.name, a.name, "Should take replacement actions:", shouldRun)

	if ok, err := a.licensedToRun(); !ok {
		logger.Println(a.region.name, a.name, "Skipping group, license limit reached:", err.Error())
		return
	}

	if spotInstance == nil {
		logger.Println("No spot instances were found for ", a.name)

		onDemandInstance := a.getAnyUnprotectedOnDemandInstance()

		if onDemandInstance == nil {
			logger.Println(a.region.name, a.name,
				"No running unprotected on-demand instances were found, nothing to do here...")
			return
		}

		if !a.needReplaceOnDemandInstances() {
			logger.Println("Not allowed to replace any of the running OD instances in ", a.name)
			return
		}

		if !shouldRun {
			logger.Println(a.region.name, a.name,
				"Skipping run, outside the enabled cron run schedule")
			return
		}

		if _, err := a.loadLaunchConfiguration(); err != nil {
			logger.Printf("Could not launch configuration: %s", err)
		}

		err := onDemandInstance.launchSpotReplacement()
		if err != nil {
			logger.Printf("Could not launch cheapest spot instance: %s", err)
		}
		return
	}

	spotInstanceID = *spotInstance.InstanceId

	if !a.needReplaceOnDemandInstances() || !shouldRun {
		logger.Println("Spot instance", spotInstanceID, "is not need anymore by ASG",
			a.name, "terminating the spot instance.")
		spotInstance.terminate()
		return
	}
	if !spotInstance.isReadyToAttach(a) {
		logger.Println("Waiting for next run while processing", a.name)
		return
	}

	logger.Println(a.region.name, "Found spot instance:", spotInstanceID,
		"Attaching it to", a.name)

	a.replaceOnDemandInstanceWithSpot(spotInstanceID)

}

func (a *autoScalingGroup) scanInstances() instances {

	logger.Println("Adding instances to", a.name)
	a.instances = makeInstances()

	for _, inst := range a.Instances {
		i := a.region.instances.get(*inst.InstanceId)

		debug.Println(i)

		if i == nil {
			continue
		}

		i.asg, i.region = a, a.region
		if inst.ProtectedFromScaleIn != nil {
			i.protected = i.protected || *inst.ProtectedFromScaleIn
		}

		if i.isSpot() {
			i.price = i.typeInfo.pricing.spot[*i.Placement.AvailabilityZone]
		} else {
			i.price = i.typeInfo.pricing.onDemand + i.typeInfo.pricing.premium
		}

		a.instances.add(i)
	}
	return a.instances
}

func (a *autoScalingGroup) replaceOnDemandInstanceWithSpot(
	spotInstanceID string) error {

	// get the details of our spot instance so we can see its AZ
	logger.Println(a.name, "Retrieving instance details for ", spotInstanceID)
	spotInst := a.region.instances.get(spotInstanceID)
	if spotInst == nil {
		return errors.New("couldn't find spot instance to use")
	}
	az := spotInst.Placement.AvailabilityZone

	logger.Println(a.name, spotInstanceID, "is in the availability zone",
		*az, "looking for an on-demand instance there")

	odInst := a.getUnprotectedOnDemandInstanceInAZ(az)

	if odInst == nil {
		logger.Println(a.name, "found no on-demand instances that could be",
			"replaced with the new spot instance", *spotInst.InstanceId,
			"terminating the spot instance.")
		spotInst.terminate()
		return errors.New("couldn't find ondemand instance to replace")
	}
	logger.Println(a.name, "found on-demand instance", *odInst.InstanceId,
		"replacing with new spot instance", *spotInst.InstanceId)

	desiredCapacity, maxSize := *a.DesiredCapacity, *a.MaxSize

	// temporarily increase AutoScaling group in case the desired capacity reaches the max size,
	// otherwise attachSpotInstance might fail
	if desiredCapacity == maxSize {
		logger.Println(a.name, "Temporarily increasing MaxSize")
		a.setAutoScalingMaxSize(maxSize + 1)
		defer a.setAutoScalingMaxSize(maxSize)
	}

	attachErr := a.attachSpotInstance(spotInstanceID)
	if attachErr != nil {
		logger.Println(a.name, "skipping detaching on-demand due to failure to",
			"attach the new spot instance ", *spotInst.InstanceId)
		return nil
	}

	switch a.config.TerminationMethod {
	case DetachTerminationMethod:
		return a.detachAndTerminateOnDemandInstance(odInst.InstanceId)
	default:
		return a.terminateInstanceInAutoScalingGroup(odInst.InstanceId)
	}
}

// Returns the information about the first running instance found in
// the group, while iterating over all instances from the
// group. It can also filter by AZ and Lifecycle.
func (a *autoScalingGroup) getInstance(
	availabilityZone *string,
	onDemand bool,
	considerInstanceProtection bool,
) *instance {

	for i := range a.instances.instances() {

		// instance is running
		if *i.State.Name == ec2.InstanceStateNameRunning {

			// the InstanceLifecycle attribute is non-nil only for spot instances,
			// where it contains the value "spot", if we're looking for on-demand
			// instances only, then we have to skip the current instance.
			if (onDemand && i.isSpot()) || (!onDemand && !i.isSpot()) {
				debug.Println(a.name, "skipping instance", *i.InstanceId,
					"having different lifecycle than what we're looking for")
				continue
			}

			if considerInstanceProtection {
				protected := i.isProtectedFromScaleIn()
				if !protected {
					protectedT, err := i.isProtectedFromTermination()
					if err != nil {
						debug.Println(a.name, "failed to determine termination protection for", *i.InstanceId)
					}
					protected = protectedT
				}

				if protected {
					debug.Println(a.name, "skipping protected instance", *i.InstanceId)
					continue
				}
			}

			if (availabilityZone != nil) && (*availabilityZone != *i.Placement.AvailabilityZone) {
				debug.Println(a.name, "skipping instance", *i.InstanceId,
					"placed in a different AZ than what we're looking for")
				continue
			}
			return i
		}
	}
	return nil
}

func (a *autoScalingGroup) getUnprotectedOnDemandInstanceInAZ(az *string) *instance {
	return a.getInstance(az, true, true)
}
func (a *autoScalingGroup) getAnyUnprotectedOnDemandInstance() *instance {
	return a.getInstance(nil, true, true)
}

func (a *autoScalingGroup) getAnyOnDemandInstance() *instance {
	return a.getInstance(nil, true, false)
}

func (a *autoScalingGroup) getAnySpotInstance() *instance {
	return a.getInstance(nil, false, false)
}

func (a *autoScalingGroup) hasMemberInstance(inst *instance) bool {
	for _, member := range a.Instances {
		if *member.InstanceId == *inst.InstanceId {
			return true
		}
	}
	return false
}

func (a *autoScalingGroup) findUnattachedInstanceLaunchedForThisASG() *instance {
	for inst := range a.region.instances.instances() {
		for _, tag := range inst.Tags {
			if *tag.Key == "launched-for-asg" && *tag.Value == a.name {
				if !a.hasMemberInstance(inst) {
					return inst
				}
			}
		}
	}
	return nil
}

func (a *autoScalingGroup) getAllowedInstanceTypes(baseInstance *instance) []string {
	var allowedInstanceTypesTag string

	// By default take the command line parameter
	allowed := strings.Replace(a.region.conf.AllowedInstanceTypes, " ", ",", -1)

	// Check option of allowed instance types
	// If we have that option we don't need to calculate the compatible instance type.
	if tagValue := a.getTagValue(AllowedInstanceTypesTag); tagValue != nil {
		allowedInstanceTypesTag = strings.Replace(*tagValue, " ", ",", -1)
	}

	// ASG Tag config has a priority to override
	if allowedInstanceTypesTag != "" {
		allowed = allowedInstanceTypesTag
	}

	if allowed == "current" {
		return []string{baseInstance.typeInfo.instanceType}
	}

	// Simple trick to avoid returning list with empty elements
	return strings.FieldsFunc(allowed, func(c rune) bool {
		return c == ','
	})
}

func (a *autoScalingGroup) getDisallowedInstanceTypes(baseInstance *instance) []string {
	var disallowedInstanceTypesTag string

	// By default take the command line parameter
	disallowed := strings.Replace(a.region.conf.DisallowedInstanceTypes, " ", ",", -1)

	// Check option of disallowed instance types
	// If we have that option we don't need to calculate the compatible instance type.
	if tagValue := a.getTagValue(DisallowedInstanceTypesTag); tagValue != nil {
		disallowedInstanceTypesTag = strings.Replace(*tagValue, " ", ",", -1)
	}

	// ASG Tag config has a priority to override
	if disallowedInstanceTypesTag != "" {
		disallowed = disallowedInstanceTypesTag
	}

	// Simple trick to avoid returning list with empty elements
	return strings.FieldsFunc(disallowed, func(c rune) bool {
		return c == ','
	})
}

func (a *autoScalingGroup) setAutoScalingMaxSize(maxSize int64) error {
	svc := a.region.services.autoScaling

	_, err := svc.UpdateAutoScalingGroup(
		&autoscaling.UpdateAutoScalingGroupInput{
			AutoScalingGroupName: aws.String(a.name),
			MaxSize:              aws.Int64(maxSize),
		})

	if err != nil {
		// Print the error, cast err to awserr.Error to get the Code and
		// Message from an error.
		logger.Println(err.Error())
		return err
	}
	return nil
}

func (a *autoScalingGroup) attachSpotInstance(spotInstanceID string) error {

	svc := a.region.services.autoScaling

	params := autoscaling.AttachInstancesInput{
		AutoScalingGroupName: aws.String(a.name),
		InstanceIds: []*string{
			&spotInstanceID,
		},
	}

	resp, err := svc.AttachInstances(&params)

	if err != nil {
		logger.Println(err.Error())
		// Pretty-print the response data.
		logger.Println(resp)
		return err
	}
	return nil
}

// Terminates an on-demand instance from the group,
// but only after it was detached from the autoscaling group
func (a *autoScalingGroup) detachAndTerminateOnDemandInstance(
	instanceID *string) error {
	logger.Println(a.region.name,
		a.name,
		"Detaching and terminating instance:",
		*instanceID)
	// detach the on-demand instance
	detachParams := autoscaling.DetachInstancesInput{
		AutoScalingGroupName: aws.String(a.name),
		InstanceIds: []*string{
			instanceID,
		},
		ShouldDecrementDesiredCapacity: aws.Bool(true),
	}

	asSvc := a.region.services.autoScaling

	if _, err := asSvc.DetachInstances(&detachParams); err != nil {
		logger.Println(err.Error())
		return err
	}

	// Wait till detachment initialize is complete before terminate instance
	time.Sleep(20 * time.Second * a.region.conf.SleepMultiplier)

	return a.instances.get(*instanceID).terminate()
}

// Terminates an on-demand instance from the group using the
// TerminateInstanceInAutoScalingGroup api call.
func (a *autoScalingGroup) terminateInstanceInAutoScalingGroup(
	instanceID *string) error {
	logger.Println(a.region.name,
		a.name,
		"Terminating instance:",
		*instanceID)
	// terminate the on-demand instance
	terminateParams := autoscaling.TerminateInstanceInAutoScalingGroupInput{
		InstanceId:                     instanceID,
		ShouldDecrementDesiredCapacity: aws.Bool(true),
	}

	asSvc := a.region.services.autoScaling
	if _, err := asSvc.TerminateInstanceInAutoScalingGroup(&terminateParams); err != nil {
		logger.Println(err.Error())
		return err
	}

	return nil
}

// Counts the number of already running instances on-demand or spot, in any or a specific AZ.
func (a *autoScalingGroup) alreadyRunningInstanceCount(
	spot bool, availabilityZone *string) (int64, int64) {

	var total, count int64
	instanceCategory := "spot"

	if !spot {
		instanceCategory = "on-demand"
	}
	logger.Println(a.name, "Counting already running on demand instances ")
	for inst := range a.instances.instances() {
		if *inst.Instance.State.Name == "running" {
			// Count running Spot instances
			if spot && inst.isSpot() &&
				(availabilityZone == nil || *inst.Placement.AvailabilityZone == *availabilityZone) {
				count++
				// Count running OnDemand instances
			} else if !spot && !inst.isSpot() &&
				(availabilityZone == nil || *inst.Placement.AvailabilityZone == *availabilityZone) {
				count++
			}
			// Count total running instances
			total++
		}
	}
	logger.Println(a.name, "Found", count, instanceCategory, "instances running on a total of", total)
	return count, total
}
