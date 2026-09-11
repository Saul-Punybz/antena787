# ADR 0011 — No aceleramos el decode por hardware: aspiramos a no decodificar

**Fecha:** 11 de septiembre de 2026
**Estado:** aceptada
**Nace de:** una pregunta de Saul — «¿dónde dejaste el decoding, si tienes
encoding?» — al día siguiente de construir `engine.Acelerador`.

## El contexto

El 11 de septiembre se construyó el acelerador de video (`engine.Acelerador`,
esquema v7), que escoge con qué se **comprime**: el procesador o la tarjeta.
Nació porque F2-11 lo exige: el watchdog tiene que relanzar el encoder colgado
«con el mismo acelerador» y caer a software si la tarjeta falla dos veces.

Al preguntarse por qué existía para el encoding y no para el decoding,
aparecieron tres cosas a la vez.

**Una: nadie lo había decidido.** `git log -S hwaccel --all` sobre el
repositorio entero: `-hwaccel` **nunca apareció ni se quitó**. Todo lo que este
proyecto llama aceleración —ADR 0001, `PRD.md` §14, F2-15— está definido como
algo del **encoder**. Menos una línea: **`PRD.md:1626` ya decía que «la
aceleración se gasta en decodificar la biblioteca (H.264)»**. Esa frase nunca
bajó a un criterio numerado, y prosa que no se vuelve criterio no se construye.

**Dos: la prosa tenía razón sobre dónde está el costo, pero el costo es chico.**
Medido el mismo día sobre un HEVC 1080p real, 30 segundos con los filtros de
casa (`docs/investigacion/MEDICION-CPU-2026-09-11.md`):

| Etapa | Núcleos |
|---|---|
| Decode | **0.40** |
| Encode MPEG-2 4 Mb/s, al transmisor | **0.15** |
| Encode H.264, a internet | 0.33 |

**El camino completo al transmisor son 0.55 núcleos de diez.** El decode cuesta
casi el triple que el encode al transmisor: **se aceleró el tercio más barato.**
Y el reporte de F0 (`docs/f0/REPORTE-mac-m4-2026-09-08.md`, F0-07) ya había
medido **110 % de un núcleo** con todo por software y dos salidas, antes de que
el acelerador existiera.

**Tres: nadie en la industria resuelve esto acelerando el decode**
(`docs/investigacion/ACELERACION-COMPARADA-2026-09-11.md`):

- La **documentación oficial de ffmpeg** dice que `-hwaccel` en decode no será
  más rápido que software en procesadores modernos, y que casi siempre hay que
  bajar los cuadros a memoria normal y se pierde lo ganado.
- **VLC** —el programa con el que CAtv emite hoy— trae el decode por hardware
  **apagado de fábrica** en todas las plataformas, Windows incluido.
- El hwaccel de decode solo paga si **todo** el camino se queda en la tarjeta.
  Con un encoder de salida por software, como el nuestro, se paga el viaje de
  vuelta a memoria sin ganar nada.
- Los sistemas de broadcast de verdad (Ross, Techex, BroadStream) hacen
  *compressed-domain switching*: **transcodifican una vez al ingerir y copian
  los bits comprimidos al aire**, porque decodificar y volver a encodear añade
  costo, añade retraso y degrada la calidad.

## La decisión

**No se construye aceleración por hardware para el decode.** Ni ahora ni como
deuda pendiente: se cierra, con esta razón escrita.

**Y la dirección correcta es la contraria: no decodificar.** La normalización al
ingerir (`internal/ingest/normalize.go`) ya deja todo en un formato único con
GOP de un segundo, que es justo la condición que hace posible copiar los bits
en vez de decodificarlos. Ese dividendo hoy se paga y no se cobra: el motor
decodifica y vuelve a encodear siempre, aunque el archivo ya esté perfecto.

Esa es la palanca, y tiene su propio trabajo por delante: guardar las versiones
que hagan falta —tratándolas como **cache**, no como datos, porque se
regeneran— y pasar los bits sin tocar cuando el códec coincida.

## Lo que esto NO dice

- **El acelerador del encoder no sobra.** Sigue siendo lo que F2-11 necesita
  para relanzar y para caer a software, y lo que salva a una estación con una
  máquina mucho más floja. Lo que cambia es su prioridad: deja de ser urgente.
- **La medición no es la máquina de la torre.** Un M4 de diez núcleos no es el
  PC con Windows 10 de CAtv, del que no sabemos ni el procesador. La proporción
  entre las etapas se mantiene; los números absolutos no. **Está pendiente
  preguntarle a Rolando qué procesador tiene**, y esa respuesta es la única que
  puede reabrir este ADR.
- **El passthrough no es gratis.** Pelea con ADR 0001 —un encoder persistente
  que nunca se reinicia entre clips, que es lo que hace que el canal no se ponga
  en negro—. Copiar bits para un bloque significa saltarse ese encoder y volver
  a entrar sin que el multiplexor pierda el stream. Merece su propio ADR y una
  prueba a escala antes de prometerse.

## La consecuencia que vale para todo el proyecto

**Prosa que no se vuelve criterio no se construye.** `PRD.md:1626` tenía razón
desde el principio y se perdió meses porque nadie la bajó a un número. Cuando
el PRD diga algo que el sistema debe hacer, o se convierte en criterio de
`docs/ACEPTACION.md`, o se borra del PRD.
