#!/bin/sh

## You'll need github.com/ktruckenmiller/docker-friend or an assumed role to do this.
docker run -it \
  -v $(pwd):/work \
  -e deploy_subdirectory=deployment/ \
  -e environ=dev \
  -e aws_region=us-west-2 \
  -e IAM_ROLE="BDPT2" \
  056684691971.dkr.ecr.us-east-1.amazonaws.com/bdp/ansible:2.2.0
