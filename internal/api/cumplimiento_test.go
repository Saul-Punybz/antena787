package api

import (
	"context"
	"net/http"
	"testing"

	"antena787/internal/app"
)

// El ajuste de tres estados de subtítulos, perfil us-fcc (PRD §12,
// COMPLIANCE.md, F1-77).

// conPerfil pone el perfil regulatorio del canal directamente, sin pasar
// por el asistente: lo que aquí importa es el efecto del perfil, no cómo se
// llegó a él.
func (c *cliente) conPerfil(perfil string) *cliente {
	c.t.Helper()
	ctx := context.Background()
	ch, err := c.a.Store.Channel.Get(ctx, c.a.ChannelID)
	if err != nil {
		c.t.Fatalf("no pude leer el canal: %v", err)
	}
	ch.RegProfile = perfil
	if err := c.a.Store.Channel.Update(ctx, ch); err != nil {
		c.t.Fatalf("no pude poner el perfil regulatorio: %v", err)
	}
	return c
}

// textoSinDecidir es el texto exacto de la alarma, tal como lo enseña
// AlarmaSubtitulos (internal/app/cumplimiento.go). tieneAlarma (definida en
// contrato_test.go) compara por ese texto.
const textoSinDecidir = "Todavía no dijiste si el canal está obligado a subtitular: dilo en Ajustes → Cumplimiento"

// Los tres valores válidos se guardan tal cual; cualquier otro se rechaza en
// cristiano y no toca lo que ya había.
func TestAjustesSubtitulosValoresValidos(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	for _, v := range []string{"obligada", "exenta", "no_se"} {
		w := c.do("PUT", "/api/v1/ajustes", map[string]string{app.KeySubtitulosEstado: v})
		if w.Code != http.StatusOK {
			t.Fatalf("guardar %q dio %d: %s", v, w.Code, w.Body.String())
		}
		if got := c.setting(app.KeySubtitulosEstado); got != v {
			t.Fatalf("se guardó %q, quedó %q", v, got)
		}
	}
}

func TestAjustesSubtitulosValorInvalido(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	// Deja un valor válido puesto para comprobar que el rechazo no lo toca.
	if w := c.do("PUT", "/api/v1/ajustes", map[string]string{app.KeySubtitulosEstado: "obligada"}); w.Code != http.StatusOK {
		t.Fatalf("el valor válido dio %d: %s", w.Code, w.Body.String())
	}
	w := c.do("PUT", "/api/v1/ajustes", map[string]string{app.KeySubtitulosEstado: "quizás"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("un valor que no es de los tres dio %d, se esperaba 400", w.Code)
	}
	var e errorBody
	c.json(w, &e)
	if e.Error == "" {
		t.Fatal("el rechazo no viene con un mensaje en cristiano")
	}
	if got := c.setting(app.KeySubtitulosEstado); got != "obligada" {
		t.Fatalf("el rechazo tocó el valor que ya había: quedó %q", got)
	}
}

// La alarma solo existe con el perfil us-fcc y el ajuste en "no_se" (el
// default), y se va sola en cuanto se decide.
func TestAlarmaSubtitulosApareceYDesaparece(t *testing.T) {
	c := nuevo(t).conClave().entrar().conPerfil("us-fcc")

	// Nadie ha puesto el ajuste todavía: cuenta como "no_se".
	if !tieneAlarma(c, textoSinDecidir) {
		t.Fatal("con el ajuste sin decidir, /estado tenía que traer la alarma")
	}

	if w := c.do("PUT", "/api/v1/ajustes", map[string]string{app.KeySubtitulosEstado: "exenta"}); w.Code != http.StatusOK {
		t.Fatalf("guardar 'exenta' dio %d: %s", w.Code, w.Body.String())
	}
	if tieneAlarma(c, textoSinDecidir) {
		t.Fatal("tras decidir 'exenta', la alarma tenía que haberse ido sola")
	}

	// Volver a "no_se" la trae de vuelta.
	if w := c.do("PUT", "/api/v1/ajustes", map[string]string{app.KeySubtitulosEstado: "no_se"}); w.Code != http.StatusOK {
		t.Fatalf("guardar 'no_se' dio %d: %s", w.Code, w.Body.String())
	}
	if !tieneAlarma(c, textoSinDecidir) {
		t.Fatal("al volver a 'no_se', la alarma tenía que reaparecer")
	}
}

// Fuera del perfil us-fcc la alarma no existe, decida lo que decida el
// ajuste: el ajuste ni se enseña fuera de ese perfil.
func TestAlarmaSubtitulosNoExisteFueraDeUSFCC(t *testing.T) {
	c := nuevo(t).conClave().entrar().conPerfil("abierto")
	if tieneAlarma(c, textoSinDecidir) {
		t.Fatal("fuera de us-fcc no debería avisar de subtítulos")
	}
}
