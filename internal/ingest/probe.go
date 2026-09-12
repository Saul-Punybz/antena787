package ingest

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Measure es todo lo que ffprobe sabe decir de un archivo. Es la materia
// prima de media_asset: de aquí salen codec, resolucion, fps, canales_audio
// y duracion_medida_ms.
type Measure struct {
	Path      string // ruta que se midió
	SizeBytes int64  // tamaño en disco
	Hash      string // SHA-256 en hexadecimal; vacío si no se pidió

	Container string // nombre corto del contenedor: "mov,mp4,m4a,..."

	HasVideo    bool
	Codec       string // códec de video: "h264", "mpeg2video", …
	Profile     string
	Width       int
	Height      int
	Resolution  string // "1280x720"; vacío si no hay video
	FPS         string // exacto y en quebrado: "30000/1001"
	FPSFloat    float64
	PixelFormat string
	Interlaced  bool // field_order dice tt/bb/tb/bt

	HasAudio      bool
	AudioCodec    string
	AudioChannels int
	SampleRate    int

	// AudioTracks son todas las pistas de sonido del archivo, en el orden en
	// que vienen. La primera es la que describen AudioCodec, AudioChannels y
	// SampleRate; las demás existen para que una persona pueda elegir cuál
	// sale al aire (F1-60).
	AudioTracks []AudioTrack

	DurationMs int64 // duración real al milisegundo (video; si no hay, formato)
	VideoMs    int64 // lo que dura la imagen según su stream; 0 si no hay o no lo dice
	AudioMs    int64 // lo que dura el sonido según su stream; 0 si no hay o no lo dice
	BitRate    int64

	HasCaptions   bool
	CaptionFormat string // "cea-608", "subrip", "mov_text", "webvtt", …
	CaptionStream int    // índice del stream de subtítulos; -1 si van dentro del video

	Tags Tags
}

// AudioTrack es una pista de sonido del archivo tal como la ve ffprobe.
// Index es su posición entre las pistas de sonido —la N de «a:N»—, no el
// número de stream del contenedor: es lo que hay que decirle a ffmpeg para
// elegirla. Language viene de la etiqueta del archivo, ya en minúsculas y
// con «spa» y «eng» pasados a «es» y «en»; vacío cuando el archivo no lo
// dice. Title es el nombre que le puso quien lo armó ("comentario",
// "descriptivo", …), vacío si no trae.
type AudioTrack struct {
	Index    int
	Language string
	Channels int
	Title    string
}

// Tags son las etiquetas embebidas que sirven para armar la ficha sin red.
type Tags struct {
	Title       string
	Show        string // serie a la que pertenece
	EpisodeName string
	Season      int
	Episode     int
	Date        string // tal como venía: "2019", "2019-04-01", …
	Year        int
	Synopsis    string // synopsis, description o comment
	Artist      string
	Album       string
	Genre       string
	MBID        string // MusicBrainz release id, si el archivo lo trae
	Raw         map[string]string
}

// ProbeTimeout es lo máximo que se espera por un ffprobe. Un archivo roto
// puede dejar a ffprobe pensando mucho rato; nadie tiene ese tiempo.
const ProbeTimeout = 60 * time.Second

// Probe mide un archivo con ffprobe. No calcula el hash —eso es leer el
// archivo entero— ni toca la red. Si el archivo no se puede leer devuelve un
// PlainError con el motivo que se le enseña a una persona.
func Probe(ctx context.Context, ffprobe, path string) (Measure, error) {
	return probeWith(ctx, ffprobe, path, nil, ProbeTimeout)
}

// probeWith es Probe con argumentos extra —el portal le pasa
// -protocol_whitelist file— y un límite de tiempo propio.
func probeWith(ctx context.Context, ffprobe, path string, extra []string, timeout time.Duration) (Measure, error) {
	return probeDe(ctx, ffprobe, path, extra, timeout, false)
}

// probeDe es probeWith con la diferencia que importa: si lo que se mide está
// **en la red** en vez de en el disco.
//
// Las comprobaciones de archivo de abajo —que exista, que no sea una carpeta,
// que no esté vacío— son buenas para un archivo y **mentira para una
// dirección**: una URL no pasa un os.Stat, así que salía «no se puede abrir el
// archivo» de algo que no es un archivo, y ffprobe no llegaba a correr nunca.
// Exactamente el tipo de pista falsa que hace perder una tarde (Rolando, 11
// sept 2026: «el problema era por el nombre del archivo, no porque tuviera un
// error de codecs»).
func probeDe(ctx context.Context, ffprobe, path string, extra []string, timeout time.Duration, remoto bool) (Measure, error) {
	m := Measure{Path: path, CaptionStream: -1}
	if !remoto {
		st, err := os.Stat(path)
		if err != nil {
			return m, Plainf(err, "no se puede abrir el archivo %q", trimName(path))
		}
		if st.IsDir() {
			return m, Plainf(nil, "%q es una carpeta, no un archivo", trimName(path))
		}
		m.SizeBytes = st.Size()
		if m.SizeBytes == 0 {
			return m, Plainf(nil, "el archivo está vacío (0 bytes): la copia no terminó o el origen falló")
		}
	}

	cctx := ctx
	if timeout > 0 {
		var cancel context.CancelFunc
		cctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	args := []string{"-hide_banner", "-loglevel", "error", "-print_format", "json",
		"-show_format", "-show_streams"}
	args = append(args, extra...)
	args = append(args, path)

	cmd := exec.CommandContext(cctx, ffprobe, args...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	cmd.Stdin = nil
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(errb.String())
		if detail == "" {
			detail = err.Error()
		}
		if cctx.Err() == context.DeadlineExceeded {
			if remoto {
				return m, Plainf(fmt.Errorf("%s", detail), "la dirección no contestó a tiempo")
			}
			return m, Plainf(fmt.Errorf("%s", detail), "el archivo tardó demasiado en abrirse: puede estar dañado o en un disco que no responde")
		}
		if remoto {
			return m, Plainf(fmt.Errorf("%s", detail), "no se pudo abrir la señal de esa dirección")
		}
		return m, Plainf(fmt.Errorf("%s", detail), "el archivo no se puede leer: está incompleto o dañado")
	}

	var raw probeJSON
	if err := json.Unmarshal(out.Bytes(), &raw); err != nil {
		return m, Plainf(err, "el archivo no se puede leer: está incompleto o dañado")
	}
	fill(&m, raw)
	if !m.HasVideo && !m.HasAudio {
		return m, Plainf(nil, "el archivo no tiene ni imagen ni sonido: no es un video ni un audio")
	}
	return m, nil
}

// HashFile calcula el SHA-256 del archivo. Es opcional: se corre una vez, en
// el ingest, y sirve para reconocer un archivo que cambió de sitio o de
// nombre. En un archivo de 4 GB tarda lo que tarda leerlo entero.
func HashFile(ctx context.Context, path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", Plainf(err, "no se puede leer el archivo para identificarlo")
	}
	defer f.Close()
	h := sha256.New()
	buf := make([]byte, 1<<20)
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		n, err := f.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", Plainf(err, "el archivo se cortó a mitad de la lectura")
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// CheckDecodable abre medio segundo del archivo con ffmpeg de verdad. ffprobe
// reconoce códecs que ffmpeg no sabe decodificar; esto lo distingue antes de
// que el archivo llegue a la parrilla.
func CheckDecodable(ctx context.Context, ffmpeg, path string) error {
	cctx, cancel := context.WithTimeout(ctx, ProbeTimeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, ffmpeg, "-nostdin", "-hide_banner", "-loglevel", "error",
		"-xerror", "-t", "0.5", "-i", path, "-f", "null", "-")
	var errb bytes.Buffer
	cmd.Stderr = &errb
	cmd.Stdin = nil
	if err := cmd.Run(); err != nil {
		return Plainf(fmt.Errorf("%s", strings.TrimSpace(errb.String())),
			"este ffmpeg no sabe abrir el contenido de %q: conviértelo antes de subirlo", trimName(path))
	}
	return nil
}

// ── el JSON de ffprobe ────────────────────────────────────────────────

type probeJSON struct {
	Format struct {
		FormatName string            `json:"format_name"`
		Duration   string            `json:"duration"`
		Size       string            `json:"size"`
		BitRate    string            `json:"bit_rate"`
		Tags       map[string]string `json:"tags"`
	} `json:"format"`
	Streams []probeStream `json:"streams"`
}

type probeStream struct {
	Index        int             `json:"index"`
	CodecName    string          `json:"codec_name"`
	CodecType    string          `json:"codec_type"`
	Profile      json.RawMessage `json:"profile"`
	Width        int             `json:"width"`
	Height       int             `json:"height"`
	PixFmt       string          `json:"pix_fmt"`
	FieldOrder   string          `json:"field_order"`
	RFrameRate   string          `json:"r_frame_rate"`
	AvgFrameRate string          `json:"avg_frame_rate"`
	Duration     string          `json:"duration"`
	NBFrames     string          `json:"nb_frames"`
	Channels     int             `json:"channels"`
	SampleRate   string          `json:"sample_rate"`
	// ClosedCaptions solo lo emiten los ffprobe viejos. Desde ffprobe 9 el
	// campo desapareció de -show_streams, así que un 0 aquí no quiere decir
	// "este video no trae subtítulos dentro de la imagen": quiere decir
	// "esta versión de ffprobe no lo dice". Ver el comentario de fill.
	ClosedCaptions int               `json:"closed_captions"`
	Tags           map[string]string `json:"tags"`
	Disposition    struct {
		AttachedPic int `json:"attached_pic"`
	} `json:"disposition"`
}

func fill(m *Measure, raw probeJSON) {
	m.Container = raw.Format.FormatName
	m.BitRate = atoi64(raw.Format.BitRate)

	tags := map[string]string{}
	for k, v := range raw.Format.Tags {
		tags[strings.ToLower(k)] = v
	}

	var videoMs, audioMs int64
	for _, s := range raw.Streams {
		for k, v := range s.Tags {
			k = strings.ToLower(k)
			if _, ok := tags[k]; !ok {
				tags[k] = v
			}
		}
		switch s.CodecType {
		case "video":
			if s.attachedPic() {
				continue // es la carátula embebida, no la película
			}
			if !m.HasVideo {
				m.HasVideo = true
				m.Codec = s.CodecName
				m.Profile = unquote(s.Profile)
				m.Width, m.Height = s.Width, s.Height
				if s.Width > 0 && s.Height > 0 {
					m.Resolution = fmt.Sprintf("%dx%d", s.Width, s.Height)
				}
				m.PixelFormat = s.PixFmt
				m.FPS, m.FPSFloat = pickRate(s.RFrameRate, s.AvgFrameRate)
				switch strings.ToLower(s.FieldOrder) {
				case "tt", "bb", "tb", "bt":
					m.Interlaced = true
				}
				videoMs = msFromSeconds(s.Duration)
				if videoMs == 0 && m.FPSFloat > 0 {
					if n := atoi64(s.NBFrames); n > 0 {
						videoMs = int64(math.Round(float64(n) / m.FPSFloat * 1000))
					}
				}
				// Subtítulos CEA-608 metidos dentro de la imagen. Solo se
				// afirma que los hay cuando ffprobe lo dice: ffprobe 9 ya no
				// emite closed_captions, y en esa versión esto nunca se
				// enciende. Es un "no lo sé", no un "no los trae" — por eso
				// no se toca HasCaptions cuando el campo falta, y el ingest
				// no debe leer HasCaptions == false como certeza. Detectarlo
				// de verdad (leer las SEI A/53) y volver a insertarlo en la
				// copia de casa es F1-04, diferido a F2: ver
				// docs/ACEPTACION.md.
				if s.ClosedCaptions == 1 && !m.HasCaptions {
					m.HasCaptions = true
					m.CaptionFormat = "cea-608"
					m.CaptionStream = -1
				}
			}
		case "audio":
			m.AudioTracks = append(m.AudioTracks, AudioTrack{
				Index:    len(m.AudioTracks),
				Language: NormalizeLanguage(s.Tags["language"]),
				Channels: s.Channels,
				Title:    strings.TrimSpace(s.Tags["title"]),
			})
			if !m.HasAudio {
				m.HasAudio = true
				m.AudioCodec = s.CodecName
				m.AudioChannels = s.Channels
				m.SampleRate = int(atoi64(s.SampleRate))
				audioMs = msFromSeconds(s.Duration)
			}
		case "subtitle":
			if m.CaptionStream < 0 || !m.HasCaptions {
				m.HasCaptions = true
				m.CaptionFormat = captionName(s.CodecName)
				m.CaptionStream = s.Index
			}
		}
	}

	m.VideoMs, m.AudioMs = videoMs, audioMs
	switch {
	case videoMs > 0:
		m.DurationMs = videoMs
	case audioMs > 0:
		m.DurationMs = audioMs
	default:
		m.DurationMs = msFromSeconds(raw.Format.Duration)
	}
	m.Tags = readTags(tags)
}

// attachedPic descarta la carátula embebida, que ffprobe reporta como un
// stream de video de un solo cuadro. No es la película.
func (s probeStream) attachedPic() bool { return s.Disposition.AttachedPic == 1 }

// NormalizeLanguage deja el idioma de una pista como lo usa el resto del
// sistema: dos letras y en minúsculas. Los archivos vienen etiquetados de
// todas las maneras —«spa», «Spanish», «es-PR»—, y «und» quiere decir que
// nadie lo etiquetó, así que sale vacío.
func NormalizeLanguage(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if i := strings.IndexAny(s, "-_"); i > 0 {
		s = s[:i]
	}
	switch s {
	case "", "und", "unknown", "zxx", "mul", "mis":
		return ""
	case "spa", "esl", "spanish", "castellano", "español", "espanol":
		return "es"
	case "eng", "english", "ingles", "inglés":
		return "en"
	case "por", "portuguese":
		return "pt"
	case "fra", "fre", "french":
		return "fr"
	case "deu", "ger", "german":
		return "de"
	case "ita", "italian":
		return "it"
	}
	return s
}

// captionName traduce el nombre de ffmpeg al que usa el resto del sistema.
func captionName(codec string) string {
	switch codec {
	case "eia_608", "cea_608", "eia608":
		return "cea-608"
	case "":
		return "desconocido"
	default:
		return codec
	}
}

// pickRate elige la tasa de cuadros. r_frame_rate es la nominal y es la que
// queremos; en un archivo de tasa variable se dispara (90000/1) y entonces
// vale más la media.
func pickRate(r, avg string) (string, float64) {
	rf := rateFloat(r)
	af := rateFloat(avg)
	if rf > 0 && rf <= 1000 {
		return r, rf
	}
	if af > 0 {
		return avg, af
	}
	if rf > 0 {
		return r, rf
	}
	return "", 0
}

func rateFloat(s string) float64 {
	num, den, ok := strings.Cut(s, "/")
	if !ok {
		f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
		return f
	}
	n, _ := strconv.ParseFloat(strings.TrimSpace(num), 64)
	d, _ := strconv.ParseFloat(strings.TrimSpace(den), 64)
	if d == 0 {
		return 0
	}
	return n / d
}

func msFromSeconds(s string) int64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || f <= 0 || math.IsInf(f, 0) || math.IsNaN(f) {
		return 0
	}
	return int64(math.Round(f * 1000))
}

func atoi64(s string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n
}

func unquote(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return strings.Trim(string(raw), `"`)
}

// readTags saca de las etiquetas lo poco que sirve para una ficha. Los
// nombres cambian entre Matroska, MP4 e ID3, así que se prueban varios.
func readTags(raw map[string]string) Tags {
	t := Tags{Raw: raw}
	get := func(keys ...string) string {
		for _, k := range keys {
			if v := strings.TrimSpace(raw[k]); v != "" {
				return v
			}
		}
		return ""
	}
	t.Title = get("title", "nam", "©nam")
	t.Show = get("show", "tvshow", "album_artist", "series")
	t.EpisodeName = get("episode_id", "episode")
	t.Season = atoi(get("season_number", "season", "tvsn"))
	t.Episode = atoi(get("episode_sort", "episode_number", "tves"))
	t.Date = get("date", "creation_time", "year", "originalyear")
	t.Year = yearOf(t.Date)
	t.Synopsis = sinopsisLegible(get("synopsis", "description", "comment", "ldes", "desc"))
	t.Artist = get("artist", "album_artist", "©art")
	t.Album = get("album", "©alb")
	t.Genre = get("genre", "©gen")
	t.MBID = get("musicbrainz_albumid", "musicbrainz album id", "musicbrainz_releasegroupid")
	return t
}

func itoa(n int) string { return strconv.Itoa(n) }

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

// yearOf saca el año de una fecha en cualquiera de las formas que se ven en
// los archivos: "2019", "2019-04-01", "2019-04-01T10:00:00.000000Z".
func yearOf(s string) int {
	s = strings.TrimSpace(s)
	if len(s) < 4 {
		return 0
	}
	n, err := strconv.Atoi(s[:4])
	if err != nil || n < 1870 || n > 2200 {
		return 0
	}
	return n
}

func trimName(path string) string {
	if i := strings.LastIndexAny(path, `/\`); i >= 0 {
		return path[i+1:]
	}
	return path
}

// sinopsisLegible tira la «sinopsis» que en realidad es la firma del
// programa que hizo el archivo («ELiTE-Fri-22-May-2026,03:44:23,1080p,21,
// fast,Y,10041788,1920,960,2»): sin espacios, o casi todo números y comas.
// Una sinopsis de verdad son frases. Lo que no lo parece no llega a la guía.
func sinopsisLegible(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	palabras := strings.Fields(s)
	if len(palabras) < 3 {
		return ""
	}
	letras, otros := 0, 0
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsSpace(r):
			letras++
		default:
			otros++
		}
	}
	if otros*2 > letras {
		return ""
	}
	return s
}

// ProbeRemoto mira una señal que está en la red en vez de en el disco: un
// HLS, un RTMP, un SRT. Es lo que sostiene el botón de «probar antes de
// guardar» (F2-116).
//
// Se diferencia de Probe en dos cosas, y las dos importan:
//
//   - **El plazo lo pone quien llama**, por el contexto, y es corto. Un
//     archivo roto merece los sesenta segundos de ProbeTimeout; una señal que
//     no contesta en cinco no sirve para salir al aire, y quien está probando
//     no puede quedarse mirando una rueda girar.
//   - **Se le dice a ffprobe que no se quede esperando el final**, porque un
//     vivo no tiene final: con -analyzeduration corto contesta con lo primero
//     que ve y se va.
func ProbeRemoto(ctx context.Context, ffprobe, url string) (Measure, error) {
	plazo := ProbeTimeout
	if fin, hay := ctx.Deadline(); hay {
		if queda := time.Until(fin); queda > 0 {
			plazo = queda
		}
	}
	return probeDe(ctx, ffprobe, url, []string{
		"-analyzeduration", "3000000", // 3 s mirando, no los 5 que trae de serie
		"-probesize", "5000000",
		"-rw_timeout", "4000000", // 4 s sin un byte y se rinde, en vez de colgarse
	}, plazo, true)
}
