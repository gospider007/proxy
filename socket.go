package proxy

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"

	"net/http"
	"net/url"

	"github.com/gospider007/requests"
	"github.com/gospider007/tools"
)

func (s *Client) udpMain(client *ProxyConn) error {
	udpConn, err := net.ListenUDP("udp", &net.UDPAddr{ //监听udp 端口
		IP: net.IPv4(0, 0, 0, 0),
	})
	if err != nil {
		return err
	}
	defer udpConn.Close()
	ip, port := client.LocalAddr().(*net.TCPAddr).IP, udpConn.LocalAddr().(*net.UDPAddr).Port
	_, err = client.Write([]byte{0x05, 0x00, 0})
	if err != nil {
		return err
	}
	err = requests.WriteUdpAddr(client, requests.Address{IP: ip, Port: port, NetWork: "udp"})
	if err != nil {
		return err
	}

	go func() {
		var buf [1]byte
		for {
			_, err := client.reader.Read(buf[:])
			if err != nil {
				udpConn.Close()
				break
			}
		}
	}()
	var (
		sourceAddr  net.Addr
		targetAddr  *net.UDPAddr
		replyPrefix []byte
		buf         [requests.MaxUdpPacket]byte
	)
	for {
		n, gotAddr, err := udpConn.ReadFrom(buf[:])
		if err != nil {
			return err
		}
		if sourceAddr == nil {
			sourceAddr = gotAddr
		}
		if sourceAddr.String() == gotAddr.String() {
			if n < 3 {
				continue
			}
			reader := bytes.NewBuffer(buf[3:n])
			addr, err := requests.ReadUdpAddr(reader)
			if err != nil {
				continue
			}
			if targetAddr == nil {
				targetAddr = &net.UDPAddr{
					IP:   addr.IP,
					Port: addr.Port,
				}
			}
			if addr.String() != targetAddr.String() {
				continue
			}
			_, err = udpConn.WriteTo(reader.Bytes(), targetAddr)
			if err != nil {
				return err
			}
		} else if targetAddr != nil && targetAddr.String() == gotAddr.String() {
			if replyPrefix == nil {
				b := bytes.NewBuffer(make([]byte, 3, 16))
				err = requests.WriteUdpAddr(b, requests.Address{IP: targetAddr.IP, Port: targetAddr.Port, NetWork: "udp"})
				if err != nil {
					return err
				}
				replyPrefix = b.Bytes()
			}
			copy(buf[len(replyPrefix):len(replyPrefix)+n], buf[:n])
			copy(buf[:len(replyPrefix)], replyPrefix)
			_, err = udpConn.WriteTo(buf[:len(replyPrefix)+n], sourceAddr)
			if err != nil {
				return err
			}
		}
	}
}

func (obj *Client) sockes5Handle(ctx context.Context, client *ProxyConn) error {
	defer client.Close()
	var err error
	if err = obj.verifySocket(client); err != nil {
		return err
	}
	//获取serverAddr
	cmd, err := obj.getCmd(client)
	if err != nil {
		return err
	}
	remoteAddress, err := requests.ReadUdpAddr(client.reader)
	if err != nil {
		return err
	}
	if cmd == 3 {
		return obj.udpMain(client)
	}
	remoteAddress.Scheme = client.option.schema
	remoteAddress.Host = remoteAddress.IP.String()
	if strings.HasPrefix(remoteAddress.String(), "127.0.0.1") || strings.HasPrefix(remoteAddress.String(), "localhost") {
		if remoteAddress.Port == obj.port {
			return errors.New("loop addr error")
		}
	}
	pu, _ := url.Parse(remoteAddress.String())
	//获取代理
	proxyUrl, err := obj.GetProxy(ctx, pu)
	if err != nil {
		return err
	}
	//获取schema
	httpsBytes, err := client.reader.Peek(1)
	if err != nil {
		return err
	}
	client.option.schema = "http"
	if httpsBytes[0] == 22 {
		client.option.schema = "https"
		client.option.method = http.MethodConnect
	}
	netword := "tcp"
	var proxyServer net.Conn
	if proxyUrl != nil {
		proxyAddress, err := requests.GetAddressWithUrl(proxyUrl)
		if err != nil {
			return err
		}

		proxyServer, err = obj.dialer.DialProxyContext(ctx, requests.GetRequestOption(ctx), netword, obj.TlsConfig(), proxyAddress, remoteAddress)
	} else {
		proxyServer, err = obj.dialer.DialContext(ctx, requests.GetRequestOption(ctx), netword, remoteAddress)
	}
	if err != nil {
		return err
	}
	server := newProxyCon(proxyServer, bufio.NewReader(proxyServer), *client.option, false)
	client.option.port = strconv.Itoa(remoteAddress.Port)
	client.option.host = remoteAddress.Host
	server.option.port = strconv.Itoa(remoteAddress.Port)
	server.option.host = remoteAddress.Host
	defer server.Close()
	if client.option.schema == "https" {
		client.option.ja3Spec = obj.ja3Spec
		client.option.h2Ja3Spec = obj.h2Ja3Spec
	}
	return obj.copyMain(ctx, client, server)
}

func (obj *Client) getCmd(client *ProxyConn) (byte, error) {
	buf := make([]byte, 3)
	_, err := io.ReadFull(client.reader, buf) //读取版本号，CMD，RSV ，ATYP ，ADDR ，PORT
	if err != nil {
		return 0, fmt.Errorf("read header failed:%w", err)
	}
	ver, cmd := buf[0], buf[1]
	if ver != 5 {
		return 0, fmt.Errorf("not supported ver:%v", ver)
	}
	return cmd, nil
}

func (obj *Client) verifySocket(client *ProxyConn) error {
	ver, err := client.reader.ReadByte() //读取第一个字节判断是否是socks5协议
	if err != nil {
		return fmt.Errorf("read ver failed:%w", err)
	}
	if ver != 5 {
		return fmt.Errorf("not supported ver:%v", ver)
	}
	methodSize, err := client.reader.ReadByte() //读取第二个字节,method 的长度，支持认证的方法数量
	if err != nil {
		return fmt.Errorf("read methodSize failed:%w", err)
	}
	methods := make([]byte, methodSize)
	if _, err = io.ReadFull(client.reader, methods); err != nil { //读取method，支持认证的方法
		return fmt.Errorf("read method failed:%w", err)
	}
	if obj.basic != "" && !obj.whiteVerify(client) { //开始验证用户名密码
		if bytes.IndexByte(methods, 2) == -1 {
			return errors.New("不支持用户名密码验证")
		}
		_, err = client.Write([]byte{5, 2}) //告诉客户端要进行用户名密码验证
		if err != nil {
			return err
		}
		okVar, err := client.reader.ReadByte() //获取版本，通常为0x01
		if err != nil {
			return err
		}
		Len, err := client.reader.ReadByte() //获取用户名的长度
		if err != nil {
			return err
		}
		user := make([]byte, Len)
		if _, err = io.ReadFull(client.reader, user); err != nil {
			return err
		}
		if Len, err = client.reader.ReadByte(); err != nil { //获取密码的长度
			return err
		}
		pass := make([]byte, Len)
		if _, err = io.ReadFull(client.reader, pass); err != nil {
			return err
		}
		if tools.BytesToString(user) != obj.usr || tools.BytesToString(pass) != obj.pwd {
			client.Write([]byte{okVar, 0xff}) //用户名密码错误
			return errors.New("用户名密码错误")
		}
		_, err = client.Write([]byte{okVar, 0}) //协商成功
		return err
	}
	if _, err = client.Write([]byte{5, 0}); err != nil { //协商成功
		return err
	}
	return err
}
