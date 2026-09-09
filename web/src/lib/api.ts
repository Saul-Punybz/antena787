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
  Ajustes,
  CambioDePlan,
  Canal,
  ElementoDelPlan,
  EnCuarentena,
  Estado,
  FichaDeTitulo,
  FilaDelPlan,
  Guia,
  Instalacion,
  MesDelPlan,
  Regla,
  ReglaNueva,
  ResumenDeImportacion,
  SemanaDelPlan,
  TituloDeBiblioteca,
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
    // Un error sin frase en cristiano no viene de Antena787 (docs/API.md):
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
  ajustes: () => pedir<Ajustes>('/ajustes'),
  guardarAjustes: (a: Ajustes) => pedir<Ajustes>('/ajustes', conCuerpo('PUT', a)),

  instalacion: () => pedir<Instalacion>('/instalacion'),
  responderPaso: (n: number, respuesta: unknown) =>
    pedir<Instalacion>(`/instalacion/paso/${n}`, conCuerpo('POST', respuesta)),

  reglas: () => pedir<Regla[]>('/reglas'),
  crearRegla: (r: ReglaNueva) => pedir<Regla>('/reglas', conCuerpo('POST', r)),
  editarRegla: (id: number, r: Partial<Regla>, soloHoy = false) =>
    pedir<Regla>(`/reglas/${id}${soloHoy ? '?solo_hoy=1' : ''}`, conCuerpo('PUT', r)),
  borrarRegla: (id: number) => pedir<{ ok: true }>(`/reglas/${id}`, conCuerpo('DELETE')),

  plan: (dia: string) => pedir<FilaDelPlan[]>(`/plan?dia=${dia}`),
  /**
   * Mueve un bloque del plan a mano, o lo suelta. El servidor devuelve el
   * elemento ya cambiado, con `fijado: true` cuando quedó clavado; si choca
   * con otro bloque contesta 409 con la frase en cristiano.
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

  biblioteca: () => pedir<TituloDeBiblioteca[]>('/biblioteca'),
  titulo: (id: number) => pedir<FichaDeTitulo>(`/biblioteca/${id}`),
  cuarentena: () => pedir<EnCuarentena[]>('/cuarentena'),
  dejarPasar: (id: number, quien: string) =>
    pedir<{ ok: true }>(`/cuarentena/${id}/dejar-pasar`, conCuerpo('POST', { quien })),

  importarHoja: (texto: string) =>
    pedir<ResumenDeImportacion>('/importar/hoja', conCuerpo('POST', { texto })),
  confirmarRelevos: (relevos: { regla: number; releva_a: number }[]) =>
    pedir<{ ok: true }>('/importar/confirmar-relevos', conCuerpo('POST', relevos)),

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
