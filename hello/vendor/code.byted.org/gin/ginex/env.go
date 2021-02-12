package ginex

import (
	"code.byted.org/gopkg/env"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"strings"

	internal_util "code.byted.org/gin/ginex/internal/util"
	"code.byted.org/gopkg/logs"
)

const (
	PROD_USER_NAME      = "tiger"
	PROCESS_NAME_FORMAT = "/proc/%d/comm"
	IS_PRODUCT          = "IS_PROD_RUNTIME"
	GINEX_DEV           = "GINEX_DEV"
)

var (
	PROD_PARENT_PROC_NAMES = map[string]bool{
		"supervise": true,
		"systemd":   true,
	}
)

//Product returns true if current service is running on product environment
//product environment is determined by:
//  - environment variable IS_PROD_RUNTIME has been set
//  - user name is tiger and parent process name is any of supervise, systemd, etc.
func Product() bool {
	if env.IsTesting() || env.IsBoe() {
		return false
	}

	if os.Getenv(GINEX_DEV) != "" {
		return false
	}
	if os.Getenv(IS_PRODUCT) == "1" {
		return true
	}
	if user, err := user.Current(); err != nil {
		logs.Errorf("Failed to get current user: %s", err)
		return false
	} else if user.Username != PROD_USER_NAME {
		return false
	}
	if procName, err := parentProcName(); err != nil {
		logs.Errorf("Failed to get parent process name: %s", err)
		return false
	} else {
		return PROD_PARENT_PROC_NAMES[procName]
	}
}

// parentProcName return the parent process name
func parentProcName() (string, error) {
	ppid := os.Getppid()
	bs, err := ioutil.ReadFile(fmt.Sprintf(PROCESS_NAME_FORMAT, ppid))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(bs)), nil
}

// LocalIP returns host's ip
func LocalIP() string {
	return internal_util.LocalIP()
}

// LocalCluster returns local logic cluster
func LocalCluster() string {
	return internal_util.LocalCluster()
}
