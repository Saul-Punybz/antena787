package app

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"antena787/internal/engine"
	"antena787/internal/ingest"
	"antena787/internal/model"
)

// Verificación de F1-58 de docs/ACEPTACION.md ("Audio de todo el material")
// al nivel en que corre de verdad: la carpeta vigilada, el ingest y la base
// juntos. Un video mudo queda parado; el archivo de sonido que llega después
// lo saca de la parada él solo, sin que nadie toque nada y sin estrenar
// ficha.

func herramientas(t *testing.T) (ffmpeg, ffprobe string) {
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

// clipMudo fabrica un video sin ninguna pista de sonido.
func clipMudo(t *testing.T, ffmpeg, dst string, segundos float64) string {
	t.Helper()
	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-t", fmt.Sprintf("%.3f", segundos), "-i", "testsrc2=s=320x240:r=30",
		"-an", "-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", "-g", "30", dst}
	if out, err := exec.Command(ffmpeg, args...).CombinedOutput(); err != nil {
		t.Fatalf("no se pudo fabricar el clip mudo: %v\n%s", err, out)
	}
	return dst
}

// audioSuelto fabrica el archivo de sonido que va al lado del video.
func audioSuelto(t *testing.T, ffmpeg, dst string, segundos float64) string {
	t.Helper()
	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-t", fmt.Sprintf("%.3f", segundos), "-i", "sine=f=440:r=48000",
		"-ac", "2", dst}
	if out, err := exec.Command(ffmpeg, args...).CombinedOutput(); err != nil {
		t.Fatalf("no se pudo fabricar el sonido de al lado: %v\n%s", err, out)
	}
	return dst
}

// eventos guarda todo lo que sale por el bus mientras dura la prueba.
type eventos struct {
	mu    sync.Mutex
	lista []Event
}

func (e *eventos) mirando(a *App, t *testing.T) {
	t.Helper()
	ch, soltar := a.Subscribe()
	t.Cleanup(soltar)
	go func() {
		for ev := range ch {
			e.mu.Lock()
			e.lista = append(e.lista, ev)
			e.mu.Unlock()
		}
	}()
}

func (e *eventos) hay(clase, nombre, trozo string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, ev := range e.lista {
		if ev.Kind == clase && ev.Name == nombre && strings.Contains(ev.Detail, trozo) {
			return true
		}
	}
	return false
}

func TestF1_58_ElSonidoQueLlegaDespuesSacaDeLaParada(t *testing.T) {
	ffmpeg, _ := herramientas(t)

	// El reloj de la aplicación va un minuto por delante del de los
	// archivos: así la carpeta vigilada da por copiado lo que acaba de
	// aparecer sin que la prueba espere diez segundos de verdad (F1-01).
	a := abre(t, func(o *Options) {
		o.Now = func() time.Time { return time.Now().Add(time.Minute) }
	})
	if a.FFmpeg == "" || a.FFprobe == "" {
		t.Skip("no hay ffmpeg: " + a.FFmpegErr.Error())
	}

	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var vistos eventos
	vistos.mirando(a, t)
	if err := a.watch(ctx, dir, false); err != nil {
		t.Fatalf("no se pudo vigilar la carpeta: %v", err)
	}

	fondo := context.Background()
	video := clipMudo(t, ffmpeg, filepath.Join(dir, "promo.mp4"), 3)

	// 1 · el video mudo se para y dice qué hacer.
	var parado model.MediaAsset
	esperar(t, 60*time.Second, func() bool {
		asset, err := a.Store.Media.GetByPath(fondo, video)
		if err != nil {
			return false
		}
		parado = asset
		return asset.State == model.AssetQuarantine
	}, "el video mudo no quedó parado en cuarentena")

	if !ingest.TextoSinAudio(parado.PlainReason) {
		t.Fatalf("el motivo no es el de «no trae sonido»: %q", parado.PlainReason)
	}

	// 2 · llega el sonido con el mismo nombre.
	audio := audioSuelto(t, ffmpeg, filepath.Join(dir, "promo.wav"), 3)

	var listo model.MediaAsset
	esperar(t, 120*time.Second, func() bool {
		asset, err := a.Store.Media.GetByPath(fondo, video)
		if err != nil {
			return false
		}
		listo = asset
		return asset.State == model.AssetReady
	}, "el video no salió de la parada cuando llegó su sonido")

	// 3 · es el mismo archivo de siempre, no uno nuevo.
	if listo.ID != parado.ID {
		t.Errorf("el video estrenó ficha (%d) en vez de actualizar la suya (%d)", listo.ID, parado.ID)
	}
	if listo.PlainReason != "" {
		t.Errorf("el motivo viejo se quedó puesto: %q", listo.PlainReason)
	}
	if listo.AudioSidecar != audio {
		t.Errorf("el archivo no dice de dónde salió su sonido: %q", listo.AudioSidecar)
	}
	if listo.AudioChannels <= 0 {
		t.Errorf("el archivo sigue diciendo que no tiene sonido: %d canales", listo.AudioChannels)
	}

	// 4 · el archivo de al lado no entró como material aparte, y del video
	// hay una sola ficha.
	todos, err := a.Store.Media.List(fondo, "")
	if err != nil {
		t.Fatalf("no pude leer el material: %v", err)
	}
	cuantos := 0
	for _, m := range todos {
		if m.Path == audio {
			t.Errorf("el sonido de al lado entró como material aparte (ficha %d)", m.ID)
		}
		if m.Path == video {
			cuantos++
		}
	}
	if cuantos != 1 {
		t.Errorf("del mismo video hay %d fichas", cuantos)
	}

	// 5 · vuelve a la cola de normalización y se cuenta como cualquier
	// archivo que entra.
	if listo.NormalizeState != ingest.NormalizePending {
		t.Errorf("el archivo quedó en %q y tenía que estar en %q", listo.NormalizeState, ingest.NormalizePending)
	}
	if a.Queue.Len() == 0 {
		t.Error("el archivo no volvió a la cola de normalización")
	}
	if !vistos.hay("ingest", "material", "entró promo.mp4") {
		t.Error("no se avisó de que el archivo entró, como con cualquier otro")
	}
}
