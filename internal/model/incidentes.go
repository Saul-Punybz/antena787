// incidentes.go es el catálogo único de tipos de incidente. Quien escriba en
// la bitácora usa una de estas constantes y nunca una cadena suelta en su
// archivo: dos partes del sistema no pueden llamar distinto a la misma cosa,
// ni igual a dos cosas distintas (docs/f2/PLAN-F2.md §3; F2-63 se verifica
// contra esta lista).
package model

// TipoIncidente es la columna `tipo` de la tabla incidente.
type TipoIncidente string

// String deja usar el tipo donde se espera texto sin escribir la conversión.
func (t TipoIncidente) String() string { return string(t) }

// Los tipos que ya escribía F1.
const (
	IncCuarentena            TipoIncidente = "cuarentena"
	IncSubidaRechazada       TipoIncidente = "subida_rechazada"
	IncNormalizacionFallida  TipoIncidente = "normalizacion_fallida"
	IncSubtitulos            TipoIncidente = "subtitulos"
	IncDiscoBajo             TipoIncidente = "disco_bajo"
	IncSaltoDeReloj          TipoIncidente = "salto_de_reloj"
	IncRellenoPorDefecto     TipoIncidente = "relleno_por_defecto"
	IncBaseRestaurada        TipoIncidente = "base_restaurada"
	IncPropuestaDelAsistente TipoIncidente = "propuesta_del_asistente"
	IncVencimiento           TipoIncidente = "vencimiento"
	IncGuiaRechazada         TipoIncidente = "guia_rechazada"
)

// Los tipos del motor y de lo que llega con él (F2). Cada tanda del plan de
// F2 añade los suyos aquí, no en su propio archivo.
const (
	// IncEncoderReiniciado — el encoder murió o se colgó y se relanzó
	// (F2-11). Lo escribe T1 al relanzarlo; el watchdog de 3 s es de T6.
	IncEncoderReiniciado TipoIncidente = "encoder_reiniciado"
	// IncFalloDeClip — un archivo del plan no se pudo poner al aire y la
	// cascada lo cubrió (F2-12).
	IncFalloDeClip TipoIncidente = "fallo_de_clip"
	// IncCartel — el aire cayó al cartel de la estación (F2-09, F2-10).
	IncCartel TipoIncidente = "cartel"
	// IncSolape — dos bloques del mismo deck quisieron salir a la vez; sale
	// el de menor id (PRD §9 paso 4).
	IncSolape TipoIncidente = "solape"
	// IncCascadaExtendida — el cartel llevaba más de 15 minutos al aire.
	IncCascadaExtendida TipoIncidente = "cascada_extendida"
	// IncVivoAusente — la fuente en vivo no llegó (T4, F2-19).
	IncVivoAusente TipoIncidente = "vivo_ausente"
	// IncManualPorTimeout — el control manual venció por silencio o negro y
	// el aire volvió solo (T5, F2-30).
	IncManualPorTimeout TipoIncidente = "manual_por_timeout"
	// IncApagon — la máquina estuvo apagada (T6).
	IncApagon TipoIncidente = "apagon"
	// IncEnlaceCaido — una salida se cayó y se reconectó (T2/T7, F2-48).
	IncEnlaceCaido TipoIncidente = "enlace_caido"
	// IncSilencioDetectado e IncNegroDetectado — el detector sobre la salida
	// (T3, F2-51 a F2-54).
	IncSilencioDetectado TipoIncidente = "silencio_detectado"
	IncNegroDetectado    TipoIncidente = "negro_detectado"
)
