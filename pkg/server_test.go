package simpleuploadserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/afero"
)

func TestGetHandler(t *testing.T) {
	type args struct {
		Method string
		Url    string
	}
	tests := []struct {
		name string
		args args
		want int
		body string
	}{
		{
			name: "get existing file",
			args: args{
				Method: http.MethodGet,
				Url:    "/files/foo/bar.txt",
			},
			want: http.StatusOK,
			body: "hello, world",
		},
		{
			name: "get non-existing file",
			args: args{
				Method: http.MethodGet,
				Url:    "/files/bar/baz",
			},
			want: http.StatusNotFound,
			body: `{"ok":false,"error":"file not found"}`,
		},
		{
			name: "get without endpoint",
			args: args{
				Method: http.MethodGet,
				Url:    "/abc",
			},
			want: http.StatusNotFound,
			body: `{"ok":false,"error":"file not found"}`,
		},
		{
			name: "get directory",
			args: args{
				Method: http.MethodGet,
				Url:    "/files/foo",
			},
			want: http.StatusNotFound,
			body: `{"ok":false,"error":"foo is a directory"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			docRoot := "/opt/app"
			fs := afero.NewMemMapFs()
			fs.MkdirAll(path.Join(docRoot, "foo"), 0755)
			afero.WriteFile(fs, path.Join(docRoot, "foo", "bar.txt"), []byte("hello, world"), 0644)
			config := ServerConfig{
				DocumentRoot: "/opt/app",
				EnableCORS:   true,
			}
			server := Server{config, afero.NewBasePathFs(fs, docRoot)}
			req, err := http.NewRequest(tt.args.Method, tt.args.Url, nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(server.handle(server.handleGet))
			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.want {
				t.Errorf("status = %d, want = %d", status, tt.want)
			}
			if body := rr.Body.String(); body != tt.body {
				t.Errorf("body = \"%s\", want = \"%s\"", body, tt.body)
			}
		})
	}
}

func TestServer_PostHandler(t *testing.T) {
	docRoot := "/opt/app"
	type args struct {
		Method  string
		Url     string
		Content []byte
		Name    string
	}
	tests := []struct {
		name string
		args args
		want int
		body string
	}{
		{
			name: "Post hello.txt",
			args: args{
				Method:  http.MethodPost,
				Url:     "/upload",
				Content: []byte("hello, world"),
				Name:    "hello.txt",
			},
			want: http.StatusCreated,
			body: `{"ok":true,"path":"/files/hello.txt"}`,
		},
		{
			name: "Post nothing",
			args: args{
				Method:  http.MethodPost,
				Url:     "/upload",
				Content: []byte{},
				Name:    "empty",
			},
			want: http.StatusCreated,
			body: `{"ok":true,"path":"/files/empty"}`,
		},
		{
			name: "Post the existing file should be rejected",
			args: args{
				Method:  http.MethodPost,
				Url:     "/upload",
				Content: []byte("overwritten!"),
				Name:    "ow.txt",
			},
			want: http.StatusConflict,
			body: `{"ok":false,"error":"the file already exists"}`,
		},
		{
			name: "Post the existing file with overwrite option should be accepted",
			args: args{
				Method:  http.MethodPost,
				Url:     "/upload?overwrite=true",
				Content: []byte("overwritten!"),
				Name:    "ow.txt",
			},
			want: http.StatusCreated,
			body: `{"ok":true,"path":"/files/ow.txt"}`,
		},
		{
			name: "POST large file should fail",
			args: args{
				Method:  http.MethodPost,
				Url:     "/upload",
				Content: []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ"),
				Name:    "toolarge",
			},
			want: http.StatusRequestEntityTooLarge,
			body: `{"ok":false,"error":"file size limit exceeded"}`,
		},
		// TODO: add text without name
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			fs.MkdirAll(docRoot, 0755)
			afero.WriteFile(fs, path.Join(docRoot, "ow.txt"), []byte("overwrite?"), 0644)
			config := ServerConfig{
				DocumentRoot:  docRoot,
				EnableCORS:    true,
				MaxUploadSize: 16,
			}
			server := Server{config, afero.NewBasePathFs(fs, docRoot)}

			b := new(bytes.Buffer)
			w := multipart.NewWriter(b)
			fw, err := w.CreateFormFile("file", tt.args.Name)
			if err != nil {
				t.Fatal(err)
			}
			written, err := fw.Write(tt.args.Content)
			if err != nil {
				t.Fatal(err)
			}
			if written == 0 && len(tt.args.Content) > 0 {
				t.Fatalf("content has %d bytes but no bytes written", len(tt.args.Content))
			}
			w.Close()

			req, err := http.NewRequest(tt.args.Method, tt.args.Url, b)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", w.FormDataContentType())

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(server.handle(server.handlePost))
			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.want {
				t.Errorf("status = %d, want = %d", status, tt.want)
				t.Logf("%+v", req)
			}
			if body := rr.Body.String(); body != tt.body {
				t.Errorf("body = \"%s\", want = \"%s\"", body, tt.body)
			}
			if rr.Code == http.StatusCreated {
				var resp struct {
					OK   bool   `json:"ok"`
					Path string `json:"path"`
				}
				if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
					t.Errorf("unexpected response body: %v", err)
				}
				localPath := strings.TrimPrefix(resp.Path, "/files/")
				localFullPath := filepath.Join(docRoot, localPath)
				f, err := fs.Open(localFullPath)
				if err != nil {
					t.Errorf("failed to verify local file. cannot open %s: %v", localFullPath, err)
					return
				}
				defer f.Close()
				uploaded, err := afero.ReadAll(f)
				if err != nil {
					t.Errorf("failed to verify local file. cannot read %s: %v", localFullPath, err)
					return
				}
				if !reflect.DeepEqual(uploaded, tt.args.Content) {
					t.Errorf("failed to verify. request body = %v, local file = %v", tt.args.Content, uploaded)
				}
			}
		})
	}
}

func TestServer_PutHandler(t *testing.T) {
	docRoot := "/opt/app"
	type args struct {
		Method  string
		Url     string
		Content []byte
		Name    string
	}
	tests := []struct {
		name string
		args args
		want int
		body string
	}{
		{
			name: "PUT /files/hello.txt with text",
			args: args{
				Method:  http.MethodPut,
				Url:     "/files/hello.txt",
				Content: []byte("hello, world"),
				Name:    "hello.txt",
			},
			want: http.StatusCreated,
			body: `{"ok":true,"path":"/files/hello.txt"}`,
		},
		{
			name: "PUT /files/empty with an empty content",
			args: args{
				Method:  http.MethodPut,
				Url:     "/files/empty",
				Content: []byte{},
				Name:    "empty",
			},
			want: http.StatusCreated,
			body: `{"ok":true,"path":"/files/empty"}`,
		},
		{
			name: "PUT /files/hello/world.txt will create directory and file",
			args: args{
				Method:  http.MethodPut,
				Url:     "/files/hello/world.txt",
				Content: []byte("hello, world"),
				Name:    "world.txt",
			},
			want: http.StatusCreated,
			body: `{"ok":true,"path":"/files/hello/world.txt"}`,
		},
		{
			name: "PUT /files/ should fail",
			args: args{
				Method:  http.MethodPut,
				Url:     "/files/",
				Content: []byte("hello"),
				Name:    "hello",
			},
			want: http.StatusMethodNotAllowed,
			body: `{"ok":false,"error":"PUT is accepted on /files/:name"}`,
		},
		{
			name: "PUT large file should fail",
			args: args{
				Method:  http.MethodPut,
				Url:     "/files/toolarge",
				Content: []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ"),
				Name:    "toolarge",
			},
			want: http.StatusRequestEntityTooLarge,
			body: `{"ok":false,"error":"file size limit exceeded"}`,
		},
		// TODO: add text without name
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			fs.MkdirAll(docRoot, 0755)
			config := ServerConfig{
				DocumentRoot:  docRoot,
				EnableCORS:    true,
				MaxUploadSize: 16,
			}
			server := Server{config, afero.NewBasePathFs(fs, docRoot)}

			b := new(bytes.Buffer)
			w := multipart.NewWriter(b)
			fw, err := w.CreateFormFile("file", tt.args.Name)
			if err != nil {
				t.Fatal(err)
			}
			written, err := fw.Write(tt.args.Content)
			if err != nil {
				t.Fatal(err)
			}
			if written == 0 && len(tt.args.Content) > 0 {
				t.Fatalf("content has %d bytes but no bytes written", len(tt.args.Content))
			}
			w.Close()

			req, err := http.NewRequest(tt.args.Method, tt.args.Url, b)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", w.FormDataContentType())

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(server.handle(server.handlePut))
			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.want {
				t.Errorf("status = %d, want = %d", status, tt.want)
			}
			if body := rr.Body.String(); body != tt.body {
				t.Errorf("body = \"%s\", want = \"%s\"", body, tt.body)
			}
			if rr.Code == http.StatusCreated || rr.Code == http.StatusOK {
				var resp struct {
					OK   bool   `json:"ok"`
					Path string `json:"path"`
				}
				if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
					t.Errorf("unexpected response body: %v", err)
				}
				localPath := strings.TrimPrefix(resp.Path, "/files/")
				localFullPath := filepath.Join(docRoot, localPath)
				t.Logf("opening %s to verify", localFullPath)
				f, err := fs.Open(localFullPath)
				if err != nil {
					t.Errorf("failed to verify local file. cannot open %s: %v", localFullPath, err)
					return
				}
				defer f.Close()
				uploaded, err := afero.ReadAll(f)
				if err != nil {
					t.Errorf("failed to verify local file. cannot read %s: %v", localFullPath, err)
					return
				}
				if !reflect.DeepEqual(uploaded, tt.args.Content) {
					t.Errorf("failed to verify. request body = %v, local file = %v", tt.args.Content, uploaded)
				}
			}
		})
	}
}

func Test_getFileSize(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
		want    int64
		wantErr bool
	}{
		{
			name:    "hello, world",
			content: []byte("hello, world"),
			want:    12,
			wantErr: false,
		},
		{
			name:    "empty bytes",
			content: []byte(""),
			want:    0,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bytes.NewReader(tt.content)
			got, err := getFileSize(r)
			if (err != nil) != tt.wantErr {
				t.Errorf("getFileSize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getFileSize() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseBoolishValue(t *testing.T) {
	tests := []struct {
		arg  string
		want bool
	}{
		{"yes", true},
		{"true", true},
		{"1", true},
		{"True", true},
		{"", false},
		{"no", false},
		{"foo", false},
	}
	for _, tt := range tests {
		t.Run(tt.arg, func(t *testing.T) {
			if got := parseBoolishValue(tt.arg); got != tt.want {
				t.Errorf("parseBoolishValue(%s) = %v, want %v", tt.arg, got, tt.want)
			}
		})
	}
}

// Tests using Server instead of testing Handlers directly.
func TestServer(t *testing.T) {
	docRoot := "/opt/app"

	fs := afero.NewMemMapFs()
	fs.MkdirAll(docRoot, 0755)
	afero.WriteFile(fs, path.Join(docRoot, "test.txt"), []byte("lorem ipsum"), 0644)
	afero.WriteFile(fs, path.Join(docRoot, "foo", "bar.txt"), []byte("hello, world"), 0644)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	port, err := getAvailablePort()
	if err != nil {
		t.Fatalf("unable to find an available port: %v", err)
	}

	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	config := ServerConfig{
		Addr:            addr,
		DocumentRoot:    docRoot,
		EnableCORS:      true,
		MaxUploadSize:   16,
		ShutdownTimeout: 5000,
	}
	server := Server{config, afero.NewBasePathFs(fs, docRoot)}
	go func() {
		t.Logf("starting server at %s", addr)
		server.Start(ctx)
	}()

	base, err := url.Parse("http://" + addr)
	if err != nil {
		t.Fatalf("failed to parse base url: %v", err)
	}

	verifyLocalFile := func(t *testing.T, path string, content []byte) {
		got, err := afero.ReadFile(fs, path)
		if err != nil {
			t.Fatalf("failed to read local file: %v", err)
		}
		if !reflect.DeepEqual(got, content) {
			t.Errorf("local file content = %s, want = %s", got, content)
		}
	}

	withPreservingOriginal := func(t *testing.T, path string, f func()) {
		original, err := afero.ReadFile(fs, path)
		if err != nil {
			t.Fatalf("failed to read local file: %v", err)
		}
		f()
		if err := afero.WriteFile(fs, path, original, 0644); err != nil {
			t.Fatalf("failed to restore original content: %v", err)
		}
	}

	t.Run("POST /upload", func(t *testing.T) {
		u := base.JoinPath("/upload")
		req, err := makeFormRequest(u, http.MethodPost, "hello.txt", bytes.NewBufferString("hello, world"))
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to POST: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusCreated)
		}
		if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %s, want = \"application/json\"", ct)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}
		var result SuccessfullyUploadedResult
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		expected := SuccessfullyUploadedResult{true, "/files/hello.txt"}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("result = %+v, want = %+v", result, expected)
		}
		verifyLocalFile(t, filepath.Join(docRoot, "hello.txt"), []byte("hello, world"))
	})

	t.Run("POST /files/foo.txt should fail due to an invalid method", func(t *testing.T) {
		u := base.JoinPath("/files/foo.txt")
		req, err := makeFormRequest(u, http.MethodPost, "foo.txt", bytes.NewBufferString("hello, world"))
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to POST: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusMethodNotAllowed)
		}
		if resp.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %s, want = \"application/json\"", resp.Header.Get("Content-Type"))
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}
		var result ErrorResult
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		expected := ErrorResult{false, "POST is not allowed on /files"}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("result = %+v, want = %+v", result, expected)
		}
	})

	t.Run("POST /upload should fail due to duplication", func(t *testing.T) {
		localPath := filepath.Join(docRoot, "test.txt")
		localOriginal, err := afero.ReadFile(fs, localPath)
		if err != nil {
			t.Fatalf("failed to read local file: %v", err)
		}

		u := base.JoinPath("/upload")
		req, err := makeFormRequest(u, http.MethodPost, "test.txt", bytes.NewBufferString("hello, new world"))
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to POST: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusConflict {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusConflict)
		}
		if resp.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %s, want = \"application/json\"", resp.Header.Get("Content-Type"))
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}
		var result ErrorResult
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		expected := ErrorResult{false, "the file already exists"}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("result = %+v, want = %+v", result, expected)
		}
		verifyLocalFile(t, localPath, localOriginal)
	})

	t.Run("POST /upload should succeed with overwrite option", func(t *testing.T) {
		localPath := filepath.Join(docRoot, "test.txt")
		withPreservingOriginal(t, localPath, func() {
			u := base.JoinPath("/upload")
			q := u.Query()
			q.Set("overwrite", "true")
			u.RawQuery = q.Encode()

			newContent := "hello, new world"
			b := bytes.NewBufferString(newContent)
			req, err := makeFormRequest(u, http.MethodPost, "test.txt", b)
			if err != nil {
				t.Fatal(err)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("failed to POST: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusCreated {
				t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusCreated)
			}
			if resp.Header.Get("Content-Type") != "application/json" {
				t.Errorf("Content-Type = %s, want = \"application/json\"", resp.Header.Get("Content-Type"))
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("failed to read response body: %v", err)
			}
			var result SuccessfullyUploadedResult
			if err := json.Unmarshal(body, &result); err != nil {
				t.Fatalf("failed to decode response body: %v", err)
			}
			expected := SuccessfullyUploadedResult{true, "/files/test.txt"}
			if !reflect.DeepEqual(result, expected) {
				t.Errorf("result = %+v, want = %+v", result, expected)
			}
			verifyLocalFile(t, localPath, []byte(newContent))
		})
	})

	t.Run("PUT /files/hello_put.txt", func(t *testing.T) {
		u := base.JoinPath("/files/hello_put.txt")
		content := bytes.NewBufferString("hello, world")
		req, err := makeFormRequest(u, http.MethodPut, "hello.txt", content)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to PUT: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusCreated)
		}
		if resp.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %s, want = \"application/json\"", resp.Header.Get("Content-Type"))
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}
		var result SuccessfullyUploadedResult
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		expected := SuccessfullyUploadedResult{true, "/files/hello_put.txt"}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("result = %+v, want = %+v", result, expected)
		}
		verifyLocalFile(t, filepath.Join(docRoot, "hello_put.txt"), []byte("hello, world"))
	})

	t.Run("PUT /files/hello/world.txt", func(t *testing.T) {
		u := base.JoinPath("/files/hello/world.txt")
		req, err := makeFormRequest(u, http.MethodPut, "world.txt", bytes.NewBufferString("hello, world"))
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to PUT: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusCreated)
		}
		if resp.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %s, want = \"application/json\"", resp.Header.Get("Content-Type"))
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}
		var result SuccessfullyUploadedResult
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		if !result.OK {
			t.Errorf("result.OK = false, want = true")
		}
		if result.Path != "/files/hello/world.txt" {
			t.Errorf("path = %s, want = \"/files/hello/world.txt\"", result.Path)
		}
		if exists, err := afero.DirExists(fs, filepath.Join(docRoot, "hello")); err != nil {
			t.Fatalf("failed to check if directory exists: %v", err)
		} else if !exists {
			t.Errorf("directory /hello does not exist")
		}
		verifyLocalFile(t, filepath.Join(docRoot, "hello", "world.txt"), []byte("hello, world"))
	})

	t.Run("PUT /upload should fail", func(t *testing.T) {
		u := base.JoinPath("/upload")
		req, err := makeFormRequest(u, http.MethodPut, "foo.txt", bytes.NewBufferString("hello, world"))
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to PUT: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusMethodNotAllowed)
		}
		if resp.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %s, want = \"application/json\"", resp.Header.Get("Content-Type"))
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}
		var result ErrorResult
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		expected := ErrorResult{false, "PUT is not allowed on /upload"}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("result = %+v, want = %+v", result, expected)
		}
	})

	t.Run("PUT /files/foo/bar.txt should fail (conflict)", func(t *testing.T) {
		u := base.JoinPath("/files/foo/bar.txt")
		req, err := makeFormRequest(u, http.MethodPut, "foo.txt", bytes.NewBufferString("new world"))
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to PUT: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusConflict {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusConflict)
		}
		if resp.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %s, want = \"application/json\"", resp.Header.Get("Content-Type"))
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}
		var result ErrorResult
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		expected := ErrorResult{false, "the file already exists"}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("result = %+v, want = %+v", result, expected)
		}
	})

	t.Run("PUT /files/foo/bar.txt should succeed (overwrite)", func(t *testing.T) {
		localPath := filepath.Join(docRoot, "foo", "bar.txt")
		withPreservingOriginal(t, localPath, func() {
			u := base.JoinPath("/files/foo/bar.txt")
			q := u.Query()
			q.Set("overwrite", "true")
			u.RawQuery = q.Encode()

			req, err := makeFormRequest(u, http.MethodPut, "foo.txt", bytes.NewBufferString("new world"))
			if err != nil {
				t.Fatal(err)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("failed to PUT: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusCreated {
				t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusCreated)
			}
			if resp.Header.Get("Content-Type") != "application/json" {
				t.Errorf("Content-Type = %s, want = \"application/json\"", resp.Header.Get("Content-Type"))
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("failed to read response body: %v", err)
			}
			var result SuccessfullyUploadedResult
			if err := json.Unmarshal(body, &result); err != nil {
				t.Fatalf("failed to decode response body: %v", err)
			}
			expected := SuccessfullyUploadedResult{true, "/files/foo/bar.txt"}
			if !reflect.DeepEqual(result, expected) {
				t.Errorf("result = %+v, want = %+v", result, expected)
			}
			verifyLocalFile(t, filepath.Join(docRoot, "foo", "bar.txt"), []byte("new world"))
		})
	})

	t.Run("GET /files/foo/bar.txt", func(t *testing.T) {
		u := base.JoinPath("/files/foo/bar.txt")
		resp, err := http.Get(u.String())
		if err != nil {
			t.Fatalf("failed to GET: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusOK)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}
		if string(body) != "hello, world" {
			t.Errorf("body = %s, want = \"hello, world\"", body)
		}
	})

	t.Run("GET /files/foo/bar/baz.txt should fail (not found)", func(t *testing.T) {
		u := base.JoinPath("/files/foo/bar/baz.txt")
		resp, err := http.Get(u.String())
		if err != nil {
			t.Fatalf("failed to GET: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusNotFound)
		}
		if resp.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %s, want = \"application/json\"", resp.Header.Get("Content-Type"))
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}
		var result ErrorResult
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		expected := ErrorResult{false, "file not found"}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("result = %+v, want = %+v", result, expected)
		}
	})

	t.Run("HEAD /files/foo/bar.txt", func(t *testing.T) {
		localPath := filepath.Join(docRoot, "foo", "bar.txt")

		u := base.JoinPath("/files/foo/bar.txt")
		resp, err := http.Head(u.String())
		if err != nil {
			t.Fatalf("failed to HEAD: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusOK)
		}
		info, err := fs.Stat(localPath)
		if err != nil {
			t.Fatalf("failed to stat local file: %v", err)
		}
		cl, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
		if err != nil {
			t.Fatalf("failed to parse Content-Length: %v", err)
		}
		if cl != info.Size() {
			t.Errorf("Content-Length = %d, want = %d", cl, info.Size())
		}
	})

	t.Run("HEAD /files/foo/bar/baz.txt", func(t *testing.T) {
		u := base.JoinPath("/files/foo/bar/baz.txt")
		resp, err := http.Head(u.String())
		if err != nil {
			t.Fatalf("failed to HEAD: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusNotFound)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}
		if len(body) > 0 {
			t.Errorf("body = %s, want empty", body)
		}
	})

	t.Run("OPTIONS /files/foo/bar.txt", func(t *testing.T) {
		u := base.JoinPath("/files/foo/bar.txt")
		req, err := http.NewRequest(http.MethodOptions, u.String(), nil)
		if err != nil {
			t.Fatalf("failed to create OPTIONS request: %v", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to request: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusNoContent)
		}
		acam := resp.Header.Get("Access-Control-Allow-Methods")
		if acam == "" {
			t.Errorf("Access-Control-Allow-Methods got empty, want not empty")
		}
		allowedMethods := strings.Split(acam, ",")
		for i := range allowedMethods {
			allowedMethods[i] = strings.TrimSpace(allowedMethods[i])
		}
		expectedAllowedMethods := []string{"GET", "HEAD", "PUT"}
		if !containsAll(allowedMethods, expectedAllowedMethods) {
			t.Errorf("Access-Control-Allow-Methods = %v, want = %v", allowedMethods, expectedAllowedMethods)
		}
		if acao := resp.Header.Get("Access-Control-Allow-Origin"); acao != "*" {
			t.Errorf("Access-Control-Allow-Origin = %s, want = \"*\"", acao)
		}
	})

	t.Run("OPTIONS /upload", func(t *testing.T) {
		u := base.JoinPath("/upload")
		req, err := http.NewRequest(http.MethodOptions, u.String(), nil)
		if err != nil {
			t.Fatalf("failed to create OPTIONS request: %v", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to request: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusNoContent)
		}
		acam := resp.Header.Get("Access-Control-Allow-Methods")
		if acam == "" {
			t.Errorf("Access-Control-Allow-Methods got empty, want not empty")
		}
		allowedMethods := strings.Split(acam, ",")
		for i := range allowedMethods {
			allowedMethods[i] = strings.TrimSpace(allowedMethods[i])
		}
		expectedAllowedMethods := []string{"POST"}
		if !containsAll(allowedMethods, expectedAllowedMethods) {
			t.Errorf("Access-Control-Allow-Methods = %v, want = %v", allowedMethods, expectedAllowedMethods)
		}
		if acao := resp.Header.Get("Access-Control-Allow-Origin"); acao != "*" {
			t.Errorf("Access-Control-Allow-Origin = %s, want = \"*\"", acao)
		}
	})

}

// containsAll reports whether a contains all elements of b.
func containsAll[T comparable](a, b []T) bool {
	for _, x := range b {
		if !slices.Contains(a, x) {
			return false
		}
	}
	return true
}

// makeFormRequest creates a new http.Request with multipart/form-data.
func makeFormRequest(url *url.URL, method, filename string, content io.Reader) (*http.Request, error) {
	b := new(bytes.Buffer)
	w := multipart.NewWriter(b)
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		return nil, err
	}
	written, err := io.Copy(fw, content)
	if err != nil {
		return nil, err
	}
	if written == 0 {
		return nil, fmt.Errorf("no bytes written")
	}
	w.Close()

	req, err := http.NewRequest(method, url.String(), b)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req, nil
}

// getAvailablePort returns an available port number randomly.
func getAvailablePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}
