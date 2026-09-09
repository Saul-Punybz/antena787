//go:build windows

package ingest

import (
	"syscall"
	"testing"
)

// bloquear deja el archivo agarrado como lo tiene el programa que lo está
// copiando: abierto sin compartir, así cualquier otro intento de abrirlo
// falla con «sharing violation». Es el caso real de Windows (F1-01).
func bloquear(t *testing.T, path string) (soltar func()) {
	t.Helper()
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := syscall.CreateFile(p, syscall.GENERIC_READ, 0, nil,
		syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatalf("no se pudo abrir sin compartir: %v", err)
	}
	return func() { _ = syscall.CloseHandle(h) }
}
