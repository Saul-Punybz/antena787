// vigilancia.go es quien decide con lo que el detector ve. El detector
// (internal/engine/detector.go) solo cuenta: negro empieza, negro sigue,
// negro termina, y lo mismo con el silencio. Aquí se sabe lo que él no puede
// saber: cuál es el umbral que fijó el canal, si el archivo que está saliendo
// abre en negro a propósito, y si el aire lo tiene un operador en la mano.
//
// La promesa es corta, y es la del PRD §9 pasos 4 y 6: **si el aire se queda
// mudo o en negro más de lo que dice el umbral, se avisa y se devuelve el
// control.** Diga lo que diga el plan (F2-51, F2-52). Y es **el mismo
// mecanismo** en automático y en manual: no hay dos detectores ni dos
// umbrales, solo cambia qué se hace al dispararse (F2-54).
//
// Dos cosas no disparan:
//
//   - Un silencio corto. Una pausa dramática de ocho segundos dentro de un
//     programa no es un canal muerto (F2-53): por eso el umbral de fábrica es
//     de quince segundos y el episodio se mide en cuadros y muestras que de
//     verdad salieron, no en hora de pared.
//   - Un clip marcado `negro_intencional`. Hay material que abre con veinte
//     segundos de negro y silencio a propósito, y mientras ese clip está al
//     aire el detector no dispara (F2-72). Una fuente en vivo nunca puede
//     desactivarlo: si un vivo se queda congelado en negro, eso es
//     justamente lo que hay que decir.
//
// Es la tanda T3 de docs/f2/PLAN-F2.md. En modo sombra no hace nada: la
// vigilancia vive y muere con el motor, y en sombra el motor no emite.
package app

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"antena787/internal/engine"
	"antena787/internal/model"
)

// Claves de ajustes de la vigilancia. Viven en settings como todo lo demás:
// no hay archivo de configuración (PRD §14.1).
const (
	// KeySilencioUmbral es cuántos segundos de silencio real en la salida
	// hacen falta para avisar. De fábrica quince (F2-51, F2-53).
	KeySilencioUmbral = "silencio_umbral_s"
	// KeyNegroUmbral es lo mismo para el negro. De fábrica quince (F2-52).
	// Es un ajuste aparte porque un canal puede querer ser más paciente con
	// la imagen que con el sonido, pero el de fábrica es el mismo número:
	// Ajustes enseña «el umbral de silencio y negro» como uno solo (PRD §13).
	KeyNegroUmbral = "negro_umbral_s"
	// KeySilencioDevuelveControl es el interruptor «Avisa y devuelve el
	// control» de Ajustes: cuando está encendido y el aire está en manual, al
	// pasar el umbral el sistema vuelve solo al automático (F2-30). Apagado,
	// avisa y no toca el aire. De fábrica encendido: la promesa del PRD es
	// que nada manual se queda colgado para siempre.
	KeySilencioDevuelveControl = "silencio_devuelve_control"
)

// Números de la vigilancia.
const (
	// UmbralSilencio y UmbralNegro son los de fábrica: quince segundos, que
	// es lo que dicen el PRD §9 paso 6 y los criterios F2-51 a F2-54 (ocho
	// segundos NO pueden disparar, F2-53).
	UmbralSilencio = 15 * time.Second
	UmbralNegro    = 15 * time.Second
	// UmbralMinimo y UmbralMaximo son lo que se acepta al guardar el ajuste.
	// Por debajo de tres segundos cualquier fundido a negro dispararía; por
	// encima de dos minutos el aviso llega cuando el televidente ya cambió de
	// canal. Son los mismos límites que ya pinta la pantalla de Ajustes.
	UmbralMinimo = 3 * time.Second
	UmbralMaximo = 120 * time.Second
	// VentanaVigilancia es cuánto plan se lee alrededor de ahora para saber
	// qué está saliendo. Igual que el motor: hacia atrás cubre un bloque
	// largo que empezó antes.
	VentanaVigilancia = 12 * time.Hour
)

// ControlDelAire es el gancho de T5 (manual.go): quién tiene el aire y cómo
// se devuelve al automático. La vigilancia no sabe cómo se cambia de clip —de
// eso sabe el motor— y no tiene por qué: solo pide que se vuelva, y dice por
// qué.
//
// Mientras T5 no exista, nadie lo instala y el gancho queda documentado y
// probado con un control de mentira: lo que se dispara entonces es el aviso,
// que es lo que el operador necesita ver igual.
type ControlDelAire interface {
	// EnManual dice si un operador tiene el aire retenido ahora mismo.
	EnManual() bool
	// VolverAlAutomatico suelta el control y devuelve el aire al plan,
	// entrando por el minuto que le toca (join-in-progress, F2-34).
	VolverAlAutomatico(ctx context.Context, motivo string) error
}

// controlDelAire es el control instalado, hecho variable de paquete como
// sostenerPorDefecto en despierto.go: T5 lo pondrá al arrancar su máquina de
// estados, y las pruebas de este archivo meten aquí uno de mentira. Un solo
// proceso, un solo canal (PRD §14.1), así que una variable alcanza.
var controlDelAire ControlDelAire

// PonerControlDelAire instala el control manual. Pasar nil lo quita, que es
// lo que hacen las pruebas al terminar.
func PonerControlDelAire(c ControlDelAire) { controlDelAire = c }

// Vigilar pone el detector entre el servidor de cuadros y el encoder, y deja
// la vigilancia corriendo mientras viva ctx. Devuelve el Sink que hay que
// darle al servidor de cuadros en lugar del encoder.
//
// Es el único gancho que hace falta en el motor, y es una línea:
//
//	srv := engine.NewServer(formato, a.Vigilar(ctx, formato, enc), fuente, fuente.Filler())
//
// (Hoy motor.go pasa `enc` directo; la línea la mete quien fusione T3, que es
// el dueño de ese archivo — ver docs/f2/PLAN-F2.md, regla de fusión.) La
// vigilancia vive exactamente lo que vive el motor: en modo sombra el motor
// no arranca, así que esto no se llama y no hay nada que apagar.
func (a *App) Vigilar(ctx context.Context, formato engine.Format, destino engine.Sink) engine.Sink {
	det := engine.NuevoDetector(formato, destino)
	det.Origen = a.Now()
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		// La misma protección contra pánico que las goroutines de casa
		// (auditoría A2): queda el incidente `panico_vigilancia` y vuelve.
		// No se usa a.guard porque esta goroutine no vive lo que vive la
		// aplicación, sino lo que vive esta vida del motor.
		for ctx.Err() == nil {
			err := a.runOnce(ctx, "vigilancia", func(c context.Context) error {
				return a.vigilanciaLoop(c, det)
			})
			if ctx.Err() != nil || err == nil || errors.Is(err, context.Canceled) {
				return
			}
			if !dormir(ctx, RelanzarVigilancia) {
				return
			}
		}
	}()
	return det
}

// RelanzarVigilancia es lo que se espera antes de volver a vigilar si la
// goroutine se cayó. Es más corta que RelaunchDelay a propósito: mientras no
// vigila, un canal mudo no se ve.
const RelanzarVigilancia = time.Second

// vigilanciaLoop consume los estados del detector y decide. No bloquea al
// detector nunca: lo único que hace en el bucle es leer el canal.
func (a *App) vigilanciaLoop(ctx context.Context, det *engine.Detector) error {
	neg := &episodioVigilado{que: "negro"}
	sil := &episodioVigilado{que: "silencio"}
	defer func() {
		// Al apagarse el motor se apagan las alarmas: lo que sale al aire ya
		// no es asunto nuestro, y una alarma huérfana en la pantalla es una
		// mentira.
		a.setAlarms("vigilancia", nil)
		a.cerrarEpisodio(ctx, neg)
		a.cerrarEpisodio(ctx, sil)
	}()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case e := <-det.Estados():
			a.atender(ctx, e, neg, sil)
		}
	}
}

// episodioVigilado es lo que la vigilancia recuerda de un tramo abierto.
type episodioVigilado struct {
	que string // "negro" | "silencio"
	// abierto dice que el detector tiene el episodio abierto.
	abierto bool
	// umbral es el que regía cuando empezó: cambiar el ajuste a mitad de un
	// silencio no mueve la portería.
	umbral time.Duration
	// permitido dice que el clip que está saliendo está marcado
	// `negro_intencional` (F2-72): se mira una vez, al abrirse el episodio.
	// Vale para los dos episodios —el del negro y el del silencio—, porque
	// el material que abre en negro a propósito abre mudo a propósito, y así
	// lo dicen el PRD §9 y el escenario de F2-72.
	permitido bool
	// disparado dice que ya se avisó: una vez por episodio, no una por
	// latido.
	disparado bool
	// incidente es la fila abierta en la bitácora, para cerrarla cuando el
	// episodio termine.
	incidente int64
	// ultimo es lo último que el detector midió de este episodio: lo que se
	// pinta en la alarma mientras el episodio sigue abierto.
	ultimo engine.Estado
}

// atender es la decisión, estado por estado.
func (a *App) atender(ctx context.Context, e engine.Estado, neg, sil *episodioVigilado) {
	switch e.Tipo {
	case engine.NegroEmpieza:
		a.abrirEpisodio(ctx, neg, KeyNegroUmbral, UmbralNegro)
	case engine.SilencioEmpieza:
		a.abrirEpisodio(ctx, sil, KeySilencioUmbral, UmbralSilencio)
	case engine.NegroSigue:
		a.quizasDisparar(ctx, neg, e)
	case engine.SilencioSigue:
		a.quizasDisparar(ctx, sil, e)
	case engine.NegroTermina:
		a.terminarEpisodio(ctx, neg, e)
	case engine.SilencioTermina:
		a.terminarEpisodio(ctx, sil, e)
	}
	// Las alarmas se repintan enteras en cada cambio: si el aire está mudo Y
	// en negro a la vez, se ven las dos cosas, y que una se arregle no apaga
	// el aviso de la otra.
	a.pintarVigilancia(neg, sil)
}

// pintarVigilancia deja en la pantalla exactamente los episodios que están
// disparados ahora mismo. Una lista vacía apaga la alarma.
func (a *App) pintarVigilancia(eps ...*episodioVigilado) {
	var lista []Alarma
	for _, ep := range eps {
		if !ep.abierto || !ep.disparado {
			continue
		}
		lista = append(lista, Alarma{
			Tipo:    ep.que + "_al_aire",
			Nivel:   NivelProblema,
			Texto:   a.textoDeVigilancia(ep, ep.ultimo),
			Detalle: a.detalleDeVigilancia(ep, ep.ultimo),
			Accion:  &AccionAlarma{Texto: "ver al aire", Ruta: "/al-aire"},
		})
	}
	a.setAlarms("vigilancia", lista)
}

// abrirEpisodio anota que empezó un tramo y decide ya si este tramo se vigila
// o no: el clip que está saliendo puede abrir en negro a propósito.
func (a *App) abrirEpisodio(ctx context.Context, ep *episodioVigilado, clave string, porDefecto time.Duration) {
	*ep = episodioVigilado{
		que:       ep.que,
		abierto:   true,
		umbral:    a.umbralDe(ctx, clave, porDefecto),
		permitido: a.negroPermitido(ctx),
	}
}

// quizasDisparar mira si el episodio ya pasó el umbral. Se dispara una sola
// vez por episodio: un canal mudo diez minutos deja un aviso y un incidente,
// no seiscientos.
func (a *App) quizasDisparar(ctx context.Context, ep *episodioVigilado, e engine.Estado) {
	if !ep.abierto || ep.permitido {
		return
	}
	if ep.disparado {
		ep.ultimo = e // la alarma cuenta cuánto lleva, no cuánto llevaba
		return
	}
	if e.Duro < ep.umbral {
		return
	}
	ep.disparado, ep.ultimo = true, e
	texto := a.textoDeVigilancia(ep, e)
	ep.incidente = a.incidenteAbierto(ctx, tipoDeIncidente(ep.que), texto)
	a.devolverElControl(ctx, ep, texto)
}

// terminarEpisodio apaga lo que se hubiera encendido. La alarma va y viene
// con el hecho: en cuanto vuelve la señal, se apaga sola.
func (a *App) terminarEpisodio(ctx context.Context, ep *episodioVigilado, e engine.Estado) {
	if !ep.abierto {
		return
	}
	disparado := ep.disparado
	id := ep.incidente
	*ep = episodioVigilado{que: ep.que}
	if !disparado {
		return
	}
	if id > 0 {
		a.cerrarIncidente(ctx, id)
	}
	a.Publish("vigilancia", ep.que+"_termina", fmt.Sprintf(
		"volvió la señal: el %s duró %s", ep.que, e.Duro.Round(time.Second)))
}

// cerrarEpisodio es lo mismo pero al apagarse el motor, cuando no hay estado
// `termina` que llegue: un incidente abierto no puede quedarse abierto para
// siempre porque el aire se apagó.
func (a *App) cerrarEpisodio(ctx context.Context, ep *episodioVigilado) {
	if ep.abierto && ep.incidente > 0 {
		a.cerrarIncidente(ctx, ep.incidente)
	}
	*ep = episodioVigilado{que: ep.que}
}

// devolverElControl es el «Avisa y devuelve el control» de Ajustes: si el
// aire está en manual y el interruptor está encendido, se vuelve al
// automático y queda el incidente `manual_por_timeout` con su hora (F2-30).
// Si el control manual todavía no está construido (T5), esto no hace nada más
// que el aviso, que es lo que el operador necesita ver igual.
func (a *App) devolverElControl(ctx context.Context, ep *episodioVigilado, texto string) {
	c := controlDelAire
	if c == nil || !c.EnManual() {
		return
	}
	if !a.devuelveControl(ctx) {
		a.Publish("vigilancia", "manual", texto+
			"; el aire está en manual y «devuelve el control» está apagado, así que no se toca")
		return
	}
	motivo := fmt.Sprintf("%s al aire más de %s", ep.que, ep.umbral.Round(time.Second))
	if err := c.VolverAlAutomatico(ctx, motivo); err != nil {
		a.Publish("vigilancia", "manual", "no pude devolver el aire al automático: "+err.Error())
		return
	}
	a.Incident(model.IncManualPorTimeout.String(), fmt.Sprintf(
		"el control manual venció por %s y el aire volvió solo al automático", motivo))
}

// ── lo que se lee de la base ───────────────────────────────────────────

// umbralDe lee un umbral de settings, en segundos. Lo que no se entiende, lo
// que falta y lo que está fuera de rango valen el de fábrica: un ajuste
// escrito a mano con un dedo torcido no puede apagar la vigilancia.
func (a *App) umbralDe(ctx context.Context, clave string, porDefecto time.Duration) time.Duration {
	v := a.setting(ctx, clave)
	if v == "" {
		return porDefecto
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return porDefecto
	}
	d := time.Duration(n) * time.Second
	if d < UmbralMinimo || d > UmbralMaximo {
		return porDefecto
	}
	return d
}

// devuelveControl es el interruptor de Ajustes. De fábrica encendido: lo que
// está mal vuelve solo (PRD §1).
func (a *App) devuelveControl(ctx context.Context) bool {
	return a.setting(ctx, KeySilencioDevuelveControl) != "no"
}

// negroPermitido dice si el bloque que está saliendo ahora mismo es un
// archivo marcado `negro_intencional` (F2-72). Solo un archivo puede
// desactivar el detector: una fuente en vivo nunca, porque un vivo congelado
// en negro es exactamente el caso que hay que avisar.
//
// Se mira una vez por episodio, no por cuadro: son dos consultas a la base
// cada vez que el aire se pone negro, no sesenta por segundo.
func (a *App) negroPermitido(ctx context.Context) bool {
	ahora := a.Now()
	items, err := a.Store.Plan.ListRange(ctx, a.ChannelID,
		ahora.Add(-VentanaVigilancia), ahora)
	if err != nil {
		return false
	}
	for _, it := range items {
		if !enCurso(it, ahora) {
			continue
		}
		if it.Origin != model.OriginAsset && it.Origin != model.OriginFiller {
			continue
		}
		if it.MediaAssetID == nil {
			continue
		}
		asset, err := a.Store.Media.Get(ctx, *it.MediaAssetID)
		if err != nil {
			continue
		}
		if asset.IntentionalBlack {
			return true
		}
	}
	return false
}

// ── la bitácora y las frases ──────────────────────────────────────────

// incidenteAbierto deja el incidente abierto —esto dura mientras dure el
// episodio— y devuelve su id para cerrarlo luego (ver el comentario de
// App.Incident en app.go).
func (a *App) incidenteAbierto(ctx context.Context, tipo, detalle string) int64 {
	inc := model.Incident{
		ChannelID: a.ChannelID,
		Kind:      tipo,
		Start:     a.Now(),
		Detail:    detalle,
	}
	c, cancel := context.WithTimeout(sinCancelar(ctx), 5*time.Second)
	defer cancel()
	if err := a.Store.Incident.Insert(c, &inc); err != nil {
		a.Publish("incidente", tipo, detalle)
		return 0
	}
	a.Publish("incidente", tipo, detalle)
	return inc.ID
}

// cerrarIncidente cierra la fila abierta con la hora de ahora.
func (a *App) cerrarIncidente(ctx context.Context, id int64) {
	c, cancel := context.WithTimeout(sinCancelar(ctx), 5*time.Second)
	defer cancel()
	_ = a.Store.Incident.Close(c, id, a.Now())
}

// sinCancelar da un contexto que sirve para escribir en la base aunque el que
// venía ya esté cancelado: el incidente de por qué se apagó el aire es justo
// el que no se puede perder.
func sinCancelar(ctx context.Context) context.Context {
	if ctx.Err() == nil {
		return ctx
	}
	return context.Background()
}

// tipoDeIncidente traduce el nombre corto del episodio al tipo del catálogo
// (internal/model/incidentes.go). Nadie escribe una cadena suelta.
func tipoDeIncidente(que string) string {
	if que == "negro" {
		return model.IncNegroDetectado.String()
	}
	return model.IncSilencioDetectado.String()
}

// textoDeVigilancia es la frase que lee una persona. Dice qué pasa y cuánto
// lleva pasando, sin jerga y sin regañar.
func (a *App) textoDeVigilancia(ep *episodioVigilado, e engine.Estado) string {
	if ep.que == "negro" {
		return fmt.Sprintf("el canal lleva %s en negro", e.Duro.Round(time.Second))
	}
	return fmt.Sprintf("el canal lleva %s sin sonido", e.Duro.Round(time.Second))
}

// detalleDeVigilancia es el número medido, para quien quiera el número.
func (a *App) detalleDeVigilancia(ep *episodioVigilado, e engine.Estado) string {
	if ep.que == "negro" {
		return fmt.Sprintf("luma media %.1f, por debajo de %.0f, medida en la salida (umbral %s)",
			e.Nivel, engine.LumaNegra, ep.umbral.Round(time.Second))
	}
	return fmt.Sprintf("%.0f dBFS, por debajo de %.0f, medidos en la salida (umbral %s)",
		e.Nivel, engine.SilencioDBFS, ep.umbral.Round(time.Second))
}

// UmbralesDeVigilancia son los umbrales vigentes, en segundos y en palabras claras.
// Ajustes enseña siempre este número (PRD §13) y la API lo sirve con los
// demás ajustes.
func (a *App) UmbralesDeVigilancia(ctx context.Context) (silencio, negro int, devuelve bool) {
	return int(a.umbralDe(ctx, KeySilencioUmbral, UmbralSilencio).Seconds()),
		int(a.umbralDe(ctx, KeyNegroUmbral, UmbralNegro).Seconds()),
		a.devuelveControl(ctx)
}

// UmbralValido dice si un ajuste de umbral se puede guardar, y si no, por qué
// no. Lo usa la API para contestar en palabras claras.
func UmbralValido(v string) (int, error) {
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("el umbral se escribe en segundos, con un número entero: %q no lo es", v)
	}
	if d := time.Duration(n) * time.Second; d < UmbralMinimo || d > UmbralMaximo {
		return 0, fmt.Errorf("el umbral va de %.0f a %.0f segundos; %d se sale",
			UmbralMinimo.Seconds(), UmbralMaximo.Seconds(), n)
	}
	return n, nil
}
