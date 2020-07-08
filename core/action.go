// Copyright (c) 2016-2019 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

type target struct {
	asg              *autoScalingGroup
	totalInstances   int64
	onDemandInstance *instance
	spotInstance     *instance
}

type runer interface {
	run()
}

// No-op run
type skipRun struct {
	reason string
}

func (s skipRun) run() {}

// enables the ASG for the new event-based logic
type enableEventHandling struct {
	target target
}

func (eeh enableEventHandling) run() {
	eeh.target.asg.enableForInstanceLaunchEventHandling()
}

// terminates a random spot instance after enabling the event-based logic
type terminateSpotInstance struct {
	target target
}

func (tsi terminateSpotInstance) run() {
	asg := tsi.target.asg

	asg.enableForInstanceLaunchEventHandling()
	asg.terminateRandomSpotInstanceIfHavingEnough(
		tsi.target.totalInstances, true)
}

// launches a spot instance replacement
type launchSpotReplacement struct {
	target target
}

func (lsr launchSpotReplacement) run() {
	spotInstanceID, err := lsr.target.onDemandInstance.launchSpotReplacement()
	if err != nil {
		logger.Printf("Could not launch cheapest spot instance: %s", err)
		return
	}
	logger.Printf("Successfully launched spot instance %s, exiting...", *spotInstanceID)
	return
}

type terminateUnneededSpotInstance struct {
	target target
}

func (tusi terminateUnneededSpotInstance) run() {
	asg := tusi.target.asg
	total := tusi.target.totalInstances
	spotInstance := tusi.target.spotInstance
	spotInstanceID := *spotInstance.InstanceId

	asg.terminateRandomSpotInstanceIfHavingEnough(total, true)
	logger.Println("Spot instance", spotInstanceID, "is not need anymore by ASG",
		asg.name, "terminating the spot instance.")
	spotInstance.terminate()
}

type swapSpotInstance struct {
	target target
}

func (ssi swapSpotInstance) run() {
	asg := ssi.target.asg
	spotInstanceID := *ssi.target.spotInstance.InstanceId
	asg.replaceOnDemandInstanceWithSpot(nil, spotInstanceID)
}
