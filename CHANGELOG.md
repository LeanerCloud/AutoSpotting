# ChangeLog

## build XXX

### New features since the last update

- The method by which AutoSpotting terminates existing Auto Scaling Group
  instances has changed.  By default, AutoSpotting now uses the EC2
  `TerminateInstanceInAutoScalingGroup` API.  This API call ensures that any
  Termination Lifecycle Hooks that might be configured on the Group are
  respected, which was not the case in previous AutoSpotting versions. Users who
  depend upon the legacy behavior, which was to detach the instance from the
  Auto Scaling Group and terminate it, can set `instance_termination_method` to
  `detach` in their deployment configurations.

## 24 January 2018, build 622

A lot of time passed since the previous Changelog update, so this is a really
beefy entry, we should definitely do this more often.

There were a lot of contributions from many individuals, thank you all for
helping improve this project!

Special thanks once again to @xlr-8 who contributed a lot throughout the
previous year, and is now a very active co-maintainer of the project.

### New features since the last update

- Consider GPUs in the instance compatbility checks
- Instance type updates and regional expansions
- Aggressive bidding strategy based on the current spot price, by @kartik894
- Prevent termination of last instance, by @universam1
- Support volume discounts (think Reserved Instance), by @masneyb
- Instance type whitelisting, by @raravena80
- Instance type blacklisting, by @binarylogic
- Consider the EBS pricing surcharge, by @vecchp
- Deploy using Terraform, by @xlr-8

### Under the hood changes

- Significant refactorings, especially done by @xlr-8
- We're now using the native Lambda Go runtime, again thanks to @xlr-8
- Scalability fixes: concurrency by @thebigjc, API pagination by @ahaverbuch,
  support handling more than 50 AutoScaling groups concurrently, by @chaner
- Much more unit test coverage, thanks to @xlr-8 and @artemnikitin
- Buildsystem changes, we now also have automated lint and vet checks
- Dependency vendoring, by @xlr-8
- Lots of bugfixes, special thanks to @xlr-8
- Countless documentation updates by too many individuals to mention here

### Breaking changes

- The CloudFormation stack needs to be updated after build 633, because of the
  change to the Go Lambda runtime
- Some of the AutoScaling group tags used to override the global configuration
  were renamed to be more consistent, please refer to the current state of the
  documentation.

~Cristian

## 29 December 2016, build 158

I forgot to update this in a while, so this is a quite big changelog entry.

I got the first major code contributions by other developers, so from now on the
changelog entries will be split by author. It may also have a header like this
one in which it will contain a short summary or a message from the author.

Special thanks to Hugo Rosnet, who contributed a lot of code that implemented a
number of major features, helped me with multiple code reviews and kept me
motivated enough to constantly work on this project.

Also thanks to Jay Wineinger who contributed a non-trivial piece of code, and to
the other folks who contributed documentation, raised or discussed various
Github issues.

~Cristian

### Changes by author

#### @cristim

- Big code refactoring to make the code more maintainable and testable.
- Buildsystem improvements (and regressions, since fixed).
- Updated regional and instance type coverage, thanks to ec2instances.info.
- Support restricting the execution to a given set of regions.
- Expose all configuration options also as CloudFormation stack parameters.
- Documentation updates and improvements.
- Random small cleanups.

#### @xlr-8

- Update Lambda function's IAM permissions.
- The algorithm now supports keeping on-demand instances in each AutoScaling
  group.
- The algorithm is now configurable using tags set on the group and based on
  flags when executing it locally as a CLI tool.
- Significant test coverage increases.
- Significant clean-up and refactoring of the core algorithm.
- Documentation improvements.

#### @jwineinger

- Pagination fix, making it work for users having many ASGs.

#### @roeyazroel

- Documentation for Elastic Beanstalk.

## 14 November 2016, build 79

 Major, breaking compatibility, packaging update: now using eawsy/aws-lambda-go
 for packaging of the Lambda function

- Switch to the golang-native eawsy/aws-lambda-go for packaging of
  the Lambda function code.
- This is a breaking change, updating already running CloudFormation
  stacks will also need a template update.
- Add versioning for the CloudFormation template.
- Buildsystem updates (both on Makefile and Travis CI configuration).
- Change build dependencies: now building Lambda code in Docker, use
  wget instead of curl in order not to download data unnecessarily.
- Remove the Python Lambda wrapper, it is no longer needed.
- Start using go-bindata for shipping static files, instead of packaging
  them in the Lambda zip file.
- Introduce a configuration object for the main functionality, not in
  use yet.
- Documentation updates and better formatting.

## 2 November 2016, build 74

- Test and fix support for EC2 Classic
- Fix corner case in handling of ephemeral storage
- Earlier spot request tagging

## 26 October 26, build 65

- Regional expansion for R3 and D2 instances

## 23 October 2016, Travis CI build 63

- Add support for the new Ohio AWS region
- Add support in all the regions for the newly released instance types:
  m4.16xlarge, p2.xlarge, p2.8xlarge, p2.16xlarge and x1.16xlarge

## Older change log entries

Before this file was created, change logs used to be posted as blog posts:

- [recent changes as of October 2016](http://blog.cloudprowess.com/aws/ec2/spot/2016/10/24/autospotting-now-supports-the-new-ohio-aws-region-and-newly-released-instance-types.html)
- in the initial phase of the project they were posted at the end of the [first
  announcement blog post](http://blog.cloudprowess.com/autoscaling/aws/ec2/spot/2016/04/21/my-approach-at-making-aws-ec2-affordable-automatic-replacement-of-autoscaling-nodes-with-equivalent-spot-instances.html)
