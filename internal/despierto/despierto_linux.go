//go:build linux

package despierto

import "context"

// SostenerCon es Sostener con las dos piezas del sistema inyectables, para
// las pruebas. En Linux, systemd-inhibit.
func SostenerCon(ctx context.Context, lanzar Lanzador, buscar Buscador) (soltar func(), caido <-chan error, err error) {
	return sostenerSystemdInhibit(ctx, lanzar, buscar)
}
