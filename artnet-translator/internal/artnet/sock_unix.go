//go:build !windows

package artnet

import "syscall"

func reuseControl(_, _ string, c syscall.RawConn) error {
	var e error
	if err := c.Control(func(fd uintptr) {
		e = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
	}); err != nil {
		return err
	}
	return e
}
