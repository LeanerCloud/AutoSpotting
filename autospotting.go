package main

import (
	"fmt"

	"github.com/cristim/autospotting/core"
	"github.com/droundy/goopt"
)

// Main intefaces with the lambda event handler wrapper written
// in javascript. The wrapper only dumps the event data to a couple
// of temporary files, and their names are then passed as command
// line arguments.
// In main we read them and pass the content we read to a processing
// function which then parses it, then takes action depending on
// that content.
func main() {
	fmt.Println("Starting agent...")

	eventFileName, contextFileName := parseCommandLineArguments()

	run(eventFileName, contextFileName, cronTopic, instancesURL)

	fmt.Println("Exiting main, nothing left to do")
}

func run(eventFileName, contextFileName, cronTopic, instancesURL string) {
	lambdaEvent := autospotting.LambdaEventFromFiles{
		EventFile:   eventFileName,
		ContextFile: contextFileName,
	}

	lambdaEvent.HandleEvent(cronTopic, instancesURL)
}

func parseCommandLineArguments() (string, string) {

	eventFile := goopt.String([]string{"-e", "--event_file"}, "/tmp/event.json", "a file name used as input for the event data")

	contextFile := goopt.String([]string{"-c", "--context_file"}, "/tmp/context.json", "a file name used as input for the context data")

	goopt.Description = func() string {
		return "Program that automates the use of AWS EC2 spot instances on existing autoscaling groups"
	}

	goopt.Version = "0.0.1"
	goopt.Summary = "spot instance automation tool"
	goopt.Parse(nil)

	return *eventFile, *contextFile
}
