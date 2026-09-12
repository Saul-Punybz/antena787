import { useEffect, useRef, useState } from 'react'
import type { Monitor } from '../lib/tipos'

/**
 * El monitor: ver lo que el canal está produciendo, sin salir de la pantalla
 * (F2-117).
 *
 * **Qué es y qué no, porque la diferencia importa.** Esto enseña los cuadros
 * que el canal está produciendo ahora mismo, comprimidos en H.264 para que un
 * navegador pueda pintarlos — **ningún navegador sabe decodificar MPEG-2**,
 * que es lo que va al transmisor. Son los mismos cuadros por otro camino.
 *
 * Lo que **no** es: la prueba de que la señal salió por la antena. Para eso
 * hace falta el retorno de aire, que es una tarjeta o un receptor mirando lo
 * que se emitió de verdad. Si esto se confundiera, alguien creería que está
 * verificando su transmisor cuando está verificando nuestro software, y esa es
 * exactamente la clase de error que se descubre el peor día.
 *
 * Por eso la etiqueta dice «lo que estás produciendo» y no «lo que sale».
 */
export function MonitorDeAire({ monitor, enSombra }: { monitor?: Monitor; enSombra: boolean }) {
  const video = useRef<HTMLVideoElement>(null)
  const [error, setError] = useState('')
  const [cargando, setCargando] = useState(false)

  useEffect(() => {
    if (enSombra || !monitor?.hay || !monitor.url || !video.current) return
    let vivo = true
    let reproductor: { destroy: () => void } | null = null

    setCargando(true)
    setError('')
    // La librería se carga solo cuando de verdad hace falta: quien nunca abra
    // el monitor no paga sus 200 KB al arrancar la interfaz.
    import('mpegts.js')
      .then(({ default: mpegts }) => {
        if (!vivo || !video.current) return
        if (!mpegts.getFeatureList().mseLivePlayback) {
          setError('este navegador no sabe pintar la señal. Prueba con Chrome o Edge.')
          setCargando(false)
          return
        }
        const p = mpegts.createPlayer(
          { type: 'mpegts', isLive: true, url: monitor.url! },
          // Lo que mantiene el retraso bajo: no acumular buffer. Un monitor
          // con diez segundos de retraso no sirve para vigilar nada.
          { enableStashBuffer: false, liveBufferLatencyChasing: true, lazyLoad: false },
        )
        reproductor = p
        p.attachMediaElement(video.current)
        p.on(mpegts.Events.ERROR, () => {
          if (vivo) {
            setError('se perdió la conexión con la señal. Se vuelve a intentar al reabrir esta pantalla.')
            setCargando(false)
          }
        })
        p.load()
        // Los navegadores no dejan sonar sin que alguien haya tocado la
        // página. No es un fallo: se pinta igual, y el sonido llega con el
        // primer clic, así que el rechazo se traga a propósito.
        void Promise.resolve(p.play()).catch(() => {})
        setCargando(false)
      })
      .catch(() => {
        if (vivo) {
          setError('no se pudo cargar el reproductor')
          setCargando(false)
        }
      })

    return () => {
      vivo = false
      reproductor?.destroy()
    }
  }, [monitor?.hay, monitor?.url, enSombra])

  if (enSombra) return null

  if (!monitor?.hay) {
    return (
      <div style={cajaVacia}>
        <div style={{ font: '600 16px var(--sans)', color: 'var(--texto-2)' }}>
          No hay monitor todavía
        </div>
        <div style={{ font: '400 14px var(--sans)', color: 'var(--texto-3)', maxWidth: 460 }}>
          {monitor?.porque ??
            'Todavía no hay una salida para ver la señal desde el navegador.'}{' '}
          Se crea en Ajustes → A dónde va la señal, escogiendo «Para verlo desde otra
          computadora» y marcando que es para un navegador.
        </div>
      </div>
    )
  }

  return (
    <>
      <video
        ref={video}
        muted
        playsInline
        style={{ width: '100%', height: '100%', objectFit: 'contain', background: '#000' }}
      />
      {(cargando || error) && (
        <div style={cajaVacia}>
          <div style={{ font: '400 14px var(--sans)', color: error ? 'var(--ambar)' : 'var(--texto-3)' }}>
            {error || 'Conectando con la señal…'}
          </div>
        </div>
      )}
      {/*
        La etiqueta no es decoración: es lo que impide que alguien crea que
        esto prueba que la señal salió por la antena.
      */}
      <span
        style={{
          position: 'absolute',
          left: 14,
          bottom: 12,
          padding: '5px 10px',
          borderRadius: 6,
          background: 'rgba(11,14,18,.82)',
          border: '1px solid var(--borde)',
          font: '600 11px var(--mono)',
          letterSpacing: 1.1,
          color: 'var(--texto-3)',
        }}
        title="Son los mismos cuadros que van al transmisor, comprimidos en H.264 para que el navegador pueda pintarlos. Ver lo que salió por la antena es el retorno de aire, que es otra cosa."
      >
        LO QUE ESTÁS PRODUCIENDO
      </span>
    </>
  )
}

const cajaVacia: React.CSSProperties = {
  position: 'absolute',
  inset: 0,
  display: 'flex',
  flexDirection: 'column',
  alignItems: 'center',
  justifyContent: 'center',
  textAlign: 'center',
  padding: 24,
  gap: 8,
}
