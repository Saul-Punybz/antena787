package store

import (
	"context"
	"path/filepath"
	"testing"
)

// baseEnLaVersion deja una base recién hecha parada en la versión que se pida,
// con sus escalones aplicados en orden y su user_version puesto. Sirve para
// probar una migración **sobre datos de antes**, que es la única forma de
// saber si se lleva algo por delante: una base vacía siempre migra bien.
func baseEnLaVersion(t *testing.T, v int) string {
	t.Helper()
	ruta := filepath.Join(t.TempDir(), "antena.db")
	db, err := openDB(ruta)
	if err != nil {
		t.Fatalf("no se pudo abrir la base de pruebas: %v", err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(schemaSQL); err != nil {
		t.Fatalf("no se pudo crear el esquema: %v", err)
	}
	for _, m := range migrations {
		if m.Version > v {
			break
		}
		if _, err := db.Exec(m.SQL); err != nil {
			t.Fatalf("no se pudo aplicar el escalón %d: %v", m.Version, err)
		}
	}
	if _, err := db.Exec("PRAGMA user_version = " + itoaTest(v)); err != nil {
		t.Fatalf("no se pudo fijar la versión: %v", err)
	}
	return ruta
}

func itoaTest(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// TestMigracion11AbreLaPuertaAUnaURLSinPerderNada rehace `live_source` entera
// para ampliar su CHECK, y eso —con las claves foráneas encendidas y dos
// tablas apuntando a ella— es exactamente donde se pierden datos si alguien se
// descuida.
//
// Sostiene las cuatro cosas que tienen que seguir siendo verdad después de
// migrar: que lo que había sigue estando con su mismo id, que las reglas que
// lo apuntaban no se quedaron huérfanas, que ahora se acepta 'url', y que lo
// inventado se sigue rechazando — ampliar un CHECK no es quitarlo.
func TestMigracion11AbreLaPuertaAUnaURLSinPerderNada(t *testing.T) {
	ctx := context.Background()
	ruta := baseEnLaVersion(t, 10)

	// Datos de antes: una fuente como las que ya existían, con su regla encima.
	previa, err := openDB(ruta)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := previa.Exec(`
		INSERT INTO channel (id, nombre, tipo, perfil_de_formato, perfil_regulatorio,
			modo, zona_horaria, hora_inicio_dia_emision, carga_maxima_por_hora,
			identificativo, comunidad_licencia, clase_licencia)
		VALUES (1, 'Prueba', 'tv', '720p59.94', 'us-fcc', 'sombra',
			'America/Puerto_Rico', 360, 12, 'WPRU-LD', 'Arecibo, PR', 'class_a')
		ON CONFLICT(id) DO NOTHING`); err != nil {
		t.Fatalf("no se pudo dejar el canal: %v", err)
	}
	if _, err := previa.Exec(`
		INSERT INTO live_source (id, channel_id, nombre, tipo, punto_de_escucha,
			retardo_ms, gracia_s)
		VALUES (77, 1, 'RadioOnce Live!', 'srt', 'srt://0.0.0.0:9000', 7000, 30)`); err != nil {
		t.Fatalf("no se pudo dejar la fuente de antes: %v", err)
	}
	if _, err := previa.Exec(`
		INSERT INTO schedule_rule (id, channel_id, tipo, live_source_id, patron_de_dias,
			hora, duracion_slot_ms, fecha_inicio, fecha_fin)
		VALUES (900, 1, 'vivo', 77, '_MMJV__', 600, 3600000, '2026-01-01', '2026-12-31')`); err != nil {
		t.Fatalf("no se pudo dejar la regla que la apunta: %v", err)
	}
	_ = previa.Close()

	// Y ahora se migra de verdad.
	s, err := Open(ruta)
	if err != nil {
		t.Fatalf("la base no sobrevivió a la migración: %v", err)
	}
	defer func() { _ = s.Close() }()

	if v, _ := s.Version(ctx); v < 11 {
		t.Fatalf("la base se quedó en la versión %d", v)
	}

	var tipo, nombre, punto string
	var retardo, gracia int
	if err := s.db.QueryRowContext(ctx,
		`SELECT tipo, nombre, punto_de_escucha, retardo_ms, gracia_s
		 FROM live_source WHERE id = 77`).Scan(&tipo, &nombre, &punto, &retardo, &gracia); err != nil {
		t.Fatalf("la fuente que ya existía desapareció al migrar: %v", err)
	}
	if tipo != "srt" || nombre != "RadioOnce Live!" || punto != "srt://0.0.0.0:9000" {
		t.Fatalf("la fuente cambió al migrar: %q %q %q", tipo, nombre, punto)
	}
	if retardo != 7000 || gracia != 30 {
		t.Fatalf("los plazos de la fuente cambiaron: retardo %d, gracia %d", retardo, gracia)
	}

	// La regla sigue apuntando a la misma fuente, y el apuntador resuelve.
	var huerfanas int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM schedule_rule r
		WHERE r.live_source_id IS NOT NULL
		  AND NOT EXISTS (SELECT 1 FROM live_source l WHERE l.id = r.live_source_id)`).
		Scan(&huerfanas); err != nil {
		t.Fatal(err)
	}
	if huerfanas != 0 {
		t.Fatalf("%d reglas quedaron apuntando a una fuente que ya no está", huerfanas)
	}

	// Lo nuevo entra: la señal que se va a buscar en vez de esperarla.
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO live_source (channel_id, nombre, tipo, punto_de_escucha)
		VALUES (1, 'El HLS de la máquina', 'url', 'http://localhost:8080/hls/for_tv/index.m3u8')`); err != nil {
		t.Fatalf("una fuente de tipo 'url' tenía que entrar y no entró: %v", err)
	}

	// Y lo inventado se sigue rechazando.
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO live_source (channel_id, nombre, tipo) VALUES (1, 'x', 'telepatia')`); err == nil {
		t.Fatal("se aceptó un tipo de fuente que no existe: el CHECK se amplió, no se quitó")
	}
}
