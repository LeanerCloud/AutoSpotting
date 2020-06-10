# AutoSpotting #

**CEASED ACTIVE DEVELOPMENT**

Hello,

I decided to stop actively working on AutoSpotting going forward.

I don't have the time to work on it and all my attempts to ramp up the income I get from it to eventually replace my full time job failed. I've also seen it repeatedly forked by companies who launched proprietary products based on it without any sort of contributions to the project, which also impacted my motivation to work on it as a hobby.

If you're using it you can keep running the current version as long as it works for you. Going forward I will keep creating stable builds occasionally for my Patreon supporters in case there are significant updates from other people, but I won't be actively working on new features for the forseable future. You can also consider migrating to some of the AWS native offerings or proprietary tools in this space.

If anyone is interested in taking over the development/maintenance work feel free to reach out on [gitter](https://gitter.im/cristim/autospotting) and/or this Github [issue](https://github.com/AutoSpotting/AutoSpotting/issues/431).

Huge thanks to everyone who supported development by purshasing stable build licenses on Patreon and to all the open source contributors who helped over the years.

Best regards,

Cristian

--------------------------

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

A simple and easy to use tool designed to significantly lower your Amazon AWS
costs by automating the use of [spot](https://aws.amazon.com/ec2/spot)
instances.

## Why? ##

We believe that AWS EC2 is often pricier than it should be, and that the pricing
models that can significantly reduce costs are hard to be reliably used by
humans and are better handled by automation.

There are many way to automate the use of spot instances, some offered out of
the box by AWS and some from third parties, each with its own characteristics
and drawbacks.

Unlike all those options, we developed a novel, simple but effective way to make
it much easier to **convert existing infrastructure within minutes, with minimal
configuration changes, negligible additional infrastructure and runtime costs,
safely and securely and without any vendor lock-in**.

Note: it's not necessarily the most cost-efficient for a given AutoScaling
group, although it does perform pretty well in practice. The main focus is the
**ease of adoption on large infrastructure** such as environments with hundreds
of AutoScaling groups or even hundreds of AWS accounts where it can be rolled
out and enabled in almost no-time compared to any other tool out there. Once you
tested it and you are confident with it, it can even be enabled against all the
groups from an AWS account without touching their configuration.

It also tries to be as cheap as possible to run, with **negligible runtime
costs** and being open source the software is **free of charge if you build it
from source**. In addition we offer inexpensive enterprise-grade support plans
that should barely be noticeable at scale, but just enough to support further
development, unlike similar commercial tools that claim a very significant chunk
of your savings.

This approach allows a large number of companies and individuals to
significantly reduce their infrastructure costs or get more bang for the same
buck. They can now easily get access to cheap compute capacity so they can spend
their scarce resources developing innovative products not paying for overpriced
compute capacity.

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
tags, but these overrides are often not needed.

A single installation can handle all enabled groups in parallel across all
available AWS regions, but can be restricted to fewer regions if desired.

Your groups will then monitor and use these spot instances just like they would
do with your on-demand instances. They will automatically join your load
balancer and start receiving traffic once passing the health checks, and the
traffic would automatically be drained on termination.

## What savings can I expect? ##

The savings it generates are often in the 60-80% range, but sometimes even up to
90%, like you can see in the graph below.

![Savings](https://autospotting.org/img/savings.png)

## What's under the hood? ##

The entire logic described above is implemented in a Lambda function deployed
using CloudFormation or Terraform stacks that can be installed and configured in
just a few minutes.

The stack assigns the function the minimal set of IAM permissions required for
it to work and requires no admin-like cross-account permissions. The entire code
base can be audited to see how these permissions are being used and even locked
down further if your audit discovers any issues. **This is not a SaaS**, there's
no component that calls home and reveals any details about your infrastructure.

The Lambda function is written in the Go programming language and the code is
compiled as a static binary compressed and uploaded to S3. For evaluation or
debugging purposes, the same binary can run out of the box locally on Linux
machines or as a Docker container on Windows or macOS. Some people even run
these containers on their existing Kubernetes clusters assuming the other
resources provided by the stack are implemented in another way on Kubernetes.

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
[Patreon](https://www.patreon.com/cristim/overview), regardless of the amount.
These donations will pay for hosting infrastructure of the easy to install
binaries and the project website, and will also encourage further development
by the main author.

Companies can also use the official stable and easy to install binaries (see
below for more details) and get support plans to support the development effort.

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

This software is distributed under the terms of the OSL-3.0 [license](LICENSE).

The official binaries are licensed under this proprietary
[license](BINARY_LICENSE).
