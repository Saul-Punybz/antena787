package importer

import (
	"fmt"
	"sort"
	"strings"

	"antena787/internal/model"
)

// Una hoja hecha a mano escribe el mismo programa de dos maneras: en las
// reglas dice "Zorro 57" y en el catálogo "Zorro (1957)", "Los Lorcanitos" y
// "Lorcanitos", "Comics 9th Art" y "Comics, the Ninth Art". No son títulos
// distintos: son erratas del mismo, y el nombre bueno es el del catálogo,
// que es donde está la ficha.
//
// Aquí se emparejan. Se normaliza el nombre (minúsculas, sin acentos, sin
// signos, sin el artículo del principio, números y años como un token más) y
// se comparan por parecido: Jaro-Winkler sobre la forma pegada, y cuántas
// palabras del título de la regla caben en el del catálogo.
//
// Se empareja **solo** cuando hay un único candidato claro: por encima del
// umbral y con distancia al segundo. Si dos empatan —"SaberMarionette" contra
// "Saber Marionette J", "Saber Marionette R" y "Saber Marionette J Again"— no
// se adivina: va a la lista de los que decide la persona.
const (
	// MatchThreshold es el parecido mínimo para dar dos nombres por el mismo.
	MatchThreshold = 0.86
	// MatchMargin es cuánto tiene que separarse el mejor candidato del
	// segundo para que se acepte sin preguntar.
	MatchMargin = 0.02
)

// TitleMatch es una errata emparejada: el nombre que traía la regla y el
// nombre bueno.
type TitleMatch struct {
	From  string  `json:"desde"`      // como venía en las reglas
	To    string  `json:"hacia"`      // el nombre bueno, el del catálogo
	Score float64 `json:"puntuacion"` // 1 cuando lo dijo una persona, no el parecido
	Text  string  `json:"texto"`
}

// Alias es un nombre de la hoja que ya se emparejó una vez: la hoja dice
// "Samurai X" y la ficha se llama "Rurouni Kenshin". No es una errata que se
// pueda adivinar por parecido —no se parecen en nada—, así que se recuerda:
// la próxima hoja que traiga ese nombre se empareja sola, sin preguntar
// (F1-66).
type Alias struct {
	Nombre string `json:"nombre"` // como viene escrito en la hoja
	Titulo string `json:"titulo"` // el nombre de la ficha del catálogo
}

// Unmatched es un título de las reglas que se quedó sin ficha: o no se
// parece a ninguno, o se parece igual a varios. Va a la pantalla para que lo
// empareje una persona, con todo lo que hace falta para reconocerlo: en qué
// reglas sale, con qué Id de la hoja y en qué franja.
type Unmatched struct {
	Name            string    `json:"nombre"`
	Candidates      []string  `json:"candidatos"`   // los que empataron, si empataron
	CandidateScores []float64 `json:"puntuaciones"` // el parecido de cada candidato, en el mismo orden
	Rules           []int     `json:"reglas"`       // índices en Result.Rules que lo usan
	SheetIDs        []string  `json:"id_hoja"`      // los Id que traía la hoja, en el mismo orden
	Slots           []string  `json:"franjas"`      // "lunes a viernes a las 7:30 AM", en el mismo orden
	Text            string    `json:"texto"`
}

// MatchResult es el catálogo y las reglas hablando del mismo título.
type MatchResult struct {
	Titles     []model.Title  // el catálogo, más los de las reglas que no tienen ficha
	TitleIndex map[string]int // nombre tal como venía en las reglas → índice en Titles
	Matched    []TitleMatch
	Unmatched  []Unmatched
	// Provisional son los índices en Titles de los títulos que se crearon
	// porque la regla los nombraba y no había ficha: quedan "por emparejar"
	// hasta que una persona diga si son una ficha del catálogo o un título
	// nuevo (F1-64). Nunca se crean callados.
	Provisional []int
	Notices     []Notice
}

// EsProvisional dice si el título i de Titles es uno de los que se quedaron
// por emparejar.
func (m *MatchResult) EsProvisional(i int) bool {
	for _, p := range m.Provisional {
		if p == i {
			return true
		}
	}
	return false
}

// MatchTitles empareja los títulos que salieron de las reglas con las fichas
// del catálogo. El nombre del catálogo manda: el de las reglas se queda como
// errata emparejada, no como título aparte.
//
// Antes que nada van los alias: lo que una persona ya emparejó en otra hoja
// no se vuelve a preguntar ni se pasa por el parecido, que nunca lo sacaría
// ("Samurai X" no se parece a "Rurouni Kenshin").
//
// Los que no encuentran ficha se comparan entre ellos, porque las reglas
// también se contradicen solas ("SamuraiX" y "Samurai X"): se quedan con una
// sola escritura, la que trae las palabras separadas.
func MatchTitles(fromRules, catalog []model.Title, aliases ...Alias) MatchResult {
	res := MatchResult{
		Titles:     append([]model.Title(nil), catalog...),
		TitleIndex: map[string]int{},
	}
	byName := map[string]int{}
	for i, t := range res.Titles {
		byName[t.Name] = i
	}
	keys := make([]nameKey, len(catalog))
	byKey := map[string]int{} // nombre normalizado de la ficha → índice en catalog
	for i, t := range catalog {
		keys[i] = newNameKey(t.Name)
		if _, ok := byKey[keys[i].compact]; !ok {
			byKey[keys[i].compact] = i
		}
	}
	// Los alias que sí apuntan a una ficha que está en este catálogo.
	porAlias := map[string]int{} // nombre normalizado de la hoja → índice en catalog
	for _, a := range aliases {
		at, ok := byKey[newNameKey(a.Titulo).compact]
		if !ok {
			continue
		}
		porAlias[newNameKey(a.Nombre).compact] = at
	}

	// Primera pasada: contra el catálogo.
	pending := []int{} // índices en fromRules que se quedaron sin ficha
	for i, t := range fromRules {
		// Lo que ya se emparejó antes no vuelve a pasar por el parecido.
		if at, ok := porAlias[newNameKey(t.Name).compact]; ok {
			res.TitleIndex[t.Name] = at
			fillFromRule(&res.Titles[at], t)
			if !sameName(t.Name, catalog[at].Name) {
				m := TitleMatch{From: t.Name, To: catalog[at].Name, Score: 1,
					Text: fmt.Sprintf("«%s» es «%s»: lo recordaba de otra hoja", t.Name, catalog[at].Name)}
				res.Matched = append(res.Matched, m)
				res.Notices = append(res.Notices, Notice{Title: t.Name, Text: m.Text})
			}
			continue
		}
		best, second, at := bestMatch(newNameKey(t.Name), keys)
		switch {
		case at >= 0 && best >= MatchThreshold && best-second >= MatchMargin:
			res.TitleIndex[t.Name] = at
			fillFromRule(&res.Titles[at], t)
			if !sameName(t.Name, catalog[at].Name) {
				m := TitleMatch{From: t.Name, To: catalog[at].Name, Score: best,
					Text: fmt.Sprintf("«%s» es «%s» en el catálogo: los junté en un solo título", t.Name, catalog[at].Name)}
				res.Matched = append(res.Matched, m)
				res.Notices = append(res.Notices, Notice{Title: t.Name, Text: m.Text})
			}
		default:
			pending = append(pending, i)
		}
	}

	// Segunda pasada: los que sobraron, entre ellos.
	canon := map[int]int{} // índice en pending → índice en fromRules que manda
	for _, i := range pending {
		canon[i] = i
	}
	for _, i := range pending {
		for _, j := range pending {
			if i >= j || canon[i] != i || canon[j] != j {
				continue
			}
			s, _, _ := bestMatch(newNameKey(fromRules[i].Name), []nameKey{newNameKey(fromRules[j].Name)})
			if s < MatchThreshold {
				continue
			}
			// Manda la escritura con las palabras separadas.
			keep, alias := i, j
			if len(newNameKey(fromRules[j].Name).tokens) > len(newNameKey(fromRules[i].Name).tokens) {
				keep, alias = j, i
			}
			canon[alias] = keep
		}
	}
	for _, i := range pending {
		if canon[i] != i {
			continue
		}
		t := fromRules[i]
		idx, ok := byName[t.Name]
		if !ok {
			idx = len(res.Titles)
			res.Titles = append(res.Titles, t)
			byName[t.Name] = idx
			// Se crea porque la regla lo nombra, pero queda marcado: es
			// provisional hasta que una persona lo empareje o diga que es
			// un título nuevo.
			res.Provisional = append(res.Provisional, idx)
		}
		res.TitleIndex[t.Name] = idx

		u := Unmatched{Name: t.Name}
		if cands, scores := tiedCandidates(newNameKey(t.Name), keys, catalog); len(cands) > 0 {
			u.Candidates, u.CandidateScores = cands, scores
			u.Text = fmt.Sprintf("«%s» se parece igual a %s: no supe cuál es y no quise adivinar",
				t.Name, listar(cands))
		} else {
			u.Text = fmt.Sprintf("«%s» no tiene ficha en el catálogo", t.Name)
		}
		res.Unmatched = append(res.Unmatched, u)
	}
	for _, i := range pending {
		if canon[i] == i {
			continue
		}
		from, to := fromRules[i].Name, fromRules[canon[i]].Name
		res.TitleIndex[from] = res.TitleIndex[to]
		m := TitleMatch{From: from, To: to, Score: 1,
			Text: fmt.Sprintf("«%s» y «%s» son el mismo título escrito de dos maneras: me quedé con «%s»", from, to, to)}
		res.Matched = append(res.Matched, m)
		res.Notices = append(res.Notices, Notice{Title: from, Text: m.Text})
	}

	sort.SliceStable(res.Matched, func(a, b int) bool { return res.Matched[a].From < res.Matched[b].From })
	sort.SliceStable(res.Unmatched, func(a, b int) bool { return res.Unmatched[a].Name < res.Unmatched[b].Name })
	return res
}

// MergeTitles junta el catálogo con lo que salió de las reglas: la ficha la
// manda el catálogo, y las erratas de la hoja se emparejan solas.
func MergeTitles(fromRules, fromCatalog []model.Title) ([]model.Title, []Unmatched) {
	m := MatchTitles(fromRules, fromCatalog)
	return m.Titles, m.Unmatched
}

// ApplyCatalog empareja los títulos de estas reglas con las fichas del
// catálogo y deja las reglas apuntando al título del catálogo, no a uno
// nuevo. Los avisos del emparejamiento se suman a los del importador.
func (r *Result) ApplyCatalog(catalog []model.Title) MatchResult {
	return r.ApplyCatalogCon(catalog, nil)
}

// ApplyCatalogCon hace lo mismo, pero primero mira los alias que ya se
// aprendieron: el nombre de la hoja que una persona emparejó otra vez va
// derecho a su ficha, sin volver a preguntar (F1-66).
//
// El emparejamiento entero queda en Result.Match, para que quien llame pueda
// enseñar lo que quedó por emparejar sin volver a correrlo.
func (r *Result) ApplyCatalogCon(catalog []model.Title, aliases []Alias) MatchResult {
	m := MatchTitles(r.Titles, catalog, aliases...)
	old := r.Titles
	r.Titles = m.Titles
	for i := range r.TitleIndex {
		t := r.TitleIndex[i]
		if t < 0 || t >= len(old) {
			continue
		}
		if idx, ok := m.TitleIndex[old[t].Name]; ok {
			r.TitleIndex[i] = idx
		}
	}
	r.contextoSinPareja(&m)
	r.Notices = append(r.Notices, m.Notices...)
	r.PossibleDuplicates = findDuplicates(r.Titles)
	r.Match = &m
	return m
}

// contextoSinPareja le pone a cada título sin pareja dónde sale: las reglas
// que lo usan, el Id que traía la hoja y la franja en palabras. Sin esto la
// pantalla enseña un nombre suelto y la persona no sabe de qué programa le
// están hablando.
func (r *Result) contextoSinPareja(m *MatchResult) {
	if len(m.Unmatched) == 0 {
		return
	}
	donde := map[int]int{} // índice en Titles → posición en Unmatched
	for u := range m.Unmatched {
		if idx, ok := m.TitleIndex[m.Unmatched[u].Name]; ok && idx >= 0 {
			donde[idx] = u
		}
	}
	for i := range r.Rules {
		if i >= len(r.TitleIndex) || r.TitleIndex[i] < 0 {
			continue
		}
		u, ok := donde[r.TitleIndex[i]]
		if !ok {
			continue
		}
		e := &m.Unmatched[u]
		e.Rules = append(e.Rules, i)
		e.SheetIDs = append(e.SheetIDs, sheetIDOf(r, i))
		e.Slots = append(e.Slots, fmt.Sprintf("%s a las %s",
			DescribePattern(r.Rules[i].Days), FormatClock(r.Rules[i].At)))
	}
}

func sheetIDOf(r *Result, i int) string {
	if i >= 0 && i < len(r.SheetIDs) {
		return r.SheetIDs[i]
	}
	return ""
}

// fillFromRule pasa a la ficha del catálogo lo poco que sabe la regla: que es
// una serie, si emitía más de un episodio por corrida.
func fillFromRule(dst *model.Title, src model.Title) {
	if dst.Kind == model.TitleProgram && src.Kind == model.TitleSeries {
		dst.Kind = model.TitleSeries
	}
	if dst.ChannelID == nil && src.ChannelID != nil {
		id := *src.ChannelID
		dst.ChannelID = &id
	}
}

func sameName(a, b string) bool { return strings.EqualFold(cleanName(a), cleanName(b)) }

func listar(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return "«" + names[0] + "»"
	}
	q := make([]string, len(names))
	for i, n := range names {
		q[i] = "«" + n + "»"
	}
	return strings.Join(q[:len(q)-1], ", ") + " y " + q[len(q)-1]
}

// tiedCandidates devuelve los nombres que empataron arriba del umbral, que es
// lo que se le enseña a la persona cuando no se puede decidir solo.
func tiedCandidates(a nameKey, keys []nameKey, catalog []model.Title) ([]string, []float64) {
	best := 0.0
	for _, k := range keys {
		if s := similarity(a, k); s > best {
			best = s
		}
	}
	if best < MatchThreshold {
		return nil, nil
	}
	var out []string
	var scores []float64
	for i, k := range keys {
		if s := similarity(a, k); s >= best-MatchMargin {
			out = append(out, catalog[i].Name)
			scores = append(scores, s)
		}
	}
	if len(out) < 2 {
		return nil, nil
	}
	return out, scores
}

// ── el parecido ───────────────────────────────────────────────────────

type nameKey struct {
	tokens  []string
	compact string
}

var articles = map[string]bool{"the": true, "el": true, "la": true, "los": true, "las": true}

// newNameKey deja el nombre comparable: minúsculas, sin acentos, sin signos,
// sin el artículo del principio. El apóstrofo se quita sin dejar hueco
// ("Gilligan's" → "gilligans"); los demás signos separan palabras, así que
// los números y los años entre paréntesis quedan como un token más.
func newNameKey(name string) nameKey {
	s := stripAccents(strings.ToLower(unescapeMarkdown(name)))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '\'' || r == '’':
			// nada: el apóstrofo no parte la palabra
		default:
			b.WriteRune(' ')
		}
	}
	tokens := strings.Fields(b.String())
	if len(tokens) > 1 && articles[tokens[0]] {
		tokens = tokens[1:]
	}
	return nameKey{tokens: tokens, compact: strings.Join(tokens, "")}
}

// similarity mezcla dos maneras de parecerse: escribirse casi igual
// (Jaro-Winkler sobre la forma pegada) y caber entero dentro del otro
// (cuántas palabras del primero están en el segundo). Lo segundo pesa un
// poco menos, para que un título que cabe dentro de otro más largo no gane
// nunca a uno que se escribe igual.
func similarity(a, b nameKey) float64 {
	s := jaroWinkler(a.compact, b.compact)
	if c := 0.9 * containment(a.tokens, b.tokens); c > s {
		s = c
	}
	return s
}

// containment dice qué parte de las palabras de a aparece en b, dejando
// pasar erratas dentro de cada palabra ("familia" y "family").
func containment(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	hits := 0.0
	for _, ta := range a {
		best := 0.0
		for _, tb := range b {
			if s := jaroWinkler(ta, tb); s > best {
				best = s
			}
		}
		if best >= 0.9 {
			hits++
		}
	}
	return hits / float64(len(a))
}

func bestMatch(a nameKey, keys []nameKey) (best, second float64, at int) {
	at = -1
	for i, k := range keys {
		s := similarity(a, k)
		switch {
		case s > best:
			second, best, at = best, s, i
		case s > second:
			second = s
		}
	}
	return best, second, at
}

// jaroWinkler es el parecido clásico entre dos cadenas: 1 si son iguales, y
// premia que compartan el principio, que es como se escriben las erratas.
func jaroWinkler(a, b string) float64 {
	j := jaro(a, b)
	if j < 0.7 {
		return j
	}
	prefix := 0
	for prefix < len(a) && prefix < len(b) && a[prefix] == b[prefix] && prefix < 4 {
		prefix++
	}
	return j + float64(prefix)*0.1*(1-j)
}

func jaro(a, b string) float64 {
	if a == b {
		return 1
	}
	la, lb := len(a), len(b)
	if la == 0 || lb == 0 {
		return 0
	}
	window := la
	if lb > window {
		window = lb
	}
	window = window/2 - 1
	if window < 0 {
		window = 0
	}
	usedA, usedB := make([]bool, la), make([]bool, lb)
	matches := 0
	for i := 0; i < la; i++ {
		lo := i - window
		if lo < 0 {
			lo = 0
		}
		hi := i + window + 1
		if hi > lb {
			hi = lb
		}
		for j := lo; j < hi; j++ {
			if usedB[j] || a[i] != b[j] {
				continue
			}
			usedA[i], usedB[j] = true, true
			matches++
			break
		}
	}
	if matches == 0 {
		return 0
	}
	transpositions, k := 0, 0
	for i := 0; i < la; i++ {
		if !usedA[i] {
			continue
		}
		for !usedB[k] {
			k++
		}
		if a[i] != b[k] {
			transpositions++
		}
		k++
	}
	m := float64(matches)
	return (m/float64(la) + m/float64(lb) + (m-float64(transpositions)/2)/m) / 3
}
