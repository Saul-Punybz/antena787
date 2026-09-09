import { useEffect, useState } from 'react'
import { api } from '../lib/api'
import { IconoAntena, IconoOk } from '../componentes/Iconos'
import type { Instalacion } from '../lib/tipos'

/**
 * Placeholder del asistente de instalación (PRD §13). Los nueve pasos, seis
 * preguntas. La lógica de cada paso la construye otro agente; aquí queda la
 * ruta, el orden y la prueba de barras, que es el paso más importante del
 * producto.
 */
const PASOS = [
  {
    n: 1,
    titulo: '¿Cómo se llama tu canal, y quién entra aquí?',
    detalle:
      'Nombre, identificativo y comunidad de licencia — con eso se genera el cartel de respaldo — y la clave de estación de cuatro a seis dígitos.',
    pregunta: true,
  },
  {
    n: 2,
    titulo: '¿Qué vas a hacer?',
    detalle: 'Internet · Transmisor · No sé',
    pregunta: true,
  },
  {
    n: 3,
    titulo: 'Revisión automática',
    detalle: 'El sistema mira la máquina y la red y cuenta lo que encontró. No pregunta nada.',
    pregunta: false,
  },
  {
    n: 4,
    titulo: '¿A dónde va tu señal, y puedes verla de vuelta?',
    detalle:
      'Nunca se pide escoger nada técnico. La segunda mitad es el retorno de aire; "todavía no" es una respuesta válida.',
    pregunta: true,
  },
  {
    n: 5,
    titulo: '¿Ves las barras de color?',
    detalle:
      'El sistema manda barras y tono a la salida y hace la única pregunta que cualquiera puede contestar. Es el paso más importante del producto.',
    pregunta: true,
    barras: true,
  },
  {
    n: 6,
    titulo: '¿Cómo se ve tu canal?',
    detalle: 'País y calidad: 480i, 720p, 1080i o 1080p, a 29.97/59.94 o 25/50.',
    pregunta: true,
  },
  {
    n: 7,
    titulo: 'Tu contenido',
    detalle: 'Arrastrar los videos o señalar la carpeta donde ya están.',
    pregunta: true,
  },
  {
    n: 8,
    titulo: 'Tu primera parrilla',
    detalle: 'El sistema propone la semana completa. No pregunta nada.',
    pregunta: false,
  },
  { n: 9, titulo: 'Al aire', detalle: 'Un botón.', pregunta: false },
]

export function Asistente() {
  const [instalacion, setInstalacion] = useState<Instalacion | null>(null)

  useEffect(() => {
    api.instalacion().then(setInstalacion).catch(() => setInstalacion(null))
  }, [])

  const actual = instalacion?.paso ?? 1

  return (
    <div style={{ maxWidth: 820, margin: '0 auto', padding: '48px 24px 64px' }}>
      <div className="fila" style={{ gap: 10, marginBottom: 26 }}>
        <IconoAntena tamano={26} color="var(--aqua)" grosor={1.8} />
        <span style={{ font: '600 19px var(--sans)', letterSpacing: '-0.3px' }}>
          Antena<span className="aqua">787</span>
        </span>
      </div>

      <h1 className="titulo-pantalla">Vamos a poner tu canal al aire</h1>
      <p className="subtitulo">
        Nueve pasos, seis preguntas. Lo que la máquina puede averiguar sola, no se
        pregunta.
      </p>

      {instalacion && Object.keys(instalacion.detectado).length > 0 && (
        <div className="tarjeta" style={{ padding: '16px 18px', marginTop: 22 }}>
          <div className="rotulo">LO QUE YA SE ENCONTRÓ SOLO</div>
          <ul style={{ margin: '10px 0 0', padding: 0, listStyle: 'none', display: 'grid', gap: 7 }}>
            {Object.entries(instalacion.detectado).map(([k, v]) => (
              <li key={k} className="fila" style={{ gap: 9 }}>
                <IconoOk tamano={15} color="var(--verde)" />
                <span style={{ fontSize: 13.5 }}>
                  <span className="apagado">{k.replace(/_/g, ' ')}: </span>
                  {v}
                </span>
              </li>
            ))}
          </ul>
        </div>
      )}

      <ol style={{ listStyle: 'none', padding: 0, margin: '22px 0 0', display: 'grid', gap: 10 }}>
        {PASOS.map((p) => (
          <li
            key={p.n}
            className={'tarjeta' + (p.n === actual ? ' tarjeta--aviso' : '')}
            style={{ padding: '15px 18px', display: 'flex', gap: 14 }}
          >
            <span
              className="mono"
              style={{
                width: 28,
                height: 28,
                borderRadius: 8,
                flexShrink: 0,
                display: 'inline-flex',
                alignItems: 'center',
                justifyContent: 'center',
                background: p.n === actual ? 'var(--aqua)' : 'var(--superficie-2)',
                color: p.n === actual ? '#06141a' : 'var(--texto-3)',
                fontWeight: 600,
                fontSize: 13,
              }}
            >
              {p.n}
            </span>
            <div>
              <div style={{ font: '600 15px var(--sans)' }}>
                {p.titulo}{' '}
                {!p.pregunta && (
                  <span className="etiqueta etiqueta--nota">no pregunta nada</span>
                )}
              </div>
              <p className="subtitulo" style={{ marginTop: 4 }}>
                {p.detalle}
              </p>
              {p.barras && (
                <div
                  style={{
                    marginTop: 12,
                    height: 64,
                    borderRadius: 8,
                    overflow: 'hidden',
                    display: 'flex',
                  }}
                >
                  {['#c0c0c0', '#c0c000', '#00c0c0', '#00c000', '#c000c0', '#c00000', '#0000c0'].map(
                    (c) => (
                      <div key={c} style={{ flex: 1, background: c }} />
                    ),
                  )}
                </div>
              )}
              {p.barras && (
                <div className="fila" style={{ marginTop: 12, gap: 10 }}>
                  <button className="boton boton--primario" disabled>
                    Sí, las veo
                  </button>
                  <button className="boton" disabled>
                    No veo nada
                  </button>
                </div>
              )}
            </div>
          </li>
        ))}
      </ol>

      <p className="ayuda" style={{ marginTop: 22, color: 'var(--texto-3)' }}>
        El asistente todavía no está construido: esta pantalla lista los pasos y
        reserva la ruta. Si la biblioteca de relleno está vacía, aquí es donde el
        sistema lo avisa y ofrece crear un relleno por defecto de un clic.
      </p>
    </div>
  )
}
