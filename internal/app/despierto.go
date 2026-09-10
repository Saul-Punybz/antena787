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
// No sostener no saca a nadie del aire: es un aviso, no un problema.

import (
	"context"
	"time"

	"antena787/internal/despierto"
)

// TextoDespiertoNoPude es lo que lee la persona cuando el sistema no dejó
// impedir la suspensión. Dice qué hacer, no qué falló.
const TextoDespiertoNoPude = "No pude impedir que esta máquina se duerma: apaga la suspensión por inactividad en el sistema"

// despiertoLoop sostiene la máquina despierta mientras viva el canal, y la
// suelta al apagarse. Si no puede, deja la alarma puesta y vuelve a intentarlo
// cada DespiertoRetry: una máquina que hoy no deja puede dejar mañana, cuando
// alguien la haya configurado sin reiniciar Antena787.
func (a *App) despiertoLoop(ctx context.Context) error {
	sostener := a.opts.Sostener
	if sostener == nil {
		sostener = despierto.Sostener
	}
	var soltar func()
	defer func() {
		if soltar != nil {
			soltar()
		}
	}()

	if soltar = a.sostenerDespierto(ctx, sostener); soltar != nil {
		// Conseguido: no hay nada más que hacer hasta que se apague.
		<-ctx.Done()
		return ctx.Err()
	}

	cada := a.opts.DespiertoRetry
	if cada <= 0 {
		cada = DespiertoRetry
	}
	t := time.NewTicker(cada)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			if soltar = a.sostenerDespierto(ctx, sostener); soltar != nil {
				<-ctx.Done()
				return ctx.Err()
			}
		}
	}
}

// sostenerDespierto intenta una vez y deja la pantalla contando la verdad:
// alarma si no pudo, alarma apagada si pudo. Devuelve nil si no lo consiguió.
func (a *App) sostenerDespierto(ctx context.Context, sostener func(context.Context) (func(), error)) func() {
	soltar, err := sostener(ctx)
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
		return nil
	}
	a.setAlarms("despierto", nil)
	if !a.despiertoAvisado.Swap(true) {
		a.Incident("maquina_despierta",
			"mientras el canal esté encendido esta máquina no se va a dormir por inactividad")
	}
	return soltar
}
