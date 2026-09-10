package app

// El ajuste de tres estados de los subtítulos, perfil us-fcc (PRD §12,
// COMPLIANCE.md, sección Subtítulos). No decide nada por la estación: los
// subtítulos que traiga un archivo se conservan y se pueden subir siempre,
// las tres respuestas funcionan igual. Lo único que cambia el ajuste es si
// /estado avisa cuando el canal sigue sin decidir.
//
// Se calcula al vuelo cada vez que alguien pide /estado, en vez de guardarse
// en un caché que haya que refrescar a mano desde cada sitio que puede
// cambiar el país o el ajuste: leer un settings y un canal es barato, y así
// la alarma nunca queda desactualizada.

import "context"

const (
	// KeySubtitulosEstado es el ajuste de tres estados. Vacío se lee igual
	// que SubtitulosNoSe (el default): "no sé" no bloquea nada y nadie tiene
	// que resolverlo antes de salir al aire.
	KeySubtitulosEstado = "subtitulos_estado"

	// SubtitulosObligada: la estación entiende que le toca rotular.
	SubtitulosObligada = "obligada"
	// SubtitulosExenta: la estación está exenta (el motivo se anota aparte:
	// ingresos, canal nuevo, programación exenta — COMPLIANCE.md).
	SubtitulosExenta = "exenta"
	// SubtitulosNoSe es el default: todavía no se ha decidido.
	SubtitulosNoSe = "no_se"
)

// SubtitulosEstadoValido dice si v es uno de los tres estados. Cualquier
// otro valor se rechaza en PUT /ajustes: no hay un cuarto estado.
func SubtitulosEstadoValido(v string) bool {
	switch v {
	case SubtitulosObligada, SubtitulosExenta, SubtitulosNoSe:
		return true
	default:
		return false
	}
}

// AlarmaSubtitulos es la alarma «todavía no dijiste si el canal está
// obligado a subtitular», y solo existe cuando hacen falta las dos cosas a
// la vez: el perfil es us-fcc (fuera de ese perfil el ajuste ni se enseña) y
// el ajuste sigue en "no sé". Devuelve nil en cualquier otro caso, incluido
// el error al leer el canal: una alarma de cumplimiento nunca puede ser la
// razón de que /estado falle.
func (a *App) AlarmaSubtitulos(ctx context.Context) *Alarma {
	ch, err := a.Store.Channel.Get(ctx, a.ChannelID)
	if err != nil || ch.RegProfile != "us-fcc" {
		return nil
	}
	v := a.setting(ctx, KeySubtitulosEstado)
	if v == "" {
		v = SubtitulosNoSe
	}
	if v != SubtitulosNoSe {
		return nil
	}
	return &Alarma{
		Tipo:  "subtitulos_sin_decidir",
		Nivel: NivelAviso,
		Texto: "Todavía no dijiste si el canal está obligado a subtitular: dilo en Ajustes → Cumplimiento",
		Accion: &AccionAlarma{
			Texto: "ir a Ajustes",
			Ruta:  "/ajustes",
		},
	}
}
