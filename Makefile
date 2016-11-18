BUCKET_NAME ?= cloudprowess

LOCAL_PATH = build/s3/dv

SHA = $(shell git rev-parse HEAD | cut -c 1-7)
BUILD := $(or ${TRAVIS_BUILD_NUMBER}, ${TRAVIS_BUILD_NUMBER}, ${SHA})

# upstream data
EC2_INSTANCES_INFO_COMMIT_SHA=f34075aa09c52233735cd879ebda3f70d77b7ca5
INSTANCES_URL="https://raw.githubusercontent.com/powdahound/ec2instances.info/${EC2_INSTANCES_INFO_COMMIT_SHA}/www/instances.json"
BINDATA_FILE=generated_bindata.go

all: build_local

clean:
	rm -rf data
	rm -rf ${BINDATA_FILE}

bindata:
	./check_deps.sh
	type go-bindata || go get -u github.com/jteeuwen/go-bindata/...
	mkdir -p data
	wget -nv -c ${INSTANCES_URL} -O data/instances.json
	echo ${BUILD} > data/BUILD
	go-bindata -o ${BINDATA_FILE} -nometadata data/


build_lambda_binary: bindata
	docker run --rm -v ${GOPATH}:/go -v ${PWD}:/tmp eawsy/aws-lambda-go

prepare_upload_data: build_lambda_binary
	rm -rf ${LOCAL_PATH}
	mkdir -p ${LOCAL_PATH}
	mv handler.zip ${LOCAL_PATH}/lambda.zip
	cp -f cloudformation/stacks/AutoSpotting/template.json ${LOCAL_PATH}/template.json
	cp -f cloudformation/stacks/AutoSpotting/template.json ${LOCAL_PATH}/template_build_${BUILD}.json
	cp -f ${LOCAL_PATH}/lambda.zip ${LOCAL_PATH}/lambda_build_${BUILD}.zip
	cp -f ${LOCAL_PATH}/lambda.zip ${LOCAL_PATH}/lambda_build_${SHA}.zip

# the following targets are only used for local development

build_local: bindata
	go build $(GOFLAGS)

upload: prepare_upload_data
	aws s3 sync build/s3/ s3://${BUCKET_NAME}/

test: build_local
	./autospotting

cover:
	go test -covermode=count -coverprofile=coverage.out ./core
	go tool cover -html=coverage.out

