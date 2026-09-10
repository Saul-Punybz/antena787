//go:build windows

package despierto

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"syscall"
)

// Las banderas de SetThreadExecutionState que hacen falta. ES_CONTINUOUS dice
// «esto vale hasta nuevo aviso» (y no «una vez»); ES_SYSTEM_REQUIRED, «no
// duermas la máquina». La pantalla sí puede apagarse: un servidor de playout
// no necesita el monitor encendido, y ES_DISPLAY_REQUIRED dejaría un
// televisor de estudio ardiendo toda la noche.
const (
	esSystemRequired = 0x00000001
	esContinuous     = 0x80000000
)

var (
	kernel32                    = syscall.NewLazyDLL("kernel32.dll")
	procSetThreadExecutionState = kernel32.NewProc("SetThreadExecutionState")
)

// SostenerCon en Windows no lanza ningún programa: se lo pide al sistema por
// kernel32.dll, sin CGo, como el resto de las llamadas de Windows de este
// repo. El lanzador y el buscador no se usan; están para que la firma sea la
// misma en los tres sistemas.
//
// La aserción es **por hilo**: quien la pone tiene que ser el mismo hilo que
// la suelta, y si ese hilo se muere la aserción se va con él. Por eso vive en
// una goroutine con runtime.LockOSThread pegada a su hilo del sistema de
// principio a fin.
func SostenerCon(ctx context.Context, _ Lanzador, _ Buscador) (func(), error) {
	listo := make(chan error, 1)
	suelta := make(chan struct{})

	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		r, _, err := procSetThreadExecutionState.Call(uintptr(esContinuous | esSystemRequired))
		if r == 0 {
			listo <- fmt.Errorf("Windows no aceptó la petición de no dormir (SetThreadExecutionState): %v", err)
			return
		}
		listo <- nil

		select {
		case <-suelta:
		case <-ctx.Done():
		}
		// Soltar es volver a ES_CONTINUOUS a secas: se queda el «hasta nuevo
		// aviso» sin la prohibición de dormir.
		_, _, _ = procSetThreadExecutionState.Call(uintptr(esContinuous))
	}()

	if err := <-listo; err != nil {
		return nil, err
	}
	var una sync.Once
	return func() { una.Do(func() { close(suelta) }) }, nil
}
