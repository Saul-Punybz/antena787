import { useEffect, useState } from 'react'
import { MonitorDeAire } from '../componentes/MonitorDeAire'
import { Link } from 'react-router'
import { EncabezadoDeAire } from '../componentes/EncabezadoDeAire'
import { Caratula } from '../componentes/Caratula'
import { Panel } from '../componentes/Panel'
import { PuertaDelAire } from '../componentes/PuertaDelAire'
import { Bitacora } from '../componentes/Bitacora'
import { IconoMano } from '../componentes/Iconos'
import { useEstado } from '../lib/estado'
import { api } from '../lib/api'
import { cuentaRegresiva, duracionLarga, hora } from '../lib/fechas'
import type { Alarma, ControlManual, FilaDelPlan, TituloDeBiblioteca } from '../lib/tipos'
import { esHueco } from '../lib/tipos'

export function AlAire() {
  const { estado } = useEstado()
  const [huecos, setHuecos] = useState<FilaDelPlan[]>([])
  const [panelControl, setPanelControl] = useState(false)
  // El control manual (T5). Se pide al entrar y se refresca al abrir el
  // panel: no hace falta sondearlo cada segundo porque cada acción devuelve
  // el estado nuevo, y lo que cambia por su cuenta —el fin de bloque, el
  // timeout— llega por la bitácora.
  const [manual, setManual] = useState<ControlManual | null>(null)

  const zona = estado?.canal.zona_horaria ?? 'UTC'
  const dia = estado?.dia_emision

  useEffect(() => {
    if (!dia) return
    api
      .plan(dia)
      .then((filas) => setHuecos(filas.filter(esHueco)))
      .catch(() => setHuecos([]))
  }, [dia])

  useEffect(() => {
    api.manual().then(setManual).catch(() => setManual(null))
  }, [])

  if (!estado) {
    return (
      <>
        <EncabezadoDeAire />
        <p className="cargando">Buscando el canal…</p>
      </>
    )
  }

  const enSombra = estado.modo !== 'aire'
  const alAire = estado.al_aire
  const siguiente = estado.siguiente
  const ahora = Date.parse(estado.ahora)
  const inicio = alAire ? Date.parse(alAire.instante_planeado) : 0
  const fin = alAire ? inicio + alAire.duracion_planeada_ms : 0
  const avance = alAire ? Math.min(1, Math.max(0, (ahora - inicio) / (fin - inicio))) : 0

  return (
    <>
      {/* El botón de salir al aire va pegado a donde se dice el modo (F2-118). */}
      <EncabezadoDeAire accion={<PuertaDelAire />} />

      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'minmax(0, 1fr) 340px',
          gap: 22,
          alignItems: 'start',
        }}
      >
        {/* El cuadro grande */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <section
            className="tarjeta"
            style={{
              position: 'relative',
              aspectRatio: '16 / 9',
              overflow: 'hidden',
              background:
                'linear-gradient(150deg, #1b2233 0%, #241f2e 45%, #14181f 100%)',
              padding: 0,
            }}
          >
            <span
              style={{
                position: 'absolute',
                top: 18,
                left: 18,
                display: 'inline-flex',
                alignItems: 'center',
                gap: 8,
                background: 'rgba(11,14,18,.82)',
                border: '1px solid var(--borde)',
                borderRadius: 7,
                padding: '7px 13px',
                font: '600 12px var(--mono)',
                letterSpacing: 1.4,
              }}
            >
              <span className={enSombra ? 'tally tally--apagado' : 'tally'} />
              {enSombra ? 'MODO SOMBRA' : 'EN VIVO'}
            </span>

            {/*
              El monitor: cuando el canal está al aire, aquí se ve lo que está
              produciendo (F2-117). En sombra no hay nada que ver y lo dice el
              cartel de abajo.
            */}
            <MonitorDeAire monitor={estado?.monitor} enSombra={enSombra} />

            {enSombra && (
              <div
                style={{
                  position: 'absolute',
                  inset: 0,
                  display: 'flex',
                  flexDirection: 'column',
                  alignItems: 'center',
                  justifyContent: 'center',
                  textAlign: 'center',
                  padding: 24,
                  gap: 8,
                }}
              >
                <div style={{ font: '600 17px var(--sans)', color: 'var(--texto-2)' }}>
                  Aquí se vería tu señal
                </div>
                <div style={{ font: '400 14px var(--sans)', color: 'var(--texto-3)' }}>
                  Antena787 está calculando el plan en paralelo, sin tocar el aire. Cuando
                  quieras encender, el botón «Salir al aire» de arriba hace las
                  comprobaciones y te dice qué va a pasar antes de hacer nada.
                </div>
              </div>
            )}

            <div
              style={{
                position: 'absolute',
                left: 0,
                right: 0,
                bottom: 0,
                padding: '26px 28px 22px',
                background: 'linear-gradient(180deg, rgba(14,17,22,0), rgba(14,17,22,.92))',
              }}
            >
              <h2 style={{ font: '700 34px var(--sans)', margin: 0, letterSpacing: '-0.6px' }}>
                {alAire?.titulo ?? 'Nada al aire'}
              </h2>
              <p className="subtitulo" style={{ fontSize: 14.5 }}>
                {alAire ? (
                  <>
                    {alAire.episodio
                      ? `${typeof alAire.episodio === 'number' ? 'Episodio ' : ''}${alAire.episodio}  ·  `
                      : ''}
                    {hora(alAire.instante_planeado, zona)} –{' '}
                    {hora(new Date(fin), zona)}
                  </>
                ) : (
                  'El plan no tiene nada en esta franja.'
                )}
              </p>
              {alAire && (
                <div className="fila" style={{ gap: 16, marginTop: 14 }}>
                  <div
                    style={{
                      flexGrow: 1,
                      height: 4,
                      borderRadius: 2,
                      background: 'rgba(255,255,255,.14)',
                      overflow: 'hidden',
                    }}
                  >
                    <div
                      style={{
                        width: `${avance * 100}%`,
                        height: '100%',
                        background: 'var(--aqua)',
                      }}
                    />
                  </div>
                  <span className="mono" style={{ fontSize: 13.5, color: 'var(--texto-2)' }}>
                    faltan {cuentaRegresiva(fin - ahora)}
                  </span>
                </div>
              )}
            </div>
          </section>

          {/* Lo que sigue */}
          <section className="tarjeta" style={{ padding: '14px 18px' }}>
            <div className="fila" style={{ gap: 16 }}>
              <span className="rotulo" style={{ width: 46 }}>
                SIGUE
              </span>
              <div style={{ width: 54, height: 38, flexShrink: 0 }}>
                <Caratula nombre={siguiente?.titulo ?? '—'} alto={38} radio={6} />
              </div>
              <div className="crece">
                <div style={{ font: '600 15.5px var(--sans)' }}>
                  {siguiente?.titulo ?? 'Nada más en el plan de hoy'}
                </div>
                <div className="subtitulo">
                  {siguiente
                    ? `${siguiente.episodio ? `${typeof siguiente.episodio === 'number' ? 'Episodio ' : ''}${siguiente.episodio} · ` : ''}${duracionLarga(siguiente.duracion_planeada_ms)}`
                    : 'Después de esto, el relleno.'}
                </div>
              </div>
              {siguiente && (
                <span className="mono aqua" style={{ fontSize: 17 }}>
                  {hora(siguiente.instante_planeado, zona)}
                </span>
              )}
            </div>
          </section>

          {/* El retorno de aire, al lado y nunca encima */}
          <section className="tarjeta" style={{ padding: '14px 18px' }}>
            <div className="entre">
              <div>
                <div className="rotulo">RETORNO DE AIRE</div>
                <p className="subtitulo" style={{ marginTop: 6 }}>
                  {estado.retorno_de_aire?.hay
                    ? 'Lo que de verdad está saliendo, después del equipo de alertas.'
                    : 'Todavía no hay retorno de aire conectado. Sin él, una interrupción de alerta hay que marcarla a mano.'}
                </p>
              </div>
              <div
                style={{
                  width: 132,
                  height: 74,
                  flexShrink: 0,
                  borderRadius: 8,
                  border: '1px solid var(--borde)',
                  background: '#0b0e12',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  color: 'var(--texto-4)',
                  fontSize: 12,
                }}
              >
                {estado.retorno_de_aire?.hay ? 'en camino' : 'sin señal'}
              </div>
            </div>
          </section>
        </div>

        {/* El semáforo y las salidas */}
        <aside style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <div className="rotulo">ESTADO</div>
          {(estado.alarmas ?? []).map((a, i) => (
            <TarjetaDeAlarma key={i} alarma={a} />
          ))}

          <div className="tarjeta" style={{ padding: '16px 18px', marginTop: 6 }}>
            <div className="rotulo">SALIDAS</div>
            <ul style={{ listStyle: 'none', margin: '12px 0 0', padding: 0, display: 'grid', gap: 11 }}>
              {estado.salidas?.map((s) => (
                <li key={s.id} className="entre">
                  <span className="fila" style={{ gap: 10 }}>
                    <span
                      className={
                        'punto ' +
                        (s.estado_conexion === 'conectada'
                          ? 'punto--bien'
                          : s.estado_conexion === 'apagada'
                            ? ''
                            : 'punto--problema')
                      }
                    />
                    <span style={{ fontSize: 14 }}>{s.nombre}</span>
                  </span>
                  <span className="tenue" style={{ fontSize: 13 }}>
                    {s.estado_conexion === 'conectada'
                      ? 'saliendo'
                      : s.estado_conexion === 'apagada'
                        ? 'apagada'
                        : 'reintentando'}
                  </span>
                </li>
              ))}
            </ul>
          </div>

          {/* Huecos de hoy */}
          {huecos.length > 0 && (
            <div className="tarjeta" style={{ padding: '16px 18px' }}>
              <div className="rotulo">HUECOS DE HOY</div>
              <ul style={{ listStyle: 'none', margin: '12px 0 0', padding: 0, display: 'grid', gap: 9 }}>
                {huecos.slice(0, 4).map((h, i) =>
                  esHueco(h) ? (
                    <li key={i} className="entre" style={{ fontSize: 13.5 }}>
                      <span className="mono">
                        {hora(h.inicio, zona)} – {hora(h.fin, zona)}
                      </span>
                      <span className="tenue">
                        {duracionLarga(Date.parse(h.fin) - Date.parse(h.inicio))}
                      </span>
                    </li>
                  ) : null,
                )}
              </ul>
              <Link to="/parrilla" style={{ fontSize: 13, display: 'inline-block', marginTop: 12 }}>
                llenarlos en la parrilla
              </Link>
            </div>
          )}

          {/* Tomar el control: siempre visible (PRD §13) */}
          <div className="tarjeta" style={{ padding: '16px 18px', textAlign: 'center' }}>
            <button
              className="boton"
              style={{ width: '100%', color: manual?.en_manual ? 'var(--ambar)' : 'var(--aqua)' }}
              disabled={enSombra}
              onClick={() => {
                setPanelControl(true)
                api.manual().then(setManual).catch(() => {})
              }}
            >
              <IconoMano tamano={17} color={manual?.en_manual ? 'var(--ambar)' : 'var(--aqua)'} />
              {manual?.en_manual
                ? manual.soy_yo
                  ? 'Tienes el control'
                  : `${manual.quien} tiene el control`
                : 'Tomar el control'}
            </button>
            <p className="ayuda" style={{ marginTop: 10 }}>
              {enSombra
                ? 'En modo sombra no hay aire que tomar: Antena787 todavía no está alimentando el transmisor.'
                : manual?.en_manual
                  ? 'Mientras alguien tiene el aire, la parrilla no lo toca.'
                  : 'El aire pasa a una persona hasta que lo suelte o termine el bloque.'}
            </p>
          </div>

          {/* Lo que el sistema hizo solo (PRD §15, issue #7) */}
          <Bitacora zona={zona} ahora={ahora} />
        </aside>
      </div>

      {panelControl && (
        <PanelDeControl
          manual={manual}
          zona={zona}
          alCambiar={setManual}
          alCerrar={() => setPanelControl(false)}
        />
      )}
    </>
  )
}

/**
 * Una alarma se pinta con lo que traiga: `detalle` y `accion` son opcionales
 * (los avisos de vencimiento llegan solo con nivel y texto) y la tarjeta tiene
 * que quedar igual de limpia sin ellos.
 */
function TarjetaDeAlarma({ alarma }: { alarma: Alarma }) {
  const nivel: Alarma['nivel'] =
    alarma.nivel === 'problema' || alarma.nivel === 'aviso' || alarma.nivel === 'bien'
      ? alarma.nivel
      : 'aviso'
  const clase =
    nivel === 'problema'
      ? 'tarjeta tarjeta--problema'
      : nivel === 'aviso'
        ? 'tarjeta tarjeta--aviso'
        : 'tarjeta'
  return (
    <div className={clase} style={{ padding: '14px 16px' }}>
      <div className="fila" style={{ alignItems: 'flex-start', gap: 12 }}>
        <span className={'punto punto--' + nivel} style={{ marginTop: 5 }} />
        <div className="crece">
          <div style={{ font: '600 14.5px var(--sans)' }}>{alarma.texto}</div>
          {alarma.detalle && (
            <div className="subtitulo" style={{ marginTop: 2 }}>
              {alarma.detalle}
            </div>
          )}
        </div>
        {alarma.accion?.ruta && alarma.accion.texto && (
          <Link to={alarma.accion.ruta} style={{ fontSize: 13, flexShrink: 0 }}>
            {alarma.accion.texto}
          </Link>
        )}
      </div>
    </div>
  )
}

/**
 * El panel del control manual (T5).
 *
 * Tres estados, y la pantalla no es la misma en ninguno:
 *
 * 1. **Nadie tiene el aire** — un botón para tomarlo, y ya.
 * 2. **Lo tiene otra persona** — su nombre y desde cuándo, y un botón
 *    explícito para quitárselo. No se le quita por accidente: es otro botón,
 *    con otro texto, y queda anotado quién lo hizo (F2-77).
 * 3. **Lo tienes tú** — el panel de disparo, que **solo existe aquí**: en
 *    automático no se enseña ni apagado (F2-35).
 *
 * Y dos formas de devolverlo que no son la misma: soltar espera a que acabe
 * lo que está sonando —como máximo un minuto— y parar todo corta en seco. La
 * diferencia importa cuando lo que suena es un spot que alguien pagó (F2-31
 * frente a F2-78).
 */
function PanelDeControl({
  manual,
  zona,
  alCambiar,
  alCerrar,
}: {
  manual: ControlManual | null
  zona: string
  alCambiar: (m: ControlManual) => void
  alCerrar: () => void
}) {
  const [error, setError] = useState('')
  const [aviso, setAviso] = useState('')
  const [ocupado, setOcupado] = useState(false)

  async function hacer(que: () => Promise<ControlManual>, texto = '') {
    setOcupado(true)
    setError('')
    try {
      alCambiar(await que())
      setAviso(texto)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'No se pudo.')
    } finally {
      setOcupado(false)
    }
  }

  async function soltar() {
    setOcupado(true)
    setError('')
    try {
      const r = await api.soltarElControl()
      alCambiar(r.manual)
      setAviso(r.texto)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'No se pudo soltar el control.')
    } finally {
      setOcupado(false)
    }
  }

  const suyo = manual?.en_manual === true && manual.soy_yo
  const deOtro = manual?.en_manual === true && !manual.soy_yo

  return (
    <Panel
      titulo={suyo ? 'Tienes el control' : deOtro ? 'El aire lo tiene otra persona' : 'Tomar el control'}
      descripcion={
        suyo
          ? 'Mientras lo tengas, la parrilla no toca el aire.'
          : deOtro
            ? 'Una persona a la vez: es lo que impide que dos saquen cosas distintas al mismo tiempo.'
            : 'El aire pasa a ti hasta que lo sueltes o termine el bloque.'
      }
      alCerrar={alCerrar}
      pie={
        <button className="boton" onClick={alCerrar}>
          Cerrar
        </button>
      }
    >
      {error && <div className="error-claro">{error}</div>}
      {aviso && <div className="nota nota--aviso">{aviso}</div>}

      {deOtro && manual && (
        <>
          <p className="subtitulo">
            <strong>{manual.quien}</strong> tiene el control desde las{' '}
            {manual.desde ? hora(manual.desde, zona) : '—'}.
          </p>
          <button
            className="boton boton--peligro"
            disabled={ocupado}
            onClick={() => void hacer(api.quitarElControl, `le quitaste el control a ${manual.quien}`)}
          >
            Quitárselo
          </button>
          <p className="ayuda">Queda anotado en la bitácora con tu nombre y la hora.</p>
        </>
      )}

      {!manual?.en_manual && (
        <button
          className="boton boton--primario"
          disabled={ocupado}
          onClick={() => void hacer(api.tomarElControl, 'el aire es tuyo')}
        >
          <IconoMano tamano={16} />
          Tomar el control
        </button>
      )}

      {suyo && manual && (
        <>
          <p className="subtitulo">
            Desde las {manual.desde ? hora(manual.desde, zona) : '—'}.
            {manual.fin_de_bloque && (
              <>
                {' '}
                Vuelve solo al automático a las {hora(manual.fin_de_bloque, zona)}, cuando
                se acabe el bloque.
              </>
            )}
          </p>
          {manual.soltandose_en && (
            <div className="nota nota--aviso">
              El aire vuelve al automático a las {hora(manual.soltandose_en, zona)}, cuando
              acabe lo que está sonando. Si hace falta antes, «parar todo» corta en seco.
            </div>
          )}

          <PanelDeDisparo
            alDisparar={async (id) => {
              setError('')
              try {
                const r = await api.dispararAlAire(id)
                alCambiar(r.manual)
                setAviso('al aire')
              } catch (e) {
                setError(e instanceof Error ? e.message : 'No se pudo poner al aire.')
              }
            }}
          />

          <div className="fila" style={{ gap: 10, flexWrap: 'wrap', marginTop: 14 }}>
            <button className="boton" disabled={ocupado} onClick={() => void soltar()}>
              Volver al automático
            </button>
            <button
              className="boton boton--peligro"
              disabled={ocupado}
              onClick={() => void hacer(api.pararTodo, 'el aire se cortó y volvió al automático')}
            >
              Parar todo
            </button>
          </div>
          <p className="ayuda">
            <strong>Volver al automático</strong> espera a que acabe lo que está sonando,
            como máximo un minuto: no corta nada por el medio.{' '}
            <strong>Parar todo</strong> corta en seco, ahora mismo, y lo que se quede a
            medias queda marcado como tal.
          </p>
        </>
      )}

      {manual && manual.historial.length > 0 && (
        <div style={{ marginTop: 18 }}>
          <div className="rotulo" style={{ marginBottom: 8 }}>
            QUIÉN HA TENIDO EL AIRE
          </div>
          <ul style={{ listStyle: 'none', margin: 0, padding: 0, display: 'grid', gap: 6 }}>
            {manual.historial.map((h, i) => (
              <li key={i} className="entre" style={{ fontSize: 13.5 }}>
                <span>
                  {h.quien || 'estación'}{' '}
                  <span className="tenue">
                    {hora(h.desde, zona)}
                    {h.hasta ? ` – ${hora(h.hasta, zona)}` : ''}
                  </span>
                </span>
                <span className="tenue">{h.texto}</span>
              </li>
            ))}
          </ul>
        </div>
      )}
    </Panel>
  )
}

/**
 * El panel de disparo: qué se pone al aire ahora mismo.
 *
 * Solo aparece con el control tomado (F2-35), y por eso vive dentro del bloque
 * que ya comprobó que el aire es tuyo — no se enseña apagado ni escondido: en
 * automático no existe.
 *
 * La biblioteca se pide al montarse, no al abrir el panel de control: cuando
 * una persona llega hasta aquí es porque ya sabe que quiere sacar algo, y
 * esperar una lista es justo lo que no se puede hacer en ese momento.
 */
function PanelDeDisparo({ alDisparar }: { alDisparar: (materialId: number) => Promise<void> }) {
  const [titulos, setTitulos] = useState<TituloDeBiblioteca[] | null>(null)
  const [busca, setBusca] = useState('')

  useEffect(() => {
    api
      .biblioteca()
      .then(setTitulos)
      .catch(() => setTitulos([]))
  }, [])

  const listos = (titulos ?? []).filter(
    (t) => t.material_id !== undefined && t.estado_material === 'listo',
  )
  const filtrados = busca.trim()
    ? listos.filter((t) => t.nombre.toLowerCase().includes(busca.trim().toLowerCase()))
    : listos

  return (
    <div style={{ marginTop: 16 }}>
      <div className="rotulo" style={{ marginBottom: 8 }}>
        PONER AL AIRE AHORA
      </div>
      {titulos === null && <p className="cargando">Buscando el material…</p>}
      {titulos !== null && listos.length === 0 && (
        <p className="ayuda">
          No hay nada preparado para el aire todavía. Lo que está a medio preparar no se
          puede disparar: saldría en otro formato y con otro volumen.
        </p>
      )}
      {listos.length > 0 && (
        <>
          <div className="campo">
            <label htmlFor="disparo-busca">Buscar</label>
            <input
              id="disparo-busca"
              value={busca}
              placeholder="parte del nombre"
              onChange={(e) => setBusca(e.target.value)}
            />
          </div>
          <ul
            style={{
              listStyle: 'none',
              margin: 0,
              padding: 0,
              display: 'grid',
              gap: 6,
              maxHeight: 260,
              overflowY: 'auto',
            }}
          >
            {filtrados.map((t) => (
              <li key={t.id} className="entre" style={{ gap: 10 }}>
                <span style={{ minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis' }}>
                  {t.nombre}{' '}
                  <span className="tenue" style={{ fontSize: 12.5 }}>
                    {duracionLarga(t.duracion_ms)}
                  </span>
                </span>
                <button
                  className="boton"
                  onClick={() => void alDisparar(t.material_id as number)}
                >
                  Al aire
                </button>
              </li>
            ))}
            {filtrados.length === 0 && (
              <li className="ayuda">nada que se llame así entre lo que está listo</li>
            )}
          </ul>
        </>
      )}
    </div>
  )
}
