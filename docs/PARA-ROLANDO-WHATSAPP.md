# Lo que hay que pedirle a Rolando, para mandar por WhatsApp

_11 de septiembre de 2026. Pensado para mandarse en bloques, no de una vez.
Cada pregunta trae opciones para que se conteste rápido, y «no sé» siempre
es una respuesta válida: lo que no sepa lo averigua su ingeniero, y saber
que no se sabe también sirve._

Prioridad: los bloques 1 y 2 desbloquean trabajo hoy. Del 3 al 5 pueden
esperar unos días. El 6 es rápido y cierra pendientes viejos.

---

## Mensaje 1 — el contexto

Rolando, buenas. Te pongo al día en corto y te pido unas cosas.

El sistema ya arma la parrilla solo, prepara el material, publica la guía y
—desde esta semana— **ya manda la señal con el formato que tu multiplexor
espera**. Lo probamos midiendo la señal de vuelta, no de palabra.

Ahora viene la parte de **conectarlo a tu equipo de verdad**, y ahí necesito
datos que solo tienes tú o tu ingeniero. Todo lo que te pregunto es para no
adivinar: cada cosa que adivinemos es una cosa que va a fallar el día que lo
prendamos en la torre.

Contéstame lo que sepas, en el orden que quieras, y lo que no sepas dilo
también. No tienes que contestarlo todo de una vez.

---

## Mensaje 2 — lo más importante: tu VLC

Lo que más me sirve, y con esto solo ya avanzo mucho:

**Mándame la configuración exacta con la que emites hoy con VLC.**

Puede venir de cualquiera de estas formas, la que te sea más fácil:

- **(a)** El acceso directo de VLC en el escritorio: clic derecho →
  Propiedades → copia lo que dice en «Destino». Eso trae toda la línea.
- **(b)** Si lo abres desde un archivo guardado (`.vlm` o `.xspf`), mándame
  ese archivo tal cual.
- **(c)** Si lo configuras a mano cada vez en la ventana de VLC (Medio →
  Emitir), tírame una foto de las pantallas donde pones el destino y el
  formato.
- **(d)** Si es un `.bat` o un script que alguien dejó hecho, mándamelo.

**Y la versión de VLC** (Ayuda → Acerca de).

> Por qué lo pido: mi compromiso contigo es que **Antena787 haga todo lo que
> hoy haces con VLC**. Con tu configuración delante puedo revisar opción por
> opción que no te falte nada antes de pedirte que lo apagues. Sin ella,
> estoy adivinando.

---

## Mensaje 3 — el multiplexor (el Technalogix TP1000)

Aquí van cinco preguntas. Si tu ingeniero las tiene más a mano, pásaselas.

**3.1 ¿Cómo recibe la señal el TP1000?**
- **(a)** A una dirección IP y un puerto fijos (por ejemplo `192.168.1.50:1234`)
- **(b)** Por multicast, a un grupo tipo `239.x.x.x`
- **(c)** No sé

Si es (a) o (b), **mándame la dirección y el puerto exactos**.

**3.2 ¿El TP1000 te exige unos números de PID y de programa concretos, o él
los reasigna?**
- **(a)** Los exige: son estos → ______
- **(b)** Él los reasigna, da igual lo que entre
- **(c)** No sé

**3.3 ¿El TP1000 arma la guía que el televidente ve en su televisor (lo que
en la jerga se llama PSIP), o eso lo pone otro equipo?**
- **(a)** Lo pone el TP1000
- **(b)** Lo pone otro equipo → ¿cuál? ______
- **(c)** No lo pone nadie / no lo sé

**3.4 ¿El TP1000 tiene página web o dirección IP de manejo?** Si sí, ¿cuál
de sus dos puertos de red usa para eso?

**3.5 ¿Hay manual del TP1000 por ahí?** Aunque sea en papel o una foto de
las páginas de configuración. Es un equipo del que casi no hay información
pública.

---

## Mensaje 4 — el equipo de alertas (el Sage)

**4.1 ¿Qué modelo de Sage es?** Está en la etiqueta del frente o en el menú
de la pantallita.
- **(a)** ENDEC 1822
- **(b)** ENDEC 3644
- **(c)** No sé / le mando foto del frente

> La diferencia importa: el 3644 se puede conectar por red, el 1822 solo por
> cable serial.

**4.2 ¿Hay algún puerto serial libre en el Sage** (de los de atrás, los que
parecen un conector de impresora vieja), o están todos ocupados? Si hay algo
conectado, ¿qué es (letrero LED, generador de caracteres, otra cosa)?

**4.3 El bloque verde de relés de atrás:** ¿qué cables tiene puestos hoy y a
dónde van? Una foto de cerca del bloque me sirve perfecto.

**4.4 ¿El Sage está conectado a la red** (tiene un cable de red puesto)? Si
sí, ¿sabes su dirección IP?

**4.5 Versión del software del Sage**, si aparece en el menú.

---

## Mensaje 5 — transmisor, amplificador y retorno

**5.1 El excitador RVR:** ¿me das el modelo completo de la placa (no solo
«Blue Digital Video») y si tiene dirección IP puesta?

**5.2 El amplificador:** buscando información no encontré ninguna marca que
se llame «ADR». ¿Me confirmas qué dice exactamente la placa? ¿Dice
«Adrenalin» en algún lado? Una foto de la etiqueta resuelve.

**5.3 ¿Alguno de los dos (excitador o amplificador) está conectado a la red
de la estación**, con IP, para poder leerle la potencia y la temperatura
desde la computadora?
- **(a)** Sí, los dos
- **(b)** Solo uno → ¿cuál? ______
- **(c)** Ninguno
- **(d)** No sé

**5.4 El retorno de aire:** hoy usas una tarjeta receptora de TV en la
computadora. ¿Qué marca y modelo es? (Está en el Administrador de
dispositivos de Windows, o en la caja si la guardaste.)

> Por qué: para que el sistema compruebe solo que lo que salió por la antena
> es lo que debía salir, necesita «verse» a sí mismo al aire. Si esa tarjeta
> no se puede leer por red, hay un aparatito de unos $110 (HDHomeRun) que lo
> resuelve limpio y se pone donde llegue bien tu señal. No hace falta
> decidirlo hoy.

---

## Mensaje 6 — tres cosas rápidas de papeleo

**6.1 ¿Hoy emites con subtítulos (closed captions)?**
- **(a)** Sí, vienen dentro de los archivos
- **(b)** Sí, los añade alguien
- **(c)** No
- **(d)** No sé

> Si no, tranquilo: una estación con ingresos por debajo de los tres millones
> al año está exenta y no hay que pedirle permiso a nadie. Solo necesito
> saberlo para que el sistema lo diga bien.

**6.2 Siendo Class A, ¿alguien lleva la cuenta de las horas de programación
infantil educativa** (lo que la FCC llama E/I, 156 horas al año)?
- **(a)** Sí, lo lleva ______
- **(b)** No lo lleva nadie
- **(c)** No sabía que aplicaba

> Sin regaño: el sistema ya puede marcar qué programas cuentan, y más
> adelante lleva la cuenta solo. Solo quiero saber de dónde partimos.

**6.3 ¿Quién es el licenciatario legal del canal** (el nombre que aparece en
la licencia de la FCC)? Es para que los reportes salgan a nombre correcto.

---

## Mensaje 7 — lo que necesito que veas tú

Tres cosas que solo se pueden dar por buenas si las miras tú. Son de diez
minutos cada una y te las puedo enseñar por pantalla compartida:

1. **Comparar una hora.** Yo te doy la parrilla que el sistema habría puesto
   para una hora, y tú me dices qué saliste de verdad esa hora con VLC.
   Con eso comprobamos que el sistema programa como tú programas.
2. **Ver que un archivo a medio preparar no se puede usar.** Que la pantalla
   te lo dice claro y no te deja mandarlo al aire por error.
3. **El relevo.** Cuando un programa se acaba y otro lo sustituye en la
   misma hora, que el sistema te pregunte «¿este releva a aquel?» y lo
   entienda bien.

---

## Cierre

Con lo del **mensaje 2** solo (tu configuración de VLC) ya puedo avanzar
bastante. Lo demás lo vamos sacando.

Y si algo de esto te parece mucho lío, dime y lo vemos juntos en una llamada
de veinte minutos con el equipo delante. Probablemente salga más rápido así.

---

## Contestado por Rolando · bloque 3 (TP1000) · 11 sept 2026

Sus palabras, resumidas, y qué decide cada una. **Esto ya no se pregunta.**

**3.1 ¿A dónde le mandas la señal?** → **Multicast.** «Los IPs y puertos son
asignados por mí.»
*Decide:* multicast es **el** camino, no una opción. La decisión 3 de
`docs/f2/PLAN-F2.md` (unicast por defecto, multicast como opción del mismo
driver) queda **al revés** y se corrige. El driver ya lo hace bien —detecta el
grupo por la dirección y le pone TTL 1, que es lo correcto para un equipo en el
mismo switch—, así que no hay código que cambiar: hay texto que corregir.

**3.2 ¿Exige PIDs y número de programa?** → **No. Él los reasigna.** «Le puedes
editar, pero prácticamente ese es su trabajo, y añadirle los números de los
canales virtuales.»
*Decide:* **se cae el bloqueo que llevaba semanas escrito.** Todo lo que decía
«los PIDs, el programa y el tsid son valores de ejemplo hasta tener la cadena de
Rolando» deja de importar para CAtv: el multiplexor los reasigna. Los campos
siguen existiendo —otra estación sí los va a exigir (PRD §10: nunca fijos)— pero
en la pantalla tienen que decir que este multiplexor los reasigna, para que
nadie los rellene por miedo. Ya son opcionales en el formulario.
*Y un dato nuevo:* el canal virtual (40.1 y los suyos) **lo pone el TP1000**, no
nosotros.

**3.3 ¿Quién arma el PSIP?** → **El TP1000, pero lo pone en blanco.** «Al no
tener contenido desde el multicast lo pone en blanco.»
*Decide:* es el hallazgo más valioso de las cinco. **Hoy el televidente de CAtv
ve la guía vacía en su televisor**, y no es culpa del TP1000: es que nadie le
manda con qué llenarla. Si el TS que sale lleva el nombre del programa, el
TP1000 tendría de dónde sacarla. **Eso es algo que VLC nunca le dio.** Falta
saber qué lee exactamente el equipo —tabla del servicio, EIT, otra cosa—, y eso
sale del manual (3.5). Hasta leerlo, no se escribe nada: invariante 3.

**3.4 ¿Tiene IP de manejo?** → **Sí, se maneja por web.** Dos puertos de
ethernet: uno recibe los videos, otro es el de manejo. Las dos IPs se asignan al
configurarlo.
*Decide:* **hueco nuevo, y no estaba en ninguna lista.** Si el PC de la torre
tiene más de una tarjeta de red, el sistema operativo escoge por cuál sale el
multicast según su tabla de rutas, y escoge mal a menudo. Hay que poder decir
**por cuál tarjeta sale** (`localaddr=` en la URL de ffmpeg). Hoy no existe: cero
apariciones en todo el repositorio.

**3.5 ¿Tienes el manual?** → **Sí, lo mandó por WhatsApp.** Pendiente de meterlo
en `docs/equipos/` y leerlo antes de escribir una línea de PSIP.

### Lo que dice el manual del TP1000 · leído el 11 sept 2026

Manual de 90 páginas que mandó Rolando (`~/Downloads/117ce1.pdf`, de
manualslib). **No se guarda en este repositorio**: es material con derechos de
autor de un tercero y el repositorio es público.

Lo que dice, con su sitio:

1. **Es un multiplexor DVB, no ATSC.** El menú se llama literalmente «SI
   Setting **(DVB)**» y edita NIT, SDT y BAT e inserta LCN (línea 873). En las
   90 páginas, **«PSIP» aparece 0 veces**; «TVCT» y «VCT», 0. El LCN que
   describe (línea 2849) es el canal virtual **de DVB**, no el `40.1` de ATSC.
2. **Program Name y Provider Name se escriben dentro del equipo**, programa por
   programa, desde su web (líneas 1875, 1974, 2069). **No llegan desde la
   señal que le entra.**
3. **`BypassTS`** pasa el TS entero sin cambiar nada (línea 821), pero entonces
   no se reparten programas uno a uno.
4. La entrada IP acepta **UDP o RTP** (línea 1406), y **la versión de IGMP tiene
   que coincidir con la del switch** (línea 1376) — si no coinciden, el
   multicast no llega y no hay nada en el software que lo explique.

**Qué le hace esto al «PSIP en blanco» de la respuesta 3.3.** Son dos cosas
distintas y conviene no mezclarlas:

- **El nombre del canal** que enseña el televisor sale del campo *Program Name*
  del TP1000. Si está vacío, **se llena en su web en cinco minutos, sin
  nosotros y sin código.**
- **La guía de verdad** —qué dan ahora, qué sigue— necesita EIT, y **en el
  manual no hay ni una función que genere EIT**: solo sale en el glosario
  (línea 3049) y en un visor de tablas (línea 818).

**Lo que no se sabe, y no se va a suponer.** En ATSC el PSIP es obligatorio por
la FCC. Si este equipo es DVB y no genera PSIP, el PSIP de CAtv lo pone otro
equipo —el excitador RVR es el candidato— o no se está poniendo. Puede también
que su unidad lleve otro firmware, o que lo que él llama PSIP sea el SDT de DVB.
**Se confirma con el equipo delante** (invariante 3): no se escribe una línea de
PSIP hasta saberlo. Las preguntas están en el bloque 8.

#### Corrección de Saul, el mismo día: DVB por dentro, ATSC por fuera

El apartado de arriba planteó mal la contradicción. **Las dos cosas son ciertas
a la vez:** CAtv corre por dentro como una cabecera de cable —DVB— y **la
conversión a ATSC pasa después**, camino del transmisor. El estándar de emisión
es ATSC y el equipo lo soporta; lo que el manual describe es la parte de dentro.

Con eso, cada tabla tiene dueño y no hay misterio:

| Qué | Quién lo hace |
|---|---|
| El Transport Stream MPEG-2 | **Antena787.** Ya está, y medido |
| PIDs, programa, tsid | **El TP1000**, los reasigna |
| SDT y nombre del servicio (DVB) | **El TP1000**, campo *Program Name*, a mano |
| **PSIP, TVCT y EIT (ATSC)** | **Quien convierte a ATSC** — el excitador RVR es el candidato |

**Lo que esto rescata.** `internal/resolver/pmcp.go` y `/guia.pmcp` generan la
guía en **PMCP (ATSC A/76)**, que es el formato con el que se le habla a un
generador de PSIP. Hasta ahora eso estaba construido **sin destino conocido**.
Ahora tiene uno probable: **si el equipo que hace el PSIP acepta PMCP por red,
la guía de Antena787 entra directa** y la guía en blanco se acaba sin que nadie
escriba nada a mano, día tras día.

**La única pregunta que queda, y decide trabajo:** qué equipo arma el PSIP, y si
tiene entrada de red para la guía (PMCP / «PSIP data input» / «EPG input»). Con
el modelo exacto basta para averiguarlo sin molestar más a Rolando.
