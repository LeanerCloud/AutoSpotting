package autospotting

import (
	"io"
)

// Config contains a number of flags and static data storing the EC2 instance
// information.
type Config struct {
	// Configuration options that can be overridden at runtime based on
	// CloudWatch Event information.
	EventOptions

	// Static data fetched from ec2instances.info
	RawInstanceData RawInstanceData

	// Logging
	LogFile io.Writer
	LogFlag int

	BuildNumber string
}

// EventOptions contains a number of settings that can be dynamically configured
// at runtime, based in data coming from each CloudWatch event. It is also used
// for parsing the CloudWatch Event JSON payload.
type EventOptions struct {
	MinOnDemandNumber     int64   `json:"MinOnDemandNumber"`
	MinOnDemandPercentage float64 `json:"MinOnDemandPercentage"`
	Regions               string  `json:"Regions"`
}
