package domain

import (
	"strings"
	"testing"
	"time"
)

func modelCall(id, parent string) *Span {
	return &Span{
		ID:       id,
		TraceID:  "t",
		ParentID: parent,
		Kind:     KindModelCall,
		Name:     "chat glm-5.3",
		Status:   StatusOk,
		Model: &ModelIdentity{
			ProviderID: "ollama-cloud",
			ModelID:    "glm-5.3",
			Variant:    Unavailable[string](),
		},
		Usage: &Usage{
			InputTokens:      Observed(int64(10), "r"),
			OutputTokens:     Observed(int64(5), "r"),
			ReasoningTokens:  Unavailable[int64](),
			CacheReadTokens:  Unavailable[int64](),
			CacheWriteTokens: Unavailable[int64](),
		},
		FinishReason: Observed("stop", "r"),
		SourceRef:    "r",
	}
}

func turn(id, parent string) *Span {
	return &Span{
		ID:        id,
		TraceID:   "t",
		ParentID:  parent,
		Kind:      KindTurn,
		Name:      "turn",
		Status:    StatusOk,
		SourceRef: "r",
	}
}

func session(id, parent string) *Span {
	return &Span{
		ID:        id,
		TraceID:   "t",
		ParentID:  parent,
		Kind:      KindSession,
		Name:      "session",
		Status:    StatusOk,
		SourceRef: "r",
	}
}

func tool(id, parent string) *Span {
	return &Span{
		ID:        id,
		TraceID:   "t",
		ParentID:  parent,
		Kind:      KindToolCall,
		Name:      "execute_tool bash",
		Status:    StatusOk,
		SourceRef: "r",
	}
}

func happyTrace() *Trace {
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
			modelCall("mc", "turn"),
			tool("tool", "mc"),
		},
		Events: []*Event{
			{ID: "e1", SpanID: "mc", Kind: EventFileEdit, Time: end},
		},
	}
}

func TestHappyTraceValid(t *testing.T) {
	if err := happyTrace().Validate(); err != nil {
		t.Errorf("happy trace should be valid, got: %v", err)
	}
}

func TestSubagentChildSessionValid(t *testing.T) {
	tr := happyTrace()
	tr.Spans = append(tr.Spans, session("child", "s"))
	tr.Events = append(tr.Events, &Event{ID: "e2", SpanID: "child", Kind: EventTaskMarker})
	if err := tr.Validate(); err != nil {
		t.Errorf("subagent child session trace should be valid, got: %v", err)
	}
}

func TestViolationCases(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*Trace)
		wantSub string
	}{
		{
			"dangling parent",
			func(tr *Trace) { tr.Spans[1].ParentID = "nope" },
			"parent",
		},
		{
			"cycle",
			func(tr *Trace) {
				tr.Spans[0].ParentID = "turn"
			},
			"cycle",
		},
		{
			"two roots",
			func(tr *Trace) { tr.Spans[3].ParentID = "" },
			"root",
		},
		{
			"orphan root kind",
			func(tr *Trace) {
				tr.Spans[1].ParentID = ""
			},
			"root",
		},
		{
			"wrong parent kind",
			func(tr *Trace) { tr.Spans[3].ParentID = "turn" },
			"parent",
		},
		{
			"model on turn",
			func(tr *Trace) {
				tr.Spans[1].Model = &ModelIdentity{ProviderID: "p", ModelID: "m"}
			},
			"model",
		},
		{
			"cost on tool_call",
			func(tr *Trace) { tr.Spans[3].Cost = &Cost{Amount: Unavailable[float64]()} },
			"cost",
		},
		{
			"event span not resolving",
			func(tr *Trace) { tr.Events[0].SpanID = "missing" },
			"does not resolve",
		},
		{
			"task marker on non-session",
			func(tr *Trace) {
				tr.Events[0].Kind = EventTaskMarker
				tr.Events[0].SpanID = "mc"
			},
			"task_marker",
		},
		{
			"duplicate span ids",
			func(tr *Trace) { tr.Spans[2].ID = "s" },
			"duplicate",
		},
		{
			"duplicate event ids",
			func(tr *Trace) {
				tr.Events = append(tr.Events, &Event{ID: "e1", SpanID: "mc", Kind: EventFileEdit})
			},
			"duplicate",
		},
		{
			"end before start",
			func(tr *Trace) {
				tr.Spans[1].StartTime = FromEpochMillis(5000)
				tr.Spans[1].EndTime = FromEpochMillis(1000)
			},
			"end time",
		},
		{
			"unknown span kind",
			func(tr *Trace) { tr.Spans[1].Kind = SpanKind(200) },
			"unknown kind",
		},
		{
			"unknown status",
			func(tr *Trace) { tr.Spans[1].Status = Status(200) },
			"unknown status",
		},
		{
			"empty span id",
			func(tr *Trace) { tr.Spans[1].ID = "" },
			"empty id",
		},
		{
			"unknown event kind",
			func(tr *Trace) { tr.Events[0].Kind = EventKind(200) },
			"unknown kind",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tr := happyTrace()
			c.mutate(tr)
			err := tr.Validate()
			if err == nil {
				t.Fatalf("expected validation error for %s", c.name)
			}
			if !strings.Contains(err.Error(), c.wantSub) {
				t.Errorf("%s: error %q should contain %q", c.name, err, c.wantSub)
			}
		})
	}
}

func TestValidateReturnsAllViolations(t *testing.T) {
	tr := happyTrace()
	tr.Spans[1].ParentID = "nope"
	tr.Spans[3].Cost = &Cost{Amount: Unavailable[float64]()}
	err := tr.Validate()
	if err == nil {
		t.Fatal("expected errors")
	}
	msg := err.Error()
	if !strings.Contains(msg, "parent") {
		t.Errorf("error should report parent violation, got: %s", msg)
	}
	if !strings.Contains(msg, "cost") {
		t.Errorf("error should report cost violation, got: %s", msg)
	}
}

func TestIncompleteEndTimeAllowed(t *testing.T) {
	tr := happyTrace()
	tr.Spans[2].EndTime = time.Time{}
	if err := tr.Validate(); err != nil {
		t.Errorf("incomplete end time should be allowed, got: %v", err)
	}
}

func TestNilTraceValidate(t *testing.T) {
	var tr *Trace
	if err := tr.Validate(); err == nil {
		t.Error("nil trace should error on nil trace")
	}
}

func TestSpanTraceIDMustMatchTrace(t *testing.T) {
	tr := happyTrace()
	tr.Spans[1].TraceID = "other-trace"
	err := tr.Validate()
	if err == nil {
		t.Fatal("span with foreign TraceID should fail validation")
	}
	if !strings.Contains(err.Error(), "does not match trace") {
		t.Errorf("error %q should mention trace id mismatch", err)
	}
}

func TestModelCallInnerFieldWrappedErrors(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*Trace)
		wantSub string
	}{
		{
			"invalid model variant",
			func(tr *Trace) {
				tr.Spans[2].Model = &ModelIdentity{ProviderID: "p", ModelID: "m", Variant: Observed("max", "")}
			},
			"model",
		},
		{
			"invalid usage input tokens",
			func(tr *Trace) {
				tr.Spans[2].Usage = &Usage{InputTokens: Observed(int64(10), "")}
			},
			"usage",
		},
		{
			"invalid cost amount",
			func(tr *Trace) {
				tr.Spans[2].Cost = &Cost{Amount: Estimated(1.0, ""), Currency: "USD"}
			},
			"cost",
		},
		{
			"invalid finish reason",
			func(tr *Trace) {
				tr.Spans[2].FinishReason = Observed("stop", "")
			},
			"finish reason",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tr := happyTrace()
			c.mutate(tr)
			err := tr.Validate()
			if err == nil {
				t.Fatalf("expected validation error for %s", c.name)
			}
			if !strings.Contains(err.Error(), c.wantSub) {
				t.Errorf("%s: error %q should contain %q", c.name, err, c.wantSub)
			}
		})
	}
}
