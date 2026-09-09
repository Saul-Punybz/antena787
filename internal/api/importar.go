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

// filaConError es una fila de la hoja que no se pudo importar, dicha para
// una persona: qué fila era, de qué programa, y por qué (auditoría B11).
type filaConError struct {
	Line   int    `json:"fila"`
	Text   string `json:"texto"`
	Reason string `json:"motivo"`
	Sheet  string `json:"id_hoja,omitempty"`
	Title  string `json:"titulo,omitempty"`
}

// relevoPropuesto es un relevo inferido, ya con los identificadores de la
// base para que confirmarlo sea un clic.
type relevoPropuesto struct {
	Expires  int64  `json:"regla_que_vence"`
	Relieves int64  `json:"regla_que_releva"`
	FromID   string `json:"id_hoja_vence,omitempty"`
	ToID     string `json:"id_hoja_releva,omitempty"`
	Clock    string `json:"hora"`
	Text     string `json:"texto"`
}

// repeticionPropuesta es un segundo pase inferido.
type repeticionPropuesta struct {
	Primary int64  `json:"regla_primaria"`
	Repeat  int64  `json:"regla_que_repite"`
	Text    string `json:"texto"`
}

// fechaCorrida es una fecha que se corrió un día atrás porque la regla es de
// madrugada y el día de emisión empieza a las 6 (auditoría B1).
type fechaCorrida struct {
	Rule   int64  `json:"regla,omitempty"`
	Line   int    `json:"fila"`
	Title  string `json:"titulo"`
	Clock  string `json:"hora"`
	Before string `json:"antes"`
	After  string `json:"despues"`
	Text   string `json:"texto"`
}

// importarHoja es "pega la hoja de Excel y ya". Nunca rechaza la hoja
// entera: importa lo que cuadra y lista fila por fila lo que no.
func (s *Server) importarHoja(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Texto string `json:"texto"`
	}
	if !decode(w, r, &body) {
		return
	}
	if strings.TrimSpace(body.Texto) == "" {
		fail(w, http.StatusBadRequest, "no llegó nada que importar: pega la hoja completa", "texto")
		return
	}
	ctx := r.Context()
	ch, err := s.App.Store.Channel.Get(ctx, s.App.ChannelID)
	if err != nil {
		failStore(w, err, "el canal")
		return
	}

	// Las tres tablas que puede traer el pegado: reglas, catálogo y parrilla.
	var reglas, catalogo importer.Sheet
	var grid *importer.Grid
	for _, sh := range importer.ParseAll(body.Texto) {
		switch sh.Kind {
		case importer.KindRules:
			if len(reglas.Rows) == 0 {
				reglas = sh
			}
		case importer.KindCatalog:
			if len(catalogo.Rows) == 0 {
				catalogo = sh
			}
		case importer.KindGrid:
			if grid == nil {
				grid = sh.Grid
			}
		}
	}
	if reglas.Kind != importer.KindRules {
		fail(w, http.StatusBadRequest,
			"no reconocí ninguna tabla de reglas ahí: espero columnas como Id, Video, Duración, Días, Horas, Fec Ini y Fec Final", "texto")
		return
	}

	// Las fuentes en vivo que ya existen se importan como bloque en vivo, y
	// su columna "Duración" se ignora con aviso (auditoría B11).
	liveByName := map[string]int64{}
	var liveNames []string
	if lives, err := s.App.Store.Live.List(ctx, s.App.ChannelID); err == nil {
		for _, l := range lives {
			liveNames = append(liveNames, l.Name)
			liveByName[strings.ToLower(l.Name)] = l.ID
		}
	}
	if len(liveNames) == 0 {
		liveNames = []string{"RadioOnce Live!"}
	}

	opts := []importer.Option{importer.WithLiveNames(liveNames...)}
	if grid != nil {
		opts = append(opts, importer.WithGrid(grid))
	}
	res := importer.Rules(reglas, ch, opts...)

	if catalogo.Kind == importer.KindCatalog {
		fichas, errs := importer.Catalog(catalogo)
		res.RowErrors = append(res.RowErrors, errs...)
		// Los alias van primero: lo que una persona ya emparejó en otra hoja
		// no se vuelve a preguntar (F1-66).
		res.ApplyCatalogCon(fichas, s.aliasDelCanal(ctx))
	}

	filas := []filaConError{}
	for _, e := range res.RowErrors {
		filas = append(filas, filaConError{
			Line: e.Line, Text: e.Title, Reason: e.Reason,
			Sheet: e.SheetID, Title: e.Title,
		})
	}

	// Lo que quedó por emparejar, por índice de título: el importador ya dijo
	// cuáles son y a qué fichas se parecían (F1-64, F1-65).
	sinPareja := map[int]*importer.Unmatched{}
	if res.Match != nil {
		for k := range res.Match.Unmatched {
			u := &res.Match.Unmatched[k]
			if idx, ok := res.Match.TitleIndex[u.Name]; ok && res.Match.EsProvisional(idx) {
				sinPareja[idx] = u
			}
		}
	}
	porNombre := map[string]int{}
	for i := range res.Titles {
		if _, ya := porNombre[res.Titles[i].Name]; !ya {
			porNombre[res.Titles[i].Name] = i
		}
	}

	// Los títulos primero: las reglas apuntan a ellos por identificador.
	titleIDs := make([]int64, len(res.Titles))
	pendientes := make([]bool, len(res.Titles))
	creados := 0
	for i := range res.Titles {
		t := res.Titles[i]
		u, provisional := sinPareja[i]
		if provisional {
			// Nace marcado: nadie crea una ficha nueva callado. Los
			// candidatos van con él para que la pantalla los ofrezca.
			t.PendienteEmparejar = true
			t.Candidatos = idsDeCandidatos(u, porNombre, titleIDs)
		}
		// Un nombre que ya se emparejó a mano va derecho a su ficha, aunque
		// la hoja de hoy venga sin catálogo detrás.
		if id, ok := s.tituloDelAlias(ctx, t.Name); ok {
			titleIDs[i] = id
			continue
		}
		existente, err := s.App.Store.Title.FindByName(ctx, t.Name)
		switch {
		case err == nil:
			titleIDs[i] = existente.ID
			pendientes[i] = existente.PendienteEmparejar
		case errors.Is(err, store.ErrNotFound):
			t.ID = 0
			if err := s.App.Store.Title.Insert(ctx, &t); err != nil {
				filas = append(filas, filaConError{
					Text: t.Name, Title: t.Name,
					Reason: "no se pudo guardar la ficha: " + err.Error(),
				})
				continue
			}
			titleIDs[i] = t.ID
			pendientes[i] = t.PendienteEmparejar
			// Un título por emparejar todavía no es una ficha del catálogo:
			// no se cuenta como título creado.
			if !provisional {
				creados++
			}
			s.audit(r, "title", &t.ID, "importado", "", t.Name)
		default:
			failStore(w, err, "leer la biblioteca")
			return
		}
	}

	// Y ahora las reglas, una a una: la que no entra se cuenta y las demás
	// siguen. Cada Insert es una transacción del store, así que una regla que
	// falla no deja nada a medias.
	ruleIDs := make([]int64, len(res.Rules))
	for i := range res.Rules {
		rule := res.Rules[i]
		rule.ID = 0
		rule.ChannelID = s.App.ChannelID
		if t := res.TitleIndex[i]; t >= 0 && t < len(titleIDs) && titleIDs[t] != 0 {
			id := titleIDs[t]
			rule.TitleID = &id
		}
		// Una fila de fuente en vivo sin fuente detrás es una regla que el
		// resolver no puede programar. Se crea la fuente con lo que se sabe
		// —el nombre— y el asistente termina de configurarla después.
		if rule.Kind == model.RuleLive && rule.LiveSourceID == nil {
			if id, err := s.liveSourceFor(ctx, nombreDeRegla(res, i), liveByName); err == nil && id != 0 {
				rule.LiveSourceID = &id
			}
		}
		if err := s.App.Store.Rule.Insert(ctx, &rule); err != nil {
			filas = append(filas, filaConError{
				Text:   nombreDeRegla(res, i),
				Sheet:  sheetID(res, i),
				Title:  nombreDeRegla(res, i),
				Reason: plainRuleError(err),
			})
			continue
		}
		ruleIDs[i] = rule.ID
		res.Rules[i] = rule
		s.audit(r, "schedule_rule", &rule.ID, "importada", "", describeRule(rule))
	}

	creadas := 0
	for _, id := range ruleIDs {
		if id != 0 {
			creadas++
		}
	}

	relevos := []relevoPropuesto{}
	for _, h := range res.Handoffs {
		if !dentro(ruleIDs, h.From) || !dentro(ruleIDs, h.To) {
			continue
		}
		relevos = append(relevos, relevoPropuesto{
			Expires: ruleIDs[h.From], Relieves: ruleIDs[h.To],
			FromID: h.FromID, ToID: h.ToID,
			Clock: h.At.String(), Text: h.Text,
		})
	}
	repeticiones := []repeticionPropuesta{}
	for _, rep := range res.Repeats {
		if !dentro(ruleIDs, rep.Primary) || !dentro(ruleIDs, rep.Repeat) {
			continue
		}
		repeticiones = append(repeticiones, repeticionPropuesta{
			Primary: ruleIDs[rep.Primary], Repeat: ruleIDs[rep.Repeat], Text: rep.Text,
		})
	}
	corridas := []fechaCorrida{}
	for _, d := range res.DatesShifted {
		f := fechaCorrida{
			Line: d.Line, Title: d.Title, Clock: d.At.String(),
			Before: string(d.FromBefore) + "→" + string(d.ToBefore),
			After:  string(d.FromAfter) + "→" + string(d.ToAfter),
			Text:   d.Text,
		}
		if dentro(ruleIDs, d.Rule) {
			f.Rule = ruleIDs[d.Rule]
		}
		corridas = append(corridas, f)
	}

	// Y lo que no se pudo emparejar solo, con su identificador ya guardado:
	// desde aquí la pantalla de Reglas lo resuelve de un clic (F1-64).
	sinEmparejar := []tituloSinEmparejar{}
	if res.Match != nil {
		for k := range res.Match.Unmatched {
			u := &res.Match.Unmatched[k]
			idx, ok := res.Match.TitleIndex[u.Name]
			if !ok || idx < 0 || idx >= len(titleIDs) || titleIDs[idx] == 0 || !pendientes[idx] {
				continue
			}
			sinEmparejar = append(sinEmparejar, tituloSinEmparejar{
				ID:         titleIDs[idx],
				Nombre:     u.Name,
				Texto:      u.Text,
				Candidatos: candidatosDeLaHoja(u, porNombre, titleIDs),
				Reglas:     len(u.Rules),
				Franjas:    sinNil(u.Slots),
			})
		}
	}
	avisos := res.Notices
	if avisos == nil {
		avisos = []importer.Notice{}
	}
	duplicados := res.PossibleDuplicates
	if duplicados == nil {
		duplicados = []importer.DuplicateWarning{}
	}

	s.App.Recalc()
	s.App.RefreshPendientes(ctx)
	writeJSON(w, http.StatusOK, map[string]any{
		"reglas_creadas":          creadas,
		"titulos_creados":         creados,
		"relevos_propuestos":      relevos,
		"repeticiones_propuestas": repeticiones,
		"filas_con_error":         filas,
		"fechas_corridas":         corridas,
		"posibles_duplicados":     duplicados,
		"avisos":                  avisos,
		"titulos_sin_emparejar":   sinEmparejar,
		"resumen":                 res.Summary(),
	})
}

// aliasDelCanal traduce los alias guardados a lo que entiende el importador:
// el nombre de la hoja y el nombre de la ficha a la que va. Los que apuntan a
// una ficha que ya no existe se quedan fuera, sin ruido.
func (s *Server) aliasDelCanal(ctx context.Context) []importer.Alias {
	guardados, err := s.App.Store.Alias.ListByChannel(ctx, s.App.ChannelID)
	if err != nil || len(guardados) == 0 {
		return nil
	}
	nombres := map[int64]string{}
	if todos, err := s.App.Store.Title.List(ctx); err == nil {
		for _, t := range todos {
			nombres[t.ID] = t.Name
		}
	}
	out := make([]importer.Alias, 0, len(guardados))
	for _, a := range guardados {
		ficha := nombres[a.TitleID]
		if ficha == "" {
			continue
		}
		out = append(out, importer.Alias{Nombre: a.Alias, Titulo: ficha})
	}
	return out
}

// tituloDelAlias dice a qué ficha va ese nombre de la hoja, si ya se aprendió
// y la ficha sigue estando.
func (s *Server) tituloDelAlias(ctx context.Context, nombre string) (int64, bool) {
	id, ok, err := s.App.Store.Alias.Resolve(ctx, s.App.ChannelID, nombre)
	if err != nil || !ok {
		return 0, false
	}
	if _, err := s.App.Store.Title.Get(ctx, id); err != nil {
		return 0, false
	}
	return id, true
}

// idsDeCandidatos traduce los nombres de los candidatos a identificadores de
// la base. Se guardan en la ficha provisional para que la pantalla los ofrezca
// sin volver a correr el emparejamiento (F1-65).
func idsDeCandidatos(u *importer.Unmatched, porNombre map[string]int, ids []int64) []int64 {
	if u == nil || len(u.Candidates) == 0 {
		return nil
	}
	out := make([]int64, 0, len(u.Candidates))
	for _, nombre := range u.Candidates {
		if idx, ok := porNombre[nombre]; ok && idx < len(ids) && ids[idx] != 0 {
			out = append(out, ids[idx])
		}
	}
	return out
}

// candidatosDeLaHoja arma los candidatos como los pinta la pantalla: la
// ficha, su nombre y cuánto se parecía.
func candidatosDeLaHoja(u *importer.Unmatched, porNombre map[string]int, ids []int64) []candidatoDeTitulo {
	out := []candidatoDeTitulo{}
	if u == nil {
		return out
	}
	for i, nombre := range u.Candidates {
		idx, ok := porNombre[nombre]
		if !ok || idx >= len(ids) || ids[idx] == 0 {
			continue
		}
		c := candidatoDeTitulo{ID: ids[idx], Nombre: nombre}
		if i < len(u.CandidateScores) {
			c.Puntuacion = u.CandidateScores[i]
		}
		out = append(out, c)
	}
	return out
}

// sinNil deja una lista de textos lista para JSON: nunca null.
func sinNil(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

// liveSourceFor devuelve la fuente en vivo de ese nombre, creándola la
// primera vez. Nace apagada y sin punto de escucha: el nombre es lo único
// que trae la hoja, y una entrada en vivo no se abre sin contraseña
// (PRD §19). Configurarla es un paso aparte, a mano.
func (s *Server) liveSourceFor(ctx context.Context, name string, cache map[string]int64) (int64, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, nil
	}
	if id, ok := cache[strings.ToLower(name)]; ok {
		return id, nil
	}
	live := model.LiveSource{
		ChannelID: s.App.ChannelID,
		Name:      name,
		Kind:      "srt",
		AudioOnly: true,
	}
	if err := s.App.Store.Live.Upsert(ctx, &live); err != nil {
		return 0, err
	}
	cache[strings.ToLower(name)] = live.ID
	return live.ID, nil
}

func dentro(ids []int64, i int) bool { return i >= 0 && i < len(ids) && ids[i] != 0 }

// nombreDeRegla es cómo se llama la fila i: el nombre del título si lo tiene
// y, si no —una fuente en vivo no es un título—, el que traía la hoja. Sin
// esto la fila de «RadioOnce Live!» se quedaría sin fuente detrás, porque su
// título ya no existe (F1-67).
func nombreDeRegla(res importer.Result, i int) string {
	return res.NameFor(i)
}

func sheetID(res importer.Result, i int) string {
	if i >= 0 && i < len(res.SheetIDs) {
		return res.SheetIDs[i]
	}
	return ""
}

// plainRuleError deja el error del store en el idioma de la gente.
func plainRuleError(err error) string {
	switch {
	case errors.Is(err, store.ErrDates):
		return store.ErrDates.Error()
	case errors.Is(err, store.ErrOverlap):
		return store.ErrOverlap.Error()
	}
	return "no se pudo guardar la regla: " + err.Error()
}

// ── confirmar relevos ─────────────────────────────────────────────────

// confirmarRelevos aplica lo que el importador solo propuso. El importador
// nunca lo hace solo: quien programa el canal decide si Zoids releva a Magic
// Knight o si son dos cosas distintas (auditoría B10).
func (s *Server) confirmarRelevos(w http.ResponseWriter, r *http.Request) {
	var body []struct {
		Rule     int64  `json:"regla"`
		HandsOff *int64 `json:"releva_a"`
		Repeats  *int64 `json:"repite_a"`
	}
	if !decode(w, r, &body) {
		return
	}
	if len(body) == 0 {
		fail(w, http.StatusBadRequest, "no llegó ningún relevo que confirmar", "")
		return
	}
	ctx := r.Context()
	aplicados := []int64{}
	for _, item := range body {
		rule, err := s.App.Store.Rule.Get(ctx, item.Rule)
		if err != nil {
			failf(w, http.StatusBadRequest, "regla", "no encuentro la regla %d", item.Rule)
			return
		}
		old := rule
		if item.HandsOff != nil {
			if err := s.exists(ctx, *item.HandsOff); err != nil {
				failf(w, http.StatusBadRequest, "releva_a", "no encuentro la regla %d, la que iba a relevar", *item.HandsOff)
				return
			}
			v := *item.HandsOff
			rule.HandsOffTo = &v
		}
		if item.Repeats != nil {
			if err := s.exists(ctx, *item.Repeats); err != nil {
				failf(w, http.StatusBadRequest, "repite_a", "no encuentro la regla %d, la que iba a repetir", *item.Repeats)
				return
			}
			v := *item.Repeats
			rule.RepeatsOf = &v
		}
		if err := s.App.Store.Rule.Update(ctx, &rule); err != nil {
			failStore(w, err, "guardar el relevo")
			return
		}
		s.auditDiff(r, "schedule_rule", &rule.ID, old, rule)
		aplicados = append(aplicados, rule.ID)
	}
	s.App.Recalc()
	writeJSON(w, http.StatusOK, map[string]any{"confirmados": aplicados})
}

// exists dice si esa regla está en la base; es lo que separa un relevo
// confirmado de un identificador copiado a mano que no apunta a nada.
func (s *Server) exists(ctx context.Context, id int64) error {
	_, err := s.App.Store.Rule.Get(ctx, id)
	return err
}
