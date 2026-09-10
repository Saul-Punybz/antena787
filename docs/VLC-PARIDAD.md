# Paridad con VLC — lo que Rolando hace hoy y Antena787 tiene que hacer

_Regla fijada por Saul el 9 de septiembre de 2026: CAtv emite hoy con VLC.
Todas las funciones de stream de VLC que Rolando usa —o pueda usar— tienen
que existir en Antena787. Si algo que él hace con VLC no se puede hacer
aquí, no va a apagar VLC, y el modo sombra no termina nunca._

La cadena de hoy (PRD §17, §25): MistServer recibe los streams en vivo, VLC
convierte a **MPEG-2 720p con audio MPEG y lo manda por UDP** al multiplexor
Technalogix TP1000. Antena787 ocupa el lugar de MistServer + VLC.

## 1 · Salida (el `sout` de VLC) contra el diseño de Antena787

| Función de VLC | Lo que hace Rolando con ella | Antena787 | Fase |
|---|---|---|---|
| **UDP unicast** (`udp{dst=ip:puerto}`, mux `ts`) | Mandar el TS al multiplexor | `udp-ts` MPEG-2 CBR (PRD §10, F2-46, F2-50) | **F2, lo primero** |
| **UDP multicast** con `ttl=` | Si el TP1000 o un receptor escucha en un grupo multicast | `udp-ts` tiene que aceptar grupo multicast y TTL como opciones visibles («¿a qué dirección lo mando?») | **F2-114** |
| **RTP** (`rtp{dst=,port=,sap,name=}`) | Alternativa al UDP crudo; SAP anuncia el stream en la red | `rtp` está nombrado en §10 («UDP-TS y RTP»); el anuncio SAP no | **F2** (RTP) · SAP solo si él lo usa |
| **Opciones del mux TS**: `pid-video`, `pid-audio`, `pid-pmt`, `tsid`, `program`, `pcr=`, `dts-delay`, `shaping`, `use-key-frames` | Los PIDs y el número de programa que el TP1000 espera | §10: «PIDs y número de programa fijos que la persona escribe una vez, PCR ≤ 40 ms, PAT/PMT a tiempo, CBR con paquetes nulos» | **F2** (ya en el diseño) |
| **Transcode**: `vcodec=mp2v` / `h264` / `hevc`, `vb=`, `scale`, `fps`, `deinterlace`, `acodec=mpga` / `mp4a` / `a52`, `ab=`, `channels`, `samplerate` | 720p MPEG-2 + audio MPEG capa II hacia el transmisor; H.264 + AAC hacia internet | Formato de casa (720p59.94) + por salida: MPEG-2 con AC-3 o MPEG L2 (`udp-ts`), H.264/AAC (`internet`); HEVC en F5 | **F2** |
| **`duplicate{dst=…,dst=…}`** | La misma señal al transmisor y a internet a la vez | Varias salidas simultáneas, cada una con su volumen y su reconexión (F2-46, F2-47, F2-49) | **F2** |
| **`display`** dentro de `duplicate` | Ver en la pantalla del PC lo que está saliendo — y hoy es la **única** forma que tiene: la versión actual de MistServer ya no abre esa ventana (Rolando, 9 sept 2026), así que sin VLC, Rolando no tiene cómo ver su salida en el PC de la torre | Decidido: ventana en el navegador a pantalla completa, servida por el propio motor en baja latencia (`http-ts`/HLS de baja latencia, no una ventana nativa, sin CGo ni SDK), ≤3 s de retraso frente al aire | **F2-117** |
| **HTTP TS** (`http{mux=ts,dst=:8080/}`) | Que otro programa (MistServer, un VLC remoto, un monitor) tire de la señal | No está construido aún: solo hay `udp-ts`, `internet` (RTMP/HLS/SRT) y `archivo`; ya es criterio (es barato: el mismo TS servido por HTTP) | **F2-115** |
| **HLS** (`livehttp`) | Salida web | `internet` → HLS (§10) | **F2** |
| **SRT** (`srt{dst=}`) | Salida a un servidor o a otra estación | `internet` → SRT (§10) | **F2** |
| **RTMP** (`rtmp://`) | YouTube, Facebook, servidor propio | `internet` → RTMP (§10, F2-48 reconexión) | **F2** |
| **RTSP** (`rtsp{sdp=}`) | Servir bajo demanda | No está. Es de VOD, raro en playout; solo si él lo usa | Pregunta |
| **`file{dst=}` / botón grabar** | Guardar lo que salió | `archivo` (§10) + grabación de la salida con retención 7/30 días (F2-43) | **F2** |
| **Repetir / bucle de la lista** | Que nunca se quede en negro | El resolver + relleno + cartel: nunca negro, nunca silencio (F1) | **Hecho** |
| **Marquesina y logo** (`marq`, `logo` sub-filters) | Logo del canal, texto en pantalla | Logo del canal (F2), tabla `overlay`, crawl de alertas CAP (ADR 0010) | **F2** |
| **`sout-keep`** (no cortar la salida entre elementos de la lista) | Que el multiplexor no pierda el stream entre archivos | Es el núcleo del motor: **un encoder persistente** que nunca se reinicia entre clips (F0, F2-01) — VLC lo hace a medias, Antena787 mejor | **Hecho en F0** |
| Salida solo audio | Radio (Océano Radio en el mismo sitio) | `solo_audio` en fuentes y en el canal de radio (F4b) | **F4b** |
| `mosaic`, `bridge`, `smem`, Chromecast | — | No aplica a un playout | no |

## 2 · Entrada (lo que VLC abre) contra el diseño

| Función de VLC | Uso en CAtv | Antena787 | Fase |
|---|---|---|---|
| Archivos locales y listas | La biblioteca | Carpeta vigilada, ingest, normalización | **Hecho (F1)** |
| **Recibir** RTMP / SRT que alguien empuja | RadioOnce Live!, un noticiero desde OBS | `rtmp-listen`, `srt-listen` (§10) | **F2** |
| **Tirar** de una URL: `udp://@`, `rtsp://`, `http://…ts`, `hls`, `rtmp://` | Una cámara IP, un stream remoto, lo que hoy le da MistServer | No está construido aún: `live_source.tipo` solo tiene `srt`, `rtmp`, `captura`. Ya es criterio **`url`** (el motor tira de la fuente, mismo camino que un vivo por SRT ante una ausencia) | **F2-116** |
| Tarjeta de captura (DirectShow en Windows) | Entrada de un mezclador o una cámara por captura | `captura` (§10), por ffmpeg `dshow`; Decklink/NDI fuera (CGo) | **F2** |
| Captura de pantalla, DVB, disco | — | No aplica | no |

## 3 · Criterio de aceptación

- **F2-113** [MANUAL] — Dado la configuración real de VLC con la que CAtv
  emite hoy (la cadena `sout` o el archivo `.vlm`/`.xspf` que usa Rolando) ·
  Cuando se configura la salida de Antena787 en el asistente · Entonces
  **cada opción de esa cadena tiene su equivalente en pantalla, en lenguaje
  llano** (destino, multicast y TTL, PIDs y programa, códecs y bitrate,
  salidas simultáneas, grabación), el TP1000 recibe el TS sin cambiar nada
  de su lado, y Rolando confirma que no le falta nada de lo que hacía con
  VLC. Sin esa firma, VLC no se apaga.

Los cuatro huecos concretos que esta tabla señalaba ya son criterio propio:
multicast y TTL explícitos (**F2-114**), salida `http-ts` (**F2-115**),
entrada `url` (**F2-116**) y ventana local de monitor (**F2-117**). Están
en `docs/ACEPTACION.md`, sección F2, justo después de F2-50.

## 4 · Preguntas para Rolando (ya, antes de F2)

1. **La cadena exacta de VLC** con la que emite: el `sout` (o el `.vlm` /
   `.xspf` guardado, o el atajo de Windows con los parámetros), y la
   versión de VLC.
2. ¿El TP1000 recibe **unicast a una IP y puerto, o multicast**? ¿Con qué TTL?
3. ¿Los PIDs y el número de programa los fija VLC, o el TP1000 los remapea?
4. ¿MistServer **empuja** a VLC o VLC **tira** de MistServer (y por qué
   protocolo: HTTP TS, RTMP, HLS)? Es lo que decide si hace falta `url`.
5. ¿Usa el `display` de VLC para ver la salida en el monitor de la torre?
   **Confirmado por Rolando, 9 sept 2026: hoy es la única forma que tiene**
   — la versión actual de MistServer ya no abre esa ventana (ver F2-117).
6. ¿Graba con VLC lo que sale, o con otra cosa?
7. ¿Alguien más tira de la señal por HTTP desde otro equipo (monitor,
   streaming) que hoy sirva VLC?
