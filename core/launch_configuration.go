// Copyright (c) 2016-2022 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"log"
	"strings"

	types "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
)

type launchConfiguration struct {
	*types.LaunchConfiguration
}

func (lc *launchConfiguration) countLaunchConfigEphemeralVolumes() int {
	count := 0

	if lc == nil || lc.BlockDeviceMappings == nil {
		return count
	}

	for _, mapping := range lc.BlockDeviceMappings {
		if mapping.VirtualName != nil &&
			strings.Contains(*mapping.VirtualName, "ephemeral") {
			debug.Println("Found ephemeral device mapping", *mapping.VirtualName)
			count++
		}
	}

	log.Printf("Launch configuration would attach %d ephemeral volumes if available", count)

	return count
}
