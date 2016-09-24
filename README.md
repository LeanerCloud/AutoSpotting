# AutoSpotting #

A simple tool designed to significantly lower your Amazon AWS costs by
automating the use of the [spot market](https://aws.amazon.com/ec2/spot). It can
often achieve savings in the range of 80-90% off the usual on-demand prices,
like shown in the screenshot below.

![Savings Graph](https://cdn.cloudprowess.com/images/autospotting-savings.png)

When enabled on your existing on-demand AutoScaling group, it starts launching
EC2 spot instances that are cheaper, at least as powerful and configured as
closely as possible as your existing on-demand instances.

It then gradually swaps them with your existing on-demand instances, which can
then be terminated.

# Features

* Easy to install and set up on existing environments, see the installation
  steps below for more details
* Designed for AutoScaling groups with long-running instances
* Should be compatible with any higher level AWS services internally backed by
  AutoScaling, such as ECS or Beanstalk.
* Optimizes for high availability over lowest costs whenever possible
* Minimalist implementation, leveraging and relying on battle-tested AWS
  services - mainly AutoScaling and ELB - for all the mission-critical stuff:
  * instance health checks
  * replacement of terminated instances
  * ELB integration
  * horizontal scaling
* Minimal cost overhead, within the Lambda free tier and with low bandwidth costs

## Getting Started ##

### Requirements ###
* You will need credentials to an AWS account able to run CloudFormation stacks.
* Some of the following steps assume you have the AWS cli tool installed, but the setup
  can also be done manually using the AWS console or using other tools able to
  launch CloudFormation stacks and set tags on AutoScaling groups.

### Installation ###

First you need to launch a CloudFormation stack in your account. Clicking the
button below and following the launch wizard to completion is all you need to
get it installed, you can safely use the default stack parameters.

[![Launch Stack](https://s3.amazonaws.com/cloudformation-examples/cloudformation-launch-stack.png)](https://console.aws.amazon.com/cloudformation/home?region=us-east-1#/stacks/new?stackName=AutoSpotting&templateURL=https://s3.amazonaws.com/cloudprowess/dv/template.json)

If you are using the AWS command-line tool, you can use this command instead:

    aws cloudformation create-stack \
    --stack-name AutoSpotting \
    --template-url https://s3.amazonaws.com/cloudprowess/dv/template.json \
    --capabilities CAPABILITY_IAM

Notes:

* For technical reasons the stack needs to be launched in US-East-1(Virginia)
  region, so make sure it's not created in another region.

### Configuration for an AutoScaling group ###

Enabling it on an AutoScaling group is a matter of setting a tag on the group:

    Key: spot-enabled
    Value: true

This can be configured with the AWS console from [this view](https://console.aws.amazon.com/ec2/autoscaling/home?region=us-east-1#AutoScalingGroups:view=details),
but in this case the region may differ, because the stack connects to all your
regions when trying to take action.

If you use the AWS command-line tools, the same can be achieved using this
command:

    aws --region us-east-1 autoscaling create-or-update-tags --tags ResourceId=my-auto-scaling-group,ResourceType=auto-scaling-group,Key=spot-enabled,Value=true,PropagateAtLaunch=false

This needs to be done for every single group where you want it enabled,
otherwise the group is ignored. If you have lots of groups you may want to
script it in some way.

### Updates and Downgrades ###

The software doesn't auto-update anymore(it used to in the first few versions), so you will need to manually perform updates using CloudFormation.

The updates need the first 8 characters of the git commit SHA of the version you would like to use, no matter if it is old or new.

Assuming you want to update to the version with the git commit SHA hash starting with d34db33f, you will need to perform a CloudFormation stack update in which you change the "LambdaZipPath" stack parameter into "dv/lambda_d34db33f.zip".

### Uninstallation ###

If at some point you want to uninstall it, just delete the stack. The
AutoScaling groups where it used to be enabled will keep running until their
spot instances eventually get outbid and terminated, then replaced by
AutoScaling with on-demand ones, eventually bringing the group to the initial
state. If you want you can speed up the process by gradually terminating the
spot instances yourself.

The tags set on the group can be deleted at any time you want.


# How it works

Once enabled on an AutoScaling group, it is gradually replacing all the
on-demand instances belonging to the group with compatible and similarly
configured but cheaper spot instances. The replacements are done using the
relatively new Attach/Detach actions supported by the AutoScaling API. A new
compatible spot instance is launched, and after a while, at least as much as the
group's grace period, it will be attached to the group, while at the same time
an on-demand instance is detached from the group and terminated.

When assessing the compatibility, it takes into account the hardware specs, such
as CPU cores, RAM size, attached instance store volumes and their type and size,
as well as the supported virtualization types (HVM or PV). The new spot instance
type is usually a few times cheaper than the original instance type, while also
often providing more computing capacity. The new instance is also configured
with the same roles, security groups and tags and set to execute the same user
data script as the original instance, so from a functionality perspective it
should be indistinguishable from other instances in the group.

When replacing multiple instances in a group, the algorithm tries to use a wide
variety of instance types, in order to reduce the probability of simultaneous
failures that may impact the availability of the entire group. It always tries
to launch the cheapest instance, but if the group already has a considerable
amount of instances of that type in the same availability zone (currently more
than 20% of the group's capacity is in that zone and of that instance type), it
picks the second cheapest compatible instance, and so on. Assuming the market
price is high enough that there are no spot instances that can be launched, it
would keep running from on-demand capacity, but it continuously attempts to
replace them until eventually the prices decrease ant it gets the chance to
convert any of the existing on-demand instances.


## Internals ##

When deployed, the software consists on a number of resources running in your
Amazon AWS account, created automatically with CloudFormation:

### Event generator ###

Similar in concept to @alestic's [unreliable-town-clock](https://alestic.com/2015/05/aws-lambda-recurring-schedule/),
but internally using the new CloudWatch events just like in his later
developments.
* It is configured generate a CloudWatch event every 5 minutes, which is then
  launching the Lambda function.

### Lambda function ###
* AWS Lambda function connected to the event generator, which triggers it
  periodically, currently every 5 minutes.
* Written in Python, but it may be rewritten/replaced at some point
  once AWS implements native support for golang.
* It has assigned a IAM role and policy with a set of permissions to call
  various AWS services within the user's account. The permissions are the
  minimal set required for it to work without the need of passing any explicit
  AWS credentials or access keys.
* When executed it checks the latest version, downloads if not already present
  and runs the agent code(a golang binary)

### agent ###

* Stripped Golang binary, build into the Lambda function ZIP file.
* It is executed from the Lambda function's Python wrapper.
* Implements all the instance replacement logic.
  * The spot instances are created by duplicating the configuration of the
    currently running on-demand instances as closely as possible(IAM roles,
    security groups, user_data script, etc.) only by adding a spot bid price
    attribute and eventually changing the instance type to a usually bigger, but
    compatible one.
  * The bid price is set to the on-demand price of the instances configured
    initially on the AutoScaling group.
  * The new launch configuration may also have a different instance type,
    determined based on compatibility with the original instance type,
    considering also how much redundancy we need to have in place in the current
    availability zone, in order to survive instance termination when outbid for
    a certain instance type.

# GitHub Badges

[![Build Status](https://travis-ci.org/cristim/autospotting.svg?branch=master)](https://travis-ci.org/cristim/autospotting)
[![Coverage Status](https://coveralls.io/repos/github/cristim/autospotting/badge.svg?branch=master)](https://coveralls.io/github/cristim/autospotting?branch=master)
[![Code Climate](https://codeclimate.com/github/cristim/autospotting/badges/gpa.svg)](https://codeclimate.com/github/cristim/autospotting)
[![Issue Count](https://codeclimate.com/github/cristim/autospotting/badges/issue_count.svg)](https://codeclimate.com/github/cristim/autospotting)
[![license](https://img.shields.io/github/license/mashape/apistatus.svg?maxAge=2592000)]()
[![Chat on Gitter](https://badges.gitter.im/cristim/autospotting.svg)](https://gitter.im/cristim/autospotting?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge)
