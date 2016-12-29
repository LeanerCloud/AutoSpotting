# Table of Contents #

* [Table of Contents](#table-of-contents)
* [AutoSpotting](#autospotting)
  * [Features and Benefits](#features-and-benefits)
  * [Getting Started](#getting-started)
    * [Requirements](#requirements)
    * [Installation](#installation)
      * [Installation options](#installation-options)
    * [Enable autospotting](#enable-autospotting)
      * [For an AutoScaling group](#for-an-autoscaling-group)
      * [For Elastic Beanstalk](#for-elastic-beanstalk)
    * [Configuration of autospotting](#configuration-of-autospotting)
      * [Testing configuration](#testing-configuration)
      * [Running configuration](#running-configuration)
        * [Minimum on-demand configuration](#minimum-on-demand-configuration)
    * [Updates and Downgrades](#updates-and-downgrades)
      * [Compatibility notices](#compatibility-notices)
    * [Uninstallation](#uninstallation)
  * [Best Practices](#best-practices)
  * [How it works](#how-it-works)
  * [Internal components](#internal-components)
    * [Event generator](#event-generator)
    * [Lambda function](#lambda-function)
  * [Compiling and Installing your own components](#compiling-and-installing-your-own-components)
  * [GitHub Badges](#github-badges)

## AutoSpotting #

A simple tool designed to significantly lower your Amazon AWS costs by
automating the use of the [spot market](https://aws.amazon.com/ec2/spot).

When enabled on an existing on-demand AutoScaling group, it starts launching
EC2 spot instances that are cheaper, at least as powerful and configured as
closely as possible as your existing on-demand instances.

It then gradually swaps them one by one with your existing on-demand instances,
just like shown below:

![Workflow](https://cdn.cloudprowess.com/images/autospotting.gif)

In this case the initial instance type was quite expensive, so the algorithm
chose a different type that had more computing capacity. At the end that group
had 3x more CPU cores and 66% more RAM than in the initial state of the group,
and all this with 33% cost savings and without running entirely on spot
instances, since some users find that a bit risky.

Nevertheless, AutoSpotting tends to be quite reliable even on all-spot
configurations (has automated failover to on-demand nodes and spreads over
multiple price zones), where it can often achieve savings up to 90% off
the usual on-demand prices, much like in the 85% price reduction shown below.
This was seen on a group of two m3.medium instances running in eu-west-1:

![Savings Graph](https://cdn.cloudprowess.com/images/autospotting-savings.png)

[![Launch](https://s3.amazonaws.com/cloudformation-examples/cloudformation-launch-stack.png)](https://console.aws.amazon.com/cloudformation/home?region=us-east-1#/stacks/new?stackName=AutoSpotting&templateURL=https://s3.amazonaws.com/cloudprowess/dv/template.json)

## Features and Benefits ##

* **Significant cost savings compared to on-demand or reserved instances**
  * up to 90% cost reduction compared to on-demand instances.
  * up to 75% cost reduction compared to reserved instances, without any
    down-payment or long term commitment.

* **Easy to install and set up on existing environments based on AutoScaling**
  * you can literally get started within minutes, unlike SpotFleets or other
    tools which may require a considerable migration effort.
  * only needs to be installed once, in a single region, and can handle all
    other regions without any additional configuration (but can also be
    restricted to just a few regions if desired).
  * easy to enable and disable for reverting to the initial configuration based
    on resource tagging, if you decide you don't want to use it anymore.
  * easy to automate migration of multiple existing stacks, simply using scripts
    that set the expected tags on multiple AutoScaling groups.

* **Designed for use against AutoScaling groups with relatively long-running
    instances**
  * for use cases where it is acceptable to run on-demand instances from time to
    time.
  * for short-term batch processing use cases you should have a look into the
    [spot
    blocks](https://aws.amazon.com/blogs/aws/new-ec2-spot-blocks-for-defined-duration-workloads/)
    instead.

* **It doesn't interfere with the group's original launch configuration**
  * any instance replacement or scaling done by AutoScaling would still launch
    your previously configured on-demand instances.
  * on-demand instances often launch faster than spot ones so you don't need to
    wait for potentially slower spot instance fulfilment when you need to scale
    out or when you eventually lose some of the spot capacity, which may happen
    when using spot fleets or other similar tools.

* **Supports any higher level AWS services internally backed
    by AutoScaling**
  * services such as ECS or Elastic Beanstalk work out of the box with minimal
    configuration changes or tweaks, unlike spot fleets or other tools.

* **Compatible out of the box with most AWS services that integrate
    with AutoScaling groups**
  * services such as ELB, ALB, CodeDeploy, CloudWatch, etc. should work out of
    the box or at most require minimal configuration changes.
  * as long as they support instances attached later to existing groups.
  * any other 3rd party services that run on top of AutoScaling groups should
    work as well.

* **Can automatically replace any instance types with any instance types
    available on the spot market**
  * as long as they are cheaper and at least as big as the original instances.
  * it doesn't matter if the original instance is available on the spot market:
    for example it is often replacing t2.medium with better m4.large instances,
    as long as they happen to be cheaper.

* **Self-hosted**
  * has no runtime dependencies on external infrastructure except for the
    regional EC2 and AutoScaling API endpoints.
  * it's not a SaaS, it fully runs within your AWS account.
  * it doesn't gather/persist/export any information about the resources running
    in your AWS account.

* **Free and open source**
  * there are no service fees at install time or run time.
  * you only pay for the small runtime costs it generates.
  * open source, so it is fully auditable and you can see the logs of everything
    it does.
  * the code is relatively small and simple so in case of bugs or missing
    features you may even be able to fix it yourself.

* **Negligible runtime costs**
  * you only pay for the bandwidth consumed performing API calls against AWS
  services across different regions.
  * backed by Lambda, with typical monthly execution time well within the Lambda
  free tier plan.

* **Minimalist and simple implementation**
  * currently about 1000 CLOC of relatively readable Golang code.
  * stateless, and without many moving parts.
  * leveraging and relying on battle-tested AWS services - namely AutoScaling -
    for most mission-critical things, such as instance health checks, horizontal
    scaling, replacement of terminated instances, integration with, ELB, ALB and
    CloudWatch.

* **Relatively safe and secure**
  * most runtime failures or crashes(quite rare nowadays) tend to be harmless.
  * often only result in failing to start new spot instances so your group will
    simply remain or fall back to on-demand capacity, just as it was before.
  * in most cases it is not impacting your running instances nor the ability to
    launch new ones.
  * only needs the minimum set of IAM permissions needed for it to do its job.
  * does not delegate any IAM permissions to resources outside of your AWS
    account.
  * execution scope can be limited to a certain set of regions.

* **Optimizes for high availability over cost whenever possible**
  * it tries to diversify the instance types to reduce the chance of
    simultaneous failures across the entire group. When having enough desired
    capacity, it is often spreading over four different spot pricing zones
    (instance type/availability zone combinations).
  * supports keeping a configurable number of on-demand instances in the group,
    either an absolute number or a percentage of the instances from the group.

## Getting Started ##

### Requirements ###

* You will need credentials to an AWS account able to start CloudFormation
  stacks.
* Some of the following steps assume you have the AWS cli tool installed, but
  the setup can also be done manually using the AWS console or using other tools
  able to launch CloudFormation stacks and set tags on AutoScaling groups.

### Installation ###

You only need to launch a CloudFormation stack in your account. Click the
button below and follow the launch wizard to completion, you can safely use the
default stack parameters.

[![Launch](https://s3.amazonaws.com/cloudformation-examples/cloudformation-launch-stack.png)](https://console.aws.amazon.com/cloudformation/home?region=us-east-1#/stacks/new?stackName=AutoSpotting&templateURL=https://s3.amazonaws.com/cloudprowess/dv/template.json)

If you are using the AWS command-line tool, you can use this command instead:

```
aws cloudformation create-stack \
--stack-name AutoSpotting \
--template-url https://s3.amazonaws.com/cloudprowess/dv/template.json \
--capabilities CAPABILITY_IAM
```

Notes:

* For technical reasons the stack needs to be launched in the
  US-East-1(Virginia) region, so make sure it's not created in another region.
* The AutoScaling groups it runs against can be in any region, since all regions
  are processed at runtime.

#### Installation options ####

The CloudFormation stack takes a number of parameters, which allows you to
configure it for your own environment. The defaults should be safe enough for
most use cases, but for testing or more advanced use cases you may want to tweak
some of them.

Some parameters control the Lambda runtime, while others allow tweaking the
AutoSpotting algorithm, for example to keep a certain amount of on-demand
capacity in the group, or run only against some AWS regions.

The algorithm parameters are just global defaults that can often be overridden
at the AutoScaling group level based on additional tags set on the group.

The full list of parameters, including relatively detailed explanations about
them and their overriding group tags can be seen in the CloudFormation AWS
console when installing or updating the stack.

In case you may want to change some of them later, you can do it at any time by
updating the stack.

Note: even though the stack template is not changing so often and it may often
support multiple software versions, due to possible compatibility issues, it is
recommended to also update the stack template when updating the software
version.

### Enable autospotting ###

#### For an AutoScaling group ####

Since AutoSpotting uses an opt-in model, no resources will be changed in your
AWS account if you just launch the stack. You will need to explicitly enable it
for each AutoScaling group where you want it to be used.

Enabling it for an AutoScaling group is a matter of setting a tag on the group:

```
Key: spot-enabled
Value: true
```

This can be configured with the AWS console from [this
view](https://console.aws.amazon.com/ec2/autoscaling/home?region=eu-west-1#AutoScalingGroups:view=details),

If you use the AWS command-line tools, the same can be achieved using this
command:

```
aws autoscaling
--region eu-west-1 \
create-or-update-tags \
--tags ResourceId=my-auto-scaling-group,ResourceType=auto-scaling-group,Key=spot-enabled,Value=true,PropagateAtLaunch=false
```

##### Note #####

* the above instructions use the eu-west-1 AWS region as an example. Depending
  on where your groups are defined, you may need to use a different region,
  since as mentioned before, your environments may be located anywhere.

This needs to be done for every single AutoScaling group where you want it
enabled, otherwise the group is ignored. If you have lots of groups you may
want to script it in some way.

One good way to automate is using CloudFormation, using this example snippet:

```
"MyAutoScalingGroup": {
  "Properties": {
    "Tags":[
    {
      "Key": "spot-enabled",
      "Value": "true",
      "PropagateAtLaunch": false
    }
    ]
  }
}
```

#### For Elastic Beanstalk ####

* In order to add tags to existing Elastic Beanstalk environment, you will
  need to rebuild the environment with the spot-enabled tag. Follow this
  [guide](http://www.boringgeek.com/add-or-update-tags-on-existing-elastic-beanstalk-environments)

### Configuration of autospotting ###

#### Testing configuration ####

The algorithm of autospotting can have custom CLI configurations. It can use
pre-selected default regions, as well as default on-demand instances to keep in
the auto-scaling groups. This is particularly useful when used during testing,
in order to limit the scope of action and/or general configurations.

To select those testing options from the command line:

```
$ ./autospotting -h
Usage of ./autospotting:
  -min_on_demand_number=0: On-demand capacity (as absolute number) ensured to be
        running in each of your groups.
        Can be overridden on a per-group basis using the tag autospotting_on_demand_number
  -min_on_demand_percentage=0: On-demand capacity (percentage of the total number
        of instances in the group) ensured to be running in each of your groups.
        Can be overridden on a per-group basis using the tag autospotting_on_demand_percentage
        It is ignored if min_on_demand_number is also set.
  -regions="": Regions where it should be activated (comma or whitespace separated
        list, also supports globs), by default it runs on all regions.
        Example: ./autospotting -regions 'eu-*,us-east-1'
```

The value of `-min_on_demand_number` has a higher priority than
`-min_on_demand_percentage`, so if you specify both options in the command line,
percentage will NOT be taken into account. It would be taken into account, ONLY
if the `-min_on_demand_number` is invalid (negativ, above the max number, etc).

The value of `-regions` impact the scope within which autospotting run, while
the options of `-min_on_demand_number` and `-min_on_demand_percentage` would impact
all auto-scaling groups within the regions.

All the flags are also exposed as environment variables, for example using the
-region CLI flag is equivalent to using the REGION environment variable.

**Note**: These configurations are also implemented when running from Lambda,
passed as environment variables set by CloudFormation for the Lambda function.

#### Running configuration ####

##### Minimum on-demand configuration #####

On top of the CLI configuration for the on-demand instances, autospotting
can read those values from the tags of the auto-scaling groups. There are two
available tags: `autospotting_on_demand_number` and
`autospotting_on_demand_percentage`.

Just like for the CLI configuration the defined number has a higher priority
than the percentage value. So the percentage will be ignored if
`autospotting_on_demand_number` is present and valid.

The order of priority from strongest to lowest for minimum on-demand
configuration is as following:

1. Tag `autospotting_on_demand_number` in ASG
1. Tag `autospotting_on_demand_percentage` in ASG
1. Option `-min_on_demand_number` in CLI
1. Option `-min_on_demand_percentage` in CLI

Note that the percentage does round up values. Therefore if we have for example
3 instances running in an autoscaling-group, and you specify 10%, autospotting
will understand that you want 0 instances. If you specify 16%, then it will
still understand that you want 0 instances, because `0.16 * 3` is equal to
`0.47999` so it is rounded down to 0; but if you specify 17%
(or more than 16.66667%) then the algorithm understands that you want at least
one instance (`0.17 * 3 = 0.51`). All in all it should work as you expect, but
this was just to explain some more the functionning of the percentage's math.

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

#### Compatibility notices ####

* As of build 79 the CloudFormation template is also versioned for every
  subsequent build, but unfortunately **this build also breaks compatibility
  with older stacks**. If you run an older build you will also need to update
  the stack when updating to a build later than 79. Although the template rarely
  changes, it's recommended that you always keep it at the same build as the
  binary. Make sure you use the following stack parameter on any newer builds:

```
LambdaHandlerFunction: handler.handle
```

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

## Best Practices ##

These recommendations apply for most cloud environments, but they become
especially important when using more volatile spot instances.

* **Set a non-zero grace period on the AutoScaling group**
  * in order to attach spot instances only after they are fully configured.
  * otherwise they may be attached prematurely before being ready.
  * they may also be terminated after failing load balancer health checks.

* **Check your instance storage and block device mapping configuration**
  * this may become an issue if you use instances which have ephemeral instance
    storage, often the case on previous instance types.
  * you should only specify ephemeral instance store in the on-demand launch
    configuration if you do make use of it by mounting it on the filesystem.
  * the replacement algorithm tries to give you instances with as much instance
    storage as your original instances, since it can't tell if you did mount it.
  * this adds more constraints on the algorithm, so it reduces the number of
    compatible instance types it can use for launching spot instances.
  * this is fine if you actually use that instance storage, but it is reducing
    your options if you don't actually use it, so it may more often fail to get
    spot instances and fall back to on-demand capacity.

* **Don't keep state on instances**
  * You should delegate all your state to external services, AWS has a wide
    offering of stateful services which allow your instances to become
    stateless.
    * Databases: RDS, DynamoDB
    * Caches: ElastiCache
    * Storage: S3, EFS
    * Queues: SQS
  * Don't attach EBS volumes to individual instances, try to use EFS instead.

* **Handle the spot instance termination signal**
  * AWS
    [informs](https://aws.amazon.com/blogs/aws/new-ec2-spot-instance-termination-notices/)
    your spot instances when they are about to be terminated, make use of that
    information to save whatever temporary state you may still have on your
    running spot instances.
  * There are existing tools which implement such an event handler, such as
    [seespot](https://github.com/acksin/seespot). This will need to be added to
    your user_data script.

## How it works ##

Once enabled on an AutoScaling group, it is gradually replacing all the
on-demand instances belonging to the group with compatible and similarly
configured but cheaper spot instances.

The replacements are done using the relatively new Attach/Detach actions
supported by the AutoScaling API. A new compatible spot instance is launched,
and after a while, at least as much as the group's grace period, it will be
attached to the group, while at the same time an on-demand instance is detached
from the group and terminated in order to keep the group at constant capacity.

When assessing the compatibility, it takes into account the hardware specs, such
as CPU cores, RAM size, attached instance store volumes and their type and size,
as well as the supported virtualization types (HVM or PV) of both instance
types. The new spot instance is usually a few times cheaper than the original
instance, while also often providing more computing capacity.

The new spot instance is configured with the same roles, security groups and
tags and set to execute the same user data script as the original instance, so
from a functionality perspective it should be indistinguishable from other
instances in the group, although its hardware specs may be slightly
different(again: at least the same, but often can be of bigger capacity).

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

Similar in concept to @alestic's
[unreliable-town-clock](https://alestic.com/2015/05/aws-lambda-recurring-schedule/),
but internally using the new CloudWatch events just like in his later
developments.

* It is configured to generate a CloudWatch event, for triggering the Lambda
  function.
* The default frequency is every 5 minutes, but it is configurable using
  CloudFormation

### Lambda function ###

* AWS Lambda function connected to the event generator, which triggers it
  periodically.
* It has assigned a IAM role and policy with a set of permissions to call the
  APIs of various AWS services(EC2 and AutoScaling for now) within the user's
  account.
* The permissions are the minimal set required for it to work without the need
  of passing any explicit AWS credentials or access keys.
* Some algorithm parameters can be configured using Lambda environment
  variables, based on some of the CloudFormation stack parameters.
* Contains a handler written in Golang, built using the
  [eawsy/aws-lambda-go](https://github.com/eawsy/aws-lambda-go) library, which
  implements a novel aproach that allows Golang code compiled natively to be
  built in such a way that it can be injected into the Lambda Python runtime.
* The handler implements all the instance replacement logic.
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

## Compiling and Installing your own components ##

It's relatively easy to build and install your own version of this tool's
binaries, removing your dependency on the author's version, and allowing any
customizations and improvements your organization needs.

[More details here](./SETUP.md)

## GitHub Badges ##

[![Build Status](https://travis-ci.org/cristim/autospotting.svg?branch=master)](https://travis-ci.org/cristim/autospotting)
[![Coverage Status](https://coveralls.io/repos/github/cristim/autospotting/badge.svg?branch=master)](https://coveralls.io/github/cristim/autospotting?branch=master)
[![Code Climate](https://codeclimate.com/github/cristim/autospotting/badges/gpa.svg)](https://codeclimate.com/github/cristim/autospotting)
[![Issue Count](https://codeclimate.com/github/cristim/autospotting/badges/issue_count.svg)](https://codeclimate.com/github/cristim/autospotting)
[![license](https://img.shields.io/github/license/mashape/apistatus.svg?maxAge=2592000)]()
[![Chat on Gitter](https://badges.gitter.im/cristim/autospotting.svg)](https://gitter.im/cristim/autospotting?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge)
