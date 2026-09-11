import { NavLink, Outlet } from 'react-router'

/** Parrilla tiene cuatro vistas de lo mismo: Día, Semana, Mes y Guía. */
export function Parrilla() {
  return <Outlet />
}

const VISTAS = [
  { a: '/parrilla/dia', texto: 'Día' },
  { a: '/parrilla', texto: 'Semana', fin: true },
  { a: '/parrilla/mes', texto: 'Mes' },
  { a: '/parrilla/guia', texto: 'Guía' },
]

/**
 * Las cuatro vistas, en una sola fila y con la de encima inequívoca: antes
 * eran cuatro pastillas iguales y no se sabía en cuál estabas.
 */
export function PestanasDeParrilla() {
  return (
    <nav className="pestanas" aria-label="Vistas de la parrilla">
      {VISTAS.map((v) => (
        <NavLink
          key={v.a}
          to={v.a}
          end={v.fin}
          className={({ isActive }) => 'pestana' + (isActive ? ' pestana--activa' : '')}
        >
          {v.texto}
        </NavLink>
      ))}
    </nav>
  )
}
