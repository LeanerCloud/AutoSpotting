// Copyright (c) 2016-2022 Cristian Măgherușan-Stanciu
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
				VolumeType:          BDM.Ebs.VolumeType,
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

func (i *instance) processLaunchConfiguration(retval *ec2.RequestLaunchTemplateData) {
	lc := i.asg.launchConfiguration

	if lc.KeyName != nil && *lc.KeyName != "" {
		retval.KeyName = lc.KeyName
	}

	if lc.SecurityGroups != nil {
		retval.SecurityGroupIds = lc.SecurityGroups
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

	retval.UserData = lc.UserData

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
				Groups:                   lc.SecurityGroups,
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

func (i *instance) createLaunchTemplateData(instanceLifecycle string) (*ec2.RequestLaunchTemplateData, error) {
	spotProduct := i.asg.getSpotProduct()

	odPrice := i.typeInfo.pricing[*spotProduct].onDemand
	mp := 1.0
	if i.asg != nil && i.asg.config.OnDemandPriceMultiplier != 0 {
		mp = i.asg.config.OnDemandPriceMultiplier
	}
	i.odPrice = odPrice * mp

	placement := ec2.LaunchTemplatePlacementRequest(*i.Placement)

	ltData := ec2.RequestLaunchTemplateData{}

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
		ltData.InstanceMarketOptions = &ec2.LaunchTemplateInstanceMarketOptionsRequest{
			MarketType: aws.String(Spot),
			SpotOptions: &ec2.LaunchTemplateSpotMarketOptionsRequest{
				MaxPrice: aws.String(strconv.FormatFloat(i.odPrice, 'g', 10, 64)),
			},
		}
	}

	ltData.Placement = &placement

	log.Printf("ltData: %+#v\n", ltData)

	return &ltData, nil
}

func (i *instance) createFleetLaunchTemplate(ltData *ec2.RequestLaunchTemplateData) (*string, *string, error) {
	ltName := "AutoSpotting-Converted-LaunchTemplate-for-" + i.asg.name

	_, err := i.region.services.ec2.CreateLaunchTemplate(&ec2.CreateLaunchTemplateInput{
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

func (i *instance) createFleetInput(ltName *string, ltVersion *string, instanceTypes []*string, lifeCycle string) *ec2.CreateFleetInput {

	var overrides []*ec2.FleetLaunchTemplateOverridesRequest

	debug.Printf("instance Details: %+#v\n", i)

	for p, inst := range instanceTypes {
		override := ec2.FleetLaunchTemplateOverridesRequest{
			InstanceType: inst,
			SubnetId:     i.SubnetId,
		}
		if i.asg.config.SpotAllocationStrategy == "capacity-optimized-prioritized" {
			override.Priority = aws.Float64(float64(p))
		}
		overrides = append(overrides, &override)
	}

	retval := &ec2.CreateFleetInput{
		LaunchTemplateConfigs: []*ec2.FleetLaunchTemplateConfigRequest{
			{
				LaunchTemplateSpecification: &ec2.FleetLaunchTemplateSpecificationRequest{
					LaunchTemplateName: ltName,
					Version:            ltVersion,
				},
				Overrides: overrides,
			},
		},
		SpotOptions: &ec2.SpotOptionsRequest{
			AllocationStrategy: aws.String(i.asg.config.SpotAllocationStrategy),
			MaxTotalPrice:      aws.String(strconv.FormatFloat(i.odPrice, 'g', 10, 64)),
		},
		OnDemandOptions: &ec2.OnDemandOptionsRequest{
			AllocationStrategy: aws.String("prioritized"),
		},
		Type: aws.String("instant"),
		TargetCapacitySpecification: &ec2.TargetCapacitySpecificationRequest{
			SpotTargetCapacity:        aws.Int64(1),
			TotalTargetCapacity:       aws.Int64(1),
			DefaultTargetCapacityType: aws.String("spot"),
		},
		TagSpecifications: i.generateTagsList(),
	}
	if lifeCycle != "spot" {
		log.Printf("Overriding default capacity type to ondemand\n")
		retval.TargetCapacitySpecification.DefaultTargetCapacityType = aws.String("on-demand")
		retval.TargetCapacitySpecification.SpotTargetCapacity = aws.Int64(0)
		retval.TargetCapacitySpecification.OnDemandTargetCapacity = aws.Int64(1)
	}
	return retval
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
			{
				Key:   aws.String("odPrice"),
				Value: aws.String(strconv.FormatFloat(i.odPrice, 'f', -1, 32)),
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

	tags.Tags = append(tags.Tags, filterTags(i.Tags)...)

	tags.Tags = deduplicateTags(tags.Tags)

	return []*ec2.TagSpecification{&tags}
}

func filterTags(tags []*ec2.Tag) []*ec2.Tag {
	var tl []*ec2.Tag

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

func deduplicateTags(tags []*ec2.Tag) []*ec2.Tag {
	seen := make(map[string]bool)
	deduplicatedTags := make([]*ec2.Tag, 0)

	for _, tag := range tags {
		key := *tag.Key
		if _, ok := seen[key]; !ok {
			seen[key] = true
			deduplicatedTags = append(deduplicatedTags, tag)
		}
	}

	return deduplicatedTags
}
