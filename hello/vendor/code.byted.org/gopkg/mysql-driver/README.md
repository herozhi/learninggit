# Mysql Driver for Toutiao
## 使用
import包过后, 该driver会被注册成"mysql2";

所以在Open时, 将driver名字设置为"mysql2"即可, 如下:

```
import (
	"code.byted.org/gopkg/gorm"
	mysqldriver "code.byted.org/gopkg/mysql-driver"
)

func main() {
    db, _ := gorm.Open("mysql2", "Your DSN")
}
```

## 特性
### consul动态解析
传入的DSN中, 不必直接指定实例的地址, 直接传入consul name即可, 如:

```
db, _ := gorm.Open("mysql2", "USERNAME:PASSWORD@tcp(consul:toutiao.mysql.ms_data_write)/DATABASE")
```

### 服务化metrics
会自动化的打出服务化的metrics, 具体请看toutiao_metrics.go;

### 打印slow sql log
可设置阈值, 当sql的调用花费超过该阈值时, 自动打出慢sql语句;

使用方法如下:

```
import (
	mysqldriver "code.byted.org/gopkg/mysql-driver"
)

func init() {
	mysqldriver.SetPSMCluster("P.S.M", "default")
	err := mysqldriver.OpenSlowSQLLog(time.Millisecond*500, &mysqlTraceLogger{})
}
```

### sql信息注入
打开sql信息注入后, 会将本机IP和当前服务的PSM作为注释, 注入到sql中;

方便出现mysql后端根据慢sql, 反推服务;

```
import (
	mysqldriver "code.byted.org/gopkg/mysql-driver"
)

func init() {
	mysqldriver.OpenInterpolation("P.S.M")
}
```