package autospotting

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

// data structure that stores information about a region
type region struct {
	name         string
	instanceData map[string]instanceInformation // the key in this map is the instance type
	enabledAsgs  []autoScalingGroup
	services     connections
	wg           sync.WaitGroup
	instances    map[string]*ec2.Instance // the key in this map is the instance ID, useful for quick retrieval of instance attributes
}

type instanceInformation struct {
	instanceType        string
	vCPU                int
	pricing             prices
	memory              float32
	virtualizationTypes []string
}

type prices struct {
	onDemand float64
	spot     spotPriceMap
}

// The key in this map is the availavility zone
type spotPriceMap map[string]float64

func (r *region) processRegion(instData *jsonInstances) {
	fmt.Println("Creating connections to the required AWS services in", r.name)
	r.services.connect(r.name)
	// only process the regions where we have AutoScaling groups set to be handled

	fmt.Println("Scanning for enabled AutoScaling groups in ", r.name)
	r.scanEnabledAutoScalingGroups()

	// only process further the region if there are any enabled autoscaling groups within it
	if r.hasEnabledAutoScalingGroups() {
		fmt.Println("Scanning instances in", r.name)
		r.scanInstances()

		fmt.Println("Scanning full instance information in", r.name)
		r.determineInstanceInformation(instData)

		fmt.Println("Processing enabled AutoScaling groups in", r.name)
		r.processEnabledAutoScalingGroups()
	}
}

func (r *region) determineInstanceInformation(instData *jsonInstances) {

	r.instanceData = make(map[string]instanceInformation)
	for _, it := range *instData {

		var price prices

		// populate on-demand information
		price.onDemand, _ = strconv.ParseFloat(it.Pricing[r.name].Linux.OnDemand, 64)

		price.spot = make(spotPriceMap)

		// if at this point the instance price is still zero, then that
		// particular instance type doesn't even exist in the current
		// region, so we don't even need to create an empty spot pricing
		// data structure for it
		if price.onDemand > 0 {
			// for each instance type populate the HW spec information
			r.instanceData[it.InstanceType] = instanceInformation{
				instanceType:        it.InstanceType,
				vCPU:                it.VCPU,
				memory:              it.Memory,
				pricing:             price,
				virtualizationTypes: it.LinuxVirtualizationTypes,
			}
		}
	}
	// this is safe to do once outside of the loop because the call will only return entries
	// about the available instance types, so no invalid instance types would be returned
	r.requestSpotPrices()

	// fmt.Printf("%#v\n", r.instanceData)
}

func (r *region) requestSpotPrices() error {

	fmt.Println(r.name, "Requesting spot prices")
	ec2Conn := r.services.ec2
	params := &ec2.DescribeSpotPriceHistoryInput{
		ProductDescriptions: []*string{
			aws.String("Linux/UNIX"),
		},
		StartTime: aws.Time(time.Now()),
		EndTime:   aws.Time(time.Now()),
	}

	resp, err := ec2Conn.DescribeSpotPriceHistory(params)

	if err != nil {
		fmt.Println(r.name, "Failed requesting spot prices:", err.Error())
		return err
	}

	spotPrices := resp.SpotPriceHistory

	// fmt.Println("Spot Price list in ", r.name, ":\n", spotPrices)

	for _, priceInfo := range spotPrices {

		price, err := strconv.ParseFloat(*priceInfo.SpotPrice, 64)
		if err != nil { // failure to parse means that the instance is not available as spot instance
			continue
		}

		instType, az := *priceInfo.InstanceType, *priceInfo.AvailabilityZone

		r.instanceData[instType].pricing.spot[az] = price

	}

	return nil
}

func (r *region) scanEnabledAutoScalingGroups() {
	filterTagName, filterValue := "spot-enabled", "true"

	svc := r.services.autoScaling
	resp, err := svc.DescribeAutoScalingGroups(nil)

	if err != nil {
		fmt.Println("scanEnabledAutoScalingGroups:", err.Error())
		return
	}

	for _, asg := range resp.AutoScalingGroups {
		for _, tag := range asg.Tags {
			if *tag.Key == filterTagName && *tag.Value == filterValue {
				var group autoScalingGroup
				group.create(r, asg)
				r.enabledAsgs = append(r.enabledAsgs, group)
			}
		}
	}
}

func (r *region) hasEnabledAutoScalingGroups() bool {

	return len(r.enabledAsgs) > 0

}

func (r *region) processEnabledAutoScalingGroups() {
	for _, asg := range r.enabledAsgs {
		r.wg.Add(1)
		go func(a autoScalingGroup) {
			a.process()
			r.wg.Done()
		}(asg)
	}
	r.wg.Wait()
}

func (r *region) findSpotInstanceRequests(forAsgName string) []*ec2.SpotInstanceRequest {
	svc := r.services.ec2

	params := &ec2.DescribeSpotInstanceRequestsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("tag:launched-for-asg"),
				Values: []*string{aws.String(forAsgName)},
			},
		},
	}
	resp, err := svc.DescribeSpotInstanceRequests(params)

	if err != nil {
		fmt.Println(err.Error())
		return nil
	}

	fmt.Println("Spot instance requests created for ", forAsgName, "\n", resp.SpotInstanceRequests)
	return resp.SpotInstanceRequests

}

func (r *region) scanInstances() {
	svc := r.services.ec2
	params := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("instance-state-name"),
				Values: []*string{
					aws.String("running"),
					aws.String("pending"),
				},
			},
		},
	}
	resp, err := svc.DescribeInstances(params)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	r.instances = make(map[string]*ec2.Instance)
	if len(resp.Reservations) > 0 && resp.Reservations[0].Instances != nil {
		for _, res := range resp.Reservations {
			for _, inst := range res.Instances {
				r.instances[*inst.InstanceId] = inst
			}
		}
	}

	fmt.Println(r.instances)
}

func (r *region) tagInstance(instanceID string, tags []*ec2.Tag) {
	svc := r.services.ec2
	params := &ec2.CreateTagsInput{
		Resources: []*string{aws.String(instanceID)},
		Tags:      tags,
	}

	resp, err := svc.CreateTags(params)

	if err != nil {
		// Print the error, cast err to awserr.Error to get the Code and
		// Message from an error.
		fmt.Println(r.name, "Failed to create tags for the spot instance request:", err.Error())
		return
	}

	// Pretty-print the response data.
	fmt.Println(resp)
}
