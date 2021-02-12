### 说明
id generator golang client

wiki: https://bytedance.feishu.cn/docs/doccn1l3tukEKeLuzPXG1QCCBNh#

目前idgenerator新增v2版本，移除了很多历史问题，重新设计并简化了API，只支持64bit client和Go mod，欢迎新业务尝试使用 [v2-readme](https://code.byted.org/gopkg/idgenerator/blob/master/v2)

### 注意
namespace, countspace 需要申请，参考wiki

### 例子

`InitIdGeneratorWrapper接口 和 InitIdGeneratorClient 接口已经被废弃，请升级到以下新接口`<br/>
`之前的接口国内默认生成52位id，国外默认生成64位id，新接口明确了要生成的id位数，升级前请明确现在使用的哪种id`

    import (
        "fmt"
        "code.byted.org/gopkg/idgenerator"
    )

    func main() {
        // 本地缓存批量请求到的id，对外通过Get接口获取单个id，推荐使用该方式
        // 生成64位id: New64BitIdGeneratorWrapper, 生成52位id: New52BitIdGeneratorWrapper
        wrapper := idgenerator.New64BitIdGeneratorWrapper("xxx", "yyy", 10) // namespace, countspace, count
        for i := 0; i < 15; i++ {
            id, err := wrapper.Get()
            fmt.Printf("id= %v err=%v\n", id, err)
        }

        // 通过GenMulti获取，每次都发起网络请求，本地不会缓存
        // 生成64位id: New64BitIdGeneratorClient, 生成52位id: New52BitIdGeneratorClient
        ig := idgenerator.New64BitIdGeneratorClient("xxx", "yyy") // namespace, countspace
        id, err := ig.Gen()
        if err == nil {
            fmt.Println(id)
        }
        ids, err := ig.GenMulti(3)
        if err == nil {
            fmt.Println(ids)
        }
    }


### 安装

    go get "code.byted.org/gopkg/idgenerator"


