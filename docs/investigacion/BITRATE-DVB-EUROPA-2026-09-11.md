# Bitrate en el mundo DVB (Europa): ¿quién lo decide?

Fecha: 11 de septiembre de 2026
Alcance: investigación de fuentes públicas para decidir si Antena787 debe controlar, recomendar, o dejar fuera de su alcance el bitrate de audio/video — hoy cableado en 192 kb/s de audio, con video configurable.

---

## Recomendación corta

**No es asunto del playout fijar el bitrate.** En el mundo DVB el número lo decide, en cascada: el regulador (cuánta carga cabe en el múltiplex, vía el estándar de modulación y el plan técnico nacional) → el operador del múltiplex (cómo reparte esa carga entre los canales que le alquilan hueco, y si usa multiplexado estadístico) → el contrato comercial entre el operador y el canal (el número que el canal tiene que entregar en el punto de entrega). El playout **entrega** al número que le digan, no lo **propone**. Ver sección final para el detalle de por qué esto también aplica al audio.

---

## 1. El techo del múltiplex: cuánto cabe, y por qué varía

El bitrate útil de un múltiplex DVB-T o DVB-T2 no es un número fijo: depende de la combinación de modulación, tasa de codificación (code rate) e intervalo de guarda que se elija para ese canal de RF. Esto está definido en el estándar ETSI mismo, no lo inventa cada operador.

**DVB-T (ETSI EN 300 744), canal de 8 MHz — Tabla 17 del estándar:**
El rango completo va de ~4.98 Mbit/s (QPSK, code rate 1/2, el extremo más robusto) hasta ~31.67 Mbit/s (64QAM, code rate 7/8, el extremo de mayor carga). Las configuraciones que de hecho se usaron en Europa para servicios comerciales están en el medio:
- 64QAM, code rate 2/3, guarda 1/8 → **22.12 Mbit/s**
- 64QAM, code rate 2/3, guarda 1/32 → **24.13 Mbit/s** (esta es la configuración que el Reino Unido usó como estándar de facto en sus múltiplex DVB-T — "UK MFN DVB-T profile")
- 64QAM, code rate 3/4, guarda 1/32 → **27.14 Mbit/s**

Fuente: ETSI EN 300 744, *Framing structure, channel coding and modulation for digital terrestrial television* — https://www.etsi.org/deliver/etsi_en/300700_300799/300744/01.06.02_60/en_300744v010602p.pdf ; tabla citada y valores confirmados en https://en.wikipedia.org/wiki/DVB-T (sección "Technical description of a DVB-T transmitter").

**DVB-T2, canal de 8 MHz:**
El rango sube bastante: de ~7.4 Mbit/s (QPSK) hasta ~50.3 Mbit/s (256QAM), según la tabla "Recommended maximum bit-rate configurations" del estándar. El punto de comparación real que encontré: el mismo perfil robusto que en DVB-T rendía 24.13 Mbit/s, en DVB-T2 (256QAM, 32K FFT, code rate 3/5, guarda 1/128) rinde **35.4 Mbit/s** con una robustez de señal equivalente — es decir, +47% de carga útil por el mismo "costo" de cobertura.

Fuente: https://en.wikipedia.org/wiki/DVB-T2 (sección "System differences with DVB-T"); DVB Fact Sheet (dvb.org, ed. sept. 2018) — https://www.antenall.rs/media/products/411/files/dvb-t2_factsheet.pdf.

**Casos reales publicados:**
- Reino Unido (Freeview): cada múltiplex es "un flujo protegido contra errores de 24, 27 o 40 megabits por segundo" — los primeros dos son perfiles DVB-T, el tercero es DVB-T2. Fuente: https://en.wikipedia.org/wiki/Freeview_(UK).
- España: la cifra que circula para un múltiplex nacional TDT es de **~19.91 Mbit/s** de carga útil, repartida entre hasta 4 canales HD. Esta cifra viene de una fuente secundaria (no del BOE), así que la trato como *habitual*, no normativa: https://bandaancha.eu/articulos/como-funciona-tdt-multiplex-canales-hd-9839.

**Conclusión del punto 1:** el techo del múltiplex es un hecho de ingeniería de RF (modulación × code rate × ancho de banda del canal), fijado por el estándar y por la elección de perfil que hace el operador de red — no por el playout. El playout nunca ve ese número directamente; ve la porción que el operador de mux le asigna a su canal.

---

## 2. ¿Se compra el ancho de banda? Sí — se alquila hueco en un múltiplex ajeno

En Europa lo normal para una estación chica **no** es operar transmisor propio con múltiplex propio: es contratar un hueco (una porción de Mbit/s, o un "slot" de servicio) dentro de un múltiplex que opera una empresa de infraestructura de difusión.

- **Reino Unido**: Arqiva y otros operan los múltiplex comerciales bajo licencia de Ofcom, y de ahí "alquilan" capacidad a los canales. Ofcom documenta que "los operadores de múltiplex planeaban poner las redes a disposición de los ganadores de múltiplex comerciales para el alquiler de capacidad, una vez concluidas las negociaciones, acordado el alquiler y recibida una fianza" — es decir, es un contrato comercial negociado, no un número que fije el regulador canal por canal. Fuente: hallado vía búsqueda pública citando documentación de Ofcom sobre renovación de licencias de múltiplex — https://www.ofcom.org.uk/tv-radio-and-on-demand/digital-tv/multiplex y https://www.gov.uk/government/consultations/consultation-on-the-renewal-of-digital-terrestrial-television-dtt-multiplex-licences-expiring-in-2022-and-2026/outcome/consultation-on-the-renewal-of-digital-terrestrial-television-dtt-multiplex-licences-full-government-response.
  - Intenté acceder a la "Reference Offer for the Provision of Network Access" de Arqiva (el documento comercial donde debería estar el precio por Mb/s y los tramos) — el enlace público que encontré devolvió error 404 al momento de esta investigación. **No pude confirmar tramos ni precio por Mb/s con una fuente primaria legible.**
- **Francia**: TDF opera la infraestructura de multiplexado/difusión (DiffHF-TNT) y publica una "offre de référence" con las condiciones técnicas y comerciales de acceso. Encontré el documento (https://www.tdf.fr/sites/default/files/offre_de_reference_diffhf_rp_2022_v1_01062022.pdf) pero el texto no fue extraíble de forma confiable para citar cláusulas exactas sobre Mbit/s o tramos — **no cito números de ese documento porque no pude verificarlos**.
- **España**: existen múltiplex nacionales, autonómicos (MAUT) y locales, y los canales "arriendan" un hueco dentro del múltiplex que gestiona el operador correspondiente; hay reportes periodísticos de venta/traspaso de múltiplex completos entre operadores (ej. Unidad Editorial vendiendo su múltiplex de TDT — https://www.elconfidencialdigital.com/articulo/medios/Unidad-Editorial-venta-multiplex-TDT/20140613192922073310.html), lo que confirma que la capacidad de un múltiplex es un activo que se compra/vende/renta, no un derecho técnico gratuito. **No encontré tarifas públicas por Mb/s.**

**Quién impone el número:** en todos los casos que documenté, es el **operador del múltiplex** el que le dice al canal (vía el contrato de acceso/alquiler) qué debe entregar en el punto de entrega — no un regulador fijando el bitrate del canal individual, y desde luego no el equipo de playout del canal.

---

## 3. ¿Hay mínimos o máximos obligatorios de bitrate en alguna normativa europea?

**No encontré ninguna normativa europea (ETSI, Comisión Europea, o de un regulador nacional) que fije un bitrate mínimo o máximo obligatorio para un canal individual de televisión dentro de un múltiplex.**

Lo que sí regulan los estados:
- El **plan técnico nacional** fija la capacidad total del múltiplex (vía el perfil de modulación permitido) y cuántos programas puede llevar como máximo — en España, el Real Decreto 250/2025 (que aprueba el Plan Técnico Nacional de la TDT) establece que todas las emisiones de TDT son en alta definición desde febrero de 2024 y que cada múltiple tiene capacidad para un máximo de cuatro canales HD — pero esto es un límite de *número de canales y resolución*, no una cifra de Mbit/s por canal. Fuente: citado en https://normativa.infocentre.es/plan-tecnico-nacional-de-la-television-digital-terrestre-tdt/ y en la cobertura de prensa técnica (https://www.televes.com/es/plan-tecnico-nacional-tdt).
- La Orden ITC/2212/2007 (España) regula obligaciones de los **gestores de múltiplex** (registro de parámetros, información de servicios) pero, según lo que pude confirmar, no fija un bitrate por canal — fuente: https://www.boe.es/buscar/act.php?id=BOE-A-2007-13973 (texto consolidado; recomiendo revisión legal directa si esto se vuelve crítico para el diseño, porque no leí el articulado completo, solo el índice/resumen).
- Reino Unido: la licencia Broadcasting Act de cada múltiplex (ej. Multiplex 2 — https://www.ofcom.org.uk/siteassets/resources/documents/manage-your-licence/tv/mux/mux-2/licence-mux-2.pdf) probablemente define "multiplex capacity" en términos técnicos, pero el documento me devolvió error 403 al intentar leerlo — **no pude confirmar el texto exacto de esa cláusula.**

**Conclusión:** lo que hay es normativo a nivel de *arquitectura del múltiplex* (cuánta gente cabe, en qué resolución), y comercial a nivel de *cuánto bitrate le toca a cada canal*. No hay un piso o techo legal por canal.

---

## 4. El audio: qué usa DVB, y si 44.1 kHz es aceptable

Fuente primaria: **ETSI TS 101 154 V2.7.1 (2022-01)**, *Implementation guidelines for the use of video and audio coding in broadcasting applications* — descargado y verificado directamente (https://www.etsi.org/deliver/etsi_ts/101100_101199/101154/02.07.01_60/ts_101154v020701p.pdf).

**Códecs permitidos:** MPEG-1/MPEG-2 Layer II (el "de siempre" en DVB), AC-3/Enhanced AC-3 (Dolby Digital), AC-4, MPEG-4 AAC/HE-AAC/HE-AAC v2, MPEG-H. El estándar dice explícitamente: *"The use of Layer II encoding is recommended for MPEG-1 audio bitstreams"* (cláusula 6.1.2, línea de la especificación). Layer II sigue siendo el códec base recomendado, no uno legado que haya que reemplazar.

**Bitrates de audio Layer II permitidos por el estándar (cláusula 6.1.3):**
32, 48, 56, 64, 80, 96, 112, **128**, 160, **192**, 224, 256, 320, 384 kbit/s — los 14 valores del bitrate_index de MPEG-1/2 Layer II, todos igualmente válidos según DVB. **Tanto 128 como 192 kb/s son valores estándar y legítimos** — no hay preferencia normativa por 192 sobre 128.

**Frecuencia de muestreo (cláusula 6.1.4) — cita textual:**
> "The audio sampling rate of primary sound services **shall be 32 kHz, 44,1 kHz or 48 kHz**."

Es decir: **44.1 kHz está explícitamente autorizado para codificación, al mismo nivel normativo que 48 kHz.** No es un caso tolerado o de compatibilidad hacia atrás — es una de las tres opciones que el estándar declara obligatoriamente soportadas tanto en el lado de codificación como de decodificación ("The IRD shall be capable of decoding audio with sampling rates of 32 kHz, 44,1 kHz and 48 kHz"). **48 kHz NO es obligatorio de facto; es una alternativa igual de válida a 44.1 kHz según el propio ETSI.**

Dato relevante para Antena787: el cliente actual entrega MPEG capa II a 128 kb/s y 44100 Hz — ambos valores están dentro de lo que DVB permite sin ninguna excepción ni nota al calce.

---

## 5. Statmux: si el múltiplex reasigna el bitrate, ¿para qué fijarlo en el playout?

Esta es la pregunta central, y la respuesta corta es: **el statmux hace que fijar un bitrate constante en el encoder sea, en el mejor de los casos, un desperdicio, y en el peor, contraproducente.**

Cómo funciona en la práctica: cada encoder evalúa la complejidad de la escena que está codificando en ese instante, reporta esa complejidad a un analizador central del múltiplex, y el analizador redistribuye el bitrate disponible entre todos los canales del mux según la demanda real de cada uno — no según un número fijo que cada canal pidió de antemano. Fuente: literatura técnica sobre statmux en DVB — https://onlinelibrary.wiley.com/doi/10.1155/2009/261231 y https://www2.cs.sfu.ca/~mhefeeda/Papers/tomccap11_statmux.pdf.

Un ejemplo documentado en el mundo real: en el Reino Unido, el paso a multiplexado estadístico completo para BBC One (a partir de la primavera de 2009) **liberó 2 Mbit/s adicionales** dentro del múltiplex — es decir, el propio operador de red *ganó* capacidad simplemente por dejar que el bitrate fluctuara según contenido, en lugar de reservarle a cada canal una tajada fija. (Cita indexada de "The Future of Digital Terrestrial Television", Ofcom — no pude abrir el PDF directamente por bloqueo del sitio, así que marco esto como *hallazgo de fuente secundaria a verificar*, aunque el documento es de Ofcom.)

**Consecuencia para el bitrate de video en Antena787 (que sí es configurable):** en un mux con statmux activo, el número que el playout "pide" en CBR es, en el mejor caso, un techo de referencia que el propio encoder de contribución del operador de red probablemente va a ignorar o renegociar; en un mux sin statmux (CBR puro, típico de estaciones chicas sin acceso a esa tecnología), sí importa que el playout entregue exactamente el número pactado, ni más ni menos, porque no hay nadie del otro lado absorbiendo la variación.

**Lo que esto significa para el diseño:** el software no puede asumir que "el bitrate correcto" es un valor universal — depende de si el operador de mux aguas abajo usa CBR fijo (necesita que el playout entregue ese número exacto) o VBR/statmux (necesita que el playout entregue *dentro de un rango*, y probablemente reciba instrucciones dinámicas de un protocolo de gestión de tasa que el playout ni siquiera implementa hoy). Ninguno de los dos escenarios se resuelve con un valor cableado en el software — se resuelve con un campo de configuración que el operador de cada estación llena según lo que le exija SU operador de mux.

---

## 6. Qué exige un operador de múltiplex a quien le entrega la señal

Busqué especificaciones de entrega ("delivery specification") publicadas por operadores de múltiplex europeos (Arqiva, TDF, Digital UK/Freeview) con el detalle técnico exacto (bitrate constante o variable, códec, muestreo, PIDs). **Resultado: no logré confirmar el contenido de ninguna con una fuente primaria legible.**
- Arqiva: el enlace público a su "Reference Offer for the Provision of Network Access" devolvió 404 en el momento de esta investigación.
- TDF: el documento existe y es público, pero el texto no fue extraíble de forma confiable para citar cláusulas.
- Ofcom (licencia de múltiplex, que suele fijar condiciones técnicas): devolvió error 403 al acceso automatizado.

Esto no significa que esas especificaciones no existan — al contrario, es casi seguro que cada operador de mux tiene un documento así, dado que alguien tiene que decirle al canal qué formato de señal aceptar en el punto de entrega (vía ASI o IP, con PIDs específicos, códec, bitrate). **Pero no puedo citar sus cláusulas exactas sin haber podido leerlas**, y prefiero decir eso a inventar un número. Si esto se vuelve crítico para el diseño (por ejemplo, para decidir si Antena787 necesita soportar entrega por IP con un protocolo de señalización de tasa), recomiendo conseguir esos documentos por otra vía (contacto directo con un operador, o un ingeniero de la industria) en vez de depender de búsqueda pública.

---

## Fuentes citadas

- ETSI EN 300 744 (DVB-T): https://www.etsi.org/deliver/etsi_en/300700_300799/300744/01.06.02_60/en_300744v010602p.pdf
- ETSI TS 101 154 V2.7.1 (audio/video DVB), verificado directamente: https://www.etsi.org/deliver/etsi_ts/101100_101199/101154/02.07.01_60/ts_101154v020701p.pdf
- DVB-T — Wikipedia (tabla de bitrates): https://en.wikipedia.org/wiki/DVB-T
- DVB-T2 — Wikipedia (comparación de perfiles): https://en.wikipedia.org/wiki/DVB-T2
- DVB Fact Sheet, DVB-T2, sept. 2018: https://www.antenall.rs/media/products/411/files/dvb-t2_factsheet.pdf
- Freeview (UK) — Wikipedia (24/27/40 Mbit/s por múltiplex): https://en.wikipedia.org/wiki/Freeview_(UK)
- Ofcom, listado de licenciatarios de múltiplex: https://www.ofcom.org.uk/tv-radio-and-on-demand/digital-tv/multiplex
- GOV.UK, respuesta consulta renovación licencias DTT: https://www.gov.uk/government/consultations/consultation-on-the-renewal-of-digital-terrestrial-television-dtt-multiplex-licences-expiring-in-2022-and-2026/outcome/consultation-on-the-renewal-of-digital-terrestrial-television-dtt-multiplex-licences-full-government-response
- Plan Técnico Nacional TDT España (RD 250/2025): https://normativa.infocentre.es/plan-tecnico-nacional-de-la-television-digital-terrestre-tdt/
- Orden ITC/2212/2007 (BOE): https://www.boe.es/buscar/act.php?id=BOE-A-2007-13973
- Capacidad múltiplex TDT España (~19.91 Mbit/s), fuente secundaria: https://bandaancha.eu/articulos/como-funciona-tdt-multiplex-canales-hd-9839
- Venta de múltiplex completo (ejemplo de mux como activo comercial): https://www.elconfidencialdigital.com/articulo/medios/Unidad-Editorial-venta-multiplex-TDT/20140613192922073310.html
- Statmux en DVB-H/DVB-T2, literatura técnica: https://onlinelibrary.wiley.com/doi/10.1155/2009/261231 y https://www2.cs.sfu.ca/~mhefeeda/Papers/tomccap11_statmux.pdf

**No verificado / intentado sin éxito** (documentado arriba, sección 2 y 6): Arqiva Reference Offer (404), TDF DiffHF-TNT offre de référence (texto no extraíble), licencia Ofcom Multiplex 2 (403), Ofcom "The Future of Digital Terrestrial Television" sobre statmux (bloqueado, cita de fuente secundaria únicamente).
