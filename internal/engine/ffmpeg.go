package engine

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// FFmpeg localiza el binario. El release lo coloca junto al ejecutable
// (ADR 0003); en desarrollo vale el del PATH o ANTENA_FFMPEG.
func FFmpeg() (string, error) { return findTool("ffmpeg") }

// FFprobe igual que FFmpeg.
func FFprobe() (string, error) { return findTool("ffprobe") }

func findTool(name string) (string, error) {
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if env := os.Getenv("ANTENA_FFMPEG"); env != "" {
		p := filepath.Join(env, name)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), name)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}
	return "", errors.New(name + " no está: ponlo junto al ejecutable, en el PATH, o en ANTENA_FFMPEG")
}

// ── lanzar procesos sin que se cuelguen en Windows ────────────────────

// PlazoDeCierre es lo que se espera a que un proceso muerto suelte sus
// tuberías antes de cerrarlas por la fuerza.
//
// Existe por el issue #14: en Windows, `cmd.Wait` se quedaba esperando para
// siempre a que ffmpeg soltara sus tuberías después de haberlo matado, y la
// F0 corta se colgaba entera. Con `WaitDelay`, `Wait` las cierra él mismo y
// sigue.
//
// Tres segundos: lo bastante para que un proceso que está terminando bien
// acabe, y lo bastante poco para que uno colgado no se lleve el aire por
// delante.
const PlazoDeCierre = 3 * time.Second

// Comando arma un proceso con el plazo de cierre ya puesto. **Úsalo siempre
// en vez de exec.Command y exec.CommandContext.**
//
// El arreglo del issue #14 se puso en un solo archivo —el decodificador— y no
// se generalizó, así que quince sitios más quedaron con el mismo cuelgue
// esperando (auditoría de Windows, 12 sept 2026). Esto lo generaliza: quien
// escriba el sitio dieciséis lo hereda sin tener que saber la historia.
func Comando(ctx context.Context, nombre string, args ...string) *exec.Cmd {
	var cmd *exec.Cmd
	if ctx != nil {
		cmd = exec.CommandContext(ctx, nombre, args...)
	} else {
		cmd = exec.Command(nombre, args...)
	}
	cmd.WaitDelay = PlazoDeCierre
	// Nada de lo que se lanza aquí lee de la entrada, y dejarla abierta hace
	// que un ffmpeg despistado se quede esperando a que alguien escriba.
	cmd.Stdin = nil
	return cmd
}
