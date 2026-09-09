//go:build !windows

package ingest

import (
	"os"
	"testing"
)

// bloquear simula el archivo agarrado quitándole el permiso de lectura: en
// Unix no hay bloqueo obligatorio de archivos, y esto es lo más parecido a
// «alguien lo tiene y no se puede abrir» (F1-01).
func bloquear(t *testing.T, path string) (soltar func()) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("como root todo archivo se abre: este caso no se puede montar")
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	return func() { _ = os.Chmod(path, 0o644) }
}
