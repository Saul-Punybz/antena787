package ingest

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"math"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Umbrales del PRD §14.1, fila "Negro y silencio en el ingest": luma media
// bajo 16 (sobre 255 son 0.0627 normalizados; ffmpeg pide 0.10 para que el
// ruido de compresión no lo tumbe), audio bajo −60 dBFS, más de medio
// segundo. En cabeza y cola se recorta; en medio son marcas de corte
// candidatas que confirma el programador.
const (
	BlackDetectFilter   = "blackdetect=d=0.5:pic_th=0.98:pix_th=0.10"
	SilenceDetectFilter = "silencedetect=n=-60dB:d=0.5"

	// edgeToleranceMs es cuánto puede separarse del borde un tramo para
	// seguir contando como cabeza o cola. Medio cuadro a 29.97 son 17 ms;
	// 120 ms cubre el arranque de cualquier códec sin comerse contenido.
	edgeToleranceMs = 120
)

// Interval es un tramo del archivo, en milisegundos desde el inicio.
type Interval struct{ StartMs, EndMs int64 }

// Duration devuelve lo que dura el tramo.
func (i Interval) Duration() time.Duration {
	return time.Duration(i.EndMs-i.StartMs) * time.Millisecond
}

// BlackSilence es el resultado crudo del análisis, por si alguien quiere ver
// los tramos y no solo el recorte.
type BlackSilence struct {
	Black    []Interval // tramos en negro
	Silence  []Interval // tramos en silencio
	Both     []Interval // negro y silencio a la vez: los únicos que cuentan
	HeadMs   int64      // recorte de cabeza
	TailMs   int64      // recorte de cola
	MarksMs  []int64    // marcas de corte candidatas, en medio del archivo
	HasVideo bool       // si el archivo no trae imagen, manda el silencio solo
}

// BlackAndSilence busca negro y silencio en una sola pasada de ffmpeg.
//
// Devuelve cuántos milisegundos hay que recortar de la cabeza y de la cola
// —solo donde el negro y el silencio coinciden— y las marcas de corte
// candidatas del medio, que NO se recortan: se guardan en
// marcas_de_corte_ms para que una persona las confirme (PRD §14.1, F1-06).
//
// dur es la duración medida del archivo; hace falta para saber qué es "cola".
// Si es 0, no se detecta cola.
func BlackAndSilence(ctx context.Context, ffmpeg, path string, dur time.Duration) (head, tail int64, candidates []int64, err error) {
	r, err := Analyze(ctx, ffmpeg, path, dur)
	if err != nil {
		return 0, 0, nil, err
	}
	return r.HeadMs, r.TailMs, r.MarksMs, nil
}

// Analyze es BlackAndSilence con el detalle completo.
func Analyze(ctx context.Context, ffmpeg, path string, dur time.Duration) (BlackSilence, error) {
	var out BlackSilence
	cmd := exec.CommandContext(ctx, ffmpeg, "-nostdin", "-hide_banner", "-loglevel", "info",
		"-i", path,
		"-vf", BlackDetectFilter,
		"-af", SilenceDetectFilter,
		"-f", "null", "-")
	var errb bytes.Buffer
	cmd.Stderr = &errb
	cmd.Stdin = nil
	runErr := cmd.Run()
	text := errb.String()
	// Un archivo con la cola dañada termina en error después de haber
	// analizado casi todo: lo que se alcanzó a leer sigue valiendo.
	if runErr != nil && !strings.Contains(text, "blackdetect") && !strings.Contains(text, "silencedetect") {
		return out, Plainf(fmt.Errorf("%s", tailLines(text, 6)), "no se pudo revisar el archivo en busca de negro y silencio")
	}
	out = parseDetect(text, dur)
	return out, nil
}

// parseDetect lee lo que blackdetect y silencedetect escriben en stderr y
// arma el resultado. Está aparte para poder probarlo sin ffmpeg.
func parseDetect(text string, dur time.Duration) BlackSilence {
	var out BlackSilence
	durMs := dur.Milliseconds()

	var silStart int64 = -1
	sc := bufio.NewScanner(strings.NewReader(text))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.Contains(line, ": Video:"):
			out.HasVideo = true
		case strings.Contains(line, "black_start"):
			s, okS := fieldMs(line, "black_start:")
			e, okE := fieldMs(line, "black_end:")
			if okS && okE && e > s {
				out.Black = append(out.Black, Interval{s, e})
			}
		case strings.Contains(line, "silence_start"):
			if v, ok := fieldMs(line, "silence_start:"); ok {
				silStart = v
			}
		case strings.Contains(line, "silence_end"):
			if v, ok := fieldMs(line, "silence_end:"); ok && silStart >= 0 && v > silStart {
				out.Silence = append(out.Silence, Interval{silStart, v})
				silStart = -1
			}
		}
	}
	// Un silencio que llega hasta el final del archivo no cierra: ffmpeg no
	// escribe silence_end. Se cierra con la duración medida.
	if silStart >= 0 && durMs > silStart {
		out.Silence = append(out.Silence, Interval{silStart, durMs})
	}

	if out.HasVideo {
		out.Both = intersect(out.Black, out.Silence)
	} else {
		// Radio: no hay imagen que mirar, así que manda el silencio solo.
		out.Both = out.Silence
	}

	for _, iv := range out.Both {
		switch {
		case iv.StartMs <= edgeToleranceMs && iv.EndMs > out.HeadMs:
			out.HeadMs = iv.EndMs
		case durMs > 0 && iv.EndMs >= durMs-edgeToleranceMs:
			if t := durMs - iv.StartMs; t > out.TailMs {
				out.TailMs = t
			}
		default:
			// En medio: marca de corte candidata en el centro del tramo,
			// que es donde entra el corte sin comerse imagen de ninguno de
			// los dos lados.
			out.MarksMs = append(out.MarksMs, (iv.StartMs+iv.EndMs)/2)
		}
	}
	sort.Slice(out.MarksMs, func(i, j int) bool { return out.MarksMs[i] < out.MarksMs[j] })
	// Cabeza y cola no pueden solaparse: un archivo entero en negro se
	// recorta como cabeza y nada más.
	if durMs > 0 && out.HeadMs+out.TailMs > durMs {
		out.TailMs = 0
	}
	return out
}

// intersect devuelve los tramos donde a y b coinciden y la coincidencia dura
// más de medio segundo, que es el umbral del PRD.
func intersect(a, b []Interval) []Interval {
	var out []Interval
	for _, x := range a {
		for _, y := range b {
			s := max64(x.StartMs, y.StartMs)
			e := min64(x.EndMs, y.EndMs)
			if e-s >= 500 {
				out = append(out, Interval{s, e})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartMs < out[j].StartMs })
	return merge(out)
}

// merge junta tramos pegados o solapados.
func merge(in []Interval) []Interval {
	var out []Interval
	for _, iv := range in {
		if n := len(out); n > 0 && iv.StartMs <= out[n-1].EndMs {
			if iv.EndMs > out[n-1].EndMs {
				out[n-1].EndMs = iv.EndMs
			}
			continue
		}
		out = append(out, iv)
	}
	return out
}

// fieldMs saca "black_start:1.234" y lo devuelve en milisegundos.
func fieldMs(line, key string) (int64, bool) {
	i := strings.Index(line, key)
	if i < 0 {
		return 0, false
	}
	rest := strings.TrimLeft(line[i+len(key):], " \t")
	end := 0
	for end < len(rest) && (rest[end] == '.' || rest[end] == '-' || rest[end] == '+' ||
		(rest[end] >= '0' && rest[end] <= '9') || rest[end] == 'e' || rest[end] == 'E') {
		end++
	}
	f, err := strconv.ParseFloat(rest[:end], 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	if f < 0 {
		f = 0
	}
	return int64(math.Round(f * 1000)), true
}

func tailLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, " | ")
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
