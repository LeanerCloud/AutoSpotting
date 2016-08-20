// This stores a bunch of sessions to various AWS APIs, in order to avoid
// connecting to them over and over again.

package autospotting

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
)

type connections struct {
	session     *session.Session
	autoScaling *autoscaling.AutoScaling
	ec2         *ec2.EC2
}

func getRegions() ([]string, error) {
	var output []string

	// this turns out to be much faster when running locally than using the
	// commented region auto-detection snipped shown below, and anyway due to
	// Lambda limitations we currently only support running it from this region.
	currentRegion := "us-east-1"

	// m := ec2metadata.New(session.New())
	// if m.Available() {
	// 	currentRegion, _ = m.Region()
	// }

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

func (c *connections) connect(region string) {

	logger.Println("Creating Service connections in", region)

	// concurrently connect to all the services we need

	c.session = session.New(
		&aws.Config{
			Region: aws.String(region)},
	)

	asConn := make(chan *autoscaling.AutoScaling)
	ec2Conn := make(chan *ec2.EC2)

	go func() { asConn <- autoscaling.New(c.session) }()
	go func() { ec2Conn <- ec2.New(c.session) }()

	c.autoScaling = <-asConn
	c.ec2 = <-ec2Conn

	logger.Println("Created service connections in", region)
}
