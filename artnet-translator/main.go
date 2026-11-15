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

	"github.com/mariance95/lj-buddy/artnet-translator/internal/engine"
	"github.com/mariance95/lj-buddy/artnet-translator/internal/web"
)

var version = "1.0.0"

func main() {
	httpAddr := flag.String("http", "127.0.0.1:6455", "dirección del panel web local")
	noBrowser := flag.Bool("no-browser", false, "no abrir el navegador al iniciar")
	sim := flag.Int("sim", 0, "cantidad de interfaces DMX simuladas (para probar sin hardware)")
	cfgPath := flag.String("config", filepath.Join(engine.ConfigDir(), "config.json"), "archivo de configuración")
	showVersion := flag.Bool("version", false, "mostrar la versión y salir")
	flag.Parse()
	if *showVersion {
		fmt.Println("LJ Buddy ArtNet Translator", version)
		return
	}
	url := "http://" + *httpAddr + "/"

	// Una sola instancia: si el panel ya está en uso, se muestra el existente.
	ln, err := net.Listen("tcp", *httpAddr)
	if err != nil {
		if !*noBrowser {
			openBrowser(url)
		}
		fmt.Fprintf(os.Stderr, "LJ Buddy ArtNet Translator ya está en ejecución (o %s está ocupado): %v\n", *httpAddr, err)
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
	log.Printf("LJ Buddy ArtNet Translator %s iniciado. Panel: %s", version, url)

	e := engine.New(*cfgPath, *sim)
	e.Start()

	quit := make(chan struct{})
	var once sync.Once
	requestQuit := func() { once.Do(func() { close(quit) }) }

	go func() {
		if err := http.Serve(ln, web.Handler(e, logs, version, requestQuit)); err != nil {
			log.Printf("Panel web: %v", err)
		}
	}()
	if !*noBrowser {
		openBrowser(url)
	}

	// Bloquea hasta que el usuario sale (bandeja, panel o Ctrl+C).
	runUI(url, quit, requestQuit)

	log.Print("Cerrando…")
	e.Stop()
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
