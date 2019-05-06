# Alternative way to deploy using CF StackSets

## Instructions

1. Grant proper permissions for using StackSets:
[instructions](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/stacksets-prereqs.html).
2. Create Stacksets with parameter **StackSets=True** in your "Base/Main" region and
set parameter **StackSetsMainRegion** equals to that region (default to us-east-1).
3. Annotate stack Outputs:
   - AutoSpottingLambdaARN
   - LambdaRegionalExecutionRoleARN
4. Update just created StackSet setting StackSet CloudFormation Parameters:
   - AutoSpottingLambdaARN
   - LambdaRegionalExecutionRoleARN

   with the previously annoted values.
5. Update StackSet and add any other region you need (now or later).

If you use autospotting in only one region, you can stop after point 4.
(in theory you can stop after point 2, but is better to execute 3 and 4
too, this way if you later need another region you can just add it).

### Internals

I use a CF condition to "know" if the stack is been creating in the
Base/Main region.

In that case the CW event for spot termination direclty trigger the main
autospotting lambda, so lambda regional resource is not created at all.

In all other cases, only the resources related to the spot termination
are created (the ones in regional_template: CW event, lambda and relative permissions),
but to work they need the AutoSpottingLambda and LambdaRegionalExecutionRole Arns
from the main stack.
