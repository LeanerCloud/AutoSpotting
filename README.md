## AutoSpotting ##

[![Build Status](https://travis-ci.org/cristim/autospotting.svg?branch=master)](https://travis-ci.org/cristim/autospotting)
[![Coverage Status](https://coveralls.io/repos/github/cristim/autospotting/badge.svg?branch=master)](https://coveralls.io/github/cristim/autospotting?branch=master)
[![Code Climate](https://codeclimate.com/github/cristim/autospotting/badges/gpa.svg)](https://codeclimate.com/github/cristim/autospotting)
[![Issue Count](https://codeclimate.com/github/cristim/autospotting/badges/issue_count.svg)](https://codeclimate.com/github/cristim/autospotting)
[![license](https://img.shields.io/github/license/mashape/apistatus.svg?maxAge=2592000)]()
[![Chat on Gitter](https://badges.gitter.im/cristim/autospotting.svg)](https://gitter.im/cristim/autospotting?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge)

Autospotting is a tool meant to automate the provitioning of AWS EC2 spot instances on existing AutoScaling groups, replacing existing instances with significantly cheaper ones.

The goal is to allow the user to achieve significant savings, often in the range of 80% off the AWS EC2 bill, in a way that maximizes the availability in the event of spot market fluctuations of certain instance types in certain availability zones, that trigger termination of spot instances.

Once enabled on an AutoScaling group, it is gradually replacing all the on-demand instances belonging to the group with compatible and similarly configured but cheaper spot instances. The replacements are done using the relatively new Attach/Detach actions supported by the AutoScaling API. A new compatible spot instance is launched, and after a while, at least as much as the group's grace period, it will be attached to the group, while at the same time an on-demand instance is detached from the group and terminated.

When assessing the compatibility, it takes into account the hardware specs, such as CPU cores, RAM size, attached instance store volumes and their type and size, as well as the supported virtualization types (HVM or PV). The new spot instance type is usually a few times cheaper than the original instance type, while also often providing more computing capacity. The new instance is also configured with the same roles, security groups and tags and set to execute the same user data script as the original instance, so from a functionality perspective it should be indistinguishable from other instances in the group.

When replacing multiple instances in a group, the algorithm tries to use a wide variety of instance types, in order to reduce the probability of simultaneous failures that may impact the availability of the group. It Always tries to launch the cheapest instance, but if the group already has a considerable amount of instances of that type in the same availability zone (currently more than 20% of the group's capacity is in that zone and of that instance type), it picks the second cheapest compatible instance, and so on. Assuming the market price is high enough that there are no spot instances that can be launched, it would keep running from on-demand capacity, but it continuously attempts to replace them until eventually the prices decrease ant it gets the chance to convert any of the existing on-demand instances.

Further details can be seen on a number of posts on the author's [blog](https://mcristi.wordpress.com)

## Getting Started ##

### Requirements ###

* You will need credentials to an AWS account able to run CloudFormation stacks.
* The following steps assume you have the AWS cli tool installed, but the setup can also be done manually using the AWS console or using other tools able to launch CloudFormation stacks and set tags on AutoScaling groups.

### Installation ###

First you need to launch a CloudFormation stack in your account. Clicking the button below and following the launch wizard to completion is all you need to get it installed.

[![Launch Stack](https://s3.amazonaws.com/cloudformation-examples/cloudformation-launch-stack.png)]
(https://console.aws.amazon.com/cloudformation/home?region=us-east-1#/stacks/new?stackName=AutoSpotting&templateURL=https://s3.amazonaws.com/cloudprowess/dv/template.json)

If you are using the AWS command-line tool, you can use this command instead:

    aws cloudformation create-stack \
    --stack-name AutoSpotting \
    --template-url https://s3.amazonaws.com/cloudprowess/dv/template.json \
    --capabilities CAPABILITY_IAM

Notes: 
- For technical reasons the stack needs to be launched in US-East-1(Virginia) region, so please make sure it's not created somewhere else in case you use an AWS credentials profile which may be defined for another region.
- The stack is based on prebuilt binaries hosted on the author's AWS account. The author is being actually charged a few pennies of network traffic each month so donations to offset this cost are welcome via Paypal if you are using it for anything serious. The binaries can be self-compiled and it can also be self-hosted with not too much effort in case you have any concerns about reliability, dependence on foreign infrastructure or running binary blobs.
- Once installed, it will automatically execute every 5 minutes and the payload binary will take action on the AutoScaling groups where it was enabled.

### Configuration for an AutoScaling group ###

Enabling it on an AutoScaling group is a matter of setting a tag on the group:

    Key: spot-enabled
    Value: true

This can be configured with the AWS console from [this view](https://console.aws.amazon.com/ec2/autoscaling/home?region=us-east-1#AutoScalingGroups:view=details), but in this case the region may differ, because the stack connects to all your regions when trying to take action.

If you use the AWS command-line tools, the same can be achieved using this command:

    aws --region us-east-1 autoscaling create-or-update-tags \
    --tags ResourceId=my-auto-scaling-group,ResourceType=auto-scaling-group,Key=spot-enabled,Value=true,PropagateAtLaunch=false
    
This needs to be done for every single group where you want it enabled, otherwise the group is ignored. If you have lots of groups you may want to script it in some way.

### Uninstallation ###

If at some point you want to uninstall it, just delete the stack. The AutoScaling groups where it used to be enabled will keep running until their spot instances eventually get outbid and terminated, then replaced by AutoScaling with on-demand ones, eventually bringing the group to the initial state. If you want you can speed up the process by gradually terminating the spot instances yourself.

The tags set on the group can be deleted at any time you want.

## Internals ##

When deployed, the software consists on a number of resources running across multiple Amazon AWS accounts, mostly created automatically with CloudFormation:

### Event generator ###

Similar in concept to @alestic's [unreliable-town-clock](https://alestic.com/2015/05/aws-lambda-recurring-schedule/), but internally using the new CloudWatch events just like in his latter developments.
* deployed in the author's AWS account, only because it is easier to configure against a fixed topic ID by hardcoding the topic name in the binary agent written in golang, and also because triggering Lambda function calls from it is free of charge.
* It is configured generate a CloudWatch event every 5 minutes, which is then sent to the SNS topic
* It has enough IAM permissions to allow anyone to attach to the topic.

### Lambda function ###

* AWS Lambda function deployed in the user's AWS account, entirely configured by CloudFormation.
* It is connected to the event generator topic. Messages sent to the topic trigger its execution, and the topic generates these every 5 minutes.
* Currently written in Python, but it may be rewritten/replaced at some point once AWS implements native support for golang.
* Out of the box it has assigned a IAM role and policy with a set of permissions to call various AWS services within the customer's account. The permissions are the minimal set required for determining the spot instance type, launching spot instances with IAM roles, attaching them to the group, detaching and then terminating on-demand instances without the need of passing any explicit AWS credentials or access keys.

Here is how it reacts on the event that it was given
* It downloads and executes the agent code(a golang binary), stored in S3 and served through a CloudFront distribution, in order to be able to replace the agent binary without customer's intervention for continuous delivery purposes.
* The agent is given all the data generated in the event that triggered the current execution of the function. At the moment the data is written to a pair of JSON files created in lambda function's writeable /tmp directory, passed as command line arguments to the binary, which are then read and parsed by the agent binary at runtime.
* The agent implements code able to handle the events it received. At the moment there are two types of events:
    * Events emmitted by the event generator's SNS topics.
    * It is also executed by the CloudFormation stack whenever the stack is being created or changed, where it implements a custom resource that currently can only attach the function to the event generator's SNS topic when the stack is created.

### agent ###

* Golang binary that gets called from the Lambda function's Python wrapper, which implements all the instance replacement logic.
* Relatively small size for a Golang binary. For achieving that it is stripped and compressed with goupx, then uploaded to an S3 bucket, from where it is currently served through a CloudFront distribution.
* Can react on the SNS events passed as files given as command like arguments
* The spot instances are created by duplicating the configuration of the currently running on-demand instances as closely as possible(IAM roles, security groups, user_data script, etc.) only by adding a spot bid price attribute and eventually changing the instance type to a usually bigger, but compatible one.
* The bid price is set to the on-demand price of the instances configured initially on the AutoScaling group.
* The new launch configuration may also have a different instance type, determined based on compatibility with the original instance type, considering also how much redundancy we need to have in place in the current availability zone, in order to survive instance termination when outbid for a certain instance type.
