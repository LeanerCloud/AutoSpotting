# IMPORTANT #

As of 26 Sep 2022, I decided to postpone publishing further code changes to this repo until I reach my sustainability goals.

Further changes will be closed source and only available to my Marketplace subscribers until further notice.

See [here](https://github.com/cloudutil/AutoSpotting/discussions/489) for further information about this change and what can you do about it.

Thank you for your understanding and your support on this matter.

Cristian

<!-- markdownlint-disable MD003 MD026 MD033 -->

## AutoSpotting ##

<img src="logo.png" width="150" align="right">

[![BuildStatus](https://travis-ci.org/AutoSpotting/AutoSpotting.svg?branch=master)](https://travis-ci.org/AutoSpotting/AutoSpotting)
[![GoReportCard](https://goreportcard.com/badge/github.com/AutoSpotting/AutoSpotting)](https://goreportcard.com/report/github.com/AutoSpotting/AutoSpotting)
[![CoverageStatus](https://coveralls.io/repos/github/AutoSpotting/AutoSpotting/badge.svg?branch=master)](https://coveralls.io/github/AutoSpotting/AutoSpotting?branch=master)
[![CodeClimate](https://codeclimate.com/github/AutoSpotting/AutoSpotting/badges/gpa.svg)](https://codeclimate.com/github/AutoSpotting/AutoSpotting)
[![IssueCount](https://codeclimate.com/github/AutoSpotting/AutoSpotting/badges/issue_count.svg)](https://codeclimate.com/github/AutoSpotting/AutoSpotting)
[![ChatOnGitter](https://badges.gitter.im/AutoSpotting/AutoSpotting.svg)](https://gitter.im/cristim/autospotting)
[![Patreon](https://img.shields.io/badge/patreon-donate-yellow.svg)](https://www.patreon.com/cristim/overview)

AutoSpotting is the leading open source spot market automation tool, optimized
towards quick/easy/frictionless adoption of the EC2 spot market at any scale.

It's being used by thousands of users around the world, from companies of all
shapes and sizes, in aggregate probably saving them in the tens of millions of
dollars monthly as per our current estimations.

It is usually set up to monitor existing long-running AutoScaling groups,
replacing their instances with Spot instances with minimal configuration
changes.

Often all it needs is just tagging them with `spot-enabled=true`, (in some cases
even that can be avoided), yielding the usual 70%-90% Spot cost
savings but in a better integrated and easier-to-adopt way
than other alternative tools and solutions.

It is particularly useful if you have a large footprint that you want to migrate
to Spot quickly due to management pressure but with minimal effort and configuration
changes.

## Guiding principles ##

- Customer-focused, designed to maximize user benefits and reduce adoption friction
- Safe and secure, hosted in your AWS account and with the minimal required set of IAM permissions
- Auditable OSS code base developed in the open
- Inexpensive, easy to install, and supported builds offered through the AWS Marketplace
- Simple, minimalist implementation

## How does it work? ##

Once installed and enabled to run against existing on-demand
AutoScaling groups, AutoSpotting gradually replaces their on-demand instances
with cheaper spot instances that are at least as large and identically
configured to the group's members, without changing the group launch
configuration in any way. You can also keep running a configurable number of
on-demand instances given as a percentage or an absolute number and it automatically
fails over to on-demand in case of spot instance terminations.

Going forward, as well as on any new ASGs that match the expected tags, any new
on-demand instances above the amount configured to be kept running will be immediately
replaced with spot clones within seconds of being launched.

If this fails temporarily due to insufficient spot capacity, AutoSpotting will
continuously attempt to replace them every few minutes until successful after
spot capacity becomes available again.

When launching Spot instances, the compatible instance types are chosen by
default using a the
[capacity-optimized-prioritized](https://docs.amazonaws.cn/en_us/AWSEC2/latest/UserGuide/ec2-fleet-examples.html#ec2-fleet-config11)
allocation strategy, which is given a list of instance types sorted by price. This
configuration offers a good tradeoff between low cost and significantly reduced
interruption rates. The lowest-priced allocation strategy is still available as a
configuration option.

This process can partly be seen in action below, you can click to expand the animation:

![Workflow](https://autospotting.org/img/autospotting.gif)

A single installation can handle all enabled groups from an entire AWS account in
parallel across all available AWS regions, but it can be restricted to fewer
regions if desired in certain situations.

Your groups will then monitor and use these Spot instances just like they would
do with your on-demand instances. They will automatically join their respective
load balancer and start receiving traffic once passing the health checks, and
the traffic would automatically be drained on termination.

## What savings can I expect? ##

The savings it generates are in the 60-90% range usually seen when using spot
instances, but they may vary depending on region and instance type.

## What's under the hood? ##

The entire logic described above is implemented in a set of Lambda functions
deployed using CloudFormation or Terraform stacks that can be installed and
configured in just a few minutes.

The stack uses the minimal set of IAM permissions required for them to
work and requires no admin-like cross-account permissions. The entire code base
can be audited to see how these permissions are being used and even locked down
further if your audit discovers any issues. **This is not a SaaS**, no
component calls home or reveals any details about your infrastructure.

The main Lambda function is written in the Go programming language and the code
is compiled as a static binary. As of August 2021, this has been included in a
Docker image used by the Lambda function.

The stack also consists of a few CloudWatch event triggers, that run the Lambda
function periodically and whenever it needs to take action against the enabled
groups. Between runs, your group is entirely managed by AutoScaling (including
any scaling policies you may have) and load balancer health checks, that can
trigger instance launches or replacements using the original on-demand launch
configuration.

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
[Terraform](https://github.com/AutoSpotting/terraform-aws-autospotting)) stack
and setting the (configurable) `spot-enabled` tag on the AutoScaling groups
where you want it enabled to `true`.

When installed from the AWS
[marketplace](https://aws.amazon.com/marketplace/pp/prodview-6uj4pruhgmun6), all
the required infrastructure and configuration will be created automatically, so
you can get started as fast as possible. Otherwise, you'll need to build it
yourself as per the instructions available [here](CUSTOM_BUILDS.md).

For more detailed information on how to get started, you can also read this
[document](START.md)

## Support ##

Marketplace subscribers can get support from [here](mailto:contact@cloudutil.io)
and any feature requests or issues raised via this communication channel
will be prioritized.

Community support is available to OSS users on the
[gitter](https://gitter.im/cristim/autospotting) chat room, where the main
authors and other users are likely to help you solve issues. This is offered on a best-effort basis and under certain conditions, such as using the latest version of the software available on the main GitHub branch, without any code
customizations, and using the default configuration options.

If you need help with a large scale rollout or migrating from alternative
tools/solutions get in touch on [gitter](https://gitter.im/cristim).

## Contributing ##

AutoSpotting is fully open source and developed in the open by a vibrant
community of dozens of contributors.

If it helps you save any significant amount of money, it would be
greatly appreciated it if you could support further development on [Github
Sponsors](https://github.com/sponsors/cristim) or would purchase it from the AWS
[Marketplace](https://aws.amazon.com/marketplace/pp/prodview-6uj4pruhgmun6).

Note: Non-trivial code should be submitted according to the contribution
[guidelines](CONTRIBUTING.md). Individuals and companies supporting the
development of the open source code get free-of-charge support in getting their
code merged upstream.

### Official binaries ###

The source code is and will always be open source, so you can build and run
it yourself, see how it works, and even enhance it if you want.

As of August 2021, fully functional, stable official binaries are only offered
through Docker images available on the AWS
[Marketplace](https://aws.amazon.com/marketplace/pp/prodview-6uj4pruhgmun6).

Evaluation binaries are built from the trunk after each commit and meant to help the
development processes are also available from the Docker Hub but they expire after
a month since compilation.

Proven referrals towards a subscription to the Marketplace will be compensated
over Paypal with 50% of the first charge of the new subscriber. You can request
them on [gitter](https://gitter.im/cristim).

### Subscriptions ###

A free low-traffic mailing list is available on <https://autospotting.io>, where
you can sign up for occasional emails related to the project, mainly related to
major changes in the open source code, savings tips, or announcements about other
tools I've been working on.

Announcements on new Marketplace releases, including the comprehensive release
notes, upgrade instructions, and tips to get the most out of AutoSpotting will be
communicated in private to Patreon
[subscribers](https://www.patreon.com/cristim/overview).

A Github [sponsors](https://github.com/sponsors/cristim) subscription is also
available for people interested in the ongoing development of AutoSpotting, with
tiers covering anything from a non-strings attached donation, prioritization of
feature requests, all the way to custom features development and maintenance of
private customized forks.

Please get in touch on [gitter](https://gitter.im/cristim) if you have any
questions about these offerings or if you have any other ideas on how I could
provide additional value to my community.

## Compiling and Installing ##

It is recommended to use the evaluation or stable Docker images available on the
AWS marketplace, which is easy to install, supports further development of the
software and allow you to get some support.

But if you have some special needs that require some customizations or you don't
want to rely on the author's infrastructure or contribute anything for longer
term use of the software, you can always build and run your customized binaries
that you maintain on your own, just keep in mind that those won't be supported
in any way.

More details are available [here](CUSTOM_BUILDS.md)

## License ##

This software is distributed under the terms of the OSL-3.0 [license](LICENSE).

The evaluation images available on Docker Hub are licensed under this proprietary
[license](BINARY_LICENSE).

The AWS Marketplace offering is made available under the standard AWS
Marketplace EULA.
