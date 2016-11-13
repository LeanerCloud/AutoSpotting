# ChangeLog

## 14 November 2016, build 79

 Major, breaking compatibility, packaging update: now using eawsy/aws-lambda-go for packaging of the Lambda function

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
- Add support in all the regions for the newly released instance types: m4.16xlarge, p2.xlarge, p2.8xlarge, p2.16xlarge and x1.16xlarge

# Older change log entries

Before this file was created, change logs used to be posted as blog posts:
- [recent changes as of October 2016](http://blog.cloudprowess.com/aws/ec2/spot/2016/10/24/autospotting-now-supports-the-new-ohio-aws-region-and-newly-released-instance-types.html)
- in the initial phase of the project they were posted at the end of the [first announcement blog post](http://blog.cloudprowess.com/autoscaling/aws/ec2/spot/2016/04/21/my-approach-at-making-aws-ec2-affordable-automatic-replacement-of-autoscaling-nodes-with-equivalent-spot-instances.html)
