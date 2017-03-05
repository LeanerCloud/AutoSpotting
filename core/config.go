package autospotting

import (
	"io"

	"github.com/cristim/autospotting/ec2instancesinfo"
)

// Config contains a number of flags and static data storing the EC2 instance
// information.
type Config struct {

	// Static data fetched from ec2instances.info
	InstanceData *ec2instancesinfo.InstanceData

	// Logging
	LogFile io.Writer
	LogFlag int

	BuildNumber string

	MinOnDemandNumber     int64
	MinOnDemandPercentage float64
	Regions               string
}
