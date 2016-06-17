package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/service/sns"
)

type InstanceEvent struct {
	EventType    string // example: termination | startup
	InstanceID   string // 					i-d34db33f
	InstanceType string //					m3.medium
	Region       string // 					us-east-1
}

var metadata_client *ec2metadata.Client

func main() {
	fmt.Println("Starting Spot termination notification agent...")

	var topic string
	var loop_sleep_time, initial_wait int
	metadata_client = ec2metadata.New(&ec2metadata.Config{})
	flag.StringVar(&topic, "topic", "", "an SNS topic where we will notify in case the instance is approaching termination")
	flag.IntVar(&loop_sleep_time, "loop-sleep-time", 5, "duration in seconds between the iterations of the SPOT termination polling")
	flag.IntVar(&initial_wait, "initial-wait", 60, "number of minutes to wait until the machine is considered ready to replace an on-demand instance, which will then be terminated")
	flag.Parse()

	if topic == "" {
		fmt.Println("Missing topic, exiting...\nuse --help to see how to run this program")
		os.Exit(1)
	}

	// this will only notify if the instance won't terminate yet by the time the initial_wait expired
	go deferred_startup_notification(topic, initial_wait)

	for { // polling for spot termination time
		time.Sleep(time.Duration(loop_sleep_time) * time.Second)

		termination := terminating()

		if termination {
			err := notify(topic, "imminent-termination")
			if err != nil {
				fmt.Println("couldn't notify yet, retrying soon...")
			} else {
				fmt.Println("successfully notified, exiting...")
				break
			}
		} else {
			fmt.Println("not terminating yet, retrying soon...")
		}
	}
}

func deferred_startup_notification(topic string, wait int) {
	time.Sleep(time.Duration(wait) * time.Minute)
	notify(topic, "started")
}

func terminating() bool {
	_, err := metadata_client.GetMetadata("spot/termination-time")
	return (err == nil)
}

func notify(topic, event_type string) error {
	r, err := metadata_client.Region()
	if err != nil {
		fmt.Println("couldn't get AWS region")
		return err
	}

	instance_type, err := metadata_client.GetMetadata("instance-type")
	if err != nil {
		fmt.Println("Couldn't get instance type")
		return err
	}

	instance_id, err := metadata_client.GetMetadata("instance-id")
	if err != nil {
		fmt.Println("Couldn't get instance ID")
		return err
	}

	client := sns.New(&aws.Config{Region: &r})

	instance_event := InstanceEvent{event_type, instance_id, instance_type, r}
	instance_event_json, _ := json.Marshal(instance_event)
	message := string(instance_event_json)

	fmt.Println("Notifying, message: ", message)
	params := &sns.PublishInput{
		Message:  aws.String(message), // Required
		TopicArn: aws.String(topic),
	}
	_, err = client.Publish(params)

	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			// Generic AWS error with Code, Message, and original error (if any)
			fmt.Println(awsErr.Code(), awsErr.Message(), awsErr.OrigErr())
			if reqErr, ok := err.(awserr.RequestFailure); ok {
				// A service error occurred
				fmt.Println(reqErr.Code(), reqErr.Message(), reqErr.StatusCode(), reqErr.RequestID())
			}
		} else {
			// This case should never be hit, the SDK should always return an
			// error which satisfies the awserr.Error interface.
			fmt.Println(err.Error())
		}
	}
	return err
}
