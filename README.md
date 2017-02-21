# AutoSpotting #

[![BuildStatus](https://travis-ci.org/cristim/autospotting.svg?branch=master)](https://travis-ci.org/cristim/autospotting)
[![GoReportCard](https://goreportcard.com/badge/github.com/cristim/autospotting)](https://goreportcard.com/report/github.com/cristim/autospotting)
[![CoverageStatus](https://coveralls.io/repos/github/cristim/autospotting/badge.svg?branch=master)](https://coveralls.io/github/cristim/autospotting?branch=master)
[![CodeClimate](https://codeclimate.com/github/cristim/autospotting/badges/gpa.svg)](https://codeclimate.com/github/cristim/autospotting)
[![IssueCount](https://codeclimate.com/github/cristim/autospotting/badges/issue_count.svg)](https://codeclimate.com/github/cristim/autospotting)
[![ChatOnGitter](https://badges.gitter.im/cristim/autospotting.svg)](https://gitter.im/cristim/autospotting?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge)

A simple and easy to use tool designed to significantly lower your Amazon AWS
costs by automating the use of the [spot
market](https://aws.amazon.com/ec2/spot).

Once enabled on an existing on-demand AutoScaling group, it launches an EC2 spot
instance that is cheaper, at least as large and configured identically to your
current on-demand instances. As soon as the new instance is ready, it is added
to the group and an on-demand instance is detached from the group
and terminated.

It continuously applies this process, gradually replacing any on-demand
instances with spot instances until the group only consists of spot instances,
but it can also be configured to keep some on-demand instances running.

All this can be seen in action below.

![Workflow](https://cdn.cloudprowess.com/images/autospotting.gif)

Read [here](./TECHNICAL_DETAILS.md) for more detailed information.

## Getting Started ##

Just like in the above animation, it's as easy as running a CloudFormation stack
and setting one or more tags on your AutoScaling group. It should only take a
few minutes until you can start saving.

[![Launch](https://s3.amazonaws.com/cloudformation-examples/cloudformation-launch-stack.png)](https://console.aws.amazon.com/cloudformation/home?region=us-east-1#/stacks/new?stackName=AutoSpotting&templateURL=https://s3.amazonaws.com/cloudprowess/dv/template.json)

For more detailed information you can read this [document](./START.md)

## Compiling and Installing ##

Even though you should normally be fine with the provided binaries, in case you
have some special needs it's relatively easy to build and install your own
customized binaries. More details [here](./SETUP.md)

## Contributing ##

This project was developed by volunteers in their own spare time, so if it makes
a difference on your company's bottom line, please consider giving something
back to ensure further development. You can do it by suggesting improvements,
contributing some code or documentation, allowing some of your employees to work
on it for a few hours per week, or even just spreading the word.

The usual GitHub contribution model applies, but if you would like to raise an
issue or start working on a pull request, please get in touch on
[gitter](https://gitter.im/cristim/autospotting) to discuss it first. Any
random questions are also better asked on gitter.

Bug reports should contain enough details to be reproduced, see
[#83](https://github.com/cristim/autospotting/issues/83): for a relatively good
example:

- build number or git commit in case of custom builds
- error stack trace
- anonymized launch configuration dump,
- AWS region
- environment type (VPC/EC2Classic/DefaultVPC)

Feature requests should explain the issue in detail, and should also be
discussed on gitter to make sure nothing was lost in translation.

The code contributions need to provide unit tests for the functionality they
are creating or changing in a meaningful way.

Thanks!
