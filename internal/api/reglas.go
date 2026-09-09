package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"antena787/internal/importer"
	"antena787/internal/model"
	"antena787/internal/store"
)

// ruleOut es una regla como la quiere la pantalla: con su título dentro y
// los días que le quedan, para no pedir dos cosas por cada fila.
type ruleOut struct {
	model.ScheduleRule
	// `titulo` es el nombre, como lo pinta la pantalla (contrato de
	// tipos.ts: Regla.titulo es texto); la ficha entera va aparte.
	Titulo   string       `json:"titulo"`
	Ficha    *model.Title `json:"ficha,omitempty"`
	DaysLeft int          `json:"dias_restantes"`
	Pattern  string       `json:"patron_en_cristiano"`
	Clock    string       `json:"hora_en_cristiano"`
}

func (s *Server) ruleOut(ctx context.Context, rule model.ScheduleRule, today model.Day) ruleOut {
	out := ruleOut{
		ScheduleRule: rule,
		DaysLeft:     rule.DaysLeft(today),
		Pattern:      importer.DescribePattern(rule.Days),
		Clock:        rule.At.String(),
	}
	if rule.TitleID != nil {
		if t, err := s.App.Store.Title.Get(ctx, *rule.TitleID); err == nil {
			out.Ficha = &t
			out.Titulo = t.Name
		}
	}
	return out
}

func (s *Server) reglasList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rules, err := s.App.Store.Rule.ListAll(ctx, s.App.ChannelID)
	if err != nil {
		failStore(w, err, "leer las reglas")
		return
	}
	ch, err := s.App.Store.Channel.Get(ctx, s.App.ChannelID)
	if err != nil {
		failStore(w, err, "el canal")
		return
	}
	today := ch.BroadcastDay(s.Now())
	out := make([]ruleOut, 0, len(rules))
	for _, rule := range rules {
		out = append(out, s.ruleOut(ctx, rule, today))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) reglasPost(w http.ResponseWriter, r *http.Request) {
	// Una regla nueva nace encendida salvo que quien la manda diga
	// `activa: false`: sin este valor previo, omitir el campo la dejaba
	// apagada en silencio y el plan la ignoraba (modo sombra, 9 sept 2026).
	rule := model.ScheduleRule{Active: true}
	if !decode(w, r, &rule) {
		return
	}
	ctx := r.Context()
	rule.ID = 0
	if !s.prepareRule(ctx, w, &rule) {
		return
	}
	if err := s.App.Store.Rule.Insert(ctx, &rule); err != nil {
		failStore(w, err, "guardar la regla")
		return
	}
	s.audit(r, "schedule_rule", &rule.ID, "creada", "", describeRule(rule))
	s.App.Recalc()

	ch, _ := s.App.Store.Channel.Get(ctx, s.App.ChannelID)
	writeJSON(w, http.StatusCreated, s.ruleOut(ctx, rule, ch.BroadcastDay(s.Now())))
}

// reglasPut edita una regla. Con ?solo_hoy=1 no se toca la regla de siempre:
// se crea una de un solo día que la releva ese día y se va sola. Es la forma
// de "hoy en vez de esto va aquello" sin romper la semana entera.
func (s *Server) reglasPut(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	old, err := s.App.Store.Rule.Get(ctx, id)
	if err != nil {
		failStore(w, err, "esa regla")
		return
	}
	nueva := old
	if !decode(w, r, &nueva) {
		return
	}
	nueva.ID = old.ID

	ch, err := s.App.Store.Channel.Get(ctx, s.App.ChannelID)
	if err != nil {
		failStore(w, err, "el canal")
		return
	}
	today := ch.BroadcastDay(s.Now())

	if soloHoy(r) {
		excepcion := nueva
		excepcion.ID = 0
		excepcion.From, excepcion.To = today, today
		excepcion.Days = onlyDay(today)
		relieved := old.ID
		excepcion.HandsOffTo = &relieved
		excepcion.LastNoticeSent = ""
		if !s.prepareRule(ctx, w, &excepcion) {
			return
		}
		if err := s.App.Store.Rule.Insert(ctx, &excepcion); err != nil {
			failStore(w, err, "guardar la excepción de hoy")
			return
		}
		s.audit(r, "schedule_rule", &excepcion.ID, "solo_hoy",
			describeRule(old), describeRule(excepcion))
		s.App.Recalc()
		writeJSON(w, http.StatusCreated, s.ruleOut(ctx, excepcion, today))
		return
	}

	if !s.prepareRule(ctx, w, &nueva) {
		return
	}
	if err := s.App.Store.Rule.Update(ctx, &nueva); err != nil {
		failStore(w, err, "guardar la regla")
		return
	}
	s.auditDiff(r, "schedule_rule", &nueva.ID, old, nueva)
	s.App.Recalc()
	writeJSON(w, http.StatusOK, s.ruleOut(ctx, nueva, today))
}

func (s *Server) reglasDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	old, err := s.App.Store.Rule.Get(ctx, id)
	if err != nil {
		failStore(w, err, "esa regla")
		return
	}
	if err := s.App.Store.Rule.Delete(ctx, id); err != nil {
		failStore(w, err, "borrar la regla")
		return
	}
	s.audit(r, "schedule_rule", &id, "borrada", describeRule(old), "")
	s.App.Recalc()
	writeJSON(w, http.StatusOK, map[string]any{"borrada": id})
}

func soloHoy(r *http.Request) bool {
	v := strings.TrimSpace(r.URL.Query().Get("solo_hoy"))
	return v == "1" || strings.EqualFold(v, "si") || strings.EqualFold(v, "true")
}

// onlyDay es el patrón de un solo día de la semana.
func onlyDay(d model.Day) model.DayPattern {
	letters := []byte("_______")
	i := d.Weekday()
	letters[i] = model.Letters[i]
	return model.DayPattern(letters)
}

// prepareRule pone los valores por defecto y valida en cristiano. Devuelve
// false cuando ya contestó con el error.
func (s *Server) prepareRule(ctx context.Context, w http.ResponseWriter, rule *model.ScheduleRule) bool {
	rule.ChannelID = s.App.ChannelID
	if rule.Kind == "" {
		rule.Kind = model.RuleNormal
	}
	if rule.SlotMs <= 0 {
		rule.SlotMs = importer.DefaultSlotMs
	}
	if rule.EpisodesPerRun <= 0 {
		rule.EpisodesPerRun = 1
	}
	if rule.From == "" || rule.To == "" {
		fail(w, http.StatusBadRequest, "la regla tiene que decir desde cuándo y hasta cuándo va", "fecha_inicio")
		return false
	}
	if _, err := model.ParseDay(string(rule.From)); err != nil {
		fail(w, http.StatusBadRequest, err.Error(), "fecha_inicio")
		return false
	}
	if _, err := model.ParseDay(string(rule.To)); err != nil {
		fail(w, http.StatusBadRequest, err.Error(), "fecha_fin")
		return false
	}
	if rule.To < rule.From {
		failf(w, http.StatusBadRequest, "fecha_fin",
			"la fecha de fin (%s) es anterior a la de inicio (%s)",
			importer.FormatDate(rule.To), importer.FormatDate(rule.From))
		return false
	}
	if !rule.Days.Valid() {
		failf(w, http.StatusBadRequest, "patron_de_dias",
			"el patrón de días %q no tiene la forma LMMJVSD: siete posiciones, con guion bajo en los días que no",
			string(rule.Days))
		return false
	}
	if rule.At < 0 || rule.At > 1439 {
		fail(w, http.StatusBadRequest, "la hora de la regla se sale del día: va entre 00:00 y 23:59", "hora")
		return false
	}

	switch rule.Kind {
	case model.RuleTimeShift:
		if rule.SourceWindowStart == nil || rule.SourceWindowEnd == nil {
			fail(w, http.StatusBadRequest, "el diferido tiene que decir qué ventana repite", "ventana_origen_inicio")
			return false
		}
	case model.RuleLive:
		if rule.LiveSourceID == nil {
			fail(w, http.StatusBadRequest, "el bloque en vivo tiene que decir de qué fuente viene", "live_source_id")
			return false
		}
	default:
		if rule.TitleID == nil && rule.LiveSourceID == nil {
			fail(w, http.StatusBadRequest, "la regla tiene que decir qué título emite", "title_id")
			return false
		}
	}

	if rule.TitleID != nil {
		title, err := s.App.Store.Title.Get(ctx, *rule.TitleID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				failf(w, http.StatusBadRequest, "title_id", "no encuentro el título %d", *rule.TitleID)
				return false
			}
			failStore(w, err, "leer el título")
			return false
		}
		if !s.titleReady(ctx, title) {
			failf(w, http.StatusBadRequest, "title_id",
				"«%s» todavía no tiene material listo para aire: entra el archivo en la carpeta de contenido y cuando termine de normalizarse la regla se puede guardar",
				title.Name)
			return false
		}
	}
	return true
}

// titleReady dice si un título tiene con qué salir al aire: su propio
// archivo, o al menos un episodio con archivo listo (auditoría B7).
func (s *Server) titleReady(ctx context.Context, t model.Title) bool {
	if t.MediaAssetID != nil && s.assetReady(ctx, *t.MediaAssetID) {
		return true
	}
	eps, err := s.App.Store.Episode.ListByTitle(ctx, t.ID)
	if err != nil {
		return false
	}
	for _, e := range eps {
		if e.MediaAssetID != nil && s.assetReady(ctx, *e.MediaAssetID) {
			return true
		}
	}
	return false
}

func (s *Server) assetReady(ctx context.Context, id int64) bool {
	a, err := s.App.Store.Media.Get(ctx, id)
	return err == nil && a.Ready()
}

// describeRule dice una regla en una línea, para la auditoría.
func describeRule(r model.ScheduleRule) string {
	return strings.Join([]string{
		string(r.Kind),
		importer.DescribePattern(r.Days),
		r.At.String(),
		string(r.From) + "→" + string(r.To),
	}, " · ")
}
