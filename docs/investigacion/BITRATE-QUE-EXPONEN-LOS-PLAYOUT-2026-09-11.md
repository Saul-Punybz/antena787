# Qué perillas de bitrate exponen los sistemas de playout que existen (2026-09-11)

## El caso concreto

El cliente de Antena787 emite hoy con VLC así:

```
vcodec=mp2v, vb=4800 kb/s, 720p, acodec=mpga, ab=128 kb/s, 44100 Hz
→ udp{mux=ts} a un multiplexor que REASIGNA los PID y los canales virtuales
```

Antena787 tiene bitrate de video y de mux configurables por salida, y el de audio cableado en 192 kb/s. Esta investigación mira **productos**, no normas ni reguladores (eso lo cubren las otras tres investigaciones en esta misma carpeta). Es investigación web con fuentes citadas; se distingue documentación oficial de foros/comunidad en cada punto.

Metodología: se despacharon cuatro investigaciones paralelas — libres (grupo A: ffplayout, CasparCG, OBS Studio, Nageru; grupo B: MistServer, OpenBroadcaster, LibreTime, Rivendell), comerciales (Cinegy Air, PlayBox Neo, BroadStream OASYS, Imagine Versio, Grass Valley, Dinesat, VirtualPowerVideo, Pebble), y el caso de al lado (Technalogix, Harmonic, Ateme, Thor Broadcast, DekTec).

---

## Las tres preguntas del corazón

### 1. ¿La mayoría de los playout entregan bitrate fijo y dejan el rate shaping al mux, o al revés?

**Depende del tipo de producto, y ahí está el hallazgo central:**

- **Los playout de código abierto que SÍ hacen su propio encoding** (ffplayout, OBS Studio, Nageru, OpenBroadcaster) fijan su propio bitrate de video con un número configurable por el operador (CBR o VBR con techo), y **no delegan el rate shaping en nada aguas abajo** — el stream que sale ya está listo para el destino final. Nageru es la excepción interesante: documenta explícitamente que su salida NO está pensada para servir usuarios finales directamente y recomienda un transcoder/repetidor externo (Cubemap o Kaeru) para esa última milla — un rol parecido, aunque no idéntico, al del mux del cliente de Antena787. Fuente: https://nageru.sesse.net/doc/streaming.html
- **CasparCG es un caso aparte**: no tiene UI de bitrate en absoluto, es un passthrough puro de argumentos de ffmpeg escritos a mano por el operador, sin defaults. Fuente oficial: https://github.com/CasparCG/help/wiki/FFMPEG-Consumer
- **MistServer, por defecto, NO transcodifica**: transmuxea/entrega el stream de origen tal cual, y solo controla bitrate si el operador activa un proceso opcional (MistProcAV). Fuente: https://mistserver.org/mistserver_digital_supply_chain y https://docs.mistserver.org/mistserver/processes/mistprocav/
- **Rivendell (radio) delega el 100% del streaming en vivo hacia el mundo a un encoder externo** (DarkIce/Liquidsoap/BUTT) — Rivendell solo manda metadata (PAD) al servidor Icecast, no toca el bitrate del audio en absoluto. Fuente: https://wiki.rivendellaudio.org/index.php/Streaming_from_Rivendell (vía Wayback Machine, wiki oficial caído al momento de la consulta).
- **En los comerciales**, la separación arquitectónica entre "automation/playlist" y "encoding" es explícita y documentada en varios: Pebble separa "Automation" (Marina — cero mención de bitrate/codec en su datasheet) del "Integrated Channel" (Dolphin/Orca, que sí encoda) — cita textual del datasheet de Dolphin: *"MPEG2 IP outputs utilise software encoders... The pipeline multiplexer outputs a fully compliant DVB stream"* (mux separado del motor de automatización). Fuente: https://www.pebble.tv/wp-content/uploads/2014/06/PBS-Dolphin-Data-Sheet-5.pdf. Grass Valley también separa transcodificación (XRE Transcoder 9) del motor de automatización iTX, aunque iTX 2.8 sumó compresión integrada para casos sin encoder externo (cobertura de industria, no oficial: https://www.svgeurope.org/blog/ibc-2015/grass-valley-adds-new-playout-features-with-itx-2-8/).
- **Cinegy Air y PlayBox Neo, en cambio, integran el control de bitrate dentro del propio motor de playout** (no lo separan en otro producto), con campos de video y audio configurables. Fuentes: https://open.cinegy.com/products/air/26.2/playout/user-manual/playback-device-settings/output/ y http://cdn-docs.av-iq.com/dataSheet/AirBox_Datasheet.pdf
- **El "caso de al lado" (mux/encoders de emisión) confirma el patrón que le importa a Antena787**: cuando existe un mux externo aguas abajo (como el del cliente), ese mux es quien tiene la autoridad final sobre PIDs, PSI/SI, PCR y, en varios casos, sobre el bitrate agregado del multiplex — vía **statmux** (Harmonic ProStream, Ateme Titan Mux) o vía **rate shaping por prioridad/descarte** (DekTec MuxXpert, con su parámetro "RemoveLevel"). Fuentes: https://www.harmonicinc.com/hubfs/datasheet/prostream-x.pdf, https://duochile.cl/wp-content/uploads/2022/11/ateme-datasheet-titan-mux.pdf, https://www.dektec.com/products/applications/MuxXpert/downloads/DTC-700%20MuxXpert%20Manual.pdf

**Respuesta corta:** no hay un patrón único de industria. Los playout que hacen su propio encoding (que es la categoría de Antena787, igual que ffplayout/OBS/Nageru/Cinegy/PlayBox) fijan un bitrate propio, número exacto, configurable por el operador — y ESE approach (bitrate configurable en el playout, PID/mux reasignado aguas abajo) es exactamente lo que ya hace Antena787 con el video. El único hueco real está en el audio (ver pregunta 3).

### 2. ¿Alguno recomienda un número al operador, o todos se limitan a una cajita vacía?

Casi todos son cajita vacía con un default de fábrica sin explicación. Las excepciones que sí **guían**, y que valen la pena copiar:

- **OBS Studio** trae en su blog oficial una tabla de bitrate de video recomendado por resolución (con el preset x264 asumido), ej. 1280×720 → 3000–5000 kbps, 1920×1080 → 5000–8000 kbps. Fuente oficial: https://obsproject.com/blog/streaming-with-x264. La documentación legada de "OBS Classic" (semi-oficial, del fundador) además recomienda calcular el bitrate como 70–80% de la velocidad de subida disponible: https://jp9000.github.io/OBS/settings/encodingsettings.html
- **Nageru** da contexto de industria real en su propia documentación: *"most TV channels use 12-15 Mbit/sec"* para 720p60, frente a su propio default de 4500 kb/s pensado para streaming web — es la única fuente que compara explícitamente un bitrate de broadcast real contra uno de "internet". Fuente: https://nageru.sesse.net/doc/streaming.html
- **LibreTime** es la más explícita en dar una regla con justificación técnica: *"below 128kbps isn't recommended for music"*, y *"96kbps or 64kbps may be acceptable for voice broadcasts"* — la única que diferencia el número según el tipo de contenido (música vs. voz). Fuente oficial: https://libretime.org/docs/admin-manual/stream-configuration/
- **Cinegy Air** es el único comercial con una advertencia tipo "esto no cabe": su manual dice textualmente que el bitrate total configurado *"should be greater than the sum of all bitrates plus some overhead to cover local spikes on complicated scenes"* — es decir, guía al operador a pensar en presupuesto de banda del canal, no solo en un número aislado. Fuente: https://open.cinegy.com/products/air/26.2/playout/user-manual/playback-device-settings/output/
- **Ateme Titan Mux y Harmonic ProStream** (el "caso de al lado") documentan el concepto de **"target average bitrate"** dentro de un pool de statmux — el operador configura un promedio objetivo por canal y el mux reparte el resto dinámicamente según complejidad de imagen. Es la forma más sofisticada de "guiar" que se encontró, pero vive en el mux, no en el playout. Fuentes: https://www.harmonicinc.com/hubfs/datasheet/prostream-x.pdf, https://duochile.cl/wp-content/uploads/2022/11/ateme-datasheet-titan-mux.pdf

**Lo que ninguno hace:** ningún producto investigado — libre o comercial — valida en tiempo real si la suma de bitrates de las salidas configuradas cabe en el ancho de banda físico del enlace o del multiplex, con un aviso bloqueante. Cinegy es el más cerca, pero es texto de manual, no una validación automática en la UI.

**Recomendación para copiar bien:** la combinación que más rinde es OBS (tabla de referencia por resolución) + Cinegy (aviso de presupuesto de banda, "la suma de tus salidas más margen para picos") + LibreTime (diferenciar el número según el tipo de contenido, voz vs. música/video con mucho movimiento). Ninguno de los tres hace las tres cosas a la vez — Antena787 podría ser el primero.

### 3. El bitrate de audio: ¿es una perilla normal o casi nadie la expone?

**Es una perilla normal en el software libre; es más pareja 50/50 en el comercial.**

- **Todos los libres que hacen su propio encoding la exponen como campo independiente**: ffplayout (`audio_bitrate`, default 128 kb/s, libre), OBS Studio (dropdown de valores discretos), Nageru (`--http-audio-bitrate`, ejemplo 128), OpenBroadcaster (dropdown 64–320 kb/s para RTMP, default 160; para Icecast 8–320 kb/s con opción de "0" = sin forzar/VBR), MistServer vía su proceso opcional MistProcAV (`bitrate` de audio, default **192,000 = 192 kb/s exactos**, mismo número que tiene fijo Antena787), LibreTime (hasta 3 streams de salida independientes, cada uno con su propio bitrate de audio). Fuentes respectivas: https://github.com/ffplayout/ffplayout/blob/master/backend/app/src/db/models.rs, https://obsproject.com/forum/threads/what-is-bitrate-and-what-values-should-i-use-for-video-and-audio-bitrate.194530/ (nota: este hilo es foro, no doc oficial — el valor discreto exacto de la lista de OBS Studio actual no se pudo confirmar línea por línea en el código vigente), https://nageru.sesse.net/doc/streaming.html, https://github.com/openbroadcaster/obplayer/blob/main/obplayer/data.py, https://docs.mistserver.org/mistserver/processes/mistprocav/, https://libretime.org/docs/user-manual/settings/
- **CasparCG y Rivendell son las excepciones libres**: CasparCG porque todo es passthrough manual de ffmpeg (no hay campo, el usuario escribe `-b:a` si quiere); Rivendell porque delega el streaming en vivo entero a un encoder externo y su propio dropdown de "Bit Rate" es solo para grabación/ingesta interna (MPEG Layer 2), no para la salida al mundo. Fuentes: https://github.com/CasparCG/help/wiki/FFMPEG-Consumer, https://opsguide.rivendellaudio.org/html/sect.rdadmin.manage_hosts.html
- **En comerciales, está partido**: Cinegy Air (default documentado 384 kb/s, editable) y PlayBox Neo AirBox (rango oficial 64–384 kb/s, sin depender del hardware) sí lo exponen como campo numérico claro. BroadStream OASYS, Imagine Versio (en su datasheet actual R2.1), Grass Valley, Dinesat y Pebble **no publican ningún valor en kb/s de audio en su documentación pública actual** — solo listan codecs soportados (AAC, Dolby D/E, PCM, MP2), sin campo de bitrate documentado. Fuentes: https://open.cinegy.com/products/air/26.2/playout/user-manual/playback-device-settings/output/, http://cdn-docs.av-iq.com/dataSheet/AirBox_Datasheet.pdf, https://broadstream.com/wp-content/uploads/2021/06/OASYS-IP-Datasheet.pdf, https://imaginecommunications.com/content/uploads/2026/05/Versio-Integrated-Playout_R2.1.pdf, https://www.pebble.tv/wp-content/uploads/2014/06/PBS-Marina-Datasheet.pdf
- **VirtualPowerVideo: no hay información pública sobre bitrate de audio ni de video** — ni siquiera un valor en kb/s en sus páginas de producto; es el único de los 17 productos comerciales/libres con documentación pública tan escasa que no permite ninguna afirmación. Fuente (ausencia confirmada): https://www.impactovirtual.com/PowerTV/

**Respuesta corta:** en el software libre que hace su propio encoding, el bitrate de audio configurable es la norma, no la excepción — siete de ocho productos libres investigados lo exponen (o delegan explícitamente a algo que sí lo expone). En el comercial es 50/50, pero los dos jugadores con documentación más rica y madura (Cinegy, PlayBox) sí lo exponen. El dato más señalado: **192 kb/s es exactamente el default de fábrica de MistServer** — no es un número arbitrario ni exagerado — pero en MistServer ese 192 es un default *editable*, mientras que en Antena787 está cableado sin forma de cambiarlo. El cliente de Antena787 usa 128 kb/s hoy con VLC; si Antena787 fuerza 192 fijo, técnicamente no rompe nada del cliente (192 > 128, no hay pérdida), pero sí le quita al operador la posibilidad de igualar su flujo actual o de bajar a 96/64 para voz, que es justo la flexibilidad que LibreTime documenta como buena práctica.

---

## Tabla comparativa — libres

| Producto | Video bitrate configurable | Audio bitrate configurable | ¿Hace su propio rate control o delega? | ¿Guía/recomienda? |
|---|---|---|---|---|
| ffplayout | Sí, número exacto o CRF+maxrate según codec | Sí, campo libre, default 128 kb/s | Propio | No, solo defaults |
| CasparCG | Sí, pero 100% manual vía args de ffmpeg, sin defaults | Sí, igual, sin defaults | Delega todo a ffmpeg/lo que reciba aguas abajo | No |
| OBS Studio | Sí, CBR/VBR/CRF, default 6000 kbps | Sí, dropdown (fuente: foro, no verificado en código vigente) | Propio | Sí, tabla oficial por resolución |
| Nageru | Sí, `--x264-bitrate`, default 4500 kb/s | Sí, `--http-audio-bitrate` | Propio, pero recomienda repetidor externo (Cubemap/Kaeru) para la entrega final | Sí, compara contra bitrates reales de TV |
| MistServer | Solo si se activa proceso opcional (default 8 Mbps "target") | Solo si se activa proceso opcional (default 192 kb/s "target") | Por defecto NO transcodifica, transmuxea tal cual | No |
| OpenBroadcaster | Sí, campo libre kbps (RTMP, default 7500), CBR real | Sí, dropdown 64–320 (RTMP) u 8–320/"0" (Icecast) | Propio | No |
| LibreTime (solo audio) | N/A | Sí, hasta 3 streams independientes; solo baja, no sube bitrate | Vía Liquidsoap, propio | Sí, con justificación técnica música/voz |
| Rivendell (solo audio) | N/A | Solo para grabación/ingesta (MPEG Layer 2); streaming en vivo delegado 100% a encoder externo | Delega para streaming en vivo | No |

## Tabla comparativa — comerciales

| Producto | Video bitrate | Audio bitrate | Automation separada del encoding? | ¿Guía/aviso de presupuesto? |
|---|---|---|---|---|
| Cinegy Air | Sí, VBR/CBR configurable | Sí, dropdown, default 384 kb/s | No, integrado | Sí — único con aviso textual de presupuesto de banda |
| PlayBox Neo (AirBox) | Sí, rangos por códec (hasta 80 Mb/s en H.264 High) | Sí, rango oficial 64–384 kb/s | No, integrado (TitleBox es solo CG) | No |
| BroadStream OASYS | No hay cifra pública, solo codecs listados | No hay cifra pública | No, integrado | No |
| Imagine Versio | Datasheet 2020 sí traía cifras (1–15 Mb/s H.264, etc.); datasheet actual R2.1 ya no las imprime | No hay cifra pública en ninguna versión | Modular por licencia, pero sin producto "Encode" aparte | No |
| Grass Valley (iTX) | No confirmado (manuales con 403); iTX 2.8 sumó compresión integrada | No confirmado | Sí — XRE Transcoder 9 es producto aparte | No |
| Dinesat | Sin cifra en doc oficial de software; cifras (128 MP3/96 AAC) son de planes de hosting que vende aparte | Igual que video | No confirmado con claridad | No |
| VirtualPowerVideo | No hay información pública | No hay información pública | No confirmado | No |
| Pebble | Solo un techo documentado: "hasta 50 Mbps" (Playout in a Box) | No hay cifra en kb/s documentada | Sí, explícito — Automation (Marina) sin mención de bitrate; encoding vive en "Integrated Channel" (Dolphin/Orca) | No |

## El caso de al lado — mux y encoders de emisión

| Producto | Qué es | Statmux o rate shaping documentado | Fuente |
|---|---|---|---|
| Technalogix | Encoder + remultiplexor integrado | No documenta statmux; sí PID remap, restamp, PSI/SI | https://technalogix.com/en-us/pages/digital-tv-amplifiers |
| Harmonic (Electra X2S / ProStream X) | Encoder con statmux integrado / mux puro aparte | Sí, statmux explícito con "target average bitrate" | https://www.harmonicinc.com/hubfs/datasheet/prostream-x.pdf |
| Ateme (Titan Live / Titan Mux) | Encoder / mux puro, productos separados | Sí, statmux con "Dynamic Statmux Pool" y "VBR Reservation" | https://duochile.cl/wp-content/uploads/2022/11/ateme-datasheet-titan-mux.pdf |
| Thor Broadcast | Encoder-modulador todo en uno con remux integrado | No statmux entre canales; VBR/CBR por canal, PID remap, PCR adjusting | https://thorbroadcast.com/upload/files/207/user-manual-thor-broadcast-h-1-4hdmi-qam-ipll-h-1-4sdi-qam-ipll-1-4-hdmi-sdi.pdf |
| DekTec (MuxXpert) | Mux puro, no encoder de video (salvo la DTA-2180) | No statmux clásico; sí rate shaping por prioridad/descarte ("RemoveLevel") + PID/PCR restamping | https://www.dektec.com/products/applications/MuxXpert/downloads/DTC-700%20MuxXpert%20Manual.pdf |

**Conclusión de este bloque:** el patrón de la industria de emisión, cuando hay un mux externo (como el del cliente de Antena787), es que **ese mux es quien tiene la autoridad final sobre PIDs, PSI/SI, PCR y, en varios casos, el bitrate agregado**. El playout/encoder de origen no necesita replicar statmux ni imponer un bitrate rígido que compita con el mux — solo necesita entregar un stream estable, tal como ya hace Antena787 hoy con VLC.

---

## Qué no se encontró (dicho explícitamente, no inventado)

- No hay información pública sobre bitrate de audio o video de **VirtualPowerVideo** — ni en su sitio oficial, ni en foros de broadcast (forum.videohelp.com, groups.io).
- No se pudo confirmar la lista exacta de valores discretos del dropdown de audio en **OBS Studio** directamente en el código fuente vigente (la cifra citada en foros no está verificada línea por línea).
- No se pudo confirmar la lista de valores del dropdown de bitrate de audio de la UI web de **LibreTime** (solo se confirmó el mecanismo y la recomendación textual, no la lista exacta de números del menú).
- **Grass Valley iTX**: los manuales de operador (Master Control, System Admin Guide) devolvieron error 403; no se pudo confirmar con cifras si iTX expone bitrate de audio/video en su UI actual.
- **BroadStream OASYS, Dinesat (software, no plan de hosting) y Pebble**: no hay cifra pública de bitrate de audio en kb/s en su documentación oficial vigente.
- Ningún producto investigado, libre o comercial, documenta una validación automática y bloqueante de "esto no cabe" contra el ancho de banda físico — el aviso más cercano (Cinegy) es texto de manual, no una regla de UI que impida guardar la configuración.

---

## Fuentes por producto (índice rápido)

- ffplayout: https://github.com/ffplayout/ffplayout/blob/master/backend/engine/src/utils/config.rs, https://github.com/ffplayout/ffplayout/wiki/Outputs
- CasparCG: https://github.com/CasparCG/help/wiki/FFMPEG-Consumer, https://github.com/CasparCG/Server/issues/142
- OBS Studio: https://github.com/obsproject/obs-studio/blob/master/plugins/obs-x264/obs-x264.c, https://obsproject.com/blog/streaming-with-x264, https://jp9000.github.io/OBS/settings/encodingsettings.html (semi-oficial/legado)
- Nageru: https://nageru.sesse.net/doc/streaming.html
- MistServer: https://docs.mistserver.org/mistserver/processes/mistprocav/, https://mistserver.org/mistserver_digital_supply_chain, https://docs.mistserver.org/howto/encoding/mkv-gstreamer/
- OpenBroadcaster: https://github.com/openbroadcaster/obplayer (streamer/rtmp.py, streamer/icecast.py, data.py, httpadmin/http/index.html)
- LibreTime: https://libretime.org/docs/user-manual/settings/, https://libretime.org/docs/admin-manual/stream-configuration/, https://libretime.org/docs/admin-manual/configuration/
- Rivendell: https://opsguide.rivendellaudio.org/html/sect.rdadmin.manage_hosts.html, https://wiki.rivendellaudio.org/index.php/Streaming_from_Rivendell (vía Wayback Machine)
- Cinegy Air: https://open.cinegy.com/products/air/26.2/playout/user-manual/playback-device-settings/output/, https://open.cinegy.com/products/air/24.11/encode/user-manual/overview/
- PlayBox Neo: http://cdn-docs.av-iq.com/dataSheet/AirBox_Datasheet.pdf, https://playboxtechnology.com/airbox-neo/, https://playboxneo.com/sites/default/files/2018-11/TitleBoxNeo.pdf
- BroadStream OASYS: https://broadstream.com/wp-content/uploads/2021/06/OASYS-IP-Datasheet.pdf, https://broadstream.com/products/oasys/
- Imagine Versio: https://www.videolink.ch/resources/public/liveedit/media/1600414324_5f6462741872f.pdf (2020), https://imaginecommunications.com/content/uploads/2026/05/Versio-Integrated-Playout_R2.1.pdf (actual)
- Grass Valley: https://www.grassvalley.com/products/software/xre-transcoder-9/, https://www.svgeurope.org/blog/ibc-2015/grass-valley-adds-new-playout-features-with-itx-2-8/ (periodismo de industria, no oficial)
- Dinesat: https://www.dinesat.com/radio/en/, https://www.dinesat.com/tv/en/, https://foro.dinesat.com/thread.aspx?id=368 (foro, no oficial)
- VirtualPowerVideo: https://www.impactovirtual.com/PowerTV/, https://impactovirtual.com/VirtualPowerVideo/
- Pebble: https://www.pebble.tv/wp-content/uploads/2014/06/PBS-Marina-Datasheet.pdf, https://www.pebble.tv/wp-content/uploads/2014/06/PBS-Dolphin-Data-Sheet-5.pdf, https://www.pebble.tv/solutions/playout-in-a-box/
- Technalogix: https://technalogix.com/en-us/pages/digital-tv-amplifiers, https://www.manualslib.com/manual/1357407/Technalogix-Tp1000.html (copia de tercero)
- Harmonic: https://www.harmonicinc.com/hubfs/datasheet/electra-x2s.pdf, https://www.harmonicinc.com/hubfs/datasheet/prostream-x.pdf
- Ateme: https://duochile.cl/wp-content/uploads/2022/11/ateme-datasheet-titan-mux.pdf (mirror de distribuidor), https://www.scenarist.com/download/TITAN%20Specs%202017.pdf
- Thor Broadcast: https://thorbroadcast.com/upload/files/207/user-manual-thor-broadcast-h-1-4hdmi-qam-ipll-h-1-4sdi-qam-ipll-1-4-hdmi-sdi.pdf
- DekTec: https://www.dektec.com/products/applications/MuxXpert/downloads/DTC-700%20MuxXpert%20Manual.pdf
