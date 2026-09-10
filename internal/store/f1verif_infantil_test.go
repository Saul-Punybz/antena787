package store

import (
	"context"
	"path/filepath"
	"testing"

	"antena787/internal/model"
)

// F1-76 (la parte de la base) — la marca de programación infantil vive en el
// esquema 6, entra apagada y sobrevive a guardar y volver a leer.
func TestF1Verif76InfantilCorePersisteEnElEsquema6(t *testing.T) {
	ctx := context.Background()
	s, _ := abrir(t)

	if v, err := s.Version(ctx); err != nil || v < 6 {
		t.Fatalf("la base quedó en la versión %d (%v); infantil_core llega en la 6", v, err)
	}

	// Un título recién creado no está marcado: nadie queda marcado sin que
	// una persona lo diga.
	canal := DefaultChannelID
	t1 := model.Title{ChannelID: &canal, Name: "Carmen Sandiego", Kind: model.TitleSeries}
	if err := s.Title.Insert(ctx, &t1); err != nil {
		t.Fatalf("no entró el título: %v", err)
	}
	leido, err := s.Title.Get(ctx, t1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if leido.InfantilCore {
		t.Fatal("un título nuevo no puede salir marcado como programación infantil")
	}

	// Marcarlo se guarda y se vuelve a leer, también en la lista.
	leido.InfantilCore = true
	if err := s.Title.Update(ctx, &leido); err != nil {
		t.Fatalf("no se pudo marcar el título: %v", err)
	}
	otra, err := s.Title.Get(ctx, t1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !otra.InfantilCore {
		t.Fatal("la marca de programación infantil no se guardó")
	}
	lista, err := s.Title.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(lista) != 1 || !lista[0].InfantilCore {
		t.Fatalf("la lista de títulos no trae la marca: %+v", lista)
	}
}

// Y una base que se quedó en la versión 5 sube a la 6 con lo que ya tenía
// dentro y sin marcar nada.
func TestF1Verif76UnaBaseDeLaVersion5SubeALa6(t *testing.T) {
	ctx := context.Background()
	ruta := filepath.Join(t.TempDir(), "vieja.db")

	db, err := openDB(ruta)
	if err != nil {
		t.Fatalf("no se pudo crear la base vieja: %v", err)
	}
	for _, paso := range []string{schemaSQL, migracion2, migracion3, migracion4, migracion5} {
		if _, err := db.ExecContext(ctx, paso); err != nil {
			t.Fatalf("no se pudo armar una base de la versión 5: %v", err)
		}
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO channel (id, nombre, tipo) VALUES (1, 'CAtv', 'tv');
		INSERT INTO title (channel_id, nombre, tipo) VALUES (1, 'Kojak', 'serie');
		PRAGMA user_version = 5`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	s, err := Open(ruta)
	if err != nil {
		t.Fatalf("la base de la versión 5 no migró: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	if v, err := s.Version(ctx); err != nil || v != SchemaVersion() {
		t.Fatalf("quedó en la versión %d (%v), se esperaba %d", v, err, SchemaVersion())
	}
	if LatestBackup(ruta) == "" {
		t.Fatal("migrar a la 6 tiene que dejar respaldo de lo que ya existía")
	}
	lista, err := s.Title.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(lista) != 1 || lista[0].Name != "Kojak" {
		t.Fatalf("la migración se llevó el catálogo: %+v", lista)
	}
	if lista[0].InfantilCore {
		t.Fatal("la migración no puede marcar como infantil lo que ya estaba")
	}
}
