package proxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
)

func (obj *Client) httpHandle(ctx context.Context, client *ProxyConn) error {
	defer client.Close()
	var err error
	clientReq, err := client.readRequest(ctx, obj.requestCallBack)
	if err != nil {
		return err
	}
	if strings.HasPrefix(clientReq.Host, "127.0.0.1") || strings.HasPrefix(clientReq.Host, "localhost") {
		if clientReq.URL.Port() == obj.port {
			return errors.New("loop addr error")
		}
	}
	if err = obj.verifyPwd(client, clientReq); err != nil {
		return err
	}
	if obj.verifyAuthWithHttp != nil {
		if err = obj.verifyAuthWithHttp(clientReq); err != nil {
			return err
		}
	}
	proxyUrl, err := obj.dialer.GetProxy(ctx, nil)
	if err != nil {
		return err
	}
	var proxyServer net.Conn
	host := clientReq.Host
	addr := net.JoinHostPort(clientReq.URL.Hostname(), clientReq.URL.Port())
	if proxyServer, err = obj.dialer.DialContextWithProxy(ctx, "tcp", client.option.schema, addr, host, proxyUrl); err != nil {
		return err
	}
	server := newProxyCon(ctx, proxyServer, bufio.NewReader(proxyServer), *client.option, false)
	defer server.Close()
	if client.option.schema == "https" {
		if obj.createSpecWithHttp != nil {
			ja3Spec, h2Ja3Spec := obj.createSpecWithHttp(clientReq)
			if ja3Spec.IsSet() {
				client.option.ja3 = true
				client.option.ja3Spec = ja3Spec
			} else if obj.ja3Spec.IsSet() {
				client.option.ja3 = true
				client.option.ja3Spec = obj.ja3Spec
			} else if obj.ja3 {
				client.option.ja3 = true
			}

			if h2Ja3Spec.IsSet() {
				client.option.h2Ja3 = true
				client.option.h2Ja3Spec = h2Ja3Spec
			} else if obj.h2Ja3Spec.IsSet() {
				client.option.h2Ja3 = true
				client.option.h2Ja3Spec = obj.h2Ja3Spec
			} else if obj.h2Ja3 {
				client.option.h2Ja3 = true
			}
		} else {
			if obj.ja3Spec.IsSet() {
				client.option.ja3 = true
				client.option.ja3Spec = obj.ja3Spec
			} else if obj.ja3 {
				client.option.ja3 = true
			}
			
			if obj.h2Ja3Spec.IsSet() {
				client.option.h2Ja3 = true
				client.option.h2Ja3Spec = obj.h2Ja3Spec
			} else if obj.h2Ja3 {
				client.option.h2Ja3 = true
			}
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
	tlsClient := tls.Server(client, &tls.Config{
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{obj.cert},
	})
	defer tlsClient.Close()
	return obj.httpHandle(ctx, newProxyCon(ctx, tlsClient, bufio.NewReader(tlsClient), *client.option, true))
}
