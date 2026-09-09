# ONG — un canal por internet, con lo que ya hay

## Quién es

Una organización sin fines de lucro que produce contenido —charlas, talleres,
testimonios, cobertura de sus actividades— y hoy lo sube suelto a redes cuando
alguien se acuerda.

Quiere **un canal**: algo que esté prendido siempre, con una programación que
la gente pueda seguir, y una guía que diga qué viene después.

**Una sola persona lo va a operar**, y esa persona tiene otras cinco cosas que
hacer. Sabe instalar software y nada más.

## Qué tiene

- **La PC de la oficina.** Windows 10, sin comprar nada.
- Un archivo de video propio en una carpeta, con nombres desordenados.
- Internet normal.
- **Presupuesto cercano a cero**, que aquí es el dato que manda.

## Qué configura

| | |
|---|---|
| **Perfil** | `internet`. Volumen −16 LUFS. **Ninguna función de cumplimiento se enciende.** |
| **Formato de casa** | `720p59.94`, H.264. En una PC de oficina reciclada con gráficos Intel recientes, esto es el caso de $0. |
| **Salidas** | Una o dos, a donde ya publica: RTMP a la plataforma de siempre, y HLS desde su propio servidor si algún día lo tiene. Cada salida se reconecta sola — un envío de 24/7 se cae siempre. |
| **Entrada en vivo** | Opcional. Si un día transmite un evento, la **entrada rápida** está siempre abierta y aparece en *Al aire* sin configurar nada. |
| **Fichas y carátulas** | Los dos por defecto, que **no piden clave**: Cover Art Archive y TVmaze. Pedirle a esta persona que se registre en un servicio y pegue una API key en algún lado es justo el paso que la hace abandonar la instalación. |
| **Relleno** | Lo primero que hay que resolver, y el asistente lo dice: una biblioteca de relleno vacía significa que el primer hueco sale en negro, y eso no se puede descubrir a las 3 AM. Ofrece **crear uno por defecto de un clic** —el cartel de la organización con una cama musical— y se cambia después. |
| **Diferido** | Una regla que vuelve a poner de noche lo del día. **Con poco contenido, es lo que hace que el canal parezca un canal.** |
| **Respaldo** | A un segundo disco o a un USB. La nube es una opción, no el camino por defecto. |

**Lo que se lleva de aquí:** el canal deja de depender de que alguien se
acuerde. Las reglas dicen qué va y cuándo, el resolver lo materializa 48 horas
por adelantado, el relleno tapa los huecos exactos, y si un archivo falla el
sistema cae a relleno **sin negro** y lo deja anotado.

## Qué NO necesita

- **Comprar hardware.** Nada. Este es literalmente el escenario de presupuesto
  cero de [`COMPRAR.md`](../COMPRAR.md).
- **Aceleración por hardware**, estrictamente. Ayuda, y si la PC la tiene se
  usa. Pero un canal 720p por internet no la exige.
- **Toda la cadena de transmisión**: encoder, multiplexor, excitador,
  amplificador, ENDEC, antena, licencia.
- **Retorno de aire.** No hay equipo aguas abajo que reemplace la señal.
- **Cumplimiento.** Ni bitácora de alertas, ni archivo político, ni registro de
  volumen, ni archivo público. **Un canal por internet no enciende ninguna de
  estas funciones, y no tiene que explicar por qué.**
- **Publicidad.** No aparece hasta que alguien registre un anunciante. Si algún
  día la organización vende patrocinios, está ahí y es libre como todo lo demás.
- **Alguien a quien llamar.** El soporte se paga solo si se quiere; el software
  se baja, se instala y no se le debe nada a nadie.

## Qué fase lo cubre

| | |
|---|---|
| **F1** | Ingest con medición real, reglas, resolver, guía. Ya se puede armar la programación entera. |
| **F2** | El aire: motor, salida por internet, relleno, cascada de respaldo, diferido, tablero. |
| **F2.5** | **Que ese Windows 10 aguante 24/7** — exclusiones de antivirus, NTP, política de Windows Update. Sin esta fase, una PC de oficina emitiendo un año no es realista. |
| **F3** | El **modo internet completo** y el proyecto publicado con instalador. Aquí es donde esta organización puede bajarlo e instalarlo sola. |

**Y una advertencia honesta:** F3 llega **después** de que el aire funcione, y
el aire está a años de distancia al ritmo real del proyecto. El calendario
completo, con sus tres escenarios, está en [`docs/ROADMAP.md`](../docs/ROADMAP.md).
