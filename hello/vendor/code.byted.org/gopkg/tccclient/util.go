package tccclient

import (
	"code.byted.org/gopkg/env"
)

// IsConfigNotFoundError returns whether an error is ConfigNotFoundError, which means the config
// has not been set
func IsConfigNotFoundError(err error) bool {
	return err == ConfigNotFoundError
}

// GetClusterFromEnv returns cluster reading from system env, "default" is returned
// when it's empty
func GetClusterFromEnv() string {
	return env.Cluster()
}

// GetServiceNameFromEnv returns service name reading from system env, "-" is returned
// when it's empty
func GetServiceNameFromEnv() string {
	return env.PSM()
}
