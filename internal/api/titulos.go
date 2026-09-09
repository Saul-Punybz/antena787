package api

// Emparejar títulos (F1-64 a F1-67).
//
// El importador nunca rechaza una hoja: si un nombre no cuadra con ninguna
// ficha del catálogo, la regla entra igual y el título queda «por emparejar».
// Aquí está lo que ve y lo que decide la persona:
//
//   - GET  /titulos/sin-emparejar   la lista, con sus candidatos y sus franjas
//   - GET  /titulos/buscar?q=       el catálogo, para elegir la ficha buena
//   - POST /titulos/{id}/emparejar  usar esta ficha · es un título propio ·
//     no es un programa
//
// El nombre del catálogo manda siempre, y lo que se empareja una vez queda
// aprendido: la próxima hoja que traiga ese nombre se empareja sola (F1-66).

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"antena787/internal/importer"
	"antena787/internal/model"
	"antena787/internal/store"
)

// CandidatosEnLaBusqueda es cuántas fichas devuelve la búsqueda del
// emparejador. Es una lista para elegir con el dedo, no un listado completo.
const CandidatosEnLaBusqueda = 20

// candidatoDeTitulo es una ficha del catálogo que se parecía al nombre de la
// hoja, con cuánto se parecía (de 0 a 1).
type candidatoDeTitulo struct {
	ID         int64   `json:"id"`
	Nombre     string  `json:"nombre"`
	Puntuacion float64 `json:"puntuacion"`
}

// tituloSinEmparejar es un nombre de la hoja que se quedó sin ficha, con todo
// lo que hace falta para reconocerlo: en cuántas reglas sale y en qué franjas.
type tituloSinEmparejar struct {
	ID         int64               `json:"id"`
	Nombre     string              `json:"nombre"`
	Texto      string              `json:"texto"`
	Candidatos []candidatoDeTitulo `json:"candidatos"`
	Reglas     int                 `json:"reglas"`
	Franjas    []string            `json:"franjas"`
}

// tituloDelCatalogo es una ficha como la enseña la búsqueda.
type tituloDelCatalogo struct {
	ID     int64  `json:"id"`
	Nombre string `json:"nombre"`
	Tipo   string `json:"tipo"`
}

// ── la lista ──────────────────────────────────────────────────────────

// titulosSinEmparejar devuelve los títulos que el importador dejó por
// emparejar, con sus candidatos y las franjas donde salen. Es lo mismo que
// devolvió la importación, pero leído de la base: la pantalla de Reglas se
// puede abrir mañana y seguir donde se quedó.
func (s *Server) titulosSinEmparejar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pendientes, err := s.App.Store.Title.ListPendientes(ctx, s.App.ChannelID)
	if err != nil {
		failStore(w, err, "leer los títulos por emparejar")
		return
	}
	reglas, err := s.App.Store.Rule.ListAll(ctx, s.App.ChannelID)
	if err != nil {
		failStore(w, err, "leer las reglas")
		return
	}

	out := make([]tituloSinEmparejar, 0, len(pendientes))
	for _, t := range pendientes {
		uno := tituloSinEmparejar{
			ID:         t.ID,
			Nombre:     t.Name,
			Candidatos: s.candidatosDe(ctx, t),
			Franjas:    []string{},
		}
		for _, regla := range reglas {
			if regla.TitleID == nil || *regla.TitleID != t.ID {
				continue
			}
			uno.Reglas++
			uno.Franjas = append(uno.Franjas, fmt.Sprintf("%s a las %s",
				importer.DescribePattern(regla.Days), importer.FormatClock(regla.At)))
		}
		uno.Texto = textoSinPareja(uno)
		out = append(out, uno)
	}
	writeJSON(w, http.StatusOK, out)
}

// textoSinPareja dice en una frase por qué ese título está esperando: o no se
// parece a ninguna ficha, o se parece igual a varias y no se quiso adivinar.
// Es la misma frase que dio el importador el día de la hoja.
func textoSinPareja(t tituloSinEmparejar) string {
	if len(t.Candidatos) == 0 {
		return fmt.Sprintf("«%s» no tiene ficha en el catálogo", t.Nombre)
	}
	nombres := make([]string, len(t.Candidatos))
	for i, c := range t.Candidatos {
		nombres[i] = "«" + c.Nombre + "»"
	}
	return fmt.Sprintf("«%s» se parece igual a %s: no supe cuál es y no quise adivinar",
		t.Nombre, enLista(nombres))
}

// enLista junta nombres como se dicen en voz alta: «a», «b» y «c».
func enLista(nombres []string) string {
	switch len(nombres) {
	case 0:
		return ""
	case 1:
		return nombres[0]
	}
	return strings.Join(nombres[:len(nombres)-1], ", ") + " y " + nombres[len(nombres)-1]
}

// candidatosDe lee las fichas que el importador dejó apuntadas en el título
// provisional y vuelve a medir cuánto se parecen, con el mismo parecido del
// importador: así la pantalla enseña el mismo porcentaje el primer día y el
// décimo.
func (s *Server) candidatosDe(ctx context.Context, t model.Title) []candidatoDeTitulo {
	out := []candidatoDeTitulo{}
	if len(t.Candidatos) == 0 {
		return out
	}
	fichas := make([]model.Title, 0, len(t.Candidatos))
	for _, id := range t.Candidatos {
		ficha, err := s.App.Store.Title.Get(ctx, id)
		if err != nil {
			continue // la ficha se borró: el candidato ya no vale
		}
		fichas = append(fichas, ficha)
	}
	puntos := puntuacionesDe(t.Name, fichas)
	for _, ficha := range fichas {
		out = append(out, candidatoDeTitulo{
			ID: ficha.ID, Nombre: ficha.Name, Puntuacion: puntos[ficha.Name],
		})
	}
	return out
}

// puntuacionesDe vuelve a pasar el nombre de la hoja por el emparejador para
// saber cuánto se parece a cada ficha. Se usa el del importador y no una
// copia: el número que ve la persona tiene que ser el que decidió.
func puntuacionesDe(nombre string, fichas []model.Title) map[string]float64 {
	out := map[string]float64{}
	if strings.TrimSpace(nombre) == "" || len(fichas) == 0 {
		return out
	}
	m := importer.MatchTitles([]model.Title{{Name: nombre}}, fichas)
	for _, x := range m.Matched {
		out[x.To] = x.Score
	}
	for _, u := range m.Unmatched {
		for i, c := range u.Candidates {
			if i < len(u.CandidateScores) {
				out[c] = u.CandidateScores[i]
			}
		}
	}
	return out
}

// ── la búsqueda ───────────────────────────────────────────────────────

// titulosBuscar busca una ficha del catálogo por nombre, para elegir con cuál
// va el título de la hoja. Los que están por emparejar no salen: emparejar un
// pendiente con otro pendiente no arregla nada.
func (s *Server) titulosBuscar(w http.ResponseWriter, r *http.Request) {
	todos, err := s.App.Store.Title.List(r.Context())
	if err != nil {
		failStore(w, err, "leer la biblioteca")
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	clave := model.ClaveDeNombre(q)
	suelto := strings.ToLower(model.QuitarAcentos(q))

	type puesto struct {
		t     model.Title
		orden int
	}
	var caben []puesto
	for _, t := range todos {
		if t.PendienteEmparejar {
			continue
		}
		if t.ChannelID != nil && *t.ChannelID != s.App.ChannelID {
			continue
		}
		orden, vale := loParecido(t, clave, suelto)
		if !vale {
			continue
		}
		caben = append(caben, puesto{t: t, orden: orden})
	}
	sort.SliceStable(caben, func(a, b int) bool {
		if caben[a].orden != caben[b].orden {
			return caben[a].orden < caben[b].orden
		}
		return caben[a].t.Name < caben[b].t.Name
	})

	out := []tituloDelCatalogo{}
	for _, p := range caben {
		if len(out) == CandidatosEnLaBusqueda {
			break
		}
		out = append(out, tituloDelCatalogo{ID: p.t.ID, Nombre: p.t.Name, Tipo: string(p.t.Kind)})
	}
	writeJSON(w, http.StatusOK, out)
}

// loParecido dice si una ficha entra en la búsqueda y en qué orden: primero
// las que empiezan por lo escrito, después las que lo llevan dentro, y al
// final las que solo coinciden tal cual se escribió. Sin nada escrito entran
// todas, que es el catálogo por orden alfabético.
func loParecido(t model.Title, clave, suelto string) (int, bool) {
	if clave == "" && suelto == "" {
		return 2, true
	}
	k := model.ClaveDeNombre(t.Name)
	switch {
	case clave != "" && strings.HasPrefix(k, clave):
		return 0, true
	case clave != "" && strings.Contains(k, clave):
		return 1, true
	case suelto != "" && strings.Contains(strings.ToLower(model.QuitarAcentos(t.Name)), suelto):
		return 2, true
	}
	return 0, false
}

// ── la decisión ───────────────────────────────────────────────────────

// Las tres cosas que se pueden decidir sobre un título por emparejar.
const (
	AccionUsar     = "usar"   // es esta ficha del catálogo (F1-66)
	AccionPropio   = "propio" // es un título nuevo de verdad (F1-67)
	AccionQuitar   = "quitar" // no es un programa: se va, con sus reglas (F1-67)
	accionesQueHay = "usar, propio o quitar"
)

// titulosEmparejar aplica la decisión de la persona. Nunca adivina: hasta que
// alguien dice cuál es, el título se queda esperando.
func (s *Server) titulosEmparejar(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		Accion  string `json:"accion"`
		TitleID int64  `json:"title_id"`
	}
	if !decode(w, r, &body) {
		return
	}
	ctx := r.Context()
	provisional, err := s.App.Store.Title.Get(ctx, id)
	if err != nil {
		failStore(w, err, "ese título")
		return
	}

	switch strings.TrimSpace(strings.ToLower(body.Accion)) {
	case AccionUsar:
		s.emparejarConLaFicha(w, r, provisional, body.TitleID)
	case AccionPropio:
		s.emparejarEsPropio(w, r, provisional)
	case AccionQuitar:
		s.emparejarQuitar(w, r, provisional)
	case "":
		failf(w, http.StatusBadRequest, "accion", "no dijiste qué hacer con «%s»: %s",
			provisional.Name, accionesQueHay)
	default:
		failf(w, http.StatusBadRequest, "accion", "no sé qué es %q: %s", body.Accion, accionesQueHay)
	}
}

// emparejarConLaFicha pasa las reglas a la ficha del catálogo y aprende el
// nombre de la hoja (F1-66).
func (s *Server) emparejarConLaFicha(w http.ResponseWriter, r *http.Request, provisional model.Title, destinoID int64) {
	if destinoID <= 0 {
		failf(w, http.StatusBadRequest, "title_id", "falta decir con qué ficha va «%s»", provisional.Name)
		return
	}
	ctx := r.Context()
	destino, err := s.App.Store.Title.Get(ctx, destinoID)
	if err != nil {
		failStore(w, err, "la ficha con la que emparejar")
		return
	}
	movidas, err := s.App.Store.Title.Emparejar(ctx, provisional.ID, destinoID)
	if err != nil {
		failEmparejar(w, err)
		return
	}
	texto := fmt.Sprintf("«%s» ahora es «%s»: %s y la próxima hoja que diga «%s» se empareja sola",
		provisional.Name, destino.Name, reglasQuePasaron(movidas), provisional.Name)
	s.audit(r, "title", &destino.ID, "emparejado", provisional.Name, texto)
	s.despuesDeEmparejar(ctx, texto)
	writeJSON(w, http.StatusOK, map[string]any{
		"reglas_movidas": movidas,
		"alias":          provisional.Name,
		"texto":          texto,
	})
}

// emparejarEsPropio deja la ficha como propia: es un título nuevo de verdad y
// deja de estar por emparejar (F1-67).
func (s *Server) emparejarEsPropio(w http.ResponseWriter, r *http.Request, provisional model.Title) {
	ctx := r.Context()
	if err := s.App.Store.Title.MarcarPropio(ctx, provisional.ID); err != nil {
		failEmparejar(w, err)
		return
	}
	texto := fmt.Sprintf("«%s» se queda como título propio del canal: ya no está por emparejar",
		provisional.Name)
	s.audit(r, "title", &provisional.ID, "pendiente_emparejar", "por emparejar", texto)
	// Las reglas no se mueven, así que el plan sigue igual: solo se apaga el
	// aviso si era el último que quedaba.
	s.App.RefreshPendientes(ctx)
	s.App.Publish("plan", "emparejar", texto)
	writeJSON(w, http.StatusOK, map[string]any{"texto": texto})
}

// emparejarQuitar se lleva el título y las reglas que lo usaban, diciendo
// cuántas (F1-67).
func (s *Server) emparejarQuitar(w http.ResponseWriter, r *http.Request, provisional model.Title) {
	ctx := r.Context()
	quitadas, err := s.App.Store.Title.QuitarProvisional(ctx, provisional.ID)
	if err != nil {
		failEmparejar(w, err)
		return
	}
	texto := fmt.Sprintf("«%s» ya no está en la parrilla: %s", provisional.Name, reglasQueSeFueron(quitadas))
	s.audit(r, "title", &provisional.ID, "quitado", provisional.Name, texto)
	s.despuesDeEmparejar(ctx, texto)
	writeJSON(w, http.StatusOK, map[string]any{
		"reglas_quitadas": quitadas,
		"texto":           texto,
	})
}

// despuesDeEmparejar deja todo cuadrado tras mover reglas: el plan se
// recalcula, la guía se vuelve a escribir con lo que hay, el aviso se ajusta
// y quien esté mirando se entera.
func (s *Server) despuesDeEmparejar(ctx context.Context, texto string) {
	s.App.Recalc()
	if _, err := s.App.Resolve(ctx); err == nil {
		_ = s.App.RefreshGuide(ctx)
	}
	s.App.RefreshPendientes(ctx)
	s.App.Publish("plan", "emparejar", texto)
}

// failEmparejar traduce los peros del store al idioma de la gente y elige el
// código: lo que se arregla escribiendo otra cosa es 400, lo que hay que
// resolver antes es 409.
func failEmparejar(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		fail(w, http.StatusNotFound, "no encontré ese título", "")
	case errors.Is(err, store.ErrMismoTitulo):
		fail(w, http.StatusBadRequest, store.ErrMismoTitulo.Error(), "title_id")
	case errors.Is(err, store.ErrDestinoPendiente):
		fail(w, http.StatusConflict, store.ErrDestinoPendiente.Error(), "title_id")
	case errors.Is(err, store.ErrNoPendiente):
		fail(w, http.StatusConflict, store.ErrNoPendiente.Error(), "")
	default:
		failf(w, http.StatusInternalServerError, "", "no se pudo emparejar el título: %s", err)
	}
}

// reglasQuePasaron y reglasQueSeFueron cuentan reglas en cristiano.
func reglasQuePasaron(n int) string {
	switch n {
	case 0:
		return "no había ninguna regla que pasar"
	case 1:
		return "1 regla pasó a esa ficha"
	}
	return fmt.Sprintf("%d reglas pasaron a esa ficha", n)
}

func reglasQueSeFueron(n int) string {
	switch n {
	case 0:
		return "ninguna regla lo usaba"
	case 1:
		return "se fue con él 1 regla que lo usaba"
	}
	return fmt.Sprintf("se fueron con él %d reglas que lo usaban", n)
}
