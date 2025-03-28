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
	"net/url"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"net/http"

	"github.com/gospider007/gtls"
	"github.com/gospider007/ja3"
	"github.com/gospider007/kinds"
	"github.com/gospider007/requests"
	"github.com/gospider007/tools"
	"github.com/gospider007/websocket"
	utls "github.com/refraction-networking/utls"
)

type ClientOption struct {
	Usr         string   //用户名
	Pwd         string   //密码
	IpWhite     []net.IP //白名单 192.168.1.1,192.168.1.2
	Addr        string
	CrtFile     []byte //公钥,根证书
	KeyFile     []byte //私钥
	DomainNames []string

	GetProxy func(ctx context.Context, url *url.URL) (string, error) //代理ip http://116.62.55.139:8888
	Proxy    string                                                  //代理ip http://192.168.1.50:8888

	DialTimeout time.Duration                   //tls 握手超时时间
	KeepAlive   time.Duration                   //保活时间
	LocalAddr   *net.TCPAddr                    //本地网卡出口
	GetAddrType func(host string) gtls.AddrType //控制host优先解析的类型
	AddrType    gtls.AddrType                   //host优先解析的类型
	Dns         *net.UDPAddr

	Debug     bool //是否打印debug
	DisVerify bool //关闭验证
	//发送请求和接收response 回调，返回error,则中断请求
	RequestCallBack func(*http.Request, *http.Response) error
	//websocket 传输回调，返回error,则中断请求
	WsCallBack func(websocket.MessageType, []byte, WsType) error
	//连接回调,返回error,则中断请求
	HttpConnectCallBack func(*http.Request) error
	//http,https 代理根据请求验证用户权限，返回error 则中断请求
	VerifyAuthWithHttp func(*http.Request) error
	//支持根据http,https代理的请求，动态生成ja3,h2指纹。注意这请求是客户端和代理协议协商的请求，不是客户端请求目标地址的请求
	//返回空结构体，则不会设置指纹
	CreateSpecWithHttp func(*http.Request) *requests.GospiderSpec
	Spec               string
	TlsConfig          *tls.Config
	UtlsConfig         *utls.Config
}
type WsType int

const (
	WsSend WsType = 1
	WsRecv WsType = 2
)

type Client struct {
	specClient          *ja3.Client
	debug               bool
	disVerify           bool
	requestCallBack     func(*http.Request, *http.Response) error
	wsCallBack          func(websocket.MessageType, []byte, WsType) error
	httpConnectCallBack func(*http.Request) error
	verifyAuthWithHttp  func(*http.Request) error
	createSpecWithHttp  func(*http.Request) *requests.GospiderSpec

	gospiderSpec *requests.GospiderSpec //gospider 指纹

	err      error //错误
	cert     tls.Certificate
	dialer   *requests.Dialer //连接的Dialer
	listener net.Listener     //Listener 服务
	basic    string
	usr      string
	pwd      string
	ipWhite  *kinds.Set[string]
	ctx      context.Context
	cnl      context.CancelFunc
	host     string
	port     int

	tlsConfig      *tls.Config
	proxyTlsConfig *tls.Config

	utlsConfig *utls.Config

	getProxy func(ctx context.Context, url *url.URL) (string, error) //代理ip http://116.62.55.139:8888
	proxy    *url.URL
}

func NewClient(pre_ctx context.Context, option ClientOption) (*Client, error) {
	if pre_ctx == nil {
		pre_ctx = context.TODO()
	}
	if option.TlsConfig == nil {
		option.TlsConfig = &tls.Config{
			InsecureSkipVerify: true,
			ClientSessionCache: tls.NewLRUClientSessionCache(0),
		}
	}
	if option.UtlsConfig == nil {
		option.UtlsConfig = &utls.Config{
			InsecureSkipVerify:                 true,
			ClientSessionCache:                 utls.NewLRUClientSessionCache(0),
			InsecureSkipTimeVerify:             true,
			OmitEmptyPsk:                       true,
			PreferSkipResumptionOnNilExtension: true,
		}
	}
	server := Client{
		specClient:          ja3.NewClient(),
		tlsConfig:           option.TlsConfig,
		utlsConfig:          option.UtlsConfig,
		getProxy:            option.GetProxy,
		debug:               option.Debug,
		disVerify:           option.DisVerify,
		httpConnectCallBack: option.HttpConnectCallBack,
		wsCallBack:          option.WsCallBack,
		requestCallBack:     option.RequestCallBack,
		verifyAuthWithHttp:  option.VerifyAuthWithHttp,
		createSpecWithHttp:  option.CreateSpecWithHttp,
	}
	if option.Addr == "" {
		option.Addr = ":0"
	}
	var err error

	var spec *requests.GospiderSpec
	if option.Spec != "" {
		if spec, err = requests.ParseGospiderSpec(option.Spec); err != nil {
			return nil, err
		}
	}
	server.gospiderSpec = spec

	if option.Proxy != "" {
		if server.proxy, err = gtls.VerifyProxy(option.Proxy); err != nil {
			return nil, err
		}
	}
	server.ctx, server.cnl = context.WithCancel(pre_ctx)
	if option.Usr != "" && option.Pwd != "" {
		server.basic = tools.Base64Encode(option.Usr + ":" + option.Pwd)
		server.usr = option.Usr
		server.pwd = option.Pwd
	}
	//白名单
	server.ipWhite = kinds.NewSet[string]()
	for _, ip_white := range option.IpWhite {
		server.ipWhite.Add(ip_white.String())
	}
	//dialer
	server.dialer = &requests.Dialer{}
	//证书

	if option.CrtFile != nil && option.KeyFile != nil {
		if server.cert, err = tls.X509KeyPair(option.CrtFile, option.KeyFile); err != nil {
			return nil, err
		}
		server.proxyTlsConfig = &tls.Config{
			NextProtos:   []string{"http/1.1"},
			Certificates: []tls.Certificate{server.cert},
		}
	} else {
		if option.DomainNames != nil {
			if server.proxyTlsConfig, err = gtls.TLS(option.DomainNames); err != nil {
				return nil, err
			}
			server.proxyTlsConfig.NextProtos = []string{"http/1.1"}
		} else {
			server.proxyTlsConfig = gtls.GetCertConfigForClient(&tls.Config{
				NextProtos: []string{"http/1.1"},
			})
		}
	}
	//构造listen
	if server.listener, err = net.Listen("tcp", option.Addr); err != nil {
		return nil, err
	}
	h, p, err := net.SplitHostPort(server.listener.Addr().String())
	if err != nil {
		return nil, err
	}
	server.host = h
	server.port, err = strconv.Atoi(p)
	if err != nil {
		return nil, err
	}
	return &server, nil
}

func (obj *Client) GetProxy(ctx context.Context, href *url.URL) (*url.URL, error) {
	if obj.proxy != nil {
		return obj.proxy, nil
	}
	if obj.getProxy != nil {
		proxy, err := obj.getProxy(ctx, href)
		if err != nil {
			return nil, err
		}
		return gtls.VerifyProxy(proxy)
	}
	return nil, nil
}

// 代理监听的端口
func (obj *Client) Addr() string {
	return net.JoinHostPort(obj.host, strconv.Itoa(obj.port))
}

func (obj *Client) Run() error {
	defer obj.Close()
	for {
		select {
		case <-obj.ctx.Done():
			obj.err = context.Cause(obj.ctx)
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
	if obj.basic == "" {
		return nil
	}
	for kk, vvs := range clientReq.Header {
		if strings.Contains(kk, "Authorization") {
			for _, vv := range vvs {
				if strings.Contains(vv, obj.basic) {
					return nil
				}
			}
		}
	}
	if obj.whiteVerify(client) {
		return nil
	}
	_, err := client.Write([]byte(fmt.Sprintf("%s 407 Authentication Required\r\nProxy-Authenticate: Basic\r\n\r\n", clientReq.Proto)))
	if err != nil {
		return err
	}
	return errors.New("auth verify fail")
}

func (obj *Client) mainHandle(ctx context.Context, client net.Conn) (err error) {
	defer recover()
	defer client.Close()
	if obj.debug {
		defer func() {
			if err != nil {
				log.Print("proxy debugger:\n", err)
				debug.PrintStack()
			}
		}()
	}
	if client == nil {
		return errors.New("client is nil")
	}
	if obj.basic == "" && !obj.whiteVerify(client) {
		return errors.New("auth verify false")
	}
	clientReader := bufio.NewReader(client)
	firstCons, err := clientReader.Peek(1)
	if err != nil {
		return err
	}
	switch firstCons[0] {
	case 5: //socks5 代理
		return obj.sockes5Handle(ctx, newProxyCon(client, clientReader, ProxyOption{}, true))
	case 22: //https 代理
		return obj.httpsHandle(ctx, newProxyCon(client, clientReader, ProxyOption{}, true))
	default: //http 代理
		return obj.httpHandle(ctx, newProxyCon(client, clientReader, ProxyOption{}, true))
	}
}
