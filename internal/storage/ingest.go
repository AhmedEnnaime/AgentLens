package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/AhmedEnnaime/AgentLens/internal/domain"
)

type traceHeader struct {
	ID           string                `json:"id"`
	Agent        string                `json:"agent"`
	StartTime    time.Time             `json:"start_time"`
	EndTime      time.Time             `json:"end_time"`
	Attributes   map[string]any        `json:"attributes,omitempty"`
	Source       domain.SourceMetadata `json:"source"`
	Capabilities domain.Capabilities   `json:"capabilities"`
}

func (s *Store) Ingest(ctx context.Context, trace *domain.Trace, raw []*domain.RawEvent) error {
	if err := trace.Validate(); err != nil {
		return err
	}
	if trace.Events != nil && len(trace.Events) == 0 {
		return fmt.Errorf("ingest: trace %s has empty non-nil events", trace.ID)
	}
	if trace.StartTime.IsZero() {
		return fmt.Errorf("ingest: trace %s has zero start_time", trace.ID)
	}
	for _, r := range raw {
		if r == nil {
			return fmt.Errorf("ingest: nil raw event")
		}
		if err := r.Validate(); err != nil {
			return err
		}
		if r.TraceID != trace.ID {
			return fmt.Errorf("ingest: raw event %s trace id %q does not match trace %q", r.ID, r.TraceID, trace.ID)
		}
		if r.CapturedAt.IsZero() {
			return fmt.Errorf("ingest: raw event %s has zero captured_at", r.ID)
		}
	}

	headerJSON, err := json.Marshal(traceHeader{
		ID:           trace.ID,
		Agent:        trace.Agent,
		StartTime:    trace.StartTime,
		EndTime:      trace.EndTime,
		Attributes:   trace.Attributes,
		Source:       trace.Source,
		Capabilities: trace.Capabilities,
	})
	if err != nil {
		return fmt.Errorf("ingest: marshal trace header: %w", err)
	}
	contentSHA, err := contentSHA256(trace)
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := ingestRawEvents(ctx, tx, trace.ID, raw); err != nil {
		return err
	}

	var existingSHA sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT content_sha256 FROM traces WHERE id = ?`, trace.ID).Scan(&existingSHA)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	treeChanged := !existingSHA.Valid || existingSHA.String != contentSHA

	if treeChanged {
		if _, err := tx.ExecContext(ctx, `DELETE FROM events WHERE trace_id = ?`, trace.ID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM spans WHERE trace_id = ?`, trace.ID); err != nil {
			return err
		}
	}

	endMillis := sql.NullInt64{}
	if !trace.EndTime.IsZero() {
		endMillis = sql.NullInt64{Int64: domain.ToEpochMillis(trace.EndTime), Valid: true}
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO traces (id, agent, root_session_id, project_directory, start_time, end_time, content_sha256, header)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			agent = excluded.agent,
			root_session_id = excluded.root_session_id,
			project_directory = excluded.project_directory,
			start_time = excluded.start_time,
			end_time = excluded.end_time,
			content_sha256 = excluded.content_sha256,
			header = excluded.header`,
		trace.ID,
		trace.Agent,
		trace.Source.RootSessionID,
		trace.Source.ProjectDirectory,
		domain.ToEpochMillis(trace.StartTime),
		endMillis,
		contentSHA,
		string(headerJSON),
	); err != nil {
		return err
	}

	if treeChanged {
		if err := ingestTree(ctx, tx, trace); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func contentSHA256(trace *domain.Trace) (string, error) {
	h := sha256.New()
	for _, sp := range trace.Spans {
		b, err := json.Marshal(sp)
		if err != nil {
			return "", fmt.Errorf("ingest: marshal span %s: %w", sp.ID, err)
		}
		h.Write(b)
	}
	for _, ev := range trace.Events {
		b, err := json.Marshal(ev)
		if err != nil {
			return "", fmt.Errorf("ingest: marshal event %s: %w", ev.ID, err)
		}
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func ingestRawEvents(ctx context.Context, tx *sql.Tx, traceID string, raw []*domain.RawEvent) error {
	maxSeq, err := maxRawSeq(ctx, tx, traceID)
	if err != nil {
		return err
	}
	nextSeq := maxSeq + 1
	for _, r := range raw {
		sha := payloadSHA256(r.Payload)
		var existing string
		err := tx.QueryRowContext(ctx, `SELECT payload_sha256 FROM raw_events WHERE trace_id = ? AND id = ?`, traceID, r.ID).Scan(&existing)
		switch {
		case err == sql.ErrNoRows:
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO raw_events (trace_id, id, seq, agent, record_type, captured_at, payload, payload_sha256)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				traceID, r.ID, nextSeq, r.Agent, r.RecordType, domain.ToEpochMillis(r.CapturedAt), []byte(r.Payload), sha,
			); err != nil {
				return err
			}
			nextSeq++
		case err != nil:
			return err
		case existing != sha:
			return domain.ErrConflict
		}
	}
	return nil
}

func maxRawSeq(ctx context.Context, tx *sql.Tx, traceID string) (int, error) {
	var m sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT MAX(seq) FROM raw_events WHERE trace_id = ?`, traceID).Scan(&m); err != nil {
		return 0, err
	}
	if !m.Valid {
		return -1, nil
	}
	return int(m.Int64), nil
}

func ingestTree(ctx context.Context, tx *sql.Tx, trace *domain.Trace) error {
	for seq, sp := range trace.Spans {
		blob, err := json.Marshal(sp)
		if err != nil {
			return fmt.Errorf("ingest: marshal span %s: %w", sp.ID, err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO spans (trace_id, id, seq, blob) VALUES (?, ?, ?, ?)`,
			trace.ID, sp.ID, seq, string(blob),
		); err != nil {
			return err
		}
	}
	for seq, ev := range trace.Events {
		blob, err := json.Marshal(ev)
		if err != nil {
			return fmt.Errorf("ingest: marshal event %s: %w", ev.ID, err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO events (trace_id, id, span_id, seq, blob) VALUES (?, ?, ?, ?, ?)`,
			trace.ID, ev.ID, ev.SpanID, seq, string(blob),
		); err != nil {
			return err
		}
	}
	return nil
}

func payloadSHA256(payload json.RawMessage) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}
