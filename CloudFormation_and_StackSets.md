# CloudFormation and StackSets

StackSets are a very powerful way to deploy software at scale across multiple
AWS accounts and also to multiple regions within a single account.

AutoSpotting supports being deployed using StackSets across multiple accounts,
and also leverages them internally to deploy some of its components across
multiple regions.

This document explains the way the current CloudFormation deployment method of
AutoSpotting uses CloudFormation StackSets internally, what consequences it has
on existing StackSet environments or when AutoSpotting is installed repeatedly
within an AWS account, and a few workarounds on how to deploy AutoSpotting on
such environments.

## Background

- The current AutoSpotting architecture deploys a few central resources in a
  main AWS region but requires additional resources to be deployed in other
  regions to enable certain advanced behaviors such as handling of spot instance
  termination or rebalancing events, immediate replacement of on-demand
  instances with spot and startup lifecycle hook emulation. Without these
  regional resources AutoSpotting will only run in the basic/legacy Cron mode,
  which is suboptimal.
- These regional resources have been historically deployed using a second
  regional CloudFormation template, deployed by a custom Lambda-backed
  CloudFormation resource across all available AWS regions. This was error-prone
  in particular when having multiple installations of AutoSpotting side by side,
  especially when some of them were uninstalled.
- There was also another installation mode in which the same main CloudFormation
  template could itself be deployed using a StackSet, deploying only the main
  resources in the main region and only the regional resources in the other
  regions based on some parameters. This duplicated a lot of infrastructure code
  between the two CloudFormation templates and many conditionals that
  overcomplicated the implementation of the main template enough that it became
  almost unmanageable, so we decided to simplify it.
- The current implementation uses a StackSet to deploy the regional resources
  instead of the custom Lambda-backed resource, and has been simplified greatly by
  removing all the conditionals and duplicated code that enabled it to use a
  StackSet for deploying the regional resources with the same template code.
- To keep the user-friendly single-click installation support, the main AutoSpotting
  CloudFormation template currently also deploys out of the box the required IAM
  resources needed for self-managed StackSet permissions that enable it to
  deploy the regional template as a StackSet, and that's why it's conflicting
  with self-managed StackSet permissions you may already have in your account or
  if they were created by another installation of AutoSpotting. If you run into
  any such installation issues, you can see the Workarounds section below.

## Instructions on how to install AutoSpotting across an Organization using a StackSet

1. Grant permissions for using StackSets at AWS Organization level using these
   [instructions](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/StackSets-orgs-enable-trusted-access.html).
   See Note 1 below for further information on this topic.
1. Use the same CloudFormation
   [template](https://s3.amazonaws.com/cloudprowess/nightly/template.yaml) and
   parameters used for deploying AutoSpotting in stand-alone mode within a
   single AWS account.
1. Create the StackSet only in the "Main" region. For the official binaries you
   will need to use `us-east-1`, otherwise installation fails. See Note 2 below
   in case you run a custom build hosted in another region.
1. Set the OrganizationUnit where you want to deploy AutoSpotting and complete
   the installation wizard. You can also use the Organization root to install
   AutoSpotting across an entire AWS Organization.
1. For faster installation you can allow a 50% failure percentage, otherwise the
   StackSet will deploy AutoSpotting only one account at a time which can be
   slow on large organizations.

## Notes

1. Self-managed StackSet permissions are not supported out of the box and will
   break the default installation of AutoSpotting if you have them configured in
   the account. See below the workaround for this issue if you run into it.
1. If you run a custom build use the region where you created the S3 bucket
   hosting the code of the customized AutoSpotting Lambda functions.

## Workarounds

- In case the installation fails because of the conflict with pre-existing IAM
  resources created for self-managed StackSet permissions or by an existing
  AutoSpotting installation, you'll need to configure the AutoSpotting Stack
  parameters to not deploy any regional resources. You can do it by setting the
  `DeployRegionalResourcesStackSet` parameter to `false`.
- This which will render the current AutoSpotting installation to run in the
  legacy cron mode, also lacking termination event handling and lifecycle hooks
  emulation. In order to re-enable the event-based execution mode and other
  advanced features, you will then need to deploy those regional resources
  yourself with a second AutoSpotting regional StackSet deployed to the regions
  you want to run AutoSpotting against. For this you will need to use the
  regional CloudFormation
  [template](https://s3.amazonaws.com/cloudprowess/nightly/regional_template.yaml).
- This regional StackSet will need a couple of parameters that enable it to send
  events to the main Lambda function: the ARN of the main Lambda function and
  the ARN of the regional execution IAM role created by the main AutoSpotting
  Stack.
- These would be set automatically when installing the main AutoSpotting
  CloudFormation template with the default parameters, but they need to be
  manually set if the regional stack is installed manually. You can get these
  values from the Outputs of the main AutoSpotting CloudFormation Stack that
  corresponds to the regional Stack you want to install, in particular the
  `AutoSpottingLambdaARN` and `LambdaRegionalStackExecutionRoleARN` output
  values.

## Known issues

- As mentioned above, the current StackSet implementation requires manual
  workarounds inc certain situations, such as when the StackSet self-managed
  StackSet permissions already exist in the account or when AutoSpotting is
  installed multiple times within an account. The above Workarounds may help in
  such situations.
- Parallel installations using StackSets will require the installation in legacy
  mode by setting the `DeployRegionalResourcesStackSet` parameter to `false` on
  all but the first StackSet, and performing the same workarounds on each target
  account where the subsequest StackSets are deployed.

## Support

As always, if you need Enterprise support for more exotic or large
configurations, you can get in touch on [gitter](https://gitter.im/cristim).
