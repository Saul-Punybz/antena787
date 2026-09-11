import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router'
import { Panel } from './Panel'
import { api } from '../lib/api'
import { useEstado } from '../lib/estado'
import type { Comprobacion, ComprobacionesDelAire } from '../lib/tipos'
import { ErrorDeApi } from '../lib/tipos'

/**
 * La puerta del aire (F2-118): el botón que saca el canal de sombra y el que
 * lo devuelve, cada uno con su panel al lado —nunca un modal encima
 * (docs/adr/0008)— con las comprobaciones, lo que va a pasar dicho sin
 * rodeos, y la confirmación escrita con la mano.
 *
 * Lo único que este componente decide es cuándo dejar apretar el botón de
 * confirmar. Todo lo que se lee —el resultado de cada comprobación, qué hacer
 * si falta algo— lo escribe el servidor: la pantalla no inventa frases sobre
 * el aire.
 *
 * PENDIENTE: `lib/api.ts` lo mantiene otra mano y todavía no expone las tres
 * llamadas del aire. Van escritas aquí como si existieran; en `api.ts` faltan:
 *
 *   comprobacionesDelAire: () => pedir<ComprobacionesDelAire>('/canal/comprobaciones'),
 *   salirAlAire: (confirmacion: string) =>
 *     pedir<ComprobacionesDelAire>('/canal/al-aire', conCuerpo('POST', { confirmacion })),
 *   volverASombra: (confirmacion: string) =>
 *     pedir<ComprobacionesDelAire>('/canal/a-sombra', conCuerpo('POST', { confirmacion })),
 *
 * con `import type { ComprobacionesDelAire } from './tipos'` (el tipo ya está
 * en `lib/tipos.ts`).
 */
export function PuertaDelAire() {
  const { estado, refrescar } = useEstado()
  const alAire = estado?.modo === 'aire'
  const [abierto, setAbierto] = useState(false)

  if (!estado) return null
  return (
    <>
      <button
        className={'boton' + (alAire ? '' : ' boton--primario')}
        onClick={() => setAbierto(true)}
      >
        {alAire ? 'Volver a modo sombra' : 'Salir al aire'}
      </button>
      {abierto &&
        (alAire ? (
          <PanelASombra
            alCerrar={() => setAbierto(false)}
            alHacerlo={() => {
              setAbierto(false)
              refrescar()
            }}
          />
        ) : (
          <PanelAlAire
            alCerrar={() => setAbierto(false)}
            alHacerlo={() => {
              setAbierto(false)
              refrescar()
            }}
          />
        ))}
    </>
  )
}

/** Lo que hay que escribir. Lo mismo que espera el servidor. */
const PALABRA_AL_AIRE = 'AL AIRE'
const PALABRA_SOMBRA = 'SOMBRA'

/** Escrito como lo compara el servidor: sin acentos, en mayúsculas. */
function normalizar(v: string): string {
  return v
    .trim()
    .toUpperCase()
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .replace(/\s+/g, ' ')
}

/** El panel de encender: las comprobaciones primero, el botón al final. */
function PanelAlAire({
  alCerrar,
  alHacerlo,
}: {
  alCerrar: () => void
  alHacerlo: () => void
}) {
  const [lista, setLista] = useState<ComprobacionesDelAire | null>(null)
  const [escrito, setEscrito] = useState('')
  const [error, setError] = useState('')
  const [enviando, setEnviando] = useState(false)

  const mirar = useCallback(() => {
    api
      .comprobacionesDelAire()
      .then(setLista)
      .catch((e: unknown) =>
        setError(e instanceof ErrorDeApi ? e.message : 'No pude hacer las comprobaciones.'),
      )
  }, [])
  useEffect(mirar, [mirar])

  const salir = () => {
    setEnviando(true)
    setError('')
    api
      .salirAlAire(escrito)
      .then(alHacerlo)
      .catch((e: unknown) => {
        setError(e instanceof ErrorDeApi ? e.message : 'No se pudo salir al aire.')
        // Lo que falta puede haber cambiado mientras el panel estaba abierto.
        mirar()
      })
      .finally(() => setEnviando(false))
  }

  const puede = lista?.puede ?? false
  const listo = puede && normalizar(escrito) === PALABRA_AL_AIRE && !enviando

  return (
    <Panel
      titulo="Salir al aire"
      descripcion="Antes de encender, esto es lo que se ha mirado."
      alCerrar={alCerrar}
      pie={
        <>
          <button className="boton" onClick={alCerrar}>
            Cancelar
          </button>
          <button className="boton boton--primario" disabled={!listo} onClick={salir}>
            {enviando ? 'Encendiendo…' : 'Salir al aire'}
          </button>
        </>
      }
    >
      {lista ? <ListaDeComprobaciones lista={lista.comprobaciones} /> : <p className="subtitulo">Mirando…</p>}

      <div className={'tarjeta ' + (puede ? 'tarjeta--aviso' : 'tarjeta--problema')} style={{ padding: '14px 16px' }}>
        <div style={{ font: '600 14.5px var(--sans)' }}>Qué va a pasar</div>
        <p className="subtitulo" style={{ marginTop: 6 }}>
          {puede ? (
            <>
              Al confirmar, la señal empieza a salir de verdad hacia el equipo que tienes
              configurado, y sigue saliendo hasta que la vuelvas a modo sombra. Lo que esté en
              la parrilla es lo que va a ver la gente.
            </>
          ) : (
            <>
              Todavía no se puede encender: arriba está lo que falta y qué hacer. Nada de esto
              cambia el aire hasta que se arregle.
            </>
          )}
        </p>
      </div>

      <div className="campo">
        <label htmlFor="confirmar-al-aire">Escribe AL AIRE para confirmar</label>
        <input
          id="confirmar-al-aire"
          type="text"
          autoComplete="off"
          value={escrito}
          placeholder={PALABRA_AL_AIRE}
          disabled={!puede}
          onChange={(e) => setEscrito(e.target.value)}
        />
        <span className="ayuda">
          Se pide escribirlo porque encender un transmisor no se deshace apretando otra vez.
        </span>
      </div>

      {error && <div className="error-en-cristiano">{error}</div>}
    </Panel>
  )
}

/** El panel de apagar. La misma seriedad: apagar deja a la gente sin canal. */
function PanelASombra({
  alCerrar,
  alHacerlo,
}: {
  alCerrar: () => void
  alHacerlo: () => void
}) {
  const [escrito, setEscrito] = useState('')
  const [error, setError] = useState('')
  const [enviando, setEnviando] = useState(false)
  const { estado } = useEstado()

  const volver = () => {
    setEnviando(true)
    setError('')
    api
      .volverASombra(escrito)
      .then(alHacerlo)
      .catch((e: unknown) =>
        setError(e instanceof ErrorDeApi ? e.message : 'No se pudo volver a modo sombra.'),
      )
      .finally(() => setEnviando(false))
  }

  const listo = normalizar(escrito) === PALABRA_SOMBRA && !enviando

  return (
    <Panel
      titulo="Volver a modo sombra"
      descripcion="La señal deja de salir."
      alCerrar={alCerrar}
      pie={
        <>
          <button className="boton" onClick={alCerrar}>
            Cancelar
          </button>
          <button className="boton boton--primario" disabled={!listo} onClick={volver}>
            {enviando ? 'Apagando…' : 'Volver a modo sombra'}
          </button>
        </>
      }
    >
      <div className="tarjeta tarjeta--problema" style={{ padding: '14px 16px' }}>
        <div style={{ font: '600 14.5px var(--sans)' }}>Qué va a pasar</div>
        <p className="subtitulo" style={{ marginTop: 6 }}>
          Al confirmar, Antena787 deja de mandar señal
          {estado?.salidas?.length
            ? ' hacia ' + estado.salidas.map((s) => `«${s.nombre}»`).join(' y ')
            : ' hacia el equipo configurado'}
          . Quien esté viendo el canal va a ver lo que haga tu equipo cuando deja de recibir
          señal: normalmente, negro. El plan y la guía se siguen armando igual.
        </p>
      </div>

      <div className="campo">
        <label htmlFor="confirmar-sombra">Escribe SOMBRA para confirmar</label>
        <input
          id="confirmar-sombra"
          type="text"
          autoComplete="off"
          value={escrito}
          placeholder={PALABRA_SOMBRA}
          onChange={(e) => setEscrito(e.target.value)}
        />
        <span className="ayuda">
          Volver al aire es otro botón y otras comprobaciones: no se apaga y se enciende por
          descuido.
        </span>
      </div>

      {error && <div className="error-en-cristiano">{error}</div>}
    </Panel>
  )
}

/**
 * Las comprobaciones, una por fila, con su punto de color y la frase del
 * servidor. La que falta lleva además qué hacer y la pantalla donde se hace.
 */
function ListaDeComprobaciones({ lista }: { lista: Comprobacion[] }) {
  return (
    <ul style={{ listStyle: 'none', margin: 0, padding: 0, display: 'grid', gap: 14 }}>
      {lista.map((c) => (
        <li key={c.clave} className="fila" style={{ alignItems: 'flex-start', gap: 11 }}>
          <span
            className={
              'punto punto--' +
              (c.resultado === 'bien' ? 'bien' : c.resultado === 'aviso' ? 'aviso' : 'problema')
            }
            style={{ marginTop: 6 }}
          />
          <div className="crece">
            <div style={{ font: '600 14px var(--sans)' }}>{c.nombre}</div>
            <div className="subtitulo" style={{ marginTop: 2 }}>
              {c.texto}
            </div>
            {c.arreglo && (
              <div className="ayuda" style={{ marginTop: 4 }}>
                {c.arreglo}{' '}
                {c.ruta && (
                  <Link to={c.ruta} style={{ fontSize: 13 }}>
                    ir ahí
                  </Link>
                )}
              </div>
            )}
          </div>
        </li>
      ))}
    </ul>
  )
}
