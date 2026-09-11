package api

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"antena787/internal/app"
	"antena787/internal/ingest"
	"antena787/internal/model"
)

// Verificación por la API de los criterios F1-59 a F1-61 de
// docs/ACEPTACION.md ("Audio de todo el material"): la pista que sale al
// aire se cambia desde Biblioteca, un archivo mudo no tiene forma de salir
// al aire, y la cuarentena dice en clave por qué paró cada archivo.

// conAudio deja en la biblioteca un archivo con dos pistas de sonido —una en
// inglés y otra en español—, ya normalizado y con sus archivos de al lado
// apuntados.
func (c *cliente) conAudio(ruta string, pistaAire int) model.MediaAsset {
	c.t.Helper()
	asset := model.MediaAsset{
		Path:           ruta,
		DurationMs:     45 * 60 * 1000,
		State:          model.AssetReady,
		NormalizeState: ingest.NormalizeReady,
		NormalizedPath: ruta + "-casa.mkv",
		PistasAudio: []model.PistaAudio{
			{Indice: 0, Idioma: "en", Canales: 2, Titulo: "Original en inglés"},
			{Indice: 1, Idioma: "es", Canales: 2, Titulo: "Doblaje en español"},
		},
		PistaAudioAire:    pistaAire,
		AudioSidecar:      ruta + ".wav",
		SubtitulosSidecar: ruta + ".srt",
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
	}
	if err := c.a.Store.Media.Insert(context.Background(), &asset); err != nil {
		c.t.Fatalf("no pude guardar el archivo: %v", err)
	}
	return asset
}

// enCuarentena deja un archivo parado con el motivo que se le pida.
func (c *cliente) enCuarentena(ruta, motivo string) model.MediaAsset {
	c.t.Helper()
	asset := model.MediaAsset{
		Path:           ruta,
		DurationMs:     30 * 1000,
		State:          model.AssetQuarantine,
		NormalizeState: ingest.NormalizePending,
		PlainReason:    motivo,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	if err := c.a.Store.Media.Insert(context.Background(), &asset); err != nil {
		c.t.Fatalf("no pude guardar el archivo: %v", err)
	}
	return asset
}

// motivoDeArchivoMudo es el motivo tal como lo escribe el ingest cuando un
// video no trae sonido y no hay audio a su lado (internal/ingest, F1-59).
const motivoDeArchivoMudo = "«promo.mp4» no trae sonido: pon a su lado un archivo de audio con el mismo nombre " +
	"(.wav, .m4a, .aac, .mp3 o .flac) y lo vuelvo a procesar"

// ── F1-61 · cambiar la pista que sale al aire ─────────────────────────

func TestF1_61_CambiarLaPistaDevuelveElArchivoALaCola(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	ctx := context.Background()
	asset := c.conAudio("/medios/Casablanca.mkv", 0)

	antes := c.a.Queue.Len()
	w := c.do("PUT", "/api/v1/material/"+itoa(asset.ID), map[string]any{"pista_audio_aire": 1})
	if w.Code != http.StatusOK {
		t.Fatalf("cambiar la pista dio %d: %s", w.Code, w.Body.String())
	}

	var respuesta struct {
		ID             int64  `json:"id"`
		MaterialID     int64  `json:"material_id"`
		PistaAudioAire int    `json:"pista_audio_aire"`
		Estado         string `json:"estado_material"`
	}
	c.json(w, &respuesta)
	if respuesta.PistaAudioAire != 1 {
		t.Errorf("la respuesta dice que sale la pista %d y se pidió la 1", respuesta.PistaAudioAire)
	}
	if respuesta.MaterialID != asset.ID || respuesta.ID != asset.ID {
		t.Errorf("la respuesta habla del archivo %d/%d y es el %d", respuesta.ID, respuesta.MaterialID, asset.ID)
	}
	if respuesta.Estado != EstadoNoListo {
		t.Errorf("después de cambiar la pista el archivo dice %q y tenía que decir %q", respuesta.Estado, EstadoNoListo)
	}

	guardado, err := c.a.Store.Media.Get(ctx, asset.ID)
	if err != nil {
		t.Fatalf("no pude releer el archivo: %v", err)
	}
	if guardado.PistaAudioAire != 1 {
		t.Errorf("en la base sigue saliendo la pista %d", guardado.PistaAudioAire)
	}
	if guardado.NormalizeState != ingest.NormalizePending {
		t.Errorf("el archivo quedó en %q y tenía que volver a %q", guardado.NormalizeState, ingest.NormalizePending)
	}
	if guardado.NormalizedPath != "" {
		t.Errorf("la copia de casa vieja sigue puesta: %q", guardado.NormalizedPath)
	}
	if c.a.Queue.Len() <= antes {
		t.Errorf("el archivo no volvió a la cola de normalización (había %d, hay %d)", antes, c.a.Queue.Len())
	}

	// Y Biblioteca lo cuenta igual.
	title := model.Title{Name: "Casablanca", Kind: model.TitleMovie, MediaAssetID: &asset.ID}
	if err := c.a.Store.Title.Insert(ctx, &title); err != nil {
		t.Fatalf("no pude guardar el título: %v", err)
	}
	w = c.do("GET", "/api/v1/biblioteca/"+itoa(title.ID), nil)
	var ficha map[string]any
	c.json(w, &ficha)
	if ficha["estado_material"] != EstadoNoListo {
		t.Errorf("Biblioteca dice %v mientras se rehace la copia de casa", ficha["estado_material"])
	}
	if ficha["pista_audio_aire"] != float64(1) {
		t.Errorf("Biblioteca dice que sale la pista %v", ficha["pista_audio_aire"])
	}
}

func TestF1_61_UnaPistaQueElArchivoNoTraeNoSeGuarda(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	asset := c.conAudio("/medios/Vertigo.mkv", 0)

	for _, indice := range []int{2, 7, -1} {
		w := c.do("PUT", "/api/v1/material/"+itoa(asset.ID), map[string]any{"pista_audio_aire": indice})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("pedir la pista %d dio %d: %s", indice, w.Code, w.Body.String())
		}
		var e errorBody
		c.json(w, &e)
		if e.Error == "" {
			t.Errorf("la negativa no explica nada: %s", w.Body.String())
		}
		if e.Field != "pista_audio_aire" {
			t.Errorf("la negativa no dice qué campo arreglar: %q", e.Field)
		}
	}

	guardado, err := c.a.Store.Media.Get(context.Background(), asset.ID)
	if err != nil {
		t.Fatalf("no pude releer el archivo: %v", err)
	}
	if guardado.PistaAudioAire != 0 || guardado.NormalizeState != ingest.NormalizeReady {
		t.Errorf("un índice que no existe movió el archivo: pista %d, %q",
			guardado.PistaAudioAire, guardado.NormalizeState)
	}

	// Un archivo que no está es 404, no 400.
	if w := c.do("PUT", "/api/v1/material/9999", map[string]any{"pista_audio_aire": 0}); w.Code != http.StatusNotFound {
		t.Fatalf("cambiar la pista de un archivo que no existe dio %d: %s", w.Code, w.Body.String())
	}
}

// ── F1-59 · lo mudo no sale al aire ───────────────────────────────────

func TestF1_59_ElArchivoMudoNoSeDejaPasar(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	ctx := context.Background()
	mudo := c.enCuarentena("/medios/promo.mp4", motivoDeArchivoMudo)

	w := c.do("POST", "/api/v1/cuarentena/"+itoa(mudo.ID)+"/dejar-pasar", map[string]string{"quien": "Rolando"})
	if w.Code != http.StatusConflict {
		t.Fatalf("dejar pasar un archivo mudo dio %d: %s", w.Code, w.Body.String())
	}
	var e errorBody
	c.json(w, &e)
	if !strings.Contains(e.Error, "no trae sonido") || !strings.Contains(e.Error, "pon a su lado") {
		t.Errorf("la negativa no dice qué hacer: %q", e.Error)
	}

	guardado, err := c.a.Store.Media.Get(ctx, mudo.ID)
	if err != nil {
		t.Fatalf("no pude releer el archivo: %v", err)
	}
	if guardado.State != model.AssetQuarantine || guardado.LetThroughBy != "" {
		t.Errorf("el archivo mudo salió de cuarentena igual: %q, %q", guardado.State, guardado.LetThroughBy)
	}
}

func TestLaCuarentenaDiceElCodigoDelMotivo(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	mudo := c.enCuarentena("/medios/promo.mp4", motivoDeArchivoMudo)
	roto := c.enCuarentena("/medios/cortado.mp4",
		"el archivo «cortado.mp4» dura cero: está incompleto o se cortó al copiarlo")

	w := c.do("GET", "/api/v1/cuarentena", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("/cuarentena dio %d: %s", w.Code, w.Body.String())
	}
	var lista []struct {
		ID           int64  `json:"id"`
		Motivo       string `json:"motivo_claro"`
		MotivoCodigo string `json:"motivo_codigo"`
	}
	c.json(w, &lista)
	codigos := map[int64]string{}
	for _, uno := range lista {
		codigos[uno.ID] = uno.MotivoCodigo
	}
	if codigos[mudo.ID] != ingest.MotivoSinAudio {
		t.Errorf("el archivo mudo sale con el código %q", codigos[mudo.ID])
	}
	if codigos[roto.ID] != "" {
		t.Errorf("el archivo cortado sale con el código %q y no tiene ninguno", codigos[roto.ID])
	}
	// Y el que no es mudo sí se puede dejar pasar.
	if w := c.do("POST", "/api/v1/cuarentena/"+itoa(roto.ID)+"/dejar-pasar",
		map[string]string{"quien": "Rolando"}); w.Code != http.StatusOK {
		t.Fatalf("dejar pasar un archivo que no es mudo dio %d: %s", w.Code, w.Body.String())
	}

	// La pantalla de cuarentena lee estos campos (web/src/lib/tipos.ts).
	uno := primero(t, w.Body.Bytes())
	exigeTodas(t, "GET /cuarentena", uno, "EnCuarentena", "titulo")
}

// ── F1-60 · el idioma preferido del canal ─────────────────────────────

func TestF1_60_ElIdiomaDeAudioSeGuardaEnAjustes(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	listo := c.conAudio("/medios/Kojak.mkv", 0)
	antes := c.a.Queue.Len()

	w := c.do("PUT", "/api/v1/ajustes", map[string]string{app.KeyAudioLanguage: "en"})
	if w.Code != http.StatusOK {
		t.Fatalf("guardar el idioma dio %d: %s", w.Code, w.Body.String())
	}
	var ajustes map[string]string
	c.json(w, &ajustes)
	if ajustes[app.KeyAudioLanguage] != "en" {
		t.Errorf("el PUT contestó %q", ajustes[app.KeyAudioLanguage])
	}

	w = c.do("GET", "/api/v1/ajustes", nil)
	c.json(w, &ajustes)
	if ajustes[app.KeyAudioLanguage] != "en" {
		t.Errorf("el ajuste no se guardó: %q", ajustes[app.KeyAudioLanguage])
	}

	// Cambiar el idioma vale para lo que entre a partir de ahora: lo que ya
	// está listo no se vuelve a procesar solo (F1-60).
	guardado, err := c.a.Store.Media.Get(context.Background(), listo.ID)
	if err != nil {
		t.Fatalf("no pude releer el archivo: %v", err)
	}
	if guardado.NormalizeState != ingest.NormalizeReady || guardado.PistaAudioAire != 0 {
		t.Errorf("cambiar el idioma movió un archivo que ya estaba listo: %q, pista %d",
			guardado.NormalizeState, guardado.PistaAudioAire)
	}
	if c.a.Queue.Len() != antes {
		t.Errorf("cambiar el idioma metió %d archivos en la cola", c.a.Queue.Len()-antes)
	}
}
