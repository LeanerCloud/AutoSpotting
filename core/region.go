package autospotting

import (
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/davecgh/go-spew/spew"
)

// data structure that stores information about a region
type region struct {
	name string

	conf *Config
	// The key in this map is the instance type.
	instanceTypeInformation map[string]instanceTypeInformation

	instances instances

	enabledASGs []autoScalingGroup
	services    connections

	wg sync.WaitGroup
}

type prices struct {
	onDemand     float64
	spot         spotPriceMap
	ebsSurcharge float64
}

// The key in this map is the availavility zone
type spotPriceMap map[string]float64

func (r *region) enabled() bool {

	var enabledRegions []string

	if r.conf.Regions != "" {
		// Allow both space- and comma-separated values for the region list.
		csv := strings.Replace(r.conf.Regions, " ", ",", -1)
		enabledRegions = strings.Split(csv, ",")
	} else {
		return true
	}

	for _, region := range enabledRegions {

		// glob matching for region names
		if match, _ := filepath.Match(region, r.name); match {
			return true
		}
	}

	return false
}

func (r *region) processRegion() {

	logger.Println("Creating connections to the required AWS services in", r.name)
	r.services.connect(r.name)
	// only process the regions where we have AutoScaling groups set to be handled

	logger.Println("Scanning for enabled AutoScaling groups in ", r.name)
	r.scanForEnabledAutoScalingGroups()

	// only process further the region if there are any enabled autoscaling groups
	// within it
	if r.hasEnabledAutoScalingGroups() {

		logger.Println("Scanning full instance information in", r.name)
		r.determineInstanceTypeInformation(r.conf)

		debug.Println(spew.Sdump(r.instanceTypeInformation))

		logger.Println("Scanning instances in", r.name)
		err := r.scanInstances()
		if err != nil {
			logger.Printf("Failed to scan instances in %s error: %s\n", r.name, err)
		}

		logger.Println("Processing enabled AutoScaling groups in", r.name)
		r.processEnabledAutoScalingGroups()
	} else {
		logger.Println(r.name, "has no enabled AutoScaling groups")
	}
}

func (r *region) scanInstances() error {
	svc := r.services.ec2
	input := &ec2.DescribeInstancesInput{
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

	r.instances = makeInstances()

	pageNum := 0
	err := svc.DescribeInstancesPages(
		input,
		func(page *ec2.DescribeInstancesOutput, lastPage bool) bool {
			pageNum++
			logger.Println("Processing page", pageNum, "of DescribeInstancesPages for", r.name)

			debug.Println(page)
			if len(page.Reservations) > 0 &&
				page.Reservations[0].Instances != nil {

				for _, res := range page.Reservations {
					for _, inst := range res.Instances {
						r.addInstance(inst)
					}
				}
			}
			return true
		},
	)

	if err != nil {
		return err
	}

	debug.Println(r.instances.dump())

	return nil
}

func (r *region) addInstance(inst *ec2.Instance) {
	r.instances.add(&instance{
		Instance: inst,
		typeInfo: r.instanceTypeInformation[*inst.InstanceType],
		region:   r,
	})
}

func (r *region) determineInstanceTypeInformation(cfg *Config) {

	r.instanceTypeInformation = make(map[string]instanceTypeInformation)

	var info instanceTypeInformation

	for _, it := range *cfg.InstanceData {

		var price prices

		debug.Println(it)

		// populate on-demand information
		price.onDemand = it.Pricing[r.name].Linux.OnDemand * cfg.OnDemandPriceMultiplier
		price.spot = make(spotPriceMap)
		price.ebsSurcharge = it.Pricing[r.name].EBSSurcharge

		// if at this point the instance price is still zero, then that
		// particular instance type doesn't even exist in the current
		// region, so we don't even need to create an empty spot pricing
		// data structure for it
		if price.onDemand > 0 {
			// for each instance type populate the HW spec information
			info = instanceTypeInformation{
				instanceType:        it.InstanceType,
				vCPU:                it.VCPU,
				memory:              it.Memory,
				GPU:                 it.GPU,
				pricing:             price,
				virtualizationTypes: it.LinuxVirtualizationTypes,
				hasEBSOptimization:  it.EBSOptimized,
			}

			if it.Storage != nil {
				info.hasInstanceStore = true
				info.instanceStoreDeviceSize = it.Storage.Size
				info.instanceStoreDeviceCount = it.Storage.Devices
				info.instanceStoreIsSSD = it.Storage.SSD
			}
			debug.Println(info)
			r.instanceTypeInformation[it.InstanceType] = info
		}
	}
	// this is safe to do once outside of the loop because the call will only
	// return entries about the available instance types, so no invalid instance
	// types would be returned

	if err := r.requestSpotPrices(); err != nil {
		logger.Println(err.Error())
	}

	debug.Println(spew.Sdump(r.instanceTypeInformation))
}

func (r *region) requestSpotPrices() error {

	s := spotPrices{conn: r.services}

	// Retrieve all current spot prices from the current region.
	// TODO: add support for other OSes
	err := s.fetch("Linux/UNIX", 0, nil, nil)

	if err != nil {
		return errors.New("Couldn't fetch spot prices in " + r.name)
	}

	// logger.Println("Spot Price list in ", r.name, ":\n", s.data)

	for _, priceInfo := range s.data {

		instType, az := *priceInfo.InstanceType, *priceInfo.AvailabilityZone

		// failure to parse this means that the instance is not available on the
		// spot market
		price, err := strconv.ParseFloat(*priceInfo.SpotPrice, 64)
		if err != nil {
			logger.Println(r.name, "Instance type ", instType,
				"is not available on the spot market")
			continue
		}

		if r.instanceTypeInformation[instType].pricing.spot == nil {
			logger.Println(r.name, "Instance data missing for", instType, "in", az,
				"skipping because this region is currently not supported")
			continue
		}

		r.instanceTypeInformation[instType].pricing.spot[az] = price

	}

	return nil
}

func (r *region) requestSpotInstanceTypes() ([]string, error) {

	var instTypes []string

	s := spotPrices{conn: r.services}

	// Retrieve all current spot prices from the current region.
	// TODO: add support for other OSes
	err := s.fetch("Linux/UNIX", 0, nil, nil)

	if err != nil {
		return nil, errors.New("Couldn't fetch spot prices in " + r.name)
	}

	for _, priceInfo := range s.data {
		instTypes = append(instTypes, *priceInfo.InstanceType)
	}

	return instTypes, nil
}

func (r *region) scanForEnabledAutoScalingGroupsByTag() []*string {
	svc := r.services.autoScaling

	var asgs []*string

	input := autoscaling.DescribeTagsInput{
		Filters: []*autoscaling.Filter{
			{Name: aws.String("key"), Values: []*string{aws.String("spot-enabled")}},
			{Name: aws.String("value"), Values: []*string{aws.String("true")}},
		},
	}
	pageNum := 0
	err := svc.DescribeTagsPages(
		&input,
		func(page *autoscaling.DescribeTagsOutput, lastPage bool) bool {
			pageNum++
			logger.Println("Processing page", pageNum, "of DescribeTagsPages for", r.name)
			for _, tag := range page.Tags {
				logger.Println(r.name, "has enabled ASG:", *tag.ResourceId)
				asgs = append(asgs, tag.ResourceId)
			}
			return true
		},
	)
	if err != nil {
		logger.Println("Failed to describe AutoScaling tags in",
			r.name,
			err.Error())
	}
	return asgs
}

func (r *region) scanForEnabledAutoScalingGroups() {

	asgNames := r.scanForEnabledAutoScalingGroupsByTag()

	if len(asgNames) == 0 {
		return
	}

	pageNum := 0
	err := r.services.autoScaling.DescribeAutoScalingGroupsPages(
		&autoscaling.DescribeAutoScalingGroupsInput{},
		func(page *autoscaling.DescribeAutoScalingGroupsOutput, lastPage bool) bool {
			pageNum++
			logger.Println("Processing page", pageNum, "of DescribeAutoScalingGroupsPages for", r.name)
			for _, asg := range page.AutoScalingGroups {
				group := autoScalingGroup{
					Group:  asg,
					name:   *asg.AutoScalingGroupName,
					region: r,
				}
				if containsString(asgNames, group.name) {
					r.enabledASGs = append(r.enabledASGs, group)
				}
			}
			return true
		},
	)

	if err != nil {
		logger.Println("Failed to describe AutoScaling groups in",
			r.name,
			err.Error())
		return
	}
}

func containsString(list []*string, a string) bool {
	for _, b := range list {
		if *b == a {
			return true
		}
	}
	return false
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
