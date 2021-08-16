# AutoSpotting Setup #

It's usually recommended to use the provided binaries available as Docker
images, but in some cases you may need to customize AutoSpotting for your own
environment.

## Docker ##

Pre-built Docker images for the latest evaluation builds are also available on
the Docker Hub at
[AutoSpotting/AutoSpotting](https://hub.docker.com/r/AutoSpotting/AutoSpotting/)

``` shell
docker run autospotting/autospotting
```

They might be useful for quick tests, otherwise you might need to build your own
docker images.

The repository contains a `Dockerfile` and `docker-compose` configuration that
allows you to build AutoSpotting Docker container images and run them
conveniently on your local machine without installing the Go build environment
usually required for local development(which is also documented below).

This can be useful for trying it out locally or even for running it on a
container hosting solution such as Kubernetes. They won't support the full
functionality that relies on CloudWatch Events but it's probably enough for some
people.

If you have `docker` and `docker-compose` installed, it's as simple as running

``` shell
docker-compose run autospotting
```

 This also accepts all the AutoSpotting command-line arguments, including
`-help` which explains all the other available options.

The usual AWS credential environment variables listed in the
`docker-compose.yaml` configuration file are passed to the running container and
will need to be set for it to actually work.

## Using your own Docker images in AWS Lambda ##

AutoSpotting uses a Lambda function configured to use a Docker image. Such a
configuration [currently](https://github.com/aws/containers-roadmap/issues/1281)
requires the Docker image to be stored in an ECR from your own account.

Also, in order for the generated Docker image to be compatible with the Lambda
runtime, the image must currently be built on a x86_64 host. Unfortunately ARM
hosts such as Apple M1 Macbook aren't currently supported because of QEMU
emulator crashes when performing the cross-compilation using `docker buildx`.

In order to support the AWS Marketplace setup, which relies on an ECR repository
hosted in another AWS-managed account, the current CloudFormation template uses
a custom resource that copies the Docker image from a source ECR (by default the
Marketplace ECR) into an ECR created inside the CloudFormation stack. This adds
some complexity but has the nice side effect of being able to push the image to
any arbitrary ECR in another account/region, offering more flexibility for
customers who may want to manage custom deployments at scale.

You'll therefore need to build an x86_64 Docker image, upload it to an ECR
repository in your AWS account and configure your CloudFormation or Terraform
stack to use this new image as a source image.

1. Set up an ECR repository in your AWS account that will host your custom
   Docker images.

1. The build system can use a `DOCKER_IMAGE` variable that tells it where to
   upload the image. Set it into your environment to the name of your ECR
   repository. When unset you'll attempt to push to the Marketplace ECR and
   you'll receive permission errors.

   ``` shell
   export DOCKER_IMAGE=1234567890123.dkr.ecr.<region>.amazonaws.com/<my-ecr-name>
   export DOCKER_IMAGE_VERSION=1.0.2 # it's strongly recommended versioning images
   ```

1. Define some AWS credentials or profile information into your
   [environment](http://docs.aws.amazon.com/cli/latest/userguide/cli-chap-getting-started.html#cli-environment).

1. Authenticate to your ECR repository

   ```shell
   make docker-login
   ```

1. Build and upload your Docker image to your ECR and configure a CloudFormation
   template to use your ECR

   ``` shell
   make docker-push-artifacts
   ```

1. Use the CloudFormation template from the `build` directory to create the
   resources. Make sure you set the parameters `SourceECR` and `SourceImage` to
   point to your ECR repository (`SourceECR` should be set to contain the
   hostname part of your ECR repository, before the first `/` character and
   `SourceImage` should contain the rest). The version number will be set
   automatically based on the value you defined earlier.

   AutoSpotting should now be running against the binaries you built locally and
   uploaded to your own ECR repository.

   The same process can be used for updating AutoSpotting to a newer version.

## Maintaining your own fork ##

It is recommended to contribute your changes into the mainline version of the
project whenever possible, so that others can benefit from your enhancements and
bug fixes, but for some reasons you may still want to run your own fork.

Unfortunately the golang import paths can make it tricky, but there is a nice
[article](http://code.openark.org/blog/development/forking-golang-repositories-on-github-and-managing-the-import-path)
which documents the problem in detail and gives a couple of possible
workarounds.

## Make directives ##

The Makefile from the root of the git repository contains a number of useful
directives, they're not documented here as they might change over time, so you
may want to have a look at it.

## Local Development setup ##

AutoSpotting is written in Go so for local development you need a Go toolchain.
You can probably also use docker-compose for this to avoid it as mentioned above
but I prefer the native Go tooling which offers faster feedback for local
development.

### Dependencies ##

1. Install [Go](https://golang.org/dl/), [git](https://git-scm.com/downloads),
   [Docker](https://www.docker.com/) and the [AWS command-line
   tool](https://aws.amazon.com/cli/). You may use the official binaries or your
   usual package manager, whatever you prefer is fine.

1. Verify that they were properly installed.

   `go version`, should be at least 1.7

   `git version`

   `docker version`

   `aws --version`

### Compiling the binaries locally ##

1. Set up a directory for your Go development. I'm using `go` in my home
   directory for this example.

1. Set the `GOPATH` environment variable to point at your `go` directory:

   `export GOPATH=$HOME/go`

   Optionally add this line to your .bash_profile to persist across console
   sessions.

1. Run the following command to install the AutoSpotting project into your
   GOPATH directory:

   `go get github.com/AutoSpotting/AutoSpotting`

   This downloads the source from GitHub, pulls in all necessary dependencies,
   builds it for local execution and deploys the binary into the golang binary
   directory which you may also want to append to your PATH.

1. Navigate to the root of the AutoSpotting repository:

   `cd $GOPATH/src/github.com/AutoSpotting/AutoSpotting`

1. (Optional) You may want to make a minor change to the source code so you can
   tell when the tool is running your own custom-built version. If so, add a
   line like this to the `autospotting.go` file's `main()` function:

   `fmt.Println("Running <my organization name> binaries")`

1. (Optional) Try building and running the test suite locally to make sure
   everything works correctly:

   `make test`

1. Build the code again:

   `make build`

### Running locally ###

1. Run the code, assuming you have AWS credentials defined in your environment
   or in the default AWS credentials profile:

   `./AutoSpotting`

   You may also pass some command line flags, see the `--help` output for more
   information on the available options.

   When you are happy with how your custom build behaves, you can generate a

   build for AWS Lambda using the Docker method documented above.

### Maintaining your own fork ##

It is recommended to contribute your changes into the mainline version of the
project whenever possible, so that others can benefit from your enhancements and
bug fixes, but for some reasons you may still want to run your own fork.

Unfortunately the golang import paths can make it tricky, but there is a nice
[article](http://code.openark.org/blog/development/forking-golang-repositories-on-github-and-managing-the-import-path)
which documents the problem in detail and gives a couple of possible
workarounds.

[Back to the main Readme](./README.md)
