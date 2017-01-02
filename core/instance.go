package autospotting

import (
	"fmt"
	"math"
	"time"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/davecgh/go-spew/spew"
)

// The key in this map is the instance ID, useful for quick retrieval of
// instance attributes.
type instances struct {
	catalog map[string]*instance
}

func (is *instances) add(inst *instance) {
	debug.Println(inst)
	is.catalog[*inst.InstanceId] = inst
}

func (is *instances) get(id string) (inst *instance) {
	return is.catalog[id]
}

type instance struct {
	*ec2.Instance
	typeInfo instanceTypeInformation
	price    float64
	region   *region
	asg      *autoScalingGroup
}

type instanceTypeInformation struct {
	instanceType             string
	vCPU                     int
	pricing                  prices
	memory                   float32
	virtualizationTypes      []string
	hasInstanceStore         bool
	instanceStoreDeviceSize  float32
	instanceStoreDeviceCount int
	instanceStoreIsSSD       bool
}

func (i *instance) isSpot() bool {
	return (i.InstanceLifecycle != nil &&
		*i.InstanceLifecycle == "spot")
}

func (i *instance) terminate() {

	if _, err := i.region.services.ec2.TerminateInstances(
		&ec2.TerminateInstancesInput{
			InstanceIds: []*string{i.InstanceId},
		}); err != nil {
		logger.Println(err.Error())
	}
}

func (i *instance) getCompatibleSpotInstanceTypes(lc *launchConfiguration) ([]string, error) {

	logger.Println("Getting spot instances compatible to ",
		*i.InstanceId, " of type", *i.InstanceType)

	debug.Println("Using this data as reference", spew.Sdump(i))

	var filteredInstanceTypes []string

	existing := i.typeInfo
	debug.Println("Using this data as reference", existing)

	debug.Println("Instance Data", spew.Sdump(i.typeInfo))

	// Count the ephemeral volumes attached to the original instance's block
	// device mappings, this number is used later when comparing with each
	// instance type.
	lcMappings, err := lc.countLaunchConfigEphemeralVolumes()

	if err == nil {
		logger.Println("Couldn't determine the launch configuration device mapping",
			"configuration")
	}

	attachedVolumesNumber := min(lcMappings, existing.instanceStoreDeviceCount)
	availabilityZone := *i.Placement.AvailabilityZone

	//filtering compatible instance types
	for _, candidate := range i.region.instanceTypeInformation {

		logger.Println("\nComparing ", candidate, " with ", existing)

		spotPriceNewInstance := candidate.pricing.spot[availabilityZone]

		if spotPriceNewInstance == 0 {
			logger.Println("Missing spot pricing information, skipping",
				candidate.instanceType)
			continue
		}

		if spotPriceNewInstance <= i.price {
			logger.Println("pricing compatible, continuing evaluation: ",
				candidate.pricing.spot[availabilityZone], "<=",
				i.price)
		} else {
			logger.Println("price too high, skipping", candidate.instanceType)
			continue
		}

		if candidate.vCPU >= existing.vCPU {
			logger.Println("CPU compatible, continuing evaluation")
		} else {
			logger.Println("Insuficient CPU cores, skipping", candidate.instanceType)
			continue
		}

		if candidate.memory >= existing.memory {
			logger.Println("memory compatible, continuing evaluation")
		} else {
			logger.Println("memory incompatible, skipping", candidate.instanceType)
			continue
		}

		// Here we check the storage compatibility, with the following evaluation
		// criteria:
		// - speed: don't accept spinning disks when we used to have SSDs
		// - number of volumes: the new instance should have enough volumes to be
		//   able to attach all the instance store device mappings defined on the
		//   original instance
		// - volume size: each of the volumes should be at least as big as the
		//   original instance's volumes

		if attachedVolumesNumber > 0 {
			logger.Println("Checking the new instance's ephemeral storage",
				"configuration because the initial instance had attached",
				"ephemeral instance store volumes")

			if candidate.instanceStoreDeviceCount >= attachedVolumesNumber {
				logger.Println("instance store volume count compatible,",
					"continuing evaluation")
			} else {
				logger.Println("instance store volume count incompatible, skipping",
					candidate.instanceType)
				continue
			}

			if candidate.instanceStoreDeviceSize >= existing.instanceStoreDeviceSize {
				logger.Println("instance store volume size compatible,",
					"continuing evaluation")
			} else {
				logger.Println("instance store volume size incompatible, skipping",
					candidate.instanceType)
				continue
			}

			// Don't accept ephemeral spinning disks if the original instance has
			// ephemeral SSDs, but accept spinning disks if we had those before.
			if candidate.instanceStoreIsSSD ||
				(candidate.instanceStoreIsSSD == existing.instanceStoreIsSSD) {
				logger.Println("instance store type(SSD/spinning) compatible,",
					"continuing evaluation")
			} else {
				logger.Println("instance store type(SSD/spinning) incompatible,",
					"skipping", candidate.instanceType)
				continue
			}
		}

		if compatibleVirtualization(*i.VirtualizationType,
			candidate.virtualizationTypes) {
			logger.Println("virtualization compatible, continuing evaluation")
		} else {
			logger.Println("virtualization incompatible, skipping",
				candidate.instanceType)
			continue
		}

		// checking how many spot instances of this type we already have, so that
		// we can see how risky it is to launch a new one.
		spotInstanceCount := i.asg.alreadyRunningSpotInstanceTypeCount(
			candidate.instanceType, availabilityZone)

		// We skip it in case we have more than 20% instances of this type already
		// running
		if spotInstanceCount == 0 ||
			(*i.asg.DesiredCapacity/spotInstanceCount > 4) {
			logger.Println(i.asg.name,
				"no redundancy issues found for", candidate.instanceType,
				"existing", spotInstanceCount,
				"spot instances, adding for comparison",
			)

			filteredInstanceTypes = append(filteredInstanceTypes, candidate.instanceType)
		} else {
			logger.Println("\nInstances ", candidate, " and ", existing,
				"are not compatible or resulting redundancy for the availability zone",
				"would be dangerously low")

		}

	}
	logger.Printf("\n Found following compatible instances: %#v\n",
		filteredInstanceTypes)
	return filteredInstanceTypes, nil

}

func (i *instance) getCheapestCompatibleSpotInstanceType() (*string, error) {

	logger.Println("Getting cheapest spot instance compatible to ",
		*i.InstanceId, " of type", *i.InstanceType)

	filteredInstanceTypes, err := i.getCompatibleSpotInstanceTypes(
		i.asg.getLaunchConfiguration())

	if err != nil {
		logger.Println("Couldn't find any compatible instance types", err)
		return nil, err
	}

	minPrice := math.MaxFloat64
	var chosenInstanceType string

	for _, instanceType := range filteredInstanceTypes {
		price := i.typeInfo.pricing.spot[*i.Placement.AvailabilityZone]

		if price < minPrice {
			minPrice, chosenInstanceType = price, instanceType
			logger.Println(i.InstanceId, "changed current minimum to ", minPrice)
		}
		logger.Println(i.InstanceId, "cheapest instance type so far is ",
			chosenInstanceType, "priced at", minPrice)
	}

	if chosenInstanceType != "" {
		logger.Println("Chose cheapest instance type", chosenInstanceType)
		return &chosenInstanceType, nil
	}
	logger.Println("Couldn't find any cheaper spot instance type")
	return nil, fmt.Errorf("no cheaper spot instance types could be found")

}

func (i *instance) tag(tags []*ec2.Tag) {

	if len(tags) == 0 {
		logger.Println(i.region.name, "Tagging spot instance", *i.InstanceId,
			"no tags were defined, skipping...")
		return
	}

	svc := i.region.services.ec2
	params := ec2.CreateTagsInput{
		Resources: []*string{i.InstanceId},
		Tags:      tags,
	}

	logger.Println(i.region.name, "Tagging spot instance", *i.InstanceId)

	for _, err := svc.CreateTags(&params); err != nil; _, err = svc.CreateTags(&params) {

		logger.Println(i.region.name,
			"Failed to create tags for the spot instance", *i.InstanceId, err.Error())

		logger.Println(i.region.name,
			"Sleeping for 5 seconds before retrying")

		time.Sleep(5 * time.Second)
	}

	logger.Println("Instance", *i.InstanceId,
		"was tagged with the following tags:", tags)
}

func compatibleVirtualization(virtualizationType string,
	availableVirtualizationTypes []string) bool {

	logger.Println("Available: ", availableVirtualizationTypes,
		"Tested: ", virtualizationType)

	for _, avt := range availableVirtualizationTypes {
		if (avt == "PV") && (virtualizationType == "paravirtual") ||
			(avt == "HVM") && (virtualizationType == "hvm") {
			logger.Println("Compatible")
			return true
		}
	}
	return false
}

// Why the heck isn't this in the Go standard library?
func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}
