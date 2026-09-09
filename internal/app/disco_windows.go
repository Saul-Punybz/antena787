//go:build windows

package app

import (
	"syscall"
	"unsafe"
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	getDiskFreeSpaceEx = kernel32.NewProc("GetDiskFreeSpaceExW")
)

// freePercent es el porcentaje libre del volumen donde vive path. Sin CGo:
// se llama a la API de Windows por el DLL, que es lo que hace el propio
// paquete syscall.
func freePercent(path string) (float64, error) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var freeForCaller, total, free uint64
	r, _, callErr := getDiskFreeSpaceEx.Call(
		uintptr(unsafe.Pointer(p)),
		uintptr(unsafe.Pointer(&freeForCaller)),
		uintptr(unsafe.Pointer(&total)),
		uintptr(unsafe.Pointer(&free)),
	)
	if r == 0 {
		return 0, callErr
	}
	if total == 0 {
		return 0, nil
	}
	return float64(freeForCaller) / float64(total) * 100, nil
}
