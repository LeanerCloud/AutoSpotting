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

	log.Printf("Parsed command line flags: "+
		"regions='%s' "+
		"min_on_demand_number=%d "+
		"min_on_demand_percentage=%.1f "+
		"allowed_instance_types=%v "+
		"disallowed_instance_types=%v "+
		"on_demand_price_multiplier=%.2f "+
		"spot_price_buffer_percentage=%.3f "+
		"bidding_policy=%s "+
		"tag_filters=%s "+
		"spot_product_description=%v "+
		"max_time_spot_request_can_be_holding=%d",
		conf.Regions,
		conf.MinOnDemandNumber,
		conf.MinOnDemandPercentage,
		conf.AllowedInstanceTypes,
		conf.DisallowedInstanceTypes,
		conf.OnDemandPriceMultiplier,
		conf.SpotPriceBufferPercentage,
		conf.BiddingPolicy,
		conf.FilterByTags,
		conf.SpotProductDescription,
		conf.MaxTimeSpotRequestCanBeHolding)

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
		"\n\tRegions where it should be activated (comma or whitespace separated list, "+
			"also supports globs), by default it runs on all regions.\n\t"+
			"Example: ./autospotting -regions 'eu-*,us-east-1'\n")

	flag.Int64Var(&c.MinOnDemandNumber, "min_on_demand_number", autospotting.DefaultMinOnDemandValue,
		"\n\tOn-demand capacity (as absolute number) ensured to be running in each of your groups.\n\t"+
			"Can be overridden on a per-group basis using the tag "+
			autospotting.OnDemandNumberLong+".\n")

	flag.Float64Var(&c.MinOnDemandPercentage, "min_on_demand_percentage", 0.0,
		"\n\tOn-demand capacity (percentage of the total number of instances in the group) "+
			"ensured to be running in each of your groups.\n\t"+
			"Can be overridden on a per-group basis using the tag "+
			autospotting.OnDemandPercentageTag+
			"\n\tIt is ignored if min_on_demand_number is also set.\n")

	flag.StringVar(&c.AllowedInstanceTypes, "allowed_instance_types", "",
		"\n\tIf specified, the spot instances will be of these types.\n"+
			"\tIf missing, the type is autodetected frome each ASG based on it's Launch Configuration.\n"+
			"\tAccepts a list of comma or whitespace seperated instance types (supports globs).\n"+
			"\tExample: ./autospotting -allowed_instance_types 'c5.*,c4.xlarge'\n")

	flag.StringVar(&c.DisallowedInstanceTypes, "disallowed_instance_types", "",
		"\n\tIf specified, the spot instances will _never_ be of these types.\n"+
			"\tAccepts a list of comma or whitespace seperated instance types (supports globs).\n"+
			"\tExample: ./autospotting -disallowed_instance_types 't2.*,c4.xlarge'\n")

	flag.Float64Var(&c.OnDemandPriceMultiplier, "on_demand_price_multiplier", 1.0,
		"\n\tMultiplier for the on-demand price. This is useful for volume discounts or if you want to\n"+
			"\tset your bid price to be higher than the on demand price to reduce the chances that your\n"+
			"\tspot instances will be terminated.\n")

	flag.Float64Var(&c.SpotPriceBufferPercentage, "spot_price_buffer_percentage", autospotting.DefaultSpotPriceBufferPercentage,
		"\n\tPercentage Value of the bid above the current spot price. A spot bid would be placed at a value :\n"+
			"\tcurrent_spot_price * [1 + (spot_price_buffer_percentage/100.0)]. The main benefit is that\n"+
			"\tit protects the group from running spot instances that got significantly more expensive than\n"+
			"\twhen they were initially launched, but still somewhat less than the on-demand price. Can be\n"+
			"\tenforced using the tag: "+autospotting.SpotPriceBufferPercentageTag+". If the bid exceeds\n"+
			"\tthe on-demand price, we place a bid at on-demand price itself.\n")

	flag.StringVar(&c.SpotProductDescription, "spot_product_description", autospotting.DefaultSpotProductDescription,
		"\n\tThe Spot Product or operating system to use when looking up spot price history in the market.\n"+
			"\tValid choices: Linux/UNIX | SUSE Linux | Windows | Linux/UNIX (Amazon VPC) | SUSE Linux (Amazon VPC) | Windows (Amazon VPC)\n")

	flag.StringVar(&c.BiddingPolicy, "bidding_policy", autospotting.DefaultBiddingPolicy,
		"\n\tPolicy choice for spot bid. If set to 'normal', we bid at the on-demand price.\n"+
			"\tIf set to 'aggressive', we bid at a percentage value above the spot price configurable using the spot_price_buffer_percentage.\n")

	flag.StringVar(&c.FilterByTags, "tag_filters", "", "Set of tags to filter the ASGs on.  Default if no value is set will be the equivalent of -tag_filters 'spot-enabled=true'\n\t"+
		"Example: ./autospotting --tag_filters 'spot-enabled=true,Environment=dev,Team=vision'\n")

	flag.Int64Var(&c.MaxTimeSpotRequestCanBeHolding, "max_time_spot_request_can_be_holding", autospotting.DefaultMaxTimeSpotRequestCanBeHolding,
		"\n\tMaximum amount of time (in seconds) that a spot request can be in the 'holding' state, before it is cancelled.\n\t"+
			"The default is to leave the spot request as it is (in the 'holding' for amazon to fullfil)\n")

	v := flag.Bool("version", false, "Print version number and exit.\n")

	flag.Parse()

	if *v {
		fmt.Println("AutoSpotting build:", Version)
		os.Exit(0)
	}

}
