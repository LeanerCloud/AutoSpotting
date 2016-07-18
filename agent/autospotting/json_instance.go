package autospotting

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
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
		//		IPsPerENI int `json:"ips_per_eni"`
		//		MaxENIs   int `json:"max_enis"`
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
	Linux struct {
		// this may contain string encoded numbers or "N/A" in some regions for
		// regionally unsupported instance types. It needs special parsing later
		OnDemand string `json:"ondemand"`
		// ignored for now, not really useful
		// Reserved interface{} `json:"reserved"`
	} `json:"linux"`

	// ignored for now, not useful
	// Mswinsqlweb interface{}  `json:"mswinSQLWeb"`
	// Mswinsql    interface{}  `json:"mswinSQL"`
	// Mswin       interface{}  `json:"mswin"`
}

//------------------------------------------------------------------------------

type jsonInstances []jsonInstance

func (i *jsonInstances) loadFromURL(url string) error {

	response, err := http.Get(url)
	if err != nil {
		logger.Println(err.Error())
		return err
	}

	defer response.Body.Close()

	contents, err := ioutil.ReadAll(response.Body)
	if err != nil {
		logger.Println(err.Error())
		return err
	}

	// logger.Println(string(contents))
	err = json.Unmarshal(contents, &i)
	if err != nil {
		logger.Println(err.Error())

		return err
	}

	return nil
}
