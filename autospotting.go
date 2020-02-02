// Copyright (c) 2016-2019 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package main

import (
	"context"
	"encoding/json"
	"log"
	"os"

	autospotting "github.com/AutoSpotting/AutoSpotting/core"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

var conf autospotting.Config

// Version represents the build version being used
var Version = "number missing"

func main() {
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		lambda.Start(Handler)
	} else {
		run()
	}
}

func run() {

	log.Println("Starting autospotting agent, build", Version)
	log.Printf("Configuration flags: %#v", conf)

	autospotting.Run(&conf)
	log.Println("Execution completed, nothing left to do")
}

// this is the equivalent of a main for when running from Lambda, but on Lambda
// the run() is executed within the handler function every time we have an event
func init() {
	conf = autospotting.Config{
		Version: Version,
	}
	autospotting.ParseConfig(&conf)
}

// Handler implements the AWS Lambda handler
func Handler(ctx context.Context, rawEvent json.RawMessage) {

	var snsEvent events.SNSEvent
	var cloudwatchEvent events.CloudWatchEvent
	parseEvent := rawEvent

	// Try to parse event as an Sns Message
	if err := json.Unmarshal(parseEvent, &snsEvent); err != nil {
		log.Println(err.Error())
		return
	}

	// If event is from Sns - extract Cloudwatch's one
	if snsEvent.Records != nil {
		snsRecord := snsEvent.Records[0]
		parseEvent = []byte(snsRecord.SNS.Message)
	}

	// Try to parse event as Cloudwatch Event Rule
	if err := json.Unmarshal(parseEvent, &cloudwatchEvent); err != nil {
		log.Println(err.Error())
		return
	}

	// If event is Instance Spot Interruption
	if cloudwatchEvent.DetailType == "EC2 Spot Instance Interruption Warning" {
		instanceID, err := autospotting.GetInstanceIDDueForTermination(cloudwatchEvent)
		if err != nil || instanceID == nil {
			return
		}

		spotTermination := autospotting.NewSpotTermination(cloudwatchEvent.Region)
		if spotTermination.IsInAutoSpottingASG(instanceID, conf.TagFilteringMode, conf.FilterByTags) {
			err := spotTermination.ExecuteAction(instanceID, conf.TerminationNotificationAction)
			if err != nil {
				log.Printf("Error executing spot termination action: %s\n", err.Error())
			}
		} else {
			log.Printf("Instance %s is not in AutoSpotting ASG\n", *instanceID)
			return
		}
	} else {
		// Event is Autospotting Cron Scheduling
		run()
	}
}
