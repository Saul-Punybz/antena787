# Universidad — un canal de práctica

## Quién es

El departamento de comunicaciones de una universidad. Hay un canal donde los
estudiantes producen: un noticiero, un programa de entrevistas, cápsulas de
clase, y la transmisión de los juegos.

**Quien lo administra es un profesor**, no un ingeniero, y **los estudiantes
rotan cada semestre**. Eso es lo que decide todo: lo que solo sabe hacer una
persona deja de funcionar en diciembre.

El canal sale **por internet**. No hay licencia ni transmisor.

## Qué tiene

- Una PC de oficina del departamento, con gráficos Intel recientes.
- Un estudio con cámaras y una mesa de switcher que ya se usa para las clases.
- Un disco con el archivo de producciones de los últimos años, en desorden.
- Internet institucional, que sube bien.

## Qué configura

| | |
|---|---|
| **Perfil** | `internet`. Sin obligaciones de radiodifusión; volumen −16 LUFS. |
| **Formato de casa** | `720p59.94`, H.264. Le sobra para la web y le baja el costo de máquina. |
| **Salidas** | Dos: una a la plataforma donde publica la universidad (RTMP o SRT), y una **a archivo**, que es la que se revisa en clase. Cada una con su propio volumen y su reconexión. |
| **Entrada en vivo** | `srt-listen` desde el switcher del estudio, **con contraseña**. Cada programa en vivo es una regla con su hora y su bloque reservado; el fin del bloque es duro. |
| **Fichas y carátulas** | Lo local primero: etiquetas y `.nfo`. Para lo producido en casa no hay ficha que buscar en internet, y no hace falta ninguna clave. |
| **Relleno** | Cápsulas cortas, identificativos y promos de la universidad. El asistente avisa si la biblioteca de relleno está vacía y ofrece crear uno de un clic. |
| **Diferido** | Lo que salió en la mañana se vuelve a poner de noche, con una regla. Es lo que llena el calendario sin producir más. |

**Lo que resuelve el problema del semestre:** las reglas son la intención del
humano —un título, un patrón de días, una hora, y una fecha de inicio y una de
fin—. **Un estudiante nuevo lee las reglas y entiende el canal en diez
minutos.** No hay una lista de reproducción que alguien tenga que mantener a
mano ni un archivo que solo el que se graduó sabía dónde estaba.

Y cuando un programa se acaba —porque el semestre se acaba— **el sistema avisa
a 30, 14 y 7 días** en vez de dejar un hueco el lunes.

## Qué NO necesita

- **Nada de la cadena de transmisión**: ni encoder, ni multiplexor, ni
  excitador, ni amplificador, ni antena.
- **ENDEC.** No hay obligación de alertas de emergencia sin licencia de
  radiodifusión.
- **Retorno de aire.** Su propia salida es su señal; no hay equipo aguas abajo
  que la reemplace.
- **Perfil de cumplimiento con obligaciones.** Nada de bitácora de alertas,
  archivo político, archivo público ni registro de volumen. `internet` no
  enciende ninguna, y **no hay que explicar por qué**.
- **Publicidad.** Ni portal, ni clasificados, ni cortes vendidos. No aparece en
  ninguna pantalla hasta que alguien registre un anunciante — y aquí no lo va a
  hacer nadie.
- **Roles ni usuarios.** Una clave de estación de cuatro a seis dígitos, que se
  pide una vez por navegador. Existe para que nadie que se conecte al wifi de
  la facultad saque el canal del aire.

## Qué fase lo cubre

| | |
|---|---|
| **F1** | Biblioteca medida, reglas, resolver, guía, avisos de vencimiento. Todo el trabajo de programación, sin tocar el aire. |
| **F2** | El aire de verdad: motor, salidas, **fuentes en vivo por SRT** —que aquí es la mitad del canal—, manual, grabación y diferido. |
| **F3** | El **modo internet completo**, que es el modo de esta universidad. Y el proyecto publicado, con instaladores y documentación bilingüe. |

**Este es exactamente el caso por el que F3 va antes que F4:** un canal
universitario no necesita publicidad — necesita que el proyecto exista y
funcione.
