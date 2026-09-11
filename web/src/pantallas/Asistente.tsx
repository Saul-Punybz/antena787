import { useEffect, useState, type ReactNode } from 'react'
import { useNavigate } from 'react-router'
import { api } from '../lib/api'
import { useEstado } from '../lib/estado'
import { haceCuanto } from '../lib/fechas'
import { IconoAlerta, IconoAntena, IconoOk } from '../componentes/Iconos'
import { ErrorDeApi } from '../lib/tipos'
import type {
  CuerposDePaso,
  Instalacion,
  NumeroDePaso,
  Opcion,
  RespuestaPaso7,
  RespuestasDePaso,
} from '../lib/tipos'

/**
 * El asistente de instalación (PRD §13). Nueve pasos, seis preguntas: los
 * pasos 3, 8 y 9 no preguntan nada. Un paso a la vez, el riel de los nueve
 * nombres siempre a la vista, y se puede volver atrás a lo ya contestado.
 *
 * Tres reglas que se cumplen aquí y no se negocian:
 *   · nunca se pide escoger nada por su nombre técnico — el servidor manda las
 *     opciones ya escritas en palabras claras y esta pantalla solo las pinta;
 *   · «todavía no» es una respuesta válida y se ve igual de legítima que las
 *     demás, no escondida al final en letra chica;
 *   · la prueba de barras no finge. Las barras se dibujan en esta pantalla; la
 *     salida de verdad se prueba cuando exista el motor de emisión, y eso se
 *     dice con todas sus letras.
 */

const NOMBRES = [
  'Tu canal',
  'Qué vas a hacer',
  'Revisión automática',
  'Tu señal',
  'La prueba de barras',
  'Cómo se ve tu canal',
  'Tu contenido',
  'Tu primera parrilla',
  'Al aire',
]

const PREGUNTAS = [
  '¿Cómo se llama tu canal, y quién entra aquí?',
  '¿Qué vas a hacer?',
  'Revisión automática',
  '¿A dónde va tu señal, y puedes verla de vuelta?',
  '¿Ves las barras de color?',
  '¿Cómo se ve tu canal?',
  'Tu contenido',
  'Tu primera parrilla',
  'Al aire',
]

const DETALLES = [
  'Con el identificativo y la comunidad de licencia se arma el cartel de respaldo: lo último que sale si todo lo demás falla.',
  'Esto no amarra nada. Se cambia después sin volver a instalar.',
  'Este paso no pregunta nada: la máquina mira lo que hay y lo cuenta.',
  'La segunda mitad es poder verla de vuelta. «Todavía no» es una respuesta válida.',
  'La pregunta más importante del producto, y la puede contestar cualquiera.',
  'Escoge lo que espera el equipo al que le mandas la señal.',
  'La carpeta donde ya están los videos. Si no existe, se crea.',
  'Este paso no pregunta nada: propone una semana con lo que hay, y tú decides.',
  'Un botón.',
]

const PASOS: NumeroDePaso[] = [1, 2, 3, 4, 5, 6, 7, 8, 9]

type Resultados = Partial<{ [N in NumeroDePaso]: RespuestasDePaso[N] }>

/**
 * El paso 9 deja el canal instalado, y entonces el armazón de la aplicación
 * vuelve a preguntar el estado y desmonta esta pantalla mientras tanto. Por eso
 * la señal de «acabo de terminar» vive fuera del componente: cuando el asistente
 * se vuelve a montar y el canal ya no necesita instalación, se va a Al aire.
 */
let recienInstalado = false

const enWindows = typeof navigator !== 'undefined' && /Win/i.test(navigator.userAgent)
const CARPETA_EJEMPLO = enWindows ? 'D:\\Contenido' : '/home/tv/contenido'

export function Asistente() {
  const { refrescar, necesitaInstalacion } = useEstado()
  const navegar = useNavigate()

  const [inst, setInst] = useState<Instalacion | null>(null)
  const [cargando, setCargando] = useState(true)
  const [paso, setPaso] = useState<NumeroDePaso>(1)
  const [alcanzado, setAlcanzado] = useState(1)
  const [res, setRes] = useState<Resultados>({})
  const [enviando, setEnviando] = useState(false)
  const [error, setError] = useState('')
  const [campoMalo, setCampoMalo] = useState('')
  const [terminado, setTerminado] = useState(false)

  // Lo que se está escribiendo en cada paso.
  const [nombre, setNombre] = useState('')
  const [identificativo, setIdentificativo] = useState('')
  const [comunidad, setComunidad] = useState('')
  const [operador, setOperador] = useState('')
  const [clave, setClave] = useState('')
  const [claveYaPuesta, setClaveYaPuesta] = useState(false)
  const [modo, setModo] = useState('')
  const [destino, setDestino] = useState('')
  const [retorno, setRetorno] = useState('')
  const [nota, setNota] = useState('')
  const [pais, setPais] = useState('')
  const [calidad, setCalidad] = useState('')
  const [veBarras, setVeBarras] = useState<boolean | null>(null)
  const [carpeta, setCarpeta] = useState('')
  const [relleno, setRelleno] = useState<{ texto: string; bien: boolean } | null>(null)
  const [creandoRelleno, setCreandoRelleno] = useState(false)

  useEffect(() => {
    let vivo = true
    api
      .instalacion()
      .then((i) => {
        if (!vivo) return
        setInst(i)
        const donde = Math.min(Math.max(i.paso || 1, 1), 9) as NumeroDePaso
        setPaso(donde)
        setAlcanzado(i.completa ? 10 : donde)
        const r = i.respuestas ?? {}
        if (r['1']) {
          setNombre(r['1'].nombre ?? '')
          setIdentificativo(r['1'].identificativo ?? '')
          setComunidad(r['1'].comunidad_licencia ?? '')
          setOperador(r['1'].nombre_operador ?? '')
          setClaveYaPuesta(true)
        }
        if (r['2']) setModo(r['2'].modo ?? '')
        if (r['4']) {
          setDestino(r['4'].destino ?? '')
          setRetorno(r['4'].retorno_de_aire ?? '')
        }
        if (r['5']) setVeBarras(r['5'].ve_barras)
        if (r['6']) {
          setPais(r['6'].pais ?? '')
          setCalidad(r['6'].calidad ?? '')
        }
        setCarpeta(r['7']?.carpeta ?? i.detectado?.carpeta_contenido ?? '')
      })
      .catch(() => {
        if (vivo) setError('No se pudo leer en qué paso vamos. Vuelve a cargar la página.')
      })
      .finally(() => vivo && setCargando(false))
    return () => {
      vivo = false
    }
  }, [])

  // Al terminar el paso 9 el canal deja de necesitar instalación: en cuanto el
  // estado se refresca, esta pantalla se va sola a Al aire.
  useEffect(() => {
    if (recienInstalado && !necesitaInstalacion) {
      recienInstalado = false
      navegar('/al-aire', { replace: true })
    }
  }, [necesitaInstalacion, navegar])

  /**
   * Si alguien cambia algo de un paso ya contestado, el botón vuelve a ser
   * «guardar»: se puede volver atrás y corregir sin quedarse trancado.
   */
  function olvidar(n: NumeroDePaso) {
    setRes((x) => {
      if (x[n] === undefined) return x
      const y = { ...x }
      delete y[n]
      return y
    })
  }

  function irA(n: number) {
    setPaso(Math.min(Math.max(n, 1), 9) as NumeroDePaso)
    setError('')
    setCampoMalo('')
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }

  async function enviar<N extends NumeroDePaso>(
    n: N,
    cuerpo: CuerposDePaso[N],
    seguirDeUna = false,
  ): Promise<boolean> {
    setEnviando(true)
    setError('')
    setCampoMalo('')
    try {
      const r = await api.responderPaso(n, cuerpo)
      setRes((x) => ({ ...x, [n]: r }))
      setAlcanzado((a) => Math.max(a, n + 1))
      if (seguirDeUna) irA(n + 1)
      return true
    } catch (e) {
      if (e instanceof ErrorDeApi) {
        setError(e.message)
        setCampoMalo(e.campo ?? '')
      } else {
        setError('No se pudo guardar. Intenta otra vez.')
      }
      return false
    } finally {
      setEnviando(false)
    }
  }

  async function crearRelleno() {
    setCreandoRelleno(true)
    try {
      const r = await api.rellenoPorDefecto()
      setRelleno({ texto: r.aviso, bien: true })
    } catch (e) {
      setRelleno({
        texto:
          e instanceof ErrorDeApi ? e.message : 'No se pudo crear el relleno. Intenta otra vez.',
        bien: e instanceof ErrorDeApi && e.estado === 409,
      })
    } finally {
      setCreandoRelleno(false)
    }
  }

  if (cargando) {
    return (
      <div className="asistente">
        <p className="cargando">Mirando en qué paso vamos…</p>
      </div>
    )
  }

  const opciones = inst?.opciones
  const detectado = inst?.detectado
  const contestado = (res as Record<number, unknown>)[paso] !== undefined
  const dijoQueNoVeBarras = veBarras === false

  // ── el cuerpo de cada paso ─────────────────────────────────────────

  let cuerpo: ReactNode = null
  let botones: ReactNode = null

  if (paso === 1) {
    const r = res[1]
    const listo = nombre.trim().length > 0 && (/^\d{4,6}$/.test(clave) || (claveYaPuesta && clave === ''))
    cuerpo = (
      <>
        <div className="paso__rejilla">
          <Campo
            etiqueta="Nombre del canal"
            valor={nombre}
            alCambiar={(v) => {
              setNombre(v)
              olvidar(1)
            }}
            malo={campoMalo === 'nombre'}
            ayuda="Como lo conoce la gente."
            ejemplo="Caribbean Advantage TV"
          />
          <Campo
            etiqueta="Identificativo"
            valor={identificativo}
            alCambiar={(v) => {
              setIdentificativo(v)
              olvidar(1)
            }}
            malo={campoMalo === 'identificativo'}
            ayuda="Las letras con las que se identifica al aire."
            ejemplo="CAtv"
          />
          <Campo
            etiqueta="Comunidad de licencia"
            valor={comunidad}
            alCambiar={(v) => {
              setComunidad(v)
              olvidar(1)
            }}
            malo={campoMalo === 'comunidad_licencia'}
            ayuda="El pueblo que dice tu licencia."
            ejemplo="Cabo Rojo, Puerto Rico"
          />
          <Campo
            etiqueta="Tu nombre (si quieres)"
            valor={operador}
            alCambiar={(v) => {
              setOperador(v)
              olvidar(1)
            }}
            ayuda="Solo para saber quién dejó el canal montado."
            ejemplo="Rolando"
          />
        </div>

        <div className="tarjeta" style={{ padding: '18px 20px', background: 'var(--superficie-2)' }}>
          <div className="rotulo">LA CLAVE DE LA ESTACIÓN</div>
          <p className="subtitulo" style={{ marginTop: 8, maxWidth: 560 }}>
            No es un usuario con contraseña ni una lista de quién puede hacer qué: es una sola clave
            para la estación, que se pide una vez por navegador y no vuelve a estorbar.
            Existe para que el sobrino que se conecta al wifi no saque el canal del aire.
          </p>
          <div className="campo" style={{ maxWidth: 220, marginTop: 14 }}>
            <label htmlFor="clave">Cuatro a seis dígitos</label>
            <input
              id="clave"
              className="entrar__clave"
              type="text"
              inputMode="numeric"
              autoComplete="off"
              maxLength={6}
              placeholder="••••"
              value={clave}
              style={campoMalo === 'clave' ? { borderColor: 'var(--rojo)' } : undefined}
              onChange={(e) => {
                setClave(e.target.value.replace(/\D/g, '').slice(0, 6))
                olvidar(1)
              }}
            />
            <span className="ayuda">
              {claveYaPuesta
                ? 'Ya hay una clave puesta. Déjalo en blanco para conservarla, o escribe otra.'
                : 'Se enseña una sola vez. Apúntala donde la encuentres después.'}
            </span>
          </div>
        </div>

        {r && (
          <div className="nota nota--bien">
            <strong style={{ fontWeight: 600 }}>Tu cartel de respaldo: {r.cartel}</strong>
            <div style={{ marginTop: 4 }}>
              Es lo último que sale al aire si un día falla todo lo demás. No hace falta
              hacer nada más con él.
            </div>
          </div>
        )}
      </>
    )
    botones = contestado ? (
      <Seguir alSeguir={() => irA(2)} />
    ) : (
      <button
        className="boton boton--primario"
        disabled={!listo || enviando}
        onClick={() =>
          enviar(1, {
            nombre: nombre.trim(),
            identificativo: identificativo.trim(),
            comunidad_licencia: comunidad.trim(),
            clave,
            nombre_operador: operador.trim(),
          })
        }
      >
        {enviando ? 'Guardando…' : 'Guardar y seguir'}
      </button>
    )
  }

  if (paso === 2) {
    cuerpo = (
      <>
        <Opciones
          lista={opciones?.modo ?? []}
          valor={modo}
          alElegir={(v) => {
            setModo(v)
            olvidar(2)
          }}
          grandes
        />
        {res[2] && <div className="nota">{res[2].aviso}</div>}
      </>
    )
    botones = contestado ? (
      <Seguir alSeguir={() => irA(3)} />
    ) : (
      <button
        className="boton boton--primario"
        disabled={!modo || enviando}
        onClick={() => enviar(2, { modo })}
      >
        {enviando ? 'Guardando…' : 'Seguir'}
      </button>
    )
  }

  if (paso === 3) {
    const problema = res[3]?.problema ?? detectado?.problema
    cuerpo = (
      <>
        <ul style={{ listStyle: 'none', margin: 0, padding: 0, display: 'grid', gap: 11 }}>
          <Hallazgo
            bien={Boolean(detectado?.ffmpeg)}
            texto={
              detectado?.ffmpeg
                ? 'Está el programa que convierte el video, y también el que lo mide.'
                : 'No aparece el programa que convierte el video.'
            }
            detalle={[detectado?.ffmpeg, detectado?.ffprobe].filter(Boolean).join('  ·  ')}
          />
          <Hallazgo bien texto={detectado?.disco ?? 'El disco tiene espacio.'} />
          <Hallazgo bien texto={detectado?.red ?? 'La red está conectada.'} />
          <Hallazgo
            bien
            texto="Los datos del canal y las copias de respaldo ya tienen dónde vivir."
            detalle={[detectado?.carpeta_datos, detectado?.carpeta_respaldo]
              .filter(Boolean)
              .join('  ·  ')}
          />
          <Hallazgo
            bien={false}
            neutro
            texto={`Aceleración de video: ${detectado?.aceleracion ?? 'todavía sin medir'}.`}
          />
        </ul>
        {problema && (
          <div className="tarjeta tarjeta--aviso" style={{ padding: '16px 18px' }}>
            <div className="fila" style={{ gap: 9, alignItems: 'flex-start' }}>
              <IconoAlerta tamano={17} color="var(--ambar)" />
              <div>
                <div style={{ font: '600 14.5px var(--sans)' }}>{problema}</div>
                <p className="subtitulo" style={{ marginTop: 5 }}>
                  Se puede seguir: los pasos que faltan no lo necesitan. Cuando lo
                  arregles, este mismo asistente lo vuelve a mirar sin tener que empezar
                  de nuevo.
                </p>
              </div>
            </div>
          </div>
        )}
      </>
    )
    botones = (
      <button
        className="boton boton--primario"
        disabled={enviando}
        onClick={() => void enviar(3, {}, true)}
      >
        {enviando ? 'Mirando…' : 'Seguir'}
      </button>
    )
  }

  if (paso === 4) {
    cuerpo = (
      <>
        <div>
          <div className="rotulo">A DÓNDE VA TU SEÑAL</div>
          <div style={{ marginTop: 10 }}>
            <Opciones
              lista={opciones?.destino ?? []}
              valor={destino}
              alElegir={(v) => {
                setDestino(v)
                olvidar(4)
              }}
            />
          </div>
        </div>
        <div>
          <div className="rotulo">¿PUEDES VERLA DE VUELTA?</div>
          <p className="subtitulo" style={{ marginTop: 4, marginBottom: 10 }}>
            Es lo que de verdad está saliendo, después de todos los equipos. Sirve para
            comprobar que lo que se emite es lo que dijiste.
          </p>
          <Opciones
            lista={opciones?.retorno ?? []}
            valor={retorno}
            alElegir={(v) => {
              setRetorno(v)
              olvidar(4)
            }}
          />
        </div>
        <div className="campo">
          <label htmlFor="nota">¿Algo que quieras dejar apuntado? (si quieres)</label>
          <input
            id="nota"
            type="text"
            value={nota}
            placeholder="El cable de la señal está detrás del rack."
            onChange={(e) => {
              setNota(e.target.value)
              olvidar(4)
            }}
          />
        </div>
        {res[4]?.aviso && <div className="nota nota--aviso">{res[4].aviso}</div>}
        {res[4] && !res[4].aviso && (
          <div className="nota nota--bien">Apuntado. Se configura de verdad más adelante.</div>
        )}
      </>
    )
    botones = contestado ? (
      <Seguir alSeguir={() => irA(5)} />
    ) : (
      <button
        className="boton boton--primario"
        disabled={!destino || !retorno || enviando}
        onClick={() =>
          enviar(4, {
            destino,
            retorno_de_aire: retorno,
            ...(nota.trim() ? { nota: nota.trim() } : {}),
          })
        }
      >
        {enviando ? 'Guardando…' : 'Seguir'}
      </button>
    )
  }

  if (paso === 5) {
    cuerpo = (
      <>
        <BarrasDeColor />
        <p className="ayuda" style={{ margin: 0, color: 'var(--texto-3)' }}>
          Estas barras están dibujadas en esta pantalla. Las mismas barras, y el tono que
          las acompaña, salen por la salida del canal cuando exista el motor de emisión:
          eso es lo que hay que mirar en el televisor. Hasta entonces, lo que contestes
          queda apuntado y no se prueba nada a tus espaldas.
        </p>
        <div className="fila" style={{ gap: 10 }}>
          <button
            className={'boton' + (veBarras !== false ? ' boton--primario' : '')}
            aria-pressed={veBarras === true}
            disabled={enviando}
            onClick={async () => {
              if (await enviar(5, { ve_barras: true })) setVeBarras(true)
            }}
          >
            Sí, las veo
          </button>
          <button
            className={'boton' + (veBarras === false ? ' boton--primario' : '')}
            aria-pressed={veBarras === false}
            disabled={enviando}
            onClick={async () => {
              if (await enviar(5, { ve_barras: false })) setVeBarras(false)
            }}
          >
            No las veo
          </button>
        </div>
        {res[5] && <div className="nota">{res[5].aviso}</div>}
        {res[5] && veBarras === false && (
          <div className="nota nota--aviso">
            Esto no es un callejón sin salida: se sigue igual y la salida se vuelve a
            revisar cuando llegue el motor de emisión.
          </div>
        )}
      </>
    )
    botones = contestado ? <Seguir alSeguir={() => irA(6)} /> : null
  }

  if (paso === 6) {
    const r = res[6]
    cuerpo = (
      <>
        {dijoQueNoVeBarras && (
          <div className="nota">
            Quedó apuntado que no veías las barras. La salida se vuelve a revisar cuando
            llegue el motor de emisión; nada de lo que sigue depende de eso.
          </div>
        )}
        <div className="campo" style={{ maxWidth: 340 }}>
          <label htmlFor="pais">¿En qué país está el canal?</label>
          <input
            id="pais"
            type="text"
            list="paises"
            value={pais}
            placeholder="Puerto Rico"
            style={campoMalo === 'pais' ? { borderColor: 'var(--rojo)' } : undefined}
            onChange={(e) => {
              setPais(e.target.value)
              olvidar(6)
            }}
          />
          <datalist id="paises">
            <option value="Puerto Rico" />
            <option value="Estados Unidos" />
          </datalist>
          <span className="ayuda">
            Con esto se sabe qué reglas de emisión te aplican. Si tu país todavía no está
            afinado, se usa un perfil abierto y el software no te regaña por eso.
          </span>
          <div className="pastillas" style={{ marginTop: 8 }}>
            {['Puerto Rico', 'Estados Unidos'].map((p) => (
              <button
                key={p}
                type="button"
                className={'pastilla' + (pais === p ? ' pastilla--activa' : '')}
                onClick={() => {
                  setPais(p)
                  olvidar(6)
                }}
              >
                {p}
              </button>
            ))}
          </div>
        </div>
        <div>
          <div className="rotulo">CALIDAD</div>
          <p className="subtitulo" style={{ marginTop: 4, marginBottom: 10 }}>
            Escoge lo que espera el equipo al que le mandas la señal. Si no sabes, la
            primera de la lista es la que usa casi todo el mundo por aquí.
          </p>
          <Opciones
            lista={opciones?.calidad ?? []}
            valor={calidad}
            alElegir={(v) => {
              setCalidad(v)
              olvidar(6)
            }}
          />
        </div>
        {r && (
          <div className="nota nota--bien">
            Tu canal queda en {r.formato}. Se cambia después en Ajustes.
          </div>
        )}
      </>
    )
    botones = contestado ? (
      <Seguir alSeguir={() => irA(7)} />
    ) : (
      <button
        className="boton boton--primario"
        disabled={!calidad || enviando}
        onClick={() => enviar(6, { pais: pais.trim(), calidad })}
      >
        {enviando ? 'Guardando…' : 'Seguir'}
      </button>
    )
  }

  if (paso === 7) {
    const r: RespuestaPaso7 | undefined = res[7]
    cuerpo = (
      <>
        <div className="campo" style={{ maxWidth: 520 }}>
          <label htmlFor="carpeta">La carpeta donde están los videos</label>
          <input
            id="carpeta"
            type="text"
            value={carpeta}
            placeholder={CARPETA_EJEMPLO}
            style={campoMalo === 'carpeta' ? { borderColor: 'var(--rojo)' } : undefined}
            onChange={(e) => {
              setCarpeta(e.target.value)
              olvidar(7)
            }}
          />
          <span className="ayuda">
            En Windows se ve como <span className="mono">D:\Contenido</span>; en Mac o
            Linux, como <span className="mono">/home/tv/contenido</span>. Si la carpeta no
            existe todavía, se crea.
          </span>
        </div>

        {r && (
          <div className="nota nota--bien">
            <div className="fila" style={{ gap: 9, alignItems: 'flex-start' }}>
              <IconoOk tamano={16} color="var(--verde)" />
              <div>
                <div style={{ fontWeight: 600 }}>{r.aviso_vigilancia}</div>
                <div className="mono" style={{ marginTop: 4, fontSize: 12.5 }}>
                  {r.carpeta}
                </div>
              </div>
            </div>
          </div>
        )}

        {r?.aviso_relleno && (
          <div className="tarjeta tarjeta--aviso" style={{ padding: '16px 18px' }}>
            <div style={{ font: '500 14px var(--sans)' }}>{r.aviso_relleno}</div>
            <p className="subtitulo" style={{ marginTop: 5 }}>
              Eso no se puede descubrir a las tres de la mañana. Se arregla de un clic: el
              cartel de la estación con una cama musical. Se cambia después; lo que no se
              puede es quedarse sin él.
            </p>
            <button
              className="boton"
              style={{ marginTop: 12 }}
              disabled={creandoRelleno || (relleno?.bien ?? false)}
              onClick={() => void crearRelleno()}
            >
              {creandoRelleno ? 'Creándolo…' : 'Crear un relleno por defecto'}
            </button>
            {relleno && (
              <div
                className={'nota ' + (relleno.bien ? 'nota--bien' : 'nota--aviso')}
                style={{ marginTop: 12 }}
              >
                {relleno.texto}
              </div>
            )}
          </div>
        )}
      </>
    )
    botones = contestado ? (
      <Seguir alSeguir={() => irA(8)} />
    ) : (
      <button
        className="boton boton--primario"
        disabled={!carpeta.trim() || enviando}
        onClick={() => enviar(7, { carpeta: carpeta.trim() })}
      >
        {enviando ? 'Mirando la carpeta…' : 'Seguir'}
      </button>
    )
  }

  if (paso === 8) {
    const r = res[8]
    cuerpo = (
      <>
        <p className="subtitulo" style={{ margin: 0, maxWidth: 620 }}>
          Con lo que hay en tu carpeta se puede armar una semana completa ahora mismo. Es
          una propuesta, no una decisión: todo se mueve, se borra y se rehace desde la
          parrilla.
        </p>
        <div className="fila" style={{ gap: 10, flexWrap: 'wrap' }}>
          <button
            className="boton boton--primario"
            disabled={enviando}
            onClick={() => enviar(8, { propuesta: 'automatica' })}
          >
            Armar una propuesta con lo que hay
          </button>
          <button
            className="boton"
            disabled={enviando}
            onClick={() => enviar(8, { propuesta: 'ninguna' })}
          >
            Lo armo yo después
          </button>
        </div>
        {r && (
          <>
            <div
              style={{
                display: 'grid',
                gridTemplateColumns: 'repeat(auto-fit, minmax(150px, 1fr))',
                gap: 12,
              }}
            >
              <Cuenta numero={r.reglas_creadas} rotulo="reglas armadas" />
              <Cuenta numero={r.bloques} rotulo="bloques en la parrilla" />
              <Cuenta numero={r.titulos_sin_material} rotulo="títulos sin material" />
            </div>
            <div className="nota">{r.aviso}</div>
            {r.avisos.length > 0 && (
              <ul style={{ listStyle: 'none', margin: 0, padding: 0, display: 'grid', gap: 9 }}>
                {r.avisos.map((a, i) => (
                  <li key={i} className="fila" style={{ gap: 9, alignItems: 'flex-start' }}>
                    <IconoAlerta tamano={16} color="var(--ambar)" />
                    <span style={{ fontSize: 13.5 }}>{a.texto}</span>
                  </li>
                ))}
              </ul>
            )}
          </>
        )}
      </>
    )
    botones = contestado ? <Seguir alSeguir={() => irA(9)} /> : null
  }

  if (paso === 9) {
    const r = res[9]
    cuerpo = (
      <>
        <p className="subtitulo" style={{ margin: 0, maxWidth: 620 }}>
          Ya está todo contestado. Este botón deja el canal montado y te lleva a la
          pantalla de aire.
        </p>
        {r && <div className="nota nota--bien">{r.aviso}</div>}
      </>
    )
    botones = r ? (
      <button
        className="boton boton--primario"
        disabled={terminado}
        onClick={() => {
          recienInstalado = true
          setTerminado(true)
          refrescar()
        }}
      >
        {terminado ? 'Entrando al canal…' : 'Entrar al canal'}
      </button>
    ) : (
      <button
        className="boton boton--primario"
        disabled={enviando}
        onClick={() => void enviar(9, {})}
      >
        {enviando ? 'Un momento…' : 'Al aire'}
      </button>
    )
  }

  // ── la pantalla ────────────────────────────────────────────────────

  const ahora = Date.now()

  return (
    <div className="asistente">
      <div className="asistente__marca">
        <IconoAntena tamano={26} color="var(--aqua)" grosor={1.8} />
        <span>
          Antena<span className="aqua">787</span>
        </span>
      </div>

      <h1 className="titulo-pantalla">Vamos a poner tu canal al aire</h1>
      <p className="subtitulo">
        Nueve pasos, seis preguntas. Lo que la máquina puede averiguar sola, no se
        pregunta.
      </p>

      <div className="asistente__cuerpo">
        <ol className="riel">
          {PASOS.map((n) => {
            const hecho = n < alcanzado && n !== paso
            const cuando = inst?.tiempos?.[String(n)]
            return (
              <li key={n}>
                <button
                  type="button"
                  className={
                    'riel__paso' +
                    (n === paso ? ' riel__paso--actual' : hecho ? ' riel__paso--hecho' : '')
                  }
                  disabled={n > alcanzado}
                  onClick={() => n <= alcanzado && irA(n)}
                >
                  <span className="riel__num">{hecho ? '✓' : n}</span>
                  <span>
                    {NOMBRES[n - 1]}
                    {hecho && cuando && (
                      <span className="riel__cuando">{haceCuanto(cuando, ahora)}</span>
                    )}
                  </span>
                </button>
              </li>
            )
          })}
        </ol>

        <div className="tarjeta paso">
          <div className="rotulo">
            PASO {paso} DE {inst?.pasos ?? 9}
            {(paso === 3 || paso === 8 || paso === 9) && (
              <span className="etiqueta etiqueta--nota" style={{ marginLeft: 10 }}>
                no pregunta nada
              </span>
            )}
          </div>
          <h2 className="paso__pregunta" style={{ marginTop: 8 }}>
            {PREGUNTAS[paso - 1]}
          </h2>
          <p className="subtitulo">{DETALLES[paso - 1]}</p>

          <div style={{ display: 'grid', gap: 18, marginTop: 20 }}>{cuerpo}</div>

          {error && (
            <div className="error-claro" style={{ marginTop: 16 }}>
              {error}
            </div>
          )}

          <div className="paso__pie">
            {paso > 1 && (
              <button className="boton" disabled={enviando} onClick={() => irA(paso - 1)}>
                Atrás
              </button>
            )}
            <span className="crece" />
            {botones}
          </div>
        </div>
      </div>
    </div>
  )
}

// ── piezas ────────────────────────────────────────────────────────────

function Seguir({ alSeguir }: { alSeguir: () => void }) {
  return (
    <button className="boton boton--primario" onClick={alSeguir}>
      Seguir
    </button>
  )
}

function Campo({
  etiqueta,
  valor,
  alCambiar,
  ayuda,
  ejemplo,
  malo,
}: {
  etiqueta: string
  valor: string
  alCambiar: (v: string) => void
  ayuda?: string
  ejemplo?: string
  malo?: boolean
}) {
  return (
    <div className="campo">
      <label>{etiqueta}</label>
      <input
        type="text"
        value={valor}
        placeholder={ejemplo}
        style={malo ? { borderColor: 'var(--rojo)' } : undefined}
        onChange={(e) => alCambiar(e.target.value)}
      />
      {ayuda && <span className="ayuda">{ayuda}</span>}
    </div>
  )
}

/**
 * Las respuestas posibles, tal como las manda el servidor. Todas se ven igual:
 * «todavía no» no lleva letra chica ni color de advertencia.
 */
function Opciones({
  lista,
  valor,
  alElegir,
  grandes,
}: {
  lista: Opcion[]
  valor: string
  alElegir: (v: string) => void
  grandes?: boolean
}) {
  if (lista.length === 0) {
    return (
      <p className="ayuda" style={{ color: 'var(--texto-3)' }}>
        El servidor todavía no mandó las opciones de este paso.
      </p>
    )
  }
  return (
    <div className={'opciones' + (grandes ? ' opciones--grandes' : '')}>
      {lista.map((o) => (
        <button
          key={o.valor}
          type="button"
          className={'opcion' + (valor === o.valor ? ' opcion--elegida' : '')}
          aria-pressed={valor === o.valor}
          onClick={() => alElegir(o.valor)}
        >
          <span className="opcion__marca" />
          <span>
            <span className="opcion__texto">{o.texto}</span>
            {o.ayuda && <p className="opcion__ayuda">{o.ayuda}</p>}
          </span>
        </button>
      ))}
    </div>
  )
}

function Hallazgo({
  bien,
  neutro,
  texto,
  detalle,
}: {
  bien: boolean
  neutro?: boolean
  texto: string
  detalle?: string
}) {
  return (
    <li className="fila" style={{ gap: 10, alignItems: 'flex-start' }}>
      {neutro ? (
        <span className="punto" style={{ marginTop: 6 }} />
      ) : bien ? (
        <IconoOk tamano={16} color="var(--verde)" />
      ) : (
        <IconoAlerta tamano={16} color="var(--ambar)" />
      )}
      <div>
        <div style={{ font: '400 14px var(--sans)' }}>{texto}</div>
        {detalle && (
          <div className="mono tenue" style={{ fontSize: 12, marginTop: 3 }}>
            {detalle}
          </div>
        )}
      </div>
    </li>
  )
}

function Cuenta({ numero, rotulo }: { numero: number; rotulo: string }) {
  return (
    <div className="tarjeta" style={{ padding: '14px 16px', background: 'var(--superficie-2)' }}>
      <div className="mono" style={{ font: '600 24px var(--mono)' }}>
        {numero}
      </div>
      <div className="tenue" style={{ fontSize: 12.5, marginTop: 2 }}>
        {rotulo}
      </div>
    </div>
  )
}

/**
 * Las barras de color de la prueba del paso 5, dibujadas aquí mismo: siete
 * barras al 75 %, la fila de castillos y la fila de abajo con el pluge. No son
 * la salida del canal — eso lo dice la pantalla, no se disimula.
 */
function BarrasDeColor() {
  const ancho = 480
  const alto = 270
  const b = ancho / 7
  const arriba = ['#c0c0c0', '#c0c000', '#00c0c0', '#00c000', '#c000c0', '#c00000', '#0000c0']
  const castillos = ['#0000c0', '#131313', '#c000c0', '#131313', '#00c0c0', '#131313', '#c0c0c0']
  const yCastillos = alto * 0.67
  const yAbajo = alto * 0.75
  return (
    <svg
      className="barras"
      viewBox={`0 0 ${ancho} ${alto}`}
      role="img"
      aria-label="Barras de color de prueba"
    >
      {arriba.map((c, i) => (
        <rect key={c} x={i * b} y={0} width={b + 0.5} height={yCastillos} fill={c} />
      ))}
      {castillos.map((c, i) => (
        <rect
          key={'c' + i}
          x={i * b}
          y={yCastillos}
          width={b + 0.5}
          height={yAbajo - yCastillos}
          fill={c}
        />
      ))}
      <rect x={0} y={yAbajo} width={b * 2} height={alto - yAbajo} fill="#00214c" />
      <rect x={b * 2} y={yAbajo} width={b} height={alto - yAbajo} fill="#ffffff" />
      <rect x={b * 3} y={yAbajo} width={b} height={alto - yAbajo} fill="#32006a" />
      <rect x={b * 4} y={yAbajo} width={b * 2} height={alto - yAbajo} fill="#131313" />
      <rect x={b * 6} y={yAbajo} width={b / 3} height={alto - yAbajo} fill="#0a0a0a" />
      <rect x={b * 6 + b / 3} y={yAbajo} width={b / 3} height={alto - yAbajo} fill="#131313" />
      <rect
        x={b * 6 + (b * 2) / 3}
        y={yAbajo}
        width={b / 3 + 0.5}
        height={alto - yAbajo}
        fill="#1c1c1c"
      />
    </svg>
  )
}
