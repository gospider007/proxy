package main

import (
	"log"
	"testing"
	"time"

	"github.com/gospider007/proxy"
	"github.com/gospider007/requests"
)

func TestProxy(t *testing.T) {
	proCli, err := proxy.NewClient(nil, proxy.ClientOption{
		DisVerify: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer proCli.Close()
	go proCli.Run()
	proxyIp := proCli.Addr()
	reqCli, err := requests.NewClient(nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := reqCli.Request(nil, "get", "http://myip.top", requests.RequestOption{
		ClientOption: requests.ClientOption{
			Proxy: "http://" + proxyIp,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if jsonData, _ := resp.Json(); jsonData.Get("ip").String() == "" {
		t.Fatal("代理bug")
	}
	resp, err = reqCli.Request(nil, "get", "http://myip.top", requests.RequestOption{
		ClientOption: requests.ClientOption{
			Proxy: "https://" + proxyIp,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if jsonData, _ := resp.Json(); jsonData.Get("ip").String() == "" {
		t.Fatal("代理bug")
	}
	resp, err = reqCli.Request(nil, "get", "http://myip.top", requests.RequestOption{
		ClientOption: requests.ClientOption{
			Proxy: "socks5://" + proxyIp,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if jsonData, _ := resp.Json(); jsonData.Get("ip").String() == "" {
		t.Fatal("代理bug")
	}
}

func TestProxy2(t *testing.T) {
	proCliPre, err := proxy.NewClient(nil, proxy.ClientOption{
		Usr: "gospider",
		Pwd: "gospider123456789",
		// DisVerify: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer proCliPre.Close()
	go proCliPre.Run()
	proIp := proCliPre.Addr()

	proCli, err := proxy.NewClient(nil, proxy.ClientOption{
		Proxy:     "https://gospider:gospider123456789@" + proIp,
		DisVerify: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer proCli.Close()
	go proCli.Run()
	proxyIp := proCli.Addr()
	reqCli, err := requests.NewClient(nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := reqCli.Request(nil, "get", "https://httpbin.org/ip", requests.RequestOption{
		ClientOption: requests.ClientOption{

			Proxy: "http://" + proxyIp,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if jsonData, _ := resp.Json(); jsonData.Get("origin").String() == "" {
		t.Fatal("代理bug")
	}
	resp, err = reqCli.Request(nil, "get", "https://httpbin.org/ip", requests.RequestOption{
		ClientOption: requests.ClientOption{
			Proxy: "https://" + proxyIp,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if jsonData, _ := resp.Json(); jsonData.Get("origin").String() == "" {
		t.Fatal("代理bug")
	}
	resp, err = reqCli.Request(nil, "get", "https://httpbin.org/ip", requests.RequestOption{
		ClientOption: requests.ClientOption{
			Proxy: "socks5://" + proxyIp,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if jsonData, _ := resp.Json(); jsonData.Get("origin").String() == "" {
		t.Fatal("代理bug")
	}
}

func TestProxyJa3(t *testing.T) {
	proCli, err := proxy.NewClient(nil, proxy.ClientOption{
		Ja3: true,
		Usr: "admin",
		Pwd: "password",
		// DisVerify: true, //关闭白名单验证和密码验证，在没有白名单和密码的情况下如果不关闭，用不了

	})
	if err != nil {
		t.Fatal(err)
	}
	defer proCli.Close()
	go proCli.Run()
	proxyIp := proCli.Addr()
	reqCli, err := requests.NewClient(nil, requests.ClientOption{})
	if err != nil {
		t.Fatal(err)
	}
	log.Print("ddd0")
	// reqCli.MaxRetries = 2
	// resp, err := reqCli.Request(nil, "get", "https://tools.scrapfly.io/api/fp/ja3?extended=1", requests.RequestOption{})
	resp, err := reqCli.Request(nil, "get", "https://tools.scrapfly.io/api/fp/ja3?extended=1", requests.RequestOption{
		ClientOption: requests.ClientOption{
			Proxy: "http://admin:password@" + proxyIp,
		},
	})
	// resp, err := reqCli.Request(nil, "get", "https://httpbin.org/ip", requests.RequestOption{ClientOption:requests.ClientOption{Proxy: "http://admin:password@" + proxyIp}})
	if err != nil {
		t.Fatal(err)
	}
	log.Print("ddd")
	jsonData, _ := resp.Json()
	if jsonData.Get("digest").String() == "" || jsonData.Get("digest").String() == "4e8e7b3f6585690ad91147bb2f5ad681" {
		log.Print(resp.Text())
		t.Fatal("代理bug")
	}
	log.Print("ddd3")
	resp, err = reqCli.Request(nil, "get", "https://tools.scrapfly.io/api/fp/ja3?extended=1", requests.RequestOption{
		ClientOption: requests.ClientOption{

			Proxy: "https://admin:password@" + proxyIp,
		},
	})
	// resp, err = reqCli.Request(nil, "get", "http://myip.top", requests.RequestOption{ClientOption:requests.ClientOption{Proxy: "https://admin:password@" + proxyI}p})
	if err != nil {
		time.Sleep(time.Second * 2)
		t.Fatal(err)
	}
	jsonData, _ = resp.Json()
	if jsonData.Get("digest").String() == "" || jsonData.Get("digest").String() == "4e8e7b3f6585690ad91147bb2f5ad681" {
		log.Print(resp.Text())
		t.Fatal("代理bug")
	}
	log.Print("ddd4")
	resp, err = reqCli.Request(nil, "get", "https://tools.scrapfly.io/api/fp/ja3?extended=1", requests.RequestOption{
		ClientOption: requests.ClientOption{

			Proxy: "socks5://admin:password@" + proxyIp,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	jsonData, _ = resp.Json()
	if jsonData.Get("digest").String() == "" || jsonData.Get("digest").String() == "4e8e7b3f6585690ad91147bb2f5ad681" {
		log.Print(resp.Text())
		t.Fatal("代理bug")
	}
}
func TestProxyH2Ja3(t *testing.T) {
	proCli, err := proxy.NewClient(nil, proxy.ClientOption{
		Ja3:       true,
		H2Ja3:     true,
		DisVerify: true, //关闭白名单验证和密码验证，在没有白名单和密码的情况下如果不关闭，用不了

	})
	if err != nil {
		t.Fatal(err)
	}
	defer proCli.Close()
	go proCli.Run()
	proxyIp := proCli.Addr()
	reqCli, err := requests.NewClient(nil, requests.ClientOption{})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := reqCli.Request(nil, "get", "https://tools.scrapfly.io/api/fp/ja3?extended=1", requests.RequestOption{
		ClientOption: requests.ClientOption{
			Proxy: "http://admin:password@" + proxyIp,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	jsonData, _ := resp.Json()
	if jsonData.Get("digest").String() == "" || jsonData.Get("digest").String() == "4e8e7b3f6585690ad91147bb2f5ad681" {
		log.Print(resp.Text())
		t.Fatal("代理bug")
	}
	resp, err = reqCli.Request(nil, "get", "https://tools.scrapfly.io/api/fp/ja3?extended=1", requests.RequestOption{
		ClientOption: requests.ClientOption{
			Proxy: "https://admin:password@" + proxyIp,
		},
	})
	// resp, err = reqCli.Request(nil, "get", "http://myip.top", requests.RequestOption{ClientOption:requests.ClientOption{Proxy: "https://admin:password@" + proxyI}p})
	if err != nil {
		t.Fatal(err)
	}
	jsonData, _ = resp.Json()
	if jsonData.Get("digest").String() == "" || jsonData.Get("digest").String() == "4e8e7b3f6585690ad91147bb2f5ad681" {
		log.Print(resp.Text())
		t.Fatal("代理bug")
	}
	resp, err = reqCli.Request(nil, "get", "https://tools.scrapfly.io/api/fp/ja3?extended=1", requests.RequestOption{
		ClientOption: requests.ClientOption{

			Proxy: "socks5://admin:password@" + proxyIp,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	jsonData, _ = resp.Json()
	if jsonData.Get("digest").String() == "" || jsonData.Get("digest").String() == "4e8e7b3f6585690ad91147bb2f5ad681" {
		log.Print(resp.Text())
		t.Fatal("代理bug")
	}
}
func TestProxyAuth(t *testing.T) {
	proCli, err := proxy.NewClient(nil, proxy.ClientOption{
		Usr: "admin",
		Pwd: "password",
		Ja3: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer proCli.Close()
	go proCli.Run()
	proxyIp := proCli.Addr()
	reqCli, err := requests.NewClient(nil, requests.ClientOption{
		MaxRetries: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	var resp *requests.Response
	resp, err = reqCli.Request(nil, "get", "https://tools.scrapfly.io/api/fp/ja3?extended=1", requests.RequestOption{ClientOption: requests.ClientOption{Proxy: "http://admin:password@" + proxyIp}})
	// resp, err := reqCli.Request(nil, "get", "https://httpbin.org/ip", requests.RequestOption{ClientOption:requests.ClientOption{Proxy: "http://admin:password@" + proxyIp}})
	if err != nil {
		t.Fatal(err)
	}
	jsonData, _ := resp.Json()
	if jsonData.Get("digest").String() == "" || jsonData.Get("digest").String() == "4e8e7b3f6585690ad91147bb2f5ad681" {
		t.Fatal("代理bug")
	}
	resp, err = reqCli.Request(nil, "get", "https://tools.scrapfly.io/api/fp/ja3?extended=1", requests.RequestOption{ClientOption: requests.ClientOption{Proxy: "https://admin:password@" + proxyIp}})
	if err != nil {
		time.Sleep(time.Second * 2)
		t.Fatal(err)
	}
	jsonData, _ = resp.Json()
	if jsonData.Get("digest").String() == "" || jsonData.Get("digest").String() == "4e8e7b3f6585690ad91147bb2f5ad681" {
		t.Fatal("代理bug")
	}
	resp, err = reqCli.Request(nil, "get", "https://tools.scrapfly.io/api/fp/ja3?extended=1", requests.RequestOption{ClientOption: requests.ClientOption{Proxy: "socks5://admin:password@" + proxyIp}})
	if err != nil {
		t.Fatal(err)
	}
	jsonData, _ = resp.Json()
	if jsonData.Get("digest").String() == "" || jsonData.Get("digest").String() == "4e8e7b3f6585690ad91147bb2f5ad681" {
		t.Fatal("代理bug")
	}
}

func TestProxyJa32(t *testing.T) {
	proCli, err := proxy.NewClient(nil, proxy.ClientOption{
		Ja3:       true,
		DisVerify: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer proCli.Close()
	go proCli.Run()
	proxyIp := proCli.Addr()
	reqCli, err := requests.NewClient(nil, requests.ClientOption{Proxy: "http://" + proxyIp})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := reqCli.Request(nil, "get", "https://tools.scrapfly.io/api/fp/ja3?extended=1")
	if err != nil {
		t.Fatal(err)
	}
	jsonData, _ := resp.Json()
	if jsonData.Get("digest").String() == "" || jsonData.Get("digest").String() == "4e8e7b3f6585690ad91147bb2f5ad681" {
		t.Fatal("代理bug")
	}

	reqCli, err = requests.NewClient(nil, requests.ClientOption{Proxy: "https://" + proxyIp})
	if err != nil {
		t.Fatal(err)
	}
	resp, err = reqCli.Request(nil, "get", "https://tools.scrapfly.io/api/fp/ja3?extended=1")
	// resp, err = reqCli.Request(nil, "get", "http://myip.top", requests.RequestOption{ClientOption:requests.ClientOption{Proxy: "https://" + proxyIp})
	if err != nil {
		time.Sleep(time.Second * 2)
		t.Fatal(err)
	}
	jsonData, _ = resp.Json()
	if jsonData.Get("digest").String() == "" || jsonData.Get("digest").String() == "4e8e7b3f6585690ad91147bb2f5ad681" {
		t.Fatal("代理bug")
	}
	reqCli, err = requests.NewClient(nil, requests.ClientOption{Proxy: "socks5://" + proxyIp})
	if err != nil {
		t.Fatal(err)
	}
	resp, err = reqCli.Request(nil, "get", "https://tools.scrapfly.io/api/fp/ja3?extended=1")
	if err != nil {
		t.Fatal(err)
	}
	jsonData, _ = resp.Json()
	if jsonData.Get("digest").String() == "" || jsonData.Get("digest").String() == "4e8e7b3f6585690ad91147bb2f5ad681" {
		log.Print(resp.Text())
		t.Fatal("代理bug")
	}
}
