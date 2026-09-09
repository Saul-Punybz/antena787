package ingest

import (
	"context"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strings"

	"antena787/internal/model"
)

// Card es una ficha: lo poco que hace falta para que un título se vea bien
// en Biblioteca y en la guía. Se llena por capas —etiquetas, .nfo, carátula
// local, y solo al final la red— y cada capa rellena únicamente lo que la
// anterior dejó vacío.
type Card struct {
	Name           string
	Kind           model.TitleKind
	Synopsis       string
	Year           int
	Genre          string
	ContentRating  string // la del contenido: "TV-14", "PG-13"
	AudienceRating string // la de audiencia, si el proveedor la da

	Show        string // serie a la que pertenece, si es un episodio
	Season      int
	Episode     int
	EpisodeName string

	ArtworkPath string // carátula que ya está en disco
	ArtworkURL  string // carátula que habría que bajar (solo la pone la red)

	Source      string   // driver que aportó lo principal
	Sources     []string // todos los que aportaron algo, en orden
	Attribution string   // texto que hay que enseñar visible si el driver lo exige
}

// Empty dice si la ficha no tiene nada que valga la pena.
func (c Card) Empty() bool {
	return c.Name == "" && c.Synopsis == "" && c.ArtworkPath == "" && c.ArtworkURL == ""
}

// Query es lo que se le pregunta a un proveedor de fichas.
type Query struct {
	Name    string
	Kind    model.TitleKind
	Year    int
	Season  int
	Episode int
	Artist  string
	Album   string
	MBID    string // MusicBrainz release id, si el archivo lo traía
}

// Provider es un driver de fichas y carátulas (PRD §10). Devolver
// (nil, nil) es la forma de decir "no sé de esto", que no es un error.
type Provider interface {
	Lookup(ctx context.Context, q Query) (*Card, error)
}

// MetadataDeps es lo que Metadata necesita de fuera. Providers vacío
// —que es el default— significa que no se toca la red.
type MetadataDeps struct {
	FFmpeg     string // opcional: si está, se extrae la carátula embebida
	ArtworkDir string // dónde dejar la carátula extraída; vacío = al lado del archivo
	Providers  []Provider
}

// ArtworkNames son los nombres de carátula que se buscan en la carpeta del
// archivo, en orden. Es la convención de Kodi, Plex y Jellyfin.
var ArtworkNames = []string{"poster.jpg", "poster.png", "folder.jpg", "folder.png", "cover.jpg", "cover.png", "cover.jpeg"}

// Metadata busca la ficha del archivo en el orden del PRD §10: local
// primero, red al final. Nunca falla por no encontrar nada — devuelve una
// ficha con lo que haya, aunque sea solo el nombre del archivo.
func Metadata(ctx context.Context, path string, m Measure, deps MetadataDeps) (Card, error) {
	var card Card

	// 1 · etiquetas embebidas (sin red)
	if tagCard := FromTags(m); !tagCard.Empty() {
		card.mergeFrom(tagCard)
	}

	// 2 · .nfo de Kodi al lado (sin red)
	if p, ok := FindNFO(path); ok {
		nfoCard, err := ReadNFO(p)
		if err == nil {
			card.mergeFrom(nfoCard)
		}
	}

	// 3 · carátula local, y si no, la que venga dentro del contenedor
	if card.ArtworkPath == "" {
		if p, ok := FindArtwork(path); ok {
			card.ArtworkPath = p
			card.addSource("caratula-local")
		}
	}
	if card.ArtworkPath == "" && deps.FFmpeg != "" {
		if p, err := ExtractEmbeddedArtwork(ctx, deps.FFmpeg, path, deps.ArtworkDir); err == nil && p != "" {
			card.ArtworkPath = p
			card.addSource("caratula-embebida")
		}
	}

	// El nombre del archivo es el último recurso local, y casi siempre el
	// mejor dato que hay en una estación pequeña.
	if card.Name == "" {
		card.Name = NameFromFile(path)
		card.addSource("nombre-de-archivo")
	}
	if card.Kind == "" {
		card.Kind = GuessKind(m, card)
	}

	// 4 · la red, solo si falta algo y solo si alguien encendió un driver
	if len(deps.Providers) == 0 || (card.Synopsis != "" && (card.ArtworkPath != "" || card.ArtworkURL != "")) {
		return card, nil
	}
	q := Query{
		Name:    firstNonEmpty(card.Show, card.Name),
		Kind:    card.Kind,
		Year:    card.Year,
		Season:  card.Season,
		Episode: card.Episode,
		Artist:  m.Tags.Artist,
		Album:   m.Tags.Album,
		MBID:    m.Tags.MBID,
	}
	for _, p := range deps.Providers {
		if err := ctx.Err(); err != nil {
			return card, nil
		}
		got, err := p.Lookup(ctx, q)
		if err != nil || got == nil {
			continue // que un servicio esté caído no rompe un ingest
		}
		card.mergeFrom(*got)
		if card.Synopsis != "" && (card.ArtworkPath != "" || card.ArtworkURL != "") {
			break
		}
	}
	return card, nil
}

// FromTags arma una ficha con las etiquetas embebidas (F1-08: con esto no se
// hace ni una llamada de red).
func FromTags(m Measure) Card {
	t := m.Tags
	year := t.Year
	if year == 0 {
		year = yearOf(t.Date)
	}
	c := Card{
		Name:        firstNonEmpty(t.Title, t.Album),
		Synopsis:    t.Synopsis,
		Year:        year,
		Genre:       t.Genre,
		Show:        t.Show,
		Season:      t.Season,
		Episode:     t.Episode,
		EpisodeName: t.EpisodeName,
	}
	if t.Show != "" || t.Season > 0 || t.Episode > 0 {
		c.Kind = model.TitleSeries
		if c.EpisodeName == "" && t.Title != "" && t.Show != "" {
			c.EpisodeName = t.Title
			c.Name = t.Show
		}
	}
	if !c.Empty() {
		c.Source = "tags-embebidas"
		c.Sources = []string{"tags-embebidas"}
	}
	return c
}

// FindNFO busca el .nfo de Kodi: primero el del mismo nombre, luego los de
// la carpeta.
func FindNFO(path string) (string, bool) {
	dir := filepath.Dir(path)
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	for _, cand := range []string{base + ".nfo", "movie.nfo", "tvshow.nfo"} {
		p := filepath.Join(dir, cand)
		if fileHasBytes(p) {
			return p, true
		}
	}
	// Windows no distingue mayúsculas pero Linux sí: se mira la carpeta.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	want := strings.ToLower(base) + ".nfo"
	for _, e := range entries {
		if !e.IsDir() && strings.ToLower(e.Name()) == want {
			return filepath.Join(dir, e.Name()), true
		}
	}
	return "", false
}

// ReadNFO lee un .nfo de Kodi. El parser es mínimo a propósito: acepta
// <movie>, <tvshow> y <episodedetails>, ignora todo lo demás y no se cae por
// una etiqueta rara, que es lo normal en archivos escritos a mano.
func ReadNFO(path string) (Card, error) {
	f, err := os.Open(path)
	if err != nil {
		return Card{}, Plainf(err, "no se puede leer la ficha %q", trimName(path))
	}
	defer f.Close()
	return ParseNFO(f)
}

// ParseNFO es ReadNFO sobre cualquier lector.
func ParseNFO(r io.Reader) (Card, error) {
	var doc nfoDoc
	dec := xml.NewDecoder(r)
	dec.Strict = false
	dec.CharsetReader = func(_ string, in io.Reader) (io.Reader, error) { return in, nil }
	if err := dec.Decode(&doc); err != nil {
		return Card{}, Plainf(err, "la ficha .nfo no se entiende: el XML está mal formado")
	}
	root := strings.ToLower(doc.XMLName.Local)
	switch root {
	case "movie", "tvshow", "episodedetails", "musicvideo":
	default:
		return Card{}, Plainf(nil, "la ficha .nfo no es de Kodi: se esperaba <movie>, <tvshow> o <episodedetails> y venía <%s>", doc.XMLName.Local)
	}
	c := Card{
		Name:          strings.TrimSpace(doc.Title),
		Synopsis:      firstNonEmpty(strings.TrimSpace(doc.Plot), strings.TrimSpace(doc.Outline)),
		Year:          doc.Year,
		ContentRating: strings.TrimSpace(doc.MPAA),
		Show:          strings.TrimSpace(doc.ShowTitle),
		Season:        doc.Season,
		Episode:       doc.Episode,
		Source:        "nfo-local",
		Sources:       []string{"nfo-local"},
	}
	if c.Year == 0 {
		c.Year = yearOf(firstNonEmpty(doc.Premiered, doc.Aired, doc.ReleaseDate))
	}
	if len(doc.Genre) > 0 {
		c.Genre = strings.TrimSpace(doc.Genre[0])
	}
	switch root {
	case "movie":
		c.Kind = model.TitleMovie
	case "tvshow":
		c.Kind = model.TitleSeries
	case "episodedetails":
		c.Kind = model.TitleSeries
		c.EpisodeName = c.Name
		if c.Show != "" {
			c.Name = c.Show
		}
	}
	for _, t := range doc.Thumb {
		if u := strings.TrimSpace(t.Value); u != "" && strings.HasPrefix(u, "http") {
			c.ArtworkURL = u
			break
		}
	}
	return c, nil
}

type nfoDoc struct {
	XMLName     xml.Name
	Title       string   `xml:"title"`
	ShowTitle   string   `xml:"showtitle"`
	Plot        string   `xml:"plot"`
	Outline     string   `xml:"outline"`
	Year        int      `xml:"year"`
	Premiered   string   `xml:"premiered"`
	Aired       string   `xml:"aired"`
	ReleaseDate string   `xml:"releasedate"`
	Genre       []string `xml:"genre"`
	MPAA        string   `xml:"mpaa"`
	Season      int      `xml:"season"`
	Episode     int      `xml:"episode"`
	Thumb       []struct {
		Aspect string `xml:"aspect,attr"`
		Value  string `xml:",chardata"`
	} `xml:"thumb"`
}

// FindArtwork busca la carátula en la carpeta del archivo: primero la que
// lleva su nombre, después las convenciones de la carpeta.
func FindArtwork(path string) (string, bool) {
	dir := filepath.Dir(path)
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	for _, suffix := range []string{"-poster", "-thumb", ""} {
		for _, ext := range []string{".jpg", ".jpeg", ".png", ".webp"} {
			p := filepath.Join(dir, base+suffix+ext)
			if fileHasBytes(p) {
				return p, true
			}
		}
	}
	for _, name := range ArtworkNames {
		p := filepath.Join(dir, name)
		if fileHasBytes(p) {
			return p, true
		}
	}
	return "", false
}

// ExtractEmbeddedArtwork saca la carátula que viene dentro del contenedor
// (el nivel 1 del PRD §10, "caratula-embebida"). Si no hay ninguna devuelve
// ruta vacía y sin error: no tenerla es lo normal.
func ExtractEmbeddedArtwork(ctx context.Context, ffmpeg, path, outDir string) (string, error) {
	if outDir == "" {
		outDir = filepath.Dir(path)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	dst := filepath.Join(outDir, base+"-cover.jpg")
	cctx, cancel := context.WithTimeout(ctx, ProbeTimeout)
	defer cancel()
	err := runFFmpeg(cctx, ffmpeg, []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y",
		"-i", path, "-map", "0:v", "-map", "-0:V", "-frames:v", "1", "-c:v", "mjpeg", "-f", "image2", dst})
	if err != nil || !fileHasBytes(dst) {
		_ = os.Remove(dst)
		return "", err
	}
	return dst, nil
}

// NameFromFile convierte "Kojak.S01E03.1080p.WEB-DL.mkv" en algo que se
// pueda leer. No adivina temporadas: eso lo hace el resolver de nombres del
// importador, que es otro paquete.
func NameFromFile(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	base = strings.NewReplacer("_", " ", ".", " ").Replace(base)
	return strings.Join(strings.Fields(base), " ")
}

// GuessKind decide qué es esto cuando nadie lo dijo. Es una suposición
// declarada, no un dato: la persona la corrige en Biblioteca.
func GuessKind(m Measure, c Card) model.TitleKind {
	switch {
	case c.Season > 0 || c.Episode > 0 || c.Show != "":
		return model.TitleSeries
	case !m.HasVideo:
		return model.TitleProgram
	case m.DurationMs > 0 && m.DurationMs <= 120_000:
		return model.TitleSpot
	case m.DurationMs >= 45*60_000:
		return model.TitleMovie
	default:
		return model.TitleProgram
	}
}

// mergeFrom rellena solo lo que está vacío: la primera capa que aporta un
// dato es la que manda. Local antes que red, siempre.
func (c *Card) mergeFrom(o Card) {
	changed := false
	set := func(dst *string, v string) {
		if *dst == "" && strings.TrimSpace(v) != "" {
			*dst = strings.TrimSpace(v)
			changed = true
		}
	}
	setInt := func(dst *int, v int) {
		if *dst == 0 && v != 0 {
			*dst = v
			changed = true
		}
	}
	set(&c.Name, o.Name)
	set(&c.Synopsis, o.Synopsis)
	set(&c.Genre, o.Genre)
	set(&c.ContentRating, o.ContentRating)
	set(&c.AudienceRating, o.AudienceRating)
	set(&c.Show, o.Show)
	set(&c.EpisodeName, o.EpisodeName)
	set(&c.ArtworkPath, o.ArtworkPath)
	set(&c.ArtworkURL, o.ArtworkURL)
	set(&c.Attribution, o.Attribution)
	setInt(&c.Year, o.Year)
	setInt(&c.Season, o.Season)
	setInt(&c.Episode, o.Episode)
	if c.Kind == "" && o.Kind != "" {
		c.Kind = o.Kind
		changed = true
	}
	if changed && o.Source != "" {
		c.addSource(o.Source)
	}
}

func (c *Card) addSource(s string) {
	if s == "" {
		return
	}
	for _, have := range c.Sources {
		if have == s {
			return
		}
	}
	c.Sources = append(c.Sources, s)
	if c.Source == "" {
		c.Source = s
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
