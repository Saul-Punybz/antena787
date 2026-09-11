// Package sage lee —y solo lee— lo que un Sage Digital ENDEC ya dijo.
//
// # Por qué existe y qué no hace
//
// El ADR 0010 fija la regla: Antena787 **no implementa el sistema de alertas,
// integra el ENDEC certificado que la estación ya tiene**. En Estados Unidos la
// obligación es de hardware certificado (47 CFR parte 11) y ningún programa
// corriendo en el PC de playout la cumple. Así que este paquete decodifica el
// estado que el ENDEC publica por su cuenta y nada más:
//
//   - **nunca genera una cabecera SAME**, ni un `NNNN`, ni la señal de
//     atención de 853+960 Hz, ni ningún tono. No hay una sola función en este
//     paquete que escriba al ENDEC ni que produzca audio.
//   - **nunca decide**. Un Evento no interrumpe nada: lo entrega por un canal
//     y quien decide es el motor (tanda T8 de docs/f2/PLAN-F2.md), que es el
//     que sabe si hay un corte comercial a medias y qué dice el ADR 0010 sobre
//     absorber el tiempo perdido en el programa.
//   - **nunca miente** (regla 3 del contrato de drivers,
//     docs/drivers/README.md). Lo que el protocolo no dice, este paquete no lo
//     inventa; abajo está la lista de lo que no se puede saber por aquí.
//
// # Los dos caminos, un solo protocolo
//
// El 1822 tiene seis puertos serial DB-9 y ninguna red. El 3644 es
// «drop-in replacement» del 1822 —mismo protocolo serial— y añade una
// **interfaz de automatización por TCP** que sirve ese mismo protocolo por red
// en un puerto configurable (`MENU.NETWORK.PORT BASE`,
// `MENU.NETWORK.AUTOMATION`), apagada de fábrica. Por eso Conectar acepta o un
// puerto serial o una dirección TCP y el analizador es el mismo: lo que cambia
// es el caño, no lo que viene por dentro (docs/drivers/catalogo/01-alertas-eas.md).
//
// # Qué manda el ENDEC por ahí
//
// Un puerto del ENDEC se configura con un «device type». Los que importan:
//
//   - **DECODER** (`MENU.DEVICES.PORT.DEVICE TYPE.DECODER`) — el estado del
//     decodificador, y **el que hay que pedir**. Formato del manual §8.1:
//
//     <tipo>:<cadena zczc>
//     <texto expandido>
//
//     con cuatro tipos: `local:` (el ENDEC **está enviando** la alerta),
//     `match:` (la oyó y coincide con un filtro de entrada), `nomatch:` (la
//     oyó y no coincide con ninguno), `dup:` (ya la había oído).
//
//   - **ENCODER** (§8.1) — los bytes crudos de la parte 11 repetidos en async:
//     tres cabeceras y tres `NNNN`, cada repetición con dieciséis bytes de
//     sincronismo 0xAB delante (en un terminal se ven como `+`, pero **no son
//     un `+`**). Sale sin prefijo de tipo.
//
//   - **NEWS FEED** (§8.4) — cada mensaje envuelto en `<ENDECSTART>` y
//     `<ENDECEND>`, con la cabecera ZCZC dentro.
//
//   - **GENERIC CGEN** (§8.3) — `<STX><severidad><texto><ETX>` para un
//     generador de caracteres. No es un formato de estado y no se pide, pero
//     un puerto mal configurado lo manda y el analizador no se atraganta.
//
// El analizador entiende los cuatro aunque solo el primero sea el que se pide,
// porque el cable que hay en la estación es el que hay y el driver no puede
// exigir que alguien vuelva a entrar al menú del ENDEC para que el software
// arranque.
//
// # Lo que por aquí NO se puede saber
//
//  1. **No hay mensaje de «la alerta terminó».** El manual §8.1 es explícito
//     en lo que manda el decoder device: el estado «at the time a message is
//     received or sent». El único fin de mensaje del protocolo es el `NNNN`, y
//     el `NNNN` sale por el **encoder** device, no por el decoder. Un driver
//     que solo lea el puerto del decodificador sabe cuándo empieza la
//     interrupción y **no sabe cuándo terminó**. Las tres maneras honestas de
//     cerrarla, en orden de confianza: el relé PTT abriéndose (reles.go), el
//     retorno de aire (`signal-compare`, capa 2 del ADR 0010), y como último
//     recurso el `+TTTT` de la cabecera, que es una **cota máxima de vigencia
//     de la alerta y no la duración de la interrupción**. Este paquete no
//     elige por T8: entrega lo que llegó y dice qué es.
//
//  2. **`match:` no es una interrupción.** Es «la oí y me interesa»; el relé
//     PENDING de fábrica es justo ese estado. Solo `local:` significa que el
//     ENDEC está tomando el aire (§8.1). Confundirlos escribe en el as-run una
//     interrupción que nunca ocurrió. Evento.Interrumpe respeta la diferencia.
//
//  3. **La cabecera cruda del encoder device tampoco distingue.** Sale igual
//     para una alerta originada aquí y para una retransmitida, porque es la
//     copia de los bytes que el ENDEC puso en el aire. Se entrega como
//     TipoCabecera, sin adivinar.
//
//  4. **Un silencio no es «todo bien».** Un ENDEC sin alertas no manda nada
//     durante semanas, así que un cable desconectado se ve exactamente igual
//     que un mes tranquilo. La prueba semanal (RWT) es el único latido que
//     tiene este protocolo: si la ventana de la prueba semanal pasó y no llegó
//     ningún Evento, el camino está sospechoso. Eso lo mide T8 contra la
//     bitácora de alertas del PRD §12; aquí solo se deja dicho.
//
// # Fuentes
//
//   - Sage ENDEC Manual Rev 1.5 (1822), §8.1 a §8.7 y §5.7: formatos de los
//     device types, programas de relé y «commercial tally».
//     docs/drivers/catalogo/01-alertas-eas.md enlaza la copia archivada.
//   - Referencia pública del protocolo en easstation.com (terceros, no es un
//     manual de Sage) para la interfaz de automatización por TCP del 3644.
//   - 47 CFR 11.31 para el formato de la cabecera SAME.
package sage
