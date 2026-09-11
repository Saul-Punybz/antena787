// cliente.go es el lado de Antena787 del enlace: el **«automation system»** en
// la jerga de SCTE 104. El otro lado es el «compression system» —el
// inyector/encoder de la estación—, que es quien convierte lo que aquí se pide
// en el SCTE-35 del transport stream (ADR 0004).
//
// Lo que el cliente sostiene es un enlace, no una llamada: abre el socket, se
// presenta con un init_request, late cada diez segundos, y si el enlace se cae
// vuelve a levantarlo con espera progresiva 1, 2, 4… con tope de 60 s, sin
// rendirse nunca (el esquema de F2-48). Cuenta los reintentos y guarda el
// último error para que la pantalla pueda decir en palabras claras cómo está el
// enlace, con el mismo vocabulario que las salidas.
//
// **Este paquete no se entera de que existe el motor ni el plan.** No conoce
// `model`, ni `app`, ni un canal: recibe un identificador de corte y unas
// duraciones. Cablearlo al plan y a la venta de cortes es otra tanda.
package scte104

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// ErrSinConexion es lo que devuelve pedir un corte cuando el enlace no está en
// pie.
//
// **No se encola nada.** Un corte es una cosa que pasa a una hora; guardarlo
// para mandarlo cuando vuelva la conexión sería mandar un corte al medio de un
// programa. Así que el corte falla, quien llama se enterá, y queda en la
// bitácora: el aire sigue saliendo igual, con el corte vendido y emitido
// localmente (PRD §9 paso 7).
var ErrSinConexion = errors.New("scte104: el enlace con el inyector no está en pie")

// EpocaGPS es el cero de la cuenta de segundos del alive: el 6 de enero de
// 1980 a las 00:00 UTC. Es la época de GPS, la misma que usa SCTE 35 para su
// hora UTC, y la que calcula el cliente de la referencia de TypeScript.
var EpocaGPS = time.Date(1980, time.January, 6, 0, 0, 0, 0, time.UTC)

// LatidoPorDefecto es cada cuánto sale un alive_request. Diez segundos es lo
// que trae de fábrica este driver; se puede cambiar, porque cada inyector tiene
// su propio plazo antes de dar el enlace por muerto.
const LatidoPorDefecto = 10 * time.Second

// PlazoRespuestaPorDefecto es cuánto se espera la respuesta del inyector a una
// petición antes de dar el enlace por muerto y reconectar. Cinco segundos es
// mucho para una red local y poco para que un corte se quede colgado.
const PlazoRespuestaPorDefecto = 5 * time.Second

// PlazoConexionPorDefecto es cuánto se espera a que el socket se abra.
const PlazoConexionPorDefecto = 5 * time.Second

// EsperaProgresiva es el 1, 2, 4… con tope de 60 s que manda F2-48: el primer
// intento espera un segundo, el segundo dos, el tercero cuatro, y desde el
// séptimo se queda en 60. **Nunca devuelve cero y nunca se rinde**: el backoff
// deja de crecer, no de intentar.
func EsperaProgresiva(intento int) time.Duration {
	if intento < 1 {
		intento = 1
	}
	const tope = 60 * time.Second
	if intento > 7 {
		return tope
	}
	d := time.Duration(1<<(intento-1)) * time.Second
	if d > tope {
		return tope
	}
	return d
}

// Opciones configura un Cliente. Todo tiene valor de fábrica menos la
// dirección; los campos de función están para que las pruebas puedan entrar
// sin red ni relojes de verdad.
type Opciones struct {
	// Direccion es host:puerto del inyector. Si no trae puerto se le pone
	// PuertoPorDefecto.
	Direccion string
	// IndicePID es el `DPI_PID_index` de todos los mensajes: a qué flujo de
	// cue del transport stream van los cortes. 0 para un canal único.
	IndicePID uint16
	// IndiceAS es el `AS_index`: qué sistema de automatización somos, cuando
	// varios comparten inyector.
	IndiceAS uint8
	// VersionSCTE35 es el `SCTE35_protocol_version` que se le pide al encoder.
	VersionSCTE35 uint8
	// Latido es cada cuánto sale un alive_request. LatidoPorDefecto si es 0.
	Latido time.Duration
	// PlazoRespuesta es cuánto se espera cada respuesta.
	// PlazoRespuestaPorDefecto si es 0.
	PlazoRespuesta time.Duration
	// Dial abre el socket. Se puede cambiar para las pruebas o para salir por
	// una interfaz concreta; por defecto es un net.Dialer con
	// PlazoConexionPorDefecto.
	Dial func(ctx context.Context, red, dir string) (net.Conn, error)
	// Espera dice cuánto esperar antes del reintento número `intento`.
	// EsperaProgresiva si es nil.
	Espera func(intento int) time.Duration
	// Reloj da la hora. time.Now si es nil.
	Reloj func() time.Time
	// Aviso, si está puesto, recibe una frase clara cada vez que el
	// enlace cambia de estado. Es el gancho para la bitácora; este paquete no
	// escribe incidentes por su cuenta porque no conoce la base.
	Aviso func(texto string)
}

func (o *Opciones) rellenar() {
	if o.Latido <= 0 {
		o.Latido = LatidoPorDefecto
	}
	if o.PlazoRespuesta <= 0 {
		o.PlazoRespuesta = PlazoRespuestaPorDefecto
	}
	if o.Espera == nil {
		o.Espera = EsperaProgresiva
	}
	if o.Reloj == nil {
		o.Reloj = time.Now
	}
	if o.Dial == nil {
		d := &net.Dialer{Timeout: PlazoConexionPorDefecto}
		o.Dial = d.DialContext
	}
}

// Estado es lo que la pantalla necesita saber del enlace. Es el mismo
// vocabulario que F2-48 pide para las salidas —conectado, reintentos, último
// error—, para que se pueda pintar con el mismo componente.
type Estado struct {
	// Conectado es si el enlace está en pie **y con el init_request
	// contestado**: un socket abierto al que el inyector no le contestó no
	// cuenta.
	Conectado bool
	// Direccion es con quién se está hablando (o intentando).
	Direccion string
	// Reintentos es cuántas veces se ha tenido que volver a levantar el
	// enlace desde que arrancó el cliente. No se pone a cero al reconectar: en
	// una estación que lleva semanas encendida, un número que crece despacio y
	// uno que crece a saltos cuentan cosas distintas.
	Reintentos int64
	// UltimoError es el último fallo, en palabras claras, o vacío si nunca hubo.
	UltimoError string
	// DesdeCuando es cuándo se abrió el enlace que está en pie.
	DesdeCuando time.Time
	// UltimoLatido es cuándo contestó el inyector el último alive_request. Es
	// la señal de que el enlace está vivo de verdad y no solo abierto.
	UltimoLatido time.Time
	// CortesPedidos es cuántos multiple_operation_message de corte se han
	// mandado y han sido aceptados.
	CortesPedidos int64
	// UltimoResultado es el `result` de la última respuesta del inyector.
	UltimoResultado uint16
}

// espera es una petición esperando su respuesta: el predicado dice cuál de los
// mensajes que llegan es la suya.
type espera struct {
	coincide func(Sencillo) bool
	buzon    chan Sencillo
}

// Cliente es el «automation system» de SCTE 104: sostiene un enlace TCP con el
// inyector y le pide cortes.
//
// Es seguro usarlo desde varias goroutines. El uso normal es uno por canal:
// Conectar una vez al encender el canal, y IniciarCorte / TerminarCorte cada
// vez que el plan lo pida.
type Cliente struct {
	opc Opciones

	// mu protege todo lo de abajo. Las escrituras al socket van por su propio
	// candado (muEscritura) para no tener el estado bloqueado mientras la red
	// se toma su tiempo.
	mu        sync.Mutex
	conn      net.Conn
	numero    uint8
	esperando []*espera
	est       Estado

	muEscritura sync.Mutex

	arranque  sync.Once
	cancelar  context.CancelFunc
	listo     chan struct{}
	unaVez    sync.Once
	terminado chan struct{}
}

// NuevoCliente arma el cliente sin tocar la red. Nada sale hasta Conectar.
func NuevoCliente(opc Opciones) *Cliente {
	opc.rellenar()
	c := &Cliente{
		opc:       opc,
		listo:     make(chan struct{}),
		terminado: make(chan struct{}),
	}
	c.est.Direccion = conPuerto(opc.Direccion)
	return c
}

// conPuerto le pone PuertoPorDefecto a una dirección que viene sin puerto, para
// que en Ajustes se pueda escribir solo la IP del encoder.
func conPuerto(dir string) string {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return ""
	}
	if _, _, err := net.SplitHostPort(dir); err == nil {
		return dir
	}
	return net.JoinHostPort(dir, fmt.Sprint(PuertoPorDefecto))
}

// Conectar abre el enlace con el inyector y **espera a que conteste el
// init_request**, que es el único momento en que se sabe que el otro lado
// existe y nos acepta.
//
// Si el primer intento falla no se rinde: sigue reintentando en segundo plano
// con la espera progresiva, y Conectar devuelve error solo si `ctx` se cancela
// antes de que alguno salga bien. Es a propósito —un encoder que arranca más
// despacio que el playout es lo normal en una cabecera— y es lo que pide F2-48:
// el backoff nunca deja de reintentar. Mientras tanto Estado() ya cuenta los
// reintentos y guarda el último error, así que la pantalla puede decir qué está
// pasando sin que nadie se quede bloqueado mirándola.
//
// `direccion` es host:puerto; si viene vacía se usa la de las Opciones, y si no
// trae puerto se le pone PuertoPorDefecto (5167). El `ctx` es el del canal: al
// cancelarlo el enlace se cierra y el cliente no vuelve a levantarse.
func (c *Cliente) Conectar(ctx context.Context, direccion string) error {
	if direccion == "" {
		direccion = c.opc.Direccion
	}
	direccion = conPuerto(direccion)
	if direccion == "" {
		return errors.New("scte104: hace falta la dirección del inyector (host:puerto)")
	}

	arrancado := false
	c.arranque.Do(func() {
		arrancado = true
		ctx, cancel := context.WithCancel(ctx)
		c.mu.Lock()
		c.est.Direccion = direccion
		c.cancelar = cancel
		c.mu.Unlock()
		go c.supervisar(ctx, direccion)
	})
	if !arrancado {
		return errors.New("scte104: este cliente ya está conectado; hace falta uno nuevo para otra dirección")
	}

	select {
	case <-c.listo:
		return nil
	case <-c.terminado:
		est := c.Estado()
		return fmt.Errorf("scte104: no se pudo abrir el enlace con %s tras %d intentos: %s",
			est.Direccion, est.Reintentos, est.UltimoError)
	}
}

// Cerrar cierra el enlace y para los reintentos. Se puede llamar dos veces.
func (c *Cliente) Cerrar() error {
	c.mu.Lock()
	cancelar := c.cancelar
	c.mu.Unlock()
	if cancelar == nil {
		return nil
	}
	cancelar()
	<-c.terminado
	return nil
}

// Estado devuelve una foto del enlace para la pantalla.
func (c *Cliente) Estado() Estado {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.est
}

// supervisar es el bucle que mantiene el enlace levantado para siempre: una
// sesión tras otra, con la espera progresiva entre medias. Solo termina cuando
// se cancela el contexto.
func (c *Cliente) supervisar(ctx context.Context, direccion string) {
	defer close(c.terminado)
	// Si el contexto se cancela antes del primer init, Conectar tiene que
	// dejar de esperar: cerrar `terminado` lo desbloquea.
	intento := 0
	for {
		if ctx.Err() != nil {
			return
		}
		abierto, err := c.sesion(ctx, direccion)
		c.cerrarSesion(err)
		if ctx.Err() != nil {
			return
		}
		if abierto {
			// El enlace estuvo en pie: la próxima caída vuelve a empezar la
			// cuenta por un segundo, no por donde iba.
			intento = 0
		}
		intento++
		c.anotarReintento(err)
		select {
		case <-ctx.Done():
			return
		case <-time.After(c.opc.Espera(intento)):
		}
	}
}

// sesion es una conexión de principio a fin: abrir, presentarse, latir, y
// volver cuando algo se rompe. El primer valor dice si se llegó a abrir de
// verdad (init_request contestado), que es lo que decide si el backoff se
// reinicia.
func (c *Cliente) sesion(ctx context.Context, direccion string) (bool, error) {
	conn, err := c.opc.Dial(ctx, "tcp", direccion)
	if err != nil {
		return false, fmt.Errorf("no se pudo abrir el socket con %s: %w", direccion, err)
	}
	defer conn.Close()

	ctxS, cancelS := context.WithCancel(ctx)
	defer cancelS()
	// Cerrar el socket es la única forma de desbloquear un io.ReadFull que ya
	// está esperando bytes; sin esto, cancelar el canal dejaría al lector
	// colgado hasta que el inyector dijera algo.
	go func() {
		<-ctxS.Done()
		_ = conn.Close()
	}()

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	finLector := make(chan error, 1)
	go func() { finLector <- c.leer(conn) }()

	// El init_request es la presentación: hasta que el inyector no lo contesta
	// no atiende ninguna petición de corte.
	resp, err := c.pedirSencillo(ctxS, OpInicioPeticion, nil, porOp(OpInicioRespuesta))
	if err != nil {
		return false, fmt.Errorf("el inyector no contestó el init_request: %w", err)
	}
	if resp.Resultado != ResultadoExito {
		return false, fmt.Errorf("el inyector rechazó el init_request: %s (result %d)",
			NombreDeResultado(resp.Resultado), resp.Resultado)
	}
	c.abrir()

	tic := time.NewTicker(c.opc.Latido)
	defer tic.Stop()
	for {
		select {
		case <-ctxS.Done():
			return true, ctxS.Err()
		case err := <-finLector:
			if err == nil {
				err = errors.New("el inyector cerró el socket")
			}
			return true, err
		case <-tic.C:
			// El latido es también el detector de enlaces zombis: un socket
			// abierto que ya no contesta se ve aquí y en ningún otro sitio.
			if _, err := c.pedirSencillo(ctxS, OpVivoPeticion, c.ahoraGPS().codificar(), porOp(OpVivoRespuesta)); err != nil {
				return true, fmt.Errorf("el inyector no contestó el alive_request: %w", err)
			}
			c.mu.Lock()
			c.est.UltimoLatido = c.opc.Reloj()
			c.mu.Unlock()
		}
	}
}

// leer va sacando marcos del socket y repartiéndolos entre las peticiones que
// esperan respuesta. Devuelve el error que cortó la lectura.
func (c *Cliente) leer(conn net.Conn) error {
	for {
		marco, err := LeerMarco(conn)
		if err != nil {
			return err
		}
		if EsMultiple(marco) {
			// Un inyector no manda mensajes múltiples; si llega uno, es que el
			// otro extremo no es lo que dice ser. No es motivo para tirar el
			// enlace, así que se ignora y se sigue.
			continue
		}
		m, err := DecodificarSencillo(marco)
		if err != nil {
			return err
		}
		c.entregar(m)
	}
}

// entregar le da el mensaje a la primera petición que lo reconozca como suyo.
// Un mensaje que nadie espera —un AS_alive_request del inyector, un acuse que
// llegó tarde— se tira sin ruido: el enlace no se rompe por eso.
func (c *Cliente) entregar(m Sencillo) {
	c.mu.Lock()
	c.est.UltimoResultado = m.Resultado
	for i, e := range c.esperando {
		if e.coincide(m) {
			c.esperando = append(c.esperando[:i], c.esperando[i+1:]...)
			c.mu.Unlock()
			e.buzon <- m
			return
		}
	}
	c.mu.Unlock()
}

// porOp empareja una respuesta por su opID. Es lo que vale para el init y el
// alive, que son los dos mensajes de los que solo hay uno en vuelo a la vez.
func porOp(op uint16) func(Sencillo) bool {
	return func(m Sencillo) bool { return m.OpID == op }
}

// porNumero empareja el acuse de un multiple_operation_message por el
// message_number que acusa (ver Sencillo.NumeroAcusado). Se aceptan los tres
// mensajes con los que un inyector puede contestar un múltiple.
func porNumero(n uint8) func(Sencillo) bool {
	return func(m Sencillo) bool {
		switch m.OpID {
		case OpInyectaRespuesta, OpInyectaCompleta, OpRespuestaGeneral:
			return m.NumeroAcusado() == n
		}
		return false
	}
}

// siguienteNumero reparte el `message_number`. Es de un byte, así que da la
// vuelta cada 256; con un plazo de respuesta de segundos no hay forma de tener
// 256 peticiones en vuelo, así que la vuelta no confunde a nadie.
func (c *Cliente) siguienteNumero() uint8 {
	c.numero++
	return c.numero
}

// esperar apunta una petición en la lista de las que esperan respuesta. Hay que
// llamarla **antes** de escribir en el socket: si se hiciera después, una
// respuesta muy rápida llegaría antes de que hubiera quien la recogiera.
func (c *Cliente) esperar(coincide func(Sencillo) bool) *espera {
	e := &espera{coincide: coincide, buzon: make(chan Sencillo, 1)}
	c.esperando = append(c.esperando, e)
	return e
}

// olvidar saca una espera de la lista cuando se agotó el plazo o murió la
// sesión, para que la lista no crezca sola.
func (c *Cliente) olvidar(e *espera) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, otra := range c.esperando {
		if otra == e {
			c.esperando = append(c.esperando[:i], c.esperando[i+1:]...)
			return
		}
	}
}

// pedirSencillo manda un single_operation_message y espera su respuesta.
func (c *Cliente) pedirSencillo(ctx context.Context, op uint16, datos []byte, coincide func(Sencillo) bool) (Sencillo, error) {
	c.mu.Lock()
	conn := c.conn
	if conn == nil {
		c.mu.Unlock()
		return Sencillo{}, ErrSinConexion
	}
	m := Sencillo{
		OpID:           op,
		Resultado:      ResultadoNoUsado,
		ResultadoExtra: ResultadoNoUsado,
		IndiceAS:       c.opc.IndiceAS,
		Numero:         c.siguienteNumero(),
		IndicePID:      c.opc.IndicePID,
		Datos:          datos,
	}
	e := c.esperar(coincide)
	c.mu.Unlock()

	b, err := m.Codificar()
	if err != nil {
		c.olvidar(e)
		return Sencillo{}, err
	}
	return c.escribirYEsperar(ctx, conn, b, e)
}

// Enviar manda un multiple_operation_message con las operaciones que se le
// pasen y devuelve el acuse del inyector. Es la puerta de entrada para todo lo
// que no sea un corte —una señal de hora, un DTMF, un comando de fabricante— y
// para agrupar varias operaciones en un solo mensaje, que es como se señala un
// bloque con varios spots de una sola vez.
//
// La marca de tiempo es SinMarca: las operaciones se ejecutan al llegar, con el
// pre-roll que traiga cada una. Para otra marca está EnviarConMarca.
func (c *Cliente) Enviar(ops ...Operacion) (Sencillo, error) {
	return c.EnviarConMarca(SinMarca, ops...)
}

// EnviarConMarca es Enviar diciendo además *cuándo*: una hora UTC, un timecode
// VITC o un flanco de GPI del propio inyector (ver Marca).
func (c *Cliente) EnviarConMarca(marca Marca, ops ...Operacion) (Sencillo, error) {
	if len(ops) == 0 {
		return Sencillo{}, errors.New("scte104: un multiple_operation_message sin operaciones no dice nada")
	}
	c.mu.Lock()
	conn := c.conn
	if conn == nil || !c.est.Conectado {
		c.mu.Unlock()
		return Sencillo{}, ErrSinConexion
	}
	m := Multiple{
		IndiceAS:      c.opc.IndiceAS,
		Numero:        c.siguienteNumero(),
		IndicePID:     c.opc.IndicePID,
		VersionSCTE35: c.opc.VersionSCTE35,
		Marca:         marca,
		Operaciones:   ops,
	}
	e := c.esperar(porNumero(m.Numero))
	c.mu.Unlock()

	b, err := m.Codificar()
	if err != nil {
		c.olvidar(e)
		return Sencillo{}, err
	}

	ctx, cancelar := context.WithCancel(context.Background())
	defer cancelar()
	resp, err := c.escribirYEsperar(ctx, conn, b, e)
	if err != nil {
		return Sencillo{}, err
	}
	if resp.Resultado != ResultadoExito {
		return resp, fmt.Errorf("scte104: el inyector rechazó %s: %s (result %d)",
			NombreDeMop(ops[0].OpID), NombreDeResultado(resp.Resultado), resp.Resultado)
	}
	return resp, nil
}

// escribirYEsperar escribe el marco y espera la respuesta hasta el plazo. La
// escritura va bajo su propio candado para que dos peticiones a la vez no
// entrelacen sus bytes en el socket.
func (c *Cliente) escribirYEsperar(ctx context.Context, conn net.Conn, b []byte, e *espera) (Sencillo, error) {
	c.muEscritura.Lock()
	_, err := conn.Write(b)
	c.muEscritura.Unlock()
	if err != nil {
		c.olvidar(e)
		return Sencillo{}, fmt.Errorf("no se pudo escribir en el socket: %w", err)
	}

	plazo := time.NewTimer(c.opc.PlazoRespuesta)
	defer plazo.Stop()
	select {
	case m, abierto := <-e.buzon:
		if !abierto {
			// El buzón cerrado es cómo cerrarSesion avisa de que la respuesta
			// ya no va a llegar. Sin este `abierto` se devolvería un Sencillo
			// en cero como si fuera una respuesta buena, y un corte se daría
			// por aceptado sin que nadie lo hubiera aceptado.
			return Sencillo{}, ErrSinConexion
		}
		return m, nil
	case <-plazo.C:
		c.olvidar(e)
		return Sencillo{}, fmt.Errorf("el inyector no contestó en %s", c.opc.PlazoRespuesta)
	case <-ctx.Done():
		c.olvidar(e)
		return Sencillo{}, ctx.Err()
	}
}

// IniciarCorte le pide al encoder que **abra** un corte publicitario: el
// spliceStart del estándar, que es lo que se convierte en el `splice_insert`
// del SCTE-35 aguas abajo.
//
//   - `id` es el `splice_event_id`. Tiene que ser único mientras el corte esté
//     vivo, porque es con él con lo que se cierra y se cancela, y es el que
//     permite comprobar más tarde que el corte salió de verdad.
//   - `preRoll` es cuánto falta para el corte. Si es 0 se manda
//     **spliceStart_immediate**; si no, **spliceStart_normal** con el
//     pre-roll en milisegundos. SCTE 104 pide no bajar de 4 s, pero aquí no se
//     valida a propósito: el que sabe cuánto margen necesita es el inyector, y
//     lo dice con el result 122 (ResultadoPreRollMuyChico), que sale en el
//     error. Inventar el límite por él sería negarle un corte que sí acepta.
//   - `duracion` es el `break_duration`, que en el cable va en décimas de
//     segundo. 0 significa que la duración no se anuncia y el corte se cierra
//     con TerminarCorte.
//   - `autoReturn` es el `auto_return_flag`: con true el aire vuelve solo al
//     cumplirse la duración, aunque el playout se haya muerto en medio. Es el
//     modo seguro y el que hay que usar salvo que haya una razón.
func (c *Cliente) IniciarCorte(id uint32, preRoll, duracion time.Duration, autoReturn bool) error {
	tipo := CorteEmpiezaNormal
	if preRoll <= 0 {
		tipo = CorteEmpiezaYa
	}
	corte, err := armarCorte(tipo, id, preRoll, duracion, autoReturn)
	if err != nil {
		return err
	}
	if _, err := c.Enviar(corte.Operacion()); err != nil {
		return err
	}
	c.mu.Lock()
	c.est.CortesPedidos++
	c.mu.Unlock()
	return nil
}

// TerminarCorte le pide al encoder que **cierre** el corte `id`: un
// spliceEnd_immediate, sin pre-roll.
//
// Es inmediato porque el fin de un corte lo decide el contenido que ya está
// saliendo —el último spot acabó— y no un plan hecho de antes. Cuando sí se
// sabe con antelación, TerminarCorteEn manda el spliceEnd_normal con su
// pre-roll.
//
// Con `autoReturn` puesto al abrir, esto es un cinturón de más: el corte se
// cerraría solo igual.
func (c *Cliente) TerminarCorte(id uint32) error {
	return c.terminar(id, 0)
}

// TerminarCorteEn cierra el corte `id` dentro de `preRoll`: el
// spliceEnd_normal, para cuando el fin del corte se sabe con antelación.
func (c *Cliente) TerminarCorteEn(id uint32, preRoll time.Duration) error {
	return c.terminar(id, preRoll)
}

func (c *Cliente) terminar(id uint32, preRoll time.Duration) error {
	tipo := CorteTerminaNormal
	if preRoll <= 0 {
		tipo = CorteTerminaYa
	}
	corte, err := armarCorte(tipo, id, preRoll, 0, false)
	if err != nil {
		return err
	}
	_, err = c.Enviar(corte.Operacion())
	return err
}

// Cancelar deshace un corte ya anunciado que **todavía no empezó**: el
// splice_cancel, por su `splice_event_id`. Un corte que ya está al aire no se
// cancela, se termina.
func (c *Cliente) Cancelar(id uint32) error {
	corte, err := armarCorte(CorteCancela, id, 0, 0, false)
	if err != nil {
		return err
	}
	_, err = c.Enviar(corte.Operacion())
	return err
}

// armarCorte traduce duraciones de Go a los campos del estándar y se queja
// antes de mandar nada si no caben. Los dos campos son de 16 bits: el pre-roll
// en milisegundos llega a 65,5 s y la duración en décimas de segundo llega a
// 109 minutos, que es de sobra para un corte y muy poco para un descuido.
func armarCorte(tipo TipoCorte, id uint32, preRoll, duracion time.Duration, autoReturn bool) (Corte, error) {
	ms := preRoll.Milliseconds()
	if ms < 0 || ms > 0xFFFF {
		return Corte{}, fmt.Errorf("scte104: un pre-roll de %s no cabe en el pre_roll_time (máximo %s)",
			preRoll, time.Duration(0xFFFF)*time.Millisecond)
	}
	decimas := duracion.Milliseconds() / 100
	if decimas < 0 || decimas > 0xFFFF {
		return Corte{}, fmt.Errorf("scte104: una duración de %s no cabe en el break_duration (máximo %s)",
			duracion, time.Duration(0xFFFF)*100*time.Millisecond)
	}
	var auto uint8
	if autoReturn {
		auto = 1
	}
	return Corte{
		Tipo:       tipo,
		EventoID:   id,
		PreRoll:    uint16(ms),
		Duracion:   uint16(decimas),
		AutoReturn: auto,
	}, nil
}

// ahoraGPS es la hora del automatismo en el formato del alive: segundos y
// microsegundos desde la época de GPS.
func (c *Cliente) ahoraGPS() Tiempo {
	d := c.opc.Reloj().UTC().Sub(EpocaGPS)
	if d < 0 {
		// Un reloj antes de 1980 es un reloj sin poner en hora. Se manda cero
		// en vez de un número enorme por desbordamiento: el inyector puede
		// quejarse de la hora, pero el enlace no se rompe por eso.
		return Tiempo{}
	}
	return Tiempo{
		Segundos:      uint32(d / time.Second),
		Microsegundos: uint32((d % time.Second) / time.Microsecond),
	}
}

// abrir marca el enlace como en pie y desbloquea a quien esté en Conectar.
func (c *Cliente) abrir() {
	c.mu.Lock()
	c.est.Conectado = true
	c.est.DesdeCuando = c.opc.Reloj()
	// UltimoError **no** se borra al reconectar, igual que Reintentos no se
	// pone a cero: en una estación que lleva semanas encendida, saber por qué
	// se cayó la última vez vale más que ver el campo limpio.
	dir := c.est.Direccion
	c.mu.Unlock()
	c.unaVez.Do(func() { close(c.listo) })
	c.avisar(fmt.Sprintf("SCTE-104: enlace abierto con el inyector %s", dir))
}

// cerrarSesion deja el enlace por caído y despierta a todas las peticiones que
// estuvieran esperando: una respuesta que ya no va a llegar tiene que fallar
// ahora, no al agotarse el plazo.
func (c *Cliente) cerrarSesion(err error) {
	c.mu.Lock()
	estaba := c.est.Conectado
	c.est.Conectado = false
	c.conn = nil
	pendientes := c.esperando
	c.esperando = nil
	c.mu.Unlock()
	for _, e := range pendientes {
		close(e.buzon)
	}
	if estaba && err != nil {
		c.avisar(fmt.Sprintf("SCTE-104: se cayó el enlace con el inyector: %s", err))
	}
}

// anotarReintento suma el intento y guarda el error para la pantalla.
func (c *Cliente) anotarReintento(err error) {
	c.mu.Lock()
	c.est.Reintentos++
	if err != nil {
		c.est.UltimoError = err.Error()
	}
	c.mu.Unlock()
}

func (c *Cliente) avisar(texto string) {
	if c.opc.Aviso != nil {
		c.opc.Aviso(texto)
	}
}
