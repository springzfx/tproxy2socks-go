package tproxy

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"syscall"
	"unsafe"
)

// IPV6_RECVORIGDSTADDR see https://github.com/torvalds/linux/blob/63bdf4284c38a48af21745ceb148a087b190cd21/include/uapi/linux/in6.h#L286
const IPV6_RECVORIGDSTADDR = 0x4a

// ListenUDP tproxy udp listen
func ListenUDP(addr string) (*net.UDPConn, error) {
	laddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}

	log.Println("udp listening: ", laddr)
	uconn, err := net.ListenUDP("udp", laddr)
	if err != nil {
		return nil, err
	}

	var f *os.File
	f, err = uconn.File()
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if err = syscall.SetsockoptInt(int(f.Fd()), syscall.SOL_IP, syscall.IP_TRANSPARENT, 1); err != nil {
		return nil, err
	}

	if err = syscall.SetsockoptInt(int(f.Fd()), syscall.SOL_IP, syscall.IP_RECVORIGDSTADDR, 1); err != nil {
		return nil, err
	}

	if laddr.IP.To4() == nil {
		if err = syscall.SetsockoptInt(int(f.Fd()), syscall.SOL_IPV6, IPV6_RECVORIGDSTADDR, 1); err != nil {
			return nil, err
		}
	}

	return uconn, nil
}

// ReadFromUDP read the first (un-socket) udp msg, and retrive orginal src and dst addr
func ReadFromUDP(conn *net.UDPConn, b []byte) (int, *net.UDPAddr, *net.UDPAddr, error) {
	oob := make([]byte, 1024)
	n, oobn, _, addr, err := conn.ReadMsgUDP(b, oob)
	if err != nil {
		return 0, nil, nil, err
	}

	msgs, err := syscall.ParseSocketControlMessage(oob[:oobn])
	if err != nil {
		return 0, nil, nil, fmt.Errorf("parsing socket control message: %s", err)
	}

	var originalDst *net.UDPAddr
	for _, msg := range msgs {
		if msg.Header.Level == syscall.SOL_IP && msg.Header.Type == syscall.IP_RECVORIGDSTADDR {
			originalDstRaw := &syscall.RawSockaddrInet4{}
			if err = binary.Read(bytes.NewReader(msg.Data), binary.LittleEndian, originalDstRaw); err != nil {
				return 0, nil, nil, fmt.Errorf("reading original destination address: %s", err)
			}
			pp := (*syscall.RawSockaddrInet4)(unsafe.Pointer(originalDstRaw))
			p := (*[2]byte)(unsafe.Pointer(&pp.Port))
			originalDst = &net.UDPAddr{
				IP:   net.IPv4(pp.Addr[0], pp.Addr[1], pp.Addr[2], pp.Addr[3]),
				Port: int(p[0])<<8 + int(p[1]),
			}

		}

		if msg.Header.Level == syscall.SOL_IPV6 && msg.Header.Type == IPV6_RECVORIGDSTADDR {
			originalDstRaw := &syscall.RawSockaddrInet6{}
			if err = binary.Read(bytes.NewReader(msg.Data), binary.LittleEndian, originalDstRaw); err != nil {
				return 0, nil, nil, fmt.Errorf("reading original destination address: %s", err)
			}
			pp := (*syscall.RawSockaddrInet6)(unsafe.Pointer(originalDstRaw))
			p := (*[2]byte)(unsafe.Pointer(&pp.Port))
			originalDst = &net.UDPAddr{
				IP:   net.IP(pp.Addr[:]),
				Port: int(p[0])<<8 + int(p[1]),
				Zone: strconv.Itoa(int(pp.Scope_id)),
			}
		}

	}
	if originalDst == nil {
		return 0, nil, nil, fmt.Errorf("unable to obtain original destination: %s", err)
	}

	return n, addr, originalDst, nil
}

// BindAndConnectUDP bind original dst addr and connect back to original src addr
func BindAndConnectUDP(laddr, raddr *net.UDPAddr) (*net.UDPConn, error) {
	fd, err := syscall.Socket(udpAddrFamily("udp", laddr, raddr), syscall.SOCK_DGRAM, 0)

	if err = syscall.SetsockoptInt(fd, syscall.SOL_IP, syscall.IP_TRANSPARENT, 1); err != nil {
		return nil, err
	}
	if err = syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
		return nil, err
	}

	sa, err := udpAddrToSocketAddr(laddr)
	if err != nil {
		return nil, err
	}
	err = syscall.Bind(fd, sa)
	if err != nil {
		return nil, err
	}

	sa, err = udpAddrToSocketAddr(raddr)
	if err != nil {
		return nil, err
	}
	if err = syscall.Connect(fd, sa); err != nil {
		return nil, err
	}

	fdFile := os.NewFile(uintptr(fd), fmt.Sprintf("tproxy-udp-dial-%s", raddr.String()))
	defer fdFile.Close()

	uconn, err := net.FileConn(fdFile)
	return uconn.(*net.UDPConn), nil
}

func udpAddrToSocketAddr(addr *net.UDPAddr) (syscall.Sockaddr, error) {
	switch {
	case addr.IP.To4() != nil:
		ip := [4]byte{}
		copy(ip[:], addr.IP.To4())

		return &syscall.SockaddrInet4{Addr: ip, Port: addr.Port}, nil

	default:
		ip := [16]byte{}
		copy(ip[:], addr.IP.To16())

		zoneID, err := strconv.ParseUint(addr.Zone, 10, 32)
		if err != nil {
			zoneID = 0
		}

		return &syscall.SockaddrInet6{Addr: ip, Port: addr.Port, ZoneId: uint32(zoneID)}, nil
	}
}

func udpAddrFamily(net string, laddr, raddr *net.UDPAddr) int {
	switch net[len(net)-1] {
	case '4':
		return syscall.AF_INET
	case '6':
		return syscall.AF_INET6
	}

	if (laddr == nil || laddr.IP.To4() != nil) &&
		(raddr == nil || laddr.IP.To4() != nil) {
		return syscall.AF_INET
	}
	return syscall.AF_INET6
}
