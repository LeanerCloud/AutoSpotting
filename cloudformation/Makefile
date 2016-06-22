all: upload

install:
		cp stacks/autospotting-lambda/template.json ../build/s3/dv/

upload: install
		aws s3 sync ../build/s3/ s3://cloudprowess/

create:
		clouds update -c autospotting-lambda

update:
		clouds update autospotting-lambda

