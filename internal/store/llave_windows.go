//go:build windows

package store

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// envolver ata la llave a esta cuenta de Windows con DPAPI, de modo que el
// archivo copiado a otra máquina o abierto por otro usuario sea basura. Es lo
// que el esquema prometía desde el principio (`credenciales BLOB -- cifradas
// (DPAPI / keyring)`) y nunca se había construido.
//
// Va sin CGo: golang.org/x/sys/windows llama a crypt32.dll por syscall, y ya
// era dependencia del proyecto.
func envolver(llave []byte) ([]byte, error) {
	entrada := windows.DataBlob{Size: uint32(len(llave)), Data: &llave[0]}
	var salida windows.DataBlob
	// CRYPTPROTECT_UI_FORBIDDEN: esto corre en un servicio que arranca solo
	// tras un apagón; no puede haber un cuadro de diálogo esperando a nadie.
	if err := windows.CryptProtectData(&entrada, nil, nil, 0, nil,
		windows.CRYPTPROTECT_UI_FORBIDDEN, &salida); err != nil {
		return nil, fmt.Errorf("Windows no pudo proteger la llave: %w", err)
	}
	defer func() { _, _ = windows.LocalFree(windows.Handle(unsafe.Pointer(salida.Data))) }()
	return copiaDeBlob(salida), nil
}

func desenvolver(guardada []byte) ([]byte, error) {
	if len(guardada) == 0 {
		return nil, fmt.Errorf("la llave está vacía")
	}
	entrada := windows.DataBlob{Size: uint32(len(guardada)), Data: &guardada[0]}
	var salida windows.DataBlob
	if err := windows.CryptUnprotectData(&entrada, nil, nil, 0, nil,
		windows.CRYPTPROTECT_UI_FORBIDDEN, &salida); err != nil {
		return nil, fmt.Errorf("Windows no pudo leer la llave: la hizo otra cuenta o otra máquina (%w)", err)
	}
	defer func() { _, _ = windows.LocalFree(windows.Handle(unsafe.Pointer(salida.Data))) }()
	return copiaDeBlob(salida), nil
}

// copiaDeBlob saca los bytes antes de que LocalFree suelte la memoria que
// Windows reservó: unsafe.Slice apunta a memoria de Windows, no de Go.
func copiaDeBlob(b windows.DataBlob) []byte {
	out := make([]byte, b.Size)
	copy(out, unsafe.Slice(b.Data, b.Size))
	return out
}
