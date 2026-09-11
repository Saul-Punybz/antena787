# Aceleración por hardware en playout: qué hacen los demás (decode vs encode)

Fecha: 11 de septiembre de 2026
Alcance: investigación web, sin cambios de código. Para el caso real de Antena787 (Class A de Puerto Rico, Windows 10, salida MPEG-2 ~4 Mb/s 720p59.94 por UDP multicast).

## Resumen para decidir rápido

1. **La documentación oficial de ffmpeg dice, con esas palabras, que la mayoría de los métodos de `-hwaccel` "están pensados para reproducción y no serán más rápidos que decodificar por software en CPUs modernas", y que ffmpeg "usualmente necesitará copiar los cuadros decodificados de la memoria de la GPU a la memoria del sistema, lo que resulta en más pérdida de rendimiento".** Fuente: [ffmpeg.org/ffmpeg.html](https://ffmpeg.org/ffmpeg.html), sección `-hwaccel`. Esto es evidencia directa del proyecto, no opinión de foro.
2. **VLC trae la decodificación por hardware apagada por defecto**, tanto en general como específicamente en Windows (Herramientas → Preferencias → Entrada/Códecs → "Hardware-accelerated decoding" = Disable). Fuente: [VideoLAN Wiki, VLC GPU Decoding](https://wiki.videolan.org/VLC_GPU_Decoding). No hay evidencia pública de que VLC acelere por hardware el lado de *encoding* al transcodificar hacia `sout`; eso requiere forzar manualmente el encoder de FFmpeg (`h264_nvenc`, `h264_amf`, etc.) vía `--sout-avcodec-codec`, no es el camino por defecto. Fuente: [VLC Desktop User Documentation, transcoding with FFmpeg AMF codecs](https://docs.videolan.me/vlc-user/desktop/3.0/en/advanced/other/transcoding_with_ffmpeg_amf_codecs.html).
3. **El hwaccel de decode solo paga cuando toda la cadena decode→filtro→encode se queda en la GPU.** Si el encoder final es por software (como el `mpeg2video`/`libx264` de Antena787), ffmpeg baja los cuadros de la GPU a RAM igual (`hwdownload`), y ese viaje de ida y vuelta por PCIe cuesta más de lo que ahorra. Esto es consistente entre la doc de ffmpeg y foros técnicos de NVIDIA. Fuentes abajo.
4. **La industria de broadcast de verdad no resuelve "decodificar más rápido con GPU": resuelve no decodificar.** El patrón documentado (Ross/Techex, BroadStream OASYS) es transcodificar una sola vez al ingerir (sin reloj, sin presión de tiempo real) y, en el aire, mover los bits comprimidos sin decodificar ("compressed-domain switching" / "stream copy"), porque el ciclo completo decode-switch-reencode "añade costo, latencia y degrada la calidad cada vez que se repite". Fuente: [Techex, Playout Protection](https://techex.tv/solutions/playout-protection).

**Recomendación en una frase:** no hay evidencia de que activar hwaccel en el decode del lado de Antena787 vaya a ayudar mientras el encoder de salida siga siendo software — la ganancia real, documentada por la industria, está en no decodificar en el air chain, no en decodificar más rápido.

---

## Tabla comparativa

| Proyecto/producto | Decode acelerado | Encode acelerado | ¿Por defecto? | Nota |
|---|---|---|---|---|
| **VLC** | Sí, soportado (DXVA2/D3D11VA en Windows, VAAPI en Linux, VideoToolbox en macOS) | No de forma nativa/automática al usar `sout`; hay que forzar encoder FFmpeg (nvenc/amf) manualmente | **No** — apagado por defecto en todas las plataformas | [wiki.videolan.org/VLC_GPU_Decoding](https://wiki.videolan.org/VLC_GPU_Decoding) |
| **ffmpeg (core)** | Sí, vía `-hwaccel` (cuda/qsv/d3d11va/dxva2/videotoolbox/vaapi) | Sí, vía encoders dedicados (`h264_nvenc`, `hevc_qsv`, etc., separado de `-hwaccel`) | No, todo opt-in | El propio manual advierte que decode-hwaccel "no será más rápido que software en CPUs modernas" si hay que bajar los cuadros a RAM. [ffmpeg.org/ffmpeg.html](https://ffmpeg.org/ffmpeg.html) |
| **ffplayout** | Existe como opción de configuración expuesta al usuario (`-hwaccel`, `-hwaccel_output_format`, ej. cuvid/vaapi) en discusiones de la comunidad | No documentado como default | No, opt-in del operador | No encontré el flag `hwaccel` en el código fuente actual del repo (`ffplayout/ffplayout`, rama activa) al buscarlo — la evidencia de uso viene de foros/discusiones de usuarios armando su propio comando, no de la documentación oficial del proyecto. Ver [Discussion #681](https://github.com/ffplayout/ffplayout/discussions/681). |
| **LibreTime** | No aplica (Liquidsoap, audio) | No aplica | — | Es automatización de **radio**; Liquidsoap transcodifica audio o copia streams sin re-encodificar cuando no hace falta, pero no hay decode de video ni GPU en juego. [liquidsoap.info](https://www.liquidsoap.info/) |
| **Rivendell** | No aplica (audio) | No aplica | — | Sistema de automatización de radio en GNU/Linux; no maneja video. [rivendellaudio.org](https://www.rivendellaudio.org/) |
| **CasparCG** | No por defecto; discutido en foros como "menos directo" que el encode | Opt-in vía FFmpeg Consumer (`nvenc_h264`), pedido explícitamente por la comunidad para Linux | No | Ver issue pidiendo la opción: [CasparCG/server#1282](https://github.com/CasparCG/server/issues/1282) — "para ayudar a justificar el costo de comprar una GPU dedicada". El decode GPU se discute en el foro como más complicado de lograr que el encode. [CasparCG forum, GPU decoding/CUDA](https://casparcgforum.org/t/gpu-decoding-cuda/5424) |
| **OBS Studio** | Sí (para *fuentes* de video en algunos casos vía media source), pero su fuerte es encode | Sí, de forma prominente: NVENC, AMF, QSV, VideoToolbox — "texture-based encoding" GPU-a-GPU sin pasar por CPU | Encode: recomendado, no forzado; el usuario elige encoder | [obsproject.com/kb/hardware-encoding](https://obsproject.com/kb/hardware-encoding) |
| **MistServer** | Vía FFmpeg/libav interno (MistProcAV) | Sí, NVENC soportado como mejora pedida y luego incorporada | Opt-in | [MistProcAV docs](https://docs.mistserver.org/mistserver/processes/mistprocav/); pedido original en [DDVTECH/mistserver#171](https://github.com/DDVTECH/mistserver/issues/171) |
| **Nageru** | No — decode no es su caso de uso (recibe SDI/USB3 en crudo) | Sí, requiere Intel Quick Sync (QSV/VAAPI) o AV1 software (SVT-AV1); casi todo el pixel-processing es GPU vía OpenGL | **Sí, es un requisito**, no opcional | [nageru.sesse.net](https://nageru.sesse.net/) |
| **OpenBroadcaster** | No aplica en el núcleo (es automatización de **radio**, con soporte de TV vía streaming, no un transcoder de video) | No documentado | — | [openbroadcaster.com](https://www.openbroadcaster.com/software/radio-automation-software/) |
| **Dinesat (Hardata)** | Sin información pública sobre GPU/hwaccel | Sin información pública | — | Es primariamente automatización de **radio**; no encontré documentación pública de requisitos GPU. |
| **VirtualPowerVideo** | Sin información pública encontrada | Sin información pública encontrada | — | No se encontró el producto en las búsquedas realizadas (nombre puede ser distinto o el producto tiene documentación no indexada). |
| **PlayBox Neo (AirBox)** | Sí, "Intel CPU/GPU hardware acceleration" mencionado en la doc de producto | Servidores "GPU acceleration ready" como opción de configuración | Opción de servidor, no un default universal | [playboxtechnology.com/2019/10/mega-upgrades-for-playbox-neo-range](https://playboxtechnology.com/2019/10/mega-upgrades-for-playbox-neo-range/) |
| **Cinegy (Air/Player)** | GPU exigido para algunos formatos (Daniel2 requiere CUDA CC 3.5+) | Sí, de forma explícita: "Cinegy leverages NVIDIA's hardware-based H.264 and H.265 encoding engines instead of using CPU" para Air PRO Bundle con Quadro M6000 | Sí en las líneas de producto "PRO"/Ultra HD | [Cinegy Player PRO 2 System Recommendations (PDF)](https://open.cinegy.com/products/player-pro/2/system-recommendations/Cinegy-Player-PRO-2-System-Recommendations.pdf), [Cinegy and NVIDIA](https://home.cinegy.com/technology-partners/cinegy-and-nvidia/) |
| **Imagine Communications / Grass Valley** | Sin mención pública específica de GPU para decode | Sin mención pública específica de GPU para encode | — | Sus productos (Versio, Nexio, ICE, Playout X) se documentan en términos de baseband/SDI, IP y despliegue cloud-nativo, no de aceleración GPU puntual. [imaginecommunications.com](https://imaginecommunications.com/make-tv/products/playout-and-channel-origination/), [grassvalley.com/products/ampp/playout-x](https://www.grassvalley.com/products/ampp/playout-x/) |
| **BroadStream OASYS** (referencia de "no transcodificar") | — | — | — | Documenta explícitamente **evitar el transcode**: reproduce `.ts` nativo "skipping the transcoding process". [broadstream.com/products/oasys](https://broadstream.com/products/oasys/) |

---

## 1. VLC — lo que usa hoy el cliente para el multicast

**Decode:** VLC sí soporta decodificación acelerada (DXVA 2.0 en Windows desde Vista, VAAPI en Linux, VideoToolbox en macOS), pero **está desactivada por defecto** en todas las plataformas, incluyendo Windows. El wiki oficial dice textualmente: *"by default, hardware acceleration is disabled"*, y advierte que ni siquiera está disponible para aplicaciones externas vía libVLC sin activarla a mano. Fuente: [wiki.videolan.org/VLC_GPU_Decoding](https://wiki.videolan.org/VLC_GPU_Decoding).

Para activarla hay que ir a Herramientas → Preferencias → Entrada/Códecs → Codecs → "Hardware-accelerated decoding", cambiar de "Disable" a "Automatic". No hay evidencia de que VLC la encienda sola según el hardware detectado; el cambio es manual.

**Transcode/`sout`:** VLC arma la cadena de transcodificación (`--sout '#transcode{...}'`) con los encoders de FFmpeg que trae compilados. Para usar el encoder de GPU hay que pedirlo explícitamente por nombre (`venc=avcodec` + `--sout-avcodec-codec=h264_nvenc` o `hevc_amf`), no es automático ni recomendado como default en la documentación de VideoLAN. Fuentes: [VLC Desktop User Docs — transcoding with FFmpeg AMF codecs](https://docs.videolan.me/vlc-user/desktop/3.0/en/advanced/other/transcoding_with_ffmpeg_amf_codecs.html), [GPUOpen AMF wiki — VLC and AMF](https://github.com/GPUOpen-LibrariesAndSDKs/AMF/wiki/VLC-and-AMF).

**Windows:** no encontré documentación de VideoLAN que diga que el comportamiento por defecto cambie en Windows frente a otras plataformas — sigue apagado. Sí hay reportes de usuarios (foros, no documentación oficial) sobre DXVA2 fallando en builds recientes de Windows 11 y recomendando usar D3D11 en su lugar; esto es señal de foro, no documento oficial de VideoLAN.

## 2. ffmpeg — qué significa `-hwaccel` de verdad

La página oficial `ffmpeg.html` documenta `-hwaccel[:stream_specifier] <method>` como una **opción de entrada** — solo afecta al decode, nunca al encode — con valores: `none` (default), `auto`, `vdpau`, `dxva2`, `d3d11va`, `vaapi`, `qsv`, `videotoolbox`. Fuente: [ffmpeg.org/ffmpeg.html](https://ffmpeg.org/ffmpeg.html).

La misma página trae la advertencia clave, citada textual:

> "most acceleration methods are intended for playback and will not be faster than software decoding on modern CPUs [...] ffmpeg will usually need to copy the decoded frames from the GPU memory into the system memory, resulting in further performance loss."

Es decir: el propio proyecto dice que decode-hwaccel es una optimización pensada para *reproducir* video en equipos con CPU débil (tablets, set-top boxes), no para *transcodificar* rápido en un servidor — y que el costo de la copia GPU→RAM (`hwdownload`) suele comerse la ganancia.

`-hwaccel_output_format` le dice a ffmpeg que deje el cuadro decodificado en formato de superficie de GPU (en vez de bajarlo a YUV en RAM automáticamente). Esto **solo tiene sentido si algo después también corre en GPU** — un filtro CUDA/VAAPI o un encoder por hardware. Si el siguiente paso es un encoder de software (como `libx264`/`mpeg2video`), ffmpeg baja los cuadros a RAM de todos modos, así que declarar `hwaccel_output_format` no aporta nada. Fuente de esta mecánica: doc de ffmpeg + explicaciones técnicas coincidentes en foros de NVIDIA sobre "mixing CPU and GPU processing" — anotado aquí como respaldo técnico, no como cita oficial de proyecto: [forums.developer.nvidia.com/t/ffmpeg-mixing-cpu-and-gpu-processing](https://forums.developer.nvidia.com/t/ffmpeg-mixing-cpu-and-gpu-processing/199899).

**Cuándo no compensa (según la propia doc de ffmpeg y la mecánica documentada):**
- Cuando el encoder de salida es por software — el `hwdownload` es obligatorio igual, y ahí se pierde lo ganado en decode.
- Cuando el CPU disponible ya decodifica sin problema en tiempo real (caso típico de MPEG-2/H.264 a 720p en un servidor moderno).
- Cuando hay múltiples decoders abriéndose y cerrándose por clip (como en Antena787): cada apertura/cierre de contexto de hwaccel tiene su propio costo de inicialización que no existe en software.

## 3. Playout libres

- **ffplayout:** no encontré documentación oficial del proyecto (README, docs/) que declare `hwaccel` como característica soportada o rechazada; la única evidencia pública es una discusión de la comunidad ([Discussion #681](https://github.com/ffplayout/ffplayout/discussions/681)) donde un usuario comparte su propio comando con `-hwaccel cuvid -hwaccel_output_format cuda`, como parámetro custom de ffmpeg que el propio operador puede pasar. Al buscar `hwaccel` en el código fuente actual del repo no aparece como flag de configuración de primera clase. No hay evidencia de que el proyecto lo haya rechazado explícitamente tampoco — simplemente no hay declaración pública al respecto.
- **LibreTime / Rivendell / OpenBroadcaster (núcleo):** son sistemas de automatización de **radio** (audio). No hay decode de video en su flujo normal, así que la pregunta de hwaccel no aplica de la misma forma. Fuentes: [liquidsoap.info](https://www.liquidsoap.info/) (motor de LibreTime), [rivendellaudio.org](https://www.rivendellaudio.org/), [openbroadcaster.com](https://www.openbroadcaster.com/software/radio-automation-software/).
- **CasparCG:** el decode acelerado por GPU se discute en el foro de la comunidad como algo "menos directo" de lograr que el encode ([forum, GPU decoding/CUDA](https://casparcgforum.org/t/gpu-decoding-cuda/5424)). El encode sí tiene un pedido explícito y documentado de la comunidad para añadir NVENC en Linux, justificado en el issue como forma de "ayudar a justificar el costo de comprar una GPU dedicada" — es decir, la motivación documentada es económica/de aprovechamiento de hardware ya comprado para gráficos, no una ganancia de rendimiento medida. [CasparCG/server#1282](https://github.com/CasparCG/server/issues/1282).
- **OBS Studio:** al revés que los demás — su documentación oficial promueve el **encode** por hardware (NVENC/AMF/QSV/VideoToolbox) como la opción recomendada porque libera al CPU y permite "texture-based encoding" (GPU a GPU, sin roundtrip por CPU). El decode acelerado existe para ciertas fuentes pero no es el foco de la documentación. [obsproject.com/kb/hardware-encoding](https://obsproject.com/kb/hardware-encoding).
- **MistServer:** soporta transcodificación vía su proceso `MistProcAV` (basado en libav/ffmpeg); el soporte de NVENC fue pedido explícitamente por la comunidad ([DDVTECH/mistserver#171](https://github.com/DDVTECH/mistserver/issues/171)) y las notas de versión mencionan mejoras de soporte Nvidia. [docs.mistserver.org/mistserver/processes/mistprocav](https://docs.mistserver.org/mistserver/processes/mistprocav/).
- **Nageru:** caso distinto — no decodifica archivos, ingiere SDI/USB3 en crudo (Blackmagic DeckLink/bmusb) y hace casi todo el procesamiento de píxeles en GPU vía OpenGL. Para el H.264 de salida **requiere** Quick Sync (QSV) o un H.264 por hardware vía VA-API — el proyecto lo documenta como una dependencia, no como una opción: *"Nageru requires an Intel processor with Intel Quick Sync, or otherwise some hardware H.264 encoder exposed through VA-API"*. Fuente: [nageru.sesse.net](https://nageru.sesse.net/).

## 4. Comerciales

- **Cinegy:** es el más explícito de todos en usar GPU para encode. Su documentación dice literalmente que Cinegy Air "leverages NVIDIA's hardware-based H.264 and H.265 (HEVC) encoding engines instead of using CPU", y que su formato propio Daniel2 requiere GPU NVIDIA con Compute Capability 3.5+. Las líneas "PRO"/Ultra HD recomiendan Quadro/Turing/Ampere/Ada. Fuentes: [Cinegy Player PRO 2 System Recommendations (PDF)](https://open.cinegy.com/products/player-pro/2/system-recommendations/Cinegy-Player-PRO-2-System-Recommendations.pdf), [Cinegy and NVIDIA](https://home.cinegy.com/technology-partners/cinegy-and-nvidia/), [NVIDIA Developer Forums — Cinegy interlace H.264](https://forums.developer.nvidia.com/t/cinegy-unlocks-nvidia-h-264-interlace-encoding-on-nvidia-turing-ampere-and-ada-gpus/317601).
- **PlayBox Neo:** su material de producto menciona "Intel CPU/GPU hardware acceleration" y servidores "GPU acceleration ready" como opción de configuración de hardware, sin especificar si es decode, encode o ambos. Fuente: [playboxtechnology.com/2019/10/mega-upgrades-for-playbox-neo-range](https://playboxtechnology.com/2019/10/mega-upgrades-for-playbox-neo-range/).
- **Dinesat / Hardata:** no encontré documentación pública sobre requisitos de GPU ni uso de hwaccel — es, ante todo, un automatizador de radio.
- **VirtualPowerVideo:** no encontré el producto en las búsquedas realizadas (puede tratarse de un nombre distinto, un producto discontinuado, o documentación no indexada por buscadores). No hay información pública que reportar.
- **Imagine Communications (Versio/Nexio) y Grass Valley (ICE/Playout X):** su documentación pública de producto habla de arquitectura (baseband/SDI, IP, cloud-nativo, virtualización) pero no encontré menciones específicas de GPU/hwaccel para decode o encode. Fuentes: [imaginecommunications.com/make-tv/products/playout-and-channel-origination](https://imaginecommunications.com/make-tv/products/playout-and-channel-origination/), [grassvalley.com/products/ampp/playout-x](https://www.grassvalley.com/products/ampp/playout-x/).

## 5. Lo que hacen las estaciones de verdad: ¿ingerir una vez, copiar los bits al aire?

Esta es la pregunta que más importa, y la evidencia pública apunta en una sola dirección.

**BroadStream OASYS** documenta explícitamente evitar el transcode: su módulo Media Exchange importa contenido de terceros "skipping the transcoding process", y el sistema reproduce archivos `.ts` en su formato nativo para "reducir el tiempo de preparación manual". Fuente: [broadstream.com/products/oasys](https://broadstream.com/products/oasys/).

**Ross Video / Techex** documentan el concepto de "compressed-domain switching" — cambiar entre streams de transporte AVC/HEVC **sin decodificar ni recodificar**, operando directo en el dominio comprimido — como alternativa consciente al decode-switch-reencode. Las frases textuales de su material de producto:

> "far cheaper than GPU-bound decode and re-encode" [...] "lower latency than decode, switch and encode" [...] "full decode, baseband-switch, re-encode approach adds cost, latency and degrades quality every time it's repeated" [...] "full video-quality transparency: no re-encoding and no compression quality loss".

Fuente: [techex.tv/solutions/playout-protection](https://techex.tv/solutions/playout-protection).

Esto es consistente con la práctica de post-producción y contribución (fuera del broadcast lineal pero con la misma lógica): "smart rendering" en herramientas de edición evita re-codificar exportando por copia de contenedor cuando el códec/wrapper lo permite, precisamente porque cada pasada de decode+encode cuesta calidad y tiempo. Fuente: [Adobe — Smart rendering supported formats in Premiere](https://helpx.adobe.com/premiere-pro/using/smart-rendering.html).

No encontré, en esta investigación, un documento único de "la industria" que declare formalmente "la aceleración por hardware es la respuesta equivocada" con esas palabras — esa frase es una síntesis mía de la evidencia, no una cita textual de nadie. Lo que sí hay, documentado por al menos dos fuentes independientes (BroadStream y Ross/Techex), es la práctica concreta de **minimizar cuántas veces se decodifica y recodifica una señal**, prefiriendo transcodificar una vez al ingerir (sin restricción de reloj) y mover bits comprimidos en el resto de la cadena.

---

## Fuentes consultadas (con URL)

- [ffmpeg.org/ffmpeg.html](https://ffmpeg.org/ffmpeg.html) — documentación oficial de `-hwaccel`, `-hwaccel_output_format`, `-hwaccel_device`.
- [wiki.videolan.org/VLC_GPU_Decoding](https://wiki.videolan.org/VLC_GPU_Decoding) — VideoLAN Wiki, decode por hardware apagado por defecto.
- [docs.videolan.me/vlc-user/desktop/3.0/en/advanced/other/transcoding_with_ffmpeg_amf_codecs.html](https://docs.videolan.me/vlc-user/desktop/3.0/en/advanced/other/transcoding_with_ffmpeg_amf_codecs.html) — cómo forzar encoders AMF en VLC.
- [github.com/GPUOpen-LibrariesAndSDKs/AMF/wiki/VLC-and-AMF](https://github.com/GPUOpen-LibrariesAndSDKs/AMF/wiki/VLC-and-AMF)
- [github.com/ffplayout/ffplayout/discussions/681](https://github.com/ffplayout/ffplayout/discussions/681) — ejemplo de comunidad usando `-hwaccel cuvid`.
- [liquidsoap.info](https://www.liquidsoap.info/) — motor de LibreTime, audio/streaming.
- [rivendellaudio.org](https://www.rivendellaudio.org/) — Rivendell, automatización de radio.
- [openbroadcaster.com/software/radio-automation-software](https://www.openbroadcaster.com/software/radio-automation-software/) — OpenBroadcaster es automatización de radio.
- [github.com/CasparCG/server/issues/1282](https://github.com/CasparCG/server/issues/1282) — pedido de NVENC en Linux, justificación económica.
- [casparcgforum.org/t/gpu-decoding-cuda/5424](https://casparcgforum.org/t/gpu-decoding-cuda/5424) — decode GPU discutido como más complicado que encode.
- [obsproject.com/kb/hardware-encoding](https://obsproject.com/kb/hardware-encoding) — OBS recomienda encode por hardware.
- [docs.mistserver.org/mistserver/processes/mistprocav](https://docs.mistserver.org/mistserver/processes/mistprocav/) — MistProcAV (libav/ffmpeg).
- [github.com/DDVTECH/mistserver/issues/171](https://github.com/DDVTECH/mistserver/issues/171) — pedido de NVENC en MistServer.
- [nageru.sesse.net](https://nageru.sesse.net/) — Nageru requiere Quick Sync/VAAPI para H.264 de salida.
- [open.cinegy.com/products/player-pro/2/system-recommendations/Cinegy-Player-PRO-2-System-Recommendations.pdf](https://open.cinegy.com/products/player-pro/2/system-recommendations/Cinegy-Player-PRO-2-System-Recommendations.pdf)
- [home.cinegy.com/technology-partners/cinegy-and-nvidia](https://home.cinegy.com/technology-partners/cinegy-and-nvidia/)
- [forums.developer.nvidia.com/t/cinegy-unlocks-nvidia-h-264-interlace-encoding-on-nvidia-turing-ampere-and-ada-gpus/317601](https://forums.developer.nvidia.com/t/cinegy-unlocks-nvidia-h-264-interlace-encoding-on-nvidia-turing-ampere-and-ada-gpus/317601)
- [playboxtechnology.com/2019/10/mega-upgrades-for-playbox-neo-range](https://playboxtechnology.com/2019/10/mega-upgrades-for-playbox-neo-range/)
- [imaginecommunications.com/make-tv/products/playout-and-channel-origination](https://imaginecommunications.com/make-tv/products/playout-and-channel-origination/)
- [grassvalley.com/products/ampp/playout-x](https://www.grassvalley.com/products/ampp/playout-x/)
- [broadstream.com/products/oasys](https://broadstream.com/products/oasys/) — ingesta nativa sin transcodificar.
- [techex.tv/solutions/playout-protection](https://techex.tv/solutions/playout-protection) — "compressed-domain switching", cita clave sobre costo/latencia/calidad del decode-reencode repetido.
- [helpx.adobe.com/premiere-pro/using/smart-rendering.html](https://helpx.adobe.com/premiere-pro/using/smart-rendering.html) — smart rendering, analogía de post-producción.
- [forums.developer.nvidia.com/t/ffmpeg-mixing-cpu-and-gpu-processing/199899](https://forums.developer.nvidia.com/t/ffmpeg-mixing-cpu-and-gpu-processing/199899) — mecánica técnica de por qué el hwaccel de decode no ayuda si el resto del pipeline es CPU (foro, no documento oficial).

No encontré información pública sobre: requisitos de hardware/GPU de Dinesat; el producto "VirtualPowerVideo"; menciones específicas de GPU en la documentación pública de Imagine Communications o Grass Valley; una declaración formal de algún proyecto de playout libre rechazando explícitamente el hwaccel de decode con una justificación documentada (lo que hay es ausencia de la característica, no un rechazo declarado).

---

## Recomendaciones para Antena787

### Lo que es evidencia (no opinión)

- El propio manual de ffmpeg advierte que decode-hwaccel no gana nada si de todas formas hay que bajar los cuadros a RAM para un encoder de software — que es exactamente la situación de Antena787 hoy (encoder mpeg2video/libx264 por software). [ffmpeg.org/ffmpeg.html](https://ffmpeg.org/ffmpeg.html).
- El propio cliente (VLC) que hoy recibe el multicast trae el decode por hardware apagado por defecto — no es una omisión rara del ecosistema, es la postura por defecto del proyecto de referencia. [wiki.videolan.org/VLC_GPU_Decoding](https://wiki.videolan.org/VLC_GPU_Decoding).
- Al menos dos fuentes de la industria de broadcast, independientes entre sí (BroadStream y Ross/Techex), documentan la misma estrategia: transcodificar una vez al ingerir, sin decodificar más en el resto de la cadena hacia el aire.

### Lo que es mi opinión, informada por lo anterior

- Para el caso puntual de Antena787 —un decoder nuevo por cada clip, un encoder persistente por software a 4 Mb/s 720p59.94— añadir `-hwaccel` al lado del decode probablemente no rinda: el costo de inicializar contexto de hardware por cada apertura de clip, sumado al `hwdownload` obligatorio antes de llegar al encoder de software, es exactamente el escenario que ffmpeg mismo advierte que no conviene.
- Si en algún momento se evalúa acelerar por hardware, el orden de prioridad razonable sería: primero confirmar que el software decode/encode actual realmente no alcanza a hacer 720p59.94 en tiempo real en el hardware del canal (parece dudoso a esa tasa y resolución en un PC moderno); si el cuello de botella real es el ingest/transcode de archivos antes de que entren a playout —no el playout en sí—, ahí sí tiene sentido investigar `-hwaccel` porque no compite con el reloj del aire.
- La opción de acelerador que ya se añadió solo para encoding tiene más sentido que agregar hwaccel de decode, en la medida en que el encoder de salida sea el cuello de botella real medido (no supuesto) — pero eso habría que medirlo con el hardware real del canal antes de invertir más tiempo en esto.
- Vale la pena, en algún momento, medir con datos del canal real (uso de CPU durante emisión en el PC de Windows 10) si hoy existe siquiera un problema de rendimiento que resolver, antes de seguir explorando hwaccel en cualquiera de los dos lados.
