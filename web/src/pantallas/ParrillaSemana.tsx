import { useEffect, useMemo, useState } from 'react'
import { PestanasDeParrilla } from './Parrilla'
import { Panel } from '../componentes/Panel'
import { api } from '../lib/api'
import { useEstado } from '../lib/estado'
import {
  diaCorto,
  diaYMes,
  horasBonitas,
  minutosAHora12,
  partes,
  sumarDias,
} from '../lib/fechas'
import type { SemanaDelPlan } from '../lib/tipos'

const DESDE_MIN = 6 * 60 // la tira va de las 6:00 AM
const HASTA_MIN = 24 * 60 // a medianoche
const ANCHO = HASTA_MIN - DESDE_MIN

interface Bloque {
  titulo: string | null
  desde: number
  hasta: number
  enVivo: boolean
}

function bloquesDelDia(franjas: { titulo: string | null; en_vivo: boolean }[]): Bloque[] {
  const out: Bloque[] = []
  for (let k = DESDE_MIN / 30; k < HASTA_MIN / 30; k++) {
    const f = franjas[k]
    const titulo = f?.titulo ?? null
    const ultimo = out[out.length - 1]
    if (ultimo && ultimo.titulo === titulo) {
      ultimo.hasta = (k + 1) * 30
    } else {
      out.push({
        titulo,
        desde: k * 30,
        hasta: (k + 1) * 30,
        enVivo: Boolean(f?.en_vivo),
      })
    }
  }
  return out
}

function colorDe(titulo: string): string {
  const semilla = [...titulo].reduce((a, c) => (a * 31 + c.charCodeAt(0)) % 360, 11)
  return `hsl(${semilla} 19% 17%)`
}

export function ParrillaSemana() {
  const { estado } = useEstado()
  const zona = estado?.canal.zona_horaria ?? 'UTC'
  const hoy = estado?.dia_emision ?? '2026-09-04'
  const [desde, setDesde] = useState<string | null>(null)
  const [semana, setSemana] = useState<SemanaDelPlan | null>(null)
  const [arrastre, setArrastre] = useState<{
    titulo: string
    dia: string
    de: number
    a: number
  } | null>(null)

  // La semana que empieza el domingo de la semana en curso.
  const inicio = useMemo(() => {
    if (desde) return desde
    const d = new Date(hoy + 'T00:00:00Z')
    return sumarDias(hoy, -d.getUTCDay() + 7) // la semana que viene, la del 6 de septiembre
  }, [desde, hoy])

  useEffect(() => {
    api.planSemana(inicio).then(setSemana).catch(() => setSemana(null))
  }, [inicio])

  const ahora = estado ? partes(estado.ahora, zona) : null
  const marcaAhora =
    ahora && ahora.minutosDelDia >= DESDE_MIN
      ? ((ahora.minutosDelDia - DESDE_MIN) / ANCHO) * 100
      : null

  const finDeSemana =
    semana?.dias.filter((d) => [0, 6].includes(new Date(d.dia + 'T00:00:00Z').getUTCDay())) ?? []
  const entreSemana =
    semana?.dias.filter((d) => ![0, 6].includes(new Date(d.dia + 'T00:00:00Z').getUTCDay())) ?? []
  const promFin = finDeSemana.length
    ? finDeSemana.reduce((a, d) => a + d.horas_vacias, 0) / finDeSemana.length
    : 0
  const promSemana = entreSemana.length
    ? entreSemana.reduce((a, d) => a + d.horas_vacias, 0) / entreSemana.length
    : 0

  return (
    <>
      <div className="encabezado">
        <div>
          <h1 className="titulo-pantalla">
            {semana
              ? `Semana del ${diaYMes(semana.dias[0].dia).replace(/ de \w+$/, '')} al ${diaYMes(semana.dias[6].dia)}`
              : 'Semana'}
          </h1>
          <p className="subtitulo">
            De 6:00 AM a medianoche. El día de emisión empieza a las{' '}
            {minutosAHora12(estado?.canal.hora_inicio_dia_emision ?? 360)}.
          </p>
        </div>
        <div className="fila" style={{ gap: 10 }}>
          <button className="boton" onClick={() => setDesde(sumarDias(inicio, -7))}>
            ←
          </button>
          <button className="boton" onClick={() => setDesde(sumarDias(inicio, 7))}>
            →
          </button>
          <PestanasDeParrilla />
        </div>
      </div>

      {!semana && <p className="cargando">Armando la semana…</p>}

      {semana && (
        <div style={{ position: 'relative' }}>
          {/* La regla de horas */}
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: '44px minmax(0,1fr) 92px',
              alignItems: 'center',
              marginBottom: 22,
            }}
          >
            <span />
            <div style={{ position: 'relative', height: 18 }}>
              {Array.from({ length: 9 }, (_, i) => DESDE_MIN + i * 120).map((m) => (
                <div
                  key={m}
                  style={{
                    position: 'absolute',
                    left: `${((m - DESDE_MIN) / ANCHO) * 100}%`,
                    top: 0,
                  }}
                >
                  <div style={{ width: 1, height: 7, background: 'var(--borde)' }} />
                  <span
                    className="mono"
                    style={{
                      position: 'absolute',
                      top: 10,
                      left: -2,
                      fontSize: 10.5,
                      color: 'var(--texto-3)',
                      whiteSpace: 'nowrap',
                    }}
                  >
                    {minutosAHora12(m).replace(':00', '')}
                  </span>
                </div>
              ))}
            </div>
            <span />
          </div>

          {/* Los siete días */}
          <div style={{ position: 'relative', display: 'grid', gap: 3 }}>
            {marcaAhora !== null && (
              <div
                style={{
                  position: 'absolute',
                  left: `calc(44px + (100% - 136px) * ${marcaAhora / 100})`,
                  top: -26,
                  bottom: 0,
                  width: 2,
                  background: 'var(--aqua)',
                  zIndex: 3,
                  pointerEvents: 'none',
                }}
              >
                <span
                  className="mono"
                  style={{
                    position: 'absolute',
                    top: -20,
                    left: '50%',
                    transform: 'translateX(-50%)',
                    background: 'var(--aqua)',
                    color: '#06141a',
                    borderRadius: 5,
                    padding: '2px 7px',
                    fontSize: 11,
                    fontWeight: 600,
                    whiteSpace: 'nowrap',
                  }}
                >
                  {minutosAHora12(ahora!.minutosDelDia)}
                </span>
              </div>
            )}

            {semana.dias.map((d) => {
              const esHoy = d.dia === hoy
              return (
                <div
                  key={d.dia}
                  style={{
                    display: 'grid',
                    gridTemplateColumns: '44px minmax(0,1fr) 92px',
                    alignItems: 'center',
                    gap: 0,
                    padding: '5px 0',
                    borderRadius: 7,
                    background: esHoy ? 'rgba(34,211,238,.05)' : undefined,
                  }}
                >
                  <span
                    style={{
                      font: `${esHoy ? 600 : 400} 13px var(--sans)`,
                      color: esHoy ? 'var(--texto)' : 'var(--texto-2)',
                    }}
                  >
                    {diaCorto(d.dia)}
                  </span>
                  <div
                    style={{ position: 'relative', height: 40 }}
                    onDragOver={(e) => e.preventDefault()}
                    onDrop={(e) => {
                      e.preventDefault()
                      const dato = e.dataTransfer.getData('text/antena-bloque')
                      if (!dato) return
                      const caja = e.currentTarget.getBoundingClientRect()
                      const x = (e.clientX - caja.left) / caja.width
                      const min =
                        Math.round((DESDE_MIN + x * ANCHO) / 30) * 30
                      const b = JSON.parse(dato) as { titulo: string; desde: number }
                      if (min === b.desde && d.dia === arrastre?.dia) return
                      setArrastre({ titulo: b.titulo, dia: d.dia, de: b.desde, a: min })
                    }}
                  >
                    {bloquesDelDia(d.franjas).map((b) => {
                      const izq = ((b.desde - DESDE_MIN) / ANCHO) * 100
                      const ancho = ((b.hasta - b.desde) / ANCHO) * 100
                      if (!b.titulo)
                        return (
                          <div
                            key={b.desde}
                            title={`Vacío: ${minutosAHora12(b.desde)} – ${minutosAHora12(b.hasta)}`}
                            style={{
                              position: 'absolute',
                              left: `${izq}%`,
                              width: `calc(${ancho}% - 1px)`,
                              top: 0,
                              bottom: 0,
                              borderRadius: 2,
                              background: 'var(--rayado-rojo)',
                            }}
                          />
                        )
                      return (
                        <div
                          key={b.desde}
                          draggable
                          onDragStart={(e) => {
                            e.dataTransfer.setData(
                              'text/antena-bloque',
                              JSON.stringify({ titulo: b.titulo, desde: b.desde }),
                            )
                          }}
                          title={`${b.titulo} · ${minutosAHora12(b.desde)}`}
                          style={{
                            position: 'absolute',
                            left: `${izq}%`,
                            width: `calc(${ancho}% - 1px)`,
                            top: 0,
                            bottom: 0,
                            borderRadius: 2,
                            overflow: 'hidden',
                            display: 'flex',
                            alignItems: 'center',
                            cursor: 'grab',
                            background: b.enVivo ? 'rgba(34,211,238,.08)' : colorDe(b.titulo),
                            border: b.enVivo ? '1.5px solid var(--aqua)' : undefined,
                          }}
                        >
                          {ancho > 6 && (
                            <span
                              style={{
                                font: '500 10px var(--sans)',
                                color: 'rgba(230,237,243,.85)',
                                paddingLeft: 5,
                                whiteSpace: 'nowrap',
                                overflow: 'hidden',
                              }}
                            >
                              {b.titulo}
                            </span>
                          )}
                        </div>
                      )
                    })}
                  </div>
                  <span
                    style={{
                      textAlign: 'right',
                      font: '500 11.5px var(--sans)',
                      color: d.horas_vacias >= 8 ? 'var(--rojo)' : 'var(--ambar)',
                    }}
                  >
                    {horasBonitas(d.horas_vacias)} vacías
                  </span>
                </div>
              )
            })}
          </div>

          {/* Leyenda */}
          <div className="entre" style={{ marginTop: 16 }}>
            <div className="fila" style={{ gap: 22, fontSize: 12.5, color: 'var(--texto-2)' }}>
              <span className="fila" style={{ gap: 8 }}>
                <i style={{ width: 26, height: 11, borderRadius: 3, background: '#39415a' }} />
                programado
              </span>
              <span className="fila" style={{ gap: 8 }}>
                <i style={{ width: 26, height: 11, borderRadius: 3, background: 'var(--rayado-rojo)' }} />
                vacío
              </span>
              <span className="fila" style={{ gap: 8 }}>
                <i
                  style={{
                    width: 26,
                    height: 11,
                    borderRadius: 3,
                    border: '1.5px solid var(--aqua)',
                  }}
                />
                en vivo
              </span>
            </div>
            {semana.nota && (
              <span className="rojo" style={{ font: '600 12.5px var(--sans)' }}>
                {semana.nota}
              </span>
            )}
          </div>

          {/* El aviso donde se toma la acción, no en un reporte aparte */}
          {promFin > promSemana + 1 && (
            <div
              style={{
                marginTop: 26,
                border: '1px dashed rgba(248,81,73,.45)',
                background:
                  'linear-gradient(rgba(14,17,22,.72), rgba(14,17,22,.72)), var(--rayado-rojo)',
                borderRadius: 10,
                padding: '22px 24px',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                gap: 20,
              }}
            >
              <div>
                <div style={{ font: '700 17px var(--sans)', color: 'var(--rojo)' }}>
                  El fin de semana tiene más aire vacío
                </div>
                <p className="subtitulo" style={{ color: 'var(--texto-2)' }}>
                  {horasBonitas(Math.round(promFin * 10) / 10)} el sábado y el domingo,
                  contra {horasBonitas(Math.round(promSemana * 10) / 10)} de lunes a
                  viernes. Lo que no se llena, sale en negro.
                </p>
              </div>
              <div className="fila" style={{ gap: 10, flexShrink: 0 }}>
                <button
                  className="boton boton--primario"
                  onClick={() =>
                    api.llenarConDiferido({
                      desde: '01:00',
                      hasta: '06:00',
                      origen_desde: '07:00',
                      origen_hasta: '12:00',
                    })
                  }
                >
                  Llenar el fin de semana
                </button>
                <button className="boton">Escoger yo</button>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Mover una regla: la pregunta va en un panel al lado, nunca en un modal */}
      {arrastre && (
        <Panel
          titulo="¿Solo hoy, o siempre?"
          descripcion={`${arrastre.titulo} pasa de las ${minutosAHora12(arrastre.de)} a las ${minutosAHora12(arrastre.a)}`}
          alCerrar={() => setArrastre(null)}
        >
          <p className="subtitulo">
            {diaCorto(arrastre.dia)} {diaYMes(arrastre.dia)}
          </p>
          <button
            className="boton"
            style={{ justifyContent: 'flex-start', textAlign: 'left', padding: '14px 16px' }}
            onClick={() => setArrastre(null)}
          >
            <div>
              <div style={{ font: '600 14.5px var(--sans)' }}>Solo hoy</div>
              <div className="subtitulo">
                Se crea una excepción para este día. La regla queda como está.
              </div>
            </div>
          </button>
          <button
            className="boton"
            style={{ justifyContent: 'flex-start', textAlign: 'left', padding: '14px 16px' }}
            onClick={() => setArrastre(null)}
          >
            <div>
              <div style={{ font: '600 14.5px var(--sans)' }}>Siempre</div>
              <div className="subtitulo">
                Cambia la hora de la regla: afecta todos los días del patrón, desde hoy
                hasta que se venza.
              </div>
            </div>
          </button>
        </Panel>
      )}
    </>
  )
}
