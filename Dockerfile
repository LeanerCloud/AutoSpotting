FROM golang:1.13-alpine as golang
RUN apk add -U --no-cache ca-certificates git make
COPY . /src
WORKDIR /src
RUN FLAVOR=nightly CGO_ENABLED=0 make

FROM scratch
COPY LICENSE BINARY_LICENSE THIRDPARTY /
COPY --from=golang /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=golang /src/AutoSpotting .
ENTRYPOINT ["./AutoSpotting"]
