# AutoSpotting Setup #

It's relatively easy to build and install your own version of this tool's
binaries, removing your dependency on the author's version, and allowing any
customizations and improvements your organization needs. You'll need to set up a
local environment to run Go, compile the binaries locally, upload them to an S3
bucket in your AWS account, and update the CloudFormation stack to use those new
binaries.

## Dependencies ##

1. Install [Go](https://golang.org/dl/), [git](https://git-scm.com/downloads),
   [Docker](https://www.docker.com/) and the [AWS command-line
   tool](https://aws.amazon.com/cli/). You may use the official binaries or your
   usual package manager, whatever you prefer is fine.

1. Verify that they were properly installed.

   `go version`
   `git version`
   `docker version`
   `aws --version`

## Compiling the binaries locally ##

1. Set up a directory for your Go development. I'm using `godev` in my home
   directory for this example.

1. Set the `GOPATH` environment variable to point at your `godev` directory:

   `export GOPATH=$HOME/godev`

   Optionally add this line to your .bash_profile to persist across console
   sessions.

1. Navigate to your `godev` directory and run the following to bring in the
   AutoSpotting project:

   `go get github.com/cristim/autospotting`

   This will download the source from GitHub as well as pull in any necessary
   dependencies.

1. Navigate to the root of the AutoSpotting repository:

   `cd src/github.com/cristim/autospotting`

1. Try building and running the code locally to make sure everything works
   correctly. More details on the available directives below.

   `make test`

1. (Optional) You may want to make a minor change to the source code so you can
   tell when the tool is running your own custom-built version. If so, add a
   line like this to the `autospotting.go` file's `main()` function:

   `fmt.Println("Running <my organization name> binaries")`

## Using your own binaries in AWS ##

1. Set up an S3 bucket in your AWS account that will host your custom binaries.

1. The Makefile can use a `BUCKET_NAME` variable that tells it where to upload
   new binaries. Set it into your environment to the name of your S3 bucket.

   `export BUCKET_NAME=my-bucket`

1. Define some AWS credentials or profile information into your
   [environment](http://docs.aws.amazon.com/cli/latest/userguide/cli-chap-getting-started.html#cli-environment).

1. Build and upload your binaries to the S3 bucket.
   `make upload`

1. If you're already set up to use the tool with the author's binaries, update
   your existing CloudFormation stack, and change the `LambdaS3Bucket` field to
   your S3 bucket name.

   Otherwise, follow the steps in [this blog
   post](http://blog.cloudprowess.com/autoscaling/aws/ec2/spot/2016/04/26/automatic-replacement-of-autoscaling-nodes-with-equivalent-spot-instances-seeing-it-in-action.html)
   to get it installed, replacing `cloudprowess` with your S3 bucket name in the
   `LambdaS3Bucket` field on the Stack Parameters section of the configuration.

   ![LambdaS3Bucket
   Configuration](https://mcristi.files.wordpress.com/2016/04/installationcloudformation2.png)

1. Save the CloudFormation configuration and let it create/update the resources.
   The tool should now be running against the binaries you built locally and
   uploaded to your own S3 bucket.

## Make directives ##

Use these directives defined in the `Makefile` to build, release, and test the
tool:

* **all (default, can be ommitted)**
  * Verifies that the necessary dependencies are installed.
  * Runs `go build` to compile the project for local development.

* **upload**
  * Prepares a special build designed to run in AWS Lambda.
  * Uploads the generated binaries from `build/s3` to the specified S3 bucket.

* **test**
  * Runs `go build` to compile the project locally.
  * Runs the tool locally

[Back to the main Readme](./README.md)
