// Copyright (c) 2016-2019 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

// This stores a bunch of sessions to various AWS APIs, in order to avoid
// connecting to them over and over again.

package autospotting

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
)

type connections struct {
	session        *session.Session
	autoScaling    autoscalingiface.AutoScalingAPI
	ec2            ec2iface.EC2API
	cloudFormation cloudformationiface.CloudFormationAPI
	region         string
}

func (c *connections) setSession(region string) {
	c.session = session.Must(
		session.NewSession(&aws.Config{Region: aws.String(region)}))
}

func (c *connections) connect(region string) {

	debug.Println("Creating service connections in", region)

	if c.session == nil {
		c.setSession(region)
	}

	asConn := make(chan *autoscaling.AutoScaling)
	ec2Conn := make(chan *ec2.EC2)
	cloudformationConn := make(chan *cloudformation.CloudFormation)

	go func() { asConn <- autoscaling.New(c.session) }()
	go func() { ec2Conn <- ec2.New(c.session) }()
	go func() { cloudformationConn <- cloudformation.New(c.session) }()

	c.autoScaling, c.ec2, c.cloudFormation, c.region = <-asConn, <-ec2Conn, <-cloudformationConn, region

	debug.Println("Created service connections in", region)
}
