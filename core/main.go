// Copyright (c) 2016-2022 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	ec2instancesinfo "github.com/LeanerCloud/ec2-instances-info"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/marketplacemetering"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/aws/aws-sdk-go/service/ssm"
)

var debug *log.Logger
var totalSavings float64

// AutoSpotting hosts global configuration and has as methods all the public
// entrypoints of this library
type AutoSpotting struct {
	config      *Config
	mainEC2Conn ec2iface.EC2API
}

var as *AutoSpotting

// Init initializes some data structures reusable across multiple event runs
func (a *AutoSpotting) Init(cfg *Config) {

	if cfg.AutomatedInstanceDataUpdate {
		key := "d29c10d16cd9b9050ccdc790e24ddfd55547e274cc9800b08067b649d4b151f3"
		err := ec2instancesinfo.UpdateData(nil, &key)
		if err != nil {
			fmt.Println("Couldn't update instance type data, reverting to static compile-time data", err.Error())
		}
	}

	data, err := ec2instancesinfo.Data()
	if err != nil {
		log.Fatal(err.Error())
	}

	cfg.InstanceData = data
	a.config = cfg
	a.config.setupLogging()
	// use this only to list all the other regions
	a.mainEC2Conn = connectEC2(a.config.MainRegion)
	as = a
}

// RunningFromLambda quite obviously returns true when running from Lambda.
func RunningFromLambda() bool {
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		log.Println("Running from Lambda")
		return true
	}
	return false
}

// ProcessCronEvent starts processing all AWS regions looking for AutoScaling groups
// enabled and taking action by replacing more pricy on-demand instances with
// compatible and cheaper spot instances.
func (a *AutoSpotting) ProcessCronEvent() {
	// Clear FinalRecap map
	a.config.FinalRecap = make(map[string][]string)

	a.config.addDefaultFilteringMode()
	a.config.addDefaultFilter()

	allRegions, err := a.getRegions()

	if err != nil {
		log.Println(err.Error())
		return
	}

	a.processRegions(allRegions)

	// Print Final Recap
	log.Println("####### BEGIN FINAL RECAP #######")
	for r, a := range a.config.FinalRecap {
		for _, t := range a {
			log.Printf("%s %s\n", r, t)
		}
	}
}

func (cfg *Config) addDefaultFilteringMode() {
	if cfg.TagFilteringMode != "opt-out" {
		debug.Printf("Configured filtering mode: '%s', considering it as 'opt-in'(default)\n",
			cfg.TagFilteringMode)
		cfg.TagFilteringMode = "opt-in"
	} else {
		debug.Println("Configured filtering mode: 'opt-out'")
	}
}

func (cfg *Config) addDefaultFilter() {
	if len(strings.TrimSpace(cfg.FilterByTags)) == 0 {
		switch cfg.TagFilteringMode {
		case "opt-out":
			cfg.FilterByTags = "spot-enabled=false"
		default:
			cfg.FilterByTags = "spot-enabled=true"
		}
	}
}

func (cfg *Config) setupLogging() {
	log.SetOutput(cfg.LogFile)
	log.SetFlags(cfg.LogFlag)

	if os.Getenv("AUTOSPOTTING_DEBUG") == "true" {
		debug = log.New(cfg.LogFile, "", cfg.LogFlag)
	} else {
		debug = log.New(ioutil.Discard, "", 0)
	}

}

// processAllRegions iterates all regions in parallel, and replaces instances
// for each of the ASGs tagged with tags as specified by slice represented by cfg.FilterByTags
// by default this is all asg with the tag 'spot-enabled=true'.
func (a *AutoSpotting) processRegions(regions []string) {
	var wg sync.WaitGroup
	var savingsMutex sync.RWMutex

	for _, r := range regions {
		wg.Add(1)
		r := region{name: r, conf: a.config}
		go func() {
			s := r.calculateSavings()
			savingsMutex.Lock()
			totalSavings += s
			savingsMutex.Unlock()
			wg.Done()
		}()
	}
	wg.Wait()

	session := session.Must(session.NewSession(&aws.Config{Region: aws.String(a.config.MainRegion)}))
	ps := parameterStore{svc: ssm.New(session, aws.NewConfig().WithRegion(a.config.MainRegion))}
	mp := marketplaceMetering{
		mpSvc: marketplacemetering.New(session, aws.NewConfig().WithRegion(a.config.MainRegion)),
		ps:    ps,
		svs: savingsStats{
			ps:              ps,
			sns:             sns.New(session, aws.NewConfig().WithRegion(a.config.MainRegion)),
			topic:           a.config.NotificationSNSTopic,
			reportFrequency: a.config.SavingsReportsFrequency,
		},
	}

	log.Printf("Total hourly savings: %f", totalSavings)
	if strings.Contains(as.config.Version, "stable") {
		log.Println("Running a stable build, submitting AWS marketplace metering data")
		if err := mp.meterMarketplaceUsage(totalSavings); err != nil {
			log.Println("Failed marketplace metering, exiting... Encountered error:", err.Error())
			return
		}
	} else {
		log.Println("Not running a stable build, skipped AWS marketplace metering")
	}

	if a.config.BillingOnly {
		log.Println("Billing only mode enabled, exiting...")
		return
	}

	for _, r := range regions {
		wg.Add(1)
		r := region{name: r, conf: a.config, autospotting: a}

		go func() {
			if r.enabled() {
				log.Printf("Enabled to run in %s, processing region.\n", r.name)
				r.processRegion()
			} else {
				debug.Println("Not enabled to run in", r.name)
				debug.Println("List of enabled regions:", r.conf.Regions)
			}

			wg.Done()
		}()
	}
	wg.Wait()
}

func connectEC2(region string) *ec2.EC2 {

	sess, err := session.NewSession()
	if err != nil {
		panic(err)
	}

	return ec2.New(sess,
		aws.NewConfig().WithRegion(region))
}

// getRegions generates a list of AWS regions.
func (a *AutoSpotting) getRegions() ([]string, error) {
	var output []string

	log.Println("Scanning for available AWS regions")

	resp, err := a.mainEC2Conn.DescribeRegions(&ec2.DescribeRegionsInput{})

	if err != nil {
		log.Println(err.Error())
		return nil, err
	}

	debug.Println(resp)

	for _, r := range resp.Regions {

		if r != nil && r.RegionName != nil {
			debug.Println("Found region", *r.RegionName)
			output = append(output, *r.RegionName)
		}
	}
	return output, nil
}

// convertRawEventToCloudwatchEvent parses a raw event into a CloudWatchEvent or
// returns an error in case of failure
func (a *AutoSpotting) convertRawEventToCloudwatchEvent(event *json.RawMessage) (*events.CloudWatchEvent, *string, error) {
	var sqsEvent events.SQSEvent
	var cloudwatchEvent events.CloudWatchEvent
	var sqsReceiptHandle *string

	log.Println("Received event: \n", string(*event))
	parseEvent := *event

	// Try to parse event as an Sqs Message
	if err := json.Unmarshal(parseEvent, &sqsEvent); err != nil {
		log.Println(err.Error())
		return nil, nil, err
	}

	// If the event comes from Sqs - extract the Cloudwatch event embedded in it
	if sqsEvent.Records != nil {
		sqsRecord := sqsEvent.Records[0]
		parseEvent = []byte(sqsRecord.Body)
		// this will tell us later if the current run was triggered from SQS events
		sqsReceiptHandle = aws.String(sqsRecord.ReceiptHandle)
	}

	// Try to parse the event as Cloudwatch Event Rule
	if err := json.Unmarshal(parseEvent, &cloudwatchEvent); err != nil {
		log.Println(err.Error())
		return nil, nil, err
	}

	return &cloudwatchEvent, sqsReceiptHandle, nil
}

// parse instance events and execute the relative methods
func (a *AutoSpotting) processEventInstance(eventType string, region string, sqsReceiptHandle *string, instanceID *string, instanceState *string) error {
	if eventType == InstanceStateChangeNotificationCode {
		// If event is Instance state change
		if sqsReceiptHandle != nil {
			log.SetPrefix(fmt.Sprintf("SQS:%s ", *instanceID))
		}
		a.handleNewInstanceLaunch(region, *instanceID, *instanceState, sqsReceiptHandle)
	} else if eventType == SpotInstanceInterruptionWarningCode || eventType == InstanceRebalanceRecommendationCode {
		if eventType == InstanceRebalanceRecommendationCode && !a.config.EnableInstanceRebalanceRecommendation {
			log.Println("Handling of instance rebalance recommendation events is disabled, exiting...")
			return nil
		}
		// If the event is for an Instance Spot Interruption/Rebalance

		instanceID, err := a.replaceTerminatingSpotInstance(*instanceID, region)
		if err != nil {
			log.Printf("Error executing spot termination/rebalance action: %s\n", err.Error())
			return err
		}

		log.Printf("Finished executing spot termination/rebalance action, launched replacement instance %s\n",
			*instanceID)
	}

	return nil
}

// parse event and execute the relative methods
func (a *AutoSpotting) processEvent(event *json.RawMessage) error {
	cloudwatchEvent, sqsReceiptHandle, err := a.convertRawEventToCloudwatchEvent(event)
	if err != nil {
		log.Println("Couldn't parse event", string(*event), err.Error())
		return err
	}

	// for eventType mapping look in core/instance_events.go
	eventType, instanceID, instanceState, err := parseEventData(*cloudwatchEvent)
	if err != nil {
		log.Println("Couldn't get event details: ", err.Error())
		return err
	}

	log.Println("Triggered by", cloudwatchEvent.DetailType)
	t := time.Now()
	log.SetPrefix(fmt.Sprintf("%s:%s ", eventType, t.Format("2006-01-02T15:04:00")))

	if (eventType == InstanceStateChangeNotificationCode ||
		eventType == SpotInstanceInterruptionWarningCode ||
		eventType == InstanceRebalanceRecommendationCode) &&
		instanceID != nil {
		// Handle Instance Events
		log.SetPrefix(fmt.Sprintf("%s:%s ", eventType, *instanceID))
		a.processEventInstance(eventType, cloudwatchEvent.Region, sqsReceiptHandle, instanceID, instanceState)
	} else if eventType == AWSAPICallCloudTrailCode {
		// CloudTrail
		a.handleLifecycleHookEvent(*cloudwatchEvent)
	} else if eventType == ScheduledEventCode {
		// Cron Scheduling
		a.ProcessCronEvent()
	}

	return nil
}

// EventHandler implements the event handling logic and is the main entrypoint of
// AutoSpotting
func (a *AutoSpotting) EventHandler(event *json.RawMessage) {

	if event == nil {
		log.Println("Missing event data, running as if triggered from a cron event...")
		// Event is Autospotting Cron Scheduling
		a.ProcessCronEvent()
		return
	}

	a.processEvent(event)
	log.SetPrefix("")
}

func isValidLifecycleHookEvent(ctEvent CloudTrailEvent) bool {
	return ctEvent.EventName == "CompleteLifecycleAction" &&
		ctEvent.ErrorCode == "ValidationException" &&
		ctEvent.RequestParameters.LifecycleActionResult == "CONTINUE" &&
		strings.HasPrefix(ctEvent.ErrorMessage, "No active Lifecycle Action found with instance ID")
}

func (a *AutoSpotting) handleLifecycleHookEvent(event events.CloudWatchEvent) error {
	var ctEvent CloudTrailEvent

	// Try to parse the event.Detail as Cloudwatch Event Rule
	if err := json.Unmarshal(event.Detail, &ctEvent); err != nil {
		log.Println(err.Error())
		return err
	}
	log.Printf("CloudTrail Event data: %#v", ctEvent)

	regionName := ctEvent.AwsRegion
	instanceID := ctEvent.RequestParameters.InstanceID
	eventASGName := ctEvent.RequestParameters.AutoScalingGroupName

	if !isValidLifecycleHookEvent(ctEvent) {
		return fmt.Errorf("unexpected event: %#v", ctEvent)
	}

	r := region{name: regionName, conf: a.config, services: connections{}}

	if !r.enabled() {
		return fmt.Errorf("region %s is not enabled", r.name)
	}
	r.services.connect(regionName, r.conf.MainRegion)
	r.setupAsgFilters()
	r.scanForEnabledAutoScalingGroups()

	if err := r.scanInstance(aws.String(instanceID)); err != nil {
		log.Printf("%s Couldn't scan instance %s: %s", regionName,
			instanceID, err.Error())
		return err
	}

	i := r.instances.get(instanceID)

	if i == nil {
		log.Printf("%s Instance %s is missing, skipping...",
			regionName, instanceID)
		return errors.New("instance missing")
	}

	if i.InstanceType != nil {
		i.typeInfo = r.instanceTypeInformation[*i.InstanceType]
	}

	if skipRun, err := i.handleInstanceStates(); skipRun {
		return err
	}

	asgName := i.getReplacementTargetASGName()

	if asgName == nil || *asgName != eventASGName {
		log.Printf("event ASG name doesn't match the ASG name set on the tags " +
			"of the unattached spot instance")
		return fmt.Errorf("ASG name mismatch: event ASG name %s doesn't match the "+
			"ASG name set on the unattached spot instance %s", eventASGName, *asgName)
	}

	asg := i.region.findEnabledASGByName(*asgName)

	if asg == nil {
		log.Printf("Missing ASG data for region %s", i.region.name)
		return fmt.Errorf("region %s is missing asg data", i.region.name)
	}

	log.Printf("%s Found instance %s is not yet attached to its ASG, "+
		"attempting to swap it against a running on-demand instance",
		i.region.name, *i.InstanceId)

	return nil
}

func (a *AutoSpotting) handleNewInstanceLaunch(regionName string, instanceID string, state string, sqsReceiptHandle *string) error {
	r := &region{name: regionName, conf: a.config, services: connections{}}

	if !r.enabled() {
		return fmt.Errorf("region %s is not enabled", regionName)
	}

	r.services.connect(regionName, a.config.MainRegion)
	r.setupAsgFilters()
	r.scanForEnabledAutoScalingGroups()

	log.Println("Scanning full instance information in", r.name)
	r.determineInstanceTypeInformation(r.conf)

	if err := r.scanInstance(aws.String(instanceID)); err != nil {
		log.Printf("%s Couldn't scan instance %s: %s", regionName,
			instanceID, err.Error())
		return err
	}

	i := r.instances.get(instanceID)
	if i == nil {
		log.Printf("%s Instance %s is missing, skipping...",
			regionName, instanceID)
		return errors.New("instance missing")
	}
	log.Printf("%s Found instance %s in state %s",
		i.region.name, *i.InstanceId, *i.State.Name)

	if state != "running" {
		log.Printf("%s Instance %s is not in the running state",
			i.region.name, *i.InstanceId)
		return errors.New("instance not in running state")
	}

	// Try OnDemand
	if err := a.handleNewOnDemandInstanceLaunch(r, i, sqsReceiptHandle); err != nil {
		log.Printf("%s Instance %s couldn't be handled as on-demand instance", i.region.name, *i.InstanceId)
		return err
	}

	lifecycle := "on-demand"
	if i.isSpot() {
		lifecycle = "spot"
	}

	log.Printf("%s Instance %s with lifecycle %s couldn't be handled as on-demand instance",
		i.region.name, lifecycle, *i.InstanceId)
	return nil
}

func (a *AutoSpotting) handleNewOnDemandInstanceLaunch(r *region, i *instance, sqsReceiptHandle *string) error {
	var spotInstanceID *string
	var err error
	lifecycle := "on-demand"

	if i.InstanceLifecycle != nil {
		lifecycle = *i.InstanceLifecycle
	}
	if i.isSpot() {
		log.Printf("%s skipping Spot instance %s: only replacing new On Demand instances",
			i.region.name, *i.InstanceId)
		debug.Printf("%#v", i)
		return fmt.Errorf("instance %s is a spot instance", *i.InstanceId)
	}

	// instance is not in an enabled ASG
	belongs, asg := i.belongsToEnabledASG()
	if !belongs {
		log.Printf("Instance %s does not belong to an enabled ASG, skipping further processing", *i.InstanceId)
		return nil
	}

	if i.shouldBeTerminated(asg) {
		log.Printf("%s terminating %s instance %s: ", i.region.name, lifecycle, *i.InstanceId)
		i.terminate()
	}

	if !i.shouldBeReplacedWithNewSpotInstance() {
		log.Printf("%s skipping %s instance %s: either doesn't belong to an "+
			"enabled ASG or should not be replaced with spot, ",
			i.region.name, lifecycle, *i.InstanceId)
		debug.Printf("%#v", i)
		return nil
	}

	i.waitForBeanstalkDeployments()

	// we anyway need to delete the message, but if we delete it now,
	// the queue will allow us to process more instances in parallel
	if a.config.AllowParallelInstanceReplacements {
		i.region.sqsDeleteMessage(i.InstanceId, OnDemand, sqsReceiptHandle)
	}

	defer i.region.sqsDeleteMessage(i.InstanceId, OnDemand, sqsReceiptHandle)

	log.Printf("%s instance %s belongs to an enabled ASG and should be "+
		"replaced with spot", i.region.name, *i.InstanceId)

	log.Printf("Attempting to launch spot replacement")
	spotInstanceID, err = i.launchSpotReplacement()
	if err != nil {
		log.Printf("%s Couldn't launch spot replacement for %s: %s",
			i.region.name, *i.InstanceId, err.Error())
		return err
	}

	log.Printf("Waiting for spot instance %s to be in status running", *spotInstanceID)
	err = r.services.ec2.WaitUntilInstanceRunning(
		&ec2.DescribeInstancesInput{
			InstanceIds: []*string{spotInstanceID},
		})
	if err != nil {
		log.Printf("Issue while waiting for spot instance %v to start: %v",
			spotInstanceID, err.Error())
		return err
	}
	if err := r.scanInstance(spotInstanceID); err != nil {
		log.Printf("%s Couldn't scan instance %s: %s", i.region.name,
			*spotInstanceID, err.Error())
		return err
	}

	spotInstance := r.instances.get(*spotInstanceID)
	if _, err := spotInstance.swapWithGroupMember(i.asg, nil); err != nil {
		log.Printf("%s, couldn't perform spot replacement of %s: %s ",
			i.region.name, *i.InstanceId, err.Error())
		return err
	}
	return nil
}

func (a *AutoSpotting) replaceTerminatingSpotInstance(instanceID, regionName string) (*string, error) {
	r := &region{name: regionName, conf: a.config, services: connections{}}

	if !r.enabled() {
		return nil, fmt.Errorf("region %s is not enabled", regionName)
	}

	r.services.connect(regionName, a.config.MainRegion)
	r.setupAsgFilters()
	r.scanForEnabledAutoScalingGroups()

	log.Println("Scanning full instance information in", r.name)
	r.determineInstanceTypeInformation(r.conf)

	if err := r.scanInstance(aws.String(instanceID)); err != nil {
		log.Printf("%s Couldn't scan instance %s: %s", regionName,
			instanceID, err.Error())
		return nil, err
	}

	i := r.instances.get(instanceID)
	if i == nil {
		log.Printf("%s Instance %s is missing, skipping...",
			regionName, instanceID)
		return nil, errors.New("instance missing")
	}
	lifecycle := i.getLifecycleValue()
	log.Printf("%s Found %s instance %s in state %s",
		i.region.name, lifecycle, *i.InstanceId, *i.State.Name)

	if *i.State.Name != "running" {
		log.Printf("%s %s Instance %s is not in the running state",
			i.region.name, lifecycle, *i.InstanceId)
		return nil, errors.New("instance not in running state")
	}

	asgName := i.getReplacementTargetASGName()

	if asgName == nil {
		log.Printf("Missing the ASG name tag\n")
		return nil, errors.New("missing ASG name tag")
	}

	i.asg = i.region.findEnabledASGByName(*asgName)
	if i.asg == nil {
		log.Println("Error: Couldn't find Autoscaling Group", *asgName)
		return nil, errors.New("couldn't find ASG " + *asgName)
	}
	i.asg.name = *i.asg.AutoScalingGroupName

	i.asg.scanInstances()
	i.asg.loadDefaultConfig()
	i.asg.loadConfigFromTags()
	i.asg.loadLaunchConfiguration()
	i.asg.loadLaunchTemplate()

	i.asg.deregisterInstanceFromLoadBalancersAndECSClusters(i)

	newInstanceID, err := i.launchSpotReplacement()

	if err != nil {
		log.Printf("Spot Instance launch failed while replacing %s, error: %s, falling back to on-demand\n", *i.InstanceId, err.Error())

		newInstanceID, err = i.launchOnDemandReplacement()
		if err != nil {
			log.Printf("Instance launch failed while replacing %s, error: %s\n", *i.InstanceId, err.Error())
			return nil, err
		}
		log.Printf("Successfully launched On demand instance %s", *newInstanceID)
	} else {
		log.Printf("Successfully launched Spot instance %s", *newInstanceID)
	}

	i.region.scanInstances()
	newInstance := i.region.instances.get(*newInstanceID)

	existingLifecycle := i.getLifecycleValue()
	newLifecycle := newInstance.getLifecycleValue()

	log.Printf("Swapping existing %s instance %s with new %s instance %s",
		existingLifecycle, *i.InstanceId, newLifecycle, *newInstanceID)
	newInstance.swapWithGroupMember(i.asg, nil)

	if err = i.asg.waitForInstanceStatus(newInstanceID, "InService", newLifecycle, 80); err != nil {
		log.Printf("%s instance %s is still not InService, trying to terminate it.",
			newLifecycle, *newInstanceID)
		newInstance.terminate()
	}

	return newInstanceID, nil
}
