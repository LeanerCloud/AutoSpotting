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
	region      string
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

	c.autoScaling, c.ec2, c.region = <-asConn, <-ec2Conn, region

	logger.Println("Created service connections in", region)
}
