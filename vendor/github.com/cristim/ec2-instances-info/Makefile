BINDATA_FILE := data/generated_bindata.go

# upstream data
INSTANCES_URL := "https://ec2instances.info/instances.json"

DEPS := "wget git jq"

all: generate-bindata run-example
.PHONY: all

check_deps:                                 ## Verify the system has all dependencies installed
	@for DEP in $(shell echo "$(DEPS)"); do \
		command -v "$$DEP" > /dev/null 2>&1 \
		|| (echo "Error: dependency '$$DEP' is absent" ; exit 1); \
	done
	@echo "all dependencies satisifed: $(DEPS)"
.PHONY: check_deps

data/instances.json:
	@mkdir -p data
	@wget --quiet -nv $(INSTANCES_URL) -O data/instances.json

generate-bindata: check_deps data/instances.json ## Convert instance data into go file
	@type go-bindata || go get -u github.com/jteeuwen/go-bindata/...
	@go-bindata -o $(BINDATA_FILE) -nometadata -pkg data data/instances.json
	@gofmt -l -s -w $(BINDATA_FILE) >/dev/null
.PHONY: prepare_bindata

run-example:
	@go get ./...
	@go run examples/instances/instances.go

clean:
	@rm -rf data
.PHONY: clean

update-data: clean all
.PHONY: update-data

test:
	@go test
.PHONY: test
