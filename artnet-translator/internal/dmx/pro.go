package dmx

import (
	"context"
	"time"
)

// Protocolo "DMX USB Pro" de Enttec: 0x7E, etiqueta, largo (LSB, MSB),
// datos, 0xE7. Lo hablan también DMXking y muchos otros fabricantes.
const (
	proSOM = 0x7e
	proEOM = 0xe7

	labelSendDMX      = 6
	labelSerial       = 10
	labelHardwareVer  = 14
	labelManufacturer = 77
	labelDeviceName   = 78
	labelUltraPort1   = 100
	labelUltraPort2   = 101

	estaDMXking    = 0x6a6b
	devUltraDMXPro = 2
)

// ProMessage arma un mensaje del protocolo Pro.
func ProMessage(label byte, data []byte) []byte {
	b := make([]byte, 0, len(data)+5)
	b = append(b, proSOM, label, byte(len(data)), byte(len(data)>>8))
	b = append(b, data...)
	return append(b, proEOM)
}

// Pro maneja interfaces con microcontrolador propio: se les envía el frame y
// ellas generan la señal DMX con su temporización.
type Pro struct {
	p      Port
	kind   Kind
	labels []byte // etiqueta de envío por salida
}

func NewPro(p Port, kind Kind) *Pro {
	if kind == KindUltraPro {
		return &Pro{p: p, kind: kind, labels: []byte{labelUltraPort1, labelUltraPort2}}
	}
	return &Pro{p: p, kind: KindPro, labels: []byte{labelSendDMX}}
}

func (d *Pro) Kind() Kind   { return d.kind }
func (d *Pro) Outputs() int { return len(d.labels) }

func (d *Pro) Run(ctx context.Context, get func(int) Frame, sent func(int)) error {
	const period = 25 * time.Millisecond // 40 Hz máximo
	last := make([]Frame, len(d.labels))
	lastTx := make([]time.Time, len(d.labels))
	payload := make([]byte, 513) // payload[0] = start code 0
	for ctx.Err() == nil {
		start := time.Now()
		for i, label := range d.labels {
			f := get(i)
			// Solo se reenvía si cambió, con un refresco por segundo.
			if f == last[i] && time.Since(lastTx[i]) < time.Second {
				continue
			}
			copy(payload[1:], f[:])
			if _, err := d.p.Write(ProMessage(label, payload)); err != nil {
				return err
			}
			last[i], lastTx[i] = f, time.Now()
			sent(i)
		}
		if !sleepCtx(ctx, period-time.Since(start)) {
			break
		}
	}
	return nil
}

// ProInfo es lo que una interfaz Pro informa de sí misma.
type ProInfo struct {
	Serial       string
	Manufacturer string
	Device       string
	ESTAID       uint16
	DeviceID     uint16
	HardwareVer  byte
}

// ProbePro pregunta a la interfaz si habla el protocolo Pro. Una Open DMX no
// tiene canal de retorno, así que el silencio significa "no es Pro".
func ProbePro(p Port, wait time.Duration) (ProInfo, bool) {
	var info ProInfo
	_ = p.ResetInputBuffer()
	for _, l := range []byte{labelManufacturer, labelDeviceName, labelHardwareVer, labelSerial} {
		if _, err := p.Write(ProMessage(l, nil)); err != nil {
			return info, false
		}
	}
	_ = p.SetReadTimeout(40 * time.Millisecond)
	var buf []byte
	tmp := make([]byte, 1024)
	found := false
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		n, err := p.Read(tmp)
		if err != nil {
			break
		}
		buf = append(buf, tmp[:n]...)
		for {
			label, data, rest, ok := nextProMessage(buf)
			if !ok {
				break
			}
			buf = rest
			switch label {
			case labelSerial:
				if len(data) == 4 {
					info.Serial = bcdSerial(data)
					found = true
				}
			case labelManufacturer:
				if len(data) >= 2 {
					info.ESTAID = uint16(data[0]) | uint16(data[1])<<8
					info.Manufacturer = cstr(data[2:])
				}
			case labelDeviceName:
				if len(data) >= 2 {
					info.DeviceID = uint16(data[0]) | uint16(data[1])<<8
					info.Device = cstr(data[2:])
				}
			case labelHardwareVer:
				if len(data) >= 1 {
					info.HardwareVer = data[0]
				}
			}
		}
		// El número de serie es la última respuesta: no hace falta esperar más.
		if found && n == 0 {
			break
		}
	}
	return info, found
}

// IsUltraPro indica si es una DMXking ultraDMX Pro (dos salidas).
func (i ProInfo) IsUltraPro() bool {
	return i.ESTAID == estaDMXking && i.DeviceID == devUltraDMXPro
}

// nextProMessage extrae el primer mensaje completo de buf.
func nextProMessage(buf []byte) (label byte, data, rest []byte, ok bool) {
	for {
		i := 0
		for i < len(buf) && buf[i] != proSOM {
			i++
		}
		buf = buf[i:]
		if len(buf) < 5 {
			return 0, nil, buf, false
		}
		n := int(buf[2]) | int(buf[3])<<8
		if n > 600 {
			buf = buf[1:]
			continue
		}
		if len(buf) < n+5 {
			return 0, nil, buf, false
		}
		if buf[n+4] != proEOM {
			buf = buf[1:]
			continue
		}
		return buf[1], buf[4 : 4+n], buf[n+5:], true
	}
}

func bcdSerial(b []byte) string {
	const hex = "0123456789ABCDEF"
	out := make([]byte, 0, 8)
	for i := 3; i >= 0; i-- {
		out = append(out, hex[b[i]>>4], hex[b[i]&0x0f])
	}
	return string(out)
}

func cstr(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}
