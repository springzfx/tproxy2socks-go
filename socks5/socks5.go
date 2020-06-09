package socks5

import (
	"log"
	"net"

	"golang.org/x/net/proxy"
)

// Connect DOC
func Connect(dstAddr, socksAddr string, auth proxy.Auth) (net.Conn, error) {
	log.Printf("socks proxy %v", dstAddr)
	d, err := proxy.SOCKS5("tcp", socksAddr, &auth, nil)
	conn, err := d.Dial("tcp", dstAddr)
	if err != nil {
		return nil, err
	}
	return conn, nil
}
