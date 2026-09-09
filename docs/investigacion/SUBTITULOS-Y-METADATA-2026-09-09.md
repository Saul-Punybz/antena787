# Subtítulos, audio secundario y metadata de contenido

Fecha: 9 de septiembre de 2026.

Verifica contra la ley y contra los sistemas reales dos partes del PRD escritas por diseño, no por investigación publicada: subtítulos/accesibilidad (§9 paso 1, §12) y metadata de contenido (§10, §13). No repite `COMPETENCIA-COMERCIAL-2026-09-09.md` ni `PROYECTOS-SIMILARES-2026-09-09.md`. Cada dato lleva fuente y fecha; donde no se encontró nada, se dice.

---

## Parte 1 · Subtítulos y accesibilidad

### 1 · La FCC (47 CFR 79.1)

**Los cuatro estándares de calidad** (orden de 2014, sin umbral numérico — la FCC lo consideró y prefirió un estándar descriptivo): precisión (palabras en orden, sin parafrasear, con quién habla y qué sonidos hay), sincronía (aparecen y terminan con el sonido), integridad (de principio a fin) y colocación (no tapan caras ni texto). ([dwt.com/insights/2014/02](https://www.dwt.com/insights/2014/02/fcc-adopts-closed-caption-quality-standards), 9 sept 2026)

**Exenciones autoaplicables (79.1(d)), sin pedir permiso.** La más relevante: **§79.1(d)(12)** — canal con ingresos brutos anuales de menos de **$3,000,000** el año anterior no tiene que gastar en subtitular. También: red nueva, sus primeros 4 años (§(d)(9)); programación local sin valor de repetición, no noticias (§(d)(8)); 2 a.m.-6 a.m. (§(d)(5)). Aparte existe la carga económica excesiva (§(f)), que sí requiere petición formal con comentario público. El PRD ya modela los tres estados (obligada/exenta/no sé) pero no cita el número ($3M) ni que sea autoaplicable — vale ponerlo en el texto de ayuda. ([law.cornell.edu/cfr/text/47/79.1](https://www.law.cornell.edu/cfr/text/47/79.1), 9 sept 2026)

**Español**: misma obligación que inglés, 100% de lo nuevo por trimestre, 75% de lo anterior a la regla — no hay vara distinta. ([fcc.gov/general/closed-captioning-video-programming-television](https://www.fcc.gov/general/closed-captioning-video-programming-television), 9 sept 2026)

**Video en línea (CVAA)**: un clip que salió al aire con subtítulos los necesita también en línea (hasta 12h de plazo si es en vivo, 8h si es near-live); no aplica a nada que nunca salió en TV. ([go.3playmedia.com/wp-fcc](https://go.3playmedia.com/wp-fcc), 9 sept 2026)

**Video description / DVS (79.3)**: aplica a afiliadas de las 4 grandes cadenas en los 90 mercados (DMA) más grandes, 50h/trimestre. **No se encontró que aplique a LPTV o Class A** — el criterio es afiliación de red + tamaño de mercado, no clase de licencia; queda como pregunta abierta, no hecho verificado. ([docs.fcc.gov/DOC-363489A1](https://docs.fcc.gov/public/attachments/DOC-363489A1.pdf), 9 sept 2026)

### 2 · CEA-608 contra 708, en la práctica

608 es línea 21 analógica heredada; 708 es nativo de ATSC digital, con fuente/color. En H.264 ambos viajan en SEI (`a53`, ATSC A/53 Part 4); en MPEG-2, en user data del picture header. La práctica de la industria es "upconvertir" 608 dentro de un 708 que solo lo repite, porque el receptor ATSC espera 708 presente. ([3playmedia.com/blog/difference-cea-608-cea-708-captions](https://www.3playmedia.com/blog/difference-cea-608-cea-708-captions/), 9 sept 2026)

**Quién lo pasa de verdad:** **CasparCG no pasa 608/708 embebidos** — limitación abierta desde 2013, con hilos pidiendo desarrollador pago para resolverla. ([github.com/CasparCG/server/issues/1400](https://github.com/CasparCG/server/issues/1400), 9 sept 2026) **Cinegy Air** es el único de los grandes con 608/708 documentado a fondo (ver `COMPETENCIA-COMERCIAL`). **ffplayout**: sin evidencia de soporte 608/708 en ningún issue ni mención. **Cablecast y TelVue** no pasan 608 embebido — generan subtítulos con ASR aparte (§4).

**Sidecars, alcance real:**

| Formato | Guarda | Uso |
|---|---|---|
| `.scc` (Scenarist) | Solo CEA-608 | Lo que Antena787 ya lee |
| `.mcc` (MacCaption) | 608 **y** 708 nativos, OP-47 | Propietario Telestream; único con 708 real |
| `.srt` / `.vtt` | Texto + tiempos | Lo que Antena787 ya convierte |
| `.ttml` / `.dfxp` | XML con estilo | Streaming |
| `.stl` (EBU) | Subtítulos abiertos | DVB, no ATSC |
| `.cap` (Cheetah) | Edición propietaria | Casas de subtitulado |

([closedcaptioncreator.com/.../scc-file-format-explained](https://www.closedcaptioncreator.com/blog/articles/scc-file-format-explained.html) · [aberdeen.io/abercap/deliverables](https://aberdeen.io/abercap/deliverables/), 9 sept 2026). `.scc` nunca resuelve 708: si algún día hace falta 708 nativo, el sidecar es `.mcc`, no `.scc` — hoy `SidecarExtensions` en `captions.go` no lo contempla; no urge en F1/F2.

### 3 · ffmpeg con 608/708: qué sabe y qué no

`-a53cc 1/0` activa/apaga el paso de datos A/53 al codificar con `libx264`/`libx265` — exactamente lo que ya usa `normalize.go`. `ccaption_dec` decodifica 608 embebido a texto/ASS. El formato `.scc` tiene demuxer/muxer, pero **solo 608**, descartando cualquier extensión 708. **ffmpeg no tiene encoder 708 nativo**: decodifica 608, no genera 708 de cero. `libzvbi` decodifica teletexto DVB — sirve para `eu-ebu`, no `us-fcc`. ([ffmpeg.org/ffmpeg-formats.html](https://ffmpeg.org/ffmpeg-formats.html), 9 sept 2026)

**`ffprobe` ≥ 9 ya no reporta si un stream trae closed captions** — la bandera `FF_CODEC_PROPERTY_CLOSED_CAPTIONS` fue retirada sin reemplazo documentado. ([github.com/ptr727/PlexCleaner/issues/497](https://github.com/ptr727/PlexCleaner/issues/497), 9 sept 2026) Ya está anotado en `ACEPTACION.md` (F1-04, diferido a F2); la investigación confirma que no hay forma confiable de *detectar* 608/708 con ffmpeg solo — un paso de F2 necesitaría `ccextractor` o `libcaption` como verificador aparte (ya en `PROYECTOS-SIMILARES`).

### 4 · Subtitulado automático (ASR)

**Cablecast + ENCO enCaption**: transcribe casi en tiempo real, un clic desde la ficha de grabación, genera sidecar emparejado. Precio especial de partner, **no publicado**. ([enco.com/blog/tightrope-and-enco-forge-partnership](https://www.enco.com/blog/tightrope-and-enco-forge-partnership-to-bring-automated-closed-captioning-to-cablecast-community-media-workflows), 9 sept 2026) **TelVue SmartCaption**: reglas por serie/fuente, precio por uso, **sin monto público**. ([telvue.com/smartcaption](https://telvue.com/smartcaption/), 9 sept 2026)

**whisper.cpp / faster-whisper**: locales, sin costo recurrente. `small` (2GB RAM) ~3.4% WER inglés, `medium` (5GB) ~2.9%, `large` ~2.5%; latencia 0.5-2s con VAD — dato del proveedor de la comparación, no benchmark independiente. ([digitalapplied.com/.../local-speech-to-text-whisper-self-hosted-transcription-2026](https://www.digitalapplied.com/blog/local-speech-to-text-whisper-self-hosted-transcription-2026), 9 sept 2026)

**¿Cuenta el ASR como cumplimiento de 79.1?** Zona gris: la FCC no puso umbral numérico (estándar descriptivo), pero hay una petición pidiéndole que aclare si el ASR sin revisión humana cumple, o exija certificación detallada. **No es un cumplimiento garantizado.** ([vitac.com/petition-asks-fcc-for-objective-metrics-on-asr-captioning-quality](https://vitac.com/petition-asks-fcc-for-objective-metrics-on-asr-captioning-quality/), 9 sept 2026)

### 5 · SAP y DVS

Cada pista de audio en el PMT lleva un descriptor AC-3 con **`bsmod`** (3 bits: completo, música/efectos, comentario, doblaje, video description, hearing impaired, emergencia, voz superpuesta) más, típicamente, un `ISO_639_language_descriptor` aparte, aunque sea opcional. ([atsc.org/.../TG1-1013r1](https://www.atsc.org/wp-content/uploads/2020/03/TG1-1013r1-Technology-Group-Report-on-ATSC-Audio-Language-Signaling.pdf), 9 sept 2026) DVS viaja como pista secundaria completa o narración sola que el receptor mezcla. ([wikipedia.org/wiki/Descriptive_Video_Service](https://en.wikipedia.org/wiki/Descriptive_Video_Service), 9 sept 2026)

**Al mux UDP-TS de Antena787** le faltarían tres cosas: segunda pista de audio codificada, descriptor AC-3 con `bsmod` correcto en el PMT, y el `ISO_639_language_descriptor`. Ffmpeg puede producir la segunda pista; el descriptor del PMT depende de si el multiplexor (TP1000) lo deja configurar — **no verificado con el fabricante**.

### 6 · Radio: RDS/RBDS y HD Radio PAD

**No se encontró que Rivendell, RadioBOSS o Zetta generen RDS directamente.** Lo que existe es software puente (MetaRadio, TREplus) que lee el "ahora sonando" (UDP de texto en Rivendell 3, puerto 34289) y lo reenvía a un encoder RDS (UECP) o HD Radio (NRSC-5) — **la automatización habla con un puente, no con el encoder**. ([mediarealm.com.au/metaradio](https://www.mediarealm.com.au/metaradio/), 9 sept 2026) HD Radio PAD exige como mínimo Artista y Título. ([hdradio.com/.../program-services-data-psd](https://hdradio.com/broadcasters/engineering-support/program-services-data-psd/), 9 sept 2026)

Antena787 ya tiene el patrón (driver por lo que responda, como telemetría): un driver `rds-udp`/`rds-serial` sería poco código nuevo sobre la ficha `Card` existente. Pero hoy no hay caso de uso real — RadioOnce Live! en CAtv corre en Océano Radio, no en Antena787 (§25 PRD). Vale solo si algún piloto de radio lo pide.

---

## Parte 2 · Metadata de contenido

### 7 · House ID, ISCI y Ad-ID

**ISCI**: 8 caracteres (4 letras + 4 números), retirado formalmente en octubre 2007, reemplazado por Ad-ID desde 2003 — pero sigue en uso en muchas agencias en vez del house ID del emisor. ([wikipedia.org/wiki/Industry_Standard_Coding_Identification](https://en.wikipedia.org/wiki/Industry_Standard_Coding_Identification), 9 sept 2026) **Ad-ID**: 11 caracteres (12 si es HD, con "H" al final); los primeros 4 son el prefijo del anunciante. Lo asigna **Ad-ID LLC** (empresa conjunta de ANA y 4A's) vía registro web; el anunciante/agencia pide el prefijo y genera sus propios códigos. ([support.ad-id.org/.../components-of-an-ad-id-code](https://support.ad-id.org/support/solutions/articles/43000540378-components-of-an-ad-id-code-including-custom-code-format-option), 9 sept 2026)

**House ID** es distinto: identificador *interno* de la estación, no el que trae el spot de la agencia. **No se encontró documentación pública** de cómo WideOrbit o Marketron mapean ISCI/Ad-ID a un House ID interno (coincidencia exacta vs. tabla editable) — los manuales públicos son notas de venta, sin esquema de datos. Lo consistente en la industria es que **el traffic log referencia el House ID, no el nombre del archivo**, y el emparejo archivo↔ID pasa una sola vez, en la entrada. El PRD hoy no tiene un campo `house_id` separado del nombre/ficha (F1-72) — vale considerarlo si algún día se importa un traffic log externo. ([thearf-org.../TX_AIP.11.pdf](https://thearf-org-unified-admin.s3.amazonaws.com/CIMM/Documents/TX_AIP.11.pdf), 9 sept 2026)

### 8 · BXF, AS-11/MXF: estándares que las estaciones chicas no usan

**BXF (SMPTE ST 2021)**: XML para horarios/as-run entre traffic y playout; 3.0 agrega ingestión de instrucciones de traffic y loudness/AFD. ([tvtechnology.com/opinions/bxf-explained](https://www.tvtechnology.com/opinions/bxf-explained), 9 sept 2026) **AS-11 DPP**: MXF OP1A restringido ("air-ready master") con metadata EBU Core, nacido en el Reino Unido, con variante norteamericana AS-11 X9. ([thedpp.com/as-11](https://www.thedpp.com/as-11), 9 sept 2026)

**Ninguno aparece en Dinesat, PowerTV, ni en ningún proyecto de `PROYECTOS-SIMILARES`** — son estándares de intercambio entre sistemas grandes, no de una estación de una persona. Confirma que apoyarse en `.nfo` de Kodi y en el nombre del archivo, no en BXF/MXF, es correcto para el tamaño de usuario de Antena787.

### 9 · Fichas y guías: precios y licencia

| Proveedor | Licencia / costo | Atribución |
|---|---|---|
| TVmaze | Gratis, CC BY-SA 4.0 | Enlace de vuelta; ShareAlike |
| TMDB | Gratis no comercial con atribución; comercial exige acuerdo aparte, pago negociado | Logo + "not endorsed or certified by TMDb" |
| TheTVDB | Gratis si <$50K/año de facturación; $1,000/año $50K-$250K; $10,000/año $250K-$1M; a medida sobre $1M | Enlace visible (o about/readme en CLI) |
| Gracenote | Licenciamiento cerrado, comercial, sin tarifa pública | — |

([tvmaze.com/api](https://www.tvmaze.com/api) · [themoviedb.org/api-terms-of-use](https://www.themoviedb.org/api-terms-of-use) · [wikipedia.org/wiki/TheTVDB](https://en.wikipedia.org/wiki/TheTVDB), 9 sept 2026). Antena787 es software libre que un tercero opera con fines comerciales (vende anuncios): si el driver TMDB se activa ahí, es el **operador** quien decide si necesita el acuerdo comercial — TVmaze (CC BY-SA, sin distinción comercial) sigue siendo el driver de menor riesgo legal por defecto.

**Lo que de verdad usan las estaciones chicas en EE. UU. para la guía: Schedules Direct**, no TVmaze/TMDB. Sin fines de lucro, **$35/año**, le compra a Gracenote licencia para redistribuir datos EE. UU./Canadá — es la fuente real detrás de casi todo software libre de guía (Kodi, Plex). ([schedulesdirect.org/faq](https://www.schedulesdirect.org/faq), 9 sept 2026) El PRD nunca lo menciona; si Antena787 quisiera *bajar* una guía de terceros en vez de solo generar la propia, esta sería la ruta real.

**EIDR**: resolver un ID es gratis; membresía abierta para registrar obras propias, pero en la práctica es de estudios/distribuidores grandes — sin evidencia de uso por estaciones locales chicas, y no resuelve nada que el nombre de archivo + TVmaze no resuelvan ya. No hace falta. ([eidr.org](https://www.eidr.org/), 9 sept 2026)

### 10 · XMLTV, PSIP y la cadena de CAtv

PSIP (ATSC A/65): MGT, VCT, STT, EIT (0-3 obligatorias, hasta 8 en algunos acuerdos de cable), ETT, más RRT para clasificación. **Class A cumple A/65C completo, incluido el Anexo B (canal virtual); un LPTV regular no** — tiene la *opción* de usar PSIP, no la obligación plena. El perfil `us-fcc` hoy trata "Class A o LPTV" como una pregunta de clase con funciones por ítem (§12), pero para PSIP la obligación sí depende de cuál — vale anotarlo, aunque hoy el multiplexor lo resuelva. ([federalregister.gov/.../2023-09843](https://www.federalregister.gov/documents/2023/05/12/2023-09843/establishing-rules-for-digital-low-power-television-and-television-translator-stations), 9 sept 2026)

**¿Quién genera PSIP en CAtv?** No se encontró que el **Technalogix TP1000** sea, en sí, generador PSIP — su material lo describe como "media gateway" (recibe/decodifica/codifica/transcodifica/scrambling/modula), sin mencionar PSIP. Sí existen, por separado en el catálogo de Technalogix, amplificadores/transmisores que anuncian "PSIP/PSI descriptor generator/injector" con VCT. **Es posible que el PSIP lo genere el transmisor/excitador, no el TP1000** — no verificado, pregunta concreta para el ingeniero (§C). ([manualslib.com/manual/1357407/Technalogix-Tp1000](https://www.manualslib.com/manual/1357407/Technalogix-Tp1000.html), 9 sept 2026)

Si Antena787 algún día necesita generar PSIP él mismo: **TSDuck** (BSD-2-Clause, ya en `PROYECTOS-SIMILARES`) lee/escribe MGT/VCT/EIT/ETT desde XML; existe además un proyecto independiente, **caritechsolutions/psip_generator_client** (Linux+PHP+MySQL+TSDuck, sondea una base de EPG y genera esas tablas por UDP), que confirma el patrón sin ser evidencia de que CAtv lo use. Añadir TSDuck sería un **segundo binario externo**, contra "solo ffmpeg" (ADR 0001) — a sopesar con cuidado si hace falta. ([github.com/caritechsolutions/psip_generator_client](https://github.com/caritechsolutions/psip_generator_client), 9 sept 2026)

**XMLTV contra PSIP, en una frase:** XMLTV es el intercambio que ya sirve Antena787 en `/guia.xml`; PSIP es lo que va dentro del transport stream y lee el receptor ATSC. Hoy la segunda depende de equipo aguas abajo.

### 11 · Clasificación (V-Chip) y programación infantil (E/I)

El **Content Advisory Descriptor** (CEA-766) lleva la clasificación V-Chip en el EIT (y opcionalmente el PMT); la **RRT** define el sistema (en EE. UU., TV Parental Guidelines + V/S/L/D/FV). Encaja con el campo `clasificacion_contenido` que el PRD ya tiene (§10) — falta mapearlo al descriptor si Antena787 genera PSIP algún día. ([tvtechnology.com/miscellaneous/psip-vchip-and-other-acronyms](https://www.tvtechnology.com/miscellaneous/psip-vchip-and-other-acronyms), 9 sept 2026)

**E/I aplica a Class A, no a LPTV simple.** Desde 1997, potencia completa y Class A emiten al menos 3h/semana (o, desde 2019, 156h/año con ≥26h/trimestre regulares) de programación "core" — educativa/informativa para 16 años o menos, ≥30 min, entre 6 a.m. y 10 p.m. Se reporta cada año por **FCC Form 2100, Schedule H**. **CAtv es Class A, así que esto aplica de verdad** — y hoy no aparece en el PRD ni en §12 ni en la ficha. No hay campo para marcar un programa "core" E/I ni conteo de horas por trimestre: es el hallazgo más concreto de este documento, una obligación real del despliegue de referencia que el software no sabe ni preguntar. ([lermansenter.com/.../kidvid-rules](https://www.lermansenter.com/fcc-makes-significant-changes-to-its-childrens-television-kidvid-rules/), 9 sept 2026)

### 12 · Idioma de la programación

No se encontró un campo de "idioma del programa" más allá del idioma del audio (declarado por `ISO_639_language_descriptor`, §5) y la obligación de subtítulos por idioma (§1). El audio principal ya lo infiere la ficha; no hace falta un campo nuevo salvo que el operador quiera declarar explícitamente "programación en español" para el reporte de cuotas de subtitulado.

---

## A · Lo que se nos quedó

| Tema | Lo que hacen los demás / la ley | Antena787 hoy | Qué falta | Fase |
|---|---|---|---|---|
| Exención de subtítulos por ingresos | Autoaplicable, <$3M/año (79.1(d)(12)) | Ajuste de 3 estados, sin el número | Escribir el umbral en el texto de ayuda | F1 (texto) |
| CEA-708 nativo | ATSC exige 708 presente; industria "upconvierte" | `-a53cc` copia lo que venga | ffmpeg no ofrece encoder 708; documentar el límite | F2 (doc), sin ruta libre |
| Sidecar `.mcc` (608+708) | Formato estándar con 708 real | Solo `.scc`/`.srt`/`.vtt` | Añadir `.mcc` si hace falta 708 real | F2/F4 |
| SAP/DVS en el mux | `bsmod` + idioma en el PMT | Reservado, sin diseño de mux | Confirmar si el TP1000 deja configurar el descriptor AC-3 | F2 |
| RDS/HD Radio PAD | Automatización habla con un puente, no el encoder | No existe | Driver `rds-udp`, solo si hay caso de uso | F4b |
| House ID separado del filename | Traffic log referencia House ID, no filename | Ficha usa el nombre (F1-72) | Evaluar campo `house_id` si se importa traffic externo | F4 (si aplica) |
| BXF / AS-11 / MXF | Intercambio entre sistemas grandes | No usados, ni por la competencia chica | Nada — confirma que no hace falta | no |
| Riesgo de licencia comercial (TMDB) | Gratis no comercial; comercial exige acuerdo | Driver opcional, sin aviso | Documentar el riesgo en `COMPLIANCE.md` | F1 (doc) |
| PSIP: quién lo genera en CAtv | El multiplexor o el transmisor, según fabricante | Asumido "el multiplexor", sin verificar | Confirmar con el ingeniero | Pregunta |
| PSIP obligatorio: Class A sí, LPTV no | A/65C Anexo B aplica a Class A/potencia completa | Perfil `us-fcc` no distingue | Anotar la distinción | F1 (doc) |
| Content Advisory Descriptor / RRT | Va en el EIT (CEA-766) | Campo `clasificacion_contenido` sin mapeo a PSIP | Mapear si se genera PSIP propio | F5 (si aplica) |
| **Programación infantil E/I** | Class A: 156h/año, FCC Form 2100 Schedule H | **No existe en absoluto** | Marcar "core" E/I; contador por trimestre; export del reporte | F1 (marcar) / F4 (reporte) |
| ASR como cumplimiento de 79.1 | Zona gris, sin umbral numérico | No aplica (no hay ASR) | Si se ofrece ASR, no venderlo como "cumple 79.1" sin más | F2+ (si se ofrece) |

---

## B · Criterios verificables propuestos

- **F1-XX** [DOC] — Dado el texto de ayuda del ajuste de subtítulos en `us-fcc` · Cuando el operador lo abre · Entonces dice que un canal con ingresos brutos anuales de menos de $3,000,000 está exento de gastar en subtitular sin pedir nada a la FCC (79.1(d)(12)). — **F1**

- **F2-XX** [AUTO] — Dado un archivo con `.mcc` al lado (608+708 nativos) · Cuando se procesa · Entonces se reconoce como sidecar válido y, si trae 708, se preserva sin degradar a 608 solo. — **F2**

- **F2-XX** [AUTO] — Dado un archivo con audio de video description (DVS) marcado en la ficha · Cuando se normaliza · Entonces se mapea a una segunda pista con `bsmod` de video description si el formato de salida lo permite; si no, se anota en el reporte. — **F2**

- **F2-XX** [AUTO] — Dado el mux UDP-TS con más de una pista de audio · Cuando se arma el PMT · Entonces cada pista lleva su `ISO_639_language_descriptor` correcto. — **F2**

- **F1-XX** [DOC] — Dado el perfil `us-fcc` con clase "Class A" · Cuando se activan sus funciones · Entonces se documenta la obligación de programación infantil (E/I) del Children's Television Act, distinta de LPTV simple. — **F1**

- **F4-XX** [AUTO] — Dado un programa marcado "core" E/I · Cuando se arma el reporte trimestral · Entonces suma las horas emitidas del trimestre y avisa si el proyectado del año cae bajo 156 horas. — **F4**

- **F4-XX** [AUTO] — Dado un programa marcado "core" E/I · Cuando se resuelve el aire · Entonces solo cuenta si dura ≥30 min y sale entre 6 a.m. y 10 p.m.; si no cumple, se programa igual pero no suma al conteo E/I. — **F4**

- **F1-XX** [DOC] — Dado el driver TMDB activo en una instalación que vende publicidad · Cuando se configura · Entonces avisa que TMDB exige acuerdo comercial aparte para ese uso, y que TVmaze no tiene esa restricción. — **F1**

- **F2-XX** [AUTO] — Dado un video con subtítulos generados por ASR sin revisión humana · Cuando el reporte de emisión los cuenta · Entonces los marca distinto de un subtítulo editado o 608/708 original. — **F2** (si se ofrece ASR)

- **F5-XX** [AUTO] — Dado un perfil que genera PSIP propio · Cuando se arma la salida · Entonces el Content Advisory Descriptor del EIT refleja `clasificacion_contenido`, usando la RRT de TV Parental Guidelines. — **F5**

---

## C · Preguntas para Rolando / el ingeniero de CAtv

1. **¿El TP1000 genera PSIP (MGT/VCT/EIT/ETT/RRT), o lo hace el excitador RVR u otro equipo?** Su material público no lo menciona; los generadores PSIP de Technalogix documentados están en la línea de transmisores, no en el TP1000. Si nada lo genera hoy, ¿CAtv sale sin PSIP completo?

2. **¿CAtv emite hoy con subtítulos?** Si sí, ¿de dónde vienen — embebidos en los archivos, o añadidos a mano? Si no, ¿es por la exención de ingresos (<$3M/año) o porque no se ha hecho?

3. **¿Quién lleva la cuenta de las horas de programación infantil (E/I) de CAtv, siendo Class A?** Si nadie, es un hueco real, no solo teórico.

4. **Modelo exacto del Sage ENDEC (1822 o 3644)** — ya pendiente en el PRD (§25); también determina el protocolo serial disponible si algún día se inyecta un aviso CAP/EAS en el crawl.

5. **¿El TP1000 deja configurar el `bsmod` de una segunda pista de audio (SAP/DVS)?** Si Antena787 manda dos pistas en el UDP-TS de entrada, ¿el TP1000 las respeta y arma el PMT correspondiente?

6. **¿CAtv está exenta de video description (79.3)?** Todo apunta a que sí (no es afiliada de red en un top-90 DMA) — vale confirmarlo, no asumirlo.

7. **¿Qué usa hoy CAtv, si algo, para la guía que ve el televidente en su receptor ATSC** — no el `/guia.xml` que serviría Antena787? Si no hay nada, es una ganancia inmediata de F1.
