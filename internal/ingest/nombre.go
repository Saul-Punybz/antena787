package ingest

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"antena787/internal/model"
)

// La ficha que se saca del nombre del archivo.
//
// En una estación pequeña casi nada llega etiquetado: lo que hay es el
// nombre con el que el archivo apareció en la carpeta. Ese nombre casi
// siempre trae la serie, la temporada y el episodio («Kojak.S01E03.El.caso»,
// «Kojak - 1x03 - El caso», «Kojak T1E3»), o el título de la película y su
// año («Casablanca.1942»), y después una cola de palabras técnicas —1080p,
// x265, WEB-DL, el grupo entre corchetes— que no son de nadie. Aquí se lee lo
// primero y se tira lo segundo. Es la capa local que va antes que cualquier
// red (F1-08) y detrás de las etiquetas y del .nfo cuando estos dicen algo
// de verdad.

var (
	// S04E01, s4e1, S04E01E02 (doble: se queda con el primero), S04.E01.
	reSxxExx = regexp.MustCompile(`(?i)(?:^|[\s._\-\[(])S(\d{1,2})[\s._-]?E(\d{1,3})(?:[\s._-]?E\d{1,3})?(?:[\s._\-\])]|$)`)
	// 4x01, 04x01.
	reNxNN = regexp.MustCompile(`(?i)(?:^|[\s._\-\[(])(\d{1,2})x(\d{2,3})(?:[\s._\-\])]|$)`)
	// T1E4 (como lo escribe Antena787), Temporada 1 Episodio 4, Season 1
	// Episode 4, Temp 1 Cap 4.
	reTemporada = regexp.MustCompile(`(?i)(?:^|[\s._\-\[(])(?:T|Temp(?:orada)?|Season)[\s._-]?(\d{1,2})[\s._-]*(?:E|Ep(?:isodio|isode)?|Cap(?:[ií]tulo)?)[\s._-]?(\d{1,3})(?:[\s._\-\])]|$)`)
	// Episodio 12, Ep 12, Cap 12, Capítulo 12: sin temporada es la 1.
	reEpisodioSuelto = regexp.MustCompile(`(?i)(?:^|[\s._\-\[(])(?:Ep(?:isodio|isode)?|Cap(?:[ií]tulo)?)[\s._-]?(\d{1,3})(?:[\s._\-\])]|$)`)
	// «Serie - 04» al final o antes de la cola técnica (lo usan los subgrupos
	// de anime): un número solo, separado por guion.
	reGuionNumero = regexp.MustCompile(`\s-\s(\d{1,3})(?:\s|$)`)
	// Un año de cuatro cifras separado del resto.
	reAnio = regexp.MustCompile(`(?:^|[\s._\-\[(])((?:19|20)\d{2})(?:[\s._\-\])]|$)`)
	// Lo que va entre corchetes o llaves es del grupo que lo subió, nunca
	// del programa.
	reCorchetes = regexp.MustCompile(`\[[^\]]*\]|\{[^}]*\}`)
	// La cola técnica: desde la primera de estas palabras hasta el final no
	// hay nada que sea del programa.
	reColaTecnica = regexp.MustCompile(`(?i)(?:^|[\s._\-(])(?:480p|576p|720p|1080p|1080i|2160p|4k|uhd|hdr|hdr10|hevc|x264|x265|h\.?264|h\.?265|av1|aac|aac2\.?0|ac3|eac3|dts|dd5\.?1|ddp5\.?1|web-?dl|webrip|bluray|blu-ray|bdrip|brrip|dvdrip|hdtv|hdrip|remux|proper|repack|internal|extended|msubs|esubs|latino|castellano|jpn|eng|spa|lat|amzn|dsnp|hmax|atvp|pmntp|10bit|8bit|xvid|divx)(?:[\s._\-)]|$)`)
	// «-MeGusta», «-ELiTE» pegado al final: la firma del grupo.
	reGrupoFinal = regexp.MustCompile(`-[A-Za-z0-9]+$`)
)

// FichaDesdeNombre lee el nombre de un archivo y devuelve la ficha que se
// puede sacar de él: serie, temporada, episodio y nombre del episodio; o
// título y año de una película. Nunca falla: si el nombre no dice nada, la
// ficha vuelve con el nombre legible y sin tipo, y otra capa decidirá.
func FichaDesdeNombre(path string) Card {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return FichaDesdeTexto(base)
}

// FichaDesdeTexto es FichaDesdeNombre sobre un texto que ya no lleva carpeta
// ni extensión: sirve para leer una etiqueta de título que en realidad es el
// nombre del archivo otra vez.
func FichaDesdeTexto(base string) Card {
	c := Card{}
	texto := strings.TrimSpace(base)
	if texto == "" {
		return c
	}

	if antes, temporada, episodio, despues, ok := marcaDeEpisodio(texto); ok {
		c.Kind = model.TitleSeries
		c.Season, c.Episode = temporada, episodio
		c.Show = limpiarNombre(antes)
		c.EpisodeName = limpiarNombre(despues)
		c.Name = c.Show
		if c.Name == "" {
			// «S01E02.mkv» a secas: no hay serie que nombrar; el nombre
			// legible del archivo es lo único que queda.
			c.Name = NameFromFile(base)
			c.Show = ""
		}
		c.Source = "nombre-de-archivo"
		c.Sources = []string{c.Source}
		return c
	}

	nombre, anio := nombreYAnio(texto)
	c.Name = nombre
	c.Year = anio
	if c.Name == "" {
		c.Name = NameFromFile(base)
	}
	c.Source = "nombre-de-archivo"
	c.Sources = []string{c.Source}
	return c
}

// marcaDeEpisodio busca la temporada y el episodio en el texto y devuelve lo
// que hay antes y después de la marca.
func marcaDeEpisodio(texto string) (antes string, temporada, episodio int, despues string, ok bool) {
	for _, re := range []*regexp.Regexp{reSxxExx, reNxNN, reTemporada} {
		loc := re.FindStringSubmatchIndex(texto)
		if loc == nil {
			continue
		}
		t, _ := strconv.Atoi(texto[loc[2]:loc[3]])
		e, _ := strconv.Atoi(texto[loc[4]:loc[5]])
		return texto[:loc[0]], t, e, texto[loc[5]:], true
	}
	if loc := reEpisodioSuelto.FindStringSubmatchIndex(texto); loc != nil {
		e, _ := strconv.Atoi(texto[loc[2]:loc[3]])
		return texto[:loc[0]], 1, e, texto[loc[3]:], true
	}
	// «Serie - 04» solo cuenta si el número no es un año ni parte de la
	// cola técnica: se mira sobre el texto ya limpio de corchetes.
	limpio := reCorchetes.ReplaceAllString(texto, " ")
	if loc := reGuionNumero.FindStringSubmatchIndex(limpio); loc != nil {
		e, _ := strconv.Atoi(limpio[loc[2]:loc[3]])
		if e > 0 {
			return limpio[:loc[0]], 1, e, limpio[loc[3]:], true
		}
	}
	return "", 0, 0, "", false
}

// nombreYAnio separa el título de una película de su año, si lo trae. El
// año tiene que ser plausible (hasta el que viene) y estar separado del
// resto; si el título empieza por el año («2001 Odisea del espacio») no es
// un año, es parte del nombre.
func nombreYAnio(texto string) (string, int) {
	limpio := limpiarNombre(texto)
	sinCorchetes := reCorchetes.ReplaceAllString(texto, " ")
	tope := time.Now().Year() + 1
	var elegido []int
	for _, loc := range reAnio.FindAllStringSubmatchIndex(sinCorchetes, -1) {
		y, _ := strconv.Atoi(sinCorchetes[loc[2]:loc[3]])
		if y < 1900 || y > tope {
			continue
		}
		if strings.TrimSpace(limpiarNombre(sinCorchetes[:loc[2]])) == "" {
			continue // el año abre el nombre: es parte de él
		}
		elegido = loc
	}
	if elegido == nil {
		return limpio, 0
	}
	y, _ := strconv.Atoi(sinCorchetes[elegido[2]:elegido[3]])
	nombre := limpiarNombre(sinCorchetes[:elegido[2]])
	if nombre == "" {
		return limpio, 0
	}
	return nombre, y
}

// limpiarNombre convierte un trozo de nombre de archivo en algo que se
// pueda leer: quita corchetes, corta la cola técnica y la firma del grupo,
// cambia puntos y guiones bajos por espacios y recorta los separadores.
func limpiarNombre(s string) string {
	s = reCorchetes.ReplaceAllString(s, " ")
	if loc := reColaTecnica.FindStringIndex(s); loc != nil {
		s = s[:loc[0]]
	}
	s = strings.TrimSpace(s)
	s = reGrupoFinal.ReplaceAllString(s, "")
	s = strings.NewReplacer("_", " ", ".", " ").Replace(s)
	s = strings.Join(strings.Fields(s), " ")
	s = strings.Trim(s, " -–—:([")
	return strings.TrimSpace(s)
}

// PareceNombreDeArchivo dice si un texto es un nombre de archivo con otro
// disfraz: trae una marca de episodio, una cola técnica, o va con puntos en
// vez de espacios. Muchos archivos llevan de título embebido su propio
// nombre con puntos («For.All.Mankind.S05E09»), y eso no es un título.
func PareceNombreDeArchivo(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if reSxxExx.MatchString(s) || reNxNN.MatchString(s) || reColaTecnica.MatchString(s) {
		return true
	}
	return strings.Count(s, ".") >= 2 && !strings.Contains(s, " ")
}

// conNombreLegible corrige la ficha de las etiquetas con lo que dice el
// nombre del archivo:
//
//   - si las etiquetas traen serie, temporada o episodio, son un etiquetado
//     de verdad y mandan;
//   - si el título embebido es el nombre del archivo con puntos, se lee como
//     nombre de archivo y no como título;
//   - si el título embebido es limpio pero no dice temporada ni episodio, y
//     el nombre del archivo sí, el título es el del episodio y la serie es la
//     del nombre (la misma regla que usa FromTags con show + title).
func conNombreLegible(tags, file Card) Card {
	if tags.Show != "" || tags.Season > 0 || tags.Episode > 0 {
		return tags
	}
	if tags.Name != "" && PareceNombreDeArchivo(tags.Name) {
		parsed := FichaDesdeTexto(tags.Name)
		if parsed.Season == 0 && parsed.Episode == 0 && (file.Season > 0 || file.Episode > 0) {
			parsed = file
		}
		tags.Name, tags.Show = parsed.Name, parsed.Show
		tags.Season, tags.Episode, tags.EpisodeName = parsed.Season, parsed.Episode, parsed.EpisodeName
		if tags.Year == 0 {
			tags.Year = parsed.Year
		}
		if parsed.Kind != "" {
			tags.Kind = parsed.Kind
		}
		tags.addSource("nombre-de-archivo")
		return tags
	}
	if tags.Name != "" && (file.Season > 0 || file.Episode > 0) && file.Show != "" {
		tags.Kind = model.TitleSeries
		// Si el nombre del archivo ya trae el nombre del episodio, ese es el
		// bueno: el título embebido de un archivo así suele ser la firma de
		// quien lo subió («by ToonsHub»), no un título.
		tags.EpisodeName = firstNonEmpty(file.EpisodeName, tags.Name)
		tags.Name, tags.Show = file.Show, file.Show
		tags.Season, tags.Episode = file.Season, file.Episode
		tags.addSource("nombre-de-archivo")
	}
	return tags
}
