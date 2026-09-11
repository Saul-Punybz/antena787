package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sync/atomic"
	"time"
)

// Event es lo que el servidor de cuadros deja escrito: cada cambio de clip
// y cada cosa que tuvo que hacer solo para proteger el aire (un Incidente
// del glosario). El analizador de F0 lee esto para saber dónde medir.
type Event struct {
	At     time.Time `json:"at"`
	Frame  int64     `json:"frame"`  // cuadro global de salida donde ocurre
	Sample int64     `json:"sample"` // muestra global de audio donde ocurre
	Kind   string    `json:"kind"`   // corte | clip_corto | clip_fallido | audio_corto | cuadro_sostenido | atraso | deriva | fuente | relleno | fin
	Clip   string    `json:"clip,omitempty"`
	Detail string    `json:"detail,omitempty"`
}

// ClipSource decide qué sale después. El motor no sabe qué es un plan_item,
// un deck ni una regla: solo pregunta qué toca ahora y hasta cuándo puede
// darlo por bueno sin volver a preguntar. La implementa internal/app.
type ClipSource interface {
	// Next devuelve el clip que le toca al instante now —que es el reloj del
	// aire, monotónico, no el de pared (F2-90)— y el instante del próximo
	// corte conocido: fin del plan_item, corte pautado, o el cero de
	// time.Time si el clip sale entero, dure lo que diga el archivo.
	Next(now time.Time) (clip Clip, until time.Time, err error)
	// Filler es el relleno vigente cuando Next no tiene nada que ofrecer.
	Filler() Clip
}

// Sink es a dónde van los cuadros ya conformados: en el canal real, el
// encoder persistente. Es una interfaz para que el conformado se pueda
// probar sin levantar ffmpeg — el archivo que un humano lee línea por línea
// (F2-109) merece pruebas que corran en un segundo. *Encoder la cumple.
type Sink interface {
	WriteFrame(fr []byte) error
	WriteAudio(pcm []byte) error
}

// Avisada es una fuente que además quiere saber qué pasó con lo que
// entregó. Es opcional —el motor funciona sin ella—: internal/app la
// implementa para cerrar el as-run cuando un bloque sale (MarkAired) y para
// poner en cuarentena el archivo que falla dos veces (F2-12).
type Avisada interface {
	// ClipSalio dice que un clip acabó de salir: cuándo empezó, en el reloj
	// del aire, y cuántos cuadros salieron. entero es falso cuando el
	// archivo no dio todo lo que decía durar.
	ClipSalio(clip Clip, arranque time.Time, cuadros int64, entero bool)
	// ClipFallo dice que un clip no se pudo poner —o se cortó a mitad— y por
	// qué. La cascada ya lo cubrió cuando esto se llama.
	ClipFallo(clip Clip, motivo string)
}

// Valores de fábrica del servidor de cuadros. Todos son decisiones del PRD
// §14.1 o números que la F0 midió, no gustos.
const (
	// PrerollCuadros es lo que se le deja de búfer a cada decodificador.
	PrerollCuadros = 30
	// PrerollLead es cuánto antes del corte se abre el clip que viene. Un
	// ffmpeg tarda unas décimas en dar su primer cuadro; con un segundo y
	// medio llega caliente aunque la máquina esté ocupada (F2-05).
	PrerollLead = 1500 * time.Millisecond
	// Lead es cuánto se adelanta el motor al reloj: se escribe el cuadro
	// antes de que toque, para que un hipo del sistema no se vea.
	Lead = 300 * time.Millisecond
	// FadeMs es el fundido de audio en cada borde de clip: cinco
	// milisegundos no se oyen y matan el clic del corte.
	FadeMs = 5
	// DerivaPorMinuto es lo más que el reloj del aire se acerca a la hora de
	// pared en un minuto. Nunca de un salto: un salto en el aire son cuadros
	// repetidos o perdidos, y eso se ve (PRD §14.1, F2-14).
	DerivaPorMinuto = 300 * time.Millisecond
	// DerivaMinima es a partir de qué diferencia se corrige. Por debajo es
	// el temblor normal de una espera del sistema, no deriva.
	DerivaMinima = 20 * time.Millisecond
	// PreguntaMinima es lo mínimo que pasa entre dos preguntas a la fuente.
	// Una fuente que contesta «pregúntame en un segundo» no puede volverse
	// una consulta a la base por cuadro.
	PreguntaMinima = 250 * time.Millisecond
)

// sinCorte es el límite de una tanda que sale entera: la fuente no fijó
// corte y el clip dura lo que diga el archivo.
const sinCorte = int64(math.MaxInt64)

// Server es el servidor de cuadros: la pieza en Go entre los decodificadores
// por clip y el encoder persistente (PRD §14.1). Entrega un flujo continuo,
// conformado y a tiempo, pase lo que pase con los archivos.
//
// Su tiempo es el **reloj del aire**: el instante que le toca a cada cuadro,
// contado en cuadros desde el arranque (AirTime). No es time.Now — si
// alguien cambia la hora de la máquina, o el NTP llega tarde, el aire no
// salta (F2-90). La diferencia con la hora de pared se cierra por deriva
// gradual, un poco cada minuto (F2-14).
type Server struct {
	Format Format
	Enc    Sink
	// Source es quien decide qué sale. Sin fuente, todo es relleno.
	Source ClipSource
	// Filler es el relleno de respaldo cuando la fuente no ofrece ninguno.
	Filler      Clip
	Preroll     int           // cuadros de búfer por decodificador
	PrerollLead time.Duration // cuánto antes del corte se abre el que viene
	Lead        time.Duration // cuánto se adelanta al reloj
	FadeMs      int           // fundido de audio en cada borde de clip
	Events      io.Writer     // JSON por línea
	Ffmpeg      string
	Ffprobe     string

	frame         int64     // cuadros entregados
	last          []byte    // último cuadro entregado, para sostenerlo
	start         time.Time // arranque, con su lectura monotónica
	origen        time.Time // la hora de pared que le tocó al cuadro 0
	correccion    time.Duration
	proximaDeriva int64
	avisos        Avisada
	enc           *json.Encoder
	behind        atomic.Int64
}

// NewServer arma el servidor de cuadros con la fuente que decide el aire.
// enc es el encoder persistente (*Encoder lo cumple); filler es el relleno
// de respaldo si la fuente no ofrece ninguno.
func NewServer(format Format, enc Sink, src ClipSource, filler Clip) *Server {
	return &Server{Format: format, Enc: enc, Source: src, Filler: filler}
}

// AirTime es el reloj del aire: el instante que le corresponde al cuadro que
// se va a entregar. Se cuenta en cuadros, así que no salta nunca.
func (s *Server) AirTime() time.Time {
	return s.origen.Add(time.Duration(float64(s.frame) * s.Format.FrameDurationNs()))
}

func (s *Server) log(kind, clip, detail string) {
	ev := Event{At: time.Now(), Frame: s.frame, Sample: s.Format.SamplesUpTo(s.frame), Kind: kind, Clip: clip, Detail: detail}
	if s.enc != nil {
		_ = s.enc.Encode(ev)
	}
}

// Run entrega cuadros hasta que pase dur —cero es hasta que se cancele el
// contexto— y no deja de escribir mientras corre: lo que falta lo cubre el
// relleno, y lo que no llega a tiempo se sostiene.
func (s *Server) Run(ctx context.Context, dur time.Duration) error {
	s.prepara()
	var deadline time.Time
	if dur > 0 {
		deadline = s.start.Add(dur)
	}

	t := s.abrirLoQueToca(ctx)
	if t == nil {
		return errors.New("no hay nada que poner al aire: ni el plan ni el relleno abrieron")
	}
	for t != nil && s.sigue(ctx, deadline) {
		s.log("corte", t.clip.Name, s.deTanda(t))
		entregados, err := s.emitir(ctx, t, deadline)
		motivo := t.dec.Close()
		if err != nil {
			s.cerrar(t.sig)
			return err
		}
		s.contar(t, entregados, motivo)

		// Lo que el archivo no dio antes del corte lo cubre el relleno: el
		// aire no espera a nadie (F2-09, F2-10).
		if falta := s.falta(t, entregados); falta > 0 {
			if err := s.rellenar(ctx, falta, deadline); err != nil {
				s.cerrar(t.sig)
				return err
			}
		}

		t = t.sig // lo que el pre-roll dejó abierto, si llegó a abrirlo
		if t == nil && s.sigue(ctx, deadline) {
			t = s.abrirLoQueToca(ctx)
		}
	}
	s.cerrar(t)
	s.log("fin", "", fmt.Sprintf("%d cuadros en %s", s.frame, time.Since(s.start).Round(time.Second)))
	return nil
}

// prepara pone los valores de fábrica y arranca el reloj del aire.
func (s *Server) prepara() {
	if s.Events != nil {
		s.enc = json.NewEncoder(s.Events)
	}
	if s.Preroll == 0 {
		s.Preroll = PrerollCuadros
	}
	if s.PrerollLead == 0 {
		s.PrerollLead = PrerollLead
	}
	if s.Lead == 0 {
		s.Lead = Lead
	}
	if s.FadeMs == 0 {
		s.FadeMs = FadeMs
	}
	if av, ok := s.Source.(Avisada); ok {
		s.avisos = av
	}
	s.start = time.Now()
	// Round(0) suelta la lectura monotónica: origen es una etiqueta de hora
	// de pared. Lo que mide el paso del tiempo es s.start, que la conserva.
	s.origen = s.start.Round(0).UTC()
	s.proximaDeriva = s.cuadros(time.Minute)
}

// sigue dice si hay que seguir entregando cuadros.
func (s *Server) sigue(ctx context.Context, deadline time.Time) bool {
	return ctx.Err() == nil && (deadline.IsZero() || time.Now().Before(deadline))
}

// cuadros convierte un tiempo en cuadros del formato de casa.
func (s *Server) cuadros(d time.Duration) int64 {
	return int64(math.Round(float64(d) / s.Format.FrameDurationNs()))
}

// relleno es el relleno vigente: el que diga la fuente y, si no dice nada,
// el que se fijó al arrancar.
func (s *Server) relleno() Clip {
	if s.Source != nil {
		if c := s.Source.Filler(); c.Path != "" {
			return c
		}
	}
	return s.Filler
}

// ── una tanda: un clip con el aire ────────────────────────────────────

// tanda es el clip que tiene el aire: su decodificador corriendo, el
// instante en que hay que dejarlo, y —cuando el corte se acerca— el clip que
// viene ya abierto.
type tanda struct {
	dec      *Decoder
	clip     Clip
	arranque time.Time // reloj del aire cuando empezó
	hasta    time.Time // el corte; cero = hasta que el archivo se acabe
	limite   int64     // cuadros desde arranque hasta el corte
	sig      *tanda    // pre-roll: lo que ya se abrió para el corte
	relleno  bool      // cubriendo un hueco: no le pregunta nada a la fuente

	proximaPregunta int64 // cuadro desde el que se puede volver a preguntar
}

// abrirLoQueToca pregunta qué va ahora mismo y lo abre.
func (s *Server) abrirLoQueToca(ctx context.Context) *tanda {
	ahora := s.AirTime()
	clip, hasta := s.pedir(ahora)
	return s.abrirClip(ctx, clip, hasta, ahora)
}

// pedir le pregunta a la fuente qué toca en ese instante del aire. Si no hay
// fuente, si no supo, o si no tiene nada, el que sale es el relleno: la
// cascada termina en el cartel y nunca en negro (F2-09, F2-10).
func (s *Server) pedir(cuando time.Time) (Clip, time.Time) {
	if s.Source != nil {
		clip, hasta, err := s.Source.Next(cuando)
		switch {
		case err != nil:
			s.log("fuente", "", "la fuente no supo qué poner: "+err.Error())
		case clip.Path != "":
			return clip, hasta
		}
	}
	return s.relleno(), time.Time{}
}

// abrirClip arranca el decodificador de un clip y lo deja listo para salir.
// Si el archivo no abre —borrado, en cero, el NAS que se reinició— sale el
// relleno y queda dicho: un archivo no puede parar el aire (F2-12).
func (s *Server) abrirClip(ctx context.Context, clip Clip, hasta, arranque time.Time) *tanda {
	d, err := StartDecoder(ctx, s.Ffmpeg, s.Ffprobe, s.Format, clip, s.Preroll)
	if err != nil {
		s.log("clip_fallido", clip.Name, err.Error())
		s.aviso(clip, err.Error())
		relleno := s.relleno()
		if relleno.Path == "" || relleno.Path == clip.Path {
			return nil
		}
		clip = relleno
		if d, err = StartDecoder(ctx, s.Ffmpeg, s.Ffprobe, s.Format, clip, s.Preroll); err != nil {
			s.log("clip_fallido", clip.Name, err.Error())
			return nil
		}
	}
	t := &tanda{dec: d, clip: clip, arranque: arranque, hasta: hasta, limite: sinCorte}
	if !hasta.IsZero() {
		t.limite = s.limiteHasta(arranque, hasta)
	}
	return t
}

// limiteHasta son los cuadros que caben entre dos instantes del aire. Nunca
// menos de uno: un corte ya pasado se cubre con un cuadro, no con ninguno.
func (s *Server) limiteHasta(arranque, hasta time.Time) int64 {
	if n := s.cuadros(hasta.Sub(arranque)); n > 1 {
		return n
	}
	return 1
}

// cerrar mata el decodificador de una tanda que no llegó a salir.
func (s *Server) cerrar(t *tanda) {
	if t != nil && t.dec != nil {
		t.dec.Close()
	}
}

// deTanda es la línea que queda en el registro al empezar un clip.
func (s *Server) deTanda(t *tanda) string {
	if t.limite == sinCorte {
		return fmt.Sprintf("esperados %d cuadros", t.dec.ExpectedFrames)
	}
	return fmt.Sprintf("esperados %d cuadros, corte a las %s", t.dec.ExpectedFrames, t.hasta.Format("15:04:05"))
}

// ── entregar un clip ──────────────────────────────────────────────────

// emitir entrega la tanda hasta su corte y devuelve cuántos cuadros salieron.
// Aquí pasan las tres promesas del conformado:
//
//   - si el audio se acaba antes que el video, lo que falta se cubre con
//     silencio digital y la imagen no se recorta (F2-02);
//   - si el video se acaba antes que el audio, se sostiene el último cuadro
//     hasta que el sonido termine, en vez de cortar el audio (F2-03);
//   - a PrerollLead del corte se abre el clip que viene, para que su
//     decodificador esté caliente cuando haga falta (F2-05).
//
// La geometría —pillarbox, tasa de cuadros, desentrelazado (F2-04)— la hace
// el decodificador, en videoFilter.
func (s *Server) emitir(ctx context.Context, t *tanda, deadline time.Time) (int64, error) {
	d := t.dec
	fade := s.Format.SampleRate * s.FadeMs / 1000
	var entregados, faltoAudio int64

	// Un cuadro de anticipación en el video: para saber que el que tengo en
	// la mano es el último y fundir el audio antes de mandarlo.
	cur, hay := d.NextFrame()
	primero := true
	for hay && entregados < t.limite && s.sigue(ctx, deadline) {
		if !t.relleno {
			s.quizasPreroll(ctx, t, entregados)
		}
		var sig []byte
		var mas bool
		if entregados+1 < t.limite {
			sig, mas = s.siguienteOSostenido(d, cur)
		}
		quiere := s.samplesFor(s.frame)
		pcm, hubo := d.ReadSamples(quiere)
		faltoAudio += int64(quiere - hubo)
		if primero {
			fadeIn(pcm, s.Format.Channels, fade)
			primero = false
		}
		if !mas {
			fadeOut(pcm, s.Format.Channels, quiere, fade)
		}
		if err := s.emit(cur, pcm); err != nil {
			return entregados, err
		}
		entregados++
		cur, hay = sig, mas
	}

	// El video se acabó y el audio no: se sostiene el último cuadro hasta
	// que el sonido termine. Cortar el audio para igualar la imagen se oye;
	// un cuadro quieto tres décimas no se ve (F2-03).
	var sostenidos int64
	for !hay && entregados < t.limite && s.sigue(ctx, deadline) {
		quiere := s.samplesFor(s.frame)
		pcm, hubo := d.ReadSamples(quiere)
		if hubo == 0 {
			break
		}
		if hubo < quiere {
			fadeOut(pcm, s.Format.Channels, quiere, fade)
		}
		if err := s.emit(s.last, pcm); err != nil {
			return entregados, err
		}
		entregados++
		sostenidos++
	}
	if sostenidos > 0 {
		s.log("cuadro_sostenido", t.clip.Name, fmt.Sprintf(
			"el video acabó %d ms antes que el audio; se sostuvo el último cuadro",
			int64(float64(sostenidos)*s.Format.FrameDurationNs()/1e6)))
	}
	// Unas muestras al final es el codificador de la fuente (AAC, AC-3) y
	// no vale un evento; más de un cuadro sí: el archivo trae menos audio
	// que video y el aire lo cubrió con silencio.
	if float64(faltoAudio) > s.Format.SamplesPerFrame() {
		s.log("audio_corto", t.clip.Name, fmt.Sprintf("faltaron %d ms de audio; se cubrieron con silencio",
			faltoAudio*1000/int64(s.Format.SampleRate)))
	}
	return entregados, nil
}

// quizasPreroll abre el clip que viene cuando el corte ya está cerca. Se
// pregunta una vez por corte; si la fuente contesta el mismo clip que está
// sonando —una fuente puede decir «pregúntame en un segundo»— no se abre
// nada: se corre el corte y el clip sigue, sin reiniciarse.
func (s *Server) quizasPreroll(ctx context.Context, t *tanda, entregados int64) {
	if t.sig != nil || s.frame < t.proximaPregunta {
		return
	}
	fin := t.limite
	if fin == sinCorte {
		if t.dec.ExpectedFrames <= 0 {
			return // no se sabe cuándo acaba: se preguntará al acabarse
		}
		fin = t.dec.ExpectedFrames
	}
	if fin-entregados > s.cuadros(s.PrerollLead) {
		return
	}
	t.proximaPregunta = s.frame + s.cuadros(PreguntaMinima)

	corte := t.hasta
	if corte.IsZero() {
		corte = t.arranque.Add(time.Duration(float64(fin) * s.Format.FrameDurationNs()))
	}
	clip, hasta := s.pedir(corte)
	if clip.Path == t.clip.Path && !hasta.IsZero() && hasta.After(t.hasta) {
		// El mismo clip sigue con el aire: se corre el corte.
		t.hasta = hasta
		t.limite = s.limiteHasta(t.arranque, hasta)
		return
	}
	t.sig = s.abrirClip(ctx, clip, hasta, corte)
}

// siguienteOSostenido pide el siguiente cuadro; si el decodificador no lo
// tiene a tiempo, sostiene el anterior y lo anota. Nunca se espera con el
// aire vacío.
func (s *Server) siguienteOSostenido(d *Decoder, cur []byte) ([]byte, bool) {
	frameDur := time.Duration(s.Format.FrameDurationNs())
	select {
	case fr, ok := <-d.frames:
		return fr, ok
	case <-time.After(s.Lead + 2*frameDur):
		s.log("cuadro_sostenido", d.Clip.Name, "el decodificador no entregó a tiempo")
		return cur, true
	}
}

// contar deja dicho cómo salió la tanda y avisa a la fuente: es lo que lee
// el analizador y lo que la fuente necesita para cerrar el as-run.
func (s *Server) contar(t *tanda, entregados int64, motivo string) {
	esperados := t.dec.ExpectedFrames
	// Cero cuadros es un fallo aunque nadie supiera cuánto duraba: un
	// archivo borrado o en cero abre y no da nada (F2-12).
	entero := entregados > 0 && (esperados <= 0 || entregados >= esperados-2 || entregados >= t.limite)
	if !entero {
		detalle := fmt.Sprintf("no dio un solo cuadro. ffmpeg: %s", oneLine(motivo))
		if entregados > 0 {
			detalle = fmt.Sprintf("entregó %d de %d cuadros; faltan %d. ffmpeg: %s",
				entregados, esperados, esperados-entregados, oneLine(motivo))
		}
		s.log("clip_corto", t.clip.Name, detalle)
		s.aviso(t.clip, detalle)
	}
	if t.relleno {
		return // el relleno no es as-run de nadie
	}
	if s.avisos != nil {
		s.avisos.ClipSalio(t.clip, t.arranque, entregados, entero)
	}
}

// aviso le dice a la fuente que un clip falló, si la fuente quiere saberlo.
func (s *Server) aviso(clip Clip, motivo string) {
	if s.avisos != nil {
		s.avisos.ClipFallo(clip, motivo)
	}
}

// falta dice cuántos cuadros hay que cubrir con relleno para llegar al
// corte. Dos cuadros de menos son el codificador de la fuente, no un hueco.
func (s *Server) falta(t *tanda, entregados int64) int64 {
	hasta := t.limite
	if hasta == sinCorte {
		hasta = t.dec.ExpectedFrames // sin corte, lo que el archivo decía durar
	}
	if hasta <= 0 || hasta == sinCorte {
		return 0
	}
	if n := hasta - entregados; n > 2 {
		return n
	}
	return 0
}

// rellenar cubre n cuadros con el relleno, en bucle si hace falta. Es el
// penúltimo escalón de la cascada; el último es sostener.
func (s *Server) rellenar(ctx context.Context, n int64, deadline time.Time) error {
	for n > 0 && s.sigue(ctx, deadline) {
		clip := s.relleno()
		if clip.Path == "" {
			return s.sostener(ctx, n, deadline, "no hay relleno cargado")
		}
		t := s.abrirClip(ctx, clip, time.Time{}, s.AirTime())
		if t == nil {
			return s.sostener(ctx, n, deadline, "el relleno no abrió")
		}
		t.relleno, t.limite = true, n
		s.log("relleno", clip.Name, fmt.Sprintf("cubre %d cuadros", n))
		hubo, err := s.emitir(ctx, t, deadline)
		t.dec.Close()
		if err != nil {
			return err
		}
		if hubo == 0 {
			return s.sostener(ctx, n, deadline, "el relleno no dio imagen")
		}
		n -= hubo
	}
	return nil
}

// sostener repite el último cuadro cuando ni el relleno da señal. Es el
// último recurso y queda dicho: un cuadro quieto es mejor que negro.
func (s *Server) sostener(ctx context.Context, n int64, deadline time.Time, porque string) error {
	s.log("cuadro_sostenido", "", fmt.Sprintf("%d cuadros sostenidos: %s", n, porque))
	for k := int64(0); k < n && s.sigue(ctx, deadline); k++ {
		if err := s.emit(s.last, s.silence(s.samplesFor(s.frame))); err != nil {
			return err
		}
	}
	return nil
}

// ── el reloj y el encoder ─────────────────────────────────────────────

// samplesFor devuelve cuántas muestras corresponden al cuadro n. La
// fracción (800.8 a 59.94) se reparte sin acumular error: F0-02 se gana aquí.
func (s *Server) samplesFor(n int64) int {
	return int(s.Format.SamplesUpTo(n+1) - s.Format.SamplesUpTo(n))
}

func (s *Server) silence(n int) []byte { return make([]byte, n*s.Format.BytesPerSample()) }

// Atrasado dice si el aire está llegando tarde a su propio reloj ahora mismo:
// más de un segundo por detrás de donde debería ir. Lo lee la cola de
// preparación desde otro hilo, para apartarse cuando el aire sufre — el aire
// manda sobre todo lo demás (ADR 0008).
//
// No es una alarma: un atraso corto se recupera solo. Es una señal de presión.
func (s *Server) Atrasado() bool { return s.behind.Load() > 0 }

// emit escribe un cuadro y su audio, a tiempo. Es el único punto que toca
// el encoder, y el único que avanza el reloj del aire.
func (s *Server) emit(frame, pcm []byte) error {
	target := s.start.Add(time.Duration(float64(s.frame)*s.Format.FrameDurationNs()) - s.correccion)
	wait := time.Until(target.Add(-s.Lead))
	if wait > 0 {
		time.Sleep(wait)
	} else if wait < -time.Second {
		n := s.behind.Add(1)
		if n%60 == 1 {
			s.log("atraso", "", fmt.Sprintf("el aire va %s atrasado", (-wait).Round(time.Millisecond)))
		}
	} else {
		s.behind.Store(0)
	}
	if frame == nil {
		frame = s.blackFrame()
	}
	if err := s.Enc.WriteFrame(frame); err != nil {
		return fmt.Errorf("escribiendo video al encoder: %w", err)
	}
	if err := s.Enc.WriteAudio(pcm); err != nil {
		return fmt.Errorf("escribiendo audio al encoder: %w", err)
	}
	s.last = frame
	s.frame++
	if s.frame >= s.proximaDeriva {
		s.corregirDeriva()
	}
	return nil
}

// corregirDeriva compara el reloj del aire con la hora de pared una vez por
// minuto y los acerca poco a poco, nunca de un salto (F2-14). Una diferencia
// mayor que un minuto ya no es deriva sino un salto de reloj, y de eso se
// encarga internal/app (F2-89): aquí se anota y se corrige lo que se pueda.
func (s *Server) corregirDeriva() {
	s.proximaDeriva = s.frame + s.cuadros(time.Minute)
	// El motor entrega cada cuadro Lead antes de su hora —ese es el colchón
	// que hace que un hipo del sistema no se vea—, así que lo que se compara
	// con la hora de pared es cuándo se entregó, no la hora del cuadro.
	diff := time.Now().Round(0).UTC().Sub(s.AirTime().Add(-s.Lead))
	if diff > -DerivaMinima && diff < DerivaMinima {
		return
	}
	paso := diff
	switch {
	case paso > DerivaPorMinuto:
		paso = DerivaPorMinuto
	case paso < -DerivaPorMinuto:
		paso = -DerivaPorMinuto
	}
	s.correccion += paso
	s.log("deriva", "", fmt.Sprintf("el aire iba %s de la hora de pared; se corrige %s en este minuto",
		diff.Round(time.Millisecond), paso.Round(time.Millisecond)))
}

// blackFrame solo existe para el primerísimo cuadro si el primer
// decodificador no dio nada. Si aparece en la salida, es un fallo y el
// analizador lo ve.
func (s *Server) blackFrame() []byte {
	b := make([]byte, s.Format.FrameBytes())
	for i := s.Format.Width * s.Format.Height; i < len(b); i++ {
		b[i] = 128
	}
	return b
}

// ── fundidos ──────────────────────────────────────────────────────────

func fadeIn(pcm []byte, ch, n int) {
	for i := 0; i < n && (i+1)*2*ch <= len(pcm); i++ {
		g := float64(i) / float64(n)
		scale(pcm, i, ch, g)
	}
}

func fadeOut(pcm []byte, ch, total, n int) {
	for i := 0; i < n; i++ {
		idx := total - 1 - i
		if idx < 0 || (idx+1)*2*ch > len(pcm) {
			break
		}
		g := float64(i) / float64(n)
		scale(pcm, idx, ch, g)
	}
}

func scale(pcm []byte, sample, ch int, g float64) {
	for c := 0; c < ch; c++ {
		p := (sample*ch + c) * 2
		v := int16(uint16(pcm[p]) | uint16(pcm[p+1])<<8)
		v = int16(math.Round(float64(v) * g))
		pcm[p] = byte(v)
		pcm[p+1] = byte(uint16(v) >> 8)
	}
}

func oneLine(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, ' ', '|', ' ')
		} else {
			out = append(out, s[i])
		}
	}
	if len(out) > 300 {
		out = out[:300]
	}
	return string(out)
}
