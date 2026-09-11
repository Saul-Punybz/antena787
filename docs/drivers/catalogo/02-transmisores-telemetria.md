# Catálogo: transmisores y su telemetría

Esto es investigación, no una interfaz de driver (PRD [§10](../../../PRD.md#10--los-drivers),
[README de drivers](../README.md)). Cubre marcas y familias de LPTV, Class A
y potencia media que se usan en estaciones chicas de EEUU y PR, y cómo se les
lee la telemetría —potencia directa/reflejada, temperatura, corriente,
alarmas— sin hablar con el equipo en la mano. Cada fila trae su fuente. Donde
no se encontró nada público, dice **no encontrado**, no una suposición.

**El driver se elige por cómo se llega, no por marca** (PRD §10). Esta lista
sirve para saber qué esperar de cada fabricante antes de preguntarle al
ingeniero, no para elegir por adelantado.

## La cadena de Rolando (referencia, PRD §17 y §25)

```
Technalogix TP1000 (multiplexor) ──ASI──▶ excitador RVR Blue Digital Video (605 MHz) ──▶ amplificador ADR (~300 W) ──▶ antena
```

El TP1000 no es transmisor de RF —es el multiplexor que junta programas en
un ASI—, pero está en la misma cadena y es del mismo fabricante que probablemente
sea el del amplificador. El excitador RVR y el amplificador "ADR" son la
parte de la que hace falta telemetría.

## RVR Elettronica (Italia)

| Campo | Detalle |
|---|---|
| Modelo | **Blue Digital Video** — excitador digital TV (familia DVB-T/ATSC 8VSB de RVR). No se encontró una ficha pública con ese nombre exacto; la línea de excitadores digitales de RVR sí existe y sí ofrece telemetría opcional por TCP/IP y SNMP. |
| Potencia y uso | Excitador (salida baja, alimenta al amplificador). LPTV/Class A digital. |
| Cómo se lee | Tres caminos posibles, **ninguno confirmado para este modelo exacto**: (1) SNMP y TCP/IP como opción de fábrica en excitadores digitales RVR; (2) **RDMODWEB**, accesorio de RVR que expone la telemetría de sus excitadores por página web sobre LAN/Ethernet 10Base-T; (3) **TLK300**, el sistema de telemetría separado de RVR para "transmisores escalables" — SNMP v2, más RS-232/RS-485/I2C hacia el equipo, y conectividad IP o GSM. |
| USB | El PRD (§25) dice que el RVR tiene "USB y red de manejo". No se encontró documentación pública de qué protocolo habla ese USB — podría ser mantenimiento/firmware y no telemetría. **Sin confirmar.** |
| Dónde está la MIB o el manual | Fichas generales en manualslib.com y broadcaststoreeurope.com; el sitio oficial (rvr.it/en/documentation) lista manuales por modelo pero no se encontró un MIB descargable sin contacto directo con RVR. |
| Driver PRD | `snmp` (excitador o TLK300) · `http` (RDMODWEB) · `serial-usb` (si el USB resulta ser puerto serie) |
| Librería Go sin CGo | `gosnmp` (SNMP) · `net/http` estándar (web) · `go.bug.st/serial` (si es serie) |
| Estado | **No público / no encontrado** para el modelo exacto. La existencia de SNMP y de una interfaz web en la línea RVR sí está verificada por el fabricante. |

Fuentes: [RVR Blue Digital 1HE TV Transmitter — BroadcastStoreEurope](https://broadcaststoreeurope.com/shop/603-rvr-elettronica-tv-transmitters/2673-rvr-blue-digital-1he-tv-transmitter-12-w/) (10 sep 2026) · [RVR TLK300/TLK2000 — rvr.it](https://www.rvr.it/en/products/components/telemetry-units-system/tlk300-series/tlk300-tlk2000-nsn2ss9y/) (10 sep 2026) · [RVR RDMODWEB manual — manuals.plus](https://manuals.plus/r-v-r-elettronica/rdmodweb-fm-transmitter-manual) (10 sep 2026) · [RVR — Documentación oficial](https://www.rvr.it/en/documentation/) (10 sep 2026).

## Amplificador "ADR" — sin confirmar la marca

No se encontró ningún fabricante de RF registrado bajo el nombre "ADR" ni
"ADR Broadcast". La hipótesis más fuerte, por contexto: **ADRENALIN**, el
sistema de control de los amplificadores de **Technalogix** (Canadá) —la
misma casa que fabrica el TP1000 que ya está en la cadena. Los amplificadores
TAUD/TAVD de Technalogix corren bajo control "Adrenalin" y muestran en
pantalla exactamente lo que el PRD describe del ADR: potencia directa y
reflejada, temperatura del disipador, voltaje.

**Esto es una hipótesis razonable, no un hecho verificado.** Puede ser
Technalogix, puede ser una marca distinta que no apareció en la búsqueda, o
un nombre interno/de instalador. Se resuelve preguntándole al ingeniero.

| Campo | Detalle (si la hipótesis Adrenalin/Technalogix es correcta) |
|---|---|
| Potencia y uso | ~300 W, amplificador de potencia media, LPTV/Class A. |
| Cómo se lee | Pantalla táctil local 4.3" WQVGA (según manual TAUD-100) + SNMP + Ethernet + puerto paralelo. Monitorea potencia directa/reflejada en el acoplador direccional y temperatura del disipador por un módulo sensor. |
| Dónde está la MIB o el manual | El manual de TAUD-100 menciona "Technalogix Management Information Base" para SNMP, pero no se encontró un enlace público de descarga — hay que pedirlo al fabricante. Manuales del TXUD1000/TAUD-100 en manualslib.com y fccid.io (por el filing de FCC). |
| Driver PRD | `snmp` o `http`, pendiente de confirmar contra el equipo real. |
| Librería Go sin CGo | `gosnmp` · `net/http` estándar |
| Estado | **Hipótesis, no verificada.** Ver "qué pedir al ingeniero" abajo — es la pregunta más importante de este documento. |

Fuentes: [Technalogix TXUD1000 — manual, fccid.io](https://fccid.io/QH5TXUD1000/User-Manual/Users-Manual-1472269) (10 sep 2026) · [Technalogix TAUD-100 manual — manualslib.com](https://www.manualslib.com/manual/1548394/Technalogix-Taud-100.html) (10 sep 2026) · [Technalogix — digital TV amplifiers](https://technalogix.com/en-us/pages/digital-tv-amplifiers) (10 sep 2026).

## Technalogix (Canadá) — línea completa

| Campo | Detalle |
|---|---|
| Potencia y uso | Transmisores y amplificadores digitales de TV, de cientos de W a varios kW; también FM. El TP1000 de la cadena de Rolando es su multiplexor. |
| Cómo se lee | Pantalla táctil a color en el equipo, más SNMP, Ethernet y puerto paralelo — los tres caminos de control, según el manual TAUD-100/TXUD1000. |
| Dónde está la MIB o el manual | Manuales por modelo en manualslib.com, fccid.io (filings de FCC) y usermanual.wiki. La "Technalogix Management Information Base" se menciona en el manual pero no hay descarga pública confirmada — se pide al fabricante. |
| Driver PRD | `snmp` · `http` |
| Librería Go sin CGo | `gosnmp` · `net/http` estándar |
| Estado | **Verificado** que SNMP, Ethernet y pantalla existen en la línea. MIB **no público**. |

Fuente: [Technalogix TXUD1000 — usermanual.wiki](https://usermanual.wiki/Technalogix/TXUD1000/html) (10 sep 2026).

## Otras marcas de la lista del PRD §10

| Marca / línea | Potencia y uso típico | Cómo se lee | MIB / manual | Driver PRD | Librería Go | Estado |
|---|---|---|---|---|---|---|
| **GatesAir Maxiva** (EEUU) | UHF, LPTV hasta alta potencia | SNMP (MIB del fabricante) + interfaz web | MIB no descargable en público; se pide a tsupport@gatesair.com. Ejemplo de integración de terceros (DataMiner) confirma el uso real de SNMP. | `snmp` · `http` | `gosnmp` | **Verificado** (SNMP existe); MIB no público |
| **Rohde & Schwarz** | TV y FM, media/alta potencia | SNMP como opción de control remoto, o interfaz paralela | "Rohde & Schwarz MIBs" mencionados en su documentación de aplicación, sin descarga pública encontrada | `snmp` | `gosnmp` | **Verificado** (SNMP existe); MIB no público |
| **Anywave** (China) | LPTV/HPTV, 1 W–150 kW, multi-estándar | SNMP + Web Server; mide directa/reflejada y temperatura de sala | Folletos de producto públicos (Magma, Marble, Slate); no hay MIB descargable encontrada | `snmp` · `http` | `gosnmp` | **Verificado** (SNMP existe); MIB no público |
| **Elenos / Screen Service** | FM y TV, todo rango de potencia | **eBox**: HTTP + SNMP unificado para transmisor, excitador, amplificador y sistema de protección, en un solo punto | **MIB pública descargable** en el portal de soporte (support.elenosgroup.com), separada por versión de firmware de telemetría (1.06 a 1.09+) | `snmp` · `http` | `gosnmp` | **Verificado**, y es el caso con **mejor documentación pública** de todos los de esta lista |
| **Hitachi-Comark** | UHF, media/alta potencia (E-Compact) | Web-GUI + SNMP integrados de fábrica | No se encontró MIB pública; solo notas de producto | `snmp` · `http` | `gosnmp` | **Verificado** (SNMP/web existen); MIB no público |
| **Electrolink** | UHF, media/alta potencia | No encontrado | No encontrado | — | — | **No encontrado** |
| **DB Broadcast / Egatel** | Egatel (España): DVB-T/T2, ATSC, ISDB-T, casi toda su línea (TVW6000, TVH4000, TUWH4000, TUWH1000, TLWH7900E) | **SNMP, Web Server o contactos secos**, según serie — las tres rutas convivan en el mismo equipo | Fichas de producto públicas por modelo en egatel.es; no se encontró MIB descargable | `snmp` · `http` · `contactos` | `gosnmp` | **Verificado** (por ficha de producto); MIB no público |

Fuentes: [GatesAir Maxiva XTE — DataMiner Docs](https://docs.dataminer.services/connector/doc/GatesAir_Maxiva_XTE_Transmitter.html) (10 sep 2026) · [Rohde & Schwarz — SNMP application note](https://www.rohde-schwarz.com/us/applications/simple-network-management-protocol-remote-controlling-for-monitoring-devices-application-note_56280-15482.html) (10 sep 2026) · [Anywave Magma brochure](https://anywavecom.net/wp-content/uploads/2022/04/Anywave-HPTV-VHF-I-III-Transmitters-Product-Brochure-4-22.pdf) (10 sep 2026) · [Elenos FM MIB Files — support.elenosgroup.com](https://support.elenosgroup.com/en/support/solutions/articles/48001077864-elenos-fm-mib-files) (10 sep 2026) · [Elenos eBox](https://www.elenos.com/remote-control-ebox/) (10 sep 2026) · [Hitachi-Comark — TV Tech](https://www.tvtechnology.com/equipment/new-hitachi-comark-all-in-one-transmitter-targets-nextgen-tv) (10 sep 2026) · [Egatel — productos](https://www.egatel.es/) (10 sep 2026), [TVW6000](https://www.egatel.es/producto/tvw6000-series), [TUWH1000](https://www.egatel.es/wp-content/uploads/2022/04/tuwh1000-series-2023-06-19-tuwh1000-series.pdf) (10 sep 2026).

**Nota aparte, útil como referencia de "cómo se consigue una MIB":**
Broadcast Electronics (bdcast.com, hoy parte del mismo grupo que Elenos)
publica sus MIBs de excitadores UHF/VHF y sus MIB de Elenos FM directamente
en su portal de soporte (support.bdcast.com), sin pedir contacto comercial
previo. Es el modelo a imitar cuando se le pregunte a cualquier otro
fabricante dónde está su MIB. Fuente: [UHF VHF Exciter & CCU SNMP v2 MIB Files — support.bdcast.com](https://support.bdcast.com/en/support/solutions/articles/48001210548-uhf-vhf-exciter-ccu-snmp-v2-mib-files) (10 sep 2026).

## Estándares

- **SNMP v1/v2c/v3.** El más común entre estos fabricantes es **v2c**
  (RVR TLK300 y Broadcast Electronics lo dicen explícito). v3 aparece en
  librerías Go modernas (`gosnmp` soporta v1, v2c y v3) pero no se confirmó
  que ningún fabricante de esta lista lo use por defecto — v1/v2c basta para
  la mayoría de estos equipos.
- **MIB-II** es el estándar IETF de red (interfaces, IP, contadores) — lo
  trae cualquier equipo con SNMP, pero no dice nada de potencia de RF ni
  temperatura.
- **No hay una MIB común de broadcast.** No se encontró ninguna MIB unificada
  publicada por la NAB ni por ningún consorcio de fabricantes. Cada marca
  trae su propia MIB privada ("enterprise MIB", bajo su propio OID
  registrado) y el driver tiene que cargar la del fabricante específico —
  eso confirma lo que dice el PRD §10: **la marca solo carga la tabla de
  nombres**, el mecanismo (SNMP) es el mismo.
- **Contactos de estado (GPO/dry contacts)** son el último recurso cuando no
  hay red ni SNMP — Egatel los ofrece como alternativa en casi toda su
  línea, y es el mismo mecanismo que ya usa `gpi-estado` en el PRD (al aire,
  falla, reflejada alta).
- **Librerías Go verificadas, sin CGo** (ADR 0002): `gosnmp/gosnmp` para
  SNMP (v1/v2c/v3, Get/GetNext/GetBulk/Walk/Traps); `go.bug.st/serial` para
  serie/USB; `net/http` de la biblioteca estándar para lo que hable HTTP. No
  hace falta ningún SDK propietario para ninguna de las rutas de esta lista.

## Qué pedirle al ingeniero de CAtv

Esto es lo que falta para escribir un driver real en vez de una hipótesis:

1. **Modelo exacto del excitador RVR** — la placa suele traer el número de
   serie y el modelo completo, no solo "Blue Digital Video".
2. **Modelo exacto del amplificador "ADR"** — marca, modelo, y si el manual
   dice "Adrenalin" en algún lado (confirmaría la hipótesis de Technalogix).
3. **¿Tienen IP de manejo asignada hoy en ese excitador y en ese
   amplificador?** Si nunca se configuró, SNMP y web pueden estar apagados
   aunque el equipo los soporte.
4. **¿SNMP está habilitado?** Y si sí, ¿qué versión (v1/v2c/v3) y qué
   comunidad o credenciales.
5. **Versión de firmware** de ambos equipos — varias MIBs (la de Elenos, por
   ejemplo) dependen de la versión de telemetría del firmware.
6. **¿El puerto USB del RVR expone un puerto serie** (por ejemplo, se ve
   como COM en Windows), o es solo para actualizar firmware con una
   memoria/cable propietario?
7. **¿Hay contactos de alarma libres** en el amplificador o en el excitador
   —normalmente abiertos o cerrados, y para qué evento cada uno— como
   respaldo si SNMP y web fallan.
8. **¿Alguno de los dos equipos "miente"** —dice conectado o normal cuando
   no lo está— en algún estado que valga la pena documentar de entrada.
