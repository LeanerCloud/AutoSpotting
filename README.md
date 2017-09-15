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
costs by automating the use of [spot](https://aws.amazon.com/ec2/spot) instances.

![Savings](https://cdn.cloudprowess.com/images/savings.png)

## How does it work?

When installed and enabled on an existing on-demand AutoScaling group, 
AutoSpotting clones one of your on-demand instances from the group with a spot 
instance that is cheaper, at least as large (automatically considering memory,
CPU cores and disk volumes) and configured identically to it. Once the new spot
instance is ready, it is attached to the group and an on-demand instance is
detached and terminated, to keep the group at constant capacity.    

It continuously applies this process, across all enabled groups from all regions,
gradually replacing your on-demand instances with much cheaper spot instances.
For your peace of mind, you can also configure it to keep running a configurable
number of on-demand instances, given as percentage or absolute number.

Your groups will then monitor and use these spot instances just like it would 
do with your on-demand ones. They will automatically join your load balancer
and start receiving traffic once passing the health checks.

The installation takes just a few minutes and the existing groups can be enabled
and configured individually by using a few additional tags.

This can be seen in action below.

![Workflow](https://cdn.cloudprowess.com/images/autospotting.gif)

Read [here](TECHNICAL_DETAILS.md) for more information and implementation
details.

Frequently asked questions about the project are answered in the [FAQ](FAQ.md),
*please read this first before asking for support*.

If you have additional questions not covered there, they can be easily added to
the [crowdsourced source of the FAQ](https://etherpad.net/p/AutoSpotting_FAQ)
and we'll do our best to answer them either there or on Gitter.

## Getting Started ##

Just like in the above animation, it's as easy as launching a CloudFormation (or
[Terraform](https://github.com/cristim/autospotting/tree/master/terraform))
stack and setting one or more tags on your AutoScaling group.

[![Launch](https://s3.amazonaws.com/cloudformation-examples/cloudformation-launch-stack.png)](https://console.aws.amazon.com/cloudformation/home?region=us-east-1#/stacks/new?stackName=AutoSpotting&templateURL=https://s3.amazonaws.com/cloudprowess/dv/template.json)

For more detailed information you can read this [document](START.md)

## Compiling and Installing ##

Even though you should normally be fine with the provided binaries, for local
development or in case you have some special needs it's relatively easy to
build and run your customized binaries.

More details are available [here](CUSTOM_BUILDS.md)

## Contributing ##

This project was developed by volunteers in their own spare time. If you find it
useful please consider contributing to its development, any help would be
greatly appreciated.

You can do it by trying it out and giving feedback, reporting bugs, writing
code, improving the documentation, assigning someone to work on it for a few
hours a week, spreading the word or simply
[contacting](https://gitter.im/cristim/autospotting) us and telling about your
setup.

Non-trivial code should be submitted according to the contribution
[guidelines](CONTRIBUTING.md)

## Support ##

Community support is available on the
[gitter](https://gitter.im/cristim/autospotting) chat room on a best effort
basis.

The main author also offers enterprise-grade support, feature development
as well as AWS-related consulting for a fee. For more information feel free 
to [get in touch on gitter](https://gitter.im/cristim).

## Users ##

Autospotting is already used by hundreds of individuals and companies around the
world, such as:

- www.cycloid.io
- www.fractalanalytics.com
- www.here.com
- www.ibibogroup.com
- www.icap.com
- www.news.co.uk
- www.parkassist.com
- www.qualcomm.com
- www.quantiphi.com
- www.realestate.co.nz
- www.roames.com
- www.spscommerce.com
- www.taitradio.com

## License ##

This software is distributed under the terms of the MIT [license](LICENSE).
