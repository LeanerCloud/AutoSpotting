package autospotting

import (
	"io"
	"time"

	"github.com/cristim/ec2-instances-info"
)

// Config contains a number of flags and static data storing the EC2 instance
// information.
type Config struct {

	// Static data fetched from ec2instances.info
	InstanceData *ec2instancesinfo.InstanceData

	// Logging
	LogFile io.Writer
	LogFlag int

	// The region where the Lambda function is deployed
	MainRegion string

	MinOnDemandNumber         int64
	MinOnDemandPercentage     float64
	Regions                   string
	AllowedInstanceTypes      string
	DisallowedInstanceTypes   string
	OnDemandPriceMultiplier   float64
	SpotPriceBufferPercentage float64
	SpotProductDescription    string
	BiddingPolicy             string

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
}
