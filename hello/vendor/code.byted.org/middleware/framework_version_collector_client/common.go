package client

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"code.byted.org/gopkg/env"
)

type Logger interface {
	Info(format string, v ...interface{})
	Warn(format string, v ...interface{})
}

var Domain string

func init() {
	Domain = "http://fvc-boe.byted.org"
	if !env.IsProduct() {
		return
	}
	switch env.Region() {
	case env.R_BOE:
		Domain = "http://fvc-boe.byted.org"
	case env.R_CN:
		Domain = "http://fvc-cn.byted.org"
	case env.R_ALISG, env.R_SG:
		Domain = "http://fvc-sg.byted.org"
	case env.R_MALIVA, env.R_US, env.R_CA:
		Domain = "http://fvc-us.byted.org"
	default:
		if strings.HasPrefix(env.Region(), env.R_SUITEKA) {
			Domain = "http://fvc-cn.byted.org"
		} else {
			// 未知区域全部归到 sg
			Domain = "http://fvc-sg.byted.org"
		}
	}
}

func PostJson(url string, data interface{}, logger Logger) (*http.Response, error) {
	b, err := json.Marshal(data)
	if err != nil {
		logger.Warn("marshal json failed: %v", err)
		return nil, err
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		logger.Warn("http post failed: %v", err)
	}
	return resp, err
}
