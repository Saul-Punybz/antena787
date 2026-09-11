// Package importer lee lo que una persona pega desde Google Sheets o Excel y
// lo convierte en reglas y títulos del dominio (internal/model).
//
// Es lógica pura: no toca la base, ni la red, ni el disco. Recibe texto y
// devuelve datos más una lista de lo que no cuadró, fila por fila y en palabras
// claras. Nunca rechaza la hoja entera (PRD §13, auditoría B11): trae lo
// que sirve y explica el resto.
//
// Formatos que entiende:
//
//   - TSV  — lo normal al pegar desde Sheets o Excel (celdas separadas por
//     tabulador, comillas dobles para celdas con separadores adentro).
//   - CSV  — lo mismo con comas.
//   - Markdown con barras `|`, como docs/catv-sheet-2026-09-04.md.
//
// Y tres tablas, que reconoce sola por sus encabezados:
//
//   - reglas    (Id, Video, Duración, Días, Horas, Fec Ini, Fec Final…)
//   - catálogo  (Título/Title, Sinopsis/Summary, Año, Clasificación, Tipo…)
//   - parrilla  (primera columna con horas y una columna por día) — se lee
//     solo para proponer duraciones de espacio, nunca se importa como reglas.
package importer

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"antena787/internal/model"
)

// SheetKind es qué tabla resultó ser la que pegaron.
type SheetKind string

const (
	KindRules   SheetKind = "reglas"
	KindCatalog SheetKind = "catalogo"
	KindGrid    SheetKind = "parrilla"
	KindUnknown SheetKind = "desconocida"
)

// Nombres canónicos de columna. Los encabezados reales se traducen a estos,
// sin importar acentos, mayúsculas ni idioma.
const (
	ColID             = "id"
	ColTitle          = "titulo"
	ColEpisodes       = "episodios"
	ColDays           = "dias"
	ColAt             = "horas"
	ColFrom           = "desde"
	ColTo             = "hasta"
	ColSynopsis       = "sinopsis"
	ColYear           = "anio"
	ColContentRating  = "clasificacion"
	ColAudienceRating = "clasificacion_audiencia"
	ColKind           = "tipo"
	ColGenre          = "genero"
)

// headerSynonyms traduce un encabezado ya normalizado (minúsculas, sin
// acentos, sin espacios de sobra) al nombre canónico de la columna.
var headerSynonyms = map[string]string{
	// reglas
	"id": ColID, "id regla": ColID, "regla": ColID, "no": ColID, "num": ColID,
	"video": ColTitle, "titulo": ColTitle, "title": ColTitle, "nombre": ColTitle,
	"programa": ColTitle, "name": ColTitle, "serie": ColTitle,
	"duracion": ColEpisodes, "episodios": ColEpisodes, "eps": ColEpisodes,
	"episodios por corrida": ColEpisodes, "cantidad": ColEpisodes,
	"dias": ColDays, "days": ColDays, "patron": ColDays, "patron de dias": ColDays,
	"horas": ColAt, "hora": ColAt, "time": ColAt, "hour": ColAt, "horario": ColAt,
	"fec ini": ColFrom, "fecha ini": ColFrom, "fecha inicio": ColFrom,
	"fecha de inicio": ColFrom, "inicio": ColFrom, "desde": ColFrom,
	"start": ColFrom, "start date": ColFrom, "from": ColFrom,
	"fec final": ColTo, "fec fin": ColTo, "fecha final": ColTo, "fecha fin": ColTo,
	"fecha de fin": ColTo, "fin": ColTo, "hasta": ColTo, "end": ColTo,
	"end date": ColTo, "to": ColTo,

	// catálogo
	"sinopsis": ColSynopsis, "summary": ColSynopsis, "resumen": ColSynopsis,
	"descripcion": ColSynopsis, "synopsis": ColSynopsis, "overview": ColSynopsis,
	"ano": ColYear, "anio": ColYear, "year": ColYear, "estreno": ColYear,
	"clasificacion": ColContentRating, "contentrating": ColContentRating,
	"content rating": ColContentRating, "clasificacion de contenido": ColContentRating,
	"clasificacion contenido": ColContentRating, "rating": ColContentRating,
	"audiencerating": ColAudienceRating, "audience rating": ColAudienceRating,
	"clasificacion de audiencia": ColAudienceRating,
	"clasificacion audiencia":    ColAudienceRating, "puntuacion": ColAudienceRating,
	"tipo": ColKind, "type": ColKind, "kind": ColKind, "categoria": ColKind,
	"genero": ColGenre, "genre": ColGenre,
}

// Row es una fila de la tabla, con la línea del texto pegado de donde salió
// (1 = la primera línea de lo que pegaron), para poder decir "fila 135".
type Row struct {
	Line  int
	Cells []string
}

// Cell devuelve la celda i ya recortada, o "" si esa columna no existe.
func (r Row) Cell(i int) string {
	if i < 0 || i >= len(r.Cells) {
		return ""
	}
	return strings.TrimSpace(r.Cells[i])
}

func (r Row) empty() bool {
	for _, c := range r.Cells {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

// Sheet es una tabla reconocida: qué es, dónde está cada columna y sus filas.
type Sheet struct {
	Kind    SheetKind
	Line    int            // línea del encabezado en el texto pegado
	Header  []string       // encabezados tal como venían
	Columns map[string]int // nombre canónico de columna → índice
	Rows    []Row          // solo las filas de datos
	Grid    *Grid          // solo si Kind == KindGrid
}

// Has dice si la tabla trae esa columna.
func (s Sheet) Has(col string) bool { _, ok := s.Columns[col]; return ok }

// Get devuelve el valor de esa columna en esa fila ("" si no está).
func (s Sheet) Get(r Row, col string) string {
	i, ok := s.Columns[col]
	if !ok {
		return ""
	}
	return r.Cell(i)
}

// Slot es una franja de la parrilla: una hora y qué hay ese día en cada
// columna de día.
type Slot struct {
	Line  int
	At    model.Minutes
	Cells []string
}

// Grid es la parrilla semanal o mensual: horas en la primera columna y una
// columna por día. Se lee para proponer, nunca para importar reglas.
type Grid struct {
	Line  int
	Days  []string
	Slots []Slot
}

// RowError es una fila que no se pudo importar, dicha para una persona: qué
// fila era, de qué programa, y por qué no cuadró. Nunca "está mal".
type RowError struct {
	Line    int    // línea del texto pegado
	SheetID string // el Id que traía la hoja, si lo traía
	Title   string // el título de la fila, si se pudo leer
	Reason  string // el motivo, en palabras claras
}

func (e RowError) Error() string {
	who := ""
	switch {
	case e.SheetID != "" && e.Title != "":
		who = fmt.Sprintf(" (Id %s, %s)", e.SheetID, e.Title)
	case e.SheetID != "":
		who = fmt.Sprintf(" (Id %s)", e.SheetID)
	case e.Title != "":
		who = fmt.Sprintf(" (%s)", e.Title)
	}
	if e.Line > 0 {
		return fmt.Sprintf("fila %d%s: %s", e.Line, who, e.Reason)
	}
	return e.Reason
}

// Notice es un aviso que no impide importar la fila: se importó, pero hay
// algo que la persona debe saber.
type Notice struct {
	Line    int    `json:"fila"`
	SheetID string `json:"id_hoja"`
	Title   string `json:"titulo"`
	Text    string `json:"texto"`
}

// ── entrada ───────────────────────────────────────────────────────────

// Parse lee lo pegado y devuelve la tabla más útil que encontró. Si el texto
// trae varias tablas (como el archivo de la hoja de CAtv completo) prefiere
// las reglas, luego el catálogo, luego la parrilla.
//
// Solo devuelve RowError cuando no reconoció ninguna tabla; los errores de
// contenido salen de Rules y de Catalog, que son quienes interpretan.
func Parse(text string) (Sheet, []RowError) {
	sheets := ParseAll(text)
	for _, want := range []SheetKind{KindRules, KindCatalog, KindGrid} {
		for _, s := range sheets {
			if s.Kind == want {
				return s, nil
			}
		}
	}
	return Sheet{Kind: KindUnknown, Columns: map[string]int{}}, []RowError{{
		Reason: "no reconocí esta tabla: para las reglas espero columnas como Id, Video, Duración, Días, Horas, Fec Ini y Fec Final; para el catálogo, Título y Sinopsis",
	}}
}

// ParseAll devuelve todas las tablas del texto, en orden. Dos tablas se
// separan por una línea en blanco o por una fila enteramente vacía, que es
// como salen de una hoja de cálculo.
func ParseAll(text string) []Sheet {
	var out []Sheet
	for _, b := range splitBlocks(text) {
		if s, ok := sheetFrom(b); ok {
			out = append(out, s)
		}
	}
	return out
}

// ParseRules es el atajo de siempre: lee el texto y exige que sea la tabla de
// reglas. Devuelve la tabla y, si no la encontró, el motivo.
func ParseRules(text string) (Sheet, []RowError) {
	for _, s := range ParseAll(text) {
		if s.Kind == KindRules {
			return s, nil
		}
	}
	return Sheet{Kind: KindUnknown, Columns: map[string]int{}}, []RowError{{
		Reason: "no encontré la tabla de reglas: espero columnas como Id, Video, Duración, Días, Horas, Fec Ini y Fec Final",
	}}
}

// ParseCatalog es lo mismo para el catálogo.
func ParseCatalog(text string) (Sheet, []RowError) {
	for _, s := range ParseAll(text) {
		if s.Kind == KindCatalog {
			return s, nil
		}
	}
	return Sheet{Kind: KindUnknown, Columns: map[string]int{}}, []RowError{{
		Reason: "no encontré la tabla del catálogo: espero al menos una columna de título y otra de sinopsis, año, tipo o clasificación",
	}}
}

// ── partir el texto en tablas ─────────────────────────────────────────

func splitBlocks(text string) [][]Row {
	format := detectFormat(text)
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")

	var blocks [][]Row
	var cur []Row
	flush := func() {
		if len(cur) > 0 {
			blocks = append(blocks, cur)
			cur = nil
		}
	}
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		cells, ok := splitLine(line, format)
		if !ok { // línea de guiones del markdown: no es fila ni corta la tabla
			continue
		}
		r := Row{Line: i + 1, Cells: cells}
		if r.empty() {
			flush()
			continue
		}
		cur = append(cur, r)
	}
	flush()
	return blocks
}

func detectFormat(text string) string {
	pipes, tabs := 0, 0
	for _, line := range strings.Split(text, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "|") && strings.Contains(t[1:], "|") {
			pipes++
		}
		if strings.Contains(line, "\t") {
			tabs++
		}
	}
	switch {
	case pipes >= 2:
		return "markdown"
	case tabs > 0:
		return "tsv"
	default:
		return "csv"
	}
}

// splitLine parte una línea en celdas. El segundo valor es false cuando la
// línea no es una fila de datos (la de guiones de una tabla markdown).
func splitLine(line, format string) ([]string, bool) {
	switch format {
	case "markdown":
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "|") {
			return nil, false
		}
		cells := splitPipes(t)
		if isRuleLine(cells) {
			return nil, false
		}
		for i, c := range cells {
			cells[i] = unescapeMarkdown(c)
		}
		return cells, true
	case "tsv":
		return splitDelim(line, '\t'), true
	default:
		return splitDelim(line, ','), true
	}
}

// splitPipes parte una fila markdown por las barras que no van escapadas, y
// quita la barra de apertura y la de cierre.
func splitPipes(line string) []string {
	var cells []string
	var b strings.Builder
	esc := false
	for _, r := range line {
		switch {
		case esc:
			b.WriteRune('\\')
			b.WriteRune(r)
			esc = false
		case r == '\\':
			esc = true
		case r == '|':
			cells = append(cells, b.String())
			b.Reset()
		default:
			b.WriteRune(r)
		}
	}
	if esc {
		b.WriteRune('\\')
	}
	cells = append(cells, b.String())
	// la primera y la última celda son lo que hay antes de la barra inicial y
	// después de la final: vacías por construcción.
	if len(cells) > 0 && strings.TrimSpace(cells[0]) == "" {
		cells = cells[1:]
	}
	if n := len(cells); n > 0 && strings.TrimSpace(cells[n-1]) == "" {
		cells = cells[:n-1]
	}
	for i, c := range cells {
		cells[i] = strings.TrimSpace(c)
	}
	return cells
}

// isRuleLine reconoce la fila de guiones (`| :-: | --- |`) del markdown.
func isRuleLine(cells []string) bool {
	if len(cells) == 0 {
		return false
	}
	for _, c := range cells {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		c = strings.Trim(c, ":")
		if c == "" || strings.Trim(c, "-") != "" {
			return false
		}
	}
	return true
}

// splitDelim parte por un separador respetando comillas dobles, que es como
// una hoja de cálculo escapa una celda con separadores o saltos adentro.
func splitDelim(line string, sep rune) []string {
	var cells []string
	var b strings.Builder
	inQuotes := false
	rs := []rune(line)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch {
		case r == '"':
			if inQuotes && i+1 < len(rs) && rs[i+1] == '"' {
				b.WriteRune('"')
				i++
				continue
			}
			inQuotes = !inQuotes
		case r == sep && !inQuotes:
			cells = append(cells, strings.TrimSpace(b.String()))
			b.Reset()
		default:
			b.WriteRune(r)
		}
	}
	cells = append(cells, strings.TrimSpace(b.String()))
	return cells
}

// unescapeMarkdown deshace los escapes que mete un exportador a markdown:
// `\_` → `_`, `\!` → `!`, `\-` → `-`.
func unescapeMarkdown(s string) string {
	if !strings.Contains(s, "\\") {
		return strings.TrimSpace(s)
	}
	var b strings.Builder
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		if rs[i] == '\\' && i+1 < len(rs) && isASCIIPunct(rs[i+1]) {
			b.WriteRune(rs[i+1])
			i++
			continue
		}
		b.WriteRune(rs[i])
	}
	return strings.TrimSpace(b.String())
}

func isASCIIPunct(r rune) bool {
	return strings.ContainsRune("!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~", r)
}

// ── reconocer qué tabla es ────────────────────────────────────────────

func sheetFrom(rows []Row) (Sheet, bool) {
	if len(rows) == 0 {
		return Sheet{}, false
	}
	header := rows[0]
	cols := map[string]int{}
	for i, h := range header.Cells {
		canon, ok := headerSynonyms[normalize(h)]
		if !ok {
			continue
		}
		if _, seen := cols[canon]; !seen {
			cols[canon] = i
		}
	}
	s := Sheet{
		Line:    header.Line,
		Header:  header.Cells,
		Columns: cols,
		Rows:    dropTrailingEmpty(rows[1:]),
	}
	switch {
	case hasAll(cols, ColDays, ColAt) && hasAny(cols, ColTitle, ColID):
		s.Kind = KindRules
	case hasAll(cols, ColTitle) && hasAny(cols, ColSynopsis, ColYear, ColKind, ColContentRating, ColAudienceRating):
		s.Kind = KindCatalog
	default:
		if g := gridFrom(rows); g != nil {
			s.Kind = KindGrid
			s.Grid = g
			return s, true
		}
		s.Kind = KindUnknown
	}
	return s, true
}

func hasAll(cols map[string]int, names ...string) bool {
	for _, n := range names {
		if _, ok := cols[n]; !ok {
			return false
		}
	}
	return true
}

func hasAny(cols map[string]int, names ...string) bool {
	for _, n := range names {
		if _, ok := cols[n]; ok {
			return true
		}
	}
	return false
}

func dropTrailingEmpty(rows []Row) []Row {
	for len(rows) > 0 && rows[len(rows)-1].empty() {
		rows = rows[:len(rows)-1]
	}
	out := make([]Row, 0, len(rows))
	for _, r := range rows {
		if !r.empty() {
			out = append(out, r)
		}
	}
	return out
}

// gridFrom arma la parrilla si al menos tres filas empiezan con una hora.
func gridFrom(rows []Row) *Grid {
	first := -1
	var slots []Slot
	for i, r := range rows {
		at, ok := ParseClock(r.Cell(0))
		if !ok {
			continue
		}
		if first < 0 {
			first = i
		}
		slots = append(slots, Slot{Line: r.Line, At: at, Cells: r.Cells})
	}
	if len(slots) < 3 {
		return nil
	}
	// La fila de encabezado de días es la última antes de las horas que trae
	// al menos tres celdas con texto fuera de la primera columna.
	dayCols, days, headLine := []int{}, []string{}, 0
	for i := first - 1; i >= 0; i-- {
		var cs []int
		for j := 1; j < len(rows[i].Cells); j++ {
			if rows[i].Cell(j) != "" {
				cs = append(cs, j)
			}
		}
		if len(cs) >= 3 {
			dayCols = cs
			headLine = rows[i].Line
			for _, j := range cs {
				days = append(days, rows[i].Cell(j))
			}
			break
		}
	}
	if len(dayCols) == 0 {
		width := 0
		for _, s := range slots {
			if len(s.Cells) > width {
				width = len(s.Cells)
			}
		}
		for j := 1; j < width; j++ {
			dayCols = append(dayCols, j)
			days = append(days, "")
		}
	}
	for i, s := range slots {
		cells := make([]string, len(dayCols))
		for k, j := range dayCols {
			cells[k] = emptyDash(Row{Cells: s.Cells}.Cell(j))
		}
		slots[i].Cells = cells
	}
	return &Grid{Line: headLine, Days: days, Slots: slots}
}

// emptyDash: en una hoja hecha a mano, un guion quiere decir "aquí no hay nada".
func emptyDash(s string) string {
	switch strings.TrimSpace(s) {
	case "-", "—", "–", "--":
		return ""
	}
	return strings.TrimSpace(s)
}

// ── lectores de celdas ────────────────────────────────────────────────

// normalize deja un encabezado comparable: minúsculas, sin acentos, sin
// signos de sobra y con un solo espacio entre palabras.
func normalize(s string) string {
	s = unescapeMarkdown(s)
	s = strings.ToLower(strings.TrimSpace(s))
	s = stripAccents(s)
	var b strings.Builder
	space := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			if space && b.Len() > 0 {
				b.WriteRune(' ')
			}
			space = false
			b.WriteRune(r)
		default:
			space = true
		}
	}
	return b.String()
}

var accents = map[rune]rune{
	'á': 'a', 'à': 'a', 'ä': 'a', 'â': 'a', 'ã': 'a', 'å': 'a',
	'é': 'e', 'è': 'e', 'ë': 'e', 'ê': 'e',
	'í': 'i', 'ì': 'i', 'ï': 'i', 'î': 'i',
	'ó': 'o', 'ò': 'o', 'ö': 'o', 'ô': 'o', 'õ': 'o',
	'ú': 'u', 'ù': 'u', 'ü': 'u', 'û': 'u',
	'ñ': 'n', 'ç': 'c',
}

func stripAccents(s string) string {
	var b strings.Builder
	for _, r := range s {
		if a, ok := accents[r]; ok {
			b.WriteRune(a)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// loose deja un nombre comparable para saber si dos títulos son el mismo:
// sin espacios, sin signos, sin acentos, sin mayúsculas. Con esto
// "SamuraiX" y "Samurai X" se ven iguales — y por eso se avisan como posible
// duplicado en vez de fusionarse solos.
func loose(s string) string {
	s = strings.ToLower(stripAccents(unescapeMarkdown(s)))
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// cleanName recorta el nombre y deja un solo espacio entre palabras, sin
// tocar la ortografía: la hoja manda.
func cleanName(s string) string {
	return strings.Join(strings.Fields(unescapeMarkdown(s)), " ")
}

// ParseClock lee una hora tal como la escribe una hoja: "7:30:00 AM",
// "12:00:00 AM", "1:00 PM", "23:30" o "23:30:00".
func ParseClock(s string) (model.Minutes, bool) {
	t := strings.ToUpper(strings.TrimSpace(unescapeMarkdown(s)))
	if t == "" {
		return 0, false
	}
	t = strings.ReplaceAll(t, ".", "")
	meridiem := ""
	for _, suf := range []string{"AM", "PM"} {
		if strings.HasSuffix(t, suf) {
			meridiem = suf
			t = strings.TrimSpace(strings.TrimSuffix(t, suf))
			break
		}
	}
	parts := strings.Split(t, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}
	h, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, false
	}
	m, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || m < 0 || m > 59 {
		return 0, false
	}
	if len(parts) == 3 {
		if sec, err := strconv.Atoi(strings.TrimSpace(parts[2])); err != nil || sec < 0 || sec > 59 {
			return 0, false
		}
	}
	switch meridiem {
	case "AM":
		if h < 1 || h > 12 {
			return 0, false
		}
		if h == 12 {
			h = 0
		}
	case "PM":
		if h < 1 || h > 12 {
			return 0, false
		}
		if h != 12 {
			h += 12
		}
	default:
		if h < 0 || h > 23 {
			return 0, false
		}
	}
	return model.Minutes(h*60 + m), true
}

// FormatClock escribe una hora como la lee una persona: "3:00 PM", "12:00 AM".
func FormatClock(m model.Minutes) string {
	h, mi := int(m)/60, int(m)%60
	suf := "AM"
	switch {
	case h == 0:
		h = 12
	case h == 12:
		suf = "PM"
	case h > 12:
		h -= 12
		suf = "PM"
	}
	return fmt.Sprintf("%d:%02d %s", h, mi, suf)
}

var dateLayouts = []string{"1/2/2006", "2006-01-02", "1-2-2006", "2006/1/2", "1/2/06"}

// ParseDate lee una fecha de hoja: "8/22/2026" (M/D/AAAA, que es lo que
// escribe el Sheet de CAtv) y también "2026-08-22".
func ParseDate(s string) (model.Day, bool) {
	t := strings.TrimSpace(unescapeMarkdown(s))
	if t == "" {
		return "", false
	}
	for _, layout := range dateLayouts {
		if d, err := time.ParseInLocation(layout, t, time.UTC); err == nil {
			return model.Day(d.Format("2006-01-02")), true
		}
	}
	return "", false
}

var months = [...]string{"ene", "feb", "mar", "abr", "may", "jun", "jul", "ago", "sep", "oct", "nov", "dic"}

// FormatDate escribe un día como lo lee una persona: "15 sep 2026".
func FormatDate(d model.Day) string {
	t, err := time.Parse("2006-01-02", string(d))
	if err != nil {
		return string(d)
	}
	return fmt.Sprintf("%d %s %d", t.Day(), months[int(t.Month())-1], t.Year())
}
