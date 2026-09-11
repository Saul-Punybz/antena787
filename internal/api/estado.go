package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"antena787/internal/app"
	"antena787/internal/engine"
	"antena787/internal/model"
)

// estadoBody es lo que contesta GET /estado (docs/API.md).
type estadoBody struct {
	Canal      model.Channel `json:"canal"`
	Modo       string        `json:"modo"`
	Ahora      time.Time     `json:"ahora"`
	DiaEmision model.Day     `json:"dia_emision"`
	AlAire     *planRow      `json:"al_aire"`
	Siguiente  *planRow      `json:"siguiente"`
	Alarmas    []app.Alarma  `json:"alarmas"`
	// Salidas es a dónde está saliendo la señal, con el estado que dejó cada
	// driver (F2-46, F2-49). Al aire lo pinta con un punto por salida.
	Salidas      []app.SalidaEnPantalla `json:"salidas"`
	Version      string                 `json:"version"`
	Instalacion  bool                   `json:"necesita_instalacion,omitempty"`
	Completa     bool                   `json:"instalacion_completa"`
	FFmpeg       bool                   `json:"ffmpeg"`
	GuiaGenerada *time.Time             `json:"guia_generada,omitempty"`
	Entraste     bool                   `json:"entraste"`
	// HayAnunciantes enciende la sexta entrada del menú, Anuncios: sin un
	// solo anunciante registrado el menú se queda en cinco (PRD §13, F1-57).
	HayAnunciantes bool `json:"hay_anunciantes"`
	// Acelerador es con qué se está comprimiendo el video de verdad, que no
	// siempre es lo que el canal tiene guardado: cuando la tarjeta deja de
	// responder dos veces en diez minutos, el canal sigue por el procesador
	// y aquí se ve (F2-11). AceleradorPorque es la frase en cristiano de por
	// qué cambió, vacía cuando es lo que se pidió.
	Acelerador       string `json:"acelerador_efectivo"`
	AceleradorPorque string `json:"acelerador_porque,omitempty"`
	// AceleradoresDisponibles es la lista que Ajustes ofrece, con el nombre
	// ya en cristiano y si este ffmpeg de verdad lo trae. Va aquí y no en
	// GET /canal para no cambiarle la forma al canal, que es el modelo pelado.
	AceleradoresDisponibles []aceleradorEnPantalla `json:"aceleradores_disponibles"`
}

// aceleradorEnPantalla es un acelerador como se le ofrece a una persona: la
// clave nunca va sola (PRD §4.3, F1-56).
type aceleradorEnPantalla struct {
	Acelerador  string `json:"acelerador"`
	Nombre      string `json:"nombre"`
	Explicacion string `json:"explicacion"`
	Disponible  bool   `json:"disponible"`
}

// losAceleradores arma la lista una vez por respuesta. Preguntarle a ffmpeg
// qué códecs trae cuesta un proceso, así que solo se hace cuando ffmpeg está
// y solo para los que no son el procesador.
func (s *Server) losAceleradores() []aceleradorEnPantalla {
	out := make([]aceleradorEnPantalla, 0, len(engine.Aceleradores()))
	for _, ac := range engine.Aceleradores() {
		hay := true
		if s.App.FFmpeg != "" {
			hay = ac.Disponible(s.App.FFmpeg)
		}
		out = append(out, aceleradorEnPantalla{
			Acelerador:  string(ac),
			Nombre:      ac.Nombre(),
			Explicacion: ac.Explicacion(),
			Disponible:  hay,
		})
	}
	return out
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
		Instalacion: !hasPIN || s.setting(r, app.KeyInstallDone) != "si",
		Completa:    s.setting(r, app.KeyInstallDone) == "si",
		FFmpeg:      s.App.FFmpeg != "" && s.App.FFprobe != "",
	}
	body.HayAnunciantes = s.hayAnunciantes(ctx)
	ac, porque := s.App.AceleradorEnCurso()
	body.Acelerador, body.AceleradorPorque = string(ac), porque
	body.AceleradoresDisponibles = s.losAceleradores()
	body.Salidas = []app.SalidaEnPantalla{}
	if al := s.App.AlarmaSubtitulos(ctx); al != nil {
		body.Alarmas = append(body.Alarmas, *al)
	}
	if body.Alarmas == nil {
		body.Alarmas = []app.Alarma{}
	}
	if _, at := s.App.Guide(); !at.IsZero() {
		body.GuiaGenerada = &at
	}
	_, body.Entraste = s.author(r)

	// El plan de ahora mismo y a dónde sale la señal solo se enseñan a quien
	// entró: la dirección del multiplexor es información del canal, no una
	// tarjeta de presentación.
	if body.Entraste {
		if salidas := s.App.SalidasDelCanal(ctx); salidas != nil {
			body.Salidas = salidas
		}
		items, err := s.App.Store.Plan.ListRange(ctx, s.App.ChannelID, now.Add(-6*time.Hour), now.Add(12*time.Hour))
		if err == nil {
			for i := range items {
				it := items[i]
				switch {
				case !it.PlannedAt.After(now) && it.End().After(now):
					body.AlAire = s.nombrar(ctx, ch.Location(), &items[i])
				case it.PlannedAt.After(now) && body.Siguiente == nil:
					body.Siguiente = s.nombrar(ctx, ch.Location(), &items[i])
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, body)
}

// hayAnunciantes dice si hay algún anunciante registrado en el canal. Se
// pregunta directamente a la base: `advertiser` todavía no tiene repositorio
// propio —el alta de anunciantes es F4— y una consulta de una línea no es
// razón para inventarle uno.
func (s *Server) hayAnunciantes(ctx context.Context) bool {
	var hay bool
	err := s.App.Store.DB().QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM advertiser WHERE channel_id = ?)`, s.App.ChannelID).Scan(&hay)
	if err != nil {
		return false
	}
	return hay
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
	// El modo no se cambia por aquí. Guardar el nombre del canal no puede
	// encender ni apagar un transmisor de paso: para eso están
	// POST /canal/al-aire y POST /canal/a-sombra, que comprueban antes y
	// dejan constancia (F2-118).
	nuevo.Mode = old.Mode

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

// Tapado es lo que se enseña en lugar de un secreto guardado. Volver a
// mandarlo tal cual en un PUT no cambia nada: el valor de verdad se queda
// como estaba, así que la pantalla de Ajustes puede leer y guardar sin
// borrar por descuido la clave que no ve.
const Tapado = "••••••"

// secretos son los ajustes cuyo valor no vuelve a salir de la máquina. La
// tabla settings ya esconde sola los que llevan «token», «secreto» o
// «contraseña» en el nombre; estos tres se tapan aquí porque no se llaman
// así y aun así son secretos.
var secretos = []string{app.KeyTMDBKey, app.KeyTelegramToken, app.KeySMTPPass}

func (s *Server) ajustesGet(w http.ResponseWriter, r *http.Request) {
	all, err := s.App.Store.Settings.GetAll(r.Context())
	if err != nil {
		failStore(w, err, "leer los ajustes")
		return
	}
	writeJSON(w, http.StatusOK, s.conLosDeFabrica(r, s.tapar(r, all)))
}

// conLosDeFabrica añade los ajustes que la pantalla enseña siempre y que
// pueden no estar guardados todavía. Ajustes tiene que poder pintar el umbral
// de silencio y negro vigente desde la primera vez que se abre (PRD §13): si
// no se manda, la tarjeta dice «—» y el número que de verdad gobierna el aire
// queda invisible.
func (s *Server) conLosDeFabrica(r *http.Request, out map[string]string) map[string]string {
	silencio, negro, devuelve := s.App.UmbralesDeVigilancia(r.Context())
	poner := func(clave, valor string) {
		if _, hay := out[clave]; !hay {
			out[clave] = valor
		}
	}
	poner(app.KeySilencioUmbral, strconv.Itoa(silencio))
	poner(app.KeyNegroUmbral, strconv.Itoa(negro))
	poner(app.KeySilencioDevuelveControl, siONo(devuelve))
	return out
}

// siONo es cómo viajan los ajustes de sí o no en esta API: «si» o «no», sin
// tilde, igual que `fichas_en_linea` e `instalacion.ve_barras`.
func siONo(v bool) string {
	if v {
		return "si"
	}
	return "no"
}

// tapar sustituye el valor de los secretos por Tapado, y añade los que la
// tabla settings no devuelve para que la pantalla sepa que están puestos.
func (s *Server) tapar(r *http.Request, all map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range all {
		out[k] = v
	}
	for _, clave := range secretos {
		if _, hay := out[clave]; hay {
			out[clave] = Tapado
			continue
		}
		if s.setting(r, clave) != "" {
			out[clave] = Tapado
		}
	}
	return out
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
		if k == app.KeySilencioUmbral || k == app.KeyNegroUmbral {
			if _, err := app.UmbralValido(v); err != nil {
				failf(w, http.StatusBadRequest, k, "%s", err.Error())
				return
			}
		}
		if k == app.KeySilencioDevuelveControl && v != "si" && v != "no" {
			failf(w, http.StatusBadRequest, k,
				"«avisa y devuelve el control» solo entiende %q o %q", "si", "no")
			return
		}
		if k == app.KeySubtitulosEstado && !app.SubtitulosEstadoValido(v) {
			failf(w, http.StatusBadRequest, k,
				"el ajuste de subtítulos solo entiende %q, %q o %q",
				app.SubtitulosObligada, app.SubtitulosExenta, app.SubtitulosNoSe)
			return
		}
		// Devolver tal cual lo que se enseñó tapado no es cambiarlo.
		if v == Tapado {
			continue
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
	if _, cambió := body[app.KeyGuideHTTP]; cambió {
		s.App.Recalc()
	}
	if _, cambió := body[app.KeyGuidePMCPHTTP]; cambió {
		s.App.Recalc()
	}
	all, err := s.App.Store.Settings.GetAll(ctx)
	if err != nil {
		failStore(w, err, "leer los ajustes")
		return
	}
	writeJSON(w, http.StatusOK, s.conLosDeFabrica(r, s.tapar(r, all)))
}
