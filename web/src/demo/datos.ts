// Datos de ejemplo: la semana de CAtv del 6 al 12 de septiembre de 2026,
// tal como está en docs/catv-sheet-2026-09-04.md. Sirven para ver la
// interfaz sin servidor (VITE_DEMO=1) y para que la interfaz siga viva si
// /api/v1/estado no responde.
//
// El reloj del canal arranca el viernes 4 de septiembre de 2026 a la 1:45 PM
// (17:45 UTC, America/Puerto_Rico) y corre desde ahí, que es el instante que
// dibujan los mockups de diseno/.

import type {
  Ajustes,
  Canal,
  EnCuarentena,
  EpisodioDeBiblioteca,
  PistaDeAudio,
  Regla,
  Salida,
  TituloDeBiblioteca,
} from '../lib/tipos'

export const ZONA = 'America/Puerto_Rico'
export const AHORA_BASE = Date.parse('2026-09-04T17:45:00Z')
export const HOY: string = '2026-09-04'

export const canal: Canal = {
  id: 1,
  nombre: 'Caribbean Advantage TV',
  tipo: 'tv',
  perfil_de_formato: '1080i 29.97',
  perfil_regulatorio: 'us-fcc',
  modo: 'sombra',
  zona_horaria: ZONA,
  hora_inicio_dia_emision: 360,
  carga_maxima_por_hora: 12,
  identificativo: 'CAtv',
  comunidad_licencia: 'Cabo Rojo, Puerto Rico',
  clase_licencia: 'LPTV',
}

export const salidas: Salida[] = [
  {
    id: 1,
    nombre: 'Transmisor',
    driver: 'udp-ts',
    estado_conexion: 'conectada',
    reintentos: 0,
    ultimo_error: '',
  },
  {
    id: 2,
    nombre: 'YouTube',
    driver: 'rtmp',
    estado_conexion: 'conectada',
    reintentos: 0,
    ultimo_error: '',
  },
  {
    id: 3,
    nombre: 'Facebook',
    driver: 'rtmp',
    estado_conexion: 'apagada',
    reintentos: 0,
    ultimo_error: '',
  },
]

// ── la semana, franja por franja ──────────────────────────────────────
// Cada fila es una franja de 30 min; las siete columnas van de domingo a
// sábado, igual que la hoja. "" es una franja sin nada.

const V = '' // vacío
export const FRANJAS: string[] = (() => {
  const out: string[] = []
  for (let m = 0; m < 24 * 60; m += 30) out.push(
    `${String(Math.floor(m / 60)).padStart(2, '0')}:${String(m % 60).padStart(2, '0')}`,
  )
  return out
})()

type FilaHoja = [string, string, string, string, string, string, string]

export const SEMANA_CATV: Record<string, FilaHoja> = {
  '00:00': ['Hack Legend', 'Hack Legend', 'Magic Knight Rayearth', 'Zoids', 'Zoids', 'Zoids', 'Zoids'],
  '00:30': [V, V, 'Mazinger Z', 'Mazinger Z', 'Mazinger Z', 'Mazinger Z', 'Mazinger Z'],
  '06:00': [V, 'Get Smart', 'Get Smart', 'Get Smart', 'Get Smart', 'Get Smart', V],
  '06:30': [V, 'I Dream of Jeannie', 'I Dream of Jeannie', 'I Dream of Jeannie', 'I Dream of Jeannie', 'I Dream of Jeannie', V],
  '07:00': [V, 'Tarzan', 'Tarzan', 'Tarzan', 'Tarzan', 'Tarzan', V],
  '07:30': ['Carmen Sandiego', 'Tarzan', 'Tarzan', 'Tarzan', 'Tarzan', 'Tarzan', 'Carmen Sandiego'],
  '08:00': ['Don Quijote', 'Kojak', 'Kojak', 'Kojak', 'Kojak', 'Kojak', 'Don Quijote'],
  '08:30': ['TinTin', 'Kojak', 'Kojak', 'Kojak', 'Kojak', 'Kojak', 'TinTin'],
  '09:00': ['Familia Robinson', 'Comics 9th Art', 'Comics 9th Art', 'Comics 9th Art', 'Comics 9th Art', 'Comics 9th Art', V],
  '09:30': ['Sonic The Hedgehog', "Gilligan's Island", "Gilligan's Island", "Gilligan's Island", "Gilligan's Island", "Gilligan's Island", 'Sonic The Hedgehog'],
  '10:00': ['Astroboy', 'RadioOnce Live!', 'RadioOnce Live!', 'RadioOnce Live!', 'RadioOnce Live!', 'RadioOnce Live!', 'Astroboy'],
  '10:30': ['Corrector Yui', 'RadioOnce Live!', 'RadioOnce Live!', 'RadioOnce Live!', 'RadioOnce Live!', 'RadioOnce Live!', 'Corrector Yui'],
  '11:00': [V, 'RadioOnce Live!', 'RadioOnce Live!', 'RadioOnce Live!', 'RadioOnce Live!', 'RadioOnce Live!', V],
  '11:30': [V, 'RadioOnce Live!', 'RadioOnce Live!', 'RadioOnce Live!', 'RadioOnce Live!', 'RadioOnce Live!', V],
  '12:00': [V, 'RadioOnce Live!', 'RadioOnce Live!', 'RadioOnce Live!', 'RadioOnce Live!', 'RadioOnce Live!', V],
  '12:30': [V, 'RadioOnce Live!', 'RadioOnce Live!', 'RadioOnce Live!', 'RadioOnce Live!', 'RadioOnce Live!', V],
  '13:00': [V, 'Los Simuladores', 'Los Simuladores', 'Los Simuladores', 'Los Simuladores', 'Los Simuladores', V],
  '13:30': [V, 'Los Simuladores', 'Los Simuladores', 'Los Simuladores', 'Los Simuladores', 'Los Simuladores', V],
  '14:00': ['Voyagesea', "You're Under Arrest", "You're Under Arrest", "You're Under Arrest", "You're Under Arrest", "You're Under Arrest", 'Voyagesea'],
  '14:30': ['Voyagesea', 'Samurai X', 'Samurai X', 'Samurai X', 'Samurai X', 'Samurai X', 'Voyagesea'],
  '15:00': [V, 'Magic Knight Rayearth', 'Zoids', 'Zoids', 'Zoids', 'Zoids', V],
  '15:30': [V, 'Mazinger Z', 'Mazinger Z', 'Mazinger Z', 'Mazinger Z', 'Mazinger Z', V],
  '16:00': [V, 'Green Hornet', 'Green Hornet', 'Green Hornet', 'Green Hornet', 'Green Hornet', V],
  '16:30': [V, 'Zorro 57', 'Zorro 57', 'Zorro 57', 'Zorro 57', 'Zorro 57', V],
  '17:00': [V, 'Los Lorcanitos', 'Los Lorcanitos', 'Los Lorcanitos', 'Los Lorcanitos', 'Los Lorcanitos', V],
  '17:30': [V, 'Los Lorcanitos', 'Los Lorcanitos', 'Los Lorcanitos', 'Los Lorcanitos', 'Los Lorcanitos', V],
  '23:00': ['SaberMarionette', "You're Under Arrest", "You're Under Arrest", "You're Under Arrest", "You're Under Arrest", "You're Under Arrest", 'SaberMarionette'],
  '23:30': ["BT'x", 'Samurai X', 'Samurai X', 'Samurai X', 'Samurai X', 'Samurai X', "BT'x"],
}

// Gaming Longplays ocupa de 6:00 PM a 11:00 PM los siete días.
for (let m = 18 * 60; m < 23 * 60; m += 30) {
  const clave = `${String(Math.floor(m / 60)).padStart(2, '0')}:${String(m % 60).padStart(2, '0')}`
  SEMANA_CATV[clave] = [
    'Gaming Longplays', 'Gaming Longplays', 'Gaming Longplays', 'Gaming Longplays',
    'Gaming Longplays', 'Gaming Longplays', 'Gaming Longplays',
  ]
}

/** Qué hay en una franja de un día calendario. "" = nada. */
export function programaEn(dia: string, hhmm: string): string {
  const fila = SEMANA_CATV[hhmm]
  if (!fila) return ''
  const [a, m, d] = dia.split('-').map(Number)
  const dow = new Date(Date.UTC(a, m - 1, d)).getUTCDay() // 0 = domingo
  const nombre = fila[dow]
  if (!nombre) return ''
  // El relevo de Magic Knight Rayearth por Zoids: el 8 de septiembre.
  if (nombre === 'Magic Knight Rayearth' && dia >= '2026-09-08') return 'Zoids'
  if (nombre === 'Zoids' && dia < '2026-09-08' && hhmm === '15:00')
    return 'Magic Knight Rayearth'
  return nombre
}

export const EN_VIVO = new Set(['RadioOnce Live!'])

// ── las reglas ────────────────────────────────────────────────────────

const L = 'LMMJV__'
const F = '_____SD'
const T = 'LMMJVSD'

type Semilla = [
  titulo: string,
  patron: string,
  horaHhMm: string,
  slotMin: number,
  desde: string,
  hasta: string,
  extra?: Partial<Regla>,
]

const SEMILLAS: Semilla[] = [
  ['Get Smart', L, '06:00', 30, '2026-03-07', '2026-09-16'],
  ['I Dream of Jeannie', L, '06:30', 30, '2026-02-16', '2026-09-15'],
  ['Tarzan', L, '07:00', 60, '2026-01-12', '2026-09-23'],
  ['Kojak', L, '08:00', 60, '2026-07-27', '2027-01-04'],
  ['Comics 9th Art', L, '09:00', 30, '2026-09-04', '2026-09-22'],
  ["Gilligan's Island", L, '09:30', 30, '2026-09-04', '2027-01-19'],
  ['RadioOnce Live!', L, '10:00', 180, '2026-09-03', '2026-12-31', { tipo: 'vivo' }],
  ['Los Simuladores', L, '13:00', 60, '2026-08-11', '2026-09-11'],
  ["You're Under Arrest", L, '14:00', 30, '2026-07-07', '2026-09-16'],
  ['Samurai X', L, '14:30', 30, '2026-04-20', '2026-11-30'],
  ['Magic Knight Rayearth', L, '15:00', 30, '2026-03-02', '2026-09-07'],
  ['Zoids', L, '15:00', 30, '2026-09-08', '2026-12-09', { releva_a_titulo: 'Magic Knight Rayearth' }],
  ['Mazinger Z', L, '15:30', 30, '2026-06-10', '2026-10-15'],
  ['Green Hornet', L, '16:00', 30, '2026-05-04', '2026-09-30'],
  ['Zorro 57', L, '16:30', 30, '2026-07-02', '2026-10-21'],
  ['Los Lorcanitos', L, '17:00', 60, '2026-06-01', '2026-11-16'],
  ['Gaming Longplays', T, '18:00', 300, '2026-08-31', '2026-09-28', { episodios_por_corrida: 10 }],
  ["You're Under Arrest", L, '23:00', 30, '2026-07-07', '2026-09-16', { repite_a_titulo: "You're Under Arrest" }],
  ['Samurai X', L, '23:30', 30, '2026-04-20', '2026-11-30', { repite_a_titulo: 'Samurai X' }],
  ['Zoids', L, '00:00', 30, '2026-09-08', '2026-12-09', { repite_a_titulo: 'Zoids' }],
  ['Mazinger Z', L, '00:30', 30, '2026-06-10', '2026-10-15', { repite_a_titulo: 'Mazinger Z' }],
  ['Magic Knight Rayearth', L, '00:00', 30, '2026-03-02', '2026-09-07', { repite_a_titulo: 'Magic Knight Rayearth' }],
  ['Carmen Sandiego', F, '07:30', 30, '2026-02-01', '2026-11-01'],
  ['Don Quijote', F, '08:00', 30, '2026-02-01', '2026-10-25'],
  ['TinTin', F, '08:30', 30, '2026-02-01', '2026-10-25'],
  ['Familia Robinson', F, '09:00', 30, '2026-03-21', '2026-09-06'],
  ['Sonic The Hedgehog', F, '09:30', 30, '2026-03-21', '2026-12-20'],
  ['Astroboy', F, '10:00', 30, '2026-01-04', '2026-12-27'],
  ['Corrector Yui', F, '10:30', 30, '2026-01-04', '2026-11-29'],
  ['Voyagesea', F, '14:00', 60, '2026-05-02', '2026-10-11'],
  ['SaberMarionette', F, '23:00', 30, '2026-05-02', '2026-10-11'],
  ["BT'x", F, '23:30', 30, '2026-05-02', '2026-10-11'],
  ['Hack Legend', F, '00:00', 30, '2026-04-11', '2026-09-14'],
  ['Los Lorcanitos', F, '17:00', 60, '2026-06-06', '2026-11-15'],
  ['Astroboy', F, '11:00', 30, '2026-09-05', '2026-12-27'],
]

function diasEntreFechas(a: string, b: string): number {
  const [a1, m1, d1] = a.split('-').map(Number)
  const [a2, m2, d2] = b.split('-').map(Number)
  return Math.round((Date.UTC(a2, m2 - 1, d2) - Date.UTC(a1, m1 - 1, d1)) / 86_400_000)
}

export const reglas: Regla[] = SEMILLAS.map(
  ([titulo, patron, hhmm, slotMin, desde, hasta, extra], i) => {
    const [h, m] = hhmm.split(':').map(Number)
    return {
      id: i + 1,
      tipo: 'normal',
      title_id: null,
      live_source_id: null,
      titulo,
      patron_de_dias: patron,
      hora: h * 60 + m,
      duracion_slot_ms: slotMin * 60_000,
      fecha_inicio: desde,
      fecha_fin: hasta,
      dias_restantes: diasEntreFechas(HOY, hasta),
      episodios_por_corrida: 1,
      releva_a: null,
      repite_a: null,
      activa: true,
      ...extra,
    } satisfies Regla
  },
)

// ── la biblioteca ─────────────────────────────────────────────────────

const SIN_PROGRAMAR = [
  'Starsky & Hutch',
  'The Time Tunnel',
  'Land of the Giants',
  'Serial Experiments Lain',
  'Cybersix',
  'The Munsters',
  'Space Cobra',
  'Kimba',
  'El Chavo animado',
  'Los Picapiedra',
]

const SINOPSIS: Record<string, string> = {
  Kojak:
    'El teniente Theo Kojak es el protagonista de este popular drama policial. Kojak es un policía duro, pero su marca personal es su afición por los chupetes.',
  'RadioOnce Live!':
    'La emisora de radio de la casa, simulcasteada por la antena. Sale en vivo de lunes a viernes, de 10:00 AM a 1:00 PM.',
  'Los Simuladores':
    'Cuatro hombres resuelven por encargo los problemas que nadie más puede resolver, montando situaciones a la medida.',
  'Gaming Longplays':
    'Partidas completas de videojuegos clásicos, sin comentarios, para llenar la noche.',
  Zoids:
    'Máquinas de guerra con forma de animal en un planeta en conflicto. Releva a Magic Knight Rayearth desde el 8 de septiembre.',
}

const ANIOS: Record<string, number> = {
  Kojak: 1973,
  'Get Smart': 1965,
  'I Dream of Jeannie': 1965,
  Tarzan: 1966,
  "Gilligan's Island": 1964,
  'Los Simuladores': 2002,
  'Mazinger Z': 1972,
  'Green Hornet': 1966,
  Astroboy: 1963,
  'The Munsters': 1964,
  'Starsky & Hutch': 1975,
  'The Time Tunnel': 1966,
  'Land of the Giants': 1968,
  'Serial Experiments Lain': 1998,
  Cybersix: 1999,
  'Space Cobra': 1982,
  Zoids: 1999,
  'Magic Knight Rayearth': 1994,
}

function iniciales(nombre: string): string {
  const limpio = nombre.replace(/['’]/g, '').replace(/[^A-Za-z0-9 ]/g, ' ').trim()
  const palabras = limpio.split(/\s+/).filter(Boolean)
  if (palabras.length === 0) return '??'
  if (palabras.length === 1) return palabras[0][0].toUpperCase()
  return (palabras[0][0] + palabras[1][0]).toUpperCase()
}

const nombresProgramados = Array.from(
  new Set(Object.values(SEMANA_CATV).flat().filter(Boolean)),
)

// ── el sonido de los archivos de ejemplo ──────────────────────────────
// El número del archivo (material_id) sale del título: el título mismo lleva
// id * 1000 y sus episodios id * 1000 + n, que es lo que reparte episodiosDe.

export const materialDeTitulo = (idTitulo: number): number => idTitulo * 1000

/** Gaming Longplays llega con dos pistas: primero la inglesa, después la nuestra. */
const PISTAS_INGLES_ESPANOL: PistaDeAudio[] = [
  { indice: 1, idioma: 'eng', canales: 2, titulo: '' },
  { indice: 2, idioma: 'spa', canales: 2, titulo: 'Doblaje' },
]

/** Kojak T1 E1 llega con la pista de casa primero. */
const PISTAS_ESPANOL_INGLES: PistaDeAudio[] = [
  { indice: 1, idioma: 'spa', canales: 2, titulo: '' },
  { indice: 2, idioma: 'eng', canales: 6, titulo: 'Original' },
]

/** Títulos que traen varias pistas o un archivo de al lado. */
const AUDIO_POR_TITULO: Record<
  string,
  { pistas?: PistaDeAudio[]; aire?: number; audio?: string; subtitulos?: string }
> = {
  'Gaming Longplays': { pistas: PISTAS_INGLES_ESPANOL, aire: 2 },
  Voyagesea: {
    audio: 'D:\\Contenido\\Voyagesea\\voyagesea.wav',
    subtitulos: 'D:\\Contenido\\Voyagesea\\voyagesea.srt',
  },
}

export const titulos: TituloDeBiblioteca[] = [
  ...nombresProgramados,
  ...SIN_PROGRAMAR,
].map((nombre, i) => {
  const regla = reglas.find((r) => r.titulo === nombre)
  const enCuarentena = nombre === 'Space Cobra'
  const sinNormalizar = nombre === 'Cybersix'
  const audio = AUDIO_POR_TITULO[nombre]
  return {
    id: i + 1,
    material_id: materialDeTitulo(i + 1),
    pistas_audio: audio?.pistas ?? [],
    pista_audio_aire: audio?.aire ?? 1,
    audio_sidecar: audio?.audio ?? '',
    subtitulos_sidecar: audio?.subtitulos ?? '',
    nombre,
    tipo: nombre === 'RadioOnce Live!' ? 'programa' : 'serie',
    sinopsis:
      SINOPSIS[nombre] ??
      `${nombre} está en la biblioteca del canal, importado de la carpeta de contenido.`,
    anio: ANIOS[nombre] ?? null,
    clasificacion_contenido: nombre === 'Kojak' ? 'TV-PG' : 'TV-G',
    caratula: iniciales(nombre),
    episodios: nombre === 'RadioOnce Live!' ? 0 : 8 + ((i * 13) % 110),
    duracion_ms: (regla?.duracion_slot_ms ?? 1_800_000) - 6 * 60_000,
    estado_material: enCuarentena
      ? 'cuarentena'
      : sinNormalizar
        ? 'aún no listo para aire'
        : 'listo',
    en_la_parrilla: Boolean(regla),
    hora: regla ? `${String(Math.floor(regla.hora / 60)).padStart(2, '0')}:${String(regla.hora % 60).padStart(2, '0')}` : null,
    regla_hasta: regla?.fecha_fin ?? null,
  } satisfies TituloDeBiblioteca
})

export function episodiosDe(titulo: TituloDeBiblioteca): EpisodioDeBiblioteca[] {
  const out: EpisodioDeBiblioteca[] = []
  const cuantos = Math.min(titulo.episodios, 24)
  for (let i = 0; i < cuantos; i++) {
    // Kojak es el ejemplo con sonido: el primer episodio trae dos pistas y el
    // segundo llegó mudo, con el audio en un archivo de al lado.
    const dosPistas = titulo.nombre === 'Kojak' && i === 0
    const conAudioAlLado = titulo.nombre === 'Kojak' && i === 1
    out.push({
      id: titulo.id * 1000 + i + 1,
      material_id: titulo.id * 1000 + i + 1,
      temporada: 1 + Math.floor(i / 12),
      numero: (i % 12) + 1,
      nombre: `Episodio ${i + 1}`,
      duracion_ms: titulo.duracion_ms,
      estado_material:
        i === 3 && titulo.nombre === 'Zorro 57' ? 'aún no listo para aire' : 'listo',
      pistas_audio: dosPistas ? PISTAS_ESPANOL_INGLES : [],
      pista_audio_aire: 1,
      audio_sidecar: conAudioAlLado ? 'D:\\Contenido\\Kojak\\kojak-t1e02.m4a' : '',
      subtitulos_sidecar: conAudioAlLado ? 'D:\\Contenido\\Kojak\\kojak-t1e02.srt' : '',
    })
  }
  return out
}

export const cuarentena: EnCuarentena[] = [
  {
    id: 9001,
    ruta: 'D:\\Contenido\\Space Cobra\\space-cobra-e04.mp4',
    titulo: 'Space Cobra · episodio 4',
    motivo_en_cristiano: 'Este video no trae sonido.',
    motivo_codigo: 'sin_audio',
    creado: '2026-09-03T14:12:00Z',
  },
  {
    id: 9002,
    ruta: 'D:\\Contenido\\Promos\\promo-verano.mov',
    titulo: 'Promo de verano',
    motivo_en_cristiano:
      'El video se corta a los 12 segundos: el archivo llegó incompleto.',
    motivo_codigo: 'incompleto',
    creado: '2026-09-02T19:40:00Z',
  },
  {
    id: 9003,
    ruta: 'D:\\Contenido\\Kojak\\kojak-t2e07.mp4',
    titulo: 'Kojak · T2 E7',
    motivo_en_cristiano:
      'Falló dos veces al aire con más de cinco minutos de diferencia.',
    motivo_codigo: 'fallo_al_aire',
    creado: '2026-08-30T02:05:00Z',
  },
]

export const ajustes: Ajustes = {
  version: '1.0.0',
  sistema_operativo: 'Windows 10 Pro',
  dias_al_aire: '34',
  hora_servidor: 'time.nist.gov',
  hora_desvio_s: '0.08',
  hora_aviso_si_pasa_de_s: '1',
  aceleracion_tarjeta: 'NVIDIA',
  aceleracion_probada: 'al arrancar',
  aceleracion_resultado: 'OK',
  silencio_avisa: 'sí',
  silencio_umbral_s: '15',
  antivirus_exclusiones: 'sí',
  energia_plan: 'sí',
  arranque_tras_corte: 'sí',
  rutas_largas: 'sí',
  actualizaciones_windows: 'no',
  respaldo_ultimo: '2026-09-04T17:04:00Z',
  respaldo_cada: '1 hora',
  respaldo_copias: '168 · 7 días',
  respaldo_tamano: '12.4 MB',
  tailscale: 'conectado',
  tailscale_direccion: 'antena787-catv',
  puertos_abiertos: 'ninguno',
  actualizaciones_disponible: '1.0.1',
  actualizaciones_instalar_sola: 'nunca',
  pais: 'Estados Unidos',
  perfil: 'FCC · ATSC',
  volumen: 'volumen de televisión de EE. UU.',
  subtitulos: 'se conservan',
  equipo_de_alertas: 'Sage ENDEC · por red',
  asistente_ia: 'apagado',
  // Avisos: por dónde sale el aviso de que una regla se vence sin relevo.
  avisos_canal: 'ninguno',
  avisos_telegram_token: '',
  avisos_telegram_chat: '',
  avisos_correo_para: '',
  avisos_smtp_servidor: '',
  avisos_smtp_usuario: '',
  avisos_smtp_clave: '',
  // Fichas de programas: sinopsis y carátulas cuando el archivo no las trae.
  fichas_en_linea: 'no',
  clave_tmdb: '',
  // Guía: además del archivo, mandarla a una dirección.
  guia_destino_http: '',
  // Audio: con qué idioma se queda cuando el archivo trae varias pistas.
  idioma_audio_preferido: 'es',
}
