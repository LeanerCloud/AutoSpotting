all: upload

install:
		cp stacks/AutoSpotting/template.yaml ../build/s3/nightly/

upload: install
		aws s3 sync ../build/s3/ s3://cloudprowess/

create:
		clouds update -c AutoSpotting

update:
		clouds update AutoSpotting

