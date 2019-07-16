package autospotting

import (
	"io"
	"time"

	ec2instancesinfo "github.com/cristim/ec2-instances-info"
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
	// by AWS in 2 minutes, without reducing the ASG capcity, so that a new instance will
	// be launched. LifeCycle Hooks are triggered.
	TerminateTerminationNotificationAction = "terminate"

	// DetachTerminationNotificationAction detach the spot instance, which will be terminated
	// by AWS in 2 minutes, without reducing the ASG capcity, so that a new instance will
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

	// The regions where it should be running, given as a single CSV-string
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
}
