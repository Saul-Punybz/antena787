// Package engine es el motor de playout: decodificadores por clip, un
// servidor de cuadros en Go y un encoder persistente (PRD §14.1, ADR 0001).
// En F0 solo existe lo necesario para responder la pregunta del §22.1.
package engine

import (
	"fmt"
	"math"
)

// Format es el formato de casa: lo único que el encoder acepta.
type Format struct {
	Width, Height  int
	FPSNum, FPSDen int // 60000/1001 = 59.94
	SampleRate     int
	Channels       int
}

// CAtv es el formato de casa del despliegue de referencia (PRD §17).
var CAtv = Format{Width: 1280, Height: 720, FPSNum: 60000, FPSDen: 1001, SampleRate: 48000, Channels: 2}

// FrameBytes es el tamaño de un cuadro yuv420p.
func (f Format) FrameBytes() int { return f.Width * f.Height * 3 / 2 }

// FPS devuelve la tasa de cuadros como flotante (solo para mostrar).
func (f Format) FPS() float64 { return float64(f.FPSNum) / float64(f.FPSDen) }

// FrameDurationNs es la duración exacta de un cuadro en nanosegundos.
func (f Format) FrameDurationNs() float64 { return 1e9 * float64(f.FPSDen) / float64(f.FPSNum) }

// SamplesPerFrame es fraccionario a 59.94 (800.8): por eso el servidor de
// cuadros cuenta muestras globales y nunca redondea por cuadro.
func (f Format) SamplesPerFrame() float64 {
	return float64(f.SampleRate) * float64(f.FPSDen) / float64(f.FPSNum)
}

// SamplesUpTo devuelve cuántas muestras de audio deben haberse entregado
// cuando se han entregado n cuadros. Es la única función que convierte
// cuadros a muestras; con ella el desfase acumulado es cero por construcción.
func (f Format) SamplesUpTo(n int64) int64 {
	return int64(math.Round(float64(n) * f.SamplesPerFrame()))
}

// BytesPerSample es s16le intercalado.
func (f Format) BytesPerSample() int { return 2 * f.Channels }

// FPSString es la forma que ffmpeg entiende.
func (f Format) FPSString() string { return fmt.Sprintf("%d/%d", f.FPSNum, f.FPSDen) }

// SizeString es la forma que ffmpeg entiende.
func (f Format) SizeString() string { return fmt.Sprintf("%dx%d", f.Width, f.Height) }
