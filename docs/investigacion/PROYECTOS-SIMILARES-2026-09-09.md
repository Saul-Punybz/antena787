# Proyectos abiertos comparables a Antena787

Investigación al 2026-09-09. Alcance de Antena787: playout + traffic
scheduling (reglas, contador de episodios, filler) + EPG/XMLTV + ingest con
normalización de loudness + (fases futuras) salida UDP-TS a multiplexor,
fuentes en vivo SRT, EAS/ENDEC, CEA-608, as-run, publicidad. Go, un binario,
AGPL-3.0, sin CGo, solo dependencia externa ffmpeg.

## 1 · Tabla comparativa (por relevancia)

| Proyecto | Lenguaje | Licencia | Últ. commit | ★ | Cubre | Usuario objetivo | Despliegue |
|---|---|---|---|---|---|---|---|
| [ffplayout](https://github.com/ffplayout/ffplayout) | Rust | GPL-3.0 | 2026-09-04 | 586 | playout, EPG export, ingest básico | estación pequeña/mediana | binario + ffmpeg, systemd, web UI |
| [jaskie/PlayoutAutomation](https://github.com/jaskie/PlayoutAutomation) | C# | GPL-2.0 | 2026-04-28 | 220 | traffic-scheduling, rundown, control de CasparCG | TV profesional (usado por TVP Polonia) | Windows, SQL Server, requiere CasparCG |
| [CasparCG/server](https://github.com/CasparCG/server) | C++ | GPL-3.0 | 2026-09-08 | 1087 | playout (motor de render gráficos+video), salida | TV profesional | Windows/Linux, sin scheduler propio |
| [CasparCG/client](https://github.com/CasparCG/client) | C++ | GPL-3.0 | 2026-06-22 | 326 | UI de control del server | TV profesional | app de escritorio |
| [nebulabroadcast/nebula](https://github.com/nebulabroadcast/nebula) | TypeScript/Python | GPL-3.0 | 2026-09-02 | 319 | MAM + traffic + playout (vía "conti") | TV/radio con equipo técnico | docker, Postgres, servicios separados |
| [immstudios/conti](https://github.com/immstudios/conti) | Python | GPL-3.0 | 2026-06-26 | 46 | playout minimalista (motor de nebula) | idem nebula | servicio Python |
| [Sofie TV Automation](https://github.com/Sofie-Automation/sofie-core) | TypeScript | MIT | 2026-09-08 | 345 | automatización de rundown en vivo (noticias), no playout 24/7 desatendido | TV profesional en vivo | docker, control de switchers/CG externos |
| [LibreTime](https://github.com/libretime/libretime) | PHP/Python | AGPL-3.0 | 2026-09-09 | 942 | traffic-scheduling + playout de radio, calendario | radio (comunitaria a profesional) | docker, Liquidsoap, Icecast |
| [Airtime (sourcefabric)](https://github.com/sourcefabric/airtime) | PHP | AGPL-3.0 | 2021-07-14 (abandonado) | 630 | igual que LibreTime, es su predecesor | radio | LAMP, Liquidsoap |
| [Rivendell](https://github.com/ElvishArtisan/rivendell) | C++ | GPL-2.0 | 2026-08-26 | 242 | playout + traffic + as-run de radio, robusto y viejo (desde 2002) | radio profesional/comercial | Linux, PostgreSQL, JACK/ALSA |
| [OpenBroadcaster OBServer](https://github.com/openbroadcaster/observer) | PHP | AGPL-3.0 | 2026-08-13 | 185 | traffic-scheduling + media library + **EAS (CAP/NAAD/NOAA)** | radio/TV comunitaria pequeña (PEG) | LAMP |
| [OpenBroadcaster OBPlayer](https://github.com/openbroadcaster/obplayer) | Python | AGPL-3.0 | 2026-08-19 | 143 | playout + **alertas EAS/CAP integradas** | radio/TV comunitaria pequeña | Linux, GStreamer |
| [AzuraCast](https://github.com/AzuraCast/AzuraCast) | PHP | AGPL-3.0 | 2026-09-07 | 4025 | radio streaming (Icecast/Liquidsoap), programación de playlists — no es traffic de TV/broadcast terrestre | radio por internet | docker (stack completo) |
| [MLT Framework](https://github.com/mltframework/mlt) | C | LGPL-2.1 | 2026-09-09 | 1840 | framework de edición/composición (no playout), usado por Shotcut | herramienta/librería | librería C, bindings |
| [XMLTV](https://github.com/XMLTV/xmltv) | Perl | GPL-2.0 | 2026-06-22 | 414 | EPG — el formato y utilidades de referencia | universal | CLI Perl |
| [iptv-org/epg](https://github.com/iptv-org/epg) | JS/TS | Unlicense | 2026-09-02 | 3282 | grabadores de EPG de cientos de fuentes → XMLTV | universal | Node, cron |
| [TSDuck](https://github.com/tsduck/tsduck) | C++ | BSD-2-Clause | 2026-09-09 | 1078 | toolkit MPEG-TS: mux, PSI/SI, monitoreo, SCTE-35 | broadcast profesional / herramienta | binario + librería (C++/Python) |
| [DVBlast](https://github.com/videolan/dvblast) | C | GPL-2.0 | 2026-06-15 | 73 | demux/streaming MPEG-TS por IP | broadcast DVB | binario Linux |
| [MuMuDVB](https://github.com/braice/MuMuDVB) | C | GPL-2.0 | 2026-07-16 | 241 | streaming IPTV desde tarjetas DVB | broadcast DVB | binario Linux |
| [OpenCaster (fork aventuri)](https://github.com/aventuri/opencaster) | C/Python | GPL-2.0 | 2024-05-04 (semi-abandonado) | 74 | generador/manipulador de TS, PSIP/SI, muxing ATSC/DVB | broadcast profesional | scripts Python + binarios C |
| [Comcast/scte35-go](https://github.com/Comcast/scte35-go) | Go | Apache-2.0 | 2026-09-09 | 47 | librería SCTE-35 (encode/decode) — no SCTE-104 | librería, reutilizable directo en Go | módulo Go |
| [m2amedia/scte35dump](https://github.com/m2amedia/scte35dump) | Rust | Apache-2.0 | 2024-08-28 | 34 | volcado de SCTE-35 desde TS/RTP | herramienta CLI | binario |
| [astronautlabs/scte104](https://github.com/astronautlabs/scte104) | TypeScript | MIT | 2024-06-21 | 14 | protocolo SCTE-104 TCP/IP (el que usa Antena787 en F4) | librería | npm |
| [dsame (cuppa-joe)](https://github.com/cuppa-joe/dsame) | Python | GPL-3.0 (ver repo) | activo 2026 | 137 | decodificador SAME/EAS (audio → alerta) | broadcast pequeño/aficionado | script Python |
| [CCExtractor](https://github.com/CCExtractor/ccextractor) | C | GPL-2.0 | 2026-09-06 | 902 | extrae CEA-608/708 de video a archivo de subtítulos | herramienta universal | binario CLI |
| [libcaption (szatmary)](https://github.com/szatmary/libcaption) | C | MIT | 2025-07-29 | 182 | encoder/decoder CEA-608/708 embebible | librería para integrar en un motor | librería C |
| [libebur128 (jiixyj)](https://github.com/jiixyj/libebur128) | C | MIT | 2023-06-25 (estable, sin cambios) | 491 | librería de referencia EBU R128 (loudness) | librería | librería C |
| [ffmpeg-normalize (slhck)](https://github.com/slhck/ffmpeg-normalize) | Python | MIT | 2026-09-02 | 1534 | CLI que envuelve el filtro `loudnorm` de ffmpeg (2 pasadas) | herramienta de ingest | script Python + ffmpeg |

Todas las licencias listadas son compatibles con AGPL-3.0 para consulta,
referencia de diseño o reescritura propia (GPL/LGPL/AGPL/MIT/BSD/Apache). Nota
de compatibilidad real: **vincular/enlazar** código GPL-2.0-only (CCExtractor,
XMLTV, DVBlast, MuMuDVB, OpenCaster, Rivendell) dentro de un binario AGPL-3.0
no es automático — GPL-2.0 "only" y AGPL-3.0 no son compatibles para
enlace directo sin la cláusula "or later"; llamarlos como **proceso aparte**
(como ya hace Antena787 con ffmpeg) evita el problema. LGPL-2.1 (MLT) y
GPL-3.0/GPL-2.0-or-later si aplica sí permiten más margen. Apache-2.0, MIT,
BSD y Unlicense no tienen restricción alguna.

## 2 · Los más cercanos (top 5)

**ffplayout** (Rust, GPL-3.0, 586★, commits activos esta semana) es el
proyecto más parecido en filosofía: un solo binario + ffmpeg, playlist en
JSON editable, pensado para estaciones chicas sin equipo técnico grande, con
salida continua 24/7 y ya soporta salida `udp` a multiplexor y streaming en
vivo. Le falta lo que es el corazón de Antena787: reglas de programación
(episode counters, filler declarativo), traffic de publicidad, portal de
anunciante, EAS y cumplimiento por país. Es la referencia de tiempo citada en
el propio ROADMAP.md de Antena787 ("tomó más de dos años en llegar a algo
estable"), y confirma que el ritmo estimado no es descabellado.

**CasparCG Server + jaskie/PlayoutAutomation** juntos forman el patrón
"motor separado del scheduler" que Antena787 evita a propósito (ADR 0001): CasparCG
es un motor de render de gráficos y video de nivel broadcast, en producción
24/7 desde 2006, pero no decide qué sale al aire — eso lo hace
PlayoutAutomation (o NRCS/rundown externos), usado en canales regionales de
TVP Polonia. Es la arquitectura estándar de la industria (motor + capa de
automatización desacoplada), la que Antena787 decidió no replicar por CGo y
por complejidad operativa para una sola persona.

**nebulabroadcast/nebula** (con su motor `conti`) es el proyecto libre que
más se acerca en ambición al alcance completo de Antena787: media asset
management, traffic-scheduling y playout en un mismo sistema, con
arquitectura de microservicios en Python/TypeScript sobre Postgres y Docker.
Le falta EAS, SCTE-35/104 nativo, y está pensado para equipo técnico
(despliegue multi-servicio), no para el operador solo que Antena787 sirve
como "extremo pequeño". Vale estudiar su modelo de datos de reglas de
programación.

**Rivendell** y **LibreTime/Airtime** cubren el lado de radio con
traffic-scheduling maduro (Rivendell lleva desde 2002, en uso comercial real,
con as-run log y logs de cortes). Antena787 declara explícitamente que radio
(F4b) reutiliza el mismo motor que TV — estos proyectos son la prueba de que
el dominio de radio-automation ya tiene soluciones libres serias, pero
ninguna comparte motor con TV: son sistemas separados, mientras la apuesta
de Antena787 es un solo core para ambos.

**OpenBroadcaster (OBServer + OBPlayer)** es, junto con Antena787, de los
pocos proyectos libres pensados explícitamente para **PEG / televisión y radio
comunitaria de bajo presupuesto** (el mismo "extremo pequeño" del README de
Antena787) y no para broadcast profesional. Trae algo que Antena787 aún no
tiene en ninguna fase publicada con este detalle: **integración EAS con CAP/NAAD/NOAA
ya construida y en producción** (repo `openbroadcaster/mapping`). Es la
referencia obligada para diseñar F2/F4 de EAS/ENDEC en Antena787, aunque su
stack (PHP + Python + LAMP) es justo la complejidad operativa que Antena787
evita con el binario único Go.

## 3 · Piezas reutilizables (librerías/herramientas que Antena787 podría llamar o inspeccionar)

- **TSDuck** (C++, BSD-2-Clause) — el toolkit de referencia para MPEG-TS:
  generación de PSI/SI, inserción SCTE-35, monitoreo de streams. Se llamaría
  como proceso aparte (como ffmpeg), nunca enlazado — evita CGo. Útil para
  F2 (`udp-ts`) y F4/F5 (SCTE-104/inyección).
- **Comcast/scte35-go** (Go, Apache-2.0) — librería Go pura para
  encode/decode de SCTE-35. Compatible de forma directa (mismo lenguaje,
  licencia permisiva, sin CGo). Candidato real para F4/F5.
- **astronautlabs/scte104** (TypeScript, MIT) — no reutilizable en Go
  directamente, pero sirve como referencia de implementación del protocolo
  TCP/IP de SCTE-104 que exige F4.
- **libcaption** (C, MIT) — encoder/decoder CEA-608/708 embebible; si algún
  día Antena787 necesitara generar/leer captions sin invocar ffmpeg, esta es
  la librería de referencia, aunque enlazarla violaría el ADR "no CGo" — se
  usaría como proceso aparte o como referencia de algoritmo para una
  reimplementación en Go puro.
- **CCExtractor** (C, GPL-2.0) — la herramienta CLI estándar para extraer
  608/708; llamarlo como binario externo evitaría el problema de licencia
  GPL-2.0 vs AGPL-3.0 por enlace.
- **ffmpeg-normalize** (Python, MIT) — no se reutiliza el código Python, pero
  documenta exactamente el patrón de dos pasadas con el filtro `loudnorm` de
  ffmpeg que Antena787 ya planea usar para el ingest — sirve de referencia de
  parámetros, no de dependencia.
- **libebur128** (C, MIT) — implementación de referencia del estándar EBU
  R128; útil si en algún punto Antena787 quisiera medir loudness sin invocar
  ffmpeg/ffprobe, aunque de nuevo el ADR "no CGo" empuja a leer el algoritmo
  y no enlazar la librería.
- **iptv-org/epg** (Node, Unlicense — dominio público de facto) — no aporta
  código a un binario Go, pero su catálogo de +100 fuentes y el propio
  formato XMLTV son la referencia de compatibilidad de guía que Antena787 ya
  declaró como salida (XMLTV validado, F1).
- **XMLTV** (Perl, GPL-2.0) — el "spec" de facto del formato, útil solo como
  documentación/DTD de referencia, no como dependencia de código.
- **dsame / dsame3** (Python, GPL) — referencia de implementación de
  decodificación SAME para EAS; el protocolo en sí (NWS SAME) es público, así
  que sirve como documentación del formato más que como código a portar.

## 4 · Huecos que nadie cubre bien (útiles para posicionar Antena787)

- **Un solo motor para TV y radio con reglas declarativas de programación**
  (episode counters, filler, broadcast day) en el mismo core: los proyectos
  de radio (Rivendell, LibreTime) y los de TV (ffplayout, CasparCG,
  PlayoutAutomation, nebula) son mundos separados; ninguno declara
  explícitamente "radio es un canal de TV sin video" como principio de
  diseño.
- **Traffic + advertising + portal de anunciante integrado y libre**: no
  existe un proyecto libre con crawl de clasificados, portal de entrega/cobro
  del anunciante y evidencia de emisión todo junto — lo comercial (Cablecast,
  WideOrbit, TVPlay/moviejaySX) lo cubre, lo libre no. Este es el hueco más
  claro de todo el relevamiento.
- **EAS/ENDEC como integración de primera clase en el flujo de playout con
  cascada de respaldo**: OpenBroadcaster tiene EAS, pero como capa aparte de
  su motor de radio; nadie lo integra con decks de prioridad y watchdog de un
  motor de TV en Go.
- **SCTE-104 con prueba de resistencia (48-72h) documentada como criterio de
  aceptación**: las implementaciones existentes (astronautlabs, TSDuck) son
  librerías/herramientas puntuales, no sistemas con ese nivel de verificación
  operativa.
- **Perfiles de cumplimiento por país seleccionables** (`us-fcc`, `eu-ebu`,
  `isdb-latam`) dentro de un mismo binario: cada proyecto existente asume un
  único mercado (ffplayout es agnóstico pero no modela perfiles; Rivendell es
  EEUU; los europeos —Sofie, TSDuck— asumen DVB).
- **Un binario único sin runtime, multiplataforma (Windows/Linux/ARM), para
  una sola persona operando sin equipo técnico**: LibreTime/nebula exigen
  Docker + varios servicios; CasparCG+PlayoutAutomation exige Windows Server
  + SQL Server; solo ffplayout y OpenBroadcaster se acercan al perfil de
  "estación de una persona", y ninguno de los dos cubre TV y radio con
  traffic de publicidad a la vez.

## 5 · Fuentes

- [ffplayout/ffplayout](https://github.com/ffplayout/ffplayout)
- [jaskie/PlayoutAutomation](https://github.com/jaskie/PlayoutAutomation)
- [CasparCG/server](https://github.com/CasparCG/server) · [CasparCG/client](https://github.com/CasparCG/client)
- [nebulabroadcast/nebula](https://github.com/nebulabroadcast/nebula) · [immstudios/conti](https://github.com/immstudios/conti)
- [Sofie-Automation/sofie-core](https://github.com/Sofie-Automation/sofie-core) · [Sofie-Automation/Sofie-TV-automation](https://github.com/Sofie-Automation/Sofie-TV-automation)
- [libretime/libretime](https://github.com/libretime/libretime)
- [sourcefabric/airtime](https://github.com/sourcefabric/airtime)
- [ElvishArtisan/rivendell](https://github.com/ElvishArtisan/rivendell)
- [openbroadcaster/observer](https://github.com/openbroadcaster/observer) · [openbroadcaster/obplayer](https://github.com/openbroadcaster/obplayer) · [openbroadcaster/mapping](https://github.com/openbroadcaster/mapping)
- [AzuraCast/AzuraCast](https://github.com/AzuraCast/AzuraCast)
- [mltframework/mlt](https://github.com/mltframework/mlt)
- [XMLTV/xmltv](https://github.com/XMLTV/xmltv)
- [iptv-org/epg](https://github.com/iptv-org/epg)
- [tsduck/tsduck](https://github.com/tsduck/tsduck)
- [videolan/dvblast](https://github.com/videolan/dvblast)
- [braice/MuMuDVB](https://github.com/braice/MuMuDVB)
- [aventuri/opencaster](https://github.com/aventuri/opencaster) (fork de Avalpa OpenCaster)
- [Comcast/scte35-go](https://github.com/Comcast/scte35-go) · [m2amedia/scte35dump](https://github.com/m2amedia/scte35dump) · [astronautlabs/scte104](https://github.com/astronautlabs/scte104)
- [cuppa-joe/dsame](https://github.com/cuppa-joe/dsame)
- [CCExtractor/ccextractor](https://github.com/CCExtractor/ccextractor)
- [szatmary/libcaption](https://github.com/szatmary/libcaption)
- [jiixyj/libebur128](https://github.com/jiixyj/libebur128)
- [slhck/ffmpeg-normalize](https://github.com/slhck/ffmpeg-normalize)
- [ebu/awesome-broadcasting](https://github.com/ebu/awesome-broadcasting) (lista curada usada para descubrir proyectos adicionales)
- [Media Realm — 20+ Open Source Broadcast Software Projects on GitHub](https://www.mediarealm.com.au/articles/open-source-broadcast-software-github/)
- Datos de estrellas/licencia/último commit obtenidos vía `gh api repos/<owner>/<repo>` el 2026-09-09.
