# Catálogo de equipos, protocolos y bibliotecas

_Hecho el 10 de septiembre de 2026 con cinco investigaciones en paralelo,
cada dato con su fuente y fecha. Es lo que T8 (telemetría y ENDEC) y T2/T4
(salidas y fuentes) necesitan antes de escribir un driver. Nada aquí es
código; cuando un dato no es público, se dice._

**La respuesta corta a «¿hay un sitio de donde sacar los drivers?»: no.** Un
equipo de estación no trae un driver instalable; trae un protocolo
documentado (serial, relés, SNMP, HTTP) o no trae nada público. El driver lo
escribe quien integra, en `internal/drivers/`, contra el manual del
fabricante. Este catálogo dice, equipo por equipo, qué protocolo es, dónde
está el manual y con qué biblioteca de Go (sin CGo) se habla.

| Archivo | Qué cubre |
|---|---|
| [`catalogo/01-alertas-eas.md`](catalogo/01-alertas-eas.md) | Sage ENDEC 1822/3644, DASDEC, Gorman-Redlich, TFT; protocolo SAME como lectura |
| [`catalogo/02-transmisores-telemetria.md`](catalogo/02-transmisores-telemetria.md) | RVR, Technalogix, GatesAir, R&S, Anywave, Elenos, Hitachi-Comark, Egatel; SNMP y MIBs |
| [`catalogo/03-multiplexores-psip-cortes.md`](catalogo/03-multiplexores-psip-cortes.md) | TP1000 y comparables, generadores PSIP (PMCP), SCTE-104/35, placas de relés, encoders externos |
| [`catalogo/04-captura-retorno-gpi.md`](catalogo/04-captura-retorno-gpi.md) | Retorno de aire (HDHomeRun, DVB/BDA), captura UVC/dshow/v4l2, GPI/GPO, audio para radio |
| [`catalogo/05-bibliotecas-go-protocolos.md`](catalogo/05-bibliotecas-go-protocolos.md) | Una biblioteca Go pura por familia, verificada; qué se escribe a mano |

## Lo que cambia decisiones

1. **Todo lo que es protocolo por red o por cable se habla desde Go sin C.**
   SNMP, serial, HTTP, syslog, UDP/TS, SRT, RTMP, RTSP, WebRTC, NTP tienen
   biblioteca pura (tabla final del archivo 05). C solo hace falta cuando
   el único camino es el SDK del fabricante: Decklink, NDI, DekTec, Dante.
   Esos quedan fuera (ADR 0002) y su alternativa es ffmpeg como binario
   externo o un equipo que hable red.
2. **Placas de relés: serial, no HID.** No existe biblioteca HID en Go sin
   CGo (`karalabe/hid`, `go-hid`, `bearsh/hid` envuelven `hidapi`). Numato,
   Denkovi, KMtronic y Sealevel hablan serial ASCII o Ethernet y se cubren
   con `go.bug.st/serial`. SainSmart HID y Ontrak ADU no.
3. **Tres cosas se escriben a mano en Go porque no existen en ningún
   lado:** SCTE-104 (mensajes binarios por TCP, ADR 0004), el decodificador
   SAME (FSK 520.83 baud, evidencia en el retorno, ADR 0010) y un lector
   CAP (XML; NOAA sin registro, IPAWS con trámite ante FEMA).
4. **La guía para PSIP no es XMLTV: es PMCP** (XML de ATSC A/76). Triveni
   GuideBuilder, el generador de referencia, lo consume nativo. Antena787
   necesita un exportador PMCP además de `/guia.xml` (F5, junto con el
   PSIP propio). Pendiente de confirmar si el TP1000 genera PSIP o si hay
   otro equipo en la cadena de CAtv.
5. **Retorno de aire recomendado: SiliconDust HDHomeRun** (≈ US$110). Expone
   el TS por HTTP con API pública; sin driver, sin C, fuera de la máquina
   de playout, y es lo que `signal-compare` necesita (ADR 0009). La tarjeta
   actual de CAtv (DVB/BDA, modelo sin confirmar) exigiría sintonizar
   aparte.
6. **Sage ENDEC 1822: verificado del todo** (manual público con protocolo
   serial, formato ZCZC, relés y programas). **Sage 3644: HTTP y correo
   confirmados; syslog y SNMP no aparecen en ninguna fuente pública** —
   `docs/drivers/README.md` los daba por hechos y ya no. DASDEC: HTTP y
   GPIO confirmados; SNMP/syslog no encontrados.
7. **El amplificador «ADR» de CAtv no existe como marca.** La hipótesis
   más fuerte es *Adrenalin*, el control de amplificadores de Technalogix
   (el mismo fabricante del TP1000). Se confirma con la placa.
8. **No hay MIB común de broadcast.** Cada fabricante trae la suya; la única
   pública sin contacto es la de Elenos/Screen Service. RVR, GatesAir, R&S,
   Anywave, Comark y Egatel hablan SNMP o web, pero la MIB se pide.

## Preguntas para el ingeniero de CAtv, todas juntas

**Sage ENDEC** (archivo 01): modelo exacto (1822 o 3644); qué puerto serial
está libre y a qué baudios; pinout real del bloque verde de relés (qué
terminal va a qué equipo y con qué función); versión de firmware; si tiene
red y si el puerto de automatización o la web están habilitados; si hay una
entrada *Manual Override* cableada (el «commercial tally» del ADR 0010).

**Excitador y amplificador** (archivo 02): modelo exacto del RVR (placa) y
del amplificador (¿dice *Adrenalin*?); si tienen IP de manejo asignada; si
SNMP está habilitado y con qué versión y comunidad; firmware de ambos; si el
USB del RVR aparece como puerto COM; qué contactos de alarma libres hay y
para qué evento; si algún equipo «miente» en algún estado.

**Multiplexor TP1000** (archivo 03): unicast a IP:puerto o multicast (grupo
y TTL); qué PIDs y número de programa espera, o si remapea; si genera PSIP
él mismo; si acepta guía por red y en qué formato (PMCP); si tiene SNMP o
web y en cuál de los dos puertos Ethernet; si deja pasar SCTE-35 al
remultiplexar.

**Retorno de aire** (archivo 04): modelo exacto de la tarjeta receptora de
TV del PC y si se puede leer por red; si están dispuestos a poner un
HDHomeRun en la torre.

**Y lo de siempre** (`docs/VLC-PARIDAD.md`): la cadena `sout` exacta de VLC.

## Cómo se usa esto

Cuando se abra una tanda que necesite un driver (T2 salidas, T4 vivos, T8
telemetría y ENDEC), el agente o la persona empieza por la fila del equipo
en su archivo, sigue el enlace al manual o a la MIB, y escribe el driver con
la biblioteca elegida en el archivo 05. Un equipo sin fila aquí empieza por
un issue con su manual, como dice `README.md`.
