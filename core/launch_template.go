// Copyright (c) 2016-2022 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"log"
	"strings"

	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type launchTemplate struct {
	*ec2Types.LaunchTemplateVersion
	*ec2Types.Image
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
