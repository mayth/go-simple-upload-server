package main

import (
	"os"
	"reflect"
	"testing"

	simpleuploadserver "github.com/mayth/go-simple-upload-server/v2/pkg"
)

func Test_parseConfig(t *testing.T) {
	t.Run("use config file only", func(t *testing.T) {
		f, err := os.CreateTemp("", "simple-upload-server-config.*.json")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(f.Name())
		defer f.Close()
		f.WriteString(`{
			"addr": ":8123",
			"document_root": "/opt/app",
			"enable_cors": true,
			"max_upload_size": 1234567,
			"file_naming_strategy": "uuid",
			"shutdown_timeout": 15000,
			"enable_auth": true,
			"read_only_tokens": ["foo", "bar"],
			"read_write_tokens": ["baz", "qux"]
		}`)
		f.Sync()
		f.Seek(0, 0)

		app := NewApp(os.Args[0])
		got, err := app.ParseConfig([]string{"-config", f.Name()})
		if err != nil {
			t.Errorf("parseConfig() error = %v", err)
			return
		}
		want := &simpleuploadserver.ServerConfig{
			Addr:               ":8123",
			DocumentRoot:       "/opt/app",
			EnableCORS:         true,
			MaxUploadSize:      1234567,
			FileNamingStrategy: "uuid",
			ShutdownTimeout:    15000,
			EnableAuth:         true,
			ReadOnlyTokens:     []string{"foo", "bar"},
			ReadWriteTokens:    []string{"baz", "qux"},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("parseConfig() = %v, want %v", got, want)
		}
	})

	t.Run("use flag only", func(t *testing.T) {
		app := NewApp(os.Args[0])
		args := []string{
			"-addr", ":8987",
			"-document_root", "/tmp/sus",
			"-enable_cors=false",
			"-max_upload_size", "987654",
			"-file_naming_strategy", "uuid",
			"-shutdown_timeout", "30000",
			"-enable_auth=true",
			"-read_only_tokens", "foo,bar",
			"-read_write_tokens", "baz,qux",
		}
		got, err := app.ParseConfig(args)
		if err != nil {
			t.Errorf("parseConfig() error = %v", err)
			return
		}
		want := &simpleuploadserver.ServerConfig{
			Addr:               ":8987",
			DocumentRoot:       "/tmp/sus",
			EnableCORS:         false,
			MaxUploadSize:      987654,
			FileNamingStrategy: "uuid",
			ShutdownTimeout:    30000,
			EnableAuth:         true,
			ReadOnlyTokens:     []string{"foo", "bar"},
			ReadWriteTokens:    []string{"baz", "qux"},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("parseConfig() = %v, want %v", got, want)
		}
	})

	t.Run("flag options precedes config file", func(t *testing.T) {
		app := NewApp(os.Args[0])
		args := []string{
			"-addr", ":8987",
			"-document_root", "/tmp/sus",
			"-enable_cors=true",
			"-max_upload_size", "987654",
			"-file_naming_strategy", "uuid",
			"-shutdown_timeout", "30000",
		}

		f, err := os.CreateTemp("", "simple-upload-server-config.*.json")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(f.Name())
		defer f.Close()
		f.WriteString(`{
			"addr": ":8123",
			"document_root": "/opt/app",
			"enable_cors": true,
			"max_upload_size": 1234567,
			"file_naming_strategy": "uuid",
			"shutdown_timeout": 15000,
			"enable_auth": true,
			"read_only_tokens": ["alice", "bob"],
			"read_write_tokens": ["charlie", "dave"]
		}`)
		f.Sync()
		f.Seek(0, 0)
		args = append(args, "-config", f.Name())

		got, err := app.ParseConfig(args)
		if err != nil {
			t.Errorf("parseConfig() error = %v", err)
			return
		}
		want := &simpleuploadserver.ServerConfig{
			Addr:               ":8987",
			DocumentRoot:       "/tmp/sus",
			EnableCORS:         true,
			MaxUploadSize:      987654,
			FileNamingStrategy: "uuid",
			ShutdownTimeout:    30000,
			EnableAuth:         true,
			ReadOnlyTokens:     []string{"alice", "bob"},
			ReadWriteTokens:    []string{"charlie", "dave"},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("parseConfig() = %v, want %v", got, want)
		}
	})
}
