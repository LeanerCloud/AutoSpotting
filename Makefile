BINARY=autospotting
LAMBDA_BINARY=autospotting_lambda
LOCAL_PATH=build/s3/dv
BUCKET_NAME=cloudprowess

# upstream data
#INSTANCES_URL="https://raw.githubusercontent.com/powdahound/ec2instances.info/master/www/instances.json"
# my Github fork
INSTANCES_URL="https://raw.githubusercontent.com/cristim/ec2instances.info/master/www/instances.json"

all: build_local

release: upload

check_deps:
	./check_deps.sh

build_local: check_deps
	go build $(GOFLAGS)

build_lambda_binary: check_deps
	GOOS=linux GOARCH=amd64 go build -o ${LAMBDA_BINARY}

strip: build_lambda_binary
	strip ${LAMBDA_BINARY}

lambda: strip
	git rev-parse HEAD | cut -c 1-8 > lambda/GIT_SHA
	mv ${LAMBDA_BINARY} lambda/
	curl ${INSTANCES_URL} --output lambda/instances.json
	zip -9 -v -j lambda lambda/*
	rm -rf ${LOCAL_PATH}
	mkdir -p ${LOCAL_PATH}
	mv lambda.zip ${LOCAL_PATH}

install: lambda

upload: install
	aws s3 sync build/s3/ s3://${BUCKET_NAME}/

test: build_local
	./autospotting core/test_data/json_instance/instances.json

cover:
	go test -covermode=count -coverprofile=coverage.out ./core
	go tool cover -html=coverage.out
