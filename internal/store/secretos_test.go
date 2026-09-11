package store

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"antena787/internal/model"
)

// TestLaClaveNoSeQuedaEnLaBase es la prueba que justifica todo este archivo:
// la base se respalda cada hora y esos respaldos se copian a otra máquina, así
// que una clave de proveedor en texto plano se va de la estación sin que nadie
// lo note. Aquí se comprueba que **los bytes de la clave no están en la base**.
func TestLaClaveNoSeQuedaEnLaBase(t *testing.T) {
	s, ruta := abrir(t)
	ctx := context.Background()

	const clave = "SuperSecreta-getstreamhosting-2026"
	c := model.DriverConfig{Kind: "entrada", Driver: "url", Params: `{"url":"https://video2.example.com/8216"}`}
	if err := s.Conexion.Upsert(ctx, &c, clave); err != nil {
		t.Fatalf("no se pudo guardar la conexión: %v", err)
	}

	// Lo que hay en la columna no se parece a la clave.
	var guardado []byte
	if err := s.db.QueryRowContext(ctx,
		`SELECT credenciales FROM driver_config WHERE id = ?`, c.ID).Scan(&guardado); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(guardado, []byte(clave)) {
		t.Fatal("la clave está en texto en la columna: cualquiera que abra un respaldo la lee")
	}
	if len(guardado) == 0 {
		t.Fatal("no se guardó nada: la clave se perdió")
	}

	// Y tampoco está en el archivo entero de la base, que es lo que de verdad
	// se copia. Esto caza el caso en que se guarde bien y se filtre por otro
	// lado —un índice, un log de SQLite, una tabla temporal—.
	_ = s.Close()
	bruto, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(bruto, []byte(clave)) {
		t.Fatal("la clave aparece en el archivo de la base: se filtró por algún lado")
	}
}

// TestLaClaveVuelveEnteraYSoloPorLaPuertaBuena: se guarda, se lee, y listar no
// la saca nunca — porque para listar no hace falta, y no sacarla es la forma
// más barata de que no se escape por un log o por la API.
func TestLaClaveVuelveEnteraYSoloPorLaPuertaBuena(t *testing.T) {
	s, _ := abrir(t)
	ctx := context.Background()

	const clave = "usuario:contraseña con acentos y ñ"
	c := model.DriverConfig{Kind: "entrada", Driver: "url"}
	if err := s.Conexion.Upsert(ctx, &c, clave); err != nil {
		t.Fatal(err)
	}

	leida, err := s.Conexion.Get(ctx, c.ID)
	if err != nil {
		t.Fatalf("no se pudo leer la conexión: %v", err)
	}
	if leida.Secret != clave {
		t.Fatalf("la clave volvió cambiada: %q", leida.Secret)
	}
	if !leida.TieneSecreto {
		t.Fatal("la conexión tiene clave y dice que no")
	}

	lista, err := s.Conexion.List(ctx, DefaultChannelID, "entrada")
	if err != nil {
		t.Fatal(err)
	}
	if len(lista) != 1 {
		t.Fatalf("se esperaba una conexión y hay %d", len(lista))
	}
	if lista[0].Secret != "" {
		t.Fatal("listar sacó la clave de la base: para listar no hace falta")
	}
	if !lista[0].TieneSecreto {
		t.Fatal("listar tiene que decir que hay clave, aunque no diga cuál")
	}
}

// TestGuardarSinClaveNoBorraLaQueHabia: la pantalla manda el formulario entero
// cada vez y no puede reenviar la clave —eso es justo como se filtran—, así
// que un secreto vacío conserva el que estaba. Vaciarla a propósito es otra
// puerta.
func TestGuardarSinClaveNoBorraLaQueHabia(t *testing.T) {
	s, _ := abrir(t)
	ctx := context.Background()

	c := model.DriverConfig{Kind: "entrada", Driver: "url"}
	if err := s.Conexion.Upsert(ctx, &c, "la-de-siempre"); err != nil {
		t.Fatal(err)
	}
	c.Params = `{"url":"https://otra.example.com"}`
	if err := s.Conexion.Upsert(ctx, &c, ""); err != nil {
		t.Fatal(err)
	}
	leida, err := s.Conexion.Get(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if leida.Secret != "la-de-siempre" {
		t.Fatalf("guardar sin clave se llevó la que había: quedó %q", leida.Secret)
	}

	// Y vaciarla a propósito sí la quita.
	if err := s.Conexion.BorrarSecreto(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	leida, err = s.Conexion.Get(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if leida.Secret != "" || leida.TieneSecreto {
		t.Fatal("se pidió quitar la clave y sigue ahí")
	}
}

// TestConOtraLlaveNoSeLee comprueba lo que protege esto de verdad: la base
// copiada a otra instalación, sin su llave, no sirve de nada.
func TestConOtraLlaveNoSeLee(t *testing.T) {
	s, ruta := abrir(t)
	ctx := context.Background()

	c := model.DriverConfig{Kind: "entrada", Driver: "url"}
	if err := s.Conexion.Upsert(ctx, &c, "solo-de-esta-maquina"); err != nil {
		t.Fatal(err)
	}
	var cifrado []byte
	if err := s.db.QueryRowContext(ctx,
		`SELECT credenciales FROM driver_config WHERE id = ?`, c.ID).Scan(&cifrado); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()

	// Otra instalación: misma base, llave distinta.
	otra := filepath.Join(t.TempDir(), "otra")
	if err := os.MkdirAll(otra, 0o755); err != nil {
		t.Fatal(err)
	}
	sec, err := AbrirSecretos(otra)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sec.Descifrar(cifrado); err == nil {
		t.Fatal("otra instalación pudo leer la clave: entonces cifrarla no servía de nada")
	}

	_ = ruta
}

// TestLaLlaveNoQuedaAbiertaATodoElMundo: da igual lo bueno que sea el cifrado
// si el archivo de la llave lo puede leer cualquiera. En Windows los permisos
// POSIX no significan lo mismo, así que ahí no se comprueba —la protección de
// esa plataforma es DPAPI, que se prueba en la máquina y no aquí—.
func TestLaLlaveNoQuedaAbiertaATodoElMundo(t *testing.T) {
	if os.Getenv("GOOS") == "windows" {
		t.Skip("en Windows la protección es DPAPI, no los permisos del archivo")
	}
	dir := t.TempDir()
	if _, err := AbrirSecretos(dir); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, NombreDeLaLlave))
	if err != nil {
		t.Fatal(err)
	}
	if modo := info.Mode().Perm(); modo&0o077 != 0 {
		t.Fatalf("la llave quedó con permisos %o: la puede leer alguien más", modo)
	}
}

// TestLaMismaLlaveSeReusa: abrir dos veces la misma carpeta no puede generar
// una llave nueva, o lo cifrado ayer deja de leerse hoy.
func TestLaMismaLlaveSeReusa(t *testing.T) {
	dir := t.TempDir()
	uno, err := AbrirSecretos(dir)
	if err != nil {
		t.Fatal(err)
	}
	cifrado, err := uno.Cifrar("lo de ayer")
	if err != nil {
		t.Fatal(err)
	}
	dos, err := AbrirSecretos(dir)
	if err != nil {
		t.Fatal(err)
	}
	claro, err := dos.Descifrar(cifrado)
	if err != nil {
		t.Fatalf("al reabrir no se pudo leer lo cifrado antes: %v", err)
	}
	if claro != "lo de ayer" {
		t.Fatalf("volvió %q", claro)
	}
}
