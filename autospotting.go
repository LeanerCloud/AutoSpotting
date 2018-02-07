package main

import (
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/cristim/autospotting/core"
	"github.com/cristim/ec2-instances-info"
	"github.com/namsral/flag"
)

type cfgData struct {
	*autospotting.Config
}

var conf *cfgData

// Version represents the build version being used
var Version string

// ExpirationDate represents the date at which the version will expire
var ExpirationDate string

func main() {
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		lambda.Start(Handler)
	} else {
		run()
	}
}

func run() {
	log.Println("Starting autospotting agent, build ", Version, "expiring on", ExpirationDate)

	if isExpired(ExpirationDate) {
		log.Fatalln("Autospotting expired, please install a newer version.")
		return
	}

	log.Printf("Parsed command line flags: "+
		"regions='%s' "+
		"min_on_demand_number=%d "+
		"min_on_demand_percentage=%.1f "+
		"allowed_instance_types=%v "+
		"disallowed_instance_types=%v "+
		"on_demand_price_multiplier=%.2f "+
		"spot_price_buffer_percentage=%.3f "+
		"bidding_policy=%s "+
		"filter_by_tag=%s",
		conf.Regions,
		conf.MinOnDemandNumber,
		conf.MinOnDemandPercentage,
		conf.AllowedInstanceTypes,
		conf.DisallowedInstanceTypes,
		conf.OnDemandPriceMultiplier,
		conf.SpotPriceBufferPercentage,
		conf.BiddingPolicy, conf.FilterByTags)

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

// Handler implements the AWS Lambda handler
func Handler(request events.APIGatewayProxyRequest) {
	run()
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
			autospotting.OnDemandPercentageTag+
			"\n\tIt is ignored if min_on_demand_number is also set.")

	flag.StringVar(&c.AllowedInstanceTypes, "allowed_instance_types", "",
		"If specified, the spot instances will have a specific instance type:\n"+
			"\tcurrent: the same as initial on-demand instances\n"+
			"\t<instance-type>: the actual instance type to use")

	flag.StringVar(&c.DisallowedInstanceTypes, "disallowed_instance_types", "",
		"If specified, the spot instances will _never_ be of this type. "+
			"This should be a list of instance types (comma or whitespace separated, "+
			"also supports globs).\n\t"+
			"Example: ./autospotting -disallowed_instance_types 't2.*,c4.xlarge'")

	flag.Float64Var(&c.OnDemandPriceMultiplier, "on_demand_price_multiplier", 1.0,
		"Multiplier for the on-demand price. This is useful for volume discounts or if you want to\n"+
			"\tset your bid price to be higher than the on demand price to reduce the chances that your\n"+
			"\tspot instances will be terminated.")

	flag.Float64Var(&c.SpotPriceBufferPercentage, "spot_price_buffer_percentage", 10,
		"Percentage Value of the bid above the current spot price. A spot bid would be placed at a value :\n"+
			"\tcurrent_spot_price * [1 + (spot_price_buffer_percentage/100.0)]. The main benefit is that\n"+
			"\tit protects the group from running spot instances that got significantly more expensive than\n"+
			"\twhen they were initially launched, but still somewhat less than the on-demand price. Can be\n"+
			"\tenforced using the tag: "+autospotting.SpotPriceBufferPercentageTag+". If the bid exceeds\n"+
			"\tthe on-demand price, we place a bid at on-demand price itself.")

	flag.StringVar(&c.BiddingPolicy, "bidding_policy", "normal",
		"Policy choice for spot bid. If set to 'normal', we bid at the on-demand price. If set to 'aggressive',\n"+
			"\twe bid at a percentage value above the spot price. ")

	flag.Var(&c.FilterByTags, "tag_filters", "Set of tags to filter the ASGs on.  Default is -tag_filters 'spot-enabled=true'\n\t"+
		"Example: ./autospotting -tag_filters 'Environment=dev' -tag_filters 'Team=vision' or -tag_filters 'spot-enabled=true,Environment=dev,Team=vision")

	v := flag.Bool("version", false, "Print version number and exit.")

	flag.Parse()

	if *v {
		fmt.Println("AutoSpotting build:", Version)
		os.Exit(0)
	}

}
