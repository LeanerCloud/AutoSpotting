# Getting started #

* [Getting started](#getting-started)
  * [Requirements](#requirements)
  * [Installation](#installation)
    * [Installation options](#installation-options)
    * [Install via cloudformation](#install-via-cloudformation)
    * [Install via terraform](#install-via-terraform)
  * [Enable autospotting](#enable-autospotting)
    * [For an AutoScaling group](#for-an-autoscaling-group)
    * [For Elastic Beanstalk](#for-elastic-beanstalk)
  * [Configuration of autospotting](#configuration-of-autospotting)
    * [Testing configuration](#testing-configuration)
    * [Running configuration](#running-configuration)
      * [Minimum on-demand configuration](#minimum-on-demand-configuration)
    * [Debug autospotting](#debug-autospotting)
  * [Updates and Downgrades](#updates-and-downgrades)
    * [Compatibility notices](#compatibility-notices)
  * [Uninstallation](#uninstallation)
    * [Uninstall via cloudformation](#uninstall-via-cloudformation)

## Binary License Notice ##

All pre-build binaries mentioned in this page are distributed under a
proprietary [license](BINARY_LICENSE), and can only be used for evaluation
purposes as long as the generated savings are less than $1000 monthly.

Project patrons and code contributors can get access to stable builds, which
have been thoroughly tested and come with enterprise-grade support.

If you don't agree with the terms of this license nor you want to become
a patron, you can still build it from source code yourself but you'll get very
limited community support if you do so.

See [autospotting.org](https://autospotting.org) for more details.

## Requirements ##

* You will need credentials to an AWS account able to start CloudFormation
  stacks.
* Some of the following steps assume you have the AWS cli tool installed, but
  the setup can also be done manually using the AWS console or using other tools
  able to launch CloudFormation stacks and set tags on AutoScaling groups.

## Installation ##

### Installation options ###

Autospotting can be installed via CloudFormation or Terraform, both install
methods take a number of parameters, which allows you to configure it for
your own environment. The defaults should be safe enough for most use cases,
but for testing or more advanced use cases you may want to tweak some of them.

Some parameters control the Lambda runtime, while others allow tweaking the
AutoSpotting algorithm, for example to keep a certain amount of on-demand
capacity in the group, or run only against some AWS regions.

The algorithm parameters are just global defaults that can often be overridden
at the AutoScaling group level based on additional tags set on the group.

The full list of parameters, including relatively detailed explanations about
them and their overriding group tags can be seen in the CloudFormation AWS
console or in the variables.tf file for Terraform.

In case you may want to change some of them later, you can do it at any time by
updating the stack via CloudFormation or Terraform.

Note: even though the CloudFormation stack template is not changing so often
and it may often support multiple software versions, due to possible
compatibility issues, it is recommended to also update the stack template when
updating the software version.

### Install via CloudFormation ###

To install it via CloudFormation, you only need to launch a CloudFormation
stack in your account. Click the button below and follow the launch wizard to
completion, you can safely use the default stack parameters.

[![Launch](https://s3.amazonaws.com/cloudformation-examples/cloudformation-launch-stack.png)](https://console.aws.amazon.com/cloudformation/home?region=us-east-1#/stacks/new?stackName=AutoSpotting&templateURL=https://s3.amazonaws.com/cloudprowess/nightly/template.yaml)

If you are using the AWS command-line tool, you can use this command instead:

``` shell
aws cloudformation create-stack \
--stack-name AutoSpotting \
--template-url https://s3.amazonaws.com/cloudprowess/nightly/template.yaml \
--capabilities CAPABILITY_IAM
```

Notes:

* For technical reasons the stack launched from the official binaries needs to
  be launched in the US-East-1(Virginia) region, so make sure it's not created
  in another region. Custom builds can be deployed in any region you prefer,
  just make sure your S3 bucket is in that region.
* The AutoScaling groups it runs against can be in any region, since all regions
  are processed at runtime, unless configured otherwise.

### Install via terraform ###

A terraform module for AutoSpotting is published in at [https://github.com/AutoSpotting/terraform-aws-autospotting](https://github.com/AutoSpotting/terraform-aws-autospotting).

### Install as Kubernetes cronjob ###

We have an example configuration file that allows you to run AutoSpotting as a
Kubernetes cron job, instead of running it in AWS Lambda.

<!-- markdownlint-disable MD013 -->

``` shell
curl https://raw.githubusercontent.com/AutoSpotting/AutoSpotting/master/kubernetes/autospotting-cron.yaml.example > autospotting-cron.yaml
```

<!-- markdownlint-enable MD013 -->

You can then edit it locally, tweaking it to suit your needs.
Once you're happy with it, you can launch it on your Kubernetes cluster:

``` shell
kubectl create -f kubernetes/autospotting-cron.yaml
```

You can tweak the configuration later using `kubectl edit cronjob autospotting`.

Keep in mind that this job automatically updates to the latest official
binaries, so you may want to host your own Docker images if you want to stick to
a certain version or you don't want to comply with the terms of our binary
license.

## Enable autospotting ##

### For an AutoScaling group ###

Since AutoSpotting by default uses an opt-in model, no resources will be changed
in your AWS account if you just launch the stack. You will need to explicitly
enable it for each AutoScaling group where you want it to be used.

Enabling it for an AutoScaling group is a matter of setting a tag on the group:

``` yaml
Key: spot-enabled
Value: true
```

This can be configured with the AWS console from [this
view](https://console.aws.amazon.com/ec2/autoscaling/home?region=eu-west-1#AutoScalingGroups:view=details),

If you use the AWS command-line tools, the same can be achieved using this
command:

``` shell
aws autoscaling
--region eu-west-1 \
create-or-update-tags \
--tags ResourceId=my-auto-scaling-group,ResourceType=auto-scaling-group,Key=spot-enabled,Value=true,PropagateAtLaunch=false
```

**Note:** the above instructions use the eu-west-1 AWS region as an example.
Depending on where your groups are defined, you may need to use a different
region, since as mentioned before, your environments may be located anywhere.

This needs to be done for every single AutoScaling group where you want it
enabled, otherwise the group is ignored. If you have lots of groups you may
want to script it in some way.

One good way to automate is using CloudFormation, using this example snippet:

``` json
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

**Note:** The `spot-enabled=true` tag for `opt-in` is configurable. See the
stack parameters for the way to override it.

**Note:** AutoSpotting now also supports an `opt-out` mode, in which it will
take over all your groups except of those tagged with the configured tag. The
default (but also configurable) `opt-out` tag is `spot-enabled=false`. This may
be risky, please handle with care.

### For Elastic Beanstalk ###

Elastic Beanstalk uses CloudFormation to create an Auto-Scaling Group. The ASG
is then in charge of automatically scaling your application up and down. As a
result, AutoSpotting works natively with Elastic Beanstalk.

Follow these steps to configure AutoSpotting with Elastic Beanstalk.

#### 1 - Add the `spot-enabled` tag ####

Similar to standalone auto-scaling groups, you need to tag your Elastic Beanstalk
environment with the `spot-enabled` tag to let AutoSpotting manage the instances
in the group.

To add tags to an existing Elastic Beanstalk environment, you will need to rebuild
or update the environment with the `spot-enabled` tag. For more details you can
follow this [guide](http://www.boringgeek.com/add-or-update-tags-on-existing-elastic-beanstalk-environments).

#### 2 - Enable `patch_beanstalk_userdata` in AutoSpotting (optional) ####

Elastic Beanstalk leverages CloudFormation for creating resources and initializing
instances. When a new instance is launched, Elastic Beanstalk configures it through
the auto-scaling configuration (`UserData` and tags).

AutoSpotting launches spot instances outside of the auto-scaling group and attaches
them to the group after a grace period. As a result, the Elastic Beanstalk
initialization process can randomly fail or be delayed by 10+ minutes.
When it is delayed, the spot instances take a long time (10+ minutes) before being
initialized, appearing as healthy in Elastic Beanstalk and being added
to the load balancer.

As a solution, you can configure AutoSpotting to alter the Elastic Beanstalk
user-data so that the Elastic Beanstalk initialization process can run even
if the spot instance is not a part of the auto-scaling group.

To enable that option, set the `patch_beanstalk_userdata` variable to `true`
in your configuration.

You will also need to update the permissions of the role used by your instances
to authorize requests to the CloudFormation API. Add the `AutoSpottingElasticBeanstalk`
policy to the role `aws-elasticbeanstalk-ec2-role` or the custom instance profile/role
used by your Beanstalk instances.

The permissions contained in `AutoSpottingElasticBeanstalk` are required if you set
`patch_beanstalk_userdata` variable to `true`. If they are not added, your spot
instances will not be able to run correctly.

You can get more information on the need for this configuration variable and
the permissions in the [bug report](https://github.com/AutoSpotting/AutoSpotting/issues/344).

## Configuration of AutoSpotting ##

### Testing configuration ###

Normally AutoSpotting runs from AWS Lambda, but for testing purposes it can also
be compiled and executed locally as a command-line tool, which can be very
useful for troubleshooting, implementing and testing new functionality.

The algorithm can use custom command-line flags. Much like many other
command-line tools, you can use the `-h` command line flag to see all the
available options:

<!-- markdownlint-disable MD013 -->

``` text
$ ./AutoSpotting -h
Usage of ./AutoSpotting:
  -allowed_instance_types="":
        If specified, the spot instances will be of these types.
        If missing, the type is autodetected frome each ASG based on it's Launch Configuration.
        Accepts a list of comma or whitespace seperated instance types (supports globs).
        Example: ./AutoSpotting -allowed_instance_types 'c5.*,c4.xlarge'

  -bidding_policy="normal":
        Policy choice for spot bid. If set to 'normal', we bid at the on-demand price.
        If set to 'aggressive', we bid at a percentage value above the spot price configurable using the spot_price_buffer_percentage.

  -disallowed_instance_types="":
        If specified, the spot instances will _never_ be of these types.
        Accepts a list of comma or whitespace seperated instance types (supports globs).
        Example: ./AutoSpotting -disallowed_instance_types 't2.*,c4.xlarge'

  -min_on_demand_number=0:
        On-demand capacity (as absolute number) ensured to be running in each of your groups.
        Can be overridden on a per-group basis using the tag autospotting_min_on_demand_number.

  -min_on_demand_percentage=0:
        On-demand capacity (percentage of the total number of instances in the group) ensured to be running in each of your groups.
        Can be overridden on a per-group basis using the tag autospotting_min_on_demand_percentage
        It is ignored if min_on_demand_number is also set.

  -on_demand_price_multiplier=1:
        Multiplier for the on-demand price. This is useful for volume discounts or if you want to
        set your bid price to be higher than the on demand price to reduce the chances that your
        spot instances will be terminated.

  -regions="":
        Regions where it should be activated (comma or whitespace separated list, also supports globs), by default it runs on all regions.
        Example: ./AutoSpotting -regions 'eu-*,us-east-1'

  -spot_price_buffer_percentage=10:
        Percentage Value of the bid above the current spot price. A spot bid would be placed at a value :
        current_spot_price * [1 + (spot_price_buffer_percentage/100.0)]. The main benefit is that
        it protects the group from running spot instances that got significantly more expensive than
        when they were initially launched, but still somewhat less than the on-demand price. Can be
        enforced using the tag: autospotting_spot_price_buffer_percentage. If the bid exceeds
        the on-demand price, we place a bid at on-demand price itself.

  -spot_product_description="Linux/UNIX (Amazon VPC)":
        The Spot Product or operating system to use when looking up spot price history in the market.
        Valid choices: Linux/UNIX | SUSE Linux | Windows | Linux/UNIX (Amazon VPC) | SUSE Linux (Amazon VPC) | Windows (Amazon VPC)
  -spot_product_premium=0:
        The Product Premium to apply to the on demand price to improve spot
        selection and savings calculations when using a premium instance type
        such as RHEL.

  -tag_filters=[{spot-enabled true}]: Set of tags to filter the ASGs on.  Default is -tag_filters 'spot-enabled=true'
        Example: ./AutoSpotting -tag_filters 'spot-enabled=true,Environment=dev,Team=vision'

  -patch_beanstalk_userdata="true":
        Controls whether AutoSpotting patches Elastic Beanstalk UserData
        scripts to use the instance role when calling CloudFormation
        helpers instead of the standard CloudFormation authentication
        method
        Example: ./AutoSpotting --patch_beanstalk_userdata true
```

<!-- markdownlint-enable MD013 -->

The value of `-min_on_demand_number` has a higher priority than
`-min_on_demand_percentage`, so if you specify both options in the command line,
percentage will NOT be taken into account. It would be taken into account, ONLY
if the `-min_on_demand_number` is invalid (negative, above the max number, etc).

The value of `-regions` controls the scope within which autospotting run, this
is particularly useful when used during testing, in order to limit the scope of
action and reduce the risk when evaluating it or experimenting with new
functionality.

All the flags are also exposed as environment variables, expected in ALL_CAPS.
For example using the `-region` command-line flag is equivalent to using the
`REGION` environment variable.

When `tag_filters` is not passed, the default operation is to look for ASG's that
have the tag `spot-enabled=true`.   If you wish to narrow the operation of
autospotting to ASGs that match more specific criteria you can specify the matching
tags as you see fit.  i.e. `-tag_filters 'spot-enabled=true,Environment=dev,Team=vision'`

#### Note ####

* These configurations are also implemented when running from Lambda, where they
  are actually passed as environment variables set by CloudFormation in the
  Lambda function's configuration.
* The above list may not be up-to-date, please run it locally to see the latest
  list of supported flags, and if you notice any difference please report it in
  a Pull request.

### Running configuration ###

#### Minimum on-demand configuration ####

On top of the CLI configuration for the on-demand instances, autospotting
can read those values from the tags of the auto-scaling groups. There are two
available tags: `autospotting_min_on_demand_number` and
`autospotting_min_on_demand_percentage`.

Just like for the CLI configuration the defined number has a higher priority
than the percentage value. So the percentage will be ignored if
`autospotting_min_on_demand_number` is present and valid.

The order of priority from strongest to lowest for minimum on-demand
configuration is as following:

<!-- markdownlint-disable MD029 -->

1. Tag `autospotting_min_on_demand_number` in ASG
2. Tag `autospotting_min_on_demand_percentage` in ASG
3. Option `-min_on_demand_number` in CLI
4. Option `-min_on_demand_percentage` in CLI

<!-- markdownlint-enable MD029 -->

**Note:** the percentage does round up values. Therefore if we have for example
3 instances running in an autoscaling-group, and you specify 10%, autospotting
will understand that you want 0 instances. If you specify 16%, then it will
still understand that you want 0 instances, because `0.16 * 3` is equal to
`0.47999` so it is rounded down to 0; but if you specify 17%
(or more than 16.66667%) then the algorithm understands that you want at least
one instance (`0.17 * 3 = 0.51`). All in all it should work as you expect, but
this was just to explain some more the functionning of the percentage's math.

### Debugging ###

In certain situations you might want to add verbosity to the project in order
to understand a bit better what it's doing. If you want to do so please run it
with the following environment variable `AUTOSPOTTING_DEBUG`.

You can do it locally with some custom binary:

``` shell
 AUTOSPOTTING_DEBUG=true ./AutoSpotting
```

Or you can do it via the Lambda console under the `Environment variables`
section. Please note those variables aren't exposed via Cloudformation nor via
terraform.

Please attach the debug output when reporting any issues.

## Updates and Downgrades ##

The software doesn't auto-update, so you will need to manually perform updates
using CloudFormation, based on the Travis CI build number of the version you
would like to use going forward.

This method can be used both for upgrades and downgrades, so assuming you would
like to switch to the build with the number 45, you will need to perform a
CloudFormation stack update in which you change the "LambdaZipPath" stack
parameter to a value that looks like `nightly/lambda_build_45.zip`.

Git commit SHAs(truncated to 7 characters) are also accepted instead of the
build numbers, so for example `nightly/lambda_build_f7f395d.zip` should also be
a valid parameter, as long as that build is available in the author's S3 bucket.

The full list of the objects available in the bucket can be seen
[here](http://s3.amazonaws.com/cloudprowess/index.html).

The full list of TravisCI builds and their respective git commits can be seen on
the Travis CI [builds page](https://travis-ci.org/AutoSpotting/AutoSpotting/builds)

### Compatibility notices ###

* The CloudFormation template is also versioned for every build. Although the
  template rarely changes, it's recommended that you always keep it at the same
  build number as the binary.

## Uninstallation ##

If at some point you want to uninstall it, the AutoScaling groups where it used
to be enabled will keep running until their spot instances eventually get
outbid and terminated, then replaced by AutoScaling with on-demand ones. This
is eventually bringing the group to the initial state. If you want, you can
speed up the process by gradually terminating the spot instances yourself.

The tags set on the group can be deleted at any time you want it to be disabled
for that group.

### Uninstall via CloudFormation ###

You just need to delete the CloudFormation stack:

``` shell
 aws cloudformation delete-stack --stack-name AutoSpotting
```
