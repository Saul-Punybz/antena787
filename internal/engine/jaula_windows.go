//go:build windows

package engine

import (
	"os/exec"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// La jaula: que ningún ffmpeg sobreviva a Antena787.
//
// En Unix, un proceso hijo se muere cuando su padre se muere. **En Windows
// no.** Si `antena.exe` se va de golpe —alguien cierra la ventana negra con
// la X, lo mata desde el Administrador de tareas, se cae la luz, un pánico—
// todos los ffmpeg que había lanzados **siguen corriendo para siempre**.
//
// En una torre desatendida eso no se nota el primer día: se nota al mes,
// cuando hay veinte ffmpeg comiéndose la máquina y el canal va a tirones sin
// que nada lo explique. Lo encontró la auditoría de Windows del 12 de
// septiembre de 2026.
//
// La forma de arreglarlo es un **Job Object** con
// `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`: todo proceso metido ahí muere cuando
// el trabajo se cierra, y el trabajo se cierra cuando el proceso que lo tiene
// abierto desaparece — pase lo que pase, incluido un kill -9. Es Windows quien
// lo garantiza, no nosotros: no hay código que tenga que llegar a correr.
//
// Va sin CGo: golang.org/x/sys/windows llama al sistema por syscall.
var (
	unaVez sync.Once
	jaula  windows.Handle
)

// laJaula abre el trabajo la primera vez que hace falta.
func laJaula() windows.Handle {
	unaVez.Do(func() {
		h, err := windows.CreateJobObject(nil, nil)
		if err != nil {
			return
		}
		info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
			BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
				LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
			},
		}
		if _, err := windows.SetInformationJobObject(h,
			windows.JobObjectExtendedLimitInformation,
			uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
			_ = windows.CloseHandle(h)
			return
		}
		jaula = h
	})
	return jaula
}

// enjaular mete un proceso ya arrancado en la jaula. Que falle no puede
// impedir que el canal emita: se pierde la garantía de limpieza, no el aire.
func enjaular(cmd *exec.Cmd) {
	h := laJaula()
	if h == 0 || cmd == nil || cmd.Process == nil {
		return
	}
	p, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE,
		false, uint32(cmd.Process.Pid))
	if err != nil {
		return
	}
	defer func() { _ = windows.CloseHandle(p) }()
	_ = windows.AssignProcessToJobObject(h, p)
}
