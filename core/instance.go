package autospotting

import (
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
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
	typeInfo  instanceTypeInformation
	price     float64
	region    *region
	protected bool
	asg       *autoScalingGroup
}

type instanceTypeInformation struct {
	instanceType             string
	vCPU                     int
	GPU                      int
	pricing                  prices
	memory                   float32
	virtualizationTypes      []string
	hasInstanceStore         bool
	instanceStoreDeviceSize  float32
	instanceStoreDeviceCount int
	instanceStoreIsSSD       bool
	hasEBSOptimization       bool
}

func (i *instance) calculatePrice(spotCandidate instanceTypeInformation) float64 {
	spotPrice := spotCandidate.pricing.spot[*i.Placement.AvailabilityZone]
	debug.Println("Comparing price spot/instance:")

	if i.EbsOptimized != nil && *i.EbsOptimized {
		spotPrice += spotCandidate.pricing.ebsSurcharge
		debug.Println("\tEBS Surcharge : ", spotCandidate.pricing.ebsSurcharge)
	}

	debug.Println("\tSpot price: ", spotPrice)
	debug.Println("\tInstance price: ", i.price)
	return spotPrice
}

func (i *instance) isSpot() bool {
	return (i.InstanceLifecycle != nil &&
		*i.InstanceLifecycle == "spot")
}

func (i *instance) isProtected() bool {
	return i.protected
}

func (i *instance) canTerminate() bool {
	return *i.State.Name != ec2.InstanceStateNameTerminated &&
		*i.State.Name != ec2.InstanceStateNameShuttingDown
}

func (i *instance) terminate() error {
	svc := i.region.services.ec2
	if i.canTerminate() {
		_, err := svc.TerminateInstances(&ec2.TerminateInstancesInput{
			InstanceIds: []*string{i.InstanceId},
		})
		if err != nil {
			logger.Printf("Issue while terminating %v: %v", *i.InstanceId, err.Error())
			return err
		}
	}
	return nil
}

func (i *instance) isPriceCompatible(spotPrice float64, bestPrice float64) bool {
	return spotPrice != 0 && spotPrice <= i.price && spotPrice <= bestPrice
}

func (i *instance) isClassCompatible(spotCandidate instanceTypeInformation) bool {
	current := i.typeInfo

	debug.Println("Comparing class spot/instance:")
	debug.Println("\tSpot CPU/memory/GPU: ", spotCandidate.vCPU,
		" / ", spotCandidate.memory, " / ", spotCandidate.GPU)
	debug.Println("\tInstance CPU/memory/GPU: ", current.vCPU,
		" / ", current.memory, " / ", current.GPU)

	return spotCandidate.vCPU >= current.vCPU &&
		spotCandidate.memory >= current.memory &&
		spotCandidate.GPU >= current.GPU
}

func (i *instance) isEBSCompatible(spotCandidate instanceTypeInformation) bool {
	if i.EbsOptimized != nil && *i.EbsOptimized && !spotCandidate.hasEBSOptimization {
		return false
	}
	return true
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
	if len(spotVirtualizationTypes) == 0 {
		spotVirtualizationTypes = []string{"HVM"}
	}
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

func (i *instance) isAllowed(instanceType string, allowedList []string, disallowedList []string) bool {
	debug.Println("Checking allowed/disallowed list")

	if len(allowedList) > 0 {
		for _, a := range allowedList {
			if match, _ := filepath.Match(a, instanceType); match {
				return true
			}
		}
		debug.Println("Instance has been excluded since it was not in the allowed instance types list")
		return false
	} else if len(disallowedList) > 0 {
		for _, a := range disallowedList {
			// glob matching
			if match, _ := filepath.Match(a, instanceType); match {
				debug.Println("Instance has been excluded since it was in the disallowed instance types list")
				return false
			}
		}
	}
	return true
}

func (i *instance) getCheapestCompatibleSpotInstanceType(allowedList []string, disallowedList []string) (instanceTypeInformation, error) {
	current := i.typeInfo
	bestPrice := math.MaxFloat64
	chosenSpotType := ""
	var cheapest instanceTypeInformation

	// Count the ephemeral volumes attached to the original instance's block
	// device mappings, this number is used later when comparing with each
	// instance type.

	usedMappings := i.asg.launchConfiguration.countLaunchConfigEphemeralVolumes()
	attachedVolumesNumber := min(usedMappings, current.instanceStoreDeviceCount)

	for _, candidate := range i.region.instanceTypeInformation {

		logger.Println("Comparing ", candidate.instanceType, " with ",
			current.instanceType)

		candidatePrice := i.calculatePrice(candidate)

		if i.isPriceCompatible(candidatePrice, bestPrice) &&
			i.isEBSCompatible(candidate) &&
			i.isClassCompatible(candidate) &&
			i.isStorageCompatible(candidate, attachedVolumesNumber) &&
			i.isVirtualizationCompatible(candidate.virtualizationTypes) &&
			i.isAllowed(candidate.instanceType, allowedList, disallowedList) {
			bestPrice = candidatePrice
			chosenSpotType = candidate.instanceType
			cheapest = candidate
			debug.Println("Best option is now: ", chosenSpotType, " at ", bestPrice)
		} else if chosenSpotType != "" {
			debug.Println("Current best option: ", chosenSpotType, " at ", bestPrice)
		}
	}
	if chosenSpotType != "" {
		debug.Println("Cheapest compatible spot instance found: ", chosenSpotType)
		return cheapest, nil
	}
	return cheapest, fmt.Errorf("No cheaper spot instance types could be found")
}

func (i *instance) launchSpotReplacement() error {
	instanceType, err := i.getCheapestCompatibleSpotInstanceType(
		i.asg.getAllowedInstanceTypes(i),
		i.asg.getDisallowedInstanceTypes(i))

	if err != nil {
		logger.Println("Couldn't determine the cheapest compatible spot instance type")
		return err
	}

	bidPrice := i.getPricetoBid(instanceType.pricing.onDemand,
		instanceType.pricing.spot[*i.Placement.AvailabilityZone])

	runInstancesInput := i.createRunInstancesInput(instanceType.instanceType, bidPrice)
	resp, err := i.region.services.ec2.RunInstances(runInstancesInput)

	if err != nil {
		logger.Println("Couldn't launch spot instance:", err.Error())
		debug.Println(runInstancesInput)
		return err
	}

	spotInst := resp.Instances[0]
	logger.Println(i.asg.name, "Created spot instance", *spotInst.InstanceId)

	debug.Println("RunInstances response:", spew.Sdump(resp))

	return nil
}

func (i *instance) getPricetoBid(
	baseOnDemandPrice float64, currentSpotPrice float64) float64 {

	logger.Println("BiddingPolicy: ", i.region.conf.BiddingPolicy)

	if i.region.conf.BiddingPolicy == DefaultBiddingPolicy {
		logger.Println("Launching spot instance with a bid =", baseOnDemandPrice)
		return baseOnDemandPrice
	}

	bufferPrice := math.Min(baseOnDemandPrice, currentSpotPrice*(1.0+i.region.conf.SpotPriceBufferPercentage/100.0))
	logger.Println("Launching spot instance with a bid =", bufferPrice)
	return bufferPrice
}

func (i *instance) convertBlockDeviceMappings(lc *launchConfiguration) []*ec2.BlockDeviceMapping {
	bds := []*ec2.BlockDeviceMapping{}
	if lc == nil || len(lc.BlockDeviceMappings) == 0 {
		debug.Println("Missing block device mappings")
		return bds
	}

	for _, lcBDM := range lc.BlockDeviceMappings {

		ec2BDM := &ec2.BlockDeviceMapping{
			DeviceName:  lcBDM.DeviceName,
			VirtualName: lcBDM.VirtualName,
		}

		if lcBDM.Ebs != nil {
			ec2BDM.Ebs = &ec2.EbsBlockDevice{
				DeleteOnTermination: lcBDM.Ebs.DeleteOnTermination,
				Encrypted:           lcBDM.Ebs.Encrypted,
				Iops:                lcBDM.Ebs.Iops,
				SnapshotId:          lcBDM.Ebs.SnapshotId,
				VolumeSize:          lcBDM.Ebs.VolumeSize,
				VolumeType:          lcBDM.Ebs.VolumeType,
			}
		}

		// it turns out that the noDevice field needs to be converted from bool to
		// *string
		if lcBDM.NoDevice != nil {
			ec2BDM.NoDevice = aws.String(fmt.Sprintf("%t", *lcBDM.NoDevice))
		}

		bds = append(bds, ec2BDM)
	}
	return bds
}

func (i *instance) convertSecurityGroups() []*string {
	groupIDs := []*string{}
	for _, sg := range i.SecurityGroups {
		groupIDs = append(groupIDs, sg.GroupId)
	}
	return groupIDs
}

func (i *instance) createRunInstancesInput(instanceType string, price float64) *ec2.RunInstancesInput {
	var retval ec2.RunInstancesInput

	retval = ec2.RunInstancesInput{

		EbsOptimized: i.EbsOptimized,

		ImageId: i.ImageId,

		InstanceMarketOptions: &ec2.InstanceMarketOptionsRequest{
			MarketType: aws.String("spot"),
			SpotOptions: &ec2.SpotMarketOptions{
				MaxPrice: aws.String(strconv.FormatFloat(price, 'g', 10, 64)),
			},
		},

		InstanceType: aws.String(instanceType),
		KeyName:      i.KeyName,
		MaxCount:     aws.Int64(1),
		MinCount:     aws.Int64(1),

		Placement: i.Placement,

		SecurityGroupIds: i.convertSecurityGroups(),

		SubnetId:          i.SubnetId,
		TagSpecifications: i.generateTagsList(),
	}

	if i.IamInstanceProfile != nil {
		retval.IamInstanceProfile = &ec2.IamInstanceProfileSpecification{
			Arn: i.IamInstanceProfile.Arn,
		}
	}

	if i.asg.LaunchTemplate != nil {
		retval.LaunchTemplate = &ec2.LaunchTemplateSpecification{
			LaunchTemplateId:   i.asg.LaunchTemplate.LaunchTemplateId,
			LaunchTemplateName: i.asg.LaunchTemplate.LaunchTemplateName,
		}
	}

	if i.asg.launchConfiguration != nil {
		lc := i.asg.launchConfiguration

		retval.UserData = lc.UserData

		BDMs := i.convertBlockDeviceMappings(lc)

		if len(BDMs) > 0 {
			retval.BlockDeviceMappings = BDMs
		}

		if lc.InstanceMonitoring != nil {
			retval.Monitoring = &ec2.RunInstancesMonitoringEnabled{
				Enabled: lc.InstanceMonitoring.Enabled}
		}

		sgIDs := i.convertSecurityGroups()

		if lc.AssociatePublicIpAddress != nil || i.SubnetId != nil {
			// Instances are running in a VPC.
			retval.NetworkInterfaces = []*ec2.InstanceNetworkInterfaceSpecification{
				{
					AssociatePublicIpAddress: lc.AssociatePublicIpAddress,
					DeviceIndex:              aws.Int64(0),
					SubnetId:                 i.SubnetId,
					Groups:                   sgIDs,
				},
			}
			retval.SubnetId, retval.SecurityGroupIds = nil, nil
		}
	}

	return &retval
}

func (i *instance) generateTagsList() []*ec2.TagSpecification {
	tags := ec2.TagSpecification{
		ResourceType: aws.String("instance"),
		Tags: []*ec2.Tag{
			{
				Key:   aws.String("LaunchConfigurationName"),
				Value: i.asg.LaunchConfigurationName,
			},
			{
				Key:   aws.String("launched-by-autospotting"),
				Value: aws.String("true"),
			},
			{
				Key:   aws.String("launched-for-asg"),
				Value: aws.String(i.asg.name),
			},
		},
	}

	for _, tag := range i.Tags {
		if !strings.HasPrefix(*tag.Key, "aws:") {
			tags.Tags = append(tags.Tags, tag)
		}
	}
	return []*ec2.TagSpecification{&tags}
}

// returns an instance ID as *string, set to nil if we need to wait for the next
// run in case there are no spot instances
func (i *instance) isReadyToAttach(asg *autoScalingGroup) bool {

	logger.Println("Considering ", *i.InstanceId, "for attaching to", asg.name)

	gracePeriod := *asg.HealthCheckGracePeriod

	instanceUpTime := time.Now().Unix() - i.LaunchTime.Unix()

	logger.Println("Instance uptime:", time.Duration(instanceUpTime)*time.Second)

	// Check if the spot instance is out of the grace period, so in that case we
	// can replace an on-demand instance with it
	if *i.State.Name == ec2.InstanceStateNameRunning &&
		instanceUpTime > gracePeriod {
		logger.Println("The spot instance", *i.InstanceId,
			"is still in the grace period,",
			"waiting for it to be ready before we can attach it to the group...")
		return true
	} else if *i.State.Name == ec2.InstanceStateNamePending {
		logger.Println("The spot instance", *i.InstanceId,
			"is still pending,",
			"waiting for it to be running before we can attach it to the group...")
		return false
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
