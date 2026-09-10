import { useEffect, useState } from 'react'
import { api } from '../lib/api'
import { useEstado } from '../lib/estado'
import { haceCuanto } from '../lib/fechas'
import { IconoEquis, IconoOk } from '../componentes/Iconos'
import type { Ajustes as MapaDeAjustes } from '../lib/tipos'

/**
 * Ajustes son las buenas prácticas vigiladas para siempre, no aplicadas una
 * vez. Muestra siempre tres números que no se ven en ningún otro lado: la
 * deriva del reloj, qué aceleración quedó elegida y con qué resultado, y el
 * umbral de silencio vigente (PRD §13).
 *
 * El volumen se enseña como "volumen de televisión de EE. UU.": la sigla del
 * estándar nunca sale a pantalla (PRD §4.3).
 */
export function Ajustes() {
  const { estado } = useEstado()
  const [ajustes, setAjustes] = useState<MapaDeAjustes | null>(null)
  const [guardado, setGuardado] = useState('')

  useEffect(() => {
    api.ajustes().then(setAjustes).catch(() => setAjustes(null))
  }, [])

  async function cambiar(clave: string, valor: string) {
    if (!ajustes) return
    const nuevo = { ...ajustes, [clave]: valor }
    setAjustes(nuevo)
    await api.guardarAjustes(nuevo).catch(() => {})
    setGuardado('Guardado')
    window.setTimeout(() => setGuardado(''), 1600)
  }

  if (!ajustes) return <p className="cargando">Leyendo los ajustes…</p>

  const salud = [
    {
      ok: ajustes.antivirus_exclusiones === 'sí',
      titulo: 'Exclusiones de antivirus',
      detalle: 'Carpetas de video y la base de datos, fuera del escaneo',
    },
    {
      ok: ajustes.energia_plan === 'sí',
      titulo: 'Plan de energía',
      detalle: 'Sin suspensión, sin apagado de disco',
    },
    {
      ok: ajustes.arranque_tras_corte === 'sí',
      titulo: 'Arranque tras corte de luz',
      detalle: 'Habilitado en el BIOS',
    },
    {
      ok: ajustes.rutas_largas === 'sí',
      titulo: 'Rutas largas',
      detalle: 'Nombres de más de 260 caracteres permitidos',
    },
    {
      ok: ajustes.actualizaciones_windows === 'sí',
      titulo: 'Actualizaciones del sistema',
      detalle:
        ajustes.actualizaciones_windows === 'sí'
          ? 'No reinician la máquina sin avisar'
          : 'Están en automático — pueden reiniciar la máquina al aire',
    },
  ]
  const porArreglar = salud.filter((s) => !s.ok).length
  const ahora = Date.parse(estado?.ahora ?? new Date().toISOString())

  return (
    <>
      <div className="encabezado">
        <div>
          <h1 className="titulo-pantalla">Ajustes</h1>
          <p className="subtitulo">
            Antena787 v{ajustes.version} · {ajustes.sistema_operativo} · al aire hace{' '}
            {ajustes.dias_al_aire} días sin interrupciones
          </p>
        </div>
        <div className="fila" style={{ gap: 12 }}>
          {guardado && <span className="verde" style={{ fontSize: 13 }}>{guardado}</span>}
          <span
            className={'franja-modo ' + (porArreglar ? 'franja-modo--sombra' : 'franja-modo--aire')}
            style={porArreglar ? { borderColor: 'rgba(248,81,73,.45)', color: 'var(--rojo)', background: 'rgba(248,81,73,.1)' } : undefined}
          >
            <span className={'punto ' + (porArreglar ? 'punto--problema' : 'punto--bien')} />
            {porArreglar
              ? `${porArreglar} cosa${porArreglar === 1 ? '' : 's'} que arreglar`
              : 'todo en orden'}
          </span>
        </div>
      </div>

      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(300px, 1fr))',
          gap: 16,
          alignItems: 'start',
        }}
      >
        {/* Salud de la máquina */}
        <Tarjeta
          rotulo="SALUD DE LA MÁQUINA"
          ancha
          problema={porArreglar > 0}
        >
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))',
              gap: '16px 24px',
            }}
          >
            {salud.map((s) => (
              <div key={s.titulo} className="fila" style={{ alignItems: 'flex-start', gap: 11 }}>
                {s.ok ? (
                  <IconoOk tamano={16} color="var(--verde)" />
                ) : (
                  <IconoEquis tamano={16} color="var(--rojo)" />
                )}
                <div>
                  <div style={{ font: '500 14px var(--sans)' }}>{s.titulo}</div>
                  <div className="tenue" style={{ fontSize: 12.5, marginTop: 2 }}>
                    {s.detalle}
                  </div>
                </div>
              </div>
            ))}
          </div>
          {porArreglar > 0 && (
            <button
              className="boton"
              style={{ marginTop: 18 }}
              onClick={() => cambiar('actualizaciones_windows', 'sí')}
            >
              Arreglar las actualizaciones del sistema
            </button>
          )}
        </Tarjeta>

        {/* Respaldo */}
        <Tarjeta rotulo="RESPALDO">
          <Linea nombre="Último" valor={haceCuanto(ajustes.respaldo_ultimo, ahora)} verde />
          <Linea nombre="Cada" valor={ajustes.respaldo_cada} />
          <Linea nombre="Copias guardadas" valor={ajustes.respaldo_copias} />
          <Linea nombre="Tamaño" valor={ajustes.respaldo_tamano} />
          <button className="boton" style={{ marginTop: 16 }}>
            Bajar una copia
          </button>
        </Tarjeta>

        {/* Actualizaciones */}
        <Tarjeta rotulo="ACTUALIZACIONES">
          <Linea nombre="Versión" valor={ajustes.version} />
          <Linea nombre="Hay disponible" valor={ajustes.actualizaciones_disponible} ambar />
          <Linea nombre="Instalar sola" valor={ajustes.actualizaciones_instalar_sola} verde />
          <p className="ayuda" style={{ marginTop: 12 }}>
            Un canal al aire no se actualiza solo. Tú escoges cuándo, con la caja de
            respaldo lista.
          </p>
          <button className="boton" style={{ marginTop: 12 }}>
            Ver qué cambia
          </button>
        </Tarjeta>

        {/* Cumplimiento */}
        <Tarjeta rotulo="CUMPLIMIENTO" ancha>
          <Linea nombre="País" valor={ajustes.pais} />
          <Linea nombre="Perfil" valor={ajustes.perfil} />
          <Linea nombre="Volumen" valor={ajustes.volumen} verde />
          <Linea nombre="Subtítulos" valor={ajustes.subtitulos} verde />
          <Linea nombre="Equipo de alertas" valor={ajustes.equipo_de_alertas} verde />
        </Tarjeta>

        {/* Acceso remoto */}
        <Tarjeta rotulo="ACCESO REMOTO">
          <Linea nombre="Tailscale" valor={ajustes.tailscale} verde />
          <Linea nombre="Dirección" valor={ajustes.tailscale_direccion} />
          <Linea nombre="Puertos abiertos" valor={ajustes.puertos_abiertos} verde />
          <p className="ayuda" style={{ marginTop: 12 }}>
            Se entra desde el celular sin abrir nada al internet.
          </p>
        </Tarjeta>

        {/* Hora */}
        <Tarjeta rotulo="HORA">
          <Linea nombre="Sincronizada con" valor={ajustes.hora_servidor} aqua />
          <Linea nombre="Desvío" valor={segundos(ajustes.hora_desvio_s)} verde />
          <p className="ayuda" style={{ marginTop: 12 }}>
            Avisa si pasa de {segundos(ajustes.hora_aviso_si_pasa_de_s)}.
          </p>
        </Tarjeta>

        {/* Canal */}
        <Tarjeta rotulo="CANAL">
          <Linea nombre="Nombre" valor={estado?.canal.nombre ?? '—'} />
          <Linea nombre="Identificativo" valor={estado?.canal.identificativo ?? '—'} />
          <Linea nombre="Comunidad de licencia" valor={estado?.canal.comunidad_licencia ?? '—'} />
          <Linea nombre="Zona horaria" valor={estado?.canal.zona_horaria ?? '—'} />
          <Linea
            nombre="El día de emisión empieza"
            valor={`${String(Math.floor((estado?.canal.hora_inicio_dia_emision ?? 360) / 60)).padStart(2, '0')}:${String((estado?.canal.hora_inicio_dia_emision ?? 360) % 60).padStart(2, '0')}`}
          />
        </Tarjeta>

        {/* Aceleración */}
        <Tarjeta rotulo="ACELERACIÓN DE VIDEO">
          <Linea nombre="Tarjeta" valor={ajustes.aceleracion_tarjeta} />
          <Linea nombre="Probada" valor={ajustes.aceleracion_probada} verde />
          <Linea nombre="Resultado" valor={ajustes.aceleracion_resultado} verde />
          <p className="ayuda" style={{ marginTop: 12 }}>
            Se prueba sola cada vez que arranca.
          </p>
        </Tarjeta>

        {/* Audio */}
        <Tarjeta rotulo="AUDIO">
          <div className="campo">
            <label htmlFor="idioma-audio">Idioma de la pista de audio para el aire</label>
            <select
              id="idioma-audio"
              value={ajustes.idioma_audio_preferido ?? 'es'}
              onChange={(e) => cambiar('idioma_audio_preferido', e.target.value)}
            >
              <option value="es">Español</option>
              <option value="en">Inglés</option>
            </select>
            <span className="ayuda">
              Cuando un archivo trae varias pistas, se elige la primera en este idioma;
              si no hay, la primera del archivo.
            </span>
          </div>
        </Tarjeta>

        {/* Detector de silencio */}
        <Tarjeta rotulo="DETECTOR DE SILENCIO">
          <div className="entre">
            <div>
              <div style={{ font: '500 14.5px var(--sans)' }}>Avisa y devuelve el control</div>
              <div className="tenue" style={{ fontSize: 12.5, marginTop: 3 }}>
                Si no sale señal más de {segundos(ajustes.silencio_umbral_s)}
              </div>
            </div>
            <button
              className="interruptor"
              role="switch"
              aria-checked={ajustes.silencio_avisa === 'sí'}
              onClick={() =>
                cambiar('silencio_avisa', ajustes.silencio_avisa === 'sí' ? 'no' : 'sí')
              }
            />
          </div>
          <div className="entre" style={{ marginTop: 20 }}>
            <label htmlFor="umbral" style={{ font: '400 14px var(--sans)' }}>
              Umbral
            </label>
            <span className="fila" style={{ gap: 7 }}>
              <input
                id="umbral"
                type="number"
                min={3}
                max={120}
                value={ajustes.silencio_umbral_s}
                onChange={(e) => cambiar('silencio_umbral_s', e.target.value)}
                className="mono"
                style={{
                  width: 62,
                  background: 'var(--superficie-2)',
                  border: '1px solid var(--borde)',
                  borderRadius: 7,
                  color: 'var(--texto)',
                  padding: '7px 9px',
                  textAlign: 'right',
                }}
              />
              <span className="tenue" style={{ fontSize: 12 }}>
                s
              </span>
            </span>
          </div>
        </Tarjeta>

        {/* Avisos */}
        <Tarjeta rotulo="AVISOS" ancha>
          <div className="campo">
            <label htmlFor="avisos-canal">Por dónde salen los avisos</label>
            <select
              id="avisos-canal"
              value={ajustes.avisos_canal ?? 'ninguno'}
              onChange={(e) => cambiar('avisos_canal', e.target.value)}
            >
              <option value="ninguno">Por ninguno · solo en pantalla</option>
              <option value="telegram">Por Telegram</option>
              <option value="correo">Por correo</option>
            </select>
            <span className="ayuda">
              A 7 días de que venza una regla sin relevo, el aviso sale también por aquí.
            </span>
          </div>

          {ajustes.avisos_canal === 'telegram' && (
            <div style={{ display: 'grid', gap: 14, marginTop: 16 }}>
              <CampoTexto
                id="avisos-telegram-token"
                etiqueta="Clave del bot de Telegram"
                tipo="password"
                valor={ajustes.avisos_telegram_token ?? ''}
                ayuda="Te la da @BotFather cuando creas el bot."
                alGuardar={(v) => cambiar('avisos_telegram_token', v)}
              />
              <CampoTexto
                id="avisos-telegram-chat"
                etiqueta="A qué conversación llega"
                valor={ajustes.avisos_telegram_chat ?? ''}
                marcador="-1001234567890"
                ayuda="El número de la conversación o del grupo donde quieres el aviso."
                alGuardar={(v) => cambiar('avisos_telegram_chat', v)}
              />
            </div>
          )}

          {ajustes.avisos_canal === 'correo' && (
            <div style={{ display: 'grid', gap: 14, marginTop: 16 }}>
              <CampoTexto
                id="avisos-correo-para"
                etiqueta="A qué correo llega"
                tipo="email"
                valor={ajustes.avisos_correo_para ?? ''}
                marcador="rolando@ejemplo.com"
                alGuardar={(v) => cambiar('avisos_correo_para', v)}
              />
              <CampoTexto
                id="avisos-smtp-servidor"
                etiqueta="Servidor de correo de salida"
                valor={ajustes.avisos_smtp_servidor ?? ''}
                marcador="correo.ejemplo.com:587"
                ayuda="El nombre del servidor y el puerto, separados por dos puntos."
                alGuardar={(v) => cambiar('avisos_smtp_servidor', v)}
              />
              <CampoTexto
                id="avisos-smtp-usuario"
                etiqueta="Usuario de ese correo"
                valor={ajustes.avisos_smtp_usuario ?? ''}
                alGuardar={(v) => cambiar('avisos_smtp_usuario', v)}
              />
              <CampoTexto
                id="avisos-smtp-clave"
                etiqueta="Contraseña de ese correo"
                tipo="password"
                valor={ajustes.avisos_smtp_clave ?? ''}
                ayuda="Se guarda en la máquina de la estación y no sale de ahí."
                alGuardar={(v) => cambiar('avisos_smtp_clave', v)}
              />
            </div>
          )}
        </Tarjeta>

        {/* Fichas de programas */}
        <Tarjeta rotulo="FICHAS DE PROGRAMAS">
          <div className="entre">
            <div>
              <div style={{ font: '500 14.5px var(--sans)' }}>Buscarlas en internet</div>
              <div className="tenue" style={{ fontSize: 12.5, marginTop: 3 }}>
                Buscar sinopsis y carátulas en internet cuando el archivo no las trae
              </div>
            </div>
            <button
              className="interruptor"
              role="switch"
              aria-checked={ajustes.fichas_en_linea === 'si'}
              onClick={() =>
                cambiar('fichas_en_linea', ajustes.fichas_en_linea === 'si' ? 'no' : 'si')
              }
            />
          </div>
          {ajustes.fichas_en_linea === 'si' && (
            <div style={{ marginTop: 16 }}>
              <CampoTexto
                id="clave-tmdb"
                etiqueta="Clave de TMDB"
                tipo="password"
                valor={ajustes.clave_tmdb ?? ''}
                ayuda="opcional; sin clave se usa TVmaze"
                alGuardar={(v) => cambiar('clave_tmdb', v)}
              />
              <p className="ayuda" style={{ marginTop: 10 }}>
                TMDB es gratis para uso no comercial; si esta estación vende
                publicidad, TMDB pide un acuerdo comercial aparte (ver
                COMPLIANCE.md). TVmaze no tiene esa restricción —solo pide
                atribución— y es el que se usa cuando no hay clave.
              </p>
            </div>
          )}
        </Tarjeta>

        {/* Guía */}
        <Tarjeta rotulo="GUÍA">
          <CampoTexto
            id="guia-destino"
            etiqueta="Mandarla también a esta dirección"
            tipo="url"
            valor={ajustes.guia_destino_http ?? ''}
            marcador="https://ejemplo.com/guia"
            ayuda="Además de escribir el archivo, enviar la guía a esta dirección (opcional; si falla no afecta al aire)"
            alGuardar={(v) => cambiar('guia_destino_http', v)}
          />
        </Tarjeta>

        {/* Asistente de IA */}
        <Tarjeta rotulo="ASISTENTE DE IA" ancha>
          <div className="entre">
            <div>
              <div style={{ font: '500 14.5px var(--sans)' }}>Conectar un asistente</div>
              <div className="tenue" style={{ fontSize: 12.5, marginTop: 3 }}>
                Apagado. Antena787 funciona completa sin esto.
              </div>
            </div>
            <button
              className="interruptor"
              role="switch"
              aria-checked={ajustes.asistente_ia === 'encendido'}
              onClick={() =>
                cambiar(
                  'asistente_ia',
                  ajustes.asistente_ia === 'encendido' ? 'apagado' : 'encendido',
                )
              }
            />
          </div>
          <p className="ayuda" style={{ marginTop: 14 }}>
            Si lo enciendes, el asistente puede leer y proponer cambios — nunca poner algo
            al aire ni facturar.
          </p>
        </Tarjeta>
      </div>
    </>
  )
}

function Tarjeta({
  rotulo,
  children,
  ancha,
  problema,
}: {
  rotulo: string
  children: React.ReactNode
  ancha?: boolean
  problema?: boolean
}) {
  return (
    <section
      className="tarjeta"
      style={{
        padding: '18px 22px 22px',
        gridColumn: ancha ? 'span 2' : undefined,
        borderColor: problema ? 'rgba(248,81,73,.5)' : 'var(--borde)',
      }}
    >
      <div className="rotulo" style={{ marginBottom: 16 }}>
        {rotulo}
      </div>
      {children}
    </section>
  )
}

/**
 * Un campo de texto que guarda cuando se sale de él o se aprieta Enter: así no
 * se manda un ajuste por cada tecla.
 */
function CampoTexto({
  id,
  etiqueta,
  valor,
  ayuda,
  marcador,
  tipo = 'text',
  alGuardar,
}: {
  id: string
  etiqueta: string
  valor: string
  ayuda?: string
  marcador?: string
  tipo?: 'text' | 'password' | 'email' | 'url'
  alGuardar: (v: string) => void
}) {
  const [borrador, setBorrador] = useState(valor)
  useEffect(() => setBorrador(valor), [valor])
  return (
    <div className="campo">
      <label htmlFor={id}>{etiqueta}</label>
      <input
        id={id}
        type={tipo}
        autoComplete="off"
        placeholder={marcador}
        value={borrador}
        onChange={(e) => setBorrador(e.target.value)}
        onBlur={() => borrador !== valor && alGuardar(borrador)}
        onKeyDown={(e) => {
          if (e.key === 'Enter') e.currentTarget.blur()
        }}
      />
      {ayuda && <span className="ayuda">{ayuda}</span>}
    </div>
  )
}

function Linea({
  nombre,
  valor,
  verde,
  ambar,
  aqua,
}: {
  nombre: string
  valor: string
  verde?: boolean
  ambar?: boolean
  aqua?: boolean
}) {
  return (
    <div className="entre" style={{ padding: '7px 0', fontSize: 14 }}>
      <span className="apagado">{nombre}</span>
      <span
        style={{
          fontWeight: 500,
          textAlign: 'right',
          color: verde
            ? 'var(--verde)'
            : ambar
              ? 'var(--ambar)'
              : aqua
                ? 'var(--aqua)'
                : 'var(--texto)',
        }}
      >
        {valor}
      </span>
    </div>
  )
}

/** «12 s», o «—» cuando el servidor todavía no manda ese dato (los de la
 * hora y el detector de silencio llegan con el motor, F2). */
function segundos(v: unknown): string {
  return v === undefined || v === null || v === '' ? '—' : `${v} s`
}
