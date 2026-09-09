import { useState } from 'react'
import { useNavigate } from 'react-router'
import { api } from '../lib/api'
import { useEstado } from '../lib/estado'
import { IconoAntena } from '../componentes/Iconos'
import { ErrorDeApi } from '../lib/tipos'

/**
 * La clave de estación: cuatro a seis dígitos, una vez por navegador
 * (PRD §13, §19). No es un usuario con contraseña ni un sistema de permisos:
 * es una sola clave para la estación, para que el sobrino que se conecta al
 * wifi no saque el canal del aire.
 */
export function Entrar() {
  const [clave, setClave] = useState('')
  const [error, setError] = useState('')
  const [enviando, setEnviando] = useState(false)
  const navegar = useNavigate()
  const { refrescar } = useEstado()

  const valida = /^\d{4,6}$/.test(clave)

  async function enviar(e: React.FormEvent) {
    e.preventDefault()
    if (!valida) {
      setError('La clave de la estación son de cuatro a seis dígitos.')
      return
    }
    setEnviando(true)
    setError('')
    try {
      await api.entrar(clave)
      refrescar()
      navegar('/al-aire', { replace: true })
    } catch (err) {
      setError(
        err instanceof ErrorDeApi ? err.message : 'No se pudo entrar. Intenta otra vez.',
      )
    } finally {
      setEnviando(false)
    }
  }

  return (
    <div className="entrar">
      <form className="entrar__caja" onSubmit={enviar}>
        <div className="fila" style={{ gap: 10 }}>
          <IconoAntena tamano={26} color="var(--aqua)" grosor={1.8} />
          <span style={{ font: '600 19px var(--sans)', letterSpacing: '-0.3px' }}>
            Antena<span className="aqua">787</span>
          </span>
        </div>
        <div>
          <h1 style={{ font: '700 20px var(--sans)', margin: 0 }}>
            Escribe la clave de la estación
          </h1>
          <p className="subtitulo">
            Se pide una vez por navegador y no vuelve a estorbar.
          </p>
        </div>
        <div className="campo">
          <label htmlFor="clave">Clave</label>
          <input
            id="clave"
            className="entrar__clave"
            type="password"
            inputMode="numeric"
            autoComplete="off"
            autoFocus
            maxLength={6}
            placeholder="••••"
            value={clave}
            onChange={(e) => {
              setClave(e.target.value.replace(/\D/g, '').slice(0, 6))
              setError('')
            }}
          />
          <span className="ayuda">Cuatro a seis dígitos.</span>
        </div>
        {error && <div className="error-en-cristiano">{error}</div>}
        <button className="boton boton--primario" disabled={!valida || enviando}>
          {enviando ? 'Entrando…' : 'Entrar'}
        </button>
        <p className="ayuda" style={{ margin: 0, color: 'var(--texto-3)' }}>
          ¿No te la sabes? La escogió quien instaló el canal, en el primer paso del
          asistente.
        </p>
      </form>
    </div>
  )
}
