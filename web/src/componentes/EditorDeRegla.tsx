import { useState } from 'react'
import { Panel } from './Panel'
import { PatronDeDias } from './PatronDeDias'
import { api } from '../lib/api'
import { diaSemanaDe, hhMmAMinutos, minutosAHhMm } from '../lib/fechas'
import { ErrorDeApi, type Regla } from '../lib/tipos'

// El formulario de una regla. Vive aquí y no en Reglas porque la Parrilla lo
// abre también: desde un hueco, con el día y la hora ya puestos. Todo lo nuevo
// entra por una regla —la parrilla es consecuencia de las reglas, no una hoja
// de celdas (PRD §9)—, así que este es el único sitio donde se escribe una.

const LETRAS = 'LMMJVSD'

/** Los valores con que otra pantalla abre una regla nueva. */
export interface InicialDeRegla {
  titulo?: string
  patron_de_dias?: string
  /** Minutos desde medianoche, como `Regla.hora`. */
  hora?: number
  duracion_slot_ms?: number
  fecha_inicio?: string
  /** De dónde salieron estos valores, para que se lea en el panel. */
  nota?: string
}

/** El patrón de un solo día: "___J___" para un jueves. */
export function patronDeUnDia(dia: string): string {
  // El patrón empieza en lunes; `diaSemanaDe` empieza en domingo.
  const i = (diaSemanaDe(dia) + 6) % 7
  return '_'.repeat(i) + LETRAS[i] + '_'.repeat(6 - i)
}

/** La opción de duración más grande que cabe en los minutos que hay libres. */
function duracionQueCabe(minutos: number): number {
  const opciones = [300, 180, 120, 90, 60, 30]
  const cabe = opciones.find((o) => o <= minutos)
  return (cabe ?? 30) * 60_000
}

/**
 * Un hueco de la parrilla → los valores de la regla que lo llenaría: ese día
 * de la semana, esa hora, y lo más largo que quepa antes de lo siguiente. La
 * fecha de fin se deja en blanco a propósito: de ella salen los avisos de
 * vencimiento y nadie más que el programador sabe hasta cuándo tiene el
 * programa.
 */
export function reglaDesdeHueco(v: {
  dia: string
  desde: number
  hasta?: number
  titulo?: string
  nota?: string
}): InicialDeRegla {
  return {
    titulo: v.titulo,
    patron_de_dias: patronDeUnDia(v.dia),
    hora: v.desde,
    duracion_slot_ms: v.hasta ? duracionQueCabe(v.hasta - v.desde) : undefined,
    fecha_inicio: v.dia,
    nota: v.nota,
  }
}

/** Validación en palabras claras: la local y la que devuelva el servidor. */
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

export function EditorDeRegla({
  regla,
  inicial,
  alCerrar,
  alGuardar,
}: {
  regla: Regla | null
  /** Con qué abre una regla nueva. Se ignora al editar una que ya existe. */
  inicial?: InicialDeRegla
  alCerrar: () => void
  alGuardar: () => void
}) {
  const de = regla ? undefined : inicial
  const [titulo, setTitulo] = useState(regla?.titulo ?? de?.titulo ?? '')
  const [patron, setPatron] = useState(
    regla?.patron_de_dias ?? de?.patron_de_dias ?? 'LMMJV__',
  )
  const [hora, setHora] = useState(minutosAHhMm(regla?.hora ?? de?.hora ?? 8 * 60))
  const [slot, setSlot] = useState(
    String((regla?.duracion_slot_ms ?? de?.duracion_slot_ms ?? 1_800_000) / 60_000),
  )
  const [desde, setDesde] = useState(regla?.fecha_inicio ?? de?.fecha_inicio ?? '')
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
      // El texto del error del servidor ya viene en palabras claras (docs/API.md).
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
      {error && <div className="error-claro">{error}</div>}

      {de?.nota && <div className="nota">{de.nota}</div>}

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
          aria-label="Es una fuente en vivo"
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
            aria-label="Solo hoy"
            onClick={() => setSoloHoy(!soloHoy)}
          />
        </div>
      )}
    </Panel>
  )
}
