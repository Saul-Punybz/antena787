package store

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"antena787/internal/model"
)

// conPistas deja en la biblioteca un archivo con tres pistas de sonido, ya
// normalizado, y devuelve su id.
func conPistas(t *testing.T, s *Store, ruta string) *model.MediaAsset {
	t.Helper()
	ctx := context.Background()
	sap := 1
	a := &model.MediaAsset{
		Path: ruta, Codec: "h264", Resolution: "1920x1080", FPS: "29.97",
		AudioChannels: 2, DurationMs: 1_800_000,
		State:          model.AssetReady,
		NormalizeState: "listo",
		NormalizedPath: ruta + ".casa.mkv",
		PistasAudio: []model.PistaAudio{
			{Indice: 0, Idioma: "en", Canales: 6, Titulo: "English 5.1"},
			{Indice: 1, Idioma: "es", Canales: 2, Titulo: "Español"},
			{Indice: 2, Idioma: "por", Canales: 2, Titulo: "Português"},
		},
		PistaAudioAire:    1,
		PistaAudioSAP:     &sap,
		AudioSidecar:      ruta + ".wav",
		SubtitulosSidecar: ruta + ".srt",
	}
	if err := s.Media.Insert(ctx, a); err != nil {
		t.Fatalf("no guardó el archivo: %v", err)
	}
	return a
}

// F1-58, F1-62, F1-63 — todo lo que el ingest averigua del sonido y de los
// archivos de al lado se guarda y vuelve igual.
func TestPistasDeAudioIdaYVuelta(t *testing.T) {
	ctx := context.Background()
	s, _ := abrir(t)

	ruta := filepath.Join("biblioteca", "pelicula.mp4")
	a := conPistas(t, s, ruta)

	leido, err := s.Media.Get(ctx, a.ID)
	if err != nil {
		t.Fatalf("no lo encontró: %v", err)
	}
	if len(leido.PistasAudio) != 3 {
		t.Fatalf("las pistas se perdieron: %+v", leido.PistasAudio)
	}
	if p := leido.PistasAudio[1]; p.Indice != 1 || p.Idioma != "es" || p.Canales != 2 || p.Titulo != "Español" {
		t.Fatalf("la pista en español volvió cambiada: %+v", p)
	}
	if leido.PistaAudioAire != 1 {
		t.Fatalf("la pista al aire volvió como %d, se esperaba 1", leido.PistaAudioAire)
	}
	if leido.PistaAudioSAP == nil || *leido.PistaAudioSAP != 1 {
		t.Fatalf("la segunda pista al aire se perdió: %+v", leido.PistaAudioSAP)
	}
	if leido.AudioSidecar != ruta+".wav" {
		t.Fatalf("el audio de al lado se perdió: %q", leido.AudioSidecar)
	}
	if leido.SubtitulosSidecar != ruta+".srt" {
		t.Fatalf("los subtítulos de al lado se perdieron: %q", leido.SubtitulosSidecar)
	}

	// Y por la lista sale igual.
	listos, err := s.Media.ListReady(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(listos) != 1 || len(listos[0].PistasAudio) != 3 || listos[0].AudioSidecar != ruta+".wav" {
		t.Fatalf("la lista no trae el sonido: %+v", listos)
	}
}

// Un archivo sin nada de esto no se rompe: la lista de pistas vuelve vacía,
// la pista al aire es la primera y la segunda no existe (F1-63).
func TestArchivoSinPistasDeclaradas(t *testing.T) {
	ctx := context.Background()
	s, _ := abrir(t)

	a := &model.MediaAsset{Path: filepath.Join("biblioteca", "spot.mp4")}
	if err := s.Media.Insert(ctx, a); err != nil {
		t.Fatal(err)
	}
	leido, err := s.Media.Get(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(leido.PistasAudio) != 0 {
		t.Fatalf("no debía traer pistas: %+v", leido.PistasAudio)
	}
	if leido.PistaAudioAire != 0 {
		t.Fatalf("la pista al aire por defecto es la primera, salió %d", leido.PistaAudioAire)
	}
	if leido.PistaAudioSAP != nil {
		t.Fatalf("la segunda pista al aire tiene que estar vacía hasta F2: %+v", leido.PistaAudioSAP)
	}
	if leido.AudioSidecar != "" || leido.SubtitulosSidecar != "" {
		t.Fatalf("no había archivos al lado: %q / %q", leido.AudioSidecar, leido.SubtitulosSidecar)
	}
}

// Update guarda los cambios del sonido como los de cualquier otro campo.
func TestUpdateGuardaElSonido(t *testing.T) {
	ctx := context.Background()
	s, _ := abrir(t)

	a := conPistas(t, s, filepath.Join("biblioteca", "serie.mp4"))
	a.PistaAudioAire = 2
	a.PistaAudioSAP = nil
	a.AudioSidecar = ""
	a.SubtitulosSidecar = filepath.Join("biblioteca", "serie.scc")
	a.PistasAudio = append(a.PistasAudio, model.PistaAudio{Indice: 3, Idioma: "fr", Canales: 2, Titulo: "Français"})
	if err := s.Media.Update(ctx, a); err != nil {
		t.Fatalf("no guardó los cambios: %v", err)
	}

	leido, err := s.Media.Get(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if leido.PistaAudioAire != 2 || leido.PistaAudioSAP != nil || leido.AudioSidecar != "" {
		t.Fatalf("los cambios del sonido no entraron: %+v", leido)
	}
	if len(leido.PistasAudio) != 4 || leido.PistasAudio[3].Idioma != "fr" {
		t.Fatalf("la pista nueva no entró: %+v", leido.PistasAudio)
	}
	if !strings.HasSuffix(leido.SubtitulosSidecar, ".scc") {
		t.Fatalf("los subtítulos de al lado no entraron: %q", leido.SubtitulosSidecar)
	}
}

// F1-61 — cambiar la pista al aire guarda el cambio y devuelve el archivo a
// la cola de normalización: hasta que la copia de casa se rehaga, no está
// listo para aire.
func TestSetPistaAudioAireVuelveALaColaDeNormalizacion(t *testing.T) {
	ctx := context.Background()
	s, _ := abrir(t)

	a := conPistas(t, s, filepath.Join("biblioteca", "clasico.mp4"))
	if err := s.Media.SetPistaAudioAire(ctx, a.ID, 2); err != nil {
		t.Fatalf("no cambió la pista: %v", err)
	}

	leido, err := s.Media.Get(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if leido.PistaAudioAire != 2 {
		t.Fatalf("la pista al aire quedó en %d, se esperaba 2", leido.PistaAudioAire)
	}
	if leido.NormalizeState != "pendiente" {
		t.Fatalf("el archivo tenía que volver a la cola de normalización, quedó en %q", leido.NormalizeState)
	}
	if leido.NormalizedPath != "" {
		t.Fatalf("la copia de casa vieja ya no vale: %q", leido.NormalizedPath)
	}
	// Las demás pistas siguen ahí: cambiar cuál suena no borra el resto.
	if len(leido.PistasAudio) != 3 {
		t.Fatalf("las pistas se perdieron al cambiar la que va al aire: %+v", leido.PistasAudio)
	}

	// Y el cambio queda anotado en la bitácora, en palabras claras.
	entradas, err := s.Audit.List(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(entradas) != 1 {
		t.Fatalf("se esperaba una anotación en la bitácora, hay %d", len(entradas))
	}
	e := entradas[0]
	if e.Entity != "media_asset" || e.EntityID == nil || *e.EntityID != a.ID || e.Field != "pista_audio_aire" {
		t.Fatalf("la anotación no apunta al archivo: %+v", e)
	}
	if !strings.Contains(e.Before, "Español") || !strings.Contains(e.After, "Português") {
		t.Fatalf("la anotación no dice qué cambió: %q -> %q", e.Before, e.After)
	}
	if _, err := s.Audit.Verify(ctx); err != nil {
		t.Fatalf("la cadena de la bitácora se rompió: %v", err)
	}
}

// Una pista que el archivo no trae no se puede poner al aire, y el intento no
// deja nada a medias.
func TestSetPistaAudioAireRechazaUnaPistaQueNoExiste(t *testing.T) {
	ctx := context.Background()
	s, _ := abrir(t)

	a := conPistas(t, s, filepath.Join("biblioteca", "documental.mp4"))
	for _, indice := range []int{3, 99, -1} {
		err := s.Media.SetPistaAudioAire(ctx, a.ID, indice)
		if !errors.Is(err, ErrPistaInexistente) {
			t.Fatalf("pista %d: se esperaba ErrPistaInexistente, salió: %v", indice, err)
		}
		// Ni jerga ni códigos: el mensaje se le enseña a una persona.
		for _, palabra := range []string{"a:", "index", "ffmpeg", "stream"} {
			if strings.Contains(strings.ToLower(err.Error()), palabra) {
				t.Fatalf("el mensaje tiene jerga (%q): %v", palabra, err)
			}
		}
	}

	leido, err := s.Media.Get(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if leido.PistaAudioAire != 1 || leido.NormalizeState != "listo" || leido.NormalizedPath == "" {
		t.Fatalf("un intento fallido no puede tocar nada: %+v", leido)
	}
	entradas, err := s.Audit.List(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(entradas) != 0 {
		t.Fatalf("un intento fallido no se anota en la bitácora: %+v", entradas)
	}
}

// Un archivo que no existe se contesta con ErrNotFound, como en el resto del
// repositorio.
func TestSetPistaAudioAireArchivoQueNoExiste(t *testing.T) {
	ctx := context.Background()
	s, _ := abrir(t)

	if err := s.Media.SetPistaAudioAire(ctx, 9999, 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("se esperaba ErrNotFound, salió: %v", err)
	}
}

// De un archivo del que no se sabe qué pistas trae solo se puede elegir la
// primera: es la única que se sabe que existe.
func TestSetPistaAudioAireSinPistasDeclaradas(t *testing.T) {
	ctx := context.Background()
	s, _ := abrir(t)

	a := &model.MediaAsset{Path: filepath.Join("biblioteca", "cortinilla.mp4"), NormalizeState: "listo"}
	if err := s.Media.Insert(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := s.Media.SetPistaAudioAire(ctx, a.ID, 0); err != nil {
		t.Fatalf("la primera pista siempre vale: %v", err)
	}
	if err := s.Media.SetPistaAudioAire(ctx, a.ID, 1); !errors.Is(err, ErrPistaInexistente) {
		t.Fatalf("se esperaba ErrPistaInexistente, salió: %v", err)
	}
}

// F1-60 en la base: lo que se guardó es lo que la ayuda del modelo usa para
// elegir la pista del canal.
func TestPistaPreferidaSobreLoGuardado(t *testing.T) {
	ctx := context.Background()
	s, _ := abrir(t)

	a := conPistas(t, s, filepath.Join("biblioteca", "estreno.mp4"))
	leido, err := s.Media.Get(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := leido.PistaPreferida("es"); got != 1 {
		t.Fatalf("la pista en español es la 1, salió %d", got)
	}
	if got := leido.PistaPreferida("de"); got != 0 {
		t.Fatalf("sin alemán se va con la primera del archivo, salió %d", got)
	}
}
