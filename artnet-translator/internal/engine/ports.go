package engine

import (
	"fmt"
	"strings"

	"github.com/mariance95/lj-buddy/artnet-translator/internal/dmx"
	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
)

// portInfo describe un puerto serie presente en el sistema.
type portInfo struct {
	Name    string
	IsUSB   bool
	VID     string
	PID     string
	Serial  string
	Product string
	sim     bool
}

const ftdiVID = "0403"

func (p portInfo) isFTDI() bool { return p.IsUSB && strings.EqualFold(p.VID, ftdiVID) }

// listPorts enumera los puertos serie. Es una variable para poder
// reemplazarla en pruebas.
var listPorts = func() ([]portInfo, error) {
	list, err := enumerator.GetDetailedPortsList()
	if err != nil {
		// Sin datos USB (p. ej. macOS sin cgo): al menos los nombres.
		names, err2 := serial.GetPortsList()
		if err2 != nil {
			return nil, err
		}
		var out []portInfo
		for _, n := range names {
			out = append(out, portInfo{Name: n})
		}
		return out, nil
	}
	var out []portInfo
	for _, p := range list {
		out = append(out, portInfo{
			Name: p.Name, IsUSB: p.IsUSB, VID: p.VID, PID: p.PID,
			Serial: p.SerialNumber, Product: p.Product,
		})
	}
	return out, nil
}

// openPort abre el puerto con los parámetros de DMX512: 250000 baudios, 8N2.
var openPort = func(name string) (dmx.Port, error) {
	p, err := serial.Open(name, &serial.Mode{
		BaudRate: 250000,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.TwoStopBits,
	})
	if err != nil {
		return nil, err
	}
	// En la Open DMX, RTS en bajo habilita el transmisor RS-485.
	_ = p.SetRTS(false)
	return p, nil
}

// deviceIDs calcula un identificador estable por puerto: el número de serie
// USB cuando existe (sobrevive a cambios de puerto COM) o el nombre del
// puerto en su defecto.
func deviceIDs(ports []portInfo) map[string]string {
	count := map[string]int{}
	base := func(p portInfo) string {
		if p.IsUSB && p.Serial != "" {
			return fmt.Sprintf("%s:%s:%s", strings.ToLower(p.VID), strings.ToLower(p.PID), p.Serial)
		}
		return ""
	}
	for _, p := range ports {
		count[base(p)]++
	}
	ids := map[string]string{}
	for _, p := range ports {
		b := base(p)
		switch {
		case b == "":
			ids[p.Name] = "port:" + p.Name
		case count[b] > 1: // chips de varios canales con el mismo número de serie
			ids[p.Name] = b + "#" + p.Name
		default:
			ids[p.Name] = b
		}
	}
	return ids
}
