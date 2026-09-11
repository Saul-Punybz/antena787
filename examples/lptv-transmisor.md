# LPTV — televisora comunitaria con transmisor propio

## Quién es

Una televisora comunitaria con **licencia LPTV**. Sale al aire de verdad, por
antena, y la ve quien sintoniza el canal en su televisor.

La opera **una persona**, con un ingeniero que aparece cuando hay que subir a
la torre. La programación se lleva hoy en una **hoja de cálculo hecha a mano**:
cada semana alguien se sienta a llenar el tiempo, a cambiar el tiempo, a
interrumpir la programación cuando entra algo en vivo, y a añadir y quitar
títulos.

**Esa hoja de cálculo funciona.** Alguien resolvió su problema con lo que
tenía. Lo que pasa es que el trabajo que representa es todas las semanas, y
nadie más que esa persona sabe hacerlo.

## Qué tiene

**La cadena, de la máquina a la antena:**

```
PC ──UDP/RTP (Ethernet)──▶ multiplexor ──ASI──▶ excitador ──▶ amplificador ──▶ antena
                                ▲
                          ENDEC ─┘   (audio y video compuesto; relés atrás)
                            ▲
                   receptor de monitoreo (AM · FM · NOAA)
```

- **Una PC con Windows 10** en la torre, que hoy corre un servidor de streaming
  y un VLC. **SSD de 500 GB + HDD de 3 TB.** Sin presupuesto: sin edición IoT
  LTSC, y **sin segunda máquina**.
- Un **multiplexor** que recibe UDP por Ethernet y saca ASI, con dos puertos de
  red —uno de manejo y otro para el stream de entrada—.
- Un **excitador** con entrada ASI y red de manejo, que **no recibe transporte
  por IP**. Por eso `udp-ts` al multiplexor es la única puerta.
- Un **amplificador** de unos cientos de vatios, con su pantalla de potencia
  directa y reflejada, corriente, voltaje y temperatura.
- Un **ENDEC certificado**, con relés atrás y serial al frente.
- Una **tarjeta receptora de TV** en la misma máquina, para el retorno de aire.
- Unas horas semanales en vivo, que hoy se dan a mano.

## Qué configura

| | |
|---|---|
| **Perfil** | `us-fcc`, clase de licencia **LPTV**. |
| **Formato de casa** | `720p59.94`. Es lo que el multiplexor espera, y a 720p la máquina de presupuesto cero alcanza. |
| **Salidas** | La principal es **`udp-ts` en MPEG-2 CBR** hacia el multiplexor: tasa constante con paquetes nulos, PIDs y número de programa fijos que se escriben una vez, PCR frecuente, y audio AC-3 o MPEG capa II — **la prueba de barras decide cuál se queda**. Y si quiere, una segunda salida por internet en H.264, a −16 LUFS, del mismo material. |
| **Entrada en vivo** | `srt-listen` con contraseña, para lo que hoy se da a mano. El bloque reservado tiene margen de gracia; si la señal no llegó pasados **30 segundos**, entra relleno y suena la alarma, y cuando vuelve el aire regresa al vivo en el siguiente borde de clip. |
| **Alertas de emergencia** | El asistente pregunta **por dónde está conectado el ENDEC** —serial, relés, red— y prueba cada camino. Aquí los relés están cableados, así que se prueba eso primero. Si hay dos caminos, se usan los dos y se cruzan. |
| **Retorno de aire** | La tarjeta receptora, que captura la señal **después del ENDEC** — el único sitio donde se puede saber qué salió de verdad *(ADR 0009)*. De ahí sale también el monitor que se ve desde el teléfono. |
| **Telemetría del transmisor** | Solo lectura. El asistente pregunta qué cables hay —red de manejo, USB, contactos— y prueba cada uno. Potencia directa en cero es fuera del aire, diga lo que diga la red. |
| **Disco** | Sistema y base en el SSD; biblioteca y grabación en el HDD. Con la biblioteca cargada, quedan semanas de grabación a 720p. |
| **Importar la hoja** | El importador **nunca rechaza la hoja entera**: trae lo válido y lista fila por fila lo que no pasó y por qué, en palabras claras. Después, la parrilla enseña las horas vacías con un botón de un clic para llenarlas con diferido. |

**Lo que cambia el lunes:** llenar el tiempo, cambiar el tiempo, interrumpir la
programación, añadir y quitar — todo eso deja de ser el trabajo de la semana y
pasa a ser un clic. Y las madrugadas, que hoy son aire vacío, se llenan solas.

**Cómo entra al aire sin apagar nada un lunes:**

1. **Modo sombra** — el sistema arma el plan y registra qué *habría* puesto,
   mientras el aire sigue saliendo como siempre. Cero riesgo.
2. **Salida paralela a archivo** — emite de verdad, pero a un archivo. Una
   semana de mirar.
3. **Madrugadas primero** — hoy son aire vacío. Una falla ahí casi nadie la ve,
   y es donde el sistema aporta valor inmediato desde el primer día.
4. **Aire completo**, con la caja de respaldo lista.

## Qué NO necesita

- **Comprar máquina.** La que ya está en la torre.
- **Aceleración por hardware para la salida principal.** Un transmisor ATSC 1.0
  recibe **MPEG-2**, que ninguna tarjeta acelera y que a 720p59.94 cabe en un
  núcleo por software. La aceleración se gasta en decodificar la biblioteca y
  en la salida de internet, no en la del transmisor.
- **Archivo de anuncios políticos ni archivo público en línea.** **Un LPTV no
  los ve** — no como una opción apagada, sino como algo que no existe en su
  instalación. Eso es de Class A y potencia completa.
- **Un ENDEC nuevo.** Antena787 **no es un ENDEC y no lo reemplaza ni lo
  certifica.** Se integra con el que ya está.
- **Tarjeta SDI ni NDI.** Antena787 no tiene salida por ahí.
- **Cambiar de edición de Windows.** IoT Enterprise LTSC sería mejor, pero
  cuesta dinero. El instalador controla Windows Update **por política** y la
  pantalla de Ajustes lo vigila permanentemente.

**Y lo que hay que decir de frente:** sin segunda máquina, la cascada de
respaldo por software —programa → relleno → cartel— deja de ser una red de
seguridad y pasa a ser **la única**. El cartel lleva el identificativo y la
comunidad de licencia, así que una caída larga sigue identificando la estación.

## Qué fase lo cubre

| | |
|---|---|
| **F1** | Importar la hoja, la biblioteca medida, las reglas, el resolver, la guía, los avisos de vencimiento. Y el **modo sombra**: el sistema propone, la persona compara. |
| **F2** | El aire: motor, decks, **la salida `udp-ts`** —que es la primera que existe, precisamente por este caso—, fuentes en vivo, manual, grabación, diferido, cascada, incidentes. |
| **F2.5** | Que ese Windows 10 aguante 24/7, en paralelo al **soak de 30 días**. |
| **F4** | La publicidad: cortes vendidos, crawl de clasificados, portal del anunciante, señalización de cortes, y la **reconciliación con las alertas de emergencia** — que es lo que permite proponer una reposición cuando una alerta tapó un anuncio. Y el perfil `us-fcc`. |

**Recortar F4 no adelanta el aire.** La publicidad viene después del aire;
quitarla no mueve la fecha ni un día.
