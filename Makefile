DEPS := "wget git go docker golint zip"

BINARY := AutoSpotting

COVER_PROFILE := /tmp/coverage.out
BUCKET_NAME ?= cloudprowess
FLAVOR ?= custom
LOCAL_PATH := build/s3/$(FLAVOR)
LICENSE_FILES := LICENSE THIRDPARTY
GOOS ?= linux
GOARCH ?= amd64

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
	@go mod tidy -v
.PHONY: build_deps

update_deps:                                                 ## Update all dependencies
	@go get -u
	@go mod tidy
.PHONY: update_deps

build:                                                       ## Build the AutoSpotting binary
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags=$(LDFLAGS) -o $(BINARY)
.PHONY: build

archive: build                                               ## Create archive to be uploaded
	@rm -rf $(LOCAL_PATH)
	@mkdir -p $(LOCAL_PATH)
	@zip $(LOCAL_PATH)/lambda.zip $(BINARY) $(LICENSE_FILES)
	@cp -f cloudformation/stacks/AutoSpotting/template.yaml $(LOCAL_PATH)/
	@cp -f cloudformation/stacks/AutoSpotting/template.yaml $(LOCAL_PATH)/template_build_$(BUILD).yaml
	@zip -j $(LOCAL_PATH)/regional_stack_lambda.zip cloudformation/stacks/AutoSpotting/regional_stack_lambda.py
	@cp -f cloudformation/stacks/AutoSpotting/regional_template.yaml $(LOCAL_PATH)/
	@sed -e "s#lambda\.zip#lambda_build_$(BUILD).zip#" $(LOCAL_PATH)/template_build_$(BUILD).yaml > $(LOCAL_PATH)/template_build_$(BUILD).yaml.new
	@mv -- $(LOCAL_PATH)/template_build_$(BUILD).yaml.new $(LOCAL_PATH)/template_build_$(BUILD).yaml
	@cp -f $(LOCAL_PATH)/lambda.zip $(LOCAL_PATH)/lambda_build_$(BUILD).zip
	@cp -f $(LOCAL_PATH)/lambda.zip $(LOCAL_PATH)/lambda_build_$(SHA).zip
	@cp -f $(LOCAL_PATH)/regional_stack_lambda.zip $(LOCAL_PATH)/regional_stack_lambda_build_$(BUILD).zip
	@cp -f $(LOCAL_PATH)/regional_stack_lambda.zip $(LOCAL_PATH)/regional_stack_lambda_build_$(SHA).zip

.PHONY: archive

upload: archive                                              ## Upload binary
	aws s3 sync build/s3/ s3://$(BUCKET_NAME)/
.PHONY: upload

vet-check: build_deps                                        ## Verify vet compliance
ifeq ($(shell go vet -all . | wc -l | tr -d '[:space:]'), 0)
	@printf "ok\tall files passed go vet\n"
else
	@printf "error\tsome files did not pass go vet\n"
	@go vet -all . 2>&1
	@exit 1
endif

.PHONY: vet-check

fmt-check: build_deps                                        ## Verify fmt compliance
ifeq ($(shell gofmt -l -s . | wc -l | tr -d '[:space:]'), 0)
	@printf "ok\tall files passed go fmt\n"
else
	@printf "error\tsome files did not pass go fmt, fix the following formatting diff:\n"
	@gofmt -l -s -d .
	@exit 1
endif

.PHONY: fmt-check

module-check: build_deps                                     ## Verify that all changes to go.mod and go.sum are checked in, and fail otherwise
	@git diff-index --exit-code HEAD -- go.mod go.sum
.PHONY: module-check

test:                                                        ## Test go code and coverage
	@go test -covermode=count -coverprofile=$(COVER_PROFILE) ./...
.PHONY: test

lint: build_deps
	@golint -set_exit_status ./...
.PHONY: lint

full-test: fmt-check vet-check test lint                     ## Pass test / fmt / vet / lint
.PHONY: full-test

html-cover: test                                             ## Display coverage in HTML
	@go tool cover -html=$(COVER_PROFILE)
.PHONY: html-cover

ci-cover: html-cover                                         ## Test & generate coverage in the TravisCI format, fails unless executed from TravisCI
ifdef COVERALLS_TOKEN
	@goveralls -coverprofile=$(COVER_PROFILE) -service=travis-ci -repotoken=$(COVERALLS_TOKEN)
endif
.PHONY: ci-cover

ci-checks: fmt-check vet-check module-check lint             ## Pass fmt / vet & lint format
.PHONY: ci-checks

ci: archive ci-checks ci-cover                               ## Executes inside the CI Docker builder
.PHONY: ci

ci-docker:                                                   ## Executed by CI
	@docker-compose up --build --abort-on-container-exit --exit-code-from autospotting
.PHONY: ci-docker

help:                                                        ## Show this help
	@printf "Rules:\n"
	@fgrep -h "##" $(MAKEFILE_LIST) | fgrep -v fgrep | sed -e 's/\\$$//' | sed -e 's/##//'
.PHONY: help
