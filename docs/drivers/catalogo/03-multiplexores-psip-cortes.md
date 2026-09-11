# Catálogo — multiplexores, PSIP/EPG, cortes y encoders externos

Este catálogo cubre cuatro familias que tocan la salida `udp-ts` y el driver
`scte104-tcp`/`gpi-out` del PRD [§10](../../PRD.md#10--los-drivers): los
multiplexores ATSC 1.0 de estación chica, los generadores de PSIP/EPG, la
señalización de cortes (SCTE-104, SCTE-35, GPI por relés) y los encoders
externos que reemplazan el TS directo por HDMI/SDI. Sigue el formato de
[`docs/hardware/MATRIZ.md`](../../hardware/MATRIZ.md): **ninguna marca se
asume**, el despliegue de referencia (CAtv, con el Technalogix TP1000) es
donde se prueba primero, no el molde, y cada dato lleva su fuente y su
fecha. Búsqueda web hecha el 2026-09-10; lo que no se pudo verificar en esa
sesión se marca **no encontrado** en vez de inventarse.

---

## 1 · Multiplexores / media gateways ATSC 1.0

Lo que el PRD exige de `udp-ts` (§10, citado arriba del catálogo) es lo que
cualquier equipo de esta familia va a pedir: tasa constante, PIDs y programa
fijos, PCR ≤ 40 ms, PAT/PMT a tiempo, MPEG-2 con AC-3 o MPEG L2. La columna
"entrada" es la que decide si el driver `udp-ts` de Antena787 le sirve tal
cual.

| Equipo | Función | Entrada / salida | Manejo | Manual / referencia | Driver PRD | Verificado |
|---|---|---|---|---|---|---|
| **Technalogix TP1000** | Multiplexor ATSC (media gateway), el del despliegue de referencia | 2 ASI in, 2 ASI out, 2 Ethernet (manejo + stream), según lo confirmado por Rolando y ya en [`MATRIZ.md`](../../hardware/MATRIZ.md); el manual de usuario ("TP1000 Media Gateway Platform User Manual V1.1RC") describe un chasis con hasta 3 submódulos y salidas de módulo Gigabit IP, ASI, 8-QAM y 4-OFDM — [ManualsLib](https://www.manualslib.com/manual/1357407/Technalogix-Tp1000.html), [Manualzz](https://manualzz.com/doc/60408159/technalogix-tp1000-user-manual) (2026-09-10; ambos bloquean la lectura completa por robots, solo el índice/resumen es público) | No confirmado para este modelo específico. Otros equipos Technalogix de la misma línea (amplificadores/moduladores) anuncian "Ethernet and SNMP" con pantalla táctil — [technalogix.com/digital-tv-amplifiers](https://technalogix.com/en-us/pages/digital-tv-amplifiers) (2026-09-10) — **pero eso es la línea de amplificadores, no confirma el TP1000** | Ver enlaces arriba | `udp-ts` (F2), `scte104-tcp` no aplica (el TP1000 es multiplexor, no encoder) | **no público** — el manual existe pero los agregadores lo bloquean; hace falta pedirlo al fabricante o a Rolando |
| **Technalogix (línea general)** | PSIP/PSI en la línea de amplificadores/moduladores Technalogix | ASI injection, remap, restamp, grooming, add-drop; "PSIP/PSI descriptors generator/injector with VCT" | Ethernet + SNMP, pantalla táctil a color | [technalogix.com/digital-tv-amplifiers](https://technalogix.com/en-us/pages/digital-tv-amplifiers) (2026-09-10) | — | **no público para el TP1000** — confirma que Technalogix como marca sí hace generación de PSIP en otros productos, no que el TP1000 lo haga |
| **Adtec DTA-3050** | Multiplexor/router ATSC, comparable de estación chica | GigE IP: UDP y RTP v2 (RFC 3550), 1–185 Mbps; agrega SPTS en un MPTS | Tablas: PSI, DVB SI, **ATSC PSIP** (MGT/TVCT, STT, RRT, EIT 0-3, cumplimiento A/65B) — no se confirmó SNMP/web en esta búsqueda | [manualmachine.com/adtecdigital/dta3050](https://manualmachine.com/adtecdigital/dta3050version60214manual/1501443-user-manual) (2026-09-10) | `udp-ts` (F2) | **verificado** (manual público) — es el comparable más claro: acepta UDP/RTP igual que el diseño de `udp-ts` y **sí genera PSIP** él mismo |
| **Harmonic ProStream 1000/9100** (ex Thomson Grass Valley) | Plataforma de multiplexado y scrambling, con módulo 8VSB RF-in opcional | PID remap/filtro/prioridad; inserción y **generación** de tablas PSI/SI; módulo 8-VSB recibe 4 señales ATSC RF y saca 4 TS MPEG-2 | Guía de software menciona "regenerating PSIP tables" — manejo no confirmado en esta búsqueda | [Datasheet ProStream 1000](https://3lsystems.ru/download/mux/Harmonic%20ProStream1000_Datasheet.pdf), [Guía HW](http://www.drinianet.com/uploads/ProStream_1000_v5_0_HW_Guide_Rev_B-1396618254.pdf) (2026-09-10) | `udp-ts` (F2) | **verificado** (datasheet y guía públicos) — de gama más alta que el TP1000, típico de mercado mediano/grande, no de estación chica de 1-2 canales |
| **Sencore AG 2700** | Gateway RF ATSC 1.0/3.0 a ASI/IP, para retransmisión (turnaround), no multiplexor de cabecera propio | RF in (ATSC 1.0/3.0) → ASI e IP out | No confirmado en esta búsqueda | [sencore.com/product/atsc-10-30-rf-demodulator-gateway-ag-2700](https://www.sencore.com/product/atsc-10-30-rf-demodulator-gateway-ag-2700/) (2026-09-10) | No aplica directo — es para recibir otra señal ATSC, no para tomar el TS de un playout | **no encontrado** que genere PSIP propio; línea de Sencore hoy está centrada en ATSC 3.0 |
| **Linear Acoustic LEX-2000** | Encoder/multiplexor ATSC 1.0 de bajo costo, usado en LPTV y como respaldo ("disaster recovery") | No detallado en esta búsqueda (entrada de audio/video + salida ASI, según la familia LEX) | No confirmado | [av-iq.com LEX-2xxx datasheet](http://cdn-docs.av-iq.com/dataSheet/LEX-2xxx%20series_Datasheet.pdf) (2026-09-10) — el datasheet encontrado es de la serie LEX-2xxx, no confirma el LEX-2000 exacto | `udp-ts` (F2), si acepta IP como entrada | **no encontrado** el detalle de entrada; queda como comparable nombrado, no verificado a fondo |
| **Ateme, Thomson (marca actual)** | Nombrados en la tarea como comparables | — | — | — | — | **no encontrado** — la búsqueda no dio un producto de multiplexor de estación chica ATSC 1.0 específico de Ateme; Thomson Grass Valley es la marca histórica detrás de ProStream, ya cubierta arriba bajo Harmonic |

---

## 2 · PSIP / EPG

**Lo que exige la FCC.** Para Class A, la búsqueda apunta a **ATSC A/65C,
Annex B** como el estándar de PSIP que aplica, con el nombre corto de canal
(call sign) y el TSID/BSID como obligatorios — [fcc.gov/media/television/class-a-television](https://www.fcc.gov/media/television/class-a-television),
[eCFR Part 73 Subpart J](https://www.ecfr.gov/current/title-47/chapter-I/subchapter-C/part-73/subpart-J)
(2026-09-10). **La cita exacta del CFR (se esperaba 47 CFR 73.682(d)) no se
confirmó en esta sesión de búsqueda** — el presupuesto de búsquedas web se
agotó antes de leer el texto completo del eCFR. Queda como pendiente de
verificación directa antes de que el perfil `us-fcc` (§12 del PRD) dependa de
este dato.

**Generadores de PSIP y qué reciben:**

| Producto | Función | Entradas que acepta | Manejo | Referencia | Verificado |
|---|---|---|---|---|---|
| **Triveni GuideBuilder** | Generador de PSIP/EPG líder de la industria para ATSC 1.0/3.0 | "Automatic inputs from TMS, Rovi, **PMCP**, and other schedule providers" (PMCP es el estándar, TMS/Rovi son proveedores de datos de guía) | Editor web multiusuario ("Program Editor"), visible en PC/tablet/móvil | [trivenidigital.com/guidebuilder-broadcast](https://www.trivenidigital.com/products/guidebuilder-broadcast.php) (2026-09-10) | **verificado** |
| **PMCP (ATSC A/76)** | El estándar de intercambio, no un producto | Esquema **XML** para describir canales virtuales y eventos de programación; interfaz simple entre el generador de PSIP y la fuente de datos (el sistema de automatización/tráfico) por Ethernet | — | [tvtechnology.com Thales A/76 PMCP](https://www.tvtechnology.com/opinions/thales-announces-a76-pmcp-support-for-pearl-psip-products), [atsc.org PMCP press release](https://www.atsc.org/news/new-atsc-pmcp-standard-powers-psip-press-release/) (2026-09-10) | **verificado** |
| **TitanTV / Rovi** | Fuente de datos de guía (no genera PSIP, alimenta al generador) | — | — | Nombrado en la tarea; no se verificó en esta búsqueda cómo entrega los datos hoy (Rovi cambió de nombre varias veces — TMS/Gracenote/Rovi) | **no encontrado** el formato exacto de entrega actual |

**Qué tendría que emitir Antena787.** Con la evidencia de arriba, la ruta
más corta es **PMCP (XML, ATSC A/76)**: es el estándar hecho justo para esto
—que un sistema de automatización describa su programación a un generador de
PSIP— y es lo que GuideBuilder declara aceptar de forma nativa. **XMLTV**
(el formato que usan Kodi, Plex y los guías de IPTV) no aparece mencionado
como entrada de GuideBuilder en la documentación encontrada; sirve para
alimentar guías web o de terceros, no para el flujo PSIP/A-76 de estación
de TV. Antena787 ya tiene el dato: el resolver (PRD §9) conoce el plan de
emisión completo, así que emitir PMCP es exportar ese plan en el esquema de
A/76 — **no está construido y no aparece como driver en el PRD hoy**; es un
hueco a proponer como driver nuevo de la familia "fichas y carátulas" o una
familia propia ("guía de programación"), con el mismo proceso de
`docs/drivers/README.md` (issue con la plantilla `driver.yml`).

---

## 3 · Señalización de cortes

### SCTE-104 sobre TCP

| Dato | Detalle | Fuente | Verificado |
|---|---|---|---|
| Naturaleza | "Automation System to Compression System" — API de señalización entre un sistema de automatización (Antena787) y un sistema de compresión (el encoder) | [linkedin.com/pulse implementing-scte-104-over-tcp](https://www.linkedin.com/pulse/implementing-scte-104-over-tcp-live-video-encoder-james-heliker) (2026-09-10) | **verificado** — coincide con ADR [0004](../adr/0004-scte104-to-the-encoder.md) |
| Puerto TCP por defecto | **5167**, según el datasheet del inserter de EEG (AI-Media) | [ai-media.tv SCTE-104 Inserter datasheet](https://www.ai-media.tv/wp-content/uploads/Data-Sheets_SCTE_104_Inserter.pdf) (2026-09-10) | **verificado por un fabricante**, no es un número asignado por IANA — puede variar por equipo; a confirmar con el encoder real de CAtv |
| Entrega | El encoder inyecta el SCTE-104 recibido en VANC, de inmediato o con un GPI de disparo para sincronía de cuadro exacta | mismo datasheet EEG | **verificado** |
| Biblioteca de referencia | `astronautlabs/scte104` — implementación completa del protocolo SCTE-104 TCP/IP, **en TypeScript**, cliente y servidor | [github.com/astronautlabs/scte104](https://github.com/astronautlabs/scte104) (2026-09-10) | **verificado, pero no es Go** |
| Biblioteca Go | **No se encontró** una biblioteca Go de SCTE-104 en esta búsqueda | búsqueda GitHub (2026-09-10) | **no encontrado** — hay que escribirla; el ADR 0004 ya lo anticipa ("no Go library exists, but the specification is clear") |

### SCTE-35 en el TS

Es la señal que el propio *encoder* escribe en el transport stream a partir
del SCTE-104 recibido (ADR 0004) — Antena787 no lo escribe. La biblioteca Go
que sí existe y está mantenida es **`Comcast/scte35-go`**, para crear,
decorar y analizar mensajes SCTE-35 conforme a ANSI/SCTE 35 hasta 2022b —
[github.com/Comcast/scte35-go](https://github.com/Comcast/scte35-go)
(2026-09-10, **verificado**). Útil si algún día hace falta *leer* SCTE-35 del
TS de retorno (verificación), no para emitirlo.

### GPI/GPO por contacto — placas de relés

Cómo se llega decide el driver, no la marca (regla del `README.md` de
drivers). Estas placas son ejemplos de familia, no un requisito de marca.

| Marca / modelo | Cómo se controla | Manual | Biblioteca Go sin CGo | Verificado |
|---|---|---|---|---|
| **Numato Lab (USB, ej. 8/16 canales)** | Comandos ASCII simples por **puerto COM virtual** (USB CDC): p. ej. `relay on 0`\<CR\>. Terminal serial cualquiera sirve | [numato.com/docs/8-channel-usb-relay-module](https://numato.com/docs/8-channel-usb-relay-module/) (2026-09-10) | **`go.bug.st/serial`** — cross-platform, evita cgo, salvo que la enumeración de puertos USB en macOS (IOKit) sí necesita cgo (no aplica: el despliegue de referencia es Windows) | **verificado** |
| **Numato Lab (Ethernet, 3/8/16/32 canales)** | Interfaz web, **Telnet**, o enlaces **HTTP** predefinidos con la IP del módulo (usuario/clave por defecto `admin`) | [numato.com/kb/configuring-and-controlling-numato-lab-ethernet-poe-modules](https://numato.com/kb/configuring-and-controlling-numato-lab-ethernet-poe-modules/) (2026-09-10) | `net/http` de la biblioteca estándar de Go — no hace falta biblioteca externa | **verificado** |
| **Denkovi (USB, varios canales)** | **Puerto COM virtual**, modo BitBang, o **USB HID** según el modelo; hay una herramienta de línea de comandos propia del fabricante | [denkovi.com/usb-16-relay-board](http://denkovi.com/usb-16-relay-board) (2026-09-10) | Igual que Numato USB: `go.bug.st/serial` si el modelo usa COM virtual; si es HID puro, ver fila de SainSmart abajo | **verificado el modo de control, no probado con Go** |
| **SainSmart (16 canales, USB, descontinuado)** | **USB HID** puro (no COM virtual) — ejemplos del fabricante en C#/hidsharp y Java | [sainsmart.com 16-channel-usb-hid-relay](https://www.sainsmart.com/products/16-channel-usb-hid-programmable-control-relay-module) (2026-09-10) | Problema real: las bibliotecas Go de HID en macOS/Windows (`karalabe/usb`, `sstallion/go-hid`, `bearsh/hid`) **usan CGo**; la única HID pura sin CGo encontrada, `zserge/hid`, **solo soporta Linux** — [github.com/zserge/hid](https://github.com/zserge/hid) (2026-09-10) | **riesgo señalado**: un relé HID puro no tiene hoy un camino limpio sin CGo en Windows — preferir COM virtual (Numato/Denkovi) o Ethernet (Numato/KMtronic) por encima de HID |
| **KMtronic** | Placas Ethernet y serial, con ejemplos de software propios del fabricante | Referencia encontrada solo de forma indirecta: `kmtronic.com/software-examples.html`, citada por la documentación de Denkovi (2026-09-10) | Igual patrón: `net/http` si es Ethernet, `go.bug.st/serial` si es serial | **no encontrado** — no se pudo abrir la página del fabricante en esta sesión (presupuesto de búsqueda agotado); protocolo exacto por confirmar |

**Conclusión para el driver `gpi-out`:** el camino más seguro para
mantenerse dentro del ADR [0002](../adr/0002-no-cgo.md) (sin CGo) es
**COM virtual o Ethernet**, no HID puro — Numato y Denkovi ofrecen ambos
caminos según el modelo que se compre, y eso decide cuál se recomienda en
la matriz de hardware el día que haya una placa en la mano.

---

## 4 · Encoders externos (HDMI/SDI en vez del TS directo)

Es la alternativa a que Antena787 mande el TS por `udp-ts`: la estación
pone un encoder de hardware con entrada de video en banda base (HDMI o SDI)
y salida ASI/IP hacia el multiplexor. **Esto le pediría a Antena787 una
salida que hoy no tiene** (SDI/HDMI en banda base, con una tarjeta de
captura de salida) — el diseño actual asume que Antena787 entrega el TS ya
comprimido por IP (§10, driver `udp-ts`), no video sin comprimir.

| Equipo | Entrada | Salida | Implicación | Referencia | Verificado |
|---|---|---|---|---|---|
| **Thor Broadcast H-1HD-EMS / H-1HD-EMH** | 1 canal HD-SDI o HDMI sin comprimir | MPEG-2 o H.264 multiplexado a **ASI**, conectable directo a un modulador QAM-IP | Necesitaría que la PC de Antena787 saque HDMI/SDI (tarjeta de salida de video), no solo un archivo o un TS | [thorbroadcast.com H-1HD encoder](https://thorbroadcast.com/product/1-ch-sdi-or-hdmi-hd-encoder-mpeg2-h264.html) (2026-09-10) | **verificado el producto**, no probado |
| **Thor Broadcast (2×HDMI + 2×SDI a RF/IP)** | 2 HDMI + 2 HD/SD-SDI | RF (QAM/ATSC) + IPTV (UDP/RTP/RTSP) + ASI | Igual: exige salida de video en banda base de la fuente | [thorbroadcast.com 2-hdmi-2-sdi](https://thorbroadcast.com/product/2-hdmi-2-hd-sd-sdi-to-rf-coax-modulators-8230.html) (2026-09-10) | **verificado el producto**, no probado |
| **QuestTel B-QAM-SDI-IP-2CH-LL** | 2 canales HD-SDI | ASI (2 puertos) + 1 puerto IP UDP | Mismo caso — encoder de hardware entre la fuente en banda base y el multiplexor | [questtel.com 2ch-sdi-qam-modulator](https://questtel.com/unit/2ch-sdi-qam-modulator-with-iptv-encoder/) (2026-09-10) | **verificado el producto**, no probado |

**Lectura para el diseño:** ninguno de estos encoders cambia el driver
`udp-ts` — son la ruta que usaría una estación que hoy alimenta su
transmisor con una cámara o un mezclador en vivo, no con un playout de
archivo. Para CAtv esto no aplica: la cadena confirmada (§25 del PRD) va de
Antena787 al TP1000 **por IP**, sin encoder de banda base en medio. Se deja
documentado por si otra instalación de referencia sí depende de un encoder
externo con entrada HDMI/SDI.

---

## Preguntas concretas para el ingeniero de CAtv sobre el TP1000

1. **¿El TP1000 recibe el TS por unicast a una IP y puerto, o por
   multicast?** Si es multicast, ¿con qué dirección de grupo y qué TTL
   espera en la red? (Decide si `udp-ts` necesita el modo multicast de
   F2-114 encendido por defecto para CAtv.)
2. **¿Qué PIDs y número de programa espera en el TS de entrada** — los fija
   él (remapea) o tiene que llegar ya con los valores exactos que él
   configuró? (VLC-PARIDAD.md, pregunta 3 a Rolando, sigue abierta.)
3. **¿El TP1000 genera PSIP/PSI (TVCT, EIT) él mismo,** o necesita que el
   TS de entrada ya lo traiga? La documentación pública de la línea
   Technalogix (no del TP1000 específico) dice que sí generan PSIP en otros
   modelos — hay que confirmarlo para este equipo puntual.
4. **¿Acepta una guía de programación por red, y en qué formato** — PMCP
   XML (A/76), un archivo, o ninguno y la guía la pone otro equipo aguas
   abajo? Si el TP1000 no toma guía, el generador de PSIP (si existe uno en
   la cadena de CAtv) es un equipo distinto a identificar.
5. **¿Tiene SNMP o página web de manejo,** y en qué de los dos puertos
   Ethernet (el de manejo o el del stream)? El manual público no lo
   confirma; solo lo dice para otra línea de producto de la misma marca.
6. **¿El TP1000 acepta SCTE-35 en el TS de entrada y lo deja pasar,** o lo
   descarta al remultiplexar? Esto decide si el corte tiene que llegar por
   `scte104-tcp` al encoder de CAtv (como ya asume el ADR 0004) o si
   también hace falta que el TP1000 no rompa la señal en el camino.
