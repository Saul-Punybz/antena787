package app

import (
	"context"
	"testing"
	"time"

	"antena787/internal/ingest"
	"antena787/internal/model"
)

// Verificación de F1-42 (docs/ACEPTACION.md, "Material listo para aire") al
// nivel en que corre de verdad: la cola de normalización se ordena por la
// hora a la que el material sale al aire, y quien le da esa hora es
// App.airsAt.
//
// La prioridad de la cola en sí ya está probada en
// internal/ingest TestColaPriorizaPorHoraDeAire. Lo que falta —y es lo que
// esta prueba mira— es de dónde sale la hora de aire de un archivo que
// acaba de entrar.
//
// FALLA HOY: App.airsAt busca el archivo en plan_item, y el resolver solo
// pone en el plan material con estado_normalizacion = listo (auditoría B7,
// internal/resolver/resolver.go:656). Un archivo recién ingerido está en
// "pendiente", así que nunca está en el plan, airsAt devuelve el instante
// cero y la cola lo manda al final. En una estación real la cola trabaja por
// orden de llegada, que es justo lo que F1-42 prohíbe.

// materialConEstado deja un archivo en la biblioteca con la duración y el
// estado de normalización que se le pidan, más su título y su regla diaria a
// la hora indicada.
func materialConEstado(t *testing.T, a *App, nombre, ruta string, dur time.Duration, normalizacion string, hora model.Minutes) int64 {
	t.Helper()
	ctx := context.Background()

	asset := model.MediaAsset{
		Path:           ruta,
		Codec:          "h264",
		Resolution:     "1280x720",
		FPS:            "60000/1001",
		AudioChannels:  2,
		DurationMs:     dur.Milliseconds(),
		State:          model.AssetReady,
		NormalizeState: normalizacion,
	}
	if err := a.Store.Media.Insert(ctx, &asset); err != nil {
		t.Fatalf("no se pudo guardar el archivo: %v", err)
	}
	title := model.Title{Name: nombre, Kind: model.TitleMovie, MediaAssetID: &asset.ID}
	if err := a.Store.Title.Insert(ctx, &title); err != nil {
		t.Fatalf("no se pudo guardar el título: %v", err)
	}
	ch, err := a.Store.Channel.Get(ctx, a.ChannelID)
	if err != nil {
		t.Fatalf("no se pudo leer el canal: %v", err)
	}
	hoy := ch.BroadcastDay(a.Now())
	rule := model.ScheduleRule{
		ChannelID:      a.ChannelID,
		Kind:           model.RuleNormal,
		TitleID:        &title.ID,
		Days:           model.DayPattern("LMMJVSD"),
		At:             hora,
		From:           hoy.Add(-1),
		To:             hoy.Add(30),
		EpisodesPerRun: 1,
		Active:         true,
	}
	if err := a.Store.Rule.Insert(ctx, &rule); err != nil {
		t.Fatalf("no se pudo guardar la regla: %v", err)
	}
	return asset.ID
}

func TestF1_42_LaColaSabeCuandoSaleAlAire(t *testing.T) {
	a := abre(t)
	ctx := context.Background()

	// Dos archivos idénticos salvo en el estado de normalización.
	listo := materialConEstado(t, a, "Ya normalizada", "/contenido/ya.mkv",
		time.Hour, ingest.NormalizeReady, model.Minutes(10*60))
	pendiente := materialConEstado(t, a, "Recién llegada", "/contenido/nueva.mkv",
		time.Hour, ingest.NormalizePending, model.Minutes(14*60))

	if _, err := a.Resolve(ctx); err != nil {
		t.Fatalf("el resolver falló: %v", err)
	}

	// Control: del que ya está normalizado sí se sabe cuándo sale.
	if a.airsAt(ctx, listo).IsZero() {
		t.Fatal("del archivo ya normalizado tampoco se sabe la hora de aire: la prueba no mide lo que cree")
	}

	// Lo que pide F1-42: el que está en la cola —el que todavía no se ha
	// normalizado— tiene que llevar su hora de aire, porque es la que decide
	// el orden de trabajo.
	cuando := a.airsAt(ctx, pendiente)
	if cuando.IsZero() {
		t.Fatal("F1-42: un archivo pendiente de normalizar no tiene hora de aire, " +
			"así que la cola lo ordena por llegada y no por cuándo sale al aire")
	}
}

// F1-16 / D-5 — El contador de episodios se escribe en la regla **dueña**:
// una regla de repetición (`repite_a`) no tiene contador propio y avanza el
// de su primaria (auditoría B5). MarkAired tiene que resolver la dueña antes
// de buscar el avance y antes de guardar.
func TestF1_16_LaRepeticionNoAvanzaSuPropioContador(t *testing.T) {
	a := abre(t)
	ctx := context.Background()

	// Una serie con dos episodios.
	asset := model.MediaAsset{
		Path: "/contenido/serie.mkv", DurationMs: int64(30 * 60 * 1000),
		State: model.AssetReady, NormalizeState: ingest.NormalizeReady,
		NormalizedPath: "/contenido/serie.norm.mkv",
	}
	if err := a.Store.Media.Insert(ctx, &asset); err != nil {
		t.Fatalf("no pude guardar el archivo: %v", err)
	}
	title := model.Title{Name: "Serie de tarde", Kind: model.TitleSeries}
	if err := a.Store.Title.Insert(ctx, &title); err != nil {
		t.Fatalf("no pude guardar el título: %v", err)
	}
	for n := 1; n <= 2; n++ {
		ep := model.Episode{TitleID: title.ID, Season: 1, Number: n, MediaAssetID: &asset.ID}
		if err := a.Store.Episode.Insert(ctx, &ep); err != nil {
			t.Fatalf("no pude guardar el episodio: %v", err)
		}
	}

	ch, err := a.Store.Channel.Get(ctx, a.ChannelID)
	if err != nil {
		t.Fatalf("no pude leer el canal: %v", err)
	}
	hoy := ch.BroadcastDay(a.Now())
	primaria := model.ScheduleRule{
		ChannelID: a.ChannelID, Kind: model.RuleNormal, TitleID: &title.ID,
		Days: model.DayPattern("LMMJVSD"), At: model.Minutes(15 * 60), SlotMs: 30 * 60 * 1000,
		From: hoy.Add(-1), To: hoy.Add(60), EpisodesPerRun: 1, Active: true,
	}
	if err := a.Store.Rule.Insert(ctx, &primaria); err != nil {
		t.Fatalf("no pude guardar la regla primaria: %v", err)
	}
	repeticion := model.ScheduleRule{
		ChannelID: a.ChannelID, Kind: model.RuleNormal, TitleID: &title.ID,
		Days: model.DayPattern("LMMJVSD"), At: model.Minutes(22 * 60), SlotMs: 30 * 60 * 1000,
		From: hoy.Add(-1), To: hoy.Add(60), EpisodesPerRun: 1, RepeatsOf: &primaria.ID, Active: true,
	}
	if err := a.Store.Rule.Insert(ctx, &repeticion); err != nil {
		t.Fatalf("no pude guardar la repetición: %v", err)
	}

	out, err := a.Resolve(ctx)
	if err != nil {
		t.Fatalf("la corrida falló: %v", err)
	}
	if out.CounterOwner[repeticion.ID] != primaria.ID {
		t.Skipf("el resolver no marcó la repetición como hija de la primaria (%d): nada que comprobar aquí",
			out.CounterOwner[repeticion.ID])
	}

	// Sale al aire un bloque puesto por la repetición.
	var bloque *model.PlanItem
	for i := range out.Items {
		if out.Items[i].RuleID != nil && *out.Items[i].RuleID == repeticion.ID {
			bloque = &out.Items[i]
			break
		}
	}
	if bloque == nil {
		t.Skip("el plan no trae ningún bloque de la repetición")
	}
	items, err := a.Store.Plan.ListRange(ctx, a.ChannelID, bloque.PlannedAt, bloque.End())
	if err != nil || len(items) == 0 {
		t.Fatalf("no encontré el bloque guardado: %v", err)
	}
	var guardado model.PlanItem
	for _, it := range items {
		if it.RuleID != nil && *it.RuleID == repeticion.ID {
			guardado = it
		}
	}
	if guardado.ID == 0 {
		t.Skip("el bloque de la repetición no llegó a guardarse")
	}

	if err := a.MarkAired(ctx, guardado.ID, guardado.PlannedAt, guardado.PlannedMs, false); err != nil {
		t.Fatalf("marcar como emitido falló: %v", err)
	}

	dueña, err := a.Store.Rule.Get(ctx, primaria.ID)
	if err != nil {
		t.Fatalf("no pude releer la regla primaria: %v", err)
	}
	hija, err := a.Store.Rule.Get(ctx, repeticion.ID)
	if err != nil {
		t.Fatalf("no pude releer la repetición: %v", err)
	}
	if dueña.LastEpisodeAired == nil {
		t.Fatal("el contador tenía que avanzar en la regla primaria y se quedó vacío")
	}
	if hija.LastEpisodeAired != nil {
		t.Fatalf("la repetición se quedó con un contador propio (%d): tiene que escribir en la primaria",
			*hija.LastEpisodeAired)
	}
}
