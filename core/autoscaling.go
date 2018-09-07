package autospotting

import (
	"errors"
	"time"

	"math"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
)

const (
	// The tag names below allow overriding global setting on a per-group level.
	// They should follow the format below:
	// "autospotting_${overridden_command_line_parameter_name}"

	// For example the tag named "autospotting_min_on_demand_number" will override
	// the command-line option named "min_on_demand_number", and so on.

	// OnDemandPercentageTag is the name of a tag that can be defined on a
	// per-group level for overriding maintained on-demand capacity given as a
	// percentage of the group's running instances.
	OnDemandPercentageTag = "autospotting_min_on_demand_percentage"

	// OnDemandNumberLong is the name of a tag that can be defined on a
	// per-group level for overriding maintained on-demand capacity given as an
	// absolute number.
	OnDemandNumberLong = "autospotting_min_on_demand_number"

	// BiddingPolicyTag stores the bidding policy for the spot instance
	BiddingPolicyTag = "autospotting_bidding_policy"

	// SpotPriceBufferPercentageTag stores percentage value above the
	// current spot price to place the bid
	SpotPriceBufferPercentageTag = "autospotting_spot_price_buffer_percentage"

	// AllowedInstanceTypesTag is the name of a tag that can indicate which
	// instance types are allowed in the current group
	AllowedInstanceTypesTag = "autospotting_allowed_instance_types"

	// DisallowedInstanceTypesTag is the name of a tag that can indicate which
	// instance types are not allowed in the current group
	DisallowedInstanceTypesTag = "autospotting_disallowed_instance_types"

	// Default constant values should be defined below:

	// DefaultSpotProductDescription stores the default operating system
	// to use when looking up spot price history in the market.
	DefaultSpotProductDescription = "Linux/UNIX (Amazon VPC)"

	// DefaultMinOnDemandValue stores the default on-demand capacity to be kept
	// running in a group managed by autospotting.
	DefaultMinOnDemandValue = 0

	// DefaultSpotPriceBufferPercentage stores the default percentage value
	// above the current spot price to place a bid
	DefaultSpotPriceBufferPercentage = 10.0

	// DefaultBiddingPolicy stores the default bidding policy for
	// the spot bid on a per-group level
	DefaultBiddingPolicy = "normal"

	// DefaultInstanceTerminationMethod is the default value for the instance termination
	// method configuration option
	DefaultInstanceTerminationMethod = AutoScalingTerminationMethod
)

type autoScalingGroup struct {
	*autoscaling.Group

	name                string
	region              *region
	launchConfiguration *launchConfiguration
	instances           instances
	minOnDemand         int64

	terminationMethod string
}

func (a *autoScalingGroup) loadPercentageOnDemand(tagValue *string) (int64, bool) {
	percentage, err := strconv.ParseFloat(*tagValue, 64)
	if err != nil {
		logger.Printf("Error with ParseFloat: %s\n", err.Error())
	} else if percentage == 0 {
		logger.Printf("Loaded MinOnDemand value to %f from tag %s\n", percentage, OnDemandPercentageTag)
		return int64(percentage), true
	} else if percentage > 0 && percentage <= 100 {
		instanceNumber := float64(a.instances.count())
		onDemand := int64(math.Floor((instanceNumber * percentage / 100.0) + .5))
		logger.Printf("Loaded MinOnDemand value to %d from tag %s\n", onDemand, OnDemandPercentageTag)
		return onDemand, true
	}

	logger.Printf("Ignoring value out of range %f\n", percentage)

	return DefaultMinOnDemandValue, false
}

func (a *autoScalingGroup) loadSpotPriceBufferPercentage(tagValue *string) (float64, bool) {
	spotPriceBufferPercentage, err := strconv.ParseFloat(*tagValue, 64)

	if err != nil {
		logger.Printf("Error with ParseFloat: %s\n", err.Error())
		return DefaultSpotPriceBufferPercentage, false
	} else if spotPriceBufferPercentage <= 0 {
		logger.Printf("Ignoring out of range value : %f\n", spotPriceBufferPercentage)
		return DefaultSpotPriceBufferPercentage, false
	}

	logger.Printf("Loaded SpotPriceBufferPercentage value to %f from tag %s\n", spotPriceBufferPercentage, SpotPriceBufferPercentageTag)
	return spotPriceBufferPercentage, true
}

func (a *autoScalingGroup) loadNumberOnDemand(tagValue *string) (int64, bool) {
	onDemand, err := strconv.Atoi(*tagValue)
	if err != nil {
		logger.Printf("Error with Atoi: %s\n", err.Error())
	} else if onDemand >= 0 && int64(onDemand) <= *a.MaxSize {
		logger.Printf("Loaded MinOnDemand value to %d from tag %s\n", onDemand, OnDemandNumberLong)
		return int64(onDemand), true
	} else {
		logger.Printf("Ignoring value out of range %d\n", onDemand)
	}
	return DefaultMinOnDemandValue, false
}

func (a *autoScalingGroup) loadConfOnDemand() bool {
	tagList := [2]string{OnDemandNumberLong, OnDemandPercentageTag}
	loadDyn := map[string]func(*string) (int64, bool){
		OnDemandPercentageTag: a.loadPercentageOnDemand,
		OnDemandNumberLong:    a.loadNumberOnDemand,
	}

	for _, tagKey := range tagList {
		if tagValue := a.getTagValue(tagKey); tagValue != nil {
			if _, ok := loadDyn[tagKey]; ok {
				if newValue, done := loadDyn[tagKey](tagValue); done {
					a.minOnDemand = newValue
					return done
				}
			}
		}
		debug.Println("Couldn't find tag", tagKey)
	}
	return false
}

func (a *autoScalingGroup) loadBiddingPolicy(tagValue *string) (string, bool) {
	biddingPolicy := *tagValue
	if biddingPolicy != "aggressive" {
		return DefaultBiddingPolicy, false
	}

	logger.Printf("Loaded BiddingPolicy value with %s from tag %s\n", biddingPolicy, BiddingPolicyTag)
	return biddingPolicy, true
}

func (a *autoScalingGroup) loadConfSpot() bool {
	tagValue := a.getTagValue(BiddingPolicyTag)
	if tagValue == nil {
		debug.Println("Couldn't find tag", BiddingPolicyTag)
		return false
	}
	if newValue, done := a.loadBiddingPolicy(tagValue); done {
		a.region.conf.BiddingPolicy = newValue
		logger.Println("BiddingPolicy =", a.region.conf.BiddingPolicy)
		return done
	}
	return false
}

func (a *autoScalingGroup) loadConfSpotPrice() bool {

	tagValue := a.getTagValue(SpotPriceBufferPercentageTag)
	if tagValue == nil {
		return false
	}

	newValue, done := a.loadSpotPriceBufferPercentage(tagValue)
	if !done {
		debug.Println("Couldn't find tag", SpotPriceBufferPercentageTag)
		return false
	}

	a.region.conf.SpotPriceBufferPercentage = newValue
	return done
}

// Add configuration of other elements here: prices, whitelisting, etc
func (a *autoScalingGroup) loadConfigFromTags() bool {

	resOnDemandConf := a.loadConfOnDemand()

	resSpotConf := a.loadConfSpot()

	resSpotPriceConf := a.loadConfSpotPrice()

	if resOnDemandConf {
		logger.Println("Found and applied configuration for OnDemand value")
	}
	if resSpotConf {
		logger.Println("Found and applied configuration for Spot Bid")
	}
	if resSpotPriceConf {
		logger.Println("Found and applied configuration for Spot Price")
	}
	if resOnDemandConf || resSpotConf || resSpotPriceConf {
		return true
	}
	return false
}

func (a *autoScalingGroup) loadDefaultConfigNumber() (int64, bool) {
	onDemand := a.region.conf.MinOnDemandNumber
	if onDemand >= 0 && onDemand <= int64(a.instances.count()) {
		logger.Printf("Loaded default value %d from conf number.", onDemand)
		return onDemand, true
	}
	logger.Println("Ignoring default value out of range:", onDemand)
	return DefaultMinOnDemandValue, false
}

func (a *autoScalingGroup) loadDefaultConfigPercentage() (int64, bool) {
	percentage := a.region.conf.MinOnDemandPercentage
	if percentage < 0 || percentage > 100 {
		logger.Printf("Ignoring default value out of range: %f", percentage)
		return DefaultMinOnDemandValue, false
	}
	instanceNumber := a.instances.count()
	onDemand := int64(math.Floor((float64(instanceNumber) * percentage / 100.0) + .5))
	logger.Printf("Loaded default value %d from conf percentage.", onDemand)
	return onDemand, true
}

func (a *autoScalingGroup) loadDefaultConfig() bool {
	done := false
	a.minOnDemand = DefaultMinOnDemandValue

	if a.region.conf.SpotPriceBufferPercentage <= 0 {
		a.region.conf.SpotPriceBufferPercentage = DefaultSpotPriceBufferPercentage
	}

	if a.region.conf.MinOnDemandNumber != 0 {
		a.minOnDemand, done = a.loadDefaultConfigNumber()
	}
	if !done && a.region.conf.MinOnDemandPercentage != 0 {
		a.minOnDemand, done = a.loadDefaultConfigPercentage()
	} else {
		logger.Println("No default value for on-demand instances specified, skipping.")
	}
	return done
}

func (a *autoScalingGroup) loadLaunchConfiguration() error {
	//already done
	if a.launchConfiguration != nil {
		return nil
	}

	lcName := a.LaunchConfigurationName

	if lcName == nil {
		return errors.New("missing launch configuration")
	}

	svc := a.region.services.autoScaling

	params := &autoscaling.DescribeLaunchConfigurationsInput{
		LaunchConfigurationNames: []*string{lcName},
	}
	resp, err := svc.DescribeLaunchConfigurations(params)

	if err != nil {
		logger.Println(err.Error())
		return err
	}

	a.launchConfiguration = &launchConfiguration{
		LaunchConfiguration: resp.LaunchConfigurations[0],
	}
	return nil
}

func (a *autoScalingGroup) needReplaceOnDemandInstances() bool {
	onDemandRunning, totalRunning := a.alreadyRunningInstanceCount(false, "")
	if onDemandRunning > a.minOnDemand {
		logger.Println("Currently more than enough OnDemand instances running")
		return true
	}
	if onDemandRunning == a.minOnDemand {
		logger.Println("Currently OnDemand running equals to the required number, skipping run")
		return false
	}
	logger.Println("Currently fewer OnDemand instances than required !")
	if a.allInstanceRunning() && a.instances.count64() >= *a.DesiredCapacity {
		logger.Println("All instances are running and desired capacity is satisfied")
		if randomSpot := a.getAnySpotInstance(); randomSpot != nil {
			if totalRunning == 1 {
				logger.Println("Warning: blocking replacement of very last instance - consider raising ASG to >= 2")
			} else {
				logger.Println("Terminating a random spot instance",
					*randomSpot.Instance.InstanceId)
				switch a.terminationMethod {
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

func (a *autoScalingGroup) allInstanceRunning() bool {
	_, totalRunning := a.alreadyRunningInstanceCount(false, "")
	return totalRunning == a.instances.count64()
}

func (a *autoScalingGroup) process() {
	var spotInstanceID string
	a.scanInstances()
	a.loadDefaultConfig()
	a.loadConfigFromTags()

	logger.Println("Finding spot instances created for", a.name)

	spotInstance := a.findUnattachedInstanceLaunchedForThisASG()

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

		a.loadLaunchConfiguration()
		err := onDemandInstance.launchSpotReplacement()
		if err != nil {
			logger.Printf("Could not launch cheapest spot instance: %s", err)
		}
		return
	}

	spotInstanceID = *spotInstance.InstanceId

	if !a.needReplaceOnDemandInstances() || !spotInstance.isReadyToAttach(a) {
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
			i.price = i.typeInfo.pricing.onDemand
		}

		a.instances.add(i)
	}
	return a.instances
}

func (a *autoScalingGroup) replaceOnDemandInstanceWithSpot(
	spotInstanceID string) error {

	minSize, maxSize := *a.MinSize, *a.MaxSize
	desiredCapacity := *a.DesiredCapacity

	// temporarily increase AutoScaling group in case it's of static size
	if minSize == maxSize {
		logger.Println(a.name, "Temporarily increasing MaxSize")
		a.setAutoScalingMaxSize(maxSize + 1)
		defer a.setAutoScalingMaxSize(maxSize)
	}

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
	// revert attach/detach order when running on minimum capacity
	if desiredCapacity == minSize {
		attachErr := a.attachSpotInstance(spotInstanceID)
		if attachErr != nil {
			logger.Println(a.name, "skipping detaching on-demand due to failure to",
				"attach the new spot instance", *spotInst.InstanceId)
			return nil
		}
	} else {
		defer a.attachSpotInstance(spotInstanceID)
	}

	switch a.terminationMethod {
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

			if considerInstanceProtection && (i.isProtectedFromScaleIn() || i.isProtectedFromTermination()) {
				debug.Println(a.name, "skipping protected instance", *i.InstanceId)
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
	spot bool, availabilityZone string) (int64, int64) {

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
				(*inst.Placement.AvailabilityZone == availabilityZone || availabilityZone == "") {
				count++
				// Count running OnDemand instances
			} else if !spot && !inst.isSpot() &&
				(*inst.Placement.AvailabilityZone == availabilityZone || availabilityZone == "") {
				count++
			}
			// Count total running instances
			total++
		}
	}
	logger.Println(a.name, "Found", count, instanceCategory, "instances running on a total of", total)
	return count, total
}

func (a *autoScalingGroup) getTagValue(keyMatch string) *string {
	for _, asgTag := range a.Tags {
		if *asgTag.Key == keyMatch {
			return asgTag.Value
		}
	}
	return nil
}
