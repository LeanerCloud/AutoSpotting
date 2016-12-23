package autospotting

import (
	"io"
)

// Config contains a number of flags and static data storing the EC2 instance
// information.
type Config struct {

	// Static data fetched from ec2instances.info
	RawInstanceData RawInstanceData

	// Logging
	LogFile io.Writer
	LogFlag int

	BuildNumber string

	MinOnDemandNumber     int64
	MinOnDemandPercentage float64
	Regions               string
}
