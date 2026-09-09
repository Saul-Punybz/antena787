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
func StartDecoder(parent context.Context, ffmpeg, ffprobe string, f Format, clip Clip, prerollFrames int) (*Decoder, error) {
	ctx, cancel := context.WithCancel(parent)
	d := &Decoder{Clip: clip, Format: f, cancel: cancel, frames: make(chan []byte, prerollFrames), vdone: make(chan error, 1)}

	if dur, err := probeDuration(ffprobe, clip.Path); err == nil {
		d.ExpectedFrames = int64(math.Round(dur * f.FPS()))
	}

	d.vcmd = exec.CommandContext(ctx, ffmpeg, "-nostdin", "-hide_banner", "-loglevel", "error",
		"-i", clip.Path, "-an", "-sn", "-dn",
		"-vf", videoFilter(f), "-f", "rawvideo", "pipe:1")
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
	d.acmd = exec.CommandContext(ctx, ffmpeg, "-nostdin", "-hide_banner", "-loglevel", "error",
		"-i", clip.Path, "-vn", "-sn", "-dn",
		"-af", audioFilter(f), "-f", "s16le", "-ar", strconv.Itoa(f.SampleRate), "-ac", strconv.Itoa(f.Channels), "pipe:1")
	d.acmd.Stderr = &d.aerr
	d.acmd.WaitDelay = 3 * time.Second
	aout, err := d.acmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	if err := d.vcmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("decodificador de video: %w", err)
	}
	if err := d.acmd.Start(); err != nil {
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
	out, err := exec.Command(ffprobe, "-v", "error", "-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1", path).Output()
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
}
