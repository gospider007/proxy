package proxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"

	"github.com/gospider007/requests"
)

func (obj *Client) httpHandle(ctx context.Context, client *ProxyConn) error {
	defer client.Close()
	var err error
	var clientReq *http.Request
	if obj.httpConnectCallBack == nil {
		clientReq, err = client.readRequest(ctx, nil, obj)
	} else {
		clientReq, err = client.readRequest(ctx, func(r1 *http.Request, r2 *http.Response) error {
			return obj.httpConnectCallBack(r1)
		}, obj)
	}
	if err != nil {
		return err
	}
	proxyUrl, err := obj.GetProxy(ctx, clientReq.URL)
	if err != nil {
		return err
	}
	var proxyServer net.Conn
	host := clientReq.Host
	addr := net.JoinHostPort(clientReq.URL.Hostname(), clientReq.URL.Port())
	if proxyServer, err = obj.dialer.DialContextWithProxy(ctx,
		requests.GetReqCtxData(ctx), "tcp",
		client.option.schema, addr, host, proxyUrl,
		obj.TlsConfig(),
	); err != nil {
		return err
	}
	server := newProxyCon(proxyServer, bufio.NewReader(proxyServer), *client.option, false)
	defer server.Close()
	if client.option.schema == "https" {
		if obj.createSpecWithHttp != nil {
			ja3Spec, h2Ja3Spec := obj.createSpecWithHttp(clientReq)
			if ja3Spec.IsSet() {
				client.option.ja3Spec = ja3Spec
			} else {
				client.option.ja3Spec = obj.ja3Spec
			}
			if h2Ja3Spec.IsSet() {
				client.option.h2Ja3Spec = h2Ja3Spec
			} else {
				client.option.h2Ja3Spec = obj.h2Ja3Spec
			}
		} else {
			client.option.ja3Spec = obj.ja3Spec
			client.option.h2Ja3Spec = obj.h2Ja3Spec
		}
	}
	if clientReq.Method == http.MethodConnect {
		if _, err = client.Write([]byte(fmt.Sprintf("%s 200 Connection established\r\n\r\n", clientReq.Proto))); err != nil {
			return err
		}
	} else {
		client.req = clientReq
	}
	return obj.copyMain(ctx, client, server)
}
func (obj *Client) httpsHandle(ctx context.Context, client *ProxyConn) error {
	defer client.Close()
	tlsClient := tls.Server(client, obj.ProxyTlsConfig())
	defer tlsClient.Close()
	if err := tlsClient.HandshakeContext(ctx); err != nil {
		return err
	}
	return obj.httpHandle(ctx, newProxyCon(tlsClient, bufio.NewReader(tlsClient), *client.option, true))
}
