import { useEffect, useState } from 'react'
import { Panel } from '../componentes/Panel'
import { api } from '../lib/api'
import {
  ErrorDeApi,
  type DriverDeSalida,
  type Salida,
  type SalidaNueva,
  type SalidasDelCanal,
} from '../lib/tipos'

// A dónde manda este canal su señal (§10, F2-46, F2-50, F2-114). La API existía
// entera desde T2 y no tenía pantalla: sin esto no había ningún sitio donde
// escribir la dirección del multiplexor de Rolando.
//
// La cadena `sout` exacta de su TP1000 todavía no llegó, así que el
// formulario no propone ni precarga nada: la persona escribe un receptor o
// un grupo multicast con la misma facilidad, y el driver es quien dice si lo
// que escribió se puede abrir.

const DRIVER_UDP_TS = 'udp-ts'
const DRIVER_ARCHIVO = 'archivo'

const ESTADOS: Record<string, { texto: string; clase: string }> = {
  conectada: { texto: 'Conectada', clase: 'punto--bien' },
  reintentando: { texto: 'Reintentando', clase: 'punto--aviso' },
  apagada: { texto: 'Apagada', clase: '' },
  sin_probar: { texto: 'Sin probar todavía', clase: '' },
}

export function Salidas() {
  const [datos, setDatos] = useState<SalidasDelCanal | null>(null)
  const [editando, setEditando] = useState<Salida | 'nueva' | null>(null)
  const [aviso, setAviso] = useState<string | null>(null)

  function cargar() {
    api
      .salidas()
      .then(setDatos)
      .catch(() => setDatos({ salidas: [], drivers_disponibles: [] }))
  }
  useEffect(cargar, [])

  const salidas = datos?.salidas ?? []
  const tipos = datos?.drivers_disponibles ?? []

  return (
    <>
      <div className="encabezado">
        <div>
          <h1 className="titulo-pantalla">Salidas</h1>
          <p className="subtitulo">
            {!datos
              ? 'Cargando las salidas del canal.'
              : salidas.length
                ? `${salidas.length} ${salidas.length === 1 ? 'salida' : 'salidas'}: a dónde manda este canal su señal.`
                : 'Este canal todavía no tiene a dónde mandar su señal.'}
          </p>
        </div>
        <button className="boton boton--primario" onClick={() => setEditando('nueva')}>
          Nueva salida
        </button>
      </div>

      {aviso && (
        <div className="nota nota--aviso" style={{ marginTop: 14 }}>
          {aviso}
        </div>
      )}

      {!datos && <p className="cargando">Cargando…</p>}

      {datos && salidas.length === 0 && (
        <div className="tarjeta tarjeta--aviso" style={{ padding: '16px 18px', marginTop: 16 }}>
          Sin ninguna salida, lo que el canal produce no tiene a dónde ir: se graba en la
          carpeta de datos y ya. Crea una para apuntar al multiplexor o a donde haga falta.
        </div>
      )}

      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fill, minmax(320px, 1fr))',
          gap: 16,
          marginTop: 16,
        }}
      >
        {salidas.map((s) => (
          <TarjetaDeSalida
            key={s.id}
            salida={s}
            alEditar={() => setEditando(s)}
            alBorrada={(av) => {
              setAviso(av || null)
              cargar()
            }}
          />
        ))}
      </div>

      {editando && (
        <EditorDeSalida
          salida={editando === 'nueva' ? null : editando}
          tipos={tipos}
          alCerrar={() => setEditando(null)}
          alGuardar={() => {
            setEditando(null)
            cargar()
          }}
        />
      )}
    </>
  )
}

function TarjetaDeSalida({
  salida,
  alEditar,
  alBorrada,
}: {
  salida: Salida
  alEditar: () => void
  alBorrada: (aviso: string) => void
}) {
  const [confirmando, setConfirmando] = useState(false)
  const [borrando, setBorrando] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const e = ESTADOS[salida.estado_conexion] ?? { texto: salida.estado_conexion, clase: '' }
  const enProblema = salida.estado_conexion === 'reintentando' || salida.estado_conexion === 'apagada'

  async function borrar() {
    setBorrando(true)
    setError(null)
    try {
      const r = await api.borrarSalida(salida.id)
      alBorrada(r.aviso)
    } catch (err) {
      setError(err instanceof ErrorDeApi ? err.message : 'No se pudo borrar la salida.')
      setBorrando(false)
    }
  }

  return (
    <div className="tarjeta" style={{ padding: '16px 18px' }}>
      <div style={{ font: '600 15px var(--sans)' }}>{salida.nombre}</div>
      {salida.texto && (
        <div className="mono tenue" style={{ fontSize: 12.5, marginTop: 5, lineHeight: 1.5 }}>
          {salida.texto}
        </div>
      )}

      <div className="fila" style={{ gap: 8, marginTop: 13 }}>
        <span
          className={'punto ' + e.clase}
          style={e.clase ? undefined : { background: 'var(--gris)' }}
        />
        <span style={{ font: '500 13px var(--sans)' }}>
          {e.texto}
          {salida.estado_conexion === 'reintentando' && salida.reintentos > 0
            ? ` · van ${salida.reintentos} ${salida.reintentos === 1 ? 'intento' : 'intentos'}`
            : ''}
        </span>
      </div>
      {enProblema && salida.ultimo_error && (
        <div className="ayuda" style={{ marginTop: 6, color: 'var(--rojo)' }}>
          {salida.ultimo_error}
        </div>
      )}
      {salida.estado_conexion === 'sin_probar' && (
        <div className="ayuda" style={{ marginTop: 6 }}>
          Todavía no se ha probado esta dirección: se prueba sola en cuanto el canal produzca
          señal.
        </div>
      )}

      {error && (
        <div className="error-en-cristiano" style={{ marginTop: 10 }}>
          {error}
        </div>
      )}

      {!confirmando ? (
        <div className="fila" style={{ gap: 8, marginTop: 14 }}>
          <button className="boton" onClick={alEditar}>
            Cambiar
          </button>
          <button className="boton boton--peligro" onClick={() => setConfirmando(true)}>
            Borrar
          </button>
        </div>
      ) : (
        <div className="tarjeta tarjeta--problema" style={{ padding: '12px 14px', marginTop: 14 }}>
          <div style={{ font: '500 13.5px var(--sans)' }}>
            ¿Borrar «{salida.nombre}»? No se puede deshacer.
          </div>
          <div className="fila" style={{ gap: 8, marginTop: 10 }}>
            <button className="boton boton--peligro" disabled={borrando} onClick={borrar}>
              {borrando ? 'Borrando…' : 'Sí, borrar'}
            </button>
            <button className="boton" disabled={borrando} onClick={() => setConfirmando(false)}>
              Mejor no
            </button>
          </div>
        </div>
      )}
    </div>
  )
}

// ── el formulario ────────────────────────────────────────────────────

/** Lo que trae guardado una salida udp-ts, para precargar el formulario al editar. */
interface CamposUDPTS {
  destino: string
  ttl: string
  bitrateMuxKbs: string
  bitrateVideoKbs: string
  pidVideo: string
  pidAudio: string
  pidPmt: string
  programa: string
  tsid: string
  pcrMs: string
  audio: string
}

const CAMPOS_UDPTS_VACIOS: CamposUDPTS = {
  destino: '',
  ttl: '',
  bitrateMuxKbs: '',
  bitrateVideoKbs: '',
  pidVideo: '',
  pidAudio: '',
  pidPmt: '',
  programa: '',
  tsid: '',
  pcrMs: '',
  audio: '',
}

interface CamposArchivo {
  ruta: string
  bitrateMuxKbs: string
  bitrateVideoKbs: string
  audio: string
}

const CAMPOS_ARCHIVO_VACIOS: CamposArchivo = {
  ruta: '',
  bitrateMuxKbs: '',
  bitrateVideoKbs: '',
  audio: '',
}

/** Lee `Salida.parametros` (el JSON del driver, `internal/model/model.go:133`). */
function parametrosGuardados(salida: Salida | null): Record<string, unknown> {
  if (!salida?.parametros) return {}
  try {
    return JSON.parse(salida.parametros) as Record<string, unknown>
  } catch {
    return {}
  }
}

function texto(v: Record<string, unknown>, clave: string): string {
  const x = v[clave]
  return x === undefined || x === null ? '' : String(x)
}

function camposUDPTSDe(salida: Salida | null): CamposUDPTS {
  if (!salida || salida.driver !== DRIVER_UDP_TS) return { ...CAMPOS_UDPTS_VACIOS }
  const p = parametrosGuardados(salida)
  return {
    destino: texto(p, 'destino'),
    ttl: texto(p, 'ttl'),
    bitrateMuxKbs: texto(p, 'bitrate_mux_kbs'),
    bitrateVideoKbs: texto(p, 'bitrate_video_kbs'),
    pidVideo: texto(p, 'pid_video'),
    pidAudio: texto(p, 'pid_audio'),
    pidPmt: texto(p, 'pid_pmt'),
    programa: texto(p, 'program'),
    tsid: texto(p, 'tsid'),
    pcrMs: texto(p, 'pcr_ms'),
    audio: texto(p, 'audio'),
  }
}

function camposArchivoDe(salida: Salida | null): CamposArchivo {
  if (!salida || salida.driver !== DRIVER_ARCHIVO) return { ...CAMPOS_ARCHIVO_VACIOS }
  const p = parametrosGuardados(salida)
  return {
    ruta: texto(p, 'ruta'),
    bitrateMuxKbs: texto(p, 'bitrate_mux_kbs'),
    bitrateVideoKbs: texto(p, 'bitrate_video_kbs'),
    audio: texto(p, 'audio'),
  }
}

/**
 * Arma `parametros` con solo lo que la persona escribió: un campo en blanco
 * no se manda, y el driver lo rellena con su valor de fábrica
 * (`internal/drivers/salida/udpts.go`, comentario de `ParamsUDPTS`). Así
 * nunca se guarda un número que nadie escribió como si fuera el suyo.
 */
function construirParametrosUDPTS(c: CamposUDPTS): string {
  const p: Record<string, unknown> = { destino: c.destino.trim() }
  if (c.ttl.trim()) p.ttl = Number(c.ttl)
  if (c.bitrateMuxKbs.trim()) p.bitrate_mux_kbs = Number(c.bitrateMuxKbs)
  if (c.bitrateVideoKbs.trim()) p.bitrate_video_kbs = Number(c.bitrateVideoKbs)
  if (c.pidVideo.trim()) p.pid_video = Number(c.pidVideo)
  if (c.pidAudio.trim()) p.pid_audio = Number(c.pidAudio)
  if (c.pidPmt.trim()) p.pid_pmt = Number(c.pidPmt)
  if (c.programa.trim()) p.program = Number(c.programa)
  if (c.tsid.trim()) p.tsid = Number(c.tsid)
  if (c.pcrMs.trim()) p.pcr_ms = Number(c.pcrMs)
  if (c.audio.trim()) p.audio = c.audio
  // El único video que este driver sabe abrir hoy (internal/drivers/salida/udpts.go:valida).
  p.video = 'mpeg2'
  return JSON.stringify(p)
}

function construirParametrosArchivo(c: CamposArchivo): string {
  const p: Record<string, unknown> = { ruta: c.ruta.trim() }
  if (c.bitrateMuxKbs.trim()) p.bitrate_mux_kbs = Number(c.bitrateMuxKbs)
  if (c.bitrateVideoKbs.trim()) p.bitrate_video_kbs = Number(c.bitrateVideoKbs)
  if (c.audio.trim()) p.audio = c.audio
  return JSON.stringify(p)
}

function EditorDeSalida({
  salida,
  tipos,
  alCerrar,
  alGuardar,
}: {
  salida: Salida | null
  tipos: DriverDeSalida[]
  alCerrar: () => void
  alGuardar: () => void
}) {
  const [nombre, setNombre] = useState(salida?.nombre ?? '')
  const [tipo, setTipo] = useState(salida?.driver ?? '')
  const [objetivoVolumen, setObjetivoVolumen] = useState(
    salida?.objetivo_volumen !== undefined ? String(salida.objetivo_volumen) : '',
  )
  const [udpts, setUdpts] = useState<CamposUDPTS>(camposUDPTSDe(salida))
  const [archivo, setArchivo] = useState<CamposArchivo>(camposArchivoDe(salida))
  const [error, setError] = useState<string | null>(null)
  const [guardando, setGuardando] = useState(false)

  const camposListos =
    tipo === DRIVER_UDP_TS
      ? udpts.destino.trim() !== ''
      : tipo === DRIVER_ARCHIVO
        ? archivo.ruta.trim() !== ''
        : false
  const puedeGuardar = nombre.trim() !== '' && tipo !== '' && camposListos && !guardando

  async function guardar() {
    if (!puedeGuardar) return
    setGuardando(true)
    setError(null)
    const cuerpo: SalidaNueva = {
      nombre: nombre.trim(),
      driver: tipo,
      parametros:
        tipo === DRIVER_UDP_TS
          ? construirParametrosUDPTS(udpts)
          : construirParametrosArchivo(archivo),
    }
    if (objetivoVolumen.trim() !== '') cuerpo.objetivo_volumen = Number(objetivoVolumen)
    try {
      if (salida) await api.guardarSalida(salida.id, cuerpo)
      else await api.crearSalida(cuerpo)
      alGuardar()
    } catch (e) {
      setError(e instanceof ErrorDeApi ? e.message : 'No se pudo guardar la salida.')
    } finally {
      setGuardando(false)
    }
  }

  return (
    <Panel
      titulo={salida ? salida.nombre : 'Nueva salida'}
      descripcion="A dónde manda este canal su señal: aquí solo se elige a dónde va y qué espera el equipo del otro lado."
      alCerrar={alCerrar}
      pie={
        <>
          <button className="boton" onClick={alCerrar}>
            Cancelar
          </button>
          <button className="boton boton--primario" disabled={!puedeGuardar} onClick={guardar}>
            {guardando ? 'Guardando…' : 'Guardar'}
          </button>
        </>
      }
    >
      {error && <div className="error-en-cristiano">{error}</div>}

      <div className="campo">
        <label htmlFor="s-nombre">Nombre</label>
        <input
          id="s-nombre"
          type="text"
          value={nombre}
          onChange={(e) => setNombre(e.target.value)}
          placeholder="Transmisor"
        />
        <span className="ayuda">Es el que se ve en Al aire.</span>
      </div>

      <div>
        <label>A dónde va</label>
        <SelectorDeTipo lista={tipos} valor={tipo} alElegir={setTipo} />
      </div>

      {tipo === DRIVER_UDP_TS && <CamposUDPTSForm valor={udpts} alCambiar={setUdpts} />}
      {tipo === DRIVER_ARCHIVO && <CamposArchivoForm valor={archivo} alCambiar={setArchivo} />}

      <div className="campo">
        <label htmlFor="s-volumen">Objetivo de volumen (nivel de entrega)</label>
        <input
          id="s-volumen"
          type="number"
          value={objetivoVolumen}
          onChange={(e) => setObjetivoVolumen(e.target.value)}
          placeholder=""
        />
        <span className="ayuda">
          Cada salida va a su propio objetivo, no a uno compartido (F2-47): al transmisor suele
          pedírsele un nivel de −24, a un destino de internet uno más alto, −16. Déjalo en
          blanco para no tocarlo.
        </span>
      </div>
    </Panel>
  )
}

/** Elegir el tipo de salida en cristiano: nunca la clave sola (PRD §4.3, tipos.ts). */
function SelectorDeTipo({
  lista,
  valor,
  alElegir,
}: {
  lista: DriverDeSalida[]
  valor: string
  alElegir: (v: string) => void
}) {
  if (lista.length === 0) {
    return (
      <p className="ayuda" style={{ color: 'var(--texto-3)' }}>
        El servidor todavía no mandó a dónde se puede mandar la señal.
      </p>
    )
  }
  return (
    <div className="opciones" style={{ marginTop: 8 }}>
      {lista.map((d) => (
        <button
          key={d.driver}
          type="button"
          className={'opcion' + (valor === d.driver ? ' opcion--elegida' : '')}
          aria-pressed={valor === d.driver}
          onClick={() => alElegir(d.driver)}
        >
          <span className="opcion__marca" />
          <span>
            <span className="opcion__texto">{d.nombre}</span>
            <p className="opcion__ayuda">{d.explicacion}</p>
          </span>
        </button>
      ))}
    </div>
  )
}

function CamposUDPTSForm({
  valor,
  alCambiar,
}: {
  valor: CamposUDPTS
  alCambiar: (v: CamposUDPTS) => void
}) {
  const set = (campo: keyof CamposUDPTS) => (v: string) => alCambiar({ ...valor, [campo]: v })
  return (
    <>
      <div className="campo">
        <label htmlFor="s-destino">Dirección y puerto</label>
        <input
          id="s-destino"
          type="text"
          value={valor.destino}
          onChange={(e) => set('destino')(e.target.value)}
          placeholder=""
        />
        <span className="ayuda">
          Un receptor: «192.168.1.50:1234». Un grupo multicast: «239.1.1.1:1234». Escribe lo
          que espere tu multiplexor; los dos se aceptan igual.
        </span>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 14 }}>
        <div className="campo">
          <label htmlFor="s-ttl">Saltos de red (TTL)</label>
          <input
            id="s-ttl"
            type="number"
            min={1}
            max={255}
            value={valor.ttl}
            onChange={(e) => set('ttl')(e.target.value)}
          />
          <span className="ayuda">Solo hace falta para un grupo multicast.</span>
        </div>
        <div className="campo">
          <label htmlFor="s-audio">Sonido</label>
          <select id="s-audio" value={valor.audio} onChange={(e) => set('audio')(e.target.value)}>
            <option value="">— sin elegir —</option>
            <option value="mp2">MPEG capa II (mp2)</option>
            <option value="ac3">AC-3 (ac3)</option>
          </select>
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 14 }}>
        <div className="campo">
          <label htmlFor="s-mux">Tasa total del mux (kb/s)</label>
          <input
            id="s-mux"
            type="number"
            min={1}
            value={valor.bitrateMuxKbs}
            onChange={(e) => set('bitrateMuxKbs')(e.target.value)}
          />
        </div>
        <div className="campo">
          <label htmlFor="s-video-kbs">Tasa de video (kb/s)</label>
          <input
            id="s-video-kbs"
            type="number"
            min={1}
            value={valor.bitrateVideoKbs}
            onChange={(e) => set('bitrateVideoKbs')(e.target.value)}
          />
          <span className="ayuda">Deja al menos 500 kb/s de margen sobre esta tasa.</span>
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 14 }}>
        <div className="campo">
          <label htmlFor="s-pid-video">PID de video</label>
          <input
            id="s-pid-video"
            type="number"
            value={valor.pidVideo}
            onChange={(e) => set('pidVideo')(e.target.value)}
          />
        </div>
        <div className="campo">
          <label htmlFor="s-pid-audio">PID de audio</label>
          <input
            id="s-pid-audio"
            type="number"
            value={valor.pidAudio}
            onChange={(e) => set('pidAudio')(e.target.value)}
          />
        </div>
        <div className="campo">
          <label htmlFor="s-pid-pmt">PID de la tabla (PMT)</label>
          <input
            id="s-pid-pmt"
            type="number"
            value={valor.pidPmt}
            onChange={(e) => set('pidPmt')(e.target.value)}
          />
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 14 }}>
        <div className="campo">
          <label htmlFor="s-programa">Número de programa</label>
          <input
            id="s-programa"
            type="number"
            min={1}
            max={65535}
            value={valor.programa}
            onChange={(e) => set('programa')(e.target.value)}
          />
        </div>
        <div className="campo">
          <label htmlFor="s-tsid">Identificador de la señal (TSID)</label>
          <input
            id="s-tsid"
            type="number"
            min={1}
            max={65535}
            value={valor.tsid}
            onChange={(e) => set('tsid')(e.target.value)}
          />
        </div>
        <div className="campo">
          <label htmlFor="s-pcr">PCR (ms entre relojes)</label>
          <input
            id="s-pcr"
            type="number"
            min={0}
            value={valor.pcrMs}
            onChange={(e) => set('pcrMs')(e.target.value)}
          />
          <span className="ayuda">Un multiplexor descarta lo que llega más tarde.</span>
        </div>
      </div>

      <p className="ayuda">
        Video: MPEG-2, el único formato que un multiplexor ATSC 1.0 entiende hoy. Lo que dejes
        en blanco arriba lo rellena el sistema con su valor de fábrica.
      </p>
    </>
  )
}

function CamposArchivoForm({
  valor,
  alCambiar,
}: {
  valor: CamposArchivo
  alCambiar: (v: CamposArchivo) => void
}) {
  const set = (campo: keyof CamposArchivo) => (v: string) => alCambiar({ ...valor, [campo]: v })
  return (
    <>
      <div className="campo">
        <label htmlFor="s-ruta">Ruta del archivo</label>
        <input
          id="s-ruta"
          type="text"
          value={valor.ruta}
          onChange={(e) => set('ruta')(e.target.value)}
          placeholder=""
        />
        <span className="ayuda">La carpeta se crea sola si todavía no existe.</span>
      </div>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 14 }}>
        <div className="campo">
          <label htmlFor="s-a-mux">Tasa total del mux (kb/s)</label>
          <input
            id="s-a-mux"
            type="number"
            min={1}
            value={valor.bitrateMuxKbs}
            onChange={(e) => set('bitrateMuxKbs')(e.target.value)}
          />
        </div>
        <div className="campo">
          <label htmlFor="s-a-video">Tasa de video (kb/s)</label>
          <input
            id="s-a-video"
            type="number"
            min={1}
            value={valor.bitrateVideoKbs}
            onChange={(e) => set('bitrateVideoKbs')(e.target.value)}
          />
        </div>
      </div>
      <div className="campo">
        <label htmlFor="s-a-audio">Sonido</label>
        <select id="s-a-audio" value={valor.audio} onChange={(e) => set('audio')(e.target.value)}>
          <option value="">— sin elegir —</option>
          <option value="mp2">MPEG capa II (mp2)</option>
          <option value="ac3">AC-3 (ac3)</option>
        </select>
      </div>
    </>
  )
}
