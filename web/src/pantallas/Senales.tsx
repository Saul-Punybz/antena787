import { useEffect, useState } from 'react'
import { api } from '../lib/api'
import type {
  Fuente,
  FuenteNueva,
  FuentesDelCanal,
  PruebaDeFuente,
  TipoDeFuente,
  TipoDeFuenteDisponible,
} from '../lib/tipos'

/**
 * Señales en vivo: de dónde sale lo que no es un archivo (F2-116).
 *
 * La pantalla se organiza por **quién llama a quién**, no por protocolo: hay
 * señales que se van a buscar y señales que se esperan. Es la distinción que
 * de verdad cambia lo que la persona tiene que hacer —una pide una dirección
 * a la que salir, la otra pide abrir un puerto y decírselo a alguien— y es
 * también la que separa las que pueden necesitar clave.
 *
 * El botón de **probar antes de guardar** es el centro de esto. De todos los
 * sistemas que se compararon, ninguno lo tiene: lo más cercano prueba después
 * (docs/investigacion/ENTRADAS-POR-URL-COMPARADAS-2026-09-11.md). Para quien
 * es su propio departamento de IT, es la diferencia entre pegar una dirección
 * y rezar, o saber en tres segundos qué hay al otro lado.
 */
export function Senales() {
  const [datos, setDatos] = useState<FuentesDelCanal | null>(null)
  const [editando, setEditando] = useState<Fuente | 'nueva' | null>(null)
  const [aviso, setAviso] = useState('')

  function cargar() {
    api
      .fuentes()
      .then(setDatos)
      .catch(() => setDatos({ fuentes: [], tipos: [] }))
  }
  useEffect(cargar, [])

  const fuentes = datos?.fuentes ?? []
  const tipos = datos?.tipos ?? []

  return (
    <>
      <div className="encabezado">
        <div>
          <h1 className="titulo-pantalla">Señales en vivo</h1>
          <p className="subtitulo">
            {!datos
              ? 'Cargando las señales del canal.'
              : fuentes.length
                ? `${fuentes.length} ${fuentes.length === 1 ? 'señal' : 'señales'}: de dónde sale lo que no es un archivo.`
                : 'Todavía no hay ninguna señal en vivo configurada.'}
          </p>
        </div>
        <button className="boton boton--primario" onClick={() => setEditando('nueva')}>
          Nueva señal
        </button>
      </div>

      {aviso && (
        <div className="nota nota--aviso" style={{ marginTop: 14 }}>
          {aviso}
        </div>
      )}

      {!datos && <p className="cargando">Cargando…</p>}

      {datos && fuentes.length === 0 && (
        <div className="tarjeta tarjeta--aviso" style={{ padding: '16px 18px', marginTop: 16 }}>
          Una señal en vivo es un programa que no sale de un archivo: un stream que ya
          existe en otro sitio, una cámara conectada, o alguien que te empuja la señal
          desde fuera. Una vez creada, la programas como cualquier otra cosa.
        </div>
      )}

      <div style={{ display: 'flex', flexDirection: 'column', gap: 12, marginTop: 18 }}>
        {fuentes.map((f) => (
          <FilaDeSenal
            key={f.id}
            fuente={f}
            alEditar={() => setEditando(f)}
            alBorrar={async () => {
              try {
                await api.borrarFuente(f.id)
                setAviso('')
                cargar()
              } catch (e) {
                setAviso(e instanceof Error ? e.message : 'No se pudo borrar.')
              }
            }}
          />
        ))}
      </div>

      {editando && (
        <EditorDeSenal
          fuente={editando === 'nueva' ? null : editando}
          tipos={tipos}
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

function FilaDeSenal({
  fuente,
  alEditar,
  alBorrar,
}: {
  fuente: Fuente
  alEditar: () => void
  alBorrar: () => void
}) {
  const [confirmando, setConfirmando] = useState(false)
  const enUso = fuente.reglas > 0 || fuente.bloques > 0

  return (
    <section className="tarjeta" style={{ padding: '16px 18px' }}>
      <div className="entre" style={{ gap: 14, flexWrap: 'wrap' }}>
        <div style={{ minWidth: 0 }}>
          <div className="fila" style={{ gap: 10, alignItems: 'baseline', flexWrap: 'wrap' }}>
            <strong style={{ font: '600 16px var(--sans)' }}>{fuente.nombre}</strong>
            <span className="etiqueta etiqueta--nota">
              {fuente.se_va_a_buscar ? 'se va a buscar' : 'se espera'}
            </span>
            {fuente.solo_audio && <span className="etiqueta etiqueta--nota">solo audio</span>}
            {fuente.tiene_clave && <span className="etiqueta etiqueta--nota">con clave</span>}
          </div>
          <p className="subtitulo" style={{ marginTop: 5 }}>
            {fuente.texto}
          </p>
          {enUso && (
            <p className="subtitulo" style={{ marginTop: 4 }}>
              La usan {fuente.reglas} {fuente.reglas === 1 ? 'regla' : 'reglas'}
              {fuente.bloques > 0 &&
                ` y ${fuente.bloques} ${
                  fuente.bloques === 1
                    ? 'bloque que todavía no salió'
                    : 'bloques que todavía no salieron'
                }`}
              .
            </p>
          )}
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
    </section>
  )
}

const VACIA: FuenteNueva = {
  nombre: '',
  tipo: 'url',
  direccion: '',
  solo_audio: false,
  usuario: '',
  clave: '',
}

function EditorDeSenal({
  fuente,
  tipos,
  alCerrar,
  alGuardar,
}: {
  fuente: Fuente | null
  tipos: TipoDeFuenteDisponible[]
  alCerrar: () => void
  alGuardar: () => void
}) {
  const [campos, setCampos] = useState<FuenteNueva>(
    fuente
      ? {
          nombre: fuente.nombre,
          tipo: fuente.tipo,
          direccion: fuente.direccion,
          solo_audio: fuente.solo_audio,
          retardo_ms: fuente.retardo_ms,
          gracia_s: fuente.gracia_s,
          reloj_de_cortes: fuente.reloj_de_cortes,
          usuario: fuente.usuario,
          clave: '',
        }
      : VACIA,
  )
  const [prueba, setPrueba] = useState<PruebaDeFuente | null>(null)
  const [probando, setProbando] = useState(false)
  const [error, setError] = useState('')
  const [guardando, setGuardando] = useState(false)

  const elTipo = tipos.find((t) => t.tipo === campos.tipo)

  function cambiar<K extends keyof FuenteNueva>(k: K, v: FuenteNueva[K]) {
    setCampos((c) => ({ ...c, [k]: v }))
    // Lo probado deja de valer en cuanto se cambia algo: enseñar un resultado
    // viejo junto a una dirección nueva es peor que no enseñar nada.
    if (k === 'direccion' || k === 'tipo' || k === 'usuario' || k === 'clave') setPrueba(null)
  }

  async function probar() {
    setProbando(true)
    setPrueba(null)
    try {
      setPrueba(await api.probarFuente(campos))
    } catch (e) {
      setPrueba({ responde: false, texto: e instanceof Error ? e.message : 'No se pudo probar.' })
    } finally {
      setProbando(false)
    }
  }

  async function guardar() {
    setGuardando(true)
    setError('')
    try {
      if (fuente) await api.guardarFuente(fuente.id, campos)
      else await api.crearFuente(campos)
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
        {fuente ? 'CAMBIAR LA SEÑAL' : 'NUEVA SEÑAL'}
      </div>

      <div className="campo">
        <label htmlFor="se-nombre">Nombre</label>
        <input
          id="se-nombre"
          value={campos.nombre}
          placeholder="RadioOnce Live!"
          onChange={(e) => cambiar('nombre', e.target.value)}
        />
        <span className="ayuda">Como la vas a reconocer cuando la programes.</span>
      </div>

      <div className="campo">
        <label htmlFor="se-tipo">Cómo llega</label>
        <select
          id="se-tipo"
          value={campos.tipo}
          onChange={(e) => cambiar('tipo', e.target.value as TipoDeFuente)}
        >
          {tipos.map((t) => (
            <option key={t.tipo} value={t.tipo}>
              {t.nombre}
            </option>
          ))}
        </select>
        {elTipo && <span className="ayuda">{elTipo.explicacion}</span>}
      </div>

      <div className="campo">
        <label htmlFor="se-direccion">
          {elTipo?.se_va_a_buscar ? 'De dónde se trae' : 'Dónde se espera'}
        </label>
        <input
          id="se-direccion"
          value={campos.direccion}
          placeholder={elTipo?.ejemplo ?? ''}
          onChange={(e) => cambiar('direccion', e.target.value)}
        />
        {elTipo && <span className="ayuda">Por ejemplo: {elTipo.ejemplo}</span>}
      </div>

      {elTipo?.se_va_a_buscar && (
        <div className="fila" style={{ gap: 14, alignItems: 'flex-start', flexWrap: 'wrap' }}>
          <div className="campo" style={{ flex: '1 1 200px' }}>
            <label htmlFor="se-usuario">Usuario</label>
            <input
              id="se-usuario"
              value={campos.usuario ?? ''}
              onChange={(e) => cambiar('usuario', e.target.value)}
            />
            <span className="ayuda">Si el sitio no pide, déjalo en blanco.</span>
          </div>
          <div className="campo" style={{ flex: '1 1 200px' }}>
            <label htmlFor="se-clave">Clave</label>
            <input
              id="se-clave"
              type="password"
              value={campos.clave ?? ''}
              placeholder={fuente?.tiene_clave ? 'hay una guardada' : ''}
              onChange={(e) => cambiar('clave', e.target.value)}
            />
            <span className="ayuda">
              {fuente?.tiene_clave
                ? 'Se guarda cifrada y no se puede volver a leer. Déjalo en blanco para conservar la que hay.'
                : 'Se guarda cifrada, fuera de la base de datos.'}
            </span>
          </div>
        </div>
      )}

      {/* Probar antes de guardar: lo que ningún otro sistema hace. */}
      <div
        style={{
          marginTop: 16,
          padding: '14px 16px',
          border: '1px solid var(--borde)',
          borderRadius: 8,
        }}
      >
        <div className="entre" style={{ gap: 12, flexWrap: 'wrap' }}>
          <div>
            <strong style={{ font: '600 14.5px var(--sans)' }}>Probar antes de guardar</strong>
            <p className="subtitulo" style={{ marginTop: 3 }}>
              Se conecta de verdad y te dice qué hay al otro lado. No guarda nada.
            </p>
          </div>
          <button
            className="boton"
            onClick={probar}
            disabled={probando || !campos.direccion.trim()}
          >
            {probando ? 'Probando…' : 'Probar'}
          </button>
        </div>

        {prueba && (
          <div style={{ marginTop: 12 }}>
            <p className={prueba.responde ? 'verde' : 'ambar'} style={{ font: '600 14.5px var(--sans)' }}>
              {prueba.responde ? '✓ ' : '✗ '}
              {prueba.texto}
            </p>
            {(prueba.video || prueba.audio || prueba.duracion) && (
              <div className="fila" style={{ gap: 18, marginTop: 8, flexWrap: 'wrap' }}>
                {prueba.video && <span className="mono tenue">imagen: {prueba.video}</span>}
                {prueba.audio && <span className="mono tenue">sonido: {prueba.audio}</span>}
                {prueba.duracion && <span className="mono tenue">{prueba.duracion}</span>}
              </div>
            )}
            {prueba.avisos?.map((a) => (
              <p key={a} className="ambar" style={{ fontSize: 13.5, marginTop: 6 }}>
                {a}
              </p>
            ))}
            {prueba.detalle && (
              <details style={{ marginTop: 8 }}>
                <summary className="subtitulo" style={{ cursor: 'pointer' }}>
                  Lo que dijo el sistema
                </summary>
                <p className="mono tenue" style={{ fontSize: 12.5, marginTop: 6 }}>
                  {prueba.detalle}
                </p>
              </details>
            )}
          </div>
        )}
      </div>

      {error && <div className="error-claro">{error}</div>}

      <div className="fila" style={{ gap: 10, marginTop: 18 }}>
        <button className="boton boton--primario" onClick={guardar} disabled={guardando}>
          {guardando ? 'Guardando…' : 'Guardar'}
        </button>
        <button className="boton" onClick={alCerrar}>
          Cancelar
        </button>
        {/* Se puede guardar sin probar a propósito: puede que la señal todavía
            no esté encendida al otro lado, y eso no es razón para no dejar
            dejarla configurada. */}
        {!prueba && <span className="subtitulo">Puedes guardar sin probar.</span>}
      </div>
    </section>
  )
}
