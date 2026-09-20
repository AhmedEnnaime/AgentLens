package domain

import (
	"errors"
	"fmt"
)

func (t *Trace) Validate() error {
	if t == nil {
		return errors.New("trace: nil trace")
	}
	var errs []error
	if t.ID == "" {
		errs = append(errs, errors.New("trace: empty id"))
	}
	if err := t.Capabilities.Validate(); err != nil {
		errs = append(errs, err)
	}
	idx := buildSpanIndex(t.Spans)
	errs = append(errs, t.validateSpans(idx)...)
	errs = append(errs, t.validateTree(idx)...)
	errs = append(errs, t.validateEvents(idx)...)
	return errors.Join(errs...)
}

func (t *Trace) validateSpans(idx spanIndex) []error {
	var errs []error
	seen := make(map[string]struct{}, len(t.Spans))
	for _, s := range t.Spans {
		if s == nil {
			errs = append(errs, errors.New("span: nil span"))
			continue
		}
		if s.ID == "" {
			errs = append(errs, errors.New("span: empty id"))
			continue
		}
		if _, dup := seen[s.ID]; dup {
			errs = append(errs, fmt.Errorf("span %s: duplicate id", s.ID))
		}
		seen[s.ID] = struct{}{}
		if !s.Kind.known() {
			errs = append(errs, fmt.Errorf("span %s: unknown kind %d", s.ID, uint8(s.Kind)))
		}
		if !s.Status.known() {
			errs = append(errs, fmt.Errorf("span %s: unknown status %d", s.ID, uint8(s.Status)))
		}
		if !s.EndTime.IsZero() && s.EndTime.Before(s.StartTime) {
			errs = append(errs, fmt.Errorf("span %s: end time before start time", s.ID))
		}
		errs = append(errs, s.validateFields()...)
	}
	return errs
}

func (s *Span) validateFields() []error {
	var errs []error
	if err := s.FinishReason.Validate(); err != nil {
		errs = append(errs, fmt.Errorf("span %s: finish reason: %w", s.ID, err))
	}
	if s.Kind == KindModelCall {
		if s.Model == nil {
			errs = append(errs, fmt.Errorf("span %s: model_call requires model", s.ID))
		} else if err := s.Model.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("span %s: model: %w", s.ID, err))
		}
		if s.Usage == nil {
			errs = append(errs, fmt.Errorf("span %s: model_call requires usage", s.ID))
		} else if err := s.Usage.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("span %s: usage: %w", s.ID, err))
		}
	} else {
		if s.Model != nil {
			errs = append(errs, fmt.Errorf("span %s: model only allowed on model_call", s.ID))
		}
		if s.Usage != nil {
			errs = append(errs, fmt.Errorf("span %s: usage only allowed on model_call", s.ID))
		}
		if s.FinishReason.Provenance != ProvenanceUnavailable {
			errs = append(errs, fmt.Errorf("span %s: finish reason only allowed on model_call", s.ID))
		}
	}
	if s.Cost != nil {
		if s.Kind != KindSession && s.Kind != KindModelCall {
			errs = append(errs, fmt.Errorf("span %s: cost only allowed on session or model_call", s.ID))
		} else if err := s.Cost.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("span %s: cost: %w", s.ID, err))
		}
	}
	return errs
}

func (t *Trace) validateTree(idx spanIndex) []error {
	var errs []error
	roots := 0
	for _, s := range t.Spans {
		if s == nil {
			continue
		}
		if s.ParentID == "" {
			roots++
			if s.Kind != KindSession {
				errs = append(errs, fmt.Errorf("span %s: root must be a session span", s.ID))
			}
			continue
		}
		if !s.Kind.known() {
			continue
		}
		parent, ok := idx[s.ParentID]
		if !ok {
			errs = append(errs, fmt.Errorf("span %s: parent %s does not resolve", s.ID, s.ParentID))
			continue
		}
		if !validParentage(s.Kind, parent.Kind) {
			errs = append(errs, fmt.Errorf("span %s: kind %s cannot have parent kind %s", s.ID, s.Kind, parent.Kind))
		}
	}
	if roots != 1 {
		errs = append(errs, fmt.Errorf("trace: expected 1 root span, found %d", roots))
	}
	errs = append(errs, detectCycles(t.Spans, idx)...)
	return errs
}

func validParentage(child, parent SpanKind) bool {
	switch child {
	case KindSession:
		return parent == KindSession
	case KindTurn:
		return parent == KindSession
	case KindModelCall:
		return parent == KindTurn
	case KindToolCall:
		return parent == KindModelCall
	default:
		return false
	}
}

func detectCycles(spans []*Span, idx spanIndex) []error {
	var errs []error
	for _, s := range spans {
		if s == nil || s.ParentID == "" {
			continue
		}
		visited := map[string]struct{}{s.ID: {}}
		cur := s.ParentID
		for cur != "" {
			if _, seen := visited[cur]; seen {
				errs = append(errs, fmt.Errorf("span %s: parent cycle detected", s.ID))
				break
			}
			visited[cur] = struct{}{}
			parent, ok := idx[cur]
			if !ok {
				break
			}
			cur = parent.ParentID
		}
	}
	return errs
}

func (t *Trace) validateEvents(idx spanIndex) []error {
	var errs []error
	seen := make(map[string]struct{}, len(t.Events))
	for _, e := range t.Events {
		if e == nil {
			errs = append(errs, errors.New("event: nil event"))
			continue
		}
		if e.ID == "" {
			errs = append(errs, errors.New("event: empty id"))
			continue
		}
		if _, dup := seen[e.ID]; dup {
			errs = append(errs, fmt.Errorf("event %s: duplicate id", e.ID))
		}
		seen[e.ID] = struct{}{}
		if !e.Kind.known() {
			errs = append(errs, fmt.Errorf("event %s: unknown kind %d", e.ID, uint8(e.Kind)))
		}
		span, ok := idx[e.SpanID]
		if !ok {
			errs = append(errs, fmt.Errorf("event %s: span %s does not resolve", e.ID, e.SpanID))
			continue
		}
		switch e.Kind {
		case EventFileEdit, EventSessionCompaction:
			if span.Kind != KindTurn && span.Kind != KindModelCall {
				errs = append(errs, fmt.Errorf("event %s: %s must attach to a message-bearing span", e.ID, e.Kind))
			}
		case EventTaskMarker:
			if span.Kind != KindSession {
				errs = append(errs, fmt.Errorf("event %s: task_marker must attach to a session span", e.ID))
			}
		}
	}
	return errs
}
