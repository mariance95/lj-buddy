package main

import (
	_ "embed"

	"fyne.io/systray"
)

//go:embed assets/icon.ico
var trayIcon []byte

// runUI muestra el icono en la bandeja del sistema, que es la única presencia
// visible del programa en Windows además del panel web.
func runUI(url string, quit <-chan struct{}, requestQuit func()) {
	systray.Run(func() {
		systray.SetIcon(trayIcon)
		systray.SetTitle("LJ Buddy ArtNet Translator")
		systray.SetTooltip("LJ Buddy ArtNet Translator")
		open := systray.AddMenuItem("Abrir panel", "Abre el panel de control en el navegador")
		systray.AddSeparator()
		exit := systray.AddMenuItem("Salir", "Detiene el puente y cierra LJ Buddy ArtNet Translator")
		go func() {
			for {
				select {
				case <-open.ClickedCh:
					openBrowser(url)
				case <-exit.ClickedCh:
					requestQuit()
				case <-quit:
					systray.Quit()
					return
				}
			}
		}()
	}, func() {})
}
