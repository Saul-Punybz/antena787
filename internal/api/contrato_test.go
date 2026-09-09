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
	return propiedadesDe(t, nombre, true)
}

// propiedadesTodas devuelve todas las propiedades de una interfaz, también
// las opcionales. Sirve para lo que es opcional en el contrato pero tiene
// que estar cuando hay algo que mandar: el sonido de un archivo que sí trae
// pistas (F1-60).
func propiedadesTodas(t *testing.T, nombre string) []string {
	t.Helper()
	return propiedadesDe(t, nombre, false)
}

func propiedadesDe(t *testing.T, nombre string, soloObligatorias bool) []string {
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
				if p := rePropiedad.FindStringSubmatch(linea); p != nil && (p[2] != "?" || !soloObligatorias) {
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

// exigeTodas comprueba que el objeto trae todas las propiedades de la
// interfaz, opcionales incluidas, menos las que se perdonen aparte.
func exigeTodas(t *testing.T, quien string, raw []byte, interfaz string, salvo ...string) {
	t.Helper()
	tiene := clavesDe(t, raw)
	perdonadas := map[string]bool{}
	for _, s := range salvo {
		perdonadas[s] = true
	}
	for _, prop := range propiedadesTodas(t, interfaz) {
		if perdonadas[prop] || tiene[prop] {
			continue
		}
		t.Errorf("%s no manda %q, que %s declara en web/src/lib/tipos.ts", quien, prop, interfaz)
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

// ── el sonido de cada archivo (F1-58 a F1-62) ─────────────────────────

// TestElServidorMandaElSonidoDeCadaArchivo es la otra mitad del contrato de
// audio: cuando el archivo trae pistas, Biblioteca las manda —en el título y
// en cada episodio— con todo lo que `AudioDelMaterial` declara, para que se
// pueda pintar el selector de pista.
func TestElServidorMandaElSonidoDeCadaArchivo(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	ctx := context.Background()

	deLaSerie := c.conAudio("/medios/Kojak S01E01.mkv", 1)
	delTitulo := c.conAudio("/medios/Kojak - presentación.mkv", 0)

	title := model.Title{Name: "Kojak con dos idiomas", Kind: model.TitleSeries, MediaAssetID: &delTitulo.ID}
	if err := c.a.Store.Title.Insert(ctx, &title); err != nil {
		t.Fatalf("no pude guardar el título: %v", err)
	}
	ep := model.Episode{TitleID: title.ID, Season: 1, Number: 1, Name: "El principio", MediaAssetID: &deLaSerie.ID}
	if err := c.a.Store.Episode.Insert(ctx, &ep); err != nil {
		t.Fatalf("no pude guardar el episodio: %v", err)
	}

	// La lista.
	w := c.do("GET", "/api/v1/biblioteca", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("/biblioteca dio %d: %s", w.Code, w.Body.String())
	}
	var lista []json.RawMessage
	c.json(w, &lista)
	var enLista []byte
	for _, uno := range lista {
		if strings.Contains(string(uno), "Kojak con dos idiomas") {
			enLista = uno
		}
	}
	if enLista == nil {
		t.Fatalf("la biblioteca no trae el título: %s", w.Body.String())
	}
	exigeTodas(t, "GET /biblioteca (un título con sonido)", enLista, "AudioDelMaterial")
	compruebaElSonido(t, "GET /biblioteca", enLista, delTitulo.ID, 0)

	// La ficha, con el episodio dentro.
	w = c.do("GET", "/api/v1/biblioteca/"+itoa(title.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("/biblioteca/{id} dio %d: %s", w.Code, w.Body.String())
	}
	exigeTodas(t, "GET /biblioteca/{id}", w.Body.Bytes(), "AudioDelMaterial")
	compruebaElSonido(t, "GET /biblioteca/{id}", w.Body.Bytes(), delTitulo.ID, 0)

	episodio := primero(t, campo(t, w.Body.Bytes(), "lista_de_episodios"))
	exigeTodas(t, "GET /biblioteca/{id} (un episodio)", episodio, "AudioDelMaterial")
	compruebaElSonido(t, "GET /biblioteca/{id} (un episodio)", episodio, deLaSerie.ID, 1)
}

// compruebaElSonido mira que los campos de sonido digan lo que dice la base.
func compruebaElSonido(t *testing.T, quien string, raw []byte, materialID int64, pistaAire int) {
	t.Helper()
	var got struct {
		MaterialID  int64 `json:"material_id"`
		PistasAudio []struct {
			Indice  int    `json:"indice"`
			Idioma  string `json:"idioma"`
			Canales int    `json:"canales"`
			Titulo  string `json:"titulo"`
		} `json:"pistas_audio"`
		PistaAudioAire    int    `json:"pista_audio_aire"`
		AudioSidecar      string `json:"audio_sidecar"`
		SubtitulosSidecar string `json:"subtitulos_sidecar"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("%s: la respuesta no se entiende: %v", quien, err)
	}
	if got.MaterialID != materialID {
		t.Errorf("%s manda material_id %d y el archivo es el %d", quien, got.MaterialID, materialID)
	}
	if len(got.PistasAudio) != 2 {
		t.Fatalf("%s manda %d pistas y el archivo trae dos: %s", quien, len(got.PistasAudio), raw)
	}
	if got.PistasAudio[1].Idioma != "es" || got.PistasAudio[1].Indice != 1 ||
		got.PistasAudio[1].Canales != 2 || got.PistasAudio[1].Titulo == "" {
		t.Errorf("%s no manda la segunda pista entera: %s", quien, raw)
	}
	if got.PistaAudioAire != pistaAire {
		t.Errorf("%s dice que sale la pista %d y es la %d", quien, got.PistaAudioAire, pistaAire)
	}
	if got.AudioSidecar == "" || got.SubtitulosSidecar == "" {
		t.Errorf("%s no dice de dónde salieron el sonido y los subtítulos de al lado: %s", quien, raw)
	}
}

// ── el asistente de instalación ───────────────────────────────────────

// TestElAsistenteMandaLoQueLaPantallaPinta es la misma prueba de contrato
// para las respuestas del asistente: GET /instalacion, el paso 8 y el
// relleno por defecto contra lo que declara web/src/lib/tipos.ts.
func TestElAsistenteMandaLoQueLaPantallaPinta(t *testing.T) {
	if !declaraLaInterfaz(t, "Instalacion", "Opcion", "OpcionesDelAsistente", "DetectadoEnLaMaquina") {
		t.Skip("web/src/lib/tipos.ts todavía no declara los tipos del asistente")
	}
	c := nuevo(t)

	w := c.do("GET", "/api/v1/instalacion", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /instalacion dio %d: %s", w.Code, w.Body.String())
	}
	exige(t, "GET /instalacion", w.Body.Bytes(), "Instalacion")
	// `problema` solo sale cuando ffmpeg falta, así que se perdona.
	exige(t, "GET /instalacion (detectado)", campo(t, w.Body.Bytes(), "detectado"), "DetectadoEnLaMaquina", "problema")
	opciones := campo(t, w.Body.Bytes(), "opciones")
	exige(t, "GET /instalacion (opciones)", opciones, "OpcionesDelAsistente")
	for _, lista := range []string{"modo", "destino", "retorno", "calidad"} {
		exige(t, "GET /instalacion (opciones."+lista+")", primero(t, campo(t, opciones, lista)), "Opcion", "ayuda")
	}

	// El paso 8, con material de verdad para que proponga algo.
	if w := c.do("POST", "/api/v1/instalacion/paso/1", map[string]any{
		"nombre": "Caribbean Advantage TV", "clave": clavePrueba,
	}); w.Code != http.StatusOK {
		t.Fatalf("el paso 1 dio %d: %s", w.Code, w.Body.String())
	}
	c.conMaterial("La Batalla")
	w = c.do("POST", "/api/v1/instalacion/paso/8", map[string]string{"propuesta": "automatica"})
	if w.Code != http.StatusOK {
		t.Fatalf("el paso 8 dio %d: %s", w.Code, w.Body.String())
	}
	if declaraLaInterfaz(t, "RespuestaPaso8") {
		exige(t, "POST /instalacion/paso/8", w.Body.Bytes(), "RespuestaPaso8")
	}

	// Y lo que ya está contestado vuelve con la forma que la pantalla espera.
	w = c.do("GET", "/api/v1/instalacion", nil)
	respuestas := campo(t, w.Body.Bytes(), "respuestas")
	if declaraLaInterfaz(t, "RespuestasDelAsistente") {
		for _, paso := range []string{"1", "8"} {
			if _, hay := clavesDe(t, respuestas)[paso]; !hay {
				t.Errorf("el paso %s está contestado y no vuelve en `respuestas`: %s", paso, respuestas)
			}
		}
	}

	// El relleno por defecto: 202 con la forma declarada.
	if !declaraLaInterfaz(t, "RellenoPorDefecto") {
		return
	}
	if c.a.FFmpeg == "" {
		t.Skip("hacen falta las herramientas de video para pedir el relleno por defecto")
	}
	if w := c.do("POST", "/api/v1/instalacion/paso/7", map[string]string{"carpeta": t.TempDir()}); w.Code != http.StatusOK {
		t.Fatalf("el paso 7 dio %d: %s", w.Code, w.Body.String())
	}
	eventos, soltar := c.a.Subscribe()
	defer soltar()
	w = c.do("POST", "/api/v1/instalacion/relleno-por-defecto", nil)
	if w.Code != http.StatusAccepted {
		t.Fatalf("el relleno por defecto dio %d: %s", w.Code, w.Body.String())
	}
	exige(t, "POST /instalacion/relleno-por-defecto", w.Body.Bytes(), "RellenoPorDefecto")
	// No se deja el trabajo colgando después de la prueba.
	esperarEvento(t, eventos, "relleno", 2*time.Minute)
}

// declaraLaInterfaz dice si tipos.ts ya trae esas interfaces. La pantalla del
// asistente y este servidor se están construyendo a la vez: mientras la
// interfaz no las declare, esta prueba se salta en vez de fallar.
func declaraLaInterfaz(t *testing.T, nombres ...string) bool {
	t.Helper()
	raw, err := os.ReadFile(tiposTS)
	if err != nil {
		t.Fatalf("no pude leer el contrato de la interfaz: %v", err)
	}
	for _, n := range nombres {
		if !strings.Contains(string(raw), "export interface "+n+" {") {
			return false
		}
	}
	return true
}
