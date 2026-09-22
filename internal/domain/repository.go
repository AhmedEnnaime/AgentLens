package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound = errors.New("agentlens: trace not found")
	ErrConflict = errors.New("agentlens: raw event id conflicts with different payload")
)

type TraceFilter struct {
	ProjectDirectory string
}

type TraceSummary struct {
	ID               string
	Agent            string
	RootSessionID    string
	ProjectDirectory string
	StartTime        time.Time
	EndTime          time.Time
	SpanCount        int
	EventCount       int
	RawEventCount    int
}

type TraceIngestor interface {
	Ingest(ctx context.Context, trace *Trace, raw []*RawEvent) error
}

type TraceReader interface {
	TraceByID(ctx context.Context, id string) (*Trace, error)
	ListTraces(ctx context.Context, f TraceFilter) ([]TraceSummary, error)
	RawEventsByTrace(ctx context.Context, traceID string) ([]*RawEvent, error)
}
