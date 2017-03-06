package autospotting

import (
	"io"

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

	MinOnDemandNumber     int64
	MinOnDemandPercentage float64
	Regions               string
}
