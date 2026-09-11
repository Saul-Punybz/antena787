import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router'
import { PestanasDeParrilla } from './Parrilla'
import { BibliotecaAlLado } from '../componentes/BibliotecaAlLado'
import { EditorDeRegla } from '../componentes/EditorDeRegla'
import { IconoChincheta } from '../componentes/Iconos'
import { api } from '../lib/api'
import { useEstado } from '../lib/estado'
import {
  diaLargo,
  diaYMes,
  duracionLarga,
  hhMmAMinutos,
  horasBonitas,
  minutosAHora12,
  partes,
  sumarDias,
} from '../lib/fechas'
import { claveDeRegla, etiquetaDelHueco, usarHueco, type Hueco } from '../lib/huecos'
import { esHueco, type ElementoDelPlan, type FilaDelPlan, type Regla } from '../lib/tipos'

// El día de emisión completo, hora por hora: qué sale, de qué regla viene y
// dónde falta algo. Es la vista con la que se revisa el aire de mañana antes
// de que ocurra (PRD §9 paso 3).

/** El minuto de reloj del canal en que empieza una fila. */
function minutoDe(it: ElementoDelPlan, zona: string): number {
  if (it.hora) return hhMmAMinutos(it.hora)
  if (it.hora_local) return hhMmAMinutos(it.hora_local)
  return partes(it.instante_planeado, zona).minutosDelDia
}

/** Lo que dura, con la frase del servidor si la mandó. */
function duracionDe(it: ElementoDelPlan): string {
  return it.duracion ?? duracionLarga(it.duracion_planeada_ms)
}

/** De dónde viene el bloque, en cristiano. */
function origenDe(it: ElementoDelPlan): string {
  switch (it.origen) {
    case 'relleno':
      return 'relleno'
    case 'cartel':
      return 'cartel de la estación'
    case 'live_source':
      return 'fuente en vivo'
    default:
      return ''
  }
}

export function ParrillaDia() {
  const { estado } = useEstado()
  const zona = estado?.canal.zona_horaria ?? 'UTC'
  const hoy = estado?.dia_emision ?? '2026-09-04'
  const anio = Number(hoy.slice(0, 4))
  const [elegido, setElegido] = useState<string | null>(null)
  const dia = elegido ?? hoy
  const [filas, setFilas] = useState<FilaDelPlan[] | null>(null)
  const [reglas, setReglas] = useState<Regla[]>([])
  // Cada regla guardada cambia el inventario: la columna se vuelve a leer.
  const [cambios, setCambios] = useState(0)
  const { hueco, reglaNueva, abrirHueco, escogerTitulo, cerrar } = usarHueco()

  const recargar = useCallback(async () => {
    const nuevas = await api.plan(dia).catch(() => null)
    if (nuevas) setFilas(nuevas)
  }, [dia])

  useEffect(() => {
    setFilas(null)
    api.plan(dia).then(setFilas).catch(() => setFilas([]))
  }, [dia])

  // Las reglas, para poder decir de cuál sale cada bloque. Se vuelven a leer
  // cuando se guarda una: si no, la recién creada saldría «sin regla».
  useEffect(() => {
    api.reglas().then(setReglas).catch(() => setReglas([]))
  }, [cambios])

  const vacias =
    (filas ?? [])
      .filter(esHueco)
      .reduce((a, h) => a + (Date.parse(h.fin) - Date.parse(h.inicio)), 0) / 3_600_000

  const titulo = diaLargo(dia)

  return (
    <>
      <div className="encabezado">
        <div>
          <h1 className="titulo-pantalla">
            {titulo.charAt(0).toUpperCase() + titulo.slice(1)} {diaYMes(dia)}
          </h1>
          <p className="subtitulo">
            El día de emisión completo, hora por hora. Lo rayado en rojo está vacío: toca
            un vacío y la regla abre a esa hora.
          </p>
        </div>
        <div className="fila" style={{ gap: 10 }}>
          <button
            className="boton"
            aria-label="El día antes"
            onClick={() => setElegido(sumarDias(dia, -1))}
          >
            ←
          </button>
          <button className="boton" disabled={dia === hoy} onClick={() => setElegido(null)}>
            Hoy
          </button>
          <button
            className="boton"
            aria-label="El día después"
            onClick={() => setElegido(sumarDias(dia, 1))}
          >
            →
          </button>
          <PestanasDeParrilla />
        </div>
      </div>

      <div className={'con-biblioteca' + (reglaNueva ? ' con-biblioteca--corrida' : '')}>
        <div>
          {!filas && <p className="cargando">Armando el día…</p>}

          {filas && (
            <>
              <div className="entre" style={{ marginBottom: 12 }}>
                <span className="tenue" style={{ fontSize: 13 }}>
                  {filas.filter((f) => !esHueco(f)).length} bloques
                </span>
                <span
                  style={{
                    font: '600 13px var(--sans)',
                    color: vacias >= 8 ? 'var(--rojo)' : 'var(--ambar)',
                  }}
                >
                  {horasBonitas(Math.round(vacias * 10) / 10)} vacías
                </span>
              </div>

              <ul className="dia-plan">
                {filas.map((f, i) =>
                  esHueco(f) ? (
                    <FilaVacia
                      key={`hueco-${f.inicio}`}
                      dia={dia}
                      zona={zona}
                      inicio={f.inicio}
                      fin={f.fin}
                      escogido={hueco}
                      alTocar={abrirHueco}
                    />
                  ) : (
                    <FilaConAlgo
                      key={f.id ?? `bloque-${i}`}
                      item={f}
                      zona={zona}
                      regla={reglas.find((r) => r.id === f.schedule_rule_id) ?? null}
                    />
                  ),
                )}
                {filas.length === 0 && (
                  <li className="ayuda">
                    El día no tiene nada todavía. Las reglas son las que lo llenan.
                  </li>
                )}
              </ul>
            </>
          )}
        </div>

        <BibliotecaAlLado
          resaltarSinProgramar={Boolean(hueco)}
          alEscoger={escogerTitulo}
          anio={anio}
          recargar={cambios}
        />
      </div>

      {reglaNueva && (
        <EditorDeRegla
          // Escoger otro título o otro hueco vuelve a llenar el formulario:
          // sin la llave, React se queda con lo que había la primera vez.
          key={claveDeRegla(reglaNueva)}
          regla={null}
          inicial={reglaNueva}
          alCerrar={cerrar}
          alGuardar={async () => {
            // El editor ya volvió a armar el plan: aquí solo hay que
            // refrescar lo que se está mirando.
            cerrar()
            // Lo sin programar ya es otro: la columna se vuelve a leer.
            setCambios((n) => n + 1)
            await recargar()
          }}
        />
      )}
    </>
  )
}

/** Un bloque del plan: hora, título, episodio, duración y de qué regla sale. */
function FilaConAlgo({
  item,
  zona,
  regla,
}: {
  item: ElementoDelPlan
  zona: string
  regla: Regla | null
}) {
  const minuto = minutoDe(item, zona)
  const origen = origenDe(item)
  return (
    <li className={'dia-fila' + (item.en_vivo ? ' dia-fila--vivo' : '')}>
      <span className="mono dia-fila__hora">{minutosAHora12(minuto)}</span>
      <div className="dia-fila__que">
        <div className="fila" style={{ gap: 9, flexWrap: 'wrap' }}>
          <span style={{ font: '600 14.5px var(--sans)' }}>
            {item.titulo || origen || 'sin nombre'}
          </span>
          {item.en_vivo && <span className="etiqueta etiqueta--vivo">EN VIVO</span>}
          {item.fijado && (
            <span className="fila tenue" style={{ gap: 5, fontSize: 12 }}>
              <IconoChincheta tamano={12} color="var(--ambar)" grosor={2} />
              <span className="ambar">puesto a mano</span>
            </span>
          )}
        </div>
        {item.episodio !== null && item.episodio !== undefined && item.episodio !== '' && (
          <div className="tenue" style={{ fontSize: 12.5, marginTop: 2 }}>
            {typeof item.episodio === 'number'
              ? `episodio ${item.episodio}`
              : item.episodio}
          </div>
        )}
      </div>
      <span className="mono tenue dia-fila__dura">{duracionDe(item)}</span>
      <span className="dia-fila__regla">
        {regla ? (
          <Link to={`/reglas?titulo=${encodeURIComponent(regla.titulo)}`}>
            regla de {minutosAHora12(regla.hora)}
          </Link>
        ) : (
          <span className="tenue">{origen || 'sin regla'}</span>
        )}
      </span>
    </li>
  )
}

/** Un vacío: se marca igual que en la semana y abre la regla que lo llenaría. */
function FilaVacia({
  dia,
  zona,
  inicio,
  fin,
  escogido,
  alTocar,
}: {
  dia: string
  zona: string
  inicio: string
  fin: string
  escogido: Hueco | null
  alTocar: (h: Hueco) => void
}) {
  const desde = partes(inicio, zona).minutosDelDia
  const hastaCrudo = partes(fin, zona).minutosDelDia
  // Un vacío que cruza la medianoche termina «antes» de donde empieza.
  const hasta = hastaCrudo > desde ? hastaCrudo : hastaCrudo + 24 * 60
  // La regla arranca en la media hora siguiente: nadie programa a las 12:54.
  // Si el vacío es tan corto que ya no cabría, se queda donde empieza.
  const alaMedia = Math.ceil(desde / 30) * 30
  const vacio: Hueco = { dia, desde: alaMedia + 30 <= hasta ? alaMedia : desde, hasta }
  const marcado = escogido?.dia === dia && escogido.desde === vacio.desde

  // Menos de media hora no es un espacio que se programe: ahí no cabe ninguna
  // regla y es lo que el relleno cubre solo. Se enseña, pero no se toca.
  if (hasta - desde < 30)
    return (
      <li className="dia-sobra">
        <span className="mono">{minutosAHora12(desde)}</span>
        <span>
          {hasta - desde} min sin nada · lo llena el relleno hasta las{' '}
          {minutosAHora12(hasta % (24 * 60))}
        </span>
      </li>
    )

  return (
    <li>
      <button
        type="button"
        className={'dia-fila dia-fila--vacia' + (marcado ? ' dia-fila--escogida' : '')}
        aria-label={etiquetaDelHueco(vacio)}
        onClick={() => alTocar(vacio)}
      >
        <span className="mono dia-fila__hora">{minutosAHora12(desde)}</span>
        <span className="dia-fila__que">
          <span className="rojo" style={{ font: '600 14px var(--sans)' }}>
            vacío hasta {minutosAHora12(hasta % (24 * 60))}
          </span>
          <span className="subtitulo" style={{ fontSize: 12.5 }}>
            Lo que no se llena sale en negro. Toca para poner algo aquí.
          </span>
        </span>
        <span className="mono tenue dia-fila__dura">
          {duracionLarga((hasta - desde) * 60_000)}
        </span>
        <span className="dia-fila__regla aqua">poner algo</span>
      </button>
    </li>
  )
}
