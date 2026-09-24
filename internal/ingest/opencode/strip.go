package opencode

import (
	"bytes"
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
		return stripMessage(mode, sourcePath, payload)
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

func stripMessage(mode config.PrivacyMode, sourcePath string, payload []byte) ([]byte, []domain.ContentRef, error) {
	env, err := parseObject(payload)
	if err != nil {
		return nil, nil, fmt.Errorf("opencode: message: %w", err)
	}
	rowID, err := fieldString(env, "id")
	if err != nil {
		return nil, nil, fmt.Errorf("opencode: message: %w", err)
	}
	dataRaw, ok := env["data"]
	if !ok {
		return nil, nil, fmt.Errorf("opencode: message: missing data")
	}
	data, err := parseObject(dataRaw)
	if err != nil {
		return nil, nil, fmt.Errorf("opencode: message: %w", err)
	}
	refs, err := stripMessageData(data, sourcePath, rowID)
	if err != nil {
		return nil, nil, err
	}
	if len(refs) == 0 {
		return payload, nil, nil
	}
	if mode == config.PrivacyContentLocal {
		return payload, refs, nil
	}
	return rebuildEnvelope(env, data, refs)
}

var (
	summaryKeys   = keySet("diffs")
	diffEntryKeys = keySet("file", "patch", "additions", "deletions", "status")
	errorKeys     = keySet("name", "data")
	errorDataKeys = keySet("message")
)

func keySet(keys ...string) map[string]bool {
	m := make(map[string]bool, len(keys))
	for _, k := range keys {
		m[k] = true
	}
	return m
}

func stripMessageData(data map[string]json.RawMessage, sourcePath, rowID string) ([]domain.ContentRef, error) {
	var refs []domain.ContentRef
	if raw, ok := data["summary"]; ok {
		out, r, err := stripSummary(raw, sourcePath, rowID)
		if err != nil {
			return nil, fmt.Errorf("opencode: message: summary: %w", err)
		}
		refs = append(refs, r...)
		if out != nil {
			data["summary"] = out
		}
	}
	if raw, ok := data["error"]; ok {
		out, r, err := stripError(raw, sourcePath, rowID)
		if err != nil {
			return nil, fmt.Errorf("opencode: message: error: %w", err)
		}
		refs = append(refs, r...)
		if out != nil {
			data["error"] = out
		}
	}
	return refs, nil
}

func stripSummary(raw json.RawMessage, sourcePath, rowID string) (json.RawMessage, []domain.ContentRef, error) {
	switch kindOf(raw) {
	case valNull, valBool, valNumber:
		return nil, nil, nil
	case valString:
		if isEmptyString(raw) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("string form is unclassifiable")
	case valArray:
		if isEmptyArray(raw) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("array form is unclassifiable")
	}
	obj, err := parseObject(raw)
	if err != nil {
		return nil, nil, err
	}
	if err := requireKeys(obj, "summary", summaryKeys); err != nil {
		return nil, nil, err
	}
	diffsRaw, ok := obj["diffs"]
	if !ok {
		return nil, nil, nil
	}
	out, refs, err := stripDiffs(diffsRaw, sourcePath, rowID)
	if err != nil {
		return nil, nil, err
	}
	if len(refs) == 0 {
		return nil, nil, nil
	}
	obj["diffs"] = out
	reb, err := marshalObject(obj)
	if err != nil {
		return nil, nil, err
	}
	return reb, refs, nil
}

func stripDiffs(raw json.RawMessage, sourcePath, rowID string) (json.RawMessage, []domain.ContentRef, error) {
	if kindOf(raw) == valArray {
		var entries []json.RawMessage
		if err := json.Unmarshal(raw, &entries); err != nil {
			return nil, nil, err
		}
		if len(entries) == 0 {
			return nil, nil, nil
		}
		var refs []domain.ContentRef
		for i, e := range entries {
			out, r, err := stripDiffEntry(e, sourcePath, rowID)
			if err != nil {
				return nil, nil, err
			}
			if out != nil {
				entries[i] = out
			}
			refs = append(refs, r...)
		}
		if len(refs) == 0 {
			return nil, nil, nil
		}
		arrBytes, err := json.Marshal(entries)
		if err != nil {
			return nil, nil, err
		}
		return arrBytes, refs, nil
	}
	return stripPureText(raw, sourcePath, rowID)
}

func stripDiffEntry(raw json.RawMessage, sourcePath, rowID string) (json.RawMessage, []domain.ContentRef, error) {
	if kindOf(raw) != valObject {
		return stripPureText(raw, sourcePath, rowID)
	}
	obj, err := parseObject(raw)
	if err != nil {
		return nil, nil, err
	}
	if err := requireKeys(obj, "diff entry", diffEntryKeys); err != nil {
		return nil, nil, err
	}
	patchRaw, ok := obj["patch"]
	if !ok {
		return nil, nil, nil
	}
	out, refs, err := stripPureText(patchRaw, sourcePath, rowID)
	if err != nil {
		return nil, nil, err
	}
	if len(refs) == 0 {
		return nil, nil, nil
	}
	obj["patch"] = out
	reb, err := marshalObject(obj)
	if err != nil {
		return nil, nil, err
	}
	return reb, refs, nil
}

func stripError(raw json.RawMessage, sourcePath, rowID string) (json.RawMessage, []domain.ContentRef, error) {
	switch kindOf(raw) {
	case valNull, valBool, valNumber:
		return nil, nil, nil
	case valString:
		if isEmptyString(raw) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("string form is unclassifiable")
	case valArray:
		if isEmptyArray(raw) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("array form is unclassifiable")
	}
	obj, err := parseObject(raw)
	if err != nil {
		return nil, nil, err
	}
	if err := requireKeys(obj, "error", errorKeys); err != nil {
		return nil, nil, err
	}
	dataRaw, ok := obj["data"]
	if !ok {
		return nil, nil, nil
	}
	out, refs, err := stripErrorData(dataRaw, sourcePath, rowID)
	if err != nil {
		return nil, nil, err
	}
	if len(refs) == 0 {
		return nil, nil, nil
	}
	obj["data"] = out
	reb, err := marshalObject(obj)
	if err != nil {
		return nil, nil, err
	}
	return reb, refs, nil
}

func stripErrorData(raw json.RawMessage, sourcePath, rowID string) (json.RawMessage, []domain.ContentRef, error) {
	switch kindOf(raw) {
	case valNull, valBool, valNumber:
		return nil, nil, nil
	case valString:
		if isEmptyString(raw) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("string form is unclassifiable")
	case valArray:
		if isEmptyArray(raw) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("array form is unclassifiable")
	}
	obj, err := parseObject(raw)
	if err != nil {
		return nil, nil, err
	}
	if err := requireKeys(obj, "error.data", errorDataKeys); err != nil {
		return nil, nil, err
	}
	msgRaw, ok := obj["message"]
	if !ok {
		return nil, nil, nil
	}
	out, refs, err := stripPureText(msgRaw, sourcePath, rowID)
	if err != nil {
		return nil, nil, err
	}
	if len(refs) == 0 {
		return nil, nil, nil
	}
	obj["message"] = out
	reb, err := marshalObject(obj)
	if err != nil {
		return nil, nil, err
	}
	return reb, refs, nil
}

func stripPureText(raw json.RawMessage, sourcePath, rowID string) (json.RawMessage, []domain.ContentRef, error) {
	switch kindOf(raw) {
	case valNull, valBool, valNumber:
		return nil, nil, nil
	case valString:
		if isEmptyString(raw) {
			return nil, nil, nil
		}
	case valArray:
		if isEmptyArray(raw) {
			return nil, nil, nil
		}
	case valObject:
	default:
		return nil, nil, fmt.Errorf("unhandled value kind")
	}
	ref := buildRef(sourcePath, "message", rowID, raw)
	return refWrapper(ref), []domain.ContentRef{ref}, nil
}

type jsonValueKind int

const (
	valNull jsonValueKind = iota
	valBool
	valNumber
	valString
	valArray
	valObject
)

func kindOf(raw json.RawMessage) jsonValueKind {
	b := bytes.TrimSpace([]byte(raw))
	if len(b) == 0 {
		return valNull
	}
	switch b[0] {
	case '{':
		return valObject
	case '[':
		return valArray
	case '"':
		return valString
	case 't', 'f':
		return valBool
	case 'n':
		return valNull
	default:
		return valNumber
	}
}

func requireKeys(obj map[string]json.RawMessage, location string, allowed map[string]bool) error {
	for k := range obj {
		if !allowed[k] {
			return fmt.Errorf("%s: unknown key %q", location, k)
		}
	}
	return nil
}

func isEmptyString(raw json.RawMessage) bool {
	return string(raw) == `""`
}

func isEmptyArray(raw json.RawMessage) bool {
	return string(raw) == "[]"
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
