package main

import (
	"fmt"
	"os"

	"github.com/cristim/ec2-instances-info"
)

func main() {

	data, err := ec2instancesinfo.Data()

	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	for _, i := range *data {
		fmt.Println("Instance type", i.InstanceType,
			"CPU:", i.VCPU, "RAM:", i.Memory,
			"cost in us-east-1: ", i.Pricing["us-east-1"].Linux.OnDemand,
			"EBS surcharge: ", i.Pricing["us-east-1"].EBSSurcharge)
	}

}
