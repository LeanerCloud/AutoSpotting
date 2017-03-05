BINDATA_FILE := generated_bindata.go

# upstream data
EC2_INSTANCES_INFO_COMMIT_SHA := bdd7a80cdb72f05fcbab4a0d5c6d8c96348f600f
INSTANCES_URL := "https://raw.githubusercontent.com/powdahound/ec2instances.info/$(EC2_INSTANCES_INFO_COMMIT_SHA)/www/instances.json"

DEPS := "wget git"

all: generate-bindata
.PHONY: all

check_deps:                                 ## Verify the system has all dependencies installed
	@for DEP in $(shell echo "$(DEPS)"); do \
		command -v "$$DEP" &>/dev/null \
		|| (echo "Error: dependency '$$DEP' is absent" ; exit 1); \
	done
	@echo "all dependencies satisifed: $(DEPS)"
.PHONY: check_deps

instances.json:
	wget --quiet -nv $(INSTANCES_URL) -O instances.json

generate-bindata: check_deps instances.json ## Convert instance data into go file
	@type go-bindata || go get -u github.com/jteeuwen/go-bindata/...
	go-bindata -o $(BINDATA_FILE) -nometadata -pkg ec2instancesinfo instances.json
	gofmt -l -s -w $(BINDATA_FILE)
.PHONY: prepare_bindata

clean:
	rm -f instances.json generated_bindata.go
