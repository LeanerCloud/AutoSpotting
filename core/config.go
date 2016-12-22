package autospotting

import (
	"io"
)

// Config contains a number of feature flags and static data storing the EC2
// instance information.
type Config struct {
	/*
		// TODO: make use of these in the code
		// Test data for mocking calls
		LoadTestData bool
		SaveTestData bool
		TestDataDir  string
		// Take no actions
		NoOp bool
	*/

	// Static data fetched from ec2instances.info
	RawInstanceData RawInstanceData

	// Logging
	LogFile io.Writer
	LogFlag int

	BuildNumber string

	Regions               string
	MinOnDemandNumber     int64
	MinOnDemandPercentage float64
}
