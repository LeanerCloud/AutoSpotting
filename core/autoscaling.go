package autospotting

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
)

const (
	// OnDemandPercentageLong is the name of a tag that can be defined on a
	// per-group level for overriding maintained on-demand capacity given as a
	// percentage of the group's running instances.
	OnDemandPercentageLong = "autospotting_on_demand_percentage"

	// OnDemandNumberLong is the name of a tag that can be defined on a
	// per-group level for overriding maintained on-demand capacity given as an
	// absolute number.
	OnDemandNumberLong = "autospotting_on_demand_number"

	// BiddingPolicyTag stores the bidding policy for the spot instance
	BiddingPolicyTag = "autospotting_bidding_policy"

	// SpotPriceBufferPercentageTag stores percentage value above the
	// current spot price to place the bid
	SpotPriceBufferPercentageTag = "spot_price_buffer_percentage"

	// DefaultMinOnDemandValue stores the default on-demand capacity to be kept
	// running in a group managed by autospotting.
	DefaultMinOnDemandValue = 0

	// DefaultSpotPriceBufferPercentage stores the default percentage value
	// above the current spot price to place a bid
	DefaultSpotPriceBufferPercentage = 10.0

	// DefaultBiddingPolicy stores the default bidding policy for
	// the spot bid on a per-group level
	DefaultBiddingPolicy = "normal"
)

type autoScalingGroup struct {
	*autoscaling.Group

	name   string
	region *region

	instances instances

	// spot instance requests generated for the current group
	spotInstanceRequests []*spotInstanceRequest
	minOnDemand          int64
}

func (a *autoScalingGroup) loadPercentageOnDemand(tagValue *string) (int64, bool) {
	percentage, err := strconv.ParseFloat(*tagValue, 64)
	if err != nil {
		logger.Printf("Error with ParseFloat: %s\n", err.Error())
	} else if percentage == 0 {
		logger.Printf("Loaded MinOnDemand value to %f from tag %s\n", percentage, OnDemandPercentageLong)
		return int64(percentage), true
	} else if percentage > 0 && percentage <= 100 {
		instanceNumber := float64(a.instances.count())
		onDemand := int64(math.Floor((instanceNumber * percentage / 100.0) + .5))
		logger.Printf("Loaded MinOnDemand value to %d from tag %s\n", onDemand, OnDemandPercentageLong)
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
	tagList := [2]string{OnDemandNumberLong, OnDemandPercentageLong}
	loadDyn := map[string]func(*string) (int64, bool){
		OnDemandPercentageLong: a.loadPercentageOnDemand,
		OnDemandNumberLong:     a.loadNumberOnDemand,
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
	logger.Println("Currently less OnDemand instances than required !")
	if a.allInstanceRunning() && a.instances.count64() >= *a.DesiredCapacity {
		logger.Println("All instances are running and desired capacity is satisfied")
		if randomSpot := a.getAnySpotInstance(); randomSpot != nil {
			if totalRunning == 1 {
				logger.Println("Warning: blocking replacement of very last instance - consider raising ASG to >= 2")
			} else {
				logger.Println("Terminating a random spot instance",
					*randomSpot.Instance.InstanceId)
				randomSpot.terminate()
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
	logger.Println("Finding spot instance requests created for", a.name)
	err := a.findSpotInstanceRequests()
	if err != nil {
		logger.Printf("Error: %s while searching for spot instances for %s\n", err, a.name)
	}
	a.scanInstances()
	a.loadDefaultConfig()
	a.loadConfigFromTags()

	debug.Println("Found spot instance requests:", a.spotInstanceRequests)

	if !a.needReplaceOnDemandInstances() {
		return
	}

	spotInstanceID, waitForNextRun := a.havingReadyToAttachSpotInstance()

	if waitForNextRun {
		logger.Println("Waiting for next run while processing", a.name)
		return
	}

	if spotInstanceID != nil {
		logger.Println(a.region.name, "Attaching spot instance",
			*spotInstanceID, "to", a.name)

		a.replaceOnDemandInstanceWithSpot(spotInstanceID)
	} else {
		// find any given on-demand instance and try to replace it with a spot one
		onDemandInstance := a.getInstance(nil, true, false)

		if onDemandInstance == nil {
			logger.Println(a.region.name, a.name,
				"No running on-demand instances were found, nothing to do here...")
			return
		}

		azToLaunchSpotIn := onDemandInstance.Placement.AvailabilityZone
		logger.Println(a.region.name, a.name,
			"Would launch a spot instance in ", *azToLaunchSpotIn)

		a.launchCheapestSpotInstance(azToLaunchSpotIn)
	}
}

func (a *autoScalingGroup) findSpotInstanceRequests() error {

	resp, err := a.region.services.ec2.DescribeSpotInstanceRequests(
		&ec2.DescribeSpotInstanceRequestsInput{
			Filters: []*ec2.Filter{
				{
					Name:   aws.String("tag:launched-for-asg"),
					Values: []*string{a.AutoScalingGroupName},
				},
			},
		})

	if err != nil {
		return err
	}
	logger.Println("Spot instance requests were previously created for", a.name)

	for _, req := range resp.SpotInstanceRequests {
		a.spotInstanceRequests = append(a.spotInstanceRequests,
			a.loadSpotInstanceRequest(req))
	}

	return nil
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

		if i.isSpot() {
			i.price = i.typeInfo.pricing.spot[*i.Placement.AvailabilityZone]
		} else {
			i.price = i.typeInfo.pricing.onDemand
		}

		i.asg = a

		a.instances.add(i)
	}
	return a.instances
}

func (a *autoScalingGroup) propagatedInstanceTags() []*ec2.Tag {
	var tags []*ec2.Tag

	for _, asgTag := range a.Tags {
		if *asgTag.PropagateAtLaunch && !strings.HasPrefix(*asgTag.Key, "aws:") {
			tags = append(tags, &ec2.Tag{
				Key:   asgTag.Key,
				Value: asgTag.Value,
			})
		}
	}
	return tags
}

func (a *autoScalingGroup) replaceOnDemandInstanceWithSpot(
	spotInstanceID *string) error {

	minSize, maxSize := *a.MinSize, *a.MaxSize
	desiredCapacity := *a.DesiredCapacity

	// temporarily increase AutoScaling group in case it's of static size
	if minSize == maxSize {
		logger.Println(a.name, "Temporarily increasing MaxSize")
		a.setAutoScalingMaxSize(maxSize + 1)
		defer a.setAutoScalingMaxSize(maxSize)
	}

	// get the details of our spot instance so we can see its AZ
	logger.Println(a.name, "Retrieving instance details for ", *spotInstanceID)
	spotInst := a.region.instances.get(*spotInstanceID)
	if spotInst == nil {
		return errors.New("couldn't find spot instance to use")
	}
	az := spotInst.Placement.AvailabilityZone

	logger.Println(a.name, *spotInstanceID, "is in the availability zone",
		*az, "looking for an on-demand instance there")

	// find an on-demand instance from the same AZ as our spot instance
	odInst := a.getOnDemandInstanceInAZ(az)

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

	return a.detachAndTerminateOnDemandInstance(odInst.InstanceId)
}

// Returns the information about the first running instance found in
// the group, while iterating over all instances from the
// group. It can also filter by AZ and Lifecycle.
func (a *autoScalingGroup) getInstance(
	availabilityZone *string,
	onDemand bool, any bool) *instance {

	var retI *instance

	for i := range a.instances.instances() {
		if retI != nil {
			continue
		}

		// instance is running
		if *i.State.Name == "running" {

			// the InstanceLifecycle attribute is non-nil only for spot instances,
			// where it contains the value "spot", if we're looking for on-demand
			// instances only, then we have to skip the current instance.
			if !any &&
				(onDemand && i.isSpot() ||
					(!onDemand && !i.isSpot())) {
				continue
			}
			if (availabilityZone != nil) &&
				(*availabilityZone != *i.Placement.AvailabilityZone) {
				continue
			}
			retI = i
		}
	}
	return retI
}

func (a *autoScalingGroup) getOnDemandInstanceInAZ(az *string) *instance {
	return a.getInstance(az, true, false)
}

func (a *autoScalingGroup) getAnyOnDemandInstance() *instance {
	return a.getInstance(nil, true, false)
}

func (a *autoScalingGroup) getAnySpotInstance() *instance {
	return a.getInstance(nil, false, false)
}

// returns an instance ID as *string and a bool that tells us if  we need to
// wait for the next run in case there are spot instances still being launched
func (a *autoScalingGroup) havingReadyToAttachSpotInstance() (*string, bool) {

	var activeSpotInstanceRequest *spotInstanceRequest

	// if there are on-demand instances but no spot instance requests yet,
	// then we can launch a new spot instance
	if len(a.spotInstanceRequests) == 0 {
		logger.Println(a.name, "no spot bids were found")
		if inst := a.getAnyOnDemandInstance(); inst != nil {
			logger.Println(a.name, "on-demand instances were found, proceeding to "+
				"launch a replacement spot instance")
			return nil, false
		}
		// Looks like we have no instances in the group, so we can stop here
		logger.Println(a.name, "no on-demand instances were found, nothing to do")
		return nil, true
	}

	logger.Println("spot bids were found, continuing")

	// Here we search for open spot requests created for the current ASG, and try
	// to wait for their instances to start.
	for _, req := range a.spotInstanceRequests {
		if *req.State == "open" && *req.Tags[0].Value == a.name {
			logger.Println(a.name, "Open bid found for current AutoScaling Group, "+
				"waiting for the instance to start so it can be tagged...")

			// Here we resume the wait for instances, initiated after requesting the
			// spot instance. This may sometimes time out the entire lambda function
			// run, just like it could time out the one done when we requested the
			// new instance. In case of timeout the next run should continue waiting
			// for the instance, and the process should continue until the new
			// instance was found. In case of failed spot requests, the first lambda
			// function timeout when waiting for the instances would break the loop,
			// because the subsequent run would find a failed spot request instead
			// of an open one.
			req.waitForAndTagSpotInstance()
			activeSpotInstanceRequest = req
		}

		// We found a spot request with a running instance.
		if *req.State == "active" &&
			*req.Status.Code == "fulfilled" {
			logger.Println(a.name, "Active bid was found, with instance already "+
				"started:", *req.InstanceId)

			// If the instance is already in the group we don't need to do anything.
			if a.instances.get(*req.InstanceId) != nil {
				logger.Println(a.name, "Instance", *req.InstanceId,
					"is already attached to the ASG, skipping...")
				continue

				// In case the instance wasn't yet attached, we prepare to attach it.
			} else {
				logger.Println(a.name, "Instance", *req.InstanceId,
					"is not yet attached to the ASG, checking if it's running")

				if i := a.instances.get(*req.InstanceId); i != nil &&
					i.State != nil &&
					*i.State.Name == "running" {
					logger.Println(a.name, "Active bid was found, with running "+
						"instances not yet attached to the ASG",
						*req.InstanceId)
					activeSpotInstanceRequest = req
					break
				} else {
					logger.Println(a.name, "Active bid was found, with no running "+
						"instances, waiting for an instance to start ...")
					req.waitForAndTagSpotInstance()
					activeSpotInstanceRequest = req
				}
			}
		}
	}

	// In case we don't have any active spot requests with instances in the
	// process of starting or already ready to be attached to the group, we can
	// launch a new spot instance.
	if activeSpotInstanceRequest == nil {
		logger.Println(a.name, "No active unfulfilled bid was found")
		return nil, false
	}

	spotInstanceID := activeSpotInstanceRequest.InstanceId

	logger.Println("Considering ", *spotInstanceID, "for attaching to", a.name)

	instData := a.region.instances.get(*spotInstanceID)
	gracePeriod := *a.HealthCheckGracePeriod

	debug.Println(instData)

	if instData == nil || instData.LaunchTime == nil {
		logger.Println("Apparently", *spotInstanceID, "is no longer running, ",
			"cancelling the spot instance request which created it...")

		a.region.services.ec2.CancelSpotInstanceRequests(
			&ec2.CancelSpotInstanceRequestsInput{
				SpotInstanceRequestIds: []*string{activeSpotInstanceRequest.SpotInstanceRequestId},
			})
		return nil, true
	}

	instanceUpTime := time.Now().Unix() - instData.LaunchTime.Unix()

	logger.Println("Instance uptime:", time.Duration(instanceUpTime)*time.Second)

	// Check if the spot instance is out of the grace period, so in that case we
	// can replace an on-demand instance with it
	if *instData.State.Name == "running" &&
		instanceUpTime < gracePeriod {
		logger.Println("The new spot instance", *spotInstanceID,
			"is still in the grace period,",
			"waiting for it to be ready before we can attach it to the group...")
		return nil, true
	} else if *instData.State.Name == "pending" {
		logger.Println("The new spot instance", *spotInstanceID,
			"is still pending,",
			"waiting for it to be running before we can attach it to the group...")
		return nil, true
	}
	return spotInstanceID, false
}

func (a *autoScalingGroup) getAllowedInstanceTypes(baseInstance *instance) []string {
	var allowed, allowedInstanceTypesTag string

	// Check option of allowed instance types
	// If we have that option we don't need to calculate the compatible instance type.
	if tagValue := a.getTagValue("allowed-instance-types"); tagValue != nil {
		allowedInstanceTypesTag = strings.Replace(*tagValue, " ", ",", -1)
	}
	allowedInstanceTypes := strings.Replace(a.region.conf.AllowedInstanceTypes, " ", ",", -1)

	// Command line config has a priority
	if allowedInstanceTypes != "" {
		allowed = allowedInstanceTypes
	} else {
		allowed = allowedInstanceTypesTag
	}

	if allowed == "current" {
		return []string{baseInstance.typeInfo.instanceType}
	}
	return strings.Split(allowed, ",")
}

func (a *autoScalingGroup) getDisallowedInstanceTypes(baseInstance *instance) []string {
	var disallowedInstanceTypesTag string

	// By default take the command line parameter
	disallowed := strings.Replace(a.region.conf.DisallowedInstanceTypes, " ", ",", -1)

	// Check option of disallowed instance types
	// If we have that option we don't need to calculate the compatible instance type.
	if tagValue := a.getTagValue("disallowed-instance-types"); tagValue != nil {
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

func (a *autoScalingGroup) getPricetoBid(
	baseOnDemandPrice float64, currentSpotPrice float64) float64 {

	logger.Println("BiddingPolicy: ", a.region.conf.BiddingPolicy)

	if a.region.conf.BiddingPolicy == DefaultBiddingPolicy {
		logger.Println("Launching spot instance with a bid =", baseOnDemandPrice)
		return baseOnDemandPrice
	}

	logger.Println("Launching spot instance with a bid =", math.Min(baseOnDemandPrice, currentSpotPrice*(1.0+a.region.conf.SpotPriceBufferPercentage/100.0)))
	return math.Min(baseOnDemandPrice, currentSpotPrice*(1.0+a.region.conf.SpotPriceBufferPercentage/100.0))
}

func (a *autoScalingGroup) launchCheapestSpotInstance(
	azToLaunchIn *string) error {

	if azToLaunchIn == nil {
		logger.Println("Can't launch instances in any AZ, nothing to do here...")
		return errors.New("invalid availability zone provided")
	}

	logger.Println("Trying to launch spot instance in", *azToLaunchIn,
		"first finding an on-demand instance to use as a template")

	baseInstance := a.getOnDemandInstanceInAZ(azToLaunchIn)

	if baseInstance == nil {
		logger.Println("Found no on-demand instances, nothing to do here...")
		return errors.New("no on-demand instances found")
	}
	logger.Println("Found on-demand instance", baseInstance.InstanceId)

	allowedInstances := a.getAllowedInstanceTypes(baseInstance)
	disallowedInstances := a.getDisallowedInstanceTypes(baseInstance)

	newInstanceType, err := baseInstance.getCheapestCompatibleSpotInstanceType(allowedInstances, disallowedInstances)
	if err != nil {
		logger.Println("No cheaper compatible instance type was found, "+
			"nothing to do here...", err)
		return errors.New("no cheaper spot instance found")
	}

	newInstance := a.region.instanceTypeInformation[newInstanceType]

	baseOnDemandPrice := baseInstance.price

	currentSpotPrice := newInstance.pricing.spot[*azToLaunchIn]
	logger.Println("Finished searching for best spot instance in ", *azToLaunchIn)
	logger.Println("Replacing an on-demand", *baseInstance.InstanceType,
		"instance having the ondemand price", baseOnDemandPrice)
	logger.Println("Launching best compatible instance:", newInstanceType,
		"with the current spot price:", currentSpotPrice)

	lc := a.getLaunchConfiguration()

	spotLS := lc.convertLaunchConfigurationToSpotSpecification(
		baseInstance,
		newInstance,
		*azToLaunchIn)

	logger.Println("Bidding for spot instance for ", a.name)
	return a.bidForSpotInstance(spotLS, a.getPricetoBid(baseOnDemandPrice, currentSpotPrice))
}

func (a *autoScalingGroup) loadSpotInstanceRequest(
	req *ec2.SpotInstanceRequest) *spotInstanceRequest {
	return &spotInstanceRequest{SpotInstanceRequest: req,
		region: a.region,
		asg:    a,
	}
}

func (a *autoScalingGroup) bidForSpotInstance(
	ls *ec2.RequestSpotLaunchSpecification,
	price float64,
) error {

	svc := a.region.services.ec2

	resp, err := svc.RequestSpotInstances(&ec2.RequestSpotInstancesInput{
		SpotPrice:           aws.String(strconv.FormatFloat(price, 'f', -1, 64)),
		LaunchSpecification: ls,
	})

	if err != nil {
		logger.Println("Failed to create spot instance request for",
			a.name, err.Error(), ls)
		return err
	}

	spotRequest := resp.SpotInstanceRequests[0]
	sr := spotInstanceRequest{SpotInstanceRequest: spotRequest,
		region: a.region,
		asg:    a,
	}

	srID := sr.SpotInstanceRequestId

	logger.Println(a.name, "Created spot instance request", *srID)

	// tag the spot instance request to associate it with the current ASG, so we
	// know where to attach the instance later. In case the waiter failed, it may
	// happen that the instance is actually tagged in the next run, but the spot
	// instance request needs to be tagged anyway.
	err = sr.tag(a.name)

	if err != nil {
		logger.Println(a.name, "Can't tag spot instance request", err.Error())
		return err
	}
	// Waiting for the instance to start so that we can then later tag it with
	// the same tags originally set on the on-demand instances.
	//
	// This waiter only returns after the instance was found and it may be
	// interrupted by the lambda function's timeout, so we also need to check in
	// the next run if we have any open spot requests with no instances and
	// resume the wait there.
	return sr.waitForAndTagSpotInstance()
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

func (a *autoScalingGroup) getLaunchConfiguration() *launchConfiguration {

	lcName := a.LaunchConfigurationName

	if lcName == nil {
		return nil
	}

	svc := a.region.services.autoScaling

	params := &autoscaling.DescribeLaunchConfigurationsInput{
		LaunchConfigurationNames: []*string{lcName},
	}
	resp, err := svc.DescribeLaunchConfigurations(params)

	if err != nil {
		logger.Println(err.Error())
		return nil
	}

	return &launchConfiguration{LaunchConfiguration: resp.LaunchConfigurations[0]}
}

func (a *autoScalingGroup) attachSpotInstance(spotInstanceID *string) error {

	svc := a.region.services.autoScaling

	params := autoscaling.AttachInstancesInput{
		AutoScalingGroupName: aws.String(a.name),
		InstanceIds: []*string{
			spotInstanceID,
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

	return a.instances.get(*instanceID).terminate()
}

// Counts the number of already running spot instances.
func (a *autoScalingGroup) alreadyRunningSpotInstanceTypeCount(
	instanceType, availabilityZone string) int64 {

	var count int64
	logger.Println(a.name, "Counting already running spot instances of type ",
		instanceType, " in AZ ", availabilityZone)
	for inst := range a.instances.instances() {
		if *inst.InstanceType == instanceType &&
			*inst.Placement.AvailabilityZone == availabilityZone &&
			inst.isSpot() {
			logger.Println(a.name, "Found running spot instance ",
				*inst.InstanceId, "of the same type:", instanceType)
			count++
		}
	}
	logger.Println(a.name, "Found", count, instanceType, "instances")
	return count
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
