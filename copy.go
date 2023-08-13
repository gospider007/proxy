package proxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"

	"net/http"

	"gitee.com/baixudong/http2"
	"gitee.com/baixudong/ja3"
	"gitee.com/baixudong/tools"
	"gitee.com/baixudong/websocket"
	utls "github.com/refraction-networking/utls"
	"golang.org/x/exp/slices"
)

func (obj *Client) wsSend(ctx context.Context, wsClient *websocket.Conn, wsServer *websocket.Conn) (err error) {
	defer wsServer.Close("close")
	defer wsClient.Close("close")
	var msgType websocket.MessageType
	var msgData []byte
	for {
		if msgType, msgData, err = wsClient.Recv(ctx); err != nil {
			return
		}
		if obj.wsCallBack != nil {
			if err = obj.wsCallBack(msgType, msgData, Send); err != nil {
				return err
			}
		}
		if err = wsServer.Send(ctx, msgType, msgData); err != nil {
			return
		}
	}
}
func (obj *Client) wsRecv(ctx context.Context, wsClient *websocket.Conn, wsServer *websocket.Conn) (err error) {
	defer wsServer.Close("close")
	defer wsClient.Close("close")
	var msgType websocket.MessageType
	var msgData []byte
	for {
		if msgType, msgData, err = wsServer.Recv(ctx); err != nil {
			return
		}
		if obj.wsCallBack != nil {
			if err = obj.wsCallBack(msgType, msgData, Recv); err != nil {
				return err
			}
		}
		if err = wsClient.Send(ctx, msgType, msgData); err != nil {
			return
		}
	}
}

type erringRoundTripper interface {
	RoundTripErr() error
}

func (obj *Client) TlsConfig() *tls.Config {
	return obj.tlsConfig.Clone()
}
func (obj *Client) UtlsConfig() *utls.Config {
	return obj.utlsConfig.Clone()
}
func (obj *Client) http22Copy(preCtx context.Context, client *ProxyConn, server *ProxyConn) (err error) {
	defer client.Close()
	defer server.Close()
	server.option.cnl2 = client.option.cnl
	serverConn, err := http2.NewClientConn(server, client.option.h2Ja3Spec)
	if err != nil {
		return err
	}
	ctx, cnl := context.WithCancel(preCtx)
	defer cnl()
	http2.NewServerConn(ctx, client, http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			r.URL.Scheme = "https"
			r.URL.Host = net.JoinHostPort(tools.GetServerName(client.option.host), client.option.port)
			if obj.requestCallBack != nil {
				if err = obj.requestCallBack(r, nil); err != nil {
					server.Close()
					client.Close()
					return
				}
			}
			resp, err := serverConn.RoundTrip(r)
			if err != nil {
				server.Close()
				client.Close()
				return
			}
			if resp.ContentLength <= 0 && resp.TransferEncoding == nil {
				resp.TransferEncoding = []string{"chunked"}
			}
			if obj.requestCallBack != nil {
				if err = obj.requestCallBack(r, resp); err != nil {
					server.Close()
					client.Close()
					return
				}
			}
			for kk, vvs := range resp.Header {
				for _, vv := range vvs {
					w.Header().Add(kk, vv)
				}
			}
			w.WriteHeader(resp.StatusCode)
			if resp.Body != nil {
				if err = tools.CopyWitchContext(r.Context(), w, resp.Body); err != nil {
					server.Close()
					client.Close()
					return
				}
			}
		},
	))
	return
}
func (obj *Client) http12Copy(ctx context.Context, client *ProxyConn, server *ProxyConn) (err error) {
	defer client.Close()
	defer server.Close()
	server.option.cnl2 = client.option.cnl
	serverConn, err := http2.NewClientConn(server, client.option.h2Ja3Spec)
	if err != nil {
		return err
	}
	var req *http.Request
	var resp *http.Response
	for {
		if client.req != nil {
			req, client.req = client.req, nil
		} else {
			if req, err = client.readRequest(client.option.ctx, obj.requestCallBack); err != nil {
				return
			}
		}
		req = req.WithContext(client.option.ctx)
		req.Proto = "HTTP/2.0"
		req.ProtoMajor = 2
		req.ProtoMinor = 0
		if resp, err = serverConn.RoundTrip(req); err != nil {
			return
		}
		if resp.ContentLength <= 0 && resp.TransferEncoding == nil {
			resp.TransferEncoding = []string{"chunked"}
		}
		resp.Proto = "HTTP/1.1"
		resp.ProtoMajor = 1
		resp.ProtoMinor = 1
		resp.Request = req.WithContext(client.option.ctx)
		if obj.requestCallBack != nil {
			if err = obj.requestCallBack(req, resp); err != nil {
				return
			}
		}
		if err = resp.Write(client); err != nil {
			return
		}
	}
}
func (obj *Client) http11Copy(ctx context.Context, client *ProxyConn, server *ProxyConn) (err error) {
	defer client.Close()
	defer server.Close()
	server.option.cnl2 = client.option.cnl
	var req *http.Request
	var rsp *http.Response

	for !server.option.isWs {
		if client.req != nil {
			req, client.req = client.req, nil
		} else {
			if req, err = client.readRequest(client.option.ctx, obj.requestCallBack); err != nil {
				return
			}
		}
		req = req.WithContext(client.option.ctx)
		if err = req.Write(server); err != nil {
			return
		}
		if rsp, err = server.readResponse(req); err != nil {
			return
		}
		rsp.Request = req.WithContext(client.option.ctx)
		if obj.requestCallBack != nil {
			if err = obj.requestCallBack(req, rsp); err != nil {
				return
			}
		}
		if err = rsp.Write(client); err != nil {
			return
		}
	}
	return
}

func (obj *Client) copyMain(ctx context.Context, client *ProxyConn, server *ProxyConn) (err error) {
	if client.option.schema == "http" {
		return obj.copyHttpMain(ctx, client, server)
	} else if client.option.schema == "https" {
		if obj.requestCallBack != nil ||
			obj.wsCallBack != nil ||
			client.option.ja3 ||
			client.option.h2Ja3 ||
			client.option.method != http.MethodConnect {
			return obj.copyHttpsMain(ctx, client, server)
		}
		return obj.copyHttpMain(ctx, client, server)
	} else {
		return errors.New("schema error")
	}
}
func (obj *Client) copyHttpMain(ctx context.Context, client *ProxyConn, server *ProxyConn) (err error) {
	defer server.Close()
	defer client.Close()
	if client.option.http2 && !server.option.http2 { //http21 逻辑
		return errors.New("没有21逻辑")
	}
	if !client.option.http2 && server.option.http2 { //http12 逻辑
		return obj.http12Copy(ctx, client, server)
	}
	if client.option.http2 && server.option.http2 { //http22 逻辑
		if obj.requestCallBack != nil ||
			client.option.h2Ja3 { //需要拦截请求 或需要设置h2指纹，就走12
			return obj.http22Copy(ctx, client, server)
		}
		go func() {
			defer client.Close()
			defer server.Close()
			tools.CopyWitchContext(ctx, client, server)
		}()
		return tools.CopyWitchContext(ctx, server, client)
	}
	if obj.wsCallBack == nil && obj.requestCallBack == nil { //没有回调直接返回
		if client.req != nil {
			if err = client.req.Write(server); err != nil {
				return err
			}
			client.req = nil
		}
		go func() {
			defer client.Close()
			defer server.Close()
			err = tools.CopyWitchContext(ctx, client, server)
		}()
		err = tools.CopyWitchContext(ctx, server, client)
		return
	}
	if err = obj.http11Copy(ctx, client, server); err != nil { //http11 开始回调
		return err
	}
	if obj.wsCallBack == nil { //没有ws 回调直接返回
		go func() {
			defer client.Close()
			defer server.Close()
			tools.CopyWitchContext(ctx, client, server)
		}()
		return tools.CopyWitchContext(ctx, server, client)
	}
	//ws 开始回调
	wsClient := websocket.NewConn(client, false, client.option.wsOption)
	wsServer := websocket.NewConn(server, true, server.option.wsOption)
	defer wsServer.Close("close")
	defer wsClient.Close("close")
	go obj.wsRecv(ctx, wsClient, wsServer)
	return obj.wsSend(ctx, wsClient, wsServer)
}
func (obj *Client) copyHttpsMain(ctx context.Context, client *ProxyConn, server *ProxyConn) (err error) {
	httpsBytes, err := client.reader.Peek(1)
	if err != nil {
		return err
	}
	if httpsBytes[0] != 22 { //客户端直连
		if client.option.method != http.MethodConnect { //服务端tls
			var nextProtos []string
			if client.option.isWs || server.option.isWs {
				nextProtos = []string{"http/1.1"}
			} else {
				nextProtos = []string{"h2", "http/1.1"}
			}
			tlsServer, _, negotiatedProtocol, err := obj.tlsServer(ctx, server, client.option.host, nextProtos, client.option.ja3, client.option.ja3Spec)
			if err != nil {
				return err
			}
			server = newProxyCon(ctx, tlsServer, bufio.NewReader(tlsServer), *server.option, false)
			server.option.http2 = negotiatedProtocol == "h2"
			return obj.copyHttpMain(ctx, client, server)
		} else { //服务端直连
			return obj.copyHttpMain(ctx, client, server)
		}
	}
	var tlsClient *tls.Conn
	var tlsServer net.Conn
	var peerCertificates []*x509.Certificate
	var negotiatedProtocol string
	tlsConfig := obj.TlsConfig()
	tlsConfig.GetConfigForClient = func(chi *tls.ClientHelloInfo) (*tls.Config, error) {
		serverName := chi.ServerName
		if serverName == "" {
			serverName = tools.GetServerName(client.option.host)
		}
		tlsServer, peerCertificates, negotiatedProtocol, err = obj.tlsServer(ctx, server, serverName, chi.SupportedProtos, client.option.ja3, client.option.ja3Spec)
		if err != nil {
			return nil, err
		}
		if negotiatedProtocol == "" {
			negotiatedProtocol = "http/1.1"
		}
		var cert tls.Certificate
		if len(peerCertificates) > 0 {
			preCert := peerCertificates[0]
			if chi.ServerName == "" && preCert.IPAddresses == nil && serverName != "" {
				if ip, ipType := tools.ParseHost(serverName); ipType != 0 {
					preCert.IPAddresses = []net.IP{ip}
				}
			}
			cert, err = tools.CreateProxyCertWithCert(nil, nil, preCert)
		} else {
			cert, err = tools.CreateProxyCertWithName(serverName)
		}
		if err != nil {
			return nil, err
		}
		tlsConfig2 := obj.TlsConfig()
		tlsConfig2.Certificates = []tls.Certificate{cert}
		tlsConfig2.NextProtos = []string{negotiatedProtocol}
		return tlsConfig2, nil
	}
	tlsClient = tls.Server(client, tlsConfig)
	if err = tlsClient.HandshakeContext(ctx); err != nil {
		return err
	}
	server.option.http2 = negotiatedProtocol == "h2"
	client.option.http2 = tlsClient.ConnectionState().NegotiatedProtocol == "h2"

	//重新包装连接
	clientProxy := newProxyCon(ctx, tlsClient, bufio.NewReader(tlsClient), *client.option, true)
	serverProxy := newProxyCon(ctx, tlsServer, bufio.NewReader(tlsServer), *server.option, false)
	return obj.copyHttpMain(ctx, clientProxy, serverProxy)
}
func (obj *Client) tlsServer(ctx context.Context, conn net.Conn, addr string, nextProtos []string, isJa3 bool, ja3Spec ja3.Ja3Spec) (net.Conn, []*x509.Certificate, string, error) {
	if isJa3 {
		utlsConfig := obj.UtlsConfig()
		utlsConfig.NextProtos = nextProtos
		utlsConfig.ServerName = tools.GetServerName(addr)
		tlsConn, err := ja3.NewClient(ctx, conn, ja3Spec, !slices.Contains(nextProtos, "h2"), utlsConfig)
		if err != nil {
			return tlsConn, nil, "", err
		}
		return tlsConn, tlsConn.ConnectionState().PeerCertificates, tlsConn.ConnectionState().NegotiatedProtocol, nil
	} else {
		tlsConfig := obj.TlsConfig()
		tlsConfig.NextProtos = nextProtos
		tlsConfig.ServerName = tools.GetServerName(addr)
		tlsConn := tls.Client(conn, tlsConfig)
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			return tlsConn, nil, "", err
		}
		return tlsConn, tlsConn.ConnectionState().PeerCertificates, tlsConn.ConnectionState().NegotiatedProtocol, nil
	}
}
