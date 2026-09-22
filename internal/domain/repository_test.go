package domain

import (
	"context"
	"errors"
	"testing"
)

type ingestStub struct{}

func (ingestStub) Ingest(_ context.Context, _ *Trace, _ []*RawEvent) error {
	return nil
}

type readStub struct{}

func (readStub) TraceByID(_ context.Context, _ string) (*Trace, error) {
	return nil, ErrNotFound
}

func (readStub) ListTraces(_ context.Context, _ TraceFilter) ([]TraceSummary, error) {
	return nil, nil
}

func (readStub) RawEventsByTrace(_ context.Context, _ string) ([]*RawEvent, error) {
	return nil, nil
}

func TestTraceIngestorInterfaceCompliance(t *testing.T) {
	var _ TraceIngestor = ingestStub{}
	var s ingestStub
	if err := s.Ingest(context.Background(), nil, nil); err != nil {
		t.Errorf("stub ingest should succeed, got %v", err)
	}
}

func TestTraceReaderInterfaceCompliance(t *testing.T) {
	var _ TraceReader = readStub{}
	var s readStub
	if _, err := s.TraceByID(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("stub should return ErrNotFound, got %v", err)
	}
	summaries, err := s.ListTraces(context.Background(), TraceFilter{})
	if err != nil || summaries != nil {
		t.Errorf("stub list should return nil, nil; got %v, %v", summaries, err)
	}
	raw, err := s.RawEventsByTrace(context.Background(), "t")
	if err != nil || raw != nil {
		t.Errorf("stub raw should return nil, nil; got %v, %v", raw, err)
	}
}

func TestSentinelErrorsAreDistinct(t *testing.T) {
	if ErrNotFound == ErrConflict {
		t.Fatal("ErrNotFound and ErrConflict must be distinct")
	}
	if errors.Is(ErrNotFound, ErrConflict) || errors.Is(ErrConflict, ErrNotFound) {
		t.Fatal("sentinel errors must not wrap each other")
	}
}

func TestTraceFilterZeroValue(t *testing.T) {
	var f TraceFilter
	if f.ProjectDirectory != "" {
		t.Errorf("zero-value TraceFilter.ProjectDirectory should be empty, got %q", f.ProjectDirectory)
	}
}

func TestTraceSummaryZeroEndTimeIsOpen(t *testing.T) {
	s := TraceSummary{ID: "t"}
	if !s.EndTime.IsZero() {
		t.Errorf("zero EndTime should signal an open trace, got %v", s.EndTime)
	}
}
