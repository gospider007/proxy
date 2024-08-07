package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"testing"

	"github.com/gospider007/gtls"
	"github.com/gospider007/proxy"
)

func TestCdp(t *testing.T) {
	var proxyHost string
	for _, addr := range gtls.GetHosts(4) {
		if addr.IsGlobalUnicast() {
			proxyHost = addr.String()
			break
		}
	}
	if proxyHost == "" {
		log.Panic("获取内网地址失败")
	}
	proxyPort := 7865
	proxyClient, err := proxy.NewClient(nil, proxy.ClientOption{
		Addr:      net.JoinHostPort(proxyHost, strconv.Itoa(proxyPort)),
		DisVerify: true,
		HttpConnectCallBack: func(r *http.Request) error {
			r.Host = fmt.Sprintf("127.0.0.1:%d", proxyPort)
			r.Header.Del("Origin")
			return nil
		},
	})
	if err != nil {
		log.Panic(err)
	}
	log.Print(proxyClient.Addr())
	// go proxyClient.Run()
}
