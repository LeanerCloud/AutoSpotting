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

// Tag represents an Asg Tag: Key, Value
type Tag struct {
	Key   string
	Value string
}

// data structure that stores information about a region
type region struct {
	name string

	conf *Config
	// The key in this map is the instance type.
	instanceTypeInformation map[string]instanceTypeInformation

	instances instances

	enabledASGs []autoScalingGroup
	services    connections

	tagsToFilterASGsBy []Tag

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

	// setup the filters for asg matching
	r.setupAsgFilters()

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

func (r *region) setupAsgFilters() {
	filters := replaceWhitespace(r.conf.FilterByTags)
	if len(filters) == 0 {
		r.tagsToFilterASGsBy = []Tag{{Key: "spot-enabled", Value: "true"}}
		return
	}

	for _, tagWithValue := range strings.Split(filters, ",") {
		tag := splitTagAndValue(tagWithValue)
		if tag != nil {
			r.tagsToFilterASGsBy = append(r.tagsToFilterASGsBy, *tag)
		}
	}

	if len(r.tagsToFilterASGsBy) == 0 {
		r.tagsToFilterASGsBy = []Tag{{Key: "spot-enabled", Value: "true"}}
	}
}

func replaceWhitespace(filters string) string {
	filters = strings.TrimSpace(filters)
	filters = strings.Replace(filters, " ", ",", -1)
	return filters
}

func splitTagAndValue(value string) *Tag {
	splitTagAndValue := strings.Split(value, "=")
	if len(splitTagAndValue) > 1 {
		return &Tag{Key: splitTagAndValue[0], Value: splitTagAndValue[1]}
	}
	return nil
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
	err := s.fetch(r.conf.SpotProductDescription, 0, nil, nil)

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

func tagsMatch(asgTag *autoscaling.TagDescription, filteringTag Tag) bool {
	return asgTag != nil && *asgTag.Key == filteringTag.Key && *asgTag.Value == filteringTag.Value
}

func isASGWithMatchingTag(tagToMatch Tag, asgTags []*autoscaling.TagDescription) bool {
	for _, asgTag := range asgTags {
		if tagsMatch(asgTag, tagToMatch) {
			return true
		}
	}
	return false
}

func isASGWithMatchingTags(asg *autoscaling.Group, tagsToMatch []Tag) bool {
	matchedTags := 0

	for _, tag := range tagsToMatch {
		if asg != nil && isASGWithMatchingTag(tag, asg.Tags) {
			matchedTags++
		}
	}

	return matchedTags == len(tagsToMatch)
}

func (r *region) findMatchingASGsInPageOfResults(groups []*autoscaling.Group,
	tagsToMatch []Tag) []autoScalingGroup {

	var asgs []autoScalingGroup
	var optInFilterMode = (r.conf.TagFilteringMode != "opt-out")

	for _, group := range groups {
		asgName := *group.AutoScalingGroupName
		groupMatchesExpectedTags := isASGWithMatchingTags(group, tagsToMatch)
		// Go lacks a logical XOR operator, this is the equivalent to that logical
		// expression. The goal is to add the matching ASGs when running in opt-in
		// mode and the other way round.
		if optInFilterMode != groupMatchesExpectedTags {
			logger.Printf("Skipping group %s because its tags, the currently "+
				"configured filtering mode (%s) and tag filters do not align\n",
				asgName, r.conf.TagFilteringMode)
			continue
		}

		logger.Printf("Enabling group %s for processing because its tags, the "+
			"currently configured  filtering mode (%s) and tag filters are aligned\n",
			asgName, r.conf.TagFilteringMode)
		asgs = append(asgs, autoScalingGroup{
			Group:  group,
			name:   asgName,
			region: r,
		})
	}
	return asgs
}

func (r *region) scanForEnabledAutoScalingGroups() {

	svc := r.services.autoScaling

	pageNum := 0
	err := svc.DescribeAutoScalingGroupsPages(
		&autoscaling.DescribeAutoScalingGroupsInput{},
		func(page *autoscaling.DescribeAutoScalingGroupsOutput, lastPage bool) bool {
			pageNum++
			logger.Println("Processing page", pageNum, "of DescribeAutoScalingGroupsPages for", r.name)
			matchingAsgs := r.findMatchingASGsInPageOfResults(page.AutoScalingGroups, r.tagsToFilterASGsBy)
			r.enabledASGs = append(r.enabledASGs, matchingAsgs...)
			return true
		},
	)

	if err != nil {
		logger.Println("Failed to describe AutoScalingGroups in", r.name, err.Error())
	}

}

func (r *region) hasEnabledAutoScalingGroups() bool {

	return len(r.enabledASGs) > 0

}

func (r *region) processEnabledAutoScalingGroups() {
	for _, asg := range r.enabledASGs {
		asg.terminationMethod = r.conf.InstanceTerminationMethod
		r.wg.Add(1)
		go func(a autoScalingGroup) {
			a.process()
			r.wg.Done()
		}(asg)
	}
	r.wg.Wait()
}
