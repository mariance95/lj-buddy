//go:build !windows

package main

import (
	"os"
	"os/signal"
	"syscall"
)

// runUI, fuera de Windows, muestra el panel en el navegador.
func runUI(ui uiOptions) {
	if !ui.hidden {
		openBrowser(ui.url)
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	for {
		select {
		case <-ui.show:
			openBrowser(ui.url)
		case <-sig:
			return
		case <-ui.quit:
			return
		}
	}
}

func allowForeground() {}
