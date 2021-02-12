package etcdproxy

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
	Code    int    `json:"errorCode"`
	Message string `json:"message"`
	Cause   string `json:"cause"`
	Node    struct {
		Value string `json:"value"`
	} `json:"node"`
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
	return resp.Node.Value, nil
}
