import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import { EncabezadoDeAire } from '../componentes/EncabezadoDeAire'
import { Caratula } from '../componentes/Caratula'
import { Panel } from '../componentes/Panel'
import { Bitacora } from '../componentes/Bitacora'
import { IconoMano } from '../componentes/Iconos'
import { useEstado } from '../lib/estado'
import { api } from '../lib/api'
import { cuentaRegresiva, duracionLarga, hora } from '../lib/fechas'
import type { Alarma, FilaDelPlan } from '../lib/tipos'
import { esHueco } from '../lib/tipos'

export function AlAire() {
  const { estado } = useEstado()
  const [huecos, setHuecos] = useState<FilaDelPlan[]>([])
  const [panelControl, setPanelControl] = useState(false)

  const zona = estado?.canal.zona_horaria ?? 'UTC'
  const dia = estado?.dia_emision

  useEffect(() => {
    if (!dia) return
    api
      .plan(dia)
      .then((filas) => setHuecos(filas.filter(esHueco)))
      .catch(() => setHuecos([]))
  }, [dia])

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
      <EncabezadoDeAire />

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
                  Hoy sigues emitiendo con VLC. Antena787 está calculando el plan en
                  paralelo, sin tocar el aire.
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
                    {alAire.temporada ? `Temporada ${alAire.temporada} · ` : ''}
                    {alAire.episodio ? `Episodio ${alAire.episodio}  ·  ` : ''}
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
                    ? `${siguiente.episodio ? `Episodio ${siguiente.episodio} · ` : ''}${duracionLarga(siguiente.duracion_planeada_ms)}`
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
              style={{ width: '100%', color: 'var(--aqua)' }}
              disabled={enSombra}
              onClick={() => setPanelControl(true)}
            >
              <IconoMano tamano={17} color="var(--aqua)" />
              Tomar el control
            </button>
            <p className="ayuda" style={{ marginTop: 10 }}>
              {enSombra
                ? 'En modo sombra no hay aire que tomar: Antena787 todavía no está alimentando el transmisor.'
                : 'Vuelve solo cuando sueltes o al terminar el bloque'}
            </p>
          </div>

          {/* Lo que el sistema hizo solo (PRD §15, issue #7) */}
          <Bitacora zona={zona} ahora={ahora} />
        </aside>
      </div>

      {panelControl && (
        <Panel
          titulo="Tomar el control"
          descripcion="El aire pasa a ti hasta que lo sueltes o termine el bloque."
          alCerrar={() => setPanelControl(false)}
          pie={
            <>
              <button className="boton" onClick={() => setPanelControl(false)}>
                Cancelar
              </button>
              <button className="boton boton--primario" disabled>
                Tomar el control
              </button>
            </>
          }
        >
          <p className="subtitulo">
            Mientras lo tengas, aparece el panel de disparo y una cuenta regresiva del
            regreso automático. La vista de aire de la izquierda no se detiene ni se
            tapa.
          </p>
          <div className="campo">
            <label htmlFor="quien">¿Quién lo toma?</label>
            <input id="quien" type="text" placeholder="Tu nombre" />
            <span className="ayuda">Queda anotado. Una persona a la vez.</span>
          </div>
          <p className="ayuda">Esta parte la construye la siguiente entrega.</p>
        </Panel>
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
