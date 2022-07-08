export VERSION=1.1.0-rc4

export SAVINGS_CUT=$1
export DOCKER_IMAGE_VERSION=$VERSION-$2

export FLAVOR=stable
export GOARCH=amd64

make docker-marketplace
make artifacts

export AWS_PROFILE=personal
unset AWS_SESSION_TOKEN AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY
make upload
