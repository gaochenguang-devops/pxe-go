//go:build !windows

package dhcp

import "net"

func setBroadcastOn(conn *net.UDPConn) error {
	return nil // Linux 下 UDP 广播默认启用，无需额外设置
}
