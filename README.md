# AutoSpotting #

<!-- markdownlint-disable MD003 MD026 MD033 -->

<img src="logo.png" width="150" align="right">

[![BuildStatus](https://travis-ci.org/cristim/autospotting.svg?branch=master)](https://travis-ci.org/cristim/autospotting)
[![GoReportCard](https://goreportcard.com/badge/github.com/cristim/autospotting)](https://goreportcard.com/report/github.com/cristim/autospotting)
[![CoverageStatus](https://coveralls.io/repos/github/cristim/autospotting/badge.svg?branch=master)](https://coveralls.io/github/cristim/autospotting?branch=master)
[![CodeClimate](https://codeclimate.com/github/cristim/autospotting/badges/gpa.svg)](https://codeclimate.com/github/cristim/autospotting)
[![IssueCount](https://codeclimate.com/github/cristim/autospotting/badges/issue_count.svg)](https://codeclimate.com/github/cristim/autospotting)
[![ChatOnGitter](https://badges.gitter.im/cristim/autospotting.svg)](https://gitter.im/cristim/autospotting?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge)
[![Open Source Helpers](https://www.codetriage.com/cristim/autospotting/badges/users.svg)](https://www.codetriage.com/cristim/autospotting)
[![Patreon](https://img.shields.io/badge/patreon-donate-yellow.svg)](https://www.patreon.com/bePatron?c=979085)

A simple and easy to use tool designed to significantly lower your Amazon AWS
costs by automating the use of [spot](https://aws.amazon.com/ec2/spot)
instances.

## Why? ##

We believe that AWS EC2 is often pricier than it should be, and that the pricing
models that can significantly reduce costs are hard to be reliably used by
humans and are better handled by automation.

We developed a novel, simple but effective way to make it much more affordable
for a significant number of existing setups within minutes, with minimal
configuration changes, negligible additional infrastructure and runtime costs,
safely and securely and without any vendor lock-in.

This already allows a large number of companies and individuals to significantly
reduce their infrastructure costs or get more bang for the same buck. They can
now easily get access to cheap compute capacity so they can spend their scarce
resources developing innovative products that hopefully make the world a better
place.

## How does it work? ##

Once installed and enabled by tagging existing on-demand AutoScaling groups,
AutoSpotting gradually replaces their on-demand instances with spot instances
that are usually much cheaper, at least as large and identically configured to
the group's members, without changing the group configuration in any way. For
your peace of mind, you can also keep running a configurable number of on-demand
instances given as percentage or absolute number.

This can be seen in action below, you can click to expand the animation:

![Workflow](https://autospotting.org/img/autospotting.gif)

It implements some complex logic aware of spot and on demand prices, including
for different spot products and configurable discounts for reserved instances or
large volume customers. It also considers the specs of all instance types and
automatically places bids to instance types and prices chosen based on flexible
configuration set globally or overridden at the group level using additional
tags.

A single installation can handle all enabled groups in parallel across all
available AWS regions, but can be restricted to fewer regions if desired.

Your groups will then monitor and use these spot instances just like they would
do with your on-demand instances. They will automatically join your load
balancer and start receiving traffic once passing the health checks.

## What? ##

The savings it generates are often in the 60-80% range, but sometimes even up to
90%, like you can see in the graph below.

![Savings](https://autospotting.org/img/savings.png)

The entire logic described above is implemented in a Lambda function deployed
using CloudFormation or Terraform stacks that can be installed and configured in
just a few minutes.

The stack assigns the function the minimal set of IAM permissions required for
it to work and has no admin-like cross-account permissions. The entire code base
can be audited to see how these permissions are being used and even locked down
further if your audit discovers any issues. This is not a SaaS, there's no
component that calls home and reveals details about your infrastructure.

The Lambda function is written in the Go programming language and the code is
compiled as a static binary compressed and uploaded to S3. For evaluation or
debugging purposes, the same binary can run out of the box locally on Linux
machines or as a Docker container. Some people even run these containers on
their existing Kubernetes clusters assuming the other resources provided by the
stack are implemented in another way on Kubernetes.

The stack also consists of a Cron-like CloudWatch event, that runs the Lambda
function periodically to take action against the enabled groups. Between runs
your group is entirely managed by AutoScaling (including any scaling policies
you may have) and load balancer health checks, that can trigger instance
launches or replacements using the original on-demand launch configuration.
These instances will be replaced later by better priced spot instances when they
are available on the spot market.

Read [here](TECHNICAL_DETAILS.md) for more information and implementation
details.

## FAQs ##

Frequently asked questions about the project are answered in the [FAQ](FAQ.md),
*please read this first before asking for support*.

If you have additional questions not covered there, they can be easily added to
the [crowdsourced source of the FAQ](https://etherpad.net/p/AutoSpotting_FAQ)
and we'll do our best to answer them either there or on Gitter.

## Getting Started ##

Just like in the above animation, it's as easy as launching a CloudFormation (or
[Terraform](https://github.com/cristim/autospotting/tree/master/terraform))
stack and setting the (configurable) `spot-enabled` tag on the AutoScaling
groups where you want it enabled to `true`.

All the required infrastructure and configuration will be created automatically,
so you can get started as fast as possible.

For more detailed information you can read this [document](START.md)

[![Launch](https://s3.amazonaws.com/cloudformation-examples/cloudformation-launch-stack.png)](https://www.patreon.com/bePatron?c=979085)

### Note ###

- the binaries launched by this stack are distributed under a proprietary
  [license](BINARY_LICENSE), see the Official Binaries section below for more
  details.
- later on, if you're confident enough to roll it out on all your groups by
  default, it can also be configured in an `opt-out` mode, in which it runs
  against all groups except for those tagged with the (configurable)
  `spot-enabled` tag set to `false`. This can be very useful for large scale
  Enterprise rollouts against lots of AWS accounts, where you can migrate to
  spot with minimal buy-in or effort from the account maintainers.

## Support ##

Community support is available on the
[gitter](https://gitter.im/cristim/autospotting) chat room, where the main
authors and other users are likely to help you solve issues with these official
binaries.

## Contributing ##

Unlike multiple commercial products in this space that cost a lot of money and
attempt to lock you in, this project is fully open source and developed in the
open by a vibrant community.

It was largely developed by volunteers who spent countless hours of their own
spare time to make it easy for you to use. If you find it useful and you
appreciate the work they put in it, please consider contributing to the
development effort as well.

You can just try it out and give
[feedback](https://gitter.im/cristim/autospotting), report issues, improve the
documentation, write some code or assign a developer to work on it, or even just
spread the word among your peers who might be interested in it. Any amount of
help would be greatly appreciated and would make a huge difference to the
project.

You can also contribute financially, we gladly accept recurrent tips on
[Patreon](https://www.patreon.com/bePatron?c=979085), regardless of the amount.
These donations will pay for hosting infrastructure of the easy to install
binaries and the project website, and will also encourage further development
by the main author.

If you can, try also convincing your organization to use the official easy to
install binaries (see below for more details).

Note: Non-trivial code should be submitted according to the contribution
[guidelines](CONTRIBUTING.md).

### Official binaries ###

The source code is and will always be open source, so you can build and run
it yourself, see how it works and even enhance it if you want.

But if you want to conveniently get started or update within minutes without
setting up and maintaining a build environment or any additional
infrastructure, we have proprietary pre-built binaries that will save you
significant amounts of time and effort.

These are free of charge for individuals and non-profits. Companies can use
them free of charge for up to 14 days, but will need to purchase a license
in order to legally use them for longer time.

Companies supporting the development of the open source code can use it free
of charge for a year since their latest contribution to the project.

The license costs $40 monthly for each AWS account and can be paid through
[Patreon](https://www.patreon.com/bePatron?c=979085).

This income motivates the main author to allocate a proportional amount
of time on a monthly basis for improving the software and supporting users
like you.

#### Note: ####

- even though these builds are usually stable enough, they may not have been
  thoroughly tested yet and come with best effort community support.

### Stable binaries ###

Carefully tested builds suitable for Enterprise use are also available for
a custom fee you can negotiate with the original author, also paid monthly
through [Patreon](https://www.patreon.com/bePatron?c=979085).

They come with support from the author, who will do his best to help you
successfully run AutoSpotting on your environment so you can get the most
out of it. Your feature requests and issues will also be treated with high
priority.

Please get in touch on [gitter](https://gitter.im/cristim) if you are
interested in getting access to these stable builds.

## Compiling and Installing ##

It is recommended to use the official or stable binaries, which are easy to
install, support further development of the software and allow you to get
Enterprise support.

But if you have some special needs that require some customizations or you don't
want to rely on the author's infrastructure or contribute anything for longer
term use of the software, you can always build and run your customized binaries
that you maintain on your own, just keep in mind that those won't be supported
in any way.

More details are available [here](CUSTOM_BUILDS.md)

## Users ##

Autospotting is already used by hundreds of individuals and organizations around
the world, and we estimate to already save them in the millions of dollars. Some
of them we know of are mentioned in the [list](USERS.md) of notable users.

The following deserve a special mention for contributing significantly to the
development effort (listed in alphabetical order):

- www.branch.io
- www.cs.utexas.edu
- www.cycloid.io
- www.here.com
- www.spscommerce.com
- www.timber.io

## License ##

This software is distributed under the terms of the MIT [license](LICENSE).

The official binaries are licensed under this proprietary
[license](BINARY_LICENSE).
