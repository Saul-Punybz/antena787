// mensajes.go es la capa de bytes: los dos sobres que define SCTE 104
// —single_operation_message y multiple_operation_message—, el encuadre con el
// que viajan por TCP, y las operaciones que van dentro del segundo.
//
// Todo entero sin signo va en **big-endian** (network byte order), que es lo
// que hacen las dos implementaciones de referencia y lo único que tiene
// sentido en un protocolo de red de 2004. No hay campos que no estén alineados
// a byte, así que no hace falta un lector de bits: todo es
// `binary.BigEndian` sobre un `[]byte`.
package scte104

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
)

// CabeceraSencillaBytes es el largo fijo de la cabecera del
// single_operation_message: opID 2 + message_size 2 + result 2 +
// result_extension 2 + protocol_version 1 + AS_index 1 + message_number 1 +
// DPI_PID_index 2 = **13 bytes**. Es el SINGLE_OPERATION_HEADER_SIZE de la
// referencia de TypeScript y el mismo orden que lee libklvanc.
const CabeceraSencillaBytes = 13

// CabeceraMultipleBytes es lo fijo del multiple_operation_message *antes* de la
// marca de tiempo: reserved 2 + message_size 2 + protocol_version 1 +
// AS_index 1 + message_number 1 + DPI_PID_index 2 + SCTE35_protocol_version 1
// = **10 bytes**. Después van la marca (de largo variable, ver Marca), el
// num_ops de un byte, y las operaciones.
const CabeceraMultipleBytes = 10

// ErrMarcoIncompleto dice que en el búfer no hay todavía un mensaje entero.
// No es un error del protocolo: es lo normal al leer de un socket.
var ErrMarcoIncompleto = errors.New("scte104: el marco está incompleto")

// LeerMarco lee de un flujo TCP un mensaje completo y devuelve sus bytes tal
// como llegaron.
//
// **El encuadre no lleva un prefijo de largo aparte: el largo es parte del
// mensaje.** Los bytes 0-1 son el opID (o el 0xFFFF que anuncia un mensaje
// múltiple) y los bytes 2-3 son el `message_size`, que cuenta el mensaje
// **entero, incluyéndose a sí mismo y al opID**. Así que leer un marco es leer
// cuatro bytes, mirar el tamaño, y leer el resto. Es lo que hacen el cliente y
// el servidor de la referencia de TypeScript, y lo que confirma libklvanc al
// recalcular `message_size` con el tamaño del búfer completo al final de
// `klvanc_convert_SCTE_104_to_packetBytes`.
//
// Un mensaje con `message_size` menor que 4 es imposible y se rechaza: si no,
// un inyector averiado —o cualquiera que consiga escribir en el socket— dejaría
// el lector girando sin avanzar nunca.
func LeerMarco(r io.Reader) ([]byte, error) {
	var cab [4]byte
	if _, err := io.ReadFull(r, cab[:]); err != nil {
		return nil, err
	}
	tam := int(binary.BigEndian.Uint16(cab[2:4]))
	if tam < len(cab) {
		return nil, fmt.Errorf("scte104: message_size %d es imposible (el mínimo es %d)", tam, len(cab))
	}
	b := make([]byte, tam)
	copy(b, cab[:])
	if _, err := io.ReadFull(r, b[len(cab):]); err != nil {
		return nil, err
	}
	return b, nil
}

// EsMultiple dice si un marco ya leído es un multiple_operation_message. Es lo
// primero que hay que preguntar: los dos sobres comparten los cuatro primeros
// bytes y nada más.
func EsMultiple(marco []byte) bool {
	return len(marco) >= 2 && binary.BigEndian.Uint16(marco[0:2]) == OpMultiple
}

// Sencillo es el **single_operation_message** (SCTE 104, tabla 7-1): una sola
// operación por mensaje. Es el sobre de todo el trámite del enlace
// (init, alive, config, fault) y el único que el inyector nos manda de vuelta.
// El corte no cabe aquí: va en un Multiple.
type Sencillo struct {
	// OpID es qué mensaje es (las constantes Op*).
	OpID uint16
	// Resultado es el `result`: qué contestó el inyector. En una petición que
	// sale de aquí va ResultadoNoUsado.
	Resultado uint16
	// ResultadoExtra es el `result_extension`, el detalle del resultado
	// cuando el código solo no alcanza. También ResultadoNoUsado al salir.
	ResultadoExtra uint16
	// Version es el `protocol_version`. 0 es la versión de SCTE 104 que
	// implementan las dos referencias libres; un inyector que quiera otra lo
	// dice con ResultadoVersionDesigual.
	Version uint8
	// IndiceAS es el `AS_index`: qué sistema de automatización es este, cuando
	// varios comparten un inyector. 0 mientras el inyector no nos asigne otro
	// en un config_response.
	IndiceAS uint8
	// Numero es el `message_number`: el correlativo con el que se empareja una
	// respuesta con su petición. Es de **un byte**, así que da la vuelta cada
	// 256 mensajes; el cliente lo tiene en cuenta.
	Numero uint8
	// IndicePID es el `DPI_PID_index`: a qué flujo de cue del transport
	// stream va el corte, cuando el inyector maneja varios. 0 es el de
	// siempre para un canal único.
	IndicePID uint16
	// Datos es el cuerpo propio de cada opID, después de los 13 bytes de
	// cabecera. Para init_request y init_response está vacío; para alive lleva
	// la hora; para inject_response lleva el número del mensaje que se acusa.
	Datos []byte
}

// Codificar arma los bytes del mensaje, con el `message_size` ya calculado.
func (m Sencillo) Codificar() ([]byte, error) {
	tam := CabeceraSencillaBytes + len(m.Datos)
	if tam > 0xFFFF {
		return nil, fmt.Errorf("scte104: el mensaje mide %d bytes y el message_size solo llega a %d", tam, 0xFFFF)
	}
	if m.OpID == OpMultiple {
		return nil, errors.New("scte104: el opID 0xFFFF es el del mensaje múltiple, no cabe en un single_operation_message")
	}
	b := make([]byte, tam)
	binary.BigEndian.PutUint16(b[0:2], m.OpID)
	binary.BigEndian.PutUint16(b[2:4], uint16(tam))
	binary.BigEndian.PutUint16(b[4:6], m.Resultado)
	binary.BigEndian.PutUint16(b[6:8], m.ResultadoExtra)
	b[8] = m.Version
	b[9] = m.IndiceAS
	b[10] = m.Numero
	binary.BigEndian.PutUint16(b[11:13], m.IndicePID)
	copy(b[CabeceraSencillaBytes:], m.Datos)
	return b, nil
}

// DecodificarSencillo lee un marco completo como single_operation_message.
//
// Es estricto con el tamaño a propósito: si el `message_size` declarado no es
// exactamente el largo del marco, el mensaje no se interpreta. Un inyector que
// manda un tamaño que no cuadra está averiado, y adivinar por él es cómo se
// mete un corte en el aire equivocado.
func DecodificarSencillo(b []byte) (Sencillo, error) {
	if len(b) < CabeceraSencillaBytes {
		return Sencillo{}, fmt.Errorf("scte104: un single_operation_message mide al menos %d bytes, llegaron %d", CabeceraSencillaBytes, len(b))
	}
	op := binary.BigEndian.Uint16(b[0:2])
	if op == OpMultiple {
		return Sencillo{}, errors.New("scte104: esto es un multiple_operation_message, no un single_operation_message")
	}
	tam := int(binary.BigEndian.Uint16(b[2:4]))
	if tam != len(b) {
		return Sencillo{}, fmt.Errorf("scte104: el message_size dice %d bytes y el marco trae %d", tam, len(b))
	}
	m := Sencillo{
		OpID:           op,
		Resultado:      binary.BigEndian.Uint16(b[4:6]),
		ResultadoExtra: binary.BigEndian.Uint16(b[6:8]),
		Version:        b[8],
		IndiceAS:       b[9],
		Numero:         b[10],
		IndicePID:      binary.BigEndian.Uint16(b[11:13]),
	}
	// La copia no es paranoia: `b` suele ser el búfer del socket, y Datos se
	// queda guardado en el mapa de esperas del cliente.
	if len(b) > CabeceraSencillaBytes {
		m.Datos = append([]byte(nil), b[CabeceraSencillaBytes:]...)
	}
	return m, nil
}

// NumeroAcusado dice a qué `message_number` responde este mensaje, que es lo
// que hace falta para emparejar una respuesta con su petición.
//
// Para casi todos es el `message_number` de la propia cabecera, que el inyector
// devuelve igual. Pero inject_response_data e inject_complete_response_data
// llevan **otro** message_number en el cuerpo —el del
// multiple_operation_message que están acusando—, y ese es el que vale: la
// cabecera de la respuesta lleva el correlativo del inyector, no el nuestro.
func (m Sencillo) NumeroAcusado() uint8 {
	switch m.OpID {
	case OpInyectaRespuesta, OpInyectaCompleta:
		if len(m.Datos) > 0 {
			return m.Datos[0]
		}
	}
	return m.Numero
}

// CuentaDeCue es el `cue_message_count` del inject_complete_response_data:
// cuántos mensajes de cue acabó inyectando el encoder. El segundo valor es
// falso si el mensaje no es ese o viene corto.
func (m Sencillo) CuentaDeCue() (uint8, bool) {
	if m.OpID != OpInyectaCompleta || len(m.Datos) < 2 {
		return 0, false
	}
	return m.Datos[1], true
}

// InicioPeticion arma un **init_request_data** (opID 0x0001): lo primero que
// manda el sistema de automatización al abrir el socket. No tiene campos
// propios; lo que dice está todo en la cabecera (quién soy, con qué versión,
// sobre qué DPI PID).
func InicioPeticion(numero uint8, indicePID uint16) Sencillo {
	return Sencillo{
		OpID:           OpInicioPeticion,
		Resultado:      ResultadoNoUsado,
		ResultadoExtra: ResultadoNoUsado,
		Numero:         numero,
		IndicePID:      indicePID,
	}
}

// InicioRespuesta arma un **init_response_data** (opID 0x0002). Lo manda el
// inyector, no nosotros; está aquí porque el servidor de prueba lo necesita y
// porque tener los dos lados escritos es lo que hace que las pruebas cubran el
// diálogo entero.
func InicioRespuesta(numero uint8, indicePID uint16, resultado uint16) Sencillo {
	return Sencillo{
		OpID:           OpInicioRespuesta,
		Resultado:      resultado,
		ResultadoExtra: ResultadoNoUsado,
		Numero:         numero,
		IndicePID:      indicePID,
	}
}

// Tiempo son los dos campos de hora del alive_request_data y del
// alive_response_data (opID 0x0003 y 0x0004): 8 bytes, segundos de 32 bits más
// microsegundos de 32 bits.
//
// **Verificado solo contra la referencia de TypeScript** (`Time` en
// `src/syntax.ts`, 32 + 32 bits). libklvanc no implementa el alive porque
// lleva los mensajes por VANC y ahí no hay enlace que mantener vivo, así que
// no hay con qué cotejarlo, y el estándar no se puede leer. Si un inyector
// real contesta otra cosa, el cliente lo va a ver como un marco de largo
// inesperado y lo va a decir en UltimoError: es un campo, no un rediseño.
//
// El cero de la cuenta es la **época de GPS** (6 de enero de 1980, 00:00 UTC),
// que es la misma que usa SCTE 35 para su hora UTC y la que calcula el cliente
// de la referencia. Ver EpocaGPS.
type Tiempo struct {
	// Segundos son los segundos enteros desde la época de GPS.
	Segundos uint32
	// Microsegundos es la fracción, de 0 a 999999.
	Microsegundos uint32
}

// TiempoBytes es el largo del cuerpo de un alive: 4 + 4.
const TiempoBytes = 8

func (t Tiempo) codificar() []byte {
	b := make([]byte, TiempoBytes)
	binary.BigEndian.PutUint32(b[0:4], t.Segundos)
	binary.BigEndian.PutUint32(b[4:8], t.Microsegundos)
	return b
}

func decodificarTiempo(b []byte) (Tiempo, error) {
	if len(b) < TiempoBytes {
		return Tiempo{}, fmt.Errorf("scte104: la hora del alive mide %d bytes, llegaron %d", TiempoBytes, len(b))
	}
	return Tiempo{
		Segundos:      binary.BigEndian.Uint32(b[0:4]),
		Microsegundos: binary.BigEndian.Uint32(b[4:8]),
	}, nil
}

// VivoPeticion arma un **alive_request_data** (opID 0x0003): el latido que
// mantiene el enlace declarado en pie. Lleva la hora del automatismo para que
// el inyector sepa si los dos relojes están de acuerdo.
func VivoPeticion(numero uint8, indicePID uint16, t Tiempo) Sencillo {
	return Sencillo{
		OpID:           OpVivoPeticion,
		Resultado:      ResultadoNoUsado,
		ResultadoExtra: ResultadoNoUsado,
		Numero:         numero,
		IndicePID:      indicePID,
		Datos:          t.codificar(),
	}
}

// VivoRespuesta arma un **alive_response_data** (opID 0x0004). Lo manda el
// inyector; un inyector real devuelve la hora que recibió.
func VivoRespuesta(numero uint8, indicePID uint16, t Tiempo) Sencillo {
	return Sencillo{
		OpID:           OpVivoRespuesta,
		Resultado:      ResultadoExito,
		ResultadoExtra: ResultadoNoUsado,
		Numero:         numero,
		IndicePID:      indicePID,
		Datos:          t.codificar(),
	}
}

// Tiempo saca la hora del cuerpo de un alive_request o un alive_response.
func (m Sencillo) Tiempo() (Tiempo, error) {
	switch m.OpID {
	case OpVivoPeticion, OpVivoRespuesta, OpASVivoPeticion, OpASVivoRespuesta:
	default:
		return Tiempo{}, fmt.Errorf("scte104: %s no lleva hora", NombreDeOp(m.OpID))
	}
	return decodificarTiempo(m.Datos)
}

// InyectaRespuesta arma un **inject_response_data** (opID 0x0007): el acuse de
// un multiple_operation_message. El cuerpo es el message_number del mensaje
// que se acusa (ver NumeroAcusado).
func InyectaRespuesta(numero uint8, indicePID uint16, resultado uint16, acusa uint8) Sencillo {
	return Sencillo{
		OpID:           OpInyectaRespuesta,
		Resultado:      resultado,
		ResultadoExtra: ResultadoNoUsado,
		Numero:         numero,
		IndicePID:      indicePID,
		Datos:          []byte{acusa},
	}
}

// InyectaCompleta arma un **inject_complete_response_data** (opID 0x0008): «ya
// lo inyecté», con el número del mensaje acusado y cuántos mensajes de cue
// salieron.
func InyectaCompleta(numero uint8, indicePID uint16, resultado uint16, acusa, cue uint8) Sencillo {
	return Sencillo{
		OpID:           OpInyectaCompleta,
		Resultado:      resultado,
		ResultadoExtra: ResultadoNoUsado,
		Numero:         numero,
		IndicePID:      indicePID,
		Datos:          []byte{acusa, cue},
	}
}

// TipoTiempo es el `time_type` de la marca de tiempo de un
// multiple_operation_message (SCTE 104, tabla 11-2): con qué reloj se dice
// *cuándo* se ejecuta la operación.
type TipoTiempo uint8

const (
	// TiempoNinguno — time_type 0: sin hora. La operación se ejecuta con el
	// pre-roll que traiga ella misma, contado desde que llega. Es el modo que
	// usa un playout que no tiene timecode que ofrecer, y es el que usa este
	// paquete por defecto.
	TiempoNinguno TipoTiempo = 0
	// TiempoUTC — time_type 1: hora UTC, con segundos y microsegundos.
	TiempoUTC TipoTiempo = 1
	// TiempoVITC — time_type 2: timecode SMPTE VITC (horas, minutos,
	// segundos, cuadros). Es el reloj de una cadena de SDI.
	TiempoVITC TipoTiempo = 2
	// TiempoGPI — time_type 3: la operación se dispara con el flanco de una
	// entrada GPI del propio inyector, no con una hora. Es lo que da
	// sincronía de cuadro exacta sin depender de relojes.
	TiempoGPI TipoTiempo = 3
)

// Los dos flancos que reconoce el time_type 3.
const (
	// FlancoCierra — 0x00: de abierto a cerrado.
	FlancoCierra = 0x00
	// FlancoAbre — 0x01: de cerrado a abierto.
	FlancoAbre = 0x01
)

// Marca es la marca de tiempo del multiple_operation_message: un byte de tipo
// y, según el tipo, ninguno, seis, cuatro o dos bytes más. Los campos que no
// son del tipo elegido se ignoran al codificar y quedan en cero al decodificar.
type Marca struct {
	// Tipo dice cuáles de los campos de abajo valen.
	Tipo TipoTiempo
	// Segundos y Microsegundos son del TiempoUTC: `UTC_seconds` (32 bits) y
	// `UTC_microseconds` (**16 bits**).
	//
	// Los 16 bits de los microsegundos no dan para 999999, y aun así son 16:
	// lo dicen las dos referencias en su código —`unsigned short
	// UTC_microseconds` en libklvanc citando la tabla 11-2, y `@Field(16)` en
	// la de TypeScript—. El detalle importa porque el **segundo vector de
	// prueba** de la referencia de TypeScript solo cuadra si se leen 32, y por
	// eso está mal: ver TestVectorExternoDos.
	Segundos      uint32
	Microsegundos uint16
	// Horas, Minutos, SegundosVITC y Cuadros son del TiempoVITC.
	Horas        uint8
	Minutos      uint8
	SegundosVITC uint8
	Cuadros      uint8
	// NumeroGPI y FlancoGPI son del TiempoGPI: qué entrada del inyector y con
	// qué flanco (las constantes Flanco*).
	NumeroGPI uint8
	FlancoGPI uint8
}

// SinMarca es la marca que se manda cuando el corte se ejecuta al llegar, con
// su propio pre-roll: time_type 0.
var SinMarca = Marca{Tipo: TiempoNinguno}

func (t Marca) codificar() ([]byte, error) {
	switch t.Tipo {
	case TiempoNinguno:
		return []byte{byte(TiempoNinguno)}, nil
	case TiempoUTC:
		b := make([]byte, 7)
		b[0] = byte(TiempoUTC)
		binary.BigEndian.PutUint32(b[1:5], t.Segundos)
		binary.BigEndian.PutUint16(b[5:7], t.Microsegundos)
		return b, nil
	case TiempoVITC:
		return []byte{byte(TiempoVITC), t.Horas, t.Minutos, t.SegundosVITC, t.Cuadros}, nil
	case TiempoGPI:
		return []byte{byte(TiempoGPI), t.NumeroGPI, t.FlancoGPI}, nil
	default:
		return nil, fmt.Errorf("scte104: time_type %d no existe (0 ninguno, 1 UTC, 2 VITC, 3 GPI)", t.Tipo)
	}
}

// decodificarMarca lee la marca y devuelve cuántos bytes ocupó.
func decodificarMarca(b []byte) (Marca, int, error) {
	if len(b) < 1 {
		return Marca{}, 0, errors.New("scte104: falta el time_type de la marca de tiempo")
	}
	t := Marca{Tipo: TipoTiempo(b[0])}
	corto := func(n int) error {
		return fmt.Errorf("scte104: la marca de time_type %d necesita %d bytes y quedan %d", t.Tipo, n, len(b))
	}
	switch t.Tipo {
	case TiempoNinguno:
		return t, 1, nil
	case TiempoUTC:
		if len(b) < 7 {
			return Marca{}, 0, corto(7)
		}
		t.Segundos = binary.BigEndian.Uint32(b[1:5])
		t.Microsegundos = binary.BigEndian.Uint16(b[5:7])
		return t, 7, nil
	case TiempoVITC:
		if len(b) < 5 {
			return Marca{}, 0, corto(5)
		}
		t.Horas, t.Minutos, t.SegundosVITC, t.Cuadros = b[1], b[2], b[3], b[4]
		return t, 5, nil
	case TiempoGPI:
		if len(b) < 3 {
			return Marca{}, 0, corto(3)
		}
		t.NumeroGPI, t.FlancoGPI = b[1], b[2]
		return t, 3, nil
	default:
		return Marca{}, 0, fmt.Errorf("scte104: time_type %d no existe (0 ninguno, 1 UTC, 2 VITC, 3 GPI)", t.Tipo)
	}
}

// Operacion es una operación de dentro de un multiple_operation_message: su
// opID (las constantes Mop*) y su cuerpo ya codificado. El `data_length` de
// dos bytes que va en el cable se calcula solo.
//
// Se guarda el cuerpo en crudo a propósito: así una operación que este paquete
// no conoce —una de fabricante, o una que añada una edición futura del
// estándar— se puede mandar y recibir sin tocar el código.
type Operacion struct {
	OpID  uint16
	Datos []byte
}

// Multiple es el **multiple_operation_message** (SCTE 104, tabla 7-2): varias
// operaciones que el inyector ejecuta como un solo acto, con una marca de
// tiempo común. Es el sobre del corte, y el que agrupa los spots de un mismo
// bloque en un solo SCTE-104 (PRD §13, `break_marker`).
type Multiple struct {
	// Version es el `protocol_version` de SCTE 104.
	Version uint8
	// IndiceAS es el `AS_index`.
	IndiceAS uint8
	// Numero es el `message_number` con el que se empareja el acuse.
	Numero uint8
	// IndicePID es el `DPI_PID_index`.
	IndicePID uint16
	// VersionSCTE35 es el `SCTE35_protocol_version`: la versión de SCTE 35 con
	// la que el encoder tiene que escribir la sección. 0 es «la que uses».
	VersionSCTE35 uint8
	// Marca dice cuándo se ejecutan las operaciones.
	Marca Marca
	// Operaciones son las operaciones, en orden. El `num_ops` del cable es un
	// byte, así que caben 255.
	Operaciones []Operacion
}

// Codificar arma los bytes del mensaje múltiple, con el `message_size`, el
// `num_ops` y cada `data_length` ya calculados.
func (m Multiple) Codificar() ([]byte, error) {
	if len(m.Operaciones) > 0xFF {
		return nil, fmt.Errorf("scte104: %d operaciones no caben en el num_ops de un byte (máximo %d)", len(m.Operaciones), 0xFF)
	}
	marca, err := m.Marca.codificar()
	if err != nil {
		return nil, err
	}
	tam := CabeceraMultipleBytes + len(marca) + 1
	for _, o := range m.Operaciones {
		if len(o.Datos) > 0xFFFF {
			return nil, fmt.Errorf("scte104: la operación %s trae %d bytes y el data_length solo llega a %d",
				NombreDeMop(o.OpID), len(o.Datos), 0xFFFF)
		}
		tam += 4 + len(o.Datos)
	}
	if tam > 0xFFFF {
		return nil, fmt.Errorf("scte104: el mensaje mide %d bytes y el message_size solo llega a %d", tam, 0xFFFF)
	}
	b := make([]byte, 0, tam)
	// El campo se llama `reserved` en el mensaje múltiple y su valor fijo
	// 0xFFFF es lo único que lo distingue de un single_operation_message.
	b = binary.BigEndian.AppendUint16(b, OpMultiple)
	b = binary.BigEndian.AppendUint16(b, uint16(tam))
	b = append(b, m.Version, m.IndiceAS, m.Numero)
	b = binary.BigEndian.AppendUint16(b, m.IndicePID)
	b = append(b, m.VersionSCTE35)
	b = append(b, marca...)
	b = append(b, byte(len(m.Operaciones)))
	for _, o := range m.Operaciones {
		b = binary.BigEndian.AppendUint16(b, o.OpID)
		b = binary.BigEndian.AppendUint16(b, uint16(len(o.Datos)))
		b = append(b, o.Datos...)
	}
	return b, nil
}

// DecodificarMultiple lee un marco completo como multiple_operation_message.
//
// Es estricto en las dos direcciones: el `message_size` tiene que ser el largo
// del marco, y las operaciones tienen que consumirlo **exactamente**. Un marco
// al que le sobran bytes después de la última operación no se acepta, porque
// eso es justo lo que pasa cuando uno de los dos lados entendió distinto el
// largo de un campo, y callarlo es cómo se manda un corte con la duración de
// otro. (Es exactamente el caso del segundo vector de la referencia de
// TypeScript: ver TestVectorExternoDos.)
func DecodificarMultiple(b []byte) (Multiple, error) {
	if len(b) < CabeceraMultipleBytes+1 {
		return Multiple{}, fmt.Errorf("scte104: un multiple_operation_message mide al menos %d bytes, llegaron %d", CabeceraMultipleBytes+1, len(b))
	}
	if r := binary.BigEndian.Uint16(b[0:2]); r != OpMultiple {
		return Multiple{}, fmt.Errorf("scte104: el campo reserved vale 0x%04X y en un mensaje múltiple vale 0x%04X", r, OpMultiple)
	}
	tam := int(binary.BigEndian.Uint16(b[2:4]))
	if tam != len(b) {
		return Multiple{}, fmt.Errorf("scte104: el message_size dice %d bytes y el marco trae %d", tam, len(b))
	}
	m := Multiple{
		Version:       b[4],
		IndiceAS:      b[5],
		Numero:        b[6],
		IndicePID:     binary.BigEndian.Uint16(b[7:9]),
		VersionSCTE35: b[9],
	}
	marca, n, err := decodificarMarca(b[CabeceraMultipleBytes:])
	if err != nil {
		return Multiple{}, err
	}
	m.Marca = marca
	p := CabeceraMultipleBytes + n
	if p >= len(b) {
		return Multiple{}, errors.New("scte104: falta el num_ops")
	}
	nops := int(b[p])
	p++
	for i := 0; i < nops; i++ {
		if p+4 > len(b) {
			return Multiple{}, fmt.Errorf("scte104: la operación %d de %d se corta en la cabecera", i+1, nops)
		}
		o := Operacion{OpID: binary.BigEndian.Uint16(b[p : p+2])}
		largo := int(binary.BigEndian.Uint16(b[p+2 : p+4]))
		p += 4
		if p+largo > len(b) {
			return Multiple{}, fmt.Errorf("scte104: la operación %d (%s) dice %d bytes y solo quedan %d",
				i+1, NombreDeMop(o.OpID), largo, len(b)-p)
		}
		if largo > 0 {
			o.Datos = append([]byte(nil), b[p:p+largo]...)
		}
		p += largo
		m.Operaciones = append(m.Operaciones, o)
	}
	if p != len(b) {
		return Multiple{}, fmt.Errorf("scte104: después de las %d operaciones sobran %d bytes; el mensaje no cuadra", nops, len(b)-p)
	}
	return m, nil
}

// TipoCorte es el `splice_insert_type` del splice_request_data (SCTE 104,
// tabla 8-5): qué se le pide al encoder.
type TipoCorte uint8

const (
	// CorteEmpiezaNormal — spliceStart_normal (1): abre el corte dentro de
	// `pre_roll_time` milisegundos. Es el modo de un playout que sabe con
	// cuánto tiempo va a llegar al corte, y es el único que le da al encoder
	// margen para poner el SCTE-35 antes del cuadro de salida.
	CorteEmpiezaNormal TipoCorte = 1
	// CorteEmpiezaYa — spliceStart_immediate (2): abre el corte ahora, sin
	// pre-roll. Es lo que se usa cuando el corte lo dispara una persona o una
	// alerta, y lo que hay que evitar cuando se puede planificar: sin pre-roll
	// el insertador de aguas abajo llega tarde.
	CorteEmpiezaYa TipoCorte = 2
	// CorteTerminaNormal — spliceEnd_normal (3): cierra el corte dentro de
	// `pre_roll_time` milisegundos.
	CorteTerminaNormal TipoCorte = 3
	// CorteTerminaYa — spliceEnd_immediate (4): cierra el corte ahora. Es lo
	// que manda TerminarCorte, porque el fin de un corte lo decide el
	// contenido que ya está saliendo, no un plan.
	CorteTerminaYa TipoCorte = 4
	// CorteCancela — splice_cancel (5): cancela un corte anunciado que
	// todavía no empezó, por su `splice_event_id`.
	CorteCancela TipoCorte = 5
)

var nombresDeTipoDeCorte = map[TipoCorte]string{
	CorteEmpiezaNormal: "spliceStart_normal",
	CorteEmpiezaYa:     "spliceStart_immediate",
	CorteTerminaNormal: "spliceEnd_normal",
	CorteTerminaYa:     "spliceEnd_immediate",
	CorteCancela:       "splice_cancel",
}

// String da el nombre del estándar, que es el que sale en el manual del
// encoder.
func (t TipoCorte) String() string {
	if n, ok := nombresDeTipoDeCorte[t]; ok {
		return n
	}
	return fmt.Sprintf("splice_insert_type %d (no definido)", uint8(t))
}

// CorteBytes es el largo fijo del splice_request_data: 1 + 4 + 2 + 2 + 2 + 1 +
// 1 + 1 = 14 bytes.
const CorteBytes = 14

// Corte es el **splice_request_data** (SCTE 104, tabla 8-5): la operación que
// de verdad marca el corte publicitario. Es el 99 % de lo que manda este
// driver.
type Corte struct {
	// Tipo es el `splice_insert_type`: empezar, terminar o cancelar.
	Tipo TipoCorte
	// EventoID es el `splice_event_id`: el identificador del corte. Tiene que
	// ser único mientras el corte esté vivo, porque es con él con lo que se
	// cierra y se cancela. Es el puente entre nuestro `break_marker` y lo que
	// aparece en el SCTE-35 del transport stream, así que es también lo que
	// permite comprobar aguas abajo que el corte salió (`signal-compare`,
	// ADR 0009).
	EventoID uint32
	// ProgramaID es el `unique_program_id`: qué programa es el que se está
	// cortando. Sirve para que un insertador que ve varios canales sepa de
	// quién es el avail.
	ProgramaID uint16
	// PreRoll es el `pre_roll_time` en **milisegundos**: cuánto falta para el
	// corte desde que el inyector recibe el mensaje.
	//
	// Las dos referencias no dicen lo mismo de la unidad: libklvanc comenta
	// «In 1/10's of a second» en este campo, pero la de TypeScript dice
	// milisegundos y añade que SCTE 104 manda no bajar de 4000 ms «siguiendo
	// el consejo de SCTE 67». El vector externo decide: trae 0x0FA0 = 4000, y
	// 4000 décimas de segundo serían 6 minutos y 40 segundos de pre-roll, que
	// no es un número que nadie ponga. Son **milisegundos**, y el comentario
	// de libklvanc en este campo está mal (en el de time_signal_request_data
	// el mismo proyecto sí dice milisegundos).
	PreRoll uint16
	// Duracion es el `break_duration` en **décimas de segundo**: cuánto dura
	// el corte. Aquí las dos referencias sí coinciden. 0 significa que la
	// duración no se anuncia y el corte se cierra con un spliceEnd.
	Duracion uint16
	// AvailNum es el `avail_num`: qué avail de los del bloque es este. 0 si no
	// se numeran.
	AvailNum uint8
	// AvailsEsperados es el `avails_expected`: cuántos avails trae el bloque.
	AvailsEsperados uint8
	// AutoReturn es el `auto_return_flag`: si vale 1, el corte se cierra solo
	// al cumplirse Duracion, sin que haga falta mandar un spliceEnd. Es el
	// modo seguro: si el playout se muere en mitad del corte, el aire vuelve
	// igual. Con 0 el corte se queda abierto hasta que alguien lo cierre.
	AutoReturn uint8
}

// Codificar arma los 14 bytes del splice_request_data.
func (c Corte) Codificar() []byte {
	b := make([]byte, CorteBytes)
	b[0] = byte(c.Tipo)
	binary.BigEndian.PutUint32(b[1:5], c.EventoID)
	binary.BigEndian.PutUint16(b[5:7], c.ProgramaID)
	binary.BigEndian.PutUint16(b[7:9], c.PreRoll)
	binary.BigEndian.PutUint16(b[9:11], c.Duracion)
	b[11] = c.AvailNum
	b[12] = c.AvailsEsperados
	b[13] = c.AutoReturn
	return b
}

// Operacion envuelve el corte para meterlo en un Multiple.
func (c Corte) Operacion() Operacion {
	return Operacion{OpID: MopCorte, Datos: c.Codificar()}
}

// DecodificarCorte lee un splice_request_data.
func DecodificarCorte(b []byte) (Corte, error) {
	if len(b) != CorteBytes {
		return Corte{}, fmt.Errorf("scte104: un splice_request_data mide %d bytes, llegaron %d", CorteBytes, len(b))
	}
	return Corte{
		Tipo:            TipoCorte(b[0]),
		EventoID:        binary.BigEndian.Uint32(b[1:5]),
		ProgramaID:      binary.BigEndian.Uint16(b[5:7]),
		PreRoll:         binary.BigEndian.Uint16(b[7:9]),
		Duracion:        binary.BigEndian.Uint16(b[9:11]),
		AvailNum:        b[11],
		AvailsEsperados: b[12],
		AutoReturn:      b[13],
	}, nil
}

// Corte saca el splice_request_data de una operación, si es de ese tipo.
func (o Operacion) Corte() (Corte, error) {
	if o.OpID != MopCorte {
		return Corte{}, fmt.Errorf("scte104: la operación es %s, no un splice_request_data", NombreDeMop(o.OpID))
	}
	return DecodificarCorte(o.Datos)
}

// CorteNulo es el **splice_null_request_data** (opID 0x0102): una operación sin
// campos, con la que se mantiene vivo el flujo de cue del transport stream sin
// anunciar ningún corte.
func CorteNulo() Operacion {
	return Operacion{OpID: MopCorteNulo}
}

// SenalDeHoraBytes es el largo del time_signal_request_data: 2 bytes.
const SenalDeHoraBytes = 2

// SenalDeHora es el **time_signal_request_data** (SCTE 104, tabla 8-23): marca
// un instante en el flujo sin abrir ni cerrar un corte. Es el `time_signal` de
// SCTE 35, el que se usa para marcar el principio de un programa o de un
// segmento cuando lo que hay que señalar no es un avail.
type SenalDeHora struct {
	// PreRoll es el `pre_roll_time` en **milisegundos**: cuánto falta para el
	// instante que se marca. Aquí las dos referencias coinciden en la unidad.
	PreRoll uint16
}

// Codificar arma los 2 bytes del time_signal_request_data.
func (s SenalDeHora) Codificar() []byte {
	b := make([]byte, SenalDeHoraBytes)
	binary.BigEndian.PutUint16(b, s.PreRoll)
	return b
}

// Operacion envuelve la señal de hora para meterla en un Multiple.
func (s SenalDeHora) Operacion() Operacion {
	return Operacion{OpID: MopSenalDeHora, Datos: s.Codificar()}
}

// DecodificarSenalDeHora lee un time_signal_request_data.
func DecodificarSenalDeHora(b []byte) (SenalDeHora, error) {
	if len(b) != SenalDeHoraBytes {
		return SenalDeHora{}, fmt.Errorf("scte104: un time_signal_request_data mide %d bytes, llegaron %d", SenalDeHoraBytes, len(b))
	}
	return SenalDeHora{PreRoll: binary.BigEndian.Uint16(b)}, nil
}

// DTMFMaximo es cuántos dígitos caben en un insert_DTMF_descriptor_request. El
// campo `dtmf_length` es de un byte y daría para 255, pero libklvanc reserva
// exactamente 7 caracteres (`char dtmf_char[7]`), que es lo que cabe en el
// descriptor de SCTE 35, así que 7 es el límite real del otro extremo.
const DTMFMaximo = 7

// DTMF es el **insert_DTMF_descriptor_request** (SCTE 104, tabla 8-28): los
// tonos DTMF con los que las redes disparan cortes en los sistemas de cabecera
// de toda la vida. Se manda junto con el corte cuando aguas abajo hay un
// insertador que escucha tonos en vez de leer SCTE-35.
type DTMF struct {
	// PreRoll es el `pre_roll_time` en **décimas de segundo** (un solo byte,
	// a diferencia del pre-roll del corte, que es de dos y va en
	// milisegundos). Las dos referencias coinciden en el tamaño.
	PreRoll uint8
	// Digitos son los caracteres DTMF (`dtmf_char`): los dígitos 0-9 más `*`
	// y `#`. Como máximo DTMFMaximo.
	Digitos string
}

// Codificar arma el insert_DTMF_descriptor_request.
func (d DTMF) Codificar() ([]byte, error) {
	if len(d.Digitos) > DTMFMaximo {
		return nil, fmt.Errorf("scte104: %q son %d dígitos DTMF y el descriptor aguanta %d", d.Digitos, len(d.Digitos), DTMFMaximo)
	}
	for _, r := range d.Digitos {
		if !strings.ContainsRune("0123456789*#", r) {
			return nil, fmt.Errorf("scte104: %q no es un dígito DTMF (solo 0-9, * y #)", r)
		}
	}
	b := make([]byte, 0, 2+len(d.Digitos))
	b = append(b, d.PreRoll, byte(len(d.Digitos)))
	b = append(b, d.Digitos...)
	return b, nil
}

// Operacion envuelve el DTMF para meterlo en un Multiple. Un DTMF inválido
// devuelve una operación vacía y el error sale al codificar el Multiple, así
// que se prefiere Codificar cuando hace falta ver el error en el sitio.
func (d DTMF) Operacion() (Operacion, error) {
	datos, err := d.Codificar()
	if err != nil {
		return Operacion{}, err
	}
	return Operacion{OpID: MopInsertaDTMF, Datos: datos}, nil
}

// DecodificarDTMF lee un insert_DTMF_descriptor_request.
func DecodificarDTMF(b []byte) (DTMF, error) {
	if len(b) < 2 {
		return DTMF{}, fmt.Errorf("scte104: un insert_DTMF_descriptor_request mide al menos 2 bytes, llegaron %d", len(b))
	}
	largo := int(b[1])
	if 2+largo != len(b) {
		return DTMF{}, fmt.Errorf("scte104: el dtmf_length dice %d dígitos y el cuerpo trae %d bytes", largo, len(b)-2)
	}
	return DTMF{PreRoll: b[0], Digitos: string(b[2:])}, nil
}

// InyectaSeccionMinimo es lo fijo del inject_section_data_request:
// SCTE35_command_length 2 + SCTE35_protocol_version 1 + SCTE35_command_type 1.
const InyectaSeccionMinimo = 4

// InyectaSeccion es el **inject_section_data_request** (opID 0x0100): mete en
// el flujo de cue una sección de SCTE-35 que armó el automatismo, tal cual,
// sin que el inyector la interprete. Es la puerta de escape para cuando hace
// falta algo que el protocolo no tiene.
//
// **Es el único mensaje de este paquete cuyo orden de campos no se pudo
// cotejar entre dos fuentes.** libklvanc lo nombra pero no lo interpreta, así
// que la única forma concreta que existe es `InjectSectionRequest` de
// `src/syntax.ts` de la referencia de TypeScript, que pone el
// `SCTE35_command_length` **antes** de la versión y el tipo. Es un orden raro
// —el largo ya está en el `data_length` de la operación— y bien podría ser al
// revés en el estándar. Si un inyector real lo rechaza con
// ResultadoSintaxisInvalida, este es el primer sitio donde mirar: mover dos
// campos de sitio y ya.
type InyectaSeccion struct {
	// Version es el `SCTE35_protocol_version`.
	Version uint8
	// TipoComando es el `SCTE35_command_type`: qué comando de SCTE 35 es el
	// que va en Comando (splice_insert, time_signal…).
	TipoComando uint8
	// Comando es el `SCTE35_command_contents`: el comando de SCTE 35 en
	// crudo.
	Comando []byte
}

// Codificar arma el inject_section_data_request.
func (s InyectaSeccion) Codificar() ([]byte, error) {
	if len(s.Comando) > 0xFFFF {
		return nil, fmt.Errorf("scte104: el comando de SCTE 35 trae %d bytes y el SCTE35_command_length llega a %d", len(s.Comando), 0xFFFF)
	}
	b := make([]byte, 0, InyectaSeccionMinimo+len(s.Comando))
	b = binary.BigEndian.AppendUint16(b, uint16(len(s.Comando)))
	b = append(b, s.Version, s.TipoComando)
	b = append(b, s.Comando...)
	return b, nil
}

// Operacion envuelve la sección para meterla en un Multiple.
func (s InyectaSeccion) Operacion() (Operacion, error) {
	datos, err := s.Codificar()
	if err != nil {
		return Operacion{}, err
	}
	return Operacion{OpID: MopInyectaSeccion, Datos: datos}, nil
}

// DecodificarInyectaSeccion lee un inject_section_data_request.
func DecodificarInyectaSeccion(b []byte) (InyectaSeccion, error) {
	if len(b) < InyectaSeccionMinimo {
		return InyectaSeccion{}, fmt.Errorf("scte104: un inject_section_data_request mide al menos %d bytes, llegaron %d", InyectaSeccionMinimo, len(b))
	}
	largo := int(binary.BigEndian.Uint16(b[0:2]))
	if InyectaSeccionMinimo+largo != len(b) {
		return InyectaSeccion{}, fmt.Errorf("scte104: el SCTE35_command_length dice %d bytes y el cuerpo trae %d", largo, len(b)-InyectaSeccionMinimo)
	}
	s := InyectaSeccion{Version: b[2], TipoComando: b[3]}
	if largo > 0 {
		s.Comando = append([]byte(nil), b[InyectaSeccionMinimo:]...)
	}
	return s, nil
}

// ComandoPropietarioMinimo es lo fijo del proprietary_command_request:
// proprietary_id 4 + proprietary_command 1. Todo lo que sigue son datos, y su
// largo se saca del `data_length` de la operación (es lo que hace libklvanc:
// `data_length = descriptor_size - 5`).
const ComandoPropietarioMinimo = 5

// ComandoPropietario es el **proprietary_command_request** (SCTE 104, tabla
// 9-30): la puerta que el estándar deja abierta para lo que cada fabricante
// quiera meter. Este paquete no interpreta nada de lo que va dentro; lo lleva
// y lo trae.
type ComandoPropietario struct {
	// ID es el `proprietary_id`: quién define el comando (normalmente el
	// identificador del fabricante).
	ID uint32
	// Comando es el `proprietary_command`: qué comando de los suyos es.
	Comando uint8
	// Datos es el `proprietary_data`, en crudo.
	Datos []byte
}

// Codificar arma el proprietary_command_request.
func (c ComandoPropietario) Codificar() []byte {
	b := make([]byte, 0, ComandoPropietarioMinimo+len(c.Datos))
	b = binary.BigEndian.AppendUint32(b, c.ID)
	b = append(b, c.Comando)
	b = append(b, c.Datos...)
	return b
}

// Operacion envuelve el comando propietario para meterlo en un Multiple.
func (c ComandoPropietario) Operacion() Operacion {
	return Operacion{OpID: MopComandoPropietario, Datos: c.Codificar()}
}

// DecodificarComandoPropietario lee un proprietary_command_request.
func DecodificarComandoPropietario(b []byte) (ComandoPropietario, error) {
	if len(b) < ComandoPropietarioMinimo {
		return ComandoPropietario{}, fmt.Errorf("scte104: un proprietary_command_request mide al menos %d bytes, llegaron %d", ComandoPropietarioMinimo, len(b))
	}
	c := ComandoPropietario{
		ID:      binary.BigEndian.Uint32(b[0:4]),
		Comando: b[4],
	}
	if len(b) > ComandoPropietarioMinimo {
		c.Datos = append([]byte(nil), b[ComandoPropietarioMinimo:]...)
	}
	return c, nil
}
