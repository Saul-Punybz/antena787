package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"time"
)

// Event es lo que el servidor de cuadros deja escrito: cada cambio de clip
// y cada cosa que tuvo que hacer solo para proteger el aire (un Incidente
// del glosario). El analizador de F0 lee esto para saber dónde medir.
type Event struct {
	At     time.Time `json:"at"`
	Frame  int64     `json:"frame"`  // cuadro global de salida donde ocurre
	Sample int64     `json:"sample"` // muestra global de audio donde ocurre
	Kind   string    `json:"kind"`   // corte | clip_corto | audio_corto | cuadro_sostenido | atraso | relleno | fin
	Clip   string    `json:"clip,omitempty"`
	Detail string    `json:"detail,omitempty"`
}

// Server es el servidor de cuadros: la pieza en Go entre los decodificadores
// por clip y el encoder persistente (PRD §14.1). Entrega un flujo continuo,
// conformado y a tiempo, pase lo que pase con los archivos.
type Server struct {
	Format   Format
	Enc      *Encoder
	Playlist []Clip
	Filler   Clip
	Preroll  int           // cuadros de pre-roll por decodificador
	Lead     time.Duration // cuánto se adelanta al reloj de pared
	FadeMs   int           // fundido de audio en cada borde de clip
	Events   io.Writer     // JSON por línea
	Ffmpeg   string
	Ffprobe  string

	frame  int64 // cuadros entregados
	last   []byte
	start  time.Time
	enc    *json.Encoder
	behind int
}

func (s *Server) log(kind, clip, detail string) {
	ev := Event{At: time.Now(), Frame: s.frame, Sample: s.Format.SamplesUpTo(s.frame), Kind: kind, Clip: clip, Detail: detail}
	if s.enc != nil {
		s.enc.Encode(ev)
	}
}

// Run emite la lista en bucle hasta que pase la duración pedida o el
// contexto se cancele. Nunca deja de escribir cuadros mientras corre.
func (s *Server) Run(ctx context.Context, dur time.Duration) error {
	if s.Events != nil {
		s.enc = json.NewEncoder(s.Events)
	}
	if s.Preroll == 0 {
		s.Preroll = 30
	}
	if s.Lead == 0 {
		s.Lead = 300 * time.Millisecond
	}
	if s.FadeMs == 0 {
		s.FadeMs = 5
	}
	s.start = time.Now()
	deadline := s.start.Add(dur)

	i := 0
	next, err := StartDecoder(ctx, s.Ffmpeg, s.Ffprobe, s.Format, s.Playlist[0], s.Preroll)
	if err != nil {
		return err
	}
	for time.Now().Before(deadline) && ctx.Err() == nil {
		cur := next
		j := (i + 1) % len(s.Playlist)
		next, err = StartDecoder(ctx, s.Ffmpeg, s.Ffprobe, s.Format, s.Playlist[j], s.Preroll)
		if err != nil {
			return err
		}
		if err := s.play(ctx, cur, cur.ExpectedFrames, deadline); err != nil {
			return err
		}
		i = j
	}
	next.Close()
	s.log("fin", "", fmt.Sprintf("%d cuadros en %s", s.frame, time.Since(s.start).Round(time.Second)))
	return nil
}

// play entrega un clip completo. Si el clip se queda corto (archivo
// corrupto, duración mentirosa), cubre lo que falta con relleno y lo deja
// registrado: el aire no espera a nadie.
func (s *Server) play(ctx context.Context, d *Decoder, expected int64, deadline time.Time) error {
	s.log("corte", d.Clip.Name, fmt.Sprintf("esperados %d cuadros", expected))
	delivered, err := s.pump(ctx, d, deadline, true)
	stderr := d.Close()
	if err != nil {
		return err
	}
	if expected > 0 && delivered < expected-2 && time.Now().Before(deadline) {
		missing := expected - delivered
		s.log("clip_corto", d.Clip.Name, fmt.Sprintf("entregó %d de %d cuadros; faltan %d. ffmpeg: %s", delivered, expected, missing, oneLine(stderr)))
		if err := s.fill(ctx, missing, deadline); err != nil {
			return err
		}
	}
	return nil
}

// fill cubre n cuadros con el relleno, en bucle si hace falta.
func (s *Server) fill(ctx context.Context, n int64, deadline time.Time) error {
	for n > 0 && time.Now().Before(deadline) && ctx.Err() == nil {
		d, err := StartDecoder(ctx, s.Ffmpeg, s.Ffprobe, s.Format, s.Filler, s.Preroll)
		if err != nil {
			return err
		}
		s.log("relleno", s.Filler.Name, fmt.Sprintf("cubre %d cuadros", n))
		got, err := s.pumpN(ctx, d, n, deadline)
		d.Close()
		if err != nil {
			return err
		}
		if got == 0 {
			// El relleno mismo no dio nada: sostener el último cuadro es
			// mejor que negro, y queda anotado.
			s.log("cuadro_sostenido", s.Filler.Name, fmt.Sprintf("%d cuadros sostenidos", n))
			for k := int64(0); k < n && time.Now().Before(deadline); k++ {
				if err := s.emit(s.last, s.silence(s.samplesFor(s.frame)), true); err != nil {
					return err
				}
			}
			return nil
		}
		n -= got
	}
	return nil
}

// pumpN entrega como máximo n cuadros del decodificador.
func (s *Server) pumpN(ctx context.Context, d *Decoder, n int64, deadline time.Time) (int64, error) {
	return s.pumpLimit(ctx, d, deadline, n, true)
}

// pump entrega todos los cuadros del decodificador.
func (s *Server) pump(ctx context.Context, d *Decoder, deadline time.Time, fade bool) (int64, error) {
	return s.pumpLimit(ctx, d, deadline, math.MaxInt64, fade)
}

func (s *Server) pumpLimit(ctx context.Context, d *Decoder, deadline time.Time, limit int64, fade bool) (int64, error) {
	var delivered int64
	fadeSamples := s.Format.SampleRate * s.FadeMs / 1000
	var audioMissing int64 // muestras que faltaron y se cubrieron con silencio

	// Un cuadro de anticipación en el video: para saber que el que tengo en
	// la mano es el último y fundir el audio antes de mandarlo.
	cur, ok := d.NextFrame()
	first := true
	for ok && delivered < limit && time.Now().Before(deadline) && ctx.Err() == nil {
		var nxt []byte
		var more bool
		if delivered+1 < limit {
			nxt, more = s.nextOrHold(d, cur)
		}
		want := s.samplesFor(s.frame)
		pcm, got := d.ReadSamples(want)
		audioMissing += int64(want - got)
		if fade {
			if first {
				fadeIn(pcm, s.Format.Channels, fadeSamples)
				first = false
			}
			if !more {
				fadeOut(pcm, s.Format.Channels, want, fadeSamples)
			}
		}
		if err := s.emit(cur, pcm, false); err != nil {
			return delivered, err
		}
		delivered++
		cur, ok = nxt, more
	}
	// Unas muestras al final es el codificador de la fuente (AAC, AC-3) y
	// no vale un evento; más de un cuadro sí: el archivo trae menos audio
	// que video y el aire lo cubrió con silencio.
	if float64(audioMissing) > s.Format.SamplesPerFrame() {
		s.log("audio_corto", d.Clip.Name, fmt.Sprintf("faltaron %d ms de audio; se cubrieron con silencio", audioMissing*1000/int64(s.Format.SampleRate)))
	}
	return delivered, nil
}

// nextOrHold pide el siguiente cuadro; si el decodificador no lo tiene a
// tiempo, sostiene el anterior y lo anota. Nunca espera con el aire vacío.
func (s *Server) nextOrHold(d *Decoder, cur []byte) ([]byte, bool) {
	frameDur := time.Duration(s.Format.FrameDurationNs())
	select {
	case fr, ok := <-d.frames:
		return fr, ok
	case <-time.After(s.Lead + 2*frameDur):
		s.log("cuadro_sostenido", d.Clip.Name, "el decodificador no entregó a tiempo")
		return cur, true
	}
}

// samplesFor devuelve cuántas muestras corresponden al cuadro n. La
// fracción (800.8 a 59.94) se reparte sin acumular error: F0-02 se gana aquí.
func (s *Server) samplesFor(n int64) int {
	return int(s.Format.SamplesUpTo(n+1) - s.Format.SamplesUpTo(n))
}

func (s *Server) silence(n int) []byte { return make([]byte, n*s.Format.BytesPerSample()) }

// emit escribe un cuadro y su audio, a tiempo. Es el único punto que toca
// el encoder, y el único que avanza el reloj del aire.
func (s *Server) emit(frame, pcm []byte, held bool) error {
	target := s.start.Add(time.Duration(float64(s.frame) * s.Format.FrameDurationNs()))
	wait := time.Until(target.Add(-s.Lead))
	if wait > 0 {
		time.Sleep(wait)
	} else if wait < -time.Second {
		s.behind++
		if s.behind%60 == 1 {
			s.log("atraso", "", fmt.Sprintf("el aire va %s atrasado", (-wait).Round(time.Millisecond)))
		}
	} else {
		s.behind = 0
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
	return nil
}

// blackFrame solo existe para el primerísimo cuadro si el primer decodificador
// no dio nada. Si aparece en la salida, es un fallo y el analizador lo ve.
func (s *Server) blackFrame() []byte {
	b := make([]byte, s.Format.FrameBytes())
	for i := s.Format.Width * s.Format.Height; i < len(b); i++ {
		b[i] = 128
	}
	return b
}

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
