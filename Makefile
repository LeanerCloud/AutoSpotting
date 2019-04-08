DEPS := "wget git go docker golint zip"

BINARY := autospotting
BINARY_PKG := ./core
CORE_GOFILES := $(shell find core -type f -name '*.go')
MAIN_GOFILES := $(shell find . -type f -name '*.go' -not -path "./core/*" -not -path "./vendor/*" )
COVER_PROFILE := /tmp/coverage.out
BUCKET_NAME ?= cloudprowess
FLAVOR ?= custom
LOCAL_PATH := build/s3/$(FLAVOR)
LICENSE_FILES := LICENSE

SHA := $(shell git rev-parse HEAD | cut -c 1-7)
BUILD := $(or $(TRAVIS_BUILD_NUMBER), $(TRAVIS_BUILD_NUMBER), $(SHA))

ifneq ($(FLAVOR), custom)
    LICENSE_FILES += BINARY_LICENSE
endif

LDFLAGS="-X main.Version=$(FLAVOR)-$(BUILD) -s -w"

all: fmt-check vet-check build test                          ## Build the code
.PHONY: all

clean:                                                       ## Remove installed packages/temporary files
	go clean ./...
	rm -rf $(BINDATA_DIR) $(LOCAL_PATH)
.PHONY: clean

check_deps:                                                  ## Verify the system has all dependencies installed
	@for DEP in "$(DEPS)"; do \
		if ! command -v "$$DEP" >/dev/null ; then echo "Error: dependency '$$DEP' is absent" ; exit 1; fi; \
	done
	@echo "all dependencies satisifed: $(DEPS)"
.PHONY: check_deps

build_deps:
	@go get -u github.com/mattn/goveralls
	@go get -u golang.org/x/lint/golint
	@go get -u golang.org/x/tools/cmd/cover
.PHONY: build_deps

update_deps:												 ## Update all dependencies
	@dep ensure -update
.PHONY: update_deps

build: build_deps                                            ## Build autospotting binary
	GOOS=linux go build -ldflags=$(LDFLAGS) -o $(BINARY)
.PHONY: build

archive: build                                               ## Create archive to be uploaded
	@rm -rf $(LOCAL_PATH)
	@mkdir -p $(LOCAL_PATH)
	@zip $(LOCAL_PATH)/lambda.zip $(BINARY) $(LICENSE_FILES)
	@cp -f cloudformation/stacks/AutoSpotting/template.yaml $(LOCAL_PATH)/
	@cp -f cloudformation/stacks/AutoSpotting/template.yaml $(LOCAL_PATH)/template_build_$(BUILD).yaml
	@zip -j $(LOCAL_PATH)/regional_stack_lambda.zip cloudformation/stacks/AutoSpotting/regional_stack_lambda.py
	@cp -f cloudformation/stacks/AutoSpotting/regional_template.yaml $(LOCAL_PATH)/
	@sed -i "s#lambda\.zip#lambda_build_$(BUILD).zip#" $(LOCAL_PATH)/template_build_$(BUILD).yaml
	@cp -f $(LOCAL_PATH)/lambda.zip $(LOCAL_PATH)/lambda_build_$(BUILD).zip
	@cp -f $(LOCAL_PATH)/lambda.zip $(LOCAL_PATH)/lambda_build_$(SHA).zip
.PHONY: archive

upload: archive                                              ## Upload binary
	aws s3 sync build/s3/ s3://$(BUCKET_NAME)/
.PHONY: upload

vet-check:                                                   ## Verify vet compliance
ifeq ($(shell go vet -all $(CORE_GOFILES) 2>&1 | wc -l | tr -d '[:space:]'), 0)
	@printf "ok\tall core files passed go vet\n"
else
	@printf "error\tsome core files did not pass go vet\n"
	@go vet -all $(CORE_GOFILES) 2>&1
endif
ifeq ($(shell go vet -all $(MAIN_GOFILES) 2>&1 | wc -l | tr -d '[:space:]'), 0)
	@printf "ok\tall main files passed go vet\n"
else
	@printf "error\tsome main files did not pass go vet\n"
	@go vet -all $(MAIN_GOFILES) 2>&1
endif
.PHONY: vet-check

fmt-check:                                                   ## Verify fmt compliance
ifeq ($(shell gofmt -l -s $(CORE_GOFILES) | wc -l | tr -d '[:space:]'), 0)
	@printf "ok\tall core files passed go fmt\n"
else
	@printf "error\tsome core files did not pass go fmt, fix the following formatting diff:\n"
	@gofmt -l -s -d $(CORE_GOFILES)
	@exit 1
endif
ifeq ($(shell gofmt -l -s $(MAIN_GOFILES) | wc -l | tr -d '[:space:]'), 0)
	@printf "ok\tall main files passed go fmt\n"
else
	@printf "error\tsome main files did not pass go fmt, fix the following formatting diff:\n"
	@gofmt -l -s -d $(MAIN_GOFILES)
	@exit 1
endif
.PHONY: fmt-check

test:                                                        ## Test go code and coverage
	@go test -covermode=count -coverprofile=$(COVER_PROFILE) $(BINARY_PKG)
.PHONY: test

lint:
	@golint -set_exit_status $(BINARY_PKG)
	@golint -set_exit_status .
.PHONY: lint

full-test: fmt-check vet-check test lint                     ## Pass test / fmt / vet / lint
.PHONY: full-test

html-cover: test                                             ## Display coverage in HTML
	@go tool cover -html=$(COVER_PROFILE)
.PHONY: html-cover

travisci-cover: html-cover                                   ## Test & generate coverage in the TravisCI format, fails unless executed from TravisCI
	@goveralls -coverprofile=$(COVER_PROFILE) -service=travis-ci -repotoken=$(COVERALLS_TOKEN)
.PHONY: travisci-cover

travisci-checks: fmt-check vet-check lint                    ## Pass fmt / vet & lint format
.PHONY: travisci-checks

travisci: archive travisci-checks travisci-cover             ## Executed by TravisCI
.PHONY: travisci

help:                                                        ## Show this help
	@printf "Rules:\n"
	@fgrep -h "##" $(MAKEFILE_LIST) | fgrep -v fgrep | sed -e 's/\\$$//' | sed -e 's/##//'
.PHONY: help
