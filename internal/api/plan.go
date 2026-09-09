package api

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"antena787/internal/model"
	"antena787/internal/resolver"
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

// weekSlot es media hora de la parrilla semanal: lo que hay puesto y si es
// un hueco. Es lo que dibuja Parrilla · Semana.
type weekSlot struct {
	Clock string `json:"hora"`
	Title string `json:"titulo"`
	State string `json:"estado,omitempty"`
	Gap   bool   `json:"hueco"`
}

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

	days := []map[string]any{}
	for n := 0; n < 7; n++ {
		day := from.Add(n)
		items, err := s.App.Store.Plan.ListDay(ctx, s.App.ChannelID, day)
		if err != nil {
			failStore(w, err, "leer el plan de la semana")
			return
		}
		slots := []weekSlot{}
		for t := ch.DayStart(day); t.Before(ch.DayEnd(day)); t = t.Add(30 * time.Minute) {
			slot := weekSlot{Clock: t.In(loc).Format("15:04"), Gap: true, Title: "sin programar"}
			for i := range items {
				it := items[i]
				if !t.Before(it.PlannedAt) && t.Before(it.End()) {
					name, _ := cat.name(it)
					slot = weekSlot{Clock: slot.Clock, Title: name, State: string(it.State)}
					break
				}
			}
			slots = append(slots, slot)
		}
		days = append(days, map[string]any{"dia_emision": day, "franjas": slots})
	}
	writeJSON(w, http.StatusOK, map[string]any{"desde": from, "dias": days})
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

	days := []map[string]any{}
	for d := first; d.Month() == first.Month(); d = d.AddDate(0, 0, 1) {
		day := model.Day(d.Format("2006-01-02"))
		items, err := s.App.Store.Plan.ListDay(ctx, s.App.ChannelID, day)
		if err != nil {
			failStore(w, err, "leer el plan del mes")
			return
		}
		var covered time.Duration
		for _, it := range items {
			covered += time.Duration(it.PlannedMs) * time.Millisecond
		}
		total := ch.DayEnd(day).Sub(ch.DayStart(day))
		libre := total - covered
		if libre < 0 {
			libre = 0
		}
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
		days = append(days, map[string]any{
			"dia_emision":      day,
			"horas_sin_llenar": round1(libre.Hours()),
			"vencimientos":     vencen,
			"estrenos":         estrenan,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"mes": mes, "dias": days})
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
	loc := ch.Location()

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
			"guia":     s.row(cat, loc, &guia[i]),
			"coincide": match != nil && match.PlannedAt.Equal(g.PlannedAt) && match.PlannedMs == g.PlannedMs,
		}
		if match != nil {
			fila["plan"] = s.row(cat, loc, match)
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
			"plan":     s.row(cat, loc, &plan[j]),
			"coincide": false,
		})
	}
	writeJSON(w, http.StatusOK, filas)
}
