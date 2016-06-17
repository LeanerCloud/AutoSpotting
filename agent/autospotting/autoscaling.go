package autospotting

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
)

type autoScalingGroup struct {
	name   string
	region *region

	asgRawData *autoscaling.Group

	// spot instance requests generated for the current group
	spotInstanceRequests []*ec2.SpotInstanceRequest
}

func (a *autoScalingGroup) create(region *region, asg *autoscaling.Group) {
	a.name = *asg.AutoScalingGroupName
	a.region = region
	a.asgRawData = asg

}

func (a *autoScalingGroup) process() {

	fmt.Println("Finding spot instance requests created for", a.name)
	a.spotInstanceRequests = a.region.findSpotInstanceRequests(a.name)

	spotInstanceID, waitForNextRun := a.havingReadyToAttachSpotInstance()

	if waitForNextRun == true {
		fmt.Println("Waiting for next run, spot instances are still in grace period for", a.name)
		return
	}

	if spotInstanceID != nil {
		fmt.Println("Replacing on-demand instance", *spotInstanceID, "from", a.name)

		a.replaceOnDemandInstanceWithSpot(*spotInstanceID)
	} else {
		var azToLaunchSpotIn *string

		if a.hasEqualAvailibilityZones() {
			fmt.Println(a.region.name, a.name, "AZs are equal in size")

			azToLaunchSpotIn = a.biggestOnDemandAvailablityZone()
		} else {
			azToLaunchSpotIn = a.smallestAvailablityZone()
		}

		if azToLaunchSpotIn == nil {
			fmt.Println(a.region.name, a.name, "No AZ can be used for launching new instances, nothing to do here...")
		} else {
			fmt.Println(a.region.name, a.name, "Would launch a spot instance in ", *azToLaunchSpotIn)
		}
		a.launchCheapestSpotInstance(azToLaunchSpotIn)
	}
}

func (a *autoScalingGroup) filterInstanceTags() []*ec2.Tag {
	var filteredTags []*ec2.Tag

	tags := a.getInstanceTags()

	fmt.Println("Unfiltered tags:", tags)
	// filtering reserved tags, which start with the "aws:" prefix
	for _, tag := range tags {
		if !strings.HasPrefix(*tag.Key, "aws:") {
			filteredTags = append(filteredTags, tag)
		}
	}

	fmt.Println("Filtered tags:", filteredTags)
	return filteredTags
}

func (a *autoScalingGroup) replaceOnDemandInstanceWithSpot(spotInstanceID string) {

	a.region.tagInstance(spotInstanceID, a.filterInstanceTags())

	biggestAZ := a.biggestAvailablityZone()
	biggestODAZ := a.biggestOnDemandAvailablityZone()

	asg := a.asgRawData

	minSize, desiredCapacity, maxSize := *asg.MinSize, *asg.DesiredCapacity, *asg.MaxSize

	// temporarily increase AutoScaling group in case it's fixed
	if minSize == maxSize {
		a.setAutoScalingMaxSize(maxSize + 1)
		defer a.setAutoScalingMaxSize(maxSize)
	}

	// revert attach/detach order when running on minimum capacity
	if desiredCapacity == minSize {
		a.attachSpotInstance(spotInstanceID)
	} else {
		defer a.attachSpotInstance(spotInstanceID)
	}

	// when all availability zones are equal, terminate an on-demand instance from where we have the most of them
	// otherwise terminate one from the biggest availability zone
	// TODO: make sure we actually have instances there
	if a.hasEqualAvailibilityZones() {
		a.detachAndTerminateOnDemandInstance(*biggestODAZ)
	} else {
		a.detachAndTerminateOnDemandInstance(*biggestAZ)
	}

}

func (a *autoScalingGroup) getInstanceTags() []*ec2.Tag {
	instance := a.findInstance()
	if instance != nil {
		return instance.Tags
	}
	return nil
}

// returns the first instance we could find
func (a *autoScalingGroup) findInstance() *ec2.Instance {

	for _, instance := range a.asgRawData.Instances {
		return a.region.instances[*instance.InstanceId]
	}
	return nil
}

func (a *autoScalingGroup) findOndemandInstance() *ec2.Instance {

	for _, instance := range a.asgRawData.Instances {
		instanceData := a.region.instances[*instance.InstanceId]
		// this attribute is non-nil only for spot instances, where it contains the value "spot"
		if instanceData != nil && instanceData.InstanceLifecycle == nil {

			return instanceData
		}
	}
	return nil
}

// returns an instance ID as *string and a bool that tells us if  we need to wait
// for the next run in case there are spot instances still being launched
func (a *autoScalingGroup) havingReadyToAttachSpotInstance() (*string, bool) {

	var activeSpotInstanceRequest *ec2.SpotInstanceRequest

	if len(a.spotInstanceRequests) == 0 {
		fmt.Println("No spot bids were found")
		return nil, true
	}

	for _, req := range a.spotInstanceRequests {
		if *req.State == "open" || *req.State == "failed" {
			fmt.Println("Open or failed bids found, waiting for the next run...")
			return nil, true
		}

		if *req.State == "active" && *req.Status.Code == "fulfilled" {
			if a.hasInstance(*req.InstanceId) {
				fmt.Println("Active bid was found, with instance already attached to the ASG, skipping...")
				continue
			} else {
				if *a.region.instances[*req.InstanceId].State.Name == "running" {
					fmt.Println("Active bid was found, with running instances not yet attached to the ASG", *req.InstanceId)
					activeSpotInstanceRequest = req
					break
				} else {
					fmt.Println("Active bid was found, with non-running instances")
					continue
				}
			}
		}
	}

	// in this case we can launch a new spot instance if we can
	if activeSpotInstanceRequest == nil {
		fmt.Println("No active unfulfilled bid was found")
		return nil, false
	}

	// Show information about the current spot instance
	spotInstanceID := activeSpotInstanceRequest.InstanceId
	fmt.Println("Considering ", *spotInstanceID, "for attaching to", a.name)

	instData := a.region.instances[*spotInstanceID]

	// check if the spot instance is out of the grace period, so
	// in that case we can replace an on-demand instance with it
	if *instData.State.Name == "running" && (time.Now().Unix()-instData.LaunchTime.Unix()) < *a.asgRawData.HealthCheckGracePeriod {
		fmt.Println("The new spot instance", *spotInstanceID, "is still in the grace period,  waiting for the next run...")
		return nil, true
	}
	return spotInstanceID, false
}

func (a *autoScalingGroup) hasInstance(instanceID string) bool {
	for _, inst := range a.asgRawData.Instances {
		if *inst.InstanceId == instanceID {
			return true
		}
	}
	return false
}

func (a *autoScalingGroup) hasEqualAvailibilityZones() bool {

	var azInstanceCount = make(map[string]int)

	asg := a.asgRawData
	min, max := math.MaxInt32, 0

	for _, az := range asg.AvailabilityZones {
		azInstanceCount[*az] = 0
	}

	for _, instance := range asg.Instances {
		azInstanceCount[*instance.AvailabilityZone]++
	}

	for _, v := range azInstanceCount {
		if v <= min {
			min = v
		}
		if v >= max {
			max = v
		}

	}

	result := (min == max)
	fmt.Println("Checking if all AZs of ", a.name, " are equal in size: ", result)
	return result
}

// returns the zone with the smallest total number of instances, where we can launch
// new spot instances without risking of them being terminated by the automated
// re-balancing operations
func (a *autoScalingGroup) smallestAvailablityZone() *string {
	var azInstanceCount = make(map[string]int)
	var smallestAZ string
	min := math.MaxInt32

	fmt.Println(a.region.name, "Getting smallest AZ in", a.name)
	for _, az := range a.asgRawData.AvailabilityZones {
		azInstanceCount[*az] = 0
	}

	for _, instance := range a.asgRawData.Instances {
		azInstanceCount[*instance.AvailabilityZone]++
	}

	fmt.Println(azInstanceCount)

	for k, v := range azInstanceCount {
		if v <= min {
			smallestAZ, min = k, v
		}
	}
	fmt.Println("Smallest AZ is ", smallestAZ)
	return &smallestAZ
}

// returns the AZ with the highest total number of instances
func (a *autoScalingGroup) biggestAvailablityZone() *string {
	fmt.Println(a.region.name, "Getting biggest AZ in", a.name)
	var azInstanceCount = make(map[string]int)
	var biggestAZ *string
	max := 0

	for _, instance := range a.asgRawData.Instances {
		azInstanceCount[*instance.AvailabilityZone]++
	}

	for k, v := range azInstanceCount {
		if max <= v {
			biggestAZ, max = &k, v
		}
	}

	fmt.Println("Biggest AZ is ", *biggestAZ)
	return biggestAZ
}

// returns the AZ with the highest number of on-demand instances,
// this is useful for terminating on-demand instances from it in case
// all the AZs are equal and we would otherwise brask the balance between them
func (a *autoScalingGroup) biggestOnDemandAvailablityZone() *string {
	fmt.Println(a.region.name, "Getting biggest OnDemand AZ from ", a.name)

	var azInstanceCount = make(map[string]int)
	var biggestAZ *string
	max := 0

	for _, instance := range a.asgRawData.Instances {
		// only spot instances have this field set to nil
		if a.region.instances[*instance.InstanceId].InstanceLifecycle == nil {
			azInstanceCount[*instance.AvailabilityZone]++
		}
	}

	for k, v := range azInstanceCount {
		if max <= v {
			biggestAZ, max = &k, v
		}
	}
	if biggestAZ != nil {
		fmt.Println("Biggest OnDemand AZ is ", *biggestAZ)
	}
	return biggestAZ
}

func (a *autoScalingGroup) launchCheapestSpotInstance(azToLaunchIn *string) {

	if azToLaunchIn == nil {
		fmt.Println("No AZ can be used to launch instances, nothing to do here...")
		return
	}

	fmt.Println("Trying to launch spot instance in", *azToLaunchIn, "\nfirst finding an on-demand instance to use as a template")

	baseInstance := a.findOndemandInstance()

	if baseInstance == nil {
		fmt.Println("Found no on-demand instances, nothing to do here...")
		return
	}
	fmt.Println("Found on-demand instance", *baseInstance.InstanceId)

	newInstanceType := a.getCheapestCompatibleSpotInstanceType(*azToLaunchIn, baseInstance)

	if newInstanceType == nil {
		fmt.Println("No cheaper compatible instance type was found, nothing to do here...")
		return
	}

	baseOnDemandPrice := a.region.instanceData[*baseInstance.InstanceType].pricing.onDemand

	currentSpotPrice := a.region.instanceData[*newInstanceType].pricing.spot[*azToLaunchIn]

	fmt.Println("Searching for best spot instance in ", *azToLaunchIn,
		"\nreplacing on-demand", *baseInstance.InstanceType, "instances",
		"with the ondemand price", baseOnDemandPrice,
		"\nLaunching best compatible instance:", *newInstanceType,
		"current spot price:", currentSpotPrice)
	lc := a.getLaunchConfiguration()

	spotLS := convertLaunchConfigurationToSpotSpecification(lc,
		*newInstanceType,
		*azToLaunchIn)

	fmt.Println("Bidding for spot instance for ", a.name)
	a.bidForSpotInstance(spotLS, baseOnDemandPrice)
}

func (a *autoScalingGroup) setAutoScalingMaxSize(maxSize int64) {
	svc := a.region.services.autoScaling

	resp, err := svc.UpdateAutoScalingGroup(&autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: aws.String(a.name),
		MaxSize:              aws.Int64(maxSize),
	})

	if err != nil {
		// Print the error, cast err to awserr.Error to get the Code and
		// Message from an error.
		fmt.Println(err.Error())
		return
	}

	// Pretty-print the response data.
	fmt.Println(resp)
}

func (a *autoScalingGroup) bidForSpotInstance(ls *ec2.RequestSpotLaunchSpecification, price float64) {
	svc := a.region.services.ec2

	resp, err := svc.RequestSpotInstances(&ec2.RequestSpotInstancesInput{
		SpotPrice: aws.String(strconv.FormatFloat(price, 'f', -1, 64)), // Required

		//ClientToken:           aws.String("String"),
		LaunchSpecification: ls,
	})

	if err != nil {
		fmt.Println("Failed to create spot instance request for ", a.name, err.Error())
		return
	}

	// Pretty-print the response data.
	fmt.Println(resp)

	a.tagSpotInstanceRequest(*resp.SpotInstanceRequests[0].SpotInstanceRequestId)
}

func (a *autoScalingGroup) tagSpotInstanceRequest(requestID string) {
	svc := a.region.services.ec2

	resp, err := svc.CreateTags(&ec2.CreateTagsInput{
		Resources: []*string{aws.String(requestID)},
		Tags: []*ec2.Tag{
			{
				Key:   aws.String("launched-for-asg"),
				Value: aws.String(a.name),
			},
		},
	})

	if err != nil {
		// Print the error, cast err to awserr.Error to get the Code and
		// Message from an error.
		fmt.Println("Failed to create tags for the spot instance request", err.Error())
		return
	}

	// Pretty-print the response data.
	fmt.Println(resp)
}

func (a *autoScalingGroup) getLaunchConfiguration() *autoscaling.LaunchConfiguration {

	lcName := a.asgRawData.LaunchConfigurationName

	svc := a.region.services.autoScaling

	params := &autoscaling.DescribeLaunchConfigurationsInput{
		LaunchConfigurationNames: []*string{lcName},
	}
	resp, err := svc.DescribeLaunchConfigurations(params)

	if err != nil {
		fmt.Println(err.Error())
		return nil
	}

	return resp.LaunchConfigurations[0]
}

func convertLaunchConfigurationToSpotSpecification(lc *autoscaling.LaunchConfiguration,
	instanceType string,
	az string) *ec2.RequestSpotLaunchSpecification {

	var spotLS ec2.RequestSpotLaunchSpecification

	fmt.Println("Launch configuration to convert", lc)

	// convert attributes
	spotLS.BlockDeviceMappings = copyBlockDeviceMappings(lc.BlockDeviceMappings)
	spotLS.EbsOptimized = lc.EbsOptimized

	// lc.IamInstanceProfile can be either an ID or ARN, so we have to see which one is it
	if lc.IamInstanceProfile != nil {
		if strings.HasPrefix(*lc.IamInstanceProfile, "arn:") {
			spotLS.IamInstanceProfile.Arn = lc.IamInstanceProfile
		} else {
			spotLS.IamInstanceProfile.Name = lc.IamInstanceProfile
		}
	}

	spotLS.ImageId = lc.ImageId

	spotLS.InstanceType = &instanceType

	// these shouldn't be copied, they break the SpotLaunchSpecification
	//spotLS.KernelId = lc.KernelId
	//spotLS.RamdiskId = lc.RamdiskId
	spotLS.KeyName = lc.KeyName

	spotLS.Monitoring = &ec2.RunInstancesMonitoringEnabled{Enabled: lc.InstanceMonitoring.Enabled}

	/* todo
	   IamInstanceProfile *IamInstanceProfileSpecification  // needs to be created from a string
	   IamInstanceProfile *string

	   InstanceMonitoring *InstanceMonitoring // to be converted

	   Monitoring *RunInstancesMonitoringEnabled

	   NetworkInterfaces []*InstanceNetworkInterfaceSpecification

	   Placement *SpotPlacement

	   SecurityGroupIds []*string

	   SpotPrice *string
	   SubnetId *string
	   The placement information for the instance. - contains AZ

	*/

	/*
	   var network ec2.InstanceNetworkInterfaceSpecification
	   network.AssociatePublicIpAddress = lc.AssociatePublicIpAddress

	   spotLS.NetworkInterfaces = append(spotLS.NetworkInterfaces, &network)
	*/

	//spotLS.SecurityGroups = lc.SecurityGroups
	spotLS.SecurityGroupIds = lc.SecurityGroups
	spotLS.UserData = lc.UserData
	spotLS.Placement = &ec2.SpotPlacement{AvailabilityZone: &az}

	fmt.Println(spotLS)

	//lc.InstanceType = instanceType

	fmt.Println("Launch configuration", lc, "\n\nconverted to spot configuration", spotLS)

	return &spotLS

}

func copyBlockDeviceMappings(lcBDMs []*autoscaling.BlockDeviceMapping) []*ec2.BlockDeviceMapping {
	var ec2BDMlist []*ec2.BlockDeviceMapping
	var ec2BDM ec2.BlockDeviceMapping

	for _, lcBDM := range lcBDMs {
		ec2BDM.DeviceName = lcBDM.DeviceName

		ec2BDM.Ebs = &ec2.EbsBlockDevice{
			DeleteOnTermination: lcBDM.Ebs.DeleteOnTermination,
			Encrypted:           lcBDM.Ebs.Encrypted,
			Iops:                lcBDM.Ebs.Iops,
			SnapshotId:          lcBDM.Ebs.SnapshotId,
			VolumeSize:          lcBDM.Ebs.VolumeSize,
			VolumeType:          lcBDM.Ebs.VolumeType,
		}
		var noDevice string
		if lcBDM.NoDevice != nil {
			noDevice = fmt.Sprintf("%t", *lcBDM.NoDevice)
			ec2BDM.NoDevice = &noDevice
		}

		ec2BDM.VirtualName = lcBDM.VirtualName

		fmt.Println("LCBDM:", lcBDM, "\n", "EC2BDM", ec2BDM, "\n appending to list")

		ec2BDMlist = append(ec2BDMlist, &ec2BDM)

	}
	return ec2BDMlist
}

func (a *autoScalingGroup) attachSpotInstance(spotInstanceID string) {

	svc := a.region.services.autoScaling

	params := autoscaling.AttachInstancesInput{
		AutoScalingGroupName: aws.String(a.name),
		InstanceIds: []*string{
			&spotInstanceID,
		},
	}

	resp, err := svc.AttachInstances(&params)

	if err != nil {
		fmt.Println(err.Error())
		// Pretty-print the response data.
		fmt.Println(resp)
	}

}

// Terminates an on-demand instance from the given availability zone,
// but only after it was detached from the autoscaling group
func (a *autoScalingGroup) detachAndTerminateOnDemandInstance(az string) {
	for _, inst := range a.asgRawData.Instances {
		if *inst.AvailabilityZone == az {

			instDetails := a.region.instances[*inst.InstanceId]

			// skip spot instances
			if instDetails.InstanceLifecycle != nil && *instDetails.InstanceLifecycle == "spot" {
				continue
			}

			detachParams := autoscaling.DetachInstancesInput{
				AutoScalingGroupName: aws.String(a.name),
				InstanceIds: []*string{
					inst.InstanceId,
				},
				ShouldDecrementDesiredCapacity: aws.Bool(true),
			}

			asSvc := a.region.services.autoScaling
			asResp, err := asSvc.DetachInstances(&detachParams)
			fmt.Println(asResp)
			if err != nil {
				fmt.Println(err.Error())
			}

			ec2Svc := a.region.services.ec2

			termParams := ec2.TerminateInstancesInput{
				InstanceIds: []*string{
					inst.InstanceId,
				},
			}

			ec2Resp, err := ec2Svc.TerminateInstances(&termParams)
			fmt.Println(ec2Resp)
			if err != nil {
				fmt.Println(err.Error())
			}
			return // we can exit after terminating a single instance from that AZ
		}

	}

}

func (a *autoScalingGroup) getCheapestCompatibleSpotInstanceType(availabilityZone string, baseInstance *ec2.Instance) *string {

	fmt.Println("Getting cheapest spot instance compatible to ",
		*baseInstance.InstanceId, " of type", *baseInstance.InstanceType)

	filteredInstances := a.getCompatibleSpotInstanceTypes(availabilityZone, baseInstance)

	minPrice := math.MaxFloat64
	var chosenInstanceType *string

	for _, instance := range filteredInstances {
		price := a.region.instanceData[instance].pricing.spot[availabilityZone]
		if price < minPrice {
			minPrice = price
			chosenInstanceType = &instance
		}
	}

	if chosenInstanceType != nil {
		fmt.Println("Chose cheapest instance type", *chosenInstanceType)
	} else {
		fmt.Println("Couldn't find any cheaper spot instance type")

	}

	return chosenInstanceType

}

func (a *autoScalingGroup) getCompatibleSpotInstanceTypes(availabilityZone string, baseInstance *ec2.Instance) []string {

	fmt.Println("Getting spot instances compatible to ",
		*baseInstance.InstanceId, " of type", *baseInstance.InstanceType)

	var filteredInstanceTypes []string

	refInstance := a.region.instanceData[*baseInstance.InstanceType]
	fmt.Println("Using this data as reference", refInstance)

	//filtering compatible instance types
	for _, inst := range a.region.instanceData {

		fmt.Println("\nComparing ", inst, " with ", refInstance)

		spotPriceNewInstance := inst.pricing.spot[availabilityZone]
		onDemandPriceExistingInstance := refInstance.pricing.onDemand

		if spotPriceNewInstance == 0 {
			fmt.Println("Missing spot pricing information, skipping", inst.instanceType)
			continue
		}

		if spotPriceNewInstance <= onDemandPriceExistingInstance {
			fmt.Println("pricing compatible, continuing evaluation: ", inst.pricing.spot[availabilityZone], "<=", refInstance.pricing.onDemand)
		} else {
			fmt.Println("pricing excessive, skipping", inst.instanceType)
			continue
		}

		if inst.vCPU >= refInstance.vCPU {
			fmt.Println("CPU compatible, continuing evaluation")
		} else {
			fmt.Println("CPU incompatible, skipping ", inst.instanceType)
			continue
		}

		if inst.memory >= refInstance.memory {
			fmt.Println("memory compatible, continuing evaluation")
		} else {
			fmt.Println("memory incompatible, skipping", inst.instanceType)
			continue
		}

		if compatibleVirtualization(*baseInstance.VirtualizationType, inst.virtualizationTypes) {
			fmt.Println("virtualization compatible, continuing evaluation")
		} else {
			fmt.Println("virtualization incompatible, skipping", inst.instanceType)
			continue
		}

		if !a.alreadyRunningSpotInstance(inst.instanceType, availabilityZone) {
			fmt.Println("no already-running spot instances, adding for comparison ", inst.instanceType)
			filteredInstanceTypes = append(filteredInstanceTypes, inst.instanceType)
		} else {
			fmt.Println("\nInstances ", inst, " and ", refInstance, " are not compatible")

		}

	}
	fmt.Printf("\n Found following compatible instances: %#v\n", filteredInstanceTypes)
	return filteredInstanceTypes

}

func compatibleVirtualization(virtualizationType string, availableVirtualizationTypes []string) bool {
	fmt.Println("Available: ", availableVirtualizationTypes, "Tested: ", virtualizationType)

	for _, avt := range availableVirtualizationTypes {
		if (avt == "PV") && (virtualizationType == "paravirtual") ||
			(avt == "HVM") && (virtualizationType == "hvm") {
			fmt.Println("Compatible")
			return true
		}
	}
	return false
}

func (a *autoScalingGroup) alreadyRunningSpotInstance(instanceType, availabilityZone string) bool {

	fmt.Println("Checking if not already running spot instances of type ",
		instanceType, " in AZ ", availabilityZone)
	for _, instDetails := range a.region.instances {
		if *instDetails.InstanceType == instanceType &&
			*instDetails.Placement.AvailabilityZone == availabilityZone &&
			instDetails.InstanceLifecycle != nil &&
			*instDetails.InstanceLifecycle == "spot" {
			fmt.Println("Found running spot instance ", *instDetails.InstanceId, "of the same type:", instanceType)
			return true
		}
	}
	fmt.Println("Found no spot instance of the type:", instanceType)
	return false
}
