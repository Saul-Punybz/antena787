# Bitrate en ISDB-T/ISDB-Tb: quién lo decide (Japón y Suramérica)

Fecha: 2026-09-11
Alcance: investigación web, sin código ni cambios al repositorio.
Caso concreto: Antena787 hoy emite MPEG-2, 720p a 4800 kb/s de video (configurable) y audio MPEG capa II a 128 kb/s / 44100 Hz, por UDP multicast a un multiplexor. En el código, el audio está cableado a 192 kb/s por defecto (`internal/ingest/normalize.go:39` y `:279`, `AudioBitrate string // "192k" por defecto"`). La pregunta es si eso tiene sentido fuera de ATSC, en el mundo ISDB-T/ISDB-Tb.

---

## Resumen de la respuesta

**Ni el playout ni el multiplexor "deciden" el bitrate en el sentido de que la norma se lo imponga; el broadcaster (o quien configure el encoder) lo elige dentro de un techo que pone el perfil/nivel del códec, y el multiplexor solo garantiza que ese número quepa en la capacidad de la capa jerárquica que le asignaron.** Ningún regulador de los que pude verificar (Japón/ARIB, Brasil/ANATEL vía ABNT, Argentina/ENACOM) fija un bitrate obligatorio de video o de audio para el servicio principal ("full-seg"). Sí hay un piso normativo, pero solo para el servicio móvil "one-seg" en Brasil (video ≥ 64 kb/s). El detalle está abajo, con cita de cada afirmación.

---

## 1. Japón, ISDB-T: el canal de 6 MHz y los 13 segmentos

Fuente primaria: **ARIB STD-B31 v1.6-E2** (traducción oficial en inglés), Association of Radio Industries and Businesses.
https://www.arib.or.jp/english/html/overview/doc/6-STD-B31v1_6-E2.pdf

- El canal de 6 MHz se divide en **13 segmentos OFDM**, cada uno de ancho igual a 1/14 del ancho del canal (ARIB STD-B31, §1.4.1, definición de "OFDM segment"; y §2, "13 successive OFDM blocks").
- La **recepción parcial ("one-seg")** usa normalmente **1 solo segmento** —el del centro de la banda, "segment No. 0"— reservado para receptores móviles/portátiles. Cuando ese segmento se usa para recepción parcial, la bandera correspondiente debe ponerse en 1 (ARIB STD-B31, Tabla 3-27 "Partial-Reception Flag", §3.15.6.4). Es decir: **12 segmentos para el servicio fijo (SD/HD) y 1 para one-seg**, que es la configuración estándar de la industria.
- **Bitrate por segmento** (Tabla 3-3, "Data Rate of a Single Segment"): con QPSK va de 280,85 kb/s (código convolucional 1/2, guarda 1/4) hasta 595,76 kb/s (código 7/8, guarda 1/32); con 16QAM va de 561,71 kb/s hasta ~1.156 kb/s según la combinación de código y guarda. El one-seg típico (QPSK, código 2/3, guarda 1/8) da **416,08 kb/s**, que es la cifra que se cita habitualmente como "416 kbps de one-seg" en la industria.
- **Capacidad total del canal**: con 64QAM y código 7/8 (todo el canal en un solo modo, sin jerarquía), ARIB STD-B31 dice textualmente: *"it is possible to achieve a transmission capacity of 20 Mbps or more per 6 MHz"* (§3.1). Otras fuentes secundarias (no ARIB) hablan de un rango de 3,651 Mb/s a 23,234 Mb/s útiles según modulación/código/guarda — no pude verificar esa cifra exacta contra el texto de ARIB, así que la marco como dato de fuente secundaria, no normativo.

### El video en Japón: MPEG-2, sin bitrate fijo

ARIB STD-B31, §3.4 ("Video coding scheme"), es la frase clave de todo este informe:

> *"Selection of each coding parameter should be made by the judgment of each broadcasting provider considering picture quality, required bit rate and reception quality, etc."*
> — ARIB STD-B31 v1.6-E2, §3.4, p. 81-82.

El techo no es un número fijo sino el **perfil/nivel de MPEG-2**: la norma dice que para el nivel más bajo (MP@LL) el bitrate de referencia es 4 Mb/s, para MP@ML es 15 Mb/s, y para los niveles más altos (MP@H14L, MP@HL) el techo es *"the maximum capacity that can be transmitted by terrestrial digital broadcasting"* — o sea, lo que quepa en la capa jerárquica asignada (ARIB STD-B31, nota bajo Tablas 3-6/3-7, §3.4.1). Ningún número de video está mandado; solo acotado.

### El audio en Japón: MPEG-2 AAC, no HE-AAC

Japón terminó de fijar ISDB antes de que se estandarizara MPEG-4, así que usa **MPEG-2 AAC**, no HE-AAC v2 (que sí usan casi todos los demás sistemas de radiodifusión del mundo, incluyendo el brasileño). Fuente: Fraunhofer/EBU, "MPEG-4 HE-AAC v2 – audio coding for today's media world", EBU Technical Review 305 (Moser):
https://tech.ebu.ch/docs/techreview/trev_305-moser.pdf
y resumen técnico en The Broadcast Bridge: https://www.thebroadcastbridge.com/content/entry/21822/standards-advanced-audio-coding-aac
No encontré, en fuente ARIB de acceso público, una tabla equivalente a la de Brasil (abajo) con los bitrates de audio permitidos en detalle — el documento con esos parámetros es ARIB STD-B32 Parte 2, y solo tuve acceso a las versiones "outline" en inglés, que no traen esa tabla. **No hay información pública gratuita que haya podido verificar sobre un bitrate mínimo/máximo de audio obligatorio en ARIB STD-B32.**

---

## 2. ISDB-Tb en Suramérica: qué cambia frente a Japón

- La **capa de transmisión (OFDM, 13 segmentos, modulación, FEC)** es la misma que en Japón: la norma brasileña ABNT NBR 15601 (transmisión) es, según su propia página descriptiva, *"identical to the Japanese ISDB-T system and ITU-R System C"* — https://en.wikipedia.org/wiki/ABNT_NBR_15601
- Lo que cambia es la **capa de codificación**: Brasil (y por extensión toda Latinoamérica que adoptó el "ISDB-T International" / ISDB-Tb) usa **H.264/MPEG-4 AVC para video** en vez de MPEG-2, y **HE-AAC v2** para audio en vez de MPEG-2 AAC. Fuente: ABNT NBR 15602 (partes 1 y 2, ver abajo) y Wikipedia ABNT NBR 15602: https://en.wikipedia.org/wiki/ABNT_NBR_15602
- También se agrega el middleware de interactividad **Ginga**, que no es relevante para el bitrate.
- **Reguladores**: en Brasil, ANATEL certifica equipos de transmisión contra las normas ABNT (p. ej. Ato nº 942/2018, sobre requisitos de evaluación de conformidad de transmisores/retransmisores ISDB-Tb — https://informacoes.anatel.gov.br/legislacao/atos-de-certificacao-de-produtos/2018/1191-ato-942), pero **no encontré una resolución de ANATEL que fije un bitrate obligatorio**; ANATEL remite a la norma técnica ABNT, y esa norma es la que revisé directamente (sección 3 de este informe).
- En Argentina, ENACOM aprobó la Norma Técnica ISDB-T (basada en la misma línea Brasil/Japón) por **Resolución 7/2013** del entonces Ministerio de Planificación Federal, con las especificaciones técnicas en su Anexo I: https://www.argentina.gob.ar/normativa/nacional/norma-218527/texto — **el Anexo I con el detalle técnico no está publicado en una página que pude leer** (el Boletín Oficial lo aloja aparte); el cuerpo de la resolución solo dice que aprueba la norma "conforme las especificaciones técnicas contenidas en el ANEXO I", sin bitrates en el texto que sí pude leer. No puedo confirmar ni descartar, con fuente pública verificada, si ese anexo fija algún piso o techo de bitrate.
- El resto de países ISDB-Tb de la lista (Perú, Colombia, Venezuela, Ecuador, Bolivia, Paraguay, Uruguay) típicamente **adoptan por referencia las normas ABNT brasileñas** en vez de escribir las suyas. Ejemplo verificado: Perú, Resolución Ministerial N.° 645-2009-MTC/03, que fija especificaciones mínimas de receptores tomando como referencia ABNT NBR 15604 (Brasil) y ARIB STD-B21 (Japón) — https://www.gob.pe/institucion/mtc/normas-legales/292605-645-2009-mtc-03 — no fija bitrate de codificación propio. **No encontré información pública sobre normas de bitrate propias en Colombia, Venezuela, Ecuador, Bolivia, Paraguay o Uruguay**; dado que todos usan la misma norma base, es razonable —pero no verificado por mí— asumir que heredan el mismo esquema sin bitrate fijo.
- Costa Rica adoptó ISDB-Tb por Decreto Ejecutivo 36009-MP-MINAET (29 abr 2010) y tiene el Reglamento Técnico RTCR 456:2011 para receptores y antenas: https://vlex.co.cr/vid/cnico-2011-sticas-ba-sicas-costa-rica-485016474 — es un reglamento de **receptores**, no de codificación del lado del transmisor; no vi mención de bitrate de codificación.

---

## 3. ¿Hay bitrates mínimos o máximos obligatorios por norma? (Brasil, verificado en el texto de la norma)

Esto es lo más importante para el diseño del software, así que lo verifiqué contra el **texto original** de ABNT NBR 15602 (partes 1 y 2), obtenido vía Wayback Machine desde el repositorio público de normas SBTVD de PUC-Rio (el enlace original está caído; usé la copia archivada):
- Parte 1 (video): http://web.archive.org/web/2020/http://www.telemidia.puc-rio.br/~rafaeldiniz/public_files/normas/SBTVD/pt_BR/ABNTNBR15602-1_2007Ed1.pdf
- Parte 2 (audio): http://web.archive.org/web/2020/http://www.telemidia.puc-rio.br/~rafaeldiniz/public_files/normas/SBTVD/pt_BR/ABNTNBR15602-2_2007Ed1.pdf

### Video, servicio principal ("full-seg", que es el que le interesa a Antena787)

La norma **no fija un número de bitrate**. Dice textualmente que la codificación debe encuadrar en el perfil High, "ficando a escolha a critério da fonte geradora" (la elección queda a criterio de la estación emisora), acotada al **nivel 4.0 de H.264 o cualquier nivel inferior** (ABNT NBR 15602-1:2007, §7, líneas correspondientes al texto: *"A codificação deve obrigatoriamente ser compatível com as restrições impostas pelo perfil high [...] ficando a escolha a critério da fonte geradora. O fluxo de bits pode se enquadrar nas restrições impostas pelo nível 4.0, ou qualquer nível inferior."*). El techo real es el que impone la Tabla 15 de la norma (traspuesta de H.264/ITU-T Rec. H.264:2005, Anexo A): para el **nivel 4.0**, el `MaxBR` es 20.000–25.000 (en unidades de 1.000 bit/s × factor de perfil), que para High Profile equivale a un techo real del orden de **25–31 Mb/s**. Es un techo, no un piso ni un valor recomendado.

### Video, servicio móvil "one-seg" — aquí sí hay un piso obligatorio

Distinto del caso anterior: ABNT NBR 15602-1:2007, §8.3.2 ("Restrições nos parâmetros de codificação de vídeo para serviços 1-seg"), Tabla 21, fija:

> **Taxa de bits: "64 kbps até a máxima taxa de bits permitida pelo perfil@nível especificado na ITU-T Recommendation H.264:2005"** (nivel tope 1.3), resolución SQVGA/QVGA/CIF, tasas de cuadro 5/10/12/15/24/30 Hz.

O sea: **para el one-seg brasileño sí hay un piso normativo (64 kb/s) y un techo (nivel 1.3 de H.264)**, pero para el servicio fijo/full-seg —el que le importa a un cliente de Antena787 sin servicio móvil— no hay ni piso ni techo fijo, solo el nivel 4.0 como techo.

### Audio, servicio principal ("full-seg")

Igual de claro. ABNT NBR 15602-2:2007, §9.1.2, Tabla 5 ("Principais parâmetros do sistema de codificação de áudio – Serviço full-seg"):

> **"Taxa máxima de bits permitida: Conforme ISO/IEC 14496-3."**
> **"Os sinais podem ser codificados em qualquer taxa suportada no perfil e nível selecionado. Ao mesmo tempo o sinal multicanal pode empregar qualquer freqüência de amostragem do perfil."**

Es decir: **no hay bitrate de audio obligatorio para el servicio fijo**. Los perfiles permitidos son LC-AAC@L2, LC-AAC@L4, HE-AAC v1@L2 (estéreo) y HE-AAC v1@L4 (multicanal); dentro de cada perfil/nivel, "cualquier tasa soportada" es válida.

### Conclusión de este punto

**Ningún país que pude verificar impone un bitrate de video/audio obligatorio para el servicio de recepción fija.** El único piso normativo que encontré en toda la investigación es el de 64 kb/s de video para el **one-seg móvil** en Brasil — y eso no aplica a un cliente de Antena787 que no va a emitir one-seg. Esto es un hallazgo importante para el diseño: **el software no necesita, y no debería, intentar cumplir una tabla de bitrates obligatorios por país** porque, hasta donde pude verificar, esa tabla no existe fuera del caso one-seg.

---

## 4. El audio: AAC, y la pregunta de 44.1 kHz vs 48 kHz

Otro hallazgo directo del texto de la norma, ABNT NBR 15602-2:2007, §8 ("Formato de entrada de áudio"):

> **"freqüência de amostragem do sinal de áudio: 32 kHz, 44,1 kHz ou 48 kHz"**

**48 kHz NO es obligatorio. 44,1 kHz está explícitamente aceptado por la norma**, junto con 32 kHz. Esto es exactamente lo que hoy sale del cliente de Antena787 (44100 Hz), y es compatible sin necesidad de resamplear.

Sobre qué perfil de AAC es obligatorio: la misma norma (§10, líneas ~981-983) dice:

> *"A versão 2 do MPEG-4 AAC-HE deve obrigatoriamente ser adotada para transmissão para dispositivos portáteis e também é obrigatória para dispositivos fixos e móveis, se estes forem recuperar o serviço one-seg."*

O sea: **HE-AAC v2 es obligatorio solo si el receptor va a recuperar el servicio one-seg** (portátil). Para el servicio fijo puro (full-seg, sin one-seg), la Tabla 5 permite LC-AAC o HE-AAC v1, a elección del broadcaster.

Sobre bitrates de audio "de referencia" (no obligatorios): había visto, en un resumen automático de búsqueda, una supuesta "Tabla 9" con rangos de 96-128 kb/s (estéreo estándar), 128-256 kb/s (estéreo alta calidad) y 288-384 kb/s (multicanal). **No pude confirmar esa tabla en el texto de la edición 2007 que sí verifiqué** (esa edición solo llega hasta la Tabla 8, y es sobre one-seg). Puede existir en una edición posterior de la norma (hay al menos una revisión de 2016 según metadatos que vi de pasada) que no pude conseguir en acceso público. **No la incluyo como cifra confirmada** porque no cumplo mi propia regla de "sin fuente no entra" — la marco aquí solo para que quede registrado que existe la pista, sin tratarla como dato verificado.

Comparación de contexto general (no específica de ISDB, para calibrar qué es "habitual" en HE-AAC): 48-64 kb/s para estéreo y ~160 kb/s para 5.1, según el propio material educativo de Fraunhofer/EBU (fuente citada arriba). Eso es orientativo, no normativo, y no viene de una norma ISDB.

---

## 5. ¿Se compra el ancho de banda? Cómo entra una estación chica al múltiplex

Aquí el patrón **no es el mismo en Brasil que en Argentina**, y es relevante para entender qué tipo de "cliente típico" tendría Antena787 en cada país.

### Brasil: cada emisora tiene su propio canal de 6 MHz, no hay múltiplex compartido entre estaciones independientes

Fuente: MCTI (hoy MCTIC), página oficial sobre retransmisión de televisión:
https://antigo.mctic.gov.br/mctic/opencms/comunicacao/SERAD/radiofusao/detalhe_tema/retransmissaoDeTelevisao.html

- Cada emisora ("geradora") recibe **su propio canal de radiofrecuencia de 6 MHz** por cada canal analógico que tenía.
- Una retransmisora (RTVD) también recibe **su propio canal de 6 MHz** para retransmitir la señal de una única generadora — **"cada estación retransmisora solo puede retransmitir la señal de una única estación generadora"**, y hoy existen unas 11.000 retransmisoras en Brasil según la misma fuente.
- Esto significa que, en el modelo brasileño clásico, **no existe un múltiplex comercial compartido** donde varias estaciones independientes "compran" espacio dentro del transporte de un tercero, como sí ocurre en algunos esquemas de DVB-T en Europa. Cada quien tiene su propio flujo de transporte de 13 segmentos. Una estación chica que actúa como generadora propia arma su propio multiplex (su propio TS) con su propia señal de video/audio/datos.

### Argentina: hay una plataforma estatal de multiplexación compartida (ARSAT)

Fuente: Decreto 835/2011 (Poder Ejecutivo Nacional), texto oficial:
https://www.argentina.gob.ar/normativa/nacional/decreto-835-2011-183617

- El Decreto 835/2011 autorizó a **ARSAT** (Empresa Argentina de Soluciones Satelitales S.A.) a prestar servicios de **uso de infraestructura, multiplexación y transmisión** para la Televisión Digital Terrestre, a través de la "Plataforma Nacional de Televisión Digital Terrestre", a los titulares de licencias y autorizaciones de TDT.
- La multiplexación bajo ISDB-T es, según se describe en esa misma línea normativa, la operación que permite que **varios servicios de comunicación audiovisual compartan el mismo canal de frecuencia** en VHF o UHF.
- Esto sí se parece más a "comprar/asignarse ancho de banda dentro de un múltiplex compartido": una estación chica en Argentina puede no operar transmisor propio y en cambio entregar su señal a la plataforma de ARSAT, que la multiplexa junto con otras señales del mismo sitio de transmisión.
- No encontré, en fuente pública, el detalle comercial exacto (tarifa, contrato tipo, cómo se reparte la capacidad entre licenciatarios) de ese servicio de multiplexación de ARSAT — solo el marco legal que lo autoriza.

### Conclusión de este punto para Antena787

El caso de uso "playout chico que entrega UDP multicast a un multiplexor" descrito por el cliente (Puerto Rico, ATSC hoy) **encaja mejor con el modelo argentino** (entregar señal a un multiplexor externo que la combina con otras) que con el modelo brasileño clásico (cada quien con su propio TS de 13 segmentos). Si Antena787 se despliega en Brasil, lo más probable es que el cliente sea una generadora con su propio multiplex completo — ahí Antena787 podría terminar generando el TS completo, no solo un ES para que otro lo multiplexe. Si se despliega en Argentina, el escenario de "entregar señal a un mux operado por terceros (ARSAT o un operador privado)" es el documentado.

---

## 6. Quién pone el número en la práctica: encoder de playout vs. multiplexor

Con toda la evidencia anterior, la respuesta es:

- El **multiplexor / plan de transmisión** fija la **capacidad disponible**: cuántos segmentos (de los 13) se le asignan a la capa jerárquica del servicio, y con qué modulación y tasa de código convolucional (eso se señaliza en TMCC: ver ARIB STD-B31 §3.15.6.5 y siguientes, Tablas 3-27 y 3-28). Ese conjunto de parámetros —segmentos × modulación × FEC— es lo que da el techo físico en bits por segundo de esa capa (ejemplo real de la norma: *"Hierarchical layer A: DQPSK, convolutional-coding rate of 1/2, 5 segments"*, ARIB STD-B31 §3.15, ejemplo de configuración).
- El **encoder de video/audio (el playout)** decide, dentro de ese techo, qué bitrate real usar para cada elemento — y la norma, tanto japonesa como brasileña, delega esa decisión explícitamente al broadcaster: *"Selection of each coding parameter should be made by the judgment of each broadcasting provider considering picture quality, required bit rate and reception quality"* (ARIB STD-B31 §3.4) y *"ficando a escolha a critério da fonte geradora"* (ABNT NBR 15602-1, §7).
- En la práctica de planta, esto normalmente lo termina fijando **el ingeniero de transmisión al configurar el multiplexor/remultiplexor** (porque es ahí donde se sabe cuánta capacidad total hay disponible y cómo se reparte entre los servicios de la capa), y el **playout simplemente tiene que respetar ese número** que le informan — no al revés. El multiplexor no "calcula" un bitrate óptimo por sí solo; alguien se lo tiene que decir, típicamente basado en la capacidad de la capa jerárquica contratada/asignada.

---

## Qué cambia una decisión de diseño

1. **No hay tabla de bitrates obligatorios por país que el software deba conocer**, salvo el caso one-seg de Brasil (64 kb/s de video, que no aplica a un cliente sin servicio móvil). Construir una tabla de "bitrates legales por país" sería trabajo sin base normativa real.
2. **44,1 kHz es válido según la norma brasileña (y por herencia, ISDB-Tb en general)** — no hay que forzar 48 kHz para audio. El cableado actual de Antena787 a 192 kb/s de audio no tiene respaldo normativo específico; es una elección de ingeniería, no un requisito de ISDB.
3. El bitrate de video/audio en ISDB-T/ISDB-Tb **lo fija quien opera la planta** (broadcaster/ingeniero de transmisión), acotado por el perfil/nivel del códec — no el multiplexor por sí solo, y no el regulador.
4. El escenario de "playout entrega ES a un multiplexor de terceros" (como hoy en Puerto Rico) tiene precedente documentado en **Argentina (ARSAT)**, pero no en el modelo clásico brasileño, donde cada generadora arma su propio TS completo.

## Recomendación

**No es asunto nuestro fijar ni "adivinar" el bitrate por país — no controlarlo, sino exponerlo como parámetro configurable (como ya está el video) y, cuando mucho, recomendar por defecto lo que ya se usa hoy (audio 128–192 kb/s, 44,1 o 48 kHz), documentando que en Brasil/Argentina eso es válido y que la cifra real la debe confirmar el cliente con quien opera su multiplexor.**
