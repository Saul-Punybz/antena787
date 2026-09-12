package ingest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"antena787/internal/engine"
)

// NormalizeOptions son las manijas del conformado. El valor cero ya sirve:
// libx264, GOP de un segundo, AAC 48 kHz estéreo y verificación del
// resultado. Nada de esto es constante del motor; todo sale del perfil del
// canal (PRD §10, "Contenedor y códec son propiedades del perfil").
type NormalizeOptions struct {
	SourceDurationMs int64 // duración medida del original; hace falta para recortar la cola
	TrimHeadMs       int64 // negro y silencio de cabeza que se recorta
	TrimTailMs       int64 // negro y silencio de cola que se recorta

	Deinterlace bool // el original viene entrelazado (Measure.Interlaced)

	// Threads es cuántos hilos puede usar ffmpeg para preparar este archivo.
	// Cero deja que ffmpeg se quede con la máquina entera, que es lo correcto
	// cuando el canal no está emitiendo. Con el canal al aire hay que
	// apartarle núcleos: preparar la biblioteca no puede comerse la CPU que
	// sostiene la señal (ADR 0008, el aire manda). Lo calcula App.
	Threads      int
	VideoEncoder string // "libx264" por defecto; el perfil puede pedir otro
	Preset       string // "medium" por defecto
	CRF          int    // 20 por defecto
	AudioEncoder string // "aac" por defecto
	AudioBitrate string // "192k" por defecto
	GOPFrames    int    // 0 = un segundo de cuadros

	KeepCaptions   bool // conservar los streams de subtítulos si el contenedor deja
	EmbeddedCEA608 bool // el video de origen trae 608 dentro (Measure.CaptionFormat)

	// AudioTrack es qué pista de sonido del original sale al aire: la N de
	// «a:N», que es como las cuenta ffmpeg. 0 es la primera, que es lo que
	// vale para la inmensa mayoría de los archivos (F1-60).
	AudioTrack int

	// AudioSidecar es el archivo de sonido que vino al lado del video, para
	// el material que no trae pista propia. Cuando está puesto, el sonido
	// sale de ahí —se mide y se corrige igual que cualquier otro— y
	// AudioTrack no se mira (F1-58).
	AudioSidecar string

	// SubtitleSidecar es el archivo de subtítulos que vino al lado, ya en un
	// formato que el contenedor de casa sabe llevar (.srt o .vtt). Los .scc y
	// los .mcc no se ponen aquí: se guardan sin tocar y los reinserta F2
	// (F1-62, F1-75).
	SubtitleSidecar string

	SkipVerify bool // no medir el resultado (más rápido, menos comprobado)

	ExtraOutputArgs []string // lo que el perfil quiera añadir al final

	// measured es lo que devolvió la primera pasada. No es público: lo pone
	// Normalize entre una pasada y la otra.
	measured *loudness
}

// LoudnessReport es lo que hay que poder enseñar después: cuánto medía antes,
// cuánto mide ahora, y que las dos pasadas se hicieron de verdad (F1-03).
//
// Todos los campos son públicos a propósito: quien llama a Normalize —hoy
// internal/app— tiene que poder guardarlos. Lo mínimo que pide F1-03 es dejar
// constancia de OutputLUFS, OutputTruePeak y Passes; para eso está
// PersistLoudness.SetLoudness (queue.go), y CaptionsNote es la nota que
// acompaña al registro cuando los subtítulos no llegaron enteros.
type LoudnessReport struct {
	// TargetLUFS y TargetTruePeak son lo que pidió el perfil del país:
	// −24 LUFS y −2 dBTP en Estados Unidos y Puerto Rico.
	TargetLUFS     float64
	TargetTruePeak float64

	// Lo que midió la primera pasada, antes de corregir nada. Siempre se
	// mide: desde F1-58 todo lo que se normaliza lleva sonido, sea el suyo o
	// el del archivo de al lado.
	MeasuredLUFS      float64 // volumen integrado del original
	MeasuredTruePeak  float64 // pico real del original, en dBTP
	MeasuredLRA       float64 // rango de volumen del original
	MeasuredThreshold float64 // umbral que usó la medición
	TargetOffset      float64 // corrección que la segunda pasada aplicó

	// Lo que mide la copia ya normalizada. Es una medición de verdad, no una
	// estimación: la tercera corrida de ffmpeg vuelve a escuchar el
	// resultado. Verified dice si esa comprobación llegó a correr
	// (SkipVerify la salta).
	OutputLUFS     float64 // volumen integrado de la copia de casa
	OutputTruePeak float64 // pico real de la copia, en dBTP
	OutputLRA      float64 // rango de volumen de la copia
	Verified       bool

	// Passes es el registro de que se hicieron las dos pasadas: 2 = medir y
	// corregir, que es lo que exige F1-03; nunca 1. Un archivo que llega
	// hasta aquí siempre tiene sonido que medir (F1-58, F1-59).
	Passes int

	// CaptionsKept dice si los subtítulos del original llegaron a la copia.
	// CaptionsNote explica en palabras claras lo que pasó cuando no llegaron
	// enteros; es texto para enseñarle a una persona, no para la máquina.
	CaptionsKept bool
	CaptionsNote string

	// Lo que se recortó y dónde quedó el resultado.
	TrimmedHeadMs    int64  // negro y silencio quitados de la cabeza
	TrimmedTailMs    int64  // negro y silencio quitados de la cola
	OutputPath       string // ruta de la copia normalizada
	OutputDurationMs int64  // cuánto dura la copia después del recorte
	VideoFilter      string // cadena de conformado de imagen que se usó
	AudioFilter      string // cadena de conformado de sonido que se usó
}

// Normalize deja una copia del archivo en el formato de casa: una
// resolución, una tasa de cuadros, GOP cerrado, un códec, un audio — y el
// volumen al objetivo del perfil, con loudnorm en sus dos pasadas (PRD §14.1,
// fila "Volumen"). Se corre una sola vez por archivo, en segundo plano y
// fuera del aire (F1-07).
//
// targetLUFS y truePeak salen del perfil del país: −24 LKFS / −2 dBTP en el
// perfil de Estados Unidos y Puerto Rico.
func Normalize(ctx context.Context, ffmpeg, src, dst string, f engine.Format, targetLUFS, truePeak float64, opts NormalizeOptions) (LoudnessReport, error) {
	rep := LoudnessReport{
		TargetLUFS:     targetLUFS,
		TargetTruePeak: truePeak,
		TrimmedHeadMs:  opts.TrimHeadMs,
		TrimmedTailMs:  opts.TrimTailMs,
		OutputPath:     dst,
	}
	if src == "" || dst == "" {
		return rep, Plainf(nil, "falta decir qué archivo normalizar y dónde dejarlo")
	}
	keepMs := opts.SourceDurationMs - opts.TrimHeadMs - opts.TrimTailMs
	if opts.TrimTailMs > 0 && opts.SourceDurationMs <= 0 {
		return rep, Plainf(nil, "para recortar el final hay que saber cuánto dura el archivo")
	}
	if opts.SourceDurationMs > 0 && keepMs <= 0 {
		return rep, Plainf(nil, "después de quitar el negro no queda nada que emitir")
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return rep, Plainf(err, "no se puede crear la carpeta donde va el archivo normalizado")
	}

	// ── pasada 1: medir ───────────────────────────────────────────────
	// El sonido puede venir del propio archivo —de la pista elegida— o de un
	// archivo de al lado. Se mide el que vaya a salir al aire, que es el que
	// hay que corregir (F1-58, F1-60).
	audioSrc, audioTrack := src, opts.AudioTrack
	if opts.AudioSidecar != "" {
		audioSrc, audioTrack = opts.AudioSidecar, 0
	}
	m, err := measureLoudness(ctx, ffmpeg, audioSrc, audioTrack, targetLUFS, truePeak)
	if err != nil {
		return rep, err
	}
	rep.MeasuredLUFS, rep.MeasuredTruePeak = m.inputI, m.inputTP
	rep.MeasuredLRA, rep.MeasuredThreshold = m.inputLRA, m.inputThresh
	rep.TargetOffset = m.offset
	rep.Passes = 1
	opts.measured = &m

	// ── pasada 2: corregir y conformar ────────────────────────────────
	rep.VideoFilter = videoConform(f, opts.Deinterlace)
	rep.AudioFilter = audioConform(f, targetLUFS, truePeak, opts.measured)

	args := normalizeArgs(src, dst, f, opts, rep, true)
	if err := runFFmpeg(ctx, ffmpeg, args); err != nil {
		if !opts.KeepCaptions && opts.SubtitleSidecar == "" {
			return rep, Plainf(err, "no se pudo convertir el archivo al formato de casa")
		}
		// Los subtítulos son la causa más probable: hay contenedores que no
		// los aceptan. Se reintenta sin ellos y queda dicho en el reporte.
		rep.CaptionsNote = "el contenedor de salida no aceptó los subtítulos del original: quedaron fuera de la copia normalizada"
		args = normalizeArgs(src, dst, f, opts, rep, false)
		if err2 := runFFmpeg(ctx, ffmpeg, args); err2 != nil {
			return rep, Plainf(err2, "no se pudo convertir el archivo al formato de casa")
		}
	} else if opts.KeepCaptions || opts.SubtitleSidecar != "" {
		rep.CaptionsKept = true
	}
	rep.Passes = 2
	if keepMs > 0 {
		rep.OutputDurationMs = keepMs
	}
	if opts.EmbeddedCEA608 {
		note := "los subtítulos CEA-608 venían dentro de la imagen: en F1 se copian si el codificador puede; la reinserción garantizada es F2"
		if rep.CaptionsNote == "" {
			rep.CaptionsNote = note
		} else {
			rep.CaptionsNote += "; " + note
		}
	}

	// ── verificación: medir el resultado ──────────────────────────────
	if !opts.SkipVerify {
		m, err := measureLoudness(ctx, ffmpeg, dst, 0, targetLUFS, truePeak)
		if err != nil {
			return rep, err
		}
		rep.OutputLUFS, rep.OutputTruePeak, rep.OutputLRA = m.inputI, m.inputTP, m.inputLRA
		rep.Verified = true
	}
	return rep, nil
}

// normalizeArgs arma la línea de ffmpeg de la segunda pasada.
func normalizeArgs(src, dst string, f engine.Format, opts NormalizeOptions, rep LoudnessReport, withCaptions bool) []string {
	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y"}
	if opts.TrimHeadMs > 0 {
		args = append(args, "-ss", secs(opts.TrimHeadMs))
	}
	args = append(args, "-i", src)

	// El sonido de al lado y los subtítulos de al lado entran como entradas
	// aparte. El recorte de cabeza se aplica a cada una: si no, el audio
	// llegaría corrido justo lo que se quitó del principio.
	entradaAudio, entradaSub := 0, -1
	if opts.AudioSidecar != "" {
		if opts.TrimHeadMs > 0 {
			args = append(args, "-ss", secs(opts.TrimHeadMs))
		}
		args = append(args, "-i", opts.AudioSidecar)
		entradaAudio = 1
	}
	if withCaptions && opts.SubtitleSidecar != "" {
		args = append(args, "-i", opts.SubtitleSidecar)
		entradaSub = entradaAudio + 1
	}

	keep := opts.SourceDurationMs - opts.TrimHeadMs - opts.TrimTailMs
	switch {
	case keep > 0 && (opts.TrimTailMs > 0 || opts.TrimHeadMs > 0):
		args = append(args, "-t", secs(keep))
	case keep > 0 && opts.AudioSidecar != "":
		// Manda la imagen: un archivo de sonido más largo que el video no
		// alarga la copia de casa.
		args = append(args, "-t", secs(keep))
	}

	args = append(args, "-map", "0:v:0", "-vf", rep.VideoFilter)
	args = append(args, "-map", fmt.Sprintf("%d:a:%d", entradaAudio, opts.audioTrackIndex()), "-af", rep.AudioFilter)
	switch {
	case entradaSub >= 0:
		args = append(args, "-map", fmt.Sprintf("%d:s:0", entradaSub), "-c:s", subtitleCodec(dst))
	case withCaptions && opts.KeepCaptions:
		args = append(args, "-map", "0:s?", "-c:s", subtitleCodec(dst))
	default:
		args = append(args, "-sn")
	}
	args = append(args, "-dn", "-map_metadata", "0", "-map_chapters", "-1")

	enc := or(opts.VideoEncoder, "libx264")
	gop := opts.GOPFrames
	if gop <= 0 {
		gop = int(math.Round(f.FPS()))
	}
	if opts.Threads > 0 {
		args = append(args, "-threads", strconv.Itoa(opts.Threads))
	}
	args = append(args, "-c:v", enc, "-pix_fmt", "yuv420p",
		"-g", strconv.Itoa(gop), "-keyint_min", strconv.Itoa(gop),
		"-sc_threshold", "0", "-flags", "+cgop")
	if strings.HasPrefix(enc, "libx26") {
		args = append(args, "-preset", or(opts.Preset, "medium"), "-crf", strconv.Itoa(orInt(opts.CRF, 20)))
	}
	if opts.EmbeddedCEA608 && enc == "libx264" {
		args = append(args, "-a53cc", "1")
	}
	args = append(args, "-c:a", or(opts.AudioEncoder, "aac"),
		"-b:a", or(opts.AudioBitrate, "192k"),
		"-ar", strconv.Itoa(f.SampleRate), "-ac", strconv.Itoa(f.Channels))
	if e := strings.ToLower(filepath.Ext(dst)); e == ".mp4" || e == ".m4v" || e == ".mov" {
		args = append(args, "-movflags", "+faststart")
	}
	args = append(args, opts.ExtraOutputArgs...)
	return append(args, dst)
}

// audioTrackIndex es la pista que se mapea: la elegida cuando el sonido sale
// del propio archivo, y la única que hay cuando sale del archivo de al lado.
func (o NormalizeOptions) audioTrackIndex() int {
	if o.AudioSidecar != "" || o.AudioTrack < 0 {
		return 0
	}
	return o.AudioTrack
}

// videoConform es el conformado de geometría y cuadros: cabe siempre,
// pillarbox si hace falta, desentrelaza solo lo entrelazado y sale a la tasa
// de casa aunque el original sea PAL o de tasa variable.
func videoConform(f engine.Format, deinterlace bool) string {
	var parts []string
	if deinterlace {
		// deint=interlaced deja pasar sin tocar los cuadros progresivos.
		parts = append(parts, "bwdif=mode=send_field:parity=auto:deint=interlaced")
	}
	parts = append(parts,
		fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease:flags=bicubic", f.Width, f.Height),
		fmt.Sprintf("pad=%d:%d:(ow-iw)/2:(oh-ih)/2", f.Width, f.Height),
		"setsar=1",
		"fps="+f.FPSString(),
		"format=yuv420p")
	return strings.Join(parts, ",")
}

// audioConform es la segunda pasada de loudnorm —con lo medido en la
// primera— más el remuestreo al formato de casa. loudnorm saca 192 kHz por
// dentro: sin el aresample de después, el archivo no queda en el formato.
func audioConform(f engine.Format, targetLUFS, truePeak float64, m *loudness) string {
	ln := fmt.Sprintf("loudnorm=I=%s:TP=%s:LRA=11", num(targetLUFS), num(truePeak))
	if m != nil {
		ln += fmt.Sprintf(":measured_I=%s:measured_TP=%s:measured_LRA=%s:measured_thresh=%s:offset=%s:linear=true",
			num(m.inputI), num(m.inputTP), num(m.inputLRA), num(m.inputThresh), num(m.offset))
	}
	layout := "stereo"
	if f.Channels == 1 {
		layout = "mono"
	}
	return fmt.Sprintf("%s,aresample=%d:first_pts=0,aformat=sample_fmts=fltp:channel_layouts=%s", ln, f.SampleRate, layout)
}

// subtitleCodec elige qué hacer con los subtítulos según el contenedor de
// salida. Matroska y TS se los tragan tal cual; MP4 solo entiende mov_text.
func subtitleCodec(dst string) string {
	switch strings.ToLower(filepath.Ext(dst)) {
	case ".mp4", ".m4v", ".mov":
		return "mov_text"
	default:
		return "copy"
	}
}

// ── la primera pasada ─────────────────────────────────────────────────

type loudness struct {
	inputI, inputTP, inputLRA, inputThresh, offset float64
}

// measureLoudness es la pasada de medición: no escribe nada, solo pregunta
// cuánto suena el archivo.
func measureLoudness(ctx context.Context, ffmpeg, path string, track int, targetLUFS, truePeak float64) (loudness, error) {
	var out loudness
	if track < 0 {
		track = 0
	}
	args := []string{"-nostdin", "-hide_banner", "-i", path, "-map", fmt.Sprintf("0:a:%d", track),
		"-af", fmt.Sprintf("loudnorm=I=%s:TP=%s:LRA=11:print_format=json", num(targetLUFS), num(truePeak)),
		"-f", "null", "-"}
	cmd := engine.Comando(ctx, ffmpeg, args...)
	var errb bytes.Buffer
	cmd.Stderr = &errb
	cmd.Stdin = nil
	if err := cmd.Run(); err != nil {
		return out, Plainf(fmt.Errorf("%s", tailLines(errb.String(), 6)), "no se pudo medir el volumen del archivo: ¿tiene sonido?")
	}
	return parseLoudnorm(errb.String())
}

// parseLoudnorm saca el bloque JSON que loudnorm imprime al final de stderr.
func parseLoudnorm(text string) (loudness, error) {
	var out loudness
	i := strings.LastIndex(text, "{")
	j := strings.LastIndex(text, "}")
	if i < 0 || j < i {
		return out, Plainf(nil, "ffmpeg no devolvió la medición de volumen")
	}
	var raw map[string]string
	if err := json.Unmarshal([]byte(text[i:j+1]), &raw); err != nil {
		return out, Plainf(err, "no se entendió la medición de volumen que devolvió ffmpeg")
	}
	get := func(k string) float64 {
		f, err := strconv.ParseFloat(strings.TrimSpace(raw[k]), 64)
		if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
			return 0
		}
		return f
	}
	out.inputI = get("input_i")
	out.inputTP = get("input_tp")
	out.inputLRA = get("input_lra")
	out.inputThresh = get("input_thresh")
	out.offset = get("target_offset")
	return out, nil
}

func runFFmpeg(ctx context.Context, ffmpeg string, args []string) error {
	cmd := engine.Comando(ctx, ffmpeg, args...)
	var errb bytes.Buffer
	cmd.Stderr = &errb
	cmd.Stdin = nil
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, tailLines(errb.String(), 6))
	}
	return nil
}

func num(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }
func secs(ms int64) string { return strconv.FormatFloat(float64(ms)/1000, 'f', 3, 64) }
func or(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}
func orInt(n, def int) int {
	if n <= 0 {
		return def
	}
	return n
}
