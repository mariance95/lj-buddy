// LJ Buddy ArtNet Translator: puente Art-Net → DMX para interfaces USB (Enttec Open DMX, DMX
// USB Pro y compatibles).
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/mariance95/lj-buddy/artnet-translator/internal/engine"
	"github.com/mariance95/lj-buddy/artnet-translator/internal/web"
)

const appName = "LJ Buddy ArtNet Translator"

var version = "1.0.0"

// uiOptions es lo que necesita la interfaz de cada sistema operativo.
type uiOptions struct {
	url         string
	hidden      bool            // arrancar sin mostrar el panel
	show        <-chan struct{} // peticiones de mostrar el panel
	showPanel   func()          // pide mostrar el panel
	quit        <-chan struct{} // se cierra cuando el programa debe terminar
	requestQuit func()
}

func main() {
	httpAddr := flag.String("http", "127.0.0.1:6455", "dirección interna del panel")
	hidden := flag.Bool("hidden", false, "iniciar sin abrir la ventana (queda en la bandeja)")
	flag.BoolVar(hidden, "no-browser", false, "igual que -hidden (nombre anterior)")
	sim := flag.Int("sim", 0, "cantidad de interfaces DMX simuladas (para probar sin hardware)")
	cfgPath := flag.String("config", filepath.Join(engine.ConfigDir(), "config.json"), "archivo de configuración")
	showVersion := flag.Bool("version", false, "mostrar la versión y salir")
	flag.Parse()
	if *showVersion {
		fmt.Println(appName, version)
		return
	}
	url := "http://" + *httpAddr + "/"

	// Una sola instancia: si ya hay una en ejecución, se muestra su ventana.
	ln, err := net.Listen("tcp", *httpAddr)
	if err != nil {
		if !*hidden && showExisting(url) {
			return
		}
		fmt.Fprintf(os.Stderr, "%s ya está en ejecución (o %s está ocupado): %v\n", appName, *httpAddr, err)
		os.Exit(1)
	}

	logs := &web.LogBuffer{}
	out := []io.Writer{logs, os.Stderr}
	if err := os.MkdirAll(engine.ConfigDir(), 0o755); err == nil {
		if f, err := os.Create(filepath.Join(engine.ConfigDir(), "lj-buddy-artnet-translator.log")); err == nil {
			defer f.Close()
			out = append(out, f)
		}
	}
	log.SetOutput(io.MultiWriter(out...))
	log.SetFlags(log.Ltime)
	log.Printf("%s %s iniciado. Panel: %s", appName, version, url)

	e := engine.New(*cfgPath, *sim)
	e.Start()

	quit := make(chan struct{})
	var once sync.Once
	requestQuit := func() { once.Do(func() { close(quit) }) }
	show := make(chan struct{}, 1)
	showPanel := func() {
		select {
		case show <- struct{}{}:
		default:
		}
	}

	go func() {
		if err := http.Serve(ln, web.Handler(e, logs, version, showPanel, requestQuit)); err != nil {
			log.Printf("Panel web: %v", err)
		}
	}()

	// Bloquea hasta que el usuario sale (bandeja, panel o Ctrl+C).
	runUI(uiOptions{url: url, hidden: *hidden, show: show, showPanel: showPanel, quit: quit, requestQuit: requestQuit})

	log.Print("Cerrando…")
	e.Stop()
}

// showExisting pide a la instancia que ya está en ejecución que muestre su panel.
func showExisting(url string) bool {
	allowForeground()
	req, err := http.NewRequest(http.MethodPost, url+"api/show", nil)
	if err != nil {
		return false
	}
	req.Header.Set("X-LJ-Buddy", "1")
	resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
