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
  tipo?: 'sobrecupo' | 'hueco' | 'vencimiento' | 'sin_relleno' | 'material' | string
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
   * Por qué quedó parado, en clave: "sin_audio" y los demás. "sin_audio" no
   * tiene salida por la vía de dejarlo pasar (F1-59): se arregla poniendo el
   * audio al lado.
   */
  motivo_codigo?: string
}

// ── importar desde la hoja ────────────────────────────────────────────

export interface RelevoPropuesto {
  regla: number
  releva_a: number
  texto: string // "¿Zoids releva a Magic Knight?"
}

export interface FilaConError {
  fila: number
  texto: string
  motivo: string
}

export interface ResumenDeImportacion {
  reglas_creadas: number
  titulos_creados: number
  relevos_propuestos: RelevoPropuesto[]
  filas_con_error: FilaConError[]
  fechas_corridas: { fila: number; texto: string }[]
}

// ── ajustes ───────────────────────────────────────────────────────────

export type Ajustes = Record<string, string>

// ── instalación ───────────────────────────────────────────────────────

export interface Instalacion {
  paso: number
  detectado: Record<string, string>
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
