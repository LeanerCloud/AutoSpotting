// Copyright (c) 2016-2022 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import "log"

type target struct {
	autospotting     *AutoSpotting
	onDemandInstance *instance
	sqsReceiptHandle *string
}

type runer interface {
	run()
}

// No-op run
type skipRun struct {
	reason string
}

func (s skipRun) run() {}

// terminates a random spot instance after enabling the event-based logic
type replaceAndTerminateInstance struct {
	target target
}

func (tsi replaceAndTerminateInstance) run() {
	autospotting := tsi.target.autospotting
	err := autospotting.handleNewOnDemandInstanceLaunch(
		tsi.target.onDemandInstance.region, tsi.target.onDemandInstance, tsi.target.sqsReceiptHandle)
	if err != nil {
		log.Printf("Failed to handle new on demand instance launch in %s : %s",
			tsi.target.onDemandInstance.region.name, err.Error())
	}
}
