package same

import (
	"strings"
	"time"
)

// La cabecera es «ZCZC-ORG-EEE-PSSCCC+TTTT-JJJHHMM-LLLLLLLL-», con de una a
// treinta y una zonas separadas por guion y un más (+) antes de la duración.
// La más corta —una sola zona— mide 42 caracteres:
//
//	ZCZC- ORG- EEE- PSSCCC +TTTT- JJJHHMM- LLLLLLLL-
//	  5    4    4     6      6       8        9      = 42
const (
	minCabecera = 42
	// maxZonas es el tope de la norma: treinta y una áreas por mensaje.
	maxZonas = 31
)

// analizarCabecera lee una cabecera cruda. Es estricto a propósito: si algo no
// cuadra devuelve falso y la cabecera no se emite. Un carácter de más en un
// código de zona es un municipio equivocado en el as-run, y para eso están las
// tres repeticiones.
//
// Lo único que no se valida es el significado: un ORG o un código de evento que
// no estén en la tabla de la norma se guardan tal cual. Esto es evidencia de lo
// que salió al aire, no un filtro de lo que debía salir.
func analizarCabecera(b []byte) (Cabecera, bool) {
	var c Cabecera
	if len(b) < minCabecera || len(b) > maxBufCabecera {
		return c, false
	}
	if string(b[:5]) != "ZCZC-" {
		return c, false
	}
	i := 5

	campo := func(n int) ([]byte, bool) {
		if i+n > len(b) {
			return nil, false
		}
		v := b[i : i+n]
		i += n
		return v, true
	}
	separador := func(s byte) bool {
		if i >= len(b) || b[i] != s {
			return false
		}
		i++
		return true
	}

	org, ok := campo(3)
	if !ok || !soloLetras(org) || !separador('-') {
		return c, false
	}
	eve, ok := campo(3)
	if !ok || !soloLetras(eve) || !separador('-') {
		return c, false
	}

	// Las zonas: de una a treinta y una, seis dígitos cada una, separadas por
	// guion. El más (+) cierra la lista.
	zonas := make([]string, 0, 4)
	for {
		z, ok := campo(6)
		if !ok || !soloDigitos(z) {
			return c, false
		}
		zonas = append(zonas, string(z))
		if i >= len(b) {
			return c, false
		}
		if b[i] == '+' {
			i++
			break
		}
		if b[i] != '-' || len(zonas) >= maxZonas {
			return c, false
		}
		i++
	}

	// TTTT es la vigencia en horas y minutos: +0015 son quince minutos, +0600
	// son seis horas. La norma solo permite múltiplos de 15 minutos hasta la
	// hora y de 30 después, pero eso no se exige: un ENDEC mal configurado que
	// manda +0020 salió al aire igual, y el as-run tiene que decirlo.
	tttt, ok := campo(4)
	if !ok || !soloDigitos(tttt) {
		return c, false
	}
	horas := int(tttt[0]-'0')*10 + int(tttt[1]-'0')
	minutos := int(tttt[2]-'0')*10 + int(tttt[3]-'0')
	if minutos > 59 || !separador('-') {
		return c, false
	}

	// JJJHHMM: día del año (1 a 366) y hora UTC.
	inst, ok := campo(7)
	if !ok || !soloDigitos(inst) {
		return c, false
	}
	dia := int(inst[0]-'0')*100 + int(inst[1]-'0')*10 + int(inst[2]-'0')
	hh := int(inst[3]-'0')*10 + int(inst[4]-'0')
	mm := int(inst[5]-'0')*10 + int(inst[6]-'0')
	if dia < 1 || dia > 366 || hh > 23 || mm > 59 || !separador('-') {
		return c, false
	}

	// LLLLLLLL: ocho caracteres con el identificador del participante,
	// rellenado con espacios. Una emisora de FM o TV manda la barra en vez del
	// guion (WXYZ/FM), porque el guion está reservado como separador.
	llam, ok := campo(8)
	if !ok || !imprimibles(llam) {
		return c, false
	}

	// Y el guion final, que tiene que ser el último carácter: si sobra algo
	// después, lo que se leyó no era esta cabecera.
	if i != len(b)-1 || b[i] != '-' {
		return c, false
	}

	c.Org = string(org)
	c.Evento = string(eve)
	c.Zonas = zonas
	c.Duracion = time.Duration(horas)*time.Hour + time.Duration(minutos)*time.Minute
	c.Instante = string(inst)
	c.Llamada = strings.TrimRight(string(llam), " ")
	return c, true
}

func soloLetras(b []byte) bool {
	for _, c := range b {
		if c < 'A' || c > 'Z' {
			return false
		}
	}
	return true
}

func soloDigitos(b []byte) bool {
	for _, c := range b {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func imprimibles(b []byte) bool {
	for _, c := range b {
		if c < 32 || c > 126 {
			return false
		}
	}
	return true
}

// votar decide qué cabecera se queda de las que se oyeron.
//
// Primero lo barato: si dos o tres copias son idénticas, esa gana y se sabe con
// cuántas. Si las tres difieren, se vota carácter por carácter entre las que
// tengan el largo más común —cada posición se queda con el carácter que
// aparezca más veces—, que es lo que arma una cabecera buena con tres copias
// sucias, siempre que no se hayan ensuciado en el mismo sitio.
//
// Devuelve la cadena elegida y cuántas de las copias coinciden exactamente con
// ella (cero si salió del voto por carácter y no coincide con ninguna).
func votar(cands []candidato) (string, int) {
	if len(cands) == 0 {
		return "", 0
	}
	mejor, mejorN := cands[0].crudo, 0
	for _, a := range cands {
		n := 0
		for _, b := range cands {
			if a.crudo == b.crudo {
				n++
			}
		}
		if n > mejorN {
			mejor, mejorN = a.crudo, n
		}
	}
	if mejorN >= 2 || len(cands) == 1 {
		return mejor, mejorN
	}

	// Todas distintas: se vota carácter por carácter entre las del largo más
	// común.
	largo, largoN := 0, 0
	for _, a := range cands {
		n := 0
		for _, b := range cands {
			if len(a.crudo) == len(b.crudo) {
				n++
			}
		}
		if n > largoN {
			largo, largoN = len(a.crudo), n
		}
	}
	if largoN < 2 {
		return mejor, mejorN
	}
	votado := make([]byte, largo)
	for k := 0; k < largo; k++ {
		var cuenta [256]int
		mejorC, mejorCN := byte(0), 0
		for _, a := range cands {
			if len(a.crudo) != largo {
				continue
			}
			c := a.crudo[k]
			cuenta[c]++
			if cuenta[c] > mejorCN {
				mejorC, mejorCN = c, cuenta[c]
			}
		}
		votado[k] = mejorC
	}
	s := string(votado)
	n := 0
	for _, a := range cands {
		if a.crudo == s {
			n++
		}
	}
	return s, n
}
