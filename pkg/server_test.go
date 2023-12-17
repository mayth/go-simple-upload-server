package simpleuploadserver

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path"
	"path/filepath"
	"reflect"
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
