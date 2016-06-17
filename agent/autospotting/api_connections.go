// This stores a bunch of sessions to various AWS APIs, in order to avoid re-connecting to them over and over again.

package autospotting

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/aws/aws-sdk-go/service/sns"
)

type connections struct {
	session     *session.Session
	autoScaling *autoscaling.AutoScaling
	ec2         *ec2.EC2
	lambda      *lambda.Lambda
	sns         *sns.SNS
}

func getRegions() ([]string, error) {
	var output []string

	currentRegion := "us-east-1"

	// m := ec2metadata.New(session.New())
	// if m.Available() {
	// 	currentRegion, _ = m.Region()
	// }

	svc := ec2.New(session.New(&aws.Config{Region: aws.String(currentRegion)}))

	resp, err := svc.DescribeRegions(&ec2.DescribeRegionsInput{})

	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}

	for _, r := range resp.Regions {
		fmt.Println("Adding region", *r.RegionName)
		output = append(output, *r.RegionName)
	}
	return output, nil
}

func (c *connections) connect(region string) {

	fmt.Println("Creating Service connections in", region)

	// concurrently connect to all the services we need

	c.session = session.New(&aws.Config{Region: aws.String(region)})

	asConn := make(chan *autoscaling.AutoScaling)
	ec2Conn := make(chan *ec2.EC2)
	lambdaConn := make(chan *lambda.Lambda)
	snsConn := make(chan *sns.SNS)

	go func() { asConn <- autoscaling.New(c.session) }()
	go func() { ec2Conn <- ec2.New(c.session) }()
	go func() { lambdaConn <- lambda.New(c.session) }()
	go func() { snsConn <- sns.New(c.session) }()

	c.autoScaling = <-asConn
	c.ec2 = <-ec2Conn
	c.lambda = <-lambdaConn
	c.sns = <-snsConn

	fmt.Println("Created service connections in", region)
}
