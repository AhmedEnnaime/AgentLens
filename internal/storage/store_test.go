package storage

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func openTemp(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "agentlens.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func rawHandle(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("raw open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestOpenMigratesAndStamps(t *testing.T) {
	s := openTemp(t)
	var version int
	if err := s.db.QueryRow(`SELECT MAX(version) FROM schema_version`).Scan(&version); err != nil {
		t.Fatalf("query schema_version: %v", err)
	}
	if version != 1 {
		t.Errorf("expected version 1 after fresh open, got %d", version)
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM schema_version`).Scan(&count); err != nil {
		t.Fatalf("count schema_version: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 schema_version row, got %d", count)
	}
}

func TestReopenIsNoOp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agentlens.db")
	s1, err := Open(path)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	s1.Close()

	s2, err := Open(path)
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	defer s2.Close()

	var count int
	if err := s2.db.QueryRow(`SELECT COUNT(*) FROM schema_version`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("reopen should not add migration rows, got %d", count)
	}
}

func TestRefuseNewerDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agentlens.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := s.db.Exec(`INSERT INTO schema_version (version, applied_at) VALUES (99, 0)`); err != nil {
		t.Fatalf("stamp newer version: %v", err)
	}
	s.Close()

	if _, err := Open(path); err == nil {
		t.Fatal("expected refuse-to-open on newer database version, got nil")
	}
}

func TestCheckContiguousRejectsGap(t *testing.T) {
	cases := []struct {
		name     string
		versions []int
		wantErr  bool
	}{
		{"empty", []int{}, false},
		{"single", []int{1}, false},
		{"contiguous", []int{1, 2, 3}, false},
		{"gap", []int{1, 3}, true},
		{"start at 2", []int{2, 3}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := checkContiguous(c.versions)
			if (err != nil) != c.wantErr {
				t.Errorf("checkContiguous(%v) = %v, wantErr %v", c.versions, err, c.wantErr)
			}
		})
	}
}

func TestCheckAppliedContiguousRejectsGap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agentlens.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := s.db.Exec(`DELETE FROM schema_version`); err != nil {
		t.Fatalf("clear versions: %v", err)
	}
	if _, err := s.db.Exec(`INSERT INTO schema_version (version, applied_at) VALUES (1, 0), (3, 0)`); err != nil {
		t.Fatalf("stamp gap: %v", err)
	}
	s.Close()

	if _, err := Open(path); err == nil {
		t.Fatal("expected version-gap error, got nil")
	}
}

func TestPragmasApplied(t *testing.T) {
	s := openTemp(t)
	cases := []struct {
		name string
		sql  string
		want string
	}{
		{"journal_mode", `PRAGMA journal_mode`, "wal"},
		{"busy_timeout", `PRAGMA busy_timeout`, "5000"},
		{"foreign_keys", `PRAGMA foreign_keys`, "1"},
		{"synchronous", `PRAGMA synchronous`, "1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got string
			if err := s.db.QueryRow(c.sql).Scan(&got); err != nil {
				t.Fatalf("query %s: %v", c.sql, err)
			}
			if got != c.want {
				t.Errorf("%s: got %q, want %q", c.name, got, c.want)
			}
		})
	}
}

func TestRawEventsImmutableTriggers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agentlens.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	raw := rawHandle(t, path)

	if _, err := raw.Exec(`INSERT INTO raw_events (trace_id, id, seq, payload, payload_sha256) VALUES ('t','r',0,X'7B7D','x')`); err != nil {
		t.Fatalf("seed raw event: %v", err)
	}

	if _, err := raw.Exec(`UPDATE raw_events SET payload = X'00' WHERE id = 'r'`); err == nil {
		t.Error("UPDATE on raw_events should abort via trigger, got nil")
	}
	if _, err := raw.Exec(`DELETE FROM raw_events WHERE id = 'r'`); err == nil {
		t.Error("DELETE on raw_events should abort via trigger, got nil")
	}

	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM raw_events`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("raw event should be untouched after failed mutations, got %d rows", count)
	}
	s.Close()
}

func TestForeignKeyViolationRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agentlens.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	raw := rawHandle(t, path)

	if _, err := raw.Exec(`INSERT INTO spans (trace_id, id, seq, blob) VALUES ('ghost','s',0,'{}')`); err == nil {
		t.Error("span with no parent trace should violate FK, got nil")
	}
	if _, err := raw.Exec(`INSERT INTO events (trace_id, id, span_id, seq, blob) VALUES ('ghost','e','s',0,'{}')`); err == nil {
		t.Error("event with ghost trace should violate FK, got nil")
	}
	s.Close()
}

func TestDefaultPathEnvOverride(t *testing.T) {
	t.Setenv("AGENTLENS_DB", "/tmp/custom-agentlens.db")
	p, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	if p != "/tmp/custom-agentlens.db" {
		t.Errorf("env override should win, got %q", p)
	}
}
