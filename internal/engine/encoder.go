package engine

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Output describe una salida del encoder persistente. En F0 hay dos clases:
// la que recibe el multiplexor de CAtv (MPEG-2 CBR por UDP y a archivo) y
// una de internet (H.264) con otro volumen, para probar dos a la vez.
type Output struct {
	Name     string
	Kind     string // "mpeg2-ts" | "h264-ts"
	File     string // archivo de salida (siempre; es lo que se analiza)
	UDP      string // opcional: udp://host:puerto para el multiplexor
	VideoKbs int
	MuxKbs   int     // tasa constante del TS (solo mpeg2-ts)
	GainDB   float64 // ajuste de volumen relativo (0 = ninguno)
}

// Encoder es el único ffmpeg de larga vida: recibe cuadros crudos y PCM por
// dos conexiones TCP locales y produce todas las salidas a la vez.
type Encoder struct {
	Format  Format
	Outputs []Output
	Cmd     *exec.Cmd
	Stderr  bytes.Buffer

	vconn, aconn net.Conn
	vw, aw       *bufio.Writer
	cancel       context.CancelFunc
	done         chan error

	// ffmpeg abre sus entradas en orden y no abre la segunda hasta haber
	// leído algo de la primera. Así que el audio se acepta aparte y lo que
	// llegue antes se guarda hasta que conecte.
	amu      sync.Mutex
	apending []byte
	aerr     error
	aready   chan struct{}
}

// StartEncoder abre los dos puertos, lanza ffmpeg y espera que conecte.
func StartEncoder(parent context.Context, ffmpeg string, f Format, outs []Output) (*Encoder, error) {
	lv, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	la, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		lv.Close()
		return nil, err
	}
	defer lv.Close()

	// El encoder NO se mata con el contexto: cuando el servidor deja de
	// escribir, Finish cierra las entradas y ffmpeg termina solo, con sus
	// archivos bien cerrados. Matarlo a mitad deja un TS truncado.
	_, cancel := context.WithCancel(parent)
	e := &Encoder{Format: f, Outputs: outs, cancel: cancel, done: make(chan error, 1)}

	args := []string{"-nostdin", "-hide_banner", "-loglevel", "warning", "-nostats",
		"-probesize", "32", "-analyzeduration", "0",
		"-f", "rawvideo", "-pix_fmt", "yuv420p", "-s", f.SizeString(), "-r", f.FPSString(),
		"-thread_queue_size", "2048", "-i", "tcp://" + lv.Addr().String(),
		"-probesize", "32", "-analyzeduration", "0",
		"-f", "s16le", "-ar", strconv.Itoa(f.SampleRate), "-ac", strconv.Itoa(f.Channels),
		"-thread_queue_size", "2048", "-i", "tcp://" + la.Addr().String(),
	}
	args = append(args, e.outputArgs()...)
	if os.Getenv("ANTENA_DEBUG") != "" {
		fmt.Fprintln(os.Stderr, "encoder:", ffmpeg, strings.Join(args, " "))
	}
	e.Cmd = exec.Command(ffmpeg, args...)
	e.Cmd.Stderr = &e.Stderr
	if err := e.Cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("encoder: %w", err)
	}
	go func() { e.done <- e.Cmd.Wait() }()

	accept := func(l net.Listener) (net.Conn, error) {
		if tl, ok := l.(*net.TCPListener); ok {
			tl.SetDeadline(time.Now().Add(15 * time.Second))
		}
		return l.Accept()
	}
	if e.vconn, err = accept(lv); err != nil {
		cancel()
		return nil, fmt.Errorf("el encoder no conectó al video: %w\n%s", err, e.Stderr.String())
	}
	e.vw = bufio.NewWriterSize(e.vconn, 4*f.FrameBytes())
	e.aready = make(chan struct{})
	la2 := la
	go func() {
		defer close(e.aready)
		c, err := accept(la2)
		la2.Close()
		e.amu.Lock()
		defer e.amu.Unlock()
		if err != nil {
			e.aerr = fmt.Errorf("el encoder no conectó al audio: %w\n%s", err, e.Stderr.String())
			return
		}
		e.aconn = c
		e.aw = bufio.NewWriterSize(c, 1<<18)
		if len(e.apending) > 0 {
			e.aw.Write(e.apending)
			e.aw.Flush()
			e.apending = nil
		}
	}()
	return e, nil
}

// outputArgs arma una salida por Output. Sin filter_complex a propósito:
// un solo grafo con audio y video hace que ffmpeg pida las entradas por
// marca de tiempo y se quede esperando al audio con el video bloqueado
// (medido: se traba al tercer cuadro). Con filtros por salida no pasa.
func (e *Encoder) outputArgs() []string {
	var args []string
	for _, o := range e.Outputs {
		vol := fmt.Sprintf("volume=%.2fdB", o.GainDB)
		switch o.Kind {
		case "mpeg2-ts":
			// Lo que ATSC 1.0 transmite y lo que un multiplexor exige (PRD §10):
			// MPEG-2 a tasa constante, audio MPEG capa II y AC-3, TS con
			// muxrate fijo y PCR frecuente.
			kb := o.VideoKbs
			args = append(args,
				"-map", "0:v", "-map", "1:a", "-map", "1:a",
				"-filter:a:0", vol, "-filter:a:1", vol,
				"-c:v", "mpeg2video", "-pix_fmt", "yuv420p", "-g", "30", "-bf", "2",
				"-b:v", fmt.Sprintf("%dk", kb), "-minrate", fmt.Sprintf("%dk", kb), "-maxrate", fmt.Sprintf("%dk", kb),
				"-bufsize", fmt.Sprintf("%dk", kb/2),
				"-c:a:0", "mp2", "-b:a:0", "192k", "-c:a:1", "ac3", "-b:a:1", "192k",
				"-max_muxing_queue_size", "1024")
			mux := fmt.Sprintf("f=mpegts:muxrate=%d:pcr_period=20:pat_period=0.1", o.MuxKbs*1000)
			if o.UDP != "" {
				args = append(args, "-f", "tee", fmt.Sprintf("[%s:onfail=ignore]%s?pkt_size=1316|[%s]%s", mux, o.UDP, mux, o.File))
			} else {
				args = append(args, "-f", "mpegts", "-muxrate", strconv.Itoa(o.MuxKbs*1000), "-pcr_period", "20", "-pat_period", "0.1", o.File)
			}
		case "h264-ts":
			args = append(args,
				"-map", "0:v", "-map", "1:a",
				"-filter:a:0", vol,
				"-c:v", "libx264", "-preset", "veryfast", "-pix_fmt", "yuv420p", "-g", "60",
				"-b:v", fmt.Sprintf("%dk", o.VideoKbs), "-maxrate", fmt.Sprintf("%dk", o.VideoKbs), "-bufsize", fmt.Sprintf("%dk", o.VideoKbs*2),
				"-c:a", "aac", "-b:a", "128k",
				"-max_muxing_queue_size", "1024",
				"-f", "mpegts", o.File)
		default:
			panic("salida desconocida: " + o.Kind)
		}
	}
	return args
}

// WriteFrame manda un cuadro yuv420p al encoder.
func (e *Encoder) WriteFrame(fr []byte) error {
	_, err := e.vw.Write(fr)
	if err == nil {
		err = e.vw.Flush()
	}
	return err
}

// WriteAudio manda PCM intercalado al encoder (o lo guarda hasta que conecte).
func (e *Encoder) WriteAudio(pcm []byte) error {
	e.amu.Lock()
	defer e.amu.Unlock()
	if e.aerr != nil {
		return e.aerr
	}
	if e.aw == nil {
		e.apending = append(e.apending, pcm...)
		if len(e.apending) > 64<<20 {
			return fmt.Errorf("el encoder nunca abrió la entrada de audio")
		}
		return nil
	}
	_, err := e.aw.Write(pcm)
	if err == nil {
		err = e.aw.Flush()
	}
	return err
}

// Finish cierra las entradas, deja que ffmpeg vacíe sus colas y termine.
func (e *Encoder) Finish() error {
	e.vw.Flush()
	e.vconn.Close()
	<-e.aready
	e.amu.Lock()
	if e.aw != nil {
		e.aw.Flush()
		e.aconn.Close()
	}
	e.amu.Unlock()
	select {
	case err := <-e.done:
		if err != nil {
			return fmt.Errorf("encoder terminó mal: %w\n%s", err, e.Stderr.String())
		}
		return nil
	case <-time.After(60 * time.Second):
		e.cancel()
		if e.Cmd.Process != nil {
			e.Cmd.Process.Kill()
		}
		return fmt.Errorf("encoder no terminó en 60 s")
	}
}

// Done avisa si el encoder murió por su cuenta (eso es un incidente).
func (e *Encoder) Done() <-chan error { return e.done }

// PID es el del proceso, para medir CPU y RAM.
func (e *Encoder) PID() int {
	if e.Cmd.Process == nil {
		return 0
	}
	return e.Cmd.Process.Pid
}
