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
	"strings"

	"dario.cat/mergo"
	simpleuploadserver "github.com/mayth/go-simple-upload-server/v2/pkg"
)

type commaSeparatedString []string

func (s *commaSeparatedString) String() string {
	return strings.Join(*s, ",")
}

func (s *commaSeparatedString) Set(value string) error {
	*s = strings.Split(value, ",")
	return nil
}

func main() {
	configFile := flag.String("config", "", "path to config file")
	config := simpleuploadserver.ServerConfig{}
	flag.StringVar(&config.DocumentRoot, "document_root", ".", "path to document root directory")
	flag.StringVar(&config.Addr, "addr", simpleuploadserver.DefaultAddr, "address to listen")
	flag.BoolVar(&config.EnableCORS, "enable_cors", true, "enable CORS header")
	flag.Int64Var(&config.MaxUploadSize, "max_upload_size", 1024*1024, "max upload size in bytes")
	flag.StringVar(&config.FileNamingStrategy, "file_naming_strategy", "uuid", "File naming strategy")
	flag.IntVar(&config.ShutdownTimeout, "shutdown_timeout", 15000, "graceful shutdown timeout in milliseconds")
	flag.BoolVar(&config.EnableAuth, "enable_auth", false, "enable authentication")
	flag.Var((*commaSeparatedString)(&config.ReadOnlyTokens), "read_only_tokens", "comma separated list of read only tokens")
	flag.Var((*commaSeparatedString)(&config.ReadWriteTokens), "read_write_tokens", "comma separated list of read write tokens")
	flag.Parse()

	if *configFile != "" {
		f, err := os.Open(*configFile)
		if err != nil {
			log.Fatalf("failed to load config: %v", err)
		}
		fileConfig := simpleuploadserver.ServerConfig{}
		if err := json.NewDecoder(f).Decode(&fileConfig); err != nil {
			log.Fatalf("failed to load config: %v", err)
		}
		if err := mergo.Merge(&fileConfig, config, mergo.WithOverride); err != nil {
			log.Fatalf("failed to merge config: %v", err)
		}
		config = fileConfig
	}

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

	s := simpleuploadserver.NewServer(config)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	err := s.Start(ctx, nil)
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
