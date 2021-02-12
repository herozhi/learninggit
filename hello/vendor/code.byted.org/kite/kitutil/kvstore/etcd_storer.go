package kvstore

import (
	"context"
	"fmt"
)

type etcdStorer struct{}

// NewETCDStorer .
func NewETCDStorer() KVStorer {
	return &etcdStorer{}
}

func (e *etcdStorer) Get(key string) (string, error) {
	val, err := getEtcdProxyClient().Get(context.Background(), key)
	if err != nil {
		return "", err
	}
	return val, nil
}

func (e *etcdStorer) GetOrCreate(key, val string) (string, error) {
	v, err := e.Get(key)
	if err != nil && IsKeyNotFound(err) {
		// key not found is not defined as an error
		// don't create KV
		return val, nil
	}

	if err != nil {
		return "", fmt.Errorf("get key=%s err: %s", key, err.Error())
	}

	return v, nil
}
