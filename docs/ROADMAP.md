# Roadmap — las fases y cuánto tardan de verdad

Este archivo dice qué entrega cada fase, en qué orden van, cuánto tardan con
los ojos abiertos, y cómo se decide que una terminó.

No promete fechas. Publica las suyas, con tres escenarios y el supuesto de
ritmo a la vista, para que la conversación incómoda del mes catorce no exista.

---

## 1 · Estado actual

**Estamos en F0, y F0 no es una fase de producto: es un experimento con
criterio de pase o fallo.**

| | |
|---|---|
| **Construida** | El experimento completo: fabricación de los archivos de prueba, motor (decodificador por clip → servidor de cuadros en Go → encoder persistente), las dos salidas simultáneas, y el analizador que mide. |
| **Corrida corta** | **La corrida de 6 minutos pasa** todo lo que se puede medir en seis minutos. |
| **Corrida larga** | **En curso.** La de 8 horas es la que decide. |
| **Dónde falta correrla** | **En la PC Windows del despliegue de referencia**, que es donde tiene que correr — no en la laptop del desarrollador. |
| **Lo que F0 no prueba** | Los subtítulos CEA-608 reales (el archivo de prueba es sintético) y el máximo de sesiones de encoder por hardware, que es una medición manual en la máquina de destino. Se marcan "—", nunca "pasa". |

**Hoy Antena787 no sirve para emitir nada.** No hay instalador, no hay
interfaz, no hay base de datos. Eso empieza en F1.

---

## 2 · Las fases

### F0 · Prueba de concepto

**Responde la pregunta técnica más cara del proyecto:** ¿el diseño "servidor de
cuadros en Go entre decodificadores por clip y un encoder persistente" produce
salida continua y limpia durante horas en hardware modesto?

Nueve casos de prueba —diez archivos: el del audio son dos, mono y 5.1— con problemas de verdad —cambio de resolución, cambio
de cuadros, material de archivo, cuadros PAL en un canal NTSC, cuadros
variables, mono y 5.1, subtítulos embebidos, audio 200 ms más corto que el
video, y uno corrupto en el último segundo— normalizados al formato de casa
720p59.94 y encadenados en bucle **8 horas** hacia dos salidas simultáneas de
volumen distinto. Una de ellas en **el formato exacto que recibe el multiplexor
de CAtv**: MPEG-2 720p59.94, audio MPEG capa II y AC-3, CBR por UDP.

También **convierte en medidas las cifras estimadas de hardware** del PRD §18,
que es lo que corrige `COMPRAR.md`.

> **Son cinco días de trabajo, no cinco días de calendario**, y hay que
> presupuestar una segunda corrida de 8 horas: la primera casi nunca es la
> buena.

**Si falla el desfase o los cuadros perdidos, el diseño del servidor de cuadros
se replantea antes de la F1.** Cinco días de trabajo en F0 valen más que doce
semanas construidas sobre un supuesto.

### F1 · Fundación — **no toca el aire**

- Esquema con `channel` en cada tabla desde el día uno.
- Contenido con **medición real** y fichas desde etiquetas, `.nfo`, Cover Art
  Archive y TVmaze (sin clave que nadie tenga que sacar).
- **Editor de reglas y línea de tiempo** — reglas como entrada, línea de tiempo
  como vista.
- El **resolver**, que materializa el plan 48 horas por adelantado.
- **Guía validada** (XMLTV), que se regenera con cada cambio de plan.
- **Avisos de vencimiento** a 30, 14 y 7 días.
- **Plan y guía revisables** — el modo sombra en su forma de F1: Antena787
  propone y un humano compara a ojo contra lo que emite hoy.

*La comparación automática contra el aire real necesita la grabación, que llega
en F2.*

### F2 · Playout — la fase grande

Motor completo · conformado · **los cuatro decks y su prioridad** · **salida
`udp-ts`**, la del multiplexor · **fuentes en vivo por SRT/RTMP**, con
identificativos, cortinillas y música programados adentro · **manual con
regreso automático** · **grabación de la salida y diferido** · **logo del
canal** · asistente de instalación con la prueba de barras · cascada de
respaldo · relleno · tablero · alarmas · **registro de incidentes**.

*Los cortes **vendidos** no están aquí: llegan en F4.*

Termina con una **prueba de resistencia de 30 días** con instrumentación de
deriva audio-video. Desglose interno en la sección 4.

### F2.5 · Endurecimiento

Que un **Windows 10 sin presupuesto aguante 24/7**: exclusiones de antivirus
verificadas, NTP, política de Windows Update, vigilancia permanente en Ajustes,
umbrales de disco, recuperación de la base, respaldos.

**Corre en paralelo al soak de 30 días**, porque es justo lo que el soak pone a
prueba.

> **El despliegue de referencia necesita esta fase aunque el proyecto nunca se
> publique.** Por eso no vive dentro de "abrir el repositorio": son dos trabajos
> distintos que antes estaban en la misma fila de una tabla.

### F3 · Público

Repositorio abierto · CI · instaladores · documentación bilingüe · **matriz de
hardware** · **guía de compra** · lo legal. Modo `internet` completo.

**Solo eso**: el endurecimiento ya pasó en F2.5.

> **F3 va antes que F4 a propósito.** Un canal universitario no necesita
> publicidad — necesita que el proyecto exista y funcione.

### F4 · Emisora

Publicidad con prioridad y rotación · **el deck comercial: cortes vendidos,
dentro y fuera de bloques en vivo** · **crawl de clasificados** · **portal del
anunciante: enlace, entrega y cobro** · **SCTE-104**, con su prueba de 48 a
72 horas · reconciliación con las alertas de emergencia · evidencia de emisión.
Perfil `us-fcc`.

### F4b · Radio

Formato de casa **solo audio** · salida de audio · el resto ya funciona tal
cual: reglas, plan, as-run, cortes, fuentes en vivo, grabación, diferido y
manual son idénticos.

**Para el despliegue de referencia no es "después"**: hay una emisora de FM en
el mismo sitio.

**Sin rotación musical** — sirve para radio hablada, deportiva y de programas.

### F4c · Rotación musical

Categorías, relojes por hora y reglas de separación —que no se repita el mismo
artista en tres horas, que no caigan dos baladas seguidas—.

**Es lo que falta para radio musical, y es un subsistema propio, no un ajuste.**

### F5 · Internacional

Perfiles `eu-ebu` e `isdb-latam` · subtítulos DVB y ARIB · exportación PSIP y
DVB-SI. Y aquí, si alguna vez, la ruta `inyeccion-ts` de señalización de cortes
— **marcada sin precedente documentado**.

**Libre, en el núcleo.**

> **Al terminar F5 el producto está completo.**

### F5b · Escala

Multi-canal · roles · redundancia · **API REST completa e integraciones** ·
Postgres opcional. **Libre, como todo.**

### F6 · MCP

**La última fase, y solo cuando todo lo anterior funcione sin ella.**

F6 es aditiva: si nunca se hiciera, Antena787 seguiría siendo un sistema
entero. *Esa es la prueba de que la IA está en el lugar correcto.*

---

## 3 · Cuánto tarda esto de verdad

**El supuesto que manda:** unas **10 horas a la semana**, que es más o menos
**un día de trabajo efectivo**. No es una queja: es el ritmo real de un
proyecto que se hace al lado de un trabajo. Todo lo de abajo sale de ahí.

| Hito | Optimista | Probable | Pesimista |
|---|---|---|---|
| **F0 decidida** | 2 semanas | **1 mes** | 2 meses |
| **F1 · modo sombra** (Antena787 propone, un humano compara) | 7 meses | **~1 año** | 1 año y medio |
| **F2 · madrugadas al aire** | 2 años | **~3.4 años** | 4 años y medio |
| **F2.5 + soak · aire completo** | 2 años y medio | **~3.9 años** | 5 años |
| **F4 · primer anunciante cobrado** | 4 años | **~5.6 años** | 7 años |

**Esto no es pesimismo, es aritmética.** ffplayout —un proyecto más chico, sin
publicidad, sin portal y sin cumplimiento— tomó más de dos años en llegar a
algo estable.

> ### La única palanca real es más horas en F1 y F2
>
> **Recortar F4 no adelanta el aire.** La publicidad viene *después* del aire;
> quitarla no mueve la fecha de salida ni un solo día. Lo mismo con F4b, F4c,
> F5, F5b y F6: todo eso está detrás de la línea del aire.
>
> **Duplicar el ritmo a 20 horas semanales sí parte los números por la mitad.**
> Esa es la decisión que hay que tomar con los ojos abiertos, y es la única que
> mueve el calendario.

---

## 4 · F2 por dentro

Una sola fila de tabla escondía **cuatro o cinco subsistemas, cada uno del
tamaño de F1**, y todos con revisión humana línea por línea: son unas **7,000
líneas críticas**, que son unas **35 horas solo de leerlas**.

Este es el orden, y cada paso se apoya en el anterior:

1. **Motor y conformado** — el servidor de cuadros, el pre-roll, el encoder
   persistente. Es lo que valida la F0.
2. **Los decks y su prioridad** — quién tiene el aire en cada instante.
3. **La salida `udp-ts`** — la del multiplexor, la primera que existe.
4. **El detector de silencio y negro sobre la salida** — porque sin él nada de
   lo anterior se puede verificar solo.
5. **Fuentes en vivo** — SRT/RTMP, margen de gracia, reconexión, elementos
   programados adentro.
6. **Manual** — tomar y soltar el control, la escalera de regreso, el único
   tenedor.
7. **Grabación y diferido** — con el retorno de aire.
8. **Cascada y watchdog** — el último escalón, y el que vigila al encoder.

**Los ocho no se pueden reordenar mucho**, y ninguno se puede saltar sin dejar
el canal sin una de las promesas del proyecto.

---

## 5 · Cómo se decide pasar de fase

**Una fase termina cuando sus criterios de aceptación pasan, no cuando el
código está escrito.**

Los criterios viven en **[`docs/ACEPTACION.md`](ACEPTACION.md)**, con IDs por
fase (`F0-01`, `F1-14`, `F2-33`…). Cada uno está en formato **Dado · Cuando ·
Entonces**, y está marcado **[AUTO]** —se puede convertir en una prueba
automatizada— o **[MANUAL]** —necesita a una persona, porque involucra ver,
oír, o hardware real—. Un criterio manual sigue siendo una verificación de un
minuto, no una revisión abierta.

**La puerta de F0 es explícita y es de pase o fallo:** si falla la
discontinuidad de audio, el desfase de 20 ms o los cuadros duplicados o
perdidos (criterios F0-01 a F0-04), el diseño del motor se replantea y **F1 no
arranca** hasta resolverlo (F0-09).

**La puerta de F2 hacia el aire completo es la prueba de resistencia de 30
días**, con la instrumentación de deriva puesta, corriendo en la máquina real.
No es una demostración: es un mes.

Y hay una regla que no depende de ninguna fase: **el motor lo escribe una IA y
lo revisa un humano, línea por línea.** Eso no es negociable y no se salta por
calendario.
