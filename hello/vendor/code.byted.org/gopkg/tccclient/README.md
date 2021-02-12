## 平台

[动态配置中心](http://cloud.bytedance.net/tcc/all)


## 文档

[动态配置中心介绍](https://wiki.bytedance.net/pages/viewpage.action?pageId=219247405)


[TCCV2介绍](https://bytedance.feishu.cn/space/doc/doccnm4sZB8wmanRfqMEn4#)


[TCCV2升级文档](https://bytedance.feishu.cn/space/doc/doccnBPg6sPlygTihD5YmF#)


## USAGE

### 基础用法

```go
package main
 
import (
    "fmt"
    "context"
    "code.byted.org/gopkg/tccclient"
)
 
var (
    // 使用ClientV2必须升级TCC到V2版本，升级文档参考上述"TCCV2升级文档"
    client *tccclient.ClientV2
)
 
func init() {
    config := tccclient.NewConfigV2()
    config.Confspace = "default" //配置空间, 可不传，默认为default
    var err error
    client, err = tccclient.NewClientV2("toutiao.tcc.xxx", config)
    if err != nil {
        panic(err)
    }
}
 
func main() {
    ctx := context.Background() // 如果使用了框架，应该用框架的ctx
    value, err := client.Get(ctx, "test")
    // err == tccclient.ConfigNotFoundError
    fmt.Println(value, err)
}
```

### 带缓存的自定义解析 GetWithParser
#### 场景&作用
如果每次 Get 到 value 后，都要对 value 做解析（比如 json.Unmarshal ）, 在 qps 过高或者 value 过于复杂的情况下，会导致耗费在解析上的成本过高。使用该接口可以对解析后的结果进行缓存，只有在 value 有更新的情况下才会触发重新解析，避免无效的解析成本

#### 使用
1. 实现一个 TCCParser, 用来将 value 解析并将结果返回
```go
// value: get key 得到的 value
// err: get key 得到的 error， 比如为 ConfigNotFoundError
// cacheResult: 缓存在 SDK 中上一次解析的结果，用户可以根据 err 来判断是否继续用上一次解析的结果，或者根据业务需求做其他逻辑
type TCCParser func(value string, err error, cacheResult interface{}) (result interface{}, err error)
```

2. 使用 GetWithParser 获取解析后的结果
```go
result, err := client.GetWithParser(key, YourTCCParser)
```
处理逻辑:
1. 有缓存的 result
   + 获取 key 的 value 成功，检查无更新，将缓存的 result 返回
   + 获取 key 的 value 成功，检查有更新，则调用 TCCParser 进行解析
   + 获取 key 的 value 失败，调用 TCCParser，并传入参数 err 和 cacheResult
2. 无缓存的 result，获取 key 的 value，并调用 TCCParser 进行解析

对 TCCParser 返回值的处理：
  + err == nil，则将返回的 result 缓存
  + err != nil，则将err返回

#### 示例
以下为一个逻辑简单的TccParser示例
```go
const defaultValue = 1

func TCCParserDemo(value string, err error, cacheResult interface{}) (interface{}, error) {
	if err != nil {
		if cacheResult == nil { // first parse, or never parse success
			// do someting
			//  - maybe return default
			//  - maybe return err
			return defaultValue, nil
			//return nil, err
		}
		// do someting
		//  - maybe return cacheResult
		//  - maybe return default
		//  - maybe return err
		return cacheResult, nil
		// return defaultValue, nil
		// return nil, err
	}

	// parse value
	valueInt, ok := strconv.Atoi(value)
	if !ok {
		// do someting
		//  - maybe return cacheResult
		//  - maybe return default
		//  - maybe return err
		return cacheResult, nil
		// return defaultValue, nil
		// return nil, errors.New("invalid int")
	}
	return valueInt, nil
}
```

### 非实时监听器 Listener
#### 场景&作用
如果需要监听某个key的变更，并在key变更后做一些复杂操作，例如更新本地的一些配置或做一些本地数据处理操作，示例逻辑如下：
```go
  for {
       value = tccclient.get(key)
       if value updated {
            // do something
       }
       time.sleep(interval)
   }
```
这种场景下可以使用监听器-Listener，将 key 和对应的回调函数注册到 Listener, Listener 会定期查询对应 key 的 value 有没有发生变更，如果变更后会调用对应的回调函数

#### 使用
1. 实现一个 Callback, 用来对更新后的 value 做处理
```go
// value: 刷新 key 得到的 value
// err: 刷新 key 得到的 error， 比如为 ConfigNotFoundError
type Callback func(value string, err error)
```
2. 将要监听的 key 和对应的 callback 注册到 Listener
```go
client.AddListener(key, callback) // AddListener 时，先默认先调用一次 callback 做 listener 的初始化操作

// 如果需要先通过 client.Get 获取到 value 做一些初始化操作，然后再做监听，可以在 AddListener 时将当前 value 传入
// 如果传入了当前 value，AddListener 不会再调用一次 callback 做 listener 的初始化操作
client.AddListener(key, callback, WithCurrentValue(value))
```
处理逻辑:
1. 当 Listener 到达刷新间隔，刷新 key 获取到 value 或者 error
   + 如果 value 有更新，调用callback
   + 如果 error 与上次返回的 error 不同，调用callback
2. 如果多个 key 的配置发生变更，会串行调用 callback

刷新间隔可以调整, 如下：
```go
  config := tccclient.NewConfigV2()
  config.ListenInterval = 60 * 10 * time.Second  // minListenInterval = 60s
  client, err = tccclient.NewClientV2(serviceName, config)
```

#### 注意
1. Listener 不能实时监听到 key 的变更，当前配置变更感知延迟最低 60s
2. Listener 会一直轮询 key 的 value，会导致对TCC Server端的请求量过高造成过大压力，如果对配置的请求频率很低且处理逻辑不复杂，优先考虑使用 `GetWithParser` 接口