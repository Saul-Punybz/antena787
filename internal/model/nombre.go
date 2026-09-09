package model

import "strings"

// ── la clave de un nombre ─────────────────────────────────────────────
//
// Emparejar títulos (F1-64 a F1-67) necesita una sola forma de reducir un
// nombre a algo comparable: la que usa el importador para buscar parecidos
// (internal/importer/match.go, newNameKey) y la que usa la base para guardar
// y resolver alias (title_alias.clave) tienen que ser la misma, o un alias
// aprendido hoy no se encontraría mañana. Por eso vive aquí, en el modelo,
// que es lo único que los dos paquetes comparten.

// articulos son los artículos que no cuentan al principio de un nombre:
// «The Green Hornet» y «Green Hornet» son el mismo programa.
var articulos = map[string]bool{"the": true, "el": true, "la": true, "los": true, "las": true}

// acentos es la tabla de letras con tilde o diéresis y su letra pelada.
var acentos = map[rune]rune{
	'á': 'a', 'à': 'a', 'ä': 'a', 'â': 'a', 'ã': 'a', 'å': 'a',
	'é': 'e', 'è': 'e', 'ë': 'e', 'ê': 'e',
	'í': 'i', 'ì': 'i', 'ï': 'i', 'î': 'i',
	'ó': 'o', 'ò': 'o', 'ö': 'o', 'ô': 'o', 'õ': 'o',
	'ú': 'u', 'ù': 'u', 'ü': 'u', 'û': 'u',
	'ñ': 'n', 'ç': 'c',
}

// ClaveDeNombre deja el nombre reducido a su clave: todo en minúsculas, sin
// acentos, sin signos, sin espacios y sin el artículo del principio. El
// apóstrofo se quita sin dejar hueco («Gilligan's» → «gilligans»); los demás
// signos separan palabras, así que los números y los años entre paréntesis
// cuentan como una palabra más.
//
// Así «Samurai X», «SAMURAIX» y «samurai-x» dan todos "samuraix", que es lo
// que hace que un alias guardado una vez sirva escriba quien escriba la hoja.
func ClaveDeNombre(s string) string {
	// Minúsculas primero y acentos después: la tabla de acentos es de letras
	// minúsculas, así que al revés una «Ñ» se escaparía sin pelar.
	s = QuitarAcentos(strings.ToLower(desescaparMarkdown(s)))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '\'' || r == '’':
			// nada: el apóstrofo no parte la palabra
		default:
			b.WriteRune(' ')
		}
	}
	palabras := strings.Fields(b.String())
	if len(palabras) > 1 && articulos[palabras[0]] {
		palabras = palabras[1:]
	}
	return strings.Join(palabras, "")
}

// QuitarAcentos cambia cada letra acentuada por su letra pelada y deja el
// resto igual.
func QuitarAcentos(s string) string {
	var b strings.Builder
	for _, r := range s {
		if a, ok := acentos[r]; ok {
			b.WriteRune(a)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// desescaparMarkdown deshace los escapes que mete un exportador a markdown
// (`\_` → `_`, `\!` → `!`, `\-` → `-`) y recorta los bordes: una hoja pegada
// desde Google Sheets llega así y la clave no puede depender de eso.
func desescaparMarkdown(s string) string {
	if !strings.Contains(s, "\\") {
		return strings.TrimSpace(s)
	}
	var b strings.Builder
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		if rs[i] == '\\' && i+1 < len(rs) && esSignoASCII(rs[i+1]) {
			b.WriteRune(rs[i+1])
			i++
			continue
		}
		b.WriteRune(rs[i])
	}
	return strings.TrimSpace(b.String())
}

func esSignoASCII(r rune) bool {
	return strings.ContainsRune("!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~", r)
}
