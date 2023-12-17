go-simple-upload-server
=======================

Simple HTTP server to save artifacts

- [Usage](#usage)
- [Authentication](#authentication)
- [TLS](#tls)
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
  -enable_cors
        enable CORS header (default true)
  -file_naming_strategy string
        File naming strategy (default "uuid")
  -max_upload_size int
        max upload size in bytes (default 1048576)
  -shutdown_timeout int
        graceful shutdown timeout in milliseconds (default 15000)
```

Configurations via the arguments take precedence over those came from the config file.

## Authentication

No authentication mechanisms are implemented yet.
This server accepts all requests from any clients.

## TLS

TLS support is not implemented yet.

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

Body
: |  Name  |   Type    |               Description               |
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

Body
: |  Name  |   Type    |               Description               |
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
