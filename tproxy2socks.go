package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"

	"github.com/springzfx/tproxy2socks-go/socks5"
	"github.com/springzfx/tproxy2socks-go/tproxy"
)

// ConnTimeout read and write timeout
const ConnTimeout time.Duration = 10

type tproxyAddrT []string

func (t *tproxyAddrT) String() string {
	return strings.Join(*t, ",")
}
func (t *tproxyAddrT) Set(v string) error {
	*t = append(*t, v)
	return nil
}

func printUsage() {
	fmt.Println("tproxy2socks -tproxy <address> -socks <address> -user <user> -passwd <password>")
	fmt.Println("usage example: tproxy2socks -tproxy \"127.0.0.1:12345\" -tproxy \"[::1]:12345\" -socks \"127.0.0.1:1080\"")
}

func main() {
	var tproxyAddr tproxyAddrT
	flag.Var(&tproxyAddr, "tproxy", "tproxy bind address")
	flag.StringVar(&socks5.Server, "socks", "127.0.0.1:1080", "socsk5 server address")
	flag.StringVar(&socks5.Username, "user", "", "socsk5 username")
	flag.StringVar(&socks5.Passwd, "passwd", "", "socsk5 password")
	flag.Parse()

	if len(tproxyAddr) == 0 {
		printUsage()
		return
	}
	for _, addr := range tproxyAddr {
		startTCPListen(addr)
		startUDPListen(addr)
	}

	// avoid exit here
	flag := make(chan int)
	<-flag
	log.Println("exiting")
}

func startTCPListen(addr string) {
	listener, err := tproxy.ListenTCP(addr)
	if err != nil {
		log.Println("tcp listen error: ", err)
		return
	}
	go func() {
		for {
			inConn, err := tproxy.AcceptTCP(listener)
			if err != nil {
				log.Println("tcp tproxy accept error: ", err)
				continue
			}
			log.Println("tcp:", inConn.RemoteAddr(), "->", inConn.LocalAddr())
			go func() {
				defer inConn.Close()
				dstAddr := inConn.LocalAddr().String()
				outConn, err := socks5.ConnectTCP(dstAddr)
				if err != nil {
					log.Println("tcp outConn connect error: ", err)
					return
				}
				defer outConn.Close()
				bridge(inConn, outConn, false)
				defer log.Println("tcp connection closed")
			}()
		}
	}()
}

func startUDPListen(addr string) {
	uconn, err := tproxy.ListenUDP(addr)
	if err != nil {
		log.Println("udp tproxy listen error: ", err)
		return
	}
	go func() {
		data := make([]byte, 1024)
		for {
			n, srcAddr, dstAddr, err := tproxy.ReadFromUDP(uconn, data)
			if err != nil {
				log.Println("udp tproxy read error,", err)
				continue
			}
			// log.Println("read from udp", srcAddr, dstAddr)
			go func() {
				inConn, err := tproxy.BindAndConnectUDP(dstAddr, srcAddr)
				if err != nil {
					log.Println("udp inConn connect error:", err)
					return
				}
				log.Println("udp:", inConn.RemoteAddr(), "->", inConn.LocalAddr())
				defer inConn.Close()
				_outConn, err := socks5.ConnectUDP(inConn.LocalAddr().String())
				if err != nil {
					log.Println("udp outConn connect error:", err)
					return
				}
				outConn := socks5.NewEncapsedUDPConn(_outConn, inConn.LocalAddr().String())
				defer outConn.Close()
				if _, err := outConn.Write(data[:n]); err != nil {
					log.Println("udp outConn write error,", err)
					return
				}
				// log.Println(inConn.RemoteAddr(), outConn.RemoteAddr())
				bridge(inConn, outConn, true)
				// defer log.Println("udp connection closed")
			}()
		}
	}()
}

func bridge(inConn, outConn net.Conn, setTimeout bool) {
	flag := make(chan int)
	streamConn := func(dst, src net.Conn) {
		for {
			if setTimeout {
				src.SetDeadline(time.Now().Add(ConnTimeout * time.Second))
				dst.SetDeadline(time.Now().Add(ConnTimeout * time.Second))
			}
			n, err := io.Copy(dst, src)
			if err != nil || n == 0 {
				break
			}
		}
		flag <- 1
	}

	go streamConn(inConn, outConn)
	go streamConn(outConn, inConn)

	<-flag
}
