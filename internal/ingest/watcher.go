package ingest

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Defaults de la carpeta vigilada (AUDITORIA C12, criterio F1-01). Los dos
// son configurables: aquí solo viven los valores de fábrica.
const (
	// WatchInterval es cada cuánto se mira la carpeta. Se sondea a propósito:
	// las notificaciones del sistema operativo no se comportan igual en
	// Windows, en Linux y sobre un disco de red, y esto no necesita ser
	// rápido — necesita no equivocarse.
	WatchInterval = 2 * time.Second

	// WatchStableFor es cuánto tiene que quedarse quieto el tamaño para dar
	// la copia por terminada.
	WatchStableFor = 10 * time.Second
)

// WatchedExtensions es todo lo que la carpeta vigilada levanta: el material
// de siempre más los archivos de subtítulos, que no son material pero avisan
// de que un video puede volver a procesarse (F1-62).
func WatchedExtensions() []string {
	out := MediaExtensions()
	return append(out, SidecarExtensions...)
}

// TempSuffixes son los archivos a medio hacer que hay que dejar en paz.
var TempSuffixes = []string{".part", ".partial", ".crdownload", ".tmp", ".temp", ".filepart", "~", ".!ut", ".downloading"}

// Los dos tipos de aviso de la carpeta vigilada. EventMedia es lo de
// siempre: un archivo de material que se ingiere por su cuenta. EventSidecar
// es un archivo que acompaña a un video —el sonido de un video mudo, o unos
// subtítulos— y que NUNCA se ingiere solo: lo que hay que volver a procesar
// es el video, y para eso está Found.For (F1-58, F1-62).
const (
	EventMedia   = "material"
	EventSidecar = "al_lado"
)

// Found es un archivo que terminó de copiarse y está listo para el ingest.
//
// Kind dice de qué aviso se trata. En EventSidecar, For lleva la ruta del
// video al que acompaña: quien recibe el aviso vuelve a correr el ingest de
// ESE video (ingest.ReingestSidecar hace justo eso), y así un video que
// quedó en cuarentena por mudo sale de cuarentena en cuanto llega su audio.
// El archivo de al lado no se guarda como material aparte.
type Found struct {
	Path      string
	SizeBytes int64
	At        time.Time
	Kind      string // EventMedia o EventSidecar
	For       string // solo en EventSidecar: el video al que acompaña
}

// WatcherOptions configura la carpeta vigilada.
type WatcherOptions struct {
	Dir        string        // carpeta a vigilar
	Interval   time.Duration // 0 = WatchInterval
	StableFor  time.Duration // 0 = WatchStableFor
	Extensions []string      // nil = WatchedExtensions()
	Recursive  bool          // mirar también las subcarpetas
	Now        func() time.Time
}

// Watcher vigila una carpeta y avisa de cada archivo que termina de
// copiarse. "Terminó de copiarse" quiere decir dos cosas a la vez: el tamaño
// no cambió en diez segundos y el archivo se puede abrir para leer. Sin eso,
// en Windows la carpeta vigilada lee archivos a medias y los manda a
// cuarentena por error (PRD §9 paso 1.0, F1-01).
type Watcher struct {
	opts   WatcherOptions
	events chan Found
	seen   map[string]*watchEntry
}

type watchEntry struct {
	size    int64
	since   time.Time // desde cuándo el tamaño es el mismo
	emitted bool
}

// NewWatcher prepara la vigilancia. No mira nada hasta que se llama a Run.
func NewWatcher(opts WatcherOptions) (*Watcher, error) {
	if strings.TrimSpace(opts.Dir) == "" {
		return nil, Plainf(nil, "hay que decir qué carpeta se vigila")
	}
	if err := os.MkdirAll(opts.Dir, 0o755); err != nil {
		return nil, Plainf(err, "no se puede usar la carpeta vigilada %q", opts.Dir)
	}
	if opts.Interval <= 0 {
		opts.Interval = WatchInterval
	}
	if opts.StableFor <= 0 {
		opts.StableFor = WatchStableFor
	}
	if len(opts.Extensions) == 0 {
		opts.Extensions = WatchedExtensions()
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return &Watcher{opts: opts, events: make(chan Found, 32), seen: map[string]*watchEntry{}}, nil
}

// Events es por donde salen los archivos listos. Se cierra cuando Run
// termina.
func (w *Watcher) Events() <-chan Found { return w.events }

// Run vigila hasta que se cancele el contexto. Devuelve el error del
// contexto, que es la forma normal de terminar.
func (w *Watcher) Run(ctx context.Context) error {
	defer close(w.events)
	t := time.NewTicker(w.opts.Interval)
	defer t.Stop()
	for {
		if err := w.Poll(ctx); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}
}

// Poll hace una pasada. Está exportada para poder probar la vigilancia sin
// esperar relojes de verdad.
func (w *Watcher) Poll(ctx context.Context) error {
	now := w.opts.Now()
	present := map[string]bool{}

	err := w.walk(func(path string, info os.FileInfo) {
		if !w.interesting(path) {
			return
		}
		present[path] = true
		e, ok := w.seen[path]
		if !ok {
			w.seen[path] = &watchEntry{size: info.Size(), since: now}
			return
		}
		if e.emitted {
			return
		}
		if e.size != info.Size() {
			e.size, e.since = info.Size(), now
			return
		}
		if now.Sub(e.since) < w.opts.StableFor {
			return
		}
		if !openableForRead(path) {
			// Alguien todavía lo tiene agarrado: se vuelve a contar el
			// tiempo de quietud desde ahora.
			e.since = now
			return
		}
		kind, para := classify(path)
		if kind == "" {
			// Unos subtítulos que todavía no acompañan a nada: no son
			// material y no se anuncian. Si más tarde llega el video, la
			// siguiente pasada sí los anuncia.
			return
		}
		e.emitted = true
		select {
		case w.events <- Found{Path: path, SizeBytes: info.Size(), At: now, Kind: kind, For: para}:
		case <-ctx.Done():
		}
	})
	if err != nil {
		return err
	}
	for path := range w.seen {
		if !present[path] {
			delete(w.seen, path)
		}
	}
	return ctx.Err()
}

func (w *Watcher) walk(fn func(string, os.FileInfo)) error {
	if !w.opts.Recursive {
		entries, err := os.ReadDir(w.opts.Dir)
		if err != nil {
			return Plainf(err, "no se puede leer la carpeta vigilada %q", w.opts.Dir)
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			fn(filepath.Join(w.opts.Dir, e.Name()), info)
		}
		return nil
	}
	return filepath.Walk(w.opts.Dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil //nolint:nilerr // una carpeta ilegible no tumba la vigilancia
		}
		fn(path, info)
		return nil
	})
}

// classify decide qué es un archivo que ya terminó de copiarse: material que
// entra por su cuenta, o un archivo que acompaña a un video. Un .wav que no
// acompaña a ningún video es música y entra como material, que es lo que
// siempre ha hecho; el mismo .wav al lado de "spot.mp4" es el sonido de ese
// spot y no se guarda aparte (F1-58).
func classify(path string) (kind, para string) {
	if EsSidecarDeSubtitulos(path) {
		if v, ok := VideoForSidecar(path); ok {
			return EventSidecar, v
		}
		return "", ""
	}
	if EsSidecarDeAudio(path) {
		if v, ok := VideoForSidecar(path); ok {
			return EventSidecar, v
		}
	}
	return EventMedia, ""
}

// interesting filtra lo que no es material: temporales, ocultos y todo lo
// que no tenga una extensión de medios conocida.
func (w *Watcher) interesting(path string) bool {
	name := filepath.Base(path)
	if name == "" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "~$") {
		return false
	}
	lower := strings.ToLower(name)
	for _, s := range TempSuffixes {
		if strings.HasSuffix(lower, s) {
			return false
		}
	}
	ext := strings.ToLower(filepath.Ext(name))
	for _, e := range w.opts.Extensions {
		if ext == strings.ToLower(e) {
			return true
		}
	}
	return false
}

// openableForRead comprueba que el archivo se puede abrir y leer de verdad,
// principio y final. En Windows, un programa que está copiando y no comparte
// la lectura hace fallar esto, que es justo la señal que se busca; en Unix
// el que manda es el tamaño estable, y esto atrapa además los permisos que
// aún no se han propagado y el disco de red que se fue.
func openableForRead(path string) bool {
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return false
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return false
	}
	if st.Size() == 0 {
		return true // vacío se deja pasar: el ingest lo manda a cuarentena y lo explica
	}
	buf := make([]byte, 1)
	if _, err := f.ReadAt(buf, 0); err != nil && err != io.EOF {
		return false
	}
	if _, err := f.ReadAt(buf, st.Size()-1); err != nil && err != io.EOF {
		return false
	}
	return true
}
