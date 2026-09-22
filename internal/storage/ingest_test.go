package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/AhmedEnnaime/AgentLens/internal/domain"
)

func validTrace(id string) *domain.Trace {
	start := domain.FromEpochMillis(1000)
	end := domain.FromEpochMillis(2000)
	return &domain.Trace{
		ID:        id,
		Agent:     "opencode",
		StartTime: start,
		EndTime:   end,
		Source: domain.SourceMetadata{
			RootSessionID:    "session-1",
			ProjectDirectory: "/proj",
			ImportedAt:       start,
		},
		Spans: []*domain.Span{
			sessionSpan(id, "s", ""),
			turnSpan(id, "turn", "s"),
			modelCallSpan(id, "mc", "turn"),
			toolCallSpan(id, "tool", "mc"),
		},
		Events: []*domain.Event{
			{ID: "e1", SpanID: "mc", Kind: domain.EventFileEdit, Time: end},
		},
	}
}

func sessionSpan(traceID, id, parent string) *domain.Span {
	return &domain.Span{
		ID:       id,
		TraceID:  traceID,
		ParentID: parent,
		Kind:     domain.KindSession,
		Name:     "session",
	}
}

func turnSpan(traceID, id, parent string) *domain.Span {
	return &domain.Span{
		ID:       id,
		TraceID:  traceID,
		ParentID: parent,
		Kind:     domain.KindTurn,
		Name:     "turn",
	}
}

func modelCallSpan(traceID, id, parent string) *domain.Span {
	return &domain.Span{
		ID:           id,
		TraceID:      traceID,
		ParentID:     parent,
		Kind:         domain.KindModelCall,
		Name:         "chat glm-5.3",
		Status:       domain.StatusOk,
		Model:        &domain.ModelIdentity{ProviderID: "p", ModelID: "m", Variant: domain.Unavailable[string]()},
		Usage:        &domain.Usage{InputTokens: domain.Observed(int64(10), "r"), OutputTokens: domain.Observed(int64(5), "r")},
		Cost:         &domain.Cost{Amount: domain.Unavailable[float64](), Currency: "USD"},
		FinishReason: domain.Observed("stop", "r"),
	}
}

func toolCallSpan(traceID, id, parent string) *domain.Span {
	return &domain.Span{
		ID:       id,
		TraceID:  traceID,
		ParentID: parent,
		Kind:     domain.KindToolCall,
		Name:     "execute_tool bash",
	}
}

func rawEvent(id, traceID string, payload json.RawMessage) *domain.RawEvent {
	return &domain.RawEvent{
		ID:         id,
		TraceID:    traceID,
		Agent:      "opencode",
		RecordType: "message",
		Payload:    payload,
		CapturedAt: domain.FromEpochMillis(1000),
	}
}

type snapshot struct {
	traceCount int
	header     string
	spans      []string
	events     []string
	raws       []rawRow
}

type rawRow struct {
	id         string
	seq        int
	agent      string
	recordType string
	capturedAt int64
	sha        string
	pay        []byte
}

func takeSnapshot(t *testing.T, s *Store, traceID string) snapshot {
	t.Helper()
	snap := snapshot{}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM traces`).Scan(&snap.traceCount); err != nil {
		t.Fatalf("trace count: %v", err)
	}
	if err := s.db.QueryRow(`SELECT header FROM traces WHERE id = ?`, traceID).Scan(&snap.header); err != nil {
		t.Fatalf("header: %v", err)
	}
	rows, err := s.db.Query(`SELECT blob FROM spans WHERE trace_id = ? ORDER BY seq`, traceID)
	if err != nil {
		t.Fatalf("spans: %v", err)
	}
	for rows.Next() {
		var b string
		if err := rows.Scan(&b); err != nil {
			t.Fatalf("scan span: %v", err)
		}
		snap.spans = append(snap.spans, b)
	}
	rows.Close()
	rows, err = s.db.Query(`SELECT blob FROM events WHERE trace_id = ? ORDER BY seq`, traceID)
	if err != nil {
		t.Fatalf("events: %v", err)
	}
	for rows.Next() {
		var b string
		if err := rows.Scan(&b); err != nil {
			t.Fatalf("scan event: %v", err)
		}
		snap.events = append(snap.events, b)
	}
	rows.Close()
	rows, err = s.db.Query(`SELECT id, seq, agent, record_type, captured_at, payload_sha256, payload FROM raw_events WHERE trace_id = ? ORDER BY seq`, traceID)
	if err != nil {
		t.Fatalf("raws: %v", err)
	}
	for rows.Next() {
		var r rawRow
		if err := rows.Scan(&r.id, &r.seq, &r.agent, &r.recordType, &r.capturedAt, &r.sha, &r.pay); err != nil {
			t.Fatalf("scan raw: %v", err)
		}
		snap.raws = append(snap.raws, r)
	}
	rows.Close()
	return snap
}

func assertSnapshotsEqual(t *testing.T, a, b snapshot) {
	t.Helper()
	if a.traceCount != b.traceCount {
		t.Errorf("traceCount: got %d, want %d", a.traceCount, b.traceCount)
	}
	if len(a.spans) != len(b.spans) {
		t.Errorf("span count: got %d, want %d", len(a.spans), len(b.spans))
	}
	for i := range min(len(a.spans), len(b.spans)) {
		if a.spans[i] != b.spans[i] {
			t.Errorf("span[%d]: got %s, want %s", i, a.spans[i], b.spans[i])
		}
	}
	if len(a.events) != len(b.events) {
		t.Errorf("event count: got %d, want %d", len(a.events), len(b.events))
	}
	for i := range min(len(a.events), len(b.events)) {
		if a.events[i] != b.events[i] {
			t.Errorf("event[%d]: got %s, want %s", i, a.events[i], b.events[i])
		}
	}
	if len(a.raws) != len(b.raws) {
		t.Errorf("raw count: got %d, want %d", len(a.raws), len(b.raws))
	}
	for i := range min(len(a.raws), len(b.raws)) {
		if a.raws[i].id != b.raws[i].id || a.raws[i].seq != b.raws[i].seq || a.raws[i].agent != b.raws[i].agent || a.raws[i].recordType != b.raws[i].recordType || a.raws[i].capturedAt != b.raws[i].capturedAt || a.raws[i].sha != b.raws[i].sha || !bytes.Equal(a.raws[i].pay, b.raws[i].pay) {
			t.Errorf("raw[%d]: got %+v, want %+v", i, a.raws[i], b.raws[i])
		}
	}
}

func TestDoubleIngestIsNoOpExceptHeader(t *testing.T) {
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
	before := takeSnapshot(t, s, "t1")

	tr.Source.ImportedAt = tr.Source.ImportedAt.Add(time.Hour)
	if err := s.Ingest(ctx, tr, raw); err != nil {
		t.Fatalf("second ingest: %v", err)
	}
	after := takeSnapshot(t, s, "t1")

	if before.header == after.header {
		t.Error("header should be refreshed on re-ingest (imported_at updates)")
	}
	before.header = after.header
	assertSnapshotsEqual(t, before, after)
}

func TestConflictOnSameRawIDDifferentPayload(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	tr := validTrace("t1")
	raw := []*domain.RawEvent{rawEvent("r1", "t1", json.RawMessage(`{"a":1}`))}

	if err := s.Ingest(ctx, tr, raw); err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	before := takeSnapshot(t, s, "t1")

	conflicting := []*domain.RawEvent{rawEvent("r1", "t1", json.RawMessage(`{"a":"changed"}`))}
	err := s.Ingest(ctx, tr, conflicting)
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}

	after := takeSnapshot(t, s, "t1")
	assertSnapshotsEqual(t, before, after)
}

func TestInvalidTraceRejectedPreWrite(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	tr := validTrace("t1")
	tr.ID = ""
	if err := s.Ingest(ctx, tr, nil); err == nil {
		t.Fatal("expected validation error for empty trace id")
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM traces`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Errorf("no rows should be written, got %d traces", count)
	}
}

func TestMismatchedRawTraceIDRejected(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	tr := validTrace("t1")
	raw := []*domain.RawEvent{rawEvent("r1", "other", json.RawMessage(`{"a":1}`))}
	err := s.Ingest(ctx, tr, raw)
	if err == nil {
		t.Fatal("expected mismatched trace id error")
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM traces`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Errorf("no rows should be written, got %d traces", count)
	}
}

func TestInvalidRawEventRejected(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	tr := validTrace("t1")
	raw := []*domain.RawEvent{rawEvent("r1", "t1", json.RawMessage(`{not json`))}
	if err := s.Ingest(ctx, tr, raw); err == nil {
		t.Fatal("expected invalid payload error")
	}
}

func TestGrownSessionAppendsRawsAndReplacesTree(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	tr := validTrace("t1")
	raw := []*domain.RawEvent{rawEvent("r1", "t1", json.RawMessage(`{"a":1}`))}

	if err := s.Ingest(ctx, tr, raw); err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	first := takeSnapshot(t, s, "t1")
	if len(first.raws) != 1 {
		t.Fatalf("expected 1 raw, got %d", len(first.raws))
	}
	if first.raws[0].seq != 0 {
		t.Errorf("first raw seq should be 0, got %d", first.raws[0].seq)
	}

	tr.Spans = append(tr.Spans, turnSpan("t1", "turn2", "s"), modelCallSpan("t1", "mc2", "turn2"))
	raw = append(raw, rawEvent("r2", "t1", json.RawMessage(`{"c":3}`)))
	if err := s.Ingest(ctx, tr, raw); err != nil {
		t.Fatalf("grown ingest: %v", err)
	}

	after := takeSnapshot(t, s, "t1")
	if len(after.raws) != 2 {
		t.Fatalf("expected 2 raws after grown import, got %d", len(after.raws))
	}
	if after.raws[0].seq != 0 || after.raws[1].seq != 1 {
		t.Errorf("raw seqs should continue 0,1; got %d,%d", after.raws[0].seq, after.raws[1].seq)
	}
	if after.raws[1].id != "r2" {
		t.Errorf("second raw should be r2, got %s", after.raws[1].id)
	}
	if len(after.spans) != 6 {
		t.Errorf("grown tree should have 6 spans, got %d", len(after.spans))
	}
}

func TestContentSHAStableAcrossReorders(t *testing.T) {
	tr := validTrace("t1")
	first, err := contentSHA256(tr)
	if err != nil {
		t.Fatalf("contentSHA256: %v", err)
	}
	tr.Spans[0], tr.Spans[1] = tr.Spans[1], tr.Spans[0]
	second, err := contentSHA256(tr)
	if err != nil {
		t.Fatalf("contentSHA256: %v", err)
	}
	if first == second {
		t.Error("content hash must be order-sensitive")
	}
}

func TestPayloadSHAStable(t *testing.T) {
	a := payloadSHA256(json.RawMessage(`{"x":1}`))
	b := payloadSHA256(json.RawMessage(`{"x":1}`))
	if a != b {
		t.Errorf("same payload must hash equal: %s vs %s", a, b)
	}
	c := payloadSHA256(json.RawMessage(`{"x":2}`))
	if a == c {
		t.Error("different payloads must hash differently")
	}
}

func TestZeroCapturedAtRejected(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	tr := validTrace("t1")
	raw := []*domain.RawEvent{
		{
			ID:         "r1",
			TraceID:    "t1",
			Agent:      "opencode",
			RecordType: "message",
			Payload:    json.RawMessage(`{"a":1}`),
			CapturedAt: time.Time{},
		},
	}
	err := s.Ingest(ctx, tr, raw)
	if err == nil {
		t.Fatal("expected zero captured_at to be rejected")
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM raw_events`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Errorf("no raw rows should be written, got %d", count)
	}
}

func TestRawEnvelopeRoundTrip(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	tr := validTrace("t1")
	raw := []*domain.RawEvent{
		{
			ID:         "r1",
			TraceID:    "t1",
			Agent:      "opencode",
			RecordType: "message",
			Payload:    json.RawMessage(`{"a":1}`),
			CapturedAt: domain.FromEpochMillis(1500),
		},
	}
	if err := s.Ingest(ctx, tr, raw); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	snap := takeSnapshot(t, s, "t1")
	if len(snap.raws) != 1 {
		t.Fatalf("expected 1 raw, got %d", len(snap.raws))
	}
	r := snap.raws[0]
	if r.agent != "opencode" {
		t.Errorf("agent: got %q, want %q", r.agent, "opencode")
	}
	if r.recordType != "message" {
		t.Errorf("record type: got %q, want %q", r.recordType, "message")
	}
	if r.capturedAt != 1500 {
		t.Errorf("captured_at: got %d, want %d", r.capturedAt, 1500)
	}
	if !bytes.Equal(r.pay, []byte(`{"a":1}`)) {
		t.Errorf("payload: got %s, want %s", r.pay, `{"a":1}`)
	}
}

func TestReingestDifferentCapturedAtIsIdempotent(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	tr := validTrace("t1")
	raw := []*domain.RawEvent{
		{
			ID:         "r1",
			TraceID:    "t1",
			Agent:      "opencode",
			RecordType: "message",
			Payload:    json.RawMessage(`{"a":1}`),
			CapturedAt: domain.FromEpochMillis(1500),
		},
	}
	if err := s.Ingest(ctx, tr, raw); err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	before := takeSnapshot(t, s, "t1")

	raw[0].CapturedAt = domain.FromEpochMillis(2500)
	if err := s.Ingest(ctx, tr, raw); err != nil {
		t.Fatalf("re-ingest with different captured_at: %v", err)
	}
	after := takeSnapshot(t, s, "t1")

	if before.raws[0].capturedAt != 1500 {
		t.Fatalf("first-capture time must be preserved, got %d", before.raws[0].capturedAt)
	}
	before.header = after.header
	assertSnapshotsEqual(t, before, after)
}
