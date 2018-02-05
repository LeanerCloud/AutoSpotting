package autospotting

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/cristim/ec2-instances-info"
)

// String formats the ArrayOfStrings type string[]
// into a string
func (i *Tags) String() string {
	return fmt.Sprintf("%v", *i)
}

// Set supports the multiple command line options with the same
// name
func (i *Tags) Set(value string) error {
	splitTagAndValue := strings.Split(value, "=")
	if len(splitTagAndValue) > 1 {
		*i = append(*i, Tag{Key: splitTagAndValue[0], Value: splitTagAndValue[1]})
	}
	return nil
}

// Tags allows the support for multiple
// command line options with the same name
// i.e -tag x -tag y
type Tags []Tag

// Tag represents an Asg Tag: Key, Value
type Tag struct {
	Key   string
	Value string
}

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
	BiddingPolicy             string

	// This is only here for tests, where we want to be able to somehow mock
	// time.Sleep without actually sleeping. While testing it defaults to 0 (which won't sleep at all), in
	// real-world usage it's expected to be set to 1
	SleepMultiplier time.Duration

	// Filter on extra ASG tags in addition to finding those
	// ASGs with spot-enabled=true
	FilterByTags Tags
}
