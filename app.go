package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"

	"dario.cat/mergo"
	simpleuploadserver "github.com/mayth/go-simple-upload-server/v2/pkg"
)

func main() {
	configFile := flag.String("config", "", "path to config file")
	config := simpleuploadserver.ServerConfig{}
	flag.StringVar(&config.DocumentRoot, "document_root", ".", "path to document root directory")
	flag.StringVar(&config.Addr, "addr", simpleuploadserver.DefaultAddr, "address to listen")
	flag.BoolVar(&config.EnableCORS, "enable_cors", true, "enable CORS header")
	flag.Int64Var(&config.MaxUploadSize, "max_upload_size", 1024*1024, "max upload size in bytes")
	flag.StringVar(&config.FileNamingStrategy, "file_naming_strategy", "uuid", "File naming strategy")
	flag.IntVar(&config.ShutdownTimeout, "shutdown_timeout", 15000, "graceful shutdown timeout in milliseconds")
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
		mergo.Merge(&fileConfig, config, mergo.WithOverride)
		config = fileConfig
	}

	s := simpleuploadserver.NewServer(config)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	err := s.Start(ctx)
	log.Printf("server stopped: %v", err)
}
