package bconfig

import (
	"encoding/json"
	"errors"
	"io"
)

var errKeyNotFound = errors.New("key not found")

func IsKeyNotFound(err error) bool {
	return err == errKeyNotFound
}

type response struct {
	Code    int    `json:"error_code"`
	Message string `json:"message"`
	Cause   string `json:"cause"`
	Data    Data   `json:"data"`
}

type Data struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func decodeResponse(r io.Reader) (string, error) {
	var resp response
	if err := json.NewDecoder(r).Decode(&resp); err != nil {
		return "", err
	}
	if resp.Code == 100 { // ErrorCodeKeyNotFound  = 100
		return "", errKeyNotFound
	}
	if resp.Message != "" {
		return "", errors.New(resp.Message)
	}
	return resp.Data.Value, nil
}
