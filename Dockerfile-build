FROM golang:1.12-alpine
RUN apk add -U --no-cache ca-certificates git make zip

COPY . /src
WORKDIR /src

# fetch deps and cache them in a Docker storage layer
RUN go get ./...
