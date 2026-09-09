package importer

import (
	"os"
	"strings"
	"testing"

	"antena787/internal/model"
)

// La hoja real de CAtv, tal como la mandó Rolando el 4 de septiembre de 2026.
const hojaPath = "../../docs/catv-sheet-2026-09-04.md"

// catv es el canal de la demostración: día de emisión a las 6:00 AM, que es
// lo que hace que las reglas de madrugada corran sus fechas un día atrás.
var catv = model.Channel{
	ID:             1,
	Name:           "Caribbean Advantage TV",
	Kind:           model.ChannelTV,
	TimeZone:       "America/Puerto_Rico",
	BroadcastDayAt: 6 * 60,
	CallSign:       "CAtv",
}

func hoja(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(hojaPath)
	if err != nil {
		t.Fatalf("no pude leer la hoja de CAtv: %v", err)
	}
	return string(b)
}

func reglasDeLaHoja(t *testing.T) Sheet {
	t.Helper()
	s, errs := Parse(hoja(t))
	if len(errs) != 0 {
		t.Fatalf("Parse devolvió errores: %v", errs)
	}
	if s.Kind != KindRules {
		t.Fatalf("Parse eligió la tabla %q, se esperaba %q", s.Kind, KindRules)
	}
	return s
}

// buscar devuelve la regla con ese Id de la hoja.
func buscar(t *testing.T, res Result, sheetID string) (int, model.ScheduleRule) {
	t.Helper()
	for i, id := range res.SheetIDs {
		if id == sheetID {
			return i, res.Rules[i]
		}
	}
	t.Fatalf("no encontré la regla con Id %s", sheetID)
	return -1, model.ScheduleRule{}
}

func TestParseReconoceLasTresTablas(t *testing.T) {
	sheets := ParseAll(hoja(t))
	var reglas, catalogo, parrillas int
	for _, s := range sheets {
		switch s.Kind {
		case KindRules:
			reglas++
		case KindCatalog:
			catalogo++
		case KindGrid:
			parrillas++
		}
	}
	if reglas != 1 || catalogo != 1 || parrillas < 1 {
		t.Fatalf("tablas reconocidas: %d de reglas, %d de catálogo, %d de parrilla", reglas, catalogo, parrillas)
	}
	// Parse, sin ayuda, elige la de reglas.
	s, _ := Parse(hoja(t))
	if s.Kind != KindRules {
		t.Fatalf("Parse eligió %q", s.Kind)
	}
	if len(s.Rows) != 35 {
		t.Fatalf("la tabla de reglas trae %d filas, se esperaban 35", len(s.Rows))
	}
}

func TestReglasDeLaHojaReal(t *testing.T) {
	res := Rules(reglasDeLaHoja(t), catv, WithLiveNames("RadioOnce Live!"))

	if len(res.Rules) != 34 {
		t.Fatalf("se importaron %d reglas, se esperaban 34 (%s)", len(res.Rules), res.Summary())
	}
	if len(res.RowErrors) != 1 {
		t.Fatalf("filas que no cuadraron: %d, se esperaba 1: %v", len(res.RowErrors), res.RowErrors)
	}

	// Hellsing: fin antes que inicio. Ni se importa ni tumba la hoja.
	e := res.RowErrors[0]
	if e.SheetID != "336" || e.Title != "Hellsing" {
		t.Fatalf("la fila que no cuadró es la %s (%s), se esperaba la 336 (Hellsing)", e.SheetID, e.Title)
	}
	want := "la fecha de fin (1 ene 2026) es anterior a la de inicio (15 sep 2026)"
	if e.Reason != want {
		t.Fatalf("motivo:\n  %s\nse esperaba:\n  %s", e.Reason, want)
	}
	for _, id := range res.SheetIDs {
		if id == "336" {
			t.Fatal("Hellsing se importó y no debía")
		}
	}

	// Familia Robinson terminaba el 6 de septiembre: va a las 9:00 AM, así que
	// su fecha no se corre.
	if _, r := buscar(t, res, "273"); r.To != model.Day("2026-09-06") {
		t.Fatalf("Familia Robinson termina %s, se esperaba 2026-09-06", r.To)
	}

	// Magic Knight Rayearth de las 12:00 AM: corrida un día atrás.
	i, r313 := buscar(t, res, "313")
	if r313.From != "2026-07-01" || r313.To != "2026-09-07" {
		t.Fatalf("313 va de %s a %s, se esperaba de 2026-07-01 a 2026-09-07", r313.From, r313.To)
	}
	// El patrón también se corre: "_MMJVS_" es martes a sábado de calendario,
	// que son lunes a viernes de día de emisión.
	if r313.Days != model.DayPattern("LMMJV__") {
		t.Fatalf("el patrón de 313 es %q, se esperaba LMMJV__", r313.Days)
	}
	if !r313.Days.Valid() {
		t.Fatalf("el patrón de 313 no pasa Valid()")
	}
	if r313.At != 0 {
		t.Fatalf("313 va a las %s, se esperaba 12:00 AM", FormatClock(r313.At))
	}
	if res.TitleFor(i).Name != "Magic Knight Rayearth" {
		t.Fatalf("el título de 313 es %q", res.TitleFor(i).Name)
	}

	// Zoids de las 12:00 AM: también corrida.
	if _, r := buscar(t, res, "353"); r.From != "2026-09-08" || r.To != "2026-12-09" || r.Days != "LMMJV__" {
		t.Fatalf("353 va de %s a %s los %s, se esperaba de 2026-09-08 a 2026-12-09 los LMMJV__", r.From, r.To, r.Days)
	}
	// Mazinger Z de las 12:30 AM se corre igual que Magic Knight.
	if _, r := buscar(t, res, "304"); r.From != "2026-06-10" || r.To != "2026-10-15" || r.Days != "LMMJV__" {
		t.Fatalf("304 va de %s a %s los %s", r.From, r.To, r.Days)
	}
	// Y "L_____D" de madrugada es sábado y domingo de día de emisión: el
	// lunes de calendario cae en el domingo de emisión.
	_, r333 := buscar(t, res, "333")
	if r333.Days != model.DayPattern("_____SD") {
		t.Fatalf("el patrón de 333 es %q, se esperaba _____SD", r333.Days)
	}
	if !r333.Days.Valid() {
		t.Fatal("el patrón corrido de 333 no pasa Valid()")
	}
	if r333.From != "2026-08-07" || r333.To != "2026-09-13" {
		t.Fatalf("333 va de %s a %s", r333.From, r333.To)
	}

	// Y el corrimiento se reporta fila por fila, con el antes y el después.
	if len(res.DatesShifted) != 4 {
		t.Fatalf("fechas corridas: %d, se esperaban 4 (313, 304, 333 y 353): %+v", len(res.DatesShifted), res.DatesShifted)
	}
	var s313 *DateShift
	for i := range res.DatesShifted {
		if res.DatesShifted[i].SheetID == "313" {
			s313 = &res.DatesShifted[i]
		}
	}
	if s313 == nil {
		t.Fatal("no se reportó el corrimiento de 313")
	}
	if s313.FromBefore != "2026-07-02" || s313.FromAfter != "2026-07-01" ||
		s313.ToBefore != "2026-09-08" || s313.ToAfter != "2026-09-07" ||
		s313.DaysBefore != "_MMJVS_" || s313.DaysAfter != "LMMJV__" {
		t.Fatalf("el corrimiento de 313 dice %+v", *s313)
	}
	if !strings.Contains(s313.Text, "12:00 AM") || !strings.Contains(s313.Text, "un día atrás") ||
		!strings.Contains(s313.Text, "martes a sábado de calendario = lunes a viernes de día de emisión") {
		t.Fatalf("el aviso del corrimiento no se entiende: %q", s313.Text)
	}

	// "Duración" son episodios por corrida, nunca minutos.
	if _, r := buscar(t, res, "327"); r.EpisodesPerRun != 10 {
		t.Fatalf("Gaming Longplays 327 trae %d episodios por corrida, se esperaban 10", r.EpisodesPerRun)
	}
	if _, r := buscar(t, res, "273"); r.EpisodesPerRun != 1 {
		t.Fatalf("Familia Robinson trae %d episodios por corrida, se esperaba 1", r.EpisodesPerRun)
	}
	if _, r := buscar(t, res, "316"); r.EpisodesPerRun != 2 {
		t.Fatalf("Tarzán trae %d episodios por corrida, se esperaban 2", r.EpisodesPerRun)
	}

	// Y el título con más de un episodio por corrida es una serie.
	if i, _ := buscar(t, res, "327"); res.TitleFor(i).Kind != model.TitleSeries {
		t.Fatalf("Gaming Longplays quedó como %q", res.TitleFor(i).Kind)
	}
	if res.Titles[0].MetadataSource != "hoja" {
		t.Fatalf("la fuente de la ficha es %q, se esperaba \"hoja\"", res.Titles[0].MetadataSource)
	}

	// RadioOnce Live! es una fuente en vivo: la Duración se ignora con aviso.
	i, live := buscar(t, res, "349")
	if live.Kind != model.RuleLive {
		t.Fatalf("349 quedó como %q, se esperaba %q", live.Kind, model.RuleLive)
	}
	if live.EpisodesPerRun != 0 {
		t.Fatalf("349 trae %d episodios por corrida; un vivo no lleva", live.EpisodesPerRun)
	}
	if res.TitleFor(i).Name != "RadioOnce Live!" {
		t.Fatalf("el título de 349 es %q", res.TitleFor(i).Name)
	}
	var aviso string
	for _, n := range res.Notices {
		if n.SheetID == "349" {
			aviso = n.Text
		}
	}
	if !strings.Contains(aviso, "fuente en vivo") || !strings.Contains(aviso, "Duración") {
		t.Fatalf("no se avisó que la Duración del vivo se ignoró: %q", aviso)
	}

	// El espacio dura media hora mientras nadie diga otra cosa.
	if _, r := buscar(t, res, "273"); r.SlotMs != DefaultSlotMs {
		t.Fatalf("el espacio de 273 dura %d ms, se esperaban %d", r.SlotMs, DefaultSlotMs)
	}

	// Todo lo importado queda activo, en el canal, y con el patrón válido.
	for i, r := range res.Rules {
		if !r.Active || r.ChannelID != catv.ID || !r.Days.Valid() || r.To < r.From {
			t.Fatalf("la regla %s quedó rara: %+v", res.SheetIDs[i], r)
		}
	}

	if !strings.Contains(res.Summary(), "34 reglas importadas") ||
		!strings.Contains(res.Summary(), "1 fila no cuadró") ||
		!strings.Contains(res.Summary(), "4 fechas corridas por hora de madrugada") {
		t.Fatalf("el resumen no se entiende: %q", res.Summary())
	}
}

func TestRelevosInferidos(t *testing.T) {
	res := Rules(reglasDeLaHoja(t), catv, WithLiveNames("RadioOnce Live!"))

	buscarRelevo := func(from, to string) HandoffProposal {
		t.Helper()
		for _, h := range res.Handoffs {
			if h.FromID == from && h.ToID == to {
				return h
			}
		}
		t.Fatalf("no se propuso el relevo %s→%s; se propusieron %+v", from, to, res.Handoffs)
		return HandoffProposal{}
	}

	// 346 → 352: Zoids releva a Magic Knight Rayearth a las 3:00 PM.
	h := buscarRelevo("346", "352")
	want := "¿Zoids releva a Magic Knight Rayearth a las 3:00 PM?"
	if h.Text != want {
		t.Fatalf("el texto del relevo es %q, se esperaba %q", h.Text, want)
	}
	if res.Rules[h.From].To.Add(1) != res.Rules[h.To].From {
		t.Fatal("el relevo propuesto no es consecutivo")
	}

	// 313 → 353: el mismo relevo a las 12:00 AM, que solo se ve después del
	// corrimiento de fechas.
	h = buscarRelevo("313", "353")
	if !strings.Contains(h.Text, "12:00 AM") {
		t.Fatalf("el relevo de las 12:00 AM dice %q", h.Text)
	}

	// Se proponen, no se aplican: nadie carga releva_a por su cuenta.
	for i, r := range res.Rules {
		if r.HandsOffTo != nil {
			t.Fatalf("la regla %s ya trae releva_a cargado; eso lo confirma la persona", res.SheetIDs[i])
		}
	}
}

func TestSegundoPaseInferido(t *testing.T) {
	res := Rules(reglasDeLaHoja(t), catv, WithLiveNames("RadioOnce Live!"))

	var found bool
	for _, r := range res.Repeats {
		if r.PrimaryID == "344" && r.RepeatID == "315" {
			found = true
			if dayOffset(res.Rules[r.Primary].At, catv) >= dayOffset(res.Rules[r.Repeat].At, catv) {
				t.Fatal("la primaria debería ser la de la tarde")
			}
			if !strings.Contains(r.Text, "You're Under Arrest") {
				t.Fatalf("el texto del segundo pase dice %q", r.Text)
			}
		}
	}
	if !found {
		t.Fatalf("no se propuso el segundo pase 344/315; se propusieron %+v", res.Repeats)
	}
	// Correr el patrón de madrugada deja a la vista tres segundos pases más,
	// que antes no se veían porque el patrón decía otro día: la corrida de
	// las 12:00 AM es la repetición de la de la tarde.
	if len(res.Repeats) != 5 {
		t.Fatalf("segundos pases propuestos: %d, se esperaban 5: %+v", len(res.Repeats), res.Repeats)
	}
	for _, par := range [][2]string{{"344", "315"}, {"346", "313"}, {"347", "304"}, {"352", "353"}, {"345", "311"}} {
		var ok bool
		for _, r := range res.Repeats {
			if r.PrimaryID == par[0] && r.RepeatID == par[1] {
				ok = true
			}
		}
		if !ok {
			t.Fatalf("falta el segundo pase %s/%s", par[0], par[1])
		}
	}
	for i, r := range res.Rules {
		if r.RepeatsOf != nil {
			t.Fatalf("la regla %s ya trae repite_a cargado; eso lo confirma la persona", res.SheetIDs[i])
		}
	}
}

func TestPosiblesDuplicados(t *testing.T) {
	res := Rules(reglasDeLaHoja(t), catv, WithLiveNames("RadioOnce Live!"))

	var samurai bool
	for _, d := range res.PossibleDuplicates {
		if loose(d.A) == "samuraix" && loose(d.B) == "samuraix" {
			samurai = true
		}
	}
	if !samurai {
		t.Fatalf("«SamuraiX» y «Samurai X» no se avisaron como posible duplicado: %+v", res.PossibleDuplicates)
	}
	// Y siguen siendo dos títulos distintos: no se fusionan solos.
	var a, b bool
	for _, ti := range res.Titles {
		switch ti.Name {
		case "SamuraiX":
			a = true
		case "Samurai X":
			b = true
		}
	}
	if !a || !b {
		t.Fatal("«SamuraiX» y «Samurai X» se fusionaron solos")
	}
}

func TestParrillaYDuracionDelEspacio(t *testing.T) {
	var semana *Grid
	for _, s := range ParseAll(hoja(t)) {
		if s.Kind == KindGrid && len(s.Grid.Days) == 7 {
			semana = s.Grid
			break
		}
	}
	if semana == nil {
		t.Fatal("no se reconoció la parrilla semanal")
	}
	if len(semana.Slots) != 48 {
		t.Fatalf("la parrilla trae %d franjas, se esperaban 48", len(semana.Slots))
	}
	if semana.Slots[0].At != 0 {
		t.Fatalf("la primera franja va a las %s", FormatClock(semana.Slots[0].At))
	}

	res := Rules(reglasDeLaHoja(t), catv, WithLiveNames("RadioOnce Live!"), WithGrid(semana))
	// Gaming Longplays ocupa de 6:00 a 11:00 PM: cinco horas, no media.
	if _, r := buscar(t, res, "327"); r.SlotMs != 5*60*60*1000 {
		t.Fatalf("el espacio de Gaming Longplays dura %d ms, se esperaban 5 horas", r.SlotMs)
	}
	if _, r := buscar(t, res, "311"); r.SlotMs != DefaultSlotMs {
		t.Fatalf("el espacio de SamuraiX dura %d ms, se esperaba media hora", r.SlotMs)
	}
}

// El TSV es lo que de verdad llega al pegar desde Sheets o Excel: las mismas
// filas, separadas por tabuladores, tienen que dar exactamente lo mismo.
func TestTSVPegadoDaLoMismo(t *testing.T) {
	md := reglasDeLaHoja(t)
	tsv := comoTSV(md, []string{"Id", "Video", "Duración", "Días", "Horas", "Fec Ini", "Fec Final", "Conflict Severity?"},
		[]string{ColID, ColTitle, ColEpisodes, ColDays, ColAt, ColFrom, ColTo}, "\t")

	pegado, errs := Parse(tsv)
	if len(errs) != 0 {
		t.Fatalf("Parse del TSV devolvió errores: %v", errs)
	}
	if pegado.Kind != KindRules {
		t.Fatalf("el TSV se reconoció como %q", pegado.Kind)
	}

	a := Rules(md, catv, WithLiveNames("RadioOnce Live!"))
	b := Rules(pegado, catv, WithLiveNames("RadioOnce Live!"))
	compararResultados(t, a, b)

	// Lo mismo con comas.
	csv := comoTSV(md, []string{"Id", "Video", "Duración", "Días", "Horas", "Fec Ini", "Fec Final"},
		[]string{ColID, ColTitle, ColEpisodes, ColDays, ColAt, ColFrom, ColTo}, ",")
	hojaCSV, errs := Parse(csv)
	if len(errs) != 0 {
		t.Fatalf("Parse del CSV devolvió errores: %v", errs)
	}
	compararResultados(t, a, Rules(hojaCSV, catv, WithLiveNames("RadioOnce Live!")))
}

// Los encabezados llegan como llegan: sin acentos, en mayúsculas, en otro
// orden y con columnas de sobra.
func TestEncabezadosSinAcentosYColumnasDeSobra(t *testing.T) {
	md := reglasDeLaHoja(t)
	tsv := comoTSV(md, []string{"NOTAS", "ID", "VIDEO", "DURACION", "DIAS", "HORAS", "FEC INI", "FEC FINAL", "Conflicts With?"},
		[]string{"", ColID, ColTitle, ColEpisodes, ColDays, ColAt, ColFrom, ColTo}, "\t")

	pegado, errs := Parse(tsv)
	if len(errs) != 0 {
		t.Fatalf("Parse devolvió errores: %v", errs)
	}
	if pegado.Kind != KindRules {
		t.Fatalf("se reconoció como %q", pegado.Kind)
	}
	compararResultados(t, Rules(md, catv, WithLiveNames("RadioOnce Live!")),
		Rules(pegado, catv, WithLiveNames("RadioOnce Live!")))
}

func TestFilasQueNoCuadranSeExplican(t *testing.T) {
	tsv := strings.Join([]string{
		"Id\tVideo\tDuración\tDías\tHoras\tFec Ini\tFec Final",
		"1\tKojak\t1\tLMMJV__\t8:00:00 AM\t8/9/2026\t12/20/2026",
		"2\t\t1\tLMMJV__\t8:30:00 AM\t8/9/2026\t12/20/2026",
		"3\tTarzan\t1\tLMMJV__\t\t8/9/2026\t12/20/2026",
		"4\tZorro 57\t1\tLXMJV__\t9:00:00 AM\t8/9/2026\t12/20/2026",
		"5\tAstroboy\t1\tLMMJV__\t9:30:00 AM\tayer\t12/20/2026",
	}, "\n")

	sheet, _ := Parse(tsv)
	res := Rules(sheet, catv)
	if len(res.Rules) != 1 || len(res.RowErrors) != 4 {
		t.Fatalf("%d reglas y %d filas con problema: %s", len(res.Rules), len(res.RowErrors), res.Summary())
	}
	motivos := map[string]string{}
	for _, e := range res.RowErrors {
		motivos[e.SheetID] = e.Reason
		if e.Line == 0 {
			t.Fatalf("la fila %s no dice de qué línea salió", e.SheetID)
		}
	}
	for id, trozo := range map[string]string{
		"2": "no dice qué programa va",
		"3": "no trae hora",
		"4": "no cuadra",
		"5": "no entendí la fecha de inicio",
	} {
		if !strings.Contains(motivos[id], trozo) {
			t.Fatalf("la fila %s dice %q y se esperaba algo con %q", id, motivos[id], trozo)
		}
	}
}

func TestCatalogoDeLaHojaReal(t *testing.T) {
	var cat Sheet
	for _, s := range ParseAll(hoja(t)) {
		if s.Kind == KindCatalog {
			cat = s
			break
		}
	}
	if cat.Kind != KindCatalog {
		t.Fatal("no se reconoció la tabla del catálogo")
	}

	titles, errs := Catalog(cat)
	if len(errs) != 0 {
		t.Fatalf("filas del catálogo que no cuadraron: %v", errs)
	}
	if len(titles) != 118 {
		t.Fatalf("el catálogo trae %d títulos, se esperaban 118", len(titles))
	}
	if n := CountWithSynopsis(titles); n != 112 {
		t.Fatalf("%d títulos con sinopsis, se esperaban 112", n)
	}

	por := map[string]model.Title{}
	for _, ti := range titles {
		por[ti.Name] = ti
	}
	buck, ok := por["Buck Rogers in the 25th Century"]
	if !ok {
		t.Fatal("falta Buck Rogers in the 25th Century")
	}
	if buck.Year == nil || *buck.Year != 1979 {
		t.Fatalf("el año de Buck Rogers es %v", buck.Year)
	}
	if buck.ContentRating != "TV-14" {
		t.Fatalf("la clasificación de Buck Rogers es %q", buck.ContentRating)
	}
	if buck.Kind != model.TitleSeries {
		t.Fatalf("Buck Rogers quedó como %q", buck.Kind)
	}
	if buck.MetadataSource != "hoja" {
		t.Fatalf("la fuente de la ficha es %q", buck.MetadataSource)
	}
	if !strings.HasPrefix(buck.Synopsis, "20th-century astronaut") {
		t.Fatalf("la sinopsis de Buck Rogers empieza con %q", buck.Synopsis)
	}
	// Los títulos sin año quedan sin año: no se inventa.
	if g, ok := por["Gaming Longplays"]; !ok || g.Year != nil || g.Synopsis != "" {
		t.Fatalf("Gaming Longplays quedó %+v", g)
	}

	// Y el catálogo se junta con lo que salió de las reglas sin duplicar: de
	// los 29 títulos de las reglas, 8 se llaman exactamente igual que una
	// ficha del catálogo, así que quedan 139 y no 147.
	res := Rules(reglasDeLaHoja(t), catv, WithLiveNames("RadioOnce Live!"))
	if len(res.Titles) != 29 {
		t.Fatalf("las reglas trajeron %d títulos, se esperaban 29", len(res.Titles))
	}
	todos, sinPareja := MergeTitles(res.Titles, titles)
	if len(todos) == 0 || len(sinPareja) == 0 {
		t.Fatalf("al juntar quedaron %d títulos y %d sin pareja", len(todos), len(sinPareja))
	}
	for _, ti := range todos {
		if ti.Name == "Kojak" && ti.Synopsis == "" {
			t.Fatal("Kojak se quedó sin la sinopsis del catálogo")
		}
		// «Zorro 57» en las reglas es «Zorro (1957)» en el catálogo: nombres
		// distintos, así que no se juntan solos. Se dice, no se adivina.
		if ti.Name == "Zorro 57" && ti.Synopsis != "" {
			t.Fatal("«Zorro 57» se emparejó solo con «Zorro (1957)»")
		}
	}
}

func TestHoraYFechaEnCristiano(t *testing.T) {
	horas := map[string]model.Minutes{
		"12:00:00 AM": 0,
		"12:30:00 AM": 30,
		"7:30:00 AM":  7*60 + 30,
		"12:00:00 PM": 12 * 60,
		"3:00 PM":     15 * 60,
		"23:30":       23*60 + 30,
		"23:30:00":    23*60 + 30,
		" 6:00 a.m. ": 6 * 60,
	}
	for in, want := range horas {
		got, ok := ParseClock(in)
		if !ok || got != want {
			t.Fatalf("ParseClock(%q) = %v, %v; se esperaba %v", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "veinticinco", "25:00", "7:70 AM", "13:00 PM"} {
		if _, ok := ParseClock(in); ok {
			t.Fatalf("ParseClock(%q) no debió entenderse", in)
		}
	}
	if FormatClock(0) != "12:00 AM" || FormatClock(15*60) != "3:00 PM" || FormatClock(12*60) != "12:00 PM" {
		t.Fatal("FormatClock no escribe la hora como la lee una persona")
	}

	for in, want := range map[string]model.Day{
		"8/22/2026":  "2026-08-22",
		"2026-08-22": "2026-08-22",
		"1/3/2027":   "2027-01-03",
	} {
		got, ok := ParseDate(in)
		if !ok || got != want {
			t.Fatalf("ParseDate(%q) = %v, %v; se esperaba %v", in, got, ok, want)
		}
	}
	if FormatDate("2026-09-15") != "15 sep 2026" || FormatDate("2026-01-01") != "1 ene 2026" {
		t.Fatal("FormatDate no escribe la fecha como la lee una persona")
	}
}

func TestNoSeReconoceLaTabla(t *testing.T) {
	_, errs := Parse("hola\tque tal\nnada\tque ver")
	if len(errs) != 1 || !strings.Contains(errs[0].Reason, "no reconocí") {
		t.Fatalf("se esperaba un motivo en cristiano, salió %v", errs)
	}
	res := Rules(Sheet{Kind: KindUnknown}, catv)
	if len(res.Rules) != 0 || len(res.RowErrors) != 1 {
		t.Fatalf("Rules sobre una tabla desconocida devolvió %+v", res)
	}
}

// Las reglas y el catálogo de una hoja hecha a mano no escriben el título
// igual: "Zorro 57" en las reglas es "Zorro (1957)" en el catálogo. Son
// erratas del mismo título y el nombre bueno es el del catálogo.
func TestEmparejaErratasConElCatalogo(t *testing.T) {
	var cat Sheet
	for _, s := range ParseAll(hoja(t)) {
		if s.Kind == KindCatalog {
			cat = s
			break
		}
	}
	fichas, _ := Catalog(cat)
	res := Rules(reglasDeLaHoja(t), catv, WithLiveNames("RadioOnce Live!"))
	if len(res.Titles) != 29 {
		t.Fatalf("las reglas trajeron %d títulos, se esperaban 29", len(res.Titles))
	}

	m := res.ApplyCatalog(fichas)

	// 17 erratas emparejadas solas; 24 de los 29 títulos de las reglas quedan
	// apuntando a una ficha del catálogo, y 4 se quedan para la pantalla.
	if len(m.Matched) != 17 {
		t.Fatalf("erratas emparejadas: %d, se esperaban 17: %+v", len(m.Matched), m.Matched)
	}
	if len(m.Unmatched) != 4 {
		t.Fatalf("títulos sin pareja: %d, se esperaban 4: %+v", len(m.Unmatched), m.Unmatched)
	}
	if len(res.Titles) != 122 {
		t.Fatalf("quedaron %d títulos, se esperaban 122 (las 118 fichas más los 4 sin ficha)", len(res.Titles))
	}

	pareja := map[string]string{}
	for _, x := range m.Matched {
		pareja[x.From] = x.To
	}
	for from, to := range map[string]string{
		"Zorro 57":            "Zorro (1957)",
		"Los Lorcanitos":      "Lorcanitos",
		"You're Under Arrest": "You're Under Arrest!",
		"Comics 9th Art":      "Comics, the Ninth Art",
		"SamuraiX":            "Samurai X",
		"Carmen Sandiego":     "Where on Earth is Carmen Sandiego?",
		"Hack Legend":         ".hack//Legend of the Twilight",
		"Tarzan":              "Tarzan (1966)",
		"Zoids":               "Zoids: Chaotic Century",
		"Astroboy":            "Astro Boy",
		"Green Hornet":        "The Green Hornet",
	} {
		if pareja[from] != to {
			t.Fatalf("«%s» se emparejó con «%s», se esperaba «%s»", from, pareja[from], to)
		}
	}

	// Las reglas quedan apuntando al título del catálogo, no a uno nuevo.
	i, _ := buscar(t, res, "314") // Zorro 57
	if got := res.TitleFor(i); got == nil || got.Name != "Zorro (1957)" {
		t.Fatalf("la regla 314 apunta a %v", got)
	}
	if res.TitleFor(i).Synopsis == "" || res.TitleFor(i).Year == nil {
		t.Fatal("la regla 314 no se quedó con la ficha del catálogo")
	}
	i, _ = buscar(t, res, "311")  // SamuraiX
	j, _ := buscar(t, res, "345") // Samurai X
	if res.TitleIndex[i] != res.TitleIndex[j] {
		t.Fatal("«SamuraiX» y «Samurai X» siguen siendo dos títulos")
	}
	con := 0
	for k := range res.Rules {
		ti := res.TitleFor(k)
		if ti == nil {
			t.Fatalf("la regla %s se quedó sin título", res.SheetIDs[k])
		}
		if ti.Synopsis != "" {
			con++
		}
	}
	// Las 8 que se quedan sin sinopsis son las que no tienen ficha (Los
	// Simuladores, RadioOnce Live!, SaberMarionette, Samurai X ×2) y las que
	// la tienen vacía en el catálogo (Gaming Longplays ×2, Lorcanitos).
	if con != 26 {
		t.Fatalf("%d de las 34 reglas quedaron con sinopsis, se esperaban 26", con)
	}

	// Lo que no se puede decidir solo no se adivina: se pregunta.
	sin := map[string]Unmatched{}
	for _, u := range m.Unmatched {
		sin[u.Name] = u
	}
	saber, ok := sin["SaberMarionette"]
	if !ok || len(saber.Candidates) != 2 {
		t.Fatalf("«SaberMarionette» debía quedar en duda entre las Saber Marionette: %+v", saber)
	}
	if !strings.Contains(saber.Text, "no quise adivinar") {
		t.Fatalf("el aviso de la duda dice %q", saber.Text)
	}
	for _, n := range []string{"Los Simuladores", "Samurai X", "RadioOnce Live!"} {
		if _, ok := sin[n]; !ok {
			t.Fatalf("«%s» debía quedar sin pareja", n)
		}
	}

	// Y el emparejamiento se cuenta fila por fila, en cristiano.
	var zorro string
	for _, n := range res.Notices {
		if n.Title == "Zorro 57" {
			zorro = n.Text
		}
	}
	if !strings.Contains(zorro, "«Zorro 57» es «Zorro (1957)» en el catálogo") {
		t.Fatalf("el aviso del emparejamiento dice %q", zorro)
	}
}

func TestParecidoDeNombres(t *testing.T) {
	iguales := [][2]string{
		{"Los Lorcanitos", "Lorcanitos"},
		{"Green Hornet", "The Green Hornet"},
		{"Astroboy", "Astro Boy"},
		{"BT'x", "B'T X"},
		{"Zorro 57", "Zorro (1957)"},
		{"Familia Robinson", "The Swiss Family Robinson: Flone of the Mysterious Island"},
	}
	for _, par := range iguales {
		if s := similarity(newNameKey(par[0]), newNameKey(par[1])); s < MatchThreshold {
			t.Fatalf("«%s» y «%s» dan %.3f, se esperaba al menos %.2f", par[0], par[1], s, MatchThreshold)
		}
	}
	distintos := [][2]string{
		{"Mazinger Z", "Great Mazinger"},
		{"Los Simuladores", "Los Lorcanitos"},
		{"Zoids", "Zenki"},
		{"Kojak", "Kolchak: The Night Stalker"},
	}
	for _, par := range distintos {
		if s := similarity(newNameKey(par[0]), newNameKey(par[1])); s >= MatchThreshold {
			t.Fatalf("«%s» y «%s» dan %.3f y no debían emparejarse", par[0], par[1], s)
		}
	}
	if DescribePattern("LMMJV__") != "lunes a viernes" ||
		DescribePattern("_____SD") != "sábado y domingo" ||
		DescribePattern("_MMJVS_") != "martes a sábado" {
		t.Fatal("DescribePattern no dice los días como una persona")
	}
	if shiftPattern("_MMJVS_") != "LMMJV__" || shiftPattern("L_____D") != "_____SD" ||
		shiftPattern("_____SD") != "____VS_" {
		t.Fatal("shiftPattern no corre el patrón un día atrás")
	}
}

// ── ayudas del test ───────────────────────────────────────────────────

// comoTSV rehace las mismas filas de la hoja como lo que llega al pegar:
// encabezados a la izquierda, celdas separadas por sep.
func comoTSV(s Sheet, header []string, cols []string, sep string) string {
	var b strings.Builder
	b.WriteString(strings.Join(header, sep))
	b.WriteString("\n")
	for _, r := range s.Rows {
		cells := make([]string, len(cols))
		for i, c := range cols {
			if c == "" {
				continue
			}
			v := s.Get(r, c)
			if strings.Contains(v, sep) || strings.Contains(v, "\"") {
				v = "\"" + strings.ReplaceAll(v, "\"", "\"\"") + "\""
			}
			cells[i] = v
		}
		b.WriteString(strings.Join(cells, sep))
		b.WriteString("\n")
	}
	return b.String()
}

func compararResultados(t *testing.T, a, b Result) {
	t.Helper()
	if len(a.Rules) != len(b.Rules) {
		t.Fatalf("reglas: %d contra %d", len(a.Rules), len(b.Rules))
	}
	for i := range a.Rules {
		if a.Rules[i] != b.Rules[i] || a.SheetIDs[i] != b.SheetIDs[i] {
			t.Fatalf("la regla %d no cuadra:\n  %+v\n  %+v", i, a.Rules[i], b.Rules[i])
		}
		ta, tb := a.TitleFor(i), b.TitleFor(i)
		if (ta == nil) != (tb == nil) || (ta != nil && ta.Name != tb.Name) {
			t.Fatalf("el título de la regla %d no cuadra", i)
		}
	}
	if len(a.RowErrors) != len(b.RowErrors) {
		t.Fatalf("filas con problema: %d contra %d", len(a.RowErrors), len(b.RowErrors))
	}
	for i := range a.RowErrors {
		if a.RowErrors[i].Reason != b.RowErrors[i].Reason || a.RowErrors[i].SheetID != b.RowErrors[i].SheetID {
			t.Fatalf("el motivo %d no cuadra:\n  %s\n  %s", i, a.RowErrors[i].Reason, b.RowErrors[i].Reason)
		}
	}
	if len(a.DatesShifted) != len(b.DatesShifted) || len(a.Handoffs) != len(b.Handoffs) ||
		len(a.Repeats) != len(b.Repeats) || len(a.Titles) != len(b.Titles) {
		t.Fatalf("las propuestas no cuadran:\n  %s\n  %s", a.Summary(), b.Summary())
	}
}
