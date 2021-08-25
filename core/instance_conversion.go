// Copyright (c) 2016-2021 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

// instance_conversion.go contains functions that help cloning OnDemand instance configuration to new Spot instances.

import (
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
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

func (i *instance) getPriceToBid(
	baseOnDemandPrice float64, currentSpotPrice float64, spotPremium float64) float64 {

	debug.Println("BiddingPolicy: ", i.region.conf.BiddingPolicy)

	if i.region.conf.BiddingPolicy == DefaultBiddingPolicy {
		log.Println("Bidding base on demand price", baseOnDemandPrice, "to replace instance", *i.InstanceId)
		return baseOnDemandPrice
	}

	bufferPrice := math.Min(baseOnDemandPrice, ((currentSpotPrice-spotPremium)*(1.0+i.region.conf.SpotPriceBufferPercentage/100.0))+spotPremium)
	log.Println("Bidding buffer-based price of", bufferPrice, "based on current spot price of", currentSpotPrice,
		"and buffer percentage of", i.region.conf.SpotPriceBufferPercentage, "to replace instance", i.InstanceId)
	return bufferPrice
}

func (i *instance) convertLaunchConfigurationBlockDeviceMappings(BDMs []*autoscaling.BlockDeviceMapping) []*ec2.LaunchTemplateBlockDeviceMappingRequest {

	bds := []*ec2.LaunchTemplateBlockDeviceMappingRequest{}
	if len(BDMs) == 0 {
		debug.Println("Missing LC block device mappings")
	}

	for _, BDM := range BDMs {

		ec2BDM := &ec2.LaunchTemplateBlockDeviceMappingRequest{
			DeviceName:  BDM.DeviceName,
			VirtualName: BDM.VirtualName,
		}

		if BDM.Ebs != nil {
			ec2BDM.Ebs = &ec2.LaunchTemplateEbsBlockDeviceRequest{
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

func (i *instance) convertLaunchTemplateBlockDeviceMappings(BDMs []*ec2.LaunchTemplateBlockDeviceMapping) []*ec2.LaunchTemplateBlockDeviceMappingRequest {

	bds := []*ec2.LaunchTemplateBlockDeviceMappingRequest{}
	if len(BDMs) == 0 {
		log.Println("Missing LT block device mappings")
	}

	for _, BDM := range BDMs {

		ec2BDM := &ec2.LaunchTemplateBlockDeviceMappingRequest{
			DeviceName:  BDM.DeviceName,
			VirtualName: BDM.VirtualName,
		}

		if BDM.Ebs != nil {
			ec2BDM.Ebs = &ec2.LaunchTemplateEbsBlockDeviceRequest{
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

func (i *instance) convertImageBlockDeviceMappings(BDMs []*ec2.BlockDeviceMapping) []*ec2.LaunchTemplateBlockDeviceMappingRequest {

	bds := []*ec2.LaunchTemplateBlockDeviceMappingRequest{}
	if len(BDMs) == 0 {
		log.Println("Missing Image block device mappings")
	}

	for _, BDM := range BDMs {

		ec2BDM := &ec2.LaunchTemplateBlockDeviceMappingRequest{
			DeviceName:  BDM.DeviceName,
			VirtualName: BDM.VirtualName,
		}

		if BDM.Ebs != nil {
			ec2BDM.Ebs = &ec2.LaunchTemplateEbsBlockDeviceRequest{
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

func (i *instance) processLaunchTemplate(retval *ec2.RequestLaunchTemplateData) error {
	ver := i.asg.LaunchTemplate.Version
	id := i.asg.LaunchTemplate.LaunchTemplateId

	ltData, err := i.getlaunchTemplate(id, ver)
	if err != nil {
		return err
	}

	retval.BlockDeviceMappings = i.convertLaunchTemplateBlockDeviceMappings(ltData.BlockDeviceMappings)

	if having, nis := i.launchTemplateHasNetworkInterfaces(ltData); having {
		for _, ni := range nis {
			retval.NetworkInterfaces = append(retval.NetworkInterfaces,
				&ec2.LaunchTemplateInstanceNetworkInterfaceSpecificationRequest{
					AssociatePublicIpAddress: ni.AssociatePublicIpAddress,
					SubnetId:                 i.SubnetId,
					DeviceIndex:              ni.DeviceIndex,
					Groups:                   i.convertSecurityGroups(),
				},
			)
		}
		retval.SecurityGroupIds = nil
	}
	return nil
}

func (i *instance) processLaunchConfiguration(retval *ec2.RequestLaunchTemplateData) {
	lc := i.asg.launchConfiguration

	if lc.KeyName != nil && *lc.KeyName != "" {
		retval.KeyName = lc.KeyName
	}

	if lc.IamInstanceProfile != nil {
		if strings.HasPrefix(*lc.IamInstanceProfile, "arn:aws:iam:") {
			retval.IamInstanceProfile = &ec2.LaunchTemplateIamInstanceProfileSpecificationRequest{
				Arn: lc.IamInstanceProfile,
			}
		} else {
			retval.IamInstanceProfile = &ec2.LaunchTemplateIamInstanceProfileSpecificationRequest{
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
		retval.Monitoring = &ec2.LaunchTemplatesMonitoringRequest{
			Enabled: lc.InstanceMonitoring.Enabled}
	}

	if lc.AssociatePublicIpAddress != nil || i.SubnetId != nil {
		// Instances are running in a VPC.
		retval.NetworkInterfaces = []*ec2.LaunchTemplateInstanceNetworkInterfaceSpecificationRequest{
			{
				AssociatePublicIpAddress: lc.AssociatePublicIpAddress,
				DeviceIndex:              aws.Int64(0),
				SubnetId:                 i.SubnetId,
				Groups:                   i.convertSecurityGroups(),
			},
		}
		retval.SecurityGroupIds = nil
	}
}

func (i *instance) processImageBlockDevices(rii *ec2.RequestLaunchTemplateData) {
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

func (i *instance) createLaunchTemplateData() (*ec2.RequestLaunchTemplateData, error) {
	i.price = i.typeInfo.pricing.onDemand / i.region.conf.OnDemandPriceMultiplier * i.asg.config.OnDemandPriceMultiplier

	placement := ec2.LaunchTemplatePlacementRequest(*i.Placement)

	ltData := ec2.RequestLaunchTemplateData{}

	// populate the base of the ltData fields from launch Template and launch
	// Configuration then set additional fields from computed values. SGs need to
	// be set first because they may also be set in the network configuration of
	// the LT or LC, in which case the below initialization will be reverted.

	ltData.SecurityGroupIds = i.convertSecurityGroups()

	if i.asg.LaunchTemplate != nil {
		err := i.processLaunchTemplate(&ltData)
		if err != nil {
			log.Println("failed to process launch template, the resulting instance configuration may be incomplete", err.Error())
			return nil, err
		}
	}
	if i.asg.launchConfiguration != nil {
		i.processLaunchConfiguration(&ltData)
	}

	ltData.EbsOptimized = i.EbsOptimized

	ltData.InstanceMarketOptions = &ec2.LaunchTemplateInstanceMarketOptionsRequest{
		MarketType: aws.String(Spot),
		SpotOptions: &ec2.LaunchTemplateSpotMarketOptionsRequest{
			MaxPrice: aws.String(strconv.FormatFloat(i.price, 'g', 10, 64)),
		},
	}

	ltData.Placement = &placement

	ltData.TagSpecifications = i.generateTagsList()

	i.processImageBlockDevices(&ltData)

	return &ltData, nil
}

func (i *instance) createFleetLaunchTemplate(ltData *ec2.RequestLaunchTemplateData) (*string, error) {
	ltName := "AutoSpotting-Temporary-LaunchTemplate-for-" + *i.Instance.InstanceId

	_, err := i.region.services.ec2.CreateLaunchTemplate(&ec2.CreateLaunchTemplateInput{
		LaunchTemplateName: aws.String(ltName),
		LaunchTemplateData: ltData,
	})

	if err != nil {
		log.Println("failed to create LaunchTemplate,", err.Error())
		//if the LT already exists maybe from a previous failed run we take it and use it
		if !strings.Contains(err.Error(), "AlreadyExistsException") {
			return nil, err
		}
		log.Println("Reusing existing LaunchTemplate ", ltName)
		err = nil
	}

	return &ltName, err
}

func (i *instance) createFleetInput(ltName *string) (*ec2.CreateFleetInput, error) {
	i.price = i.typeInfo.pricing.onDemand / i.region.conf.OnDemandPriceMultiplier * i.asg.config.OnDemandPriceMultiplier
	instanceTypes, err := i.getCompatibleSpotInstanceTypesListSortedAscendingByPrice(
		i.asg.getAllowedInstanceTypes(i),
		i.asg.getDisallowedInstanceTypes(i))

	if err != nil {
		log.Println("Couldn't determine the cheapest compatible spot instance type")
		return nil, err
	}

	var overrides []*ec2.FleetLaunchTemplateOverridesRequest

	for _, inst := range instanceTypes {
		override := ec2.FleetLaunchTemplateOverridesRequest{
			InstanceType: aws.String(inst.instanceType),
		}
		overrides = append(overrides, &override)
	}

	retval := &ec2.CreateFleetInput{
		LaunchTemplateConfigs: []*ec2.FleetLaunchTemplateConfigRequest{
			{
				LaunchTemplateSpecification: &ec2.FleetLaunchTemplateSpecificationRequest{
					LaunchTemplateName: ltName,
					Version:            aws.String("$Latest"),
				},
				Overrides: overrides,
			},
		},
		SpotOptions: &ec2.SpotOptionsRequest{
			AllocationStrategy: aws.String("capacity-optimized-prioritized"), // TODO : make the Spot AllocationStrategy configurable globally and per ASG
		},
		Type: aws.String("instant"),
		TargetCapacitySpecification: &ec2.TargetCapacitySpecificationRequest{
			SpotTargetCapacity:        aws.Int64(1),
			TotalTargetCapacity:       aws.Int64(1),
			DefaultTargetCapacityType: aws.String("spot"),
		},
	}
	return retval, err
}

func (i *instance) generateTagsList() []*ec2.LaunchTemplateTagSpecificationRequest {
	tags := ec2.LaunchTemplateTagSpecificationRequest{
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
			*tag.Key != "LaunchConfigurationName" {
			tags.Tags = append(tags.Tags, tag)
		}
	}
	return []*ec2.LaunchTemplateTagSpecificationRequest{&tags}
}
