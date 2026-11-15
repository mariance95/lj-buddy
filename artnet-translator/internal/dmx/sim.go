package dmx

import (
	"context"
	"time"
)

// Sim es una interfaz simulada para probar el programa sin hardware.
type Sim struct{ outputs int }

func NewSim(outputs int) *Sim { return &Sim{outputs} }

func (*Sim) Kind() Kind     { return KindSim }
func (s *Sim) Outputs() int { return s.outputs }

func (s *Sim) Run(ctx context.Context, get func(int) Frame, sent func(int)) error {
	for sleepCtx(ctx, 25*time.Millisecond) {
		for i := 0; i < s.outputs; i++ {
			_ = get(i)
			sent(i)
		}
	}
	return nil
}
