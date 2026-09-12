# Auditoría: lanzar, esperar y matar procesos en Windows

Repo `antena787`, rama `main`. Foco único: cómo se comportan los procesos de ffmpeg
(y el propio `antena.exe`) cuando esto corre en Windows 10 arrancado con el `.bat`.
No se tocó git ni se escribió código de producción.

## 1. Ventanas negras (`exec.Command` / `CREATE_NO_WINDOW`)

Búsqueda en todo el repo: no hay ni un `SysProcAttr`, ni `HideWindow`, ni
`CREATE_NO_WINDOW` en ningún archivo `.go` (confirmado por grep de
`SysProcAttr\|HideWindow\|CREATE_NO_WINDOW` sobre `*.go`).

Con el `.bat` actual (`scripts/Arrancar-Antena787.bat`) esto **no rompe nada**:
el `.bat` llama `antena.exe -datos "datos"` directo, sin `start`, así que
`antena.exe` hereda la consola de `cmd.exe`. Todo hijo que lanza sin pedir una
consola nueva (que es el caso de todos los `exec.Command`/`CommandContext` de
este repo) se cuelga de esa misma consola heredada — no abre ventana propia.
Revisado: no hay ventanas negras parpadeando en el uso previsto (doble clic al
`.bat`).

**Caveat real:** el comentario de `cmd/antena/main.go:20-22` dice que en
producción esto corre "bajo el supervisor del sistema — servicio de Windows o
systemd". Un servicio de Windows no tiene consola. Si algún día se instala
`antena.exe` como servicio (NSSM u otro), cada `exec.Command` a ffmpeg SÍ va a
abrir una consola nueva y visible por un instante, porque no hay
`CREATE_NO_WINDOW` puesto en ningún lado. Para el `.bat` de hoy no aplica; para
un futuro "instálalo como servicio" sí hay que ponerlo.

## 2. Matar un proceso (`Matar()` / `Close()`)

- `internal/engine/encoder.go:653-659` (`Matar`) y
  `internal/engine/decoder.go:199-210` (`Close`) matan con
  `cmd.Process.Kill()` — en Windows eso es `TerminateProcess`, sin aviso
  previo. No existe un `SIGTERM` que decirle a ffmpeg para que cierre el
  archivo de salida bien.
- Esto está **documentado y aceptado a propósito** en el propio código
  (`encoder.go:283-285`: "Matarlo a mitad deja un TS truncado" — por eso
  `Finish()` existe como camino normal y `Matar()` es solo para un ffmpeg
  colgado que no tiene nada que vaciar). No es un descuido; es una decisión
  tomada sabiendo la limitación de Windows.
- Consecuencia real en Windows cuando sí se usa `Matar()`/`Close()`: el
  archivo de salida (o el pipe) queda cortado a la mitad de un frame, tal
  como dice el comentario — comportamiento esperado, no una sorpresa.

## 3. Procesos huérfanos — el hallazgo más grave

Grep de `JobObject\|CREATE_NEW_PROCESS_GROUP\|CTRL_BREAK\|Job\b` sobre todo el
repo: nada. **No hay Job Object en ningún lado.**

En Windows los procesos hijos NO mueren con el padre a menos que estén
metidos en un Job Object con
`JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`. Sin eso, si `antena.exe` muere de golpe
— lo mata el Administrador de Tareas, se cae la máquina y arranca sola, un
`panic` no recuperado — **todos los ffmpeg que estaban vivos en ese momento
(el encoder persistente + los decodificadores de los clips en aire + los que
estén midiendo/normalizando) se quedan corriendo para siempre**, sin nadie
que los pare, comiéndose CPU y disco hasta que alguien los mate a mano en el
Administrador de Tareas. Con el uso diario que describe la tarea (decenas de
ffmpeg al día) esto se acumula rápido si la estación reinicia `antena.exe`
más de una vez sin limpiar a mano.

Esto es serio y **no está resuelto en ningún archivo del repo**.

## 4. `Wait` y `WaitDelay` — arreglado a medias

El proyecto ya se colgó una vez en Windows por esto (issue #14, documentado en
`internal/engine/decoder.go:120-123`): sin `WaitDelay`, `Wait()` se queda
esperando a que ffmpeg suelte sus tuberías después de matarlo, y en Windows
eso puede no pasar nunca.

El arreglo (`d.vcmd.WaitDelay = 3 * time.Second` / `d.acmd.WaitDelay = 3 *
time.Second`) está puesto **solo en `internal/engine/decoder.go:123` y
`:134`**. En ningún otro sitio del repo hay un `WaitDelay` puesto (grep de
`WaitDelay` sobre todo `*.go`: dos resultados, los dos en `decoder.go`, más el
comentario que los explica). Concretamente **falta** en:

- `internal/engine/encoder.go:301` (`e.Cmd`) — se mata con
  `Process.Kill()` en `:638` y `:658`, y se espera con `e.Cmd.Wait()` en
  `:307`, sin `WaitDelay`. Mismo patrón que causó el issue #14.
- `internal/ingest/probe.go:149` y `:217` — `exec.CommandContext` con
  `context.WithTimeout` (`:141`, `:215`); cuando el plazo se cumple, Go mata
  el proceso y `cmd.Run()` (que hace `Wait()` por dentro, línea `:154` y
  `:222`) puede quedarse colgado igual que el decoder antes del arreglo. Esto
  corre en cada archivo que se ingiere — un archivo dañado o un disco de red
  lento puede trabar la cola de ingesta entera en Windows.
- `internal/f0/analyze.go:412-486` (`checkAudio`): mata con
  `cmd.Process.Kill()` en `:485` y espera con `cmd.Wait()` en `:486` sobre un
  `cmd` con `StdoutPipe()` (`:414`) — el mismo patrón exacto del issue #14,
  sin el arreglo. (Esto es una herramienta de verificación F0, no el camino
  en vivo, pero es el mismo bug reapareciendo.)
- `internal/ingest/normalize.go:358`, `:396` y `internal/ingest/blacksilence.go:71`
  también usan `exec.CommandContext` sin `WaitDelay`.

En corto: el arreglo del issue #14 se hizo puntual para el decodificador y no
se generalizó. Todo lo demás que mata o cancela por contexto un ffmpeg con
tubería puede volver a colgarse en Windows.

## 5. Tuberías y stdout sin plazo

Cubierto en el punto 4: los sitios de riesgo son los que combinan
`StdoutPipe`/`Stderr` de buffer con un `Kill()` o una cancelación de
contexto sin `WaitDelay` (arriba). No se encontró un sitio que lea de una
tubería en un bucle infinito sin ningún mecanismo de salida (todos los
`for { io.ReadFull(...) }` cortan al primer error/EOF) — el riesgo no es "se
lee para siempre", es "`Wait()` no vuelve nunca" tras matar al proceso.

## 6. El `.bat` (`scripts/Arrancar-Antena787.bat`)

Revisado línea por línea:

- `cd /d "%~dp0"` — correcto: `/d` cambia también de unidad, y `%~dp0` entre
  comillas cubre rutas con espacios. Sin problema con espacios en la ruta.
- Acentos en el nombre de usuario de Windows (p. ej. `C:\Users\José\...`):
  no hay evidencia de un problema aquí — el `.bat` no construye rutas con el
  nombre de usuario, y `antena.exe` recibe `"datos"` como ruta relativa ya
  posicionado por el `cd /d`. Revisado, no hay problema.
- Los textos del `echo` están escritos sin tildes a propósito ("esta",
  "proposito") — evita el problema clásico de code page de `cmd.exe`
  mostrando símbolos raros en vez de tildes. Bien pensado.
- Cerrar con la X en vez de Ctrl-C: Windows manda un evento de cierre de
  consola a todo lo que está colgado de esa consola — y por el punto 1, eso
  incluye a los ffmpeg hijos, porque comparten la consola. Windows da muy
  poco tiempo (unos segundos) antes de forzar el cierre de todo el árbol.
  `apagar()` en `cmd/antena/main.go` tiene un plazo de 10 s
  (`context.WithTimeout(context.Background(), 10*time.Second)`) y
  `Encoder.Finish()` (`internal/engine/encoder.go:635`) espera hasta 60 s
  antes de matar — ninguno de los dos plazos le va a dar tiempo a terminar
  antes de que Windows fuerce el cierre. Consecuencia esperable: cerrar con
  la X trunca el archivo en curso en vez de cerrarlo limpio, igual que un
  apagón. No es un huérfano (el punto 3 es peor: ahí SÍ quedan corriendo)
  porque aquí mueren todos juntos por estar en la misma consola — pero sí es
  un cierre sucio, no el cierre ordenado que el código intenta hacer.
- El `pause` final es correcto para que alguien lea el error si el programa
  se cae solo; si se cierra con la X, el `pause` nunca corre — pero eso es
  inevitable y no cambia nada más.

## Lo que está bien hecho

- `internal/engine/decoder.go:120-134`: el `WaitDelay` puesto sí, con
  comentario explicando el issue #14 — la única parte del repo que ya
  aprendió la lección de Windows.
- `internal/despierto/despierto_windows.go`: no lanza ningún subproceso para
  evitar que la máquina duerma; usa `SetThreadExecutionState` directo por
  `syscall.NewLazyDLL`, sin CGo. Ningún riesgo de proceso ahí.
- `internal/app/disco_windows.go`: mide espacio en disco con
  `GetDiskFreeSpaceExW` por DLL, no con un subproceso — evita depender de
  `wmic`/PowerShell para algo tan simple.
- El `.bat` en sí (fuera del punto de cierre con X) está bien escrito: rutas
  entre comillas, `/d`, sin tildes en los textos, `pause` al final.
