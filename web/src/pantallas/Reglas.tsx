import { useEffect, useMemo, useRef, useState } from 'react'
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
import {
  ErrorDeApi,
  type DecisionDeEmparejar,
  type Regla,
  type ResumenDeImportacion,
  type TituloDelCatalogo,
  type TituloSinEmparejar,
} from '../lib/tipos'

type Filtro = 'todas' | 'vencen' | 'vivo'

export function Reglas() {
  const { estado } = useEstado()
  const [reglas, setReglas] = useState<Regla[] | null>(null)
  const [filtro, setFiltro] = useState<Filtro>('todas')
  const [editando, setEditando] = useState<Regla | 'nueva' | null>(null)
  const [importando, setImportando] = useState(false)
  const [sinEmparejar, setSinEmparejar] = useState<TituloSinEmparejar[]>([])

  const anio = Number((estado?.dia_emision ?? '2026-01-01').slice(0, 4))

  function cargar() {
    api.reglas().then(setReglas).catch(() => setReglas([]))
  }
  function cargarSinEmparejar() {
    api
      .titulosSinEmparejar()
      .then((xs) => setSinEmparejar(xs ?? []))
      .catch(() => setSinEmparejar([]))
  }
  useEffect(() => {
    cargar()
    cargarSinEmparejar()
  }, [])

  /**
   * Ya se decidió uno: sale de la lista, las reglas cambiaron de título y la
   * parrilla se vuelve a armar aquí mismo, igual que al editar una regla.
   */
  async function tituloResuelto(id: number) {
    setSinEmparejar((xs) => xs.filter((x) => x.id !== id))
    await api.recalcular().catch(() => {})
    cargar()
    cargarSinEmparejar()
  }

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

      {sinEmparejar.length > 0 && (
        <SeccionSinEmparejar titulos={sinEmparejar} alResolver={tituloResuelto} />
      )}

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
          alCambiarTitulos={cargarSinEmparejar}
          alTerminar={() => {
            setImportando(false)
            cargar()
            cargarSinEmparejar()
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

// ── títulos por emparejar (F1-64 a F1-67) ─────────────────────────────

/**
 * Lo que la hoja llamó de una manera y el catálogo de otra. Nunca se adivina
 * (F1-65): la lista se queda ahí hasta que una persona diga cuál es cada uno.
 */
function SeccionSinEmparejar({
  titulos,
  alResolver,
}: {
  titulos: TituloSinEmparejar[]
  alResolver: (id: number) => void
}) {
  return (
    <section>
      <div className="rotulo">TÍTULOS POR EMPAREJAR</div>
      <p className="subtitulo" style={{ marginTop: 5 }}>
        La hoja los trae con un nombre que el catálogo no usa. Las reglas ya entraron; solo
        falta decir cuál es cada uno. El nombre del catálogo manda, y lo que decidas queda
        anotado: la próxima hoja se empareja sola.
      </p>
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fill, minmax(340px, 1fr))',
          gap: 14,
          marginTop: 13,
        }}
      >
        {titulos.map((t) => (
          <TarjetaSinEmparejar key={t.id} titulo={t} alResolver={alResolver} />
        ))}
      </div>
    </section>
  )
}

function cuantasReglas(n: number): string {
  return `${n} ${n === 1 ? 'regla' : 'reglas'}`
}

function TarjetaSinEmparejar({
  titulo,
  alResolver,
}: {
  titulo: TituloSinEmparejar
  alResolver: (id: number) => void
}) {
  const [elegido, setElegido] = useState<{ id: number; nombre: string } | null>(null)
  const [busqueda, setBusqueda] = useState('')
  const [resultados, setResultados] = useState<TituloDelCatalogo[]>([])
  const [buscando, setBuscando] = useState(false)
  const [confirmandoQuitar, setConfirmandoQuitar] = useState(false)
  const [trabajando, setTrabajando] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [hecho, setHecho] = useState<string | null>(null)
  // Una lista vacía puede llegar como null desde el servidor: se pinta igual.
  const candidatos = titulo.candidatos ?? []
  const franjas = titulo.franjas ?? []

  // La tarjeta se va sola cuando ya se decidió, pero primero se lee lo que dijo
  // el servidor: que quedó anotado para la próxima hoja es la mitad del asunto.
  const alResolverRef = useRef(alResolver)
  alResolverRef.current = alResolver
  useEffect(() => {
    if (!hecho) return
    const t = window.setTimeout(() => alResolverRef.current(titulo.id), 2200)
    return () => window.clearTimeout(t)
  }, [hecho, titulo.id])

  // La búsqueda espera a que la persona deje de escribir: una llamada por
  // pausa, no una por tecla.
  useEffect(() => {
    const q = busqueda.trim()
    if (q.length < 2) {
      setResultados([])
      setBuscando(false)
      return
    }
    setBuscando(true)
    const t = window.setTimeout(() => {
      api
        .buscarTitulos(q)
        .then((r) => setResultados(r ?? []))
        .catch(() => setResultados([]))
        .finally(() => setBuscando(false))
    }, 250)
    return () => window.clearTimeout(t)
  }, [busqueda])

  async function decidir(decision: DecisionDeEmparejar) {
    setTrabajando(true)
    setError(null)
    try {
      const r = await api.emparejarTitulo(titulo.id, decision)
      setHecho(r.texto)
    } catch (e) {
      setError(e instanceof ErrorDeApi ? e.message : 'No se pudo guardar la decisión.')
      setTrabajando(false)
    }
  }

  function escoger(id: number, nombre: string) {
    setElegido({ id, nombre })
    setConfirmandoQuitar(false)
  }

  if (hecho) {
    return (
      <div className="tarjeta" style={{ padding: '16px 18px' }}>
        <div className="fila" style={{ alignItems: 'flex-start', gap: 11 }}>
          <span className="punto punto--bien" style={{ marginTop: 5 }} />
          <span style={{ font: '500 14px var(--sans)' }}>{hecho}</span>
        </div>
      </div>
    )
  }

  return (
    <div className="tarjeta tarjeta--aviso" style={{ padding: '16px 18px' }}>
      <div className="entre" style={{ gap: 10 }}>
        <span style={{ font: '600 15px var(--sans)' }}>{titulo.nombre}</span>
        <span className="etiqueta etiqueta--nota">{cuantasReglas(titulo.reglas)}</span>
      </div>
      <p className="subtitulo" style={{ marginTop: 5 }}>
        {titulo.texto}
      </p>
      {franjas.length > 0 && (
        <div className="mono tenue" style={{ fontSize: 12.5, marginTop: 7 }}>
          va en {franjas.join(' · ')}
        </div>
      )}

      {candidatos.length > 0 && (
        <div style={{ marginTop: 15 }}>
          <div className="rotulo">¿ES ALGUNO DE ESTOS?</div>
          <div className="pastillas" style={{ marginTop: 8 }}>
            {candidatos.map((c) => (
              <button
                key={c.id}
                className={'pastilla' + (elegido?.id === c.id ? ' pastilla--activa' : '')}
                onClick={() => escoger(c.id, c.nombre)}
              >
                {c.nombre}
                <span className="tenue" style={{ marginLeft: 7, fontSize: 11.5 }}>
                  {Math.round(c.puntuacion * 100)}%
                </span>
              </button>
            ))}
          </div>
        </div>
      )}

      <div className="campo" style={{ marginTop: 15 }}>
        <label htmlFor={`emparejar-${titulo.id}`}>Buscar en el catálogo</label>
        <input
          id={`emparejar-${titulo.id}`}
          type="text"
          autoComplete="off"
          value={busqueda}
          onChange={(e) => {
            setBusqueda(e.target.value)
            setElegido(null)
          }}
          placeholder="Escribe el nombre de la ficha"
        />
      </div>

      {busqueda.trim().length >= 2 && (
        <div style={{ display: 'grid', gap: 6, marginTop: 9 }}>
          {buscando && <span className="ayuda">Buscando…</span>}
          {!buscando && resultados.length === 0 && (
            <span className="ayuda">
              Ninguna ficha del catálogo se llama así. Si de verdad es nuevo, dale a «Es un
              título nuevo».
            </span>
          )}
          {resultados.map((r) => (
            <button
              key={r.id}
              className={'pastilla' + (elegido?.id === r.id ? ' pastilla--activa' : '')}
              style={{ textAlign: 'left' }}
              onClick={() => escoger(r.id, r.nombre)}
            >
              {r.nombre}
              <span className="tenue" style={{ marginLeft: 8, fontSize: 11.5 }}>
                {r.tipo}
              </span>
            </button>
          ))}
        </div>
      )}

      {elegido && (
        <div className="ayuda" style={{ marginTop: 9 }}>
          Va a quedar como «{elegido.nombre}»: el nombre del catálogo es el que manda.
        </div>
      )}

      {error && (
        <div className="error-en-cristiano" style={{ marginTop: 11 }}>
          {error}
        </div>
      )}

      <div className="fila" style={{ gap: 8, marginTop: 14, flexWrap: 'wrap' }}>
        <button
          className="boton boton--primario"
          disabled={!elegido || trabajando}
          onClick={() => elegido && decidir({ accion: 'usar', title_id: elegido.id })}
        >
          Es este
        </button>
        <button
          className="boton"
          disabled={trabajando}
          onClick={() => decidir({ accion: 'propio' })}
        >
          Es un título nuevo
        </button>
        <button
          className="boton boton--peligro"
          disabled={trabajando}
          onClick={() => setConfirmandoQuitar(true)}
        >
          No es un programa, quitar
        </button>
      </div>

      {confirmandoQuitar && (
        <div className="tarjeta tarjeta--problema" style={{ padding: '13px 15px', marginTop: 11 }}>
          <div style={{ font: '500 13.5px var(--sans)' }}>
            Se van con él {cuantasReglas(titulo.reglas)} de la parrilla
            {franjas.length > 0 ? ` (${franjas.join(' · ')})` : ''}.
          </div>
          <div className="fila" style={{ gap: 8, marginTop: 11 }}>
            <button
              className="boton boton--peligro"
              disabled={trabajando}
              onClick={() => decidir({ accion: 'quitar' })}
            >
              Sí, quitarlo
            </button>
            <button className="boton" onClick={() => setConfirmandoQuitar(false)}>
              Mejor no
            </button>
          </div>
        </div>
      )}
    </div>
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
  alCambiarTitulos,
}: {
  alCerrar: () => void
  alTerminar: () => void
  alCambiarTitulos: () => void
}) {
  const [texto, setTexto] = useState('')
  const [resumen, setResumen] = useState<ResumenDeImportacion | null>(null)
  const [sinEmparejar, setSinEmparejar] = useState<TituloSinEmparejar[]>([])
  const [error, setError] = useState<string | null>(null)
  const [trabajando, setTrabajando] = useState(false)

  async function importar() {
    setTrabajando(true)
    setError(null)
    try {
      const r = await api.importarHoja(texto)
      setResumen(r)
      setSinEmparejar(r.titulos_sin_emparejar ?? [])
      alCambiarTitulos()
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

          {sinEmparejar.length > 0 && (
            <SeccionSinEmparejar
              titulos={sinEmparejar}
              alResolver={(id) => {
                setSinEmparejar((xs) => xs.filter((x) => x.id !== id))
                alCambiarTitulos()
                api.recalcular().catch(() => {})
              }}
            />
          )}

          {(resumen.avisos ?? []).length > 0 && (
            <div className="tarjeta" style={{ padding: '16px 18px' }}>
              <div className="rotulo">NOMBRES QUE SE JUNTARON SOLOS</div>
              <p className="subtitulo" style={{ marginTop: 6 }}>
                La hoja los escribía de otra manera y el catálogo ya los tenía. No hay nada
                que hacer: quedaron en un solo título.
              </p>
              <ul
                style={{ margin: '12px 0 0', paddingLeft: 18, fontSize: 13, lineHeight: 1.6 }}
              >
                {(resumen.avisos ?? []).map((a, i) => (
                  <li key={`${a.titulo}-${i}`}>{a.texto}</li>
                ))}
              </ul>
            </div>
          )}

          {(resumen.posibles_duplicados ?? []).length > 0 && (
            <div className="tarjeta tarjeta--aviso" style={{ padding: '16px 18px' }}>
              <div className="rotulo">DOS NOMBRES QUE PARECEN EL MISMO</div>
              <ul
                style={{ margin: '10px 0 0', padding: 0, listStyle: 'none', display: 'grid', gap: 8 }}
              >
                {(resumen.posibles_duplicados ?? []).map((d, i) => (
                  <li key={`${d.a}-${d.b}-${i}`} style={{ fontSize: 13.5 }}>
                    <span className="mono tenue">
                      {d.a} · {d.b}
                    </span>
                    <div style={{ marginTop: 2 }}>{d.texto}</div>
                  </li>
                ))}
              </ul>
            </div>
          )}

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
