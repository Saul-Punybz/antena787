package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// ── F1-56 · la lista cerrada de jerga prohibida ───────────────────────

// jerga son los cinco términos que el principio 3 del PRD prohíbe en todo lo
// que llega a ver el operador. Es una lista cerrada, y por eso es
// verificable.
var jerga = []string{`driver`, `c[oó]dec`, `GOP`, `LKFS`, `transport stream`}

var reJerga = regexp.MustCompile(`(?i)\b(` + strings.Join(jerga, "|") + `)(e?s)?\b`)

// raiz es la raíz del repositorio vista desde internal/api.
const raiz = "../.."

type hallazgo struct {
	archivo string
	linea   int
	termino string
	texto   string
}

func (h hallazgo) String() string {
	return h.archivo + ":" + strconv.Itoa(h.linea) + " · «" + h.termino + "» en: " + h.texto
}

// TestF1Verif56SinJergaEnLaInterfaz revisa el paquete de cadenas que llega a
// ver el operador: los textos de la interfaz (web/src) y los textos que el
// servidor le manda en cristiano (errores de la API, motivo_en_cristiano,
// avisos del resolver, motivos del importador).
func TestF1Verif56SinJergaEnLaInterfaz(t *testing.T) {
	var todos []hallazgo
	todos = append(todos, jergaEnLaInterfaz(t)...)
	todos = append(todos, jergaEnLosTextosDelServidor(t)...)

	if len(todos) > 0 {
		var b strings.Builder
		b.WriteString("hay jerga prohibida en cadenas que ve el operador (PRD §4.3, F1-56):\n")
		for _, h := range todos {
			b.WriteString("  " + h.String() + "\n")
		}
		t.Fatal(b.String())
	}
}

// ── la interfaz ───────────────────────────────────────────────────────

var (
	reComentarioLinea  = regexp.MustCompile(`(?m)//.*$`)
	reComentarioBloque = regexp.MustCompile(`(?s)/\*.*?\*/`)
)

// jergaEnLaInterfaz recorre web/src/**/*.{ts,tsx}. Se excluyen los
// identificadores de código —claves de objeto (`driver:`), campos de tipo y
// accesos a propiedad (`.driver`)— porque nunca se pintan: lo que se busca
// es texto que llegue a la pantalla.
func jergaEnLaInterfaz(t *testing.T) []hallazgo {
	t.Helper()
	dir := filepath.Join(raiz, "web", "src")
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("no encontré la interfaz en %s: %v", dir, err)
	}
	var out []hallazgo
	err := filepath.WalkDir(dir, func(ruta string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if ext := filepath.Ext(ruta); ext != ".ts" && ext != ".tsx" {
			return nil
		}
		crudo, err := os.ReadFile(ruta)
		if err != nil {
			return err
		}
		// Los comentarios no se pintan.
		limpio := reComentarioBloque.ReplaceAllStringFunc(string(crudo), blanquear)
		limpio = reComentarioLinea.ReplaceAllString(limpio, "")

		for i, linea := range strings.Split(limpio, "\n") {
			for _, m := range reJerga.FindAllStringIndex(linea, -1) {
				if esIdentificador(linea, m[0], m[1]) {
					continue
				}
				out = append(out, hallazgo{
					archivo: ruta, linea: i + 1,
					termino: linea[m[0]:m[1]],
					texto:   strings.TrimSpace(linea),
				})
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("no pude recorrer la interfaz: %v", err)
	}
	return out
}

// esIdentificador dice si la aparición es código y no texto: clave de objeto
// o campo de tipo (`driver:`), acceso a propiedad (`.driver`) o parte de un
// identificador más largo.
func esIdentificador(linea string, ini, fin int) bool {
	antes := strings.TrimRight(linea[:ini], " \t")
	if strings.HasSuffix(antes, ".") || strings.HasSuffix(antes, "_") {
		return true
	}
	despues := strings.TrimLeft(linea[fin:], " \t")
	if strings.HasPrefix(despues, ":") || strings.HasPrefix(despues, "?:") ||
		strings.HasPrefix(despues, "=") || strings.HasPrefix(despues, "(") {
		return true
	}
	return false
}

func blanquear(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '\n' {
			b.WriteRune('\n')
			continue
		}
		b.WriteRune(' ')
	}
	return b.String()
}

// ── los textos que manda el servidor ──────────────────────────────────

// sinksUserFacing son las funciones cuyo texto sale tal cual a pantalla: los
// errores de la API en cristiano, motivo_en_cristiano del ingest, y los
// textos con formato que acaban en un aviso o en la bitácora.
var sinksUserFacing = map[string]bool{
	"fail": true, "failf": true, "failStore": true,
	"Plain": true, "Plainf": true,
	"Errorf": true, "New": true,
	"Publish": true, "Incident": true,
}

// camposUserFacing son los campos de struct que llevan texto para una
// persona: los avisos del resolver, los motivos del importador y el motivo
// en cristiano de un archivo en cuarentena.
var camposUserFacing = map[string]bool{
	"Text": true, "Reason": true, "PlainReason": true, "Notice": true,
	"Detail": true, "Motivo": true, "Attribution": true,
}

// jergaEnLosTextosDelServidor recorre los paquetes del servidor con go/ast y
// solo mira las cadenas que acaban delante de una persona.
func jergaEnLosTextosDelServidor(t *testing.T) []hallazgo {
	t.Helper()
	var out []hallazgo
	paquetes := []string{"api", "app", "resolver", "importer", "ingest", "store", "model"}
	fset := token.NewFileSet()

	for _, p := range paquetes {
		dir := filepath.Join(raiz, "internal", p)
		entradas, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("no pude leer %s: %v", dir, err)
		}
		for _, e := range entradas {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			ruta := filepath.Join(dir, e.Name())
			archivo, err := parser.ParseFile(fset, ruta, nil, 0)
			if err != nil {
				t.Fatalf("no pude leer %s: %v", ruta, err)
			}
			ast.Inspect(archivo, func(n ast.Node) bool {
				var literales []*ast.BasicLit
				switch v := n.(type) {
				case *ast.CallExpr:
					if !sinksUserFacing[nombreDeLlamada(v.Fun)] {
						return true
					}
					for _, arg := range v.Args {
						if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
							literales = append(literales, lit)
						}
					}
				case *ast.KeyValueExpr:
					k, ok := v.Key.(*ast.Ident)
					if !ok || !camposUserFacing[k.Name] {
						return true
					}
					if lit, ok := v.Value.(*ast.BasicLit); ok && lit.Kind == token.STRING {
						literales = append(literales, lit)
					}
				default:
					return true
				}
				for _, lit := range literales {
					texto, err := strconv.Unquote(lit.Value)
					if err != nil {
						texto = lit.Value
					}
					for _, m := range reJerga.FindAllString(texto, -1) {
						pos := fset.Position(lit.Pos())
						out = append(out, hallazgo{
							archivo: pos.Filename, linea: pos.Line,
							termino: m, texto: texto,
						})
					}
				}
				return true
			})
		}
	}
	return out
}

func nombreDeLlamada(fn ast.Expr) string {
	switch v := fn.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		return v.Sel.Name
	}
	return ""
}

// ── F1-57 · revelación progresiva del menú ────────────────────────────

// TestF1Verif57MenuDeCincoYSeis comprueba el menú de la interfaz: cinco
// entradas sin ningún anunciante registrado, seis al registrar el primero
// (aparece Anuncios), y ni multi-canal ni roles ni nombres de usuario
// visibles en ninguna pantalla.
func TestF1Verif57MenuDeCincoYSeis(t *testing.T) {
	ruta := filepath.Join(raiz, "web", "src", "componentes", "Armazon.tsx")
	crudo, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("no encontré el armazón de la interfaz: %v", err)
	}
	fuente := string(crudo)

	entradas := regexp.MustCompile(`\{\s*a:\s*'([^']+)',\s*texto:\s*'([^']+)'([^}]*)\}`).
		FindAllStringSubmatch(fuente, -1)
	if len(entradas) == 0 {
		t.Fatalf("no supe leer el menú de %s", ruta)
	}

	var siempre, conAnunciantes []string
	for _, e := range entradas {
		if strings.Contains(e[3], "soloConAnunciantes") {
			conAnunciantes = append(conAnunciantes, e[2])
			continue
		}
		siempre = append(siempre, e[2])
	}

	if len(siempre) != 5 {
		t.Fatalf("sin anunciantes el menú tiene que traer cinco entradas y trae %d: %v", len(siempre), siempre)
	}
	if len(siempre)+len(conAnunciantes) != 6 {
		t.Fatalf("con el primer anunciante el menú tiene que traer seis y trae %d: %v",
			len(siempre)+len(conAnunciantes), append(siempre, conAnunciantes...))
	}
	if len(conAnunciantes) != 1 || conAnunciantes[0] != "Anuncios" {
		t.Fatalf("la sexta entrada tiene que ser Anuncios y es %v", conAnunciantes)
	}
	for _, quiero := range []string{"Al aire", "Parrilla", "Reglas", "Biblioteca", "Ajustes"} {
		if !contiene(siempre, quiero) {
			t.Fatalf("falta la entrada %q en el menú de siempre: %v", quiero, siempre)
		}
	}

	// Ni multi-canal, ni roles, ni nombres de usuario en ninguna pantalla.
	prohibidos := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bcambiar de canal\b`),
		regexp.MustCompile(`(?i)\bselector de canal\b`),
		regexp.MustCompile(`(?i)\bmis canales\b`),
		regexp.MustCompile(`(?i)\brol(es)?\s+(de|del)\s+usuario`),
		regexp.MustCompile(`(?i)\bpermisos?\b`),
		regexp.MustCompile(`(?i)\bnombre de usuario\b`),
		regexp.MustCompile(`(?i)\busuarios?\s+y\s+`),
		regexp.MustCompile(`(?i)\biniciar sesión con\b`),
	}
	dir := filepath.Join(raiz, "web", "src")
	err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if ext := filepath.Ext(p); ext != ".ts" && ext != ".tsx" {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		limpio := reComentarioBloque.ReplaceAllStringFunc(string(b), blanquear)
		limpio = reComentarioLinea.ReplaceAllString(limpio, "")
		for _, re := range prohibidos {
			if m := re.FindString(limpio); m != "" {
				t.Errorf("%s enseña %q: en F1 no hay multi-canal, ni roles, ni nombres de usuario", p, m)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("no pude recorrer la interfaz: %v", err)
	}
}

func contiene(lista []string, q string) bool {
	for _, v := range lista {
		if v == q {
			return true
		}
	}
	return false
}

// TestF1Verif57ElServidorDiceSiHayAnunciantes comprueba lo otro que hace
// falta para que el menú pase de cinco a seis: que GET /estado diga si hay
// algún anunciante registrado. La interfaz lo lee como `hay_anunciantes`
// (web/src/lib/tipos.ts, web/src/componentes/Armazon.tsx).
func TestF1Verif57ElServidorDiceSiHayAnunciantes(t *testing.T) {
	c := nuevo(t).conClave().entrar()

	w := c.do("GET", "/api/v1/estado", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("/estado dio %d: %s", w.Code, w.Body.String())
	}
	var cuerpo map[string]any
	c.json(w, &cuerpo)

	if _, ok := cuerpo["hay_anunciantes"]; !ok {
		t.Fatalf("GET /estado no dice `hay_anunciantes`, así que el menú nunca puede pasar "+
			"de cinco a seis entradas: la interfaz lo da por false para siempre "+
			"(web/src/componentes/Armazon.tsx:29). Campos servidos: %v", claves(cuerpo))
	}
	if cuerpo["hay_anunciantes"] != false {
		t.Fatalf("un canal recién instalado no tiene anunciantes y /estado dice %v", cuerpo["hay_anunciantes"])
	}
}

func claves(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// ── F1-49 · /guia.xml siempre contesta ────────────────────────────────

// TestF1Verif49GuiaXMLSiempreContesta comprueba que /guia.xml sirve el XMLTV
// vigente sin clave —lo lee el transmisor, que no tiene navegador— y que lo
// que sirve es exactamente la guía que la aplicación tiene en memoria.
func TestF1Verif49GuiaXMLSiempreContesta(t *testing.T) {
	c := nuevo(t).conClave()

	// Sin cookie ninguna: el transmisor no entra con clave.
	w := c.do("GET", "/guia.xml", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("/guia.xml sin clave dio %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "xml") {
		t.Fatalf("la guía se sirve como %q", ct)
	}
	if w.Body.Len() == 0 {
		t.Fatal("/guia.xml contestó vacío: tiene que armar la guía aunque no haya corrido el resolver")
	}
	if w.Header().Get("Last-Modified") == "" {
		t.Fatal("/guia.xml no dice cuándo se generó lo que sirve")
	}

	enMemoria, _ := c.a.Guide()
	if string(enMemoria) != w.Body.String() {
		t.Fatal("lo que sirve /guia.xml no es la guía vigente de la aplicación")
	}

	// Y el alias bajo /api/v1 contesta lo mismo.
	w2 := c.do("GET", "/api/v1/guia.xml", nil)
	if w2.Code != http.StatusOK || w2.Body.String() != w.Body.String() {
		t.Fatalf("/api/v1/guia.xml no contesta lo mismo (%d)", w2.Code)
	}
}
