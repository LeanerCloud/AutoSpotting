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
* Designed for use against AutoScaling groups with relatively long-running
  instances, where it's acceptable to run costlier on-demand instances from
  time to time, as opposed to short-term batch processing tasks
* Supports higher level AWS services internally backed by AutoScaling, such as 
  ECS or Elastic Beanstalk.
* Optimizes for high availability over lowest costs whenever possible, but it
  still often achieves significant cost savings.
* Minimalist implementation, leveraging and relying on battle-tested AWS
  services - mainly AutoScaling - for all the mission-critical stuff:
  * instance health checks
  * replacement of terminated instances
  * ELB or ALB integration
  * horizontal scaling
* Should be compatible out of the box with most AWS services that integrate
  with your AutoScaling groups, such as CodeDeploy, CloudWatch, etc. as long
  as they support instances attached later to existing groups
* Can automatically replace any instance types with any instance types available
  on the spot market
  * as long as they are cheaper and at least as big as the original instances
  * it doesn't matter if the original instance is available on the spot market:
  for example it is often replacing t2.medium with better m4.large instances,
  as long as they happen to be cheaper.
* Self-contained, has no runtime dependencies on external infrastructure,
  except for the regional EC2 and AutoScaling API endpoints
* Minimal cost overhead, typically a few cents per month
  * backed by Lambda, with typical execution time well within the Lambda
  free tier
  * all you pay for running it are tiny bandwidth costs, measured in 
  cents/month, for performing API calls against all regional API endpoints
  of the EC2 and AutoScaling AWS services.
 
## Getting Started ##

### Requirements ###

* You will need credentials to an AWS account able to start CloudFormation stacks.
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

* For technical reasons the stack needs to be launched in the US-East-1(Virginia)
  region, so make sure it's not created in another region.
* The AutoScaling groups it runs against can be in any region, since all regions
  are processed at runtime.

### Configuration for an AutoScaling group ###

Enabling it on an AutoScaling group is a matter of setting a tag on the group:

    Key: spot-enabled
    Value: true

This can be configured with the AWS console from [this view](https://console.aws.amazon.com/ec2/autoscaling/home?region=us-east-1#AutoScalingGroups:view=details), (the region may differ).

As mentioned before, your environments may be in any AWS region.

If you use the AWS command-line tools, the same can be achieved using this
command:

    aws --region us-east-1 autoscaling create-or-update-tags --tags ResourceId=my-auto-scaling-group,ResourceType=auto-scaling-group,Key=spot-enabled,Value=true,PropagateAtLaunch=false

This needs to be done for every single AutoScaling group where you want it
enabled, otherwise the group is ignored. If you have lots of groups you may
want to script it in some way.

### Updates and Downgrades ###

The software doesn't auto-update anymore(it used to in the first few versions),
so you will need to manually perform updates using CloudFormation, based on the
Travis CI build number of the version you would like to use going forward.

This method can be used both for upgrades and downgrades, so assuming you would
like to switch to the build with the number 45, you will need to perform a
CloudFormation stack update in which you change the "LambdaZipPath" stack
parameter to a value that looks like `dv/lambda_build_45.zip`.

Git commit SHAs(truncated to 7 characters) are also accepted instead of the
build numbers, so for example `dv/lambda_build_f7f395d.zip` should also be a
valid parameter, as long as that build is available in the author's
[S3 bucket](http://s3.amazonaws.com/cloudprowess).

The full list of builds and their respective git commits can be seen on the
Travis CI [builds page](https://travis-ci.org/cristim/autospotting/builds)

### Uninstallation ###

If at some point you want to uninstall it, you just need to delete the
CloudFormation stack.

The AutoScaling groups where it used to be enabled will
keep running until their spot instances eventually get outbid and terminated,
then replaced by AutoScaling with on-demand ones. This is eventually bringing
the group to the initial state. If you want, you can speed up the process by
gradually terminating the spot instances yourself.

The tags set on the group can be deleted at any time you want it to be
disabled for that group.

# How it works

Once enabled on an AutoScaling group, it is gradually replacing all the
on-demand instances belonging to the group with compatible and similarly
configured but cheaper spot instances.

The replacements are done using the relatively new Attach/Detach actions supported
by the AutoScaling API. A new compatible spot instance is launched, and after a
while, at least as much as the group's grace period, it will be attached to the
group, while at the same time an on-demand instance is detached from the group
and terminated in order to keep the group at constant capacity.

When assessing the compatibility, it takes into account the hardware specs, such
as CPU cores, RAM size, attached instance store volumes and their type and size,
as well as the supported virtualization types (HVM or PV) of both instance types.
The new spot instance is usually a few times cheaper than the original instance,
while also often providing more computing capacity.

The new spot instance is configured with the same roles, security groups and tags
and set to execute the same user data script as the original instance, so from a
functionality perspective it should be indistinguishable from other instances in
the group, although its hardware specs may be slightly different(again: at least
the same, but often can be of bigger capacity).

When replacing multiple instances in a group, the algorithm tries to use a wide
variety of instance types, in order to reduce the probability of simultaneous
failures that may impact the availability of the entire group. It always tries
to launch the cheapest available compatible instance type, but if the group
already has a considerable amount of instances of that type in the same
availability zone (currently more than 20% of the group's capacity is in that
zone and of that instance type), it picks the second cheapest compatible
instance, and so on.

During multiple replacements performed on a given group, it only swaps them one
at a time per Lambda function invocation, in order to not change the group too
fast, but instances belonging to multiple groups can be replaced concurrently.
If you find this slow, the Lambda function invocation frequency (defaulting to
once every 5 minutes) can be changed by updating the CloudFormation stack, which
has a parameter for it.

In the (so far unlikely) case in which the market price is high enough that 
there are no spot instances that can be launched, (and also in case of software
crashes which may still rarely happen), the group would not be changed and it
would keep running as it is, but AutoSpotting will continuously attempt to
replace them, until eventually the prices decrease again and replaecments may
succeed again.


## Internal components ##

When deployed, the software consists on a number of resources running in your
Amazon AWS account, created automatically with CloudFormation:

### Event generator ###

Similar in concept to @alestic's [unreliable-town-clock](https://alestic.com/2015/05/aws-lambda-recurring-schedule/),
but internally using the new CloudWatch events just like in his later
developments.
* It is configured to generate a CloudWatch event, for triggering the Lambda function.
* The default frequency is every 5 minutes, but it is configurable using
CloudFormation


### Lambda function ###
* AWS Lambda function connected to the event generator, which triggers it
  periodically.
* Small handler written in Python, but it may be rewritten/replaced at some point
  once AWS implements native support for golang.
* It has assigned a IAM role and policy with a set of permissions to call
  various AWS services within the user's account.
* The permissions are the minimal set required for it to work without the need
  of passing any explicit AWS credentials or access keys.
* When executed, it runs the agent code(a golang binary) also included in the
Lambda function's code ZIP archive.

### agent ###

* Stripped Golang binary, bundled at build time into the Lambda function ZIP file.
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
