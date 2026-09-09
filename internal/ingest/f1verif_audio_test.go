package ingest

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"antena787/internal/engine"
	"antena787/internal/model"
)

// Pruebas de verificación de los criterios F1-58 a F1-63 de
// docs/ACEPTACION.md ("Audio de todo el material", decisión del 9 de
// septiembre de 2026): todo lo que sale al aire lleva sonido.

// ── piezas de prueba ──────────────────────────────────────────────────

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
func audioSuelto(t *testing.T, ffmpeg, dst string, segundos float64, hz int) string {
	t.Helper()
	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-t", fmt.Sprintf("%.3f", segundos),
		"-i", fmt.Sprintf("sine=f=%d:r=48000", hz),
		"-ac", "2", dst}
	if out, err := exec.Command(ffmpeg, args...).CombinedOutput(); err != nil {
		t.Fatalf("no se pudo fabricar el sonido de al lado: %v\n%s", err, out)
	}
	return dst
}

// clipDosPistas fabrica un video con dos pistas de sonido etiquetadas por
// idioma y con tonos bien distintos: la inglesa es grave (300 Hz) y la
// española aguda (3000 Hz). Así se puede oír cuál de las dos acabó en la
// copia de casa sin creerle a nadie de palabra.
func clipDosPistas(t *testing.T, ffmpeg, dst string) string {
	t.Helper()
	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-t", "4", "-i", "testsrc2=s=320x240:r=30",
		"-f", "lavfi", "-t", "4", "-i", "sine=f=300:r=48000",
		"-f", "lavfi", "-t", "4", "-i", "sine=f=3000:r=48000",
		"-map", "0:v", "-map", "1:a", "-map", "2:a",
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", "-g", "30",
		"-c:a", "aac", "-ar", "48000", "-ac", "2",
		"-metadata:s:a:0", "language=eng", "-metadata:s:a:0", "title=Original en inglés",
		"-metadata:s:a:1", "language=spa", "-metadata:s:a:1", "title=Doblaje en español",
		dst}
	if out, err := exec.Command(ffmpeg, args...).CombinedOutput(); err != nil {
		t.Fatalf("no se pudo fabricar el clip de dos pistas: %v\n%s", err, out)
	}
	return dst
}

// volumenConFiltro devuelve el volumen medio del archivo después de pasarlo
// por un filtro. Con un pasa-altos y un pasa-bajos se distingue un tono grave
// de uno agudo sin más herramientas que ffmpeg.
func volumenConFiltro(t *testing.T, ffmpeg, path, filtro string) float64 {
	t.Helper()
	cmd := exec.Command(ffmpeg, "-nostdin", "-hide_banner", "-i", path,
		"-af", filtro+",volumedetect", "-f", "null", "-")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("no se pudo medir el volumen de %q: %v\n%s", path, err, out)
	}
	for _, l := range strings.Split(string(out), "\n") {
		i := strings.Index(l, "mean_volume:")
		if i < 0 {
			continue
		}
		campos := strings.Fields(l[i+len("mean_volume:"):])
		if len(campos) == 0 {
			continue
		}
		v, err := strconv.ParseFloat(campos[0], 64)
		if err != nil {
			t.Fatalf("no se entendió el volumen medio %q", l)
		}
		return v
	}
	t.Fatalf("ffmpeg no dijo el volumen medio de %q:\n%s", path, out)
	return 0
}

// depsDePrueba es la configuración del ingest que usan estas pruebas: sin
// esperas y con los archivos de al lado dados por copiados en cuanto están.
func depsDePrueba(ffmpeg, ffprobe, dir string) Deps {
	return Deps{
		FFmpeg: ffmpeg, FFprobe: ffprobe,
		Format: engine.CAtv, TargetLUFS: -24, TruePeak: -2,
		ThumbnailDir:     filepath.Join(dir, "miniaturas"),
		Retry:            RetryPolicy{Delay: 0, Sleep: SleepCtx},
		SidecarStableFor: time.Nanosecond,
	}
}

// ── F1-58 · el sonido viene al lado ───────────────────────────────────

func TestF1_58_AudioAlLadoEntraEnLaCopiaDeCasa(t *testing.T) {
	ffmpeg, ffprobe := tools(t)
	dir := t.TempDir()
	video := clipMudo(t, ffmpeg, filepath.Join(dir, "Fiesta en la Placita.mp4"), 4)
	audio := audioSuelto(t, ffmpeg, filepath.Join(dir, "Fiesta en la Placita.wav"), 4, 440)

	deps := depsDePrueba(ffmpeg, ffprobe, dir)
	p := &persistenciaDePrueba{}
	deps.Persist = p

	asset, _, _, err := Ingest(context.Background(), deps, video)
	if err != nil {
		t.Fatalf("con el sonido al lado el archivo no va a cuarentena: %v", err)
	}
	if asset.State != model.AssetReady {
		t.Fatalf("estado = %q, se esperaba listo", asset.State)
	}
	if asset.AudioChannels != 2 {
		t.Errorf("canales_audio = %d, se esperaban los 2 del archivo de al lado", asset.AudioChannels)
	}
	if got := p.sonido(asset.ID); got.audio != audio {
		t.Errorf("audio_sidecar guardado = %q, se esperaba %q", got.audio, audio)
	}

	// Y la copia de casa sale con sonido, medido en dos pasadas.
	m, err := Probe(context.Background(), ffprobe, video)
	if err != nil {
		t.Fatal(err)
	}
	opts := NormalizeOptionsFor(asset, m)
	if opts.AudioSidecar != audio {
		t.Fatalf("las opciones de normalización no llevan el sonido de al lado: %q", opts.AudioSidecar)
	}
	opts.Preset = "ultrafast"
	dst := filepath.Join(dir, "casa.mkv")
	rep, err := Normalize(context.Background(), ffmpeg, video, dst, engine.CAtv, -24, -2, opts)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if rep.Passes != 2 {
		t.Errorf("pasadas = %d, se esperaban las dos de siempre", rep.Passes)
	}
	if rep.MeasuredLUFS == 0 {
		t.Error("no quedó registro de la primera pasada: el sonido de al lado también se mide")
	}
	if rep.OutputLUFS < -25 || rep.OutputLUFS > -23 {
		t.Errorf("la copia de casa mide %.2f, el perfil pide −24 ±1", rep.OutputLUFS)
	}
	casa, err := Probe(context.Background(), ffprobe, dst)
	if err != nil {
		t.Fatalf("Probe de la copia de casa: %v", err)
	}
	if !casa.HasAudio || casa.AudioChannels != 2 {
		t.Fatalf("la copia de casa salió sin sonido: %+v", casa.AudioTracks)
	}
	if casa.DurationMs < 3800 || casa.DurationMs > 4300 {
		t.Errorf("la copia dura %d ms, se esperaban ~4000", casa.DurationMs)
	}
}

// TestF1_58_ElAudioLlegaDespues es la segunda mitad del criterio: el video ya
// está en cuarentena por mudo y el sonido aparece más tarde. La carpeta
// vigilada lo anuncia como archivo de al lado —no como material— y el ingest
// del video vuelve a correr y lo saca de cuarentena.
func TestF1_58_ElAudioLlegaDespues(t *testing.T) {
	ffmpeg, ffprobe := tools(t)
	dir := t.TempDir()
	video := clipMudo(t, ffmpeg, filepath.Join(dir, "spot.mp4"), 3)

	deps := depsDePrueba(ffmpeg, ffprobe, dir)
	p := &persistenciaDePrueba{}
	deps.Persist = p

	asset, _, _, err := Ingest(context.Background(), deps, video)
	if err == nil || asset.State != model.AssetQuarantine {
		t.Fatalf("sin sonido y sin archivo al lado tiene que ir a cuarentena: estado=%q err=%v", asset.State, err)
	}
	if !EsSinAudio(err) {
		t.Fatalf("el motivo no lleva el código de «sin sonido»: %q", Motivo(err))
	}
	idAsset := asset.ID
	if idAsset == 0 {
		t.Fatal("el archivo en cuarentena tenía que quedar guardado con su ID")
	}

	// Llega el sonido. La carpeta vigilada lo ve y avisa de que acompaña al
	// video, no de que sea material nuevo.
	reloj := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	w, err := NewWatcher(WatcherOptions{Dir: dir, StableFor: 10 * time.Second, Now: func() time.Time { return reloj }})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := w.Poll(ctx); err != nil { // primera vuelta: solo se apunta lo que hay
		t.Fatal(err)
	}
	audio := audioSuelto(t, ffmpeg, filepath.Join(dir, "spot.wav"), 3, 440)
	if err := w.Poll(ctx); err != nil {
		t.Fatal(err)
	}
	reloj = reloj.Add(30 * time.Second)
	if err := w.Poll(ctx); err != nil {
		t.Fatal(err)
	}

	var aviso Found
	for hay := true; hay; {
		select {
		case ev := <-w.Events():
			if ev.Path == audio {
				aviso = ev
			}
		default:
			hay = false
		}
	}
	if aviso.Path == "" {
		t.Fatal("la carpeta vigilada no avisó del archivo de sonido que apareció")
	}
	if aviso.Kind != EventSidecar {
		t.Errorf("aviso = %q, se esperaba %q: un .wav al lado de un video no es material aparte", aviso.Kind, EventSidecar)
	}
	if aviso.For != video {
		t.Errorf("el aviso dice que acompaña a %q, se esperaba %q", aviso.For, video)
	}

	// Y volver a procesar el video lo saca de cuarentena.
	asset2, _, _, err := ReingestSidecar(ctx, deps, aviso.Path)
	if err != nil {
		t.Fatalf("volver a procesar el video con su sonido: %v", err)
	}
	if asset2.State != model.AssetReady {
		t.Fatalf("estado = %q, se esperaba listo", asset2.State)
	}
	if asset2.Path != video {
		t.Errorf("se volvió a procesar %q y se esperaba el video %q", asset2.Path, video)
	}
	guardado, ok := p.guardado(idAsset)
	if !ok {
		t.Fatal("el archivo se duplicó en vez de actualizarse")
	}
	if guardado.State != model.AssetReady {
		t.Errorf("en la base quedó %q: la cuarentena tenía que levantarse", guardado.State)
	}
	if got := p.sonido(idAsset); got.audio != audio {
		t.Errorf("audio_sidecar guardado = %q, se esperaba %q", got.audio, audio)
	}
}

// ── F1-59 · nada sale mudo ────────────────────────────────────────────

func TestF1_59_SinSonidoYSinAudioAlLadoNoHaySalida(t *testing.T) {
	ffmpeg, ffprobe := tools(t)
	dir := t.TempDir()
	video := clipMudo(t, ffmpeg, filepath.Join(dir, "promo.mp4"), 3)

	asset, _, _, err := Ingest(context.Background(), depsDePrueba(ffmpeg, ffprobe, dir), video)
	if err == nil {
		t.Fatal("un archivo mudo y sin sonido al lado no puede darse por bueno")
	}
	if asset.State != model.AssetQuarantine {
		t.Fatalf("estado = %q, se esperaba cuarentena", asset.State)
	}
	esperado := "«promo.mp4» no trae sonido: pon a su lado un archivo de audio con el mismo nombre " +
		"(.wav, .m4a, .aac, .mp3 o .flac) y lo vuelvo a procesar"
	if asset.PlainReason != esperado {
		t.Errorf("motivo_en_cristiano = %q\nse esperaba            %q", asset.PlainReason, esperado)
	}
	// El código es lo que mira la aplicación para NO ofrecer el botón de
	// "dejarlo pasar": no hay camino para soltarlo mudo.
	if Motivo(err) != MotivoSinAudio {
		t.Errorf("código de motivo = %q, se esperaba %q", Motivo(err), MotivoSinAudio)
	}
	if !EsSinAudio(err) {
		t.Error("EsSinAudio no reconoció el motivo")
	}
	if !TextoSinAudio(asset.PlainReason) {
		t.Error("TextoSinAudio no reconoce el motivo ya guardado en la base")
	}
	// Y el motivo habla como una persona.
	for _, jerga := range []string{"driver", "códec", "codec", "GOP", "LKFS", "transport stream", "ffmpeg", "mux"} {
		if strings.Contains(strings.ToLower(asset.PlainReason), strings.ToLower(jerga)) {
			t.Errorf("el motivo lleva jerga (%q): %q", jerga, asset.PlainReason)
		}
	}
	if asset.Ready() {
		t.Error("un archivo sin sonido no puede quedar disponible para programarse")
	}
}

// ── F1-60 · varias pistas: manda el idioma del canal ──────────────────

func TestF1_60_EligeLaPistaEnElIdiomaDelCanal(t *testing.T) {
	ffmpeg, ffprobe := tools(t)
	dir := t.TempDir()
	src := clipDosPistas(t, ffmpeg, filepath.Join(dir, "pelicula.mkv"))

	m, err := Probe(context.Background(), ffprobe, src)
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if len(m.AudioTracks) != 2 {
		t.Fatalf("pistas_audio = %+v, se esperaban dos", m.AudioTracks)
	}
	if m.AudioTracks[0].Index != 0 || m.AudioTracks[0].Language != "en" {
		t.Errorf("la primera pista = %+v, se esperaba la inglesa", m.AudioTracks[0])
	}
	if m.AudioTracks[1].Index != 1 || m.AudioTracks[1].Language != "es" {
		t.Errorf("la segunda pista = %+v, se esperaba la española", m.AudioTracks[1])
	}
	if m.AudioTracks[1].Channels != 2 || m.AudioTracks[1].Title == "" {
		t.Errorf("faltan canales o título en la pista: %+v", m.AudioTracks[1])
	}

	// El canal quiere el aire en español: gana la segunda.
	prefs := Preferencias{IdiomaAudio: "es"}
	if got := PistaDeAire(m.AudioTracks, prefs); got != 1 {
		t.Fatalf("pista elegida = %d, se esperaba la española (1)", got)
	}
	// Sin decir nada también, porque «es» es el valor de fábrica.
	if got := PistaDeAire(m.AudioTracks, Preferencias{}); got != 1 {
		t.Errorf("sin idioma puesto se eligió la pista %d; el valor de fábrica es español", got)
	}
	// Si ninguna está en el idioma del canal, la primera del archivo.
	if got := PistaDeAire(m.AudioTracks, Preferencias{IdiomaAudio: "fr"}); got != 0 {
		t.Errorf("sin pista en francés se eligió la %d, se esperaba la primera", got)
	}
	// Y lo que elige una persona manda sobre todo lo demás.
	cero := 0
	if got := PistaDeAire(m.AudioTracks, Preferencias{IdiomaAudio: "es", PistaAudio: &cero}); got != 0 {
		t.Errorf("la elección del operador no mandó: salió la pista %d", got)
	}

	// El ingest la guarda y la copia de casa sale con ESA pista.
	deps := depsDePrueba(ffmpeg, ffprobe, dir)
	deps.Preferencias = prefs
	p := &persistenciaDePrueba{}
	deps.Persist = p
	asset, _, _, err := Ingest(context.Background(), deps, src)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	reg := p.sonido(asset.ID)
	if len(reg.pistas) != 2 || reg.pistaAire != 1 {
		t.Fatalf("lo guardado = pistas %+v, al aire %d", reg.pistas, reg.pistaAire)
	}

	opts := NormalizeOptionsFor(asset, m, prefs)
	if opts.AudioTrack != 1 {
		t.Fatalf("las opciones de normalización llevan la pista %d", opts.AudioTrack)
	}
	opts.Preset = "ultrafast"
	opts.SkipVerify = true
	dst := filepath.Join(dir, "casa.mkv")
	if _, err := Normalize(context.Background(), ffmpeg, src, dst, engine.CAtv, -24, -2, opts); err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	casa, err := Probe(context.Background(), ffprobe, dst)
	if err != nil {
		t.Fatal(err)
	}
	if len(casa.AudioTracks) != 1 {
		t.Errorf("la copia de casa tiene %d pistas, se esperaba una sola", len(casa.AudioTracks))
	}
	// La pista española era el tono agudo: en la copia de casa tiene que
	// oírse por encima del pasa-altos y no por debajo del pasa-bajos.
	agudo := volumenConFiltro(t, ffmpeg, dst, "highpass=f=800")
	grave := volumenConFiltro(t, ffmpeg, dst, "lowpass=f=800")
	if agudo < grave+10 {
		t.Errorf("la copia de casa no suena a la pista española: agudo %.1f dB, grave %.1f dB", agudo, grave)
	}
}

// ── F1-62 · subtítulos al lado ────────────────────────────────────────

func TestF1_62_SubtitulosAlLadoEntranEnLaCopiaDeCasa(t *testing.T) {
	ffmpeg, ffprobe := tools(t)
	dir := t.TempDir()
	src := makeClip(t, ffmpeg, filepath.Join(dir, "capitulo.mp4"), "30", segment{seconds: 3})
	srt := filepath.Join(dir, "capitulo.srt")
	if err := os.WriteFile(srt, []byte(srtBueno), 0o644); err != nil {
		t.Fatal(err)
	}

	deps := depsDePrueba(ffmpeg, ffprobe, dir)
	p := &persistenciaDePrueba{}
	deps.Persist = p
	asset, _, _, err := Ingest(context.Background(), deps, src)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if got := p.sonido(asset.ID); got.subtitulos != srt {
		t.Errorf("subtitulos_sidecar guardado = %q, se esperaba %q", got.subtitulos, srt)
	}

	m, err := Probe(context.Background(), ffprobe, src)
	if err != nil {
		t.Fatal(err)
	}
	opts := NormalizeOptionsFor(asset, m)
	if opts.SubtitleSidecar != srt {
		t.Fatalf("las opciones de normalización no llevan el .srt: %q", opts.SubtitleSidecar)
	}
	opts.Preset = "ultrafast"
	opts.SkipVerify = true
	dst := filepath.Join(dir, "casa.mkv")
	if _, err := Normalize(context.Background(), ffmpeg, src, dst, engine.CAtv, -24, -2, opts); err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if !tieneSubtitulos(t, ffprobe, dst) {
		t.Error("la copia de casa salió sin la pista de texto del .srt")
	}
}

func TestF1_62_SCCSoloSeGuarda(t *testing.T) {
	ffmpeg, ffprobe := tools(t)
	dir := t.TempDir()
	src := makeClip(t, ffmpeg, filepath.Join(dir, "noticiero.mp4"), "30", segment{seconds: 3})
	scc := filepath.Join(dir, "noticiero.scc")
	if err := os.WriteFile(scc, []byte("Scenarist_SCC V1.0\n\n00:00:01:00\t94ae 94ae 9420 9420\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	deps := depsDePrueba(ffmpeg, ffprobe, dir)
	p := &persistenciaDePrueba{}
	deps.Persist = p
	asset, _, _, err := Ingest(context.Background(), deps, src)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if got := p.sonido(asset.ID); got.subtitulos != scc {
		t.Errorf("subtitulos_sidecar guardado = %q, se esperaba %q", got.subtitulos, scc)
	}
	if asset.ExternalCaptions == nil || *asset.ExternalCaptions != scc {
		t.Errorf("subtitulos_externos = %v", asset.ExternalCaptions)
	}

	m, err := Probe(context.Background(), ffprobe, src)
	if err != nil {
		t.Fatal(err)
	}
	// El .scc no se muxea: se queda guardado tal cual para que F2 lo vuelva
	// a meter dentro de la imagen.
	if opts := NormalizeOptionsFor(asset, m); opts.SubtitleSidecar != "" {
		t.Errorf("el .scc no se muxea en F1, y las opciones lo llevan: %q", opts.SubtitleSidecar)
	}
	if SubtituloMuxeable(scc) {
		t.Error("SubtituloMuxeable dijo que un .scc se puede meter como pista de texto")
	}
}

// ── el archivo de al lado nunca es material por su cuenta ─────────────

func TestSidecarNoEntraComoMaterialAparte(t *testing.T) {
	dir := t.TempDir()
	// Un .wav al lado de un video es el sonido de ese video…
	video := filepath.Join(dir, "spot.mp4")
	audio := filepath.Join(dir, "spot.wav")
	// …y un .wav que no acompaña a nadie es música, y entra como siempre.
	musica := filepath.Join(dir, "plena.wav")
	suelto := filepath.Join(dir, "suelto.srt")
	for _, p := range []string{video, audio, musica, suelto} {
		if err := os.WriteFile(p, make([]byte, 512), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	casos := []struct {
		path, kind, para string
	}{
		{video, EventMedia, ""},
		{audio, EventSidecar, video},
		{musica, EventMedia, ""},
		{suelto, "", ""},
	}
	for _, c := range casos {
		kind, para := classify(c.path)
		if kind != c.kind || para != c.para {
			t.Errorf("%s: aviso = %q/%q, se esperaba %q/%q", filepath.Base(c.path), kind, para, c.kind, c.para)
		}
	}
}
