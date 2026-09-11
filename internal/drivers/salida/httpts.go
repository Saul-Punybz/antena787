package salida

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"antena787/internal/engine"
	"antena787/internal/model"
	"antena787/internal/ts"
)

// httpts.go sirve la misma señal del canal por HTTP para que otro programa
// tire de ella: un VLC remoto, MistServer, o la ventana de monitor que viene
// después (F2-115, F2-117). Es el `http{mux=ts,dst=:8080/}` de VLC
// (docs/VLC-PARIDAD.md, fila «HTTP TS»).
//
// Quien sirve el HTTP es Go, no ffmpeg: el protocolo http de ffmpeg acepta un
// solo cliente a la vez y el criterio pide dos a la vez sin que uno afecte al
// otro. Así que ffmpeg entrega el transport stream por una conexión de esta
// misma máquina —el mismo truco que el encoder ya usa para RECIBIR video y
// audio, ver engine.StartEncoder— y este archivo lo reparte a N clientes,
// cada uno en su goroutine.
//
// Las tres cosas que aquí se cuidan, porque son las que tumban el aire:
//
//  1. Un cliente lento no frena el aire. Si no lee, su espera se llena, se le
//     corta a él y los demás siguen. La lectura de ffmpeg nunca se bloquea.
//  2. Sin ningún cliente se sigue leyendo y se tira lo leído. Si Go dejara de
//     leer, el búfer de ffmpeg se llena y se cuelga el encoder ENTERO: el
//     canal se queda sin aire por las demás salidas también, que es justo lo
//     que el vigilante de F2-11 acaba de aprender a detectar.
//  3. Un cliente que llega a mitad tiene que poder sincronizar. Un transport
//     stream se lee en paquetes de 188 bytes que empiezan por 0x47, y sin la
//     tabla de programas (PAT) y la del programa (PMT) no se entiende nada.
//     Por eso a cada cliente nuevo se le empieza a mandar en una PAT, no en
//     mitad de un paquete.

// ParamsHTTPTS es lo que se guarda en `output.parametros` de una salida que se
// sirve por HTTP. Las mismas etiquetas que `ParamsHTTPTS` de
// web/src/lib/tipos.ts. Todo tiene valor de fábrica: un cero o un vacío
// quieren decir «esto no lo escribí».
type ParamsHTTPTS struct {
	// Puerto es dónde escucha esta máquina, p. ej. 8080.
	Puerto int `json:"puerto"`
	// Ruta es cómo se pide, con la barra delante: «/stream.ts».
	Ruta string `json:"ruta"`
	// Codec es «mpeg2» para un VLC remoto o un multiplexor, y «h264» para que
	// lo pueda pintar un navegador: ningún navegador sabe decodificar MPEG-2.
	Codec           string `json:"codec"`
	BitrateVideoKbs int    `json:"bitrate_video_kbs"`
	BitrateMuxKbs   int    `json:"bitrate_mux_kbs"`
	Audio           string `json:"audio"`
	// BitrateAudioKbs es el bitrate del sonido, de 64 a 384. Vacío = 192, que
	// es el valor de fábrica y el mismo que trae MistServer. CAtv emite a 128.
	BitrateAudioKbs int `json:"bitrate_audio_kbs"`
}

// Valores de fábrica de una salida servida por HTTP.
const (
	// PuertoHTTPTSPorDefecto es el de la fila «HTTP TS» de VLC-PARIDAD.
	PuertoHTTPTSPorDefecto = 8080
	// RutaHTTPTSPorDefecto es la que espera quien ya usaba VLC.
	RutaHTTPTSPorDefecto  = "/stream.ts"
	CodecHTTPTSPorDefecto = "mpeg2"
	// Las tasas de la copia para la web son más bajas que las del
	// transmisor: al otro lado hay un navegador o un programa mirando, no una
	// emisora, y H.264 rinde bastante más que MPEG-2 con el mismo dinero.
	VideoKbsWebPorDefecto = 2500
	MuxKbsWebPorDefecto   = 3200
	// AudioWebPorDefecto es lo que ffmpeg saca en una salida h264-ts.
	AudioWebPorDefecto = "aac"
)

// Cómo se reparte, medido en lo que le cuesta al aire si sale mal.
const (
	// BloquesEnEspera es cuántos trozos de transport stream se le guardan a
	// un cliente que va lento antes de cortarle. Con trozos de hasta 32 KB
	// son unos 2 MB por cliente: medio segundo de señal a las tasas de la
	// web. Quien no lea eso en medio segundo no está mirando.
	BloquesEnEspera = 64
	// LeeDeUnaVez es lo que se le pide a la conexión de ffmpeg cada vuelta.
	LeeDeUnaVez = 32 * 1024
	// PlazoDeEntrega es lo que se espera a que ffmpeg conecte a entregar el
	// transport stream. El encoder se da 15 s para conectar sus entradas;
	// esto va después, así que se le da el doble.
	PlazoDeEntrega = 30 * time.Second
	// PlazoDeCabeceras es lo que se le da a un cliente para pedir lo que
	// quiere. Lo que sale después no tiene plazo: es una señal, no dura.
	PlazoDeCabeceras = 10 * time.Second
)

// httpts es el driver: los parámetros ya leídos y validados.
type httpts struct {
	salida model.Output
	reg    Registro
	p      ParamsHTTPTS
}

func nuevoHTTPTS(o model.Output, reg Registro) (Driver, error) {
	p, err := leerParamsHTTPTS(o.Params)
	if err != nil {
		return nil, err
	}
	d := &httpts{salida: o, reg: reg, p: p}
	if err := d.valida(); err != nil {
		return nil, err
	}
	return d, nil
}

// leerParamsHTTPTS lee el JSON de la salida y rellena lo que falte.
func leerParamsHTTPTS(raw string) (ParamsHTTPTS, error) {
	p := ParamsHTTPTS{}
	if s := strings.TrimSpace(raw); s != "" && s != "{}" {
		if err := json.Unmarshal([]byte(s), &p); err != nil {
			return p, fmt.Errorf("no entiendo lo que está guardado de esta salida: %w", err)
		}
	}
	if p.Puerto == 0 {
		p.Puerto = PuertoHTTPTSPorDefecto
	}
	p.Codec = strings.ToLower(strings.TrimSpace(p.Codec))
	if p.Codec == "" {
		p.Codec = CodecHTTPTSPorDefecto
	}
	p.Ruta = strings.TrimSpace(p.Ruta)
	if p.Ruta == "" {
		p.Ruta = RutaHTTPTSPorDefecto
	}
	if !strings.HasPrefix(p.Ruta, "/") {
		// Nadie tiene que acordarse de la barra: se le pone.
		p.Ruta = "/" + p.Ruta
	}
	p.Audio = strings.ToLower(strings.TrimSpace(p.Audio))
	if p.Codec == "h264" {
		// El audio de una salida H.264 lo pone el encoder: AAC, que es lo
		// único que un navegador sabe oír (engine.argsH264TS).
		if p.Audio == "" {
			p.Audio = AudioWebPorDefecto
		}
		if p.BitrateVideoKbs == 0 {
			p.BitrateVideoKbs = VideoKbsWebPorDefecto
		}
		if p.BitrateMuxKbs == 0 {
			p.BitrateMuxKbs = MuxKbsWebPorDefecto
		}
		return p, nil
	}
	if p.Audio == "" {
		p.Audio = AudioPorDefecto
	}
	if p.BitrateVideoKbs == 0 {
		p.BitrateVideoKbs = VideoKbsPorDefecto
	}
	if p.BitrateMuxKbs == 0 {
		p.BitrateMuxKbs = MuxKbsPorDefecto
	}
	return p, nil
}

// valida dice en palabras claras lo que no se puede servir, y qué hacer.
func (d *httpts) valida() error {
	p := d.p
	switch {
	case p.Puerto < 1 || p.Puerto > 65535:
		return fmt.Errorf("el puerto %d no existe: va del 1 al 65535, y el de costumbre para esto es el %d",
			p.Puerto, PuertoHTTPTSPorDefecto)
	case strings.ContainsAny(p.Ruta, " ?#"):
		return fmt.Errorf("la dirección %q lleva un espacio o un signo que no se puede usar: escríbela como %s",
			p.Ruta, RutaHTTPTSPorDefecto)
	case p.Codec != "mpeg2" && p.Codec != "h264":
		return fmt.Errorf("esta salida se sirve en MPEG-2 (mpeg2) —para VLC o un multiplexor— o en H.264 (h264) —para verla en un navegador—, y pusiste %q",
			p.Codec)
	case p.BitrateVideoKbs <= 0 || p.BitrateMuxKbs <= 0:
		return fmt.Errorf("las dos tasas —la de la imagen y la de la señal entera— tienen que ser mayores que cero")
	case p.BitrateVideoKbs+MargenDelMuxKbs > p.BitrateMuxKbs:
		return fmt.Errorf("el video pide %d kb/s y la señal entera solo lleva %d: el audio y las tablas no caben, deja al menos %d kb/s de margen sobre el video",
			p.BitrateVideoKbs, p.BitrateMuxKbs, MargenDelMuxKbs)
	case p.Codec == "h264" && p.Audio != AudioWebPorDefecto:
		return fmt.Errorf("una salida para el navegador lleva el sonido en AAC (aac) y pusiste %q: el MPEG capa II y el AC-3 son para el transmisor",
			p.Audio)
	case p.Codec == "mpeg2" && p.Audio != "mp2" && p.Audio != "ac3":
		return fmt.Errorf("el sonido de esta salida es MPEG capa II (mp2) o AC-3 (ac3), y pusiste %q; CAtv emite hoy en mp2", p.Audio)
	}
	return nil
}

// Abrir levanta las dos puntas: la escucha de esta máquina a la que ffmpeg
// entrega el transport stream, y el puerto HTTP del que tiran los clientes.
//
// Se llama una vez por vida del encoder. La escucha de ffmpeg es nueva cada
// vez —puerto que da el sistema, se cierra sola al terminar—; el puerto HTTP
// se comparte y sobrevive, que es lo que hace que un VLC que estaba mirando
// no se quede sin a dónde volver cuando el encoder se relanza.
func (d *httpts) Abrir(engine.Format) (engine.Output, error) {
	// Solo en esta máquina: por aquí no entra nadie de fuera, es el caño
	// interno entre ffmpeg y nosotros.
	entrega, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return engine.Output{}, fmt.Errorf("no pude abrir el camino interno por el que llega la señal: %w", err)
	}
	srv, err := servidorEn(d.p.Puerto)
	if err != nil {
		_ = entrega.Close()
		return engine.Output{}, err
	}
	rep := nuevoReparto()
	srv.poner(d.p.Ruta, rep)
	go recibirYRepartir(entrega, srv, d.p.Ruta, rep)

	out := engine.Output{
		Name:     d.nombre(),
		Kind:     "mpeg2-ts",
		TCP:      entrega.Addr().String(),
		VideoKbs: d.p.BitrateVideoKbs,
		MuxKbs:   d.p.BitrateMuxKbs,
		Audio:    d.p.Audio,
		AudioKbs: d.p.BitrateAudioKbs,
		PCRms:    engine.PCRmsPorDefecto,
	}
	if d.p.Codec == "h264" {
		// El encoder pone AAC y H.264 él solo en esta clase de salida; la
		// tasa del mux no manda aquí, que esto no va a un transmisor.
		out.Kind, out.Audio = "h264-ts", ""
	}
	return out, nil
}

// Vigilar deja escrito cómo le va a esta salida.
func (d *httpts) Vigilar(ctx context.Context, salida model.Output, listo <-chan error) {
	vigilar(ctx, d.reg, salida, listo)
}

// Descripcion es la dirección que hay que escribir en el otro programa.
func (d *httpts) Descripcion() string {
	video, sonido := "MPEG-2", NombreDelAudio(d.p.Audio)
	quien := "para VLC, un multiplexor u otro programa"
	if d.p.Codec == "h264" {
		video, sonido = "H.264", "AAC"
		quien = "para verla en un navegador"
	}
	return fmt.Sprintf("a quien tire de http://«esta máquina»:%d%s · %s %d kb/s de imagen, sonido %s · %s",
		d.p.Puerto, d.p.Ruta, video, d.p.BitrateVideoKbs, sonido, quien)
}

func (d *httpts) nombre() string {
	if n := strings.TrimSpace(d.salida.Name); n != "" {
		return n
	}
	return "copia por HTTP"
}

// ── el puerto que escucha ─────────────────────────────────────────────

// Los puertos HTTP abiertos por este proceso, uno por número de puerto. Se
// comparten entre vidas del encoder y no se cierran hasta que el programa
// termina, a propósito:
//
//   - Si se cerrara al final de cada vida, el relanzado del encoder tendría
//     que volver a tomar el mismo puerto medio segundo después, y el sistema
//     todavía lo tiene retenido: la salida fallaría al abrir cada vez que el
//     canal se recupera, que es el peor momento posible.
//   - Y quien esté mirando desde fuera apunta a una dirección fija. Que la
//     dirección desaparezca y vuelva cada vez que el encoder se relanza es
//     exactamente lo que no queremos.
var (
	muServidores sync.Mutex
	servidores   = map[int]*servidorHTTPTS{}
)

// servidorHTTPTS es un puerto escuchando y las direcciones que sirve. Cada
// dirección tiene el reparto de la vida del encoder en curso, o ninguno
// cuando el canal no está produciendo.
type servidorHTTPTS struct {
	mu    sync.Mutex
	rutas map[string]*reparto
}

// servidorEn devuelve el servidor de ese puerto, y lo levanta la primera vez.
func servidorEn(puerto int) (*servidorHTTPTS, error) {
	muServidores.Lock()
	defer muServidores.Unlock()
	if s := servidores[puerto]; s != nil {
		return s, nil
	}
	// En todas las direcciones de la máquina: el que tira de la señal está
	// en otro equipo de la torre, no en este.
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", puerto))
	if err != nil {
		return nil, fmt.Errorf("no pude escuchar en el puerto %d: seguramente otro programa de esta máquina ya lo está usando; escoge otro puerto o cierra el que lo tiene (%w)",
			puerto, err)
	}
	s := &servidorHTTPTS{rutas: map[string]*reparto{}}
	// Sin plazo de escritura: lo que sale por aquí es una señal, no una
	// página, y no termina nunca.
	hs := &http.Server{Handler: s, ReadHeaderTimeout: PlazoDeCabeceras}
	go func() { _ = hs.Serve(l) }()
	servidores[puerto] = s
	return s, nil
}

// poner deja esta dirección sirviendo este reparto. Sustituye al de la vida
// anterior del encoder, si quedaba alguno.
func (s *servidorHTTPTS) poner(ruta string, rep *reparto) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rutas[ruta] = rep
}

// quitar deja la dirección sin señal, pero solo si el reparto que se va es el
// que estaba puesto: el de la vida anterior no puede apagar al de la nueva.
func (s *servidorHTTPTS) quitar(ruta string, rep *reparto) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.rutas[ruta] == rep {
		delete(s.rutas, ruta)
	}
}

func (s *servidorHTTPTS) de(ruta string) *reparto {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rutas[ruta]
}

// ServeHTTP entrega el transport stream tal cual, sin fin. Cada cliente va en
// su propia goroutine —esta— y ninguno sabe de los demás.
func (s *servidorHTTPTS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Esta dirección solo entrega la señal: se pide con GET.", http.StatusMethodNotAllowed)
		return
	}
	rep := s.de(r.URL.Path)
	if rep == nil {
		http.Error(w, "Aquí no está saliendo nada ahora mismo. Comprueba la dirección, y que el canal esté al aire.",
			http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "video/mp2t")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	lector := rep.entra()
	if lector == nil {
		http.Error(w, "El canal acaba de dejar de mandar señal por aquí. Vuelve a pedirla en un momento.",
			http.StatusServiceUnavailable)
		return
	}
	defer rep.sale(lector)

	w.WriteHeader(http.StatusOK)
	// Sin esto, la señal se queda en el búfer del servidor y al otro lado no
	// se ve nada hasta que se llena: en una señal en vivo eso es el retraso.
	vaciar, _ := w.(http.Flusher)
	if vaciar != nil {
		vaciar.Flush()
	}
	for {
		select {
		case <-r.Context().Done():
			return
		case bloque, abierto := <-lector.bloques:
			if !abierto {
				return // o se fue muy lento, o el canal dejó de producir
			}
			if _, err := w.Write(bloque); err != nil {
				return
			}
			if vaciar != nil {
				vaciar.Flush()
			}
		}
	}
}

// ── el reparto ────────────────────────────────────────────────────────

// reparto es un transport stream y los clientes que están tirando de él.
type reparto struct {
	mu       sync.Mutex
	clientes map[*lectorHTTP]struct{}
	cerrado  bool
}

// lectorHTTP es un cliente conectado. `bloques` es lo único que se le manda;
// `sincronizado` dice si ya recibió la tabla de programas y puede entender lo
// que le llega.
type lectorHTTP struct {
	bloques      chan []byte
	sincronizado bool
	cerrado      bool
}

func nuevoReparto() *reparto {
	return &reparto{clientes: map[*lectorHTTP]struct{}{}}
}

// entra apunta un cliente nuevo. Devuelve nil si este reparto ya se acabó.
func (r *reparto) entra() *lectorHTTP {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cerrado {
		return nil
	}
	l := &lectorHTTP{bloques: make(chan []byte, BloquesEnEspera)}
	r.clientes[l] = struct{}{}
	return l
}

// sale quita a un cliente que se fue.
func (r *reparto) sale(l *lectorHTTP) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.soltar(l)
}

// soltar quita a un cliente y le cierra la espera. Con r.mu tomado. Se puede
// llamar dos veces —el cliente se va justo cuando se le corta— y la segunda
// no hace nada.
func (r *reparto) soltar(l *lectorHTTP) {
	delete(r.clientes, l)
	if !l.cerrado {
		l.cerrado = true
		close(l.bloques)
	}
}

// manda reparte un trozo de transport stream. `desdePAT` es dónde empieza,
// dentro del trozo, la primera tabla de programas, y vale -1 si no hay
// ninguna.
//
// Nunca se bloquea: quien no lee se queda sin señal él solo. El aire no
// espera a nadie.
func (r *reparto) manda(bloque []byte, desdePAT int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for l := range r.clientes {
		trozo := bloque
		if !l.sincronizado {
			// Un cliente que llega a mitad no entiende nada hasta la
			// siguiente tabla de programas: se le empieza ahí, no antes.
			if desdePAT < 0 {
				continue
			}
			trozo = bloque[desdePAT:]
			l.sincronizado = true
		}
		select {
		case l.bloques <- trozo:
		default:
			// Lleva medio segundo sin leer: se le corta a él y se sigue.
			r.soltar(l)
		}
	}
}

// cerrar deja el reparto sin señal y suelta a todos: el encoder dejó de
// producir. Quien estaba mirando lo ve cortarse y vuelve a pedirla.
func (r *reparto) cerrar() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cerrado = true
	for l := range r.clientes {
		r.soltar(l)
	}
}

// ── lo que llega de ffmpeg ────────────────────────────────────────────

// recibirYRepartir espera a que ffmpeg conecte a entregar el transport
// stream, y desde ahí lee sin parar hasta que se corte.
//
// Lee SIEMPRE, haya o no haya clientes: si dejara de leer, se llena el búfer
// de ffmpeg y se cuelga el encoder entero, y con él todas las demás salidas
// del canal.
func recibirYRepartir(entrega net.Listener, srv *servidorHTTPTS, ruta string, rep *reparto) {
	defer srv.quitar(ruta, rep)
	defer rep.cerrar()

	if tl, ok := entrega.(*net.TCPListener); ok {
		_ = tl.SetDeadline(time.Now().Add(PlazoDeEntrega))
	}
	conn, err := entrega.Accept()
	// Una sola entrega por vida del encoder: en cuanto conecta, se cierra la
	// puerta.
	_ = entrega.Close()
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()

	crudo := make([]byte, LeeDeUnaVez)
	// Lo que sobró de la vuelta anterior: un paquete de 188 bytes puede
	// llegar partido en dos lecturas y no se reparte a medias.
	var pendiente []byte
	var alineado bool
	for {
		n, err := conn.Read(crudo)
		if n > 0 {
			pendiente = append(pendiente, crudo[:n]...)
			corte := 0
			if !alineado {
				corte = primerPaquete(pendiente)
			}
			if corte < 0 {
				// Todavía no se sabe dónde empieza un paquete. Con ffmpeg no
				// pasa —escribe el transport stream desde el primer byte—;
				// está por si algún día no es ffmpeg quien entrega.
				if len(pendiente) > 4*ts.PacketSize {
					pendiente = pendiente[len(pendiente)-4*ts.PacketSize:]
				}
				if err != nil {
					return
				}
				continue
			}
			pendiente, alineado = pendiente[corte:], true
			enteros := len(pendiente) / ts.PacketSize * ts.PacketSize
			if enteros > 0 {
				// Copia propia: el trozo se le entrega a varios clientes a la
				// vez y nadie puede escribir encima mientras lo leen.
				bloque := make([]byte, enteros)
				copy(bloque, pendiente[:enteros])
				rep.manda(bloque, desdeLaPAT(bloque))
				pendiente = append(pendiente[:0], pendiente[enteros:]...)
			}
		}
		if err != nil {
			return // el encoder terminó o se cayó; quien llama lo relanza
		}
	}
}

// primerPaquete busca dónde empieza un paquete de transport stream: un 0x47
// con otro 0x47 exactamente 188 bytes más allá. Devuelve -1 si todavía no se
// puede saber.
func primerPaquete(b []byte) int {
	for i := 0; i+ts.PacketSize < len(b); i++ {
		if b[i] == 0x47 && b[i+ts.PacketSize] == 0x47 {
			return i
		}
	}
	return -1
}

// desdeLaPAT dice en qué byte del trozo empieza la tabla de programas, que es
// por donde tiene que arrancar un cliente que llega a mitad: sin ella no sabe
// qué programa hay ni en qué PID viene. La tabla del programa (PMT) sale
// pegada detrás, porque el encoder las repite juntas (engine.PATPeriodo).
//
// Devuelve -1 si en este trozo no hay ninguna.
func desdeLaPAT(bloque []byte) int {
	for i := 0; i+ts.PacketSize <= len(bloque); i += ts.PacketSize {
		p := bloque[i:]
		pid := uint16(p[1]&0x1f)<<8 | uint16(p[2])
		inicioDeTabla := p[1]&0x40 != 0
		if pid == 0 && inicioDeTabla {
			return i
		}
	}
	return -1
}
