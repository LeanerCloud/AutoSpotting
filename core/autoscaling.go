package autospotting

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
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

	// DefaultSIRRequestCompleteTagName is the name of the Tag
	// used to mark that the Spot Instance Request has been satisfied
	// And the new sport instance has been added to the asg for replace the
	// pre-existing on demand instance
	DefaultSIRRequestCompleteTagName = "autospotting-complete"

	// DefaultSecondsSpotRequestValidFor is the default amount of  time in
	// seconds that the sport request is valid for
	// from the moment the requests iss created
	DefaultSecondsSpotRequestValidFor = 240
)

const (
	// InstanceStateCodePending is a InstanceState Code enum value
	InstanceStateCodePending = 0

	// InstanceStateCodeRunning is a InstanceState Code enum value
	InstanceStateCodeRunning = 16

	// InstanceStateCodeShuttingDown is a InstanceState Code enum value
	InstanceStateCodeShuttingDown = 32

	// InstanceStateCodeTerminated is a InstanceState Code enum value
	InstanceStateCodeTerminated = 48

	// InstanceStateCodeStopping is a InstanceState Code enum value
	InstanceStateCodeStopping = 64

	// InstanceStateCodeStopped is a InstanceState Code enum value
	InstanceStateCodeStopped = 80
)

type autoScalingGroup struct {
	*autoscaling.Group

	name   string
	region *region

	instances instances

	// spot instance requests generated for the current group
	spotInstanceRequests []*spotInstanceRequest
	minOnDemand          int64

	// for caching
	launchConfiguration *launchConfiguration
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
		// exit early.  If unable to search for spot instance requests.  Then we should not continue
		// and wait for next run instead.
		return
	}
	a.scanInstances()
	a.loadDefaultConfig()
	a.loadConfigFromTags()

	debug.Println("Found spot instance requests:", a.spotInstanceRequests)

	if !a.needReplaceOnDemandInstances() {
		return
	}

	spotInstanceID, spotRequestID, waitForNextRun := a.havingReadyToAttachSpotInstance()

	if waitForNextRun {
		logger.Println("Waiting for next run while processing", a.name)
		return
	}

	if spotInstanceID != nil {
		logger.Println(a.region.name, "Attaching spot instance",
			*spotInstanceID, "to", a.name)

		a.replaceOnDemandInstanceWithSpot(spotInstanceID)
		a.markSpotInstanceRequestAsCompete(spotRequestID)
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

		err := a.launchCheapestSpotInstance(azToLaunchSpotIn)
		if err != nil {
			logger.Printf("Could not launch cheapest spot instance: %s", err)
		}
	}
}

func (a *autoScalingGroup) markSpotInstanceRequestAsCompete(spotInstanceRequestID *string) {
	svc := a.region.services.ec2
	input := &ec2.CreateTagsInput{
		Resources: []*string{spotInstanceRequestID},
		Tags: []*ec2.Tag{
			{
				Key:   aws.String(DefaultSIRRequestCompleteTagName),
				Value: aws.String("true"),
			},
		},
	}

	tagged := false
	for count := 0; count < 10; count++ {
		_, err := svc.CreateTags(input)
		if err == nil {
			tagged = true
			break
		}

		logger.Println("Failed to mark the spot instance request as complete", *spotInstanceRequestID, "retrying in 5 seconds...")
		time.Sleep(5 * time.Second * a.region.conf.SleepMultiplier)
	}

	if tagged {
		logger.Println("Tagged spot instance request as compete", *spotInstanceRequestID)
	}

}

func (a *autoScalingGroup) filterOutCancelledRequestsWithNoInstances(requests []*ec2.SpotInstanceRequest) []*ec2.SpotInstanceRequest {
	var outStandingSpotRequests []*ec2.SpotInstanceRequest
	for _, req := range requests {
		if *req.State == "cancelled" && req.InstanceId == nil {
			a.markSpotInstanceRequestAsCompete(req.SpotInstanceRequestId)
		} else {
			outStandingSpotRequests = append(outStandingSpotRequests, req)
		}
	}
	return outStandingSpotRequests
}

func isSpotInstanceRequestComplete(req *ec2.SpotInstanceRequest) bool {
	sirIsComplete := false

	for _, tag := range req.Tags {
		if *tag.Key == DefaultSIRRequestCompleteTagName && *tag.Value == "true" {
			sirIsComplete = true
			break
		}
	}

	return sirIsComplete
}

func filterOutCompleteSpotInstanceRequests(requests []*ec2.SpotInstanceRequest) []*ec2.SpotInstanceRequest {
	var outStandingSpotRequests []*ec2.SpotInstanceRequest
	for _, req := range requests {
		if !isSpotInstanceRequestComplete(req) {
			outStandingSpotRequests = append(outStandingSpotRequests, req)
		}
	}
	return outStandingSpotRequests
}

func (a *autoScalingGroup) describeSpotInstanceRequests() (*ec2.DescribeSpotInstanceRequestsOutput, error) {
	return a.region.services.ec2.DescribeSpotInstanceRequests(
		&ec2.DescribeSpotInstanceRequestsInput{
			Filters: []*ec2.Filter{
				{
					Name:   aws.String("tag:launched-for-asg"),
					Values: []*string{a.AutoScalingGroupName},
				},
			},
		})
}

func (a *autoScalingGroup) findSpotInstanceRequests() error {

	resp, err := a.describeSpotInstanceRequests()

	if err != nil {
		return err
	}

	//
	// Filter out completed spot instance requests
	//
	spotRequests := filterOutCompleteSpotInstanceRequests(resp.SpotInstanceRequests)

	//
	// Filter out cancelled spot instance requests that have timed out, and have no instances
	//
	spotRequests = a.filterOutCancelledRequestsWithNoInstances(spotRequests)

	if len(spotRequests) > 0 {
		logger.Println("Spot instance requests were previously created for", a.name)
	}

	for _, req := range spotRequests {
		a.spotInstanceRequests = append(a.spotInstanceRequests,
			a.loadSpotInstanceRequest(req))
	}

	// Check if the ASG's desired capacity has been set to 0 inbetween
	// spot instance creation, and next run to assign spot instance
	// to the asg
	a.checkDesiredCapacity()

	return nil
}

func (a *autoScalingGroup) checkDesiredCapacity() {
	if *a.Group.DesiredCapacity == 0 && len(a.spotInstanceRequests) > 0 {
		logger.Println("Desired Capacity is Zero. However, there are Spot Instance Requests.  Proceeding with cancelling them")
		for _, req := range a.spotInstanceRequests {
			terminated := true
			err := a.cancelSIR(req.SpotInstanceRequestId)

			if req.InstanceId != nil {
				terminated = a.terminateInstance(req.InstanceId, a.getInstanceState(req.InstanceId))
			}

			if err == nil && terminated {
				a.markSpotInstanceRequestAsCompete(req.SpotInstanceRequestId)
			}
		}
	}

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
	var tags, additionalTags []*ec2.Tag

	additionalTags = []*ec2.Tag{
		{
			Key:   aws.String("LaunchConfigurationName"),
			Value: a.LaunchConfigurationName,
		},
		{
			Key:   aws.String("launched-by-autospotting"),
			Value: aws.String("true"),
		},
	}

	for _, tag := range additionalTags {
		tags = append(tags, tag)
	}

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
		if *i.State.Name == ec2.InstanceStateNameRunning {

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

func isInstanceNotFound(err error) bool {
	if aerr, ok := err.(awserr.Error); ok {
		if aerr.Code() == "InvalidInstanceID.NotFound" {
			return true
		}
	}
	return false
}

func (a *autoScalingGroup) getInstanceState(instanceID *string) int64 {
	svc := a.region.services.ec2
	input := &ec2.DescribeInstanceStatusInput{
		InstanceIds: []*string{
			instanceID,
		},
		IncludeAllInstances: aws.Bool(true),
	}

	out, err := svc.DescribeInstanceStatus(input)
	if err != nil {
		if isInstanceNotFound(err) {
			logger.Println("Instance not found:", *instanceID)
			return InstanceStateCodeTerminated
		}
		logger.Println("Error describing instance status:", *instanceID, err)
		return InstanceStateCodePending
	}

	if len(out.InstanceStatuses) > 0 {
		return *out.InstanceStatuses[0].InstanceState.Code
	}

	// If instance is not showing then it's terminated
	return InstanceStateCodeTerminated
}

func canTerminateInstance(instanceID *string, instanceState int64) bool {
	return instanceID != nil && instanceState != InstanceStateCodeTerminated && instanceState != InstanceStateCodeShuttingDown
}

func (a *autoScalingGroup) terminateInstance(instanceID *string, instanceState int64) bool {
	svc := a.region.services.ec2
	if canTerminateInstance(instanceID, instanceState) {
		_, err := svc.TerminateInstances(&ec2.TerminateInstancesInput{
			InstanceIds: []*string{instanceID},
		})
		if err != nil {
			return false
		}
	}
	return true
}

func (a *autoScalingGroup) cancelSIR(sirRequestID *string) error {
	svc := a.region.services.ec2
	_, err := svc.CancelSpotInstanceRequests(
		&ec2.CancelSpotInstanceRequestsInput{
			SpotInstanceRequestIds: []*string{sirRequestID},
		})

	return err
}

func (a *autoScalingGroup) cancelSIRAndTerminateInstance(instanceID *string, sirRequestID *string, instanceState int64) {

	if !a.terminateInstance(instanceID, instanceState) {
		logger.Println("Unable to terminate instance:", *instanceID)
		return
	}

	err := a.cancelSIR(sirRequestID)

	if err == nil {
		a.markSpotInstanceRequestAsCompete(sirRequestID)
	}
}

func (a *autoScalingGroup) processUnattachedRunningInstance(req *spotInstanceRequest, instanceID *string) (bool, bool) {
	logger.Println(a.name, "Active bid was found, with running "+
		"instances not yet attached to the ASG",
		*instanceID)
	// we need to re-scan in order to have the information a
	err := req.region.scanInstances()
	if err != nil {
		logger.Printf("Failed to scan instances: %s for %s\n", err, req.asg.name)
	}

	tags := req.asg.propagatedInstanceTags()

	i := req.region.instances.get(*instanceID)

	if i != nil {
		i.tag(tags, defaultTimeout)
	} else {
		logger.Println(req.asg.name, "new spot instance", *instanceID, "has disappeared")
	}
	return true, false
}

func isInstanceRunning(spotInstanceRunning int64) bool {
	return spotInstanceRunning == InstanceStateCodeRunning
}

func isInstanceInUsableState(spotInstanceRequest int64) bool {
	return spotInstanceRequest > InstanceStateCodeRunning
}

func (a *autoScalingGroup) processUnattachedInstance(req *spotInstanceRequest, instanceID *string) (bool, bool) {
	// var validRequest *spotInstanceRequest
	// processNextSIR, waitForNextExecution := false, false

	spotInstanceRunning := a.getInstanceState(instanceID)
	if isInstanceRunning(spotInstanceRunning) {
		return a.processUnattachedRunningInstance(req, instanceID)
	} else if isInstanceInUsableState(spotInstanceRunning) {
		// stopped, terminated, shutting-dowm etc,
		a.cancelSIRAndTerminateInstance(instanceID, req.SpotInstanceRequestId, spotInstanceRunning)
		// processNextSIR = true
		return false, false
	} else {
		logger.Println(a.name, "Active bid was found, with no running "+
			"instances, waiting for an instance to start ...")
		return true, true
	}

}

func (a *autoScalingGroup) processInstanceID(req *spotInstanceRequest, instanceID *string) (bool, bool) {
	//
	// If the instance is already in the group we should mark the
	// SIR as completed
	//
	if a.instances.get(*instanceID) != nil {
		logger.Println(a.name, "Instance", instanceID,
			"is already attached to the ASG, tagging SIR as complete and skipping...")
		a.markSpotInstanceRequestAsCompete(req.SpotInstanceRequestId)
		return false, false
		// In case the instance wasn't yet attached, we prepare to attach it.
	}

	logger.Println(a.name, "Instance", *instanceID,
		"is not yet attached to the ASG, checking if it's running")

	return a.processUnattachedInstance(req, instanceID)
}

func (a *autoScalingGroup) isSpotInstanceRequestClosedAndComplete(req *spotInstanceRequest) bool {
	isComplete := false
	switch *req.State {
	case "cancelled":
		if req.InstanceId == nil {
			isComplete = true
		}
	case "failed":
		fallthrough
	case "closed":
		isComplete = true
	}
	if isComplete {
		a.markSpotInstanceRequestAsCompete(req.SpotInstanceRequestId)
	}
	return isComplete
}

func (a *autoScalingGroup) isOpenOrActiveRequestWithNoInstance(req *spotInstanceRequest) bool {
	switch *req.State {
	case "active":
		fallthrough
	case "open":
		if req.InstanceId == nil {
			return true
		}
		return false
	default:
		return false
	}
}

func (a *autoScalingGroup) removeClosedOrCompleteRequests(requests []*spotInstanceRequest) []*spotInstanceRequest {
	var filtered []*spotInstanceRequest
	for _, req := range a.spotInstanceRequests {
		// If spot request is failed, closed or cancelled with no instances
		// continue to next SIR
		if a.isSpotInstanceRequestClosedAndComplete(req) {
			continue
		} else {
			filtered = append(filtered, req)
		}
	}
	return filtered
}

func (a *autoScalingGroup) filterRequestsWithRunningInstances(potentialRequests []*spotInstanceRequest) ([]*spotInstanceRequest, []*spotInstanceRequest) {
	var eligibleRequestsWithInstances []*spotInstanceRequest
	var eligibleRequestsWithNoInstances []*spotInstanceRequest

	for _, req := range potentialRequests {
		eligibleSIR, instanceNotRunning := a.processInstanceID(req, req.InstanceId)

		if eligibleSIR {
			if instanceNotRunning {
				// If instance isn't in asg, but is pending, wait for next run of autospotting
				eligibleRequestsWithNoInstances = append(eligibleRequestsWithNoInstances, req)
			}
			// If instance isn't in asg, but is running, use the SIR
			eligibleRequestsWithInstances = append(eligibleRequestsWithInstances, req)
		}
	}

	return eligibleRequestsWithInstances, eligibleRequestsWithNoInstances
}

func (a *autoScalingGroup) separateRequestsIntoThoseWithAndWithoutInstances(requests []*spotInstanceRequest) ([]*spotInstanceRequest, []*spotInstanceRequest) {
	var potentialRequestsWithInstances []*spotInstanceRequest

	var eligibleRequestsWithNoInstances []*spotInstanceRequest

	//
	// Here we search for spot requests created for the current ASG,
	// Use the first matching spot request with a instance ready for attaching
	//
	for _, req := range requests {
		if a.isOpenOrActiveRequestWithNoInstance(req) {
			eligibleRequestsWithNoInstances = append(eligibleRequestsWithNoInstances, req)
		} else {
			potentialRequestsWithInstances = append(potentialRequestsWithInstances, req)
		}
	}

	return potentialRequestsWithInstances, eligibleRequestsWithNoInstances
}

func (a *autoScalingGroup) findSpotInstanceRequest() (*spotInstanceRequest, bool) {

	// If spot request is failed, closed or cancelled with no instances
	// remove from list of eligible requests
	requests := a.removeClosedOrCompleteRequests(a.spotInstanceRequests)

	potentialRequestsWithInstances, eligibleRequestsWithNoInstances := a.separateRequestsIntoThoseWithAndWithoutInstances(requests)

	eligibleRequestsWithInstances, additionalEligibleRequestsWithNoInstances := a.filterRequestsWithRunningInstances(potentialRequestsWithInstances)

	for _, req := range additionalEligibleRequestsWithNoInstances {
		eligibleRequestsWithNoInstances = append(eligibleRequestsWithNoInstances, req)
	}

	if len(eligibleRequestsWithInstances) > 0 {
		return eligibleRequestsWithInstances[0], false
	}

	if len(eligibleRequestsWithNoInstances) > 0 {
		return eligibleRequestsWithNoInstances[0], true
	}

	return nil, false

}

// returns an instance ID as *string, the id of the spot request and a bool that
// tells us if  we need to wait for the next run in case there are spot
// instances still being launched
func (a *autoScalingGroup) havingReadyToAttachSpotInstance() (*string, *string, bool) {

	var activeSpotInstanceRequest *spotInstanceRequest

	// default we have found a spot request, don't create a new one
	waitForNextRun := false

	// if there are on-demand instances but no spot instance requests yet,
	// then we can launch a new spot instance
	if len(a.spotInstanceRequests) == 0 {
		logger.Println(a.name, "no spot bids were found")
		if inst := a.getAnyOnDemandInstance(); inst != nil {
			logger.Println(a.name, "on-demand instances were found, proceeding to "+
				"launch a replacement spot instance")
			return nil, nil, false
		}
		// Looks like we have no instances in the group, so we can stop here
		logger.Println(a.name, "no on-demand instances were found, nothing to do")
		return nil, nil, true
	}

	logger.Println("spot bids were found, continuing")

	// find a spot instance request
	activeSpotInstanceRequest, waitForNextRun = a.findSpotInstanceRequest()

	// In case we don't have any active spot requests with instances in the
	// process of starting or already ready to be attached to the group, we can
	// launch a new spot instance.
	if activeSpotInstanceRequest == nil {
		logger.Println(a.name, "No active unfulfilled bid was found")
		return nil, nil, waitForNextRun
	}

	spotInstanceID := activeSpotInstanceRequest.InstanceId

	if spotInstanceID == nil {
		logger.Println(a.name,
			"No instance was launched from the active spot instance request",
			*activeSpotInstanceRequest.SpotInstanceRequestId)
		return nil, nil, true
	}

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

		a.markSpotInstanceRequestAsCompete(activeSpotInstanceRequest.SpotInstanceRequestId)
		return nil, activeSpotInstanceRequest.SpotInstanceRequestId, true
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
		return nil, activeSpotInstanceRequest.SpotInstanceRequestId, true
	} else if *instData.State.Name == "pending" {
		logger.Println("The new spot instance", *spotInstanceID,
			"is still pending,",
			"waiting for it to be running before we can attach it to the group...")
		return nil, activeSpotInstanceRequest.SpotInstanceRequestId, true
	}
	return spotInstanceID, activeSpotInstanceRequest.SpotInstanceRequestId, false
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

	baseInstance, newInstanceType, err := a.getBaseAndNewInstanceTypeToStart(azToLaunchIn)
	if err != nil {
		return err
	}

	lc := a.getLaunchConfiguration()

	spotLS, err := lc.convertLaunchConfigurationToSpotSpecification(
		baseInstance,
		*newInstanceType,
		&a.region.services,
		*azToLaunchIn,
	)
	if err != nil {
		return fmt.Errorf("could not convert launchConfiguration to SpotSpecification: %s", err)
	}

	baseOnDemandPrice := baseInstance.price
	currentSpotPrice := newInstanceType.pricing.spot[*azToLaunchIn]

	logger.Println("Bidding for spot instance for ", a.name)
	return a.bidForSpotInstance(spotLS, a.getPricetoBid(baseOnDemandPrice, currentSpotPrice))
}

func (a *autoScalingGroup) getBaseAndNewInstanceTypeToStart(azToLaunchIn *string) (*instance, *instanceTypeInformation, error) {
	if azToLaunchIn == nil {
		logger.Println("Can't launch instances in any AZ, nothing to do here...")
		return nil, nil, errors.New("invalid availability zone provided")
	}

	logger.Println("Trying to launch spot instance in", *azToLaunchIn,
		"first finding an on-demand instance to use as a template")

	baseInstance := a.getOnDemandInstanceInAZ(azToLaunchIn)

	if baseInstance == nil {
		logger.Println("Found no on-demand instances, nothing to do here...")
		return nil, nil, errors.New("no on-demand instances found")
	}
	logger.Println("Found on-demand instance", *baseInstance.InstanceId)

	allowedInstances := a.getAllowedInstanceTypes(baseInstance)
	disallowedInstances := a.getDisallowedInstanceTypes(baseInstance)

	newInstanceTypeStr, err := baseInstance.getCheapestCompatibleSpotInstanceType(allowedInstances, disallowedInstances)
	if err != nil {
		logger.Println("No cheaper compatible instance type was found, "+
			"nothing to do here...", err)
		return nil, nil, errors.New("no cheaper spot instance found")
	}

	newInstanceType := a.region.instanceTypeInformation[newInstanceTypeStr]

	currentSpotPrice := newInstanceType.pricing.spot[*azToLaunchIn]
	logger.Println("Finished searching for best spot instance in ", *azToLaunchIn)
	logger.Println("Replacing an on-demand", *baseInstance.InstanceType,
		"instance having the ondemand price", baseInstance.price)
	logger.Println("Launching best compatible instance:", newInstanceType,
		"with the current spot price:", currentSpotPrice)

	return baseInstance, &newInstanceType, nil
}

func (a *autoScalingGroup) loadSpotInstanceRequest(
	req *ec2.SpotInstanceRequest) *spotInstanceRequest {
	return &spotInstanceRequest{SpotInstanceRequest: req,
		region: a.region,
		asg:    a,
	}
}

func createSpotInstanceRequestInput(price float64, ls *ec2.RequestSpotLaunchSpecification) *ec2.RequestSpotInstancesInput {
	var duration time.Duration
	duration = DefaultSecondsSpotRequestValidFor * time.Second
	validUtil := time.Now().Add(duration)
	return &ec2.RequestSpotInstancesInput{
		SpotPrice:           aws.String(strconv.FormatFloat(price, 'f', -1, 64)),
		LaunchSpecification: ls,
		ValidUntil:          &validUtil,
	}
}

func (a *autoScalingGroup) bidForSpotInstance(
	ls *ec2.RequestSpotLaunchSpecification,
	price float64,
) error {

	svc := a.region.services.ec2

	resp, err := svc.RequestSpotInstances(createSpotInstanceRequestInput(price, ls))

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
	// return sr.waitForAndTagSpotInstance()
	return nil
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
	if a.launchConfiguration != nil {
		return a.launchConfiguration
	}

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

	a.launchConfiguration = &launchConfiguration{
		LaunchConfiguration: resp.LaunchConfigurations[0],
	}
	return a.launchConfiguration
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

	// Wait till detachment initialize is complete before terminate instance
	time.Sleep(20 * time.Second * a.region.conf.SleepMultiplier)

	return a.instances.get(*instanceID).terminate()
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
