// motor.go es el motor dentro del proceso de verdad: el bucle que enciende
// el encoder persistente y el servidor de cuadros cuando el canal está al
// aire, y la fuente que traduce el plan a clips. Es la tanda T1 de
// docs/f2/PLAN-F2.md.
//
// Un solo carril: aquí todavía no hay decks ni prioridad —eso es T2—. Lo que
// hay es la promesa del §9 paso 4 del PRD: sale lo que dice el plan, y cuando
// el plan no tiene nada sale el relleno; si no hay relleno, el cartel de la
// estación. Nunca negro, nunca silencio.
//
// En modo sombra este archivo no hace nada más que decirlo una vez: F1 no
// cambia de conducta porque F2 exista.
package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"antena787/internal/engine"
	"antena787/internal/model"
)

// Los dos modos del canal (channel.modo).
const (
	ModoSombra = "sombra"
	ModoAire   = "aire"
)

// Números del motor. Los de F2-11 y F2-12 son del PRD; los demás son lo más
// simple que funciona y están aquí para poder cambiarlos de un sitio.
const (
	// MotorPoll es cada cuánto se mira si el canal ya pasó a estar al aire.
	MotorPoll = 10 * time.Second
	// EsperaMotorMin y EsperaMotorMax son la espera progresiva entre
	// reinicios del encoder: la primera vez un segundo, el doble cada vez,
	// tope de medio minuto (F2-11).
	EsperaMotorMin = time.Second
	EsperaMotorMax = 30 * time.Second
	// VidaBuena es lo que tiene que aguantar una corrida del encoder para
	// que la espera vuelva a empezar por abajo.
	VidaBuena = 5 * time.Minute
	// VentanaAtras y VentanaAdelante son cuánto plan se lee alrededor de
	// ahora. Hacia atrás cubre un bloque largo que empezó antes.
	VentanaAtras    = 12 * time.Hour
	VentanaAdelante = 6 * time.Hour
	// SeekMinimo es desde cuánto vale la pena entrar a un archivo por el
	// medio. Por debajo se abre desde el principio: buscar un cuadro clave
	// cuesta más que los dos segundos que se ahorran (F2-13).
	SeekMinimo = 2 * time.Second
	// FalloRepetido es lo que tienen que estar separados dos fallos del mismo
	// archivo para que el segundo lo mande a cuarentena. Un parpadeo de red
	// no saca de la parrilla un programa bueno (F2-12).
	FalloRepetido = 5 * time.Minute
	// RellenoSuelto es lo que se le da al relleno cuando no se sabe cuánto
	// dura ni cuándo empieza lo siguiente.
	RellenoSuelto = time.Minute
	// RepetirIncidente es cada cuánto se vuelve a anotar el mismo motivo de
	// parada. Entre medias se dice por el bus, para que la bitácora siga
	// siendo legible cuando algo lleva horas mal.
	RepetirIncidente = 10 * time.Minute
)

// motorLoop es la goroutine del aire. Corre bajo guard como todas: si algo
// dentro entra en pánico, queda el incidente y vuelve sola (auditoría A2).
func (a *App) motorLoop(ctx context.Context) error {
	dijoSombra := false
	espera := EsperaMotorMin
	var ultimoMotivo string
	var ultimoIncidente time.Time
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		ch, err := a.Store.Channel.Get(ctx, a.ChannelID)
		if err != nil {
			if !dormir(ctx, MotorPoll) {
				return ctx.Err()
			}
			continue
		}
		if ch.Mode != ModoAire {
			// Se dice una vez, no cada diez segundos: la bitácora es para
			// leerla.
			if !dijoSombra {
				a.Publish("motor", ModoSombra,
					"el canal está en modo sombra: se arma el plan y se publica la guía, pero no se emite")
				dijoSombra = true
			}
			if !dormir(ctx, MotorPoll) {
				return ctx.Err()
			}
			continue
		}
		dijoSombra = false

		arranque := time.Now()
		err = a.correrMotor(ctx, ch)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if time.Since(arranque) >= VidaBuena {
			espera = EsperaMotorMin
		}
		motivo := "el encoder terminó por su cuenta"
		if err != nil {
			motivo = err.Error()
		}
		texto := fmt.Sprintf("el motor se paró después de %s (%s) y vuelve en %s",
			time.Since(arranque).Round(time.Second), motivo, espera)
		// Un incidente por vuelta llenaría la bitácora el día que lo que
		// falte sea ffmpeg: el mismo motivo se anota una vez cada diez
		// minutos y, entre medias, se dice por el bus.
		if motivo != ultimoMotivo || time.Since(ultimoIncidente) > RepetirIncidente {
			a.Incident(model.IncEncoderReiniciado.String(), texto)
			ultimoMotivo, ultimoIncidente = motivo, time.Now()
		} else {
			a.Publish("motor", "reintento", texto)
		}
		if !dormir(ctx, espera) {
			return ctx.Err()
		}
		if espera *= 2; espera > EsperaMotorMax {
			espera = EsperaMotorMax
		}
	}
}

// dormir espera d o hasta que se cancele el contexto. Devuelve false si lo
// que llegó fue la cancelación.
func dormir(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}

// correrMotor enciende el encoder y el servidor de cuadros, y no vuelve hasta
// que algo se rompe o se pide parar. Una vida del encoder por llamada: si
// muere, quien llama espera y entra otra vez (F2-11; el watchdog de los 3 s
// sobre el encoder colgado es de T6).
func (a *App) correrMotor(ctx context.Context, ch model.Channel) error {
	if a.FFmpeg == "" {
		return a.FFmpegErr
	}
	formato := FormatOf(ch.FormatProfile)
	salidas, err := a.salidasDelMotor(ctx)
	if err != nil {
		return err
	}

	// Lo que quedó cargado de la vida anterior vuelve a planeado: el motor
	// recalcula desde el instante real y entra al archivo por donde toca,
	// nunca desde el principio (F2-13).
	if n, err := a.Store.Plan.ResetCuedToPlanned(ctx, a.ChannelID); err == nil && n > 0 {
		a.Publish("motor", "arranque", fmt.Sprintf(
			"%d bloques que estaban cargados vuelven a planeados; el que toque entra por el minuto que le corresponde", n))
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	enc, err := engine.StartEncoder(ctx, a.FFmpeg, formato, salidas)
	if err != nil {
		return fmt.Errorf("no se pudo encender la salida: %w", err)
	}
	// Si el encoder se muere por su cuenta, el servidor de cuadros no puede
	// seguir escribiendo a un caño roto: se para y quien llama lo relanza.
	// El vigilante suelta enc.Done() en cuanto se cancela el contexto, porque
	// Finish lee ese mismo canal y si se lo queda el vigilante, Finish espera
	// un minuto por nada.
	muerto := make(chan error, 1)
	vigilado := make(chan struct{})
	go func() {
		defer close(vigilado)
		select {
		case err := <-enc.Done():
			muerto <- err
			cancel()
		case <-ctx.Done():
		}
	}()

	fuente := a.nuevaFuenteDelPlan(ctx, formato)
	srv := engine.NewServer(formato, enc, fuente, fuente.Filler())
	srv.Ffmpeg, srv.Ffprobe = a.FFmpeg, a.FFprobe
	if f, err := a.registroDelMotor(); err == nil {
		srv.Events = f
		defer func() { _ = f.Close() }()
	}

	a.Publish("motor", "aire", fmt.Sprintf("el canal está al aire por %s", nombresDeSalidas(salidas)))
	runErr := srv.Run(ctx, 0)
	cancel()
	<-vigilado
	select {
	case err := <-muerto:
		// El encoder ya no está: no hay nada que cerrar bien.
		if err != nil {
			return fmt.Errorf("el encoder murió: %w", err)
		}
		return errors.New("el encoder terminó solo")
	default:
	}
	// Cerrar las entradas y dejar que ffmpeg vacíe sus colas: matarlo a mitad
	// deja el último trozo de la salida truncado.
	if err := enc.Finish(); err != nil && runErr == nil {
		return err
	}
	return runErr
}

// registroDelMotor abre el archivo donde el servidor de cuadros deja lo que
// hizo: un JSON por línea, uno por día. Es la evidencia de F2-01 y lo que
// medirá la prueba de resistencia (T10).
func (a *App) registroDelMotor() (*os.File, error) {
	dir := filepath.Join(a.DataDir, "motor")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	nombre := fmt.Sprintf("eventos-%s.jsonl", a.Now().Format("2006-01-02"))
	return os.OpenFile(filepath.Join(dir, nombre), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
}

// ── a dónde sale ──────────────────────────────────────────────────────

// salidasDelMotor traduce las salidas configuradas del canal a lo que el
// encoder entiende. En T1 hay una sola y es la misma que midió la F0: MPEG-2
// TS por UDP para el multiplexor, o a un archivo. El traductor de verdad
// —`internal/drivers/salida`, con PIDs, programa, multicast y TTL— es T2.
func (a *App) salidasDelMotor(ctx context.Context) ([]engine.Output, error) {
	salidas, err := a.Store.Output.List(ctx, a.ChannelID)
	if err != nil {
		return nil, err
	}
	for _, s := range salidas {
		out := engine.Output{Name: s.Name, Kind: "mpeg2-ts", VideoKbs: 8000, MuxKbs: 10000}
		var p struct {
			Destino string `json:"destino"`
			Ruta    string `json:"ruta"`
		}
		if s.Params != "" {
			_ = json.Unmarshal([]byte(s.Params), &p)
		}
		switch {
		case p.Destino != "":
			out.UDP = p.Destino
		case p.Ruta != "":
			out.File = p.Ruta
		default:
			continue // una salida sin destino no es una salida
		}
		return []engine.Output{out}, nil
	}

	// Nadie ha dicho todavía a qué dirección va la señal. En vez de no
	// emitir —y dejar el canal callado sin explicar por qué— se graba en la
	// carpeta de datos y se dice. La retención de esa grabación es T7.
	ruta := filepath.Join(a.DataDir, "aire", fmt.Sprintf("aire-%s.ts", a.Now().Format("20060102-1504")))
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		return nil, err
	}
	a.Publish("motor", "salida", "todavía no me has dicho a qué dirección mandar la señal: por ahora se graba en "+ruta)
	return []engine.Output{{Name: "archivo", Kind: "mpeg2-ts", File: ruta, VideoKbs: 8000, MuxKbs: 10000}}, nil
}

// nombresDeSalidas dice a dónde está saliendo, en cristiano.
func nombresDeSalidas(outs []engine.Output) string {
	for _, o := range outs {
		if o.UDP != "" {
			return o.UDP
		}
		if o.File != "" {
			return o.File
		}
	}
	return "ninguna salida"
}

// ── la fuente: del plan a clips ───────────────────────────────────────

// fuenteDelPlan es el engine.ClipSource de verdad. Contesta tres preguntas y
// nada más: qué sale ahora, hasta cuándo, y qué relleno hay puesto.
//
// Todo lo que sabe de tiempo se lo dice el motor: `now` es el reloj del aire,
// no time.Now (F2-90).
type fuenteDelPlan struct {
	app     *App
	ctx     context.Context
	formato engine.Format
	cartel  engine.Clip // el cartel de la estación, hecho al arrancar el motor

	mu      sync.Mutex
	cargado map[int64]bool      // plan_item ya marcado como cargado
	fallos  map[int64]time.Time // último fallo por media_asset (F2-12)
	dicho   map[string]bool     // lo que ya se dijo una vez
}

// nuevaFuenteDelPlan deja la fuente lista, con un cartel de la estación a
// mano: la cascada tiene que terminar en algo, y ese algo nunca es negro
// (F2-09, F2-10).
func (a *App) nuevaFuenteDelPlan(ctx context.Context, formato engine.Format) *fuenteDelPlan {
	f := &fuenteDelPlan{
		app: a, ctx: ctx, formato: formato,
		cargado: map[int64]bool{},
		fallos:  map[int64]time.Time{},
		dicho:   map[string]bool{},
	}
	f.cartel = a.cartelALaMano(ctx, formato)
	return f
}

// Next devuelve el clip que le toca al instante del aire y cuándo hay que
// volver a preguntar. Un solo carril: el plan_item que cubre ese instante.
func (f *fuenteDelPlan) Next(now time.Time) (engine.Clip, time.Time, error) {
	items, err := f.app.Store.Plan.ListRange(f.ctx, f.app.ChannelID,
		now.Add(-VentanaAtras), now.Add(VentanaAdelante))
	if err != nil {
		return f.Filler(), now.Add(RellenoSuelto), err
	}

	item, hay := f.queToca(items, now)
	if !hay {
		// Hueco: sale el relleno hasta que empiece lo siguiente.
		return f.Filler(), f.hastaLoSiguiente(items, now), nil
	}
	hasta := f.corteDe(items, item, now)

	clip, err := f.clipDe(item, now)
	if err != nil {
		f.registrarFallo(item, err.Error())
		return f.Filler(), hasta, nil
	}
	f.cargar(item)
	return clip, hasta, nil
}

// queToca elige el plan_item que cubre el instante. Con un solo carril la
// regla es corta: el que empezó y no ha terminado. Si hay dos —el esquema lo
// impide, esto es el cinturón además del tirante— sale el de menor id y queda
// el incidente (PRD §9 paso 4). Un elemento programado `dentro_de` otro manda
// sobre el que lo contiene: mientras suena, tiene el aire (F2-16).
func (f *fuenteDelPlan) queToca(items []model.PlanItem, now time.Time) (model.PlanItem, bool) {
	var elegido model.PlanItem
	var hay, solapa bool
	for _, it := range items {
		if !enCurso(it, now) {
			continue
		}
		if !hay {
			elegido, hay = it, true
			continue
		}
		// Uno dentro del otro no es un solape: es el elemento que interrumpe.
		switch {
		case it.Inside != nil && elegido.Inside == nil:
			elegido = it
		case it.Inside == nil && elegido.Inside != nil:
			// se queda el que ya estaba
		default:
			solapa = true
			if it.ID < elegido.ID {
				elegido = it
			}
		}
	}
	if solapa && f.primeraVez("solape", elegido.ID) {
		f.app.Incident(model.IncSolape.String(), fmt.Sprintf(
			"dos bloques querían salir a las %s; sale el de menor id (%d)",
			now.Format("15:04:05"), elegido.ID))
	}
	return elegido, hay
}

// enCurso dice si el ítem cubre el instante y todavía puede salir.
func enCurso(it model.PlanItem, now time.Time) bool {
	if it.State != model.Planned && it.State != model.Cued {
		return false
	}
	return !it.PlannedAt.After(now) && it.End().After(now)
}

// corteDe es hasta cuándo se puede dar por bueno lo que sale: el fin del
// ítem o, si algo empieza antes —un ID programado dentro del bloque—, ese
// instante.
func (f *fuenteDelPlan) corteDe(items []model.PlanItem, item model.PlanItem, now time.Time) time.Time {
	corte := item.End()
	for _, it := range items {
		if it.ID == item.ID || it.State != model.Planned && it.State != model.Cued {
			continue
		}
		if it.PlannedAt.After(now) && it.PlannedAt.Before(corte) {
			corte = it.PlannedAt
		}
	}
	return corte
}

// hastaLoSiguiente es cuánto puede durar el relleno de un hueco: hasta que
// empiece el próximo bloque del plan.
func (f *fuenteDelPlan) hastaLoSiguiente(items []model.PlanItem, now time.Time) time.Time {
	hasta := now.Add(RellenoSuelto)
	for _, it := range items {
		if it.State != model.Planned && it.State != model.Cued {
			continue
		}
		if it.PlannedAt.After(now) && it.PlannedAt.Before(hasta) {
			hasta = it.PlannedAt
		}
	}
	return hasta
}

// clipDe traduce un plan_item al archivo que sale al aire: la copia
// normalizada del media_asset, que es la única que está en el formato de
// casa y con el volumen del canal. Si no hay copia normalizada —y nadie la
// dejó pasar a mano— este bloque no puede salir, y quien llama cae a la
// cascada.
func (f *fuenteDelPlan) clipDe(item model.PlanItem, now time.Time) (engine.Clip, error) {
	switch item.Origin {
	case model.OriginSlate:
		c := f.Filler()
		c.Ref = item.ID
		return c, nil
	case model.OriginLiveSource:
		// Las fuentes en vivo son otra tanda. Hasta entonces la franja sale
		// con relleno y se dice una vez, no cada vez que se pregunta.
		if f.primeraVez("vivo", item.ID) {
			f.app.Publish("motor", "plan", fmt.Sprintf(
				"el bloque de las %s es una fuente en vivo y el vivo todavía no está construido: sale el relleno",
				item.PlannedAt.Format("15:04")))
		}
		return engine.Clip{}, errors.New("las fuentes en vivo todavía no están construidas")
	case model.OriginAsset, model.OriginFiller:
	default:
		return engine.Clip{}, fmt.Errorf("origen %q que el motor no sabe poner", item.Origin)
	}
	if item.MediaAssetID == nil {
		return engine.Clip{}, errors.New("el bloque no dice qué archivo hay que poner")
	}
	asset, err := f.app.Store.Media.Get(f.ctx, *item.MediaAssetID)
	if err != nil {
		return engine.Clip{}, err
	}
	ruta := asset.NormalizedPath
	if ruta == "" && asset.LetThroughBy != "" {
		// Alguien lo dejó pasar bajo su responsabilidad: sale el original
		// tal cual, que es lo que dice Biblioteca.
		ruta = asset.Path
	}
	if ruta == "" {
		return engine.Clip{}, errors.New("el archivo todavía no está preparado para el aire")
	}
	if st, err := os.Stat(ruta); err != nil {
		return engine.Clip{}, fmt.Errorf("el archivo no está donde debería: %w", err)
	} else if st.Size() == 0 {
		return engine.Clip{}, errors.New("el archivo está en cero")
	}

	clip := engine.Clip{Path: ruta, Name: f.app.TituloDelArchivo(f.ctx, asset), Ref: item.ID}
	// Entrar por el medio: el bloque empezó antes de ahora porque el
	// servicio se reinició, o porque un elemento de dentro le quitó el aire
	// un momento (F2-13, F2-16).
	if dentro := now.Sub(item.PlannedAt); dentro >= SeekMinimo {
		clip.SeekMs = dentro.Milliseconds()
	}
	return clip, nil
}

// Filler es el relleno vigente: lo primero de la biblioteca de relleno y, si
// está vacía, el cartel de la estación. Nunca devuelve nada vacío mientras
// el cartel exista.
func (f *fuenteDelPlan) Filler() engine.Clip {
	fillers, err := f.app.Store.Filler.List(f.ctx, f.app.ChannelID)
	if err == nil {
		for _, fa := range fillers {
			asset, err := f.app.Store.Media.Get(f.ctx, fa.MediaAssetID)
			if err != nil {
				continue
			}
			ruta := asset.AirablePath()
			if ruta == "" {
				continue
			}
			if st, err := os.Stat(ruta); err != nil || st.Size() == 0 {
				continue
			}
			return engine.Clip{Path: ruta, Name: "relleno · " + f.app.TituloDelArchivo(f.ctx, asset)}
		}
	}
	return f.cartel
}

// ── lo que el motor cuenta de vuelta ──────────────────────────────────

// ClipSalio cierra el as-run del bloque que acaba de salir: el instante real,
// lo que de verdad duró, y si salió entero. Es quien llama por fin a
// MarkAired, que estaba escrita desde F1 esperando al motor.
func (f *fuenteDelPlan) ClipSalio(clip engine.Clip, arranque time.Time, cuadros int64, entero bool) {
	if clip.Ref == 0 {
		return
	}
	ms := int64(float64(cuadros) * f.formato.FrameDurationNs() / 1e6)
	if err := f.app.MarkAired(f.ctx, clip.Ref, arranque, ms, !entero); err != nil && f.ctx.Err() == nil {
		f.app.Publish("motor", "as-run", fmt.Sprintf("no pude anotar lo que salió del bloque %d: %v", clip.Ref, err))
	}
}

// ClipFallo es lo que el motor vio cuando un archivo no dio lo que decía:
// dispara la cascada (ya la disparó) y cuenta el fallo para F2-12.
func (f *fuenteDelPlan) ClipFallo(clip engine.Clip, motivo string) {
	if clip.Ref == 0 {
		return
	}
	item, err := f.app.PlanItem(f.ctx, clip.Ref)
	if err != nil {
		return
	}
	f.registrarFallo(item, motivo)
}

// registrarFallo deja el incidente, marca el bloque como fallido y, si es el
// segundo fallo del mismo archivo separado por más de cinco minutos, lo manda
// a cuarentena solo: para que no se programe otra vez la semana que viene y
// falle igual (F2-12).
func (f *fuenteDelPlan) registrarFallo(item model.PlanItem, motivo string) {
	f.app.Incident(model.IncFalloDeClip.String(), fmt.Sprintf(
		"el bloque de las %s no pudo salir (%s); lo cubrió el relleno",
		item.PlannedAt.Format("15:04"), motivo))
	if err := f.app.Store.Plan.SetState(f.ctx, item.ID, model.Failed); err != nil && f.ctx.Err() == nil {
		f.app.Publish("motor", "plan", fmt.Sprintf("no pude marcar el bloque %d como fallido: %v", item.ID, err))
	}
	if item.MediaAssetID == nil {
		return
	}
	id := *item.MediaAssetID
	ahora := f.app.Now()

	f.mu.Lock()
	antes, habia := f.fallos[id]
	f.fallos[id] = ahora
	f.mu.Unlock()
	if !habia || ahora.Sub(antes) <= FalloRepetido {
		// Un solo parpadeo de red no saca de la parrilla un programa bueno.
		return
	}
	if err := f.app.Store.Media.SetState(f.ctx, id, model.AssetQuarantine,
		"falló dos veces al aire: "+motivo); err != nil {
		return
	}
	f.app.Incident(model.IncCuarentena.String(), fmt.Sprintf(
		"el archivo del bloque de las %s falló dos veces al aire y queda parado: %s",
		item.PlannedAt.Format("15:04"), motivo))
	f.app.RefreshCuarentena(f.ctx)
}

// cargar marca el bloque como cargado (cued) la primera vez que sale. Lo que
// quede cargado si el servicio se reinicia vuelve a planeado (F2-13).
func (f *fuenteDelPlan) cargar(item model.PlanItem) {
	f.mu.Lock()
	ya := f.cargado[item.ID]
	f.cargado[item.ID] = true
	f.mu.Unlock()
	if ya || item.State == model.Cued {
		return
	}
	if err := f.app.Store.Plan.SetState(f.ctx, item.ID, model.Cued); err != nil && f.ctx.Err() == nil {
		f.app.Publish("motor", "plan", fmt.Sprintf("no pude marcar el bloque %d como cargado: %v", item.ID, err))
	}
}

// primeraVez dice si algo pasa por primera vez con ese bloque. La fuente se
// consulta muchas veces por bloque; la bitácora no tiene por qué saberlo.
func (f *fuenteDelPlan) primeraVez(que string, id int64) bool {
	clave := fmt.Sprintf("%s#%d", que, id)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.dicho[clave] {
		return false
	}
	f.dicho[clave] = true
	return true
}

// ── el cartel de emergencia ───────────────────────────────────────────

// cartelALaMano devuelve el cartel de la estación que puede salir ahora
// mismo: el que dejó el asistente si está, y si no uno que se dibuja en el
// acto en la carpeta de datos. Es el último escalón de la cascada y tiene que
// existir siempre (F2-09, F2-10, F2-69).
func (a *App) cartelALaMano(ctx context.Context, formato engine.Format) engine.Clip {
	nombre := "cartel de la estación"
	if ruta := a.setting(ctx, KeyDefaultFiller); ruta != "" {
		if st, err := os.Stat(ruta); err == nil && st.Size() > 0 {
			return engine.Clip{Path: ruta, Name: nombre}
		}
	}
	ruta := filepath.Join(a.DataDir, "motor", ArchivoDelCartel)
	if st, err := os.Stat(ruta); err == nil && st.Size() > 0 {
		return engine.Clip{Path: ruta, Name: nombre}
	}
	if a.FFmpeg == "" {
		return engine.Clip{}
	}
	ch, err := a.Store.Channel.Get(ctx, a.ChannelID)
	if err != nil {
		return engine.Clip{}
	}
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		return engine.Clip{}
	}
	cuadro := ruta + ".png"
	defer func() { _ = os.Remove(cuadro) }()
	if err := DibujarCartel(ch, formato, cuadro); err != nil {
		a.Incident(model.IncCartel.String(), "no pude dibujar el cartel de la estación: "+err.Error())
		return engine.Clip{}
	}
	if err := correrFFmpegDelCartel(ctx, a.FFmpeg, cuadro, ruta, formato); err != nil {
		a.Incident(model.IncCartel.String(), "no pude preparar el cartel de la estación: "+err.Error())
		return engine.Clip{}
	}
	return engine.Clip{Path: ruta, Name: nombre}
}
