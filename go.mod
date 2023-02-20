module github.com/AutoSpotting/AutoSpotting

go 1.14

require (
	github.com/LeanerCloud/ec2-instances-info v0.0.0-20230220145158-57eda6429c43
	github.com/aws/aws-lambda-go v1.37.0
	github.com/aws/aws-sdk-go v1.44.204
	github.com/aws/aws-sdk-go-v2 v1.17.4
	github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue v1.10.13
	github.com/aws/aws-sdk-go-v2/service/costexplorer v1.25.2
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.18.3
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.86.0
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
