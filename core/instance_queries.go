// Copyright (c) 2016-2022 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

// instance_queries.go contains read-only functions that return various information about instances.

package autospotting

import (
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	ec2instancesinfo "github.com/cristim/ec2-instances-info"
)

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
	return i.InstanceLifecycle != nil &&
		*i.InstanceLifecycle == Spot
}

func (i *instance) getSavings() float64 {
	odPrice := i.typeInfo.pricing.onDemand
	spotPrice := i.typeInfo.pricing.spot[*i.Placement.AvailabilityZone]

	log.Printf("Calculating savings for instance %s with OD price %f and Spot price %f\n", *i.InstanceId, odPrice, spotPrice)
	return odPrice - spotPrice
}

func (i *instance) isProtectedFromTermination() (bool, error) {
	debug.Println("\tChecking termination protection for instance: ", *i.InstanceId)

	// determine and set the API termination protection field
	diaRes, err := i.region.services.ec2.DescribeInstanceAttribute(
		&ec2.DescribeInstanceAttributeInput{
			Attribute:  aws.String("disableApiTermination"),
			InstanceId: i.InstanceId,
		})

	if err != nil {
		// better safe than sorry!
		log.Printf("Couldn't describe instance attributes, assuming instance %v is protected: %v\n",
			*i.InstanceId, err.Error())
		return true, err
	}

	if diaRes != nil &&
		diaRes.DisableApiTermination != nil &&
		diaRes.DisableApiTermination.Value != nil &&
		*diaRes.DisableApiTermination.Value {
		log.Printf("\t: %v Instance, %v is protected from termination\n",
			*i.Placement.AvailabilityZone, *i.InstanceId)
		return true, nil
	}
	return false, nil
}

func (i *instance) isProtectedFromScaleIn() bool {
	if i.asg == nil {
		return false
	}

	for _, inst := range i.asg.Instances {
		if *inst.InstanceId == *i.InstanceId &&
			*inst.ProtectedFromScaleIn {
			log.Printf("\t: %v Instance, %v is protected from scale-in\n",
				*inst.AvailabilityZone,
				*inst.InstanceId)
			return true
		}
	}
	return false
}

func (i *instance) canTerminate() bool {
	return *i.State.Name != ec2.InstanceStateNameTerminated &&
		*i.State.Name != ec2.InstanceStateNameShuttingDown
}

func (i *instance) shouldBeReplacedWithSpot() bool {
	protT, _ := i.isProtectedFromTermination()
	return i.belongsToEnabledASG() &&
		i.asgNeedsReplacement() &&
		!i.isSpot() &&
		!i.isProtectedFromScaleIn() &&
		!protT
}

func (i *instance) belongsToEnabledASG() bool {
	belongs, asgName := i.belongsToAnASG()
	if !belongs {
		log.Printf("%s instane %s doesn't belong to any ASG",
			i.region.name, *i.InstanceId)
		return false
	}

	for _, asg := range i.region.enabledASGs {
		if asg.name == *asgName {
			asg.config = i.region.conf.AutoScalingConfig
			asg.scanInstances()
			asg.loadDefaultConfig()
			asg.loadConfigFromTags()
			asg.loadLaunchConfiguration()
			asg.loadLaunchTemplate()
			i.asg = &asg
			i.price = i.typeInfo.pricing.onDemand
			log.Printf("%s instace %s belongs to enabled ASG %s", i.region.name,
				*i.InstanceId, i.asg.name)
			return true
		}
	}
	return false
}

func (i *instance) belongsToAnASG() (bool, *string) {
	for _, tag := range i.Tags {
		if *tag.Key == "aws:autoscaling:groupName" {
			return true, tag.Value
		}
	}
	return false, nil
}

func (i *instance) getReplacementTargetASGName() *string {
	for _, tag := range i.Tags {
		if *tag.Key == "launched-for-asg" {
			return tag.Value
		}
	}
	return nil
}

func (i *instance) asgNeedsReplacement() bool {
	ret, _ := i.asg.needReplaceOnDemandInstances()
	return ret
}

func (i *instance) isPriceCompatible(spotPrice float64) bool {
	if spotPrice == 0 {
		debug.Printf("\tUnavailable in this Availability Zone")
		return false
	}

	if spotPrice <= i.price {
		return true
	}

	debug.Printf("\tNot price compatible")
	return false
}

func (i *instance) isClassCompatible(spotCandidate *instanceTypeInformation) bool {
	current := i.typeInfo

	debug.Println("Comparing class spot/instance:")
	debug.Println("\tSpot CPU/memory/GPU: ", spotCandidate.vCPU,
		" / ", spotCandidate.memory, " / ", spotCandidate.GPU)
	debug.Println("\tInstance CPU/memory/GPU: ", current.vCPU,
		" / ", current.memory, " / ", current.GPU)

	if i.isSameArch(spotCandidate) &&
		spotCandidate.vCPU >= current.vCPU &&
		spotCandidate.memory >= current.memory &&
		spotCandidate.GPU >= current.GPU {
		return true
	}
	debug.Println("\tNot class compatible (CPU/memory/GPU)")
	return false
}

func (i *instance) isSameArch(other *instanceTypeInformation) bool {
	thisCPU := i.typeInfo.PhysicalProcessor
	otherCPU := other.PhysicalProcessor

	ret := (isIntelCompatible(thisCPU) && isIntelCompatible(otherCPU)) ||
		(isARM(thisCPU) && isARM(otherCPU))

	if !ret {
		debug.Println("\tInstance CPU architecture mismatch, current CPU architecture",
			thisCPU, "is incompatible with candidate CPU architecture", otherCPU)
	}
	return ret
}

func isIntelCompatible(cpuName string) bool {
	return isIntel(cpuName) || isAMD(cpuName)
}

func isIntel(cpuName string) bool {
	// t1.micro seems to be the only one to have this set to 'Variable'
	return strings.Contains(cpuName, "Intel") || strings.Contains(cpuName, "Variable")
}

func isAMD(cpuName string) bool {
	return strings.Contains(cpuName, "AMD")
}

func isARM(cpuName string) bool {
	// The ARM chips are so far all called "AWS Graviton Processor"
	return strings.Contains(cpuName, "AWS")
}

func (i *instance) isEBSCompatible(spotCandidate *instanceTypeInformation) bool {
	if spotCandidate.EBSThroughput < i.typeInfo.EBSThroughput {
		debug.Println("\tEBS throughput insufficient:", spotCandidate.EBSThroughput, "<", i.typeInfo.EBSThroughput)
		return false
	}
	return true
}

// Here we check the storage compatibility, with the following evaluation
// criteria:
//   - speed: don't accept spinning disks when we used to have SSDs
//   - number of volumes: the new instance should have enough volumes to be
//     able to attach all the instance store device mappings defined on the
//     original instance
//   - volume size: each of the volumes should be at least as big as the
//     original instance's volumes
func (i *instance) isStorageCompatible(spotCandidate *instanceTypeInformation, attachedVolumes int) bool {
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

	if attachedVolumes == 0 ||
		(spotCandidate.instanceStoreDeviceCount >= attachedVolumes &&
			spotCandidate.instanceStoreDeviceSize >= existing.instanceStoreDeviceSize &&
			(spotCandidate.instanceStoreIsSSD ||
				spotCandidate.instanceStoreIsSSD == existing.instanceStoreIsSSD)) {
		return true
	}
	debug.Println("\tNot storage compatible")
	return false
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
	debug.Println("\tNot virtualization compatible")
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
		debug.Println("\tNot in the list of allowed instance types")
		return false
	} else if len(disallowedList) > 0 {
		for _, a := range disallowedList {
			// glob matching
			if match, _ := filepath.Match(a, instanceType); match {
				debug.Println("\tIn the list of disallowed instance types")
				return false
			}
		}
	}
	return true
}

func (i *instance) getCompatibleSpotInstanceTypesList(PrioritizationBias string, allowedList []string,
	disallowedList []string) ([]*string, error) {
	current := i.typeInfo
	var acceptableInstanceTypes []acceptableInstance

	// Count the ephemeral volumes attached to the original instance's block
	// device mappings, this number is used later when comparing with each
	// instance type.

	lcMappings := i.asg.launchConfiguration.countLaunchConfigEphemeralVolumes()
	ltMappings := i.asg.launchTemplate.countLaunchTemplateEphemeralVolumes()
	usedMappings := max(lcMappings, ltMappings)
	attachedVolumesNumber := min(usedMappings, current.instanceStoreDeviceCount)

	// Iterate alphabetically by instance type
	keys := make([]string, 0)
	for k := range i.region.instanceTypeInformation {
		keys = append(keys, k)
	}

	if len(keys) == 0 {
		log.Println("Missing instance type information for ", i.region.name)
	}

	sort.Strings(keys)

	// Find all compatible and not blocked instance types
	for _, k := range keys {
		candidate := i.region.instanceTypeInformation[k]

		candidatePrice := i.calculatePrice(candidate)
		debug.Println("Comparing current type", current.instanceType, "with price", i.price,
			"with candidate", candidate.instanceType, "with price", candidatePrice)

		if i.isAllowed(candidate.instanceType, allowedList, disallowedList) && i.isCompatible(&candidate, candidatePrice, attachedVolumesNumber) {
			acceptableInstanceTypes = append(acceptableInstanceTypes, acceptableInstance{candidate, candidatePrice, candidate.generationDelta})
			log.Println("\tMATCH FOUND, added", candidate.instanceType, "to launch candidates list for instance", *i.InstanceId)
		} else if candidate.instanceType != "" {
			debug.Println("Non compatible option found:", candidate.instanceType, "at", candidatePrice, " - discarding")
		}
	}

	if acceptableInstanceTypes != nil {
		sort.Slice(acceptableInstanceTypes, func(i, j int) bool {
			if PrioritizationBias == "prefer_newer_generations" {
				log.Printf("Sorting biased towards newer instance types, comparing %v"+
					" of generation delta %v and price %v(adjusted to %v) with %v of generation delta %v and price %v (adjusted to %v)\n",
					acceptableInstanceTypes[i].instanceTI.instanceType,
					acceptableInstanceTypes[i].generationDelta,
					acceptableInstanceTypes[i].price,
					acceptableInstanceTypes[i].price*(1.0+0.1*float64(acceptableInstanceTypes[i].generationDelta)),
					acceptableInstanceTypes[j].instanceTI.instanceType,
					acceptableInstanceTypes[j].generationDelta,
					acceptableInstanceTypes[j].price,
					acceptableInstanceTypes[j].price*(1.0+0.1*float64(acceptableInstanceTypes[j].generationDelta)))
				return acceptableInstanceTypes[i].price*(1.0+0.1*float64(acceptableInstanceTypes[i].generationDelta)) <
					acceptableInstanceTypes[j].price*(1.0+0.1*float64(acceptableInstanceTypes[j].generationDelta))
			}
			return acceptableInstanceTypes[i].price < acceptableInstanceTypes[j].price

		})
		log.Println("List of cheapest compatible spot instances found, sorted ascending by price/bias: ",
			acceptableInstanceTypes)
		var result []*string
		for _, ai := range acceptableInstanceTypes {
			result = append(result, aws.String(ai.instanceTI.instanceType))
		}
		return result, nil
	}

	return nil, fmt.Errorf("no cheaper spot instance types could be found")
}

func (i *instance) isCompatible(candidate *instanceTypeInformation, candidatePrice float64, attachedVolumesNumber int) bool {
	return i.isPriceCompatible(candidatePrice) &&
		i.isEBSCompatible(candidate) &&
		i.isClassCompatible(candidate) &&
		i.isStorageCompatible(candidate, attachedVolumesNumber) &&
		i.isVirtualizationCompatible(candidate.virtualizationTypes)
}

func (i *instance) getReplacementTargetInstanceID() *string {
	for _, tag := range i.Tags {
		if *tag.Key == "launched-for-replacing-instance" {
			return tag.Value
		}
	}
	return nil
}

func (i *instance) isLaunchedByAutoSpotting() bool {
	for _, tag := range i.Tags {
		if *tag.Key == "launched-by-autospotting" {
			return true
		}
	}
	return false
}

func (i *instance) launchTemplateHasNetworkInterfaces(ltData *ec2.ResponseLaunchTemplateData) (bool, []*ec2.LaunchTemplateInstanceNetworkInterfaceSpecification) {
	if ltData == nil {
		log.Println("Missing launch template data for ", *i.InstanceId)
		return false, nil
	}

	nis := ltData.NetworkInterfaces
	if len(nis) > 0 {
		return true, nis
	}
	return false, nil
}

func (i *instance) isUnattachedSpotInstanceLaunchedForAnEnabledASG() bool {
	asgName := i.getReplacementTargetASGName()
	if asgName == nil {
		log.Printf("%s is missing the tag value for 'launched-for-asg'", *i.InstanceId)
		return false
	}
	asg := i.region.findEnabledASGByName(*asgName)

	if asg != nil &&
		!asg.hasMemberInstance(i) &&
		i.isSpot() {
		log.Println("Found unattached spot instance", *i.InstanceId)
		return true
	}
	return false
}

func calculateGenerationDelta(data *ec2instancesinfo.InstanceData,
	instanceType string,
	itfic *instanceTypeFamilyInfoCache,
	itmgc *instanceTypeMaxGenerationCache) int64 {

	family, generation := calculateInstanceTypeGeneration(instanceType, itfic)

	if (*itmgc)[family] != 0 {
		mg := (*itmgc)[family]
		debug.Println("Found in cache for family", family, "latest generation ", mg)
		delta := mg - generation
		debug.Println("Calculated generation delta for instance type", instanceType, "of generation", generation, "to be", delta)
		return delta
	}

	maxGeneration := generation
	for i := range *data {
		f, g := calculateInstanceTypeGeneration((*data)[i].InstanceType, itfic)
		if f == family && g > maxGeneration {
			maxGeneration = g
		}
	}
	log.Println("Caching maxgeneration", maxGeneration, "for family", family,
		"while processing instance type", instanceType, "of generation", generation)
	(*itmgc)[family] = maxGeneration
	delta := maxGeneration - generation
	debug.Println("Calculated generation delta for instance type", instanceType, "of generation", generation, "to be", delta)
	return delta
}

// for c5ad.2xlarge returns the tuple ("c", 5)
// for inf1.6xlarge returns the tuple ("inf", 1)
// for g5g.4xlarge returns the tuple ("g", 5)
func calculateInstanceTypeGeneration(InstanceType string, itfic *instanceTypeFamilyInfoCache) (string, int64) {
	if it := (*itfic)[InstanceType]; it != nil {
		return it.family, it.generation
	}

	re := regexp.MustCompile(`^(\w+)(\d+)(\w+)?\.\w+$`)
	match := re.FindStringSubmatch(InstanceType)
	if len(match) == 0 {
		(*itfic)[InstanceType] = &instanceTypeFamilyInfo{family: InstanceType, generation: 1}
		return InstanceType, 1
	}
	family := match[1]
	generation, _ := strconv.ParseInt(match[2], 10, 64)

	(*itfic)[InstanceType] = &instanceTypeFamilyInfo{family: family, generation: generation}
	return family, generation
}
