package app

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"antena787/internal/model"
)

func unPreset(t *testing.T, a *App, nombre string, aj model.AjustesDePreset) int64 {
	t.Helper()
	datos, err := json.Marshal(aj)
	if err != nil {
		t.Fatal(err)
	}
	canal := a.ChannelID
	p := model.Preset{ChannelID: &canal, Name: nombre, Settings: string(datos)}
	if err := a.Store.Preset.Insert(context.Background(), &p); err != nil {
		t.Fatalf("no se pudo guardar el preset: %v", err)
	}
	return p.ID
}

// unArchivo deja un archivo en la biblioteca, sin bloque en la parrilla: para
// estas pruebas solo hace falta algo a lo que colgarle un preset.
func unArchivo(t *testing.T, a *App, nombre string) model.MediaAsset {
	t.Helper()
	asset := model.MediaAsset{
		Path:           filepath.Join(t.TempDir(), nombre+".mkv"),
		DurationMs:     600000,
		State:          model.AssetReady,
		NormalizeState: model.NormalizeReady,
		CreatedAt:      a.Now(),
		UpdatedAt:      a.Now(),
	}
	if err := a.Store.Media.Insert(context.Background(), &asset); err != nil {
		t.Fatalf("no se pudo guardar el archivo: %v", err)
	}
	return asset
}

func f64(v float64) *float64 { return &v }
func i64(v int64) *int64     { return &v }
func b(v bool) *bool         { return &v }

// Los tres niveles: canal, título y archivo, y gana el más específico. Un
// nivel que no dice nada **hereda**, no tapa con un cero — esa distinción es
// la razón de que los campos sean punteros, y si se rompiera, un preset con un
// campo vacío pisaría el del canal en silencio.
func TestElPresetMasEspecificoManda(t *testing.T) {
	a := abre(t)
	ctx := context.Background()

	// El canal: calidad alta y tres segundos de recorte de cabeza.
	delCanal := unPreset(t, a, "del canal", model.AjustesDePreset{
		Calidad: "alta", RecorteCabezaMs: i64(3000),
	})
	ch, err := a.Store.Channel.Get(ctx, a.ChannelID)
	if err != nil {
		t.Fatal(err)
	}
	ch.PresetID = &delCanal
	if err := a.Store.Channel.Update(ctx, ch); err != nil {
		t.Fatal(err)
	}

	asset := unArchivo(t, a, "Kojak")

	// Sin nada más, manda el del canal.
	got := a.PresetDe(ctx, asset)
	if got.Calidad != "alta" || got.RecorteCabezaMs == nil || *got.RecorteCabezaMs != 3000 {
		t.Fatalf("el preset del canal no llegó: %+v", got)
	}

	// El del archivo cambia la calidad y NO dice nada del recorte.
	delArchivo := unPreset(t, a, "de este archivo", model.AjustesDePreset{
		Calidad: "baja", VolumenRelativoDB: f64(3),
	})
	asset.PresetID = &delArchivo
	if err := a.Store.Media.Update(ctx, &asset); err != nil {
		t.Fatal(err)
	}

	got = a.PresetDe(ctx, asset)
	if got.Calidad != "baja" {
		t.Fatalf("el archivo tenía que mandar sobre el canal y la calidad quedó en %q", got.Calidad)
	}
	if got.VolumenRelativoDB == nil || *got.VolumenRelativoDB != 3 {
		t.Fatal("lo que solo dice el archivo se perdió")
	}
	if got.RecorteCabezaMs == nil || *got.RecorteCabezaMs != 3000 {
		t.Fatal("el archivo no decía nada del recorte y aun así tapó el del canal: un nivel que calla hereda, no borra")
	}
}

// Sin ningún preset puesto, todo vacío: añadir esta función no puede cambiarle
// el comportamiento a una instalación que no los use.
func TestSinPresetsNoCambiaNada(t *testing.T) {
	a := abre(t)
	got := a.PresetDe(context.Background(), unArchivo(t, a, "Get Smart"))
	if got.Calidad != "" || got.NoTocar != nil || got.RecorteCabezaMs != nil || got.VolumenRelativoDB != nil {
		t.Fatalf("sin presets tenía que salir todo vacío y salió %+v", got)
	}
}

// «No tocar» es el que de verdad ahorra: el archivo sale como vino.
func TestNoTocarDejaElArchivoComoVino(t *testing.T) {
	a := abre(t)
	ctx := context.Background()
	asset := unArchivo(t, a, "Ya viene bien")

	id := unPreset(t, a, "no lo toques", model.AjustesDePreset{NoTocar: b(true)})
	asset.PresetID = &id
	if err := a.Store.Media.Update(ctx, &asset); err != nil {
		t.Fatal(err)
	}
	got := a.PresetDe(ctx, asset)
	if got.NoTocar == nil || !*got.NoTocar {
		t.Fatal("el preset dice que no se toque y no llegó")
	}
}
