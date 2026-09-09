# Verificación de F1 — 9 de septiembre de 2026

Los 57 criterios de `docs/ACEPTACION.md` (F1-01 a F1-57) recorridos uno por uno
contra lo construido en la rama `main` (a partir del commit `1c5c9bb`), con
pruebas automáticas nuevas donde no las había (`f1verif_*_test.go` en cada
paquete) y comprobación a mano donde el criterio es [MANUAL].

Máquina: Mac M4 · Go 1.26 · ffmpeg/ffprobe 9.0.1 (Homebrew).

## 1 · Veredicto inicial (antes de corregir)

| | |
|---|---|
| PASA | 39 |
| FALLA | 5 (F1-04, F1-12, F1-26, F1-46, F1-56) |
| PARCIAL | 10 (F1-09, F1-10, F1-22, F1-28, F1-39, F1-42, F1-44, F1-54, F1-55, F1-57) |
| MANUAL (pendiente de firma humana) | 3 (F1-32, F1-43, F1-53) |

## 2 · Tabla criterio por criterio (estado inicial)

| ID | Veredicto | Evidencia (archivo:línea / test) | Nota |
|---|---|---|---|
| **F1-01** | PASA | `internal/ingest/watcher.go:24` (`WatchStableFor = 10s`), `:132` (quietud), `:135` y `:213` (`openableForRead`) · `internal/ingest/pipeline_test.go:199` `TestWatcherEsperaAQueTermineLaCopia` · **nuevo** `internal/ingest/f1verif_ingest_test.go:38` `TestF1_01_ArchivoConBloqueoNoSeAnuncia` | Las dos mitades del criterio corregido están: 10 s de tamaño quieto **y** apertura sin bloqueo (lee el primer y el último byte). Un archivo bloqueado reinicia el reloj de quietud en vez de anunciarse. Los temporales (`.part`, `.crdownload`, …) se ignoran (`watcher.go:28`). Nada llega a cuarentena por lectura a medias: además el ingest reintenta una vez a los 5 min los errores pasajeros (`ingest.go:169`, `TestIngestReintentaAntesDeCuarentena`). |
| **F1-02** | PASA | `internal/ingest/probe.go:219-293` (`fill`) · `internal/ingest/ingest_test.go:74` `TestProbeMideLoQueDicelPRD` | Códec, resolución, fps en quebrado exacto (`30000/1001`) y `DurationMs` al milisegundo desde `stream.duration`, con respaldo por `nb_frames/fps` y por `format.duration`. La prueba comprueba explícitamente que la duración no está redondeada al segundo. Se persiste en `media_asset.duracion_medida_ms` (`internal/store/media.go:73`). |
| **F1-03** | PASA | `internal/ingest/normalize.go:110-145` (pasada 1 y 2), `:159-166` (verificación) · **nuevo** `f1verif_ingest_test.go:126` `TestF1_03_DeMenos18AMenos24EnDosPasadas` | Clip fabricado a −18.0 LUFS medidos; el normalizado sale a −24 ±1 LUFS con true peak ≤ −2 dBTP, y `LoudnessReport.Passes == 2` deja constancia de que fueron dos pasadas (medir → corregir con `measured_I/TP/LRA/thresh/offset:linear=true`). |
| **F1-04** | **FALLA** | `internal/ingest/normalize.go:149-156` (la nota del propio código: *"la reinserción garantizada es F2"*), `:191-195` y `:209-211` · `internal/ingest/probe.go:212,261` · **nuevo (falla a propósito)** `f1verif_ingest_test.go:222` `TestF1_04_CEA608SobreviveAlFormatoDeCasa` | No existe ningún paso de extracción de CEA-608 ni de reinserción en el mux. Ver "Defectos", D-1. |
| **F1-05** | PASA | `internal/ingest/blacksilence.go:22-23` (umbrales), `:134-148` (cabeza / cola / medio) · `internal/ingest/ingest_test.go:118` `TestNegroYSilencioEnCabeza` | El caso exacto del criterio: 0.8 s de negro+silencio en cabeza se recorta, 0.3 s en cola no. El recorte se aplica en la normalización (`NormalizeOptionsFor`, `pipeline.go:212`; `TestNormalizeRecortaLaCabeza`), y `negro_intencional` lo desactiva (`pipeline.go:222`). |
| **F1-06** | PASA | `internal/ingest/blacksilence.go:142-147` · `internal/ingest/pipeline.go:147` · `internal/store/media.go:84` (`marcas_de_corte_ms`) · `ingest_test.go:144` `TestNegroEnMedioEsMarcaDeCorte` · **nuevo** `f1verif_ingest_test.go:264` `TestF1_06_MarcaDeCorteCandidataQuedaEnElAsset` | El ingest completo deja `HeadBlackMs = TailBlackMs = 0` y una marca candidata en el centro del tramo; `NormalizeOptionsFor` no la convierte en recorte. La confirmación por el programador está expuesta en `PUT /material/{id}` (`docs/API.md`, sección Biblioteca). |
| **F1-07** | PASA | `internal/ingest/normalize.go:171-220` (`normalizeArgs`), `:225-238` (`videoConform`) · `internal/ingest/queue.go:133-171` · `internal/app/media.go:125` y `:293-320` · **nuevo** `f1verif_ingest_test.go:304` `TestF1_07_FormatoDeCasaConGopCerrado` y `:382` `TestF1_07_SeNormalizaUnaSolaVez` | El normalizado sale en un códec (h264), una resolución (1280x720), una tasa (59.94) y un audio (2 ch / 48 kHz), con `-flags +cgop -g 60 -keyint_min 60 -sc_threshold 0`; ffprobe confirma un cuadro clave exactamente cada 60. La cola llama al trabajo **una sola vez** por archivo y lo deja en `listo`; `IngestFile` además no reingiere una ruta ya conocida. |
| **F1-08** | PASA | `internal/ingest/metadata.go:78-121` (local primero), `:150-178` (`FromTags`) · `internal/app/media.go:217-226` (`Providers` sin poblar = default de fábrica) · `pipeline_test.go:131` `TestMetadataNoTocaLaRedSinProveedores` · **nuevo** `f1verif_ingest_test.go:438` `TestF1_08_EtiquetasEmbebidasSinRed` | Con etiquetas embebidas de título y año, y sin `.nfo` ni carátula, el ingest arma la ficha con `tags-embebidas` y hace **cero** llamadas de red (comprobado con un `httptest.Server` centinela que cuenta peticiones). Matiz declarado: si alguien encendiera un driver, `metadata.go:119` sí saldría a la red a buscar la sinopsis y la carátula que las etiquetas no traen — es lo que pide PRD §10, pero es más de lo que la letra de F1-08 permite. Hoy no hay forma de encender un driver (ver F1-09). |
| **F1-09** | **PARCIAL** | `internal/ingest/providers.go:33-53` (orden) · `internal/ingest/metadata.go:132-144` · **nuevo** `f1verif_ingest_test.go:504` `TestF1_09_TVmazeAntesQueDriverConClave` (pasa) · `internal/app/media.go:217-226` (nunca se pasan `Providers`) | El orden es el correcto y está probado: `Providers()` devuelve `[TVmaze, CoverArtArchive, TMDB]` y `Metadata` para de consultar en cuanto TVmaze contesta, sin llegar al driver con clave. Lo que falta es el cableado: `ingest.Providers` no se llama en ningún sitio fuera del paquete (grep en `internal/` y `cmd/`), `app.ingestDeps` deja `Providers` en `nil` y no hay ajuste ni pantalla para encenderlo. En el producto que corre hoy, TVmaze **nunca** se consulta. Ver D-3. |
| **F1-10** | **PARCIAL** | `internal/ingest/pipeline.go:117-125` · `pipeline_test.go:295` `TestIngestArchivoRotoVaACuarentena` · **nuevo** `f1verif_ingest_test.go:579` `TestF1_10_ErrorDeLecturaVaACuarentena` (pasa) y **nuevo (falla a propósito)** `:613` `TestF1_10_SinAudioVaACuarentena` | La rama "ffprobe reporta error de lectura" se cumple entera: cuarentena, `motivo_en_cristiano` sin jerga, `Ready() == false` y el resolver no lo programa (`resolver.go:656`, `TestSoloMaterialListo`). La rama "sin audio detectable" **no**: el ingest solo manda a cuarentena si no hay *ni* imagen *ni* sonido. Ver D-2. |
| **F1-11** | **PASA** | `internal/store/schema.sql:164` `CHECK (fecha_fin >= fecha_inicio)` · `internal/store/f1verif_esquema_test.go:30` `TestF1Verif11FechasAlRevesLasRechazaElEsquema` (SQL crudo, INSERT y UPDATE) · `internal/store/store_test.go:109` `TestReglaConFechasCruzadasFalla` | Verificado a nivel de base: el rechazo llega como `CHECK constraint failed`, no de código Go. El resolver además se cubre las espaldas (`internal/resolver/resolver.go:332` `ruleProblem`, probado en `TestHellsingNoSeProgramaJamas`). Se comprobó también el borde `fecha_fin == fecha_inicio` (un solo día): entra. |
| **F1-12** | **FALLA** | `internal/store/f1verif_esquema_test.go:67` `TestF1Verif12PlanItemFueraDelRangoDeSuRegla` (falla) · `internal/store/schema.sql:175-205` (tabla `plan_item`, sin CHECK ni trigger que ate `dia_emision` a la vigencia de `schedule_rule_id`) | El esquema acepta un `plan_item` con `dia_emision = 2026-12-21` apuntando a una regla `2026-08-09 … 2026-12-20`, y también uno anterior a `fecha_inicio`. Hoy nadie lo produce (el resolver solo instancia días que pasan `ScheduleRule.Covers`, `internal/model/model.go:285`), pero la garantía que pide el criterio no existe. Ver defecto **D1**. |
| **F1-13** | **PASA** | `internal/resolver/f1verif_reglas_test.go:51` `TestF1Verif13ElContadorAvanzaYRecuerda` · `internal/resolver/resolver.go:610` `pickEpisodes` | Con `episodios_por_corrida = 1` y `ultimo_episodio_emitido = T2E5`, la corrida siguiente programa **T2E6** (no repite el 5), y `EpisodeAdvance` queda en T2E6. Se comprobó también el día siguiente (T2E7). |
| **F1-14** | **PASA** | `internal/resolver/f1verif_reglas_test.go:88` `TestF1Verif14DiezEpisodiosCuandoCaben` | Con espacio suficiente (10 × 24 min en la franja de 18:00 a 23:00) salen **10 `plan_item` consecutivos**, un episodio distinto cada uno, pegados sin huecos, sin aviso de sobrecupo. El caso "si no caben, manda el reloj" está en F1-38. |
| **F1-15** | **PASA** | `internal/resolver/resolver_test.go:80` `TestRelevoNoEsConflicto` · `internal/resolver/resolver.go:367` `resolveClashes` | Cubre los tres casos: relevo consecutivo, relevo solapado (gana quien releva, sin aviso) y solape sin relevo (sí avisa). |
| **F1-16** | **PASA** | `internal/resolver/resolver_test.go:170` `TestSegundoPaseMismoEpisodio` · `internal/resolver/resolver.go:496-560` `placeProgram` | `repite_a` emite el mismo episodio que puso la primaria ese día de emisión, ambos `plan_item` existen y `EpisodeAdvance` solo trae la regla primaria. `es_segundo_pase` no existe en el esquema. **Observación** (no cambia el veredicto, es conducta de F2): `App.MarkAired` (`internal/app/resolve.go:370`) cae al `EpisodeID` del ítem cuando la regla no está en `EpisodeAdvance`, así que al aire escribiría `ultimo_episodio_emitido` **también en la regla de repetición**. Hoy es inofensivo —el resolver lee siempre el contador del dueño (`owner = primary.ID`)— pero deja un contador propio guardado que se volvería la verdad si alguien quita el `repite_a`. Ver defecto **D5** (menor). |
| **F1-17** | **PASA** | `internal/resolver/f1verif_reglas_test.go:133` `TestF1Verif17VentanaDeCuarentaYOchoHoras` · `internal/resolver/resolver.go:49` `DefaultHorizon = 48h` | Corriendo el lunes a las 10:00 AM con la parrilla real de CAtv: la ventana queda **cubierta segundo a segundo** hasta el miércoles a las 10:00 AM y **ningún** ítem arranca en ese instante o después (ni antes del arranque). |
| **F1-18** | **PASA** | `internal/resolver/f1verif_reglas_test.go:158` `TestF1Verif18ReglaVencidaQuedaFueraSinRuido` · `internal/resolver/resolver_test.go:14` `TestFechaFinInclusiva` | La regla con `fecha_fin` = hoy no pone nada al correr a las 00:05 del calendario siguiente, y **no** genera aviso de `regla_invalida` ni de `sin_material`: queda fuera del plan sin ruido. La regla vigente de al lado sí pone lo suyo. |
| **F1-19** | **PASA** | `internal/resolver/f1verif_reglas_test.go:190` `TestF1Verif19DosReglasEnLaMismaFranjaEsConflicto` · `internal/resolver/resolver_test.go:80` | Dos reglas sin relevo a la misma hora: sale el aviso `regla_invalida` ("ninguna releva a la otra") y **solo una** de las dos genera `plan_item`. |
| **F1-20** | **PASA** | `internal/resolver/resolver_test.go:526` `TestSoloMaterialListo` · `internal/resolver/resolver.go:648` `readyAsset` | Un `media_asset` en `cuarentena` no entra al plan y sale el aviso `sin_material` ("no tiene archivo listo para aire"). El hueco cae a relleno. |
| **F1-21** | **PASA** | `internal/resolver/f1verif_reglas_test.go:216` `TestF1Verif21ElRellenoCuadraExactoEnElPlan` · `internal/resolver/resolver_test.go:469` `TestRellenoEmpaqueta` · `internal/resolver/fill.go:22` `packFillers` | Hueco de 7:00 con biblioteca 2:00/3:00/4:00 → el plan real tiene **3:00 + 4:00 en ese orden** y el hueco residual es 0 (el relleno pega exacto con la regla de las 8:30). Sin cartel. |
| **F1-22** | **PARCIAL** | `internal/resolver/f1verif_reglas_test.go:316` `TestF1Verif22ExcesoDeTresSegundosSeRecortaEnElUltimoClip` (pasa) · `internal/resolver/resolver.go:54` `FillerFadeMs` (declarada y **nunca usada**) | Lo verificado: hueco de 5:57 con clips de 2:00/3:00 → 3:00 + 3:00 con el **último** clip recortado a 2:57 (exceso de 3 s ≤ 5 s), hueco residual 0, sin cartel; y si el exceso mínimo pasa de 5 s (solo clips de 4:00) la combinación **se descarta** y lo que sobra va al cartel. Lo que falta: el **fundido de 1 segundo**. Ver defecto **D3**. |
| **F1-23** | **PASA** | `internal/resolver/f1verif_reglas_test.go:257` `TestF1Verif23LosTresUmbralesDeVencimiento` · `internal/resolver/resolver_test.go:560` `TestAvisosDeVencimiento` · `internal/resolver/resolver.go:859` `expiryWarnings` · `internal/app/resolve.go:227` `expiryNotices` | Los tres umbrales (30, 14, 7) salen en orden a medida que la regla se acerca a su `fecha_fin`, uno por umbral, y correr el resolver otra vez el mismo día con `ultimo_aviso_enviado` ya guardado **no lo repite**. La persistencia del umbral la hace `App.expiryNotices`. |
| **F1-24** | **PASA** | `internal/resolver/f1verif_reglas_test.go:369` `TestF1Verif24LasTrescientasTreintaYSeisMediasHoras` · `internal/resolver/resolver_test.go:659` `TestSemanaDeCAtvSinHuecos` | Con la parrilla real de CAtv (34 reglas) y horizonte de 7 días: las **336 medias-horas** de la semana tienen `plan_item` o relleno, y `covered()` confirma que no queda un solo segundo de aire vacío. |
| **F1-25** | **PASA** | `internal/resolver/f1verif_reglas_test.go:397` `TestF1Verif25y34LaMadrugadaEsDelDiaAnterior` · `internal/model/model.go:114` `Channel.BroadcastDay` | Con `hora_inicio_dia_emision = 6:00 AM`, el ítem de las **12:30 AM del martes** de calendario queda con `dia_emision = lunes`, `hora_local = 00:30`; y a las 6:00 AM ya cambia de día. |
| **F1-26** [MANUAL] | **FALLA** | `internal/app/f1verif_guia_test.go:229` `TestF1Verif26LaEdicionAManoSobreviveAlResolver` (rojo) · `internal/app/resolve.go:96` · `internal/store/plan.go:163` `DeleteFuturePlanned` · `internal/model/model.go:327-348` · `web/src/pantallas/ParrillaSemana.tsx:387-424` | No existe ruta de API para editar un `plan_item`, ni marca de «tocado a mano» en el modelo. `Resolve` borra **todo** lo `planned` futuro y lo vuelve a crear: la edición desaparece. El panel «¿Solo hoy, o siempre?» de la parrilla tiene los dos botones cableados a `setArrastre(null)` — no llaman a nada. |
| **F1-27** [AUTO] | **PASA** | `internal/resolver/xmltv_test.go:21` `TestXMLTVDeLaParrillaReal` · `internal/resolver/xmltv.go:57` `ChannelID` · corrida real del binario | Con el canal instalado como «CAtv» el XMLTV sale con `<channel id="catv.antena787">` y `<display-name>CAtv</display-name>`. Nada de `sports1.channel`. |
| **F1-28** [AUTO] | **PARCIAL** | Lógica: `internal/resolver/xmltv.go:213` `ValidateXMLTV` + `internal/resolver/xmltv_test.go:60` `TestValidadorDeLaGuia` (verde, caso «termina antes de empezar»). Cableado: `internal/app/f1verif_guia_test.go:290` `TestF1Verif28ElValidadorCorreAntesDePublicar` (rojo) | El validador existe y caza el caso *Hellsing*, pero **nadie lo llama fuera de las pruebas**: `rebuildGuide` (`internal/app/resolve.go:255`) escribe el archivo y llena `/guia.xml` sin validar. La publicación no se puede rechazar porque no hay puerta. |
| **F1-29** [AUTO] | **PASA** | `internal/resolver/f1verif_guia_test.go:13` `TestF1Verif29GuiaDelDiaMuestraElRelevo` | El bloque de las 8:00 del día del relevo dice «Zoids» y no «Magic Knight Rayearth»; la guía pasa el validador. |
| **F1-30** [AUTO] | **PASA** | `internal/resolver/f1verif_guia_test.go:65` `TestF1Verif30SombraNoEscribeAsRun` | Los 300+ ítems de la semana de CAtv salen con `instante_real`, `duracion_real` y `cued_en` en nil y `estado=planned`; `instante_planeado`/`duracion_planeada` poblados. |
| **F1-31** [AUTO] | **PASA** | `internal/app/f1verif_guia_test.go:181` `TestF1Verif31SombraNoTocaElAire` · `cmd/antena/main.go` (no importa `internal/engine`) · `internal/app/app.go:248` (solo `engine.FFmpeg()`/`FFprobe()`, que son búsquedas de ruta) | El único consumidor del motor de F0 es `cmd/f0`. El binario `antena` no arranca encoder ni salida; ninguna `output` queda en `conectada`. |
| **F1-32** [MANUAL] | **MANUAL** (superficie parcial) | `internal/api/plan.go:126` `GET /plan?dia=` · `internal/api/plan.go:455` `GET /guia?dia=` · `web/src/pantallas/ParrillaGuia.tsx` | **Cómo comprobarlo:** entrar, abrir Parrilla · Día del bloque en cuestión (o `curl -b cookie 'http://127.0.0.1:7870/api/v1/plan?dia=AAAA-MM-DD'`), imprimirlo y compararlo a mano con la libreta de Rolando. **Lo que falta:** no hay dónde registrar dentro del sistema lo que se emitió con VLC (`instante_real` solo lo escribiría el motor de F2, ver F1-30), así que la comparación es en papel. Además `web/src/pantallas/ParrillaGuia.tsx:14` tiene el día **fijado a `'2026-09-08'`** y espera `{filas: [...]}` mientras la API devuelve un arreglo pelado (`internal/api/plan.go:521`) — esa pantalla no sirve para la comparación tal como está. |
| **F1-33** | **PASA** | `internal/resolver/resolver_test.go:47` `TestMadrugadaPorDiaDeEmision` · `internal/resolver/resolver_test.go:433` `TestReglaDeLasCincoCincuentaYNueve` · `internal/resolver/resolver.go:282` `instantOf` | La regla que termina el lunes de emisión **sigue vigente** a las 12:00 AM del martes de calendario (Magic Knight 313, `fecha_fin = 2026-09-07`, emite el 2026-09-08 a las 00:00); y la de las 5:59 AM del martes de calendario cuenta para el lunes. `fecha_fin` es inclusiva hasta el cierre del día de emisión. |
| **F1-34** | **PASA** | `internal/resolver/f1verif_reglas_test.go:397` (segunda mitad) · `internal/resolver/resolver_test.go:47` | Patrón `L______` a las 2:00 AM → `plan_item` el **martes de calendario** a las 2:00 con `dia_emision = lunes`, y nada en el martes de emisión. El patrón se lee por día de emisión, no por calendario. |
| **F1-35** | **PASA** | `internal/resolver/f1verif_reglas_test.go:450` `TestF1Verif35NoHayRecurrenciaAnualNiCasoDeBisiesto` | Ni el tipo `model.ScheduleRule` ni la tabla `schedule_rule` del esquema tienen campo de recurrencia anual ni manejo del 29 de febrero (busca *anual/annual/yearly/aniversario/bisiesto/leap/02-29/febrero*). El patrón es de 7 letras. Comprobación de conducta añadida: el **29 de febrero de 2028** sale como un día de emisión normal. |
| **F1-36** | **PASA** | `internal/resolver/resolver_test.go:231` `TestSegundoPaseSinPrimaria` · `internal/resolver/resolver.go:503-520` | Con la primaria vencida, la repetición toma el **siguiente** episodio y `EpisodeAdvance` avanza el contador **compartido** (bajo el id de la primaria). |
| **F1-37** | **PASA** | `internal/resolver/resolver_test.go:191-229` (bucle de 20 días dentro de `TestSegundoPaseMismoEpisodio`) | Veinte días de emisión seguidos con primaria a las 2 PM y repetición a las 11 PM: siempre el mismo `episode_id`, un solo contador (`EpisodeAdvance[344]`), nunca se desincronizan. |
| **F1-38** | **PASA** | `internal/resolver/resolver_test.go:127` `TestSobrecupoDeDiezCabenNueve` · `internal/resolver/resolver.go:530-545` | Diez episodios de 33 min en cinco horas: salen **9**, el décimo **no arranca**, el resto (22:57→23:00) va a relleno, sale el aviso "de 10 episodios caben 9" y el contador avanza solo por los nueve. Ningún episodio cortado. |
| **F1-39** | **PARCIAL** | Pasa: `internal/resolver/f1verif_reglas_test.go:502` `TestF1Verif39ElFinDelVivoEsUnInicioDuro` · `internal/resolver/resolver_test.go:252` `TestVivoFinDuro`. **Falla:** `internal/resolver/f1verif_reglas_test.go:544` `TestF1Verif39ReglaDentroDeLaVentanaDelVivo` | El fin del bloque en vivo es duro **hacia afuera** (nada arranca antes si no termina, y a las 13:00 entra puntual la regla siguiente). Pero (a) en F1 el resolver **nunca materializa un ítem `dentro_de`**, así que el enunciado literal del criterio no se puede ejercitar todavía, y (b) una regla que arranca **dentro** de la ventana del vivo lo recorta y se desborda del fin. Ver defecto **D4**. |
| **F1-40** | **PASA** | `internal/resolver/resolver_test.go:448` `TestProgramaQueCruzaElIniciodelDia` · `internal/resolver/resolver.go:414-421` (`resolveHardEnds`: sin `duracion_slot_ms` la regla manda hasta la siguiente, así que cruza el borde del día) | Un bloque de tres horas que arranca a las 5:00 AM del martes de calendario dura las tres horas completas, pertenece **entero** al día de emisión del lunes y termina a las 8:00 AM. |
| **F1-41** | PASA | `internal/resolver/resolver.go:58-60` (`normalizeReady`), `:656` (`readyAsset`), `:842` (relleno), `:532` (aviso) · `internal/resolver/resolver_test.go:526` `TestSoloMaterialListo` · `internal/model/model.go:426` (`Ready()`) · `internal/api/reglas.go:244-246` | Solo entra al plan material con `estado = listo` **y** `estado_normalizacion = listo`; lo demás produce el mismo aviso `WarnNoMaterial` que un archivo en cuarentena. La prueba existente cubre las dos variantes (cuarentena y normalización a medias). La API además impide guardar una regla contra un título sin material listo, con la frase en cristiano. |
| **F1-42** | **PARCIAL** | `internal/ingest/queue.go:191-201` (`jobHeap.Less`) · `internal/ingest/pipeline_test.go:404` `TestColaPriorizaPorHoraDeAire` (pasa) · `internal/app/media.go:147`, `:192-204` (`airsAt`) y `:286` · **nuevo (falla a propósito)** `internal/app/f1verif_cola_test.go:75` `TestF1_42_LaColaSabeCuandoSaleAlAire` | La cola en sí ordena exactamente como pide el criterio (8:00 → 15:00 → mañana → sin fecha, y a igualdad por llegada). Lo que no funciona es de dónde sale esa hora: `App.airsAt` la busca en `plan_item`, y el resolver solo pone en el plan material ya normalizado (F1-41), así que un archivo recién ingerido —el único que está en la cola— nunca tiene hora de aire y la prioridad degenera en orden de llegada. Ver D-4. |
| **F1-43** | **MANUAL** — superficie parcial | `internal/api/biblioteca.go:22-27` (`EstadoNoListo = "aún no listo para aire"`), `:51-86` (`titleOut`) · `internal/model/model.go:426` · `web/src/lib/tipos.ts:189,201,213` · `web/src/pantallas/Biblioteca.tsx:309-325,347-405` · `docs/API.md` (sección Biblioteca) | Cómo comprobarlo y qué existe: ver §3. La API sí distingue los tres estados; la pantalla de Biblioteca los pinta, pero lee un campo con **otro nombre** que el que la API emite, así que contra el servidor real la etiqueta no aparece. "Anuncios" y la compra del portal no existen en F1 (son F4): esa mitad del criterio es **N/A en F1**. Ver D-5. |
| **F1-44** | **PARCIAL** | Pasa: `internal/store/schema.sql:206-232` (triggers `plan_item_sin_solape_insert` / `_update`) · `internal/store/f1verif_esquema_test.go:112` `TestF1Verif44NoSolapeEnElEsquema` (SQL crudo: solape interno, envolvente, de 1 ms, UPDATE que mueve encima; y los casos legítimos: pegado sin pisar, otro deck). **Falla:** `internal/store/f1verif_esquema_test.go:177` `TestF1Verif44RevivirUnDescartadoNoDebeSolapar` | El rechazo lo hace el esquema, no el código Go (el mensaje es `plan_item solapado en el mismo deck`, traducido a `ErrOverlap` en `internal/store/errors.go:52`). Dos huecos: el trigger de UPDATE no vigila la columna `estado` (ver **D2**) y `plan_item` **no tiene columna de salida**, así que "la misma salida" del criterio se cubre solo por `channel_id + deck_id` (con un solo canal y una salida en F1 es equivalente; deja de serlo cuando haya varias salidas). |
| **F1-45** | **PASA** | `internal/resolver/resolver_test.go:587-597` (bloque de relevo de `TestAvisosDeVencimiento`) · `internal/resolver/resolver.go:866` `if r.relieve[rule.ID] { continue }` | Una regla que vence en 7 días con otra que la releva (`releva_a`) no genera **ningún** aviso de vencimiento. El mapa `relieve` se arma en `newRun` (`resolver.go:237`) con todos los `HandsOffTo`. |
| **F1-46** [MANUAL] | **FALLA** | Canal de avisos: sin ninguna coincidencia de `telegram`/`whatsapp`/`smtp`/`correo` en `internal/`, `cmd/`, `web/src/` (solo la columna `advertiser.whatsapp` de `internal/store/schema.sql:258`). Al aire: `web/src/pantallas/AlAire.tsx:229` pinta solo `estado.alarmas`, y `internal/app/app.go:390` `Alarms()` + `internal/app/mantenimiento.go:97,121` solo llevan alarmas de **disco**. | De las tres pantallas: **Reglas** sí (`web/src/pantallas/Reglas.tsx:33-45,141-199`, semáforo por `dias_restantes`), **Parrilla · Mes** sí (`web/src/pantallas/ParrillaMes.tsx:143-146`, «vence X»), **Al aire no**. El aviso de vencimiento se guarda como incidente (`internal/app/resolve.go:236` `a.Incident("vencimiento", …)`) pero no llega a `alarmas`. Y no existe ningún canal de avisos (Telegram / WhatsApp / correo): ni ajuste, ni código, ni pantalla. La supresión por relevo y los umbrales 30/14/7 sí están (`internal/resolver/resolver.go:859-892`). |
| **F1-47** [AUTO] | **PASA** | `internal/app/f1verif_guia_test.go:64` `TestF1Verif47LaGuiaSaleEnLaMismaCorrida` · `internal/app/resolve.go:117` | `Resolve` llama a `rebuildGuide` dentro de la misma corrida; no hay temporizador propio de la guía. Antes de la primera resolución `Guide()` está vacío; después de una sola corrida, lleno. |
| **F1-48** [AUTO] | **PASA** | `internal/app/f1verif_guia_test.go:94` `TestF1Verif48LaGuiaSigueAlPlan` | Un cambio en el plan (regla nueva) aparece en la guía en la misma corrida; se comprobó además ítem por ítem que `GuideItems()` y `plan_item` coinciden en instante y duración. Todas las mutaciones de la API llaman a `Recalc()` (`internal/api/reglas.go:122,137,152`, `internal/api/estado.go:141,210`) y el debounce del bucle es de 2 s (`internal/app/app.go:78`), muy por debajo del minuto. |
| **F1-49** [AUTO] | **PASA** | `internal/api/f1verif_jerga_test.go:369` `TestF1Verif49GuiaXMLSiempreContesta` · `internal/app/f1verif_guia_test.go:149` `TestF1Verif49LaGuiaSeEscribeEnLaRuta` · corrida real | Con el binario levantado: `GET /guia.xml` sin cookie → `200 application/xml` con `Last-Modified`; tras `PUT /ajustes {"ruta_guia_xml": …}` + `POST /plan/recalcular` el archivo en disco es **idéntico** al servido (`diff -q` → IDENTICOS). Escritura atómica vía `.nuevo` + `rename` (`internal/app/resolve.go:288-296`). *El envío opcional por HTTP a un destino externo no está construido* — no hay nada que pueda fallar, pero tampoco hay nada que probar. |
| **F1-50** [AUTO] | **PASA** | `internal/importer/f1verif_hoja_test.go:13` `TestF1Verif50DoscientasFilasSieteMalas` | 200 filas, 7 sembradas malas → 193 reglas importadas, 7 `RowErrors` con línea y motivo en cristiano, ninguna con jerga de programador. La hoja nunca se rechaza entera (`internal/importer/rules.go:178`). |
| **F1-51** [AUTO] | **PASA** | `internal/importer/f1verif_hoja_test.go:80` `TestF1Verif51MadrugadaCorreLaFecha` · `internal/importer/rules.go:388` `shiftDates` · `internal/importer/importer_test.go:157` | Fila de 2:00 AM del martes 8/9/2026 → `desde=2026-09-07`, `hasta=2026-12-07`, patrón `_M_____`→`L______`, y un `DateShift` con la línea de la hoja y el texto explicando el corrimiento. La de las 8:00 AM no se toca. |
| **F1-52** [AUTO] | **PASA** | `internal/importer/f1verif_hoja_test.go:130` `TestF1Verif52FuenteEnVivoIgnoraEpisodios` · `internal/importer/rules.go:207-217` | «RadioOnce Live!» con `Duración = 6` → `Kind = vivo`, `EpisodesPerRun = 0`, y un `Notice` que dice el nombre, que es en vivo y que la columna decía 6. |
| **F1-53** [MANUAL] | **MANUAL** (superficie completa, lista para firmar) | `internal/importer/rules.go:491` (texto `"¿%s releva a %s a las %s?"`) · `internal/importer/importer_test.go:238` `TestRelevosInferidos` · `internal/api/importar.go:239,306` · `web/src/pantallas/Reglas.tsx:460-487` | **Cómo comprobarlo:** Reglas → «Pegar desde Excel o Google Sheets» → pegar la hoja de CAtv → el panel muestra «RELEVOS PROPUESTOS» con la frase «¿Zoids releva a Magic Knight Rayearth a las 3:00 PM?» y el botón «Confirmar los relevos», que hace `POST /importar/confirmar-relevos` y carga `releva_a`. Todo el camino existe y está probado en la parte Go. |
| **F1-54** [MANUAL] | **PARCIAL** | API: `internal/api/plan.go:340` `planDiferido` + `internal/api/api_test.go:494` `TestLlenarConDiferido` (verde). Interfaz: `web/src/pantallas/ParrillaSemana.tsx:363-384` | La regla de un clic **funciona en la API**. En la interfaz falla en tres cosas: (a) el botón se llama «Llenar el fin de semana», no «Llenar con diferido»; (b) solo aparece si el fin de semana tiene más de una hora vacía por encima del promedio de la semana (`promFin > promSemana + 1`), no colgado de cada tramo vacío; (c) las horas están fijas en el código (01:00–06:00 desde 07:00–12:00) y no salen del hueco que se está llenando. Además la parrilla semanal **no marca bien los tramos vacíos contra la API real**: ver defecto D-5. |
| **F1-55** [AUTO] | **PARCIAL** | Parrilla: `internal/resolver/f1verif_guia_test.go:99` `TestF1Verif55BloqueArrendadoEnLaParrilla` (verde). Ingresos: sin ninguna coincidencia de `ingreso`/`revenue`/`facturac` en `internal/`, `web/src/`, `docs/API.md`. | La regla `tipo = bloque_arrendado` con `advertiser_id` y `cobro` se materializa como cualquier otra (cae en el `default` de `internal/resolver/resolver.go:355`), queda en el plan atada a su regla y sale en la guía. **No existe reporte de ingresos** en ninguna parte — ni tabla en Go, ni ruta de API, ni pantalla. (La tabla de la desviación del §«Cobertura» coloca publicidad y cobro en F4; si la mitad de ingresos se difiere a propósito, hay que decirlo en `ACEPTACION.md`, porque el criterio como está escrito no se cumple.) |
| **F1-56** [AUTO] | **FALLA** | `internal/api/f1verif_jerga_test.go:44` `TestF1Verif56SinJergaEnLaInterfaz` (rojo). Hallazgo único: `internal/ingest/providers.go:164` | Interfaz (`web/src/**/*.{ts,tsx}`): **limpia** — las cuatro apariciones de `driver` son claves de objeto y campos de tipo (`web/src/demo/datos.ts:43,51,59`, `web/src/lib/tipos.ts:31`) que nunca se pintan; el bundle compilado (`internal/api/ui/dist/assets/index-GEKvXtjX.js`) las lleva solo como claves de los datos de demostración. Servidor: una cadena en cristiano con jerga (ver defecto D-3). Ajustes dice «volumen de televisión de EE. UU.» (`web/src/pantallas/Ajustes.tsx:14`) y Biblioteca «importados de la carpeta» (`web/src/pantallas/Biblioteca.tsx:81`) — las dos frases que el criterio nombra están bien. |
| **F1-57** [AUTO] | **PARCIAL** | Menú: `internal/api/f1verif_jerga_test.go:225` `TestF1Verif57MenuDeCincoYSeis` (verde) · `web/src/componentes/Armazon.tsx:12-19`. Servidor: `internal/api/f1verif_jerga_test.go:331` `TestF1Verif57ElServidorDiceSiHayAnunciantes` (rojo) | La estructura del menú es exacta: cinco entradas (Al aire, Parrilla, Reglas, Biblioteca, Ajustes) y una sexta, Anuncios, tras `soloConAnunciantes`. Ni multi-canal, ni roles, ni nombres de usuario en ninguna pantalla (barrido de `web/src` limpio). **Pero `GET /estado` nunca manda `hay_anunciantes`** (`internal/api/estado.go:15-28`), así que `Armazon.tsx:29` lo da por `false` para siempre y el menú **no puede llegar nunca a seis**. Encima no hay repositorio ni ruta de API de `advertiser` (la tabla existe en `internal/store/schema.sql:251` y nada más), así que tampoco hay forma de registrar el primer anunciante. |

## 3 · Defectos encontrados y qué se hizo con cada uno

Resumen de decisiones (detalle debajo):

| Defecto | Criterio | Decisión | Estado |
|---|---|---|---|
| Sin CEA-608 en el archivo normalizado | F1-04 | **diferido a F2** (tamaño L, ffprobe 9) | anotado en `ACEPTACION.md`, prueba saltada |
| Archivo sin audio pasaba como listo | F1-10 | corregido: va a cuarentena con motivo | hecho |
| Drivers de ficha sin cablear | F1-09 | corregido: ajuste `fichas_en_linea` + `clave_tmdb` | hecho |
| Cola de normalización sin hora de aire | F1-42 | corregido: la hora sale de las reglas | hecho |
| Biblioteca con nombres de campo distintos | F1-43 | corregido: la API emite el contrato de `tipos.ts` | hecho |
| `plan_item` fuera de la vigencia de su regla | F1-12 | corregido: triggers en migración 2 | hecho |
| Revivir un descartado podía solapar | F1-44 | corregido: el trigger vigila `estado` | hecho |
| Fundido de 1 s inexistente | F1-22 | F1 guarda `fundido_salida_ms`; F2 lo aplica | hecho + anotado |
| Regla dentro de un vivo lo recortaba | F1-39 | corregido: el bloque manda, la regla avisa | hecho; mitad `dentro_de` es F2 |
| Contador escrito en la regla de repetición | F1-16 | corregido en `MarkAired` (dueño del contador) | hecho |
| Edición a mano no sobrevivía al resolver | F1-26 | corregido: `fijado`, `PUT /plan/{id}`, resolver respeta | hecho |
| Validador de la guía sin llamar | F1-28 | corregido: corre antes de publicar | hecho |
| Jerga (*driver*) en un texto en cristiano | F1-56 | corregido | hecho |
| `hay_anunciantes` no existía | F1-57 | corregido; el alta de anunciantes es F4 | hecho + anotado |
| Interfaz escrita contra la demo, no la API | F1-54 · F1-32 · F1-46 | corregido: la API emite el contrato de la interfaz; prueba de contrato | hecho |
| Sin canal de avisos | F1-46 | corregido: Telegram y correo, solo a 7 días | hecho |
| Sin reporte de ingresos | F1-55 | **diferido a F4** | anotado en `ACEPTACION.md` |
| Envío opcional de la guía por HTTP | F1-49 | corregido: `guia_destino_http` | hecho |
| `LoudnessReport` se perdía | F1-03 | corregido: se guardan LUFS, pico y pasadas | hecho |

### 3.1 · Ingest (bloque A)

### D-1 · F1-04 · Los subtítulos CEA-608 no llegan al archivo normalizado — FALLA · tamaño **L**

**Lo que se espera.** *"Los subtítulos se extraen antes de la recodificación y
se reinsertan explícitamente en el mux del archivo normalizado — el archivo de
salida tiene subtítulos, no solo el de entrada."* (PRD §9 paso 1.3 lo dice
todavía más fuerte: *"es un paso propio del pipeline, no un efecto
secundario"*.)

**Lo que pasa.**

1. **No hay paso de extracción.** En todo `internal/ingest` no existe ninguna
   lectura de SEI A/53, ni un volcado a `.scc`, ni una reinserción. Lo único
   que hay es `-a53cc 1` cuando el codificador es exactamente `libx264`
   (`normalize.go:209-211`) — que además es el **valor por defecto** de
   libx264, o sea que no añade nada — y un `-map 0:s? -c:s <codec>` cuando el
   original trae los subtítulos en un stream aparte (`normalize.go:191-195`).
   Con cualquier otro codificador (`h264_videotoolbox`, `hevc`, NVENC…) no se
   hace absolutamente nada.
2. **El contenedor de casa los tira.** `internal/app/media.go:313` normaliza a
   `.mkv`. `subtitleCodec` devuelve `copy` para matroska
   (`normalize.go:258-265`), matroska no acepta `eia_608`, el primer ffmpeg
   falla, y el reintento de `normalize.go:134` corre con `-sn`: el archivo de
   casa sale **sin subtítulos**, y lo único que queda es una nota en el
   reporte, que ni siquiera se guarda en la base.
3. **La detección de 608 dentro de la imagen tampoco funciona con este
   ffmpeg.** `probe.go:212,261` depende del campo `closed_captions` de
   `ffprobe -show_streams`. ffprobe 9.0.1 **ya no emite ese campo** (ni
   `closed_captions` ni `film_grain` aparecen en la salida de ningún stream de
   video; comprobado con `ffprobe -show_entries stream=index,film_grain,closed_captions`).
   Es decir: en la máquina de desarrollo y en cualquier instalación con
   ffmpeg ≥ 9, `Measure.HasCaptions` nunca se enciende para 608 embebido, y ni
   siquiera se llega a poner el `-a53cc`.

**Prueba.** `internal/ingest/f1verif_ingest_test.go:222`
`TestF1_04_CEA608SobreviveAlFormatoDeCasa` — fabrica un `.mov` con una pista
CEA-608 real (muxeada desde un `.scc`; ffmpeg no sabe inyectar SEI A/53 por
línea de comando, así que es lo más cercano que se puede montar sin
herramientas de fuera, y recorre el mismo camino de código:
`CaptionFormat == "cea-608"`, `EmbeddedCEA608 == true`), normaliza al `.mkv`
que usa la aplicación, y comprueba con ffprobe que el resultado tenga
subtítulos. Falla con:

```
F1-04: el archivo normalizado salió SIN subtítulos (nota del ingest:
"el contenedor de salida no aceptó los subtítulos del original: quedaron
fuera de la copia normalizada; los subtítulos CEA-608 venían dentro de la
imagen: en F1 se copian si el codificador puede; la reinserción garantizada
es F2")
```

**Dónde se arregla.**
- `internal/ingest/probe.go` — dejar de depender de `closed_captions`: mirar
  las SEI con `ffprobe -show_frames -show_entries frame_side_data` o decodificar
  un tramo con `-f lavfi` / `cc_dec`.
- `internal/ingest/normalize.go` — un paso propio: extraer los 608 a un
  sidecar antes de recodificar, y volver a inyectarlos en el mux
  (`-a53cc` con side data reconstruida, o pista `c608` en el contenedor).
- `internal/app/media.go:313` — el contenedor de casa (`.mkv`) tiene que poder
  llevarlos, o el pipeline tiene que convertirlos a algo que sí quepa.
- `LoudnessReport.CaptionsNote` debería acabar en `media_asset` /
  `incidente`, no solo en un valor de retorno que nadie guarda.

Nota: el propio código declara la desviación (*"la reinserción garantizada es
F2"*), pero `docs/ACEPTACION.md` mantiene F1-04 dentro de F1 y no lo lista
entre los criterios corregidos, así que se marca FALLA. Si la decisión real es
diferirlo, hay que corregirlo en `ACEPTACION.md` — no en el código.

### D-2 · F1-10 · Un archivo sin audio pasa como listo — PARCIAL · tamaño **S**

**Lo que se espera.** *"Dado un archivo **sin audio detectable**, o con
`ffprobe` reportando error de lectura · … · el archivo queda en estado
`cuarentena` con `motivo_en_cristiano`."*

**Lo que pasa.** `internal/ingest/pipeline.go:117` solo manda a cuarentena
cuando `!m.HasVideo && !m.HasAudio`. Un `.mp4` con video y **sin pista de
audio** termina en `estado = listo`, y la normalización le fabrica silencio
(`NormalizeOptions.NoAudio`, `normalize.go:178-179,187`). Además, en ese caso
`LoudnessReport.Passes` queda en **0** (`normalize.go:141-145`): el archivo
sale al aire sin haberse medido nunca el volumen.

**Prueba.** `internal/ingest/f1verif_ingest_test.go:613`
`TestF1_10_SinAudioVaACuarentena`. Falla con
`un archivo sin audio detectable quedó en "listo" (err=<nil>)`.

**Dónde se arregla.** `internal/ingest/pipeline.go:117` — separar las dos
condiciones y mandar a cuarentena también `!m.HasAudio`, con un motivo del
estilo *"«X» no trae sonido: revisa el máster antes de programarlo"*, y dejar
que salga por la puerta de "dejarlo pasar bajo mi responsabilidad" que ya
existe. **Decisión de producto pendiente:** si el silencio sintético es
deliberado (hay material mudo legítimo), lo que hay que corregir es F1-10 en
`docs/ACEPTACION.md`, no el código.

### D-3 · F1-09 · Los drivers de ficha no están cableados — PARCIAL · tamaño **M**

**Lo que se espera.** Que ante un archivo sin etiquetas, sin `.nfo` y sin
carátula, identificado como serie, el ingest consulte **TVmaze primero** y
solo después un driver con clave.

**Lo que pasa.** El orden es correcto y está probado
(`f1verif_ingest_test.go:504`, pasa), pero `ingest.Providers(...)` no se llama
en ningún punto del producto: `grep -r "ingest.Providers\|ProviderConfig"
internal cmd --include='*.go'` fuera del propio `providers.go` no devuelve
nada. `app.ingestDeps` (`internal/app/media.go:217-226`) construye `Deps` sin
`Providers`, no hay clave en `settings` ni casilla en Ajustes, y `TMDB.Lookup`
devuelve directamente *"no está construido todavía"* (`providers.go:163`).
Resultado: el criterio es verdadero como propiedad de la librería y vacío como
comportamiento del sistema.

**Dónde se arregla.** `internal/app/media.go:217` (leer `settings` →
`ingest.ProviderConfig` → `Deps.Providers`), más las claves en
`internal/store` y la casilla en Ajustes de `web/`. En cuanto se encienda hay
que revisar también el matiz de F1-08 (ver su nota).

### D-4 · F1-42 · La cola nunca sabe cuándo sale al aire lo que tiene que normalizar — PARCIAL · tamaño **M**

**Lo que se espera.** *"Dado tres archivos en la cola de normalización cuyas
primeras salidas al aire son a las 8:00 AM de hoy, a las 3:00 PM de hoy y
mañana · … · los normaliza en ese orden — por cuándo salen al aire, no por
orden de llegada."*

**Lo que pasa.** El montículo ordena bien (`queue.go:191-201`, probado en
`TestColaPriorizaPorHoraDeAire`). Pero la hora de aire se la da
`App.airsAt` (`internal/app/media.go:192-204`), que busca el archivo entre los
`plan_item` de los próximos 7 días — y el resolver solo materializa
`plan_item` para material con `estado_normalizacion = listo`
(`internal/resolver/resolver.go:656`, que es justamente F1-41). Un archivo
pendiente de normalizar **nunca** está en el plan, `airsAt` devuelve el
instante cero, y `jobHeap.Less` manda al final todo lo que no tiene fecha: la
cola entera funciona por orden de llegada. Es circular: para tener prioridad
hay que estar en el plan, y para estar en el plan hay que estar ya normalizado.

**Prueba.** `internal/app/f1verif_cola_test.go:75`
`TestF1_42_LaColaSabeCuandoSaleAlAire` — dos archivos idénticos salvo el
estado de normalización, cada uno con su título y su regla diaria. El control
(archivo ya normalizado) sí tiene hora de aire; el pendiente no. Falla con
`un archivo pendiente de normalizar no tiene hora de aire`.

**Dónde se arregla.** `internal/app/media.go:192-204` — calcular la hora de
aire **desde las reglas**, no desde el plan: buscar la primera regla activa
cuyo `title_id` (o cuyo episodio) apunte a ese `media_asset` y proyectar su
próxima ocurrencia con `Channel.BroadcastDay` / `ScheduleRule.Covers`.
Alternativa: que el resolver emita, junto a los avisos `sin_material`, la
hora prevista de cada título sin material, y que `IngestFile` /
`requeuePending` la usen.

### D-5 · F1-43 · La Biblioteca real no puede pintar "aún no listo para aire" — PARCIAL · tamaño **S**

**Lo que se espera.** Que en Biblioteca el archivo en normalización aparezca
marcado *"aún no listo para aire"*.

**Lo que pasa.** El backend lo calcula y lo emite bien
(`internal/api/biblioteca.go:22-27` y `:51-86`, con
`model.MediaAsset.Ready()`), pero el contrato de nombres no coincide con el
que espera la interfaz:

| Lo que emite `GET /api/v1/biblioteca` | Lo que lee `web/src/lib/tipos.ts` |
|---|---|
| `material` (`titleOut.Material`, `biblioteca.go:33`) | `estado_material` (`tipos.ts:201`) |
| `duracion` (texto, `biblioteca.go:34`) | `duracion_ms` (número, `tipos.ts:200`) |
| — | `en_la_parrilla`, `hora`, `regla_hasta` (`tipos.ts:202-204`) |
| `GET /biblioteca/{id}` → `{"titulo":…, "episodios":[…]}` (`biblioteca.go:100-103`) | `FichaDeTitulo.lista_de_episodios[]` con `estado_material` por episodio (`tipos.ts:210-217`) |

`Biblioteca.tsx:309` compara `t.estado_material !== 'listo'`, que contra el
servidor real es siempre `undefined`: la etiqueta ámbar se pinta con el texto
vacío. En el servidor de demostración (`web/src/demo/servidor.ts:460`) sí
funciona, porque ese devuelve `estado_material`. `model.Episode` (`model.go:216`)
tampoco lleva estado de material.

**Dónde se arregla.** Alinear los dos lados: o `internal/api/biblioteca.go`
emite `estado_material`, `duracion_ms`, `en_la_parrilla` y
`lista_de_episodios[].estado_material`, o `web/src/lib/tipos.ts` +
`Biblioteca.tsx` se adaptan a los nombres del Go. Lo primero es lo coherente
con `docs/API.md`, que ya promete esos tres estados en Biblioteca.

**Fuera de F1 (N/A):** la segunda mitad de F1-43 —"y Anuncios, si es un spot"
y "una compra del portal no se da por lista hasta que termina"— no tiene
superficie en F1: no hay pantalla de Anuncios ni endpoint de compras. El
esquema ya prevé el estado (`internal/store/schema.sql:276`,
`pagado_sin_material`), pero la pantalla y el flujo son F4 según
`docs/ROADMAP.md`. Cuando F4 entre en construcción, esa mitad se vuelve a
verificar.

---

### 3.2 · Reglas y resolver (bloque B)

### D1 · F1-12 — nada impide un `plan_item` fuera del rango de fechas de su regla · **FALLA** · tamaño **S**

- **Se espera:** que la escritura falle. Un `plan_item` con `dia_emision` fuera
  de `[fecha_inicio, fecha_fin]` de su `schedule_rule_id` no puede existir.
- **Lo que pasa:** el INSERT entra sin protestar, tanto por arriba
  (`dia_emision = 2026-12-21` con la regla terminando el 20) como por abajo
  (`2026-08-08` con la regla empezando el 9). No hay CHECK ni trigger.
- **Dónde se arregla:** `internal/store/schema.sql`, junto a los dos triggers de
  no-solape (líneas 206-232). Hace falta un par de triggers
  `plan_item_dentro_de_su_regla_insert` / `_update` (este último también sobre
  `UPDATE OF schedule_rule_id, dia_emision`) del estilo:
  `SELECT RAISE(ABORT, 'plan_item fuera de la vigencia de su regla') WHERE NEW.schedule_rule_id IS NOT NULL AND EXISTS (SELECT 1 FROM schedule_rule s WHERE s.id = NEW.schedule_rule_id AND (NEW.dia_emision < s.fecha_inicio OR NEW.dia_emision > s.fecha_fin));`
  Como el esquema ya está publicado (versión 1), esto entra como
  `Migration{Version: 2, ...}` en `internal/store/migrate.go:24`, y el error
  crudo se traduce en `internal/store/errors.go:47` (`translate`) a un
  `ErrOutOfRange` nuevo, en cristiano.
- **Riesgo hoy:** bajo. El resolver nunca produce ese ítem (`ScheduleRule.Covers`,
  `internal/model/model.go:285`) y en F1 no hay endpoint que mueva un `plan_item`
  a mano. Es una garantía que falta, no un bug visible — pero es exactamente el
  tipo de garantía que el PRD §15 pide que viva en el esquema.
- **Prueba:** `internal/store/f1verif_esquema_test.go:67` (queda en rojo hasta
  que se arregle).

### D2 · F1-44 — revivir un `plan_item` descartado puede crear un solape · **PARCIAL** · tamaño **S**

- **Se espera:** que la base nunca deje dos `plan_item` del mismo deck con
  intervalos solapados, sin importar por qué camino se llegue ahí.
- **Lo que pasa:** los triggers excluyen los estados `skipped` y `fallido`
  (`internal/store/schema.sql:213` y `:226`), y el trigger de UPDATE solo vigila
  `instante_planeado_ms, duracion_planeada_ms, deck_id`
  (`internal/store/schema.sql:218`). Un `UPDATE plan_item SET estado = 'planned'`
  sobre un ítem `skipped` cuyo hueco ya lo ocupa otro **entra**, y quedan dos
  ítems solapados en el mismo deck.
- **Dónde se arregla:** `internal/store/schema.sql:218` — añadir `estado` a la
  lista de columnas del `BEFORE UPDATE OF …`, y saltar la comprobación cuando el
  nuevo estado sea `skipped`/`fallido` (`WHERE NEW.estado NOT IN ('skipped','fallido') AND EXISTS (...)`).
  Igual que D1, entra como migración 2.
- **Riesgo hoy:** bajo. `PlanRepo.SetState` (`internal/store/plan.go:190`) es el
  único camino y en F1 nadie devuelve un `skipped` a `planned`; en F2, con
  `preempted` y reintentos, deja de ser hipotético.
- **Prueba:** `internal/store/f1verif_esquema_test.go:177` (en rojo).

### D3 · F1-22 — el fundido de 1 segundo del clip recortado no existe en ninguna parte · **PARCIAL** · tamaño **S** (F1) / **M** (con F2)

- **Se espera:** que el último clip de relleno del hueco se recorte **con un
  fundido de 1 segundo** (decisión C13 de la auditoría).
- **Lo que pasa:** el recorte sí se hace (`internal/resolver/resolver.go:801`,
  `ms -= plan.trim`), pero la constante `FillerFadeMs = 1000`
  (`internal/resolver/resolver.go:54`) **no se usa en ningún sitio del
  repositorio** (`grep -rn FillerFadeMs` solo devuelve su declaración). El
  `model.PlanItem` no lleva ningún campo que diga "este clip va recortado, hay
  que fundirlo", así que el motor no puede distinguirlo de un clip entero, y su
  fundido por defecto es de **5 ms**, no de 1 s
  (`internal/engine/frameserver.go:65-66`).
- **Dónde se arregla:** hay dos mitades. La de F1 es dejar el dato en el plan:
  un campo tipo `fundido_salida_ms` en `model.PlanItem` / `plan_item` que
  `fill()` (`internal/resolver/resolver.go:781-812`) rellene con `FillerFadeMs`
  en el clip recortado — o, si se prefiere no tocar el esquema, que el criterio
  se reescriba para decir que el fundido es responsabilidad del motor (F2) y
  que `FillerFadeMs` viva en `internal/engine`. La otra mitad (aplicar el
  fundido de verdad) es F2.
- **Estado como criterio:** hoy es **inverificable**: en modo sombra nada sale al
  aire, y el plan no guarda la intención. Decidir cuál de las dos salidas se
  toma es una decisión de diseño, no un bug de código.

### D4 · F1-39 — una regla que arranca dentro de un bloque en vivo lo recorta y se desborda de su fin · **PARCIAL** · tamaño **M**

- **Se espera:** *"el fin del bloque en vivo es tan duro como el de un slot de
  archivo"*: un elemento que no termine antes de las 13:00 **no se arranca**.
- **Lo que pasa:** con un vivo de 10:00 a 13:00 (`duracion_slot_ms = 3 h`) y una
  regla de archivo a las 12:30 con un programa de 45 min, el plan sale así:

  ```
  10:00  «en vivo»          150.0 min   ← recortado: debía durar 180
  12:30  El que se pasa      45.0 min   ← termina a las 13:15, fuera del vivo
  ```

  Es decir: el bloque en vivo pierde media hora y el programa **sí** arranca y
  se desborda. Lo esperado sería el vivo entero (10:00-13:00) y las 12:30
  cubiertas por relleno, con su aviso de sobrecupo.
- **Causa:** `resolveHardEnds` (`internal/resolver/resolver.go:414-431`) trata el
  arranque de la instancia siguiente como fin duro de la anterior
  (`internal/resolver/resolver.go:424`) **incluso cuando la anterior tiene
  `duracion_slot_ms` explícito**, y no propaga hacia atrás el fin declarado del
  vivo como fin duro de la instancia que arranca dentro de él.
- **Dónde se arregla:** `internal/resolver/resolver.go:414-431`. Dos cambios: (1)
  una instancia con `SlotMs > 0` (vivo o `bloque_arrendado`) no se deja recortar
  por la que arranca después —eso es un conflicto, y debería avisar como tal en
  `resolveClashes`, `resolver.go:367`—; (2) el `hardEnd` de una instancia que
  cae dentro de la ventana de un vivo se limita al fin de ese vivo.
- **Además, para el enunciado literal:** el criterio habla de un elemento
  `dentro_de`. En F1 el resolver no materializa ítems `dentro_de` (solo el
  esquema los admite, `internal/store/schema.sql:190`, probado en
  `TestPlanItemDentroDeUnVivo`); la inserción de cortes dentro de un vivo llega
  con F2/F4. Esa mitad del criterio no se puede cerrar en F1.
- **Prueba:** `internal/resolver/f1verif_reglas_test.go:544` (en rojo).

### D5 · F1-16 — `MarkAired` escribiría un contador propio en la regla de repetición · observación · tamaño **S**

- **Se espera:** que una regla con `repite_a` **no avance ningún contador
  propio** (auditoría B5).
- **Lo que pasa:** `App.MarkAired` (`internal/app/resolve.go:359-378`) busca la
  regla del ítem en `EpisodeAdvance`; para un ítem de repetición esa clave no
  está (el resolver la guarda bajo el id de la **primaria**), así que cae al
  `else` de la línea 370 y llama a `SetLastEpisode(reglaDeRepeticion, episodio)`.
  Queda escrito un `ultimo_episodio_emitido` en la regla de repetición.
- **Impacto hoy:** ninguno. El resolver siempre lee el contador del dueño
  (`owner = primary.ID`, `internal/resolver/resolver.go:497-504`) y en F1 nadie
  llama a `MarkAired` (canal en modo sombra). El riesgo es futuro: si algún día
  se le quita el `repite_a` a esa regla, hereda un contador viejo y arranca
  desincronizada.
- **Dónde se arregla:** `internal/app/resolve.go:370` — si el ítem viene de una
  regla con `repite_a`, escribir el contador en la regla primaria, no en ella.
  Es de F2 (el motor todavía no llama a `MarkAired`); se anota aquí para que no
  se pierda.
- **No cambia el veredicto de F1-16:** el resolver, que es lo que F1 construye,
  hace lo correcto y está probado.

---

### 3.3 · Guía, importador e interfaz (bloque C)

### D-1 · F1-26 — La edición a mano de la parrilla no sobrevive al resolver · **M**

**Qué se espera.** El programador cambia un `plan_item` en estado `planned` de mañana; el
cambio se guarda y la corrida automática de la hora siguiente no lo pisa.

**Qué pasa.** No hay por dónde hacerlo, y si se hace a mano en la base, se pierde:

- `docs/API.md` y `internal/api/server.go:88-100` no traen ninguna ruta
  `PUT/PATCH /plan/{id}`: el plan solo se lee (`GET /plan`, `/plan/semana`, `/plan/mes`)
  y se recalcula entero (`POST /plan/recalcular`).
- `model.PlanItem` (`internal/model/model.go:327-348`) no tiene ninguna marca de origen
  humano ni de «no me toques»; el esquema (`internal/store/schema.sql:176-198`) tampoco.
- `App.Resolve` borra **todo** lo `planned` desde el corte hacia adelante
  (`internal/app/resolve.go:96` → `internal/store/plan.go:163` `DeleteFuturePlanned`,
  `WHERE estado = 'planned' AND instante_planeado_ms >= ?`) y reinserta lo que el
  resolver acaba de calcular. Lo `cued`/`aired` se respeta; lo tocado a mano no,
  porque no se distingue de lo generado.
- En la interfaz el panel de arrastre «¿Solo hoy, o siempre?»
  (`web/src/pantallas/ParrillaSemana.tsx:387-424`) tiene los dos botones cableados a
  `setArrastre(null)`: no llaman a la API, no guardan nada.

Reproducido en `TestF1Verif26LaEdicionAManoSobreviveAlResolver`
(`internal/app/f1verif_guia_test.go:229`): se mueve un bloque de mañana cinco minutos y
la corrida siguiente lo devuelve a su hora original.

**Dónde se arregla.** Un estado o bandera nueva (`manual_hold` ya existe como estado de
`plan_item` en el esquema, o una columna `fijado`); excluirla en
`store.PlanRepo.DeleteFuturePlanned`; enseñarle al resolver a no reprogramar esa franja
(`resolver.Input.Existing` ya llega, hoy solo se usa para lo que está en marcha,
`internal/resolver/resolver.go:687`); una ruta `PUT /plan/{id}` con auditoría; y cablear
los dos botones del panel de arrastre. **Tamaño M.**

---

### D-2 · F1-28 — El validador de la guía es código muerto · **S**

**Qué se espera.** El validador propio corre **antes de publicar** el XMLTV y rechaza la
publicación cuando algo no cuadra.

**Qué pasa.** `resolver.ValidateXMLTV` (`internal/resolver/xmltv.go:213`) está bien escrito
y bien probado — caza solapes, fin antes que inicio, canal no declarado, huecos largos y
XML roto — pero `grep -rn ValidateXMLTV internal/ cmd/ | grep -v _test.go` devuelve **solo
su propia definición**. `App.rebuildGuide` (`internal/app/resolve.go:255-299`) llama a
`resolver.XMLTV`, guarda el resultado en `a.guide` y lo escribe en disco sin pasar por él.

**Dónde se arregla.** En `internal/app/resolve.go`, dentro de `rebuildGuide`, entre
`resolver.XMLTV(...)` y el `a.mu.Lock()`: si `ValidateXMLTV(data)` devuelve problemas,
no reemplazar `a.guide` ni escribir el archivo, publicar un evento/incidente con los
problemas y devolver error (`Resolve` ya lo pinta con
«no se pudo publicar la guía: …», `internal/app/resolve.go:118`). Prueba pinchada:
`TestF1Verif28ElValidadorCorreAntesDePublicar`. **Tamaño S.**

---

### D-3 · F1-56 — Jerga prohibida en un texto en cristiano del ingest · **S**

**Qué se espera.** Ninguno de los cinco términos (*driver*, *códec*, *GOP*, *LKFS*,
*transport stream*) en nada que llegue a ver el operador.

**Qué pasa.** Una sola cadena:

```
internal/ingest/providers.go:164
  return nil, Plainf(nil, "el driver de TMDB no está construido todavía: llega en F2,
  con su clave y su atribución visible en pantalla")
```

`Plainf` es exactamente la máquina de `motivo_en_cristiano` (`internal/ingest/ingest.go:49`),
así que ese texto acaba en la ficha de cuarentena que se enseña en Biblioteca.

**Dónde se arregla.** Reescribir la frase sin la palabra: p. ej. *«todavía no busco fichas
en TMDB: llega en F2, con su clave y su crédito visible en pantalla»*. **Tamaño S.**

*(Fuera de esa cadena, la jerga que queda en el repo está en comentarios, nombres de
columna SQL e identificadores de Go —`internal/ingest/normalize.go`, `internal/store/media.go`,
`internal/model/model.go:132,156`— que nunca llegan a pantalla y que el criterio no cubre.)*

---

### D-4 · F1-57 — `hay_anunciantes` no existe en el servidor · **S/M**

**Qué se espera.** Cinco entradas de menú sin anunciantes; seis al registrar el primero.

**Qué pasa.** La interfaz lee `estado.hay_anunciantes`
(`web/src/componentes/Armazon.tsx:29`, tipo en `web/src/lib/tipos.ts:100`) y
`GET /api/v1/estado` no lo manda: `estadoBody` (`internal/api/estado.go:15-28`) tiene
`canal, modo, ahora, dia_emision, al_aire, siguiente, alarmas, version,
necesita_instalacion, instalacion_completa, ffmpeg, guia_generada, entraste` y nada más.
El `?? false` de la interfaz deja el menú clavado en cinco entradas para siempre.

Además no hay forma de registrar un anunciante: la tabla `advertiser`
(`internal/store/schema.sql:251`) no tiene repositorio en `internal/store`
(`Store` no expone `Advertiser`, `internal/store/store.go:50-67`) ni ruta en `docs/API.md`.

**Dónde se arregla.** `internal/api/estado.go`: añadir el campo y llenarlo con un
`SELECT EXISTS(SELECT 1 FROM advertiser WHERE channel_id = ?)`. Si además se quiere
poder pasar a seis de verdad, hace falta el repositorio mínimo de `advertiser` y una
ruta de alta. **Tamaño S** (solo el campo), **M** con el alta de anunciantes.

---

### D-5 · F1-54 / F1-32 / F1-46 — La interfaz está escrita contra los datos de demostración, no contra la API · **M**

Este es el defecto que más criterios de interfaz arrastra. La interfaz cae en modo
demostración si el servidor no contesta (`web/src/lib/api.ts:30-48`), y varios tipos de
`web/src/lib/tipos.ts` describen el JSON de `web/src/demo/`, no el que sirve `internal/api`:

| Pantalla | La interfaz espera | La API manda | Efecto |
|---|---|---|---|
| Al aire | `alarmas: Alarma[]` con `{nivel, texto, detalle, accion}` (`tipos.ts:78-83`, `AlAire.tsx:229-231`, `AlAire.tsx:337-360`) | `alarmas: []string` (`internal/api/estado.go:22`, `internal/app/app.go:390`) | Las tarjetas de alarma salen en blanco. Y ahí es donde F1-46 pide el aviso de vencimiento. |
| Al aire | `estado.salidas`, `estado.retorno_de_aire`, `estado.control_manual` | no se sirven | Bloque «SALIDAS» vacío. |
| Parrilla · Semana | `dias[].dia`, `franjas[].en_vivo`, `franjas[].duracion_ms`, `dias[].horas_vacias`, `semana.nota` (`tipos.ts:130-148`) | `dias[].dia_emision`, `franjas[] = {hora, titulo, estado, hueco}` con `titulo = "sin programar"` para los huecos (`internal/api/plan.go:170-231`) | Los huecos **no se marcan** (nunca son `null`, son un título más) y la tira se dibuja desde la medianoche mientras la API la manda desde el inicio del día de emisión (6:00 AM). Esto es la mitad de F1-54. |
| Parrilla · Mes | `dias[].dia`, `dias[].franjas_llenas`, `horas_vacias_mes`, `porcentaje_vacio` (`tipos.ts:150-163`, `ParrillaMes.tsx:86-134`) | `dias[].dia_emision`, `horas_sin_llenar`, `vencimientos`, `estrenos` (`internal/api/plan.go:290-296`) | `d.dia.slice(8)` sobre `undefined` revienta la pantalla. Es una de las tres pantallas que F1-46 exige. |
| Parrilla · Guía | `{filas: [...]}` y día fijo `'2026-09-08'` (`ParrillaGuia.tsx:13-20`) | arreglo pelado (`internal/api/plan.go:521`) | La comparación guía-contra-plan de F1-32 no se puede mirar. |

**Dónde se arregla.** Alinear las dos puntas: o el servidor emite los campos que
`web/src/lib/tipos.ts` promete (más rico: `nivel`/`accion` en las alarmas,
`franjas_llenas`, `en_vivo`, `horas_vacias`), o la interfaz se reescribe contra
`internal/api`. Lo primero es más fiel al PRD, porque los campos de la interfaz son los
que hacen falta para pintar. Conviene además una prueba de contrato que compare las
claves de `tipos.ts` contra las etiquetas `json` de Go. **Tamaño M** (cinco pantallas,
sin lógica nueva).

---

### D-6 · F1-46 — No existe el canal de avisos · **M**

**Qué se espera.** Además de las tres pantallas, el aviso de los 7 días sale por el
**canal de avisos configurado** (Telegram, WhatsApp o correo).

**Qué pasa.** No hay nada: ni ajuste (`internal/app/app.go` solo define
`KeyGuidePath` y compañía), ni código de envío, ni pantalla de configuración en
`web/src/pantallas/Ajustes.tsx`. El único rastro de la palabra en todo el repo es la
columna `advertiser.whatsapp` de `internal/store/schema.sql:258`, que es del anunciante.

El resto de F1-46 sí está: los umbrales 30/14/7 con supresión por relevo y sin duplicar
dentro del día (`internal/resolver/resolver.go:859-892` + `internal/app/resolve.go:225`
`expiryNotices` con `LastNoticeSent`), y dos de las tres pantallas.

**Dónde se arregla.** Tres piezas: (a) llevar el aviso de vencimiento a `App.alarms`
para que aparezca en Al aire — hoy `setAlarms` solo lo usa el vigilante de disco
(`internal/app/mantenimiento.go:97,121`); (b) unos ajustes
`canal_avisos` / `canal_avisos_destino` y un enviador (Telegram bot API y `net/smtp`
cubren dos de los tres sin dependencias); (c) llamarlo desde `expiryNotices` solo en el
umbral de 7 días. **Tamaño M.**

---

### D-7 · F1-55 — No hay reporte de ingresos · **M**

**Qué se espera.** El bloque arrendado queda «en la parrilla **y en el reporte de
ingresos**, no en una libreta aparte».

**Qué pasa.** La primera mitad está: el resolver materializa `bloque_arrendado` como
cualquier otra regla (cae en el `default` de `ruleProblem`,
`internal/resolver/resolver.go:355`) y `advertiser_id`/`cobro` se guardan y se leen
(`internal/store/schedule.go:25-43,84,113`). La segunda no existe: cero coincidencias de
`ingreso`, `revenue` o `facturac` en `internal/`, `web/src/` y `docs/API.md`; no hay
repositorio de `advertiser` y la pantalla Anuncios es un cartel de «todavía no está
construida» (`web/src/App.tsx:88-95`).

**Dónde se arregla.** O se construye un reporte mínimo (una ruta
`GET /reportes/ingresos?mes=` que sume `cobro` por anunciante sobre los `plan_item` de
reglas `bloque_arrendado`, más la pantalla Anuncios), o se anota en `docs/ACEPTACION.md`
que la mitad de ingresos se difiere a F4 —que es donde la propia tabla de cobertura
coloca publicidad y cobro— y se recorta el criterio. **Tamaño M**, o **S** si es una
decisión de alcance.

---

### D-8 · F1-49 — El envío opcional de la guía por HTTP no está construido · **S** (informativo)

El criterio describe un envío opcional del XMLTV a un destino externo que puede fallar
sin afectar la guía local ni el aire. Las dos mitades obligatorias (servir `/guia.xml` y
escribir la ruta de archivo) funcionan y están probadas; el envío opcional simplemente no
existe, así que hoy no hay nada que pueda fallar. Se anota para que quede claro que ese
tercio del criterio no está verificado, no que esté roto. Si se construye, va en
`App.rebuildGuide` después del `os.Rename`, en una goroutine con su propio error, nunca
en el camino que devuelve error a `Resolve`.

---

## 4 · Estado final (cierre de la sesión, 9 de septiembre de 2026)

Todo lo de la tabla del §3 marcado «hecho» está en el árbol y probado. Los dos
diferidos (F1-04 a F2, el reporte de ingresos de F1-55 a F4) y los cuatro
alcances ajustados (F1-22, F1-39, F1-43, F1-57) quedan anotados en
`docs/ACEPTACION.md`, junto al criterio y en la tabla del Resumen.

| | |
|---|---|
| Criterios [AUTO] verificados en verde | 52 de 52 en alcance F1 (F1-04 saltado con nota) |
| Criterios [MANUAL] pendientes de firma humana | 3 — F1-32 (comparar el plan con lo que Rolando emitió), F1-43 (Biblioteca marca «aún no listo para aire»), F1-53 (el importador pregunta «¿Zoids releva a Magic Knight?») — la superficie existe para los tres |
| Pruebas nuevas | 11 archivos `f1verif_*_test.go` + `contrato_test.go`, `plan_manual_test.go`, `avisos_test.go` |
| Migración de esquema | versión 2: `fundido_salida_ms`, `fijado`, triggers de vigencia, trigger de solape que vigila `estado`; base nueva y base migrada quedan idénticas (probado) |

Un ajuste de última hora fuera de los defectos: `resolver.ValidateXMLTVPorGravedad`
separa lo grave (no se publica) de los avisos de aire sin describir (se publica y
se avisa), y `internal/app` ya no clasifica por el texto del mensaje.

```
go vet ./...            → limpio
go test ./... -count=1  → ok api · app · importer · ingest · resolver · store
make antena             → bin/antena con la interfaz dentro
```

Humo con el binario nuevo sobre una copia de los datos de la noche anterior:
arranca, aplica la migración 2 (los triggers `plan_item_dentro_de_su_regla_*`
aparecen en `sqlite_master`), `GET /api/v1/estado` trae `entraste`,
`hay_anunciantes` y las alarmas como objetos, y `/guia.xml` responde 200.

**F1 queda cerrada** salvo la firma humana de los tres criterios manuales, que
se hace con Rolando en el modo sombra con archivos de verdad (siguiente paso en
`CONTINUAR.md`).
