package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"antena787/internal/app"
	"antena787/internal/ingest"
	"antena787/internal/model"
	"antena787/internal/resolver"
)

// La hoja real de CAtv, tal como la mandó Rolando el 4 de septiembre de 2026.
const hojaPath = "../../docs/catv-sheet-2026-09-04.md"

// La clave de estación de las pruebas.
const clavePrueba = "1234"

// ── el andamio ────────────────────────────────────────────────────────

// cliente es un navegador de mentira: guarda la cookie de sesión, como haría
// el de verdad.
type cliente struct {
	t      *testing.T
	s      *Server
	a      *app.App
	cookie *http.Cookie
}

func nuevo(t *testing.T) *cliente {
	t.Helper()
	a, err := app.Open(app.Options{DataDir: t.TempDir(), Version: "prueba"})
	if err != nil {
		t.Fatalf("no pude abrir la aplicación: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return &cliente{t: t, s: New(a, Options{}), a: a}
}

// conClave deja la clave puesta, como la habría dejado el paso 1 del asistente.
func (c *cliente) conClave() *cliente {
	c.t.Helper()
	if err := c.a.Store.Settings.SetPIN(context.Background(), clavePrueba); err != nil {
		c.t.Fatalf("no pude poner la clave: %v", err)
	}
	return c
}

func (c *cliente) do(method, path string, body any) *httptest.ResponseRecorder {
	c.t.Helper()
	var r *http.Request
	if body == nil {
		r = httptest.NewRequest(method, path, nil)
	} else {
		raw, err := json.Marshal(body)
		if err != nil {
			c.t.Fatalf("no pude serializar el cuerpo: %v", err)
		}
		r = httptest.NewRequest(method, path, bytes.NewReader(raw))
		r.Header.Set("Content-Type", "application/json")
	}
	if c.cookie != nil {
		r.AddCookie(c.cookie)
	}
	w := httptest.NewRecorder()
	c.s.ServeHTTP(w, r)
	for _, ck := range w.Result().Cookies() {
		if ck.Name == CookieName && ck.Value != "" {
			c.cookie = ck
		}
	}
	return w
}

// entrar mete la clave buena y se queda con la cookie.
func (c *cliente) entrar() *cliente {
	c.t.Helper()
	w := c.do("POST", "/api/v1/entrar", map[string]string{"clave": clavePrueba})
	if w.Code != http.StatusOK {
		c.t.Fatalf("entrar con la clave buena dio %d: %s", w.Code, w.Body.String())
	}
	if c.cookie == nil {
		c.t.Fatal("entrar no dejó la cookie de sesión")
	}
	return c
}

func (c *cliente) json(w *httptest.ResponseRecorder, into any) {
	c.t.Helper()
	if err := json.Unmarshal(w.Body.Bytes(), into); err != nil {
		c.t.Fatalf("la respuesta no es JSON (%d): %s", w.Code, w.Body.String())
	}
}

// conMaterial deja un título con archivo listo para aire, que es lo que una
// regla necesita para poder guardarse.
func (c *cliente) conMaterial(nombre string) model.Title {
	c.t.Helper()
	ctx := context.Background()
	asset := model.MediaAsset{
		Path:           "/medios/" + nombre + ".mkv",
		DurationMs:     30 * 60 * 1000,
		State:          model.AssetReady,
		NormalizeState: ingest.NormalizeReady,
		NormalizedPath: "/medios/" + nombre + ".norm.mkv",
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	if err := c.a.Store.Media.Insert(ctx, &asset); err != nil {
		c.t.Fatalf("no pude guardar el archivo: %v", err)
	}
	title := model.Title{Name: nombre, Kind: model.TitleProgram, MediaAssetID: &asset.ID}
	if err := c.a.Store.Title.Insert(ctx, &title); err != nil {
		c.t.Fatalf("no pude guardar el título: %v", err)
	}
	return title
}

func hoy(c *cliente) model.Day {
	ch, err := c.a.Store.Channel.Get(context.Background(), c.a.ChannelID)
	if err != nil {
		c.t.Fatalf("no pude leer el canal: %v", err)
	}
	return ch.BroadcastDay(time.Now().UTC())
}

// ── la clave ──────────────────────────────────────────────────────────

func TestEntrarConClaveMalaYBuena(t *testing.T) {
	c := nuevo(t).conClave()

	w := c.do("POST", "/api/v1/entrar", map[string]string{"clave": "9999"})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("la clave mala dio %d, se esperaba 401: %s", w.Code, w.Body.String())
	}
	var e errorBody
	c.json(w, &e)
	if !strings.Contains(e.Error, "clave de la estación") {
		t.Fatalf("el error de la clave mala dice %q y no se entiende", e.Error)
	}
	if c.cookie != nil {
		t.Fatal("la clave mala dejó cookie")
	}

	c.entrar()
	if !c.cookie.HttpOnly || c.cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("la cookie tiene que ser HttpOnly y SameSite=Strict; es %+v", c.cookie)
	}
}

func TestSinCookieTodoEs401(t *testing.T) {
	c := nuevo(t).conClave()
	for _, ruta := range []string{
		"/api/v1/reglas", "/api/v1/plan", "/api/v1/biblioteca",
		"/api/v1/ajustes", "/api/v1/incidentes", "/api/v1/auditoria",
	} {
		if w := c.do("GET", ruta, nil); w.Code != http.StatusUnauthorized {
			t.Fatalf("%s sin cookie dio %d, se esperaba 401", ruta, w.Code)
		}
	}
	// El estado y la guía sí contestan: son las dos puertas abiertas a
	// propósito (docs/API.md).
	if w := c.do("GET", "/api/v1/estado", nil); w.Code != http.StatusOK {
		t.Fatalf("/estado sin cookie dio %d, se esperaba 200", w.Code)
	}
	if w := c.do("GET", "/guia.xml", nil); w.Code != http.StatusOK {
		t.Fatalf("/guia.xml sin cookie dio %d, se esperaba 200", w.Code)
	}
}

func TestEstadoDiceQueFaltaInstalar(t *testing.T) {
	c := nuevo(t) // sin clave todavía
	w := c.do("GET", "/api/v1/estado", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("/estado dio %d", w.Code)
	}
	var body map[string]any
	c.json(w, &body)
	if body["necesita_instalacion"] != true {
		t.Fatalf("sin clave, /estado tiene que decir necesita_instalacion: %v", body)
	}
}

func TestLimiteDeIntentosDeClave(t *testing.T) {
	c := nuevo(t).conClave()
	for i := 0; i < MaxAttempts; i++ {
		if w := c.do("POST", "/api/v1/entrar", map[string]string{"clave": "0000"}); w.Code != http.StatusUnauthorized {
			t.Fatalf("el intento %d dio %d, se esperaba 401", i+1, w.Code)
		}
	}
	w := c.do("POST", "/api/v1/entrar", map[string]string{"clave": clavePrueba})
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("el sexto intento en un minuto dio %d, se esperaba 429", w.Code)
	}
}

// ── las reglas ────────────────────────────────────────────────────────

func TestCrearReglaValida(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	title := c.conMaterial("Los Simuladores")
	today := hoy(c)

	w := c.do("POST", "/api/v1/reglas", map[string]any{
		"tipo":                  "normal",
		"title_id":              title.ID,
		"patron_de_dias":        "LMMJV__",
		"hora":                  13 * 60,
		"duracion_slot_ms":      60 * 60 * 1000,
		"fecha_inicio":          string(today),
		"fecha_fin":             string(today.Add(30)),
		"episodios_por_corrida": 1,
		"activa":                true,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("crear la regla dio %d: %s", w.Code, w.Body.String())
	}
	var out ruleOut
	c.json(w, &out)
	if out.ID == 0 || out.Ficha == nil || out.Ficha.Name != "Los Simuladores" {
		t.Fatalf("la regla creada no trae su título: %+v", out)
	}
	if out.Pattern == "" {
		t.Fatal("la regla no dice su patrón en cristiano")
	}
}

func TestReglaConFechasCruzadas(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	title := c.conMaterial("Hellsing")

	w := c.do("POST", "/api/v1/reglas", map[string]any{
		"title_id":       title.ID,
		"patron_de_dias": "L_____D",
		"hora":           0,
		"fecha_inicio":   "2026-09-15",
		"fecha_fin":      "2026-01-01",
		"activa":         true,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("la regla con fechas cruzadas dio %d, se esperaba 400: %s", w.Code, w.Body.String())
	}
	var e errorBody
	c.json(w, &e)
	const quiero = "la fecha de fin (15 sep 2026) es anterior a la de inicio"
	if !strings.HasPrefix(e.Error, "la fecha de fin") || !strings.Contains(e.Error, "anterior a la de inicio") {
		t.Fatalf("el error dice:\n  %s\ny tenía que empezar por %q", e.Error, quiero)
	}
	if e.Field != "fecha_fin" {
		t.Fatalf("el error apunta al campo %q, se esperaba fecha_fin", e.Field)
	}
}

func TestReglaConPatronInvalido(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	title := c.conMaterial("Zoids")
	today := hoy(c)

	w := c.do("POST", "/api/v1/reglas", map[string]any{
		"title_id":       title.ID,
		"patron_de_dias": "XXXXXXX",
		"hora":           15 * 60,
		"fecha_inicio":   string(today),
		"fecha_fin":      string(today.Add(7)),
		"activa":         true,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("el patrón inválido dio %d, se esperaba 400", w.Code)
	}
	var e errorBody
	c.json(w, &e)
	if !strings.Contains(e.Error, "LMMJVSD") || e.Field != "patron_de_dias" {
		t.Fatalf("el error del patrón no ayuda: %+v", e)
	}
}

func TestReglaSinMaterialListo(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	title := model.Title{Name: "Serie sin archivo", Kind: model.TitleSeries}
	if err := c.a.Store.Title.Insert(context.Background(), &title); err != nil {
		t.Fatalf("no pude guardar el título: %v", err)
	}
	today := hoy(c)

	w := c.do("POST", "/api/v1/reglas", map[string]any{
		"title_id":       title.ID,
		"patron_de_dias": "LMMJVSD",
		"hora":           9 * 60,
		"fecha_inicio":   string(today),
		"fecha_fin":      string(today.Add(3)),
		"activa":         true,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("la regla sin material dio %d, se esperaba 400: %s", w.Code, w.Body.String())
	}
	var e errorBody
	c.json(w, &e)
	if !strings.Contains(e.Error, "todavía no tiene material listo para aire") {
		t.Fatalf("el error no lo explica: %q", e.Error)
	}
}

func TestReglaSoloHoyCreaUnaExcepcion(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	title := c.conMaterial("Gaming Longplays")
	otro := c.conMaterial("Astroboy")
	today := hoy(c)

	w := c.do("POST", "/api/v1/reglas", map[string]any{
		"title_id":       title.ID,
		"patron_de_dias": "LMMJVSD",
		"hora":           18 * 60,
		"fecha_inicio":   string(today),
		"fecha_fin":      string(today.Add(90)),
		"activa":         true,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("crear la regla dio %d: %s", w.Code, w.Body.String())
	}
	var base ruleOut
	c.json(w, &base)

	w = c.do("PUT", "/api/v1/reglas/"+itoa(base.ID)+"?solo_hoy=1", map[string]any{
		"title_id": otro.ID,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("la excepción de hoy dio %d: %s", w.Code, w.Body.String())
	}
	var exc ruleOut
	c.json(w, &exc)
	if exc.ID == base.ID {
		t.Fatal("solo_hoy cambió la regla en vez de crear una excepción")
	}
	if exc.From != today || exc.To != today {
		t.Fatalf("la excepción va de %s a %s; tenía que ser solo hoy (%s)", exc.From, exc.To, today)
	}
	if exc.HandsOffTo == nil || *exc.HandsOffTo != base.ID {
		t.Fatalf("la excepción no releva a la regla de siempre: %+v", exc.HandsOffTo)
	}

	// Y la de siempre sigue intacta.
	original, err := c.a.Store.Rule.Get(context.Background(), base.ID)
	if err != nil {
		t.Fatalf("no encuentro la regla original: %v", err)
	}
	if original.To != today.Add(90) || *original.TitleID != title.ID {
		t.Fatalf("la regla de siempre se tocó: %+v", original)
	}
}

// ── el plan y la guía ─────────────────────────────────────────────────

func TestRecalcularDevuelveBloquesYAvisos(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	title := c.conMaterial("Los Simuladores")
	today := hoy(c)

	w := c.do("POST", "/api/v1/reglas", map[string]any{
		"title_id":         title.ID,
		"patron_de_dias":   "LMMJVSD",
		"hora":             13 * 60,
		"duracion_slot_ms": 30 * 60 * 1000,
		"fecha_inicio":     string(today),
		"fecha_fin":        string(today.Add(3)), // vence pronto: tiene que avisar
		"activa":           true,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("crear la regla dio %d: %s", w.Code, w.Body.String())
	}

	w = c.do("POST", "/api/v1/plan/recalcular", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("recalcular dio %d: %s", w.Code, w.Body.String())
	}
	var out struct {
		Bloques int                `json:"bloques"`
		Avisos  []resolver.Warning `json:"avisos"`
	}
	c.json(w, &out)
	if out.Bloques == 0 {
		t.Fatalf("recalcular no puso nada en el plan: %s", w.Body.String())
	}
	if len(out.Avisos) == 0 {
		t.Fatal("una regla que vence en tres días tenía que avisar de algo")
	}
	for _, a := range out.Avisos {
		if strings.TrimSpace(a.Text) == "" {
			t.Fatalf("hay un aviso sin texto: %+v", a)
		}
	}

	// Y el día se puede pedir, con sus huecos calculados.
	w = c.do("GET", "/api/v1/plan?dia="+string(today), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("el plan del día dio %d: %s", w.Code, w.Body.String())
	}
	var dia struct {
		Items []map[string]any `json:"items"`
	}
	c.json(w, &dia)
	if len(dia.Items) == 0 {
		t.Fatal("el plan del día llegó vacío")
	}
	huecos := 0
	for _, it := range dia.Items {
		if it["hueco"] == true {
			huecos++
		}
	}
	if huecos == 0 {
		t.Fatal("un día con una sola regla tiene huecos y el plan no los cuenta")
	}
}

func TestGuiaXMLEsValida(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	title := c.conMaterial("Kojak")
	today := hoy(c)

	w := c.do("POST", "/api/v1/reglas", map[string]any{
		"title_id":         title.ID,
		"patron_de_dias":   "LMMJVSD",
		"hora":             8 * 60,
		"duracion_slot_ms": 30 * 60 * 1000,
		"fecha_inicio":     string(today),
		"fecha_fin":        string(today.Add(60)),
		"activa":           true,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("crear la regla dio %d: %s", w.Code, w.Body.String())
	}
	if w := c.do("POST", "/api/v1/plan/recalcular", nil); w.Code != http.StatusOK {
		t.Fatalf("recalcular dio %d: %s", w.Code, w.Body.String())
	}

	w = c.do("GET", "/guia.xml", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("/guia.xml dio %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "xml") {
		t.Fatalf("la guía se sirve como %q", ct)
	}
	// El validador se queja de los huecos sin describir, que en un canal con
	// una sola regla son reales y no son un error de la guía. De lo demás no
	// puede quejarse.
	for _, p := range resolver.ValidateXMLTV(w.Body.Bytes()) {
		if strings.Contains(p, "sin describir") {
			continue
		}
		t.Fatalf("la guía no es publicable: %s", p)
	}

	// Y la guía contra el plan coincide consigo misma. Se pregunta por el día
	// del primer bloque que la guía anuncia: según la hora a la que corra la
	// prueba, el de las 8:00 de hoy ya pasó y el primero es el de mañana.
	var anunciado model.Day
	for _, it := range c.a.GuideItems() {
		if it.Origin == model.OriginAsset {
			anunciado = it.BroadcastDay
			break
		}
	}
	if anunciado == "" {
		t.Fatal("la guía no anuncia ni un bloque de contenido")
	}
	w = c.do("GET", "/api/v1/guia?dia="+string(anunciado), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("/guia dio %d: %s", w.Code, w.Body.String())
	}
	// La guía contra el plan viaja como {"dia":…, "filas":[…]}, que es lo que
	// pinta la pantalla Parrilla · Guía (web/src/lib/tipos.ts, `Guia`).
	var comparacion struct {
		Dia   model.Day        `json:"dia"`
		Filas []map[string]any `json:"filas"`
	}
	c.json(w, &comparacion)
	filas := comparacion.Filas
	if len(filas) == 0 {
		t.Fatal("la comparación de guía contra plan llegó vacía")
	}
	if comparacion.Dia != anunciado {
		t.Fatalf("la comparación dice ser del día %q y se pidió la del %q", comparacion.Dia, anunciado)
	}
	for _, f := range filas {
		if f["coincide"] != true {
			t.Fatalf("la guía no coincide con el plan que la generó: %+v", f)
		}
	}
}

func TestLlenarConDiferido(t *testing.T) {
	c := nuevo(t).conClave().entrar()

	w := c.do("POST", "/api/v1/plan/llenar-con-diferido", map[string]string{
		"desde": "01:00", "hasta": "06:00",
		"origen_desde": "07:00", "origen_hasta": "12:00",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("llenar con diferido dio %d: %s", w.Code, w.Body.String())
	}
	var out struct {
		Regla ruleOut `json:"regla"`
	}
	c.json(w, &out)
	if out.Regla.Kind != model.RuleTimeShift {
		t.Fatalf("la regla creada es %q, se esperaba diferido", out.Regla.Kind)
	}
	if out.Regla.SourceWindowStart == nil || out.Regla.SourceWindowEnd == nil {
		t.Fatal("el diferido no dice qué ventana repite")
	}
	if got := out.Regla.SlotMs; got != 5*60*60*1000 {
		t.Fatalf("el diferido dura %d ms, se esperaban cinco horas", got)
	}
}

// ── importar la hoja de CAtv ──────────────────────────────────────────

type hojaOut struct {
	ReglasCreadas  int                   `json:"reglas_creadas"`
	TitulosCreados int                   `json:"titulos_creados"`
	Relevos        []relevoPropuesto     `json:"relevos_propuestos"`
	Repeticiones   []repeticionPropuesta `json:"repeticiones_propuestas"`
	Filas          []filaConError        `json:"filas_con_error"`
	Fechas         []fechaCorrida        `json:"fechas_corridas"`
	Resumen        string                `json:"resumen"`
}

func TestImportarLaHojaDeCAtv(t *testing.T) {
	raw, err := os.ReadFile(hojaPath)
	if err != nil {
		t.Fatalf("no pude leer la hoja de CAtv: %v", err)
	}
	c := nuevo(t).conClave().entrar()

	w := c.do("POST", "/api/v1/importar/hoja", map[string]string{"texto": string(raw)})
	if w.Code != http.StatusOK {
		t.Fatalf("importar la hoja dio %d: %s", w.Code, w.Body.String())
	}
	var out hojaOut
	c.json(w, &out)

	if out.ReglasCreadas != 34 {
		t.Fatalf("se crearon %d reglas, se esperaban 34 (%s)", out.ReglasCreadas, out.Resumen)
	}
	if out.TitulosCreados == 0 {
		t.Fatal("no se creó ningún título")
	}

	// Hellsing: la fecha de fin es anterior a la de inicio. Ni se importa, ni
	// tumba la hoja entera (auditoría B11).
	var hellsing *filaConError
	for i := range out.Filas {
		if strings.EqualFold(out.Filas[i].Title, "Hellsing") {
			hellsing = &out.Filas[i]
		}
	}
	if hellsing == nil {
		t.Fatalf("Hellsing tenía que estar en filas_con_error: %+v", out.Filas)
	}
	if !strings.Contains(hellsing.Reason, "anterior a la de inicio") {
		t.Fatalf("el motivo de Hellsing no se entiende: %q", hellsing.Reason)
	}
	if hellsing.Line == 0 {
		t.Fatal("la fila de Hellsing no dice de qué línea salió")
	}

	if len(out.Relevos) == 0 {
		t.Fatal("no se propuso ningún relevo, y la hoja tiene el de Zoids sobre Magic Knight")
	}
	if len(out.Fechas) == 0 {
		t.Fatal("no se corrió ninguna fecha, y la hoja tiene reglas de madrugada")
	}

	// Las reglas se guardaron de verdad.
	reglas, err := c.a.Store.Rule.ListAll(context.Background(), c.a.ChannelID)
	if err != nil {
		t.Fatalf("no pude releer las reglas: %v", err)
	}
	if len(reglas) != 34 {
		t.Fatalf("en la base hay %d reglas, se esperaban 34", len(reglas))
	}

	// Y un relevo se confirma de un clic.
	rel := out.Relevos[0]
	w = c.do("POST", "/api/v1/importar/confirmar-relevos", []map[string]any{
		{"regla": rel.Relieves, "releva_a": rel.Expires},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("confirmar el relevo dio %d: %s", w.Code, w.Body.String())
	}
	confirmada, err := c.a.Store.Rule.Get(context.Background(), rel.Relieves)
	if err != nil {
		t.Fatalf("no encuentro la regla que releva: %v", err)
	}
	if confirmada.HandsOffTo == nil || *confirmada.HandsOffTo != rel.Expires {
		t.Fatalf("el relevo no quedó guardado: %+v", confirmada.HandsOffTo)
	}
}

func TestImportarHojaVacia(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	w := c.do("POST", "/api/v1/importar/hoja", map[string]string{"texto": "  "})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("una hoja vacía dio %d, se esperaba 400", w.Code)
	}
	var e errorBody
	c.json(w, &e)
	if !strings.Contains(e.Error, "pega la hoja") {
		t.Fatalf("el error no dice qué hacer: %q", e.Error)
	}
}

// ── cuarentena y bitácora ─────────────────────────────────────────────

func TestCuarentenaYDejarPasar(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	ctx := context.Background()

	malo := model.MediaAsset{
		Path:           "/medios/spot-roto.mp4",
		DurationMs:     30000,
		State:          model.AssetQuarantine,
		PlainReason:    "el audio va en mono y el canal sale en estéreo",
		NormalizeState: ingest.NormalizeFailed,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	if err := c.a.Store.Media.Insert(ctx, &malo); err != nil {
		t.Fatalf("no pude guardar el archivo: %v", err)
	}

	w := c.do("GET", "/api/v1/cuarentena", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("la cuarentena dio %d", w.Code)
	}
	var enCuarentena []model.MediaAsset
	c.json(w, &enCuarentena)
	if len(enCuarentena) != 1 || enCuarentena[0].PlainReason == "" {
		t.Fatalf("la cuarentena tiene que decir por qué: %+v", enCuarentena)
	}

	w = c.do("POST", "/api/v1/cuarentena/"+itoa(malo.ID)+"/dejar-pasar",
		map[string]string{"quien": "Rolando"})
	if w.Code != http.StatusOK {
		t.Fatalf("dejar pasar dio %d: %s", w.Code, w.Body.String())
	}
	var dejado model.MediaAsset
	c.json(w, &dejado)
	if dejado.State != model.AssetReady || dejado.LetThroughBy != "Rolando" {
		t.Fatalf("el archivo no quedó listo a nombre de nadie: %+v", dejado)
	}
	if !dejado.Ready() {
		t.Fatal("un archivo que se deja pasar tiene que poder salir al aire")
	}

	// Y queda en la bitácora, con nombre y apellido.
	w = c.do("GET", "/api/v1/auditoria?entidad=media_asset&id="+itoa(malo.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("la auditoría dio %d", w.Code)
	}
	var entradas []model.AuditEntry
	c.json(w, &entradas)
	encontrada := false
	for _, e := range entradas {
		if e.Field == "dejado_pasar_por" && e.After == "Rolando" && e.Origin == "humano" {
			encontrada = true
		}
	}
	if !encontrada {
		t.Fatalf("dejar pasar no dejó la entrada en la bitácora: %+v", entradas)
	}
}

func TestAuditoriaSeVerifica(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	title := c.conMaterial("Tarzan")
	today := hoy(c)

	if w := c.do("POST", "/api/v1/reglas", map[string]any{
		"title_id":       title.ID,
		"patron_de_dias": "LMMJV__",
		"hora":           7 * 60,
		"fecha_inicio":   string(today),
		"fecha_fin":      string(today.Add(10)),
		"activa":         true,
	}); w.Code != http.StatusCreated {
		t.Fatalf("crear la regla dio %d: %s", w.Code, w.Body.String())
	}

	w := c.do("GET", "/api/v1/auditoria/verificar", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("verificar dio %d", w.Code)
	}
	var out struct {
		Intacta bool   `json:"intacta"`
		Texto   string `json:"texto"`
	}
	c.json(w, &out)
	if !out.Intacta {
		t.Fatalf("la bitácora tendría que estar intacta: %s", out.Texto)
	}
}

func TestRellenoVacioAvisa(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	w := c.do("GET", "/api/v1/relleno", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("el relleno dio %d", w.Code)
	}
	var out struct {
		Items []model.FillerAsset `json:"items"`
		Aviso string              `json:"aviso"`
	}
	c.json(w, &out)
	if len(out.Items) != 0 || out.Aviso == "" {
		t.Fatalf("una biblioteca de relleno vacía tiene que avisar: %+v", out)
	}
}

// ── el asistente ──────────────────────────────────────────────────────

func TestAsistenteDePrincipioAFin(t *testing.T) {
	c := nuevo(t) // sin clave: el asistente es lo único abierto
	ctx := context.Background()

	w := c.do("GET", "/api/v1/instalacion", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("el asistente dio %d: %s", w.Code, w.Body.String())
	}

	// Paso 1: nombre, identificativo, comunidad y clave.
	w = c.do("POST", "/api/v1/instalacion/paso/1", map[string]any{
		"nombre":             "Caribbean Advantage TV",
		"identificativo":     "CAtv",
		"comunidad_licencia": "Aguadilla, PR",
		"clave":              clavePrueba,
		"nombre_operador":    "Rolando",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("el paso 1 dio %d: %s", w.Code, w.Body.String())
	}
	if c.cookie == nil {
		t.Fatal("el paso 1 tenía que dejar la sesión abierta")
	}
	if hay, _ := c.a.Store.Settings.HasPIN(ctx); !hay {
		t.Fatal("el paso 1 no guardó la clave de la estación")
	}
	ch, _ := c.a.Store.Channel.Get(ctx, c.a.ChannelID)
	if ch.Name != "Caribbean Advantage TV" || ch.CallSign != "CAtv" || ch.LicenseCity != "Aguadilla, PR" {
		t.Fatalf("el paso 1 no guardó el canal: %+v", ch)
	}

	// Una clave demasiado corta no pasa.
	if w := c.do("POST", "/api/v1/instalacion/paso/1", map[string]any{
		"nombre": "X", "clave": "12",
	}); w.Code != http.StatusBadRequest {
		t.Fatalf("una clave de dos dígitos dio %d, se esperaba 400", w.Code)
	}

	// Paso 2: qué vas a hacer. En F1 el canal se queda en sombra.
	w = c.do("POST", "/api/v1/instalacion/paso/2", map[string]string{"modo": "transmisor"})
	if w.Code != http.StatusOK {
		t.Fatalf("el paso 2 dio %d: %s", w.Code, w.Body.String())
	}
	ch, _ = c.a.Store.Channel.Get(ctx, c.a.ChannelID)
	if ch.Mode != "sombra" {
		t.Fatalf("en F1 el canal se queda en sombra, y está en %q", ch.Mode)
	}

	for _, paso := range []struct {
		n    int
		body any
	}{
		{3, nil},
		{4, map[string]string{"destino": "red", "retorno_de_aire": "receptor-tv", "nota": "el cable va al armario del pasillo"}},
		{5, map[string]bool{"ve_barras": true}},
		{6, map[string]string{"pais": "PR", "calidad": "720p59.94"}},
		{7, map[string]string{"carpeta": t.TempDir()}},
		{8, map[string]string{"propuesta": "automatica"}},
		{9, nil},
	} {
		w := c.do("POST", "/api/v1/instalacion/paso/"+itoa(int64(paso.n)), paso.body)
		if w.Code != http.StatusOK {
			t.Fatalf("el paso %d dio %d: %s", paso.n, w.Code, w.Body.String())
		}
	}

	ch, _ = c.a.Store.Channel.Get(ctx, c.a.ChannelID)
	if ch.RegProfile != "us-fcc" {
		t.Fatalf("el país PR tenía que encender el perfil us-fcc; está en %q", ch.RegProfile)
	}
	if ch.FormatProfile != "720p59.94" {
		t.Fatalf("la calidad no se guardó: %q", ch.FormatProfile)
	}
	if c.setting(app.KeyInstallDone) != "si" {
		t.Fatal("el paso 9 no marcó la instalación como completa")
	}
	if c.setting(app.KeyContentFolder) == "" {
		t.Fatal("el paso 7 no guardó la carpeta de contenido")
	}

	// Y ahora el estado ya no pide instalar.
	w = c.do("GET", "/api/v1/estado", nil)
	var estado map[string]any
	c.json(w, &estado)
	if estado["necesita_instalacion"] == true {
		t.Fatalf("después del asistente, /estado sigue pidiendo instalar: %v", estado)
	}
	if estado["instalacion_completa"] != true {
		t.Fatalf("/estado no dice que la instalación esté completa: %v", estado)
	}
}

// El paso 7 avisa cuando no hay relleno: un hueco sin relleno sale al cartel
// y eso no se descubre a las tres de la mañana (auditoría B12).
func TestPaso7AvisaSiNoHayRelleno(t *testing.T) {
	c := nuevo(t)
	if w := c.do("POST", "/api/v1/instalacion/paso/1", map[string]any{
		"nombre": "Canal de prueba", "clave": clavePrueba,
	}); w.Code != http.StatusOK {
		t.Fatalf("el paso 1 dio %d: %s", w.Code, w.Body.String())
	}
	w := c.do("POST", "/api/v1/instalacion/paso/7", map[string]string{"carpeta": t.TempDir()})
	if w.Code != http.StatusOK {
		t.Fatalf("el paso 7 dio %d: %s", w.Code, w.Body.String())
	}
	var out map[string]any
	c.json(w, &out)
	aviso, _ := out["aviso_relleno"].(string)
	if !strings.Contains(aviso, "relleno") {
		t.Fatalf("el paso 7 no avisó de la biblioteca de relleno vacía: %v", out)
	}
}

// ── ajustes ───────────────────────────────────────────────────────────

func TestAjustesNoEnseñanLaClave(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	w := c.do("GET", "/api/v1/ajustes", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("los ajustes dieron %d", w.Code)
	}
	var todos map[string]string
	c.json(w, &todos)
	for k := range todos {
		if strings.HasPrefix(k, "clave_estacion.") || strings.Contains(k, "secreto") {
			t.Fatalf("los ajustes enseñan %q, que es secreto", k)
		}
	}
	if w := c.do("PUT", "/api/v1/ajustes", map[string]string{
		"clave_estacion.hash": "trampa",
	}); w.Code != http.StatusBadRequest {
		t.Fatalf("cambiar la clave por Ajustes dio %d, se esperaba 400", w.Code)
	}
}

func TestSubirSinCarpetaLoDice(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	w := c.do("POST", "/api/v1/material/subir", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("subir sin carpeta dio %d, se esperaba 400", w.Code)
	}
	var e errorBody
	c.json(w, &e)
	if !strings.Contains(e.Error, "carpeta de contenido") {
		t.Fatalf("el error no dice qué falta: %q", e.Error)
	}
}

// ── ayudas ────────────────────────────────────────────────────────────

func (c *cliente) setting(key string) string {
	v, err := c.a.Store.Settings.Get(context.Background(), key)
	if err != nil {
		return ""
	}
	return v
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// ── el WebSocket ──────────────────────────────────────────────────────

// El apretón de manos es el del RFC 6455 y lo primero que llega es el
// estado. Se prueba contra un servidor de verdad porque hace falta poder
// soltar la conexión (Hijack), y httptest.NewRecorder no puede.
func TestWebSocketEmpujaElEstado(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	srv := httptest.NewServer(c.s)
	defer srv.Close()

	conn, err := net.Dial("tcp", strings.TrimPrefix(srv.URL, "http://"))
	if err != nil {
		t.Fatalf("no pude conectar: %v", err)
	}
	defer func() { _ = conn.Close() }()

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

	// Y el primer marco es el estado.
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
	payload := make([]byte, n)
	if _, err := io.ReadFull(br, payload); err != nil {
		t.Fatalf("el marco vino cortado: %v", err)
	}
	var msg map[string]any
	if err := json.Unmarshal(payload, &msg); err != nil {
		t.Fatalf("el marco no es JSON: %s", payload)
	}
	if msg["canal"] == nil {
		t.Fatalf("el empujón de estado tiene que traer el canal, como GET /estado: %v", msg)
	}
	if msg["tipo"] != "estado" || msg["modo"] != "sombra" {
		t.Fatalf("el primer marco no es el estado: %v", msg)
	}
}

// Sin clave, el WebSocket no se abre.
func TestWebSocketSinClaveEs401(t *testing.T) {
	c := nuevo(t).conClave()
	w := c.do("GET", "/api/v1/ws", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("el WebSocket sin cookie dio %d, se esperaba 401", w.Code)
	}
}
