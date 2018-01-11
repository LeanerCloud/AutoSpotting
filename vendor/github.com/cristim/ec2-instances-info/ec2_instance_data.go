package ec2instancesinfo

// In this file we generate a raw data structure unmarshaled from the
// ec2instances.info JSON file, embedded into the binary at build time based on
// a generated snipped created using the go-bindata tool.

import (
	"encoding/json"

	"github.com/cristim/ec2-instances-info/data"
	"github.com/pkg/errors"
)

// AWS Instances JSON Structure Definitions
type jsonInstance struct {
	Family             string          `json:"family"`
	EnhancedNetworking bool            `json:"enhanced_networking"`
	VCPURaw            json.RawMessage `json:"vCPU"`
	VCPU               int
	Generation         string                  `json:"generation"`
	EBSIOPS            float32                 `json:"ebs_iops"`
	NetworkPerformance string                  `json:"network_performance"`
	EBSThroughput      float32                 `json:"ebs_throughput"`
	PrettyName         string                  `json:"pretty_name"`
	GPU                int                     `json:"GPU"`
	Pricing            map[string]RegionPrices `json:"pricing"`

	Storage *StorageConfiguration `json:"storage"`

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

type StorageConfiguration struct {
	SSD     bool    `json:"ssd"`
	Devices int     `json:"devices"`
	Size    float32 `json:"size"`
}

type RegionPrices struct {
	Linux        LinuxPricing `json:"linux"`
	EBSSurcharge float64      `json:"ebs,string"`
	// ignored for now
	// Mswinsqlweb interface{}  `json:"mswinSQLWeb"`
	// Mswinsql    interface{}  `json:"mswinSQL"`
	// Mswin       interface{}  `json:"mswin"`
}

type LinuxPricing struct {
	OnDemand float64 `json:"ondemand,string"`
	// ignored for now
	// Reserved interface{} `json:"reserved"`
}

//------------------------------------------------------------------------------

// InstanceData is a large data structure containing pricing and specs
// information about all the EC2 instance types from all AWS regions.
type InstanceData []jsonInstance

// Data generates the InstanceData object based on data sourced from
// ec2instances.info. The data is available there as a JSON blob, which is
// converted into golang source-code by the go-bindata tool and unmarshaled into
// a golang data structure by this library.
func Data() (*InstanceData, error) {

	var d InstanceData

	raw, err := data.Asset("data/instances.json")
	if err != nil {
		return nil, errors.Errorf("couldn't read the data asset: %s", err.Error())
	}

	err = json.Unmarshal(raw, &d)
	if err != nil {
		return nil, errors.Errorf("couldn't read the data asset: %s", err.Error())
	}

	// Handle "N/A" values in the VCPU field for i3.metal instance type
	for i := range d {
		var n int
		if err = json.Unmarshal(d[i].VCPURaw, &n); err == nil {
			d[i].VCPU = n
		}
	}

	return &d, nil
}
