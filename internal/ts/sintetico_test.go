package ts

import "bytes"

// Este archivo arma transport streams a mano, paquete por paquete de 188
// bytes, para poder probar el medidor sin ffmpeg y —sobre todo— para poder
// meterle defectos a propósito: una continuidad rota, un PCR que falta, una
// tasa que se cae a la mitad. Un TS de ffmpeg sirve para confirmar que lo
// real también se mide (ts_ffmpeg_test.go), pero no sirve para provocar el
// fallo: ffmpeg no emite streams malos cuando se le pide.
//
// No hay TSDuck (ADR 0003) ni CRC32: internal/ts no valida el CRC, así que
// aquí tampoco se calcula. Los cuatro bytes van en 0xFF y quedan dichos.

const (
	pidPAT = uint16(0x0000)
	pidNul = uint16(0x1fff)
)

// Valores del stream de referencia que usan casi todas las pruebas. Son
// distintos de los de fábrica de ffmpeg a propósito: si el medidor los leyera
// de una constante en vez de del stream, se notaría.
const (
	pidPMT   = uint16(0x1000)
	pidVideo = uint16(0x0101)
	pidAudio = uint16(0x0102)
	programa = uint16(7)
	tsid     = uint16(42)
)

// constructor acumula paquetes y lleva el contador de continuidad de cada
// PID, que es justo lo que el medidor comprueba. Quien quiera romperlo lo
// rompe a mano con saltarCC.
type constructor struct {
	buf bytes.Buffer
	cc  map[uint16]byte
}

func nuevoConstructor() *constructor { return &constructor{cc: map[uint16]byte{}} }

func (c *constructor) bytes() []byte { return c.buf.Bytes() }

// cc devuelve el contador que le toca a este PID y lo avanza. El contador es
// de cuatro bits y da la vuelta en 15 → 0; esa vuelta es correcta y el
// medidor no debe contarla como error.
func (c *constructor) siguienteCC(pid uint16) byte {
	v := c.cc[pid]
	c.cc[pid] = (v + 1) & 0x0f
	return v
}

// saltarCC se come un valor del contador sin emitir paquete: el siguiente
// paquete de ese PID llegará con el contador adelantado, que es exactamente
// un paquete perdido en el camino.
func (c *constructor) saltarCC(pid uint16) { c.siguienteCC(pid) }

// cabecera arma los cuatro bytes de cabecera de un paquete de 188, con el
// resto en ceros. afc es el adaptation_field_control: 1 solo carga, 2 solo
// adaptación (sin carga, y por eso sin avanzar el contador), 3 las dos.
func cabecera(pid uint16, pusi bool, afc byte, cc byte) []byte {
	p := make([]byte, PacketSize)
	p[0] = 0x47
	p[1] = byte(pid>>8) & 0x1f
	if pusi {
		p[1] |= 0x40
	}
	p[2] = byte(pid)
	p[3] = (afc << 4) | (cc & 0x0f)
	return p
}

// carga escribe un paquete de pura carga, relleno con un byte fijo para
// poder distinguir a ojo el video del audio en un volcado.
func (c *constructor) carga(pid uint16, relleno byte) {
	p := cabecera(pid, false, 1, c.siguienteCC(pid))
	for i := 4; i < len(p); i++ {
		p[i] = relleno
	}
	c.buf.Write(p)
}

// soloAdaptacion escribe un paquete sin carga (afc=2). Por norma el contador
// de continuidad no avanza en estos paquetes, así que el constructor tampoco
// lo avanza: si el medidor los contara, saldría un error de continuidad
// donde no hay ninguno.
func (c *constructor) soloAdaptacion(pid uint16) {
	p := cabecera(pid, false, 2, c.cc[pid])
	p[4] = byte(PacketSize - 5) // la adaptación llena el paquete entero
	c.buf.Write(p)
}

// conPCR escribe un paquete con reloj de programa (PCR) y carga. El PCR va
// en segundos y se codifica como la norma manda —base de 33 bits a 90 kHz
// más extensión de 9 bits a 27 MHz—, que es lo que Analyze decodifica de
// vuelta.
func (c *constructor) conPCR(pid uint16, segundos float64, relleno byte) {
	p := cabecera(pid, false, 3, c.siguienteCC(pid))
	const al = 7 // banderas(1) + base y extensión del PCR (6)
	p[4] = al
	p[5] = 0x10 // PCR_flag
	base := uint64(segundos * 90000)
	p[6] = byte(base >> 25)
	p[7] = byte(base >> 17)
	p[8] = byte(base >> 9)
	p[9] = byte(base >> 1)
	p[10] = byte((base&1)<<7) | 0x7e // reservados en 1, extensión en 0
	p[11] = 0x00
	for i := 4 + 1 + al; i < len(p); i++ {
		p[i] = relleno
	}
	c.buf.Write(p)
}

// nulo escribe un paquete de relleno (PID 0x1FFF). Es lo que un multiplexor
// a tasa constante mete cuando no hay nada que decir; no tiene contador que
// valga y el medidor solo lo cuenta.
func (c *constructor) nulo() {
	p := cabecera(pidNul, false, 1, 0)
	for i := 4; i < len(p); i++ {
		p[i] = 0xff
	}
	c.buf.Write(p)
}

// seccion mete una sección PSI (PAT o PMT) en un paquete con PUSI y el
// puntero en cero, rellenando el resto con 0xFF.
func (c *constructor) seccion(pid uint16, sec []byte) {
	p := cabecera(pid, true, 1, c.siguienteCC(pid))
	off := 4
	p[off] = 0x00 // pointer_field: la sección empieza justo aquí
	off++
	off += copy(p[off:], sec)
	for ; off < len(p); off++ {
		p[off] = 0xff
	}
	c.buf.Write(p)
}

// crudo deja escribir un paquete tal cual, para las pruebas de basura.
func (c *constructor) crudo(p []byte) { c.buf.Write(p) }

// programaPAT es una entrada de la tabla de programas.
type programaPAT struct {
	Numero uint16
	PMTPid uint16
}

// armarPAT construye una sección PAT con las entradas que se le den. Un
// número de programa 0 es la entrada de la tabla de red (NIT), no un
// programa, y el medidor debe saltársela.
func armarPAT(idTS uint16, entradas ...programaPAT) []byte {
	b := []byte{
		0x00,       // table_id de la PAT
		0x00, 0x00, // section_length: se rellena al final
		byte(idTS >> 8), byte(idTS),
		0xc1,       // reservados + versión 0 + current_next=1
		0x00, 0x00, // section_number, last_section_number
	}
	for _, e := range entradas {
		b = append(b, byte(e.Numero>>8), byte(e.Numero), 0xe0|byte(e.PMTPid>>8), byte(e.PMTPid))
	}
	return cerrarSeccion(b)
}

// flujo es un flujo elemental de la PMT: su tipo y su PID.
type flujo struct {
	Tipo uint8
	PID  uint16
}

// armarPMT construye una sección PMT con el PID del PCR y los flujos dados,
// sin descriptores de programa.
func armarPMT(numero, pcrPID uint16, flujos ...flujo) []byte {
	b := []byte{
		0x02,       // table_id de la PMT
		0x00, 0x00, // section_length: se rellena al final
		byte(numero >> 8), byte(numero),
		0xc1,
		0x00, 0x00,
		0xe0 | byte(pcrPID>>8), byte(pcrPID),
		0xf0, 0x00, // program_info_length = 0
	}
	for _, f := range flujos {
		b = append(b, f.Tipo, 0xe0|byte(f.PID>>8), byte(f.PID), 0xf0, 0x00)
	}
	return cerrarSeccion(b)
}

// cerrarSeccion pega el CRC de relleno y escribe el section_length, que
// cuenta todo lo que va detrás de esos dos bytes.
func cerrarSeccion(b []byte) []byte {
	b = append(b, 0xff, 0xff, 0xff, 0xff) // CRC32: no se valida en internal/ts
	largo := len(b) - 3
	b[1] = 0xb0 | byte(largo>>8&0x0f)
	b[2] = byte(largo)
	return b
}

// pes escribe el arranque de un paquete PES con sus marcas de tiempo, que es
// de donde el medidor saca si el tiempo va hacia adelante. pts y dts van en
// unidades de 90 kHz; dts negativo manda solo PTS, que es el caso corriente
// del audio.
func (c *constructor) pes(pid uint16, pts, dts int64) {
	p := cabecera(pid, true, 1, c.siguienteCC(pid))
	p[4], p[5], p[6] = 0x00, 0x00, 0x01 // prefijo de arranque de paquete
	p[7] = 0xe0                         // stream_id de video
	p[8], p[9] = 0x00, 0x00             // PES_packet_length: 0 = sin declarar
	p[10] = 0x80                        // '10' + banderas
	if dts >= 0 {
		p[11] = 0xc0 // PTS y DTS
		p[12] = 10   // PES_header_data_length
		escribirMarca(p[13:], pts)
		escribirMarca(p[18:], dts)
	} else {
		p[11] = 0x80 // solo PTS
		p[12] = 5
		escribirMarca(p[13:], pts)
	}
	c.buf.Write(p)
}

// escribirMarca codifica una marca de tiempo de 33 bits en los cinco bytes
// con los bits de relleno que la norma intercala, que es lo que readTS
// deshace.
func escribirMarca(b []byte, v int64) {
	b[0] = byte(v>>29) & 0x0e
	b[1] = byte(v >> 22)
	b[2] = byte(v>>14)&0xfe | 1
	b[3] = byte(v >> 7)
	b[4] = byte(v<<1) | 1
}

// streamLimpio arma el TS de referencia: PAT y PMT al principio, video con
// PCR cada diez paquetes —100 ms de reloj entre uno y el siguiente, como
// pide el PRD §10—, audio, y un nulo cada veinte. Sin un solo defecto.
// Las pruebas que quieren un defecto parten de este mismo molde y lo
// tuercen, para que la diferencia medida sea solo el defecto.
func streamLimpio(paquetesPorPID int) []byte {
	c := nuevoConstructor()
	c.seccion(pidPAT, armarPAT(tsid, programaPAT{Numero: programa, PMTPid: pidPMT}))
	c.seccion(pidPMT, armarPMT(programa, pidVideo,
		flujo{Tipo: TipoMPEG2Video, PID: pidVideo},
		flujo{Tipo: TipoAC3, PID: pidAudio}))
	for i := 0; i < paquetesPorPID; i++ {
		if i%10 == 0 {
			c.conPCR(pidVideo, float64(i)/100, 0xaa)
		} else {
			c.carga(pidVideo, 0xaa)
		}
		c.carga(pidAudio, 0xbb)
		if i%20 == 0 {
			c.nulo()
		}
	}
	return c.bytes()
}
