# Use AutoSpotting with AWS CodeDeploy #

## CodeDeploy Limitations:
- Doesn't work on SPOT instances natively
- Doesn't work on instances that aren't booted by the autoscaling group

## Why this method:

- This method is to allow for AutoSpotting and SPOT instances to work around the limitations of CodeDeploy and get our code on newly booted SPOT instances.


##  CodeDeploy Console
- Setup the AWS CodeDeploy Deployment Groups to use Tag Groups
	1.	Groups should be based around the autoscaling groups you plan to use
	2.	For example:
		3.	Environment:staging
		4.	Product:nginx
		5.	Role:web

## Instance AMI

### get-meta

- https://gist.github.com/sc-chad/99ba78a7cb1e7b5573ea131cf2015cad
- Save this file to /usr/bin/get-meta on the SPOT AMI to be used.
- This file will be sourced into our deployment script and provide us the tags to look for

### check-codedeploy

- A simple version of a deployment script that is ran on-boot
- https://gist.github.com/sc-chad/ae0f4acbb5b7283a2dc0b25a3277cf50
- This file will need to be deployed to the same AMI that the SPOT instances will use
- If you are using Amazon Linux saving this file to /etc/rc3.d/S99deploycode will run it after all networking components are available
