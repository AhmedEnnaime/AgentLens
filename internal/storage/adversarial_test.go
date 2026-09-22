package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/AhmedEnnaime/AgentLens/internal/domain"
)

func allTableCounts(t *testing.T, s *Store) (traces, spans, events, raws int) {
	t.Helper()
	var row [4]int
	qs := []string{
		`SELECT COUNT(*) FROM traces`,
		`SELECT COUNT(*) FROM spans`,
		`SELECT COUNT(*) FROM events`,
		`SELECT COUNT(*) FROM raw_events`,
	}
	for i, q := range qs {
		if err := s.db.QueryRow(q).Scan(&row[i]); err != nil {
			t.Fatalf("count query %d: %v", i, err)
		}
	}
	return row[0], row[1], row[2], row[3]
}

func assertEmpty(t *testing.T, s *Store) {
	t.Helper()
	tr, sp, ev, ra := allTableCounts(t, s)
	if tr != 0 || sp != 0 || ev != 0 || ra != 0 {
		t.Fatalf("expected empty store, got traces=%d spans=%d events=%d raws=%d", tr, sp, ev, ra)
	}
}

func TestPartialFailureDuplicateRawIDRollsBackAllTables(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	tr := validTrace("t1")
	raw := []*domain.RawEvent{
		rawEvent("r1", "t1", json.RawMessage(`{"a":1}`)),
		rawEvent("r1", "t1", json.RawMessage(`{"b":2}`)),
	}
	err := s.Ingest(ctx, tr, raw)
	if err == nil {
		t.Fatal("expected error on duplicate raw id within one call")
	}
	assertEmpty(t, s)
}

func TestPartialFailureDuplicateSpanIDRollsBackAllTables(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	tr := validTrace("t1")
	dup := turnSpan("t1", "s", "s")
	tr.Spans = append(tr.Spans, dup)
	raw := []*domain.RawEvent{rawEvent("r1", "t1", json.RawMessage(`{"a":1}`))}
	err := s.Ingest(ctx, tr, raw)
	if err == nil {
		t.Fatal("expected validation error on duplicate span id")
	}
	assertEmpty(t, s)
}

func TestPartialFailureInvalidRawAtPositionN(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	tr := validTrace("t1")
	raw := []*domain.RawEvent{
		rawEvent("r1", "t1", json.RawMessage(`{"a":1}`)),
		rawEvent("r2", "t1", json.RawMessage(`{"b":2}`)),
		rawEvent("r3", "t1", json.RawMessage(`not json`)),
	}
	err := s.Ingest(ctx, tr, raw)
	if err == nil {
		t.Fatal("expected validation error at position 2")
	}
	assertEmpty(t, s)
}

func TestErrConflictRollsBackTreeAndHeader(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	tr := validTrace("t1")
	raw := []*domain.RawEvent{rawEvent("r1", "t1", json.RawMessage(`{"a":1}`))}
	if err := s.Ingest(ctx, tr, raw); err != nil {
		t.Fatalf("seed ingest: %v", err)
	}
	before := takeSnapshot(t, s, "t1")

	changed := validTrace("t1")
	changed.Spans[1].Name = "renamed-turn"
	conflict := []*domain.RawEvent{
		rawEvent("r1", "t1", json.RawMessage(`{"a":1}`)),
		rawEvent("r2", "t1", json.RawMessage(`{"c":3}`)),
		rawEvent("r2", "t1", json.RawMessage(`{"d":4}`)),
	}
	err := s.Ingest(ctx, changed, conflict)
	if err == nil {
		t.Fatal("expected conflict on same raw id different payload")
	}
	after := takeSnapshot(t, s, "t1")
	assertSnapshotsEqual(t, before, after)
}

func TestConcurrentSameTraceIngestNoCorruption(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	tr := validTrace("t1")
	raw := []*domain.RawEvent{
		rawEvent("r1", "t1", json.RawMessage(`{"a":1}`)),
		rawEvent("r2", "t1", json.RawMessage(`{"b":2}`)),
	}

	const n = 8
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = s.Ingest(ctx, tr, raw)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err == nil {
			continue
		}
		if errors.Is(err, domain.ErrConflict) {
			t.Errorf("goroutine %d: unexpected ErrConflict (interleaved writes corrupted idempotency): %v", i, err)
		}
	}

	got, err := s.TraceByID(ctx, "t1")
	if err != nil {
		t.Fatalf("TraceByID after concurrent ingest: %v", err)
	}
	pre, err := json.Marshal(tr)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	post, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal got: %v", err)
	}
	if !bytes.Equal(pre, post) {
		t.Errorf("concurrent ingest produced non-byte-identical trace\npre:  %s\npost: %s", pre, post)
	}
	if ra := rawCount(t, s, "t1"); ra != 2 {
		t.Errorf("expected exactly 2 raw events after concurrent ingest, got %d", ra)
	}
}

func TestTripleIngestByteIdentical(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	tr := validTrace("t1")
	raw := []*domain.RawEvent{
		rawEvent("r1", "t1", json.RawMessage(`{"a":1}`)),
		rawEvent("r2", "t1", json.RawMessage(`{"b":2}`)),
	}
	if err := s.Ingest(ctx, tr, raw); err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	snap1 := takeSnapshot(t, s, "t1")
	if err := s.Ingest(ctx, tr, raw); err != nil {
		t.Fatalf("second ingest: %v", err)
	}
	snap2 := takeSnapshot(t, s, "t1")
	if err := s.Ingest(ctx, tr, raw); err != nil {
		t.Fatalf("third ingest: %v", err)
	}
	snap3 := takeSnapshot(t, s, "t1")
	snap1.header = snap3.header
	snap2.header = snap3.header
	assertSnapshotsEqual(t, snap1, snap2)
	assertSnapshotsEqual(t, snap2, snap3)
}

func TestPayloadOnlyHashDifferentEnvelopeIdempotent(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	tr := validTrace("t1")
	payload := json.RawMessage(`{"a":1}`)
	first := []*domain.RawEvent{
		{ID: "r1", TraceID: "t1", Agent: "opencode", RecordType: "message", Payload: payload, CapturedAt: domain.FromEpochMillis(1500)},
	}
	if err := s.Ingest(ctx, tr, first); err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	snap := takeSnapshot(t, s, "t1")

	second := []*domain.RawEvent{
		{ID: "r1", TraceID: "t1", Agent: "claude", RecordType: "tool_use", Payload: payload, CapturedAt: domain.FromEpochMillis(9999)},
	}
	if err := s.Ingest(ctx, tr, second); err != nil {
		t.Fatalf("re-ingest with different envelope: %v", err)
	}
	after := takeSnapshot(t, s, "t1")
	snap.header = after.header
	assertSnapshotsEqual(t, snap, after)
	if after.raws[0].agent != "opencode" {
		t.Errorf("first-capture envelope must be preserved: got agent %q, want opencode", after.raws[0].agent)
	}
	if after.raws[0].capturedAt != 1500 {
		t.Errorf("first-capture captured_at must be preserved: got %d, want 1500", after.raws[0].capturedAt)
	}
}

func TestNewerSchemaVersionRefusesActionableError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agentlens.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := s.db.Exec(`INSERT INTO schema_version (version, applied_at) VALUES (999, 0)`); err != nil {
		t.Fatalf("stamp 999: %v", err)
	}
	s.Close()
	_, err = Open(path)
	if err == nil {
		t.Fatal("expected refuse-to-open")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("newer")) {
		t.Errorf("error should be actionable (mention newer), got: %v", err)
	}
}

func TestNoDowngradePathExists(t *testing.T) {
	versions, err := migrationVersions()
	if err != nil {
		t.Fatalf("migrationVersions: %v", err)
	}
	if len(versions) != 1 || versions[0] != 1 {
		t.Fatalf("expected exactly migration 1, got %v", versions)
	}
	for _, name := range migrationNames() {
		if isDown(name) {
			t.Errorf("down migration present: %s", name)
		}
	}
}

func migrationNames() []string {
	entries, _ := migrationsFS.ReadDir("migrations")
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func isDown(name string) bool {
	return bytes.Contains([]byte(name), []byte("down")) || bytes.Contains([]byte(name), []byte("downgrade"))
}

func TestUpdateEnvelopeColumnAborts(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	tr := validTrace("t1")
	raw := []*domain.RawEvent{rawEvent("r1", "t1", json.RawMessage(`{"a":1}`))}
	if err := s.Ingest(ctx, tr, raw); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if _, err := s.db.Exec(`UPDATE raw_events SET captured_at = captured_at + 1 WHERE id = 'r1'`); err == nil {
		t.Error("UPDATE touching only envelope column captured_at should abort via trigger, got nil")
	}
	if _, err := s.db.Exec(`UPDATE raw_events SET record_type = 'evil' WHERE id = 'r1'`); err == nil {
		t.Error("UPDATE touching only envelope column record_type should abort via trigger, got nil")
	}
	if _, err := s.db.Exec(`UPDATE raw_events SET agent = 'evil' WHERE id = 'r1'`); err == nil {
		t.Error("UPDATE touching only envelope column agent should abort via trigger, got nil")
	}
	if _, err := s.db.Exec(`UPDATE raw_events SET seq = seq WHERE id = 'r1'`); err == nil {
		t.Error("UPDATE touching non-envelope column seq should abort via trigger, got nil")
	}
}

func TestFKEventsUnknownSpanRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agentlens.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()
	raw := rawHandle(t, path)

	if _, err := raw.Exec(`INSERT INTO traces (id, agent, root_session_id, project_directory, start_time, end_time, content_sha256, header) VALUES ('t','a','r','/p',0,0,'x','{}')`); err != nil {
		t.Fatalf("seed trace: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO spans (trace_id, id, seq, blob) VALUES ('t','s',0,'{}')`); err != nil {
		t.Fatalf("seed span: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO events (trace_id, id, span_id, seq, blob) VALUES ('t','e','ghost-span',1,'{}')`); err == nil {
		t.Error("event with unknown span_id should violate FK, got nil")
	}
	if _, err := raw.Exec(`INSERT INTO events (trace_id, id, span_id, seq, blob) VALUES ('ghost-trace','e2','s',1,'{}')`); err == nil {
		t.Error("event with unknown trace_id should violate FK, got nil")
	}
}

func TestLabelsDirectInsertAllowedNoCodePathWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agentlens.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()
	raw := rawHandle(t, path)

	if _, err := raw.Exec(`INSERT INTO traces (id, agent, root_session_id, project_directory, start_time, end_time, content_sha256, header) VALUES ('t','a','r','/p',0,0,'x','{}')`); err != nil {
		t.Fatalf("seed trace: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO labels (trace_id, key, value, created_at) VALUES ('t','k','v',1)`); err != nil {
		t.Errorf("labels direct insert should be allowed (reserved != blocked), got: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO labels (trace_id, key, value, created_at) VALUES ('ghost','k','v',1)`); err == nil {
		t.Error("labels insert with unknown trace should violate FK, got nil")
	}
}

func TestRawEventsNoFKToTracesButIngestGatesMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agentlens.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()
	raw := rawHandle(t, path)

	if _, err := raw.Exec(`INSERT INTO raw_events (trace_id, id, seq, agent, record_type, captured_at, payload, payload_sha256) VALUES ('no-such-trace','r',0,'a','m',1,X'7B7D','x')`); err != nil {
		t.Errorf("raw_events has no FK to traces (D7): direct insert should succeed, got: %v", err)
	}

	ctx := context.Background()
	tr := validTrace("t1")
	bad := []*domain.RawEvent{rawEvent("r1", "other", json.RawMessage(`{"a":1}`))}
	if err := s.Ingest(ctx, tr, bad); err == nil {
		t.Error("ingest must reject mismatched TraceID pre-write")
	}
	if _, err := s.RawEventsByTrace(ctx, "no-trace-no-raws"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("RawEventsByTrace nonexistent trace should ErrNotFound, got %v", err)
	}
}

func TestLargePayloadRoundTripByteIdentical(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	tr := validTrace("t1")
	big := make([]byte, 2*1024*1024)
	for i := range big {
		big[i] = byte('a' + i%26)
	}
	payload := json.RawMessage(`"` + string(big) + `"`)
	raw := []*domain.RawEvent{rawEvent("r1", "t1", payload)}
	if err := s.Ingest(ctx, tr, raw); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	got, err := s.RawEventsByTrace(ctx, "t1")
	if err != nil {
		t.Fatalf("RawEventsByTrace: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 raw, got %d", len(got))
	}
	if !bytes.Equal(got[0].Payload, payload) {
		t.Errorf("large payload not byte-identical: got %d bytes, want %d", len(got[0].Payload), len(payload))
	}
}

func TestNilEventsRoundTripByteIdentical(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	tr := validTrace("t1")
	tr.Events = nil
	raw := []*domain.RawEvent{rawEvent("r1", "t1", json.RawMessage(`{"a":1}`))}
	if err := s.Ingest(ctx, tr, raw); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	pre, err := json.Marshal(tr)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got, err := s.TraceByID(ctx, "t1")
	if err != nil {
		t.Fatalf("TraceByID: %v", err)
	}
	post, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal got: %v", err)
	}
	if !bytes.Equal(pre, post) {
		t.Errorf("nil events broke byte-identity (events:null vs events:[])\npre:  %s\npost: %s", pre, post)
	}
}

func TestEmptyNonNilEventsRejectedPreWrite(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	tr := validTrace("t1")
	tr.Events = []*domain.Event{}
	raw := []*domain.RawEvent{rawEvent("r1", "t1", json.RawMessage(`{"a":1}`))}
	err := s.Ingest(ctx, tr, raw)
	if err == nil {
		t.Fatal("expected gate error for empty non-nil events")
	}
	if errors.Is(err, domain.ErrConflict) || errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("gate error must be a plain error, got %v", err)
	}
	trc, spc, evc, rac := allTableCounts(t, s)
	if trc != 0 || spc != 0 || evc != 0 || rac != 0 {
		t.Errorf("gate must persist nothing: traces=%d spans=%d events=%d raws=%d", trc, spc, evc, rac)
	}
}

func TestPreallocatedThenFilledEventsPassGate(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	tr := validTrace("t1")
	events := make([]*domain.Event, 0, 4)
	events = append(events,
		&domain.Event{ID: "e1", SpanID: "mc", Kind: domain.EventFileEdit, Time: tr.EndTime},
		&domain.Event{ID: "e2", SpanID: "mc", Kind: domain.EventFileEdit, Time: tr.EndTime},
		&domain.Event{ID: "e3", SpanID: "turn", Kind: domain.EventSessionCompaction, Time: tr.EndTime},
		&domain.Event{ID: "e4", SpanID: "s", Kind: domain.EventTaskMarker, Time: tr.EndTime},
	)
	tr.Events = events
	raw := []*domain.RawEvent{rawEvent("r1", "t1", json.RawMessage(`{"a":1}`))}
	if err := s.Ingest(ctx, tr, raw); err != nil {
		t.Fatalf("preallocated-then-filled events must pass gate: %v", err)
	}
	pre, err := json.Marshal(tr)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got, err := s.TraceByID(ctx, "t1")
	if err != nil {
		t.Fatalf("TraceByID: %v", err)
	}
	post, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal got: %v", err)
	}
	if !bytes.Equal(pre, post) {
		t.Errorf("preallocated events not byte-identical\npre:  %s\npost: %s", pre, post)
	}
}

func TestZeroStartTimeRejectedPreWrite(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	tr := validTrace("t1")
	tr.StartTime = time.Time{}
	raw := []*domain.RawEvent{rawEvent("r1", "t1", json.RawMessage(`{"a":1}`))}
	err := s.Ingest(ctx, tr, raw)
	if err == nil {
		t.Fatal("expected zero start_time to be rejected")
	}
	if errors.Is(err, domain.ErrConflict) || errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("start_time gate error must be a plain error, got %v", err)
	}
	trc, spc, evc, rac := allTableCounts(t, s)
	if trc != 0 || spc != 0 || evc != 0 || rac != 0 {
		t.Errorf("gate must persist nothing: traces=%d spans=%d events=%d raws=%d", trc, spc, evc, rac)
	}
}

func TestAttributesNilAndEmptyRoundTripByteIdentical(t *testing.T) {
	placements := []struct {
		name string
		set  func(*domain.Trace)
	}{
		{"trace nil", func(tr *domain.Trace) { tr.Attributes = nil }},
		{"trace empty", func(tr *domain.Trace) { tr.Attributes = map[string]any{} }},
		{"span nil", func(tr *domain.Trace) { tr.Spans[0].Attributes = nil }},
		{"span empty", func(tr *domain.Trace) { tr.Spans[0].Attributes = map[string]any{} }},
		{"event nil", func(tr *domain.Trace) { tr.Events[0].Attributes = nil }},
		{"event empty", func(tr *domain.Trace) { tr.Events[0].Attributes = map[string]any{} }},
	}
	for _, p := range placements {
		t.Run(p.name, func(t *testing.T) {
			s := openTemp(t)
			ctx := context.Background()
			tr := validTrace("t1")
			tr.Events[0].Attributes = map[string]any{"seed": "x"}
			p.set(tr)
			raw := []*domain.RawEvent{rawEvent("r1", "t1", json.RawMessage(`{"a":1}`))}
			if err := s.Ingest(ctx, tr, raw); err != nil {
				t.Fatalf("ingest: %v", err)
			}
			pre, err := json.Marshal(tr)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			got, err := s.TraceByID(ctx, "t1")
			if err != nil {
				t.Fatalf("TraceByID: %v", err)
			}
			post, err := json.Marshal(got)
			if err != nil {
				t.Fatalf("marshal got: %v", err)
			}
			if !bytes.Equal(pre, post) {
				t.Errorf("attributes %s not byte-identical\npre:  %s\npost: %s", p.name, pre, post)
			}
		})
	}
}

func TestEmptyAttributesRoundTripByteIdentical(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	tr := validTrace("t1")
	tr.Attributes = nil
	tr.Spans[0].Attributes = nil
	raw := []*domain.RawEvent{rawEvent("r1", "t1", json.RawMessage(`{"a":1}`))}
	if err := s.Ingest(ctx, tr, raw); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	pre, err := json.Marshal(tr)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got, err := s.TraceByID(ctx, "t1")
	if err != nil {
		t.Fatalf("TraceByID: %v", err)
	}
	post, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal got: %v", err)
	}
	if !bytes.Equal(pre, post) {
		t.Errorf("empty attributes broke byte-identity\npre:  %s\npost: %s", pre, post)
	}
}
