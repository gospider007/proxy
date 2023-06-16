# 功能概述
* 快如闪电的正向代理
* 支持http,https,socks五
* 支持隧道代理的开发
* 支持白名单，用户名密码
* 支持ja3,h2 指纹代理,使客户端隐藏自身ja3指纹
* 支持根据请求动态设置指纹
* 支持链式代理，设置下游代理
* 支持http,https,websocket,http2 抓包

#  一个端口同时实现 http,https,socks五 代理
```go
func main() {
	proCli, err := proxy.NewClient(nil, proxy.ClientOption{
		Port:    7006,
		DisVerify:true,//关闭白名单验证和密码验证，在没有白名单和密码的情况下如果不关闭，用不了
	})
	if err != nil {
		log.Panic(err)
	}
	log.Print(proCli.Addr())
	log.Panic(proCli.Run())
}
```
# 设置白名单
```go
func main() {
	proCli, err := proxy.NewClient(nil, proxy.ClientOption{
		Port:    7006,
        IpWhite: []net.IP{
			net.IPv4(192, 168, 1, 11),
		},
	})
	if err != nil {
		log.Panic(err)
	}
	log.Print(proCli.Addr())
	log.Panic(proCli.Run())
}
```
# 设置账号密码
```go
func main() {
	proCli, err := proxy.NewClient(nil, proxy.ClientOption{
		Port:    7006,
       	Usr:     "admin",
		Pwd:     "password",
	})
	if err != nil {
		log.Panic(err)
	}
	log.Print(proCli.Addr())
	log.Panic(proCli.Run())
}
```
## ja3指纹开关
```go
func main() {
	proCli, err := proxy.NewClient(nil, proxy.ClientOption{
		Port: 7006,
		Usr:  "admin",
		Pwd:  "password",
		Ja3:true,//开启ja3指纹
	})
	if err != nil {
		log.Panic(err)
	}
	log.Print(proCli.Addr())
	log.Panic(proCli.Run())
}
```
## 指定ja3指纹
```go
func main() {
	spec, err := ja3.CreateSpecWithStr("771,4865-4866-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,35-45-43-5-17513-16-23-27-11-0-18-65281-13-51-10-21,29-23-24,0")
	if err != nil {
		log.Panic(err)
	}
	proCli, err := proxy.NewClient(nil, proxy.ClientOption{
		Port:    7006,
		Usr:     "admin",
		Pwd:     "password",
		Ja3Spec: spec,
	})
	if err != nil {
		log.Panic(err)
	}
	log.Print(proCli.Addr())
	log.Panic(proCli.Run())
}
```

## h2指纹开关
```go
func main() {
	proCli, err := proxy.NewClient(nil, proxy.ClientOption{
		Port: 7006,
		DisVerify : true,
		H2Ja3 : true,
	})
	if err != nil {
		log.Panic(err)
	}
	log.Print(proCli.Addr())
	log.Panic(proCli.Run())
}
```

## h2指纹自定义
```go
func main() {
	proCli, err := proxy.NewClient(nil, proxy.ClientOption{
		Port: 7006,
		DisVerify:true,
		H2Ja3Spec:ja3.H2Ja3Spec{
		InitialSetting: []ja3.Setting{
			{Id: 1, Val: 65555},
			{Id: 2, Val: 1},
			{Id: 3, Val: 2000},
			{Id: 4, Val: 6291457},
			{Id: 6, Val: 262145},
		},
		ConnFlow: 15663106,
		OrderHeaders: []string{
			":method",
			":path",
			":scheme",
			":authority",
		},
	},
	})
	if err != nil {
		log.Panic(err)
	}
	log.Print(proCli.Addr())
	log.Panic(proCli.Run())
}
```