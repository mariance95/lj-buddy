package engine

import (
	"bytes"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/mariance95/lj-buddy/internal/artnet"
	"github.com/mariance95/lj-buddy/internal/dmx"
)

// fakePort simula una interfaz: si ultra es true responde como una DMXking
// ultraDMX Pro; si no, queda muda como una Open DMX.
type fakePort struct {
	mu    sync.Mutex
	ultra bool
	rx    bytes.Buffer
	last  map[byte][]byte // último payload por etiqueta Pro
	open  []byte          // última trama cruda (Open DMX)
}

func (f *fakePort) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.ultra {
		f.open = append([]byte(nil), p...)
		return len(p), nil
	}
	label := p[1]
	switch label {
	case 10:
		f.rx.Write(dmx.ProMessage(10, []byte{1, 2, 3, 4}))
	case 77:
		f.rx.Write(dmx.ProMessage(77, append([]byte{0x6b, 0x6a}, "DMXking"...)))
	case 78:
		f.rx.Write(dmx.ProMessage(78, append([]byte{2, 0}, "ultraDMX Pro"...)))
	default:
		f.last[label] = append([]byte(nil), p[4:len(p)-1]...)
	}
	return len(p), nil
}
func (f *fakePort) Read(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.rx.Len() == 0 {
		return 0, nil
	}
	return f.rx.Read(p)
}
func (*fakePort) Break(time.Duration) error          { return nil }
func (*fakePort) Drain() error                       { return nil }
func (*fakePort) SetReadTimeout(time.Duration) error { return nil }
func (*fakePort) ResetInputBuffer() error            { return nil }
func (*fakePort) Close() error                       { return nil }

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for end := time.Now().Add(3 * time.Second); time.Now().Before(end); time.Sleep(10 * time.Millisecond) {
		if cond() {
			return
		}
	}
	t.Fatalf("no ocurrió: %s", what)
}

func TestAutoAssignAndRouting(t *testing.T) {
	ports := map[string]*fakePort{
		"COM3": {},
		"COM4": {ultra: true, last: map[byte][]byte{}},
	}
	listPorts = func() ([]portInfo, error) {
		return []portInfo{
			{Name: "COM3", IsUSB: true, VID: "0403", PID: "6001", Serial: "AL01"},
			{Name: "COM4", IsUSB: true, VID: "0403", PID: "6001", Serial: "KING"},
			{Name: "COM9", IsUSB: true, VID: "1A86", PID: "7523"},
		}, nil
	}
	openPort = func(name string) (dmx.Port, error) { return ports[name], nil }

	e := New("", 0)
	e.cfg.ListenIP = "127.0.0.1"
	e.Start()
	defer e.Stop()

	waitFor(t, "ambas interfaces transmitiendo", func() bool {
		s := e.Snapshot()
		return len(s.Devices) == 2 && s.Devices[0].Running && s.Devices[1].Running
	})
	s := e.Snapshot()
	if s.Devices[0].Kind != "opendmx" || s.Devices[0].Outputs[0].Universe != 0 {
		t.Fatalf("Open DMX mal asignada: %+v", s.Devices[0])
	}
	d := s.Devices[1]
	if d.Kind != "ultrapro" || len(d.Outputs) != 2 || d.Outputs[0].Universe != 1 || d.Outputs[1].Universe != 2 {
		t.Fatalf("ultraDMX mal asignada: %+v", d)
	}
	if len(s.OtherPorts) != 1 || s.OtherPorts[0].Name != "COM9" {
		t.Fatalf("puertos no FTDI: %+v", s.OtherPorts)
	}

	// Un universo distinto a cada salida.
	from := &net.UDPAddr{IP: net.IPv4(10, 0, 0, 9), Port: 6454}
	for u := 0; u < 3; u++ {
		data := bytes.Repeat([]byte{byte(10 * (u + 1))}, 512)
		e.handlePacket(nil, from, artnet.BuildDMX(uint16(u), 0, data))
	}
	waitFor(t, "cada universo en su salida", func() bool {
		a, b := ports["COM3"], ports["COM4"]
		a.mu.Lock()
		defer a.mu.Unlock()
		b.mu.Lock()
		defer b.mu.Unlock()
		return len(a.open) == 513 && a.open[1] == 10 && a.open[512] == 10 &&
			len(b.last[100]) == 513 && b.last[100][1] == 20 &&
			len(b.last[101]) == 513 && b.last[101][512] == 30
	})

	// Reasignación en caliente: la Open DMX pasa a emitir el universo 2.
	if err := e.UpdateDevice(s.Devices[0].ID, DeviceUpdate{Universes: []int{2}}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "reasignación", func() bool {
		a := ports["COM3"]
		a.mu.Lock()
		defer a.mu.Unlock()
		return a.open[1] == 30
	})

	// Filtro por IP de origen: los datos de otro emisor se ignoran.
	if err := e.SetNetwork("127.0.0.1", "10.0.0.50", 0); err != nil {
		t.Fatal(err)
	}
	e.handlePacket(nil, from, artnet.BuildDMX(2, 0, bytes.Repeat([]byte{200}, 512)))
	if e.Levels(2)[0] != 0 {
		t.Fatal("el filtro de origen no se aplicó")
	}
}
