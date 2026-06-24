//go:build !windows

package main

import (
	"context"
	"net"
	"syscall"
)

func listenUDP4Broadcast(ctx context.Context) (net.PacketConn, error) {
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			var sockErr error
			if err := c.Control(func(fd uintptr) {
				sockErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
			}); err != nil {
				return err
			}
			return sockErr
		},
	}
	return lc.ListenPacket(ctx, "udp4", ":0")
}
