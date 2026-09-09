// Package app es el cableado: abre la base, encuentra ffmpeg, y levanta las
// goroutines que hacen que el canal exista solo —el resolver, la vigilancia
// de la carpeta de contenido, la cola de normalización, el respaldo de cada
// hora, la vigilancia del disco y la del reloj—. No sabe nada de HTTP: eso
// es internal/api. Un solo proceso, un solo canal (PRD §14.1).
//
// La regla que manda aquí es la de la auditoría A2: **un pánico en cualquier
// rincón no puede tumbar el proceso**. Cada goroutine corre bajo guard, que
// atrapa el pánico, deja un incidente `panico_<nombre>` y la relanza a los
// cinco segundos.
//
// En F1 el canal está en modo **sombra**: se resuelve el plan, se publica la
// guía y se ingiere el contenido, pero no hay motor al aire. Por eso nadie
// marca ítems como emitidos y MarkAired existe pero no la llama nadie.
package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"antena787/internal/engine"
	"antena787/internal/ingest"
	"antena787/internal/model"
	"antena787/internal/resolver"
	"antena787/internal/store"
)

// Claves de settings que este paquete y la API comparten. Toda la
// configuración vive en la tabla settings: no hay archivo de configuración
// (PRD §14.1).
const (
	// KeyContentFolder es la carpeta vigilada de donde entra el contenido.
	KeyContentFolder = "carpeta_contenido"
	// KeyBackupFolder es a dónde va el respaldo de cada hora.
	KeyBackupFolder = "carpeta_respaldo"
	// KeyGuidePath es el archivo donde se escribe la guía XMLTV, si alguien
	// la lee de disco (el transmisor, MistServer). Vacío = solo /guia.xml.
	KeyGuidePath = "ruta_guia_xml"
	// KeyOperator es el nombre de quien opera, para el autor de la auditoría.
	KeyOperator = "nombre_operador"
	// KeySessionSecret firma la cookie de sesión. Es secreto: no sale en
	// GET /ajustes ni en un respaldo que salga de la máquina.
	KeySessionSecret = "sesion.secreto"
	// KeyInstallStep es el paso del asistente en el que va la instalación.
	KeyInstallStep = "instalacion.paso"
	// KeyInstallDone dice que el asistente terminó.
	KeyInstallDone = "instalacion_completa"
	// KeyPlannedMode es lo que la persona contestó en el paso 2 del
	// asistente. No es channel.modo: en F1 el canal se queda en sombra.
	KeyPlannedMode = "modo_previsto"
	// KeyOutputTarget y KeyAirReturn son las respuestas del paso 4.
	KeyOutputTarget = "salida.destino"
	KeyAirReturn    = "salida.retorno_de_aire"
	// KeyBarsSeen es la respuesta del paso 5 (el motor de barras es F2).
	KeyBarsSeen = "instalacion.ve_barras"
	// KeyCountry y KeyQuality son las del paso 6.
	KeyCountry = "pais"
	KeyQuality = "calidad"
)

// DBName es el nombre del archivo de la base dentro de la carpeta de datos.
const DBName = "antena.db"

// Valores de fábrica de las goroutines de mantenimiento. Todos son números
// del PRD, no gustos.
const (
	// RelaunchDelay es lo que se espera antes de relanzar una goroutine que
	// se cayó (auditoría A2).
	RelaunchDelay = 5 * time.Second
	// RecalcDebounce junta varios cambios seguidos en una sola corrida.
	RecalcDebounce = 2 * time.Second
	// BackupEvery y BackupsKept son el respaldo de cada hora (PRD §19).
	BackupEvery = time.Hour
	BackupsKept = 24
	// DiskEvery es cada cuánto se mira el disco.
	DiskEvery = 5 * time.Minute
	// ClockEvery es cada cuánto se compara el reloj de pared con el
	// monotónico, y ClockJump es lo que separa una deriva de un salto.
	ClockEvery = 10 * time.Second
	ClockJump  = 60 * time.Second
	// WatchPoll es cada cuánto se relee qué carpeta hay que vigilar.
	WatchPoll = 3 * time.Second
)

// Task es una goroutine más, con nombre, que corre bajo la misma protección
// contra pánico que las de casa. Existe para que las pruebas puedan meter
// una que falle a propósito.
type Task struct {
	Name string
	Run  func(context.Context) error
}

// Options es lo que hace falta para abrir la aplicación.
type Options struct {
	// DataDir es la carpeta de datos: la base, los respaldos, el portal.
	DataDir string
	// Version es la versión del ejecutable, para /estado.
	Version string
	// Now es el reloj; nil = time.Now.
	Now func() time.Time
	// Horizon es la ventana que materializa el resolver; 0 = 48 h.
	Horizon time.Duration
	// RelaunchDelay es lo que se espera antes de relanzar una goroutine
	// caída; 0 = RelaunchDelay.
	RelaunchDelay time.Duration
	// Tasks son goroutines extra, con la misma protección contra pánico.
	Tasks []Task
	// NoMaintenance apaga respaldo, disco y reloj (las pruebas no los
	// quieren). El resolver y el ingest siguen.
	NoMaintenance bool
}

// App es el proceso: la base abierta, las rutas de ffmpeg y las goroutines.
type App struct {
	Store   *store.Store
	DataDir string
	Version string

	// FFmpeg y FFprobe son las rutas encontradas; FFmpegErr dice en cristiano
	// por qué no están, si no están. Que falten no impide arrancar: se puede
	// entrar a la interfaz y arreglarlo.
	FFmpeg    string
	FFprobe   string
	FFmpegErr error

	// Restored dice que la base venía dañada y se restauró un respaldo.
	Restored     bool
	RestoredFrom string

	ChannelID int64
	Queue     *ingest.Queue

	opts   Options
	now    func() time.Time
	bus    *bus
	recalc chan struct{}

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// resolveMu deja correr una sola resolución a la vez: la de cada hora y
	// la que pide una persona pueden coincidir.
	resolveMu sync.Mutex

	mu         sync.RWMutex
	guide      []byte
	guideItems []model.PlanItem
	guideAt    time.Time
	warnings   []resolver.Warning
	advance    map[int64]int64
	alarms     []string
	started    bool
}

// Open abre la base y deja la aplicación lista para Start.
//
// Si la base no pasa la verificación de integridad (PRD §19, auditoría D2) se
// restaura sola el respaldo más reciente, se abre, y queda un incidente
// `base_restaurada`. Si no hay respaldo que restaurar, el error sube: eso ya
// no lo puede arreglar el programa.
func Open(opts Options) (*App, error) {
	if strings.TrimSpace(opts.DataDir) == "" {
		return nil, errors.New("hace falta decir dónde van los datos")
	}
	if err := os.MkdirAll(opts.DataDir, 0o755); err != nil {
		return nil, fmt.Errorf("no se pudo usar la carpeta de datos %q: %w", opts.DataDir, err)
	}
	dbPath := filepath.Join(opts.DataDir, DBName)

	var restored bool
	var restoredFrom string
	st, err := store.Open(dbPath)
	if err != nil {
		var roto *store.ErrCorrupt
		if !errors.As(err, &roto) {
			return nil, err
		}
		if roto.LastBackup == "" {
			return nil, err
		}
		if rerr := restoreBackup(roto.LastBackup, dbPath); rerr != nil {
			return nil, fmt.Errorf("%s; además, restaurar el respaldo falló: %w", roto.Error(), rerr)
		}
		st, err = store.Open(dbPath)
		if err != nil {
			return nil, fmt.Errorf("se restauró el respaldo %q y la base sigue sin abrir: %w", roto.LastBackup, err)
		}
		restored, restoredFrom = true, roto.LastBackup
	}

	a := &App{
		Store:        st,
		DataDir:      opts.DataDir,
		Version:      opts.Version,
		Restored:     restored,
		RestoredFrom: restoredFrom,
		ChannelID:    store.DefaultChannelID,
		Queue:        ingest.NewQueue(),
		opts:         opts,
		now:          opts.Now,
		bus:          newBus(),
		recalc:       make(chan struct{}, 1),
		advance:      map[int64]int64{},
	}
	if a.now == nil {
		a.now = time.Now
	}
	a.FFmpeg, a.FFprobe, a.FFmpegErr = findFFmpeg()

	if restored {
		a.Incident("base_restaurada",
			fmt.Sprintf("la base estaba dañada y se restauró sola el respaldo %q; revisa si falta algo de las últimas horas", restoredFrom))
	}
	return a, nil
}

// restoreBackup pone el respaldo en el lugar de la base, guardando la dañada
// al lado: nunca se tira nada, aunque no sirva.
func restoreBackup(backup, dbPath string) error {
	if _, err := os.Stat(dbPath); err == nil {
		roto := dbPath + ".dañada-" + time.Now().UTC().Format("20060102-150405")
		if err := os.Rename(dbPath, roto); err != nil {
			return err
		}
	}
	// Los archivos laterales del WAL de la base dañada no valen para el
	// respaldo restaurado.
	for _, ext := range []string{"-wal", "-shm"} {
		_ = os.Remove(dbPath + ext)
	}
	in, err := os.ReadFile(backup)
	if err != nil {
		return err
	}
	return os.WriteFile(dbPath, in, 0o644)
}

// findFFmpeg busca las dos herramientas y traduce el fallo a cristiano.
func findFFmpeg() (ffmpeg, ffprobe string, err error) {
	ffmpeg, e1 := engine.FFmpeg()
	ffprobe, e2 := engine.FFprobe()
	switch {
	case e1 != nil && e2 != nil:
		return "", "", errors.New("no encuentro ffmpeg ni ffprobe: van junto al ejecutable o en el PATH")
	case e1 != nil:
		return "", ffprobe, errors.New("no encuentro ffmpeg: va junto al ejecutable o en el PATH")
	case e2 != nil:
		return ffmpeg, "", errors.New("no encuentro ffprobe: va junto al ejecutable o en el PATH")
	}
	return ffmpeg, ffprobe, nil
}

// Now es el reloj de la aplicación, en UTC.
func (a *App) Now() time.Time { return a.now().UTC() }

// Start levanta las goroutines. Devuelve enseguida; se para con Close o
// cancelando el contexto.
func (a *App) Start(parent context.Context) {
	a.mu.Lock()
	if a.started {
		a.mu.Unlock()
		return
	}
	a.started = true
	a.mu.Unlock()

	a.ctx, a.cancel = context.WithCancel(parent)

	a.guard("resolver", a.resolverLoop)
	a.guard("ingest", a.ingestLoop)
	a.guard("normalizacion", a.normalizeLoop)
	a.guard("portal", a.portalLoop)
	if !a.opts.NoMaintenance {
		a.guard("respaldo", a.backupLoop)
		a.guard("disco", a.diskLoop)
		a.guard("reloj", a.clockLoop)
	}
	for _, t := range a.opts.Tasks {
		a.guard(t.Name, t.Run)
	}
}

// Close para todas las goroutines y cierra la base.
func (a *App) Close() error {
	if a.cancel != nil {
		a.cancel()
	}
	a.wg.Wait()
	a.bus.close()
	if a.Store != nil {
		return a.Store.Close()
	}
	return nil
}

// Context es el contexto de la aplicación, ya arrancada.
func (a *App) Context() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

// guard corre fn en su propia goroutine, atrapa su pánico, deja el incidente
// `panico_<nombre>` y la relanza pasados cinco segundos (auditoría A2). Es la
// única forma en que este paquete arranca una goroutine.
func (a *App) guard(name string, fn func(context.Context) error) {
	ctx := a.ctx
	delay := a.opts.RelaunchDelay
	if delay <= 0 {
		delay = RelaunchDelay
	}
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		for {
			if ctx.Err() != nil {
				return
			}
			err := a.runOnce(ctx, name, fn)
			if ctx.Err() != nil {
				return
			}
			if err != nil && !errors.Is(err, context.Canceled) {
				a.Publish("goroutine", name, fmt.Sprintf("%s se cayó y vuelve en %s: %v", name, delay, err))
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
		}
	}()
}

// runOnce corre fn una vez y convierte su pánico en un error y un incidente.
func (a *App) runOnce(ctx context.Context, name string, fn func(context.Context) error) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("pánico en %s: %v", name, p)
			a.Incident("panico_"+name, fmt.Sprintf("%v\n%s", p, oneScreen(debug.Stack())))
		}
	}()
	return fn(ctx)
}

// oneScreen recorta la traza a lo que cabe en una pantalla: el incidente lo
// lee una persona, no un depurador.
func oneScreen(stack []byte) string {
	s := string(stack)
	if len(s) > 2000 {
		s = s[:2000] + "…"
	}
	return s
}

// Incident deja un incidente en la bitácora y lo empuja por el WebSocket.
// Nunca devuelve error: si no se puede escribir, el aire sigue igual.
func (a *App) Incident(kind, detail string) {
	inc := model.Incident{
		ChannelID: a.ChannelID,
		Kind:      kind,
		Start:     a.Now(),
		Detail:    detail,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = a.Store.Incident.Insert(ctx, &inc)
	a.Publish("incidente", kind, detail)
}

// Recalc pide una corrida del resolver. No bloquea ni encola dos: la corrida
// que venga junta todo lo que haya pasado.
func (a *App) Recalc() {
	select {
	case a.recalc <- struct{}{}:
	default:
	}
}

// Alarms son las alarmas vivas que enseña /estado.
func (a *App) Alarms() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := append([]string(nil), a.alarms...)
	if a.FFmpegErr != nil {
		out = append(out, a.FFmpegErr.Error())
	}
	if a.Restored {
		out = append(out, "la base estaba dañada y se restauró el respaldo más reciente")
	}
	return out
}

func (a *App) setAlarms(list []string) {
	a.mu.Lock()
	a.alarms = list
	a.mu.Unlock()
}
