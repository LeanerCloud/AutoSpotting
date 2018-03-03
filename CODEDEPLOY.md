# Use AutoSpotting with AWS CodeDeploy

## CodeDeploy Limitations

- Doesn't work on spot instances natively
- Doesn't work on instances that aren't booted by the autoscaling group

## Why this method

This method is to allow for AutoSpotting and spot instances to work around the
limitations of CodeDeploy and get our code on newly booted spot instances

## CodeDeploy Console

- Setup the AWS CodeDeploy Deployment Groups to use Tag Groups
- Groups should be based around the autoscaling groups you plan to use
- For example:
  - Environment:staging
  - Product:nginx
  - Role:web

## Instance AMI Scripts

### get-meta

- This file will be sourced into our deployment script
  - [get-meta](https://gist.github.com/cristim/82fc6bfe56c67a22ee264a0e3b655df5)
  - Save this file to /usr/bin/get-meta on the AMI to be used

### check-codedeploy

- A simple version of a deployment script that is ran on-boot
- This file will need to be deployed to the same AMI
  - [check-codedeploy](https://gist.github.com/cristim/7e9cd403fbf38aee18c4fb6a30bcef0a)
- If you are using Amazon Linux saving this file to `/etc/rc3.d/S99deploycode`
  - This will make run it after all networking components are available
