package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRawEventValidateValid(t *testing.T) {
	payload := json.RawMessage(`{"id":"msg-1","unknown_field":{"nested":true}}`)
	r := &RawEvent{
		ID:         "re-1",
		TraceID:    "t",
		Agent:      "opencode",
		RecordType: "message",
		Payload:    payload,
	}
	if err := r.Validate(); err != nil {
		t.Errorf("valid raw event: %v", err)
	}
}

func TestRawEventInvalidJSON(t *testing.T) {
	r := &RawEvent{
		ID:         "re-1",
		TraceID:    "t",
		Agent:      "opencode",
		RecordType: "message",
		Payload:    json.RawMessage(`{not json`),
	}
	err := r.Validate()
	if err == nil {
		t.Fatal("invalid payload should error")
	}
	if !strings.Contains(err.Error(), "json") {
		t.Errorf("error %q should mention json", err)
	}
}

func TestRawEventEmptyFields(t *testing.T) {
	cases := []struct {
		name string
		r    RawEvent
		want string
	}{
		{"empty id", RawEvent{TraceID: "t", RecordType: "r", Payload: json.RawMessage(`{}`)}, "id"},
		{"empty trace", RawEvent{ID: "i", RecordType: "r", Payload: json.RawMessage(`{}`)}, "trace"},
		{"empty record type", RawEvent{ID: "i", TraceID: "t", Payload: json.RawMessage(`{}`)}, "record"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.r.Validate()
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %q should contain %q", err, c.want)
			}
		})
	}
}

func TestRawEventPayloadPreservedVerbatim(t *testing.T) {
	orig := json.RawMessage(`{"id":"x","custom_unknown":[1,2,3],"extra":"kept"}`)
	r := RawEvent{
		ID:         "re-1",
		TraceID:    "t",
		Agent:      "opencode",
		RecordType: "message",
		Payload:    orig,
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back RawEvent
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if string(back.Payload) != string(orig) {
		t.Errorf("payload not preserved: got %s, want %s", back.Payload, orig)
	}
	var pm map[string]any
	if err := json.Unmarshal(back.Payload, &pm); err != nil {
		t.Fatalf("re-parse payload: %v", err)
	}
	if _, ok := pm["custom_unknown"]; !ok {
		t.Errorf("unknown field lost: %v", pm)
	}
	if _, ok := pm["extra"]; !ok {
		t.Errorf("extra field lost: %v", pm)
	}
}
