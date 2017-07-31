package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/cristim/autospotting/core"
	"github.com/cristim/ec2-instances-info"
	"github.com/eawsy/aws-lambda-go-core/service/lambda/runtime"
	"github.com/namsral/flag"
)

type cfgData struct {
	*autospotting.Config
}

var conf *cfgData

// Version stores the build number and is set by the build system using a
// ldflags parameter.
var Version string

func main() {
	run()
}

func run() {
	log.Println("Starting autospotting agent, build:", Version)

	log.Printf("Parsed command line flags: "+
		"regions='%s' "+
		"min_on_demand_number=%d "+
		"min_on_demand_percentage=%.1f "+
		"keep_instance_type=%t",
		conf.Regions,
		conf.MinOnDemandNumber,
		conf.MinOnDemandPercentage,
		conf.KeepInstanceType)

	autospotting.Run(conf.Config)
	log.Println("Execution completed, nothing left to do")
}

// this is the equivalent of a main for when running from Lambda, but on Lambda
// the run() is executed within the handler function every time we have an event
func init() {
	var region string

	if r := os.Getenv("AWS_REGION"); r != "" {
		region = r
	} else {
		region = endpoints.UsEast1RegionID
	}

	conf = &cfgData{
		&autospotting.Config{
			LogFile:         os.Stdout,
			LogFlag:         log.Ldate | log.Ltime | log.Lshortfile,
			MainRegion:      region,
			SleepMultiplier: 1,
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

	flag.StringVar(&c.KeepInstanceType, "keep_instance_type", "",
		"If specified, the spot instances will have a specific instance type:\n"+
			"\tcurrent: the same as initial on-demand instances\n"+
			"\t<instance-type>: the actual instance type to use")
	v := flag.Bool("version", false, "Print version number and exit.")

	flag.Parse()

	if *v {
		fmt.Println("AutoSpotting build:", Version)
		os.Exit(0)
	}

}
