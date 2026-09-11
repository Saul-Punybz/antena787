package api

// ── F1-78 · un hueco de la parrilla es accionable ──────────────────────
//
// La promesa es de interfaz y no hay corredor de pruebas para la interfaz, así
// que se verifica como se verifica el contrato (contrato_test.go): leyendo los
// archivos de web/src desde aquí. Lo que se comprueba es justo el defecto que
// se encontró el 11 de septiembre de 2026 —un vacío sin acción y un botón sin
// `onClick`—, no el aspecto de la pantalla.

import (
	"os"
	"strings"
	"testing"
)

// pantallasDeParrilla son las cuatro vistas de la parrilla y lo que comparten.
var pantallasDeParrilla = []string{
	"web/src/pantallas/ParrillaSemana.tsx",
	"web/src/pantallas/ParrillaDia.tsx",
	"web/src/pantallas/ParrillaMes.tsx",
	"web/src/pantallas/ParrillaGuia.tsx",
	"web/src/pantallas/Parrilla.tsx",
}

func leerDeLaInterfaz(t *testing.T, ruta string) string {
	t.Helper()
	raw, err := os.ReadFile(raiz + "/" + ruta)
	if err != nil {
		t.Fatalf("no pude leer %s: %v", ruta, err)
	}
	return string(raw)
}

// etiquetasDeBoton devuelve las propiedades de cada <button> del archivo: lo
// que hay entre `<button` y el primer hijo o el cierre. No se busca el `>` de
// la etiqueta porque las propiedades de JSX llevan flechas («=>») dentro.
func etiquetasDeBoton(fuente string) []string {
	var out []string
	for i := 0; ; {
		j := strings.Index(fuente[i:], "<button")
		if j < 0 {
			return out
		}
		resto := fuente[i+j+len("<button"):]
		corte := len(resto)
		for _, marca := range []string{"</button>", "<button", "<div", "<span", "<p ", "<Icono"} {
			if k := strings.Index(resto, marca); k >= 0 && k < corte {
				corte = k
			}
		}
		out = append(out, resto[:corte])
		i += j + len("<button")
	}
}

// TestF1Verif78HuecoEsUnBotonConAccion: la franja vacía de Semana y la fila
// vacía de Día son botones de verdad, con su etiqueta para quien no ve la
// pantalla, y lo que hacen es abrir la regla a esa hora.
func TestF1Verif78HuecoEsUnBotonConAccion(t *testing.T) {
	casos := []struct {
		ruta  string
		clase string
	}{
		{"web/src/pantallas/ParrillaSemana.tsx", "hueco"},
		{"web/src/pantallas/ParrillaDia.tsx", "dia-fila--vacia"},
	}
	for _, c := range casos {
		texto := leerDeLaInterfaz(t, c.ruta)
		if !strings.Contains(texto, c.clase) {
			t.Errorf("%s: no pinta el vacío con la clase %q", c.ruta, c.clase)
		}
		// Tiene que ser un <button>: con un <div> no se llega con el teclado.
		var elVacio string
		for _, etiqueta := range etiquetasDeBoton(texto) {
			if strings.Contains(etiqueta, "etiquetaDelHueco(") {
				elVacio = etiqueta
			}
		}
		if elVacio == "" {
			t.Errorf("%s: el vacío no es un <button> con aria-label del hueco", c.ruta)
			continue
		}
		if !strings.Contains(elVacio, `type="button"`) {
			t.Errorf("%s: el botón del vacío no dice type=\"button\"", c.ruta)
		}
		if !strings.Contains(elVacio, "onClick") {
			t.Errorf("%s: tocar el vacío no hace nada", c.ruta)
		}
	}
}

// TestF1Verif78NingunBotonMuertoEnLaParrilla: todo botón de las vistas de la
// parrilla hace algo. El «Escoger yo» del aviso de fin de semana vacío estuvo
// sin `onClick`: un botón que no hace nada enseña a desconfiar de todos.
func TestF1Verif78NingunBotonMuertoEnLaParrilla(t *testing.T) {
	for _, ruta := range pantallasDeParrilla {
		texto := leerDeLaInterfaz(t, ruta)
		for _, etiqueta := range etiquetasDeBoton(texto) {
			if strings.Contains(etiqueta, "onClick") || strings.Contains(etiqueta, "type=\"submit\"") {
				continue
			}
			resumen := strings.Join(strings.Fields(etiqueta), " ")
			if len(resumen) > 140 {
				resumen = resumen[:140] + "…"
			}
			t.Errorf("%s: botón sin acción · %s", ruta, resumen)
		}
	}
}

// TestF1Verif78LaReglaAbreConDiaYHora: los valores con que abre la regla salen
// del hueco, y la hora se redondea a la media hora, que es la rejilla de la
// parrilla. Y todo sigue entrando por una regla: la parrilla no pone contenido
// directo, así que no hay POST /plan (decisión de Saul del 11 sept 2026).
func TestF1Verif78LaReglaAbreConDiaYHora(t *testing.T) {
	huecos := leerDeLaInterfaz(t, "web/src/lib/huecos.ts")
	if !strings.Contains(huecos, "Math.round(crudo / 30) * 30") {
		t.Error("web/src/lib/huecos.ts: la hora del clic no se redondea a la media hora")
	}
	editor := leerDeLaInterfaz(t, "web/src/componentes/EditorDeRegla.tsx")
	for _, pieza := range []string{
		"patron_de_dias: patronDeUnDia(v.dia)", // el día del hueco, ya puesto
		"hora: v.desde",                        // la hora del hueco, ya puesta
		"fecha_inicio: v.dia",
	} {
		if !strings.Contains(editor, pieza) {
			t.Errorf("EditorDeRegla.tsx: la regla del hueco no lleva %q", pieza)
		}
	}
	cliente := leerDeLaInterfaz(t, "web/src/lib/api.ts")
	if strings.Contains(cliente, "'/plan', conCuerpo('POST'") {
		t.Error("web/src/lib/api.ts: apareció un POST /plan; a la parrilla se entra por reglas")
	}
}
