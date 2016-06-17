# README #

This README tries to document whatever steps are necessary to get the autospotting application up and running.

## What is this repository for? ##

Autospotting is a tool meant to automate the handling of EC2 spot instances on Amazon AWS EC2.

The goal is to allow the user to achieve significant savings, often in the range of 80% off the AWS EC2 bill, in a way that maximizes the availability in the event of spot market fluctuations of certain instance types in certain availability zones, that trigger termination of spot instances.

Once enabled on an AutoScaling group, it is gradually replacing all the on-demand instances belonging to the group with compatible but cheaper spot instances. The replacements are done using the relatively new Detach/Attach actions supported by the AutoScaling API. A new compatible spot instance is launched, and after a while, at least as much as the group's grace period, it will be attached to the group, while at the same time an on-demand instance is detached from the group and terminated.

When assessing the compatibility, it takes into account the hardware specs, such as CPU cores, RAM size, instance store volumes and size, as well as the supported virtualization types (HVM or PV). The new spot instance type is usually a few times cheaper than the original instance type, while also often providing more computing capacity. The new instance is also configured to execute the same user data script as the original instance, so from a functionality perspective it should be indistinguishable from other instances in the group.

When replacing multiple instances in a group, the algorithm tries to use a wide variety of instance types, in order to reduce the probability of simultaneous failures that may impact the availability of the group.


## How do I get set up? ##

### Summary of the set up ###

When deployed, the software consists on a number of resources running across multiple Amazon AWS accounts, mostly created automatically with CloudFormation:

#### Event generator SNS topic ####

 * deployed in the CloudProwess AWS account, only because it is easier to configure against a fixed topic ID
 * It was configured generate a CloudWatch event every 5 minutes, which is then sent to the SNS topic
 * It has enough IAM permissions to allow anyone to attach to the topic.

#### autospotting-lambda ####

* AWS Lambda function deployed in the user's AWS account, entirely configured by CloudFormation.
* Currently written in node.js because Python was not available, may be rewritten/replaced at some point once AWS implements native support for golang.
* out of the box it has assigned IAM role and policy with enough permissions to call various AWS services within the customer's account, without the need of passing any explicit AWS credentials or access keys.
* It is connected to the event generator topic. Messages sent to the topic trigger its execution, and the topic generates these every 5 minutes.

* Here is how it reacts on the event that it was given: It downloads and executes the agent code(a golang binary), stored in S3 and served through a CloudFront distribution, in order to be able to replace the agent binary without customer's intervention for continuous delivery purposes.

* The agent is given all the data generated in the event that triggered the current execution of the function. At the moment the data is written to a pair of JSON files created in /tmp, passed as command line arguments, read and parsed by the agent binary at runtime.
* The agent implements code able to handle the events.

#### agent ####

* golang binary that gets called from the autospotting-lambda function
* small size, it is stripped and compressed with goupx, then uploaded to S3
* Can react on the SNS events that may trigger its execution

* The spot instances are created by duplicating the launch configuration of the currently running on-demand instances, only by adding a spot bid price attribute.
* The bid price is set to the on-demand price of the instances configured initially on the AutoScaling group.
* The new launch configuration may also have a different instance type, determined based on compatibility with the original instance type, considering also how much redundancy we need to have in placein the current availability zone, in order to survive instance termination when outbid for a certain instance type.

#### notificator (not yet in use) ####

* lightweight golang binary that may later get injected in the user_data script of the newly launched spot instances.
* periodically polls the metadata service for instance termination notifications
* in the event of a termination notification set in the metadata of the current instance, it may trigger an execution of the lambda function by sending a message to the SNS topic
* after a while since the spot instance was started, it sends another type of message to the SNS topic, so that we can now terminate an on-demand instance that was replaced by a spot one so it is no longer needed.

# Configuration

## Dependencies

* You will need an AWS account where to run this function. AWS provides free tier where you can run Lambda for free as long as the code doesn't run continuously (1M GB*s/month). EC2 is free for the first year when using micro instances. Micro spot instances cost around $3/month, charged on a hourly basis.

The CloudFormation stacks can be easier launched, we recommend to use the clouds tool, which can be installed as a ruby gem:
   gem install clouds

## How to run tests

We have very limited test coverage, abut we're working on improving it.

## Deployment instructions

* You need to have an AWS profile configured, or also have the secret keys in the environment.
* Most directories contain Makefiles which have multiple targets for compiling the various components.
* There are also targets for packaging and uploading Lambda code to S3, etc.
* CloudFormation stacks can be launched with the clouds tool.

# Contribution guidelines ###

* Please consider writing tests, we have very little coverage and we need to have it improved.
* Code review: please create pull requests.

