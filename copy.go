package proxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"log"
	"net"
	"slices"

	"net/http"

	"github.com/gospider007/gtls"
	"github.com/gospider007/http2"
	"github.com/gospider007/tools"
	"github.com/gospider007/websocket"
	utls "github.com/refraction-networking/utls"
)

func (obj *Client) wsCopy(wsWriter *websocket.Conn, wsReader *websocket.Conn) (err error) {
	defer wsWriter.Close()
	defer wsReader.Close()
	var msgType websocket.MessageType
	var msgData []byte
	for {
		if msgType, msgData, err = wsReader.ReadMessage(); err != nil {
			return
		}
		if obj.wsCallBack != nil {
			if wsReader.IsClient() {
				if err = obj.wsCallBack(msgType, msgData, WsRecv); err != nil {
					return err
				}
			} else {
				if err = obj.wsCallBack(msgType, msgData, WsSend); err != nil {
					return err
				}
			}
		}
		if err = wsWriter.WriteMessage(msgType, msgData); err != nil {
			return
		}
	}
}

func (obj *Client) TlsConfig() *tls.Config {
	return obj.tlsConfig.Clone()
}
func (obj *Client) ProxyTlsConfig() *tls.Config {
	return obj.proxyTlsConfig.Clone()
}
func (obj *Client) UtlsConfig() *utls.Config {
	return obj.utlsConfig.Clone()
}

func (obj *Client) http22Copy(preCtx context.Context, client *ProxyConn, server *ProxyConn) error {
	defer client.Close()
	defer server.Close()

	http2Client := http2.Http2NewReaderFramer(client)
	http2Server := http2.Http2NewReaderFramer(client)
	// client.option.gospiderSpec.H2Spec
	go func() {
		defer client.Close()
		defer server.Close()
		for {
			_, data, err := http2Server.ReadFrame()
			if err != nil {
				return
			}
			_, err = client.Write(data)
			if err != nil {
				return
			}
		}
	}()
	for {
		_, data, err := http2Client.ReadFrame()
		if err != nil {
			return err
		}
		_, err = server.Write(data)
		if err != nil {
			return err
		}
	}
}

//	func (obj *Client) http12Copy(ctx context.Context, client *ProxyConn, server *ProxyConn) (err error) {
//		defer client.Close()
//		defer server.Close()
//		serverConn, err := http2.NewClientConn(func() {
//			client.Close()
//		}, server, client.option.hSpec)
//		if err != nil {
//			return err
//		}
//		var req *http.Request
//		var resp *http.Response
//		for {
//			if client.req != nil {
//				req, client.req = client.req, nil
//			} else {
//				if req, err = client.readRequest(ctx, obj.requestCallBack, nil); err != nil {
//					return
//				}
//			}
//			req.Proto = "HTTP/2.0"
//			req.ProtoMajor = 2
//			req.ProtoMinor = 0
//			if resp, err = serverConn.RoundTrip(req); err != nil {
//				return
//			}
//			if resp.ContentLength <= 0 && resp.TransferEncoding == nil {
//				resp.TransferEncoding = []string{"chunked"}
//			}
//			resp.Proto = "HTTP/1.1"
//			resp.ProtoMajor = 1
//			resp.ProtoMinor = 1
//			if obj.requestCallBack != nil {
//				if err = obj.requestCallBack(req, resp); err != nil {
//					return
//				}
//			}
//			if err = resp.Write(client); err != nil {
//				return
//			}
//		}
//	}
func (obj *Client) http11Copy(ctx context.Context, client *ProxyConn, server *ProxyConn) (err error) {
	var req *http.Request
	var rsp *http.Response
	for {
		if client.req != nil {
			req, client.req = client.req, nil
		} else {
			if req, err = client.readRequest(ctx, obj.requestCallBack, nil); err != nil {
				return
			}
		}
		if err = req.Write(server); err != nil {
			return
		}
		if rsp, err = server.readResponse(req); err != nil {
			return
		}
		if obj.requestCallBack != nil {
			if err = obj.requestCallBack(req, rsp); err != nil {
				return
			}
		}
		if err = rsp.Write(client); err != nil {
			return
		}
		if rsp.StatusCode == 101 {
			return
		}
	}
}

func (obj *Client) copyMain(ctx context.Context, client *ProxyConn, server *ProxyConn) (err error) {
	if client.option.schema == "http" {
		return obj.copyHttpMain(ctx, client, server)
	} else if client.option.schema == "https" {
		if obj.requestCallBack != nil ||
			obj.wsCallBack != nil ||
			client.option.gospiderSpec != nil ||
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
	// if !client.option.http2 && server.option.http2 { //http12 逻辑
	// 	return obj.http12Copy(ctx, client, server)
	// }
	if client.option.http2 && server.option.http2 { //http22 逻辑
		if obj.requestCallBack != nil ||
			(client.option.gospiderSpec != nil && client.option.gospiderSpec.H2Spec != nil) { //需要拦截请求 或需要设置h2指纹，就走12
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
		log.Print(err)
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
	wsServer := websocket.NewConn(client, false, server.option.wsExtensions)
	wsClient := websocket.NewConn(server, true, server.option.wsExtensions)
	log.Print(server.option.wsExtensions)
	go obj.wsCopy(wsClient, wsServer)
	return obj.wsCopy(wsServer, wsClient)
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
			tlsServer, negotiatedProtocol, err := obj.tlsServer(ctx, server, client.option.host, nextProtos, client.option)
			if err != nil {
				return err
			}
			server = newProxyCon(tlsServer, bufio.NewReader(tlsServer), *server.option, false)
			server.option.http2 = negotiatedProtocol == "h2"
		}
		return obj.copyHttpMain(ctx, client, server)
	}
	var tlsClient *tls.Conn
	var tlsServer net.Conn
	var negotiatedProtocol string
	tlsConfig := obj.TlsConfig()
	tlsConfig.GetConfigForClient = func(chi *tls.ClientHelloInfo) (*tls.Config, error) {
		serverName := chi.ServerName
		if serverName == "" {
			serverName = gtls.GetServerName(client.option.host)
		}
		tlsServer, negotiatedProtocol, err = obj.tlsServer(ctx, server, serverName, chi.SupportedProtos, client.option)
		if err != nil {
			return nil, err
		}
		if negotiatedProtocol == "" {
			negotiatedProtocol = "http/1.1"
		}
		var cert tls.Certificate
		cert, err = gtls.CreateCertWithName(serverName)
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
	clientProxy := newProxyCon(tlsClient, bufio.NewReader(tlsClient), *client.option, true)
	serverProxy := newProxyCon(tlsServer, bufio.NewReader(tlsServer), *server.option, false)
	return obj.copyHttpMain(ctx, clientProxy, serverProxy)
}
func (obj *Client) tlsServer(ctx context.Context, conn net.Conn, addr string, nextProtos []string, clientOption *ProxyOption) (net.Conn, string, error) {
	if clientOption.gospiderSpec.TLSSpec != nil {
		utlsConfig := obj.UtlsConfig()
		utlsConfig.NextProtos = nextProtos
		tlsConn, err := obj.specClient.Client(ctx, conn, clientOption.gospiderSpec.TLSSpec, utlsConfig, gtls.GetServerName(addr), !slices.Contains(nextProtos, "h2"))
		if err != nil {
			return tlsConn, "", err
		}
		return tlsConn, tlsConn.ConnectionState().NegotiatedProtocol, nil
	}
	tlsConfig := obj.TlsConfig()
	tlsConfig.NextProtos = nextProtos
	tlsConfig.ServerName = gtls.GetServerName(addr)
	tlsConn := tls.Client(conn, tlsConfig)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		return tlsConn, "", err
	}
	return tlsConn, tlsConn.ConnectionState().NegotiatedProtocol, nil
}
