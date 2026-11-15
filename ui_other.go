//go:build !windows

package main

import (
	"os"
	"os/signal"
	"syscall"
)

func runUI(_ string, quit <-chan struct{}, _ func()) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	select {
	case <-sig:
	case <-quit:
	}
}
