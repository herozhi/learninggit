package bytedmysql

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	"code.byted.org/gopkg/asyncache"
	consulHttp "code.byted.org/gopkg/consul/http"
	"code.byted.org/gopkg/env"
	"code.byted.org/inf/infsecc"
)

const (
	dbAuthService      = "toutiao.mysql.dbauth_service"
	unknownServiceName = "toutiao.unknown.unknown"
	authKeySuffix      = "_AUTHKEY"
)

var (
	consulHTTPClient *consulHttp.HttpClient
	httpClient       *http.Client
	authCache        *asyncache.Asyncache
	transport        = &http.Transport{
		DisableKeepAlives: true,
	}
)

func init() {
	httpClient = &http.Client{
		Transport: transport,
		Timeout:   500 * time.Millisecond,
	}
	consulHTTPClient = consulHttp.NewHttpClient(consulHttp.WithKeepAlives(false))
	consulHTTPClient.Timeout = 500 * time.Millisecond

	authGetter := func(key string) (interface{}, error) {
		userPwd, err := getServiceInfo(key)
		if err != nil {
			return nil, err
		}
		return userPwd, nil
	}
	authErrHandler := func(key string, err error) {
		if err != nil {
			log.Printf("bytedmysql: get key %s failed, error is %s", key, err.Error())
		}
	}
	authCache = asyncache.NewAsyncache(asyncache.Options{
		BlockIfFirst:    true,
		RefreshDuration: time.Second * 120,
		Fetcher:         authGetter,
		ErrHandler:      authErrHandler,
	})
}

type dbInfo struct {
	user   string
	pwd    string
	dbName string
}

func getDbInfo(dbServiceName string) (*dbInfo, error) {
	// 我把 key 换了，反正 key serviceName 和 psm 又不会变
	item := authCache.Get(dbServiceName, nil)
	if item == nil {
		return nil, fmt.Errorf("bytedmysql: get info from cache error")
	}

	if v, ok := item.(string); ok {
		if v == "" {
			return nil, fmt.Errorf("bytedmysql: auth failed")
		}
		tmp := strings.Split(v, "-")
		if len(tmp) != 3 {
			return nil, fmt.Errorf("bytedmysql: cache format is wrong, format is %v", v)
		}
		return &dbInfo{
			user:   tmp[0],
			pwd:    tmp[1],
			dbName: tmp[2],
		}, nil
	}

	return nil, fmt.Errorf("bytedmysql: cache type string expected, cache is %v", item)
}

type dbInfoReq struct {
	// 要访问的数据库的 PSM
	ServiceName string `json:"serviceName"`
	// 被授权的服务 PSM
	Psm string `json:"psm"`
	// Dps Token向数据库平台获取 Auth 信息的凭证
	Token string `json:"token"`
	// Auth Key 旧方案，被 Dps Token 替代
	AuthKey string `json:"authkey"`
	Version string `json:"version"`
}

const version = "bytedmysql-" + Version

func getServiceInfo(dbServiceName string) (res string, err error) {
	serviceName := env.PSM()
	if serviceName == env.PSMUnknown {
		// DBA 要求
		serviceName = unknownServiceName
	}

	authKey := os.Getenv(consulName2EnvKey(dbServiceName))
	token, _ := infsecc.GetToken(true)

	req := &dbInfoReq{
		ServiceName: dbServiceName,
		Psm:         serviceName,
		AuthKey:     authKey,
		Token:       token,
		Version:     version,
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	// 先用 consul
	res, err = post(fmt.Sprintf("http://%s/getdbinfo", dbAuthService), payload, true)
	if err == nil {
		return res, nil
	}

	log.Printf("bytedmysql: get dbinfo err, err is %s. Using dbinfo domain as a fallback, and this does not cause any error.", err)

	// 再降级用域名
	res, err = post("http://dbauth.byted.org/getdbinfo", payload, false)
	if err == nil {
		return res, nil
	}

	return "", err
}

func post(url string, data []byte, useConsul bool) (string, error) {
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Add("Content-Type", "application/json")

	var resp *http.Response
	if useConsul {
		resp, err = consulHTTPClient.Do(req)
	} else {
		resp, err = httpClient.Do(req)
	}
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func consulName2EnvKey(s string) string {
	s = strings.Replace(s, ".", "_", -1)
	s = strings.ToUpper(s) + authKeySuffix
	return s
}

// 插入认证所需的元信息，包括 username、password、db
func interpolateAuthMeta(cfg *mysql.Config, dbPsm string) error {
	// notice: Cfg.Addr must be `psm` of the database
	useGDPRToken := false
	if v, ok := cfg.Params["use_gdpr_auth"]; ok {
		if v == "true" {
			useGDPRToken = true
		}
		delete(cfg.Params, "use_gdpr_auth")
	}
	if !useGDPRToken {
		return nil
	}

	dbInfo, err := getDbInfo(dbPsm)
	if err != nil {
		log.Printf("bytedmysql: auth failed, use original auth info which user provided")
		return nil
	}
	cfg.User = dbInfo.user
	cfg.Passwd = dbInfo.pwd
	cfg.DBName = dbInfo.dbName
	return nil
}
