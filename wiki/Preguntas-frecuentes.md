# Preguntas frecuentes

Respuestas cortas, con el enlace a donde está la respuesta larga. Si algo
todavía no se sabe, aquí lo dice — **el proyecto no vende certeza que no
tiene.**

> **Antes que nada: hoy Antena787 no sirve para emitir nada.** Está en Fase 0 y
> no hay instalador, ni interfaz, ni base de datos. Ver
> [Estado del proyecto](Estado-del-proyecto).

---

### ¿Sirve con mi equipo?

**Ese es el punto entero.** Ninguna marca ni modelo se asume: todo lo que toca
el mundo exterior es un **driver**, y el driver se elige **por cómo se llega al
equipo, no por marca** — red, serial, USB, contactos secos. La marca solo carga
la tabla de nombres.

Las marcas más usadas en Estados Unidos —ENDEC de Sage y DASDEC, transmisores
por SNMP, web o serial— **vienen con el suyo**, y para lo que no se conoce está
`signal-compare`, que no le habla al equipo: **compara la señal transmitida
contra la que el plan decía.** Funciona con hardware que el proyecto nunca va a
tener en la mano.

*→ [`PRD.md` §10](https://github.com/Saul-Punybz/antena787/blob/main/PRD.md#10--los-drivers) ·
[`docs/drivers/README.md`](https://github.com/Saul-Punybz/antena787/blob/main/docs/drivers/README.md)*

### ¿Corre en Windows?

**Sí.** Windows, Linux y ARM desde el mismo código: Go se compila cruzado sin
esfuerzo, que es exactamente por lo que se escogió.

Y con Windows hay trabajo extra hecho a propósito: **el instalador tiene que
hacer que un Windows 10 normal aguante 24/7** —exclusiones de antivirus,
energía sin suspensión, arranque tras corte, y control de Windows Update **por
política**— porque *"cómprate la edición LTSC"* no es una opción con presupuesto
cero. **Windows Update reiniciando la máquina a las 3 AM es el riesgo operativo
#1** de una instalación en Windows, y la pantalla de Ajustes lo vigila
permanentemente, no solo el instalador.

*→ [`PRD.md` §19](https://github.com/Saul-Punybz/antena787/blob/main/PRD.md#19--instalación-y-operación)*

### ¿Necesita internet?

**No para salir al aire.** Nada de lo que necesita internet saca el canal del
aire, y nada falla en silencio: todo degrada con un aviso claro.

La hora se puede tomar de un servidor de la red local; la guía se sigue
generando y sirviendo localmente; los avisos se acumulan y salen cuando vuelve
la conexión; y **las fichas y carátulas ya están guardadas junto al archivo
desde el ingest** — **nada se consulta en el momento de salir al aire.**

Lo único que de verdad necesita salida a internet es el **portal del
anunciante**, que hace falta que sea alcanzable desde afuera. Sin eso, sirve
solo en la red local y el material entra por WhatsApp o a mano.

*→ [`PRD.md` §19](https://github.com/Saul-Punybz/antena787/blob/main/PRD.md#19--instalación-y-operación)*

### ¿Qué cuesta?

**El software es gratis, para siempre, sin recortes.** Lo bajas, lo instalas y
no le debes nada a nadie. Todo está en la versión libre —programación, playout,
vivo, publicidad, portal, cumplimiento, MCP, **sin límite de canales**— y no hay
versión recortada, pantallas de recordatorio, funciones que caducan ni marca de
agua.

**Lo que se paga es tener a quién llamar:** $1,200 de acompañamiento hasta estar
al aire, $100 al mes de línea disponible. Y aparte, a quien lo pida: drivers a
medida, perfiles de un país nuevo, migración, adiestramiento.

*Hardware:* el caso de referencia corre con **presupuesto cero**, en un Windows
10 que la estación ya tenía. Un mini PC nuevo ronda los $200.

*→ ADR [0006](https://github.com/Saul-Punybz/antena787/blob/main/docs/adr/0006-support-not-features.md) ·
[`COMPRAR.md`](https://github.com/Saul-Punybz/antena787/blob/main/COMPRAR.md)*

### ¿Por qué no Plex?

Porque Plex es **inspiración de interfaz, no una dependencia**. La pared de
carátulas de Biblioteca está inspirada en Plex; los datos no vienen de ahí.

Las fichas salen de etiquetas embebidas, `.nfo` de Kodi y las APIs **sin clave**
de Cover Art Archive (música) y TVmaze (series) —TMDB con clave gratis— sin
ningún servidor de terceros de por medio. **Plex informa cómo se *ve* la
biblioteca, no de dónde salen sus datos.**

*→ ADR [0003](https://github.com/Saul-Punybz/antena787/blob/main/docs/adr/0003-one-bundled-binary.md)*

### ¿Por qué no CasparCG?

**No por calidad.** CasparCG es maduro y la televisión pública sueca corre
canales nacionales con él desde 2006. Se rechazó **por tamaño y forma**: es un
compositor de gráficos en tiempo real que exige una GPU con OpenGL 4.5, para un
problema que aquí es reproducción secuencial de archivos.

Y sobre todo, **es un servicio aparte con su propia instalación, configuración,
bitácoras y modos de fallo**, lo que rompe la promesa central de un archivo y un
instalador. Para un voluntario que corre un canal comunitario solo, *"algo falló
en CasparCG"* no tiene arreglo.

**Lo que se pierde a cambio, dicho de frente:** gráficos en vivo más allá del
logo del canal y el crawl de clasificados. Los rótulos inferiores no son de la
versión 1.

*→ ADR [0001](https://github.com/Saul-Punybz/antena787/blob/main/docs/adr/0001-own-playout-engine-in-go.md)*

### ¿Usa IA?

**Adentro, cero.** Ni un modelo, ni una clave, ni un costo impuesto a una
organización que no tiene ninguno. **El producto tiene que ser 100 % funcional
sin una sola línea de IA.**

Lo que sí hay es un **servidor MCP, apagado por defecto**, al que el usuario
conecta el asistente que quiera — y es **la última fase**: si nunca se
construyera, Antena787 seguiría siendo un sistema entero.

**Y la razón no es de principios, es de estructura.** *"La IA propone, el humano
aprueba"* es una convención, y las convenciones se rompen cuando alguien tiene
prisa. Como servidor MCP, **la regla se vuelve el protocolo: no existe ninguna
herramienta capaz de poner algo al aire.** Un modelo no puede sacar una estación
del aire porque la función no existe.

*(Aparte: el **código** sí se escribe mayormente con ayuda de IA, con revisión
humana línea por línea del motor, el conformado y el watchdog. Eso es cómo se
construye, no qué lleva adentro.)*

*→ ADR [0007](https://github.com/Saul-Punybz/antena787/blob/main/docs/adr/0007-ai-only-through-mcp.md) ·
[`PRD.md` §20](https://github.com/Saul-Punybz/antena787/blob/main/PRD.md#20--la-ia-al-final-y-por-fuera)*

### ¿Hace el EAS?

**No, y no lo va a hacer.** **Las alertas de emergencia se cumplen con hardware
certificado, no con este software.** Antena787 **se integra** con el equipo que
ya está aguas abajo — Sage, DASDEC, Gorman-Redlich, TFT — por serial, por relés
o por red, y si hay dos caminos usa los dos y los cruza.

Lo que sí hace es **enterarse de que hubo una alerta y reconciliarla**: los
tramos tapados se marcan, **los spots tapados no se facturan y se reprograman
como reposición**, y la bitácora distingue las pruebas semanales y mensuales de
las activaciones reales.

**Y eso solo funciona si se mira el sitio correcto:** el **retorno de aire**,
después del equipo de alertas. Comparar contra nuestra propia salida no detecta
nada, porque el motor siempre cree que emitió.

*→ ADR [0009](https://github.com/Saul-Punybz/antena787/blob/main/docs/adr/0009-truth-is-the-transmitted-signal.md) ·
[`COMPLIANCE.md`](https://github.com/Saul-Punybz/antena787/blob/main/COMPLIANCE.md)*

### ¿Qué es el retorno de aire, y por qué tanto insistir?

**Es la señal ya transmitida, capturada de vuelta después del equipo de
alertas** — con una tarjeta sintonizadora, una entrada de captura, o el stream
de monitoreo del transmisor.

**Es el único sitio donde se puede observar lo que de verdad salió.** Cuando el
equipo de alertas aguas abajo reemplaza la señal, Antena787 sigue emitiendo tan
tranquilo y **nunca se entera**: si la grabación y la verificación miraran
nuestra propia salida, una interrupción real no se vería jamás, el registro
quedaría **limpio y falso**, y la reposición del spot que nadie vio no se
propondría nunca.

**Cuesta algo, y algunas estaciones no lo van a tener. Eso se permite:** el
sistema corre en **modo `degradado` declarado**, la interfaz lo dice, y las
interrupciones se marcan a mano —también hacia atrás—, guardando siempre si lo
detectó el sistema o lo marcó una persona. **Lo que no se permite es fingir que
se verifica desde un sitio donde verificar es imposible.**

*→ ADR [0009](https://github.com/Saul-Punybz/antena787/blob/main/docs/adr/0009-truth-is-the-transmitted-signal.md)*

### ¿Cuándo está listo?

**No hay fecha, y el proyecto publica sus números en vez de prometer.** Sobre un
supuesto de **10 horas de trabajo a la semana** —un día efectivo, que es el
ritmo real de algo que se hace al lado de un trabajo:

| Hito | Optimista | Probable | Pesimista |
|---|---|---|---|
| Fase 0 decidida | 2 semanas | 1 mes | 2 meses |
| Modo sombra (propone y se compara) | 7 meses | **~1 año** | 1 año y medio |
| Madrugadas al aire | 2 años | **~3.4 años** | 4 años y medio |
| Aire completo | 2 años y medio | **~3.9 años** | 5 años |
| Primer anunciante cobrado | 4 años | **~5.6 años** | 7 años |

**Esto no es pesimismo, es aritmética.** ffplayout —un proyecto más chico, sin
publicidad, sin portal y sin cumplimiento— tomó más de dos años en llegar a algo
estable. **La única palanca real es más horas**, no recortar funciones: la
publicidad viene después del aire, y quitarla no adelanta el aire ni un día.

*→ [`PRD.md` §22.2](https://github.com/Saul-Punybz/antena787/blob/main/PRD.md#222--cuánto-tarda-esto-de-verdad) ·
[Estado del proyecto](Estado-del-proyecto)*

### ¿Puedo venderlo?

**Puedes cobrar por instalarlo, mantenerlo, adiestrar, y por el canal que corras
con él.** Lo que la licencia AGPL-3.0 exige es que **quien construya algo
comercial encima libere su propio código bajo los mismos términos** — y la
cláusula Affero cierra además el hueco del servicio alojado, que es la manera
obvia de comercializar esto sin devolver nada.

**Una estación comunitaria que vende anuncios para sostenerse no tiene ningún
problema.** De hecho, una licencia "no comercial" se descartó justo por eso:
habría dejado a los usuarios que el proyecto busca en una zona gris, y el módulo
de publicidad existe precisamente para ellos.

*→ ADR [0005](https://github.com/Saul-Punybz/antena787/blob/main/docs/adr/0005-agpl-with-dco.md) ·
[`LICENSE`](https://github.com/Saul-Punybz/antena787/blob/main/LICENSE)*

### ¿Sirve para radio?

**Sí. Una emisora de radio es un canal de televisión sin video.** Reglas, plan,
as-run, cortes publicitarios, fuentes en vivo, grabación, diferido y control
manual son **idénticos**; lo que cambia es el formato de casa —solo audio— y la
salida.

**Un canal de radio puede salir por tres lados a la vez** —la antena de FM,
internet y un canal de televisión— **sin ser tres canales**: la salida de TV
recibe ese audio con el **cartel visual** que dibuja el sistema (carátula,
título, logo, crawl). Nada se configura dos veces.

**Con una excepción dicha de frente:** la **radio musical no está completa**.
Necesita rotación musical —categorías con relojes por hora y reglas de
separación, que no se repita el mismo artista en tres horas— y **eso es un
subsistema propio, no un ajuste**. Va como fase aparte. Para radio hablada,
deportiva y de programas, funciona tal cual.

*→ [`PRD.md` §6](https://github.com/Saul-Punybz/antena787/blob/main/PRD.md#6--radio-y-televisión)*

### ¿Y ATSC 3.0?

**Está contemplado y no cambia el diseño**, porque el formato ya es un perfil y
la salida ya es un driver. Es otra pila —HEVC, AC-4, ROUTE/DASH sobre IP en vez
de MPEG-TS— y entra por ahí, no como reescritura.

**En orden de prioridad va el último**, a propósito: la FCC votó en mayo de 2026
obligar a las estaciones **de potencia completa** a completar la transición
hacia el cuarto trimestre de 2027, pero **LPTV, traductores y Class A están hoy
exentos** — y buena parte de los usuarios de Antena787 cae ahí. Se construye
**cuando haya un transmisor donde probarlo.**

*→ [`PRD.md` §12](https://github.com/Saul-Punybz/antena787/blob/main/PRD.md#12--internacional-y-cumplimiento)*

### ¿Cuántos canales aguanta?

**No hay tope de canales en el software** — ni licencia que contar, ni función
que caduque. **Un proceso sirve un canal**, así que se corre una instancia por
canal, y operar varias es una capa de administración encima que comparte
biblioteca, almacenamiento, anunciantes y grabación. **Eso también es libre.**

El límite es la máquina, no el programa: un mini PC N100 para un canal, un
servidor de 8 núcleos con GPU para cuatro a seis. **Y esas cifras son estimadas:
la Fase 0 las mide de verdad y la tabla se corrige antes de publicar la guía de
compra.**

**Para quien tiene un canal, nada de esto existe:** multi-canal, roles y
permisos **aparecen solo cuando alguien añade el segundo**.

*→ [`PRD.md` §11](https://github.com/Saul-Punybz/antena787/blob/main/PRD.md#11--escala) ·
[`PRD.md` §18](https://github.com/Saul-Punybz/antena787/blob/main/PRD.md#18--hardware)*

### ¿Y si se va la luz?

**El sistema arranca solo y vuelve al minuto que le toca.** El instalador
configura el arranque automático y el arranque tras corte; al volver, todo lo
que estaba preparado vuelve a "planificado", el motor calcula **qué debería
estar al aire ahora** y abre ese archivo **con seek al segundo correcto**.
**Nunca reinicia el bloque desde cero, y nunca vuelve a emitir algo que ya
salió.**

Y **cuenta lo que pasó**: al volver manda por el canal de avisos *"estuve fuera
6 horas 12 minutos"* y deja el incidente anotado con su hora de inicio y de fin.
Nadie tiene que reconstruirlo de memoria para explicárselo a alguien después.

Un apagón largo trae además el caso que rompe un playout: **el salto de reloj.**
Una corrección de más de 60 segundos no se trata como deriva — levanta su propia
alarma, nada ya emitido se re-emite, y el plan se recalcula **desde el instante
real**.

*→ [`PRD.md` §14.1](https://github.com/Saul-Punybz/antena787/blob/main/PRD.md#141--decisiones-técnicas-tomadas-para-poder-empezar) ·
[`PRD.md` §19](https://github.com/Saul-Punybz/antena787/blob/main/PRD.md#19--instalación-y-operación)*

### ¿Qué pasa si un archivo falla al aire?

**Nunca sale negro.** Hay una cascada: **programa → relleno → cartel**, y el
cartel es el último escalón — lleva el identificativo de la estación y su
comunidad de licencia, así que **una caída larga sigue identificando la
estación**. Si el cartel pasa de 15 minutos al aire, queda registrado como
incidente.

**El archivo que falló vuelve a cuarentena solo**, con el motivo, para que no se
programe otra vez la semana que viene y falle igual — **pero solo tras dos
fallos separados por más de cinco minutos**, para que un parpadeo de la red no
saque de la parrilla un programa bueno.

*→ [`PRD.md` §9 paso 4](https://github.com/Saul-Punybz/antena787/blob/main/PRD.md#paso-4--sale-al-aire)*

### ¿Puede el sistema quedarse en manual toda la noche?

**No. Nunca se queda en manual.** Si el plan dice que algo debería estar al aire
y nadie disparó nada, el sistema **se devuelve al automático y avisa**.

Pero ojo con qué mide: **"inactivo" nunca significa "el operador no tocó nada".**
Significa que **no está saliendo señal.** El temporizador se arma sobre silencio
o negro **real en la salida** —audio muy bajo o pantalla oscura por más de 15
segundos, configurable—. **Una entrevista de diez minutos con el micrófono
abierto no lo dispara nunca; un canal mudo con el operador ausente, sí.**

Y el regreso **no es una sorpresa**: la cuenta regresiva está visible todo el
tiempo, pasa a ámbar a 60 segundos y a rojo con tono en los últimos 10.

*→ [`PRD.md` §9 paso 6](https://github.com/Saul-Punybz/antena787/blob/main/PRD.md#paso-6--alguien-toma-el-control-y-lo-suelta)*

### ¿Me obliga a cumplir con algo?

**No. El software nunca regaña.** No le dice a nadie que está en falta, **no
bloquea nada por cumplimiento**, y no da por sentado que el operador le debe
algo a alguien. **Así corren muchos canales locales en Puerto Rico y en el
mundo, y no pasa nada.**

Lo que hace es tener **listo lo que a veces alguien pide** —la bitácora de
alertas, el archivo de anuncios políticos, el registro del medidor de volumen—
activado con el perfil del país, corriendo solo por debajo, exportable el día
que llegue una carta. **No un deber: un seguro que no cuesta trabajo.** Y **se
puede apagar entero.**

Al lanzar hay dos perfiles: **`us-fcc`** e **`internet`**. `eu-ebu` e
`isdb-latam` vienen después, y **cada perfil necesita fuente citada y un
mantenedor que opere en esa jurisdicción**, o se marca experimental.

**Y hay algo que sí conviene saber:** el as-run **no es un requisito de la FCC**.
La FCC eliminó los registros de programación. El as-run existe para **facturar,
reponer un spot tapado y probarle a un anunciante que su anuncio salió** — que
es plata, no papeleo. **No se vende cumplimiento que no existe.**

*→ [`PRD.md` §12](https://github.com/Saul-Punybz/antena787/blob/main/PRD.md#12--internacional-y-cumplimiento) ·
[`COMPLIANCE.md`](https://github.com/Saul-Punybz/antena787/blob/main/COMPLIANCE.md) ·
[`docs/profiles/README.md`](https://github.com/Saul-Punybz/antena787/blob/main/docs/profiles/README.md)*

### ¿Puedo migrar mi Google Sheet o mi Excel?

**Sí, y está pensado para eso** — se pega directo desde Excel o Google Sheets.
Es una herramienta de migración: se usa una vez.

**El importador nunca rechaza la hoja entera.** Trae lo que sirve y lista **fila
por fila** lo que no pasó y por qué, en palabras claras. Sabe además que una hoja hecha
a mano usa fechas de calendario, así que para las filas de la madrugada corre las
fechas un día atrás —para que coincidan con el día de emisión— y **lo reporta,
en vez de hacerlo callado**. Y cuando una regla empieza justo al día siguiente
de que termina otra, en la misma franja, pregunta: *"¿Zoids releva a Magic
Knight?"*

Después de importar, **las horas vacías se ven y se llenan de un clic** con un
botón que crea la regla de diferido. **Es un botón, no un proyecto.**

*→ [`PRD.md` §13](https://github.com/Saul-Punybz/antena787/blob/main/PRD.md#13--las-pantallas)*

### ¿Puedo probarlo hoy?

**Puedes correr la Fase 0**, que no es el producto: es el experimento que decide
si el diseño del motor aguanta. Compila con Go 1.26, necesita ffmpeg, y la
corrida completa de 8 horas ocupa unos 48 GB de disco.

**Y si la corres, el resultado es una contribución de verdad** — sobre todo en
Windows y en hardware modesto, que es donde el proyecto necesita cifras medidas
en vez de estimadas.

*→ [Home](Home) ·
[`f0/README.md`](https://github.com/Saul-Punybz/antena787/blob/main/f0/README.md)*

### ¿Dónde reporto un fallo, o una vulnerabilidad?

**Un fallo o una duda:** un issue en el repositorio.
**Una vulnerabilidad: en privado, nunca en un issue público.** El procedimiento
está en
[`SECURITY.md`](https://github.com/Saul-Punybz/antena787/blob/main/SECURITY.md).
