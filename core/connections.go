// Copyright (c) 2016-2022 Cristian Măgherușan-Stanciu
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
	"github.com/aws/aws-sdk-go/service/codedeploy"
	"github.com/aws/aws-sdk-go/service/codedeploy/codedeployiface"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/aws/aws-sdk-go/service/lambda/lambdaiface"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
)

type connections struct {
	session        *session.Session
	autoScaling    autoscalingiface.AutoScalingAPI
	ec2            ec2iface.EC2API
	cloudFormation cloudformationiface.CloudFormationAPI
	lambda         lambdaiface.LambdaAPI
	sqs            sqsiface.SQSAPI
	codedeploy     codedeployiface.CodeDeployAPI
	region         string
}

func (c *connections) setSession(region string) {
	c.session = session.Must(
		session.NewSession(&aws.Config{
			Region:   aws.String(region),
			Endpoint: aws.String(os.Getenv("AWS_ENDPOINT_URL")),
		}))
}

func (c *connections) connect(region, mainRegion string) {
	// When using custom Endpoint Url, to avoid unwanted additional costs, enforce ALL AWS login/security variables.
	if len(os.Getenv("AWS_ENDPOINT_URL")) > 0 {
		os.Setenv("AWS_ACCESS_KEY_ID", "fake_testing_key")
		os.Setenv("AWS_SECRET_ACCESS_KEY", "fake_testing_key")
		os.Setenv("AWS_SECURITY_TOKEN", "fake_testing_key")
		os.Setenv("AWS_SESSION_TOKEN", "fake_testing_key")
	}

	debug.Println("Creating service connections in", region)

	if c.session == nil {
		c.setSession(region)
	}

	asConn := make(chan *autoscaling.AutoScaling)
	ec2Conn := make(chan *ec2.EC2)
	cloudformationConn := make(chan *cloudformation.CloudFormation)
	lambdaConn := make(chan *lambda.Lambda)
	sqsConn := make(chan *sqs.SQS)
	codedeployConn := make(chan *codedeploy.CodeDeploy)

	go func() { asConn <- autoscaling.New(c.session) }()
	go func() { ec2Conn <- ec2.New(c.session) }()
	go func() { lambdaConn <- lambda.New(c.session) }()
	go func() { cloudformationConn <- cloudformation.New(c.session) }()
	go func() { codedeployConn <- codedeploy.New(c.session) }()
	go func() { sqsConn <- sqs.New(c.session, aws.NewConfig().WithRegion(mainRegion)) }()

	c.autoScaling, c.ec2, c.cloudFormation, c.lambda, c.sqs, c.codedeploy, c.region = <-asConn, <-ec2Conn, <-cloudformationConn, <-lambdaConn, <-sqsConn, <-codedeployConn, region

	debug.Println("Created service connections in", region)
}
