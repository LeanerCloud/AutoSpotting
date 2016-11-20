package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	autospotting "github.com/cristim/autospotting/core"
	lambda "github.com/eawsy/aws-lambda-go/service/lambda/runtime"
)

type cfgData struct {
	autospotting.Config
}

var conf *cfgData

func main() {
	run()
}

func run() {
	fmt.Printf("Starting autospotting agent, build %s", conf.BuildNumber)
	autospotting.Run(conf.Config)
	fmt.Println("Execution completed, nothing left to do")
}

// this is the equivalent of a main for when running from Lambda, but on Lambda the
// run() is executed within the handler function every time we have an event
func init() {

	conf = &cfgData{
		autospotting.Config{
			LogFile: os.Stdout,
			LogFlag: log.Lshortfile,
		},
	}

	conf.initialize()

	lambda.HandleFunc(handle)
}

func handle(evt json.RawMessage, ctx *lambda.Context) (interface{}, error) {
	run()
	return nil, nil
}

// Configuration handling
func (c *cfgData) initialize() {

	build, instanceInfo := readAssets()

	c.parseCommandLineFlags()
	c.BuildNumber = string(build)

	err := c.RawInstanceData.LoadFromAssetContent(instanceInfo)
	if err != nil {
		log.Fatal(err.Error())
	}
}

func (c *cfgData) parseCommandLineFlags() {

	flag.StringVar(&c.Regions, "regions", "", "Regions(comma separated list)"+
		"where it should run, by default runs on all regions")

	// flag.StringVar(&cfg.Regions, "region", "", "Regions(comma separated list)"+
	//    "where it should run, by default runs on all regions")

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
