// Package artnet implementa lo mínimo de Art-Net 4 que necesita un nodo de
// salida: recibir ArtDmx y responder ArtPoll con ArtPollReply.
package artnet

import (
	"net"
)

const (
	Port = 6454

	OpPoll      = 0x2000
	OpPollReply = 0x2100
	OpDmx       = 0x5000
)

var header = [8]byte{'A', 'r', 't', '-', 'N', 'e', 't', 0}

// OpCode devuelve el código de operación de un paquete Art-Net válido.
func OpCode(b []byte) (uint16, bool) {
	if len(b) < 10 || [8]byte(b[:8]) != header {
		return 0, false
	}
	return uint16(b[8]) | uint16(b[9])<<8, true
}

// DMXPacket es un ArtDmx decodificado. Universe es el Port-Address de 15 bits
// (Net << 8 | SubNet << 4 | Universe).
type DMXPacket struct {
	Universe uint16
	Sequence uint8
	Physical uint8
	Data     []byte
}

// ParseDMX decodifica un ArtDmx. Data apunta dentro de b.
func ParseDMX(b []byte) (DMXPacket, bool) {
	op, ok := OpCode(b)
	if !ok || op != OpDmx || len(b) < 18 {
		return DMXPacket{}, false
	}
	n := int(b[16])<<8 | int(b[17])
	if n > 512 {
		n = 512
	}
	if n > len(b)-18 {
		n = len(b) - 18
	}
	return DMXPacket{
		Universe: (uint16(b[15]&0x7f) << 8) | uint16(b[14]),
		Sequence: b[12],
		Physical: b[13],
		Data:     b[18 : 18+n],
	}, true
}

// BuildDMX arma un ArtDmx (se usa en pruebas y en el generador de prueba).
func BuildDMX(universe uint16, seq uint8, data []byte) []byte {
	n := len(data)
	if n > 512 {
		n = 512
	}
	if n%2 == 1 {
		n++
	}
	b := make([]byte, 18+n)
	copy(b, header[:])
	b[8], b[9] = 0x00, 0x50
	b[10], b[11] = 0, 14
	b[12] = seq
	b[14] = byte(universe)
	b[15] = byte(universe>>8) & 0x7f
	b[16], b[17] = byte(n>>8), byte(n)
	copy(b[18:], data)
	return b
}

// BuildPollReplies arma los ArtPollReply que anuncian las salidas indicadas.
// Cada respuesta admite hasta 4 puertos que comparten Net y SubNet, así que
// se generan tantas como hagan falta, numeradas con BindIndex.
func BuildPollReplies(ip net.IP, mac net.HardwareAddr, shortName, longName string, universes []uint16) [][]byte {
	type group struct {
		netSub uint16
		unis   []uint16
	}
	var groups []group
	for _, u := range universes {
		ns := u >> 4
		placed := false
		for i := range groups {
			if groups[i].netSub == ns && len(groups[i].unis) < 4 {
				groups[i].unis = append(groups[i].unis, u)
				placed = true
				break
			}
		}
		if !placed {
			groups = append(groups, group{ns, []uint16{u}})
		}
	}
	if len(groups) == 0 {
		groups = append(groups, group{})
	}
	ip4 := ip.To4()
	if ip4 == nil {
		ip4 = net.IPv4zero.To4()
	}
	var out [][]byte
	for gi, g := range groups {
		b := make([]byte, 239)
		copy(b, header[:])
		b[8], b[9] = 0x00, 0x21
		copy(b[10:14], ip4)
		b[14], b[15] = 0x36, 0x19 // puerto 6454, little endian
		b[16], b[17] = 1, 0       // versión de firmware
		b[18] = byte(g.netSub>>4) & 0x7f
		b[19] = byte(g.netSub) & 0x0f
		b[20], b[21] = 0x00, 0xff // OEM desconocido
		b[23] = 0xd0              // indicadores normales, dirección por red
		copy(b[26:43], shortName) // 18 bytes con terminador
		copy(b[44:107], longName) // 64 bytes con terminador
		copy(b[108:171], "#0001 [0000] LJ Buddy ArtNet Translator OK")
		b[173] = byte(len(g.unis))
		for i, u := range g.unis {
			b[174+i] = 0x80 // salida DMX512
			b[182+i] = 0x80 // transmitiendo datos
			b[190+i] = byte(u) & 0x0f
		}
		b[200] = 0x00 // StNode
		if len(mac) == 6 {
			copy(b[201:207], mac)
		}
		copy(b[207:211], ip4)
		b[211] = byte(gi + 1) // BindIndex
		b[212] = 0x08         // soporta Port-Address de 15 bits
		out = append(out, b)
	}
	return out
}
