package domain

import "fmt"

type SpanKind uint8

const (
	KindSession SpanKind = iota + 1
	KindTurn
	KindModelCall
	KindToolCall
)

func (k SpanKind) String() string {
	switch k {
	case KindSession:
		return "session"
	case KindTurn:
		return "turn"
	case KindModelCall:
		return "model_call"
	case KindToolCall:
		return "tool_call"
	default:
		return fmt.Sprintf("span_kind(%d)", uint8(k))
	}
}

func (k SpanKind) OTelKind() string {
	switch k {
	case KindModelCall:
		return "CLIENT"
	default:
		return "INTERNAL"
	}
}

func (k SpanKind) known() bool {
	switch k {
	case KindSession, KindTurn, KindModelCall, KindToolCall:
		return true
	}
	return false
}

type EventKind uint8

const (
	EventFileEdit EventKind = iota + 1
	EventSessionCompaction
	EventTaskMarker
)

func (k EventKind) String() string {
	switch k {
	case EventFileEdit:
		return "file_edit"
	case EventSessionCompaction:
		return "session_compaction"
	case EventTaskMarker:
		return "task_marker"
	default:
		return fmt.Sprintf("event_kind(%d)", uint8(k))
	}
}

func (k EventKind) known() bool {
	switch k {
	case EventFileEdit, EventSessionCompaction, EventTaskMarker:
		return true
	}
	return false
}
