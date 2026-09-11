package api

// Emparejar títulos (F1-64 a F1-67), de punta a punta contra la hoja real de
// CAtv: lo que el importador deja por emparejar, lo que la pantalla enseña, y
// lo que pasa cuando la persona decide.

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"

	"antena787/internal/app"
	"antena787/internal/model"
)

// ── el andamio ────────────────────────────────────────────────────────

// hojaImportada trae la hoja de CAtv ya importada y la respuesta del
// servidor.
type hojaImportada struct {
	ReglasCreadas  int                  `json:"reglas_creadas"`
	TitulosCreados int                  `json:"titulos_creados"`
	SinEmparejar   []tituloSinEmparejar `json:"titulos_sin_emparejar"`
	Avisos         []struct {
		Titulo string `json:"titulo"`
		Texto  string `json:"texto"`
	} `json:"avisos"`
	Resumen string `json:"resumen"`
}

func importarLaHoja(t *testing.T, c *cliente) hojaImportada {
	t.Helper()
	raw, err := os.ReadFile(hojaPath)
	if err != nil {
		t.Fatalf("no pude leer la hoja de CAtv: %v", err)
	}
	w := c.do("POST", "/api/v1/importar/hoja", map[string]string{"texto": string(raw)})
	if w.Code != http.StatusOK {
		t.Fatalf("importar la hoja dio %d: %s", w.Code, w.Body.String())
	}
	var out hojaImportada
	c.json(w, &out)
	return out
}

// porNombre indexa lo que quedó por emparejar.
func porNombre(lista []tituloSinEmparejar) map[string]tituloSinEmparejar {
	out := map[string]tituloSinEmparejar{}
	for _, t := range lista {
		out[t.Nombre] = t
	}
	return out
}

// fichaLlamada busca una ficha del catálogo por su nombre exacto.
func fichaLlamada(t *testing.T, c *cliente, nombre string) model.Title {
	t.Helper()
	ficha, err := c.a.Store.Title.FindByName(context.Background(), nombre)
	if err != nil {
		t.Fatalf("no encuentro la ficha «%s»: %v", nombre, err)
	}
	return ficha
}

// alarmaDe devuelve la alarma de ese tipo que enseña /estado, si la hay.
func alarmaDe(t *testing.T, c *cliente, tipo string) *app.Alarma {
	t.Helper()
	w := c.do("GET", "/api/v1/estado", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("/estado dio %d: %s", w.Code, w.Body.String())
	}
	var estado struct {
		Alarmas []app.Alarma `json:"alarmas"`
	}
	c.json(w, &estado)
	for i := range estado.Alarmas {
		if estado.Alarmas[i].Tipo == tipo {
			return &estado.Alarmas[i]
		}
	}
	return nil
}

// ── F1-64 y F1-65 · lo que la hoja deja por emparejar ─────────────────

// TestF1Verif64LaHojaDejaTitulosPorEmparejar: la hoja de CAtv trae tres
// nombres que no cuadran con ninguna ficha. No se crean callados: entran
// marcados, salen en la respuesta y encienden el aviso de Al aire.
func TestF1Verif64LaHojaDejaTitulosPorEmparejar(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	out := importarLaHoja(t, c)

	if len(out.SinEmparejar) != 3 {
		t.Fatalf("quedaron %d títulos por emparejar, se esperaban 3: %+v", len(out.SinEmparejar), out.SinEmparejar)
	}
	sin := porNombre(out.SinEmparejar)

	// «Samurai X»: su ficha se llama «Rurouni Kenshin» y no se parecen en
	// nada, así que no hay candidatos que ofrecer.
	samurai, ok := sin["Samurai X"]
	if !ok {
		t.Fatalf("«Samurai X» tenía que quedar por emparejar: %+v", out.SinEmparejar)
	}
	if len(samurai.Candidatos) != 0 {
		t.Fatalf("«Samurai X» no se parece a ninguna ficha y trae candidatos: %+v", samurai.Candidatos)
	}
	if samurai.ID == 0 {
		t.Fatal("«Samurai X» salió sin identificador: la pantalla no lo podría emparejar")
	}
	if samurai.Reglas < 2 {
		t.Fatalf("«Samurai X» sale en %d reglas y la hoja lo pone dos veces", samurai.Reglas)
	}
	if len(samurai.Franjas) != samurai.Reglas {
		t.Fatalf("«Samurai X» dice %d reglas y %d franjas", samurai.Reglas, len(samurai.Franjas))
	}
	if !strings.Contains(samurai.Franjas[0], " a las ") {
		t.Fatalf("la franja no se lee en palabras claras: %q", samurai.Franjas[0])
	}
	if !strings.Contains(samurai.Texto, "no tiene ficha en el catálogo") {
		t.Fatalf("el texto de «Samurai X» dice %q", samurai.Texto)
	}

	// «SaberMarionette»: se parece igual a dos fichas. No se adivina (F1-65).
	saber, ok := sin["SaberMarionette"]
	if !ok {
		t.Fatalf("«SaberMarionette» tenía que quedar por emparejar: %+v", out.SinEmparejar)
	}
	if len(saber.Candidatos) != 2 {
		t.Fatalf("«SaberMarionette» tenía que quedar en duda entre dos fichas: %+v", saber.Candidatos)
	}
	for _, cand := range saber.Candidatos {
		if cand.ID == 0 {
			t.Fatalf("un candidato de «SaberMarionette» vino sin identificador: %+v", cand)
		}
		if !strings.HasPrefix(cand.Nombre, "Saber Marionette") {
			t.Fatalf("un candidato de «SaberMarionette» es «%s»", cand.Nombre)
		}
		if cand.Puntuacion <= 0 || cand.Puntuacion > 1 {
			t.Fatalf("el parecido de «%s» es %v", cand.Nombre, cand.Puntuacion)
		}
	}
	if !strings.Contains(saber.Texto, "no quise adivinar") {
		t.Fatalf("el texto de «SaberMarionette» dice %q", saber.Texto)
	}

	// «Los Simuladores»: la ficha del catálogo está en inglés («Pretenders»),
	// así que tampoco tiene candidatos.
	simuladores, ok := sin["Los Simuladores"]
	if !ok {
		t.Fatalf("«Los Simuladores» tenía que quedar por emparejar: %+v", out.SinEmparejar)
	}
	if len(simuladores.Candidatos) != 0 {
		t.Fatalf("«Los Simuladores» trae candidatos: %+v", simuladores.Candidatos)
	}

	// F1-67 — el nombre de una fuente en vivo nunca entra al catálogo como
	// título, pero su regla sí sabe de qué fuente habla.
	ctx := context.Background()
	if _, err := c.a.Store.Title.FindByName(ctx, "RadioOnce Live!"); err == nil {
		t.Fatal("«RadioOnce Live!» es una fuente en vivo y entró al catálogo como título")
	}
	reglas, err := c.a.Store.Rule.ListAll(ctx, c.a.ChannelID)
	if err != nil {
		t.Fatalf("no pude releer las reglas: %v", err)
	}
	enVivo := 0
	for _, regla := range reglas {
		if regla.Kind != model.RuleLive {
			continue
		}
		enVivo++
		if regla.LiveSourceID == nil {
			t.Fatal("la regla en vivo se quedó sin fuente detrás")
		}
		if regla.TitleID != nil {
			t.Fatal("la regla en vivo apunta a un título, y una fuente en vivo no es un título")
		}
	}
	if enVivo == 0 {
		t.Fatal("la hoja trae «RadioOnce Live!» y no salió ninguna regla en vivo")
	}
	fuentes, err := c.a.Store.Live.List(ctx, c.a.ChannelID)
	if err != nil || len(fuentes) == 0 {
		t.Fatalf("no se creó la fuente en vivo: %v %+v", err, fuentes)
	}
	if fuentes[0].Name != "RadioOnce Live!" {
		t.Fatalf("la fuente en vivo se llama «%s»", fuentes[0].Name)
	}

	// Los títulos por emparejar no cuentan como fichas creadas.
	if out.TitulosCreados == 0 {
		t.Fatal("no se creó ningún título")
	}

	// La lista se puede volver a pedir mañana, y dice lo mismo.
	w := c.do("GET", "/api/v1/titulos/sin-emparejar", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /titulos/sin-emparejar dio %d: %s", w.Code, w.Body.String())
	}
	var guardados []tituloSinEmparejar
	c.json(w, &guardados)
	if len(guardados) != 3 {
		t.Fatalf("la base guarda %d títulos por emparejar: %+v", len(guardados), guardados)
	}
	deLaBase := porNombre(guardados)
	if deLaBase["SaberMarionette"].Reglas == 0 {
		t.Fatalf("«SaberMarionette» sale en 0 reglas según la base: %+v", deLaBase["SaberMarionette"])
	}
	if len(deLaBase["SaberMarionette"].Candidatos) != 2 {
		t.Fatalf("los candidatos no se guardaron: %+v", deLaBase["SaberMarionette"].Candidatos)
	}
	if p := deLaBase["SaberMarionette"].Candidatos[0].Puntuacion; p <= 0 {
		t.Fatalf("el parecido guardado es %v", p)
	}
	if len(deLaBase["Samurai X"].Franjas) == 0 {
		t.Fatalf("«Samurai X» se quedó sin franjas: %+v", deLaBase["Samurai X"])
	}

	// Y mientras quede alguno, Al aire lo dice (F1-64).
	aviso := alarmaDe(t, c, "emparejar")
	if aviso == nil {
		t.Fatal("Al aire no avisa de los títulos por emparejar")
	}
	if aviso.Texto != "3 títulos por emparejar" {
		t.Fatalf("el aviso dice %q", aviso.Texto)
	}
	if !strings.Contains(aviso.Detalle, "Samurai X") {
		t.Fatalf("el aviso no dice de cuáles habla: %q", aviso.Detalle)
	}
	if aviso.Accion == nil || aviso.Accion.Ruta != "/reglas" {
		t.Fatalf("el aviso no lleva a ninguna parte: %+v", aviso.Accion)
	}
}

// ── F1-66 · emparejar y recordar ──────────────────────────────────────

// TestF1Verif66EmparejarPasaLasReglasYSeAcuerda: la persona dice que
// «Samurai X» es «Rurouni Kenshin». Las reglas pasan a la ficha, el
// provisional desaparece y el nombre queda aprendido para la próxima hoja.
func TestF1Verif66EmparejarPasaLasReglasYSeAcuerda(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	out := importarLaHoja(t, c)
	samurai := porNombre(out.SinEmparejar)["Samurai X"]
	if samurai.ID == 0 {
		t.Fatalf("«Samurai X» no quedó por emparejar: %+v", out.SinEmparejar)
	}
	ctx := context.Background()
	kenshin := fichaLlamada(t, c, "Rurouni Kenshin")

	// La búsqueda es la que ofrece la ficha buena.
	w := c.do("GET", "/api/v1/titulos/buscar?q=kenshin", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /titulos/buscar dio %d: %s", w.Code, w.Body.String())
	}
	var encontrados []tituloDelCatalogo
	c.json(w, &encontrados)
	if len(encontrados) == 0 {
		t.Fatal("buscar «kenshin» no devolvió nada")
	}
	if encontrados[0].Nombre != "Rurouni Kenshin" {
		t.Fatalf("buscar «kenshin» devuelve «%s» primero", encontrados[0].Nombre)
	}
	if encontrados[0].ID != kenshin.ID || encontrados[0].Tipo == "" {
		t.Fatalf("la búsqueda no manda la ficha entera: %+v", encontrados[0])
	}
	for _, f := range encontrados {
		if f.Nombre == "Samurai X" {
			t.Fatal("la búsqueda ofrece un título que también está por emparejar")
		}
	}

	// Y se empareja de un clic.
	w = c.do("POST", "/api/v1/titulos/"+itoa(samurai.ID)+"/emparejar",
		map[string]any{"accion": "usar", "title_id": kenshin.ID})
	if w.Code != http.StatusOK {
		t.Fatalf("emparejar dio %d: %s", w.Code, w.Body.String())
	}
	var hecho struct {
		ReglasMovidas int    `json:"reglas_movidas"`
		Alias         string `json:"alias"`
		Texto         string `json:"texto"`
	}
	c.json(w, &hecho)
	if hecho.ReglasMovidas != samurai.Reglas {
		t.Fatalf("pasaron %d reglas y el título salía en %d", hecho.ReglasMovidas, samurai.Reglas)
	}
	if hecho.Alias != "Samurai X" {
		t.Fatalf("el alias guardado es %q", hecho.Alias)
	}
	for _, trozo := range []string{"«Samurai X» ahora es «Rurouni Kenshin»", "se empareja sola"} {
		if !strings.Contains(hecho.Texto, trozo) {
			t.Fatalf("el texto no dice %q: %q", trozo, hecho.Texto)
		}
	}

	// El provisional ya no está, y las reglas apuntan a la ficha.
	if _, err := c.a.Store.Title.Get(ctx, samurai.ID); err == nil {
		t.Fatal("el título provisional sigue en la base")
	}
	reglas, err := c.a.Store.Rule.ListAll(ctx, c.a.ChannelID)
	if err != nil {
		t.Fatalf("no pude releer las reglas: %v", err)
	}
	conKenshin := 0
	for _, regla := range reglas {
		if regla.TitleID != nil && *regla.TitleID == kenshin.ID {
			conKenshin++
		}
	}
	if conKenshin != hecho.ReglasMovidas {
		t.Fatalf("%d reglas apuntan a «Rurouni Kenshin» y se movieron %d", conKenshin, hecho.ReglasMovidas)
	}

	// El nombre de la hoja quedó aprendido.
	id, ok, err := c.a.Store.Alias.Resolve(ctx, c.a.ChannelID, "SAMURAI-X")
	if err != nil || !ok || id != kenshin.ID {
		t.Fatalf("el alias no se guardó: %d %v %v", id, ok, err)
	}

	// Y el aviso baja a dos.
	aviso := alarmaDe(t, c, "emparejar")
	if aviso == nil || aviso.Texto != "2 títulos por emparejar" {
		t.Fatalf("el aviso quedó en %+v", aviso)
	}

	// La misma hoja otra vez: «Samurai X» ya no se pregunta, y se dice por qué.
	otra := importarLaHoja(t, c)
	for _, sin := range otra.SinEmparejar {
		if sin.Nombre == "Samurai X" || sin.Nombre == "SamuraiX" {
			t.Fatalf("«%s» se volvió a preguntar después de emparejarlo", sin.Nombre)
		}
	}
	recordado := ""
	for _, aviso := range otra.Avisos {
		if aviso.Titulo == "Samurai X" {
			recordado = aviso.Texto
		}
	}
	if !strings.Contains(recordado, "lo recordaba de otra hoja") {
		t.Fatalf("la segunda hoja no dice que se acordaba: %q", recordado)
	}
}

// ── F1-67 · es propio, o no es un programa ────────────────────────────

// TestF1Verif67EsUnTituloPropio: «Los Simuladores» es un programa de verdad
// aunque el catálogo lo llame de otra manera. Se queda como ficha propia y
// sus reglas no se tocan.
func TestF1Verif67EsUnTituloPropio(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	out := importarLaHoja(t, c)
	simuladores := porNombre(out.SinEmparejar)["Los Simuladores"]
	if simuladores.ID == 0 {
		t.Fatalf("«Los Simuladores» no quedó por emparejar: %+v", out.SinEmparejar)
	}
	ctx := context.Background()
	antes, err := c.a.Store.Rule.ListAll(ctx, c.a.ChannelID)
	if err != nil {
		t.Fatalf("no pude leer las reglas: %v", err)
	}

	w := c.do("POST", "/api/v1/titulos/"+itoa(simuladores.ID)+"/emparejar",
		map[string]any{"accion": "propio"})
	if w.Code != http.StatusOK {
		t.Fatalf("decir que es propio dio %d: %s", w.Code, w.Body.String())
	}
	var hecho struct {
		Texto string `json:"texto"`
	}
	c.json(w, &hecho)
	if !strings.Contains(hecho.Texto, "«Los Simuladores»") {
		t.Fatalf("el texto no dice de qué habla: %q", hecho.Texto)
	}

	ficha, err := c.a.Store.Title.Get(ctx, simuladores.ID)
	if err != nil {
		t.Fatalf("la ficha propia desapareció: %v", err)
	}
	if ficha.PendienteEmparejar {
		t.Fatal("«Los Simuladores» sigue por emparejar después de decir que es propio")
	}
	despues, err := c.a.Store.Rule.ListAll(ctx, c.a.ChannelID)
	if err != nil {
		t.Fatalf("no pude releer las reglas: %v", err)
	}
	if len(antes) != len(despues) {
		t.Fatalf("decir que es propio se llevó reglas: %d → %d", len(antes), len(despues))
	}

	// Ya no sale en la lista, y la búsqueda sí lo ofrece.
	w = c.do("GET", "/api/v1/titulos/sin-emparejar", nil)
	var quedan []tituloSinEmparejar
	c.json(w, &quedan)
	if _, sigue := porNombre(quedan)["Los Simuladores"]; sigue {
		t.Fatalf("«Los Simuladores» sigue en la lista: %+v", quedan)
	}
	w = c.do("GET", "/api/v1/titulos/buscar?q=simuladores", nil)
	var encontrados []tituloDelCatalogo
	c.json(w, &encontrados)
	if len(encontrados) == 0 || encontrados[0].Nombre != "Los Simuladores" {
		t.Fatalf("la búsqueda no ofrece la ficha propia: %+v", encontrados)
	}
}

// TestF1Verif67NoEsUnPrograma: lo que no es un programa se quita, y se dice
// cuántas reglas se van con él.
func TestF1Verif67NoEsUnPrograma(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	out := importarLaHoja(t, c)
	saber := porNombre(out.SinEmparejar)["SaberMarionette"]
	if saber.ID == 0 {
		t.Fatalf("«SaberMarionette» no quedó por emparejar: %+v", out.SinEmparejar)
	}
	ctx := context.Background()
	antes, err := c.a.Store.Rule.ListAll(ctx, c.a.ChannelID)
	if err != nil {
		t.Fatalf("no pude leer las reglas: %v", err)
	}

	w := c.do("POST", "/api/v1/titulos/"+itoa(saber.ID)+"/emparejar", map[string]any{"accion": "quitar"})
	if w.Code != http.StatusOK {
		t.Fatalf("quitar dio %d: %s", w.Code, w.Body.String())
	}
	var hecho struct {
		ReglasQuitadas int    `json:"reglas_quitadas"`
		Texto          string `json:"texto"`
	}
	c.json(w, &hecho)
	if hecho.ReglasQuitadas != saber.Reglas {
		t.Fatalf("se quitaron %d reglas y el título salía en %d", hecho.ReglasQuitadas, saber.Reglas)
	}
	if !strings.Contains(hecho.Texto, "«SaberMarionette»") {
		t.Fatalf("el texto no dice qué se quitó: %q", hecho.Texto)
	}

	if _, err := c.a.Store.Title.Get(ctx, saber.ID); err == nil {
		t.Fatal("el título quitado sigue en la base")
	}
	despues, err := c.a.Store.Rule.ListAll(ctx, c.a.ChannelID)
	if err != nil {
		t.Fatalf("no pude releer las reglas: %v", err)
	}
	if len(antes)-len(despues) != hecho.ReglasQuitadas {
		t.Fatalf("se dijeron %d reglas quitadas y se fueron %d", hecho.ReglasQuitadas, len(antes)-len(despues))
	}

	// Y cuando no queda ninguno, el aviso se apaga.
	for _, sin := range porNombre(out.SinEmparejar) {
		if sin.ID == saber.ID {
			continue
		}
		if w := c.do("POST", "/api/v1/titulos/"+itoa(sin.ID)+"/emparejar",
			map[string]any{"accion": "propio"}); w.Code != http.StatusOK {
			t.Fatalf("decir que «%s» es propio dio %d: %s", sin.Nombre, w.Code, w.Body.String())
		}
	}
	if aviso := alarmaDe(t, c, "emparejar"); aviso != nil {
		t.Fatalf("no queda nada por emparejar y Al aire sigue avisando: %+v", aviso)
	}
}

// ── los peros ─────────────────────────────────────────────────────────

// TestEmparejarLoQueNoSePuede: cada pero se dice en palabras claras y con su
// código.
func TestEmparejarLoQueNoSePuede(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	out := importarLaHoja(t, c)
	sin := porNombre(out.SinEmparejar)
	samurai := sin["Samurai X"]
	saber := sin["SaberMarionette"]
	kenshin := fichaLlamada(t, c, "Rurouni Kenshin")

	casos := []struct {
		nombre  string
		ruta    string
		cuerpo  map[string]any
		codigo  int
		campo   string
		trozo   string
		llevaID bool
	}{
		{
			nombre: "un título que no existe",
			ruta:   "/api/v1/titulos/999999/emparejar",
			cuerpo: map[string]any{"accion": "usar", "title_id": kenshin.ID},
			codigo: http.StatusNotFound,
		},
		{
			nombre: "sin decir con qué ficha",
			ruta:   "/api/v1/titulos/" + itoa(samurai.ID) + "/emparejar",
			cuerpo: map[string]any{"accion": "usar"},
			codigo: http.StatusBadRequest,
			campo:  "title_id",
		},
		{
			nombre: "una acción que no existe",
			ruta:   "/api/v1/titulos/" + itoa(samurai.ID) + "/emparejar",
			cuerpo: map[string]any{"accion": "borrar todo"},
			codigo: http.StatusBadRequest,
			campo:  "accion",
			trozo:  "usar, propio o quitar",
		},
		{
			nombre: "consigo mismo",
			ruta:   "/api/v1/titulos/" + itoa(samurai.ID) + "/emparejar",
			cuerpo: map[string]any{"accion": "usar", "title_id": samurai.ID},
			codigo: http.StatusBadRequest,
			campo:  "title_id",
			trozo:  "no hay nada que emparejar",
		},
		{
			nombre: "contra una ficha que también está por emparejar",
			ruta:   "/api/v1/titulos/" + itoa(samurai.ID) + "/emparejar",
			cuerpo: map[string]any{"accion": "usar", "title_id": saber.ID},
			codigo: http.StatusConflict,
			campo:  "title_id",
			trozo:  "resuélvela primero",
		},
		{
			nombre: "quitar una ficha del catálogo",
			ruta:   "/api/v1/titulos/" + itoa(kenshin.ID) + "/emparejar",
			cuerpo: map[string]any{"accion": "quitar"},
			codigo: http.StatusConflict,
			trozo:  "no está por emparejar",
		},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			w := c.do("POST", caso.ruta, caso.cuerpo)
			if w.Code != caso.codigo {
				t.Fatalf("dio %d, se esperaba %d: %s", w.Code, caso.codigo, w.Body.String())
			}
			var e errorBody
			c.json(w, &e)
			if e.Error == "" {
				t.Fatal("el error llegó sin explicación")
			}
			if caso.campo != "" && e.Field != caso.campo {
				t.Fatalf("el error señala el campo %q y se esperaba %q", e.Field, caso.campo)
			}
			if caso.trozo != "" && !strings.Contains(e.Error, caso.trozo) {
				t.Fatalf("el error dice %q y tenía que decir %q", e.Error, caso.trozo)
			}
		})
	}

	// Sin clave no se toca nada.
	sinClave := nuevo(t).conClave()
	for _, ruta := range []string{"/api/v1/titulos/sin-emparejar", "/api/v1/titulos/buscar?q=a"} {
		if w := sinClave.do("GET", ruta, nil); w.Code != http.StatusUnauthorized {
			t.Fatalf("%s sin clave dio %d, se esperaba 401", ruta, w.Code)
		}
	}
	if w := sinClave.do("POST", "/api/v1/titulos/1/emparejar",
		map[string]any{"accion": "propio"}); w.Code != http.StatusUnauthorized {
		t.Fatalf("emparejar sin clave dio %d, se esperaba 401", w.Code)
	}
}

// TestSinTitulosPorEmparejarLaListaEsVacia: sin nada importado, las tres
// rutas contestan listas vacías, nunca null.
func TestSinTitulosPorEmparejarLaListaEsVacia(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	for _, ruta := range []string{"/api/v1/titulos/sin-emparejar", "/api/v1/titulos/buscar?q=nada-de-nada"} {
		w := c.do("GET", ruta, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("%s dio %d: %s", ruta, w.Code, w.Body.String())
		}
		if strings.TrimSpace(w.Body.String()) != "[]" {
			t.Fatalf("%s contestó %q y se esperaba una lista vacía", ruta, w.Body.String())
		}
	}
	if aviso := alarmaDe(t, c, "emparejar"); aviso != nil {
		t.Fatalf("sin nada importado ya hay aviso: %+v", aviso)
	}
}
