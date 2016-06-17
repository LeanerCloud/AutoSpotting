BINARY=notifier

all: build

release: upload

build:
	go build $(GOFLAGS)

strip: build
	goupx --strip-binary --lzma --best ${BINARY}

install: strip
	cp ${BINARY} ../build/s3/agent/dv/

upload: install
	aws s3 sync ../build/s3/agent s3://cloudprowess/  
