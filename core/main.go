// Copyright (c) 2016-2019 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"io/ioutil"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
)

var logger, debug *log.Logger

var hourlySavings float64
var savingsMutex = &sync.RWMutex{}

// Run starts processing all AWS regions looking for AutoScaling groups
// enabled and taking action by replacing more pricy on-demand instances with
// compatible and cheaper spot instances.
func Run(cfg *Config) {

	setupLogging(cfg)

	debug.Println(*cfg)

	// use this only to list all the other regions
	ec2Conn := connectEC2(cfg.MainRegion)

	addDefaultFilteringMode(cfg)
	addDefaultFilter(cfg)

	allRegions, err := getRegions(ec2Conn)

	if err != nil {
		logger.Println(err.Error())
		return
	}

	processRegions(allRegions, cfg)

}

func addDefaultFilteringMode(cfg *Config) {
	if cfg.TagFilteringMode != "opt-out" {
		debug.Printf("Configured filtering mode: '%s', considering it as 'opt-in'(default)\n",
			cfg.TagFilteringMode)
		cfg.TagFilteringMode = "opt-in"
	} else {
		debug.Println("Configured filtering mode: 'opt-out'")
	}
}

func addDefaultFilter(cfg *Config) {
	if len(strings.TrimSpace(cfg.FilterByTags)) == 0 {
		switch cfg.TagFilteringMode {
		case "opt-out":
			cfg.FilterByTags = "spot-enabled=false"
		default:
			cfg.FilterByTags = "spot-enabled=true"
		}
	}
}

func setupLogging(cfg *Config) {
	logger = log.New(cfg.LogFile, "", cfg.LogFlag)

	if os.Getenv("AUTOSPOTTING_DEBUG") == "true" {
		debug = log.New(cfg.LogFile, "", cfg.LogFlag)
	} else {
		debug = log.New(ioutil.Discard, "", 0)
	}

}

// processAllRegions iterates all regions in parallel, and replaces instances
// for each of the ASGs tagged with tags as specified by slice represented by cfg.FilterByTags
// by default this is all asg with the tag 'spot-enabled=true'.
func processRegions(regions []string, cfg *Config) {

	var wg sync.WaitGroup

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

func connectEC2(region string) *ec2.EC2 {

	sess, err := session.NewSession()
	if err != nil {
		panic(err)
	}

	return ec2.New(sess,
		aws.NewConfig().WithRegion(region))
}

// getRegions generates a list of AWS regions.
func getRegions(ec2conn ec2iface.EC2API) ([]string, error) {
	var output []string

	logger.Println("Scanning for available AWS regions")

	resp, err := ec2conn.DescribeRegions(&ec2.DescribeRegionsInput{})

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
