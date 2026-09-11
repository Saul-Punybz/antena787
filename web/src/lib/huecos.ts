// Un hueco de la parrilla y lo que se hace con él.
//
// La parrilla no acepta poner contenido directo: es consecuencia de las
// reglas, no una hoja de celdas (PRD §9, decisión del 11 de septiembre de
// 2026). Lo que gana es el atajo: un hueco abre la regla que lo llenaría, con
// el día y la hora ya puestos. Semana y Día usan lo mismo.

import { useState } from 'react'
import { reglaDesdeHueco, type InicialDeRegla } from '../componentes/EditorDeRegla'
import { diaLargo, minutosAHora12 } from './fechas'
import type { TituloDeBiblioteca } from './tipos'

export interface Hueco {
  /** Día de emisión, "AAAA-MM-DD". */
  dia: string
  /** Minutos desde medianoche en que empieza el vacío. */
  desde: number
  /** Minutos desde medianoche en que vuelve a haber algo. */
  hasta: number
}

/** "de 1:00 PM a 6:00 PM del sábado 12" */
export function textoDelHueco(h: Hueco): string {
  return (
    `de ${minutosAHora12(h.desde)} a ${minutosAHora12(h.hasta)} ` +
    `del ${diaLargo(h.dia)} ${Number(h.dia.slice(8))}`
  )
}

/** Lo que lee quien va con teclado o con lector de pantalla. */
export function etiquetaDelHueco(h: Hueco): string {
  return `Vacío ${textoDelHueco(h)}: poner algo aquí`
}

/**
 * Redondea a la media hora de donde se hizo clic, sin salirse del hueco. Con
 * el teclado no hay posición, así que cae al principio del vacío.
 */
export function mediaHoraDelClic(
  clicX: number,
  caja: { left: number; width: number },
  h: { desde: number; hasta: number },
): number {
  const x = caja.width > 0 ? (clicX - caja.left) / caja.width : 0
  const crudo = h.desde + x * (h.hasta - h.desde)
  const redondeado = Math.round(crudo / 30) * 30
  return Math.min(Math.max(redondeado, h.desde), Math.max(h.desde, h.hasta - 30))
}

/**
 * La llave del formulario: cambia cuando cambia con qué tiene que abrir, y
 * así React lo vuelve a llenar en vez de quedarse con lo primero que vio.
 */
export function claveDeRegla(r: InicialDeRegla): string {
  return [r.titulo ?? '', r.patron_de_dias ?? '', r.hora ?? '', r.fecha_inicio ?? ''].join('|')
}

/**
 * El estado del atajo: qué hueco se escogió y con qué valores abre la regla.
 * Mientras haya un hueco escogido, la biblioteca al lado resalta lo que no
 * está programado, que es lo que sirve para llenarlo.
 */
export function usarHueco() {
  const [hueco, setHueco] = useState<Hueco | null>(null)
  const [reglaNueva, setReglaNueva] = useState<InicialDeRegla | null>(null)

  /** Clic en un vacío: la regla abre a esa hora de ese día. */
  function abrirHueco(h: Hueco) {
    setHueco(h)
    setReglaNueva(
      reglaDesdeHueco({
        ...h,
        nota: `Sale del vacío ${textoDelHueco(h)}. Cambia lo que haga falta antes de guardar.`,
      }),
    )
  }

  /** Un título de la biblioteca: con un hueco escogido, entra a su hora. */
  function escogerTitulo(t: TituloDeBiblioteca) {
    if (hueco) {
      setReglaNueva(
        reglaDesdeHueco({
          ...hueco,
          titulo: t.nombre,
          nota: `${t.nombre} va en el vacío ${textoDelHueco(hueco)}. Ponle hasta cuándo dura y queda hecho.`,
        }),
      )
      return
    }
    setReglaNueva({
      titulo: t.nombre,
      nota: `${t.nombre} no está en la parrilla. Dile qué días y a qué hora sale.`,
    })
  }

  /** Una regla nueva sin hueco de por medio, con lo que se sepa ya puesto. */
  function abrirRegla(inicial: InicialDeRegla) {
    setHueco(null)
    setReglaNueva(inicial)
  }

  function cerrar() {
    setReglaNueva(null)
    setHueco(null)
  }

  return { hueco, reglaNueva, abrirHueco, abrirRegla, escogerTitulo, cerrar }
}
