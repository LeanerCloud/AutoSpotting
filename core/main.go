package autospotting

import (
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

// Run starts processing all AWS regions looking for AutoScaling groups
// enabled and taking action by replacing more pricy on-demand instances with
// compatible and cheaper spot instances.
func Run(instancesFile string) {

	initLogger()

	var jsonInst jsonInstances

	logger.Println("Loading on-demand instance pricing information")
	jsonInst.loadFromFile(instancesFile)

	processAllRegions(&jsonInst)

}

// processAllRegions iterates all regions in parallel, and replaces instances
// for each of the ASGs tagged with 'spot-enabled=true'.
func processAllRegions(instData *jsonInstances) {

	var wg sync.WaitGroup

	regions, err := getRegions()

	if err != nil {
		logger.Println(err.Error())
		return
	}
	for _, r := range regions {
		wg.Add(1)
		r := region{name: r}
		go func() {
			r.processRegion(instData)
			wg.Done()
		}()
	}
	wg.Wait()
}

// getRegions generates a list of AWS regions.
func getRegions() ([]string, error) {
	var output []string

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

	for _, r := range resp.Regions {
		logger.Println("Adding region", *r.RegionName)
		output = append(output, *r.RegionName)
	}
	return output, nil
}
