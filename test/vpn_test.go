package main

import (
	"flag"
	"log"
	"os"
	"testing"

	"github.com/gospider007/proxy"
)

func pubVpn(domain, email, user, password string, addr string) {
	cli, err := proxy.NewClient(nil, proxy.ClientOption{
		Addr:       addr,
		Usr:        user,
		Pwd:        password,
		AcmeDomain: domain,
		AcmeEmail:  email,
	})
	if err != nil {
		log.Panic(err)
	}
	log.Print(cli.Addr())
	log.Print(cli.Run())
}
func vpnMain() {
	addr := flag.String("addr", "", "监听的地址(必填)")
	user := flag.String("usr", "", "用户名(必填)")
	password := flag.String("pwd", "", "密码(必填)")
	domain := flag.String("domain", "", "域名")
	email := flag.String("email", "", "邮箱")
	flag.Parse()
	if *user == "" || *password == "" || *addr == "" {
		log.Print("参数错误：\n -h 查看命令行参数")
		os.Exit(0)
	}
	pubVpn(*domain, *email, *user, *password, *addr)
}

func TestVpn(t *testing.T) {
	vpnMain()
}
