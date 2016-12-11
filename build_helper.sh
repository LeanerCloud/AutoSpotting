#!/bin/bash

SHA=$(git rev-parse HEAD | cut -c 1-7)
BUILD=${TRAVIS_BUILD_NUMBER:-$SHA}

# upstream data
EC2_INSTANCES_INFO_COMMIT_SHA=ac8f6729ba24df485e42395941045d827be2a67d
INSTANCES_URL="https://raw.githubusercontent.com/powdahound/ec2instances.info/${EC2_INSTANCES_INFO_COMMIT_SHA}/www/instances.json"
BINDATA_FILE=generated_bindata.go

LOCAL_PATH=build/s3/dv

# default bucket name, can be overridden using an environment variable
BUCKET_NAME=${BUCKET_NAME:-cloudprowess}

# dependencies
DEPS="wget git go docker"


function clean {
  rm -rf data
  rm -rf ${BINDATA_FILE}
}

function check_dep {
  command -v $1 >/dev/null 2>&1 || {
    echo >&2 "The required dependency program '$1' is not installed. Aborting."
    exit 1
  }
}

function check_all_dependencies {
  echo "Checking for the presence of the following dependencies: ${DEPS}"

  for DEP in $DEPS
  do
    check_dep $DEP && echo "Found ${DEP}"
  done
  echo "All dependencies were successfully found, we're good to go!"
}

function prepare_bindata {
  check_all_dependencies
  type go-bindata || go get -u github.com/jteeuwen/go-bindata/...
  mkdir -p data
  wget -nv -c ${INSTANCES_URL} -O data/instances.json
  echo ${BUILD} > data/BUILD
  go-bindata -o ${BINDATA_FILE} -nometadata data/
  go fmt ${BINDATA_FILE}
}

function build_lambda_binary {
  docker run --rm -v ${GOPATH}:/go -v ${PWD}:/tmp eawsy/aws-lambda-go
}

function prepare_upload_data {
  rm -rf ${LOCAL_PATH}
  mkdir -p ${LOCAL_PATH}
  mv handler.zip ${LOCAL_PATH}/lambda.zip
}

function prepare_cloudformation_code {
  cp -f cloudformation/stacks/AutoSpotting/template.json ${LOCAL_PATH}/template.json
  cp -f cloudformation/stacks/AutoSpotting/template.json ${LOCAL_PATH}/template_build_${BUILD}.json
  cp -f ${LOCAL_PATH}/lambda.zip ${LOCAL_PATH}/lambda_build_${BUILD}.zip
  cp -f ${LOCAL_PATH}/lambda.zip ${LOCAL_PATH}/lambda_build_${SHA}.zip
}

function build_local {
  go build ${GOFLAGS}
}

function upload {
  aws s3 sync build/s3/ s3://${BUCKET_NAME}/
}

function calculate_coverage {
  go test -covermode=count -coverprofile=coverage.out ./core
  go tool cover -html=coverage.out
}

case "$1" in
    clean)
      clean
    ;;
    prepare_bindata)
      prepare_bindata
    ;;
    build_lambda_binary)
      build_lambda_binary
    ;;
    prepare_upload_data)
      prepare_upload_data
    ;;
    build_local)
      build_local
    ;;
    upload)
      upload
    ;;
    calculate_coverage)
      calculate_coverage
    ;;
    *)
        echo -e "Unknown target $1\nUsage: $SCRIPTNAME {clean|prepare_bindata|build_lambda_binary|prepare_upload_data|build_local|upload|calculate_coverage}" >&2
        exit 1
        ;;
esac
