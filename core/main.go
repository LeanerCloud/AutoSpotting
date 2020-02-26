// Copyright (c) 2016-2019 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"sync"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
)

var logger, debug *log.Logger

var hourlySavings float64
var savingsMutex = &sync.RWMutex{}

// Run is the entry point of autospotting.
func Run(cfg *Config) {
	logger = cfg.logger
	debug = cfg.debug

	debug.Println(*cfg)

	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		lambda.Start(handler(cfg))
	} else {
		start(cfg)
	}
}

// start processes all AWS regions looking for AutoScaling groups
// enabled and taking action by replacing more pricy on-demand instances with
// compatible and cheaper spot instances.
func start(cfg *Config) {
	// use this only to list all the other regions
	ec2Conn := connectEC2(cfg.MainRegion)
	allRegions, err := getRegions(ec2Conn)

	if err != nil {
		logger.Println(err.Error())
		return
	}

	processRegions(allRegions, cfg)
}

// processAllRegions iterates all regions in parallel, and replaces instances
// for each of the ASGs tagged with tags as specified by slice represented by cfg.FilterByTags
// by default this is all asg with the tag 'spot-enabled=true'.
func processRegions(regions []string, cfg *Config) {

	var wg sync.WaitGroup

	for _, r := range regions {

		wg.Add(1)
		r := region{name: r, conf: cfg}

		go func() {

			if r.enabled() {
				logger.Printf("Enabled to run in %s, processing region.\n", r.name)
				r.processRegion()
			} else {
				debug.Println("Not enabled to run in", r.name)
				debug.Println("List of enabled regions:", cfg.Regions)
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
func getRegions(ec2conn ec2iface.EC2API) ([]string, error) {
	var output []string

	logger.Println("Scanning for available AWS regions")

	resp, err := ec2conn.DescribeRegions(&ec2.DescribeRegionsInput{})

	if err != nil {
		logger.Println(err.Error())
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

// handler returns an AWS Lambda handler given a config.
func handler(conf *Config) func(context.Context, json.RawMessage) error {
	return func(ctx context.Context, rawEvent json.RawMessage) error {

		var snsEvent events.SNSEvent
		var cloudwatchEvent events.CloudWatchEvent
		parseEvent := rawEvent

		// Try to parse event as an SNS Message
		if err := json.Unmarshal(parseEvent, &snsEvent); err != nil {
			log.Println(err.Error())
			return err
		}

		// If event is from SNS - extract Cloudwatch's one
		if snsEvent.Records != nil {
			snsRecord := snsEvent.Records[0]
			parseEvent = []byte(snsRecord.SNS.Message)
		}

		// Try to parse event as Cloudwatch Event Rule
		if err := json.Unmarshal(parseEvent, &cloudwatchEvent); err != nil {
			log.Println(err.Error())
			return err
		}

		// If event is Instance Spot Interruption
		if cloudwatchEvent.DetailType != "EC2 Spot Instance Interruption Warning" {
			// Event is Autospotting Cron Scheduling
			start(conf)
			return nil
		}

		instanceID, err := GetInstanceIDDueForTermination(cloudwatchEvent)
		if err != nil || instanceID == nil {
			log.Println("Couldn't get ID of instance due for termination", err.Error())
			return err
		}

		spotTermination := NewSpotTermination(cloudwatchEvent.Region)
		if !spotTermination.IsInAutoSpottingASG(instanceID, conf.TagFilteringMode, conf.FilterByTags) {
			log.Printf("Instance %s is not in AutoSpotting ASG\n", *instanceID)
			return nil
		}

		err = spotTermination.ExecuteAction(instanceID, conf.TerminationNotificationAction)
		if err != nil {
			log.Printf("Error executing spot termination action: %s\n", err.Error())
			return err
		}

		return nil
	}
}
