package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Pruebas del conformado, que es lo que la F2 pide medir del motor: audio
// corto (F2-02), video corto (F2-03), pillarbox (F2-04) y el cambio de clip
// sin cuadros perdidos ni duplicados (F2-05).
//
// Corren sin encoder: el servidor de cuadros escribe a un recolector en
// memoria (engine.Sink). Así se puede medir cuadro a cuadro y en segundos,
// no en horas. La evidencia de punta a punta —sobre un transport stream de
// verdad— es la F0 corta, que mide lo mismo con marcadores en la imagen.

// formatoDePrueba es pequeño y a 30 cuadros justos: la cuenta de cuadros es
// exacta y una prueba de dos segundos tarda dos segundos.
var formatoDePrueba = Format{Width: 320, Height: 180, FPSNum: 30, FPSDen: 1, SampleRate: 48000, Channels: 2}

// recolector es el Sink de las pruebas: se queda con lo que el motor le
// entrega en vez de mandarlo a ffmpeg.
type recolector struct {
	cuadros [][]byte
	audio   []byte
}

func (r *recolector) WriteFrame(fr []byte) error {
	r.cuadros = append(r.cuadros, fr)
	return nil
}

func (r *recolector) WriteAudio(pcm []byte) error {
	r.audio = append(r.audio, pcm...)
	return nil
}

// luma es el brillo medio del cuadro, que es lo que distingue un clip de
// otro en estas pruebas.
func luma(fr []byte, f Format) float64 {
	n := f.Width * f.Height
	var suma float64
	for i := 0; i < n; i++ {
		suma += float64(fr[i])
	}
	return suma / float64(n)
}

// lumaEn es el brillo de un punto del cuadro.
func lumaEn(fr []byte, f Format, x, y int) float64 { return float64(fr[y*f.Width+x]) }

// pico es la muestra más alta de un trozo de PCM s16le, en valor absoluto.
func pico(pcm []byte) int {
	var max int
	for i := 0; i+1 < len(pcm); i += 2 {
		v := int(int16(uint16(pcm[i]) | uint16(pcm[i+1])<<8))
		if v < 0 {
			v = -v
		}
		if v > max {
			max = v
		}
	}
	return max
}

func herramientas(t *testing.T) (ffmpeg, ffprobe string) {
	t.Helper()
	ff, err := FFmpeg()
	if err != nil {
		t.Skip("no hay ffmpeg: " + err.Error())
	}
	fp, err := FFprobe()
	if err != nil {
		t.Skip("no hay ffprobe: " + err.Error())
	}
	return ff, fp
}

// clipAV fabrica un clip con el video y el audio de las duraciones que se
// pidan —que es como se prueba el conformado— con audio PCM para que el
// codificador no añada relleno por su cuenta.
func clipAV(t *testing.T, ffmpeg, dst, color string, w, h int, vSeg, aSeg float64) string {
	t.Helper()
	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-t", fmt.Sprintf("%.3f", vSeg),
		"-i", fmt.Sprintf("color=c=%s:s=%dx%d:r=30", color, w, h),
		"-f", "lavfi", "-t", fmt.Sprintf("%.3f", aSeg), "-i", "sine=f=440:r=48000",
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", "-g", "30",
		"-c:a", "pcm_s16le", "-ar", "48000", "-ac", "2", dst}
	if out, err := exec.Command(ffmpeg, args...).CombinedOutput(); err != nil {
		t.Fatalf("no se pudo fabricar el clip: %v\n%s", err, out)
	}
	return dst
}

// corre pone el motor a andar con esos clips y devuelve lo que salió y los
// eventos que dejó escritos.
func corre(t *testing.T, clips []Clip, dur time.Duration) (*recolector, []Event) {
	t.Helper()
	ffmpeg, ffprobe := herramientas(t)
	rec := &recolector{}
	var registro strings.Builder
	srv := NewServer(formatoDePrueba, rec, NuevaLista(clips, clips[len(clips)-1]), clips[len(clips)-1])
	srv.Ffmpeg, srv.Ffprobe, srv.Events = ffmpeg, ffprobe, &registro

	ctx, cancel := context.WithTimeout(context.Background(), dur+20*time.Second)
	defer cancel()
	if err := srv.Run(ctx, dur); err != nil {
		t.Fatalf("el motor se paró: %v", err)
	}

	var eventos []Event
	for _, linea := range strings.Split(strings.TrimSpace(registro.String()), "\n") {
		if linea == "" {
			continue
		}
		var ev Event
		if err := json.Unmarshal([]byte(linea), &ev); err != nil {
			t.Fatalf("evento ilegible %q: %v", linea, err)
		}
		eventos = append(eventos, ev)
	}
	return rec, eventos
}

// cuadrosDelPrimerClip son los cuadros que salieron del primer clip: hasta
// el segundo «corte» del registro. La lista da vueltas, así que sin esto se
// medirían dos pasadas juntas.
func cuadrosDelPrimerClip(eventos []Event, total int) int64 {
	vistos := 0
	for _, e := range eventos {
		if e.Kind != "corte" {
			continue
		}
		if vistos++; vistos == 2 {
			return e.Frame
		}
	}
	return int64(total)
}

// audioDe devuelve el trozo de audio que corresponde a los cuadros [de, a).
func audioDe(rec *recolector, f Format, de, a int64) []byte {
	i := f.SamplesUpTo(de) * int64(f.BytesPerSample())
	j := f.SamplesUpTo(a) * int64(f.BytesPerSample())
	if j > int64(len(rec.audio)) {
		j = int64(len(rec.audio))
	}
	if i < 0 || i >= j {
		return nil
	}
	return rec.audio[i:j]
}

func hayEvento(eventos []Event, kind string) bool {
	for _, e := range eventos {
		if e.Kind == kind {
			return true
		}
	}
	return false
}

// F2-02 — el audio mide menos que el video: se rellena con silencio y la
// imagen no se recorta.
func TestF2_02AudioCortoSeRellenaConSilencio(t *testing.T) {
	ffmpeg, _ := herramientas(t)
	dir := t.TempDir()
	clip := Clip{Path: clipAV(t, ffmpeg, filepath.Join(dir, "audio-corto.mkv"), "white", 320, 180, 2, 1.8), Name: "audio-corto"}

	rec, eventos := corre(t, []Clip{clip}, 2*time.Second)

	// Los dos segundos de video salieron enteros: 60 cuadros a 30.
	primero := cuadrosDelPrimerClip(eventos, len(rec.cuadros))
	if primero < 59 {
		t.Fatalf("el clip salió con %d cuadros: el video se recortó para igualar al audio", primero)
	}
	// Cada cuadro lleva su audio: la cuenta de muestras es la del reloj, no
	// la del archivo.
	quiere := int(formatoDePrueba.SamplesUpTo(int64(len(rec.cuadros)))) * formatoDePrueba.BytesPerSample()
	if len(rec.audio) != quiere {
		t.Fatalf("salieron %d bytes de audio y tocaban %d", len(rec.audio), quiere)
	}
	// El archivo traía 1.8 s de sonido: los últimos cuatro cuadros del clip
	// —del 1.87 al 2.0— son silencio digital, no el trozo anterior repetido.
	if p := pico(audioDe(rec, formatoDePrueba, primero-4, primero)); p > 300 {
		t.Fatalf("la cola del audio no es silencio: pico %d", p)
	}
	if !hayEvento(eventos, "audio_corto") {
		t.Fatal("el motor no dejó dicho que faltaba audio")
	}
}

// F2-03 — el video mide menos que el audio: se sostiene el último cuadro y
// el sonido no se corta.
func TestF2_03VideoCortoSostieneElUltimoCuadro(t *testing.T) {
	ffmpeg, _ := herramientas(t)
	dir := t.TempDir()
	clip := Clip{Path: clipAV(t, ffmpeg, filepath.Join(dir, "video-corto.mkv"), "white", 320, 180, 1.7, 2), Name: "video-corto"}

	rec, eventos := corre(t, []Clip{clip}, 2*time.Second)

	// 1.7 s de video son 51 cuadros; con el audio de 2 s tienen que salir
	// unos 60, sosteniendo el último.
	primero := cuadrosDelPrimerClip(eventos, len(rec.cuadros))
	if primero < 57 {
		t.Fatalf("el clip salió con %d cuadros: el audio se cortó para igualar al video", primero)
	}
	// El sonido sigue sonando en los cuadros sostenidos.
	if p := pico(audioDe(rec, formatoDePrueba, primero-3, primero-1)); p < 200 {
		t.Fatalf("el audio de los últimos cuadros está mudo (pico %d): se cortó el sonido", p)
	}
	// Los cuadros sostenidos son el mismo cuadro: el brillo no cambia.
	ultimo := luma(rec.cuadros[primero-1], formatoDePrueba)
	anterior := luma(rec.cuadros[primero-4], formatoDePrueba)
	if math.Abs(ultimo-anterior) > 1 {
		t.Fatalf("los últimos cuadros no son el mismo: %.1f y %.1f", anterior, ultimo)
	}
	if !hayEvento(eventos, "cuadro_sostenido") {
		t.Fatal("el motor no dejó dicho que sostuvo el último cuadro")
	}
}

// F2-04 — un 4:3 en un canal 16:9 sale con barras a los lados, no estirado.
func TestF2_04CuatroTercosSaleConBarrasALosLados(t *testing.T) {
	ffmpeg, _ := herramientas(t)
	dir := t.TempDir()
	clip := Clip{Path: clipAV(t, ffmpeg, filepath.Join(dir, "cuatro-tercios.mkv"), "white", 320, 240, 1, 1), Name: "4:3"}

	rec, _ := corre(t, []Clip{clip}, time.Second)
	if len(rec.cuadros) == 0 {
		t.Fatal("no salió ningún cuadro")
	}
	fr := rec.cuadros[len(rec.cuadros)/2]
	medio := lumaEn(fr, formatoDePrueba, formatoDePrueba.Width/2, formatoDePrueba.Height/2)
	borde := lumaEn(fr, formatoDePrueba, 2, formatoDePrueba.Height/2)
	if medio < 150 {
		t.Fatalf("el centro del cuadro no es la imagen blanca: luma %.0f", medio)
	}
	if borde > 40 {
		t.Fatalf("el borde del cuadro no es barra negra: luma %.0f (la imagen se estiró)", borde)
	}
}

// F2-05 — en el cambio de clip no se pierde ni se duplica un cuadro, y el
// decodificador del que entra ya estaba abierto.
func TestF2_05ElCambioDeClipNoPierdeNiDuplicaCuadros(t *testing.T) {
	ffmpeg, _ := herramientas(t)
	dir := t.TempDir()
	blanco := Clip{Path: clipAV(t, ffmpeg, filepath.Join(dir, "blanco.mkv"), "white", 320, 180, 1, 1), Name: "blanco"}
	gris := Clip{Path: clipAV(t, ffmpeg, filepath.Join(dir, "gris.mkv"), "gray", 320, 180, 1, 1), Name: "gris"}

	rec, eventos := corre(t, []Clip{blanco, gris}, 2*time.Second)

	// Cada clip es de un brillo distinto: la salida tiene que ser un tramo
	// de cada uno, sin mezclarse y sin repetir.
	var tramos []struct {
		claro bool
		n     int
	}
	for _, fr := range rec.cuadros {
		claro := luma(fr, formatoDePrueba) > 180
		if len(tramos) == 0 || tramos[len(tramos)-1].claro != claro {
			tramos = append(tramos, struct {
				claro bool
				n     int
			}{claro, 0})
		}
		tramos[len(tramos)-1].n++
	}
	// La lista da vueltas y el motor va unas décimas por delante del reloj,
	// así que puede haber empezado el tercer tramo. Los dos primeros son los
	// que se miden.
	if len(tramos) < 2 {
		t.Fatalf("la salida tiene %d tramos y el cambio de clip tenía que verse: %v", len(tramos), tramos)
	}
	for i, tr := range tramos[:2] {
		// Un segundo son 30 cuadros justos; se admite uno de margen por el
		// redondeo de la duración que declara el archivo.
		if tr.n < 29 || tr.n > 31 {
			t.Fatalf("el tramo %d salió con %d cuadros y tenía que traer 30: se perdieron o se duplicaron", i, tr.n)
		}
	}
	if hayEvento(eventos, "cuadro_sostenido") {
		t.Fatal("hubo que sostener un cuadro: el clip que entraba no estaba pre-cargado")
	}
	if hayEvento(eventos, "relleno") {
		t.Fatal("hizo falta relleno en un cambio de clip normal")
	}
}

// El relleno cubre el hueco de un clip que no existe, y el aire no se para
// (F2-09, F2-10: la cascada nunca termina en negro).
func TestElClipQueNoExisteLoCubreElRelleno(t *testing.T) {
	ffmpeg, _ := herramientas(t)
	dir := t.TempDir()
	relleno := Clip{Path: clipAV(t, ffmpeg, filepath.Join(dir, "relleno.mkv"), "white", 320, 180, 1, 1), Name: "relleno"}
	fantasma := Clip{Path: filepath.Join(dir, "no-existe.mkv"), Name: "fantasma"}

	rec, eventos := corre(t, []Clip{fantasma, relleno}, time.Second)
	if len(rec.cuadros) < 20 {
		t.Fatalf("salieron %d cuadros: el aire se paró con el clip que falta", len(rec.cuadros))
	}
	for i, fr := range rec.cuadros {
		if luma(fr, formatoDePrueba) < 20 {
			t.Fatalf("el cuadro %d salió negro", i)
		}
	}
	if !hayEvento(eventos, "clip_corto") && !hayEvento(eventos, "clip_fallido") {
		t.Fatal("el motor no dejó dicho que el clip no daba imagen")
	}
}
