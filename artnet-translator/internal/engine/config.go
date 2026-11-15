package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// DeviceConfig es lo que se recuerda de cada interfaz entre sesiones.
type DeviceConfig struct {
	Name      string `json:"name"`
	Universes []int  `json:"universes"` // un universo Art-Net por salida
	Enabled   bool   `json:"enabled"`
	Kind      string `json:"kind"` // "" = autodetectar, "opendmx", "pro"
}

// Config es la configuración persistente.
type Config struct {
	ListenIP      string                   `json:"listenIP"`      // interfaz de red donde escuchar
	SourceIP      string                   `json:"sourceIP"`      // "" = aceptar cualquier emisor
	FirstUniverse int                      `json:"firstUniverse"` // base para la asignación automática
	ManualPorts   []string                 `json:"manualPorts"`   // puertos serie no FTDI agregados a mano
	Devices       map[string]*DeviceConfig `json:"devices"`
}

func defaultConfig() Config {
	return Config{ListenIP: "0.0.0.0", Devices: map[string]*DeviceConfig{}}
}

// ConfigDir es la carpeta de datos del programa.
func ConfigDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		base = "."
	}
	return filepath.Join(base, "LJBuddyArtNetTranslator")
}

func loadConfig(path string) Config {
	c := defaultConfig()
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &c)
	}
	if c.Devices == nil {
		c.Devices = map[string]*DeviceConfig{}
	}
	if c.ListenIP == "" {
		c.ListenIP = "0.0.0.0"
	}
	return c
}

func (c *Config) save(path string) error {
	if path == "" {
		return nil
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
