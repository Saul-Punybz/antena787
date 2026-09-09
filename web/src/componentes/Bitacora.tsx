import { useCallback, useEffect, useMemo, useState } from 'react'
import { api, suscribirseAEventos } from '../lib/api'
import { duracionLarga, fechaCorta, haceCuanto, hora, partes } from '../lib/fechas'
import type { Incidente } from '../lib/tipos'
import { Panel } from './Panel'

/** Cuántos incidentes caben en la tarjeta de Al aire antes de mandar al panel. */
const EN_LA_TARJETA = 4
/** Cada cuánto se vuelve a pedir la lista aunque no llegue ningún evento. */
const REFRESCO_MS = 60_000

const RANGOS = [
  { dias: 7, texto: 'últimos 7 días' },
  { dias: 30, texto: 'últimos 30 días' },
  { dias: 90, texto: 'últimos 90 días' },
]

/**
 * La bitácora: lo que el sistema hizo solo para proteger el aire, o lo que le
 * pasó y anotó (PRD §13 «pendientes de diseño», §15 `incidente`, §23). Es
 * evidencia, no regaño: cuándo, qué, cuánto duró. En Al aire va como tarjeta
 * con lo último; el resto se abre en un panel al lado, nunca encima
 * (docs/adr/0008).
 */
export function Bitacora({ zona, ahora }: { zona: string; ahora: number }) {
  const [lista, setLista] = useState<Incidente[] | null>(null)
  const [abierta, setAbierta] = useState(false)

  const cargar = useCallback(() => {
    api
      .incidentes()
      .then((l) => setLista(ordenar(l)))
      .catch(() => setLista([]))
  }, [])

  useEffect(() => {
    cargar()
    const t = window.setInterval(cargar, REFRESCO_MS)
    const soltar = suscribirseAEventos((e) => {
      if (e.clase === 'incidente') cargar()
    })
    return () => {
      window.clearInterval(t)
      soltar()
    }
  }, [cargar])

  const ultimos = (lista ?? []).slice(0, EN_LA_TARJETA)

  return (
    <>
      <div className="tarjeta" style={{ padding: '16px 18px' }}>
        <div className="entre">
          <div className="rotulo">BITÁCORA</div>
          <span className="tenue" style={{ fontSize: 12 }}>
            {lista === null ? '' : lista.length === 0 ? 'nada esta semana' : 'esta semana'}
          </span>
        </div>
        {lista !== null && lista.length === 0 && (
          <p className="subtitulo" style={{ marginTop: 10 }}>
            El sistema no ha tenido que hacer nada solo en los últimos 7 días.
          </p>
        )}
        {ultimos.length > 0 && (
          <ul style={{ listStyle: 'none', margin: '12px 0 0', padding: 0, display: 'grid', gap: 10 }}>
            {ultimos.map((i) => (
              <li key={i.id} style={{ fontSize: 13.5, lineHeight: 1.35 }}>
                <div className="entre" style={{ gap: 10, alignItems: 'baseline' }}>
                  <span style={{ fontWeight: 500 }}>{textoDe(i)}</span>
                  <span className="tenue mono" style={{ fontSize: 11.5, flexShrink: 0 }}>
                    {haceCuanto(i.inicio, ahora)}
                  </span>
                </div>
                {i.detalle && (
                  <div
                    className="tenue"
                    style={{
                      fontSize: 12.5,
                      marginTop: 2,
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                      whiteSpace: 'nowrap',
                    }}
                    title={i.detalle}
                  >
                    {i.detalle}
                  </div>
                )}
              </li>
            ))}
          </ul>
        )}
        <button
          className="boton"
          onClick={() => setAbierta(true)}
          style={{ marginTop: 12, width: '100%', fontSize: 13, padding: '8px 14px' }}
        >
          Ver la bitácora
        </button>
      </div>

      {abierta && <PanelBitacora zona={zona} ahora={ahora} alCerrar={() => setAbierta(false)} />}
    </>
  )
}

/** El panel al lado: la lista entera, por día, con un rango de fechas. */
function PanelBitacora({
  zona,
  ahora,
  alCerrar,
}: {
  zona: string
  ahora: number
  alCerrar: () => void
}) {
  const [dias, setDias] = useState(RANGOS[0].dias)
  const [lista, setLista] = useState<Incidente[] | null>(null)
  // `ahora` avanza cada segundo; el rango se fija al abrir el panel y solo se
  // recalcula al cambiar de días.
  const [abiertoEn] = useState(ahora)

  useEffect(() => {
    setLista(null)
    const desde = new Date(abiertoEn - dias * 86_400_000).toISOString()
    const hasta = new Date(abiertoEn + 60_000).toISOString()
    api
      .incidentes(desde, hasta)
      .then((l) => setLista(ordenar(l)))
      .catch(() => setLista([]))
  }, [dias, abiertoEn])

  const porDia = useMemo(() => {
    const grupos = new Map<string, Incidente[]>()
    for (const i of lista ?? []) {
      const d = partes(i.inicio, zona).dia
      const g = grupos.get(d)
      if (g) g.push(i)
      else grupos.set(d, [i])
    }
    return [...grupos.entries()]
  }, [lista, zona])

  return (
    <Panel
      titulo="Bitácora"
      descripcion="Lo que el sistema hizo solo para cuidar el aire, y lo que le pasó. Cuándo, qué y cuánto duró."
      alCerrar={alCerrar}
    >
      <div className="fila" style={{ gap: 8 }}>
        {RANGOS.map((r) => (
          <button
            key={r.dias}
            className="boton"
            onClick={() => setDias(r.dias)}
            style={{
              padding: '6px 12px',
              fontSize: 13,
              borderColor: r.dias === dias ? 'var(--aqua)' : undefined,
              color: r.dias === dias ? 'var(--aqua)' : undefined,
            }}
          >
            {r.texto}
          </button>
        ))}
      </div>

      {lista === null && <p className="cargando">Leyendo la bitácora…</p>}
      {lista !== null && lista.length === 0 && (
        <p className="subtitulo">
          Nada anotado en los {dias === 7 ? 'últimos 7 días' : `últimos ${dias} días`}. El
          sistema no tuvo que hacer nada solo.
        </p>
      )}
      {porDia.map(([dia, items]) => (
        <section key={dia}>
          <div className="rotulo" style={{ marginBottom: 8 }}>
            {fechaCorta(items[0].inicio, zona).toUpperCase()} · {items.length}
          </div>
          <ul style={{ listStyle: 'none', margin: 0, padding: 0, display: 'grid', gap: 8 }}>
            {items.map((i) => (
              <li
                key={i.id}
                style={{
                  background: 'var(--superficie-2)',
                  border: '1px solid var(--borde)',
                  borderRadius: 9,
                  padding: '11px 13px',
                }}
              >
                <div className="entre" style={{ gap: 10, alignItems: 'baseline' }}>
                  <span style={{ font: '600 13.5px var(--sans)' }}>{textoDe(i)}</span>
                  <span className="mono tenue" style={{ fontSize: 11.5, flexShrink: 0 }}>
                    {hora(i.inicio, zona)}
                    {duracionDe(i) && ` · ${duracionDe(i)}`}
                  </span>
                </div>
                {i.detalle && (
                  <div
                    className="tenue"
                    style={{ fontSize: 12.5, marginTop: 4, lineHeight: 1.4, whiteSpace: 'pre-wrap' }}
                  >
                    {i.detalle}
                  </div>
                )}
                <div className="mono tenue" style={{ fontSize: 10.5, marginTop: 5 }}>
                  {i.tipo}
                </div>
              </li>
            ))}
          </ul>
        </section>
      ))}
    </Panel>
  )
}

/** Lo más reciente primero: la bitácora se lee de arriba abajo. */
function ordenar(l: Incidente[]): Incidente[] {
  return [...l].sort((a, b) => Date.parse(b.inicio) - Date.parse(a.inicio) || b.id - a.id)
}

/** La frase del servidor o, si no la manda, el tipo legible. */
function textoDe(i: Incidente): string {
  if (i.texto) return i.texto
  const limpio = i.tipo.replace(/_/g, ' ')
  return limpio.charAt(0).toUpperCase() + limpio.slice(1)
}

/** «3 min» si el incidente ya cerró; nada si sigue abierto o fue instantáneo. */
function duracionDe(i: Incidente): string {
  if (!i.fin) return ''
  const ms = Date.parse(i.fin) - Date.parse(i.inicio)
  return ms >= 1000 ? duracionLarga(ms) : ''
}
