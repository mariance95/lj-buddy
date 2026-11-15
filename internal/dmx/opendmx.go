package dmx

import (
	"context"
	"time"
)

// OpenDMX maneja interfaces sin microcontrolador (Enttec Open DMX USB y
// clones): el PC genera toda la trama DMX512 a 250000 baudios 8N2, y el
// BREAK inicial se produce con la señal de break del puerto serie.
type OpenDMX struct {
	p Port
}

func NewOpenDMX(p Port) *OpenDMX { return &OpenDMX{p: p} }

func (*OpenDMX) Kind() Kind   { return KindOpenDMX }
func (*OpenDMX) Outputs() int { return 1 }

const (
	openBreak = 1 * time.Millisecond // mínimo DMX512: 88 µs
	// 513 slots x 44 µs = 22,6 ms. Se deja margen para que el chip termine
	// de vaciar su buffer antes del siguiente BREAK (~36 Hz).
	openPeriod = 27 * time.Millisecond
)

func (o *OpenDMX) Run(ctx context.Context, get func(int) Frame, sent func(int)) error {
	buf := make([]byte, 513) // buf[0] = start code 0
	for ctx.Err() == nil {
		f := get(0)
		copy(buf[1:], f[:])
		if err := o.p.Break(openBreak); err != nil {
			return err
		}
		// El Mark After Break (mín. 12 µs) lo da el propio retardo de la
		// llamada siguiente.
		start := time.Now()
		if _, err := o.p.Write(buf); err != nil {
			return err
		}
		_ = o.p.Drain()
		sent(0)
		if !sleepCtx(ctx, openPeriod-time.Since(start)) {
			break
		}
	}
	return nil
}
