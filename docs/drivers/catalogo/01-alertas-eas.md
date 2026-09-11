# Catálogo de equipos EAS

Referencia para las familias `gpi-serial`, `gpi-gpio`, `sage-endec`,
`dasdec`, `syslog`, `snmp-trap`, `cap-poll` (PRD §10,
`docs/drivers/README.md`). No define interfaces de Go — todavía no
existen. No cambia ADR [0010](../../adr/0010-eas-integrate-the-endec-never-replace-it.md):
Antena787 no construye equipo de alertas, integra el que ya está
certificado. Fecha de consulta de todas las fuentes: **10 de septiembre de
2026**.

---

## Sage Digital ENDEC 1822

Manual público completo (Harris Broadcast, Rev 1.5), archivado por un
radioaficionado porque ya no está en la web de Sage:
[SAGE-1822-Users-Manual.pdf](https://kj7bre.com/eas-manual-archive/manual/SAGE-1822-Users-Manual.pdf).

**Serial:** seis puertos DB-9, rol fijo. COM2/COM6 a 9600 baud fijo,
COM4/COM5 a 1200 fijo, COM3 y "Computer" a baudios variables. Pinout:
2=RxD, 3=TxD, 5=GND, 9=+8V accesorios (§12.1).

**"Encoder device" (salida cruda):** repite el formato síncrono de la
Parte 11 en async: tres cabeceras, silencios, tres EOM, con `0xAB` como
byte de sincronismo (se ve como `+`) — p.ej.
`++++ZCZC-EAS-RWT-006013+0015-1020624-SAGE   -` seguido de tres `NNNN`.

**"Decoder device" (estado):** `<tipo>:<cadena ZCZC>` + texto expandido.
Tipo es `local` (el ENDEC envía = alerta iniciada), `match` (oída, filtro
coincide), `nomatch`, `dup`. No hay mensaje explícito de "alerta
terminada" aparte del EOM dentro del mismo bloque (§8.1, pág. 59).

**Relés:** tres, normalmente abiertos, 1A / 125VAC-60VDC máx. Se asignan a
un "programa": `PTT` (cerrado mientras se envía = alerta activa),
`Pending`/`Pending Done` (recibida, sin decidir), `ATTN Detect/Send`,
`Ready`, y variantes con demora (`Delay Pre/Post`, `End Pulse Pre/Post`).
Por defecto: ATTN Active=`ATTN DETECT`, Encoder Active=`PTT`, Decoder
Active=`PENDING` (§4.5, §5.7).

**Entrada "Manual Override":** la automatización puede cerrarla para
retener una alerta no crítica hasta el fin de una tanda comercial
("commercial tally", máx. 15 min; EAN/EAT la ignoran siempre) — el
mecanismo que describe ADR 0010 para CAtv, ya viene en el equipo (§8.5,
pág. 61). **Red:** no tiene, solo serial y relés.

**Driver probable:** `sage-endec` para el framing propio de Sage (no es
SAME crudo); `gpi-gpio` para los relés. **Librería:** `go.bug.st/serial`
(sin CGo). **Estado: verificado.**

---

## Sage Digital ENDEC 3644

Mismo protocolo serial que el 1822 ("drop-in replacement" según el
fabricante) más una capa de red que el 1822 no tiene. El Sage de CAtv está
cableado por relés (PRD §10); no se sabe si es 1822 o 3644, ni si el
serial está libre.

Hoja de especificaciones oficial (2011), la fuente más confiable
encontrada: [endec_spec_sheet.pdf](https://www.sagealertingsystems.com/endec_spec_sheet.pdf).
Confirma: **LAN** 10/100 RJ-45 (DHCP o IP fija); **HTTP/HTTPS** para
programar y ver estado; **NTP**; **correo** para registro con audio y
texto adjunto (STARTTLS, SSL); **FTP** mencionado solo en mercadotecnia,
sin protocolo detallado. Seis puertos serial DB-9 (igual que el 1822), 5
entradas de propósito general y 4 contactos secos de salida.

**SNMP: no encontrado.** Ninguna fuente revisada —ni la hoja técnica, ni el
manual preliminar v1.0 (FCC ID V2W3644), ni una referencia de terceros
sobre el mismo protocolo— lo menciona.

**Syslog: no encontrado como tal.** El README del proyecto describe la red
del 3644 como "HTTP y syslog", pero ningún documento de Sage consultado usa
la palabra "syslog". Lo documentado es registro por HTTP, correo, y —según
la referencia de terceros de abajo— un puerto TCP de automatización
(`MENU.NETWORK.PORT BASE`) apagado de fábrica. Puede existir en un manual o
firmware más nuevo que no se pudo consultar (los enlaces del manual
completo v95 dieron error 403). **Verificar contra el equipo real.**

Referencia de terceros sobre el protocolo (no es manual de Sage, es pista):
[easstation.com/…/SAGE_ENDEC](https://easstation.com/docs/reference/protocols/SAGE_ENDEC)
— agrega al 1822 un generador de caracteres genérico
(`<STX><severidad><texto><ETX>`) y un formato para sistemas de noticias
(`<ENDECSTART>...<ENDECEND>`).

**Driver probable:** `sage-endec` (TCP o serial); `gpi-gpio` (contactos);
`syslog`/`snmp-trap` solo si se confirman. **Librería:** `go.bug.st/serial`,
`net/http`, `log/syslog` y `gosnmp` (`github.com/gosnmp/gosnmp`, sin CGo)
si aplican. **Estado: parcial** — red y HTTP confirmados; syslog y SNMP no.

---

## DASDEC I / II / III / 1000 (Digital Alert Systems, antes Monroe Electronics)

Digital Alert Systems es Monroe Electronics desde que licenció en
exclusiva la fabricación del DASDEC (2004-2005): misma línea, no dos
fabricantes. Común en cabeceras de cable (Comcast, Charter, Cox) y en
TV/radio. Manual del DASDEC-II (el del fabricante dio 403, esta copia de un
radiodifusor cargó):
[DASDEC_II_EAS-Manual.pdf](https://ear.ewtn.com/downloads/engineering/DASDEC_II_EAS-Manual.pdf).

**HTTP:** servidor web integrado, es la interfaz completa del equipo, con
opción SSL/HTTPS. **GPIO de fábrica:** 2 GPO + 2 GPI; GPO 1 limitado a
eventos de audio/video, GPO 2 admite más tipos de evento; una GPI puede
retener ("hold") una alerta pendiente, mismo mecanismo que el "Manual
Override" del Sage. **GPIO en red (opcional):** módulos externos (Titus
WR-300, Control-by-Web) agregan relés por red en vez del panel trasero.
**Serial:** un RS-232 de fábrica, hasta 4 más por USB, hablando los
protocolos de CG "TFT Standard" y "Sage Generic" — puede imitar la salida
de un TFT o un Sage hacia un generador de caracteres existente. **SNMP y
syslog: no encontrados** en este manual; no se descarta que existan en el
DASDEC-III o firmware más nuevo.

**Driver probable:** `dasdec` (HTTP); `gpi-gpio` (GPO/GPI). **Librería:**
`net/http`, `go.bug.st/serial`. **Estado: parcial.**

---

## Gorman-Redlich EAS-1 / EAS-1CG

Fabricante pequeño, común en radio y TV de baja potencia por precio.
Manual: `gorman-redlich.com/PDF/EASManual.pdf` (el sitio del fabricante
falla el TLS a un fetch directo; se leyó vía un lector externo, y el
contenido coincide con lo indexado en ManualsLib).

**Serial:** 1200 baudios. Puerto "COMPC" (programación/registro), "COM1"
libre, "COM2" para rótulo/letrero, "COM3" para módem; terminales RS-485
"para futuro equipo de control remoto", no confirmado que estén activas.
**Relés (5 en total):** "Control Out – Alert" (cierra 1 s al terminar las
tres ráfagas de FSK del EOM), "Control Out – Send Alert" (energizado
mientras se envía la alerta), "Control Out – EAS Complete" (1 s al
terminar la interrupción). **Sin red:** no hay HTTP, SNMP ni syslog
documentados. **Configuración:** un programa Windows (reemplazo de
`EASETUP.EXE` de DOS) sube la config por "COMPC" — carga de archivo, no
protocolo en caliente.

**Driver probable:** `gpi-serial` (no tiene el framing propio de Sage);
`gpi-gpio`. **Librería:** `go.bug.st/serial`. **Estado: parcial** — baudios
y relés confirmados; el set de comandos exacto del puerto de programación
no se pudo leer completo. No inventar el protocolo binario sin el manual
en mano o una captura real.

---

## TFT EAS 911 / EAS911+ / EAS911D

Fabricante veterano (ver `bh.hallikainen.org`), equipo simple: relés y
serial, sin red. El EAS911+ es la versión más reciente (manual FCC ID
BIOEAS911PLUS, 82 páginas).

**Lo público es limitado.** Los enlaces de manual (`usermanual.wiki`,
`manuals.plus`) bloquearon el acceso automático con verificación anti-bot;
no se pudo leer el contenido técnico completo. Consistente entre FCC y
ManualsLib: tiene relés y un puerto serial (usado, entre otras cosas, para
alimentar el canal de audio del EAS 930A hacia el propio EAS911+); ninguna
mención de red en ningún listado revisado.

**Driver probable:** `gpi-serial` + `gpi-gpio`, sin confirmar. **Librería:**
`go.bug.st/serial` si se confirma. **Estado: no verificado en detalle.**

---

## TFT EAS 930A (receptor monitor, no es un ENDEC)

Recibe las fuentes (NOAA, otra emisora) que un ENDEC necesita escuchar, y
entrega esa audio a las entradas "Monitor 1/2" de un Sage, DASDEC o TFT
EAS911+. El manual (ManualsLib, 53 páginas) tuvo el mismo bloqueo anti-bot;
el índice confirma que es multicanal ("Multi Module Receiver") con salida
de audio hacia el encoder/decoder. Importa para el driver de retorno de
CAtv, no para el de alertas: no reporta nada digital, solo alimenta audio
al Sage. No aplica ningún driver de esta familia. **Estado: no accesible
en detalle.**

---

## Otros fabricantes del PRD/README

**Monroe Electronics** es Digital Alert Systems hoy (ver DASDEC).
**Trilithic / Viavi**: se buscó explícitamente para cable/LPTV; ningún
producto EAS de ninguno de los dos apareció — **no encontrado**. Si CAtv o
algún LPTV tiene uno, hace falta el modelo exacto para investigarlo.

---

## Tabla resumen

| Modelo | Qué hace | Conexión | Protocolo / documentación | Driver probable | Librería Go | Estado |
|---|---|---|---|---|---|---|
| Sage 1822 | ENDEC | Serial (6 puertos) + 3 relés | Formato Sage propio; manual público completo | `sage-endec` + `gpi-gpio` | `go.bug.st/serial` | Verificado |
| Sage 3644 | ENDEC + red | Serial + relés (4 GPO/5 GPI) + LAN | HTTP/correo confirmados; syslog/SNMP no confirmados | `sage-endec` + `gpi-gpio`; `syslog`/`snmp-trap` si se confirman | `go.bug.st/serial`, `net/http`, `gosnmp`? | Parcial |
| DASDEC I/II/III/1000 | ENDEC + red | HTTP + 2 GPO/2 GPI (+ módulos red) + serial | Web completa; SNMP/syslog no encontrados | `dasdec` + `gpi-gpio` | `net/http`, `go.bug.st/serial` | Parcial |
| Gorman-Redlich EAS-1 | ENDEC | Serial 1200 baud + 5 relés | Manual del fabricante; comandos de relé sí, serial completo no | `gpi-serial` + `gpi-gpio` | `go.bug.st/serial` | Parcial |
| TFT EAS911+/911D | ENDEC | Serial + relés | Existencia confirmada (FCC/ManualsLib); técnico bloqueado | `gpi-serial` + `gpi-gpio` | `go.bug.st/serial` | No verificado |
| TFT EAS 930A | Receptor monitor (no ENDEC) | Audio hacia el ENDEC | No aplica driver de alertas | — | — | No accesible |
| Monroe Electronics | — | — | Es Digital Alert Systems hoy | ver DASDEC | — | N/A |
| Trilithic / Viavi | — | — | Ningún producto EAS encontrado | — | — | No encontrado |

---

## (a) Qué pedirle al ingeniero de CAtv sobre su Sage

1. **Modelo exacto** — 1822 o 3644 (el 3644 trae red, el 1822 no); está en
   la etiqueta frontal o en el menú del panel.
2. **Si el serial está libre**, y cuál de los seis puertos — cada uno tiene
   baudios fijo distinto, y algunos pueden estar tomados por letrero LED,
   generador de caracteres o control remoto.
3. **Pinout real del bloque verde de relés** — qué terminales están
   cableados hoy, a qué equipo, con qué función (¿programas de fábrica, o
   reprogramados?). Sin esto no se sabe qué señal cruzar.
4. **Firmware/versión de software** — determina si el 3644 tiene las
   funciones de red más nuevas (CAP 1.2).
5. **Si tiene red** — IP, si está en la red de la estación o aislado, y si
   el puerto de automatización o la web están habilitados (apagados de
   fábrica por defecto).
6. **Si hay una entrada "Manual Override" cableada** — el mecanismo exacto
   para el "commercial tally" que ya describe ADR 0010.

---

## (b) Protocolo SAME — solo como lectura

**Referencia normativa.** 47 CFR 11.31 define el protocolo EAS: FSK a
520.83 baudios, marca 2083.3 Hz / espacio 1562.5 Hz, ASCII de 7 bits con
un octavo bit nulo. Cuatro partes: cabecera SAME, señal de atención
(853+960 Hz simultáneos), audio del mensaje, cabecera de fin de mensaje
(`NNNN`). Texto completo:
[ecfr.gov — 47 CFR 11.31](https://www.ecfr.gov/current/title-47/chapter-I/subchapter-A/part-11/subpart-B/section-11.31).

**Proyectos abiertos que lo decodifican — para leer el algoritmo, no para
enlazar ni copiar** (mismo criterio que ADR 0010 con `dsame`): `dsame` /
`dsame3` (Python, la referencia más legible), `multimon-ng` (su modo EAS,
en C, es el demodulador que muchos proyectos de SDR usan), `sameold` /
`samedec` (Rust, pensado como reemplazo directo del modo EAS de
`multimon-ng`). Lo útil para `signal-compare` (ADR 0010, capa 2) es la
idea — demodular FSK a 520.83 baudios y parsear `ZCZC-...-NNNN` — no el
código de ninguno de los tres.
