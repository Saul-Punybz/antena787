package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"antena787/internal/model"
)

// UserAgent es lo que se identifica ante los servicios de fichas.
// MusicBrainz lo exige por sus términos; los demás lo agradecen.
var UserAgent = "Antena787/1.0 (playout libre; https://github.com/antena787)"

// ProviderConfig enciende drivers de red. El valor cero no enciende
// ninguno, que es el default del PRD: la instalación funciona entera sin
// internet y sin que nadie saque una clave (§10).
type ProviderConfig struct {
	CoverArtArchive bool          // música, sin clave
	TVmaze          bool          // series de TV, sin clave
	TMDBAPIKey      string        // el de mejores datos; pide clave y atribución visible
	Timeout         time.Duration // 0 = 10 s
	Client          *http.Client  // para pruebas; 0 = uno propio con el timeout
}

// Providers arma la lista de drivers en el orden del PRD: primero los que no
// piden clave, después los que sí. Devuelve nil si no se encendió ninguno.
func Providers(cfg ProviderConfig) []Provider {
	client := cfg.Client
	if client == nil {
		to := cfg.Timeout
		if to <= 0 {
			to = 10 * time.Second
		}
		client = &http.Client{Timeout: to}
	}
	var out []Provider
	if cfg.TVmaze {
		out = append(out, &TVmaze{Client: client})
	}
	if cfg.CoverArtArchive {
		out = append(out, &CoverArtArchive{Client: client})
	}
	if cfg.TMDBAPIKey != "" {
		out = append(out, &TMDB{APIKey: cfg.TMDBAPIKey, Client: client})
	}
	return out
}

// ── nivel 2 · sin clave ───────────────────────────────────────────────

// TVmaze es el driver de series de TV. No pide clave, no pide registro y su
// API es pública. Es el default para televisión (PRD §10, F1-09).
type TVmaze struct {
	Client  *http.Client
	BaseURL string // vacío = https://api.tvmaze.com
}

// TVmazeBaseURL es el servicio real.
const TVmazeBaseURL = "https://api.tvmaze.com"

// Lookup pregunta por el nombre de la serie. Devuelve (nil, nil) si el
// archivo no es una serie o si TVmaze no la conoce.
func (t *TVmaze) Lookup(ctx context.Context, q Query) (*Card, error) {
	if q.Name == "" || (q.Kind != "" && q.Kind != model.TitleSeries && q.Kind != model.TitleProgram) {
		return nil, nil
	}
	base := or(t.BaseURL, TVmazeBaseURL)
	u := base + "/singlesearch/shows?q=" + url.QueryEscape(q.Name)
	var show struct {
		Name      string   `json:"name"`
		Premiered string   `json:"premiered"`
		Summary   string   `json:"summary"`
		Genres    []string `json:"genres"`
		Image     struct {
			Original string `json:"original"`
			Medium   string `json:"medium"`
		} `json:"image"`
		Rating struct {
			Average float64 `json:"average"`
		} `json:"rating"`
	}
	if err := getJSON(ctx, t.Client, u, &show); err != nil {
		return nil, err
	}
	if show.Name == "" {
		return nil, nil
	}
	c := &Card{
		Name:       show.Name,
		Kind:       model.TitleSeries,
		Synopsis:   stripHTML(show.Summary),
		Year:       yearOf(show.Premiered),
		ArtworkURL: firstNonEmpty(show.Image.Original, show.Image.Medium),
		Source:     "tvmaze",
	}
	if len(show.Genres) > 0 {
		c.Genre = show.Genres[0]
	}
	c.Sources = []string{"tvmaze"}
	return c, nil
}

// CoverArtArchive es el driver de carátulas de música, por MBID de
// MusicBrainz. No pide clave. Solo trae imagen: los datos del disco vienen
// de las etiquetas del archivo, que para música casi siempre están.
type CoverArtArchive struct {
	Client  *http.Client
	BaseURL string // vacío = https://coverartarchive.org
}

// CoverArtBaseURL es el servicio real.
const CoverArtBaseURL = "https://coverartarchive.org"

// Lookup solo sabe responder si el archivo trae el MBID del lanzamiento.
// Sin MBID no hay búsqueda por nombre aquí: eso es MusicBrainz, otro
// servicio, con su propio límite de peticiones.
func (c *CoverArtArchive) Lookup(ctx context.Context, q Query) (*Card, error) {
	if q.MBID == "" {
		return nil, nil
	}
	u := fmt.Sprintf("%s/release/%s/front", or(c.BaseURL, CoverArtBaseURL), url.PathEscape(q.MBID))
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	resp, err := clientOf(c.Client).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}
	return &Card{ArtworkURL: u, Source: "coverart-archive", Sources: []string{"coverart-archive"}}, nil
}

// ── nivel 3 · clave gratis, atribución obligatoria ────────────────────

// TMDBAttribution es el texto que hay que enseñar visible en el producto si
// se usa TMDB. Es obligación de sus términos, y por eso vive aquí y no en un
// archivo de configuración que nadie lee (PRD §10).
const TMDBAttribution = "Este producto usa la API de TMDB pero no está avalado ni certificado por TMDB."

// TMDB es el driver de mejores datos —pósters en alta, fondos, reparto— y es
// gratis incluso para uso comercial, pero pide una clave y su atribución
// visible. En F1 la interfaz queda documentada y el driver, sin implementar:
// no se enciende nada que obligue a nadie a registrarse (PRD §10, nivel 3).
type TMDB struct {
	APIKey  string
	Client  *http.Client
	BaseURL string // vacío = https://api.themoviedb.org/3
}

// Lookup todavía no consulta nada. Devuelve el error diciendo por qué, para
// que quien lo encienda sepa exactamente qué falta.
func (t *TMDB) Lookup(ctx context.Context, q Query) (*Card, error) {
	return nil, Plainf(nil, "todavía no busco fichas en TMDB: llega en F2, con su clave y su crédito visible en pantalla")
}

// ── ayudas ────────────────────────────────────────────────────────────

func clientOf(c *http.Client) *http.Client {
	if c == nil {
		return &http.Client{Timeout: 10 * time.Second}
	}
	return c
}

func getJSON(ctx context.Context, client *http.Client, u string, into any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := clientOf(client).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		return Plainf(nil, "el servicio de fichas respondió %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	return json.Unmarshal(body, into)
}

// stripHTML quita las etiquetas de un resumen. TVmaze devuelve el suyo con
// <p> y <b> dentro, y eso no se dibuja bien en una guía.
func stripHTML(s string) string {
	var b strings.Builder
	depth := 0
	for _, r := range s {
		switch {
		case r == '<':
			depth++
		case r == '>':
			if depth > 0 {
				depth--
			}
		case depth == 0:
			b.WriteRune(r)
		}
	}
	out := strings.NewReplacer("&amp;", "&", "&quot;", `"`, "&#39;", "'", "&lt;", "<", "&gt;", ">", "&nbsp;", " ").Replace(b.String())
	return strings.Join(strings.Fields(out), " ")
}
