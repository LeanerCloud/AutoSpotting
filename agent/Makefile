BINARY=agent
LAMBDA_BINARY=agent_lambda

all: build_local

release: upload

build_local:
	go build $(GOFLAGS)

build_lambda:
	GOOS=linux GOARCH=amd64 go build -o ${LAMBDA_BINARY}

strip: build_lambda
	goupx --strip-binary --lzma --best ${LAMBDA_BINARY}

install: strip
	mv ${LAMBDA_BINARY} ../build/s3/agent/dv/${BINARY}

upload: install
	aws s3 sync ../build/s3/agent s3://cloudprowess/  
