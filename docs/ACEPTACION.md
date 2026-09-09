# Criterios de aceptación — Antena787

> Fuente: `PRD.md` (§9-§10, §12, §14.1, §15, §19, §22, §23) y `CONTEXT.md`.
> Este documento no añade requisitos nuevos: traduce lo que el PRD ya dice a
> algo que se puede ejecutar o comprobar en un minuto.
>
> **Revisado el 8 de septiembre de 2026** contra el PRD posterior a la
> auditoría de nueve agentes (`docs/AUDITORIA_2026-09-04.md`). Los criterios
> que contradecían una decisión nueva se corrigieron en su sitio —no se
> borraron— y las decisiones nuevas se añadieron al final de su fase. Los
> trece requisitos que la versión anterior no pudo convertir por falta de un
> número ya tienen ese número en el PRD: la sección final dice en qué
> criterio quedó cada uno.

**Cómo leer los IDs:** `F0-##`, `F1-##`, `F2-##` — fase y número corrido
dentro de la fase. **[AUTO]** = se puede convertir en una prueba
automatizada (unitaria, de integración, o de instrumentación sobre una
corrida de motor). **[MANUAL]** = necesita a una persona — porque involucra
percepción humana (ver, oír), hardware real que el proyecto no controla en
una prueba automatizada, o una pantalla que hay que mirar — pero sigue
siendo una verificación de un minuto, no una revisión abierta.

Formato: **Dado** (estado de partida) · **Cuando** (la acción o el
instante) · **Entonces** (el resultado exacto y verificable).

---

## F0 · Prueba de concepto (§22.1)

F0 es un experimento con puerta de pase/fallo, no una fase de producto. Todos
los criterios comparten el mismo montaje salvo que se indique otra cosa:

> **Montaje común:** los 9 archivos de prueba del §22.1 (1080p29.97 H.264,
> 720p59.94, 480i29.97 MPEG-2, 1080p25 HEVC, VFR, mono, 5.1, con CEA-608, y
> uno corrupto en el último segundo), normalizados a 720p59.94, encadenados
> **en bucle durante 8 horas**, con **dos salidas simultáneas de volumen
> distinto**, corriendo en **el Windows 10 de Rolando o una máquina
> equivalente** — nunca en la laptop del desarrollador.

- **F0-01** [AUTO] — Dado el montaje común corriendo · Cuando ocurre cada
  cambio de clip a lo largo de las 8 horas · Entonces ninguna discontinuidad
  de muestras de audio en el punto de corte supera −40 dBFS.
- **F0-02** [AUTO] — Dado el montaje común corriendo durante 8 horas
  continuas · Cuando se mide el desfase acumulado entre audio y video al
  final de la corrida · Entonces el desfase es **menor a 20 ms**.
- **F0-03** [AUTO] — Dado el montaje común corriendo · Cuando ocurre cada uno
  de los cambios de clip · Entonces no hay cuadros duplicados ni perdidos en
  ese cambio (0 en cada uno, no en promedio).
- **F0-04** [AUTO] — Dado el flujo de salida completo de las 8 horas · Cuando
  se inspeccionan las marcas de tiempo de salida · Entonces son
  **monotónicas y sin saltos** en toda la corrida.
- **F0-05** [AUTO] — Dado el archivo de prueba #7 (con CEA-608 embebido)
  reproduciéndose dentro del bucle · Cuando llega a la salida · Entonces los
  subtítulos están presentes en el flujo de salida (verificable con
  `ccextractor` o equivalente sobre la grabación de salida).
- **F0-06** [AUTO] — Dado el archivo de prueba #9 (corrupto en el último
  segundo) reproduciéndose en el bucle · Cuando el motor llega a la parte
  corrupta · Entonces cae a relleno **sin negro** en la salida, y el evento
  queda registrado (no se pierde en silencio).
- **F0-07** [AUTO] — Dado el montaje común con una salida activa, y luego con
  dos salidas activas simultáneas · Cuando corre durante las 8 horas ·
  Entonces el uso de CPU y RAM de cada configuración queda medido con
  herramientas, y esas cifras reemplazan las estimaciones de §18 (nunca se
  publican como definitivas las que llevan la etiqueta *(estimado)*).
- **F0-08** [MANUAL] — Dado el hardware real de prueba (equivalente a
  Rolando) · Cuando se agregan sesiones de codificación por hardware una a
  una (QuickSync/NVENC/VAAPI) hasta que la máquina degrada o falla · Entonces
  el número máximo de sesiones simultáneas que aguanta esa máquina queda
  anotado como medida, no como supuesto.
- **F0-09** [AUTO] — Dado que cualquiera de los criterios F0-01 a F0-04
  falla (discontinuidad de audio, desfase ≥20 ms, o cuadros
  duplicados/perdidos) · Cuando se evalúa el resultado del experimento ·
  Entonces el diseño "servidor de cuadros en Go entre decodificadores por
  clip y encoder persistente" se replantea, y **F1 no arranca** hasta
  resolverlo.

---

## F1 · Fundación (ingest, reglas, resolver, guía — no toca el aire)

### Ingest (§9 paso 1)

- **F1-01** [AUTO] — Dado un archivo cuyo tamaño en disco sigue cambiando
  cada vez que se lee (se está copiando todavía) · Cuando la carpeta
  vigilada lo detecta · Entonces el ingest no lo abre ni lo procesa hasta que
  el tamaño se mantiene **estable durante 10 segundos** (default
  configurable) **y el archivo se abre sin bloqueo** — y nunca lo manda a
  cuarentena por haberlo leído a medias.
- **F1-02** [AUTO] — Dado un archivo de video válido colocado en la carpeta
  vigilada · Cuando el ingest lo mide con `ffprobe` · Entonces guarda códec,
  resolución, cuadros por segundo y **duración real en milisegundos**
  (no redondeada al segundo).
- **F1-03** [AUTO] — Dado un archivo cuyo volumen medido es −18 LUFS y el
  perfil del canal exige −24 LKFS · Cuando el ingest corre `loudnorm` en sus
  **dos pasadas** (medir, luego corregir) · Entonces el archivo normalizado
  queda dentro de la tolerancia del perfil, y existe un registro de que se
  hicieron las dos pasadas (no una).
- **F1-04** [AUTO] — Dado un archivo con subtítulos CEA-608 embebidos en el
  video de origen · Cuando el ingest lo normaliza al formato de casa (que
  decodifica y recodifica) · Entonces los subtítulos se extraen antes de la
  recodificación y se reinsertan explícitamente en el mux del archivo
  normalizado — el archivo de salida tiene subtítulos, no solo el de
  entrada.
  *Diferido a F2 el 9 de septiembre de 2026: no existe paso de extracción ni reinserción, y ffprobe ≥ 9 dejó de emitir `closed_captions`, así que la detección del 608 embebido tampoco es fiable. La prueba `TestF1_04_CEA608SobreviveAlFormatoDeCasa` queda escrita y saltada hasta que F2 la cierre. Ver la tabla del 9 de septiembre en el Resumen.*
- **F1-05** [AUTO] — Dado un archivo con 0.8 segundos de negro (luma media
  < 16) y silencio (audio < −60 dBFS) al inicio, y 0.3 segundos al final ·
  Cuando el ingest lo analiza · Entonces recorta el tramo inicial (≥ 0.5 s,
  sobre el umbral) y **no** recorta el final (< 0.5 s, bajo el umbral).
- **F1-06** [AUTO] — Dado un archivo de archivo (stock) con 2 segundos de
  negro y silencio **en la mitad**, no en cabeza ni cola · Cuando el ingest
  lo analiza · Entonces no lo recorta automáticamente: registra una **marca
  de corte candidata** en `marcas_de_corte_ms`, pendiente de que el
  programador la confirme.
- **F1-07** [AUTO] — Dado un archivo válido de cualquier códec soportado ·
  Cuando termina el ingest · Entonces existe una copia del archivo en el
  formato de casa configurado del canal (un códec, una resolución, GOP
  cerrado, un audio) generada **una sola vez**, no en cada reproducción.
- **F1-08** [AUTO] — Dado un archivo sin ficha ni carátula asociada, con
  etiquetas embebidas que incluyen título y año · Cuando el ingest busca
  metadata · Entonces usa las etiquetas embebidas y **no** hace ninguna
  llamada de red (verificable interceptando/mockeando las llamadas salientes).
- **F1-09** [AUTO] — Dado un archivo sin etiquetas, sin `.nfo` y sin
  carátula embebida, identificado como serie de TV · Cuando el ingest busca
  ficha · Entonces consulta primero TVmaze (sin clave) antes que cualquier
  driver que pida clave (TMDB, etc.), en ese orden.
- **F1-10** [AUTO] — Dado un archivo sin audio detectable, o con
  `ffprobe` reportando error de lectura · Cuando termina el intento de
  ingest · Entonces el archivo queda en estado `cuarentena` con
  `motivo_en_cristiano` explicando el problema, y **nunca** aparece
  disponible para programarse en una regla.

### Reglas (§9 paso 2, §15)

- **F1-11** [AUTO] — Dado un intento de guardar una `schedule_rule` con
  `fecha_fin` anterior a `fecha_inicio` · Cuando se ejecuta el guardado ·
  Entonces la base de datos rechaza la escritura por una restricción del
  **esquema** (no por una validación en el código de la aplicación) — recrea
  el caso *Hellsing* del §3 y lo hace imposible de raíz.
- **F1-12** [AUTO] — Dado una regla con `fecha_inicio` = 9 de agosto y
  `fecha_fin` = 20 de diciembre · Cuando el sistema intenta crear un
  `plan_item` para esa regla con fecha 21 de diciembre · Entonces la
  operación falla: **ningún** `plan_item` puede existir fuera del rango de
  fechas de su regla.
- **F1-13** [AUTO] — Dado una regla de "Kojak" con `episodios_por_corrida` =
  1 y `ultimo_episodio_emitido` = temporada 2, episodio 5 · Cuando la regla
  vuelve a correr al día siguiente · Entonces programa el episodio 6 de la
  temporada 2 (avanza y recuerda dónde quedó), no repite el episodio 5.
- **F1-14** [AUTO] — Dado una regla de "Gaming Longplays" con
  `episodios_por_corrida` = 10 **y espacio suficiente antes del siguiente
  inicio duro** · Cuando corre una vez · Entonces genera 10 `plan_item`
  consecutivos, uno por episodio, sin huecos entre ellos. (Si no cabieran los
  diez, manda el reloj: ver F1-38.)
- **F1-15** [AUTO] — Dado la Regla A vigente hasta el 20 de diciembre en el
  horario de las 8:00 AM, y la Regla B con `releva_a` apuntando a la Regla A
  desde el 21 de diciembre en el mismo horario · Cuando el resolver corre el
  21 de diciembre · Entonces genera el `plan_item` de la Regla B sin marcar
  **ningún** conflicto de solape con la Regla A.
- **F1-16** [AUTO] — Dado una regla de repetición con `repite_a` apuntando a
  su regla primaria, y la primaria ya emitió el episodio 12 en ese día de
  emisión · Cuando el resolver materializa la repetición · Entonces programa
  **el mismo episodio 12** —no el siguiente— y **no avanza ningún contador
  propio**. Ambos `plan_item` existen, cada uno bajo su regla, sin tratarse
  como duplicado ni como conflicto. *(El campo `es_segundo_pase` ya no
  existe: la relación es `repite_a`, y el contador es compartido.)*

### Resolver (§9 paso 3)

- **F1-17** [AUTO] — Dado el resolver corriendo a las 10:00 AM del lunes ·
  Cuando termina de correr · Entonces existen `plan_item` para el canal
  cubriendo, como mínimo, hasta las 10:00 AM del miércoles (48 horas por
  adelantado), y ninguno más allá de esa ventana.
- **F1-18** [AUTO] — Dado una regla con `fecha_fin` = hoy · Cuando el
  resolver corre a las 00:05 de mañana · Entonces **no** genera `plan_item`
  para esa regla, y el sistema no la trata como error silencioso — queda
  simplemente fuera del plan.
- **F1-19** [AUTO] — Dado dos reglas distintas (sin relación de relevo)
  programadas para el mismo canal en la misma franja horaria del mismo día ·
  Cuando el resolver corre · Entonces reporta un conflicto de solape y no
  genera `plan_item` para ambas a la vez.
- **F1-20** [AUTO] — Dado una regla que apunta a un `media_asset` en estado
  `cuarentena` · Cuando el resolver intenta materializarla · Entonces
  reporta el conflicto (archivo no disponible) en vez de programar un
  archivo que nunca debería llegar al aire.
- **F1-21** [AUTO] — Dado un hueco de 7:00 min y relleno disponible de 2:00,
  3:00 y 4:00 · Cuando el resolver corre · Entonces el plan tiene 3:00 + 4:00
  min, en ese orden, y el hueco residual es 0.
- **F1-22** [AUTO] — Dado un hueco de 5:57 min y relleno disponible solo en
  bloques de 2:00 y 3:00 (ninguna combinación exacta suma 5:57) · Cuando el
  resolver corre · Entonces arma 3:00 + 3:00 = 6:00, que **excede el hueco en
  3 segundos — dentro del límite de 5 s del §14.1**, y recorta **el último
  clip de relleno del hueco** con un **fundido de 1 segundo**. El hueco
  residual queda en 0, y no hay subtítulos que romper porque el relleno no
  lleva. Si el exceso mínimo posible superara los 5 s, la combinación se
  descarta y se busca otra.
  *Alcance de F1 (9 de septiembre de 2026): el resolver deja `fundido_salida_ms = 1000` en el clip recortado; aplicar el fundido de verdad es del motor (F2).*
- **F1-23** [AUTO] — Dado un plan resuelto sin vencimientos próximos ·
  Cuando una regla llega a 30, luego a 14, luego a 7 días de su
  `fecha_fin` · Entonces se genera un aviso de vencimiento en cada uno de
  esos tres umbrales (no solo al final), sin duplicarse si el resolver corre
  varias veces dentro del mismo día.
- **F1-24** [AUTO] — Dado 20 reglas configuradas cubriendo cada franja de la
  semana de un canal, con relleno suficiente cargado · Cuando el resolver
  corre · Entonces produce las 336 medias-horas de la semana (7 días × 24 h
  × 2) sin ninguna franja sin `plan_item` ni relleno asignado.
- **F1-25** [AUTO] — Dado un canal con `hora_inicio_dia_emision` = 6:00 AM ·
  Cuando un `plan_item` sale a las 12:30 AM del martes · Entonces pertenece
  al **día de emisión del lunes**, no al martes (se refleja así en cualquier
  reporte o conteo agrupado por día de emisión).
- **F1-26** [MANUAL] — Dado el plan de mañana ya resuelto por el resolver ·
  Cuando un programador abre la parrilla y cambia un `plan_item` en estado
  `planned` para mañana (no el que está al aire ahora) · Entonces el cambio
  se guarda y el resolver no lo sobreescribe en su siguiente corrida
  automática.

### Guía XMLTV (§9 paso 2-3, §13, §3)

- **F1-27** [AUTO] — Dado un canal configurado como "Caribbean Advantage
  TV" con su parrilla real cargada · Cuando se genera el XMLTV · Entonces
  el `channel display-name` y los `programme` apuntan al canal y al
  contenido reales configurados — **nunca** a datos de una plantilla de
  ejemplo (el bug del §3 medido en CAtv: XMLTV con `sports1.channel` y
  fechas de mayo mientras el canal decía otra cosa).
- **F1-28** [AUTO] — Dado un `plan_item` cuya `fecha_fin` calculada resulte
  anterior a su `instante_planeado` (por un error hipotético aguas arriba) ·
  Cuando el validador propio de la guía corre antes de publicar el XMLTV ·
  Entonces **rechaza** la publicación y reporta el error — no lo marca "✓
  OK" como hacía `tv_validate_file` con *Hellsing* en el §3.
- **F1-29** [AUTO] — Dado la Regla A vigente hasta ayer a las 8:00 AM y la
  Regla B (`releva_a` = Regla A) vigente desde hoy en la misma franja ·
  Cuando se genera el XMLTV de hoy · Entonces el bloque de las 8:00 AM
  muestra el título de la Regla B, no el de la Regla A.

### As-run en modo sombra (§9 paso 2, §17, §22)

- **F1-30** [AUTO] — Dado un canal en modo sombra · Cuando el resolver
  genera un `plan_item` · Entonces sus campos `instante_real` y
  `duracion_real` quedan vacíos (no hay motor de aire escribiendo en ellos),
  mientras `instante_planeado` y `duracion_planeada` sí están poblados.
- **F1-31** [AUTO] — Dado un canal en modo sombra · Cuando el sistema opera
  con normalidad durante un día completo · Entonces ningún proceso de
  Antena787 escribe a una salida de video/audio real — F1 arma plan y guía,
  pero **no toca el aire**.
- **F1-32** [MANUAL] — Dado el plan que Antena787 habría puesto para un
  bloque de una hora, y el registro manual de lo que Rolando emitió con VLC
  en esa misma hora · Cuando se comparan ambos a ojo · Entonces se anota si
  coinciden o no coinciden, como evidencia acumulada antes de tocar el aire
  real.

### Día de emisión y fechas (§9 pasos 2-3, §15)

- **F1-33** [AUTO] — Dado un canal con `hora_inicio_dia_emision` = 6:00 AM y
  una regla con `fecha_fin` = lunes 20 de diciembre · Cuando el resolver
  evalúa la franja de las 2:00 AM del **martes 21 de calendario** · Entonces
  la regla **sigue vigente**: `fecha_fin` es inclusiva hasta el cierre del día
  de emisión, o sea hasta las **5:59:59 AM del día calendario siguiente**.
- **F1-34** [AUTO] — Dado una regla con patrón de días `L` (solo lunes) a las
  2:00 AM · Cuando el resolver la materializa · Entonces genera el
  `plan_item` a las 2:00 AM del **martes de calendario**, porque esa hora
  pertenece al **día de emisión del lunes**: el patrón LMMJVSD se lee por día
  de emisión, nunca por fecha de calendario.
- **F1-35** [AUTO] — Dado el modelo completo de `schedule_rule` · Cuando se
  busca un campo de recurrencia anual o un manejo especial del 29 de febrero ·
  Entonces **no existe ninguno**: el patrón es semanal y las fechas son
  absolutas, así que el bisiesto no es un caso borde que haya que programar.
- **F1-40** [AUTO] — Dado un canal con `hora_inicio_dia_emision` = 6:00 AM y
  un `plan_item` de tres horas que arranca a las 5:00 AM del martes de
  calendario · Cuando se le atribuye un día de emisión · Entonces pertenece
  **completo al día de emisión del lunes** —el de su instante de inicio—
  aunque termine a las 8:00 AM, y así lo cuentan los reportes y el diferido.

### Repeticiones y contador de episodios (§9 paso 2, §15)

- **F1-36** [AUTO] — Dado una regla de repetición con `repite_a` cuya regla
  primaria **no emitió nada** ese día de emisión (vencida, en conflicto, o
  fuera de su patrón) · Cuando el resolver materializa la repetición ·
  Entonces toma el **siguiente episodio de la primaria** y avanza el contador
  **compartido** de la serie: la serie nunca se estanca.
- **F1-37** [AUTO] — Dado una serie con su regla primaria a las 2:00 PM y su
  repetición a las 11:00 PM, corriendo veinte días de emisión seguidos ·
  Cuando se comparan los episodios que emitió cada una · Entonces existe **un
  solo `ultimo_episodio_emitido`** para esa serie en esa franja, y las dos
  reglas nunca se desincronizan: la de las 11 PM jamás va por delante ni por
  detrás de la de las 2 PM.

### Sobrecupo: el reloj manda sobre el contador (§9 paso 3)

- **F1-38** [AUTO] — Dado una regla con `episodios_por_corrida` = 10 en una
  franja que termina en un inicio duro, y episodios cuya suma no cabe ·
  Cuando el resolver materializa la franja · Entonces programa **los 9 que
  caben**, **no arranca el décimo**, rellena el resto del espacio, avisa *"de
  10 episodios caben 9"*, y `ultimo_episodio_emitido` avanza **solo por lo que
  salió**. **Ningún episodio queda cortado a la mitad por no haber cabido.**
- **F1-39** [AUTO] — Dado un bloque en vivo de 10:00 a 13:00 y un elemento
  programado `dentro_de` que no terminaría antes de las 13:00 · Cuando el
  resolver lo materializa · Entonces **no lo arranca**: el fin del bloque en
  vivo es tan duro como el de un slot de archivo.
  *Alcance de F1 (9 de septiembre de 2026): el resolver de F1 no materializa ítems `dentro_de` (llegan con F2); lo que F1 verifica es que una regla que arranca dentro de un bloque en vivo no lo recorta ni se desborda de su fin, y que se avisa en cristiano.*

### Material listo para aire (§9 pasos 1 y 3)

- **F1-41** [AUTO] — Dado un `media_asset` con `estado_normalizacion`
  distinto de `listo` · Cuando el resolver materializa una regla que lo usa ·
  Entonces **no lo programa**: solo entra al plan material `listo` para aire,
  y la regla queda reportada como conflicto igual que un archivo en
  cuarentena.
- **F1-42** [AUTO] — Dado tres archivos en la cola de normalización cuyas
  primeras salidas al aire son a las 8:00 AM de hoy, a las 3:00 PM de hoy y
  mañana · Cuando la cola decide el orden de trabajo · Entonces los normaliza
  **en ese orden — por cuándo salen al aire**, no por orden de llegada.
- **F1-43** [MANUAL] — Dado un archivo todavía en normalización · Cuando se
  abre Biblioteca (y Anuncios, si es un spot) · Entonces aparece marcado
  **"aún no listo para aire"**, no como disponible; y una compra del portal no
  se da por lista hasta que termina.
  *Alcance de F1 (9 de septiembre de 2026): la mitad de Anuncios y la compra del portal es F4; en F1 se verifica Biblioteca.*

### No-solape en el esquema (§15)

- **F1-44** [AUTO] — Dado dos `plan_item` del **mismo `deck` y la misma
  salida** con intervalos que se solapan · Cuando se intenta insertar el
  segundo · Entonces **la base de datos lo rechaza** —índice más verificación
  al insertar—, no una validación del código de la aplicación. (El motor tiene
  además su cinturón en runtime: F2-73.)

### Avisos de vencimiento (§9 paso 3)

- **F1-45** [AUTO] — Dado una regla que vence en 7 días y otra regla con
  `releva_a` apuntando a ella en la misma franja · Cuando corre el cálculo de
  avisos · Entonces **no se genera ningún aviso** para la regla que vence: hay
  quien la sustituya, y avisar de ella entrena al operador a ignorar los
  avisos.
- **F1-46** [MANUAL] — Dado una regla a 7 días de su `fecha_fin` y **sin**
  relevo cargado · Cuando corre el cálculo de avisos · Entonces el aviso
  aparece en **Al aire, en Reglas y en Parrilla · Mes**, y además sale por el
  **canal de avisos** configurado (Telegram, WhatsApp o correo). A 30 y a 14
  días aparece solo en las tres pantallas.

### La guía se regenera con el plan (§9 paso 3)

- **F1-47** [AUTO] — Dado el resolver corriendo su ciclo horario · Cuando
  termina de materializar el plan · Entonces **reescribe la guía XMLTV en esa
  misma corrida**, sin esperar a un proceso aparte ni a un temporizador
  propio.
- **F1-48** [AUTO] — Dado un plan ya publicado en la guía · Cuando el plan
  cambia por cualquier motivo —edición manual, relevo, conflicto resuelto,
  sobrecupo, cambio de última hora— · Entonces la guía se regenera **en ese
  instante**, y en ninguna medición el desfase entre la guía servida y el plan
  vigente supera **un minuto**.
- **F1-49** [AUTO] — Dado un canal con guía configurada · Cuando se pide
  **`/guia.xml`** · Entonces responde siempre con el XMLTV vigente; el mismo
  contenido se escribe además en la ruta de archivo configurada —la que lee el
  servidor de streaming o el transmisor—, y el envío opcional por HTTP a un
  destino externo puede fallar **sin afectar** ni la guía local ni el aire.

### El importador de la hoja de cálculo (§13)

- **F1-50** [AUTO] — Dado una hoja importada con 200 filas de las cuales 7 no
  se pueden interpretar · Cuando corre el importador · Entonces importa las
  **193 válidas** y devuelve una lista **fila por fila** de las 7, con el
  motivo en cristiano. **Nunca rechaza la hoja entera.**
- **F1-51** [AUTO] — Dado una fila de la hoja con hora 2:00 AM y fecha de
  calendario del martes · Cuando corre el importador · Entonces guarda la
  regla con la fecha **corrida un día atrás** (lunes, que es su día de
  emisión) y lo **reporta en la lista fila por fila**, en vez de hacerlo
  callado.
- **F1-52** [AUTO] — Dado una fila cuyo título coincide con una `live_source`
  existente (`RadioOnce Live!`) y que trae `Duración = 6` · Cuando corre el
  importador · Entonces la importa como **bloque en vivo** e **ignora la
  columna de episodios**, avisándolo en la lista fila por fila.
- **F1-53** [MANUAL] — Dado una regla A que termina el día 7 y una regla B que
  empieza el día 8 en la misma franja y con el mismo patrón · Cuando corre el
  importador · Entonces pregunta *"¿Zoids releva a Magic Knight?"* y, si se
  confirma, carga `releva_a` — para que un relevo no se cuente después como un
  vencimiento sin reemplazo.
- **F1-54** [MANUAL] — Dado una hoja recién importada, sin reglas de relleno ni
  de diferido · Cuando se abre Parrilla · Entonces los tramos vacíos se ven
  marcados y ofrecen el botón **"Llenar con diferido"**, que de un clic crea
  la regla que retransmite la mañana en la madrugada.

### Bloque arrendado y revelación progresiva (§9 paso 2, §11, §13)

- **F1-55** [AUTO] — Dado una `schedule_rule` con `tipo = bloque_arrendado`,
  su `advertiser` y su cobro · Cuando el resolver corre · Entonces la
  materializa como cualquier otra regla y el bloque queda **en la parrilla y
  en el reporte de ingresos**, no en una libreta aparte.
  *Alcance de F1 (9 de septiembre de 2026): en F1 el bloque queda en la parrilla y en la guía con su anunciante y su cobro guardados; el reporte de ingresos es F4, donde la tabla de cobertura coloca publicidad y cobro.*
- **F1-56** [AUTO] — Dado el paquete completo de cadenas de la interfaz —todo
  lo que llega a ver el operador— · Cuando se buscan los cinco términos que el
  principio 3 prohíbe (*driver*, *códec*, *GOP*, *LKFS*, *transport stream*) ·
  Entonces **no aparece ninguno**: Ajustes dice *"volumen de televisión de
  EE. UU."* y no *"−24 LKFS"*, y Biblioteca dice *"importados de la carpeta"*
  y no el nombre de un driver. Es una lista cerrada, y por eso es verificable.
- **F1-57** [AUTO] — Dado un canal sin ningún `advertiser` registrado y sin
  segundo canal ni segunda persona · Cuando se carga la interfaz · Entonces el
  menú tiene **cinco** entradas; al registrar el primer anunciante pasa a
  **seis** (aparece Anuncios); y ni multi-canal ni roles ni nombres de usuario
  son visibles en ninguna pantalla.
  *Alcance de F1 (9 de septiembre de 2026): el servidor informa `hay_anunciantes` leyendo la tabla; el alta de anunciantes (la pantalla que hace pasar de cinco a seis) es F4.*

### Audio de todo el material (decisión del 9 de septiembre de 2026, §9 paso 1)

- **F1-58** [AUTO] — Dado un archivo de video sin pista de audio y, a su lado
  en la carpeta vigilada, un archivo de audio con el **mismo nombre** y otra
  extensión (`.wav`, `.m4a`, `.aac`, `.mp3`, `.flac`) · Cuando el ingest lo
  procesa · Entonces **no** va a cuarentena: el audio de al lado se muxea en el
  formato de casa, el `media_asset` guarda de dónde salió (`audio_sidecar`), y
  el archivo queda `listo` con su volumen medido en dos pasadas como cualquier
  otro. Si el audio de al lado llega **después** de que el video ya está en
  cuarentena por mudo, el ingest lo detecta y vuelve a procesar el video solo.
- **F1-59** [AUTO] — Dado un archivo de video sin audio y **sin** archivo de
  audio al lado · Cuando termina el ingest · Entonces queda en `cuarentena`
  con un motivo que dice qué hacer (*«no trae sonido: pon a su lado un archivo
  de audio con el mismo nombre»*), y **no existe** camino para soltarlo mudo:
  todo lo que sale al aire lleva audio.
- **F1-60** [AUTO] — Dado un archivo con **varias pistas de audio**
  etiquetadas por idioma y un canal con `idioma_audio_preferido` = `es` ·
  Cuando el ingest lo procesa · Entonces guarda la lista de pistas
  (`pistas_audio`: índice, idioma, canales, título) y elige para el aire la
  primera pista en `es`; si ninguna está en ese idioma, la primera del
  archivo. El formato de casa se genera con **esa** pista.
- **F1-61** [AUTO] — Dado un archivo con varias pistas ya normalizado · Cuando
  el programador cambia `pista_audio_aire` desde Biblioteca · Entonces el
  cambio se guarda, el archivo vuelve a la cola de normalización y el nuevo
  formato de casa sale con la pista elegida; mientras tanto aparece *«aún no
  listo para aire»*.
- **F1-62** [AUTO] — Dado un archivo de video con un archivo de subtítulos al
  lado con el mismo nombre (`.srt`, `.vtt`, `.scc`) · Cuando el ingest lo
  procesa · Entonces guarda la ruta en `subtitulos_sidecar`; los `.srt`/`.vtt`
  se muxean en el formato de casa como pista de texto; los `.scc` se guardan
  sin tocar para que F2 los reinserte como CEA-608.
- **F1-63** [AUTO] — Dado el esquema de `media_asset` · Cuando se busca dónde
  irá la **segunda pista al aire** (SAP: español/inglés) · Entonces existe
  `pista_audio_sap` (nulo hasta F2) y ningún otro campo hay que inventar
  después: el motor de F2 la lleva al mux sin cambiar el esquema.

### Emparejar títulos (issue #13, 9 de septiembre de 2026; §9 importador)

- **F1-64** [AUTO] — Dado una hoja con un título que no se parece a ninguno
  del catálogo (`Samurai X`, cuya ficha es `Rurouni Kenshin`) · Cuando corre
  el importador · Entonces **no** crea una ficha nueva callado: la regla se
  importa igual (la hoja nunca se rechaza), el título queda marcado
  **«por emparejar»**, aparece en la lista `titulos_sin_emparejar` de la
  respuesta y en la pantalla de Reglas, y sale un aviso en Al aire mientras
  quede alguno.
- **F1-65** [AUTO] — Dado un título que se parece **igual** a varios del
  catálogo (`SaberMarionette` ↔ `Saber Marionette J` / `R`) · Cuando corre el
  importador · Entonces **no adivina**: lo deja por emparejar con esos
  candidatos listados, y el nombre del catálogo manda cuando la persona elige.
- **F1-66** [AUTO] — Dado un título por emparejar · Cuando la persona lo
  empareja con una ficha del catálogo · Entonces las reglas que lo usaban
  pasan a esa ficha, el título provisional desaparece, el plan se recalcula,
  y el nombre de la hoja queda guardado como **alias** de la ficha: la
  siguiente hoja que traiga ese nombre se empareja sola, sin preguntar.
- **F1-67** [AUTO] — Dado un título por emparejar · Cuando la persona dice
  «es un título nuevo» o «no es un programa» · Entonces en el primer caso la
  ficha se queda como propia y deja de estar por emparejar; en el segundo el
  título y las reglas que lo usaban se quitan, diciendo cuántas. Y el nombre
  de una fuente en vivo (`RadioOnce Live!`) **nunca** entra al catálogo como
  título.

---

### Cuarentena e incidentes en pantalla (issues #6 y #7, 9 de septiembre de 2026; §13, §15)

- **F1-68** [AUTO] — Dado un archivo en `cuarentena` que un título o un
  episodio ya fichó, y otro que nadie fichó · Cuando Biblioteca pide la
  cuarentena · Entonces cada fila trae `titulo` con nombre de persona
  (*«Space Cobra · T1E4 La joya»*; el nombre del archivo sin extensión si no
  hay ficha), su `motivo_en_cristiano` y el botón de dejarlo pasar bajo un
  nombre que queda en `audit_log`. Y mientras quede alguno, **Al aire** enseña
  el aviso *«N archivos en cuarentena»* con camino a Biblioteca; se recalcula
  al arrancar, tras cada ingest y al dejar pasar uno, y se apaga solo cuando
  no queda ninguno. Es un aviso, no un problema: el sistema no regaña.
- **F1-69** [AUTO] — Dado incidentes de varios tipos en la tabla `incidente`
  (uno conocido, un `panico_<goroutine>`, uno que la lista aún no conoce) ·
  Cuando se pide `GET /incidentes` · Entonces cada fila trae lo que la
  pantalla pinta (`id`, `tipo`, `inicio`, `fin`, `detalle`) **más `texto`**,
  la frase en cristiano de su tipo, en un solo sitio del servidor; el pánico
  dice qué parte se relanzó y el desconocido sale legible. Un rango sin
  incidentes es una lista vacía y una fecha mal escrita es `400`. Al aire
  la enseña como tarjeta con lo último y un panel al lado (nunca un modal,
  ADR 0008) con 7, 30 o 90 días, agrupada por día y con la duración de lo que
  ya cerró; se refresca sola con cada evento del WebSocket.

### Lo aprendido de los proyectos comparables (`docs/investigacion/CUARENTENA-Y-BITACORA-COMPARADAS-2026-09-09.md`; §9 paso 1)

- **F1-70** [AUTO] — Dado un archivo cuya imagen dura más de 4 s que su sonido
  (o al revés), y otro con un desfase de 1 s · Cuando el ingest los procesa ·
  Entonces el primero queda en `cuarentena` con `motivo_codigo` =
  `duracion_av_no_coincide` y un motivo que dice las dos duraciones (*«imagen
  0:10, sonido 0:02»*) y que se puede dejar pasar; el segundo entra normal. El
  umbral es `ingest.DesfaseAVMaximo` (4 s, como ffplayout); sin medida de
  alguna de las dos no se afirma nada. El código se guarda en su columna
  (`media_asset.motivo_codigo`, esquema **versión 5**, con relleno hacia atrás
  del único código que existía).
- **F1-71** [AUTO] — Dado una normalización que no termina · Cuando pasa su
  plazo (`max(15 min, 4 × duración del archivo)`, `App.normalizeDeadline`) ·
  Entonces se cancela y cuenta como intento fallido con un motivo que dice que
  se quedó colgada; y cuando la cola la da por perdida, el archivo **no** se
  queda «aún no listo para aire» para siempre: pasa a `cuarentena` con
  `motivo_codigo` = `normalizacion_fallida`, Al aire lo cuenta en el aviso,
  queda el incidente, y dejarlo pasar lo saca al aire tal cual.

### Lo aprendido del modo sombra con archivos reales (`docs/f1/SOMBRA-2026-09-09.md`; §9 paso 1, §10)

- **F1-72** [AUTO] — Dado archivos con el nombre con el que llegan de verdad
  (`Serie.S04E01.Título.1080p.x265-Grupo[web].mkv`, `Serie - 4x01 - Título`,
  `Serie T1E4`, `Película.2026.1080p.WEBRip.mp4`, `[Sub] Serie - 04 [1080p]`)
  y sin `.nfo` ni etiquetas útiles · Cuando el ingest los ficha · Entonces la
  serie, la temporada, el episodio y el nombre del episodio salen del nombre
  (`ingest.FichaDesdeNombre`), la película sale con su año, la cola técnica y
  la firma del grupo no aparecen en ningún título, y los episodios de la
  misma serie caen en **un solo** título. Una etiqueta de título que es el
  propio nombre del archivo con puntos («For.All.Mankind.S05E09») o la firma
  de quien lo subió («by ToonsHub») no es un título: se lee como nombre de
  archivo. Un etiquetado de verdad (serie/temporada/episodio en las etiquetas
  o en el `.nfo`) manda sobre el nombre. `fuente_ficha` dice todo lo que
  aportó algo.
- **F1-73** [AUTO] — Dado un archivo que acaba de terminar de copiarse ·
  Cuando el ingest empieza a medirlo · Entonces su fila existe desde el
  primer segundo en estado `ingiriendo`, `GET /material?estado=ingiriendo` lo
  devuelve y Biblioteca lo enseña en una franja «Entrando» que se refresca
  sola con los eventos del servidor; al terminar pasa a `listo` o a
  `cuarentena` en una sola escritura que ya lleva sus archivos de al lado, y
  un archivo que ni se puede medir nunca se queda en `ingiriendo`.

---

## F2 · Playout (motor, decks, fuentes en vivo, manual, diferido, grabación, salidas)

### Motor: cambio de clip, conformado, decks y prioridad (§9 paso 4, §14.1)

- **F2-01** [AUTO] — Dado el motor operando en automático durante 8 horas
  continuas de contenido variado · Cuando se mide la salida completa ·
  Entonces el proceso `ffmpeg` del encoder de salida **nunca se reinicia**
  por sí mismo durante la corrida (0 reinicios no solicitados).
- **F2-02** [AUTO] — Dado un clip cuyo audio mide 200 ms menos que su video
  · Cuando el motor lo conforma antes de entregarlo al encoder · Entonces
  rellena los 200 ms finales del audio con silencio digital, y la duración
  de video no se recorta para igualarlo.
- **F2-03** [AUTO] — Dado un clip cuyo video mide 300 ms menos que su audio
  · Cuando el motor lo conforma · Entonces sostiene el **último cuadro** de
  video durante 300 ms adicionales en vez de cortar el audio.
- **F2-04** [AUTO] — Dado un clip 4:3 en un canal de formato de casa 16:9 ·
  Cuando el motor lo conforma · Entonces aplica pillarbox (barras laterales)
  en vez de estirar la imagen.
- **F2-05** [AUTO] — Dado dos clips consecutivos en el plan · Cuando el
  motor se acerca al final del primero · Entonces el decodificador del
  segundo ya arrancó (pre-roll) antes de que el primero termine, y el
  cambio no produce cuadros duplicados/perdidos ni discontinuidad de audio
  sobre −40 dBFS (mismos umbrales validados en F0).
- **F2-06** [AUTO] — Dado que el deck programa tiene un `plan_item` en
  curso y el deck comercial tiene un corte pautado exactamente a las
  14:00:00 · Cuando el reloj llega a las 14:00:00 · Entonces el aire pasa al
  deck comercial en ese instante, sin esperar a que el `plan_item` de
  programa termine su propio corte natural.
- **F2-07** [AUTO] — Dado un espacio de 30 minutos compuesto por 24 minutos
  de programa (origen archivo) y 6 minutos de cortes en las marcas
  `marcas_de_corte_ms` del archivo · Cuando el motor llega a una de esas
  marcas · Entonces **pausa** el programa, reproduce el corte, y **reanuda
  el programa exactamente donde iba** al terminar el corte.
- **F2-08** [AUTO] — Dado un `plan_item` cuyo origen es un `live_source` en
  curso, y un corte pautado dentro de esa misma franja · Cuando llega la
  hora del corte · Entonces la señal en vivo **sigue corriendo por debajo**
  (no se pausa) y, al terminar el corte, el aire regresa a la señal en el
  **instante actual** de esa señal, no al punto en que se interrumpió.
- **F2-09** [AUTO] — Dado que termina un `plan_item` de programa y no hay
  relleno cargado para el canal · Cuando el motor necesita producir salida
  para el siguiente instante · Entonces cae a **cartel (Slate)**, nunca a
  negro.
- **F2-10** [AUTO] — Dado que no hay programa y no hay relleno · Cuando el
  motor necesita producir salida · Entonces cae al **cartel, que es el último
  escalón de la cascada** —programa → relleno → cartel— y **nunca a negro ni
  a barras**. El cartel siempre existe: lo genera el paso 1 del asistente con
  el identificativo y la comunidad de licencia, así que un canal recién
  instalado ya lo tiene y una caída larga sigue identificando la estación.
  *(Corregido: la versión anterior de este criterio ponía barras y tono como
  último escalón. Las barras y el tono existen solo para la prueba del paso 5
  del asistente — ver F2-69 y F2-104.)*
- **F2-11** [AUTO] — Dado el proceso encoder de salida congelado (no consume
  un solo cuadro) · Cuando pasan **3 segundos** sin que consuma cuadro
  (default configurable) · Entonces el motor manda el **cartel** al aire, mata
  el encoder, lo **relanza con el mismo acelerador** y registra un incidente
  `encoder_reiniciado`. Si vuelve a fallar **dos veces en 10 minutos**, lo
  relanza **por software** y avisa en cristiano: *"tu tarjeta de video dejó de
  responder"*.
- **F2-12** [AUTO] — Dado un `plan_item` de archivo que falla al reproducirse
  en el aire (error de decodificación, archivo borrado, archivo en cero, o el
  NAS que se reinició) · Cuando el motor detecta el fallo · Entonces cuenta
  como **fallo de clip** y dispara la cascada de inmediato; pero el
  `media_asset` **solo pasa a `cuarentena` tras dos fallos separados por más
  de 5 minutos**, con el motivo registrado y sin que un humano lo mande. Un
  solo parpadeo de red **no** saca de la parrilla un programa bueno; dos
  fallos de verdad sí, para que no se programe otra vez la semana siguiente y
  falle igual.
- **F2-13** [AUTO] — Dado un `plan_item` de 20:00 minutos que empezó hace
  07:32 cuando el servicio de Antena787 se reinicia · Cuando el motor
  arranca de nuevo · Entonces todo lo que estaba en `cued` vuelve a
  `planned`, calcula que debería estar en el minuto 07:32 de ese archivo,
  abre el mismo archivo con **seek** a ese segundo, y arranca — **nunca**
  reinicia el bloque desde 00:00.
- **F2-14** [AUTO] — Dado el reloj **monotónico** del motor, derivado de las
  marcas de tiempo del encoder · Cuando se compara contra la hora de pared
  cada minuto y la diferencia es **de hasta 60 segundos** · Entonces se
  corrige por **deriva gradual**, nunca por salto brusco, sin importar si son
  milisegundos o decenas de segundos. *(Una diferencia mayor a 60 segundos ya
  no es deriva sino un salto de reloj, y se trata aparte: ver F2-89.)*
- **F2-15** [MANUAL] — Dado el motor arrancando en una máquina con
  QuickSync, NVENC, VAAPI y software disponibles · Cuando corre la prueba de
  10 segundos con cada encoder candidato · Entonces la pantalla de Ajustes
  muestra cuál fue elegido y el uso de CPU medido de cada uno.
- **F2-16** [AUTO] — Dado un ID de estación programado `dentro_de` el
  bloque de RadioOnce Live! · Cuando llega su hora · Entonces toma el aire
  (deck programa) y, al terminar, la señal en vivo regresa sola, sin
  intervención manual.
- **F2-17** [AUTO] — Dado un archivo con audio 5.1 y otro con audio mono
  (archivos de prueba #6 de F0) programados consecutivamente en producción
  · Cuando el motor los reproduce · Entonces ambos salen sin fallo y sin
  discontinuidad de canal audible en el cambio.

### Fuente en vivo (§9 paso 5)

- **F2-18** [AUTO] — Dado un bloque en vivo reservado a las 10:00:00 con
  señal SRT recibida en el punto de escucha desde las 09:59:50 · Cuando el
  reloj llega a las 10:00:00 · Entonces el aire pasa a la señal en vivo
  exactamente en ese instante.
- **F2-19** [AUTO] — Dado un bloque en vivo reservado a las 10:00:00 sin
  señal SRT recibida a esa hora, y `live_source.gracia_s` = 30 (default) ·
  Cuando el motor llega a las 10:00:00 · Entonces **dentro del margen de
  gracia** sostiene el último cuadro del programa anterior o pone el cartel,
  **sin alarma y sin incidente** —llegar a las 10:00:45 es normal, no una
  falla—; y **solo pasado ese margen** entra el relleno y se registra el
  incidente `vivo_ausente` con su alarma. El bloque sigue reservado en los dos
  casos.
- **F2-20** [AUTO] — Dado el escenario anterior (relleno cubriendo la
  ausencia, con reintentos de espera progresiva 1, 2, 4… hasta 60 segundos de
  tope) y la señal SRT aparece a las 10:03:00, dentro de la
  `duración_prevista` del bloque · Cuando el motor detecta la señal
  disponible · Entonces vuelve al vivo **en el siguiente borde de clip de
  relleno —como máximo 60 segundos después; si el clip de relleno en curso
  dura más, se corta con fundido de 1 s— y con fundido cruzado**, sin
  esperar al siguiente ciclo del resolver y sin que nadie tenga que apretar
  nada. No corta el clip de relleno por el medio.
- **F2-21** [AUTO] — Dado un bloque en vivo al aire con un `reloj_de_cortes`
  fijo a los minutos 18 y 48 de cada hora · Cuando el reloj llega al minuto
  18 · Entonces el corte entra automáticamente en ese minuto exacto, sin
  intervención del operador.
- **F2-22** [AUTO] — Dado un `driver_de_cue` configurado (contacto seco
  simulado) sobre una `live_source` con su `retardo_ms` **por defecto de
  7 000 ms** (el *profanity delay* del §9 paso 5, configurable) · Cuando el
  driver recibe la señal de corte de la fuente · Entonces el motor dispone de
  esos 7 segundos de búfer para cortar limpio y no comerse la primera palabra
  al volver — y el búfer es de segundos, no de minutos: no rompe el reloj de
  pared.
- **F2-23** [AUTO] — Dado la configuración de salida hacia el transmisor ·
  Cuando se inspecciona la configuración de red del canal · Entonces no
  existe ningún driver que "jale" la señal desde una plataforma externa
  (YouTube/Facebook) hacia el transmisor: el flujo siempre entra por
  `srt-listen`/`rtmp-listen` desde el equipo de streaming, y sale desde ahí
  a todas las salidas simultáneamente.
- **F2-24** [AUTO] — Dado un `live_source` reservado en el plan para las
  10:00-13:00 sin que la señal haya llegado todavía · Cuando el resolver
  arma el plan de esa franja · Entonces **no** la trata como un hueco de
  relleno libre — la franja sigue reservada al `live_source` hasta la hora
  de inicio.
- **F2-25** [MANUAL] — Dado el equipo de streaming (OBS) enviando por SRT y
  por RTMP en pruebas separadas al mismo punto de escucha · Cuando se mide
  la latencia de cada una · Entonces SRT queda por debajo de 1 segundo y
  RTMP entre 2 y 5 segundos, confirmando la preferencia por SRT para
  alimentar transmisor.
- **F2-26** [MANUAL] — Dado un bloque en vivo activo con la señal SRT
  cortándose a mitad de transmisión (se desconecta el push) · Cuando el motor
  lo detecta · Entonces sigue **exactamente el mismo camino que una ausencia
  al inicio**: entra el relleno, suena la alarma, el bloque queda reservado,
  el motor reintenta con espera progresiva (1, 2, 4… hasta 60 s) y, cuando la
  señal vuelve, regresa al vivo en el siguiente borde de clip de relleno
  —máximo 60 s— con fundido cruzado (F2-19, F2-20). Nadie tiene que ir a
  apretar nada.

### Manual: tomar, soltar, fin de bloque, silencio real, escalera, join-in-progress (§9 paso 6)

- **F2-27** [AUTO] — Dado un canal recién iniciado sin que nadie tome el
  control · Cuando se consulta su estado · Entonces está en `AUTOMÁTICO`
  (nunca arranca en manual).
- **F2-28** [AUTO] — Dado un operador que presiona "Tomar el control" ·
  Cuando el sistema confirma la acción · Entonces el deck manual retiene el
  aire y queda en estado `MANUAL`, disparando lo que el operador decida.
- **F2-29** [AUTO] — Dado un operador en manual que abre el micrófono y
  habla de forma continua durante 10 minutos sin disparar ningún clip
  (audio real por encima de −60 dBFS todo el tiempo) · Cuando pasan esos 10
  minutos · Entonces el sistema **no** libera el control ni registra
  incidente — la actividad real evita el timeout aunque no se haya
  "disparado" nada.
- **F2-30** [AUTO] — Dado un operador en manual con silencio o negro real
  (audio < −60 dBFS o luma < 16) durante más de 15 segundos configurados ·
  Cuando se cumple ese umbral · Entonces el sistema suelta el control por sí
  solo, vuelve a `AUTOMÁTICO`, y registra un incidente tipo
  `manual_por_timeout` con su hora.
- **F2-31** [AUTO] — Dado un operador en manual con un spot pagado sonando
  que presiona **"Volver al automático"** · Cuando se confirma la acción ·
  Entonces el sistema **espera a que termine el clip en curso —como máximo 60
  segundos—** y solo entonces entrega el aire, con **fundido** y no con corte
  seco. Soltar el control nunca corta nada por el medio; para eso está "Parar
  todo" (F2-78).
- **F2-32** [AUTO] — Dado un operador que tomó el control durante un bloque
  y no lo suelta manualmente · Cuando el bloque termina (según el plan) ·
  Entonces el sistema vuelve solo a `AUTOMÁTICO` sin necesidad de que el
  operador presione nada.
- **F2-33** [MANUAL] — Dado un operador reteniendo el control · Cuando pasa
  el tiempo hacia el límite de retención · Entonces la cuenta regresiva está
  **visible todo el tiempo** que dura la retención (no solo al final); el
  color pasa a **ámbar a 60 segundos** del regreso; y en los **últimos 10
  segundos** pasa a **rojo** y suena un tono que se repite y se acelera,
  audiblemente distinto de la alarma final que suena al soltarse el control.
  Los tres números son defaults configurables.
- **F2-34** [AUTO] — Dado que el sistema regresa a automático por cualquiera
  de las tres vías (botón, fin de bloque, o timeout por silencio) · Cuando
  ocurre el regreso · Entonces el motor entra por el **minuto que le toca**
  en el plan actual (join-in-progress) con fundido, **no** reinicia el
  bloque desde su inicio.
- **F2-35** [AUTO] — Dado el panel de disparo del operador · Cuando el canal
  está en `AUTOMÁTICO` · Entonces el panel de disparo no está visible/activo
  — solo aparece mientras el deck manual tiene el control.

### Diferido (§9 paso 8)

- **F2-36** [AUTO] — Dado una ventana origen de 7:00 AM a 12:00 PM con 3
  `plan_item` cuyo origen fue `asset` (archivo) · Cuando se resuelve la
  ventana diferida de 1:00 a 6:00 AM configurada como
  `ventana_origen_inicio`/`ventana_origen_fin` apuntando a esa mañana ·
  Entonces se generan **nuevos** `plan_item` para la ventana diferida que
  apuntan a los **mismos `media_asset`**, no a la grabación (`air_recording`)
  de esa mañana.
- **F2-37** [AUTO] — Dado los cortes de la ventana origen (7:00-12:00) con
  sus `spot_airing` ya emitidos y facturados · Cuando se generan los cortes
  de la ventana diferida (1:00-6:00) · Entonces son registros **nuevos** de
  `corte`/`break_marker` — Avails distintos, no una copia ni una repetición
  de los `spot_airing` originales.
- **F2-38** [AUTO] — Dado que, dentro de la ventana origen (7:00-12:00), un
  tramo de 15 minutos fue **relleno** (no un programa de archivo) · Cuando
  se resuelve el diferido de esa ventana · Entonces ese tramo se **salta**
  (no se reprograma), y el hueco que deja en la ventana diferida se rellena
  con la biblioteca de relleno normal.
- **F2-39** [AUTO] — Dado que, dentro de la ventana origen, un tramo fue a
  su vez un **diferido** de una ventana anterior · Cuando se resuelve el
  nuevo diferido que la incluye · Entonces ese tramo se salta — **nunca hay
  recursión** de diferidos.
- **F2-40** [AUTO] — Dado que, dentro de la ventana origen, un tramo fue un
  **bloque en vivo** (sin `media_asset` propio) · Cuando se resuelve el
  diferido de esa ventana · Entonces ese tramo usa la **grabación**
  (`air_recording`) del bloque en vivo, **sin** sus cortes originales, que
  se sustituyen por cortes nuevos generados para la ventana diferida.
- **F2-41** [AUTO] — Dado que dentro de la ventana origen ocurrió un
  `alert_event` real (no prueba semanal/mensual) que cubrió parte del
  contenido · Cuando se resuelve el diferido de esa ventana · Entonces el
  tramo cubierto por la alerta **no** se retransmite en el diferido.

### Grabación (§9 paso 8, §15)

- **F2-42** [AUTO] — Dado el canal al aire · Cuando pasan 24 horas de
  operación continua · Entonces existe un `air_recording` sin huecos que
  cubre esas 24 horas (grabación continua de la salida real).
- **F2-43** [AUTO] — Dado un canal con la retención de grabación **por
  defecto de 7 días** (sube a **30** si el disco alcanza; ambos
  configurables) · Cuando pasa un día más allá de la retención vigente desde
  que se grabó un tramo · Entonces ese tramo se purga automáticamente (no
  crece sin límite), y la pantalla lo comunica **en días de historial, no en
  gigabytes**.
- **F2-44** [AUTO] — Dado un `plan_item` con `instante_planeado` =
  10:00-10:30 · Cuando se consulta qué `air_recording` cubre ese
  `plan_item` · Entonces la consulta devuelve el tramo de grabación del
  mismo canal cuyo rango de tiempo se solapa con 10:00-10:30 (cruce por
  canal + solape de tiempo, no por coincidencia exacta de instantes).
- **F2-45** [MANUAL] — Dado el canal transmitiendo en vivo ahora mismo ·
  Cuando un operador abre la vista de grabación y retrocede 20 minutos ·
  Entonces puede revisar ese tramo sin que la señal en vivo se detenga o se
  vea afectada.

### Salidas (§7, §10)

- **F2-46** [AUTO] — Dado un canal configurado con salida `udp-ts` al
  transmisor y salida a archivo simultáneamente · Cuando el motor produce
  su flujo continuo · Entonces ambas salidas reciben la señal al mismo
  tiempo, cada una con su propio estado de conexión.
- **F2-47** [AUTO] — Dado el mismo contenido saliendo por dos salidas, una
  configurada a −24 LKFS (transmisor, perfil `us-fcc`) y otra a −16 LUFS
  (internet) · Cuando se mide el volumen de cada salida por separado ·
  Entonces cada una está en su propio objetivo, no en un volumen único
  compartido.
- **F2-48** [AUTO] — Dado una salida de internet (ej. RTMP a un servidor) que
  se desconecta · Cuando el driver de salida detecta la caída · Entonces
  reintenta reconectar automáticamente y sin intervención humana, con
  **espera progresiva de 1, 2, 4… segundos y tope de 60**, y cada intento
  queda contabilizado en `reintentos` con su `ultimo_error`. El backoff nunca
  se rinde ni deja de reintentar.
- **F2-49** [AUTO] — Dado dos salidas activas del mismo canal, y una de ellas
  cae · Cuando se mide la continuidad de la salida que **no** cayó ·
  Entonces sigue produciéndose sin interrupción ni degradación mientras la
  otra reconecta.
- **F2-50** [AUTO] — Dado un canal recién configurado sin salida elegida
  todavía · Cuando se listan los drivers de salida disponibles · Entonces
  `udp-ts` está entre los disponibles desde F2 (confirmado como el primero
  que CAtv necesita, §25).

### Detector de silencio/negro en la salida (§9 pasos 4 y 6)

- **F2-51** [AUTO] — Dado la salida real del canal con audio por debajo de
  −60 dBFS durante 16 segundos continuos (por encima del umbral de 15 s) ·
  Cuando el detector evalúa la salida · Entonces dispara alarma y registra
  un incidente, **sin importar** lo que diga el plan en ese instante.
- **F2-52** [AUTO] — Dado la salida real del canal con luma por debajo de 16
  durante 16 segundos continuos · Cuando el detector evalúa la salida ·
  Entonces dispara alarma y registra un incidente.
- **F2-53** [AUTO] — Dado la salida real del canal con silencio de 8
  segundos (por debajo del umbral configurado de 15 s) dentro de una pausa
  dramática legítima de un programa · Cuando el detector evalúa la salida ·
  Entonces **no** dispara alarma ni incidente (evita falsos positivos en
  pausas cortas).
- **F2-54** [AUTO] — Dado el mismo umbral (audio < −60 dBFS o luma < 16 por
  > 15 s) configurado una sola vez para el canal · Cuando se dispara tanto
  en modo automático (§9 paso 4) como durante un `MANUAL_HOLD` (§9 paso 6) ·
  Entonces es el **mismo mecanismo** el que gobierna ambos casos, no dos
  implementaciones distintas (verificable por código: una sola fuente de
  verdad para el umbral y la detección).

### NTP y reloj (§14.1, §19)

- **F2-55** [MANUAL] — Dado un instalador terminando la instalación en una
  máquina nueva · Cuando finaliza · Entonces el instalador ya forzó la
  sincronización NTP, y la pantalla de Ajustes muestra la deriva de reloj
  contra el servidor de hora.
- **F2-56** [AUTO] — Dado la deriva del reloj del sistema contra el servidor
  NTP configurada en 1.5 segundos (por encima del umbral de 1 segundo) ·
  Cuando la pantalla de Ajustes evalúa la deriva · Entonces muestra un
  aviso; con deriva de 0.5 segundos, no lo muestra.
- **F2-57** [AUTO] — Dado un perfil con horario de verano activo (ej.
  `us-fcc` fuera de Puerto Rico) y una regla programada a las 2:30 AM del
  día del cambio de primavera (hora que no existe ese día) · Cuando el
  resolver la programa · Entonces la corre a la **siguiente hora válida**
  (3:30 AM), no la descarta ni falla.
- **F2-58** [AUTO] — Dado el mismo tipo de perfil y una regla programada a
  la 1:30 AM del día del cambio de otoño (hora que ocurre dos veces) ·
  Cuando el resolver la programa · Entonces sale en la **primera**
  ocurrencia de esa hora, y el diferido de esa misma noche —que usa la
  ventana en UTC— no la duplica ni se la salta.

### As-run e incidentes (§9 paso 3-4, §15, §23)

- **F2-59** [AUTO] — Dado un `plan_item` con `instante_planeado` = 10:00:00
  y `duracion_planeada` = 30:00, cuyo instante real de salida fue 10:00:04 y
  duró 29:58 · Cuando se consulta el registro después de emitirse ·
  Entonces `instante_planeado`/`duracion_planeada` **siguen intactos** en
  10:00:00/30:00, y `instante_real`/`duracion_real` reflejan lo que de
  verdad pasó — es la misma fila, no una tabla aparte.
- **F2-60** [AUTO] — Dado un `plan_item` en su ciclo de vida normal ·
  Cuando pasa por el motor · Entonces transiciona `planned` → `cued` →
  `aired`, en ese orden, sin saltarse `cued`.
- **F2-61** [AUTO] — Dado un `plan_item` cuyo archivo falla al aire (F2-12) ·
  Cuando se consulta su estado final · Entonces quedó en `fallido`, no en
  `aired`.
- **F2-62** [AUTO] — Dado un `plan_item` de programa cubierto por un
  `MANUAL_HOLD` mientras el operador tenía el control · Cuando se consulta
  su estado final · Entonces queda marcado `manual_hold`, distinguible de
  un `aired` normal.
- **F2-63** [AUTO] — Dado cada uno de los escenarios que generan incidente
  en este documento (encoder colgado F2-11, vivo ausente F2-19, timeout
  manual F2-30, caída a cartel F2-09/F2-10, cascada extendida F2-68, solape
  F2-73, apagón F2-96, salto de reloj F2-89, enlace caído F2-71) · Cuando
  ocurre cada uno · Entonces existe una fila en `incidente` con tipo, inicio,
  fin y detalle — ninguno ocurre sin dejar registro, y **cada tipo se
  distingue de los demás**.
- **F2-64** [AUTO] — Dado un incidente `vivo_ausente` donde el relleno cubrió
  la ausencia completa sin dejar aire negro (F2-19) · Cuando se calcula la
  métrica de "interrupciones no planificadas" del §23 · Entonces ese
  incidente **no** cuenta contra la métrica (el sistema funcionó como se
  diseñó), aunque sigue existiendo como fila en `incidente` para trazabilidad.
- **F2-65** [AUTO] — Dado 30 días de operación sin ningún incidente que deje
  negro, silencio no solicitado o cartel fuera de lo que pedía el plan ·
  Cuando se calcula la métrica de interrupciones no planificadas · Entonces
  el resultado es 0.
- **F2-66** [AUTO] — Dado un día completo de aire sin incidentes que afecten
  la programación · Cuando se compara la guía publicada contra el as-run de
  ese día · Entonces el 100% de los programas aparece en el espacio que la
  guía anunció, y el instante real de cada uno está dentro de **±30
  segundos** del instante planeado.

### Cascada, cartel y alarmas que se distinguen (§9 paso 4, §13)

- **F2-67** [MANUAL] — Dado una instalación recién terminada con el paso 1 del
  asistente contestado —nombre, identificativo y comunidad de licencia— ·
  Cuando se fuerza la cascada hasta su último escalón · Entonces sale el
  **cartel generado en ese paso**, con el identificativo y la comunidad de
  licencia visibles. Un canal recién instalado nunca se queda sin cartel, y
  una caída larga sigue identificando la estación.
- **F2-68** [AUTO] — Dado el cartel al aire como último escalón de la cascada ·
  Cuando lleva más de **15 minutos** seguidos · Entonces se registra un
  `incidente.tipo = cascada_extendida`, además de la alarma que ya disparó la
  caída.
- **F2-69** [AUTO] — Dado el motor en operación normal · Cuando se recorre la
  cascada completa de respaldo (programa → relleno → cartel) · Entonces
  **barras y tono no son alcanzables** como escalón de respaldo del aire:
  existen **solo** para la prueba del paso 5 del asistente (F2-104).
- **F2-70** [AUTO] — Dado el binario de `ffmpeg` borrado del disco mientras el
  canal está al aire (el antivirus se lo llevó) · Cuando el watchdog reacciona
  · Entonces **distingue este caso de un encoder colgado** y dice exactamente
  *"tu antivirus bloqueó ffmpeg — vuelve a aplicar las exclusiones en
  Ajustes"*, no *"el encoder no responde"*.
- **F2-71** [AUTO] — Dado el cable de red del canal desconectado · Cuando el
  sistema lee el estado de la interfaz · Entonces levanta la alarma
  `enlace_caido`, **distinta** de `encoder_no_responde` y distinta de un fallo
  de clip, y cada una dice qué pasó y qué hacer.
- **F2-72** [AUTO] — Dado un `media_asset` con `negro_intencional = true` que
  abre con 20 segundos de negro y silencio · Cuando sale al aire · Entonces el
  detector de silencio y negro **no dispara** mientras dura ese clip, y vuelve
  a estar activo en cuanto termina. Un `plan_item` cuyo origen es un
  `live_source` **nunca** puede desactivarlo.
- **F2-73** [AUTO] — Dado dos `plan_item` solapados que llegaron a existir pese
  a la restricción del esquema (F1-44) · Cuando el motor los encuentra en
  runtime · Entonces reproduce el de **menor `id`** y registra
  `incidente.tipo = solape` — el cinturón además del tirante.

### Fuente en vivo: fin de bloque, conformado y entrada rápida (§9 paso 5)

- **F2-74** [AUTO] — Dado un bloque en vivo reservado hasta las 13:00:00 con la
  señal todavía llegando · Cuando el reloj llega a las 13:00:00 · Entonces el
  audio del vivo hace un **fundido de 2 segundos** y el aire corta **al minuto
  planificado**: el vivo no se roba el programa siguiente.
- **F2-75** [AUTO] — Dado una señal en vivo que entra en 4:3 y varios decibeles
  por encima del objetivo de la salida · Cuando sale al aire · Entonces pasa
  por el **mismo conformado que un archivo** —escala, pillarbox y volumen al
  objetivo de cada salida—. **No existe una ruta "cruda" para el vivo.**
- **F2-76** [MANUAL] — Dado una instalación recién terminada, sin ninguna
  `live_source` creada · Cuando alguien apunta OBS al **puerto SRT fijo** de la
  entrada rápida, con su contraseña · Entonces la entrada aparece en *Al aire*
  sin haber configurado nada, y el operador la pone al aire con el botón de
  tomar el control. **Sin contraseña el puerto no acepta la conexión.**

### Manual: un solo tenedor, soltar y parar (§9 paso 6)

- **F2-77** [MANUAL] — Dado un operador con el control desde las 3:12 PM ·
  Cuando una segunda persona abre *Al aire* y pulsa **Tomar el control** ·
  Entonces ve *"Rolando tiene el control desde las 3:12 PM"* y un botón
  explícito para quitárselo; si lo usa, el traspaso queda en `audit_log` con
  nombre y hora. **Nunca hay dos tenedores de `manual_hold` a la vez.**
- **F2-78** [AUTO] — Dado un operador en manual a mitad de un spot pagado ·
  Cuando pulsa **"Parar todo"** · Entonces el aire corta **en seco**, sin
  esperar al clip, y el `plan_item` interrumpido queda marcado
  `parcial = true`. Es la diferencia con "Volver al automático" (F2-31).
- **F2-79** [AUTO] — Dado un `manual_hold` abierto cuando el servicio se cae y
  vuelve a arrancar · Cuando el motor se recupera · Entonces cierra la
  retención con `motivo_fin = caida_del_sistema` —ni `soltado`, ni
  `fin_de_bloque`, ni `timeout`—, porque sin ese motivo el registro miente.

### Diferido: la ventana vacía (§9 paso 8)

- **F2-80** [AUTO] — Dado una regla de diferido cuya ventana de origen no tuvo
  **ningún** `plan_item` del deck programa (estuvo vacía, o fue toda relleno) ·
  Cuando se resuelve el diferido · Entonces **no produce nada** y esa hora se
  rellena como cualquier otra. **No se inventa contenido para cumplir la
  regla.**

### Retorno de aire: dónde se captura la verdad (§9 paso 8, §10, ADR 0009)

- **F2-81** [AUTO] — Dado un canal con `capture_input` configurado —entrada de
  captura, receptor, o el propio flujo del transmisor— · Cuando se inspecciona
  de dónde leen `air_recording` y el driver `signal-compare` · Entonces leen
  del **retorno de aire: la señal ya transmitida, después del equipo de
  alertas**, nunca de la salida propia de Antena787.
- **F2-82** [MANUAL] — Dado una estación que no puede dar retorno de aire ·
  Cuando se configura el canal sin `capture_input` · Entonces
  `capture_input.modo` queda en **`degradado`**, la interfaz lo dice sin
  rodeos, y `signal-compare` **no promete detectar nada** — en vez de reportar
  un as-run limpio y falso.
- **F2-83** [AUTO] — Dado un canal en modo `degradado` y un tramo **ya
  emitido** que fue interrumpido por una alerta real · Cuando una persona lo
  marca desde la bitácora de incidentes, **hacia atrás** · Entonces se crea el
  `alert_event` con `origen = manual` y los tramos tapados quedan marcados
  igual que si el sistema los hubiera detectado solo.
- **F2-84** [AUTO] — Dado un canal con retorno de aire activo · Cuando
  `signal-compare` detecta que lo transmitido no es lo que decía el plan ·
  Entonces crea el `alert_event` con `origen = automatico`: el campo distingue
  **siempre** quién lo detectó.
- **F2-85** [AUTO] — Dado un `alert_event` de hace 18 meses · Cuando corre
  cualquier rutina de purga o de retención · Entonces **no se borra**: la
  retención mínima de `alert_event` es de **24 meses** y nada lo purga antes.
- **F2-111** [AUTO] — Dado un corte comercial en curso que una alerta real del
  ENDEC interrumpe a la mitad de un spot · Cuando el equipo de alertas suelta el
  aire · Entonces el deck comercial **retoma el corte y lo termina entero** —el
  spot interrumpido vuelve a salir desde su cabeza, nunca recortado ni sustituido
  por negro—, el programa **absorbe el tiempo perdido** (se reincorpora en
  curso o se recorta por la cola) sin mover el reloj ni el corte siguiente, el
  spot que salió entero después cuenta como `aired` sin make-good, y solo el
  que ya no cupo en su corte queda `preempted` y va a la propuesta de
  reposición (§9 paso 10). Es lo que CAtv hace hoy a mano (ADR 0010,
  9 sept 2026): la alerta nunca se retrasa por un comercial; lo que se protege
  es lo que viene después.
- **F2-112** [AUTO] — Dado un canal encendido (en sombra o al aire) en una
  máquina con suspensión por inactividad · Cuando pasa el tiempo de inactividad
  del sistema · Entonces la máquina **no se duerme**: el proceso sostiene la
  aserción de «no dormir» de cada sistema (macOS `IOPMAssertion`/`caffeinate`,
  Windows `SetThreadExecutionState`, Linux `systemd-inhibit`) mientras el canal
  esté encendido, la suelta al apagarse, y Al aire dice si no pudo sostenerla.
  Si aun así el reloj monotónico se queda atrás del de pared, el incidente
  `salto_de_reloj` lo dice como posible sueño de la máquina y recalcula. Viene
  del modo sombra del 9 sept 2026: la Mac durmió 19 min en dos ratos y nada lo
  impidió (`docs/f1/SOMBRA-2026-09-09.md`, S-8).

### Que un fallo interno no tumbe el aire (§14.1)

- **F2-86** [AUTO] — Dado un pánico provocado a propósito en el **servidor de
  cuadros** —y, en corridas separadas, en el **watchdog** y en el
  **resolver**— · Cuando ocurre · Entonces esa goroutine **atrapa su propio
  pánico**, registra un `incidente` y **se relanza sola**: el proceso `antena`
  no muere y el aire no se interrumpe.
- **F2-87** [AUTO] — Dado el proceso `antena` terminado de golpe · Cuando pasan
  unos segundos · Entonces el **supervisor del sistema** —servicio de Windows o
  systemd— lo vuelve a levantar solo, y la recuperación es la de F2-13: seek al
  segundo que toca, nunca reiniciar el bloque desde cero.
- **F2-88** [AUTO] — Dado el consumo de memoria residente del proceso subiendo
  por encima del umbral configurado · Cuando se cruza ese umbral · Entonces
  suena la alarma **antes** de que el sistema operativo mate el proceso, no
  después.
- **F2-89** [AUTO] — Dado el motor al aire y la hora de pared movida **más de
  60 segundos** de golpe, hacia adelante o hacia atrás (un NTP que llega tarde,
  o alguien que cambia la hora) · Cuando el motor lo detecta · Entonces levanta
  la alarma **`salto_de_reloj`, distinta de la de deriva**, **no re-emite nada
  que ya esté `aired`**, **no marca nada como vencido de golpe**, y el resolver
  recalcula el plan **desde el instante real**.
- **F2-90** [AUTO] — Dado el motor al aire y la hora del sistema operativo
  cambiada a mano hacia atrás · Cuando se mide la continuidad de la salida ·
  Entonces el aire **no salta ni repite un solo cuadro**: el conteo del aire va
  por **reloj monotónico**, no por `time.Sleep` ni por la hora de pared.

### Endurecimiento: disco, base y corriente (F2.5, §19)

- **F2-91** [AUTO] — Dado el disco de grabación con menos del **10 %** libre ·
  Cuando el vigilante de disco evalúa · Entonces suena la alarma, y nada más:
  el aire sigue igual.
- **F2-92** [AUTO] — Dado el disco por debajo del **5 %** libre · Cuando el
  vigilante evalúa · Entonces la grabación **se purga sola** —de lo más viejo a
  lo más nuevo— hasta liberar espacio, sin tocar el aire.
- **F2-93** [AUTO] — Dado el disco por debajo del **2 %** libre · Cuando el
  vigilante evalúa · Entonces la base **deja de escribir el as-run** y lo dice
  en rojo, **pero el aire sigue saliendo** desde el plan que ya está en
  memoria: se pierde el registro, nunca la señal.
- **F2-94** [AUTO] — Dado un archivo de base de datos corrupto · Cuando el
  sistema arranca y corre **`PRAGMA integrity_check`** · Entonces la
  verificación falla, el sistema **restaura solo el respaldo más reciente**,
  arranca, y avisa lo que pasó — no se queda esperando a que llegue un humano.
- **F2-95** [AUTO] — Dado el plan de las próximas **48 horas** ya resuelto ·
  Cuando la base se vuelve inaccesible (disco al 2 %, corrupción, o bloqueo) ·
  Entonces el motor sigue emitiendo desde la **copia en memoria** de esas 48
  horas: ni la pantalla queda en blanco ni el aire en silencio.
- **F2-96** [AUTO] — Dado un apagón de 6 horas y 12 minutos · Cuando vuelve la
  corriente y el sistema arranca · Entonces manda por el canal de avisos
  *"estuve fuera 6 horas 12 minutos"* y crea un `incidente.tipo = apagon` con
  su hora de inicio y de fin.
- **F2-97** [AUTO] — Dado un canal sin salida a internet y sin servidor de hora
  en la red local · Cuando el reloj lleva **24 horas** sin sincronizar ·
  Entonces avisa; a los **7 días** levanta **alarma**; y en ningún momento deja
  de emitir por eso.
- **F2-98** [AUTO] — Dado el binario de `ffmpeg` junto al ejecutable · Cuando
  el sistema se instala y **cada vez que arranca** · Entonces verifica su
  **SHA-256** contra el valor de la release, y si no coincide lo dice **antes**
  de salir al aire, no a mitad de un programa.
- **F2-99** [AUTO] — Dado el instalador aplicando las exclusiones de antivirus
  sobre las carpetas de medios · Cuando termina · Entonces la carpeta
  **`portal/entrada/` no queda excluida**, y un archivo subido por el portal
  llega a la biblioteca **solo después** de pasar el límite de tamaño y
  duración, la medición con `ffprobe` **sin acceso a la red y con límite de
  tiempo**, y el escaneo del antivirus del sistema si existe.
- **F2-100** [MANUAL] — Dado una máquina Windows 10 con la política de
  actualizaciones ya aplicada por el instalador —diferimiento, sin reinicio
  automático, horas activas— · Cuando alguien la desconfigura · Entonces la
  pantalla de **Ajustes lo detecta y lo avisa**: la vigilancia es permanente,
  no solo del instalador.
- **F2-101** [AUTO] — Dado Tailscale caído · Cuando se mide la salida del canal
  · Entonces **no cambia nada**: el acceso remoto no es parte de la cadena de
  emisión, y lo único que se pierde es poder mirar desde afuera.
- **F2-102** [AUTO] — Dado el canal sin salida a internet durante seis horas,
  con alarmas ocurriendo en ese tramo · Cuando vuelve la conexión · Entonces el
  canal de avisos manda **todos los mensajes acumulados**, ninguno se perdió, y
  ninguno de los servicios de red falló en silencio mientras no había
  conexión: cada uno degradó con un aviso en cristiano.

### El asistente y las puertas de fase (§13, §16, §22.3, §23)

- **F2-103** [MANUAL] — Dado una instalación desde cero · Cuando se recorre el
  asistente completo, sus nueve pasos · Entonces el usuario contesta **seis
  preguntas o menos**: los pasos 3 (revisión automática), 8 (primera parrilla
  propuesta) y 9 (al aire) no preguntan nada.
- **F2-104** [MANUAL] — Dado el paso 5 del asistente · Cuando llega su turno,
  **antes de que se haya subido un solo video** · Entonces el sistema manda
  **barras y tono** a la salida configurada y pregunta *"¿Ves las barras de
  color?"*; la respuesta *"no"* lleva a una salida con un valor por defecto que
  funciona, nunca a un callejón sin salida.
- **F2-105** [MANUAL] — Dado el paso 4 del asistente (*"¿A dónde va tu
  señal?"*) · Cuando el usuario lo contesta · Entonces **nunca se le pide
  elegir un driver por su nombre**: el sistema escanea la máquina y la red
  primero, y propone en lenguaje llano lo que encontró (principio 1).
- **F2-106** [MANUAL] — Dado un asistente que llega al paso de la primera
  parrilla con la **biblioteca de relleno vacía** · Cuando evalúa el relleno
  disponible · Entonces avisa que el primer hueco saldría en negro y ofrece
  **crear un relleno por defecto de un clic** —el cartel de la estación con una
  cama musical—, en vez de dejar que se descubra a las 3 AM.
- **F2-107** [MANUAL] — Dado una máquina limpia y el instalador · Cuando se
  cronometra desde el primer clic · Entonces hay **barras de color en pantalla
  en menos de 15 minutos** y **canal al aire con programación en menos de una
  hora** (§23).
- **F2-108** [MANUAL] — Dado el conjunto de instalaciones acompañadas con
  soporte —la única fuente de datos que existe, porque no hay telemetría y no
  la habrá sin consentimiento explícito— · Cuando se cuenta cuántas se
  abandonaron dentro del asistente · Entonces la proporción es **menor al
  10 %**, y queda anotado en qué paso se abandonó.
- **F2-109** [MANUAL] — Dado el motor, el conformado y el watchdog terminados
  —unas **7,000 líneas críticas**, §22.3— · Cuando se quiere cerrar F2 ·
  Entonces existe **constancia registrada** de que un humano las leyó **línea
  por línea**, no un resumen ni un muestreo; sin esa constancia F2 no se cierra.
- **F2-110** [MANUAL] — Dado el editor de reglas terminado · Cuando **al menos
  una persona no técnica**, sin ayuda, arma una semana completa de parrilla ·
  Entonces la completa dentro de la hora del §23 y contestando seis preguntas o
  menos; si no lo logra, el editor se rediseña **antes** de publicar (§24).

---

## Resumen

- **F0:** 9 criterios (F0-01 a F0-09).
- **F1:** 71 criterios (F1-01 a F1-71; del 9 de septiembre de 2026: F1-58 a F1-63 audio de todo el material, F1-64 a F1-67 emparejar títulos, F1-68 y F1-69 cuarentena e incidentes en pantalla, F1-70 y F1-71 lo aprendido de los proyectos comparables).
- **F2:** 110 criterios (F2-01 a F2-110).
- **Total: 184 criterios de aceptación**, de los cuales **158 son [AUTO]** y
  **26 son [MANUAL]**.

**Qué cambió en la revisión del 8 de septiembre de 2026** (contra el PRD
posterior a `docs/AUDITORIA_2026-09-04.md`):

| | |
|---|---|
| Criterios **corregidos en su sitio** | 18 — ninguno se borró |
| Criterios **añadidos** | 69 (25 en F1, 44 en F2) |
| Requisitos que antes eran "no verificables por falta de número" | 13 → **todos convertidos** en criterio |
| Total | 107 → **176** |

**Los 18 corregidos, y contra qué decisión del PRD chocaban:**

| Criterio | Decía antes | Dice ahora |
|---|---|---|
| Encabezado | "donde el PRD no da un número, no se inventó uno" | el PRD ya da los números (C12 de la auditoría) |
| **F1-01** | "hasta que el tamaño deje de cambiar" | **tamaño estable 10 s** y el archivo se abre sin bloqueo |
| **F1-14** | 10 episodios siempre generan 10 `plan_item` | solo si caben; si no, manda el reloj (F1-38) |
| **F1-16** | `es_segundo_pase = true`, Encore con regla propia | **`repite_a`**: el mismo episodio, **sin contador propio** |
| **F1-22** | ejemplo con exceso de 0:57 dentro de un límite de 5 s | ejemplo correcto: exceso ≤5 s, se recorta **el último clip de relleno** con fundido de **1 s** |
| **F2-10** | la cascada terminaba en **barras y tono** | termina en el **cartel**; barras y tono solo en la prueba del asistente |
| **F2-11** | "el PRD no fija el umbral de *no responde*" | **3 s**, mismo acelerador, **dos fallos en 10 min → software** |
| **F2-12** | cuarentena al primer fallo al aire | cuarentena **tras dos fallos separados por más de 5 min** |
| **F2-14** | toda diferencia se corrige por deriva, "nunca por salto" | deriva **hasta 60 s**; por encima es `salto_de_reloj` (F2-89) |
| **F2-19** | sin señal a la hora exacta → relleno y alarma | **margen de gracia de 30 s** sin alarma; después, relleno y alarma |
| **F2-20** | vuelve al vivo "en ese instante" | vuelve **en el siguiente borde de clip, máx. 60 s, con fundido cruzado** |
| **F2-22** | `retardo_ms` = 3000 "de ejemplo" | **7 000 ms**, el default del PRD |
| **F2-26** | "el PRD no dice si conmuta a relleno" | caída a mitad de bloque = **el mismo camino** que la ausencia inicial |
| **F2-31** | "Volver al automático" hace fundido | además **espera a que termine el clip en curso, máx. 60 s** |
| **F2-33** | tono acelerado en los **últimos 15 s** | **ámbar a 60 s**, **rojo a 10 s** |
| **F2-43** | retención de "N días" sin default | **7 días** por defecto, **30** si el disco alcanza |
| **F2-48** | "reintenta automáticamente" | **espera progresiva 1, 2, 4… con tope de 60 s** |
| **F2-63** | listaba 4 tipos de incidente | lista los 9, y exige que **cada tipo se distinga** |

**Qué cambió en la verificación del 9 de septiembre de 2026** (los 57 criterios
de F1 recorridos uno por uno contra lo construido; informe en
`docs/f1/VERIFICACION-F1-2026-09-09.md`):

| Criterio | Decisión | Por qué |
|---|---|---|
| **F1-04** | **diferido a F2** | no hay paso de extracción/reinserción de CEA-608 (tamaño L) y ffprobe ≥ 9 ya no emite `closed_captions`; la prueba queda escrita y saltada |
| **F1-22** | F1 deja el dato, F2 aplica el fundido | el plan guarda `fundido_salida_ms = 1000` en el clip recortado; el motor lo aplica |
| **F1-39** | la mitad `dentro_de` es F2 | el resolver de F1 no materializa `dentro_de`; sí se verifica que el fin del vivo es duro para lo que arranca dentro |
| **F1-43** | la mitad de Anuncios/portal es F4 | no hay pantalla de Anuncios ni compras en F1 |
| **F1-55** | la mitad del reporte de ingresos es F4 | anunciante y cobro se guardan y el bloque sale en parrilla y guía; el reporte vive con publicidad (F4) |
| **F1-57** | el alta de anunciantes es F4 | `hay_anunciantes` se calcula de la tabla; la pantalla de alta llega con Anuncios |

Ninguno se borró ni se renumeró. Los seis siguen contando dentro de los 57 de F1
con su alcance ajustado; la parte diferida se vuelve a verificar en su fase.

---

## Los 13 requisitos que antes no eran verificables — dónde quedó cada uno

La versión anterior cerraba con trece enunciados que no se pudieron convertir
en criterio, casi todos porque **al PRD le faltaba un número**. El PRD
posterior a la auditoría los da todos. Esto es la traza:

| # | Enunciado | Lo que faltaba | Ahora |
|---|---|---|---|
| 1 | Principio 1, *"detectar antes de preguntar"* (§4) | una instancia concreta | **F2-105** (nunca se pide elegir un driver) y **F2-103** (≤6 preguntas) |
| 2 | Principio 3, *"nada de jerga"* (§4) | una lista cerrada de palabras | **F1-56** — el PRD nombra los cinco términos: *driver, códec, GOP, LKFS, transport stream* |
| 3 | Principio 9, *"revelación progresiva"* (§2, §4, §11) | un umbral observable | **F1-57** — cinco entradas de menú, seis con Anuncios |
| 4 | *"espera a que el archivo termine de copiarse"* (§9 paso 1.0) | el intervalo | **F1-01** — tamaño estable **10 s** y se abre sin bloqueo |
| 5 | Watchdog, *"si el encoder no responde"* (§9 paso 4) | los segundos | **F2-11** — **3 s**, y **2 fallos en 10 min** para caer a software |
| 6 | *"el color pasa a ámbar"* (§9 paso 6) | en qué segundo | **F2-33** — **ámbar a 60 s**, **rojo a 10 s** |
| 7 | *Profanity delay* de la fuente en vivo (§9 paso 5) | el default | **F2-22** — **7 segundos** |
| 8 | La fuente en vivo *"se cae a la mitad"* (§9 paso 5) | qué hace el sistema | **F2-26** — relleno, alarma, reintento y regreso en borde de clip |
| 9 | *"reconexión con espera progresiva"* (§10) | el esquema de backoff | **F2-48** — **1, 2, 4… tope 60 s**, sin rendirse |
| 10 | Retención de la grabación (§19, §21) | los días | **F2-43** — **7 días**, **30** si el disco alcanza |
| 11 | *"instalaciones abandonadas < 10 %"* (§23) | la fuente de datos | **F2-108** — se mide sobre **las instalaciones con soporte**; el PRD dice explícitamente que no hay ni habrá telemetría sin consentimiento |
| 12 | *"el motor se lee línea por línea"* (§16) | qué contar | **F2-109** — **~7,000 líneas críticas** (§22.3), como **puerta de cierre de F2** |
| 13 | *"probarlo con alguien no técnico"* (§24) | cuántas personas y qué umbral | **F2-110** — **al menos una** persona no técnica, con los umbrales del §23: **< 1 hora** y **≤ 6 preguntas** |

**Lo que sigue sin ser una prueba automatizable**, y se dice de frente: los
criterios 1, 2, 3, 11, 12 y 13 se convirtieron en **puertas de fase [MANUAL]**
—alguien mira y firma—, no en pruebas de integración. Cumplen su función como
criterio de aceptación porque ahora tienen un umbral que se puede fallar; lo
que no tienen es un runner que los corra solo.

---

## Cobertura contra las decisiones de la auditoría

Traza rápida de las decisiones de `docs/AUDITORIA_2026-09-04.md` que caen
dentro de F0-F2.5, contra el criterio que las verifica:

| Decisión | Criterio |
|---|---|
| A1 · captura después del equipo de alertas, `capture_input`, modo `degradado`, `alert_event.origen` | F2-81 · F2-82 · F2-83 · F2-84 |
| A2 · pánico por goroutine, supervisor, vigilancia de memoria | F2-86 · F2-87 · F2-88 |
| A3 · `portal/entrada/` fuera de la exclusión de antivirus | F2-99 |
| B1 · todo por día de emisión, `fecha_fin` inclusiva | F1-33 · F1-34 · F1-51 |
| B2 · no hay reglas anuales | F1-35 |
| B3 · el ítem pertenece al día de su inicio | F1-40 |
| B4 · sobrecupo, "de 10 caben 9" | F1-38 · F1-39 |
| B5 · `repite_a` con contador compartido | F1-16 · F1-36 · F1-37 |
| B6 · no-solape en el esquema | F1-44 · F2-73 |
| B7 · solo material `listo`, cola por hora de aire | F1-41 · F1-42 · F1-43 |
| B8 · XMLTV en cada corrida y cada cambio, ≤1 min, `/guia.xml` | F1-47 · F1-48 · F1-49 |
| B9 · ventana sin programa no produce diferido | F2-80 |
| B10 · avisos suprimidos por relevo, canal de avisos a 7 días | F1-45 · F1-46 |
| B11 · el importador nunca rechaza la hoja entera | F1-50 · F1-51 · F1-52 · F1-53 |
| B12 · relleno por defecto, "Llenar con diferido" | F1-54 · F2-106 |
| C1 · gracia 30 s, regreso en borde de clip, fundido de 2 s, mismo conformado | F2-19 · F2-20 · F2-26 · F2-74 · F2-75 |
| C2 · `negro_intencional` | F2-72 |
| C3 · un solo tenedor de `manual_hold` | F2-77 |
| C4 · "Soltar" espera al clip; "Parar todo" marca `parcial` | F2-31 · F2-78 |
| C5 · `motivo_fin = caida_del_sistema` | F2-79 |
| C6 · la cascada termina en el cartel; `cascada_extendida` | F2-10 · F2-67 · F2-68 · F2-69 |
| C7 · watchdog de 3 s, mismo acelerador, software a la segunda | F2-11 |
| C8 · cuarentena tras dos fallos separados >5 min | F2-12 |
| C10 · alarma `enlace_caido` | F2-71 |
| C11 · "tu antivirus bloqueó ffmpeg" | F2-70 |
| C12 · los defaults (10 s · 3 s · 60/10 s · 7 s · 1-60 s · 7/30 días · 15 s) | F1-01 · F2-11 · F2-33 · F2-22 · F2-48 · F2-43 · F2-51 a F2-54 |
| C13 · se recorta el último clip de relleno, fundido de 1 s | F1-22 |
| D1 · umbrales de disco 10/5/2 % | F2-91 · F2-92 · F2-93 |
| D2 · `integrity_check` y restauración sola; plan de 48 h en memoria | F2-94 · F2-95 |
| D3 · reloj monotónico y salto >60 s | F2-89 · F2-90 |
| D4 · "estuve fuera 6 h 12 min", `incidente.tipo=apagon` | F2-96 |
| D5 · Tailscale caído no afecta el aire | F2-101 |
| D6 · sin internet todo degrada con aviso; NTP 24 h / 7 días | F2-97 · F2-102 |
| E7 · retención de 24 meses de `alert_event` | F2-85 |
| E9 · entrada rápida de vivo; `bloque_arrendado` | F2-76 · F1-55 |
| F · SHA-256 de ffmpeg; contraseña en las entradas en vivo | F2-98 · F2-76 |
| G · Ajustes vigila deriva, aceleración y umbral de silencio | F2-15 · F2-55 · F2-56 · F2-100 |

**Fuera del alcance de este documento, a propósito:** las decisiones E1-E6,
E8 y E9 de publicidad, portal, cobro, make-good y perfil `us-fcc` pertenecen a
**F4**, y A5-A8 (calendario, partición de fases, ruta SCTE alterna) son
decisiones de planificación, no conductas del sistema. Cuando F4 entre en
construcción, este documento gana su sección.
