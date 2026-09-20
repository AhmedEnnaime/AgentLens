package domain

import "time"

type Trace struct {
	ID           string         `json:"id"`
	Agent        string         `json:"agent"`
	StartTime    time.Time      `json:"start_time"`
	EndTime      time.Time      `json:"end_time"`
	Attributes   map[string]any `json:"attributes,omitempty"`
	Source       SourceMetadata `json:"source"`
	Capabilities Capabilities   `json:"capabilities"`
	Spans        []*Span        `json:"spans"`
	Events       []*Event       `json:"events"`
}

type spanIndex map[string]*Span

func buildSpanIndex(spans []*Span) spanIndex {
	idx := make(spanIndex, len(spans))
	for _, s := range spans {
		if s == nil {
			continue
		}
		idx[s.ID] = s
	}
	return idx
}
