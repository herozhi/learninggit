package ginex

import (
	"io/ioutil"
	"os"
	"path"

	"gopkg.in/yaml.v2"
)

var (
	configEnv  string
	customMode string
)

func init() {
	configEnv = os.Getenv("CONF_ENV")
}

// UnmarshallYMLConfig parses the file specified by confName and CONF_ENV to a YML object;
// If file "confName.CONF_ENV" doesn't exist, then try to unmarshal "confName" as default;
func UnmarshallYMLConfig(confName string, obj interface{}) error {
	buf, err := ReadConfig(confName)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(buf, obj)
}

// ReadConfig reads the config specified by confName and CONF_ENV;
// If file "confName.CONF_ENV" doesn't exist, then try to read "confName" as default;
func ReadConfig(confName string) ([]byte, error) {
	confName = path.Join(ConfDir(), confName)
	if GetConfEnv() != "" {
		confName = confName + "." + GetConfEnv()
	}
	return ioutil.ReadFile(confName)
}

// GetConfEnv returns the CONF_ENV
func GetConfEnv() string {
	return configEnv
}

// GetCustomMode: returns user-define customMode
func GetCustomMode() string {
	return customMode
}

// WithModeKey: set customMode by user
func WithModeKey(mode string) {
	customMode = mode
}
