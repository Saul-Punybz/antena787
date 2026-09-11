# Auditoría consolidada — 4 de septiembre de 2026

Nueve agentes independientes revisaron `PRD.md`, `CONTEXT.md`, los ADR y las 13 pantallas. Este archivo junta **todos** los hallazgos con la **decisión tomada** para cada uno. Es el insumo de la pasada de corrección y queda como registro de por qué el PRD dice lo que dice.

Los nueve: (1) semana real de Rolando 6–12 sept · (2) modos de fallo · (3) seguridad · (4) operación sin internet · (5) criterios de aceptación → `docs/ACEPTACION.md` · (6) pantallas contra requisitos · (7) FCC con fuentes primarias · (8) estimación honesta · (9) dueño escéptico de emisora.

Convención: **D** = decisión tomada aquí y aplicada al PRD · **R** = decisión de Rolando (va a §25 / PARA-ROLANDO) · **S** = decisión de Saul, pendiente.

---

## 0 · Marco (fijado por Saul el 8 de septiembre, manda sobre todo lo demás)

**El Sheet de CAtv es una demostración, no la operación real.** Rolando lo mandó para enseñar *cómo* lo corre hoy; está incompleto a propósito: no tiene anuncios, ni pausas, ni la programación completa. Los "huecos" que encontró la simulación de la semana (madrugadas vacías, domingo vacío, Hellsing con fechas cruzadas, sin relleno, sin diferido) **no son errores de Rolando** — son la evidencia de lo que la herramienta tiene que hacer trivial: **llenar el tiempo, cambiar el tiempo, interrumpir la programación, añadir, quitar.** Eso es lo que el PRD §3 debe decir: "esto muestra el trabajo que hoy es a mano y la herramienta vuelve de un clic", nunca "esto está mal corrido".

**Sobre la FCC: así corren muchos canales locales en Puerto Rico y en el mundo, y no pasa nada.** El software **nunca regaña**. No dice "estás en violación", no bloquea nada por cumplimiento, no asume que el operador debe nada. Las funciones de cumplimiento (bitácora EAS, archivo político, registro de loudness, identificación de patrocinio) están **ahí para quien las quiera**, se activan con el perfil del país, y se presentan como "lo tienes listo si alguien lo pide", no como obligación. Todo lo de la sección E se aplica con ese tono. El PRD §12 y §24 se revisan para quitar cualquier frase que suene a advertencia legal.

Lo que sí se mantiene firme, porque es a favor de Rolando y no en su contra: nunca negro, nunca silencio, nada manual que no vuelva solo.

---

## A · Lo que cambia arquitectura o promesa (aplicar primero)

| # | Hallazgo | Decisión |
|---|---|---|
| A1 | **Punto de captura de la señal.** `signal-compare` y la grabación continua nunca dicen *dónde* miran. Si miran la salida propia de Antena787 (antes del ENDEC), una alerta real de emergencia **no se detecta nunca**: el motor siguió emitiendo, el as-run queda "aired" limpio y falso, y la bitácora EAS de `us-fcc` es ficción. (#1, #2) | **D.** La verdad es la señal transmitida, no nuestra salida. `signal-compare` y la grabación capturan **después del equipo de alertas** (retorno de aire: entrada de captura, receptor ATSC, o el propio stream del transmisor). Si la estación no puede dar un retorno, el driver corre en modo `degradado`, el PRD lo dice sin rodeos, y las alertas se marcan a mano. **Nuevo ADR 0009.** Se añade `alert_event.origen = automatico\|manual` y la vía de marcado manual/retroactivo. |
| A2 | **Pánico/OOM del servidor de cuadros tumba todo el proceso** (web + resolver + motor juntos por §14.1). Contradice "nunca negro". (#2) | **D.** Cada goroutine crítica (servidor de cuadros, watchdog, resolver) recupera pánico, registra `incidente`, y se reinicia sola sin tumbar el proceso. El proceso corre bajo supervisor (servicio de Windows / systemd) con reinicio automático; la recuperación de estado es la de §14.1 (seek al segundo correcto). Vigilancia de memoria: RSS por encima de umbral → alarma antes del OOM. Sigue siendo un solo proceso; §14.1 se corrige para decir esto. |
| A3 | **El portal público sube archivos a una carpeta que nosotros excluimos del antivirus** (§19). (#2, #3) | **D.** La carpeta de entrada del portal (`portal/entrada/`) **no** hereda la exclusión. Lo subido se valida ahí (límite de tamaño y duración, `ffprobe` con `-protocol_whitelist file` y sin red, escaneo del antivirus del sistema si existe) y solo entonces se mueve a la biblioteca. Nada del portal llega a la carpeta excluida sin pasar por eso. |
| A4 | **El as-run no es un requisito de la FCC.** La FCC eliminó los registros de programación; la bitácora de estación (73.1820) cubre luces de torre y EAS. El PRD lo presenta cerca de "cumplimiento". (#7) | **D.** §12 y §9 Paso 8 dicen explícito: el as-run es evidencia comercial (facturación, make-goods, prueba ante anunciantes), **no** obligación regulatoria. Lo que sí es obligación: bitácora EAS (con retención de 2 años), identificación de estación, loudness. No vender cumplimiento que no existe. |
| A5 | **Calendario honesto.** A 10 h/semana: modo sombra ~1 año, madrugadas ~3.4 años, aire completo ~3.9 años, primer anunciante cobrado ~5.6 años (probable). "F0 son cinco días" es fantasía a ese ritmo. ffplayout, más chico, tomó >2 años. (#8) | **D.** §22 lleva la tabla de calendario (optimista / probable / pesimista) y el supuesto de ritmo. Se dice que la única palanca real es más horas en F1/F2, no recortar F4. F0 pasa a "5 días de trabajo, no 5 días de calendario; presupuestar una segunda corrida de 8 h". |
| A6 | **Partir F3.** Mezcla "que un Windows 10 sin presupuesto aguante 24/7" (que Rolando necesita aunque el proyecto nunca se publique) con "abrir el repo". (#8) | **D.** Nueva **F2.5 · Endurecimiento** (antivirus, NTP, política de Windows Update, vigilancia permanente) en paralelo al soak de 30 días. F3 queda solo "Público" (CI, docs, legal, matriz de hardware). |
| A7 | **La ruta de inyección SCTE-35 de respaldo** es 3–4× el crawl, sin caso de referencia en el mundo, y es escribir un remuxer — lo que ADR 0004 dice que evitamos. (#8) | **D.** Fuera de v1. Si el encoder de Rolando no habla SCTE-104 (§25 pregunta 5, **R**), CAtv vende cortes localmente sin sistemas de inserción de terceros. La ruta de respaldo pasa a F5 y se marca "sin precedente documentado". |
| A8 | **F2 subestimada por tabla.** Una fila junta 4–5 subsistemas del tamaño de F1, todos con revisión línea por línea (~7,000 líneas críticas = 35 h solo de lectura). (#8) | **D.** §22 desglosa F2 en sus subsistemas con orden interno: motor+conformado → decks → salida udp-ts → detector de salida → fuentes en vivo → manual → grabación/diferido → cascada/watchdog. |

## B · Tiempo y resolver

| # | Hallazgo | Decisión |
|---|---|---|
| B1 | `fecha_inicio`/`fecha_fin` y el patrón LMMJVSD: ¿por fecha calendario o por día de emisión? Afecta a toda regla de las 12:00–5:59 AM (313, 304, 336, 333, 353 del Sheet). (#1, #2) | **D.** **Todo por día de emisión.** `fecha_fin` es inclusiva hasta el cierre del día de emisión (5:59:59 AM del día calendario siguiente). El importador de Excel/Sheets sabe que el Sheet de CAtv usa fecha calendario: para reglas cuya hora cae entre 12:00 y 5:59 AM corre las fechas un día atrás y lo reporta fila por fila. |
| B2 | Regla anual y 29 de febrero. (#2) | **D.** Una línea en §15 Convenciones: las reglas no tienen fecha "anual"; el patrón es semanal y las fechas son absolutas, así que el bisiesto no existe como caso. |
| B3 | Programa que **cruza** las 6 AM (3 h desde las 5 AM). (#2) | **D.** El `plan_item` completo pertenece al día de emisión de su **inicio**. Reportes y diferido usan esa atribución. |
| B4 | **Sobrecupo:** contenido real que **excede** el slot (Gaming Longplays ×10 en 5 h). Solo se define el subcupo. (#1) | **D.** El reloj manda. Un episodio que no termina antes del siguiente inicio duro **no se arranca**: el resolver lo deja fuera, rellena el resto, y avisa "de 10 episodios caben 9". El contador de episodios avanza solo por lo que salió. Nunca se corta un episodio a la mitad por sobrecupo. Aplica igual a fuentes en vivo (el fin del bloque es duro). |
| B5 | **Segundo pase:** cada regla lleva su contador; el pase de las 11 PM puede desincronizarse del de las 2 PM. (#1) | **D.** `es_segundo_pase` se reemplaza por `repite_a: schedule_rule.id`. Un segundo pase **repite el mismo episodio** que su regla primaria emitió ese día de emisión. Si la primaria no emitió nada ese día, el segundo pase toma el siguiente episodio de la primaria y avanza el contador **compartido** — la serie nunca se estanca. Un solo contador por serie-en-slot. |
| B6 | Dos `plan_item` solapados por bug, sin restricción de esquema. (#2) | **D.** Restricción en el esquema: dentro de un mismo `deck` y `output` no puede haber dos `plan_item` con intervalos solapados (índice + verificación al insertar). El motor, además, si encuentra solape en runtime, reproduce el de menor `id` y registra `incidente.tipo=solape`. |
| B7 | Siguiente clip aún no normalizado / normalización tarda 6 h. (#2) | **D.** El resolver solo programa `media_asset.estado_normalizacion = listo`. Cola de normalización priorizada por "cuándo sale al aire": lo que sale antes se normaliza antes. Biblioteca y Anuncios muestran "aún no listo para aire". Una compra del portal no se da por lista hasta terminar. |
| B8 | XMLTV: ¿cuándo se regenera, dónde se publica, y qué pasa en un relevo? (#1, #4) | **D.** Se regenera en cada corrida del resolver (cada hora) **y** en el instante de cualquier cambio de plan. Se sirve siempre en `/guia.xml` y se escribe en una ruta configurable (archivo local que lee MistServer/el transmisor); opcional: HTTP PUT a un destino. La guía nunca puede tener más de un minuto de desfase con el plan. |
| B9 | Diferido cuya ventana de origen es un **Gap puro** (no relleno ni diferido). (#1) | **D.** El diferido copia solo `plan_item` del deck `programa`. Si la ventana fuente no tuvo programa, el diferido no produce nada y la hora cae a relleno. Se enumera como tercer caso en §9 Paso 8. |
| B10 | Avisos de vencimiento (30/14/7): ¿dónde aparecen? ¿se suprimen si ya hay relevo? Importador no detecta relevos (346→352). (#1) | **D.** Aparecen en Al Aire, Reglas y Mes; a 7 días también por el canal de avisos (Telegram/WhatsApp). Se suprimen cuando hay `releva_a` cargado. El importador infiere relevo cuando una regla B empieza el día siguiente al fin de A en el mismo slot y patrón, y pide confirmar "¿Zoids releva a Magic Knight?". |
| B11 | Importador ante Hellsing (fin antes de inicio) y ante filas de fuente en vivo (RadioOnce, "Duración=6"). (#1) | **D.** El importador **nunca rechaza la hoja entera**: importa lo válido, lista fila por fila lo que no pasó y por qué, en palabras claras. Filas cuyo título coincide con una `live_source` se importan como bloque en vivo y "Duración" se ignora con aviso. |
| B12 | Al migrar, el Sheet no tiene relleno ni reglas de diferido: el 30% de aire muerto sigue igual. (#1, #9) | **D.** El asistente de instalación avisa cuando la biblioteca de relleno está vacía y ofrece crear uno por defecto (cartel de la estación + cama musical). Después de importar, Parrilla muestra las horas vacías con un botón "Llenar con diferido" de un clic que crea la regla. |

## C · Motor, vivo y manual

| # | Hallazgo | Decisión |
|---|---|---|
| C1 | Fuente en vivo: sin margen de gracia (10:00:45), sin reconexión a mitad de bloque, sin salida ordenada al final. (#1, #2, #5) | **D.** `live_source.gracia_s` (default 30): antes de eso, se sostiene el último cuadro del programa anterior o el cartel, sin alarma. Pasado el margen: relleno + alarma. Mientras dura el bloque reservado el motor sigue intentando; cuando la señal vuelve, regresa a vivo en el siguiente borde de clip de relleno (máx. 60 s) con fundido cruzado. Fin de bloque: fundido de audio de 2 s y corte al minuto planificado. La fuente en vivo pasa por el **mismo conformado** que un archivo (escala, pillarbox, loudness). |
| C2 | Detector de silencio/negro: falsos positivos en negro intencional. (#2) | **D.** Umbral 15 s configurable (default), y `media_asset.negro_intencional = true` lo desactiva durante ese clip. Un vivo nunca lo desactiva. |
| C3 | Manual: dos operadores a la vez. (#2) | **D.** Un solo tenedor de `manual_hold`. El segundo ve "Rolando tiene el control desde 3:12 PM" y puede quitárselo con un botón explícito; queda en `audit_log`. |
| C4 | Manual: soltar a mitad de un spot pagado. (#2) | **D.** "Soltar" espera a que termine el clip en curso (máx. 60 s). "Parar todo" corta en seco y marca `plan_item.parcial = true`. |
| C5 | `manual_hold.motivo_fin` no tiene "se cayó el sistema". (#2) | **D.** Se añade `caida_del_sistema`. |
| C6 | Cascada extendida (3 h el domingo, 5 h cada madrugada) sin identificación de estación. (#1) | **D.** La cascada termina en el **cartel**, no en barras: el cartel lleva identificativo y comunidad de licencia, y lo genera el asistente en el paso 1. Barras y tono existen solo para la prueba del asistente. El cartel cumple el ID; el resolver además registra un `incidente.tipo=cascada_extendida` si pasa de 15 min. |
| C7 | Encoder de hardware colgado en runtime; ffmpeg colgado sin morir: ¿mata o desvía? (#2) | **D.** Watchdog: sin cuadro consumido por el encoder en 3 s → cartel al aire, mata el encoder, relanza con el **mismo** acelerador; si falla dos veces en 10 min, relanza en software y avisa "tu tarjeta de video dejó de responder". |
| C8 | Archivo borrado/0 bytes mientras suena; NAS reinicia. (#2) | **D.** Cualquier error de lectura a mitad de clip = fallo de clip → cascada. Cuarentena solo tras **dos** fallos separados por más de 5 min (no por un blip de red). §14 dice: la biblioteca puede estar en NAS, la base no. |
| C9 | Salida UDP sin nadie escuchando (no hay error). (#2) | **D.** Punto ciego estructural, dicho explícito: la verificación es local (A1). |
| C10 | Cable de red desconectado sin alarma propia. (#2) | **D.** Alarma `enlace_caido` distinta de `encoder_no_responde`, leyendo el estado de la interfaz. |
| C11 | Windows Defender pone `ffmpeg.exe` en cuarentena. (#2) | **D.** El watchdog distingue "ffmpeg desapareció del disco" y lo dice: "tu antivirus bloqueó ffmpeg — vuelve a aplicar las exclusiones en Ajustes". |
| C12 | Números que ACEPTACION no pudo verificar. (#5) | **D.** Defaults, todos configurables: "terminó de copiarse" = tamaño estable 10 s y se abre sin bloqueo · encoder no responde = 3 s · escalera de manual: ámbar a 60 s del regreso, rojo a 10 s · profanity delay 7 s · backoff de reconexión 1, 2, 4… 60 s tope · retención de grabación 7 días (30 si el disco alcanza) · silencio/negro 15 s. |
| C13 | Relleno que "excede hasta 5 s recortando con fundido": ¿qué se recorta, rompe subtítulos? (#8) | **D.** Se recorta el **último** clip de relleno del hueco; el fundido es de 1 s; el relleno no lleva subtítulos por definición. |
| C14 | Crawl "en caliente" sobre ffmpeg no es una operación documentada. (#8) | **D.** El crawl se renderiza en el **servidor de cuadros** (Go), no como filtro de ffmpeg. El encoder nunca se reinicia por un cambio de texto. |

## D · Infraestructura y reloj

| # | Hallazgo | Decisión |
|---|---|---|
| D1 | Disco de grabación lleno; disco de BD lleno. (#2) | **D.** Umbrales: <10 % → alarma; <5 % → la grabación se purga hasta liberar; <2 % → la BD deja de escribir as-run (el aire sigue desde el plan en memoria) y alarma roja. Nunca el disco lleno tumba el aire. |
| D2 | SQLite corrupto sin ruta de restauración. (#2) | **D.** `PRAGMA integrity_check` al arrancar; si falla, restaura el respaldo más reciente solo, arranca, y avisa. El plan de las próximas 48 h vive también en memoria. |
| D3 | Salto de reloj (adelante o atrás), no solo deriva. (#2) | **D.** El motor usa reloj monotónico para el aire y compara contra el de pared. Salto >60 s: no se re-emite nada ya `aired`, no se marca nada como vencido de golpe, alarma `salto_de_reloj` distinta de la deriva, y el resolver recalcula desde el instante real. |
| D4 | Apagón de 6 h: nada avisa fuera de banda; registro para el expediente. (#2) | **D.** Al volver, el sistema manda por el canal de avisos "estuve fuera de 6 h 12 min" y crea `incidente.tipo=apagon`. |
| D5 | Tailscale caído: aire sin afectación, decirlo. (#2) | **D.** Una línea en §19. |
| D6 | Sin internet: cap-poll, NTP, XMLTV, WhatsApp, portal, respaldo. (#4) | **D.** Todo lo de internet degrada con aviso claro, nunca con error. NTP: servidor de la LAN como alternativa; sin sincronía 24 h → aviso; 7 días → alarma. Canal de avisos = driver `notify` (Telegram gratis; WhatsApp vía proveedor). El portal necesita que la estación sea alcanzable: Tailscale Funnel o Cloudflare Tunnel, documentado en el asistente. Respaldo a segundo disco / USB / carpeta de red; nube después. |

## E · Publicidad, portal y FCC

| # | Hallazgo | Decisión |
|---|---|---|
| E1 | Portal: archivo de 4 GB; contenido inapropiado; obscenidad en clasificado; enlace filtrado en redes; webhook duplicado; pago sin archivo. (#2, #3) | **D.** Límite 500 MB / 5 min por spot con mensaje claro. **Cola de aprobación humana** antes del primer aire de cualquier compra (spot o clasificado). `portal_link`: token de 32 bytes, vence en 30 días, revocable, ligado a un anunciante; filtrado no permite subir por otro. `payment.clave_idempotencia = referencia_externa` única. `insertion_order.estado` gana `pagado_sin_material` con recordatorio a las 24 h y 72 h. |
| E2 | Corte sobrevendido igual. (#2) | **D.** Desempate por `insertion_order.prioridad`, luego por fecha de compra; el que cae se registra y va a make-good propuesto. Nunca silencio. |
| E3 | Alerta 30 s tarde: tolerancia. (#2) | **D.** ±15 s. Un spot tapado en más del 50 % es `preempted`; menos, `parcial`. |
| E4 | **Anuncios políticos** (73.1942/73.1943): sin campos. (#7) | **D.** `advertiser.tipo = comercial\|politico\|no_lucrativo`. Político exige candidato/cargo/elección y genera el **archivo político** exportable (solicitud, aceptación/rechazo, tarifas, horarios) con retención de 2 años. La tarifa mínima es un **aviso**, no un cálculo. Aplica solo si la licencia es Class A/full power (**R**, pregunta 7). |
| E5 | **Identificación de patrocinio en el crawl** (73.1212). (#7) | **D.** Todo clasificado lleva prefijo automático "Anuncio pagado por ‹nombre›"; no se puede quitar en perfil `us-fcc`. |
| E6 | **Registro de uso del medidor de loudness** (73.682(e)). (#7) | **D.** `audit_log.tipo=loudness` diario y automático: "medidor activo, N archivos normalizados, 0 fallos". Exportable. |
| E7 | EAS: retención 2 años; ventanas RWT/RMT. (#7) | **D.** `alert_event` nunca se purga antes de 24 meses. `alert_event.tipo=prueba_semanal` se valida contra la ventana configurada; fuera → aviso. |
| E8 | Subtítulos: exención; OPIF condicional; Part 17 fuera de alcance. (#7) | **D.** Ajuste "obligación de subtítulos: sí / exento (con motivo)" — **R**. OPIF solo si Class A. COMPLIANCE.md dice que luces de torre (Part 17) no son cosa de Antena787. |
| E9 | Dueño escéptico. (#9) | **D.** Cuarentena: botón "Dejarlo pasar bajo mi responsabilidad" (queda en `audit_log`). Make-good para **cualquier** incidente que tape un spot, no solo EAS. Reporte de ingresos por anunciante/mes (vendido vs emitido vs cobrado). `media_asset.sin_logo` para spots que lo piden. Entrada rápida de vivo siempre abierta (puerto SRT fijo, aparece en Al Aire sin configurar). `schedule_rule.tipo=bloque_arrendado` con anunciante y cobro. Factura PDF simple desde `payment`. `overlay.fecha_fin` (logo de temporada vence solo). |

## F · Seguridad (#3)

**D.** Todo se aplica. PIN de estación en el paso 1 del asistente (no es login con roles — es una sola clave, se pide una vez por navegador, cookie `SameSite=Strict`); la interfaz escucha solo en loopback y Tailscale por defecto; el portal es el único punto público. `ffprobe`/`ffmpeg` sobre lo subido: `-protocol_whitelist file`, sin red, con límite de tiempo. Clasificados: lista blanca de caracteres. MCP por stdio con clave. SRT con contraseña obligatoria. `audit_log` con cadena de hash (`hash_prev`). Credenciales con DPAPI (Windows) / keyring (Linux). Respaldos cifrados si salen de la máquina. Nota de inyección de prompt en §20. ffmpeg verificado por SHA-256 al instalar y al arrancar. Roles antes de F4 (cuando hay segunda persona, y el portal es esa "segunda persona" en cierto modo). Límite de peticiones en el portal. Actualizaciones con firma verificada.

## G · Pantallas (#6)

**Contradicciones a corregir ya en el diseño:** EnVivo 10 s → 15 s · Biblioteca "importados de Plex" → "importados de la carpeta" · Biblioteca "derechos hasta ene 2027" → "regla hasta ene 2027" · Ajustes "Windows IoT Enterprise LTSC" → "Windows 10 Pro" · Ajustes "−24 LKFS" → "volumen de televisión de EE. UU." · menú de 6 entradas en las 13 pantallas (CAtv ya tiene anunciantes) · Mes: 1 de sept de 2026 es martes · Ferretería del Este en un solo estado (campaña activa, 14 de 20) en Anunciantes y Portal · "vence Magic Knight" una sola vez, día 7; Reglas y Guía coinciden en que Zoids empieza el 8 · "890 GB" no puede ser libres y usados a la vez.

**Faltantes a añadir ya:** botón **Tomar el control** siempre visible en Al Aire · en Ajustes: deriva NTP, resultado de aceleración por hardware, umbral de silencio.

**Pantallas por diseñar** (se listan en §13 como pendientes): asistente de instalación con prueba de barras · cuarentena con motivo y "dejarlo pasar" · bitácora de incidentes · revisión de cambios del MCP · configurar salida · aprobación de make-good · clasificados (crear, previsualizar, aprobar) · reloj de cortes de fuente en vivo · aprobación de envío de reporte · configuración del logo.

## H · Modelo de datos — resumen de cambios (§15)

`schedule_rule`: `repite_a` (reemplaza `es_segundo_pase`), `tipo` gana `bloque_arrendado` · `plan_item`: restricción de no-solape por deck+output · `media_asset`: `negro_intencional`, `sin_logo` · `live_source`: `gracia_s` · `manual_hold.motivo_fin`: + `caida_del_sistema` · `incidente.tipo`: + `solape`, `cascada_extendida`, `apagon`, `salto_de_reloj`, `enlace_caido` · `alert_event`: `origen`, retención 24 meses · `advertiser`: `tipo` y campos políticos · `classified`: `estado` (pendiente\|aprobado\|rechazado), línea de patrocinio · `insertion_order.estado`: + `pagado_sin_material`; `prioridad` · `payment.clave_idempotencia` · `portal_link`: `vence`, `revocado` · `overlay.fecha_fin` · `audit_log`: `hash_prev`, `tipo` gana `loudness` · nueva `capture_input` (retorno de aire para A1) · `settings`: PIN (hash), umbrales.

## I · Preguntas nuevas para Rolando (§25)

- Clase de licencia (LPTV / Class A / full power) e ingresos — decide archivo político, OPIF, CALM y exención de subtítulos.
- ¿Hay retorno de aire? ¿Puede la máquina ver la señal **después** del ENDEC (captura, receptor, stream del transmisor)?
- ¿Cuánto disco tiene? (sigue abierta)

## J · Respuestas del 8 de septiembre (Saul, con fotos del rack de CAtv) — aplicadas

| Pregunta | Respuesta | Dónde quedó |
|---|---|---|
| RadioOnce Live! | Programa de radio por IP, sale por TV de vez en cuando → fuente en vivo solo audio + cartel del programa. Reloj de cortes sigue abierto. | §9 paso 5, §25 |
| Día de emisión | Cualquier hora, 6 AM solo default | §8 |
| Licencia | Cualquiera; el perfil `us-fcc` pregunta la clase y enciende lo que toca. CAtv (~300 W UHF) es LPTV o Class A, falta cuál. | §12, §25 |
| SCTE-104 | El encoder lo acepta; la señalización se elige de una lista (`scte104-tcp`, `gpi-out`, `hls-daterange`, `ninguna`) | §10 |
| ENDEC | GPI y red, los dos drivers. Marca/modelo pendiente. | §10, §25 |
| Resolución / fps | La persona elige de la lista (480i–1080p, 29.97/59.94/25/50); 1080i entrelazado real | §13 paso 6 |
| Subtítulos | Se conservan y **se suben** (.scc directo; .srt/.vtt → 608 en Go propio) | §9 paso 1, §15 `media_asset.subtitulos_externos` |
| Contenido | Servidor con respaldo a la web → NAS permitido, driver `nube` | §10 Respaldo |
| Retorno de aire | Sí: tarjeta receptora de TV + monitoreo por streaming → `capture_input` `receptor-tv`; el monitor por streaming es parte del producto y se ve en Al aire | §10, §13, ADR 0009 |
| Transmisor (fotos) | Excitador RVR Blue Digital Video multiestándar, 605 MHz, RF MONITOR + USB; amplificador ADR ~300 W con FWD/RFL/corriente/voltaje/temperatura → driver de **telemetría** `snmp`/`http`/`serial-usb`, alarma por FWD en cero, RFL alta, temperatura | §10 Transmisor, §13 Al aire |
| ENDEC (foto) | **Sage Digital ENDEC** (familia 1822 por la carátula; el 3644 tendría Ethernet). Serial RS-232 al frente + relés atrás. Monitorea WCMN 1280 AM, WKAQ-FM 104.7, NOAA ch 1 vía TFT EAS 930A. → `sage-endec` serial y `gpi-serial` primero. Falta 1822 vs 3644. | §10, §25 |
| Orban Optimod-FM 2200 (foto) | Procesador de radio FM en el rack → ¿emisora FM en el sitio? Si sí, F4b sube. | §25 pregunta 9, PARA-ROLANDO Q12 |
| Equipo 1U con BNC + compuesto (foto) | Probablemente el encoder; etiqueta ilegible. Pedir foto de frente y trasera. | PARA-ROLANDO Q5 |

## K · Supuestos de diseño (Saul, 8 sept, después de las fotos) — mandan sobre J

El software no le resuelve a Rolando: le resuelve a cualquiera, con cualquier equipo, en cualquier parte. CAtv es donde se prueba primero. Fijado en §2 y §25:
LPTV **y** Class A · reloj de cortes por fuente de radio · marcas y modelos varían, y las más usadas en EEUU traen driver (Sage 1822/3644, DASDEC, Gorman-Redlich, TFT; transmisores por SNMP/web/serial) · ENDEC por serial, relés **y** red · 29.97 **y** 59.94 · 1 a 10 TB, retención calculada · telemetría por USB, red o SNMP · un canal de TV **o** una radio que también sale online o por TV (§6: tres salidas, un canal).

## L · Respuestas de Rolando, 8 de septiembre (hechos, no supuestos)

Cadena: Antena787 → UDP/RTP → **Technalogix TP1000** (2 ASI in/out, 2 Ethernet: manejo + stream) → ASI → **RVR** (605 MHz, ASI in, red de manejo sin IP de transporte) → **ADR** ~300 W. **Class A.** **720p59.94.** RadioOnce corta **cada 15 min** (se supone) y es programa de **WRBM 89.3 FM Océano Radio**, en el mismo sitio → F4b para CAtv. Sage con XLR in/out y **relés en bloque verde**; modelo lo confirma el ingeniero. Disco **3.5 TB** (SSD 500 GB + HDD 3 TB) → sistema/base al SSD, biblioteca/grabación al HDD. Retorno de aire = tarjeta receptora. Pendiente: modelo del Sage, camino de telemetría, obligación de subtítulos. Aplicado en §12, §17, §19, §22, §25 y PARA-ROLANDO.

**8 sept, cierre:** las tres pendientes (Sage 1822/3644, camino de telemetría, obligación de subtítulos) se resuelven trayendo todas las opciones: el asistente pregunta *por dónde está conectado* y prueba cada camino; subtítulos con ajuste de tres estados que no bloquea. Ninguna pregunta de CAtv queda abierta.

**8 sept, Rolando:** la PC está en la torre; MistServer recibe los streams, VLC los hace 720p MPEG-2 + audio MPEG por UDP al TP1000, que es **multiplexor** (junta y saca ASI). Antena787 reemplaza MistServer + VLC. Consecuencias aplicadas: `udp-ts` con CBR, PIDs fijos, PCR ≤ 40 ms, MPEG-2 + AC-3/MP2 (§10); aceleración no aplica al MPEG-2 (§18); F0 emite el formato exacto de CAtv y mide bitrate/PCR (§22.1).

**8 sept, Saul:** todos los formatos y salidas se construyen; la prioridad es `udp-ts` MPEG-2 CBR al multiplexor (CAtv), luego internet H.264 (SRT/RTMP/HLS), archivo, DVB/ISDB (F5), ATSC 3.0. Fijado en §10 y §12.
