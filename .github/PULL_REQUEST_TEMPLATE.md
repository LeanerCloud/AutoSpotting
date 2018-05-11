# Issue Type

<!--
Pick one below and delete the others.

Also feel free to delete these comments for brevity once they were fully
acknowledged.
-->

- Feature Pull Request
- Bugfix Pull Request
- Documentation Pull Request

## Summary

<!--
Describe the change, including rationale and design decisions, and use a brief
but descriptive PR title.

Please include "Fixes #nnn" here or in your commit message in order to
link to an issue in which you describe the addressed problem in more detail.

If you want to address more issues do it in different pull requests instead of a
single big one, it will speed up the review process considerably.

Code review process:

The issue should be tagged with 'review wanted' label before the checklist below
is satisfied from the perspective of the PR author and the code is ready to be
peer-reviewed.

The label should be set by a maintainer as soon as the review is requested on
Gitter and should only be cleared after the PR is merged.
-->

## Code contribution checklist

<!--
The code review should be largely a matter of going through this checklist.
-->

1. [ ] The contribution fixes a single existing github issue, and it is linked
   to it.
1. [ ] The code is as simple as possible, readable and follows the idiomatic Go
  [guidelines](https://golang.org/doc/effective_go.html).
1. [ ] All new functionality is covered by automated test cases so the overall
  test coverage doesn't decrease.
1. [ ] No issues are reported when running `make full-test`.
1. [ ] Functionality not applicable to all users should be configurable.
1. [ ] Configurations should be exposed through Lambda function environment
   variables which are also passed as parameters to the
   [CloudFormation](https://github.com/cristim/autospotting/blob/master/cloudformation/stacks/AutoSpotting/template.yaml)
   and
   [Terraform](https://github.com/cristim/autospotting/blob/master/terraform/autospotting.tf)
   stacks defined as infrastructure code.
1. [ ] Global configurations set from the infrastructure stack level should also
   support per-group overrides using tags.
1. [ ] Tags names and expected values should be similar to the other existing
   configurations.
1. [ ] Both global and tag-based configuration mechanisms should be tested and
  proven to work using log output from various test runs.
1. [ ] The logs should be kept as clean as possible (use log levels as
  appropriate) and formatted consistently to the existing log output.
1. [ ] The documentation is updated to cover the new behavior, as well as the
   new configuration options for both stack parameters and tag overrides.
1. [ ] A code reviewer reproduced the problem and can confirm the code
  contribution actually resolves it.
