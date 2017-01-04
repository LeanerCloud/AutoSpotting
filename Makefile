DEPS := "wget git go docker"

BINARY := autospotting
BINARY_PKG := ./core
COVER_PROFILE := /tmp/coverage.out
BUCKET_NAME ?= cloudprowess
LOCAL_PATH := build/s3/dv

DOCKER_IMG := eawsy/aws-lambda-go

BINDATA_DIR := data
BINDATA_FILE := generated_bindata.go

SHA := $(shell git rev-parse HEAD | cut -c 1-7)
BUILD := $(or $(TRAVIS_BUILD_NUMBER), $(TRAVIS_BUILD_NUMBER), $(SHA))

# upstream data
EC2_INSTANCES_INFO_COMMIT_SHA := e655e36660bc617713c4ef9a1409cc65a209cb27
INSTANCES_URL := "https://raw.githubusercontent.com/powdahound/ec2instances.info/$(EC2_INSTANCES_INFO_COMMIT_SHA)/www/instances.json"

all: fmt-check vet-check build_local test                                                   ## Build the code
.PHONY: all

clean:                                                                      ## Remove installed packages/temporary files
	go clean ./...
	rm -rf $(BINDATA_DIR) $(BINDATA_FILE)
	make -f Makefile.lambda clean
.PHONY: clean

check_deps:                                                                 ## Verify the system has all dependencies installed
	@for DEP in $(shell echo "$(DEPS)"); do \
		command -v "$$DEP" &>/dev/null \
		|| (echo "Error: dependency '$$DEP' is absent" ; exit 1); \
	done
	@echo "all dependencies satisifed: $(DEPS)"
.PHONY: check_deps

build_deps:
	@go get github.com/mattn/goveralls
	@go get golang.org/x/tools/cmd/cover
	@docker pull eawsy/aws-lambda-go-shim:latest
.PHONY: build_deps

prepare_bindata: check_deps build_deps                                      ## Convert instance data into go file
	@type go-bindata || go get -u github.com/jteeuwen/go-bindata/...
	@mkdir -p $(BINDATA_DIR)
	wget --quiet -nv $(INSTANCES_URL) -O $(BINDATA_DIR)/instances.json
	@echo $(BUILD) > $(BINDATA_DIR)/BUILD
	go-bindata -o $(BINDATA_FILE) -nometadata $(BINDATA_DIR)
	gofmt -l -s -w $(BINDATA_FILE)
	go get ./...
.PHONY: prepare_bindata

build_lambda_binary: prepare_bindata                                        ## Build lambda binary
	make -f Makefile.lambda docker
.PHONY: build_lambda_binary

prepare_upload_data: build_lambda_binary                                    ## Create archive to be uploaded
	@rm -rf $(LOCAL_PATH)
	@mkdir -p $(LOCAL_PATH)
	@mv handler.zip $(LOCAL_PATH)/lambda.zip
	@cp -f cloudformation/stacks/AutoSpotting/template.json $(LOCAL_PATH)/template.json
	@cp -f cloudformation/stacks/AutoSpotting/template.json $(LOCAL_PATH)/template_build_$(BUILD).json
	@cp -f $(LOCAL_PATH)/lambda.zip $(LOCAL_PATH)/lambda_build_$(BUILD).zip
	@cp -f $(LOCAL_PATH)/lambda.zip $(LOCAL_PATH)/lambda_build_$(SHA).zip
	@make -f Makefile.lambda clean
.PHONY: prepare_upload_data

build_local: prepare_bindata                                                ## Build binary - local dev
	go build $(GOFLAGS) -o $(BINARY)
.PHONY: build_local

upload: prepare_upload_data                                                 ## Upload binary
	aws s3 sync build/s3/ s3://$(BUCKET_NAME)/
.PHONY: upload

vet-check:                                                                  ## Verify vet compliance
ifeq ($(shell go tool vet -all -shadow=true . 2>&1 | wc -l), 0)
	@printf "ok\tall files passed go vet\n"
else
	@printf "error\tsome files did not pass go vet\n"
	@go tool vet -all -shadow=true . 2>&1
endif
.PHONY: vet-check

fmt:
	gofmt -l -s -w .
.PHONY: fmt

fmt-check:                                                                  ## Verify fmt compliance
ifneq ($(shell gofmt -l -s . | wc -l), 0)
	@printf "error\tsome files did not pass go fmt, fix the following formatting diff or run 'make fmt'\n"
	@gofmt -l -s -d .
	@exit 1
else
	@printf "ok\tall files passed go fmt\n"
endif
.PHONY: fmt-check

test:                                                                       ## Test go code and coverage
	@go test -covermode=count -coverprofile=$(COVER_PROFILE) $(BINARY_PKG)
.PHONY: test

full-test: fmt-check vet-check test                                         ## Pass test / fmt / vet
.PHONY: full-test

html-cover: test                                                            ## Display coverage in HTML
	@go tool cover -html=$(COVER_PROFILE)
.PHONY: html-cover

travisci-cover: html-cover                                                  ## Generate coverage in the TravisCI format, fails unless executed from TravisCI
	@goveralls -coverprofile=$(COVER_PROFILE) -service=travis-ci
.PHONY: travisci-cover

travisci-checks: fmt-check vet-check                                        ## Pass fmt / vet and calculate test coverage
.PHONY: travisci-checks

travisci: prepare_bindata travisci-checks prepare_upload_data travisci-cover## Executed by TravisCI
.PHONY: travisci

help:                                                                       ## Show this help
	@printf "Rules:\n"
	@fgrep -h "##" $(MAKEFILE_LIST) | fgrep -v fgrep | sed -e 's/\\$$//' | sed -e 's/##//'
.PHONY: help
