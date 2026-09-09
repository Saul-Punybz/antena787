package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"antena787/internal/model"
	"antena787/internal/resolver"
	"antena787/internal/store"
)

// planRow es una fila de la parrilla: o un plan_item con su título puesto, o
// un hueco. Los dos se dibujan igual y por eso viajan en la misma lista.
type planRow struct {
	*model.PlanItem
	Gap      bool      `json:"hueco,omitempty"`
	From     time.Time `json:"inicio,omitempty"`
	To       time.Time `json:"fin,omitempty"`
	Title    string    `json:"titulo,omitempty"`
	Episode  string    `json:"episodio,omitempty"`
	Duration string    `json:"duracion,omitempty"`
	Clock    string    `json:"hora,omitempty"`
}

// catalog es el catálogo en memoria para poner nombres a los ítems sin
// pedirle a la base una consulta por fila.
type catalog struct {
	titles   map[int64]model.Title
	episodes map[int64]model.Episode
	byAsset  map[int64]model.Title
}

func (s *Server) catalog(ctx context.Context) (catalog, error) {
	c := catalog{
		titles:   map[int64]model.Title{},
		episodes: map[int64]model.Episode{},
		byAsset:  map[int64]model.Title{},
	}
	titles, err := s.App.Store.Title.List(ctx)
	if err != nil {
		return c, err
	}
	for _, t := range titles {
		c.titles[t.ID] = t
		if t.MediaAssetID != nil {
			c.byAsset[*t.MediaAssetID] = t
		}
		eps, err := s.App.Store.Episode.ListByTitle(ctx, t.ID)
		if err != nil {
			return c, err
		}
		for _, e := range eps {
			c.episodes[e.ID] = e
			if e.MediaAssetID != nil {
				if _, seen := c.byAsset[*e.MediaAssetID]; !seen {
					c.byAsset[*e.MediaAssetID] = t
				}
			}
		}
	}
	return c, nil
}

// name dice de qué es un plan_item, con las palabras de la parrilla.
func (c catalog) name(it model.PlanItem) (title, episode string) {
	if it.EpisodeID != nil {
		if e, ok := c.episodes[*it.EpisodeID]; ok {
			episode = fmt.Sprintf("T%d E%d", e.Season, e.Number)
			if e.Name != "" {
				episode += " · " + e.Name
			}
			if t, ok := c.titles[e.TitleID]; ok {
				return t.Name, episode
			}
		}
	}
	if it.MediaAssetID != nil {
		if t, ok := c.byAsset[*it.MediaAssetID]; ok {
			return t.Name, episode
		}
	}
	switch it.Origin {
	case model.OriginFiller:
		return "relleno", episode
	case model.OriginSlate:
		return "cartel de la estación", episode
	case model.OriginLiveSource:
		return "en vivo", episode
	}
	return "", episode
}

func (s *Server) row(c catalog, loc *time.Location, it *model.PlanItem) planRow {
	title, episode := c.name(*it)
	return planRow{
		PlanItem: it,
		Title:    title,
		Episode:  episode,
		Duration: humanMs(it.PlannedMs),
		Clock:    it.PlannedAt.In(loc).Format("15:04"),
	}
}

func humanMs(ms int64) string {
	d := time.Duration(ms) * time.Millisecond
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	sec := int(d.Seconds()) % 60
	switch {
	case h > 0:
		return fmt.Sprintf("%d h %02d min", h, m)
	case m > 0 && sec > 0:
		return fmt.Sprintf("%d min %02d s", m, sec)
	case m > 0:
		return fmt.Sprintf("%d min", m)
	}
	return fmt.Sprintf("%d s", sec)
}

// ── el día ────────────────────────────────────────────────────────────

func (s *Server) planDia(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ch, err := s.App.Store.Channel.Get(ctx, s.App.ChannelID)
	if err != nil {
		failStore(w, err, "el canal")
		return
	}
	day, err := queryDay(r, "dia", ch.BroadcastDay(s.Now()))
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error(), "dia")
		return
	}
	items, err := s.App.Store.Plan.ListDay(ctx, s.App.ChannelID, day)
	if err != nil {
		failStore(w, err, "leer el plan del día")
		return
	}
	cat, err := s.catalog(ctx)
	if err != nil {
		failStore(w, err, "leer la biblioteca")
		return
	}
	loc := ch.Location()

	rows := []planRow{}
	cursor := ch.DayStart(day)
	end := ch.DayEnd(day)
	for i := range items {
		it := &items[i]
		if it.PlannedAt.After(cursor) {
			rows = append(rows, hueco(cursor, it.PlannedAt, loc))
		}
		rows = append(rows, s.row(cat, loc, it))
		if fin := it.End(); fin.After(cursor) {
			cursor = fin
		}
	}
	if cursor.Before(end) {
		rows = append(rows, hueco(cursor, end, loc))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"dia_emision": day,
		"inicio":      ch.DayStart(day),
		"fin":         end,
		"items":       rows,
	})
}

func hueco(from, to time.Time, loc *time.Location) planRow {
	return planRow{
		Gap:      true,
		From:     from,
		To:       to,
		Title:    "sin programar",
		Duration: humanMs(to.Sub(from).Milliseconds()),
		Clock:    from.In(loc).Format("15:04"),
	}
}

// ── la semana ─────────────────────────────────────────────────────────

// weekSlot es media hora de la parrilla semanal: qué la ocupa, o nada. Un
// tramo vacío llega con `titulo` y `plan_id` en nulo, que es como la
// interfaz sabe que ahí no hay nada puesto (web/src/lib/tipos.ts,
// `FranjaSemana`).
type weekSlot struct {
	Clock    string  `json:"hora"`
	Title    *string `json:"titulo"`
	Live     bool    `json:"en_vivo"`
	Duration int64   `json:"duracion_ms"`
	PlanID   *int64  `json:"plan_id"`
	Fijado   bool    `json:"fijado"`
	State    string  `json:"estado,omitempty"`
}

// FranjasPorDia son las medias horas que tiene un día en la tira semanal.
const FranjasPorDia = 48

// franjaMs es lo que dura una franja de la tira semanal.
const franjaMs int64 = 30 * 60 * 1000

func (s *Server) planSemana(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ch, err := s.App.Store.Channel.Get(ctx, s.App.ChannelID)
	if err != nil {
		failStore(w, err, "el canal")
		return
	}
	from, err := queryDay(r, "desde", ch.BroadcastDay(s.Now()))
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error(), "desde")
		return
	}
	cat, err := s.catalog(ctx)
	if err != nil {
		failStore(w, err, "leer la biblioteca")
		return
	}
	loc := ch.Location()

	type diaSemana struct {
		Day    model.Day  `json:"dia"`
		Slots  []weekSlot `json:"franjas"`
		Vacias float64    `json:"horas_vacias"`
	}
	dias := []diaSemana{}
	vaciasSemana := 0.0
	for n := 0; n < 7; n++ {
		day := from.Add(n)
		// La tira se dibuja sobre el reloj del día natural —la interfaz la
		// recorre de las 6:00 a la medianoche— así que se pide el plan de
		// ese calendario completo, no el del día de emisión, que empieza y
		// termina desplazado.
		medianoche := day.Time(loc)
		items, err := s.App.Store.Plan.ListRange(ctx, s.App.ChannelID,
			medianoche, medianoche.AddDate(0, 0, 1))
		if err != nil {
			failStore(w, err, "leer el plan de la semana")
			return
		}
		slots := make([]weekSlot, 0, FranjasPorDia)
		vacias := 0.0
		for k := 0; k < FranjasPorDia; k++ {
			t := medianoche.Add(time.Duration(k) * 30 * time.Minute)
			slot := weekSlot{
				Clock:    t.In(loc).Format("15:04"),
				Duration: franjaMs,
			}
			ocupada := false
			for i := range items {
				it := items[i]
				if t.Before(it.PlannedAt) || !t.Before(it.End()) {
					continue
				}
				name, _ := cat.name(it)
				titulo := name
				id := it.ID
				slot.Title = &titulo
				slot.PlanID = &id
				slot.Fijado = it.Fijado
				slot.State = string(it.State)
				slot.Live = it.Origin == model.OriginLiveSource
				ocupada = true
				break
			}
			if !ocupada {
				vacias += 0.5
			}
			slots = append(slots, slot)
		}
		vaciasSemana += vacias
		dias = append(dias, diaSemana{Day: day, Slots: slots, Vacias: vacias})
	}

	todas := make([][]weekSlot, 0, len(dias))
	for _, d := range dias {
		todas = append(todas, d.Slots)
	}
	cuerpo := map[string]any{
		"desde":               from,
		"dias":                dias,
		"horas_vacias_semana": vaciasSemana,
	}
	if nota := notaDeLaSemana(todas); nota != "" {
		cuerpo["nota"] = nota
	}
	writeJSON(w, http.StatusOK, cuerpo)
}

// notaDeLaSemana busca el tramo más largo que está vacío **los siete días** y
// lo dice con palabras. Es la frase que la parrilla enseña debajo de la tira:
// "1:00 – 6:00 AM está vacío los siete días".
func notaDeLaSemana(dias [][]weekSlot) string {
	if len(dias) == 0 {
		return ""
	}
	vaciaSiempre := func(k int) bool {
		for _, slots := range dias {
			if k >= len(slots) || slots[k].Title != nil {
				return false
			}
		}
		return true
	}
	mejorIni, mejorLargo := -1, 0
	ini, largo := -1, 0
	for k := 0; k <= FranjasPorDia; k++ {
		if k < FranjasPorDia && vaciaSiempre(k) {
			if ini < 0 {
				ini, largo = k, 0
			}
			largo++
			continue
		}
		if ini >= 0 && largo > mejorLargo {
			mejorIni, mejorLargo = ini, largo
		}
		ini, largo = -1, 0
	}
	// Menos de dos horas seguidas no merece una frase.
	if mejorIni < 0 || mejorLargo < 4 {
		return ""
	}
	desde := dias[0][mejorIni].Clock
	hasta := "24:00"
	if fin := mejorIni + mejorLargo; fin < FranjasPorDia {
		hasta = dias[0][fin].Clock
	}
	return fmt.Sprintf("Además, de %s a %s está vacío los siete días", desde, hasta)
}

// ── el mes ────────────────────────────────────────────────────────────

func (s *Server) planMes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ch, err := s.App.Store.Channel.Get(ctx, s.App.ChannelID)
	if err != nil {
		failStore(w, err, "el canal")
		return
	}
	mes := strings.TrimSpace(r.URL.Query().Get("mes"))
	if mes == "" {
		mes = s.Now().In(ch.Location()).Format("2006-01")
	}
	first, err := time.Parse("2006-01", mes)
	if err != nil {
		failf(w, http.StatusBadRequest, "mes", "%q no es un mes: se escribe AAAA-MM", mes)
		return
	}
	rules, err := s.App.Store.Rule.ListAll(ctx, s.App.ChannelID)
	if err != nil {
		failStore(w, err, "leer las reglas")
		return
	}
	cat, err := s.catalog(ctx)
	if err != nil {
		failStore(w, err, "leer la biblioteca")
		return
	}

	type diaMes struct {
		Day      model.Day `json:"dia"`
		Libres   float64   `json:"horas_sin_llenar"`
		Llenas   []bool    `json:"franjas_llenas"`
		Vencen   []string  `json:"vencimientos"`
		Estrenan []string  `json:"estrenos"`
	}
	loc := ch.Location()
	days := []diaMes{}
	vaciasMes := 0.0
	for d := first; d.Month() == first.Month(); d = d.AddDate(0, 0, 1) {
		day := model.Day(d.Format("2006-01-02"))
		medianoche := day.Time(loc)
		items, err := s.App.Store.Plan.ListRange(ctx, s.App.ChannelID,
			medianoche, medianoche.AddDate(0, 0, 1))
		if err != nil {
			failStore(w, err, "leer el plan del mes")
			return
		}
		// La barra del mes es la misma tira de 48 medias horas que la de la
		// semana: llena o vacía, sin nombres.
		llenas := make([]bool, FranjasPorDia)
		libres := 0.0
		for k := 0; k < FranjasPorDia; k++ {
			t := medianoche.Add(time.Duration(k) * 30 * time.Minute)
			for _, it := range items {
				if !t.Before(it.PlannedAt) && t.Before(it.End()) {
					llenas[k] = true
					break
				}
			}
			if !llenas[k] {
				libres += 0.5
			}
		}
		vaciasMes += libres

		vencen, estrenan := []string{}, []string{}
		for _, rule := range rules {
			name := ruleTitleName(cat, rule)
			if rule.To == day {
				vencen = append(vencen, name)
			}
			if rule.From == day {
				estrenan = append(estrenan, name)
			}
		}
		days = append(days, diaMes{
			Day:      day,
			Libres:   round1(libres),
			Llenas:   llenas,
			Vencen:   vencen,
			Estrenan: estrenan,
		})
	}
	porcentaje := 0
	if total := float64(len(days)) * 24; total > 0 {
		porcentaje = int(vaciasMes/total*100 + 0.5)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"mes":              mes,
		"dias":             days,
		"horas_vacias_mes": round1(vaciasMes),
		"porcentaje_vacio": porcentaje,
	})
}

func ruleTitleName(cat catalog, rule model.ScheduleRule) string {
	if rule.TitleID != nil {
		if t, ok := cat.titles[*rule.TitleID]; ok {
			return t.Name
		}
	}
	if rule.Kind == model.RuleTimeShift {
		return "el diferido"
	}
	return "una regla sin título"
}

func round1(f float64) float64 {
	return float64(int(f*10+0.5)) / 10
}

// ── recalcular ────────────────────────────────────────────────────────

func (s *Server) planRecalcular(w http.ResponseWriter, r *http.Request) {
	out, err := s.App.Resolve(r.Context())
	if err != nil {
		failStore(w, err, "armar el plan")
		return
	}
	avisos := out.Warnings
	if avisos == nil {
		avisos = []resolver.Warning{}
	}
	s.audit(r, "plan", nil, "recalcular", "", fmt.Sprintf("%d bloques", len(out.Items)))
	writeJSON(w, http.StatusOK, map[string]any{
		"bloques": len(out.Items),
		"avisos":  avisos,
	})
}

// ── llenar con diferido ───────────────────────────────────────────────

// planDiferido crea de un clic la regla de diferido que llena una franja
// vacía con lo que ya salió antes (auditoría B12: después de importar, la
// parrilla enseña las horas vacías con este botón).
func (s *Server) planDiferido(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Desde       string `json:"desde"`
		Hasta       string `json:"hasta"`
		OrigenDesde string `json:"origen_desde"`
		OrigenHasta string `json:"origen_hasta"`
		Dias        string `json:"patron_de_dias"`
		FechaInicio string `json:"fecha_inicio"`
		FechaFin    string `json:"fecha_fin"`
	}
	if !decode(w, r, &body) {
		return
	}
	ctx := r.Context()
	ch, err := s.App.Store.Channel.Get(ctx, s.App.ChannelID)
	if err != nil {
		failStore(w, err, "el canal")
		return
	}

	horas := map[string]*model.Minutes{}
	for campo, raw := range map[string]string{
		"desde": body.Desde, "hasta": body.Hasta,
		"origen_desde": body.OrigenDesde, "origen_hasta": body.OrigenHasta,
	} {
		m, err := model.ParseClock(raw)
		if err != nil {
			fail(w, http.StatusBadRequest, err.Error(), campo)
			return
		}
		v := m
		horas[campo] = &v
	}

	dur := int64(*horas["hasta"]-*horas["desde"]) * 60 * 1000
	if dur <= 0 {
		dur += 24 * 60 * 60 * 1000 // cruza la medianoche: la madrugada del día siguiente
	}

	today := ch.BroadcastDay(s.Now())
	rule := model.ScheduleRule{
		ChannelID:         s.App.ChannelID,
		Kind:              model.RuleTimeShift,
		Days:              model.DayPattern("LMMJVSD"),
		At:                *horas["desde"],
		SlotMs:            dur,
		From:              today,
		To:                today.Add(365),
		SourceWindowStart: horas["origen_desde"],
		SourceWindowEnd:   horas["origen_hasta"],
		EpisodesPerRun:    1,
		Active:            true,
	}
	if body.Dias != "" {
		rule.Days = model.DayPattern(body.Dias)
	}
	if body.FechaInicio != "" {
		rule.From = model.Day(body.FechaInicio)
	}
	if body.FechaFin != "" {
		rule.To = model.Day(body.FechaFin)
	}
	if !s.prepareRule(ctx, w, &rule) {
		return
	}
	if err := s.App.Store.Rule.Insert(ctx, &rule); err != nil {
		failStore(w, err, "guardar la regla de diferido")
		return
	}
	s.audit(r, "schedule_rule", &rule.ID, "diferido", "", describeRule(rule))

	out, err := s.App.Resolve(ctx)
	if err != nil {
		failStore(w, err, "armar el plan")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"regla":   s.ruleOut(ctx, rule, today),
		"bloques": len(out.Items),
		"avisos":  out.Warnings,
	})
}

// ── la guía ───────────────────────────────────────────────────────────

// guiaXML sirve el XMLTV vigente. Sin clave a propósito: lo lee el
// transmisor o el servidor de streaming, que no tienen dónde escribirla.
func (s *Server) guiaXML(w http.ResponseWriter, r *http.Request) {
	data, at := s.App.Guide()
	if len(data) == 0 {
		if err := s.App.RefreshGuide(r.Context()); err != nil {
			fail(w, http.StatusInternalServerError, "no se pudo armar la guía: "+err.Error(), "")
			return
		}
		data, at = s.App.Guide()
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	if !at.IsZero() {
		w.Header().Set("Last-Modified", at.UTC().Format(http.TimeFormat))
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// guiaContraPlan compara lo que dice la guía publicada con lo que dice el
// plan de verdad. Es la pantalla que contesta "¿lo que anuncié es lo que va
// a salir?".
func (s *Server) guiaContraPlan(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ch, err := s.App.Store.Channel.Get(ctx, s.App.ChannelID)
	if err != nil {
		failStore(w, err, "el canal")
		return
	}
	day, err := queryDay(r, "dia", ch.BroadcastDay(s.Now()))
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error(), "dia")
		return
	}
	plan, err := s.App.Store.Plan.ListDay(ctx, s.App.ChannelID, day)
	if err != nil {
		failStore(w, err, "leer el plan del día")
		return
	}
	cat, err := s.catalog(ctx)
	if err != nil {
		failStore(w, err, "leer la biblioteca")
		return
	}

	guia := []model.PlanItem{}
	for _, it := range s.App.GuideItems() {
		if it.BroadcastDay == day && it.Origin != model.OriginFiller && it.Origin != model.OriginSlate {
			guia = append(guia, it)
		}
	}
	sort.SliceStable(guia, func(i, j int) bool { return guia[i].PlannedAt.Before(guia[j].PlannedAt) })

	usados := map[int64]bool{}
	filas := []map[string]any{}
	for i := range guia {
		g := guia[i]
		var match *model.PlanItem
		for j := range plan {
			p := plan[j]
			if usados[p.ID] || p.ID != g.ID {
				continue
			}
			match = &plan[j]
			usados[p.ID] = true
			break
		}
		fila := map[string]any{
			"guia":     programaDeGuia(cat, guia[i]),
			"coincide": match != nil && match.PlannedAt.Equal(g.PlannedAt) && match.PlannedMs == g.PlannedMs,
		}
		if match != nil {
			fila["plan"] = programaDeGuia(cat, *match)
		} else {
			fila["plan"] = nil
		}
		filas = append(filas, fila)
	}
	// Lo que está en el plan y la guía no anunció: también hay que verlo.
	for j := range plan {
		p := plan[j]
		if usados[p.ID] || p.Origin == model.OriginFiller || p.Origin == model.OriginSlate {
			continue
		}
		filas = append(filas, map[string]any{
			"guia":     nil,
			"plan":     programaDeGuia(cat, plan[j]),
			"coincide": false,
		})
	}

	cuerpo := map[string]any{
		"dia":                    day,
		"filas":                  filas,
		"identificador_de_canal": resolver.ChannelID(ch),
	}
	if _, at := s.App.Guide(); !at.IsZero() {
		cuerpo["revalidada"] = at
	}
	if porque := porQueNoCoinciden(filas); porque != "" {
		cuerpo["por_que_no_coinciden"] = porque
	}
	writeJSON(w, http.StatusOK, cuerpo)
}

// programaDeGuia es un bloque tal como lo lee la pantalla de la guía: qué
// es, cuándo empieza y cuánto dura (web/src/lib/tipos.ts, `ProgramaDeGuia`).
func programaDeGuia(cat catalog, it model.PlanItem) map[string]any {
	title, _ := cat.name(it)
	return map[string]any{
		"titulo":      title,
		"inicio":      it.PlannedAt,
		"duracion_ms": it.PlannedMs,
	}
}

// porQueNoCoinciden explica en una frase por qué la guía y el plan no dicen
// lo mismo, cuando no lo dicen.
func porQueNoCoinciden(filas []map[string]any) string {
	malas := 0
	for _, f := range filas {
		if f["coincide"] != true {
			malas++
		}
	}
	if malas == 0 {
		return ""
	}
	if malas == 1 {
		return "Un bloque cambió después de publicar la guía. Se arregla solo en el próximo recálculo; " +
			"si corre prisa, pulsa «Recalcular» en la parrilla."
	}
	return fmt.Sprintf("%d bloques cambiaron después de publicar la guía. Se arreglan solos en el "+
		"próximo recálculo; si corre prisa, pulsa «Recalcular» en la parrilla.", malas)
}

// ── mover un bloque a mano (F1-26) ────────────────────────────────────

// cambioDePlan es el cuerpo de PUT /plan/{id}: cualquier subconjunto de las
// tres cosas que una persona puede tocar. Los punteros distinguen "no lo
// mandó" de "lo mandó vacío".
type cambioDePlan struct {
	Instante *string `json:"instante_planeado"`
	Duracion *int64  `json:"duracion_planeada_ms"`
	Fijado   *bool   `json:"fijado"`
}

// planPut mueve un bloque del plan a mano, o lo suelta.
//
// Con `instante_planeado` o `duracion_planeada_ms` el bloque queda **fijado**:
// la corrida siguiente del resolver ni lo mueve ni lo borra. Con
// `{"fijado": false}` a secas se suelta y vuelve a mandar la regla. En los
// dos casos la guía se rehace en la misma corrida (F1-48) y se avisa por el
// bus, para que la parrilla se refresque sola.
func (s *Server) planPut(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var body cambioDePlan
	if !decode(w, r, &body) {
		return
	}
	ctx := r.Context()
	antes, err := s.App.PlanItem(ctx, id)
	if err != nil {
		failStore(w, err, "ese bloque del plan")
		return
	}

	switch {
	case body.Instante != nil || body.Duracion != nil:
		if !s.fijarPlan(w, r, antes, body) {
			return
		}
	case body.Fijado != nil && !*body.Fijado:
		if err := s.App.Store.Plan.Liberar(ctx, id); err != nil {
			s.falloDelPlan(w, ctx, err, time.Time{}, 0)
			return
		}
		s.audit(r, "plan_item", &id, "fijado", "fijado", "suelto")
	case body.Fijado != nil && *body.Fijado:
		if err := s.App.Store.Plan.Fijar(ctx, id, model.Ms(antes.PlannedAt), nil); err != nil {
			s.falloDelPlan(w, ctx, err, antes.PlannedAt, antes.PlannedMs)
			return
		}
		s.audit(r, "plan_item", &id, "fijado", "suelto", "fijado")
	default:
		fail(w, http.StatusBadRequest,
			"no me dijiste qué cambiar: la hora, la duración, o soltarlo con \"fijado\": false", "")
		return
	}

	// La guía sigue al plan en la misma corrida y la interfaz se entera por
	// el mismo evento que publica el recálculo.
	if err := s.App.RefreshGuide(ctx); err != nil {
		s.App.Publish("plan", "guia", "no se pudo publicar la guía: "+err.Error())
	}
	s.App.Publish("plan", "recalculado", "alguien movió un bloque a mano en la parrilla")

	despues, err := s.App.PlanItem(ctx, id)
	if err != nil {
		failStore(w, err, "ese bloque del plan")
		return
	}
	ch, err := s.App.Store.Channel.Get(ctx, s.App.ChannelID)
	if err != nil {
		failStore(w, err, "el canal")
		return
	}
	cat, err := s.catalog(ctx)
	if err != nil {
		failStore(w, err, "leer la biblioteca")
		return
	}
	writeJSON(w, http.StatusOK, s.row(cat, ch.Location(), &despues))
}

// fijarPlan aplica la parte de mover y estirar. Devuelve false si ya contestó
// con un error.
func (s *Server) fijarPlan(w http.ResponseWriter, r *http.Request, antes model.PlanItem, body cambioDePlan) bool {
	ctx := r.Context()
	cuando := antes.PlannedAt
	if body.Instante != nil {
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(*body.Instante))
		if err != nil {
			failf(w, http.StatusBadRequest, "instante_planeado",
				"no entendí la hora %q: se escribe como 2026-09-08T15:30:00-04:00", *body.Instante)
			return false
		}
		cuando = t.UTC()
	}
	var duracion *int64
	if body.Duracion != nil {
		if *body.Duracion <= 0 {
			fail(w, http.StatusBadRequest,
				"la duración tiene que ser mayor que cero", "duracion_planeada_ms")
			return false
		}
		d := *body.Duracion
		duracion = &d
	}
	if err := s.App.Store.Plan.Fijar(ctx, antes.ID, model.Ms(cuando), duracion); err != nil {
		dur := antes.PlannedMs
		if duracion != nil {
			dur = *duracion
		}
		s.falloDelPlan(w, ctx, err, cuando, dur)
		return false
	}
	s.audit(r, "plan_item", &antes.ID, "instante_planeado",
		antes.PlannedAt.Format(time.RFC3339), cuando.Format(time.RFC3339))
	if duracion != nil {
		s.audit(r, "plan_item", &antes.ID, "duracion_planeada_ms",
			humanMs(antes.PlannedMs), humanMs(*duracion))
	}
	return true
}

// falloDelPlan traduce lo que el esquema rechazó a la frase que ve la
// persona, con el código que le corresponde.
func (s *Server) falloDelPlan(w http.ResponseWriter, ctx context.Context, err error,
	desde time.Time, durMs int64) {
	switch {
	case errors.Is(err, store.ErrOverlap):
		fail(w, http.StatusConflict, s.choque(ctx, desde, durMs), "instante_planeado")
	case errors.Is(err, store.ErrFueraDeVigencia):
		fail(w, http.StatusBadRequest,
			"esa hora se sale de las fechas de la regla que puso el bloque: cambia la regla o suelta el bloque",
			"instante_planeado")
	case errors.Is(err, store.ErrNotFound):
		fail(w, http.StatusNotFound, "ese bloque del plan ya no está", "")
	default:
		failf(w, http.StatusInternalServerError, "", "no se pudo mover el bloque: %s", err)
	}
}

// choque busca qué hay puesto donde la persona quiso poner el bloque, para
// poder decirlo por su nombre. Si no se puede averiguar barato, lo dice sin
// nombre: la frase sigue siendo útil.
func (s *Server) choque(ctx context.Context, desde time.Time, durMs int64) string {
	generico := "a esa hora ya hay otra cosa puesta: muévela primero, o elige otro hueco"
	if desde.IsZero() || durMs <= 0 {
		return generico
	}
	hasta := desde.Add(time.Duration(durMs) * time.Millisecond)
	items, err := s.App.Store.Plan.ListRange(ctx, s.App.ChannelID, desde, hasta)
	if err != nil || len(items) == 0 {
		return generico
	}
	cat, err := s.catalog(ctx)
	if err != nil {
		return generico
	}
	ch, err := s.App.Store.Channel.Get(ctx, s.App.ChannelID)
	if err != nil {
		return generico
	}
	for _, it := range items {
		name, _ := cat.name(it)
		if name == "" {
			continue
		}
		return fmt.Sprintf("a esa hora ya está %s, de %s a %s: muévelo primero, o elige otro hueco",
			name,
			it.PlannedAt.In(ch.Location()).Format("15:04"),
			it.End().In(ch.Location()).Format("15:04"))
	}
	return generico
}
