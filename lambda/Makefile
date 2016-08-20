SHELL := /bin/bash
.PHONY: all build_install

all: build install

build:
	rm -f lambda.zip
	zip -j lambda *.py

install:
	mv lambda.zip ../build/s3/dv/

upload:
	aws s3 sync ../build/s3/ s3://cloudprowess/

## EOF
