// Package ingest es el paso 1 del PRD (§9): entra un archivo, se mide, se
// mira si trae negro y silencio, se le saca una miniatura, se le busca la
// ficha y la carátula, y queda listo para que alguien lo programe. La
// normalización al formato de casa no ocurre aquí: se encola y corre después
// en segundo plano, priorizada por la hora a la que el material sale al aire
// (AUDITORIA B7).
//
// Reglas que este paquete respeta y que no se negocian:
//
//   - Nada de estado global: todo entra por parámetros y todo lleva
//     context.Context.
//   - Los motivos que ve una persona van en español y sin jerga; viven en
//     PlainError.Reason y terminan en MediaAsset.PlainReason.
//   - Local primero, red al final. Los proveedores de red vienen apagados.
//   - Un error de lectura pasajero —el NAS que se reinicia, el antivirus que
//     tenía el archivo un segundo— no manda nada a cuarentena: se reintenta
//     una vez pasados cinco minutos (AUDITORIA C8).
package ingest

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Los códigos de motivo. El motivo en cristiano es para la persona; el
// código es para el programa, que a veces tiene que hacer algo distinto
// según por qué se paró un archivo. Hoy hay uno solo: el material sin
// sonido, que no tiene botón de «dejarlo pasar» porque todo lo que sale al
// aire lleva audio (F1-59).
const MotivoSinAudio = "sin_audio"

// PlainError es un error con su motivo escrito para una persona que no es
// técnica. Reason es lo que se muestra en pantalla y lo que se guarda en
// motivo_en_cristiano; Code es el código de motivo —vacío en casi todos— y
// Err es el detalle para el registro.
type PlainError struct {
	Reason string
	Code   string
	Err    error
}

func (e *PlainError) Error() string {
	if e.Err == nil {
		return e.Reason
	}
	return e.Reason + " (" + e.Err.Error() + ")"
}

func (e *PlainError) Unwrap() error { return e.Err }

// Plainf arma un PlainError con el motivo ya formateado.
func Plainf(cause error, format string, args ...any) *PlainError {
	return &PlainError{Reason: fmt.Sprintf(format, args...), Err: cause}
}

// Plainc arma un PlainError con su código de motivo puesto.
func Plainc(cause error, code, format string, args ...any) *PlainError {
	return &PlainError{Reason: fmt.Sprintf(format, args...), Code: code, Err: cause}
}

// Motivo devuelve el código de motivo de un error, o cadena vacía si no
// lleva ninguno.
func Motivo(err error) string {
	if err == nil {
		return ""
	}
	var p *PlainError
	if errors.As(err, &p) {
		return p.Code
	}
	return ""
}

// EsSinAudio dice si el archivo se paró por no traer sonido. Es lo que mira
// quien pinta el botón de «dejarlo pasar bajo mi responsabilidad»: en este
// caso no se pinta, porque no hay forma de sacar al aire algo mudo (F1-59).
func EsSinAudio(err error) bool { return Motivo(err) == MotivoSinAudio }

// TextoSinAudio reconoce el motivo de «no trae sonido» ya guardado en
// motivo_en_cristiano. Hace falta porque después de reiniciar solo queda el
// texto: el código del error no se guarda en ninguna columna.
func TextoSinAudio(reason string) bool {
	return strings.Contains(reason, "no trae sonido") && strings.Contains(reason, "pon a su lado")
}

// Plain devuelve el motivo en cristiano de un error, o su texto si el error
// no trae uno. Sirve para llenar motivo_en_cristiano sin preguntar tipos.
func Plain(err error) string {
	if err == nil {
		return ""
	}
	var p *PlainError
	if errors.As(err, &p) {
		return p.Reason
	}
	return err.Error()
}

// ── extensiones que el ingest reconoce ────────────────────────────────

// VideoExtensions y AudioExtensions son lo que la carpeta vigilada levanta.
// Todo en minúsculas y con el punto.
var (
	VideoExtensions = []string{".mp4", ".mkv", ".mov", ".ts", ".mpg", ".mpeg", ".m4v", ".avi"}
	AudioExtensions = []string{".mp3", ".flac", ".wav", ".m4a", ".aac", ".ogg"}
)

// MediaExtensions es la unión de las dos, que es lo que vigila el Watcher.
func MediaExtensions() []string {
	out := make([]string, 0, len(VideoExtensions)+len(AudioExtensions))
	out = append(out, VideoExtensions...)
	return append(out, AudioExtensions...)
}

// IsMedia dice si el nombre tiene una extensión de medios conocida.
func IsMedia(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	for _, e := range MediaExtensions() {
		if e == ext {
			return true
		}
	}
	return false
}

// ── errores de lectura pasajeros ──────────────────────────────────────

// transientHints son los textos que delatan un fallo de lectura pasajero:
// el disco de red que se fue un momento, el antivirus que tenía el archivo
// abierto, el permiso que todavía no se había propagado. Ninguno de estos
// manda un archivo a cuarentena en el primer intento (AUDITORIA C8).
var transientHints = []string{
	"resource temporarily unavailable",
	"input/output error",
	"device not configured",
	"no such device",
	"transport endpoint",
	"stale file handle",
	"permission denied",
	"being used by another process",
	"access is denied",
	"the device is not ready",
	"network path",
	"connection reset",
	"timed out",
	"i/o timeout",
	"text file busy",
	"interrupted system call",
}

// IsTransient dice si un error huele a fallo de lectura pasajero.
func IsTransient(err error) bool {
	if err == nil {
		return false
	}
	// Que el archivo "no exista" también entra: cuando un disco de red se
	// reinicia, el sistema operativo dice exactamente eso. AUDITORIA C8 pide
	// dos fallos separados antes de dar nada por perdido.
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, os.ErrPermission) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	s := strings.ToLower(err.Error())
	for _, h := range transientHints {
		if strings.Contains(s, h) {
			return true
		}
	}
	return false
}

// RetryPolicy es cómo se reintenta un fallo de lectura pasajero. El default
// del PRD son cinco minutos y un solo reintento; las pruebas cambian Sleep
// para no esperar de verdad.
type RetryPolicy struct {
	Delay time.Duration
	Sleep func(ctx context.Context, d time.Duration) error
}

// DefaultRetry es la política del PRD: un reintento a los cinco minutos.
func DefaultRetry() RetryPolicy {
	return RetryPolicy{Delay: 5 * time.Minute, Sleep: SleepCtx}
}

// SleepCtx duerme sin ignorar la cancelación.
func SleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// retry corre fn y, solo si el fallo parece pasajero, la corre una segunda
// vez pasado el retardo. Dos fallos separados por el retardo ya son un fallo
// de verdad.
func (r RetryPolicy) retry(ctx context.Context, fn func() error) error {
	err := fn()
	if err == nil || !IsTransient(err) {
		return err
	}
	sleep := r.Sleep
	if sleep == nil {
		sleep = SleepCtx
	}
	if serr := sleep(ctx, r.Delay); serr != nil {
		return err
	}
	return fn()
}
