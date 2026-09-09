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
  ElementoDelPlan,
  EpisodioDeBiblioteca,
  Estado,
  FilaDelPlan,
  FranjaSemana,
  Guia,
  MesDelPlan,
  Regla,
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
    instalacion_completa: true,
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
  if (p === '/instalacion')
    return json({
      paso: 1,
      detectado: {
        ffmpeg: 'encontrado',
        aceleracion: 'NVIDIA',
        disco: '890 GB libres',
        red: 'una tarjeta de red conectada',
      },
    })
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
