package environ

import (
	"os"

	"code.byted.org/gopkg/env"
)

const (
	ENV        = "ENV"
	OnlineEnv  = "online"
	SandBoxEnv = "sandbox"
	DevEnv     = "dev"
	BoeEnv     = "boe"
)

// GetCurrentEnv 获取当前环境变量
func GetCurrentEnv() string {
	env := os.Getenv(ENV)
	if env == "" {
		return DevEnv
	}
	return env
}

// GetEnvironmentVal 根据环境变量 key 获取 value
func GetEnvironmentVal(key string) string {
	if key == "" {
		return ""
	}

	return os.Getenv(key)
}

// IsOnline 判断是否是线上环境
func IsOnline() bool {
	return GetCurrentEnv() == OnlineEnv
}

// IsSandbox 判断是否是沙盒环境
func IsSandbox() bool {
	return GetCurrentEnv() == SandBoxEnv
}

// IsDevelop 判断是否是本地测试环境
func IsDevelop() bool {
	env := GetCurrentEnv()

	if env == DevEnv {
		return true
	}

	if env != OnlineEnv && env != SandBoxEnv && env != BoeEnv {
		return true
	}

	return false
}

// IsBoe 是否为线下环境
func IsBoe() bool {
	return GetCurrentEnv() == BoeEnv
}

// GetServicePSM 获取本服务PSM
func GetServicePSM() string {
	psm := os.Getenv("LOAD_SERVICE_PSM")
	if psm != "" {
		return psm
	}
	psm = os.Getenv("TCE_PSM")
	if psm != "" {
		return psm
	}
	return ""
}

// 是否为国内机房
func IsCN() bool {
	switch env.IDC() {
	case env.DC_LF, env.DC_HL, env.DC_LQ, env.DC_BOE:
		return true
	default:
		return false
	}
}

// 是否是海外机房
func IsOverseas() bool {
	return IsSG() || IsVA()
}

// 是否为新加坡机房
func IsSG() bool {
	return env.IDC() == env.DC_ALISG || env.IDC() == env.DC_SG1
}

// 是否为美东机房
func IsVA() bool {
	return env.IDC() == env.DC_MALIVA || env.IDC() == env.DC_USEAST3 || env.IDC() == env.DC_IBOE
}

const (
	CN      = "CN"
	SG      = "SG"
	VA      = "VA"
	UNKNOWN = "UNKNOWN"
)

func Region() string {
	switch {
	case IsCN():
		return CN
	case IsSG():
		return SG
	case IsVA():
		return VA
	default:
		return UNKNOWN
	}
}

func LocalCluster() string {
	if cluster := os.Getenv("TCE_CLUSTER"); cluster != "" {
		return cluster
	} else if cluster = os.Getenv("SERVICE_CLUSTER"); cluster != "" {
		return cluster
	} else {
		return "default"
	}
}
