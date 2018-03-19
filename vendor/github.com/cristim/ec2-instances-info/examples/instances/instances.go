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
		fmt.Print(
			"Instance type: ", i.InstanceType,
			",\tCPU cores: ", i.VCPU,
			",\tMemory(GB): ", i.Memory,
			",\tcost in us-east-1: ", i.Pricing["us-east-1"].Linux.OnDemand,
			",\tcost in eu-central-1: ")

		p := i.Pricing["eu-central-1"].Linux.OnDemand

		if p == 0 {
			fmt.Print("UNAVAILABLE")
		} else {
			fmt.Print(p)
		}

		fmt.Println(",\tEBS surcharge: ", i.Pricing["us-east-1"].EBSSurcharge)
	}

}
