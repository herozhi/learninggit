package conf

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"code.byted.org/golf/ssconf"
	"code.byted.org/gopkg/env"
	"code.byted.org/gopkg/logs"
	"git.byted.org/ttgame/platform/nvwa/environ"
)

const (
	defaultDirPath   = "./conf"
	filenameTemplate = "deploy_%s.conf"
)

var (
	globalConfs map[string]string
	initOnce    sync.Once
	filePath    string
)

func InitConf() {
	InitConfWithPath(defaultDirPath)
}

func InitConfWithPath(dirPath string) {
	logs.Warnf("检查到当前环境为 %v 环境", environ.GetCurrentEnv())
	filePath = filepath.Join(dirPath, filenameTemplate)
	initOnce.Do(initConf)
}

func initConf() {
	var name string
	if environ.IsDevelop() {
		if os.Getenv("TCE_CLUSTER") != "" {
			logs.Fatalf("当前的环境为TCE环境，但是没有配置环境变量ENV")
			panic("当前的环境为TCE环境，但是没有配置环境变量ENV")
		}
		name = "dev"
		//海外开发机特殊处理
		if environ.GetCurrentEnv() == "devi18n" {
			name = "devi18n"
		}
	} else {
		switch env.IDC() {
		case env.DC_LF, env.DC_HL, env.DC_LQ:
			name = "lf"
		case env.DC_MALIVA, env.DC_USEAST3:
			name = "maliva"
		case env.DC_SG1, env.DC_ALISG:
			name = "alisg"
		default:
			name = env.IDC()
		}
		if environ.IsSandbox() { //沙箱环境
			name = fmt.Sprintf("%s_sandbox", name)
		}
	}

	//先读取公共的
	commonFilePath := fmt.Sprintf(filePath, "common")
	if isExist(commonFilePath) {
		globalConfs = readConf(commonFilePath)
	} else {
		globalConfs = make(map[string]string)
	}
	//再读取具体地区的配置
	regionFilePath := fmt.Sprintf(filePath, name)
	confs := readConf(regionFilePath)
	for k, v := range confs {
		if globalConfs[k] != "" {
			logs.Info("覆盖common中的配置: %v: %v => %v", k, globalConfs[k], v)
		}
		globalConfs[k] = v
	}

	logs.Warn("读取的配置文件是: %v\n配置内容:%+v", regionFilePath, globalConfs)
}

func isExist(f string) bool {
	_, err := os.Stat(f)
	return err == nil || os.IsExist(err)
}

//读取配置文件
func readConf(path string) map[string]string {
	conf, err := ssconf.LoadSsConfFile(path)
	if err != nil {
		logs.Fatalf("load conf file err=%v", err)
		panic("load conf file error " + err.Error())
	}
	logs.Info("正在读取的配置文件: %v\n配置内容:%+v", path, conf)
	return conf
}

//GetConf 获取配置
func GetConf(key string) string {
	return globalConfs[key]
}

func SetConf(key, val string) {
	globalConfs[key] = val
}

func GetDuration(key string) (time.Duration, error) {
	val, err := time.ParseDuration(globalConfs[key])
	if err != nil {
		logs.Fatalf("[GetDuration] %v error, err=%v", globalConfs[key], err)
		return 0, err
	}
	return val, nil
}

func MustGetDuration(key string) time.Duration {
	d, _ := GetDuration(key)
	return d
}

func GetInt(key string) (int, error) {
	val, err := strconv.Atoi(globalConfs[key])
	if err != nil {
		logs.Fatalf("[GetInt] %v error, err=%v", globalConfs[key], err)
		return 0, err
	}
	return val, nil
}

func MustGetInt(key string) int {
	val, _ := GetInt(key)
	return val
}

func GetBool(key string) (bool, error) {
	val, exists := globalConfs[key]
	if !exists {
		return false, fmt.Errorf("%s not exists", key)
	}
	switch val {
	case "yes", "y", "true", "1":
		return true, nil
	}
	return false, nil
}

func MustGetBool(key string) bool {
	val, _ := GetBool(key)
	return val
}
