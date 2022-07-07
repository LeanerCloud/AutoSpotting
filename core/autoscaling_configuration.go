// Copyright (c) 2016-2022 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"log"
	"math"
	"strconv"
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

	// OnDemandPriceMultiplierTag is the name of a tag that can be defined on a
	// per-group level for overriding multiplier for the on-demand price.
	OnDemandPriceMultiplierTag = "autospotting_on_demand_price_multiplier"

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

	// DefaultSpotProductPremium stores the default value to add to the
	// on demand price for premium instance types.
	DefaultSpotProductPremium = 0.0

	// DefaultMinOnDemandValue stores the default on-demand capacity to be kept
	// running in a group managed by autospotting.
	DefaultMinOnDemandValue = 0

	// DefaultSpotPriceBufferPercentage stores the default percentage value
	// above the current spot price to place a bid
	DefaultSpotPriceBufferPercentage = 10.0

	// DefaultBiddingPolicy stores the default bidding policy for
	// the spot bid on a per-group level
	DefaultBiddingPolicy = "normal"

	// DefaultOnDemandPriceMultiplier stores the default OnDemand price multiplier
	// on a per-group level
	DefaultOnDemandPriceMultiplier = 1.0

	// DefaultInstanceTerminationMethod is the default value for the instance termination
	// method configuration option
	DefaultInstanceTerminationMethod = AutoScalingTerminationMethod

	// ScheduleTag is the name of the tag set on the AutoScaling Group that
	// can override the global value of the Schedule parameter
	ScheduleTag = "autospotting_cron_schedule"

	// TimezoneTag is the name of the tag set on the AutoScaling Group that
	// can override the global value of the Timezone parameter
	TimezoneTag = "autospotting_cron_timezone"

	// CronScheduleStateOn controls whether to run or not to run during the time interval
	// specified in the Schedule variable or its per-group tag overrides. It
	// accepts "on|off" as valid values
	CronScheduleStateOn = "on"

	// CronScheduleStateTag is the name of the tag set on the AutoScaling Group that
	// can override the global value of the CronScheduleState parameter
	CronScheduleStateTag = "autospotting_cron_schedule_state"

	// EnableInstanceLaunchEventHandlingTag is the name of the tag set on the
	// AutoScaling Group that enables the event-based instance replacement logic
	// for this group. It is set automatically once the legacy cron-based
	// replacement logic is done replacing instances in any given group.
	EnableInstanceLaunchEventHandlingTag = "autospotting_enable_instance_launch_event_handling"

	// PatchBeanstalkUserdataTag is the name of the tag set on the AutoScaling Group that
	// can override the global value of the PatchBeanstalkUserdata parameter
	PatchBeanstalkUserdataTag = "autospotting_patch_beanstalk_userdata"

	// GP2ConversionThresholdTag is the name of the tag set on the AutoScaling Group that
	// can override the global value of the GP2ConversionThreshold parameter
	GP2ConversionThresholdTag = "autospotting_gp2_conversion_threshold"

	// SpotAllocationStrategyTag is the name of the tag set on the AutoScaling Group that
	// can override the global value of the SpotAllocationStrategy parameter
	SpotAllocationStrategyTag = "autospotting_spot_allocation_strategy"

	// PrioritizedInstanceTypesBiasTag is the name of the tag set on the AutoScaling Group that
	// can override the global value of the PrioritizedInstanceTypesBias parameter
	PrioritizedInstanceTypesBiasTag = "autospotting_prioritized_instance_types_bias"
)

// AutoScalingConfig stores some group-specific configurations that can override
// their corresponding global values
type AutoScalingConfig struct {
	MinOnDemand             int64
	MinOnDemandNumber       int64
	MinOnDemandPercentage   float64
	AllowedInstanceTypes    string
	DisallowedInstanceTypes string

	OnDemandPriceMultiplier   float64
	SpotPriceBufferPercentage float64

	SpotProductDescription string
	SpotProductPremium     float64

	BiddingPolicy string

	TerminationMethod string

	// Instance termination method
	InstanceTerminationMethod string

	// Termination Notification action
	TerminationNotificationAction string

	CronSchedule      string
	CronTimezone      string
	CronScheduleState string // "on" or "off", dictate whether to run inside the CronSchedule or not

	PatchBeanstalkUserdata bool

	// Threshold for converting EBS volumes from GP2 to GP3, since after a certain
	// size GP2 may be more performant than GP3.
	GP2ConversionThreshold int64

	// Controls the instance type selection when launching new Spot instances.
	// Further information about this is available at
	// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-fleet-allocation-strategy.html
	SpotAllocationStrategy string

	// PrioritizedInstanceTypesBias can be used to tweak the ordering of the instance types when using the
	//"capacity-optimized-prioritized" allocation strategy, biasing towards newer instance types.
	PrioritizedInstanceTypesBias string
}

func (a *autoScalingGroup) loadPercentageOnDemand(tagValue *string) (int64, bool) {
	percentage, err := strconv.ParseFloat(*tagValue, 64)
	if err != nil {
		log.Printf("Error with ParseFloat: %s\n", err.Error())
	} else if percentage == 0 {
		log.Printf("Loaded MinOnDemand value to %f from tag %s\n", percentage, OnDemandPercentageTag)
		return int64(percentage), true
	} else if percentage > 0 && percentage <= 100 {
		instanceNumber := float64(a.instances.count())
		onDemand := int64(math.Floor((instanceNumber * percentage / 100.0) + .5))
		log.Printf("Loaded MinOnDemand value to %d from tag %s\n", onDemand, OnDemandPercentageTag)
		return onDemand, true
	}

	log.Printf("Ignoring value out of range %f\n", percentage)

	return DefaultMinOnDemandValue, false
}

func (a *autoScalingGroup) loadSpotPriceBufferPercentage(tagValue *string) (float64, bool) {
	spotPriceBufferPercentage, err := strconv.ParseFloat(*tagValue, 64)

	if err != nil {
		log.Printf("Error with ParseFloat: %s\n", err.Error())
		return DefaultSpotPriceBufferPercentage, false
	} else if spotPriceBufferPercentage < 0 {
		log.Printf("Ignoring out of range value : %f\n", spotPriceBufferPercentage)
		return DefaultSpotPriceBufferPercentage, false
	}

	log.Printf("Loaded SpotPriceBufferPercentage value to %f from tag %s\n", spotPriceBufferPercentage, SpotPriceBufferPercentageTag)
	return spotPriceBufferPercentage, true
}

func (a *autoScalingGroup) loadNumberOnDemand(tagValue *string) (int64, bool) {
	onDemand, err := strconv.Atoi(*tagValue)
	if err != nil {
		log.Printf("Error with Atoi: %s\n", err.Error())
	} else if onDemand >= 0 && int64(onDemand) <= *a.MaxSize {
		log.Printf("Loaded MinOnDemand value to %d from tag %s\n", onDemand, OnDemandNumberLong)
		return int64(onDemand), true
	} else {
		log.Printf("Ignoring value out of range %d\n", onDemand)
	}
	return DefaultMinOnDemandValue, false
}

func (a *autoScalingGroup) loadOnDemandPriceMultiplier(tagValue *string) (float64, bool) {
	onDemandPriceMultiplier, err := strconv.ParseFloat(*tagValue, 64)

	if err != nil {
		log.Printf("Error with ParseFloat: %s\n", err.Error())
		return DefaultOnDemandPriceMultiplier, false
	} else if onDemandPriceMultiplier <= 0 {
		log.Printf("Ignoring out of range value : %f\n", onDemandPriceMultiplier)
		return DefaultOnDemandPriceMultiplier, false
	}

	log.Printf("Loaded OnDemandPriceMultiplier value to %f from tag %s\n", onDemandPriceMultiplier, OnDemandPriceMultiplierTag)
	return onDemandPriceMultiplier, true
}

func (a *autoScalingGroup) getTagValue(keyMatch string) *string {
	for _, asgTag := range a.Tags {
		if *asgTag.Key == keyMatch {
			return asgTag.Value
		}
	}
	return nil
}

func (a *autoScalingGroup) setMinOnDemandIfLarger(newValue int64, hasMinOnDemand bool) bool {
	if !hasMinOnDemand || newValue > a.config.MinOnDemand {
		a.config.MinOnDemand = newValue
	}
	return true
}

func (a *autoScalingGroup) loadConfOnDemand() bool {
	tagList := [2]string{OnDemandNumberLong, OnDemandPercentageTag}
	loadDyn := map[string]func(*string) (int64, bool){
		OnDemandPercentageTag: a.loadPercentageOnDemand,
		OnDemandNumberLong:    a.loadNumberOnDemand,
	}

	foundLimit := false
	for _, tagKey := range tagList {
		if tagValue := a.getTagValue(tagKey); tagValue != nil {
			if _, ok := loadDyn[tagKey]; ok {
				if newValue, done := loadDyn[tagKey](tagValue); done {
					foundLimit = a.setMinOnDemandIfLarger(newValue, foundLimit)
				}
			}
		}
		debug.Println("Couldn't find tag", tagKey)
	}
	return foundLimit
}

func (a *autoScalingGroup) loadPatchBeanstalkUserdata() bool {
	tagValue := a.getTagValue(PatchBeanstalkUserdataTag)

	if tagValue != nil {
		log.Printf("Loaded PatchBeanstalkUserdata value %v from tag %v\n", *tagValue, PatchBeanstalkUserdataTag)
		val, err := strconv.ParseBool(*tagValue)

		if err != nil {
			log.Printf("Failed to parse PatchBeanstalkUserdata value %v as a boolean", *tagValue)
			return false
		}
		a.config.PatchBeanstalkUserdata = val
		return true
	}
	debug.Println("Couldn't find tag", PatchBeanstalkUserdataTag, "on the group", a.name, "using the default configuration")
	a.config.PatchBeanstalkUserdata = a.region.conf.PatchBeanstalkUserdata
	return false
}

func (a *autoScalingGroup) loadSpotAllocationStrategy() bool {
	a.config.SpotAllocationStrategy = a.region.conf.SpotAllocationStrategy

	tagValue := a.getTagValue(SpotAllocationStrategyTag)

	if tagValue != nil {
		log.Printf("Loaded AllocationStrategy value %v from tag %v\n", *tagValue, SpotAllocationStrategyTag)
		a.config.SpotAllocationStrategy = *tagValue
		return true
	}

	debug.Println("Couldn't find tag", SpotAllocationStrategyTag, "on the group", a.name, "using the default configuration")
	return false
}

func (a *autoScalingGroup) loadPrioritizedInstanceTypesBiasTag() bool {
	a.config.PrioritizedInstanceTypesBias = a.region.conf.PrioritizedInstanceTypesBias

	tagValue := a.getTagValue(PrioritizedInstanceTypesBiasTag)

	if tagValue != nil {
		log.Printf("Loaded PrioritizedInstanceTypesBiasTag value %v from tag %v\n", *tagValue, PrioritizedInstanceTypesBiasTag)
		a.config.PrioritizedInstanceTypesBias = *tagValue
		return true
	}

	debug.Println("Couldn't find tag", PrioritizedInstanceTypesBiasTag, "on the group", a.name, "using the default configuration")
	return false
}

func (a *autoScalingGroup) loadGP2ConversionThreshold() bool {
	// setting the default value
	a.config.GP2ConversionThreshold = a.region.conf.GP2ConversionThreshold

	tagValue := a.getTagValue(GP2ConversionThresholdTag)
	if tagValue == nil {
		log.Printf("Couldn't load the GP2ConversionThreshold from tag %v, using the globally configured value of %v\n", GP2ConversionThresholdTag, a.config.GP2ConversionThreshold)
		return false
	}

	log.Printf("Loaded GP2ConversionThreshold value %v from tag %v\n", *tagValue, GP2ConversionThresholdTag)

	threshold, err := strconv.Atoi(*tagValue)
	if err != nil {
		log.Printf("Error parsing %v qs integer: %s\n", *tagValue, err.Error())
		return false
	}

	debug.Println("Successfully parsed", GP2ConversionThresholdTag, "on the group", a.name, "overriding the default configuration")
	a.config.GP2ConversionThreshold = int64(threshold)
	return true
}

func (a *autoScalingGroup) loadBiddingPolicy(tagValue *string) (string, bool) {
	biddingPolicy := *tagValue
	if biddingPolicy != "aggressive" {
		return DefaultBiddingPolicy, false
	}

	log.Printf("Loaded BiddingPolicy value with %s from tag %s\n", biddingPolicy, BiddingPolicyTag)
	return biddingPolicy, true
}

func (a *autoScalingGroup) LoadCronSchedule() bool {
	tagValue := a.getTagValue(ScheduleTag)

	if tagValue != nil {
		log.Printf("Loaded CronSchedule value %v from tag %v\n", *tagValue, ScheduleTag)
		a.config.CronSchedule = *tagValue
		return true
	}

	debug.Println("Couldn't find tag", ScheduleTag, "on the group", a.name, "using the default configuration")
	a.config.CronSchedule = a.region.conf.CronSchedule
	return false
}

func (a *autoScalingGroup) LoadCronTimezone() bool {
	tagValue := a.getTagValue(TimezoneTag)

	if tagValue != nil {
		log.Printf("Loaded CronTimezone value %v from tag %v\n", *tagValue, TimezoneTag)
		a.config.CronTimezone = *tagValue
		return true
	}

	debug.Println("Couldn't find tag", TimezoneTag, "on the group", a.name, "using the default configuration")
	a.config.CronTimezone = a.region.conf.CronTimezone
	return false
}

func (a *autoScalingGroup) LoadCronScheduleState() bool {
	tagValue := a.getTagValue(CronScheduleStateTag)
	if tagValue != nil {
		log.Printf("Loaded CronScheduleState value %v from tag %v\n", *tagValue, CronScheduleStateTag)
		a.config.CronScheduleState = *tagValue
		return true
	}

	debug.Println("Couldn't find tag", CronScheduleStateTag, "on the group", a.name, "using the default configuration")
	a.config.CronScheduleState = a.region.conf.CronScheduleState
	return false
}

func (a *autoScalingGroup) loadConfSpot() bool {
	tagValue := a.getTagValue(BiddingPolicyTag)
	if tagValue == nil {
		debug.Println("Couldn't find tag", BiddingPolicyTag)
		return false
	}
	if newValue, done := a.loadBiddingPolicy(tagValue); done {
		a.region.conf.BiddingPolicy = newValue
		debug.Println("BiddingPolicy =", a.region.conf.BiddingPolicy)
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

func (a *autoScalingGroup) loadConfOnDemandPriceMultiplier() bool {
	a.config.OnDemandPriceMultiplier = a.region.conf.OnDemandPriceMultiplier
	tagValue := a.getTagValue(OnDemandPriceMultiplierTag)
	if tagValue == nil {
		return false
	}

	newValue, done := a.loadOnDemandPriceMultiplier(tagValue)
	if !done {
		debug.Println("Couldn't find tag", OnDemandPriceMultiplierTag)
		return false
	}

	a.config.OnDemandPriceMultiplier = newValue
	return done
}

// Add configuration of other elements here: prices, whitelisting, etc
func (a *autoScalingGroup) loadConfigFromTags() bool {
	ret := false

	if a.loadConfOnDemand() {
		log.Println("Found and applied configuration for OnDemand value")
		ret = true
	}

	if a.loadConfOnDemandPriceMultiplier() {
		log.Println("Found and applied configuration for OnDemand Price Multiplier")
		ret = true
	}

	if a.loadConfSpot() {
		log.Println("Found and applied configuration for Spot Bid")
		ret = true
	}

	if a.loadConfSpotPrice() {
		log.Println("Found and applied configuration for Spot Price")
		ret = true
	}

	if a.LoadCronSchedule() {
		log.Println("Found and applied configuration for Cron Schedule")
		ret = true
	}

	if a.LoadCronTimezone() {
		log.Println("Found and applied configuration for Cron Timezone")
		ret = true
	}

	if a.LoadCronScheduleState() {
		log.Println("Found and applied configuration for Cron Schedule State")
		ret = true
	}

	if a.loadPatchBeanstalkUserdata() {
		log.Println("Found and applied configuration for Beanstalk Userdata")
		ret = true
	}

	if a.loadGP2ConversionThreshold() {
		log.Println("Found and applied configuration for GP2 Conversion Threshold")
		ret = true
	}

	if a.loadSpotAllocationStrategy() {
		log.Println("Found and applied configuration for Spot Allocation Strategy")
		ret = true
	}

	if a.loadPrioritizedInstanceTypesBiasTag() {
		log.Println("Found and applied configuration for Prioritized Instance Types Bias")
		ret = true
	}

	return ret
}

func (a *autoScalingGroup) loadDefaultConfigNumber() (int64, bool) {
	onDemand := a.region.conf.MinOnDemandNumber
	if onDemand >= 0 && onDemand <= int64(a.instances.count()) {
		log.Printf("Loaded default value %d from conf number.", onDemand)
		return onDemand, true
	}
	log.Println("Ignoring default value out of range:", onDemand)
	return DefaultMinOnDemandValue, false
}

func (a *autoScalingGroup) loadDefaultConfigPercentage() (int64, bool) {
	percentage := a.region.conf.MinOnDemandPercentage
	if percentage < 0 || percentage > 100 {
		log.Printf("Ignoring default value out of range: %f", percentage)
		return DefaultMinOnDemandValue, false
	}
	instanceNumber := a.instances.count()
	onDemand := int64(math.Floor((float64(instanceNumber) * percentage / 100.0) + .5))
	log.Printf("Loaded default value %d from conf percentage.", onDemand)
	return onDemand, true
}

func (a *autoScalingGroup) loadDefaultConfig() bool {
	done := false
	a.config.MinOnDemand = DefaultMinOnDemandValue

	if a.region.conf.SpotPriceBufferPercentage <= 0 {
		a.region.conf.SpotPriceBufferPercentage = DefaultSpotPriceBufferPercentage
	}

	if a.region.conf.MinOnDemandNumber != 0 {
		a.config.MinOnDemand, done = a.loadDefaultConfigNumber()
	}
	if !done && a.region.conf.MinOnDemandPercentage != 0 {
		a.config.MinOnDemand, done = a.loadDefaultConfigPercentage()
	} else {
		log.Println("No default value for on-demand instances specified, skipping.")
	}
	return done
}
