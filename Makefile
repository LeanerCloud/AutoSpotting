
all: build_local

clean:
	./build_helper.sh clean

prepare_bindata:
	./build_helper.sh prepare_bindata

build_lambda_binary: prepare_bindata
	./build_helper.sh build_lambda_binary

prepare_upload_data: build_lambda_binary
	./build_helper.sh prepare_upload_data

# the following targets are only used for local development

build_local: prepare_bindata
	./build_helper.sh build_local

upload: prepare_upload_data
	./build_helper.sh upload

test: build_local
	./autospotting

cover:
	./build_helper.sh calculate_coverage