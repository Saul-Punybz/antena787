package ingest

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Audio de todo el material (decisión del 9 de septiembre de 2026, F1-58 a
// F1-63): todo lo que sale al aire lleva sonido. Aquí viven las tres piezas
// de esa decisión que no son ni medir ni convertir: el archivo de sonido que
// viene al lado del video, la elección de pista cuando el archivo trae
// varias, y el idioma preferido del canal.

// AudioSidecarExtensions son los archivos de sonido que se aceptan al lado
// de un video mudo, en orden de preferencia: el .wav es el que menos pierde,
// y los comprimidos van después (F1-58).
var AudioSidecarExtensions = []string{".wav", ".m4a", ".aac", ".mp3", ".flac"}

// IdiomaAudioPorDefecto es el idioma en el que se quiere el aire cuando el
// canal no dice otra cosa. Esto es Puerto Rico: se emite en español.
const IdiomaAudioPorDefecto = "es"

// Preferencias son las decisiones que el ingest no puede adivinar solo: en
// qué idioma quiere el canal el aire y qué pista eligió una persona a mano.
// Se pasan a NormalizeOptionsFor —que las acepta como parámetro opcional
// para no obligar a tocar a quien no las use— y viven en Deps para el
// ingest (F1-60, F1-61).
type Preferencias struct {
	// IdiomaAudio es el idioma_audio_preferido del canal. Vacío = "es".
	IdiomaAudio string

	// PistaAudio es la pista que eligió el operador desde Biblioteca. nil =
	// que decida el ingest. Si el número no existe en el archivo se ignora y
	// se vuelve a decidir solo: una elección vieja no deja al archivo mudo.
	PistaAudio *int

	// AudioSidecar y SubtitulosSidecar son los archivos de al lado que ya se
	// guardaron en el media_asset. Vacíos, se vuelven a buscar en la carpeta.
	AudioSidecar      string
	SubtitulosSidecar string
}

// idioma devuelve el idioma preferido, ya normalizado.
func (p Preferencias) idioma() string {
	if s := NormalizeLanguage(p.IdiomaAudio); s != "" {
		return s
	}
	return IdiomaAudioPorDefecto
}

// PistaDeAire elige qué pista de sonido sale al aire (F1-60): manda lo que
// eligió una persona, si esa pista existe; si no, la primera del idioma
// preferido; y si el archivo no trae ninguna en ese idioma, la primera del
// archivo.
func PistaDeAire(pistas []AudioTrack, prefs Preferencias) int {
	if prefs.PistaAudio != nil {
		n := *prefs.PistaAudio
		if n >= 0 && (len(pistas) == 0 || n < len(pistas)) {
			return n
		}
	}
	idioma := prefs.idioma()
	for _, p := range pistas {
		if p.Language == idioma {
			return p.Index
		}
	}
	return 0
}

// FindAudioSidecar busca el archivo de sonido que acompaña a un video mudo:
// la misma carpeta, el mismo nombre y otra extensión (F1-58). La extensión
// se compara sin distinguir mayúsculas, que es como llegan de Windows.
func FindAudioSidecar(path string) (string, bool) {
	return findByStem(path, AudioSidecarExtensions)
}

// FindSubtitleSidecar busca el archivo de subtítulos que acompaña al video.
// Es lo mismo que FindSidecar y está aquí solo para que las dos búsquedas se
// lean iguales desde el ingest.
func FindSubtitleSidecar(path string) (string, bool) { return FindSidecar(path) }

// SubtituloMuxeable dice si un archivo de subtítulos de al lado se puede
// meter en la copia de casa como pista de texto: los .srt y los .vtt sí. Los
// .scc y los .mcc no —son CEA-608/708, que van dentro de la imagen— y se
// guardan tal cual para que F2 los reinserte (F1-62, F1-75).
func SubtituloMuxeable(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".srt", ".vtt":
		return true
	}
	return false
}

// findByStem busca en la carpeta del archivo otro que se llame igual y tenga
// una de las extensiones pedidas, en el orden en que vienen.
func findByStem(path string, exts []string) (string, bool) {
	dir := filepath.Dir(path)
	stem := strings.ToLower(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	best, bestRank := "", len(exts)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		rank := -1
		for i, want := range exts {
			if ext == want {
				rank = i
			}
		}
		if rank < 0 || rank >= bestRank {
			continue
		}
		if strings.ToLower(strings.TrimSuffix(name, filepath.Ext(name))) != stem {
			continue
		}
		best, bestRank = filepath.Join(dir, name), rank
	}
	return best, best != ""
}

// EsSidecarDeAudio y EsSidecarDeSubtitulos dicen si una extensión es de las
// que acompañan a un video en vez de ser material por su cuenta.
func EsSidecarDeAudio(path string) bool      { return tieneExt(path, AudioSidecarExtensions) }
func EsSidecarDeSubtitulos(path string) bool { return tieneExt(path, SidecarExtensions) }
func esVideo(path string) bool               { return tieneExt(path, VideoExtensions) }

func tieneExt(path string, exts []string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, e := range exts {
		if ext == e {
			return true
		}
	}
	return false
}

// VideoForSidecar busca a qué video acompaña un archivo de sonido o de
// subtítulos: mismo nombre en la misma carpeta. Devuelve la ruta del video y
// si lo encontró. Un .wav que no acompaña a nadie es material por su cuenta
// —música— y este no es su camino.
func VideoForSidecar(path string) (string, bool) {
	dir := filepath.Dir(path)
	stem := strings.ToLower(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !esVideo(name) {
			continue
		}
		vstem := strings.ToLower(strings.TrimSuffix(name, filepath.Ext(name)))
		// El .srt admite el idioma en medio: "Kojak S01E03.es.srt".
		if vstem == stem || strings.HasPrefix(stem, vstem+".") {
			return filepath.Join(dir, name), true
		}
	}
	return "", false
}

// StableSidecar dice si un archivo de al lado terminó de copiarse, con la
// misma regla que la carpeta vigilada (F1-01): lleva quieto el tiempo que se
// pide y se puede abrir para leer de punta a punta. Aquí se mira la fecha de
// modificación en vez de sondear el tamaño, porque el ingest no está para
// quedarse esperando: si el archivo todavía se está copiando, el aviso de la
// carpeta vigilada volverá a pasar por aquí cuando termine.
//
// stableFor 0 usa WatchStableFor. now es el reloj, para las pruebas.
func StableSidecar(path string, stableFor time.Duration, now time.Time) bool {
	if stableFor == 0 {
		stableFor = WatchStableFor
	}
	st, err := os.Stat(path)
	if err != nil || st.IsDir() || st.Size() == 0 {
		return false
	}
	if now.IsZero() {
		now = time.Now()
	}
	if now.Sub(st.ModTime()) < stableFor {
		return false
	}
	return openableForRead(path)
}
