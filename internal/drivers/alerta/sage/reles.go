// reles.go NO implementa los relés. Documenta cómo se leerían y deja la
// interfaz mínima, y eso es todo lo que se puede hacer con honestidad hoy.
//
// # Por qué no hay implementación
//
// Leer un contacto seco no es hablar con el Sage: es hablar con **la placa de
// entradas que alguien enchufe entre el bloque verde del ENDEC y el PC**. Y esa
// placa no se sabe cuál es. El Sage de CAtv está cableado por relés (PRD §10) y
// las preguntas que faltan están juntas en
// docs/drivers/catalogo/CATALOGO.md: qué terminales están cableados hoy, a qué
// equipo y con qué programa de relé. Escribir un driver contra una placa
// imaginaria es exactamente lo que el principio 1 del PRD prohíbe.
//
// Lo que sí está decidido (docs/drivers/catalogo/05-bibliotecas-go-protocolos.md):
// **las placas de relés se hablan por serial, no por HID.** No existe una
// biblioteca HID pura en Go —las tres conocidas envuelven hidapi/libusb en C y
// rompen el ADR 0002—, así que la familia es `gpi-serial` con
// `go.bug.st/serial`: Numato, Denkovi, KMtronic y Sealevel hablan ASCII por
// USB-serial o Ethernet; SainSmart HID y Ontrak ADU quedan fuera. En Linux con
// GPIO de placa, el camino alterno es `gpi-gpio` con go-gpiocdev.
//
// # Qué relé significa qué (manual §4.5 y §5.7, verificado)
//
// El 1822 trae **tres relés normalmente abiertos**, 1 A a 125 VAC / 60 VDC, en
// el bloque de terminales de atrás. Cada uno se puede asignar a cualquier
// «programa de relé». Los de fábrica:
//
//	ATTN Active     ATTN DETECT  cerrado mientras se oye la señal de atención
//	Encoder Active  PTT          cerrado **mientras el ENDEC está enviando**
//	Decoder Active  PENDING      cerrado desde que llega una alerta seleccionada
//	                             para retransmitir hasta que se retransmitió
//
// Y los demás programas que se pueden asignar: `Pending Done` (como PENDING
// pero solo cuando el mensaje ya se recibió entero y su audio se puede
// revisar), `Ready` (hay una alerta lista para salir: es la que se usa para
// avisarle a la automatización), `ATTN Send`, `PTT Pre`, `Delay Pre/Post`
// (con retardo y con el aviso al generador de caracteres antes o después),
// `End Pulse Pre/Post`, `MSG` y `CONSOLE PTT`.
//
// **El único que contesta «el aire es del ENDEC ahora mismo» es PTT**, y por
// eso es el que le falta al camino serial: el DECODER device dice cuándo
// empieza la interrupción y no dice cuándo termina (ver sage.go), mientras que
// PTT abre exactamente al terminar. Serial y relés juntos dan la historia
// completa —con qué alerta y desde cuándo por el serial, hasta cuándo por el
// relé— y eso es el «si hay dos caminos, se usan los dos y se cruzan» de
// docs/drivers/README.md. Con un solo relé cableado y sin serial, se sabe que
// hubo una interrupción y **no se sabe de qué**: el as-run lo tiene que decir
// así, no rellenar el hueco.
//
// # La entrada, que es la que importa para el ADR 0010
//
// El ENDEC también **escucha** un contacto: la entrada MANUAL OVERRIDE en modo
// «hold off» (§8.5, «Commercial Tally»). Si la automatización cierra ese
// contacto, el ENDEC **retiene** la alerta que quería mandar y la manda cuando
// el contacto se abre. Tope de quince minutos, y EAN/EAT lo ignoran siempre
// (más los tipos que se elijan en MENU.CONFIG.HOLDOFF IGNORE). Es el mecanismo
// que el ADR 0010 describe para CAtv —terminar la tanda comercial antes de
// soltarle el aire a la alerta— y **ya viene en el equipo**: no hay que
// construirlo, hay que cablearlo.
//
// Y por eso mismo no está aquí: cerrar ese contacto es **escribir**, es
// retrasar una alerta de emergencia, y es una decisión de cumplimiento que no
// la toma un paquete de driver. Cuando se implemente será una salida con su
// propio interruptor en Ajustes, apagada de fábrica, con el tope de quince
// minutos verificado contra el equipo y con el ADR 0010 citado en la pantalla.
// La contrapartida de lectura para eso —«¿está la tanda comercial dentro?»— es
// el programa `Ready` del ENDEC leído como entrada, que es el aviso de dos
// pasos que describe §8.5.
//
// # Cómo se probaría sin la placa
//
// Con un EntradaDeContacto de mentira en una prueba (eso es lo que existe hoy),
// y contra el equipo con lo que dice docs/drivers/README.md: cerrar el contacto
// a mano con un puente y ver si el sistema lo nota en menos de un segundo. La
// prueba de diez segundos del asistente para esta familia es literalmente
// «pon el ENDEC en prueba semanal y dime si el punto se encendió».
package sage

import "context"

// EntradaDeContacto es un contacto seco que se puede leer. Es todo lo que el
// resto del sistema necesita saber de una placa de relés, y a propósito no
// tiene nada más: la placa concreta —Numato, Denkovi, KMtronic, un GPIO de
// Linux— la implementa cuando se sepa cuál es.
//
// No hay Write ni Cerrar el contacto: **este paquete no escribe en el mundo**.
// La entrada MANUAL OVERRIDE del ENDEC, que sí sería una escritura, está
// explicada arriba y no se ofrece aquí.
type EntradaDeContacto interface {
	// Nombre es cómo se le dice a una persona: «PTT del ENDEC», «alerta
	// pendiente». Nunca el número de terminal a secas, que no le dice nada a
	// nadie.
	Nombre() string

	// Cerrado dice si el contacto está cerrado ahora. **Un error no es un
	// false.** Los relés del Sage son normalmente abiertos, así que «abierto»
	// significa «no pasa nada» y devolver eso cuando en realidad no se pudo
	// leer la placa es inventarse la calma: por eso el error es el segundo
	// valor y quien llame tiene que mirarlo (regla 3 del contrato de drivers:
	// lo que no se sabe se dice).
	Cerrado(ctx context.Context) (bool, error)

	// Cerrar suelta la placa. Llamarlo dos veces no puede hacer daño.
	Cerrar() error
}
