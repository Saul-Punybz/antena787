// Los tipos vienen del contrato en docs/API.md y de las etiquetas json de
// internal/model. Nombres de campo en español, iguales a los de la base.
// Todo instante viaja como RFC 3339 en UTC; los días de emisión como
// "AAAA-MM-DD"; las horas del día como "HH:MM" en la zona del canal.

export type DiaEmision = string // "AAAA-MM-DD"
export type HoraDelDia = string // "HH:MM"
export type Instante = string // RFC 3339 UTC

export type ModoCanal = 'sombra' | 'aire'
export type TipoCanal = 'tv' | 'radio'

export interface Canal {
  id: number
  nombre: string
  tipo: TipoCanal
  perfil_de_formato: string
  perfil_regulatorio: string
  modo: ModoCanal
  zona_horaria: string
  hora_inicio_dia_emision: number // minutos desde medianoche
  carga_maxima_por_hora: number
  identificativo: string
  comunidad_licencia: string
  clase_licencia: string
}

export interface Salida {
  id: number
  nombre: string
  driver: string // interno: nunca se muestra tal cual (PRD §4.3)
  estado_conexion: string // conectada | apagada | reintentando
  reintentos: number
  ultimo_error: string
}

export type EstadoPlan =
  | 'planned'
  | 'cued'
  | 'aired'
  | 'skipped'
  | 'preempted'
  | 'fallido'
  | 'manual_hold'

export interface ElementoDelPlan {
  id: number
  dia_emision: DiaEmision
  instante_planeado: Instante
  duracion_planeada_ms: number
  instante_real?: Instante | null
  duracion_real_ms?: number | null
  origen: 'asset' | 'live_source' | 'relleno' | 'cartel'
  estado: EstadoPlan
  hora_local: HoraDelDia
  /**
   * Lo movió una persona a mano desde la parrilla: el resolver no lo vuelve a
   * pisar. Se suelta con PUT /plan/{id} {"fijado": false}.
   */
  fijado?: boolean
  // embebidos que el servidor añade para la interfaz
  titulo?: string
  temporada?: number | null
  episodio?: number | null
  en_vivo?: boolean
}

export interface HuecoDelPlan {
  hueco: true
  inicio: Instante
  fin: Instante
}

export type FilaDelPlan = ElementoDelPlan | HuecoDelPlan

/** Cuerpo de PUT /plan/{id}: mover un bloque a mano, o soltarlo. */
export interface CambioDePlan {
  /** ISO 8601 con desfase, p. ej. "2026-09-08T15:30:00-04:00". */
  instante_planeado?: Instante
  duracion_planeada_ms?: number
  /** false suelta el bloque: vuelve a mandar la regla. */
  fijado?: boolean
}

export function esHueco(x: FilaDelPlan): x is HuecoDelPlan {
  return (x as HuecoDelPlan).hueco === true
}

export type NivelAlarma = 'bien' | 'aviso' | 'problema'

export interface Alarma {
  tipo?:
    | 'sobrecupo'
    | 'hueco'
    | 'vencimiento'
    | 'sin_relleno'
    | 'material'
    | 'emparejar'
    | string
  nivel: NivelAlarma
  texto: string
  detalle?: string
  accion?: { texto: string; ruta: string }
}

export interface Estado {
  necesita_instalacion?: boolean
  /** La instalación se terminó de contestar (los nueve pasos del asistente). */
  instalacion_completa?: boolean
  /** Hay cookie de sesión válida. En falso, la interfaz lleva a Entrar. */
  entraste?: boolean
  canal: Canal
  modo: ModoCanal
  ahora: Instante
  dia_emision: DiaEmision
  al_aire: ElementoDelPlan | null
  siguiente: ElementoDelPlan | null
  alarmas: Alarma[]
  salidas: Salida[]
  version: string
  // el retorno de aire (capture_input): puede no existir todavía
  retorno_de_aire?: { hay: boolean; texto: string }
  control_manual?: { activo: boolean; quien?: string; vuelve_en_s?: number }
  /** Anuncios solo aparece en el menú al registrar el primer anunciante (PRD §13). */
  hay_anunciantes?: boolean
}

// ── reglas ────────────────────────────────────────────────────────────

export type TipoRegla = 'normal' | 'diferido' | 'bloque_arrendado' | 'vivo'

export interface Regla {
  id: number
  tipo: TipoRegla
  title_id: number | null
  live_source_id: number | null
  titulo: string
  patron_de_dias: string // siete posiciones, "LMMJV__"
  hora: number // minutos desde medianoche
  duracion_slot_ms: number
  fecha_inicio: DiaEmision
  fecha_fin: DiaEmision
  dias_restantes: number
  episodios_por_corrida: number
  releva_a: number | null
  releva_a_titulo?: string | null
  repite_a: number | null
  repite_a_titulo?: string | null
  activa: boolean
}

export type ReglaNueva = Omit<Regla, 'id' | 'dias_restantes'>

// ── parrilla ──────────────────────────────────────────────────────────

export interface FranjaSemana {
  // una franja de 30 min: qué la ocupa, o nada
  titulo: string | null
  en_vivo: boolean
  duracion_ms: number
  /** El plan_item que ocupa la franja, para poder moverlo o soltarlo. */
  plan_id?: number | null
  /** Ese plan_item lo fijó una persona a mano. */
  fijado?: boolean
}

export interface DiaDeLaSemana {
  dia: DiaEmision
  franjas: FranjaSemana[] // 48 franjas de 30 min, desde medianoche
  horas_vacias: number
}

export interface SemanaDelPlan {
  desde: DiaEmision
  dias: DiaDeLaSemana[]
  nota?: string
}

export interface DiaDelMes {
  dia: DiaEmision
  horas_sin_llenar: number
  franjas_llenas: boolean[] // 48 franjas de 30 min: true = programada
  vencimientos: string[]
  estrenos: string[]
}

export interface MesDelPlan {
  mes: string // "AAAA-MM"
  dias: DiaDelMes[]
  horas_vacias_mes: number
  porcentaje_vacio: number
}

// ── guía ──────────────────────────────────────────────────────────────

export interface ProgramaDeGuia {
  titulo: string
  inicio: Instante
  duracion_ms: number
}

export interface FilaDeGuia {
  guia: ProgramaDeGuia | null
  plan: ProgramaDeGuia | null
  coincide: boolean
}

export interface Guia {
  dia: DiaEmision
  filas: FilaDeGuia[]
  identificador_de_canal: string
  revalidada: Instante
  por_que_no_coinciden?: string
}

// ── biblioteca ────────────────────────────────────────────────────────

export type EstadoMaterial = 'listo' | 'aún no listo para aire' | 'cuarentena'

/** Una pista de sonido de un archivo (F1-60). El índice es el que manda el servidor. */
export interface PistaDeAudio {
  indice: number
  /** Código del idioma tal como viene del archivo: "es", "spa", "en", "eng", "und"… */
  idioma: string
  canales: number
  titulo: string
}

/**
 * El sonido de un archivo: la lista de pistas, cuál sale al aire y de dónde
 * salieron el audio y los subtítulos si vinieron en un archivo de al lado
 * (F1-58 a F1-62). Todo opcional: una respuesta vieja sigue pintando igual.
 */
export interface AudioDelMaterial {
  /** El archivo en sí, para PUT /material/{id}. */
  material_id?: number
  pistas_audio?: PistaDeAudio[]
  pista_audio_aire?: number
  /** Ruta del archivo de audio de al lado; vacío cuando no hubo. */
  audio_sidecar?: string
  /** Ruta del archivo de subtítulos de al lado; vacío cuando no hubo. */
  subtitulos_sidecar?: string
}

export interface TituloDeBiblioteca extends AudioDelMaterial {
  id: number
  nombre: string
  tipo: string
  sinopsis: string
  anio: number | null
  clasificacion_contenido: string
  caratula: string
  episodios: number
  duracion_ms: number
  estado_material: EstadoMaterial
  en_la_parrilla: boolean
  hora?: HoraDelDia | null
  regla_hasta?: DiaEmision | null
}

export interface EpisodioDeBiblioteca extends AudioDelMaterial {
  id: number
  temporada: number
  numero: number
  nombre: string
  duracion_ms: number
  estado_material: EstadoMaterial
}

/** Cuerpo de PUT /material/{id}: por ahora, cambiar la pista que sale al aire. */
export interface CambioDeMaterial {
  pista_audio_aire?: number
}

/**
 * Lo que contesta PUT /material/{id}. Solo se leen estos campos: el resto de
 * las medidas del archivo no las usa ninguna pantalla todavía.
 */
export interface MaterialDeAudio extends AudioDelMaterial {
  id?: number
  estado_material?: EstadoMaterial
}

export interface FichaDeTitulo extends TituloDeBiblioteca {
  lista_de_episodios: EpisodioDeBiblioteca[]
}

export interface EnCuarentena {
  id: number
  ruta: string
  titulo: string
  motivo_en_cristiano: string
  creado: Instante
  /**
   * Por qué quedó parado, en clave. "sin_audio" no tiene salida por la vía
   * de dejarlo pasar (F1-59): se arregla poniendo el audio al lado.
   * "duracion_av_no_coincide" (imagen y sonido de distinta duración, F1-70)
   * y "normalizacion_fallida" (no se pudo preparar o se quedó colgado, F1-71)
   * sí se pueden dejar pasar; en el segundo caso sale el original tal cual.
   * Vacío en lo demás.
   */
  motivo_codigo?: string
}

// ── la bitácora: lo que el sistema hizo solo ──────────────────────────

/**
 * Un incidente es algo que el sistema hizo solo para proteger el aire, o
 * algo que le pasó y anotó (PRD §15, `incidente`). Cuándo, qué, cuánto duró.
 * `fin` es nulo mientras sigue abierto. `texto` es la frase en cristiano del
 * tipo; si un servidor viejo no la manda, la pantalla enseña el tipo legible.
 */
export interface Incidente {
  id: number
  tipo: string
  inicio: Instante
  fin: Instante | null
  detalle: string
  texto?: string
}

// ── importar desde la hoja ────────────────────────────────────────────

export interface RelevoPropuesto {
  regla: number
  releva_a: number
  texto: string // "¿Zoids releva a Magic Knight?"
}

/** Una repetición que el importador propone marcar como segundo pase. */
export interface RepeticionPropuesta {
  regla_primaria: number
  regla_que_repite: number
  texto: string
}

export interface FilaConError {
  fila: number
  texto: string
  motivo: string
  id_hoja?: string
  titulo?: string
}

export interface FechaCorrida {
  fila: number
  texto: string
  regla?: number
  titulo?: string
  hora?: HoraDelDia
  antes?: string
  despues?: string
}

/**
 * Dos nombres de la hoja que parecen el mismo programa escrito de dos maneras.
 * El importador los junta y lo dice; aquí solo se pinta lo que hizo.
 */
export interface PosibleDuplicado {
  a: string
  b: string
  texto: string
}

/** Lo que el importador arregló solo, ya en frases: erratas que juntó y demás. */
export interface AvisoDeImportacion {
  fila: number
  id_hoja: string
  titulo: string
  texto: string
}

// ── emparejar títulos (F1-64 a F1-67) ─────────────────────────────────

/** Una ficha del catálogo que se parece al nombre que trajo la hoja. */
export interface CandidatoDeTitulo {
  id: number
  nombre: string
  /** De 0 a 1: cuánto se parecen los dos nombres. Se enseña como porcentaje. */
  puntuacion: number
}

/**
 * Un nombre de la hoja que no se pudo emparejar solo con una ficha del
 * catálogo. La regla se importó igual —la hoja nunca se rechaza—, pero el
 * título queda «por emparejar» hasta que una persona diga cuál es.
 */
export interface TituloSinEmparejar {
  id: number
  /** El nombre tal como venía en la hoja. */
  nombre: string
  /** La frase en cristiano del servidor: por qué quedó pendiente. */
  texto: string
  candidatos: CandidatoDeTitulo[]
  /** Cuántas reglas lo usan: es lo que se pierde si se quita. */
  reglas: number
  /** Las franjas donde va, ya en cristiano: «L-V 2:30 PM». */
  franjas: string[]
}

/** Un título del catálogo, como lo devuelve la búsqueda del emparejador. */
export interface TituloDelCatalogo {
  id: number
  nombre: string
  tipo: string
}

/** Lo que se manda a POST /titulos/{id}/emparejar. */
export type DecisionDeEmparejar =
  | { accion: 'usar'; title_id: number }
  | { accion: 'propio' }
  | { accion: 'quitar' }

/** Lo que contesta el servidor tras emparejar. Siempre trae `texto`. */
export interface ResultadoDeEmparejar {
  texto: string
  /** Con «usar»: cuántas reglas pasaron a la ficha del catálogo. */
  reglas_movidas?: number
  /** Con «usar»: el nombre de la hoja que quedó guardado como alias. */
  alias?: string
  /** Con «quitar»: cuántas reglas se fueron con el título. */
  reglas_quitadas?: number
}

export interface ResumenDeImportacion {
  reglas_creadas: number
  titulos_creados: number
  relevos_propuestos: RelevoPropuesto[]
  filas_con_error: FilaConError[]
  fechas_corridas: FechaCorrida[]
  /** Opcionales: una respuesta vieja sin ellos sigue pintando igual. */
  repeticiones_propuestas?: RepeticionPropuesta[]
  titulos_sin_emparejar?: TituloSinEmparejar[]
  posibles_duplicados?: PosibleDuplicado[]
  avisos?: AvisoDeImportacion[]
  resumen?: string
}

// ── ajustes ───────────────────────────────────────────────────────────

export type Ajustes = Record<string, string>

// ── instalación ───────────────────────────────────────────────────────

/**
 * Una respuesta posible que propone el servidor. El valor es interno (nunca se
 * enseña); `texto` es lo que lee la persona y `ayuda` la frase de abajo. El
 * asistente no inventa listas: pinta lo que venga en `opciones` (PRD §10: «el
 * usuario nunca ve la palabra driver»).
 */
export interface Opcion {
  valor: string
  texto: string
  ayuda?: string
}

/** Lo que la máquina averiguó sola, ya en frases, no en claves. */
export interface DetectadoEnLaMaquina {
  ffmpeg: string
  ffprobe: string
  carpeta_datos: string
  carpeta_contenido: string
  carpeta_respaldo: string
  /** Cuántas piezas de relleno hay. Cero significa que el primer hueco sale en negro. */
  relleno: number
  /** Una frase, nunca el nombre de una tarjeta: se mide al arrancar el motor. */
  aceleracion: string
  problema?: string
  disco: string
  red: string
}

export interface OpcionesDelAsistente {
  modo: Opcion[]
  destino: Opcion[]
  retorno: Opcion[]
  calidad: Opcion[]
}

/** Lo ya contestado, para volver a pintar el asistente igual tras recargar. */
export interface RespuestasDelAsistente {
  '1'?: {
    nombre: string
    identificativo: string
    comunidad_licencia: string
    nombre_operador: string
  }
  '2'?: { modo: string }
  '4'?: { destino: string; retorno_de_aire: string }
  '5'?: { ve_barras: boolean }
  '6'?: { pais: string; calidad: string }
  '7'?: { carpeta: string }
  '8'?: { propuesta: string }
}

export interface Instalacion {
  paso: number
  pasos: number
  completa: boolean
  necesita_instalacion: boolean
  canal: Canal
  detectado: DetectadoEnLaMaquina
  opciones: OpcionesDelAsistente
  respuestas: RespuestasDelAsistente
  /** Cuándo se contestó cada paso, en RFC 3339. Sirve para saber dónde se abandona. */
  tiempos: Record<string, Instante>
}

/** Un aviso del resolver (paso 8): siempre trae la frase en cristiano. */
export interface AvisoDeParrilla {
  tipo: string
  texto: string
  schedule_rule_id?: number
  dia_emision?: DiaEmision
  instante?: Instante
  aviso?: string
}

// Lo que se manda en cada paso. El servidor contesta siempre
// {paso, siguiente} más lo suyo, o {error, campo?} con 400.

export interface CuerpoPaso1 {
  nombre: string
  identificativo: string
  comunidad_licencia: string
  /** Cuatro a seis dígitos. Una sola clave para la estación (PRD §13). */
  clave: string
  nombre_operador: string
}

export interface CuerposDePaso {
  1: CuerpoPaso1
  2: { modo: string }
  3: Record<string, never>
  4: { destino: string; retorno_de_aire: string; nota?: string }
  5: { ve_barras: boolean }
  6: { pais: string; calidad: string }
  7: { carpeta: string }
  8: { propuesta: 'automatica' | 'ninguna' }
  9: Record<string, never>
}

interface PasoContestado {
  paso: number
  siguiente: number
}

export interface RespuestaPaso1 extends PasoContestado {
  canal: Canal
  /** El cartel de respaldo: "<identificativo> · <comunidad de licencia>". */
  cartel: string
}
export interface RespuestaPaso2 extends PasoContestado {
  modo_del_canal: ModoCanal
  aviso: string
}
export interface RespuestaPaso3 extends PasoContestado {
  ffmpeg: string
  ffprobe: string
  problema?: string
}
export interface RespuestaPaso4 extends PasoContestado {
  aviso?: string
}
export interface RespuestaPaso5 extends PasoContestado {
  aviso: string
}
export interface RespuestaPaso6 extends PasoContestado {
  canal: Canal
  formato: string
}
export interface RespuestaPaso7 extends PasoContestado {
  carpeta: string
  aviso_vigilancia: string
  aviso_relleno?: string
}
export interface RespuestaPaso8 extends PasoContestado {
  reglas_creadas: number
  bloques: number
  avisos: AvisoDeParrilla[]
  titulos_sin_material: number
  aviso: string
}
export interface RespuestaPaso9 extends PasoContestado {
  completa: true
  modo_del_canal: ModoCanal
  aviso: string
}

export interface RespuestasDePaso {
  1: RespuestaPaso1
  2: RespuestaPaso2
  3: RespuestaPaso3
  4: RespuestaPaso4
  5: RespuestaPaso5
  6: RespuestaPaso6
  7: RespuestaPaso7
  8: RespuestaPaso8
  9: RespuestaPaso9
}

export type NumeroDePaso = keyof RespuestasDePaso

/** POST /instalacion/relleno-por-defecto: 202 con el archivo ya creado. */
export interface RellenoPorDefecto {
  archivo: string
  aviso: string
}

// ── errores ───────────────────────────────────────────────────────────

export class ErrorDeApi extends Error {
  campo?: string
  estado: number
  constructor(mensaje: string, estado: number, campo?: string) {
    super(mensaje)
    this.name = 'ErrorDeApi'
    this.estado = estado
    this.campo = campo
  }
}
