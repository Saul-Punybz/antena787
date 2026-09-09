/** La carátula de un título: un degradado estable y las iniciales encima. */
export function Caratula({
  nombre,
  iniciales,
  alto,
  radio = 0,
}: {
  nombre: string
  iniciales?: string
  alto?: number | string
  radio?: number
}) {
  const semilla = [...nombre].reduce((a, c) => (a * 31 + c.charCodeAt(0)) % 360, 7)
  const a = `hsl(${semilla} 24% 21%)`
  const b = `hsl(${(semilla + 28) % 360} 20% 11%)`
  const letras =
    iniciales ??
    nombre
      .replace(/['’]/g, '')
      .replace(/[^A-Za-z0-9 ]/g, ' ')
      .trim()
      .split(/\s+/)
      .filter(Boolean)
      .slice(0, 2)
      .map((p) => p[0])
      .join('')
      .toUpperCase()
  return (
    <div
      className="caratula"
      style={{
        height: alto,
        width: '100%',
        borderRadius: radio,
        background: `linear-gradient(145deg, ${a}, ${b})`,
      }}
    >
      <span>{letras}</span>
    </div>
  )
}
