package main

import (
	"context"
	"crypto"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"

	"dario.cat/mergo"
	simpleuploadserver "github.com/mayth/go-simple-upload-server/v2/pkg"
)

var DefaultConfig = ServerConfig{
	DocumentRoot:       ".",
	Addr:               simpleuploadserver.DefaultAddr,
	EnableCORS:         nil,
	MaxUploadSize:      1024 * 1024,
	FileNamingStrategy: "uuid",
	ShutdownTimeout:    15000,
	EnableAuth:         nil,
	ReadOnlyTokens:     []string{},
	ReadWriteTokens:    []string{},
}

func BoolPointer(v bool) *bool {
	return &v
}

type triBool struct {
	value bool
	isSet bool
}

type boolOptFlag triBool

func (f *boolOptFlag) Set(value string) error {
	v, err := strconv.ParseBool(value)
	if err != nil {
		return err
	}
	f.value = v
	f.isSet = true
	return nil
}

func (f boolOptFlag) String() string {
	return strconv.FormatBool(f.value)
}

func (f boolOptFlag) IsBoolFlag() bool {
	return true
}

func (f boolOptFlag) IsSet() bool {
	return f.isSet
}

func (f boolOptFlag) Get() any {
	return f.value
}

type stringArrayFlag []string

func (f *stringArrayFlag) Set(value string) error {
	ss := strings.Split(value, ",")
	*f = ss
	return nil
}

func (f stringArrayFlag) String() string {
	return strings.Join(f, ",")
}

// ServerConfig wraps simpleuploadserver.ServerConfig to provide JSON marshaling.
type ServerConfig struct {
	// Address where the server listens on.
	Addr string `json:"addr"`
	// Path to the document root.
	DocumentRoot string `json:"document_root"`
	// Determines whether to enable CORS header.
	EnableCORS *bool `json:"enable_cors"`
	// Maximum upload size in bytes.
	MaxUploadSize int64 `json:"max_upload_size"`
	// File naming strategy.
	FileNamingStrategy string `json:"file_naming_strategy"`
	// Graceful shutdown timeout in milliseconds.
	ShutdownTimeout int `json:"shutdown_timeout"`
	// Enable authentication.
	EnableAuth *bool `json:"enable_auth"`
	// Authentication tokens for read-only access.
	ReadOnlyTokens []string `json:"read_only_tokens"`
	// Authentication tokens for read-write access.
	ReadWriteTokens []string `json:"read_write_tokens"`
}

func (c *ServerConfig) AsConfig() simpleuploadserver.ServerConfig {
	if c.EnableCORS == nil {
		c.EnableCORS = BoolPointer(true)
	}
	if c.EnableAuth == nil {
		c.EnableAuth = BoolPointer(false)
	}

	return simpleuploadserver.ServerConfig{
		Addr:               c.Addr,
		DocumentRoot:       c.DocumentRoot,
		EnableCORS:         *c.EnableCORS,
		MaxUploadSize:      c.MaxUploadSize,
		FileNamingStrategy: c.FileNamingStrategy,
		ShutdownTimeout:    c.ShutdownTimeout,
		EnableAuth:         *c.EnableAuth,
		ReadOnlyTokens:     c.ReadOnlyTokens,
		ReadWriteTokens:    c.ReadWriteTokens,
	}
}

func main() {
	NewApp(os.Args[0]).Run(os.Args[1:])
}

type app struct {
	flagSet            *flag.FlagSet
	configFilePath     string
	documentRoot       string
	addr               string
	enableCORS         boolOptFlag
	maxUploadSize      int64
	fileNamingStrategy string
	shutdownTimeout    int
	enableAuth         boolOptFlag
	readOnlyTokens     stringArrayFlag
	readWriteTokens    stringArrayFlag
}

func NewApp(name string) *app {
	a := &app{}
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	fs.StringVar(&a.configFilePath, "config", "", "path to config file")
	fs.StringVar(&a.documentRoot, "document_root", "", "path to document root directory")
	fs.StringVar(&a.addr, "addr", "", "address to listen")
	fs.Var(&a.enableCORS, "enable_cors", "enable CORS header")
	fs.Int64Var(&a.maxUploadSize, "max_upload_size", 0, "max upload size in bytes")
	fs.StringVar(&a.fileNamingStrategy, "file_naming_strategy", "", "File naming strategy")
	fs.IntVar(&a.shutdownTimeout, "shutdown_timeout", 0, "graceful shutdown timeout in milliseconds")
	fs.Var(&a.enableAuth, "enable_auth", "enable authentication")
	fs.Var(&a.readOnlyTokens, "read_only_tokens", "comma separated list of read only tokens")
	fs.Var(&a.readWriteTokens, "read_write_tokens", "comma separated list of read write tokens")
	a.flagSet = fs
	return a
}

func (a *app) Run(args []string) {
	config, err := a.ParseConfig(args)
	if err != nil {
		log.Fatalf("failed to parse config: %v", err)
	}
	log.Printf("configured: %+v", config)

	if config.EnableAuth && len(config.ReadOnlyTokens) == 0 && len(config.ReadWriteTokens) == 0 {
		log.Print("[NOTICE] Authentication is enabled but no tokens provided. generating random tokens")
		readOnlyToken, err := generateToken()
		if err != nil {
			log.Fatalf("failed to generate read only token: %v", err)
		}
		readWriteToken, err := generateToken()
		if err != nil {
			log.Fatalf("failed to generate read write token: %v", err)
		}
		config.ReadOnlyTokens = append(config.ReadOnlyTokens, readOnlyToken)
		config.ReadWriteTokens = append(config.ReadWriteTokens, readWriteToken)
		log.Printf("generated read only token: %s", readOnlyToken)
		log.Printf("generated read write token: %s", readWriteToken)
	}

	s := simpleuploadserver.NewServer(*config)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	err = s.Start(ctx, nil)
	log.Printf("server stopped: %v", err)
}

func generateToken() (string, error) {
	randBytes := make([]byte, 32)
	if _, err := rand.Read(randBytes); err != nil {
		return "", err
	}
	b := crypto.SHA256.New().Sum(randBytes)
	return fmt.Sprintf("%x", b), nil
}

// parseConfig parses the configuration from the `src` and merges it with the `orig` configuration.
func (a *app) ParseConfig(args []string) (*simpleuploadserver.ServerConfig, error) {
	if err := a.flagSet.Parse(args); err != nil {
		return nil, err
	}

	config := DefaultConfig

	if a.configFilePath != "" {
		f, err := os.Open(a.configFilePath)
		if err != nil {
			log.Fatalf("failed to open config file: %v", err)
		}
		defer f.Close()
		fileConfig := ServerConfig{}
		if err := json.NewDecoder(f).Decode(&fileConfig); err != nil {
			return nil, fmt.Errorf("failed to load config: %w", err)
		}
		log.Printf("loaded config from source file: %+v", fileConfig)
		if err := mergo.Merge(&config, fileConfig, mergo.WithOverride); err != nil {
			return nil, fmt.Errorf("failed to merge config from file: %w", err)
		}
		log.Printf("merged file config: %+v", config)
	} else {
		log.Print("no config file provided")
	}

	configFromFlags := ServerConfig{
		DocumentRoot:       a.documentRoot,
		Addr:               a.addr,
		MaxUploadSize:      a.maxUploadSize,
		FileNamingStrategy: a.fileNamingStrategy,
		ShutdownTimeout:    a.shutdownTimeout,
		ReadOnlyTokens:     a.readOnlyTokens,
		ReadWriteTokens:    a.readWriteTokens,
	}
	if a.enableCORS.IsSet() {
		configFromFlags.EnableCORS = &a.enableCORS.value
	}
	if a.enableAuth.IsSet() {
		configFromFlags.EnableAuth = &a.enableAuth.value
	}
	log.Printf("config from flag: %+v", configFromFlags)
	if err := mergo.Merge(&config, configFromFlags, mergo.WithOverride); err != nil {
		return nil, fmt.Errorf("failed to merge config from flags: %w", err)
	}
	log.Printf("merged flag config: %+v", config)

	v := config.AsConfig()
	return &v, nil
}
