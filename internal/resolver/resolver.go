// Package resolver arma el plan: convierte las reglas de programación en
// plan_item con instante y duración exactos, rellena lo que sobra y avisa en
// cristiano de lo que no cuadra. Es lógica pura —sin base de datos, sin red y
// sin ffmpeg—: recibe todo en memoria y devuelve el estado deseado de la
// ventana. Aplicar el diff contra lo que ya está guardado es trabajo del
// store; correr Resolve dos veces con la misma entrada da exactamente lo
// mismo, y por eso nunca duplica nada.
//
// Lo que decide, y dónde está escrito:
//
//   - Día de emisión (PRD §8 y §15, auditoría B1 y B3): las fechas y el
//     patrón LMMJVSD de una regla se leen por día de emisión, fecha_fin es
//     inclusiva hasta el cierre del día, y un ítem pertenece al día en que
//     empieza aunque termine del otro lado de la frontera.
//   - Horario de verano (PRD §15): la hora que no existe se corre a la
//     siguiente válida; la que existe dos veces sale en la primera.
//   - El reloj manda (PRD §9 paso 3, auditoría B4): un episodio que no
//     termina antes del siguiente inicio duro no se arranca; se rellena lo
//     que queda y se avisa "de 10 episodios caben 9". Nunca se corta nada a
//     la mitad. El fin de un bloque en vivo es igual de duro.
//   - Repeticiones (auditoría B5): repite_a emite el mismo episodio que su
//     regla primaria puso ese día de emisión; si la primaria no puso nada,
//     toma el siguiente y avanza el contador compartido. Un solo contador
//     por serie en su franja.
//   - Relevo (PRD §9 paso 3): un relevo no es un conflicto. Dos reglas que
//     arrancan a la misma hora el mismo día sin relevo sí lo son: gana la de
//     menor id y se avisa.
//   - Relleno (PRD §14.1): suma exacta si se puede; si no, se excede hasta
//     5 s y se recorta el último clip del hueco con un fundido de 1 s. Sin
//     relleno, cartel: nunca queda hueco residual.
//   - Diferido (PRD §9 paso 8, auditoría B9): reprograma los archivos del
//     deck programa de la ventana de origen del mismo día de emisión; una
//     ventana sin programa no produce nada y esa hora cae a relleno.
//   - Material listo (auditoría B7): solo entra al plan lo que está en
//     estado listo y con la normalización terminada.
//   - Lo tocado a mano (F1-26): un plan_item fijado por una persona ocupa su
//     hora como si estuviera ya al aire —nada se le pone encima, el relleno
//     lo rodea— y no genera aviso ninguno.
package resolver

import (
	"fmt"
	"sort"
	"time"

	"antena787/internal/model"
)

// Valores por defecto. Todos son números del PRD, no gustos.
const (
	// DefaultHorizon es la ventana que materializa el resolver (PRD §9 paso 3).
	DefaultHorizon = 48 * time.Hour
	// MaxFillerExcess es lo que se permite exceder un hueco cuando no hay
	// combinación exacta de relleno (PRD §14.1).
	MaxFillerExcess = 5 * time.Second
	// FillerFadeMs es el fundido con que se recorta el último clip de relleno.
	FillerFadeMs = 1000
	// GapNotice es el hueco a partir del cual se avisa que esa hora no está
	// programada, para que la parrilla pueda ofrecer "llenar con diferido".
	GapNotice = 30 * time.Minute
	// normalizeReady es el valor de media_asset.estado_normalizacion que
	// deja pasar un archivo al plan.
	normalizeReady = "listo"
)

// expiryNotices son los umbrales de aviso de vencimiento (auditoría B10).
var expiryNotices = []int{30, 14, 7}

// WarningKind es el tipo de aviso. Son los seis casos que el resolver sabe
// contar; la interfaz los agrupa por aquí y el texto lo lee una persona.
type WarningKind string

const (
	// WarnOverbook — no cabe lo que la regla pide antes del siguiente inicio duro.
	WarnOverbook WarningKind = "sobrecupo"
	// WarnGap — un tramo de aire sin nada programado (se llenó con relleno).
	WarnGap WarningKind = "hueco"
	// WarnExpiry — una regla se está acabando (30, 14 o 7 días).
	WarnExpiry WarningKind = "vencimiento"
	// WarnNoFiller — no hubo relleno para cubrir un hueco; fue al cartel.
	WarnNoFiller WarningKind = "sin_relleno"
	// WarnNoMaterial — el título no tiene archivo listo para aire.
	WarnNoMaterial WarningKind = "sin_material"
	// WarnBadRule — la regla no se puede programar tal como está.
	WarnBadRule WarningKind = "regla_invalida"
)

// Warning es un aviso del resolver. Text está escrito para que lo entienda
// quien programa el canal, no quien escribió el código.
type Warning struct {
	Kind   WarningKind `json:"tipo"`
	Text   string      `json:"texto"`
	RuleID *int64      `json:"schedule_rule_id,omitempty"`
	Day    model.Day   `json:"dia_emision,omitempty"`
	At     time.Time   `json:"instante,omitempty"`
	// Notice solo se llena en los avisos de vencimiento: "30", "14" o "7".
	// Es lo que el store guardaría en schedule_rule.ultimo_aviso_enviado.
	Notice string `json:"aviso,omitempty"`
}

// Input es todo lo que el resolver necesita saber. Nada de esto lo busca él:
// se lo da el store.
type Input struct {
	Channel  model.Channel
	Rules    []model.ScheduleRule
	Titles   map[int64]model.Title
	Episodes map[int64][]model.Episode  // por title, ordenados temporada/número
	Assets   map[int64]model.MediaAsset // por id; solo entra al plan lo listo
	Fillers  []model.FillerAsset
	// Existing es lo que ya está guardado en la ventana: lo cued/aired y lo
	// que una persona fijó a mano. Nada de eso se toca.
	Existing []model.PlanItem
	Decks    map[model.DeckKind]int64
	Now      time.Time
	Horizon  time.Duration // 48 h por defecto
}

// Output es el plan deseado para la ventana. Items trae solo lo nuevo, en
// estado planned y ordenado por instante.
type Output struct {
	Items    []model.PlanItem
	Warnings []Warning
	// EpisodeAdvance dice, por regla dueña del contador, cuál sería el último
	// episodio emitido si el plan sale tal cual. El store lo guarda en
	// schedule_rule.ultimo_episodio_emitido cuando el ítem sale al aire.
	EpisodeAdvance map[int64]int64
	// CounterOwner dice, para cada regla que puso algo, qué regla es la dueña
	// del contador de episodios: la propia regla, o su primaria si es una
	// repetición (repite_a). Un plan_item lleva su schedule_rule_id, así que
	// con este mapa se sabe siempre en qué regla se escribe
	// ultimo_episodio_emitido cuando ese ítem sale al aire: nunca en la de
	// repetición, que no tiene contador propio (auditoría B5).
	CounterOwner map[int64]int64
}

type interval struct {
	from, to time.Time
}

// instance es una corrida de una regla en un día de emisión concreto.
type instance struct {
	rule    model.ScheduleRule
	day     model.Day
	start   time.Time
	hardEnd time.Time // el siguiente inicio duro, o el fin del slot
}

type run struct {
	in      Input
	ch      model.Channel
	loc     *time.Location
	start   time.Time
	end     time.Time
	today   model.Day
	byID    map[int64]model.ScheduleRule
	relieve map[int64]bool // reglas que ya tienen quien las releve

	reserved   []interval
	items      []model.PlanItem
	warns      []Warning
	advance    map[int64]int64
	owner      map[int64]int64            // regla → regla dueña de su contador
	cursor     map[int64]*int64           // contador por regla dueña de la serie
	airedToday map[int64][]model.Episode  // lo que puso hoy cada regla
	assets     map[int64]model.MediaAsset // los mismos, para leer sin miedo
}

// Resolve arma el plan de la ventana. No toca nada de lo que recibe.
func Resolve(in Input) Output {
	r := newRun(in)
	r.expiryWarnings()

	insts := r.instances()
	r.resolveHardEnds(insts)

	byDay := map[model.Day][]instance{}
	days := []model.Day{}
	for _, it := range insts {
		if _, seen := byDay[it.day]; !seen {
			days = append(days, it.day)
		}
		byDay[it.day] = append(byDay[it.day], it)
	}
	sort.Slice(days, func(i, j int) bool { return days[i] < days[j] })

	for _, d := range days {
		r.airedToday = map[int64][]model.Episode{}
		day := byDay[d]
		// Primero lo normal, después las repeticiones (necesitan saber qué
		// puso su primaria hoy) y al final el diferido (copia lo del día).
		for _, it := range day {
			if it.rule.RepeatsOf == nil && it.rule.Kind != model.RuleTimeShift {
				r.place(it)
			}
		}
		for _, it := range day {
			if it.rule.RepeatsOf != nil {
				r.place(it)
			}
		}
		for _, it := range day {
			if it.rule.RepeatsOf == nil && it.rule.Kind == model.RuleTimeShift {
				r.place(it)
			}
		}
	}

	r.fillGaps()

	sort.SliceStable(r.items, func(i, j int) bool {
		return r.items[i].PlannedAt.Before(r.items[j].PlannedAt)
	})
	sort.SliceStable(r.warns, func(i, j int) bool {
		a, b := r.warns[i], r.warns[j]
		if !a.At.Equal(b.At) {
			return a.At.Before(b.At)
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Text < b.Text
	})
	return Output{Items: r.items, Warnings: r.warns, EpisodeAdvance: r.advance, CounterOwner: r.owner}
}

func newRun(in Input) *run {
	ch := in.Channel
	loc := ch.Location()
	horizon := in.Horizon
	if horizon <= 0 {
		horizon = DefaultHorizon
	}
	start := in.Now.Truncate(time.Second)
	r := &run{
		in:      in,
		ch:      ch,
		loc:     loc,
		start:   start,
		end:     start.Add(horizon),
		today:   ch.BroadcastDay(start),
		byID:    map[int64]model.ScheduleRule{},
		relieve: map[int64]bool{},
		advance: map[int64]int64{},
		owner:   map[int64]int64{},
		cursor:  map[int64]*int64{},
		assets:  in.Assets,
	}
	for _, rule := range in.Rules {
		r.byID[rule.ID] = rule
		if rule.HandsOffTo != nil {
			r.relieve[*rule.HandsOffTo] = true
		}
		last := rule.LastEpisodeAired
		r.cursor[rule.ID] = last
	}
	for _, it := range in.Existing {
		if ocupaElAire(it) {
			r.reserved = append(r.reserved, interval{it.PlannedAt, it.End()})
		}
	}
	sort.Slice(r.reserved, func(i, j int) bool { return r.reserved[i].from.Before(r.reserved[j].from) })
	return r
}

// ── instantes y horario de verano ─────────────────────────────────────

// localInstant devuelve el instante de una hora de pared, con la política de
// §15: la hora que no existe se corre a la siguiente válida; la que existe
// dos veces sale en la primera.
func localInstant(loc *time.Location, y int, m time.Month, d int, min model.Minutes) time.Time {
	want := int(min)
	for add := 0; add <= 180; add++ {
		w := want + add
		if w >= 24*60 {
			break
		}
		t := time.Date(y, m, d, w/60, w%60, 0, 0, loc)
		if t.Hour()*60+t.Minute() != w || t.Day() != d {
			continue // esa hora de pared no existe ese día: se corre
		}
		// Si existe dos veces, la primera es la que sale.
		for _, back := range []time.Duration{time.Hour, 30 * time.Minute} {
			e := t.Add(-back)
			if e.Hour()*60+e.Minute() == w && e.Day() == d {
				t = e
				break
			}
		}
		return t
	}
	return time.Date(y, m, d, want/60, want%60, 0, 0, loc)
}

// instantOf devuelve el instante en que una hora local cae dentro de un día
// de emisión: lo anterior al inicio del día vive en el calendario siguiente.
func (r *run) instantOf(day model.Day, min model.Minutes) time.Time {
	base := day.Time(r.loc)
	if min < r.ch.BroadcastDayAt {
		base = base.AddDate(0, 0, 1)
	}
	return localInstant(r.loc, base.Year(), base.Month(), base.Day(), min)
}

// ── reglas → corridas ─────────────────────────────────────────────────

func (r *run) instances() []instance {
	first := r.ch.BroadcastDay(r.start).Add(-1)
	last := r.ch.BroadcastDay(r.end).Add(1)

	rules := append([]model.ScheduleRule(nil), r.in.Rules...)
	sort.Slice(rules, func(i, j int) bool { return rules[i].ID < rules[j].ID })

	var out []instance
	for _, rule := range rules {
		if rule.ChannelID != 0 && r.ch.ID != 0 && rule.ChannelID != r.ch.ID {
			continue
		}
		if !rule.Active {
			continue // apagada a propósito: no es un problema
		}
		if msg := r.ruleProblem(rule); msg != "" {
			id := rule.ID
			r.warns = append(r.warns, Warning{
				Kind: WarnBadRule, Text: msg, RuleID: &id, Day: r.today, At: r.start,
			})
			continue
		}
		for d := first; d <= last; d = d.Add(1) {
			if !rule.Covers(d) {
				continue
			}
			out = append(out, instance{rule: rule, day: d, start: r.instantOf(d, rule.At)})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].start.Equal(out[j].start) {
			return out[i].start.Before(out[j].start)
		}
		return out[i].rule.ID < out[j].rule.ID
	})
	return r.dropInsideBlocks(r.resolveClashes(out))
}

// ruleProblem devuelve el motivo, en cristiano, por el que una regla no se
// puede programar. Vacío significa que está bien.
func (r *run) ruleProblem(rule model.ScheduleRule) string {
	if rule.To < rule.From {
		return fmt.Sprintf("la regla termina (%s) antes de empezar (%s): no se programa nada", rule.To, rule.From)
	}
	if !rule.Days.Valid() {
		return fmt.Sprintf("el patrón de días %q no tiene la forma LMMJVSD", string(rule.Days))
	}
	if rule.At < 0 || rule.At > 1439 {
		return "la hora de la regla está fuera del día"
	}
	if rule.RepeatsOf != nil {
		if _, ok := r.byID[*rule.RepeatsOf]; !ok {
			return fmt.Sprintf("repite a una regla que no existe (%d)", *rule.RepeatsOf)
		}
	}
	switch rule.Kind {
	case model.RuleTimeShift:
		if rule.SourceWindowStart == nil || rule.SourceWindowEnd == nil {
			return "el diferido no dice qué ventana repite"
		}
	case model.RuleLive:
		if rule.LiveSourceID == nil {
			return "el bloque en vivo no dice de qué fuente viene"
		}
	default:
		if rule.TitleID == nil && rule.LiveSourceID == nil {
			return "la regla no dice qué título emite"
		}
	}
	return ""
}

// resolveClashes deja una sola regla por instante de arranque. Un relevo no
// es un conflicto; dos reglas sueltas a la misma hora sí, y gana la de menor
// id (PRD §9 paso 3).
func (r *run) resolveClashes(in []instance) []instance {
	var out []instance
	for i := 0; i < len(in); {
		j := i
		for j < len(in) && in[j].start.Equal(in[i].start) {
			j++
		}
		group := in[i:j]
		if len(group) == 1 {
			out = append(out, group[0])
			i = j
			continue
		}
		ids := map[int64]bool{}
		for _, g := range group {
			ids[g.rule.ID] = true
		}
		var relievers []instance
		for _, g := range group {
			if g.rule.HandsOffTo != nil && *g.rule.HandsOffTo != g.rule.ID && ids[*g.rule.HandsOffTo] {
				relievers = append(relievers, g)
			}
		}
		winner := group[0] // ya vienen ordenadas por id
		if len(relievers) == 1 {
			winner = relievers[0]
		} else {
			var names []string
			for _, g := range group {
				names = append(names, r.titleName(g.rule))
			}
			id := winner.rule.ID
			r.warns = append(r.warns, Warning{
				Kind: WarnBadRule,
				Text: fmt.Sprintf("%s y %s empiezan las dos a las %s del %s y ninguna releva a la otra; sale %s",
					names[0], names[1], group[0].start.In(r.loc).Format("15:04"), group[0].day, r.titleName(winner.rule)),
				RuleID: &id, Day: group[0].day, At: group[0].start,
			})
		}
		out = append(out, winner)
		i = j
	}
	return out
}

// blockEnd devuelve el fin declarado de una corrida que trae duración de slot
// —un bloque en vivo o un bloque arrendado— y si lo trae. Ese fin es suyo: no
// se lo recorta nadie.
func (it instance) blockEnd() (time.Time, bool) {
	if it.rule.SlotMs <= 0 {
		return time.Time{}, false
	}
	return it.start.Add(time.Duration(it.rule.SlotMs) * time.Millisecond), true
}

// resolveHardEnds fija el fin duro de cada corrida: su duración de slot, o el
// arranque de la siguiente regla si llega antes.
func (r *run) resolveHardEnds(insts []instance) {
	for i := range insts {
		// Sin duración de slot, la regla es dueña del aire hasta que arranca
		// la siguiente: por eso un programa de tres horas desde las 5:00 AM
		// cruza el inicio del día de emisión sin que nadie lo corte.
		end := r.end
		bloque, esBloque := insts[i].blockEnd()
		if esBloque {
			// Un bloque con hora de fin declarada dura lo que dice. Que otra
			// regla arranque dentro no lo recorta: eso es un conflicto, y se
			// avisa aparte (F1-39).
			end = bloque
		} else if i+1 < len(insts) {
			if next := insts[i+1].start; next.After(insts[i].start) && next.Before(end) {
				end = next
			}
		}
		// El fin de un bloque en vivo es tan duro por dentro como por fuera:
		// lo que arranque dentro de su ventana tiene que terminar antes.
		for j := range insts {
			otro, ok := insts[j].blockEnd()
			if !ok || j == i {
				continue
			}
			if insts[i].start.After(insts[j].start) && insts[i].start.Before(otro) && otro.Before(end) {
				end = otro
			}
		}
		// Lo que ya está cargado, ya salió o lo fijó una persona también es
		// un inicio duro.
		for _, iv := range r.reserved {
			if !iv.from.Before(insts[i].start) && iv.from.Before(end) {
				end = iv.from
			}
		}
		insts[i].hardEnd = end
	}
}

// dropInsideBlocks quita las corridas que arrancan dentro de un bloque con
// hora de fin declarada (en vivo o arrendado). El bloque manda y sale entero:
// meterle otra regla encima sería dos cosas al aire a la vez. Se avisa en
// cristiano para que quien programa mueva la regla (F1-39).
func (r *run) dropInsideBlocks(in []instance) []instance {
	var out []instance
	for i := range in {
		dentroDe := -1
		for j := range in {
			fin, ok := in[j].blockEnd()
			if !ok || j == i {
				continue
			}
			if in[i].start.After(in[j].start) && in[i].start.Before(fin) {
				dentroDe = j
				break
			}
		}
		if dentroDe < 0 {
			out = append(out, in[i])
			continue
		}
		if in[i].start.Before(r.start) || !in[i].start.Before(r.end) {
			continue // fuera de la ventana que se materializa: no se avisa dos veces
		}
		bloque := in[dentroDe]
		fin, _ := bloque.blockEnd()
		id := in[i].rule.ID
		r.warns = append(r.warns, Warning{
			Kind: WarnBadRule,
			Text: fmt.Sprintf("%s empieza a las %s del %s, cuando todavía está al aire %s (de %s a %s): manda el bloque, que sale entero, y esa regla no se programa",
				r.titleName(in[i].rule), in[i].start.In(r.loc).Format("15:04"), in[i].day,
				r.titleName(bloque.rule), bloque.start.In(r.loc).Format("15:04"), fin.In(r.loc).Format("15:04")),
			RuleID: &id, Day: in[i].day, At: in[i].start,
		})
	}
	return out
}

func (r *run) titleName(rule model.ScheduleRule) string {
	if rule.Kind == model.RuleTimeShift {
		return "el diferido"
	}
	if rule.TitleID != nil {
		if t, ok := r.in.Titles[*rule.TitleID]; ok && t.Name != "" {
			return t.Name
		}
	}
	if rule.LiveSourceID != nil {
		return "el bloque en vivo"
	}
	return fmt.Sprintf("la regla %d", rule.ID)
}

// ── colocar ───────────────────────────────────────────────────────────

func (r *run) place(it instance) {
	if it.start.Before(r.start) || !it.start.Before(r.end) {
		return // fuera de la ventana que se materializa
	}
	if !it.hardEnd.After(it.start) {
		return
	}
	if r.busy(it.start) {
		return // ese instante ya lo tiene algo cued o aired
	}
	switch {
	case it.rule.Kind == model.RuleTimeShift:
		r.placeTimeShift(it)
	case it.rule.Kind == model.RuleLive || it.rule.LiveSourceID != nil:
		r.placeLive(it)
	default:
		r.placeProgram(it)
	}
}

// ocupaElAire dice si un ítem que ya está guardado le quita el sitio al
// resolver: lo que está cargado o ya salió, y lo que una persona fijó a mano
// (F1-26). Lo fijado es tan duro como un bloque en vivo: el resolver lo deja
// donde está, no avisa de ello y rellena a su alrededor.
func ocupaElAire(p model.PlanItem) bool {
	if p.State == model.Skipped || p.State == model.Failed {
		return false
	}
	return p.State == model.Cued || p.State == model.Aired || p.Fijado
}

func (r *run) busy(t time.Time) bool {
	for _, iv := range r.reserved {
		if !t.Before(iv.from) && t.Before(iv.to) {
			return true
		}
	}
	return false
}

func (r *run) placeLive(it instance) {
	id := it.rule.ID
	r.add(model.PlanItem{
		DeckID:       r.deck(model.DeckProgram),
		RuleID:       &id,
		PlannedAt:    it.start,
		PlannedMs:    it.hardEnd.Sub(it.start).Milliseconds(),
		Origin:       "live_source",
		LiveSourceID: it.rule.LiveSourceID,
	})
}

func (r *run) placeProgram(it instance) {
	rule := it.rule
	owner := rule.ID
	primary := rule
	if rule.RepeatsOf != nil {
		if p, ok := r.byID[*rule.RepeatsOf]; ok {
			primary = p
			owner = p.ID
		}
	}

	var picks []model.Episode
	repeat := false
	if rule.RepeatsOf != nil {
		if same, ok := r.airedToday[primary.ID]; ok && len(same) > 0 {
			// La primaria ya puso su episodio hoy: la repetición pone el mismo
			// y no avanza ningún contador (auditoría B5).
			picks = same
			repeat = true
		}
	}
	titleID := rule.TitleID
	if rule.RepeatsOf != nil && primary.TitleID != nil {
		titleID = primary.TitleID
	}
	if titleID == nil {
		return
	}

	if !repeat {
		var ok bool
		picks, ok = r.pickEpisodes(owner, *titleID, rule.EpisodesPerRun)
		if !ok {
			id := rule.ID
			r.warns = append(r.warns, Warning{
				Kind:   WarnNoMaterial,
				Text:   fmt.Sprintf("%s no tiene archivo listo para aire: las %s del %s van a relleno", r.titleName(rule), it.start.In(r.loc).Format("15:04"), it.day),
				RuleID: &id, Day: it.day, At: it.start,
			})
			return
		}
	}

	cursor := it.start
	placed := 0
	var last *model.Episode
	for i := range picks {
		ep := picks[i]
		asset, ok := r.readyAsset(ep.MediaAssetID)
		if !ok {
			continue
		}
		end := cursor.Add(time.Duration(asset.DurationMs) * time.Millisecond)
		if end.After(it.hardEnd) {
			break // el reloj manda: no se arranca lo que no termina a tiempo
		}
		ruleID := rule.ID
		epID := ep.ID
		assetID := asset.ID
		item := model.PlanItem{
			DeckID:       r.deck(model.DeckProgram),
			RuleID:       &ruleID,
			PlannedAt:    cursor,
			PlannedMs:    asset.DurationMs,
			Origin:       "asset",
			MediaAssetID: &assetID,
		}
		if ep.ID != 0 {
			item.EpisodeID = &epID
		}
		r.add(item)
		cursor = end
		placed++
		e := ep
		last = &e
	}

	if placed < len(picks) {
		id := rule.ID
		r.warns = append(r.warns, Warning{
			Kind: WarnOverbook,
			Text: fmt.Sprintf("de %d episodios de %s caben %d antes de las %s: el resto se rellena",
				len(picks), r.titleName(rule), placed, it.hardEnd.In(r.loc).Format("15:04")),
			RuleID: &id, Day: it.day, At: it.start,
		})
	}
	if placed == 0 {
		return
	}
	// Quede claro en qué regla se escribe el contador cuando esto salga al
	// aire: en la primaria si esto es una repetición (auditoría B5).
	r.owner[rule.ID] = owner
	if !repeat && last != nil && last.ID != 0 {
		r.cursor[owner] = &last.ID
		r.advance[owner] = last.ID
	}
	if rule.RepeatsOf == nil {
		r.airedToday[rule.ID] = picks[:placed]
	}
}

// pickEpisodes toma los siguientes n episodios con material listo a partir
// del contador de la regla dueña, dando la vuelta al final de la serie.
func (r *run) pickEpisodes(owner, titleID int64, n int) ([]model.Episode, bool) {
	if n < 1 {
		n = 1
	}
	eps := append([]model.Episode(nil), r.in.Episodes[titleID]...)
	sort.SliceStable(eps, func(i, j int) bool {
		if eps[i].Season != eps[j].Season {
			return eps[i].Season < eps[j].Season
		}
		if eps[i].Number != eps[j].Number {
			return eps[i].Number < eps[j].Number
		}
		return eps[i].ID < eps[j].ID
	})
	if len(eps) == 0 {
		// Una película o un programa suelto: el archivo cuelga del título.
		t, ok := r.in.Titles[titleID]
		if !ok || t.MediaAssetID == nil {
			return nil, false
		}
		if _, ok := r.readyAsset(t.MediaAssetID); !ok {
			return nil, false
		}
		return []model.Episode{{TitleID: titleID, MediaAssetID: t.MediaAssetID}}, true
	}

	from := 0
	if last := r.cursor[owner]; last != nil {
		for i, e := range eps {
			if e.ID == *last {
				from = i + 1
				break
			}
		}
	}
	var out []model.Episode
	skipped := 0
	for i := 0; len(out) < n && skipped < len(eps); i++ {
		ep := eps[(from+i)%len(eps)]
		if _, ok := r.readyAsset(ep.MediaAssetID); !ok {
			skipped++
			continue
		}
		skipped = 0
		out = append(out, ep)
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

func (r *run) readyAsset(id *int64) (model.MediaAsset, bool) {
	if id == nil {
		return model.MediaAsset{}, false
	}
	a, ok := r.assets[*id]
	if !ok {
		return model.MediaAsset{}, false
	}
	if a.State != model.AssetReady || a.NormalizeState != normalizeReady || a.DurationMs <= 0 {
		return model.MediaAsset{}, false
	}
	return a, true
}

// placeTimeShift reprograma los archivos del deck programa de la ventana de
// origen del mismo día de emisión (PRD §9 paso 8). No reproduce grabación, no
// copia relleno ni otro diferido, y una ventana sin programa no produce nada.
func (r *run) placeTimeShift(it instance) {
	rule := it.rule
	from := r.instantOf(it.day, *rule.SourceWindowStart)
	to := r.instantOf(it.day, *rule.SourceWindowEnd)
	if !to.After(from) {
		to = to.AddDate(0, 0, 1)
	}

	seen := map[string]bool{}
	var src []model.PlanItem
	collect := func(items []model.PlanItem) {
		for _, p := range items {
			if p.Origin != "asset" || p.MediaAssetID == nil {
				continue // relleno, cartel y vivo no se difieren en F1
			}
			if p.PlannedAt.Before(from) || !p.PlannedAt.Before(to) {
				continue
			}
			if r.ch.BroadcastDay(p.PlannedAt) != it.day {
				continue
			}
			if p.RuleID != nil {
				if o, ok := r.byID[*p.RuleID]; ok && o.Kind == model.RuleTimeShift {
					continue // nunca hay recursión
				}
			}
			key := fmt.Sprintf("%d|%d", p.PlannedAt.UnixMilli(), *p.MediaAssetID)
			if seen[key] {
				continue
			}
			seen[key] = true
			src = append(src, p)
		}
	}
	collect(r.items)
	var existing []model.PlanItem
	for _, p := range r.in.Existing {
		if ocupaElAire(p) {
			existing = append(existing, p)
		}
	}
	collect(existing)
	if len(src) == 0 {
		return // una ventana sin programa no produce diferido (auditoría B9)
	}
	sort.Slice(src, func(i, j int) bool { return src[i].PlannedAt.Before(src[j].PlannedAt) })

	cursor := it.start
	placed := 0
	for _, p := range src {
		end := cursor.Add(time.Duration(p.PlannedMs) * time.Millisecond)
		if end.After(it.hardEnd) {
			break
		}
		id := rule.ID
		item := model.PlanItem{
			DeckID:       r.deck(model.DeckProgram),
			RuleID:       &id,
			PlannedAt:    cursor,
			PlannedMs:    p.PlannedMs,
			Origin:       "asset",
			MediaAssetID: p.MediaAssetID,
			EpisodeID:    p.EpisodeID,
		}
		r.add(item)
		cursor = end
		placed++
	}
	if placed < len(src) {
		id := rule.ID
		r.warns = append(r.warns, Warning{
			Kind: WarnOverbook,
			Text: fmt.Sprintf("el diferido de las %s repite %d de los %d programas de la ventana: el resto no cabe antes de las %s",
				it.start.In(r.loc).Format("15:04"), placed, len(src), it.hardEnd.In(r.loc).Format("15:04")),
			RuleID: &id, Day: it.day, At: it.start,
		})
	}
}

// ── huecos ────────────────────────────────────────────────────────────

func (r *run) fillGaps() {
	busy := make([]interval, 0, len(r.items)+len(r.reserved))
	for _, p := range r.items {
		busy = append(busy, interval{p.PlannedAt, p.End()})
	}
	busy = append(busy, r.reserved...)
	sort.Slice(busy, func(i, j int) bool { return busy[i].from.Before(busy[j].from) })

	cursor := r.start
	var gaps []interval
	for _, iv := range busy {
		if iv.from.After(cursor) {
			end := iv.from
			if end.After(r.end) {
				end = r.end
			}
			if end.After(cursor) {
				gaps = append(gaps, interval{cursor, end})
			}
		}
		if iv.to.After(cursor) {
			cursor = iv.to
		}
		if !cursor.Before(r.end) {
			break
		}
	}
	if cursor.Before(r.end) {
		gaps = append(gaps, interval{cursor, r.end})
	}
	for _, g := range gaps {
		r.fill(g)
	}
}

func (r *run) fill(g interval) {
	length := g.to.Sub(g.from)
	if length <= 0 {
		return
	}
	day := r.ch.BroadcastDay(g.from)
	if length >= GapNotice {
		r.warns = append(r.warns, Warning{
			Kind: WarnGap,
			Text: fmt.Sprintf("de %s a %s del %s no hay nada programado (%s); se llenó con relleno",
				g.from.In(r.loc).Format("15:04"), g.to.In(r.loc).Format("15:04"), day, humanDuration(length)),
			Day: day, At: g.from,
		})
	}

	plan := packFillers(length.Milliseconds(), r.usableFillers())
	cursor := g.from
	for i, clip := range plan.clips {
		ms := clip.DurationMs
		fundido := int64(0)
		if i == len(plan.clips)-1 && plan.trim > 0 {
			ms -= plan.trim
			// Solo el clip recortado sale con fundido de 1 s (PRD §14.1): el
			// resto del relleno sale entero, y el plan lo dice para que el
			// motor no tenga que adivinarlo.
			fundido = FillerFadeMs
		}
		if ms <= 0 {
			continue
		}
		assetID := clip.MediaAssetID
		r.add(model.PlanItem{
			DeckID:       r.deck(model.DeckFiller),
			PlannedAt:    cursor,
			PlannedMs:    ms,
			Origin:       "relleno",
			MediaAssetID: &assetID,
			FadeOutMs:    fundido,
		})
		cursor = cursor.Add(time.Duration(ms) * time.Millisecond)
	}
	if plan.short > 0 {
		r.warns = append(r.warns, Warning{
			Kind: WarnNoFiller,
			Text: fmt.Sprintf("no hubo relleno para %s a las %s del %s: sale el cartel de la estación",
				humanDuration(time.Duration(plan.short)*time.Millisecond), cursor.In(r.loc).Format("15:04"), day),
			Day: day, At: cursor,
		})
		r.add(model.PlanItem{
			DeckID:    r.deck(model.DeckFiller),
			PlannedAt: cursor,
			PlannedMs: plan.short,
			Origin:    "cartel",
		})
	}
}

func (r *run) usableFillers() []model.FillerAsset {
	var out []model.FillerAsset
	for _, f := range r.in.Fillers {
		if f.DurationMs <= 0 {
			continue
		}
		if f.ChannelID != nil && r.ch.ID != 0 && *f.ChannelID != r.ch.ID {
			continue
		}
		if a, ok := r.assets[f.MediaAssetID]; ok {
			if a.State != model.AssetReady || a.NormalizeState != normalizeReady {
				continue
			}
		}
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].DurationMs != out[j].DurationMs {
			return out[i].DurationMs > out[j].DurationMs
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// ── vencimientos ──────────────────────────────────────────────────────

func (r *run) expiryWarnings() {
	rules := append([]model.ScheduleRule(nil), r.in.Rules...)
	sort.Slice(rules, func(i, j int) bool { return rules[i].ID < rules[j].ID })
	for _, rule := range rules {
		if !rule.Active || rule.To < rule.From {
			continue
		}
		if r.relieve[rule.ID] {
			continue // ya hay quien la releve: avisar entrena a ignorar avisos
		}
		left := rule.DaysLeft(r.today)
		if left < 0 {
			continue
		}
		notice := 0
		for _, n := range expiryNotices {
			if left <= n && (notice == 0 || n < notice) {
				notice = n
			}
		}
		if notice == 0 {
			continue
		}
		if rule.LastNoticeSent == fmt.Sprint(notice) {
			continue // ya se avisó de este umbral
		}
		id := rule.ID
		r.warns = append(r.warns, Warning{
			Kind: WarnExpiry,
			Text: fmt.Sprintf("%s de las %s se acaba el %s: quedan %d días y no hay quien lo releve",
				r.titleName(rule), rule.At, rule.To, left),
			RuleID: &id, Day: r.today, At: r.start, Notice: fmt.Sprint(notice),
		})
	}
}

// ── ayudas ────────────────────────────────────────────────────────────

func (r *run) deck(kind model.DeckKind) int64 { return r.in.Decks[kind] }

func (r *run) add(item model.PlanItem) {
	item.ChannelID = r.ch.ID
	item.State = model.Planned
	item.BroadcastDay = r.ch.BroadcastDay(item.PlannedAt)
	item.LocalClock = item.PlannedAt.In(r.loc).Format("15:04")
	r.items = append(r.items, item)
}

func humanDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	switch {
	case h > 0 && m > 0:
		return fmt.Sprintf("%d h %d min", h, m)
	case h > 0:
		return fmt.Sprintf("%d h", h)
	case m > 0 && s > 0:
		return fmt.Sprintf("%d min %d s", m, s)
	case m > 0:
		return fmt.Sprintf("%d min", m)
	default:
		return fmt.Sprintf("%d s", s)
	}
}
