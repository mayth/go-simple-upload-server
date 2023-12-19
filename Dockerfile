FROM --platform=$BUILDPLATFORM golang:1.21 AS build-env
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH

LABEL org.opencontainers.image.authors="Mei Akizuru <chimeaquas@hotmail.com>"

RUN mkdir -p /go/src/app
WORKDIR /go/src/app

# resolve dependency before copying whole source code
COPY go.mod .
COPY go.sum .
RUN go mod download

# copy other sources & build
COPY . /go/src/app
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -o /go/bin/app

FROM --platform=linux/${TARGETARCH} alpine:3.19 AS runtime-env
COPY --from=build-env /go/bin/app /usr/local/bin/app
ENTRYPOINT ["/usr/local/bin/app"]
