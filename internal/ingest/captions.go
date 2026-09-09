package ingest

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"antena787/internal/model"
)

// SidecarExtensions son los archivos de subtítulos que se aceptan al lado
// del video, en orden de preferencia: .scc ya es CEA-608 y entra tal cual;
// .srt y .vtt hay que convertirlos, y eso es F2 (PRD §9 paso 1.3).
var SidecarExtensions = []string{".scc", ".srt", ".vtt"}

// Cue es una línea de subtítulo con su ventana de tiempo.
type Cue struct {
	StartMs int64
	EndMs   int64
	Text    string
}

// FindSidecar busca un archivo de subtítulos al lado del video: el mismo
// nombre con otra extensión, y también la forma con idioma en medio
// ("Kojak S01E03.es.srt"). Devuelve la ruta y si encontró algo.
func FindSidecar(path string) (string, bool) {
	dir := filepath.Dir(path)
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	lowerBase := strings.ToLower(base)
	best, bestRank := "", len(SidecarExtensions)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		rank := -1
		for i, want := range SidecarExtensions {
			if ext == want {
				rank = i
			}
		}
		if rank < 0 || rank >= bestRank {
			continue
		}
		stem := strings.ToLower(strings.TrimSuffix(name, filepath.Ext(name)))
		if stem != lowerBase && !strings.HasPrefix(stem, lowerBase+".") {
			continue
		}
		best, bestRank = filepath.Join(dir, name), rank
	}
	return best, best != ""
}

// AttachCaptions valida el archivo de subtítulos y lo anota en el asset. En
// F1 no se convierte nada: se guarda la ruta y se comprueba que el archivo
// se entiende, para que nadie descubra en el aire que el .srt estaba roto.
func AttachCaptions(asset *model.MediaAsset, sidecar string) error {
	if asset == nil {
		return Plainf(nil, "no hay ficha de archivo donde anotar los subtítulos")
	}
	format, err := ValidateSidecar(sidecar)
	if err != nil {
		return err
	}
	p := sidecar
	asset.ExternalCaptions = &p
	asset.HasCaptions = true
	if asset.CaptionFormat == "" {
		asset.CaptionFormat = format
	}
	return nil
}

// ValidateSidecar comprueba que el archivo de subtítulos se puede leer y
// devuelve su formato: "scc", "srt" o "vtt".
func ValidateSidecar(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", Plainf(err, "no se puede abrir el archivo de subtítulos %q", trimName(path))
	}
	defer f.Close()

	switch strings.ToLower(filepath.Ext(path)) {
	case ".scc":
		if err := CheckSCC(f); err != nil {
			return "", err
		}
		return "scc", nil
	case ".srt":
		cues, err := ParseSRT(f)
		if err != nil {
			return "", err
		}
		if len(cues) == 0 {
			return "", Plainf(nil, "el archivo de subtítulos %q no tiene ni una línea", trimName(path))
		}
		return "srt", nil
	case ".vtt":
		cues, err := ParseVTT(f)
		if err != nil {
			return "", err
		}
		if len(cues) == 0 {
			return "", Plainf(nil, "el archivo de subtítulos %q no tiene ni una línea", trimName(path))
		}
		return "vtt", nil
	default:
		return "", Plainf(nil, "%q no es un archivo de subtítulos que sepamos leer (.scc, .srt o .vtt)", trimName(path))
	}
}

// CheckSCC comprueba la cabecera de un archivo Scenarist, que es lo que
// distingue un .scc de verdad de un archivo de texto cualquiera. El
// contenido son pares de bytes CEA-608 en hexadecimal y no se toca en F1.
func CheckSCC(r io.Reader) error {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(stripBOM(sc.Text()))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "Scenarist_SCC V1.0") {
			return nil
		}
		return Plainf(nil, "este .scc no empieza por «Scenarist_SCC V1.0»: no es un archivo de subtítulos 608")
	}
	if err := sc.Err(); err != nil {
		return Plainf(err, "no se pudo leer el archivo de subtítulos")
	}
	return Plainf(nil, "el archivo de subtítulos está vacío")
}

// ParseSRT lee un SubRip. Es un parser mínimo y a propósito: acepta lo que
// se ve en la calle —numeración opcional, comas o puntos en los decimales,
// finales de línea de Windows— y rechaza lo que no tiene tiempos.
func ParseSRT(r io.Reader) ([]Cue, error) { return parseCues(r, false) }

// ParseVTT lee un WebVTT. Exige la cabecera WEBVTT y por lo demás se parece
// bastante al SRT.
func ParseVTT(r io.Reader) ([]Cue, error) { return parseCues(r, true) }

func parseCues(r io.Reader, webvtt bool) ([]Cue, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	var (
		out     []Cue
		cur     *Cue
		line    string
		n       int
		sawHead bool
	)
	for sc.Scan() {
		n++
		line = strings.TrimRight(stripBOM(sc.Text()), "\r")
		trimmed := strings.TrimSpace(line)

		if webvtt && !sawHead {
			if trimmed == "" {
				continue
			}
			if !strings.HasPrefix(trimmed, "WEBVTT") {
				return nil, Plainf(nil, "este archivo no empieza por «WEBVTT»: no es un archivo de subtítulos web")
			}
			sawHead = true
			continue
		}
		if strings.Contains(trimmed, "-->") {
			start, end, err := parseTimeRange(trimmed)
			if err != nil {
				return nil, Plainf(err, "los tiempos de la línea %d del archivo de subtítulos no se entienden", n)
			}
			out = append(out, Cue{StartMs: start, EndMs: end})
			cur = &out[len(out)-1]
			continue
		}
		if trimmed == "" {
			cur = nil
			continue
		}
		if cur == nil {
			// Número de bloque del SRT, identificador de cue del VTT, o una
			// línea de NOTE: no estorban.
			continue
		}
		if cur.Text != "" {
			cur.Text += "\n"
		}
		cur.Text += trimmed
	}
	if err := sc.Err(); err != nil {
		return nil, Plainf(err, "no se pudo leer el archivo de subtítulos")
	}
	if len(out) == 0 {
		return nil, Plainf(nil, "el archivo de subtítulos no tiene ni un tiempo: puede que no sea un archivo de subtítulos")
	}
	return out, nil
}

// parseTimeRange lee "00:00:01,000 --> 00:00:04,000" y sus variantes.
func parseTimeRange(line string) (int64, int64, error) {
	left, right, ok := strings.Cut(line, "-->")
	if !ok {
		return 0, 0, Plainf(nil, "falta la flecha entre los dos tiempos")
	}
	start, err := parseTimestamp(left)
	if err != nil {
		return 0, 0, err
	}
	// El VTT permite ajustes de posición después del segundo tiempo.
	rightFields := strings.Fields(strings.TrimSpace(right))
	if len(rightFields) == 0 {
		return 0, 0, Plainf(nil, "falta el tiempo final")
	}
	end, err := parseTimestamp(rightFields[0])
	if err != nil {
		return 0, 0, err
	}
	if end < start {
		return 0, 0, Plainf(nil, "el subtítulo termina antes de empezar")
	}
	return start, end, nil
}

// parseTimestamp lee HH:MM:SS,mmm · HH:MM:SS.mmm · MM:SS.mmm.
func parseTimestamp(s string) (int64, error) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", "."))
	if s == "" {
		return 0, Plainf(nil, "falta un tiempo")
	}
	parts := strings.Split(s, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, Plainf(nil, "el tiempo %q no tiene la forma HH:MM:SS,mmm", s)
	}
	var total float64
	for _, p := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil || v < 0 {
			return 0, Plainf(nil, "el tiempo %q no tiene la forma HH:MM:SS,mmm", s)
		}
		total = total*60 + v
	}
	return int64(total*1000 + 0.5), nil
}

func stripBOM(s string) string { return strings.TrimPrefix(s, "\ufeff") }
