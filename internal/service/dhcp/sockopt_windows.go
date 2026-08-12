package dhcp

import (
	"net"
	"syscall"
)

func setBroadcastOn(conn *net.UDPConn) error {
	raw, err := conn.SyscallConn()
	if err != nil {
		return err
	}
	var inner error
	err = raw.Control(func(fd uintptr) {
		inner = syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
	})
	if err != nil {
		return err
	}
	return inner
}
