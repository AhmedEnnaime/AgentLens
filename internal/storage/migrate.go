package storage

import (
	"embed"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func migrationVersions() ([]int, error) {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return nil, err
	}
	versions := make([]int, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}
		prefix, _, _ := strings.Cut(name, "_")
		v, err := strconv.Atoi(prefix)
		if err != nil {
			return nil, fmt.Errorf("storage: migration filename %q has non-numeric prefix", name)
		}
		versions = append(versions, v)
	}
	sort.Ints(versions)
	return versions, nil
}

func migrationSQL(v int) (string, error) {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		name := e.Name()
		prefix, _, _ := strings.Cut(name, "_")
		n, err := strconv.Atoi(prefix)
		if err != nil {
			continue
		}
		if n == v {
			b, err := migrationsFS.ReadFile("migrations/" + name)
			if err != nil {
				return "", err
			}
			return string(b), nil
		}
	}
	return "", fmt.Errorf("storage: no migration for version %d", v)
}

func (s *Store) migrate() error {
	versions, err := migrationVersions()
	if err != nil {
		return err
	}
	if len(versions) == 0 {
		return fmt.Errorf("storage: no embedded migrations")
	}
	if err := checkContiguous(versions); err != nil {
		return err
	}
	if err := s.ensureSchemaVersion(); err != nil {
		return err
	}
	current, err := s.currentVersion()
	if err != nil {
		return err
	}
	highest := versions[len(versions)-1]
	if current > highest {
		return fmt.Errorf("storage: database version %d was created by a newer AgentLens (this binary supports up to %d)", current, highest)
	}
	if err := s.checkAppliedContiguous(); err != nil {
		return err
	}
	for _, v := range versions {
		if v <= current {
			continue
		}
		if err := s.applyMigration(v); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ensureSchemaVersion() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (
		version    INTEGER PRIMARY KEY,
		applied_at INTEGER NOT NULL
	)`)
	return err
}

func (s *Store) currentVersion() (int, error) {
	var v int
	err := s.db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&v)
	if err != nil {
		return 0, err
	}
	return v, nil
}

func (s *Store) checkAppliedContiguous() error {
	rows, err := s.db.Query(`SELECT version FROM schema_version ORDER BY version`)
	if err != nil {
		return err
	}
	defer rows.Close()
	i := 0
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return err
		}
		i++
		if v != i {
			return fmt.Errorf("storage: applied migration versions have a gap: expected %d, found %d (corrupt store)", i, v)
		}
	}
	return rows.Err()
}

func checkContiguous(versions []int) error {
	for i, v := range versions {
		if v != i+1 {
			return fmt.Errorf("storage: embedded migration versions have a gap at %d (corrupt binary)", v)
		}
	}
	return nil
}

func (s *Store) applyMigration(v int) error {
	prev, err := s.currentVersion()
	if err != nil {
		return err
	}
	if v != prev+1 {
		return fmt.Errorf("storage: migration version gap: at %d, next embedded is %d (corrupt store)", prev, v)
	}
	sqlText, err := migrationSQL(v)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(sqlText); err != nil {
		return fmt.Errorf("storage: migration %d: %w", v, err)
	}
	if _, err := tx.Exec(`INSERT INTO schema_version (version, applied_at) VALUES (?, ?)`, v, nowMillis()); err != nil {
		return fmt.Errorf("storage: stamp migration %d: %w", v, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("storage: commit migration %d: %w", v, err)
	}
	return nil
}
