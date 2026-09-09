# Radio FM que también sale online y por un canal de TV

## Quién es

Una emisora de FM regional. Programación hablada: noticias en la mañana, un
programa de entrevistas al mediodía, deportes por la tarde, y bloques
grabados de noche y de madrugada.

Ya sale por **la antena de FM**. Sale también por **internet**, con un
software distinto que alguien montó hace años. Y un par de horas al día su
programa estelar **se simulcastea por un canal de televisión** — hoy eso es un
humano apuntando una cámara a un cartel.

**Son tres cosas que se mantienen por separado, y no deberían serlo.**

## El punto entero de este ejemplo

> **Una emisora de radio es un canal de televisión sin video.** Reglas, plan,
> as-run, cortes publicitarios, fuentes en vivo, grabación de la salida,
> diferido y control manual son **idénticos**. Lo que cambia es el formato de
> casa —solo audio— y la salida.

Y de ahí sale lo importante:

> **Un canal de radio puede salir por tres lados a la vez —la antena de FM,
> internet, y un canal de televisión— sin ser tres canales.**

El formato de casa es audio. La salida de TV y la de internet reciben ese mismo
audio con el **cartel visual** que dibuja el sistema: carátula, el título de lo
que suena, el logo, el crawl. **Nada se configura dos veces**: las reglas son
unas, los cortes son unos, el as-run es uno.

## Qué tiene

- Una PC en la estación.
- La cadena de FM que ya está: procesador de audio, excitador, amplificador,
  antena. Nada de eso cambia.
- Un estudio del que salen los programas en vivo.
- Un servidor de streaming para la web, que es lo que se va a jubilar.
- Anunciantes locales que compran cuñas, y unos cuantos que compran una línea
  de texto.

## Qué configura

| | |
|---|---|
| **Perfil** | El de su país. En Estados Unidos, `us-fcc`, con la clase de licencia que le toque. |
| **Formato de casa** | **Solo audio.** Un objetivo de volumen, una configuración de audio, y ya. No hay resolución ni cuadros que escoger. |
| **Salidas — tres, un solo canal** | **1)** La de la **antena de FM**, con su objetivo de volumen. **2)** La de **internet** (SRT, RTMP o HLS), con el suyo, que es distinto. **3)** La del **canal de televisión**, que lleva el mismo audio con el **cartel visual** encima. Cada salida tiene su propio volumen, su estado de conexión y su reconexión con espera progresiva. |
| **Cartel visual** | Lo dibuja el sistema, no un humano con una cámara: carátula, título que suena, logo del canal, y el crawl de clasificados si lo hay. |
| **Fuentes en vivo** | Los programas del estudio, por SRT con contraseña. Un bloque en vivo de radio es **solo audio**, y el video de la salida de TV es el cartel del programa. |
| **Reloj de cortes** | **Esto es lo propio de la radio.** A qué minutos de cada hora se va a comerciales —por ejemplo `[0, 15, 30, 45]`— es **propio de cada fuente en vivo y de cada emisora como canal**. El sistema lo pregunta una vez y alinea los cortes con él, en vez de meter una cuña encima de alguien hablando. |
| **Carga publicitaria** | **Doce minutos por hora** de espacios vendibles por defecto, configurable. |
| **Fichas y carátulas** | **Cover Art Archive**, que es el que sirve para música y **no pide clave**. Se guarda junto al archivo una sola vez; **la cadena de aire funciona con el internet caído**. |
| **Grabación y diferido** | La salida se graba, y el programa de la mañana se vuelve a poner de madrugada con una regla. Los cortes de la ventana repetida son espacios nuevos, nunca copias de los originales. |

**Y una cosa que no es obvia:** *deck* aquí no significa lo que significa en
automatización de radio. En Antena787 un **deck** es una de las colas paralelas
del canal —manual, comercial, programa y relleno, en ese orden de prioridad—,
no dos reproductores que se funden entre sí.

## Qué NO necesita

- **Tres sistemas.** Ese es el punto: uno.
- **Configurar nada dos veces.** Ni las reglas, ni los cortes, ni los
  anunciantes, ni el as-run.
- **Un humano apuntando una cámara al cartel** para el simulcast de TV.
- **Tocar la cadena de FM.** El procesador, el excitador y el amplificador
  siguen igual. Antena787 reemplaza el playout, no la RF.
- **Rotación musical**, *si su programación es hablada*. Y aquí está el límite
  honesto:

> **Antena787 sirve hoy para radio hablada, deportiva y de programas** —se
> programa por bloques, igual que televisión—. **Para radio musical no
> completamente**, y no se disimula: la radio musical no programa por título
> sino por **categorías con relojes por hora** y **reglas de separación** —que
> no se repita el mismo artista en tres horas, que no caigan dos baladas
> seguidas—. Eso es un **subsistema aparte, no un ajuste**, y va en su propia
> fase.

## Qué fase lo cubre

| | |
|---|---|
| **F1 y F2** | Toda la maquinaria compartida: reglas, resolver, guía, motor, decks, salidas, fuentes en vivo, manual, grabación, diferido. **Nada de esto se escribe otra vez para radio.** |
| **F4** | Publicidad: cuñas vendidas con su prioridad y su rotación, cortes dentro y fuera de bloques en vivo, crawl de clasificados, portal del anunciante, evidencia de emisión. |
| **F4b · Radio** | **Formato de casa solo audio y salida de audio. Eso es todo lo que falta** — el resto ya funciona tal cual. Sirve para radio hablada, deportiva y de programas. |
| **F4c · Rotación musical** | Categorías, relojes por hora y reglas de separación. **Es lo que falta para radio musical**, y es un subsistema propio. |

**F4b no es "para después".** Donde hay una televisora y una emisora de FM en
el mismo sitio —que en América Latina es el caso normal, no la excepción— la
radio entra junto con el resto.
