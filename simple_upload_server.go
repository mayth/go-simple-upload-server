package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"crypto/rand"

	"github.com/sirupsen/logrus"
)

var logger *logrus.Logger

func run(args []string) int {
	bindAddress := flag.String("ip", "0.0.0.0", "IP address to bind")
	listenPort := flag.Int("port", 25478, "port number to listen on")
	tlsListenPort := flag.Int("tlsport", 25443, "port number to listen on with TLS")
	// 5,242,880 bytes == 5 MiB
	maxUploadSize := flag.Int64("upload_limit", 5242880, "max size of uploaded file (byte)")
	tokenFlag := flag.String("token", "", "specify the security token (it is automatically generated if empty)")
	protectedMethodFlag := flag.String("protected_method", "GET,POST,HEAD,PUT", "specify methods intended to be protect by the security token")
	logLevelFlag := flag.String("loglevel", "info", "logging level")
	certFile := flag.String("cert", "", "path to certificate file")
	keyFile := flag.String("key", "", "path to key file")
	corsEnabled := flag.Bool("cors", false, "if true, add ACAO header to support CORS")
	flag.Parse()
	serverRoot := flag.Arg(0)
	if len(serverRoot) == 0 {
		flag.Usage()
		return 2
	}
	if logLevel, err := logrus.ParseLevel(*logLevelFlag); err != nil {
		logrus.WithError(err).Error("failed to parse logging level, so set to default")
	} else {
		logger.Level = logLevel
	}
	token := *tokenFlag
	if token == "" {
		count := 10
		b := make([]byte, count)
		if _, err := rand.Read(b); err != nil {
			logger.WithError(err).Fatal("could not generate token")
			return 1
		}
		token = fmt.Sprintf("%x", b)
		logger.WithField("token", token).Warn("token generated")
	}
	protectedMethods := []string{}
	for _, method := range strings.Split((*protectedMethodFlag), ",") {
		if strings.EqualFold("GET", method) {
			protectedMethods = append(protectedMethods, http.MethodGet)
		} else if strings.EqualFold("POST", method) {
			protectedMethods = append(protectedMethods, http.MethodPost)
		} else if strings.EqualFold("HEAD", method) {
			protectedMethods = append(protectedMethods, http.MethodHead)
		} else if strings.EqualFold("PUT", method) {
			protectedMethods = append(protectedMethods, http.MethodPut)
		} else if strings.EqualFold("OPTIONS", method) {
			protectedMethods = append(protectedMethods, http.MethodOptions)
		}
	}
	tlsEnabled := *certFile != "" && *keyFile != ""
	server := NewServer(serverRoot, *maxUploadSize, token, *corsEnabled, protectedMethods)
	http.Handle("/upload", server)
	http.Handle("/files/", server)

	errors := make(chan error)

	go func() {
		logger.WithFields(logrus.Fields{
			"ip":               *bindAddress,
			"port":             *listenPort,
			"token":            token,
			"protected_method": protectedMethods,
			"upload_limit":     *maxUploadSize,
			"root":             serverRoot,
			"cors":             *corsEnabled,
		}).Info("start listening")

		if err := http.ListenAndServe(fmt.Sprintf("%s:%d", *bindAddress, *listenPort), nil); err != nil {
			errors <- err
		}
	}()

	if tlsEnabled {
		go func() {
			logger.WithFields(logrus.Fields{
				"cert": *certFile,
				"key":  *keyFile,
				"port": *tlsListenPort,
			}).Info("start listening TLS")

			if err := http.ListenAndServeTLS(fmt.Sprintf("%s:%d", *bindAddress, *tlsListenPort), *certFile, *keyFile, nil); err != nil {
				errors <- err
			}
		}()
	}

	err := <-errors
	logger.WithError(err).Info("closing server")

	return 0
}

func main() {
	logger = logrus.New()
	logger.Info("starting up simple-upload-server")

	result := run(os.Args)
	os.Exit(result)
}
