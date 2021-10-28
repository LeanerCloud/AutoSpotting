FROM golang:alpine as golang
RUN apk add -U --no-cache ca-certificates git make

COPY go.mod go.sum /src/
# Download dependencies
WORKDIR /src
RUN GOPROXY=direct go mod download

COPY . /src
RUN FLAVOR=nightly CGO_ENABLED=0 GOPROXY=direct make

FROM scratch
COPY LICENSE BINARY_LICENSE THIRDPARTY /
COPY --from=golang /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=golang /src/AutoSpotting .
ENTRYPOINT ["./AutoSpotting"]
