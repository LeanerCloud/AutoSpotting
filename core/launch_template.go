// Copyright (c) 2016-2019 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"log"
	"strings"

	"github.com/aws/aws-sdk-go/service/ec2"
)

type launchTemplate struct {
	*ec2.LaunchTemplateVersion
	*ec2.Image
}

func (lt *launchTemplate) countLaunchTemplateEphemeralVolumes() int {
	count := 0

	if lt == nil || lt.Image == nil || lt.Image.BlockDeviceMappings == nil {
		return count
	}

	for _, mapping := range lt.Image.BlockDeviceMappings {
		if mapping.VirtualName != nil &&
			strings.Contains(*mapping.VirtualName, "ephemeral") {
			debug.Println("Found ephemeral device mapping", *mapping.VirtualName)
			count++
		}
	}

	log.Printf("Launch template version would attach %d ephemeral volumes if available", count)

	return count
}
