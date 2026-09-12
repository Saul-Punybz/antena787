import { useEffect, useState } from 'react'
import { api } from '../lib/api'
import type { AjustesDePreset, Canal, Preset, PresetsDelCanal } from '../lib/tipos'

/**
 * Presets de preparación: cómo quiere el dueño del canal que suene y se vea su
 * material (esquema v10).
 *
 * Se aplican en tres niveles —el canal entero, un programa, o un archivo
 * suelto— y **gana el más específico**. Lo que un nivel no dice lo hereda del
 * de arriba, y eso hay que decirlo en la pantalla: si no, dejar un campo vacío
 * parece que lo pone en cero.
 *
 * El campo delicado es el volumen. En Estados Unidos el CALM Act manda −24
 * LKFS para televisión, así que ese número sale del perfil regulatorio del
 * canal y cambiarlo viene con su aviso. No se prohíbe —una emisora de otro
 * país tiene otro número— pero no se cambia sin saber que se está cambiando.
 */
export function Presets() {
  const [datos, setDatos] = useState<PresetsDelCanal | null>(null)
  const [canal, setCanal] = useState<Canal | null>(null)
  const [editando, setEditando] = useState<Preset | 'nuevo' | null>(null)
  const [aviso, setAviso] = useState('')
  const [error, setError] = useState('')

  function cargar() {
    api
      .presets()
      .then(setDatos)
      .catch(() => setDatos({ presets: [], volumen_lkfs: 0, pico_db: 0, volumen_porque: '' }))
    api.canal().then(setCanal).catch(() => {})
  }
  useEffect(cargar, [])

  const presets = datos?.presets ?? []

  // Aplicar un preset a todo el canal es el nivel de más abajo de los tres, y
  // el único que no tiene otro sitio donde vivir: el del programa se pone en
  // su ficha y el del archivo al lado del archivo.
  async function aplicarAlCanal(id: number | null) {
    const antes = canal
    setError('')
    setCanal((c) => (c ? { ...c, preset_id: id } : c))
    try {
      setCanal(await api.guardarCanal({ preset_id: id }))
      cargar()
    } catch (e) {
      setCanal(antes)
      setError(e instanceof Error ? e.message : 'No se pudo aplicar al canal.')
    }
  }

  return (
    <>
      <div className="encabezado">
        <div>
          <h1 className="titulo-pantalla">Presets de preparación</h1>
          <p className="subtitulo">
            Cómo se prepara el material antes de salir al aire: volumen, recortes y
            calidad.
          </p>
        </div>
        <button className="boton boton--primario" onClick={() => setEditando('nuevo')}>
          Nuevo preset
        </button>
      </div>

      {aviso && (
        <div className="nota nota--aviso" style={{ marginTop: 14 }}>
          {aviso}
        </div>
      )}

      {/* El volumen vigente, que es el número que gobierna todo el canal. */}
      {datos && datos.volumen_lkfs !== 0 && (
        <section className="tarjeta" style={{ padding: '16px 18px', marginTop: 16 }}>
          <div className="rotulo">VOLUMEN DEL CANAL</div>
          <p style={{ font: '600 17px var(--sans)', marginTop: 6 }}>
            {datos.volumen_lkfs}
            <span className="tenue" style={{ font: '400 14px var(--sans)', marginLeft: 10 }}>
              pico máximo {datos.pico_db} dBTP
            </span>
          </p>
          <p className="subtitulo" style={{ marginTop: 6, maxWidth: '70ch' }}>
            {datos.volumen_porque}
          </p>
        </section>
      )}

      {canal && presets.length > 0 && (
        <section className="tarjeta" style={{ padding: '16px 18px', marginTop: 12 }}>
          <div className="rotulo">TODO EL CANAL</div>
          <div className="campo" style={{ marginTop: 10 }}>
            <label htmlFor="preset-canal">Con qué preset se prepara todo</label>
            <EligePreset
              id="preset-canal"
              presets={presets}
              valor={canal.preset_id ?? null}
              heredado="los valores de fábrica"
              alElegir={(id) => void aplicarAlCanal(id)}
            />
            <span className="ayuda">
              Lo que no diga un programa ni un archivo suelto se prepara así.
            </span>
          </div>
          {error && <div className="error-claro">{error}</div>}
        </section>
      )}

      {!datos && <p className="cargando">Cargando…</p>}

      {datos && presets.length === 0 && (
        <div className="tarjeta tarjeta--aviso" style={{ padding: '16px 18px', marginTop: 16 }}>
          Sin ningún preset, todo el material se prepara igual, con los valores de
          fábrica — que es lo correcto para casi todo. Un preset sirve para lo que se
          sale de la norma: un programa que viene siempre bajito de volumen, uno al que
          hay que recortarle la cabecera, o material que ya viene listo y no hay que
          volver a tocar.
        </div>
      )}

      <div style={{ display: 'flex', flexDirection: 'column', gap: 12, marginTop: 18 }}>
        {presets.map((p) => (
          <FilaDePreset
            key={p.id}
            preset={p}
            alEditar={() => setEditando(p)}
            alBorrar={async () => {
              try {
                const r = await api.borrarPreset(p.id)
                setAviso(r.aviso || '')
                cargar()
              } catch (e) {
                setAviso(e instanceof Error ? e.message : 'No se pudo borrar.')
              }
            }}
          />
        ))}
      </div>

      {editando && (
        <EditorDePreset
          preset={editando === 'nuevo' ? null : editando}
          volumenDelCanal={datos?.volumen_lkfs ?? 0}
          porqueElVolumen={datos?.volumen_porque ?? ''}
          alCerrar={() => setEditando(null)}
          alGuardar={() => {
            setEditando(null)
            setAviso('')
            cargar()
          }}
        />
      )}
    </>
  )
}

/** Lo que hace un preset, escrito para leerlo de un vistazo. */
function queHace(a: AjustesDePreset): string[] {
  const dice: string[] = []
  if (a.no_tocar) dice.push('no se toca: sale tal como viene')
  if (a.volumen_relativo_db != null)
    dice.push(`volumen ${a.volumen_relativo_db > 0 ? '+' : ''}${a.volumen_relativo_db} dB`)
  if (a.objetivo_volumen_lkfs != null) dice.push(`se nivela a ${a.objetivo_volumen_lkfs}`)
  if (a.recorte_cabeza_ms) dice.push(`recorta ${(a.recorte_cabeza_ms / 1000).toFixed(1)} s de cabeza`)
  if (a.recorte_cola_ms) dice.push(`recorta ${(a.recorte_cola_ms / 1000).toFixed(1)} s de cola`)
  if (a.calidad) dice.push(`calidad ${a.calidad}`)
  if (a.gop_segundos) dice.push(`cuadro clave cada ${a.gop_segundos} s`)
  if (a.codec_video) dice.push(`video en ${a.codec_video}`)
  if (a.codec_audio) dice.push(`sonido en ${a.codec_audio}`)
  return dice
}

function FilaDePreset({
  preset,
  alEditar,
  alBorrar,
}: {
  preset: Preset
  alEditar: () => void
  alBorrar: () => void
}) {
  const [confirmando, setConfirmando] = useState(false)
  const hace = queHace(preset.ajustes)
  const usos =
    (preset.en_canal ? 1 : 0) + preset.titulos + preset.archivos

  return (
    <section className="tarjeta" style={{ padding: '16px 18px' }}>
      <div className="entre" style={{ gap: 14, flexWrap: 'wrap' }}>
        <div style={{ minWidth: 0 }}>
          <div className="fila" style={{ gap: 10, alignItems: 'baseline', flexWrap: 'wrap' }}>
            <strong style={{ font: '600 16px var(--sans)' }}>{preset.nombre}</strong>
            {preset.en_canal && <span className="etiqueta etiqueta--nota">todo el canal</span>}
            {preset.titulos > 0 && (
              <span className="etiqueta etiqueta--nota">
                {preset.titulos} {preset.titulos === 1 ? 'programa' : 'programas'}
              </span>
            )}
            {preset.archivos > 0 && (
              <span className="etiqueta etiqueta--nota">
                {preset.archivos} {preset.archivos === 1 ? 'archivo' : 'archivos'}
              </span>
            )}
            {usos === 0 && <span className="etiqueta etiqueta--nota">sin usar</span>}
          </div>
          <p className="subtitulo" style={{ marginTop: 5 }}>
            {hace.length ? hace.join(' · ') : 'no cambia nada todavía'}
          </p>
        </div>
        <div className="fila" style={{ gap: 8, flexShrink: 0 }}>
          <button className="boton" onClick={alEditar}>
            Cambiar
          </button>
          {!confirmando ? (
            <button className="boton boton--peligro" onClick={() => setConfirmando(true)}>
              Borrar
            </button>
          ) : (
            <>
              <button className="boton boton--peligro" onClick={alBorrar}>
                Sí, borrar
              </button>
              <button className="boton" onClick={() => setConfirmando(false)}>
                No
              </button>
            </>
          )}
        </div>
      </div>
      {confirmando && usos > 0 && (
        <p className="ambar" style={{ fontSize: 13.5, marginTop: 10 }}>
          Lo usan {usos} {usos === 1 ? 'cosa' : 'cosas'}. Al borrarlo vuelven a prepararse
          como el resto del canal; no se pierde nada del material.
        </p>
      )}
    </section>
  )
}

type Campos = {
  nombre: string
  no_tocar: boolean
  volumen_relativo_db: string
  objetivo_volumen_lkfs: string
  recorte_cabeza_s: string
  recorte_cola_s: string
  calidad: '' | 'alta' | 'normal' | 'baja'
  gop_segundos: string
}

function camposDe(p: Preset | null): Campos {
  const a = p?.ajustes ?? {}
  const s = (n: number | undefined) => (n == null ? '' : String(n))
  return {
    nombre: p?.nombre ?? '',
    no_tocar: a.no_tocar ?? false,
    volumen_relativo_db: s(a.volumen_relativo_db),
    objetivo_volumen_lkfs: s(a.objetivo_volumen_lkfs),
    recorte_cabeza_s: a.recorte_cabeza_ms == null ? '' : String(a.recorte_cabeza_ms / 1000),
    recorte_cola_s: a.recorte_cola_ms == null ? '' : String(a.recorte_cola_ms / 1000),
    calidad: a.calidad ?? '',
    gop_segundos: s(a.gop_segundos),
  }
}

function ajustesDe(c: Campos): AjustesDePreset {
  const a: AjustesDePreset = {}
  // Solo entra lo que se escribió. Un campo vacío **hereda del nivel de
  // arriba**, no vale cero: mandar un cero aquí pisaría el preset del canal
  // sin que nadie lo pidiera.
  if (c.no_tocar) a.no_tocar = true
  if (c.volumen_relativo_db.trim()) a.volumen_relativo_db = Number(c.volumen_relativo_db)
  if (c.objetivo_volumen_lkfs.trim()) a.objetivo_volumen_lkfs = Number(c.objetivo_volumen_lkfs)
  if (c.recorte_cabeza_s.trim()) a.recorte_cabeza_ms = Math.round(Number(c.recorte_cabeza_s) * 1000)
  if (c.recorte_cola_s.trim()) a.recorte_cola_ms = Math.round(Number(c.recorte_cola_s) * 1000)
  if (c.calidad) a.calidad = c.calidad
  if (c.gop_segundos.trim()) a.gop_segundos = Number(c.gop_segundos)
  return a
}

function EditorDePreset({
  preset,
  volumenDelCanal,
  porqueElVolumen,
  alCerrar,
  alGuardar,
}: {
  preset: Preset | null
  volumenDelCanal: number
  porqueElVolumen: string
  alCerrar: () => void
  alGuardar: () => void
}) {
  const [c, setC] = useState<Campos>(camposDe(preset))
  const [error, setError] = useState('')
  const [guardando, setGuardando] = useState(false)

  const set = <K extends keyof Campos>(k: K) => (v: Campos[K]) => setC((x) => ({ ...x, [k]: v }))

  // El aviso del volumen: no impide nada, solo dice lo que se está haciendo.
  const objetivo = c.objetivo_volumen_lkfs.trim() ? Number(c.objetivo_volumen_lkfs) : null
  const avisoVolumen =
    objetivo != null && volumenDelCanal !== 0 && objetivo !== volumenDelCanal
      ? `Estás poniendo ${objetivo} y lo que le corresponde a tu canal es ${volumenDelCanal}. Tú decides; solo que quede dicho.`
      : ''

  async function guardar() {
    setGuardando(true)
    setError('')
    try {
      if (preset) await api.guardarPreset(preset.id, c.nombre, ajustesDe(c))
      else await api.crearPreset(c.nombre, ajustesDe(c))
      alGuardar()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'No se pudo guardar.')
    } finally {
      setGuardando(false)
    }
  }

  return (
    <section className="tarjeta" style={{ padding: '20px 22px', marginTop: 18 }}>
      <div className="rotulo" style={{ marginBottom: 14 }}>
        {preset ? 'CAMBIAR EL PRESET' : 'NUEVO PRESET'}
      </div>

      <div className="campo">
        <label htmlFor="p-nombre">Nombre</label>
        <input
          id="p-nombre"
          value={c.nombre}
          placeholder="Las películas viejas"
          onChange={(e) => set('nombre')(e.target.value)}
        />
        <span className="ayuda">Como lo vas a reconocer cuando se lo apliques a algo.</span>
      </div>

      <div className="campo">
        <label className="fila" style={{ gap: 10, cursor: 'pointer' }}>
          <input
            type="checkbox"
            checked={c.no_tocar}
            onChange={(e) => set('no_tocar')(e.target.checked)}
          />
          <span>No tocar este material: sale tal como viene</span>
        </label>
        <span className="ayuda">
          Para lo que ya llega listo. Volver a comprimir algo que ya está bien solo lo
          puede empeorar, y además ahorra todo el trabajo de prepararlo.
        </span>
      </div>

      {!c.no_tocar && (
        <>
          <div className="fila" style={{ gap: 14, flexWrap: 'wrap', alignItems: 'flex-start' }}>
            <div className="campo" style={{ flex: '1 1 180px' }}>
              <label htmlFor="p-vol">Corrección de volumen (dB)</label>
              <input
                id="p-vol"
                type="number"
                step="0.5"
                value={c.volumen_relativo_db}
                onChange={(e) => set('volumen_relativo_db')(e.target.value)}
              />
              <span className="ayuda">
                Para lo que viene bajito o alto. Se suma a la normalización, no la
                sustituye.
              </span>
            </div>
            <div className="campo" style={{ flex: '1 1 180px' }}>
              <label htmlFor="p-calidad">Calidad</label>
              <select
                id="p-calidad"
                value={c.calidad}
                onChange={(e) => set('calidad')(e.target.value as Campos['calidad'])}
              >
                <option value="">— la de siempre —</option>
                <option value="alta">Alta</option>
                <option value="normal">Normal</option>
                <option value="baja">Baja</option>
              </select>
              <span className="ayuda">Más calidad ocupa más disco.</span>
            </div>
          </div>

          <div className="fila" style={{ gap: 14, flexWrap: 'wrap', alignItems: 'flex-start' }}>
            <div className="campo" style={{ flex: '1 1 180px' }}>
              <label htmlFor="p-cabeza">Recortar de la cabeza (segundos)</label>
              <input
                id="p-cabeza"
                type="number"
                step="0.5"
                min={0}
                value={c.recorte_cabeza_s}
                onChange={(e) => set('recorte_cabeza_s')(e.target.value)}
              />
            </div>
            <div className="campo" style={{ flex: '1 1 180px' }}>
              <label htmlFor="p-cola">Recortar de la cola (segundos)</label>
              <input
                id="p-cola"
                type="number"
                step="0.5"
                min={0}
                value={c.recorte_cola_s}
                onChange={(e) => set('recorte_cola_s')(e.target.value)}
              />
            </div>
          </div>

          <details style={{ marginTop: 14 }}>
            <summary className="subtitulo" style={{ cursor: 'pointer' }}>
              Lo técnico, para quien lo necesite
            </summary>
            <div style={{ marginTop: 12 }}>
              <div className="campo">
                <label htmlFor="p-volumen-objetivo">A qué volumen se nivela todo</label>
                <input
                  id="p-volumen-objetivo"
                  type="number"
                  step="1"
                  placeholder={volumenDelCanal ? String(volumenDelCanal) : ''}
                  value={c.objetivo_volumen_lkfs}
                  onChange={(e) => set('objetivo_volumen_lkfs')(e.target.value)}
                />
                <span className="ayuda">{porqueElVolumen}</span>
                {avisoVolumen && (
                  <p className="ambar" style={{ fontSize: 13.5, marginTop: 8 }}>
                    {avisoVolumen}
                  </p>
                )}
              </div>
              <div className="campo">
                <label htmlFor="p-cuadro-clave">Cuadro clave cada (segundos)</label>
                <input
                  id="p-cuadro-clave"
                  type="number"
                  min={1}
                  max={10}
                  value={c.gop_segundos}
                  onChange={(e) => set('gop_segundos')(e.target.value)}
                />
                <span className="ayuda">
                  Uno es lo de fábrica y es lo que hace posible emitir sin volver a
                  comprimir. Subirlo ahorra disco y quita esa posibilidad.
                </span>
              </div>
            </div>
          </details>
        </>
      )}

      <p className="subtitulo" style={{ marginTop: 16, maxWidth: '72ch' }}>
        Lo que dejes en blanco <strong>se hereda</strong> de lo que valga para todo el
        canal. No se pone en cero.
      </p>

      {error && <div className="error-claro">{error}</div>}

      <div className="fila" style={{ gap: 10, marginTop: 16 }}>
        <button
          className="boton boton--primario"
          onClick={guardar}
          disabled={guardando || !c.nombre.trim()}
        >
          {guardando ? 'Guardando…' : 'Guardar'}
        </button>
        <button className="boton" onClick={alCerrar}>
          Cancelar
        </button>
      </div>
    </section>
  )
}

/**
 * El selector de preset, el mismo en los tres niveles —el canal aquí, el
 * programa en su ficha, el archivo al lado del archivo—.
 *
 * Vive en esta pantalla y no en un archivo aparte porque es parte de los
 * presets: quien venga a cambiar cómo se resuelven los tres niveles tiene que
 * ver el control y la lista en el mismo sitio.
 *
 * Lo que hace distinto: la opción vacía **no dice «ninguno»**. Dice de qué
 * hereda. «Ninguno» haría pensar que no se prepara nada, cuando lo que pasa
 * es que manda el nivel de arriba.
 */
export function EligePreset({
  id,
  presets,
  valor,
  heredado,
  alElegir,
  deshabilitado,
}: {
  id: string
  presets: Preset[]
  valor: number | null
  heredado: string
  alElegir: (id: number | null) => void
  deshabilitado?: boolean
}) {
  return (
    <select
      id={id}
      value={valor ?? ''}
      disabled={deshabilitado}
      onChange={(e) => alElegir(e.target.value === '' ? null : Number(e.target.value))}
    >
      <option value="">— como {heredado} —</option>
      {presets.map((p) => (
        <option key={p.id} value={p.id}>
          {p.nombre}
        </option>
      ))}
    </select>
  )
}
