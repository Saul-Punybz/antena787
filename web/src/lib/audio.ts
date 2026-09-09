// Cómo se lee el sonido de un archivo en pantalla (F1-58 a F1-62).
//
// Todo lo que el operador ve es en cristiano: el idioma con su nombre, los
// canales como «mono», «estéreo» o «5.1», y el archivo de al lado por su
// nombre, no por su ruta entera.

import type { PistaDeAudio } from './tipos'

/**
 * Los códigos de idioma llegan como vengan del archivo: de dos letras
 * ("es"), de tres ("spa", "eng") o vacíos. Lo que no se conoce se dice
 * «sin idioma», nunca el código pelado.
 */
const IDIOMAS: Record<string, string> = {
  es: 'Español',
  spa: 'Español',
  esp: 'Español',
  en: 'Inglés',
  eng: 'Inglés',
  fr: 'Francés',
  fra: 'Francés',
  fre: 'Francés',
  pt: 'Portugués',
  por: 'Portugués',
}

export function nombreDeIdioma(codigo: string): string {
  return IDIOMAS[(codigo ?? '').trim().toLowerCase()] ?? 'sin idioma'
}

export function nombreDeCanales(canales: number): string {
  if (canales === 1) return 'mono'
  if (canales === 2) return 'estéreo'
  if (canales === 6) return '5.1'
  return `${canales} canales`
}

/**
 * «1 · Español · estéreo», y con el nombre de la pista detrás cuando el
 * archivo lo trae. El número es la posición en la lista: el índice de
 * verdad viaja en `indice` y es lo que se le manda al servidor.
 */
export function nombreDePista(pista: PistaDeAudio, posicion: number): string {
  const partes = [
    String(posicion),
    nombreDeIdioma(pista.idioma),
    nombreDeCanales(pista.canales),
  ]
  if (pista.titulo?.trim()) partes.push(pista.titulo.trim())
  return partes.join(' · ')
}

/** El nombre del archivo, sin la carpeta, venga con / o con \. */
export function nombreDeArchivo(ruta: string): string {
  const partes = (ruta ?? '').split(/[\\/]/).filter(Boolean)
  return partes[partes.length - 1] ?? ''
}
