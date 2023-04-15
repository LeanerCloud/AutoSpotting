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
	"github.com/aws/aws-sdk-go/service/elasticbeanstalk"
	"github.com/aws/aws-sdk-go/service/elasticbeanstalk/elasticbeanstalkiface"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/aws/aws-sdk-go/service/elb/elbiface"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/elbv2/elbv2iface"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/aws/aws-sdk-go/service/lambda/lambdaiface"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/aws/aws-sdk-go/service/sns/snsiface"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
)

type connections struct {
	session        *session.Session
	autoScaling    autoscalingiface.AutoScalingAPI
	ec2            ec2iface.EC2API
	elbv2          elbv2iface.ELBV2API
	elb            elbiface.ELBAPI
	cloudFormation cloudformationiface.CloudFormationAPI
	lambda         lambdaiface.LambdaAPI
	sqs            sqsiface.SQSAPI
	codedeploy     codedeployiface.CodeDeployAPI
	ssm            ssmiface.SSMAPI
	sns            snsiface.SNSAPI
	beanstalk      elasticbeanstalkiface.ElasticBeanstalkAPI
	region         string
}

func (c *connections) setSession(region string) {
	c.session = session.Must(
		session.NewSession(&aws.Config{Region: aws.String(region)}))
}

func (c *connections) connect(region, mainRegion string) {

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
	ssmConn := make(chan *ssm.SSM)
	snsConn := make(chan *sns.SNS)
	beanstalkConn := make(chan *elasticbeanstalk.ElasticBeanstalk)
	elbConn := make(chan *elb.ELB)
	elbv2Conn := make(chan *elbv2.ELBV2)

	go func() { asConn <- autoscaling.New(c.session) }()
	go func() { ec2Conn <- ec2.New(c.session) }()
	go func() { lambdaConn <- lambda.New(c.session) }()
	go func() { cloudformationConn <- cloudformation.New(c.session) }()
	go func() { codedeployConn <- codedeploy.New(c.session) }()
	go func() { sqsConn <- sqs.New(c.session, aws.NewConfig().WithRegion(mainRegion)) }()
	go func() { ssmConn <- ssm.New(c.session, aws.NewConfig().WithRegion(mainRegion)) }()
	go func() { snsConn <- sns.New(c.session, aws.NewConfig().WithRegion(mainRegion)) }()
	go func() { beanstalkConn <- elasticbeanstalk.New(c.session) }()
	go func() { elbConn <- elb.New(c.session) }()
	go func() { elbv2Conn <- elbv2.New(c.session) }()

	c.autoScaling, c.ec2, c.cloudFormation, c.lambda, c.sqs, c.codedeploy, c.ssm, c.sns, c.beanstalk, c.elb, c.elbv2, c.region =
		<-asConn, <-ec2Conn, <-cloudformationConn, <-lambdaConn, <-sqsConn, <-codedeployConn, <-ssmConn, <-snsConn, <-beanstalkConn, <-elbConn, <-elbv2Conn, region

	debug.Println("Created service connections in", region)
}
