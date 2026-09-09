# Guía de compra — qué equipo hace falta de verdad

Esta guía es para decidir en qué gastar y en qué no. Está escrita para quien
tiene una estación chica y un presupuesto que no da para equivocarse.

> **Todas las cifras de esta guía llevan la etiqueta *(estimado)* mientras no
> estén medidas.** Son estimaciones honestas, no mediciones de este proyecto.
> La Fase 0 las mide de verdad en hardware real y esta guía se corrige con los
> números que salgan (PRD §22.1, criterio F0-07). Hasta entonces, léelas como
> un orden de magnitud.

---

## 1 · La máquina

| Escenario | Máquina | Aprox. |
|---|---|---|
| **Presupuesto cero** | **La máquina que ya tienes.** | **$0** |
| Canal por internet, 720p | PC de oficina reciclado con gráficos Intel recientes | **$0** *(estimado)* |
| Canal 1080 con transmisor | Mini PC Intel N100 o N305, 16 GB | **~$200** *(estimado)* |
| 4 a 6 canales | Servidor de 8+ núcleos con GPU, 32 GB | **~$1,500** *(estimado)* |

### El caso de presupuesto cero: la PC que ya tienes

**No hay que comprar nada para empezar, y esto no es una concesión — es el
caso principal.**

El despliegue de referencia del proyecto, CAtv, corre así: un **Windows 10 que
ya estaba ahí**, sin comprar hardware. Emite a **720p** —no a 1080— y eso es
exactamente lo que da el margen. La máquina está en la torre y hoy corre
MistServer y VLC; Antena787 ocupa ese mismo lugar.

**Si funciona ahí, la tesis del proyecto se sostiene sola.**

Lo que sí hay que saber de un Windows 10 normal:

- **Windows 10/11 IoT Enterprise LTSC sería lo ideal** —sin actualizaciones de
  funcionalidad por diez años, diseñado para equipos dedicados— **pero cuesta
  dinero, y con presupuesto cero no es una opción.** No la compres solo por
  esto.
- Antena787 tiene que hacer que un Windows 10 normal aguante 24/7, y para eso
  el instalador **controla Windows Update por política** —diferimiento, sin
  reinicio automático, horas activas— en vez de exigirte cambiar de edición.
  Es menos robusto que LTSC, y hay que decirlo, pero es lo que hay. La pantalla
  de Ajustes lo vigila permanentemente y avisa si la política se desconfigura.
- **Windows Update reiniciando la máquina a las 3 AM es el riesgo operativo
  número uno** de una instalación en Windows. No se compra nada para
  resolverlo; se vigila.
- El instalador también aplica **exclusiones de antivirus** en las carpetas de
  medios. Escanear terabytes de video puede provocar errores de archivo en
  pleno aire.

### Dos máquinas idénticas

**Para una instalación con licencia de transmisión, dos máquinas idénticas son
lo recomendable.** Una emite y la otra está lista.

**Con presupuesto cero no existen, y hay que decir la consecuencia:** cuando no
hay segunda máquina, la cascada de respaldo por software —programa → relleno →
cartel— deja de ser una red de seguridad y pasa a ser **la única**. Eso sube el
listón de lo que el software tiene que aguantar solo, y por eso la prueba de
resistencia de 30 días existe.

Si el presupuesto aparece algún día, **la segunda máquina idéntica es la mejor
compra que puedes hacer**, por encima de cualquier mejora de la primera.

### La Raspberry Pi

**La recomendación de Raspberry Pi está retirada** hasta verificar si el Pi 5
tiene encoder H.264 por hardware. Sin él no se sostiene. No la compres para
esto todavía.

---

## 2 · Por qué la aceleración por hardware importa, y cuándo no

**Importa, y es estructural, no una conveniencia.** Con un encoder persistente,
recodificar 1080p cuesta:

| | Costo *(estimado)* |
|---|---|
| Con QuickSync en un N100 | **~15 % del CPU** |
| Por software, sin aceleración | **un núcleo completo** |

Sin aceleración se rompe el costo casi lineal por canal: la diferencia entre
un canal y cuatro deja de ser aritmética y pasa a ser una compra de servidor.

### La excepción, y favorece al caso chico

**Un transmisor ATSC 1.0 recibe MPEG-2, que ninguna tarjeta acelera y que no
hace falta acelerar.** A 720p59.94 cabe en un núcleo por software, y es
literalmente lo que VLC ya hace hoy en la PC de CAtv.

Dicho de otro modo: **si tu única salida es un multiplexor ATSC 1.0, la
aceleración por hardware no te compra la salida principal.** Donde sí se gasta
es en:

- **decodificar** la biblioteca, que está en H.264, y
- **las salidas de internet**, que también son H.264.

O sea: un canal que sale *solo* por el transmisor la necesita mucho menos que
uno que sale por transmisor **y** por internet a la vez. **La Fase 0 lo mide en
vez de suponerlo.**

---

## 3 · Disco: la regla es biblioteca + grabación

El disco no se compra por "cuántos terabytes suena bien". Se compra sumando
dos cosas:

1. **La biblioteca** — todo el contenido normalizado, que es lo que ocupa de
   verdad.
2. **La grabación de la salida** — cuántos días de lo que salió al aire
   quieres poder mirar hacia atrás.

**Y se separa en dos discos, con criterio:**

| Va en | Qué |
|---|---|
| **SSD** | El sistema operativo y la base de datos. |
| **HDD** | La biblioteca y la grabación de la salida. |

El SSD no tiene que ser grande; tiene que ser rápido y estable. El HDD es donde
va el volumen.

### El ejemplo de CAtv

**SSD M.2 de 500 GB + HDD de 3 TB = 3.5 TB.** Sistema y base en el SSD,
biblioteca y grabación en el HDD. Con **~1.2 TB de biblioteca** quedan
**30 días de grabación a 720p** *(estimado)*.

### El disco lleno nunca te saca del aire

Vale la pena saberlo antes de comprar de más. Los umbrales son configurables:

- Por debajo del **10 %** libre, suena la alarma.
- Por debajo del **5 %**, la grabación se purga sola hasta liberar espacio.
- Por debajo del **2 %**, la base deja de escribir el as-run —y lo dice en
  rojo— **pero el aire sigue**, saliendo del plan que ya está en memoria.

Se pierde el registro, nunca la señal.

---

## 4 · Lo que hay aguas abajo, y Antena787 NO compra por ti

Antena787 termina donde empieza tu cadena de transmisión. Todo lo de esta
lista es equipo que la estación tiene, compra o alquila por su cuenta. El
software **se integra** con ello; no lo reemplaza.

| Equipo | Qué hace | Lo que Antena787 hace con él |
|---|---|---|
| **Encoder / multiplexor** | Recibe la señal, la comprime si hace falta, junta varios programas y saca ASI a tasa fija. | Le entrega `udp-ts` en MPEG-2 CBR, con PIDs y número de programa fijos y PCR frecuente — exactamente lo que un multiplexor exige. |
| **Excitador** | Pone la señal en la frecuencia. | Nada directo; a veces se le lee telemetría por red o USB. |
| **Amplificador** | La potencia. | Lee su telemetría —potencia directa y reflejada, corriente, voltaje, temperatura— **solo de lectura**, y la muestra en *Al aire*. |
| **ENDEC** (equipo de alertas) | Interrumpe la señal con las alertas de emergencia. **Hardware certificado.** | Se entera de *cuándo* interrumpió —por relés, serial o red— y lo anota. **Ni lo reemplaza ni lo certifica** (ver `COMPLIANCE.md`). |
| **Retorno de aire** | Devuelve a la máquina la señal que de verdad salió, **después** del ENDEC. Una tarjeta receptora de TV, una entrada de captura, o un stream de monitoreo del transmisor. | Es de donde sale la verdad: la grabación, la verificación de lo que salió, y el monitor que se ve desde el teléfono *(ADR 0009)*. |

> **El retorno de aire es la compra pequeña con mejor retorno de todas.** Una
> tarjeta receptora de TV en la misma máquina es barata y es lo único que
> convierte "creo que salió" en "salió". Sin ella el sistema corre en modo
> degradado, lo dice, y las interrupciones se marcan a mano — funciona, pero
> verifica menos.

**Y lo que definitivamente no hace falta:** una tarjeta SDI o NDI. Antena787
no tiene salida por SDI ni por NDI —esos SDK obligan a enlazar C, que este
proyecto no hace *(ADR 0002)*— así que no la compres pensando en este software.

---

## 5 · Dos listas cortas

### Para un canal por internet

1. **Una PC.** La que tengas. Con gráficos Intel recientes, mejor.
2. **Disco** suficiente para tu biblioteca, más lo que quieras grabar.
3. **Internet de subida estable.** Es la parte que de verdad decide si el canal
   se ve bien.
4. Nada más.

**No hace falta:** encoder, multiplexor, excitador, amplificador, ENDEC,
retorno de aire, licencia, ni perfil de cumplimiento. El perfil `internet` no
enciende ninguna obligación de radiodifusión.

### Para un transmisor

1. **Una PC**, con las notas de arriba sobre Windows.
2. **Disco** con la regla de biblioteca + grabación.
3. **Encoder / multiplexor** — al que Antena787 le manda `udp-ts`. Este es el
   equipo cuyas exigencias hay que conocer: qué bitrate, qué PIDs, qué audio.
4. **Excitador y amplificador** — la cadena de RF.
5. **ENDEC certificado** — no es opcional y no es este software.
6. **Retorno de aire** — la compra pequeña que más cambia las cosas.
7. **La licencia**, que tampoco es este software.

**Antena787 solo reemplaza lo que hoy hace el playout** —en el caso de CAtv,
MistServer y VLC—. Del multiplexor en adelante nada cambia.

---

## 6 · Lo que no cuesta dinero

**El software es gratis y sin recortes.** Se baja, se instala y no se le debe
nada a nadie. Sin versión recortada, sin funciones que caducan, sin marca de
agua, sin tope de canales.

Lo que se paga, **a quien lo quiera**, es tener a quién llamar:

| | Precio |
|---|---|
| **Soporte inicial** — acompañamiento hasta estar al aire | **$1,200** |
| **Soporte continuo** — línea disponible, atención a fallos | **$100 / mes** |

Una estación que sale en negro a las 3 AM tiene un problema esa misma noche, y
a esa hora no hay a quién llamar. Eso es lo que se paga: no un programa, **que
haya alguien despierto del otro lado.**

---

## 7 · Cuándo vuelve a escribirse esta guía

Los números *(estimado)* de arriba se reemplazan por **medidos** cuando termine
la Fase 0 (PRD §22.1). Concretamente, el criterio **F0-07** mide el CPU y la
RAM con una salida y con dos, en el Windows 10 del despliegue de referencia o
una máquina equivalente — nunca en la laptop de un desarrollador. El **F0-08**
mide cuántas sesiones de encoder por hardware aguanta esa máquina.

**Hasta que eso pase, ninguna cifra de esta guía se publica como definitiva.**
