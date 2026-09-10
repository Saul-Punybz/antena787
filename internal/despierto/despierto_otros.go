//go:build !darwin && !linux && !windows

package despierto

import (
	"context"
	"errors"
	"runtime"
)

// SostenerCon en un sistema que no es ninguno de los tres que Antena787
// soporta: se dice la verdad y no se finge que está sostenido.
func SostenerCon(_ context.Context, _ Lanzador, _ Buscador) (soltar func(), caido <-chan error, err error) {
	return nil, nil, errors.New("no sé cómo impedir que se duerma un sistema " + runtime.GOOS + ": apaga la suspensión por inactividad a mano")
}
