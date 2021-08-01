DEPS := "wget git go docker golint zip"

BINARY := AutoSpotting

COVER_PROFILE := /tmp/coverage.out
BUCKET_NAME ?= cloudprowess
FLAVOR ?= custom
LOCAL_PATH := build/s3/$(FLAVOR)
LICENSE_FILES := LICENSE THIRDPARTY
GOOS ?= linux
GOARCH ?= amd64

DOCKER_IMAGE ?= autospotting

MARKETPLACE_IMAGE ?= 709825985650.dkr.ecr.us-east-1.amazonaws.com/cloudutil/autospotting
MARKETPLACE_VERSION ?= 1.0

SHA := $(shell git rev-parse HEAD | cut -c 1-7)
BUILD := $(or $(TRAVIS_BUILD_NUMBER), $(TRAVIS_BUILD_NUMBER), $(SHA))
EXPIRATION := $(shell go run ./scripts/expiration_date.go)

ifneq ($(FLAVOR), custom)
    LICENSE_FILES += BINARY_LICENSE
endif

LDFLAGS="-X main.Version=$(FLAVOR)-$(BUILD) -X main.ExpirationDate=$(EXPIRATION) -s -w"

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

build_deps:                                                  ## Install all dependencies specified in tools.go
	@grep _ tools.go | cut -d '"' -f 2 | xargs go install
.PHONY: build_deps

update_deps:                                                 ## Update all dependencies
	@go get -u
	@go mod tidy
.PHONY: update_deps

build:                                                       ## Build the AutoSpotting binary
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags=$(LDFLAGS) -o $(BINARY)
.PHONY: build

marketplace-artifacts:                                       ## Create marketplace artifacts to be uploaded to S3
	@rm -rf $(LOCAL_PATH)
	@mkdir -p $(LOCAL_PATH)
	@cp -f cloudformation/stacks/AutoSpotting/template.yaml $(LOCAL_PATH)/
	@cp -f cloudformation/stacks/AutoSpotting/template.yaml $(LOCAL_PATH)/template_build_$(BUILD).yaml
	@cp -f cloudformation/stacks/AutoSpotting/regional_template.yaml $(LOCAL_PATH)/
	@sed -e "s#latest#$(MARKETPLACE_VERSION)#" $(LOCAL_PATH)/template_build_$(BUILD).yaml > $(LOCAL_PATH)/template_build_$(BUILD).yaml.new
	@mv -- $(LOCAL_PATH)/template_build_$(BUILD).yaml.new $(LOCAL_PATH)/template_build_$(BUILD).yaml

.PHONY: artifacts

docker:
	docker build -t $(DOCKER_IMAGE) .
.PHONY: docker

docker-marketplace:
	docker build -f Dockerfile.marketplace -t $(MARKETPLACE_IMAGE):$(MARKETPLACE_VERSION) .
.PHONY: docker-marketplace

docker-marketplace-push: docker-marketplace
	docker push $(MARKETPLACE_IMAGE):$(MARKETPLACE_VERSION)
.PHONY: docker-marketplace-push

upload: marketplace-artifacts                                ## Upload data to S3
	aws s3 sync build/s3/ s3://$(BUCKET_NAME)/
.PHONY: upload

vet-check:                                                   ## Verify vet compliance
	@go vet -all ./...
.PHONY: vet-check

fmt-check:                                                   ## Verify fmt compliance
	@sh -c 'test -z "$$(gofmt -l -s -d . | tee /dev/stderr)"'
.PHONY: fmt-check

module-check: build_deps                                     ## Verify that all changes to go.mod and go.sum are checked in, and fail otherwise
	@go mod tidy -v
	git diff --exit-code HEAD -- go.mod go.sum
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

ci: artifacts ci-checks ci-cover                               ## Executes inside the CI Docker builder
.PHONY: ci

ci-docker:                                                   ## Executed by CI
	@docker-compose up --build --abort-on-container-exit --exit-code-from autospotting
.PHONY: ci-docker

help:                                                        ## Show this help
	@printf "Rules:\n"
	@fgrep -h "##" $(MAKEFILE_LIST) | fgrep -v fgrep | sed -e 's/\\$$//' | sed -e 's/##//'
.PHONY: help
