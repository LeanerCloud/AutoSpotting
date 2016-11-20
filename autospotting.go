package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	autospotting "github.com/cristim/autospotting/core"
	lambda "github.com/eawsy/aws-lambda-go/service/lambda/runtime"
)

var cfg autospotting.Config

func main() {
	prepareConfig()
	run()
}

// this is the equivalent of a main for when running from Lambda, but on Lambda the
// run() is executed within the handler function every time we have an event
func init() {
	prepareConfig()
	lambda.HandleFunc(handle)
}

func handle(evt json.RawMessage, ctx *lambda.Context) (interface{}, error) {
	run()
	return nil, nil
}

func run() {
	fmt.Printf("Starting autospotting agent, build %s", cfg.BuildNumber)
	autospotting.Run(cfg)
	fmt.Println("Execution completed, nothing left to do")
}

func prepareConfig() {
	build, instanceInfo := readAssets()

	cfg = parseCommandLineFlags()
	cfg.BuildNumber = string(build)

	err := cfg.RawInstanceData.LoadFromAssetContent(instanceInfo)
	if err != nil {
		log.Fatal(err.Error())
	}
}

func readAssets() (string, []byte) {

	// contains the build number
	build, err := Asset("data/BUILD")
	if err != nil {
		log.Fatal(err.Error())
	}

	instanceInfo, err := Asset("data/instances.json")
	if err != nil {
		log.Fatal(err.Error())
	}

	return string(build), instanceInfo
}

func parseCommandLineFlags() autospotting.Config {

	cfg := autospotting.Config{
		LogFile: os.Stdout,
		LogFlag: log.Lshortfile,
	}

	return cfg
}
