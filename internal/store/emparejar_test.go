package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"antena787/internal/model"
)

// ── ayudas ────────────────────────────────────────────────────────────

// titulo mete un título en el catálogo y devuelve su id.
func titulo(t *testing.T, s *Store, nombre string, pendiente bool, candidatos ...int64) *model.Title {
	t.Helper()
	canal := DefaultChannelID
	x := &model.Title{
		ChannelID:          &canal,
		Name:               nombre,
		Kind:               model.TitleSeries,
		PendienteEmparejar: pendiente,
		Candidatos:         candidatos,
	}
	if err := s.Title.Insert(context.Background(), x); err != nil {
		t.Fatalf("no se pudo guardar el título %q: %v", nombre, err)
	}
	return x
}

// regla mete una regla que usa ese título y devuelve su id.
func regla(t *testing.T, s *Store, titleID int64, hora model.Minutes) *model.ScheduleRule {
	t.Helper()
	r := &model.ScheduleRule{
		ChannelID: DefaultChannelID, Kind: model.RuleNormal, TitleID: &titleID,
		Days: "LMMJV__", At: hora, SlotMs: 30 * 60 * 1000,
		From: "2026-09-01", To: "2026-12-31", Active: true,
	}
	if err := s.Rule.Insert(context.Background(), r); err != nil {
		t.Fatalf("no se pudo guardar la regla: %v", err)
	}
	return r
}

// sinReferenciasSueltas comprueba que la base no quedó con llaves foráneas
// apuntando a nada.
func sinReferenciasSueltas(t *testing.T, s *Store) {
	t.Helper()
	rows, err := s.DB().QueryContext(context.Background(), `PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatalf("no se pudo comprobar las llaves foráneas: %v", err)
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		t.Fatal("quedaron referencias apuntando a nada después de emparejar")
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

// ── el esquema nuevo ──────────────────────────────────────────────────

// F1-64: un título creado por el importador se guarda marcado por emparejar
// y con sus candidatos, y vuelve igual de la base.
func TestTituloPorEmparejarVaYVuelve(t *testing.T) {
	ctx := context.Background()
	s, _ := abrir(t)

	j := titulo(t, s, "Saber Marionette J", false)
	r := titulo(t, s, "Saber Marionette R", false)
	pendiente := titulo(t, s, "SaberMarionette", true, j.ID, r.ID)

	leido, err := s.Title.Get(ctx, pendiente.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !leido.PendienteEmparejar {
		t.Fatal("el título no quedó marcado por emparejar")
	}
	if len(leido.Candidatos) != 2 || leido.Candidatos[0] != j.ID || leido.Candidatos[1] != r.ID {
		t.Fatalf("los candidatos volvieron mal: %v", leido.Candidatos)
	}

	// Y una ficha normal ni está pendiente ni trae candidatos.
	normal, err := s.Title.FindByName(ctx, "saber marionette j")
	if err != nil {
		t.Fatal(err)
	}
	if normal.PendienteEmparejar || len(normal.Candidatos) != 0 {
		t.Fatalf("una ficha del catálogo no debería estar pendiente: %+v", normal)
	}

	// Update también los guarda.
	leido.Candidatos = []int64{j.ID}
	if err := s.Title.Update(ctx, &leido); err != nil {
		t.Fatal(err)
	}
	otra, err := s.Title.Get(ctx, pendiente.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(otra.Candidatos) != 1 || otra.Candidatos[0] != j.ID {
		t.Fatalf("Update no guardó los candidatos: %v", otra.Candidatos)
	}
}

func TestListPendientesSoloTraeLosQueFaltan(t *testing.T) {
	ctx := context.Background()
	s, _ := abrir(t)

	titulo(t, s, "Rurouni Kenshin", false)
	titulo(t, s, "Samurai X", true)
	titulo(t, s, "Los Simuladores", true)

	pendientes, err := s.Title.ListPendientes(ctx, DefaultChannelID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pendientes) != 2 {
		t.Fatalf("se esperaban 2 títulos por emparejar, salieron %d: %v", len(pendientes), pendientes)
	}
	if pendientes[0].Name != "Los Simuladores" || pendientes[1].Name != "Samurai X" {
		t.Fatalf("no salieron por nombre: %q, %q", pendientes[0].Name, pendientes[1].Name)
	}
}

// ── los alias ─────────────────────────────────────────────────────────

// F1-66: el alias se busca por la clave del nombre, no por su ortografía.
func TestResolveEncuentraElAliasComoSeaQueSeEscriba(t *testing.T) {
	ctx := context.Background()
	s, _ := abrir(t)

	kenshin := titulo(t, s, "Rurouni Kenshin", false)
	canal := DefaultChannelID
	if err := s.Alias.Insert(ctx, &model.TitleAlias{
		ChannelID: &canal, Alias: "Samurai X", TitleID: kenshin.ID,
	}); err != nil {
		t.Fatal(err)
	}

	for _, escrito := range []string{"Samurai X", "SAMURAIX", "samurai-x", " samurai  x "} {
		id, ok, err := s.Alias.Resolve(ctx, DefaultChannelID, escrito)
		if err != nil {
			t.Fatal(err)
		}
		if !ok || id != kenshin.ID {
			t.Fatalf("«%s» no resolvió a la ficha (%d, %v)", escrito, id, ok)
		}
	}

	// Lo que nadie enseñó no se inventa.
	if _, ok, err := s.Alias.Resolve(ctx, DefaultChannelID, "Los Simuladores"); err != nil || ok {
		t.Fatalf("resolvió un nombre que nadie enseñó (%v, %v)", ok, err)
	}
	// Ni un nombre que no deja clave.
	if _, ok, err := s.Alias.Resolve(ctx, DefaultChannelID, "!!!"); err != nil || ok {
		t.Fatalf("un nombre sin letras no puede resolver nada (%v, %v)", ok, err)
	}
}

// Un mismo nombre no puede apuntar a dos fichas en el mismo canal: aprenderlo
// otra vez lo reapunta, no lo duplica.
func TestAliasEsUnoPorCanalYSeReapunta(t *testing.T) {
	ctx := context.Background()
	s, _ := abrir(t)

	uno := titulo(t, s, "Rurouni Kenshin", false)
	dos := titulo(t, s, "Kenshin: el guerrero samurái", false)
	canal := DefaultChannelID

	a := &model.TitleAlias{ChannelID: &canal, Alias: "Samurai X", TitleID: uno.ID}
	if err := s.Alias.Insert(ctx, a); err != nil {
		t.Fatal(err)
	}
	b := &model.TitleAlias{ChannelID: &canal, Alias: "samurai-x", TitleID: dos.ID}
	if err := s.Alias.Insert(ctx, b); err != nil {
		t.Fatalf("volver a aprender el mismo nombre tenía que reapuntarlo: %v", err)
	}
	if b.ID != a.ID {
		t.Fatalf("se creó un alias nuevo (%d) en vez de reapuntar el que había (%d)", b.ID, a.ID)
	}

	todos, err := s.Alias.ListByChannel(ctx, DefaultChannelID)
	if err != nil {
		t.Fatal(err)
	}
	if len(todos) != 1 {
		t.Fatalf("quedaron %d alias con la misma clave: %v", len(todos), todos)
	}
	if todos[0].Alias != "samurai-x" || todos[0].TitleID != dos.ID {
		t.Fatalf("el alias no quedó reapuntado: %+v", todos[0])
	}
	if todos[0].Clave != model.ClaveDeNombre("Samurai X") {
		t.Fatalf("la clave se guardó mal: %q", todos[0].Clave)
	}
	if todos[0].Creado.IsZero() {
		t.Fatal("el alias no guardó cuándo se aprendió")
	}

	// El crudo también lo impide: la restricción está en el esquema.
	_, err = s.DB().ExecContext(ctx, `
		INSERT INTO title_alias (channel_id, alias, clave, title_id, creado)
		VALUES (?, 'Samurai X', ?, ?, '2026-09-09T00:00:00Z')`,
		DefaultChannelID, model.ClaveDeNombre("Samurai X"), uno.ID)
	if err == nil {
		t.Fatal("el esquema aceptó dos alias con la misma clave en el mismo canal")
	}

	// Por título y borrar.
	porTitulo, err := s.Alias.ListByTitle(ctx, dos.ID)
	if err != nil || len(porTitulo) != 1 {
		t.Fatalf("ListByTitle devolvió %v (%v)", porTitulo, err)
	}
	if err := s.Alias.Delete(ctx, porTitulo[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Alias.Delete(ctx, porTitulo[0].ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("borrar dos veces tenía que dar ErrNotFound, salió: %v", err)
	}
}

// Un alias vale mientras viva su ficha: si la ficha se va, el alias también.
func TestElAliasSeVaConSuFicha(t *testing.T) {
	ctx := context.Background()
	s, _ := abrir(t)

	x := titulo(t, s, "Los Simuladores", true)
	canal := DefaultChannelID
	if err := s.Alias.Insert(ctx, &model.TitleAlias{
		ChannelID: &canal, Alias: "Simuladores", TitleID: x.ID,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Title.QuitarProvisional(ctx, x.ID); err != nil {
		t.Fatal(err)
	}
	todos, err := s.Alias.ListByChannel(ctx, DefaultChannelID)
	if err != nil {
		t.Fatal(err)
	}
	if len(todos) != 0 {
		t.Fatalf("el alias sobrevivió a su ficha: %v", todos)
	}
	sinReferenciasSueltas(t, s)
}

// ── emparejar ─────────────────────────────────────────────────────────

// F1-66: las reglas pasan a la ficha, el provisional desaparece y el nombre
// de la hoja queda aprendido.
func TestEmparejarMueveLasReglasYAprendeElAlias(t *testing.T) {
	ctx := context.Background()
	s, _ := abrir(t)

	kenshin := titulo(t, s, "Rurouni Kenshin", false)
	samurai := titulo(t, s, "Samurai X", true, kenshin.ID)
	r1 := regla(t, s, samurai.ID, 20*60)
	r2 := regla(t, s, samurai.ID, 21*60)
	otra := regla(t, s, kenshin.ID, 22*60)

	movidas, err := s.Title.Emparejar(ctx, samurai.ID, kenshin.ID)
	if err != nil {
		t.Fatalf("no emparejó: %v", err)
	}
	if movidas != 2 {
		t.Fatalf("movió %d reglas, se esperaban 2", movidas)
	}

	for _, id := range []int64{r1.ID, r2.ID, otra.ID} {
		leida, err := s.Rule.Get(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if leida.TitleID == nil || *leida.TitleID != kenshin.ID {
			t.Fatalf("la regla %d no quedó en la ficha: %v", id, leida.TitleID)
		}
	}

	// El provisional ya no está.
	if _, err := s.Title.Get(ctx, samurai.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("el título provisional sobrevivió: %v", err)
	}
	// Y no quedó nada colgando.
	sinReferenciasSueltas(t, s)

	// La próxima hoja que traiga ese nombre se empareja sola.
	id, ok, err := s.Alias.Resolve(ctx, DefaultChannelID, "SamuraiX")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || id != kenshin.ID {
		t.Fatalf("no aprendió el alias (%d, %v)", id, ok)
	}

	// Y quedó anotado en la bitácora, con la cadena sana.
	entradas, err := s.Audit.List(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(entradas) == 0 {
		t.Fatal("emparejar no dejó nada en la bitácora")
	}
	ultima := entradas[len(entradas)-1]
	if ultima.Entity != "title" || !strings.Contains(ultima.After, "emparejado con «Rurouni Kenshin»") {
		t.Fatalf("la bitácora no cuenta lo que pasó: %+v", ultima)
	}
	if !strings.Contains(ultima.After, "2 reglas") {
		t.Fatalf("la bitácora no dice cuántas reglas se movieron: %q", ultima.After)
	}
	if rota, err := s.Audit.Verify(ctx); err != nil || rota != nil {
		t.Fatalf("la cadena de la bitácora quedó rota: %v (%v)", rota, err)
	}
}

// Los episodios del provisional se van con las reglas.
func TestEmparejarSeLlevaLosEpisodios(t *testing.T) {
	ctx := context.Background()
	s, _ := abrir(t)

	kenshin := titulo(t, s, "Rurouni Kenshin", false)
	samurai := titulo(t, s, "Samurai X", true)
	for _, n := range []int{1, 2} {
		if err := s.Episode.Insert(ctx, &model.Episode{TitleID: samurai.ID, Season: 1, Number: n}); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := s.Title.Emparejar(ctx, samurai.ID, kenshin.ID); err != nil {
		t.Fatal(err)
	}
	eps, err := s.Episode.ListByTitle(ctx, kenshin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 2 {
		t.Fatalf("la ficha se quedó con %d episodios, se esperaban 2", len(eps))
	}
	sinReferenciasSueltas(t, s)
}

func TestEmparejarSeQuejaBien(t *testing.T) {
	ctx := context.Background()
	s, _ := abrir(t)

	kenshin := titulo(t, s, "Rurouni Kenshin", false)
	samurai := titulo(t, s, "Samurai X", true)
	otroPendiente := titulo(t, s, "SaberMarionette", true)

	if _, err := s.Title.Emparejar(ctx, samurai.ID, samurai.ID); !errors.Is(err, ErrMismoTitulo) {
		t.Fatalf("se esperaba ErrMismoTitulo, salió: %v", err)
	}
	if _, err := s.Title.Emparejar(ctx, 9999, kenshin.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("se esperaba ErrNotFound, salió: %v", err)
	}
	if _, err := s.Title.Emparejar(ctx, samurai.ID, 9999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("se esperaba ErrNotFound, salió: %v", err)
	}
	if _, err := s.Title.Emparejar(ctx, samurai.ID, otroPendiente.ID); !errors.Is(err, ErrDestinoPendiente) {
		t.Fatalf("se esperaba ErrDestinoPendiente, salió: %v", err)
	}
	// Y después de cada negativa no se movió nada.
	if _, err := s.Title.Get(ctx, samurai.ID); err != nil {
		t.Fatalf("el provisional tenía que seguir ahí: %v", err)
	}
}

// ── es un título nuevo / no es un programa ────────────────────────────

// F1-67, primer caso: se queda como ficha propia.
func TestMarcarPropioLoSacaDeLaLista(t *testing.T) {
	ctx := context.Background()
	s, _ := abrir(t)

	kenshin := titulo(t, s, "Rurouni Kenshin", false)
	nuevo := titulo(t, s, "Los Simuladores", true, kenshin.ID)
	r := regla(t, s, nuevo.ID, 20*60)

	if err := s.Title.MarcarPropio(ctx, nuevo.ID); err != nil {
		t.Fatal(err)
	}
	leido, err := s.Title.Get(ctx, nuevo.ID)
	if err != nil {
		t.Fatal(err)
	}
	if leido.PendienteEmparejar || len(leido.Candidatos) != 0 {
		t.Fatalf("el título siguió por emparejar: %+v", leido)
	}
	// La regla no se toca.
	if _, err := s.Rule.Get(ctx, r.ID); err != nil {
		t.Fatalf("marcar propio no puede tocar las reglas: %v", err)
	}
	pendientes, err := s.Title.ListPendientes(ctx, DefaultChannelID)
	if err != nil || len(pendientes) != 0 {
		t.Fatalf("siguió en la lista de pendientes: %v (%v)", pendientes, err)
	}
	// Repetirlo no rompe nada.
	if err := s.Title.MarcarPropio(ctx, nuevo.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Title.MarcarPropio(ctx, 9999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("se esperaba ErrNotFound, salió: %v", err)
	}
	if rota, err := s.Audit.Verify(ctx); err != nil || rota != nil {
		t.Fatalf("la cadena de la bitácora quedó rota: %v (%v)", rota, err)
	}
}

// F1-67, segundo caso: el nombre de una fuente en vivo no es un programa; se
// va con sus reglas y se dice cuántas.
func TestQuitarProvisionalSeLlevaLasReglasYDiceCuantas(t *testing.T) {
	ctx := context.Background()
	s, _ := abrir(t)

	vivo := titulo(t, s, "RadioOnce Live!", true)
	regla(t, s, vivo.ID, 30)
	regla(t, s, vivo.ID, 90)
	quieto := titulo(t, s, "Rurouni Kenshin", false)
	otra := regla(t, s, quieto.ID, 20*60)

	quitadas, err := s.Title.QuitarProvisional(ctx, vivo.ID)
	if err != nil {
		t.Fatalf("no lo quitó: %v", err)
	}
	if quitadas != 2 {
		t.Fatalf("quitó %d reglas, se esperaban 2", quitadas)
	}
	if _, err := s.Title.Get(ctx, vivo.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("el título sobrevivió: %v", err)
	}
	if _, err := s.Rule.Get(ctx, otra.ID); err != nil {
		t.Fatalf("se llevó por delante una regla que no era suya: %v", err)
	}
	sinReferenciasSueltas(t, s)

	// Sobre una ficha del catálogo, ni hablar.
	if _, err := s.Title.QuitarProvisional(ctx, quieto.ID); !errors.Is(err, ErrNoPendiente) {
		t.Fatalf("se esperaba ErrNoPendiente, salió: %v", err)
	}
	if _, err := s.Title.QuitarProvisional(ctx, 9999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("se esperaba ErrNotFound, salió: %v", err)
	}

	entradas, err := s.Audit.List(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	ultima := entradas[len(entradas)-1]
	if !strings.Contains(ultima.After, "no es un programa") || !strings.Contains(ultima.After, "2 reglas") {
		t.Fatalf("la bitácora no dice qué pasó: %q", ultima.After)
	}
	if rota, err := s.Audit.Verify(ctx); err != nil || rota != nil {
		t.Fatalf("la cadena de la bitácora quedó rota: %v (%v)", rota, err)
	}
}

// Quitar un provisional también se lleva lo que ya estaba planificado con sus
// reglas: si no, el plan quedaría citando reglas que ya no existen.
func TestQuitarProvisionalSeLlevaElPlan(t *testing.T) {
	ctx := context.Background()
	s, _ := abrir(t)

	vivo := titulo(t, s, "RadioOnce Live!", true)
	r := regla(t, s, vivo.ID, 30)

	programa, err := s.Deck.ByKind(ctx, DefaultChannelID, model.DeckProgram)
	if err != nil {
		t.Fatal(err)
	}
	item := &model.PlanItem{
		ChannelID: DefaultChannelID, DeckID: programa.ID, RuleID: &r.ID,
		BroadcastDay: "2026-09-07",
		PlannedAt:    model.Day("2026-09-07").Time(time.UTC),
		PlannedMs:    30 * 60 * 1000,
		Origin:       model.OriginFiller, State: model.Planned,
	}
	if err := s.Plan.Insert(ctx, item); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Title.QuitarProvisional(ctx, vivo.ID); err != nil {
		t.Fatalf("no lo quitó: %v", err)
	}
	var quedan int
	if err := s.DB().QueryRowContext(ctx,
		`SELECT count(*) FROM plan_item WHERE schedule_rule_id = ?`, r.ID).Scan(&quedan); err != nil {
		t.Fatal(err)
	}
	if quedan != 0 {
		t.Fatalf("quedaron %d ítems del plan citando una regla que ya no existe", quedan)
	}
	sinReferenciasSueltas(t, s)
}
