import { useEffect, useState } from 'react'
import { PestanasDeParrilla } from './Parrilla'
import { api } from '../lib/api'
import { useEstado } from '../lib/estado'
import { diaSemanaDe, horasBonitas, mesLargo } from '../lib/fechas'
import type { MesDelPlan } from '../lib/tipos'

const CABECERAS = ['DOM', 'LUN', 'MAR', 'MIÉ', 'JUE', 'VIE', 'SÁB']

/** Barra de 24 horas: lo lleno en gris, lo vacío rayado en rojo. */
function BarraDelDia({ franjas }: { franjas: boolean[] }) {
  return (
    <div style={{ display: 'flex', height: 6, borderRadius: 3, overflow: 'hidden' }}>
      {franjas.map((lleno, i) => (
        <div
          key={i}
          style={{
            flex: 1,
            background: lleno ? '#39415a' : 'var(--rayado-rojo)',
          }}
        />
      ))}
    </div>
  )
}

export function ParrillaMes() {
  const { estado } = useEstado()
  const hoy = estado?.dia_emision ?? '2026-09-04'
  const [mes, setMes] = useState(hoy.slice(0, 7))
  const [datos, setDatos] = useState<MesDelPlan | null>(null)

  useEffect(() => {
    api.planMes(mes).then(setDatos).catch(() => setDatos(null))
  }, [mes])

  function correrMes(n: number) {
    const [a, m] = mes.split('-').map(Number)
    const d = new Date(Date.UTC(a, m - 1 + n, 1))
    setMes(d.toISOString().slice(0, 7))
  }

  const relleno = datos ? diaSemanaDe(datos.dias[0].dia) : 0

  return (
    <>
      <div className="encabezado">
        <div>
          <h1 className="titulo-pantalla">{mesLargo(mes)}</h1>
          <p className="subtitulo">
            Cada día muestra sus 24 horas. Lo rayado en rojo está vacío.
          </p>
        </div>
        <div className="fila" style={{ gap: 10 }}>
          <button className="boton" onClick={() => correrMes(-1)}>
            ←
          </button>
          <button className="boton" onClick={() => correrMes(1)}>
            →
          </button>
          <PestanasDeParrilla />
        </div>
      </div>

      <div className="entre">
        <div className="fila" style={{ gap: 22, fontSize: 12.5, color: 'var(--texto-2)' }}>
          <span className="fila" style={{ gap: 8 }}>
            <i style={{ width: 26, height: 9, borderRadius: 3, background: '#39415a' }} />
            programado
          </span>
          <span className="fila" style={{ gap: 8 }}>
            <i style={{ width: 26, height: 9, borderRadius: 3, background: 'var(--rayado-rojo)' }} />
            vacío
          </span>
          <span className="fila" style={{ gap: 8 }}>
            <i className="punto punto--aviso" />
            se vence una regla
          </span>
          <span className="fila" style={{ gap: 8 }}>
            <i className="punto" style={{ background: 'var(--aqua)' }} />
            entra al aire
          </span>
        </div>
        {datos && (
          <span className="rojo" style={{ font: '600 13px var(--sans)' }}>
            {horasBonitas(datos.horas_vacias_mes)} vacías este mes ·{' '}
            {datos.porcentaje_vacio}% del aire
          </span>
        )}
      </div>

      {!datos && <p className="cargando">Contando el mes…</p>}

      {datos && (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(7, minmax(0,1fr))', gap: 10 }}>
          {CABECERAS.map((c) => (
            <div key={c} className="rotulo" style={{ textAlign: 'center', paddingBottom: 4 }}>
              {c}
            </div>
          ))}
          {Array.from({ length: relleno }, (_, i) => (
            <div key={'hueco' + i} />
          ))}
          {datos.dias.map((d) => {
            const esHoy = d.dia === hoy
            const muyVacio = d.horas_sin_llenar >= 8
            return (
              <div
                key={d.dia}
                className="tarjeta"
                style={{
                  padding: '11px 12px 12px',
                  minHeight: 118,
                  borderColor: esHoy
                    ? 'var(--aqua)'
                    : muyVacio
                      ? 'rgba(248,81,73,.4)'
                      : 'var(--borde)',
                  display: 'flex',
                  flexDirection: 'column',
                  gap: 8,
                }}
              >
                <div className="entre">
                  <span style={{ font: '400 15px var(--sans)', color: 'var(--texto)' }}>
                    {Number(d.dia.slice(8))}
                  </span>
                  {esHoy && (
                    <span className="aqua" style={{ font: '600 11px var(--sans)', letterSpacing: 1 }}>
                      HOY
                    </span>
                  )}
                </div>
                <BarraDelDia franjas={d.franjas_llenas} />
                <span
                  style={{
                    font: '400 11.5px var(--sans)',
                    color: muyVacio ? 'var(--rojo)' : 'var(--texto-2)',
                  }}
                >
                  {horasBonitas(d.horas_sin_llenar)} vacías
                </span>
                {d.vencimientos.map((v) => (
                  <span key={v} className="fila" style={{ gap: 6, fontSize: 11.5 }}>
                    <i className="punto punto--aviso" style={{ width: 6, height: 6 }} />
                    <span className="ambar">vence {v}</span>
                  </span>
                ))}
                {d.estrenos.map((v) => (
                  <span key={v} className="fila" style={{ gap: 6, fontSize: 11.5 }}>
                    <i className="punto" style={{ width: 6, height: 6, background: 'var(--aqua)' }} />
                    <span className="aqua">empieza {v}</span>
                  </span>
                ))}
              </div>
            )
          })}
        </div>
      )}
    </>
  )
}
