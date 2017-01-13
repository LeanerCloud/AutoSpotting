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
  * [Updates and Downgrades](#updates-and-downgrades)
    * [Compatibility notices](#compatibility-notices)
  * [Uninstallation](#uninstallation)
    * [Uninstall via cloudformation](#uninstall-via-cloudformation)
    * [Uninstall via terraform](#uninstall-via-terraform)

## Requirements ##

* You will need credentials to an AWS account able to start CloudFormation
  stacks.
* Some of the following steps assume you have the AWS cli tool installed, but
  the setup can also be done manually using the AWS console or using other tools
  able to launch CloudFormation stacks and set tags on AutoScaling groups.

## Installation ##

### Installation options ###

Autospotting can be installed via Cloudformation or Terraform, both install
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
console or in the variables.tf file.

In case you may want to change some of them later, you can do it at any time by
updating the stack via Cloudformation or Terraform.

Note: even though the Cloudformation stack template is not changing so often
and it may often support multiple software versions, due to possible
compatibility issues, it is recommended to also update the stack template when
updating the software version.

### Install via cloudformation ###

To install it via cloudformation, you only need to launch a CloudFormation
stack in your account. Click the button below and follow the launch wizard to
completion, you can safely use the default stack parameters.

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
  are processed at runtime, unless configured otherwise.

### Install via terraform ###

To install it via terraform, you need to have
[terraform installed](https://www.terraform.io/downloads.html) on your machine.
If you are only using autospotting as such, you can install the stack by doing:

```
 cd terraform/
 terraform get # in order for terraform to get the module
 export AWS_DEFAULT_REGION=XXXX
 export AWS_ACCESS_KEY_ID=XXXX
 export AWS_SECRET_ACCESS_KEY=XXXX
 terraform apply
```

To use custom parameters, please refer to the `variables.tf` in `terraform/`.
Here is an example modifying both autospotting and lambda configuration:

```bash
 terraform apply \
   -var asg_regions_enabled="eu*,us*" \
   -var asg_min_on_demand_percentage="33.3" \
   -var lambda_memory_size=1024
```

If you are using autospotting integrated to your infrastructure, then you can
use the module directly:

```
module "autospotting" {
  source = "github.com/cristim/autospotting/terraform/autospotting"

  autospotting_min_on_demand_number = "0"
  autospotting_min_on_demand_percentage = "50.0"
  autospotting_regions_enabled = "eu*,us*"

  lambda_zipname = "./my-autospotting-build.zip"
  lambda_runtime = "200"
  lambda_memory_size = "2048"
  lambda_timeout = "600"
  lambda_run_frequency = "rate(1 minutes)"
}
```

Note: Apart from AWS variable, no variables are required. The module can be run
as such, and would function. But you might want to tweak at least the default
on-demand values and/or the regions in which autospotting runs.
The extra parameters can also be overridden to suit your needs.

## Enable autospotting ##

### For an AutoScaling group ###

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

**Note:** the above instructions use the eu-west-1 AWS region as an example. Depending
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

### For Elastic Beanstalk ###

* In order to add tags to existing Elastic Beanstalk environment, you will
  need to rebuild the environment with the spot-enabled tag. Follow this
  [guide](http://www.boringgeek.com/add-or-update-tags-on-existing-elastic-beanstalk-environments)

## Configuration of autospotting ##

### Testing configuration ###

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

### Running configuration ###

#### Minimum on-demand configuration ####

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

**Note:** the percentage does round up values. Therefore if we have for example
3 instances running in an autoscaling-group, and you specify 10%, autospotting
will understand that you want 0 instances. If you specify 16%, then it will
still understand that you want 0 instances, because `0.16 * 3` is equal to
`0.47999` so it is rounded down to 0; but if you specify 17%
(or more than 16.66667%) then the algorithm understands that you want at least
one instance (`0.17 * 3 = 0.51`). All in all it should work as you expect, but
this was just to explain some more the functionning of the percentage's math.

## Updates and Downgrades ##

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

### Compatibility notices ###

* As of build 79 the CloudFormation template is also versioned for every
  subsequent build, but unfortunately **this build also breaks compatibility
  with older stacks**. If you run an older build you will also need to update
  the stack when updating to a build later than 79. Although the template rarely
  changes, it's recommended that you always keep it at the same build as the
  binary. Make sure you use the following stack parameter on any newer builds:

```
LambdaHandlerFunction: handler.handle
```

## Uninstallation ##

If at some point you want to uninstall it, the AutoScaling groups where it used
to be enabled will keep running until their spot instances eventually get
outbid and terminated, then replaced by AutoScaling with on-demand ones. This
is eventually bringing the group to the initial state. If you want, you can
speed up the process by gradually terminating the spot instances yourself.

The tags set on the group can be deleted at any time you want it to be
disabled for that group.

### Uninstall via cloudformation ###

You just need to delete the CloudFormation stack:

```
 aws cloudformation delete-stack --stack-name AutoSpotting
```

### Uninstall via terraform ###

You just need to delete the elements via terraform:

```
 cd terraform/
 echo "yes" | terraform destroy
```