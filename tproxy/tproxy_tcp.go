package tproxy

import (
	"log"
	"net"
	"os"
	"syscall"
)

// ListenTCP tproxy tcp listen
func ListenTCP(addr string) (*net.TCPListener, error) {
	var ln *net.TCPListener
	var err error
	laddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, err
	}

	log.Println("tcp listening: ", laddr)
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

	fd := int(f.Fd())
	if err = syscall.SetsockoptInt(fd, syscall.SOL_IP, syscall.IP_TRANSPARENT, 1); err != nil {
		return nil, err
	}
	if err = syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
		return nil, err
	}

	return ln, nil
}

// AcceptTCP accept new tcp connection
func AcceptTCP(ln *net.TCPListener) (net.Conn, error) {
	conn, err := ln.AcceptTCP()
	if err != nil {
		return nil, err
	}
	return conn, nil
}
