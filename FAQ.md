# Frequently Asked Questions

Feel free to add here any questions related to the AutoSpotting project and to
edit existing items if you notice any inaccuracies. Once the list is in a decent
shape it will be added to the official project documentation under a new FAQ
section.

For editing please try to use Markdown syntax or simply just write unformatted
text, we can take care of the formatting later.

## What is AutoSpotting?

AutoSpotting is a tool implementing an automated bidding algorithm against the
spot market, which often gives you much cheaper spot instances, allowing you to
generate hefty savings.

## What are spot instances, what is the spot market and how does it work?

Cloud providers such as Amazon AWS need to always have some spare capacity
available, so that any customer willing to launch new compute machines would be
able to do it without getting errors. Normally this capacity would be sitting
idle but still consuming power until someone needs to use it.

Instead of wasting this idle capacity, Amazon created a marketplace for anyone
willing to pay some money in order to use these machines, but knowingly taking
the risk that this spare capacity may be taken back within minutes when it needs
to be allocated to an ordinary on-demand user.

This is known as the *spot market*, and these volatile compute machines are
called *spot instances*.

The market automatically computes a price based on the current supply and
demand, updated every minute, and everyone from the same AvailabilityZone (note:
availability zone names may not necessarily match between different AWS
accounts) within a region pays the same price for a given instance type.

Luckily the market price is most of the times a number of times less than the
normal, on-demand price, sometimes up to 10 times less, but usually in the 5x-7x
range. In some regions the price is so stable that spot instances may run for
weeks or even months at about 80% savings, so it's basically a waste of money
not to run spot instances there.

This sounds too nice to be true, and it almost is. In case of demand surges or
maintenance events in which some of the capacity is reclaimed, the price can
also shoot much over the normal price, sometimes maybe even 10 times more. The
spot instances are terminated with a two minute notice when the market price is
more than the bid price for that particular instance type/availability zone
combination.

In those cases it would obviously be better to fall back to the normal on-demand
instances in order not to pay a surge premium, but some people still bid a lot
more than the on-demand price in order to dodge these surges without their spot
instances being terminated.

Since each availability zone and instance type combination has a different price
graph, in case one of them became too pricy and terminated all its instances, it
is possible to find another cheap one which may still run your workload, at
least for a while.

With enough redundancy(which should be in place anyway where it really matters),
a surprisingly large amount of applications are able to migrate the user traffic
to surviving machines in the event of spot instance terminations on some of the
availability zones.

One could generate a lot of savings if somehow could protect against these
unexpected terminations, like by quicly switching to another instance type or
even falling back to on-demand instances for relatively brief periods of time.

Autospotting is implementing exactly this kind of automation of switching
between different spot instance types, also falling back to on demand during
price surges.

## How does AutoSpotting work?

Normally the spot bidding process is static and can be done manually using the
AWS console, but since AWS exposes pretty much everything via APIs, it can also
be automated against those APIs.

AutoSpotting is a tool implementing such automation against the AWS APIs.

It monitors some of your AutoScaling groups where it was enabled and it
 continuously replaces any on-demand instances found in those groups with
 compatible, idenically configured spot instances.

## How do I install it?

 You can launch it using the provided CloudFormation stack. It should be doable
 within a couple of minutes and just needs a few clicks in the AWS console or a
 single execution of awscli from the command-line.

 Alternatively, there is also a community-supported Terraform stack which works
 similarly.

You can see the Getting started guide for more information on both installation
methods, as wel as the initial setup procedure.

## In which region should I install it?

For technical reasons, the CloudFormation installation only works with the
US-East region.

The community-supported Terraform stack can be launched in any AWS region.

You only need to install it once, at runtime it will by default connect across
all regions in order to take action against your enabled AutoScaling groups.
This is configurable in case you want to only have it running against a smaller
set of regions.

 ## How much does it cost me to run it?

AutoSpotting is designed to have minimal footprint, and it will only cost you a
few pennies monthly.

It is based on AWS Lambda, and it should be well within the monthly free tier,
so you will only pay a bit for logging and network traffic performed against AWS
API endpoints.

The default configuration is triggering the Lambda function once every 5
minutes, and most of the time it runs for just a couple of seconds,enough to
evaluate the current state and notice that no action needs to be taken. In case
replacement actions are taken it may run for more time sometimes even until the
Lambda function timeouts, but it is designed to cleanly recoverbetween runs in
the event of such timeouts.

The Cloudwatch logs are by default configured with a 7 days retention period
which should be enough for debugging, but shouldn't cost you so much. If
desired, you can easily configure the log retention and execution frequency to
any other values.

## How about the software costs?

The software itself is free and open source so there is no monthly subscription
fee if you use the open source code straight from trunk. The software itself is
largely community-supported, well-documented and is designed to be easy to set
up so it shouldn't need much support.

But if you really find it useful please consider giving a recurring tip on
[Patreon](https://www.patreon.com/cristim) to encourage further development.

If your tip is over 5% of the money it saves you monthly, you will get access to
stable builds that were carefully tested by the author and should be more
reliable for production use cases, and if you have any issues they will be
handled with higher urgency by the author.

The author also offers custom feature development and deployment support for a
fee, feel free to [get in touch](https://gitter.im/cristim) if you need any
help.

 ## How do I enable it?

The entire configuration is based on tags applied on your AutoScaling groups.

It will only take action against groups that have the "spot-enabled" tag set to
"true", regardless in which region they are.


## What if I have groups in multiple AWS regions?

As mentioned before, the groups can be in any region, AutoSpotting will connect
to all regions unless configured otherwise at installation time.

The region selection can be changed later by updating the CloudFormation or
Terraform stack you used to install it.

## Will it replace all my on-demand instances wth spot instances? Can I keep some running just in case?

First, AutoSpotting will ignore all your groups, unless configured otherwise
using tags for every single group.

For your peace of mind, AutoSpotting can also keep some on-demand instances
running in each of the enabled groups, if so configured.

On the enabled groups it will by default replace all on-demand instances with
spot, unless configured otherwise in the CloudFormation stack parameters. This
global setting can be overridden on a per-group level.

You can set an absolute number or a percentage of the total capacity, also using
tags set on each group.

For more information about these tags, please refer to the Gettins Started
guide.

## What does it mean "compatible"?

The launched spot instances can be of different type than your initial
instances, unless configured otherwise, but always at least as large as the
initial ones. You can also constrain it to the same type or a list of types,
also using some optional tags.

By large it means the launched spot instances have at least as much memory, disk
volumes(both size and their total number), CPU and GPU cores, EBS optimization,
etc. as the initial on-demand instance type.

The price comparison also takes into account the EBS optimization extra fee
charged on some instance types, so if your original instance was EBS optimized
you will still get an EBS optimized instance, and the prices are correctly
compared, taking the EBS optimization surcharge into account.

Often the initial instance type, or the same size but from a different
generation, will be the cheapest so it is quite likely that you will get at
least some instances from the original instance type, but it can also happen
that you get much beefier spot instances.

The algorithm will also consider your original instances in case they are not
available on the spot market, such as if you originally have burstable
instances. For example it was seen to replace t2.medium on-demand instances with
m4.large spot instances.

## How is the spot instance replacement working?

AutoSpotting will eventually run against your enabled group and notice that
on-demand instances exist.

It will randomly pick an on-demand instance and initiate a replacement, by
launching a compatible spot instance chosen to be the cheapest available at that
time in the same availability zone as the on-demand instance selected for
replacement. The spot instance is configured identically to the original
instances, sharing the security groups, SSH key, EBS block device mappings, EBS
optimization flag, etc. This information is taken mainly from the group's launch
configuration, which is kept unchanged and would still launch on-demand
instances if needed.

The spot bid price is currently set to the hourly price of the original
on-demand instance type, but this is subject to change in the future once we
implement more bidding strategies.

The spot instance request will be tagged with the name of the AutoScaling group
for which it was launched, and the algorithm waits for the spot instance to be
launched. Once the spot instance resource was launched and became available
enough for receiving API calls, the instance is tagged to the same tags set on
the initial instance, then the algorithm stops processing instances in that
group. It does this in parallel over all your groups across all AWS regions
where it was configured to run.

On the next run, maybe 5 minutes later, it verifies if the launched spot
instance was running for enough time and is ready to be added to the group. It
currently considers the grace period interval set on the group and compares it
with the instance's uptime.

If the spot instance is past its grace period, AutoSpotting will attach it to
the group and immediately detach and terminate an on-demand instance from the
same availability zone. Note that if draining connection is configured on ELB
then Auto Scaling waits for in-flight requests to complete before detaching
the instance. The terminated on-demand instance is not necessarily the same used
initially, just in case that may have been terminated by some scaling operations
or for failing health checks.

## What happens in the event of spot instance terminations?

Since AutoSpotting will run by default only every 5 minutes, nothing will happen
immediately, unless you configure your spot instances to take notice of the two
minute termination notification.

The notification is only visible from within the instance being terminated,
which must periodically query a certain metadata endpoint, so you can only do it
by running some additional code on the instance, and AutoSpotting can't directly
see this notification.

There are tools(or even simple bash scripts) which continuously poll the
instance metadata and detect the notification. Once detected, they can take some
draining action by running custom scripts. You can do things like removing the
instance from your load balancer, pushing log files to S3, draining ECS cluster
tasks, etc. and hopefully manage to drain it completely before it's terminated.

Once the instance was terminated, your AutoScaling group's health checks will
eventually fail, and the group will handle this as any instance failure, by
launching a new on-demand instance in the same availability zone, as initially
configured on the group.

That on-demand instance will be replaced later on by AutoSpotting using the
normal replacement process which can be seen above.

## What bidding price does AutoSpotting use?

By default AutoSpotting is placing spot bids with the hourly price of your
original on-demand instances, so you never pay more than that in the event of
price surges.

Another bidding strategy is placing bids based on the current spot price, with a
bit of buffer (default 10%) on top of the current spot price. This will
terminate your spot instances on significant price increases, to give the
algorithm the chance to search for better priced instance types.

## What are the goals and design principles of AutoSpotting?

To paraphrase Rancher's motto: "Using Spot instances is hard, AutoSpotting makes
it easy"

AutoSpotting is designed to be used against existing AutoScaling groups with
long-running instances, and it is trying to look and feel as close as possible
as a native AWS service. Ideally this is how the AutoScaling spot integration
should have been implemented by AWS in the first place.

Once installed and set up, it hopefully becomes invisible, both from the
configuration management perspective but also from the incurred runtime costs,
and security risks which should be negligible.

The configuration should be minimalist and everything should just work without
much tweaking. You're not expected to need to determine which instance types are
as good as your initial ones, which instance type is the cheapest in a given
availability zone, and so on. Everything should be determined based on the
original instance type using publicly available information and querying the
current spot prices in real time. Your main job is to make sure you configure a
proper draining action suitable for your application and environment.

It also tries as much as possible to avoid locking you in, so if you later
decide that spot instances aren't for you and you want to disable it, you can
easily do it with just a few clicks or commands, and immediately revert your
environment to your initial on-demand setup, unlike most other solutions where
the back-and-forth migration effort may become quite significant.

From the security perspective, it was carefully configured to use the minimum
set of IAM permissions needed to get its job done, nothing more, nothing less.
There is no cross-accounting IAM role, everything runs from within your AWS
account.

## What is the use case in which AutoSpotting makes most sense to use?

Any workload which can be quickly drained from soon to be terminated instances.

AutoSpotting is designed to work best with relatively similar-sized, redundant
and somewhat long-running stateless instances in AutoScaling groups, running
workloads easy to transfer or re-do on other nodes in the event of spot instance
terminations.

Here are some classical examples:

- Development environments where maybe short downtime caused by spot
  terminations is not an issue even when instances are not drained at all.
- Stateless web server or application server tiers with relatively fast response
  times (less than a minute in average) where draining is easy to ensure
- Batch processing workers taking their jobs from SQS queues, in which the order
  of processing the items is not so important and short delays are acceptable.
- Docker container hosts in ECS, Kubernetes or Swarm clusters.

Note: AutoSpotting doesn't currently implement the termination monitoring and draining
logic, which may depend a lot on your application. But there are some tools
implementing spot instance termination handling and allowing you to customize
the draining action.

## What are some use cases in which AutoSpotting should not be used? What should I use instead?

Anything that doesn't really match the above cases.

### Groups that have no redundancy

If you have a single instance in the group, spot terminations may often leave
your group without any nodes. If this is a problem, you should not run
AutoSpotting in such groups, but instead use reserved instances, maybe of T2
burstable instance types if your application works well on those.

### Instances which can't be drained quickly

If your application is expected to serve long-running requests, without timing
out after longer than a couple of minutes, AutoSpotting(or any spot automation)
may not be for you, and you should be running reserved instances.

### Cases in which the order of processing queued items is strict

Spot instance termination may impact such use cases, you should be running them
on on-demand or reserved instances.

### Stateful workloads

AutoSpotting doesn't support stateful workloads out of the box, particularly in
case certain EBS persistent volumes need to be attached to running instances.

The replacement spot instances will be started but they will fail to attach the
volume at boot because it is still attached to the original instance. Additional
configuration would have to be in place in order to re-attempt the attach
operation a number of times, until the previous on-demand instance is terminated
and the volume can be successfully attached to your spot instance. The spot
instance's software configuration may need to be changed in order to accommodate
this EBS volume.

## Q: How does AutoSpotting compare to the the AutoScaling spot integration?

Or why would I use AutoSpotting instead of the normal AutoScaling groups which set
a spot price in the launch configuration?

The answer is it's more reliable than the normal spot AutoScaling groups,
because they are using a fixed instance type so they ignore any better priced
spot instance types, and they don't fallback to on-demand instances when the
market price is higher than the bid price across all the availability zones, so
the group may be left without any running instances.

AutoSpotting-managed groups will out of the box launch on-demand instances
immediately after spot instance terminations and AutoSpotting will only try to
replace them with spot instances when compatible instances are available on the
market at a better price than those on-demand instances. The spot capacity is
still temporarily decreased during the price surge for a short time, until your
on-demand instances are launched and configured, but the group soon recovers all
its lost instances.

On top of that, Autospotting allows you to configure the number or percentage of
spot instance that you tolerate in your ASG, while the integration of AWS would
try to replace them all, causing potential downtime if they were to disapear at
the same time.

## How does AutoSpotting compare to the the spot fleet AWS offering?

Or why would I use AutoSpotting instead of the spot fleets? And when would I be
better off running spot fleets?

The spot fleets are groups of spot instances of different types, managed much
like AutoScaling groups, but with a different API and configuration mechanism.
Each instance type needs to be explicitly configured with a certain bid price
and weight, so that the group's capacity can be scaled out over various instance
types.

These groups are quite resilient because they are usually spread over multiple
spot instance types, so it's quite unlikely that the price will surge on all of
them at once. But much like the default AutoScaling spot mechanism they are also
unable to fall back to on-demand capacity in case the prices surge across all
their instance types. AutoSpotting will also try to avoid using all instances of
the same type, in many cases, with enough capacity by spreading over three or
four different spot market price graphs, which in addition to the on-demand
fallback capability should be also quite resilient in the event of spot
terminations.

Also the SpotFleet configuration mechanism is quite complex so it's relatively
hard to migrate to/from them if you already run your application on AutoScaling
groups, which is trivial to do with AutoSpotting.

The SpotFleets are also much less widely used than the AutoScaling groups, and
many other AWS services and third-party applications are integrated out of the
box with AutoScaling but not with SpotFleets. Things like ELB/ALB, CodeDeploy,
and Beanstalk would run pretty much out of the box on AutoScaling groups managed
by AutoSpotting, while integrating them with SpotFleets may need additional work
or would simply be impossible in their current implementation. People also tend
to be much more familiar with the AutoScaling group concept, which is easier to
grasp and makage by developers which have more limited exposure with AWS.

Spot Fleets are great for use cases in which the instance type is not important
and can vary widely, or workloads can be somehow scheduled on certain instance
types, like for example in case of ECS clusters.

AutoSpotting, on the other hand will try to keep the instances relatively
consistent with each other, so instances will be in a narrower range than
usually configured on the spot fleets. This is a consequence of the current
implementation which doesn't have any weighting mechanism, so in order to
meaningfully scale capacity with the same AutoScaling policies, the instances
have to be roughly of the same size. The original on-demand price used for spot
instance bidding will also constrain the spot instance types to a relatively
narrow range, which is not the case for SpotFleets.

## How does AutoSpotting compare to commercial offerings such as SpotInst?

Many of these commercial offerings have in common a number of things:
- SaaS model, requiring admin-like privileges and cross-account access to all
  target AWS accounts which usually raises eyebrows from security auditors. They
  can read a lot of information from your AWS account and send it back to the
  vendor and since they are closed source you can't tell how they make use of
  this data. Instead, AutoSpotting is launched within each target account so it
  needs no cross-account permissions, and no data is exported out of your
  account. Also since it's open source you can entirely audit it and see exactly
  what it does with your data and you can tweak it to suit your needs.
- Implement new constructs that mimic existing AWS services and expose them with
  proprietary APIs, such as clones of AutoScaling groups, maybe sometimes
  extended to load balancers, databases and functions, which expect custom
  configuration replicating the initial resources from the AWS offering. Much
  like with spot fleets, this makes it quite hard and work-intensive to migrate
  towards but also away from them, which is a great vendor lock-in mechanism if
  you're a start-up, but not so nice if you are a user. Many of these resources
  require custom integrations with AWS services, which need to be implemented by
  the vendor. Instead, AutoSpotting's goal is to be invisible, easy to install
  and remove, so there's no vendor lock-in. Under the hood it's all good-old
  AutoScaling, and all its integrations are available out of the box. If you
  need to integrate it with other services you can even do it yourself since
  it's open source.
- they're all pay-as-you-go solutions charging a percentage of the savings. For
  example spotinst charges 25% or often as much as you will pay AWS for spot
  instances, which I find obscene for how simple this functionality can be built
  in AutoSpotting. They justify this by nice looking dashboards and buzz words
  such as Machine Learning, but although that's nice to have, it's not really
  needed to implement this type of automation. Predicting the spot prices is
  hard so it's better to invest the time automating the draining process and
  making it faster to react when terminations happen. Their goal is to sell a
  product which has to look and feel polished enough for people to buy it.
  AutoSpotting's goal is to simply be useful, and as invisible as possible, also
  from the price persoective. If you need to see a saving dashboard, just wait
  for the end of the month and then look at the Bills section of the AWS
  console, or feel free to contribute implementation for one if you need it.
- From the functionality perspective they are indeed more feature-rich and
  polished than both AWS Spot Fleets and AutoSpotting, and they may be
  cloud-provider-agnostic, but their price tag is huge.

## Does AutoSpotting continuously search and use cheaper spot instances?

If I attach autospotting to a auto scaling group that is 100% spot instances,
will it autobid for cheaper compatible ones when found later on?

The answer is No. The current logic won't terminate any running spot instances
as long as they are running, and since they are using the on-demand price as bid
value they may run for a relatively long time while other cheaper spot instance
may become available on the market. The only times when AutoSpotting interacts
with your instances is at the beginning, after scaling actions or immediately
after spot instances are terminated and on-demand instances are launched again
in the group.

This behavior may be changed once implementing
[#119](https://github.com/cristim/autospotting/issues/119), in which we may
implement a strategy bidding closer to the current spot price in order to avoid
running that spot instance after significant spot price increases.

## The lambda function was launched but nothing happens. What may cause this?

I have a couple of on demand instances behind an asg configured with the
required tags but still no spot instance is bieng launched. What is the problem?

Have a look at the logs for more details.

Spot instances may fail to launch for a number of reasons, such as market
conditions that manifest in high prices across all the compatible instance
types, but also known bugs or limitations in the current implementation, such as
[#105](https://github.com/cristim/autospotting/issues/105),
[#106](https://github.com/cristim/autospotting/issues/106) and
[110](https://github.com/cristim/autospotting/issues/110), which would need to
be fixed or simply implemented. If you are impacted by such issues please
consider [contributing](CONTRIBUTING.md) a fix.

Other cases may need to be reported as additional issues.

# Which IAM permissions are granted to the AutoSpotting CloudFormation Stack and why are they needed?

Just like users who pipe curl output into their shell for installing software
should carefully review those installation scripts, users should pay attention
and audit the infrastructure code when launching CloudFormation or Terraform
stacks available on the Internet, especially in case they are given significant
permissions against the AWS infrastructure.

AWS is quite helpful and by default it forbids installation of stacks which have
the potential to be used for escalation of privileges, but it turns out
AutoSpotting needs such permissions in order to work.

In order to launch the AutoSpotting stack, you will need to have admin-like
permissions in the target AWS account and you need to give the stack a special
permission, called `CAPABILITY_IAM`, which is needed because the stack creates
additional IAM resources which could in theory be abused for privilege
escalation. You can read more about this in the official AWS
[documentation](http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/using-iam-template.html#using-iam-capabilities)

The AutoSpotting stack needs this capability in order to create a custom IAM
role that allows the Lambda function to perform all the instance replacement
actions against your instances and autoscaling groups.

This configuration was carefully crafted to contain the minimum amount of
permissions needed for the instance replacement and logging its actions. The
full list can be seen in the Cloudformation stack
[template](https://github.com/cristim/autospotting/blob/master/cloudformation/stacks/AutoSpotting/template.json#L91),
but it basically boils down to the following:
- describing the resources you have in order to decide what needs to be done
  (things such as regions, instances, spot prices, existing spot requests,
  AutoScaling groups, etc.)
- launching spot instances
- attaching and detaching instances to/from Autoscaling groups
- terminating detached instances
- logging all actions to CloudWatch Logs

In addition to these, for similar privileges escalation concerns, the
AutoSpotting Lambda function's IAM role also needs another special IAM
permission called `iam:passRole`, which is needed in order to be able to clone
the IAM roles used by the on demand instances when launching the replacement
spot instances. This requirement is also pretty well
[documented](https://aws.amazon.com/blogs/security/granting-permission-to-launch-ec2-instances-with-iam-roles-passrole-permission/)
by AWS.

Since AutoSpotting is open source software, you can audit it and see exactly how
all these capabilities are being used, and if you notice any issues you can
improve it yourself and you are more than welcome to contribute such fixes so
anyone else can benefit from them.

## Is the project going to be discontinued anytime soon?

No way!

The project is actually growing fast in popularity and there are no plans to
discontinue it, actually it's quite the opposite, external contributions are
accelerating and the software is maturing fast. There are already hundreds of
installations and many companies are evaluating it or using it for development
environments, while some are already using it in production.

Since it's open source anyone can participate in the development, contribute
fixes and improvements benefitting anyone else, so it's no longer a tiny
one-man-show open source hobby project.

## How do I Uninstall it?

You just need to remove the AutoSpotting CloudFormation or Terraform stack.

The groups will eventually revert to the original state once the spot market
price fluctuations terminate all the spot instances. In some cases this may take
months, so you can also terminate them immediately, the best way to achieve this
is by configuring autospotting to use 100% on-demand capacity.

Fine-grained control on a per group level can be achieved by removing or setting
the `spot-enabled` tag to any other value. AutoSpotting only touches groups
where this tag is set to `true`.

## Shall I contribute to Autospotting code?

Of course, all contributions are welcome :)

For detais on how to contribute have a look [here](CONTRIBUTING.md)
