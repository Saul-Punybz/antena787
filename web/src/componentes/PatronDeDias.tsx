const LETRAS = 'LMMJVSD'

/** Las siete pills del patrón. Índice 0 = lunes, "_" = ese día no. */
export function PatronDeDias({
  patron,
  alCambiar,
}: {
  patron: string
  alCambiar?: (nuevo: string) => void
}) {
  const relleno = (patron + '_______').slice(0, 7)
  return (
    <div className="dias" role={alCambiar ? 'group' : undefined} aria-label="Días">
      {LETRAS.split('').map((letra, i) => {
        const encendido = relleno[i] !== '_' && relleno[i] !== ' '
        const clase = `dias__dia${encendido ? ' dias__dia--encendido' : ''}`
        if (!alCambiar)
          return (
            <span key={i} className={clase}>
              {letra}
            </span>
          )
        return (
          <button
            key={i}
            type="button"
            className={clase}
            aria-pressed={encendido}
            title={
              ['lunes', 'martes', 'miércoles', 'jueves', 'viernes', 'sábado', 'domingo'][i]
            }
            onClick={() => {
              const arr = relleno.split('')
              arr[i] = encendido ? '_' : LETRAS[i]
              alCambiar(arr.join(''))
            }}
          >
            {letra}
          </button>
        )
      })}
    </div>
  )
}
