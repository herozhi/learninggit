
metainfo
========

该库的目的是提供一种不同于 RPC 框架中的 base 的手段来在调用链上传递某些元信息的方法，并且规范这个过程。

信息分为两类，transient 和 persistent：前者只会传递一跳，从客户端传递到下游，然后消失；后者需要在整个调用链上一直传递，直到被丢弃。

由于传递过程使用了 go 语言的 context，因此为了避免服务端的 context 直接传递到下游的 client 时造成 transient 数据透传，需要引入一个中间态：transient-upstream，以和客户端自己设置的 transient 数据作区分。

元信息在网络上的传输，该库提供了一套信息以什么形式存在于请求载体中的约定，但是最终传输过程由各个基础框架来实现，一般将信息放在请求header或这个base等字段中。

API 参考
-------

**注意**

1. 出于兼容性和普适性，元信息的形式为字符串的 key value 对。
2. 空串作为 key 或者 value 都是无效的。
3. 由于 context 的特性，程序对 metainfo 的增删改只会对拥有相同的 contetxt 或者其子 context 的代码可见。

**常量**

metainfo 包提供了几个常量字符串前缀，用于无 context（例如网络传输）的场景下标记元信息的类型。

典型的业务代码通常不需要用到这些前缀。

- `PrefixPersistent`
- `PrefixTransient`
- `PrefixTransientUpstream`

**方法**

- `TransferForward(ctx context.Context) context.Context`
    - 向前传递，用于将上游传来的 transient 数据转化为 transient-upstream 数据，并过滤掉原有的 transient-upstream 数据。
- `GetValue(ctx context.Context, k string) (string, bool)`
    - 从 context 里获取指定 key 的 transient 数据（包括 transient-upstream 数据）。
- `GetAllValues(ctx context.Context) map[string]string`
    - 从 context 里获取所有 transient 数据（包括 transient-upstream 数据）。
- `WithValue(ctx context.Context, k string, v string) context.Context`
    - 向 context 里添加一个 transient 数据。
- `DelValue(ctx context.Context, k string) context.Context`
    - 从 context 里删除指定的 transient 数据。
- `GetPersistentValue(ctx context.Context, k string) (string, bool)`
    - 从 context 里获取指定 key 的 persistent 数据。
- `GetAllPersistentValues(ctx context.Context) map[string]string`
    - 从 context 里获取所有 persistent 数据。
- `WithPersistentValue(ctx context.Context, k string, v string) context.Context`
    - 向 context 里添加一个 persistent 数据。
- `DelPersistentValue(ctx context.Context, k string) context.Context`
    - 从 context 里删除指定的 persistent 数据。

**传输**

- 元信息通过RPC请求传递
    - `SaveMetaInfoToMap(ctx context.Context, m map[string]string)`
        - 将元信息设置到目标map中（map可以通过请求传递，见kitc相关的MiddleWare）
    - `SetMetaInfoFromMap(ctx context.Context, m map[string]string) context.Context`
        - 从map中解析出元信息（map为请求携带的，见kite相关的MiddleWare）
- 元信息通过HTTP请求传递
    - `SetMetaInfoToHeader(ctx context.Context, setter HeaderSetter)`
        - 将元信息设置到目标HTTP请求中
    - `SetMetaInfoFromHeader(ctx context.Context, header WithHeader) context.Context`
        - 从HTTP请求中解析出元信息

其他语言版本
------------

- Python
    - [Euler 框架的 Context](http://python.byted.org/euler.html#euler.context.Context)

...

