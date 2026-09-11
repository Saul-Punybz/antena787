package api

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"antena787/internal/app"
	"antena787/internal/ingest"
	"antena787/internal/model"
)

// ── GET /instalacion ──────────────────────────────────────────────────

// TestInstalacionTraeTodoLoQuePintaLaPantalla comprueba el contrato del
// asistente: lo detectado (con disco y red, que docs/API.md prometía y no
// llegaban), el catálogo de respuestas posibles, lo ya contestado y cuándo se
// contestó cada paso (F2-108).
func TestInstalacionTraeTodoLoQuePintaLaPantalla(t *testing.T) {
	c := nuevo(t) // sin clave: el asistente es lo único abierto

	w := c.do("GET", "/api/v1/instalacion", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("el asistente dio %d: %s", w.Code, w.Body.String())
	}
	var cuerpo struct {
		Paso       int                                 `json:"paso"`
		Pasos      int                                 `json:"pasos"`
		Detectado  map[string]any                      `json:"detectado"`
		Opciones   map[string][]app.OpcionDelAsistente `json:"opciones"`
		Respuestas map[string]map[string]any           `json:"respuestas"`
		Tiempos    map[string]string                   `json:"tiempos"`
	}
	c.json(w, &cuerpo)

	if cuerpo.Pasos != PasoFinal || cuerpo.Paso != 1 {
		t.Fatalf("el asistente empieza en el paso 1 de %d y dice %d de %d", PasoFinal, cuerpo.Paso, cuerpo.Pasos)
	}
	for _, campo := range []string{"ffmpeg", "carpeta_datos", "aceleracion", "disco", "red"} {
		if _, hay := cuerpo.Detectado[campo]; !hay {
			t.Fatalf("lo detectado no trae %q: %v", campo, cuerpo.Detectado)
		}
	}
	disco, _ := cuerpo.Detectado["disco"].(string)
	if disco == "" {
		t.Fatal("el disco llegó vacío: o se mide, o se dice que no se pudo")
	}
	red, _ := cuerpo.Detectado["red"].(string)
	if !strings.Contains(red, "conectado") && !strings.Contains(red, "sin red") {
		t.Fatalf("la red no se dice en palabras claras: %q", red)
	}

	for _, lista := range []string{"modo", "destino", "retorno", "calidad"} {
		if len(cuerpo.Opciones[lista]) == 0 {
			t.Fatalf("no llegaron las opciones de %q: %v", lista, cuerpo.Opciones)
		}
		for _, o := range cuerpo.Opciones[lista] {
			if o.Valor == "" || o.Texto == "" {
				t.Fatalf("una opción de %q viene a medias: %+v", lista, o)
			}
		}
	}
	if len(cuerpo.Respuestas) != 0 {
		t.Fatalf("recién empezado no hay nada contestado y llegó: %v", cuerpo.Respuestas)
	}
	if len(cuerpo.Tiempos) != 0 {
		t.Fatalf("recién empezado no hay ningún paso hecho y llegó: %v", cuerpo.Tiempos)
	}

	// Se contestan tres pasos y los tres tienen que verse de vuelta.
	if w := c.do("POST", "/api/v1/instalacion/paso/1", map[string]any{
		"nombre": "Caribbean Advantage TV", "identificativo": "CAtv",
		"comunidad_licencia": "Aguadilla, PR", "clave": clavePrueba, "nombre_operador": "Rolando",
	}); w.Code != http.StatusOK {
		t.Fatalf("el paso 1 dio %d: %s", w.Code, w.Body.String())
	}
	if w := c.do("POST", "/api/v1/instalacion/paso/4", map[string]string{
		"destino": "red", "retorno_de_aire": "ninguno", "nota": "el cable va al armario del pasillo",
	}); w.Code != http.StatusOK {
		t.Fatalf("el paso 4 dio %d: %s", w.Code, w.Body.String())
	}
	if w := c.do("POST", "/api/v1/instalacion/paso/5", map[string]bool{"ve_barras": false}); w.Code != http.StatusOK {
		t.Fatalf("el paso 5 dio %d: %s", w.Code, w.Body.String())
	}

	w = c.do("GET", "/api/v1/instalacion", nil)
	c.json(w, &cuerpo)

	uno, hay := cuerpo.Respuestas["1"]
	if !hay {
		t.Fatalf("el paso 1 está contestado y no vuelve: %v", cuerpo.Respuestas)
	}
	if uno["nombre"] != "Caribbean Advantage TV" || uno["identificativo"] != "CAtv" ||
		uno["comunidad_licencia"] != "Aguadilla, PR" || uno["nombre_operador"] != "Rolando" {
		t.Fatalf("el paso 1 no vuelve como se contestó: %v", uno)
	}
	cuatro := cuerpo.Respuestas["4"]
	if cuatro["destino"] != "red" || cuatro["retorno_de_aire"] != "ninguno" ||
		cuatro["nota"] != "el cable va al armario del pasillo" {
		t.Fatalf("el paso 4 no vuelve como se contestó: %v", cuatro)
	}
	if cinco := cuerpo.Respuestas["5"]; cinco["ve_barras"] != false {
		t.Fatalf("el paso 5 no vuelve como se contestó: %v", cinco)
	}
	if _, hay := cuerpo.Respuestas["6"]; hay {
		t.Fatalf("el paso 6 no está contestado y sale igual: %v", cuerpo.Respuestas["6"])
	}

	// La clave de estación no sale por aquí, ni por asomo.
	if strings.Contains(w.Body.String(), clavePrueba) {
		t.Fatal("la clave de la estación se está devolviendo en GET /instalacion")
	}

	for _, n := range []string{"1", "4", "5"} {
		cuando, hay := cuerpo.Tiempos[n]
		if !hay {
			t.Fatalf("no quedó apuntado cuándo se contestó el paso %s: %v", n, cuerpo.Tiempos)
		}
		if _, err := time.Parse(time.RFC3339, cuando); err != nil {
			t.Fatalf("el instante del paso %s no es una fecha: %q", n, cuando)
		}
	}
	if _, hay := cuerpo.Tiempos["9"]; hay {
		t.Fatal("el paso 9 no se ha hecho y ya tiene hora")
	}
}

// ── paso 4 ────────────────────────────────────────────────────────────

func TestPaso4SoloAceptaLoQueElAsistenteOfrecio(t *testing.T) {
	c := conAsistenteEmpezado(t)

	// Lo bueno pasa, y «todavía no» es una respuesta como cualquier otra.
	w := c.do("POST", "/api/v1/instalacion/paso/4", map[string]string{
		"destino": "ninguna", "retorno_de_aire": "ninguno",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("«todavía no» tiene que ser una respuesta válida y dio %d: %s", w.Code, w.Body.String())
	}
	var out map[string]any
	c.json(w, &out)
	aviso, _ := out["aviso"].(string)
	if !strings.Contains(aviso, "retorno de aire") {
		t.Fatalf("sin retorno de aire hay que decirlo, y el aviso fue %q", aviso)
	}
	if out["siguiente"] != float64(5) {
		t.Fatalf("el paso 4 tiene que llevar al 5: %v", out)
	}

	// Y lo que no está en la lista no pasa, con el campo señalado.
	for campo, cuerpo := range map[string]map[string]string{
		"destino":         {"destino": "udp://239.0.0.1:1234", "retorno_de_aire": "ninguno"},
		"retorno_de_aire": {"destino": "red", "retorno_de_aire": "una tarjeta que compré"},
	} {
		w := c.do("POST", "/api/v1/instalacion/paso/4", cuerpo)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s con un valor de fuera dio %d, se esperaba 400", campo, w.Code)
		}
		var e errorBody
		c.json(w, &e)
		if e.Field != campo {
			t.Fatalf("el error tenía que señalar %q y señala %q", campo, e.Field)
		}
		if e.Error == "" {
			t.Fatal("el error llegó sin explicación")
		}
	}
}

// ── paso 6 ────────────────────────────────────────────────────────────

func TestPaso6ValidaLaCalidadContraLaLista(t *testing.T) {
	c := conAsistenteEmpezado(t)

	for _, calidad := range []string{"720p59.94", "1080i59.94", "480i59.94", "1080p25"} {
		w := c.do("POST", "/api/v1/instalacion/paso/6", map[string]string{"pais": "PR", "calidad": calidad})
		if w.Code != http.StatusOK {
			t.Fatalf("la calidad %q dio %d: %s", calidad, w.Code, w.Body.String())
		}
		ch, _ := c.a.Store.Channel.Get(context.Background(), c.a.ChannelID)
		if ch.FormatProfile != calidad {
			t.Fatalf("la calidad %q no se guardó: %q", calidad, ch.FormatProfile)
		}
	}

	w := c.do("POST", "/api/v1/instalacion/paso/6", map[string]string{"pais": "PR", "calidad": "4K"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("una calidad que no existe dio %d, se esperaba 400: %s", w.Code, w.Body.String())
	}
	var e errorBody
	c.json(w, &e)
	if e.Field != "calidad" {
		t.Fatalf("el error tenía que señalar la calidad y señala %q", e.Field)
	}
	if !strings.Contains(e.Error, "720p") {
		t.Fatalf("el error tiene que decir qué se aceptaba y dice %q", e.Error)
	}
	// Y la calidad buena de antes sigue puesta: un intento malo no rompe nada.
	ch, _ := c.a.Store.Channel.Get(context.Background(), c.a.ChannelID)
	if ch.FormatProfile != "1080p25" {
		t.Fatalf("el intento malo cambió el formato de casa: %q", ch.FormatProfile)
	}
}

// ── paso 8 ────────────────────────────────────────────────────────────

func TestPaso8ProponeLaPrimeraParrilla(t *testing.T) {
	c := conAsistenteEmpezado(t)
	serieDePrueba(c, "Kojak", 2)
	serieDePrueba(c, "Zoids", 2)
	c.conMaterial("La Batalla")

	w := c.do("POST", "/api/v1/instalacion/paso/8", map[string]string{"propuesta": "automatica"})
	if w.Code != http.StatusOK {
		t.Fatalf("el paso 8 dio %d: %s", w.Code, w.Body.String())
	}
	var out struct {
		Reglas      int    `json:"reglas_creadas"`
		Bloques     int    `json:"bloques"`
		SinMaterial int    `json:"titulos_sin_material"`
		Aviso       string `json:"aviso"`
		Siguiente   int    `json:"siguiente"`
		Avisos      []any  `json:"avisos"`
	}
	c.json(w, &out)
	if out.Reglas != 3 {
		t.Fatalf("con dos series y una película se esperaban tres espacios y salieron %d", out.Reglas)
	}
	if out.Bloques == 0 {
		t.Fatal("la propuesta no dejó ni un bloque en el plan")
	}
	if out.Siguiente != 9 {
		t.Fatalf("el paso 8 tiene que llevar al 9 y lleva al %d", out.Siguiente)
	}
	if out.Aviso == "" {
		t.Fatal("el paso 8 tiene que decir qué hacer ahora")
	}
	if !strings.Contains(out.Aviso, "Reglas") {
		t.Fatalf("el aviso tiene que decir dónde se cambia la propuesta: %q", out.Aviso)
	}

	reglas, _ := c.a.Store.Rule.ListAll(context.Background(), c.a.ChannelID)
	if len(reglas) != 3 {
		t.Fatalf("quedaron %d reglas guardadas", len(reglas))
	}
}

func TestPaso8NoTocaLaParrillaQueYaEstaba(t *testing.T) {
	c := conAsistenteEmpezado(t)
	titulo := c.conMaterial("La Batalla")
	ctx := context.Background()
	hoy := hoy(c)
	mia := model.ScheduleRule{
		ChannelID: c.a.ChannelID, Kind: model.RuleNormal, TitleID: &titulo.ID,
		Days: "LMMJV__", At: 20 * 60, SlotMs: 2 * 60 * 60 * 1000,
		From: hoy, To: hoy.Add(10), EpisodesPerRun: 1, Active: true,
	}
	if err := c.a.Store.Rule.Insert(ctx, &mia); err != nil {
		t.Fatalf("no pude guardar la regla: %v", err)
	}

	w := c.do("POST", "/api/v1/instalacion/paso/8", map[string]string{"propuesta": "automatica"})
	if w.Code != http.StatusOK {
		t.Fatalf("el paso 8 dio %d: %s", w.Code, w.Body.String())
	}
	var out struct {
		Reglas int    `json:"reglas_creadas"`
		Aviso  string `json:"aviso"`
	}
	c.json(w, &out)
	if out.Reglas != 0 {
		t.Fatalf("ya había parrilla y el asistente creó %d reglas encima", out.Reglas)
	}
	if !strings.Contains(out.Aviso, "no toqué nada") {
		t.Fatalf("cuando ya hay parrilla hay que decir que no se tocó: %q", out.Aviso)
	}
	reglas, _ := c.a.Store.Rule.ListAll(ctx, c.a.ChannelID)
	if len(reglas) != 1 {
		t.Fatalf("quedaron %d reglas y solo había una", len(reglas))
	}

	// Y «dejarlo para después» sigue valiendo: no crea nada.
	if w := c.do("POST", "/api/v1/instalacion/paso/8", map[string]string{"propuesta": "ninguna"}); w.Code != http.StatusOK {
		t.Fatalf("dejarlo para después dio %d: %s", w.Code, w.Body.String())
	}
	// Un valor que no existe sí se rechaza.
	w = c.do("POST", "/api/v1/instalacion/paso/8", map[string]string{"propuesta": "sorpréndeme"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("una respuesta que no existe dio %d, se esperaba 400", w.Code)
	}
	var e errorBody
	c.json(w, &e)
	if e.Field != "propuesta" {
		t.Fatalf("el error tenía que señalar la propuesta y señala %q", e.Field)
	}
}

// ── el relleno por defecto (F2-106) ───────────────────────────────────

func TestRellenoPorDefectoDeUnClic(t *testing.T) {
	c := conAsistenteEmpezado(t)
	if c.a.FFmpeg == "" || c.a.FFprobe == "" {
		t.Skip("hacen falta las herramientas de video para esta prueba")
	}
	ctx := context.Background()

	// Sin carpeta de contenido todavía no hay dónde ponerlo, y se dice.
	w := c.do("POST", "/api/v1/instalacion/relleno-por-defecto", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("sin carpeta de contenido dio %d, se esperaba 400: %s", w.Code, w.Body.String())
	}
	var e errorBody
	c.json(w, &e)
	if e.Field != "carpeta" {
		t.Fatalf("el error tenía que señalar la carpeta y señala %q", e.Field)
	}

	// La definición más chica, para que la prueba no tarde.
	ch, _ := c.a.Store.Channel.Get(ctx, c.a.ChannelID)
	ch.FormatProfile = "480i59.94"
	if err := c.a.Store.Channel.Update(ctx, ch); err != nil {
		t.Fatalf("no pude guardar el canal: %v", err)
	}
	if w := c.do("POST", "/api/v1/instalacion/paso/7", map[string]string{"carpeta": t.TempDir()}); w.Code != http.StatusOK {
		t.Fatalf("el paso 7 dio %d: %s", w.Code, w.Body.String())
	}

	eventos, soltar := c.a.Subscribe()
	defer soltar()

	w = c.do("POST", "/api/v1/instalacion/relleno-por-defecto", nil)
	if w.Code != http.StatusAccepted {
		t.Fatalf("el relleno por defecto dio %d, se esperaba 202: %s", w.Code, w.Body.String())
	}
	var out struct {
		Archivo string `json:"archivo"`
		Aviso   string `json:"aviso"`
	}
	c.json(w, &out)
	if out.Archivo == "" || !strings.HasSuffix(out.Archivo, app.ArchivoDelCartel) {
		t.Fatalf("el relleno no dice a dónde va: %q", out.Archivo)
	}
	if !strings.Contains(out.Aviso, "Biblioteca") {
		t.Fatalf("el aviso no dice dónde va a aparecer: %q", out.Aviso)
	}

	// El segundo clic no lo hace dos veces.
	w2 := c.do("POST", "/api/v1/instalacion/relleno-por-defecto", nil)
	if w2.Code != http.StatusConflict {
		t.Fatalf("el segundo clic dio %d, se esperaba 409: %s", w2.Code, w2.Body.String())
	}

	// Y cuando termina, avisa por el bus y está en la biblioteca de relleno.
	esperarEvento(t, eventos, "relleno", 2*time.Minute)
	rellenos, err := c.a.Store.Filler.List(ctx, c.a.ChannelID)
	if err != nil {
		t.Fatalf("no pude leer el relleno: %v", err)
	}
	if len(rellenos) != 1 {
		t.Fatalf("el cartel no entró como relleno: %+v", rellenos)
	}
	if rellenos[0].DurationMs < 58_000 || rellenos[0].DurationMs > 62_000 {
		t.Fatalf("el cartel dice durar %d ms y se esperaba un minuto", rellenos[0].DurationMs)
	}
}

// ── andamio ───────────────────────────────────────────────────────────

// conAsistenteEmpezado deja el paso 1 contestado, que es lo que abre la
// sesión y deja seguir con el resto.
func conAsistenteEmpezado(t *testing.T) *cliente {
	t.Helper()
	c := nuevo(t)
	w := c.do("POST", "/api/v1/instalacion/paso/1", map[string]any{
		"nombre": "Caribbean Advantage TV", "identificativo": "CAtv",
		"comunidad_licencia": "Aguadilla, PR", "clave": clavePrueba,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("el paso 1 dio %d: %s", w.Code, w.Body.String())
	}
	return c
}

// serieDePrueba deja una serie con episodios listos para aire.
func serieDePrueba(c *cliente, nombre string, episodios int) model.Title {
	c.t.Helper()
	ctx := context.Background()
	titulo := model.Title{Name: nombre, Kind: model.TitleSeries}
	if err := c.a.Store.Title.Insert(ctx, &titulo); err != nil {
		c.t.Fatalf("no pude guardar la serie %q: %v", nombre, err)
	}
	for i := 1; i <= episodios; i++ {
		ahora := time.Now().UTC()
		ruta := "/medios/" + nombre + itoa(int64(i))
		asset := model.MediaAsset{
			Path:           ruta + ".mkv",
			DurationMs:     22 * 60 * 1000,
			State:          model.AssetReady,
			NormalizeState: ingest.NormalizeReady,
			NormalizedPath: ruta + ".norm.mkv",
			CreatedAt:      ahora,
			UpdatedAt:      ahora,
		}
		if err := c.a.Store.Media.Insert(ctx, &asset); err != nil {
			c.t.Fatalf("no pude guardar el episodio %d: %v", i, err)
		}
		e := model.Episode{TitleID: titulo.ID, Season: 1, Number: i, Name: nombre, MediaAssetID: &asset.ID}
		if err := c.a.Store.Episode.Insert(ctx, &e); err != nil {
			c.t.Fatalf("no pude guardar el episodio %d: %v", i, err)
		}
	}
	return titulo
}

func esperarEvento(t *testing.T, eventos <-chan app.Event, clase string, espera time.Duration) {
	t.Helper()
	limite := time.After(espera)
	for {
		select {
		case e, abierto := <-eventos:
			if !abierto {
				t.Fatalf("el bus se cerró sin avisar de %q", clase)
			}
			if e.Kind == clase {
				return
			}
		case <-limite:
			t.Fatalf("pasaron %s y nunca llegó el aviso de %q", espera, clase)
		}
	}
}
