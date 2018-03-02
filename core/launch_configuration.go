package autospotting

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
)

type launchConfiguration struct {
	*autoscaling.LaunchConfiguration

	secGroupRegex *regexp.Regexp
}

func (lc *launchConfiguration) countLaunchConfigEphemeralVolumes() int {
	count := 0

	if lc.BlockDeviceMappings == nil {
		return count
	}

	for _, mapping := range lc.BlockDeviceMappings {
		if mapping.VirtualName != nil &&
			strings.Contains(*mapping.VirtualName, "ephemeral") {
			debug.Println("Found ephemeral device mapping", *mapping.VirtualName)
			count++
		}
	}

	logger.Printf("Launch configuration would attach %d ephemeral volumes if available", count)

	return count
}

func (lc *launchConfiguration) convertLaunchConfigurationToSpotSpecification(
	baseInstance *instance,
	newInstance instanceTypeInformation,
	conn *connections,
	az string) (*ec2.RequestSpotLaunchSpecification, error) {

	var spotLS ec2.RequestSpotLaunchSpecification

	// convert attributes
	spotLS.BlockDeviceMappings = copyBlockDeviceMappings(lc.BlockDeviceMappings)

	if lc.EbsOptimized != nil {
		spotLS.EbsOptimized = lc.EbsOptimized
	}

	if newInstance.hasEBSOptimization && newInstance.pricing.ebsSurcharge == 0.0 {
		spotLS.SetEbsOptimized(true)
	}

	// The launch configuration's IamInstanceProfile field can store either a
	// human-friendly ID or an ARN, so we have to see which one is it
	var iamInstanceProfile ec2.IamInstanceProfileSpecification

	if lc.IamInstanceProfile != nil {

		if strings.HasPrefix(*lc.IamInstanceProfile, "arn:aws:") {
			iamInstanceProfile.Arn = lc.IamInstanceProfile
		} else {
			iamInstanceProfile.Name = lc.IamInstanceProfile
		}

		spotLS.IamInstanceProfile = &iamInstanceProfile
	}

	spotLS.ImageId = lc.ImageId

	spotLS.InstanceType = &newInstance.instanceType

	// these ones should NOT be copied, they break the SpotLaunchSpecification,
	// so that it can't be launched
	// - spotLS.KernelId
	// - spotLS.RamdiskId

	if lc.KeyName != nil && *lc.KeyName != "" {
		spotLS.KeyName = lc.KeyName
	}

	if lc.InstanceMonitoring != nil {
		spotLS.Monitoring = &ec2.RunInstancesMonitoringEnabled{
			Enabled: lc.InstanceMonitoring.Enabled,
		}
	}

	secGroupIDs, err := lc.getSecurityGroupIDs(conn, lc.SecurityGroups)
	if err != nil {
		return nil, err
	}

	if lc.AssociatePublicIpAddress != nil || baseInstance.SubnetId != nil {
		// Instances are running in a VPC.
		spotLS.NetworkInterfaces = []*ec2.InstanceNetworkInterfaceSpecification{
			{
				AssociatePublicIpAddress: lc.AssociatePublicIpAddress,
				DeviceIndex:              aws.Int64(0),
				SubnetId:                 baseInstance.SubnetId,
				Groups:                   secGroupIDs,
			},
		}
	} else {
		// Instances are running in EC2 Classic
		spotLS.SecurityGroupIds = secGroupIDs
	}

	if lc.UserData != nil && *lc.UserData != "" {
		spotLS.UserData = lc.UserData
	}

	spotLS.Placement = &ec2.SpotPlacement{AvailabilityZone: &az}

	return &spotLS, nil
}

func copyBlockDeviceMappings(
	lcBDMs []*autoscaling.BlockDeviceMapping) []*ec2.BlockDeviceMapping {

	var ec2BDMlist []*ec2.BlockDeviceMapping

	for _, lcBDM := range lcBDMs {
		var ec2BDM ec2.BlockDeviceMapping

		ec2BDM.DeviceName = lcBDM.DeviceName

		// EBS volume information
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

		ec2BDM.VirtualName = lcBDM.VirtualName

		ec2BDMlist = append(ec2BDMlist, &ec2BDM)

	}
	return ec2BDMlist
}

// We don't know whether we got security group names or ids. We assume
// that the ones starting with "sg-" are ids and then search for the IDs
// of the other ones.
func (lc *launchConfiguration) getSecurityGroupIDs(conn *connections, secGroups []*string) ([]*string, error) {
	var (
		names    []*string
		ids      []*string
		outNames *ec2.DescribeSecurityGroupsOutput
		err      error
	)

	for _, secGroupStr := range secGroups {
		// we assume strings that match are IDs already
		if lc.secGroupRegex.MatchString(*secGroupStr) {
			ids = append(ids, aws.String(*secGroupStr))
		} else {
			names = append(names, aws.String(*secGroupStr))
		}
	}

	if len(names) > 0 {
		inputNames := &ec2.DescribeSecurityGroupsInput{
			GroupNames: names,
		}

		outNames, err = conn.ec2.DescribeSecurityGroups(inputNames)
		if err != nil {
			return nil, err
		}
	}

	if outNames != nil {
		for _, group := range outNames.SecurityGroups {
			ids = append(ids, aws.String(*group.GroupId))
		}
	}

	return ids, nil
}
