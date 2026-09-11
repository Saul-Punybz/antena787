// motor.go es el motor dentro del proceso de verdad: el bucle que enciende
// el encoder persistente y el servidor de cuadros cuando el canal está al
// aire, y la fuente que traduce el plan a clips. Es la tanda T1 de
// docs/f2/PLAN-F2.md.
//
// Desde T2 hay cuatro carriles, los decks del §9 paso 4 del PRD, y en cada
// instante el aire lo tiene el de más prioridad que tenga algo que poner:
// manual > comercial > programa > relleno. Un corte pautado a las 14:00:00
// entra a esa hora exacta, el programa que venía de un archivo **se pausa y
// reanuda donde iba** (F2-06, F2-07), y una señal en vivo no se pausa: sigue
// corriendo por debajo y se vuelve a ella en su instante actual (F2-08).
// Cuando ningún deck tiene nada sale el relleno; si no hay relleno, el cartel
// de la estación. Nunca negro, nunca silencio.
//
// En modo sombra este archivo no hace nada más que decirlo una vez: F1 no
// cambia de conducta porque F2 exista.
package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"antena787/internal/engine"
	"antena787/internal/model"
)

// Los dos modos del canal (channel.modo).
const (
	ModoSombra = "sombra"
	ModoAire   = "aire"
)

// Números del motor. Los de F2-11 y F2-12 son del PRD; los demás son lo más
// simple que funciona y están aquí para poder cambiarlos de un sitio.
const (
	// MotorPoll es cada cuánto se mira si el canal ya pasó a estar al aire.
	MotorPoll = 10 * time.Second
	// EsperaMotorMin y EsperaMotorMax son la espera progresiva entre
	// reinicios del encoder: la primera vez un segundo, el doble cada vez,
	// tope de medio minuto (F2-11).
	EsperaMotorMin = time.Second
	EsperaMotorMax = 30 * time.Second
	// VidaBuena es lo que tiene que aguantar una corrida del encoder para
	// que la espera vuelva a empezar por abajo.
	VidaBuena = 5 * time.Minute
	// VentanaAtras y VentanaAdelante son cuánto plan se lee alrededor de
	// ahora. Hacia atrás cubre un bloque largo que empezó antes.
	VentanaAtras    = 12 * time.Hour
	VentanaAdelante = 6 * time.Hour
	// SeekMinimo es desde cuánto vale la pena entrar a un archivo por el
	// medio. Por debajo se abre desde el principio: buscar un cuadro clave
	// cuesta más que los dos segundos que se ahorran (F2-13).
	SeekMinimo = 2 * time.Second
	// FalloRepetido es lo que tienen que estar separados dos fallos del mismo
	// archivo para que el segundo lo mande a cuarentena. Un parpadeo de red
	// no saca de la parrilla un programa bueno (F2-12).
	FalloRepetido = 5 * time.Minute
	// RellenoSuelto es lo que se le da al relleno cuando no se sabe cuánto
	// dura ni cuándo empieza lo siguiente.
	RellenoSuelto = time.Minute
	// RepetirIncidente es cada cuánto se vuelve a anotar el mismo motivo de
	// parada. Entre medias se dice por el bus, para que la bitácora siga
	// siendo legible cuando algo lleva horas mal.
	RepetirIncidente = 10 * time.Minute
	// EncoderColgado es lo que se le da al encoder para tragarse un cuadro
	// antes de darlo por colgado: tres segundos, el número que pide F2-11.
	// Se puede cambiar en ajustes con KeyEncoderColgadoS.
	EncoderColgado = 3 * time.Second
	// ColgadoMinimo y ColgadoMaximo son lo que se acepta de ese ajuste. Por
	// debajo de un segundo, un hipo del disco relanzaría el encoder; por
	// encima de un minuto el televidente ya cambió de canal.
	ColgadoMinimo = time.Second
	ColgadoMaximo = time.Minute
	// LatidoDelVigilante es cada cuánto se mira si el encoder sigue tragando.
	// Medio segundo: mirar un reloj no cuesta nada y el plazo no se pasa por
	// más de eso.
	LatidoDelVigilante = 500 * time.Millisecond
	// VentanaDeLaTarjeta y ColgadasParaSoftware son la regla de F2-11: dos
	// veces colgado en diez minutos ya no es mala suerte, es una tarjeta que
	// dejó de responder, y el canal sigue por el procesador.
	VentanaDeLaTarjeta   = 10 * time.Minute
	ColgadasParaSoftware = 2
	// GraciaDelCartel es cuánto sale el cartel de la estación después de
	// relanzar un encoder colgado. Lo que colgó a ffmpeg pudo ser el clip que
	// estaba saliendo: volver a metérselo al encoder recién nacido es pedirle
	// que se cuelgue otra vez. Diez segundos es lo que tarda la cascada en
	// volver a pedir el plan con el aire ya estable.
	GraciaDelCartel = 10 * time.Second
)

// KeyEncoderColgadoS es cada cuántos segundos sin tragar un cuadro se da el
// encoder por colgado. Vive en settings como todo lo demás: no hay archivo de
// configuración (PRD §14.1). De fábrica tres (F2-11).
const KeyEncoderColgadoS = "encoder_colgado_s"

// FraseTarjetaCaida es lo que lee la persona del canal cuando la tarjeta de
// video dejó de responder dos veces en diez minutos. Está escrita una sola vez
// porque va a tres sitios —la bitácora, el bus y la pantalla de estado— y las
// tres tienen que decir lo mismo, sin jerga (F2-11).
const FraseTarjetaCaida = "tu tarjeta de video dejó de responder"

// motorLoop es la goroutine del aire. Corre bajo guard como todas: si algo
// dentro entra en pánico, queda el incidente y vuelve sola (auditoría A2).
func (a *App) motorLoop(ctx context.Context) error {
	dijoSombra := false
	espera := EsperaMotorMin
	var ultimoMotivo string
	var ultimoIncidente time.Time
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		ch, err := a.Store.Channel.Get(ctx, a.ChannelID)
		if err != nil {
			if !dormir(ctx, MotorPoll) {
				return ctx.Err()
			}
			continue
		}
		if ch.Mode != ModoAire {
			// Se dice una vez, no cada diez segundos: la bitácora es para
			// leerla.
			if !dijoSombra {
				a.Publish("motor", ModoSombra,
					"el canal está en modo sombra: se arma el plan y se publica la guía, pero no se emite")
				dijoSombra = true
			}
			// Se despierta en el acto cuando alguien saca el canal de sombra
			// (F2-118), y si no, en el próximo repaso.
			if !a.dormirOCambioDeModo(ctx, MotorPoll) {
				return ctx.Err()
			}
			continue
		}
		dijoSombra = false

		arranque := time.Now()
		err = a.correrMotor(ctx, ch)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Si el aire se apagó a propósito, el motor no se cayó: lo apagaron.
		// Ni incidente de encoder ni espera progresiva; la vuelta de arriba
		// dice que el canal está en sombra y ahí se queda (F2-118).
		if ahora, e := a.Store.Channel.Get(ctx, a.ChannelID); e == nil && ahora.Mode != ModoAire {
			espera, ultimoMotivo = EsperaMotorMin, ""
			continue
		}
		if time.Since(arranque) >= VidaBuena {
			espera = EsperaMotorMin
		}
		motivo := "el encoder terminó por su cuenta"
		if err != nil {
			motivo = err.Error()
		}
		texto := fmt.Sprintf("el motor se paró después de %s (%s) y vuelve en %s",
			time.Since(arranque).Round(time.Second), motivo, espera)
		// Un incidente por vuelta llenaría la bitácora el día que lo que
		// falte sea ffmpeg: el mismo motivo se anota una vez cada diez
		// minutos y, entre medias, se dice por el bus.
		if motivo != ultimoMotivo || time.Since(ultimoIncidente) > RepetirIncidente {
			a.Incident(model.IncEncoderReiniciado.String(), texto)
			ultimoMotivo, ultimoIncidente = motivo, time.Now()
		} else {
			a.Publish("motor", "reintento", texto)
		}
		if !dormir(ctx, espera) {
			return ctx.Err()
		}
		if espera *= 2; espera > EsperaMotorMax {
			espera = EsperaMotorMax
		}
	}
}

// dormir espera d o hasta que se cancele el contexto. Devuelve false si lo
// que llegó fue la cancelación.
func dormir(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}

// dormirOCambioDeModo es dormir, pero se levanta también si alguien cambió el
// modo del canal: así «salir al aire» enciende cuando se aprieta el botón y
// no en el próximo repaso.
func (a *App) dormirOCambioDeModo(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-a.modoCambio:
		return true
	case <-time.After(d):
		return true
	}
}

// vigilarElModo para el motor cuando el canal deja de estar al aire. Sin esto,
// devolver el canal a sombra no apagaría nada: correrMotor no vuelve hasta que
// el encoder se muere solo, así que la señal seguiría saliendo (F2-118).
func (a *App) vigilarElModo(ctx context.Context, parar context.CancelFunc) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-a.modoCambio:
		case <-time.After(MotorPoll):
		}
		if ctx.Err() != nil {
			return
		}
		ch, err := a.Store.Channel.Get(ctx, a.ChannelID)
		if err != nil || ch.Mode == ModoAire {
			continue
		}
		a.Publish("motor", "apagado",
			"el canal volvió a modo sombra: el motor se para y la señal deja de salir")
		parar()
		return
	}
}

// correrMotor enciende el encoder y el servidor de cuadros, y no vuelve hasta
// que algo se rompe o se pide parar. Una vida del encoder por llamada: si
// muere —o si el vigilante lo mata por colgado— quien llama espera y entra
// otra vez (F2-11).
func (a *App) correrMotor(ctx context.Context, ch model.Channel) error {
	if a.FFmpeg == "" {
		return a.FFmpegErr
	}
	formato := FormatOf(ch.FormatProfile)
	salidas, err := a.abrirSalidas(ctx, formato)
	if err != nil {
		return err
	}
	defer salidas.cerrar()

	// Con qué se comprime esta vida del encoder. Es lo que el canal tiene
	// guardado, salvo que el vigilante ya haya bajado el canal al procesador
	// porque la tarjeta dejó de responder: mientras eso esté forzado manda
	// eso, sin tocar lo que la persona escogió (F2-11).
	acel := a.AceleradorDelCanal(ctx)
	if forzado, porque := a.AceleradorEnCurso(); porque != "" {
		acel = forzado
	}
	for i := range salidas.outs {
		salidas.outs[i].Accel = acel
	}

	// Lo que quedó cargado de la vida anterior vuelve a planeado: el motor
	// recalcula desde el instante real y entra al archivo por donde toca,
	// nunca desde el principio (F2-13).
	if n, err := a.Store.Plan.ResetCuedToPlanned(ctx, a.ChannelID); err == nil && n > 0 {
		a.Publish("motor", "arranque", fmt.Sprintf(
			"%d bloques que estaban cargados vuelven a planeados; el que toque entra por el minuto que le corresponde", n))
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Los vigilantes de las salidas arrancan antes que el encoder: si el
	// encoder no llega a encender, cada salida queda con su motivo escrito.
	salidas.vigilar(ctx)
	// Y el que mira si alguien apagó el aire: apagar tiene que apagar.
	go a.vigilarElModo(ctx, cancel)
	enc, err := engine.StartEncoder(ctx, a.FFmpeg, formato, salidas.outs)
	if err != nil {
		salidas.avisar(err)
		return fmt.Errorf("no se pudo encender la salida: %w", err)
	}
	salidas.avisar(nil)
	a.UsandoAcelerador(acel)
	// Si el encoder se muere por su cuenta, el servidor de cuadros no puede
	// seguir escribiendo a un caño roto: se para y quien llama lo relanza.
	// El vigilante suelta enc.Done() en cuanto se cancela el contexto, porque
	// Finish lee ese mismo canal y si se lo queda el vigilante, Finish espera
	// un minuto por nada.
	muerto := make(chan error, 1)
	vigilado := make(chan struct{})
	go func() {
		defer close(vigilado)
		select {
		case err := <-enc.Done():
			muerto <- err
			cancel()
		case <-ctx.Done():
		}
	}()

	fuente := a.nuevaFuenteDelPlan(ctx, formato)
	// El vigilante de los tres segundos va pegado al encoder, por debajo del
	// detector de negro: lo que mide es si ffmpeg se traga los cuadros, y eso
	// solo se ve en la última escritura de la cadena (F2-11).
	srv := engine.NewServer(formato, a.Vigilar(ctx, formato, a.vigilarElEncoder(ctx, enc, acel)), fuente, fuente.Filler())
	srv.Ffmpeg, srv.Ffprobe = a.FFmpeg, a.FFprobe
	if f, err := a.registroDelMotor(); err == nil {
		srv.Events = f
		defer func() { _ = f.Close() }()
	}

	a.Publish("motor", "aire", fmt.Sprintf("el canal está al aire %s", salidas.texto()))
	runErr := srv.Run(ctx, 0)
	cancel()
	<-vigilado
	select {
	case err := <-muerto:
		// El encoder ya no está: no hay nada que cerrar bien.
		if err != nil {
			salidas.avisar(err)
			return fmt.Errorf("el encoder murió: %w", err)
		}
		salidas.avisar(errors.New("el encoder terminó solo"))
		return errors.New("el encoder terminó solo")
	default:
	}
	// Cerrar las entradas y dejar que ffmpeg vacíe sus colas: matarlo a mitad
	// deja el último trozo de la salida truncado.
	if err := enc.Finish(); err != nil && runErr == nil {
		return err
	}
	return runErr
}

// registroDelMotor abre el archivo donde el servidor de cuadros deja lo que
// hizo: un JSON por línea, uno por día. Es la evidencia de F2-01 y lo que
// medirá la prueba de resistencia (T10).
func (a *App) registroDelMotor() (*os.File, error) {
	dir := filepath.Join(a.DataDir, "motor")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	nombre := fmt.Sprintf("eventos-%s.jsonl", a.Now().Format("2006-01-02"))
	return os.OpenFile(filepath.Join(dir, nombre), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
}

// ── el vigilante del encoder colgado (F2-11) ──────────────────────────
//
// Un encoder que muere se nota solo: enc.Done() suelta el error y correrMotor
// vuelve. Un encoder **colgado** es otra cosa y es peor: el proceso sigue
// vivo, no consume un solo cuadro, y como no lee su socket, la escritura del
// servidor de cuadros se queda bloqueada para siempre. Desde fuera no se ve
// nada: ni error, ni proceso muerto, ni salida que se corte — solo un aire
// congelado. Por eso lo único que hay que mirar es cuánto lleva bloqueada la
// escritura que está en vuelo.

// encoderVigilado es el encoder con un reloj encima: apunta cuándo empezó la
// escritura que todavía no ha vuelto. No mide cuadros por segundo ni nada
// parecido, porque el síntoma no es lentitud: es una escritura que no vuelve.
type encoderVigilado struct {
	destino engine.Sink

	mu      sync.Mutex
	empezo  time.Time // cuándo empezó la escritura que está en vuelo
	enVuelo bool
}

func (v *encoderVigilado) WriteFrame(fr []byte) error {
	return v.escribir(func() error { return v.destino.WriteFrame(fr) })
}

func (v *encoderVigilado) WriteAudio(pcm []byte) error {
	return v.escribir(func() error { return v.destino.WriteAudio(pcm) })
}

func (v *encoderVigilado) escribir(mandar func() error) error {
	v.mu.Lock()
	v.empezo, v.enVuelo = time.Now(), true
	v.mu.Unlock()
	err := mandar()
	v.mu.Lock()
	v.enVuelo = false
	v.mu.Unlock()
	return err
}

// atascado es cuánto lleva bloqueada la escritura en vuelo, y cero si no hay
// ninguna. Que no haya escritura en vuelo nunca cuenta como colgado: el
// servidor de cuadros puede estar abriendo el clip que viene, y relanzar
// ffmpeg por eso sería el remedio peor que la enfermedad.
func (v *encoderVigilado) atascado(ahora time.Time) time.Duration {
	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.enVuelo {
		return 0
	}
	return ahora.Sub(v.empezo)
}

// laTarjeta es lo que el vigilante recuerda de una vida del encoder a la
// siguiente: cuándo se colgó y hasta cuándo el aire sale con el cartel. Es una
// variable de paquete, como controlDelAire en vigilancia.go y por lo mismo: un
// proceso, un canal (PRD §14.1).
var laTarjeta struct {
	mu       sync.Mutex
	colgadas []time.Time
	cartel   time.Time
}

// apuntarColgada anota una colgada y dice cuántas van dentro de la ventana de
// diez minutos. De paso abre la gracia del cartel, que es lo que va a salir en
// cuanto el encoder nuevo encienda.
func apuntarColgada(ahora time.Time) int {
	laTarjeta.mu.Lock()
	defer laTarjeta.mu.Unlock()
	vivas := laTarjeta.colgadas[:0]
	for _, t := range laTarjeta.colgadas {
		if ahora.Sub(t) <= VentanaDeLaTarjeta {
			vivas = append(vivas, t)
		}
	}
	laTarjeta.colgadas = append(vivas, ahora)
	laTarjeta.cartel = ahora.Add(GraciaDelCartel)
	return len(laTarjeta.colgadas)
}

// elCartelManda dice si el aire todavía tiene que salir con el cartel después
// de una colgada, y cuánto le queda. Se mide contra el reloj de pared a
// propósito: la gracia cruza de una vida del encoder a la siguiente, y el
// reloj del aire empieza de cero en cada una.
func elCartelManda() (time.Duration, bool) {
	laTarjeta.mu.Lock()
	defer laTarjeta.mu.Unlock()
	queda := time.Until(laTarjeta.cartel)
	return queda, queda > 0
}

// plazoDelEncoder es lo que se le da al encoder para tragarse un cuadro antes
// de darlo por colgado. De fábrica tres segundos (F2-11); lo que no se
// entiende o se sale de rango vale el de fábrica, que un ajuste escrito con un
// dedo torcido no puede apagar el vigilante.
func (a *App) plazoDelEncoder(ctx context.Context) time.Duration {
	n, err := strconv.Atoi(a.setting(ctx, KeyEncoderColgadoS))
	if err != nil {
		return EncoderColgado
	}
	d := time.Duration(n) * time.Second
	if d < ColgadoMinimo || d > ColgadoMaximo {
		return EncoderColgado
	}
	return d
}

// vigilarElEncoder pone el reloj entre el servidor de cuadros y el encoder, y
// deja corriendo el vigilante mientras viva esta vida del motor. Devuelve el
// Sink que hay que darle al servidor en lugar del encoder.
func (a *App) vigilarElEncoder(ctx context.Context, enc *engine.Encoder, acel engine.Acelerador) engine.Sink {
	v := &encoderVigilado{destino: enc}
	plazo := a.plazoDelEncoder(ctx)
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		for {
			if !dormir(ctx, LatidoDelVigilante) {
				return
			}
			if v.atascado(time.Now()) < plazo {
				continue
			}
			a.elEncoderSeColgo(enc, acel, plazo)
			// Una vez por vida del encoder: matarlo ya hace que correrMotor
			// vuelva y que motorLoop encienda otro con su propio vigilante.
			return
		}
	}()
	return v
}

// elEncoderSeColgo es lo que pasa cuando el encoder lleva el plazo entero sin
// tragarse un cuadro: queda el incidente, el cartel toma el aire, el proceso
// colgado se mata —al morir, la escritura bloqueada devuelve error y el motor
// entra otra vez con un encoder nuevo y el mismo acelerador— y, si es la
// segunda vez en diez minutos, el canal sigue emitiendo por software (F2-11).
func (a *App) elEncoderSeColgo(enc *engine.Encoder, acel engine.Acelerador, plazo time.Duration) {
	veces := apuntarColgada(time.Now())
	texto := fmt.Sprintf(
		"la salida de video dejó de aceptar imagen durante %s: sale el cartel de la estación y el canal vuelve solo",
		plazo.Round(time.Second))
	// Dos veces en diez minutos ya no es mala suerte. El canal se queda con el
	// procesador, que es más lento y nunca falla, y se dice por qué en palabras
	// claras: quien lo lee no es técnico.
	if veces >= ColgadasParaSoftware && acel.Resolver() != engine.AcelSoftware {
		a.ForzarAcelerador(engine.AcelSoftware, FraseTarjetaCaida)
		texto = fmt.Sprintf("%s (%d veces en %s): el canal sigue emitiendo con el procesador, que es más lento pero no falla",
			FraseTarjetaCaida, veces, VentanaDeLaTarjeta)
	}
	a.Incident(model.IncEncoderReiniciado.String(), texto)
	enc.Matar()
}

// ── la fuente: del plan a clips ───────────────────────────────────────

// fuenteDelPlan es el engine.ClipSource de verdad. Contesta tres preguntas y
// nada más: qué sale ahora, hasta cuándo, y qué relleno hay puesto.
//
// Todo lo que sabe de tiempo se lo dice el motor: `now` es el reloj del aire,
// no time.Now (F2-90).
type fuenteDelPlan struct {
	app     *App
	ctx     context.Context
	formato engine.Format
	cartel  engine.Clip // el cartel de la estación, hecho al arrancar el motor

	mu      sync.Mutex
	cargado map[int64]bool          // plan_item ya marcado como cargado
	fallos  map[int64]time.Time     // último fallo por media_asset (F2-12)
	dicho   map[string]bool         // lo que ya se dijo una vez
	decks   map[int64]model.Deck    // deck_id → deck, para saber quién manda
	avance  map[int64]time.Duration // por dónde va cada bloque que ya salió (F2-07)
	// Quién tiene el aire ahora mismo, para saber cuándo se pausa un bloque
	// y cuándo cambia de deck.
	sirviendo int64
	desde     time.Time     // instante del aire en que lo tomó
	posDesde  time.Duration // por dónde iba el bloque cuando lo tomó
	deck      model.DeckKind
}

// nuevaFuenteDelPlan deja la fuente lista, con un cartel de la estación a
// mano: la cascada tiene que terminar en algo, y ese algo nunca es negro
// (F2-09, F2-10).
func (a *App) nuevaFuenteDelPlan(ctx context.Context, formato engine.Format) *fuenteDelPlan {
	f := &fuenteDelPlan{
		app: a, ctx: ctx, formato: formato,
		cargado: map[int64]bool{},
		fallos:  map[int64]time.Time{},
		dicho:   map[string]bool{},
		decks:   map[int64]model.Deck{},
		avance:  map[int64]time.Duration{},
	}
	// Los cuatro decks del canal. Si la base no los tiene —una base a medio
	// migrar—, el motor sigue: sin decks, todo se trata como programa.
	if decks, err := a.Store.Deck.List(ctx, a.ChannelID); err == nil {
		for _, d := range decks {
			f.decks[d.ID] = d
		}
	}
	f.cartel = a.cartelALaMano(ctx, formato)
	return f
}

// prioridadDe es la prioridad del deck de un bloque: menor número, más
// prioridad (manual 0, comercial 1, programa 2, relleno 3 — PRD §9 paso 4).
// Un bloque cuyo deck no está en la base cuenta como programa, que es lo
// menos sorprendente.
func (f *fuenteDelPlan) prioridadDe(it model.PlanItem) int {
	if d, hay := f.decks[it.DeckID]; hay {
		return d.Priority
	}
	return model.DeckPriority[model.DeckProgram]
}

// deckDe es el tipo de deck de un bloque, para decirlo en la bitácora.
func (f *fuenteDelPlan) deckDe(it model.PlanItem) model.DeckKind {
	if d, hay := f.decks[it.DeckID]; hay {
		return d.Kind
	}
	return model.DeckProgram
}

// Next devuelve el clip que le toca al instante del aire y cuándo hay que
// volver a preguntar: el bloque del deck de más prioridad que cubra ese
// instante y, como corte, lo primero que le vaya a quitar el aire.
func (f *fuenteDelPlan) Next(now time.Time) (engine.Clip, time.Time, error) {
	// Menos cuando el encoder se acaba de colgar: entonces el aire vuelve con
	// el cartel de la estación unos segundos, porque lo que colgó a ffmpeg
	// pudo ser justo el clip que estaba saliendo (F2-11).
	if queda, manda := elCartelManda(); manda && f.cartel.Path != "" {
		f.sueltaElAire(now, model.DeckFiller, f.cartel.Name)
		return f.cartel, now.Add(queda), nil
	}
	items, err := f.app.Store.Plan.ListRange(f.ctx, f.app.ChannelID,
		now.Add(-VentanaAtras), now.Add(VentanaAdelante))
	if err != nil {
		return f.Filler(), now.Add(RellenoSuelto), err
	}

	item, hay := f.queToca(items, now)
	if !hay {
		// Hueco: sale el relleno hasta que empiece lo siguiente.
		f.sueltaElAire(now, model.DeckFiller, "relleno")
		return f.Filler(), f.hastaLoSiguiente(items, now), nil
	}
	hasta := f.corteDe(items, item, now)

	clip, err := f.clipDe(item, now)
	if err != nil {
		f.registrarFallo(item, err.Error())
		f.sueltaElAire(now, model.DeckFiller, "relleno")
		return f.Filler(), hasta, nil
	}
	f.tomaElAire(item, now, clip.Name)
	f.cargar(item)
	return clip, hasta, nil
}

// tomaElAire apunta que este bloque tiene el aire desde este instante, y
// cierra lo que salía antes: los milisegundos que de verdad salieron del
// bloque anterior quedan guardados, que es lo que hace que un programa pausado
// por un corte reanude donde iba (F2-07).
//
// Cada cambio de deck queda dicho una vez, con la hora: es la constancia de
// quién tenía el aire cuando algo pasó (PRD §9 paso 4).
func (f *fuenteDelPlan) tomaElAire(item model.PlanItem, now time.Time, nombre string) {
	deck := f.deckDe(item)
	pos := f.posicionDe(item, now)
	f.mu.Lock()
	if f.sirviendo == item.ID {
		f.mu.Unlock()
		return
	}
	f.cierraLoQueSalia(now)
	f.sirviendo, f.desde, f.posDesde = item.ID, now, pos
	cambio := f.deck != deck
	f.deck = deck
	f.mu.Unlock()

	if cambio {
		f.app.Publish("motor", "deck", fmt.Sprintf("a las %s el aire lo toma el deck %s: %s",
			now.Format("15:04:05"), deck, nombre))
	}
}

// sueltaElAire apunta que ningún bloque del plan tiene el aire: lo que sale es
// el relleno.
func (f *fuenteDelPlan) sueltaElAire(now time.Time, deck model.DeckKind, nombre string) {
	f.mu.Lock()
	f.cierraLoQueSalia(now)
	f.sirviendo = 0
	cambio := f.deck != deck
	f.deck = deck
	f.mu.Unlock()

	if cambio {
		f.app.Publish("motor", "deck", fmt.Sprintf("a las %s el aire lo toma el deck %s: %s",
			now.Format("15:04:05"), deck, nombre))
	}
}

// cierraLoQueSalia apunta por dónde se quedó el bloque que salía: por donde
// iba cuando tomó el aire más lo que estuvo al aire. Se cuenta sobre la línea
// del propio bloque y no sobre la hora de pared, que es lo que hace que
// pausar y reanudar cuadre al milisegundo. Se llama con el candado tomado.
func (f *fuenteDelPlan) cierraLoQueSalia(now time.Time) {
	if f.sirviendo == 0 || f.desde.IsZero() {
		return
	}
	if d := now.Sub(f.desde); d > 0 {
		f.avance[f.sirviendo] = f.posDesde + d
	}
}

// posicionDe es por dónde tiene que entrar un bloque: por donde se quedó si ya
// salió antes —un corte comercial lo pausó (F2-07)— y, si nunca salió, por lo
// que haya pasado desde su hora, que es el caso del servicio que se reinició a
// mitad de programa (F2-13).
//
// Una señal en vivo no se pausa nunca: mientras sale el corte sigue corriendo
// por debajo, así que al volver se entra por su instante actual y esos minutos
// se pierden (F2-08).
func (f *fuenteDelPlan) posicionDe(item model.PlanItem, now time.Time) time.Duration {
	if item.Origin == model.OriginLiveSource {
		return 0
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if d, ya := f.avance[item.ID]; ya {
		return d
	}
	return now.Sub(item.PlannedAt)
}

// queToca elige el plan_item que cubre el instante. La regla es la de los
// decks (PRD §9 paso 4): de todo lo que está en curso, el aire lo tiene el
// **deck de más prioridad** —manual, comercial, programa, relleno—, y por eso
// un corte pautado a las 14:00:00 le quita el aire al programa a esa hora
// exacta sin esperar a que el programa llegue a su corte natural (F2-06).
//
// Dentro del mismo deck: un elemento programado `dentro_de` otro manda sobre
// el que lo contiene (F2-16) y, si quedan dos —el esquema lo impide, esto es
// el cinturón además del tirante—, sale el de menor id y queda el incidente.
func (f *fuenteDelPlan) queToca(items []model.PlanItem, now time.Time) (model.PlanItem, bool) {
	var elegido model.PlanItem
	var hay, solapa bool
	for _, it := range items {
		if !enCurso(it, now) {
			continue
		}
		if !hay {
			elegido, hay = it, true
			continue
		}
		switch {
		case f.prioridadDe(it) < f.prioridadDe(elegido):
			elegido = it
		case f.prioridadDe(it) > f.prioridadDe(elegido):
			// se queda el que ya estaba: su deck manda sobre este
		// Del mismo deck: uno dentro del otro no es un solape, es el
		// elemento que interrumpe.
		case it.Inside != nil && elegido.Inside == nil:
			elegido = it
		case it.Inside == nil && elegido.Inside != nil:
			// se queda el que ya estaba
		default:
			solapa = true
			if it.ID < elegido.ID {
				elegido = it
			}
		}
	}
	if solapa && f.primeraVez("solape", elegido.ID) {
		f.app.Incident(model.IncSolape.String(), fmt.Sprintf(
			"dos bloques querían salir a las %s; sale el de menor id (%d)",
			now.Format("15:04:05"), elegido.ID))
	}
	return elegido, hay
}

// enCurso dice si el ítem cubre el instante y todavía puede salir.
func enCurso(it model.PlanItem, now time.Time) bool {
	if it.State != model.Planned && it.State != model.Cued {
		return false
	}
	return !it.PlannedAt.After(now) && it.End().After(now)
}

// corteDe es hasta cuándo se puede dar por bueno lo que sale: el fin del ítem
// o, si algo le va a quitar el aire antes, ese instante. Quitan el aire tres
// cosas: un bloque de un deck de más prioridad —el corte comercial de las
// 14:00 sobre el programa (F2-06)—, el siguiente del mismo deck, y un elemento
// programado dentro de este (F2-16). Un bloque de menos prioridad que empiece
// en medio no corta nada: espera su turno.
func (f *fuenteDelPlan) corteDe(items []model.PlanItem, item model.PlanItem, now time.Time) time.Time {
	corte := item.End()
	mio := f.prioridadDe(item)
	for _, it := range items {
		if it.ID == item.ID || it.State != model.Planned && it.State != model.Cued {
			continue
		}
		if !it.PlannedAt.After(now) || !it.PlannedAt.Before(corte) {
			continue
		}
		dentro := it.Inside != nil && *it.Inside == item.ID
		if dentro || f.prioridadDe(it) <= mio {
			corte = it.PlannedAt
		}
	}
	return corte
}

// hastaLoSiguiente es cuánto puede durar el relleno de un hueco: hasta que
// empiece el próximo bloque del plan.
func (f *fuenteDelPlan) hastaLoSiguiente(items []model.PlanItem, now time.Time) time.Time {
	hasta := now.Add(RellenoSuelto)
	for _, it := range items {
		if it.State != model.Planned && it.State != model.Cued {
			continue
		}
		if it.PlannedAt.After(now) && it.PlannedAt.Before(hasta) {
			hasta = it.PlannedAt
		}
	}
	return hasta
}

// clipDe traduce un plan_item al archivo que sale al aire: la copia
// normalizada del media_asset, que es la única que está en el formato de
// casa y con el volumen del canal. Si no hay copia normalizada —y nadie la
// dejó pasar a mano— este bloque no puede salir, y quien llama cae a la
// cascada.
func (f *fuenteDelPlan) clipDe(item model.PlanItem, now time.Time) (engine.Clip, error) {
	switch item.Origin {
	case model.OriginSlate:
		c := f.Filler()
		c.Ref = item.ID
		return c, nil
	case model.OriginLiveSource:
		// Las fuentes en vivo son otra tanda. Hasta entonces la franja sale
		// con relleno y se dice una vez, no cada vez que se pregunta.
		if f.primeraVez("vivo", item.ID) {
			f.app.Publish("motor", "plan", fmt.Sprintf(
				"el bloque de las %s es una fuente en vivo y el vivo todavía no está construido: sale el relleno",
				item.PlannedAt.Format("15:04")))
		}
		return engine.Clip{}, errors.New("las fuentes en vivo todavía no están construidas")
	case model.OriginAsset, model.OriginFiller:
	default:
		return engine.Clip{}, fmt.Errorf("origen %q que el motor no sabe poner", item.Origin)
	}
	if item.MediaAssetID == nil {
		return engine.Clip{}, errors.New("el bloque no dice qué archivo hay que poner")
	}
	asset, err := f.app.Store.Media.Get(f.ctx, *item.MediaAssetID)
	if err != nil {
		return engine.Clip{}, err
	}
	ruta := asset.NormalizedPath
	if ruta == "" && asset.LetThroughBy != "" {
		// Alguien lo dejó pasar bajo su responsabilidad: sale el original
		// tal cual, que es lo que dice Biblioteca.
		ruta = asset.Path
	}
	if ruta == "" {
		return engine.Clip{}, errors.New("el archivo todavía no está preparado para el aire")
	}
	if st, err := os.Stat(ruta); err != nil {
		return engine.Clip{}, fmt.Errorf("el archivo no está donde debería: %w", err)
	} else if st.Size() == 0 {
		return engine.Clip{}, errors.New("el archivo está en cero")
	}

	clip := engine.Clip{Path: ruta, Name: f.app.TituloDelArchivo(f.ctx, asset), Ref: item.ID}
	// Entrar por el medio: el bloque se pausó porque entró un corte (F2-07),
	// un elemento de dentro le quitó el aire un momento (F2-16), o el
	// servicio se reinició a mitad de programa (F2-13). En los tres casos se
	// entra por donde el bloque iba, no por la hora de pared.
	// Al milisegundo más cercano: el reloj del aire trae fracciones y truncar
	// iría dejando un milisegundo de menos en cada pausa.
	if dentro := f.posicionDe(item, now).Round(time.Millisecond); dentro >= SeekMinimo {
		clip.SeekMs = dentro.Milliseconds()
	}
	return clip, nil
}

// Filler es el relleno vigente: lo primero de la biblioteca de relleno y, si
// está vacía, el cartel de la estación. Nunca devuelve nada vacío mientras
// el cartel exista.
func (f *fuenteDelPlan) Filler() engine.Clip {
	fillers, err := f.app.Store.Filler.List(f.ctx, f.app.ChannelID)
	if err == nil {
		for _, fa := range fillers {
			asset, err := f.app.Store.Media.Get(f.ctx, fa.MediaAssetID)
			if err != nil {
				continue
			}
			ruta := asset.AirablePath()
			if ruta == "" {
				continue
			}
			if st, err := os.Stat(ruta); err != nil || st.Size() == 0 {
				continue
			}
			return engine.Clip{Path: ruta, Name: "relleno · " + f.app.TituloDelArchivo(f.ctx, asset)}
		}
	}
	return f.cartel
}

// ── lo que el motor cuenta de vuelta ──────────────────────────────────

// ClipSalio cierra el as-run del bloque que acaba de salir: el instante real,
// lo que de verdad duró, y si salió entero. Es quien llama por fin a
// MarkAired, que estaba escrita desde F1 esperando al motor.
func (f *fuenteDelPlan) ClipSalio(clip engine.Clip, arranque time.Time, cuadros int64, entero bool) {
	if clip.Ref == 0 {
		return
	}
	ms := int64(float64(cuadros) * f.formato.FrameDurationNs() / 1e6)
	if err := f.app.MarkAired(f.ctx, clip.Ref, arranque, ms, !entero); err != nil && f.ctx.Err() == nil {
		f.app.Publish("motor", "as-run", fmt.Sprintf("no pude anotar lo que salió del bloque %d: %v", clip.Ref, err))
	}
}

// ClipFallo es lo que el motor vio cuando un archivo no dio lo que decía:
// dispara la cascada (ya la disparó) y cuenta el fallo para F2-12.
func (f *fuenteDelPlan) ClipFallo(clip engine.Clip, motivo string) {
	if clip.Ref == 0 {
		return
	}
	item, err := f.app.PlanItem(f.ctx, clip.Ref)
	if err != nil {
		return
	}
	f.registrarFallo(item, motivo)
}

// registrarFallo deja el incidente, marca el bloque como fallido y, si es el
// segundo fallo del mismo archivo separado por más de cinco minutos, lo manda
// a cuarentena solo: para que no se programe otra vez la semana que viene y
// falle igual (F2-12).
func (f *fuenteDelPlan) registrarFallo(item model.PlanItem, motivo string) {
	f.app.Incident(model.IncFalloDeClip.String(), fmt.Sprintf(
		"el bloque de las %s no pudo salir (%s); lo cubrió el relleno",
		item.PlannedAt.Format("15:04"), motivo))
	if err := f.app.Store.Plan.SetState(f.ctx, item.ID, model.Failed); err != nil && f.ctx.Err() == nil {
		f.app.Publish("motor", "plan", fmt.Sprintf("no pude marcar el bloque %d como fallido: %v", item.ID, err))
	}
	if item.MediaAssetID == nil {
		return
	}
	id := *item.MediaAssetID
	ahora := f.app.Now()

	f.mu.Lock()
	antes, habia := f.fallos[id]
	f.fallos[id] = ahora
	f.mu.Unlock()
	if !habia || ahora.Sub(antes) <= FalloRepetido {
		// Un solo parpadeo de red no saca de la parrilla un programa bueno.
		return
	}
	if err := f.app.Store.Media.SetState(f.ctx, id, model.AssetQuarantine,
		"falló dos veces al aire: "+motivo); err != nil {
		return
	}
	f.app.Incident(model.IncCuarentena.String(), fmt.Sprintf(
		"el archivo del bloque de las %s falló dos veces al aire y queda parado: %s",
		item.PlannedAt.Format("15:04"), motivo))
	f.app.RefreshCuarentena(f.ctx)
}

// cargar marca el bloque como cargado (cued) la primera vez que sale. Lo que
// quede cargado si el servicio se reinicia vuelve a planeado (F2-13).
func (f *fuenteDelPlan) cargar(item model.PlanItem) {
	f.mu.Lock()
	ya := f.cargado[item.ID]
	f.cargado[item.ID] = true
	f.mu.Unlock()
	if ya || item.State == model.Cued {
		return
	}
	if err := f.app.Store.Plan.SetState(f.ctx, item.ID, model.Cued); err != nil && f.ctx.Err() == nil {
		f.app.Publish("motor", "plan", fmt.Sprintf("no pude marcar el bloque %d como cargado: %v", item.ID, err))
	}
}

// primeraVez dice si algo pasa por primera vez con ese bloque. La fuente se
// consulta muchas veces por bloque; la bitácora no tiene por qué saberlo.
func (f *fuenteDelPlan) primeraVez(que string, id int64) bool {
	clave := fmt.Sprintf("%s#%d", que, id)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.dicho[clave] {
		return false
	}
	f.dicho[clave] = true
	return true
}

// ── el cartel de emergencia ───────────────────────────────────────────

// cartelALaMano devuelve el cartel de la estación que puede salir ahora
// mismo: el que dejó el asistente si está, y si no uno que se dibuja en el
// acto en la carpeta de datos. Es el último escalón de la cascada y tiene que
// existir siempre (F2-09, F2-10, F2-69).
func (a *App) cartelALaMano(ctx context.Context, formato engine.Format) engine.Clip {
	nombre := "cartel de la estación"
	if ruta := a.setting(ctx, KeyDefaultFiller); ruta != "" {
		if st, err := os.Stat(ruta); err == nil && st.Size() > 0 {
			return engine.Clip{Path: ruta, Name: nombre}
		}
	}
	ruta := filepath.Join(a.DataDir, "motor", ArchivoDelCartel)
	if st, err := os.Stat(ruta); err == nil && st.Size() > 0 {
		return engine.Clip{Path: ruta, Name: nombre}
	}
	if a.FFmpeg == "" {
		return engine.Clip{}
	}
	ch, err := a.Store.Channel.Get(ctx, a.ChannelID)
	if err != nil {
		return engine.Clip{}
	}
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		return engine.Clip{}
	}
	cuadro := ruta + ".png"
	defer func() { _ = os.Remove(cuadro) }()
	if err := DibujarCartel(ch, formato, cuadro); err != nil {
		a.Incident(model.IncCartel.String(), "no pude dibujar el cartel de la estación: "+err.Error())
		return engine.Clip{}
	}
	if err := correrFFmpegDelCartel(ctx, a.FFmpeg, cuadro, ruta, formato); err != nil {
		a.Incident(model.IncCartel.String(), "no pude preparar el cartel de la estación: "+err.Error())
		return engine.Clip{}
	}
	return engine.Clip{Path: ruta, Name: nombre}
}
