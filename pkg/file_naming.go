package simpleuploadserver

import (
	"crypto/sha256"
	"fmt"
	"io"
	"mime/multipart"
	"strings"

	"github.com/google/uuid"
)

type FileNamingStrategy func(multipart.File, *multipart.FileHeader) (string, error)

func UUIDStrategy(multipart.File, *multipart.FileHeader) (string, error) {
	return uuid.NewString(), nil
}

func SHA256Strategy(file multipart.File, info *multipart.FileHeader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

var strategies = map[string]FileNamingStrategy{
	"uuid":   UUIDStrategy,
	"sha256": SHA256Strategy,
}

var DefaultNamingStrategy FileNamingStrategy = UUIDStrategy

func ResolveFileNamingStrategy(name string) FileNamingStrategy {
	if name == "" {
		return DefaultNamingStrategy
	}
	return strategies[strings.ToLower(name)]
}
