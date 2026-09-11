// Package store es la persistencia de Antena787: una base SQLite en un
// archivo, abierta con modernc.org/sqlite (SQLite transpilado a Go, sin CGo
// — ADR 0002), con un solo escritor, WAL, llaves foráneas y espera de 5
// segundos si algo está ocupado.
//
// Lo que este paquete promete:
//
//   - El esquema (schema.sql) es el contrato. Las reglas de integridad —no
//     solape dentro de un deck, fecha_fin >= fecha_inicio— viven allí, en
//     CHECK y triggers; aquí solo se traducen a errores que una persona
//     entiende (ErrOverlap, ErrDates).
//   - Antes de aplicar cualquier migración se copia la base a un respaldo
//     con fecha, y al abrir se corre PRAGMA integrity_check. Si la base está
//     rota se devuelve *ErrCorrupt con la ruta del último respaldo, para que
//     quien arranca pueda restaurar y avisar (PRD §19).
//   - Un fallo de base nunca tumba el aire: todo error se devuelve, nunca se
//     entra en pánico.
//
// Convenciones: identificadores en inglés, comentarios y mensajes en
// español; los instantes se guardan como milisegundos UTC y se convierten
// con los ayudantes de internal/model.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"antena787/internal/model"

	_ "modernc.org/sqlite" // driver "sqlite", sin CGo
)

// DefaultChannelID es el canal que se siembra al crear la base. Mientras
// haya un solo canal (hasta F5b) todo el mundo apunta aquí.
const DefaultChannelID int64 = 1

// busyTimeout es lo que espera una escritura si la base está ocupada.
const busyTimeout = 5 * time.Second

// Store es la base abierta y sus repositorios. Se crea con Open y se cierra
// con Close. Es seguro usarlo desde varias goroutines: la base se abre con
// una sola conexión, así que hay un solo escritor por construcción.
type Store struct {
	db   *sql.DB
	path string

	Channel  *ChannelRepo
	Output   *OutputRepo
	Media    *MediaAssetRepo
	Preset   *PresetRepo
	Title    *TitleRepo
	Alias    *AliasRepo
	Episode  *EpisodeRepo
	Filler   *FillerRepo
	Live     *LiveSourceRepo
	Rule     *ScheduleRuleRepo
	Deck     *DeckRepo
	Plan     *PlanRepo
	Incident *IncidentRepo
	Audit    *AuditRepo
	Settings *SettingsRepo
}

// Open abre (o crea) la base en path, verifica su integridad, aplica las
// migraciones pendientes —respaldando antes— y siembra el canal por defecto
// la primera vez.
//
// Si la verificación de integridad falla devuelve un *ErrCorrupt con la ruta
// del respaldo más reciente disponible; quien llama decide si restaura.
func Open(path string) (*Store, error) {
	ctx := context.Background()
	if path == "" {
		return nil, errors.New("hace falta la ruta de la base de datos")
	}
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("no se pudo crear la carpeta de la base: %w", err)
		}
	}

	db, err := openDB(path)
	if err != nil {
		// Un archivo que ni siquiera abre como base es el mismo problema que
		// una base que no pasa la verificación: hay que restaurar.
		if huelaARota(err) {
			return nil, &ErrCorrupt{Path: path, LastBackup: LatestBackup(path), Detail: err.Error()}
		}
		return nil, err
	}

	s := &Store{db: db, path: path}
	s.wire()

	if err := s.CheckIntegrity(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := s.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := s.seed(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// openDB abre la conexión con los PRAGMA que este proyecto da por hechos.
// Van en el DSN para que cualquier conexión nueva los herede, no solo la
// primera.
func openDB(path string) (*sql.DB, error) {
	dsn := path +
		"?_pragma=busy_timeout(" + fmt.Sprint(busyTimeout.Milliseconds()) + ")" +
		"&_pragma=foreign_keys(1)" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("no se pudo abrir la base %q: %w", path, err)
	}
	// Un solo escritor: SQLite no gana nada con más conexiones de escritura y
	// así no hay dos transacciones peleándose por el archivo.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)
	if err := db.PingContext(context.Background()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("no se pudo abrir la base %q: %w", path, err)
	}
	return db, nil
}

// wire cuelga los repositorios de la base ya abierta.
func (s *Store) wire() {
	s.Channel = &ChannelRepo{db: s.db}
	s.Output = &OutputRepo{db: s.db}
	s.Audit = &AuditRepo{db: s.db}
	// La biblioteca anota en la bitácora lo que se cambia a mano (la pista de
	// sonido que va al aire), así que necesita la cadena de auditoría.
	s.Media = &MediaAssetRepo{db: s.db, audit: s.Audit}
	s.Preset = &PresetRepo{db: s.db}
	// El catálogo anota en la bitácora lo que se empareja a mano (F1-66,
	// F1-67), así que necesita la cadena de auditoría.
	s.Title = &TitleRepo{db: s.db, audit: s.Audit}
	s.Alias = &AliasRepo{db: s.db}
	s.Episode = &EpisodeRepo{db: s.db}
	s.Filler = &FillerRepo{db: s.db}
	s.Live = &LiveSourceRepo{db: s.db}
	s.Rule = &ScheduleRuleRepo{db: s.db}
	s.Deck = &DeckRepo{db: s.db}
	s.Incident = &IncidentRepo{db: s.db}
	// El plan anota en la bitácora lo que se toca a mano (fijar y soltar un
	// ítem), así que necesita la cadena de auditoría.
	s.Plan = &PlanRepo{db: s.db, audit: s.Audit}
	s.Settings = &SettingsRepo{db: s.db}
}

// Path es la ruta del archivo de la base.
func (s *Store) Path() string { return s.path }

// DB expone la conexión para consultas que no cubre ningún repositorio.
// Úsala poco: lo que se repite merece un método aquí.
func (s *Store) DB() *sql.DB { return s.db }

// Close cierra la base. Antes hace un checkpoint del WAL para que el archivo
// quede completo y un respaldo por copia sea válido.
func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	_, _ = s.db.ExecContext(context.Background(), "PRAGMA wal_checkpoint(TRUNCATE)")
	err := s.db.Close()
	s.db = nil
	return err
}

// CheckIntegrity corre PRAGMA integrity_check. Si la base está dañada
// devuelve un *ErrCorrupt con la ruta del respaldo más reciente.
func (s *Store) CheckIntegrity(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, "PRAGMA integrity_check")
	if err != nil {
		return &ErrCorrupt{Path: s.path, LastBackup: LatestBackup(s.path), Detail: err.Error()}
	}
	defer func() { _ = rows.Close() }()

	var problemas []string
	for rows.Next() {
		var linea string
		if err := rows.Scan(&linea); err != nil {
			return fmt.Errorf("no se pudo leer la verificación de integridad: %w", err)
		}
		if linea != "ok" {
			problemas = append(problemas, linea)
		}
	}
	if err := rows.Err(); err != nil {
		return &ErrCorrupt{Path: s.path, LastBackup: LatestBackup(s.path), Detail: err.Error()}
	}
	if len(problemas) > 0 {
		detalle := problemas[0]
		if len(problemas) > 1 {
			detalle = fmt.Sprintf("%s (y %d problemas más)", detalle, len(problemas)-1)
		}
		return &ErrCorrupt{Path: s.path, LastBackup: LatestBackup(s.path), Detail: detalle}
	}
	return nil
}

// Backup deja una copia consistente de la base en dst, con la base abierta y
// sin parar el aire. Es lo que usa el respaldo de cada hora (PRD §19).
func (s *Store) Backup(ctx context.Context, dst string) error {
	if dst == "" {
		return errors.New("hace falta la ruta del respaldo")
	}
	if dir := filepath.Dir(dst); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("no se pudo crear la carpeta del respaldo: %w", err)
		}
	}
	// VACUUM INTO exige que el destino no exista.
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("ya existe un archivo en %q: el respaldo no se sobrescribe", dst)
	}
	if _, err := s.db.ExecContext(ctx, "VACUUM INTO ?", dst); err != nil {
		return fmt.Errorf("no se pudo respaldar la base en %q: %w", dst, err)
	}
	return nil
}

// Version es la versión del esquema que tiene la base (PRAGMA user_version).
func (s *Store) Version(ctx context.Context) (int, error) {
	var v int
	if err := s.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&v); err != nil {
		return 0, fmt.Errorf("no se pudo leer la versión del esquema: %w", err)
	}
	return v, nil
}

// LatestBackup devuelve el respaldo más reciente que acompaña a una base,
// o "" si no hay ninguno. Los respaldos se llaman <ruta>.respaldo-<v>-<fecha>.db.
func LatestBackup(path string) string {
	candidatos, err := filepath.Glob(path + ".respaldo-*.db")
	if err != nil || len(candidatos) == 0 {
		return ""
	}
	mejor, mejorMod := "", time.Time{}
	for _, c := range candidatos {
		info, err := os.Stat(c)
		if err != nil {
			continue
		}
		if mejor == "" || info.ModTime().After(mejorMod) {
			mejor, mejorMod = c, info.ModTime()
		}
	}
	return mejor
}

// ── ayudas internas ───────────────────────────────────────────────────

// copyFile copia un archivo tal cual. Es el plan B del respaldo cuando
// VACUUM INTO no se puede usar; solo vale con la base cerrada o después de
// un checkpoint del WAL.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// nullInt64 convierte un puntero en lo que espera el driver para una columna
// que admite NULL.
func nullInt64(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullInt(p *int) any {
	if p == nil {
		return nil
	}
	return int64(*p)
}

func nullMinutes(p *model.Minutes) any {
	if p == nil {
		return nil
	}
	return int64(*p)
}

func nullFloat64(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullString(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

// nullMs guarda un instante que puede faltar.
func nullMs(t *time.Time) any {
	if t == nil {
		return nil
	}
	return model.Ms(*t)
}

func ptrInt64(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	v := n.Int64
	return &v
}

func ptrInt(n sql.NullInt64) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int64)
	return &v
}

func ptrMinutes(n sql.NullInt64) *model.Minutes {
	if !n.Valid {
		return nil
	}
	v := model.Minutes(n.Int64)
	return &v
}

func ptrFloat64(n sql.NullFloat64) *float64 {
	if !n.Valid {
		return nil
	}
	v := n.Float64
	return &v
}

func ptrString(n sql.NullString) *string {
	if !n.Valid {
		return nil
	}
	v := n.String
	return &v
}

func ptrTime(n sql.NullInt64) *time.Time {
	if !n.Valid {
		return nil
	}
	t := model.FromMs(n.Int64)
	return &t
}

// jsonInt64s serializa la lista de marcas de corte. Nunca escribe "null":
// la columna es NOT NULL y el valor vacío es "[]".
func jsonInt64s(v []int64) string {
	if len(v) == 0 {
		return "[]"
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func parseInt64s(s string) []int64 {
	if s == "" {
		return nil
	}
	var v []int64
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return nil
	}
	return v
}

// jsonInts serializa el reloj de cortes ([0,15,30,45]).
func jsonInts(v []int) string {
	if len(v) == 0 {
		return "[]"
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func parseInts(s string) []int {
	if s == "" {
		return nil
	}
	var v []int
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return nil
	}
	return v
}
