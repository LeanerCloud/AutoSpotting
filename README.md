# AutoSpotting #

<!-- markdownlint-disable MD033 -->

<img src="logo.png" width="150" align="right">

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

Read [here](TECHNICAL_DETAILS.md) for more information and implementation
details.

## Getting Started ##

Just like in the above animation, it's as easy as launching a CloudFormation (or
[Terraform](https://github.com/cristim/autospotting/tree/master/terraform))
stack and setting one or more tags on your AutoScaling group. It should only
take a few minutes to get started.

[![Launch](https://s3.amazonaws.com/cloudformation-examples/cloudformation-launch-stack.png)](https://console.aws.amazon.com/cloudformation/home?region=us-east-1#/stacks/new?stackName=AutoSpotting&templateURL=https://s3.amazonaws.com/cloudprowess/dv/template.json)

For more detailed information you can read this [document](START.md)

## Compiling and Installing ##

Even though you should normally be fine with the provided binaries, for local
development or in case you have some special needs it's relatively easy to
build, run or install your customized binaries.

More details are available [here](CUSTOM_BUILDS.md)

## Contributing ##

This project was developed by volunteers in their own spare time. If you find it
useful please consider contributing to its development, any help would be
greatly appreciated.

You can do it by trying it out and giving feedback, reporting bugs, writing
code, improving the documentation, assigning a colleague to work on it for a few
hours a week, spreading the word or simply
[contacting](https://gitter.im/cristim/autospotting) us and telling about your
setup.

Non-trivial work should be submitted according to the contribution
[guidelines](CONTRIBUTING.md)

## License ##

This software is distributed under the terms of the MIT [license](LICENSE).
