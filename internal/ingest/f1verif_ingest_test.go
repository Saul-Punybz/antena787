package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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

// Pruebas de verificación de los criterios F1-01 a F1-10 de
// docs/ACEPTACION.md que no tenían una prueba propia. No tocan el código de
// producción: solo lo ejercitan.
//
// TestF1_04_CEA608SobreviveAlFormatoDeCasa se salta: F1-04 quedó diferido a
// F2 por decisión de producto (ver docs/ACEPTACION.md). El cuerpo se conserva
// para que la prueba vuelva a correr cuando F2 construya la reinserción.
//
// Las demás describen el comportamiento que hay hoy; no se debilitan.

// ── F1-01 · tamaño estable 10 s Y el archivo se abre sin bloqueo ───────

// TestF1_01_ArchivoConBloqueoNoSeAnuncia cubre la mitad del criterio que la
// prueba existente (TestWatcherEsperaAQueTermineLaCopia) no toca: aunque el
// tamaño lleve más de diez segundos quieto, un archivo que no se puede abrir
// para leer no se anuncia, y el reloj de quietud vuelve a empezar.
func TestF1_01_ArchivoConBloqueoNoSeAnuncia(t *testing.T) {
	dir := t.TempDir()
	reloj := time.Date(2026, 9, 9, 8, 0, 0, 0, time.UTC)
	w, err := NewWatcher(WatcherOptions{Dir: dir, StableFor: 10 * time.Second, Now: func() time.Time { return reloj }})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	destino := filepath.Join(dir, "spot.mp4")
	if err := os.WriteFile(destino, make([]byte, 4096), 0o644); err != nil {
		t.Fatal(err)
	}
	// Alguien lo tiene agarrado: en Windows, abierto sin compartir, como lo
	// tiene el programa que copia; en Unix, sin permiso de lectura
	// (bloqueo_windows_test.go / bloqueo_otros_test.go).
	soltar := bloquear(t, destino)
	t.Cleanup(soltar)

	sinEventos := func(cuando string) {
		t.Helper()
		select {
		case ev := <-w.Events():
			t.Fatalf("%s: se anunció un archivo que no se puede abrir: %v", cuando, ev)
		default:
		}
	}

	if err := w.Poll(ctx); err != nil {
		t.Fatal(err)
	}
	sinEventos("primera pasada")

	reloj = reloj.Add(30 * time.Second) // muchísimo más de 10 s quieto
	if err := w.Poll(ctx); err != nil {
		t.Fatal(err)
	}
	sinEventos("con el archivo bloqueado")

	// Se suelta el bloqueo: el reloj de quietud se contó desde el último
	// intento fallido, así que todavía no toca.
	soltar()
	reloj = reloj.Add(3 * time.Second)
	if err := w.Poll(ctx); err != nil {
		t.Fatal(err)
	}
	sinEventos("tres segundos después de soltarlo")

	reloj = reloj.Add(11 * time.Second)
	if err := w.Poll(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case ev := <-w.Events():
		if ev.Path != destino {
			t.Fatalf("anunció %q y se esperaba %q", ev.Path, destino)
		}
	default:
		t.Fatal("no anunció el archivo después de soltarse el bloqueo")
	}
}

// ── F1-03 · dos pasadas de loudnorm, de −18 LUFS a −24 LKFS ───────────

// clipConVolumen fabrica un clip con el volumen que se pida: sine=f=440 más
// la ganancia en dB. Con volume=3.8dB la medición de loudnorm da −18.0 LUFS,
// que es justo el caso del criterio.
func clipConVolumen(t *testing.T, ffmpeg, dst, gananciaDB string, segundos float64) string {
	t.Helper()
	dur := fmt.Sprintf("%.3f", segundos)
	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-t", dur, "-i", "testsrc2=s=320x240:r=30",
		"-f", "lavfi", "-t", dur, "-i", "sine=f=440:r=48000",
		"-af", "volume=" + gananciaDB,
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", "-g", "30",
		"-c:a", "aac", "-ar", "48000", "-ac", "2", dst}
	if out, err := exec.Command(ffmpeg, args...).CombinedOutput(); err != nil {
		t.Fatalf("no se pudo fabricar el clip de volumen: %v\n%s", err, out)
	}
	return dst
}

func TestF1_03_DeMenos18AMenos24EnDosPasadas(t *testing.T) {
	ffmpeg, _ := tools(t)
	dir := t.TempDir()
	src := clipConVolumen(t, ffmpeg, filepath.Join(dir, "menos18.mp4"), "3.8dB", 6)
	dst := filepath.Join(dir, "casa.mp4")

	rep, err := Normalize(context.Background(), ffmpeg, src, dst, engine.CAtv, -24, -2,
		NormalizeOptions{SourceDurationMs: 6000, Preset: "ultrafast"})
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	// La primera pasada midió de verdad, y midió lo que el criterio dice.
	if rep.MeasuredLUFS < -20 || rep.MeasuredLUFS > -16 {
		t.Fatalf("la primera pasada midió %.2f LUFS; el clip de prueba tenía que estar en −18", rep.MeasuredLUFS)
	}
	// Y hay constancia de que fueron dos, no una.
	if rep.Passes != 2 {
		t.Errorf("pasadas = %d, el PRD §14.1 exige dos (medir y corregir)", rep.Passes)
	}
	if rep.TargetLUFS != -24 || rep.TargetTruePeak != -2 {
		t.Errorf("objetivo del perfil = %.1f LUFS / %.1f dBTP", rep.TargetLUFS, rep.TargetTruePeak)
	}
	if !rep.Verified {
		t.Fatal("no se midió el resultado: sin eso no hay tolerancia que comprobar")
	}
	if rep.OutputLUFS < -25 || rep.OutputLUFS > -23 {
		t.Errorf("el normalizado mide %.2f LUFS, el perfil pide −24 ±1", rep.OutputLUFS)
	}
	if rep.OutputTruePeak > -2 {
		t.Errorf("true peak = %.2f dBTP, el perfil pide −2 o menos", rep.OutputTruePeak)
	}
}

// ── F1-04 · los subtítulos tienen que salir en el archivo normalizado ──

// clipConSubtitulos608 fabrica un .mov con una pista de subtítulos CEA-608 de
// verdad (c608, codec eia_608 para ffprobe). No es exactamente lo que dice
// F1-04 —608 dentro de la imagen— porque ffmpeg no sabe inyectar SEI A/53 por
// línea de comando; es lo más cercano que se puede montar sin herramientas de
// fuera, y el camino del código es el mismo: Measure.CaptionFormat = cea-608.
func clipConSubtitulos608(t *testing.T, ffmpeg, dir string) string {
	t.Helper()
	scc := filepath.Join(dir, "cc.scc")
	body := "Scenarist_SCC V1.0\n\n" +
		"00:00:01:00\t9425 9425 94ad 94ad 9470 9470 c8ef ec61 2080 8080 8080 942f 942f\n\n" +
		"00:00:03:00\t9425 9425 94ad 94ad 9470 9470 c1ee f4e5 6e61 2038 3837 942f 942f\n\n"
	if err := os.WriteFile(scc, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "con608.mov")
	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-t", "5", "-i", "testsrc2=s=320x240:r=30",
		"-f", "lavfi", "-t", "5", "-i", "sine=f=440:r=48000",
		"-i", scc,
		"-map", "0:v", "-map", "1:a", "-map", "2:s",
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-ar", "48000", "-ac", "2", "-c:s", "copy", dst}
	if out, err := exec.Command(ffmpeg, args...).CombinedOutput(); err != nil {
		t.Skipf("este ffmpeg no sabe muxear CEA-608 en un .mov: %v\n%s", err, out)
	}
	return dst
}

// tieneSubtitulos pregunta a ffprobe si el archivo trae alguna pista de
// subtítulos.
func tieneSubtitulos(t *testing.T, ffprobe, path string) bool {
	t.Helper()
	out, err := exec.Command(ffprobe, "-v", "error", "-print_format", "json",
		"-show_streams", path).Output()
	if err != nil {
		t.Fatalf("ffprobe sobre %q: %v", path, err)
	}
	var raw struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatal(err)
	}
	for _, s := range raw.Streams {
		if s.CodecType == "subtitle" {
			return true
		}
	}
	return false
}

// TestF1_04_CEA608SobreviveAlFormatoDeCasa comprueba lo que exige el
// criterio: el archivo NORMALIZADO tiene subtítulos, no solo el de entrada.
// El contenedor de casa que usa la aplicación es .mkv (internal/app/media.go).
//
// DIFERIDO A F2 por decisión de producto: no existe paso de extracción ni de
// reinserción (el ingest se limita a pasarle -c:s copy a ffmpeg, matroska no
// acepta eia_608, y el reintento de normalize.go vuelve a correr con -sn), y
// además ffprobe 9 dejó de emitir closed_captions, así que el 608 embebido ya
// ni se detecta. El cuerpo de la prueba se queda entero: cuando F2 construya
// el paso, se le quita el Skip y tiene que pasar tal cual está.
func TestF1_04_CEA608SobreviveAlFormatoDeCasa(t *testing.T) {
	t.Skip("F1-04 diferido a F2: ver docs/ACEPTACION.md")
	ffmpeg, ffprobe := tools(t)
	dir := t.TempDir()
	src := clipConSubtitulos608(t, ffmpeg, dir)

	if !tieneSubtitulos(t, ffprobe, src) {
		t.Fatal("el clip de partida se fabricó sin subtítulos: la prueba no sirve")
	}
	m, err := Probe(context.Background(), ffprobe, src)
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if !m.HasCaptions || m.CaptionFormat != "cea-608" {
		t.Fatalf("el ingest no reconoció los subtítulos: tiene=%v formato=%q", m.HasCaptions, m.CaptionFormat)
	}

	asset := model.MediaAsset{DurationMs: m.DurationMs}
	opts := NormalizeOptionsFor(asset, m)
	opts.Preset = "ultrafast"
	opts.SkipVerify = true
	if !opts.EmbeddedCEA608 {
		t.Error("NormalizeOptionsFor no marcó el material como CEA-608")
	}

	dst := filepath.Join(dir, "casa.mkv") // el contenedor real de app.NormalizedDir
	rep, err := Normalize(context.Background(), ffmpeg, src, dst, engine.CAtv, -24, -2, opts)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if !tieneSubtitulos(t, ffprobe, dst) {
		t.Fatalf("F1-04: el archivo normalizado salió SIN subtítulos (nota del ingest: %q)", rep.CaptionsNote)
	}
	if !rep.CaptionsKept {
		t.Error("el reporte no dice que los subtítulos se conservaron")
	}
}

// ── F1-06 · negro en medio = marca candidata, no recorte ───────────────

// TestF1_06_MarcaDeCorteCandidataQuedaEnElAsset corre el ingest entero, no
// solo el analizador, y comprueba que la marca acaba en el campo del modelo
// que después guarda la base (media_asset.marcas_de_corte_ms).
func TestF1_06_MarcaDeCorteCandidataQuedaEnElAsset(t *testing.T) {
	ffmpeg, ffprobe := tools(t)
	dir := t.TempDir()
	src := makeClip(t, ffmpeg, filepath.Join(dir, "stock.mp4"), "30",
		segment{seconds: 2},
		segment{seconds: 2, black: true, silent: true},
		segment{seconds: 2})

	deps := Deps{
		FFmpeg: ffmpeg, FFprobe: ffprobe,
		Format: engine.CAtv, TargetLUFS: -24, TruePeak: -2,
		ThumbnailDir: filepath.Join(dir, "miniaturas"),
		Retry:        RetryPolicy{Delay: 0, Sleep: SleepCtx},
	}
	asset, _, _, err := Ingest(context.Background(), deps, src)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if asset.HeadBlackMs != 0 || asset.TailBlackMs != 0 {
		t.Errorf("no se recorta nada: cabeza=%d cola=%d", asset.HeadBlackMs, asset.TailBlackMs)
	}
	if len(asset.BreakMarksMs) != 1 {
		t.Fatalf("marcas_de_corte_ms = %v, se esperaba exactamente una", asset.BreakMarksMs)
	}
	if asset.BreakMarksMs[0] < 2200 || asset.BreakMarksMs[0] > 3800 {
		t.Errorf("la marca cayó en %d ms, se esperaba dentro del negro (2000-4000)", asset.BreakMarksMs[0])
	}
	// Y la marca no se aplica sola: las opciones de normalización no la usan.
	m, err := Probe(context.Background(), ffprobe, src)
	if err != nil {
		t.Fatal(err)
	}
	opts := NormalizeOptionsFor(asset, m)
	if opts.TrimHeadMs != 0 || opts.TrimTailMs != 0 {
		t.Errorf("la marca del medio no puede convertirse en recorte: %+v", opts)
	}
}

// ── F1-07 · formato de casa, GOP cerrado, una sola vez ────────────────

func TestF1_07_FormatoDeCasaConGopCerrado(t *testing.T) {
	ffmpeg, ffprobe := tools(t)
	dir := t.TempDir()
	src := makeClip(t, ffmpeg, filepath.Join(dir, "origen.mp4"), "25", segment{seconds: 4})
	dst := filepath.Join(dir, "casa.mkv")

	f := engine.CAtv // 1280x720, 60000/1001, 48 kHz, estéreo
	rep, err := Normalize(context.Background(), ffmpeg, src, dst, f, -24, -2,
		NormalizeOptions{SourceDurationMs: 4000, Preset: "ultrafast", SkipVerify: true})
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if rep.OutputPath != dst {
		t.Errorf("ruta de salida = %q", rep.OutputPath)
	}

	m, err := Probe(context.Background(), ffprobe, dst)
	if err != nil {
		t.Fatalf("Probe del normalizado: %v", err)
	}
	if m.Codec != "h264" {
		t.Errorf("códec = %q, se esperaba uno solo (h264)", m.Codec)
	}
	if m.Resolution != "1280x720" {
		t.Errorf("resolución = %q, se esperaba 1280x720", m.Resolution)
	}
	// Matroska guarda los tiempos en milisegundos, así que ffprobe devuelve
	// la tasa como 19001/317 en vez de 60000/1001: se compara el número.
	if m.FPSFloat < 59.9 || m.FPSFloat > 60.0 {
		t.Errorf("fps = %q (%.4f), se esperaba 59.94", m.FPS, m.FPSFloat)
	}
	if m.AudioChannels != 2 || m.SampleRate != 48000 {
		t.Errorf("audio = %d canales a %d Hz", m.AudioChannels, m.SampleRate)
	}

	// GOP cerrado: la bandera va en la línea de ffmpeg…
	args := normalizeArgs(src, dst, f, NormalizeOptions{SourceDurationMs: 4000}, rep, false)
	linea := strings.Join(args, " ")
	for _, quiero := range []string{"-flags +cgop", "-g 60", "-keyint_min 60", "-sc_threshold 0"} {
		if !strings.Contains(linea, quiero) {
			t.Errorf("falta %q en la línea de conformado: %s", quiero, linea)
		}
	}
	// …y el resultado tiene un cuadro clave cada 60, que son los 60000/1001.
	claves := cuadrosClave(t, ffprobe, dst)
	if len(claves) < 3 {
		t.Fatalf("cuadros clave = %v, se esperaba uno por segundo", claves)
	}
	if claves[0] != 0 {
		t.Errorf("el primer cuadro clave está en %d, se esperaba en 0", claves[0])
	}
	for i := 1; i < len(claves); i++ {
		if d := claves[i] - claves[i-1]; d != 60 {
			t.Errorf("entre cuadros clave hay %d cuadros, se esperaban 60 (GOP de un segundo): %v", d, claves)
			break
		}
	}
}

// cuadrosClave devuelve el índice de cada cuadro clave del video.
func cuadrosClave(t *testing.T, ffprobe, path string) []int {
	t.Helper()
	out, err := exec.Command(ffprobe, "-v", "error", "-select_streams", "v:0",
		"-show_entries", "frame=key_frame", "-of", "csv=p=0", path).Output()
	if err != nil {
		t.Fatalf("ffprobe -show_entries frame: %v", err)
	}
	// Una línea por cuadro: «1» o «0», a veces con coma al final y, en
	// Windows, con retorno de carro. Cualquier otra línea (datos de al lado
	// que algún ffprobe intercala) no es un cuadro y no cuenta.
	var claves []int
	cuadro := 0
	for _, l := range strings.Split(string(out), "\n") {
		l = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(l), ","))
		switch l {
		case "1":
			claves = append(claves, cuadro)
			cuadro++
		case "0":
			cuadro++
		}
	}
	if len(claves) == 0 || claves[0] != 0 {
		t.Logf("salida cruda de ffprobe (primeras líneas): %q", primeras(string(out), 5))
	}
	return claves
}

func primeras(s string, n int) []string {
	ls := strings.Split(s, "\n")
	if len(ls) > n {
		ls = ls[:n]
	}
	return ls
}

// TestF1_07_SeNormalizaUnaSolaVez: la cola normaliza cada archivo una vez y
// lo deja en listo. No se vuelve a llamar en cada reproducción.
func TestF1_07_SeNormalizaUnaSolaVez(t *testing.T) {
	q := NewQueue()
	q.RetryDelay = 0
	q.Enqueue(11, time.Now())

	p := &persistenciaDePrueba{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	llamadas := 0
	go func() {
		_ = q.Run(ctx, p, func(ctx context.Context, j Job) (string, error) {
			mu.Lock()
			llamadas++
			mu.Unlock()
			return "/datos/normalizado/casa.mkv", nil
		})
	}()
	esperaHasta(t, time.Second, func() bool { return p.last(11).state == NormalizeReady })
	time.Sleep(50 * time.Millisecond) // por si alguien lo reencolara
	cancel()

	mu.Lock()
	defer mu.Unlock()
	if llamadas != 1 {
		t.Errorf("se normalizó %d veces, el PRD §9 paso 1.5 dice una sola", llamadas)
	}
	if got := p.last(11); got.state != NormalizeReady || got.path != "/datos/normalizado/casa.mkv" {
		t.Errorf("estado final = %+v, se esperaba listo con su ruta normalizada", got)
	}
	if q.Len() != 0 {
		t.Errorf("quedaron %d trabajos en la cola después de terminar bien", q.Len())
	}
}

// ── F1-08 · etiquetas embebidas, sin red ──────────────────────────────

// clipConEtiquetas fabrica un clip con título y año embebidos y nada más: ni
// .nfo, ni carátula al lado, ni carátula dentro.
func clipConEtiquetas(t *testing.T, ffmpeg, dst, titulo, anio string) string {
	t.Helper()
	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-t", "3", "-i", "testsrc2=s=320x240:r=30",
		"-f", "lavfi", "-t", "3", "-i", "sine=f=440:r=48000",
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-ar", "48000", "-ac", "2",
		"-metadata", "title=" + titulo,
		"-metadata", "date=" + anio,
		dst}
	if out, err := exec.Command(ffmpeg, args...).CombinedOutput(); err != nil {
		t.Fatalf("no se pudo fabricar el clip con etiquetas: %v\n%s", err, out)
	}
	return dst
}

func TestF1_08_EtiquetasEmbebidasSinRed(t *testing.T) {
	ffmpeg, ffprobe := tools(t)
	dir := t.TempDir()
	src := clipConEtiquetas(t, ffmpeg, filepath.Join(dir, "sinficha.mp4"), "El Cartero", "1994")

	m, err := Probe(context.Background(), ffprobe, src)
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if m.Tags.Title != "El Cartero" || m.Tags.Year != 1994 {
		t.Fatalf("las etiquetas embebidas no se leyeron: %+v", m.Tags)
	}

	// La configuración de fábrica del ingest —la que arma internal/app— no
	// enciende ningún driver de red (PRD §10). Un servidor que no debería
	// recibir nada vigila que así sea.
	var tocado int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tocado++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	deps := Deps{
		FFmpeg: ffmpeg, FFprobe: ffprobe,
		Format: engine.CAtv, TargetLUFS: -24, TruePeak: -2,
		ThumbnailDir: filepath.Join(dir, "miniaturas"),
		ArtworkDir:   filepath.Join(dir, "caratulas"),
		Retry:        RetryPolicy{Delay: 0, Sleep: SleepCtx},
		// Providers: vacío a propósito. Es el default de app.ingestDeps.
	}
	asset, title, _, err := Ingest(context.Background(), deps, src)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if asset.State != model.AssetReady {
		t.Fatalf("estado = %q", asset.State)
	}
	if title.Name != "El Cartero" {
		t.Errorf("nombre = %q, se esperaba el de la etiqueta", title.Name)
	}
	if title.Year == nil || *title.Year != 1994 {
		t.Errorf("año = %v, se esperaba 1994", title.Year)
	}
	if !strings.Contains(title.MetadataSource, "tags-embebidas") {
		t.Errorf("fuente_ficha = %q, se esperaba tags-embebidas", title.MetadataSource)
	}
	if tocado != 0 {
		t.Errorf("el ingest hizo %d llamadas de red y no debía hacer ninguna", tocado)
	}
}

// ── F1-09 · TVmaze antes que cualquier driver con clave ───────────────

// proveedorConNombre registra en qué orden lo consultaron.
type proveedorConNombre struct {
	nombre string
	orden  *[]string
	card   *Card
}

func (p *proveedorConNombre) Lookup(ctx context.Context, q Query) (*Card, error) {
	*p.orden = append(*p.orden, p.nombre)
	return p.card, nil
}

func TestF1_09_TVmazeAntesQueDriverConClave(t *testing.T) {
	// 1 · el orden en que se arma la lista de drivers.
	lista := Providers(ProviderConfig{TVmaze: true, CoverArtArchive: true, TMDBAPIKey: "clave-de-prueba"})
	if len(lista) != 3 {
		t.Fatalf("drivers = %d, se esperaban 3", len(lista))
	}
	if _, ok := lista[0].(*TVmaze); !ok {
		t.Fatalf("el primer driver es %T; F1-09 pide TVmaze primero", lista[0])
	}
	for i, p := range lista {
		if _, ok := p.(*TMDB); ok && i == 0 {
			t.Fatal("TMDB, que pide clave, quedó antes que los que no la piden")
		}
	}
	if _, ok := lista[2].(*TMDB); !ok {
		t.Errorf("el último driver es %T; se esperaba TMDB (el que pide clave)", lista[2])
	}

	// 2 · y que Metadata los consulte en ese orden y se pare cuando TVmaze ya
	// contestó, sin llegar al que pide clave.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/singlesearch/shows") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"Kojak","premiered":"1973-10-24",
			"summary":"<p>Un teniente de policía.</p>","genres":["Crime"],
			"image":{"original":"http://ejemplo/kojak.jpg"}}`))
	}))
	defer srv.Close()

	var orden []string
	tvmaze := &TVmaze{Client: srv.Client(), BaseURL: srv.URL}
	espia := &proveedorConNombre{nombre: "driver-con-clave", orden: &orden}

	dir := t.TempDir()
	video := filepath.Join(dir, "Kojak.mkv")
	if err := os.WriteFile(video, []byte("no hace falta que sea un video"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Sin etiquetas, sin .nfo y sin carátula: lo único que el ingest sabe es
	// que dura media hora, que es lo que hace que GuessKind lo tome por
	// televisión (programa) y no por película.
	m := Measure{HasVideo: true, DurationMs: 30 * 60_000}
	card, err := Metadata(context.Background(), video, m, MetadataDeps{
		Providers: []Provider{tvmaze, espia},
	})
	if err != nil {
		t.Fatalf("Metadata: %v", err)
	}
	if card.Synopsis == "" || card.ArtworkURL == "" {
		t.Fatalf("TVmaze no llenó la ficha: %+v", card)
	}
	if !containsStr(card.Sources, "tvmaze") {
		t.Errorf("fuentes = %v, se esperaba tvmaze", card.Sources)
	}
	if len(orden) != 0 {
		t.Errorf("se consultó %v teniendo ya la respuesta de TVmaze", orden)
	}
}

func containsStr(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// ── F1-10 · sin audio o con error de lectura → cuarentena ─────────────

// TestF1_10_ErrorDeLecturaVaACuarentena es la mitad del criterio que sí se
// cumple: ffprobe no puede leer el archivo → cuarentena con su motivo.
func TestF1_10_ErrorDeLecturaVaACuarentena(t *testing.T) {
	ffmpeg, ffprobe := tools(t)
	dir := t.TempDir()
	roto := filepath.Join(dir, "medio-copiado.mp4")
	if err := os.WriteFile(roto, []byte("ftyp roto a la mitad de la copia"), 0o644); err != nil {
		t.Fatal(err)
	}
	deps := Deps{FFmpeg: ffmpeg, FFprobe: ffprobe, Retry: RetryPolicy{Delay: 0, Sleep: SleepCtx}}
	asset, _, _, err := Ingest(context.Background(), deps, roto)
	if err == nil {
		t.Fatal("un archivo que ffprobe no puede leer tiene que fallar")
	}
	if asset.State != model.AssetQuarantine {
		t.Fatalf("estado = %q, se esperaba cuarentena", asset.State)
	}
	if asset.PlainReason == "" {
		t.Error("falta motivo_en_cristiano")
	}
	for _, jerga := range []string{"ffprobe", "ffmpeg", "codec", "moov", "atom"} {
		if strings.Contains(strings.ToLower(asset.PlainReason), jerga) {
			t.Errorf("el motivo lleva jerga (%q): %q", jerga, asset.PlainReason)
		}
	}
	// Y no queda disponible para programarse: Ready() es lo que mira el resolver.
	if asset.Ready() {
		t.Error("un archivo en cuarentena no puede dar Ready() == true")
	}
}

// TestF1_10_SinAudioVaACuarentena es la otra mitad: un archivo con imagen y
// sin pista de sonido no se da por listo solo. Queda en cuarentena con su
// motivo en cristiano.
//
// Desde la decisión del 9 de septiembre de 2026 (F1-58 y F1-59) la cuarentena
// es además definitiva mientras no aparezca el sonido: no hay «dejarlo pasar»
// que valga, porque todo lo que sale al aire lleva audio. Lo que sí lo saca
// de ahí es poner un archivo de sonido con el mismo nombre al lado.
func TestF1_10_SinAudioVaACuarentena(t *testing.T) {
	ffmpeg, ffprobe := tools(t)
	dir := t.TempDir()
	mudo := filepath.Join(dir, "mudo.mp4")
	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-t", "3", "-i", "testsrc2=s=320x240:r=30",
		"-an", "-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", mudo}
	if out, err := exec.Command(ffmpeg, args...).CombinedOutput(); err != nil {
		t.Fatalf("no se pudo fabricar el clip mudo: %v\n%s", err, out)
	}

	m, err := Probe(context.Background(), ffprobe, mudo)
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if m.HasAudio {
		t.Fatal("el clip de prueba se fabricó con audio: la prueba no sirve")
	}

	deps := Deps{
		FFmpeg: ffmpeg, FFprobe: ffprobe,
		Format: engine.CAtv, TargetLUFS: -24, TruePeak: -2,
		ThumbnailDir: filepath.Join(dir, "miniaturas"),
		Retry:        RetryPolicy{Delay: 0, Sleep: SleepCtx},
	}
	asset, _, _, err := Ingest(context.Background(), deps, mudo)
	if asset.State != model.AssetQuarantine {
		t.Fatalf("F1-10: un archivo sin audio detectable quedó en %q (err=%v); se esperaba cuarentena",
			asset.State, err)
	}
	if asset.PlainReason == "" {
		t.Error("falta motivo_en_cristiano explicando que no tiene sonido")
	}
}
