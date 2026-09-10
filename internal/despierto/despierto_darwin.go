//go:build darwin

package despierto

import "context"

// SostenerCon es Sostener con las dos piezas del sistema inyectables, para
// las pruebas. En macOS, caffeinate.
func SostenerCon(ctx context.Context, lanzar Lanzador, buscar Buscador) (func(), error) {
	return sostenerCaffeinate(ctx, lanzar, buscar)
}
