module github.com/AutoSpotting/AutoSpotting

go 1.14

require (
	github.com/LeanerCloud/ec2-instances-info v0.0.0-20240226150038-00f4136555ac
	github.com/aws/aws-lambda-go v1.46.0
	github.com/aws/aws-sdk-go v1.50.29
	github.com/aws/aws-sdk-go-v2 v1.25.2
	github.com/aws/aws-sdk-go-v2/config v1.27.4
	github.com/aws/aws-sdk-go-v2/service/autoscaling v1.40.1
	github.com/aws/aws-sdk-go-v2/service/cloudformation v1.46.1
	github.com/aws/aws-sdk-go-v2/service/codedeploy v1.25.1
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.149.1
	github.com/aws/aws-sdk-go-v2/service/ecs v1.41.1
	github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk v1.23.1
	github.com/aws/aws-sdk-go-v2/service/elasticloadbalancing v1.24.1
	github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2 v1.30.1
	github.com/aws/aws-sdk-go-v2/service/marketplacemetering v1.21.1
	github.com/aws/aws-sdk-go-v2/service/sns v1.29.1
	github.com/aws/aws-sdk-go-v2/service/sqs v1.31.1
	github.com/aws/aws-sdk-go-v2/service/ssm v1.49.1
	github.com/davecgh/go-spew v1.1.1
	github.com/mattn/goveralls v0.0.12
	github.com/namsral/flag v0.0.0-20170814194028-67f268f20922
	github.com/robfig/cron/v3 v3.0.1
	github.com/stretchr/testify v1.7.2
	golang.org/x/lint v0.0.0-20210508222113-6edffad5e616
	golang.org/x/tools v0.18.0 // indirect
	golang.org/x/tools/cmd/cover v0.1.0-deprecated
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gotest.tools/v3 v3.0.0
)
