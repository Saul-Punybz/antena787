package api

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
)

// Cómo se empaqueta la interfaz
//
// `go:embed` no admite carpetas que no existan, y `web/` la construye otro
// paquete —con npm— que puede no haber corrido todavía. Así que lo embebido
// es **esta** carpeta, `internal/api/ui/`, que siempre está en el repo:
//
//   - `ui/index.html` es el marcador: dice que la API está viva y dónde
//     mirarla. Se sirve cuando no hay interfaz construida.
//   - `ui/dist/` es la interfaz de verdad. La deja ahí el Makefile:
//     `npm --prefix web run build && rm -rf internal/api/ui/dist &&
//     cp -R web/dist internal/api/ui/dist`. Si existe al compilar, entra en
//     el binario y es lo que se sirve.
//
// Y `cmd/antena -web <carpeta>` sirve una carpeta del disco en vez de lo
// embebido, que es lo cómodo mientras se desarrolla la interfaz.

//go:embed all:ui
var uiFS embed.FS

// UI devuelve la interfaz embebida: `ui/dist` si el Makefile la copió, y si
// no el marcador.
func UI() fs.FS {
	if sub, err := fs.Sub(uiFS, "ui/dist"); err == nil {
		if _, err := fs.Stat(sub, "index.html"); err == nil {
			return sub
		}
	}
	sub, err := fs.Sub(uiFS, "ui")
	if err != nil {
		return uiFS
	}
	return sub
}

// UIBuilt dice si lo que se va a servir es la interfaz de verdad o el
// marcador. El arranque lo imprime, para que nadie se pregunte por qué la
// pantalla dice "en construcción".
func UIBuilt() bool {
	sub, err := fs.Sub(uiFS, "ui/dist")
	if err != nil {
		return false
	}
	_, err = fs.Stat(sub, "index.html")
	return err == nil
}

// static sirve la interfaz. Una ruta que no es un archivo cae en index.html,
// que es lo que necesita una aplicación de una sola página; una ruta de
// /api/ que llegue hasta aquí es un 404 de la API, no una página.
func (s *Server) static() http.Handler {
	files := http.FileServer(http.FS(s.opts.UI))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if strings.HasPrefix(clean, "api/") {
			fail(w, http.StatusNotFound, "esa ruta de la API no existe", "")
			return
		}
		if clean == "" || clean == "." {
			clean = "index.html"
		}
		if _, err := fs.Stat(s.opts.UI, clean); err != nil {
			if !os.IsNotExist(err) {
				fail(w, http.StatusInternalServerError, "no se pudo leer la interfaz: "+err.Error(), "")
				return
			}
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}
		files.ServeHTTP(w, r)
	})
}
