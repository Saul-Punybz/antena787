import { useEffect, useState } from 'react'
import { api } from '../lib/api'
import { useEstado } from '../lib/estado'
import type { Ajustes as MapaDeAjustes } from '../lib/tipos'

/**
 * Ajustes son las buenas prácticas vigiladas para siempre, no aplicadas una
 * vez (PRD §13).
 *
 * Regla de esta pantalla: no se pinta ni un dato que el servidor no mande. Lo
 * que llega con el motor se dice con esas palabras, y una tarjeta sin datos
 * enseña «—» o la frase de por qué, nunca «undefined» ni «hace NaN días»
 * (auditoría de contrato, 11 sept 2026).
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

  return (
    <>
      <div className="encabezado">
        <div>
          <h1 className="titulo-pantalla">Ajustes</h1>
          <p className="subtitulo">
            Antena787 v{estado?.version ?? '—'} · la máquina y el tiempo al aire se
            miden cuando arranque el motor
          </p>
        </div>
        <div className="fila" style={{ gap: 12 }}>
          {guardado && <span className="verde" style={{ fontSize: 13 }}>{guardado}</span>}
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
        {/* Salud de la máquina: F2.5 todavía no existe (PRD §13). Antes esta
            tarjeta leía ajustes que el servidor nunca manda y salía roja para
            siempre; mientras no haya quien lo compruebe de verdad, mejor
            decirlo que fingir un dato (auditoría de contrato, 11 sept 2026). */}
        <Tarjeta rotulo="SALUD DE LA MÁQUINA" ancha>
          <p className="ayuda">
            Antivirus, plan de energía, arranque tras corte de luz, rutas largas y
            actualizaciones del sistema se dejan bien puestos a mano al instalar la
            máquina (ver la guía de instalación). Antena787 todavía no los comprueba
            solo: esta tarjeta vuelve cuando pueda.
          </p>
        </Tarjeta>

        {/* Respaldo: la copia de la base ya corre sola cada hora (carpeta_respaldo),
            pero cómo va —cuándo fue la última, cuántas hay— todavía no lo dice
            ningún ajuste. Antes esta tarjeta leía ajustes que el servidor nunca
            manda y pintaba «hace NaN días» (auditoría de contrato, 11 sept 2026). */}
        <Tarjeta rotulo="RESPALDO">
          <p className="ayuda">
            La base se respalda sola cada hora en la carpeta de datos de la
            estación. Cuándo fue la última copia y cuántas quedan guardadas todavía
            no lo enseña ningún ajuste: esta tarjeta vuelve cuando lo haga.
          </p>
        </Tarjeta>

        {/* Actualizaciones */}
        <Tarjeta rotulo="ACTUALIZACIONES">
          <Linea nombre="Versión" valor={estado?.version} />
          <p className="ayuda" style={{ marginTop: 12 }}>
            Un canal al aire no se actualiza solo. Antena787 todavía no busca versiones
            nuevas ni sabe qué trae la que viene: hoy se cambia a mano, se baja la
            versión nueva y se reinicia el servicio, con la copia de la base lista.
          </p>
        </Tarjeta>

        {/* Cumplimiento */}
        <Tarjeta rotulo="CUMPLIMIENTO" ancha>
          <Linea nombre="País" valor={ajustes.pais} />
          <Linea nombre="Perfil" valor={estado?.canal.perfil_regulatorio} />
          <Linea nombre="Calidad de salida" valor={ajustes.calidad} />
          {estado?.canal.perfil_regulatorio === 'us-fcc' && (
            <div className="campo" style={{ marginTop: 16 }}>
              <label htmlFor="subtitulos-estado">¿El canal está obligado a subtitular?</label>
              <select
                id="subtitulos-estado"
                value={ajustes.subtitulos_estado || 'no_se'}
                onChange={(e) => cambiar('subtitulos_estado', e.target.value)}
              >
                <option value="obligada">Estamos obligados a subtitular</option>
                <option value="exenta">Estamos exentos</option>
                <option value="no_se">No lo sé todavía</option>
              </select>
              <span className="ayuda">
                Un canal con ingresos brutos anuales de menos de $3,000,000 el año
                anterior está exento sin pedirle nada a la FCC (47 CFR 79.1(d)(12)).
                Las tres respuestas funcionan igual —los subtítulos que traiga un
                archivo se conservan y se pueden subir siempre— y solo cambian si
                el sistema avisa cuando un programa sale sin subtítulos (ver
                COMPLIANCE.md). "No lo sé todavía" no bloquea nada.
              </span>
            </div>
          )}
        </Tarjeta>

        {/* Acceso remoto */}
        <Tarjeta rotulo="ACCESO REMOTO">
          <p className="ayuda">
            Se entra desde el celular sin abrir nada al internet, con Tailscale. Cómo
            está la red de la máquina no se comprueba desde aquí todavía: el asistente
            de instalación lo dice al detectarla.
          </p>
        </Tarjeta>

        {/* Hora */}
        <Tarjeta rotulo="HORA">
          <Linea nombre="Reloj del canal" valor={estado?.canal.zona_horaria} aqua />
          <p className="ayuda" style={{ marginTop: 12 }}>
            La deriva del reloj se mide contra el aire que de verdad sale, así que llega
            con el motor. Mientras, manda la hora del sistema operativo.
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
          <p className="ayuda">
            No se cree lo que dice la tarjeta: se mide de verdad al arrancar el motor.
            Todavía no hay motor, así que aquí no hay nada que enseñar.
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

        {/* Detector de silencio y negro sobre la salida real (F2-51 a F2-54) */}
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
              aria-checked={ajustes.silencio_devuelve_control !== 'no'}
              onClick={() =>
                cambiar(
                  'silencio_devuelve_control',
                  ajustes.silencio_devuelve_control === 'no' ? 'si' : 'no',
                )
              }
            />
          </div>
          <UmbralEnSegundos
            id="umbral"
            rotulo="Umbral de silencio"
            valor={ajustes.silencio_umbral_s}
            onCambio={(v) => cambiar('silencio_umbral_s', v)}
          />
          <UmbralEnSegundos
            id="umbral-negro"
            rotulo="Umbral de negro"
            valor={ajustes.negro_umbral_s}
            onCambio={(v) => cambiar('negro_umbral_s', v)}
          />
          <div className="tenue" style={{ fontSize: 12.5, marginTop: 14 }}>
            Se mide sobre lo que de verdad sale —audio bajo −60 dBFS o luma bajo 16—, no
            sobre el plan. Un archivo marcado «abre en negro a propósito» no lo dispara.
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

/**
 * Una fila «nombre → valor». Lo que el servidor no manda sale «—»: en blanco
 * parecía que la pantalla se había roto (auditoría de contrato, 11 sept 2026).
 */
function Linea({
  nombre,
  valor,
  verde,
  ambar,
  aqua,
}: {
  nombre: string
  valor: string | null | undefined
  verde?: boolean
  ambar?: boolean
  aqua?: boolean
}) {
  const hay = valor !== null && valor !== undefined && valor !== ''
  return (
    <div className="entre" style={{ padding: '7px 0', fontSize: 14 }}>
      <span className="apagado">{nombre}</span>
      <span
        style={{
          fontWeight: 500,
          textAlign: 'right',
          color: !hay
            ? 'var(--texto-3)'
            : verde
              ? 'var(--verde)'
              : ambar
                ? 'var(--ambar)'
                : aqua
                  ? 'var(--aqua)'
                  : 'var(--texto)',
        }}
      >
        {hay ? valor : '—'}
      </span>
    </div>
  )
}

/** «12 s», o «—» cuando el servidor todavía no manda ese dato (los de la
 * hora llegan con el motor, F2). */
function segundos(v: unknown): string {
  return v === undefined || v === null || v === '' ? '—' : `${v} s`
}

/** Un umbral en segundos, con los mismos límites que valida el servidor
 * (de 3 a 120: `app.UmbralValido`). */
function UmbralEnSegundos({
  id,
  rotulo,
  valor,
  onCambio,
}: {
  id: string
  rotulo: string
  valor: string | undefined
  onCambio: (v: string) => void
}) {
  return (
    <div className="entre" style={{ marginTop: 20 }}>
      <label htmlFor={id} style={{ font: '400 14px var(--sans)' }}>
        {rotulo}
      </label>
      <span className="fila" style={{ gap: 7 }}>
        <input
          id={id}
          type="number"
          min={3}
          max={120}
          value={valor ?? ''}
          onChange={(e) => onCambio(e.target.value)}
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
  )
}
