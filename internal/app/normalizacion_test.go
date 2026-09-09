package app

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"antena787/internal/ingest"
	"antena787/internal/model"
)

// ── la preparación que no termina (F1-71) ─────────────────────────────

func TestNormalizeDeadlineNuncaEsMenorQueElMinimo(t *testing.T) {
	a := abre(t)
	if got := a.normalizeDeadline(2 * 60 * 1000); got != NormalizeMinTimeout {
		t.Errorf("2 min de archivo → plazo %s; el mínimo es %s", got, NormalizeMinTimeout)
	}
	if got := a.normalizeDeadline(2 * 60 * 60 * 1000); got != 8*time.Hour {
		t.Errorf("2 h de archivo → plazo %s; se esperaban 8 h (×%d)", got, NormalizeTimeoutFactor)
	}
	if plazoEnCristiano(15*time.Minute) != "15 min" || plazoEnCristiano(8*time.Hour) != "8 h" ||
		plazoEnCristiano(2*time.Hour+40*time.Minute) != "2 h 40 min" {
		t.Errorf("el plazo no se escribe como una persona: %q %q %q",
			plazoEnCristiano(15*time.Minute), plazoEnCristiano(8*time.Hour), plazoEnCristiano(2*time.Hour+40*time.Minute))
	}
}

// TestNormalizacionColgadaVaACuarentena: una normalización que se pasa de su
// plazo cuenta como intento fallido con un motivo que dice que se colgó; y
// cuando la cola la da por perdida, el archivo no se queda «aún no listo
// para aire» para siempre: va a cuarentena con su código, Al aire avisa, y
// dejarlo pasar lo saca al aire tal cual.
func TestNormalizacionColgadaVaACuarentena(t *testing.T) {
	a := abre(t, func(o *Options) { o.NormalizeTimeout = time.Nanosecond })
	if a.FFmpeg == "" || a.FFprobe == "" {
		t.Skip("no hay ffmpeg: " + a.FFmpegErr.Error())
	}
	ctx := context.Background()

	dir := t.TempDir()
	ruta := filepath.Join(dir, "Kojak S01E01.mp4")
	out, err := exec.Command(a.FFmpeg, "-nostdin", "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-t", "2", "-i", "testsrc2=s=320x240:r=30",
		"-f", "lavfi", "-t", "2", "-i", "sine=f=440:r=48000",
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-shortest", ruta).CombinedOutput()
	if err != nil {
		t.Fatalf("no se pudo fabricar el clip: %v\n%s", err, out)
	}
	asset := model.MediaAsset{
		Path: ruta, DurationMs: 2000, State: model.AssetReady,
		NormalizeState: ingest.NormalizePending,
		CreatedAt:      a.Now(), UpdatedAt: a.Now(),
	}
	if err := a.Store.Media.Insert(ctx, &asset); err != nil {
		t.Fatal(err)
	}

	// El intento: el plazo de un nanosegundo hace que ffmpeg no llegue ni a
	// arrancar, que es lo mismo que quedarse colgado.
	_, err = a.normalizeOne(ctx, ingest.Job{AssetID: asset.ID})
	if err == nil {
		t.Fatal("con un plazo de un nanosegundo la normalización tenía que darse por colgada")
	}
	if !strings.Contains(ingest.Plain(err), "se quedó colgada") {
		t.Errorf("el motivo no dice que se colgó: %q", ingest.Plain(err))
	}

	// La cola la da por perdida tras sus intentos: eso es lo que persist ve.
	p := &persist{a: a}
	motivo := "no se pudo dejar el archivo en el formato de casa después de 2 intentos: " + ingest.Plain(err)
	if err := p.SetNormalizeState(ctx, asset.ID, ingest.NormalizeFailed, "", motivo); err != nil {
		t.Fatal(err)
	}
	parado, err := a.Store.Media.Get(ctx, asset.ID)
	if err != nil {
		t.Fatal(err)
	}
	if parado.State != model.AssetQuarantine {
		t.Errorf("estado = %q; un archivo que no se pudo preparar va a cuarentena, no al limbo", parado.State)
	}
	if parado.MotivoCodigo != ingest.MotivoNormalizacion {
		t.Errorf("código = %q, se esperaba %q", parado.MotivoCodigo, ingest.MotivoNormalizacion)
	}
	if !strings.Contains(parado.PlainReason, "colgada") {
		t.Errorf("el motivo guardado no dice qué pasó: %q", parado.PlainReason)
	}
	hay := false
	for _, al := range a.Alarms() {
		if al.Texto == "1 archivo en cuarentena" {
			hay = true
		}
	}
	if !hay {
		t.Errorf("Al aire no avisa del archivo parado: %+v", a.Alarms())
	}

	// Y queda constancia en la bitácora, con su tipo.
	incs, err := a.Store.Incident.List(ctx, a.ChannelID, a.Now().Add(-time.Hour), a.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	tipos := []string{}
	for _, i := range incs {
		tipos = append(tipos, i.Kind)
	}
	if !strings.Contains(fmt.Sprint(tipos), "normalizacion_fallida") {
		t.Errorf("la bitácora no anotó normalizacion_fallida: %v", tipos)
	}
}
