package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/cristim/autospotting/core"
	"github.com/cristim/autospotting/ec2instancesinfo"
	"github.com/eawsy/aws-lambda-go-core/service/lambda/runtime"
	"github.com/namsral/flag"
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

// this is the equivalent of a main for when running from Lambda, but on Lambda
// the run() is executed within the handler function every time we have an event
func init() {

	conf = &cfgData{
		autospotting.Config{
			LogFile: os.Stdout,
			LogFlag: log.Ldate | log.Ltime | log.Lshortfile,
		},
	}

	conf.initialize()

}

// Handle implements the AWS Lambda handler
func Handle(evt json.RawMessage, ctx *runtime.Context) (interface{}, error) {
	run()
	return nil, nil
}

// Configuration handling
func (c *cfgData) initialize() {

	c.parseCommandLineFlags()
	//c.BuildNumber = build

	log.Printf("Current Configuration: %+v\n", c)

	data, err := ec2instancesinfo.Data()
	if err != nil {
		log.Fatal(err.Error())
	}
	c.InstanceData = data
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
