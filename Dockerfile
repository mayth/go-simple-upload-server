FROM golang:1.13 AS build-env

MAINTAINER Mei Akizuru

RUN mkdir -p /go/src/app
COPY . /go/src/app

WORKDIR /go/src/app

RUN go get -v -d
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go install -v

FROM alpine:3.11 AS runtime-env

COPY --from=build-env /go/bin/app /usr/local/bin/app
