FROM golang:1.9 AS build-env

MAINTAINER Mei Akizuru

RUN mkdir -p /go/src/app
COPY . /go/src/app

WORKDIR /go/src/app

# download the dependencies and build the application
RUN go-wrapper download
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go-wrapper install

FROM alpine:3.5 AS runtime-env

COPY --from=build-env /go/bin/app /usr/local/bin/app
