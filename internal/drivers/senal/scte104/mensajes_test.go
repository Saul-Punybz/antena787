package scte104

import (
	"bytes"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

// vectorExternoUno es el primero de los dos únicos vectores de prueba de
// SCTE-104 que existen en abierto. Copiado tal cual de
// `src/syntax.test.ts` de github.com/astronautlabs/scte104 (la referencia que
// nombra docs/drivers/catalogo/03-multiplexores-psip-cortes.md), del caso
// «parses a simple SCTE-104 message with no timestamp associated correctly».
//
// Es un multiple_operation_message de 30 bytes con un splice_request_data que
// abre un corte: 4000 ms de pre-roll (el mínimo que aconseja SCTE 67), 240 s de
// duración y auto_return puesto. Cuadra campo por campo con la estructura que
// documenta libklvanc, y su message_size declarado (0x1E = 30) es exactamente
// el largo del arreglo, así que sirve de vector de ida y vuelta byte a byte.
var vectorExternoUno = []byte{
	0xff, 0xff, 0x00, 0x1e, 0x00, 0x00, 0x25,
	0x00, 0x00, 0x00, 0x00, 0x01, 0x01, 0x01,
	0x00, 0x0e, 0x01, 0x60, 0xc6, 0x5c, 0x03,
	0x56, 0xc3, 0x0f, 0xa0, 0x09, 0x60, 0x00,
	0x00, 0x01,
}

// vectorExternoDosPublicado es el segundo vector del mismo archivo de la
// referencia de TypeScript, del caso «parses a SCTE-104 message correctly when
// it has a UTC timestamp associated», copiado tal cual.
//
// **Está mal, y ese es el hallazgo.** Son 38 bytes y declara message_size 0x26
// = 38, pero con la estructura que implementan las dos referencias libres —
// UTC_seconds de 32 bits y UTC_microseconds de **16**— el mensaje solo llega a
// 36: sobran dos bytes 0x00 entre los microsegundos y el num_ops. Solo cuadra
// si los microsegundos se leen de 32 bits, y eso lo contradicen las dos
// implementaciones en su propio código: `unsigned short UTC_microseconds` en
// `vanc-scte_104.h` de libklvanc, citando la tabla 11-2 de ANSI/SCTE 104 2019a,
// y `@Field(16) microseconds` en el `src/syntax.ts` de la propia referencia de
// TypeScript. Es decir: el vector contradice al código que lo acompaña.
//
// La prueba de abajo no lo arregla en silencio: comprueba que el decodificador
// lo **rechaza**, porque un mensaje al que le sobran bytes es justo la forma en
// que se ve que los dos extremos entendieron distinto el largo de un campo, y
// eso es lo que hay que gritar, no adivinar.
var vectorExternoDosPublicado = []byte{
	0xff, 0xff, 0x00, 0x26, 0x00, 0x00, 0x01, 0x00, 0x00,
	0x00, 0x01, 0x60, 0xc6, 0x54, 0x5b, 0x00, 0x00, 0x00, 0x00,
	0x01, 0x01, 0x01, 0x00, 0x0e, 0x01, 0x60, 0xc6, 0x54, 0x5b,
	0x56, 0xc3, 0x0f, 0xa0, 0x09, 0x60, 0x00, 0x00, 0x01,
}

// vectorExternoDosCorregido es el mismo mensaje de arriba con los dos bytes de
// sobra fuera y el message_size bajado a 0x24 = 36, que es lo que mide un
// multiple_operation_message con marca de tiempo UTC y un splice_request_data
// según la estructura de las dos referencias. **Es un vector derivado, no
// copiado**: la marca de tiempo UTC queda verificada contra la estructura que
// documentan las dos fuentes, no contra bytes ajenos.
var vectorExternoDosCorregido = []byte{
	0xff, 0xff, 0x00, 0x24, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00,
	0x01, 0x60, 0xc6, 0x54, 0x5b, 0x00, 0x00,
	0x01,
	0x01, 0x01, 0x00, 0x0e,
	0x01, 0x60, 0xc6, 0x54, 0x5b, 0x56, 0xc3, 0x0f, 0xa0, 0x09, 0x60, 0x00, 0x00, 0x01,
}

// corteDelVectorExterno son los campos del splice_request_data que traen los
// dos vectores de arriba (el segundo repite el corte del primero con otro
// splice_event_id).
var corteDelVectorExterno = Corte{
	Tipo:            CorteEmpiezaNormal,
	EventoID:        0x60C65C03,
	ProgramaID:      0x56C3,
	PreRoll:         4000, // milisegundos
	Duracion:        2400, // décimas de segundo = 240 s
	AvailNum:        0,
	AvailsEsperados: 0,
	AutoReturn:      1,
}

// TestVectorExternoUno es la prueba que más vale de todo el paquete: decodifica
// el vector ajeno, comprueba campo por campo, y lo vuelve a codificar
// esperando **los mismos bytes**. Si esto pasa, el
// multiple_operation_message y el splice_request_data están bien.
func TestVectorExternoUno(t *testing.T) {
	if !EsMultiple(vectorExternoUno) {
		t.Fatal("el vector tiene que ser un multiple_operation_message")
	}
	m, err := DecodificarMultiple(vectorExternoUno)
	if err != nil {
		t.Fatalf("no se pudo decodificar el vector externo: %v", err)
	}
	quiero := Multiple{
		Version:       0,
		IndiceAS:      0,
		Numero:        0x25,
		IndicePID:     0,
		VersionSCTE35: 0,
		Marca:         Marca{Tipo: TiempoNinguno},
		Operaciones:   []Operacion{corteDelVectorExterno.Operacion()},
	}
	if !reflect.DeepEqual(m, quiero) {
		t.Errorf("el mensaje decodificado no es el esperado\n  salió:  %+v\n  quiero: %+v", m, quiero)
	}

	corte, err := m.Operaciones[0].Corte()
	if err != nil {
		t.Fatalf("la operación no se leyó como splice_request_data: %v", err)
	}
	if corte != corteDelVectorExterno {
		t.Errorf("el corte no es el esperado\n  salió:  %+v\n  quiero: %+v", corte, corteDelVectorExterno)
	}
	if corte.Tipo.String() != "spliceStart_normal" {
		t.Errorf("el tipo de corte se llama %q", corte.Tipo.String())
	}

	b, err := m.Codificar()
	if err != nil {
		t.Fatalf("no se pudo volver a codificar: %v", err)
	}
	if !bytes.Equal(b, vectorExternoUno) {
		t.Errorf("la ida y vuelta no dio los mismos bytes\n  salió:  % x\n  quiero: % x", b, vectorExternoUno)
	}
}

// TestVectorExternoDos documenta el desacuerdo del segundo vector con su propia
// referencia, y comprueba que el decodificador no se lo traga.
func TestVectorExternoDos(t *testing.T) {
	t.Run("el publicado se rechaza porque le sobran dos bytes", func(t *testing.T) {
		_, err := DecodificarMultiple(vectorExternoDosPublicado)
		if err == nil {
			t.Fatal("el vector publicado tiene dos bytes de sobra: tenía que fallar")
		}
		if !strings.Contains(err.Error(), "sobran") {
			t.Errorf("el error tenía que decir qué sobra, dijo: %v", err)
		}
	})

	t.Run("el corregido va y vuelve byte a byte", func(t *testing.T) {
		m, err := DecodificarMultiple(vectorExternoDosCorregido)
		if err != nil {
			t.Fatalf("no se pudo decodificar: %v", err)
		}
		quiero := Marca{Tipo: TiempoUTC, Segundos: 0x60C6545B, Microsegundos: 0}
		if m.Marca != quiero {
			t.Errorf("la marca de tiempo no es la esperada\n  salió:  %+v\n  quiero: %+v", m.Marca, quiero)
		}
		if m.Numero != 1 {
			t.Errorf("el message_number salió %d, quiero 1", m.Numero)
		}
		corte, err := m.Operaciones[0].Corte()
		if err != nil {
			t.Fatalf("la operación no se leyó como corte: %v", err)
		}
		// Este vector repite el corte del primero con otro splice_event_id.
		quieroCorte := corteDelVectorExterno
		quieroCorte.EventoID = 0x60C6545B
		if corte != quieroCorte {
			t.Errorf("el corte no es el esperado\n  salió:  %+v\n  quiero: %+v", corte, quieroCorte)
		}
		b, err := m.Codificar()
		if err != nil {
			t.Fatalf("no se pudo volver a codificar: %v", err)
		}
		if !bytes.Equal(b, vectorExternoDosCorregido) {
			t.Errorf("la ida y vuelta no dio los mismos bytes\n  salió:  % x\n  quiero: % x", b, vectorExternoDosCorregido)
		}
	})
}

// TestMarcasDeTiempo recorre los cuatro time_type con los largos que manda la
// tabla 11-2: 1, 7, 5 y 3 bytes contando el propio byte de tipo. Son los largos
// que implementan las dos referencias; solo el TiempoUTC tiene además un vector
// externo detrás (el corregido de arriba).
func TestMarcasDeTiempo(t *testing.T) {
	casos := []struct {
		nombre string
		marca  Marca
		bytes  []byte
	}{
		{"ninguno", Marca{Tipo: TiempoNinguno}, []byte{0x00}},
		{"utc", Marca{Tipo: TiempoUTC, Segundos: 0x60C6545B, Microsegundos: 0x1234},
			[]byte{0x01, 0x60, 0xc6, 0x54, 0x5b, 0x12, 0x34}},
		{"vitc", Marca{Tipo: TiempoVITC, Horas: 19, Minutos: 30, SegundosVITC: 12, Cuadros: 15},
			[]byte{0x02, 19, 30, 12, 15}},
		{"gpi", Marca{Tipo: TiempoGPI, NumeroGPI: 3, FlancoGPI: FlancoCierra},
			[]byte{0x03, 3, 0x00}},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			b, err := c.marca.codificar()
			if err != nil {
				t.Fatalf("no se pudo codificar: %v", err)
			}
			if !bytes.Equal(b, c.bytes) {
				t.Fatalf("salió % x, quiero % x", b, c.bytes)
			}
			vuelta, n, err := decodificarMarca(b)
			if err != nil {
				t.Fatalf("no se pudo decodificar: %v", err)
			}
			if n != len(c.bytes) {
				t.Errorf("consumió %d bytes, quiero %d", n, len(c.bytes))
			}
			if vuelta != c.marca {
				t.Errorf("la vuelta no coincide\n  salió:  %+v\n  quiero: %+v", vuelta, c.marca)
			}
		})
	}

	t.Run("un time_type que no existe se rechaza", func(t *testing.T) {
		if _, err := (Marca{Tipo: 9}).codificar(); err == nil {
			t.Error("el time_type 9 tenía que fallar al codificar")
		}
		if _, _, err := decodificarMarca([]byte{9, 0, 0}); err == nil {
			t.Error("el time_type 9 tenía que fallar al decodificar")
		}
	})

	t.Run("una marca cortada se rechaza", func(t *testing.T) {
		if _, _, err := decodificarMarca([]byte{byte(TiempoUTC), 0x60, 0xc6}); err == nil {
			t.Error("una marca UTC de 3 bytes tenía que fallar")
		}
	})
}

// TestSencilloIdaYVuelta recorre los mensajes del trámite del enlace. Ninguno
// de estos tiene vector externo: la cabecera de 13 bytes está verificada
// contra las dos referencias (el mismo orden de campos en `parse_SCTE_104` de
// libklvanc y en `SingleOperationMessage` de la de TypeScript), pero los bytes
// exactos de cada cuerpo solo están verificados contra este propio código.
func TestSencilloIdaYVuelta(t *testing.T) {
	casos := []struct {
		nombre string
		m      Sencillo
		largo  int
	}{
		{"init_request", InicioPeticion(37, 0), CabeceraSencillaBytes},
		{"init_response", InicioRespuesta(37, 0, ResultadoExito), CabeceraSencillaBytes},
		{"alive_request", VivoPeticion(38, 2, Tiempo{Segundos: 1_400_000_000, Microsegundos: 123456}), CabeceraSencillaBytes + TiempoBytes},
		{"alive_response", VivoRespuesta(38, 2, Tiempo{Segundos: 1_400_000_000, Microsegundos: 123456}), CabeceraSencillaBytes + TiempoBytes},
		{"inject_response", InyectaRespuesta(9, 0, ResultadoExito, 38), CabeceraSencillaBytes + 1},
		{"inject_complete_response", InyectaCompleta(10, 0, ResultadoExito, 38, 2), CabeceraSencillaBytes + 2},
		{"general_response", Sencillo{OpID: OpRespuestaGeneral, Resultado: ResultadoCorteFallo, ResultadoExtra: ResultadoNoUsado, Numero: 40}, CabeceraSencillaBytes},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			b, err := c.m.Codificar()
			if err != nil {
				t.Fatalf("no se pudo codificar: %v", err)
			}
			if len(b) != c.largo {
				t.Fatalf("mide %d bytes, quiero %d", len(b), c.largo)
			}
			// El message_size tiene que ser el largo del marco entero,
			// incluyéndose a sí mismo y al opID.
			if tam := int(b[2])<<8 | int(b[3]); tam != len(b) {
				t.Errorf("el message_size dice %d y el marco mide %d", tam, len(b))
			}
			vuelta, err := DecodificarSencillo(b)
			if err != nil {
				t.Fatalf("no se pudo decodificar: %v", err)
			}
			if !reflect.DeepEqual(vuelta, c.m) {
				t.Errorf("la vuelta no coincide\n  salió:  %+v\n  quiero: %+v", vuelta, c.m)
			}
			otra, err := vuelta.Codificar()
			if err != nil {
				t.Fatalf("no se pudo volver a codificar: %v", err)
			}
			if !bytes.Equal(otra, b) {
				t.Errorf("la segunda codificación cambió los bytes\n  salió:  % x\n  quiero: % x", otra, b)
			}
		})
	}
}

// TestHoraDelAlive comprueba los 8 bytes del cuerpo del alive y que no se pida
// la hora de un mensaje que no la lleva.
func TestHoraDelAlive(t *testing.T) {
	quiero := Tiempo{Segundos: 0x60C6545B, Microsegundos: 999999}
	m := VivoPeticion(1, 0, quiero)
	t.Run("va y vuelve", func(t *testing.T) {
		salio, err := m.Tiempo()
		if err != nil {
			t.Fatalf("no se pudo leer la hora: %v", err)
		}
		if salio != quiero {
			t.Errorf("salió %+v, quiero %+v", salio, quiero)
		}
	})
	t.Run("el init no lleva hora", func(t *testing.T) {
		if _, err := InicioPeticion(1, 0).Tiempo(); err == nil {
			t.Error("un init_request no lleva hora: tenía que fallar")
		}
	})
	t.Run("un alive cortado se rechaza", func(t *testing.T) {
		roto := Sencillo{OpID: OpVivoRespuesta, Datos: []byte{1, 2, 3}}
		if _, err := roto.Tiempo(); err == nil {
			t.Error("un alive con 3 bytes de hora tenía que fallar")
		}
	})
}

// TestNumeroAcusado prueba la única parte del emparejar respuestas que no es
// obvia: que los dos acuses de inyección llevan el message_number del mensaje
// que acusan **en el cuerpo**, no en la cabecera.
func TestNumeroAcusado(t *testing.T) {
	acuse := InyectaRespuesta(99, 0, ResultadoExito, 42)
	if n := acuse.NumeroAcusado(); n != 42 {
		t.Errorf("el inject_response acusa al %d, quiero al 42", n)
	}
	completa := InyectaCompleta(100, 0, ResultadoExito, 42, 3)
	if n := completa.NumeroAcusado(); n != 42 {
		t.Errorf("el inject_complete_response acusa al %d, quiero al 42", n)
	}
	if cue, ok := completa.CuentaDeCue(); !ok || cue != 3 {
		t.Errorf("el cue_message_count salió %d (ok=%v), quiero 3", cue, ok)
	}
	general := Sencillo{OpID: OpRespuestaGeneral, Numero: 42}
	if n := general.NumeroAcusado(); n != 42 {
		t.Errorf("el general_response acusa al %d, quiero al 42", n)
	}
	if _, ok := general.CuentaDeCue(); ok {
		t.Error("un general_response no trae cue_message_count")
	}
}

// TestOperacionesIdaYVuelta recorre las operaciones que van dentro de un
// mensaje múltiple. **Solo el splice_request_data tiene vector externo** (el de
// arriba); las demás están verificadas contra las dos referencias en cuanto al
// orden y tamaño de sus campos, y contra este propio código en cuanto a los
// bytes.
func TestOperacionesIdaYVuelta(t *testing.T) {
	t.Run("splice_request_data en sus cinco tipos", func(t *testing.T) {
		for _, tipo := range []TipoCorte{CorteEmpiezaNormal, CorteEmpiezaYa, CorteTerminaNormal, CorteTerminaYa, CorteCancela} {
			c := Corte{Tipo: tipo, EventoID: 0xDEADBEEF, ProgramaID: 787, PreRoll: 4000, Duracion: 1200, AvailNum: 2, AvailsEsperados: 6, AutoReturn: 1}
			b := c.Codificar()
			if len(b) != CorteBytes {
				t.Fatalf("%s mide %d bytes, quiero %d", tipo, len(b), CorteBytes)
			}
			vuelta, err := DecodificarCorte(b)
			if err != nil {
				t.Fatalf("%s: %v", tipo, err)
			}
			if vuelta != c {
				t.Errorf("%s: la vuelta no coincide\n  salió:  %+v\n  quiero: %+v", tipo, vuelta, c)
			}
			if strings.Contains(tipo.String(), "no definido") {
				t.Errorf("%d tenía que tener nombre", uint8(tipo))
			}
		}
		if TipoCorte(77).String() == "" || !strings.Contains(TipoCorte(77).String(), "no definido") {
			t.Errorf("un splice_insert_type inventado tenía que decirse así: %q", TipoCorte(77))
		}
		if _, err := DecodificarCorte([]byte{1, 2, 3}); err == nil {
			t.Error("un splice_request_data de 3 bytes tenía que fallar")
		}
	})

	t.Run("splice_null_request_data no tiene cuerpo", func(t *testing.T) {
		o := CorteNulo()
		if o.OpID != MopCorteNulo || len(o.Datos) != 0 {
			t.Errorf("salió %+v", o)
		}
	})

	t.Run("time_signal_request_data", func(t *testing.T) {
		s := SenalDeHora{PreRoll: 4000}
		b := s.Codificar()
		if !bytes.Equal(b, []byte{0x0f, 0xa0}) {
			t.Fatalf("salió % x, quiero 0f a0", b)
		}
		vuelta, err := DecodificarSenalDeHora(b)
		if err != nil || vuelta != s {
			t.Errorf("la vuelta salió %+v, err %v", vuelta, err)
		}
		if _, err := DecodificarSenalDeHora([]byte{1}); err == nil {
			t.Error("un time_signal de 1 byte tenía que fallar")
		}
		if o := s.Operacion(); o.OpID != MopSenalDeHora {
			t.Errorf("el opID salió 0x%04X", o.OpID)
		}
	})

	t.Run("insert_DTMF_descriptor_request", func(t *testing.T) {
		d := DTMF{PreRoll: 40, Digitos: "*123#"}
		b, err := d.Codificar()
		if err != nil {
			t.Fatalf("no se pudo codificar: %v", err)
		}
		quiero := append([]byte{40, 5}, []byte("*123#")...)
		if !bytes.Equal(b, quiero) {
			t.Fatalf("salió % x, quiero % x", b, quiero)
		}
		vuelta, err := DecodificarDTMF(b)
		if err != nil || vuelta != d {
			t.Errorf("la vuelta salió %+v, err %v", vuelta, err)
		}
		if _, err := (DTMF{Digitos: "12345678"}).Codificar(); err == nil {
			t.Errorf("más de %d dígitos tenía que fallar", DTMFMaximo)
		}
		if _, err := (DTMF{Digitos: "12A"}).Codificar(); err == nil {
			t.Error("una letra no es un dígito DTMF: tenía que fallar")
		}
		if _, err := DecodificarDTMF([]byte{40, 9, '1'}); err == nil {
			t.Error("un dtmf_length que no cuadra tenía que fallar")
		}
		o, err := d.Operacion()
		if err != nil || o.OpID != MopInsertaDTMF {
			t.Errorf("la operación salió %+v, err %v", o, err)
		}
	})

	t.Run("inject_section_data_request", func(t *testing.T) {
		s := InyectaSeccion{Version: 0, TipoComando: 5, Comando: []byte{0xfc, 0x30, 0x11}}
		b, err := s.Codificar()
		if err != nil {
			t.Fatalf("no se pudo codificar: %v", err)
		}
		quiero := []byte{0x00, 0x03, 0x00, 0x05, 0xfc, 0x30, 0x11}
		if !bytes.Equal(b, quiero) {
			t.Fatalf("salió % x, quiero % x", b, quiero)
		}
		vuelta, err := DecodificarInyectaSeccion(b)
		if err != nil {
			t.Fatalf("no se pudo decodificar: %v", err)
		}
		if !reflect.DeepEqual(vuelta, s) {
			t.Errorf("la vuelta salió %+v, quiero %+v", vuelta, s)
		}
		if _, err := DecodificarInyectaSeccion([]byte{0x00, 0x09, 0, 0}); err == nil {
			t.Error("un SCTE35_command_length que no cuadra tenía que fallar")
		}
		o, err := s.Operacion()
		if err != nil || o.OpID != MopInyectaSeccion {
			t.Errorf("la operación salió %+v, err %v", o, err)
		}
	})

	t.Run("proprietary_command_request", func(t *testing.T) {
		c := ComandoPropietario{ID: 0x0000001F, Comando: 7, Datos: []byte{1, 2, 3}}
		b := c.Codificar()
		quiero := []byte{0x00, 0x00, 0x00, 0x1f, 0x07, 1, 2, 3}
		if !bytes.Equal(b, quiero) {
			t.Fatalf("salió % x, quiero % x", b, quiero)
		}
		vuelta, err := DecodificarComandoPropietario(b)
		if err != nil {
			t.Fatalf("no se pudo decodificar: %v", err)
		}
		if !reflect.DeepEqual(vuelta, c) {
			t.Errorf("la vuelta salió %+v, quiero %+v", vuelta, c)
		}
		// Sin datos: el largo se saca del data_length de la operación, así que
		// cinco bytes es un comando propietario válido.
		corto := ComandoPropietario{ID: 1, Comando: 2}
		vuelta, err = DecodificarComandoPropietario(corto.Codificar())
		if err != nil || !reflect.DeepEqual(vuelta, corto) {
			t.Errorf("sin datos salió %+v, err %v", vuelta, err)
		}
		if _, err := DecodificarComandoPropietario([]byte{1, 2, 3, 4}); err == nil {
			t.Error("un proprietary_command de 4 bytes tenía que fallar")
		}
	})

	t.Run("una operación de fabricante pasa en crudo", func(t *testing.T) {
		// Lo que este paquete no conoce lo lleva y lo trae igual: es lo que
		// hace que un encoder con extensiones propias no obligue a tocar
		// código.
		m := Multiple{Operaciones: []Operacion{{OpID: 0xC001, Datos: []byte{9, 8, 7}}}}
		b, err := m.Codificar()
		if err != nil {
			t.Fatalf("no se pudo codificar: %v", err)
		}
		vuelta, err := DecodificarMultiple(b)
		if err != nil {
			t.Fatalf("no se pudo decodificar: %v", err)
		}
		if !reflect.DeepEqual(vuelta, m) {
			t.Errorf("la vuelta salió %+v, quiero %+v", vuelta, m)
		}
		if n := NombreDeMop(0xC001); n != "definido por el fabricante" {
			t.Errorf("el nombre salió %q", n)
		}
	})
}

// TestMultipleConVariasOperaciones comprueba lo que hace falta para señalar un
// bloque entero con un solo mensaje: varias operaciones en el mismo sobre.
func TestMultipleConVariasOperaciones(t *testing.T) {
	dtmf, err := (DTMF{PreRoll: 40, Digitos: "*123#"}).Operacion()
	if err != nil {
		t.Fatalf("no se pudo armar el DTMF: %v", err)
	}
	m := Multiple{
		Numero:      7,
		IndicePID:   0x0040,
		Marca:       Marca{Tipo: TiempoVITC, Horas: 19, Minutos: 0, SegundosVITC: 0, Cuadros: 0},
		Operaciones: []Operacion{corteDelVectorExterno.Operacion(), dtmf, (SenalDeHora{PreRoll: 4000}).Operacion()},
	}
	b, err := m.Codificar()
	if err != nil {
		t.Fatalf("no se pudo codificar: %v", err)
	}
	vuelta, err := DecodificarMultiple(b)
	if err != nil {
		t.Fatalf("no se pudo decodificar: %v", err)
	}
	if len(vuelta.Operaciones) != 3 {
		t.Fatalf("volvieron %d operaciones, quiero 3", len(vuelta.Operaciones))
	}
	if !reflect.DeepEqual(vuelta, m) {
		t.Errorf("la vuelta no coincide\n  salió:  %+v\n  quiero: %+v", vuelta, m)
	}
}

// TestMensajesRotos comprueba que nada de lo que puede llegar por un socket
// tumba el decodificador ni se cuela a medias.
func TestMensajesRotos(t *testing.T) {
	casos := []struct {
		nombre string
		bytes  []byte
	}{
		{"vacío", nil},
		{"solo el opID", []byte{0xff, 0xff}},
		{"múltiple sin num_ops", []byte{0xff, 0xff, 0x00, 0x0b, 0, 0, 1, 0, 0, 0, 0}},
		{"múltiple con una operación cortada", []byte{0xff, 0xff, 0x00, 0x0e, 0, 0, 1, 0, 0, 0, 0, 1, 0x01, 0x01}},
		{"múltiple que promete más datos de los que trae", []byte{0xff, 0xff, 0x00, 0x10, 0, 0, 1, 0, 0, 0, 0, 1, 0x01, 0x01, 0x00, 0xff}},
		{"múltiple con el message_size mentiroso", []byte{0xff, 0xff, 0x00, 0xff, 0, 0, 1, 0, 0, 0, 0, 0}},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if _, err := DecodificarMultiple(c.bytes); err == nil {
				t.Errorf("tenía que fallar, y no falló")
			}
		})
	}

	t.Run("un sencillo no se lee como múltiple ni al revés", func(t *testing.T) {
		sencillo, err := InicioPeticion(1, 0).Codificar()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := DecodificarMultiple(sencillo); err == nil {
			t.Error("un init_request no es un mensaje múltiple")
		}
		if _, err := DecodificarSencillo(vectorExternoUno); err == nil {
			t.Error("un mensaje múltiple no es un single_operation_message")
		}
		if _, err := (Sencillo{OpID: OpMultiple}).Codificar(); err == nil {
			t.Error("el opID 0xFFFF no cabe en un single_operation_message")
		}
	})

	t.Run("un sencillo con el message_size mentiroso se rechaza", func(t *testing.T) {
		b, err := InicioPeticion(1, 0).Codificar()
		if err != nil {
			t.Fatal(err)
		}
		b[3] = 99
		if _, err := DecodificarSencillo(b); err == nil {
			t.Error("tenía que fallar")
		}
	})

	t.Run("más de 255 operaciones no caben", func(t *testing.T) {
		m := Multiple{Operaciones: make([]Operacion, 256)}
		if _, err := m.Codificar(); err == nil {
			t.Error("256 operaciones no caben en el num_ops de un byte")
		}
	})
}

// TestLeerMarco prueba el encuadre de TCP: el largo va dentro del mensaje, no
// delante, así que un flujo con dos mensajes pegados se separa sin ambigüedad.
func TestLeerMarco(t *testing.T) {
	uno, err := InicioPeticion(1, 0).Codificar()
	if err != nil {
		t.Fatal(err)
	}
	flujo := bytes.NewReader(append(append([]byte{}, uno...), vectorExternoUno...))

	primero, err := LeerMarco(flujo)
	if err != nil {
		t.Fatalf("no se pudo leer el primer marco: %v", err)
	}
	if !bytes.Equal(primero, uno) {
		t.Errorf("el primer marco salió % x, quiero % x", primero, uno)
	}
	segundo, err := LeerMarco(flujo)
	if err != nil {
		t.Fatalf("no se pudo leer el segundo marco: %v", err)
	}
	if !bytes.Equal(segundo, vectorExternoUno) {
		t.Errorf("el segundo marco salió % x, quiero % x", segundo, vectorExternoUno)
	}
	if _, err := LeerMarco(flujo); !errors.Is(err, io.EOF) {
		t.Errorf("al final del flujo quiero io.EOF, salió %v", err)
	}

	t.Run("un marco a medias da ErrUnexpectedEOF", func(t *testing.T) {
		_, err := LeerMarco(bytes.NewReader(vectorExternoUno[:20]))
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Errorf("quiero io.ErrUnexpectedEOF, salió %v", err)
		}
	})

	t.Run("un message_size imposible se rechaza en vez de girar", func(t *testing.T) {
		// Sin esto, un inyector averiado dejaría al lector pidiendo cero bytes
		// para siempre.
		if _, err := LeerMarco(bytes.NewReader([]byte{0x00, 0x01, 0x00, 0x02})); err == nil {
			t.Error("un message_size de 2 tenía que fallar")
		}
	})
}

// TestNombres comprueba las frases que van a la pantalla y a la bitácora: el
// nombre del estándar es el que aparece en el manual del encoder, y es el que
// sirve para llamar al ingeniero de la cabecera.
func TestNombres(t *testing.T) {
	if n := NombreDeResultado(ResultadoExito); n != "éxito" {
		t.Errorf("el result 100 salió %q", n)
	}
	if n := NombreDeResultado(ResultadoPreRollMuyChico); !strings.Contains(n, "pre-roll") {
		t.Errorf("el result 122 salió %q", n)
	}
	if n := NombreDeResultado(200); n != "resultado desconocido" {
		t.Errorf("un result inventado salió %q", n)
	}
	if n := NombreDeOp(OpInicioPeticion); n != "init_request_data" {
		t.Errorf("el opID 0x0001 salió %q", n)
	}
	if n := NombreDeOp(0x0005); n != "definido por el fabricante" {
		t.Errorf("el opID 0x0005 salió %q", n)
	}
	if n := NombreDeOp(0x0050); n != "reservado" {
		t.Errorf("el opID 0x0050 salió %q", n)
	}
	if n := NombreDeMop(MopCorte); n != "splice_request_data" {
		t.Errorf("el opID 0x0101 salió %q", n)
	}
	if n := NombreDeMop(0x0050); n != "reservado" {
		t.Errorf("el opID 0x0050 de operación salió %q", n)
	}
}
