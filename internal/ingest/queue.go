package ingest

import (
	"container/heap"
	"context"
	"sync"
	"time"

	"antena787/internal/model"
)

// Los estados de estado_normalizacion. El resolver solo programa lo que está
// en NormalizeReady (AUDITORIA B7).
const (
	NormalizePending = "pendiente"
	NormalizeRunning = "en_curso"
	NormalizeReady   = "listo"
	NormalizeFailed  = "fallido"
)

// NormalizeAttempts es cuántas veces se intenta normalizar antes de dar el
// archivo por malo. Dos: una puede fallar por cualquier cosa; dos ya es el
// archivo (PRD §9 paso 1.8).
const NormalizeAttempts = 2

// Persist es lo mínimo que el ingest necesita de la base. Lo implementa
// internal/store; aquí solo se declara para no depender de él.
type Persist interface {
	// SaveAsset guarda o actualiza el media_asset y le pone el ID si es nuevo.
	SaveAsset(ctx context.Context, a *model.MediaAsset) error
	// SetNormalizeState mueve estado_normalizacion y, si terminó bien, apunta
	// la ruta normalizada; si terminó mal, el motivo claro.
	SetNormalizeState(ctx context.Context, assetID int64, state, normalizedPath, plainReason string) error
	// SetAudioTracks guarda las pistas de sonido que trae el archivo —índice,
	// idioma, canales y título— y cuál de ellas sale al aire. pistaAire es el
	// índice dentro de la lista, no el número de stream del contenedor
	// (F1-60). Una lista vacía es un archivo cuyo sonido vino de al lado.
	SetAudioTracks(ctx context.Context, assetID int64, pistas []AudioTrack, pistaAire int) error
	// SetSidecars guarda de dónde salieron el sonido y los subtítulos que
	// venían al lado del video (F1-58, F1-62). Cadena vacía = no había.
	SetSidecars(ctx context.Context, assetID int64, audio, subtitulos string) error
}

// PersistLoudness es Persist más la parte del volumen: lo implementa quien,
// además de mover el estado, sepa guardar cómo quedó medido el archivo. Es
// una interfaz aparte a propósito —la cola funciona igual con una
// persistencia que no la implemente— para que el LoudnessReport que devuelve
// Normalize no se muera en memoria: F1-03 pide que quede constancia de que
// las dos pasadas se hicieron de verdad.
type PersistLoudness interface {
	Persist
	// SetLoudness anota cómo quedó el archivo normalizado: lufs y truePeak
	// son los del resultado ya medido (LoudnessReport.OutputLUFS y
	// OutputTruePeak), pasadas es LoudnessReport.Passes —2 cuando se midió y
	// se corrigió, 0 cuando el archivo no traía sonido— y nota es el texto
	// en palabras claras que acompaña al registro (hoy
	// LoudnessReport.CaptionsNote), vacío si no hay nada que contar.
	SetLoudness(ctx context.Context, assetID int64, lufs, truePeak float64, pasadas int, nota string) error
}

// SaveLoudness guarda el reporte de volumen si la persistencia sabe hacerlo.
// Devuelve false —sin error— cuando no sabe: el archivo se normalizó igual y
// no se pierde nada más que el registro. Lo llama quien tiene el reporte en
// la mano, que es quien normaliza.
func SaveLoudness(ctx context.Context, p Persist, assetID int64, rep LoudnessReport) (bool, error) {
	pl, ok := p.(PersistLoudness)
	if !ok {
		return false, nil
	}
	return true, pl.SetLoudness(ctx, assetID, rep.OutputLUFS, rep.OutputTruePeak, rep.Passes, rep.CaptionsNote)
}

// Job es un archivo esperando a que lo normalicen.
type Job struct {
	AssetID  int64
	AirsAt   time.Time // cuándo sale al aire; el cero significa "no hay fecha"
	Attempts int

	seq   int64 // orden de llegada, para desempatar
	index int
}

// NormalizeFunc es el trabajo de verdad: normaliza y devuelve dónde quedó la
// copia. Se le pasa a Run para que la cola no dependa de ffmpeg.
type NormalizeFunc func(ctx context.Context, j Job) (normalizedPath string, err error)

// Queue es la cola de normalización, priorizada por hora de aire: lo que
// sale antes se normaliza antes (AUDITORIA B7). Lo que no tiene hora de aire
// va al final, después de todo lo que sí la tiene.
//
// Es una cola en memoria a propósito: al arrancar, el store vuelve a encolar
// todo lo que quedó en pendiente o en_curso.
type Queue struct {
	// RetryDelay es cuánto se espera antes de reintentar un fallo. Cinco
	// minutos, como el reintento de lectura (AUDITORIA C8).
	RetryDelay time.Duration
	// MaxAttempts es cuántos intentos antes de darlo por fallido.
	MaxAttempts int
	// Permiso, si está puesto, se pregunta antes de empezar cada archivo.
	// Mientras conteste que no, la cola espera con la fila intacta y sin
	// gastar nada. Es por donde el aire manda sobre la preparación: preparar
	// la biblioteca nunca puede costarle la señal a nadie (ADR 0008).
	// `porque` se dice tal cual en la bitácora, así que se escribe para que
	// lo lea una persona.
	Permiso func() (puede bool, porque string)
	// Aviso deja constancia de que la cola se paró o siguió. Puede ser nil.
	Aviso func(texto string)

	mu     sync.Mutex
	h      jobHeap
	seq    int64
	wake   chan struct{}
	closed bool
}

// NewQueue crea la cola con los defaults del PRD.
func NewQueue() *Queue {
	return &Queue{RetryDelay: 5 * time.Minute, MaxAttempts: NormalizeAttempts, wake: make(chan struct{}, 1)}
}

// Enqueue mete un archivo en la cola con la hora a la que sale al aire.
func (q *Queue) Enqueue(assetID int64, airsAt time.Time) {
	q.push(Job{AssetID: assetID, AirsAt: airsAt})
}

func (q *Queue) push(j Job) {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return
	}
	q.seq++
	j.seq = q.seq
	heap.Push(&q.h, &j)
	q.mu.Unlock()
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

// Len dice cuántos hay esperando.
func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.h.Len()
}

// Pop saca el siguiente sin esperar. El segundo valor dice si había alguno.
func (q *Queue) Pop() (Job, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.h.Len() == 0 {
		return Job{}, false
	}
	j := heap.Pop(&q.h).(*Job)
	return *j, true
}

// Next espera hasta que haya trabajo o hasta que se cancele el contexto.
// PermisoPoll es cada cuánto se vuelve a preguntar si el aire ya aguanta.
// Cinco segundos: lo bastante corto para no perder tiempo de preparación, lo
// bastante largo para no preguntar mil veces por minuto.
const PermisoPoll = 5 * time.Second

func (q *Queue) Next(ctx context.Context) (Job, bool) {
	for {
		if j, ok := q.Pop(); ok {
			return j, true
		}
		select {
		case <-ctx.Done():
			return Job{}, false
		case <-q.wake:
		case <-time.After(250 * time.Millisecond):
		}
	}
}

// Run es el trabajador: toma de la cola, normaliza, y anota en la base cómo
// fue. Un fallo se reintenta pasado RetryDelay; dos fallos dejan el archivo
// en fallido con su motivo claro. Corre hasta que se cancele el
// contexto.
func (q *Queue) Run(ctx context.Context, p Persist, work NormalizeFunc) error {
	for {
		// El aire manda (ADR 0008). Antes de empezar un archivo se pregunta
		// si se puede; mientras la respuesta sea que no, la cola espera con
		// la fila intacta. Se pregunta ANTES de sacar de la fila, para que lo
		// que entre mientras tanto se ordene igual por su hora de aire: si se
		// guardara el trabajo ya sacado, al reanudar saldría uno viejo
		// delante de otro que corre más prisa.
		if !q.esperarPermiso(ctx) {
			return ctx.Err()
		}
		j, ok := q.Next(ctx)
		if !ok {
			return ctx.Err()
		}
		if p != nil {
			_ = p.SetNormalizeState(ctx, j.AssetID, NormalizeRunning, "", "")
		}
		path, err := work(ctx, j)
		if err == nil {
			if p != nil {
				if serr := p.SetNormalizeState(ctx, j.AssetID, NormalizeReady, path, ""); serr != nil {
					return serr
				}
			}
			continue
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		j.Attempts++
		max := q.MaxAttempts
		if max <= 0 {
			max = NormalizeAttempts
		}
		if j.Attempts >= max {
			if p != nil {
				_ = p.SetNormalizeState(ctx, j.AssetID, NormalizeFailed, "",
					"no se pudo dejar el archivo en el formato de casa después de "+itoa(j.Attempts)+" intentos: "+Plain(err))
			}
			continue
		}
		if p != nil {
			_ = p.SetNormalizeState(ctx, j.AssetID, NormalizePending, "", "")
		}
		q.requeueLater(j)
	}
}

// requeueLater vuelve a poner el trabajo pasado el retardo, sin bloquear al
// trabajador: mientras tanto se sigue normalizando lo demás.
func (q *Queue) requeueLater(j Job) {
	if q.RetryDelay <= 0 {
		q.push(j)
		return
	}
	time.AfterFunc(q.RetryDelay, func() { q.push(j) })
}

// ── el montículo ──────────────────────────────────────────────────────

type jobHeap []*Job

func (h jobHeap) Len() int { return len(h) }

// Less es la regla de prioridad: primero lo que sale antes al aire; lo que
// no tiene hora de aire, al final; y a igualdad, el que llegó primero.
func (h jobHeap) Less(i, j int) bool {
	a, b := h[i], h[j]
	az, bz := a.AirsAt.IsZero(), b.AirsAt.IsZero()
	if az != bz {
		return bz
	}
	if !az && !a.AirsAt.Equal(b.AirsAt) {
		return a.AirsAt.Before(b.AirsAt)
	}
	return a.seq < b.seq
}

func (h jobHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index, h[j].index = i, j
}

func (h *jobHeap) Push(x any) {
	j := x.(*Job)
	j.index = len(*h)
	*h = append(*h, j)
}

func (h *jobHeap) Pop() any {
	old := *h
	n := len(old)
	j := old[n-1]
	old[n-1] = nil
	*h = old[:n-1]
	return j
}


// esperarPermiso bloquea mientras Permiso conteste que no. Devuelve false solo
// si se canceló el contexto. Sin Permiso puesto no espera nunca, que es como
// se comportaba la cola antes de que esto existiera.
func (q *Queue) esperarPermiso(ctx context.Context) bool {
	if q.Permiso == nil {
		return true
	}
	dicho := ""
	for {
		puede, porque := q.Permiso()
		if puede {
			if dicho != "" && q.Aviso != nil {
				q.Aviso("la preparación de archivos sigue: el aire se puso al día")
			}
			return true
		}
		// Se dice una vez por motivo, no cada vuelta: esto se mira cada pocos
		// segundos y la bitácora es para leerla.
		if porque != dicho && q.Aviso != nil {
			q.Aviso("la preparación de archivos se detiene: " + porque)
		}
		dicho = porque
		select {
		case <-ctx.Done():
			return false
		case <-time.After(PermisoPoll):
		}
	}
}
