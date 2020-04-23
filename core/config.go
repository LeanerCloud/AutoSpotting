// Copyright (c) 2016-2019 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws/endpoints"
	ec2instancesinfo "github.com/cristim/ec2-instances-info"
	"github.com/namsral/flag"
)

const (
	// AutoScalingTerminationMethod uses the TerminateInstanceInAutoScalingGroup
	// API method to terminate instances.  This method is recommended because it
	// will require termination Lifecycle Hooks that have been configured on the
	// Auto Scaling Group to be invoked before terminating the instance.  It's
	// also safe even if there are no such hooks configured.
	AutoScalingTerminationMethod = "autoscaling"

	// DetachTerminationMethod detaches the instance from the Auto Scaling Group
	// and then terminates it.  This method exists for historical reasons and is
	// no longer recommended.
	DetachTerminationMethod = "detach"

	// TerminateTerminationNotificationAction terminate the spot instance, which will be terminated
	// by AWS in 2 minutes, without reducing the ASG capacity, so that a new instance will
	// be launched. LifeCycle Hooks are triggered.
	TerminateTerminationNotificationAction = "terminate"

	// DetachTerminationNotificationAction detach the spot instance, which will be terminated
	// by AWS in 2 minutes, without reducing the ASG capacity, so that a new instance will
	// be launched. LifeCycle Hooks are not triggered.
	DetachTerminationNotificationAction = "detach"

	// AutoTerminationNotificationAction if ASG has a LifeCycleHook with LifecycleTransition = EC2_INSTANCE_TERMINATING
	// terminate the spot instance (as TerminateTerminationNotificationAction), if not detach it.
	AutoTerminationNotificationAction = "auto"

	// DefaultSchedule is the default value for the execution schedule in
	// simplified Cron-style definition the cron format only accepts the hour and
	// day of week fields, for example "9-18 1-5" would define the working week
	// hours. AutoSpotting will only run inside this time interval. The action can
	// also be reverted using the CronScheduleState parameter, so in order to run
	// outside this interval set the CronScheduleStateq qto "off" either globally or
	// on a per-group override.
	DefaultSchedule = "* *"
)

// Config extends the AutoScalingConfig struct and in addition contains a
// number of global flags.
type Config struct {
	AutoScalingConfig

	// Static data fetched from ec2instances.info
	InstanceData *ec2instancesinfo.InstanceData

	// Logging
	LogFile io.Writer
	LogFlag int

	// The regions where it should be running
	Regions string

	// The region where the Lambda function is deployed
	MainRegion string

	// This is only here for tests, where we want to be able to somehow mock
	// time.Sleep without actually sleeping. While testing it defaults to 0 (which won't sleep at all), in
	// real-world usage it's expected to be set to 1
	SleepMultiplier time.Duration

	// Filter on ASG tags
	// for example: spot-enabled=true,environment=dev,team=interactive
	FilterByTags string
	// Controls how are the tags used to filter the groups.
	// Available options: 'opt-in' and 'opt-out', default: 'opt-in'
	TagFilteringMode string

	// The AutoSpotting version
	Version string

	// The license of this AutoSpotting build
	LicenseType string

	// Controls whether AutoSpotting patches Elastic Beanstalk UserData scripts to use
	// the instance role when calling CloudFormation helpers instead of the standard CloudFormation
	// authentication method
	PatchBeanstalkUserdata string
}

// ParseConfig loads configuration from command line flags, environments variables, and config files.
func ParseConfig(conf *Config) {

	// The use of FlagSet allows us to parse config multiple times, which is useful for unit tests.
	flagSet := flag.NewFlagSet("AutoSpotting", flag.ExitOnError)

	var region string

	if r := os.Getenv("AWS_REGION"); r != "" {
		region = r
	} else {
		region = endpoints.UsEast1RegionID
	}

	conf.LogFile = os.Stdout
	conf.LogFlag = log.Ldate | log.Ltime | log.Lshortfile
	conf.MainRegion = region
	conf.SleepMultiplier = 1

	flagSet.StringVar(&conf.AllowedInstanceTypes, "allowed_instance_types", "",
		"\n\tIf specified, the spot instances will be searched only among these types.\n\tIf missing, any instance type is allowed.\n"+
			"\tAccepts a list of comma or whitespace separated instance types (supports globs).\n"+
			"\tExample: ./AutoSpotting -allowed_instance_types 'c5.*,c4.xlarge'\n")
	flagSet.StringVar(&conf.BiddingPolicy, "bidding_policy", DefaultBiddingPolicy,
		"\n\tPolicy choice for spot bid. If set to 'normal', we bid at the on-demand price(times the multiplier).\n"+
			"\tIf set to 'aggressive', we bid at a percentage value above the spot price \n"+
			"\tconfigurable using the spot_price_buffer_percentage.\n")
	flagSet.StringVar(&conf.DisallowedInstanceTypes, "disallowed_instance_types", "",
		"\n\tIf specified, the spot instances will _never_ be of these types.\n"+
			"\tAccepts a list of comma or whitespace separated instance types (supports globs).\n"+
			"\tExample: ./AutoSpotting -disallowed_instance_types 't2.*,c4.xlarge'\n")
	flagSet.StringVar(&conf.InstanceTerminationMethod, "instance_termination_method", DefaultInstanceTerminationMethod,
		"\n\tInstance termination method.  Must be one of '"+DefaultInstanceTerminationMethod+"' (default),\n"+
			"\t or 'detach' (compatibility mode, not recommended)\n")
	flagSet.StringVar(&conf.TerminationNotificationAction, "termination_notification_action", DefaultTerminationNotificationAction,
		"\n\tTermination Notification Action.\n"+
			"\tValid choices:\n"+
			"\t'"+DefaultTerminationNotificationAction+
			"' (terminate if lifecyclehook else detach) | 'terminate' (lifecyclehook triggered)"+
			" | 'detach' (lifecyclehook not triggered)\n")
	flagSet.Int64Var(&conf.MinOnDemandNumber, "min_on_demand_number", DefaultMinOnDemandValue,
		"\n\tNumber of on-demand nodes to be kept running in each of the groups.\n\t"+
			"Can be overridden on a per-group basis using the tag "+OnDemandNumberLong+".\n")
	flagSet.Float64Var(&conf.MinOnDemandPercentage, "min_on_demand_percentage", 0.0,
		"\n\tPercentage of the total number of instances in each group to be kept on-demand\n\t"+
			"Can be overridden on a per-group basis using the tag "+OnDemandPercentageTag+
			"\n\tIt is ignored if min_on_demand_number is also set.\n")
	flagSet.Float64Var(&conf.OnDemandPriceMultiplier, "on_demand_price_multiplier", 1.0,
		"\n\tMultiplier for the on-demand price. Numbers less than 1.0 are useful for volume discounts.\n"+
			"\tExample: ./AutoSpotting -on_demand_price_multiplier 0.6 will have the on-demand price "+
			"considered at 60% of the actual value.\n")
	flagSet.StringVar(&conf.Regions, "regions", "",
		"\n\tRegions where it should be activated (separated by comma or whitespace, also supports globs).\n"+
			"\tBy default it runs on all regions.\n"+
			"\tExample: ./AutoSpotting -regions 'eu-*,us-east-1'\n")
	flagSet.Float64Var(&conf.SpotPriceBufferPercentage, "spot_price_buffer_percentage", DefaultSpotPriceBufferPercentage,
		"\n\tBid a given percentage above the current spot price.\n\tProtects the group from running spot"+
			"instances that got significantly more expensive than when they were initially launched\n"+
			"\tThe tag "+SpotPriceBufferPercentageTag+" can be used to override this on a group level.\n"+
			"\tIf the bid exceeds the on-demand price, we place a bid at on-demand price itself.\n")
	flagSet.StringVar(&conf.SpotProductDescription, "spot_product_description", DefaultSpotProductDescription,
		"\n\tThe Spot Product to use when looking up spot price history in the market.\n"+
			"\tValid choices: Linux/UNIX | SUSE Linux | Windows | Linux/UNIX (Amazon VPC) | \n"+
			"\tSUSE Linux (Amazon VPC) | Windows (Amazon VPC) | Red Hat Enterprise Linux\n\tDefault value: "+DefaultSpotProductDescription+"\n")
	flagSet.Float64Var(&conf.SpotProductPremium, "spot_product_premium", DefaultSpotProductPremium,
		"\n\tThe Product Premium to apply to the on demand price to improve spot selection and savings calculations\n"+
			"\twhen using a premium instance type such as RHEL.")
	flagSet.StringVar(&conf.TagFilteringMode, "tag_filtering_mode", "opt-in", "\n\tControls the behavior of the tag_filters option.\n"+
		"\tValid choices: opt-in | opt-out\n\tDefault value: 'opt-in'\n\tExample: ./AutoSpotting --tag_filtering_mode opt-out\n")
	flagSet.StringVar(&conf.FilterByTags, "tag_filters", "", "\n\tSet of tags to filter the ASGs on.\n"+
		"\tDefault if no value is set will be the equivalent of -tag_filters 'spot-enabled=true'\n"+
		"\tIn case the tag_filtering_mode is set to opt-out, it defaults to 'spot-enabled=false'\n"+
		"\tExample: ./AutoSpotting --tag_filters 'spot-enabled=true,Environment=dev,Team=vision'\n")

	flagSet.StringVar(&conf.CronSchedule, "cron_schedule", "* *", "\n\tCron-like schedule in which to"+
		"\tperform(or not) spot replacement actions. Format: hour day-of-week\n"+
		"\tExample: ./AutoSpotting --cron_schedule '9-18 1-5' # workdays during the office hours \n")
	flagSet.StringVar(&conf.CronTimezone, "cron_timezone", "UTC", "\n\tTimezone to"+
		"\tperform(or not) spot replacement actions. Format: timezone\n"+
		"\tExample: ./AutoSpotting --cron_timezone 'Europe/London' \n")

	flagSet.StringVar(&conf.CronScheduleState, "cron_schedule_state", "on", "\n\tControls whether to take actions "+
		"inside or outside the schedule defined by cron_schedule. Allowed values: on|off\n"+
		"\tExample: ./AutoSpotting --cron_schedule_state='off' --cron_schedule '9-18 1-5'  # would only take action outside the defined schedule\n")
	flagSet.StringVar(&conf.LicenseType, "license", "evaluation", "\n\tControls the terms under which you use AutoSpotting"+
		"Allowed values: evaluation|I_am_supporting_it_on_Patreon|I_contributed_to_development_within_the_last_year|I_built_it_from_source_code\n"+
		"\tExample: ./AutoSpotting --license evaluation\n")
	flagSet.StringVar(&conf.PatchBeanstalkUserdata, "patch_beanstalk_userdata", "", "\n\tControls whether AutoSpotting patches Elastic Beanstalk UserData scripts to use the instance role when calling CloudFormation helpers instead of the standard CloudFormation authentication method\n"+
		"\tExample: ./AutoSpotting --patch_beanstalk_userdata true\n")

	printVersion := flagSet.Bool("version", false, "Print version number and exit.\n")

	if err := flagSet.Parse(os.Args[1:]); err != nil {
		fmt.Printf("Error parsing config: %s\n", err.Error())
	}

	if *printVersion {
		fmt.Println("AutoSpotting build:", conf.Version)
		os.Exit(0)
	}

	data, err := ec2instancesinfo.Data()
	if err != nil {
		log.Fatal(err.Error())
	}
	conf.InstanceData = data
}
