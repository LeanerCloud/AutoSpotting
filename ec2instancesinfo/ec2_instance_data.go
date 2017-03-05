package ec2instancesinfo

// In this file we generate a raw data structure unmarshaled from the
// ec2instances.info JSON file, embedded into the binary at build time using go-
// bindata.

import (
	"encoding/json"

	"github.com/pkg/errors"
)

// AWS Instances JSON Structure Definitions
type jsonInstance struct {
	Family             string                  `json:"family"`
	EnhancedNetworking bool                    `json:"enhanced_networking"`
	VCPU               int                     `json:"vCPU"`
	Generation         string                  `json:"generation"`
	EBSIOPS            float32                 `json:"ebs_iops"`
	NetworkPerformance string                  `json:"network_performance"`
	EBSThroughput      float32                 `json:"ebs_throughput"`
	PrettyName         string                  `json:"pretty_name"`
	Pricing            map[string]regionPrices `json:"pricing"`

	Storage *storageConfiguration `json:"storage"`

	VPC struct {
		//    IPsPerENI int `json:"ips_per_eni"`
		//    MaxENIs   int `json:"max_enis"`
	} `json:"vpc"`

	Arch                     []string `json:"arch"`
	LinuxVirtualizationTypes []string `json:"linux_virtualization_types"`
	EBSOptimized             bool     `json:"ebs_optimized"`

	MaxBandwidth float32 `json:"max_bandwidth"`
	InstanceType string  `json:"instance_type"`

	// ECU is ignored because it's useless and also unreliable when parsing the
	// data structure: usually it's a number, but it can also be the string
	// "variable"
	// ECU float32 `json:"ECU"`

	Memory          float32 `json:"memory"`
	EBSMaxBandwidth float32 `json:"ebs_max_bandwidth"`
}

type storageConfiguration struct {
	SSD     bool    `json:"ssd"`
	Devices int     `json:"devices"`
	Size    float32 `json:"size"`
}

type regionPrices struct {
	Linux Linux `json:"linux"`
	// ignored for now, not useful
	// Mswinsqlweb interface{}  `json:"mswinSQLWeb"`
	// Mswinsql    interface{}  `json:"mswinSQL"`
	// Mswin       interface{}  `json:"mswin"`
}

type Linux struct {
	// this may contain string encoded numbers or "N/A" in some regions for
	// regionally unsupported instance types. It needs special parsing later
	OnDemand string `json:"ondemand"`
	// ignored for now, not really useful
	// Reserved interface{} `json:"reserved"`
}

//------------------------------------------------------------------------------

// InstanceData is a large data structure containing pricing and hardware
// information about all the EC2 instance types from all AWS regions.
type InstanceData []jsonInstance

// Load generates the RawInstanceData object based on data sourced from
// ec2instances.info. The data is shipped as a JSON blob, which is converted
// into golang source-code by the go-bindata tool and compiled into this
// library.
func Data() (*InstanceData, error) {

	var data InstanceData

	raw, err := Asset("instances.json")
	if err != nil {
		return nil, errors.Errorf("couldn't read the data asset")
	}

	err = json.Unmarshal(raw, &data)
	if err != nil {
		return nil, errors.Errorf("couldn't read the data asset")
	}

	return &data, nil
}
