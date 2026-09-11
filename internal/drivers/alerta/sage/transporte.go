// transporte.go es el caño: abre el camino al ENDEC, lee bytes, se los da al
// analizador y entrega Eventos por un canal. Y se reconecta solo, porque un
// enlace de 24/7 se cae siempre (regla 5 del contrato de drivers,
// docs/drivers/README.md: espera progresiva 1, 2, 4… con tope de 60 s).
//
// Dos caminos, el mismo protocolo por dentro (ver sage.go):
//
//   - **serial**, para el 1822 y el 3644. Los baudios no se eligen: cada
//     puerto DB-9 del ENDEC los tiene fijos por rol —COM2/COM6 a 9600,
//     COM4/COM5 a 1200, COM3 y «Computer» variables (manual §12.1)—, así que
//     el número lo dice el ingeniero de la estación y el driver lo obedece.
//   - **TCP**, solo para el 3644: la interfaz de automatización sirve el mismo
//     protocolo por red en un puerto configurable, y **viene apagada de
//     fábrica**. Si el socket rechaza la conexión, lo primero que hay que
//     mirar no es el cable, es `MENU.NETWORK.AUTOMATION`.
//
// El sistema operativo no entra aquí a la fuerza: AbrirSerial y Marcar son
// campos inyectables, igual que el Lanzador de internal/despierto, así que
// todas las pruebas de este paquete corren sin un puerto y sin un socket.
//
// **Trampa de go.bug.st/serial, comprobada el 11 de septiembre de 2026:** el
// paquete raíz es Go puro y compila con CGO_ENABLED=0 en los tres sistemas,
// pero su subpaquete `enumerator/` —el que lista los puertos disponibles—
// **enlaza CoreFoundation e IOKit en macOS con `import "C"`**
// (`enumerator/usb_darwin.go`). Importarlo rompería el ADR 0002, así que aquí
// no se importa y **este driver no enumera puertos**: el nombre del puerto lo
// dice quien instala. Cuando el asistente necesite ofrecer la lista de puertos
// del paso 4, eso es un escaneo aparte —leer /dev y el registro de Windows— y
// no una dependencia nueva.
package sage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"go.bug.st/serial"
)

// Enlace es lo poco que hace falta de un puerto serial o de un socket: leer y
// cerrar. No se escribe nunca —este driver no le habla al ENDEC (ADR 0010)— y
// por eso la interfaz no tiene Write: lo que no se puede hacer, no se ofrece.
type Enlace interface {
	io.Reader
	io.Closer
}

// AbrirSerial abre un puerto serial. Nil vale por el del sistema.
type AbrirSerial func(puerto string, baudios int) (Enlace, error)

// Marcar abre una conexión de red. Nil vale por net.Dialer.
type Marcar func(ctx context.Context, red, direccion string) (Enlace, error)

// Serial es el camino por cable. Puerto vacío significa «no hay camino serial».
type Serial struct {
	// Puerto es el nombre del sistema: COM3 en Windows, /dev/ttyUSB0 en Linux,
	// /dev/cu.usbserial-… en macOS.
	Puerto string
	// Baudios los dice el puerto del ENDEC al que se enchufó el cable. 0 usa
	// 9600, que es el de COM2 y COM6, los dos puertos de baudios fijo más
	// probables para un device type de estado.
	Baudios int
}

// TCP es el camino por red: la interfaz de automatización del 3644.
// Direccion vacía significa «no hay camino de red».
type TCP struct {
	// Direccion es host:puerto. El puerto sale de MENU.NETWORK.PORT BASE del
	// ENDEC, no hay uno de fábrica que se pueda suponer.
	Direccion string
}

// Config es todo lo que Conectar necesita.
type Config struct {
	// Serial y TCP: exactamente uno de los dos. Un ENDEC cableado por los dos
	// caminos se abre dos veces y se cruza, y **cruzarlos es trabajo de T8**,
	// no de este paquete: aquí un Conectar es un camino, para que cuando uno
	// se caiga se sepa cuál.
	Serial Serial
	TCP    TCP

	// AbrirSerial y Marcar: nil usa el sistema. Existen para las pruebas.
	AbrirSerial AbrirSerial
	Marcar      Marcar

	// Reloj: nil usa time.Now. El año del JJJHHMM depende de él.
	Reloj func() time.Time

	// Dormir es la espera entre reintentos; nil espera de verdad. Una prueba de
	// reconexión que duerma sesenta segundos no es una prueba.
	Dormir func(ctx context.Context, d time.Duration)

	// TopeDeEspera es el techo de la espera progresiva. 0 usa 60 s (§9 paso 5).
	TopeDeEspera time.Duration

	// Pendientes es el tamaño del canal de eventos. 0 usa 64. Una alerta nunca
	// se tira por falta de sitio: si el canal se llena, el lector se bloquea
	// hasta que alguien lea o hasta que se cancele el contexto. Es lo contrario
	// del detector de silencio y negro (internal/engine/detector.go), que sí
	// tira estados, y la diferencia es que allí el aire espera y aquí no:
	// nadie sale al aire desde este canal.
	Pendientes int
}

const (
	baudiosPorDefecto  = 9600
	esperaInicial      = time.Second
	topePorDefecto     = 60 * time.Second
	pendientesPorDefec = 64
)

// Conectar abre el camino al ENDEC y devuelve el canal por el que van saliendo
// los Eventos.
//
// **El primer intento es sincrónico a propósito.** Si el puerto no existe o el
// socket no contesta, Conectar devuelve el error ahí mismo, en palabras claras, para
// que la prueba de diez segundos del asistente pueda decir «este cable no» en
// vez de dejar a alguien mirando una pantalla que no cambia (regla 4 del
// contrato de drivers). A partir de ahí, cada caída se reconecta sola con
// espera progresiva y sin molestar a nadie: lo que sale por el canal son dos
// eventos, TipoEnlaceCaido y TipoEnlaceVuelto, y quien decide qué hacer con
// ellos es T8.
//
// El canal se cierra cuando se cancela el contexto, y solo entonces. Un ENDEC
// no «termina»: mientras el canal esté encendido, este driver sigue intentando.
func Conectar(ctx context.Context, cfg Config) (<-chan Evento, error) {
	abrir, err := caminoDe(cfg)
	if err != nil {
		return nil, err
	}
	if cfg.Reloj == nil {
		cfg.Reloj = time.Now
	}
	if cfg.Dormir == nil {
		cfg.Dormir = dormirDeVerdad
	}
	if cfg.TopeDeEspera <= 0 {
		cfg.TopeDeEspera = topePorDefecto
	}
	if cfg.Pendientes <= 0 {
		cfg.Pendientes = pendientesPorDefec
	}

	enlace, err := abrir(ctx)
	if err != nil {
		return nil, err
	}

	eventos := make(chan Evento, cfg.Pendientes)
	go leerParaSiempre(ctx, cfg, abrir, enlace, eventos)
	return eventos, nil
}

// caminoDe valida la configuración y devuelve la función que abre el enlace.
// Los dos errores posibles son de una persona, no del equipo, y se dicen como
// se le dirían a una persona.
func caminoDe(cfg Config) (func(context.Context) (Enlace, error), error) {
	haySerial := cfg.Serial.Puerto != ""
	hayRed := cfg.TCP.Direccion != ""
	switch {
	case haySerial && hayRed:
		return nil, errors.New("dime un puerto serial o una dirección de red, no las dos: un ENDEC cableado por los dos caminos se abre dos veces, una por camino, para saber cuál se cayó")
	case !haySerial && !hayRed:
		return nil, errors.New("no me dijiste por dónde está conectado el ENDEC: hace falta un puerto serial (con sus baudios) o la dirección de la interfaz de automatización del 3644")
	case haySerial:
		baudios := cfg.Serial.Baudios
		if baudios <= 0 {
			baudios = baudiosPorDefecto
		}
		abrir := cfg.AbrirSerial
		if abrir == nil {
			abrir = abrirSerialDelSistema
		}
		puerto := cfg.Serial.Puerto
		return func(context.Context) (Enlace, error) {
			e, err := abrir(puerto, baudios)
			if err != nil {
				return nil, fmt.Errorf("no pude abrir %s a %d baudios: %w — comprueba que el cable va a un puerto del ENDEC puesto en DECODER (MENU.DEVICES.PORT.DEVICE TYPE) y que los baudios son los de ese puerto", puerto, baudios, err)
			}
			return e, nil
		}, nil
	default:
		marcar := cfg.Marcar
		if marcar == nil {
			marcar = marcarDelSistema
		}
		direccion := cfg.TCP.Direccion
		return func(ctx context.Context) (Enlace, error) {
			e, err := marcar(ctx, "tcp", direccion)
			if err != nil {
				return nil, fmt.Errorf("no pude conectar con el ENDEC en %s: %w — la interfaz de automatización del 3644 viene apagada de fábrica; hay que encenderla en MENU.NETWORK.AUTOMATION y el puerto es el de MENU.NETWORK.PORT BASE", direccion, err)
			}
			return e, nil
		}, nil
	}
}

// leerParaSiempre es el bucle de vida del driver: lee hasta que el enlace se
// rompe, avisa, espera lo que toque y vuelve a abrir. No termina nunca por su
// cuenta; solo lo para el contexto.
func leerParaSiempre(ctx context.Context, cfg Config, abrir func(context.Context) (Enlace, error), enlace Enlace, eventos chan<- Evento) {
	defer close(eventos)

	espera := esperaInicial
	for {
		// Un analizador nuevo por conexión: lo que quedara a medias en el
		// enlace roto no se pega con lo primero que llegue por el nuevo, que
		// es como se fabrica una cabecera que nadie emitió.
		an := NuevoAnalizador(cfg.Reloj)
		motivo := bombear(ctx, an, enlace, eventos)
		_ = enlace.Close()
		if ctx.Err() != nil {
			return
		}

		if !entregar(ctx, eventos, Evento{
			Tipo:     TipoEnlaceCaido,
			Crudo:    motivoEnPalabras(motivo),
			Texto:    motivoEnPalabras(motivo),
			Recibido: cfg.Reloj(),
		}) {
			return
		}

		// Espera progresiva 1, 2, 4… con tope. Se reintenta para siempre: un
		// ENDEC desconectado un fin de semana entero tiene que volver solo el
		// lunes, sin que nadie reinicie nada.
		for {
			cfg.Dormir(ctx, espera)
			if ctx.Err() != nil {
				return
			}
			nuevo, err := abrir(ctx)
			if err == nil {
				enlace = nuevo
				break
			}
			if espera < cfg.TopeDeEspera {
				if espera *= 2; espera > cfg.TopeDeEspera {
					espera = cfg.TopeDeEspera
				}
			}
		}
		espera = esperaInicial

		if !entregar(ctx, eventos, Evento{
			Tipo:     TipoEnlaceVuelto,
			Crudo:    "el ENDEC volvió a contestar",
			Texto:    "el ENDEC volvió a contestar",
			Recibido: cfg.Reloj(),
		}) {
			return
		}
	}
}

// bombear lee del enlace hasta que se rompe y devuelve por qué. Lo que quedara
// sin salto de línea al final se entrega igual (Analizador.Cerrar): el último
// mensaje de un cable que se corta suele quedarse justo ahí.
func bombear(ctx context.Context, an *Analizador, enlace Enlace, eventos chan<- Evento) error {
	// Un Read de socket se queda esperando para siempre, y un ENDEC puede
	// estar callado semanas: la única manera de que cancelar el contexto
	// devuelva el control es cerrarle el enlace por debajo. El vigilante muere
	// con la función, así que no queda una goroutine por conexión perdida.
	fin := make(chan struct{})
	defer close(fin)
	go func() {
		select {
		case <-ctx.Done():
			_ = enlace.Close()
		case <-fin:
		}
	}()

	buf := make([]byte, 4<<10)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		n, err := enlace.Read(buf)
		if n > 0 {
			for _, e := range an.Escribir(buf[:n]) {
				if !entregar(ctx, eventos, e) {
					return ctx.Err()
				}
			}
		}
		if err != nil {
			for _, e := range an.Cerrar() {
				if !entregar(ctx, eventos, e) {
					return ctx.Err()
				}
			}
			return err
		}
	}
}

// entregar pone un evento en el canal, o devuelve false si se canceló el
// contexto mientras esperaba sitio. Bloquea a propósito: una alerta no se tira.
func entregar(ctx context.Context, eventos chan<- Evento, e Evento) bool {
	select {
	case eventos <- e:
		return true
	case <-ctx.Done():
		return false
	}
}

func motivoEnPalabras(err error) string {
	switch {
	case err == nil || errors.Is(err, io.EOF):
		return "el ENDEC dejó de mandar datos (cable desconectado, equipo apagado, o el puerto cambió de device type)"
	default:
		return "se cortó la lectura del ENDEC: " + err.Error()
	}
}

func dormirDeVerdad(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
	case <-ctx.Done():
	}
}

// abrirSerialDelSistema es el único sitio del paquete que toca hardware.
//
// 8N1 porque la parte 11 es ASCII de siete bits con el octavo en cero y el
// manual dice «async, one stop bit» (§8.1): ocho bits de datos sin paridad lo
// llevan tal cual, y es lo que espera cualquier capturador serial.
//
// El plazo de lectura es de un segundo y no infinito para que Read vuelva y el
// bucle pueda mirar el contexto: sin él, cerrar el canal esperaría a que el
// ENDEC dijera algo, y un ENDEC puede estar callado semanas.
func abrirSerialDelSistema(puerto string, baudios int) (Enlace, error) {
	p, err := serial.Open(puerto, &serial.Mode{
		BaudRate: baudios,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	})
	if err != nil {
		return nil, err
	}
	if err := p.SetReadTimeout(time.Second); err != nil {
		_ = p.Close()
		return nil, err
	}
	return &puertoSerial{p: p}, nil
}

// puertoSerial envuelve el puerto para que un plazo de lectura vencido no
// parezca el fin del mundo: go.bug.st/serial devuelve (0, nil) cuando no llegó
// nada, y eso es exactamente lo que el bucle de bombear necesita —seguir
// leyendo— sin confundirlo con un enlace roto.
type puertoSerial struct{ p serial.Port }

func (s *puertoSerial) Read(b []byte) (int, error) { return s.p.Read(b) }
func (s *puertoSerial) Close() error               { return s.p.Close() }

func marcarDelSistema(ctx context.Context, red, direccion string) (Enlace, error) {
	d := net.Dialer{Timeout: 10 * time.Second}
	c, err := d.DialContext(ctx, red, direccion)
	if err != nil {
		return nil, err
	}
	return c, nil
}
