package bytedmysql

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"log"
	"math/rand"
	"os"

	"github.com/go-sql-driver/mysql"

	"code.byted.org/gopkg/consul"
)

type Driver struct{}

var (
	originDriver = mysql.MySQLDriver{}
	consulLookup = consul.LookupName // 为了单测
)

var ErrNoEndpoints = errors.New("bytedmysql: consul returned empty endpoint")

var makeConn = func(conn driver.Conn, cfg *mysql.Config, to string) (driver.Conn, error) {
	return &Connection{
		WrappedConn: conn,
		Cfg:         cfg,
		DBPSM:       to,
	}, nil
}

// 用于业务封装 conn
func SetMakeConn(f func(conn driver.Conn, cfg *mysql.Config, to string) (driver.Conn, error)) {
	makeConn = f
}

// 对 Driver 接口的实现
func (d Driver) Open(dsn string) (driver.Conn, error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, err
	}

	to := cfg.Addr

	// 如果开启 Mesh，走 Mesh
	if os.Getenv("TCE_ENABLE_MYSQL_SIDECAR_EGRESS") == "True" && os.Getenv("SERVICE_MESH_MYSQL_ADDR") != "" {
		cfg.Net = "unix"
		cfg.User = cfg.Addr
		cfg.Addr = os.Getenv("SERVICE_MESH_MYSQL_ADDR")
		delete(cfg.Params, "use_gdpr_auth")
		return d.open(cfg, to)
	}

	// 服务发现
	if cfg.Net == "sd" {
		cfg.Net = "tcp"
		dbPsm := cfg.Addr

		tmpEndpoints, err := consulLookup(dbPsm)
		if err != nil {
			return nil, err
		}

		if len(tmpEndpoints) == 0 {
			return nil, ErrNoEndpoints
		}

		endpoints := make(consul.Endpoints, len(tmpEndpoints))

		copy(endpoints, tmpEndpoints)

		rand.Shuffle(len(endpoints), func(i, j int) {
			endpoints[i], endpoints[j] = endpoints[j], endpoints[i]
		})

		var conn driver.Conn

		// retry all endpoints
		for _, ed := range endpoints {
			cfg.Addr = ed.Addr
			conn, err = d.openByToken(cfg, dbPsm)
			if err == nil {
				return conn, nil
			}
			log.Printf("bytedmysql: connect %s failed", ed.Addr)
		}

		return nil, err
	}

	return d.open(cfg, to)
}

// 通过 Dps Token 获取认证信息并进行连接
func (d Driver) openByToken(cfg *mysql.Config, dbPsm string) (driver.Conn, error) {
	// 授权之后，如果是部署在 TCE 的服务，Token 将由 TCE 注入，如果是部署在物理机的服务，Token 从 Dps Agent 获取。
	// 通过 Token，到 DBAuth 服务获取认证的元信息并注入 Cfg 中
	if err := interpolateAuthMeta(cfg, dbPsm); err != nil {
		return nil, err
	}

	return d.open(cfg, dbPsm)
}

// open a new wrapped Connection
func (d Driver) open(cfg *mysql.Config, to string) (driver.Conn, error) {
	wrappedConn, err := originDriver.Open(cfg.FormatDSN())
	if err != nil {
		return wrappedConn, err
	}
	return makeConn(wrappedConn, cfg, to)
}

func init() {
	// register bytedmysql driver
	sql.Register("bytedmysql", Driver{})
}
