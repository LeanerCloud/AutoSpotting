package autospotting

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func (i *instance) getAttachedEBSVolumes() []ec2Types.InstanceBlockDeviceMapping {
	if i != nil && i.BlockDeviceMappings != nil {
		return i.BlockDeviceMappings
	}
	return []ec2Types.InstanceBlockDeviceMapping{}
}

func (i *instance) getAttachedElasticIPs() ([]elasticIPDetails, error) {
	var eipDetails []elasticIPDetails

	input := &ec2.DescribeAddressesInput{
		Filters: []ec2Types.Filter{
			{
				Name:   aws.String("instance-id"),
				Values: []string{*i.InstanceId},
			},
		},
	}

	result, err := i.region.services.ec2.DescribeAddresses(context.TODO(), input)
	if err != nil {
		return nil, err
	}

	for _, address := range result.Addresses {
		eipDetails = append(eipDetails, elasticIPDetails{
			AllocationID:  address.AllocationId,
			AssociationID: address.AssociationId,
		})
	}

	return eipDetails, nil
}

func (i *instance) detachEBSVolume(volume *ec2Types.InstanceBlockDeviceMapping) error {
	if *volume.DeviceName == "/dev/xvda" {
		return nil // Skip root device
	}

	input := &ec2.DetachVolumeInput{
		VolumeId: volume.Ebs.VolumeId,
	}
	log.Printf("Attempting to detach EBS volume %s from instance %s", *volume.Ebs.VolumeId, *i.InstanceId)

	_, err := i.region.services.ec2.DetachVolume(context.TODO(), input)
	if err != nil {
		return err
	}

	err = i.region.waitUntilVolumeAvailable(volume.Ebs.VolumeId)
	if err != nil {
		log.Printf("Failed waiting for EBS volume %s to become available, error: %s", *volume.Ebs.VolumeId, err.Error())
	}
	return err
}

func (i *instance) detachElasticIP(eip elasticIPDetails) error {
	if eip.AssociationID == nil {
		return nil // Skip if not associated
	}
	log.Printf("Attempting to detach Elastic IP with association ID %s", *eip.AssociationID)

	input := &ec2.DisassociateAddressInput{
		AssociationId: eip.AssociationID,
	}

	_, err := i.region.services.ec2.DisassociateAddress(context.TODO(), input)
	log.Printf("Elastic IP with association ID %s successfully detached", *eip.AssociationID)
	return err
}

func (i *instance) attachElasticIP(eip elasticIPDetails) error {
	if eip.AllocationID == nil {
		return fmt.Errorf("elastic IP does not have an allocation ID")
	}

	input := &ec2.AssociateAddressInput{
		InstanceId:   i.InstanceId,
		AllocationId: eip.AllocationID,
	}
	log.Printf("Attempting to attach Elastic IP with allocation ID %s to instance %s", *eip.AllocationID, *i.InstanceId)

	_, err := i.region.services.ec2.AssociateAddress(context.TODO(), input)
	if err != nil {
		return err
	}

	err = i.region.tagResource(eip.AllocationID, "attached-instance", *i.InstanceId)
	if err != nil {
		return err
	}

	log.Printf("Elastic IP with allocation ID %s successfully attached to instance %s", *eip.AllocationID, *i.InstanceId)
	return nil
}
func (i *instance) attachEBSVolume(volume *ec2Types.InstanceBlockDeviceMapping, deviceName string) error {
	if *volume.DeviceName == "/dev/xvda" {
		return nil // Skip root device
	}

	input := &ec2.AttachVolumeInput{
		InstanceId: i.InstanceId,
		VolumeId:   volume.Ebs.VolumeId,
		Device:     aws.String(deviceName),
	}

	log.Printf("Attempting to attach EBS volume %s to instance %s", *volume.Ebs.VolumeId, *i.InstanceId)
	_, err := i.region.services.ec2.AttachVolume(context.TODO(), input)
	if err != nil {
		return err
	}

	// Tag the volume after attaching
	err = i.region.tagResource(volume.Ebs.VolumeId, "attached-instance", *i.InstanceId)
	if err != nil {
		return fmt.Errorf("error tagging EBS volume %s: %v", *volume.Ebs.VolumeId, err)
	}

	// Tag the volume with the device name
	err = i.region.tagResource(volume.Ebs.VolumeId, "device-name", deviceName)
	if err != nil {
		return fmt.Errorf("error tagging EBS volume %s with device name: %v", *volume.Ebs.VolumeId, err)
	}

	log.Printf("EBS volume %s successfully attached and tagged to instance %s", *volume.Ebs.VolumeId, *i.InstanceId)
	return nil
}

func (i *instance) getUnattachedTaggedEBSVolumes() ([]ec2Types.Volume, error) {
	input := &ec2.DescribeVolumesInput{
		Filters: []ec2Types.Filter{
			{
				Name:   aws.String("tag:attached-instance"),
				Values: []string{*i.InstanceId},
			},
			{
				Name:   aws.String("status"),
				Values: []string{"available"},
			},
		},
	}
	log.Printf("Retrieving unattached tagged EBS volumes for instance %s", *i.InstanceId)

	result, err := i.region.services.ec2.DescribeVolumes(context.TODO(), input)
	if err != nil {
		return nil, err
	}

	log.Printf("Found %d unattached tagged EBS volumes for instance %s", len(result.Volumes), *i.InstanceId)
	return result.Volumes, nil
}

func (i *instance) getUnattachedTaggedElasticIPs() ([]ec2Types.Address, error) {
	// Define the input with only one filter for the attached instance tag
	input := &ec2.DescribeAddressesInput{
		Filters: []ec2Types.Filter{
			{
				Name:   aws.String("tag:attached-instance"),
				Values: []string{*i.InstanceId},
			},
		},
	}
	log.Printf("Retrieving tagged Elastic IPs for instance %s", *i.InstanceId)

	// Call DescribeAddresses
	result, err := i.region.services.ec2.DescribeAddresses(context.TODO(), input)
	if err != nil {
		return nil, err
	}

	// Create a slice to hold unattached addresses
	var unattachedAddresses []ec2Types.Address

	// Iterate over the addresses to check the association
	for _, address := range result.Addresses {
		if address.InstanceId == nil || *address.InstanceId == "" {
			unattachedAddresses = append(unattachedAddresses, address)
		}
	}

	log.Printf("Found %d unattached tagged Elastic IPs for instance %s", len(unattachedAddresses), *i.InstanceId)
	return unattachedAddresses, nil
}

func detachAndAttachEBSVolume(existingInstance *instance, targetInstance *instance, vol *ec2Types.InstanceBlockDeviceMapping, errChan chan<- error) {
	log.Printf("Starting to detach EBS volume %s from instance %s", *vol.Ebs.VolumeId, *existingInstance.InstanceId)
	if err := existingInstance.detachEBSVolume(vol); err != nil {
		errChan <- fmt.Errorf("error detaching EBS volume %s: %v", *vol.Ebs.VolumeId, err)
		return
	}
	log.Printf("EBS volume %s detached successfully", *vol.Ebs.VolumeId)

	log.Printf("Starting to attach EBS volume %s to instance %s", *vol.Ebs.VolumeId, *targetInstance.InstanceId)
	if err := targetInstance.attachEBSVolume(vol, *vol.DeviceName); err != nil {
		errChan <- fmt.Errorf("error attaching EBS volume %s: %v", *vol.Ebs.VolumeId, err)
	}
}

func detachAndAttachElasticIP(existingInstance *instance, targetInstance *instance, eipDetail elasticIPDetails, errChan chan<- error) {
	log.Printf("Starting to detach Elastic IP with association ID %s", *eipDetail.AssociationID)
	if err := existingInstance.detachElasticIP(eipDetail); err != nil {
		errChan <- fmt.Errorf("error detaching Elastic IP with association ID %s: %v", *eipDetail.AssociationID, err)
		return
	}
	log.Printf("Elastic IP with association ID %s detached successfully", *eipDetail.AssociationID)

	log.Printf("Starting to attach Elastic IP with allocation ID %s to instance %s", *eipDetail.AllocationID, *targetInstance.InstanceId)
	if err := targetInstance.attachElasticIP(eipDetail); err != nil {
		errChan <- fmt.Errorf("error attaching Elastic IP with allocation ID %s: %v", *eipDetail.AllocationID, err)
	}
}
func migrateUnattachedTaggedEBSVolume(targetInstance *instance, vol *ec2Types.Volume, errChan chan<- error) {
	log.Printf("Starting to attach unattached tagged EBS volume %s to instance %s", *vol.VolumeId, *targetInstance.InstanceId)

	// Retrieve device name from volume tags
	deviceName := getDeviceNameFromTags(vol.Tags)

	ebsMapping := &ec2Types.InstanceBlockDeviceMapping{
		Ebs: &ec2Types.EbsInstanceBlockDevice{
			VolumeId: vol.VolumeId,
		},
		DeviceName: aws.String(deviceName),
	}

	if err := targetInstance.attachEBSVolume(ebsMapping, deviceName); err != nil {
		errChan <- fmt.Errorf("error attaching unattached tagged EBS volume %s: %v", *vol.VolumeId, err)
		return
	}
	// Tag the volume again to ensure it is updated
	err := targetInstance.region.tagResource(vol.VolumeId, "attached-instance", *targetInstance.InstanceId)
	if err != nil {
		errChan <- fmt.Errorf("error re-tagging EBS volume %s: %v", *vol.VolumeId, err)
	}
}

func migrateUnattachedTaggedElasticIP(targetInstance *instance, address *ec2Types.Address, errChan chan<- error) {
	log.Printf("Starting to attach unattached tagged Elastic IP with allocation ID %s to instance %s", *address.AllocationId, *targetInstance.InstanceId)
	eipDetails := elasticIPDetails{
		AllocationID: address.AllocationId,
	}

	if err := targetInstance.attachElasticIP(eipDetails); err != nil {
		errChan <- fmt.Errorf("error attaching unattached tagged Elastic IP with allocation ID %s: %v", *address.AllocationId, err)
	}
}

func getDeviceNameFromTags(tags []ec2Types.Tag) string {
	for _, tag := range tags {
		if *tag.Key == "device-name" {
			return *tag.Value
		}
	}
	return "" // Return a sensible default or handle this case as needed
}

func (i *instance) migrateResources(existingInstance *instance, attachedEBSVolumes []ec2Types.InstanceBlockDeviceMapping, attachedElasticEIPs []elasticIPDetails) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(attachedEBSVolumes)+len(attachedElasticEIPs))
	var errList []error
	var mu sync.Mutex

	// Goroutine to collect errors
	go func() {
		for err := range errChan {
			mu.Lock()
			errList = append(errList, err)
			mu.Unlock()
			log.Printf("Error encountered: %v", err)
		}
	}()

	// Migrate Attached EBS Volumes
	for _, volume := range attachedEBSVolumes {
		wg.Add(1)
		go func(vol ec2Types.InstanceBlockDeviceMapping) {
			defer wg.Done()
			detachAndAttachEBSVolume(existingInstance, i, &vol, errChan)
		}(volume)
	}

	// Migrate Attached Elastic IPs
	for _, eip := range attachedElasticEIPs {
		wg.Add(1)
		go func(eipDetail elasticIPDetails) {
			defer wg.Done()
			detachAndAttachElasticIP(existingInstance, i, eipDetail, errChan)
		}(eip)
	}

	// Migrate Unattached Tagged EBS Volumes
	unattachedTaggedVolumes, err := existingInstance.getUnattachedTaggedEBSVolumes()
	if err != nil {
		return fmt.Errorf("error retrieving unattached tagged EBS volumes: %v", err)
	}
	for _, volume := range unattachedTaggedVolumes {
		wg.Add(1)
		go func(vol ec2Types.Volume) {
			defer wg.Done()
			migrateUnattachedTaggedEBSVolume(i, &vol, errChan)
		}(volume)
	}

	// Migrate Unattached Tagged Elastic IPs
	unattachedTaggedEIPs, err := existingInstance.getUnattachedTaggedElasticIPs()
	if err != nil {
		return fmt.Errorf("error retrieving unattached tagged Elastic IPs: %v", err)
	}
	for _, address := range unattachedTaggedEIPs {
		wg.Add(1)
		go func(addr ec2Types.Address) {
			defer wg.Done()
			migrateUnattachedTaggedElasticIP(i, &addr, errChan)
		}(address)
	}

	wg.Wait()
	close(errChan)

	if len(errList) > 0 {
		var combinedErrorMessages strings.Builder
		for _, err := range errList {
			combinedErrorMessages.WriteString(err.Error() + "; ")
		}
		log.Printf("Failures encountered migrating stateful resources from instance %s to %s: %s",
			*existingInstance.InstanceId, *i.InstanceId, combinedErrorMessages.String())
		return fmt.Errorf("multiple errors occurred during migration: %s", combinedErrorMessages.String())
	}

	log.Printf("Finished migrating stateful resources from instance %s to %s",
		*existingInstance.InstanceId, *i.InstanceId)
	return nil
}
