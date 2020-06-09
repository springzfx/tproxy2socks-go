package tproxy

import (
	"log"
	"net"
	"os"
	"syscall"
)

func Listen(addr string) (*net.TCPListener, error) {
	var ln *net.TCPListener
	var err error
	laddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, err
	}

	log.Println("listening: ", laddr)
	ln, err = net.ListenTCP("tcp", laddr)
	if err != nil {
		return nil, err
	}

	var f *os.File
	f, err = ln.File()
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if err = syscall.SetsockoptInt(int(f.Fd()), syscall.SOL_IP, syscall.IP_TRANSPARENT, 1); err != nil {
		return nil, err
	}

	return ln, nil
}

func Accept(ln *net.TCPListener) (net.Conn, error) {
	conn, err := ln.AcceptTCP()
	if err != nil {
		return nil, err
	}
	return conn, nil
}
