package engine

import (
	"errors"
	"net"
	"sort"
	"time"

	"github.com/mariance95/lj-buddy/artnet-translator/internal/artnet"
	"github.com/mariance95/lj-buddy/artnet-translator/internal/dmx"
)

// Tipos que consume el panel web.

type OutputState struct {
	Universe int  `json:"universe"`
	FPS      int  `json:"fps"`
	Signal   bool `json:"signal"` // llega Art-Net para ese universo
}

type DeviceState struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Port      string        `json:"port"`
	Serial    string        `json:"serial"`
	Kind      string        `json:"kind"`
	KindLabel string        `json:"kindLabel"`
	KindMode  string        `json:"kindMode"` // "" auto, o tipo forzado
	Detail    string        `json:"detail"`
	Present   bool          `json:"present"`
	Running   bool          `json:"running"`
	Enabled   bool          `json:"enabled"`
	Test      bool          `json:"test"`
	Error     string        `json:"error"`
	Outputs   []OutputState `json:"outputs"`
}

type UniverseState struct {
	Universe int    `json:"universe"`
	PPS      int    `json:"pps"`
	Active   bool   `json:"active"`
	Source   string `json:"source"`
	Routed   bool   `json:"routed"` // alguna interfaz lo está emitiendo
}

type SourceState struct {
	IP     string `json:"ip"`
	Active bool   `json:"active"`
}

type PortState struct {
	Name string `json:"name"`
	Desc string `json:"desc"`
}

type State struct {
	Listening     bool            `json:"listening"`
	ListenError   string          `json:"listenError"`
	ScanError     string          `json:"scanError"`
	ListenIP      string          `json:"listenIP"`
	SourceIP      string          `json:"sourceIP"`
	FirstUniverse int             `json:"firstUniverse"`
	LocalIPs      []string        `json:"localIPs"`
	Sources       []SourceState   `json:"sources"`
	Universes     []UniverseState `json:"universes"`
	Devices       []DeviceState   `json:"devices"`
	OtherPorts    []PortState     `json:"otherPorts"`
}

// Snapshot devuelve el estado completo para mostrar.
func (e *Engine) Snapshot() State {
	local := artnet.LocalIPs()
	now := time.Now()
	e.mu.Lock()
	defer e.mu.Unlock()
	s := State{
		Listening: e.listening, ListenError: e.listenErr, ScanError: e.scanErr,
		ListenIP: e.cfg.ListenIP, SourceIP: e.cfg.SourceIP, FirstUniverse: e.cfg.FirstUniverse,
		LocalIPs: local,
		Sources:  []SourceState{}, Universes: []UniverseState{}, Devices: []DeviceState{}, OtherPorts: []PortState{},
	}
	for ip, src := range e.sources {
		s.Sources = append(s.Sources, SourceState{IP: ip, Active: now.Sub(src.lastSeen) < 3*time.Second})
	}
	sort.Slice(s.Sources, func(i, j int) bool { return s.Sources[i].IP < s.Sources[j].IP })

	routed := map[int]bool{}
	for id, d := range e.devs {
		dc := e.cfg.Devices[id]
		ds := DeviceState{
			ID: id, Port: d.port.Name, Serial: d.port.Serial,
			Kind: string(d.kind), KindLabel: d.kind.Label(), Detail: d.detail,
			Present: d.present, Running: d.running, Test: d.test, Error: d.err,
			Enabled: true, Outputs: []OutputState{},
		}
		if d.kind == "" {
			ds.KindLabel = "Sin identificar"
		}
		if dc != nil {
			ds.Name, ds.Enabled, ds.KindMode = dc.Name, dc.Enabled, dc.Kind
			for i, u := range dc.Universes {
				o := OutputState{Universe: u}
				if d.running && i < len(d.fps) {
					o.FPS = d.fps[i]
					routed[u] = true
				}
				if us := e.unis[uint16(u)]; us != nil {
					o.Signal = now.Sub(us.lastSeen) < 3*time.Second
				}
				ds.Outputs = append(ds.Outputs, o)
			}
		}
		if ds.Name == "" {
			ds.Name = describe(d.port)
		}
		s.Devices = append(s.Devices, ds)
	}
	sort.Slice(s.Devices, func(i, j int) bool {
		a, b := s.Devices[i], s.Devices[j]
		if a.Present != b.Present {
			return a.Present
		}
		ua, ub := maxUniverse+1, maxUniverse+1
		if len(a.Outputs) > 0 {
			ua = a.Outputs[0].Universe
		}
		if len(b.Outputs) > 0 {
			ub = b.Outputs[0].Universe
		}
		if ua != ub {
			return ua < ub
		}
		return a.ID < b.ID
	})

	for n, u := range e.unis {
		s.Universes = append(s.Universes, UniverseState{
			Universe: int(n), PPS: u.pps, Source: u.source,
			Active: now.Sub(u.lastSeen) < 3*time.Second, Routed: routed[int(n)],
		})
	}
	sort.Slice(s.Universes, func(i, j int) bool { return s.Universes[i].Universe < s.Universes[j].Universe })

	for _, p := range e.others {
		s.OtherPorts = append(s.OtherPorts, PortState{Name: p.Name, Desc: describe(p)})
	}
	return s
}

// Levels devuelve los 512 canales actuales de un universo.
func (e *Engine) Levels(u int) []byte {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]byte, 512)
	if us := e.unis[uint16(u)]; us != nil {
		copy(out, us.data[:])
	}
	return out
}

// SetNetwork cambia la interfaz de escucha, el filtro de emisor y la base de
// asignación automática.
func (e *Engine) SetNetwork(listenIP, sourceIP string, firstUniverse int) error {
	if listenIP == "" {
		listenIP = "0.0.0.0"
	}
	if ip := net.ParseIP(listenIP); ip == nil || ip.To4() == nil {
		return errors.New("IP de escucha inválida")
	}
	if sourceIP != "" {
		ip := net.ParseIP(sourceIP)
		if ip == nil || ip.To4() == nil {
			return errors.New("IP de origen inválida")
		}
		sourceIP = ip.String()
	}
	if firstUniverse < 0 || firstUniverse > maxUniverse {
		return errors.New("universo inicial fuera de rango (0-32767)")
	}
	e.mu.Lock()
	rebind := e.cfg.ListenIP != listenIP
	if e.cfg.SourceIP != sourceIP {
		// Al cambiar de emisor se descartan los datos del anterior.
		e.unis = map[uint16]*universe{}
	}
	e.cfg.ListenIP, e.cfg.SourceIP, e.cfg.FirstUniverse = listenIP, sourceIP, firstUniverse
	e.saveLocked()
	e.mu.Unlock()
	if rebind {
		poke(e.rebind)
	}
	return nil
}

// DeviceUpdate son los cambios que el usuario puede hacer a una interfaz.
// Los campos nil no se tocan.
type DeviceUpdate struct {
	Universes []int   `json:"universes"`
	Enabled   *bool   `json:"enabled"`
	Kind      *string `json:"kind"`
	Test      *bool   `json:"test"`
}

func (e *Engine) UpdateDevice(id string, up DeviceUpdate) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	dc, d := e.cfg.Devices[id], e.devs[id]
	if dc == nil || d == nil {
		return errors.New("interfaz desconocida")
	}
	if up.Universes != nil {
		if len(up.Universes) != len(dc.Universes) {
			return errors.New("cantidad de universos incorrecta")
		}
		for _, u := range up.Universes {
			if u < 0 || u > maxUniverse {
				return errors.New("universo fuera de rango (0-32767)")
			}
		}
		dc.Universes = append([]int(nil), up.Universes...)
	}
	restart := false
	if up.Enabled != nil && *up.Enabled != dc.Enabled {
		dc.Enabled = *up.Enabled
		restart = true
	}
	if up.Kind != nil && *up.Kind != dc.Kind {
		switch *up.Kind {
		case "", string(dmx.KindOpenDMX), string(dmx.KindPro):
		default:
			return errors.New("tipo de interfaz inválido")
		}
		dc.Kind = *up.Kind
		restart = true
	}
	if up.Test != nil {
		d.test = *up.Test && d.running
	}
	if restart {
		if d.cancel != nil {
			d.cancel()
		}
		d.err = ""
		// Se da tiempo a que el driver suelte el puerto antes de reabrirlo.
		time.AfterFunc(300*time.Millisecond, func() { poke(e.rescan) })
	}
	e.saveLocked()
	return nil
}

// ForgetDevice borra una interfaz de la configuración. Si sigue enchufada se
// volverá a detectar y asignar como nueva.
func (e *Engine) ForgetDevice(id string) {
	e.mu.Lock()
	if d := e.devs[id]; d != nil {
		if d.cancel != nil {
			d.cancel()
		}
		for i, n := range e.cfg.ManualPorts {
			if n == d.port.Name || "port:"+n == id {
				e.cfg.ManualPorts = append(e.cfg.ManualPorts[:i], e.cfg.ManualPorts[i+1:]...)
				break
			}
		}
		delete(e.devs, id)
	}
	delete(e.cfg.Devices, id)
	e.saveLocked()
	e.mu.Unlock()
	time.AfterFunc(300*time.Millisecond, func() { poke(e.rescan) })
}

// AddManualPort usa como interfaz DMX un puerto serie que no es FTDI.
func (e *Engine) AddManualPort(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	found := false
	for _, p := range e.others {
		found = found || p.Name == name
	}
	if !found {
		return errors.New("puerto no encontrado")
	}
	e.cfg.ManualPorts = append(e.cfg.ManualPorts, name)
	e.saveLocked()
	poke(e.rescan)
	return nil
}
