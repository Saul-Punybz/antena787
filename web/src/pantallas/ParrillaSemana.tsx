import { useCallback, useEffect, useMemo, useState } from 'react'
import { PestanasDeParrilla } from './Parrilla'
import { BibliotecaAlLado } from '../componentes/BibliotecaAlLado'
import { EditorDeRegla } from '../componentes/EditorDeRegla'
import { Panel } from '../componentes/Panel'
import { IconoChincheta } from '../componentes/Iconos'
import { api } from '../lib/api'
import { useEstado } from '../lib/estado'
import {
  diaCorto,
  diaYMes,
  horasBonitas,
  instanteEnZona,
  minutosAHhMm,
  minutosAHora12,
  partes,
  sumarDias,
} from '../lib/fechas'
import { claveDeRegla, etiquetaDelHueco, mediaHoraDelClic, usarHueco } from '../lib/huecos'
import { ErrorDeApi, esHueco, type FranjaSemana, type SemanaDelPlan } from '../lib/tipos'

const DESDE_MIN = 6 * 60 // la tira va de las 6:00 AM
const HASTA_MIN = 24 * 60 // a medianoche
const ANCHO = HASTA_MIN - DESDE_MIN

interface Bloque {
  titulo: string | null
  desde: number
  hasta: number
  enVivo: boolean
  /** El plan_item que ocupa la franja, cuando el servidor lo manda. */
  planId: number | null
  /** Alguien lo movió a mano: el recálculo no lo pisa. */
  fijado: boolean
}

function bloquesDelDia(franjas: FranjaSemana[]): Bloque[] {
  const out: Bloque[] = []
  for (let k = DESDE_MIN / 30; k < HASTA_MIN / 30; k++) {
    const f = franjas[k]
    const titulo = f?.titulo ?? null
    const planId = f?.plan_id ?? null
    const ultimo = out[out.length - 1]
    if (ultimo && ultimo.titulo === titulo && ultimo.planId === planId) {
      ultimo.hasta = (k + 1) * 30
    } else {
      out.push({
        titulo,
        desde: k * 30,
        hasta: (k + 1) * 30,
        enVivo: Boolean(f?.en_vivo),
        planId,
        fijado: Boolean(f?.fijado),
      })
    }
  }
  return out
}

/**
 * «Semana del 6 al 12 de septiembre», y con el mes de los dos cuando la semana
 * cruza de mes: «del 30 de agosto al 5 de septiembre».
 */
function rangoDeLaSemana(semana: SemanaDelPlan): string {
  const primero = semana.dias[0].dia
  const ultimo = semana.dias[6].dia
  const mismoMes = primero.slice(0, 7) === ultimo.slice(0, 7)
  const desde = mismoMes ? diaYMes(primero).replace(/ de \w+$/, '') : diaYMes(primero)
  return `Semana del ${desde} al ${diaYMes(ultimo)}`
}

function colorDe(titulo: string): string {
  const semilla = [...titulo].reduce((a, c) => (a * 31 + c.charCodeAt(0)) % 360, 11)
  return `hsl(${semilla} 19% 17%)`
}

interface Arrastre {
  titulo: string
  dia: string
  de: number
  a: number
  planId: number | null
}

interface Fijado {
  titulo: string
  dia: string
  desde: number
  planId: number | null
}

export function ParrillaSemana() {
  const { estado } = useEstado()
  const zona = estado?.canal.zona_horaria ?? 'UTC'
  const hoy = estado?.dia_emision ?? '2026-09-04'
  const anio = Number(hoy.slice(0, 4))
  const [desde, setDesde] = useState<string | null>(null)
  const [semana, setSemana] = useState<SemanaDelPlan | null>(null)
  const [arrastre, setArrastre] = useState<Arrastre | null>(null)
  const [soltando, setSoltando] = useState<Fijado | null>(null)
  const [guardando, setGuardando] = useState(false)
  const [problema, setProblema] = useState<string | null>(null)
  // Cada regla guardada cambia el inventario: la columna se vuelve a leer.
  const [cambios, setCambios] = useState(0)
  // El atajo del hueco: escogerlo abre la regla que lo llenaría.
  const { hueco, reglaNueva, abrirHueco, abrirRegla, escogerTitulo, cerrar } = usarHueco()

  // La semana que empieza el domingo de la semana en curso.
  const inicio = useMemo(() => {
    if (desde) return desde
    const d = new Date(hoy + 'T00:00:00Z')
    return sumarDias(hoy, -d.getUTCDay())
  }, [desde, hoy])

  const recargar = useCallback(async () => {
    const nueva = await api.planSemana(inicio).catch(() => null)
    if (nueva) setSemana(nueva)
  }, [inicio])

  useEffect(() => {
    api.planSemana(inicio).then(setSemana).catch(() => setSemana(null))
  }, [inicio])

  /**
   * El número del bloque. La semana lo trae en `plan_id`; si el servidor
   * todavía no lo manda, se busca en el plan del día por la hora en que
   * empieza.
   */
  async function idDelBloque(dia: string, minuto: number, planId: number | null) {
    if (planId) return planId
    const filas = await api.plan(dia).catch(() => [])
    for (const f of filas) {
      if (!esHueco(f) && f.hora_local === minutosAHhMm(minuto)) return f.id
    }
    return null
  }

  function decirElProblema(e: unknown, porDefecto: string) {
    setProblema(e instanceof ErrorDeApi ? e.message : porDefecto)
  }

  /** Solo hoy: se clava este bloque en su hora nueva y la regla queda igual. */
  async function moverSoloHoy() {
    if (!arrastre) return
    setGuardando(true)
    setProblema(null)
    try {
      const id = await idDelBloque(arrastre.dia, arrastre.de, arrastre.planId)
      if (!id) throw new ErrorDeApi('Ese bloque ya no está en la parrilla.', 404)
      await api.cambiarPlan(id, {
        instante_planeado: instanteEnZona(arrastre.dia, arrastre.a, zona),
      })
      setArrastre(null)
      await recargar()
    } catch (e) {
      decirElProblema(e, 'No se pudo mover el bloque.')
    } finally {
      setGuardando(false)
    }
  }

  /** Siempre: cambia la hora de la regla y se vuelve a armar la parrilla. */
  async function moverSiempre() {
    if (!arrastre) return
    setGuardando(true)
    setProblema(null)
    try {
      const reglas = await api.reglas()
      const regla =
        reglas.find((r) => r.titulo === arrastre.titulo && r.hora === arrastre.de) ??
        reglas.find((r) => r.titulo === arrastre.titulo)
      if (!regla)
        throw new ErrorDeApi(
          `No hay ninguna regla de ${arrastre.titulo}: este bloque no sale de una regla, así que solo se puede mover por hoy.`,
          404,
        )
      await api.editarRegla(regla.id, {
        tipo: regla.tipo,
        title_id: regla.title_id,
        live_source_id: regla.live_source_id,
        titulo: regla.titulo,
        patron_de_dias: regla.patron_de_dias,
        hora: arrastre.a,
        duracion_slot_ms: regla.duracion_slot_ms,
        fecha_inicio: regla.fecha_inicio,
        fecha_fin: regla.fecha_fin,
        episodios_por_corrida: regla.episodios_por_corrida,
        releva_a: regla.releva_a,
        repite_a: regla.repite_a,
        activa: regla.activa,
      })
      await api.recalcular()
      setArrastre(null)
      await recargar()
    } catch (e) {
      decirElProblema(e, 'No se pudo cambiar la hora de la regla.')
    } finally {
      setGuardando(false)
    }
  }

  /** Soltar: el bloque vuelve a obedecer a su regla. */
  async function soltar() {
    if (!soltando) return
    setGuardando(true)
    setProblema(null)
    try {
      const id = await idDelBloque(soltando.dia, soltando.desde, soltando.planId)
      if (!id) throw new ErrorDeApi('Ese bloque ya no está en la parrilla.', 404)
      await api.cambiarPlan(id, { fijado: false })
      await api.recalcular().catch(() => {})
      setSoltando(null)
      await recargar()
    } catch (e) {
      decirElProblema(e, 'No se pudo soltar el bloque.')
    } finally {
      setGuardando(false)
    }
  }

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

  /**
   * El primer vacío del fin de semana que valga la pena: de una hora para
   * arriba. Es a donde lleva «Escoger yo»; si no hay ninguno tan grande,
   * sirve el primero que haya.
   */
  function primerHuecoDelFinDeSemana() {
    let primero: { dia: string; desde: number; hasta: number } | null = null
    for (const d of finDeSemana) {
      for (const b of bloquesDelDia(d.franjas)) {
        if (b.titulo) continue
        const vacio = { dia: d.dia, desde: b.desde, hasta: b.hasta }
        if (b.hasta - b.desde >= 60) return vacio
        primero = primero ?? vacio
      }
    }
    return primero
  }

  return (
    <>
      <div className="encabezado">
        <div>
          <h1 className="titulo-pantalla">
            {semana ? rangoDeLaSemana(semana) : 'Semana'}
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

      {/* La tira a la izquierda y el inventario al lado: se ve qué falta y
          qué hay para ponerlo sin cambiar de pantalla. */}
      <div className={'con-biblioteca' + (reglaNueva ? ' con-biblioteca--corrida' : '')}>
        <div>
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
                          const b = JSON.parse(dato) as {
                            titulo: string
                            desde: number
                            planId: number | null
                          }
                          if (min === b.desde && d.dia === arrastre?.dia) return
                          setProblema(null)
                          setArrastre({
                            titulo: b.titulo,
                            dia: d.dia,
                            de: b.desde,
                            a: min,
                            planId: b.planId ?? null,
                          })
                        }}
                      >
                        {bloquesDelDia(d.franjas).map((b) => {
                          const izq = ((b.desde - DESDE_MIN) / ANCHO) * 100
                          const ancho = ((b.hasta - b.desde) / ANCHO) * 100
                          if (!b.titulo) {
                            const vacio = { dia: d.dia, desde: b.desde, hasta: b.hasta }
                            const escogido =
                              hueco?.dia === d.dia &&
                              hueco.desde >= b.desde &&
                              hueco.desde < b.hasta
                            return (
                              <button
                                key={b.desde}
                                type="button"
                                className={'hueco' + (escogido ? ' hueco--escogido' : '')}
                                aria-label={etiquetaDelHueco(vacio)}
                                title={`Vacío: ${minutosAHora12(b.desde)} – ${minutosAHora12(b.hasta)} · toca para poner algo`}
                                onClick={(e) =>
                                  abrirHueco({
                                    ...vacio,
                                    desde: mediaHoraDelClic(
                                      e.clientX,
                                      e.currentTarget.getBoundingClientRect(),
                                      b,
                                    ),
                                  })
                                }
                                style={{
                                  left: `${izq}%`,
                                  width: `calc(${ancho}% - 1px)`,
                                }}
                              />
                            )
                          }
                          return (
                            <div
                              key={b.desde}
                              draggable
                              onDragStart={(e) => {
                                e.dataTransfer.setData(
                                  'text/antena-bloque',
                                  JSON.stringify({
                                    titulo: b.titulo,
                                    desde: b.desde,
                                    planId: b.planId,
                                  }),
                                )
                              }}
                              title={
                                `${b.titulo} · ${minutosAHora12(b.desde)}` +
                                (b.fijado ? ' · puesto a mano' : '')
                              }
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
                                border: b.enVivo
                                  ? '1.5px solid var(--aqua)'
                                  : b.fijado
                                    ? '1.5px solid var(--ambar)'
                                    : undefined,
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
                              {b.fijado && (
                                <button
                                  className="chincheta"
                                  aria-label={`${b.titulo} está puesto a mano a las ${minutosAHora12(b.desde)}. Soltarlo.`}
                                  title="Puesto a mano · soltar"
                                  onClick={(e) => {
                                    e.stopPropagation()
                                    setProblema(null)
                                    setSoltando({
                                      titulo: b.titulo ?? '',
                                      dia: d.dia,
                                      desde: b.desde,
                                      planId: b.planId,
                                    })
                                  }}
                                >
                                  <IconoChincheta tamano={11} color="var(--ambar)" grosor={2} />
                                </button>
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
                  <span className="fila" style={{ gap: 8 }}>
                    <i
                      style={{
                        width: 26,
                        height: 11,
                        borderRadius: 3,
                        border: '1.5px solid var(--ambar)',
                      }}
                    />
                    puesto a mano
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
                    <button
                      className="boton"
                      onClick={() => {
                        const vacio = primerHuecoDelFinDeSemana()
                        if (vacio) abrirHueco(vacio)
                        // Sin un vacío concreto, la regla abre para el fin de
                        // semana y la hora la pone quien programa.
                        else
                          abrirRegla({
                            patron_de_dias: '_____SD',
                            nota: 'Sábados y domingos. Ponle la hora y el programa.',
                          })
                      }}
                    >
                      Escoger yo
                    </button>
                  </div>
                </div>
              )}
            </div>
          )}
        </div>

        <BibliotecaAlLado
          resaltarSinProgramar={Boolean(hueco)}
          alEscoger={escogerTitulo}
          anio={anio}
          recargar={cambios}
        />
      </div>

      {/* El hueco escogido abre la regla que lo llenaría: con el día y la hora
          puestos, y el título si vino de la biblioteca. */}
      {reglaNueva && (
        <EditorDeRegla
          // Escoger otro título o otro hueco vuelve a llenar el formulario:
          // sin la llave, React se queda con lo que había la primera vez.
          key={claveDeRegla(reglaNueva)}
          regla={null}
          inicial={reglaNueva}
          alCerrar={cerrar}
          alGuardar={async () => {
            // El editor ya volvió a armar el plan: aquí solo hay que
            // refrescar lo que se está mirando.
            cerrar()
            // Lo sin programar ya es otro: la columna se vuelve a leer.
            setCambios((n) => n + 1)
            await recargar()
          }}
        />
      )}

      {/* Mover una regla: la pregunta va en un panel al lado, nunca en un modal */}
      {arrastre && (
        <Panel
          titulo="¿Solo hoy, o siempre?"
          descripcion={`${arrastre.titulo} pasa de las ${minutosAHora12(arrastre.de)} a las ${minutosAHora12(arrastre.a)}`}
          alCerrar={() => {
            setArrastre(null)
            setProblema(null)
          }}
        >
          <p className="subtitulo">
            {diaCorto(arrastre.dia)} {diaYMes(arrastre.dia)}
          </p>
          {problema && <div className="error-en-cristiano">{problema}</div>}
          <button
            className="boton"
            disabled={guardando}
            style={{ justifyContent: 'flex-start', textAlign: 'left', padding: '14px 16px' }}
            onClick={moverSoloHoy}
          >
            <div>
              <div style={{ font: '600 14.5px var(--sans)' }}>Solo hoy</div>
              <div className="subtitulo">
                Este bloque queda puesto a mano a esa hora. La regla no cambia y el
                recálculo no lo vuelve a mover.
              </div>
            </div>
          </button>
          <button
            className="boton"
            disabled={guardando}
            style={{ justifyContent: 'flex-start', textAlign: 'left', padding: '14px 16px' }}
            onClick={moverSiempre}
          >
            <div>
              <div style={{ font: '600 14.5px var(--sans)' }}>Siempre</div>
              <div className="subtitulo">
                Cambia la hora de la regla: afecta todos los días del patrón, desde hoy
                hasta que se venza.
              </div>
            </div>
          </button>
          {guardando && <p className="cargando">Guardando el cambio…</p>}
        </Panel>
      )}

      {/* Soltar un bloque puesto a mano */}
      {soltando && (
        <Panel
          titulo="Puesto a mano"
          descripcion={`${soltando.titulo} está clavado a las ${minutosAHora12(soltando.desde)}`}
          alCerrar={() => {
            setSoltando(null)
            setProblema(null)
          }}
          pie={
            <>
              <button
                className="boton"
                onClick={() => {
                  setSoltando(null)
                  setProblema(null)
                }}
              >
                Dejarlo así
              </button>
              <button className="boton boton--primario" onClick={soltar} disabled={guardando}>
                {guardando ? 'Soltando…' : 'Soltar'}
              </button>
            </>
          }
        >
          <p className="subtitulo">
            {diaCorto(soltando.dia)} {diaYMes(soltando.dia)}
          </p>
          {problema && <div className="error-en-cristiano">{problema}</div>}
          <p className="subtitulo">
            Alguien lo movió a mano, así que la parrilla lo deja donde está. Al soltarlo
            vuelve a la hora que le da su regla.
          </p>
        </Panel>
      )}
    </>
  )
}
