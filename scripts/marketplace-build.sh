export VERSION=1.1.0-rc5

export SAVINGS_CUT=$1
export DOCKER_IMAGE_VERSION=$VERSION

export FLAVOR=stable
export GOARCH=amd64

make docker-marketplace
make artifacts

export AWS_PROFILE=personal
unset AWS_SESSION_TOKEN AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY
make upload

export TEMPLATE_URL="https://s3.amazonaws.com/cloudprowess/$FLAVOR/$VERSION/template.yaml"
export CFN_INSTALL_PREFIX=https://console.aws.amazon.com/cloudformation/home?region=us-east-1#/stacks/create/template?stackName=AutoSpotting&templateURL

echo CloudFormation template URL: $TEMPLATE_URL
echo CloudFormation quick install URL: $CFN_INSTALL_PREFIX=$TEMPLATE_URL
