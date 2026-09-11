package hdhomerun

import (
	"bytes"

	"antena787/internal/ts"
)

// Este archivo construye un TS mínimo a mano —PAT, PMT, un PID de video y
// uno de audio, PCR periódico y paquetes nulos— para probar Medir sin
// depender de ffmpeg ni de un HDHomeRun de verdad. No hay TSDuck (ADR 0003)
// ni CRC32: como internal/ts, esto lee y escribe lo justo para la prueba.

const (
	pidPMTSintetico   = uint16(0x1000)
	pidVideoSintetico = uint16(0x0101)
	pidAudioSintetico = uint16(0x0102)
	programaSintetico = 1
)

// pkt47 arma la cabecera de un paquete TS de 188 bytes, con el resto en
// ceros.
func pkt47(pid uint16, pusi bool, afc byte, cc byte) []byte {
	p := make([]byte, ts.PacketSize)
	p[0] = 0x47
	p[1] = byte(pid>>8) & 0x1f
	if pusi {
		p[1] |= 0x40
	}
	p[2] = byte(pid)
	p[3] = (afc << 4) | (cc & 0x0f)
	return p
}

// seccionPaquete mete una sección PSI (PAT o PMT) en un paquete con PUSI,
// pointer_field en 0, y el resto relleno de 0xFF.
func seccionPaquete(pid uint16, cc byte, seccion []byte) []byte {
	p := pkt47(pid, true, 1, cc)
	off := 4
	p[off] = 0x00 // pointer_field: la sección empieza aquí mismo
	off++
	copy(p[off:], seccion)
	off += len(seccion)
	for ; off < len(p); off++ {
		p[off] = 0xff
	}
	return p
}

// pesquetePayload es un paquete de puro payload (afc=01), relleno con un
// byte fijo para distinguir a simple vista video de audio si hace falta.
func pesquetePayload(pid uint16, cc byte, relleno byte) []byte {
	p := pkt47(pid, false, 1, cc)
	for i := 4; i < len(p); i++ {
		p[i] = relleno
	}
	return p
}

// pesqueteConPCR es un paquete con adaptation field de solo PCR (7 bytes:
// flags + base + extensión), seguido del payload. El PCR va en segundos —lo
// que ts.Analyze espera decodificar de vuelta con la misma cuenta.
func pesqueteConPCR(pid uint16, cc byte, pcrSegundos float64, relleno byte) []byte {
	p := pkt47(pid, false, 3, cc) // afc=11: adaptation + payload
	const al = 7                  // flags(1) + base+extensión PCR (6)
	p[4] = al
	p[5] = 0x10 // PCR_flag
	base := uint64(pcrSegundos * 90000)
	p[6] = byte(base >> 25)
	p[7] = byte(base >> 17)
	p[8] = byte(base >> 9)
	p[9] = byte(base >> 1)
	p[10] = byte((base&1)<<7) | 0x7e // reservados en 1, extensión en 0
	p[11] = 0x00
	off := 4 + 1 + al
	for i := off; i < len(p); i++ {
		p[i] = relleno
	}
	return p
}

func pesqueteNulo() []byte { return pkt47(0x1fff, false, 1, 0) }

// construirPAT arma una sección PAT con un solo programa.
func construirPAT(numeroPrograma int, pmtPID uint16) []byte {
	b := []byte{
		0x00,       // table_id
		0x00, 0x00, // section_length: se llena abajo
		0x00, 0x01, // transport_stream_id
		0xc1,       // reservado(2) + version(5)=0 + current_next=1
		0x00, 0x00, // section_number, last_section_number
		byte(numeroPrograma >> 8), byte(numeroPrograma),
		0xe0 | byte(pmtPID>>8), byte(pmtPID),
		0xff, 0xff, 0xff, 0xff, // CRC32: no se valida ni aquí ni en internal/ts
	}
	seccionLen := len(b) - 3
	b[1] = 0xb0 | byte(seccionLen>>8&0x0f)
	b[2] = byte(seccionLen)
	return b
}

type flujoPMT struct {
	Tipo byte
	PID  uint16
}

// construirPMT arma una sección PMT con los flujos dados, sin descriptores.
func construirPMT(numeroPrograma int, pcrPID uint16, flujos []flujoPMT) []byte {
	b := []byte{
		0x02,       // table_id
		0x00, 0x00, // section_length: se llena abajo
		byte(numeroPrograma >> 8), byte(numeroPrograma),
		0xc1,
		0x00, 0x00,
		0xe0 | byte(pcrPID>>8), byte(pcrPID),
		0xf0, 0x00, // program_info_length = 0
	}
	for _, f := range flujos {
		b = append(b, f.Tipo, 0xe0|byte(f.PID>>8), byte(f.PID), 0xf0, 0x00)
	}
	b = append(b, 0xff, 0xff, 0xff, 0xff) // CRC32
	seccionLen := len(b) - 3
	b[1] = 0xb0 | byte(seccionLen>>8&0x0f)
	b[2] = byte(seccionLen)
	return b
}

// construirTSSintetico arma paquetes*2 de video/audio con PCR cada diez
// paquetes de video y algún paquete nulo intercalado, sin errores de
// continuidad. Es a propósito una sola función de una franja: las pruebas
// que quieren un defecto (CC roto, sin PCR, sin audio) parten de aquí y lo
// tuercen.
func construirTSSintetico(paquetesPorPID int) []byte {
	var buf bytes.Buffer
	buf.Write(seccionPaquete(0x0000, 0, construirPAT(programaSintetico, pidPMTSintetico)))
	buf.Write(seccionPaquete(pidPMTSintetico, 0, construirPMT(programaSintetico, pidVideoSintetico, []flujoPMT{
		{Tipo: 0x02, PID: pidVideoSintetico}, // MPEG-2 video
		{Tipo: 0x81, PID: pidAudioSintetico}, // AC-3 audio
	})))

	var vcc, acc byte
	for i := 0; i < paquetesPorPID; i++ {
		if i%10 == 0 {
			buf.Write(pesqueteConPCR(pidVideoSintetico, vcc, float64(i)/10, 0xaa))
		} else {
			buf.Write(pesquetePayload(pidVideoSintetico, vcc, 0xaa))
		}
		vcc = (vcc + 1) & 0x0f

		buf.Write(pesquetePayload(pidAudioSintetico, acc, 0xbb))
		acc = (acc + 1) & 0x0f

		if i%20 == 0 {
			buf.Write(pesqueteNulo())
		}
	}
	return buf.Bytes()
}
