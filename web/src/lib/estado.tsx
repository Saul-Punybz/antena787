import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react'
import { alCambiarDeModo, api, estaEnDemo, suscribirseAlEstado } from './api'
import type { Estado } from './tipos'

interface Contexto {
  estado: Estado | null
  cargando: boolean
  demo: boolean
  necesitaInstalacion: boolean
  sinSesion: boolean
  refrescar: () => void
}

const Ctx = createContext<Contexto>({
  estado: null,
  cargando: true,
  demo: false,
  necesitaInstalacion: false,
  sinSesion: false,
  refrescar: () => {},
})

export function ProveedorDeEstado({ children }: { children: ReactNode }) {
  const [estado, setEstado] = useState<Estado | null>(null)
  const [cargando, setCargando] = useState(true)
  const [demo, setDemo] = useState(estaEnDemo())
  const [necesitaInstalacion, setNecesita] = useState(false)
  const [sinSesion, setSinSesion] = useState(false)
  const [ronda, setRonda] = useState(0)

  useEffect(() => alCambiarDeModo(setDemo), [])

  useEffect(() => {
    let vivo = true
    setCargando(true)
    api
      .estado()
      .then((e) => {
        if (!vivo) return
        setDemo(estaEnDemo())
        if (e.necesita_instalacion) {
          setNecesita(true)
          setEstado(null)
        } else {
          setNecesita(false)
          setEstado(e)
        }
        setSinSesion(false)
      })
      .catch((err: unknown) => {
        if (!vivo) return
        const codigo = (err as { estado?: number }).estado
        if (codigo === 401) setSinSesion(true)
      })
      .finally(() => vivo && setCargando(false))
    return () => {
      vivo = false
    }
  }, [ronda])

  useEffect(() => {
    if (necesitaInstalacion || sinSesion || cargando) return
    return suscribirseAlEstado((e) => {
      setEstado(e)
      setDemo(estaEnDemo())
    })
  }, [necesitaInstalacion, sinSesion, cargando])

  const valor = useMemo<Contexto>(
    () => ({
      estado,
      cargando,
      demo,
      necesitaInstalacion,
      sinSesion,
      refrescar: () => setRonda((r) => r + 1),
    }),
    [estado, cargando, demo, necesitaInstalacion, sinSesion],
  )

  return <Ctx.Provider value={valor}>{children}</Ctx.Provider>
}

export function useEstado() {
  return useContext(Ctx)
}

/** La zona horaria del canal; UTC mientras no se sepa. */
export function useZona(): string {
  const { estado } = useEstado()
  return estado?.canal.zona_horaria ?? 'UTC'
}
