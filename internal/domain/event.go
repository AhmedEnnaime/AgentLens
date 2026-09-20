package domain

import "time"

type Event struct {
	ID         string         `json:"id"`
	SpanID     string         `json:"span_id"`
	Kind       EventKind      `json:"kind"`
	Time       time.Time      `json:"time"`
	Attributes map[string]any `json:"attributes,omitempty"`
	SourceRef  string         `json:"source_ref"`
}
