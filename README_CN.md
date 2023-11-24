# config-apollo 

[English](README.md)

使用 **apollo** 作为 **Kitex** 的配置中心

##  这个项目应当如何使用?

### 基本使用

#### 服务端

```go
package main

import (
	"context"
	"log"
	"net"

	"github.com/cloudwego/kitex-examples/kitex_gen/api"
	"github.com/cloudwego/kitex-examples/kitex_gen/api/echo"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/server"
	"github.com/kitex-contrib/config-apollo/apollo"
	apolloserver "github.com/kitex-contrib/config-apollo/server"
	"github.com/kitex-contrib/config-apollo/utils"
)

type configLog struct{}

func (cl *configLog) Apply(opt *utils.Options) {
	fn := func(cp *apollo.ConfigParam) {
		klog.Infof("apollo config %v", cp)
	}
	opt.ApolloCustomFunctions = append(opt.ApolloCustomFunctions, fn)
}

var _ api.Echo = &EchoImpl{}

// EchoImpl implements the last service interface defined in the IDL.
type EchoImpl struct{}

// Echo implements the Echo interface.
func (s *EchoImpl) Echo(ctx context.Context, req *api.Request) (resp *api.Response, err error) {
	klog.Info("echo called")
	return &api.Response{Message: req.Message}, nil
}

func main() {
	klog.SetLevel(klog.LevelDebug)
	apolloClient, err := apollo.NewClient(apollo.Options{})
	if err != nil {
		panic(err)
	}
	serviceName := "ServiceName"
	addr, err := net.ResolveTCPAddr("tcp", "localhost:8899")
	if err != nil {
		panic(err)
	}

	cl := &configLog{}

	svr := echo.NewServer(
		new(EchoImpl),
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: serviceName}),
		server.WithSuite(apolloserver.NewSuite(serviceName, apolloClient, cl)),
		server.WithServiceAddr(addr),
	)
	if err := svr.Run(); err != nil {
		log.Println("server stopped with error:", err)
	} else {
		log.Println("server stopped")
	}
}

```

#### 客户端

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/cloudwego/kitex-examples/kitex_gen/api"
	"github.com/cloudwego/kitex-examples/kitex_gen/api/echo"
	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/kitex-contrib/config-apollo/apollo"
	apolloclient "github.com/kitex-contrib/config-apollo/client"
	"github.com/kitex-contrib/config-apollo/utils"
)

type configLog struct{}

func (cl *configLog) Apply(opt *utils.Options) {
	fn := func(cp *apollo.ConfigParam) {
		klog.Infof("apollo config %v", cp)
	}
	opt.ApolloCustomFunctions = append(opt.ApolloCustomFunctions, fn)
}

func main() {
	klog.SetLevel(klog.LevelDebug)

	apolloClient, err := apollo.NewClient(apollo.Options{})
	if err != nil {
		panic(err)
	}
	cl := &configLog{}

	serviceName := "ServiceName"
	clientName := "ClientName"
	client, err := echo.NewClient(
		serviceName,
		client.WithHostPorts("0.0.0.0:8899"),
		client.WithSuite(apolloclient.NewSuite(serviceName, clientName, apolloClient, cl)),
	)
	if err != nil {
		log.Fatal(err)
	}
	for {
		req := &api.Request{Message: "my request"}
		resp, err := client.Echo(context.Background(), req)
		if err != nil {
			klog.Errorf("take request error: %v", err)
		} else {
			klog.Infof("receive response %v", resp)
		}
		time.Sleep(time.Second * 10)
	}
}
```
### Apollo 配置

根据 Options 的参数初始化 client，建立链接之后 suite 会根据`AppId` `NameSpace` 以及 `ServiceName` 或者 `ClientName` 订阅对应的配置并动态更新自身策略，具体参数参考下面Options。 

配置的格式默认支持 `json` ,可以使用函数 [SetParser](https://github.com/kitex-contrib/config-nacos/blob/eb006978517678dd75a81513142d3faed6a66f8d/nacos/nacos.go#L68) 进行自定义格式解析方式，并在 `NewSuite` 的时候使用 实现了`Option` 接口的实例中的字段函数修改订阅函数的格式。

#### CustomFunction

允许用户自定义 apollo 的参数. 

#### Options 默认值

| 参数 | 变量默认值 | 作用 |
| :------------------------ | :--------------------------------: | --------------------------------- |
| ConfigServerURL | 127.0.0.1:8080                     | apollo config service 地址 |
| AppID            | KitexApp | apollo 的 appid |
| ClientKeyFormat | {{.ClientServiceName}}.{{.ServerServiceName}}  | 使用 go [template](https://pkg.go.dev/text/template) 语法渲染生成对应的 ID, 使用 `ClientServiceName` `ServiceName` 两个元数据 |
| ServerKeyFormat | {{.ServerServiceName}}.{{.Category}}  | 使用 go [template](https://pkg.go.dev/text/template) 语法渲染生成对应的 ID, 使用 `ServiceName` ` 单个元数据         |
| cluster             | default                      | 使用固定值，也可以动态渲染，用法同 KeyFormat  |

#### 治理策略

下面例子中的 namespace 使用策略固定值， APPID 以及 Cluster 均使用默认值，服务名称为 ServiceName，客户端名称为 ClientName

##### 限流 Category=limit
> 限流目前只支持服务端，所以 ClientServiceName 为空。

[JSON Schema](https://github.com/cloudwego/kitex/blob/develop/pkg/limiter/item_limiter.go#L33)

|字段|说明|
|----|----|
|connection_limit|最大并发数量| 
|qps_limit|每 100ms 内的最大请求数量| 

例子：
```
namespace: `limit`
key: `ServiceName`
{
  "connection_limit": 100, 
  "qps_limit": 2000        
}
```
注：

- 限流配置的粒度是 Server 全局，不分 client、method
- 「未配置」或「取值为 0」表示不开启
- connection_limit 和 qps_limit 可以独立配置，例如 connection_limit = 100, qps_limit = 0

##### 重试 Category=retry

[JSON Schema](https://github.com/cloudwego/kitex/blob/develop/pkg/retry/policy.go#L63)

|参数|说明|
|----|----|
|type| 0: failure_policy 1: backup_policy| 
|failure_policy.backoff_policy| 可以设置的策略： `fixed` `none` `random` | 

例子：
```
namespace: `retry`
key: `ClientName.ServiceName`
{
    "*": {  
        "enable": true,
        "type": 0,                 
        "failure_policy": {
            "stop_policy": {
                "max_retry_times": 3,
                "max_duration_ms": 2000,
                "cb_policy": {
                    "error_rate": 0.5
                }
            },
            "backoff_policy": {
                "backoff_type": "fixed", 
                "cfg_items": {
                    "fix_ms": 50
                }
            },
            "retry_same_node": false
        }
    },
    "echo": { 
        "enable": true,
        "type": 1,                 
        "backup_policy": {
            "retry_delay_ms": 100,
            "retry_same_node": false,
            "stop_policy": {
                "max_retry_times": 2,
                "max_duration_ms": 300,
                "cb_policy": {
                    "error_rate": 0.2
                }
            }
        }
    }
}
```
注：retry.Container 内置支持用 * 通配符指定默认配置（详见 [getRetryer](https://github.com/cloudwego/kitex/blob/v0.5.1/pkg/retry/retryer.go#L240) 方法）

##### 超时 Category=rpc_timeout

[JSON Schema](https://github.com/cloudwego/kitex/blob/develop/pkg/rpctimeout/item_rpc_timeout.go#L42)

例子：
```
namespace: `rpc_timeout`
key: `ClientName.ServiceName`
{
  "*": {
    "conn_timeout_ms": 100, 
    "rpc_timeout_ms": 3000
  },
  "echo": {
    "conn_timeout_ms": 50,
    "rpc_timeout_ms": 1000
  }
}
```
注：kitex 的熔断实现目前不支持修改全局默认配置（详见 [initServiceCB](https://github.com/cloudwego/kitex/blob/v0.5.1/pkg/circuitbreak/cbsuite.go#L195)）

##### 熔断: Category=circuit_break

[JSON Schema](https://github.com/cloudwego/kitex/blob/develop/pkg/circuitbreak/item_circuit_breaker.go#L30)

|参数|说明|
|----|----|
|min_sample| 最小的统计样本数| 
例子：
```
echo 方法使用下面的配置（0.3、100），其他方法使用全局默认配置（0.5、200）
namespace: `circuit_break`
key: `ClientName.ServiceName`
{
  "echo": {
    "enable": true,
    "err_rate": 0.3, 
    "min_sample": 100 
  }
}
```

### 更多信息

更多示例请参考 [example](github.com/kitex-contrib/examples/config/apollo)

主要贡献者： [xz](https://github.com/XZ0730)
