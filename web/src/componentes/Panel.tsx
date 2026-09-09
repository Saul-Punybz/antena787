import type { ReactNode } from 'react'
import { useEffect } from 'react'

/**
 * Un panel al lado, nunca un modal encima
 * (docs/adr/0008-never-interrupt-air.md). No tapa la pantalla, no bloquea el
 * fondo y no oscurece nada: la vista de aire sigue viéndose y actualizándose.
 */
export function Panel({
  titulo,
  descripcion,
  alCerrar,
  pie,
  children,
}: {
  titulo: string
  descripcion?: string
  alCerrar: () => void
  pie?: ReactNode
  children: ReactNode
}) {
  useEffect(() => {
    const escuchar = (e: KeyboardEvent) => {
      if (e.key === 'Escape') alCerrar()
    }
    window.addEventListener('keydown', escuchar)
    return () => window.removeEventListener('keydown', escuchar)
  }, [alCerrar])

  return (
    <aside className="panel" aria-label={titulo}>
      <div className="panel__cabeza">
        <div>
          <h2 style={{ font: '600 17px var(--sans)', margin: 0 }}>{titulo}</h2>
          {descripcion && <p className="subtitulo">{descripcion}</p>}
        </div>
        <button className="panel__cerrar" onClick={alCerrar} aria-label="Cerrar">
          ×
        </button>
      </div>
      <div className="panel__cuerpo">{children}</div>
      {pie && <div className="panel__pie">{pie}</div>}
    </aside>
  )
}
