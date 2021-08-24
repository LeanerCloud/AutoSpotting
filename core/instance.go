// Copyright (c) 2016-2021 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/davecgh/go-spew/spew"
)

var unsupportedIO2Regions = [...]string{
	"us-gov-west-1",
	"us-gov-east-1",
	"sa-east-1",
	"cn-north-1",
	"cn-northwest-1",
	"eu-south-1",
	"af-south-1",
	"eu-west-3",
	"ap-northeast-3",
}

// instance wraps an ec2.instance and has some additional fields and functions
type instance struct {
	*ec2.Instance
	typeInfo  instanceTypeInformation
	price     float64
	region    *region
	protected bool
	asg       *autoScalingGroup
}

func (i *instance) terminate() error {
	var err error
	log.Printf("Instance: %v\n", i)

	log.Printf("Terminating %v", *i.InstanceId)
	svc := i.region.services.ec2

	if !i.canTerminate() {
		log.Printf("Can't terminate %v, current state: %s",
			*i.InstanceId, *i.State.Name)
		return fmt.Errorf("can't terminate %s", *i.InstanceId)
	}

	_, err = svc.TerminateInstances(&ec2.TerminateInstancesInput{
		InstanceIds: []*string{i.InstanceId},
	})

	if err != nil {
		log.Printf("Issue while terminating %v: %v", *i.InstanceId, err.Error())
	}

	return err
}

func (i *instance) launchSpotReplacement() (*string, error) {
	i.price = i.typeInfo.pricing.onDemand / i.region.conf.OnDemandPriceMultiplier * i.asg.config.OnDemandPriceMultiplier
	instanceTypes, err := i.getCompatibleSpotInstanceTypesListSortedAscendingByPrice(
		i.asg.getAllowedInstanceTypes(i),
		i.asg.getDisallowedInstanceTypes(i))

	if err != nil {
		log.Println("Couldn't determine the cheapest compatible spot instance type")
		return nil, err
	}

	//Go through all compatible instances until one type launches or we are out of options.
	for _, instanceType := range instanceTypes {
		az := *i.Placement.AvailabilityZone
		bidPrice := i.getPriceToBid(i.price,
			instanceType.pricing.spot[az], instanceType.pricing.premium)

		runInstancesInput, err := i.createRunInstancesInput(instanceType.instanceType, bidPrice)
		if err != nil {
			log.Println(az, i.asg.name, "Failed to generate run instances input, ", err.Error(), "skipping instance type ", instanceType.instanceType)
			continue
		}

		log.Println(az, i.asg.name, "Launching spot instance of type", instanceType.instanceType, "with bid price", bidPrice)
		log.Println(az, i.asg.name)
		resp, err := i.region.services.ec2.RunInstances(runInstancesInput)

		if err != nil {
			if strings.Contains(err.Error(), "InsufficientInstanceCapacity") {
				log.Println("Couldn't launch spot instance due to lack of capacity, trying next instance type:", err.Error())
			} else {
				log.Println("Couldn't launch spot instance:", err.Error(), "trying next instance type")
				debug.Println(runInstancesInput)
			}
		} else {
			spotInst := resp.Instances[0]
			log.Println(i.asg.name, "Successfully launched spot instance", *spotInst.InstanceId,
				"of type", *spotInst.InstanceType,
				"with bid price", bidPrice,
				"current spot price", instanceType.pricing.spot[az])

			debug.Println("RunInstances response:", spew.Sdump(resp))
			// add to FinalRecap
			recapText := fmt.Sprintf("%s Launched spot instance %s", i.asg.name, *spotInst.InstanceId)
			i.region.conf.FinalRecap[i.region.name] = append(i.region.conf.FinalRecap[i.region.name], recapText)
			return spotInst.InstanceId, nil
		}
	}

	log.Println(i.asg.name, "Exhausted all compatible instance types without launch success. Aborting.")
	return nil, errors.New("exhausted all compatible instance types")

}

func (i *instance) convertLaunchConfigurationBlockDeviceMappings(BDMs []*autoscaling.BlockDeviceMapping) []*ec2.BlockDeviceMapping {

	bds := []*ec2.BlockDeviceMapping{}
	if len(BDMs) == 0 {
		debug.Println("Missing LC block device mappings")
	}

	for _, BDM := range BDMs {

		ec2BDM := &ec2.BlockDeviceMapping{
			DeviceName:  BDM.DeviceName,
			VirtualName: BDM.VirtualName,
		}

		if BDM.Ebs != nil {
			ec2BDM.Ebs = &ec2.EbsBlockDevice{
				DeleteOnTermination: BDM.Ebs.DeleteOnTermination,
				Encrypted:           BDM.Ebs.Encrypted,
				Iops:                BDM.Ebs.Iops,
				SnapshotId:          BDM.Ebs.SnapshotId,
				VolumeSize:          BDM.Ebs.VolumeSize,
				VolumeType:          convertLaunchConfigurationEBSVolumeType(BDM.Ebs, i.asg),
			}
		}

		// handle the noDevice field directly by skipping the device if set to true
		if BDM.NoDevice != nil && *BDM.NoDevice {
			continue
		}
		bds = append(bds, ec2BDM)
	}

	if len(bds) == 0 {
		return nil
	}
	return bds
}

func (i *instance) convertLaunchTemplateBlockDeviceMappings(BDMs []*ec2.LaunchTemplateBlockDeviceMapping) []*ec2.BlockDeviceMapping {

	bds := []*ec2.BlockDeviceMapping{}
	if len(BDMs) == 0 {
		log.Println("Missing LT block device mappings")
	}

	for _, BDM := range BDMs {

		ec2BDM := &ec2.BlockDeviceMapping{
			DeviceName:  BDM.DeviceName,
			VirtualName: BDM.VirtualName,
		}

		if BDM.Ebs != nil {
			ec2BDM.Ebs = &ec2.EbsBlockDevice{
				DeleteOnTermination: BDM.Ebs.DeleteOnTermination,
				Encrypted:           BDM.Ebs.Encrypted,
				Iops:                BDM.Ebs.Iops,
				SnapshotId:          BDM.Ebs.SnapshotId,
				VolumeSize:          BDM.Ebs.VolumeSize,
				VolumeType:          convertLaunchTemplateEBSVolumeType(BDM.Ebs, i.asg),
			}
		}

		// handle the noDevice field directly by skipping the device if set to true, apparently NoDevice is here a string instead of a bool.
		if BDM.NoDevice != nil && *BDM.NoDevice == "true" {
			continue
		}
		bds = append(bds, ec2BDM)
	}

	if len(bds) == 0 {
		return nil
	}
	return bds
}

func (i *instance) convertImageBlockDeviceMappings(BDMs []*ec2.BlockDeviceMapping) []*ec2.BlockDeviceMapping {

	bds := []*ec2.BlockDeviceMapping{}
	if len(BDMs) == 0 {
		log.Println("Missing Image block device mappings")
	}

	for _, BDM := range BDMs {

		ec2BDM := &ec2.BlockDeviceMapping{
			DeviceName:  BDM.DeviceName,
			VirtualName: BDM.VirtualName,
		}

		if BDM.Ebs != nil {
			ec2BDM.Ebs = &ec2.EbsBlockDevice{
				DeleteOnTermination: BDM.Ebs.DeleteOnTermination,
				Encrypted:           BDM.Ebs.Encrypted,
				Iops:                BDM.Ebs.Iops,
				SnapshotId:          BDM.Ebs.SnapshotId,
				VolumeSize:          BDM.Ebs.VolumeSize,
				VolumeType:          convertImageEBSVolumeType(BDM.Ebs, i.asg),
			}
		}

		// handle the noDevice field directly by skipping the device if set to true, apparently NoDevice is here a string instead of a bool.
		if BDM.NoDevice != nil && *BDM.NoDevice == "true" {
			continue
		}
		bds = append(bds, ec2BDM)
	}

	if len(bds) == 0 {
		return nil
	}
	return bds
}

func convertLaunchConfigurationEBSVolumeType(ebs *autoscaling.Ebs, a *autoScalingGroup) *string {
	// convert IO1 to IO2 in supported regions
	r := a.region.name
	asg := a.name

	if ebs.VolumeType == nil {
		log.Println(r, ": Empty EBS VolumeType while converting LC volume for ASG", asg)
		return nil
	}

	if *ebs.VolumeType == "io1" && supportedIO2region(r) {
		log.Println(r, ": Converting IO1 volume to IO2 for new instance launched for", asg)
		return aws.String("io2")
	}

	// convert GP2 to GP3 below the threshold where GP2 becomes more performant. The Threshold is configurable
	if *ebs.VolumeType == "gp2" && *ebs.VolumeSize <= a.config.GP2ConversionThreshold {
		log.Println(r, ": Converting GP2 EBS volume to GP3 for new instance launched for", asg)
		return aws.String("gp3")
	}
	log.Println(r, ": No EBS volume conversion could be done for", asg)
	return ebs.VolumeType
}

func convertLaunchTemplateEBSVolumeType(ebs *ec2.LaunchTemplateEbsBlockDevice, a *autoScalingGroup) *string {
	// convert IO1 to IO2 in supported regions
	r := a.region.name
	asg := a.name
	if *ebs.VolumeType == "io1" && supportedIO2region(r) {
		log.Println(r, ": Converting IO1 volume to IO2 for new instance launched for", asg)
		return aws.String("io2")
	}

	// convert GP2 to GP3 below the threshold where GP2 becomes more performant. The Threshold is configurable
	if *ebs.VolumeType == "gp2" && *ebs.VolumeSize <= a.config.GP2ConversionThreshold {
		log.Println(r, ": Converting GP2 EBS volume to GP3 for new instance launched for", asg)
		return aws.String("gp3")
	}
	log.Println(r, ": No EBS volume conversion could be done for", asg)
	return ebs.VolumeType
}

func convertImageEBSVolumeType(ebs *ec2.EbsBlockDevice, a *autoScalingGroup) *string {
	// convert IO1 to IO2 in supported regions
	r := a.region.name
	asg := a.name
	if *ebs.VolumeType == "io1" && supportedIO2region(r) {
		log.Println(r, ": Converting IO1 volume to IO2 for new instance launched for", asg)
		return aws.String("io2")
	}

	// convert GP2 to GP3 below the threshold where GP2 becomes more performant. The Threshold is configurable
	if *ebs.VolumeType == "gp2" && *ebs.VolumeSize <= a.config.GP2ConversionThreshold {
		log.Println(r, ": Converting GP2 EBS volume to GP3 for new instance launched for", asg)
		return aws.String("gp3")
	}
	log.Println(r, ": No EBS volume conversion could be done for", asg)
	return ebs.VolumeType
}

func supportedIO2region(region string) bool {
	for _, r := range unsupportedIO2Regions {
		if region == r {
			log.Println("IO2 EBS volumes are not available in", region)
			return false
		}
	}
	return true
}

func (i *instance) convertSecurityGroups() []*string {
	groupIDs := []*string{}
	for _, sg := range i.SecurityGroups {
		groupIDs = append(groupIDs, sg.GroupId)
	}
	return groupIDs
}

func (i *instance) getlaunchTemplate(id, ver *string) (*ec2.ResponseLaunchTemplateData, error) {
	res, err := i.region.services.ec2.DescribeLaunchTemplateVersions(
		&ec2.DescribeLaunchTemplateVersionsInput{
			Versions:         []*string{ver},
			LaunchTemplateId: id,
		},
	)

	if err != nil {
		log.Println("Failed to describe launch template", *id, "version", *ver,
			"encountered error:", err.Error())
		return nil, err
	}
	if len(res.LaunchTemplateVersions) == 1 {
		return res.LaunchTemplateVersions[0].LaunchTemplateData, nil
	}
	return nil, fmt.Errorf("missing launch template version information")
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

func (i *instance) processLaunchTemplate(retval *ec2.RunInstancesInput) error {
	ver := i.asg.LaunchTemplate.Version
	id := i.asg.LaunchTemplate.LaunchTemplateId

	retval.LaunchTemplate = &ec2.LaunchTemplateSpecification{
		LaunchTemplateId: id,
		Version:          ver,
	}

	ltData, err := i.getlaunchTemplate(id, ver)
	if err != nil {
		return err
	}

	retval.BlockDeviceMappings = i.convertLaunchTemplateBlockDeviceMappings(ltData.BlockDeviceMappings)

	if having, nis := i.launchTemplateHasNetworkInterfaces(ltData); having {
		for _, ni := range nis {
			retval.NetworkInterfaces = append(retval.NetworkInterfaces,
				&ec2.InstanceNetworkInterfaceSpecification{
					AssociatePublicIpAddress: ni.AssociatePublicIpAddress,
					SubnetId:                 i.SubnetId,
					DeviceIndex:              ni.DeviceIndex,
					Groups:                   i.convertSecurityGroups(),
				},
			)
		}
		retval.SubnetId, retval.SecurityGroupIds = nil, nil
	}
	return nil
}

func (i *instance) processLaunchConfiguration(retval *ec2.RunInstancesInput) {
	lc := i.asg.launchConfiguration

	if lc.KeyName != nil && *lc.KeyName != "" {
		retval.KeyName = lc.KeyName
	}

	if lc.IamInstanceProfile != nil {
		if strings.HasPrefix(*lc.IamInstanceProfile, "arn:aws:iam:") {
			retval.IamInstanceProfile = &ec2.IamInstanceProfileSpecification{
				Arn: lc.IamInstanceProfile,
			}
		} else {
			retval.IamInstanceProfile = &ec2.IamInstanceProfileSpecification{
				Name: lc.IamInstanceProfile,
			}
		}
	}
	retval.ImageId = lc.ImageId

	if strings.ToLower(i.asg.config.PatchBeanstalkUserdata) == "true" {
		retval.UserData = getPatchedUserDataForBeanstalk(lc.UserData)
	} else {
		retval.UserData = lc.UserData
	}

	BDMs := i.convertLaunchConfigurationBlockDeviceMappings(lc.BlockDeviceMappings)

	if len(BDMs) > 0 {
		retval.BlockDeviceMappings = BDMs
	}

	if lc.InstanceMonitoring != nil {
		retval.Monitoring = &ec2.RunInstancesMonitoringEnabled{
			Enabled: lc.InstanceMonitoring.Enabled}
	}

	if lc.AssociatePublicIpAddress != nil || i.SubnetId != nil {
		// Instances are running in a VPC.
		retval.NetworkInterfaces = []*ec2.InstanceNetworkInterfaceSpecification{
			{
				AssociatePublicIpAddress: lc.AssociatePublicIpAddress,
				DeviceIndex:              aws.Int64(0),
				SubnetId:                 i.SubnetId,
				Groups:                   i.convertSecurityGroups(),
			},
		}
		retval.SubnetId, retval.SecurityGroupIds = nil, nil
	}
}

func (i *instance) processImageBlockDevices(rii *ec2.RunInstancesInput) {
	svc := i.region.services.ec2

	resp, err := svc.DescribeImages(
		&ec2.DescribeImagesInput{
			ImageIds: []*string{i.ImageId},
		})

	if err != nil {
		log.Println(err.Error())
		return
	}
	if len(resp.Images) == 0 {
		log.Println("missing image data")
		return
	}

	rii.BlockDeviceMappings = i.convertImageBlockDeviceMappings(resp.Images[0].BlockDeviceMappings)
}

func (i *instance) createRunInstancesInput(instanceType string, price float64) (*ec2.RunInstancesInput, error) {
	// information we must (or can safely) copy/convert from the currently running
	// on-demand instance or we had to compute in order to place the spot bid
	retval := ec2.RunInstancesInput{

		EbsOptimized: i.EbsOptimized,

		InstanceMarketOptions: &ec2.InstanceMarketOptionsRequest{
			MarketType: aws.String(Spot),
			SpotOptions: &ec2.SpotMarketOptions{
				MaxPrice: aws.String(strconv.FormatFloat(price, 'g', 10, 64)),
			},
		},

		InstanceType: aws.String(instanceType),
		MaxCount:     aws.Int64(1),
		MinCount:     aws.Int64(1),

		Placement: i.Placement,

		SecurityGroupIds: i.convertSecurityGroups(),

		SubnetId:          i.SubnetId,
		TagSpecifications: i.generateTagsList(),
	}

	i.processImageBlockDevices(&retval)

	//populate the rest of the retval fields from launch Template and launch Configuration
	if i.asg.LaunchTemplate != nil {
		err := i.processLaunchTemplate(&retval)
		if err != nil {
			log.Println("failed to process launch template, the resulting instance configuration may be incomplete", err.Error())
			return nil, err
		}
	}
	if i.asg.launchConfiguration != nil {
		i.processLaunchConfiguration(&retval)
	}
	return &retval, nil
}

func (i *instance) generateTagsList() []*ec2.TagSpecification {
	tags := ec2.TagSpecification{
		ResourceType: aws.String("instance"),
		Tags: []*ec2.Tag{
			{
				Key:   aws.String("launched-by-autospotting"),
				Value: aws.String("true"),
			},
			{
				Key:   aws.String("launched-for-asg"),
				Value: aws.String(i.asg.name),
			},
			{
				Key:   aws.String("launched-for-replacing-instance"),
				Value: i.InstanceId,
			},
		},
	}

	if i.asg.LaunchTemplate != nil {
		tags.Tags = append(tags.Tags, &ec2.Tag{
			Key:   aws.String("LaunchTemplateID"),
			Value: i.asg.LaunchTemplate.LaunchTemplateId,
		})
		tags.Tags = append(tags.Tags, &ec2.Tag{
			Key:   aws.String("LaunchTemplateVersion"),
			Value: i.asg.LaunchTemplate.Version,
		})
	} else if i.asg.LaunchConfigurationName != nil {
		tags.Tags = append(tags.Tags, &ec2.Tag{
			Key:   aws.String("LaunchConfigurationName"),
			Value: i.asg.LaunchConfigurationName,
		})
	}

	for _, tag := range i.Tags {
		if !strings.HasPrefix(*tag.Key, "aws:") &&
			*tag.Key != "launched-by-autospotting" &&
			*tag.Key != "launched-for-asg" &&
			*tag.Key != "launched-for-replacing-instance" &&
			*tag.Key != "LaunchTemplateID" &&
			*tag.Key != "LaunchTemplateVersion" &&
			*tag.Key != "LaunchConfiguationName" {
			tags.Tags = append(tags.Tags, tag)
		}
	}
	return []*ec2.TagSpecification{&tags}
}

func (i *instance) swapWithGroupMember(asg *autoScalingGroup) (*instance, error) {
	odInstanceID := i.getReplacementTargetInstanceID()
	if odInstanceID == nil {
		log.Println("Couldn't find target on-demand instance of", *i.InstanceId)
		return nil, fmt.Errorf("couldn't find target instance for %s", *i.InstanceId)
	}

	if err := i.region.scanInstance(odInstanceID); err != nil {
		log.Printf("Couldn't describe the target on-demand instance %s", *odInstanceID)
		return nil, fmt.Errorf("target instance %s couldn't be described", *odInstanceID)
	}

	odInstance := i.region.instances.get(*odInstanceID)
	if odInstance == nil {
		log.Printf("Target on-demand instance %s couldn't be found", *odInstanceID)
		return nil, fmt.Errorf("target instance %s is missing", *odInstanceID)
	}

	if !odInstance.shouldBeReplacedWithSpot() {
		log.Printf("Target on-demand instance %s shouldn't be replaced", *odInstanceID)
		i.terminate()
		return nil, fmt.Errorf("target instance %s should not be replaced with spot",
			*odInstanceID)
	}

	asg.suspendProcesses()
	defer asg.resumeProcesses()

	desiredCapacity, maxSize := *asg.DesiredCapacity, *asg.MaxSize

	// temporarily increase AutoScaling group in case the desired capacity reaches the max size,
	// otherwise attachSpotInstance might fail
	if desiredCapacity == maxSize {
		log.Println(asg.name, "Temporarily increasing MaxSize")
		asg.setAutoScalingMaxSize(maxSize + 1)
		defer asg.setAutoScalingMaxSize(maxSize)
	}

	log.Printf("Attaching spot instance %s to the group %s",
		*i.InstanceId, asg.name)
	err := asg.attachSpotInstance(*i.InstanceId, true)

	if err != nil {
		log.Printf("Spot instance %s couldn't be attached to the group %s, terminating it...",
			*i.InstanceId, asg.name)
		i.terminate()
		return nil, fmt.Errorf("couldn't attach spot instance %s ", *i.InstanceId)
	}

	log.Printf("Terminating on-demand instance %s from the group %s",
		*odInstanceID, asg.name)
	if err := asg.terminateInstanceInAutoScalingGroup(odInstanceID, true, true); err != nil {
		log.Printf("On-demand instance %s couldn't be terminated, re-trying...",
			*odInstanceID)
		return nil, fmt.Errorf("couldn't terminate on-demand instance %s",
			*odInstanceID)
	}

	return odInstance, nil
}
