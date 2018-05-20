package autospotting

import (
	"strings"

	"github.com/aws/aws-sdk-go/service/autoscaling"
)

type launchConfiguration struct {
	*autoscaling.LaunchConfiguration
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

	logger.Printf("Launch configuration would attach %d ephemeral volumes if available", count)

	return count
}
