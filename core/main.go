package autospotting

import (
	"io/ioutil"
	"log"
	"os"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

var logger, debug *log.Logger

// Run starts processing all AWS regions looking for AutoScaling groups
// enabled and taking action by replacing more pricy on-demand instances with
// compatible and cheaper spot instances.
func Run(cfg Config) {

	setupLogging(cfg)

	debug.Println(cfg)

	processAllRegions(cfg)

}

func disableLogging() {
	setupLogging(Config{LogFile: ioutil.Discard})
}

func setupLogging(cfg Config) {
	logger = log.New(cfg.LogFile, "", cfg.LogFlag)

	if os.Getenv("AUTOSPOTTING_DEBUG") == "true" {
		debug = log.New(cfg.LogFile, "", cfg.LogFlag)
	} else {
		debug = log.New(ioutil.Discard, "", 0)
	}

}

// processAllRegions iterates all regions in parallel, and replaces instances
// for each of the ASGs tagged with 'spot-enabled=true'.
func processAllRegions(cfg Config) {

	var wg sync.WaitGroup

	regions, err := getRegions()

	if err != nil {
		logger.Println(err.Error())
		return
	}

	for _, r := range regions {

		wg.Add(1)
		r := region{name: r, conf: cfg}

		go func() {

			if r.enabled() {
				logger.Printf("Enabled to run in %s, processing region.\n", r.name)
				r.processRegion()
			} else {
				debug.Println("Not enabled to run in", r.name)
				debug.Println("List of enabled regions:", cfg.Regions)
			}

			wg.Done()
		}()
	}
	wg.Wait()
}

// getRegions generates a list of AWS regions.
func getRegions() ([]string, error) {
	var output []string

	logger.Println("Scanning for available AWS regions")

	// This turns out to be much faster when running locally than using region
	// auto-detection, and anyway due to Lambda limitations we currently only
	// support running it from this region.
	currentRegion := "us-east-1"

	svc := ec2.New(
		session.New(
			&aws.Config{
				Region: aws.String(currentRegion),
			}))

	resp, err := svc.DescribeRegions(&ec2.DescribeRegionsInput{})

	if err != nil {
		logger.Println(err.Error())
		return nil, err
	}

	debug.Println(resp)

	for _, r := range resp.Regions {

		if r != nil && r.RegionName != nil {
			debug.Println("Found region", *r.RegionName)
			output = append(output, *r.RegionName)
		}
	}
	return output, nil
}
