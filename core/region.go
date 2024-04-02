// Copyright (c) 2016-2022 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"context"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	autoscalingTypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"

	"github.com/aws/aws-sdk-go-v2/service/cloudformation"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go/aws"
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
	instanceTypeInformation map[string]*instanceTypeInformation

	instances instances

	enabledASGs []autoScalingGroup
	services    *connections

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

	debug.Println("Creating connections to the required AWS services in", r.name)
	if err := r.services.connect(r.name, r.conf.MainRegion); err != nil {
		log.Printf("Error connecting services in region %s with main region %s: %s",
			r.name, r.conf.MainRegion, err.Error())
	} // only process the regions where we have AutoScaling groups set to be handled

	// setup the filters for asg matching
	r.setupAsgFilters()

	log.Println("Scanning for enabled AutoScaling groups in ", r.name)
	r.scanForEnabledAutoScalingGroups()

	// only process further the region if there are any enabled autoscaling groups
	// within it
	if r.hasEnabledAutoScalingGroups() {
		log.Println("Scanning full instance information in", r.name)
		r.determineInstanceTypeInformation(r.conf)

		err := r.scanInstances()
		if err != nil {
			log.Printf("Failed to scan instances in %s error: %s\n", r.name, err)
		}

		log.Println("Processing enabled AutoScaling groups in", r.name)
		r.processEnabledAutoScalingGroups()
	} else {
		log.Println(r.name, "has no enabled AutoScaling groups")
	}

	err := r.scanInstances()
	if err != nil {
		log.Printf("Failed to scan instances in %s error: %s\n", r.name, err)
	}
	r.reapUnattachedInstances()

	if err := r.reapDuplicatedInstances(); err != nil {
		log.Printf("Error reaping duplicated instances in region %s: %s", r.name, err.Error())
	}

	if err := r.reapOldLaunchTemplatesCreatedByAutoSpotting(); err != nil {
		log.Printf("Error reaping old launch templates created by AutoSpotting in region %s: %s", r.name, err)
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

func (r *region) processDescribeInstancesPage(page *ec2.DescribeInstancesOutput) {
	debug.Println("Processing page of DescribeInstancesPages for", r.name)

	if len(page.Reservations) > 0 &&
		page.Reservations[0].Instances != nil {

		for _, res := range page.Reservations {
			for _, inst := range res.Instances {
				i := inst
				r.addInstance(&i)
			}
		}
	}
}

func (r *region) scanInstances() error {
	log.Println("Scanning instances in", r.name)
	svc := r.services.ec2
	input := &ec2.DescribeInstancesInput{
		Filters: []ec2Types.Filter{
			{
				Name: aws.String("instance-state-name"),
				Values: []string{
					"running",
					"pending",
				},
			},
		},
	}

	r.instances = makeInstances()

	paginator := ec2.NewDescribeInstancesPaginator(svc, input)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {

			return err
		}
		r.processDescribeInstancesPage(page)

	}

	return nil
}

func (r *region) scanInstance(instanceID string) error {
	svc := r.services.ec2
	input := &ec2.DescribeInstancesInput{
		Filters: []ec2Types.Filter{
			{
				Name:   aws.String("instance-id"),
				Values: []string{instanceID},
			},
		},
	}

	r.instances = makeInstances()

	paginator := ec2.NewDescribeInstancesPaginator(svc, input)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			return err
		}

		r.processDescribeInstancesPage(page)
	}
	return nil
}

func (r *region) addInstance(inst *ec2Types.Instance) {
	r.instances.add(&instance{
		Instance: inst,
		typeInfo: r.instanceTypeInformation[string(inst.InstanceType)],
		region:   r,
	})
}

func (r *region) determineInstanceTypeInformation(cfg *Config) {

	r.instanceTypeInformation = make(map[string]*instanceTypeInformation)

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

		var info instanceTypeInformation

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
			debug.Printf("Adding instance type information for instance type %s, with memory %.2f",
				it.InstanceType, it.Memory)

			r.instanceTypeInformation[it.InstanceType] = &info
		}
	}
	// this is safe to do once outside of the loop because the call will only
	// return entries about the available instance ec2Types, so no invalid instance
	// ec2Types would be returned

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
		if priceInfo.AvailabilityZone == nil || priceInfo.SpotPrice == nil {
			log.Printf("Missing spot price or availability zone information, skipping entry: %#v", priceInfo)
			continue
		}

		instType, az := priceInfo.InstanceType, *priceInfo.AvailabilityZone

		// failure to parse this means that the instance is not available on the spot market
		price, err := strconv.ParseFloat(*priceInfo.SpotPrice, 64)
		if err != nil {
			log.Println(r.name, "Instance type ", instType, "is not available on the spot market")
			continue
		}

		instanceTypeInfo, ok := r.instanceTypeInformation[string(instType)]
		if !ok || instanceTypeInfo.pricing == nil {
			debug.Println(r.name, "No instance type information or pricing for", instType)
			continue
		}

		productDescription := string(priceInfo.ProductDescription)
		if instanceTypeInfo.pricing[productDescription].spot == nil {
			log.Println(r.name, "Instance data missing for", instType, "in", az, "skipping because this region is currently not supported")
			continue
		}

		instanceTypeInfo.pricing[productDescription].spot[az] = price
	}

	return nil
}

func tagsMatch(asgTag *autoscalingTypes.TagDescription, filteringTag Tag) bool {
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

func isASGWithMatchingTag(tagToMatch Tag, asgTags []autoscalingTypes.TagDescription) bool {
	for _, tag := range asgTags {
		asgTag := tag
		if tagsMatch(&asgTag, tagToMatch) {
			return true
		}
	}
	return false
}

func isASGWithMatchingTags(asg *autoscalingTypes.AutoScalingGroup, tagsToMatch []Tag) bool {
	matchedTags := 0

	for _, tag := range tagsToMatch {
		if asg != nil && isASGWithMatchingTag(tag, asg.Tags) {
			matchedTags++
		}
	}

	return matchedTags == len(tagsToMatch)
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

	if output, err := svc.DescribeStacks(context.TODO(), &input); err != nil {
		log.Println("Failed to describe stack", *stackName, "with error:", err.Error())
	} else {
		stackStatus := output.Stacks[0].StackStatus
		if _, exists := stackCompleteStatuses[string(stackStatus)]; !exists {
			return string(stackStatus), true
		}
	}

	return "", false
}

func (r *region) findMatchingASGsInPageOfResults(groups []autoscalingTypes.AutoScalingGroup, tagsToMatch []Tag) []autoScalingGroup {
	var asgs []autoScalingGroup
	var optInFilterMode = (r.conf.TagFilteringMode != "opt-out")

	for _, myGroup := range groups {
		group := myGroup
		asgName := *group.AutoScalingGroupName

		groupMatchesExpectedTags := isASGWithMatchingTags(&group, tagsToMatch)
		if optInFilterMode != groupMatchesExpectedTags {
			debug.Printf("Skipping group %s because its tags, the currently configured filtering mode (%s) and tag filters do not align\n", asgName, r.conf.TagFilteringMode)
			continue
		}

		log.Printf("Enabling group %s for processing because its tags, the currently configured filtering mode (%s),"+
			" and tag filters are aligned, and it passed all eligibility checks\n",
			asgName, r.conf.TagFilteringMode)
		asgs = append(asgs, autoScalingGroup{
			AutoScalingGroup: &group,
			name:             asgName,
			region:           r,
		})
	}
	return asgs
}

func (r *region) scanForEnabledAutoScalingGroups() {

	svc := r.services.autoScaling

	input := &autoscaling.DescribeAutoScalingGroupsInput{}
	paginator := autoscaling.NewDescribeAutoScalingGroupsPaginator(svc, input)

	pageNum := 0
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			log.Println("Failed to describe AutoScalingGroups in", r.name, err.Error())
			return
		}

		pageNum++
		debug.Println("Processing page", pageNum, "of DescribeAutoScalingGroups for", r.name)
		matchingAsgs := r.findMatchingASGsInPageOfResults(page.AutoScalingGroups, r.tagsToFilterASGsBy)
		r.enabledASGs = append(r.enabledASGs, matchingAsgs...)
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
			log.Printf("Processing enabled autoscaling group %s", a.name)
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

func (r *region) sqsDeleteMessage(eventKind string, instanceID *string, instanceLifecycle string, sqsReceiptHandle *string) error {
	svc := r.services.sqs

	time.Sleep(r.conf.SleepMultiplier * 5 * time.Second)

	_, err := svc.DeleteMessage(context.TODO(),
		&sqs.DeleteMessageInput{
			QueueUrl:      &r.conf.SQSQueueURL,
			ReceiptHandle: sqsReceiptHandle,
		})
	if err != nil {
		if instanceID != nil {
			log.Printf("%s Error deleting %s event message for %s instance %s "+
				"from the SQS Queue %s: %s", r.name, eventKind, instanceLifecycle, *instanceID, r.conf.SQSQueueURL, err)
		} else {
			log.Printf("%s Error deleting %s message "+
				"from the SQS Queue %s: %s", r.name, eventKind, r.conf.SQSQueueURL, err)
		}
		return err
	}
	if instanceID != nil {
		log.Printf("%s Successfully deleted %s event message for %s instance %s "+
			"from the SQS Queue %s", r.name, eventKind, instanceLifecycle, *instanceID, r.conf.SQSQueueURL)
	} else {
		log.Printf("%s Successfully deleted %s event message "+
			"from the SQS Queue %s: %s", r.name, eventKind, r.conf.SQSQueueURL, err)
	}
	return nil

}

func (r *region) calculateSavings() float64 {
	savings := 0.0
	asgSavings := make(map[string]float64)

	if err := r.services.connect(r.name, r.conf.MainRegion); err != nil {
		log.Printf("Error connecting to services in region %s: %v", r.name, err)
		return 0.0
	}

	debug.Println("Scanning full instance information in", r.name)
	r.determineInstanceTypeInformation(r.conf)

	debug.Println("Scanning instances in", r.name)
	if err := r.scanInstances(); err != nil {
		log.Printf("Failed to scan instances in %s error: %s\n", r.name, err)
	}

	debug.Println("Calculating AutoSpotting savings in", r.name)

	for inst := range r.instances.instances() {

		if inst.isSpot() || inst.isLaunchedByAutoSpotting() {

			is := inst.getSavings()
			log.Printf("Found AutoSpotting instance %s(%s) in %s with hourly savings %f\n",
				*inst.InstanceId, inst.InstanceType, r.name, is)
			savings += is
			if belongs, asg := inst.belongsToAnASG(); belongs {
				asgSavings[*asg] += is
			}
		}
	}
	debug.Printf("Total savings in %s: %f\n", r.name, savings)

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

func (r *region) reapUnattachedInstances() {
	for inst := range r.instances.instances() {
		lifecycle := inst.getLifecycleValue()
		debug.Printf("Looking at %s instance %s", lifecycle, *inst.InstanceId)
		if inst.isLaunchedByAutoSpotting() {
			debug.Printf("%s instance %s is launched by AutoSpotting",
				lifecycle, *inst.InstanceId)
			if belongs, asg := inst.belongsToAnASG(); !belongs {
				debug.Printf("%s instance %s doesn't belong to any ASG", lifecycle, *inst.InstanceId)
				if inst.isOlderThan(15 * time.Minute) {
					log.Printf("Reaping %s instance %s was launched by AutoSpotting but not attached to any ASG after more than 15min (%s), terminating it",
						lifecycle, *inst.InstanceId, time.Since(*inst.LaunchTime))
					if err := inst.terminate(); err != nil {
						log.Printf("Error terminating instance %s in %s: %s",
							*inst.InstanceId, inst.region.name, err.Error())
					}
				} else {
					debug.Printf("%s instance %s has recently been launched", lifecycle, *inst.InstanceId)
				}
			} else {
				debug.Printf("%s instance %s belongs to ASG %s", lifecycle, *inst.InstanceId, *asg)
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

func (r *region) tagResource(resourceID *string, tagKey string, tagValue string) error {
	tagInput := &ec2.CreateTagsInput{
		Resources: []string{*resourceID},
		Tags: []ec2Types.Tag{
			{
				Key:   aws.String(tagKey),
				Value: aws.String(tagValue),
			},
		},
	}

	_, err := r.services.ec2.CreateTags(context.TODO(), tagInput)
	if err != nil {
		return fmt.Errorf("error tagging resource %s: %v", *resourceID, err)
	}

	return nil
}

func (r *region) reapDuplicatedInstances() error {
	tagKey := "launched-for-replacing-instance"
	debug.Println("Starting the reaping of duplicated instances process")

	// Map to hold instances by their launched-for-replacing-instance tag value.
	instancesByTag := make(map[string][]*instance)

	// Populate the map.
	for inst := range r.instances.instances() {
		for _, tag := range inst.Tags {
			if *tag.Key == tagKey {
				if belongs, _ := inst.belongsToAnASG(); !belongs {
					continue
				}
				instancesByTag[*tag.Value] = append(instancesByTag[*tag.Value], inst)
				log.Printf("Instance %s has tag %s with value %s", *inst.InstanceId, tagKey, *tag.Value)
			}
		}
	}

	debug.Printf("Checking for duplicated instances with tag key: %s attached to an ASG", tagKey)
	// Iterate over the map to find duplicates and terminate the newest.
	for tagValue, instances := range instancesByTag {
		if len(instances) > 1 {
			log.Printf("Found %d instances with duplicated tag value: %s", len(instances), tagValue)
			newestInstance := findNewestInstance(instances)
			asg, err := r.findASGForInstance(newestInstance)
			if err != nil {
				log.Printf("Error finding ASG for instance %s: %v", *newestInstance.InstanceId, err)
				continue
			}
			if err := newestInstance.terminateInstanceInAutoScalingGroup(asg); err != nil {
				log.Printf("Failed to terminate instance %s in ASG %s: %v", *newestInstance.InstanceId, asg.name, err)
				return err
			}
			log.Printf("Successfully terminated newest instance %s with duplicated tag %s in ASG %s", *newestInstance.InstanceId, tagKey, asg.name)
		}
	}

	debug.Printf("Completed the reaping of duplicated instances process in %s", r.name)
	return nil
}

func findNewestInstance(instances []*instance) *instance {
	var newest *instance
	log.Println("Identifying the newest instance among the duplicates")
	for _, inst := range instances {
		if newest == nil || inst.LaunchTime.After(*newest.LaunchTime) {
			log.Printf("Newest instance found: %s (Launch Time: %s)", *inst.InstanceId, inst.LaunchTime)
			newest = inst
		}
	}
	if newest != nil {
		log.Printf("Newest instance identified: %s", *newest.InstanceId)
	} else {
		log.Println("No newest instance found")
	}
	return newest
}
func (r *region) findASGForInstance(inst *instance) (*autoScalingGroup, error) {
	belongs, asgName := inst.belongsToAnASG()
	if !belongs || asgName == nil {
		log.Printf("Instance %s does not belong to ASG", *inst.InstanceId)
	}

	r.scanForEnabledAutoScalingGroups()

	log.Printf("Finding ASG for instance: %s", *inst.InstanceId)
	for _, asg := range r.enabledASGs {
		if asg.name == *asgName && asg.containsInstance(inst) {
			log.Printf("Found ASG: %s for instance: %s", asg.name, *inst.InstanceId)
			return &asg, nil
		}
	}
	errMsg := fmt.Sprintf("No ASG found for instance %s", *inst.InstanceId)
	log.Println(errMsg)
	return nil, errors.New(errMsg)
}
func (r *region) waitForInstanceRunning(instanceID *string) error {
	// Define the timeout for the waiter
	waitTimeout := 5 * time.Minute

	// Define the polling interval
	pollInterval := 15 * time.Second

	// Record the time when we start waiting
	startTime := time.Now()

	// Context for the operation
	ctx := context.TODO()

	for {
		// Check if we've exceeded the timeout
		if time.Since(startTime) > waitTimeout {
			return fmt.Errorf("timed out waiting for instance to be running")
		}

		// Describe the instance
		input := &ec2.DescribeInstancesInput{
			InstanceIds: []string{*instanceID},
		}
		result, err := r.services.ec2.DescribeInstances(ctx, input)
		if err != nil {
			log.Printf("failed to describe instance: %s", err.Error())
			time.Sleep(pollInterval)
			continue
		}

		// Check the instance state
		if len(result.Reservations) > 0 && len(result.Reservations[0].Instances) > 0 {
			state := result.Reservations[0].Instances[0].State
			if state != nil && state.Name == ec2Types.InstanceStateNameRunning {
				return nil
			}
		}

		// Wait for the next poll interval
		time.Sleep(pollInterval)
	}
}

func (r *region) waitUntilVolumeAvailable(volumeID *string) error {
	// Define the timeout for the waiter
	waitTimeout := 5 * time.Minute

	// Define the polling interval
	pollInterval := 15 * time.Second

	// Record the time when we start waiting
	startTime := time.Now()

	ctx := context.TODO()

	for {
		// Check if we've exceeded the timeout
		if time.Since(startTime) > waitTimeout {
			return fmt.Errorf("timed out waiting for volume to become available")
		}

		// Describe the volume
		input := &ec2.DescribeVolumesInput{
			VolumeIds: []string{*volumeID},
		}
		resp, err := r.services.ec2.DescribeVolumes(ctx, input)
		if err != nil {
			return fmt.Errorf("failed to describe volume: %w", err)
		}

		if len(resp.Volumes) == 0 {
			return fmt.Errorf("no volume found with the provided ID")
		}

		// Check the volume state
		if resp.Volumes[0].State == ec2Types.VolumeStateAvailable {
			log.Printf("EBS volume %s successfully detached and is now available", *volumeID)
			return nil
		}

		// Wait for the next poll interval
		time.Sleep(pollInterval)
	}
}

func (r *region) reapOldLaunchTemplatesCreatedByAutoSpotting() error {
	oneWeekAgo := time.Now().AddDate(0, 0, -7)
	autoSpottingPrefix := "AutoSpotting-Converted-LaunchTemplate"

	paginator := ec2.NewDescribeLaunchTemplatesPaginator(r.services.ec2, &ec2.DescribeLaunchTemplatesInput{})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			log.Printf("Error getting page of launch templates: %s", err)
			return err
		}

		for _, lt := range page.LaunchTemplates {
			// Check if the launch template name starts with the AutoSpotting prefix and is older than a week
			if strings.HasPrefix(*lt.LaunchTemplateName, autoSpottingPrefix) && lt.CreateTime.Before(oneWeekAgo) {
				log.Printf("Launch template %s (%s) created on %s is older than a week and created by AutoSpotting, deleting...",
					*lt.LaunchTemplateName, *lt.LaunchTemplateId, lt.CreateTime)

				// Delete the launch template
				_, err := r.services.ec2.DeleteLaunchTemplate(context.TODO(), &ec2.DeleteLaunchTemplateInput{
					LaunchTemplateId: lt.LaunchTemplateId,
				})
				if err != nil {
					log.Printf("Error deleting launch template %s: %s", *lt.LaunchTemplateName, err)
					return err
				}
				log.Printf("Successfully deleted launch template %s", *lt.LaunchTemplateName)
			}
		}
	}
	return nil
}

func (r *region) terminateInstanceByID(instanceID string) error {
	_, err := r.services.ec2.TerminateInstances(context.TODO(), &ec2.TerminateInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		log.Printf("Error terminating instance %s: %s", instanceID, err.Error())
		return err
	}
	return nil
}
