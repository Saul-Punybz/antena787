package app

// La cuarentena y la bitácora tal como se enseñan (issues #6 y #7).
//
// Un archivo parado tiene nombre de persona, no de ruta: «Space Cobra ·
// T1E4», no `D:\Contenido\space-cobra-e04.mp4`. Y mientras quede alguno sale
// un aviso en Al aire, porque un archivo en cuarentena es un bloque que va a
// faltar y que nadie ve hasta que le toca salir. La alarma es un estado, no
// un suceso: se vuelve a calcular al arrancar, después de cada ingest y
// cuando alguien deja pasar uno.
//
// La bitácora es lo que el sistema hizo solo. Cada tipo de incidente tiene
// su frase en cristiano aquí, en un solo sitio, para que la pantalla, el
// WebSocket y quien lea la API digan lo mismo.

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"antena787/internal/model"
)

// EnCuarentenaEnElAviso es cuántos nombres caben en el detalle de la alarma
// antes de resumir el resto en «y N más».
const EnCuarentenaEnElAviso = 3

// TituloDelArchivo es el nombre con el que una persona reconoce un archivo:
// el título (o el episodio) que lo usa; si nadie lo fichó, el nombre del
// archivo sin extensión.
func (a *App) TituloDelArchivo(ctx context.Context, asset model.MediaAsset) string {
	if a.Store != nil && a.Store.Title != nil {
		if nombre, err := a.Store.Title.NombreDelArchivo(ctx, asset.ID); err == nil && nombre != "" {
			return nombre
		}
	}
	base := filepath.Base(asset.Path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// RefreshCuarentena vuelve a contar los archivos parados y deja (o apaga) el
// aviso de Al aire. Nunca devuelve error: que no se pueda leer la lista no
// puede parar un ingest ni el aire.
func (a *App) RefreshCuarentena(ctx context.Context) {
	if a.Store == nil || a.Store.Media == nil {
		return
	}
	parados, err := a.Store.Media.List(ctx, model.AssetQuarantine)
	if err != nil {
		return
	}
	if len(parados) == 0 {
		a.setAlarms("cuarentena", nil)
		return
	}
	nombres := make([]string, 0, EnCuarentenaEnElAviso)
	for _, m := range parados {
		if len(nombres) == EnCuarentenaEnElAviso {
			break
		}
		nombres = append(nombres, "«"+a.TituloDelArchivo(ctx, m)+"»")
	}
	detalle := strings.Join(nombres, ", ")
	if resto := len(parados) - len(nombres); resto > 0 {
		detalle += fmt.Sprintf(" y %d más", resto)
	}
	texto := fmt.Sprintf("%d archivos en cuarentena", len(parados))
	if len(parados) == 1 {
		texto = "1 archivo en cuarentena"
	}
	a.setAlarms("cuarentena", []Alarma{{
		Tipo:    "material",
		Nivel:   NivelAviso,
		Texto:   texto,
		Detalle: detalle,
		Accion:  &AccionAlarma{Texto: "ver la biblioteca", Ruta: "/biblioteca"},
	}})
}

// textosDeIncidente es la frase de cada tipo de incidente. Los de F2 ya
// están para que la bitácora los pinte el día que el motor los escriba.
var textosDeIncidente = map[string]string{
	// F1
	"cuarentena":              "Un archivo quedó en cuarentena",
	"subida_rechazada":        "Se rechazó un archivo que llegó por el portal",
	"normalizacion_fallida":   "No se pudo preparar un archivo para el aire",
	"subtitulos":              "Nota sobre los subtítulos de un archivo",
	"disco_bajo":              "Queda poco espacio en el disco",
	"salto_de_reloj":          "El reloj de la máquina saltó y el plan se rehizo",
	"relleno_por_defecto":     "Se creó el cartel de la estación como relleno",
	"base_restaurada":         "La base estaba dañada y se restauró el respaldo",
	"propuesta_del_asistente": "El asistente propuso una parrilla",
	"vencimiento":             "Una regla se acerca a su fin",
	"guia_rechazada":          "La guía no se publicó; sigue puesta la anterior",
	"maquina_despierta":       "La máquina no se va a dormir mientras el canal esté encendido",
	"guardian_caido":          "El guardián que impide dormir a la máquina se cayó y se volvió a levantar",
	// F2 — los nombres son los del catálogo de internal/model/incidentes.go.
	"vivo_ausente":       "La fuente en vivo no llegó y se cubrió con relleno",
	"encoder_reiniciado": "El encoder se paró y se relanzó solo",
	"fallo_de_clip":      "Un bloque no pudo salir y lo cubrió el relleno",
	"manual_por_timeout": "El control manual venció y el aire volvió solo",
	"cartel":             "El aire cayó al cartel de la estación",
	"cascada_extendida":  "El relleno cubrió más de lo previsto",
	"solape":             "Dos bloques quisieron salir a la vez",
	"apagon":             "El sistema estuvo apagado",
	"enlace_caido":       "Una salida se cayó y se reconectó",
	"silencio_detectado": "Se detectó silencio en la salida",
	"negro_detectado":    "Se detectó negro en la salida",
	// Los dos nombres que se usaron antes de que el catálogo existiera. Se
	// dejan para que una fila vieja siga teniendo su frase; nadie escribe ya
	// con ellos.
	"encoder_colgado": "El encoder se paró y se relanzó solo",
	"timeout_manual":  "El control manual venció y el aire volvió solo",
}

// TextoDeIncidente es la frase en cristiano de un tipo de incidente. Un tipo
// que no está en la lista se enseña legible igual: `panico_ingest` →
// «Una parte del sistema falló y se relanzó sola (ingest)».
func TextoDeIncidente(tipo string) string {
	if t, ok := textosDeIncidente[tipo]; ok {
		return t
	}
	if resto, es := strings.CutPrefix(tipo, "panico_"); es {
		return fmt.Sprintf("Una parte del sistema falló y se relanzó sola (%s)", resto)
	}
	return strings.ReplaceAll(tipo, "_", " ")
}
