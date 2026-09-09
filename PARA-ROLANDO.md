# Antena787 — para Rolando

**De:** Saul Gonzalez
**Para:** Rolando Rosa, Caribbean Advantage TV
**Fecha:** 8 de septiembre de 2026

Este documento no es para venderte nada. Es para que tomes decisiones. Al final hay doce preguntas que necesito que contestes — cada una en una frase — porque sin esas respuestas hay partes del proyecto que no puedo empezar a construir.

Todo lo que dice "medido" en este documento salió de tu propio Google Sheet, revisado tabla por tabla el 4 de septiembre. Todo lo que dice "estimado" es un cálculo mío, no un hecho verificado.

---

## 1. Qué es esto y por qué

Antena787 es un programa que hace lo que tú haces hoy a mano con el Sheet, VLC y tu servidor de streaming: decide qué sale al aire, lo saca al aire, vende y prueba la publicidad alrededor, y publica la guía electrónica. Es software libre — cualquiera lo usa gratis para siempre — y CAtv va a ser el primer canal real donde se prueba, con tu consentimiento.

Tú aceptaste ser el despliegue de referencia. Eso no significa que un lunes se apague VLC. Significa que durante meses el sistema va a correr al lado de lo que ya tienes, sin arriesgar tu aire, hasta que gane tu confianza paso por paso. Eso está detallado en la sección 5.

Por qué existe: montar un canal de televisión hoy cuesta más en software de traffic y playout que en el transmisor mismo. Ese software cuesta decenas o cientos de miles de dólares, y no existe una alternativa libre seria — se buscó, y no hay ni un competidor de código abierto en esa categoría. Tú resolviste ese hueco con un Sheet y un ser humano despierto. Funciona, pero tiene límites que ya se pueden medir.

---

## 2. Lo que la demo enseña que hoy es a mano

Lo que mandaste es una demostración de cómo corres el canal hoy — sabemos que está incompleta a propósito: no tiene los anuncios, ni las pausas, ni toda la programación. Así corren muchos canales locales en Puerto Rico y en el mundo, y funciona. Lo que la demo nos da es la lista exacta de las cosas que hoy haces tú a mano y que el sistema nuevo tiene que volver de un clic: **llenar el tiempo, cambiar el tiempo, interrumpir la programación, añadir, quitar.** Con números tuyos, medidos el 4 de septiembre de 2026:

1. **La guía electrónica no se genera sola.** El archivo XMLTV es todavía la plantilla de ejemplo (`sports1.channel`, béisbol inventado). Con el sistema, la guía sale del plan y se actualiza sola cada vez que cambias algo.

2. **Llenar el tiempo es trabajo.** 103 de 343 espacios de la semana están sin asignar en la demo — la madrugada de 1:00 a 6:00 AM los siete días, y unas 10 horas cada día del fin de semana. El sistema los enseña de un vistazo y los llena con un clic: relleno, un diferido de la mañana, o lo que tú decidas. Nunca sale negro.

3. **Los vencimientos se llevan de memoria.** 18 programas terminan en los próximos 60 días. El sistema avisa a 30, 14 y 7 días, en pantalla y por Telegram o WhatsApp, y deja de avisar en cuanto cargas el relevo.

4. **Una fecha cruzada pasa sin que nada avise.** *Hellsing* tiene fin antes que inicio y la hoja lo marca "✓ OK". El sistema no deja guardar eso, y al importar tu Sheet te lista fila por fila lo que no cuadró, sin rechazar el resto.

5. **Los anuncios no tienen dónde vivir.** Tienes capacidad para 12 minutos por hora las 24 horas — 518,400 segundos al mes — y hoy no hay dónde anotar un cliente. El sistema te da el inventario, el enlace por anunciante, el cobro y el reporte.

6. **RadioOnce Live! depende de que alguien esté mirando.** 15 horas semanales en vivo. El sistema espera la señal, pone relleno si no llega, y vuelve solo al vivo en cuanto aparece.

7. **Cambiar cualquier cosa es editar celdas.** 35 reglas arman la semana; una semana son 336 celdas. En el sistema, una regla es una tarjeta: la arrastras, cambias la hora, la quitas, y el resto se acomoda solo.

## 3. Qué va a hacer el software, pantalla por pantalla

Son 9 pantallas. Ninguna te va a pedir que sepas qué es un códec o un contenedor de video.

- **Al aire** — el tablero: qué está saliendo ahora mismo, qué sigue, y las alarmas activas. Aquí es donde ves de un vistazo si algo necesita tu atención.

- **En vivo** — donde se define y se vigila RadioOnce Live! (y cualquier otra fuente en vivo que tengas o agregues): cuánto dura, y qué debe salir al aire automáticamente si la señal no llega a su hora, con una alarma para que te enteres. Hoy esa lógica de respaldo vive en tu cabeza; aquí queda escrita.

- **Parrilla** — la línea de tiempo de la semana, dibujada de verdad: los bloques se ven del ancho que realmente duran, así que un hueco de 6 minutos se ve como un hueco de 6 minutos, no se esconde en una celda. Arrastras para mover algo puntual y el sistema pregunta si es solo por hoy o para siempre — como cualquier calendario que ya conoces. También hay una vista de cuadrícula, como tu Sheet de hoy, pero de solo lectura: para mirar o imprimir, no para editar.

- **Mes** — el mes completo de un vistazo, como la pestaña `Calendar` de tu Sheet.

- **Guía** — la guía electrónica (XMLTV), generada del plan real y revisada automáticamente antes de publicarse, para que el bug de `sports1.channel` no pueda volver a pasar.

- **Reglas** — donde escribes la intención, no la celda: "Kojak, lunes a viernes, 8:00 AM, del 9 de agosto al 20 de diciembre" es una tarjeta, no 110 celdas escritas a mano. Unas 35 tarjetas arman tu canal entero. El sistema entiende episodios corridos (como tu columna de "Duración" — que en realidad cuenta episodios seguidos, no minutos), relevos de franja entre una regla que termina y otra que empieza al otro día, y repeticiones del mismo programa dos veces en el día — las tres cosas que tu Sheet maneja hoy sin nombrarlas.

- **Biblioteca** — tu catálogo: los 118 títulos, con sus sinopsis, años y clasificaciones. Arrastras archivos o apuntas a una carpeta, y el sistema mide la duración real, corrige el volumen, revisa que traiga subtítulos si los tiene, y te avisa en español si algo está roto — nunca con un mensaje técnico. Lo que falla nunca llega al aire.

- **Anuncios** — clientes, órdenes de compra, cuánto tiempo vendible tienes por hora, y la prueba de que cada anuncio salió al aire. Es lo que activa tus 518,400 segundos hoy en cero.

- **Ajustes** — todo lo que se configura una vez: a dónde sale tu señal, el formato de tu canal, el perfil regulatorio de tu país, y el equipo conectado (transmisor, alertas de emergencia).

---

## 4. Qué cambia en tu día a día

| Hoy | Con Antena787 |
|---|---|
| Editas 336 celdas por semana a mano | Escribes o ajustas ~35 reglas; el sistema arma el resto |
| Un hueco se descubre mirando el Sheet | El sistema nunca te deja ver una parrilla vacía — propone relleno automático |
| Un vencimiento te agarra de sorpresa | Avisos automáticos a 30, 14 y 7 días antes |
| Un conflicto de fechas puede pasar sin que nadie lo note | El validador no te deja guardar algo lógicamente imposible |
| La guía se mantiene aparte, a mano | Tu guía se genera del plan real y se revisa antes de publicarse |
| RadioOnce Live! depende de que alguien esté mirando | Respaldo automático con alarma si la señal no llega |
| Tu inventario publicitario no existe como concepto | Clientes, órdenes y prueba de emisión, listos para vender |
| Las alarmas dependen de que estés despierto | WhatsApp, Telegram o correo — sobre todo a las 3 AM, que es cuando nadie está mirando |

---

## 5. Cómo entra al aire, sin poder tumbarte

Cuatro etapas, cada una más comprometida que la anterior. No se salta ninguna.

1. **Modo sombra.** Antena787 arma su propio plan, genera su propia guía y registra qué habría pasado — pero tú sigues emitiendo con VLC exactamente igual que hoy. Se compara lo que el sistema habría puesto contra lo que de verdad salió al aire. Riesgo cero para ti: nada de lo que hace el sistema en esta etapa toca tu transmisor.

2. **Salida paralela a archivo.** El sistema ya emite de verdad, pero a un archivo o a un stream privado, no a tu transmisor. Es la primera vez que su motor de playout corre en serio, con una semana de observación antes de seguir.

3. **Madrugadas primero.** Le entregamos al sistema exactamente el bloque que hoy es tu peor problema: la 1:00 a 6:00 AM, que hoy queda sin llenar los siete días. Si algo sale mal ahí, casi nadie lo nota — y si sale bien, resuelve de inmediato tu problema más grande.

4. **Aire completo**, con VLC listo como respaldo por si hace falta volver atrás.

---

## 6. Lo que necesitamos de ti

Cada pregunta la puedes contestar en una frase. Te explico por qué importa cada una y qué cambia según tu respuesta.

**1. ✅ Cada 15 minutos.** — *Programa de WRBM 89.3 FM, llega por IP y sale por TV de vez en cuando; que llega por IP y sale por TV de vez en cuando; el sistema le pone el cartel del programa como video.* Lo que falta es el reloj de cortes. Define qué tan automático puede ser el respaldo si la señal falla, y bloquea que se termine de construir el módulo de playout completo.

**2. ¿Cómo vive hoy tu catálogo — lo puedes exportar a un archivo, o solo existe adentro de un servidor Plex o Jellyfin que corres tú?** — Antena787 **no se va a conectar a tu Plex**: tener el canal dependiendo a diario de otro servidor es un punto de fallo que no vale la pena. La biblioteca vive adentro de Antena787 y saca las fichas de los archivos `.nfo` que quedan al lado de los videos, de las etiquetas embebidas, y de TheTVDB o TMDb cuando falta algo. Para tus 118 títulos, que ya tienen sinopsis y años, hacemos una **importación de una sola vez** desde donde estén hoy, para no perder ese trabajo. Solo necesito saber en qué formato están.

**3. ¿A qué hora consideras tú que empieza tu día de emisión?** — *El sistema lo deja poner a cualquier hora; 6:00 AM es solo el default.* Un programa de las 12:30 AM del martes, para quien programa, es "lunes en la noche". Tu respuesta define cómo se van a leer los patrones de días que ya tienes en el Sheet, y cómo se va a ver tu parrilla en pantalla.

**4. ✅ Class A, en 605 MHz.** — Es la pregunta de la que más cosas cuelgan. Decide si te aplica la fecha de ATSC 3.0 (julio de 2027; LPTV y Class A están exentos hoy), y qué registros te conviene tener listos por si alguien los pide: archivo de anuncios políticos, archivo público, subtítulos. Nada de eso se te impone — el sistema los prepara si activas el perfil de Estados Unidos, y ya.

**5. ✅ Technalogix TP1000** — 2 ASI de entrada, 2 de salida, 2 Ethernet (manejo y stream). *Acepta SCTE-104; el sistema deja elegir la señalización de una lista, igual que UDP o RTP.* Sobre SCTE-104: Si acepta, es la ruta estándar de la industria y tus cortes quedan listos para sistemas de inserción de terceros. Si no, la primera versión vende los cortes localmente igual — los anuncios salen, lo que no hay es esa conexión con terceros, que hoy no usas. Decide el alcance de la fase de publicidad, no si ocurre.

**6. ¿Qué protocolo o interfaz de entrada usa ese equipo hoy?** — *Ya contestaste: UDP y RTP.* De eso depende cómo Antena787 le entrega la señal.

**7b. Tu VLC, tal cual.** — *Mándanos la cadena exacta con la que emites hoy: el `sout` de VLC, o el archivo `.vlm`/`.xspf` guardado, o el atajo de Windows con los parámetros, y la versión de VLC. Con eso comprobamos, opción por opción, que Antena787 hace todo lo que hoy haces con VLC (destino, multicast, PIDs, códecs, grabación) antes de pedirte que lo apagues.* También: ¿MistServer te empuja la señal o VLC tira de él? ¿Usas la ventana de VLC para ver la salida en el monitor de la torre? ¿Alguien más tira de tu señal por HTTP?

**7. ✅ No hace falta.** — *El sistema trae los dos modelos de Sage (1822 y 3644), DASDEC, Gorman-Redlich y TFT, y al instalar pregunta por dónde está conectado —serial, relés, red— y prueba cada uno. Con el bloque verde de relés ya alcanza.* Si tu ingeniero sabe el modelo, mejor, pero no lo esperamos. El 1822 se habla por el puerto serial del frente y por relés; el 3644 también por red. Eso decide cuál pieza de conexión ("driver") hay que escribir primero para que el sistema sepa cuándo tu equipo interrumpió el aire.

**8. ✅ 720p a 59.94.** — Define el formato único al que se convierte todo al entrar.

**9. ✅ No hace falta.** — *Los subtítulos embebidos siempre se conservan y siempre se pueden subir. El ajuste de obligación arranca en "no sé" y solo cambia si el sistema te avisa cuando algo sale sin subtítulos.* Lo pones cuando lo sepas.

**10. ✅ 3.5 TB: SSD M.2 de 500 GB + HDD de 3 TB.** — *Sistema y base al SSD; biblioteca y grabación al HDD. Caben 30 días de grabación.* Decide si tu disco de hoy alcanza para la biblioteca más la grabación continua.

**11. ✅ No hace falta.** — *El sistema prueba USB, SNMP, la página web del equipo y los contactos de estado; usa lo que responda.* Sobre lo que ya sabemos: *Ya sabemos que el excitador tiene entrada ASI y red de manejo pero no recibe IP de transporte. Ya sabemos que tu servidor tiene tarjeta receptora de TV y monitoreo por streaming: ese es el retorno de aire, y el sistema lo usa y lo enseña en pantalla.* Lo nuevo: tu amplificador ADR muestra potencia directa, reflejada, corriente y temperatura, y el excitador RVR tiene USB. Si el sistema puede leer eso, sabe físicamente si estás al aire y te avisa si la reflejada sube. Sobre el retorno de aire: es la única forma de que el sistema sepa cuándo tu equipo de alertas tapó un anuncio y proponga la reposición. Si no hay forma, funciona igual — solo que esas interrupciones las marcas tú a mano.

**12. ✅ Sí: WRBM 89.3 FM, Océano Radio.** — La parte de radio del software —que ya está pensada— se construye para ti, no para después.

Ninguna de estas doce preguntas bloquea que se empiece a construir. Las primeras dos fases del proyecto (la fundación y el modo sombra) se pueden construir sin ellas. Pero mientras más tarde en contestarlas, más tarde llegamos a que tu transmisor vea algo del sistema nuevo.

---

## 7. Lo que NO vamos a hacer

Dime si algo de esta lista es algo que tú sí necesitas — porque si es así, hay que discutirlo ahora, no después.

- No se construye el equipo de alertas de emergencia. Es hardware certificado; Antena787 se conecta a él, no lo reemplaza.
- No se construyen códecs de video ni audio — eso lo hace ffmpeg, una herramienta ya existente que el sistema usa por debajo.
- No se construye publicidad direccionable (un anuncio distinto por hogar o código postal). El sistema emite la señal estándar de la industria para que otros construyan eso encima si algún día hace falta.
- No se genera contenido para redes sociales ni se administran tus redes.
- No hay funciones de inteligencia artificial adentro del programa. Si algún día quieres conectar un asistente, es aparte y opcional, y nunca va a poder poner algo al aire por sí solo, facturar, o apagar una alarma de cumplimiento — esas puertas literalmente no van a existir en el programa.

---

## 8. Cuánto tiempo y en qué orden

Esto es un proyecto en construcción activa, no una promesa de entrega. Te doy el orden, y por qué es ese orden, y después un número honesto de tiempo.

| Etapa | Qué entrega | Toca tu aire? |
|---|---|---|
| **Fundación** | Catálogo con medición real, reglas, editor de parrilla, guía validada, avisos de vencimiento | No. Habilita el modo sombra (etapa 1 de la sección 5) |
| **Playout** | El motor completo, respaldo automático, tablero, alarmas — con 30 días seguidos de prueba contra un archivo antes de acercarse a un transmisor de verdad | Solo después de esa prueba de 30 días |
| **Endurecimiento** | Que tu Windows 10 aguante 24/7: antivirus, hora, actualizaciones sin reinicio | Sí — es lo que evita el reinicio a las 3 AM |
| **Público** | El proyecto se abre al mundo: instaladores, documentación, guía de compra de equipo | No directamente a ti |
| **Emisora** | Publicidad completa, cortes comerciales (SCTE-104), con su propia prueba de 48 a 72 horas antes de que dependa dinero de ella | Sí — es cuando tu inventario publicitario se vuelve vendible |
| **Internacional** | Perfiles de cumplimiento de otros países | No te afecta directamente |

Nada de esto se le entrega a tu transmisor sin haber pasado antes por una prueba larga contra un archivo. La antena es lo último, no lo primero.

**El número honesto.** Esto lo construye una persona a tiempo parcial con ayuda de IA, unas 10 horas por semana. A ese ritmo, y según lo que le ha tomado a proyectos parecidos pero más chicos: el modo sombra en tu canal está a cerca de **un año**; las madrugadas al aire, a **poco más de tres**; el canal completo, a **cerca de cuatro**; el primer anunciante cobrando por el portal, a **cinco y pico**. Puede ser menos si hay más horas; no va a ser semanas. Mientras tanto, VLC sigue siendo tu aire, y nada de lo que se construya te lo toca.

---

## 9. Los riesgos que te afectan a ti

- **El motor que saca la señal al aire lo escribe en buena parte una inteligencia artificial, con revisión humana línea por línea antes de tocar cualquier transmisor.** Es la parte de más riesgo de todo el proyecto, y por eso tiene la prueba de 30 días seguidos contra un archivo antes de acercarse a tu antena.
- **Si tu encoder no acepta SCTE-104 (pregunta 5), la primera versión vende los cortes localmente y ya.** La ruta alterna para meter esas marcas sin ayuda del encoder no tiene un caso documentado de funcionar 24/7 en ningún canal del mundo, y queda para después. No cambia nada de lo que tú vendes: los anuncios salen igual; lo que no hay es conexión con sistemas de inserción de terceros, que hoy no usas.
- **Windows reiniciándose solo a las 3 AM es, en la experiencia de otros, el riesgo operativo número uno de cualquier canal en Windows.** Con tu Windows 10 de hoy se resuelve fijando la política de actualizaciones, vigilándola todo el tiempo desde Ajustes, y con una alarma si pasa de todos modos.
- **El antivirus puede interferir con archivos de video en pleno aire.** El instalador configura las excepciones necesarias automáticamente.
- **Un reporte de anuncios con cifras inventadas sería un documento de negocio falso.** Por eso las cifras de cuántas veces salió un anuncio siempre se cuentan de un registro, nunca las redacta ni las calcula ningún asistente de IA.
- **Ninguna alarma de emergencia depende de este software para ser legal.** El cumplimiento de las alertas de emergencia sigue siendo tu equipo certificado; Antena787 ayuda a integrarlo, no lo certifica ni lo reemplaza.

---

Las doce preguntas de la sección 6 están contestadas o resueltas por el sistema. Si algo cambia, dímelo cuando puedas — no hace falta que sea el mismo día, pero mientras antes las tenga, antes avanza la parte que de verdad va a tocar tu aire.
