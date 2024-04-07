// Copyright (c) 2016-2022 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"context"
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
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/aws/aws-sdk-go-v2/service/marketplacemetering"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

var debug *log.Logger
var totalSavings float64

// AutoSpotting hosts global configuration and has as methods all the public
// entrypoints of this library
type AutoSpotting struct {
	config      *Config
	mainEC2Conn EC2API
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
	var totalSavings float64

	for _, r := range regions {
		wg.Add(1)
		r := region{name: r, conf: a.config, services: &connections{}}
		go func(r *region) {
			s := r.calculateSavings()
			savingsMutex.Lock()
			totalSavings += s
			savingsMutex.Unlock()
			wg.Done()
		}(&r)
	}
	wg.Wait()

	ctx := context.TODO()
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(a.config.MainRegion))
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	mpCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-1"))
	if err != nil {
		log.Fatalf("unable to load Marketplace SDK config, %v", err)
	}

	ssmClient := ssm.NewFromConfig(cfg)
	ps := parameterStore{svc: ssmClient}
	mpSvc := marketplacemetering.NewFromConfig(mpCfg)
	snsClient := sns.NewFromConfig(cfg)

	accountID, err := getAccountID(os.Getenv("NOTIFICATION_SNS_TOPIC"), os.Getenv("SQS_QUEUE_URL"))
	if err != nil {

		log.Printf("Couldn't get account ID, error: %s", err.Error())
	} else {
		log.Printf("Account ID: %s", *accountID)
	}

	mp := marketplaceMetering{
		mpSvc: *mpSvc,
		ps:    ps,
		svs: savingsStats{
			ps:              ps,
			sns:             snsClient,
			topic:           a.config.NotificationSNSTopic,
			reportFrequency: a.config.SavingsReportsFrequency,
		},
		accountID: accountID,
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
		r := region{name: r, conf: a.config,
			autospotting: a, services: &connections{}}

		go func() {
			if r.enabled() {
				debug.Printf("Enabled to run in %s, processing region.\n", r.name)
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

func connectEC2(region string) *ec2.Client {
	ctx := context.TODO()
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		panic(err)
	}

	return ec2.NewFromConfig(cfg)
}

// getRegions generates a list of AWS regions.
func (a *AutoSpotting) getRegions() ([]string, error) {
	var output []string

	log.Println("Scanning for available AWS regions")

	resp, err := a.mainEC2Conn.DescribeRegions(context.TODO(), &ec2.DescribeRegionsInput{})

	if err != nil {
		log.Println(err.Error())
		return nil, err
	}

	debug.Println(resp)

	for _, r := range resp.Regions {

		if r.RegionName != nil {
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
		err := a.handleNewInstanceLaunch(region, *instanceID, *instanceState, sqsReceiptHandle)
		if err != nil {
			log.Printf("Error Handling new instance launch for instance %s: %s", *instanceID, err.Error())
			return err
		}
	} else if eventType == SpotInstanceInterruptionWarningCode || eventType == InstanceRebalanceRecommendationCode {
		if eventType == InstanceRebalanceRecommendationCode && !a.config.EnableInstanceRebalanceRecommendation {
			log.Println("Handling of instance rebalance recommendation events is disabled, exiting...")
			return nil
		}
		// If the event is for an Instance Spot Interruption/Rebalance

		instanceID, err := a.replaceTerminatingSpotInstance(*instanceID, region, sqsReceiptHandle)
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
		if err := a.processEventInstance(eventType, cloudwatchEvent.Region, sqsReceiptHandle, instanceID, instanceState); err != nil {
			log.Printf("Error processing event instance for eventType %s in region %s with instance ID %s: %s",
				eventType, cloudwatchEvent.Region, *instanceID, err.Error())

			return fmt.Errorf("error processing event instance for eventType %s in region %s with instance ID %s: %w",
				eventType, cloudwatchEvent.Region, *instanceID, err)
		}
	} else if eventType == AWSAPICallCloudTrailCode {
		// CloudTrail
		if err := a.handleLifecycleHookEvent(*cloudwatchEvent, sqsReceiptHandle); err != nil {
			log.Printf("Error handling lifecycle hook event: %s", err)
			return fmt.Errorf("error handling lifecycle hook event: %w", err)
		}
	} else if eventType == ScheduledEventCode {
		// Cron Scheduling
		log.Println("Handling as cron event")
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

	if err := a.processEvent(event); err != nil {
		log.Printf("Error processing event: %s", err.Error())
	}
	log.SetPrefix("")
}

func isValidLifecycleHookEvent(ctEvent CloudTrailEvent) bool {
	return ctEvent.EventName == "CompleteLifecycleAction" &&
		ctEvent.ErrorCode == "ValidationException" &&
		ctEvent.RequestParameters.LifecycleActionResult == "CONTINUE" &&
		strings.HasPrefix(ctEvent.ErrorMessage, "No active Lifecycle Action found with instance ID")
}

func (a *AutoSpotting) handleLifecycleHookEvent(event events.CloudWatchEvent, sqsReceiptHandle *string) error {
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

	r := region{name: regionName, conf: a.config, services: &connections{}}

	if !r.enabled() {
		return fmt.Errorf("region %s is not enabled", r.name)
	}
	if err := r.services.connect(regionName, r.conf.MainRegion); err != nil {
		log.Printf("Error connecting to services in region %s with main region %s: %s",
			regionName, r.conf.MainRegion, err.Error())
		return fmt.Errorf("error connecting to services in region %s with main region %s: %w",
			regionName, r.conf.MainRegion, err)
	}

	if sqsReceiptHandle != nil {
		log.Println("Trying to delete message from SQS queue")
		if err := r.sqsDeleteMessage("lifecycle hook", &instanceID, Spot, sqsReceiptHandle); err != nil {
			log.Printf("Error deleting SQS message: %v", err)
		}
	} else {
		log.Printf("Failed to delete message from SQS queue, missing SQS receipt handle")
	}

	r.setupAsgFilters()
	r.scanForEnabledAutoScalingGroups()

	if err := r.scanInstance(instanceID); err != nil {
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

	if i.InstanceType != "" {
		i.typeInfo = r.instanceTypeInformation[string(i.InstanceType)]
	}

	if skipRun, err := i.handleInstanceStates(); skipRun {
		return err
	}

	belongs, asgName := i.belongsToAnASG()

	if !belongs || asgName == nil || *asgName != eventASGName {
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
	r := &region{name: regionName, conf: a.config, services: &connections{}}

	if !r.enabled() {
		return fmt.Errorf("region %s is not enabled", regionName)
	}

	if err := r.services.connect(regionName, a.config.MainRegion); err != nil {
		log.Printf("Error connecting to services in region %s with main region %s: %s",
			regionName, a.config.MainRegion, err.Error())

		return fmt.Errorf("error connecting to services in region %s with main region %s: %w",
			regionName, a.config.MainRegion, err)
	}

	// we anyway need to delete the message, but if we delete it now,
	// the queue will allow us to process more instances in parallel

	if sqsReceiptHandle != nil {
		log.Println("Trying to delete message from SQS queue")
		if err := r.sqsDeleteMessage("new instance launch", &instanceID, OnDemand, sqsReceiptHandle); err != nil {
			log.Printf("Error deleting SQS message: %v", err)
		}
	} else {
		log.Println("Failed to delete message from SQS queue, missing SQS receipt handle")
	}

	r.setupAsgFilters()
	r.scanForEnabledAutoScalingGroups()

	log.Println("Scanning full instance information in", r.name)
	r.determineInstanceTypeInformation(r.conf)

	if err := r.scanInstance(instanceID); err != nil {
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
		i.region.name, *i.InstanceId, i.State.Name)

	if state != "running" {
		log.Printf("%s Instance %s is not in the running state",
			i.region.name, *i.InstanceId)
		return errors.New("instance not in running state")
	}

	// Try OnDemand
	if err := a.handleNewOnDemandInstanceLaunch(r, i); err != nil {
		log.Printf("%s Instance %s couldn't be handled as on-demand instance, encountered error: %s",
			i.region.name, *i.InstanceId, err.Error())
		return fmt.Errorf("error handling new on-demand instance: %w", err)
	}

	lifecycle := "on-demand"
	if i.isSpot() {
		lifecycle = "spot"
	}

	log.Printf("%s Instance %s with lifecycle %s couldn't be handled as on-demand instance",
		i.region.name, lifecycle, *i.InstanceId)
	return nil
}

func (a *AutoSpotting) handleNewOnDemandInstanceLaunch(r *region, i *instance) error {
	var spotInstanceID *string
	var err error
	lifecycle := "on-demand"

	log.Printf("Handling instance launch as on-demand for instance %s", *i.InstanceId)

	if i.InstanceLifecycle != "" {
		lifecycle = string(i.InstanceLifecycle)
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

	if eligible, reason := asg.isASGElegibleForProcessing(); !eligible {
		log.Printf("On Demand instance %s belonging to ASG %s is not eligible for further processing, reason: %s",
			*i.InstanceId, asg.name, reason)
		return nil
	}

	if term, detach := i.shouldBeTerminated(asg); term {
		if detach {
			log.Printf("%s terminating %s instance %s, detaching it from the ASG: ", i.region.name, lifecycle, *i.InstanceId)
			if err := asg.terminateInstanceInAutoScalingGroup(i, false, true); err != nil {
				return fmt.Errorf("error terminating instance %s in auto scaling group %s: %w", *i.InstanceId, asg.name, err)
			}
		} else {
			log.Printf("%s terminating %s instance %s without detaching it from the ASG: ", i.region.name, lifecycle, *i.InstanceId)
			if err := i.terminate(); err != nil {
				return fmt.Errorf("error terminating instance %s in region %s: %w",
					*i.InstanceId, i.region.name, err)
			}
		}
		return nil
	}

	if !i.shouldBeReplacedWithNewSpotInstance() {
		log.Printf("%s skipping %s instance %s: either doesn't belong to an "+
			"enabled ASG or should not be replaced with spot, ",
			i.region.name, lifecycle, *i.InstanceId)
		debug.Printf("%#v", i)
		return nil
	}

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

	if err := r.waitForInstanceRunning(spotInstanceID); err != nil {
		log.Printf("Issue while waiting for spot instance %v to start: %v",
			spotInstanceID, err.Error())
		return err
	}

	if err := r.scanInstance(*spotInstanceID); err != nil {
		log.Printf("%s Couldn't scan instance %s: %s", i.region.name,
			*spotInstanceID, err.Error())
		return err
	}

	log.Printf("Swapping new instance %s against existing instance %s",
		*spotInstanceID, *i.InstanceId)

	spotInstance := r.instances.get(*spotInstanceID)
	if spotInstance == nil {
		log.Printf("Couldn't get instance info for Spot instance %s, terminating it...", *spotInstanceID)
		if err := r.terminateInstanceByID(*spotInstanceID); err != nil {
			log.Printf("Couldn't terminate instance %s", *spotInstanceID)
		}

		return fmt.Errorf("couldn't get instance info for Spot instance %s", *spotInstanceID)
	}

	if _, err := spotInstance.swapWithGroupMember(i.asg); err != nil {
		log.Printf("%s, couldn't perform spot replacement of %s: %s ",
			i.region.name, *i.InstanceId, err.Error())
		return err
	}
	return nil
}

func (a *AutoSpotting) replaceTerminatingSpotInstance(instanceID, regionName string, sqsReceiptHandle *string) (*string, error) {
	r := &region{name: regionName, conf: a.config, services: &connections{}}

	if !r.enabled() {
		return nil, fmt.Errorf("region %s is not enabled", regionName)
	}

	if err := r.services.connect(regionName, a.config.MainRegion); err != nil {
		log.Printf("Error connecting to services in region %s with main region %s: %s",
			regionName, a.config.MainRegion, err.Error())
		return nil, fmt.Errorf("error connecting to services in region %s with main region %s: %w",
			regionName, a.config.MainRegion, err)
	}

	// we anyway need to delete the message, but if we delete it as early as possible,
	// the queue will allow us to process more events in parallel
	if sqsReceiptHandle != nil {
		log.Printf("%s Trying to delete message from SQS queue when processing terminating Spo instance %s", r.name, instanceID)
		if err := r.sqsDeleteMessage("Spot termination", &instanceID, Spot, sqsReceiptHandle); err != nil {
			log.Printf("Error deleting SQS message: %v", err)
		}
	} else {
		log.Printf("Failed to delete message from SQS queue when processing main region %s, missing SQS receipt handle", r.name)
	}

	r.setupAsgFilters()
	r.scanForEnabledAutoScalingGroups()

	log.Println("Scanning full instance information in", r.name)
	r.determineInstanceTypeInformation(r.conf)

	if err := r.scanInstance(instanceID); err != nil {
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
		i.region.name, lifecycle, *i.InstanceId, i.State.Name)

	if (i.State.Name != "running") && (i.State.Name != "shutting-down") {
		log.Printf("%s %s Instance %s is not in the running or shutting down state",
			i.region.name, lifecycle, *i.InstanceId)
		return nil, errors.New("instance not in running/shutting down state")
	}

	belongs, asgName := i.belongsToAnASG()
	if !belongs || asgName == nil {
		log.Printf("Instance %s does not belong to ASG", *i.InstanceId)
		return nil, errors.New("instance %s does not belong to ASG")
	}

	i.asg = i.region.findEnabledASGByName(*asgName)
	if i.asg == nil {
		log.Println("Error: Couldn't find Autoscaling Group", *asgName)
		return nil, errors.New("couldn't find ASG " + *asgName)
	}
	i.asg.name = *i.asg.AutoScalingGroupName
	desiredCapacity := *i.asg.DesiredCapacity

	log.Printf("ASG %s, Desired capacity: %d", i.asg.name, desiredCapacity)

	i.asg.scanInstances()
	i.asg.loadDefaultConfig()
	i.asg.loadConfigFromTags()
	if _, err := i.asg.loadLaunchConfiguration(); err != nil {
		log.Printf("Error loading launch configuration: %v", err)
	}
	if _, err := i.asg.loadLaunchTemplate(); err != nil {
		log.Printf("Error loading launch template: %v", err)
	}

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

	log.Printf("Waiting for new instance %s to be in status running", *newInstanceID)
	err = r.waitForInstanceRunning(newInstanceID)
	if err != nil {
		log.Printf("Issue while waiting for new instance %v to start: %v",
			newInstanceID, err.Error())
		return nil, err
	}

	if err := i.region.scanInstances(); err != nil {
		log.Printf("error scanning instances in region %s: %s", i.region.name, err.Error())
		return nil, fmt.Errorf("error scanning instances in region %s: %w", i.region.name, err)
	}

	if err := r.scanInstance(*newInstanceID); err != nil {
		log.Printf("%s Couldn't scan instance %s: %s", i.region.name,
			*newInstanceID, err.Error())
		return nil, err
	}
	newInstance := i.region.instances.get(*newInstanceID)

	log.Printf("Swapping new instance %s against existing instance %s",
		*newInstanceID, *i.InstanceId)

	existingLifecycle := i.getLifecycleValue()
	newLifecycle := newInstance.getLifecycleValue()

	log.Printf("Swapping existing %s instance %s with new %s instance %s",
		existingLifecycle, *i.InstanceId, newLifecycle, *newInstanceID)

	if _, err := newInstance.swapWithGroupMember(i.asg); err != nil {
		log.Printf("Error swapping instance %s with a memberof ASG %s: %s",
			*newInstance.InstanceId, *asgName, err.Error())
		return nil, err
	}
	if err = i.asg.waitForInstanceStatus(newInstanceID, "InService", newLifecycle, 80); err != nil {
		log.Printf("%s instance %s is still not InService, trying to terminate it.",
			newLifecycle, *newInstanceID)
		err := newInstance.terminate()
		if err != nil {
			log.Printf("Error terminating instance: %v", err)
		}
	}

	return newInstanceID, nil
}

// parse the account ID from the ARN of NOTIFICATION_SNS_TOPIC or SQS_QUEUE_URL environment variables
func getAccountID(topicARN, queueURL string) (*string, error) {

	if topicARN != "" {
		// Split the ARN string by ":" and check if it has the correct format
		parts := strings.Split(topicARN, ":")
		if len(parts) < 6 {
			return nil, errors.New("invalid ARN format")
		}
		// The account ID is the 5th element in the split ARN (0-based indexing)
		return &(parts[4]), nil
	}

	// If ARN is not found, try to get the SQS queue URL from SQS_QUEUE_URL environment variable
	if queueURL != "" {
		// Split the URL by "/" and check if it has the correct format
		parts := strings.Split(queueURL, "/")
		if len(parts) < 5 {
			return nil, errors.New("invalid SQS queue URL format")
		}
		// The account ID is the 4th element in the split URL (0-based indexing)
		return &(parts[3]), nil
	}

	// If neither environment variable is set, return an error
	return nil, errors.New("neither ARN nor SQS queue URL found in environment variables")
}
