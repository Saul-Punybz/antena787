import { useEffect, useState } from 'react'
import { PestanasDeParrilla } from './Parrilla'
import { api } from '../lib/api'
import { useEstado } from '../lib/estado'
import { diaYMes, duracionLarga, haceCuanto, hora, sumarDias } from '../lib/fechas'
import { IconoAlerta, IconoEquis, IconoOk } from '../componentes/Iconos'
import type { Guia } from '../lib/tipos'

/** Lo que dice la guía contra lo que va a salir, en el mismo eje. */
export function ParrillaGuia() {
  const { estado } = useEstado()
  const zona = estado?.canal.zona_horaria ?? 'UTC'
  const [guia, setGuia] = useState<Guia | null>(null)
  // Por defecto, el día que está saliendo al aire; las flechas mueven de día,
  // igual que en Semana y en Mes.
  const hoy = estado?.dia_emision ?? null
  const [elegido, setElegido] = useState<string | null>(null)
  const dia = elegido ?? hoy
  const [regenerando, setRegenerando] = useState(false)

  /**
   * La guía se reescribe con el plan, así que regenerarla es volver a armar el
   * plan y leerla otra vez. Antes este botón no hacía nada.
   */
  async function regenerar() {
    if (!dia) return
    setRegenerando(true)
    await api.recalcular().catch(() => {})
    await api
      .guia(dia)
      .then(setGuia)
      .catch(() => {})
    setRegenerando(false)
  }

  useEffect(() => {
    if (!dia) return
    api.guia(dia).then(setGuia).catch(() => setGuia(null))
  }, [dia])

  const malas = guia?.filas.filter((f) => !f.coincide).length ?? 0

  return (
    <>
      <div className="encabezado">
        <div>
          <h1 className="titulo-pantalla">Guía electrónica</h1>
          <p className="subtitulo">
            Lo que ve el televidente en su televisor, comparado con lo que de verdad va a
            salir{dia ? `, el ${diaYMes(dia)}` : ''}.
          </p>
        </div>
        <div className="fila" style={{ gap: 10 }}>
          <button
            className="boton"
            aria-label="El día antes"
            disabled={!dia}
            onClick={() => dia && setElegido(sumarDias(dia, -1))}
          >
            ←
          </button>
          <button
            className="boton"
            aria-label="El día después"
            disabled={!dia}
            onClick={() => dia && setElegido(sumarDias(dia, 1))}
          >
            →
          </button>
          <PestanasDeParrilla />
        </div>
      </div>

      {!guia && <p className="cargando">Comparando la guía con el plan…</p>}

      {guia && (
        <>
          <div
            className={'tarjeta' + (malas ? ' tarjeta--problema' : '')}
            style={{ padding: '20px 24px', display: 'flex', gap: 28, alignItems: 'center' }}
          >
            <div className="crece">
              <div
                style={{
                  font: '700 17px var(--sans)',
                  color: malas ? 'var(--rojo)' : 'var(--verde)',
                }}
              >
                {malas
                  ? `${malas} programa${malas === 1 ? '' : 's'} no coincide${malas === 1 ? '' : 'n'}`
                  : 'La guía dice lo mismo que el plan'}
              </div>
              <p className="subtitulo">
                {malas
                  ? 'La guía anuncia algo distinto de lo que va a salir al aire'
                  : 'Todo lo anunciado va a salir a su hora'}
              </p>
            </div>
            <ul style={{ listStyle: 'none', margin: 0, padding: 0, display: 'grid', gap: 9 }}>
              <li className="fila" style={{ gap: 10, fontSize: 13.5 }}>
                <IconoOk tamano={15} color="var(--verde)" />
                Identificador de canal correcto
              </li>
              <li className="fila" style={{ gap: 10, fontSize: 13.5 }}>
                <IconoOk tamano={15} color="var(--verde)" />
                {guia.filas.length} programas con hora y duración
              </li>
              <li className="fila" style={{ gap: 10, fontSize: 13.5 }}>
                {malas ? (
                  <IconoEquis tamano={15} color="var(--rojo)" />
                ) : (
                  <IconoOk tamano={15} color="var(--verde)" />
                )}
                {malas
                  ? `${malas} programa no coincide con el plan`
                  : 'Ningún programa desalineado'}
              </li>
            </ul>
            <button
              className="boton boton--primario"
              style={{ flexShrink: 0 }}
              disabled={regenerando}
              onClick={regenerar}
            >
              {regenerando ? 'Regenerando…' : 'Regenerar la guía'}
            </button>
          </div>

          <div style={{ overflowX: 'auto' }}>
            <div style={{ minWidth: 900 }}>
              <div
                style={{
                  display: 'flex',
                  gap: 8,
                  borderTop: '1px solid var(--borde)',
                  paddingTop: 7,
                  marginTop: 8,
                }}
              >
                {guia.filas.map((f, i) => (
                  <span
                    key={i}
                    className="mono tenue"
                    style={{
                      flex: `${Math.max(1, (f.plan?.duracion_ms ?? 0) / 60000)} 0 0`,
                      minWidth: 110,
                      fontSize: 11.5,
                    }}
                  >
                    {f.plan ? hora(f.plan.inicio, zona) : ''}
                  </span>
                ))}
              </div>
              <div className="rotulo" style={{ margin: '10px 0 8px' }}>
                LA GUÍA DICE
              </div>
              <Tira
                filas={guia.filas.map((f) => ({
                  titulo: f.guia?.titulo ?? '—',
                  ms: f.guia?.duracion_ms ?? 0,
                  coincide: f.coincide,
                }))}
              />
              <div className="rotulo aqua" style={{ margin: '18px 0 8px', color: 'var(--aqua)' }}>
                LO QUE VA A SALIR
              </div>
              <Tira
                filas={guia.filas.map((f) => ({
                  titulo: f.plan?.titulo ?? '—',
                  ms: f.plan?.duracion_ms ?? 0,
                  coincide: f.coincide,
                }))}
              />
            </div>
          </div>

          {guia.por_que_no_coinciden && (
            <div className="tarjeta" style={{ padding: '20px 24px' }}>
              <div className="rotulo">POR QUÉ NO COINCIDEN</div>
              <div className="fila" style={{ gap: 16, marginTop: 14, alignItems: 'flex-start' }}>
                <span
                  style={{
                    width: 40,
                    height: 40,
                    borderRadius: 9,
                    flexShrink: 0,
                    background: 'rgba(248,81,73,.12)',
                    display: 'inline-flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                  }}
                >
                  <IconoAlerta tamano={20} color="var(--rojo)" />
                </span>
                <div>
                  <p style={{ margin: 0, fontSize: 15, lineHeight: 1.55 }}>
                    {guia.por_que_no_coinciden}
                  </p>
                  <p className="subtitulo" style={{ marginTop: 8 }}>
                    Regenerar la guía lo arregla. Antena787 la revalida sola cada vez que
                    cambia el plan.
                  </p>
                </div>
              </div>
            </div>
          )}

          <div className="fila" style={{ gap: 9, fontSize: 13, color: 'var(--texto-2)' }}>
            <span className="punto punto--bien" />
            Publicada en{' '}
            <a className="mono" href="/guia.xml" target="_blank" rel="noreferrer">
              /guia.xml
            </a>{' '}
            · identificador de canal{' '}
            <span className="mono" style={{ color: 'var(--texto)' }}>
              {guia.identificador_de_canal}
            </span>{' '}
            · revalidada{' '}
            {haceCuanto(guia.revalidada, Date.parse(estado?.ahora ?? new Date().toISOString()))}
          </div>
        </>
      )}
    </>
  )
}

function Tira({
  filas,
}: {
  filas: { titulo: string; ms: number; coincide: boolean }[]
}) {
  return (
    <div style={{ display: 'flex', gap: 8 }}>
      {filas.map((f, i) => (
        <div
          key={i}
          className="tarjeta"
          style={{
            flex: `${Math.max(1, f.ms / 60000)} 0 0`,
            minWidth: 110,
            padding: '15px 16px',
            borderColor: f.coincide ? 'var(--borde)' : 'var(--rojo)',
            background: f.coincide ? 'var(--superficie)' : 'rgba(248,81,73,.07)',
          }}
        >
          <div
            style={{
              font: '600 14px var(--sans)',
              color: f.coincide ? 'var(--texto)' : 'var(--rojo)',
              whiteSpace: 'nowrap',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
            }}
          >
            {f.titulo}
          </div>
          <div className="mono tenue" style={{ fontSize: 12, marginTop: 4 }}>
            {duracionLarga(f.ms)}
          </div>
        </div>
      ))}
    </div>
  )
}
