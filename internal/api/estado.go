package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"antena787/internal/app"
	"antena787/internal/model"
)

// estadoBody es lo que contesta GET /estado (docs/API.md).
type estadoBody struct {
	Canal        model.Channel   `json:"canal"`
	Modo         string          `json:"modo"`
	Ahora        time.Time       `json:"ahora"`
	DiaEmision   model.Day       `json:"dia_emision"`
	AlAire       *model.PlanItem `json:"al_aire"`
	Siguiente    *model.PlanItem `json:"siguiente"`
	Alarmas      []string        `json:"alarmas"`
	Version      string          `json:"version"`
	Instalacion  bool            `json:"necesita_instalacion,omitempty"`
	Completa     bool            `json:"instalacion_completa"`
	FFmpeg       bool            `json:"ffmpeg"`
	GuiaGenerada *time.Time      `json:"guia_generada,omitempty"`
	Entraste     bool            `json:"entraste"`
}

// estado es la única ruta que contesta sin clave: es la que le dice a la
// interfaz si hay que abrir el asistente.
func (s *Server) estado(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	hasPIN, err := s.App.Store.Settings.HasPIN(ctx)
	if err != nil {
		fail(w, http.StatusInternalServerError, "no se pudo leer la clave de la estación: "+err.Error(), "")
		return
	}
	ch, err := s.App.Store.Channel.Get(ctx, s.App.ChannelID)
	if err != nil {
		failStore(w, err, "el canal")
		return
	}
	now := s.Now()
	body := estadoBody{
		Canal:       ch,
		Modo:        ch.Mode,
		Ahora:       now,
		DiaEmision:  ch.BroadcastDay(now),
		Alarmas:     s.App.Alarms(),
		Version:     s.App.Version,
		Instalacion: !hasPIN,
		Completa:    s.setting(r, app.KeyInstallDone) == "si",
		FFmpeg:      s.App.FFmpeg != "" && s.App.FFprobe != "",
	}
	if body.Alarmas == nil {
		body.Alarmas = []string{}
	}
	if _, at := s.App.Guide(); !at.IsZero() {
		body.GuiaGenerada = &at
	}
	_, body.Entraste = s.author(r)

	// El plan de ahora mismo solo se enseña a quien entró: es información
	// del canal, no una tarjeta de presentación.
	if body.Entraste {
		items, err := s.App.Store.Plan.ListRange(ctx, s.App.ChannelID, now.Add(-6*time.Hour), now.Add(12*time.Hour))
		if err == nil {
			for i := range items {
				it := items[i]
				switch {
				case !it.PlannedAt.After(now) && it.End().After(now):
					body.AlAire = &items[i]
				case it.PlannedAt.After(now) && body.Siguiente == nil:
					body.Siguiente = &items[i]
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, body)
}

// setting lee un ajuste sin ruido.
func (s *Server) setting(r *http.Request, key string) string {
	v, err := s.App.Store.Settings.Get(r.Context(), key)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(v)
}

// ── el canal ──────────────────────────────────────────────────────────

func (s *Server) canalGet(w http.ResponseWriter, r *http.Request) {
	ch, err := s.App.Store.Channel.Get(r.Context(), s.App.ChannelID)
	if err != nil {
		failStore(w, err, "el canal")
		return
	}
	writeJSON(w, http.StatusOK, ch)
}

// canalPut cambia el canal. Cambiar la zona horaria o la hora de inicio del
// día de emisión mueve todo el plan, así que recalcula.
func (s *Server) canalPut(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	old, err := s.App.Store.Channel.Get(ctx, s.App.ChannelID)
	if err != nil {
		failStore(w, err, "el canal")
		return
	}
	nuevo := old
	if !decode(w, r, &nuevo) {
		return
	}
	nuevo.ID = old.ID

	if _, err := time.LoadLocation(nuevo.TimeZone); err != nil {
		failf(w, http.StatusBadRequest, "zona_horaria",
			"no conozco la zona horaria %q: se escribe como America/Puerto_Rico", nuevo.TimeZone)
		return
	}
	if nuevo.BroadcastDayAt < 0 || nuevo.BroadcastDayAt > 1439 {
		fail(w, http.StatusBadRequest, "la hora de inicio del día de emisión tiene que estar dentro del día", "hora_inicio_dia_emision")
		return
	}
	// En F1 no hay motor: el canal se queda en sombra dígase lo que se diga.
	nuevo.Mode = "sombra"

	if err := s.App.Store.Channel.Update(ctx, nuevo); err != nil {
		failStore(w, err, "guardar el canal")
		return
	}
	s.auditDiff(r, "channel", &nuevo.ID, old, nuevo)

	if nuevo.TimeZone != old.TimeZone || nuevo.BroadcastDayAt != old.BroadcastDayAt {
		s.App.Recalc()
	}
	writeJSON(w, http.StatusOK, nuevo)
}

// auditDiff anota campo por campo lo que cambió. Es lo que hace que
// "¿quién movió esto?" tenga respuesta.
func (s *Server) auditDiff(r *http.Request, entity string, id *int64, before, after any) {
	b := toMap(before)
	a := toMap(after)
	for k, av := range a {
		bv, had := b[k]
		if had && string(bv) == string(av) {
			continue
		}
		s.audit(r, entity, id, k, trimJSON(bv), trimJSON(av))
	}
}

func toMap(v any) map[string]json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	out := map[string]json.RawMessage{}
	_ = json.Unmarshal(raw, &out)
	return out
}

// trimJSON deja el valor como lo leería una persona: sin las comillas de una
// cadena JSON.
func trimJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(raw)
}

// ── los ajustes ───────────────────────────────────────────────────────

func (s *Server) ajustesGet(w http.ResponseWriter, r *http.Request) {
	all, err := s.App.Store.Settings.GetAll(r.Context())
	if err != nil {
		failStore(w, err, "leer los ajustes")
		return
	}
	writeJSON(w, http.StatusOK, all)
}

// ajustesPut guarda un mapa clave→valor. La clave de estación no se toca por
// aquí: para eso está el paso 1 del asistente.
func (s *Server) ajustesPut(w http.ResponseWriter, r *http.Request) {
	var body map[string]string
	if !decode(w, r, &body) {
		return
	}
	ctx := r.Context()
	for k, v := range body {
		if strings.HasPrefix(strings.ToLower(k), "clave_estacion.") || strings.Contains(strings.ToLower(k), "secreto") {
			failf(w, http.StatusBadRequest, k, "el ajuste %q no se cambia desde aquí", k)
			return
		}
		before, _ := s.App.Store.Settings.Get(ctx, k)
		if err := s.App.Store.Settings.Set(ctx, k, v); err != nil {
			failStore(w, err, "guardar los ajustes")
			return
		}
		s.audit(r, "settings", nil, k, before, v)
	}
	if _, cambió := body[app.KeyGuidePath]; cambió {
		s.App.Recalc()
	}
	all, err := s.App.Store.Settings.GetAll(ctx)
	if err != nil {
		failStore(w, err, "leer los ajustes")
		return
	}
	writeJSON(w, http.StatusOK, all)
}
