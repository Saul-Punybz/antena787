import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router'
import { Caratula } from './Caratula'
import { IconoBuscar } from './Iconos'
import { api } from '../lib/api'
import { duracionLarga, fechaDeRegla, hhMmAMinutos, minutosAHora12 } from '../lib/fechas'
import type { TituloDeBiblioteca } from '../lib/tipos'

// La biblioteca al lado de la parrilla. El problema que resuelve es de vista:
// se está mirando un sábado vacío y, para saber qué se puede poner, había que
// irse a Biblioteca, mirar, acordarse y volver. Arriba va lo que no está
// programado, que es lo que sirve para llenar un hueco.

const LLAVE = 'antena787.parrilla.biblioteca'

/** Se acuerda de si estaba abierta o plegada. Sin localStorage, abierta. */
function leerAbierta(): boolean {
  try {
    const v = window.localStorage.getItem(LLAVE)
    return v === null ? true : v === '1'
  } catch {
    return true
  }
}

function guardarAbierta(abierta: boolean) {
  try {
    window.localStorage.setItem(LLAVE, abierta ? '1' : '0')
  } catch {
    // Una ventana privada: la columna funciona igual, solo que no se acuerda.
  }
}

export function BibliotecaAlLado({
  resaltarSinProgramar,
  alEscoger,
  anio,
  recargar = 0,
}: {
  /** Hay un hueco escogido: lo que no está programado es lo que sirve. */
  resaltarSinProgramar: boolean
  alEscoger: (t: TituloDeBiblioteca) => void
  anio: number
  /** Sube de uno cuando la parrilla cambió: lo sin programar es otro. */
  recargar?: number
}) {
  const [titulos, setTitulos] = useState<TituloDeBiblioteca[] | null>(null)
  const [busqueda, setBusqueda] = useState('')
  const [abierta, setAbierta] = useState(leerAbierta)

  useEffect(() => {
    api.biblioteca().then(setTitulos).catch(() => setTitulos([]))
  }, [recargar])

  function plegar(valor: boolean) {
    setAbierta(valor)
    guardarAbierta(valor)
  }

  const { sinProgramar, enParrilla } = useMemo(() => {
    const q = busqueda.trim().toLowerCase()
    const filtrados = (titulos ?? []).filter((t) => t.nombre.toLowerCase().includes(q))
    return {
      sinProgramar: filtrados.filter((t) => !t.en_la_parrilla),
      enParrilla: filtrados
        .filter((t) => t.en_la_parrilla)
        .sort((a, b) => hhMmAMinutos(a.hora ?? '00:00') - hhMmAMinutos(b.hora ?? '00:00')),
    }
  }, [titulos, busqueda])

  const cuantosLibres = (titulos ?? []).filter((t) => !t.en_la_parrilla).length

  if (!abierta)
    return (
      <aside className="biblio biblio--plegada">
        <button
          className="biblio__pliegue"
          aria-expanded={false}
          aria-label={`Abrir la biblioteca: ${cuantosLibres} títulos que no estás usando`}
          onClick={() => plegar(true)}
        >
          <span className="biblio__pliegue-texto">Biblioteca</span>
          <span className="biblio__cuenta">{cuantosLibres}</span>
        </button>
      </aside>
    )

  return (
    <aside className="biblio" aria-label="Biblioteca">
      <div className="biblio__cabeza">
        <div>
          <div className="rotulo">BIBLIOTECA</div>
          <p className="subtitulo" style={{ fontSize: 12.5 }}>
            {resaltarSinProgramar
              ? 'Escoge uno y la regla abre con el día y la hora del hueco.'
              : 'Lo que hay para poner, sin cambiar de pantalla.'}
          </p>
        </div>
        <button
          className="panel__cerrar"
          aria-expanded
          aria-label="Plegar la biblioteca"
          onClick={() => plegar(false)}
        >
          ×
        </button>
      </div>

      <div className="biblio__buscador">
        <IconoBuscar tamano={15} color="var(--texto-3)" />
        <input
          value={busqueda}
          onChange={(e) => setBusqueda(e.target.value)}
          placeholder={`Buscar en ${titulos?.length ?? 0} títulos…`}
          aria-label="Buscar en la biblioteca"
        />
      </div>

      <div className="biblio__cuerpo">
        {!titulos && <p className="cargando">Buscando los títulos…</p>}

        {titulos && (
          <Estante
            rotulo="SIN PROGRAMAR"
            nota={
              cuantosLibres === 1
                ? '1 título que no estás usando'
                : `${cuantosLibres} títulos que no estás usando`
            }
            notaAmbar
            vacia="Todo lo que hay está en la parrilla."
            titulos={sinProgramar}
            resaltar={resaltarSinProgramar}
            alEscoger={alEscoger}
            anio={anio}
          />
        )}

        {titulos && (
          <Estante
            rotulo="EN LA PARRILLA"
            nota={`${enParrilla.length} ${enParrilla.length === 1 ? 'título' : 'títulos'}`}
            vacia="Todavía no hay nada programado."
            titulos={enParrilla}
            resaltar={false}
            alEscoger={alEscoger}
            anio={anio}
          />
        )}
      </div>
    </aside>
  )
}

function Estante({
  rotulo,
  nota,
  notaAmbar,
  vacia,
  titulos,
  resaltar,
  alEscoger,
  anio,
}: {
  rotulo: string
  nota: string
  notaAmbar?: boolean
  vacia: string
  titulos: TituloDeBiblioteca[]
  resaltar: boolean
  alEscoger: (t: TituloDeBiblioteca) => void
  anio: number
}) {
  return (
    <section>
      {/* En una columna estrecha el rótulo y el conteo no caben en la misma
          línea: el conteo va debajo y se lee entero. */}
      <div style={{ marginBottom: 8 }}>
        <div className="rotulo">{rotulo}</div>
        <div className={notaAmbar ? 'ambar' : 'tenue'} style={{ fontSize: 11.5, marginTop: 2 }}>
          {nota}
        </div>
      </div>
      {titulos.length === 0 ? (
        <p className="ayuda">{vacia}</p>
      ) : (
        <div className="biblio__lista">
          {titulos.map((t) => (
            <Ficha
              key={t.id}
              titulo={t}
              resaltada={resaltar}
              alEscoger={alEscoger}
              anio={anio}
            />
          ))}
        </div>
      )}
    </section>
  )
}

/** La tarjeta compacta: carátula, nombre y lo que hace falta para decidir. */
function Ficha({
  titulo: t,
  resaltada,
  alEscoger,
  anio,
}: {
  titulo: TituloDeBiblioteca
  resaltada: boolean
  alEscoger: (t: TituloDeBiblioteca) => void
  anio: number
}) {
  const esSerie = t.episodios > 1
  return (
    <div className={'biblio__ficha' + (resaltada ? ' biblio__ficha--resaltada' : '')}>
      <button className="biblio__tocar" onClick={() => alEscoger(t)}>
        <div className="biblio__caratula">
          <Caratula nombre={t.nombre} iniciales={t.caratula} alto="100%" radio={5} />
        </div>
        <div className="biblio__datos">
          <div className="biblio__nombre">{t.nombre}</div>
          <div className="biblio__linea">
            {t.tipo} · {duracionLarga(t.duracion_ms)}
            {esSerie ? ` · ${t.episodios} episodios` : ''}
          </div>
          {t.hora && (
            <div className="biblio__linea aqua">
              {minutosAHora12(hhMmAMinutos(t.hora))}
              {t.regla_hasta ? ` · hasta el ${fechaDeRegla(t.regla_hasta, anio)}` : ''}
            </div>
          )}
          {t.estado_material !== 'listo' && (
            <div className="biblio__linea ambar">{t.estado_material}</div>
          )}
        </div>
      </button>
      {t.en_la_parrilla && (
        <Link
          className="biblio__regla"
          to={`/reglas?titulo=${encodeURIComponent(t.nombre)}`}
        >
          ver su regla
        </Link>
      )}
    </div>
  )
}
