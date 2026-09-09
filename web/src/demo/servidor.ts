// Servidor de mentira: responde exactamente las rutas de docs/API.md con los
// datos de datos.ts. Se enciende con VITE_DEMO=1, o solo cuando el servidor
// de verdad no contesta, para que la interfaz nunca quede en blanco.
//
// Simplificación consciente: Puerto Rico no cambia la hora, así que aquí la
// zona del canal es un desfase fijo de −4. El servidor Go de verdad hace las
// cuentas con la zona horaria completa.

import type {
  Ajustes,
  Alarma,
  DetectadoEnLaMaquina,
  ElementoDelPlan,
  EpisodioDeBiblioteca,
  Estado,
  FilaDelPlan,
  FranjaSemana,
  Guia,
  Instalacion,
  MesDelPlan,
  Opcion,
  OpcionesDelAsistente,
  Regla,
  RespuestasDelAsistente,
  ResumenDeImportacion,
  SemanaDelPlan,
  TituloDeBiblioteca,
} from '../lib/tipos'
import {
  AHORA_BASE,
  ajustes as ajustesDemo,
  canal,
  cuarentena,
  EN_VIVO,
  episodiosDe,
  FRANJAS,
  programaEn,
  reglas as reglasDemo,
  salidas,
  titulos,
} from './datos'

const DESFASE_H = -4 // America/Puerto_Rico, sin horario de verano

let reglas: Regla[] = reglasDemo.map((r) => ({ ...r }))
let ajustes: Ajustes = { ...ajustesDemo }
const dejadosPasar = new Set<number>()
let siguienteId = 1000

/**
 * La pista de sonido que alguien eligió para un archivo (F1-61). El servidor
 * de verdad lo guarda en el media_asset y lo manda a rehacer; aquí basta con
 * acordarse y contestar «aún no listo para aire» de ahí en adelante.
 */
const pistaElegida = new Map<number, number>()

/**
 * Lo que alguien movió a mano en la parrilla. El servidor de verdad lo guarda
 * en el plan_item; aquí basta con acordarse del cambio y aplicarlo encima de
 * lo que calculan las reglas.
 */
interface CambioAMano {
  instante_planeado?: string
  duracion_planeada_ms?: number
  fijado: boolean
}
const aMano = new Map<number, CambioAMano>()

/** El reloj del canal: arranca en el instante de los mockups y corre. */
export function ahoraDemo(): Date {
  return new Date(AHORA_BASE + (Date.now() - arranqueReal))
}
const arranqueReal = Date.now()

function utcDe(dia: string, minutos: number): Date {
  const [a, m, d] = dia.split('-').map(Number)
  return new Date(Date.UTC(a, m - 1, d, -DESFASE_H, 0, 0) + minutos * 60_000)
}

function hhmm(minutos: number): string {
  return `${String(Math.floor(minutos / 60)).padStart(2, '0')}:${String(minutos % 60).padStart(2, '0')}`
}

function sumar(dia: string, n: number): string {
  const [a, m, d] = dia.split('-').map(Number)
  return new Date(Date.UTC(a, m - 1, d + n)).toISOString().slice(0, 10)
}

/** El día de emisión al que pertenece un instante (empieza a las 6:00 AM). */
function diaEmisionDe(t: Date): string {
  const local = new Date(t.getTime() + DESFASE_H * 3_600_000)
  const corrido = new Date(local.getTime() - canal.hora_inicio_dia_emision * 60_000)
  return corrido.toISOString().slice(0, 10)
}

// ── el plan de un día ─────────────────────────────────────────────────

/**
 * Junta franjas consecutivas del mismo título en un solo elemento del plan.
 * La duración real es seis minutos menos que el slot: ese sobrante es el
 * hueco que la interfaz tiene que enseñar (PRD §13).
 */
interface BloqueDemo {
  item: ElementoDelPlan
  desdeMin: number
  hastaMin: number
}

/** El minuto del reloj del canal al que cae un instante. */
function minutoLocalDe(instante: string): number {
  const t = new Date(Date.parse(instante) + DESFASE_H * 3_600_000)
  return t.getUTCHours() * 60 + t.getUTCMinutes()
}

function bloquesDelDia(dia: string): BloqueDemo[] {
  const out: BloqueDemo[] = []
  let i = 0
  let n = 0
  while (i < FRANJAS.length) {
    const nombre = programaEn(dia, FRANJAS[i])
    if (!nombre) {
      i++
      continue
    }
    let j = i
    while (j + 1 < FRANJAS.length && programaEn(dia, FRANJAS[j + 1]) === nombre) j++
    const inicioMin = i * 30
    const slotMs = (j - i + 1) * 30 * 60_000
    const enVivo = EN_VIVO.has(nombre)
    const item: ElementoDelPlan = {
      id: Number(dia.replace(/-/g, '')) * 100 + n++,
      dia_emision: dia,
      instante_planeado: utcDe(dia, inicioMin).toISOString(),
      duracion_planeada_ms: enVivo ? slotMs : slotMs - 6 * 60_000,
      origen: enVivo ? 'live_source' : 'asset',
      estado: 'planned',
      hora_local: hhmm(inicioMin),
      titulo: nombre,
      temporada: enVivo ? null : 1 + ((i + dia.length) % 3),
      episodio: enVivo ? null : 1 + ((i * 7 + dia.charCodeAt(9)) % 26),
      en_vivo: enVivo,
    }
    let desde = inicioMin
    let hasta = inicioMin + (j - i + 1) * 30
    const cambio = aMano.get(item.id)
    if (cambio) {
      if (cambio.instante_planeado) {
        item.instante_planeado = new Date(Date.parse(cambio.instante_planeado)).toISOString()
        desde = minutoLocalDe(item.instante_planeado)
        hasta = desde + (j - i + 1) * 30
        item.hora_local = hhmm(desde % (24 * 60))
      }
      if (cambio.duracion_planeada_ms) item.duracion_planeada_ms = cambio.duracion_planeada_ms
      if (cambio.fijado) item.fijado = true
    }
    out.push({ item, desdeMin: desde, hastaMin: hasta })
    i = j + 1
  }
  out.sort((a, b) => a.desdeMin - b.desdeMin)
  return out
}

function elementosDelDia(dia: string): ElementoDelPlan[] {
  return bloquesDelDia(dia).map((b) => b.item)
}

/** Busca un plan_item por su número en el día que lleva codificado. */
function bloquePorId(id: number): BloqueDemo | null {
  const codigo = String(Math.floor(id / 100))
  if (codigo.length !== 8) return null
  const dia = `${codigo.slice(0, 4)}-${codigo.slice(4, 6)}-${codigo.slice(6, 8)}`
  return bloquesDelDia(dia).find((b) => b.item.id === id) ?? null
}

/** El día de emisión completo: de las 6:00 AM a las 6:00 AM del otro día. */
function planDelDiaEmision(dia: string): FilaDelPlan[] {
  const inicio = canal.hora_inicio_dia_emision
  const hoy = elementosDelDia(dia).filter((e) => minutoLocal(e) >= inicio)
  const manana = elementosDelDia(sumar(dia, 1))
    .filter((e) => minutoLocal(e) < inicio)
    .map((e) => ({ ...e, dia_emision: dia }))
  const items = [...hoy, ...manana]
  const filas: FilaDelPlan[] = []
  let cursor = utcDe(dia, inicio).getTime()
  const fin = utcDe(sumar(dia, 1), inicio).getTime()
  for (const it of items) {
    const t = Date.parse(it.instante_planeado)
    if (t - cursor >= 60_000) {
      filas.push({
        hueco: true,
        inicio: new Date(cursor).toISOString(),
        fin: new Date(t).toISOString(),
      })
    }
    filas.push(it)
    cursor = t + it.duracion_planeada_ms
  }
  if (fin - cursor >= 60_000) {
    filas.push({
      hueco: true,
      inicio: new Date(cursor).toISOString(),
      fin: new Date(fin).toISOString(),
    })
  }
  return filas
}

function minutoLocal(e: ElementoDelPlan): number {
  const [h, m] = e.hora_local.split(':').map(Number)
  return h * 60 + m
}

/** Franjas de 30 min llenas o vacías, en el día de emisión que empieza a las 6. */
function franjasDelDiaEmision(dia: string): boolean[] {
  const out: boolean[] = []
  const inicio = canal.hora_inicio_dia_emision / 30
  for (let k = 0; k < 48; k++) {
    const idx = (k + inicio) % 48
    const d = k + inicio >= 48 ? sumar(dia, 1) : dia
    out.push(Boolean(programaEn(d, FRANJAS[idx])))
  }
  return out
}

function horasVacias(dia: string): number {
  return franjasDelDiaEmision(dia).filter((x) => !x).length / 2
}

// ── el estado ─────────────────────────────────────────────────────────

function alarmasDe(dia: string): Alarma[] {
  const vencen = reglas.filter((r) => r.dias_restantes >= 0 && r.dias_restantes <= 7)
  const filas = planDelDiaEmision(dia)
  const huecoGordo = filas.find(
    (f) => 'hueco' in f && Date.parse(f.fin) - Date.parse(f.inicio) >= 3_600_000,
  )
  const out: Alarma[] = [
    {
      tipo: 'plan',
      nivel: 'bien',
      texto: 'Programación cargada',
      detalle: 'para los próximos 12 días',
    },
  ]
  if (vencen.length) {
    const primera = [...vencen].sort((a, b) => a.dias_restantes - b.dias_restantes)[0]
    out.push({
      tipo: 'vencimiento',
      nivel: 'aviso',
      texto: `${vencen.length} programas se vencen`,
      detalle:
        primera.dias_restantes === 0
          ? `${primera.titulo} hoy`
          : `${primera.titulo} en ${primera.dias_restantes} ${primera.dias_restantes === 1 ? 'día' : 'días'}`,
      accion: { texto: 'ver', ruta: '/reglas?filtro=vencen' },
    })
  }
  if (huecoGordo && 'hueco' in huecoGordo) {
    const ms = Date.parse(huecoGordo.fin) - Date.parse(huecoGordo.inicio)
    out.push({
      tipo: 'hueco',
      nivel: 'problema',
      texto: `${Math.round(ms / 3_600_000)} horas sin programar`,
      detalle: horaLocalDe(huecoGordo.inicio) + ' – ' + horaLocalDe(huecoGordo.fin),
      accion: { texto: 'llenar', ruta: '/parrilla' },
    })
  }
  out.push({
    tipo: 'material',
    nivel: 'bien',
    texto: `${titulos
      .filter((t) => t.estado_material === 'listo')
      .reduce((a, t) => a + Math.max(1, t.episodios), 0)} videos listos`,
    detalle: '890 GB libres · sitio para unas 400 horas más',
  })
  return out
}

function horaLocalDe(instante: string): string {
  const t = new Date(Date.parse(instante) + DESFASE_H * 3_600_000)
  const h = t.getUTCHours()
  const m = t.getUTCMinutes()
  const s = h < 12 ? 'AM' : 'PM'
  return `${h % 12 === 0 ? 12 : h % 12}:${String(m).padStart(2, '0')} ${s}`
}

function estado(): Estado {
  const ahora = ahoraDemo()
  const dia = diaEmisionDe(ahora)
  const filas = planDelDiaEmision(dia)
  const items = filas.filter((f): f is ElementoDelPlan => !('hueco' in f))
  const t = ahora.getTime()
  const alAire =
    items.find(
      (e) =>
        Date.parse(e.instante_planeado) <= t &&
        Date.parse(e.instante_planeado) + e.duracion_planeada_ms > t,
    ) ?? null
  const siguiente = items.find((e) => Date.parse(e.instante_planeado) > t) ?? null
  return {
    canal,
    modo: canal.modo,
    ahora: ahora.toISOString(),
    dia_emision: dia,
    al_aire: alAire,
    siguiente,
    alarmas: alarmasDe(dia),
    salidas,
    version: ajustes.version ?? '1.0.0',
    retorno_de_aire: { hay: false, texto: 'Todavía no hay retorno de aire conectado' },
    control_manual: { activo: false },
    // En modo demostración siempre se está adentro: no hay clave que pedir.
    entraste: true,
    necesita_instalacion: !asistente.completa,
    instalacion_completa: asistente.completa,
  }
}

// ── las pantallas de parrilla ─────────────────────────────────────────

function semana(desde: string): SemanaDelPlan {
  const dias = []
  for (let i = 0; i < 7; i++) {
    const dia = sumar(desde, i)
    const franjas: FranjaSemana[] = Array.from({ length: 48 }, () => ({
      titulo: null,
      en_vivo: false,
      duracion_ms: 30 * 60_000,
      plan_id: null,
    }))
    for (const b of bloquesDelDia(dia)) {
      for (let k = Math.floor(b.desdeMin / 30); k < Math.ceil(b.hastaMin / 30); k++) {
        if (k < 0 || k > 47) continue
        franjas[k] = {
          titulo: b.item.titulo ?? null,
          en_vivo: Boolean(b.item.en_vivo),
          duracion_ms: 30 * 60_000,
          plan_id: b.item.id,
          fijado: b.item.fijado,
        }
      }
    }
    dias.push({ dia, franjas, horas_vacias: horasVacias(dia) })
  }
  return {
    desde,
    dias,
    nota: 'Además, 1:00 – 6:00 AM está vacío los siete días',
  }
}

function mes(mesStr: string): MesDelPlan {
  const [a, m] = mesStr.split('-').map(Number)
  const cuantos = new Date(Date.UTC(a, m, 0)).getUTCDate()
  const dias = []
  let vacias = 0
  for (let d = 1; d <= cuantos; d++) {
    const dia = `${mesStr}-${String(d).padStart(2, '0')}`
    const franjas = franjasDelDiaEmision(dia)
    const sin = franjas.filter((x) => !x).length / 2
    vacias += sin
    dias.push({
      dia,
      horas_sin_llenar: sin,
      franjas_llenas: franjas,
      vencimientos: [
        ...new Set(reglas.filter((r) => r.fecha_fin === dia).map((r) => r.titulo)),
      ],
      estrenos: [
        ...new Set(
          reglas
            .filter((r) => r.fecha_inicio === dia && r.fecha_inicio > '2026-09-01')
            .map((r) => r.titulo),
        ),
      ],
    })
  }
  return {
    mes: mesStr,
    dias,
    horas_vacias_mes: vacias,
    porcentaje_vacio: Math.round((vacias / (cuantos * 24)) * 100),
  }
}

function guia(dia: string): Guia {
  const items = elementosDelDia(dia).filter((e) => minutoLocal(e) >= 12 * 60 && minutoLocal(e) < 18 * 60)
  const filas = items.map((e) => {
    // La guía se publicó antes del relevo: sigue anunciando el programa viejo.
    const desalineado = e.titulo === 'Zoids' && e.hora_local === '15:00'
    return {
      guia: {
        titulo: desalineado ? 'Magic Knight Rayearth' : (e.titulo ?? ''),
        inicio: e.instante_planeado,
        duracion_ms: e.duracion_planeada_ms,
      },
      plan: {
        titulo: e.titulo ?? '',
        inicio: e.instante_planeado,
        duracion_ms: e.duracion_planeada_ms,
      },
      coincide: !desalineado,
    }
  })
  const malas = filas.filter((f) => !f.coincide).length
  return {
    dia,
    filas,
    identificador_de_canal: 'catv.pr',
    revalidada: new Date(ahoraDemo().getTime() - 6 * 60_000).toISOString(),
    por_que_no_coinciden: malas
      ? 'La regla de Magic Knight Rayearth venció el 7 de septiembre y desde el 8 Zoids ocupa las 3:00 PM. La guía se publicó antes del relevo y quedó anunciando el programa viejo.'
      : undefined,
  }
}

// ── importar desde la hoja ────────────────────────────────────────────

function importarHoja(texto: string): ResumenDeImportacion {
  const lineas = texto.split('\n').map((l) => l.trimEnd()).filter((l) => l.trim() !== '')
  const errores: ResumenDeImportacion['filas_con_error'] = []
  const corridas: ResumenDeImportacion['fechas_corridas'] = []
  let creadas = 0
  let nuevosTitulos = 0
  const vistos = new Set(titulos.map((t) => t.nombre))
  lineas.forEach((linea, i) => {
    const celdas = linea.split(/\t|\s{2,}|,/).map((c) => c.trim()).filter(Boolean)
    if (celdas.length < 2) {
      errores.push({ fila: i + 1, texto: linea, motivo: 'la fila no trae hora ni programa' })
      return
    }
    const [primera, ...resto] = celdas
    const hora = primera.match(/^(\d{1,2}):(\d{2})\s*(AM|PM|am|pm)?/)
    if (!hora) {
      errores.push({ fila: i + 1, texto: linea, motivo: 'la primera celda no es una hora' })
      return
    }
    let h = Number(hora[1]) % 12
    if (/pm/i.test(hora[3] ?? '')) h += 12
    const nombre = resto.join(' ')
    if (!vistos.has(nombre)) {
      vistos.add(nombre)
      nuevosTitulos++
    }
    creadas++
    if (h >= 0 && h < 6) {
      corridas.push({
        fila: i + 1,
        texto: `${nombre} a las ${hora[0]}: la fecha se corrió un día atrás para que caiga en el día de emisión que le toca`,
      })
    }
  })
  return {
    reglas_creadas: creadas,
    titulos_creados: nuevosTitulos,
    relevos_propuestos: creadas
      ? [
          {
            regla: 12,
            releva_a: 11,
            texto: '¿Zoids releva a Magic Knight Rayearth?',
          },
        ]
      : [],
    filas_con_error: errores,
    fechas_corridas: corridas,
  }
}

// ── el sonido de los archivos ─────────────────────────────────────────

type ConAudio = TituloDeBiblioteca | EpisodioDeBiblioteca

/** Aplica encima lo que alguien eligió a mano para ese archivo. */
function conPistaElegida<T extends ConAudio>(m: T): T {
  const id = m.material_id
  if (id === undefined || !pistaElegida.has(id)) return m
  return {
    ...m,
    pista_audio_aire: pistaElegida.get(id),
    estado_material: 'aún no listo para aire',
  }
}

/** Busca un archivo por su número, sea el del título o el de un episodio. */
function materialPorId(id: number): ConAudio | null {
  const titulo = titulos.find((t) => t.material_id === id)
  if (titulo) return titulo
  const dueno = titulos.find((t) => t.id === Math.floor(id / 1000))
  if (!dueno) return null
  return episodiosDe(dueno).find((e) => e.material_id === id) ?? null
}

// ── el asistente de instalación ───────────────────────────────────────

/**
 * El asistente de mentira. Guarda lo contestado en localStorage para que
 * recargar la página caiga en el mismo paso, igual que con el servidor de
 * verdad (que lo guarda en `settings`).
 *
 * Para verlo desde el paso 1 hay dos maneras: `VITE_DEMO_INSTALAR=1` al
 * levantar Vite, o abrir la interfaz con `?instalar=1` en la dirección. Sin
 * eso, el canal de ejemplo ya está instalado y el asistente se puede visitar
 * con todo contestado.
 */
const LLAVE_ASISTENTE = 'antena787.demo.asistente'

interface EstadoDelAsistente {
  paso: number
  completa: boolean
  respuestas: RespuestasDelAsistente
  tiempos: Record<string, string>
  carpeta_contenido: string
  relleno: number
  reglas_creadas: number
}

function quiereInstalar(): boolean {
  if (import.meta.env.VITE_DEMO_INSTALAR === '1') return true
  try {
    return new URLSearchParams(window.location.search).get('instalar') === '1'
  } catch {
    return false
  }
}

function asistenteEnBlanco(): EstadoDelAsistente {
  if (quiereInstalar()) {
    return {
      paso: 1,
      completa: false,
      respuestas: {},
      tiempos: {},
      carpeta_contenido: '',
      relleno: 0,
      reglas_creadas: 0,
    }
  }
  // El canal de ejemplo ya está instalado: el asistente se ve contestado.
  return {
    paso: 9,
    completa: true,
    respuestas: {
      '1': {
        nombre: canal.nombre,
        identificativo: canal.identificativo,
        comunidad_licencia: canal.comunidad_licencia,
        nombre_operador: 'Rolando',
      },
      '2': { modo: 'transmisor' },
      '4': { destino: 'red', retorno_de_aire: 'receptor-tv' },
      '5': { ve_barras: true },
      '6': { pais: 'Puerto Rico', calidad: '1080i59.94' },
      '7': { carpeta: '/srv/antena787/contenido' },
      '8': { propuesta: 'automatica' },
    },
    tiempos: {},
    carpeta_contenido: '/srv/antena787/contenido',
    relleno: 1,
    reglas_creadas: 6,
  }
}

function leerAsistente(): EstadoDelAsistente {
  // Sin la marca de instalar, el canal de ejemplo ya está montado: nunca se
  // lee lo guardado, para que una prueba a medias no deje la demostración
  // pidiendo instalación para siempre.
  if (!quiereInstalar()) return asistenteEnBlanco()
  try {
    // `?instalar=1&reiniciar=1` empieza el asistente desde el paso 1 otra vez.
    if (new URLSearchParams(window.location.search).get('reiniciar') === '1') {
      window.localStorage.removeItem(LLAVE_ASISTENTE)
      return asistenteEnBlanco()
    }
    const crudo = window.localStorage.getItem(LLAVE_ASISTENTE)
    if (crudo) return { ...asistenteEnBlanco(), ...(JSON.parse(crudo) as EstadoDelAsistente) }
  } catch {
    // Sin localStorage (una ventana privada) el asistente sigue funcionando,
    // solo que recargar lo devuelve al principio.
  }
  return asistenteEnBlanco()
}

let asistente: EstadoDelAsistente = leerAsistente()

function guardarAsistente() {
  try {
    window.localStorage.setItem(LLAVE_ASISTENTE, JSON.stringify(asistente))
  } catch {
    // Da igual: lo que importa es la sesión que se está mirando.
  }
}

function apuntarPaso(n: number) {
  // Hora de reloj de pared, no la del canal de ejemplo: es cuándo se contestó.
  asistente.tiempos[String(n)] = new Date().toISOString()
  asistente.paso = Math.min(n + 1, 9)
  guardarAsistente()
}

/**
 * Lo que el asistente propone en cada pregunta. Categorías en cristiano: el
 * valor es interno y nunca sale a pantalla (PRD §10). «Todavía no» está en la
 * lista como una respuesta más, no como el renglón chiquito del final.
 */
const OPCIONES: OpcionesDelAsistente = {
  modo: [
    {
      valor: 'internet',
      texto: 'Salir por internet',
      ayuda: 'El canal se ve en una página, una aplicación o una red social.',
    },
    {
      valor: 'transmisor',
      texto: 'Salir por transmisor',
      ayuda: 'La señal va a la caja que junta los canales, o directo al transmisor.',
    },
    {
      valor: 'no_se',
      texto: 'Todavía no sé',
      ayuda: 'Se sigue igual. Esto se cambia después sin volver a instalar.',
    },
  ],
  destino: [
    {
      valor: 'red',
      texto: 'A un equipo aquí en la estación',
      ayuda: 'Por el cable de red, a la caja que junta los canales antes del transmisor.',
    },
    {
      valor: 'internet',
      texto: 'A internet',
      ayuda: 'A un servicio de video en línea, para que se vea desde afuera.',
    },
    {
      valor: 'archivo',
      texto: 'A un archivo en esta máquina',
      ayuda: 'Se graba lo que saldría al aire. Sirve para mirar sin emitir.',
    },
    {
      valor: 'ninguna',
      texto: 'Todavía no',
      ayuda: 'No hay a dónde mandarla por ahora. Se puede seguir y decidirlo cuando toque.',
    },
  ],
  retorno: [
    {
      valor: 'receptor-tv',
      texto: 'Esta máquina sintoniza el canal',
      ayuda: 'Tiene adentro una tarjeta que recibe la señal, como un televisor.',
    },
    {
      valor: 'captura',
      texto: 'Entra un cable de video a esta máquina',
      ayuda: 'Viene del transmisor o de un receptor que está en la estación.',
    },
    {
      valor: 'stream',
      texto: 'Hay una dirección para mirar el canal',
      ayuda: 'El transmisor o la red publican lo que sale y se puede ver por ahí.',
    },
    {
      valor: 'ninguno',
      texto: 'Todavía no',
      ayuda: 'Se puede seguir. Sin verla de vuelta no se puede comparar lo que sale con lo que emites, y se dice.',
    },
  ],
  calidad: [
    {
      valor: '1080i59.94',
      texto: '1080i a 59.94',
      ayuda: 'Lo que espera casi todo equipo en Estados Unidos y Puerto Rico.',
    },
    { valor: '720p59.94', texto: '720p a 59.94' },
    { valor: '1080p29.97', texto: '1080p a 29.97' },
    { valor: '1080p59.94', texto: '1080p a 59.94' },
    {
      valor: '480i59.94',
      texto: '480i a 59.94',
      ayuda: 'Televisión estándar, la de antes de la alta definición.',
    },
    { valor: '1080i50', texto: '1080i a 50', ayuda: 'Europa, América del Sur y buena parte del resto.' },
    { valor: '720p50', texto: '720p a 50' },
    { valor: '1080p25', texto: '1080p a 25' },
    { valor: '576i50', texto: '576i a 50', ayuda: 'Televisión estándar fuera de América del Norte.' },
  ],
}

const ACELERACION = 'se mide al arrancar el motor (F2); todavía no hay motor'

function detectado(): DetectadoEnLaMaquina {
  return {
    ffmpeg: '/usr/local/bin/ffmpeg 7.1',
    ffprobe: '/usr/local/bin/ffprobe 7.1',
    carpeta_datos: '/var/lib/antena787',
    carpeta_contenido: asistente.carpeta_contenido,
    carpeta_respaldo: '/var/lib/antena787/respaldo',
    relleno: asistente.relleno,
    aceleracion: ACELERACION,
    disco: 'El disco donde va el contenido tiene 890 GB libres.',
    red: 'Hay una tarjeta de red conectada, con dirección fija.',
  }
}

function instalacion(): Instalacion {
  return {
    paso: asistente.paso,
    pasos: 9,
    completa: asistente.completa,
    necesita_instalacion: !asistente.completa,
    canal,
    detectado: detectado(),
    opciones: OPCIONES,
    respuestas: asistente.respuestas,
    tiempos: asistente.tiempos,
  }
}

/** El texto del formato, como lo arma `app.FormatOf` en el servidor de verdad. */
function formatoDe(calidad: string): string {
  const m = /^(\d+)([ip])([\d.]+)$/.exec(calidad)
  if (!m) return calidad
  const alto = Number(m[1])
  const ancho = { 480: 720, 576: 720, 720: 1280, 1080: 1920 }[alto] ?? 1920
  const campos = Number(m[3])
  const cuadros = m[2] === 'i' ? campos / 2 : campos
  const texto = Number.isInteger(cuadros) ? String(cuadros) : cuadros.toFixed(2)
  return `${ancho}x${alto} a ${texto}`
}

function perfilDePais(pais: string): string {
  const p = pais.trim().toLowerCase()
  if (!p) return ''
  if (['pr', 'puerto rico', 'us', 'usa', 'eeuu', 'estados unidos'].includes(p)) return 'us-fcc'
  return 'abierto'
}

function tieneOpcion(lista: Opcion[], valor: string): boolean {
  return lista.some((o) => o.valor === valor)
}

/**
 * Los nueve pasos. Cada uno contesta {paso, siguiente} más lo suyo, y los
 * errores traen la frase en cristiano y el campo, igual que el servidor Go.
 */
function pasoDelAsistente(n: number, cuerpo: Record<string, unknown> | undefined): Response {
  const c = cuerpo ?? {}
  // El paso 9 no tiene siguiente: se queda en 9, igual que el servidor Go.
  const sigue = { paso: n, siguiente: Math.min(n + 1, 9) }
  switch (n) {
    case 1: {
      const nombre = String(c.nombre ?? '').trim()
      const clave = String(c.clave ?? '')
      if (!nombre)
        return json({ error: 'El canal necesita un nombre.', campo: 'nombre' }, 400)
      const yaHabiaClave = Boolean(asistente.respuestas['1'])
      if (!(clave === '' && yaHabiaClave) && !/^\d{4,6}$/.test(clave))
        return json(
          { error: 'la clave de la estación son de cuatro a seis dígitos', campo: 'clave' },
          400,
        )
      const identificativo = String(c.identificativo ?? '').trim()
      const comunidad = String(c.comunidad_licencia ?? '').trim()
      const operador = String(c.nombre_operador ?? '').trim()
      canal.nombre = nombre
      canal.identificativo = identificativo
      canal.comunidad_licencia = comunidad
      asistente.respuestas['1'] = {
        nombre,
        identificativo,
        comunidad_licencia: comunidad,
        nombre_operador: operador,
      }
      apuntarPaso(1)
      const cartel = [identificativo, comunidad].filter(Boolean).join(' · ') || nombre
      return json({ ...sigue, canal, cartel })
    }
    case 2: {
      const modo = String(c.modo ?? '')
      if (modo && modo !== 'no sé' && !tieneOpcion(OPCIONES.modo, modo))
        return json({ error: 'Escoge una de las tres.', campo: 'modo' }, 400)
      asistente.respuestas['2'] = { modo }
      apuntarPaso(2)
      return json({
        ...sigue,
        modo_del_canal: 'sombra',
        aviso:
          'esto queda apuntado; el canal sigue en modo sombra hasta que exista el motor de emisión',
      })
    }
    case 3: {
      apuntarPaso(3)
      const d = detectado()
      return json({ ...sigue, ffmpeg: d.ffmpeg, ffprobe: d.ffprobe })
    }
    case 4: {
      const destino = String(c.destino ?? '')
      const retorno = String(c.retorno_de_aire ?? '')
      if (destino && !tieneOpcion(OPCIONES.destino, destino))
        return json({ error: 'Escoge a dónde va la señal.', campo: 'destino' }, 400)
      if (retorno && !tieneOpcion(OPCIONES.retorno, retorno))
        return json(
          { error: 'Escoge cómo la puedes ver de vuelta.', campo: 'retorno_de_aire' },
          400,
        )
      asistente.respuestas['4'] = { destino, retorno_de_aire: retorno }
      apuntarPaso(4)
      const sinRetorno = !retorno || retorno === 'ninguno'
      return json({
        ...sigue,
        ...(sinRetorno
          ? {
              aviso:
                'sin retorno de aire no podemos comparar lo que sale con lo que emites: se puede seguir, y se dice.',
            }
          : {}),
      })
    }
    case 5: {
      if (typeof c.ve_barras !== 'boolean')
        return json({ error: 'Contesta sí o no.', campo: 've_barras' }, 400)
      asistente.respuestas['5'] = { ve_barras: c.ve_barras }
      apuntarPaso(5)
      return json({
        ...sigue,
        aviso:
          'la prueba de barras necesita el motor de emisión, que llega en F2. Tu respuesta queda apuntada.',
      })
    }
    case 6: {
      const pais = String(c.pais ?? '').trim()
      const calidad = String(c.calidad ?? '')
      if (!tieneOpcion(OPCIONES.calidad, calidad))
        return json({ error: 'Escoge una calidad de la lista.', campo: 'calidad' }, 400)
      canal.perfil_regulatorio = perfilDePais(pais)
      canal.perfil_de_formato = calidad
      asistente.respuestas['6'] = { pais, calidad }
      apuntarPaso(6)
      return json({ ...sigue, canal, formato: formatoDe(calidad) })
    }
    case 7: {
      const carpeta = String(c.carpeta ?? '').trim()
      if (!carpeta)
        return json({ error: 'Señala la carpeta donde están los videos.', campo: 'carpeta' }, 400)
      asistente.carpeta_contenido = carpeta
      asistente.respuestas['7'] = { carpeta }
      apuntarPaso(7)
      return json({
        ...sigue,
        carpeta,
        aviso_vigilancia:
          'ya la estoy mirando: lo que dejes ahí entra solo en cuanto termine de copiarse',
        ...(asistente.relleno === 0
          ? {
              aviso_relleno:
                'la biblioteca de relleno está vacía: el primer hueco saldría en negro.',
            }
          : {}),
      })
    }
    case 8: {
      const propuesta = String(c.propuesta ?? '')
      if (propuesta !== 'automatica' && propuesta !== 'ninguna')
        return json({ error: 'Escoge una de las dos.', campo: 'propuesta' }, 400)
      asistente.respuestas['8'] = { propuesta }
      if (propuesta === 'automatica') asistente.reglas_creadas = 6
      apuntarPaso(8)
      if (propuesta === 'ninguna')
        return json({
          ...sigue,
          reglas_creadas: 0,
          bloques: 0,
          avisos: [],
          titulos_sin_material: 0,
          aviso: 'queda para después: la parrilla se arma en la pantalla Parrilla cuando quieras.',
        })
      return json({
        ...sigue,
        reglas_creadas: 6,
        bloques: 84,
        avisos: [
          {
            tipo: 'hueco',
            texto: 'Quedan 5 horas sin programar entre 1:00 y 6:00 AM los siete días.',
          },
          {
            tipo: 'sin_relleno',
            texto:
              asistente.relleno === 0
                ? 'Esos huecos saldrían en negro: todavía no hay relleno.'
                : 'Esos huecos los cubre el relleno.',
          },
        ],
        titulos_sin_material: 1,
        aviso: 'esta es una propuesta, no una decisión: se mueve, se borra y se rehace.',
      })
    }
    case 9: {
      asistente.completa = true
      asistente.paso = 9
      apuntarPaso(9)
      return json({
        ...sigue,
        completa: true,
        modo_del_canal: 'sombra',
        aviso:
          'listo. El canal queda en modo sombra: resuelve el plan y publica la guía, pero todavía no emite — el motor es la fase siguiente.',
      })
    }
    default:
      return json({ error: 'Ese paso no existe: el asistente tiene nueve.' }, 400)
  }
}

// ── el ruteador ───────────────────────────────────────────────────────

function json(cuerpo: unknown, estadoHttp = 200): Response {
  return new Response(JSON.stringify(cuerpo), {
    status: estadoHttp,
    headers: { 'content-type': 'application/json' },
  })
}

export async function responder(ruta: string, init?: RequestInit): Promise<Response> {
  const url = new URL(ruta, 'http://demo.local')
  const p = url.pathname.replace(/^\/api\/v1/, '')
  const metodo = (init?.method ?? 'GET').toUpperCase()
  const cuerpo = init?.body ? JSON.parse(String(init.body)) : undefined
  await new Promise((r) => setTimeout(r, 40))

  if (p === '/entrar' && metodo === 'POST') {
    const clave = String(cuerpo?.clave ?? '')
    if (!/^\d{4,6}$/.test(clave))
      return json({ error: 'La clave de estación son de cuatro a seis dígitos.', campo: 'clave' }, 400)
    return json({ ok: true })
  }
  if (p === '/estado') return json(estado())
  if (p === '/canal') {
    if (metodo === 'PUT') Object.assign(canal, cuerpo)
    return json(canal)
  }
  if (p === '/ajustes') {
    if (metodo === 'PUT') ajustes = { ...ajustes, ...cuerpo }
    return json(ajustes)
  }
  if (p === '/instalacion') return json(instalacion())
  if (p === '/instalacion/relleno-por-defecto' && metodo === 'POST') {
    if (asistente.relleno > 0)
      return json(
        { error: 'Ya hay un relleno por defecto en la biblioteca. No hace falta otro.' },
        409,
      )
    asistente.relleno = 1
    guardarAsistente()
    return json(
      {
        archivo: 'relleno/cartel-de-la-estacion.mp4',
        aviso:
          'listo: el cartel de la estación con una cama musical ya está en la biblioteca de relleno. Se cambia cuando quieras; lo que no se puede es quedarse sin él.',
      },
      202,
    )
  }
  const pasoAsistente = p.match(/^\/instalacion\/paso\/(\d+)$/)
  if (pasoAsistente && metodo === 'POST')
    return pasoDelAsistente(Number(pasoAsistente[1]), cuerpo as Record<string, unknown> | undefined)
  if (p === '/reglas') {
    if (metodo === 'POST') {
      const nueva: Regla = { ...(cuerpo as Regla), id: siguienteId++, dias_restantes: 0 }
      reglas = [nueva, ...reglas]
      return json(nueva, 201)
    }
    return json(reglas)
  }
  const reglaId = p.match(/^\/reglas\/(\d+)$/)
  if (reglaId) {
    const id = Number(reglaId[1])
    if (metodo === 'DELETE') {
      reglas = reglas.filter((r) => r.id !== id)
      return json({ ok: true })
    }
    reglas = reglas.map((r) => (r.id === id ? { ...r, ...cuerpo } : r))
    return json(reglas.find((r) => r.id === id))
  }
  if (p === '/plan') return json(planDelDiaEmision(url.searchParams.get('dia') ?? diaEmisionDe(ahoraDemo())))
  if (p === '/plan/semana') return json(semana(url.searchParams.get('desde') ?? '2026-09-06'))
  if (p === '/plan/mes') return json(mes(url.searchParams.get('mes') ?? '2026-09'))
  if (p === '/plan/recalcular')
    return json([
      { tipo: 'hueco', texto: 'Quedan 5 horas sin programar entre 1:00 y 6:00 AM los siete días.' },
      { tipo: 'vencimiento', texto: 'Familia Robinson se vence el 6 de septiembre.' },
    ])
  if (p === '/plan/llenar-con-diferido') return json({ regla_creada: siguienteId++ })
  const planId = p.match(/^\/plan\/(\d+)$/)
  if (planId && metodo === 'PUT') {
    const id = Number(planId[1])
    const bloque = bloquePorId(id)
    if (!bloque) return json({ error: 'Ese bloque ya no está en la parrilla.' }, 404)
    if (cuerpo?.fijado === false) {
      aMano.delete(id)
      const suelto = bloquePorId(id)
      return json(suelto ? suelto.item : bloque.item)
    }
    if (cuerpo?.instante_planeado && Number.isNaN(Date.parse(String(cuerpo.instante_planeado))))
      return json(
        { error: 'Esa hora no se entiende. Escríbela con fecha y hora.', campo: 'instante_planeado' },
        400,
      )
    const previo = aMano.get(id)
    const instante = cuerpo?.instante_planeado
      ? new Date(Date.parse(String(cuerpo.instante_planeado))).toISOString()
      : previo?.instante_planeado
    const duracion =
      typeof cuerpo?.duracion_planeada_ms === 'number'
        ? cuerpo.duracion_planeada_ms
        : previo?.duracion_planeada_ms
    // Choque con otro bloque del mismo día: no se pisa nada al aire.
    if (instante) {
      const inicio = Date.parse(instante)
      const largo = duracion ?? bloque.item.duracion_planeada_ms
      const choque = bloquesDelDia(bloque.item.dia_emision).find((otro) => {
        if (otro.item.id === id) return false
        const a = Date.parse(otro.item.instante_planeado)
        return inicio < a + otro.item.duracion_planeada_ms && a < inicio + largo
      })
      if (choque)
        return json(
          {
            error: `A esa hora ya está ${choque.item.titulo}. Mueve uno de los dos o acorta el bloque.`,
            campo: 'instante_planeado',
          },
          409,
        )
    }
    aMano.set(id, {
      instante_planeado: instante,
      duracion_planeada_ms: duracion,
      fijado: true,
    })
    const nuevo = bloquePorId(id)
    return json(nuevo ? nuevo.item : bloque.item)
  }
  if (p === '/guia') return json(guia(url.searchParams.get('dia') ?? '2026-09-08'))
  if (p === '/biblioteca') return json(titulos.map(conPistaElegida))
  const tituloId = p.match(/^\/biblioteca\/(\d+)$/)
  if (tituloId) {
    const t = titulos.find((x) => x.id === Number(tituloId[1]))
    if (!t) return json({ error: 'Ese título no está en la biblioteca.' }, 404)
    return json({
      ...conPistaElegida(t),
      lista_de_episodios: episodiosDe(t).map(conPistaElegida),
    })
  }
  if (p === '/cuarentena') return json(cuarentena.filter((c) => !dejadosPasar.has(c.id)))
  const pasar = p.match(/^\/cuarentena\/(\d+)\/dejar-pasar$/)
  if (pasar) {
    const quien = String(cuerpo?.quien ?? '').trim()
    if (!quien)
      return json({ error: 'Escribe tu nombre: queda anotado quién lo dejó pasar.', campo: 'quien' }, 400)
    dejadosPasar.add(Number(pasar[1]))
    return json({ ok: true, dejado_pasar_por: quien })
  }
  if (p === '/material/subir') return json({ recibidos: 1, estado: 'ingiriendo' }, 202)
  const materialId = p.match(/^\/material\/(\d+)$/)
  if (materialId && metodo === 'PUT') {
    const id = Number(materialId[1])
    const m = materialPorId(id)
    if (!m) return json({ error: 'Ese archivo ya no está en la biblioteca.' }, 404)
    if (typeof cuerpo?.pista_audio_aire === 'number') {
      const pista = Number(cuerpo.pista_audio_aire)
      const pistas = m.pistas_audio ?? []
      if (!pistas.some((x) => x.indice === pista))
        return json(
          {
            error: 'Ese archivo no trae esa pista de sonido. Escoge una de las que tiene.',
            campo: 'pista_audio_aire',
          },
          400,
        )
      pistaElegida.set(id, pista)
    }
    return json(conPistaElegida(materialPorId(id) as ConAudio))
  }
  if (p === '/importar/hoja') return json(importarHoja(String(cuerpo?.texto ?? '')))
  if (p === '/importar/confirmar-relevos') return json({ ok: true })
  if (p === '/relleno')
    return json([{ id: 1, nombre: 'Cartel de la estación con cama musical', duracion_ms: 30_000 }])
  if (p === '/incidentes') return json([])
  return json({ error: `El modo demo no tiene esta ruta todavía: ${p}` }, 404)
}

/** El WebSocket de mentira: empuja el estado cada segundo. */
export function suscribirDemo(alRecibir: (e: Estado) => void): () => void {
  alRecibir(estado())
  const t = window.setInterval(() => alRecibir(estado()), 1000)
  return () => window.clearInterval(t)
}
