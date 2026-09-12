// Cliente de la API de Antena787 (docs/API.md).
//
// Prefijo /api/v1. La sesión va en una cookie HttpOnly que pone POST /entrar,
// así que aquí solo hace falta `credentials: 'include'`.
//
// Modo demo: si VITE_DEMO=1, o si el servidor de verdad no contesta el primer
// /estado, todas las llamadas se atienden con los datos de src/demo/. La
// interfaz se puede mirar completa sin tener el servidor Go levantado.

import { responder, suscribirDemo } from '../demo/servidor'
import type {
  Fuente,
  FuenteNueva,
  FuentesDelCanal,
  PruebaDeFuente,
  AjustesDePreset,
  Preset,
  PresetsDelCanal,
  Ajustes,
  CambioDeMaterial,
  CambioDePlan,
  CambioDeTitulo,
  Canal,
  ComprobacionesDelAire,
  CuerposDePaso,
  DecisionDeEmparejar,
  ElementoDelPlan,
  EnCuarentena,
  Estado,
  FichaDeTitulo,
  FilaDelPlan,
  Guia,
  Incidente,
  Instalacion,
  MaterialDeAudio,
  MesDelPlan,
  Regla,
  ReglaNueva,
  RellenoPorDefecto,
  RespuestasDePaso,
  ResultadoDeEmparejar,
  ResumenDeImportacion,
  Salida,
  SalidaBorrada,
  SalidaNueva,
  SalidasDelCanal,
  SemanaDelPlan,
  TituloDeBiblioteca,
  TituloDelCatalogo,
  TituloSinEmparejar,
  ArchivoEntrando,
} from './tipos'
import { ErrorDeApi } from './tipos'

const PREFIJO = '/api/v1'

const forzarDemo = import.meta.env.VITE_DEMO === '1'
let enDemo = forzarDemo
const oyentesDeModo = new Set<(demo: boolean) => void>()

export function estaEnDemo(): boolean {
  return enDemo
}

export function alCambiarDeModo(f: (demo: boolean) => void): () => void {
  oyentesDeModo.add(f)
  return () => oyentesDeModo.delete(f)
}

function entrarEnDemo() {
  if (enDemo) return
  enDemo = true
  for (const f of oyentesDeModo) f(true)
}

async function pedir<T>(ruta: string, init?: RequestInit): Promise<T> {
  const respuesta = enDemo
    ? await responder(ruta, init)
    : await fetch(PREFIJO + ruta, {
        credentials: 'include',
        headers: init?.body
          ? { 'content-type': 'application/json', ...(init?.headers ?? {}) }
          : init?.headers,
        ...init,
      }).catch(() => {
        // El servidor no contesta: la interfaz sigue viva con datos de ejemplo.
        entrarEnDemo()
        return responder(ruta, init)
      })

  const texto = await respuesta.text()
  let cuerpo: unknown = null
  try {
    cuerpo = texto ? (JSON.parse(texto) as unknown) : null
  } catch {
    // Contestó algo que no es JSON: no hay servidor de Antena787 al otro lado
    // (una página de error, un servidor estático). La interfaz sigue viva.
    if (!enDemo) {
      entrarEnDemo()
      return pedir<T>(ruta, init)
    }
    throw new ErrorDeApi('El servidor contestó algo que no se entiende.', respuesta.status)
  }
  if (!respuesta.ok) {
    const e = cuerpo as { error?: string; campo?: string } | null
    // Un error sin frase clara no viene de Antena787 (docs/API.md):
    // al otro lado hay otra cosa. La interfaz sigue con datos de ejemplo.
    if (!enDemo && respuesta.status !== 401 && typeof e?.error !== 'string') {
      entrarEnDemo()
      return pedir<T>(ruta, init)
    }
    throw new ErrorDeApi(
      e?.error ?? 'Algo salió mal y el servidor no dijo qué.',
      respuesta.status,
      e?.campo,
    )
  }
  return cuerpo as T
}

function conCuerpo(metodo: string, cuerpo?: unknown): RequestInit {
  return {
    method: metodo,
    body: cuerpo === undefined ? undefined : JSON.stringify(cuerpo),
    headers: { 'content-type': 'application/json' },
  }
}

// ── entrar ────────────────────────────────────────────────────────────

export const api = {
  entrar: (clave: string) => pedir<{ ok: true }>('/entrar', conCuerpo('POST', { clave })),

  estado: () => pedir<Estado>('/estado'),
  canal: () => pedir<Canal>('/canal'),
  guardarCanal: (c: Partial<Canal>) => pedir<Canal>('/canal', conCuerpo('PUT', c)),

  // La puerta del aire (internal/api/aire.go). PUT /canal no toca el modo a
  // propósito: la transición pasa por aquí, con confirmación escrita.
  comprobacionesDelAire: () => pedir<ComprobacionesDelAire>('/canal/comprobaciones'),
  salirAlAire: (confirmacion: string) =>
    pedir<ComprobacionesDelAire>('/canal/al-aire', conCuerpo('POST', { confirmacion })),
  volverASombra: (confirmacion: string) =>
    pedir<ComprobacionesDelAire>('/canal/a-sombra', conCuerpo('POST', { confirmacion })),

  // Las salidas: a dónde va la señal. La API existía entera desde T2 y no
  // tenía cliente ni pantalla, así que el canal no se podía apuntar a nada.
  salidas: () => pedir<SalidasDelCanal>('/salidas'),
  crearSalida: (nueva: SalidaNueva) => pedir<Salida>('/salidas', conCuerpo('POST', nueva)),
  guardarSalida: (id: number, cambio: Partial<SalidaNueva>) =>
    pedir<Salida>(`/salidas/${id}`, conCuerpo('PUT', cambio)),
  borrarSalida: (id: number) => pedir<SalidaBorrada>(`/salidas/${id}`, conCuerpo('DELETE')),
  ajustes: () => pedir<Ajustes>('/ajustes'),
  // Los presets de preparación (esquema v10). `ajustes` viaja como objeto:
  // la pantalla no serializa nada a mano.
  presets: () => pedir<PresetsDelCanal>('/presets'),
  crearPreset: (nombre: string, ajustes: AjustesDePreset) =>
    pedir<Preset>('/presets', conCuerpo('POST', { nombre, ajustes })),
  guardarPreset: (id: number, nombre: string, ajustes: AjustesDePreset) =>
    pedir<Preset>(`/presets/${id}`, conCuerpo('PUT', { nombre, ajustes })),
  borrarPreset: (id: number) =>
    pedir<{ borrado: number; aviso: string }>(`/presets/${id}`, conCuerpo('DELETE', {})),
  // Cambiar un preset no rehace solo lo ya convertido: hay que pedirlo.
  volverAPreparar: (id: number) =>
    pedir<{ encolado: number; texto: string }>(
      `/material/${id}/volver-a-preparar`,
      conCuerpo('POST', {}),
    ),

  // Las señales en vivo (F2-116). `clave` solo se manda cuando cambia:
  // reenviar una clave en cada guardado es como se filtran.
  fuentes: () => pedir<FuentesDelCanal>('/fuentes'),
  crearFuente: (f: FuenteNueva) => pedir<Fuente>('/fuentes', conCuerpo('POST', f)),
  guardarFuente: (id: number, f: FuenteNueva) =>
    pedir<Fuente>(`/fuentes/${id}`, conCuerpo('PUT', f)),
  borrarFuente: (id: number) =>
    pedir<{ borrada: number }>(`/fuentes/${id}`, conCuerpo('DELETE', {})),
  // Probar sin guardar: abre la señal de verdad y dice qué hay al otro lado.
  probarFuente: (f: Partial<FuenteNueva>) =>
    pedir<PruebaDeFuente>('/fuentes/probar', conCuerpo('POST', f)),

  guardarAjustes: (a: Ajustes) => pedir<Ajustes>('/ajustes', conCuerpo('PUT', a)),

  instalacion: () => pedir<Instalacion>('/instalacion'),
  /**
   * Contesta un paso del asistente. Cada paso tiene su cuerpo y su respuesta;
   * todas traen {paso, siguiente} y lo suyo. Volver atrás y contestar otra vez
   * es válido: el servidor pisa lo que había.
   */
  responderPaso: <N extends keyof RespuestasDePaso>(n: N, respuesta: CuerposDePaso[N]) =>
    pedir<RespuestasDePaso[N]>(`/instalacion/paso/${n}`, conCuerpo('POST', respuesta)),
  /**
   * Crea el relleno por defecto —el cartel de la estación con una cama
   * musical— de un clic (PRD §13). 202 cuando lo creó; 409 si ya existía.
   */
  rellenoPorDefecto: () =>
    pedir<RellenoPorDefecto>('/instalacion/relleno-por-defecto', conCuerpo('POST')),

  reglas: () => pedir<Regla[]>('/reglas'),
  crearRegla: (r: ReglaNueva) => pedir<Regla>('/reglas', conCuerpo('POST', r)),
  editarRegla: (id: number, r: Partial<Regla>, soloHoy = false) =>
    pedir<Regla>(`/reglas/${id}${soloHoy ? '?solo_hoy=1' : ''}`, conCuerpo('PUT', r)),
  borrarRegla: (id: number) => pedir<{ ok: true }>(`/reglas/${id}`, conCuerpo('DELETE')),

  /**
   * Las filas del plan de un día, huecos incluidos. El servidor las manda
   * dentro de un sobre con el día y sus bordes (`{dia_emision, inicio, fin,
   * items}`); aquí se devuelven las filas, que es lo único que las pantallas
   * usan. Antes esto decía que el servidor devolvía una lista pelada y la
   * vista por día se quedaba en negro contra el servidor de verdad, porque
   * solo el demo la mandaba así (11 sept 2026).
   */
  plan: (dia: string) =>
    pedir<{ items?: FilaDelPlan[] } | FilaDelPlan[]>(`/plan?dia=${dia}`).then((r) =>
      Array.isArray(r) ? r : (r.items ?? []),
    ),
  /**
   * Mueve un bloque del plan a mano, o lo suelta. El servidor devuelve el
   * elemento ya cambiado, con `fijado: true` cuando quedó clavado; si choca
   * con otro bloque contesta 409 con la frase clara.
   */
  cambiarPlan: (id: number, cambio: CambioDePlan) =>
    pedir<ElementoDelPlan>(`/plan/${id}`, conCuerpo('PUT', cambio)),
  planSemana: (desde: string) => pedir<SemanaDelPlan>(`/plan/semana?desde=${desde}`),
  planMes: (mes: string) => pedir<MesDelPlan>(`/plan/mes?mes=${mes}`),
  recalcular: () =>
    pedir<{ tipo: string; texto: string }[]>('/plan/recalcular', conCuerpo('POST')),
  llenarConDiferido: (v: {
    desde: string
    hasta: string
    origen_desde: string
    origen_hasta: string
  }) => pedir<{ regla_creada: number }>('/plan/llenar-con-diferido', conCuerpo('POST', v)),

  guia: (dia: string) => pedir<Guia>(`/guia?dia=${dia}`),

  /**
   * Cambia algo del archivo en sí. Hoy: cuál de sus pistas de sonido sale al
   * aire. El archivo vuelve a la cola y queda «aún no listo para aire» hasta
   * que se rehace con esa pista (F1-61).
   */
  cambiarMaterial: (id: number, cambio: CambioDeMaterial) =>
    pedir<MaterialDeAudio>(`/material/${id}`, conCuerpo('PUT', cambio)),

  biblioteca: () => pedir<TituloDeBiblioteca[]>('/biblioteca'),
  titulo: (id: number) => pedir<FichaDeTitulo>(`/biblioteca/${id}`),
  /**
   * Cambia algo de la ficha del título. Hoy: si es un programa infantil
   * educativo (F1-76). Lo que no se manda se queda como estaba.
   */
  cambiarTitulo: (id: number, cambio: CambioDeTitulo) =>
    pedir<FichaDeTitulo>(`/biblioteca/${id}`, conCuerpo('PUT', cambio)),
  cuarentena: () => pedir<EnCuarentena[]>('/cuarentena'),
  /** Los archivos que se están midiendo ahora mismo: se ven desde el primer segundo. */
  entrando: () => pedir<ArchivoEntrando[]>('/material?estado=ingiriendo'),
  dejarPasar: (id: number, quien: string) =>
    pedir<{ ok: true }>(`/cuarentena/${id}/dejar-pasar`, conCuerpo('POST', { quien })),

  /**
   * La bitácora de lo que el sistema hizo solo (PRD §15). Sin fechas, el
   * servidor manda la última semana; `desde`/`hasta` van en RFC 3339.
   */
  incidentes: (desde?: string, hasta?: string) => {
    const q = new URLSearchParams()
    if (desde) q.set('desde', desde)
    if (hasta) q.set('hasta', hasta)
    const sufijo = q.toString()
    return pedir<Incidente[]>('/incidentes' + (sufijo ? '?' + sufijo : ''))
  },

  importarHoja: (texto: string) =>
    pedir<ResumenDeImportacion>('/importar/hoja', conCuerpo('POST', { texto })),
  confirmarRelevos: (relevos: { regla: number; releva_a: number }[]) =>
    pedir<{ ok: true }>('/importar/confirmar-relevos', conCuerpo('POST', relevos)),

  /**
   * Los títulos que la hoja trajo con un nombre que el catálogo no tiene
   * (F1-64). Queda guardado entre importaciones: la lista sigue ahí hasta que
   * alguien decida. Vacía cuando no hay ninguno.
   */
  titulosSinEmparejar: () => pedir<TituloSinEmparejar[]>('/titulos/sin-emparejar'),
  /** Busca fichas del catálogo por nombre, para escoger a mano con cuál es. */
  buscarTitulos: (q: string) =>
    pedir<TituloDelCatalogo[]>(`/titulos/buscar?q=${encodeURIComponent(q)}`),
  /**
   * Decide un título por emparejar: `usar` lo manda a una ficha del catálogo
   * (y guarda el nombre de la hoja como alias, para que la próxima se empareje
   * sola), `propio` lo deja como ficha suya, `quitar` se lleva el título y las
   * reglas que lo usaban (F1-66, F1-67).
   */
  emparejarTitulo: (id: number, decision: DecisionDeEmparejar) =>
    pedir<ResultadoDeEmparejar>(`/titulos/${id}/emparejar`, conCuerpo('POST', decision)),

  /** Sube material por multipart a la carpeta vigilada. */
  async subirMaterial(archivos: File[]): Promise<{ recibidos: number }> {
    if (enDemo) {
      await new Promise((r) => setTimeout(r, 400))
      return { recibidos: archivos.length }
    }
    const forma = new FormData()
    for (const a of archivos) forma.append('archivo', a, a.name)
    const r = await fetch(PREFIJO + '/material/subir', {
      method: 'POST',
      credentials: 'include',
      body: forma,
    }).catch(() => {
      entrarEnDemo()
      return null
    })
    if (!r) return { recibidos: archivos.length }
    if (!r.ok) {
      const e = (await r.json().catch(() => null)) as { error?: string } | null
      throw new ErrorDeApi(e?.error ?? 'No se pudo subir el material.', r.status)
    }
    return (await r.json()) as { recibidos: number }
  },
}

// ── el estado en vivo ─────────────────────────────────────────────────

/** Un `{"tipo":"evento"}` del WebSocket: algo pasó (un incidente, un cambio de plan). */
export interface EventoDelServidor {
  tipo: 'evento'
  /** De qué familia es: "incidente", "plan", "ingest", "material"… */
  clase: string
  /** El tipo de incidente, el nombre de la goroutine… */
  nombre: string
  detalle: string
  instante: string
}

const oyentesDeEventos = new Set<(e: EventoDelServidor) => void>()

/**
 * Avisa cada vez que el servidor empuja un evento. Es lo que hace que la
 * bitácora de Al aire se actualice sola cuando el sistema anota algo. En
 * modo demo no llega ninguno: la pantalla se refresca por su cuenta.
 */
export function suscribirseAEventos(f: (e: EventoDelServidor) => void): () => void {
  oyentesDeEventos.add(f)
  return () => oyentesDeEventos.delete(f)
}

/**
 * WS /ws empuja {"tipo":"estado", ...} cada segundo. Se reconecta solo, con
 * espera creciente, y cae al modo demo si nunca llega a conectarse.
 */
export function suscribirseAlEstado(alRecibir: (e: Estado) => void): () => void {
  if (enDemo) return suscribirDemo(alRecibir)

  let socket: WebSocket | null = null
  let temporizador = 0
  let espera = 500
  let vivo = true
  let intentos = 0

  const conectar = () => {
    if (!vivo) return
    const esquema = location.protocol === 'https:' ? 'wss' : 'ws'
    socket = new WebSocket(`${esquema}://${location.host}${PREFIJO}/ws`)
    socket.onopen = () => {
      espera = 500
      intentos = 0
    }
    socket.onmessage = (ev) => {
      try {
        const dato = JSON.parse(ev.data as string) as { tipo?: string } & Estado
        if (dato.tipo === 'evento') {
          for (const f of oyentesDeEventos) f(dato as unknown as EventoDelServidor)
          return
        }
        if (dato.tipo === 'estado' || dato.canal) alRecibir(dato)
      } catch {
        // Un mensaje roto no tumba la vista de aire.
      }
    }
    socket.onclose = () => {
      if (!vivo) return
      intentos++
      if (intentos >= 3 && !enDemo) {
        // El servidor no está: la interfaz sigue con datos de ejemplo.
        entrarEnDemo()
        cerrar = suscribirDemo(alRecibir)
        return
      }
      temporizador = window.setTimeout(conectar, espera)
      espera = Math.min(espera * 2, 10_000)
    }
    socket.onerror = () => socket?.close()
  }

  let cerrar: (() => void) | null = null
  conectar()

  return () => {
    vivo = false
    window.clearTimeout(temporizador)
    socket?.close()
    cerrar?.()
  }
}
