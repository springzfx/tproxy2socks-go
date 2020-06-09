package main

import (
	"io"
	"log"
	"net"

	"github.com/springzfx/tproxy2socks-go/socks5"
	"github.com/springzfx/tproxy2socks-go/tproxy"
	"golang.org/x/net/proxy"
)

func bridge(inConn, outConn net.Conn) {
	defer inConn.Close()
	defer outConn.Close()
	flag := make(chan int)
	streamConn := func(dst io.Writer, src io.Reader) {
		for {
			_, err := io.Copy(dst, src)
			if err != nil {
				break
			}
		}
		flag <- 1
	}

	go streamConn(outConn, inConn)
	go streamConn(inConn, outConn)

	<-flag
	return
}

func main() {
	startTcpListen("127.0.0.1:12347")
	startTcpListen("[::1]:12347")
	flag := make(chan int)
	<-flag
}

func startTcpListen(addr string) {
	listener4, err := tproxy.Listen(addr)
	if err != nil {
		log.Fatal("listen failed: ", err)
		return
	}
	go func() {
		for {
			inConn, err := tproxy.Accept(listener4)
			if err != nil {
				log.Printf("accept error: %s\n", err)
				continue
			}
			go func() {
				dstAddr := inConn.LocalAddr().String()
				outConn, err := socks5.Connect(dstAddr, "127.0.0.1:1080", proxy.Auth{})
				if err != nil {
					log.Printf("socks connect error: %s\n", err)
					inConn.Close()
					return
				}
				bridge(inConn, outConn)
			}()
		}
	}()

}
