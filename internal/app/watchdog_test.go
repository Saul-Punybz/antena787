// watchdog_test.go mide el criterio F2-11 entero: el encoder de salida
// congelado —vivo, sin consumir un solo cuadro— se detecta a los tres
// segundos, sale el cartel de la estación, queda el incidente
// `encoder_reiniciado`, el aire vuelve solo con el mismo acelerador, y a la
// segunda vez en diez minutos el canal sigue emitiendo por software.
//
// La prueba de punta a punta congela el ffmpeg de verdad con `kill -STOP`,
// que es congelado y no muerto: un encoder muerto ya lo veía F1 por
// enc.Done(), y no es lo que F2-11 describe.
package app

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"antena787/internal/engine"
	"antena787/internal/model"
)

// olvidarLaTarjeta deja en limpio lo que el vigilante recuerda entre vidas del
// encoder. Es estado de paquete —un proceso, un canal, PRD §14.1—, así que
// cada prueba lo borra al entrar y al salir.
func olvidarLaTarjeta(t *testing.T) {
	t.Helper()
	limpiar := func() {
		laTarjeta.mu.Lock()
		laTarjeta.colgadas, laTarjeta.cartel = nil, time.Time{}
		laTarjeta.mu.Unlock()
	}
	limpiar()
	t.Cleanup(limpiar)
}

// ── lo que el vigilante mira ──────────────────────────────────────────

// encoderQueSeCuelga es un encoder de mentira que se traga los cuadros hasta
// que se congela: a partir de ahí la escritura no vuelve nunca, que es
// exactamente lo que le pasa a un ffmpeg que dejó de leer su socket.
type encoderQueSeCuelga struct {
	colgado atomic.Bool
	soltar  chan struct{}
}

func (e *encoderQueSeCuelga) WriteFrame([]byte) error { return e.escribir() }
func (e *encoderQueSeCuelga) WriteAudio([]byte) error { return e.escribir() }

func (e *encoderQueSeCuelga) escribir() error {
	if e.colgado.Load() {
		<-e.soltar
	}
	return nil
}

// F2-11 — el síntoma es una escritura que no vuelve, y el vigilante la ve.
// Mientras el encoder traga, no hay nada atascado: el motor no puede relanzar
// ffmpeg porque el servidor de cuadros esté abriendo el clip que viene.
func TestF2_11ElVigilanteVeLaEscrituraQueNoVuelve(t *testing.T) {
	e := &encoderQueSeCuelga{soltar: make(chan struct{})}
	v := &encoderVigilado{destino: e}

	if err := v.WriteFrame(nil); err != nil {
		t.Fatalf("con el encoder sano la escritura vuelve: %v", err)
	}
	if d := v.atascado(time.Now()); d != 0 {
		t.Fatalf("sin escritura en vuelo no hay nada atascado, y midió %s", d)
	}

	e.colgado.Store(true)
	volvio := make(chan struct{})
	go func() {
		defer close(volvio)
		_ = v.WriteFrame(nil)
	}()
	esperar(t, 5*time.Second, func() bool { return v.atascado(time.Now()) >= 100*time.Millisecond },
		"el vigilante no vio que la escritura lleva rato sin volver")

	close(e.soltar)
	select {
	case <-volvio:
	case <-time.After(5 * time.Second):
		t.Fatal("la escritura no se despegó al soltar el encoder")
	}
	if d := v.atascado(time.Now()); d != 0 {
		t.Fatalf("con la escritura ya devuelta no queda nada atascado, y midió %s", d)
	}
}

// F2-11 — el plazo de fábrica son tres segundos y se puede cambiar en
// ajustes; lo que no se entiende o se sale de rango vale el de fábrica.
func TestF2_11ElPlazoDeFabricaSonTresSegundosYSeCambia(t *testing.T) {
	a := abre(t)
	ctx := context.Background()

	if d := a.plazoDelEncoder(ctx); d != 3*time.Second {
		t.Fatalf("el plazo de fábrica son tres segundos (F2-11) y es %s", d)
	}
	for escrito, quiere := range map[string]time.Duration{
		"5":      5 * time.Second,
		"medio":  EncoderColgado,
		"0":      EncoderColgado,
		"100000": EncoderColgado,
	} {
		if err := a.Store.Settings.Set(ctx, KeyEncoderColgadoS, escrito); err != nil {
			t.Fatalf("no pude guardar el ajuste: %v", err)
		}
		if d := a.plazoDelEncoder(ctx); d != quiere {
			t.Fatalf("con %q escrito el plazo es %s y tenía que ser %s", escrito, d, quiere)
		}
	}
}

// ── lo que el vigilante hace ──────────────────────────────────────────

// F2-11 — al colgarse el encoder queda el incidente `encoder_reiniciado`, y
// su texto lo entiende quien no es técnico.
func TestF2_11ElEncoderColgadoDejaElIncidenteEnCristiano(t *testing.T) {
	olvidarLaTarjeta(t)
	a := abre(t)

	a.elEncoderSeColgo(&engine.Encoder{}, engine.AcelSoftware, EncoderColgado)

	texto, hay := elIncidente(t, a, model.IncEncoderReiniciado)
	if !hay {
		t.Fatal("colgarse el encoder no dejó el incidente encoder_reiniciado en la bitácora")
	}
	if !strings.Contains(texto, "cartel de la estación") {
		t.Fatalf("el incidente no dice que sale el cartel: %q", texto)
	}
	for _, jerga := range []string{"timeout", "watchdog", "hardware", "buffer", "ffmpeg"} {
		if strings.Contains(strings.ToLower(texto), jerga) {
			t.Fatalf("el incidente lo lee una persona que no es técnica y dice %q: %q", jerga, texto)
		}
	}
}

// F2-11 — después de una colgada el aire vuelve con el cartel de la estación,
// no con el clip que pudo ser quien colgó a la salida. Pasada la gracia,
// manda el plan otra vez.
func TestF2_11TrasLaColgadaElAireVuelveConElCartel(t *testing.T) {
	olvidarLaTarjeta(t)
	a := abre(t)
	ctx := context.Background()
	cartel := conCartel(t, a)

	ahora := a.Now()
	_, item := conBloque(t, a, "Nitro", ahora, 30*time.Minute)

	// Sin colgada manda el plan, como siempre.
	f := a.nuevaFuenteDelPlan(ctx, engine.CAtv)
	clip, _, err := f.Next(ahora)
	if err != nil {
		t.Fatalf("la fuente no supo qué poner: %v", err)
	}
	if clip.Ref != item.ID {
		t.Fatalf("sin colgada sale el plan y salió el clip %q (bloque %d)", clip.Name, clip.Ref)
	}

	// Con la colgada apuntada, la vida siguiente del encoder abre con el
	// cartel y solo durante la gracia.
	apuntarColgada(time.Now())
	f = a.nuevaFuenteDelPlan(ctx, engine.CAtv)
	clip, hasta, err := f.Next(ahora)
	if err != nil {
		t.Fatalf("la fuente no supo qué poner tras la colgada: %v", err)
	}
	if clip.Path != cartel {
		t.Fatalf("tras la colgada tenía que salir el cartel %q y salió %q", cartel, clip.Path)
	}
	if clip.Ref != 0 {
		t.Fatalf("el cartel no es un bloque del plan y salió con la referencia %d", clip.Ref)
	}
	if !hasta.After(ahora) || hasta.After(ahora.Add(GraciaDelCartel+time.Second)) {
		t.Fatalf("el cartel manda hasta %s y la gracia es de %s desde %s", hasta, GraciaDelCartel, ahora)
	}

	// Pasada la gracia, el plan recupera el aire sin que nadie lo toque.
	laTarjeta.mu.Lock()
	laTarjeta.cartel = time.Now().Add(-time.Second)
	laTarjeta.mu.Unlock()
	clip, _, err = f.Next(ahora)
	if err != nil {
		t.Fatalf("la fuente no supo qué poner pasada la gracia: %v", err)
	}
	if clip.Ref != item.ID {
		t.Fatalf("pasada la gracia vuelve el plan y salió %q", clip.Name)
	}
}

// F2-11 — una colgada es mala suerte y el canal sigue con su tarjeta; dos en
// diez minutos es una tarjeta que dejó de responder, y entonces el canal
// sigue emitiendo por software y lo dice en palabras claras.
func TestF2_11DosColgadasEnDiezMinutosBajanElCanalAlProcesador(t *testing.T) {
	olvidarLaTarjeta(t)
	a := abre(t)
	ctx := context.Background()
	t.Cleanup(func() { a.ForzarAcelerador("", "") })

	ch, err := a.Store.Channel.Get(ctx, a.ChannelID)
	if err != nil {
		t.Fatalf("no pude leer el canal: %v", err)
	}
	ch.Accel = string(engine.AcelNVENC)
	if err := a.Store.Channel.Update(ctx, ch); err != nil {
		t.Fatalf("no pude poner el canal a comprimir con la tarjeta: %v", err)
	}

	a.elEncoderSeColgo(&engine.Encoder{}, engine.AcelNVENC, EncoderColgado)
	if ac, porque := a.AceleradorEnCurso(); ac != engine.AcelNVENC || porque != "" {
		t.Fatalf("con una sola colgada el canal se queda con su tarjeta, y quedó en %q (%q)", ac, porque)
	}

	a.elEncoderSeColgo(&engine.Encoder{}, engine.AcelNVENC, EncoderColgado)
	ac, porque := a.AceleradorEnCurso()
	if ac != engine.AcelSoftware {
		t.Fatalf("dos colgadas en diez minutos dejan el canal en software y quedó en %q", ac)
	}
	if !strings.Contains(porque, FraseTarjetaCaida) {
		t.Fatalf("el porqué tiene que decir %q en palabras claras y dice %q", FraseTarjetaCaida, porque)
	}
	if texto, _ := elIncidente(t, a, model.IncEncoderReiniciado); !strings.Contains(texto, FraseTarjetaCaida) {
		t.Fatalf("el incidente de la segunda colgada tiene que decir %q y dice %q", FraseTarjetaCaida, texto)
	}
}

// ── el encoder de verdad, congelado ───────────────────────────────────

// F2-11 de punta a punta — el ffmpeg de salida congelado con `kill -STOP`: la
// señal deja de salir, a los tres segundos el motor lo mata, sale el cartel,
// queda el incidente y el aire vuelve solo. Repetido, el canal se queda
// emitiendo por software y dice por qué.
func TestF2_11ElEncoderCongeladoVuelveSoloYSaleElCartel(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("congelar un proceso sin matarlo no existe en Windows; el vigilante sí corre allí")
	}
	if _, err := exec.LookPath("pgrep"); err != nil {
		t.Skip("no hay pgrep para encontrar el encoder: " + err.Error())
	}
	ffmpeg, _ := herramientas(t)
	olvidarLaTarjeta(t)
	a := abre(t)
	ctx := context.Background()
	t.Cleanup(func() { a.ForzarAcelerador("", "") })
	var vistos eventos
	vistos.mirando(a, t)

	// El canal al aire, en formato chico —la prueba cabe en segundos— y
	// comprimiendo con la tarjeta, que es lo único que deja ver la caída a
	// software. En MPEG-2 la tarjeta NVIDIA comprime igual con el procesador,
	// así que ffmpeg arranca en cualquier máquina.
	ch, err := a.Store.Channel.Get(ctx, a.ChannelID)
	if err != nil {
		t.Fatalf("no pude leer el canal: %v", err)
	}
	ch.Mode, ch.FormatProfile, ch.Accel = ModoAire, "480i59.94", string(engine.AcelNVENC)
	if err := a.Store.Channel.Update(ctx, ch); err != nil {
		t.Fatalf("no pude poner el canal al aire: %v", err)
	}

	// La señal sale a un puerto de esta prueba: contar los paquetes que
	// llegan es la única forma honesta de saber si el aire se quedó pegado.
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("no pude abrir el puerto de la prueba: %v", err)
	}
	defer pc.Close()
	aire := &aireQueLlega{}
	go aire.contar(pc)

	dir := t.TempDir()
	clip := clipConSonido(t, ffmpeg, filepath.Join(dir, "programa.mkv"), 30)
	if err := a.Store.Settings.Set(ctx, KeyDefaultFiller, clip); err != nil {
		t.Fatalf("no pude apuntar el cartel: %v", err)
	}
	asset := model.MediaAsset{
		Path: clip, NormalizedPath: clip, DurationMs: 30000,
		State: model.AssetReady, NormalizeState: model.NormalizeReady,
		CreatedAt: a.Now(), UpdatedAt: a.Now(),
	}
	if err := a.Store.Media.Insert(ctx, &asset); err != nil {
		t.Fatalf("no pude guardar el archivo: %v", err)
	}
	conBloqueDe(t, a, asset, model.DeckProgram, a.Now(), 30*time.Minute, nil)

	if err := a.Store.Output.Upsert(ctx, &model.Output{
		ChannelID: a.ChannelID, Name: "transmisor", Driver: "udp-ts",
		Params: fmt.Sprintf(`{"destino": %q, "bitrate_mux_kbs": 2000, "bitrate_video_kbs": 1000}`,
			pc.LocalAddr().String()),
	}); err != nil {
		t.Fatalf("no pude configurar la salida: %v", err)
	}

	vivo, cancel := context.WithCancel(ctx)
	defer cancel()
	a.Start(vivo)

	esperar(t, 60*time.Second, func() bool { return aire.cuantos() > 50 },
		"la señal nunca llegó a salir: no hay nada que congelar")

	// ── primera congelada ──
	pid := elEncoderDeSalida(t, "")
	congelar(t, pid)

	// Con el encoder congelado no sale un paquete más: el aire está pegado.
	time.Sleep(1500 * time.Millisecond)
	pegado := aire.cuantos()
	time.Sleep(500 * time.Millisecond)
	if aire.cuantos() != pegado {
		t.Fatalf("el encoder está congelado y siguieron saliendo paquetes (%d → %d)", pegado, aire.cuantos())
	}

	// Y vuelve solo: el motor lo mata, enciende otro y la señal sale otra vez.
	esperar(t, 60*time.Second, func() bool { return aire.cuantos() > pegado+50 },
		"el aire no volvió solo después de congelar el encoder")

	if _, hay := elIncidente(t, a, model.IncEncoderReiniciado); !hay {
		t.Fatal("la bitácora no registró encoder_reiniciado")
	}
	esperar(t, 10*time.Second, func() bool { return vistos.hay("motor", "deck", "cartel de la estación") },
		"el aire no volvió con el cartel de la estación")

	// Con una sola colgada el canal sigue con la tarjeta que se escogió.
	if ac, porque := a.AceleradorEnCurso(); ac != engine.AcelNVENC || porque != "" {
		t.Fatalf("con una colgada el canal se relanza con el mismo acelerador, y quedó en %q (%q)", ac, porque)
	}

	// ── segunda congelada, dentro de los diez minutos ──
	otro := elEncoderDeSalida(t, pid)
	congelar(t, otro)
	antes := aire.cuantos()
	esperar(t, 60*time.Second, func() bool { return aire.cuantos() > antes+50 },
		"el aire no volvió después de la segunda congelada")

	ac, porque := a.AceleradorEnCurso()
	if ac != engine.AcelSoftware {
		t.Fatalf("dos veces colgado en diez minutos deja el canal en software y quedó en %q", ac)
	}
	if !strings.Contains(porque, FraseTarjetaCaida) {
		t.Fatalf("el canal tiene que decir %q y dice %q", FraseTarjetaCaida, porque)
	}
}

// ── ayudas ────────────────────────────────────────────────────────────

// aireQueLlega cuenta los paquetes que salen del canal de verdad. Es lo que
// mide si el aire se quedó pegado: lo que el programa crea que está haciendo
// no vale aquí.
type aireQueLlega struct {
	mu       sync.Mutex
	paquetes int
}

func (q *aireQueLlega) contar(pc net.PacketConn) {
	buf := make([]byte, 4096)
	for {
		n, _, err := pc.ReadFrom(buf)
		if err != nil {
			return
		}
		if n == 0 {
			continue
		}
		q.mu.Lock()
		q.paquetes++
		q.mu.Unlock()
	}
}

func (q *aireQueLlega) cuantos() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.paquetes
}

// elEncoderDeSalida es el pid del ffmpeg que recibe los cuadros: el único que
// lee por TCP local, porque los decodificadores leen archivos. distinto es el
// pid de la vida anterior, para no volver a congelar al que ya está muerto.
func elEncoderDeSalida(t *testing.T, distinto string) string {
	t.Helper()
	var pid string
	esperar(t, 60*time.Second, func() bool {
		out, err := exec.Command("pgrep", "-f", "tcp://127.0.0.1").Output()
		if err != nil {
			return false
		}
		pids := strings.Fields(string(out))
		if len(pids) != 1 || pids[0] == distinto {
			return false
		}
		pid = pids[0]
		return true
	}, "no encontré el ffmpeg de salida para congelarlo")
	return pid
}

// congelar detiene el proceso sin matarlo. Es el escenario de F2-11: el
// encoder sigue vivo y no consume un solo cuadro.
func congelar(t *testing.T, pid string) {
	t.Helper()
	if out, err := exec.Command("kill", "-STOP", pid).CombinedOutput(); err != nil {
		t.Fatalf("no pude congelar el proceso %s: %v\n%s", pid, err, out)
	}
	// Si la prueba se cae antes de que el motor lo mate, que no quede un
	// ffmpeg congelado en la máquina de nadie.
	t.Cleanup(func() { _ = exec.Command("kill", "-KILL", pid).Run() })
}

// elIncidente devuelve el texto del último incidente de ese tipo.
func elIncidente(t *testing.T, a *App, tipo model.TipoIncidente) (string, bool) {
	t.Helper()
	incs, err := a.Store.Incident.List(context.Background(), a.ChannelID,
		a.Now().Add(-time.Hour), a.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("no pude leer la bitácora: %v", err)
	}
	texto, hay := "", false
	for _, inc := range incs {
		if inc.Kind == tipo.String() {
			texto, hay = inc.Detail, true
		}
	}
	return texto, hay
}
