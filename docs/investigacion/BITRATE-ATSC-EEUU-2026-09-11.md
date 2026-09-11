# Bitrate en una cadena ATSC de Estados Unidos: quién decide y qué le toca a Antena787

Fecha: 11 de septiembre de 2026
Caso: Caribbean Advantage TV, estación Class A en Puerto Rico, ATSC 1.0, multiplexor Technalogix TP1000.

---

## Resumen ejecutivo (la respuesta a la pregunta única)

**El bitrate en una cadena ATSC no lo decide el software de playout ni un contrato de ancho de banda: lo decide la física del canal de 6 MHz que la FCC ya licenció, y dentro de ese techo, lo reparte el operador según su propio criterio editorial — sin mínimo ni máximo normativo de la FCC o del ATSC para el video.** La única pieza que sí tiene un piso normativo duro es el audio: la FCC, por referencia obligatoria a los estándares ATSC, exige AC-3 (Dolby Digital) muestreado a 48 kHz — no MPEG capa II, no 44.1 kHz. Ver hallazgo crítico en la sección 4.

---

## 1. El techo del canal ATSC de 6 MHz

- **ATSC 1.0 (8-VSB):** la tasa de carga útil nominal es **19.39 Mb/s** (a veces citada como 19.4 o 19.38 Mb/s) dentro de un canal de 6 MHz. Esto viene definido en el estándar **ATSC A/53** (ATSC Digital Television Standard, Parts 1-6, 2007).
  Fuente: https://www.atsc.org/wp-content/uploads/2021/04/a_53-Part-1-6-2007.pdf
- Ese número (19.39 Mb/s) es la tasa **total de la señal transmitida** (el multiplex/transport stream completo, no solo el video). Es el mismo dato que confirma la ingeniería de planta: el enlace estudio-transmisor (STL) típicamente lleva un MPEG-2 Transport Stream de ~19.39 Mb/s por DVB-ASI o SMPTE-310M.
  Fuente: https://www.tvtechnology.com/miscellaneous/atsc-transport-streams
- **Class A y LPTV (baja potencia) usan el mismo techo de 19.38-19.39 Mb/s.** La FCC no les reduce la capacidad del canal; lo que sí aclara es que **no tienen que llenarlo**: el servicio de video gratuito solo necesita ser "comparable en calidad técnica a NTSC", no ocupar el 100% de la capacidad.
  Fuente: FCC 04-220, https://docs.fcc.gov/public/attachments/FCC-04-220A1.pdf
- Legalmente, Class A cae bajo el mismo estándar de transmisión que la TV de potencia plena: la Subparte J (Class A, §§73.6000-73.6029) exige cumplir con **§73.682** — el mismo artículo que rige a las estaciones full-power.
  Fuente: eCFR, https://www.ecfr.gov/current/title-47/chapter-I/subchapter-C/part-73/subpart-J ; y https://www.law.cornell.edu/cfr/text/47/73.682
- **ATSC 3.0 (OFDM):** no tiene un techo fijo único — depende de la modulación y el coding rate elegidos. El rango citado va de ~1 Mb/s (máxima robustez) hasta un máximo teórico de **57 Mb/s** en un canal de 6 MHz; en despliegues reales comparables en cobertura a ATSC 1.0 se habla de cifras cercanas a 26-28 Mb/s. No hay una sola fuente primaria del ATSC con "el" número — es una tabla de combinaciones modulación/FEC.
  Fuente (estimado 26 Mb/s comparable en cobertura): https://www.tvtechnology.com/atsc3/increasing-channel-bandwidth-to-broadcast-8k ; (rango 1-57 Mb/s con 64 PLPs): resultados de búsqueda sobre la especificación de capa física ATSC 3.0, ver también el paper técnico de DekTec: https://www.atsc.org/wp-content/uploads/2020/05/f-36-26-12666874_ZItpm7tg_ATSC3p0Paper.pdf

**No aplica hoy:** Caribbean Advantage TV emite ATSC 1.0, así que el techo real de su cadena es 19.39 Mb/s, punto.

---

## 2. ¿Se compra el ancho de banda? Confirmado: no se contrata por tramos

Se confirma la premisa. Para una estación al aire (no cable, no streaming), el bitrate **no es un servicio que se contrata en tramos** como un enlace de internet. Viene dado, de una sola vez, por:

1. La licencia de la FCC que le asigna a la estación un canal de 6 MHz específico (VHF o UHF).
2. La física de la modulación 8-VSB de ese canal, fijada por el estándar ATSC A/53 en 19.39 Mb/s.

No hay un "plan" de más o menos Mb/s que un operador de Class A pueda comprar — a diferencia de un enlace de cable (donde 47 CFR y las prácticas de cable sí permiten 38.8 Mb/s en 6 MHz con QAM 256, otro mundo regulatorio). Todo lo que hay para repartir dentro de esos 19.39 Mb/s es el reparto **interno** entre video, audio y subcanales, y eso lo decide el operador, no un vendedor de ancho de banda.

Fuente (38.8 Mb/s en cable como contraste, y 19.4 Mb/s fijo al aire): resultados de búsqueda con cita a foros técnicos de ingeniería de broadcast (AVS Forum / RadioDiscussions), consistente con A/53. No encontré un documento único de la FCC que lo diga en esas palabras exactas ("no se contrata"); es una inferencia directa y sólida de que 47 CFR 73.682 fija el estándar de transmisión, no un mercado de capacidad.

---

## 3. ¿Hay mínimo o máximo obligatorio de bitrate de video o audio?

- **No hay un mínimo ni máximo de la FCC o del ATSC para el bitrate de video en sí.** El 47 CFR §73.682 (TV transmission standards) incorpora por referencia a ATSC A/52, A/53 Partes 1-4 y 6, y A/65C — pero el texto del reglamento **no fija cifras de Mb/s de video o audio**; delega toda la ingeniería fina a esos documentos del ATSC, y esos documentos tampoco imponen un piso o techo de bitrate de video — solo el techo agregado del canal (19.39 Mb/s) y las restricciones de perfil/nivel de MPEG-2 (Main Profile @ High Level) que aplica ese estándar.
  Fuente: eCFR 73.682, https://www.ecfr.gov/current/title-47/chapter-I/subchapter-C/part-73/subpart-E/section-73.682 ; texto vía Cornell LII: https://www.law.cornell.edu/cfr/text/47/73.682
- **Lo único con un requisito de calidad mínima** (no de Mb/s, sino de resultado visible) es la regla de servicio gratuito: el video ATSC de una estación Class A/LPTV debe ser "comparable en calidad técnica a NTSC" — una vara subjetiva de calidad de imagen, no una cifra de bitrate.
  Fuente: FCC 04-220, https://docs.fcc.gov/public/attachments/FCC-04-220A1.pdf
- **Conclusión normativa vs. habitual:** el reparto de Mb/s entre video/audio/subcanales es decisión **del operador** (lo habitual), no una regla de la FCC (lo normativo). La única regla normativa dura que si aparece es la del §3, audio.

---

## 4. El audio — hallazgo crítico para Caribbean Advantage TV

Esta es la sección que más le importa al caso concreto, porque el cliente hoy emite con **MPEG capa II a 44100 Hz**, y esa combinación **no cumple con lo que la FCC exige por referencia normativa**.

### 4.1 AC-3 es obligatorio, no opcional

- El 47 CFR §73.682(d) incorpora por referencia, entre otros, **ATSC A/52** (el estándar de compresión de audio digital AC-3/E-AC-3) como parte del "Digital broadcast television transmission standard" de obligatorio cumplimiento.
  Fuente: resumen textual de 73.682 vía Cornell LII (confirmando "ATSC A/52" en la lista de documentos incorporados por referencia): https://www.law.cornell.edu/cfr/text/47/73.682 ; eCFR: https://www.ecfr.gov/current/title-47/chapter-I/subchapter-C/part-73/subpart-E/section-73.682
- Class A cae bajo esta misma obligación porque la Subparte J la remite a §73.682.
  Fuente: eCFR Subparte J: https://www.ecfr.gov/current/title-47/chapter-I/subchapter-C/part-73/subpart-J
- **MPEG capa II no es parte del estándar ATSC de transmisión terrestre en Estados Unidos.** No encontré ningún documento de la FCC o del ATSC que autorice MPEG capa II como códec de audio de emisión ATSC — es el códec europeo/DVB, no el de A/52. Esto es "no hay información pública que lo permita", que es distinto de "está expresamente prohibido palabra por palabra"; pero como AC-3 es obligatorio por incorporación normativa, usar otro códec en el multiplex final de aire es, en la práctica, un incumplimiento técnico del estándar de transmisión.
  Fuente base del estándar AC-3: https://www.atsc.org/atsc-documents/a522012-digital-audio-compression-ac-3-e-ac-3-standard-12172012/

### 4.2 El muestreo: 48 kHz es obligatorio, 44.1 kHz no está permitido en el aire

- Según **ATSC A/53 Parte 5** (AC-3 Audio System): si se usan entradas digitales, la tasa de muestreo de entrada **debe ser 48 kHz**, o el codificador de audio debe tener conversores de tasa de muestreo que la lleven a 48 kHz. Si se usan entradas analógicas, los conversores A/D deben muestrear a 48 kHz.
  Fuente: ATSC A/53 Parte 5:2014, https://www.atsc.org/wp-content/uploads/2021/04/A53-Part-5-2014.pdf
- Aunque el códec AC-3 en sí soporta 32, 44.1 y 48 kHz como sintaxis, **el perfil que exige la transmisión terrestre ATSC restringe esto a 48 kHz únicamente**. 44.1 kHz es el estándar de audio de CD/música, no el de emisión de TV en Estados Unidos.
  Fuente: mismo documento A/53 Parte 5 arriba, y contraste general en https://en.wikipedia.org/wiki/48,000_Hz

### 4.3 Qué significa esto para el caso concreto

Caribbean Advantage TV emite hoy MPEG capa II a 44100 Hz desde VLC. **Ninguno de esos dos parámetros de audio cumple con lo que el ATSC/FCC exige para la señal final de aire.** Dos caminos posibles, y no es asunto de Antena787 decidir cuál — pero sí de señalarlo:

1. El Technalogix TP1000 (u otro elemento de la cadena) transcodifica el audio antes de salir al aire, convirtiéndolo a AC-3/48 kHz. Si es así, el incumplimiento de Antena787 nunca llega a la señal radiada y es irrelevante en la práctica — pero conviene confirmarlo con el fabricante/el ingeniero de planta, no asumirlo.
2. Si nadie transcodifica, la señal que sale por la antena hoy no es, técnicamente, audio ATSC conforme — independientemente de si el bitrate de audio es 128 o 192 kb/s. Esto es un asunto de cumplimiento de la estación licenciataria, más allá de lo que decida el software de playout.

No encontré documentación pública específica del TP1000 sobre si hace conversión de audio (su manual describe remultiplexado, remapeo de PID, PSIP e inserción, pero las fuentes públicas no detallan transcodificación de audio).
Fuente: manual TP1000, https://www.manualslib.com/manual/1357407/Technalogix-Tp1000.html

---

## 5. El reparto entre subcanales — cifras típicas (práctica de la industria, no norma)

No hay una regla de la FCC que fije cuánto le toca a cada subcanal — es decisión editorial del operador dentro del techo de 19.39 Mb/s. Cifras habituales citadas por ingenieros de broadcast:

- **HD principal en 1080i:** PBS ha usado aproximadamente 14.4-14.5 Mb/s para su canal principal 1080i.
- **HD principal en 720p:** ABC ha probado transmisión a 15 Mb/s para 720p.
- **SD (480i) por subcanal:** típicamente 3.5-4 Mb/s cada uno.
- **Configuraciones reales reportadas:** estaciones ION usan 2 canales HD en 720p + 6 subcanales SD en 480i; hay estaciones que llegan a 17 subcanales SD detrás de un canal principal HD.
- Regla general repetida en foros técnicos de broadcast: "buena calidad" en 720p necesita al menos 12-15 Mb/s, y en 1080i al menos 15 Mb/s (hasta 28 Mb/s para eliminar artefactos, algo que ninguna estación con subcanales puede darse el lujo de usar).

Fuentes: AVS Forum (hilo de ingeniería, cifras de PBS/ABC): https://www.avsforum.com/threads/how-is-38-8-mbps-bitrate-for-720p-possible-for-ota.824270/ ; ejemplos de configuraciones reales de estaciones: https://radiodiscussions.com/threads/atsc-1-0-up-to-24-sd-or-6-hd.768998/ y https://tvradioschedules.fandom.com/wiki/Digital_subchannel

**Nota de calidad de fuente:** estas cifras vienen de foros de ingenieros de broadcast, no de un documento normativo. Son "lo habitual", no "lo exigido" — se citan aquí exactamente para eso, como referencia de mercado, no como regla.

---

## 6. ¿Quién pone el número en la práctica: el encoder de playout o el multiplexor?

- **El multiplexor (o un statmux dedicado) es quien normalmente controla y reparte el bitrate final**, no el encoder de playout de forma aislada. En sistemas con multiplexación estadística (statmux), el flujo es bidireccional: los encoders reportan parámetros de complejidad de la señal ("need parameters") al controlador del statmux, y este le devuelve a cada encoder la asignación de bitrate que debe usar en ese instante, de modo que la suma de todos los programas nunca exceda la capacidad fija del canal (constant bitrate, CBR) de salida.
  Fuente: resumen de literatura de patentes de sistemas de multiplexación estadística (Anexo de búsqueda), y documentación de AWS Elemental sobre rate control con statmux: https://docs.aws.amazon.com/elemental-server/latest/ug/vq-statmux.html
- **Rate shaping:** un remultiplexor estadístico puede re-codificar (o forzar el recorte de) programas individuales para que, en conjunto, no superen la tasa de bits del canal — a esto se le llama "rate shaping", y es una función clásica del multiplexor, no del encoder de origen.
  Fuente: literatura de patentes de sistemas de rate shaping en multiplexación estadística (resultados de búsqueda técnica).
- **En el caso concreto de Caribbean Advantage TV:** el flujo es playout (VLC) → UDP multicast a un TS ya codificado con vb=4800 kb/s fijo → entra al Technalogix TP1000, que **remapea PID y números de canal virtual**, no que hace statmux clásico entre múltiples encoders independientes. El manual público del TP1000 describe remultiplexado con "PID editor, ASI injection, remap, restamp, grooming, and add-drop" — es decir, sí puede reempaquetar y ajustar el TS, pero las fuentes públicas no confirman que haga un rate-shaping activo tipo statmux con retroalimentación al encoder. Lo más probable, dado el flujo descrito, es que el TP1000 reciba un TS con el bitrate ya fijado por VLC (4800 kb/s video + audio) y lo remultiplexe manteniendo esa tasa dentro del budget de 19.39 Mb/s del canal completo, sin renegociar bitrate con el origen.
  Fuente: manual TP1000: https://www.manualslib.com/manual/1357407/Technalogix-Tp1000.html
- **Qué sobrevive del bitrate de entrada al remultiplexar:** en un remux simple (sin recodificación), el bitrate de cada elemental stream (video, audio) sobrevive intacto — lo que cambia son los metadatos de transporte (PID, PCR, PSIP/VCT, PAT/PMT). Si el mux hiciera statmux real con recodificación, ahí sí el bitrate de entrada no sobrevive tal cual. No hay evidencia pública de que el TP1000 haga esto último en este despliegue.

---

## Qué cambia una decisión y qué no

| Pregunta | Respuesta | Fuente |
|---|---|---|
| ¿El bitrate de video se "compra"? | No — lo da el canal de 6 MHz licenciado, 19.39 Mb/s fijo en ATSC 1.0 | A/53, 47 CFR 73.682 |
| ¿Hay un mínimo/máximo FCC para video? | No, es decisión del operador dentro del techo del canal | 47 CFR 73.682 (no fija cifra) |
| ¿AC-3 es obligatorio? | Sí, por incorporación normativa de A/52 en 73.682(d), también para Class A | 73.682, Subparte J |
| ¿MPEG capa II es válido en ATSC de aire? | No hay evidencia de que esté permitido; el estándar exige AC-3 | A/52, 73.682 |
| ¿44.1 kHz es aceptable? | No — A/53 Parte 5 exige 48 kHz | A/53 Parte 5:2014 |
| ¿Quién reparte el bitrate en la práctica? | El multiplexor/statmux, no el encoder aislado; en este caso probablemente el TP1000 solo remultiplexa sin renegociar | AWS Elemental docs, manual TP1000 |

---

## Fuentes citadas (lista completa)

- ATSC A/53, Parts 1-6, 2007: https://www.atsc.org/wp-content/uploads/2021/04/a_53-Part-1-6-2007.pdf
- ATSC A/53 Parte 4 (MPEG-2 Video System Characteristics): https://www.atsc.org/wp-content/uploads/2021/04/A_53-Part-4-2009.pdf
- ATSC A/53 Parte 5:2014 (AC-3 Audio System): https://www.atsc.org/wp-content/uploads/2021/04/A53-Part-5-2014.pdf
- ATSC A/52:2012/2018 (Digital Audio Compression, AC-3/E-AC-3): https://www.atsc.org/atsc-documents/a522012-digital-audio-compression-ac-3-e-ac-3-standard-12172012/
- 47 CFR §73.682 (eCFR): https://www.ecfr.gov/current/title-47/chapter-I/subchapter-C/part-73/subpart-E/section-73.682
- 47 CFR §73.682 (Cornell LII, texto): https://www.law.cornell.edu/cfr/text/47/73.682
- 47 CFR Part 73 Subpart J — Class A Television Broadcast Stations (eCFR): https://www.ecfr.gov/current/title-47/chapter-I/subchapter-C/part-73/subpart-J
- FCC 04-220 (reglas de LPTV/Class A digital, calidad comparable a NTSC): https://docs.fcc.gov/public/attachments/FCC-04-220A1.pdf
- TV Technology, "ATSC transport streams": https://www.tvtechnology.com/miscellaneous/atsc-transport-streams
- TV Technology, "Increasing Channel Bandwidth to Broadcast 8K" (ATSC 3.0): https://www.tvtechnology.com/atsc3/increasing-channel-bandwidth-to-broadcast-8k
- ATSC 3.0 Physical Layer paper (DekTec, vía atsc.org): https://www.atsc.org/wp-content/uploads/2020/05/f-36-26-12666874_ZItpm7tg_ATSC3p0Paper.pdf
- AVS Forum, hilo sobre bitrate real de 720p/1080i (cifras de PBS/ABC): https://www.avsforum.com/threads/how-is-38-8-mbps-bitrate-for-720p-possible-for-ota.824270/
- RadioDiscussions, ejemplos de reparto de subcanales por estación: https://radiodiscussions.com/threads/atsc-1-0-up-to-24-sd-or-6-hd.768998/
- Digital subchannel (Fandom wiki, ejemplos de configuración): https://tvradioschedules.fandom.com/wiki/Digital_subchannel
- AWS Elemental, "Encoding – Statmux Rate Control": https://docs.aws.amazon.com/elemental-server/latest/ug/vq-statmux.html
- Manual Technalogix TP1000 (ManualsLib): https://www.manualslib.com/manual/1357407/Technalogix-Tp1000.html
- Wikipedia, 48,000 Hz (contexto de por qué 48 kHz es el estándar de video/broadcast): https://en.wikipedia.org/wiki/48,000_Hz

---

## Lo que no se encontró (dicho explícitamente)

- No hay un documento único de la FCC o del ATSC que declare, en esas palabras, "el bitrate no se contrata" — es una conclusión derivada de que 73.682 define un estándar técnico de transmisión, no un mercado de capacidad contratable.
- No hay evidencia pública sobre si el Technalogix TP1000, en este despliegue específico, transcodifica el audio de MPEG capa II a AC-3 o simplemente lo remultiplexa tal cual. Esto se debe confirmar con quien programó o instaló el TP1000 en la planta de Caribbean Advantage TV — es información operativa, no pública.
- No encontré una cifra única y oficial del ATSC para "el" bitrate máximo de ATSC 3.0 en 6 MHz — el estándar permite combinaciones de modulación/FEC, y las fuentes dan un rango (1 a 57 Mb/s teóricos), no un número fijo como en ATSC 1.0.
