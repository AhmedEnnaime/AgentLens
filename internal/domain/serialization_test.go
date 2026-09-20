package domain

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

func fullProvenanceTrace() *Trace {
	start := FromEpochMillis(1000)
	end := FromEpochMillis(2000)

	return &Trace{
		ID:        "t",
		Agent:     "opencode",
		StartTime: start,
		EndTime:   end,
		Spans: []*Span{
			session("s", ""),
			turn("turn", "s"),
			{
				ID:       "mc",
				TraceID:  "t",
				ParentID: "turn",
				Kind:     KindModelCall,
				Name:     "chat glm-5.3",
				Status:   StatusOk,
				EndTime:  time.Time{},
				Model: &ModelIdentity{
					ProviderID: "ollama-cloud",
					ModelID:    "glm-5.3",
					Variant:    Unavailable[string](),
				},
				Usage: &Usage{
					InputTokens:      Observed(int64(10), "r"),
					OutputTokens:     Observed(int64(5), "r"),
					ReasoningTokens:  Estimated(int64(3), "pricing-v1"),
					CacheReadTokens:  Derived(int64(2)),
					CacheWriteTokens: Inferred(int64(1), "run-42"),
				},
				Cost:         &Cost{Amount: Unavailable[float64](), Currency: "USD"},
				FinishReason: Observed("stop", "r"),
				SourceRef:    "r",
			},
			tool("tool", "mc"),
		},
		Events: []*Event{
			{ID: "e1", SpanID: "mc", Kind: EventFileEdit, Time: end},
			{ID: "e2", SpanID: "turn", Kind: EventSessionCompaction, Time: end},
			{ID: "e3", SpanID: "s", Kind: EventTaskMarker, Time: end},
		},
	}
}

func roundTripTrace(t *testing.T, tr *Trace) []byte {
	t.Helper()
	first, err := json.Marshal(tr)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back Trace
	if err := json.Unmarshal(first, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if err := back.Validate(); err != nil {
		t.Fatalf("round-tripped trace should validate, got: %v", err)
	}
	second, err := json.Marshal(&back)
	if err != nil {
		t.Fatalf("second marshal: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Errorf("round-trip not byte-identical:\nfirst:  %s\nsecond: %s", first, second)
	}
	return first
}

func TestSpanRoundTripStability(t *testing.T) {
	tr := fullProvenanceTrace()
	for _, s := range tr.Spans {
		t.Run(s.ID, func(t *testing.T) {
			first, err := json.Marshal(s)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var back Span
			if err := json.Unmarshal(first, &back); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			second, err := json.Marshal(&back)
			if err != nil {
				t.Fatalf("second marshal: %v", err)
			}
			if !bytes.Equal(first, second) {
				t.Errorf("span %s not byte-identical:\nfirst:  %s\nsecond: %s", s.ID, first, second)
			}
		})
	}
}

func TestTraceRoundTripStability(t *testing.T) {
	tr := fullProvenanceTrace()
	first := roundTripTrace(t, tr)
	if len(first) == 0 {
		t.Fatal("marshalled trace should not be empty")
	}
}
