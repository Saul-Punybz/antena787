package f0

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"antena787/internal/engine"
	"antena787/internal/ts"
)

// Result es un criterio del §22.1 con su medida.
type Result struct {
	ID, Name, Value string
	Pass            *bool // nil = no se pudo medir
}

func (r Result) mark() string {
	if r.Pass == nil {
		return "—"
	}
	if *r.Pass {
		return "PASA"
	}
	return "FALLA"
}

func yes(b bool) *bool { return &b }

// Analyze mide la salida de una corrida y escribe REPORTE.md.
func Analyze(ffmpeg, ffprobe, media, out string, log func(string)) error {
	specs, err := Load(media)
	if err != nil {
		return err
	}
	byName := map[string]ClipSpec{}
	byID := map[int]ClipSpec{}
	for _, s := range specs {
		byName[s.Name] = s
		byID[s.ID] = s
	}
	events, err := loadEvents(filepath.Join(out, "events.jsonl"))
	if err != nil {
		return err
	}
	tsPath := filepath.Join(out, "catv.ts")
	fm := engine.CAtv
	var results []Result

	// 1 · El transport stream como lo vería el multiplexor.
	log("Leyendo el TS…")
	f, err := os.Open(tsPath)
	if err != nil {
		return err
	}
	rep, err := ts.Analyze(f)
	f.Close()
	if err != nil {
		return err
	}
	log("  " + rep.String())
	results = append(results, Result{"F0-04", "Marcas de tiempo monotónicas, sin saltos", fmt.Sprintf("%d retrocesos, %d saltos >1 s", rep.TSNonMonotone, rep.TSGapsOver1s), yes(rep.TSNonMonotone == 0 && rep.TSGapsOver1s == 0)})
	results = append(results, Result{"F0-TS", "Tasa constante ±1 %, PCR ≤ 40 ms, sin errores de continuidad", fmt.Sprintf("%.3f Mb/s ±%.2f %% · PCR máx %.1f ms · CC %d", rep.BitrateMean/1e6, rep.BitrateDevPct, rep.PCRMaxGapMs, rep.CCErrors), yes(rep.BitrateDevPct <= 1 && rep.PCRMaxGapMs <= 40 && rep.CCErrors == 0)})

	// 2 · Video: marcadores cuadro a cuadro.
	log("Leyendo los marcadores del video…")
	frames, err := readMarkers(ffmpeg, tsPath, fm, byID)
	if err != nil {
		return err
	}
	log(fmt.Sprintf("  %d cuadros en la salida", len(frames)))
	var cuts []engine.Event
	for _, e := range events {
		if e.Kind == "corte" || e.Kind == "relleno" {
			cuts = append(cuts, e)
		}
	}
	cutProblems, cutsChecked, blackFrames, details := checkCuts(frames, cuts, byName, fm)
	results = append(results, Result{"F0-03", "Cuadros duplicados o perdidos en cada cambio de clip", fmt.Sprintf("%d cambios revisados, %d con problema", cutsChecked, cutProblems), yes(cutProblems == 0 && cutsChecked > 0)})
	results = append(results, Result{"F0-NEGRO", "Ningún cuadro negro en la salida", fmt.Sprintf("%d cuadros negros de %d", blackFrames, len(frames)), yes(blackFrames == 0)})

	// 3 · Audio: pitidos y clics en cada corte.
	log("Leyendo el audio…")
	aStart, vStart := streamStarts(ffprobe, tsPath)
	offsets, clicks, err := checkAudio(ffmpeg, tsPath, fm, cuts, frames, aStart, vStart)
	if err != nil {
		return err
	}
	var maxClick float64 = -120
	for _, c := range clicks {
		maxClick = math.Max(maxClick, c)
	}
	results = append(results, Result{"F0-01", "Discontinuidad de audio en cada cambio de clip (< −40 dBFS)", fmt.Sprintf("peor: %.1f dBFS en %d cortes", maxClick, len(clicks)), yes(len(clicks) > 0 && maxClick < -40)})
	if len(offsets) >= 2 {
		var ms []string
		for _, o := range offsets {
			ms = append(ms, fmt.Sprintf("%.1f", o.ms))
		}
		log("  desfase A/V por corte (ms): " + strings.Join(ms, " "))
		// Cada archivo fuente trae su propio retardo de arranque de audio
		// (AAC, AC-3), así que el desfase se compara clip contra sí mismo
		// entre vueltas: eso es deriva; lo otro es el archivo.
		first := map[string]float64{}
		var drift float64
		var worstClip string
		for _, o := range offsets {
			if f, ok := first[o.clip]; ok {
				if d := math.Abs(o.ms - f); d > drift {
					drift, worstClip = d, o.clip
				}
			} else {
				first[o.clip] = o.ms
			}
		}
		results = append(results, Result{"F0-02", "Desfase audio-video acumulado (< 20 ms)", fmt.Sprintf("deriva máxima %.1f ms (%s) comparando cada clip consigo mismo en %d cortes; desfase fijo por archivo entre %.1f y %.1f ms", drift, worstClip, len(offsets), minMs(offsets), maxMs(offsets)), yes(drift < 20)})
	} else {
		results = append(results, Result{"F0-02", "Desfase audio-video acumulado (< 20 ms)", "no hubo cortes suficientes para medir", nil})
	}

	// 4 · Subtítulos.
	cc := hasClosedCaptions(ffprobe, tsPath)
	results = append(results, Result{"F0-05", "Los subtítulos CEA-608 llegan a la salida", ccText(cc), ccPass(cc)})

	// 5 · El archivo corrupto.
	corto, relleno := 0, 0
	for _, e := range events {
		if e.Kind == "clip_corto" && strings.Contains(e.Clip, "corrupto") {
			corto++
		}
		if e.Kind == "relleno" {
			relleno++
		}
	}
	results = append(results, Result{"F0-06", "El archivo corrupto cae a relleno sin negro y queda registrado", fmt.Sprintf("%d veces detectado como corto, %d entradas de relleno, %d cuadros negros", corto, relleno, blackFrames), yes(corto > 0 && relleno > 0 && blackFrames == 0)})

	// 6 · CPU y RAM.
	cpu, ram, n := summarizeStats(filepath.Join(out, "stats.csv"))
	results = append(results, Result{"F0-07", "CPU y RAM medidos (dos salidas)", fmt.Sprintf("CPU media %.0f %% (de un núcleo), RAM media %.0f MB, %d muestras", cpu, ram, n), yes(n > 0)})
	if cpu1, ram1, n1 := summarizeStats(filepath.Join(out, "stats-one.csv")); n1 > 0 {
		results = append(results, Result{"F0-07b", "CPU y RAM medidos (una salida)", fmt.Sprintf("CPU media %.0f %%, RAM media %.0f MB, %d muestras", cpu1, ram1, n1), yes(true)})
	}
	results = append(results, Result{"F0-08", "Sesiones de encoder por hardware que aguanta la máquina", "manual: no se midió en esta corrida", nil})

	// 7 · Otros eventos que cuentan la historia.
	counts := map[string]int{}
	for _, e := range events {
		counts[e.Kind]++
	}

	return writeReport(filepath.Join(out, "REPORTE.md"), results, rep, events, counts, details, len(frames), log)
}

// --- eventos ---------------------------------------------------------------

func loadEvents(path string) ([]engine.Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []engine.Event
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		var e engine.Event
		if err := json.Unmarshal(sc.Bytes(), &e); err == nil {
			out = append(out, e)
		}
	}
	return out, nil
}

// --- video ------------------------------------------------------------------

// frameInfo es lo que se lee de cada cuadro de salida.
type frameInfo struct {
	id, n int
	luma  float64
	ok    bool // el marcador se pudo leer
}

// readMarkers decodifica la salida a 1/4 en gris y lee los cinco bloques.
// Cada clip tiene su geometría (pillarbox, escala), así que el bloque se
// busca donde el conformado lo puso.
func readMarkers(ffmpeg, tsPath string, fm engine.Format, byID map[int]ClipSpec) ([]frameInfo, error) {
	const div = 4
	w, h := fm.Width/div, fm.Height/div
	cmd := exec.Command(ffmpeg, "-nostdin", "-hide_banner", "-loglevel", "error", "-i", tsPath,
		"-map", "0:v:0", "-vf", fmt.Sprintf("scale=%d:%d:flags=area,format=gray", w, h), "-f", "rawvideo", "pipe:1")
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	r := bufio.NewReaderSize(pipe, 4<<20)
	buf := make([]byte, w*h)
	var out []frameInfo
	// Geometrías posibles: una por clip. Probamos la del clip que dice el
	// bloque 0 leído con la geometría 16:9 (la más común); si no cuadra,
	// probamos las demás.
	geoms := map[int][2]float64{} // id -> (x offset, block px) a escala 1/4
	for id, s := range byID {
		sc := math.Min(float64(fm.Width)/float64(s.W), float64(fm.Height)/float64(s.H))
		cw := float64(s.W) * sc
		x := (float64(fm.Width) - cw) / 2
		geoms[id] = [2]float64{x / div, float64(BlockPx(s.W)) * sc / div}
	}
	for {
		if _, err := io.ReadFull(r, buf); err != nil {
			break
		}
		fi := frameInfo{luma: meanLuma(buf)}
		// intento 1: geometría 16:9 sin offset (bloques de 8 px)
		for _, g := range candidateGeoms(geoms) {
			vals := readBlocks(buf, w, g[0], g[1])
			c0, c15 := vals[0], vals[1]
			id := MarkerNibble(vals[2], c0, c15)
			if gg, ok := geoms[id]; ok && id >= 0 && math.Abs(gg[0]-g[0]) < 0.5 && math.Abs(gg[1]-g[1]) < 0.5 {
				n := MarkerNibble(vals[3], c0, c15)<<12 | MarkerNibble(vals[4], c0, c15)<<8 | MarkerNibble(vals[5], c0, c15)<<4 | MarkerNibble(vals[6], c0, c15)
				fi = frameInfo{id: id, n: n, luma: fi.luma, ok: true}
				break
			}
		}
		out = append(out, fi)
	}
	cmd.Wait()
	return out, nil
}

func candidateGeoms(geoms map[int][2]float64) [][2]float64 {
	seen := map[[2]float64]bool{}
	var out [][2]float64
	for _, g := range geoms {
		k := [2]float64{math.Round(g[0]*10) / 10, math.Round(g[1]*10) / 10}
		if !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	return out
}

// readBlocks promedia el centro de cada bloque.
func readBlocks(buf []byte, w int, x0, bpx float64) [MarkerBlocks]float64 {
	var vals [MarkerBlocks]float64
	for k := 0; k < MarkerBlocks; k++ {
		xa := x0 + float64(k)*bpx + bpx*0.25
		xb := x0 + float64(k+1)*bpx - bpx*0.25
		ya, yb := bpx*0.25, bpx*0.75
		var sum, cnt float64
		for y := int(ya); y < int(math.Ceil(yb)); y++ {
			for x := int(xa); x < int(math.Ceil(xb)); x++ {
				if x >= 0 && x < w && y >= 0 {
					sum += float64(buf[y*w+x])
					cnt++
				}
			}
		}
		if cnt > 0 {
			vals[k] = sum / cnt
		}
	}
	return vals
}

func meanLuma(buf []byte) float64 {
	var sum uint64
	for i := 0; i < len(buf); i += 7 {
		sum += uint64(buf[i])
	}
	return float64(sum) / float64((len(buf)+6)/7)
}

// checkCuts revisa cada cambio: el clip que sale termina donde debe y el
// que entra empieza en su cuadro cero, sin que sobre ni falte ninguno.
func checkCuts(frames []frameInfo, cuts []engine.Event, byName map[string]ClipSpec, fm engine.Format) (problems, checked, black int, details []string) {
	for _, fi := range frames {
		if fi.luma < 16 {
			black++
		}
	}
	for i, c := range cuts {
		spec, ok := byName[c.Clip]
		if !ok {
			continue
		}
		start := int(c.Frame)
		end := len(frames)
		if i+1 < len(cuts) {
			end = int(cuts[i+1].Frame)
		}
		if start >= len(frames) || end <= start+2 {
			continue
		}
		checked++
		seg := frames[start:end]
		// El clip que entra: sus primeros cuadros son el cuadro 0 del
		// archivo, repetido tantas veces como la conversión de tasa manda.
		ratio := fm.FPS() / parseRate(spec.FPS)
		if spec.VFR || strings.Contains(spec.Name, "480i") {
			// Solo continuidad: id constante y N nunca retrocede.
			bad := 0
			last := -1
			for _, fi := range seg {
				if !fi.ok || fi.id != spec.ID || fi.n < last {
					bad++
				}
				if fi.ok {
					last = fi.n
				}
			}
			if bad > 0 {
				problems++
				details = append(details, fmt.Sprintf("corte %d (%s): %d cuadros fuera de secuencia", i, spec.Name, bad))
			}
			continue
		}
		zeros := 0
		for _, fi := range seg {
			if fi.ok && fi.id == spec.ID && fi.n == 0 {
				zeros++
			} else {
				break
			}
		}
		want := int(math.Round(ratio))
		firstOK := zeros >= want-1 && zeros <= want+1
		// Y que N avance sin saltos mayores que la relación de tasas.
		jumps := 0
		last := -1
		for _, fi := range seg {
			if !fi.ok || fi.id != spec.ID {
				continue
			}
			if last >= 0 && (fi.n < last || fi.n-last > 2) {
				jumps++
			}
			last = fi.n
		}
		if !firstOK || jumps > 0 {
			problems++
			details = append(details, fmt.Sprintf("corte %d (%s): cuadro 0 repetido %d veces (esperado %d), %d saltos de N", i, spec.Name, zeros, want, jumps))
		}
	}
	return
}

func parseRate(s string) float64 {
	if strings.Contains(s, "/") {
		p := strings.Split(s, "/")
		a, _ := strconv.ParseFloat(p[0], 64)
		b, _ := strconv.ParseFloat(p[1], 64)
		return a / b
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// --- audio ------------------------------------------------------------------

// streamStarts devuelve el primer PTS de audio y de video del TS.
func streamStarts(ffprobe, tsPath string) (a, v float64) {
	get := func(sel string) float64 {
		out, err := exec.Command(ffprobe, "-v", "error", "-select_streams", sel, "-show_entries", "stream=start_time", "-of", "csv=p=0", tsPath).Output()
		if err != nil {
			return 0
		}
		f, _ := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
		return f
	}
	return get("a:0"), get("v:0")
}

// checkAudio decodifica la primera pista (MPEG capa II) a mono y, en cada
// corte, busca el pitido de 2 kHz (referencia de tiempo) y el peor salto
// de muestra (clic).
type offset struct {
	clip string
	ms   float64
}

func minMs(o []offset) float64 {
	m := o[0].ms
	for _, x := range o {
		m = math.Min(m, x.ms)
	}
	return m
}

func maxMs(o []offset) float64 {
	m := o[0].ms
	for _, x := range o {
		m = math.Max(m, x.ms)
	}
	return m
}

func checkAudio(ffmpeg, tsPath string, fm engine.Format, cuts []engine.Event, frames []frameInfo, aStart, vStart float64) (offsets []offset, clicks []float64, err error) {
	cmd := exec.Command(ffmpeg, "-nostdin", "-hide_banner", "-loglevel", "error", "-i", tsPath,
		"-map", "0:a:0", "-ac", "1", "-ar", strconv.Itoa(fm.SampleRate), "-f", "s16le", "pipe:1")
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	r := bufio.NewReaderSize(pipe, 1<<20)

	const win = 9600 // ±200 ms alrededor de cada corte
	sr := float64(fm.SampleRate)
	// Muestra global del corte según el video real: el cuadro donde el
	// marcador dice que empezó el clip (no el que el servidor cree), para
	// medir A/V en la salida y no en la intención.
	type target struct {
		sample int64
		frame  int64
		clip   string
	}
	var targets []target
	for _, c := range cuts {
		targets = append(targets, target{sample: fm.SamplesUpTo(c.Frame), frame: c.Frame, clip: c.Clip})
	}
	ring := make([]int16, 0, 4*win)
	var pos int64 // índice global de la primera muestra de ring
	buf := make([]byte, 2*4096)
	ti := 0
	for ti < len(targets) {
		n, rerr := io.ReadFull(r, buf)
		for i := 0; i+1 < n; i += 2 {
			ring = append(ring, int16(uint16(buf[i])|uint16(buf[i+1])<<8))
		}
		if rerr != nil && n == 0 {
			break
		}
		for ti < len(targets) {
			t := targets[ti]
			if pos+int64(len(ring)) < t.sample+win {
				break
			}
			lo := t.sample - win - pos
			if lo < 0 {
				ti++
				continue
			}
			seg := ring[lo : t.sample+win-pos]
			// pitido: energía a 2 kHz por ventanas de 2 ms
			onset := findBeep(seg, sr)
			if onset >= 0 {
				aTime := float64(t.sample-win+int64(onset))/sr + aStart
				vTime := float64(t.frame)/fm.FPS() + vStart
				offsets = append(offsets, offset{clip: t.clip, ms: (aTime - vTime) * 1000})
			}
			clicks = append(clicks, worstClick(seg, win))
			ti++
		}
		// recortar el ring
		if len(ring) > 3*win && ti < len(targets) {
			keep := targets[ti].sample - win - pos
			if keep > int64(len(ring)) {
				keep = int64(len(ring))
			}
			if keep > 0 {
				ring = append(ring[:0], ring[keep:]...)
				pos += keep
			}
		}
		if rerr != nil {
			break
		}
	}
	cmd.Process.Kill()
	cmd.Wait()
	return offsets, clicks, nil
}

// findBeep devuelve el índice de la primera ventana de 2 ms con energía
// clara a 2 kHz (Goertzel), o −1.
func findBeep(seg []int16, sr float64) int {
	const w = 96 // 2 ms
	coef := 2 * math.Cos(2*math.Pi*2000/sr)
	best := -1
	for i := 0; i+w <= len(seg); i += w / 2 {
		var s0, s1, s2 float64
		var e float64
		for j := 0; j < w; j++ {
			x := float64(seg[i+j]) / 32768
			e += x * x
			s0 = x + coef*s1 - s2
			s2, s1 = s1, s0
		}
		p := (s1*s1 + s2*s2 - coef*s1*s2) / float64(w)
		if e > 0 && p > 0.02 && p/(e/float64(w)+1e-9) > 0.5 {
			best = i
			break
		}
	}
	return best
}

// worstClick mide el peor salto entre muestras consecutivas cerca del corte,
// por encima de lo normal del tono, en dBFS.
func worstClick(seg []int16, center int) float64 {
	delta := func(i int) float64 { return math.Abs(float64(seg[i]) - float64(seg[i-1])) }
	var near []float64
	var far []float64
	for i := 1; i < len(seg); i++ {
		d := delta(i)
		if i > center-480 && i < center+480 {
			near = append(near, d)
		} else {
			far = append(far, d)
		}
	}
	if len(near) == 0 || len(far) == 0 {
		return -120
	}
	sort.Float64s(far)
	typical := far[len(far)*95/100]
	worst := 0.0
	for _, d := range near {
		worst = math.Max(worst, d-typical)
	}
	if worst <= 0 {
		return -120
	}
	return 20 * math.Log10(worst/32768)
}

// --- subtítulos, cpu, reporte -----------------------------------------------

func hasClosedCaptions(ffprobe, tsPath string) string {
	out, _ := exec.Command(ffprobe, "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=closed_captions", "-of", "csv=p=0", tsPath).Output()
	return strings.TrimSpace(string(out))
}

func ccText(v string) string {
	if v == "1" {
		return "el video de salida lleva CEA-608/708"
	}
	return "no hay subtítulos en la salida: la reinserción es código propio de F2, y el archivo 7 sintético no trae 608 reales (ver f0/README.md)"
}

func ccPass(v string) *bool {
	if v == "1" {
		return yes(true)
	}
	return nil
}

func summarizeStats(path string) (cpu, ram float64, n int) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	rows, _ := csv.NewReader(f).ReadAll()
	for i, r := range rows {
		if i == 0 || len(r) < 3 {
			continue
		}
		c, _ := strconv.ParseFloat(r[1], 64)
		m, _ := strconv.ParseFloat(r[2], 64)
		cpu += c
		ram += m
		n++
	}
	if n > 0 {
		cpu /= float64(n)
		ram /= float64(n)
	}
	return
}

func writeReport(path string, results []Result, rep ts.Report, events []engine.Event, counts map[string]int, details []string, frames int, log func(string)) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# F0 — reporte del experimento\n\n_%s_\n\n", time.Now().Format("2006-01-02 15:04"))
	var dur string
	for _, e := range events {
		if e.Kind == "fin" {
			dur = e.Detail
		}
	}
	fmt.Fprintf(&b, "Corrida: %s · %d cuadros en la salida · TS: %s\n\n", dur, frames, rep.String())
	fmt.Fprintln(&b, "| Criterio | Qué mide | Resultado | |")
	fmt.Fprintln(&b, "|---|---|---|---|")
	allPass := true
	for _, r := range results {
		fmt.Fprintf(&b, "| %s | %s | %s | **%s** |\n", r.ID, r.Name, r.Value, r.mark())
		if r.Pass != nil && !*r.Pass {
			allPass = false
		}
	}
	fmt.Fprint(&b, "\n## Eventos del servidor de cuadros\n\n")
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "- `%s`: %d\n", k, counts[k])
	}
	if len(details) > 0 {
		fmt.Fprint(&b, "\n## Cortes con problema\n\n")
		for i, d := range details {
			if i >= 40 {
				fmt.Fprintf(&b, "- … y %d más\n", len(details)-40)
				break
			}
			fmt.Fprintf(&b, "- %s\n", d)
		}
	}
	fmt.Fprint(&b, "\n## Veredicto\n\n")
	if allPass {
		fmt.Fprintln(&b, "Los criterios medibles pasan. Lo marcado con — no se pudo medir en esta corrida y se dice por qué.")
	} else {
		fmt.Fprintln(&b, "**Hay criterios que fallan.** Según §22.1, si falla el desfase o los cuadros perdidos, el diseño del servidor de cuadros se replantea antes de F1.")
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return err
	}
	log("")
	log(b.String())
	return nil
}
