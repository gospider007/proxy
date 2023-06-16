package proxy

import (
	"bufio"
	"context"
	"crypto/tls"
	_ "embed"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"

	"gitee.com/baixudong/gospider/ja3"
	"gitee.com/baixudong/gospider/kinds"
	"gitee.com/baixudong/gospider/requests"
	"gitee.com/baixudong/gospider/tools"
	"gitee.com/baixudong/gospider/websocket"
)

type ClientOption struct {
	ProxyJa3     bool                //连接代理时是否开启ja3
	ProxyJa3Spec ja3.ClientHelloSpec //连接代理时指定ja3Spec,//指定ja3Spec,使用ja3.CreateSpecWithStr 或者ja3.CreateSpecWithId 生成
	Usr          string              //用户名
	Pwd          string              //密码
	IpWhite      []net.IP            //白名单 192.168.1.1,192.168.1.2
	Port         int                 //代理端口
	Host         string              //代理host
	CrtFile      []byte              //公钥,根证书
	KeyFile      []byte              //私钥

	TLSHandshakeTimeout int64                                                   //tls 握手超时时间
	DnsCacheTime        int64                                                   //dns 缓存时间
	GetProxy            func(ctx context.Context, url *url.URL) (string, error) //代理ip http://116.62.55.139:8888
	Proxy               string                                                  //代理ip http://192.168.1.50:8888
	KeepAlive           int64                                                   //保活时间
	LocalAddr           string                                                  //本地网卡出口
	ServerName          string                                                  //https 域名或ip
	Vpn                 bool                                                    //是否是vpn
	Dns                 string                                                  //dns
	AddrType            requests.AddrType                                       //host优先解析的类型
	GetAddrType         func(string) requests.AddrType                          //控制host优先解析的类型

	Debug     bool //是否打印debug
	DisVerify bool //关闭验证
	//接收response 回调，返回error,则中断请求
	ResponseCallBack func(*http.Request, *http.Response) error
	//websocket 传输回调，返回error,则中断请求
	WsCallBack func(websocket.MessageType, []byte, WsType) error
	//发送request 回调，返回error,则中断请求
	RequestCallBack func(*http.Request) error
	//http,https 代理根据请求验证用户权限，返回error 则中断请求
	VerifyAuthWithHttp func(*http.Request) error
	//支持根据http,https代理的请求，动态生成ja3,h2指纹。注意这请求是客户端和代理协议协商的请求，不是客户端请求目标地址的请求
	//返回空结构体，则不会设置指纹
	CreateSpecWithHttp func(*http.Request) (ja3.ClientHelloSpec, ja3.H2Ja3Spec)
	Ja3                bool                //是否开启ja3
	Ja3Spec            ja3.ClientHelloSpec //指定ja3Spec,使用ja3.CreateSpecWithStr 或者ja3.CreateSpecWithId 生成
	H2Ja3              bool                //是否开启h2 指纹
	H2Ja3Spec          ja3.H2Ja3Spec       //h2 指纹
}
type WsType int

const (
	Send = 1
	Recv = 2
)

type Client struct {
	debug              bool
	disVerify          bool
	responseCallBack   func(*http.Request, *http.Response) error
	wsCallBack         func(websocket.MessageType, []byte, WsType) error
	requestCallBack    func(*http.Request) error
	verifyAuthWithHttp func(*http.Request) error
	createSpecWithHttp func(*http.Request) (ja3.ClientHelloSpec, ja3.H2Ja3Spec)

	ja3     bool                //是否开启ja3
	ja3Spec ja3.ClientHelloSpec //指定ja3Spec,使用ja3.CreateSpecWithStr 或者ja3.CreateSpecWithId 生成

	h2Ja3     bool          //是否开启h2 指纹
	h2Ja3Spec ja3.H2Ja3Spec //h2 指纹

	err      error //错误
	cert     tls.Certificate
	dialer   *requests.DialClient //连接的Dialer
	listener net.Listener         //Listener 服务
	basic    string
	usr      string
	pwd      string
	vpn      bool
	ipWhite  *kinds.Set[string]
	ctx      context.Context
	cnl      context.CancelFunc
	host     string
	port     string
}

func NewClient(pre_ctx context.Context, option ClientOption) (*Client, error) {
	if pre_ctx == nil {
		pre_ctx = context.TODO()
	}
	ctx, cnl := context.WithCancel(pre_ctx)
	server := Client{
		debug:              option.Debug,
		disVerify:          option.DisVerify,
		responseCallBack:   option.ResponseCallBack,
		wsCallBack:         option.WsCallBack,
		requestCallBack:    option.RequestCallBack,
		verifyAuthWithHttp: option.VerifyAuthWithHttp,
		createSpecWithHttp: option.CreateSpecWithHttp,
		ja3:                option.Ja3,
		ja3Spec:            option.Ja3Spec,
		h2Ja3:              option.H2Ja3,
		h2Ja3Spec:          option.H2Ja3Spec,
	}
	server.ctx = ctx
	server.cnl = cnl
	if option.Vpn {
		server.vpn = option.Vpn
	}
	if option.Usr != "" && option.Pwd != "" {
		server.basic = "Basic " + tools.Base64Encode(option.Usr+":"+option.Pwd)
		server.usr = option.Usr
		server.pwd = option.Pwd
	}
	//白名单
	server.ipWhite = kinds.NewSet[string]()
	for _, ip_white := range option.IpWhite {
		server.ipWhite.Add(ip_white.String())
	}
	var err error
	//dialer
	if server.dialer, err = requests.NewDail(requests.DialOption{
		TLSHandshakeTimeout: option.TLSHandshakeTimeout,
		DnsCacheTime:        option.DnsCacheTime,
		GetProxy:            option.GetProxy,
		Proxy:               option.Proxy,
		KeepAlive:           option.KeepAlive,
		LocalAddr:           option.LocalAddr,
		ProxyJa3:            option.ProxyJa3,
		ProxyJa3Spec:        option.ProxyJa3Spec,
		Ja3:                 option.Ja3,
		Ja3Spec:             option.Ja3Spec,

		Dns:         option.Dns,
		GetAddrType: option.GetAddrType,
		AddrType:    option.AddrType,
	}); err != nil {
		return nil, err
	}
	//证书
	if option.CrtFile == nil || option.KeyFile == nil {
		if server.cert, err = tools.CreateProxyCertWithName(option.ServerName); err != nil {
			return nil, err
		}
	} else {
		if server.cert, err = tls.X509KeyPair(option.CrtFile, option.KeyFile); err != nil {
			return nil, err
		}
	}
	//构造listen
	if server.listener, err = net.Listen("tcp", net.JoinHostPort(option.Host, strconv.Itoa(option.Port))); err != nil {
		return nil, err
	}
	if server.host, server.port, err = net.SplitHostPort(server.listener.Addr().String()); err != nil {
		return nil, err
	}
	return &server, nil
}

// 代理监听的端口
func (obj *Client) Addr() string {
	return net.JoinHostPort(obj.host, obj.port)
}

func (obj *Client) Run() error {
	defer obj.Close()
	for {
		select {
		case <-obj.ctx.Done():
			obj.err = obj.ctx.Err()
			return obj.err
		default:
			client, err := obj.listener.Accept() //接受数据
			if err != nil {
				obj.err = err
				return err
			}
			go obj.mainHandle(obj.ctx, client)
		}
	}
}
func (obj *Client) Close() {
	obj.listener.Close()
	obj.cnl()
}
func (obj *Client) Done() <-chan struct{} {
	return obj.ctx.Done()
}

func (obj *Client) whiteVerify(client net.Conn) bool {
	if obj.disVerify {
		return true
	}
	host, _, err := net.SplitHostPort(client.RemoteAddr().String())
	if err != nil || !obj.ipWhite.Has(host) {
		return false
	}
	return true
}

// 返回:请求所有内容,第一行的内容被" "分割的数组,第一行的内容,error
func (obj *Client) verifyPwd(client net.Conn, clientReq *http.Request) error {
	if obj.basic != "" && clientReq.Header.Get("Proxy-Authorization") != obj.basic && !obj.whiteVerify(client) { //验证密码是否正确
		client.Write([]byte(fmt.Sprintf("%s 407 Proxy Authentication Required\r\nProxy-Authenticate: Basic\r\n\r\n", clientReq.Proto)))
		return errors.New("auth verify fail")
	}
	return nil
}
func (obj *Client) getHttpProxyConn(ctx context.Context, ipUrl *url.URL) (net.Conn, error) {
	return obj.dialer.DialContext(ctx, "tcp", net.JoinHostPort(ipUrl.Hostname(), ipUrl.Port()))
}
func (obj *Client) mainHandle(ctx context.Context, client net.Conn) (err error) {
	defer recover()
	if obj.debug {
		defer func() {
			if err != nil {
				log.Print("proxy debugger:\n", err)
			}
		}()
	}
	if client == nil {
		return errors.New("client is nil")
	}
	defer client.Close()
	if obj.basic == "" && !obj.whiteVerify(client) {
		return errors.New("auth verify false")
	}
	clientReader := bufio.NewReader(client)
	firstCons, err := clientReader.Peek(1)
	if err != nil {
		return err
	}
	if obj.vpn {
		if firstCons[0] == 22 {
			return obj.httpsHandle(ctx, newProxyCon(ctx, client, clientReader, ProxyOption{}, true))
		}
		return fmt.Errorf("vpn error first byte: %d", firstCons[0])
	}
	switch firstCons[0] {
	case 5: //socks5 代理
		return obj.sockes5Handle(ctx, newProxyCon(ctx, client, clientReader, ProxyOption{}, true))
	case 22: //https 代理
		return obj.httpsHandle(ctx, newProxyCon(ctx, client, clientReader, ProxyOption{}, true))
	default: //http 代理
		return obj.httpHandle(ctx, newProxyCon(ctx, client, clientReader, ProxyOption{}, true))
	}
}
