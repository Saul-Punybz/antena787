# Estado del proyecto

**Al 8 de septiembre de 2026.**

> ## Fase 0 · Prueba de concepto
>
> **Hoy Antena787 no sirve para emitir nada.** No hay instalador, no hay
> interfaz, no hay base de datos, no hay canal, y `cmd/antena` —el ejecutable
> del producto— **todavía no existe.**

La lista de fases, en orden y con lo que entrega cada una, está en
[`docs/ROADMAP.md`](https://github.com/Saul-Punybz/antena787/blob/main/docs/ROADMAP.md).
Esta página dice **dónde estamos hoy**.

---

## Qué se está haciendo

**La Fase 0 es un experimento con criterio de pase o fallo, no un objetivo.**
Responde la pregunta técnica más cara del proyecto:

> ¿El diseño *"un servidor de cuadros en Go entre decodificadores por clip y un
> encoder persistente"* produce salida continua y limpia durante horas en
> hardware modesto?

Se corre, se mide **con herramientas y no con el oído**, y se reporta. **Si
falla, el diseño del motor se replantea antes de escribir la primera línea de la
Fase 1** — cinco días de trabajo en F0 valen más que doce semanas construidas
sobre un supuesto.

**Cómo se prueba:** nueve casos de prueba fabricados a propósito —H.264, MPEG-2, HEVC,
cuadros variables, mono y 5.1, audio 200 ms más corto que el video, uno
corrupto en el último segundo— normalizados al formato de casa 720p59.94 y
encadenados en bucle durante **8 horas**, con **dos salidas simultáneas de
volumen distinto**. Una de ellas es **el formato exacto que recibe el
multiplexor del despliegue de referencia**: MPEG-2 720p59.94, audio MPEG capa II
y AC-3, tasa constante por UDP — porque de nada sirve probar el motor en un
formato que el transmisor no va a ver. Y se corre **en la máquina de destino, no
en la laptop del desarrollador.**

**Qué tiene que pasar para que pase:** ninguna discontinuidad de audio por
encima de −40 dBFS en los cambios de clip · **menos de 20 ms de desfase
audio-video acumulado a las 8 horas** · cero cuadros duplicados o perdidos en
cada cambio · marcas de tiempo monotónicas · bitrate constante ±1 % con PCR cada
40 ms o menos y sin errores de continuidad · el archivo corrupto cae a relleno
**sin un solo cuadro negro** · y CPU y RAM **medidos**, que reemplazan las
cifras estimadas de la guía de hardware.

**Y lo que la Fase 0 deliberadamente no prueba, dicho de frente:** los
subtítulos CEA-608 —el archivo de prueba es sintético y no trae 608 reales; se
marca "—", no "pasa"— y cuántas sesiones de encoder por hardware aguanta la
máquina, que es una medición manual en el equipo de destino.

*→ [`f0/README.md`](https://github.com/Saul-Punybz/antena787/blob/main/f0/README.md)
· [`PRD.md` §22.1](https://github.com/Saul-Punybz/antena787/blob/main/PRD.md#221--la-f0-definida-como-experimento)*

---

## Qué existe hoy en el repositorio

Unas **2,039 líneas de Go**, escritas para responder esa pregunta y nada más:

| Qué | Estado |
|---|---|
| `internal/engine` — el motor mínimo: formato de casa, decodificador por clip, servidor de cuadros, encoder persistente | **Construido** para lo que F0 pregunta |
| `internal/f0` — fabricación de los archivos de prueba, analizador, medición de CPU y RAM | **Construido** |
| `internal/ts` — lectura de transport stream paquete a paquete: continuidad, PCR, tasa, marcas de tiempo | **Construido** |
| `cmd/f0` — el ejecutable del experimento | **Construido** |
| `PRD.md`, `CONTEXT.md`, los nueve ADR, la auditoría, los criterios de aceptación, las 13 pantallas dibujadas | **Escritos** |
| `cmd/antena`, el resolver, el ingest, los drivers, la base de datos, la interfaz web | **No existen** |

**Una corrida corta de seis minutos pasa todo lo medible.** La corrida larga de
8 horas en la máquina de destino es lo que decide la fase — y hay que
presupuestar **una segunda corrida**: la primera casi nunca es la buena.

*→ [`docs/ARQUITECTURA.md`](https://github.com/Saul-Punybz/antena787/blob/main/docs/ARQUITECTURA.md),
sección **k**, para la comparación línea por línea entre la estructura prevista
y la real.*

---

## Lo que viene después

| Fase | Qué entrega |
|---|---|
| **F1 · Fundación** | Esquema, contenido con medición real y fichas sin clave, **editor de reglas y línea de tiempo**, resolver, guía validada, avisos de vencimiento. **El modo sombra: Antena787 propone y una persona compara. No toca el aire** |
| **F2 · Playout** | La fase grande. Motor completo, conformado, **los cuatro decks**, la salida `udp-ts`, fuentes en vivo, manual con regreso automático, grabación y diferido, cascada de respaldo, alarmas. **Prueba de resistencia de 30 días** |
| **F2.5 · Endurecimiento** | Que un Windows 10 sin presupuesto aguante 24/7. Corre en paralelo al soak |
| **F3 · Público** | Repositorio abierto, CI, instaladores, documentación bilingüe, matriz de hardware, lo legal. Modo internet completo |
| **F4 · Emisora** | Publicidad, crawl de clasificados, portal del anunciante con cobro, SCTE-104, reconciliación con alertas, evidencia de emisión. Perfil `us-fcc` |
| **F4b · Radio** · **F4c · Rotación musical** | Formato de casa solo audio · y después el subsistema que le falta a la radio musical |
| **F5 · Internacional** · **F5b · Escala** | Perfiles `eu-ebu` e `isdb-latam` · multi-canal, roles, API completa |
| **F6 · MCP** | **La última, y solo cuando todo lo anterior funcione sin ella** |

**F3 va antes que F4 a propósito:** un canal universitario no necesita
publicidad — necesita que el proyecto exista y funcione.

**Al terminar F5 el producto está completo.** F6 es aditiva: si nunca se
hiciera, Antena787 seguiría siendo un sistema entero. *Esa es la prueba de que
la IA está en el lugar correcto.*

*→ [`docs/ROADMAP.md`](https://github.com/Saul-Punybz/antena787/blob/main/docs/ROADMAP.md)
· [`PRD.md` §22](https://github.com/Saul-Punybz/antena787/blob/main/PRD.md#22--las-fases)*

---

## Cuánto falta, con los ojos abiertos

**El supuesto que manda: unas 10 horas a la semana**, que es más o menos un día
de trabajo efectivo. No es una queja: es el ritmo real de un proyecto que se
hace al lado de un trabajo.

| Hito | Optimista | Probable | Pesimista |
|---|---|---|---|
| **F0 decidida** | 2 semanas | 1 mes | 2 meses |
| **F1 · modo sombra** | 7 meses | **~1 año** | 1 año y medio |
| **F2 · madrugadas al aire** | 2 años | **~3.4 años** | 4 años y medio |
| **F2.5 + soak · aire completo** | 2 años y medio | **~3.9 años** | 5 años |
| **F4 · primer anunciante cobrado** | 4 años | **~5.6 años** | 7 años |

**Esto no es pesimismo, es aritmética.** ffplayout —un proyecto más chico, sin
publicidad, sin portal y sin cumplimiento— tomó más de dos años en llegar a algo
estable. **Decirlo aquí evita la conversación incómoda del mes catorce.**

> **La única palanca real es más horas en F1 y F2.** Recortar F4 no acelera
> nada: la publicidad viene después del aire, y quitarla no adelanta el aire ni
> un día. Duplicar el ritmo a 20 horas semanales sí parte los números por la
> mitad.

*→ [`PRD.md` §22.2](https://github.com/Saul-Punybz/antena787/blob/main/PRD.md#222--cuánto-tarda-esto-de-verdad)*

---

## El despliegue de referencia

**Caribbean Advantage TV, Puerto Rico.** Un canal comunitario que hoy opera al
aire con un Google Sheet hecho a mano, un humano, VLC, un servidor de streaming,
otro VLC, el equipo de alertas y el transmisor — **sobre un Windows 10 que ya
tenía, sin comprar hardware.** Es Class A, emite 720p59.94, y su cadena está
confirmada al detalle.

**Es donde se prueba primero, no el molde:** cada equipo que aparece ahí es un
ejemplo de una familia que tiene driver.

**Y aceptar ser el despliegue de referencia no puede significar apagar VLC un
lunes.** Por eso la entrada al aire tiene cuatro escalones: **modo sombra**
(Antena787 arma el plan y se compara contra lo que salió, **cero riesgo**) →
**salida paralela a archivo** → **madrugadas primero** —que hoy son aire vacío,
donde una falla casi nadie la ve y donde el sistema además aporta valor
inmediato— → **aire completo**.

*→ [`PRD.md` §17](https://github.com/Saul-Punybz/antena787/blob/main/PRD.md#17--cómo-entra-al-aire-de-rolando)*

---

## Lo que todavía no se sabe

**Nada de esto bloquea el diseño ni una fase** — el sistema asume todas las
variantes. Solo decide qué se prueba primero:

1. **Si el Raspberry Pi 5 tiene encoder H.264 por hardware.** Decide si vuelve
   como recomendación de compra.
2. **El modelo exacto del ENDEC** del despliegue de referencia. No hace falta
   antes de instalar: el asistente prueba serial, relés y red, y usa lo que
   responda.
3. **Por dónde va la telemetría del transmisor** — USB, SNMP, web, contactos.
   Ídem: se prueban todos los que tengan cable.
4. **Si esa estación está obligada a rotular subtítulos.** El ajuste arranca en
   *"no sé"*, los subtítulos se conservan y se suben igual, y lo único que
   cambia es si el sistema avisa.

*→ [`PRD.md` §25](https://github.com/Saul-Punybz/antena787/blob/main/PRD.md#25--lo-que-todavía-no-sabemos)*

---

## Cómo ayudar hoy

**Lo más útil ahora mismo no es código.** Correr la Fase 0 en tu máquina —sobre
todo en Windows y en hardware modesto— y reportar lo que midió. Contar qué
equipo tienes y cómo se llega a él. Traer las reglas de tu país con sus fuentes
primarias. Y decir dónde esta documentación miente o no se entiende.

*→ [Home](Home) ·
[`CONTRIBUTING.md`](https://github.com/Saul-Punybz/antena787/blob/main/CONTRIBUTING.md)*
