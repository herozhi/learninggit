package tccclient

import (
	"fmt"
	"time"
)

const (
	DefaultCluster = "default"
	DefaultEnv     = "prod"

	DefaultConfspace = "default"
)

type Config struct {
	// ServiceName string
	Cluster string
	Env     string

	DisableMetrics bool
}

func NewConfig() *Config {
	c := Config{
		Cluster: DefaultCluster,
		Env:     DefaultEnv,
	}
	return &c
}

type ConfigV2 struct {
	Confspace      string
	ListenInterval time.Duration
}

func NewConfigV2() *ConfigV2 {
	c := ConfigV2{
		Confspace:      DefaultConfspace,
		ListenInterval: 60 * time.Second,
	}
	return &c
}

func (c *ConfigV2) check() error {
	if c.ListenInterval < minListenInterval {
		return fmt.Errorf("[tcc] config.ListenInterval < minListenInterval:%v", minListenInterval)
	}
	return nil
}
