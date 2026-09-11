package hdhomerun

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"antena787/internal/ts"
)

// Medida es lo que Medir mide de un canal en una ventana de tiempo: lo que
// internal/ts ve del TS —PIDs, continuidad, PCR, tasa aproximada— más lo que
// hace falta para saber si el canal sirve de retorno de aire: el número de
// programa, si hay tipos de flujo de video y de audio, y el nivel de señal
// cuando el equipo lo entrega.
type Medida struct {
	ts.Report
	NumeroPrograma int
	HayVideo       bool
	HayAudio       bool
	Señal          *Señal
}

// Medir abre url, lee hasta duracion (o hasta que el canal se corte antes) y
// devuelve lo medido. No reintenta: a diferencia de Abrir, una medición
// fallida es información en sí misma —el canal no sirve así como está— y
// quien llama decide si lo intenta de nuevo.
func (r *Retorno) Medir(ctx context.Context, canalURL string, duracion time.Duration) (Medida, error) {
	ctxMedida, cancel := context.WithTimeout(ctx, duracion)
	defer cancel()

	req, err := http.NewRequestWithContext(ctxMedida, http.MethodGet, canalURL, nil)
	if err != nil {
		return Medida{}, err
	}
	resp, err := r.Cliente.http().Do(req)
	if err != nil {
		return Medida{}, fmt.Errorf("no pude abrir %s para medir: %w", canalURL, err)
	}
	defer resp.Body.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, resp.Body); err != nil && ctxMedida.Err() == nil {
		// Un error que no es "se acabó el tiempo de medir" es de verdad: la
		// conexión se cayó antes de terminar.
		return Medida{}, fmt.Errorf("%s se cortó midiendo: %w", canalURL, err)
	}
	if buf.Len() < ts.PacketSize {
		return Medida{}, fmt.Errorf("%s no mandó ni un paquete de TS completo en %s", canalURL, duracion)
	}

	informe, err := ts.Analyze(bytes.NewReader(buf.Bytes()))
	if err != nil {
		return Medida{}, fmt.Errorf("TS de %s no se pudo leer: %w", canalURL, err)
	}

	m := Medida{Report: informe}
	m.NumeroPrograma, m.HayVideo, m.HayAudio = leerPrograma(bytes.NewReader(buf.Bytes()))

	if host := r.hostSeñal(canalURL); host != "" {
		if señal, err := r.Cliente.obtenerSeñal(ctx, host); err == nil {
			m.Señal = señal
			r.marcar(func(e *Estado) { e.Señal = señal })
		}
	}

	return m, nil
}

// hostSeñal decide dónde pedir status.json: HostAPI si Retorno lo trae
// (normalmente Dispositivo.BaseURL, el host de verdad de la API), o si no,
// el propio host de la URL del canal como mejor esfuerzo. En la mayoría de
// los HDHomeRun el streaming va por un puerto distinto al de la API
// (5004 contra 80), así que sin HostAPI esto puede no encontrar nada — no es
// un error, es la razón por la que HostAPI existe.
func (r *Retorno) hostSeñal(canalURL string) string {
	if r.HostAPI != "" {
		return normalizarBase(r.HostAPI)
	}
	u, err := url.Parse(canalURL)
	if err != nil || u.Hostname() == "" {
		return ""
	}
	return "http://" + u.Hostname()
}

// leerPrograma busca en el TS el PAT y el PMT del primer programa: su número
// y si trae tipos de flujo de video y de audio. Como internal/ts, esto no es
// un demultiplexor completo —no comprueba CRC—: basta con saber que el canal
// trae imagen y sonido, que es lo que hace falta para decidir si sirve como
// retorno de aire.
func leerPrograma(rd io.Reader) (numeroPrograma int, hayVideo, hayAudio bool) {
	br := bufio.NewReaderSize(rd, 1<<20)
	var pkt [ts.PacketSize]byte
	var pmtPID uint16 = 0xffff
	patVisto := false

	for {
		if _, err := io.ReadFull(br, pkt[:]); err != nil {
			break
		}
		if pkt[0] != 0x47 {
			break
		}
		pid := uint16(pkt[1]&0x1f)<<8 | uint16(pkt[2])
		payload, ok := cargaUtilPSI(pkt[:])
		if !ok {
			continue
		}

		if !patVisto && pid == 0x0000 {
			if pn, pmt, ok := leerPAT(payload); ok {
				numeroPrograma, pmtPID = pn, pmt
				patVisto = true
			}
		}
		if patVisto && pid == pmtPID {
			hv, ha := leerPMT(payload)
			hayVideo = hayVideo || hv
			hayAudio = hayAudio || ha
			if hayVideo && hayAudio {
				break
			}
		}
	}
	return
}

// cargaUtilPSI devuelve el payload de un paquete con PUSI activo ya sin el
// pointer_field: el primer byte de una sección empieza justo después de él.
// Paquetes sin PUSI, o sin payload, se descartan — una sección PAT/PMT
// pequeña siempre empieza al principio de un paquete en la práctica.
func cargaUtilPSI(pkt []byte) ([]byte, bool) {
	if pkt[1]&0x40 == 0 { // sin PUSI: no es el principio de una sección
		return nil, false
	}
	afc := (pkt[3] >> 4) & 3
	if afc&1 == 0 { // sin payload
		return nil, false
	}
	off := 4
	if afc&2 == 2 {
		al := int(pkt[4])
		off += 1 + al
	}
	if off >= ts.PacketSize {
		return nil, false
	}
	ptr := int(pkt[off])
	off += 1 + ptr
	if off >= ts.PacketSize {
		return nil, false
	}
	return pkt[off:ts.PacketSize], true
}

// leerPAT lee una sección PAT ya sin pointer_field: el número del primer
// programa real (no la entrada de network_pid, que trae número 0) y el PID
// de su PMT.
func leerPAT(b []byte) (numeroPrograma int, pmtPID uint16, ok bool) {
	if len(b) < 8 || b[0] != 0x00 {
		return 0, 0, false
	}
	seccionLen := int(b[1]&0x0f)<<8 | int(b[2])
	fin := 3 + seccionLen - 4 // menos el CRC32 final
	if fin > len(b) {
		fin = len(b)
	}
	for i := 8; i+4 <= fin; i += 4 {
		pn := int(b[i])<<8 | int(b[i+1])
		pid := uint16(b[i+2]&0x1f)<<8 | uint16(b[i+3])
		if pn != 0 {
			return pn, pid, true
		}
	}
	return 0, 0, false
}

// leerPMT lee una sección PMT ya sin pointer_field: si alguno de sus flujos
// elementales es de video o de audio, por su stream_type (ISO 13818-1 y los
// añadidos habituales de ATSC: AC-3 0x81, E-AC-3 0x87).
func leerPMT(b []byte) (hayVideo, hayAudio bool) {
	if len(b) < 12 || b[0] != 0x02 {
		return false, false
	}
	seccionLen := int(b[1]&0x0f)<<8 | int(b[2])
	fin := 3 + seccionLen - 4
	if fin > len(b) {
		fin = len(b)
	}
	progInfoLen := int(b[10]&0x0f)<<8 | int(b[11])
	i := 12 + progInfoLen
	for i+5 <= fin {
		streamType := b[i]
		esInfoLen := int(b[i+3]&0x0f)<<8 | int(b[i+4])
		switch streamType {
		case 0x01, 0x02, 0x1b, 0x24: // MPEG-1/2, H.264, HEVC
			hayVideo = true
		case 0x03, 0x04, 0x0f, 0x81, 0x87: // MPEG audio, AAC ADTS, AC-3, E-AC-3
			hayAudio = true
		}
		i += 5 + esInfoLen
	}
	return
}

// estadoTunerJSON es el status.json que traen los HDHomeRun recientes por
// HTTP. A diferencia de discover.json y lineup.json, este endpoint NO está
// en la documentación pública de SiliconDust (aviso del paquete): la lectura
// es tolerante a que no exista o traiga otros campos.
type estadoTunerJSON struct {
	Lock                  string `json:"Lock"`
	SignalStrengthPercent *int   `json:"SignalStrengthPercent"`
	SignalQualityPercent  *int   `json:"SignalQualityPercent"`
	SymbolQualityPercent  *int   `json:"SymbolQualityPercent"`
}

// obtenerSeñal pide status.json a baseHost (una BaseURL, "http://ip[:puerto]")
// y devuelve el primer sintonizador que traiga algún número de señal. No es
// un error no encontrar nada: el llamador decide si lo trata como "sin dato".
func (c *Cliente) obtenerSeñal(ctx context.Context, baseHost string) (*Señal, error) {
	var tuners []estadoTunerJSON
	if err := c.obtenerJSON(ctx, baseHost+"/status.json", &tuners); err != nil {
		return nil, err
	}
	for _, t := range tuners {
		if t.SignalStrengthPercent == nil && t.SignalQualityPercent == nil && t.SymbolQualityPercent == nil {
			continue
		}
		s := &Señal{Bloqueo: t.Lock}
		if t.SignalStrengthPercent != nil {
			s.FuerzaPct = *t.SignalStrengthPercent
		}
		if t.SignalQualityPercent != nil {
			s.RuidoPct = *t.SignalQualityPercent
		}
		if t.SymbolQualityPercent != nil {
			s.SimboloPct = *t.SymbolQualityPercent
		}
		return s, nil
	}
	return nil, fmt.Errorf("status.json de %s no trae señal", baseHost)
}
