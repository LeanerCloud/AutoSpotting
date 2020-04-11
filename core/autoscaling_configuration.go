// Copyright (c) 2016-2019 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
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

	// DefaultInstanceTerminationMethod is the default value for the instance termination
	// method configuration option
	DefaultInstanceTerminationMethod = AutoScalingTerminationMethod

	// ScheduleTag is the name of the tag set on the AutoScaling Group that
	// can override the global value of the Schedule parameter
	ScheduleTag = "autospotting_cron_schedule"

	// TimezoneTag is the name of the tag set on the AutoScaling Group that
	// can override the global value of the Timezone parameter
	TimezoneTag = "autospotting_cron_timezone"

	// CronScheduleState controls whether to run or not to run during the time interval
	// specified in the Schedule variable or its per-group tag overrides. It
	// accepts "on|off" as valid values
	CronScheduleState = "on"

	// CronScheduleStateTag is the name of the tag set on the AutoScaling Group that
	// can override the global value of the CronScheduleState parameter
	CronScheduleStateTag = "autospotting_cron_schedule_state"

	// PatchBeanstalkUserdataTag is the name of the tag set on the AutoScaling Group that
	// can override the global value of the PatchBeanstalkUserdata parameter
	PatchBeanstalkUserdataTag = "patch_beanstalk_userdata"
)

// AutoScalingConfig stores some group-specific configurations that can override
// their corresponding global values
type AutoScalingConfig struct {
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

	PatchBeanstalkUserdata string
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
	} else if spotPriceBufferPercentage < 0 {
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

func (a *autoScalingGroup) getTagValue(keyMatch string) *string {
	for _, asgTag := range a.Tags {
		if *asgTag.Key == keyMatch {
			return asgTag.Value
		}
	}
	return nil
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

func (a *autoScalingGroup) loadPatchBeanstalkUserdata() {
	tagValue := a.getTagValue(PatchBeanstalkUserdataTag)

	if tagValue != nil {
		logger.Printf("Loaded PatchBeanstalkUserdata value %v from tag %v\n", *tagValue, PatchBeanstalkUserdataTag)
		a.config.PatchBeanstalkUserdata = *tagValue
		return
	}

	debug.Println("Couldn't find tag", PatchBeanstalkUserdataTag, "on the group", a.name, "using the default configuration")
	a.config.PatchBeanstalkUserdata = a.region.conf.PatchBeanstalkUserdata
}

func (a *autoScalingGroup) loadBiddingPolicy(tagValue *string) (string, bool) {
	biddingPolicy := *tagValue
	if biddingPolicy != "aggressive" {
		return DefaultBiddingPolicy, false
	}

	logger.Printf("Loaded BiddingPolicy value with %s from tag %s\n", biddingPolicy, BiddingPolicyTag)
	return biddingPolicy, true
}

func (a *autoScalingGroup) LoadCronSchedule() {
	tagValue := a.getTagValue(ScheduleTag)

	if tagValue != nil {
		logger.Printf("Loaded CronSchedule value %v from tag %v\n", *tagValue, ScheduleTag)
		a.config.CronSchedule = *tagValue
		return
	}

	debug.Println("Couldn't find tag", ScheduleTag, "on the group", a.name, "using the default configuration")
	a.config.CronSchedule = a.region.conf.CronSchedule
}

func (a *autoScalingGroup) LoadCronTimezone() {
	tagValue := a.getTagValue(TimezoneTag)

	if tagValue != nil {
		logger.Printf("Loaded CronTimezone value %v from tag %v\n", *tagValue, TimezoneTag)
		a.config.CronTimezone = *tagValue
		return
	}

	debug.Println("Couldn't find tag", TimezoneTag, "on the group", a.name, "using the default configuration")
	a.config.CronTimezone = a.region.conf.CronTimezone
}

func (a *autoScalingGroup) LoadCronScheduleState() {
	tagValue := a.getTagValue(CronScheduleStateTag)
	if tagValue != nil {
		logger.Printf("Loaded CronScheduleState value %v from tag %v\n", *tagValue, CronScheduleStateTag)
		a.config.CronScheduleState = *tagValue
		return
	}

	debug.Println("Couldn't find tag", CronScheduleStateTag, "on the group", a.name, "using the default configuration")
	a.config.CronScheduleState = a.region.conf.CronScheduleState
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

// Add configuration of other elements here: prices, whitelisting, etc
func (a *autoScalingGroup) loadConfigFromTags() bool {

	resOnDemandConf := a.loadConfOnDemand()

	resSpotConf := a.loadConfSpot()

	resSpotPriceConf := a.loadConfSpotPrice()

	a.LoadCronSchedule()
	a.LoadCronTimezone()
	a.LoadCronScheduleState()
	a.loadPatchBeanstalkUserdata()

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
