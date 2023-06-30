// Copyright (c) 2016-2022 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"errors"
	"log"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/sqs"
)

// Tag represents an Asg Tag: Key, Value
type Tag struct {
	Key   string
	Value string
}

// data structure that stores information about a region
type region struct {
	name string

	autospotting *AutoSpotting
	conf         *Config
	// The key in this map is the instance type.
	instanceTypeInformation map[string]instanceTypeInformation

	instances instances

	enabledASGs []autoScalingGroup
	services    connections

	tagsToFilterASGsBy []Tag

	wg sync.WaitGroup
}

// the key in this map is the Spot product (or OS)
type prices map[string]instancePrice
type instancePrice struct {
	onDemand float64
	spot     spotPriceMap
}

// The key in this map is the availability zone
type spotPriceMap map[string]float64

type instanceTypeFamilyInfo struct {
	family     string
	generation int64
}
type instanceTypeFamilyInfoCache map[string]*instanceTypeFamilyInfo

// Stores the maximum generation for each instance type
type instanceTypeMaxGenerationCache map[string]int64

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

	log.Println("Creating connections to the required AWS services in", r.name)
	r.services.connect(r.name, r.conf.MainRegion)
	// only process the regions where we have AutoScaling groups set to be handled

	// setup the filters for asg matching
	r.setupAsgFilters()

	log.Println("Scanning instances in", r.name)
	err := r.scanInstances()
	if err != nil {
		log.Printf("Failed to scan instances in %s error: %s\n", r.name, err)
	}

	log.Println("Scanning for enabled AutoScaling groups in ", r.name)
	r.scanForEnabledAutoScalingGroups()

	// only process further the region if there are any enabled autoscaling groups
	// within it
	if r.hasEnabledAutoScalingGroups() {
		log.Println("Scanning full instance information in", r.name)
		r.determineInstanceTypeInformation(r.conf)

		log.Println("Processing enabled AutoScaling groups in", r.name)
		r.processEnabledAutoScalingGroups()
	} else {
		log.Println(r.name, "has no enabled AutoScaling groups")
	}
	r.reapUnattachedSpotInstances()
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

func (r *region) processDescribeInstancesPage(page *ec2.DescribeInstancesOutput, lastPage bool) bool {
	debug.Println("Processing page of DescribeInstancesPages for", r.name)

	if len(page.Reservations) > 0 &&
		page.Reservations[0].Instances != nil {

		for _, res := range page.Reservations {
			for _, inst := range res.Instances {
				r.addInstance(inst)
			}
		}
	}
	return true
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

	err := svc.DescribeInstancesPages(
		input,
		r.processDescribeInstancesPage)

	if err != nil {
		return err
	}

	debug.Println(r.instances.dump())

	return nil
}

func (r *region) scanInstance(instanceID *string) error {
	svc := r.services.ec2
	input := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("instance-id"),
				Values: []*string{instanceID},
			},
		},
	}

	r.instances = makeInstances()

	err := svc.DescribeInstancesPages(
		input,
		r.processDescribeInstancesPage)

	if err != nil {
		return err
	}

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

	itfic := make(instanceTypeFamilyInfoCache)
	itmgc := make(instanceTypeMaxGenerationCache)

	for _, it := range *cfg.InstanceData {

		price := make(prices)

		// populate on-demand information
		price[LinuxSpotProduct] = instancePrice{
			onDemand: it.Pricing[r.name].Linux.OnDemand * cfg.OnDemandPriceMultiplier,
			spot:     make(spotPriceMap),
		}

		price[WindowsSpotProduct] = instancePrice{
			onDemand: it.Pricing[r.name].MSWin.OnDemand * cfg.OnDemandPriceMultiplier,
			spot:     make(spotPriceMap),
		}

		price[RHELSpotProduct] = instancePrice{
			onDemand: it.Pricing[r.name].RHEL.OnDemand * cfg.OnDemandPriceMultiplier,
			spot:     make(spotPriceMap),
		}

		price[SLESSpotProduct] = instancePrice{
			onDemand: it.Pricing[r.name].SLES.OnDemand * cfg.OnDemandPriceMultiplier,
			spot:     make(spotPriceMap),
		}

		// if at this point the instance price is still zero, then that
		// particular instance type doesn't even exist in the current
		// region, so we don't even need to create an empty spot pricing
		// data structure for it
		if price[LinuxSpotProduct].onDemand > 0 {
			// for each instance type populate the HW spec information
			info = instanceTypeInformation{
				instanceType:        it.InstanceType,
				vCPU:                it.VCPU,
				PhysicalProcessor:   it.PhysicalProcessor,
				memory:              it.Memory,
				GPU:                 it.GPU,
				pricing:             price,
				virtualizationTypes: it.LinuxVirtualizationTypes,
				hasEBSOptimization:  it.EBSOptimized,
				EBSThroughput:       it.EBSThroughput,
				generationDelta:     calculateGenerationDelta(cfg.InstanceData, it.InstanceType, &itfic, &itmgc),
			}

			if it.Storage != nil {
				info.hasInstanceStore = true
				info.instanceStoreDeviceSize = it.Storage.Size
				info.instanceStoreDeviceCount = it.Storage.Devices
				info.instanceStoreIsSSD = it.Storage.SSD
			}
			r.instanceTypeInformation[it.InstanceType] = info
		}
	}
	// this is safe to do once outside of the loop because the call will only
	// return entries about the available instance types, so no invalid instance
	// types would be returned

	if err := r.requestSpotPrices(); err != nil {
		log.Println(err.Error())
	}

}

func (r *region) requestSpotPrices() error {

	s := spotPrices{conn: r.services}

	// Retrieve all current spot prices from the current region.
	// TODO: add support for other OSes
	err := s.fetch(0, nil, nil)

	if err != nil {
		return errors.New("Couldn't fetch spot prices in " + r.name)
	}

	// log.Println("Spot Price list in ", r.name, ":\n", s.data)

	for _, priceInfo := range s.data {

		instType, az := *priceInfo.InstanceType, *priceInfo.AvailabilityZone

		// failure to parse this means that the instance is not available on the
		// spot market
		price, err := strconv.ParseFloat(*priceInfo.SpotPrice, 64)
		if err != nil {
			debug.Println(r.name, "Instance type ", instType,
				"is not available on the spot market")
			continue
		}

		if r.instanceTypeInformation[instType].pricing[*priceInfo.ProductDescription].spot == nil {
			debug.Println(r.name, "Instance data missing for", instType, "in", az,
				"skipping because this region is currently not supported")
			continue
		}

		r.instanceTypeInformation[instType].pricing[*priceInfo.ProductDescription].spot[az] = price

	}

	return nil
}

func tagsMatch(asgTag *autoscaling.TagDescription, filteringTag Tag) bool {
	if asgTag != nil && *asgTag.Key == filteringTag.Key {
		matched, err := filepath.Match(filteringTag.Value, *asgTag.Value)
		if err != nil {
			log.Printf("%s Invalid glob expression or text input in filter %s, the instance list may be smaller than expected", filteringTag.Key, filteringTag.Value)
			return false
		}
		return matched
	}
	return false
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

func getTagValueFromASGWithMatchingTag(asg *autoscaling.Group, tagToMatch Tag) *string {
	for _, asgTag := range asg.Tags {
		if tagsMatch(asgTag, tagToMatch) {
			return asgTag.Value
		}
	}
	return nil
}

func (r *region) isStackUpdating(stackName *string) (string, bool) {
	stackCompleteStatuses := map[string]struct{}{
		"CREATE_IN_PROGRESS":       {}, // allow replacing instances from brand new ASGs
		"CREATE_COMPLETE":          {},
		"UPDATE_COMPLETE":          {},
		"UPDATE_ROLLBACK_COMPLETE": {},
		"UPDATE_ROLLBACK_FAILED":   {},
	}

	svc := r.services.cloudFormation
	input := cloudformation.DescribeStacksInput{
		StackName: stackName,
	}

	if output, err := svc.DescribeStacks(&input); err != nil {
		log.Println("Failed to describe stack", *stackName, "with error:", err.Error())
	} else {
		stackStatus := output.Stacks[0].StackStatus
		if _, exists := stackCompleteStatuses[*stackStatus]; !exists {
			return *stackStatus, true
		}
	}

	return "", false
}

func (r *region) findMatchingASGsInPageOfResults(groups []*autoscaling.Group,
	tagsToMatch []Tag) []autoScalingGroup {

	var asgs []autoScalingGroup
	var optInFilterMode = (r.conf.TagFilteringMode != "opt-out")

	tagCloudFormationStackName := Tag{Key: "aws:cloudformation:stack-name", Value: "*"}

	for _, group := range groups {
		asgName := *group.AutoScalingGroupName

		groupMatchesExpectedTags := isASGWithMatchingTags(group, tagsToMatch)
		// Go lacks a logical XOR operator, this is the equivalent to that logical
		// expression. The goal is to add the matching ASGs when running in opt-in
		// mode and the other way round.
		if optInFilterMode != groupMatchesExpectedTags {
			debug.Printf("Skipping group %s because its tags, the currently "+
				"configured filtering mode (%s) and tag filters do not align\n",
				asgName, r.conf.TagFilteringMode)
			continue
		}

		if stackName := getTagValueFromASGWithMatchingTag(group, tagCloudFormationStackName); stackName != nil {
			debug.Println("Stack: ", *stackName)
			if status, updating := r.isStackUpdating(stackName); updating {
				log.Printf("Skipping group %s because stack %s is in state %s\n",
					asgName, *stackName, status)
				continue
			}
		}

		log.Printf("Enabling group %s for processing because its tags, the "+
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
			debug.Println("Processing page", pageNum, "of DescribeAutoScalingGroupsPages for", r.name, "lastPage is", lastPage)
			matchingAsgs := r.findMatchingASGsInPageOfResults(page.AutoScalingGroups, r.tagsToFilterASGsBy)
			r.enabledASGs = append(r.enabledASGs, matchingAsgs...)
			return true
		},
	)

	if err != nil {
		log.Println("Failed to describe AutoScalingGroups in", r.name, err.Error())
	}

}

func (r *region) hasEnabledAutoScalingGroups() bool {

	return len(r.enabledASGs) > 0

}

func (r *region) processEnabledAutoScalingGroups() {
	for _, asg := range r.enabledASGs {

		// Pass default configs to the group
		asg.config = r.conf.AutoScalingConfig
		asg.autospotting = r.autospotting

		r.wg.Add(1)
		go func(a autoScalingGroup) {
			action := a.cronEventAction()
			action.run()
			r.wg.Done()
		}(asg)
	}
	r.wg.Wait()
}

func (r *region) findEnabledASGByName(name string) *autoScalingGroup {
	for _, asg := range r.enabledASGs {
		if asg.name == name {
			return &asg
		}
	}
	return nil
}

func (r *region) sqsDeleteMessage(instanceID *string, instanceLifecycle string, sqsReceiptHandle *string) error {
	svc := r.services.sqs

	_, err := svc.DeleteMessage(
		&sqs.DeleteMessageInput{
			QueueUrl:      &r.conf.SQSQueueURL,
			ReceiptHandle: sqsReceiptHandle,
		})
	if err != nil {
		log.Printf("%s Error deleting %s instance %s launch event message "+
			"from the SQS Queue %s: %s", r.name, instanceLifecycle, *instanceID, r.conf.SQSQueueURL, err)
		return err
	}

	log.Printf("%s Successfully deleted spot instance %s launch event message "+
		"from the SQS Queue %s", r.name, *instanceID, r.conf.SQSQueueURL)

	return nil

}

func (r *region) calculateSavings() float64 {
	savings := 0.0
	asgSavings := make(map[string]float64)

	r.services.connect(r.name, r.conf.MainRegion)

	log.Println("Scanning full instance information in", r.name)
	r.determineInstanceTypeInformation(r.conf)

	log.Println("Scanning instances in", r.name)
	err := r.scanInstances()
	if err != nil {
		log.Printf("Failed to scan instances in %s error: %s\n", r.name, err)
	}

	log.Println("Calculating AutoSpotting savings in", r.name)

	for inst := range r.instances.instances() {

		if inst.isSpot() || inst.isLaunchedByAutoSpotting() {

			is := inst.getSavings()
			log.Printf("Found AutoSpotting instance %s(%s) in %s with hourly savings %f\n",
				*inst.InstanceId, *inst.InstanceType, r.name, is)
			savings += is
			if belongs, asg := inst.belongsToAnASG(); belongs {
				asgSavings[*asg] += is
			}
		}
	}
	log.Printf("Total savings in %s: %f\n", r.name, savings)

	ss := savingsStats{
		sns:             r.services.sns,
		topic:           r.conf.NotificationSNSTopic,
		reportFrequency: r.conf.SavingsReportsFrequency,
		ps: parameterStore{
			svc: r.services.ssm,
		},
	}
	ss.persistHourlyASGSavings(r.name, asgSavings)
	return savings
}

func (r *region) reapUnattachedSpotInstances() {
	for inst := range r.instances.instances() {
		debug.Printf("Looking at instance %s", *inst.InstanceId)
		if inst.isSpot() && inst.isLaunchedByAutoSpotting() {
			debug.Printf("instance %s is Spot and launched by AutoSpotting", *inst.InstanceId)
			if belongs, asg := inst.belongsToAnASG(); !belongs {
				debug.Printf("instance %s doesn't belong to any ASG", *inst.InstanceId)
				if inst.LaunchTime.Before(time.Now().Add(-15 * time.Minute)) {
					log.Printf("Reaping Spot instance %s launched by AutoSpotting but not attached to any ASG after more than 15min (%s), terminating it",
						*inst.InstanceId, time.Since(*inst.LaunchTime))
					inst.terminate()
				} else {
					debug.Printf("instance %s has recently been launched", *inst.InstanceId)
				}
			} else {
				debug.Printf("instance %s belongs to ASG %s", *inst.InstanceId, *asg)
			}
		}
	}
}

func (r *region) extractECSClusterName(clusterARN string) (string, error) {
	if !strings.Contains(clusterARN, ":cluster/") {
		return "", errors.New("input string is not in the expected format")
	}

	return strings.TrimPrefix(strings.Split(clusterARN, "/")[1], ":"), nil
}
