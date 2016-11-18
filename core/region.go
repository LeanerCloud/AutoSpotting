package autospotting

import (
	"strconv"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/autoscaling"
)

// data structure that stores information about a region
type region struct {
	name string

	cfg Config
	// The key in this map is the instance type.
	instanceData map[string]instanceInformation

	// The key in this map is the instance ID, useful for quick retrieval of
	// instance attributes.
	instances map[string]*ec2.Instance

	//
	enabledASGs []autoScalingGroup
	services    connections
	wg          sync.WaitGroup
}

type instanceInformation struct {
	instanceType             string
	vCPU                     int
	pricing                  prices
	memory                   float32
	virtualizationTypes      []string
	hasInstanceStore         bool
	instanceStoreDeviceSize  float32
	instanceStoreDeviceCount int
	instanceStoreIsSSD       bool
}

type prices struct {
	onDemand float64
	spot     spotPriceMap
}

// The key in this map is the availavility zone
type spotPriceMap map[string]float64

func (r *region) processRegion(cfg Config) {
	logger.Println("Creating connections to the required AWS services in", r.name)
	r.services.connect(r.name)
	// only process the regions where we have AutoScaling groups set to be handled

	logger.Println("Scanning for enabled AutoScaling groups in ", r.name)
	r.scanForEnabledAutoScalingGroups()

	// only process further the region if there are any enabled autoscaling groups
	// within it
	if r.hasEnabledAutoScalingGroups() {
		logger.Println("Scanning instances in", r.name)
		r.scanInstances()

		logger.Println("Scanning full instance information in", r.name)
		r.determineInstanceInformation(cfg)

		logger.Println("Processing enabled AutoScaling groups in", r.name)
		r.processEnabledAutoScalingGroups()
	} else {
		logger.Println(r.name, "has no enabled AutoScaling groups")
	}
}

func (r *region) determineInstanceInformation(cfg Config) {

	r.instanceData = make(map[string]instanceInformation)

	var info instanceInformation

	for _, it := range cfg.InstanceData {

		var price prices

		// populate on-demand information
		price.onDemand, _ = strconv.ParseFloat(
			it.Pricing[r.name].Linux.OnDemand, 64)

		price.spot = make(spotPriceMap)

		// if at this point the instance price is still zero, then that
		// particular instance type doesn't even exist in the current
		// region, so we don't even need to create an empty spot pricing
		// data structure for it
		if price.onDemand > 0 {
			// for each instance type populate the HW spec information
			info = instanceInformation{
				instanceType:        it.InstanceType,
				vCPU:                it.VCPU,
				memory:              it.Memory,
				pricing:             price,
				virtualizationTypes: it.LinuxVirtualizationTypes,
			}

			if it.Storage != nil {
				info.hasInstanceStore = true
				info.instanceStoreDeviceSize = it.Storage.Size
				info.instanceStoreDeviceCount = it.Storage.Devices
				info.instanceStoreIsSSD = it.Storage.SSD
			}
			r.instanceData[it.InstanceType] = info
		}
	}
	// this is safe to do once outside of the loop because the call will only
	// return entries about the available instance types, so no invalid instance
	// types would be returned
	if err := r.requestSpotPrices(); err != nil {
		logger.Println(err.Error())
	}
	// logger.Printf("%#v\n", r.instanceData)
}

func (r *region) requestSpotPrices() error {

	logger.Println(r.name, "Requesting spot prices")
	ec2Conn := r.services.ec2
	params := &ec2.DescribeSpotPriceHistoryInput{
		ProductDescriptions: []*string{
			aws.String("Linux/UNIX"),
		},
		StartTime: aws.Time(time.Now()),
		EndTime:   aws.Time(time.Now()),
	}

	resp, err := ec2Conn.DescribeSpotPriceHistory(params)

	if err != nil {
		logger.Println(r.name, "Failed requesting spot prices:", err.Error())
		return err
	}

	spotPrices := resp.SpotPriceHistory

	// logger.Println("Spot Price list in ", r.name, ":\n", spotPrices)

	for _, priceInfo := range spotPrices {

		instType, az := *priceInfo.InstanceType, *priceInfo.AvailabilityZone

		// failure to parse this means that the instance is not available on the
		// spot market
		price, err := strconv.ParseFloat(*priceInfo.SpotPrice, 64)
		if err != nil {
			logger.Println(r.name, "Instance type ", instType,
				"is not available on the spot market")
			continue
		}

		if r.instanceData[instType].pricing.spot == nil {
			logger.Println(r.name, "Instance data missing for", instType, "in", az,
				"skipping because this region is currently not supported")
			continue
		}

		r.instanceData[instType].pricing.spot[az] = price

	}

	return nil
}

func (r *region) scanForEnabledAutoScalingGroupsByTag(asgs *[]*string) {
	svc := r.services.autoScaling

	input := autoscaling.DescribeTagsInput{
		Filters: []*autoscaling.Filter{
			{Name: aws.String("key"), Values: []*string{aws.String("spot-enabled")}},
			{Name: aws.String("value"), Values: []*string{aws.String("true")}},
		},
	}
	resp, err := svc.DescribeTags(&input)

	if err != nil {
		logger.Println("Failed to describe AutoScaling tags in",
			r.name,
			err.Error())
		return
	}

	for _, tag := range resp.Tags {
		logger.Println("Found enabled ASG:", *tag.ResourceId)
		*asgs = append(*asgs, tag.ResourceId)
	}
}

func (r *region) scanForEnabledAutoScalingGroups() {
	asgs := []*string{}

	r.scanForEnabledAutoScalingGroupsByTag(&asgs)

	if len(asgs) == 0 {
		return
	}

	svc := r.services.autoScaling

	input := autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: asgs,
	}
	resp, err := svc.DescribeAutoScalingGroups(&input)

	if err != nil {
		logger.Println("Failed to describe AutoScaling groups in",
			r.name,
			err.Error())
		return
	}

	for _, asg := range resp.AutoScalingGroups {
		group := autoScalingGroup{
			name:       *asg.AutoScalingGroupName,
			region:     r,
			asgRawData: asg,
		}
		r.enabledASGs = append(r.enabledASGs, group)
	}
}

func (r *region) hasEnabledAutoScalingGroups() bool {

	return len(r.enabledASGs) > 0

}

func (r *region) processEnabledAutoScalingGroups() {
	for _, asg := range r.enabledASGs {
		r.wg.Add(1)
		go func(a autoScalingGroup) {
			a.process()
			r.wg.Done()
		}(asg)
	}
	r.wg.Wait()
}

func (r *region) findSpotInstanceRequests(
	forAsgName string) []*ec2.SpotInstanceRequest {

	svc := r.services.ec2

	params := &ec2.DescribeSpotInstanceRequestsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("tag:launched-for-asg"),
				Values: []*string{aws.String(forAsgName)},
			},
		},
	}
	resp, err := svc.DescribeSpotInstanceRequests(params)

	if err != nil {
		logger.Println(err.Error())
		return nil
	}

	logger.Println("Spot instance requests were previously created for",
		forAsgName)
	return resp.SpotInstanceRequests

}

func (r *region) scanInstances() {
	svc := r.services.ec2
	params := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("instance-state-name"),
				Values: []*string{
					aws.String("running"),
					aws.String("pending"),
				},
			},
		},
	}

	resp, err := svc.DescribeInstances(params)
	if err != nil {
		logger.Println(err.Error())
		return
	}

	r.instances = make(map[string]*ec2.Instance)

	if len(resp.Reservations) > 0 &&
		resp.Reservations[0].Instances != nil {

		for _, res := range resp.Reservations {
			for _, inst := range res.Instances {
				r.instances[*inst.InstanceId] = inst
			}
		}
	}

	// logger.Println(r.instances)
}

func (r *region) tagInstance(instanceID *string, tags []*ec2.Tag) {

	if len(tags) == 0 {
		logger.Println(r.name, "Tagging spot instance", *instanceID,
			"no tags were defined, skipping...")
		return
	}

	svc := r.services.ec2
	params := ec2.CreateTagsInput{
		Resources: []*string{instanceID},
		Tags:      tags,
	}

	logger.Println(r.name, "Tagging spot instance", *instanceID)

	for _, err := svc.CreateTags(&params); err != nil; _, err =
		svc.CreateTags(&params) {

		logger.Println(r.name,
			"Failed to create tags for the spot instance", *instanceID, err.Error())

		logger.Println(r.name,
			"Sleeping for 5 seconds before retrying")

		time.Sleep(5 * time.Second)
	}

	logger.Println("Instance", *instanceID,
		"was tagged with the following tags:", tags)
}
