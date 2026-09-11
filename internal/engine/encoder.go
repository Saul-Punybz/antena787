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
//
// Lo que sigue a GainDB es lo que un multiplexor de transmisor exige y la
// persona escribe una vez (PRD §10): PID y número de programa fijos, PCR a
// tiempo, un códec de audio elegido. Nadie lo arma a mano: lo traduce
// internal/drivers/salida desde los parámetros de la salida configurada
// (T2). En cero, cada campo cae en el valor de fábrica de abajo, que es lo
// que ya emitía la F0.
type Output struct {
	Name string
	Kind string // "mpeg2-ts" | "h264-ts"
	// File es el archivo de salida: en el arnés de F0 es lo que se analiza.
	// Al aire puede ir vacío —el canal manda por UDP y no graba nada en el
	// disco—; grabar la salida de verdad, con su retención, es F2-36 (T7).
	File     string
	UDP      string // opcional: udp://host:puerto para el multiplexor
	VideoKbs int
	MuxKbs   int     // tasa constante del TS (solo mpeg2-ts)
	GainDB   float64 // ajuste de volumen relativo (0 = ninguno)

	PIDVideo int // PID del video dentro del TS
	PIDAudio int // PID del audio
	PIDPMT   int // PID donde viaja la tabla del programa
	Programa int // número de programa (el service_id del TS)
	TSID     int // identificador del transport stream
	PCRms    int // cada cuánto se repite el PCR, en ms: nunca más de 40
	// Audio es el códec del audio de esta salida: "mp2" (MPEG capa II, lo
	// que CAtv emite hoy) o "ac3". Vacío saca las dos pistas, que es lo que
	// medía la F0 cuando había que decidir cuál se quedaba.
	Audio string
	// TTL y PktSize son de la propia red: cuántos saltos vive el paquete
	// —hace falta para multicast (F2-114)— y de qué tamaño sale.
	TTL     int
	PktSize int
	// TCP es «127.0.0.1:puerto» de una escucha de ESTE proceso a la que
	// ffmpeg entrega el transport stream para que Go lo reparta a varios
	// clientes por HTTP (F2-115). Es el tercer destino posible, junto a UDP
	// y File: ffmpeg escribe a un sitio, no a tres protocolos distintos.
	//
	// Va aquí y no dentro de File porque un archivo se cierra y esto no: la
	// conexión vive lo que vive el encoder.
	TCP string
	// Accel es con qué se comprime el video de esta salida. En blanco sale
	// exactamente lo que salía antes de que el campo existiera: el
	// procesador. Lo pone App igual en todas las salidas del canal, y el
	// watchdog lo baja a software cuando la tarjeta falla dos veces en diez
	// minutos (F2-11).
	Accel Acelerador
}

// Valores de fábrica de una salida de TS. Los tres primeros son los que ya
// emitía la F0; los demás son lo que un multiplexor espera encontrar cuando
// nadie le dijo otra cosa, y se pueden cambiar todos (PRD §10: nunca fijos).
const (
	// PCRmsPorDefecto es cada cuánto se repite el PCR. El PRD pide 40 ms o
	// menos; 20 deja la mitad de margen y no cuesta nada.
	PCRmsPorDefecto = 20
	// PCRmsMaximo es lo que el PRD §10 permite como máximo.
	PCRmsMaximo = 40
	// PktSizePorDefecto son 7 paquetes de TS por datagrama UDP: 1316 bytes,
	// lo que cabe en una trama Ethernet sin fragmentar. Es lo mismo que pone
	// VLC.
	PktSizePorDefecto = 1316
	// PATPeriodo es cada cuánto se repiten PAT y PMT, en segundos.
	PATPeriodo = "0.1"
)

// Acelerador es con qué se comprime el video: la tarjeta de video o el
// procesador. Existe porque F2-11 manda relanzar el encoder colgado «con el
// mismo acelerador», y caer «por software» si falla dos veces en diez
// minutos: sin un nombre para eso, no hay nada que conservar ni a qué caer.
//
// Nunca se le enseña la clave a nadie (PRD §4.3): la pantalla usa Nombre().
type Acelerador string

const (
	// AcelAuto es el de fábrica. Hoy resuelve a software a propósito —ver
	// Resolver—: cambiar solo el encoder de una estación que ya emite, sin
	// haber probado que la tarjeta aguanta, es exactamente lo que no se
	// hace. Empieza a escoger de verdad cuando exista la prueba de diez
	// segundos por candidato (F2-15).
	AcelAuto Acelerador = "auto"
	// AcelSoftware es el procesador. Siempre está, siempre funciona, y es
	// el suelo al que cae el watchdog cuando la tarjeta deja de responder.
	AcelSoftware Acelerador = "software"
	// Las tarjetas. Cada una es un cambio de códec y nada más: el resto de
	// los argumentos —tasa, PIDs, mux— no depende de quién comprima.
	AcelNVENC        Acelerador = "nvenc"        // NVIDIA
	AcelQSV          Acelerador = "qsv"          // Intel Quick Sync
	AcelVAAPI        Acelerador = "vaapi"        // Linux, AMD e Intel
	AcelVideoToolbox Acelerador = "videotoolbox" // macOS
)

// Aceleradores son los que se le ofrecen a una persona, en ese orden.
//
// AcelVAAPI no está: la constante existe porque el valor está reservado, pero
// VAAPI no es un cambio de códec —pide subir el cuadro a la tarjeta con su
// propio filtro y un dispositivo abierto— y eso se escribe con la máquina
// delante, no de memoria. Ofrecerlo antes sería ofrecer algo que comprime por
// procesador diciendo que usa la tarjeta. Entra cuando esté escrito.
func Aceleradores() []Acelerador {
	return []Acelerador{AcelAuto, AcelSoftware, AcelNVENC, AcelQSV, AcelVideoToolbox}
}

// Nombre es cómo se llama en pantalla. Sin jerga y sin la clave (F1-56).
func (a Acelerador) Nombre() string {
	switch a {
	case AcelSoftware:
		return "El procesador"
	case AcelNVENC:
		return "La tarjeta NVIDIA"
	case AcelQSV:
		return "El video del procesador Intel"
	case AcelVAAPI:
		return "La tarjeta de video (Linux)"
	case AcelVideoToolbox:
		return "La tarjeta de video (Mac)"
	default:
		return "Automático"
	}
}

// Explicacion es la línea de debajo, la que dice qué pasa si se escoge.
func (a Acelerador) Explicacion() string {
	switch a {
	case AcelSoftware:
		return "Comprime con el procesador. Es el más lento y el que nunca falla; es a donde vuelve el canal solo si la tarjeta deja de responder."
	case AcelNVENC, AcelQSV, AcelVAAPI, AcelVideoToolbox:
		return "Comprime con la tarjeta y le quita casi todo el trabajo al procesador. Si deja de responder dos veces en diez minutos, el canal sigue emitiendo con el procesador y te avisa."
	default:
		return "Usa el procesador. Escoger la tarjeta sola llega cuando el sistema pueda probarla antes de confiarle el aire."
	}
}

// Resolver dice con qué se comprime de verdad. Es el único sitio donde
// 'auto' y el vacío se convierten en algo concreto, para que el watchdog y
// la pantalla no lo adivinen cada uno por su lado.
func (a Acelerador) Resolver() Acelerador {
	switch a {
	case "", AcelAuto:
		return AcelSoftware
	default:
		return a
	}
}

// codecVideo es el códec de este acelerador para un códec base. base es
// "h264" o "mpeg2video"; el vacío y lo desconocido devuelven el de software,
// que es lo que emitía la F0.
func (a Acelerador) codecVideo(base string) string {
	if base == "mpeg2video" {
		// MPEG-2 comprimido por tarjeta solo existe en Intel. Todo lo demás
		// —incluida la NVIDIA— lo hace con el procesador, así que decirle
		// otra cosa a ffmpeg sería mentirle.
		if a.Resolver() == AcelQSV {
			return "mpeg2_qsv"
		}
		return "mpeg2video"
	}
	switch a.Resolver() {
	case AcelNVENC:
		return "h264_nvenc"
	case AcelQSV:
		return "h264_qsv"
	case AcelVideoToolbox:
		return "h264_videotoolbox"
	case AcelVAAPI:
		// VAAPI no es un cambio de códec: pide subir el cuadro a la tarjeta
		// con su propio filtro y un dispositivo abierto. Se escribe con la
		// máquina delante, no de memoria (invariante 3 de la obra).
		return "libx264"
	default:
		return "libx264"
	}
}

// Disponible dice si este ffmpeg trae el códec de este acelerador. Es una
// pregunta al binario, no una suposición sobre la máquina: un ffmpeg sin
// NVENC compilado no lo tiene aunque la tarjeta esté puesta.
func (a Acelerador) Disponible(ffmpeg string) bool {
	if a.Resolver() == AcelSoftware {
		return true
	}
	// El códec de verdad, no el que se acaba usando: si se preguntara por lo
	// que devuelve codecVideo, VAAPI —que hoy cae en libx264— diría que sí lo
	// hay en cualquier máquina, y quien lo escogiera comprimiría por
	// procesador creyendo que usa la tarjeta.
	var codec string
	switch a.Resolver() {
	case AcelNVENC:
		codec = "h264_nvenc"
	case AcelQSV:
		codec = "h264_qsv"
	case AcelVAAPI:
		codec = "h264_vaapi"
	case AcelVideoToolbox:
		codec = "h264_videotoolbox"
	default:
		return false
	}
	out, err := exec.Command(ffmpeg, "-hide_banner", "-loglevel", "error", "-encoders").Output()
	if err != nil {
		return false
	}
	return bytes.Contains(out, []byte(codec))
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
		switch o.Kind {
		case "mpeg2-ts":
			args = append(args, o.argsMPEG2TS()...)
		case "h264-ts":
			args = append(args, o.argsH264TS()...)
		default:
			panic("salida desconocida: " + o.Kind)
		}
	}
	return args
}

// argsMPEG2TS es lo que ATSC 1.0 transmite y lo que un multiplexor exige
// (PRD §10): MPEG-2 a tasa constante rellenada con paquetes nulos, audio
// MPEG capa II o AC-3, y un transport stream con los PID, el número de
// programa y el PCR que la persona escribió una vez.
func (o Output) argsMPEG2TS() []string {
	vol := fmt.Sprintf("volume=%.2fdB", o.GainDB)
	kb := o.VideoKbs
	// Un códec de audio, el que se eligió; los dos cuando nadie eligió, que
	// es como la F0 medía cuál se quedaba.
	codecs := []string{o.Audio}
	if o.Audio == "" {
		codecs = []string{"mp2", "ac3"}
	}

	args := []string{"-map", "0:v"}
	for range codecs {
		args = append(args, "-map", "1:a")
	}
	for i := range codecs {
		args = append(args, fmt.Sprintf("-filter:a:%d", i), vol)
	}
	args = append(args,
		"-c:v", o.Accel.codecVideo("mpeg2video"), "-pix_fmt", "yuv420p", "-g", "30", "-bf", "2",
		"-b:v", fmt.Sprintf("%dk", kb), "-minrate", fmt.Sprintf("%dk", kb), "-maxrate", fmt.Sprintf("%dk", kb),
		"-bufsize", fmt.Sprintf("%dk", kb/2))
	for i, c := range codecs {
		args = append(args, fmt.Sprintf("-c:a:%d", i), c, fmt.Sprintf("-b:a:%d", i), "192k")
	}
	args = append(args, "-max_muxing_queue_size", "1024")
	args = append(args, o.streamIDs(len(codecs))...)

	switch {
	case o.UDP != "" && o.File != "":
		// El mismo TS a la red y al disco. onfail=ignore en la rama de red:
		// que nadie escuche el UDP no puede parar la grabación.
		mux := o.especDelTee()
		args = append(args, "-f", "tee",
			fmt.Sprintf("[%s:onfail=ignore]%s|[%s]%s", mux, o.urlUDP(), mux, o.File))
	case o.UDP != "":
		args = append(args, o.flagsDelMux()...)
		args = append(args, o.urlUDP())
	default:
		args = append(args, o.flagsDelMux()...)
		args = append(args, o.File)
	}
	return args
}

// argsH264TS es la salida de internet: H.264 y AAC. Las de verdad —RTMP, HLS,
// SRT, y el http-ts de paridad con VLC— son T7; esto es lo que la F0 usaba
// para medir dos salidas con volúmenes distintos a la vez.
func (o Output) argsH264TS() []string {
	return []string{
		"-map", "0:v", "-map", "1:a",
		"-filter:a:0", fmt.Sprintf("volume=%.2fdB", o.GainDB),
		"-c:v", o.Accel.codecVideo("h264"), "-preset", "veryfast", "-pix_fmt", "yuv420p", "-g", "60",
		"-b:v", fmt.Sprintf("%dk", o.VideoKbs), "-maxrate", fmt.Sprintf("%dk", o.VideoKbs),
		"-bufsize", fmt.Sprintf("%dk", o.VideoKbs*2),
		"-c:a", "aac", "-b:a", "128k",
		"-max_muxing_queue_size", "1024",
		"-f", "mpegts", o.File,
	}
}

// opcionesDelMux son las opciones del multiplexor de TS, en el mismo orden
// siempre: es lo que en VLC se escribe como pid-video, pid-audio, pid-pmt,
// tsid, program y pcr (docs/VLC-PARIDAD.md, fila «Opciones del mux TS»). Lo
// que vale cero no se pone: ffmpeg pone entonces lo que trae de fábrica.
func (o Output) opcionesDelMux() [][2]string {
	pcr := o.PCRms
	if pcr <= 0 {
		pcr = PCRmsPorDefecto
	}
	opts := [][2]string{
		{"muxrate", strconv.Itoa(o.MuxKbs * 1000)},
		{"pcr_period", strconv.Itoa(pcr)},
		{"pat_period", PATPeriodo},
	}
	if o.PIDPMT > 0 {
		opts = append(opts, [2]string{"mpegts_pmt_start_pid", strconv.Itoa(o.PIDPMT)})
	}
	if o.PIDVideo > 0 {
		opts = append(opts, [2]string{"mpegts_start_pid", strconv.Itoa(o.PIDVideo)})
	}
	if o.Programa > 0 {
		opts = append(opts, [2]string{"mpegts_service_id", strconv.Itoa(o.Programa)})
	}
	if o.TSID > 0 {
		opts = append(opts, [2]string{"mpegts_transport_stream_id", strconv.Itoa(o.TSID)})
	}
	return opts
}

// flagsDelMux son esas mismas opciones como banderas de una salida suelta.
func (o Output) flagsDelMux() []string {
	args := []string{"-f", "mpegts"}
	for _, kv := range o.opcionesDelMux() {
		args = append(args, "-"+kv[0], kv[1])
	}
	return args
}

// especDelTee son esas mismas opciones como las quiere el multiplexor tee,
// que las lleva dentro de los corchetes de cada rama.
func (o Output) especDelTee() string {
	spec := "f=mpegts"
	for _, kv := range o.opcionesDelMux() {
		spec += ":" + kv[0] + "=" + kv[1]
	}
	return spec
}

// streamIDs fija el PID de cada flujo. mpegts_start_pid solo mueve el
// primero y va numerando; VLC deja escribir el del video y el del audio por
// separado, así que aquí también (docs/VLC-PARIDAD.md).
func (o Output) streamIDs(audios int) []string {
	var args []string
	if o.PIDVideo > 0 {
		args = append(args, "-streamid", "0:"+strconv.Itoa(o.PIDVideo))
	}
	if o.PIDAudio > 0 {
		for i := 0; i < audios; i++ {
			args = append(args, "-streamid", fmt.Sprintf("%d:%d", i+1, o.PIDAudio+i))
		}
	}
	return args
}

// urlUDP es el destino con lo que la red necesita: el tamaño del datagrama y,
// cuando va a un grupo multicast, los saltos que vive el paquete (F2-114). Si
// alguien ya escribió sus propias opciones en la dirección, se respetan.
func (o Output) urlUDP() string {
	if o.UDP == "" || strings.Contains(o.UDP, "?") {
		return o.UDP
	}
	pkt := o.PktSize
	if pkt <= 0 {
		pkt = PktSizePorDefecto
	}
	url := fmt.Sprintf("%s?pkt_size=%d", o.UDP, pkt)
	if o.TTL > 0 {
		url += fmt.Sprintf("&ttl=%d", o.TTL)
	}
	return url
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

// Matar mata el encoder sin esperar a que termine bien. Es lo contrario de
// Finish y existe para un solo caso: un ffmpeg colgado, que no tiene colas que
// vaciar porque no está haciendo nada, y al que Finish le regalaría un minuto
// de aire congelado. Al morir el proceso, el sistema cierra sus conexiones y
// la escritura que estaba bloqueada devuelve error, que es lo que hace que el
// motor vuelva a entrar con un encoder nuevo (F2-11).
//
// Se puede llamar dos veces y desde otra goroutine: matar a un muerto no hace
// nada.
func (e *Encoder) Matar() {
	if e.cancel != nil {
		e.cancel()
	}
	if e.Cmd != nil && e.Cmd.Process != nil {
		_ = e.Cmd.Process.Kill()
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
