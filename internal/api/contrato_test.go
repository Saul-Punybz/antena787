package api

// La prueba de contrato de D-5: la interfaz y el servidor tienen que hablar
// del mismo JSON.
//
// `web/src/lib/tipos.ts` es el contrato escrito —lo que la interfaz pinta— y
// las etiquetas `json` de Go son lo que el servidor manda. Aquí se leen los
// dos y se comprueba que cada propiedad **obligatoria** de las interfaces
// que importan aparece de verdad en la respuesta. No se compara al revés: el
// servidor puede mandar más de lo que la interfaz usa.

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"antena787/internal/model"
)

// tiposTS es el archivo de tipos de la interfaz visto desde internal/api.
const tiposTS = "../../web/src/lib/tipos.ts"

var (
	reInterfaz  = regexp.MustCompile(`(?m)^export interface (\w+)[^{]*\{`)
	rePropiedad = regexp.MustCompile(`^\s*(\w+)(\??)\s*:`)
)

// propiedadesObligatorias devuelve los nombres de las propiedades sin `?` de
// una interfaz de tipos.ts. Las que llevan `?` son opcionales: el servidor
// puede no mandarlas.
func propiedadesObligatorias(t *testing.T, nombre string) []string {
	t.Helper()
	raw, err := os.ReadFile(tiposTS)
	if err != nil {
		t.Fatalf("no pude leer el contrato de la interfaz: %v", err)
	}
	texto := string(raw)
	loc := reInterfaz.FindAllStringSubmatchIndex(texto, -1)
	for _, m := range loc {
		if texto[m[2]:m[3]] != nombre {
			continue
		}
		fin := strings.Index(texto[m[1]:], "\n}")
		if fin < 0 {
			t.Fatalf("la interfaz %s de tipos.ts no se cierra", nombre)
		}
		cuerpo := texto[m[1] : m[1]+fin]
		var out []string
		hondura := 0
		for _, linea := range strings.Split(cuerpo, "\n") {
			limpia := strings.TrimSpace(linea)
			if strings.HasPrefix(limpia, "//") || strings.HasPrefix(limpia, "*") ||
				strings.HasPrefix(limpia, "/*") {
				continue
			}
			if hondura == 0 {
				if p := rePropiedad.FindStringSubmatch(linea); p != nil && p[2] != "?" {
					out = append(out, p[1])
				}
			}
			hondura += strings.Count(limpia, "{") - strings.Count(limpia, "}")
		}
		return out
	}
	t.Fatalf("tipos.ts no declara la interfaz %s", nombre)
	return nil
}

// claves devuelve las claves del primer nivel de un objeto JSON.
func clavesDe(t *testing.T, raw []byte) map[string]bool {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("la respuesta no es un objeto JSON: %s", raw)
	}
	out := map[string]bool{}
	for k := range m {
		out[k] = true
	}
	return out
}

// exige comprueba que el objeto trae todas las propiedades obligatorias de
// la interfaz, menos las que se declaren aparte (las que son de F2 y que la
// interfaz ya sabe pintar vacías).
func exige(t *testing.T, quien string, raw []byte, interfaz string, salvo ...string) {
	t.Helper()
	tiene := clavesDe(t, raw)
	perdonadas := map[string]bool{}
	for _, s := range salvo {
		perdonadas[s] = true
	}
	for _, prop := range propiedadesObligatorias(t, interfaz) {
		if perdonadas[prop] || tiene[prop] {
			continue
		}
		t.Errorf("%s no manda %q, que %s declara obligatorio en web/src/lib/tipos.ts",
			quien, prop, interfaz)
	}
}

// primero saca el primer elemento de una lista JSON.
func primero(t *testing.T, raw []byte) []byte {
	t.Helper()
	var lista []json.RawMessage
	if err := json.Unmarshal(raw, &lista); err != nil {
		t.Fatalf("la respuesta no es una lista JSON: %s", raw)
	}
	if len(lista) == 0 {
		t.Fatalf("la lista llegó vacía: %s", raw)
	}
	return lista[0]
}

// campo saca un campo de un objeto JSON tal cual.
func campo(t *testing.T, raw []byte, nombre string) []byte {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("no es un objeto JSON: %s", raw)
	}
	v, ok := m[nombre]
	if !ok {
		t.Fatalf("el objeto no trae %q: %s", nombre, raw)
	}
	return v
}

// TestElServidorMandaLoQueLaInterfazPinta es la prueba de contrato: recorre
// las cinco respuestas que dibujan las pantallas y las compara contra
// tipos.ts.
func TestElServidorMandaLoQueLaInterfazPinta(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	title := c.conMaterial("Kojak")
	id := title.ID
	w := c.do("POST", "/api/v1/reglas", map[string]any{
		"tipo":                  "normal",
		"title_id":              id,
		"patron_de_dias":        "LMMJVSD",
		"hora":                  14 * 60,
		"duracion_slot_ms":      30 * 60 * 1000,
		"fecha_inicio":          string(hoy(c)),
		"fecha_fin":             string(hoy(c).Add(30)),
		"episodios_por_corrida": 1,
		"activa":                true,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("crear la regla dio %d: %s", w.Code, w.Body.String())
	}
	if w := c.do("POST", "/api/v1/plan/recalcular", nil); w.Code != http.StatusOK {
		t.Fatalf("recalcular dio %d: %s", w.Code, w.Body.String())
	}

	// Estado. `salidas`, `retorno_de_aire` y `control_manual` son de F2: en
	// F1 no hay motor y la interfaz los pinta vacíos.
	w = c.do("GET", "/api/v1/estado", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("/estado dio %d", w.Code)
	}
	exige(t, "GET /estado", w.Body.Bytes(), "Estado", "salidas")

	// Un elemento del plan.
	w = c.do("GET", "/api/v1/plan?dia="+string(hoy(c)), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("/plan dio %d: %s", w.Code, w.Body.String())
	}
	items := campo(t, w.Body.Bytes(), "items")
	var filas []map[string]json.RawMessage
	if err := json.Unmarshal(items, &filas); err != nil {
		t.Fatalf("los items del plan no son una lista: %s", items)
	}
	var bloque []byte
	for _, f := range filas {
		if _, esHueco := f["hueco"]; esHueco {
			continue
		}
		bloque, _ = json.Marshal(f)
		break
	}
	if bloque == nil {
		t.Fatal("el plan del día no trae ni un bloque de contenido")
	}
	exige(t, "GET /plan (un bloque)", bloque, "ElementoDelPlan")

	// La semana: el día y una franja.
	w = c.do("GET", "/api/v1/plan/semana?desde="+string(hoy(c)), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("/plan/semana dio %d: %s", w.Code, w.Body.String())
	}
	exige(t, "GET /plan/semana", w.Body.Bytes(), "SemanaDelPlan")
	dia := primero(t, campo(t, w.Body.Bytes(), "dias"))
	exige(t, "GET /plan/semana (un día)", dia, "DiaDeLaSemana")
	exige(t, "GET /plan/semana (una franja)", primero(t, campo(t, dia, "franjas")), "FranjaSemana")

	// El mes.
	w = c.do("GET", "/api/v1/plan/mes?mes="+string(hoy(c))[:7], nil)
	if w.Code != http.StatusOK {
		t.Fatalf("/plan/mes dio %d: %s", w.Code, w.Body.String())
	}
	exige(t, "GET /plan/mes", w.Body.Bytes(), "MesDelPlan")
	exige(t, "GET /plan/mes (un día)", primero(t, campo(t, w.Body.Bytes(), "dias")), "DiaDelMes")

	// La guía contra el plan.
	w = c.do("GET", "/api/v1/guia?dia="+string(hoy(c)), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("/guia dio %d: %s", w.Code, w.Body.String())
	}
	exige(t, "GET /guia", w.Body.Bytes(), "Guia", "por_que_no_coinciden")

	// La biblioteca.
	w = c.do("GET", "/api/v1/biblioteca", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("/biblioteca dio %d: %s", w.Code, w.Body.String())
	}
	exige(t, "GET /biblioteca", primero(t, w.Body.Bytes()), "TituloDeBiblioteca")

	w = c.do("GET", "/api/v1/biblioteca/"+itoa(id), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("/biblioteca/{id} dio %d: %s", w.Code, w.Body.String())
	}
	exige(t, "GET /biblioteca/{id}", w.Body.Bytes(), "FichaDeTitulo")
	exige(t, "GET /biblioteca/{id}", w.Body.Bytes(), "TituloDeBiblioteca")
}

// TestLaBibliotecaDiceElEstadoDelMaterial es la otra mitad de D-5: la
// etiqueta ámbar de Biblioteca lee `estado_material`, y por episodio también.
func TestLaBibliotecaDiceElEstadoDelMaterial(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	ctx := context.Background()

	// Un título con un episodio a medio normalizar.
	asset := model.MediaAsset{
		Path:           "/medios/zoids-01.mkv",
		DurationMs:     22 * 60 * 1000,
		State:          model.AssetReady,
		NormalizeState: "pendiente",
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	if err := c.a.Store.Media.Insert(ctx, &asset); err != nil {
		t.Fatalf("no pude guardar el archivo: %v", err)
	}
	title := model.Title{Name: "Zoids", Kind: model.TitleSeries}
	if err := c.a.Store.Title.Insert(ctx, &title); err != nil {
		t.Fatalf("no pude guardar el título: %v", err)
	}
	ep := model.Episode{TitleID: title.ID, Season: 1, Number: 1, Name: "El principio", MediaAssetID: &asset.ID}
	if err := c.a.Store.Episode.Insert(ctx, &ep); err != nil {
		t.Fatalf("no pude guardar el episodio: %v", err)
	}

	w := c.do("GET", "/api/v1/biblioteca", nil)
	var lista []map[string]any
	c.json(w, &lista)
	encontrado := false
	for _, tt := range lista {
		if tt["nombre"] != "Zoids" {
			continue
		}
		encontrado = true
		if tt["estado_material"] != EstadoNoListo {
			t.Fatalf("un título a medio normalizar sale como %v", tt["estado_material"])
		}
		if tt["en_la_parrilla"] != false {
			t.Fatalf("ese título no está en ninguna regla y dice %v", tt["en_la_parrilla"])
		}
		if tt["duracion_ms"] == nil {
			t.Fatal("la biblioteca no manda duracion_ms")
		}
	}
	if !encontrado {
		t.Fatalf("la biblioteca no trae el título: %s", w.Body.String())
	}

	w = c.do("GET", "/api/v1/biblioteca/"+itoa(title.ID), nil)
	var ficha map[string]any
	c.json(w, &ficha)
	eps, ok := ficha["lista_de_episodios"].([]any)
	if !ok || len(eps) != 1 {
		t.Fatalf("la ficha no trae lista_de_episodios: %s", w.Body.String())
	}
	uno, _ := eps[0].(map[string]any)
	if uno["estado_material"] != EstadoNoListo {
		t.Fatalf("el episodio a medio normalizar sale como %v", uno["estado_material"])
	}
}
