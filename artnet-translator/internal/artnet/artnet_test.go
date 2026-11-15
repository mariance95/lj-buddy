package artnet

import (
	"bytes"
	"net"
	"testing"
)

func TestDMXRoundTrip(t *testing.T) {
	data := make([]byte, 512)
	for i := range data {
		data[i] = byte(i)
	}
	p, ok := ParseDMX(BuildDMX(0x1234, 7, data))
	if !ok || p.Universe != 0x1234 || p.Sequence != 7 || !bytes.Equal(p.Data, data) {
		t.Fatalf("mal decodificado: %+v %v", p.Universe, ok)
	}
}

func TestParseRejectsGarbage(t *testing.T) {
	if _, ok := ParseDMX([]byte("hola")); ok {
		t.Fatal("aceptó basura")
	}
	b := BuildDMX(1, 0, make([]byte, 512))
	b[16], b[17] = 0xff, 0xff // longitud declarada mayor a la real
	if p, ok := ParseDMX(b[:100]); !ok || len(p.Data) != 82 {
		t.Fatalf("longitud no acotada: %d %v", len(p.Data), ok)
	}
}

func TestPollReplies(t *testing.T) {
	r := BuildPollReplies(net.IPv4(2, 0, 0, 5), nil, "LJ", "LJ Buddy", []uint16{0, 1, 2, 3, 4, 0x105})
	if len(r) != 3 {
		t.Fatalf("esperaba 3 respuestas, hay %d", len(r))
	}
	for i, b := range r {
		if op, ok := OpCode(b); !ok || op != OpPollReply || len(b) != 239 || b[211] != byte(i+1) {
			t.Fatalf("respuesta %d inválida", i)
		}
	}
	if r[0][173] != 4 || r[1][173] != 1 || r[1][190] != 4 {
		t.Fatal("agrupación de puertos incorrecta")
	}
	if r[2][18] != 1 || r[2][19] != 0 || r[2][190] != 5 {
		t.Fatal("Net/SubNet incorrectos")
	}
}
