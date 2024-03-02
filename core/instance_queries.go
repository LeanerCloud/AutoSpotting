// Copyright (c) 2016-2022 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

// instance_queries.go contains read-only functions that return various information about instances.

package autospotting

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	ec2instancesinfo "github.com/LeanerCloud/ec2-instances-info"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func (i *instance) calculatePrice(spotCandidate *instanceTypeInformation) float64 {

	spotProduct := i.asg.getSpotProduct()

	if spotProduct == nil {
		log.Printf("Couldn't get Spot product for instance: %s", *i.InstanceId)
		return 0
	}

	spotPrice := spotCandidate.pricing[*spotProduct].spot[*i.Placement.AvailabilityZone]
	debug.Println("Comparing price spot/instance:")

	if i.EbsOptimized != nil && *i.EbsOptimized {
		spotPrice += spotCandidate.ebsSurcharge
		debug.Println("\tEBS Surcharge : ", spotCandidate.ebsSurcharge)
	}

	debug.Println("\tSpot price: ", spotPrice)
	debug.Println("\tInstance price: ", i.initialPrice)
	return spotPrice
}

func (i *instance) isSpot() bool {
	return i.InstanceLifecycle == Spot
}

func (i *instance) getSavings() float64 {
	var spotProduct *string
	if belongs, asg := i.belongsToEnabledASG(); belongs {
		spotProduct = asg.getSpotProduct()
	} else {
		spotProduct = i.getSpotProductFromAMI()
	}

	if spotProduct == nil {
		log.Printf("Couldn't get Spot product for instance: %s", *i.InstanceId)
		return 0
	}

	odPrice := i.getODPrice()
	spotPrice := i.typeInfo.pricing[*spotProduct].spot[*i.Placement.AvailabilityZone]

	log.Printf("Calculating savings for instance %s with base OnDemand price %f and Spot price %f\n", *i.InstanceId, odPrice, spotPrice)
	return math.Abs(odPrice - spotPrice)
}

func (i *instance) getODPrice() float64 {
	if price, err := i.getODPriceFromTag(); err == nil {
		log.Printf("Found OnDemand price %f for instance %s in ASG tag\n",
			price, *i.InstanceId)
		return price
	}

	var spotProduct *string
	if belongs, asg := i.belongsToEnabledASG(); belongs {
		spotProduct = asg.getSpotProduct()
	} else {
		spotProduct = i.getSpotProductFromAMI()
	}

	if spotProduct == nil {
		log.Printf("Couldn't get Spot product for instance: %s", *i.InstanceId)
		return 0
	}

	return i.typeInfo.pricing[*spotProduct].onDemand

}

func (i *instance) canTerminate() bool {
	return i.State.Name != ec2Types.InstanceStateNameTerminated &&
		i.State.Name != ec2Types.InstanceStateNameShuttingDown
}

func (i *instance) isOlderThan(duration time.Duration) bool {
	return i.LaunchTime.Before(time.Now().Add(-1 * duration))
}

func (i *instance) shouldBeReplacedWithNewSpotInstance() bool {
	// only replace Spot instances if they are older than 2 minutes
	if i.isSpot() {
		if i.isOlderThan(2 * time.Minute) {
			log.Printf("Instance %s is a Spot instance and older than 2 minutes, will be replaced with a new Spot instance", *i.InstanceId)
			return true
		}
		return false
	}
	lifecycle := i.getLifecycleValue()

	// instance is not in an enabled ASG
	if belongs, _ := i.belongsToEnabledASG(); !belongs {
		log.Printf("%s instance %s does not belong to an enabled ASG, will not be replaced with a new Spot instance",
			lifecycle, *i.InstanceId)
		return false
	}

	if i.asgNeedsReplacement() {
		log.Printf("%s instance %s needs to be replaced by the ASG, will be replaced with a new Spot instance",
			lifecycle, *i.InstanceId)
		return true
	}

	if inst, _ := i.region.findReplacementInstance(*i.InstanceId); inst == nil {
		log.Printf("%s instance %s doesn't already have a replacement instance", lifecycle, *i.InstanceId)
		return true
	}

	log.Printf("%s instance %s does not need to be replaced with a new Spot instance",
		lifecycle, *i.InstanceId)
	return false
}
func (i *instance) shouldBeTerminated(asg *autoScalingGroup) (bool, bool) {
	lifecycle := i.getLifecycleValue()
	log.Printf("Evaluating if %s instance %s should be terminated", lifecycle, *i.InstanceId)

	// Check for recent spot terminations in the ASG
	recentSpotTermination, err := i.asg.hadRecentSpotTermination()
	if err != nil {
		log.Printf("Error checking for recent spot terminations in ASG %s: %s", i.asg.name, err)
	} else {
		log.Printf("Recent spot termination check for ASG %s: %t", i.asg.name, recentSpotTermination)
	}

	// Determine if the instance is a non-spot and non-AutoSpotting instance launched after a recent spot termination
	if !i.isSpot() {
		log.Printf("%s instance %s is not a Spot instance", lifecycle, *i.InstanceId)
		if !i.isLaunchedByAutoSpotting() {
			log.Printf("%s instance %s was not launched by AutoSpotting", lifecycle, *i.InstanceId)
			if recentSpotTermination {
				log.Printf("%s instance %s was launched by the ASG right after a Spot instance was terminated", lifecycle, *i.InstanceId)
				return true, true
			}
		}
	}

	// Check ASG's MixedInstancesPolicy
	if asg.MixedInstancesPolicy == nil {
		log.Printf("%s instance %s belongs to an ASG without MixedInstancesPolicy, will not be considered for termination", lifecycle, *i.InstanceId)
		return false, false
	}

	// Check against the ASG's DesiredCapacity
	count := int32(asg.instances.count())
	if *asg.DesiredCapacity < count {
		log.Printf("%s instance %s belongs to an ASG with MixedInstancesPolicy and DesiredCapacity(%d) < number of instances(%d), so needs to be terminated",
			lifecycle, *i.InstanceId, *asg.DesiredCapacity, count)
		return true, false
	}

	log.Printf("%s instance %s does not meet any criteria for termination", lifecycle, *i.InstanceId)
	return false, false
}

func (i *instance) belongsToEnabledASG() (bool, *autoScalingGroup) {
	belongs, asgName := i.belongsToAnASG()
	lifecycle := i.getLifecycleValue()
	if !belongs {
		log.Printf("%s: %s instance %s doesn't belong to any ASG",
			i.region.name, lifecycle, *i.InstanceId)
		return false, nil
	}

	log.Printf("%s: %s instance %s belongs to ASG %s",
		i.region.name, lifecycle, *i.InstanceId, *asgName)

	for _, asg := range i.region.enabledASGs {
		if asg.name == *asgName {
			asg.config = i.region.conf.AutoScalingConfig
			asg.scanInstances()
			asg.loadDefaultConfig()
			asg.loadConfigFromTags()
			if _, err := asg.loadLaunchConfiguration(); err != nil {
				log.Printf("Error loading launch configuration: %v", err)

			}

			if _, err := asg.loadLaunchTemplate(); err != nil {
				log.Printf("Error loading launch template: %v", err)

			}
			asg.getSpotProduct()
			a := asg
			i.asg = &a

			spotProduct := i.asg.getSpotProduct()

			if spotProduct == nil {
				log.Printf("Couldn't get Spot product for %s instance: %s", lifecycle, *i.InstanceId)
				continue
			}

			i.initialPrice = i.typeInfo.pricing[*spotProduct].onDemand
			log.Printf("%s: %s instace %s belongs to enabled ASG %s",
				i.region.name, lifecycle, *i.InstanceId, i.asg.name)
			return true, &asg
		}
	}
	return false, nil
}

func (i *instance) belongsToAnASG() (bool, *string) {
	for _, tag := range i.Tags {
		if *tag.Key == "aws:autoscaling:groupName" {
			return true, tag.Value
		}
	}
	return false, nil
}

func (i *instance) getODPriceFromTag() (float64, error) {
	if i == nil && i.Tags == nil {
		return 0, errors.New("no tags found")
	}
	for _, tag := range i.Tags {
		if *tag.Key == "odPrice" {
			if price, err := strconv.ParseFloat(*tag.Value, 64); err == nil {
				return price, nil
			}
		}
	}
	return 0, errors.New("failed to find/parse the odPrice tag")
}

func (i *instance) getBeanstalkDeploymentInformation() (*string, *string) {
	var envName, envID *string

	for _, tag := range i.Tags {
		if *tag.Key == "elasticbeanstalk:environment-name" {
			envName = tag.Value
		}
		if *tag.Key == "elasticbeanstalk:environment-id" {
			envID = tag.Value
		}
	}
	return envName, envID
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

	if spotPrice <= i.initialPrice {
		return true
	}

	debug.Printf("\tNot price compatible")
	return false
}

func (i *instance) haveTooManyInstancesOfThisTypeRunning(instanceType string) bool {

	ret, err := i.asg.willCauseInstanceTypeImbalance(instanceType, *i.Placement.AvailabilityZone)
	if err != nil {
		log.Printf("Encountered error while calling i.asg.haveTooManyInstancesOfInstanceType: %v", err)
		return false
	}

	if ret {
		log.Printf("Having too many instances of instance type %s in AZ %v, skipping this instance type.",
			i.InstanceType, *i.Placement.AvailabilityZone)
	}
	return ret
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

	if !i.asg.config.ConsiderEBSBandwidth {
		debug.Printf("Ignoring EBS Bandwidth when comparing instance type %v with %v \n ", i.InstanceType, spotCandidate.instanceType)
		return true
	}

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
	current := i.VirtualizationType
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

func (i *instance) isAllowed(instanceType string, allowed string, disallowed string) bool {
	debug.Println("Checking allowed/disallowed list")

	if allowed == "current" {
		return instanceType == string(i.InstanceType)
	}

	allowed = strings.Replace(allowed, " ", ",", -1)
	disallowed = strings.Replace(disallowed, " ", ",", -1)

	allowedList := strings.FieldsFunc(allowed, func(c rune) bool {
		return c == ','
	})

	disallowedList := strings.FieldsFunc(disallowed, func(c rune) bool {
		return c == ','
	})

	if len(allowedList) > 0 {
		for _, a := range allowedList {
			if match, _ := filepath.Match(a, instanceType); match {
				debug.Printf("\t Instance type %v matches the list of allowed instance types %v\n",
					instanceType, allowed)
				return true
			}
		}
		debug.Printf("\t Instance type %v doesn't match the list of allowed instance types, skipping %v\n",
			instanceType, allowed)
		return false
	} else if len(disallowedList) > 0 {
		for _, a := range disallowedList {
			// glob matching
			if match, _ := filepath.Match(a, instanceType); match {
				debug.Printf("\t Instance type %v is in the list of disallowed instance types %v, skipping it\n",
					instanceType, disallowed)
				return false
			}
		}
	}
	debug.Printf("\t Instance type %v doesn't match any allowed (%v) or disallowed (%v) instance types, allowing\n",
		instanceType, allowed, disallowed)
	return true
}

func (i *instance) getCompatibleSpotInstanceTypesList(PrioritizationBias string, allowedList string,
	disallowedList string) ([]string, error) {
	current := i.typeInfo
	var acceptableInstanceTypes []acceptableInstance

	// Count the ephemeral volumes attached to the original instance's block
	// device mappings, this number is used later when comparing with each
	// instance type.

	log.Printf("Generating list of compatible instance types for %s with initial price %f",
		i.InstanceType, i.initialPrice)

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
		debug.Println("Comparing current type", current.instanceType, "with price", i.initialPrice,
			"with candidate", candidate.instanceType, "with price", candidatePrice)

		if i.isAllowed(candidate.instanceType, allowedList, disallowedList) && i.isCompatible(candidate, candidatePrice, attachedVolumesNumber) {
			acceptableInstanceTypes = append(acceptableInstanceTypes, acceptableInstance{candidate, candidatePrice, candidate.generationDelta})
			log.Printf("\tMATCH FOUND, added %s with price %.4f to launch candidates list for instance %s\n",
				candidate.instanceType, candidatePrice, *i.InstanceId)
		} else if candidate.instanceType != "" {
			debug.Printf("Non compatible option found: %s at %v - discarding\n",
				candidate.instanceType, candidatePrice)
		}
	}

	if acceptableInstanceTypes != nil {
		sort.Slice(acceptableInstanceTypes, func(i, j int) bool {
			if PrioritizationBias == "prefer_newer_generations" {
				debug.Printf("Sorting biased towards newer instance types, comparing %v"+
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
		debug.Printf("List of cheapest compatible spot instances found, sorted ascending by price/bias: %#v",
			acceptableInstanceTypes)
		var result []string
		for _, ai := range acceptableInstanceTypes {
			it := ai.instanceTI.instanceType
			if len(acceptableInstanceTypes) > 5 && i.haveTooManyInstancesOfThisTypeRunning(it) {
				log.Println("Having enough diversification potential, skipping instance type: ", it)
				continue
			}
			result = append(result, it)
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

func (i *instance) isManagedByECSCapacityProvider() bool {
	for _, tag := range i.Tags {
		if *tag.Key == "AmazonECSManaged" {
			return true
		}
	}
	return false
}

func (i *instance) isUnattachedSpotInstanceLaunchedForAnEnabledASG() bool {
	belongs, asgName := i.belongsToAnASG()
	if !belongs || asgName == nil {
		log.Printf("%s doesn't belong to an ASG", *i.InstanceId)
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
	debug.Println("Caching maxgeneration", maxGeneration, "for family", family,
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

// Retrieves the Spot product associated with the instance's AMI ID.
func (i *instance) getSpotProductFromAMI() *string {

	if i.spotProduct != nil {
		return i.spotProduct
	}

	// Retrieve the image details for the AMI ID of the instance
	ec2Svc := i.region.services.ec2
	resp, err := ec2Svc.DescribeImages(
		context.TODO(),
		&ec2.DescribeImagesInput{
			ImageIds: []string{*i.ImageId},
		})
	if err != nil {
		log.Printf("Error describing AMI '%s': %v", *i.ImageId, err)
		return nil
	}

	if len(resp.Images) == 0 {
		log.Printf("No AMIs found with ID '%s'", *i.ImageId)
		return nil
	}

	image := resp.Images[0]
	if image.PlatformDetails != nil {
		log.Printf("Detected '%s' product for AMI '%s'",
			*image.PlatformDetails,
			*i.ImageId)
		return image.PlatformDetails
	}

	return nil
}

// findReplacementInstance searches for the first instance in the region
// that has the "launched-for-replacing-instance" tag set to the specified instance ID.
func (r *region) findReplacementInstance(instanceID string) (*instance, error) {
	// Ensure the instances are scanned before searching
	err := r.scanInstances()
	if err != nil {
		return nil, fmt.Errorf("failed to scan instances in %s: %w", r.name, err)
	}

	for inst := range r.instances.instances() {
		for _, tag := range inst.Tags {
			if *tag.Key == "launched-for-replacing-instance" && *tag.Value == instanceID {
				return inst, nil
			}
		}
	}

	return nil, nil // No matching instance found
}
