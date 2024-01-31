// Copyright (c) 2016-2022 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

// instance_conversion.go contains functions that help cloning OnDemand instance configuration to new Spot instances.

import (
	"context"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	autoscalingTypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
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

func (i *instance) convertLaunchConfigurationBlockDeviceMappings(BDMs []autoscalingTypes.BlockDeviceMapping) []ec2Types.LaunchTemplateBlockDeviceMappingRequest {

	bds := []ec2Types.LaunchTemplateBlockDeviceMappingRequest{}
	if len(BDMs) == 0 {
		debug.Println("Missing LC block device mappings")
	}

	for _, BDM := range BDMs {

		ec2BDM := &ec2Types.LaunchTemplateBlockDeviceMappingRequest{
			DeviceName:  BDM.DeviceName,
			VirtualName: BDM.VirtualName,
		}

		if BDM.Ebs != nil {
			ec2BDM.Ebs = &ec2Types.LaunchTemplateEbsBlockDeviceRequest{
				DeleteOnTermination: BDM.Ebs.DeleteOnTermination,
				Encrypted:           BDM.Ebs.Encrypted,
				Iops:                BDM.Ebs.Iops,
				SnapshotId:          BDM.Ebs.SnapshotId,
				VolumeSize:          BDM.Ebs.VolumeSize,
				VolumeType:          ec2Types.VolumeType(*BDM.Ebs.VolumeType),
			}
		}

		// handle the noDevice field directly by skipping the device if set to true
		if BDM.NoDevice != nil && *BDM.NoDevice {
			continue
		}
		bds = append(bds, *ec2BDM)
	}

	if len(bds) == 0 {
		return nil
	}
	return bds
}

func (i *instance) convertImageBlockDeviceMappings(BDMs []ec2Types.BlockDeviceMapping) []ec2Types.LaunchTemplateBlockDeviceMappingRequest {

	bds := []ec2Types.LaunchTemplateBlockDeviceMappingRequest{}
	if len(BDMs) == 0 {
		log.Println("Missing Image block device mappings")
	}

	for _, BDM := range BDMs {

		ec2BDM := ec2Types.LaunchTemplateBlockDeviceMappingRequest{
			DeviceName:  BDM.DeviceName,
			VirtualName: BDM.VirtualName,
		}

		if BDM.Ebs != nil {
			ec2BDM.Ebs = &ec2Types.LaunchTemplateEbsBlockDeviceRequest{
				DeleteOnTermination: BDM.Ebs.DeleteOnTermination,
				Encrypted:           BDM.Ebs.Encrypted,
				Iops:                BDM.Ebs.Iops,
				SnapshotId:          BDM.Ebs.SnapshotId,
				VolumeSize:          BDM.Ebs.VolumeSize,
				VolumeType:          BDM.Ebs.VolumeType,
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

func (i *instance) convertSecurityGroups() []string {
	groupIDs := []string{}
	for _, sg := range i.SecurityGroups {
		groupIDs = append(groupIDs, *sg.GroupId)
	}

	return groupIDs
}

func (i *instance) getlaunchTemplate(id, ver *string) (*ec2Types.ResponseLaunchTemplateData, error) {
	res, err := i.region.services.ec2.DescribeLaunchTemplateVersions(
		context.TODO(),
		&ec2.DescribeLaunchTemplateVersionsInput{
			Versions:         []string{*ver},
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

func (i *instance) processLaunchConfiguration(retval *ec2Types.RequestLaunchTemplateData) {
	lc := i.asg.launchConfiguration

	if lc.KeyName != nil && *lc.KeyName != "" {
		retval.KeyName = lc.KeyName
	}

	if lc.SecurityGroups != nil {
		retval.SecurityGroupIds = lc.SecurityGroups
	}

	if lc.IamInstanceProfile != nil {
		if strings.HasPrefix(*lc.IamInstanceProfile, "arn:aws:iam:") {
			retval.IamInstanceProfile = &ec2Types.LaunchTemplateIamInstanceProfileSpecificationRequest{
				Arn: lc.IamInstanceProfile,
			}
		} else {
			retval.IamInstanceProfile = &ec2Types.LaunchTemplateIamInstanceProfileSpecificationRequest{
				Name: lc.IamInstanceProfile,
			}
		}
	}
	retval.ImageId = lc.ImageId

	retval.UserData = lc.UserData

	BDMs := i.convertLaunchConfigurationBlockDeviceMappings(lc.BlockDeviceMappings)

	if len(BDMs) > 0 {
		retval.BlockDeviceMappings = BDMs
	}

	if lc.InstanceMonitoring != nil {
		retval.Monitoring = &ec2Types.LaunchTemplatesMonitoringRequest{
			Enabled: lc.InstanceMonitoring.Enabled}
	}

	if lc.AssociatePublicIpAddress != nil || i.SubnetId != nil {
		// Instances are running in a VPC.
		retval.NetworkInterfaces = []ec2Types.LaunchTemplateInstanceNetworkInterfaceSpecificationRequest{
			{
				AssociatePublicIpAddress: lc.AssociatePublicIpAddress,
				DeviceIndex:              aws.Int32(0),
				SubnetId:                 i.SubnetId,
				Groups:                   lc.SecurityGroups,
			},
		}
		retval.SecurityGroupIds = nil
	}
}

func (i *instance) processImageBlockDevices(rii *ec2Types.RequestLaunchTemplateData) {
	svc := i.region.services.ec2

	resp, err := svc.DescribeImages(
		context.TODO(),
		&ec2.DescribeImagesInput{
			ImageIds: []string{*i.ImageId},
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

func (i *instance) createLaunchTemplateData(instanceLifecycle string) (*ec2Types.RequestLaunchTemplateData, error) {
	spotProduct := i.asg.getSpotProduct()

	odPrice := i.typeInfo.pricing[*spotProduct].onDemand
	mp := 1.0
	if i.asg != nil && i.asg.config.OnDemandPriceMultiplier != 0 {
		mp = i.asg.config.OnDemandPriceMultiplier
	}
	i.initialPrice = odPrice * mp

	placement := ec2Types.LaunchTemplatePlacementRequest(*i.Placement)

	ltData := ec2Types.RequestLaunchTemplateData{}

	// populate the base of the ltData fields from launch Template and launch
	// Configuration then set additional fields from computed values. SGs need to
	// be set first because they may also be set in the network configuration of
	// the LT or LC, in which case the below initialization will be reverted.

	sgs := i.convertSecurityGroups()
	if len(sgs) > 0 {
		ltData.SecurityGroupIds = sgs
	}

	i.processImageBlockDevices(&ltData)

	if i.asg.launchConfiguration != nil {
		i.processLaunchConfiguration(&ltData)
	}

	ltData.EbsOptimized = i.EbsOptimized

	if instanceLifecycle == "spot" {
		ltData.InstanceMarketOptions = &ec2Types.LaunchTemplateInstanceMarketOptionsRequest{
			MarketType: Spot,
			SpotOptions: &ec2Types.LaunchTemplateSpotMarketOptionsRequest{
				MaxPrice: aws.String(strconv.FormatFloat(i.initialPrice, 'g', 10, 64)),
			},
		}
	}

	ltData.Placement = &placement

	log.Printf("ltData: %+#v\n", ltData)

	return &ltData, nil
}

func (i *instance) createFleetLaunchTemplate(ltData *ec2Types.RequestLaunchTemplateData) (*string, *string, error) {
	ltName := "AutoSpotting-Converted-LaunchTemplate-for-" + i.asg.name

	_, err := i.region.services.ec2.CreateLaunchTemplate(
		context.TODO(),
		&ec2.CreateLaunchTemplateInput{
			LaunchTemplateName: aws.String(ltName),
			LaunchTemplateData: ltData,
		})

	if err != nil {
		log.Println("failed to create LaunchTemplate,", err.Error())
		// if the LT already exists maybe from a previous failed run we take it and use it
		if !strings.Contains(err.Error(), "AlreadyExistsException") {
			return nil, nil, err
		}
		log.Println("Reusing existing LaunchTemplate ", ltName)
		err = nil
	}

	return &ltName, aws.String("$Latest"), err
}

func (i *instance) createFleetInput(ltName *string, ltVersion *string, instanceTypes []string, lifeCycle string) *ec2.CreateFleetInput {

	var overrides []ec2Types.FleetLaunchTemplateOverridesRequest

	debug.Printf("instance Details: %+#v\n", i)

	for p, inst := range instanceTypes {
		override := ec2Types.FleetLaunchTemplateOverridesRequest{
			InstanceType: ec2Types.InstanceType(inst),
			SubnetId:     i.SubnetId,
		}
		if i.asg.config.SpotAllocationStrategy == "capacity-optimized-prioritized" {
			override.Priority = aws.Float64(float64(p))
		}
		overrides = append(overrides, override)
	}

	retval := &ec2.CreateFleetInput{
		LaunchTemplateConfigs: []ec2Types.FleetLaunchTemplateConfigRequest{
			{
				LaunchTemplateSpecification: &ec2Types.FleetLaunchTemplateSpecificationRequest{
					LaunchTemplateName: ltName,
					Version:            ltVersion,
				},
				Overrides: overrides,
			},
		},
		SpotOptions: &ec2Types.SpotOptionsRequest{
			AllocationStrategy: ec2Types.SpotAllocationStrategy(i.asg.config.SpotAllocationStrategy),
			MaxTotalPrice:      aws.String(strconv.FormatFloat(i.initialPrice, 'g', 10, 64)),
		},
		OnDemandOptions: &ec2Types.OnDemandOptionsRequest{
			AllocationStrategy: "prioritized",
		},
		Type: "instant",
		TargetCapacitySpecification: &ec2Types.TargetCapacitySpecificationRequest{
			SpotTargetCapacity:        aws.Int32(1),
			TotalTargetCapacity:       aws.Int32(1),
			DefaultTargetCapacityType: "spot",
		},
		TagSpecifications: i.generateTagsList(),
	}
	if lifeCycle != "spot" {
		log.Printf("Overriding default capacity type to ondemand\n")
		retval.TargetCapacitySpecification.DefaultTargetCapacityType = "on-demand"
		retval.TargetCapacitySpecification.SpotTargetCapacity = aws.Int32(0)
		retval.TargetCapacitySpecification.OnDemandTargetCapacity = aws.Int32(1)
	}
	return retval
}

func (i *instance) generateTagsList() []ec2Types.TagSpecification {
	tags := ec2Types.TagSpecification{
		ResourceType: "instance",
		Tags: []ec2Types.Tag{
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
			{
				Key:   aws.String("odPrice"),
				Value: aws.String(strconv.FormatFloat(i.initialPrice, 'f', -1, 32)),
			},
		},
	}

	if i.asg.LaunchTemplate != nil {
		tags.Tags = append(tags.Tags, ec2Types.Tag{
			Key:   aws.String("LaunchTemplateID"),
			Value: i.asg.LaunchTemplate.LaunchTemplateId,
		})
		tags.Tags = append(tags.Tags, ec2Types.Tag{
			Key:   aws.String("LaunchTemplateVersion"),
			Value: i.asg.LaunchTemplate.Version,
		})
	} else if i.asg.LaunchConfigurationName != nil {
		tags.Tags = append(tags.Tags, ec2Types.Tag{
			Key:   aws.String("LaunchConfigurationName"),
			Value: i.asg.LaunchConfigurationName,
		})
	}

	tags.Tags = append(tags.Tags, filterTags(i.Tags)...)

	tags.Tags = deduplicateTags(tags.Tags)

	return []ec2Types.TagSpecification{tags}
}

func filterTags(tags []ec2Types.Tag) []ec2Types.Tag {
	var tl []ec2Types.Tag

	var tagsToSkip = []string{
		"launched-by-autospotting",
		"launched-for-asg",
		"launched-for-replacing-instance",
		"LaunchTemplateID",
		"LaunchTemplateVersion",
		"LaunchConfigurationName",
	}

	for _, tag := range tags {
		if !strings.HasPrefix(*tag.Key, "aws:") &&
			!itemInSlice(*tag.Key, tagsToSkip) {
			tl = append(tl, tag)
		}
	}
	return tl
}

func deduplicateTags(tags []ec2Types.Tag) []ec2Types.Tag {
	seen := make(map[string]bool)
	deduplicatedTags := make([]ec2Types.Tag, 0)

	for _, tag := range tags {
		key := *tag.Key
		if _, ok := seen[key]; !ok {
			seen[key] = true
			deduplicatedTags = append(deduplicatedTags, tag)
		}
	}

	return deduplicatedTags
}
