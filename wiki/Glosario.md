# Glosario

Estos son los términos del dominio de Antena787. **El código, los textos de la
interfaz y la documentación usan estas palabras y ninguna otra.**

La fuente es
[`CONTEXT.md`](https://github.com/Saul-Punybz/antena787/blob/main/CONTEXT.md),
que está en inglés porque el código lo está. Esta página es su traducción de
consulta: **si algo no coincide, manda `CONTEXT.md`.** Allí está además, para
cada término, la lista de las palabras que **no** se usan (*Avoid*) y, cuando
existe, cómo lo llama la industria (*Industry says*) — dos cosas que esta tabla
no repite.

---

## Programación

| En inglés | En español | Qué es |
|---|---|---|
| **Rule** | **Regla** | La intención permanente de una persona — un título, un patrón de días, una hora, y una fecha de inicio y una de fin. Las reglas son lo que se edita; la parrilla se deriva de ellas. Las fechas no son un sistema de derechos: son dos campos que el programador ya mantiene, de los que salen los avisos de vencimiento |
| **Plan Item** | **Elemento del plan** *(`plan_item`)* | Una emisión resuelta a instante exacto, que nombra un archivo concreto y su duración medida. Se resuelve de las reglas 48 horas por adelantado |
| **As-run** | **As-run** | Un elemento del plan después de salir al aire, con las horas en que de verdad ocurrió. **No es un registro aparte: es la misma fila en un estado posterior** |
| **Broadcast Day** | **Día de emisión** | El día de programación, que empieza a una hora configurada —6:00 AM por defecto— y no a medianoche. Algo que sale a las 12:30 AM del martes pertenece al día de emisión del lunes; la fecha de fin de una regla incluye ese día de emisión completo |
| **Slot** | **Slot** | Una posición nominal en la parrilla, normalmente de 30 minutos. La duración real de un elemento del plan casi siempre es menor, y la diferencia es un **hueco**; cuando sería mayor, es un **sobrecupo** |
| **Overrun** | **Sobrecupo** | Contenido que pasaría del siguiente inicio duro. **Nunca se arranca:** manda el reloj, el resolver lo deja fuera, rellena el resto y lo dice. Nada se corta por el medio para que quepa |
| **Gap** | **Hueco** | Tiempo de aire sin ningún elemento del plan. Desde los segundos que sobran dentro de un slot hasta horas de noche sin programar |
| **Run** | **Corrida** | Un disparo de una regla, que puede emitir varios episodios seguidos |
| **Episodes per Run** | **Episodios por corrida** | Cuántos episodios seguidos emite una corrida, avanzando en la serie y recordando dónde se quedó |
| **Handoff** | **Relevo** | Una regla que termina en una franja horaria y otra que la toma al día siguiente. **Un relevo no es un conflicto** |
| **Encore** | **Repetición** *(`repite_a`)* | Una segunda emisión, bajo su propia regla, del mismo episodio que su regla primaria puso ese día de emisión. **No lleva contador propio**; si la primaria no emitió nada ese día, la repetición toma el siguiente episodio y avanza el contador compartido, así que la serie nunca se estanca |
| **Deck** | **Deck** | Una de las colas paralelas de un canal —manual, comercial, programa o relleno, en ese orden de prioridad—. En cada instante el aire lo tiene el deck de mayor prioridad que tenga algo que poner: programa va por secuencia, comercial va por reloj, relleno va cuando ningún otro tiene nada |
| **Manual Hold** | **Retención manual** | El estado en que un operador ha tomado el aire. Mientras dura, el deck manual es dueño del aire incluso entre las cosas que el operador dispara. Termina cuando lo suelta, cuando termina el bloque en que lo tomó, cuando la salida no ha llevado señal —silencio o negro— más allá de un límite configurado (y entonces el sistema lo suelta solo y levanta alarma), o cuando el propio sistema se cayó, que se registra como tal. **Inactivo nunca significa que el operador dejó de tocar cosas: significa que no está saliendo nada.** Una persona lo tiene a la vez; otra puede quitárselo, por nombre |
| **Daypart** | **Franja** | Un tramo con nombre del día de emisión bajo el que se pueden agrupar reglas — *"mañanas entre semana"* |

## Contenido

| En inglés | En español | Qué es |
|---|---|---|
| **Asset** | **Archivo** *(`media_asset`)* | Un archivo de medios más todo lo medido sobre él: duración real, volumen, subtítulos, negro de cabeza y de cola |
| **Title** | **Título** | La obra — una serie, una película, una promo, un ID de estación o un spot. Un archivo es una copia de parte de un título |
| **House Format** | **Formato de casa** | La codificación única a la que se convierte todo archivo al entrar: un códec, una resolución, una tasa de cuadros, GOP cerrados, una configuración de audio, un objetivo de volumen. Se configura por canal |
| **Normalization** | **Normalización** | Convertir un archivo al formato de casa. Ocurre **una sola vez**, al ingerirlo, en segundo plano |
| **Conform** | **Conformado** | Las correcciones por clip que se aplican **al reproducir**: rellenar con silencio el audio corto, sostener el último cuadro del video corto, poner pillarbox a la geometría equivocada. Distinto de la normalización: la normalización ocurre una vez sobre el archivo, el conformado ocurre cada vez que suena |
| **Filler** | **Relleno** | Archivos cortos —promos, IDs de estación, cortinillas— que el resolver usa para llenar un hueco de forma exacta |
| **Slate** | **Cartel** | Una tarjeta fija, generada en la instalación a partir del nombre y la comunidad de la estación, que sale al aire cuando no hay nada más que poner. Es el último escalón de la cascada de respaldo: **lleva la identificación de la estación, así que la cascada nunca termina en barras peladas.** Las barras y el tono existen solo para la prueba de instalación |
| **Quarantine** | **Cuarentena** | A donde va un archivo cuando el ingest le encuentra un problema, o cuando falla dos veces al aire separadas por más de cinco minutos. **Un archivo en cuarentena no llega nunca al aire**, salvo que una persona lo deje pasar bajo su propio nombre, lo cual queda registrado |

## Vivo

| En inglés | En español | Qué es |
|---|---|---|
| **Live Source** | **Fuente en vivo** | Un elemento del plan alimentado por una señal entrante en vez de por un archivo. Reserva su hora; si la señal sigue ausente pasado un breve margen de gracia, sale relleno y suena una alarma, y cuando la señal vuelve dentro del tiempo reservado el aire regresa a ella en el siguiente borde de clip de relleno, un minuto como mucho. **Su fin es un inicio duro para lo que siga.** Se conforma como cualquier archivo, y puede traer solo audio — entonces el video es el cartel del programa, que dibuja el sistema |

## Publicidad

| En inglés | En español | Qué es |
|---|---|---|
| **Advertiser** | **Anunciante** | Quien compra tiempo al aire |
| **Insertion Order** | **Orden de inserción** | La compra de un anunciante: cuántos spots de qué duración, en qué fechas, en qué franjas |
| **Spot** | **Spot** | Un archivo comercial, vendido por duración — 15, 30, 45 o 60 segundos |
| **Break** | **Corte** | Un tramo de aire programado, **a una hora de reloj**, en el que se insertan uno o más spots. Dentro de un programa de archivo pausa el programa, que reanuda después; dentro de una fuente en vivo reemplaza la señal, que **no se detiene**, así que tiene que alinearse con el reloj de cortes de la propia fuente |
| **Avail** | **Avail** | Los segundos vendibles dentro de un corte. **Un corte tiene una posición; un avail tiene un precio** |
| **Load** | **Carga** | Cuántos minutos por hora de avails permite el canal. Se configura por canal; doce por defecto |
| **Spot Airing** | **Emisión de spot** | El registro de que un spot salió en un instante, reconciliado contra los eventos de alerta de emergencia. Es la unidad que cuenta la evidencia de emisión |
| **Proof of Play** | **Evidencia de emisión** | El reporte que se le entrega a un anunciante mostrando qué emisiones de spot ocurrieron. **Se cuenta de registros, nunca se compone** |
| **Make-good** | **Reposición** | Una emisión de spot reprogramada porque un evento de alerta de emergencia o cualquier otro incidente tapó la original. **La propone el sistema, la confirma una persona, y nunca se factura dos veces** |
| **Classified** | **Clasificado** | Una línea de texto pagada que corre en el crawl. **Siempre lleva quién la pagó**, y solo sale al aire después de que una persona la aprobó |
| **Approval Queue** | **Cola de aprobación** | Donde espera cualquier cosa comprada por el portal —un spot o un clasificado— a que una persona la vea antes de su primer aire. **Las revisiones técnicas no son aprobación** |

## Emergencia y cumplimiento

| En inglés | En español | Qué es |
|---|---|---|
| **Emergency Alert Event** | **Evento de alerta de emergencia** *(`alert_event`)* | Un período en el que hardware certificado aguas abajo reemplazó la salida del canal. El playout sigue corriendo y **nunca se entera directamente**, así que el evento se registra desde un driver de alerta —o a mano, cuando ningún driver pudo verlo— y se reconcilia contra el as-run. Se guarda al menos dos años |
| **Incident** | **Incidente** | Cualquier cosa que el sistema tuvo que hacer solo para proteger el aire, registrada con su hora: un reinicio del encoder, una fuente en vivo que no llegó, una retención manual soltada por tiempo, una caída al cartel, un salto de reloj, un apagón, una cascada que duró más de unos minutos. **La cuenta de incidentes es una métrica de éxito, así que cada uno se anota** |
| **Time-shift** | **Diferido** | Una regla que reemite, más tarde, los programas que salieron en una ventana anterior del mismo día de emisión. **Reprograma los mismos archivos como elementos nuevos del plan; no reproduce la grabación**, salvo el tramo que fue una fuente en vivo. Los cortes de la ventana reemitida son avails nuevos, nunca copias de los originales. Una ventana de origen sin programa no produce nada |
| **Preempted** | **Tapado** *(`preempted`)* | Estado de un elemento del plan que quedó cubierto por un evento de alerta de emergencia. **Distinto de "saltado"**, que significa que el sistema decidió no ponerlo |
| **Compliance Profile** | **Perfil de cumplimiento** | Las reglas del país donde transmite el canal — objetivo de volumen, formato de subtítulos, registros. **Se escoge por país, nunca se supone, y se ofrece en vez de imponerse:** mantiene los registros listos para quien los pida, y **nunca regaña**. El as-run no es uno de esos registros: es evidencia comercial |

## Salida

| En inglés | En español | Qué es |
|---|---|---|
| **Channel** | **Canal** | Una señal programada, con su propio formato de casa, sus salidas, su perfil de cumplimiento y su parrilla. **Un proceso de playout sirve un canal** |
| **Output** | **Salida** | Un destino al que se manda la señal de un canal. Un canal puede tener varias a la vez, cada una con su propio objetivo de volumen, su estado de conexión y su política de reconexión |
| **Output Driver** | **Driver de salida** | La implementación detrás de una salida: cómo llega la señal a una clase de destino. **Vocabulario interno; nunca se le muestra a un usuario** |
| **Return Feed** | **Retorno de aire** *(`capture_input`)* | La señal transmitida devuelta al sistema, capturada **después** del equipo de alertas de emergencia. **El único sitio donde se puede observar lo que de verdad salió al aire**: la grabación y la comparación contra el plan leen de aquí, nunca de la salida propia del canal. Sin él, la detección de alertas queda degradada y los eventos se marcan a mano. Es también lo que ve el monitor en un teléfono |
| **Alert Driver** | **Driver de alerta** | La implementación que se entera de cuándo ocurrió un evento de alerta de emergencia: por un cierre de contacto, por una interfaz de red, o comparando el retorno de aire contra el plan. **Vocabulario interno; nunca se le muestra a un usuario** |
| **Break Marker** | **Marca de corte** | La señal que se manda aguas abajo diciéndole a un encoder que empieza un corte, para que los sistemas de inserción de publicidad puedan actuar sobre ella |

---

*Fuente:
[`CONTEXT.md`](https://github.com/Saul-Punybz/antena787/blob/main/CONTEXT.md).
Cómo se usan estos términos por dentro:
[`docs/ARQUITECTURA.md`](https://github.com/Saul-Punybz/antena787/blob/main/docs/ARQUITECTURA.md).*
