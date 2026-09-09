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

// espacioLibre devuelve cuánto queda libre y cuánto cabe en total en el
// volumen donde vive path, en bytes. Sin CGo: se llama a la API de Windows
// por el DLL, que es lo que hace el propio paquete syscall.
func espacioLibre(path string) (libre, total uint64, err error) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, err
	}
	var freeForCaller, bytesTotal, bytesFree uint64
	r, _, callErr := getDiskFreeSpaceEx.Call(
		uintptr(unsafe.Pointer(p)),
		uintptr(unsafe.Pointer(&freeForCaller)),
		uintptr(unsafe.Pointer(&bytesTotal)),
		uintptr(unsafe.Pointer(&bytesFree)),
	)
	if r == 0 {
		return 0, 0, callErr
	}
	return freeForCaller, bytesTotal, nil
}

// freePercent es el porcentaje libre del volumen donde vive path.
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
