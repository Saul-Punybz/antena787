# ADR 0012 — Un canal por proceso: los subcanales son del multiplexor

**Fecha:** 11 de septiembre de 2026
**Estado:** aceptada
**Nace de:** una pregunta de Saul — «esto corre un solo canal; podría correr
varios subcanales, pero no vi ningún software que lo hiciera, ¿tú viste alguno?»

## La decisión

**Antena787 produce un canal por proceso, y eso no es una limitación
pendiente de resolver: es donde va la frontera.** Los subcanales los arma el
multiplexor.

## Por qué

Un subcanal de ATSC es **un programa más dentro del mismo Transport Stream**.
El playout produce un programa; el multiplexor toma varios programas y los mete
en los 6 MHz del canal licenciado, y de paso les pone los números de canal
virtual.

La evidencia no es teórica: es del propio despliegue. Rolando, describiendo su
Technalogix TP1000 (`docs/equipos/CADENA-CATV-2026-09-11.md`):

> «El los reasigna, le puedes editar, pero prácticamente ese es su trabajo,
> **y añadirle los números de los canales virtuales**.»

Está describiendo exactamente el trabajo que un playout no hace.

Por eso una estación con tres subcanales corre **tres playout entrando al mismo
multiplexor**, no uno que produzca tres. Los sistemas comerciales que corren
«varios canales» —Cinegy Air, PlayBox— corren varios canales **independientes,
cada uno con su salida**, que es otra cosa.

## Lo que esto permite, y sale barato

Tres subcanales son **tres instalaciones del mismo binario**. Antena787 es un
ejecutable único, sin Docker y sin servicios que orquestar, así que el costo de
eso es una carpeta de datos más y un puerto más. La frontera está en el sitio
correcto y además es la barata.

## Lo que NO dice este ADR

- **No dice que multicanal en un proceso sea mala idea para siempre.** Dice que
  no hace falta para emitir subcanales, que es lo que se preguntó. Si algún día
  aparece otra razón —una sola pantalla para operar cuatro canales, por
  ejemplo— eso es un producto distinto y merece su propio ADR.
- **No se verificó exhaustivamente** que ningún playout del mundo produzca
  varios programas en un TS. Las cuatro investigaciones del 11 de septiembre
  preguntaban por bitrate, no por multicanal, así que su silencio sobre esto es
  **ausencia de evidencia, no evidencia de ausencia**. La decisión se sostiene
  en el argumento de dónde va la frontera y en lo que dice el operador del
  equipo, no en un censo de productos.
