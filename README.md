# go-simple-upload-server
Simple HTTP server to save artifacts

# Usage

## Start Server

```
$ mkdir $HOME/tmp
$ ./simple_upload_server -token f9403fc5f537b4ab332d $HOME/tmp
```

(see "Security" section below for `-token` option)

## Uploading

You can upload files with `POST /upload`.
The filename is taken from the original file if available. If not, SHA1 hex digest will be used as the filename.

```
$ echo 'Hello, world!' > sample.txt
$ curl -Ffile=@sample.txt 'http://localhost:25478/upload?token=f9403fc5f537b4ab332d'
{"ok":true,"path":"/files/sample.txt"}
```

```
$ cat $HOME/tmp/sample.txt
hello, world!
```

**OR**

Use `PUT /files/(filename)`.
In this case, the original file name is ignored, and the name is taken from the URL.

```
$ curl -X PUT -Ffile=@sample.txt "http://localhost:25478/files/another_sample.txt?token=f9403fc5f537b4ab332d"
{"ok":true,"path":"/files/another_sample.txt"}
```

## Downloading

`GET /files/(filename)`.

```
$ curl 'http://localhost:25478/files/sample.txt?token=f9403fc5f537b4ab332d'
hello, world!
```

## Existence Check

`HEAD /files/(filename)`.

```
$ curl -I 'http://localhost:25478/files/foobar.txt?token=f9403fc5f537b4ab332d'
HTTP/1.1 200 OK
Accept-Ranges: bytes
Content-Length: 9
Content-Type: text/plain; charset=utf-8
Last-Modified: Sun, 09 Oct 2016 14:35:39 GMT
Date: Sun, 09 Oct 2016 14:35:43 GMT

$ curl 'http://localhost:25478/files/foobar.txt?token=f9403fc5f537b4ab332d'
hello!!!

$ curl -I 'http://localhost:25478/files/unknown?token=f9403fc5f537b4ab332d'
HTTP/1.1 404 Not Found
Content-Type: text/plain; charset=utf-8
X-Content-Type-Options: nosniff
Date: Sun, 09 Oct 2016 14:37:48 GMT
Content-Length: 19
```


## CORS Preflight Request

* `OPTIONS /files/(filename)`
* `OPTIONS /upload`

```
$ curl -I 'http://localhost:25478/files/foo'
HTTP/1.1 204 No Content
Access-Control-Allow-Methods: PUT,GET,HEAD
Access-Control-Allow-Origin: *
Date: Sun, 06 Sep 2020 09:45:20 GMT

$ curl -I -XOPTIONS 'http://localhost:25478/upload'
HTTP/1.1 204 No Content
Access-Control-Allow-Methods: POST
Access-Control-Allow-Origin: *
Date: Sun, 06 Sep 2020 09:45:32 GMT
```

notes:

* Requests using `*` as a path, like as `OPTIONS * HTTP/1.1`, are not supported.
* On sending `OPTIONS` request, `token` parameter is not required.
* For `/files/(filename)` request, server replies "204 No Content" even if the specified file does not exist.


# TLS

To enable TLS support, add `-cert` and `-key` options:

```
$ ./simple_upload_server -cert ./cert.pem -key ./key.pem root/
INFO[0000] starting up simple-upload-server
WARN[0000] token generated                               token=28d93c74c8589ab62b5e
INFO[0000] start listening TLS                           cert=./cert.pem key=./key.pem port=25443
INFO[0000] start listening                               ip=0.0.0.0 port=25478 root=root token=28d93c74c8589ab62b5e upload_limit=5242880
...
```

This server listens on `25443/tcp` for TLS connections by default. This can be changed by passing `-tlsport` option.

NOTE: The endpoint using HTTP is still active even if TLS is enabled.


# Security

## Token

There is no Basic/Digest authentication. This app implements dead simple authentication: "security token".

All requests should have `token` parameter (it can be passed as a query string or a form parameter). The server accepts the request only when the token is matched; otherwise, the server rejects the request and respond `401 Unauthorized`.

You can specify the server's token on startup by `-token` option. If you don't so, the server generates the token and writes it to STDOUT at WARN level log, like as:

```
$ ./simple_upload_server root
INFO[0000] starting up simple-upload-server
WARN[0000] token generated                               token=2dd30b90536d688e19f7
INFO[0000] start listening                               ip=0.0.0.0 port=25478 root=root token=2dd30b90536d688e19f7 upload_limit=5242880
```

NOTE: The token is generated from the random number, so it will change every time you start the server.

## CORS

If you enable CORS support using `-cors` option, the server append `Access-Control-Allow-Origin` header to the response. This feature is disabled by default.

# Docker

```
$ docker run -p 25478:25478 -v $HOME/tmp:/var/root mayth/simple-upload-server -token f9403fc5f537b4ab332d /var/root
```

# Docker-compose

To run go-simple-upload-server as part of docker-compose with traefik as reverse proxy, you can find inspiration in this partial snippet:
```
version: "3.6"

services:
  reverse-proxy:
    # The official v2 Traefik docker image
    image: traefik:v2.6.1
    container_name: traefik_public
    restart: unless-stopped
    command:
      - --api.dashboard=true
      - --providers.docker=true
      - --providers.docker.exposedbydefault=false
      - --entrypoints.web.address=:80
      - --entrypoints.websecure.address=:443
      - --certificatesresolvers.certresolver.acme.httpchallenge=true
      - --certificatesresolvers.certresolver.acme.httpchallenge.entrypoint=web
      - --certificatesresolvers.certresolver.acme.caserver=https://acme-v02.api.letsencrypt.org/directory
      - --certificatesresolvers.certresolver.acme.email=yourmail@example.sk
      - --certificatesresolvers.certresolver.acme.storage=/letsencrypt/acme.json
      - --providers.docker.network=proxy
      - --log.level=INFO
    networks:
      - proxy
    ports:
      # The HTTP port
      - "80:80"
      - "443:443"
    volumes:
      # So that Traefik can listen to the Docker events
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - /dockers/letsencrypt:/letsencrypt
    labels:
      - traefik.frontend.rule=Host(`traefik.example.sk`)"
      - traefik.port=8081"
  uploader:
    image: mayth/simple-upload-server
    command:
      - -token
      - c21beeafd4289e8ff916666666692333
      - -protected_method
      - POST,PUT
      - -upload_limit
        # 768 MB
      - "805306368"
      - -port
      - "80"
        # dir in docker to use for uploads/downloads
      - /var/data
    volumes:
      - /dockers/uploader:/var/data
    labels:
      - traefik.enable=true
      - traefik.http.middlewares.myredirect.redirectscheme.scheme=https
      - traefik.http.routers.uplhttp.middlewares=myredirect
      - traefik.http.routers.uplhttp.rule=Host(`upload.example.sk`,`www.upload.example.sk`)
      - traefik.http.routers.uplhttp.entrypoints=web
      - traefik.http.routers.uplsecure.rule=Host(`upload.example.sk`,`www.upload.example.sk`)
      - traefik.http.routers.uplsecure.entrypoints=websecure
      - traefik.http.routers.uplsecure.tls=true
      - traefik.http.routers.uplsecure.tls.certresolver=certresolver
      - traefik.http.services.uploader-service.loadbalancer.server.port=80
      - traefik.http.services.uploader-service.loadbalancer.server.scheme=http
    networks:
      - proxy
```
