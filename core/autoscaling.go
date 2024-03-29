// Copyright (c) 2016-2022 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	autoscalingTypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"

	"github.com/aws/aws-sdk-go-v2/service/codedeploy"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	beanstalkTypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"

	"github.com/aws/aws-sdk-go/aws"
)

type autoScalingGroup struct {
	*autoscalingTypes.AutoScalingGroup

	name                string
	region              *region
	launchConfiguration *launchConfiguration
	launchTemplate      *launchTemplate
	autospotting        *AutoSpotting
	instances           instances
	config              AutoScalingConfig
	spotProduct         *string
}

func (a *autoScalingGroup) loadLaunchConfiguration() (*launchConfiguration, error) {
	//already done
	if a.launchConfiguration != nil {
		return a.launchConfiguration, nil
	}

	lcName := a.LaunchConfigurationName

	if lcName == nil {
		return nil, errors.New("missing launch configuration")
	}

	svc := a.region.services.autoScaling

	params := &autoscaling.DescribeLaunchConfigurationsInput{
		LaunchConfigurationNames: []string{*lcName},
	}
	resp, err := svc.DescribeLaunchConfigurations(context.TODO(), params)

	if err != nil {
		log.Println(err.Error())
		return nil, err
	}

	a.launchConfiguration = &launchConfiguration{
		LaunchConfiguration: &resp.LaunchConfigurations[0],
	}

	return a.launchConfiguration, nil
}

func (a *autoScalingGroup) loadLaunchTemplate() (*launchTemplate, error) {
	//already done
	if a.launchTemplate != nil {
		return a.launchTemplate, nil
	}

	lt := a.LaunchTemplate

	if lt == nil {
		return nil, errors.New("missing launch template")
	}

	ltID := lt.LaunchTemplateId
	ltVer := lt.Version

	if ltID == nil || ltVer == nil {
		return nil, errors.New("missing launch template")
	}

	svc := a.region.services.ec2

	params := &ec2.DescribeLaunchTemplateVersionsInput{
		LaunchTemplateId: ltID,
		Versions:         []string{*ltVer},
	}

	resp, err := svc.DescribeLaunchTemplateVersions(context.TODO(), params)

	if err != nil {
		log.Println(err.Error())
		return nil, err
	}

	if len(resp.LaunchTemplateVersions) == 0 {
		return nil, errors.New("missing launch template")
	}

	ltv := resp.LaunchTemplateVersions[0]

	params2 := &ec2.DescribeImagesInput{
		ImageIds: []string{*ltv.LaunchTemplateData.ImageId},
	}

	resp2, err2 := svc.DescribeImages(context.TODO(), params2)

	if err2 != nil {
		log.Println(err2.Error())
		return nil, err2
	}

	if len(resp2.Images) == 0 {
		return nil, errors.New("missing launch template image")
	}

	a.launchTemplate = &launchTemplate{
		LaunchTemplateVersion: &ltv,
		Image:                 &resp2.Images[0],
	}
	return a.launchTemplate, nil
}

func (a *autoScalingGroup) needReplaceOnDemandInstances() (bool, int32) {
	onDemandRunning, totalRunning := a.alreadyRunningInstanceCount(false, nil)
	debug.Printf("onDemandRunning=%v totalRunning=%v a.minOnDemand=%v",
		onDemandRunning, totalRunning, a.config.MinOnDemand)

	if totalRunning == 0 {
		log.Printf("The group %s is currently empty or in the process of launching new instances",
			a.name)
		return true, totalRunning
	}

	if onDemandRunning > a.config.MinOnDemand {
		log.Println("Currently more than enough OnDemand instances running")
		return true, totalRunning
	}

	if onDemandRunning == a.config.MinOnDemand {
		log.Println("Currently OnDemand running equals to the required number, skipping run")
		return false, totalRunning
	}
	log.Println("Currently fewer OnDemand instances than required !")
	return false, totalRunning
}

// Error for an invalid instance
var errInvalidInstance = errors.New("invalid instance")

// Check if instance is valid
func isValidInstance(instance autoscalingTypes.Instance) error {
	if instance.AvailabilityZone == nil || instance.InstanceType == nil {
		return errInvalidInstance
	}
	return nil
}

// Count Spot instances of a given instance type in an availability zone, and also return the total of Spot instances in that AZ
func (a *autoScalingGroup) countSpotInstancesInAZ(instanceType, az string) (int64, int64, error) {
	var countByType, countSpot int64
	for _, instance := range a.AutoScalingGroup.Instances {
		if err := isValidInstance(instance); err != nil {
			return 0, 0, err
		}

		inst := a.region.instances.get(*instance.InstanceId)
		if inst == nil {
			if err := a.region.scanInstance(*instance.InstanceId); err != nil {
				log.Printf("Failed to scan instance %s in region %s", *instance.InstanceId, a.region.name)
			}
			inst = a.region.instances.get(*instance.InstanceId)
		}
		if inst == nil {
			return 0, 0, fmt.Errorf("instance not found: %s", *instance.InstanceId)
		}

		if !inst.isSpot() {
			debug.Printf("Ignoring On Demand instance %s in availability zone %s", *instance.InstanceId, az)
			continue
		}

		if *instance.AvailabilityZone == az {
			countSpot++
			if *instance.InstanceType == instanceType {
				countByType++
			}
		}
	}
	return countByType, countSpot, nil
}

// Error for an invalid auto scaling group or group
var errInvalidGroup = errors.New("invalid autoScalingGroup or Group is nil")

// Check if adding an instance will cause instance type imbalance
func (a *autoScalingGroup) willCauseInstanceTypeImbalance(instanceType, az string) (bool, error) {
	if a == nil || a.AutoScalingGroup == nil {
		return false, errInvalidGroup
	}

	log.Printf("Starting process: Checking if there are too many instances of type %s in availability zone %s\n", instanceType, az)

	totalSpotCountByType, totalSpotCount, err := a.countSpotInstancesInAZ(instanceType, az)
	if err != nil {
		return false, err
	}
	futureCountOfSimilarSpotInstancesInAZ := totalSpotCountByType + 1
	futureTotalSpotCountInAz := totalSpotCount + 1

	// make sure diversification is at least 1 to avoid division by zero
	diversificationTarget := int64(max(int(a.config.NumberOfInstanceTypesPerAZ), 1))

	// we should allow an additional instance: for
	// example if we have existing instances of types A,B and C and we want to add
	// an instance of type A, it should be allowed. But if we have A,A,B and C and we
	// try to add an A it should not be allowed.
	allowedInstancesOfInstanceTypeInAZ := int64(math.Ceil(float64(futureTotalSpotCountInAz) / float64(diversificationTarget)))

	if futureCountOfSimilarSpotInstancesInAZ > allowedInstancesOfInstanceTypeInAZ {
		log.Printf("After adding new instance, type %s in availability zone %s would be imbalanced\n", instanceType, az)
		return true, nil
	}

	debug.Printf("After adding new instance, type %s in availability zone %s would not be imbalanced\n", instanceType, az)
	return false, nil
}
func (a *autoScalingGroup) cronEventAction() runer {

	log.Println(a.region.name, a.name, "Starting cronEventAction")

	a.EnsureCapacityRebalancingDisabled()

	a.scanInstances()
	debug.Println(a.region.name, a.name, "Completed scanInstances")

	a.loadDefaultConfig()
	debug.Println(a.region.name, a.name, "Completed loadDefaultConfig")

	a.loadConfigFromTags()
	debug.Println(a.region.name, a.name, "Completed loadConfigFromTags")

	shouldRun := cronRunAction(time.Now(), a.config.CronSchedule, a.config.CronTimezone, a.config.CronScheduleState)
	debug.Println(a.region.name, a.name, "CronRunAction evaluated:", shouldRun)

	if !shouldRun {
		log.Println(a.region.name, a.name,
			"Skipping run, outside the enabled cron run schedule")
		return skipRun{reason: "outside-cron-schedule"}
	}

	debug.Printf("ASG %s has following instances: %#v", a.name, a.Instances)

	a.clearScaleInProtectionOfSpotInstancesInDownsizedASGs()
	debug.Println(a.region.name, a.name, "Completed clearScaleInProtectionOfSpotInstancesInDownsizedASGs")

	onDemandInstance := a.getInstance(nil, true)
	if onDemandInstance != nil && onDemandInstance.InstanceId != nil {
		debug.Println(a.region.name, a.name, "onDemandInstance acquired: ", *onDemandInstance.InstanceId)
	}
	if onDemandInstance == nil {
		log.Println(a.region.name, a.name,
			"No running unprotected on-demand instances were found, nothing to do here...")
		return skipRun{reason: "no-instances-to-replace"}
	}

	onDemandInstance.typeInfo = a.region.instanceTypeInformation[string(onDemandInstance.InstanceType)]
	onDemandInstance.initialPrice = onDemandInstance.getODPrice()

	debug.Printf("Instance data: %v", onDemandInstance)

	if a.MixedInstancesPolicy != nil {
		log.Printf("Group %s is using a mixed instances policy", a.name)
	}

	if need, total := a.needReplaceOnDemandInstances(); !need {
		log.Printf("Not allowed to replace any more of the running OD instances in %s, currently running %d on-demand instances", a.name, total)
		return skipRun{reason: "not-allowed-to-replace-more-instances"}
	}

	recapText := fmt.Sprintf("%s Triggered replacement for on-demand instance %s", a.name, *onDemandInstance.Instance.InstanceId)
	a.region.conf.FinalRecap[a.region.name] = append(a.region.conf.FinalRecap[a.region.name], recapText)

	log.Printf("%s CronEvent: returning action to replace on demand instance %s in asg %s",
		a.region.name, *onDemandInstance.InstanceId, a.name)
	debug.Println(a.region.name, a.name, "Exiting cronEventAction")
	return replaceAndTerminateInstance{target{
		autospotting:     a.autospotting,
		onDemandInstance: onDemandInstance,
	},
	}
}

func (a *autoScalingGroup) clearScaleInProtectionOfSpotInstancesInDownsizedASGs() {
	count := int32(a.instances.count())
	capacityDelta := count - *a.DesiredCapacity
	if capacityDelta <= 0 {
		return
	}

	instancesToUnprotect := a.getInstancesToUnprotect()

	for _, instanceID := range instancesToUnprotect {
		err := a.removeScaleInProtection(instanceID)
		if err != nil {
			log.Printf("Error removing scale in protection for instance %s: %s", instanceID, err.Error())
		} else {
			log.Println("Successfully removed scale in protection for instance:", instanceID)
		}
	}

}

func (a *autoScalingGroup) EnsureCapacityRebalancingDisabled() {

	if a.CapacityRebalance == nil || !(*a.CapacityRebalance) {
		log.Printf("Spot capacity rebalancing is already disabled for ASG %s", a.name)
		return
	}

	input := &autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: aws.String(a.name),
		CapacityRebalance:    aws.Bool(false),
	}

	_, err := a.region.services.autoScaling.UpdateAutoScalingGroup(context.Background(), input)
	if err != nil {
		log.Printf("Failed to update Auto Scaling group %s: %v", a.name, err)
	}

	log.Printf("Successfully disabled Spot capacity rebalancing for ASG %s", a.name)
}

func (a *autoScalingGroup) getInstancesToUnprotect() []string {
	extraInstancesCount := a.instances.count() - int(*a.DesiredCapacity)

	azInstancesMap := make(map[string][]autoscalingTypes.Instance)
	for _, instance := range a.Instances {
		azInstancesMap[*instance.AvailabilityZone] = append(azInstancesMap[*instance.AvailabilityZone], instance)
	}

	azs := make([]string, 0, len(azInstancesMap))
	for az := range azInstancesMap {
		azs = append(azs, az)
	}

	sort.Slice(azs, func(i, j int) bool {
		return len(azInstancesMap[azs[i]]) > len(azInstancesMap[azs[j]])
	})

	unprotectedInstanceIDs := make([]string, 0, extraInstancesCount)
	for _, az := range azs {
		azInstances := azInstancesMap[az]
		if len(azInstances) <= extraInstancesCount {
			for _, instance := range azInstances {
				unprotectedInstanceIDs = append(unprotectedInstanceIDs, *instance.InstanceId)
			}
			extraInstancesCount -= len(azInstances)
		} else {
			for _, instance := range azInstances[:extraInstancesCount] {
				unprotectedInstanceIDs = append(unprotectedInstanceIDs, *instance.InstanceId)
			}
			break
		}
	}

	return unprotectedInstanceIDs
}

func (a *autoScalingGroup) scanInstances() instances {

	log.Println("Adding instances to", a.name)
	a.instances = makeInstances()
	for _, inst := range a.Instances {
		debug.Printf("Adding instance %s to ASG %s with ASG ID %s",
			*inst.InstanceId, a.name, *a.AutoScalingGroupName)

		i := a.region.instances.get(*inst.InstanceId)

		if i == nil {
			debug.Println("Missing instance data for ", *inst.InstanceId, "scanning it again")
			if err := a.region.scanInstance(*inst.InstanceId); err != nil {
				log.Printf("Failed to scan instance data for %s in %s ", *inst.InstanceId, a.region.name)
			}

			i = a.region.instances.get(*inst.InstanceId)
			if i == nil {
				debug.Println("Failed to scan instance", *inst.InstanceId)
				continue
			}
		}

		i.asg, i.region = a, a.region
		if inst.ProtectedFromScaleIn != nil {
			i.protected = i.protected || *inst.ProtectedFromScaleIn
		}

		spotProduct := a.getSpotProduct()

		if spotProduct == nil {
			log.Printf("Couldn't find spot product from the ASG when processing instance %s, attempting to get it from its AMI", *inst.InstanceId)
			spotProduct = i.getSpotProductFromAMI()
			if spotProduct == nil {
				log.Printf("Couldn't find spot product from the ASG when processing instance %s", *inst.InstanceId)
				continue
			}
		}

		if i.typeInfo == nil || i.typeInfo.pricing == nil {
			log.Printf("Missing type information or pricing for instance %s of instance type %s, continuing...",
				*inst.InstanceId, i.InstanceType)
			continue
		}

		if i.isSpot() {
			log.Printf("Setting initial price for instance %s as Spot", *inst.InstanceId)
			i.initialPrice = i.typeInfo.pricing[*spotProduct].spot[*i.Placement.AvailabilityZone]
		} else {
			log.Printf("Setting initial price for instance %s as OnDemand for %s ", *inst.InstanceId, *spotProduct)
			i.initialPrice = i.typeInfo.pricing[*spotProduct].onDemand
		}

		// Avoid adding instance in Terminating (Wait|Proceed) Lifecycle State
		if strings.HasPrefix(string(inst.LifecycleState), "Terminating") {
			continue
		}

		a.instances.add(i)
	}
	return a.instances
}

// Returns the information about the first running instance found in
// the group, while iterating over all instances from the
// group. It can also filter by AZ and Lifecycle.
func (a *autoScalingGroup) getInstance(
	availabilityZone *string,
	onDemand bool,
) *instance {

	for i := range a.instances.instances() {

		found := false
		for _, in := range a.Instances {
			debug.Printf("checking if ASG instance %s exists as instance ID %s in asg %s",
				*in.InstanceId, *in.InstanceId, a.name)
			if *in.InstanceId == *i.InstanceId {
				found = true
			}
		}
		if !found {
			log.Printf("Instance %s not found in the ASG %s",
				*i.InstanceId, a.name)
			continue
		}

		// instance is running
		if i.State.Name == ec2Types.InstanceStateNameRunning {

			// the InstanceLifecycle attribute is non-nil only for spot instances,
			// where it contains the value "spot", if we're looking for on-demand
			// instances only, then we have to skip the current instance.
			if (onDemand && i.isSpot()) || (!onDemand && !i.isSpot()) {
				debug.Println(a.name, "skipping instance", *i.InstanceId,
					"having different lifecycle than what we're looking for")
				continue
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

func (a *autoScalingGroup) hasMemberInstance(inst *instance) bool {
	for _, member := range a.Instances {
		if *member.InstanceId == *inst.InstanceId {
			return true
		}
	}
	return false
}

func (a *autoScalingGroup) waitForInstanceStatus(instanceID *string, status, lifecycle string, maxRetry int) error {
	isInstanceInStatus := false
	for retry := 0; !isInstanceInStatus; retry++ {
		if retry > maxRetry {
			log.Printf("Failed waiting instance %s in status %s",
				*instanceID, status)
			break
		} else {
			result, err := a.region.services.autoScaling.DescribeAutoScalingInstances(
				context.TODO(),
				&autoscaling.DescribeAutoScalingInstancesInput{
					InstanceIds: []string{*instanceID},
				})

			if err != nil {
				log.Println(err.Error())
				continue
			}

			autoScalingInstances := result.AutoScalingInstances

			if len(autoScalingInstances) > 0 {
				if instanceStatus := *autoScalingInstances[0].LifecycleState; instanceStatus != status {
					log.Printf("Waiting for new %s instance %s to be in status %s [%s]",
						lifecycle, *instanceID, status, instanceStatus)
					if instanceStatus == "Terminating" {
						log.Printf("%s instance %s is terminating, no need to wait further", lifecycle, *instanceID)
						return nil
					}
				} else {
					isInstanceInStatus = true
					return nil
				}
			} else {
				log.Printf("Waiting for %s instance %s to be in AutoScalingGroup with status %s",
					lifecycle, *instanceID, status)
			}

			time.Sleep(time.Duration(10) * time.Second)
		}
	}

	return errors.New("")
}

func (a *autoScalingGroup) setAutoScalingMaxSize(maxSize int32) error {
	svc := a.region.services.autoScaling

	_, err := svc.UpdateAutoScalingGroup(context.TODO(),
		&autoscaling.UpdateAutoScalingGroupInput{
			AutoScalingGroupName: aws.String(a.name),
			MaxSize:              aws.Int32(maxSize),
		})

	if err != nil {
		// Print the error, cast err to aws.APIError to get the Code and
		// Message from an error.
		log.Println(err.Error())
		return err
	}
	return nil
}

func (a *autoScalingGroup) attachNewInstance(newInstanceID string, lifecycle string, wait bool) error {

	if wait {
		log.Printf("Waiting for %s instance %s to start", lifecycle, newInstanceID)
		if err := a.region.waitForInstanceRunning(&newInstanceID); err != nil {
			log.Printf("Issue while waiting for %s instance %s to start: %v",
				lifecycle, newInstanceID, err.Error())
		}
	}

	log.Printf("Attaching %s instance %s to ASG %v", lifecycle, newInstanceID, a.name)
	resp, err := a.region.services.autoScaling.AttachInstances(
		context.TODO(),
		&autoscaling.AttachInstancesInput{
			AutoScalingGroupName: aws.String(a.name),
			InstanceIds: []string{
				newInstanceID,
			},
		},
	)

	if err != nil && !strings.Contains(err.Error(), "is already part of AutoScalingGroup") {
		log.Println(err.Error())
		// Pretty-print the response data.
		log.Println(resp)
		return err
	}
	log.Printf("Waiting for %s instance %s to become in service", lifecycle, newInstanceID)
	if err := a.waitForInstanceStatus(&newInstanceID, "InService", lifecycle, 80); err != nil {
		log.Printf("new %s instance %s couldn't be attached to the group %s: %v",
			lifecycle, newInstanceID, a.name, err.Error())
		return err
	}

	if a.MixedInstancesPolicy != nil {
		if err := a.protectFromScaleIn(newInstanceID); err != nil {
			log.Printf("%s instance %s couldn't be protected from scale-in: %v",
				lifecycle, newInstanceID, err.Error())
			return err
		}
	}

	if hasLH, hook := a.hasCodeDeployLifecycleHook(); hasLH {
		if err := a.triggerCodeDeployDeployment(hook); err != nil {
			log.Printf("Cannot trigger CodeDeploy deployment for instance %s", newInstanceID)
		}
	}

	return nil
}

func (a *autoScalingGroup) protectFromScaleIn(instanceID string) error {
	return a.setInstanceScaleInProtection(instanceID, true)
}
func (a *autoScalingGroup) removeScaleInProtection(instanceID string) error {
	return a.setInstanceScaleInProtection(instanceID, false)
}

func (a *autoScalingGroup) setInstanceScaleInProtection(instanceID string, status bool) error {
	protectionInput := &autoscaling.SetInstanceProtectionInput{
		AutoScalingGroupName: a.AutoScalingGroupName,
		InstanceIds:          []string{instanceID},
		ProtectedFromScaleIn: aws.Bool(status),
	}

	_, err := a.region.services.autoScaling.SetInstanceProtection(context.TODO(), protectionInput)
	if err != nil {
		return fmt.Errorf("unable to set instance protection: %v", err)
	}
	return nil
}

// Terminates an instance from the group using the
// TerminateInstanceInAutoScalingGroup api call.
func (a *autoScalingGroup) terminateInstanceInAutoScalingGroup(
	i *instance, wait bool, decreaseCapacity bool) error {
	lifecycle := i.getLifecycleValue()

	var found bool
	for _, instance := range a.Instances {
		if *instance.InstanceId == *i.InstanceId {
			found = true
		}
	}
	if !found {
		log.Printf("%s Not terminating %s instance: %s because it is not in the ASG %s",
			a.region.name, lifecycle, *i.InstanceId, a.name)
		return nil
	}

	//return if we can't terminate the instance
	if !i.canTerminate() {
		log.Printf("%s %s Not terminating %s instance: %s because it is already terminated or terminating\n",
			a.region.name, a.name, lifecycle, *i.InstanceId)
		return nil
	}

	if wait {

		if err := a.region.waitForInstanceRunning(i.InstanceId); err != nil {
			log.Printf("Issue while waiting for %s instance %v to start: %v",
				lifecycle, *i.InstanceId, err.Error())
		}

		if err := a.waitForInstanceStatus(i.InstanceId, "InService", lifecycle, 5); err != nil {
			log.Printf("Instance %s is still not InService, trying to terminate it anyway.",
				*i.InstanceId)
		}
	}

	a.deregisterInstanceFromLoadBalancersAndECSClusters(i)

	log.Printf("%s %s, terminating %s instance %s:",
		a.region.name, a.name, lifecycle, *i.InstanceId)

	asSvc := a.region.services.autoScaling

	resDLH, err := asSvc.DescribeLifecycleHooks(
		context.TODO(),
		&autoscaling.DescribeLifecycleHooksInput{
			AutoScalingGroupName: a.AutoScalingGroupName,
		})

	if err != nil {
		log.Println(err.Error())
		return err
	}

	for _, hook := range resDLH.LifecycleHooks {
		_, err := asSvc.CompleteLifecycleAction(
			context.TODO(),
			&autoscaling.CompleteLifecycleActionInput{
				AutoScalingGroupName:  a.AutoScalingGroupName,
				InstanceId:            i.InstanceId,
				LifecycleHookName:     hook.LifecycleHookName,
				LifecycleActionResult: aws.String("ABANDON"),
			})
		if err != nil {
			log.Printf("Could not complete lifecycle hook action for ASG %s : %s", a.name, err.Error())
		}
	}

	resTIIASG, err := asSvc.TerminateInstanceInAutoScalingGroup(
		context.TODO(),
		&autoscaling.TerminateInstanceInAutoScalingGroupInput{
			InstanceId:                     i.InstanceId,
			ShouldDecrementDesiredCapacity: aws.Bool(decreaseCapacity),
		})

	if err != nil {
		log.Println(err.Error())
		return err
	}

	if resTIIASG != nil && resTIIASG.Activity != nil && resTIIASG.Activity.Description != nil {
		log.Println(*resTIIASG.Activity.Description)
	}

	return nil
}

func (a *autoScalingGroup) deregisterInstanceFromLoadBalancersAndECSClusters(i *instance) {
	// Create wait group
	var wg sync.WaitGroup
	lifecycle := i.getLifecycleValue()

	if a.LoadBalancerNames != nil {
		for _, alb := range a.LoadBalancerNames {
			wg.Add(1)
			go func(alb string) {
				defer wg.Done()
				log.Printf("%s %s Deregistering %s instance: %s from load balancer %s",
					a.region.name, a.name, lifecycle, *i.InstanceId, alb)
				err := i.deregisterInstanceFromClasicLoadBalancer(alb)
				if err != nil {
					log.Printf("failed to deregister instance: %v", err)
				}
			}(alb)
		}
	}

	if a.TargetGroupARNs != nil {
		for _, tg := range a.TargetGroupARNs {
			wg.Add(1)
			go func(tg string) {
				defer wg.Done()
				log.Printf("%s %s Deregistering %s instance: %s from target group %s", a.region.name, a.name, lifecycle, *i.InstanceId, tg)
				err := i.deregisterInstanceFromTargetGroup(tg)
				if err != nil {
					log.Printf("failed to deregister %s instance %s: %v", lifecycle, *i.InstanceId, err)
				}
			}(tg)
		}
	}

	if i.isManagedByECSCapacityProvider() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			log.Printf("%s %s Deregistering ECS managed %s instance %s from all Target groups", a.region.name, a.name, lifecycle, *i.InstanceId)
			err := i.deregisterInstanceFromAllTargetGroups()
			if err != nil {
				log.Printf("failed to deregister %s instance %s: %v", lifecycle, *i.InstanceId, err)
			}
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			log.Printf("%s %s Deregistering ECS managed %s instance %s from all ECS clusters", a.region.name, a.name, lifecycle, *i.InstanceId)
			err := i.setToDrainingInAllECSClusters()
			if err != nil {
				log.Printf("failed to deregister %s instance %s: %v", lifecycle, *i.InstanceId, err)
			}
		}()
	}

	log.Println("Waiting for all deregistration actions to complete")
	wg.Wait()
}

// Counts the number of already running instances on-demand or spot, in any or a specific AZ.
func (a *autoScalingGroup) alreadyRunningInstanceCount(
	spot bool, availabilityZone *string) (int32, int32) {

	var total, count int32
	instanceCategory := Spot

	if a == nil {
		log.Println("AutoScalingGroup is nil, this shouldn't happen")
		return 0, 0
	}

	if !spot {
		instanceCategory = OnDemand
	}
	log.Println(a.name, "Counting already running", instanceCategory, "instances")
	for inst := range a.instances.instances() {

		if inst.Instance.State.Name == "running" {
			// Count total running instances
			total++
			if availabilityZone == nil || *inst.Placement.AvailabilityZone == *availabilityZone {
				if (spot && inst.isSpot()) || (!spot && !inst.isSpot()) {
					count++
				}
			}
		}
	}
	log.Println(a.name, "Found", count, instanceCategory, "instances running on a total of", total)
	return count, total
}
func (a *autoScalingGroup) suspendProcesses(processes []string) {

	log.Printf("Suspending processes on ASG %s", a.name)

	_, err := a.region.services.autoScaling.SuspendProcesses(
		context.TODO(),
		&autoscaling.SuspendProcessesInput{
			AutoScalingGroupName: a.AutoScalingGroupName,
			ScalingProcesses:     processes,
		})
	if err != nil {
		log.Printf("couldn't suspend processes on ASG %s ", a.name)
	}
	time.Sleep(30 * time.Second * a.region.conf.SleepMultiplier)
}

func (a *autoScalingGroup) resumeProcesses(processes []string, delay time.Duration) {

	log.Printf("Resuming processes on ASG %s, waiting for %.0f seconds", a.name, delay.Seconds())
	time.Sleep(delay * a.region.conf.SleepMultiplier)
	log.Printf("Resuming processes on ASG %s, waiting complete, performing API call", a.name)

	_, err := a.region.services.autoScaling.ResumeProcesses(
		context.TODO(),
		&autoscaling.ResumeProcessesInput{
			AutoScalingGroupName: a.AutoScalingGroupName,
			ScalingProcesses:     processes,
		})
	if err != nil {
		log.Printf("couldn't resume processes on ASG %s ", a.name)
	}
}

func (a *autoScalingGroup) hasLifecycleHook(hookType string) (bool, *autoscalingTypes.LifecycleHook) {
	result, err := a.region.services.autoScaling.
		DescribeLifecycleHooks(
			context.TODO(),
			&autoscaling.DescribeLifecycleHooksInput{
				AutoScalingGroupName: a.AutoScalingGroupName,
			})

	if err != nil {
		log.Println(err.Error())
		return false, nil
	}

	for _, lfh := range result.LifecycleHooks {
		if *lfh.LifecycleTransition == hookType {
			log.Println("Found Hook", *lfh.LifecycleHookName)
			return true, &lfh
		}
	}
	return false, nil
}

func (a *autoScalingGroup) hasCodeDeployLifecycleHook() (bool, *autoscalingTypes.LifecycleHook) {
	hasLH, lHook := a.hasLifecycleHook("autoscaling:EC2_INSTANCE_LAUNCHING")

	if !hasLH {
		return false, nil
	}
	if strings.HasPrefix(*lHook.LifecycleHookName, "CodeDeploy-managed-automatic-launch-deployment-hook") {
		return true, lHook
	}
	return false, nil

}

func (a *autoScalingGroup) triggerCodeDeployDeployment(hook *autoscalingTypes.LifecycleHook) error {

	appName, deploymentGroupName, err := a.findDeployment(*hook.LifecycleHookName)

	if err != nil {
		log.Println(err.Error())
		return err
	}

	return a.triggerDeployment(appName, deploymentGroupName)

}

func (a *autoScalingGroup) findDeployment(hookName string) (*string, *string, error) {

	apps, err := a.region.services.codedeploy.ListApplications(
		context.TODO(),
		&codedeploy.ListApplicationsInput{})

	if err != nil {
		log.Println(err.Error())
		return nil, nil, err
	}

	for _, myApp := range apps.Applications {
		app := myApp
		log.Println("Processing CodeDeploy application:", app)

		groups, err := a.region.services.codedeploy.ListDeploymentGroups(
			context.TODO(),
			&codedeploy.ListDeploymentGroupsInput{
				ApplicationName: &app,
			})

		if err != nil {
			log.Println(err.Error())
			return nil, nil, err
		}

		for _, myGroup := range groups.DeploymentGroups {
			group := myGroup
			log.Printf("Processing CodeDeploy deployment group %s for application %s", group, app)
			gd, err := a.region.services.codedeploy.GetDeploymentGroup(
				context.TODO(),
				&codedeploy.GetDeploymentGroupInput{
					ApplicationName:     &app,
					DeploymentGroupName: &group,
				})

			if err != nil {
				log.Println(err.Error())
				return nil, nil, err
			}

			if gd.DeploymentGroupInfo.AutoScalingGroups == nil ||
				len(gd.DeploymentGroupInfo.AutoScalingGroups) == 0 {
				log.Printf("Deployment group %s for application %s has no ASG configuration, skipping...", group, app)
				continue
			}

			asg := gd.DeploymentGroupInfo.AutoScalingGroups[0]

			log.Printf("Deployment group %s for application %s has the ASG %s and hook %s", group, app, *asg.Name, *asg.Hook)

			if *asg.Hook == hookName && *asg.Name == a.name {
				log.Printf("Found matching deployment group %s for application %s for the ASG %s and hook %s",
					group, app, *asg.Name, *asg.Hook)
				return &app, &group, nil
			}
		}
	}
	return nil, nil, fmt.Errorf("unable to find deployment group information")

}

func (a *autoScalingGroup) triggerDeployment(appName, deploymentGroupName *string) error {

	_, err := a.region.services.codedeploy.CreateDeployment(
		context.TODO(),
		&codedeploy.CreateDeploymentInput{
			ApplicationName:             appName,
			DeploymentGroupName:         deploymentGroupName,
			Description:                 aws.String("AutoSpoting triggered deployment after new Spot instance launch"),
			UpdateOutdatedInstancesOnly: true,
		})

	if err != nil {
		log.Println(err.Error())
		return err
	}

	return nil
}

// Returns the Spot product set on the ASG tags, and if it doesn't exist, determines it
// from the AMI of one of the instances and persists it in the same tag.
func (a *autoScalingGroup) getSpotProduct() *string {

	if a.spotProduct != nil {
		return a.spotProduct
	}

	// Check if the Spot product is already set in a tag on the Auto Scaling group
	spotProductTag := "autospotting_spot_product"
	for _, tag := range a.Tags {
		if *tag.Key == spotProductTag {
			debug.Printf("Found Spot product '%s' set in tag on Auto Scaling group '%s'", *tag.Value, a.name)
			return tag.Value
		}
	}

	// Determine the Spot product based on the AMI ID of an instance in the Auto Scaling group
	instance := a.getInstance(nil, false)
	if instance == nil {
		log.Printf("Could not find any Spot instances in Auto Scaling group '%s', cannot set Spot product", a.name)
		instance = a.getInstance(nil, true)
		if instance == nil {
			log.Printf("Also couldn't find OnDemand instances in Auto Scaling group '%s', cannot set Spot product", a.name)
			return nil
		}
	}

	log.Printf("Determining Spot product for Auto Scaling group '%s' based on instance '%s'", a.name, *instance.InstanceId)
	spotProduct := instance.getSpotProductFromAMI()

	if spotProduct == nil {
		log.Printf("Couldn't determinine Spot product for Auto Scaling group '%s'", a.name)
		return nil
	}

	// Set the Spot product in a tag on the Auto Scaling group

	_, err := a.region.services.autoScaling.CreateOrUpdateTags(
		context.TODO(),
		&autoscaling.CreateOrUpdateTagsInput{
			Tags: []autoscalingTypes.Tag{
				{
					ResourceId:        aws.String(a.name),
					ResourceType:      aws.String("auto-scaling-group"),
					Key:               aws.String(spotProductTag),
					Value:             spotProduct,
					PropagateAtLaunch: aws.Bool(true),
				},
			},
		})
	if err != nil {
		log.Printf("Error setting Spot product '%s' in tag on Auto Scaling group '%s': %v", *spotProduct, a.name, err)
		return nil
	}
	log.Printf("Set Spot product '%s' in tag on Auto Scaling group '%s'", *spotProduct, a.name)

	a.spotProduct = spotProduct
	return spotProduct
}
func (a *autoScalingGroup) hadRecentSpotTermination() (bool, error) {
	if a == nil || a.AutoScalingGroup == nil {
		log.Println("AutoScalingGroup or its Group is nil")
		return false, errInvalidGroup
	}

	log.Printf("Checking for recent spot terminations in ASG: %s", *a.AutoScalingGroupName)

	svc := a.region.services.autoScaling
	input := &autoscaling.DescribeScalingActivitiesInput{
		AutoScalingGroupName: a.AutoScalingGroupName,
	}

	resp, err := svc.DescribeScalingActivities(context.TODO(), input)
	if err != nil {
		log.Printf("Error retrieving scaling activities for ASG %s: %s", *a.AutoScalingGroupName, err)
		return false, err
	}

	// Define the termination message we're looking for
	terminationMessage := "was taken out of service in response to an EC2 Spot Instance interruption notice."

	// Log the number of activities retrieved
	log.Printf("Retrieved %d scaling activities for ASG %s", len(resp.Activities), *a.AutoScalingGroupName)

	// Check if any recent activities (within the last 3 minutes) contain the termination message
	for _, activity := range resp.Activities {
		if activity.Cause != nil && strings.Contains(*activity.Cause, terminationMessage) {
			activityTime := *activity.StartTime
			if time.Since(activityTime).Minutes() <= 4 {
				log.Printf("Recent spot termination found in ASG %s, activity time: %s", *a.AutoScalingGroupName, activityTime)
				return true, nil
			}
			log.Printf("Spot termination activity found in ASG %s, but it's older than 4 minutes, activity time: %s", *a.AutoScalingGroupName, activityTime)
		}
	}

	log.Printf("No recent spot terminations found in ASG %s", *a.AutoScalingGroupName)
	return false, nil
}

func (a *autoScalingGroup) containsInstance(inst *instance) bool {
	if a == nil || a.AutoScalingGroup == nil || a.Instances == nil {
		log.Println("AutoScalingGroup or its Group or Instances are nil")
		return false
	}

	if inst == nil || inst.InstanceId == nil {
		log.Println("Provided instance or its InstanceId is nil")
		return false
	}

	for _, asgInst := range a.Instances {
		if asgInst.InstanceId != nil && *asgInst.InstanceId == *inst.InstanceId {
			return true
		}
	}

	return false
}

func (a *autoScalingGroup) isAutoScalingRefreshInProgress() (bool, error) {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	result, err := a.region.services.autoScaling.DescribeInstanceRefreshes(
		ctx,
		&autoscaling.DescribeInstanceRefreshesInput{
			AutoScalingGroupName: aws.String(a.name),
		})

	if err != nil {
		log.Printf("Error describing instance refreshes for ASG %s: %v", a.name, err)
		return false, err
	}

	// Check if any refresh action is in the list of ongoing statuses.
	for _, refresh := range result.InstanceRefreshes {
		if refresh.Status == "Pending" || refresh.Status == "InProgress" ||
			refresh.Status == "Cancelling" || refresh.Status == "RollbackInProgress" {
			return true, nil // Indicate that a refresh action is in progress.
		}
	}

	// If no refresh actions are in progress, return false.
	return false, nil
}

func (a *autoScalingGroup) isBeanstalkEnvironmentUpdating() (bool, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Use the new method to get the Beanstalk deployment information
	envName, envID := a.getBeanstalkDeploymentInformation()

	// Validate that both environment name and ID are available
	if envName == nil || envID == nil {
		log.Printf("Elastic Beanstalk environment name or ID tags are missing from ASG %s", a.name)
		return false, errors.New("Elastic Beanstalk environment name or ID tags are missing")
	}

	input := &elasticbeanstalk.DescribeEnvironmentsInput{
		EnvironmentNames: []string{*envName},
		EnvironmentIds:   []string{*envID},
	}

	resp, err := a.region.services.beanstalk.DescribeEnvironments(ctx, input)
	if err != nil {
		log.Printf("Could not describe Beanstalk environment %s with ID %s: %v", *envName, *envID, err.Error())
		return false, err
	}

	if len(resp.Environments) == 0 {
		log.Printf("No environments found for names %v or IDs %v. Exiting.", *envName, *envID)
		return false, fmt.Errorf("no environments found for names %v or IDs %v", *envName, *envID)
	}

	env := resp.Environments[0]
	// Consider "updating" to be any state other than ready, terminated, or terminating.
	isUpdating := env.Status != beanstalkTypes.EnvironmentStatusReady &&
		env.Status != beanstalkTypes.EnvironmentStatusTerminated &&
		env.Status != beanstalkTypes.EnvironmentStatusTerminating

	return isUpdating, nil
}

func (a *autoScalingGroup) getBeanstalkDeploymentInformation() (*string, *string) {
	var envName, envID *string

	for _, tag := range a.Tags {
		if *tag.Key == "elasticbeanstalk:environment-name" {
			envName = tag.Value
		} else if *tag.Key == "elasticbeanstalk:environment-id" {
			envID = tag.Value
		}
	}
	return envName, envID
}

func (a *autoScalingGroup) isASGElegibleForProcessing() (bool, string) {
	// Check for CloudFormation stack update
	tagCloudFormationStackName := "aws:cloudformation:stack-name"
	stackName := a.getTagValue(tagCloudFormationStackName)
	if stackName != nil {
		if status, updating := a.region.isStackUpdating(stackName); updating {
			return false, fmt.Sprintf("Stack %s is in state %s", *stackName, status)
		}
	}

	// Check for ASG instance refresh progress
	if updating, err := a.isAutoScalingRefreshInProgress(); err != nil {
		return false, fmt.Sprintf("Error checking ASG instance refresh progress: %v", err)
	} else if updating {
		return false, "ASG instance refresh is in progress"
	}

	// Check if the Beanstalk environment is updating
	envName, envID := a.getBeanstalkDeploymentInformation()
	if envName != nil && envID != nil { // Only proceed if both values are found
		if updating, err := a.isBeanstalkEnvironmentUpdating(); err != nil {
			return false, fmt.Sprintf("Error checking Beanstalk environment update progress: %v", err)
		} else if updating {
			return false, "Elastic Beanstalk environment update is in progress"
		}
	}

	return true, "" // ASG is eligible for processing
}
