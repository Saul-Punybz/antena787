import { HashRouter, Navigate, Route, Routes } from 'react-router'
import { Armazon } from './componentes/Armazon'
import { ProveedorDeEstado, useEstado } from './lib/estado'
import { AlAire } from './pantallas/AlAire'
import { Ajustes } from './pantallas/Ajustes'
import { Asistente } from './pantallas/Asistente'
import { Biblioteca } from './pantallas/Biblioteca'
import { Entrar } from './pantallas/Entrar'
import { Parrilla } from './pantallas/Parrilla'
import { ParrillaDia } from './pantallas/ParrillaDia'
import { ParrillaGuia } from './pantallas/ParrillaGuia'
import { ParrillaMes } from './pantallas/ParrillaMes'
import { ParrillaSemana } from './pantallas/ParrillaSemana'
import { Reglas } from './pantallas/Reglas'

/**
 * Rutas por hash: la build es estática y el binario Go la sirve con go:embed,
 * así que no hay servidor que reescriba rutas.
 */
export function App() {
  return (
    <ProveedorDeEstado>
      <HashRouter>
        <Rutas />
      </HashRouter>
    </ProveedorDeEstado>
  )
}

function Rutas() {
  const { cargando, necesitaInstalacion, sinSesion } = useEstado()

  if (cargando) {
    return (
      <div className="entrar">
        <p className="cargando">Buscando el canal…</p>
      </div>
    )
  }

  // Si no hay clave configurada todavía, solo el asistente está abierto.
  if (necesitaInstalacion) {
    return (
      <Routes>
        <Route path="/asistente" element={<Asistente />} />
        <Route path="*" element={<Navigate to="/asistente" replace />} />
      </Routes>
    )
  }

  if (sinSesion) {
    return (
      <Routes>
        <Route path="/entrar" element={<Entrar />} />
        <Route path="*" element={<Navigate to="/entrar" replace />} />
      </Routes>
    )
  }

  return (
    <Routes>
      <Route path="/entrar" element={<Entrar />} />
      <Route path="/asistente" element={<Asistente />} />
      <Route element={<Armazon />}>
        <Route path="/al-aire" element={<AlAire />} />
        <Route path="/parrilla" element={<Parrilla />}>
          <Route index element={<ParrillaSemana />} />
          <Route path="dia" element={<ParrillaDia />} />
          <Route path="mes" element={<ParrillaMes />} />
          <Route path="guia" element={<ParrillaGuia />} />
        </Route>
        <Route path="/reglas" element={<Reglas />} />
        <Route path="/biblioteca" element={<Biblioteca />} />
        <Route path="/ajustes" element={<Ajustes />} />
        {/* Estas las construye la siguiente entrega; el menú las esconde
            hasta que existan. */}
        <Route path="/en-vivo" element={<PorHacer nombre="En vivo" />} />
        <Route path="/anuncios" element={<PorHacer nombre="Anuncios" />} />
        <Route path="*" element={<Navigate to="/al-aire" replace />} />
      </Route>
      <Route path="*" element={<Navigate to="/al-aire" replace />} />
    </Routes>
  )
}

function PorHacer({ nombre }: { nombre: string }) {
  return (
    <>
      <h1 className="titulo-pantalla">{nombre}</h1>
      <p className="subtitulo">Esta pantalla todavía no está construida.</p>
    </>
  )
}
