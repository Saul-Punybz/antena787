# Catálogo de bibliotecas Go y protocolos para los drivers

Este catálogo es insumo para quien escriba un driver (PRD [§10](../../../PRD.md#10--los-drivers)),
no una decisión tomada: cada fila lleva su fuente para que se pueda volver a
verificar. Todo lo que aparece aquí tiene que respetar la regla dura del
proyecto — **sin CGo, nunca** (ADR [0002](../../adr/0002-no-cgo.md)) — porque el
momento en que una dependencia enlaza C, `GOOS=windows go build` desde Linux
deja de producir un binario de Windows, y la compilación cruzada trivial es
la razón por la que el proyecto es Go.

Verificado el 10 de septiembre de 2026, con `gh api` contra la API de GitHub
(licencia, estrellas, fecha del último *push*/*tag*, archivado o no) y lectura
del `README`/`go.mod`/código fuente de cada repositorio para confirmar CGo o
su ausencia. El nombre de cada biblioteca enlaza a su fuente. Donde no se
pudo verificar algo con certeza, la fila dice **dudoso** en vez de inventarlo.

---

## SNMP (transmisor, telemetría)

| Biblioteca | Cubre | Puro Go | Licencia | Último release/push | Madurez | Para qué driver | Estado |
|---|---|---|---|---|---|---|---|
| [`gosnmp/gosnmp`](https://github.com/gosnmp/gosnmp) | Get/GetNext/GetBulk/Walk/Set, **v1/v2c/v3**, envío y recepción de **Traps** (`SendTrap`, `Listen`) | sí | BSD-style (`LICENSE`) | push 2026-09-07 · tag v1.44.0 | 1,253 estrellas, activo | `snmp` (transmisor), `snmp-trap` (alertas) | verificado |
| [`sleepinggenius2/gosmi`](https://github.com/sleepinggenius2/gosmi) | Parser de MIBs en Go, recomendado por el propio README de gosnmp | sí | MIT | push 2024-04-24 | 115 estrellas, ~2.4 años sin cambios | opcional, resolución de nombres de OID | verificado, mantenimiento dudoso |

**En la práctica no hace falta compilar MIBs.** gosnmp mismo lo dice: *"I
don't have any plans to write a mib parser"*. Los OIDs de un transmisor
(potencia directa/reflejada, temperatura, estado) son un puñado, conocidos y
estables por fabricante — se escriben a mano como constantes numéricas. gosmi
queda como opción para quien quiera nombres de MIB en vez de OIDs numéricos,
pero no es una dependencia que el driver necesite.

**Recomendación de familia: `gosnmp/gosnmp`, con OIDs numéricos a mano — sin
parser de MIB.**

---

## Serial y USB-serial

| Biblioteca | Cubre | Puro Go | Licencia | Último release/push | Madurez | Para qué driver | Estado |
|---|---|---|---|---|---|---|---|
| [`go.bug.st/serial`](https://github.com/bugst/go-serial) | Puerto serial y USB-serial; paquete `enumerator/` que **sí lista puertos en Windows, macOS y Linux** | sí — `go.mod` solo depende de `golang.org/x/sys` | BSD-3-Clause | push 2026-07-15 | 873 estrellas, activo | `gpi-serial`, `serial-usb`, camino serial de `sage-endec`/`dasdec` | verificado |
| [`tarm/serial`](https://github.com/tarm/serial) | Serial básico | sí | BSD-3-Clause | push 2023-10-16 | 1,694 estrellas, **~3 años sin cambios** | — | verificado, abandonado |
| [`jacobsa/go-serial`](https://github.com/jacobsa/go-serial) | Serial básico | sí | Apache-2.0 | push 2021-09-29 | 648 estrellas, **~5 años sin cambios** | — | verificado, abandonado |

`go.bug.st/serial` es la única de las tres con actividad reciente, y es la
única con enumeración de puertos multiplataforma confirmada por código
(archivos separados `serial_darwin.go`, `serial_linux.go`, `serial_windows.go`,
`serial_bsd.go` más el paquete `enumerator/`). `tarm/serial` y
`jacobsa/go-serial` tienen más estrellas por antigüedad, no por vigencia.

**Recomendación de familia: `go.bug.st/serial`.**

---

## HID USB (placas de relés)

| Biblioteca | Puro Go | Licencia | Último release/push | Madurez | Para qué driver | Estado |
|---|---|---|---|---|---|---|
| [`karalabe/hid`](https://github.com/karalabe/hid) | **no** — build tag exige `cgo` y compila `hidapi`+`libusb` en C (`#cgo CFLAGS`/`LDFLAGS` en `hid_enabled.go`) | BSD-3 (con nota: `libusb` es LGPL) | push 2026-03-15, pero **repositorio archivado** | 312 estrellas | — (rompe ADR 0002) | verificado |
| [`sstallion/go-hid`](https://github.com/sstallion/go-hid) | **no** — trae `hid_darwin.c`, `hid_libusb.c`, `hid_linux.c` | BSD-2-Clause | push 2025-05-23 | 98 estrellas | — (rompe ADR 0002) | verificado |
| [`bearsh/hid`](https://github.com/bearsh/hid) | **no** — el propio README dice *"wrapped using CGO"*, vendorea `hidapi`+`libusb` | sin licencia clara (NOASSERTION) | push 2025-05-19 | 10 estrellas | — (rompe ADR 0002) | verificado |

**Conclusión dura: no existe una biblioteca HID USB pura en Go hoy.** Las
tres opciones conocidas dependen de CGo (vía `hidapi`/`libusb` en C), y
`karalabe/hid` — la más usada — está además archivada. Cualquiera de las tres
rompe el ADR 0002.

**Recomendación de familia: ninguna biblioteca HID.** El equivalente dentro
de la restricción sin-CGo es usar **placas de relés que hablen serial**
(USB-serial estándar, vía `go.bug.st/serial`) en vez de HID USB nativo — es
exactamente el mismo patrón que ya cubre `gpi-serial`, y evita por completo
la dependencia de `libusb`/`hidapi`.

---

## GPIO (Linux)

| Biblioteca | Puro Go | Licencia | Último release/push | Madurez | Para qué driver | Estado |
|---|---|---|---|---|---|---|
| [`warthog618/go-gpiocdev`](https://github.com/warthog618/go-gpiocdev) (antes `gpiod`, mismo autor, sucesor directo) | sí — usa el *character device* `/dev/gpiochipN` vía llamadas al kernel Linux, sin CGo | MIT | push 2026-02-16 | 517 estrellas, activo | `gpi-gpio`, cruce de contactos de `gpi-estado` | verificado |
| [`periph.io/x/conn` / `x/host`](https://periph.io) (repos `periph/conn`, `periph/host`) | sí — `go.mod` sin dependencias C; único hallazgo de "cgo" en el repo es el flag `CGO_ENABLED=0` del propio CI | Apache-2.0 | push 2026-04-07 | 90 y 73 estrellas — pequeño tras la reorganización del proyecto original `google/periph` (archivado, 1,730 estrellas históricas) | alternativa si algún driver necesita I2C/SPI además de GPIO | verificado |

`go-gpiocdev` es la opción más ajustada al caso de uso: solo GPIO, sin la
superficie de I2C/SPI/1-Wire que trae periph.io, y con más tracción reciente
que la reorganización de periph.

**Recomendación de familia: `warthog618/go-gpiocdev`.**

---

## Syslog, HTTP, WebSocket

| Protocolo | Qué usar | Para qué driver | Nota |
|---|---|---|---|
| HTTP | `net/http` (estándar) | `dasdec`, `snmp` con página web, canales de avisos | Nada que evaluar; es la biblioteca estándar de Go. |
| WebSocket | **ya hay uno propio en el repo**, hecho a mano sobre `net/http` ([`internal/api/ws.go`](../../../internal/api/ws.go), [`internal/app/bus.go`](../../../internal/app/bus.go)) | monitor por streaming, panel en vivo | No se necesita `gorilla/websocket` ni `nhooyr.io/websocket`: el protocolo RFC 6455 ya está resuelto en la base de código. |
| Syslog — **emitir** | `log/syslog` (estándar), con `Dial("udp", ...)` o `Dial("tcp", ...)` explícito | canal de avisos hacia sistemas externos | El paquete estándar solo falla en Windows cuando se usa sin `Dial` (intenta un socket Unix local); con `Dial` a una red explícita funciona en las tres plataformas. |
| Syslog — **recibir** | escribir el listener a mano (UDP/TCP + parser RFC 3164) | `syslog` (alertas de emergencia) | [`mcuadros/go-syslog`](https://github.com/mcuadros/go-syslog) (MIT, 547 estrellas) existe pero no tiene push desde 2023-11-09 — **dudoso** en mantenimiento. RFC 3164 es un formato de texto simple y estable desde 2001; un receptor mínimo es poco código y sin la dependencia. |

---

## MPEG-TS: demux/mux, SCTE-35, SCTE-104

| Biblioteca | Cubre | Puro Go | Licencia | Último release/push | Madurez | Para qué driver | Estado |
|---|---|---|---|---|---|---|---|
| [`asticode/go-astits`](https://github.com/asticode/go-astits) | Demux y mux de TS: lee y escribe PAT/PMT y PID por PID, con soporte de PCR en las estructuras | sí | MIT | push 2026-08-15 | 617 estrellas, activo | `red`/`udp-ts` si algún día hace falta mux propio fuera de ffmpeg | verificado |
| [`Comcast/gots`](https://github.com/Comcast/gots) (v3) | Lectura de PSI/TS en general, incluye demux de secciones SCTE-35 | sí | MIT (archivo `LICENSE`; la API de GitHub lo marca NOASSERTION) | push 2026-08-11 | 310 estrellas, activo | referencia general de TS; no sustituye `internal/ts` | verificado |
| [`Comcast/scte35-go`](https://github.com/Comcast/scte35-go) | Construcción y parseo de secciones SCTE-35 en Go puro | sí | Apache-2.0 | push 2026-09-09 | 47 estrellas, activo | `signal-compare` verificando SCTE-35 en el retorno de aire | verificado |
| **SCTE-104** | — | — | — | — | — | `scte104-tcp` | **no existe ninguna biblioteca Go** — búsqueda en GitHub (`scte-104 language:go`, `scte104 language:go`) no devuelve nada salvo este mismo repositorio |

**El análisis de PCR/continuidad/tasa ya está escrito a mano en el repo, y
no hace falta go-astits para eso.** [`internal/ts/ts.go`](../../../internal/ts/ts.go)
es un lector propio de TS de 188 bytes por paquete que ya mide continuidad,
PCR, tasa y saltos de DTS/PTS, usado por [`internal/f0/analyze.go`](../../../internal/f0/analyze.go)
para el criterio F0-TS. `go-astits` queda disponible para el día que se
necesite construir o inspeccionar TS a bajo nivel por fuera de ffmpeg, pero
v1 no lo necesita: ffmpeg hace el mux, `internal/ts` hace el análisis.

**SCTE-35 sí tiene un lugar concreto donde una biblioteca ayuda: verificar,
no generar.** ADR [0004](../../adr/0004-scte104-to-the-encoder.md) fija que
Antena787 nunca escribe SCTE-35 — manda SCTE-104 al encoder y este genera el
SCTE-35. Pero `signal-compare` (ADR [0009](../../adr/0009-truth-is-the-transmitted-signal.md))
lee el retorno de aire, y ahí sí tiene sentido parsear el SCTE-35 que quedó
en el TS transmitido para comprobar que el encoder efectivamente convirtió
el SCTE-104 en el marcador correcto. Para eso, `Comcast/scte35-go` es más
ajustado que `Comcast/gots`, que es una librería de TS general con un demux
de SCTE-35 como una pieza más.

**SCTE-104 se escribe a mano, como ya adelantaba el ADR 0004.** Es un mensaje
TCP autocontenido (SMPTE 2010) sin PCR, PTS ni contador de continuidad que
mantener — mucho más simple que SCTE-35. El trabajo es: framing del mensaje,
`init_request`/`init_response`, el bloque `multiple_operation_message` con
sus `splice_request_data` (evento, PTS opcional, tipo de *splice*), y poco
más. Es codificación de una estructura binaria fija, no un protocolo con
estado — cientos de líneas, no miles.

**Recomendación de familia:** `go-astits` disponible pero no urgente para v1
(el análisis ya existe); `Comcast/scte35-go` para cuando `signal-compare`
necesite leer SCTE-35 del retorno de aire; **SCTE-104 escrito a mano**, sin
alternativa en el ecosistema Go.

---

## SRT, RTMP, RTSP, HLS, WebRTC

| Biblioteca | Cubre | Puro Go | Licencia | Último release/push | Madurez | Para qué driver | Estado |
|---|---|---|---|---|---|---|---|
| [`datarhei/gosrt`](https://github.com/datarhei/gosrt) | SRT (handshake v4/v5, modo mensaje) — el README lo describe como *"pure Go with minimal dependencies"* | sí | MIT | push 2026-09-08 | 205 estrellas, activo | `srt-listen`, `internet` (salida SRT) | verificado |
| [`bluenviron/gortmplib`](https://github.com/bluenviron/gortmplib) | RTMP, del mismo equipo que `mediamtx` | sí | MIT | push 2026-09-08 | 23 estrellas — **joven**, pero mantenimiento activo confirmado | `internet` (salida RTMP), `rtmp-listen` | verificado |
| [`yutopp/go-rtmp`](https://github.com/yutopp/go-rtmp) | RTMP | sí | BSL-1.0 (Boost, no la licencia permisiva habitual en Go) | push 2024-07-21, tag v0.0.7 | 436 estrellas, **~2 años sin cambios** | — | verificado, estancado |
| [`nareix/joy4`](https://github.com/nareix/joy4) | RTMP y más (contenedor multi-protocolo) | sí | MIT | push 2021-09-08 | 2,715 estrellas, **~5 años sin cambios** | — | verificado, abandonado |
| [`bluenviron/gortsplib`](https://github.com/bluenviron/gortsplib) | RTSP cliente y servidor | sí | MIT | push 2026-09-10 | 937 estrellas, muy activo | entrada `url` tirando de `rtsp://` (F2-116) | verificado |
| [`pion/webrtc`](https://github.com/pion/webrtc) | WebRTC completo (SDP, ICE, DTLS, SCTP, media) | sí | MIT | push 2026-09-09 | 16,772 estrellas, muy activo, estándar de facto en Go | ventana local de baja latencia (F2-117) | verificado |
| [`grafov/m3u8`](https://github.com/grafov/m3u8) | Generación/parseo de playlists HLS | sí | BSD-3-Clause | push 2025-09-11, luego **archivado** | 1,289 estrellas | `internet` (salida HLS), `hls-daterange` | verificado, sin sucesor mantenido a la vista |

**RTMP es la familia más floja del lote — y aquí las estrellas mienten.** El
líder histórico (`joy4`) lleva cinco años sin cambios; `yutopp/go-rtmp` lleva
dos y usa una licencia poco común (Boost); el más nuevo, `bluenviron/gortmplib`,
tiene pocas estrellas pero *push* de esta semana y viene del equipo que
sostiene `gortsplib` y `mediamtx`, proyectos serios y activos.

**HLS no tiene una biblioteca de generación de playlists vigente**:
`grafov/m3u8` era la referencia y está archivada sin sucesor. El formato es
texto plano y trivial (`#EXTINF`, `#EXT-X-DATERANGE` para las marcas de
`hls-daterange`) — escribirlo a mano es más barato que adoptar una
dependencia archivada, el mismo criterio que ya aplica el proyecto en otras
piezas pequeñas.

**Recomendación por familia:** SRT → `datarhei/gosrt`. RTMP → `bluenviron/gortmplib`
(pese a las pocas estrellas, por venir de un equipo activo — para el driver
`internet` y como receptor en `rtmp-listen`). RTSP → `bluenviron/gortsplib`
(para el `url` de entrada que tira de `rtsp://`, F2-116). HLS → escrito a
mano (playlists + *daterange*, sin biblioteca). WebRTC → `pion/webrtc`, para
la ventana local de baja latencia (F2-117).

---

## EAS/SAME y CAP

No existe ningún decodificador SAME en Go: la búsqueda en GitHub
(`SAME EAS decoder language:go`, `ZCZC go`) no devuelve nada. Esto confirma
lo que ya fija ADR [0010](../../adr/0010-eas-integrate-the-endec-never-replace-it.md):
la capa de evidencia de `signal-compare` necesita un decodificador SAME
propio, en Go puro, contra el retorno de aire — tono FSK a 520.83 baudios,
cabecera `ZCZC-ORG-EEE-PSSCCC+TTTT-JJJHHMM-LLLLLLLL-` y los tonos de fin de
mensaje. Las referencias son de lectura, no de enlace — el ADR es explícito
en que nada se enlaza:

- [`cuppa-joe/dsame`](https://github.com/cuppa-joe/dsame) y su fork activo
  [`jamieden/dsame3`](https://github.com/jamieden/dsame3) — Python, la
  implementación más legible del decodificador SAME. Verificado 2026-09-10.
- [`kripton/multimon-ng`](https://github.com/kripton/multimon-ng) — C, con
  plugin SAME entre otros modos (POCSAG, AFSK). El propio proyecto marca su
  soporte de EAS/SAME como **alpha**, con aviso explícito de no confiar en él
  para recepción real de alertas. Verificado 2026-09-10.

Para CAP (XML, esquema OASIS CAP-v1.2), la búsqueda en GitHub
(`"common alerting protocol" language:go`) solo encuentra bibliotecas
pequeñas y muertas: [`IBM/cap`](https://github.com/IBM/cap) (2019, 21
estrellas), [`alerting/go-cap`](https://github.com/alerting/go-cap) (2018,
5), [`mark-adams/cap-go`](https://github.com/mark-adams/cap-go) (2018, 3),
[`TheTannerRyan/cap`](https://github.com/TheTannerRyan/cap) (v1.0.0, 2019-01-10,
sin actividad en 5+ años) — ninguna con actividad reciente. Ninguna vale la
dependencia; `encoding/xml` (estándar) contra el esquema es suficiente dado
lo simple del formato.

**Los feeds, verificados 2026-09-10:**

- [**NOAA / NWS**](https://www.weather.gov/documentation/services-web-api) —
  `api.weather.gov/alerts` está confirmado abierto, sin clave, y soporta
  `Accept: application/cap+xml` además de JSON-LD; recomienda no más de una
  petición cada 30 segundos y expone una ventana de 7 días. Es el feed de
  poll directo para `cap-poll` en Estados Unidos.
- [**IPAWS (FEMA)**](https://www.fema.gov/emergency-managers/practitioners/integrated-public-alert-warning-system/all-hazards-information-feed) —
  el *All-Hazards Information Feed* público existe, pero el acceso requiere
  registro en el "IPAWS User Portal" (contacto `fema-ipaws-lab@fema.dhs.gov`);
  no hay una URL de feed abierta y documentada sin ese paso. `cap-poll` contra
  IPAWS depende de esa gestión previa, a diferencia de NOAA.

**Recomendación de familia:** ambos se escriben a mano. El decodificador SAME
es demodulación FSK simple sobre el audio del retorno de aire, ya previsto en
ADR 0010 como parte de la capa 2 de `signal-compare`. El poller de CAP es,
como ya lo dice el mismo ADR, "unas pocas centenas de líneas" contra
`encoding/xml` — el esquema es público y estable; NOAA se puede consultar sin
registro, IPAWS necesita el trámite con FEMA primero.

---

## NTP, mDNS, SSDP

| Biblioteca | Cubre | Puro Go | Licencia | Último release/push | Madurez | Para qué driver | Estado |
|---|---|---|---|---|---|---|---|
| [`beevik/ntp`](https://github.com/beevik/ntp) | Cliente NTP/SNTP (RFC 5905) | sí | BSD-2-Clause | push 2026-02-22 · v1.5.0 (2025-09-29) | 618 estrellas, estable (el protocolo no cambia) | hora del sistema para el as-run y las marcas de tiempo | verificado |
| [`hashicorp/mdns`](https://github.com/hashicorp/mdns) | mDNS (descubrir equipos en la red local) | sí | MPL-2.0 | push 2026-08-28 | 1,371 estrellas, activo — aunque hay reportes externos de mantenimiento discontinuo | asistente de instalación, descubrir transmisor/ENDEC en la red | verificado |
| [`libp2p/zeroconf`](https://github.com/libp2p/zeroconf) | mDNS/DNS-SD (RFC 6762/6763), fork activo de `grandcat/zeroconf` que absorbe sus PRs pendientes | sí | MIT | activo | alternativa a `hashicorp/mdns` | verificado |
| [`grandcat/zeroconf`](https://github.com/grandcat/zeroconf) | mDNS/Bonjour | sí | NOASSERTION | push 2023-12-07 | 904 estrellas, **~3 años sin cambios** | — | verificado, estancado |
| [`koron/go-ssdp`](https://github.com/koron/go-ssdp) | SSDP (descubrimiento UPnP), empaquetado en Debian/Guix | sí | BSD-3-Clause | push 2026-08-22 | 135 estrellas, activo | asistente de instalación, descubrir equipos UPnP | verificado |

**Recomendación de familia:** `beevik/ntp`, `hashicorp/mdns` (con
`libp2p/zeroconf` como alternativa si el reporte de mantenimiento discontinuo
de hashicorp se confirma en la práctica) y `koron/go-ssdp` — las tres para el
asistente de instalación cuando pregunta "qué hay en la red" antes de armar
la lista de equipos candidatos (transmisor, ENDEC).

---

## Windows

[`golang.org/x/sys/windows`](https://pkg.go.dev/golang.org/x/sys/windows) —
ya está en `go.mod` como indirecta (v0.47.0) por transitividad de otra
dependencia, y es el paquete oficial del equipo de Go para todo lo que no
tiene equivalente POSIX: el Service Control Manager (`golang.org/x/sys/windows/svc`,
confirmado) para correr como servicio de Windows, y las llamadas de bajo
nivel que hacen falta para hablar con hardware por USB/serial en Windows sin
CGo. `SetThreadExecutionState` específicamente **queda como dudoso**: no se
confirmó con certeza si está expuesto ya como función del paquete o si hace
falta declararlo a mano vía `syscall`/`x/sys/windows` contra `kernel32.dll`
— cualquiera de las dos formas es sin CGo, pero la fila queda marcada para
verificación manual antes de escribir el driver de energía. DirectShow queda
fuera de esto por completo: es territorio de ffmpeg (ADR 0003), nunca de una
biblioteca Go.

---

## Resumen: una biblioteca por familia

| Familia | Elegida | Se escribe a mano |
|---|---|---|
| SNMP | `gosnmp/gosnmp` | OIDs numéricos (sin MIB) |
| Serial/USB-serial | `go.bug.st/serial` | — |
| HID USB | ninguna — no existe pura | usar placas seriales en vez de HID |
| GPIO | `warthog618/go-gpiocdev` | — |
| Syslog | `log/syslog` (emitir) | receptor (RFC 3164) |
| MPEG-TS análisis | — | ya existe: `internal/ts` |
| SCTE-35 (verificar retorno) | `Comcast/scte35-go` | — |
| SCTE-104 | — | sí, completo — no hay biblioteca en ningún lenguaje |
| SRT | `datarhei/gosrt` | — |
| RTMP | `bluenviron/gortmplib` | — |
| RTSP | `bluenviron/gortsplib` | — |
| HLS (playlists) | — | sí — formato de texto trivial |
| WebRTC | `pion/webrtc` | — |
| EAS/SAME | — | sí, completo — no existe en Go |
| CAP | — | sí — pocas centenas de líneas contra `encoding/xml`; NOAA sin registro, IPAWS con trámite FEMA |
| NTP | `beevik/ntp` | — |
| mDNS | `hashicorp/mdns` | — |
| SSDP | `koron/go-ssdp` | — |
| WebSocket | — | ya existe: `internal/api/ws.go` |
