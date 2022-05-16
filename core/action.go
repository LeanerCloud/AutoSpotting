// Copyright (c) 2016-2022 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

type target struct {
	autospotting     *AutoSpotting
	onDemandInstance *instance
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
	autospotting.handleNewOnDemandInstanceLaunch(tsi.target.onDemandInstance.region, tsi.target.onDemandInstance)
}
