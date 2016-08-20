all: upload

install:
		cp stacks/AutoSpotting/template.json ../build/s3/dv/

upload: install
		aws s3 sync ../build/s3/ s3://cloudprowess/

create:
		clouds update -c AutoSpotting

update:
		clouds update AutoSpotting

