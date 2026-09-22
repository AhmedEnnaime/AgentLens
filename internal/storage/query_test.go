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

func TestTraceByIDNotFound(t *testing.T) {
	s := openTemp(t)
	_, err := s.TraceByID(context.Background(), "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestTraceByIDReassemblesByteIdentical(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	tr := validTrace("t1")
	raw := []*domain.RawEvent{rawEvent("r1", "t1", json.RawMessage(`{"a":1}`))}
	if err := s.Ingest(ctx, tr, raw); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	pre, err := json.Marshal(tr)
	if err != nil {
		t.Fatalf("pre-marshal: %v", err)
	}
	got, err := s.TraceByID(ctx, "t1")
	if err != nil {
		t.Fatalf("TraceByID: %v", err)
	}
	post, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("post-marshal: %v", err)
	}
	if !bytes.Equal(pre, post) {
		t.Errorf("reassembled trace not byte-identical:\npre:  %s\npost: %s", pre, post)
	}
}

func TestListTracesNewestFirst(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()

	older := validTrace("t1")
	older.StartTime = domain.FromEpochMillis(1000)
	older.EndTime = domain.FromEpochMillis(1500)
	older.Source.ProjectDirectory = "/proj/a"

	newer := validTrace("t2")
	newer.StartTime = domain.FromEpochMillis(3000)
	newer.EndTime = domain.FromEpochMillis(3500)
	newer.Source.ProjectDirectory = "/proj/b"

	if err := s.Ingest(ctx, older, nil); err != nil {
		t.Fatalf("ingest older: %v", err)
	}
	if err := s.Ingest(ctx, newer, nil); err != nil {
		t.Fatalf("ingest newer: %v", err)
	}

	all, err := s.ListTraces(ctx, domain.TraceFilter{})
	if err != nil {
		t.Fatalf("ListTraces: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 traces, got %d", len(all))
	}
	if all[0].ID != "t2" || all[1].ID != "t1" {
		t.Errorf("expected newest-first order [t2 t1], got [%s %s]", all[0].ID, all[1].ID)
	}
}

func TestListTracesProjectFilter(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()

	a := validTrace("t1")
	a.Source.ProjectDirectory = "/proj/a"
	b := validTrace("t2")
	b.Source.ProjectDirectory = "/proj/b"

	if err := s.Ingest(ctx, a, nil); err != nil {
		t.Fatalf("ingest a: %v", err)
	}
	if err := s.Ingest(ctx, b, nil); err != nil {
		t.Fatalf("ingest b: %v", err)
	}

	filtered, err := s.ListTraces(ctx, domain.TraceFilter{ProjectDirectory: "/proj/a"})
	if err != nil {
		t.Fatalf("filtered ListTraces: %v", err)
	}
	if len(filtered) != 1 || filtered[0].ID != "t1" {
		t.Fatalf("expected only t1, got %+v", filtered)
	}

	empty, err := s.ListTraces(ctx, domain.TraceFilter{ProjectDirectory: "/proj/none"})
	if err != nil {
		t.Fatalf("empty ListTraces: %v", err)
	}
	if empty == nil {
		t.Error("empty filter result must be non-nil slice")
	}
	if len(empty) != 0 {
		t.Errorf("expected 0 traces, got %d", len(empty))
	}
}

func TestListTracesCounts(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	tr := validTrace("t1")
	raw := []*domain.RawEvent{
		rawEvent("r1", "t1", json.RawMessage(`{"a":1}`)),
		rawEvent("r2", "t1", json.RawMessage(`{"b":2}`)),
	}
	if err := s.Ingest(ctx, tr, raw); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	sums, err := s.ListTraces(ctx, domain.TraceFilter{})
	if err != nil {
		t.Fatalf("ListTraces: %v", err)
	}
	if len(sums) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(sums))
	}
	s0 := sums[0]
	if s0.SpanCount != len(tr.Spans) {
		t.Errorf("span count: got %d, want %d", s0.SpanCount, len(tr.Spans))
	}
	if s0.EventCount != len(tr.Events) {
		t.Errorf("event count: got %d, want %d", s0.EventCount, len(tr.Events))
	}
	if s0.RawEventCount != len(raw) {
		t.Errorf("raw event count: got %d, want %d", s0.RawEventCount, len(raw))
	}
	if !s0.StartTime.Equal(tr.StartTime) {
		t.Errorf("start time: got %v, want %v", s0.StartTime, tr.StartTime)
	}
	if !s0.EndTime.Equal(tr.EndTime) {
		t.Errorf("end time: got %v, want %v", s0.EndTime, tr.EndTime)
	}
}

func TestRawEventsByTraceNotFound(t *testing.T) {
	s := openTemp(t)
	_, err := s.RawEventsByTrace(context.Background(), "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestRawEventsByTraceEnvelopeAndOrder(t *testing.T) {
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
			CapturedAt: domain.FromEpochMillis(1100),
		},
		{
			ID:         "r2",
			TraceID:    "t1",
			Agent:      "claude",
			RecordType: "tool_use",
			Payload:    json.RawMessage(`{"b":2}`),
			CapturedAt: domain.FromEpochMillis(1200),
		},
	}
	if err := s.Ingest(ctx, tr, raw); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	got, err := s.RawEventsByTrace(ctx, "t1")
	if err != nil {
		t.Fatalf("RawEventsByTrace: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 raws, got %d", len(got))
	}
	if got[0].ID != "r1" || got[1].ID != "r2" {
		t.Errorf("raw events out of seq order: [%s %s]", got[0].ID, got[1].ID)
	}
	if got[0].Agent != "opencode" || got[0].RecordType != "message" {
		t.Errorf("envelope r1: agent=%q record=%q", got[0].Agent, got[0].RecordType)
	}
	if got[1].Agent != "claude" || got[1].RecordType != "tool_use" {
		t.Errorf("envelope r2: agent=%q record=%q", got[1].Agent, got[1].RecordType)
	}
	if !bytes.Equal(got[0].Payload, json.RawMessage(`{"a":1}`)) {
		t.Errorf("payload r1 byte mismatch: %s", got[0].Payload)
	}
	if domain.ToEpochMillis(got[0].CapturedAt) != 1100 {
		t.Errorf("captured_at r1: got %d, want 1100", domain.ToEpochMillis(got[0].CapturedAt))
	}
	if domain.ToEpochMillis(got[1].CapturedAt) != 1200 {
		t.Errorf("captured_at r2: got %d, want 1200", domain.ToEpochMillis(got[1].CapturedAt))
	}
}

func TestRawEventsByTracePreservesGrownSessionOrder(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	tr := validTrace("t1")
	raw := []*domain.RawEvent{rawEvent("r1", "t1", json.RawMessage(`{"a":1}`))}
	if err := s.Ingest(ctx, tr, raw); err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	raw = append(raw, rawEvent("r2", "t1", json.RawMessage(`{"c":3}`)))
	tr.Spans = append(tr.Spans, turnSpan("t1", "turn2", "s"), modelCallSpan("t1", "mc2", "turn2"))
	if err := s.Ingest(ctx, tr, raw); err != nil {
		t.Fatalf("grown ingest: %v", err)
	}
	got, err := s.RawEventsByTrace(ctx, "t1")
	if err != nil {
		t.Fatalf("RawEventsByTrace: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 raws, got %d", len(got))
	}
	if got[0].ID != "r1" || got[1].ID != "r2" {
		t.Errorf("append order across grown session: [%s %s]", got[0].ID, got[1].ID)
	}
}

func TestTraceByIDPreservesCapturedAtMsFidelity(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	tr := validTrace("t1")
	tr.StartTime = domain.FromEpochMillis(1234)
	raw := []*domain.RawEvent{rawEvent("r1", "t1", json.RawMessage(`{"a":1}`))}
	if err := s.Ingest(ctx, tr, raw); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	got, err := s.TraceByID(ctx, "t1")
	if err != nil {
		t.Fatalf("TraceByID: %v", err)
	}
	if !got.StartTime.Equal(tr.StartTime) {
		t.Errorf("start time ms fidelity: got %v, want %v", got.StartTime, tr.StartTime)
	}
	if !got.EndTime.Equal(tr.EndTime) {
		t.Errorf("end time ms fidelity: got %v, want %v", got.EndTime, tr.EndTime)
	}
}

func TestTraceByIDNoRaws(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	tr := validTrace("t1")
	if err := s.Ingest(ctx, tr, nil); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	got, err := s.TraceByID(ctx, "t1")
	if err != nil {
		t.Fatalf("TraceByID: %v", err)
	}
	if got.ID != "t1" {
		t.Errorf("id: got %q", got.ID)
	}
}

func TestRawEventsByTraceEmptyIsNotFound(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	tr := validTrace("t1")
	if err := s.Ingest(ctx, tr, nil); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	got, err := s.RawEventsByTrace(ctx, "t1")
	if err != nil {
		t.Fatalf("existing trace with no raws should not error, got %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("existing trace with no raws should yield empty non-nil slice, got %v", got)
	}
}

func TestListTracesScaleBenchmarkTrace(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	tr, raw := scaleTrace(2500)
	if err := s.Ingest(ctx, tr, raw); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	start := time.Now()
	sums, err := s.ListTraces(ctx, domain.TraceFilter{})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("ListTraces: %v", err)
	}
	if len(sums) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(sums))
	}
	if sums[0].SpanCount != len(tr.Spans) {
		t.Errorf("span count: got %d, want %d", sums[0].SpanCount, len(tr.Spans))
	}
	if sums[0].RawEventCount != len(raw) {
		t.Errorf("raw count: got %d, want %d", sums[0].RawEventCount, len(raw))
	}
	t.Logf("ListTraces over %d spans / %d raws: %v", len(tr.Spans), len(raw), elapsed)
	if elapsed >= time.Second {
		t.Errorf("ListTraces too slow: %v", elapsed)
	}
}

func scaleTrace(messages int) (*domain.Trace, []*domain.RawEvent) {
	start := domain.FromEpochMillis(1000)
	tr := &domain.Trace{
		ID:        "scale",
		Agent:     "opencode",
		StartTime: start,
		EndTime:   start.Add(time.Duration(messages) * time.Second),
		Source: domain.SourceMetadata{
			RootSessionID:    "session",
			ProjectDirectory: "/proj",
		},
		Spans: []*domain.Span{
			{ID: "s", TraceID: "scale", Kind: domain.KindSession, Name: "session"},
		},
	}
	raw := make([]*domain.RawEvent, 0, messages*2)
	for i := 0; i < messages; i++ {
		turnID := "turn-" + itoa(i)
		mcID := "mc-" + itoa(i)
		tr.Spans = append(tr.Spans,
			&domain.Span{ID: turnID, TraceID: "scale", ParentID: "s", Kind: domain.KindTurn, Name: "turn"},
			&domain.Span{
				ID:           mcID,
				TraceID:      "scale",
				ParentID:     turnID,
				Kind:         domain.KindModelCall,
				Name:         "chat",
				Status:       domain.StatusOk,
				Model:        &domain.ModelIdentity{ProviderID: "p", ModelID: "m", Variant: domain.Unavailable[string]()},
				Usage:        &domain.Usage{InputTokens: domain.Observed(int64(10), "r")},
				Cost:         &domain.Cost{Amount: domain.Unavailable[float64](), Currency: "USD"},
				FinishReason: domain.Observed("stop", "r"),
			},
		)
		raw = append(raw, &domain.RawEvent{
			ID:         "r-" + itoa(i),
			TraceID:    "scale",
			Agent:      "opencode",
			RecordType: "message",
			Payload:    json.RawMessage(`{"seq":` + itoa(i) + `}`),
			CapturedAt: start.Add(time.Duration(i) * time.Second),
		})
		raw = append(raw, &domain.RawEvent{
			ID:         "r2-" + itoa(i),
			TraceID:    "scale",
			Agent:      "opencode",
			RecordType: "message",
			Payload:    json.RawMessage(`{"seq":` + itoa(i) + `,"b":2}`),
			CapturedAt: start.Add(time.Duration(i) * time.Second),
		})
	}
	return tr, raw
}
