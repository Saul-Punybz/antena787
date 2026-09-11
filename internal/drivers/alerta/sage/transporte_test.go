package sage

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// Ninguna prueba de este archivo abre un puerto serial ni un socket: el ENDEC
// es un tubo de mentira (io.Pipe) que la prueba llena y rompe cuando quiere, y
// el reloj y la espera se inyectan. Es el mismo patrón de inyección de
// internal/despierto, por la misma razón: una prueba de reconexión que duerma
// sesenta segundos de verdad no es una prueba.

// enlaceFalso es el ENDEC de mentira. Escribir mete bytes «por el cable» y
// Romper simula que se cayó el enlace.
type enlaceFalso struct {
	r       *io.PipeReader
	w       *io.PipeWriter
	cerrado chan struct{}
	una     sync.Once
}

func nuevoEnlaceFalso() *enlaceFalso {
	r, w := io.Pipe()
	return &enlaceFalso{r: r, w: w, cerrado: make(chan struct{})}
}

func (e *enlaceFalso) Read(p []byte) (int, error) { return e.r.Read(p) }

func (e *enlaceFalso) Close() error {
	e.una.Do(func() { close(e.cerrado) })
	return e.r.Close()
}

func (e *enlaceFalso) Escribir(t *testing.T, texto string) {
	t.Helper()
	if _, err := e.w.Write([]byte(texto)); err != nil {
		t.Fatalf("el tubo de mentira no aceptó los datos: %v", err)
	}
}

// Romper es el cable que alguien desenchufa.
func (e *enlaceFalso) Romper() { _ = e.w.CloseWithError(io.ErrUnexpectedEOF) }

func (e *enlaceFalso) SeCerro() bool {
	select {
	case <-e.cerrado:
		return true
	default:
		return false
	}
}

// esperaFalsa apunta cada espera y no duerme nada.
type esperaFalsa struct {
	mu      sync.Mutex
	esperas []time.Duration
}

func nuevaEsperaFalsa() *esperaFalsa { return &esperaFalsa{} }

func (e *esperaFalsa) dormir(_ context.Context, d time.Duration) {
	e.mu.Lock()
	e.esperas = append(e.esperas, d)
	e.mu.Unlock()
}

func (e *esperaFalsa) lista() []time.Duration {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]time.Duration(nil), e.esperas...)
}

// siguiente saca un evento del canal o falla: una prueba no se queda colgada.
func siguiente(t *testing.T, eventos <-chan Evento) Evento {
	t.Helper()
	select {
	case e, abierto := <-eventos:
		if !abierto {
			t.Fatal("el canal de eventos se cerró y esperaba un evento")
		}
		return e
	case <-time.After(5 * time.Second):
		t.Fatal("pasaron cinco segundos sin evento")
		return Evento{}
	}
}

// El camino serial de punta a punta: se abre el puerto, llega la prueba semanal
// del manual, y sale el Evento con su cabecera.
func TestSerialEntregaLaPruebaSemanal(t *testing.T) {
	enlace := nuevoEnlaceFalso()
	abiertos := make(chan struct {
		puerto  string
		baudios int
	}, 4)

	ctx, cancelar := context.WithCancel(context.Background())
	defer cancelar()

	eventos, err := Conectar(ctx, Config{
		Serial: Serial{Puerto: "COM2", Baudios: 9600},
		AbrirSerial: func(puerto string, baudios int) (Enlace, error) {
			abiertos <- struct {
				puerto  string
				baudios int
			}{puerto, baudios}
			return enlace, nil
		},
		Reloj:  relojFijo,
		Dormir: func(context.Context, time.Duration) {},
	})
	if err != nil {
		t.Fatalf("no conectó: %v", err)
	}

	if a := <-abiertos; a.puerto != "COM2" || a.baudios != 9600 {
		t.Fatalf("abrió %q a %d baudios", a.puerto, a.baudios)
	}

	enlace.Escribir(t, "local:ZCZC-EAS-RWT-006013+0015-1020638-SAGE    -\r\n")
	e := siguiente(t, eventos)
	if e.Tipo != TipoLocal || e.Cabecera == nil || e.Cabecera.Evento != "RWT" {
		t.Fatalf("llegó %+v", e)
	}
	if e.Clase != ClasePruebaSemanal {
		t.Fatalf("clase %q", e.Clase)
	}
}

// Los baudios no se adivinan pero sí tienen un valor por defecto, y tiene que
// ser 9600: es el de COM2 y COM6, los dos puertos de baudios fijo del ENDEC más
// probables para un device type de estado (manual §12.1).
func TestBaudiosPorDefecto(t *testing.T) {
	listo := make(chan int, 1)
	ctx, cancelar := context.WithCancel(context.Background())
	defer cancelar()
	if _, err := Conectar(ctx, Config{
		Serial: Serial{Puerto: "/dev/ttyUSB0"},
		AbrirSerial: func(_ string, baudios int) (Enlace, error) {
			listo <- baudios
			return nuevoEnlaceFalso(), nil
		},
		Reloj:  relojFijo,
		Dormir: func(context.Context, time.Duration) {},
	}); err != nil {
		t.Fatalf("no conectó: %v", err)
	}
	if b := <-listo; b != 9600 {
		t.Fatalf("abrió a %d baudios y esperaba 9600", b)
	}
}

// El camino de red: la interfaz de automatización del 3644, el mismo protocolo
// por un socket.
func TestTCPEntregaLoMismoQueElSerial(t *testing.T) {
	enlace := nuevoEnlaceFalso()
	marcados := make(chan string, 4)

	ctx, cancelar := context.WithCancel(context.Background())
	defer cancelar()

	eventos, err := Conectar(ctx, Config{
		TCP: TCP{Direccion: "10.0.0.9:10000"},
		Marcar: func(_ context.Context, red, direccion string) (Enlace, error) {
			marcados <- red + " " + direccion
			return enlace, nil
		},
		Reloj:  relojFijo,
		Dormir: func(context.Context, time.Duration) {},
	})
	if err != nil {
		t.Fatalf("no conectó: %v", err)
	}
	if m := <-marcados; m != "tcp 10.0.0.9:10000" {
		t.Fatalf("marcó %q", m)
	}

	enlace.Escribir(t, "local:ZCZC-WXR-TOR-072127+0030-2531830-KWNS/NWS-\r\n")
	e := siguiente(t, eventos)
	if e.Clase != ClaseReal || !e.Interrumpe() {
		t.Fatalf("llegó %+v", e)
	}
}

// El primer intento es sincrónico a propósito: si el cable no es ese, la
// persona se entera en el momento en que elige el driver, no mirando una
// pantalla que no cambia. Y el error tiene que decirle qué mirar.
func TestPrimerIntentoFallidoSeDiceEnLaCara(t *testing.T) {
	_, err := Conectar(context.Background(), Config{
		Serial:      Serial{Puerto: "COM9", Baudios: 1200},
		AbrirSerial: func(string, int) (Enlace, error) { return nil, errors.New("no such file or directory") },
		Reloj:       relojFijo,
		Dormir:      func(context.Context, time.Duration) {},
	})
	if err == nil {
		t.Fatal("el puerto no existía y Conectar no dio error")
	}
	for _, quiero := range []string{"COM9", "1200", "DECODER", "baudios"} {
		if !strings.Contains(err.Error(), quiero) {
			t.Fatalf("el error no menciona %q y no le sirve a nadie: %q", quiero, err)
		}
	}

	_, err = Conectar(context.Background(), Config{
		TCP:    TCP{Direccion: "10.0.0.9:10000"},
		Marcar: func(context.Context, string, string) (Enlace, error) { return nil, errors.New("connection refused") },
		Reloj:  relojFijo,
		Dormir: func(context.Context, time.Duration) {},
	})
	if err == nil {
		t.Fatal("el socket no contestó y Conectar no dio error")
	}
	// Lo primero que hay que mirar en un 3644 no es el cable: es que la
	// interfaz de automatización viene apagada de fábrica.
	if !strings.Contains(err.Error(), "MENU.NETWORK.AUTOMATION") {
		t.Fatalf("el error no dice que la automatización viene apagada de fábrica: %q", err)
	}
}

// Sin camino, y con los dos caminos, el error es de una persona y se dice como
// se le dice a una persona.
func TestConfiguracionQueNoSePuedeObedecer(t *testing.T) {
	if _, err := Conectar(context.Background(), Config{}); err == nil {
		t.Fatal("sin puerto ni dirección tenía que dar error")
	} else if !strings.Contains(err.Error(), "por dónde está conectado") {
		t.Fatalf("el error no pregunta lo que hay que preguntar: %q", err)
	}

	_, err := Conectar(context.Background(), Config{
		Serial: Serial{Puerto: "COM2", Baudios: 9600},
		TCP:    TCP{Direccion: "10.0.0.9:10000"},
	})
	if err == nil {
		t.Fatal("con los dos caminos a la vez tenía que dar error")
	}
	if !strings.Contains(err.Error(), "dos veces") {
		t.Fatalf("el error no explica que cada camino se abre aparte: %q", err)
	}
}

// La reconexión: espera progresiva 1, 2, 4… con tope de 60 s, para siempre y
// sin que nadie reinicie nada (regla 5 del contrato de drivers).
func TestReconectaConEsperaProgresivaHastaSesentaSegundos(t *testing.T) {
	primero := nuevoEnlaceFalso()
	segundo := nuevoEnlaceFalso()

	var mu sync.Mutex
	intentos := 0
	abrir := func(string, int) (Enlace, error) {
		mu.Lock()
		defer mu.Unlock()
		intentos++
		switch {
		case intentos == 1:
			return primero, nil
		case intentos <= 8: // siete fallos seguidos: 1, 2, 4, 8, 16, 32, 60
			return nil, errors.New("device not configured")
		default:
			return segundo, nil
		}
	}

	espera := nuevaEsperaFalsa()
	ctx, cancelar := context.WithCancel(context.Background())
	defer cancelar()

	eventos, err := Conectar(ctx, Config{
		Serial:      Serial{Puerto: "COM2", Baudios: 9600},
		AbrirSerial: abrir,
		Reloj:       relojFijo,
		Dormir:      espera.dormir,
	})
	if err != nil {
		t.Fatalf("no conectó: %v", err)
	}

	primero.Escribir(t, "local:ZCZC-EAS-RWT-006013+0015-1020638-SAGE    -\r\n")
	if e := siguiente(t, eventos); e.Tipo != TipoLocal {
		t.Fatalf("antes de la caída llegó %+v", e)
	}

	primero.Romper()

	if e := siguiente(t, eventos); e.Tipo != TipoEnlaceCaido {
		t.Fatalf("se cayó el enlace y llegó %+v", e)
	}
	if e := siguiente(t, eventos); e.Tipo != TipoEnlaceVuelto {
		t.Fatalf("volvió el enlace y llegó %+v", e)
	}
	if !primero.SeCerro() {
		t.Fatal("el enlace roto no se cerró: así se filtra un descriptor por caída")
	}

	quiero := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second,
		16 * time.Second, 32 * time.Second, 60 * time.Second, 60 * time.Second}
	hubo := espera.lista()
	if len(hubo) != len(quiero) {
		t.Fatalf("esperó %v y esperaba %v", hubo, quiero)
	}
	for i := range quiero {
		if hubo[i] != quiero[i] {
			t.Fatalf("la espera %d fue %v y tenía que ser %v (progresión completa: %v)", i+1, hubo[i], quiero[i], hubo)
		}
	}

	// Y por el enlace nuevo sigue llegando todo, con un analizador limpio: lo
	// que quedara a medias en el cable roto no se pega con lo primero del
	// nuevo, que es como se fabrica una cabecera que nadie emitió.
	segundo.Escribir(t, "local:ZCZC-EAS-RMT-001001-001005+0015-0390352-SAGE    -\r\n")
	e := siguiente(t, eventos)
	if e.Tipo != TipoLocal || e.Clase != ClasePruebaMensual {
		t.Fatalf("después de reconectar llegó %+v", e)
	}
}

// Media cabecera en el cable roto no se pega con la primera línea del cable
// nuevo. Es la prueba de que el analizador se tira en cada reconexión.
func TestLoQueQuedoAMediasNoSePegaConElEnlaceNuevo(t *testing.T) {
	primero := nuevoEnlaceFalso()
	segundo := nuevoEnlaceFalso()
	var mu sync.Mutex
	intentos := 0
	abrir := func(string, int) (Enlace, error) {
		mu.Lock()
		defer mu.Unlock()
		intentos++
		if intentos == 1 {
			return primero, nil
		}
		return segundo, nil
	}

	ctx, cancelar := context.WithCancel(context.Background())
	defer cancelar()
	eventos, err := Conectar(ctx, Config{
		Serial:      Serial{Puerto: "COM2", Baudios: 9600},
		AbrirSerial: abrir,
		Reloj:       relojFijo,
		Dormir:      func(context.Context, time.Duration) {},
	})
	if err != nil {
		t.Fatalf("no conectó: %v", err)
	}

	// Se corta el cable a media cabecera. Lo que quedó se entrega tal cual
	// —como Evento sin cabecera, porque está a medias— y no espera a nada.
	primero.Escribir(t, "local:ZCZC-EAS-RWT-0060")
	primero.Romper()

	e := siguiente(t, eventos)
	if e.Tipo != TipoLocal || e.Cabecera != nil {
		t.Fatalf("la media cabecera tenía que salir sin parsear: %+v", e)
	}
	if e := siguiente(t, eventos); e.Tipo != TipoEnlaceCaido {
		t.Fatalf("esperaba el aviso de caída y llegó %+v", e)
	}
	if e := siguiente(t, eventos); e.Tipo != TipoEnlaceVuelto {
		t.Fatalf("esperaba el aviso de vuelta y llegó %+v", e)
	}

	// Y el resto de aquella cabecera, llegando por el cable nuevo, no puede
	// completarla: sería una alerta que nadie emitió.
	segundo.Escribir(t, "13+0015-1020638-SAGE    -\r\nlocal:ZCZC-WXR-TOR-072127+0030-2531830-KWNS/NWS-\r\n")
	for {
		e := siguiente(t, eventos)
		if e.Tipo == TipoLocal {
			if e.Cabecera == nil || e.Cabecera.Evento != "TOR" {
				t.Fatalf("se fabricó una alerta pegando dos cables: %+v", e)
			}
			return
		}
	}
}

// Cancelar el contexto cierra el enlace y el canal, y no deja goroutines
// esperando a un ENDEC que puede estar callado semanas.
func TestCancelarElContextoCierraTodo(t *testing.T) {
	enlace := nuevoEnlaceFalso()
	ctx, cancelar := context.WithCancel(context.Background())
	eventos, err := Conectar(ctx, Config{
		Serial:      Serial{Puerto: "COM2", Baudios: 9600},
		AbrirSerial: func(string, int) (Enlace, error) { return enlace, nil },
		Reloj:       relojFijo,
		Dormir:      func(context.Context, time.Duration) {},
	})
	if err != nil {
		t.Fatalf("no conectó: %v", err)
	}

	cancelar()

	plazo := time.After(5 * time.Second)
	for {
		select {
		case _, abierto := <-eventos:
			if !abierto {
				if !enlace.SeCerro() {
					t.Fatal("se cerró el canal y el enlace se quedó abierto")
				}
				return
			}
		case <-plazo:
			t.Fatal("cancelé el contexto y el canal no se cerró en cinco segundos")
		}
	}
}

// Una alerta no se tira nunca por falta de sitio en el canal, ni aunque el
// canal sea de uno y lleguen tres seguidas. Es lo contrario del detector de
// silencio y negro, y la diferencia es que aquí nadie sale al aire esperándonos.
func TestNingunaAlertaSeTiraPorFaltaDeSitio(t *testing.T) {
	enlace := nuevoEnlaceFalso()
	ctx, cancelar := context.WithCancel(context.Background())
	defer cancelar()
	eventos, err := Conectar(ctx, Config{
		Serial:      Serial{Puerto: "COM2", Baudios: 9600},
		AbrirSerial: func(string, int) (Enlace, error) { return enlace, nil },
		Reloj:       relojFijo,
		Dormir:      func(context.Context, time.Duration) {},
		Pendientes:  1,
	})
	if err != nil {
		t.Fatalf("no conectó: %v", err)
	}

	go func() {
		enlace.Escribir(t, strings.Repeat("local:ZCZC-EAS-RWT-006013+0015-1020638-SAGE    -\r\n", 3))
	}()

	for i := 0; i < 3; i++ {
		if e := siguiente(t, eventos); e.Tipo != TipoLocal {
			t.Fatalf("la alerta %d llegó como %+v", i+1, e)
		}
	}
}
