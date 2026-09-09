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
		res.ApplyCatalog(fichas)
	}

	filas := []filaConError{}
	for _, e := range res.RowErrors {
		filas = append(filas, filaConError{
			Line: e.Line, Text: e.Title, Reason: e.Reason,
			Sheet: e.SheetID, Title: e.Title,
		})
	}

	// Los títulos primero: las reglas apuntan a ellos por identificador.
	titleIDs := make([]int64, len(res.Titles))
	creados := 0
	for i := range res.Titles {
		t := res.Titles[i]
		existente, err := s.App.Store.Title.FindByName(ctx, t.Name)
		switch {
		case err == nil:
			titleIDs[i] = existente.ID
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
			creados++
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

	s.App.Recalc()
	writeJSON(w, http.StatusOK, map[string]any{
		"reglas_creadas":          creadas,
		"titulos_creados":         creados,
		"relevos_propuestos":      relevos,
		"repeticiones_propuestas": repeticiones,
		"filas_con_error":         filas,
		"fechas_corridas":         corridas,
		"posibles_duplicados":     res.PossibleDuplicates,
		"avisos":                  res.Notices,
		"resumen":                 res.Summary(),
	})
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

func nombreDeRegla(res importer.Result, i int) string {
	if t := res.TitleFor(i); t != nil {
		return t.Name
	}
	return ""
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
