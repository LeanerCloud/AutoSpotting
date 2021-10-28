make docker-marketplace
make artifacts

export AWS_PROFILE=personal
unset AWS_SESSION_TOKEN AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY
make upload
