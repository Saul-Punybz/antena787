package ingest

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"antena787/internal/engine"
)

// tools localiza ffmpeg y ffprobe. Sin ellos no hay medios de verdad que
// medir, así que la prueba se salta con un mensaje que dice qué falta.
func tools(t *testing.T) (ffmpeg, ffprobe string) {
	t.Helper()
	ff, err := engine.FFmpeg()
	if err != nil {
		t.Skip("no hay ffmpeg: " + err.Error())
	}
	fp, err := engine.FFprobe()
	if err != nil {
		t.Skip("no hay ffprobe: " + err.Error())
	}
	return ff, fp
}

// segment es un tramo del archivo de prueba: cuánto dura, si es negro y si
// va en silencio. Con esto se fabrican los casos del PRD sin traer archivos.
type segment struct {
	seconds float64
	black   bool
	silent  bool
}

// makeClip fabrica un archivo pegando los tramos uno detrás de otro, con la
// misma receta que internal/f0: fuentes de lavfi y H.264 + AAC.
func makeClip(t *testing.T, ffmpeg, dst string, rate string, segs ...segment) string {
	t.Helper()
	if rate == "" {
		rate = "30"
	}
	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y"}
	var chain strings.Builder
	for i, s := range segs {
		dur := fmt.Sprintf("%.3f", s.seconds)
		video := "testsrc2=s=320x240:r=" + rate
		if s.black {
			video = "color=c=black:s=320x240:r=" + rate
		}
		audio := "sine=f=440:r=48000"
		if s.silent {
			audio = "anullsrc=r=48000:cl=stereo"
		}
		args = append(args, "-f", "lavfi", "-t", dur, "-i", video)
		args = append(args, "-f", "lavfi", "-t", dur, "-i", audio)
		fmt.Fprintf(&chain, "[%d:v][%d:a]", 2*i, 2*i+1)
	}
	fmt.Fprintf(&chain, "concat=n=%d:v=1:a=1[v][a]", len(segs))
	args = append(args,
		"-filter_complex", chain.String(), "-map", "[v]", "-map", "[a]",
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", "-g", "30",
		"-c:a", "aac", "-ar", "48000", "-ac", "2", dst)
	out, err := exec.Command(ffmpeg, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("no se pudo fabricar el medio de prueba: %v\n%s", err, out)
	}
	return dst
}

func TestProbeMideLoQueDicelPRD(t *testing.T) {
	ffmpeg, ffprobe := tools(t)
	dir := t.TempDir()
	src := makeClip(t, ffmpeg, filepath.Join(dir, "clip.mp4"), "30000/1001", segment{seconds: 3})

	m, err := Probe(context.Background(), ffprobe, src)
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if !m.HasVideo || !m.HasAudio {
		t.Fatalf("se esperaba video y audio, se obtuvo %+v", m)
	}
	if m.Codec != "h264" {
		t.Errorf("códec = %q, se esperaba h264", m.Codec)
	}
	if m.Resolution != "320x240" {
		t.Errorf("resolución = %q, se esperaba 320x240", m.Resolution)
	}
	if m.FPS != "30000/1001" {
		t.Errorf("fps = %q, se esperaba 30000/1001", m.FPS)
	}
	if m.AudioChannels != 2 {
		t.Errorf("canales = %d, se esperaban 2", m.AudioChannels)
	}
	if d := m.DurationMs; d < 2900 || d > 3100 {
		t.Errorf("duración = %d ms, se esperaban ~3000", d)
	}
	if m.DurationMs%1000 == 0 && m.DurationMs != 3000 {
		t.Errorf("la duración parece redondeada al segundo: %d ms", m.DurationMs)
	}

	h, err := HashFile(context.Background(), src)
	if err != nil {
		t.Fatalf("HashFile: %v", err)
	}
	if len(h) != 64 {
		t.Errorf("el hash mide %d, se esperaban 64 caracteres", len(h))
	}
	h2, _ := HashFile(context.Background(), src)
	if h != h2 {
		t.Error("el hash del mismo archivo cambió entre dos lecturas")
	}
}

func TestNegroYSilencioEnCabeza(t *testing.T) {
	ffmpeg, _ := tools(t)
	dir := t.TempDir()
	// El caso exacto de ACEPTACION F1-05: 0.8 s de negro y silencio al
	// principio (por encima del umbral: se recorta) y 0.3 s al final (por
	// debajo: no se toca).
	src := makeClip(t, ffmpeg, filepath.Join(dir, "cabeza.mp4"), "30",
		segment{seconds: 0.8, black: true, silent: true},
		segment{seconds: 3},
		segment{seconds: 0.3, black: true, silent: true})

	head, tail, marks, err := BlackAndSilence(context.Background(), ffmpeg, src, 4100*time.Millisecond)
	if err != nil {
		t.Fatalf("BlackAndSilence: %v", err)
	}
	if head < 700 || head > 1200 {
		t.Errorf("cabeza = %d ms, se esperaban ~800", head)
	}
	if tail != 0 {
		t.Errorf("cola = %d ms: 0.3 s está por debajo del umbral y no se recorta", tail)
	}
	if len(marks) != 0 {
		t.Errorf("marcas = %v, se esperaba ninguna", marks)
	}
}

func TestNegroEnMedioEsMarcaDeCorte(t *testing.T) {
	ffmpeg, _ := tools(t)
	dir := t.TempDir()
	src := makeClip(t, ffmpeg, filepath.Join(dir, "medio.mp4"), "30",
		segment{seconds: 2},
		segment{seconds: 2, black: true, silent: true},
		segment{seconds: 2})

	head, tail, marks, err := BlackAndSilence(context.Background(), ffmpeg, src, 6*time.Second)
	if err != nil {
		t.Fatalf("BlackAndSilence: %v", err)
	}
	if head != 0 || tail != 0 {
		t.Errorf("no se debe recortar nada: cabeza=%d cola=%d", head, tail)
	}
	if len(marks) != 1 {
		t.Fatalf("marcas = %v, se esperaba exactamente una", marks)
	}
	if marks[0] < 2200 || marks[0] > 3800 {
		t.Errorf("la marca cayó en %d ms, se esperaba dentro del negro (2000-4000)", marks[0])
	}
}

func TestParseDetectSinFFmpeg(t *testing.T) {
	// La misma forma que escribe ffmpeg en stderr, sin tener que correrlo.
	text := strings.Join([]string{
		"  Stream #0:0: Video: h264, yuv420p, 320x240",
		"  Stream #0:1: Audio: aac, 48000 Hz, stereo",
		"[blackdetect @ 0x1] black_start:0 black_end:1.001 black_duration:1.001",
		"[silencedetect @ 0x2] silence_start: 0",
		"[silencedetect @ 0x2] silence_end: 1.001 | silence_duration: 1.001",
		"[blackdetect @ 0x1] black_start:3 black_end:4.2 black_duration:1.2",
		"[silencedetect @ 0x2] silence_start: 2.9",
		"[silencedetect @ 0x2] silence_end: 4.3 | silence_duration: 1.4",
		"[blackdetect @ 0x1] black_start:9 black_end:10 black_duration:1",
		"[silencedetect @ 0x2] silence_start: 9",
	}, "\n")
	got := parseDetect(text, 10*time.Second)
	if got.HeadMs != 1001 {
		t.Errorf("cabeza = %d, se esperaban 1001", got.HeadMs)
	}
	if got.TailMs != 1000 {
		t.Errorf("cola = %d, se esperaban 1000", got.TailMs)
	}
	if len(got.MarksMs) != 1 || got.MarksMs[0] != 3600 {
		t.Errorf("marcas = %v, se esperaba [3600]", got.MarksMs)
	}
}

func TestNormalizeDejaElVolumenEnElObjetivo(t *testing.T) {
	ffmpeg, ffprobe := tools(t)
	dir := t.TempDir()
	src := makeClip(t, ffmpeg, filepath.Join(dir, "fuerte.mp4"), "30", segment{seconds: 6})
	dst := filepath.Join(dir, "casa.mp4")

	f := engine.Format{Width: 1280, Height: 720, FPSNum: 60000, FPSDen: 1001, SampleRate: 48000, Channels: 2}
	rep, err := Normalize(context.Background(), ffmpeg, src, dst, f, -24, -2, NormalizeOptions{
		SourceDurationMs: 6000,
		Preset:           "ultrafast",
	})
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if rep.Passes != 2 {
		t.Errorf("pasadas = %d, el PRD exige dos (medir y corregir)", rep.Passes)
	}
	if rep.MeasuredLUFS == 0 {
		t.Error("no quedó registro de la primera pasada (LUFS medido)")
	}
	if !rep.Verified {
		t.Fatal("no se verificó el resultado")
	}
	if rep.OutputLUFS < -25 || rep.OutputLUFS > -23 {
		t.Errorf("el archivo normalizado mide %.2f LUFS, se esperaba −24 ±1", rep.OutputLUFS)
	}
	if rep.OutputTruePeak > -1 {
		t.Errorf("true peak = %.2f dBTP, se esperaba por debajo de −2", rep.OutputTruePeak)
	}

	m, err := Probe(context.Background(), ffprobe, dst)
	if err != nil {
		t.Fatalf("Probe del normalizado: %v", err)
	}
	if m.Resolution != "1280x720" {
		t.Errorf("resolución = %q, se esperaba 1280x720", m.Resolution)
	}
	if m.FPS != "60000/1001" {
		t.Errorf("fps = %q, se esperaba 60000/1001", m.FPS)
	}
	if m.AudioChannels != 2 || m.SampleRate != 48000 {
		t.Errorf("audio = %d canales a %d Hz, se esperaba 2 a 48000", m.AudioChannels, m.SampleRate)
	}
}

func TestNormalizeRecortaLaCabeza(t *testing.T) {
	ffmpeg, ffprobe := tools(t)
	dir := t.TempDir()
	src := makeClip(t, ffmpeg, filepath.Join(dir, "conslate.mp4"), "30",
		segment{seconds: 1, black: true, silent: true},
		segment{seconds: 4})
	dst := filepath.Join(dir, "recortado.mp4")

	rep, err := Normalize(context.Background(), ffmpeg, src, dst, engine.CAtv, -24, -2, NormalizeOptions{
		SourceDurationMs: 5000,
		TrimHeadMs:       1000,
		Preset:           "ultrafast",
		SkipVerify:       true,
	})
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if rep.TrimmedHeadMs != 1000 {
		t.Errorf("recorte de cabeza = %d ms", rep.TrimmedHeadMs)
	}
	m, err := Probe(context.Background(), ffprobe, dst)
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if m.DurationMs < 3800 || m.DurationMs > 4200 {
		t.Errorf("el normalizado dura %d ms, se esperaban ~4000", m.DurationMs)
	}
}

func TestThumbnail(t *testing.T) {
	ffmpeg, _ := tools(t)
	dir := t.TempDir()
	src := makeClip(t, ffmpeg, filepath.Join(dir, "clip.mp4"), "30", segment{seconds: 3})
	dst := filepath.Join(dir, "miniaturas", "clip.png")

	if err := Thumbnail(context.Background(), ffmpeg, src, ThumbnailAt(3000), dst); err != nil {
		t.Fatalf("Thumbnail: %v", err)
	}
	st, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("no se escribió la miniatura: %v", err)
	}
	if st.Size() == 0 {
		t.Error("la miniatura quedó vacía")
	}
	head, err := os.ReadFile(dst)
	if err != nil || len(head) < 8 || string(head[1:4]) != "PNG" {
		t.Error("la miniatura no es un PNG")
	}
}
