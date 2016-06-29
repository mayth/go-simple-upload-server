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


# Security

There is no Basic/Digest authentication. This app implements dead simple authentication: "security token".

All requests should have `token` parameter (it can be passed from query strings or a form parameter). If it is completely matched with the server's token, the access is allowed; otherwise, returns `401 Unauthorized`.

You can specify the server's token on startup by `-token` option. If you don't so, the server generates the token and writes it to STDOUT at WARN level log, like as:

```
$ ./simple_upload_server root
INFO[0000] starting up simple-upload-server
WARN[0000] token generated                               token=2dd30b90536d688e19f7
INFO[0000] start listening                               ip=0.0.0.0 port=25478 root=root token=2dd30b90536d688e19f7 upload_limit=5242880
```

NOTE: The token is generated from the random number, so it will change every time you start the server.

# Docker

```
$ docker run -p 25478:25478 -v $HOME/tmp:/var/root mayth/go-simple-upload-server app -token f9403fc5f537b4ab332d /var/root
```