package main

import (
	"encoding/json"
	"io"
	"net/http"
)

type response struct {
	OK bool `json:"ok"`
}

type uploadedResponse struct {
	response
	Path string `json:"path"`
}

func newUploadedResponse(path string) uploadedResponse {
	return uploadedResponse{response: response{OK: true}, Path: path}
}

type errorResponse struct {
	response
	Message string `json:"error"`
}

func newErrorResponse(err error) errorResponse {
	return errorResponse{response: response{OK: false}, Message: err.Error()}
}

func writeError(w http.ResponseWriter, err error) (int, error) {
	body := newErrorResponse(err)
	b, e := json.Marshal(body)
	// if an error is occured on marshaling, write empty value as response.
	if e != nil {
		return w.Write([]byte{})
	}
	return w.Write(b)
}

func writeSuccess(w http.ResponseWriter, path string) (int, error) {
	body := newUploadedResponse(path)
	b, e := json.Marshal(body)
	// if an error is occured on marshaling, write empty value as response.
	if e != nil {
		return w.Write([]byte{})
	}
	return w.Write(b)
}

func getSize(content io.Seeker) (int64, error) {
	size, err := content.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, err
	}
	_, err = content.Seek(0, io.SeekStart)
	if err != nil {
		return 0, err
	}
	return size, nil
}
