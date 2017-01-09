DEPS := "wget git go docker"

BINARY := autospotting
BINARY_PKG := ./core
COVER_PROFILE := /tmp/coverage.out
BUCKET_NAME ?= cloudprowess
LOCAL_PATH := build/s3/dv

DOCKER_IMG := eawsy/aws-lambda-go

GO_FILES := $(shell find ./ -name '*.go')

BINDATA_DIR := data
BINDATA_FILE := generated_bindata.go

SHA := $(shell git rev-parse HEAD | cut -c 1-7)
BUILD := $(or $(TRAVIS_BUILD_NUMBER), $(TRAVIS_BUILD_NUMBER), $(SHA))

# upstream data
EC2_INSTANCES_INFO_COMMIT_SHA := e655e36660bc617713c4ef9a1409cc65a209cb27
INSTANCES_URL := "https://raw.githubusercontent.com/powdahound/ec2instances.info/$(EC2_INSTANCES_INFO_COMMIT_SHA)/www/instances.json"

all: build_local                                             ## Build the code

clean:                                                       ## Remove installed packages/temporary files
	go clean ./...
	rm -rf $(BINDATA_DIR) $(BINDATA_FILE)

check_dep:                                                   ## Verify the system has all dependencies installed
	@for DEP in $(shell echo "$(DEPS)"); do \
		command -v "$$DEP" &>/dev/null \
		|| (echo "Error: dependency '$$DEP' is absent" ; exit 1); \
	done
	@echo "all dependencies satisifed: $(DEPS)"

prepare_bindata: check_dep                                   ## Convert instance data into go file
	go get ./...
	@type go-bindata || go get -u github.com/jteeuwen/go-bindata/...
	@mkdir -p $(BINDATA_DIR)
	wget --quiet -nv $(INSTANCES_URL) -O $(BINDATA_DIR)/instances.json
	@echo $(BUILD) > $(BINDATA_DIR)/BUILD
	go-bindata -o $(BINDATA_FILE) -nometadata $(BINDATA_DIR)
	@gofmt -w $(BINDATA_FILE)

build_lambda_binary: bindata                                 ## Build lambda binary
	docker run --rm -v $(GOPATH):/go -v $(PWD):/tmp $(DOCKER_IMG)

prepare_upload_data: build_lambda_binary                     ## Create archive to be uploaded
	@rm -rf $(LOCAL_PATH)
	@mkdir -p $(LOCAL_PATH)
	@mv handler.zip $(LOCAL_PATH)/lambda.zip
	@cp -f cloudformation/stacks/AutoSpotting/template.json $(LOCAL_PATH)/template.json
	@cp -f cloudformation/stacks/AutoSpotting/template.json $(LOCAL_PATH)/template_build_$(BUILD).json
	@cp -f $(LOCAL_PATH)/lambda.zip $(LOCAL_PATH)/lambda_build_$(BUILD).zip
	@cp -f $(LOCAL_PATH)/lambda.zip $(LOCAL_PATH)/lambda_build_$(SHA).zip

build_local: bindata                                         ## Build binary - local dev
	go build $(GOFLAGS) -o $(BINARY)

upload: prepare_upload_data                                  ## Upload binary
	aws s3 sync build/s3/ s3://$(BUCKET_NAME)/

vet-check:                                                   ## Verify vet compliance
ifeq ($(shell go tool vet -all -shadow=true $(GO_FILES) 2>&1 | wc -l), 0)
	@printf "ok\tall files passed go vet\n"
else
	@printf "error\tsome files did not pass go vet\n"
	@go tool vet -all -shadow=true $(GO_FILES) 2>&1
endif

fmt-check:                                                   ## Verify fmt compliance
ifneq ($(shell gofmt -l $(GO_FILES) | wc -l), 0)
	@printf "error\tsome files did not pass go fmt: $(shell gofmt -l $(GO_FILES))\n"; exit 1
else
	@printf "ok\tall files passed go fmt\n"
endif

test:                                                        ## Test go code and coverage
	@go test -covermode=count -coverprofile=$(COVER_PROFILE) $(BINARY_PKG)

full-test: test fmt-check vet-check                          ## Pass test / fmt / vet

html-cover: test                                             ## Display coverage in HTML
	@go tool cover -html=$(COVER_PROFILE)

help:                                                        ## Show this help
	@printf "Rules:\n"
	@fgrep -h "##" $(MAKEFILE_LIST) | fgrep -v fgrep | sed -e 's/\\$$//' | sed -e 's/##//'

.PHONY: all clean check_dep bindata build_lambda_binary prepare_upload_data build_local upload vet-check fmt-check
.PHONY: test full-test html-cover help
