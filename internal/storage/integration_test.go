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

func comprehensiveTrace() *domain.Trace {
	start := domain.FromEpochMillis(1000)
	end := domain.FromEpochMillis(9000)
	return &domain.Trace{
		ID:        "t",
		Agent:     "opencode",
		StartTime: start,
		EndTime:   end,
		Attributes: map[string]any{
			"string_attr": "value",
			"int_attr":    int64(42),
		},
		Source: domain.SourceMetadata{
			RootSessionID:    "session-1",
			ProjectDirectory: "/proj",
			AgentVersion:     "1.0.0",
			ImportedAt:       start,
			AgentLensVersion: "v0.1.0",
		},
		Capabilities: domain.Capabilities{
			PerCallUsage:        true,
			CacheTokenUsage:     true,
			ReasoningTokenUsage: true,
			ModelVariant:        true,
			SubagentSessions:    true,
			FileEditEvents:      true,
			CompactionEvents:    true,
			TaskMarkers:         true,
		},
		Spans: []*domain.Span{
			{
				ID:      "s",
				TraceID: "t",
				Kind:    domain.KindSession,
				Name:    "session",
				EndTime: end,
				Cost:    &domain.Cost{Amount: domain.Observed(float64(0.99), "r"), Currency: "USD"},
			},
			{
				ID:       "sub",
				TraceID:  "t",
				ParentID: "s",
				Kind:     domain.KindSession,
				Name:     "subagent",
			},
			{
				ID:       "sturn",
				TraceID:  "t",
				ParentID: "sub",
				Kind:     domain.KindTurn,
				Name:     "subagent turn",
			},
			{
				ID:           "smc",
				TraceID:      "t",
				ParentID:     "sturn",
				Kind:         domain.KindModelCall,
				Name:         "subagent call",
				Status:       domain.StatusError,
				Model:        &domain.ModelIdentity{ProviderID: "p", ModelID: "small", Variant: domain.Estimated("v2", "r")},
				Usage:        &domain.Usage{InputTokens: domain.Observed(int64(1), "r")},
				FinishReason: domain.Inferred("length", "r"),
			},
			{
				ID:       "stool",
				TraceID:  "t",
				ParentID: "smc",
				Kind:     domain.KindToolCall,
				Name:     "subagent tool",
			},
			{
				ID:       "turn",
				TraceID:  "t",
				ParentID: "s",
				Kind:     domain.KindTurn,
				Name:     "turn",
			},
			{
				ID:       "mc",
				TraceID:  "t",
				ParentID: "turn",
				Kind:     domain.KindModelCall,
				Name:     "chat glm-5.3",
				Status:   domain.StatusOk,
				Model: &domain.ModelIdentity{
					ProviderID: "ollama-cloud",
					ModelID:    "glm-5.3",
					Variant:    domain.Unavailable[string](),
				},
				Usage: &domain.Usage{
					InputTokens:      domain.Observed(int64(10), "r"),
					OutputTokens:     domain.Derived(int64(5)),
					ReasoningTokens:  domain.Estimated(int64(3), "pricing-v1"),
					CacheReadTokens:  domain.Inferred(int64(2), "run-42"),
					CacheWriteTokens: domain.Unavailable[int64](),
				},
				Cost:         &domain.Cost{Amount: domain.Unavailable[float64](), Currency: "USD"},
				FinishReason: domain.Observed("stop", "r"),
				SourceRef:    "r",
			},
			{
				ID:       "tool",
				TraceID:  "t",
				ParentID: "mc",
				Kind:     domain.KindToolCall,
				Name:     "execute_tool bash",
				Status:   domain.StatusOk,
			},
			{
				ID:       "incomplete",
				TraceID:  "t",
				ParentID: "mc",
				Kind:     domain.KindToolCall,
				Name:     "incomplete span",
			},
		},
		Events: []*domain.Event{
			{ID: "e1", SpanID: "mc", Kind: domain.EventFileEdit, Time: end},
			{ID: "e2", SpanID: "turn", Kind: domain.EventSessionCompaction, Time: end},
			{ID: "e3", SpanID: "s", Kind: domain.EventTaskMarker, Time: end},
			{ID: "e4", SpanID: "smc", Kind: domain.EventFileEdit, Time: end},
		},
	}
}

func comprehensiveRaw() []*domain.RawEvent {
	return []*domain.RawEvent{
		{ID: "r1", TraceID: "t", Agent: "opencode", RecordType: "message", Payload: json.RawMessage(`{"role":"user","content":"hi"}`), CapturedAt: domain.FromEpochMillis(1100)},
		{ID: "r2", TraceID: "t", Agent: "opencode", RecordType: "tool_use", Payload: json.RawMessage(`{"tool":"bash","input":{"cmd":"ls"}}`), CapturedAt: domain.FromEpochMillis(1200)},
		{ID: "r3", TraceID: "t", Agent: "opencode", RecordType: "message", Payload: json.RawMessage(`{"role":"assistant","content":[{"type":"text","text":"done"}]}`), CapturedAt: domain.FromEpochMillis(1300)},
	}
}

func TestRoundTripByteIdentityComprehensive(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	tr := comprehensiveTrace()
	raw := comprehensiveRaw()

	pre, err := json.Marshal(tr)
	if err != nil {
		t.Fatalf("pre-marshal: %v", err)
	}
	if err := s.Ingest(ctx, tr, raw); err != nil {
		t.Fatalf("ingest: %v", err)
	}

	got, err := s.TraceByID(ctx, tr.ID)
	if err != nil {
		t.Fatalf("TraceByID: %v", err)
	}
	post, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("post-marshal: %v", err)
	}
	if !bytes.Equal(pre, post) {
		t.Errorf("trace not byte-identical:\npre:  %s\npost: %s", pre, post)
	}
}

func TestRawEnvelopeByteIdentity(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	tr := comprehensiveTrace()
	raw := comprehensiveRaw()
	if err := s.Ingest(ctx, tr, raw); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	got, err := s.RawEventsByTrace(ctx, tr.ID)
	if err != nil {
		t.Fatalf("RawEventsByTrace: %v", err)
	}
	if len(got) != len(raw) {
		t.Fatalf("raw count: got %d, want %d", len(got), len(raw))
	}
	for i := range raw {
		if got[i].ID != raw[i].ID {
			t.Errorf("raw[%d] id: got %q, want %q", i, got[i].ID, raw[i].ID)
		}
		if got[i].Agent != raw[i].Agent {
			t.Errorf("raw[%d] agent: got %q, want %q", i, got[i].Agent, raw[i].Agent)
		}
		if got[i].RecordType != raw[i].RecordType {
			t.Errorf("raw[%d] record_type: got %q, want %q", i, got[i].RecordType, raw[i].RecordType)
		}
		if !bytes.Equal(got[i].Payload, raw[i].Payload) {
			t.Errorf("raw[%d] payload byte mismatch:\ngot:  %s\nwant: %s", i, got[i].Payload, raw[i].Payload)
		}
		if domain.ToEpochMillis(got[i].CapturedAt) != domain.ToEpochMillis(raw[i].CapturedAt) {
			t.Errorf("raw[%d] captured_at: got %d, want %d", i, domain.ToEpochMillis(got[i].CapturedAt), domain.ToEpochMillis(raw[i].CapturedAt))
		}
	}
}

func fullTableSnapshot(t *testing.T, s *Store) string {
	t.Helper()
	var sb bytes.Buffer
	row := s.db.QueryRow(`SELECT COUNT(*) FROM traces`)
	var n int
	if err := row.Scan(&n); err != nil {
		t.Fatalf("trace count: %v", err)
	}
	sb.WriteString("traces:")
	sb.WriteString(itoa(n))
	rows, err := s.db.Query(`SELECT id, seq, blob FROM spans ORDER BY trace_id, seq`)
	if err != nil {
		t.Fatalf("spans: %v", err)
	}
	for rows.Next() {
		var id string
		var seq int
		var blob string
		if err := rows.Scan(&id, &seq, &blob); err != nil {
			t.Fatalf("scan span: %v", err)
		}
		sb.WriteString(id)
		sb.WriteString(blob)
	}
	rows.Close()
	rows, err = s.db.Query(`SELECT id, seq, blob FROM events ORDER BY trace_id, seq`)
	if err != nil {
		t.Fatalf("events: %v", err)
	}
	for rows.Next() {
		var id string
		var seq int
		var blob string
		if err := rows.Scan(&id, &seq, &blob); err != nil {
			t.Fatalf("scan event: %v", err)
		}
		sb.WriteString(id)
		sb.WriteString(blob)
	}
	rows.Close()
	rows, err = s.db.Query(`SELECT id, seq, agent, record_type, captured_at, payload_sha256, payload FROM raw_events ORDER BY trace_id, seq`)
	if err != nil {
		t.Fatalf("raws: %v", err)
	}
	for rows.Next() {
		var id, agent, recordType, sha string
		var seq int
		var capturedAt int64
		var payload []byte
		if err := rows.Scan(&id, &seq, &agent, &recordType, &capturedAt, &sha, &payload); err != nil {
			t.Fatalf("scan raw: %v", err)
		}
		sb.WriteString(id)
		sb.WriteString(agent)
		sb.WriteString(recordType)
		sb.Write(payload)
	}
	rows.Close()
	return sb.String()
}

func TestDoubleIngestIdempotencyFullTable(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	tr := comprehensiveTrace()
	raw := comprehensiveRaw()
	if err := s.Ingest(ctx, tr, raw); err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	before := fullTableSnapshot(t, s)
	if err := s.Ingest(ctx, tr, raw); err != nil {
		t.Fatalf("second ingest: %v", err)
	}
	after := fullTableSnapshot(t, s)
	if before != after {
		t.Errorf("double ingest must be idempotent (full table), got divergence")
	}
}

func TestChangedContentReingestReplacesTreePreservesRaws(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	tr := comprehensiveTrace()
	raw := comprehensiveRaw()
	if err := s.Ingest(ctx, tr, raw); err != nil {
		t.Fatalf("first ingest: %v", err)
	}

	rawsBefore := rawCount(t, s, "t")
	spansBefore := spanCount(t, s, "t")

	tr.Spans[1].Name = "subagent-renamed"
	tr.Events = append(tr.Events, &domain.Event{ID: "e5", SpanID: "mc", Kind: domain.EventFileEdit, Time: tr.EndTime})
	raw = append(raw, &domain.RawEvent{ID: "r4", TraceID: "t", Agent: "opencode", RecordType: "message", Payload: json.RawMessage(`{"role":"assistant"}`), CapturedAt: domain.FromEpochMillis(1400)})

	if err := s.Ingest(ctx, tr, raw); err != nil {
		t.Fatalf("re-ingest: %v", err)
	}

	if rawCount(t, s, "t") != rawsBefore+1 {
		t.Errorf("raws preserved+appended: got %d, want %d", rawCount(t, s, "t"), rawsBefore+1)
	}
	if spanCount(t, s, "t") != spansBefore {
		t.Errorf("span count should be unchanged after rename: got %d, want %d", spanCount(t, s, "t"), spansBefore)
	}

	got, err := s.TraceByID(ctx, "t")
	if err != nil {
		t.Fatalf("TraceByID: %v", err)
	}
	found := false
	for _, sp := range got.Spans {
		if sp.ID == "sub" && sp.Name == "subagent-renamed" {
			found = true
		}
	}
	if !found {
		t.Error("renamed span not reflected in reassembled trace")
	}
}

func rawCount(t *testing.T, s *Store, traceID string) int {
	t.Helper()
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM raw_events WHERE trace_id = ?`, traceID).Scan(&n); err != nil {
		t.Fatalf("raw count: %v", err)
	}
	return n
}

func spanCount(t *testing.T, s *Store, traceID string) int {
	t.Helper()
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM spans WHERE trace_id = ?`, traceID).Scan(&n); err != nil {
		t.Fatalf("span count: %v", err)
	}
	return n
}

func TestRawEventsImmutableProbe(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	tr := comprehensiveTrace()
	raw := comprehensiveRaw()
	if err := s.Ingest(ctx, tr, raw); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if _, err := s.db.Exec(`UPDATE raw_events SET agent = 'evil' WHERE id = 'r1'`); err == nil {
		t.Error("UPDATE envelope column should abort, got nil")
	}
	if _, err := s.db.Exec(`UPDATE raw_events SET payload = X'00' WHERE id = 'r1'`); err == nil {
		t.Error("UPDATE payload should abort, got nil")
	}
	if _, err := s.db.Exec(`DELETE FROM raw_events WHERE id = 'r1'`); err == nil {
		t.Error("DELETE should abort, got nil")
	}
	if rawCount(t, s, "t") != 3 {
		t.Errorf("raw count after probes: got %d, want 3", rawCount(t, s, "t"))
	}
}

func TestListTracesFilterExactMatch(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	a := comprehensiveTrace()
	a.ID = "t1"
	retagTrace(a, "t1")
	a.Source.ProjectDirectory = "/proj/x"
	b := comprehensiveTrace()
	b.ID = "t2"
	retagTrace(b, "t2")
	b.Source.ProjectDirectory = "/proj/xy"
	if err := s.Ingest(ctx, a, nil); err != nil {
		t.Fatalf("ingest a: %v", err)
	}
	if err := s.Ingest(ctx, b, nil); err != nil {
		t.Fatalf("ingest b: %v", err)
	}
	got, err := s.ListTraces(ctx, domain.TraceFilter{ProjectDirectory: "/proj/x"})
	if err != nil {
		t.Fatalf("ListTraces: %v", err)
	}
	if len(got) != 1 || got[0].ID != "t1" {
		t.Fatalf("exact filter should return only t1, got %+v", got)
	}
}

func retagTrace(tr *domain.Trace, id string) {
	for _, sp := range tr.Spans {
		sp.TraceID = id
	}
}

func TestErrNotFoundMapping(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	if _, err := s.TraceByID(ctx, "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("TraceByID missing: got %v", err)
	}
	if _, err := s.RawEventsByTrace(ctx, "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("RawEventsByTrace missing: got %v", err)
	}
}

func BenchmarkLargeTraceIngest(b *testing.B) {
	s := openBench(b)
	ctx := context.Background()
	tr, raw := syntheticTrace(500)
	pre, err := json.Marshal(tr)
	if err != nil {
		b.Fatalf("marshal: %v", err)
	}
	b.Logf("synthetic trace: %d spans, %d raws, %d bytes", len(tr.Spans), len(raw), len(pre))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := s.Ingest(ctx, tr, raw); err != nil {
			b.Fatalf("ingest: %v", err)
		}
	}
}

func openBench(b *testing.B) *Store {
	b.Helper()
	path := b.TempDir() + "/bench.db"
	s, err := Open(path)
	if err != nil {
		b.Fatalf("open: %v", err)
	}
	b.Cleanup(func() { s.Close() })
	return s
}

func syntheticTrace(messages int) (*domain.Trace, []*domain.RawEvent) {
	start := domain.FromEpochMillis(1000)
	tr := &domain.Trace{
		ID:        "bench",
		Agent:     "opencode",
		StartTime: start,
		EndTime:   start.Add(time.Duration(messages) * time.Second),
		Source: domain.SourceMetadata{
			RootSessionID:    "session",
			ProjectDirectory: "/proj",
		},
		Spans: []*domain.Span{
			{ID: "s", TraceID: "bench", Kind: domain.KindSession, Name: "session"},
		},
	}
	raw := make([]*domain.RawEvent, 0, messages*10)
	for i := 0; i < messages; i++ {
		turnID := "turn-" + itoa(i)
		mcID := "mc-" + itoa(i)
		toolID := "tool-" + itoa(i)
		tool2ID := "tool2-" + itoa(i)
		attrs := map[string]any{"content": bigPadding()}
		tr.Spans = append(tr.Spans,
			&domain.Span{ID: turnID, TraceID: "bench", ParentID: "s", Kind: domain.KindTurn, Name: "turn", Attributes: attrs},
			&domain.Span{
				ID:         mcID,
				TraceID:    "bench",
				ParentID:   turnID,
				Kind:       domain.KindModelCall,
				Name:       "chat",
				Status:     domain.StatusOk,
				Attributes: attrs,
				Model:      &domain.ModelIdentity{ProviderID: "p", ModelID: "m", Variant: domain.Unavailable[string]()},
				Usage: &domain.Usage{
					InputTokens:      domain.Observed(int64(100), "r"),
					OutputTokens:     domain.Derived(int64(50)),
					ReasoningTokens:  domain.Estimated(int64(10), "p"),
					CacheReadTokens:  domain.Unavailable[int64](),
					CacheWriteTokens: domain.Unavailable[int64](),
				},
				Cost:         &domain.Cost{Amount: domain.Unavailable[float64](), Currency: "USD"},
				FinishReason: domain.Observed("stop", "r"),
			},
			&domain.Span{ID: toolID, TraceID: "bench", ParentID: mcID, Kind: domain.KindToolCall, Name: "tool", Attributes: attrs},
			&domain.Span{ID: tool2ID, TraceID: "bench", ParentID: mcID, Kind: domain.KindToolCall, Name: "tool", Attributes: attrs},
		)
		for j := 0; j < 10; j++ {
			raw = append(raw, &domain.RawEvent{
				ID:         "r-" + itoa(i) + "-" + itoa(j),
				TraceID:    "bench",
				Agent:      "opencode",
				RecordType: "message",
				Payload:    json.RawMessage(`{"seq":` + itoa(j) + `,"content":"` + padding() + `"}`),
				CapturedAt: start.Add(time.Duration(i)*time.Second + time.Duration(j)*time.Millisecond),
			})
		}
	}
	return tr, raw
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func padding() string {
	return "Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation."
}

func bigPadding() string {
	return padding() + padding() + padding() + padding() + padding()
}
