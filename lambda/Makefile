SHELL := /bin/bash
.PHONY: all build_install

all: build install

build:
	rm -f lambda.zip
	zip -j lambda *.js

install:
	mv lambda.zip ../build/s3/agent/dv/

upload:
	aws s3 sync ../build/s3/agent s3://cloudprowess/

## EOF
