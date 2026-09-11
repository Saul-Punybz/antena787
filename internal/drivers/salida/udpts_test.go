package salida

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"antena787/internal/engine"
	"antena787/internal/model"
)

// Pruebas del driver de salida al multiplexor (T2 de F2): que lo que la
// persona escribe una vez se traduzca a lo que un multiplexor exige, y que lo
// que un multiplexor no perdonaría se diga antes de encender nada.

// registro de mentira: se queda con lo último que le dijo el driver.
// registro es el doble de prueba de salida.Registro. Lleva candado porque
// el vigilante escribe desde su propia goroutine mientras la prueba lee, que
// es justo lo que el contrato advierte.
type registro struct {
	mu         sync.Mutex
	estado     string
	reintentos int
	error      string
	veces      int
	avisa      chan struct{}
}

func (r *registro) SetConnection(_ context.Context, _ int64, estado string, reintentos int, ultimo string) error {
	r.mu.Lock()
	r.estado, r.reintentos, r.error = estado, reintentos, ultimo
	r.veces++
	avisa := r.avisa
	r.mu.Unlock()
	if avisa != nil {
		select {
		case avisa <- struct{}{}:
		default:
		}
	}
	return nil
}

// lee devuelve lo último que quedó escrito, sin carreras.
func (r *registro) lee() (estado string, reintentos int, ultimo string, veces int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.estado, r.reintentos, r.error, r.veces
}

// F2-46 — la salida al multiplexor sale con lo que el multiplexor exige, y lo
// que la persona no escribió cae en los valores de ejemplo.
func TestLaSalidaAlMultiplexorTraeLoQueElMultiplexorExige(t *testing.T) {
	d, err := Para(model.Output{ID: 1, Name: "transmisor", Driver: DriverUDPTS,
		Params: `{"destino": "192.168.1.50:1234"}`}, nil)
	if err != nil {
		t.Fatalf("el driver no abrió: %v", err)
	}
	out, err := d.Abrir(engine.CAtv)
	if err != nil {
		t.Fatalf("no se pudo abrir la salida: %v", err)
	}
	quiere := engine.Output{
		Name: "transmisor", Kind: "mpeg2-ts", UDP: "udp://192.168.1.50:1234",
		VideoKbs: VideoKbsPorDefecto, MuxKbs: MuxKbsPorDefecto,
		PIDVideo: PIDVideoPorDefecto, PIDAudio: PIDAudioPorDefecto, PIDPMT: PIDPMTPorDefecto,
		Programa: ProgramaPorDefecto, TSID: TSIDPorDefecto,
		PCRms: engine.PCRmsPorDefecto, Audio: AudioPorDefecto,
		AudioKbs: engine.AudioKbsPorDefecto,
		PktSize:  engine.PktSizePorDefecto,
	}
	if out != quiere {
		t.Fatalf("la salida es\n  %+v\ny tenía que ser\n  %+v", out, quiere)
	}
	if !strings.Contains(d.Descripcion(), "al receptor 192.168.1.50:1234") {
		t.Fatalf("la descripción no dice a dónde va: %q", d.Descripcion())
	}
}

// F2-114 — el mismo driver manda a un grupo multicast con su TTL, y no hay que
// cambiar de driver para volver a unicast.
func TestElMismoDriverMandaAUnGrupoMulticastConSuTTL(t *testing.T) {
	d, err := Para(model.Output{Name: "transmisor", Driver: DriverUDPTS,
		Params: `{"destino": "239.10.10.10:5000", "ttl": 4, "pid_video": 256, "pid_audio": 257,
			"pid_pmt": 4096, "program": 7, "tsid": 99, "pcr_ms": 30, "audio": "ac3",
			"bitrate_mux_kbs": 12000, "bitrate_video_kbs": 9000}`}, nil)
	if err != nil {
		t.Fatalf("el driver no abrió: %v", err)
	}
	out, _ := d.Abrir(engine.CAtv)
	quiere := engine.Output{
		Name: "transmisor", Kind: "mpeg2-ts", UDP: "udp://239.10.10.10:5000",
		VideoKbs: 9000, MuxKbs: 12000,
		PIDVideo: 256, PIDAudio: 257, PIDPMT: 4096, Programa: 7, TSID: 99,
		PCRms: 30, Audio: "ac3", AudioKbs: engine.AudioKbsPorDefecto,
		TTL: 4, PktSize: engine.PktSizePorDefecto,
	}
	if out != quiere {
		t.Fatalf("la salida a un grupo es\n  %+v\ny tenía que ser\n  %+v", out, quiere)
	}
	if !strings.Contains(d.Descripcion(), "al grupo 239.10.10.10:5000, 4 salto(s)") {
		t.Fatalf("la descripción no dice que es un grupo con su TTL: %q", d.Descripcion())
	}

	// A un grupo sin TTL escrito se le pone uno: no sale de la propia red, que
	// es lo que nadie lamenta.
	d, err = Para(model.Output{Name: "x", Driver: DriverUDPTS, Params: `{"destino": "239.1.1.1:1234"}`}, nil)
	if err != nil {
		t.Fatalf("el driver no abrió: %v", err)
	}
	if out, _ := d.Abrir(engine.CAtv); out.TTL != TTLDeGrupoPorDefecto {
		t.Fatalf("el grupo salió con TTL %d y tenía que salir con %d", out.TTL, TTLDeGrupoPorDefecto)
	}

	// Y unicast sigue siendo el mismo driver, sin TTL.
	d, _ = Para(model.Output{Name: "x", Driver: DriverUDPTS, Params: `{"destino": "10.0.0.9:1234"}`}, nil)
	if out, _ := d.Abrir(engine.CAtv); out.TTL != 0 {
		t.Fatalf("unicast salió con TTL %d y no tenía que llevar ninguno", out.TTL)
	}
}

// Lo que un multiplexor no perdonaría se dice antes de encender nada, y se
// dice en palabras claras: nada de «parámetro inválido».
func TestLoQueElMultiplexorNoPerdonaSeDiceEnCristiano(t *testing.T) {
	casos := []struct {
		nombre string
		params string
		espera string // un trozo del texto que tiene que salir
	}{
		{"sin destino", `{}`, "no me has dicho a qué dirección"},
		{"destino sin puerto", `{"destino": "192.168.1.50"}`, "hace falta la dirección y el puerto"},
		{"puerto imposible", `{"destino": "192.168.1.50:99999"}`, "no existe"},
		{"PCR demasiado tarde", `{"destino": "10.0.0.1:1234", "pcr_ms": 60}`,
			"cada 40 ms o menos"},
		{"PID de relleno", `{"destino": "10.0.0.1:1234", "pid_video": 8191}`,
			"8191 son los paquetes de relleno"},
		{"PID de tabla", `{"destino": "10.0.0.1:1234", "pid_audio": 20}`, "va del 32 al 8190"},
		{"dos flujos en el mismo PID", `{"destino": "10.0.0.1:1234", "pid_video": 512, "pid_audio": 512}`,
			"no pueden ir en el mismo PID"},
		{"el video no cabe en el TS", `{"destino": "10.0.0.1:1234", "bitrate_video_kbs": 9000, "bitrate_mux_kbs": 9000}`,
			"el audio y las tablas no caben"},
		{"audio que no existe", `{"destino": "10.0.0.1:1234", "audio": "opus"}`,
			"MPEG capa II (mp2) o AC-3"},
		{"video que no acepta", `{"destino": "10.0.0.1:1234", "video": "h264"}`,
			"espera video MPEG-2"},
		{"TTL imposible", `{"destino": "239.1.1.1:1234", "ttl": 999}`, "van de 1 a 255"},
		{"programa fuera de rango", `{"destino": "10.0.0.1:1234", "program": 70000}`,
			"del 1 al 65535"},
		{"JSON roto", `{"destino":`, "no entiendo lo que está guardado"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			_, err := Para(model.Output{Name: "transmisor", Driver: DriverUDPTS, Params: c.params}, nil)
			if err == nil {
				t.Fatalf("%s pasó la validación y no tenía que pasar", c.nombre)
			}
			if !strings.Contains(err.Error(), c.espera) {
				t.Fatalf("el error dice %q y tenía que decir algo con %q", err, c.espera)
			}
		})
	}
}

// Un driver que esta versión no sabe abrir se dice por su nombre, y no se
// queda callado.
func TestUnDriverQueNoExisteTodaviaSeDice(t *testing.T) {
	if _, err := Para(model.Output{Name: "YouTube", Driver: "rtmp"}, nil); err == nil ||
		!strings.Contains(err.Error(), "todavía no sé mandar la señal por \"rtmp\"") {
		t.Fatalf("el error de un driver que no existe es %v", err)
	}
	if _, err := Para(model.Output{Name: "nada", Driver: ""}, nil); err == nil ||
		!strings.Contains(err.Error(), "no dice a dónde") {
		t.Fatalf("el error de una salida sin driver es %v", err)
	}
	// Y `udp-ts` está entre los que se ofrecen desde F2 (F2-50).
	var hay bool
	for _, f := range Disponibles() {
		if f.Driver == DriverUDPTS && f.Nombre != "" && f.Explicacion != "" {
			hay = true
		}
	}
	if !hay {
		t.Fatal("udp-ts no está entre los drivers de salida que se pueden ofrecer")
	}
}

// La salida a un archivo crea su carpeta sola: nadie tiene que acordarse.
func TestLaSalidaAArchivoCreaSuCarpeta(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "aire", "nuevo", "salida.ts")
	// La ruta va por el codificador de JSON, no pegada a mano: en Windows
	// `C:\Users\...` lleva barras invertidas y `\U` es un escape inválido.
	params, err := json.Marshal(map[string]string{"ruta": ruta})
	if err != nil {
		t.Fatal(err)
	}
	d, err := Para(model.Output{Name: "archivo", Driver: DriverArchivo,
		Params: string(params)}, nil)
	if err != nil {
		t.Fatalf("el driver no abrió: %v", err)
	}
	out, err := d.Abrir(engine.CAtv)
	if err != nil {
		t.Fatalf("no se pudo abrir la salida a archivo: %v", err)
	}
	if out.File != ruta || out.Kind != "mpeg2-ts" || out.UDP != "" {
		t.Fatalf("la salida a archivo es %+v", out)
	}
	if _, err := Para(model.Output{Name: "archivo", Driver: DriverArchivo, Params: `{}`}, nil); err == nil {
		t.Fatal("una salida a archivo sin ruta pasó la validación")
	}
}

// F2-47 — cada salida va a su propio objetivo de volumen, no a uno compartido.
func TestCadaSalidaVaASuPropioVolumen(t *testing.T) {
	// La biblioteca está normalizada a −24 LKFS: al transmisor sale tal cual y
	// a internet ocho decibelios más arriba.
	if g := Ganancia(-24, model.Output{TargetLoudness: -24}); g != 0 {
		t.Fatalf("al transmisor le tocan 0 dB y le tocaron %.1f", g)
	}
	if g := Ganancia(-24, model.Output{TargetLoudness: -16}); g != 8 {
		t.Fatalf("a internet le tocan +8 dB y le tocaron %.1f", g)
	}
	if g := Ganancia(-24, model.Output{}); g != 0 {
		t.Fatalf("una salida sin objetivo sale como está normalizada, y le tocaron %.1f", g)
	}
}

// F2-48/F2-49 — el vigilante deja escrito cómo le va a cada salida: cuándo
// está saliendo, cuántas veces se reintentó y con qué error, con espera
// progresiva de 1, 2, 4… y tope de un minuto.
func TestElVigilanteDejaEscritoComoLeVaALaSalida(t *testing.T) {
	if d := EsperaDe(1); d != time.Second {
		t.Fatalf("el primer reintento espera %s y tenía que esperar 1 s", d)
	}
	if d := EsperaDe(3); d != 4*time.Second {
		t.Fatalf("el tercer reintento espera %s y tenía que esperar 4 s", d)
	}
	if d := EsperaDe(20); d != EsperaMax {
		t.Fatalf("el reintento veinte espera %s y el tope es %s", d, EsperaMax)
	}

	reg := &registro{avisa: make(chan struct{}, 4)}
	d, err := Para(model.Output{ID: 7, Name: "transmisor", Driver: DriverUDPTS,
		Params: `{"destino": "127.0.0.1:1234"}`}, reg)
	if err != nil {
		t.Fatalf("el driver no abrió: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	listo := make(chan error, 2)
	go d.Vigilar(ctx, model.Output{ID: 7}, listo)

	listo <- nil
	<-reg.avisa
	if estado, reintentos, _, _ := reg.lee(); estado != Conectada || reintentos != 0 {
		t.Fatalf("con el encoder encendido la salida quedó en %q con %d reintentos", estado, reintentos)
	}
	listo <- errSuelto("el cable de red está desconectado")
	<-reg.avisa
	if estado, reintentos, ultimo, _ := reg.lee(); estado != Reintentando || reintentos != 1 ||
		!strings.Contains(ultimo, "cable de red") {
		t.Fatalf("tras la caída la salida quedó en %q, %d reintentos, error %q",
			estado, reintentos, ultimo)
	}
	cancel()
	esperar(t, func() bool { estado, _, _, _ := reg.lee(); return estado == Apagada }, "la salida no quedó apagada al parar el motor")
}

// errSuelto es un error de una línea, para las pruebas.
type errSuelto string

func (e errSuelto) Error() string { return string(e) }

// esperar reintenta hasta que la condición se cumpla o se agote el plazo.
func esperar(t *testing.T, cond func() bool, queja string) {
	t.Helper()
	limite := time.Now().Add(3 * time.Second)
	for time.Now().Before(limite) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(queja)
}

// TestElSonidoSePuedeCambiarYTieneLimites: el bitrate del audio estuvo
// cableado en 192 hasta el 11 de septiembre de 2026, y no debía estarlo —
// siete de ocho sistemas de playout libres lo exponen y CAtv emite a 128.
// Esta prueba sostiene las tres cosas: que el valor de fábrica sigue siendo
// 192, que se puede cambiar, y que no se acepta cualquier número.
func TestElSonidoSePuedeCambiarYTieneLimites(t *testing.T) {
	abrir := func(kbs int) (engine.Output, error) {
		p := ParamsUDPTS{Destino: "239.1.1.1:1234", BitrateVideoKbs: 4800,
			BitrateMuxKbs: 6000, Audio: "mp2", Video: "mpeg2", BitrateAudioKbs: kbs}
		datos, _ := json.Marshal(p)
		d, err := nuevoUDPTS(model.Output{Name: "t", Driver: DriverUDPTS, Params: string(datos)}, nil)
		if err != nil {
			return engine.Output{}, err
		}
		return d.Abrir(engine.CAtv)
	}

	if out, err := abrir(0); err != nil || out.AudioKbs != engine.AudioKbsPorDefecto {
		t.Fatalf("sin decir nada el sonido tiene que quedar en %d y quedó en %d (%v)",
			engine.AudioKbsPorDefecto, out.AudioKbs, err)
	}
	// El número de CAtv, que es el caso que motivó todo esto.
	if out, err := abrir(128); err != nil || out.AudioKbs != 128 {
		t.Fatalf("128 es lo que emite CAtv y tiene que poder ponerse; quedó en %d (%v)", out.AudioKbs, err)
	}
	for _, malo := range []int{32, 512} {
		if _, err := abrir(malo); err == nil {
			t.Fatalf("%d kb/s está fuera del rango de %d a %d y se aceptó",
				malo, engine.AudioKbsMinimo, engine.AudioKbsMaximo)
		}
	}
}
