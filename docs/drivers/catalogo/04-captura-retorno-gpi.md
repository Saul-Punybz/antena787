# Catálogo: retorno de aire, captura, GPI/GPO y audio de radio

> Este catálogo es investigación, no código. Sigue el patrón de
> [`docs/hardware/MATRIZ.md`](../../hardware/MATRIZ.md): **verificado** quiere
> decir que la fuente lo confirma por escrito, no que Antena787 ya lo probó.
> Nada de esto reemplaza la matriz — la alimenta.

---

## 1 · Retorno de aire (`capture_input`, ADR 0009)

La pregunta de fondo, dicho en el PRD §9 paso 8: la verdad de lo que salió es
la **señal transmitida**, no lo que nuestro proceso creyó mandar. Estos son
los caminos para conseguir esa señal de vuelta.

| Marca/modelo | Función | Conexión y protocolo | ffmpeg solo vs. driver propio | Manual/API | Driver PRD | Biblioteca Go sin CGo | Estado |
|---|---|---|---|---|---|---|---|
| **SiliconDust HDHomeRun** (Flex, Flex Duo, Flex Quatro, Connect) | Sintonizador ATSC de red, expone el TS por HTTP | Ethernet, IP propia en la red local. `GET http://<dispositivo>/auto/v<canal>` devuelve MPEG-TS crudo por HTTP; sin login | **ffmpeg solo basta.** Es una URL HTTP: `ffmpeg -i http://<ip>/auto/v6.1 ...` o, mejor, un `net/http.Get` en Go puro contra esa misma URL — no hace falta ni ffmpeg de por medio para leerlo | Guía HTTP oficial: `http://info.hdhomerun.com/info/http_api`; PDF: `hdhomerun_http_development.pdf` (silicondust.com) | `capture_input` tipo `stream` | Ninguna: `net/http` de la librería estándar | **Verificado** (10 sep 2026) — endpoint, formato TS y ausencia de autenticación confirmados en la doc oficial |
| **Hauppauge** USB/PCIe ATSC (WinTV-HVR, quadHD) y familias equivalentes | Tarjeta receptora de TV en la propia máquina | Windows: BDA vía DirectShow. Linux: kernel DVB, nodos `/dev/dvb/adapterN/{frontend,dvr}` | **ffmpeg no sintoniza solo.** En Linux hay que sintonizar primero con una herramienta DVB (`dvbv5-zap`/`v4l-utils`) y luego leer el TS ya sintonizado de `/dev/dvb/adapterN/dvr0` — con `ffmpeg -i` o directamente por lectura de archivo en Go. En Windows, `ffmpeg -f dshow` puede listar y abrir el dispositivo BDA como una fuente dshow más, pero sintonizar el canal (frecuencia, ATSC) normalmente requiere el filtro BDA de la tarjeta, no solo dshow genérico | `hauppauge.com/pages/support/support_linux.html`; kernel DVB API en `linuxtv.org` | `capture_input` tipo `receptor-tv` | Ninguna necesaria si se delega la sintonía a `dvbv5-zap`/`v4l-utils` (binarios externos, no CGo) y se lee el nodo de dispositivo como archivo | **Verificado** el camino Linux (DVB + ffmpeg pipe); **a verificar** el camino Windows exacto por dshow puro sin el software del fabricante |
| **Otras USB ATSC genéricas** (chips Realtek/Silicon Labs vendidas como "USB TV tuner") | Igual que Hauppauge | Igual que Hauppauge: DVB en Linux, BDA en Windows | Igual que arriba | Depende del chipset; kernel DVB las trae si están soportadas | `capture_input` tipo `receptor-tv` | Igual que Hauppauge | **No verificado por modelo** — el patrón DVB/BDA es el mismo, pero cada chipset necesita su propio driver de kernel/BDA |
| **Sencore AG 2700** (y línea VideoBRIDGE) | Receptor RF profesional con salida ASI e IP | Red (IP) y ASI. Documentación de producto pública; API/protocolo detallado no publicado sin contacto comercial | Si la salida IP es un TS por UDP/RTP simple, se lee sin SDK; si el control y la telemetría van por un protocolo propio, hace falta su documentación | Páginas de producto: `sencore.com/product-category/monitoring-and-analysis`; `tvtechnology.com` (anuncio AG 2700) | `capture_input` tipo `stream` (si la salida IP es TS plano) | `net/http` o socket UDP puro si el TS viaja así | **No público** — falta la especificación exacta de la salida IP/ASI |
| **DekTec DTU-238** y receptores DTA/DTU en general | Receptor RF profesional (DVB-T2, ATSC/ISDB según modelo) con SDK propio | USB/PCIe, controlado por **DTAPI**, biblioteca en C con enlace nativo | **Fuera de alcance.** DTAPI es C: enlazarlo obliga a CGo, lo que viola ADR 0002. No hay ruta ffmpeg-solo documentada para estos modelos | `dektec.com/products/sdk/`, `dektec.com/downloads/SDK/` | Ninguno — **no cubierto** | No aplica (requiere CGo) | **Verificado que queda fuera**: el SDK es C puro, sin alternativa HTTP/red documentada |
| **Monitoreo por streaming** (HLS/RTMP público del propio canal, o del transmisor) | Ver de vuelta lo que salió sin hardware adicional | Red: una URL HLS o RTMP que ya existe porque el canal también sale por internet | **ffmpeg solo basta** — es el mismo tipo de lectura que cualquier entrada de red; en Go, un cliente HLS/RTMP mínimo o delegar a ffmpeg como subproceso | N/A — es protocolo estándar (HLS/RTMP), no de un fabricante | `capture_input` tipo `stream` | Ninguna imprescindible; hay paquetes Go para HLS si se quiere evitar el subproceso de ffmpeg | **Verificado** como concepto — es la misma idea que ya usa CAtv (Rolando, PRD §25) |

> **Por qué `stream` es el tipo más barato de construir.** No importa si la
> URL sale de un HDHomeRun, del HLS público o de un endpoint IP de un
> receptor profesional: en los tres casos el retorno de aire es una URL que
> se lee por red, sin enlazar ninguna librería en C. `receptor-tv` (DVB/BDA)
> y `captura` (una tarjeta de captura alimentada por el RF MONITOR del
> excitador) son los caminos que sí tocan un dispositivo local y por eso
> cargan más peso de driver.

---

## 2 · Captura de video/audio para fuentes en vivo

| Marca/modelo | Función | Conexión y protocolo | ffmpeg solo vs. driver propio | Manual/API | Driver PRD | Biblioteca Go sin CGo | Estado |
|---|---|---|---|---|---|---|---|
| **Dongles HDMI→USB genéricos** (chips MacroSilicon MS2109/MS2130 y similares, sin marca) | Entrada de video/audio HDMI barata | USB, clase **UVC** (video) + **UAC** (audio) — sin driver propio | **ffmpeg solo basta.** Aparecen como cualquier webcam: `-f dshow` en Windows, `-f v4l2` (video) + `-f alsa` (audio) en Linux | Ninguno oficial — es la clase USB estándar, documentada por el USB-IF, no por el vendedor | `captura` | No aplica (no hay protocolo propio que envolver) | **Verificado como patrón**; el chipset exacto varía por lote y no siempre es UVC estricto |
| **Elgato Cam Link 4K** | Captura HDMI a USB, gama media | USB 3.0, plug-and-play; la página oficial no confirma UVC explícitamente pero describe instalación automática de drivers y compatibilidad directa con OBS/Zoom sin configuración | Compatible con el mismo patrón `-f dshow` / `-f v4l2` que un dispositivo de clase estándar, a juzgar por el comportamiento "funciona en cualquier app de captura" que reporta el fabricante | `elgato.com/us/en/p/cam-link-4k` | `captura` | No aplica | **No confirmado por escrito que sea UVC puro** — el fabricante no lo dice explícitamente; el comportamiento reportado es consistente con UVC |
| **Magewell USB Capture HDMI (Gen 2 / Plus / 4K Plus)** | Captura HDMI profesional gama alta | USB 3.0, **UVC/UAC certificado por el propio fabricante** — "fully UVC compatible, driver-free" | **ffmpeg solo basta**, confirmado con ejemplos reales del fabricante y de terceros: `-f v4l2 -i /dev/videoN` en Linux, `-f dshow` en Windows | `magewell.com/tech-specs/usb-capture-hdmi-plus`; `popey.com/blog/2021/01/magewell-hdmi-capture-with-ffmpeg` | `captura` | No aplica | **Verificado** — es la opción con mejor documentación pública de las tres |
| **Blackmagic DeckLink** (PCIe y Duo/Mini) | Captura profesional SDI/HDMI | PCIe interno, **Desktop Video driver + Decklink SDK propietario en C** | **No es "ffmpeg solo".** El soporte `--enable-decklink` de ffmpeg existe, pero **exige compilar ffmpeg de nuevo** contra el SDK de Blackmagic (`--extra-cflags`/`--extra-ldflags` apuntando al SDK) — es un binario de ffmpeg distinto al que Antena787 empaqueta por defecto, igual que pasa con NDI (ADR 0002) | `forum.blackmagicdesign.com` (hilo de compilación); gists públicos de compilación en Ubuntu/Windows | Ninguno — **fuera del alcance de la build oficial**, mismo argumento que NDI (PRD §5) | No aplica (SDK en C) | **Verificado**: el soporte existe en ffmpeg upstream pero solo en una build especial con el SDK propietario enlazado, no en el binario genérico |
| **Interfaces de audio USB** (Focusrite Scarlett, Behringer UCA, genéricas) | Entrada de audio para radio en vivo | USB, clase **UAC** (Audio Class), sin driver propio en la mayoría de modelos de consumo | **ffmpeg solo basta**: `-f alsa` en Linux, `-f dshow` (dispositivo de audio) en Windows | Documentación de clase USB Audio, no del fabricante | `captura` (fuente en vivo, solo audio) | No aplica | **Verificado como patrón** — el driver de clase es del sistema operativo, no del proyecto |

---

## 3 · GPI/GPO por contacto seco

Es la ruta que el PRD llama **señal de corte desde la fuente** (§9 paso 5) y
**alertas de emergencia por relés** (§10): contacto seco desde una consola o
un ENDEC hacia una entrada digital.

| Marca/modelo | Función | Conexión y protocolo | Qué hace falta | Manual/API | Driver PRD | Biblioteca Go sin CGo | Estado |
|---|---|---|---|---|---|---|---|
| **Numato Lab** (relés 1/4/8/16/64 canales) | Relés y entradas digitales USB | Aparece como **puerto COM virtual**; protocolo **ASCII legible** (`relay on 0`, `relay read 0`, eco de vuelta con `>`) | Abrir el puerto serie y escribir comandos de texto con `\r` al final | `numato.com/docs/16-channel-usb-relay-module/`; `numato.com/kb/sending-commands-through-serial-terminal-emulator` | `gpi-serial` | **`go.bug.st/serial`** — puro Go; única salvedad: la enumeración de puertos en macOS necesita CGo para IOKit, no la lectura/escritura en sí | **Verificado** — protocolo documentado por el fabricante con ejemplos exactos |
| **Denkovi** (USB 8/16 relés, "Virtual COM Port") | Relés y entradas | Puerto COM virtual; comando binario de 2-3 bytes (estado de cada relé en un byte, no siempre ASCII imprimible); también hay variantes ModBus RTU y HID/bitbang | Igual que Numato pero el payload es binario, no texto — se construye el frame de bytes a mano | `denkovi.com/Documents/USB-Relay-16Channels-v3/UserManual.pdf` | `gpi-serial` (variantes serie/ModBus); `gpi-gpio` si es la variante HID | `go.bug.st/serial`, y si es ModBus, una biblioteca ModBus RTU pura en Go (p. ej. `goburrow/modbus`, sin CGo) | **Verificado** el protocolo serie; **a verificar** cuál variante concreta convendría para CAtv |
| **KMtronic** (relés RS232/USB, 1 a 8 canales) | Relés | Serie, 9600 8N1; comando fijo de 3 bytes: `FF <canal> <00|01>` | Escribir 3 bytes por el puerto serie | `info.kmtronic.com/rs232-serial-com-controlled-eight-channel-relay-board.html` | `gpi-serial` | `go.bug.st/serial` | **Verificado** — protocolo público, de los más simples del catálogo |
| **SainSmart** (relés RS232 4ch y relés HID 8/16ch) | Relés | Dos familias distintas: la RS232 usa el mismo esquema de 3 bytes que KMtronic; la **HID** (VID `0x0416`, PID `0x5020`) manda un *report* HID con una firma propia (`HID_CMD_SIGNATURE 0x43444948`) | La serie es igual que KMtronic. La HID necesita hablar HID de verdad, no un puerto serie | `sainsmart.com` (fichas de producto); `github.com/lowerpower/SainSmartUsbRelay` (implementación de referencia en C) | RS232 → `gpi-serial`; HID → `gpi-gpio` | RS232: `go.bug.st/serial`. HID: **no hay opción pura-Go madura y multiplataforma** — `karalabe/hid` y `sstallion/go-hid` envuelven HIDAPI con CGo; `rafaelmartins/usbhid` se anuncia puro Go y multiplataforma pero es un proyecto chico, sin el uso ni la revisión de los anteriores | Serie: **verificado**. HID: **no encontrado** un camino sin CGo probado en producción |
| **Sealevel SeaI/O** (relés y E/S digital USB, líneas 410U/420U/450U/462U) | Relés y E/S industrial | USB, hablando **ModBus RTU** estándar, o la biblioteca propietaria **SeaMAX** (Windows/Linux) | Si se usa ModBus RTU puro sobre el puerto serie virtual, no hace falta el SDK del fabricante | `sealevel.com` (fichas por modelo); manual genérico SeaI/O (`merging.com`, alojado por un tercero) | `gpi-serial` | `go.bug.st/serial` + una biblioteca ModBus RTU pura en Go | **Verificado** que ModBus RTU es una opción documentada, alternativa a SeaMAX |
| **Ontrak ADU200 / ADU208 / ADU218** | Relés y entradas/contadores | **USB HID** con comandos ASCII dentro del *report* (`SK0`, `RK0`, `RPK0`…); en Windows hay un mini-driver DLL del fabricante, en Linux/macOS el propio fabricante documenta el uso de **HidApi** | Hace falta hablar HID (no es un puerto serie normal, aunque los comandos sean texto) | `ontrak.net/pdfs/adu208218v2.pdf`; `ontrak.net/pythonhidapi.htm` | `gpi-gpio` | Igual que SainSmart HID: sin opción pura-Go consolidada; CGo con `karalabe/hid` es la ruta probada, aunque contradice ADR 0002 | **Verificado** el protocolo; **el acceso sin CGo queda pendiente de resolver** |

> **El patrón que se repite: serie ASCII gana, HID complica.** Todo lo que se
> presenta como **puerto COM virtual** (Numato, Denkovi, KMtronic, la
> variante RS232 de SainSmart, Sealevel por ModBus) se resuelve con
> `go.bug.st/serial` sin tocar CGo. Todo lo que se presenta como **HID puro**
> (la variante de SainSmart, Ontrak ADU) no tiene todavía una biblioteca Go
> sin CGo con el mismo historial que `karalabe/hid` — **este es el hueco real
> del catálogo**, y la salida más limpia hoy es preferir, a igualdad de
> precio, el modelo de la misma marca que hable serie en vez de HID.

---

## 4 · Audio para radio

| Tecnología | Qué es | ffmpeg solo vs. SDK propio | Driver PRD | Biblioteca Go sin CGo | Estado |
|---|---|---|---|---|---|
| **Interfaces de audio USB** (Focusrite, Behringer, genéricas) | Entrada/salida de audio de consumo o semi-profesional | **ffmpeg solo basta** — clase UAC estándar, `-f alsa`/`-f dshow` (ver tabla §2) | `captura` | No aplica | **Verificado** |
| **AES67** | Estándar abierto de audio por IP: RTP para el transporte, PTPv2 para sincronía, SDP para describir el flujo | Es RTP simple, sin SDK de fabricante — pero **no hay una biblioteca AES67 completa (RTP+PTP+SDP) lista y probada en Go** en este catálogo; construirlo es viable en teoría (no exige CGo) pero es trabajo propio, no una integración de un fin de semana | No cubierto todavía por ningún driver del PRD; encajaría como una variante de `captura` de audio en red | Ninguna encontrada como paquete único; se ensamblaría con paquetes RTP/PTP sueltos de Go | **A verificar** — el estándar es abierto y documentado, pero la pieza de software no existe hecha |
| **Dante** (Audinate) | Audio por IP propietario, líder del mercado AV | El **SDK de Audinate es C/C++**, atado al chip Brooklyn/Ultimo — enlazarlo exige CGo | Ninguno — **fuera de alcance** por el mismo argumento que NDI y DekTec (ADR 0002) | No aplica | **Verificado que el SDK es C**; la salida es usar el **modo AES67** que muchos dispositivos Dante ya ofrecen (ver fila de arriba), no el SDK propio |
| **Livewire / Livewire+** (Telos Axia) | Audio por IP de la industria de radio, con reloj de cortes propio de la red | Hardware Axia moderno soporta **modo AES67** además del Livewire nativo; el nativo tiene su propio protocolo de descubrimiento no documentado públicamente al mismo nivel que AES67 | Igual que Dante: la ruta viable sin SDK es el **modo AES67**, no el protocolo Livewire nativo | No aplica directamente | **No público** el protocolo Livewire nativo completo; **verificado** que el modo AES67 es la puerta de entrada sin SDK |

---

## Recomendación para CAtv

**El retorno de aire de CAtv hoy es "tarjeta receptora de TV en la máquina +
monitoreo por streaming" (PRD §25), y el modelo exacto de esa tarjeta todavía
no está confirmado** (`docs/hardware/MATRIZ.md`). Si al confirmarlo resulta
ser una tarjeta DVB/BDA genérica que exige sintonizar aparte (§1, fila
Hauppauge) — es decir, más piezas moviéndose en la misma máquina que hace el
playout — la alternativa más barata y más alineada con los principios del
proyecto (cero CGo, cero SDK, "probar no asumir") es:

**Sustituir o complementar esa tarjeta con un SiliconDust HDHomeRun Flex Duo**
(dos sintonizadores ATSC 1.0, ronda los US$110, verificado en la tienda
oficial el 10 sep 2026). Razones concretas para CAtv:

1. **Vive en la red, no en la máquina de playout.** Se coloca donde llegue
   bien la señal aérea del canal 6 propio, y Antena787 lo lee por Ethernet —
   nada que instalar en el PC de Windows 10 sin presupuesto que ya describe
   el PRD §25.
2. **Es una URL HTTP, no un dispositivo.** `capture_input` tipo `stream`
   contra `http://<dispositivo>/auto/v<canal>` entrega el TS crudo sin
   ninguna biblioteca de por medio — cumple ADR 0002 sin esfuerzo, a
   diferencia de la ruta DVB (necesita `dvbv5-zap` como binario externo) o
   la ruta BDA (atada al filtro del fabricante en Windows).
3. **`signal-compare` queda servido de verdad.** Es exactamente el escenario
   que exige ADR 0009: la señal que se compara viene de después del
   transmisor, capturada por aire de forma independiente al PC de playout.
4. **No compite con el retorno ya existente.** El monitoreo por streaming que
   ya tiene CAtv (driver `stream`) sigue funcionando igual; el HDHomeRun es
   la segunda pierna, más barata que resolver los drivers DVB/BDA de una
   tarjeta cuyo modelo todavía no se confirma.

**Lo que hay que verificar antes de comprarlo**: que el canal 6 de CAtv llegue
con suficiente señal al sitio donde se instale el HDHomeRun (necesita antena
propia, no puede jalar del excitador por IP) — es la misma prueba de "¿ves las
barras de color?" que rige todo el asistente (PRD principio 2).
