package domain

import (
	"encoding/json"
	"fmt"
	"time"
)

type RawEvent struct {
	ID         string          `json:"id"`
	TraceID    string          `json:"trace_id"`
	Agent      string          `json:"agent"`
	RecordType string          `json:"record_type"`
	Payload    json.RawMessage `json:"payload"`
	CapturedAt time.Time       `json:"captured_at"`
}

func (r *RawEvent) Validate() error {
	if r.ID == "" {
		return fmt.Errorf("raw event: empty id")
	}
	if r.TraceID == "" {
		return fmt.Errorf("raw event %s: empty trace id", r.ID)
	}
	if r.RecordType == "" {
		return fmt.Errorf("raw event %s: empty record type", r.ID)
	}
	if !json.Valid(r.Payload) {
		return fmt.Errorf("raw event %s: payload is not valid json", r.ID)
	}
	return nil
}
