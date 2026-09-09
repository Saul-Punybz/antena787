// Fecha y hora siempre con Intl y siempre en la zona horaria del canal.
// Nada de "toLocaleString()" a secas: el canal puede estar en otra zona que
// la máquina que mira la interfaz.

const DIAS_CORTOS = ['Dom', 'Lun', 'Mar', 'Mié', 'Jue', 'Vie', 'Sáb']
const DIAS_LARGOS = [
  'domingo',
  'lunes',
  'martes',
  'miércoles',
  'jueves',
  'viernes',
  'sábado',
]
const MESES = [
  'enero',
  'febrero',
  'marzo',
  'abril',
  'mayo',
  'junio',
  'julio',
  'agosto',
  'septiembre',
  'octubre',
  'noviembre',
  'diciembre',
]
const MESES_CORTOS = [
  'ene',
  'feb',
  'mar',
  'abr',
  'may',
  'jun',
  'jul',
  'ago',
  'sep',
  'oct',
  'nov',
  'dic',
]

const cache = new Map<string, Intl.DateTimeFormat>()

function formateador(zona: string, opciones: Intl.DateTimeFormatOptions) {
  const clave = zona + JSON.stringify(opciones)
  let f = cache.get(clave)
  if (!f) {
    f = new Intl.DateTimeFormat('es-PR', { timeZone: zona, ...opciones })
    cache.set(clave, f)
  }
  return f
}

/** "1:45 PM" en la zona del canal. */
export function hora(instante: string | Date, zona: string): string {
  const d = typeof instante === 'string' ? new Date(instante) : instante
  return formateador(zona, { hour: 'numeric', minute: '2-digit', hour12: true })
    .format(d)
    .replace(/ /g, ' ')
    .replace('a. m.', 'AM')
    .replace('p. m.', 'PM')
    .replace('a.m.', 'AM')
    .replace('p.m.', 'PM')
}

/** "1:45:07 PM" */
export function horaConSegundos(instante: string | Date, zona: string): string {
  const d = typeof instante === 'string' ? new Date(instante) : instante
  return formateador(zona, {
    hour: 'numeric',
    minute: '2-digit',
    second: '2-digit',
    hour12: true,
  })
    .format(d)
    .replace(/ /g, ' ')
    .replace('a. m.', 'AM')
    .replace('p. m.', 'PM')
    .replace('a.m.', 'AM')
    .replace('p.m.', 'PM')
}

/** "4 sep 2026" */
export function fechaCorta(instante: string | Date, zona: string): string {
  const d = typeof instante === 'string' ? new Date(instante) : instante
  const partes = formateador(zona, {
    day: 'numeric',
    month: 'numeric',
    year: 'numeric',
  }).formatToParts(d)
  const dia = String(Number(partes.find((p) => p.type === 'day')?.value ?? '0'))
  const mes = Number(partes.find((p) => p.type === 'month')?.value ?? '1') - 1
  const anio = partes.find((p) => p.type === 'year')?.value ?? ''
  return `${dia} ${MESES_CORTOS[mes]} ${anio}`
}

/** Partes calendario de un instante, ya en la zona del canal. */
export function partes(instante: string | Date, zona: string) {
  const d = typeof instante === 'string' ? new Date(instante) : instante
  const p = formateador(zona, {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
    weekday: 'short',
  }).formatToParts(d)
  const v = (t: string) => p.find((x) => x.type === t)?.value ?? '0'
  const iso = `${v('year')}-${v('month')}-${v('day')}`
  const h = Number(v('hour')) % 24
  return {
    dia: iso,
    hora: h,
    minuto: Number(v('minute')),
    segundo: Number(v('second')),
    minutosDelDia: h * 60 + Number(v('minute')),
    diaSemana: diaSemanaDe(iso),
  }
}

/** 0 = domingo … 6 = sábado, leído de un "AAAA-MM-DD" sin pasar por zonas. */
export function diaSemanaDe(dia: string): number {
  const [a, m, d] = dia.split('-').map(Number)
  return new Date(Date.UTC(a, m - 1, d)).getUTCDay()
}

/** "Vie" */
export function diaCorto(dia: string): string {
  return DIAS_CORTOS[diaSemanaDe(dia)]
}

/** "viernes" */
export function diaLargo(dia: string): string {
  return DIAS_LARGOS[diaSemanaDe(dia)]
}

/** "6 de septiembre" */
export function diaYMes(dia: string): string {
  const [, m, d] = dia.split('-').map(Number)
  return `${d} de ${MESES[m - 1]}`
}

/** "4 sep" o "4 ene 2027" si no es el año en curso. */
export function fechaDeRegla(dia: string, anioActual: number): string {
  const [a, m, d] = dia.split('-').map(Number)
  return a === anioActual
    ? `${d} ${MESES_CORTOS[m - 1]}`
    : `${d} ${MESES_CORTOS[m - 1]} ${a}`
}

/** "Septiembre 2026" */
export function mesLargo(mes: string): string {
  const [a, m] = mes.split('-').map(Number)
  const nombre = MESES[m - 1]
  return nombre.charAt(0).toUpperCase() + nombre.slice(1) + ' ' + a
}

/** Suma días de calendario a un "AAAA-MM-DD". */
export function sumarDias(dia: string, n: number): string {
  const [a, m, d] = dia.split('-').map(Number)
  const t = new Date(Date.UTC(a, m - 1, d + n))
  return t.toISOString().slice(0, 10)
}

/** Días de calendario entre dos "AAAA-MM-DD" (b - a). */
export function diasEntre(a: string, b: string): number {
  const [a1, m1, d1] = a.split('-').map(Number)
  const [a2, m2, d2] = b.split('-').map(Number)
  return Math.round(
    (Date.UTC(a2, m2 - 1, d2) - Date.UTC(a1, m1 - 1, d1)) / 86_400_000,
  )
}

/** Minutos desde medianoche → "8:00 AM". */
export function minutosAHora12(min: number): string {
  const h = Math.floor(min / 60) % 24
  const m = min % 60
  const sufijo = h < 12 ? 'AM' : 'PM'
  const h12 = h % 12 === 0 ? 12 : h % 12
  return `${h12}:${String(m).padStart(2, '0')} ${sufijo}`
}

/** Minutos desde medianoche → "08:00" (para <input type="time">). */
export function minutosAHhMm(min: number): string {
  return `${String(Math.floor(min / 60) % 24).padStart(2, '0')}:${String(min % 60).padStart(2, '0')}`
}

/** "08:00" → 480. */
export function hhMmAMinutos(s: string): number {
  const [h, m] = s.split(':').map(Number)
  return (h || 0) * 60 + (m || 0)
}

/** "1 h 25 min", "25 min", "48 s". */
export function duracionLarga(ms: number): string {
  const min = Math.round(ms / 60_000)
  if (min < 1) return `${Math.round(ms / 1000)} s`
  if (min < 60) return `${min} min`
  const h = Math.floor(min / 60)
  const r = min % 60
  return r ? `${h} h ${r} min` : `${h} h`
}

/** "14:32" — cuenta regresiva en mono. */
export function cuentaRegresiva(ms: number): string {
  const s = Math.max(0, Math.round(ms / 1000))
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const seg = s % 60
  if (h > 0)
    return `${h}:${String(m).padStart(2, '0')}:${String(seg).padStart(2, '0')}`
  return `${m}:${String(seg).padStart(2, '0')}`
}

/** "hace 6 minutos", "hace 41 minutos", "hace 2 horas". */
export function haceCuanto(instante: string, ahora: number): string {
  const ms = ahora - new Date(instante).getTime()
  const min = Math.round(ms / 60_000)
  if (min < 1) return 'hace un momento'
  if (min === 1) return 'hace 1 minuto'
  if (min < 60) return `hace ${min} minutos`
  const h = Math.round(min / 60)
  if (h === 1) return 'hace 1 hora'
  if (h < 48) return `hace ${h} horas`
  return `hace ${Math.round(h / 24)} días`
}

/** "quedan 122 días", "vence hoy", "venció hace 3 días". */
export function textoDiasRestantes(dias: number): string {
  if (dias < 0) return `venció hace ${-dias} ${-dias === 1 ? 'día' : 'días'}`
  if (dias === 0) return 'vence hoy'
  if (dias === 1) return 'queda 1 día'
  return `quedan ${dias} días`
}

/** Horas con una cifra decimal cuando hace falta: "9 h", "7.5 h". */
export function horasBonitas(h: number): string {
  return Number.isInteger(h) ? `${h} h` : `${h.toFixed(1).replace('.0', '')} h`
}
