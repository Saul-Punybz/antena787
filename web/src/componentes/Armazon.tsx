import { NavLink, Outlet } from 'react-router'
import { useEstado } from '../lib/estado'
import {
  IconoAjustes,
  IconoAntena,
  IconoAnuncios,
  IconoBiblioteca,
  IconoParrilla,
  IconoReglas,
} from './Iconos'

// Salidas (a dónde manda el canal su señal) no está aquí: configurarla es
// instalación, no algo que se toque a diario. Se llega desde Ajustes
// («A dónde va la señal»), pero la ruta /salidas sigue viva (F1-57).
const MENU = [
  { a: '/al-aire', texto: 'Al aire', Icono: IconoAntena },
  { a: '/parrilla', texto: 'Parrilla', Icono: IconoParrilla },
  { a: '/reglas', texto: 'Reglas', Icono: IconoReglas },
  { a: '/biblioteca', texto: 'Biblioteca', Icono: IconoBiblioteca },
  { a: '/anuncios', texto: 'Anuncios', Icono: IconoAnuncios, soloConAnunciantes: true },
  { a: '/ajustes', texto: 'Ajustes', Icono: IconoAjustes },
]

/**
 * El armazón: menú lateral fijo, contenido a la derecha y el modo del canal
 * pintado como color de fondo del app entero. Nunca hay un modal encima:
 * lo que se edita se abre en un panel al lado (docs/adr/0008).
 */
export function Armazon() {
  const { estado } = useEstado()
  const modo = estado?.modo ?? 'sombra'
  const hayAnunciantes = estado?.hay_anunciantes ?? false

  return (
    <div className="app" data-modo={modo}>
      <nav className="lateral">
        <div className="lateral__marca">
          <IconoAntena tamano={25} color="var(--aqua)" grosor={1.8} />
          <span>
            Antena<span>787</span>
          </span>
        </div>
        <div className="lateral__menu">
          {MENU.filter((m) => !m.soloConAnunciantes || hayAnunciantes).map(
            ({ a, texto, Icono }) => (
              <NavLink
                key={a}
                to={a}
                className={({ isActive }) =>
                  'lateral__enlace' + (isActive ? ' lateral__enlace--activo' : '')
                }
              >
                {({ isActive }) => (
                  <>
                    <Icono color={isActive ? 'var(--aqua)' : 'var(--texto-3)'} />
                    <span>{texto}</span>
                  </>
                )}
              </NavLink>
            ),
          )}
        </div>
        <dl className="lateral__pie">
          <dt>CANAL</dt>
          <dd>{estado?.canal.nombre ?? '—'}</dd>
          {/*
            Cómo está el canal se ve desde cualquier pantalla, no solo en Al
            aire: nadie tiene que ir a mirar si está emitiendo (F2-118).
          */}
          <dt style={{ marginTop: 12 }}>AHORA MISMO</dt>
          <dd className="fila" style={{ gap: 8 }}>
            <span className={'punto ' + (modo === 'aire' ? 'punto--bien' : 'punto--aviso')} />
            {modo === 'aire' ? 'Al aire' : 'Modo sombra'}
          </dd>
        </dl>
      </nav>
      <main className="contenido">
        <Outlet />
      </main>
    </div>
  )
}
