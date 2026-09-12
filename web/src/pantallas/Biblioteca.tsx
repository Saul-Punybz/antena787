import { useEffect, useMemo, useRef, useState } from 'react'
import { Caratula } from '../componentes/Caratula'
import { Panel } from '../componentes/Panel'
import { IconoBuscar } from '../componentes/Iconos'
import { EligePreset } from './Presets'
import { api, suscribirseAEventos } from '../lib/api'
import { nombreDeArchivo, nombreDePista } from '../lib/audio'
import { duracionLarga, fechaDeRegla, minutosAHora12, hhMmAMinutos } from '../lib/fechas'
import { useEstado } from '../lib/estado'
import type {
  AudioDelMaterial,
  EnCuarentena,
  EpisodioDeBiblioteca,
  EstadoMaterial,
  FichaDeTitulo,
  MaterialDeAudio,
  Preset,
  TituloDeBiblioteca,
  ArchivoEntrando,
} from '../lib/tipos'

export function Biblioteca() {
  const { estado } = useEstado()
  const anio = Number((estado?.dia_emision ?? '2026-01-01').slice(0, 4))
  const [titulos, setTitulos] = useState<TituloDeBiblioteca[] | null>(null)
  const [cuarentena, setCuarentena] = useState<EnCuarentena[]>([])
  const [entrando, setEntrando] = useState<ArchivoEntrando[]>([])
  // Los presets se cargan aquí y bajan a la ficha: son pocos, cambian poco, y
  // pedirlos otra vez cada vez que se abre un título sería pedirlos de más.
  const [presets, setPresets] = useState<Preset[]>([])
  const [busqueda, setBusqueda] = useState('')
  const [ficha, setFicha] = useState<FichaDeTitulo | null>(null)
  const [dejarPasar, setDejarPasar] = useState<EnCuarentena | null>(null)
  const [arrastrando, setArrastrando] = useState(false)
  const [subiendo, setSubiendo] = useState('')
  const entrada = useRef<HTMLInputElement>(null)

  function cargar() {
    api.biblioteca().then(setTitulos).catch(() => setTitulos([]))
    api.cuarentena().then(setCuarentena).catch(() => setCuarentena([]))
    api.entrando().then(setEntrando).catch(() => setEntrando([]))
    api
      .presets()
      .then((p) => setPresets(p.presets))
      .catch(() => setPresets([]))
  }
  useEffect(() => {
    cargar()
    // Cada archivo que entra o termina de medirse avisa por el WebSocket:
    // la pantalla se refresca sola, sin que nadie recargue.
    return suscribirseAEventos((e) => {
      if (e.clase === 'ingest' || e.clase === 'material') cargar()
    })
  }, [])

  const filtrados = useMemo(
    () =>
      (titulos ?? []).filter((t) =>
        t.nombre.toLowerCase().includes(busqueda.trim().toLowerCase()),
      ),
    [titulos, busqueda],
  )
  const enParrilla = filtrados
    .filter((t) => t.en_la_parrilla)
    .sort((a, b) => hhMmAMinutos(a.hora ?? '00:00') - hhMmAMinutos(b.hora ?? '00:00'))
  const sinProgramar = filtrados.filter((t) => !t.en_la_parrilla)
  const alAire = estado?.al_aire?.titulo
  const destacado =
    (alAire ? enParrilla.find((t) => t.nombre === alAire) : undefined) ?? enParrilla[0]

  async function subir(archivos: FileList | File[] | null) {
    if (!archivos || archivos.length === 0) return
    const lista = Array.from(archivos)
    setSubiendo(`Subiendo ${lista.length} archivo${lista.length === 1 ? '' : 's'}…`)
    try {
      await api.subirMaterial(lista)
      setSubiendo(
        `${lista.length} archivo${lista.length === 1 ? '' : 's'} en camino. El sistema los mide y los prepara solo; te avisa si algo no sirve.`,
      )
    } catch {
      setSubiendo('No se pudo subir. Vuelve a intentarlo.')
    }
  }

  return (
    <>
      <div className="encabezado">
        <div
          className="tarjeta crece"
          style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '12px 18px' }}
        >
          <IconoBuscar tamano={18} color="var(--texto-3)" />
          <input
            value={busqueda}
            onChange={(e) => setBusqueda(e.target.value)}
            placeholder={`Buscar en ${titulos?.length ?? 0} títulos…`}
            style={{
              background: 'none',
              border: 0,
              outline: 'none',
              color: 'var(--texto)',
              font: '400 15px var(--sans)',
              width: '100%',
            }}
          />
        </div>
        <div className="franja-modo franja-modo--aire" style={{ flexShrink: 0 }}>
          <span className="punto punto--bien" />
          {titulos?.length ?? 0} títulos · importados de la carpeta una vez
        </div>
        {/*
          El botón de añadir. Estaba solo como zona de arrastre allá abajo, y
          una zona de arrastre no se ve: quien no sabe que puede arrastrar no
          descubre nunca que también se puede hacer clic. Aquí arriba, con el
          signo delante y diciendo que busca en la computadora.
        */}
        <button
          className="boton boton--primario"
          onClick={() => entrada.current?.click()}
          style={{ flexShrink: 0, display: 'flex', alignItems: 'center', gap: 8 }}
        >
          <span style={{ font: '400 19px var(--sans)', lineHeight: 1 }}>+</span>
          Añadir contenido
        </button>
      </div>

      {/* La ficha grande de arriba, como los mockups */}
      {destacado && (
        <button
          className="tarjeta"
          onClick={() => api.titulo(destacado.id).then(setFicha)}
          style={{
            padding: '30px 34px',
            textAlign: 'left',
            cursor: 'pointer',
            color: 'inherit',
            background:
              'linear-gradient(100deg, var(--superficie) 40%, rgba(34,211,238,.07) 100%)',
            border: '1px solid var(--borde)',
          }}
        >
          <div className="rotulo aqua" style={{ color: 'var(--aqua)' }}>
            AHORA EN LA PARRILLA
          </div>
          <h2 style={{ font: '700 34px var(--sans)', margin: '10px 0 0', letterSpacing: '-0.6px' }}>
            {destacado.nombre}
          </h2>
          <p
            className="subtitulo"
            style={{ maxWidth: 640, fontSize: 14.5, lineHeight: 1.55, marginTop: 10 }}
          >
            {destacado.sinopsis}
          </p>
          <div className="fila" style={{ gap: 16, marginTop: 16, fontSize: 13.5 }}>
            {destacado.anio && <span className="mono tenue">{destacado.anio}</span>}
            <span className="etiqueta etiqueta--nota">{destacado.clasificacion_contenido}</span>
            <span className="mono tenue">
              {destacado.episodios} episodios · {duracionLarga(destacado.duracion_ms)}
            </span>
            {destacado.regla_hasta && (
              <span className="fila verde" style={{ gap: 7 }}>
                <i className="punto punto--bien" />
                regla hasta {fechaDeRegla(destacado.regla_hasta, anio)}
              </span>
            )}
          </div>
        </button>
      )}

      <Estante
        titulo="En la parrilla esta semana"
        nota={`${enParrilla.length} títulos`}
        titulos={enParrilla}
        alAbrir={(t) => api.titulo(t.id).then(setFicha)}
      />
      <Estante
        titulo="Sin programar"
        nota={`${sinProgramar.length} títulos que no estás usando`}
        notaAmbar
        titulos={sinProgramar}
        alAbrir={(t) => api.titulo(t.id).then(setFicha)}
      />

      {/* Arrastrar archivos */}
      <div
        onDragOver={(e) => {
          e.preventDefault()
          setArrastrando(true)
        }}
        onDragLeave={() => setArrastrando(false)}
        onDrop={(e) => {
          e.preventDefault()
          setArrastrando(false)
          void subir(e.dataTransfer.files)
        }}
        onClick={() => entrada.current?.click()}
        style={{
          border: `1px dashed ${arrastrando ? 'var(--aqua)' : 'var(--borde)'}`,
          background: arrastrando ? 'rgba(34,211,238,.06)' : 'transparent',
          borderRadius: 10,
          padding: '26px 24px',
          textAlign: 'center',
          cursor: 'pointer',
        }}
      >
        <div style={{ font: '600 15px var(--sans)' }}>
          Arrastra videos aquí, o haz clic para buscarlos en tu computadora
        </div>
        <p className="subtitulo">
          Caen en la carpeta vigilada. El sistema los mide, los prepara y te dice en palabras
          claras si alguno no sirve.
        </p>
        {subiendo && (
          <p className="aqua" style={{ fontSize: 13, marginTop: 10 }}>
            {subiendo}
          </p>
        )}
        <input
          ref={entrada}
          type="file"
          multiple
          hidden
          onChange={(e) => void subir(e.target.files)}
        />
      </div>

      {/* Entrando: se ve desde el primer segundo, mientras se mide */}
      {entrando.length > 0 && (
        <section className="tarjeta" style={{ padding: '18px 20px' }}>
          <div className="entre">
            <div>
              <div className="rotulo">ENTRANDO</div>
              <p className="subtitulo" style={{ marginTop: 5 }}>
                Se están midiendo. Pasan a la biblioteca en cuanto terminan; una
                película tarda unos minutos.
              </p>
            </div>
            <span className="ambar" style={{ fontSize: 13 }}>
              {entrando.length === 1 ? '1 archivo' : `${entrando.length} archivos`}
            </span>
          </div>
          <ul style={{ listStyle: 'none', margin: '12px 0 0', padding: 0, display: 'grid', gap: 6 }}>
            {entrando.map((a) => (
              <li key={a.id} style={{ font: '400 13.5px var(--sans)' }}>
                {nombreDeArchivo(a.ruta)}
              </li>
            ))}
          </ul>
        </section>
      )}

      {/* Cuarentena */}
      <section className="tarjeta" style={{ padding: '18px 20px' }}>
        <div className="entre">
          <div>
            <div className="rotulo">CUARENTENA</div>
            <p className="subtitulo" style={{ marginTop: 5 }}>
              Nada de esto llega al aire hasta que una persona lo deje pasar bajo su
              nombre.
            </p>
          </div>
          <span className={cuarentena.length ? 'ambar' : 'verde'} style={{ fontSize: 13 }}>
            {cuarentena.length
              ? `${cuarentena.length} en cuarentena`
              : 'no hay nada en cuarentena'}
          </span>
        </div>
        <ul style={{ listStyle: 'none', margin: '16px 0 0', padding: 0, display: 'grid', gap: 10 }}>
          {cuarentena.map((c) => (
            <li
              key={c.id}
              className="entre"
              style={{
                background: 'var(--superficie-2)',
                border: '1px solid var(--borde)',
                borderRadius: 9,
                padding: '13px 15px',
              }}
            >
              <div>
                <div style={{ font: '600 14px var(--sans)' }}>{c.titulo}</div>
                <div className="ambar" style={{ fontSize: 13, marginTop: 3 }}>
                  {c.motivo_claro}
                </div>
                {c.motivo_codigo === 'sin_audio' && (
                  <div className="tenue" style={{ fontSize: 12.5, marginTop: 4, maxWidth: 520 }}>
                    Pon a su lado un archivo de audio con el mismo nombre (.wav, .m4a,
                    .aac, .mp3 o .flac) y se procesa solo.
                  </div>
                )}
                {c.motivo_codigo === 'normalizacion_fallida' && (
                  <div className="tenue" style={{ fontSize: 12.5, marginTop: 4, maxWidth: 520 }}>
                    Si lo dejas pasar, sale el archivo original tal cual, sin ajustar el
                    volumen. Vuelve a copiarlo a la carpeta para intentarlo de nuevo.
                  </div>
                )}
                <div className="mono tenue" style={{ fontSize: 11.5, marginTop: 4 }}>
                  {c.ruta}
                </div>
              </div>
              {c.motivo_codigo !== 'sin_audio' && (
                <button className="boton" onClick={() => setDejarPasar(c)} style={{ flexShrink: 0 }}>
                  Dejarlo pasar bajo mi responsabilidad
                </button>
              )}
            </li>
          ))}
        </ul>
      </section>

      {ficha && (
        <FichaLateral
          ficha={ficha}
          presets={presets}
          alCerrar={() => setFicha(null)}
          alActualizar={setFicha}
          anio={anio}
        />
      )}
      {dejarPasar && (
        <PanelDejarPasar
          item={dejarPasar}
          alCerrar={() => setDejarPasar(null)}
          alHecho={() => {
            setDejarPasar(null)
            cargar()
          }}
        />
      )}
    </>
  )
}

function Estante({
  titulo,
  nota,
  notaAmbar,
  titulos,
  alAbrir,
}: {
  titulo: string
  nota: string
  notaAmbar?: boolean
  titulos: TituloDeBiblioteca[]
  alAbrir: (t: TituloDeBiblioteca) => void
}) {
  if (titulos.length === 0) return null
  return (
    <section>
      <div className="entre" style={{ marginBottom: 12 }}>
        <h2 style={{ font: '600 17px var(--sans)', margin: 0 }}>{titulo}</h2>
        <span className={notaAmbar ? 'ambar' : 'tenue'} style={{ fontSize: 13 }}>
          {nota}
        </span>
      </div>
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fill, minmax(126px, 1fr))',
          gap: 16,
        }}
      >
        {titulos.map((t) => (
          <button
            key={t.id}
            onClick={() => alAbrir(t)}
            style={{
              background: 'none',
              border: 0,
              padding: 0,
              cursor: 'pointer',
              textAlign: 'left',
              color: 'inherit',
            }}
          >
            <div style={{ position: 'relative', borderRadius: 8, overflow: 'hidden' }}>
              <div style={{ aspectRatio: '2 / 3' }}>
                <Caratula nombre={t.nombre} iniciales={t.caratula} alto="100%" radio={8} />
              </div>
              {t.hora && (
                <span
                  className="mono"
                  style={{
                    position: 'absolute',
                    top: 8,
                    right: 8,
                    background: 'var(--aqua)',
                    color: '#06141a',
                    borderRadius: 5,
                    padding: '2px 7px',
                    fontSize: 11,
                    fontWeight: 600,
                  }}
                >
                  {minutosAHora12(hhMmAMinutos(t.hora)).replace(/ (AM|PM)$/, '')}
                </span>
              )}
              {t.estado_material !== 'listo' && (
                <span
                  style={{
                    position: 'absolute',
                    bottom: 8,
                    left: 8,
                    right: 8,
                    background: 'rgba(11,14,18,.9)',
                    border: `1px solid ${t.estado_material === 'cuarentena' ? 'var(--rojo)' : 'var(--ambar)'}`,
                    color: t.estado_material === 'cuarentena' ? 'var(--rojo)' : 'var(--ambar)',
                    borderRadius: 5,
                    padding: '3px 6px',
                    fontSize: 10.5,
                    textAlign: 'center',
                  }}
                >
                  {t.estado_material}
                </span>
              )}
            </div>
            <div style={{ font: '400 13.5px var(--sans)', marginTop: 9 }}>{t.nombre}</div>
          </button>
        ))}
      </div>
    </section>
  )
}

function FichaLateral({
  ficha,
  presets,
  alCerrar,
  alActualizar,
  anio,
}: {
  ficha: FichaDeTitulo
  presets: Preset[]
  alCerrar: () => void
  alActualizar: (f: FichaDeTitulo) => void
  anio: number
}) {
  const color =
    ficha.estado_material === 'listo'
      ? 'verde'
      : ficha.estado_material === 'cuarentena'
        ? 'rojo'
        : 'ambar'
  return (
    <Panel titulo={ficha.nombre} alCerrar={alCerrar}>
      <div style={{ borderRadius: 10, overflow: 'hidden' }}>
        <Caratula nombre={ficha.nombre} iniciales={ficha.caratula} alto={150} radio={10} />
      </div>
      <p style={{ fontSize: 14, lineHeight: 1.6, margin: 0, color: 'var(--texto-2)' }}>
        {ficha.sinopsis}
      </p>
      <div className="fila" style={{ gap: 14, flexWrap: 'wrap', fontSize: 13 }}>
        {ficha.anio && <span className="mono tenue">{ficha.anio}</span>}
        <span className="etiqueta etiqueta--nota">{ficha.clasificacion_contenido}</span>
        <span className={'fila ' + color} style={{ gap: 7 }}>
          <i className={'punto punto--' + (color === 'verde' ? 'bien' : color === 'rojo' ? 'problema' : 'aviso')} />
          {ficha.estado_material}
        </span>
        {ficha.hora && (
          <span className="mono aqua">sale a las {minutosAHora12(hhMmAMinutos(ficha.hora))}</span>
        )}
        {ficha.regla_hasta && (
          <span className="tenue">regla hasta {fechaDeRegla(ficha.regla_hasta, anio)}</span>
        )}
      </div>

      <SonidoDelMaterial
        material={ficha}
        presets={presets}
        alCambiar={(cambio) => alActualizar({ ...ficha, ...cambio })}
      />

      <PresetDelPrograma ficha={ficha} presets={presets} alActualizar={alActualizar} />

      <ProgramaInfantil ficha={ficha} alActualizar={alActualizar} />

      {ficha.lista_de_episodios.length > 0 && (
        <div>
          <div className="rotulo" style={{ marginBottom: 10 }}>
            EPISODIOS ({ficha.episodios})
          </div>
          <ul style={{ listStyle: 'none', margin: 0, padding: 0, display: 'grid', gap: 6 }}>
            {ficha.lista_de_episodios.map((e) => (
              <li
                key={e.id}
                style={{
                  padding: '9px 12px',
                  borderRadius: 8,
                  background: 'var(--superficie)',
                  border: '1px solid var(--borde)',
                  fontSize: 13.5,
                }}
              >
                <div className="entre">
                  <span>
                    <span className="mono tenue">
                      T{e.temporada} E{String(e.numero).padStart(2, '0')}
                    </span>{' '}
                    {e.nombre}
                  </span>
                  <span
                    className={e.estado_material === 'listo' ? 'tenue' : 'ambar'}
                    style={{ fontSize: 12.5 }}
                  >
                    {e.estado_material === 'listo'
                      ? duracionLarga(e.duracion_ms)
                      : e.estado_material}
                  </span>
                </div>
                <SonidoDelMaterial
                  material={e}
                  presets={presets}
                  alCambiar={(cambio) =>
                    alActualizar({
                      ...ficha,
                      lista_de_episodios: ficha.lista_de_episodios.map((otro) =>
                        otro.id === e.id ? { ...otro, ...cambio } : otro,
                      ) as EpisodioDeBiblioteca[],
                    })
                  }
                />
              </li>
            ))}
          </ul>
          {ficha.episodios > ficha.lista_de_episodios.length && (
            <p className="ayuda" style={{ marginTop: 10 }}>
              y {ficha.episodios - ficha.lista_de_episodios.length} episodios más.
            </p>
          )}
        </div>
      )}
    </Panel>
  )
}

/**
 * El preset del programa: el nivel de en medio de los tres (canal → programa
 * → archivo). Se pone aquí, en la ficha, porque es donde una persona piensa
 * «este programa siempre suena bajito», que es el caso que justifica que los
 * presets existan.
 *
 * Lo que cambia no toca lo ya convertido. Eso se dice, y se ofrece rehacerlo
 * archivo por archivo más abajo: rehacer un programa entero de golpe puede
 * ser media biblioteca, y eso no se dispara desde un desplegable.
 */
function PresetDelPrograma({
  ficha,
  presets,
  alActualizar,
}: {
  ficha: FichaDeTitulo
  presets: Preset[]
  alActualizar: (f: FichaDeTitulo) => void
}) {
  const [error, setError] = useState('')
  if (presets.length === 0) return null

  async function elegir(id: number | null) {
    const antes = ficha.preset_id ?? null
    setError('')
    alActualizar({ ...ficha, preset_id: id })
    try {
      const guardado = await api.cambiarTitulo(ficha.id, { preset_id: id })
      alActualizar({ ...ficha, preset_id: guardado.preset_id ?? null })
    } catch (e) {
      setError((e as Error).message)
      alActualizar({ ...ficha, preset_id: antes })
    }
  }

  return (
    <div className="campo">
      <label htmlFor={`preset-titulo-${ficha.id}`}>Cómo se prepara este programa</label>
      <EligePreset
        id={`preset-titulo-${ficha.id}`}
        presets={presets}
        valor={ficha.preset_id ?? null}
        heredado="el resto del canal"
        alElegir={(id) => void elegir(id)}
      />
      <span className="ayuda">
        Vale para todos sus episodios. Un archivo suelto puede llevar el suyo y
        entonces manda ése.
      </span>
      {error && <div className="error-claro">{error}</div>}
    </div>
  )
}

/**
 * La marca de programa infantil educativo (E/I, F1-76). Cuenta para las horas
 * de programación infantil de una estación Class A. Se ofrece, no se exige: el
 * interruptor está apagado hasta que una persona lo enciende, y la ayuda dice
 * en llano qué es sin dar por sentado que alguien está en falta.
 */
function ProgramaInfantil({
  ficha,
  alActualizar,
}: {
  ficha: FichaDeTitulo
  alActualizar: (f: FichaDeTitulo) => void
}) {
  const [error, setError] = useState('')
  const [guardando, setGuardando] = useState(false)

  async function cambiar(valor: boolean) {
    setError('')
    setGuardando(true)
    alActualizar({ ...ficha, infantil_core: valor })
    try {
      const guardado = await api.cambiarTitulo(ficha.id, { infantil_core: valor })
      alActualizar({ ...ficha, infantil_core: guardado.infantil_core })
    } catch (e) {
      setError((e as Error).message)
      alActualizar({ ...ficha, infantil_core: !valor })
    } finally {
      setGuardando(false)
    }
  }

  return (
    <div style={{ display: 'grid', gap: 8 }}>
      <div className="entre">
        <div>
          <div style={{ font: '500 14.5px var(--sans)' }}>
            Programa infantil educativo (E/I)
          </div>
          <div className="tenue" style={{ fontSize: 12.5, marginTop: 3, maxWidth: 420 }}>
            Cuenta para las horas de programación infantil que una estación Class A tiene
            que emitir. Solo márcalo si el programa es de educación o información para
            niños.
          </div>
        </div>
        <button
          className="interruptor"
          role="switch"
          aria-checked={ficha.infantil_core}
          aria-label="Programa infantil educativo (E/I)"
          disabled={guardando}
          onClick={() => void cambiar(!ficha.infantil_core)}
        />
      </div>
      {error && <div className="error-claro">{error}</div>}
    </div>
  )
}

/**
 * El sonido de un archivo: con qué pista sale al aire y de dónde salieron el
 * audio y los subtítulos cuando vinieron en un archivo de al lado (F1-58 a
 * F1-62). Un archivo de una sola pista y sin nada al lado no enseña nada.
 */
function SonidoDelMaterial({
  material,
  presets,
  alCambiar,
}: {
  material: AudioDelMaterial & { estado_material: EstadoMaterial }
  presets: Preset[]
  alCambiar: (cambio: Partial<AudioDelMaterial> & { estado_material?: EstadoMaterial }) => void
}) {
  const [error, setError] = useState('')
  const [cambiado, setCambiado] = useState(false)
  const [guardando, setGuardando] = useState(false)
  const [rehaciendo, setRehaciendo] = useState('')

  const pistas = material.pistas_audio ?? []
  const audio = material.audio_sidecar?.trim() ?? ''
  const subtitulos = material.subtitulos_sidecar?.trim() ?? ''
  const hayQueElegir = pistas.length > 1 && material.material_id !== undefined
  // El preset del archivo es el tercer nivel y solo se ofrece cuando hay
  // presets que ofrecer: un canal que no ha creado ninguno no tiene por qué
  // ver un desplegable vacío.
  const hayPresets = presets.length > 0 && material.material_id !== undefined
  if (!hayQueElegir && !hayPresets && !audio && !subtitulos) return null

  async function ponerPreset(id: number | null) {
    const idArchivo = material.material_id
    if (idArchivo === undefined) return
    const antes = material.preset_archivo ?? null
    setError('')
    setRehaciendo('')
    alCambiar({ preset_archivo: id })
    try {
      await api.cambiarMaterial(idArchivo, { preset_id: id })
    } catch (e) {
      setError((e as Error).message)
      alCambiar({ preset_archivo: antes })
    }
  }

  // Cambiar el preset no rehace lo que ya está convertido: el archivo de ayer
  // sigue como estaba hasta que alguien lo pida. Se pide aquí, y lo que
  // contesta el servidor es el texto que sale — no uno inventado en la
  // pantalla, que podría no coincidir con lo que de verdad se encoló.
  async function rehacer() {
    const idArchivo = material.material_id
    if (idArchivo === undefined) return
    setError('')
    try {
      const r = await api.volverAPreparar(idArchivo)
      setRehaciendo(r.texto)
    } catch (e) {
      setError((e as Error).message)
    }
  }

  async function elegir(indice: number) {
    const id = material.material_id
    if (id === undefined) return
    const pistaAntes = material.pista_audio_aire
    const estadoAntes = material.estado_material
    setError('')
    setGuardando(true)
    // El aviso sale de una: el archivo se rehace con la pista nueva (F1-61).
    setCambiado(true)
    alCambiar({ pista_audio_aire: indice, estado_material: 'aún no listo para aire' })
    try {
      const m: MaterialDeAudio = await api.cambiarMaterial(id, { pista_audio_aire: indice })
      alCambiar({
        pista_audio_aire: m.pista_audio_aire ?? indice,
        estado_material: m.estado_material ?? 'aún no listo para aire',
      })
    } catch (e) {
      setCambiado(false)
      setError((e as Error).message)
      alCambiar({ pista_audio_aire: pistaAntes, estado_material: estadoAntes })
    } finally {
      setGuardando(false)
    }
  }

  return (
    <div style={{ display: 'grid', gap: 8, marginTop: hayQueElegir ? 12 : 8 }}>
      {hayQueElegir && (
        <div className="campo">
          <label htmlFor={`pista-audio-${material.material_id}`}>Pista de audio al aire</label>
          <select
            id={`pista-audio-${material.material_id}`}
            value={material.pista_audio_aire ?? pistas[0].indice}
            disabled={guardando}
            onChange={(e) => void elegir(Number(e.target.value))}
          >
            {pistas.map((p, i) => (
              <option key={p.indice} value={p.indice}>
                {nombreDePista(p, i + 1)}
              </option>
            ))}
          </select>
        </div>
      )}
      {hayPresets && (
        <div className="campo">
          <label htmlFor={`preset-archivo-${material.material_id}`}>
            Cómo se prepara este archivo
          </label>
          <EligePreset
            id={`preset-archivo-${material.material_id}`}
            presets={presets}
            valor={material.preset_archivo ?? null}
            heredado="el resto del programa"
            alElegir={(id) => void ponerPreset(id)}
          />
          <div className="fila" style={{ gap: 10, marginTop: 8, flexWrap: 'wrap' }}>
            <button className="boton" onClick={() => void rehacer()}>
              Volver a preparar
            </button>
            {rehaciendo && (
              <span className="tenue" style={{ fontSize: 12.5 }}>
                {rehaciendo}
              </span>
            )}
          </div>
          <span className="ayuda">
            Cambiar el preset no rehace lo que ya se convirtió: eso se pide aquí, y se
            hace en la cola, sin quitarle sitio al aire.
          </span>
        </div>
      )}
      {cambiado && material.estado_material === 'aún no listo para aire' && (
        <span
          className="ambar"
          style={{
            fontSize: 12,
            border: '1px solid var(--ambar)',
            borderRadius: 6,
            padding: '3px 8px',
            justifySelf: 'start',
          }}
        >
          aún no listo para aire
        </span>
      )}
      {error && <div className="error-claro">{error}</div>}
      {audio && (
        <div className="tenue" style={{ fontSize: 12.5 }}>
          Audio: archivo de al lado · <span className="mono">{nombreDeArchivo(audio)}</span>
        </div>
      )}
      {subtitulos && (
        <div className="tenue" style={{ fontSize: 12.5 }}>
          Subtítulos: <span className="mono">{nombreDeArchivo(subtitulos)}</span>
        </div>
      )}
    </div>
  )
}

function PanelDejarPasar({
  item,
  alCerrar,
  alHecho,
}: {
  item: EnCuarentena
  alCerrar: () => void
  alHecho: () => void
}) {
  const [quien, setQuien] = useState('')
  const [error, setError] = useState<string | null>(null)

  async function confirmar() {
    if (!quien.trim()) {
      setError('Escribe tu nombre: queda anotado quién lo dejó pasar.')
      return
    }
    try {
      await api.dejarPasar(item.id, quien.trim())
      alHecho()
    } catch (e) {
      setError((e as Error).message)
    }
  }

  return (
    <Panel
      titulo="Dejarlo pasar bajo mi responsabilidad"
      descripcion={item.titulo}
      alCerrar={alCerrar}
      pie={
        <>
          <button className="boton" onClick={alCerrar}>
            Cancelar
          </button>
          <button className="boton boton--primario" onClick={confirmar}>
            Dejarlo pasar
          </button>
        </>
      }
    >
      <div className="tarjeta tarjeta--aviso" style={{ padding: '14px 16px' }}>
        <div className="ambar" style={{ fontSize: 14 }}>
          {item.motivo_claro}
        </div>
        <div className="mono tenue" style={{ fontSize: 11.5, marginTop: 6 }}>
          {item.ruta}
        </div>
      </div>
      <p className="subtitulo">
        Esto va a salir al aire tal como está. Queda anotado tu nombre y la hora, para que
        después se sepa quién lo autorizó.
      </p>
      {error && <div className="error-claro">{error}</div>}
      <div className="campo">
        <label htmlFor="quien-pasa">¿Quién lo autoriza?</label>
        <input
          id="quien-pasa"
          type="text"
          value={quien}
          onChange={(e) => {
            setQuien(e.target.value)
            setError(null)
          }}
          placeholder="Tu nombre"
        />
      </div>
    </Panel>
  )
}
