//go:build !windows

package app

import "syscall"

// espacioLibre devuelve cuánto queda libre y cuánto cabe en total en el
// sistema de archivos donde vive path, en bytes.
func espacioLibre(path string) (libre, total uint64, err error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0, err
	}
	tam := uint64(st.Bsize)
	return uint64(st.Bavail) * tam, uint64(st.Blocks) * tam, nil
}

// freePercent es el porcentaje libre del sistema de archivos donde vive path.
func freePercent(path string) (float64, error) {
	libre, total, err := espacioLibre(path)
	if err != nil {
		return 0, err
	}
	if total == 0 {
		return 0, nil
	}
	return float64(libre) / float64(total) * 100, nil
}
