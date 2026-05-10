ARG ARCH=amd64
FROM golang:1.26 AS build

LABEL org.opencontainers.image.authors="Mei Akizuru <chimeaquas@hotmail.com>"

RUN mkdir -p /go/src/app
WORKDIR /go/src/app

# resolve dependency before copying whole source code
COPY go.mod go.sum ./
RUN go mod download

# copy other sources & build
COPY . /go/src/app
RUN GOOS=linux GOARCH=${ARCH} CGO_ENABLED=0 go build -o /go/bin/app

# hadolint ignore=DL3007 # distroless does not provide fixed tag.
FROM gcr.io/distroless/static-debian13:latest
COPY --from=build /go/bin/app /usr/local/bin/app
ENTRYPOINT ["/usr/local/bin/app"]
