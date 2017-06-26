package autospotting

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/davecgh/go-spew/spew"
)

// The key in this map is the instance ID, useful for quick retrieval of
// instance attributes.
type instanceManager struct {
	sync.RWMutex
	catalog map[string]*instance
}

type instances interface {
	add(inst *instance)
	get(string) *instance
	count() int
	count64() int64
	make()
	instances() <-chan *instance
	dump() string
}

func makeInstances() instances {
	return &instanceManager{catalog: map[string]*instance{}}
}

func makeInstancesWithCatalog(catalog map[string]*instance) instances {
	return &instanceManager{catalog: catalog}
}

func (is *instanceManager) dump() string {
	is.RLock()
	defer is.RUnlock()
	return spew.Sdump(is.catalog)
}

func (is *instanceManager) make() {
	is.Lock()
	is.catalog = make(map[string]*instance)
	is.Unlock()
}

func (is *instanceManager) add(inst *instance) {
	if inst == nil {
		return
	}
	debug.Println(inst)
	is.Lock()
	defer is.Unlock()
	is.catalog[*inst.InstanceId] = inst
}

func (is *instanceManager) get(id string) (inst *instance) {
	is.RLock()
	defer is.RUnlock()
	return is.catalog[id]
}

func (is *instanceManager) count() int {
	is.RLock()
	defer is.RUnlock()

	return len(is.catalog)
}

func (is *instanceManager) count64() int64 {
	return int64(is.count())
}

func (is *instanceManager) instances() <-chan *instance {
	retC := make(chan *instance)
	go func() {
		is.RLock()
		defer is.RUnlock()
		defer close(retC)
		for _, i := range is.catalog {
			retC <- i
		}
	}()

	return retC
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
	hasEBSOptimization       bool
}

func (i *instance) isSpot() bool {
	return (i.InstanceLifecycle != nil &&
		*i.InstanceLifecycle == "spot")
}

func (i *instance) terminate() error {

	_, err := i.region.services.ec2.TerminateInstances(
		&ec2.TerminateInstancesInput{
			InstanceIds: []*string{i.InstanceId},
		},
	)
	if err != nil {
		logger.Printf("Issue while terminating %v: %v", *i.InstanceId, err.Error())
		return err
	}
	return nil
}

// We skip it in case we have more than 25% instances of this type already running
func (i *instance) isSpotQuantityCompatible(spotCandidate instanceTypeInformation) bool {
	spotInstanceCount := i.asg.alreadyRunningSpotInstanceTypeCount(
		spotCandidate.instanceType, *i.Placement.AvailabilityZone)

	debug.Println("Checking current spot quantity:")
	debug.Println("\tSpot count: ", spotInstanceCount)
	if spotInstanceCount != 0 {
		debug.Println("\tRatio desired/spot currently running: ",
			(*i.asg.DesiredCapacity/spotInstanceCount > 4))
	}
	return spotInstanceCount == 0 || *i.asg.DesiredCapacity/spotInstanceCount > 4
}

func (i *instance) isPriceCompatible(spotCandidate instanceTypeInformation, bestPrice float64) bool {
	spotPrice := spotCandidate.pricing.spot[*i.Placement.AvailabilityZone]

	if *i.EbsOptimized {
		spotPrice += spotCandidate.pricing.ebsSurcharge
	}

	debug.Println("Comparing price spot/instance:")
	debug.Println("\tSpot price: ", spotPrice)
	debug.Println("\tInstance price: ", i.price)
	return spotPrice != 0 && spotPrice <= i.price && spotPrice <= bestPrice
}

func (i *instance) isClassCompatible(spotCandidate instanceTypeInformation) bool {
	current := i.typeInfo

	debug.Println("Comparing class spot/instance:")
	debug.Println("\tSpot CPU/memory: ", spotCandidate.vCPU, " / ", spotCandidate.memory)
	debug.Println("\tInstance CPU/memory: ", current.vCPU, " / ", current.memory)

	return spotCandidate.vCPU >= current.vCPU && spotCandidate.memory >= current.memory
}

// Here we check the storage compatibility, with the following evaluation
// criteria:
// - speed: don't accept spinning disks when we used to have SSDs
// - number of volumes: the new instance should have enough volumes to be
//   able to attach all the instance store device mappings defined on the
//   original instance
// - volume size: each of the volumes should be at least as big as the
//   original instance's volumes
func (i *instance) isStorageCompatible(spotCandidate instanceTypeInformation, attachedVolumes int) bool {
	existing := i.typeInfo

	debug.Println("Comparing storage spot/instance:")
	debug.Println("\tSpot volumes/size/ssd: ",
		spotCandidate.instanceStoreDeviceCount,
		spotCandidate.instanceStoreDeviceSize,
		spotCandidate.instanceStoreIsSSD)
	debug.Println("\tInstance volumes/size/ssd: ",
		attachedVolumes,
		existing.instanceStoreDeviceSize,
		existing.instanceStoreIsSSD)

	return attachedVolumes == 0 ||
		(spotCandidate.instanceStoreDeviceCount >= attachedVolumes &&
			spotCandidate.instanceStoreDeviceSize >= existing.instanceStoreDeviceSize &&
			(spotCandidate.instanceStoreIsSSD ||
				spotCandidate.instanceStoreIsSSD == existing.instanceStoreIsSSD))
}

func (i *instance) isVirtualizationCompatible(spotVirtualizationTypes []string) bool {
	current := *i.VirtualizationType

	debug.Println("Comparing virtualization spot/instance:")
	debug.Println("\tSpot virtualization: ", spotVirtualizationTypes)
	debug.Println("\tInstance virtualization: ", current)

	for _, avt := range spotVirtualizationTypes {
		if (avt == "PV") && (current == "paravirtual") ||
			(avt == "HVM") && (current == "hvm") {
			return true
		}
	}
	return false
}

func (i *instance) getCheapestCompatibleSpotInstanceType() (string, error) {
	current := i.typeInfo
	bestPrice := math.MaxFloat64
	chosenSpotType := ""
	attachedVolumesNumber := current.instanceStoreDeviceCount

	// Count the ephemeral volumes attached to the original instance's block
	// device mappings, this number is used later when comparing with each
	// instance type.
	lc := i.asg.getLaunchConfiguration()

	if lc != nil {
		lcMappings := lc.countLaunchConfigEphemeralVolumes()
		attachedVolumesNumber = min(lcMappings, current.instanceStoreDeviceCount)
	}

	for _, candidate := range i.region.instanceTypeInformation {

		logger.Println("Comparing ", candidate.instanceType, " with ",
			current.instanceType)

		if i.isSpotQuantityCompatible(candidate) &&
			i.isPriceCompatible(candidate, bestPrice) &&
			i.isClassCompatible(candidate) &&
			i.isStorageCompatible(candidate, attachedVolumesNumber) &&
			i.isVirtualizationCompatible(candidate.virtualizationTypes) {
			bestPrice = candidate.pricing.spot[*i.Placement.AvailabilityZone]
			chosenSpotType = candidate.instanceType
			debug.Println("Best option is now: ", chosenSpotType, " at ", bestPrice)
		} else if chosenSpotType != "" {
			debug.Println("Current best option: ", chosenSpotType, " at ", bestPrice)
		}
	}
	if chosenSpotType != "" {
		debug.Println("Cheapest compatible spot instance found: ", chosenSpotType)
		return chosenSpotType, nil
	}
	return chosenSpotType, fmt.Errorf("No cheaper spot instance types could be found")
}

func (i *instance) tag(tags []*ec2.Tag, maxIter int, sleepFunc func(d time.Duration)) error {
	var (
		n   int
		err error
	)

	if len(tags) == 0 {
		logger.Println(i.region.name, "Tagging spot instance", *i.InstanceId,
			"no tags were defined, skipping...")
		return nil
	}

	svc := i.region.services.ec2
	params := ec2.CreateTagsInput{
		Resources: []*string{i.InstanceId},
		Tags:      tags,
	}

	logger.Println(i.region.name, "Tagging spot instance", *i.InstanceId)

	for n = 0; n < maxIter; n++ {
		_, err = svc.CreateTags(&params)
		if err == nil {
			logger.Println("Instance", *i.InstanceId,
				"was tagged with the following tags:", tags)
			break
		}
		logger.Println(i.region.name,
			"Failed to create tags for the spot instance", *i.InstanceId, err.Error())
		logger.Println(i.region.name,
			"Sleeping for 5 seconds before retrying")
		sleepFunc(5 * time.Second)
	}
	return err
}

// Why the heck isn't this in the Go standard library?
func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}
