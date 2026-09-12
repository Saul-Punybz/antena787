package ingest

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"antena787/internal/model"
)

func TestFichaDesdeNombreLeeSerieTemporadaYEpisodio(t *testing.T) {
	casos := []struct {
		nombre   string
		serie    string
		t, e     int
		episodio string
	}{
		{"Star.Trek.Strange.New.Worlds.S04E01.1080p.HEVC.x265-MeGusta[EZTVx.to].mkv", "Star Trek Strange New Worlds", 4, 1, ""},
		{"For All Mankind S05E09 1080p x265-ELiTE[EZTVx.to].mkv", "For All Mankind", 5, 9, ""},
		{"Solo.Leveling.S02E04.I.Need.to.Stop.Faking.1080p.NF.WEB-DL.JPN.AAC2.0.H.264.MSubs-ToonsHub.mkv", "Solo Leveling", 2, 4, "I Need to Stop Faking"},
		{"Kojak - 1x03 - El caso del sombrero.mp4", "Kojak", 1, 3, "El caso del sombrero"},
		{"Kojak T1E3.mp4", "Kojak", 1, 3, ""},
		{"Zorro (1957) - Temporada 2 Episodio 12.avi", "Zorro (1957)", 2, 12, ""},
		{"Samurai X Cap 07.mp4", "Samurai X", 1, 7, ""},
		{"[SubGrupo] Hellsing - 04 [1080p].mkv", "Hellsing", 1, 4, ""},
		{"s01e02.mkv", "", 1, 2, ""},
	}
	for _, c := range casos {
		got := FichaDesdeNombre("/videos/" + c.nombre)
		if got.Kind != model.TitleSeries {
			t.Errorf("%s: tipo %q, se esperaba serie", c.nombre, got.Kind)
		}
		if got.Show != c.serie || got.Season != c.t || got.Episode != c.e || got.EpisodeName != c.episodio {
			t.Errorf("%s: serie %q T%dE%d %q; se esperaba %q T%dE%d %q",
				c.nombre, got.Show, got.Season, got.Episode, got.EpisodeName, c.serie, c.t, c.e, c.episodio)
		}
		if c.serie == "" && got.Name == "" {
			t.Errorf("%s: sin serie tiene que quedar al menos el nombre legible", c.nombre)
		}
	}
}

func TestFichaDesdeNombreLeePeliculaYAnio(t *testing.T) {
	casos := []struct {
		nombre string
		titulo string
		anio   int
	}{
		{"Avatar.The.Legend.of.Aang.The.Last.Airbender.2026.1080p.PMNTP.WEBRip.AAC2.0.H264-[LEAK].mp4", "Avatar The Legend of Aang The Last Airbender", 2026},
		{"Casablanca (1942).mkv", "Casablanca", 1942},
		{"Blade Runner 2049 1080p BluRay.mkv", "Blade Runner 2049", 0},
		{"2001 Odisea del espacio.mkv", "2001 Odisea del espacio", 0},
		{"Cortinilla_estacion_v2.mov", "Cortinilla estacion v2", 0},
		{"Noticiero 6pm.mp4", "Noticiero 6pm", 0},
	}
	for _, c := range casos {
		got := FichaDesdeNombre(c.nombre)
		if got.Kind != "" {
			t.Errorf("%s: el nombre solo no decide el tipo; salió %q", c.nombre, got.Kind)
		}
		if got.Name != c.titulo || got.Year != c.anio {
			t.Errorf("%s: %q (%d); se esperaba %q (%d)", c.nombre, got.Name, got.Year, c.titulo, c.anio)
		}
	}
}

func TestEtiquetaQueEsElNombreDelArchivoSeLeeComoNombre(t *testing.T) {
	// El caso real de la prueba sombra: el mkv trae de título embebido su
	// propio nombre con puntos.
	tags := Card{Name: "For.All.Mankind.S05E09", Source: "tags-embebidas", Sources: []string{"tags-embebidas"}}
	file := FichaDesdeNombre("For All Mankind S05E09 1080p x265-ELiTE[EZTVx.to].mkv")
	got := conNombreLegible(tags, file)
	if got.Name != "For All Mankind" || got.Show != "For All Mankind" || got.Season != 5 || got.Episode != 9 || got.Kind != model.TitleSeries {
		t.Fatalf("salió %+v", got)
	}

	// Un título embebido limpio en un archivo con marca de episodio es el
	// nombre del episodio.
	tags = Card{Name: "El caso del sombrero"}
	file = FichaDesdeNombre("Kojak.S01E03.mkv")
	got = conNombreLegible(tags, file)
	if got.Show != "Kojak" || got.EpisodeName != "El caso del sombrero" || got.Season != 1 || got.Episode != 3 {
		t.Fatalf("salió %+v", got)
	}

	// El caso real de Solo Leveling: título embebido «by ToonsHub» y el
	// nombre del episodio en el archivo. Gana el archivo.
	tags = Card{Name: "by ToonsHub"}
	file = FichaDesdeNombre("Solo.Leveling.S02E04.I.Need.to.Stop.Faking.1080p.NF.WEB-DL.JPN.AAC2.0.H.264.MSubs-ToonsHub.mkv")
	got = conNombreLegible(tags, file)
	if got.Show != "Solo Leveling" || got.EpisodeName != "I Need to Stop Faking" || got.Season != 2 || got.Episode != 4 {
		t.Fatalf("salió %+v", got)
	}

	// Un etiquetado de verdad manda sobre el nombre.
	tags = Card{Name: "Rurouni Kenshin", Show: "Rurouni Kenshin", Season: 1, Episode: 3}
	file = FichaDesdeNombre("Samurai.X.S01E03.mkv")
	if got := conNombreLegible(tags, file); got.Show != "Rurouni Kenshin" {
		t.Fatalf("las etiquetas tenían que mandar: %+v", got)
	}
}

func TestPareceNombreDeArchivo(t *testing.T) {
	for s, quiere := range map[string]bool{
		"For.All.Mankind.S05E09": true,
		"Kojak 1080p":            true,
		"El caso del sombrero":   false,
		"Sr. y Sra. Smith":       false,
		"":                       false,
	} {
		if got := PareceNombreDeArchivo(s); got != quiere {
			t.Errorf("%q: %v, se esperaba %v", s, got, quiere)
		}
	}
}

// grabaEstados apunta, en orden, el estado con el que se guardó la fila cada
// vez: sirve para ver que la primera vez fue «ingiriendo».
type grabaEstados struct {
	persistenciaDePrueba
	muEstados sync.Mutex
	estados   []model.AssetState
}

func (g *grabaEstados) SaveAsset(ctx context.Context, a *model.MediaAsset) error {
	g.muEstados.Lock()
	g.estados = append(g.estados, a.State)
	g.muEstados.Unlock()
	return g.persistenciaDePrueba.SaveAsset(ctx, a)
}

// F1-73: quien deja un archivo en la carpeta lo ve entrar desde el primer
// segundo, no cuando termina de medirse.
func TestF1_73_ElArchivoSeVeMientrasSeMide(t *testing.T) {
	ffmpeg, ffprobe := tools(t)
	dir := t.TempDir()
	clip := clipConVolumen(t, ffmpeg, filepath.Join(dir, "Kojak.S01E03.El.caso.mp4"), "-18", 3)
	g := &grabaEstados{}
	deps := depsDePrueba(ffmpeg, ffprobe, dir)
	deps.Persist = g

	asset, title, eps, err := Ingest(context.Background(), deps, clip)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.estados) == 0 || g.estados[0] != model.AssetIngesting {
		t.Fatalf("la primera fila guardada tiene que ser «ingiriendo»; estados: %v", g.estados)
	}
	if g.estados[len(g.estados)-1] != model.AssetReady {
		t.Fatalf("la última tiene que ser «listo»; estados: %v", g.estados)
	}
	if asset.ID == 0 {
		t.Fatal("el asset sale con ID: la fila ya existía")
	}
	// Y de paso, la ficha sale del nombre del archivo (F1-72).
	if title.Name != "Kojak" || title.Kind != model.TitleSeries || len(eps) != 1 || eps[0].Season != 1 || eps[0].Number != 3 || eps[0].Name != "El caso" {
		t.Fatalf("ficha: %+v episodios: %+v", title, eps)
	}

	// Un archivo que ni se puede leer también existe desde el primer
	// segundo, y vuelve en cuarentena con su ID para que quien lo guarde no
	// deje la fila a medias.
	roto := filepath.Join(dir, "roto.mp4")
	if err := os.WriteFile(roto, []byte("ftyp roto"), 0o644); err != nil {
		t.Fatal(err)
	}
	g2 := &grabaEstados{}
	deps.Persist = g2
	deps.Retry = RetryPolicy{Delay: 0, Sleep: SleepCtx}
	asset, _, _, err = Ingest(context.Background(), deps, roto)
	if err == nil || asset.State != model.AssetQuarantine || asset.ID == 0 {
		t.Fatalf("estado %q id %d err %v", asset.State, asset.ID, err)
	}
	if len(g2.estados) == 0 || g2.estados[0] != model.AssetIngesting {
		t.Fatalf("estados del roto: %v", g2.estados)
	}
}

func TestSinopsisLegibleTiraLaFirmaDelEncoder(t *testing.T) {
	for entrada, quiere := range map[string]string{
		"ELiTE-Fri-22-May-2026,03:44:23,1080p,21,fast,Y,10041788,1920,960,2": "",
		"x265 10bit":          "",
		"Encoded by ToonsHub": "Encoded by ToonsHub",
		"Un detective calvo resuelve casos en Nueva York.": "Un detective calvo resuelve casos en Nueva York.",
		"": "",
	} {
		if got := sinopsisLegible(entrada); got != quiere {
			t.Errorf("%q: %q, se esperaba %q", entrada, got, quiere)
		}
	}
}

// Windows no deja ciertos caracteres ni ciertos nombres, y los nombres los
// pone quien sube el archivo. Un título con dos puntos —«Solo Leveling:
// Season 2»— entra sin problema en un Mac y revienta en Windows con un error
// del sistema que no dice nada.
func TestNombreDeArchivoSeguro(t *testing.T) {
	casos := []struct{ entra, sale, porque string }{
		{"Solo Leveling: Season 2.mkv", "Solo Leveling- Season 2.mkv", "los dos puntos son comunísimos en títulos de serie"},
		{`que*pasa?.mp4`, "que-pasa-.mp4", "los comodines no se pueden usar en un nombre"},
		{"CON.mp4", "_CON.mp4", "CON está tomado por Windows desde MS-DOS, y falla hasta con extensión"},
		{"nul.mkv", "_nul.mkv", "los reservados no distinguen mayúsculas"},
		{"peli .mp4 ", "peli.mp4", "Windows se come los espacios del final sin avisar, y dos nombres distintos acaban siendo el mismo"},
		{"normal.mp4", "normal.mp4", "lo que ya está bien no se toca"},
		{"con acentos ñ.mkv", "con acentos ñ.mkv", "los acentos y la eñe sí valen"},
		{"../../etc/passwd", "passwd", "una ruta no puede escaparse de la carpeta"},
		{"", "", "sin nombre no hay archivo"},
	}
	for _, c := range casos {
		if got := NombreDeArchivoSeguro(c.entra); got != c.sale {
			t.Errorf("%q dio %q y tenía que dar %q — %s", c.entra, got, c.sale, c.porque)
		}
	}
}
