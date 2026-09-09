package app

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"antena787/internal/ingest"
	"antena787/internal/model"
)

// conPrograma deja un título con archivo listo y una regla que lo pone todos
// los días a la hora dada. Devuelve el título creado.
func conPrograma(t *testing.T, a *App, nombre string, hora string) model.Title {
	t.Helper()
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
	if err := a.Store.Media.Insert(ctx, &asset); err != nil {
		t.Fatalf("no pude guardar el archivo: %v", err)
	}
	title := model.Title{Name: nombre, Kind: model.TitleProgram, MediaAssetID: &asset.ID}
	if err := a.Store.Title.Insert(ctx, &title); err != nil {
		t.Fatalf("no pude guardar el título: %v", err)
	}

	ch, err := a.Store.Channel.Get(ctx, a.ChannelID)
	if err != nil {
		t.Fatalf("no pude leer el canal: %v", err)
	}
	hoy := ch.BroadcastDay(a.Now())
	at, err := model.ParseClock(hora)
	if err != nil {
		t.Fatalf("hora mal escrita: %v", err)
	}
	id := title.ID
	rule := model.ScheduleRule{
		ChannelID: a.ChannelID, Kind: model.RuleNormal, TitleID: &id,
		Days: model.DayPattern("LMMJVSD"), At: at, SlotMs: 30 * 60 * 1000,
		From: hoy, To: hoy.Add(120), EpisodesPerRun: 1, Active: true,
	}
	if err := a.Store.Rule.Insert(ctx, &rule); err != nil {
		t.Fatalf("no pude guardar la regla: %v", err)
	}
	return title
}

// F1-47 — El resolver reescribe la guía XMLTV en la misma corrida en que
// materializa el plan: no hay proceso aparte ni temporizador propio.
func TestF1Verif47LaGuiaSaleEnLaMismaCorrida(t *testing.T) {
	a := abre(t)
	ctx := context.Background()

	if g, _ := a.Guide(); len(g) != 0 {
		t.Fatal("antes de resolver no debería haber guía")
	}

	antes := time.Now()
	if _, err := a.Resolve(ctx); err != nil {
		t.Fatalf("la corrida falló: %v", err)
	}
	guia, generada := a.Guide()
	if len(guia) == 0 {
		t.Fatal("Resolve terminó y no dejó guía: la guía no sale de la misma corrida")
	}
	if generada.IsZero() {
		t.Fatal("la guía no dice cuándo se generó")
	}
	if time.Since(antes) > time.Minute {
		t.Fatalf("la corrida tardó %s en dejar la guía", time.Since(antes))
	}
	if !strings.Contains(string(guia), "<tv ") && !strings.Contains(string(guia), "<tv>") {
		t.Fatalf("lo que quedó no parece un XMLTV:\n%s", primeros(string(guia), 300))
	}
}

// F1-48 — Cuando el plan cambia por cualquier motivo, la guía se regenera en
// ese instante: el desfase entre la guía servida y el plan vigente nunca
// pasa de un minuto.
func TestF1Verif48LaGuiaSigueAlPlan(t *testing.T) {
	a := abre(t)
	ctx := context.Background()

	if _, err := a.Resolve(ctx); err != nil {
		t.Fatalf("la primera corrida falló: %v", err)
	}
	primera, _ := a.Guide()
	if strings.Contains(string(primera), "Kojak") {
		t.Fatal("la guía ya traía Kojak antes de crear la regla")
	}

	// El plan cambia: entra una regla nueva.
	conPrograma(t, a, "Kojak", "14:00")

	antes := time.Now()
	if _, err := a.Resolve(ctx); err != nil {
		t.Fatalf("la segunda corrida falló: %v", err)
	}
	segunda, generada := a.Guide()
	if !strings.Contains(string(segunda), "Kojak") {
		t.Fatalf("la guía no recogió el cambio del plan:\n%s", primeros(string(segunda), 800))
	}
	if desfase := time.Since(antes); desfase > time.Minute {
		t.Fatalf("la guía tardó %s en ponerse al día", desfase)
	}
	if generada.Before(antes.Add(-time.Minute)) {
		t.Fatalf("la marca de la guía (%s) es más vieja que el cambio", generada)
	}

	// Y lo que se sirve coincide con el plan guardado, ítem por ítem.
	plan, err := a.Store.Plan.ListRange(ctx, a.ChannelID, a.Now(), a.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatalf("no pude leer el plan: %v", err)
	}
	guiaItems := map[int64]model.PlanItem{}
	for _, it := range a.GuideItems() {
		guiaItems[it.ID] = it
	}
	for _, it := range plan {
		g, ok := guiaItems[it.ID]
		if !ok {
			t.Fatalf("el plan tiene el bloque %d a las %s y la guía no lo conoce",
				it.ID, it.PlannedAt)
		}
		if !g.PlannedAt.Equal(it.PlannedAt) || g.PlannedMs != it.PlannedMs {
			t.Fatalf("la guía y el plan discrepan en el bloque %d: %s/%d vs %s/%d",
				it.ID, g.PlannedAt, g.PlannedMs, it.PlannedAt, it.PlannedMs)
		}
	}
}

// F1-49 (mitad del archivo) — El mismo contenido que se sirve en /guia.xml se
// escribe en la ruta de archivo configurada, la que lee el servidor de
// streaming o el transmisor.
func TestF1Verif49LaGuiaSeEscribeEnLaRuta(t *testing.T) {
	a := abre(t)
	ctx := context.Background()

	ruta := filepath.Join(t.TempDir(), "guia", "guia.xml")
	if err := a.Store.Settings.Set(ctx, KeyGuidePath, ruta); err != nil {
		t.Fatalf("no pude guardar la ruta: %v", err)
	}
	conPrograma(t, a, "Get Smart", "10:00")

	if _, err := a.Resolve(ctx); err != nil {
		t.Fatalf("la corrida falló: %v", err)
	}
	enDisco, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("la guía no se escribió en la ruta configurada: %v", err)
	}
	servida, _ := a.Guide()
	if string(enDisco) != string(servida) {
		t.Fatal("la guía del disco no es la misma que se sirve")
	}
	if !strings.Contains(string(enDisco), "Get Smart") {
		t.Fatalf("la guía del disco no trae el programa:\n%s", primeros(string(enDisco), 600))
	}
	// Y no queda basura del escritura atómica.
	if _, err := os.Stat(ruta + ".nuevo"); err == nil {
		t.Fatal("quedó el archivo temporal .nuevo al lado de la guía")
	}
}

// F1-31 — En modo sombra, un día entero de operación normal no escribe a
// ninguna salida de video o audio real: F1 arma plan y guía, no toca el aire.
func TestF1Verif31SombraNoTocaElAire(t *testing.T) {
	a := abre(t)
	ctx := context.Background()

	ch, err := a.Store.Channel.Get(ctx, a.ChannelID)
	if err != nil {
		t.Fatalf("no pude leer el canal: %v", err)
	}
	if ch.Mode != "sombra" {
		t.Fatalf("el canal arranca en modo %q y F1 es sombra", ch.Mode)
	}

	// Ninguna salida está encendida.
	outs, err := a.Store.Output.List(ctx, a.ChannelID)
	if err != nil {
		t.Fatalf("no pude leer las salidas: %v", err)
	}
	for _, o := range outs {
		if o.ConnectionState == "conectada" {
			t.Fatalf("la salida %q (%s) dice estar conectada en modo sombra", o.Name, o.Driver)
		}
	}

	// Y aunque alguien pida modo aire, F1 no arranca ningún motor: el plan
	// se materializa y nada más.
	conPrograma(t, a, "Tarzan", "12:00")
	if _, err := a.Resolve(ctx); err != nil {
		t.Fatalf("la corrida falló: %v", err)
	}
	plan, err := a.Store.Plan.ListRange(ctx, a.ChannelID, a.Now(), a.Now().Add(48*time.Hour))
	if err != nil {
		t.Fatalf("no pude leer el plan: %v", err)
	}
	if len(plan) == 0 {
		t.Fatal("el plan salió vacío")
	}
	for _, it := range plan {
		if it.State != model.Planned {
			t.Fatalf("el bloque de las %s quedó en estado %q: en sombra nadie lo carga ni lo emite",
				it.PlannedAt, it.State)
		}
		if it.ActualAt != nil || it.ActualMs != nil || it.CuedAt != nil {
			t.Fatalf("el bloque de las %s trae as-run y en sombra no hay motor que lo escriba: %+v",
				it.PlannedAt, it)
		}
	}
}

// F1-26 — Un cambio a mano sobre un plan_item planeado de mañana tiene que
// sobrevivir a la siguiente corrida automática del resolver.
func TestF1Verif26LaEdicionAManoSobreviveAlResolver(t *testing.T) {
	a := abre(t)
	ctx := context.Background()

	conPrograma(t, a, "Kojak", "14:00")
	if _, err := a.Resolve(ctx); err != nil {
		t.Fatalf("la primera corrida falló: %v", err)
	}

	// El programador toca el bloque de mañana: lo mueve media hora.
	manana := a.Now().Add(24 * time.Hour)
	plan, err := a.Store.Plan.ListRange(ctx, a.ChannelID, manana.Add(-12*time.Hour), manana.Add(12*time.Hour))
	if err != nil {
		t.Fatalf("no pude leer el plan: %v", err)
	}
	var objetivo *model.PlanItem
	for i := range plan {
		if plan[i].State == model.Planned && plan[i].Origin == model.OriginAsset &&
			plan[i].PlannedAt.After(a.Now().Add(6*time.Hour)) {
			objetivo = &plan[i]
			break
		}
	}
	if objetivo == nil {
		t.Skip("no hay ningún bloque de mañana que editar")
	}
	// Se mueve por el mismo camino que usa PUT /plan/{id}, que es el que
	// tiene ahora la parrilla: el bloque queda fijado y deja de ser del
	// resolver. (Antes esta prueba tocaba la columna a pelo porque no había
	// ninguna otra forma de mover nada.)
	movido := objetivo.PlannedAt.Add(-5 * time.Minute)
	if err := a.Store.Plan.Fijar(ctx, objetivo.ID, model.Ms(movido), nil); err != nil {
		t.Fatalf("no pude editar el bloque a mano: %v", err)
	}

	// La corrida automática de la hora siguiente.
	if _, err := a.Resolve(ctx); err != nil {
		t.Fatalf("la segunda corrida falló: %v", err)
	}

	despues, err := a.Store.Plan.ListRange(ctx, a.ChannelID, manana.Add(-12*time.Hour), manana.Add(12*time.Hour))
	if err != nil {
		t.Fatalf("no pude releer el plan: %v", err)
	}
	for _, it := range despues {
		if it.ID == objetivo.ID {
			if !it.PlannedAt.Equal(movido) {
				t.Fatalf("el resolver movió el bloque editado a mano: quedó en %s y se había puesto en %s",
					it.PlannedAt, movido)
			}
			return
		}
	}
	t.Fatalf("el resolver borró el bloque %d que se había editado a mano para mañana: "+
		"no hay ni marca de «tocado a mano» en plan_item ni ruta de API para editarlo", objetivo.ID)
}

func primeros(s string, n int) string {
	if len(s) < n {
		return s
	}
	return s[:n]
}

// F1-28 (la mitad que falta) — El validador propio de la guía tiene que
// correr **antes de publicar** el XMLTV y rechazar la publicación. La lógica
// existe y está probada (resolver.ValidateXMLTV, TestValidadorDeLaGuia), pero
// esta prueba comprueba lo otro: que alguien la llame en el camino de
// publicación. Hoy no la llama nadie fuera de las pruebas.
func TestF1Verif28ElValidadorCorreAntesDePublicar(t *testing.T) {
	fset := token.NewFileSet()
	llamada := false
	for _, paquete := range []string{"app", "api"} {
		dir := filepath.Join("..", paquete)
		entradas, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("no pude leer %s: %v", dir, err)
		}
		for _, e := range entradas {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			ruta := filepath.Join(dir, e.Name())
			f, err := parser.ParseFile(fset, ruta, nil, 0)
			if err != nil {
				t.Fatalf("no pude leer %s: %v", ruta, err)
			}
			ast.Inspect(f, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok && (sel.Sel.Name == "ValidateXMLTV" || sel.Sel.Name == "ValidateXMLTVPorGravedad") {
					llamada = true
				}
				return true
			})
		}
	}
	if !llamada {
		t.Fatal("nadie llama a resolver.ValidateXMLTV en internal/app ni en internal/api: " +
			"la guía se escribe en disco y se sirve en /guia.xml sin pasar por el validador " +
			"(internal/app/resolve.go, rebuildGuide)")
	}
}
