# Seguridad

## Cómo reportar una vulnerabilidad

**En privado, por GitHub Security Advisories.** En la pestaña **Security** de
este repositorio, **Report a vulnerability**. Eso abre un aviso privado que
solo ven quienes mantienen el proyecto, y permite discutir y arreglar la
falla antes de que se haga pública.

**No abras un issue público** para algo que pueda sacar a un canal del aire o
exponer datos de una estación. Un issue es visible desde el segundo cero.

Si no puedes usar Security Advisories, escribe a **saul9saga@gmail.com** y
di en el asunto que es un reporte de seguridad.

### Qué ayuda que traiga el reporte

- Qué versión o commit, y en qué sistema operativo.
- Qué hace falta para reproducirlo: pasos, configuración, archivos de prueba
  si los hay.
- Qué consigue un atacante con esto: ¿saca el canal del aire? ¿lee o cambia
  datos que no debería? ¿mete algo al aire?
- Si aplica, el `events.jsonl` o el registro de incidentes de la corrida.

No hace falta un exploit funcionando. Una descripción clara vale igual.

---

## Qué cubre esto

**El software de Antena787**: el motor, el servidor de cuadros, el servidor
web y su API, los drivers, la base de datos, el instalador y todo lo que
viva en este repositorio.

## Qué NO cubre

- **El equipo de alertas de emergencia.** Es hardware certificado de
  terceros, aguas abajo del sistema. Antena787 se integra con él y nunca lo
  reemplaza; si encuentras algo en ese equipo, va a su fabricante.
- **El transmisor, el multiplexor, el encoder de hardware y las tarjetas de
  captura.** Ídem: no son nuestros, y no podemos arreglarlos.
- **ffmpeg**, que es la única dependencia externa. Una falla de ffmpeg se
  reporta al proyecto ffmpeg; lo que sí nos toca —y sí queremos saber— es si
  la manera en que **nosotros** lo invocamos abre un hueco (rutas, argumentos,
  entradas sin validar).
- **La red, el sistema operativo y la configuración de la estación.** Lo que
  sí cubrimos es lo que el instalador toca por su cuenta.
- **Servicios de terceros** (APIs de fichas, plataformas de streaming,
  pasarelas de cobro).

---

## Estado del proyecto, y qué significa para esto

**Antena787 está en la Fase 0 y todavía no emite nada.** No hay release, no
hay instalador, no hay versión soportada: lo que existe es el experimento del
motor. Hasta que haya un primer release público **no hay versiones viejas que
parchear** — se arregla en `main` y ya.

Cuando existan releases, esta página dirá cuáles reciben arreglos.

## Tiempos de respuesta

Esto lo mantiene poca gente, así que los tiempos son los que se pueden
cumplir de verdad, no los que quedan bonitos:

| | |
|---|---|
| Acuse de recibo | dentro de **5 días laborables** |
| Primera evaluación (¿es real? ¿qué tan grave?) | dentro de **15 días laborables** |
| Arreglo o plan con fecha | según la gravedad, y se te dice cuál es |

Si no hay respuesta en ese plazo, insiste — es un olvido, no una política.

**Divulgación coordinada.** Se trabaja en privado hasta que haya arreglo, y
el aviso se publica con crédito a quien lo reportó, salvo que prefiera
quedarse anónimo. Si el arreglo se atrasa, se acuerda una fecha contigo; no
se pide silencio indefinido.

## Sin recompensas

**No hay programa de recompensas ni pagos por reportes.** Este es un proyecto
libre sostenido por servicios de soporte, no hay presupuesto para eso, y
prometerlo sería mentir. Lo que sí hay: crédito público en el aviso y en las
notas del release, y agradecimiento sincero.
