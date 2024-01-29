module github.com/AutoSpotting/AutoSpotting

go 1.14

require (
	github.com/LeanerCloud/ec2-instances-info v0.0.0-20230220145158-57eda6429c43
	github.com/aws/aws-lambda-go v1.37.0
	github.com/aws/aws-sdk-go v1.44.204
	github.com/aws/aws-sdk-go-v2 v1.24.1
	github.com/aws/aws-sdk-go-v2/config v1.26.6 // indirect
	github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue v1.10.13
	github.com/aws/aws-sdk-go-v2/service/autoscaling v1.36.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/cloudformation v1.42.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/codedeploy v1.22.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/costexplorer v1.25.2
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.18.3
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.145.0
	github.com/aws/aws-sdk-go-v2/service/ecs v1.38.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk v1.20.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/elasticloadbalancing v1.21.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2 v1.26.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/lambda v1.49.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/marketplacemetering v1.19.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/sns v1.26.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/sqs v1.29.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssm v1.44.7 // indirect
	github.com/davecgh/go-spew v1.1.1
	github.com/mattn/goveralls v0.0.11
	github.com/namsral/flag v0.0.0-20170814194028-67f268f20922
	github.com/robfig/cron/v3 v3.0.1
	golang.org/x/lint v0.0.0-20210508222113-6edffad5e616
	golang.org/x/tools v0.6.0 // indirect
	golang.org/x/tools/cmd/cover v0.1.0-deprecated
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gotest.tools/v3 v3.0.0
)
