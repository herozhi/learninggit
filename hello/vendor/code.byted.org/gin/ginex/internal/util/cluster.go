package util

import (
	"os"

	"code.byted.org/gin/ginex/internal"
)

const (
	DEFAULT_CLUSTER = "default"
)

var (
	localCluster string
)

// LocalCluter returns service's local cluster
func LocalCluster() string {
	return localCluster
}

func getLocalCluster() string {
	if cluster := os.Getenv(internal.TCE_CLUSTER); cluster != "" {
		return cluster
	} else if cluster = os.Getenv(internal.SERVICE_CLUSTER); cluster != "" {
		return cluster
	} else {
		return DEFAULT_CLUSTER
	}
}

func init() {
	localCluster = getLocalCluster()
}
