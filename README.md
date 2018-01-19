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
costs by automating the use of [spot](https://aws.amazon.com/ec2/spot)
instances.

![Savings](https://cdn.cloudprowess.com/images/savings.png)

## How does it work? ##

When installed and enabled on an existing on-demand AutoScaling group,
AutoSpotting clones one of your on-demand instances from the group with a spot
instance that is cheaper, at least as large (automatically considering memory,
CPU cores and disk volumes) and configured identically to it. Once the new spot
instance is ready, it is attached to the group and an on-demand instance is
detached and terminated, to keep the group at constant capacity.

It continuously applies this process, across all enabled groups from all
regions, gradually replacing your on-demand instances with much cheaper spot
instances. For your peace of mind, you can also configure it to keep running a
configurable number of on-demand instances, given as percentage or absolute
number.

Your groups will then monitor and use these spot instances just like they would
do with your on-demand instances. They will automatically join your load
balancer and start receiving traffic once passing the health checks.

The installation takes just a few minutes and the existing groups can be enabled
and configured individually by using a few additional tags.

This can be seen in action below, you can click to expand the animation:

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
stack and setting the `spot-enabled` tag on the AutoScaling groups where
you want it enabled to `true`.

For more detailed information you can read this [document](START.md)

[![Launch](https://s3.amazonaws.com/cloudformation-examples/cloudformation-launch-stack.png)](https://console.aws.amazon.com/cloudformation/home?region=us-east-1#/stacks/new?stackName=AutoSpotting&templateURL=https://s3.amazonaws.com/cloudprowess/nightly/template.json)

Note: this installs the latest nightly build, generated automatically from the
 code after each commit. Even though it's generally stable, it is meant to be
 used for evaluation purposes and is **not recommended for production use**. It
 expires after two months from the date in which the binary was built and comes
 with no support or warranty.

Stable, carefully tested and supported builds are available from the original
author, and given to major [Patrons](https://www.patreon.com/cristim). Using
these builds you are also helping the further development of the software.

Please get in touch on [gitter](https://gitter.im/cristim) if you are a Patron
interested in getting a stable build.

## Compiling and Installing ##

Even though it's recommended to use the provided stable binaries, in case you
have some special needs, you can tweak the software, then build and run your
customized binaries, since these do not expire.

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

You can also contribute financially, we gladly accept tips on
[Patreon](https://www.patreon.com/cristim) or
[Paypal](https://paypal.me/cristim). Even a small percentage, 1 to 5% cut of the
generated monthly savings would make a huge difference to the development of the
project, please convince your organization to invest in it, they will reap the
benefits of any further improvements.

## Support ##

Community support is available on the
[gitter](https://gitter.im/cristim/autospotting) chat room on a best effort
basis, and people may help you solve issues with the nightly/evaluation
binaries.

The main author also offers enterprise-grade support and will do as much as
possible to help you out with any issues you may have. Custom feature
development as well as AWS-related consulting are also available for a fee,
often proportional to just a month worth of savings. For more information feel
free to get in touch on [gitter](https://gitter.im/cristim).

## Users ##

Autospotting is already used by hundreds of individuals and organizations around
the world, some of them we know of are mentioned in the [list](USERS.md) of
notable users.

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
If you want it to stay like this forever, please consider contributing.
