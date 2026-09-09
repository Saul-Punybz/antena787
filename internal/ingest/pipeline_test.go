package ingest

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"antena787/internal/engine"
	"antena787/internal/model"
)

const srtBueno = `1
00:00:00,500 --> 00:00:02,000
Buenas noches, esto es Antena787.

2
00:00:02,100 --> 00:00:04,000
La segunda línea.
`

func TestSubtitulosAlLado(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "Kojak S01E03.mp4")
	if err := os.WriteFile(video, []byte("no importa"), 0o644); err != nil {
		t.Fatal(err)
	}
	srt := filepath.Join(dir, "Kojak S01E03.srt")
	if err := os.WriteFile(srt, []byte(srtBueno), 0o644); err != nil {
		t.Fatal(err)
	}

	got, ok := FindSidecar(video)
	if !ok || got != srt {
		t.Fatalf("FindSidecar = %q, %v; se esperaba %q", got, ok, srt)
	}

	var asset model.MediaAsset
	if err := AttachCaptions(&asset, srt); err != nil {
		t.Fatalf("AttachCaptions: %v", err)
	}
	if asset.ExternalCaptions == nil || *asset.ExternalCaptions != srt {
		t.Errorf("subtitulos_externos = %v, se esperaba %q", asset.ExternalCaptions, srt)
	}
	if !asset.HasCaptions || asset.CaptionFormat != "srt" {
		t.Errorf("tiene_subtitulos=%v formato=%q", asset.HasCaptions, asset.CaptionFormat)
	}

	// El .scc manda sobre el .srt: ya es CEA-608 y no hay que convertirlo.
	scc := filepath.Join(dir, "Kojak S01E03.scc")
	if err := os.WriteFile(scc, []byte("Scenarist_SCC V1.0\n\n00:00:01:00\t94ae 94ae\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, _ := FindSidecar(video); got != scc {
		t.Errorf("con .scc presente FindSidecar devolvió %q", got)
	}
}

func TestSubtitulosRotosSeRechazan(t *testing.T) {
	dir := t.TempDir()
	casos := map[string]string{
		"vacio.srt":   "",
		"basura.srt":  "esto no es un subtítulo, es un documento de texto\ncon dos líneas",
		"tiempos.srt": "1\n00:00:0X,000 --> 00:00:02,000\nhola\n",
		"falso.scc":   "esto no es Scenarist\n",
		"raro.vtt":    "1\n00:00:01.000 --> 00:00:02.000\nsin cabecera WEBVTT\n",
	}
	for name, body := range casos {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := ValidateSidecar(p); err == nil {
			t.Errorf("%s: se esperaba un error y pasó", name)
		} else if Plain(err) == "" {
			t.Errorf("%s: el error no trae motivo en cristiano", name)
		}
	}
	ok := filepath.Join(dir, "bueno.vtt")
	if err := os.WriteFile(ok, []byte("WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nhola\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if f, err := ValidateSidecar(ok); err != nil || f != "vtt" {
		t.Errorf("ValidateSidecar(vtt) = %q, %v", f, err)
	}
}

func TestNFODeKodi(t *testing.T) {
	dir := t.TempDir()
	nfo := filepath.Join(dir, "pelicula.nfo")
	body := `<?xml version="1.0" encoding="UTF-8"?>
<movie>
  <title>El Cartero</title>
  <plot>Un cartero en una isla.</plot>
  <year>1994</year>
  <genre>Drama</genre>
  <genre>Romance</genre>
  <mpaa>PG</mpaa>
  <thumb aspect="poster">http://ejemplo/poster.jpg</thumb>
</movie>`
	if err := os.WriteFile(nfo, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := ReadNFO(nfo)
	if err != nil {
		t.Fatalf("ReadNFO: %v", err)
	}
	if c.Name != "El Cartero" || c.Year != 1994 || c.Genre != "Drama" || c.ContentRating != "PG" {
		t.Errorf("ficha inesperada: %+v", c)
	}
	if c.Kind != model.TitleMovie {
		t.Errorf("tipo = %q, se esperaba película", c.Kind)
	}
	if c.Source != "nfo-local" {
		t.Errorf("fuente = %q", c.Source)
	}

	malo := filepath.Join(dir, "malo.nfo")
	if err := os.WriteFile(malo, []byte("<html><body>esto no</body></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadNFO(malo); err == nil {
		t.Error("un .nfo que no es de Kodi debería dar error")
	}
}

func TestMetadataNoTocaLaRedSinProveedores(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "Fiesta en la Placita.mp4")
	if err := os.WriteFile(video, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	poster := filepath.Join(dir, "poster.jpg")
	if err := os.WriteFile(poster, []byte("jpeg"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := Measure{DurationMs: 1_800_000, HasVideo: true, HasAudio: true,
		Tags: Tags{Title: "Fiesta en la Placita", Date: "2019", Synopsis: "Música y baile."}}

	// Sin proveedores no hay red que valga: si alguno se colara, la lista
	// vacía lo haría imposible (F1-08).
	c, err := Metadata(context.Background(), video, m, MetadataDeps{})
	if err != nil {
		t.Fatalf("Metadata: %v", err)
	}
	if c.Name != "Fiesta en la Placita" || c.Year != 2019 {
		t.Errorf("ficha = %+v", c)
	}
	if c.ArtworkPath != poster {
		t.Errorf("carátula = %q, se esperaba %q", c.ArtworkPath, poster)
	}
	if c.Sources[0] != "tags-embebidas" {
		t.Errorf("la primera fuente debe ser local: %v", c.Sources)
	}
}

func TestMetadataLocalAntesQueRed(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "serie.mp4")
	if err := os.WriteFile(video, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	espia := &proveedorEspia{card: &Card{Name: "De la red", Synopsis: "de la red", Source: "falso"}}
	m := Measure{HasVideo: true, HasAudio: true, DurationMs: 1_800_000,
		Tags: Tags{Title: "Kojak", Show: "Kojak", Season: 1, Episode: 3, Synopsis: "Un caso más."}}

	c, err := Metadata(context.Background(), video, m, MetadataDeps{Providers: []Provider{espia}})
	if err != nil {
		t.Fatalf("Metadata: %v", err)
	}
	if c.Synopsis != "Un caso más." {
		t.Errorf("la red pisó lo local: %q", c.Synopsis)
	}
	if espia.veces == 0 {
		t.Error("se esperaba que preguntara por la carátula, que no estaba en local")
	}
	if c.ArtworkURL != "" {
		t.Log("el proveedor de prueba no da carátula, y eso está bien")
	}
	if c.Kind != model.TitleSeries || c.Season != 1 || c.Episode != 3 {
		t.Errorf("ficha de serie mal armada: %+v", c)
	}
}

type proveedorEspia struct {
	card  *Card
	veces int
}

func (p *proveedorEspia) Lookup(ctx context.Context, q Query) (*Card, error) {
	p.veces++
	return p.card, nil
}

func TestWatcherEsperaAQueTermineLaCopia(t *testing.T) {
	dir := t.TempDir()
	reloj := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)
	w, err := NewWatcher(WatcherOptions{Dir: dir, StableFor: 10 * time.Second, Now: func() time.Time { return reloj }})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	destino := filepath.Join(dir, "spot.mp4")

	escribe := func(name string, n int) {
		f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write(make([]byte, n)); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
	sinEventos := func(cuando string) {
		t.Helper()
		select {
		case ev := <-w.Events():
			t.Fatalf("%s: se avisó del archivo antes de tiempo: %v", cuando, ev)
		default:
		}
	}

	// Un temporal no se mira nunca, por mucho que se quede quieto.
	escribe(filepath.Join(dir, "spot.mp4.part"), 4096)
	// El archivo empieza a copiarse.
	escribe(destino, 1000)
	if err := w.Poll(ctx); err != nil {
		t.Fatal(err)
	}
	sinEventos("primera pasada")

	reloj = reloj.Add(3 * time.Second)
	escribe(destino, 1000) // sigue copiándose: el reloj de quietud se reinicia
	if err := w.Poll(ctx); err != nil {
		t.Fatal(err)
	}
	sinEventos("mientras copia")

	reloj = reloj.Add(9 * time.Second) // 9 s quieto: todavía no son 10
	if err := w.Poll(ctx); err != nil {
		t.Fatal(err)
	}
	sinEventos("a los 9 segundos")

	reloj = reloj.Add(2 * time.Second) // 11 s quieto
	if err := w.Poll(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case ev := <-w.Events():
		if ev.Path != destino {
			t.Fatalf("avisó de %q y se esperaba %q", ev.Path, destino)
		}
		if ev.SizeBytes != 2000 {
			t.Errorf("tamaño = %d, se esperaban 2000", ev.SizeBytes)
		}
	default:
		t.Fatal("no avisó del archivo terminado")
	}

	// Y no lo vuelve a anunciar.
	reloj = reloj.Add(time.Minute)
	if err := w.Poll(ctx); err != nil {
		t.Fatal(err)
	}
	sinEventos("segunda vuelta")
}

func TestIngestArchivoVacioVaACuarentena(t *testing.T) {
	dir := t.TempDir()
	vacio := filepath.Join(dir, "vacio.mp4")
	if err := os.WriteFile(vacio, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	asset, _, _, err := Ingest(context.Background(), Deps{}, vacio)
	if err == nil {
		t.Fatal("un archivo de 0 bytes tiene que fallar")
	}
	if asset.State != model.AssetQuarantine {
		t.Errorf("estado = %q, se esperaba cuarentena", asset.State)
	}
	if !strings.Contains(asset.PlainReason, "vacío") {
		t.Errorf("motivo_en_cristiano = %q", asset.PlainReason)
	}
	if asset.PlainReason != Plain(err) {
		t.Error("el motivo del asset y el del error no coinciden")
	}
}

func TestIngestArchivoRotoVaACuarentena(t *testing.T) {
	ffmpeg, ffprobe := tools(t)
	dir := t.TempDir()
	roto := filepath.Join(dir, "roto.mp4")
	if err := os.WriteFile(roto, []byte("esto no es un mp4, es texto"), 0o644); err != nil {
		t.Fatal(err)
	}
	asset, _, _, err := Ingest(context.Background(), Deps{FFmpeg: ffmpeg, FFprobe: ffprobe}, roto)
	if err == nil {
		t.Fatal("un archivo dañado tiene que fallar")
	}
	if asset.State != model.AssetQuarantine {
		t.Errorf("estado = %q, se esperaba cuarentena", asset.State)
	}
	if asset.PlainReason == "" || strings.Contains(asset.PlainReason, "ffprobe") {
		t.Errorf("el motivo no está en cristiano: %q", asset.PlainReason)
	}
}

func TestIngestCompleto(t *testing.T) {
	ffmpeg, ffprobe := tools(t)
	dir := t.TempDir()
	src := makeClip(t, ffmpeg, filepath.Join(dir, "Kojak S01E03.mp4"), "30",
		segment{seconds: 1, black: true, silent: true},
		segment{seconds: 3})
	if err := os.WriteFile(filepath.Join(dir, "Kojak S01E03.srt"), []byte(srtBueno), 0o644); err != nil {
		t.Fatal(err)
	}
	nfo := `<episodedetails><title>Uno de los nuestros</title><showtitle>Kojak</showtitle>` +
		`<season>1</season><episode>3</episode><plot>Kojak investiga.</plot><aired>1973-11-14</aired></episodedetails>`
	if err := os.WriteFile(filepath.Join(dir, "Kojak S01E03.nfo"), []byte(nfo), 0o644); err != nil {
		t.Fatal(err)
	}

	deps := Deps{
		FFmpeg: ffmpeg, FFprobe: ffprobe,
		Format: engine.CAtv, TargetLUFS: -24, TruePeak: -2,
		ThumbnailDir: filepath.Join(dir, "miniaturas"),
		ComputeHash:  true,
		Retry:        RetryPolicy{Delay: 0, Sleep: SleepCtx},
	}
	asset, title, eps, err := Ingest(context.Background(), deps, src)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if asset.State != model.AssetReady {
		t.Errorf("estado = %q, se esperaba listo", asset.State)
	}
	if asset.NormalizeState != NormalizePending {
		t.Errorf("estado_normalizacion = %q, se esperaba pendiente", asset.NormalizeState)
	}
	if asset.DurationMs < 3900 || asset.DurationMs > 4200 {
		t.Errorf("duración = %d ms", asset.DurationMs)
	}
	if asset.HeadBlackMs < 800 || asset.HeadBlackMs > 1400 {
		t.Errorf("negro de cabeza = %d ms, se esperaban ~1000", asset.HeadBlackMs)
	}
	if len(asset.Hash) != 64 {
		t.Errorf("hash = %q", asset.Hash)
	}
	if !fileHasBytes(asset.Thumbnail) {
		t.Errorf("no hay miniatura en %q", asset.Thumbnail)
	}
	if asset.ExternalCaptions == nil {
		t.Error("no se registró el .srt de al lado")
	}
	if title.Name != "Kojak" || title.Kind != model.TitleSeries {
		t.Errorf("título = %+v", title)
	}
	if title.Synopsis != "Kojak investiga." {
		t.Errorf("sinopsis = %q", title.Synopsis)
	}
	if title.Year == nil || *title.Year != 1973 {
		t.Errorf("año = %v", title.Year)
	}
	if !strings.Contains(title.MetadataSource, "nfo-local") {
		t.Errorf("fuente_ficha = %q", title.MetadataSource)
	}
	if len(eps) != 1 || eps[0].Season != 1 || eps[0].Number != 3 {
		t.Fatalf("episodios = %+v", eps)
	}
	if eps[0].Name != "Uno de los nuestros" {
		t.Errorf("nombre del episodio = %q", eps[0].Name)
	}
}

func TestIngestReintentaAntesDeCuarentena(t *testing.T) {
	dir := t.TempDir()
	fantasma := filepath.Join(dir, "nas-caido.mp4")
	var dormidas int
	deps := Deps{Retry: RetryPolicy{Delay: 5 * time.Minute, Sleep: func(ctx context.Context, d time.Duration) error {
		dormidas++
		if d != 5*time.Minute {
			t.Errorf("el reintento esperó %v, el default son 5 minutos", d)
		}
		return nil
	}}}
	asset, _, _, err := Ingest(context.Background(), deps, fantasma)
	if err == nil {
		t.Fatal("un archivo que no está tiene que fallar")
	}
	if dormidas != 1 {
		t.Errorf("reintentos = %d, se esperaba exactamente uno (AUDITORIA C8)", dormidas)
	}
	if asset.State != model.AssetQuarantine {
		t.Errorf("estado = %q", asset.State)
	}
}

func TestColaPriorizaPorHoraDeAire(t *testing.T) {
	q := NewQueue()
	base := time.Date(2026, 9, 8, 18, 0, 0, 0, time.UTC)
	q.Enqueue(10, base.Add(3*time.Hour)) // el de las 21:00
	q.Enqueue(20, time.Time{})           // sin fecha de aire: al final
	q.Enqueue(30, base)                  // el de las 18:00
	q.Enqueue(40, base.Add(time.Hour))   // el de las 19:00
	q.Enqueue(50, base)                  // empata con el 30: gana el que llegó antes

	var orden []int64
	for {
		j, ok := q.Pop()
		if !ok {
			break
		}
		orden = append(orden, j.AssetID)
	}
	esperado := []int64{30, 50, 40, 10, 20}
	if len(orden) != len(esperado) {
		t.Fatalf("orden = %v", orden)
	}
	for i := range esperado {
		if orden[i] != esperado[i] {
			t.Fatalf("orden = %v, se esperaba %v", orden, esperado)
		}
	}
}

func TestColaMarcaFallidoDespuesDeDosIntentos(t *testing.T) {
	q := NewQueue()
	q.RetryDelay = 0 // sin esperas en la prueba
	q.Enqueue(7, time.Now())

	p := &persistenciaDePrueba{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var mu sync.Mutex
	intentos := 0
	go func() {
		_ = q.Run(ctx, p, func(ctx context.Context, j Job) (string, error) {
			mu.Lock()
			intentos++
			mu.Unlock()
			return "", Plainf(nil, "ffmpeg se quedó sin memoria")
		})
	}()
	esperaHasta(t, time.Second, func() bool { return p.last(7).state == NormalizeFailed })
	cancel()

	mu.Lock()
	defer mu.Unlock()
	if intentos != 2 {
		t.Errorf("intentos = %d, se esperaban 2", intentos)
	}
	if got := p.last(7); got.state != NormalizeFailed {
		t.Errorf("estado final = %q, se esperaba fallido", got.state)
	} else if !strings.Contains(got.reason, "ffmpeg se quedó sin memoria") {
		t.Errorf("motivo = %q", got.reason)
	}
}

// esperaHasta sondea una condición hasta que se cumple o se acaba el plazo.
func esperaHasta(t *testing.T, plazo time.Duration, cond func() bool) {
	t.Helper()
	limite := time.Now().Add(plazo)
	for time.Now().Before(limite) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("la condición no se cumplió en %v", plazo)
}

type estadoNorm struct{ state, path, reason string }

type registroVolumen struct {
	lufs, truePeak float64
	pasadas        int
	nota           string
}

// registroAudio es lo que la base guardaría de las pistas de sonido y de los
// archivos de al lado (F1-58, F1-60, F1-62).
type registroAudio struct {
	pistas      []AudioTrack
	pistaAire   int
	audio       string
	subtitulos  string
	vecesPistas int
}

type persistenciaDePrueba struct {
	mu      sync.Mutex
	assets  []model.MediaAsset
	estados map[int64][]estadoNorm
	volumen map[int64]registroVolumen
	audio   map[int64]registroAudio
}

func (p *persistenciaDePrueba) SaveAsset(ctx context.Context, a *model.MediaAsset) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	// Un archivo que ya tiene ID se actualiza en su sitio: es lo que pasa
	// cuando el ingest vuelve a correr sobre el mismo video porque apareció
	// el audio de al lado (F1-58).
	if a.ID != 0 {
		for i := range p.assets {
			if p.assets[i].ID == a.ID {
				p.assets[i] = *a
				return nil
			}
		}
	}
	for i := range p.assets {
		if p.assets[i].Path == a.Path {
			a.ID = p.assets[i].ID
			p.assets[i] = *a
			return nil
		}
	}
	a.ID = int64(len(p.assets) + 1)
	p.assets = append(p.assets, *a)
	return nil
}

// SetAudioTracks y SetSidecars guardan lo que el ingest averiguó del sonido.
func (p *persistenciaDePrueba) SetAudioTracks(ctx context.Context, id int64, pistas []AudioTrack, pistaAire int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.audio == nil {
		p.audio = map[int64]registroAudio{}
	}
	r := p.audio[id]
	r.pistas, r.pistaAire, r.vecesPistas = pistas, pistaAire, r.vecesPistas+1
	p.audio[id] = r
	return nil
}

func (p *persistenciaDePrueba) SetSidecars(ctx context.Context, id int64, audio, subtitulos string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.audio == nil {
		p.audio = map[int64]registroAudio{}
	}
	r := p.audio[id]
	r.audio, r.subtitulos = audio, subtitulos
	p.audio[id] = r
	return nil
}

func (p *persistenciaDePrueba) sonido(id int64) registroAudio {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.audio[id]
}

// guardado devuelve el asset con ese ID tal como quedó en la "base".
func (p *persistenciaDePrueba) guardado(id int64) (model.MediaAsset, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, a := range p.assets {
		if a.ID == id {
			return a, true
		}
	}
	return model.MediaAsset{}, false
}

func (p *persistenciaDePrueba) SetNormalizeState(ctx context.Context, id int64, state, path, reason string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.estados == nil {
		p.estados = map[int64][]estadoNorm{}
	}
	p.estados[id] = append(p.estados[id], estadoNorm{state, path, reason})
	return nil
}

// SetLoudness hace de persistenciaDePrueba una PersistLoudness: guarda el
// registro de volumen igual que lo guardaría la base.
func (p *persistenciaDePrueba) SetLoudness(ctx context.Context, id int64, lufs, truePeak float64, pasadas int, nota string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.volumen == nil {
		p.volumen = map[int64]registroVolumen{}
	}
	p.volumen[id] = registroVolumen{lufs, truePeak, pasadas, nota}
	return nil
}

func (p *persistenciaDePrueba) loudness(id int64) registroVolumen {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.volumen[id]
}

// TestSaveLoudnessGuardaElRegistroDeLasDosPasadas comprueba que el reporte
// que devuelve Normalize se puede guardar tal cual (F1-03), y que una
// persistencia que no sepa hacerlo no rompe nada.
func TestSaveLoudnessGuardaElRegistroDeLasDosPasadas(t *testing.T) {
	ctx := context.Background()
	rep := LoudnessReport{OutputLUFS: -24.1, OutputTruePeak: -2.4, Passes: 2, CaptionsNote: "sin novedad"}

	p := &persistenciaDePrueba{}
	guardado, err := SaveLoudness(ctx, p, 7, rep)
	if err != nil {
		t.Fatalf("SaveLoudness: %v", err)
	}
	if !guardado {
		t.Fatal("la persistencia de prueba sí sabe guardar el volumen")
	}
	if got := p.loudness(7); got.lufs != -24.1 || got.truePeak != -2.4 || got.pasadas != 2 || got.nota != "sin novedad" {
		t.Errorf("registro guardado = %+v", got)
	}

	// Una persistencia de las de antes no rompe: solo no guarda.
	guardado, err = SaveLoudness(ctx, soloEstado{}, 7, rep)
	if err != nil || guardado {
		t.Errorf("guardado=%v err=%v; se esperaba que no guardara y no fallara", guardado, err)
	}
}

// soloEstado es una persistencia que no sabe de volumen.
type soloEstado struct{}

func (soloEstado) SaveAsset(context.Context, *model.MediaAsset) error { return nil }
func (soloEstado) SetNormalizeState(context.Context, int64, string, string, string) error {
	return nil
}
func (soloEstado) SetAudioTracks(context.Context, int64, []AudioTrack, int) error { return nil }
func (soloEstado) SetSidecars(context.Context, int64, string, string) error       { return nil }

func (p *persistenciaDePrueba) last(id int64) estadoNorm {
	p.mu.Lock()
	defer p.mu.Unlock()
	l := p.estados[id]
	if len(l) == 0 {
		return estadoNorm{}
	}
	return l[len(l)-1]
}

func TestValidateUploadRechazaEnCristiano(t *testing.T) {
	ffmpeg, ffprobe := tools(t)
	dir := t.TempDir()
	ctx := context.Background()

	bueno := makeClip(t, ffmpeg, filepath.Join(dir, "spot.mp4"), "30", segment{seconds: 3})
	if _, err := ValidateUpload(ctx, ffprobe, bueno, 0, 0); err != nil {
		t.Fatalf("un spot normal debería pasar: %v", err)
	}

	// Dura más de lo permitido.
	if _, err := ValidateUpload(ctx, ffprobe, bueno, 0, time.Second); err == nil {
		t.Error("un archivo más largo que el máximo debe rechazarse")
	} else if !strings.Contains(Plain(err), "no parece un anuncio") {
		t.Errorf("motivo = %q", Plain(err))
	}

	// Pesa más de lo permitido.
	if _, err := ValidateUpload(ctx, ffprobe, bueno, 100, 0); err == nil {
		t.Error("un archivo más pesado que el máximo debe rechazarse")
	} else if !strings.Contains(Plain(err), "pesa") {
		t.Errorf("motivo = %q", Plain(err))
	}

	// Sin sonido.
	mudo := filepath.Join(dir, "mudo.mp4")
	out, err := exec.Command(ffmpeg, "-nostdin", "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-t", "2", "-i", "testsrc2=s=320x240:r=30",
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", mudo).CombinedOutput()
	if err != nil {
		t.Fatalf("no se pudo fabricar el mudo: %v\n%s", err, out)
	}
	if _, err := ValidateUpload(ctx, ffprobe, mudo, 0, 0); err == nil {
		t.Error("un anuncio sin sonido debe rechazarse")
	} else if !strings.Contains(Plain(err), "sonido") {
		t.Errorf("motivo = %q", Plain(err))
	}

	// Vacío y con extensión que no es de medios.
	vacio := filepath.Join(dir, "vacio.mp4")
	if err := os.WriteFile(vacio, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateUpload(ctx, ffprobe, vacio, 0, 0); err == nil {
		t.Error("un archivo vacío debe rechazarse")
	}
	raro := filepath.Join(dir, "anuncio.exe")
	if err := os.WriteFile(raro, []byte("MZ"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateUpload(ctx, ffprobe, raro, 0, 0); err == nil {
		t.Error("un ejecutable no es un anuncio")
	}
}

func TestCheckDurationHablaComoUnaPersona(t *testing.T) {
	m := Measure{DurationMs: 32_000}
	err := CheckDuration(m, 30*time.Second)
	if err == nil {
		t.Fatal("32 segundos contra 30 comprados tiene que avisar")
	}
	if !strings.Contains(Plain(err), "32 segundos") || !strings.Contains(Plain(err), "30 segundos") {
		t.Errorf("motivo = %q", Plain(err))
	}
	if err := CheckDuration(Measure{DurationMs: 30_200}, 30*time.Second); err != nil {
		t.Errorf("200 ms de diferencia entran en la tolerancia: %v", err)
	}
}

// ── imagen y sonido de distinta duración (F1-70) ──────────────────────

// clipDesfasado fabrica un archivo cuya imagen dura más que su sonido, como
// queda uno que se cortó al copiarlo por la pista de audio.
func clipDesfasado(t *testing.T, ffmpeg, dst string, videoS, audioS float64) string {
	t.Helper()
	out, err := exec.Command(ffmpeg, "-nostdin", "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-t", fmt.Sprintf("%.3f", videoS), "-i", "testsrc2=s=320x240:r=30",
		"-f", "lavfi", "-t", fmt.Sprintf("%.3f", audioS), "-i", "sine=f=440:r=48000",
		"-map", "0:v", "-map", "1:a",
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-ar", "48000", "-ac", "2", dst).CombinedOutput()
	if err != nil {
		t.Fatalf("no se pudo fabricar el clip desfasado: %v\n%s", err, out)
	}
	return dst
}

func TestDesfaseAV(t *testing.T) {
	casos := []struct {
		video, audio int64
		quiere       time.Duration
	}{
		{60_000, 60_000, 0},
		{60_000, 57_500, 2500 * time.Millisecond},
		{57_500, 60_000, 2500 * time.Millisecond},
		{60_000, 0, 0}, // sin medida de audio no se afirma nada
		{0, 60_000, 0},
	}
	for _, c := range casos {
		if got := DesfaseAV(c.video, c.audio); got != c.quiere {
			t.Errorf("DesfaseAV(%d, %d) = %s, se esperaba %s", c.video, c.audio, got, c.quiere)
		}
	}
	if mmss(21*60_000+41_000) != "21:41" || mmss(3_723_000) != "1:02:03" {
		t.Errorf("mmss no escribe como una persona: %q %q", mmss(21*60_000+41_000), mmss(3_723_000))
	}
}

// TestIngestImagenYSonidoDeDistintaDuracionVaACuarentena: un archivo con la
// imagen 8 s más larga que el sonido se para con su código y una frase que
// dice las dos duraciones; uno con un desfase chico entra normal.
func TestIngestImagenYSonidoDeDistintaDuracionVaACuarentena(t *testing.T) {
	ffmpeg, ffprobe := tools(t)
	dir := t.TempDir()
	deps := Deps{FFmpeg: ffmpeg, FFprobe: ffprobe}

	cortado := clipDesfasado(t, ffmpeg, filepath.Join(dir, "Cortado.mp4"), 10, 2)
	asset, _, _, err := Ingest(context.Background(), deps, cortado)
	if err == nil {
		t.Fatal("un archivo con 8 s de desfase entre imagen y sonido tenía que pararse")
	}
	if asset.State != model.AssetQuarantine {
		t.Errorf("estado = %q, se esperaba cuarentena", asset.State)
	}
	if Motivo(err) != MotivoDesfaseAV || asset.MotivoCodigo != MotivoDesfaseAV {
		t.Errorf("código = %q / %q, se esperaba %q", Motivo(err), asset.MotivoCodigo, MotivoDesfaseAV)
	}
	for _, frase := range []string{"distinta duración", "imagen 0:10", "sonido 0:02", "dejar pasar"} {
		if !strings.Contains(asset.PlainReason, frase) {
			t.Errorf("el motivo no dice %q: %q", frase, asset.PlainReason)
		}
	}

	// Un desfase de un segundo es normal en material bien hecho: no se para.
	casi := clipDesfasado(t, ffmpeg, filepath.Join(dir, "Casi.mp4"), 4, 3)
	asset, _, _, err = Ingest(context.Background(), deps, casi)
	if err != nil {
		t.Fatalf("un desfase de 1 s no debía parar el archivo: %v", err)
	}
	if asset.State == model.AssetQuarantine || asset.MotivoCodigo != "" {
		t.Errorf("el archivo con desfase chico quedó %q con código %q", asset.State, asset.MotivoCodigo)
	}
}
