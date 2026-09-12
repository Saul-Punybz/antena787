// manual.go es el aire en manos de una persona: tomarlo, disparar, y
// soltarlo — por botón, por fin de bloque, por silencio o porque se cayó el
// servicio (PRD §9 paso 6, tanda T5 de docs/f2/PLAN-F2.md).
//
// Hasta hoy el botón «Tomar el control» existía en la pantalla y no llamaba a
// nada, y la tabla `manual_hold` no la tocaba una sola línea de código. Eso
// estaba dicho en el documento que se le entregó al cliente —«el botón está
// pero todavía no hace nada»— y ya no hace falta decirlo.
//
// **La regla que gobierna todo esto**: mientras una persona tiene el aire, el
// plan no lo toca. Los bloques cuya hora pasa durante la retención no salen y
// quedan marcados `manual_hold`, que es distinto de `aired` y distinto de
// `skipped`: no es que se saltaran, es que había alguien al mando (F2-62).
package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"antena787/internal/model"
)

// EsperaAlSoltar es lo máximo que «volver al automático» espera a que termine
// lo que está sonando antes de entregar el aire (F2-31). Soltar el control
// nunca corta nada por el medio; para cortar en seco está «parar todo».
const EsperaAlSoltar = 60 * time.Second

// retencion es lo que el motor necesita saber del control manual sin ir a la
// base en cada clip. `Next` se llama una vez por clip —a veces cada pocos
// segundos— y una consulta ahí es una consulta en el camino del aire.
type retencion struct {
	abierta bool
	id      int64
	quien   string
	desde   time.Time

	// hastaFinDeBloque es cuándo se acaba el bloque que estaba al aire al
	// tomar el control. Llegada esa hora, el aire vuelve solo al automático
	// sin que nadie pulse nada (F2-32). Cero = se tomó el control en un
	// hueco, y entonces no hay bloque que se acabe.
	hastaFinDeBloque time.Time

	// soltandoseEn es la hora en que se entrega el aire de verdad, cuando
	// alguien ya pulsó «volver al automático» y se está esperando a que
	// termine lo que suena (F2-31). Cero = nadie ha pulsado nada.
	soltandoseEn time.Time
}

// hastaAhoraMismo es el tope que hay que pedirle a Plan.ListRange para que
// entre lo que está al aire **en este instante**.
//
// El intervalo de ListRange es [from, to), así que pedir `to = ahora` deja
// fuera justo el bloque que empieza ahora — y un bloque que empieza ahora es
// exactamente lo que está al aire ahora. Un milisegundo más, que es la
// resolución que guarda la base, lo mete.
func hastaAhoraMismo(ahora time.Time) time.Time { return ahora.Add(time.Millisecond) }

// ── lo que sabe la aplicación ─────────────────────────────────────────

// CargarElManual pone al día lo que la aplicación recuerda del control
// manual, y **cierra lo que se quedó abierto**.
//
// Una retención abierta al arrancar solo puede significar una cosa: el
// servicio se cayó con una persona al mando. Se cierra con
// `caida_del_sistema` y no con `soltado`, porque nadie soltó nada y un
// registro que dijera lo contrario mentiría sobre lo que pasó esa noche
// (F2-79). Y como se cierra al arrancar, el canal **siempre** empieza en
// automático (F2-27).
func (a *App) CargarElManual(ctx context.Context) {
	n, err := a.Store.Manual.CerrarLasQueQuedaron(ctx, a.ChannelID, a.Now())
	if err != nil {
		a.Publish("manual", "arranque", "no pude revisar si alguien tenía el control: "+err.Error())
	}
	if n > 0 {
		a.Incident(model.IncManualPorTimeout.String(),
			"el servicio se reinició con el aire en manual; el control volvió solo al automático")
		a.Publish("manual", "caida",
			"el aire estaba en manual cuando se reinició el servicio: volvió al automático")
	}
	a.manualMu.Lock()
	a.manual = retencion{}
	a.manualMu.Unlock()
}

// EnManual dice si una persona tiene el aire retenido ahora mismo. Es la
// mitad de app.ControlDelAire, el hueco que T3 dejó preparado para esto.
func (a *App) EnManual() bool {
	a.manualMu.RLock()
	defer a.manualMu.RUnlock()
	return a.manual.abierta
}

// QuienTieneElControl es quién y desde cuándo, o vacío si nadie.
func (a *App) QuienTieneElControl() (string, time.Time, bool) {
	a.manualMu.RLock()
	defer a.manualMu.RUnlock()
	return a.manual.quien, a.manual.desde, a.manual.abierta
}

// ── tomar ─────────────────────────────────────────────────────────────

// TomarElControl le da el aire a una persona.
//
// Falla con *store.ErrOtroTieneElControl si ya hay alguien, y ese error trae
// dentro quién y desde cuándo porque es lo primero que pregunta la segunda
// persona (F2-77). Para quitárselo hay otra puerta, explícita, que deja
// constancia.
func (a *App) TomarElControl(ctx context.Context, quien string) (model.ManualHold, error) {
	quien = strings.TrimSpace(quien)
	if quien == "" {
		return model.ManualHold{}, errors.New("escribe tu nombre: queda anotado quién tomó el aire")
	}
	ahora := a.Now()
	h, err := a.Store.Manual.Tomar(ctx, a.ChannelID, quien, ahora)
	if err != nil {
		return model.ManualHold{}, err
	}
	a.abrirLaRetencion(ctx, h, ahora)
	a.Publish("manual", "tomado", quien+" tomó el control del aire")
	return h, nil
}

// QuitarleElControl se lo quita a quien lo tenga y se lo queda. Es una puerta
// aparte de TomarElControl a propósito: quitarle el aire a otra persona no
// puede ser lo que pasa cuando alguien pulsa el mismo botón dos veces.
func (a *App) QuitarleElControl(ctx context.Context, quien string) (model.ManualHold, string, error) {
	quien = strings.TrimSpace(quien)
	if quien == "" {
		return model.ManualHold{}, "", errors.New("escribe tu nombre: queda anotado quién se lo quitó")
	}
	ahora := a.Now()
	h, aQuien, err := a.Store.Manual.Quitar(ctx, a.ChannelID, quien, ahora)
	if err != nil {
		return model.ManualHold{}, "", err
	}
	a.abrirLaRetencion(ctx, h, ahora)
	if aQuien != "" {
		a.Publish("manual", "quitado", quien+" le quitó el control del aire a "+aQuien)
	} else {
		a.Publish("manual", "tomado", quien+" tomó el control del aire")
	}
	return h, aQuien, nil
}

// abrirLaRetencion apunta en memoria lo que el motor necesita, incluido
// cuándo se acaba el bloque que está al aire: llegada esa hora el aire vuelve
// solo, sin que nadie pulse nada (F2-32).
func (a *App) abrirLaRetencion(ctx context.Context, h model.ManualHold, ahora time.Time) {
	a.manualMu.Lock()
	a.manual = retencion{
		abierta:          true,
		id:               h.ID,
		quien:            h.User,
		desde:            h.Start,
		hastaFinDeBloque: a.finDelBloqueEnCurso(ctx, ahora),
	}
	a.manualMu.Unlock()
}

// finDelBloqueEnCurso es cuándo se acaba lo que está al aire según el plan.
//
// Cero si lo que sale es **relleno**, y eso no es un detalle: en una
// instalación recién hecha el plan empieza con un bloque de cartel que puede
// durar días, y tomarlo por «el bloque» dejaría una vuelta automática
// programada para pasado mañana. De un hueco no se sale por fin de bloque —se
// sale soltándolo, o por silencio—. Lo enseñó el binario, no la suite.
func (a *App) finDelBloqueEnCurso(ctx context.Context, ahora time.Time) time.Time {
	items, err := a.Store.Plan.ListRange(ctx, a.ChannelID, ahora.Add(-VentanaAtras), hastaAhoraMismo(ahora))
	if err != nil {
		return time.Time{}
	}
	decks := a.decksPorID(ctx)
	var fin time.Time
	mejor := 99
	for _, it := range items {
		if !enCurso(it, ahora) || it.End().Before(ahora) {
			continue
		}
		d, hay := decks[it.DeckID]
		if hay && d.Kind == model.DeckFiller {
			continue
		}
		p := model.DeckPriority[model.DeckProgram]
		if hay {
			p = d.Priority
		}
		if p < mejor {
			mejor, fin = p, it.End()
		}
	}
	return fin
}

// decksPorID son los decks del canal indexados por su id.
func (a *App) decksPorID(ctx context.Context) map[int64]model.Deck {
	out := map[int64]model.Deck{}
	decks, err := a.Store.Deck.List(ctx, a.ChannelID)
	if err != nil {
		return out
	}
	for _, d := range decks {
		out[d.ID] = d
	}
	return out
}

// ── soltar ────────────────────────────────────────────────────────────

// SoltarElControl devuelve el aire al plan **sin cortar nada por el medio**:
// espera a que termine lo que está sonando, como máximo un minuto, y solo
// entonces entrega (F2-31). Para cortar en seco está PararTodo.
//
// Devuelve cuándo se entrega de verdad, que es lo que la pantalla enseña
// mientras tanto: un botón que parece no hacer nada durante cuarenta segundos
// se pulsa tres veces más.
func (a *App) SoltarElControl(ctx context.Context) (time.Time, error) {
	a.manualMu.Lock()
	if !a.manual.abierta {
		a.manualMu.Unlock()
		return time.Time{}, errors.New("el aire ya está en automático")
	}
	if !a.manual.soltandoseEn.IsZero() {
		cuando := a.manual.soltandoseEn
		a.manualMu.Unlock()
		return cuando, nil
	}
	a.manualMu.Unlock()

	ahora := a.Now()
	cuando := a.finDeLoQueSuena(ctx, ahora)
	tope := ahora.Add(EsperaAlSoltar)
	if cuando.IsZero() || cuando.After(tope) {
		cuando = tope
	}
	if !cuando.After(ahora) {
		return ahora, a.cerrarLaRetencion(ctx, model.FinSoltado, ahora)
	}

	a.manualMu.Lock()
	a.manual.soltandoseEn = cuando
	a.manualMu.Unlock()
	a.Publish("manual", "soltando", fmt.Sprintf(
		"el aire vuelve al automático cuando acabe lo que está sonando, a las %s",
		cuando.In(a.zonaDelCanal(ctx)).Format("3:04:05 PM")))
	return cuando, nil
}

// finDeLoQueSuena es cuándo acaba el bloque manual que está al aire. Cero si
// no hay ninguno: entonces no hay nada que esperar.
func (a *App) finDeLoQueSuena(ctx context.Context, ahora time.Time) time.Time {
	items, err := a.Store.Plan.ListRange(ctx, a.ChannelID, ahora.Add(-VentanaAtras), hastaAhoraMismo(ahora))
	if err != nil {
		return time.Time{}
	}
	deck, err := a.Store.Deck.ByKind(ctx, a.ChannelID, model.DeckManual)
	if err != nil {
		return time.Time{}
	}
	var fin time.Time
	for _, it := range items {
		if it.DeckID != deck.ID || !enCurso(it, ahora) {
			continue
		}
		if it.End().After(fin) {
			fin = it.End()
		}
	}
	return fin
}

// PararTodo corta el aire en seco, sin esperar a nada, y devuelve el plan.
//
// Es lo contrario de soltar: aquí **sí** se corta por el medio, porque el
// caso de uso es que está saliendo algo que no tenía que salir. Lo que se
// quedó a medias queda marcado `parcial`, que es lo que distingue este corte
// de una emisión completa cuando alguien vaya a facturar el spot (F2-78).
func (a *App) PararTodo(ctx context.Context, quien string) error {
	a.manualMu.RLock()
	abierta := a.manual.abierta
	a.manualMu.RUnlock()
	if !abierta {
		return errors.New("el aire ya está en automático")
	}
	ahora := a.Now()
	n := a.marcarParcialLoQueSonaba(ctx, ahora)
	if err := a.cerrarLaRetencion(ctx, model.FinPararTodo, ahora); err != nil {
		return err
	}
	detalle := strings.TrimSpace(quien) + " paró el aire en seco y lo devolvió al automático"
	if n > 0 {
		detalle += fmt.Sprintf("; %d bloque(s) quedaron a medias", n)
	}
	a.Publish("manual", "parado", strings.TrimSpace(detalle))
	return nil
}

// marcarParcialLoQueSonaba deja constancia de que lo que estaba al aire se
// cortó por el medio. Devuelve cuántos bloques se marcaron.
func (a *App) marcarParcialLoQueSonaba(ctx context.Context, ahora time.Time) int {
	items, err := a.Store.Plan.ListRange(ctx, a.ChannelID, ahora.Add(-VentanaAtras), hastaAhoraMismo(ahora))
	if err != nil {
		return 0
	}
	deck, err := a.Store.Deck.ByKind(ctx, a.ChannelID, model.DeckManual)
	if err != nil {
		return 0
	}
	n := 0
	for _, it := range items {
		if it.DeckID != deck.ID || !enCurso(it, ahora) {
			continue
		}
		salio := ahora.Sub(it.PlannedAt).Milliseconds()
		if salio < 0 {
			salio = 0
		}
		if err := a.Store.Plan.MarkAired(ctx, it.ID, it.PlannedAt, salio, true); err == nil {
			n++
		}
	}
	return n
}

// VolverAlAutomatico es la otra mitad de app.ControlDelAire: la que llama la
// vigilancia cuando el aire lleva más del umbral en silencio o en negro
// (F2-30). Aquí **no se espera a nada**: si lo que sale es silencio, esperar
// a que el silencio termine es esperar para siempre.
func (a *App) VolverAlAutomatico(ctx context.Context, motivo string) error {
	a.manualMu.RLock()
	abierta := a.manual.abierta
	a.manualMu.RUnlock()
	if !abierta {
		return nil
	}
	if err := a.cerrarLaRetencion(ctx, model.FinPorTimeout, a.Now()); err != nil {
		return err
	}
	a.Publish("manual", "timeout", "el aire volvió solo al automático: "+motivo)
	return nil
}

// cerrarLaRetencion acaba la retención con su motivo y marca los bloques que
// no salieron porque había alguien al mando (F2-62).
func (a *App) cerrarLaRetencion(ctx context.Context, motivo string, cuando time.Time) error {
	a.manualMu.Lock()
	if !a.manual.abierta {
		a.manualMu.Unlock()
		return nil
	}
	id, desde := a.manual.id, a.manual.desde
	a.manual = retencion{}
	a.manualMu.Unlock()

	if err := a.Store.Manual.Cerrar(ctx, id, cuando, motivo); err != nil {
		return err
	}
	a.marcarLosQueNoSalieron(ctx, desde, cuando)
	return nil
}

// marcarLosQueNoSalieron pone `manual_hold` en los bloques cuya hora pasó
// entera mientras una persona tenía el aire.
//
// No es lo mismo que `skipped` y por eso no se reusa: `skipped` es un bloque
// que el sistema decidió no poner, y esto es un bloque que no salió porque
// había alguien al mando. Cuando alguien mire por qué no salió un spot
// pagado, la diferencia es la respuesta (F2-62).
func (a *App) marcarLosQueNoSalieron(ctx context.Context, desde, hasta time.Time) {
	items, err := a.Store.Plan.ListRange(ctx, a.ChannelID, desde, hasta)
	if err != nil {
		return
	}
	deck, errDeck := a.Store.Deck.ByKind(ctx, a.ChannelID, model.DeckManual)
	for _, it := range items {
		if errDeck == nil && it.DeckID == deck.ID {
			continue // los del propio operador sí salieron
		}
		if it.State != model.Planned && it.State != model.Cued {
			continue
		}
		// Solo los que se quedaron enteros dentro de la retención: uno que
		// empezó antes ya estaba al aire, y uno que acaba después todavía
		// puede salir por el minuto que le toca (F2-34).
		if it.PlannedAt.Before(desde) || it.End().After(hasta) {
			continue
		}
		_ = a.Store.Plan.SetState(ctx, it.ID, model.EnManual)
	}
}

// ── disparar ──────────────────────────────────────────────────────────

// DispararAlAire pone un archivo al aire **ahora**, en el deck manual, que es
// el de más prioridad: le quita el aire a lo que hubiera (PRD §9 paso 4).
//
// Se crea como un bloque del plan y no como un caso aparte a propósito: así
// sale por el mismo camino que todo lo demás, cuenta en el as-run, y lo que
// se emitió queda registrado igual que si lo hubiera puesto la parrilla.
func (a *App) DispararAlAire(ctx context.Context, assetID int64) (model.PlanItem, error) {
	a.manualMu.RLock()
	abierta := a.manual.abierta
	a.manualMu.RUnlock()
	if !abierta {
		return model.PlanItem{}, errors.New("para disparar algo hay que tener el control del aire")
	}

	asset, err := a.Store.Media.Get(ctx, assetID)
	if err != nil {
		return model.PlanItem{}, err
	}
	if asset.NormalizedPath == "" && asset.LetThroughBy == "" {
		return model.PlanItem{}, errors.New("ese archivo todavía no está preparado para el aire")
	}
	if asset.DurationMs <= 0 {
		return model.PlanItem{}, errors.New("ese archivo no dice cuánto dura: no se puede poner al aire")
	}
	deck, err := a.Store.Deck.ByKind(ctx, a.ChannelID, model.DeckManual)
	if err != nil {
		return model.PlanItem{}, err
	}
	ch, err := a.Store.Channel.Get(ctx, a.ChannelID)
	if err != nil {
		return model.PlanItem{}, err
	}

	ahora := a.Now()
	it := model.PlanItem{
		ChannelID:    a.ChannelID,
		DeckID:       deck.ID,
		BroadcastDay: ch.BroadcastDay(ahora),
		PlannedAt:    ahora,
		PlannedMs:    asset.DurationMs,
		Origin:       model.OriginAsset,
		MediaAssetID: &assetID,
		State:        model.Planned,
		LocalClock:   ahora.In(ch.Location()).Format("15:04"),
		// Fijado: lo puso una persona, así que el resolver no lo mueve ni lo
		// borra en su próxima corrida.
		Fijado: true,
	}
	if err := a.Store.Plan.Insert(ctx, &it); err != nil {
		return model.PlanItem{}, err
	}
	a.Publish("manual", "disparo", "al aire a mano: "+a.TituloDelArchivo(ctx, asset))
	return it, nil
}

// ── lo que el motor consulta en cada clip ─────────────────────────────

// revisarElManual cierra la retención si llegó su hora —el bloque se acabó
// (F2-32) o se pidió soltarla y ya pasó la espera (F2-31)— y contesta si el
// aire lo tiene una persona en este instante.
//
// Lo llama el motor dentro de Next, así que no puede ir a la base salvo
// cuando de verdad hay algo que cerrar.
func (a *App) revisarElManual(ctx context.Context, ahora time.Time) bool {
	a.manualMu.RLock()
	r := a.manual
	a.manualMu.RUnlock()
	if !r.abierta {
		return false
	}
	switch {
	case !r.soltandoseEn.IsZero() && !ahora.Before(r.soltandoseEn):
		if err := a.cerrarLaRetencion(ctx, model.FinSoltado, ahora); err == nil {
			a.Publish("manual", "soltado", "el aire volvió al automático")
		}
		return false
	case !r.hastaFinDeBloque.IsZero() && !ahora.Before(r.hastaFinDeBloque):
		if err := a.cerrarLaRetencion(ctx, model.FinDeBloque, ahora); err == nil {
			a.Publish("manual", "fin_de_bloque",
				"se acabó el bloque y el aire volvió solo al automático")
		}
		return false
	}
	return true
}

// EstadoDelManual es lo que enseña la pantalla: si el aire está en manual,
// de quién, desde cuándo, y si ya se pidió soltarlo.
type EstadoDelManual struct {
	EnManual bool       `json:"en_manual"`
	Quien    string     `json:"quien,omitempty"`
	Desde    *time.Time `json:"desde,omitempty"`
	// SoltandoseEn es cuándo se entrega el aire, cuando alguien ya pulsó
	// «volver al automático» y se está esperando a que acabe lo que suena.
	SoltandoseEn *time.Time `json:"soltandose_en,omitempty"`
	// FinDeBloque es cuándo vuelve solo al automático porque se acaba el
	// bloque que estaba al aire cuando se tomó el control (F2-32).
	FinDeBloque *time.Time `json:"fin_de_bloque,omitempty"`
}

// ElManual contesta el estado del control manual.
func (a *App) ElManual() EstadoDelManual {
	a.manualMu.RLock()
	defer a.manualMu.RUnlock()
	if !a.manual.abierta {
		return EstadoDelManual{}
	}
	e := EstadoDelManual{EnManual: true, Quien: a.manual.quien}
	desde := a.manual.desde
	e.Desde = &desde
	if !a.manual.soltandoseEn.IsZero() {
		t := a.manual.soltandoseEn
		e.SoltandoseEn = &t
	}
	if !a.manual.hastaFinDeBloque.IsZero() {
		t := a.manual.hastaFinDeBloque
		e.FinDeBloque = &t
	}
	return e
}

// zonaDelCanal es la zona horaria del canal, con UTC de respaldo.
func (a *App) zonaDelCanal(ctx context.Context) *time.Location {
	ch, err := a.Store.Channel.Get(ctx, a.ChannelID)
	if err != nil {
		return time.UTC
	}
	return ch.Location()
}

// controlManual es App vista como app.ControlDelAire, el hueco que T3 dejó
// preparado (vigilancia.go). Es un tipo aparte y no App directamente para que
// quede dicho en un sitio qué dos cosas hacen falta para ser «el control».
type controlManual struct{ app *App }

func (c controlManual) EnManual() bool { return c.app.EnManual() }

func (c controlManual) VolverAlAutomatico(ctx context.Context, motivo string) error {
	return c.app.VolverAlAutomatico(ctx, motivo)
}

var _ ControlDelAire = controlManual{}
