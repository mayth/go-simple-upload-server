go-simple-upload-server
=======================

Simple HTTP server to save artifacts

- [Usage](#usage)
- [Authentication](#authentication)
- [TLS](#tls)
- [Testing](#testing)
- [API](#api)
  - [`POST /upload`](#post-upload)
  - [`PUT /files/:path`](#put-filespath)
  - [`GET /files/:path`](#get-filespath)
  - [`HEAD /files/:path`](#head-filespath)
  - [`OPTIONS /files/:path`](#options-filespath)
  - [`OPTIONS /upload`](#options-upload)


## Usage

```
  -addr string
        address to listen (default "127.0.0.1:8080")
  -config string
        path to config file
  -document_root string
        path to document root directory (default ".")
  -enable_auth
        enable authentication
  -enable_cors
        enable CORS header (default true)
  -file_naming_strategy string
        File naming strategy (default "uuid")
  -max_upload_size int
        max upload size in bytes (default 1048576)
  -read_only_tokens value
        comma separated list of read only tokens
  -read_write_tokens value
        comma separated list of read write tokens
  -shutdown_timeout int
        graceful shutdown timeout in milliseconds (default 15000)
```

Configurations via the arguments take precedence over those came from the config file.

## Authentication

This server does not require authentication by default. Anyone who can access the server can get/upload files.

The server implements a simple authentication mechanism using tokens.

1. Configure the server to enable authentication: `"enable_auth": true` or `-enable_auth=true`.
2. Prepare tokens. Any string value is valid as a token.
3. Add them to the configuration. There are two keys in configuration: `read_only_tokens` and `read_write_tokens`.
   If authentication is enabled but no tokens provided, the server generates a read-only token and a read-write token on its starting up.
4. Request with the token. Add Authorization header with value `Bearer <TOKEN>` or `token=<TOKEN>` to the query parameter. Authorization header takes precedence.

| Token Type |             Allowed Operations             |
| ---------- | ------------------------------------------ |
| read-only  | `GET`, `HEAD`                              |
| read-write | `POST`, `PUT` in addition to read-only ops |

Note that `OPTIONS` is always allowed without authentication.

Authentication is failed when:

* A request has no tokens.
* A request has a token but not registered to the server.
* A request has a token but not allowed to the requested operation.

In these cases, the server respond with `401 Unauthorized` with body like as: `{"ok": false, "error": "unauthorized"}`.

No one can request write operations if you configures the server with read-only tokens only.
As a result, the server operates like read-only mode.

## TLS

v1 has TLS support but I decided to omit it from v2.

Please consider using a reverse proxy like nginx.

## Testing

To run all tests, just run `go test` as usual:

```
$ go test ./...
```

This includes end-to-end tests. By default, the server with on-memory FileSystem is created and it starts listening on
the port chosen randomly. You can control this behavior by setting the environment variables.

If `TEST_WITH_REAL_FS=${PATH_TO_DOCUMENT_ROOT}` is set, the test server uses the real filesystem. Make sure the document
root directory contains no files; otherwise, some tests might be failed. The directory will not be cleaned after testing.

If `TEST_TARGET_ADDR-${HOST}:${PORT}` is set, the test program doesn't start a local test server and sends requests to
`http://${HOST}:${PORT}`. Note that the target server's document root should be cleared prior to testing.

This repository has `docker-compose.e2e.yml` to run the E2E test. To run tests using this:

```
$ docker compose -f docker-compose.e2e.yml run --rm test
$ docker compose -f docker-compose.e2e.yml down --rmi local --volumes
```

## API

### `POST /upload`

Uploads a new file. The name of the local (= server-side) file is taken from the uploading file.

#### Request

Content-Type
: `multipart/form-data`

Parameters:

|    Name     | Required? |   Type    |                         Description                          | Default |
| ----------- | :-------: | --------- | ------------------------------------------------------------ | ------- |
| `file`      |     x     | Form Data | A content of the file.                                       |         |
| `overwrite` |           | `boolean` | Allow overwriting the existing file on the server if `true`. | `false` |

#### Response

##### On Successful

Status Code
: `201 Created`

Content-Type
: `application/json`

Body:

|  Name  |   Type    |               Description               |
| ------ | --------- | --------------------------------------- |
| `ok`   | `boolean` | `true` if successful.                   |
| `path` | `string`  | A path to access this file in this API. |

##### On Failure

|   StatusCode   |                                              When                                              |
| -------------- | ---------------------------------------------------------------------------------------------- |
| `409 Conflict` | There is the file whose name is the same as the uploading file and overwriting is not allowed. |

#### Example

```
$ echo 'Hello, world!' > sample.txt
$ curl -Ffile=@sample.txt http://localhost:25478/upload
{"ok":true,"path":"/files/sample.txt"}
```

```
$ cat $DOCROOT/sample.txt
Hello, world!
```

### `PUT /files/:path`

Uploads a file. The original file name is ignored and the name is taken from the path in the request URL.

#### Parameters

|    Name     | Required? |   Type    |                    Description                     | Default |
| ----------- | :-------: | --------- | -------------------------------------------------- | ------- |
| `:path`     |     x     | `string`  | Path to the file.                                  |         |
| `file`      |     x     | Form Data | A content of the file.                             |         |
| `overwrite` |           | `boolean` | Allow overwriting the existing file on the server. | `false` |

#### Response

##### On Successful

Status Code
: `201 Created`

Content-Type
: `application/json`

Body:

|  Name  |   Type    |               Description               |
| ------ | --------- | --------------------------------------- |
| `ok`   | `boolean` | `true` if successful.                   |
| `path` | `string`  | A path to access this file in this API. |

##### On Failure

|   StatusCode   |                                              When                                              |
| -------------- | ---------------------------------------------------------------------------------------------- |
| `409 Conflict` | There is the file whose name is the same as the uploading file and overwriting is not allowed. |

#### Example

```
$ curl -XPUT -Ffile=@sample.txt "http://localhost:25478/files/foobar.txt"
{"ok":true,"path":"/files/foobar.txt"}

$ cat $DOCROOT/foobar.txt
Hello, world!
```

### `GET /files/:path`

Downloads a file.

#### Request

Parameters:

|  Name  | Required? |   Type   |     Description     | Default |
| ------ | :-------: | -------- | ------------------- | ------- |
| `path` |     x     | `string` | A path to the file. |         |

#### Response

##### On Successful

Status Code
: `200 OK`

Content-Type
: Depends on the content.

Body
: The content of the request file.

##### On Failure

Content-Type
: `application/json`

|   StatusCode    |          When          |
| --------------- | ---------------------- |
| `404 Not Found` | There is no such file. |

#### Example

```
$ curl http://localhost:25478/files/sample.txt
Hello, world!
```

### `HEAD /files/:path`

Check existence of a file.

#### Request

Parameters:

|  Name  | Required? |   Type   |     Description     | Default |
| ------ | :-------: | -------- | ------------------- | ------- |
| `path` |     x     | `string` | A path to the file. |         |

#### Response

##### On Successful

Status Code
: `200 OK`

Body
: Not Available

##### On Failure

|   StatusCode   |                                              When                                              |
| -------------- | ---------------------------------------------------------------------------------------------- |
| `404 Not Found` | No such file on the server. |

#### Example

```
$ curl -I http://localhost:25478/files/foobar.txt
```

### `OPTIONS /files/:path`
### `OPTIONS /upload`

CORS preflight request.

#### Request

Parameters:

|  Name  | Required? |   Type   |     Description     | Default |
| ------ | :-------: | -------- | ------------------- | ------- |
| `path` |     x     | `string` | A path to the file. |         |

#### Response

##### On Successful

Status Code
: `204 No Content`

##### On Failure

#### Example

TODO

#### Notes

* Requests using `*` as a path, like as `OPTIONS * HTTP/1.1`, are not supported.
* On sending `OPTIONS` request, `token` parameter is not required.
* For `/files/:path` request, server replies "204 No Content" even if the specified file does not exist.
