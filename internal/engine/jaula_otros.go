//go:build !windows

package engine

import "os/exec"

// Fuera de Windows no hace falta jaula: un hijo se muere con su padre, o lo
// recoge init y no se queda comiendo máquina. Ver jaula_windows.go para por
// qué allá sí hace falta.
func enjaular(*exec.Cmd) {}
