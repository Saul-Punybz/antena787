package app

// Mientras el canal esté encendido —en sombra o al aire— la máquina no se
// duerme (F2-112). El 9 de septiembre de 2026 la Mac de pruebas durmió 19
// minutos en dos ratos y lo único que quedó fueron cuatro incidentes
// `salto_de_reloj`: nadie impidió que un servidor de playout se echara una
// siesta (`docs/f1/SOMBRA-2026-09-09.md`, S-8).
//
// El cómo es cosa de internal/despierto. Lo de aquí es la política: se sostiene
// al arrancar, se reintenta cada cinco minutos si el sistema no dejó, y
// mientras no se consiga hay una alarma de nivel aviso en Al aire diciendo qué
// tiene que hacer la persona. En cuanto se consigue, la alarma se apaga y
// queda un incidente `maquina_despierta` —una sola vez— para que la bitácora
// tenga constancia de que este canal ya no depende de la suerte.
//
// El guardián (`caffeinate`/`systemd-inhibit`) puede caerse solo mientras el
// canal sigue encendido —alguien lo mata a mano, o el sistema—, y eso es tan
// peligroso como no haberlo sostenido nunca: la máquina se puede dormir sin
// que nadie se entere. Por eso, mientras está sostenido, este bucle también
// vigila el canal `caido` que da internal/despierto; si el guardián se cae,
// vuelve a sostener enseguida (y si no lo consigue a la primera, con una
// espera que se dobla —1, 2, 4… hasta un tope de un minuto—), la alarma
// vuelve a salir mientras no lo consiga, y en cuanto se repone queda el
// incidente `guardian_caido` —una vez por caída, no por cada reintento—.
//
// No sostener no saca a nadie del aire: es un aviso, no un problema.

import (
	"context"
	"time"

	"antena787/internal/despierto"
)

// TextoDespiertoNoPude es lo que lee la persona cuando el sistema no dejó
// impedir la suspensión. Dice qué hacer, no qué falló, tanto si nunca se
// pudo sostener como si el guardián se cayó y todavía no se ha repuesto.
const TextoDespiertoNoPude = "No pude impedir que esta máquina se duerma: apaga la suspensión por inactividad en el sistema"

// TextoGuardianCaido es la frase de la bitácora cuando el guardián se cae
// solo y se vuelve a levantar.
const TextoGuardianCaido = "El guardián que impide dormir a la máquina se cayó y se volvió a levantar"

const (
	// despiertoRetryCaidaInicial es la primera espera al reintentar sostener
	// tras una caída del guardián, y despiertoRetryCaidaTope el techo al que
	// llega doblándola (1, 2, 4… 60 s). No es DespiertoRetry: esa es la
	// espera de cuando el sistema nunca dejó sostener desde el arranque; aquí
	// ya se pudo una vez, así que vale la pena insistir más y más seguido.
	despiertoRetryCaidaInicial = time.Second
	despiertoRetryCaidaTope    = 60 * time.Second
)

// sostenerFunc es Sostener con el canal de caída incluido: soltar, el canal
// por el que se avisa si el guardián se cae por su cuenta (nil si el sistema
// no puede avisar, como Windows), y el error si no se pudo sostener.
type sostenerFunc func(context.Context) (soltar func(), caido <-chan error, err error)

// sostenerPorDefecto es despierto.Sostener hecho variable de paquete: las
// pruebas de este archivo meten aquí un guardián de mentira con su canal de
// caída, algo que a.opts.Sostener no puede llevar (esa firma es la vieja, de
// antes de F2-112 caído, y no hay por qué romper a quien ya la usa).
var sostenerPorDefecto sostenerFunc = despierto.Sostener

// despiertoLoop sostiene la máquina despierta mientras viva el canal, y la
// suelta al apagarse. Si no puede, deja la alarma puesta y vuelve a intentarlo
// cada DespiertoRetry: una máquina que hoy no deja puede dejar mañana, cuando
// alguien la haya configurado sin reiniciar Antena787. Si el guardián se cae
// mientras está sostenido, vuelve a intentarlo enseguida (ver reponerTrasCaida).
func (a *App) despiertoLoop(ctx context.Context) error {
	sostener := a.sostenerConCaida()

	var soltar func()
	var caido <-chan error
	defer func() {
		if soltar != nil {
			soltar()
		}
	}()

	if soltar, caido = a.sostenerDespierto(ctx, sostener); soltar == nil {
		cada := a.opts.DespiertoRetry
		if cada <= 0 {
			cada = DespiertoRetry
		}
		t := time.NewTicker(cada)
		defer t.Stop()
	arrancando:
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-t.C:
				if soltar, caido = a.sostenerDespierto(ctx, sostener); soltar != nil {
					break arrancando
				}
			}
		}
	}

	// Conseguido: vigilar hasta que se apague, o hasta que el guardián se
	// caiga por su cuenta.
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-caido:
			if soltar, caido = a.reponerTrasCaida(ctx, sostener); soltar == nil {
				return ctx.Err()
			}
		}
	}
}

// sostenerConCaida es la función rica que usa el bucle: si hay una de
// pruebas en a.opts.Sostener (la firma vieja, sin canal), se envuelve tal
// cual —esas pruebas no cubren la caída del guardián, y no tienen por qué—;
// si no hay ninguna, sostenerPorDefecto, con canal de verdad.
func (a *App) sostenerConCaida() sostenerFunc {
	if legacy := a.opts.Sostener; legacy != nil {
		return func(ctx context.Context) (func(), <-chan error, error) {
			soltar, err := legacy(ctx)
			return soltar, nil, err
		}
	}
	return sostenerPorDefecto
}

// sostenerDespierto intenta una vez y deja la pantalla contando la verdad:
// alarma si no pudo, alarma apagada si pudo. Devuelve nil, nil si no lo
// consiguió.
func (a *App) sostenerDespierto(ctx context.Context, sostener sostenerFunc) (soltar func(), caido <-chan error) {
	soltar, caido, err := sostener(ctx)
	if err != nil || soltar == nil {
		detalle := ""
		if err != nil {
			detalle = err.Error()
		}
		a.setAlarms("despierto", []Alarma{{
			Tipo:    "maquina_puede_dormirse",
			Nivel:   NivelAviso,
			Texto:   TextoDespiertoNoPude,
			Detalle: detalle,
			Accion:  &AccionAlarma{Texto: "ver los ajustes", Ruta: "/ajustes"},
		}})
		return nil, nil
	}
	a.setAlarms("despierto", nil)
	if !a.despiertoAvisado.Swap(true) {
		a.Incident("maquina_despierta",
			"mientras el canal esté encendido esta máquina no se va a dormir por inactividad")
	}
	return soltar, caido
}

// reponerTrasCaida se activa cuando el guardián se cayó por su cuenta:
// mientras no se consiga volver a sostener, la pantalla dice lo mismo que si
// nunca se hubiera podido (misma alarma), y se reintenta con una espera que
// se dobla —1, 2, 4… hasta despiertoRetryCaidaTope—, empezando enseguida, sin
// esperar antes del primer intento. En cuanto se repone, queda el incidente
// `guardian_caido` una sola vez: la caída ya pasó una, no importa cuántos
// reintentos hicieran falta para levantarse.
func (a *App) reponerTrasCaida(ctx context.Context, sostener sostenerFunc) (soltar func(), caido <-chan error) {
	espera := despiertoRetryCaidaInicial
	for {
		if soltar, caido = a.sostenerDespierto(ctx, sostener); soltar != nil {
			a.Incident("guardian_caido", TextoGuardianCaido)
			return soltar, caido
		}
		select {
		case <-ctx.Done():
			return nil, nil
		case <-time.After(espera):
		}
		if espera < despiertoRetryCaidaTope {
			espera *= 2
			if espera > despiertoRetryCaidaTope {
				espera = despiertoRetryCaidaTope
			}
		}
	}
}
