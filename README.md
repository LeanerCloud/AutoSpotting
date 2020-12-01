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

AutoSpotting is the leading open source spot market automation tool, optimized
towards quick/easy/frictionless adoption of the EC2 spot market at any scale.

It is usually set up to monitor existing long-running AutoScaling groups with
minimal configuration changes(often just tagging them, but even that can be
avoided by using their existing tags), yielding the usual 70%-90% Spot cost
savings but in a better integrated, more cost effective and easier to adopt way
than the alternative tools and solutions.

## How does it work? ##

Once installed and enabled by tagging to run against existing on-demand
AutoScaling groups, AutoSpotting gradually replaces their on-demand instances
with cheaper spot instances that are at least as large and identically
configured to the group's members, without changing the group launch
configuration in any way. You can also keep running a configurable number of
on-demand instances given as percentage or absolute number and it automatically
fails over to on-demand in case of spot instance terminations.

Going forward, as well as on any new ASGs that match the expected tags, any new
on-demand instances above the amount configured to be kept running will be immediately
replaced with spot clones within seconds of being launched.

If this fails temporarily due to insufficient spot capacity, AutoSpotting will
continuously attempt to replace them every few minutes until successful after
spot capacity becomes available again. When launching Spot instances, the
compatible instance types are attempted in increasing order of their price,
until one is successfully launched, lazily achieving diversification in case of
temporary unavailability of certain instance types.

This process can partly be seen in action below, you can click to expand the animation:

![Workflow](https://autospotting.org/img/autospotting.gif)

Additionally, it implements an advanced logic that is aware of spot and on
demand prices, including for different spot products and configurable discounts
for reserved instances or large volume customers. It also considers the specs of
all instance types and automatically launches the cheapest available instance
types based on flexible configuration set globally or overridden at the group
level using additional tags, but these overrides are often not needed.

A single installation can handle all enabled groups from an entire AWS account in
parallel across all available AWS regions, but it can be restricted to fewer
regions if desired in certain situations.

Your groups will then monitor and use these spot instances just like they would
do with your on-demand instances. They will automatically join their respective
load balancer and start receiving traffic once passing the health checks, and
the traffic would automatically be drained on termination.

## What savings can I expect? ##

The savings it generates are in the 60-90% range usually seen when using spot
instances, but they may vary depending on region and instance type.

![Savings](https://autospotting.org/img/savings.png)

## What's under the hood? ##

The entire logic described above is implemented in a set of Lambda functions
deployed using CloudFormation or Terraform stacks that can be installed and
configured in just a few minutes.

The stack assigns them the minimal set of IAM permissions required for them to
work and requires no admin-like cross-account permissions. The entire code base
can be audited to see how these permissions are being used and even locked down
further if your audit discovers any issues. **This is not a SaaS**, there's no
component that calls home and reveals any details about your infrastructure.

The main Lambda function is written in the Go programming language and the code
is compiled as a static binary compressed and uploaded to S3. For evaluation or
debugging purposes, the same binary can run out of the box locally on Linux
machines or as a Docker container on Windows or macOS. Some people even run
these containers on their existing Kubernetes clusters assuming the other
resources provided by the stack are implemented in another way on Kubernetes.

The stack also consists of a few CloudWatch event triggers, that run the Lambda
function periodically and whenever it needs to take action against the enabled
groups. Between runs your group is entirely managed by AutoScaling (including
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

All the required infrastructure and configuration will be created automatically,
so you can get started as fast as possible.

For more detailed information you can read this [document](START.md)

### Launch latest evaluation build

Warning: it may occasionally be broken or even unavailable, see the below notes
for more information.

[![Launch](https://s3.amazonaws.com/cloudformation-examples/cloudformation-launch-stack.png)](https://bit.ly/AutoSpotting)

- The evaluation binaries launched by the above link are built from git after
  each commit.
- They are free of charge but expire after 30 days since the time they were
  built.
- They may occasionally be buggy or even unavailable if no code was committed
  within the last month.
- Their main purpose is to quickly find issues of the recently committed code
  and not necessarily to ease installation for new users, which is just a nice
  side-effect when development is relatively active.
- To avoind these limitations you can sign up
  [here](https://patreon.com/cristim) to get access to the latest stable and
  supported version that's as easy to install but has no such short expiration.
- Alternatively you can also build AutoSpotting from the open source code.
- the Docker images available on DockerHub are also distributed under the
  same evaluation terms.

## Support ##

Community support is available on the
[gitter](https://gitter.im/cristim/autospotting) chat room, where the main
authors and other users are likely to help you solve issues.

Note: This is offered on a best effort basis and under certain conditions, such
as using the latest version of the evaluation builds.

If you need help for a large scale rollout or migrating from alternative
tools/solutions, you can purchase a comprehensive support plan guaranteed to
save you lots of time and money, if you have any questions you can always get in
touch on [gitter](https://gitter.im/cristim) if you need such help.

## Contributing ##

Unlike multiple commercial products in this space that cost a lot of money and
attempt to lock you in, this project is fully open source and developed in the
open by a vibrant community of dozens of contributors.

If this software helps you save any significant amount of money, it would be
much appreciated if you could support further development on [Github
Sponsors](https://github.com/sponsors/cristim).

Financial sponsorship is voluntary, it's also fine if you just try it out and
give [feedback](https://gitter.im/cristim/autospotting), report issues, improve
the documentation, write some code or assign a developer to work on it, or even
just spread the word among your peers who might be interested in it. Any sort of
support would be greatly appreciated and would make a huge difference to the
project.

Note: Non-trivial code should be submitted according to the contribution
[guidelines](CONTRIBUTING.md).

### Proprietary binaries ###

The source code is and will always be open source, so you can build and run
it yourself, see how it works and even enhance it if you want.

As mentioned before, we also have evaluation binaries that allow you to try it out
quickly fof free but come with a few limitations.

To avoid those limitations and also receive access to stable, supported code builds, you can
sign up to our inexpensive subscription [here](https://www.patreon.com/cristim/overview).

Individuals and companies supporting the development of the open source code get
free of charge support in getting their code merged upstream and upon demand
also can get stable build access for a year since their latest contribution to
the project.

Proven referrals that manifest through a subscription to the stable builds will
be compensated over Paypal with the amount of the first monthly fee of the
new subscriber (starting from $29). You can request them on
[gitter](https://gitter.im/cristim).

### Stable build benefits ###

Installation instructions for the stable builds suitable for Enterprise use will
be communicated in private to the stable
[subscribers](https://www.patreon.com/cristim/overview) as soon as they join and
subsequently when new version become available later.

Comprehensive release notes, upgrade instructions and tips to get the most out
of AutoSpotting will accompany new stable releases and also made available
through the same private communciated channel. These documents will not be
communicated to the public audience.


These come with comprehensive support from the author, who will do his best to
help you successfully run AutoSpotting on your environment so you can get the
most out of it.

The feature requests and issues raised on Github or via private communication
channels will be prioritized.

There is also a private forum where stable build users can interact with each
other.

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
