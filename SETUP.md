# AutoSpotting Setup #

It's relatively easy to build and install your own version of this tool's binaries, 
removing your dependency on the author's version, and allowing any customizations
and improvements your organization needs. You'll need to set up a local environment
to run Go, compile the binaries locally, upload them to an S3 bucket in your AWS
account, and update the CloudFormation stack to use those new binaries.

## Compiling the binaries locally ##

1. Install Go via the binaries [here](https://golang.org/dl/) or Homebrew:

   `brew install go`
2. Verify that Go is installed

   `go version`
   
3. Set up a directory for your Go development. I'm using `godev` in my home directory
for this example.

4. Set the `GOPATH` environment variable to point at your `godev` directory:

   `export GOPATH=$HOME/godev`
   
   Optionally add this line to your .bash_profile to persist across console sessions.

5. Navigate to your `godev` directory and run the following to bring in the AutoSpotting 
   project:
   
   `go get github.com/cristim/autospotting`
   
   This will download the source from GitHub as well as pull in any necessary 
   dependencies.

6. Navigate to the root of the AutoSpotting repository:

   `cd src/github.com/cristim/autospotting`

7. Try building and running the tests to make sure everything pulled in correctly. More 
   details on the available directives below.

   `make install`

   `make test`

8. (Optional) You may want to make a minor change to the source code so you can tell when
   the tool is running your own custom-built version. If so, add a line like this to the 
   `autospotting.go` file's `main()` function:

   `fmt.Println("Running <my organization name> binaries")`

## Using your own binaries in AWS ##

1. Set up an S3 bucket in AWS that will host the binaries

2. The Makefile contains a `BUCKET_NAME` variable that tells it where to upload new
   binaries. Edit the Makefile and change it from the default `cloudprowess` to the name 
   of your S3 bucket.

3. Build and upload your binaries to the S3 bucket

   `make release`

4. If you're already set up to use the tool with the author's binaries, update your
   existing CloudFormation stack, and change `cloudprowess` to your bucket name in the 
   `LambdaS3Bucket` field.

   Otherwise, follow the steps in [this blog post](http://blog.cloudprowess.com/autoscaling/aws/ec2/spot/2016/04/26/automatic-replacement-of-autoscaling-nodes-with-equivalent-spot-instances-seeing-it-in-action.html)
   to get it installed, replacing `cloudprowess` with your S3 bucket name in the 
   `LambdaS3Bucket` field on the Stack Parameters section of the configuration.

   ![LambdaS3Bucket Configuration](https://mcristi.files.wordpress.com/2016/04/installationcloudformation2.png)

5. Save the CloudFormation configuration and let it create/update the resources. The 
   tool should now be running against the binaries you built locally and uploaded to 
   your own S3 bucket.

## Make directives ##

Use these directives defined in the `Makefile` to build, release, and test the tool:

* **install**:
   * Verifies that the necessary dependencies are installed
   * Runs `go build` to compile the project for deployment
   * Retrieves the `instances.json` file
   * Bundles the binaries into a `lambda.zip` file under `build/s3/dv`
* **release**:
   * Runs the `install` directives
   * Uploads the binaries from `build/s3` to the specified S3 bucket
* **test**:
   * Runs `go build` to compile the project locally
   * Runs the tool against a JSON file of test data


[Back to the main Readme](./README.md)