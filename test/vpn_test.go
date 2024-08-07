package main

import (
	"flag"
	"log"
	"os"
	"testing"

	"github.com/gospider007/proxy"
)

func vpnMain() {
	addr := flag.String("addr", "", "监听的地址(必填)")
	user := flag.String("usr", "", "用户名(必填)")
	password := flag.String("pwd", "", "密码(必填)")
	domain := flag.String("domain", "", "域名")
	flag.Parse()
	if *user == "" || *password == "" || *addr == "" {
		log.Print("参数错误：\n -h 查看命令行参数")
		os.Exit(0)
	}
	var domainNames []string
	if *domain != "" {
		domainNames = []string{*domain}
	}
	cli, err := proxy.NewClient(nil, proxy.ClientOption{
		Addr:        *addr,
		Usr:         *user,
		Pwd:         *password,
		DomainNames: domainNames,
	})
	if err != nil {
		log.Panic(err)
	}
	log.Print(cli.Addr())
	log.Print(cli.Run())
}

func TestVpn(t *testing.T) {
	vpnMain()
}
