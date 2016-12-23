package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"

	autospotting "github.com/cristim/autospotting/core"
	"github.com/eawsy/aws-lambda-go/service/lambda/runtime"
)

type cfgData struct {
	autospotting.Config
}

var conf *cfgData

func main() {
	run()
}

func run() {
	log.Print("Starting autospotting agent, build", conf.BuildNumber)
	autospotting.Run(conf.Config)
	log.Println("Execution completed, nothing left to do")
}

// this is the equivalent of a main for when running from Lambda, but on Lambda the
// run() is executed within the handler function every time we have an event
func init() {

	conf = &cfgData{
		autospotting.Config{
			LogFile: os.Stdout,
			LogFlag: log.Ldate | log.Ltime | log.Lshortfile,
		},
	}

	conf.initialize()

	runtime.HandleFunc(handle)
}

func handle(evt json.RawMessage, ctx *runtime.Context) (interface{}, error) {
	conf.applyCloudWatchEventConfigs(evt)
	run()
	return nil, nil
}

// Configuration handling
func (c *cfgData) initialize() {

	build, instanceInfo := readAssets()

	c.parseCommandLineFlags()
	c.BuildNumber = string(build)

	log.Println("Current Configuration", c)

	err := c.RawInstanceData.LoadFromAssetContent(instanceInfo)
	if err != nil {
		log.Fatal(err.Error())
	}
}

func (c *cfgData) parseCommandLineFlags() {
	flag.StringVar(&c.Regions, "regions", "", "Regions (comma separated list)"+
		"where it should run, by default runs on all regions")
	flag.Int64Var(&c.MinOnDemandNumber, "minOnDemandNumber", 0, "Minimum "+
		"on-demand instances (number) running in ASG.")
	flag.Float64Var(&c.MinOnDemandPercentage, "minOnDemandPercentage", 0.0,
		"Minimum on-demand instances (percentage) running in ASG.")
	flag.Parse()
	log.Println("Parsed command line flags")
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

func (c *cfgData) applyCloudWatchEventConfigs(evt json.RawMessage) {

	var eventOptions autospotting.EventOptions

	err := json.Unmarshal(evt, &eventOptions)

	if err != nil {
		log.Println("Error parsing CloudWatch event data:", err)
		return
	}

	log.Printf("Received CloudWatch event %v\n", eventOptions)
	c.EventOptions = eventOptions
}
