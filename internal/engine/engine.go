// Package engine une todo: recibe Art-Net, detecta las interfaces DMX y
// reparte cada universo a la salida que le corresponde.
package engine

import (
	"context"
	"fmt"
	"log"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/mariance95/lj-buddy/internal/artnet"
	"github.com/mariance95/lj-buddy/internal/dmx"
)

const maxUniverse = 32767

type universe struct {
	data     dmx.Frame
	lastSeen time.Time
	source   string
	count    uint64
	lastCnt  uint64
	pps      int
}

type source struct {
	lastSeen time.Time
	count    uint64
}

type device struct {
	id      string
	port    portInfo
	kind    dmx.Kind
	outputs int
	detail  string // lo que informó la interfaz al sondearla
	present bool   // el puerto existe ahora mismo
	running bool   // hay un driver transmitiendo
	err     string
	test    bool // salida de prueba: todos los canales al máximo
	cancel  context.CancelFunc
	sent    []uint64
	lastCnt []uint64
	fps     []int
}

// Engine es el puente Art-Net → DMX.
type Engine struct {
	mu      sync.Mutex
	cfg     Config
	cfgPath string
	sim     int

	unis    map[uint16]*universe
	sources map[string]*source
	devs    map[string]*device
	others  []portInfo

	conn      *net.UDPConn
	listening bool
	listenErr string
	scanErr   string

	rescan chan struct{}
	rebind chan struct{}
	ctx    context.Context
	stop   context.CancelFunc
	wg     sync.WaitGroup
}

// New crea el motor. cfgPath vacío = no persistir; sim = cantidad de
// interfaces simuladas.
func New(cfgPath string, sim int) *Engine {
	e := &Engine{
		cfgPath: cfgPath,
		sim:     sim,
		cfg:     defaultConfig(),
		unis:    map[uint16]*universe{},
		sources: map[string]*source{},
		devs:    map[string]*device{},
		rescan:  make(chan struct{}, 1),
		rebind:  make(chan struct{}, 1),
	}
	if cfgPath != "" {
		e.cfg = loadConfig(cfgPath)
	}
	return e
}

// Start lanza la recepción Art-Net y la detección de interfaces.
func (e *Engine) Start() {
	e.ctx, e.stop = context.WithCancel(context.Background())
	e.wg.Add(3)
	go e.listenLoop()
	go e.scanLoop()
	go e.statsLoop()
}

// Stop detiene todo y libera los puertos.
func (e *Engine) Stop() {
	e.stop()
	e.mu.Lock()
	if e.conn != nil {
		e.conn.Close()
	}
	e.mu.Unlock()
	e.wg.Wait()
}

func poke(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

func (e *Engine) saveLocked() {
	if err := e.cfg.save(e.cfgPath); err != nil {
		log.Printf("No se pudo guardar la configuración: %v", err)
	}
}

// ---------------------------------------------------------------- Art-Net

func (e *Engine) listenLoop() {
	defer e.wg.Done()
	for e.ctx.Err() == nil {
		e.mu.Lock()
		ip := e.cfg.ListenIP
		e.mu.Unlock()

		conn, err := artnet.Listen(ip)
		if err != nil {
			e.mu.Lock()
			changed := e.listenErr != err.Error()
			e.listening, e.listenErr = false, err.Error()
			e.mu.Unlock()
			if changed {
				log.Printf("Art-Net: no se pudo escuchar en %s:%d: %v", ip, artnet.Port, err)
			}
			select {
			case <-e.ctx.Done():
				return
			case <-e.rebind:
			case <-time.After(3 * time.Second):
			}
			continue
		}
		e.mu.Lock()
		e.conn, e.listening, e.listenErr = conn, true, ""
		e.mu.Unlock()
		log.Printf("Art-Net: escuchando en %s:%d", ip, artnet.Port)

		done := make(chan struct{})
		go func() {
			select {
			case <-e.rebind:
			case <-e.ctx.Done():
			case <-done:
			}
			conn.Close()
		}()
		buf := make([]byte, 2048)
		for {
			n, addr, err := conn.ReadFromUDP(buf)
			if err != nil {
				break
			}
			e.handlePacket(conn, addr, buf[:n])
		}
		close(done)
		e.mu.Lock()
		e.conn, e.listening = nil, false
		e.mu.Unlock()
	}
}

func (e *Engine) handlePacket(conn *net.UDPConn, addr *net.UDPAddr, b []byte) {
	op, ok := artnet.OpCode(b)
	if !ok {
		return
	}
	switch op {
	case artnet.OpPoll:
		e.replyPoll(conn, addr)
	case artnet.OpDmx:
		p, ok := artnet.ParseDMX(b)
		if !ok {
			return
		}
		src := addr.IP.String()
		now := time.Now()
		e.mu.Lock()
		defer e.mu.Unlock()
		s := e.sources[src]
		if s == nil {
			s = &source{}
			e.sources[src] = s
			log.Printf("Art-Net: nuevo emisor %s", src)
		}
		s.lastSeen = now
		s.count++
		if e.cfg.SourceIP != "" && e.cfg.SourceIP != src {
			return
		}
		u := e.unis[p.Universe]
		if u == nil {
			u = &universe{}
			e.unis[p.Universe] = u
		}
		copy(u.data[:], p.Data)
		u.lastSeen, u.source = now, src
		u.count++
	}
}

func (e *Engine) replyPoll(conn *net.UDPConn, addr *net.UDPAddr) {
	e.mu.Lock()
	var unis []uint16
	for _, d := range e.devs {
		if !d.running {
			continue
		}
		if dc := e.cfg.Devices[d.id]; dc != nil {
			for _, u := range dc.Universes {
				unis = append(unis, uint16(u))
			}
		}
	}
	e.mu.Unlock()
	sort.Slice(unis, func(i, j int) bool { return unis[i] < unis[j] })
	ip, mac := artnet.LocalIPFor(addr.IP)
	dst := &net.UDPAddr{IP: addr.IP, Port: artnet.Port}
	for _, r := range artnet.BuildPollReplies(ip, mac, "LJ Buddy", "LJ Buddy Art-Net a DMX", unis) {
		_, _ = conn.WriteToUDP(r, dst)
	}
}

// ------------------------------------------------------------ Interfaces

func (e *Engine) scanLoop() {
	defer e.wg.Done()
	for {
		e.scan()
		select {
		case <-e.ctx.Done():
			e.mu.Lock()
			for _, d := range e.devs {
				if d.cancel != nil {
					d.cancel()
				}
			}
			e.mu.Unlock()
			return
		case <-e.rescan:
		case <-time.After(2 * time.Second):
		}
	}
}

func (e *Engine) scan() {
	ports, err := listPorts()
	for i := 1; i <= e.sim; i++ {
		ports = append(ports, portInfo{
			Name: fmt.Sprintf("SIM%d", i), IsUSB: true, VID: ftdiVID, PID: "6001",
			Serial: fmt.Sprintf("SIM%05d", i), Product: "Interfaz simulada", sim: true,
		})
	}
	ids := deviceIDs(ports)

	e.mu.Lock()
	if err != nil {
		e.scanErr = err.Error()
	} else {
		e.scanErr = ""
	}
	manual := map[string]bool{}
	for _, n := range e.cfg.ManualPorts {
		manual[n] = true
	}
	seen := map[string]bool{}
	var others []portInfo
	var toStart []*device
	for _, p := range ports {
		if !p.isFTDI() && !manual[p.Name] {
			others = append(others, p)
			continue
		}
		id := ids[p.Name]
		seen[id] = true
		d := e.devs[id]
		if d == nil {
			d = &device{id: id}
			e.devs[id] = d
			log.Printf("Interfaz detectada: %s en %s", describe(p), p.Name)
		}
		d.port, d.present = p, true
		dc := e.cfg.Devices[id]
		if !d.running && (dc == nil || dc.Enabled) {
			toStart = append(toStart, d)
		}
	}
	for id, d := range e.devs {
		if seen[id] {
			continue
		}
		if d.present {
			log.Printf("Interfaz desconectada: %s", d.port.Name)
		}
		d.present = false
		if d.cancel != nil {
			d.cancel()
		}
		if _, known := e.cfg.Devices[id]; !known {
			delete(e.devs, id)
		}
	}
	// Interfaces conocidas que no están enchufadas: se listan como ausentes.
	for id := range e.cfg.Devices {
		if e.devs[id] == nil {
			e.devs[id] = &device{id: id}
		}
	}
	e.others = others
	e.mu.Unlock()

	for _, d := range toStart {
		e.startDevice(d)
	}
}

func describe(p portInfo) string {
	if p.Product != "" {
		return p.Product
	}
	if p.IsUSB {
		return fmt.Sprintf("USB %s:%s", p.VID, p.PID)
	}
	return "puerto serie"
}

// startDevice abre el puerto, averigua qué tipo de interfaz es y lanza su
// driver. Se llama sin el mutex tomado porque el sondeo tarda.
func (e *Engine) startDevice(d *device) {
	e.mu.Lock()
	p := d.port
	forced := ""
	if dc := e.cfg.Devices[d.id]; dc != nil {
		forced = dc.Kind
	}
	e.mu.Unlock()

	var drv dmx.Driver
	var port dmx.Port
	detail := ""
	if p.sim {
		drv = dmx.NewSim(1)
	} else {
		var err error
		port, err = openPort(p.Name)
		if err != nil {
			e.mu.Lock()
			msg := "No se pudo abrir " + p.Name + ": " + err.Error()
			if d.err != msg {
				log.Print(msg)
			}
			d.err = msg
			e.mu.Unlock()
			return
		}
		switch forced {
		case string(dmx.KindOpenDMX):
			drv = dmx.NewOpenDMX(port)
		default:
			info, isPro := dmx.ProbePro(port, 400*time.Millisecond)
			switch {
			case isPro && info.IsUltraPro():
				drv = dmx.NewPro(port, dmx.KindUltraPro)
			case isPro || forced == string(dmx.KindPro):
				drv = dmx.NewPro(port, dmx.KindPro)
			default:
				drv = dmx.NewOpenDMX(port)
			}
			if isPro {
				detail = proDetail(info)
			}
		}
	}

	ctx, cancel := context.WithCancel(e.ctx)
	e.mu.Lock()
	d.kind, d.outputs, d.detail = drv.Kind(), drv.Outputs(), detail
	d.running, d.err, d.cancel = true, "", cancel
	d.sent = make([]uint64, d.outputs)
	d.lastCnt = make([]uint64, d.outputs)
	d.fps = make([]int, d.outputs)
	dc := e.ensureConfigLocked(d)
	log.Printf("%s (%s) lista: %s → universos %v", p.Name, describe(p), d.kind.Label(), dc.Universes)
	e.mu.Unlock()

	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		err := drv.Run(ctx,
			func(i int) dmx.Frame { return e.frameFor(d, i) },
			func(i int) {
				e.mu.Lock()
				d.sent[i]++
				e.mu.Unlock()
			})
		cancel()
		if port != nil {
			port.Close()
		}
		e.mu.Lock()
		d.running, d.cancel, d.test = false, nil, false
		if err != nil {
			d.err = "Error de transmisión: " + err.Error()
			log.Printf("%s: %v", p.Name, err)
		}
		e.mu.Unlock()
	}()
}

func proDetail(i dmx.ProInfo) string {
	s := ""
	if i.Manufacturer != "" {
		s = i.Manufacturer + " "
	}
	if i.Device != "" {
		s += i.Device + " "
	} else if i.HardwareVer >= 2 {
		s += "DMX USB Pro Mk2 "
	}
	return s + "N/S " + i.Serial
}

// ensureConfigLocked crea o completa la configuración de una interfaz,
// asignando a cada salida el primer universo libre.
func (e *Engine) ensureConfigLocked(d *device) *DeviceConfig {
	dc := e.cfg.Devices[d.id]
	changed := false
	if dc == nil {
		dc = &DeviceConfig{Enabled: true}
		e.cfg.Devices[d.id] = dc
		changed = true
	}
	if name := describe(d.port); dc.Name != name && d.port.Name != "" {
		dc.Name = name
		changed = true
	}
	if len(dc.Universes) > d.outputs {
		dc.Universes = dc.Universes[:d.outputs]
		changed = true
	}
	for len(dc.Universes) < d.outputs {
		used := map[int]bool{}
		for _, o := range e.cfg.Devices {
			for _, u := range o.Universes {
				used[u] = true
			}
		}
		u := e.cfg.FirstUniverse
		for used[u] && u < maxUniverse {
			u++
		}
		dc.Universes = append(dc.Universes, u)
		changed = true
	}
	if changed {
		e.saveLocked()
	}
	return dc
}

func (e *Engine) frameFor(d *device, i int) dmx.Frame {
	e.mu.Lock()
	defer e.mu.Unlock()
	var f dmx.Frame
	if d.test {
		for k := range f {
			f[k] = 255
		}
		return f
	}
	dc := e.cfg.Devices[d.id]
	if dc == nil || i >= len(dc.Universes) {
		return f
	}
	if u := e.unis[uint16(dc.Universes[i])]; u != nil {
		return u.data
	}
	return f
}

func (e *Engine) statsLoop() {
	defer e.wg.Done()
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-e.ctx.Done():
			return
		case <-t.C:
		}
		e.mu.Lock()
		for _, u := range e.unis {
			u.pps = int(u.count - u.lastCnt)
			u.lastCnt = u.count
		}
		for _, d := range e.devs {
			for i := range d.sent {
				d.fps[i] = int(d.sent[i] - d.lastCnt[i])
				d.lastCnt[i] = d.sent[i]
			}
		}
		for ip, s := range e.sources {
			if time.Since(s.lastSeen) > time.Minute {
				delete(e.sources, ip)
			}
		}
		e.mu.Unlock()
	}
}
