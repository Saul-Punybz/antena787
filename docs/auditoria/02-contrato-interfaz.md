# Auditoría de contrato servidor↔interfaz — 11 sept 2026

Alcance: solo lectura, ni una línea de código tocada. Comparación ruta por
ruta entre lo que `internal/api/server.go` monta, lo que cada handler manda
de verdad (`internal/api/*.go` y `internal/model/model.go`) y lo que
`web/src/lib/tipos.ts` declara y las pantallas (`web/src/pantallas/`,
`web/src/componentes/`) leen. Cada fila se comprobó leyendo el código de los
dos lados, no por inspección de nombres.

## 1 · Ruta por ruta

| Ruta | Tipo en Go | Tipo en tipos.ts | Estado | Evidencia |
|---|---|---|---|---|
| `GET /estado` | `estadoBody` | `Estado` | **coincide** (con extras tolerados `ffmpeg`, `guia_generada`; `retorno_de_aire`/`control_manual` nunca se mandan, son opcionales) | `estado.go:16-36` vs `tipos.ts:131-151` |
| `GET/PUT /canal` | `model.Channel` | `Canal` | **coincide**, campo a campo | `model.go:88-101` vs `tipos.ts:13-26` |
| `GET/PUT /ajustes` | `map[string]string` (+`conLosDeFabrica`) | `Ajustes = Record<string,string>` | **tipo genérico coincide; contenido roto** (§3) | `estado.go:224-334` vs `tipos.ts:491` |
| `POST /entrar`, `/salir` | cuerpos ad hoc, sin interfaz declarada | — | N/A | `clave.go:269-315` |
| `GET /salidas` | `salidasBody{Salidas, Drivers}` | `SalidasDelCanal` | **coincide** | `salidas.go:23-40` vs `tipos.ts:55-58` |
| `POST/PUT/DELETE /salidas` | `model.Output` / `app.SalidaEnPantalla` | `Salida`, `DriverDeSalida` | **coincide** | `model.go:128-138`, `app/salidas.go:174-181` vs `tipos.ts:13-52` |
| `GET/POST/PUT /reglas` | `ruleOut{model.ScheduleRule, Titulo, Ficha, DaysLeft,...}` | `Regla` | **roto (silencioso)**: `releva_a_titulo`/`repite_a_titulo` no existen en ningún lado del servidor (0 resultados) pero la pantalla los lee | `reglas.go:16-41` vs `tipos.ts:157-176`; lectura en `Reglas.tsx:247-251` |
| `DELETE /reglas/{id}` | `{"borrada": id}` | — | N/A | `reglas.go:165` |
| `GET /plan` | `{"dia_emision","inicio","fin","items":[]planRow}` — **objeto**, no arreglo | tipos.ts no declara esta forma; `api.ts` la tipa `FilaDelPlan[]` | **roto** | `plan.go:193-198` vs `web/src/lib/api.ts:149`; usado como arreglo en `ParrillaSemana.tsx:131-133` (`for...of`) y `AlAire.tsx:24-27` (`filas.filter`) |
| `PUT /plan/{id}` | `cambioDePlan{Instante,Duracion,Fijado}` (cuerpo) → `planRow` | `CambioDePlan` / `ElementoDelPlan` | **coincide** en el cuerpo; la respuesta hereda el hueco de `temporada`/`en_vivo` de abajo | `plan.go:797-800` vs `tipos.ts:100-107` |
| `GET /plan/semana` | `weekSlot`/día/semana | `SemanaDelPlan`, `DiaDeLaSemana`, `FranjaSemana` | **coincide** (verificado también por `contrato_test.go:238-245`) | `plan.go:218-296` |
| `GET /plan/mes` | día del mes | `MesDelPlan`, `DiaDelMes` | **coincide** (`contrato_test.go:248-253`) | `plan.go:440-444` |
| `POST /plan/recalcular`, `/plan/llenar-con-diferido` | ad hoc, sin interfaz en tipos.ts | — | N/A (nada que romper) | `api_test.go:368,519` |
| `GET /guia.xml`, `/guia.pmcp` | XML/PMCP, no JSON | fuera del contrato de tipos.ts | N/A | `plan.go:637-677` |
| `GET /guia` | objeto `Guia` | `Guia` | **coincide** (`contrato_test.go:256-260`) | `plan.go:678+` |
| `GET /biblioteca`, `/biblioteca/{id}` | `[]titleOut`, ficha con `lista_de_episodios` | `TituloDeBiblioteca`, `FichaDeTitulo` | **coincide** (`contrato_test.go:263-274`) | `biblioteca.go:81-100,205-238` |
| `GET /material` | `[]model.MediaAsset` (superset) | `ArchivoEntrando[]` (solo con `?estado=ingiriendo`) | **coincide**, el servidor manda de más y eso está permitido | `biblioteca.go:278-298` vs `tipos.ts:260-264` |
| `POST /material/subir` | `{"recibidos": n}` | ad hoc | N/A | `biblioteca.go:535+` |
| `GET /material/{id}` | `model.MediaAsset` crudo (campo `estado`, no `estado_material`) | no se llama desde la interfaz (`api.ts` solo usa el `PUT`) | **tolerado**, sin uso | `biblioteca.go:312-323`; `grep material/ web/src/lib/api.ts` solo devuelve la línea del `PUT` |
| `PUT /material/{id}` | `s.materialOut(...)`: añade `material_id` y `estado_material` a mano | `MaterialDeAudio` | **coincide** | `biblioteca.go:395-415` |
| `GET /cuarentena` | `[]enCuarentenaOut{model.MediaAsset, Titulo, MotivoCodigo}` | `EnCuarentena` | **coincide** (`contrato_test.go:614`) | `biblioteca.go:426-449` |
| `POST /cuarentena/{id}/dejar-pasar` | `{"quien"}` cuerpo, sin interfaz de respuesta fija | — | N/A | `biblioteca.go:461+` |
| `GET /relleno` | `{"items":[]model.FillerAsset,"aviso"?}` | sin interfaz `Relleno` en tipos.ts | N/A (nada que comparar) | `biblioteca.go:516-531` |
| `POST /importar/hoja` | `ResumenDeImportacion` con `relevoPropuesto{Expires,Relieves,...}` mandando `regla_que_vence`/`regla_que_releva` | `RelevoPropuesto{regla, releva_a, texto}` | **roto** — nombres de campo distintos | `importar.go:26-32` vs `tipos.ts:374-378`; ver §6/prioridad |
| — mismo, resto de subtipos (`FilaConError`, `FechaCorrida`, `RepeticionPropuesta`, `AvisoDeImportacion`, `PosibleDuplicado`) | structs propios | interfaces propias | **coincide**, nombre a nombre | `importar.go:16-51` vs `tipos.ts:381-421` |
| `POST /importar/confirmar-relevos` | cuerpo `{regla, releva_a, repite_a}` | `{regla:number, releva_a:number}[]` (uso real en `Reglas.tsx`) | **roto** — depende del campo roto de arriba | `importar.go:461-465` vs `Reglas.tsx:558` |
| `GET /titulos/sin-emparejar`, `/buscar`; `POST /titulos/{id}/emparejar` | `tituloSinEmparejar`, `tituloDelCatalogo`, `ResultadoDeEmparejar` | mismos nombres en tipos.ts | **coincide** (`contrato_test.go:534-565`) | `titulos.go:36-57` |
| `GET /incidentes` | `[]incidenteOut{model.Incident, Texto}` | `Incidente` | **coincide** (`contrato_test.go:704`) | `registro.go:43-45` |
| `GET /auditoria`, `/auditoria/verificar` | `model.AuditEntry` y similares, sin interfaz en tipos.ts | — | N/A | `registro.go:53-104` |
| `GET /instalacion`, `POST /instalacion/paso/{n}`, `/relleno-por-defecto` | `Instalacion`, `RespuestaPasoN`, `RellenoPorDefecto` | mismos nombres | **coincide**, extensamente probado | `instalacion.go:140-161` vs `contrato_test.go:441-496` |
| Alarma (embebida en `/estado`, WS, y en varias rutas) | `app.Alarma{Tipo,Nivel,Texto,Detalle,Accion}` | `Alarma` | **coincide** | `app.go:124-138` vs `tipos.ts:115-129` |

## 2 · El empujón del WebSocket contra `GET /estado`

`pushStatus` (`ws.go:112-158`) ya trae `canal` — el bug de "modo sombra, 9
sept 2026" (WS sin `canal`, pantalla en negro) está arreglado y hay un
comentario explícito en el propio código señalándolo (`ws.go:129-130`) y una
prueba que lo cubre (`api_test.go:915-987`, línea 981: `if msg["canal"] ==
nil`).

Comparando campo por campo `pushStatus` (`ws.go:126-157`) contra
`estadoBody` (`estado.go:16-36`):

- **Ausentes de todo empujón, siempre:** `necesita_instalacion`,
  `instalacion_completa`, `ffmpeg`, `guia_generada`, `entraste`. No es un
  bug visible hoy: `lib/estado.tsx:79-84` fusiona el empujón con el estado
  anterior (`{...antes, ...e}`), así que estos campos se quedan con el
  valor que trajo el primer `GET /estado` y nunca se borran.
- **El caso real que sí queda mal:** `al_aire` y `siguiente` no son
  campos con `omitempty`, son claves que el mapa del empujón **solo pone si
  encuentra un elemento** (`ws.go:148-156`). Si no hay nada al aire ahora
  mismo (un hueco), la clave no se manda en absoluto. `GET /estado` en
  cambio usa punteros sin `omitempty` (`estado.go:21-22`,
  `AlAire *planRow \`json:"al_aire"\``) y manda `null` explícito cuando no
  hay nada. Como la interfaz fusiona por claves presentes, un `al_aire`
  puesto por el último programa que sí estaba en pantalla **se queda
  congelado** durante todo el hueco, hasta el siguiente `GET /estado`
  completo (recarga de página). `TestWebSocketEmpujaElEstado` no prueba
  este caso: solo comprueba el primer marco con algo al aire.

## 3 · Ajustes: lo que la pantalla lee contra lo que el servidor manda

`GET /ajustes` = `Settings.GetAll()` + tres valores de fábrica
(`silencio_umbral_s`, `negro_umbral_s`, `silencio_devuelve_control`,
`estado.go:238-249`). Las claves reales que el servidor puede llegar a
tener están en `app.Key*` (`app/app.go`, `app/vigilancia.go`,
`app/cumplimiento.go`): `subtitulos_estado`, `fichas_en_linea`,
`clave_tmdb`, `idioma_audio_preferido`, `avisos_canal` y sus variantes de
Telegram/correo/SMTP, `guia_destino_http`, `guia_pmcp_destino_http`, `pais`,
`calidad`. Esas **coinciden** con lo que `Ajustes.tsx` guarda con `cambiar(...)`.

Todo lo demás que `Ajustes.tsx` lee con `ajustes.xxx` **no existe en ningún
lugar del backend** (comprobado con grep sobre todo `internal/`, cero
resultados):

`version`, `sistema_operativo`, `dias_al_aire`, `antivirus_exclusiones`,
`energia_plan`, `arranque_tras_corte`, `rutas_largas`,
`actualizaciones_windows`, `actualizaciones_disponible`,
`actualizaciones_instalar_sola`, `respaldo_ultimo`, `respaldo_cada`,
`respaldo_copias`, `respaldo_tamano`, `tailscale`, `tailscale_direccion`,
`puertos_abiertos`, `hora_servidor`, `hora_desvio_s`,
`hora_aviso_si_pasa_de_s`, `aceleracion_tarjeta`, `aceleracion_probada`,
`aceleracion_resultado`, `perfil`, `volumen`, `equipo_de_alertas`
(`Ajustes.tsx:37-256`).

Consecuencias verificadas leyendo el código, no supuestas:

- Las cinco tarjetas de "SALUD DE LA MÁQUINA" (`Ajustes.tsx:37-66`)
  comparan contra `'sí'` **con tilde**; el único ajuste con botón para
  arreglarlo (`actualizaciones_windows`, línea 135) escribe también `'sí'`
  con tilde, así que ese uno sí puede llegar a marcarse bien una vez que se
  aprieta el botón. Los otros cuatro (`antivirus_exclusiones`,
  `energia_plan`, `arranque_tras_corte`, `rutas_largas`) no tienen botón ni
  proceso en el servidor que los ponga nunca en `'sí'`: se quedan en rojo
  para siempre.
- `haceCuanto(ajustes.respaldo_ultimo, ahora)` (`Ajustes.tsx:144`) con
  `respaldo_ultimo` indefinido hace `new Date(undefined).getTime()` → `NaN`,
  y `fechas.ts:229` cae en el último `return` con `min`/`h` en `NaN`: la
  tarjeta de RESPALDO pinta literalmente **"hace NaN días"**.
- El encabezado (`Ajustes.tsx:76-78`, versión / sistema operativo / días al
  aire) y las tarjetas de ACTUALIZACIONES, ACELERACIÓN y ACCESO REMOTO
  quedan en blanco de forma permanente (JSX no pinta `undefined`).
- `ajustes.perfil` (línea 170) no coincide con la clave real
  (`app.KeyQuality = "calidad"`, `app/app.go:71`): la línea "Perfil" queda
  en blanco aunque el servidor sí tenga guardada la calidad.

## 4 · Botones, formularios y enlaces sin acción

Revisado con grep dirigido (`<button`, `<form`, `<a href`, `<Link`, `<NavLink`)
sobre `web/src/pantallas/` y `web/src/componentes/`, con lectura manual de
cada candidato (74 `<button>` en 12 archivos).

- `Ajustes.tsx:148-150` — botón **"Bajar una copia"**, sin `onClick`.
- `Ajustes.tsx:162-164` — botón **"Ver qué cambia"**, sin `onClick`.
- `ParrillaSemana.tsx:560` — botón **"Escoger yo"**, sin `onClick`, al lado
  de "Llenar el fin de semana" que sí funciona.
- `ParrillaGuia.tsx:103-105` — botón **"Regenerar la guía"**, sin `onClick`;
  no existe ninguna función `regenerarGuia` en `lib/api.ts`.
- `AlAire.tsx:322-324` — el botón de confirmar **"Tomar el control"** tiene
  `disabled` fijo (no depende de ningún estado): el panel se abre
  (`AlAire.tsx:294`) pero no hay manera de completarlo. Coherente con que
  `Estado.control_manual` nunca lo manda el servidor (§1) y que no existe
  ninguna llamada de control manual en `lib/api.ts`. Es una pantalla
  decorativa completa, no solo un botón suelto.
- Formularios: solo hay un `<form>` en toda la interfaz
  (`Entrar.tsx:46`, con `onSubmit`) — correcto, no es un caso roto.
- Enlaces de alarma (`<Link to={alarma.accion.ruta}>`, `AlAire.tsx:373`):
  las rutas que manda el servidor (`/ajustes`, `/biblioteca`, `/parrilla`,
  `/al-aire`, `/reglas?filtro=vencen`, grep sobre `internal/app/*.go`)
  existen todas en `App.tsx:60-80`. Único matiz: `Reglas.tsx` nunca lee el
  parámetro `?filtro=vencen` de la URL (no hay `useSearchParams` ni
  `useLocation` en el archivo): el enlace llega bien pero la pestaña
  "vencen" no queda preseleccionada — **tolerado**, no rompe la pantalla.
- Menú (`Armazon.tsx:12-19,41`): "Anuncios" se esconde sin
  `hay_anunciantes`; `/en-vivo` no está en el menú (solo alcanzable a mano)
  y ambas rutas llevan a un `PorHacer` explícito (`App.tsx:75-90`) — es a
  propósito, documentado en el propio código, no es un hallazgo.

## 5 · El servidor demo contra el servidor real

Ninguna ruta que la interfaz llama (`web/src/lib/api.ts:120-232`) se queda
sin atender en `web/src/demo/servidor.ts` (33 rutas, todas presentes). No
hay rutas fantasma del demo que no existan en el servidor real.

El hallazgo real está al revés: **el demo esconde el bug de la sección 1**.
`GET /plan` en el servidor de verdad contesta un objeto
`{dia_emision, inicio, fin, items}` (`plan.go:193-198`), pero `api.ts:149`
tipa esa llamada como `FilaDelPlan[]` (arreglo) y así la consumen
`AlAire.tsx:24-27` y `ParrillaSemana.tsx:131-133`
(`for (const f of filas)`). El servidor demo (`servidor.ts:1270`) manda un
**arreglo plano**, igual que espera `api.ts` — o sea, el demo es fiel al
tipo (equivocado) de la interfaz, no al servidor real. Cualquiera que
pruebe estas dos pantallas en modo demo las ve funcionar bien y no tiene
manera de notar que contra el servidor de Go de verdad `for...of` sobre un
objeto lanza `TypeError: filas is not iterable`.

Aparte de eso: `POST /material/subir` y `/ws` tienen código en el demo que
nunca se alcanza desde la interfaz en modo demo (cortocircuitado antes en
`api.ts`) — código muerto inocuo, no un bug.

## 6 · Cobertura de la prueba de contrato

`contrato_test.go` cubre con `exige`/`exigeTodas` (comparación campo por
campo contra `tipos.ts`, **solo presencia de la clave, nunca el tipo del
valor**): `Estado`, `ElementoDelPlan`, `SemanaDelPlan`/`DiaDeLaSemana`/
`FranjaSemana`, `MesDelPlan`/`DiaDelMes`, `Guia`, `TituloDeBiblioteca`/
`FichaDeTitulo`, `AudioDelMaterial`, `Instalacion` y las nueve
`RespuestaPasoN`, `TituloSinEmparejar`/`CandidatoDeTitulo`/
`TituloDelCatalogo`/`ResultadoDeEmparejar`, `EnCuarentena`, `Incidente`.

`Regla` tiene un chequeo manual, no genérico, que sí mira el **tipo** del
valor: `reglas[0]["titulo"].(string)` (`contrato_test.go:199-201`) — es
justo la regresión de "modo sombra, 9 sept 2026" y por eso está escrita a
mano en vez de con `exige`. Es la prueba de que el mecanismo genérico
**no habría detectado ese bug ni el de esta semana**: `exige` solo
comprueba que la clave existe, no que sea texto y no un objeto.

Sin cobertura alguna (ni genérica ni manual):

- `Canal`, `Alarma` como interfaces completas.
- `Salida`/`SalidasDelCanal`/`DriverDeSalida` (`salidas_test.go` prueba
  comportamiento, no contrato).
- `Ajustes` — y no puede probarse con este mecanismo porque el tipo es
  `Record<string,string>`, no una interfaz con propiedades.
- La forma real de `GET /plan` (`dia_emision/inicio/fin/items`): **no
  tiene interfaz en tipos.ts**, así que ni se puede alimentar a `exige`. Es
  la raíz de §1/§5.
- `Regla` completa (solo se mira `titulo`; `dias_restantes`,
  `releva_a_titulo`, `repite_a_titulo`, `live_source_id` no se tocan).
- `ResumenDeImportacion` y subtipos — `TestImportarLaHojaDeCAtv`
  (`api_test.go:556`) prueba comportamiento, no compara contra tipos.ts.
  Es la prueba que habría atrapado el bug de `RelevoPropuesto` de §1 si
  comprobara nombres de campo.
- El empujón del WebSocket contra `Estado` completo: `TestWebSocketEmpujaElEstado`
  (`api_test.go:915-987`) solo mira `canal`, `tipo` y `modo` (línea
  981-985); no corre `exige(..., "Estado")` sobre el marco, así que el
  hallazgo de §2 (`al_aire`/`siguiente` ausentes en vez de `null`) no está
  cubierto.

### Lista mínima de rutas que faltan cubrir

1. El marco del WebSocket completo contra `Estado`, con el caso "nada al
   aire ahora" (para que `al_aire`/`siguiente` tengan que ir explícitos).
2. `Regla` completa vía `exige`, no solo `titulo`.
3. Una interfaz nueva en `tipos.ts` para la respuesta envuelta de `GET
   /plan` y `exige` sobre ella — hoy el bug de §1 es literalmente
   invisible para la prueba de contrato porque no hay tipo contra el que
   comparar.
4. `RelevoPropuesto` y el resto de `ResumenDeImportacion` vía `exige`.
5. `Canal`, `Alarma`, `SalidasDelCanal`/`Salida`/`DriverDeSalida`.
6. Una prueba de humo para `Ajustes` que compruebe presencia de las claves
   que `Ajustes.tsx` de verdad lee, ya que `Record<string,string>` no da
   cobertura automática con el mecanismo actual.

Nota de método: incluso cubriendo las seis, el mecanismo `exige` solo
comprueba **presencia de clave**. El bug de `RelevoPropuesto` (nombres
distintos) sí lo atraparía porque la clave completa falta; el bug de
"`titulo` como objeto" **no lo habría atrapado nunca** sin el chequeo de
tipo escrito a mano que ya existe para esa fila. Cualquier ruta nueva que
se agregue a `exige` sigue expuesta a ese mismo tipo de regresión salvo que
se le añada, como a `Regla.titulo`, una comprobación explícita del tipo del
valor.

## Prioridad — lo que rompe una pantalla hoy primero

1. **`RelevoPropuesto` con nombres de campo distintos**
   (`regla_que_vence`/`regla_que_releva` contra `regla`/`releva_a`,
   `importar.go:26-32` vs `Reglas.tsx:558`): cada clic en "confirmar
   relevos" manda `{regla: undefined, releva_a: undefined}`, que el
   servidor decodifica como regla `0` y contesta 400 "no encuentro la
   regla 0" (`importar.go:475-476`). Rompe el importador de hojas de CAtv
   por completo en ese paso.
2. **`GET /plan` como objeto contra `FilaDelPlan[]` esperado**
   (`plan.go:193-198` vs `api.ts:149`): `ParrillaSemana.tsx:131-133` lanza
   `TypeError` al mover un bloque sin `plan_id` conocido; `AlAire.tsx:24-27`
   se queda con la lista de huecos siempre vacía (el `.catch()` la
   silencia). El modo demo lo esconde (§5).
3. **Ajustes**: casi toda la mitad de la pantalla (salud de la máquina,
   respaldo, actualizaciones, aceleración, acceso remoto, encabezado) lee
   claves que el servidor no manda nunca; una de ellas pinta "hace NaN
   días" en pantalla.
4. **WS: `al_aire`/`siguiente` se congelan** durante un hueco en vez de
   limpiarse, porque el empujón omite la clave en vez de mandar `null`
   como sí hace `GET /estado`.
5. Botones sin acción (`Ajustes.tsx` ×2, `ParrillaSemana.tsx`,
   `ParrillaGuia.tsx`) y el panel de "Tomar el control" sin salida: no
   rompen la pantalla, pero prometen algo que no hacen nada.
6. Silenciosos, sin romper nada visible: `Regla.releva_a_titulo`/
   `repite_a_titulo` y `ElementoDelPlan.temporada` nunca los manda el
   servidor pero sí los pintan `Reglas.tsx` y `AlAire.tsx` — la etiqueta
   "releva a…" y el "Temporada N" del programa al aire nunca aparecen.
7. Cobertura de prueba: falta `GET /plan` (no tiene ni interfaz),
   `RelevoPropuesto`, `Regla` completa, el WS completo, `Canal`, `Alarma`,
   `Salidas` y `Ajustes` — en ese orden de riesgo real.
