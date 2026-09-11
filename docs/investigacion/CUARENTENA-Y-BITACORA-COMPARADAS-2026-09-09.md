# Cuarentena y bitácora: Antena787 comparado con proyectos abiertos

Fecha: 9 de septiembre de 2026.
Alcance: comparar la cuarentena de material y la bitácora de incidentes de Antena787 con ffplayout (Rust), nebula/conti (Python+TS), Rivendell (C++), LibreTime (Python/PHP+Liquidsoap) y jaskie/PlayoutAutomation (C#). Investigación hecha leyendo código fuente y documentación oficial en GitHub, sin tocar el repo de Antena787.

---

## 1. Resumen ejecutivo

Ninguno de los cinco proyectos tiene algo tan completo como la cuarentena de Antena787: motivo en texto humano, código de motivo, quién autorizó, botón de "dejarlo pasar" con auditoría, y una distinción explícita de qué motivos NO se pueden pasar por alto (sin audio). Tampoco ninguno tiene una tabla de incidentes tipada con panel 7/30/90 días y API por rango de fechas — todos usan logs de texto plano (ffplayout, Rivendell), pub/sub efímero (nebula) o archivos de log por servicio (LibreTime), y ffplayout incluso mezcla "fallback por diseño" con "falla real" bajo el mismo nivel `ERROR`, justo lo que Antena787 evita a propósito. Rivendell documenta un crash real (issue #86) cuando falta un archivo programado — exactamente el escenario que el resolver de Antena787 previene.

Cosas que vale la pena copiar o adaptar: (1) la prueba de reproducibilidad real de LibreTime (intenta reproducir el archivo con Liquidsoap, no solo probea metadata) como validación adicional a ffprobe; (2) el umbral de discrepancia de duración audio/video de ffplayout (4s) y jaskie (1s) como nuevo `motivo_codigo`; (3) el timeout de "Pending" a "Failed" de LibreTime (1 hora) para que un ingest colgado no desaparezca silenciosamente; (4) el as-run rico de Rivendell (ISCI, anunciante, evento start/stop en tiempo real) como inspiración para F4 de publicidad; (5) la exportación CSV/PDF con pestañas (Log/Resumen por archivo/Resumen por programa) de LibreTime, pensada para evidencia ante regalías o anunciantes.

Antena787 ya va por delante en: la persistencia estructurada y consultable de incidentes (nadie más lo tiene), la distinción de motivos override-ables vs no override-ables, y el resolver que nunca programa material en cuarentena (evitando el tipo de crash que sí sufre Rivendell).

---

## 2. Tabla comparativa

| Proyecto | Estado de material inválido | Campos que guarda | Override humano registrado | Validaciones de ingest | Bitácora de eventos (dónde vive / campos) | Interfaz de la bitácora | Exportación | As-run |
|---|---|---|---|---|---|---|---|---|
| **Antena787** (referencia) | `cuarentena` en `media_asset` | `motivo_claro`, `motivo_codigo` (hoy solo `sin_audio`), `dejado_pasar_por` | Sí, botón + nombre → `audit_log`; `sin_audio` NO es override-able | ffprobe (léase, audio) | Tabla `incidente(channel_id, tipo, inicio_ms, fin_ms, detalle)` | Pantalla Al aire: últimos 4 + panel 7/30/90 días | API `GET /incidentes?desde&hasta` | `plan_item` con estado `aired`/`manual_hold`; `audit_log` con cadena de hash |
| **ffplayout** (Rust) | Ninguno persistido. Un validador de playlist (`check_media`) solo escribe línea de log si falla el probe | Nada en BD; solo texto de log | No existe | Falta de duración/streams; diferencia AV >4s (`MAX_AV_DURATION_DIFFERENCE_SECONDS`); silencio total (opcional, `logging.detect_silence`); no valida loudness, resolución, fps ni negro/silencio inicial-final como gate de aceptación — los "arregla" (letterbox, fps, congela último frame, agrega silencio) | Archivos de log rotativos por canal (`flexi_logger`), texto plano; fallback a filler se loguea como `error!` igual que una falla real | Visor de texto (`LoggingView.vue`) con filtro de severidad client-side; no agrupa por tipo ni tiene panel 7/30/90 | `GET /api/log/{id}?download=true` (texto plano, no CSV/PDF) | No encontrado; `GET /api/program/{id}` es la playlist planeada, no lo emitido |
| **nebula** (Python/TS) | `ObjectStatus.CORRUPTED` (técnico, sin motivo) + `qc/state` manual (NEW/AUTO_REJECTED/AUTO_ACCEPTED/REJECTED/ACCEPTED, sin automatizar) | Ninguno estructurado; `qc/report` es texto libre opcional | `qc/state` se edita a mano en el editor, sin registrar quién ni cuándo; solo el plugin opcional `fill_solver` exige `qc/state=4` | Solo "¿ffprobe pudo leerlo?"; sin chequeo de audio ausente, duración mínima, fps, negro/silencio; campos de loudness (`audio/r128/*`) existen en el esquema pero ningún servicio los calcula | Sin tabla ni archivo; todo es texto a stdout/stderr o mensajes Redis pub/sub efímeros (si nadie miraba la pantalla, se pierde) | Estado como palabra de color en columna de tabla; sin lista de incidentes ni motivo | No encontrada | Tabla `asrun(id_channel, id_item, start, stop)`, mínima, sin duración planeada vs real; doc oficial promete "commercial billing export" pero no hay código público que lo haga |
| **Rivendell** (C++) | Ninguno persistido; el archivo malo simplemente no se importa | Nada en BD; motivo va por correo (`Journal::addFailure`, texto libre, solo si el grupo tiene email configurado) | No existe | Solo lectura/formato (probe) + normalización de picos; sin loudness ni duración mínima. `NoCart`/`NoCut` es un estado en memoria (colorea la línea de rojo), no se persiste | Syslog de Unix, texto libre (`rda->syslog(...)`), sin tipo/severidad estructurados; issue #86 documenta que un cart faltante puede **congelar RDAirPlay** | Ninguna en la app; se consulta con herramientas del SO (`journalctl`, `grep`) | N/A para el "log" de sistema | `ELR_LINES` (rico: ISRC, ISCI, anunciante, evento start/stop en tiempo real) alimenta ~20 formatos de reporte (BMI/ASCAP, SoundExchange, WideOrbit, etc.) vía RDLogManager, pero las filas se **borran** tras exportar — no es un historial permanente |
| **LibreTime** (Python/PHP) | `import_status` en `cc_files`: Success(0)/Pending(1)/Failed(2), un solo código genérico | Motivo va reciclado en el campo `comment` ("hack" según el propio código); timeout de 1h pasa Pending→Failed automático | No existe; única acción en UI es "Clear" (borrar) | Prueba de **reproducción real** con Liquidsoap (`output.dummy(...)`), no solo metadata — si Liquidsoap no puede tocarlo, falla | Sin tabla de incidentes tipada. `cc_live_log` solo cuenta minutos vivo/programado. Dead air/fallback a silencio en Liquidsoap no se persiste, solo logs de texto por servicio. "System Status" es snapshot en vivo vía `monit`, sin historial | Página de estado en vivo (no histórica); logs de texto por servicio en disco | No para incidentes | `cc_playout_history` (separada del schedule, alimentada en tiempo real desde Liquidsoap); UI "Playout History" con 3 pestañas (Log Sheet, File Summary, Show Summary) y export Copy/CSV/PDF/Print, pensado para regalías (SoundExchange/BMI) pero requiere tagging manual de metadata |
| **jaskie/PlayoutAutomation** (C#) | `TMediaStatus` enum (Unknown/Available/CopyPending/Copying/Copied/Deleted/CopyError/Required/**ValidationError**) | Nada estructurado más allá del enum; log NLog a archivo | No encontrado | Compara duración audio vs video (>1s de diferencia → `ValidationError`); archivo sin audio se acepta igual (no bloquea) | No encontrado ningún log de eventos consultable en UI; solo NLog a archivo de texto, no expuesto al operador | Aviso por evento del rundown (fondo rojo + tooltip vía `MediaErrorInfo`: Missing/TooShort/Expired), no hay bandeja centralizada | No encontrada | No encontrado |

---

## 3. Detalle por proyecto

### ffplayout (Rust)

El repo fue reescrito completo en Rust puro; hoy es monorepo (`backend/app`, `backend/engine`, `frontend` Vue 3). Confirmé el esquema SQLite completo y no existe ninguna tabla de cuarentena ni de incidentes:
- https://github.com/ffplayout/ffplayout/blob/main/migrations/00001_create_tables.sql

El validador de playlist solo loguea, nunca bloquea ni persiste:
```rust
if probe.format.duration.is_none() && node.duration <= 0.0 {
    error_list.push("Engine probe returned no media duration".to_string());
}
if probe.video.is_empty() && probe.audio.is_empty() {
    error_list.push("Engine probe returned no audio or video stream".to_string());
}
```
https://github.com/ffplayout/ffplayout/blob/main/backend/app/src/player/utils/json_validate.rs

Detección de silencio confirma explícitamente que un archivo sin pista de audio es un error distinto de "silencio", vía test:
```rust
#[test]
fn silence_detection_rejects_media_without_audio_stream() {
    let error = detect_audio_silence(&media_mix_asset("no_audio.mp4"), 0.0, 15.0, -30.0, 15.0)
        .expect_err("no_audio.mp4 must not be analyzed as silent audio");
    assert!(error.to_string().contains("input has no audio stream"), "{error:#}");
}
```
https://github.com/ffplayout/ffplayout/blob/main/backend/engine/src/utils/media_info.rs

El README declara como *feature* el auto-conformado en vez de rechazo: "conform audio and video... letterbox or pillarbox... change fps... add silence if audio duration is too short; hold the last frame if video duration is too short" — https://github.com/ffplayout/ffplayout/blob/main/README.md

Un fallback a filler se loguea con el mismo nivel `ERROR` que una falla real, sin distinguir "cubierto por diseño" de "falló de verdad":
```rust
ClipResult::Fallback { reason } => {
    if !node.source.is_empty() {
        error!(channel = id; "failed while playing {}: {reason}; fallback generated", display_source);
    }
}
```
https://github.com/ffplayout/ffplayout/blob/main/backend/app/src/player/output/playout.rs

No hay endpoint de aprobación de medios en la API: https://github.com/ffplayout/ffplayout/blob/main/docs/api.md — y el explorador de archivos (`MediaView.vue`) no muestra estado ni motivo: https://github.com/ffplayout/ffplayout/blob/main/frontend/src/views/MediaView.vue

Exportación de log: `GET /api/log/{id}?date&timezone&download=true`, texto plano — https://github.com/ffplayout/ffplayout/blob/main/backend/app/src/api/routes/log.rs

### nebula (Python/TS) + conti

`ObjectStatus.CORRUPTED`: "Object is corrupted, and cannot be used" — https://github.com/nebulabroadcast/nebula/blob/develop/backend/nebula/enum.py

Ojo: `quarantine_time` en Nebula NO es un estado de rechazo — es solo el tiempo que un watchfolder espera antes de tocar un archivo, para no procesar uno a medio subir — https://github.com/nebulabroadcast/nebula-worker/blob/develop/services/watch/watch.py

El servicio de ingest solo distingue "se pudo leer" vs "no":
```python
nebula.log.warning(f"{asset}: Asset is corrupted")
asset["status"] = ObjectStatus.CORRUPTED
asset["qc/state"] = 0
```
https://github.com/nebulabroadcast/nebula-worker/blob/develop/services/meta/__init__.py

`qc/state` (NEW/AUTO_REJECTED/AUTO_ACCEPTED/REJECTED/ACCEPTED) existe en el esquema pero `AUTO_REJECTED`/`AUTO_ACCEPTED` nunca se asignan en ningún servicio — solo se edita a mano, sin registro de quién ni cuándo: https://github.com/nebulabroadcast/nebula/blob/develop/backend/nebula/enum.py y https://github.com/nebulabroadcast/nebula/blob/develop/frontend/src/pages/AssetEditor/EditorNav.tsx

Campos de loudness (`audio/r128/i`, `audio/r128/t`, `audio/r128/lra`) están en el esquema de metadata pero ningún servicio de `nebula-worker` los calcula: https://github.com/nebulabroadcast/nebula/blob/develop/backend/setup/defaults/meta_types.py

El motor `conti` (repo separado `immstudios/conti`) no recupera automáticamente de un error de encoder; hay un TODO explícito en el código:
```python
self.logger.error("Encoder error")
self.stop()
# TODO: start encoder again, seek to the same position and resume
```
https://github.com/immstudios/conti/blob/develop/conti/conti.py

El esquema completo del servidor (`schema.sql`) no tiene ninguna tabla `log`/`incident`/`audit_log`: https://github.com/nebulabroadcast/nebula/blob/develop/backend/schema/schema.sql

`asrun(id, id_channel, id_item, start, stop)` — mínima: https://github.com/nebulabroadcast/nebula/blob/develop/backend/schema/schema.sql. La documentación oficial promete más de lo que el código abierto entrega:
> "As-Run Log: An automatic audit trail... Nebula compiles this log for compliance reports, copyright auditing, and commercial billing exports."
https://github.com/nebulabroadcast/nebula-docs/blob/main/docs/basics/concepts.md — pero no hay módulo de facturación visible en los repos públicos revisados.

### Rivendell (C++)

Un archivo que no se puede leer simplemente no se importa; el "motivo" solo llega por correo si el grupo tiene email configurado:
```cpp
subject=QObject::tr("Rivendell import FAILURE for file")+": "+filename;
body+=QObject::tr("Reason")+": "+err_msg+"\n";
```
https://github.com/ElvishArtisan/rivendell/blob/d81f7ea3f0755ddfc4af4119eb3bdf637028f060/utils/rdimport/journal.cpp

No hay chequeo de loudness ni duración mínima (`grep` sin resultados en `rdimport`); solo formato/lectura y normalización de picos. `NoCart`/`NoCut` es un estado en memoria (colorea línea de rojo), nunca se escribe a la tabla de la Library: https://github.com/ElvishArtisan/rivendell/blob/d81f7ea3f0755ddfc4af4119eb3bdf637028f060/lib/rdlog_line.h

Consecuencia documentada de no tener cuarentena: issue #86, "rdairplay Crashes when missing audio file is encountered" — *"rdairplay freezes... No audio is played back from this moment forward until rdairplay is restarted"* — https://github.com/ElvishArtisan/rivendell/issues/86. Esto es justo lo que el resolver de Antena787 evita al no programar nunca material en cuarentena.

El "log de sistema" es el syslog de Unix, texto libre — la propia wiki lo remite ahí: https://wiki.rivendellaudio.org/index.php/Log_Creation

El as-run real es `ELR_LINES`, alimentado en el momento exacto del segue/stop:
```cpp
LogTraffic(logline,(RDLogLine::PlaySource)(play_id+1), RDAirPlayConf::TrafficFinish, play_onair_flag);
```
con campos ricos (ISRC, ISCI, anunciante, compositor, `EVENT_DATETIME`, `EVENT_TYPE`) — https://github.com/ElvishArtisan/rivendell/blob/d81f7ea3f0755ddfc4af4119eb3bdf637028f060/lib/rdlogplay.cpp y https://github.com/ElvishArtisan/rivendell/blob/d81f7ea3f0755ddfc4af4119eb3bdf637028f060/docs/tables/elr_lines.txt

Esas filas se generan para ~20 formatos de reporte (`RDReport::ExportFilter`: CbsiDeltaFlex, BmiEmr, SoundExchange, RadioTraffic, NaturalLog, CutLog, etc. — https://github.com/ElvishArtisan/rivendell/blob/d81f7ea3f0755ddfc4af4119eb3bdf637028f060/lib/rdreport.h) y luego se **borran** tras exportar: https://github.com/ElvishArtisan/rivendell/blob/d81f7ea3f0755ddfc4af4119eb3bdf637028f060/lib/rdreport.cpp — es decir, es una cola de reconciliación, no un historial permanente como el `plan_item`/`audit_log` de Antena787.

La documentación oficial describe el propósito exacto que Antena787 también persigue con su bitácora:
> "A Rivendell report is a data output that details whether certain events aired as scheduled, and under what circumstances... as a means of reconciling exported schedules."
https://opsguide.rivendellaudio.org/html/sect.rdlogmanager.generating_reports.html

### LibreTime (Python/PHP + Liquidsoap)

`import_status` con 3 valores (Success/Pending/Failed) y detección por **reproducción real**, no solo metadata:
```python
def analyze_playability(filename, metadata):
    try:
        _liquidsoap("-v", *("-c", "output.dummy(audio_to_stereo(single(argv(1))))"), "--", filename)
    except CalledProcessError as exception:
        raise UnplayableFileError() from exception
```
https://github.com/libretime/libretime/blob/main/analyzer/libretime_analyzer/pipeline/analyze_playability.py

El motivo del fallo se guarda reciclando el campo `comment`, marcado como "hack" en el propio código:
```python
audio_metadata["comment"] = reason  # hack attack
```
https://github.com/libretime/libretime/blob/main/analyzer/libretime_analyzer/status_reporter.py

Timeout automático de 1 hora para ingest colgado:
```php
public const PENDING_FILE_TIMEOUT_SECONDS = 3600;
```
https://github.com/libretime/libretime/blob/main/legacy/application/services/MediaService.php

Única acción en UI para un archivo fallido es "Clear" (borrar) — no hay override: https://github.com/libretime/libretime/blob/main/legacy/public/js/airtime/library/plupload.js

El dead air / fallback a silencio existe en el script de Liquidsoap pero no se registra como evento en BD, solo aparece como metadata "offline" en el stream — https://github.com/libretime/libretime/blob/main/playout/libretime_playout/liquidsoap/2.1/ls_script.liq. "System Status" es un snapshot en vivo vía `monit`, sin historial: https://github.com/libretime/libretime/blob/main/legacy/application/models/Systemstatus.php

El as-run (`cc_playout_history`) sí está separado del schedule y alimentado en tiempo real desde Liquidsoap vía `libretime-playout-notify`: https://github.com/libretime/libretime/blob/main/playout/libretime_playout/liquidsoap/2.1/ls_lib.liq y https://github.com/libretime/libretime/blob/main/api/libretime_api/history/models/played.py

La UI de "Playout History" documenta exportación explícita pensada para regalías:
> "exported as data in CSV format... exported as a document in PDF format... This page has three tabs: Log Sheet, File Summary and Show Summary."
https://libretime.org/docs/user-manual/playout-history/ — pero requiere tagging manual de Composer/Copyright para ser útil ante SoundExchange/BMI: https://libretime.org/docs/appendix/rights-royalties/

### jaskie/PlayoutAutomation (C#) — prioridad baja

Repo cerrado desde 2024, sin docs dedicadas a estos temas; toda la evidencia es de código.

`TMediaStatus` incluye `ValidationError`, y `MediaChecker` compara duración audio/video (>1s de diferencia → `ValidationError`); un archivo sin audio se acepta sin bloquear:
https://raw.githubusercontent.com/jaskie/PlayoutAutomation/develop/TAS.Server/Media/MediaChecker.cs

El aviso al operador es por evento dentro del rundown (fondo rojo + tooltip), no una bandeja centralizada de "material parado":
https://raw.githubusercontent.com/jaskie/PlayoutAutomation/develop/TAS.Client/ViewModels/EventPanelMovieViewModel.cs

No se encontró ningún log de eventos consultable en la UI (solo NLog a archivo, no expuesto al operador) ni ningún as-run/reporte de lo emitido. Se revisaron explícitamente los 48 viewmodels de `TAS.Client/ViewModels` sin encontrar nada equivalente: https://github.com/jaskie/PlayoutAutomation/tree/develop/TAS.Client/ViewModels

---

## 4. Recomendaciones concretas para Antena787

1. **Agregar una prueba de reproducibilidad real, no solo ffprobe.** Inspirado en LibreTime (intenta reproducir con Liquidsoap antes de aceptar). En Antena787 sería un intento corto de decode con ffmpeg (no solo ffprobe) para atrapar archivos que "probean bien" pero fallan al reproducirse de verdad.
   - Fase: F1 ahora.
   - Esfuerzo: mediano.
   - Contradice algún principio: no.

2. **Nuevo `motivo_codigo`: `duracion_av_no_coincide`** cuando la duración de audio y video difieren más de un umbral (ffplayout usa 4s, jaskie usa 1s). Antena787 hoy solo tiene `sin_audio`; este es el candidato más claro para el segundo motivo automático.
   - Fase: F1 ahora.
   - Esfuerzo: chico.
   - Contradice algún principio: no.

3. **Timeout de ingest colgado.** Si el procesamiento de ffprobe/normalización nunca termina, pasar el archivo a cuarentena con motivo `procesamiento_colgado` en vez de dejarlo en limbo indefinido (como el timeout de 1h de LibreTime que pasa Pending→Failed automático). Evita que un archivo se "pierda" silenciosamente de la vista del operador.
   - Fase: F1 ahora.
   - Esfuerzo: chico.
   - Contradice algún principio: no — de hecho refuerza "nunca se programa material en cuarentena" al no dejar zonas grises.

4. **Validación de negro/silencio al inicio o al final del archivo.** Ninguno de los cinco proyectos investigados hace esto tal como se pregunta (ffplayout solo detecta silencio total del clip, no específicamente al inicio/fin, y es opcional). Es una oportunidad real de diferenciación, no de copiar a nadie.
   - Fase: F1 ahora (si el esfuerzo de ffmpeg lo permite) o F2 si requiere más trabajo de encoder.
   - Esfuerzo: mediano.
   - Contradice algún principio: no.

5. **NO copiar el auto-conformado silencioso de ffplayout** (letterbox/pillarbox automático, cambio de fps, relleno de audio corto, congelar último frame) como sustituto de la cuarentena. Sí vale la pena considerar el auto-conformado de **aspecto/fps como transformación cosmética visible en incidente** (tipo `normalizacion_fallida` ya existe; se podría agregar `normalizacion_aplicada` para que quede evidencia de que se ajustó algo, no que se ocultó un problema de contenido).
   - Fase: F2 playout, si se decide hacer.
   - Esfuerzo: grande.
   - Contradice principio: en su forma "silenciosa" (como en ffplayout) sí contradice "el software no regaña pero tampoco esconde" y la filosofía de transparencia de Antena787 — solo adoptar la versión que deja rastro en la bitácora.

6. **No repetir el error de ffplayout de loguear "fallback por diseño" con la misma severidad que una falla real.** Antena787 ya tiene tipos distintos en `incidente` (`relleno_por_defecto` vs `panico_<goroutine>`, etc.), lo cual ya evita este problema — se recomienda mantener esa separación estricta al agregar nuevos tipos en F2, y no colapsarlos en un solo nivel de severidad genérico.
   - Fase: F2 playout (aplica al diseñar los tipos nuevos: `vivo_ausente`, `encoder_colgado`, etc.).
   - Esfuerzo: chico (es disciplina de diseño, no código nuevo).
   - Contradice principio: al revés, refuerza "la bitácora es evidencia, no un regaño".

7. **As-run enriquecido para F4 (publicidad).** Inspirado en `ELR_LINES` de Rivendell (ISCI/código de anunciante, evento de start/stop en tiempo real) — agregar campos de identificación comercial al `plan_item` o a una tabla derivada, para que el as-run sirva directamente para reconciliación de pauta y make-goods, tal como ya está previsto en la filosofía de Antena787.
   - Fase: F4 publicidad.
   - Esfuerzo: mediano.
   - Contradice principio: no.

8. **Exportación CSV/PDF del as-run y de la bitácora de incidentes**, con vistas agregadas al estilo de LibreTime (Log Sheet / resumen por archivo / resumen por programa). Hoy Antena787 tiene `GET /incidentes?desde&hasta` pero no un export empaquetado para entregar a un anunciante o regulador.
   - Fase: F4 publicidad (para as-run) / F2 playout (para incidentes, si se necesita antes).
   - Esfuerzo: mediano.
   - Contradice principio: no — es justo lo que pide la filosofía de "evidencia, no regaño": hacerla exportable y presentable.

9. **Mantener y documentar como ventaja competitiva** el hecho de que el resolver nunca programa material en cuarentena. Rivendell no tiene ese guardrail y sufre crashes documentados (issue #86) cuando falta un archivo referenciado en el log. No es una recomendación de cambio, sino de comunicación/documentación pública del proyecto (README, comparativas), porque es una diferencia real y verificable frente al proyecto más establecido del grupo.
   - Fase: ninguna (documentación, no código).
   - Esfuerzo: chico.
   - Contradice principio: no.

10. **No agregar un campo "QC manual" tipo `qc/state` de nebula sin conectarlo a una regla real.** Nebula demuestra el riesgo: un enum de 5 estados (`NEW/AUTO_REJECTED/AUTO_ACCEPTED/REJECTED/ACCEPTED`) donde los dos "AUTO_" nunca se usan en código real — un campo de estado que no bloquea nada y nadie audita es peor que no tenerlo. Si en el futuro se agrega un QC manual, debe quedar en `audit_log` con nombre de quien lo marcó, igual que "dejar pasar".
   - Fase: advertencia para cuando se diseñe F2/F4, no una tarea en sí.
   - Esfuerzo: N/A.
   - Contradice principio: N/A (es una advertencia de diseño, no una recomendación positiva).

---

## 5. Fuentes

### ffplayout
- https://github.com/ffplayout/ffplayout
- https://github.com/ffplayout/ffplayout/blob/main/README.md
- https://github.com/ffplayout/ffplayout/blob/main/docs/api.md
- https://github.com/ffplayout/ffplayout/blob/main/docs/notifications.md
- https://github.com/ffplayout/ffplayout/blob/main/migrations/00001_create_tables.sql
- https://github.com/ffplayout/ffplayout/blob/main/migrations/00002_recording.sql
- https://github.com/ffplayout/ffplayout/blob/main/backend/app/src/player/utils/json_validate.rs
- https://github.com/ffplayout/ffplayout/blob/main/backend/engine/src/utils/media_info.rs
- https://github.com/ffplayout/ffplayout/blob/main/backend/app/src/utils/logging.rs
- https://github.com/ffplayout/ffplayout/blob/main/backend/app/src/api/routes/log.rs
- https://github.com/ffplayout/ffplayout/blob/main/backend/app/src/api/routes/program.rs
- https://github.com/ffplayout/ffplayout/blob/main/backend/app/src/player/output/playout.rs
- https://github.com/ffplayout/ffplayout/blob/main/backend/app/src/player/input/playlist.rs
- https://github.com/ffplayout/ffplayout/blob/main/frontend/src/views/MediaView.vue
- https://github.com/ffplayout/ffplayout/blob/main/frontend/src/views/LoggingView.vue

### nebula / conti
- https://github.com/nebulabroadcast/nebula
- https://github.com/nebulabroadcast/nebula-worker
- https://github.com/immstudios/conti
- https://github.com/nebulabroadcast/nebula-docs/blob/main/docs/setup/watchfolders.md
- https://github.com/nebulabroadcast/nebula-docs/blob/main/docs/basics/concepts.md
- https://github.com/nebulabroadcast/nebula/blob/develop/backend/nebula/enum.py
- https://github.com/nebulabroadcast/nebula/blob/develop/backend/schema/schema.sql
- https://github.com/nebulabroadcast/nebula/blob/develop/backend/setup/defaults/meta_types.py
- https://github.com/nebulabroadcast/nebula/blob/develop/frontend/src/lib/tableFormat/formatObjectStatus.tsx
- https://github.com/nebulabroadcast/nebula/blob/develop/frontend/src/pages/AssetEditor/EditorNav.tsx
- https://github.com/nebulabroadcast/nebula-worker/blob/develop/services/meta/__init__.py
- https://github.com/nebulabroadcast/nebula-worker/blob/develop/services/psm/psm.py
- https://github.com/nebulabroadcast/nebula-worker/blob/develop/services/play/play.py
- https://github.com/nebulabroadcast/nebula-worker/blob/develop/nebula/mediaprobe.py
- https://github.com/nebulabroadcast/nebula-worker/blob/develop/nebula/log.py
- https://github.com/nebulabroadcast/powerpack/blob/main/solver/fill_solver.py
- https://github.com/nebulabroadcast/powerpack/blob/main/api/asset_runs.py
- https://github.com/nebulabroadcast/nebula-server-plugins/blob/main/plugins/api/show_runs.py
- https://github.com/immstudios/conti/blob/develop/conti/conti.py

### Rivendell
- https://github.com/ElvishArtisan/rivendell
- https://github.com/ElvishArtisan/rivendell/blob/d81f7ea3f0755ddfc4af4119eb3bdf637028f060/utils/rdimport/rdimport.cpp
- https://github.com/ElvishArtisan/rivendell/blob/d81f7ea3f0755ddfc4af4119eb3bdf637028f060/utils/rdimport/journal.cpp
- https://github.com/ElvishArtisan/rivendell/blob/d81f7ea3f0755ddfc4af4119eb3bdf637028f060/lib/rdlog_line.h
- https://github.com/ElvishArtisan/rivendell/blob/d81f7ea3f0755ddfc4af4119eb3bdf637028f060/rdairplay/rdairplay.cpp
- https://github.com/ElvishArtisan/rivendell/blob/d81f7ea3f0755ddfc4af4119eb3bdf637028f060/rdairplay/colors.h
- https://github.com/ElvishArtisan/rivendell/blob/d81f7ea3f0755ddfc4af4119eb3bdf637028f060/lib/rdcae.cpp
- https://github.com/ElvishArtisan/rivendell/blob/d81f7ea3f0755ddfc4af4119eb3bdf637028f060/lib/rdlogplay.cpp
- https://github.com/ElvishArtisan/rivendell/blob/d81f7ea3f0755ddfc4af4119eb3bdf637028f060/lib/rdreport.h
- https://github.com/ElvishArtisan/rivendell/blob/d81f7ea3f0755ddfc4af4119eb3bdf637028f060/lib/rdreport.cpp
- https://github.com/ElvishArtisan/rivendell/blob/d81f7ea3f0755ddfc4af4119eb3bdf637028f060/docs/tables/elr_lines.txt
- https://github.com/ElvishArtisan/rivendell/issues/86
- https://opsguide.rivendellaudio.org/html/sect.rdlogmanager.generating_reports.html
- https://opsguide.rivendellaudio.org/html/chapter.rdlogedit.html
- https://opsguide.rivendellaudio.org/html/sect.utilities.rdimport.html
- https://wiki.rivendellaudio.org/index.php/Log_Creation

### LibreTime
- https://github.com/libretime/libretime
- https://github.com/libretime/libretime/blob/main/analyzer/libretime_analyzer/pipeline/pipeline.py
- https://github.com/libretime/libretime/blob/main/analyzer/libretime_analyzer/pipeline/analyze_playability.py
- https://github.com/libretime/libretime/blob/main/analyzer/libretime_analyzer/status_reporter.py
- https://github.com/libretime/libretime/blob/main/api/libretime_api/storage/models/file.py
- https://github.com/libretime/libretime/blob/main/legacy/application/models/airtime/CcFiles.php
- https://github.com/libretime/libretime/blob/main/legacy/application/services/MediaService.php
- https://github.com/libretime/libretime/blob/main/legacy/public/js/airtime/library/plupload.js
- https://github.com/libretime/libretime/blob/main/api/libretime_api/legacy/migrations/sql/schema.sql
- https://github.com/libretime/libretime/blob/main/legacy/application/models/Systemstatus.php
- https://github.com/libretime/libretime/blob/main/legacy/application/models/LiveLog.php
- https://github.com/libretime/libretime/blob/main/api/libretime_api/history/models/played.py
- https://github.com/libretime/libretime/blob/main/api/libretime_api/schedule/models/schedule.py
- https://github.com/libretime/libretime/blob/main/playout/libretime_playout/liquidsoap/2.1/ls_script.liq
- https://github.com/libretime/libretime/blob/main/playout/libretime_playout/liquidsoap/2.1/ls_lib.liq
- https://libretime.org/docs/user-manual/playout-history/
- https://libretime.org/docs/appendix/rights-royalties/
- https://libretime.org/docs/admin-manual/troubleshooting/
- https://github.com/LibreTime/libretime/issues/508

### jaskie/PlayoutAutomation
- https://github.com/jaskie/PlayoutAutomation
- https://github.com/jaskie/playoutautomation/wiki/Concepts
- https://raw.githubusercontent.com/jaskie/PlayoutAutomation/develop/TAS.Common/Enums.cs
- https://raw.githubusercontent.com/jaskie/PlayoutAutomation/develop/TAS.Server/Media/MediaBase.cs
- https://raw.githubusercontent.com/jaskie/PlayoutAutomation/develop/TAS.Server/Media/MediaChecker.cs
- https://raw.githubusercontent.com/jaskie/PlayoutAutomation/develop/TAS.Client/ViewModels/EventPanelMovieViewModel.cs
- https://raw.githubusercontent.com/jaskie/PlayoutAutomation/develop/TAS.Client/ViewModels/EngineViewModel.cs
- https://raw.githubusercontent.com/jaskie/PlayoutAutomation/develop/TAS.Client/Views/EventPanelMovieView.xaml
- https://raw.githubusercontent.com/jaskie/PlayoutAutomation/develop/TVPlayClient/NLog.config
