# Entradas de señal por URL — cómo lo resuelve la gente que ya lo resolvió

**Fecha:** 11 de septiembre de 2026
**Pregunta que origina esto:** vamos a construir la entrada de señal por URL en Antena787 (el canal toma un stream remoto como fuente, no solo un archivo local). El cliente —una Class A en Puerto Rico— hoy emite con VLC leyendo HLS de MistServer local (`http://localhost:8080/hls/for_tv/index.m3u8`) y de un proveedor externo (`video2.getstreamhosting.com:19360/...`). Antes de diseñar la pantalla y el modelo de datos, se investigó cómo otros productos guardan, prueban y reconectan fuentes remotas.

**Método:** documentación oficial y repositorios de cada producto, vía búsqueda web y lectura directa de las páginas citadas. Cuando la documentación no dice algo, se anota explícitamente — no se rellenó con suposiciones.

---

## 1. VLC — VLM, `.xspf`, `.m3u`

### Qué es VLM y qué guarda un `.vlm`

VLM (VideoLAN Manager) es, en palabras de la propia wiki, "un pequeño gestor multimedia diseñado para controlar múltiples streams con una sola instancia de VLC". Un archivo `.vlm` es una lista de líneas de comando — una línea, un comando — y se carga al arrancar VLC con `--vlm-conf <archivo.vlm>`. Fuente: [Documentation:Streaming HowTo/VLM — VideoLAN Wiki](https://wiki.videolan.org/Documentation:Streaming_HowTo/VLM/).

Un "medio" (`media`) en VLM se compone de una lista de **inputs**, un **output** y opciones. Hay dos tipos: `vod` (bajo demanda) y `broadcast` (como un canal de TV, que el administrador arranca/para/pausa). Ejemplo textual de la wiki:

```
new channel1 broadcast enabled
setup channel1 input http://host.mydomain/movie.mpeg
setup channel1 output #rtp{mux=ts,dst=239.255.1.1,port=5004,sdp=sap://,name="Channel 1"}
```

El comando `save` guarda toda la configuración de medios en un archivo `.vlm` para reusarla después. Fuente: [doc/vlm.txt en el repo de VLC](https://github.com/videolan/vlc/blob/master/doc/vlm.txt), [VLM - Multiple streaming and Video on demand](https://www.videolan.org/doc/streaming-howto/src/en/vlm-vod.xml).

**Entonces sí, un `.vlm` funciona como preset de entradas reutilizable** — cada `media` tiene nombre (`channel1`) y se puede tener varios definidos en el mismo archivo, arrancados o detenidos por separado.

### Credenciales en la URL

La documentación oficial de VLM **no cubre autenticación** de ningún tipo (ni básica HTTP ni usuario/clave). Lo que sí está bien documentado y es práctica extendida de la comunidad: VLC no tiene caja de login, así que todo va metido en la URL — `http://usuario:clave@host/...` o como query string (`?username=USER&password=PASS`). Cita textual de una guía consultada por usuarios de IPTV: "VLC has no login boxes, so you fold your username and password into one get.php link, then paste that into Open Network Stream". Fuente (guía de terceros, no oficial): [IPTV M3U Username Password in VLC](https://iptv-subs.com/iptv-m3u-username-password/). No se encontró ninguna fuente, oficial o de foro, que documente cifrado de credenciales en VLC — van en texto plano dentro de la URL o el archivo `.vlm`/`.m3u`.

### `.xspf` y `.m3u`

Son formatos de lista de reproducción (playlist), no de gestión de "fuentes" con metadata propia: guardan una lista de URLs/rutas con poco más que un título. No tienen campo de credenciales separado del propio string de la URL — mismo patrón que arriba.

---

## 2. MistServer — lo que corre hoy en la máquina del cliente

### Modelo de datos de un stream

Según la documentación oficial: "All streams have two main settings that are mandatory and must at all times be configured for the stream to function at all. These are the stream name and stream source settings." El nombre debe ser de máximo 100 caracteres, solo minúsculas, números, `_`, `-` y `.`. El **source** es "literally just that: the source of the media data. It is a simple text field" — o sea, un string de texto libre, sin estructura de credenciales separada. Fuente: [Stream Settings — MistServer docs](https://docs.mistserver.org/mistserver/concepts/streams/).

### Pull vs push — **sí se distinguen, con esquemas de URL distintos**

MistServer trata **pull** y **push** como dos mecanismos claramente separados, cada uno con su propio formato de URL de origen:

- **Pull** (MistServer va a buscar la señal): se llena el campo "source" con la dirección y MistServer se conecta. Para HLS: `http://host/path/to/playlist.m3u8` (debe empezar en `http://` y terminar en `.m3u`/`.m3u8`). Para RTSP: `rtsp://[account:password@]host[:port][/path]` — aquí sí, credenciales embebidas en la URL, con la nota explícita "Account and password can be used for authentication, if not set no authentication will be attempted". Fuente: [Pull input for streams — MistServer docs](https://docs.mistserver.org/mistserver/concepts/streams/pull_inputs/).
- **Push** (algo nos empuja la señal): el source se pone como `push://` y funciona para RTMP y SRT. RTMP usa `rtmp://hostname:port/passphrase`. SRT usa `srt://mistserveraddress:port?streamid=TOKEN`. Fuente: [Push input for streams — MistServer docs](https://docs.mistserver.org/mistserver/concepts/streams/push_inputs/), con detalle adicional de autenticación por *push tokens* en [Beginners guide to push tokens](https://docs.mistserver.org/howto/integration/pushtokens/).

**Esto es exactamente el "for_tv" del cliente: MistServer hace pull del proveedor externo y sirve un HLS local que VLC vuelve a leer.**

### Prueba de conexión antes de guardar

La documentación **no menciona ningún botón de "probar conexión" ni validación previa de la fuente al crearla** — solo dice que ciertos caracteres inválidos en el nombre se descartan (y hay un bug reportado donde eso llega a rechazar la creación). Fuente: [Stream Settings — MistServer docs](https://docs.mistserver.org/mistserver/concepts/streams/). Lo que sí existe, **después** de crear el stream, es un botón de **"preview"** en el panel de Streams: da acceso al stream embebido en el navegador, con un "player log" (estado y mensajes de debug) y "meta information" (tipo de stream e información de pistas/tracks — códec, resolución, etc.). Fuente: [Streams Panel — MistServer docs](https://docs.mistserver.org/mistserver/configuration/interface/streams/). Es decir: no hay pre-validación antes de guardar, pero sí un diagnóstico post-guardado con info de códec.

### Reconexión y fallback

MistServer separa dos conceptos:
1. **Reconexión de la fuente pull en sí** — la documentación de pull inputs no da cifras de reintentos ni timeout. Textualmente: no hay información pública sobre esto en la documentación consultada.
2. **Fallback de contenido cuando el output no puede conectar al input** (fuente caída, stream no configurado, o live offline): se puede configurar `fallback_stream` por stream, y si no hay, se usa el `defaultStream` global. Los fallback stream son recursivos (un fallback puede tener otro fallback), y el sistema advierte que los fallback personalizados **no tienen protección contra bucles infinitos**, mientras que el `defaultStream` global sí la tiene y se aplica como máximo una vez. Fuente: [Fallback streams — MistServer docs](https://docs.mistserver.org/mistserver/concepts/streams/fallback_inputs/). No se documentan tiempos de espera concretos.

---

## 3. ffplayout

Soporta fuentes remotas como URL directamente en el playlist JSON (`"source": "https://example.org/big_buck_bunny.webm"`) y también HLS como salida por defecto. Fuente: documentación del repo, ver [ffplayout/docs](https://github.com/ffplayout/ffplayout/tree/master/docs) y [preview_stream.md](https://github.com/ffplayout/ffplayout/blob/master/docs/preview_stream.md).

Tiene una función específica de **live ingest** (inyectar una señal en vivo dentro del playout) documentada en [`live_ingest.md`](https://github.com/ffplayout/ffplayout/blob/master/docs/live_ingest.md). Dato de seguridad citado en la propia documentación: "FFmpeg has no built-in authentication mechanism and simply listens to the protocol and port" — así que ffplayout compensa monitoreando la salida de ffmpeg y **cortando el ingest si el nombre de app/stream RTMP entrante no coincide con el configurado**. Esto es la versión de ffplayout de "push" (alguien empuja hacia ffplayout). Hay un bug abierto documentando que el ingest en vivo **no funciona en modo HLS** — [issue #549](https://github.com/ffplayout/ffplayout/issues/549) — señal de que esta zona del producto está inmadura incluso en un proyecto hermano de Antena787 (mismo stack Rust/ffmpeg, mismo problema que estamos resolviendo).

No se encontró documentación de una lista de "fuentes con nombre" reutilizable tipo preset — el source va directo en el JSON del playlist o la config. No se encontró información pública sobre botón de prueba de conexión ni sobre política de reconexión con cifras.

---

## 4. OBS Studio — VLC Video Source y Media Source

La fuente "VLC Video Source" de OBS usa las DLL de VLC y acepta "every path/url VLC is able to play", con una lista editable de reproducción que admite archivos y URLs. Fuente: [obs-studio/plugins/vlc-video — GitHub](https://github.com/obsproject/obs-studio/tree/master/plugins/vlc-video), confirmado también en foros de OBS ([Media Playlist Source](https://obsproject.com/forum/resources/media-playlist-source.1765/)).

No se encontró documentación oficial sobre cómo maneja credenciales (hereda el comportamiento de VLC: van en la URL). No se encontró documentación pública sobre un botón de prueba de conexión ni sobre parámetros de reconexión automática específicos para esta fuente — el propio buscador de OBS lo confirma: "there doesn't appear to be comprehensive official documentation specifically covering credentials and authentication for URLs."

---

## 5. nginx-rtmp-module — el más explícito en push vs. pull, con cifras

Este es el hallazgo más útil para el diseño de reconexión. El módulo trata **push** y **pull** como directivas separadas y opuestas:

- `push <destino>` — cuando algo se publica en una aplicación, nginx-rtmp lo reenvía automáticamente a los destinos listados.
- `pull <origen> pageUrl=...` — nginx-rtmp va y busca streams de una máquina remota para reproducirlos localmente.

Fuente: [nginx-rtmp-module/README.md](https://github.com/arut/nginx-rtmp-module/blob/master/README.md), [Directives wiki](https://github.com/arut/nginx-rtmp-module/wiki/Directives).

**Reconexión, con cifras concretas y documentadas oficialmente:**
- `push_reconnect <timeout>`: tiempo de espera antes de reconectar una conexión push que se cayó. **Default: 3 segundos.** Poner el timeout en 0 hace la reconexión inmediata pero puede causar 100% de CPU si hay problemas de red persistentes.
- `pull_reconnect <timeout>`: lo mismo para pull, ejemplo dado: `2s`.
- `rtmp_auto_push_reconnect 1s`: intervalo de reconexión para el reenvío automático entre workers.

Fuente: [Streaming with nginx-rtmp-module: Push reconnect](https://nginx-rtmp.blogspot.com/2012/07/push-reconnect.html), discusión técnica en [issue #328](https://github.com/arut/nginx-rtmp-module/issues/328) y [issue #194](https://github.com/arut/nginx-rtmp-module/issues/194) del repo oficial.

No se documentó un límite máximo de reintentos — reintenta indefinidamente al intervalo configurado hasta que alguien lo detenga.

---

## 6. Wowza Streaming Engine

### Credenciales — cómo las guarda, en texto plano

Para fuentes RTMP/RTSP publicadas *hacia* Wowza (push), las credenciales de "source authentication" se guardan en `[install-dir]/conf/publish.password`, con una copia por aplicación en `[install-dir]/conf/[application]/publish.password`. El nombre de usuario y clave solo aceptan alfanuméricos, `.`, `_` y `-`. Fuente: [Publish from RTMP/RTSP with authentication](https://www.wowza.com/docs/how-to-enable-username-password-authentication-for-rtmp-and-rtsp-publishing). La documentación consultada no confirma cifrado de ese archivo específico (sí hay hash/bcrypt documentado para `admin.password`, el de la interfaz de administración, pero no para `publish.password`) — es decir, todo apunta a texto plano para las credenciales de fuente.

### Pull con nombre reutilizable — los `.stream` files

Para fuentes remotas que Wowza va a buscar (pull), existe el mecanismo de **stream files** (`.stream`): un archivo que actúa de **alias** de una URI de origen compleja. Cita textual: "Stream files provide a method to replace (alias) complex stream names... Players can then use mycoolevent.stream in playback URLs in place of the more complex stream name." Fuente: [Re-stream from another source in Wowza Streaming Engine](https://www.wowza.com/docs/how-to-create-and-use-stream-files-in-wowza-streaming-engine-manager). Esto es, en la práctica, la respuesta más limpia encontrada a la pregunta 1: **un nombre corto y reutilizable que esconde la URL real**, muy parecido a lo que necesitamos para "MistServer local" vs. "proveedor externo".

### Reconexión con cifra oficial

`streamTimeout` en `Application.xml`: tiempo en milisegundos que Wowza espera antes de intentar reconectar a un stream que se cayó, aplicable a RTP nativo, MPEG-TS, RTSP/RTP y SHOUTcast/Icecast. Ejemplo documentado: `<Value>12000</Value>` (**12 segundos**). Fuente: [Monitor and reconnect offline streams](https://www.wowza.com/docs/how-to-reconnect-to-offline-streams-native-rtp-mpeg-ts-rtsp-rtp-shoutcast-icecast). La documentación consultada no detalla número máximo de intentos ni qué se pone al aire durante la caída (para eso, ver SRS/OpenBroadcaster abajo).

---

## 7. SRS (Simple Realtime Server)

Distingue explícitamente **ingest** (SRS actúa de cliente y jala un stream — de archivo, stream remoto o dispositivo) de **publicar/push** (un encoder empuja hacia SRS). El ingest se define por `vhost`, con bloques `input { type stream; url rtmp://...; }` y un motor de salida que reencapsula. Fuente: [Ingest — SRS docs](https://ossrs.io/lts/en-us/docs/v5/doc/ingest), [v1_EN_Ingest wiki](https://github.com/ossrs/srs/wiki/v1_EN_Ingest). No se encontró documentación pública sobre cifras de reconexión ni sobre un botón de prueba de conexión — SRS es config por archivo de texto, no tiene interfaz gráfica de gestión de fuentes.

---

## 8. Restreamer (datarhei)

Tiene un campo único "Network source" que acepta HTTP/HTTPS (HLS/DASH), RTP, RTSP, RTMP y SRT, con nota textual de que "if required, the access data of the video source can be entered here" (o sea, sí hay un campo separado para credenciales, no todo forzado dentro de la URL). Fuente: [Network source — Restreamer docs](https://docs.datarhei.com/restreamer/knowledge-base/manual/edit-livestream/general/video-settings/network-source). Confirmado que reconecta automáticamente: "Restreamer will reconnect to the source and target immediately starting up." Fuente: [FAQ — Restreamer](https://docs.datarhei.com/restreamer/knowledge-base/faq). La documentación pública consultada **no da cifras de intervalo de reintento ni un botón explícito de "probar conexión" antes de arrancar** — el flujo documentado es pegar la URL y darle "Start" directamente.

---

## 9. Rivendell, OpenBroadcaster, Nageru

- **Rivendell**: es automatización de radio (audio), no maneja "fuentes de video por URL" en el sentido de esta investigación. Su streaming se hace vía JACK hacia un encoder; no hay gestión de fuentes remotas de entrada documentada. Fuente: [Streaming from Rivendell — wiki](https://wiki.rivendellaudio.org/index.php/Streaming_from_Rivendell).
- **OpenBroadcaster (OBPlayer/OBServer)**: dato más útil aquí es su **cadena de fallback documentada** (radio, no TV, pero el patrón aplica): "If the schedule has gaps, it plays a default playlist. If that fails, it switches to Fallback Media Mode, then to analog input bypass, and finally to a test signal as a last resort." **Aviso importante:** esta cita viene de un resumen de búsqueda de terceros, no se pudo confirmar textualmente contra la documentación oficial de OpenBroadcaster (la página de troubleshooting revisada directamente no la contenía) — tratarla como pista a verificar, no como hecho confirmado. Fuente citada originalmente: [Obplayer — OpenBroadcaster support](https://support.openbroadcaster.com/obplayer/); página oficial revisada sin esa confirmación: [Troubleshooting — OpenBroadcaster](https://support.openbroadcaster.com/troubleshooting). El **patrón de cascada de fallback en sí** (contenido normal → medio de respaldo → entrada análoga → señal de prueba) es, aun así, la idea de diseño más aprovechable de toda la investigación para la pregunta 4.
- **Nageru**: mezclador en vivo, no playout con fuentes-URL gestionadas — acepta "anything FFmpeg accepts, including network streams" como input de un chain, pero no hay concepto de "fuente guardada con nombre" documentado. Fuente: [Video inputs — Nageru docs](https://nageru.sesse.net/doc/streaming.html).

---

## 10. Cinegy, PlayBox, BroadStream OASYS (broadcast profesional)

**BroadStream OASYS**: es un sistema de playout SDI/IP con datasheet público pero sin manual técnico de configuración de entradas accesible en la búsqueda — no hay información pública suficiente sobre cómo guarda credenciales o gestiona reconexión. Fuente (solo datasheet de marketing): [OASYS IP Datasheet](https://broadstream.com/wp-content/uploads/2021/06/OASYS-IP-Datasheet.pdf).

**Cinegy Air/Encode**: sí tiene un patrón de diseño documentado y aprovechable — un campo de **"Default URL"** a nivel de dispositivo de entrada, que se usa automáticamente para ítems en vivo, pero que **se puede sobreescribir por ítem individual** en el diálogo de propiedades. Fuente: resultado de búsqueda sobre [Input Configuration — Cinegy Open](https://open.cinegy.com/products/air/24.1/encode/user-manual/input-configuration/) (la página se intentó abrir directamente y redirigió sin contenido accesible — la cita viene del resumen de búsqueda, no de lectura directa del manual). Esto es exactamente el patrón "preset con override": una fuente por defecto reutilizable, pero cualquier bloque de programación puede apuntar a otra URL puntual sin tocar el preset. No se encontró información pública sobre timeouts de reconexión ni botón de prueba en las páginas accesibles — se intentó `RTP/UDP/SRT Input` de Cinegy Open y esa página **sí confirma que existe selección de adaptador de red y offsets de tiempo, pero no publica cifras de reconexión**. Fuente: [RTP/UDP/SRT Input — Cinegy Open](https://open.cinegy.com/products/air/24.11/playout/user-manual/configuration/rtp/).

**Playbox**: no se encontró documentación técnica pública específica sobre su modelo de entradas IP en esta búsqueda — no hay información pública sobre esto.

---

## Las cinco preguntas, respondidas

**1. ¿Lista de fuentes con nombre, reutilizable, o URL suelta cada vez?**
Casi todos los que gestionan más de una fuente la nombran: VLC/VLM llama a cada una `media` (con nombre tipo `channel1`); MistServer la llama `stream` (nombre corto, obligatorio, con reglas de caracteres); Wowza la llama **stream file** (`.stream`) y la describe literalmente como *alias* de una URL compleja; Cinegy usa **"Default URL"** por dispositivo con override por ítem. Ninguno de los productos con interfaz gráfica revisados obliga a pegar la URL suelta cada vez — todos dan un nombre corto de por medio. Los que son puro archivo de config (SRS, nginx-rtmp) también nombran el bloque (`ingest livestream { ... }`, aplicación de relay), aunque no hay "pantalla" — es config de texto.

**2. Credenciales — ¿cifradas, texto plano, o dentro de la URL?**
El patrón dominante es **credenciales dentro de la URL** (`rtsp://usuario:clave@host`, `srt://host?streamid=TOKEN`): así lo hace MistServer (pull RTSP), VLC/VLM (todo), y SRT en general vía `streamid`. La excepción es **Restreamer**, que documenta un campo separado para "access data" fuera del campo de dirección — mejor práctica visible en esta investigación. **Wowza** guarda usuario/clave en un archivo de texto separado (`publish.password`) pero sin evidencia de cifrado para ese archivo específico. **No se encontró en ninguna fuente oficial una recomendación explícita de cifrar credenciales de fuente** — el estado del arte real, no el ideal, es texto plano en algún lado (URL o archivo de config).

**3. ¿Se prueba la URL antes de guardarla?**
Ninguno de los productos con documentación pública revisada confirma una validación **previa** a guardar (ni MistServer, ni Restreamer, ni Cinegy). Lo que sí existe, y es el mejor precedente encontrado, es el botón **"preview"** de MistServer **después** de crear el stream: reproduce en el navegador y muestra "player log" (estado/debug) y "meta information" (tipo de stream, tracks/códec). Es diagnóstico post-guardado, no un gate previo.

**4. Reconexión — cifras concretas.**
- nginx-rtmp: `push_reconnect` default **3s**; `pull_reconnect` ejemplo **2s**; `rtmp_auto_push_reconnect` **1s**. Reintenta indefinidamente, sin límite de intentos documentado.
- Wowza: `streamTimeout` ejemplo **12000 ms (12s)** antes de reintentar.
- MistServer: fallback a otro stream (`fallback_stream` → `defaultStream`) cuando la fuente no conecta, sin cifras de tiempo documentadas, pero con protección anti-bucle solo en el `defaultStream` global.
- OpenBroadcaster (patrón, no cifra, y sin confirmar en fuente oficial directa): cascada default playlist → fallback media → bypass análogo → señal de prueba.
- VLC/VLM, ffplayout, Nageru, SRS: no se encontró documentación pública con cifras de reconexión.

**5. ¿Pull vs. push como cosas distintas?**
Sí, y donde está mejor resuelto es **MistServer** (esquema de URL distinto: `push://` vs. dirección directa para pull) y **nginx-rtmp** (directivas `push` y `pull` separadas, cada una con su propio timeout de reconexión). ffplayout solo documenta el caso push (ingest, con verificación de nombre de stream por seguridad) y no tiene modelo de pull con nombre. SRS también separa "ingest" (pull) de "publish" (push) a nivel conceptual y de configuración.

---

## Qué cambia nuestro diseño

1. **Nombrar la fuente, no pegar la URL suelta.** El modelo mínimo viable, visto en todos los productos serios: `nombre` + `url` + `tipo` (pull/push) + `credenciales` (si aplica) — igual que un `.stream` de Wowza o un `stream` de MistServer.
2. **Separar pull de push desde el modelo de datos**, no como una casilla — son dos formas de operar con implicaciones distintas de seguridad (push necesita validar quién te empuja, como hace ffplayout verificando el nombre de stream RTMP entrante). El caso del cliente hoy es 100% pull (MistServer jala del proveedor externo, VLC jala de MistServer).
3. **Credenciales: campo separado de la URL, no concatenado.** Es el patrón de Restreamer, y evita que la clave quede pegada en logs o en la URL mostrada en pantalla — aunque el mundo real (VLC, MistServer RTSP) las mete en la URL, no hay que copiar esa parte floja.
4. **No hay estándar de "probar antes de guardar" que copiar — hay que inventarlo bien**, porque nadie lo hace bien documentado. Lo más cercano es el "preview" post-guardado de MistServer (con meta de códec/tracks). Recomendación: al guardar la fuente, intentar un `ffprobe` corto contra la URL y mostrar códec/resolución/bitrate antes de confirmar — eso sería mejor que lo que existe hoy en el mercado.
5. **Reconexión con cifras explícitas y un límite razonable**, tomando nginx-rtmp/Wowza como referencia: reintentar cada 2-5 segundos, sin tope de intentos para pull (el canal no se puede quedar sin intentar), y definir qué se pone al aire mientras tanto (slate/barra de color o el input previo) — el patrón de cascada de OpenBroadcaster (fallback → bypass → señal de prueba) es el modelo a copiar para esa lógica, aunque haya que documentarlo nosotros mismos porque nadie más lo hizo bien.

---

## Fuentes citadas (todas oficiales salvo donde se indica)

- [Documentation:Streaming HowTo/VLM — VideoLAN Wiki](https://wiki.videolan.org/Documentation:Streaming_HowTo/VLM/)
- [doc/vlm.txt — repo VLC](https://github.com/videolan/vlc/blob/master/doc/vlm.txt)
- [VLM - Multiple streaming and Video on demand](https://www.videolan.org/doc/streaming-howto/src/en/vlm-vod.xml)
- [IPTV M3U Username Password in VLC](https://iptv-subs.com/iptv-m3u-username-password/) (guía de terceros, no oficial)
- [Stream Settings — MistServer docs](https://docs.mistserver.org/mistserver/concepts/streams/)
- [Pull input for streams — MistServer docs](https://docs.mistserver.org/mistserver/concepts/streams/pull_inputs/)
- [Push input for streams — MistServer docs](https://docs.mistserver.org/mistserver/concepts/streams/push_inputs/)
- [Beginners guide to push tokens — MistServer docs](https://docs.mistserver.org/howto/integration/pushtokens/)
- [Fallback streams — MistServer docs](https://docs.mistserver.org/mistserver/concepts/streams/fallback_inputs/)
- [Streams Panel — MistServer docs](https://docs.mistserver.org/mistserver/configuration/interface/streams/)
- [ffplayout/docs — repo](https://github.com/ffplayout/ffplayout/tree/master/docs)
- [live_ingest.md — ffplayout](https://github.com/ffplayout/ffplayout/blob/master/docs/live_ingest.md)
- [issue #549 — Live ingest does not work in HLS mode](https://github.com/ffplayout/ffplayout/issues/549)
- [obs-studio/plugins/vlc-video — GitHub](https://github.com/obsproject/obs-studio/tree/master/plugins/vlc-video)
- [Media Playlist Source — OBS forums](https://obsproject.com/forum/resources/media-playlist-source.1765/)
- [nginx-rtmp-module/README.md](https://github.com/arut/nginx-rtmp-module/blob/master/README.md)
- [Directives — nginx-rtmp-module wiki](https://github.com/arut/nginx-rtmp-module/wiki/Directives)
- [Streaming with nginx-rtmp-module: Push reconnect](https://nginx-rtmp.blogspot.com/2012/07/push-reconnect.html)
- [issue #328 — Push reconnect on connection drop](https://github.com/arut/nginx-rtmp-module/issues/328)
- [issue #194 — reconnect on_publish directive](https://github.com/arut/nginx-rtmp-module/issues/194)
- [Publish from RTMP/RTSP with authentication — Wowza](https://www.wowza.com/docs/how-to-enable-username-password-authentication-for-rtmp-and-rtsp-publishing)
- [Re-stream from another source in Wowza Streaming Engine](https://www.wowza.com/docs/how-to-create-and-use-stream-files-in-wowza-streaming-engine-manager)
- [Monitor and reconnect offline streams — Wowza](https://www.wowza.com/docs/how-to-reconnect-to-offline-streams-native-rtp-mpeg-ts-rtsp-rtp-shoutcast-icecast)
- [Ingest — SRS docs](https://ossrs.io/lts/en-us/docs/v5/doc/ingest)
- [v1_EN_Ingest — SRS wiki](https://github.com/ossrs/srs/wiki/v1_EN_Ingest)
- [Network source — Restreamer docs](https://docs.datarhei.com/restreamer/knowledge-base/manual/edit-livestream/general/video-settings/network-source)
- [FAQ — Restreamer docs](https://docs.datarhei.com/restreamer/knowledge-base/faq)
- [Streaming from Rivendell — wiki](https://wiki.rivendellaudio.org/index.php/Streaming_from_Rivendell)
- [Obplayer — OpenBroadcaster support](https://support.openbroadcaster.com/obplayer/) / [Troubleshooting — OpenBroadcaster](https://support.openbroadcaster.com/troubleshooting) (cascada de fallback sin confirmar textualmente en la página oficial revisada)
- [Video inputs / Streaming — Nageru docs](https://nageru.sesse.net/doc/streaming.html)
- [OASYS IP Datasheet — BroadStream](https://broadstream.com/wp-content/uploads/2021/06/OASYS-IP-Datasheet.pdf) (solo marketing)
- [Input Configuration — Cinegy Open](https://open.cinegy.com/products/air/24.1/encode/user-manual/input-configuration/) (cita de resumen de búsqueda, página no se pudo leer directamente)
- [RTP/UDP/SRT Input — Cinegy Open](https://open.cinegy.com/products/air/24.11/playout/user-manual/configuration/rtp/)
