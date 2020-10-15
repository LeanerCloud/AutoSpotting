# AutoSpotting #
<!-- markdownlint-disable MD003 MD026 MD033 -->

<img src="logo.png" width="150" align="right">

[![BuildStatus](https://travis-ci.org/AutoSpotting/AutoSpotting.svg?branch=master)](https://travis-ci.org/AutoSpotting/AutoSpotting)
[![GoReportCard](https://goreportcard.com/badge/github.com/AutoSpotting/AutoSpotting)](https://goreportcard.com/report/github.com/AutoSpotting/AutoSpotting)
[![CoverageStatus](https://coveralls.io/repos/github/AutoSpotting/AutoSpotting/badge.svg?branch=master)](https://coveralls.io/github/AutoSpotting/AutoSpotting?branch=master)
[![CodeClimate](https://codeclimate.com/github/AutoSpotting/AutoSpotting/badges/gpa.svg)](https://codeclimate.com/github/AutoSpotting/AutoSpotting)
[![IssueCount](https://codeclimate.com/github/AutoSpotting/AutoSpotting/badges/issue_count.svg)](https://codeclimate.com/github/AutoSpotting/AutoSpotting)
[![ChatOnGitter](https://badges.gitter.im/AutoSpotting/AutoSpotting.svg)](https://gitter.im/cristim/autospotting)
[![Open Source Helpers](https://www.codetriage.com/AutoSpotting/AutoSpotting/badges/users.svg)](https://www.codetriage.com/AutoSpotting/AutoSpotting)
[![Patreon](https://img.shields.io/badge/patreon-donate-yellow.svg)](https://www.patreon.com/cristim/overview)

AutoSpotting is the leading open source spot market automation tool, optimized towards quick/easy/frictionless adoption of the EC2 spot market at any scale.

It takes over existing long-running ASGs with minimal configuration changes(usually just tagging them, but even that can be avoided), yielding the usual 70%-90% cost savings on compute but in a better integrated, more cost effective and easier to adopt way than the commercial SaaS alternatives in this space and for some mass adoption use cases it's even a better fit than the current native AWS offerings.

## How does it work? ##

Once installed and enabled (usually by tagging) on existing on-demand AutoScaling groups,
AutoSpotting gradually replaces their on-demand instances with cheaper spot instances
that are at least as large and identically configured to
the group's members, without changing the group launch configuration in any way. For
your peace of mind, you can also keep running a configurable number of on-demand
instances given as percentage or absolute number and it automatically fails over to on-demand in case of spot instance terminations.

Going forward, as well as on any new ASGs that match the expected tags, any new on-demand instances above the amount configured to be keept will be immediately replaced with spot clones within seconds of being launched. If this fails due to insufficient spot capacity, the usual cron events will replace them later once spot capacity becomes available again. When launching Spot instances, the compatible instance types are attempted in increasing order of their price, until one is successfully launched.

This process can partly be seen in action below, you can click to expand the animation:

![Workflow](https://autospotting.org/img/autospotting.gif)

Additionally, it implements some complex logic aware of spot and on demand prices, including
for different spot products and configurable discounts for reserved instances or
large volume customers. It also considers the specs of all instance types and
automatically places bids to instance types and prices chosen based on flexible
configuration set globally or overridden at the group level using additional
tags, but these overrides are often not needed.

A single installation can handle all enabled groups fronm an AWS account in parallel across all
available AWS regions, but it can be restricted to fewer regions if desired.

Your groups will then monitor and use these spot instances just like they would
do with your on-demand instances. They will automatically join your load
balancer and start receiving traffic once passing the health checks, and the
traffic would automatically be drained on termination.

## What savings can I expect? ##

The savings it generates are often in the 60-80% range, depending on region and instance type, but sometimes even up to
90% of on-demand prices, like you can see in the graph below.

![Savings](https://autospotting.org/img/savings.png)

## What's under the hood? ##

The entire logic described above is implemented in a set of Lambda functions deployed
using CloudFormation or Terraform stacks that can be installed and configured in
just a few minutes.

The stack assigns them the minimal set of IAM permissions required for
them to work and requires no admin-like cross-account permissions. The entire code
base can be audited to see how these permissions are being used and even locked
down further if your audit discovers any issues. **This is not a SaaS**, there's
no component that calls home and reveals any details about your infrastructure.

The main Lambda function is written in the Go programming language and the code is
compiled as a static binary compressed and uploaded to S3. For evaluation or
debugging purposes, the same binary can run out of the box locally on Linux
machines or as a Docker container on Windows or macOS. Some people even run
these containers on their existing Kubernetes clusters assuming the other
resources provided by the stack are implemented in another way on Kubernetes.

The stack also consists of a few CloudWatch event triggers, that run the Lambda
function periodically and whenever it needs to take action against the enabled groups.
Between runs your group is entirely managed by AutoScaling (including any scaling policies
you may have) and load balancer health checks, that can trigger instance
launches or replacements using the original on-demand launch configuration.

Read [here](TECHNICAL_DETAILS.md) for more information and implementation
details.

## FAQs ##

Frequently asked questions about the project are answered in the
[FAQ](https://autospotting.org/faq/index.html), *please read this first before
asking for support*.

If you have additional questions not covered there, they can be easily added to
the
[source](https://github.com/AutoSpotting/autospotting.org/blob/master/content/faq.md)
of the FAQ by editing in the browser and creating a pull request, and we'll
answer them while reviewing the pull request.

## Getting Started ##

Just like in the above animation, it's as easy as launching a CloudFormation (or
[Terraform](https://github.com/AutoSpotting/terraform-aws-autospotting))
stack and setting the (configurable) `spot-enabled` tag on the AutoScaling
groups where you want it enabled to `true`.

All the required infrastructure and configuration will be created automatically,
so you can get started as fast as possible.

For more detailed information you can read this [document](START.md)

[![Launch](https://s3.amazonaws.com/cloudformation-examples/cloudformation-launch-stack.png)](https://autospotting.org)

### Note ###

- the binaries launched by this stack are distributed under a proprietary
  [license](BINARY_LICENSE), and are **free to use for evaluation, up to $1000
  monthly savings**. Once you reach this limit you'll need to either switch to the
  inexpensive supported binaries (designed to cost a small fraction of around
  1% of your total savings for supporting further development), or you can build
  your own binaries based on the open source code and run it for free.

## Support ##

Community support is available on the
[gitter](https://gitter.im/cristim/autospotting) chat room, where the main
authors and other users are likely to help you solve issues.

Note: This is offered on a best effort basis and under certain conditions, such
as using the latest version of the evaluation binaries.

If you need more comprehensive support you will need to purchase a support plan.

## Contributing ##

Unlike multiple commercial products in this space that cost a lot of money and
attempt to lock you in, this project is fully open source and developed in the
open by a vibrant community of dozens of contributors.

We urge you to support us on [Github Sponsors](https://github.com/sponsors/cristim)
if this software helps you save any significant amount of money, this will greatly help
further development.

Financial sponsorship is voluntary, it's also fine if you just try it out and give
[feedback](https://gitter.im/cristim/autospotting), report issues, improve the
documentation, write some code or assign a developer to work on it, or even just
spread the word among your peers who might be interested in it. Any sort of
support would be greatly appreciated and would make a huge difference
to the project.

Note: Non-trivial code should be submitted according to the contribution
[guidelines](CONTRIBUTING.md).

### Proprietary binaries ###

The source code is and will always be open source, so you can build and run
it yourself, see how it works and even enhance it if you want.

But if you want to conveniently get started or update within minutes without
setting up and maintaining a build environment or any additional infrastructure,
we have pre-built evaluation binaries that will save you significant
amounts of time and effort.

These can be used for evaluation purposes as long as the generated monthly
savings are less than $1000. Once you reach this level you will need to either
purchase an inexpensive stable build that doesn't have this limitation, and also
comes with a support plan, or you can build AutoSpotting from source code.

The support license costs vary by group, region and AWS account coverage and can
also be paid through [Patreon](https://www.patreon.com/cristim/overview).

Individuals and companies supporting the development of the open source code get
free of charge support and stable build access for a year since their latest
contribution to the project.

#### Note: ####

- even though these evaluation builds are usually stable enough, they may
  not have been thoroughly tested yet and come with best effort community
  support.
- the docker images available on DockerHub are also distributed under the
  same binary license and the costs are the same.

### Stable builds ###

Carefully tested builds suitable for Enterprise use will be communicated
to [Patreon](https://www.patreon.com/cristim/overview) backers as soon as
they join.

They come with support from the author, who will do his best to help you
successfully run AutoSpotting on your environment so you can get the most out of
it. The feature requests and issues will also be prioritized based on the
Patreon tier.

Please get in touch on [gitter](https://gitter.im/cristim) if you have any
questions about these stable builds.

## Compiling and Installing ##

It is recommended to use the evaluation or stable binaries, which are easy to
install, support further development of the software and allow you to get
support.

But if you have some special needs that require some customizations or you don't
want to rely on the author's infrastructure or contribute anything for longer
term use of the software, you can always build and run your customized binaries
that you maintain on your own, just keep in mind that those won't be supported
in any way.

More details are available [here](CUSTOM_BUILDS.md)

## Users ##

Autospotting is already used by hundreds of individuals and organizations around
the world and as per internal AWS information a significant(but undisclosed) 
percentage of all currently running spot instances were launched using it.

## License ##

This software is distributed under the terms of the OSL-3.0 [license](LICENSE).

The official binaries are licensed under this proprietary
[license](BINARY_LICENSE).
