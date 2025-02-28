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
	addr, err := requests.GetAddressWithUrl(clientReq.URL)
	if err != nil {
		return err
	}
	if proxyUrl != nil {
		proxyAddress, err := requests.GetAddressWithUrl(proxyUrl)
		if err != nil {
			return err
		}
		remoteAddress, err := requests.GetAddressWithUrl(clientReq.URL)
		if err != nil {
			return err
		}
		remoteAddress.Scheme = client.option.schema
		if _, proxyServer, err = obj.dialer.DialProxyContext(requests.NewResponse(ctx, requests.RequestOption{}), "tcp", obj.TlsConfig(), proxyAddress, remoteAddress); err != nil {
			return err
		}
	} else {
		if proxyServer, err = obj.dialer.DialContext(requests.NewResponse(ctx, requests.RequestOption{}), "tcp", addr); err != nil {
			return err
		}
	}
	server := newProxyCon(proxyServer, bufio.NewReader(proxyServer), *client.option, false)
	defer server.Close()
	if client.option.schema == "https" {
		if obj.createSpecWithHttp != nil {
			spec := obj.createSpecWithHttp(clientReq)
			if spec != nil {
				client.option.gospiderSpec = spec
			} else {
				client.option.gospiderSpec = obj.gospiderSpec
			}
		} else {
			client.option.gospiderSpec = obj.gospiderSpec
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
