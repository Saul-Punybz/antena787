package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"antena787/internal/ingest"
	"antena787/internal/model"
)

// F1-46 — El aviso de los siete días sale por el canal configurado. Aquí se
// comprueba el camino de Telegram entero contra un servidor de mentira: la
// ruta que se llama, lo que se le manda y a quién.
func TestF1_46_ElAvisoSalePorTelegram(t *testing.T) {
	tipo := make(chan map[string]string, 1)
	ruta := make(chan string, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ruta <- r.URL.Path
		cuerpo, _ := io.ReadAll(r.Body)
		var m map[string]string
		_ = json.Unmarshal(cuerpo, &m)
		tipo <- m
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	cfg := avisoConfig{Canal: CanalTelegram, Token: "123:abc", Chat: "-100777"}
	err := enviarPorTelegram(context.Background(), srv.URL, cfg,
		TextoDeAviso("Antena787 · una regla se acaba", "«Zoids» se acaba en 7 días"))
	if err != nil {
		t.Fatalf("el aviso por Telegram falló: %v", err)
	}

	if got := <-ruta; got != "/bot123:abc/sendMessage" {
		t.Fatalf("el aviso se mandó a %q", got)
	}
	m := <-tipo
	if m["chat_id"] != "-100777" {
		t.Fatalf("el aviso salió al chat %q", m["chat_id"])
	}
	if !strings.Contains(m["text"], "Zoids") || !strings.Contains(m["text"], "7 días") {
		t.Fatalf("el texto del aviso no dice lo que tiene que decir: %q", m["text"])
	}
}

// Un servidor que contesta mal no rompe nada: devuelve error, y quien llama
// lo convierte en alarma.
func TestF1_46_TelegramQueFallaSoloDaError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"ok":false,"description":"chat not found"}`, http.StatusBadRequest)
	}))
	defer srv.Close()

	cfg := avisoConfig{Canal: CanalTelegram, Token: "123:abc", Chat: "nadie"}
	err := enviarPorTelegram(context.Background(), srv.URL, cfg, "hola")
	if err == nil {
		t.Fatal("un rechazo de Telegram tiene que devolver error")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Fatalf("el error no dice qué contestó: %v", err)
	}
}

// Sin canal configurado no se manda nada y nadie se entera: Notificar es una
// operación silenciosa.
func TestF1_46_SinCanalNoSeMandaNada(t *testing.T) {
	a := abre(t)
	viejo := telegramAPI
	llamadas := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		llamadas++
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	telegramAPI = srv.URL
	defer func() { telegramAPI = viejo }()

	a.Notificar(context.Background(), "asunto", "texto")
	if llamadas != 0 {
		t.Fatalf("con el canal de avisos apagado se mandaron %d avisos", llamadas)
	}
	if len(a.Alarms()) != 0 {
		t.Fatalf("no mandar nada no es una alarma: %+v", a.Alarms())
	}
}

// Con Telegram encendido en los ajustes, Notificar sale por él; si el otro
// lado contesta mal, queda una alarma de nivel aviso y nada más.
func TestF1_46_NotificarUsaLosAjustes(t *testing.T) {
	a := abre(t)
	ctx := context.Background()
	llegó := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		llegó <- struct{}{}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	viejo := telegramAPI
	telegramAPI = srv.URL
	defer func() { telegramAPI = viejo }()

	for k, v := range map[string]string{
		KeyNoticeChannel: CanalTelegram,
		KeyTelegramToken: "123:abc",
		KeyTelegramChat:  "42",
	} {
		if err := a.Store.Settings.Set(ctx, k, v); err != nil {
			t.Fatalf("no pude guardar %s: %v", k, err)
		}
	}
	a.Notificar(ctx, "Antena787", "una regla se acaba")
	select {
	case <-llegó:
	case <-time.After(3 * time.Second):
		t.Fatal("el aviso no salió por Telegram con el ajuste puesto")
	}

	// Ahora con el servidor caído: alarma de aviso, y nada más.
	srv.Close()
	a.Notificar(ctx, "Antena787", "otra regla se acaba")
	alarmas := a.Alarms()
	if len(alarmas) == 0 {
		t.Fatal("un aviso que no sale tiene que dejar alarma")
	}
	if alarmas[0].Nivel != NivelAviso {
		t.Fatalf("la alarma de un aviso que no sale es de nivel %q", alarmas[0].Nivel)
	}
	if !strings.Contains(alarmas[0].Texto, "no pude enviar el aviso por Telegram") {
		t.Fatalf("la alarma no dice qué pasó: %q", alarmas[0].Texto)
	}
}

// El correo se arma con las cabeceras que hacen falta y el texto en UTF-8.
func TestF1_46_ElCorreoSeArmaBien(t *testing.T) {
	crudo := string(CorreoCrudo("antena787@correo.local", "rolando@catv.pr",
		"Antena787 · una regla se acaba", "«Zoids» se acaba en 7 días.\nRenuévala o pon quién la releva."))

	for _, quiero := range []string{
		"From: antena787@correo.local\r\n",
		"To: rolando@catv.pr\r\n",
		"Subject: Antena787 · una regla se acaba\r\n",
		"Content-Type: text/plain; charset=utf-8\r\n",
		"\r\n\r\n«Zoids» se acaba en 7 días.\r\nRenuévala o pon quién la releva.\r\n",
	} {
		if !strings.Contains(crudo, quiero) {
			t.Fatalf("al correo le falta %q:\n%s", quiero, crudo)
		}
	}
	// Un asunto con salto de línea no puede partir las cabeceras.
	con := string(CorreoCrudo("a@b", "c@d", "uno\r\ndos", "hola"))
	if strings.Count(strings.SplitN(con, "\r\n\r\n", 2)[0], "\r\n") != 5 {
		t.Fatalf("un asunto con salto de línea rompió las cabeceras:\n%s", con)
	}
}

func TestTextoDeAviso(t *testing.T) {
	if got := TextoDeAviso("Asunto", "Cuerpo"); got != "Asunto\n\nCuerpo" {
		t.Fatalf("el mensaje quedó %q", got)
	}
	if got := TextoDeAviso("", "Cuerpo"); got != "Cuerpo" {
		t.Fatalf("sin asunto el mensaje quedó %q", got)
	}
	if got := TextoDeAviso("Asunto", ""); got != "Asunto" {
		t.Fatalf("sin cuerpo el mensaje quedó %q", got)
	}
}

// F1-49 — El envío opcional de la guía a un destino externo: sale el XML,
// con su tipo de contenido, y que falle no toca ni la guía local ni el aire.
func TestF1_49_LaGuiaSeMandaAlDestino(t *testing.T) {
	a := abre(t)
	ctx := context.Background()

	tipo := make(chan string, 1)
	cuerpo := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tipo <- r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		cuerpo <- string(b)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	if err := a.Store.Settings.Set(ctx, KeyGuideHTTP, srv.URL); err != nil {
		t.Fatalf("no pude guardar el destino: %v", err)
	}
	conPrograma(t, a, "Kojak", "14:00")
	if _, err := a.Resolve(ctx); err != nil {
		t.Fatalf("la corrida falló: %v", err)
	}

	select {
	case ct := <-tipo:
		if !strings.Contains(ct, "application/xml") {
			t.Fatalf("la guía se mandó como %q", ct)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("la guía no llegó al destino configurado")
	}
	xml := <-cuerpo
	if !strings.Contains(xml, "<tv ") || !strings.Contains(xml, "Kojak") {
		t.Fatalf("lo que llegó al destino no es la guía:\n%s", xml)
	}
	local, _ := a.Guide()
	if string(local) != xml {
		t.Fatal("lo que se mandó no es la misma guía que se publicó en casa")
	}
}

// Un destino que no contesta deja alarma y una línea en la bitácora, y la
// guía local se publica igual: es lo que pide F1-49.
func TestF1_49_UnDestinoCaidoNoRompeLaGuia(t *testing.T) {
	a := abre(t)
	ctx := context.Background()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no", http.StatusInternalServerError)
	}))
	destino := srv.URL
	defer srv.Close()

	if err := a.Store.Settings.Set(ctx, KeyGuideHTTP, destino); err != nil {
		t.Fatalf("no pude guardar el destino: %v", err)
	}
	conPrograma(t, a, "Tarzan", "09:00")
	if _, err := a.Resolve(ctx); err != nil {
		t.Fatalf("la corrida falló aunque el destino no conteste: %v", err)
	}
	if g, _ := a.Guide(); len(g) == 0 {
		t.Fatal("la guía local no se publicó porque el destino externo falló")
	}
	esperar(t, 5*time.Second, func() bool {
		for _, al := range a.Alarms() {
			if strings.Contains(al.Texto, "no pude mandar la guía") {
				return true
			}
		}
		return false
	}, "un destino de guía caído tiene que dejar alarma")
}

// Comprobación directa de enviarGuia: el error trae lo que contestó el otro
// lado, para que la alarma diga algo útil.
func TestElEnvioDeLaGuiaCuentaQuePaso(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no me gusta", http.StatusForbidden)
	}))
	defer srv.Close()

	err := enviarGuia(srv.URL, []byte("<tv/>"))
	if err == nil {
		t.Fatal("un 403 tiene que devolver error")
	}
	if !strings.Contains(err.Error(), "403") || !strings.Contains(err.Error(), "no me gusta") {
		t.Fatalf("el error no cuenta qué pasó: %v", err)
	}
}

// F1-03 — El reporte de volumen deja de morirse en memoria: lo que midió la
// segunda pasada queda escrito en el archivo de la biblioteca.
func TestF1_03_ElVolumenMedidoQuedaGuardado(t *testing.T) {
	a := abre(t)
	ctx := context.Background()

	asset := model.MediaAsset{
		Path: "/contenido/pelicula.mkv", DurationMs: 90 * 60 * 1000,
		State: model.AssetReady, NormalizeState: ingest.NormalizeReady,
	}
	if err := a.Store.Media.Insert(ctx, &asset); err != nil {
		t.Fatalf("no pude guardar el archivo: %v", err)
	}

	p := &persist{a: a}
	// La cola llama a SaveLoudness con el reporte que devolvió Normalize.
	guardó, err := ingest.SaveLoudness(ctx, p, asset.ID, ingest.LoudnessReport{
		OutputLUFS: -23.8, OutputTruePeak: -2.4, Passes: 2,
	})
	if err != nil {
		t.Fatalf("guardar el reporte falló: %v", err)
	}
	if !guardó {
		t.Fatal("la persistencia de la aplicación tiene que saber guardar el volumen")
	}

	leído, err := a.Store.Media.Get(ctx, asset.ID)
	if err != nil {
		t.Fatalf("no pude releer el archivo: %v", err)
	}
	if leído.LUFS == nil || *leído.LUFS != -23.8 {
		t.Fatalf("el volumen medido no se guardó: %v", leído.LUFS)
	}
	if leído.TruePeak == nil || *leído.TruePeak != -2.4 {
		t.Fatalf("el pico medido no se guardó: %v", leído.TruePeak)
	}

	// Un archivo sin sonido no se mide, y entonces no se escribe un cero que
	// parecería una medición.
	mudo := model.MediaAsset{Path: "/contenido/mudo.mkv", State: model.AssetReady}
	if err := a.Store.Media.Insert(ctx, &mudo); err != nil {
		t.Fatalf("no pude guardar el archivo mudo: %v", err)
	}
	if _, err := ingest.SaveLoudness(ctx, p, mudo.ID, ingest.LoudnessReport{Passes: 0}); err != nil {
		t.Fatalf("guardar el reporte del mudo falló: %v", err)
	}
	leído, err = a.Store.Media.Get(ctx, mudo.ID)
	if err != nil {
		t.Fatalf("no pude releer el archivo mudo: %v", err)
	}
	if leído.LUFS != nil {
		t.Fatalf("un archivo sin sonido salió con volumen medido: %v", *leído.LUFS)
	}
}

// F1-09 — Los servicios de fichas se encienden desde los ajustes y no antes.
func TestF1_09_LasFichasEnLineaSeEnciendenDesdeLosAjustes(t *testing.T) {
	a := abre(t)
	ctx := context.Background()

	if p := a.fichasEnLinea(ctx); len(p) != 0 {
		t.Fatalf("de fábrica no se consulta nada por internet, y salieron %d servicios", len(p))
	}
	if err := a.Store.Settings.Set(ctx, KeyOnlineInfo, "si"); err != nil {
		t.Fatalf("no pude guardar el ajuste: %v", err)
	}
	sinClave := a.fichasEnLinea(ctx)
	if len(sinClave) != 2 {
		t.Fatalf("sin clave se consultan los dos servicios que no la piden, y salieron %d", len(sinClave))
	}
	if err := a.Store.Settings.Set(ctx, KeyTMDBKey, "una-clave"); err != nil {
		t.Fatalf("no pude guardar la clave: %v", err)
	}
	conClave := a.fichasEnLinea(ctx)
	if len(conClave) != 3 {
		t.Fatalf("con clave se consultan tres servicios, y salieron %d", len(conClave))
	}

	// Y el ingest los recibe.
	if a.FFmpeg == "" || a.FFprobe == "" {
		t.Skip("sin ffmpeg no se puede armar el ingest")
	}
	deps, err := a.ingestDeps(ctx)
	if err != nil {
		t.Fatalf("no pude armar el ingest: %v", err)
	}
	if len(deps.Providers) != 3 {
		t.Fatalf("el ingest recibió %d servicios de fichas", len(deps.Providers))
	}
}
