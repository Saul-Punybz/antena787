package ingest

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"antena787/internal/model"
)

// Defaults del portal del anunciante (PRD §9 paso 9 y §19). La carpeta
// portal/entrada/ no hereda la exclusión de antivirus: lo que sube alguien de
// fuera se examina ahí y solo después se mueve a la biblioteca
// (AUDITORIA A3).
const (
	// UploadDir es el nombre de la carpeta, relativo a la raíz de datos.
	UploadDir = "portal/entrada"

	// UploadMaxBytes es lo máximo que puede pesar un anuncio: 500 MB.
	UploadMaxBytes int64 = 500 << 20

	// UploadMaxDuration es lo máximo que puede durar: 5 minutos.
	UploadMaxDuration = 5 * time.Minute

	// UploadProbeTimeout es el límite de tiempo de la medición. Diez
	// segundos: un archivo que tarda más que eso en abrirse no va al aire.
	UploadProbeTimeout = 10 * time.Second
)

// UploadWhitelist le quita a ffprobe todo lo que no sea leer un archivo
// local. Sin esto, un "archivo" que en realidad es una lista de reproducción
// con una URL dentro hace que ffprobe salga a la red desde la máquina que
// emite.
var UploadWhitelist = []string{"-protocol_whitelist", "file"}

// ValidateUpload examina un archivo recién subido por el portal. No toca la
// red, no ejecuta nada del archivo y no tarda más de diez segundos. Los
// errores están escritos para el anunciante, que no es técnico y está en el
// teléfono.
//
// maxBytes y maxDur en cero usan los defaults del PRD (500 MB, 5 minutos).
func ValidateUpload(ctx context.Context, ffprobe, path string, maxBytes int64, maxDur time.Duration) (Measure, error) {
	if maxBytes <= 0 {
		maxBytes = UploadMaxBytes
	}
	if maxDur <= 0 {
		maxDur = UploadMaxDuration
	}
	var m Measure

	st, err := os.Stat(path)
	if err != nil {
		return m, Plainf(err, "no encontramos el archivo que subiste: vuelve a intentarlo")
	}
	if st.IsDir() {
		return m, Plainf(nil, "eso es una carpeta, no un archivo: sube el video suelto")
	}
	if st.Size() == 0 {
		return m, Plainf(nil, "el archivo llegó vacío: la subida se cortó, inténtalo otra vez")
	}
	if st.Size() > maxBytes {
		return m, Plainf(nil, "el archivo pesa %s y el máximo son %s: súbelo más liviano o más corto",
			humanBytes(st.Size()), humanBytes(maxBytes))
	}
	if !IsMedia(path) {
		return m, Plainf(nil, "no reconocemos %q como un video ni como un audio: sube un archivo .mp4 o .mov",
			trimName(path))
	}

	m, err = probeWith(ctx, ffprobe, path, UploadWhitelist, UploadProbeTimeout)
	if err != nil {
		return m, Plainf(err, "no pudimos abrir este archivo: puede estar dañado o no ser un video")
	}
	if !m.HasVideo && !m.HasAudio {
		return m, Plainf(nil, "este archivo no tiene ni imagen ni sonido: no es un video")
	}
	if !m.HasAudio {
		return m, Plainf(nil, "este archivo no tiene sonido")
	}
	if m.DurationMs <= 0 {
		return m, Plainf(nil, "este archivo dura cero: la subida se cortó o el video está dañado")
	}
	if d := time.Duration(m.DurationMs) * time.Millisecond; d > maxDur {
		return m, Plainf(nil, "esto dura %s y el máximo son %s: esto no parece un anuncio, súbelo más corto",
			humanDuration(d), humanDuration(maxDur))
	}
	return m, nil
}

// UploadTolerance es cuánto se le perdona a un anuncio contra lo que se
// compró. Medio segundo: por debajo de eso nadie lo nota en el aire.
const UploadTolerance = 500 * time.Millisecond

// CheckDuration compara lo que dura el anuncio con lo que el anunciante
// compró y lo dice como lo diría una persona (PRD §9 paso 9).
func CheckDuration(m Measure, bought time.Duration) error {
	if bought <= 0 {
		return nil
	}
	got := time.Duration(m.DurationMs) * time.Millisecond
	diff := got - bought
	if diff < 0 {
		diff = -diff
	}
	if diff <= UploadTolerance {
		return nil
	}
	if got > bought {
		return Plainf(nil, "tu anuncio dura %s y compraste %s: ¿lo cortamos o subes otro?",
			humanDuration(got), humanDuration(bought))
	}
	return Plainf(nil, "tu anuncio dura %s y compraste %s: entra igual, pero el resto del espacio queda en negro",
		humanDuration(got), humanDuration(bought))
}

// AcceptUpload es lo que pasa cuando el archivo pasó el examen: se mueve de
// portal/entrada a la biblioteca. Se hace con copia y borrado si el destino
// está en otro disco, que es lo normal cuando la biblioteca vive en un NAS.
func AcceptUpload(src, libraryDir string) (string, error) {
	if err := os.MkdirAll(libraryDir, 0o755); err != nil {
		return "", Plainf(err, "no se puede escribir en la biblioteca")
	}
	dst := filepath.Join(libraryDir, filepath.Base(src))
	for i := 1; fileHasBytes(dst); i++ {
		ext := filepath.Ext(src)
		base := strings.TrimSuffix(filepath.Base(src), ext)
		dst = filepath.Join(libraryDir, fmt.Sprintf("%s (%d)%s", base, i, ext))
	}
	if err := os.Rename(src, dst); err == nil {
		return dst, nil
	}
	in, err := os.Open(src)
	if err != nil {
		return "", Plainf(err, "no se puede leer el archivo subido")
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return "", Plainf(err, "no se puede escribir en la biblioteca")
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(dst)
		return "", Plainf(err, "la copia a la biblioteca se cortó a medias")
	}
	if err := out.Close(); err != nil {
		return "", Plainf(err, "la copia a la biblioteca no se cerró bien")
	}
	_ = os.Remove(src)
	return dst, nil
}

// UploadKind es lo que entra por el portal: siempre un spot, hasta que una
// persona lo apruebe y le diga otra cosa (PRD §9 paso 9, cola de aprobación).
const UploadKind = model.TitleSpot

// ── cómo se le dicen los números a una persona ────────────────────────

func humanBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/float64(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.0f MB", float64(n)/float64(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(n)/float64(1<<10))
	default:
		return fmt.Sprintf("%d bytes", n)
	}
}

func humanDuration(d time.Duration) string {
	total := int64(d.Round(time.Second) / time.Second)
	if total < 60 {
		if total == 1 {
			return "1 segundo"
		}
		return fmt.Sprintf("%d segundos", total)
	}
	min, sec := total/60, total%60
	unit := "minutos"
	if min == 1 {
		unit = "minuto"
	}
	if sec == 0 {
		return fmt.Sprintf("%d %s", min, unit)
	}
	return fmt.Sprintf("%d %s y %d s", min, unit, sec)
}
