package importer

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"antena787/internal/model"
)

// DefaultSlotMs es lo que dura un espacio cuando nadie dice otra cosa: media
// hora, que es la retícula de la hoja de CAtv.
const DefaultSlotMs int64 = 30 * 60 * 1000

// DateShift es una fecha que se corrió un día atrás porque la hora de la
// regla cae de madrugada — antes de que empiece el día de emisión (auditoría
// B1). Se guarda el antes y el después para poder enseñarlo fila por fila.
type DateShift struct {
	Rule       int    // índice en Result.Rules
	Line       int    // línea del texto pegado
	SheetID    string // el Id que traía la hoja
	Title      string
	At         model.Minutes
	FromBefore model.Day
	ToBefore   model.Day
	FromAfter  model.Day
	ToAfter    model.Day
	DaysBefore model.DayPattern
	DaysAfter  model.DayPattern
	Text       string
}

// HandoffProposal es un relevo inferido: la regla que vence y la que parece
// tomar su lugar al día siguiente, en la misma hora y con el mismo patrón.
// El importador NO lo aplica: lo propone, y confirma la persona.
type HandoffProposal struct {
	From   int    // índice en Result.Rules de la regla que vence
	To     int    // índice de la que parece relevarla
	FromID string // Id de la hoja de la que vence
	ToID   string // Id de la hoja de la que releva
	At     model.Minutes
	Text   string
}

// RepeatProposal es un segundo pase inferido: el mismo título, el mismo
// patrón y casi las mismas fechas en dos horas distintas. La de más temprano
// en el día de emisión es la primaria; la otra la repite. Tampoco se aplica
// solo.
type RepeatProposal struct {
	Primary   int // índice en Result.Rules de la regla primaria
	Repeat    int // índice de la que la repite
	PrimaryID string
	RepeatID  string
	Text      string
}

// DuplicateWarning avisa que dos títulos se parecen demasiado. El importador
// nunca los fusiona solo: "SamuraiX" y "Samurai X" quedan como dos títulos y
// decide la persona.
type DuplicateWarning struct {
	A    string `json:"a"`
	B    string `json:"b"`
	Text string `json:"texto"`
}

// Result es todo lo que salió de la hoja: lo que se importa, lo que no
// cuadró, lo que se corrigió y lo que hay que preguntar.
type Result struct {
	Rules      []model.ScheduleRule
	Titles     []model.Title
	SheetIDs   []string // SheetIDs[i] es el Id que traía la hoja para Rules[i]
	Names      []string // Names[i] es el nombre tal como venía en la hoja para Rules[i]
	TitleIndex []int    // TitleIndex[i] es el índice en Titles del título de Rules[i] (-1 si ninguno)

	RowErrors          []RowError
	Notices            []Notice
	DatesShifted       []DateShift
	Handoffs           []HandoffProposal
	Repeats            []RepeatProposal
	PossibleDuplicates []DuplicateWarning

	// Match es lo que dejó el emparejamiento con el catálogo: las erratas
	// que se juntaron solas y los títulos que quedaron por emparejar. Es nil
	// mientras no se llame a ApplyCatalog.
	Match *MatchResult
}

// TitleFor devuelve el título de la regla i, o nil.
//
// Los identificadores de base no existen todavía: el importador es lógica
// pura y no guarda nada. Por eso ScheduleRule.TitleID queda vacío y la
// relación se dice con TitleIndex; el store la traduce a ids al guardar.
func (r *Result) TitleFor(i int) *model.Title {
	if i < 0 || i >= len(r.TitleIndex) {
		return nil
	}
	t := r.TitleIndex[i]
	if t < 0 || t >= len(r.Titles) {
		return nil
	}
	return &r.Titles[t]
}

// NameFor devuelve el nombre de la regla i: el del título si lo tiene y, si
// no lo tiene —una fuente en vivo no es un título—, el que traía la hoja.
func (r *Result) NameFor(i int) string {
	if t := r.TitleFor(i); t != nil {
		return t.Name
	}
	if i >= 0 && i < len(r.Names) {
		return r.Names[i]
	}
	return ""
}

// Summary cuenta en una línea qué pasó, para enseñárselo a una persona.
func (r Result) Summary() string {
	parts := []string{plural(len(r.Rules), "%d regla importada", "%d reglas importadas")}
	if n := len(r.Titles); n > 0 {
		parts = append(parts, plural(n, "%d título", "%d títulos"))
	}
	if n := len(r.RowErrors); n > 0 {
		parts = append(parts, plural(n, "%d fila no cuadró", "%d filas no cuadraron"))
	}
	if n := len(r.DatesShifted); n > 0 {
		parts = append(parts, plural(n,
			"%d fecha corrida por hora de madrugada",
			"%d fechas corridas por hora de madrugada"))
	}
	if n := len(r.Handoffs); n > 0 {
		parts = append(parts, plural(n, "%d relevo propuesto", "%d relevos propuestos"))
	}
	if n := len(r.Repeats); n > 0 {
		parts = append(parts, plural(n, "%d repetición propuesta", "%d repeticiones propuestas"))
	}
	if n := len(r.PossibleDuplicates); n > 0 {
		parts = append(parts, plural(n, "%d posible duplicado", "%d posibles duplicados"))
	}
	return strings.Join(parts, ", ")
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf(one, n)
	}
	return fmt.Sprintf(many, n)
}

// ── opciones ──────────────────────────────────────────────────────────

type options struct {
	liveNames []string
	grid      *Grid
	slotMs    int64
}

// Option ajusta cómo se leen las reglas.
type Option func(*options)

// WithLiveNames dice qué títulos son fuentes en vivo ("RadioOnce Live!").
// Esas filas entran como bloque en vivo y su columna "Duración" se ignora
// con aviso (auditoría B11).
func WithLiveNames(names ...string) Option {
	return func(o *options) { o.liveNames = append(o.liveNames, names...) }
}

// WithGrid le pasa la parrilla leída de la misma hoja. Con ella el
// importador calcula cuánto dura cada espacio —hasta que cambia el título en
// la parrilla, o hasta la siguiente regla del mismo patrón— en vez de asumir
// media hora.
func WithGrid(g *Grid) Option {
	return func(o *options) { o.grid = g }
}

// WithSlotMs cambia la duración de espacio por defecto (media hora).
func WithSlotMs(ms int64) Option {
	return func(o *options) { o.slotMs = ms }
}

// ── la conversión ─────────────────────────────────────────────────────

type parsed struct {
	row      Row
	sheetID  string
	name     string
	episodes int
	days     model.DayPattern
	at       model.Minutes
	from     model.Day
	to       model.Day
	live     bool
}

// Rules convierte la tabla de reglas en reglas y títulos del canal.
//
// Nunca rechaza la hoja entera: la fila que no cuadra va a Result.RowErrors
// con el motivo en cristiano y el resto se importa igual.
func Rules(sheet Sheet, ch model.Channel, opts ...Option) Result {
	o := options{slotMs: DefaultSlotMs}
	for _, f := range opts {
		f(&o)
	}
	live := map[string]bool{}
	for _, n := range o.liveNames {
		live[loose(n)] = true
	}

	res := Result{}
	if sheet.Kind != KindRules {
		res.RowErrors = append(res.RowErrors, RowError{
			Reason: "esta tabla no parece la de reglas: espero columnas como Id, Video, Duración, Días, Horas, Fec Ini y Fec Final",
		})
		return res
	}

	titles := map[string]int{} // nombre normalizado → índice en res.Titles
	maxEpisodes := map[int]int{}
	var rows []parsed

	for _, row := range sheet.Rows {
		p, err := readRuleRow(sheet, row)
		if err != nil {
			res.RowErrors = append(res.RowErrors, *err)
			continue
		}
		if live[loose(p.name)] {
			p.live = true
			if raw := strings.TrimSpace(sheet.Get(row, ColEpisodes)); raw != "" && raw != "1" {
				res.Notices = append(res.Notices, Notice{
					Line: row.Line, SheetID: p.sheetID, Title: p.name,
					Text: fmt.Sprintf("«%s» es una fuente en vivo, así que no lleva episodios: ignoré la columna Duración, que decía %s", p.name, raw),
				})
			}
			p.episodes = 0
		}
		rows = append(rows, p)
	}

	for _, p := range rows {
		// El título: se crea por nombre y no se duplica. Nombres que solo
		// difieren en espacios o mayúsculas son el mismo; los demás no se
		// fusionan solos (ver PossibleDuplicates).
		//
		// El nombre de una fuente en vivo no es un título y nunca entra al
		// catálogo (F1-67): "RadioOnce Live!" es de dónde sale la señal, no
		// un programa con ficha. Esas reglas se quedan sin título y apuntan
		// a su fuente por LiveSourceID, que pone el store.
		ti := -1
		if !p.live {
			key := strings.ToLower(cleanName(p.name))
			var ok bool
			ti, ok = titles[key]
			if !ok {
				ti = len(res.Titles)
				titles[key] = ti
				t := model.Title{
					Name:           cleanName(p.name),
					Kind:           model.TitleProgram,
					MetadataSource: "hoja",
				}
				if ch.ID != 0 {
					id := ch.ID
					t.ChannelID = &id
				}
				res.Titles = append(res.Titles, t)
			}
			if p.episodes > maxEpisodes[ti] {
				maxEpisodes[ti] = p.episodes
			}
		}

		rule := model.ScheduleRule{
			ChannelID:      ch.ID,
			Kind:           model.RuleNormal,
			Days:           p.days,
			At:             p.at,
			SlotMs:         o.slotMs,
			From:           p.from,
			To:             p.to,
			EpisodesPerRun: p.episodes,
			Active:         true,
		}
		if p.live {
			rule.Kind = model.RuleLive
		}
		idx := len(res.Rules)
		res.Rules = append(res.Rules, rule)
		res.SheetIDs = append(res.SheetIDs, p.sheetID)
		res.Names = append(res.Names, cleanName(p.name))
		res.TitleIndex = append(res.TitleIndex, ti)

		// Corrimiento por día de emisión (auditoría B1): la hoja usa fecha de
		// calendario. Una regla de las 12:00 AM del martes es, en realidad,
		// del día de emisión del lunes.
		if shift := shiftDates(&res.Rules[idx], ch); shift != nil {
			shift.Rule = idx
			shift.Line = p.row.Line
			shift.SheetID = p.sheetID
			shift.Title = cleanName(p.name)
			res.DatesShifted = append(res.DatesShifted, *shift)
		}
	}

	// Un título con más de un episodio por corrida es una serie; de lo que no
	// se sabe, se dice "programa".
	for ti, n := range maxEpisodes {
		if n > 1 {
			res.Titles[ti].Kind = model.TitleSeries
		}
	}

	// La duración del espacio, si nos pasaron la parrilla.
	if o.grid != nil {
		for i := range res.Rules {
			if ms := slotFromGrid(o.grid, res.Rules[i].At, res.NameFor(i)); ms > 0 {
				res.Rules[i].SlotMs = ms
				continue
			}
			if ms := slotFromNextRule(res.Rules, i, o.slotMs); ms > 0 {
				res.Rules[i].SlotMs = ms
			}
		}
	}

	res.Handoffs = proposeHandoffs(&res)
	res.Repeats = proposeRepeats(&res, ch)
	res.PossibleDuplicates = findDuplicates(res.Titles)
	return res
}

// readRuleRow interpreta una fila. El error, si lo hay, ya viene escrito para
// una persona: dice qué fila, de qué programa, y qué fue lo que no se pudo
// leer — nunca "está mal".
func readRuleRow(sheet Sheet, row Row) (parsed, *RowError) {
	p := parsed{row: row, sheetID: strings.TrimSpace(sheet.Get(row, ColID))}
	p.name = cleanName(sheet.Get(row, ColTitle))
	fail := func(reason string) (parsed, *RowError) {
		return p, &RowError{Line: row.Line, SheetID: p.sheetID, Title: p.name, Reason: reason}
	}

	if p.name == "" || p.name == "-" {
		return fail("esta fila no dice qué programa va, así que no supe qué programar")
	}

	rawAt := sheet.Get(row, ColAt)
	at, ok := ParseClock(rawAt)
	if !ok {
		if strings.TrimSpace(rawAt) == "" {
			return fail("esta fila no trae hora, y sin hora no hay a qué hora sacarla")
		}
		return fail(fmt.Sprintf("no entendí la hora «%s»: se espera algo como 7:30:00 AM o 23:30", strings.TrimSpace(rawAt)))
	}
	p.at = at

	rawDays := strings.ToUpper(strings.TrimSpace(sheet.Get(row, ColDays)))
	rawDays = strings.ReplaceAll(rawDays, " ", "_")
	p.days = model.DayPattern(rawDays)
	if !p.days.Valid() {
		if rawDays == "" {
			return fail("esta fila no trae los días, y sin días no se sabe cuándo va: se espera algo como LMMJV__")
		}
		return fail(fmt.Sprintf("el patrón de días «%s» no cuadra: se esperan siete posiciones, una por día de lunes a domingo, con la letra que toca o «_», como LMMJV__", rawDays))
	}

	rawFrom, rawTo := sheet.Get(row, ColFrom), sheet.Get(row, ColTo)
	from, okFrom := ParseDate(rawFrom)
	if !okFrom {
		if strings.TrimSpace(rawFrom) == "" {
			return fail("esta fila no trae fecha de inicio, así que no supe desde cuándo va")
		}
		return fail(fmt.Sprintf("no entendí la fecha de inicio «%s»: se espera algo como 8/22/2026 o 2026-08-22", strings.TrimSpace(rawFrom)))
	}
	to, okTo := ParseDate(rawTo)
	if !okTo {
		if strings.TrimSpace(rawTo) == "" {
			return fail("esta fila no trae fecha de fin, así que no supe hasta cuándo va")
		}
		return fail(fmt.Sprintf("no entendí la fecha de fin «%s»: se espera algo como 1/3/2027 o 2027-01-03", strings.TrimSpace(rawTo)))
	}
	// El bug del Hellsing: la hoja lo marca "✓ OK" y aquí no pasa. Se
	// reportan las fechas tal como venían en la hoja, sin corrimiento, que es
	// lo que la persona tiene delante.
	if to < from {
		return fail(fmt.Sprintf("la fecha de fin (%s) es anterior a la de inicio (%s)", FormatDate(to), FormatDate(from)))
	}
	p.from, p.to = from, to

	p.episodes = 1
	if raw := strings.TrimSpace(sheet.Get(row, ColEpisodes)); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			// No es motivo para dejar fuera la fila: se asume una corrida.
			p.episodes = 1
		} else {
			p.episodes = n
		}
	}
	return p, nil
}

// shiftDates corre las fechas —y el patrón de días— un día atrás cuando la
// hora cae antes del inicio del día de emisión. Devuelve nil si no hubo nada
// que correr.
//
// El patrón también se corre, no solo las fechas: la hoja marca el día de
// calendario, y las 12:00 AM del martes son el día de emisión del lunes. Así
// que el día de calendario N pasa a ser el día de emisión N−1 y el patrón
// rota una posición a la izquierda: "_MMJVS_" (martes a sábado de
// calendario) es "LMMJV__" (lunes a viernes de emisión), y "L_____D" es
// "_____SD".
func shiftDates(r *model.ScheduleRule, ch model.Channel) *DateShift {
	if ch.BroadcastDayAt <= 0 || r.At >= ch.BroadcastDayAt {
		return nil
	}
	sh := DateShift{
		At:         r.At,
		FromBefore: r.From,
		ToBefore:   r.To,
		DaysBefore: r.Days,
	}
	r.From = r.From.Add(-1)
	r.To = r.To.Add(-1)
	r.Days = shiftPattern(r.Days)
	sh.FromAfter, sh.ToAfter, sh.DaysAfter = r.From, r.To, r.Days
	sh.Text = fmt.Sprintf(
		"va a las %s, antes de que empiece el día de emisión (%s), así que en la hoja esa hora ya es del día siguiente: corrí sus fechas y sus días un día atrás (%s de calendario = %s de día de emisión; %s → %s y %s → %s)",
		FormatClock(r.At), FormatClock(ch.BroadcastDayAt),
		DescribePattern(sh.DaysBefore), DescribePattern(sh.DaysAfter),
		FormatDate(sh.FromBefore), FormatDate(sh.FromAfter),
		FormatDate(sh.ToBefore), FormatDate(sh.ToAfter))
	return &sh
}

// shiftPattern rota el patrón una posición a la izquierda: lo que en la hoja
// es el día de calendario N sale el día de emisión N−1, y el lunes de
// calendario cae en el domingo de emisión.
func shiftPattern(p model.DayPattern) model.DayPattern {
	if !p.Valid() {
		return p
	}
	out := []byte("_______")
	for i := 0; i < 7; i++ {
		if p[(i+1)%7] != '_' {
			out[i] = model.Letters[i]
		}
	}
	return model.DayPattern(out)
}

var dayNames = [7]string{"lunes", "martes", "miércoles", "jueves", "viernes", "sábado", "domingo"}

// DescribePattern dice un patrón en palabras: "lunes a viernes", "sábado y
// domingo", "lunes, miércoles y viernes".
func DescribePattern(p model.DayPattern) string {
	if !p.Valid() {
		return string(p)
	}
	var on []int
	for i := 0; i < 7; i++ {
		if p[i] != '_' {
			on = append(on, i)
		}
	}
	switch len(on) {
	case 0:
		return "ningún día"
	case 7:
		return "todos los días"
	}
	seguidos := true
	for i := 1; i < len(on); i++ {
		if on[i] != on[i-1]+1 {
			seguidos = false
			break
		}
	}
	if seguidos && len(on) > 2 {
		return dayNames[on[0]] + " a " + dayNames[on[len(on)-1]]
	}
	names := make([]string, len(on))
	for i, d := range on {
		names[i] = dayNames[d]
	}
	if len(names) == 1 {
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " y " + names[len(names)-1]
}

// ── propuestas ────────────────────────────────────────────────────────

// proposeHandoffs infiere relevos (auditoría B10): la regla B empieza el día
// siguiente al fin de A, a la misma hora y con el mismo patrón. Se propone;
// lo confirma la persona (la API tiene `confirmar-relevos`).
func proposeHandoffs(res *Result) []HandoffProposal {
	var out []HandoffProposal
	for a := range res.Rules {
		for b := range res.Rules {
			if a == b {
				continue
			}
			ra, rb := res.Rules[a], res.Rules[b]
			if ra.At != rb.At || ra.Days != rb.Days || ra.To.Add(1) != rb.From {
				continue
			}
			na, nb := res.NameFor(a), res.NameFor(b)
			if na == "" || nb == "" || loose(na) == loose(nb) {
				continue
			}
			out = append(out, HandoffProposal{
				From: a, To: b,
				FromID: res.SheetIDs[a], ToID: res.SheetIDs[b],
				At:   ra.At,
				Text: fmt.Sprintf("¿%s releva a %s a las %s?", nb, na, FormatClock(ra.At)),
			})
		}
	}
	return out
}

// proposeRepeats infiere segundos pases (auditoría B5): el mismo título, el
// mismo patrón, fechas parecidas y dos horas distintas. La de más temprano en
// el día de emisión es la primaria; la otra la repite.
func proposeRepeats(res *Result, ch model.Channel) []RepeatProposal {
	const nearDays = 3
	var out []RepeatProposal
	seen := map[[2]int]bool{}
	for a := range res.Rules {
		for b := range res.Rules {
			if a == b {
				continue
			}
			ra, rb := res.Rules[a], res.Rules[b]
			if ra.Days != rb.Days || ra.At == rb.At {
				continue
			}
			na, nb := res.NameFor(a), res.NameFor(b)
			if na == "" || nb == "" || loose(na) != loose(nb) {
				continue
			}
			if absDays(ra.From, rb.From) > nearDays || absDays(ra.To, rb.To) > nearDays {
				continue
			}
			primary, repeat := a, b
			if dayOffset(rb.At, ch) < dayOffset(ra.At, ch) {
				primary, repeat = b, a
			}
			key := [2]int{primary, repeat}
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, RepeatProposal{
				Primary: primary, Repeat: repeat,
				PrimaryID: res.SheetIDs[primary], RepeatID: res.SheetIDs[repeat],
				Text: fmt.Sprintf("«%s» de las %s parece la repetición de «%s» de las %s: ¿la marco como segundo pase?",
					res.NameFor(repeat), FormatClock(res.Rules[repeat].At),
					res.NameFor(primary), FormatClock(res.Rules[primary].At)),
			})
		}
	}
	return out
}

// dayOffset dice cuántos minutos lleva esa hora dentro del día de emisión,
// para que las 12:00 AM cuenten como el final del día y no como el principio.
func dayOffset(at model.Minutes, ch model.Channel) int {
	return ((int(at) - int(ch.BroadcastDayAt)) + 1440) % 1440
}

func absDays(a, b model.Day) int {
	d := int(a.Time(time.UTC).Sub(b.Time(time.UTC)).Hours() / 24)
	if d < 0 {
		return -d
	}
	return d
}

// findDuplicates avisa de nombres que se parecen demasiado. No fusiona: dice.
func findDuplicates(titles []model.Title) []DuplicateWarning {
	var out []DuplicateWarning
	for i := 0; i < len(titles); i++ {
		for j := i + 1; j < len(titles); j++ {
			if loose(titles[i].Name) != loose(titles[j].Name) {
				continue
			}
			out = append(out, DuplicateWarning{
				A: titles[i].Name, B: titles[j].Name,
				Text: fmt.Sprintf("«%s» y «%s» se escriben casi igual: ¿son el mismo título? No los junté por si acaso", titles[i].Name, titles[j].Name),
			})
		}
	}
	return out
}

// ── duración del espacio ──────────────────────────────────────────────

// slotFromGrid mira la parrilla: cuántas franjas seguidas lleva el mismo
// título a partir de esa hora.
func slotFromGrid(g *Grid, at model.Minutes, name string) int64 {
	if g == nil || name == "" {
		return 0
	}
	start := -1
	for i, s := range g.Slots {
		if s.At == at {
			start = i
			break
		}
	}
	if start < 0 || !slotHas(g.Slots[start], name) {
		return 0
	}
	end := start + 1
	for end < len(g.Slots) && slotHas(g.Slots[end], name) {
		end++
	}
	if end < len(g.Slots) {
		d := int64(g.Slots[end].At-g.Slots[start].At) * 60 * 1000
		if d > 0 {
			return d
		}
	}
	return int64(end-start) * DefaultSlotMs
}

func slotHas(s Slot, name string) bool {
	want := loose(name)
	for _, c := range s.Cells {
		if c != "" && loose(c) == want {
			return true
		}
	}
	return false
}

// slotFromNextRule es el respaldo: el espacio dura hasta la siguiente regla
// del mismo patrón de días.
func slotFromNextRule(rules []model.ScheduleRule, i int, fallback int64) int64 {
	best := -1
	for j := range rules {
		if j == i || rules[j].Days != rules[i].Days || rules[j].At <= rules[i].At {
			continue
		}
		d := int(rules[j].At - rules[i].At)
		if best < 0 || d < best {
			best = d
		}
	}
	if best <= 0 {
		return fallback
	}
	return int64(best) * 60 * 1000
}
