# Matriz de compatibilidad

*"¿Sirve con mi equipo?"* es la primera pregunta de todo el que llega, y esta
tabla es la respuesta. **La mantiene la comunidad**, no el autor: cada fila la
puso alguien que conectó ese equipo y contó cómo le fue.

---

## Cómo leer el estado

| Estado | Qué significa |
|---|---|
| **probado** | Alguien lo conectó a Antena787 y funcionó. La fila dice quién y cuándo. |
| **reportado** | Alguien dice que funciona, pero no está verificado por el proyecto. Vale, y se marca como lo que es. |
| **pendiente** | Está en la matriz porque se sabe que existe y se espera que funcione, pero **nadie lo ha conectado todavía**. |

**Hoy casi todo está en *pendiente*, y eso es honesto.** El sistema todavía no
emite (ver [`docs/ROADMAP.md`](../ROADMAP.md)): lo único que ha corrido es el
experimento de la Fase 0, y ni siquiera ha corrido aún en la PC del despliegue
de referencia. **Ninguna fila puede decir "probado" hasta que alguien lo pruebe
de verdad.**

> **Ninguna marca ni modelo se asume.** El despliegue de referencia es donde se
> prueba primero, no el molde. Cada equipo que aparece aquí es un ejemplo de
> una **familia** que tiene driver, y el driver se elige **por cómo se llega al
> equipo** —cable serial, cable de relés, cable de red, USB— no por la marca. La
> marca solo carga la tabla de nombres.

---

## La matriz

### Máquina

| Equipo | Marca y modelo | Cómo se conecta | Estado | Reportó | Fecha | Notas |
|---|---|---|---|---|---|---|
| Máquina de playout | PC Windows 10, SSD M.2 500 GB + HDD 3 TB | — | pendiente | Rolando (CAtv) | 2026-09-08 | Sistema y base en el SSD; biblioteca y grabación en el HDD. Sin presupuesto: sin IoT LTSC y sin segunda máquina. Es donde falta correr la Fase 0. |

### Encoder / multiplexor

| Equipo | Marca y modelo | Cómo se conecta | Estado | Reportó | Fecha | Notas |
|---|---|---|---|---|---|---|
| Multiplexor | **Technalogix TP1000** | `udp-ts` por Ethernet (UDP/RTP), sale ASI | pendiente | Rolando (CAtv) | 2026-09-08 | 2 entradas ASI, 2 salidas ASI, 2 puertos de red (uno de manejo, otro para el stream de entrada). Recibe hoy 720p59.94 MPEG-2 con audio MPEG capa II. Junta el programa con lo demás y saca el ASI. |

### Transmisor (excitador y amplificador)

| Equipo | Marca y modelo | Cómo se conecta | Estado | Reportó | Fecha | Notas |
|---|---|---|---|---|---|---|
| Excitador | **RVR Blue Digital Video** | Entrada ASI · red de manejo · USB · RF MONITOR | pendiente | Rolando (CAtv) | 2026-09-08 | 605 MHz, multiestándar. **No recibe transporte por IP** (los modelos nuevos sí), por eso la única puerta es `udp-ts` al multiplexor. El RF MONITOR puede alimentar un retorno de aire por entrada de captura. |
| Amplificador | **ADR ~300 W** | Telemetría por USB, red o contactos — a confirmar | pendiente | Rolando (CAtv) | 2026-09-08 | Muestra potencia directa y reflejada, corriente, voltaje y temperatura. Driver `serial-usb`, `snmp` o `http`; el asistente prueba los que tengan cable. Solo lectura. |

### ENDEC (equipo de alertas de emergencia)

| Equipo | Marca y modelo | Cómo se conecta | Estado | Reportó | Fecha | Notas |
|---|---|---|---|---|---|---|
| ENDEC | **Sage Digital ENDEC** | Relés en el bloque verde de atrás · serial RS-232 al frente | pendiente | Rolando (CAtv) | 2026-09-08 | Audio XLR de entrada y salida; entra al multiplexor por audio y video compuesto. **Modelo exacto (1822 o 3644) pendiente de confirmar por el ingeniero** — no hace falta antes de instalar: el asistente prueba serial, relés y red y usa lo que responda. `gpi-serial` sobre los relés es lo primero que se prueba. |
| Receptor de monitoreo | **TFT EAS 930A** | — (alimenta al Sage) | pendiente | Rolando (CAtv) | 2026-09-08 | Monitorea WCMN 1280 AM, WKAQ-FM 104.7 y NOAA canal 1. No se le habla directamente: es la fuente del Sage. |

### Retorno de aire

| Equipo | Marca y modelo | Cómo se conecta | Estado | Reportó | Fecha | Notas |
|---|---|---|---|---|---|---|
| Tarjeta receptora de TV | modelo por confirmar | `capture_input` tipo `receptor-tv`, en la misma máquina | pendiente | Rolando (CAtv) | 2026-09-08 | Captura la señal **después** del ENDEC — que es el único sitio donde se puede saber qué salió de verdad *(ADR 0009)*. También alimenta el monitor por streaming. |

### Tarjeta de captura

*(Sin filas todavía. Una entrada de captura alimentada por el RF MONITOR del
excitador o por un receptor externo es la otra forma de hacer el retorno de
aire.)*

---

## Familias con driver previsto

Estas no son filas de la matriz: son las familias que el proyecto trae de
entrada porque en Estados Unidos los equipos son pocos y se conocen. **Que
tengan driver previsto no significa que estén probadas** — significa que
cuando alguien las conecte, hay código esperándolas.

### Alertas de emergencia

| Familia | Por dónde se llega |
|---|---|
| **Sage Digital ENDEC 1822** | serial RS-232 y relés |
| **Sage Digital ENDEC 3644** | serial, relés, **y red** (HTTP y syslog) |
| **DASDEC** (Digital Alert Systems) | red (HTTP, SNMP, syslog) y relés |
| **Gorman-Redlich** | relés y serial |
| **TFT** | relés |

**No hace falta saber el modelo antes de instalar.** El asistente pregunta *por
dónde está conectado* —cable serial, cable de relés, cable de red, o varios— y
prueba cada uno. Si hay dos caminos, se usan los dos y se cruzan.

Y por encima de todos: **`signal-compare`**, que no le habla al equipo — compara
la señal transmitida contra lo que el plan decía. **Funciona con hardware que
este proyecto nunca va a tener en la mano**, incluido el EWBS de Sudamérica.
Necesita retorno de aire; sin él corre en modo `degradado` y lo dice.

### Transmisores (telemetría, solo lectura)

**SNMP es el común denominador**, y lo que no habla SNMP suele tener página web
o puerto serial/USB.

| Ámbito | Marcas |
|---|---|
| **Televisión** | GatesAir · Rohde & Schwarz · Anywave · Comark |
| **Radio** | Nautel · Broadcast Electronics · Elenos |

Los caminos disponibles son `snmp`, `http` (la página web del equipo),
`serial-usb` y `gpi-estado` (contactos de estado: al aire, falla, reflejada
alta). **Se traen todos y se pueden combinar.**

Lo que se lee es lo que el equipo ya muestra en su pantalla —potencia directa y
reflejada, corriente, voltaje, temperatura— y **es la única respuesta física a
"¿estamos al aire?"**: potencia directa en cero es fuera del aire, diga lo que
diga la red.

---

## Cómo añadir una fila

**Si conectaste tu equipo, cuéntalo.** Da igual si funcionó, si funcionó a
medias o si no funcionó: las tres cosas son útiles, y una fila que dice "no
funcionó, y esto fue lo que pasó" vale tanto como una que dice que sí.

**Dos maneras, la que te quede cómoda:**

1. **Un issue** con la plantilla `hardware.yml` — se llena un formulario, no
   hace falta tocar este archivo ni saber git.
2. **Un pull request** que añada la fila directamente a la tabla de su familia
   en este archivo.

**Qué poner en la fila:**

| Columna | Qué va |
|---|---|
| **Equipo** | Qué es, en palabras claras: "multiplexor", "ENDEC", "amplificador". |
| **Familia** | Máquina · encoder/multiplexor · transmisor · ENDEC · retorno de aire · tarjeta de captura. Es la sección donde va la fila. |
| **Marca y modelo** | Lo que diga la etiqueta. Si no se lee, dilo así. |
| **Cómo se conecta** | El cable y el protocolo: "UDP por Ethernet", "relés en bloque verde", "serial RS-232", "SNMP". **Esta es la columna que más sirve**, porque el driver se elige por aquí. |
| **Estado** | probado · reportado · pendiente. Sé conservador. |
| **Quién lo reportó** | Nombre o usuario. Basta con algo con lo que se te pueda preguntar. |
| **Fecha** | AAAA-MM-DD. |
| **Notas** | Lo que te costó trabajo averiguar. El bitrate que aceptó, el PID que había que fijar, el cable que no era el que parecía, la cosa rara que hace a las 3 AM. |

> **La matriz de compatibilidad decide la adopción.** Por eso se publica en F3,
> junto con el repositorio, y no al final — y por eso una fila incompleta es
> mejor que ninguna fila.
