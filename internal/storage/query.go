package storage

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/AhmedEnnaime/AgentLens/internal/domain"
)

var (
	_ domain.TraceIngestor = (*Store)(nil)
	_ domain.TraceReader   = (*Store)(nil)
)

func (s *Store) TraceByID(ctx context.Context, id string) (*domain.Trace, error) {
	var header string
	err := s.db.QueryRowContext(ctx, `SELECT header FROM traces WHERE id = ?`, id).Scan(&header)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var h traceHeader
	if err := json.Unmarshal([]byte(header), &h); err != nil {
		return nil, err
	}
	tr := &domain.Trace{
		ID:           h.ID,
		Agent:        h.Agent,
		StartTime:    h.StartTime,
		EndTime:      h.EndTime,
		Attributes:   h.Attributes,
		Source:       h.Source,
		Capabilities: h.Capabilities,
	}
	spans, err := s.spansByTrace(ctx, id)
	if err != nil {
		return nil, err
	}
	events, err := s.eventsByTrace(ctx, id)
	if err != nil {
		return nil, err
	}
	tr.Spans = spans
	tr.Events = events
	return tr, nil
}

func (s *Store) spansByTrace(ctx context.Context, traceID string) ([]*domain.Span, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT blob FROM spans WHERE trace_id = ? ORDER BY seq`, traceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Span
	for rows.Next() {
		var blob string
		if err := rows.Scan(&blob); err != nil {
			return nil, err
		}
		var sp domain.Span
		if err := json.Unmarshal([]byte(blob), &sp); err != nil {
			return nil, err
		}
		out = append(out, &sp)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) eventsByTrace(ctx context.Context, traceID string) ([]*domain.Event, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT blob FROM events WHERE trace_id = ? ORDER BY seq`, traceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Event
	for rows.Next() {
		var blob string
		if err := rows.Scan(&blob); err != nil {
			return nil, err
		}
		var ev domain.Event
		if err := json.Unmarshal([]byte(blob), &ev); err != nil {
			return nil, err
		}
		out = append(out, &ev)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) ListTraces(ctx context.Context, f domain.TraceFilter) ([]domain.TraceSummary, error) {
	query := `
		SELECT t.id, t.agent, t.root_session_id, t.project_directory, t.start_time, t.end_time,
		       COUNT(DISTINCT s.id), COUNT(DISTINCT e.id), COUNT(DISTINCT r.id)
		FROM traces t
		LEFT JOIN spans s ON s.trace_id = t.id
		LEFT JOIN events e ON e.trace_id = t.id
		LEFT JOIN raw_events r ON r.trace_id = t.id`
	args := make([]any, 0, 1)
	if f.ProjectDirectory != "" {
		query += ` WHERE t.project_directory = ?`
		args = append(args, f.ProjectDirectory)
	}
	query += ` GROUP BY t.id ORDER BY t.start_time DESC, t.id`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	summaries := make([]domain.TraceSummary, 0)
	for rows.Next() {
		var sum domain.TraceSummary
		var startMillis int64
		var endMillis sql.NullInt64
		if err := rows.Scan(
			&sum.ID, &sum.Agent, &sum.RootSessionID, &sum.ProjectDirectory,
			&startMillis, &endMillis, &sum.SpanCount, &sum.EventCount, &sum.RawEventCount,
		); err != nil {
			return nil, err
		}
		sum.StartTime = domain.FromEpochMillis(startMillis)
		if endMillis.Valid {
			sum.EndTime = domain.FromEpochMillis(endMillis.Int64)
		}
		summaries = append(summaries, sum)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return summaries, nil
}

func (s *Store) RawEventsByTrace(ctx context.Context, traceID string) ([]*domain.RawEvent, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, agent, record_type, captured_at, payload FROM raw_events WHERE trace_id = ? ORDER BY seq`, traceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*domain.RawEvent, 0)
	for rows.Next() {
		var id, agent, recordType string
		var capturedAt int64
		var payload []byte
		if err := rows.Scan(&id, &agent, &recordType, &capturedAt, &payload); err != nil {
			return nil, err
		}
		out = append(out, &domain.RawEvent{
			ID:         id,
			TraceID:    traceID,
			Agent:      agent,
			RecordType: recordType,
			Payload:    json.RawMessage(payload),
			CapturedAt: domain.FromEpochMillis(capturedAt),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		var exists int
		err := s.db.QueryRowContext(ctx, `SELECT 1 FROM traces WHERE id = ?`, traceID).Scan(&exists)
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
