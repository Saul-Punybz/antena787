import type { ReactNode } from 'react'
import { useEffect, useState } from 'react'
import { useEstado } from '../lib/estado'
import { fechaCorta, hora } from '../lib/fechas'

/**
 * El encabezado de la vista de aire: el punto rojo de tally siempre en el
 * mismo sitio, la fecha y la hora del canal, y el modo escrito con todas sus
 * letras además del color. Nunca un icono solo (docs/adr/0008).
 *
 * `accion` va justo al lado de donde se dice el modo: ahí es donde tiene que
 * estar el botón de salir al aire y el de volver a sombra (F2-118), pegado a
 * la frase que dice cómo está el canal.
 */
export function EncabezadoDeAire({ accion }: { accion?: ReactNode }) {
  const { estado, demo } = useEstado()
  const [tic, setTic] = useState(0)
  useEffect(() => {
    const t = window.setInterval(() => setTic((x) => x + 1), 1000)
    return () => window.clearInterval(t)
  }, [])
  void tic

  const zona = estado?.canal.zona_horaria ?? 'UTC'
  const ahora = estado?.ahora ?? new Date().toISOString()
  const modo = estado?.modo ?? 'sombra'
  const alAire = modo === 'aire'

  const salidaMala = estado?.salidas?.some(
    (s) => s.estado_conexion !== 'conectada' && s.estado_conexion !== 'apagada',
  )

  return (
    <header className="encabezado">
      <div className="fila">
        <span className={'rotulo-aire'}>
          <span className={alAire ? 'tally' : 'tally tally--apagado'} aria-hidden />
          AL AIRE
        </span>
        <span className="reloj">
          {fechaCorta(ahora, zona)} · {hora(ahora, zona)}
        </span>
      </div>
      <div className="fila">
        {demo && (
          <span className="franja-modo franja-modo--demo">
            Datos de ejemplo · el servidor no está contestando
          </span>
        )}
        <span className={'franja-modo franja-modo--' + (alAire ? 'aire' : 'sombra')}>
          <span className={'punto ' + (alAire ? 'punto--bien' : 'punto--aviso')} />
          {alAire
            ? salidaMala
              ? 'Al aire · una salida está reintentando'
              : 'Señal saliendo bien'
            : 'Modo sombra · todavía no estás al aire'}
        </span>
        {accion}
      </div>
    </header>
  )
}
