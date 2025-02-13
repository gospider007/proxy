package proxy

import (
	"bufio"
	"context"
	"net"
	"time"

	"net/http"

	"github.com/gospider007/ja3"
	"github.com/gospider007/websocket"
)

type ProxyOption struct {
	spec  any       //ja3指纹
	hSpec ja3.HSpec //h2Ja3指纹

	init     bool
	http2    bool
	host     string
	schema   string
	method   string
	port     string
	isWs     bool
	wsOption websocket.Option
}
type ProxyConn struct {
	client bool
	conn   net.Conn
	req    *http.Request
	reader *bufio.Reader
	option *ProxyOption
}

func newProxyCon(conn net.Conn, reader *bufio.Reader, option ProxyOption, client bool) *ProxyConn {
	return &ProxyConn{conn: conn, reader: reader, option: &option, client: client}
}
func (obj *ProxyConn) Read(b []byte) (int, error) {
	if err := obj.SetDefaultDeadline(); err != nil {
		return 0, err
	}
	n, err := obj.reader.Read(b)
	if err != nil {
		obj.Close()
	}
	return n, err
}
func (obj *ProxyConn) Write(b []byte) (int, error) {
	if err := obj.SetDefaultDeadline(); err != nil {
		return 0, err
	}
	n, err := obj.conn.Write(b)
	if err != nil {
		obj.Close()
	}
	return n, err
}
func (obj *ProxyConn) Close() error {
	return obj.conn.Close()
}
func (obj *ProxyConn) LocalAddr() net.Addr {
	return obj.conn.LocalAddr()
}
func (obj *ProxyConn) RemoteAddr() net.Addr {
	return obj.conn.RemoteAddr()
}
func (obj *ProxyConn) SetDeadline(t time.Time) error {
	return obj.conn.SetDeadline(t)
}
func (obj *ProxyConn) SetDefaultDeadline() error {
	return obj.SetDeadline(time.Now().Add(time.Second * 300))
}
func (obj *ProxyConn) SetReadDeadline(t time.Time) error {
	return obj.conn.SetReadDeadline(t)
}
func (obj *ProxyConn) SetWriteDeadline(t time.Time) error {
	return obj.conn.SetWriteDeadline(t)
}
func (obj *ProxyConn) readResponse(req *http.Request) (*http.Response, error) {
	response, err := http.ReadResponse(obj.reader, req)
	if err != nil {
		return nil, err
	}
	if response.StatusCode == 101 && response.Header.Get("Upgrade") == "WebSocket" {
		obj.option.isWs = true
		obj.option.wsOption = websocket.GetResponseHeaderOption(response.Header)
	}
	return response, err
}
func (obj *ProxyConn) readRequest(ctx context.Context, requestCallBack func(*http.Request, *http.Response) error, client *Client) (*http.Request, error) {
	var clientReq *http.Request
	var err error
	done := make(chan struct{})
	go func() {
		defer close(done)
		clientReq, err = http.ReadRequest(obj.reader)
	}()
	select {
	case <-ctx.Done():
		return nil, context.Cause(ctx)
	case <-done:
	}
	if err != nil {
		return clientReq, err
	}
	if client != nil {
		if client.verifyAuthWithHttp != nil {
			if err = client.verifyAuthWithHttp(clientReq); err != nil {
				return clientReq, err
			}
		} else if err = client.verifyPwd(obj, clientReq); err != nil {
			return clientReq, err
		}
	}
	if requestCallBack != nil {
		if err = requestCallBack(clientReq, nil); err != nil {
			return clientReq, err
		}
	}
	obj.option.init = true
	if clientReq.Header.Get("Upgrade") == "websocket" {
		obj.option.isWs = true
	}

	hostName := clientReq.URL.Hostname()
	obj.option.method = clientReq.Method
	if obj.option.host == "" {
		if headHost := clientReq.Host; headHost != "" {
			obj.option.host = headHost
		} else if clientReq.Host != "" {
			obj.option.host = clientReq.Host
		} else if hostName != "" {
			obj.option.host = hostName
		}
	}
	if hostName == "" {
		if clientReq.Host != "" {
			clientReq.URL.Host = clientReq.Host
		} else {
			clientReq.URL.Host = obj.option.host
		}
	}

	if hostName := clientReq.URL.Hostname(); hostName == "" {
		clientReq.URL.Host = clientReq.Host
	} else if clientReq.Host == "" {
		clientReq.Host = hostName
	}
	if obj.option.schema == "" {
		if clientReq.URL.Scheme == "" {
			if clientReq.Method == http.MethodConnect {
				obj.option.schema = "https"
			} else {
				obj.option.schema = "http"
			}
			clientReq.URL.Scheme = obj.option.schema
		} else {
			obj.option.schema = clientReq.URL.Scheme
		}
	} else if clientReq.URL.Scheme == "" {
		clientReq.URL.Scheme = obj.option.schema
	}
	if obj.option.port == "" {
		if clientReq.URL.Port() == "" {
			if obj.option.schema == "https" {
				obj.option.port = "443"
			} else {
				obj.option.port = "80"
			}
			clientReq.URL.Host = clientReq.URL.Hostname() + ":" + obj.option.port
		} else {
			obj.option.port = clientReq.URL.Port()
		}
	} else if clientReq.URL.Port() == "" {
		clientReq.URL.Host = clientReq.URL.Hostname() + ":" + obj.option.port
	}
	return clientReq, err
}
