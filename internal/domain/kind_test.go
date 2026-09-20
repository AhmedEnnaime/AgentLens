package domain

import (
	"encoding/json"
	"testing"
)

func TestSpanKindString(t *testing.T) {
	cases := []struct {
		k    SpanKind
		want string
	}{
		{KindSession, "session"},
		{KindTurn, "turn"},
		{KindModelCall, "model_call"},
		{KindToolCall, "tool_call"},
	}
	for _, c := range cases {
		if got := c.k.String(); got != c.want {
			t.Errorf("SpanKind(%d).String() = %q, want %q", uint8(c.k), got, c.want)
		}
	}
}

func TestSpanKindOTelKind(t *testing.T) {
	cases := []struct {
		k    SpanKind
		want string
	}{
		{KindSession, "INTERNAL"},
		{KindTurn, "INTERNAL"},
		{KindModelCall, "CLIENT"},
		{KindToolCall, "INTERNAL"},
	}
	for _, c := range cases {
		if got := c.k.OTelKind(); got != c.want {
			t.Errorf("SpanKind(%d).OTelKind() = %q, want %q", uint8(c.k), got, c.want)
		}
	}
}

func TestSpanKindJSON(t *testing.T) {
	for _, k := range []SpanKind{KindSession, KindTurn, KindModelCall, KindToolCall} {
		data, err := json.Marshal(k)
		if err != nil {
			t.Fatalf("marshal %s: %v", k, err)
		}
		if string(data) != `"`+k.String()+`"` {
			t.Errorf("marshal %s = %s", k, data)
		}
		var got SpanKind
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("unmarshal %s: %v", data, err)
		}
		if got != k {
			t.Errorf("round-trip: got %s, want %s", got, k)
		}
	}
	var k SpanKind
	if err := json.Unmarshal([]byte(`"bogus"`), &k); err == nil {
		t.Error("unmarshal unknown span kind should error")
	}
}

func TestEventKindString(t *testing.T) {
	cases := []struct {
		k    EventKind
		want string
	}{
		{EventFileEdit, "file_edit"},
		{EventSessionCompaction, "session_compaction"},
		{EventTaskMarker, "task_marker"},
	}
	for _, c := range cases {
		if got := c.k.String(); got != c.want {
			t.Errorf("EventKind(%d).String() = %q, want %q", uint8(c.k), got, c.want)
		}
	}
}

func TestEventKindJSON(t *testing.T) {
	for _, k := range []EventKind{EventFileEdit, EventSessionCompaction, EventTaskMarker} {
		data, err := json.Marshal(k)
		if err != nil {
			t.Fatalf("marshal %s: %v", k, err)
		}
		var got EventKind
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("unmarshal %s: %v", data, err)
		}
		if got != k {
			t.Errorf("round-trip: got %s, want %s", got, k)
		}
	}
}

func TestStatusString(t *testing.T) {
	cases := []struct {
		s    Status
		want string
	}{
		{StatusUnset, "unset"},
		{StatusOk, "ok"},
		{StatusError, "error"},
	}
	for _, c := range cases {
		if got := c.s.String(); got != c.want {
			t.Errorf("Status(%d).String() = %q, want %q", uint8(c.s), got, c.want)
		}
	}
}

func TestStatusJSON(t *testing.T) {
	for _, s := range []Status{StatusUnset, StatusOk, StatusError} {
		data, err := json.Marshal(s)
		if err != nil {
			t.Fatalf("marshal %s: %v", s, err)
		}
		var got Status
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("unmarshal %s: %v", data, err)
		}
		if got != s {
			t.Errorf("round-trip: got %s, want %s", got, s)
		}
	}
}
