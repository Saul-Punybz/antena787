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
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"antena787/internal/app"
	"antena787/internal/model"
)

// tiposTS es el archivo de tipos de la interfaz visto desde internal/api.
const tiposTS = "../../web/src/lib/tipos.ts"

var (
	reInterfaz  = regexp.MustCompile(`(?m)^export interface (\w+)[^{]*\{`)
	rePropiedad = regexp.MustCompile(`^\s*(\w+)(\??)\s*:`)
)

// propiedad es una propiedad declarada en tipos.ts: cómo se llama, si la
// interfaz la da por opcional, y el tipo escrito tal cual («number», «string |
// null», «Alarma[]»). El tipo es lo que permite comprobar que el servidor no
// solo manda la clave, sino que manda el tipo de valor que la pantalla espera:
// `titulo` como objeto en vez de texto tumbó la pantalla de Reglas entera
// (modo sombra, 9 sept 2026) y la prueba de antes no lo veía.
type propiedad struct {
	Nombre   string
	Opcional bool
	Tipo     string
}

// textoDeTipos lee el contrato de la interfaz.
func textoDeTipos(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(tiposTS)
	if err != nil {
		t.Fatalf("no pude leer el contrato de la interfaz: %v", err)
	}
	return string(raw)
}

// propiedadesObligatorias devuelve los nombres de las propiedades sin `?` de
// una interfaz de tipos.ts. Las que llevan `?` son opcionales: el servidor
// puede no mandarlas.
func propiedadesObligatorias(t *testing.T, nombre string) []string {
	t.Helper()
	var out []string
	for _, p := range propiedadesDe(t, nombre) {
		if !p.Opcional {
			out = append(out, p.Nombre)
		}
	}
	return out
}

// propiedadesDe saca las propiedades del primer nivel de una interfaz, con su
// tipo. Las líneas de comentario no cuentan, y lo de dentro de un objeto
// anidado tampoco: eso se compara aparte si hace falta.
func propiedadesDe(t *testing.T, nombre string) []propiedad {
	t.Helper()
	texto := textoDeTipos(t)
	for _, m := range reInterfaz.FindAllStringSubmatchIndex(texto, -1) {
		if texto[m[2]:m[3]] != nombre {
			continue
		}
		fin := strings.Index(texto[m[1]:], "\n}")
		if fin < 0 {
			t.Fatalf("la interfaz %s de tipos.ts no se cierra", nombre)
		}
		var out []propiedad
		hondura := 0
		for _, linea := range strings.Split(texto[m[1]:m[1]+fin], "\n") {
			limpia := strings.TrimSpace(sinComentario(linea))
			if limpia == "" || esComentario(linea) {
				continue
			}
			if hondura == 0 {
				if p := rePropiedad.FindStringSubmatch(linea); p != nil {
					out = append(out, propiedad{
						Nombre:   p[1],
						Opcional: p[2] == "?",
						Tipo:     limpia[strings.Index(limpia, ":")+1:],
					})
					hondura += llaves(limpia)
					continue
				}
				// Una unión que sigue en la línea de abajo: `tipo?:` y luego
				// una lista de `| 'algo'` (así está escrita Alarma.tipo).
				if len(out) > 0 {
					out[len(out)-1].Tipo += " " + limpia
				}
			}
			hondura += llaves(limpia)
		}
		return out
	}
	t.Fatalf("tipos.ts no declara la interfaz %s", nombre)
	return nil
}

func esComentario(linea string) bool {
	l := strings.TrimSpace(linea)
	return strings.HasPrefix(l, "//") || strings.HasPrefix(l, "*") || strings.HasPrefix(l, "/*")
}

// sinComentario quita el comentario de final de línea («hora: number // …»).
func sinComentario(linea string) string {
	if i := strings.Index(linea, "//"); i >= 0 {
		return linea[:i]
	}
	return linea
}

func llaves(s string) int {
	return strings.Count(s, "{") - strings.Count(s, "}")
}

// ── el tipo del valor, no solo la clave ───────────────────────────────

// clase dice de qué es un valor JSON, con la palabra que sale en el error.
func clase(raw json.RawMessage) string {
	s := strings.TrimSpace(string(raw))
	if s == "" {
		return "nada"
	}
	switch s[0] {
	case '"':
		return "texto"
	case '{':
		return "objeto"
	case '[':
		return "lista"
	case 't', 'f':
		return "booleano"
	case 'n':
		return "nulo"
	}
	return "número"
}

// clasesQueValen traduce un tipo de TypeScript a las clases de JSON que puede
// tomar. Devuelve nil cuando el tipo no se entiende: entonces no se exige
// nada, que es mejor que fallar por un tipo que esta prueba no sabe leer.
func clasesQueValen(t *testing.T, tipo string) map[string]bool {
	t.Helper()
	return clasesEn(t, textoDeTipos(t), tipo, 0)
}

func clasesEn(t *testing.T, texto, tipo string, prof int) map[string]bool {
	t.Helper()
	tipo = strings.TrimSpace(tipo)
	if tipo == "" || prof > 5 {
		return nil
	}
	out := map[string]bool{}
	for _, alt := range uniones(tipo) {
		alt = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(alt), ";"))
		for strings.HasPrefix(alt, "(") && strings.HasSuffix(alt, ")") {
			alt = strings.TrimSpace(alt[1 : len(alt)-1])
		}
		switch {
		case alt == "":
			continue
		case strings.HasSuffix(alt, "[]"), strings.HasPrefix(alt, "Array<"):
			out["lista"] = true
		case strings.HasPrefix(alt, "{"), strings.HasPrefix(alt, "Record<"), strings.HasPrefix(alt, "Partial<"):
			out["objeto"] = true
		case alt == "string", strings.HasPrefix(alt, "'"), strings.HasPrefix(alt, `"`), strings.HasPrefix(alt, "`"):
			out["texto"] = true
		case alt == "number":
			out["número"] = true
		case alt == "boolean", alt == "true", alt == "false":
			out["booleano"] = true
		case alt == "null", alt == "undefined":
			out["nulo"] = true
		case alt == "unknown", alt == "any", alt == "object":
			return nil
		case strings.Contains(texto, "export interface "+alt+" {"):
			out["objeto"] = true
		default:
			alias := aliasDe(texto, alt)
			if alias == "" {
				return nil // un tipo que esta prueba no sabe leer
			}
			mas := clasesEn(t, texto, alias, prof+1)
			if mas == nil {
				return nil
			}
			for k := range mas {
				out[k] = true
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// uniones parte un tipo por las barras de primer nivel: las de dentro de un
// objeto, de una lista o de unos paréntesis no cuentan.
func uniones(tipo string) []string {
	var out []string
	hondura, comilla, ini := 0, rune(0), 0
	for i, r := range tipo {
		switch {
		case comilla != 0:
			if r == comilla {
				comilla = 0
			}
		case r == '\'' || r == '"' || r == '`':
			comilla = r
		case r == '{' || r == '[' || r == '(' || r == '<':
			hondura++
		case r == '}' || r == ']' || r == ')' || r == '>':
			hondura--
		case r == '|' && hondura == 0:
			out = append(out, tipo[ini:i])
			ini = i + len("|")
		}
	}
	return append(out, tipo[ini:])
}

// aliasDe devuelve lo que hay a la derecha de `export type Nombre =`, aunque
// siga en varias líneas. Vacío si no existe ese alias.
func aliasDe(texto, nombre string) string {
	re := regexp.MustCompile(`(?m)^export type ` + regexp.QuoteMeta(nombre) + `\s*=`)
	m := re.FindStringIndex(texto)
	if m == nil {
		return ""
	}
	var b strings.Builder
	hondura := 0
	for _, linea := range strings.Split(texto[m[1]:], "\n") {
		limpia := strings.TrimSpace(sinComentario(linea))
		b.WriteString(" " + limpia)
		hondura += llaves(limpia)
		hecho := strings.TrimSpace(b.String())
		if hondura <= 0 && hecho != "" && !strings.HasSuffix(hecho, "|") {
			return hecho
		}
	}
	return strings.TrimSpace(b.String())
}

// claves devuelve las claves del primer nivel de un objeto JSON.
func clavesDe(t *testing.T, raw []byte) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	for k := range objetoDe(t, raw) {
		out[k] = true
	}
	return out
}

func objetoDe(t *testing.T, raw []byte) map[string]json.RawMessage {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("la respuesta no es un objeto JSON: %s", raw)
	}
	return m
}

// exige comprueba que el objeto trae todas las propiedades obligatorias de
// la interfaz, menos las que se declaren aparte (las que son de F2 y que la
// interfaz ya sabe pintar vacías). De las que sí vienen —obligatorias u
// opcionales— comprueba además el tipo del valor.
func exige(t *testing.T, quien string, raw []byte, interfaz string, salvo ...string) {
	t.Helper()
	compara(t, quien, raw, interfaz, false, salvo)
}

// exigeTodas comprueba que el objeto trae todas las propiedades de la
// interfaz, opcionales incluidas, menos las que se perdonen aparte.
func exigeTodas(t *testing.T, quien string, raw []byte, interfaz string, salvo ...string) {
	t.Helper()
	compara(t, quien, raw, interfaz, true, salvo)
}

func compara(t *testing.T, quien string, raw []byte, interfaz string, tambienOpcionales bool, salvo []string) {
	t.Helper()
	tiene := objetoDe(t, raw)
	perdonadas := map[string]bool{}
	for _, s := range salvo {
		perdonadas[s] = true
	}
	for _, p := range propiedadesDe(t, interfaz) {
		if perdonadas[p.Nombre] {
			continue
		}
		valor, hay := tiene[p.Nombre]
		if !hay {
			if !p.Opcional {
				t.Errorf("%s no manda %q, que %s declara obligatorio en web/src/lib/tipos.ts",
					quien, p.Nombre, interfaz)
			} else if tambienOpcionales {
				t.Errorf("%s no manda %q, que %s declara en web/src/lib/tipos.ts",
					quien, p.Nombre, interfaz)
			}
			continue
		}
		valen := clasesQueValen(t, p.Tipo)
		if valen == nil {
			continue
		}
		if got := clase(valor); !valen[got] {
			t.Errorf("%s manda %q como %s y %s.%s es %s en web/src/lib/tipos.ts: %s",
				quien, p.Nombre, got, interfaz, p.Nombre, strings.TrimSpace(p.Tipo), valor)
		}
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

	// Reglas: `titulo` es el nombre en texto, que es lo que la pantalla
	// ordena, busca y pinta en la carátula (con un objeto se caía Reglas
	// entera, modo sombra del 9 sept 2026).
	w = c.do("GET", "/api/v1/reglas", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("/reglas dio %d", w.Code)
	}
	var reglas []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &reglas); err != nil || len(reglas) == 0 {
		t.Fatalf("/reglas no devolvió la regla: %v %s", err, w.Body.String())
	}
	if nombre, ok := reglas[0]["titulo"].(string); !ok || nombre == "" {
		t.Fatalf("Regla.titulo tiene que ser el nombre en texto, como en tipos.ts: %v", reglas[0]["titulo"])
	}
	// Y la regla entera, campo a campo: `dias_restantes`, `releva_a`,
	// `live_source_id` y las demás, que antes nadie comparaba.
	exige(t, "GET /reglas (una regla)", primero(t, w.Body.Bytes()), "Regla")

	// El canal, que pinta el menú y media pantalla de Ajustes.
	w = c.do("GET", "/api/v1/canal", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("/canal dio %d: %s", w.Code, w.Body.String())
	}
	exige(t, "GET /canal", w.Body.Bytes(), "Canal")

	// Estado. `salidas` ya se sirve desde T2 de F2; `retorno_de_aire` y
	// `control_manual` son de tandas que todavía no están y la interfaz los
	// pinta vacíos.
	w = c.do("GET", "/api/v1/estado", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("/estado dio %d", w.Code)
	}
	exige(t, "GET /estado", w.Body.Bytes(), "Estado")

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

// ── emparejar títulos (F1-64 a F1-67) ─────────────────────────────────

// TestElEmparejadorMandaLoQueLaPantallaPinta compara las respuestas nuevas
// con lo que declara web/src/lib/tipos.ts.
func TestElEmparejadorMandaLoQueLaPantallaPinta(t *testing.T) {
	if !declaraLaInterfaz(t, "TituloSinEmparejar", "CandidatoDeTitulo", "ResultadoDeEmparejar") {
		t.Skip("web/src/lib/tipos.ts todavía no declara los tipos del emparejador")
	}
	c := nuevo(t).conClave().entrar()
	out := importarLaHoja(t, c)
	if len(out.SinEmparejar) == 0 {
		t.Fatal("la hoja no dejó nada por emparejar")
	}

	w := c.do("GET", "/api/v1/titulos/sin-emparejar", nil)
	uno := primero(t, w.Body.Bytes())
	exige(t, "GET /titulos/sin-emparejar", uno, "TituloSinEmparejar")

	var lista []tituloSinEmparejar
	c.json(w, &lista)
	var conCandidatos []byte
	var todos []json.RawMessage
	c.json(w, &todos)
	for i := range lista {
		if len(lista[i].Candidatos) > 0 {
			conCandidatos = todos[i]
		}
	}
	if conCandidatos == nil {
		t.Fatal("ninguno de los títulos por emparejar trae candidatos")
	}
	exige(t, "GET /titulos/sin-emparejar (un candidato)",
		primero(t, campo(t, conCandidatos, "candidatos")), "CandidatoDeTitulo")

	if declaraLaInterfaz(t, "TituloDelCatalogo") {
		w = c.do("GET", "/api/v1/titulos/buscar?q=kenshin", nil)
		exige(t, "GET /titulos/buscar", primero(t, w.Body.Bytes()), "TituloDelCatalogo")
	}

	// Y la respuesta de la decisión.
	kenshin := fichaLlamada(t, c, "Rurouni Kenshin")
	samurai := porNombre(out.SinEmparejar)["Samurai X"]
	w = c.do("POST", "/api/v1/titulos/"+itoa(samurai.ID)+"/emparejar",
		map[string]any{"accion": "usar", "title_id": kenshin.ID})
	if w.Code != http.StatusOK {
		t.Fatalf("emparejar dio %d: %s", w.Code, w.Body.String())
	}
	exigeTodas(t, "POST /titulos/{id}/emparejar", w.Body.Bytes(), "ResultadoDeEmparejar", "reglas_quitadas")
}

// ── la cuarentena y la bitácora en pantalla (issues #6 y #7) ──────────

// TestLaCuarentenaDiceComoSeLlamaElArchivo: la lista de cuarentena de
// Biblioteca pinta `titulo` como nombre de persona —el título o el episodio
// que usa el archivo—, y si nadie lo fichó, el nombre del archivo. Antes el
// servidor no lo mandaba y la fila salía sin nombre (F1-68).
func TestLaCuarentenaDiceComoSeLlamaElArchivo(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	ctx := context.Background()

	// Un episodio parado, con ficha.
	parado := model.MediaAsset{
		Path:        "/medios/space-cobra-e04.mp4",
		DurationMs:  22 * 60 * 1000,
		State:       model.AssetQuarantine,
		PlainReason: "El video se corta a los 12 segundos: el archivo llegó incompleto.",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := c.a.Store.Media.Insert(ctx, &parado); err != nil {
		t.Fatalf("no pude guardar el archivo: %v", err)
	}
	title := model.Title{Name: "Space Cobra", Kind: model.TitleSeries}
	if err := c.a.Store.Title.Insert(ctx, &title); err != nil {
		t.Fatalf("no pude guardar el título: %v", err)
	}
	ep := model.Episode{TitleID: title.ID, Season: 1, Number: 4, Name: "La joya", MediaAssetID: &parado.ID}
	if err := c.a.Store.Episode.Insert(ctx, &ep); err != nil {
		t.Fatalf("no pude guardar el episodio: %v", err)
	}
	// Y uno parado que nadie llegó a fichar.
	suelto := model.MediaAsset{
		Path:        "/medios/Promos/promo-verano.mov",
		State:       model.AssetQuarantine,
		PlainReason: "No se pudo leer el archivo.",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := c.a.Store.Media.Insert(ctx, &suelto); err != nil {
		t.Fatalf("no pude guardar el archivo: %v", err)
	}

	w := c.do("GET", "/api/v1/cuarentena", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("/cuarentena dio %d: %s", w.Code, w.Body.String())
	}
	exige(t, "GET /cuarentena", primero(t, w.Body.Bytes()), "EnCuarentena")

	var lista []map[string]any
	c.json(w, &lista)
	titulos := map[string]string{}
	for _, fila := range lista {
		titulos[fila["ruta"].(string)] = fila["titulo"].(string)
	}
	if got := titulos[parado.Path]; got != "Space Cobra · T1E4 La joya" {
		t.Errorf("el episodio parado se llama %q; tenía que ser «Space Cobra · T1E4 La joya»", got)
	}
	if got := titulos[suelto.Path]; got != "promo-verano" {
		t.Errorf("el archivo sin ficha se llama %q; tenía que ser el nombre del archivo, «promo-verano»", got)
	}

	// Y mientras haya algo parado, Al aire lo avisa con camino a Biblioteca.
	c.a.RefreshCuarentena(ctx)
	w = c.do("GET", "/api/v1/estado", nil)
	var estado struct {
		Alarmas []struct {
			Tipo   string `json:"tipo"`
			Nivel  string `json:"nivel"`
			Texto  string `json:"texto"`
			Accion *struct {
				Ruta string `json:"ruta"`
			} `json:"accion"`
		} `json:"alarmas"`
	}
	c.json(w, &estado)
	hay := false
	for _, al := range estado.Alarmas {
		if al.Texto == "2 archivos en cuarentena" {
			hay = true
			if al.Nivel != "aviso" {
				t.Errorf("la cuarentena es un aviso, no %q: el sistema no regaña", al.Nivel)
			}
			if al.Accion == nil || al.Accion.Ruta != "/biblioteca" {
				t.Errorf("la alarma de cuarentena tiene que llevar a /biblioteca: %+v", al.Accion)
			}
		}
	}
	if !hay {
		t.Errorf("/estado no avisa de los 2 archivos en cuarentena: %s", w.Body.String())
	}

	// Al dejar pasar uno, el aviso baja a 1; al dejar pasar el otro, se apaga.
	if w := c.do("POST", "/api/v1/cuarentena/"+itoa(suelto.ID)+"/dejar-pasar", map[string]string{"quien": "Rolando"}); w.Code != http.StatusOK {
		t.Fatalf("dejar pasar dio %d: %s", w.Code, w.Body.String())
	}
	if !tieneAlarma(c, "1 archivo en cuarentena") {
		t.Error("después de dejar pasar uno, el aviso tenía que decir «1 archivo en cuarentena»")
	}
	if w := c.do("POST", "/api/v1/cuarentena/"+itoa(parado.ID)+"/dejar-pasar", map[string]string{"quien": "Rolando"}); w.Code != http.StatusOK {
		t.Fatalf("dejar pasar dio %d: %s", w.Code, w.Body.String())
	}
	if tieneAlarma(c, "1 archivo en cuarentena") || tieneAlarma(c, "2 archivos en cuarentena") {
		t.Error("sin nada en cuarentena, el aviso tenía que apagarse")
	}
}

func tieneAlarma(c *cliente, texto string) bool {
	w := c.do("GET", "/api/v1/estado", nil)
	var estado struct {
		Alarmas []struct {
			Texto string `json:"texto"`
		} `json:"alarmas"`
	}
	c.json(w, &estado)
	for _, al := range estado.Alarmas {
		if al.Texto == texto {
			return true
		}
	}
	return false
}

// TestLaBitacoraMandaLoQueLaPantallaPinta: GET /incidentes trae cada fila con
// lo que `Incidente` declara y, además, la frase en cristiano de su tipo,
// para que Al aire no tenga que saber qué es un `salto_de_reloj` (F1-69).
func TestLaBitacoraMandaLoQueLaPantallaPinta(t *testing.T) {
	c := nuevo(t).conClave().entrar()

	c.a.Incident("cuarentena", "space-cobra-e04.mp4: Este video no trae sonido.")
	c.a.Incident("panico_ingest", "se cayó leyendo la carpeta")
	c.a.Incident("algo_nuevo_de_f2", "un tipo que la lista todavía no conoce")

	w := c.do("GET", "/api/v1/incidentes", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("/incidentes dio %d: %s", w.Code, w.Body.String())
	}
	exige(t, "GET /incidentes", primero(t, w.Body.Bytes()), "Incidente")

	var lista []map[string]any
	c.json(w, &lista)
	if len(lista) != 3 {
		t.Fatalf("la bitácora tenía 3 incidentes y devolvió %d: %s", len(lista), w.Body.String())
	}
	textos := map[string]string{}
	for _, fila := range lista {
		textos[fila["tipo"].(string)] = fila["texto"].(string)
	}
	casos := map[string]string{
		"cuarentena":       "Un archivo quedó en cuarentena",
		"panico_ingest":    "Una parte del sistema falló y se relanzó sola (ingest)",
		"algo_nuevo_de_f2": "algo nuevo de f2",
	}
	for tipo, quiere := range casos {
		if textos[tipo] != quiere {
			t.Errorf("el incidente %q se enseña como %q; tenía que ser %q", tipo, textos[tipo], quiere)
		}
	}

	// El rango: pedir solo el futuro no trae nada; un rango mal escrito es 400.
	desde := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	w = c.do("GET", "/api/v1/incidentes?desde="+desde, nil)
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Errorf("pedir la bitácora del futuro tenía que dar una lista vacía: %d %s", w.Code, w.Body.String())
	}
	if w := c.do("GET", "/api/v1/incidentes?desde=ayer", nil); w.Code != http.StatusBadRequest {
		t.Errorf("una fecha mal escrita tenía que ser 400 con frase en cristiano: %d %s", w.Code, w.Body.String())
	}
}

// ── el empujón del WebSocket contra GET /estado ────────────────────────

// TestElEmpujonDelWebSocketEsElEstadoEntero: la interfaz fusiona cada empujón
// con lo que ya tenía (`{...antes, ...e}` en lib/estado.tsx), así que una clave
// que el empujón omite se queda con el valor viejo para siempre. Con un hueco
// —nada al aire ahora mismo— eso dejaba en pantalla el programa anterior hasta
// la próxima recarga. El empujón manda el contrato entero de `Estado`, y
// `al_aire`/`siguiente` van siempre, en `null` cuando no hay nada.
func TestElEmpujonDelWebSocketEsElEstadoEntero(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	marco := primerMarcoDelWS(t, c)

	crudo, err := json.Marshal(marco)
	if err != nil {
		t.Fatalf("el marco no se puede volver a serializar: %v", err)
	}
	// Lo mismo que exige GET /estado: lo que falta son opcionales de tandas
	// que no están (`retorno_de_aire`, `control_manual`) y los tres que la
	// interfaz solo necesita del primer /estado.
	exige(t, "el empujón del WebSocket", crudo, "Estado")

	for _, clave := range []string{"al_aire", "siguiente"} {
		valor, hay := marco[clave]
		if !hay {
			t.Fatalf("el empujón no manda %q; con un hueco la pantalla se queda con el programa anterior", clave)
		}
		if clase(valor) != "nulo" {
			t.Errorf("sin nada al aire, %q tenía que ser nulo y llegó %s: %s", clave, clase(valor), valor)
		}
	}
}

// primerMarcoDelWS abre el WebSocket como lo abriría el navegador y devuelve
// el primer marco de estado. Hace falta un servidor de verdad: el empujón
// necesita quedarse con la conexión (Hijack) y httptest.NewRecorder no puede.
func primerMarcoDelWS(t *testing.T, c *cliente) map[string]json.RawMessage {
	t.Helper()
	srv := httptest.NewServer(c.s)
	t.Cleanup(srv.Close)

	conn, err := net.Dial("tcp", strings.TrimPrefix(srv.URL, "http://"))
	if err != nil {
		t.Fatalf("no pude conectar: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	peticion := "GET /api/v1/ws HTTP/1.1\r\n" +
		"Host: " + strings.TrimPrefix(srv.URL, "http://") + "\r\n" +
		"Upgrade: websocket\r\nConnection: Upgrade\r\n" +
		"Sec-WebSocket-Version: 13\r\n" +
		"Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n" +
		"Cookie: " + c.cookie.Name + "=" + c.cookie.Value + "\r\n\r\n"
	if _, err := conn.Write([]byte(peticion)); err != nil {
		t.Fatalf("no pude mandar el apretón: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))

	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatalf("no llegó respuesta al apretón: %v", err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("el apretón dio %d, se esperaba 101", resp.StatusCode)
	}
	// La firma del RFC 6455 para esa clave de ejemplo.
	if got := resp.Header.Get("Sec-WebSocket-Accept"); got != "s3pPLMBiTxaQ9kYGzzhZRbK+xOo=" {
		t.Fatalf("la firma del apretón es %q", got)
	}

	var head [2]byte
	if _, err := io.ReadFull(br, head[:]); err != nil {
		t.Fatalf("no llegó ningún marco: %v", err)
	}
	if op := head[0] & 0x0F; op != opText {
		t.Fatalf("el primer marco es del tipo %d, se esperaba texto", op)
	}
	n := int64(head[1] & 0x7F)
	switch n {
	case 126:
		var ext [2]byte
		if _, err := io.ReadFull(br, ext[:]); err != nil {
			t.Fatal(err)
		}
		n = int64(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte
		if _, err := io.ReadFull(br, ext[:]); err != nil {
			t.Fatal(err)
		}
		n = int64(binary.BigEndian.Uint64(ext[:]))
	}
	carga := make([]byte, n)
	if _, err := io.ReadFull(br, carga); err != nil {
		t.Fatalf("el marco vino cortado: %v", err)
	}
	var marco map[string]json.RawMessage
	if err := json.Unmarshal(carga, &marco); err != nil {
		t.Fatalf("el marco no es JSON: %s", carga)
	}
	return marco
}

// ── importar la hoja y confirmar un relevo ────────────────────────────

// TestConfirmarUnRelevoDeLaHoja es el viaje completo del importador: se pega
// la hoja de CAtv, el servidor propone un relevo, la pantalla devuelve la
// propuesta tal cual y la regla queda con `releva_a` puesto.
//
// El nombre de los campos es el contrato: mientras el servidor mandó
// `regla_que_vence`/`regla_que_releva` y la interfaz esperaba `regla`/
// `releva_a`, cada clic mandaba dos `undefined` y contestaba 400 «no encuentro
// la regla 0» (auditoría de contrato, 11 sept 2026). Aquí el cuerpo se manda
// **tal como llegó**: si los nombres se separan otra vez, esto falla.
func TestConfirmarUnRelevoDeLaHoja(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	raw, err := os.ReadFile(hojaPath)
	if err != nil {
		t.Fatalf("no pude leer la hoja de CAtv: %v", err)
	}
	w := c.do("POST", "/api/v1/importar/hoja", map[string]string{"texto": string(raw)})
	if w.Code != http.StatusOK {
		t.Fatalf("importar la hoja dio %d: %s", w.Code, w.Body.String())
	}
	exige(t, "POST /importar/hoja", w.Body.Bytes(), "ResumenDeImportacion")

	propuestas := campo(t, w.Body.Bytes(), "relevos_propuestos")
	uno := primero(t, propuestas)
	exige(t, "POST /importar/hoja (un relevo propuesto)", uno, "RelevoPropuesto")

	// La propuesta, devuelta sin tocar nada: es lo que hace la pantalla.
	w = c.do("POST", "/api/v1/importar/confirmar-relevos", json.RawMessage(propuestas))
	if w.Code != http.StatusOK {
		t.Fatalf("confirmar los relevos dio %d: %s", w.Code, w.Body.String())
	}

	var rel struct {
		Regla   int64 `json:"regla"`
		RelevaA int64 `json:"releva_a"`
		Texto   string
	}
	if err := json.Unmarshal(uno, &rel); err != nil {
		t.Fatalf("el relevo propuesto no se entiende: %v", err)
	}
	if rel.Regla == 0 || rel.RelevaA == 0 {
		t.Fatalf("el relevo propuesto tiene que traer las dos reglas de la base: %s", uno)
	}

	// Y la regla quedó con el relevo puesto, con nombre y todo.
	w = c.do("GET", "/api/v1/reglas", nil)
	var reglas []struct {
		ID            int64  `json:"id"`
		RelevaA       *int64 `json:"releva_a"`
		RelevaATitulo string `json:"releva_a_titulo"`
	}
	c.json(w, &reglas)
	encontrada := false
	for _, r := range reglas {
		if r.ID != rel.Regla {
			continue
		}
		encontrada = true
		if r.RelevaA == nil || *r.RelevaA != rel.RelevaA {
			t.Fatalf("la regla %d no quedó relevando a la %d: %+v", r.ID, rel.RelevaA, r.RelevaA)
		}
		if r.RelevaATitulo == "" {
			t.Error("la regla releva a otra y el servidor no dice a qué programa: la etiqueta «releva a …» queda en blanco")
		}
	}
	if !encontrada {
		t.Fatalf("la regla %d del relevo no está en /reglas", rel.Regla)
	}
}

// ── las alarmas y los ajustes ─────────────────────────────────────────

// TestUnaAlarmaLlegaComoLaPantallaLaPinta: Al aire pinta cada alarma con su
// nivel, su frase y, si la hay, el camino a la pantalla donde se arregla.
func TestUnaAlarmaLlegaComoLaPantallaLaPinta(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	c.enCuarentena("/medios/promo-verano.mov", "No se pudo leer el archivo.")
	c.a.RefreshCuarentena(context.Background())

	w := c.do("GET", "/api/v1/estado", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("/estado dio %d", w.Code)
	}
	alarmas := campo(t, w.Body.Bytes(), "alarmas")
	una := primero(t, alarmas)
	// `detalle` es opcional de verdad: la cuarentena no lo manda.
	exigeTodas(t, "GET /estado (una alarma)", una, "Alarma", "detalle")

	accion := objetoDe(t, campo(t, una, "accion"))
	for _, clave := range []string{"texto", "ruta"} {
		if clase(accion[clave]) != "texto" {
			t.Errorf("el botón de la alarma no trae %q como texto: %s", clave, una)
		}
	}
}

// TestLosAjustesQueLaPantallaLeeLosManadaElServidor es la prueba de humo de
// `Ajustes`, que no se puede comparar con `exige` porque el tipo de la
// interfaz es `Record<string, string>`, no una interfaz con propiedades.
//
// Comprueba lo que de verdad se rompió: media pantalla de Ajustes leía claves
// que el servidor no tiene ni tendrá (`respaldo_ultimo`, `aceleracion_tarjeta`,
// `dias_al_aire`…) y quedaba en blanco, o pintaba «hace NaN días». Así que
// aquí se leen las claves que la pantalla pide de verdad y se exige que el
// servidor las conozca.
func TestLosAjustesQueLaPantallaLeeLosManadaElServidor(t *testing.T) {
	const pantalla = "../../web/src/pantallas/Ajustes.tsx"
	raw, err := os.ReadFile(pantalla)
	if err != nil {
		t.Fatalf("no pude leer la pantalla de Ajustes: %v", err)
	}
	// Las claves que el servidor conoce: las de app.Key* que se guardan desde
	// Ajustes, más los tres de fábrica del detector.
	conocidas := map[string]bool{}
	for _, k := range []string{
		app.KeyCountry, app.KeyQuality, app.KeySubtitulosEstado, app.KeyAudioLanguage,
		app.KeyNoticeChannel, app.KeyTelegramToken, app.KeyTelegramChat, app.KeyNoticeMailTo,
		app.KeySMTPServer, app.KeySMTPUser, app.KeySMTPPass, app.KeyOnlineInfo, app.KeyTMDBKey,
		app.KeyGuideHTTP, app.KeyGuidePMCPHTTP,
		app.KeySilencioUmbral, app.KeyNegroUmbral, app.KeySilencioDevuelveControl,
	} {
		conocidas[k] = true
	}
	// asistente_ia es memoria pura del interruptor de Ajustes: nadie la lee en
	// el servidor, pero el almacén de ajustes es clave→texto libre (ajustesPut
	// no la rechaza), así que guardarla y devolverla ya funciona sin que el
	// servidor tenga que saber qué significa.
	conocidas["asistente_ia"] = true
	reLee := regexp.MustCompile(`ajustes\.(\w+)`)
	pedidas := map[string]bool{}
	for _, m := range reLee.FindAllStringSubmatch(string(raw), -1) {
		pedidas[m[1]] = true
		if !conocidas[m[1]] {
			t.Errorf("Ajustes.tsx lee el ajuste %q y el servidor no lo manda nunca: "+
				"o lo manda el servidor, o la pantalla no lo pinta", m[1])
		}
	}
	if len(pedidas) == 0 {
		t.Fatal("no encontré ni un ajuste leído en la pantalla: ¿cambió la forma de leerlos?")
	}

	// Y los tres de fábrica llegan siempre, desde la primera vez que se abre.
	c := nuevo(t).conClave().entrar()
	w := c.do("GET", "/api/v1/ajustes", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("/ajustes dio %d: %s", w.Code, w.Body.String())
	}
	ajustes := objetoDe(t, w.Body.Bytes())
	for _, k := range []string{app.KeySilencioUmbral, app.KeyNegroUmbral, app.KeySilencioDevuelveControl} {
		if clase(ajustes[k]) != "texto" {
			t.Errorf("GET /ajustes no manda %q como texto: los ajustes viajan como clave→texto", k)
		}
	}
}
