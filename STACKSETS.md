# Alternative way to deploy using CF StackSets

## Instructions

1. Grant proper permissions for using StackSets:
   [instructions](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/stacksets-prereqs.html).
1. Create Stackset with parameter **DeployUsingStackSets=True** in "Base/Main" region,
   set parameter **StackSetsMainRegion** equals to that region (default to us-east-1)
   and add that region as first when you specify in which regions deploy stack.

### Internals

I use a CF condition to "know" if the stack is been creating in the
Base/Main region.

In that case the CW event for spot termination direclty trigger the main
autospotting lambda, so lambda regional resource is not created at all.

In all other cases, only the resources related to the spot termination
are created (CW event, lambda and relative permissions),
but to work they need the AutoSpottingLambda Arn from the main stack.

We retrieve it automatically using CF Custom Resource and inline Lambda
and add it as an additional key/value to the cloudwatch event input (InputTransformer).
