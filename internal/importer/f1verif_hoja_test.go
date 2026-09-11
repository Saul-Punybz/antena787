package importer

import (
	"fmt"
	"strings"
	"testing"

	"antena787/internal/model"
)

// F1-50 — Una hoja de 200 filas de las cuales 7 no se pueden interpretar:
// se importan las 193 válidas y salen las 7 fila por fila, con el motivo en palabras
// claras. Nunca se rechaza la hoja entera.
func TestF1Verif50DoscientasFilasSieteMalas(t *testing.T) {
	lineas := []string{"Id\tVideo\tDuración\tDías\tHoras\tFec Ini\tFec Final"}

	// Las siete que no cuadran, con un motivo distinto cada una.
	malas := map[int]string{
		12:  "\t\t1\tLMMJV__\t8:00:00 AM\t8/9/2026\t12/20/2026",       // sin título
		37:  "\tKojak\t1\tLMMJV__\t\t8/9/2026\t12/20/2026",            // sin hora
		58:  "\tTarzan\t1\tLXMJV__\t9:00:00 AM\t8/9/2026\t12/20/2026", // patrón inválido
		91:  "\tAstroboy\t1\tLMMJV__\t9:30:00 AM\tayer\t12/20/2026",   // fecha ilegible
		120: "\tZorro 57\t1\tLMMJV__\tal mediodía\t8/9/2026\t12/20/2026",
		154: "\t-\t1\tLMMJV__\t10:00:00 AM\t8/9/2026\t12/20/2026", // título vacío ("-")
		188: "\tGet Smart\t1\tLMMJV__\t11:00:00 AM\t8/9/2026\tel año que viene",
	}

	for n := 1; n <= 200; n++ {
		if mala, ok := malas[n]; ok {
			lineas = append(lineas, fmt.Sprint(n)+mala)
			continue
		}
		// Una fila buena: media hora distinta para cada una, dentro del día.
		h := 6 + (n % 16)
		m := (n % 2) * 30
		lineas = append(lineas, fmt.Sprintf("%d\tPrograma %d\t1\tLMMJV__\t%d:%02d:00\t8/9/2026\t12/20/2026", n, n, h, m))
	}

	sheet, errs := Parse(strings.Join(lineas, "\n"))
	if len(errs) != 0 {
		t.Fatalf("Parse rechazó la hoja entera: %v", errs)
	}
	res := Rules(sheet, catv)

	if len(res.Rules) != 193 {
		t.Fatalf("se esperaban 193 reglas importadas y salieron %d — %s", len(res.Rules), res.Summary())
	}
	if len(res.RowErrors) != 7 {
		t.Fatalf("se esperaban 7 filas con problema y salieron %d: %+v", len(res.RowErrors), res.RowErrors)
	}
	for _, e := range res.RowErrors {
		if e.Line == 0 {
			t.Fatalf("la fila %q no dice de qué línea del pegado salió: %+v", e.SheetID, e)
		}
		if strings.TrimSpace(e.Reason) == "" {
			t.Fatalf("la fila %q no trae motivo", e.SheetID)
		}
		// El motivo se lee, no se descifra: nada de códigos ni de "error".
		if strings.Contains(strings.ToLower(e.Reason), "parse") ||
			strings.Contains(strings.ToLower(e.Reason), "invalid") {
			t.Fatalf("el motivo de la fila %q no está en palabras claras: %q", e.SheetID, e.Reason)
		}
	}
	// Las siete son exactamente las que se sembraron.
	vistas := map[string]bool{}
	for _, e := range res.RowErrors {
		vistas[e.SheetID] = true
	}
	for n := range malas {
		if !vistas[fmt.Sprint(n)] {
			t.Fatalf("la fila %d no se reportó como problema; sí reportadas: %v", n, vistas)
		}
	}
}

// F1-51 — Una fila con hora 2:00 AM y fecha de calendario del martes se
// guarda con la fecha corrida un día atrás (lunes, su día de emisión) y se
// reporta en la lista fila por fila.
func TestF1Verif51MadrugadaCorreLaFecha(t *testing.T) {
	// 8/9/2026 y 12/8/2026 son martes; el canal empieza el día a las 6:00 AM.
	tsv := strings.Join([]string{
		"Id\tVideo\tDuración\tDías\tHoras\tFec Ini\tFec Final",
		"901\tMadrugada\t1\t_M_____\t2:00:00 AM\t9/8/2026\t12/8/2026",
		"902\tMañana\t1\t_M_____\t8:00:00 AM\t9/8/2026\t12/8/2026",
	}, "\n")

	sheet, errs := Parse(tsv)
	if len(errs) != 0 {
		t.Fatalf("Parse devolvió errores: %v", errs)
	}
	res := Rules(sheet, catv)
	if len(res.Rules) != 2 {
		t.Fatalf("se esperaban 2 reglas: %s — %+v", res.Summary(), res.RowErrors)
	}

	_, madrugada := buscar(t, res, "901")
	if madrugada.From != model.Day("2026-09-07") {
		t.Fatalf("la de las 2:00 AM tiene que arrancar el lunes 2026-09-07 y arranca el %s", madrugada.From)
	}
	if madrugada.To != model.Day("2026-12-07") {
		t.Fatalf("la de las 2:00 AM tiene que terminar el lunes 2026-12-07 y termina el %s", madrugada.To)
	}
	if string(madrugada.Days) != "L______" {
		t.Fatalf("el patrón también se corre: esperaba L______ y salió %q", madrugada.Days)
	}

	// La de las 8:00 AM, ya dentro del día de emisión, no se toca.
	_, manana := buscar(t, res, "902")
	if manana.From != model.Day("2026-09-08") || string(manana.Days) != "_M_____" {
		t.Fatalf("la de las 8:00 AM no se corre: %s / %s", manana.From, manana.Days)
	}

	// Y se dice, fila por fila.
	if len(res.DatesShifted) != 1 {
		t.Fatalf("se esperaba 1 fecha corrida reportada y hay %d: %+v", len(res.DatesShifted), res.DatesShifted)
	}
	sh := res.DatesShifted[0]
	if sh.SheetID != "901" || sh.Line == 0 {
		t.Fatalf("el aviso no señala la fila de la hoja: %+v", sh)
	}
	for _, trozo := range []string{"2:00", "día de emisión", "un día atrás"} {
		if !strings.Contains(sh.Text, trozo) {
			t.Fatalf("el aviso no explica el corrimiento (falta %q): %q", trozo, sh.Text)
		}
	}
}

// F1-52 — Una fila cuyo título coincide con una live_source existente
// («RadioOnce Live!») y que trae Duración = 6 entra como bloque en vivo,
// ignora la columna de episodios y lo avisa fila por fila.
func TestF1Verif52FuenteEnVivoIgnoraEpisodios(t *testing.T) {
	tsv := strings.Join([]string{
		"Id\tVideo\tDuración\tDías\tHoras\tFec Ini\tFec Final",
		"305\tRadioOnce Live!\t6\tLMMJV__\t12:00:00 PM\t9/7/2026\t12/7/2026",
	}, "\n")

	sheet, errs := Parse(tsv)
	if len(errs) != 0 {
		t.Fatalf("Parse devolvió errores: %v", errs)
	}
	res := Rules(sheet, catv, WithLiveNames("RadioOnce Live!"))
	if len(res.Rules) != 1 {
		t.Fatalf("se esperaba 1 regla: %s — %+v", res.Summary(), res.RowErrors)
	}

	_, r := buscar(t, res, "305")
	if r.Kind != model.RuleLive {
		t.Fatalf("la fila de una fuente en vivo entra como bloque en vivo y salió %q", r.Kind)
	}
	if r.EpisodesPerRun != 0 {
		t.Fatalf("la columna de episodios se ignora en un bloque en vivo: quedó en %d", r.EpisodesPerRun)
	}

	var aviso string
	for _, n := range res.Notices {
		if n.SheetID == "305" {
			aviso = n.Text
		}
	}
	if aviso == "" {
		t.Fatalf("no se avisó de que se ignoró la columna: %+v", res.Notices)
	}
	for _, trozo := range []string{"RadioOnce Live!", "en vivo", "6"} {
		if !strings.Contains(aviso, trozo) {
			t.Fatalf("el aviso no dice lo que se ignoró (falta %q): %q", trozo, aviso)
		}
	}
}
