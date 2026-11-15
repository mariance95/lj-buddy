// Package web sirve el panel de control local.
package web

import (
	_ "embed"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/mariance95/lj-buddy/artnet-translator/internal/engine"
)

//go:embed index.html
var indexHTML []byte

// LogBuffer guarda las últimas líneas de registro para mostrarlas en el panel.
type LogBuffer struct {
	mu    sync.Mutex
	lines []string
}

func (l *LogBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, s := range strings.Split(strings.TrimRight(string(p), "\n"), "\n") {
		l.lines = append(l.lines, s)
	}
	if n := len(l.lines); n > 200 {
		l.lines = append([]string(nil), l.lines[n-200:]...)
	}
	return len(p), nil
}

func (l *LogBuffer) Lines() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string{}, l.lines...)
}

// Handler arma las rutas HTTP. quit se invoca cuando el usuario pide salir.
func Handler(e *engine.Engine, logs *LogBuffer, version string, quit func()) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(indexHTML)
	})
	mux.HandleFunc("GET /api/state", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, struct {
			engine.State
			Version string   `json:"version"`
			Log     []string `json:"log"`
		}{e.Snapshot(), version, logs.Lines()})
	})
	mux.HandleFunc("GET /api/levels", func(w http.ResponseWriter, r *http.Request) {
		u, _ := strconv.Atoi(r.URL.Query().Get("u"))
		writeJSON(w, map[string]any{"universe": u, "levels": e.Levels(u)}) // []byte → base64
	})
	mux.HandleFunc("POST /api/network", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ListenIP      string `json:"listenIP"`
			SourceIP      string `json:"sourceIP"`
			FirstUniverse int    `json:"firstUniverse"`
		}
		if !readJSON(w, r, &in) {
			return
		}
		reply(w, e.SetNetwork(strings.TrimSpace(in.ListenIP), strings.TrimSpace(in.SourceIP), in.FirstUniverse))
	})
	mux.HandleFunc("POST /api/device", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ID string `json:"id"`
			engine.DeviceUpdate
		}
		if !readJSON(w, r, &in) {
			return
		}
		reply(w, e.UpdateDevice(in.ID, in.DeviceUpdate))
	})
	mux.HandleFunc("POST /api/forget", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ID string `json:"id"`
		}
		if !readJSON(w, r, &in) {
			return
		}
		e.ForgetDevice(in.ID)
		reply(w, nil)
	})
	mux.HandleFunc("POST /api/addport", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Port string `json:"port"`
		}
		if !readJSON(w, r, &in) {
			return
		}
		reply(w, e.AddManualPort(in.Port))
	})
	mux.HandleFunc("POST /api/quit", func(w http.ResponseWriter, r *http.Request) {
		reply(w, nil)
		go quit()
	})

	return guard(mux)
}

// guard limita el panel al propio equipo: rechaza nombres de host ajenos
// (DNS rebinding) y exige en los POST una cabecera que un sitio web externo
// no puede enviar sin permiso CORS.
func guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		if host != "127.0.0.1" && host != "localhost" {
			http.Error(w, "host no permitido", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet && r.Header.Get("X-LJ-Buddy") != "1" {
			http.Error(w, "petición no permitida", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(v)
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(v); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return false
	}
	return true
}

func reply(w http.ResponseWriter, err error) {
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}
