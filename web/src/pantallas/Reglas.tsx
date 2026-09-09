import { useEffect, useMemo, useState } from 'react'
import { Caratula } from '../componentes/Caratula'
import { Panel } from '../componentes/Panel'
import { PatronDeDias } from '../componentes/PatronDeDias'
import { IconoMas } from '../componentes/Iconos'
import { api } from '../lib/api'
import { useEstado } from '../lib/estado'
import {
  fechaDeRegla,
  hhMmAMinutos,
  minutosAHhMm,
  minutosAHora12,
  textoDiasRestantes,
} from '../lib/fechas'
import { ErrorDeApi, type Regla, type ResumenDeImportacion } from '../lib/tipos'

type Filtro = 'todas' | 'vencen' | 'vivo'

export function Reglas() {
  const { estado } = useEstado()
  const [reglas, setReglas] = useState<Regla[] | null>(null)
  const [filtro, setFiltro] = useState<Filtro>('todas')
  const [editando, setEditando] = useState<Regla | 'nueva' | null>(null)
  const [importando, setImportando] = useState(false)

  const anio = Number((estado?.dia_emision ?? '2026-01-01').slice(0, 4))

  function cargar() {
    api.reglas().then(setReglas).catch(() => setReglas([]))
  }
  useEffect(cargar, [])

  const vencenPronto = useMemo(
    () => (reglas ?? []).filter((r) => r.dias_restantes <= 30),
    [reglas],
  )
  const enVivo = useMemo(() => (reglas ?? []).filter((r) => r.tipo === 'vivo'), [reglas])

  // Lo que urge, arriba: el semáforo de vencimiento es para verlo sin buscar.
  const visibles = (
    filtro === 'vencen' ? vencenPronto : filtro === 'vivo' ? enVivo : (reglas ?? [])
  )
    .slice()
    .sort((a, b) => {
      const urge = (r: Regla) => (r.dias_restantes <= 3 ? 0 : 1)
      return urge(a) - urge(b) || a.hora - b.hora || a.titulo.localeCompare(b.titulo)
    })

  return (
    <>
      <div className="encabezado">
        <div>
          <h1 className="titulo-pantalla">Reglas de programación</h1>
          <p className="subtitulo">
            {reglas
              ? `${reglas.length} reglas arman la semana completa. La parrilla sale de aquí.`
              : 'Cargando las reglas del canal.'}
          </p>
        </div>
        <div className="fila" style={{ gap: 10 }}>
          <button className="boton" onClick={() => setImportando(true)}>
            Pegar desde Excel/Sheets
          </button>
          <button className="boton boton--primario" onClick={() => setEditando('nueva')}>
            <IconoMas tamano={16} color="#06141a" />
            Nueva regla
          </button>
        </div>
      </div>

      <div className="pastillas">
        <button
          className={'pastilla' + (filtro === 'todas' ? ' pastilla--activa' : '')}
          onClick={() => setFiltro('todas')}
        >
          Todas · {reglas?.length ?? 0}
        </button>
        <button
          className={
            'pastilla pastilla--roja' + (filtro === 'vencen' ? ' pastilla--activa' : '')
          }
          onClick={() => setFiltro('vencen')}
        >
          Se vencen pronto · {vencenPronto.length}
        </button>
        <button
          className={'pastilla' + (filtro === 'vivo' ? ' pastilla--activa' : '')}
          onClick={() => setFiltro('vivo')}
        >
          En vivo · {enVivo.length}
        </button>
      </div>

      {!reglas && <p className="cargando">Cargando…</p>}

      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))',
          gap: 16,
        }}
      >
        {visibles.map((r) => (
          <TarjetaDeRegla key={r.id} regla={r} anio={anio} alAbrir={() => setEditando(r)} />
        ))}
      </div>

      {editando && (
        <EditorDeRegla
          regla={editando === 'nueva' ? null : editando}
          alCerrar={() => setEditando(null)}
          alGuardar={() => {
            setEditando(null)
            cargar()
          }}
        />
      )}

      {importando && (
        <PanelDeImportacion
          alCerrar={() => setImportando(false)}
          alTerminar={() => {
            setImportando(false)
            cargar()
          }}
        />
      )}
    </>
  )
}

function TarjetaDeRegla({
  regla,
  anio,
  alAbrir,
}: {
  regla: Regla
  anio: number
  alAbrir: () => void
}) {
  const urge = regla.dias_restantes <= 3
  const pronto = regla.dias_restantes <= 14
  return (
    <button
      className="tarjeta"
      onClick={alAbrir}
      style={{
        overflow: 'hidden',
        padding: 0,
        textAlign: 'left',
        cursor: 'pointer',
        borderColor: urge ? 'var(--rojo)' : 'var(--borde)',
        background: 'var(--superficie)',
        color: 'inherit',
        font: 'inherit',
      }}
    >
      <div style={{ height: 86 }}>
        <Caratula nombre={regla.titulo} alto="100%" />
      </div>
      <div style={{ padding: '13px 15px 15px' }}>
        <div
          style={{
            display: 'flex',
            alignItems: 'baseline',
            justifyContent: 'space-between',
            gap: 8,
          }}
        >
          <span
            style={{
              font: '600 15px var(--sans)',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
            }}
          >
            {regla.titulo}
          </span>
          <span className="mono aqua" style={{ fontSize: 13, flexShrink: 0 }}>
            {minutosAHora12(regla.hora)}
          </span>
        </div>
        <div style={{ marginTop: 11 }}>
          <PatronDeDias patron={regla.patron_de_dias} />
        </div>
        <div className="fila" style={{ gap: 8, marginTop: 12 }}>
          <span
            className={
              'punto ' + (urge ? 'punto--problema' : pronto ? 'punto--aviso' : 'punto--bien')
            }
          />
          <span
            style={{
              font: '500 13px var(--sans)',
              color: urge ? 'var(--rojo)' : pronto ? 'var(--ambar)' : 'var(--verde)',
            }}
          >
            {textoDiasRestantes(regla.dias_restantes)}
          </span>
        </div>
        <div className="mono tenue" style={{ fontSize: 12.5, marginTop: 6 }}>
          {fechaDeRegla(regla.fecha_inicio, anio)} – {fechaDeRegla(regla.fecha_fin, anio)}
        </div>
        <div className="fila" style={{ gap: 7, marginTop: 12, flexWrap: 'wrap' }}>
          {urge && <span className="etiqueta etiqueta--renovar">RENOVAR YA</span>}
          {regla.tipo === 'vivo' && <span className="etiqueta etiqueta--vivo">EN VIVO</span>}
          {regla.episodios_por_corrida > 1 && (
            <span className="etiqueta etiqueta--nota">
              ×{regla.episodios_por_corrida} episodios
            </span>
          )}
          {regla.releva_a_titulo && (
            <span className="etiqueta etiqueta--nota">releva a {regla.releva_a_titulo}</span>
          )}
          {regla.repite_a_titulo && (
            <span className="etiqueta etiqueta--nota">repite a {regla.repite_a_titulo}</span>
          )}
        </div>
      </div>
    </button>
  )
}

/** Validación en cristiano: la local y la que devuelva el servidor. */
function validar(v: {
  titulo: string
  patron: string
  desde: string
  hasta: string
}): string | null {
  if (!v.titulo.trim()) return 'Ponle el nombre del programa.'
  if (!/[LMJVSD]/.test(v.patron))
    return 'Escoge por lo menos un día de la semana: sin días, la regla no sale nunca.'
  if (!v.desde) return 'Falta la fecha en que empieza.'
  if (!v.hasta) return 'Falta la fecha en que termina.'
  if (v.hasta < v.desde)
    return 'La fecha de fin cae antes que la de inicio. Revísalas: la regla terminaría antes de empezar.'
  return null
}

function EditorDeRegla({
  regla,
  alCerrar,
  alGuardar,
}: {
  regla: Regla | null
  alCerrar: () => void
  alGuardar: () => void
}) {
  const [titulo, setTitulo] = useState(regla?.titulo ?? '')
  const [patron, setPatron] = useState(regla?.patron_de_dias ?? 'LMMJV__')
  const [hora, setHora] = useState(minutosAHhMm(regla?.hora ?? 8 * 60))
  const [slot, setSlot] = useState(String((regla?.duracion_slot_ms ?? 1_800_000) / 60_000))
  const [desde, setDesde] = useState(regla?.fecha_inicio ?? '')
  const [hasta, setHasta] = useState(regla?.fecha_fin ?? '')
  const [episodios, setEpisodios] = useState(String(regla?.episodios_por_corrida ?? 1))
  const [enVivo, setEnVivo] = useState(regla?.tipo === 'vivo')
  const [soloHoy, setSoloHoy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [guardando, setGuardando] = useState(false)

  async function guardar() {
    const problema = validar({ titulo, patron, desde, hasta })
    if (problema) {
      setError(problema)
      return
    }
    setError(null)
    setGuardando(true)
    const cuerpo = {
      tipo: (enVivo ? 'vivo' : 'normal') as Regla['tipo'],
      title_id: regla?.title_id ?? null,
      live_source_id: regla?.live_source_id ?? null,
      titulo,
      patron_de_dias: patron,
      hora: hhMmAMinutos(hora),
      duracion_slot_ms: Number(slot) * 60_000,
      fecha_inicio: desde,
      fecha_fin: hasta,
      episodios_por_corrida: Number(episodios) || 1,
      releva_a: regla?.releva_a ?? null,
      repite_a: regla?.repite_a ?? null,
      activa: true,
    }
    try {
      if (regla) await api.editarRegla(regla.id, cuerpo, soloHoy)
      else await api.crearRegla(cuerpo)
      // La regla sola no mueve nada: el plan se vuelve a armar aquí mismo,
      // para que la parrilla enseñe el cambio sin esperar la corrida del reloj.
      await api.recalcular().catch(() => {})
      alGuardar()
    } catch (e) {
      // El texto del error del servidor ya viene en cristiano (docs/API.md).
      setError(e instanceof ErrorDeApi ? e.message : 'No se pudo guardar la regla.')
    } finally {
      setGuardando(false)
    }
  }

  async function borrar() {
    if (!regla) return
    await api.borrarRegla(regla.id).catch(() => {})
    await api.recalcular().catch(() => {})
    alGuardar()
  }

  return (
    <Panel
      titulo={regla ? regla.titulo : 'Nueva regla'}
      descripcion="Un título, un patrón de días, una hora y dos fechas. De aquí sale la parrilla."
      alCerrar={alCerrar}
      pie={
        <>
          {regla && (
            <button className="boton boton--peligro" onClick={borrar}>
              Borrar
            </button>
          )}
          <button className="boton" onClick={alCerrar}>
            Cancelar
          </button>
          <button className="boton boton--primario" onClick={guardar} disabled={guardando}>
            {guardando ? 'Guardando…' : 'Guardar'}
          </button>
        </>
      }
    >
      {error && <div className="error-en-cristiano">{error}</div>}

      <div className="campo">
        <label htmlFor="r-titulo">Programa</label>
        <input
          id="r-titulo"
          type="text"
          value={titulo}
          onChange={(e) => setTitulo(e.target.value)}
          placeholder="Kojak"
        />
      </div>

      <div className="campo">
        <label>Qué días sale</label>
        <PatronDeDias patron={patron} alCambiar={setPatron} />
        <span className="ayuda">Toca los días. Sin ningún día, la regla no sale nunca.</span>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 14 }}>
        <div className="campo">
          <label htmlFor="r-hora">A qué hora</label>
          <input
            id="r-hora"
            type="time"
            value={hora}
            onChange={(e) => setHora(e.target.value)}
          />
        </div>
        <div className="campo">
          <label htmlFor="r-slot">Cuánto dura</label>
          <select id="r-slot" value={slot} onChange={(e) => setSlot(e.target.value)}>
            <option value="30">media hora</option>
            <option value="60">una hora</option>
            <option value="90">hora y media</option>
            <option value="120">dos horas</option>
            <option value="180">tres horas</option>
            <option value="300">cinco horas</option>
          </select>
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 14 }}>
        <div className="campo">
          <label htmlFor="r-desde">Empieza</label>
          <input
            id="r-desde"
            type="date"
            value={desde}
            onChange={(e) => setDesde(e.target.value)}
          />
        </div>
        <div className="campo">
          <label htmlFor="r-hasta">Termina</label>
          <input
            id="r-hasta"
            type="date"
            value={hasta}
            onChange={(e) => setHasta(e.target.value)}
          />
          <span className="ayuda">De aquí salen los avisos de vencimiento.</span>
        </div>
      </div>

      <div className="campo">
        <label htmlFor="r-eps">Cuántos episodios seguidos</label>
        <input
          id="r-eps"
          type="number"
          min={1}
          value={episodios}
          onChange={(e) => setEpisodios(e.target.value)}
        />
        <span className="ayuda">
          El sistema se acuerda de por dónde iba la serie y sigue desde ahí.
        </span>
      </div>

      <div className="entre">
        <div>
          <div style={{ font: '500 14px var(--sans)' }}>Es una fuente en vivo</div>
          <div className="ayuda">Reserva el tiempo aunque la señal todavía no llegue.</div>
        </div>
        <button
          className="interruptor"
          role="switch"
          aria-checked={enVivo}
          onClick={() => setEnVivo(!enVivo)}
        />
      </div>

      {regla && (
        <div className="entre">
          <div>
            <div style={{ font: '500 14px var(--sans)' }}>¿Solo hoy?</div>
            <div className="ayuda">
              Encendido, se crea una excepción de un día y la regla queda como está.
            </div>
          </div>
          <button
            className="interruptor"
            role="switch"
            aria-checked={soloHoy}
            onClick={() => setSoloHoy(!soloHoy)}
          />
        </div>
      )}
    </Panel>
  )
}

function PanelDeImportacion({
  alCerrar,
  alTerminar,
}: {
  alCerrar: () => void
  alTerminar: () => void
}) {
  const [texto, setTexto] = useState('')
  const [resumen, setResumen] = useState<ResumenDeImportacion | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [trabajando, setTrabajando] = useState(false)

  async function importar() {
    setTrabajando(true)
    setError(null)
    try {
      setResumen(await api.importarHoja(texto))
      // La hoja crea reglas: la parrilla se vuelve a armar de una vez.
      await api.recalcular().catch(() => {})
    } catch (e) {
      setError(e instanceof ErrorDeApi ? e.message : 'No se pudo leer lo que pegaste.')
    } finally {
      setTrabajando(false)
    }
  }

  async function confirmarRelevos() {
    if (!resumen) return
    await api
      .confirmarRelevos(
        resumen.relevos_propuestos.map((r) => ({ regla: r.regla, releva_a: r.releva_a })),
      )
      .catch(() => {})
    // Un relevo cambia quién ocupa la franja: hay que volver a armar el plan.
    await api.recalcular().catch(() => {})
    alTerminar()
  }

  return (
    <Panel
      titulo="Pegar desde Excel o Google Sheets"
      descripcion="Se usa una vez, para traer lo que ya tenías. Nunca se rechaza la hoja entera."
      alCerrar={alCerrar}
      pie={
        resumen ? (
          <>
            <button className="boton" onClick={alTerminar}>
              Listo
            </button>
            {resumen.relevos_propuestos.length > 0 && (
              <button className="boton boton--primario" onClick={confirmarRelevos}>
                Confirmar los relevos
              </button>
            )}
          </>
        ) : (
          <>
            <button className="boton" onClick={alCerrar}>
              Cancelar
            </button>
            <button
              className="boton boton--primario"
              onClick={importar}
              disabled={!texto.trim() || trabajando}
            >
              {trabajando ? 'Leyendo…' : 'Traer lo que sirva'}
            </button>
          </>
        )
      }
    >
      {!resumen && (
        <>
          {error && <div className="error-en-cristiano">{error}</div>}
          <div className="campo">
            <label htmlFor="hoja">Pega aquí las celdas</label>
            <textarea
              id="hoja"
              value={texto}
              onChange={(e) => setTexto(e.target.value)}
              placeholder={'8:00 AM\tKojak\n8:30 AM\tKojak\n9:00 AM\tComics 9th Art'}
            />
            <span className="ayuda">
              Una fila por franja: la hora en la primera celda y el programa al lado.
            </span>
          </div>
        </>
      )}

      {resumen && (
        <>
          <div className="tarjeta" style={{ padding: '16px 18px' }}>
            <div className="rotulo">LO QUE ENTRÓ</div>
            <div className="fila" style={{ gap: 26, marginTop: 12 }}>
              <div>
                <div className="mono" style={{ fontSize: 22 }}>
                  {resumen.reglas_creadas}
                </div>
                <div className="tenue" style={{ fontSize: 12.5 }}>
                  reglas
                </div>
              </div>
              <div>
                <div className="mono" style={{ fontSize: 22 }}>
                  {resumen.titulos_creados}
                </div>
                <div className="tenue" style={{ fontSize: 12.5 }}>
                  programas nuevos
                </div>
              </div>
            </div>
          </div>

          {resumen.fechas_corridas.length > 0 && (
            <div className="tarjeta tarjeta--aviso" style={{ padding: '16px 18px' }}>
              <div className="rotulo">FECHAS QUE SE CORRIERON UN DÍA</div>
              <ul style={{ margin: '10px 0 0', paddingLeft: 18, fontSize: 13, lineHeight: 1.6 }}>
                {resumen.fechas_corridas.map((f) => (
                  <li key={f.fila}>
                    <span className="tenue">fila {f.fila}: </span>
                    {f.texto}
                  </li>
                ))}
              </ul>
            </div>
          )}

          {resumen.filas_con_error.length > 0 && (
            <div className="tarjeta tarjeta--problema" style={{ padding: '16px 18px' }}>
              <div className="rotulo">FILAS QUE NO CUADRARON</div>
              <ul style={{ margin: '10px 0 0', padding: 0, listStyle: 'none', display: 'grid', gap: 9 }}>
                {resumen.filas_con_error.map((f) => (
                  <li key={f.fila} style={{ fontSize: 13 }}>
                    <span className="mono tenue">fila {f.fila}</span>{' '}
                    <span className="mono" style={{ color: 'var(--texto-2)' }}>
                      {f.texto.slice(0, 48)}
                    </span>
                    <div className="rojo" style={{ marginTop: 2 }}>
                      {f.motivo}
                    </div>
                  </li>
                ))}
              </ul>
            </div>
          )}

          {resumen.relevos_propuestos.length > 0 && (
            <div className="tarjeta" style={{ padding: '16px 18px' }}>
              <div className="rotulo">RELEVOS PROPUESTOS</div>
              <p className="subtitulo" style={{ marginTop: 6 }}>
                Una regla empieza justo al día siguiente de que termina otra, en la misma
                franja. Si es un relevo, no es un vencimiento sin reemplazo.
              </p>
              <ul style={{ margin: '12px 0 0', padding: 0, listStyle: 'none', display: 'grid', gap: 8 }}>
                {resumen.relevos_propuestos.map((r) => (
                  <li key={r.regla} style={{ fontSize: 14 }}>
                    {r.texto}
                  </li>
                ))}
              </ul>
            </div>
          )}
        </>
      )}
    </Panel>
  )
}
