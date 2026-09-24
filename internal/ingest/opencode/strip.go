package opencode

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/AhmedEnnaime/AgentLens/internal/config"
	"github.com/AhmedEnnaime/AgentLens/internal/domain"
)

const contentKey = "agentlens.content"

func StripContent(mode config.PrivacyMode, sourcePath, recordType string, payload []byte) ([]byte, []domain.ContentRef, error) {
	switch mode {
	case config.PrivacyMetadataOnly, config.PrivacyContentLocal:
	case config.PrivacyRedactedExport:
		return nil, nil, fmt.Errorf("opencode: redacted-export mode is not supported for import")
	default:
		return nil, nil, fmt.Errorf("opencode: unknown privacy mode %d", uint8(mode))
	}
	switch recordType {
	case "part":
		return stripPart(mode, sourcePath, payload)
	case "todo":
		return stripTodo(mode, sourcePath, payload)
	case "session":
		return stripSession(mode, sourcePath, payload)
	case "message":
		return payload, nil, nil
	default:
		return nil, nil, fmt.Errorf("opencode: unknown record type %q", recordType)
	}
}

func stripPart(mode config.PrivacyMode, sourcePath string, payload []byte) ([]byte, []domain.ContentRef, error) {
	env, err := parseObject(payload)
	if err != nil {
		return nil, nil, fmt.Errorf("opencode: part: %w", err)
	}
	rowID, err := fieldString(env, "id")
	if err != nil {
		return nil, nil, fmt.Errorf("opencode: part: %w", err)
	}
	dataRaw, ok := env["data"]
	if !ok {
		return nil, nil, fmt.Errorf("opencode: part: missing data")
	}
	data, err := parseObject(dataRaw)
	if err != nil {
		return nil, nil, fmt.Errorf("opencode: part: %w", err)
	}
	typ, err := fieldString(data, "type")
	if err != nil {
		return nil, nil, fmt.Errorf("opencode: part: %w", err)
	}
	switch typ {
	case "text", "reasoning":
		refs := stripText(data, sourcePath, rowID)
		if len(refs) == 0 {
			return payload, nil, nil
		}
		if mode == config.PrivacyContentLocal {
			return payload, refs, nil
		}
		return rebuildEnvelope(env, data, refs)
	case "tool":
		refs := stripToolState(data, sourcePath, rowID)
		if len(refs) == 0 {
			return payload, nil, nil
		}
		if mode == config.PrivacyContentLocal {
			return payload, refs, nil
		}
		return rebuildEnvelope(env, data, refs)
	case "patch", "compaction", "step-start", "step-finish":
		return payload, nil, nil
	default:
		return nil, nil, fmt.Errorf("opencode: part: unknown type %q", typ)
	}
}

func stripText(data map[string]json.RawMessage, sourcePath, rowID string) []domain.ContentRef {
	raw, ok := data["text"]
	if !ok || isEmptyContent(raw) {
		return nil
	}
	ref := buildRef(sourcePath, "part", rowID, raw)
	data["text"] = refWrapper(ref)
	return []domain.ContentRef{ref}
}

func stripToolState(data map[string]json.RawMessage, sourcePath, rowID string) []domain.ContentRef {
	stateRaw, ok := data["state"]
	if !ok {
		return nil
	}
	state, err := parseObject(stateRaw)
	if err != nil {
		return nil
	}
	fields := []string{"input", "output", "error"}
	var refs []domain.ContentRef
	for _, field := range fields {
		raw, ok := state[field]
		if !ok || isEmptyContent(raw) {
			continue
		}
		refs = append(refs, buildRef(sourcePath, "part", rowID, raw))
	}
	if len(refs) == 0 {
		return nil
	}
	i := 0
	for _, field := range fields {
		raw, ok := state[field]
		if !ok || isEmptyContent(raw) {
			continue
		}
		state[field] = refWrapper(refs[i])
		i++
	}
	stateBytes, err := marshalObject(state)
	if err != nil {
		return nil
	}
	data["state"] = stateBytes
	return refs
}

func stripTodo(mode config.PrivacyMode, sourcePath string, payload []byte) ([]byte, []domain.ContentRef, error) {
	env, err := parseObject(payload)
	if err != nil {
		return nil, nil, fmt.Errorf("opencode: todo: %w", err)
	}
	rowID, err := todoRowID(env)
	if err != nil {
		return nil, nil, fmt.Errorf("opencode: todo: %w", err)
	}
	raw, ok := env["content"]
	if !ok || isEmptyContent(raw) {
		return payload, nil, nil
	}
	ref := buildRef(sourcePath, "todo", rowID, raw)
	if mode == config.PrivacyContentLocal {
		return payload, []domain.ContentRef{ref}, nil
	}
	env["content"] = refWrapper(ref)
	out, err := marshalObject(env)
	if err != nil {
		return nil, nil, err
	}
	return out, []domain.ContentRef{ref}, nil
}

func stripSession(mode config.PrivacyMode, sourcePath string, payload []byte) ([]byte, []domain.ContentRef, error) {
	env, err := parseObject(payload)
	if err != nil {
		return nil, nil, fmt.Errorf("opencode: session: %w", err)
	}
	rowID, err := fieldString(env, "id")
	if err != nil {
		return nil, nil, fmt.Errorf("opencode: session: %w", err)
	}
	raw, ok := env["summary_diffs"]
	if !ok || isEmptyContent(raw) {
		return payload, nil, nil
	}
	ref := buildRef(sourcePath, "session", rowID, raw)
	if mode == config.PrivacyContentLocal {
		return payload, []domain.ContentRef{ref}, nil
	}
	env["summary_diffs"] = refWrapper(ref)
	out, err := marshalObject(env)
	if err != nil {
		return nil, nil, err
	}
	return out, []domain.ContentRef{ref}, nil
}

func rebuildEnvelope(env, data map[string]json.RawMessage, refs []domain.ContentRef) ([]byte, []domain.ContentRef, error) {
	dataBytes, err := marshalObject(data)
	if err != nil {
		return nil, nil, err
	}
	env["data"] = dataBytes
	out, err := marshalObject(env)
	if err != nil {
		return nil, nil, err
	}
	return out, refs, nil
}

func buildRef(sourcePath, table, rowID string, raw json.RawMessage) domain.ContentRef {
	return domain.ContentRef{
		Kind:   domain.ContentKindSQLiteRow,
		Path:   sourcePath,
		Table:  table,
		RowID:  rowID,
		SHA256: domain.ContentSHA256(raw),
		Bytes:  int64(len(raw)),
	}
}

func refWrapper(ref domain.ContentRef) json.RawMessage {
	refBytes, err := json.Marshal(ref)
	if err != nil {
		panic(err)
	}
	return json.RawMessage(`{"` + contentKey + `":` + string(refBytes) + `}`)
}

func parseObject(raw json.RawMessage) (map[string]json.RawMessage, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	if m == nil {
		return nil, fmt.Errorf("expected json object")
	}
	return m, nil
}

func marshalObject(m map[string]json.RawMessage) (json.RawMessage, error) {
	return json.Marshal(m)
}

func fieldString(m map[string]json.RawMessage, key string) (string, error) {
	raw, ok := m[key]
	if !ok {
		return "", fmt.Errorf("missing %s", key)
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", fmt.Errorf("expected string value for %s", key)
	}
	return s, nil
}

func todoRowID(env map[string]json.RawMessage) (string, error) {
	sessionID, err := fieldString(env, "session_id")
	if err != nil {
		return "", err
	}
	posRaw, ok := env["position"]
	if !ok {
		return "", fmt.Errorf("missing position")
	}
	var pos int64
	if err := json.Unmarshal(posRaw, &pos); err != nil {
		return "", fmt.Errorf("expected integer value for position")
	}
	return sessionID + ":" + strconv.FormatInt(pos, 10), nil
}

func isEmptyContent(raw json.RawMessage) bool {
	s := string(raw)
	return s == `""` || s == "null"
}
