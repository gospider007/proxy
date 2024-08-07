package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"testing"

	"github.com/gospider007/proxy"
)

func changeReq(r *http.Request, openaiKey string) {
	r.Host = "api.openai.com"
	r.URL.Host = "api.openai.com:443"
	r.URL.User = nil
	r.Header.Set("Host", "api.openai.com:443")
	if openaiKey != "" {
		r.Header.Set("Authorization", "Bearer "+openaiKey)
	}
	r.URL.Scheme = "https"
}
func openaiProxy() {
	usrFlag := flag.String("usr", "", "用户名(必填)")
	pwdFlag := flag.String("pwd", "", "密码(必填)")
	addrFlag := flag.String("addr", "", "监听地址(必填)")
	openaiKeyFlag := flag.String("openaiKey", "", "openaiKey(强制覆盖) (必填)")

	proxyFlag := flag.String("proxy", "", "代理地址")
	domainFlag := flag.String("domain", "", "域名")
	flag.Parse()
	if *usrFlag != "" || *pwdFlag != "" || *pwdFlag == "" || *openaiKeyFlag == "" {
		log.Print("参数错误：\n -h 查看命令行参数")
		os.Exit(0)
	}
	var domainNames []string
	if *domainFlag != "" {
		domainNames = []string{*domainFlag}
	}
	proxyCli, err := proxy.NewClient(nil, proxy.ClientOption{
		Addr:        *addrFlag,
		Usr:         *usrFlag,
		Pwd:         *pwdFlag,
		Proxy:       *proxyFlag,
		DomainNames: domainNames,
		HttpConnectCallBack: func(r *http.Request) error {
			changeReq(r, *openaiKeyFlag)
			return nil
		},
		RequestCallBack: func(r *http.Request, res *http.Response) error {
			if res == nil {
				changeReq(r, *openaiKeyFlag)
			}
			return nil
		},
	})
	if err != nil {
		log.Panic(err)
	}
	log.Print(proxyCli.Addr())
	log.Print(proxyCli.Run())
}

func TestOpenAiProxy(t *testing.T) {
	openaiProxy()
}
