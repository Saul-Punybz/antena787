# Auditoría: red, puertos y lo de ayer en Windows — 12 sept 2026

Alcance: solo red/puertos (multicast, HTTP, IPv6/localhost) y las tres piezas
escritas el 11-12 sept 2026 que solo se han compilado para Windows (DPAPI,
reparto HTTP, SQLite). No se tocó código ni rutas de proceso — eso lo cubren
otros dos agentes en paralelo.

## Parte 1 · Red

### 1. Multicast sin `localaddr` — CRÍTICO, confirmado y ya conocido por el proyecto

`internal/engine/encoder.go:565-578` (`urlUDP`) arma la URL de ffmpeg con
`pkt_size` y `ttl`, pero **nunca con `localaddr=`**. Confirmé con
`grep -rn "localaddr" .` que la cadena no aparece en ningún `.go` del
repositorio: cero implementación.

El propio equipo ya lo escribió como hueco el 11 sept 2026, después de que
Rolando confirmara que el TP1000 tiene **dos tarjetas de red** (una recibe
video, otra es de manejo) — `docs/PARA-ROLANDO-WHATSAPP.md:230-235`:

> «Si el PC de la torre tiene más de una tarjeta de red, el sistema operativo
> escoge por cuál sale el multicast según su tabla de rutas, y escoge mal a
> menudo. Hay que poder decir por cuál tarjeta sale (`localaddr=` en la URL de
> ffmpeg). Hoy no existe: cero apariciones en todo el repositorio.»

Eso sigue siendo cierto hoy, 12 sept 2026: nadie lo construyó todavía.

**Qué le pasa a la estación.** En una torre con más de una tarjeta (la de
video hacia el TP1000 y la de manejo/internet), Windows decide la interfaz de
salida del datagrama multicast según su tabla de rutas — el mismo
comportamiento que documentan reportes de proyectos comparables (Ant Media
Server, MediaMTX) para este escenario exacto: [Multicast stream not received
— ant-media-server#7418](https://github.com/ant-media/ant-media-server/issues/7418),
[Specify listening/capturing interface for UDP — mediamtx#2533](https://github.com/bluenviron/mediamtx/issues/2533).
Si Windows elige la tarjeta equivocada, el paquete sale por donde el
multiplexor no está escuchando y **no llega nada al aire**, sin ningún error
visible en Antena787 — porque UDP no avisa si nadie lo recibe. El síntoma en
la torre sería «el canal dice que está transmitiendo pero el TP1000 no ve
nada», que es exactamente el tipo de falla más difícil de diagnosticar a
control remoto.

**Riesgo adicional del propio equipo, ya leído del manual del TP1000**
(`docs/PARA-ROLANDO-WHATSAPP.md:255-259`): la versión de IGMP del PC tiene que
coincidir con la del switch, o el multicast tampoco llega — y nada en el
software lo explica.

### 2. `httpts.go` escucha en todas las interfaces — grave pero ya advertido en la guía

`internal/drivers/salida/httpts.go:294`: `net.Listen("tcp", fmt.Sprintf(":%d", puerto))` —
a propósito, según el comentario de la línea 292-293 («quien tira de la señal
está en otro equipo de la torre»). Esto es correcto para el caso de uso.

**Primera vez que arranca:** el cortafuegos de Windows va a preguntar. Ya está
cubierto en `docs/INSTALAR-WINDOWS.md:66-71` («Dale a permitir en redes
privadas»). No es un hueco nuevo.

**Puerto ya tomado:** `servidorEn` (línea 286-306) devuelve un error en
palabras claras («seguramente otro programa de esta máquina ya lo está
usando; escoge otro puerto o cierra el que lo tiene») y `INSTALAR-WINDOWS.md:112-113`
ya le dice a la persona cómo cambiar el puerto con `-escucha`. **Bien resuelto.**

Un caso que ni el código ni la guía cubren: un antivirus corporativo o un
firewall administrado que bloquea el puerto **en silencio**, sin mostrar el
cuadro de diálogo de Windows (pasa con algunas suites de seguridad
gestionadas). Ahí no hay ningún mensaje de Antena787 que lo distinga de «el
canal no está al aire»: quien mira desde otra computadora simplemente no
conecta. Es un caso de borde, no un hueco grande. Documentarlo en la
bitácora de «si algo no funciona» sería barato.

### 3. IPv6 / `localhost` / `127.0.0.1` — revisado, no hay problema

- La escucha de la interfaz web es literal `127.0.0.1:7870`
  (`cmd/antena/main.go:43`), no `localhost`: no depende de cómo resuelva el
  sistema. `INSTALAR-WINDOWS.md:110` sugiere `http://localhost:7870` como
  alternativa si `127.0.0.1` no abre, pero como el servidor solo escucha en
  IPv4 explícito, si algún día `localhost` resolviera primero a `::1` en el
  navegador, esa sugerencia fallaría — hoy no rompe nada porque la
  recomendación principal ya usa la IP literal.
- Las dos escuchas internas ffmpeg↔Go (`internal/engine/encoder.go:272,276` y
  `internal/drivers/salida/httpts.go:204`) son `127.0.0.1:0`: mismo host,
  mismo proceso, sin ambigüedad de IPv6.
- `internal/app/media.go:1001-1003` ya deja escrita una trampa de
  `url.Parse` con `localhost:puerto` sin esquema (el `Host` queda vacío y el
  `Scheme` se lee «localhost») y el código ya se defiende comprobando la
  lista de esquemas válidos, no que el esquema no esté vacío. Bien hecho.
- `internal/app/avisos.go:206` usa `"localhost"` solo como parte del
  remitente de un correo cuando no hay servidor SMTP configurado — no toca
  la red de la torre.

En resumen — revisado, no hay problema de IPv4/IPv6 en las rutas
que importan para el multiplexor o el HTTP de monitor.

## Parte 2 · Lo de ayer, nunca corrido en Windows

### 4. DPAPI (`internal/store/llave_windows.go`)

**Manejo de memoria — bien hecho.** `envolver` y `desenvolver` (líneas 19-44)
llaman `copiaDeBlob(salida)` **dentro del `return`**, y el `defer
windows.LocalFree(...)` corre después de evaluar los valores de retorno pero
antes de que la función entregue el control a quien llamó — en Go los defers
se ejecutan tras calcular los valores de retorno. Así que la copia a un slice
de Go pasa **antes** de que `LocalFree` suelte la memoria que Windows
reservó. No hay use-after-free. `copiaDeBlob` (líneas 48-52) usa
`unsafe.Slice(b.Data, b.Size)` solo para leer y copiar, nunca para retener el
puntero. Revisado con cuidado: correcto.

**Diseño de "si falla, ¿arranca igual?" — bien hecho.**
`AbrirSecretos`/`nuevaLlave` en `internal/store/secretos.go:59-112` documentan
explícitamente que un fallo de llave **no puede impedir que el canal
arranque** (comentario líneas 60-62), y el llamador guarda el error en
`errLlave` sin abortar — el canal sigue al aire sin poder leer/guardar
credenciales de proveedores, que es preferible a quedarse sin aire.

**Lo que sí es un hueco real: el mensaje de error no cubre el caso más
probable.** `llave_windows.go:40` dice, si `CryptUnprotectData` falla:
«Windows no pudo leer la llave: la hizo otra cuenta o otra máquina». Pero hay
un tercer caso, más probable en una estación que en una oficina: **un
administrador resetea la contraseña de Windows sin conocer la anterior** (no
un cambio normal hecho por el propio usuario). Microsoft documenta que ese
reseteo puede dejar la master key de DPAPI **irrecuperable**, porque el
sistema necesita la contraseña vieja para volver a cifrar la master key con
la nueva, y un reset administrativo no la tiene — [Cannot access DPAPI data
after an administrator resets your password — Microsoft
Support](https://support.microsoft.com/en-us/topic/cannot-access-dpapi-data-after-an-administrator-resets-your-password-on-a-windows-server-2012-r2-based-domain-controller-3967321a-26ff-a39d-86af-00f452ae8917).
En una torre de TV, "el técnico local resetea la clave de Windows porque
alguien la olvidó" es un escenario plausible y el mensaje de error apuntaría a
la causa equivocada. No rompe el aire (por el diseño de arriba), pero
confunde a quien intenta resolverlo.

**Servicio con otra cuenta — hoy no aplica, pero es una trampa latente.**
DPAPI en modo usuario (sin `CRYPTPROTECT_LOCAL_MACHINE`, que es como está
usado aquí) ata la llave al perfil de la cuenta que la cifró; si el proceso
corriera como un servicio de Windows bajo otra cuenta (p. ej. una cuenta de
servicio o `LocalSystem`), `CryptUnprotectData` fallaría siempre —
[CryptProtectData — Microsoft
Learn](https://learn.microsoft.com/en-us/windows/win32/api/dpapi/nf-dpapi-cryptprotectdata)
y confirmado por reportes de terceros sobre DPAPI y cuentas de servicio.
`docs/INSTALAR-WINDOWS.md:4` dice explícitamente que Antena787 **no instala
servicios** — corre interactivo vía `.bat` bajo la cuenta de quien lo
arranca — así que hoy este caso no se da. Queda anotado por si el proyecto
alguna vez se empaqueta como servicio de Windows.

### 5. Reparto HTTP (`internal/drivers/salida/httpts.go`) — revisado, no hay problema de Unix

Todo el mecanismo (`recibirYRepartir`, `reparto`, `lectorHTTP`, líneas
384-570) usa sockets TCP y canales de Go puros: sin `SIGPIPE`, sin sockets de
dominio Unix, sin `epoll`/`kqueue` a mano, sin nada que dependa del manejo de
señales de Unix. `net.TCPListener.SetDeadline` y `http.Server` son
multiplataforma. La única llamada a `Process.Kill()` relevante
(`internal/engine/encoder.go:638,658`) usa la API de `os/exec` de Go, que en
Windows llama a `TerminateProcess` — no asume `SIGKILL`. No encontré
dependencia de comportamiento de Unix en este archivo.

### 6. SQLite / WAL (`internal/store/store.go:135-139`) — GRAVE, por la guía, no por el motor

El motor en sí (`modernc.org/sqlite`, WAL, `busy_timeout`) es correcto y
portable. El problema es operativo: **la guía de instalación dice que
descomprimir en el escritorio "está bien"** (`docs/INSTALAR-WINDOWS.md:27`),
y en Windows 10 **OneDrive sincroniza el Escritorio por defecto** (Known
Folder Move) para casi todas las cuentas configuradas con una cuenta
Microsoft/de trabajo.

La documentación oficial de SQLite es tajante sobre WAL y sincronización:

> «All processes using a database must be on the same host computer; WAL does
> not work over a network filesystem.» — [SQLite: Write-Ahead
> Logging](https://www.sqlite.org/wal.html)

Antena787 no corre sobre un filesystem de red en sentido estricto, pero el
riesgo práctico con OneDrive es el mismo tipo de problema por otra vía: el
motor de sincronización de OneDrive **lee y sube el archivo mientras
Antena787 lo modifica** (`datos/*.db`, `*.db-wal`, `*.db-shm`, más los
respaldos por hora que la guía promete en la línea 78). Es un patrón conocido
de corrupción/duplicados de WAL cuando un proceso de sincronización externo
toca archivos que un motor con WAL está escribiendo activamente — ver, como
caso documentado del mismo tipo de fallo con otro cliente de sincronización,
[Disk exhaustion on Windows due to massive SQLite WAL growth caused by
cross-platform sync error loops —
desktop-kDrive#1476](https://github.com/Infomaniak/desktop-kDrive/issues/1476).
En el mejor caso, OneDrive genera copias «-nombre (conflicto en este
dispositivo)» de la base o del WAL; en el peor, sube una copia a medio
escribir a la nube justo cuando Antena787 hace un checkpoint, o compite por
el candado del archivo con el propio SQLite (`internal/store/store.go:188` ya
hace checkpoint antes de cerrar, así que al menos ese momento está cubierto,
pero no los momentos intermedios).

**Recomendación concreta:** cambiar la guía para recomendar `C:\Antena787`
como la opción única, o al menos advertir explícitamente que el Escritorio en
Windows 10 puede estar sincronizado con OneDrive y que hay que excluir la
carpeta `datos` de esa sincronización (clic derecho → "Liberar espacio"
desactivado / excluir carpeta en la configuración de OneDrive). Hoy la guía
dice lo contrario de lo prudente.

---

## Resumen (máximo 20 líneas, por gravedad)

1. `internal/engine/encoder.go:565-578` (`urlUDP`) → sin `localaddr=`, Windows con 2+ tarjetas puede sacar el multicast por la interfaz equivocada y el TP1000 nunca lo recibe, sin error visible → **CRÍTICO**, ya lo documentó el propio equipo el 11 sept y sigue sin construirse.
2. `docs/INSTALAR-WINDOWS.md:27` + WAL de `internal/store/store.go:135-139` → OneDrive sincroniza el Escritorio por defecto en Windows 10 y puede pelear el candado del archivo o subir copias a medio escribir de la base/WAL → **GRAVE**, es la propia guía la que manda a la carpeta de riesgo.
3. `internal/store/llave_windows.go:40` → el mensaje de error de DPAPI no contempla el caso más probable en una torre (un admin resetea la clave de Windows sin la anterior, no "otra cuenta u otra máquina") → **MEDIO**, no tumba el aire (el diseño ya lo tolera) pero confunde el diagnóstico.
4. `internal/drivers/salida/httpts.go:294` con antivirus corporativo que bloquea el puerto en silencio (sin el diálogo de Windows) → **MENOR**, caso de borde no cubierto en la bitácora de fallas.
5. DPAPI bajo un futuro servicio de Windows con otra cuenta rompería el descifrado siempre → **LATENTE**, hoy no aplica: Antena787 no instala servicios (`INSTALAR-WINDOWS.md:4`).

**Revisado y sin problema:** manejo de memoria de DPAPI (copia antes de
`LocalFree`, líneas 19-44), diseño de "la llave falla pero el canal arranca"
(`secretos.go`), cortafuegos de Windows y puerto ocupado en `httpts.go` (ya
resueltos en código y guía), reparto HTTP sin dependencias de Unix, IPv4
literal en las tres escuchas internas (`127.0.0.1`), y la trampa de
`url.Parse("localhost:...")` ya defendida en `media.go`.
