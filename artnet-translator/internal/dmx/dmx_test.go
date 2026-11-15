package dmx

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// fakePort registra lo que escribe el driver y responde como una interfaz.
type fakePort struct {
	mu      sync.Mutex
	events  []string // "break" o "write:N"
	written bytes.Buffer
	rx      bytes.Buffer
	respond func(label byte) []byte
	failAt  int
}

func (f *fakePort) Read(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.rx.Len() == 0 {
		return 0, nil
	}
	return f.rx.Read(p)
}

func (f *fakePort) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failAt > 0 && len(f.events) >= f.failAt {
		return 0, errors.New("desconectado")
	}
	f.events = append(f.events, "write")
	f.written.Write(p)
	if f.respond != nil && len(p) >= 5 && p[0] == proSOM {
		f.rx.Write(f.respond(p[1]))
	}
	return len(p), nil
}

func (f *fakePort) Break(time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, "break")
	return nil
}
func (*fakePort) Drain() error                       { return nil }
func (*fakePort) SetReadTimeout(time.Duration) error { return nil }
func (*fakePort) ResetInputBuffer() error            { return nil }
func (*fakePort) Close() error                       { return nil }

func TestOpenDMXFraming(t *testing.T) {
	p := &fakePort{failAt: 6}
	var f Frame
	f[0], f[511] = 11, 22
	n := 0
	err := NewOpenDMX(p).Run(context.Background(), func(int) Frame { return f }, func(int) { n++ })
	if err == nil {
		t.Fatal("debía fallar al desconectarse")
	}
	if n != 3 {
		t.Fatalf("frames enviados: %d", n)
	}
	want := []string{"break", "write", "break", "write", "break", "write", "break"}
	if len(p.events) != len(want) {
		t.Fatalf("eventos: %v", p.events)
	}
	for i := range want {
		if p.events[i] != want[i] {
			t.Fatalf("eventos: %v", p.events)
		}
	}
	b := p.written.Bytes()
	if len(b) != 3*513 || b[0] != 0 || b[1] != 11 || b[512] != 22 {
		t.Fatalf("trama incorrecta: len=%d", len(b))
	}
}

func TestProSendsOnChange(t *testing.T) {
	p := &fakePort{}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	var f Frame
	f[0] = 99
	sent := map[int]int{}
	if err := NewPro(p, KindUltraPro).Run(ctx, func(int) Frame { return f }, func(i int) { sent[i]++ }); err != nil {
		t.Fatal(err)
	}
	// Sin cambios, un solo envío por salida.
	if sent[0] != 1 || sent[1] != 1 {
		t.Fatalf("envíos: %v", sent)
	}
	b := p.written.Bytes()
	if len(b) != 2*518 {
		t.Fatalf("largo %d", len(b))
	}
	m := b[:518]
	if m[0] != 0x7e || m[1] != 100 || m[2] != 0x01 || m[3] != 0x02 || m[4] != 0 || m[5] != 99 || m[517] != 0xe7 {
		t.Fatalf("mensaje incorrecto: % x ...", m[:8])
	}
	if b[518+1] != 101 {
		t.Fatal("segunda salida sin etiqueta 101")
	}
}

func TestProbe(t *testing.T) {
	silent := &fakePort{}
	if _, ok := ProbePro(silent, 100*time.Millisecond); ok {
		t.Fatal("una interfaz muda no es Pro")
	}

	pro := &fakePort{respond: func(l byte) []byte {
		if l == labelSerial {
			return append([]byte{0x55, 0x00}, ProMessage(labelSerial, []byte{0x78, 0x56, 0x34, 0x12})...)
		}
		return nil
	}}
	info, ok := ProbePro(pro, 200*time.Millisecond)
	if !ok || info.Serial != "12345678" || info.IsUltraPro() {
		t.Fatalf("pro: %+v %v", info, ok)
	}

	ultra := &fakePort{respond: func(l byte) []byte {
		switch l {
		case labelSerial:
			return ProMessage(l, []byte{1, 0, 0, 0})
		case labelManufacturer:
			return ProMessage(l, append([]byte{0x6b, 0x6a}, "DMXking\x00"...))
		case labelDeviceName:
			return ProMessage(l, append([]byte{2, 0}, "ultraDMX Pro"...))
		}
		return nil
	}}
	info, ok = ProbePro(ultra, 200*time.Millisecond)
	if !ok || !info.IsUltraPro() || info.Manufacturer != "DMXking" || info.Device != "ultraDMX Pro" {
		t.Fatalf("ultra: %+v %v", info, ok)
	}
}
