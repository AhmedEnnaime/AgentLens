package domain

import "time"

type Span struct {
	ID            string         `json:"id"`
	TraceID       string         `json:"trace_id"`
	ParentID      string         `json:"parent_id"`
	Kind          SpanKind       `json:"kind"`
	Name          string         `json:"name"`
	StartTime     time.Time      `json:"start_time"`
	EndTime       time.Time      `json:"end_time"`
	Status        Status         `json:"status"`
	StatusMessage string         `json:"status_message"`
	Attributes    map[string]any `json:"attributes,omitempty"`
	Model         *ModelIdentity `json:"model,omitempty"`
	Usage         *Usage         `json:"usage,omitempty"`
	Cost          *Cost          `json:"cost,omitempty"`
	FinishReason  Value[string]  `json:"finish_reason"`
	SourceRef     string         `json:"source_ref"`
}
