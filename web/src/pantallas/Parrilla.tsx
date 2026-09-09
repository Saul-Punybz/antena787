import { NavLink, Outlet } from 'react-router'

/** Parrilla tiene tres vistas de lo mismo: Semana, Mes y Guía. */
export function Parrilla() {
  return <Outlet />
}

export function PestanasDeParrilla() {
  return (
    <div className="pastillas" style={{ flexShrink: 0 }}>
      {[
        { a: '/parrilla', texto: 'Semana', fin: true },
        { a: '/parrilla/mes', texto: 'Mes' },
        { a: '/parrilla/guia', texto: 'Guía' },
      ].map((p) => (
        <NavLink
          key={p.a}
          to={p.a}
          end={p.fin}
          className={({ isActive }) => 'pastilla' + (isActive ? ' pastilla--activa' : '')}
          style={{ fontWeight: 600 }}
        >
          {p.texto}
        </NavLink>
      ))}
    </div>
  )
}
