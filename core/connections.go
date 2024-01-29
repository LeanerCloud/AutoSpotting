package autospotting

import (
	"context"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/codedeploy"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancing"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

type connections struct {
	cfg            aws.Config
	autoScaling    *autoscaling.Client
	ec2            *ec2.Client
	elbv2          *elasticloadbalancingv2.Client
	elb            *elasticloadbalancing.Client
	cloudFormation *cloudformation.Client
	lambda         *lambda.Client
	sqs            *sqs.Client
	codedeploy     *codedeploy.Client
	ssm            *ssm.Client
	sns            *sns.Client
	beanstalk      *elasticbeanstalk.Client
	ecs            *ecs.Client
	region         string
}

func (c *connections) setConfig(ctx context.Context, region string) error {
	var err error
	c.cfg, err = config.LoadDefaultConfig(ctx, config.WithRegion(region))
	return err
}

func (c *connections) connect(region, mainRegion string) error {
	log.Println("Creating service connections in", region)

	ctx := context.TODO()

	if c.cfg.Region == "" {
		err := c.setConfig(ctx, region)
		if err != nil {
			return err
		}
	}

	c.autoScaling = autoscaling.NewFromConfig(c.cfg)
	c.ec2 = ec2.NewFromConfig(c.cfg)
	c.cloudFormation = cloudformation.NewFromConfig(c.cfg)
	c.lambda = lambda.NewFromConfig(c.cfg)
	c.codedeploy = codedeploy.NewFromConfig(c.cfg)
	c.ecs = ecs.NewFromConfig(c.cfg)
	c.beanstalk = elasticbeanstalk.NewFromConfig(c.cfg)
	c.elbv2 = elasticloadbalancingv2.NewFromConfig(c.cfg)
	c.elb = elasticloadbalancing.NewFromConfig(c.cfg)

	// For SQS, SSM, and SNS, using mainRegion if specified
	if mainRegion != "" {
		mainCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(mainRegion))
		if err != nil {
			return err
		}
		c.sqs = sqs.NewFromConfig(mainCfg)
		c.ssm = ssm.NewFromConfig(mainCfg)
		c.sns = sns.NewFromConfig(mainCfg)
	} else {
		c.sqs = sqs.NewFromConfig(c.cfg)
		c.ssm = ssm.NewFromConfig(c.cfg)
		c.sns = sns.NewFromConfig(c.cfg)
	}

	c.region = region

	log.Println("Created service connections in", region)
	return nil
}
