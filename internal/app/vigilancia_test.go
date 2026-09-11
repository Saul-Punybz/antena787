package app

import (
	"context"
	"fmt"
	"math"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"antena787/internal/engine"
	"antena787/internal/model"
)

// Pruebas de la T3 de F2: el detector sobre la salida real y lo que la
// vigilancia decide con lo que ve. Los criterios que se miden aquí son F2-51,
// F2-52, F2-53, F2-54 y F2-72.
//
// El aire se fabrica a mano: cuadros y bloques de audio escritos en el
// detector, y los estados que salen pasados por a.atender uno a uno. Así una
// prueba de dieciséis segundos de silencio tarda milisegundos y no depende de
// ningún reloj de pared. La única que corre en tiempo real es la de punta a
// punta, con un clip negro y mudo hecho con ffmpeg pasando por el motor de T1.

// formatoChico es el de las pruebas del conformado: 30 cuadros justos, así
// que la cuenta de segundos es exacta.
var formatoChico = engine.Format{Width: 320, Height: 180, FPSNum: 30, FPSDen: 1, SampleRate: 48000, Channels: 2}

// relojFalso es un reloj que no se mueve solo: la prueba dice qué hora es.
type relojFalso struct{ ahora atomic.Int64 }

func nuevoReloj(t time.Time) *relojFalso {
	r := &relojFalso{}
	r.ahora.Store(t.UnixNano())
	return r
}

func (r *relojFalso) Now() time.Time        { return time.Unix(0, r.ahora.Load()).UTC() }
func (r *relojFalso) Corre(d time.Duration) { r.ahora.Add(int64(d)) }

// nadaSink es el encoder de mentira: el detector le pasa la salida y él la
// tira. Lo que importa en estas pruebas es lo que el detector midió.
type nadaSink struct{}

func (nadaSink) WriteFrame([]byte) error { return nil }
func (nadaSink) WriteAudio([]byte) error { return nil }

// aireDePrueba es una salida que se puede escribir cuadro a cuadro y una
// vigilancia que la atiende sin goroutines: síncrono y determinista.
type aireDePrueba struct {
	a        *App
	det      *engine.Detector
	neg, sil *episodioVigilado
}

func vigilado(t *testing.T, a *App) *aireDePrueba {
	t.Helper()
	det := engine.NuevoDetector(formatoChico, nadaSink{})
	det.Origen = a.Now()
	return &aireDePrueba{
		a:   a,
		det: det,
		neg: &episodioVigilado{que: "negro"},
		sil: &episodioVigilado{que: "silencio"},
	}
}

// emite escribe esos segundos de salida con esa luma y ese nivel de audio, y
// atiende todos los estados que el detector contó.
func (p *aireDePrueba) emite(t *testing.T, segundos float64, luma byte, dbfs float64) {
	t.Helper()
	ctx := context.Background()
	f := p.det.Formato
	cuadros := int(math.Round(segundos * f.FPS()))
	fr := cuadroConLuma(f, luma)
	amp := amplitudDeDBFS(dbfs)
	for i := 0; i < cuadros; i++ {
		vistos := p.det.Cuadros()
		n := int(f.SamplesUpTo(vistos+1) - f.SamplesUpTo(vistos))
		if err := p.det.WriteFrame(fr); err != nil {
			t.Fatalf("el detector no pasó el cuadro: %v", err)
		}
		if err := p.det.WriteAudio(audioConNivel(f, n, amp)); err != nil {
			t.Fatalf("el detector no pasó el audio: %v", err)
		}
		p.atender(ctx)
	}
}

// atender pasa por la vigilancia lo que haya en el canal de estados.
func (p *aireDePrueba) atender(ctx context.Context) {
	for {
		select {
		case e := <-p.det.Estados():
			p.a.atender(ctx, e, p.neg, p.sil)
		default:
			return
		}
	}
}

// cuadroConLuma y audioConNivel fabrican la salida: un cuadro yuv420p con
// toda la imagen a la misma luma, y un bloque de PCM s16le con todas las
// muestras a la misma amplitud (el RMS del bloque es esa amplitud).
func cuadroConLuma(f engine.Format, luma byte) []byte {
	fr := make([]byte, f.FrameBytes())
	n := f.Width * f.Height
	for i := 0; i < n; i++ {
		fr[i] = luma
	}
	for i := n; i < len(fr); i++ {
		fr[i] = 128
	}
	return fr
}

func audioConNivel(f engine.Format, muestras int, amplitud int16) []byte {
	pcm := make([]byte, muestras*f.BytesPerSample())
	for i := 0; i+1 < len(pcm); i += 2 {
		pcm[i] = byte(uint16(amplitud))
		pcm[i+1] = byte(uint16(amplitud) >> 8)
	}
	return pcm
}

func amplitudDeDBFS(dbfs float64) int16 {
	return int16(math.Round(math.Pow(10, dbfs/20) * 32768))
}

// alarmaDe busca una alarma por tipo entre las vivas.
func alarmaDe(a *App, tipo string) *Alarma {
	for _, al := range a.Alarms() {
		if al.Tipo == tipo {
			copia := al
			return &copia
		}
	}
	return nil
}

// incidentesDe son los incidentes de un tipo que hay en la bitácora.
func incidentesDe(t *testing.T, a *App, tipo string) []model.Incident {
	t.Helper()
	ctx := context.Background()
	todos, err := a.Store.Incident.List(ctx, a.ChannelID, a.Now().Add(-24*time.Hour), a.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("no pude leer la bitácora: %v", err)
	}
	var out []model.Incident
	for _, i := range todos {
		if i.Kind == tipo {
			out = append(out, i)
		}
	}
	return out
}

// ── F2-51 · silencio real en la salida ────────────────────────────────

// Dieciséis segundos de audio bajo −60 dBFS —por encima del umbral de 15 s—
// disparan alarma e incidente, diga lo que diga el plan (aquí el plan está
// vacío: nadie pidió silencio y aun así se avisa).
func TestF2_51SilencioAlAireAvisaYQuedaEnLaBitacora(t *testing.T) {
	a := abre(t)
	p := vigilado(t, a)

	p.emite(t, 16, 120, -70) // imagen buena, audio mudo

	al := alarmaDe(a, "silencio_al_aire")
	if al == nil {
		t.Fatalf("no salió la alarma de silencio: %v", a.Alarms())
	}
	if al.Nivel != NivelProblema {
		t.Errorf("la alarma es de nivel %q y el canal está mudo", al.Nivel)
	}
	if !strings.Contains(al.Texto, "sin sonido") {
		t.Errorf("la alarma dice %q y tenía que decirlo en palabras claras", al.Texto)
	}
	incs := incidentesDe(t, a, model.IncSilencioDetectado.String())
	if len(incs) != 1 {
		t.Fatalf("hay %d incidentes de silencio y tenía que haber uno", len(incs))
	}
	// Dura mientras dure: el incidente se inserta abierto y se cierra cuando
	// vuelve la señal (ver el comentario de App.Incident).
	if incs[0].End != nil {
		t.Error("el incidente se cerró en el mismo instante y el silencio sigue")
	}
	if TextoDeIncidente(incs[0].Kind) == incs[0].Kind {
		t.Error("el tipo de incidente no tiene frase clara")
	}
}

// ── F2-52 · negro real en la salida ───────────────────────────────────

func TestF2_52NegroAlAireAvisaYQuedaEnLaBitacora(t *testing.T) {
	a := abre(t)
	p := vigilado(t, a)

	p.emite(t, 16, 16, -20) // sonido bueno, imagen en negro de verdad (16 = negro digital)

	al := alarmaDe(a, "negro_al_aire")
	if al == nil {
		t.Fatalf("no salió la alarma de negro: %v", a.Alarms())
	}
	if !strings.Contains(al.Detalle, "luma") {
		t.Errorf("el detalle dice %q y tenía que dar el número medido", al.Detalle)
	}
	if n := len(incidentesDe(t, a, model.IncNegroDetectado.String())); n != 1 {
		t.Fatalf("hay %d incidentes de negro y tenía que haber uno", n)
	}
	// El negro con sonido no es silencio: no se cuenta dos veces la misma cosa.
	if n := len(incidentesDe(t, a, model.IncSilencioDetectado.String())); n != 0 {
		t.Errorf("se contaron %d silencios con el audio a −20 dBFS", n)
	}
}

// ── F2-53 · la pausa dramática no dispara ─────────────────────────────

// Ocho segundos de silencio, por debajo del umbral de quince: ni alarma ni
// incidente. Es el criterio que impide que el detector sea un lobo que grita.
func TestF2_53OchoSegundosDeSilencioNoDisparan(t *testing.T) {
	a := abre(t)
	p := vigilado(t, a)

	p.emite(t, 8, 120, -70)
	p.emite(t, 2, 120, -20) // y vuelve el sonido

	if al := alarmaDe(a, "silencio_al_aire"); al != nil {
		t.Fatalf("ocho segundos de pausa dispararon la alarma: %q", al.Texto)
	}
	if n := len(incidentesDe(t, a, model.IncSilencioDetectado.String())); n != 0 {
		t.Fatalf("ocho segundos de pausa dejaron %d incidentes", n)
	}
}

// La alarma va y viene con el hecho, y el incidente se cierra: la bitácora de
// una semana no puede arrastrar para siempre un silencio que ya pasó.
func TestLaAlarmaSeApagaCuandoVuelveLaSenal(t *testing.T) {
	a := abre(t)
	p := vigilado(t, a)

	p.emite(t, 16, 120, -70)
	if alarmaDe(a, "silencio_al_aire") == nil {
		t.Fatal("no salió la alarma")
	}
	p.emite(t, 2, 120, -20)

	if al := alarmaDe(a, "silencio_al_aire"); al != nil {
		t.Fatalf("la alarma se quedó puesta con el sonido de vuelta: %q", al.Texto)
	}
	incs := incidentesDe(t, a, model.IncSilencioDetectado.String())
	if len(incs) != 1 || incs[0].End == nil {
		t.Fatalf("el incidente no se cerró al volver la señal: %+v", incs)
	}
}

// Un episodio largo deja un incidente, no uno por latido.
func TestUnIncidentePorEpisodio(t *testing.T) {
	a := abre(t)
	p := vigilado(t, a)

	p.emite(t, 60, 120, -70)

	if n := len(incidentesDe(t, a, model.IncSilencioDetectado.String())); n != 1 {
		t.Fatalf("un minuto de silencio dejó %d incidentes y tenía que dejar uno", n)
	}
	// Y la alarma sigue contando cuánto lleva, no cuánto llevaba al disparar.
	al := alarmaDe(a, "silencio_al_aire")
	if al == nil || !strings.Contains(al.Texto, "59s") && !strings.Contains(al.Texto, "1m0s") {
		t.Errorf("la alarma dice %v y el silencio lleva un minuto", al)
	}
}

// Mudo y en negro a la vez son dos cosas, y las dos se ven.
func TestSilencioYNegroALaVezSonDosAvisos(t *testing.T) {
	a := abre(t)
	p := vigilado(t, a)

	p.emite(t, 16, 16, -90)

	if alarmaDe(a, "negro_al_aire") == nil || alarmaDe(a, "silencio_al_aire") == nil {
		t.Fatalf("faltó uno de los dos avisos: %v", a.Alarms())
	}
	// Y que vuelva la imagen no apaga el aviso de que sigue mudo.
	p.emite(t, 2, 120, -90)
	if alarmaDe(a, "negro_al_aire") != nil {
		t.Error("la alarma de negro se quedó puesta con la imagen de vuelta")
	}
	if alarmaDe(a, "silencio_al_aire") == nil {
		t.Error("volvió la imagen y se apagó el aviso de que el canal sigue mudo")
	}
}

// El umbral se lee de settings, y lo que no se entiende vale el de fábrica:
// un ajuste escrito con un dedo torcido no puede apagar la vigilancia.
func TestElUmbralSeLeeDeAjustesYSeDefiende(t *testing.T) {
	a := abre(t)
	ctx := context.Background()

	if d := a.umbralDe(ctx, KeySilencioUmbral, UmbralSilencio); d != UmbralSilencio {
		t.Errorf("sin ajuste guardado el umbral es %s y tenía que ser el de fábrica", d)
	}
	for _, malo := range []string{"", "muchos", "0", "2", "999", "-8"} {
		if err := a.Store.Settings.Set(ctx, KeySilencioUmbral, malo); err != nil {
			t.Fatalf("no pude guardar el ajuste: %v", err)
		}
		if d := a.umbralDe(ctx, KeySilencioUmbral, UmbralSilencio); d != UmbralSilencio {
			t.Errorf("con el ajuste %q el umbral quedó en %s", malo, d)
		}
	}
	if err := a.Store.Settings.Set(ctx, KeySilencioUmbral, "25"); err != nil {
		t.Fatalf("no pude guardar el ajuste: %v", err)
	}
	if d := a.umbralDe(ctx, KeySilencioUmbral, UmbralSilencio); d != 25*time.Second {
		t.Errorf("con el ajuste en 25 el umbral quedó en %s", d)
	}

	// Y el umbral guardado es el que manda: con 30 s, dieciséis no disparan.
	if err := a.Store.Settings.Set(ctx, KeySilencioUmbral, "30"); err != nil {
		t.Fatalf("no pude guardar el ajuste: %v", err)
	}
	p := vigilado(t, a)
	p.emite(t, 16, 120, -70)
	if alarmaDe(a, "silencio_al_aire") != nil {
		t.Error("con el umbral en 30 s, dieciséis segundos dispararon")
	}
	p.emite(t, 16, 120, -70)
	if alarmaDe(a, "silencio_al_aire") == nil {
		t.Error("con el umbral en 30 s, treinta y dos segundos no dispararon")
	}

	// UmbralValido es lo que la API contesta en palabras claras.
	if _, err := UmbralValido("15"); err != nil {
		t.Errorf("15 s tenía que valer: %v", err)
	}
	if _, err := UmbralValido("1"); err == nil {
		t.Error("1 s tenía que sobrar por abajo")
	}
	if _, err := UmbralValido("ocho"); err == nil {
		t.Error("«ocho» no es un número de segundos")
	}
}

// ── F2-72 · negro_intencional ─────────────────────────────────────────

// Un archivo marcado `negro_intencional` que abre con veinte segundos de
// negro y silencio no dispara nada mientras dura, y el detector vuelve a
// estar activo en cuanto termina.
func TestF2_72NegroIntencionalNoDispara(t *testing.T) {
	reloj := nuevoReloj(time.Date(2026, 9, 10, 20, 0, 0, 0, time.UTC))
	a := abre(t, func(o *Options) { o.Now = reloj.Now })
	ctx := context.Background()

	// El bloque que está saliendo ahora mismo es ese archivo.
	asset, _ := conBloque(t, a, "Cortinilla que abre en negro", a.Now().Add(-5*time.Second), 10*time.Minute)
	asset.IntentionalBlack = true
	if err := a.Store.Media.Update(ctx, &asset); err != nil {
		t.Fatalf("no pude marcar el archivo: %v", err)
	}

	p := vigilado(t, a)
	p.emite(t, 20, 16, -90)

	if al := alarmaDe(a, "negro_al_aire"); al != nil {
		t.Fatalf("el negro a propósito disparó la alarma: %q", al.Texto)
	}
	if al := alarmaDe(a, "silencio_al_aire"); al != nil {
		t.Fatalf("el silencio del negro a propósito disparó la alarma: %q", al.Texto)
	}
	if n := len(incidentesDe(t, a, model.IncNegroDetectado.String())); n != 0 {
		t.Fatalf("el negro a propósito dejó %d incidentes", n)
	}

	// Se acabó el clip a propósito y lo que sale ahora es otro archivo, sin la
	// marca: el negro vuelve a ser negro. El episodio se cierra con imagen y
	// se abre otro, que es cuando se vuelve a mirar el plan.
	asset.IntentionalBlack = false
	if err := a.Store.Media.Update(ctx, &asset); err != nil {
		t.Fatalf("no pude quitar la marca: %v", err)
	}
	p.emite(t, 1, 120, -20)
	p.emite(t, 16, 16, -90)
	if alarmaDe(a, "negro_al_aire") == nil {
		t.Fatalf("terminado el clip a propósito, el negro tenía que disparar: %v", a.Alarms())
	}
}

// Una fuente en vivo nunca desactiva el detector: un vivo congelado en negro
// es exactamente el caso que hay que avisar (F2-72, segunda frase).
func TestF2_72UnVivoNoDesactivaElDetector(t *testing.T) {
	reloj := nuevoReloj(time.Date(2026, 9, 10, 21, 0, 0, 0, time.UTC))
	a := abre(t, func(o *Options) { o.Now = reloj.Now })
	ctx := context.Background()

	asset, item := conBloque(t, a, "RadioOnce Live", a.Now().Add(-time.Minute), 30*time.Minute)
	asset.IntentionalBlack = true
	if err := a.Store.Media.Update(ctx, &asset); err != nil {
		t.Fatalf("no pude marcar el archivo: %v", err)
	}
	// El mismo bloque, pero su origen es una fuente en vivo.
	if _, err := a.Store.DB().ExecContext(ctx,
		`UPDATE plan_item SET origen = ? WHERE id = ?`, model.OriginLiveSource, item.ID); err != nil {
		t.Fatalf("no pude cambiar el origen: %v", err)
	}

	p := vigilado(t, a)
	p.emite(t, 16, 16, -90)

	if alarmaDe(a, "negro_al_aire") == nil {
		t.Fatalf("un vivo congelado en negro no avisó: %v", a.Alarms())
	}
}

// ── F2-54 · el mismo mecanismo en automático y en manual ──────────────

// controlDeMentira es el gancho de T5 hasta que T5 exista.
type controlDeMentira struct {
	manual  bool
	vueltas atomic.Int64
	motivo  atomic.Value
}

func (c *controlDeMentira) EnManual() bool { return c.manual }

func (c *controlDeMentira) VolverAlAutomatico(_ context.Context, motivo string) error {
	c.vueltas.Add(1)
	c.motivo.Store(motivo)
	return nil
}

// conControl instala el control de mentira y lo quita al terminar.
func conControl(t *testing.T, c ControlDelAire) {
	t.Helper()
	PonerControlDelAire(c)
	t.Cleanup(func() { PonerControlDelAire(nil) })
}

// En manual, con «avisa y devuelve el control» encendido, el mismo umbral que
// gobierna el automático suelta el aire y deja el incidente con su hora
// (F2-30, F2-54).
func TestF2_54EnManualElMismoUmbralDevuelveElControl(t *testing.T) {
	reloj := nuevoReloj(time.Date(2026, 9, 10, 3, 0, 0, 0, time.UTC))
	a := abre(t, func(o *Options) { o.Now = reloj.Now })
	c := &controlDeMentira{manual: true}
	conControl(t, c)

	p := vigilado(t, a)
	p.emite(t, 16, 120, -70)

	if c.vueltas.Load() != 1 {
		t.Fatalf("el aire volvió al automático %d veces y tenía que volver una", c.vueltas.Load())
	}
	if m, _ := c.motivo.Load().(string); !strings.Contains(m, "silencio") {
		t.Errorf("el motivo que se le pasó al control es %q", m)
	}
	// El aviso del silencio sale igual —es el mismo mecanismo— y además
	// queda dicho que el control venció.
	if alarmaDe(a, "silencio_al_aire") == nil {
		t.Error("en manual no salió el aviso de silencio")
	}
	incs := incidentesDe(t, a, model.IncManualPorTimeout.String())
	if len(incs) != 1 {
		t.Fatalf("hay %d incidentes de manual_por_timeout", len(incs))
	}
	if !incs[0].Start.Equal(a.Now()) {
		t.Errorf("el incidente dice %s y el reloj dice %s", incs[0].Start, a.Now())
	}
}

// Con el interruptor apagado se avisa y no se toca el aire: el operador manda.
func TestSinDevolverElControlSoloSeAvisa(t *testing.T) {
	a := abre(t)
	if err := a.Store.Settings.Set(context.Background(), KeySilencioDevuelveControl, "no"); err != nil {
		t.Fatalf("no pude guardar el ajuste: %v", err)
	}
	c := &controlDeMentira{manual: true}
	conControl(t, c)

	p := vigilado(t, a)
	p.emite(t, 16, 120, -70)

	if c.vueltas.Load() != 0 {
		t.Fatalf("con el interruptor apagado el aire volvió solo %d veces", c.vueltas.Load())
	}
	if alarmaDe(a, "silencio_al_aire") == nil {
		t.Error("con el interruptor apagado tampoco se avisó")
	}
	if n := len(incidentesDe(t, a, model.IncManualPorTimeout.String())); n != 0 {
		t.Errorf("se anotaron %d vencimientos del manual sin haberlo soltado", n)
	}
}

// En automático el gancho no se llama: no hay control que devolver.
func TestEnAutomaticoNoSeTocaElControl(t *testing.T) {
	a := abre(t)
	c := &controlDeMentira{manual: false}
	conControl(t, c)

	p := vigilado(t, a)
	p.emite(t, 16, 120, -70)

	if c.vueltas.Load() != 0 {
		t.Fatalf("en automático se pidió volver al automático %d veces", c.vueltas.Load())
	}
	if alarmaDe(a, "silencio_al_aire") == nil {
		t.Error("en automático no se avisó")
	}
}

// ── de punta a punta, con el motor de T1 ──────────────────────────────

// Un clip negro y mudo de verdad —hecho con ffmpeg— pasando por el servidor
// de cuadros de T1 y por el detector enganchado con App.Vigilar: la alarma y
// el incidente salen solos, sin que nadie mire el plan.
//
// Es la prueba del gancho: lo que hace esta prueba es exactamente la línea
// que motor.go tiene que meter al fusionar T3 (ver el comentario de Vigilar).
func TestF2_51DePuntaAPuntaConUnClipNegroYMudo(t *testing.T) {
	ffmpeg, ffprobe := ffmpegDePrueba(t)
	a := abre(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Umbral al mínimo para que la prueba tarde segundos y no minutos; el
	// mecanismo es el mismo con quince.
	for clave, v := range map[string]string{KeySilencioUmbral: "3", KeyNegroUmbral: "3"} {
		if err := a.Store.Settings.Set(ctx, clave, v); err != nil {
			t.Fatalf("no pude guardar el ajuste: %v", err)
		}
	}

	ruta := filepath.Join(t.TempDir(), "negro-y-mudo.mkv")
	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-t", "12", "-i", fmt.Sprintf("color=c=black:s=%dx%d:r=30", formatoChico.Width, formatoChico.Height),
		"-f", "lavfi", "-t", "12", "-i", "anullsrc=r=48000:cl=stereo",
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", "-g", "30",
		"-c:a", "pcm_s16le", ruta}
	if out, err := exec.Command(ffmpeg, args...).CombinedOutput(); err != nil {
		t.Fatalf("no pude fabricar el clip negro y mudo: %v\n%s", err, out)
	}

	clip := engine.Clip{Path: ruta, Name: "negro y mudo"}
	// El gancho: el detector se pone entre el servidor de cuadros y el
	// encoder. Aquí el «encoder» es un sumidero, que es lo que le toca a una
	// prueba que mide la vigilancia y no ffmpeg.
	salida := a.Vigilar(ctx, formatoChico, nadaSink{})
	srv := engine.NewServer(formatoChico, salida, engine.NuevaLista([]engine.Clip{clip}, clip), clip)
	srv.Ffmpeg, srv.Ffprobe = ffmpeg, ffprobe

	hecho := make(chan error, 1)
	go func() { hecho <- srv.Run(ctx, 7*time.Second) }()

	esperar(t, 12*time.Second, func() bool {
		return alarmaDe(a, "negro_al_aire") != nil && alarmaDe(a, "silencio_al_aire") != nil
	}, "el clip negro y mudo salió al aire y no se avisó")

	cancel()
	select {
	case <-hecho:
	case <-time.After(10 * time.Second):
		t.Fatal("el motor no paró")
	}

	esperar(t, 5*time.Second, func() bool {
		return len(incidentesDe(t, a, model.IncNegroDetectado.String())) == 1 &&
			len(incidentesDe(t, a, model.IncSilencioDetectado.String())) == 1
	}, "los incidentes de negro y silencio no quedaron en la bitácora")
}

// ffmpegDePrueba salta la prueba si la máquina no tiene las herramientas.
func ffmpegDePrueba(t *testing.T) (ffmpeg, ffprobe string) {
	t.Helper()
	a := abre(t)
	if a.FFmpegErr != nil {
		t.Skip("no hay ffmpeg: " + a.FFmpegErr.Error())
	}
	return a.FFmpeg, a.FFprobe
}
