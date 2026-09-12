package engine

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Clip es un archivo que va a salir al aire.
type Clip struct {
	Path string
	Name string
	// SeekMs es por dónde empieza: 0 es desde el principio. Lo usa el
	// arranque a mitad de programa —el servicio se reinició en el minuto
	// 07:32 de un bloque— para abrir el archivo donde toca y no volver a
	// empezarlo (F2-13).
	SeekMs int64
	// Ref es de quien pone el clip; el motor no la mira. internal/app guarda
	// ahí el id del plan_item, para cerrar el as-run cuando el clip sale.
	Ref int64
	// EnVivo dice que esto no es un archivo, es una señal que está pasando
	// ahora en otro sitio (F2-116). Cambia tres cosas y las tres importan:
	//
	//   - **No se le mide la duración.** Un vivo no tiene final, y preguntarlo
	//     cuelga o devuelve cero.
	//   - **No se entra por el medio.** Entrar por el minuto 7 de algo que
	//     está pasando ahora no significa nada; se entra donde va.
	//   - **Se le dice a ffmpeg que insista.** Un archivo que se corta está
	//     roto; una señal que se corta casi siempre vuelve.
	EnVivo bool
}

// Decoder es la pareja de procesos ffmpeg que convierten un clip a cuadros
// yuv420p y PCM s16le en el formato de casa. Uno por clip; se abre con
// pre-roll antes de que el anterior termine, y muere al terminar.
type Decoder struct {
	Clip           Clip
	Format         Format
	ExpectedFrames int64 // según la duración que reporta ffprobe

	frames chan []byte
	audio  *pcmReader
	vcmd   *exec.Cmd
	acmd   *exec.Cmd
	verr   bytes.Buffer
	aerr   bytes.Buffer
	cancel context.CancelFunc
	wg     sync.WaitGroup
	vdone  chan error
}

// videoFilter es el conformado de geometría y tasa de cuadros: cabe siempre,
// pillarbox si hace falta, desentrelaza solo lo entrelazado, y sale a la
// tasa de casa aunque la fuente sea VFR o PAL.
func videoFilter(f Format) string {
	return fmt.Sprintf(
		"bwdif=mode=send_field:parity=auto:deint=interlaced,"+
			"scale=%d:%d:force_original_aspect_ratio=decrease:flags=bicubic,"+
			"pad=%d:%d:(ow-iw)/2:(oh-ih)/2,fps=%s,format=yuv420p",
		f.Width, f.Height, f.Width, f.Height, f.FPSString())
}

// audioFilter remuestrea a la tasa de casa y reduce o sube a estéreo.
func audioFilter(f Format) string {
	return fmt.Sprintf("aresample=%d:async=1:first_pts=0,aformat=sample_fmts=s16:channel_layouts=stereo", f.SampleRate)
}

// StartDecoder lanza los dos procesos y empieza a llenar el pre-roll.
//
// **Son dos ffmpeg por clip a propósito, y juntarlos empeora las cosas.** Cada
// uno escribe su salida cruda a `pipe:1`, y un proceso solo tiene un stdout;
// sacar video y audio del mismo ffmpeg pide dos tuberías de salida, que en
// Windows es un lío. Pero la razón de verdad no es esa: **se midió**, el 11 de
// septiembre de 2026, sobre un HEVC 1080p real y 30 segundos de trabajo
// (docs/investigacion/MEDICION-CPU-2026-09-11.md):
//
//	dos procesos, como está aquí     1.58 s de reloj · 11.98 s de CPU
//	uno solo con dos salidas         3.64 s de reloj · 14.65 s de CPU
//
// Los dos corren en núcleos distintos; uno solo serializa el trabajo. Y el
// audio sale gratis: solo el video cuesta 11.91 s, o sea que el segundo
// proceso no se nota. Si a alguien le parece un desperdicio obvio —le pareció
// a quien escribe esto—, que mida antes de tocarlo.
func StartDecoder(parent context.Context, ffmpeg, ffprobe string, f Format, clip Clip, prerollFrames int) (*Decoder, error) {
	ctx, cancel := context.WithCancel(parent)
	d := &Decoder{Clip: clip, Format: f, cancel: cancel, frames: make(chan []byte, prerollFrames), vdone: make(chan error, 1)}

	// A un vivo no se le pregunta cuánto dura: no tiene final, y ffprobe se
	// queda esperando a que lo tenga.
	if dur, err := probeDuration(ffprobe, clip.Path); err == nil && !clip.EnVivo {
		dur -= float64(clip.SeekMs) / 1000 // lo que queda desde el seek
		if dur < 0 {
			dur = 0
		}
		d.ExpectedFrames = int64(math.Round(dur * f.FPS()))
	}

	// El seek va antes de -i: así ffmpeg salta por índice y no decodifica lo
	// que no va a salir. Un archivo sin índice cae solo en el modo lento.
	seek := seekArgs(clip.SeekMs)
	if clip.EnVivo {
		// Entrar por el medio de algo que está pasando ahora no significa
		// nada, y los argumentos de insistir van delante de -i como el seek.
		seek = argsDeVivo(clip.Path)
	}

	d.vcmd = Comando(ctx, ffmpeg, append(append([]string{
		"-nostdin", "-hide_banner", "-loglevel", "error"}, seek...),
		"-i", clip.Path, "-an", "-sn", "-dn",
		"-vf", videoFilter(f), "-f", "rawvideo", "pipe:1")...)
	d.vcmd.Stderr = &d.verr
	// En Windows, Wait se quedaba esperando a que ffmpeg soltara sus tuberías
	// después de matarlo (issue #14): con WaitDelay, Wait cierra las tuberías
	// él mismo y sigue.
	d.vcmd.WaitDelay = 3 * time.Second
	vout, err := d.vcmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	d.acmd = Comando(ctx, ffmpeg, append(append([]string{
		"-nostdin", "-hide_banner", "-loglevel", "error"}, seek...),
		"-i", clip.Path, "-vn", "-sn", "-dn",
		"-af", audioFilter(f), "-f", "s16le", "-ar", strconv.Itoa(f.SampleRate), "-ac", strconv.Itoa(f.Channels), "pipe:1")...)
	d.acmd.Stderr = &d.aerr
	d.acmd.WaitDelay = 3 * time.Second
	aout, err := d.acmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	if err := Arrancar(d.vcmd); err != nil {
		cancel()
		return nil, fmt.Errorf("decodificador de video: %w", err)
	}
	if err := Arrancar(d.acmd); err != nil {
		cancel()
		d.vcmd.Wait()
		return nil, fmt.Errorf("decodificador de audio: %w", err)
	}

	d.audio = newPCMReader(bufio.NewReaderSize(aout, 1<<20), f.BytesPerSample())

	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		defer close(d.frames)
		r := bufio.NewReaderSize(vout, 4<<20)
		n := f.FrameBytes()
		for {
			buf := make([]byte, n)
			if _, err := io.ReadFull(r, buf); err != nil {
				if err != io.EOF && err != io.ErrUnexpectedEOF {
					d.vdone <- err
				} else {
					d.vdone <- nil
				}
				return
			}
			select {
			case d.frames <- buf:
			case <-ctx.Done():
				return
			}
		}
	}()
	return d, nil
}

// seekArgs es el salto de entrada, vacío cuando el clip sale desde el
// principio (que es lo normal).
func seekArgs(ms int64) []string {
	if ms <= 0 {
		return nil
	}
	return []string{"-ss", fmt.Sprintf("%.3f", float64(ms)/1000)}
}

// NextFrame entrega el siguiente cuadro o false cuando el clip terminó.
func (d *Decoder) NextFrame() ([]byte, bool) {
	fr, ok := <-d.frames
	return fr, ok
}

// ReadSamples devuelve hasta n muestras (intercaladas). Devuelve menos solo
// cuando el audio del clip se acabó: el servidor rellena con silencio.
func (d *Decoder) ReadSamples(n int) ([]byte, int) { return d.audio.read(n) }

// Close mata lo que quede y recoge los procesos. Devuelve lo que ffmpeg
// escribió en stderr, que es el "por qué" de un clip corrupto.
func (d *Decoder) Close() (stderr string) {
	d.cancel()
	// Matar a mano además de cancelar el contexto: en Windows el proceso
	// seguía vivo después de la cancelación (issue #14).
	for _, c := range []*exec.Cmd{d.vcmd, d.acmd} {
		if c.Process != nil {
			_ = c.Process.Kill()
		}
	}
	d.vcmd.Wait()
	d.acmd.Wait()
	d.wg.Wait()
	s := strings.TrimSpace(d.verr.String() + "\n" + d.aerr.String())
	return s
}

// pcmReader lee PCM intercalado y sirve n muestras exactas.
type pcmReader struct {
	r   *bufio.Reader
	bps int
	eof bool
}

func newPCMReader(r *bufio.Reader, bytesPerSample int) *pcmReader {
	return &pcmReader{r: r, bps: bytesPerSample}
}

func (p *pcmReader) read(n int) ([]byte, int) {
	buf := make([]byte, n*p.bps)
	if p.eof {
		return buf, 0
	}
	got, err := io.ReadFull(p.r, buf)
	if err != nil {
		p.eof = true
	}
	return buf, got / p.bps
}

func probeDuration(ffprobe, path string) (float64, error) {
	out, err := Comando(nil, ffprobe, "-v", "error", "-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1", path).Output()
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
}

// argsDeVivo son los argumentos que se le ponen a ffmpeg **antes** de -i
// cuando lo que abre es una señal y no un archivo.
//
// Los números salen de lo que hace la gente que ya lo resolvió
// (docs/investigacion/ENTRADAS-POR-URL-COMPARADAS-2026-09-11.md): nginx-rtmp
// reintenta cada 2 segundos para lo que va a buscar, sin límite de intentos.
// Aquí el reintento de ffmpeg cubre el corte corto —el que no llega a notarse
// al aire— y el motor cubre el largo volviendo a abrir el decodificador; entre
// los dos no hay hueco.
func argsDeVivo(ruta string) []string {
	// rw_timeout vale para todo: sin él, una señal que acepta la conexión y
	// después se calla deja al decodificador colgado para siempre, y desde
	// fuera parece que está funcionando.
	args := []string{"-rw_timeout", "8000000"} // 8 s sin un byte y se rinde
	if esHTTP(ruta) {
		// Estos solo los entiende el protocolo http de ffmpeg. Ponérselos a
		// un srt:// no rompe nada, pero tampoco hace nada: mejor no mentir
		// sobre lo que está puesto.
		args = append(args,
			"-reconnect", "1",
			"-reconnect_streamed", "1",
			"-reconnect_on_network_error", "1",
			"-reconnect_delay_max", "2")
	}
	return args
}

func esHTTP(ruta string) bool {
	r := strings.ToLower(ruta)
	return strings.HasPrefix(r, "http://") || strings.HasPrefix(r, "https://")
}
