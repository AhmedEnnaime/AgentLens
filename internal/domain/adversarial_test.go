package domain

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

func TestZeroCostObservedIsValid(t *testing.T) {
	c := &Cost{Amount: Observed(0.0, "ref"), Currency: "USD"}
	if err := c.Validate(); err != nil {
		t.Errorf("observed zero cost should be valid, got: %v", err)
	}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	amount := m["amount"].(map[string]any)
	if amount["provenance"] != "observed" {
		t.Errorf("observed zero cost provenance = %v, want observed", amount["provenance"])
	}
	if amount["value"] != float64(0) {
		t.Errorf("observed zero cost value = %v, want 0", amount["value"])
	}
}

func TestUnavailableCostNeverRendersAsObservedZero(t *testing.T) {
	c := &Cost{Amount: Unavailable[float64](), Currency: "USD"}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	amount := m["amount"].(map[string]any)
	if amount["provenance"] != "unavailable" {
		t.Errorf("unavailable cost provenance = %v, want unavailable", amount["provenance"])
	}
	if v, ok := amount["value"]; !ok || v != nil {
		t.Errorf("unavailable cost value = %v (present=%v), want null", v, ok)
	}
	var back Cost
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal round-trip: %v", err)
	}
	if back.Amount.Provenance != ProvenanceUnavailable {
		t.Errorf("round-trip provenance = %s, want unavailable", back.Amount.Provenance)
	}
	if back.Amount.V != 0 {
		t.Errorf("round-trip V = %v, want 0 (never a fabricated observed zero)", back.Amount.V)
	}
}

func TestFinishReasonOpenVocabularyRoundTrip(t *testing.T) {
	for _, reason := range []string{"tool-calls", "stop", "length", "unknown", "content_filter", "some-future-value"} {
		v := Observed(reason, "ref")
		data, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal %q: %v", reason, err)
		}
		var got Value[string]
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("unmarshal %q: %v", reason, err)
		}
		if got.V != reason {
			t.Errorf("finish reason round-trip = %q, want %q", got.V, reason)
		}
		if got.Provenance != ProvenanceObserved {
			t.Errorf("finish reason provenance = %s, want observed", got.Provenance)
		}
	}
}

func TestIncompleteModelCallZeroEndTime(t *testing.T) {
	tr := happyTrace()
	tr.Spans[2].EndTime = time.Time{}
	if err := tr.Validate(); err != nil {
		t.Errorf("model_call with zero EndTime should validate, got: %v", err)
	}
	data, err := json.Marshal(tr.Spans[2])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	raw, ok := m["end_time"].(string)
	if !ok {
		t.Fatalf("end_time key missing or wrong type: %v", m)
	}
	if raw != "0001-01-01T00:00:00Z" {
		t.Errorf("zero EndTime = %q, want explicit zero-time string (distinguishable from absent key)", raw)
	}
}

func TestSubagentChildSessionTreeShape(t *testing.T) {
	tr := happyTrace()
	tr.Spans = append(tr.Spans, session("child", "s"))
	if err := tr.Validate(); err != nil {
		t.Errorf("session parented by session should validate, got: %v", err)
	}
}

func TestChildSessionMissingParentFails(t *testing.T) {
	tr := happyTrace()
	tr.Spans = append(tr.Spans, session("child", "missing-parent"))
	err := tr.Validate()
	if err == nil {
		t.Fatal("child session with missing parent should fail validation")
	}
	if !strings.Contains(err.Error(), "does not resolve") {
		t.Errorf("error %q should mention unresolved parent", err)
	}
}

func TestCapabilitiesCacheWithoutPerCallFails(t *testing.T) {
	c := Capabilities{CacheTokenUsage: true, PerCallUsage: false}
	if err := c.Validate(); err == nil {
		t.Error("cache token usage without per-call usage should fail")
	}
	r := Capabilities{ReasoningTokenUsage: true, PerCallUsage: false}
	if err := r.Validate(); err == nil {
		t.Error("reasoning token usage without per-call usage should fail")
	}
	both := Capabilities{CacheTokenUsage: true, ReasoningTokenUsage: true, PerCallUsage: true}
	if err := both.Validate(); err != nil {
		t.Errorf("cache+reasoning with per-call should be valid: %v", err)
	}
}

func TestValueGenericsAcrossTypes(t *testing.T) {
	intV := Observed(int64(42), "r")
	if intV.V != int64(42) {
		t.Errorf("int64 V = %v", intV.V)
	}
	floatV := Observed(3.14, "r")
	if floatV.V != 3.14 {
		t.Errorf("float64 V = %v", floatV.V)
	}
	strV := Observed("hello", "r")
	if strV.V != "hello" {
		t.Errorf("string V = %v", strV.V)
	}
	for _, v := range []any{intV, floatV, strV} {
		data, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal %T: %v", v, err)
		}
		if !strings.Contains(string(data), `"provenance":"observed"`) {
			t.Errorf("%T json missing provenance key: %s", v, data)
		}
	}
}

func TestValueAllConstructorsJSONProvenanceKeyAlwaysPresent(t *testing.T) {
	values := []any{
		Observed(1, "r"),
		Derived(2),
		Estimated(3, "pricing"),
		Inferred(4, "run"),
		Unavailable[int](),
	}
	for _, v := range values {
		data, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal %T: %v", v, err)
		}
		if !strings.Contains(string(data), `"provenance":`) {
			t.Errorf("%T json missing provenance key: %s", v, data)
		}
	}
}

func TestUnavailableMarshalsValueAsNullAcrossTypes(t *testing.T) {
	for _, v := range []any{Unavailable[int64](), Unavailable[float64](), Unavailable[string]()} {
		data, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal %T: %v", v, err)
		}
		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("unmarshal %T: %v", v, err)
		}
		if val, ok := m["value"]; !ok || val != nil {
			t.Errorf("%T unavailable value = %v (present=%v), want null", v, val, ok)
		}
	}
}

func TestErrorsJoinReportsAllThreeViolations(t *testing.T) {
	tr := happyTrace()
	tr.ID = ""
	tr.Spans[1].ParentID = "nope"
	tr.Spans[3].Cost = &Cost{Amount: Unavailable[float64]()}
	err := tr.Validate()
	if err == nil {
		t.Fatal("expected errors")
	}
	msg := err.Error()
	for _, want := range []string{"empty id", "parent", "cost"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error should report %q violation, got: %s", want, msg)
		}
	}
}

func TestRawEventInvalidJSONFails(t *testing.T) {
	r := &RawEvent{ID: "r", TraceID: "t", Agent: "a", RecordType: "message", Payload: json.RawMessage(`{"unterminated"`)}
	if err := r.Validate(); err == nil {
		t.Error("invalid JSON payload should fail")
	}
}

func TestRawEventExtraFieldsPreservedByteIdentically(t *testing.T) {
	orig := json.RawMessage(`{"id":"x","custom_unknown":{"nested":[1,2]},"extra":"kept","z":null}`)
	r := RawEvent{ID: "r", TraceID: "t", Agent: "a", RecordType: "message", Payload: orig}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back RawEvent
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if string(back.Payload) != string(orig) {
		t.Errorf("payload byte-identity lost:\n got %s\nwant %s", back.Payload, orig)
	}
}

func TestEpochMillisBoundaryFidelity(t *testing.T) {
	for _, ms := range []int64{0, 1, 1789080895833, -1, -1789080895833} {
		if got := ToEpochMillis(FromEpochMillis(ms)); got != ms {
			t.Errorf("round-trip %d = %d", ms, got)
		}
	}
}

func TestEpochMillisLargeValueDoesNotOverflow(t *testing.T) {
	tm := FromEpochMillis(math.MaxInt64)
	if tm.Year() <= 1970 {
		t.Errorf("math.MaxInt64 ms overflowed to year %d (should be far future)", tm.Year())
	}
	if tm.Nanosecond()%int(time.Millisecond) != 0 {
		t.Errorf("ms precision lost for MaxInt64: ns=%d", tm.Nanosecond())
	}
}

func TestEnumJSONUnknownStringErrors(t *testing.T) {
	var sk SpanKind
	if err := json.Unmarshal([]byte(`"bogus"`), &sk); err == nil {
		t.Error("unknown span kind should error, not silently default")
	}
	var ek EventKind
	if err := json.Unmarshal([]byte(`"bogus"`), &ek); err == nil {
		t.Error("unknown event kind should error")
	}
	var st Status
	if err := json.Unmarshal([]byte(`"bogus"`), &st); err == nil {
		t.Error("unknown status should error")
	}
	var p Provenance
	if err := json.Unmarshal([]byte(`"bogus"`), &p); err == nil {
		t.Error("unknown provenance should error")
	}
}

func TestEventAttachmentRules(t *testing.T) {
	fileEditOnModelCall := happyTrace()
	fileEditOnModelCall.Events = []*Event{{ID: "e1", SpanID: "mc", Kind: EventFileEdit}}
	if err := fileEditOnModelCall.Validate(); err != nil {
		t.Errorf("file_edit on model_call should be OK: %v", err)
	}

	fileEditOnSession := happyTrace()
	fileEditOnSession.Events = []*Event{{ID: "e1", SpanID: "s", Kind: EventFileEdit}}
	if err := fileEditOnSession.Validate(); err == nil {
		t.Error("file_edit on session span should fail")
	}

	compactionOnTurn := happyTrace()
	compactionOnTurn.Events = []*Event{{ID: "e1", SpanID: "turn", Kind: EventSessionCompaction}}
	if err := compactionOnTurn.Validate(); err != nil {
		t.Errorf("session_compaction on turn should be OK: %v", err)
	}

	taskMarkerOnSession := happyTrace()
	taskMarkerOnSession.Events = []*Event{{ID: "e1", SpanID: "s", Kind: EventTaskMarker}}
	if err := taskMarkerOnSession.Validate(); err != nil {
		t.Errorf("task_marker on session should be OK: %v", err)
	}

	taskMarkerOnTurn := happyTrace()
	taskMarkerOnTurn.Events = []*Event{{ID: "e1", SpanID: "turn", Kind: EventTaskMarker}}
	if err := taskMarkerOnTurn.Validate(); err == nil {
		t.Error("task_marker on turn should fail")
	}
}

func TestEmptyAndDuplicateIDs(t *testing.T) {
	emptySpan := happyTrace()
	emptySpan.Spans[0].ID = ""
	if err := emptySpan.Validate(); err == nil {
		t.Error("empty span id should fail")
	}

	dupSpan := happyTrace()
	dupSpan.Spans[2].ID = "s"
	if err := dupSpan.Validate(); err == nil {
		t.Error("duplicate span id should fail")
	}

	emptyEvent := happyTrace()
	emptyEvent.Events[0].ID = ""
	if err := emptyEvent.Validate(); err == nil {
		t.Error("empty event id should fail")
	}

	dupEvent := happyTrace()
	dupEvent.Events = append(dupEvent.Events, &Event{ID: "e1", SpanID: "mc", Kind: EventFileEdit})
	if err := dupEvent.Validate(); err == nil {
		t.Error("duplicate event id should fail")
	}
}

func TestNilSafety(t *testing.T) {
	tr := &Trace{ID: "t", Agent: "opencode", Spans: nil, Events: nil}
	if err := tr.Validate(); err == nil {
		t.Error("trace with no spans should fail (no root)")
	}

	nilSpan := happyTrace()
	nilSpan.Spans = append(nilSpan.Spans, nil)
	if err := nilSpan.Validate(); err == nil {
		t.Error("trace with nil span should fail")
	}

	nilEvent := happyTrace()
	nilEvent.Events = append(nilEvent.Events, nil)
	if err := nilEvent.Validate(); err == nil {
		t.Error("trace with nil event should fail")
	}

	var nilTrace *Trace
	if err := nilTrace.Validate(); err == nil {
		t.Error("nil trace should fail")
	}
}

func TestNilAttributesMarshal(t *testing.T) {
	span := &Span{ID: "s", Kind: KindSession, Attributes: nil}
	data, err := json.Marshal(span)
	if err != nil {
		t.Fatalf("marshal nil attributes: %v", err)
	}
	if strings.Contains(string(data), `"attributes"`) {
		t.Errorf("nil attributes should be omitted, got: %s", data)
	}
}
