// Package dmx contiene los drivers de salida para interfaces USB-DMX que se
// presentan como puerto serie (chip FTDI u otros).
package dmx

import (
	"context"
	"time"
)

// Kind identifica el protocolo que habla la interfaz.
type Kind string

const (
	KindOpenDMX  Kind = "opendmx"  // Enttec Open DMX USB y clones FTDI "tontos"
	KindPro      Kind = "pro"      // Enttec DMX USB Pro y compatibles
	KindUltraPro Kind = "ultrapro" // DMXking ultraDMX Pro (2 salidas)
	KindSim      Kind = "sim"      // interfaz simulada, sin hardware
)

// Frame son los 512 canales de un universo.
type Frame = [512]byte

// Port es lo que los drivers necesitan de un puerto serie.
type Port interface {
	Read(p []byte) (int, error)
	Write(p []byte) (int, error)
	Break(time.Duration) error
	Drain() error
	SetReadTimeout(time.Duration) error
	ResetInputBuffer() error
	Close() error
}

// Driver envía DMX por una interfaz con una o más salidas.
type Driver interface {
	Kind() Kind
	Outputs() int
	// Run transmite hasta que ctx termina o la interfaz falla. get devuelve el
	// frame actual de la salida i; sent se llama por cada frame enviado.
	Run(ctx context.Context, get func(i int) Frame, sent func(i int)) error
}

// Label devuelve el nombre legible de un tipo de interfaz.
func (k Kind) Label() string {
	switch k {
	case KindOpenDMX:
		return "Open DMX"
	case KindPro:
		return "DMX USB Pro"
	case KindUltraPro:
		return "DMXking ultraDMX Pro"
	case KindSim:
		return "Simulada"
	}
	return string(k)
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
