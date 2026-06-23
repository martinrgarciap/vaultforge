package health

import (
	"context"
	"errors"
)

type Pinger interface {
	Ping(context.Context) error
}

type ReadinessPinger struct {
	dependencies []Pinger
}

func NewReadinessPinger(dependencies ...Pinger) *ReadinessPinger {
	return &ReadinessPinger{
		dependencies: append([]Pinger(nil), dependencies...),
	}
}

func (pinger *ReadinessPinger) Ping(ctx context.Context) error {
	if pinger == nil || len(pinger.dependencies) == 0 {
		return errors.New("readiness dependencies unavailable")
	}

	for _, dependency := range pinger.dependencies {
		if dependency == nil {
			return errors.New("readiness dependency unavailable")
		}

		if err := dependency.Ping(ctx); err != nil {
			return errors.New("readiness dependency unavailable")
		}
	}

	return nil
}
