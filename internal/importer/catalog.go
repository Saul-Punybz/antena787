package importer

import (
	"fmt"
	"strconv"
	"strings"

	"antena787/internal/model"
)

// Catalog convierte la tabla del catálogo en títulos: sinopsis, año,
// clasificaciones y tipo. Como el resto del importador, no rechaza la hoja
// entera: la fila sin título va a la lista de lo que no cuadró y las demás
// entran igual.
//
// El tipo sale de la columna correspondiente cuando la hay ("show"/"serie" →
// serie, "movie"/"película" → película). Lo que no se sabe queda como
// "programa", nunca inventado. Un título que además aparece en las reglas con
// Duración > 1 es una serie: eso lo decide Rules, y MergeTitles junta las dos
// lecturas.
func Catalog(sheet Sheet) ([]model.Title, []RowError) {
	if sheet.Kind != KindCatalog {
		return nil, []RowError{{
			Reason: "esta tabla no parece la del catálogo: espero al menos una columna de título y otra de sinopsis, año, tipo o clasificación",
		}}
	}

	var titles []model.Title
	var errs []RowError
	seen := map[string]int{}

	for _, row := range sheet.Rows {
		name := cleanName(sheet.Get(row, ColTitle))
		if name == "" || name == "-" {
			errs = append(errs, RowError{
				Line:   row.Line,
				Reason: "esta fila del catálogo no trae título, así que no supe de qué ficha era",
			})
			continue
		}
		key := strings.ToLower(name)
		if i, ok := seen[key]; ok {
			// El catálogo repite una ficha: se completa la que ya está en vez
			// de crear dos títulos iguales.
			fillTitle(&titles[i], sheet, row)
			continue
		}
		t := model.Title{
			Name:           name,
			Kind:           model.TitleProgram,
			MetadataSource: "hoja",
		}
		fillTitle(&t, sheet, row)
		seen[key] = len(titles)
		titles = append(titles, t)
	}
	return titles, errs
}

func fillTitle(t *model.Title, sheet Sheet, row Row) {
	if s := cleanText(sheet.Get(row, ColSynopsis)); s != "" && t.Synopsis == "" {
		t.Synopsis = s
	}
	if s := strings.TrimSpace(sheet.Get(row, ColContentRating)); s != "" && t.ContentRating == "" {
		t.ContentRating = s
	}
	if s := strings.TrimSpace(sheet.Get(row, ColAudienceRating)); s != "" && t.AudienceRating == "" {
		t.AudienceRating = s
	}
	if s := strings.TrimSpace(sheet.Get(row, ColGenre)); s != "" && t.Genre == "" {
		t.Genre = s
	}
	if t.Year == nil {
		if y, ok := parseYear(sheet.Get(row, ColYear)); ok {
			t.Year = &y
		}
	}
	if k, ok := parseTitleKind(sheet.Get(row, ColKind)); ok {
		t.Kind = k
	}
}

func cleanText(s string) string {
	return strings.Join(strings.Fields(unescapeMarkdown(s)), " ")
}

func parseYear(s string) (int, bool) {
	s = strings.TrimSpace(unescapeMarkdown(s))
	if len(s) > 4 {
		s = s[:4] // "1966-1970" y "1989 (remake)" cuentan como 1966 y 1989
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1800 || n > 2200 {
		return 0, false
	}
	return n, true
}

// parseTitleKind traduce lo que diga la hoja al vocabulario del modelo.
func parseTitleKind(s string) (model.TitleKind, bool) {
	switch normalize(s) {
	case "serie", "series", "show", "tv", "tv show", "temporada", "anime", "novela":
		return model.TitleSeries, true
	case "pelicula", "peli", "movie", "film", "largometraje":
		return model.TitleMovie, true
	case "programa", "program", "especial", "documental", "documentary":
		return model.TitleProgram, true
	case "promo", "promocion":
		return model.TitlePromo, true
	case "id", "identificativo":
		return model.TitleID, true
	case "spot", "anuncio", "comercial":
		return model.TitleSpot, true
	case "cortinilla", "bumper":
		return model.TitleBumper, true
	}
	return model.TitleProgram, false
}

// CountWithSynopsis cuenta cuántas fichas traen sinopsis, que es lo que se
// enseña después de importar ("118 títulos, 112 con sinopsis").
func CountWithSynopsis(titles []model.Title) int {
	n := 0
	for _, t := range titles {
		if strings.TrimSpace(t.Synopsis) != "" {
			n++
		}
	}
	return n
}

// CatalogSummary cuenta en una línea qué trajo el catálogo.
func CatalogSummary(titles []model.Title, errs []RowError) string {
	parts := []string{plural(len(titles), "%d título importado", "%d títulos importados")}
	if n := CountWithSynopsis(titles); n > 0 {
		parts = append(parts, fmt.Sprintf("%d con sinopsis", n))
	}
	if n := len(errs); n > 0 {
		parts = append(parts, plural(n, "%d fila no cuadró", "%d filas no cuadraron"))
	}
	return strings.Join(parts, ", ")
}
