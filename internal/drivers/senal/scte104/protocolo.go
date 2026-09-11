// Package scte104 habla ANSI/SCTE 104 sobre TCP: el protocolo con el que un
// sistema de automatización —nosotros— le pide a un sistema de compresión —el
// encoder de la estación— que marque un corte publicitario. El encoder es el
// que escribe el SCTE-35 en el transport stream; Antena787 nunca lo escribe
// (ADR 0004, PRD §9 paso 7). Este es el driver `scte104-tcp` de la familia
// «señalización de cortes» del PRD §10.
//
// **No existe ninguna biblioteca de SCTE-104 en Go**, así que está escrito a
// mano (docs/drivers/catalogo/05-bibliotecas-go-protocolos.md). Tampoco existe
// el estándar en abierto: SCTE lo publica tras registro y el catálogo de SCTE
// responde 403 a cualquier lectura automática. Así que la forma exacta de cada
// mensaje se sacó de las dos únicas implementaciones libres que hay, leídas
// byte a byte y comparadas entre sí:
//
//   - `astronautlabs/scte104` (TypeScript, cliente y servidor TCP) —
//     https://github.com/astronautlabs/scte104, `src/syntax.ts` y
//     `src/protocol.ts`. Es la referencia del catálogo, y la única que
//     implementa el diálogo de TCP completo (init y alive).
//   - `stoth68000/libklvanc` (C, de Kernel Labs / LTN Global, en producción en
//     Open Broadcast Encoder) — `src/libklvanc/vanc-scte_104.h` y
//     `src/core-packet-scte_104.c`, que citan tabla por tabla la edición
//     ANSI/SCTE 104 2019a. Lleva el mismo mensaje por VANC (SMPTE 2010) en vez
//     de por TCP, pero el mensaje es el mismo.
//
// Las dos coinciden campo por campo en todo lo que este paquete codifica. Lo
// que NO coincide está anotado en el código, en el sitio exacto, con quién
// dice qué: es la única forma honesta de escribir un protocolo del que no se
// puede leer el estándar.
//
// El vocabulario del estándar se mantiene en los comentarios (`opID`,
// `splice_request_data`, `pre_roll_time`…) para que se pueda cotejar con un
// manual de encoder sin traducir nada en la cabeza.
package scte104

// PuertoPorDefecto es el puerto TCP en el que escucha un inyector de SCTE-104.
//
// **No es un número asignado por IANA.** Es el que documenta el datasheet del
// SCTE-104 Inserter de EEG/AI-Media, y el que trae por defecto la referencia
// de TypeScript; puede variar por equipo, así que la dirección del enlace
// siempre se configura y esto es solo el relleno cuando nadie pone puerto
// (docs/drivers/catalogo/03-multiplexores-psip-cortes.md).
const PuertoPorDefecto = 5167

// Los `opID` del single_operation_message (SCTE 104, tabla 7-1). Son los
// mensajes de trámite: presentarse, seguir vivo, y los acuses que manda el
// inyector. El corte de verdad no viaja en ninguno de estos, viaja en un
// multiple_operation_message.
const (
	// OpRespuestaGeneral — general_response_data. El «te oí» genérico del
	// inyector; su campo result es lo único que dice qué pasó.
	OpRespuestaGeneral = 0x0000
	// OpInicioPeticion — init_request_data. Lo primero que manda el sistema
	// de automatización al abrir el socket: sin esto el inyector no atiende
	// ninguna petición de corte.
	OpInicioPeticion = 0x0001
	// OpInicioRespuesta — init_response_data. El inyector acepta (result 100)
	// o dice por qué no.
	OpInicioRespuesta = 0x0002
	// OpVivoPeticion — alive_request_data. El latido que mantiene el enlace
	// declarado en pie; lleva la hora del automatismo.
	OpVivoPeticion = 0x0003
	// OpVivoRespuesta — alive_response_data. El latido de vuelta.
	OpVivoRespuesta = 0x0004
	// OpInyectaRespuesta — inject_response_data. Acuse de un
	// multiple_operation_message: «lo recibí y lo voy a inyectar».
	OpInyectaRespuesta = 0x0007
	// OpInyectaCompleta — inject_complete_response_data. «Ya lo inyecté», con
	// la cuenta de mensajes de cue que salieron.
	OpInyectaCompleta = 0x0008
	// OpConfigPeticion — config_request_data.
	OpConfigPeticion = 0x0009
	// OpConfigRespuesta — config_response_data.
	OpConfigRespuesta = 0x000A
	// OpProvisionPeticion — provisioning_request_data.
	OpProvisionPeticion = 0x000B
	// OpProvisionRespuesta — provisioning_response_data.
	OpProvisionRespuesta = 0x000C
	// OpFallaPeticion — fault_request_data.
	OpFallaPeticion = 0x000F
	// OpFallaRespuesta — fault_response_data.
	OpFallaRespuesta = 0x0010
	// OpASVivoPeticion — AS_alive_request_data: el latido en el otro sentido,
	// del inyector hacia el automatismo.
	OpASVivoPeticion = 0x0011
	// OpASVivoRespuesta — AS_alive_response_data.
	OpASVivoRespuesta = 0x0012
	// OpMultiple — el valor 0xFFFF en el lugar del opID es lo que distingue a
	// un multiple_operation_message de un single_operation_message. En el
	// mensaje múltiple ese campo se llama `reserved` y siempre vale 0xFFFF.
	OpMultiple = 0xFFFF
)

// Los `opID` de las operaciones que van *dentro* de un
// multiple_operation_message (SCTE 104, tabla 7-2 y capítulos 8 y 9). Aquí sí
// está el corte.
const (
	// MopInyectaSeccion — inject_section_data_request: mete una sección de
	// SCTE-35 ya armada por nosotros, tal cual, sin que el inyector la
	// interprete.
	MopInyectaSeccion = 0x0100
	// MopCorte — splice_request_data: empezar, terminar o cancelar un corte.
	// Es la operación que usa el 99 % del tráfico de este driver.
	MopCorte = 0x0101
	// MopCorteNulo — splice_null_request_data: un latido dentro del flujo de
	// cue, sin campos propios.
	MopCorteNulo = 0x0102
	// MopEmpiezaDescargaHorario — start_schedule_download_request.
	MopEmpiezaDescargaHorario = 0x0103
	// MopSenalDeHora — time_signal_request_data: marca un instante sin abrir
	// ni cerrar un corte (lo que en SCTE-35 es un time_signal).
	MopSenalDeHora = 0x0104
	// MopTransmiteHorario — transmit_schedule_request.
	MopTransmiteHorario = 0x0105
	// MopModoComponenteDPI — component_mode_DPI_request.
	MopModoComponenteDPI = 0x0106
	// MopDPICifrado — encrypted_DPI_request.
	MopDPICifrado = 0x0107
	// MopInsertaDescriptor — insert_descriptor_request: descriptores de
	// SCTE-35 en crudo.
	MopInsertaDescriptor = 0x0108
	// MopInsertaDTMF — insert_DTMF_descriptor_request: los tonos DTMF con los
	// que las redes han disparado cortes locales desde hace décadas.
	MopInsertaDTMF = 0x0109
	// MopInsertaAvail — insert_avail_descriptor_request.
	MopInsertaAvail = 0x010A
	// MopInsertaSegmentacion — insert_segmentation_descriptor_request.
	MopInsertaSegmentacion = 0x010B
	// MopComandoPropietario — proprietary_command_request: la puerta que el
	// estándar deja abierta para lo que cada fabricante quiera meter.
	MopComandoPropietario = 0x010C
	// MopHorarioModoComponente — schedule_component_mode_request.
	MopHorarioModoComponente = 0x010D
	// MopDefinicionHorario — schedule_definition_request.
	MopDefinicionHorario = 0x010E
	// MopInsertaTier — insert_tier_data.
	MopInsertaTier = 0x010F
	// MopInsertaDescriptorDeHora — insert_time_descriptor.
	MopInsertaDescriptorDeHora = 0x0110
	// MopBorraPalabraClave — delete_controlword_request.
	MopBorraPalabraClave = 0x0300
	// MopActualizaPalabraClave — update_controlword_request.
	MopActualizaPalabraClave = 0x0301
)

// Los códigos del campo `result` (SCTE 104, tabla 7-3). Solo el inyector los
// llena; nosotros mandamos ResultadoNoUsado.
//
// La lista viene de `src/protocol.ts` de la referencia de TypeScript, que es
// la única de las dos implementaciones libres que la trae (libklvanc no
// nombra los resultados porque no dialoga por TCP).
const (
	// ResultadoExito — 100: el único que significa que salió bien.
	ResultadoExito = 100
	// ResultadoAccesoDenegado — 101.
	ResultadoAccesoDenegado = 101
	// ResultadoFaltaPalabraClave — 102.
	ResultadoFaltaPalabraClave = 102
	// ResultadoDesprovisto — 103.
	ResultadoDesprovisto = 103
	// ResultadoNoSoportado — 104.
	ResultadoNoSoportado = 104
	// ResultadoNombreDuplicado — 105.
	ResultadoNombreDuplicado = 105
	// ResultadoNombreDuplicadoOK — 106.
	ResultadoNombreDuplicadoOK = 106
	// ResultadoCifradoNoSoportado — 107.
	ResultadoCifradoNoSoportado = 107
	// ResultadoPIDIlegal — 108.
	ResultadoPIDIlegal = 108
	// ResultadoPIDInconsistente — 109.
	ResultadoPIDInconsistente = 109
	// ResultadoInyectorOcupado — 110.
	ResultadoInyectorOcupado = 110
	// ResultadoNoProvistoParaAS — 111.
	ResultadoNoProvistoParaAS = 111
	// ResultadoNoProvistoParaDPI — 112.
	ResultadoNoProvistoParaDPI = 112
	// ResultadoInyectorSeraReemplazado — 113.
	ResultadoInyectorSeraReemplazado = 113
	// ResultadoTamanoInvalido — 114: el message_size no cuadra.
	ResultadoTamanoInvalido = 114
	// ResultadoSintaxisInvalida — 115.
	ResultadoSintaxisInvalida = 115
	// ResultadoVersionInvalida — 116.
	ResultadoVersionInvalida = 116
	// ResultadoSinFalla — 117.
	ResultadoSinFalla = 117
	// ResultadoFaltaNombreServicio — 118.
	ResultadoFaltaNombreServicio = 118
	// ResultadoPIDNoEncontrado — 119.
	ResultadoPIDNoEncontrado = 119
	// ResultadoCorteFallo — 120: splice_request_failed.
	ResultadoCorteFallo = 120
	// ResultadoParametroDeCorteMalo — 121.
	ResultadoParametroDeCorteMalo = 121
	// ResultadoPreRollMuyChico — 122: el pre_roll_time no le da tiempo al
	// encoder. Es el resultado que contesta un inyector cuando se le pide un
	// corte con menos de los 4000 ms que aconseja SCTE 67, y por eso este
	// paquete **no** valida el pre-roll por su cuenta: el inyector real es el
	// que sabe cuánto necesita, y lo dice con este código.
	ResultadoPreRollMuyChico = 122
	// ResultadoTipoDeTiempoNoSoportado — el inyector no maneja el time_type
	// de la marca que se le mandó.
	//
	// **Valor sin confirmar.** La referencia de TypeScript lo pone en 134, lo
	// que deja un hueco en el 123 y rompe la corrida densa 100-128 de todos
	// los demás; lo más probable es que el estándar diga 123 y que sea una
	// errata de la referencia. Se deja el 134 porque es el único dato que hay,
	// y como este código solo se *lee* (nunca se manda), equivocarlo cuesta
	// nada más que una frase «resultado desconocido» en la pantalla.
	ResultadoTipoDeTiempoNoSoportado = 134
	// ResultadoFallaDesconocida — 124.
	ResultadoFallaDesconocida = 124
	// ResultadoOpIDDesconocido — 125.
	ResultadoOpIDDesconocido = 125
	// ResultadoPIDDesconocido — 126.
	ResultadoPIDDesconocido = 126
	// ResultadoVersionDesigual — 127.
	ResultadoVersionDesigual = 127
	// ResultadoRespuestaDeProxy — 128.
	ResultadoRespuestaDeProxy = 128
)

// ResultadoNoUsado es lo que van en `result` y `result_extension` de una
// petición: los dos campos están en la cabecera de ida igual que en la de
// vuelta, pero solo el inyector los llena. 0xFFFF es el «no aplica» que pone
// la referencia de TypeScript en toda petición que sale.
const ResultadoNoUsado = 0xFFFF

var nombresDeResultado = map[uint16]string{
	ResultadoExito:                   "éxito",
	ResultadoAccesoDenegado:          "acceso denegado",
	ResultadoFaltaPalabraClave:       "falta la palabra clave",
	ResultadoDesprovisto:             "desprovisto",
	ResultadoNoSoportado:             "no soportado",
	ResultadoNombreDuplicado:         "nombre de servicio duplicado",
	ResultadoNombreDuplicadoOK:       "nombre de servicio duplicado, aceptado",
	ResultadoCifradoNoSoportado:      "cifrado no soportado",
	ResultadoPIDIlegal:               "DPI PID ilegal",
	ResultadoPIDInconsistente:        "DPI PID inconsistente",
	ResultadoInyectorOcupado:         "el inyector está ocupado",
	ResultadoNoProvistoParaAS:        "el inyector no está provisto para este sistema de automatización",
	ResultadoNoProvistoParaDPI:       "el inyector no está provisto para DPI",
	ResultadoInyectorSeraReemplazado: "el inyector va a ser reemplazado",
	ResultadoTamanoInvalido:          "message_size inválido",
	ResultadoSintaxisInvalida:        "sintaxis del mensaje inválida",
	ResultadoVersionInvalida:         "versión inválida",
	ResultadoSinFalla:                "no se encontró ninguna falla",
	ResultadoFaltaNombreServicio:     "falta el nombre del servicio",
	ResultadoPIDNoEncontrado:         "no se encontró el DPI PID",
	ResultadoCorteFallo:              "la petición de corte falló",
	ResultadoParametroDeCorteMalo:    "parámetro de corte inválido",
	ResultadoPreRollMuyChico:         "el pre-roll es muy chico para el encoder",
	ResultadoTipoDeTiempoNoSoportado: "tipo de marca de tiempo no soportado",
	ResultadoFallaDesconocida:        "falla desconocida",
	ResultadoOpIDDesconocido:         "opID desconocido",
	ResultadoPIDDesconocido:          "DPI PID desconocido",
	ResultadoVersionDesigual:         "las versiones no coinciden",
	ResultadoRespuestaDeProxy:        "respuesta de un proxy",
	ResultadoNoUsado:                 "sin usar",
}

// NombreDeResultado dice en cristiano qué contestó el inyector, para la
// bitácora y para la pantalla. Un código que no está en la tabla no es un
// error: el estándar reserva rangos y cada fabricante puede añadir los suyos.
func NombreDeResultado(r uint16) string {
	if n, ok := nombresDeResultado[r]; ok {
		return n
	}
	return "resultado desconocido"
}

var nombresDeOp = map[uint16]string{
	OpRespuestaGeneral:   "general_response_data",
	OpInicioPeticion:     "init_request_data",
	OpInicioRespuesta:    "init_response_data",
	OpVivoPeticion:       "alive_request_data",
	OpVivoRespuesta:      "alive_response_data",
	OpInyectaRespuesta:   "inject_response_data",
	OpInyectaCompleta:    "inject_complete_response_data",
	OpConfigPeticion:     "config_request_data",
	OpConfigRespuesta:    "config_response_data",
	OpProvisionPeticion:  "provisioning_request_data",
	OpProvisionRespuesta: "provisioning_response_data",
	OpFallaPeticion:      "fault_request_data",
	OpFallaRespuesta:     "fault_response_data",
	OpASVivoPeticion:     "AS_alive_request_data",
	OpASVivoRespuesta:    "AS_alive_response_data",
	OpMultiple:           "multiple_operation_message",
}

// NombreDeOp devuelve el nombre del estándar para un opID de
// single_operation_message. Se usa en los mensajes de error y en la bitácora:
// el nombre del estándar es el que aparece en el manual del encoder, así que
// es el que sirve para llamar al ingeniero.
func NombreDeOp(op uint16) string {
	if n, ok := nombresDeOp[op]; ok {
		return n
	}
	switch {
	case op == 0x0005 || op == 0x0006, op >= 0x000D && op <= 0x000E,
		op >= 0x8000 && op <= 0xBFFF:
		return "definido por el fabricante"
	default:
		return "reservado"
	}
}

var nombresDeMop = map[uint16]string{
	MopInyectaSeccion:          "inject_section_data_request",
	MopCorte:                   "splice_request_data",
	MopCorteNulo:               "splice_null_request_data",
	MopEmpiezaDescargaHorario:  "start_schedule_download_request",
	MopSenalDeHora:             "time_signal_request_data",
	MopTransmiteHorario:        "transmit_schedule_request",
	MopModoComponenteDPI:       "component_mode_DPI_request",
	MopDPICifrado:              "encrypted_DPI_request",
	MopInsertaDescriptor:       "insert_descriptor_request",
	MopInsertaDTMF:             "insert_DTMF_descriptor_request",
	MopInsertaAvail:            "insert_avail_descriptor_request",
	MopInsertaSegmentacion:     "insert_segmentation_descriptor_request",
	MopComandoPropietario:      "proprietary_command_request",
	MopHorarioModoComponente:   "schedule_component_mode_request",
	MopDefinicionHorario:       "schedule_definition_request",
	MopInsertaTier:             "insert_tier_data",
	MopInsertaDescriptorDeHora: "insert_time_descriptor",
	MopBorraPalabraClave:       "delete_controlword_request",
	MopActualizaPalabraClave:   "update_controlword_request",
}

// NombreDeMop devuelve el nombre del estándar para el opID de una operación de
// dentro de un multiple_operation_message.
func NombreDeMop(op uint16) string {
	if n, ok := nombresDeMop[op]; ok {
		return n
	}
	if op >= 0xC000 && op <= 0xFFFE {
		return "definido por el fabricante"
	}
	return "reservado"
}
