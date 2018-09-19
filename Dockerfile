FROM golang:1.11-alpine as golang
RUN apk add -U --no-cache ca-certificates git make
RUN go get -v github.com/cristim/autospotting/...
WORKDIR /go/src/github.com/cristim/autospotting/
RUN FLAVOR=nightly CGO_ENABLED=0 make

FROM scratch
WORKDIR /
COPY LICENSE BINARY_LICENSE /
COPY --from=golang /go/src/github.com/cristim/autospotting/autospotting .
COPY --from=golang /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT ["./autospotting"]
