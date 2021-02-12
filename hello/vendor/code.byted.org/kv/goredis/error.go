package goredis

import "errors"

var (
	ErrEmptyClusterName      = errors.New("Redisclient: cluster name is empty")
	ErrClusterConfigNotFound = errors.New("Redisclient: cluster config not found")
	ErrEmptyServerList       = errors.New("Redisclient: server list is empty")
	ErrConsulServerEmpty     = errors.New("Redisclient: no servers found by consul")
	ErrEtcdServerEmpty       = errors.New("Redisclient: no servers found by etcd")
	ErrDegradated            = errors.New("Redisclient: rpc degradated")
)
