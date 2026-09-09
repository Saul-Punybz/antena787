package app

// El aviso de los títulos por emparejar (F1-64).
//
// Cuando el importador no encuentra ficha para un nombre de la hoja, la
// regla entra igual y el título queda «por emparejar». Mientras quede alguno
// sale un aviso en Al aire, porque un título sin ficha es un hueco que nadie
// ve hasta que le toca salir.
//
// La alarma es un estado, no un suceso: se vuelve a calcular al arrancar,
// después de importar una hoja y después de cada decisión de la persona. No
// se manda por el canal de avisos: se arregla mirando la pantalla de Reglas,
// no de madrugada.

import (
	"context"
	"fmt"
	"strings"
)

// PendientesEnElAviso es cuántos nombres caben en el detalle de la alarma
// antes de resumir el resto en «y N más».
const PendientesEnElAviso = 3

// RefreshPendientes vuelve a mirar cuántos títulos están por emparejar y
// deja (o apaga) el aviso de Al aire. Nunca devuelve error: que no se pueda
// leer la lista no puede parar ni una importación ni el aire.
func (a *App) RefreshPendientes(ctx context.Context) {
	if a.Store == nil || a.Store.Title == nil {
		return
	}
	pendientes, err := a.Store.Title.ListPendientes(ctx, a.ChannelID)
	if err != nil {
		return
	}
	if len(pendientes) == 0 {
		a.setAlarms("emparejar", nil)
		return
	}
	nombres := make([]string, 0, PendientesEnElAviso)
	for _, t := range pendientes {
		if len(nombres) == PendientesEnElAviso {
			break
		}
		nombres = append(nombres, "«"+t.Name+"»")
	}
	detalle := strings.Join(nombres, ", ")
	if resto := len(pendientes) - len(nombres); resto > 0 {
		detalle += fmt.Sprintf(" y %d más", resto)
	}
	texto := fmt.Sprintf("%d títulos por emparejar", len(pendientes))
	if len(pendientes) == 1 {
		texto = "1 título por emparejar"
	}
	a.setAlarms("emparejar", []Alarma{{
		Tipo:    "emparejar",
		Nivel:   NivelAviso,
		Texto:   texto,
		Detalle: detalle,
		Accion:  &AccionAlarma{Texto: "emparejar", Ruta: "/reglas"},
	}})
}
