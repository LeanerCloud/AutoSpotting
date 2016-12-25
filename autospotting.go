package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/namsral/flag"

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
	run()
	return nil, nil
}

// Configuration handling
func (c *cfgData) initialize() {

	build, instanceInfo := readAssets()

	c.parseCommandLineFlags()
	c.BuildNumber = string(build)

	log.Printf("Current Configuration: %+v\n", c)

	err := c.RawInstanceData.LoadFromAssetContent(instanceInfo)
	if err != nil {
		log.Fatal(err.Error())
	}
}

func (c *cfgData) parseCommandLineFlags() {

	flag.StringVar(&c.Regions, "regions", "",
		"Regions where it should be activated (comma or whitespace separated list, "+
			"also supports globs), by default it runs on all regions.\n\t"+
			"Example: ./autospotting -regions 'eu-*,us-east-1'")

	flag.Int64Var(&c.MinOnDemandNumber, "min_on_demand_number", 0,
		"On-demand capacity (as absolute number) ensured to be running in each of your groups.\n\t"+
			"Can be overridden on a per-group basis using the tag "+
			autospotting.OnDemandNumberLong)

	flag.Float64Var(&c.MinOnDemandPercentage, "min_on_demand_percentage", 0.0,
		"On-demand capacity (percentage of the total number of instances in the group) "+
			"ensured to be running in each of your groups.\n\t"+
			"Can be overridden on a per-group basis using the tag "+
			autospotting.OnDemandPercentageLong+
			"\n\tIt is ignored if min_on_demand_number is also set.")

	flag.Parse()
	log.Printf("Parsed command line flags: regions='%s' min_on_demand_number=%d min_on_demand_percentage=%.1f",
		c.Regions, c.MinOnDemandNumber, c.MinOnDemandPercentage)
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
