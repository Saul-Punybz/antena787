package salida

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"

	"antena787/internal/engine"
	"antena787/internal/model"
)

// ParamsUDPTS es lo que se guarda en `output.parametros` de una salida al
// multiplexor. Son las mismas preguntas que VLC hace en su `sout`
// (docs/VLC-PARIDAD.md, fila «Opciones del mux TS»), con nombres de persona.
//
// Todo tiene valor de fábrica menos el destino, y un cero quiere decir «esto
// no lo escribí»: se rellena con el de fábrica. **Ninguno está fijo en el
// código**: mientras no llegue la cadena `sout` exacta de Rolando, los PID,
// el programa y las tasas son valores de ejemplo que se cambian desde la
// pantalla (docs/f2/PLAN-F2.md, riesgo de T2).
type ParamsUDPTS struct {
	// Destino es «192.168.1.50:1234» para un receptor, o «239.1.1.1:1234»
	// para un grupo multicast (F2-114). Se acepta con o sin «udp://».
	Destino string `json:"destino"`
	// TTL son los saltos de red que vive el paquete. Solo hace falta para un
	// grupo multicast; en un grupo sin TTL escrito se pone 1, que no sale de
	// la propia red.
	TTL             int    `json:"ttl"`
	BitrateMuxKbs   int    `json:"bitrate_mux_kbs"`
	BitrateVideoKbs int    `json:"bitrate_video_kbs"`
	PIDVideo        int    `json:"pid_video"`
	PIDAudio        int    `json:"pid_audio"`
	PIDPMT          int    `json:"pid_pmt"`
	Programa        int    `json:"program"`
	TSID            int    `json:"tsid"`
	PCRms           int    `json:"pcr_ms"`
	Audio           string `json:"audio"` // ac3 | mp2
	Video           string `json:"video"` // mpeg2
}

// Valores de ejemplo, todos configurables (PRD §10: «PIDs y número de
// programa fijos que la persona escribe una vez», no fijos en el programa).
// Las dos tasas son las que midió la F0; los PID son los que usa media
// industria (0x1E0 para la PMT, 0x200 y 0x201 para video y audio).
const (
	MuxKbsPorDefecto   = 10000
	VideoKbsPorDefecto = 8000
	PIDVideoPorDefecto = 512
	PIDAudioPorDefecto = 513
	PIDPMTPorDefecto   = 480
	ProgramaPorDefecto = 1
	TSIDPorDefecto     = 1
	AudioPorDefecto    = "mp2" // lo que CAtv emite hoy y se oye en los televisores
	VideoPorDefecto    = "mpeg2"
	// TTLDeGrupoPorDefecto es lo que se le pone a un grupo multicast cuando
	// nadie dijo cuántos saltos: uno, que se queda en la propia red.
	TTLDeGrupoPorDefecto = 1
	// MargenDelMuxKbs es lo que hay que dejarle al audio y a las tablas por
	// encima del video: si no cabe, el TS se desborda y el multiplexor lo
	// nota antes que nadie.
	MargenDelMuxKbs = 500
)

// Los PID que no son de nadie: 0 es la tabla de programas, del 1 al 31 son
// tablas, y 8191 son los paquetes de relleno que hacen la tasa constante.
const (
	PIDMinimoDeTabla = 16
	PIDMinimo        = 32
	PIDMaximo        = 8190
)

// udpts es el driver de la salida al multiplexor: MPEG-2 CBR por UDP,
// unicast a una dirección o a un grupo multicast con su TTL (F2-46, F2-114).
type udpts struct {
	salida model.Output
	reg    Registro
	p      ParamsUDPTS
	host   string
	puerto int
	grupo  bool // el destino es un grupo multicast
}

func nuevoUDPTS(o model.Output, reg Registro) (Driver, error) {
	p, err := leerParamsUDPTS(o.Params)
	if err != nil {
		return nil, err
	}
	host, puerto, err := destino(p.Destino)
	if err != nil {
		return nil, err
	}
	d := &udpts{salida: o, reg: reg, p: p, host: host, puerto: puerto, grupo: esGrupo(host)}
	if d.grupo && d.p.TTL == 0 {
		d.p.TTL = TTLDeGrupoPorDefecto
	}
	if err := d.valida(); err != nil {
		return nil, err
	}
	return d, nil
}

// leerParamsUDPTS lee el JSON de la salida y rellena lo que falte con los
// valores de ejemplo.
func leerParamsUDPTS(raw string) (ParamsUDPTS, error) {
	p := ParamsUDPTS{}
	if s := strings.TrimSpace(raw); s != "" && s != "{}" {
		if err := json.Unmarshal([]byte(s), &p); err != nil {
			return p, fmt.Errorf("no entiendo lo que está guardado de esta salida: %w", err)
		}
	}
	if p.BitrateMuxKbs == 0 {
		p.BitrateMuxKbs = MuxKbsPorDefecto
	}
	if p.BitrateVideoKbs == 0 {
		p.BitrateVideoKbs = VideoKbsPorDefecto
	}
	if p.PIDVideo == 0 {
		p.PIDVideo = PIDVideoPorDefecto
	}
	if p.PIDAudio == 0 {
		p.PIDAudio = PIDAudioPorDefecto
	}
	if p.PIDPMT == 0 {
		p.PIDPMT = PIDPMTPorDefecto
	}
	if p.Programa == 0 {
		p.Programa = ProgramaPorDefecto
	}
	if p.TSID == 0 {
		p.TSID = TSIDPorDefecto
	}
	if p.PCRms == 0 {
		p.PCRms = engine.PCRmsPorDefecto
	}
	if p.Audio == "" {
		p.Audio = AudioPorDefecto
	}
	if p.Video == "" {
		p.Video = VideoPorDefecto
	}
	p.Audio = strings.ToLower(strings.TrimSpace(p.Audio))
	p.Video = strings.ToLower(strings.TrimSpace(p.Video))
	return p, nil
}

// destino parte «239.1.1.1:1234» —con o sin udp:// delante— en dirección y
// puerto, y lo dice en cristiano cuando no se entiende.
func destino(s string) (string, int, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "udp://"), "@")
	if s == "" {
		return "", 0, fmt.Errorf("no me has dicho a qué dirección mandar la señal: se escribe como 192.168.1.50:1234")
	}
	host, puerto, err := net.SplitHostPort(s)
	if err != nil {
		return "", 0, fmt.Errorf("no entiendo la dirección %q: hace falta la dirección y el puerto, como 192.168.1.50:1234", s)
	}
	n, err := strconv.Atoi(puerto)
	if err != nil || n < 1 || n > 65535 {
		return "", 0, fmt.Errorf("el puerto %q no existe: va del 1 al 65535", puerto)
	}
	if strings.TrimSpace(host) == "" {
		return "", 0, fmt.Errorf("falta la dirección antes del puerto: se escribe como 192.168.1.50:%d", n)
	}
	return host, n, nil
}

// esGrupo dice si la dirección es un grupo multicast (224.0.0.0 a
// 239.255.255.255, y los ff00::/8 de IPv6).
func esGrupo(host string) bool {
	ip := net.ParseIP(host)
	return ip != nil && ip.IsMulticast()
}

// valida dice en cristiano lo que un multiplexor no perdonaría. Nada de
// «parámetro inválido»: lo que se lee es lo que pasa y qué hacer.
func (d *udpts) valida() error {
	p := d.p
	switch {
	case p.PCRms > engine.PCRmsMaximo:
		return fmt.Errorf("el PCR tiene que salir cada %d ms o menos y pusiste %d: un multiplexor descarta lo que llega más tarde (PRD §10)",
			engine.PCRmsMaximo, p.PCRms)
	case p.PCRms < 0:
		return fmt.Errorf("el PCR no puede ser negativo")
	case p.TTL < 0 || p.TTL > 255:
		return fmt.Errorf("los saltos de red (TTL) van de 1 a 255 y pusiste %d", p.TTL)
	case p.BitrateVideoKbs <= 0 || p.BitrateMuxKbs <= 0:
		return fmt.Errorf("las dos tasas —la de la imagen y la de la señal entera— tienen que ser mayores que cero")
	case p.BitrateVideoKbs+MargenDelMuxKbs > p.BitrateMuxKbs:
		return fmt.Errorf("el video pide %d kb/s y la señal entera solo lleva %d: el audio y las tablas no caben, deja al menos %d kb/s de margen sobre el video",
			p.BitrateVideoKbs, p.BitrateMuxKbs, MargenDelMuxKbs)
	case p.Audio != "mp2" && p.Audio != "ac3":
		return fmt.Errorf("el audio del multiplexor es MPEG capa II (mp2) o AC-3 (ac3), y pusiste %q; CAtv emite hoy en mp2", p.Audio)
	case p.Video != "mpeg2":
		return fmt.Errorf("un multiplexor de ATSC 1.0 espera video MPEG-2 y pusiste %q: el H.264 sale por una salida de internet", p.Video)
	case p.Programa < 1 || p.Programa > 65535:
		return fmt.Errorf("el número de programa va del 1 al 65535 y pusiste %d", p.Programa)
	case p.TSID < 1 || p.TSID > 65535:
		return fmt.Errorf("el identificador de la señal (tsid) va del 1 al 65535 y pusiste %d", p.TSID)
	}
	if err := pidValido("el video", p.PIDVideo, PIDMinimo); err != nil {
		return err
	}
	if err := pidValido("el audio", p.PIDAudio, PIDMinimo); err != nil {
		return err
	}
	if err := pidValido("la tabla del programa (PMT)", p.PIDPMT, PIDMinimoDeTabla); err != nil {
		return err
	}
	if p.PIDVideo == p.PIDAudio || p.PIDVideo == p.PIDPMT || p.PIDAudio == p.PIDPMT {
		return fmt.Errorf("el video, el audio y la tabla del programa no pueden ir en el mismo PID (%d, %d y %d)",
			p.PIDVideo, p.PIDAudio, p.PIDPMT)
	}
	return nil
}

// pidValido dice si un PID se puede usar, y por qué no si no se puede.
func pidValido(que string, pid, minimo int) error {
	if pid < minimo || pid > PIDMaximo {
		return fmt.Errorf("el PID de %s va del %d al %d y pusiste %d: el 0 es la tabla de programas y el 8191 son los paquetes de relleno que hacen la tasa constante",
			que, minimo, PIDMaximo, pid)
	}
	return nil
}

// Abrir traduce la salida a los argumentos del encoder. El formato de casa
// no cambia nada aquí —el conformado ya lo hizo el servidor de cuadros—:
// entra para que el driver pueda quejarse si algún día no cuadra.
func (d *udpts) Abrir(engine.Format) (engine.Output, error) {
	return engine.Output{
		Name:     d.nombre(),
		Kind:     "mpeg2-ts",
		UDP:      fmt.Sprintf("udp://%s:%d", d.host, d.puerto),
		VideoKbs: d.p.BitrateVideoKbs,
		MuxKbs:   d.p.BitrateMuxKbs,
		PIDVideo: d.p.PIDVideo,
		PIDAudio: d.p.PIDAudio,
		PIDPMT:   d.p.PIDPMT,
		Programa: d.p.Programa,
		TSID:     d.p.TSID,
		PCRms:    d.p.PCRms,
		Audio:    d.p.Audio,
		TTL:      d.p.TTL,
		PktSize:  engine.PktSizePorDefecto,
	}, nil
}

// Vigilar deja escrito cómo le va a esta salida.
func (d *udpts) Vigilar(ctx context.Context, salida model.Output, listo <-chan error) {
	vigilar(ctx, d.reg, salida, listo)
}

// Descripcion es a dónde va, dicho como se le dice a una persona.
func (d *udpts) Descripcion() string {
	donde := fmt.Sprintf("al receptor %s:%d", d.host, d.puerto)
	if d.grupo {
		donde = fmt.Sprintf("al grupo %s:%d, %d salto(s) de red", d.host, d.puerto, d.p.TTL)
	}
	return fmt.Sprintf("%s · MPEG-2 %d kb/s de imagen, %d kb/s en total · programa %d, PID %d/%d, sonido %s",
		donde, d.p.BitrateVideoKbs, d.p.BitrateMuxKbs, d.p.Programa, d.p.PIDVideo, d.p.PIDAudio,
		NombreDelAudio(d.p.Audio))
}

// NombreDelAudio dice el sonido como se le dice a una persona: nadie tiene por
// qué saber qué es «mp2».
func NombreDelAudio(audio string) string {
	switch audio {
	case "mp2":
		return "MPEG capa II"
	case "ac3":
		return "AC-3"
	}
	return audio
}

func (d *udpts) nombre() string {
	if n := strings.TrimSpace(d.salida.Name); n != "" {
		return n
	}
	return "transmisor"
}
