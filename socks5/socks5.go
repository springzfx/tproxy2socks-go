package socks5

import (
	"bytes"
	"fmt"
	"net"
	"net/url"
	"strconv"

	"github.com/ginuerzh/gosocks5"
	"github.com/ginuerzh/gosocks5/client"
)

const version byte = 0x05

var (
	// Server socks5 server addr
	Server string = "127.0.0.1:1080"
	// Username socks5 user
	Username string
	// Passwd socks5 password
	Passwd string
)

// socks5Dial connect to socks5 server
func socks5Dial() (net.Conn, error) {
	// dial to socks5 server
	cs := client.NewClientSelector(url.UserPassword(Username, Passwd), gosocks5.MethodNoAuth, gosocks5.MethodUserPass)
	socks5Conn, err := client.Dial(Server, client.SelectorDialOption(cs))
	if err != nil {
		return nil, fmt.Errorf("socks5 server connect failed: %v", err)
	}
	// log.Println("socks5 server connect success")
	// defer socks5Conn.Close()
	return socks5Conn, nil
}

// ConnectTCP setup new tcp connection with CONNECT
func ConnectTCP(dstAddr string) (net.Conn, error) {
	socks5Conn, err := socks5Dial()
	if err != nil {
		return nil, err
	}

	// CONNECT command
	addr, err := gosocks5.NewAddr(dstAddr)
	if err := gosocks5.NewRequest(gosocks5.CmdConnect, addr).Write(socks5Conn); err != nil {
		return nil, fmt.Errorf("connect command io error: %v", err)
	}
	reply, err := gosocks5.ReadReply(socks5Conn)
	if err != nil {
		return nil, fmt.Errorf("read reply io error: %v", err)
	}
	switch reply.Rep {
	case gosocks5.Succeeded:
		return socks5Conn, nil
	default:
		return nil, fmt.Errorf("connect command failed: %v", reply.Rep)
	}
}

// ConnectUDP setup new udp connection to addr from UDP ASSOCIATED
func ConnectUDP(dstAddr string) (*net.UDPConn, error) {
	socks5Conn, err := socks5Dial()
	if err != nil {
		return nil, err
	}

	// UDP ASSOCIATE command
	addr, _ := gosocks5.NewAddr(dstAddr)
	if err := gosocks5.NewRequest(gosocks5.CmdUdp, addr).Write(socks5Conn); err != nil {
		return nil, fmt.Errorf("socks5 udp associate request error: %v", err)
	}
	reply, err := gosocks5.ReadReply(socks5Conn)
	if err != nil {
		return nil, fmt.Errorf("socks5 udp associate reply error: %v", err)
	}
	switch reply.Rep {
	case gosocks5.Succeeded:
		remoteUDPAddr, _ := net.ResolveUDPAddr("udp", getAddrFromReply(reply))
		// log.Println("socks5 udp associate success:", remoteUDPAddr, reply)
		return net.DialUDP("udp", nil, remoteUDPAddr)
	case gosocks5.CmdUnsupported:
		return nil, fmt.Errorf("socks5 udp associate not supported")
	default:
		return nil, fmt.Errorf("socks5 udp associate failed with reply: %v", reply)
	}
}

func getAddrFromReply(reply *gosocks5.Reply) string {
	return reply.Addr.Host + ":" + strconv.Itoa(int(reply.Addr.Port))
}

// UDPConn encaps data to socks5 udp format
type UDPConn struct {
	*net.UDPConn
	oriDstAddr string
}

func (sc *UDPConn) Write(p []byte) (int, error) {
	addr, _ := gosocks5.NewAddr(sc.oriDstAddr)
	header := gosocks5.NewUDPHeader(0, 0, addr)
	err := gosocks5.NewUDPDatagram(header, p).Write(sc.UDPConn)
	if err != nil {
		return 0, err
	}
	// log.Printf("udp write to %v: %v\n", sc.RemoteAddr(), header)
	return len(p), nil
}

func (sc *UDPConn) Read(p []byte) (int, error) {
	n, err := sc.UDPConn.Read(p)
	data, err := gosocks5.ReadUDPDatagram(bytes.NewBuffer(p[:n]))
	if err != nil {
		return 0, err
	}
	// log.Printf("udp read from %v: %v\n", sc.RemoteAddr(), data)
	copy(p, data.Data)
	return len(data.Data), nil
}

// NewEncapsedUDPConn create socks5 encapsed udp conn
func NewEncapsedUDPConn(conn *net.UDPConn, dst string) *UDPConn {
	return &UDPConn{conn, dst}
}
