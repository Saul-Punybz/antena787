//go:build !windows

package app

import "syscall"

// freePercent es el porcentaje libre del sistema de archivos donde vive path.
func freePercent(path string) (float64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}
	total := float64(st.Blocks) * float64(st.Bsize)
	if total <= 0 {
		return 0, nil
	}
	free := float64(st.Bavail) * float64(st.Bsize)
	return free / total * 100, nil
}
