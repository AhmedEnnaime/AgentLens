package opencode

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/AhmedEnnaime/AgentLens/internal/config"
	"github.com/AhmedEnnaime/AgentLens/internal/domain"
)

var fixtureNames = []string{"single-turn", "multi-turn-build", "subagent-tree", "tool-heavy", "cache-heavy"}

func fixturePath(name string) string {
	return filepath.Join("..", "..", "..", "testdata", "fixtures", "opencode", name, "fixture.db")
}

func openFixture(t *testing.T, name string) *sql.DB {
	t.Helper()
	path := fixturePath(name)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("fixture %s missing: %v", name, err)
	}
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		t.Fatalf("open fixture %s: %v", name, err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func recomputeSHA(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func assertRefIntegrity(t *testing.T, ref domain.ContentRef, sourcePath, table string, raw json.RawMessage) {
	t.Helper()
	if err := ref.Validate(); err != nil {
		t.Errorf("invalid ref: %v", err)
	}
	if ref.Path != sourcePath {
		t.Errorf("ref.Path = %q, want %q", ref.Path, sourcePath)
	}
	if ref.Table != table {
		t.Errorf("ref.Table = %q, want %q", ref.Table, table)
	}
	if ref.SHA256 != recomputeSHA(raw) {
		t.Errorf("ref.SHA256 = %s, want %s (independent recompute)", ref.SHA256, recomputeSHA(raw))
	}
	if ref.Bytes != int64(len(raw)) {
		t.Errorf("ref.Bytes = %d, want %d", ref.Bytes, len(raw))
	}
}

func TestStripContentFixturesMetadataOnly(t *testing.T) {
	sourcePath := "/source/opencode.db"
	for _, name := range fixtureNames {
		t.Run(name, func(t *testing.T) {
			db := openFixture(t, name)
			assertPartRows(t, db, config.PrivacyMetadataOnly, sourcePath, name)
			assertTodoRows(t, db, config.PrivacyMetadataOnly, sourcePath, name)
			assertSessionRows(t, db, config.PrivacyMetadataOnly, sourcePath, name)
			assertMessageRows(t, db, config.PrivacyMetadataOnly, sourcePath, name)
		})
	}
}

func TestStripContentFixturesContentLocal(t *testing.T) {
	sourcePath := "/source/opencode.db"
	for _, name := range fixtureNames {
		t.Run(name, func(t *testing.T) {
			db := openFixture(t, name)
			assertPartRows(t, db, config.PrivacyContentLocal, sourcePath, name)
			assertTodoRows(t, db, config.PrivacyContentLocal, sourcePath, name)
			assertSessionRows(t, db, config.PrivacyContentLocal, sourcePath, name)
			assertMessageRows(t, db, config.PrivacyContentLocal, sourcePath, name)
		})
	}
}

func assertPartRows(t *testing.T, db *sql.DB, mode config.PrivacyMode, sourcePath, fixture string) {
	t.Helper()
	rows, err := db.Query(`SELECT id, data FROM part`)
	if err != nil {
		t.Fatalf("query part: %v", err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var id string
		var data string
		if err := rows.Scan(&id, &data); err != nil {
			t.Fatalf("scan part: %v", err)
		}
		payload := partEnvelope(id, data)
		stripped, refs, err := StripContent(mode, sourcePath, "part", payload)
		if err != nil {
			t.Fatalf("strip part %s: %v", id, err)
		}
		verifyPartStrip(t, mode, sourcePath, payload, stripped, refs)
		count++
	}
	if count == 0 {
		t.Errorf("fixture %s has no part rows", fixture)
	}
	t.Logf("fixture %s: %d part rows", fixture, count)
}

func assertTodoRows(t *testing.T, db *sql.DB, mode config.PrivacyMode, sourcePath, fixture string) {
	t.Helper()
	rows, err := db.Query(`SELECT session_id, position, content, status, priority, time_created, time_updated FROM todo`)
	if err != nil {
		t.Fatalf("query todo: %v", err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var sessionID string
		var position int64
		var content string
		var status, priority string
		var timeCreated, timeUpdated int64
		if err := rows.Scan(&sessionID, &position, &content, &status, &priority, &timeCreated, &timeUpdated); err != nil {
			t.Fatalf("scan todo: %v", err)
		}
		payload := todoEnvelope(sessionID, position, content, status, priority, timeCreated, timeUpdated)
		stripped, refs, err := StripContent(mode, sourcePath, "todo", payload)
		if err != nil {
			t.Fatalf("strip todo %s:%d: %v", sessionID, position, err)
		}
		verifyTodoStrip(t, mode, sourcePath, payload, stripped, refs)
		count++
	}
	t.Logf("fixture %s: %d todo rows", fixture, count)
}

func assertSessionRows(t *testing.T, db *sql.DB, mode config.PrivacyMode, sourcePath, fixture string) {
	t.Helper()
	rows, err := db.Query(`SELECT id, title, summary_diffs FROM session`)
	if err != nil {
		t.Fatalf("query session: %v", err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var id, title string
		var summaryDiffs sql.NullString
		if err := rows.Scan(&id, &title, &summaryDiffs); err != nil {
			t.Fatalf("scan session: %v", err)
		}
		payload := sessionEnvelope(id, title, summaryDiffs)
		stripped, refs, err := StripContent(mode, sourcePath, "session", payload)
		if err != nil {
			t.Fatalf("strip session %s: %v", id, err)
		}
		verifySessionStrip(t, mode, sourcePath, payload, stripped, refs)
		count++
	}
	t.Logf("fixture %s: %d session rows", fixture, count)
}

var messageRefDriftAlarms = map[string]int{
	"single-turn":      0,
	"multi-turn-build": 54,
	"subagent-tree":    46,
	"tool-heavy":       1,
	"cache-heavy":      0,
}

func assertMessageRows(t *testing.T, db *sql.DB, mode config.PrivacyMode, sourcePath, fixture string) {
	t.Helper()
	rows, err := db.Query(`SELECT id, data FROM message`)
	if err != nil {
		t.Fatalf("query message: %v", err)
	}
	defer rows.Close()
	count := 0
	refCount := 0
	for rows.Next() {
		var id, data string
		if err := rows.Scan(&id, &data); err != nil {
			t.Fatalf("scan message: %v", err)
		}
		payload := messageEnvelope(id, data)
		stripped, refs, err := StripContent(mode, sourcePath, "message", payload)
		if err != nil {
			t.Fatalf("strip message %s: %v", id, err)
		}
		verifyMessageStrip(t, mode, sourcePath, payload, stripped, refs)
		refCount += len(refs)
		count++
	}
	if want, ok := messageRefDriftAlarms[fixture]; ok && refCount != want {
		t.Errorf("fixture %s: %d message refs, drift alarm expects exactly %d", fixture, refCount, want)
	}
	t.Logf("fixture %s: %d message rows, %d refs", fixture, count, refCount)
}

func verifyMessageStrip(t *testing.T, mode config.PrivacyMode, sourcePath string, payload, stripped []byte, refs []domain.ContentRef) {
	t.Helper()
	var srcEnv struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(payload, &srcEnv); err != nil {
		t.Fatalf("parse source message: %v", err)
	}
	var srcData map[string]json.RawMessage
	if err := json.Unmarshal(srcEnv.Data, &srcData); err != nil {
		t.Fatalf("parse source message data: %v", err)
	}
	expectedRefs := expectedMessageRefs(t, srcData, sourcePath)
	if mode == config.PrivacyContentLocal {
		if !bytes.Equal(stripped, payload) {
			t.Errorf("content-local message not byte-identical")
		}
		if len(refs) != len(expectedRefs) {
			t.Errorf("content-local message refs = %d, want %d", len(refs), len(expectedRefs))
		}
		for _, ref := range refs {
			assertRefIntegrity(t, ref, sourcePath, "message", rawForSHA(srcData, ref.SHA256))
		}
		return
	}
	if len(expectedRefs) == 0 {
		if len(refs) != 0 {
			t.Errorf("message produced %d refs, want 0", len(refs))
		}
		if !bytes.Equal(stripped, payload) {
			t.Errorf("nothing-stripped message should return original bytes")
		}
		return
	}
	if len(refs) != len(expectedRefs) {
		t.Errorf("message refs = %d, want %d", len(refs), len(expectedRefs))
	}
	if bytes.Equal(stripped, payload) {
		t.Errorf("content-bearing message should be stripped")
	}
	var outEnv struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(stripped, &outEnv); err != nil {
		t.Fatalf("parse stripped message: %v", err)
	}
	var outData map[string]json.RawMessage
	if err := json.Unmarshal(outEnv.Data, &outData); err != nil {
		t.Fatalf("parse stripped message data: %v", err)
	}
	assertMessageContentStripped(t, sourcePath, srcData, outData)
	for _, ref := range refs {
		assertRefIntegrity(t, ref, sourcePath, "message", rawForSHA(srcData, ref.SHA256))
	}
}

func expectedMessageRefs(t *testing.T, data map[string]json.RawMessage, sourcePath string) []domain.ContentRef {
	t.Helper()
	var refs []domain.ContentRef
	if raw, ok := data["summary"]; ok {
		refs = append(refs, expectedSummaryRefs(t, raw, sourcePath)...)
	}
	if raw, ok := data["error"]; ok {
		refs = append(refs, expectedErrorRefs(t, raw, sourcePath)...)
	}
	return refs
}

func expectedSummaryRefs(t *testing.T, raw json.RawMessage, sourcePath string) []domain.ContentRef {
	t.Helper()
	kind := kindOf(raw)
	switch kind {
	case valNull, valBool, valNumber, valString:
		return nil
	case valArray:
		if isEmptyArray(raw) {
			return nil
		}
		return nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("summary object: %v", err)
	}
	diffsRaw, ok := obj["diffs"]
	if !ok {
		return nil
	}
	return expectedDiffsRefs(t, diffsRaw, sourcePath)
}

func expectedDiffsRefs(t *testing.T, raw json.RawMessage, sourcePath string) []domain.ContentRef {
	t.Helper()
	if kindOf(raw) == valArray {
		var entries []json.RawMessage
		if err := json.Unmarshal(raw, &entries); err != nil {
			t.Fatalf("diffs array: %v", err)
		}
		var refs []domain.ContentRef
		for _, e := range entries {
			if kindOf(e) != valObject {
				continue
			}
			var em map[string]json.RawMessage
			if err := json.Unmarshal(e, &em); err != nil {
				t.Fatalf("diffs entry: %v", err)
			}
			patchRaw, ok := em["patch"]
			if !ok || isEmptyText(patchRaw) {
				continue
			}
			refs = append(refs, buildRef(sourcePath, "message", "row", patchRaw))
		}
		return refs
	}
	if isEmptyText(raw) {
		return nil
	}
	return []domain.ContentRef{buildRef(sourcePath, "message", "row", raw)}
}

func expectedErrorRefs(t *testing.T, raw json.RawMessage, sourcePath string) []domain.ContentRef {
	t.Helper()
	kind := kindOf(raw)
	switch kind {
	case valNull, valBool, valNumber, valString:
		return nil
	case valArray:
		return nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("error object: %v", err)
	}
	dataRaw, ok := obj["data"]
	if !ok {
		return nil
	}
	if kindOf(dataRaw) != valObject {
		return nil
	}
	var dataObj map[string]json.RawMessage
	if err := json.Unmarshal(dataRaw, &dataObj); err != nil {
		t.Fatalf("error.data object: %v", err)
	}
	msgRaw, ok := dataObj["message"]
	if !ok || isEmptyText(msgRaw) {
		return nil
	}
	return []domain.ContentRef{buildRef(sourcePath, "message", "row", msgRaw)}
}

func isEmptyText(raw json.RawMessage) bool {
	k := kindOf(raw)
	switch k {
	case valNull, valBool, valNumber:
		return true
	case valString:
		return string(raw) == `""`
	case valArray:
		return isEmptyArray(raw)
	default:
		return false
	}
}

func assertMessageContentStripped(t *testing.T, sourcePath string, src, out map[string]json.RawMessage) {
	t.Helper()
	if s, ok := src["summary"]; ok {
		assertSummaryStripped(t, sourcePath, s, out["summary"])
	}
	if e, ok := src["error"]; ok {
		assertErrorStripped(t, sourcePath, e, out["error"])
	}
	for k, v := range src {
		switch k {
		case "summary", "error":
			continue
		}
		if !bytes.Equal(out[k], v) {
			t.Errorf("non-content field %q altered", k)
		}
	}
}

func assertSummaryStripped(t *testing.T, sourcePath string, src, out json.RawMessage) {
	t.Helper()
	kind := kindOf(src)
	switch kind {
	case valNull, valBool, valNumber:
		if !bytes.Equal(out, src) {
			t.Errorf("text-incapable summary altered: %s -> %s", src, out)
		}
		return
	case valString, valArray:
		if isEmptyText(src) {
			if !bytes.Equal(out, src) {
				t.Errorf("empty summary altered: %s -> %s", src, out)
			}
		}
		return
	}
	var srcObj map[string]json.RawMessage
	if err := json.Unmarshal(src, &srcObj); err != nil {
		t.Fatalf("summary object: %v", err)
	}
	var outObj map[string]json.RawMessage
	if err := json.Unmarshal(out, &outObj); err != nil {
		t.Fatalf("stripped summary object: %v", err)
	}
	assertDiffsStripped(t, sourcePath, srcObj["diffs"], outObj["diffs"])
}

func assertDiffsStripped(t *testing.T, sourcePath string, src, out json.RawMessage) {
	t.Helper()
	if kindOf(src) == valArray {
		var srcEntries []json.RawMessage
		if err := json.Unmarshal(src, &srcEntries); err != nil {
			t.Fatalf("diffs array: %v", err)
		}
		var outEntries []json.RawMessage
		if err := json.Unmarshal(out, &outEntries); err != nil {
			t.Fatalf("stripped diffs array: %v", err)
		}
		if len(outEntries) != len(srcEntries) {
			t.Fatalf("diffs entries count = %d, want %d", len(outEntries), len(srcEntries))
		}
		for i := range srcEntries {
			assertDiffEntryStripped(t, sourcePath, srcEntries[i], outEntries[i])
		}
		return
	}
	if isEmptyText(src) {
		if !bytes.Equal(out, src) {
			t.Errorf("empty diffs altered")
		}
		return
	}
	assertRefAt(t, out, sourcePath, "message")
}

func assertDiffEntryStripped(t *testing.T, sourcePath string, src, out json.RawMessage) {
	t.Helper()
	if kindOf(src) != valObject {
		return
	}
	var srcObj map[string]json.RawMessage
	if err := json.Unmarshal(src, &srcObj); err != nil {
		t.Fatalf("diff entry: %v", err)
	}
	var outObj map[string]json.RawMessage
	if err := json.Unmarshal(out, &outObj); err != nil {
		t.Fatalf("stripped diff entry: %v", err)
	}
	for k, v := range srcObj {
		if k == "patch" {
			if isEmptyText(v) {
				if !bytes.Equal(outObj[k], v) {
					t.Errorf("empty patch altered")
				}
			} else {
				assertRefAt(t, outObj[k], sourcePath, "message")
			}
			continue
		}
		if !bytes.Equal(outObj[k], v) {
			t.Errorf("diff entry field %q altered: %s -> %s", k, v, outObj[k])
		}
	}
}

func assertErrorStripped(t *testing.T, sourcePath string, src, out json.RawMessage) {
	t.Helper()
	kind := kindOf(src)
	switch kind {
	case valNull, valBool, valNumber, valString, valArray:
		return
	}
	var srcObj map[string]json.RawMessage
	if err := json.Unmarshal(src, &srcObj); err != nil {
		t.Fatalf("error object: %v", err)
	}
	var outObj map[string]json.RawMessage
	if err := json.Unmarshal(out, &outObj); err != nil {
		t.Fatalf("stripped error object: %v", err)
	}
	if nameRaw, ok := srcObj["name"]; ok {
		if !bytes.Equal(outObj["name"], nameRaw) {
			t.Errorf("error.name altered: %s -> %s", nameRaw, outObj["name"])
		}
	}
	if dataRaw, ok := srcObj["data"]; ok {
		assertErrorDataStripped(t, sourcePath, dataRaw, outObj["data"])
	}
}

func assertErrorDataStripped(t *testing.T, sourcePath string, src, out json.RawMessage) {
	t.Helper()
	if kindOf(src) != valObject {
		return
	}
	var srcObj map[string]json.RawMessage
	if err := json.Unmarshal(src, &srcObj); err != nil {
		t.Fatalf("error.data object: %v", err)
	}
	var outObj map[string]json.RawMessage
	if err := json.Unmarshal(out, &outObj); err != nil {
		t.Fatalf("stripped error.data object: %v", err)
	}
	msgRaw, ok := srcObj["message"]
	if !ok {
		return
	}
	if isEmptyText(msgRaw) {
		if !bytes.Equal(outObj["message"], msgRaw) {
			t.Errorf("empty error.data.message altered")
		}
		return
	}
	assertRefAt(t, outObj["message"], sourcePath, "message")
}

func rawForSHA(data map[string]json.RawMessage, sha string) json.RawMessage {
	var found json.RawMessage
	var walk func(v json.RawMessage)
	walk = func(v json.RawMessage) {
		if found != nil {
			return
		}
		if recomputeSHA(v) == sha {
			found = v
			return
		}
		switch kindOf(v) {
		case valObject:
			var m map[string]json.RawMessage
			if json.Unmarshal(v, &m) == nil {
				for _, child := range m {
					walk(child)
				}
			}
		case valArray:
			var arr []json.RawMessage
			if json.Unmarshal(v, &arr) == nil {
				for _, child := range arr {
					walk(child)
				}
			}
		}
	}
	for _, v := range data {
		walk(v)
	}
	return found
}

func verifyPartStrip(t *testing.T, mode config.PrivacyMode, sourcePath string, payload, stripped []byte, refs []domain.ContentRef) {
	t.Helper()
	var dataEnvelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(payload, &dataEnvelope); err != nil {
		t.Fatalf("parse payload envelope: %v", err)
	}
	var data struct {
		Type  string          `json:"type"`
		Text  json.RawMessage `json:"text"`
		State json.RawMessage `json:"state"`
	}
	if err := json.Unmarshal(dataEnvelope.Data, &data); err != nil {
		t.Fatalf("parse payload data: %v", err)
	}
	switch data.Type {
	case "text", "reasoning":
		if isEmptyContent(data.Text) {
			if len(refs) != 0 {
				t.Errorf("empty text produced %d refs, want 0", len(refs))
			}
			if !bytes.Equal(stripped, payload) {
				t.Errorf("empty text payload should be unchanged")
			}
			return
		}
		if mode == config.PrivacyContentLocal {
			if !bytes.Equal(stripped, payload) {
				t.Errorf("content-local payload not byte-identical")
			}
		} else if bytes.Equal(stripped, payload) {
			t.Errorf("metadata-only %s payload should be stripped (unchanged bytes)", data.Type)
		}
	case "tool":
		if mode == config.PrivacyContentLocal {
			if !bytes.Equal(stripped, payload) {
				t.Errorf("content-local payload not byte-identical")
			}
		} else if bytes.Equal(stripped, payload) {
			t.Errorf("metadata-only %s payload should be stripped (unchanged bytes)", data.Type)
		}
	default:
		if !bytes.Equal(stripped, payload) {
			t.Errorf("non-content part type %s should pass through unchanged", data.Type)
		}
		if len(refs) != 0 {
			t.Errorf("non-content part type %s produced refs", data.Type)
		}
		return
	}
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(stripped, &env); err != nil {
		t.Fatalf("parse stripped: %v", err)
	}
	switch data.Type {
	case "text", "reasoning":
		if mode == config.PrivacyContentLocal {
			if len(refs) != 1 {
				t.Errorf("text part: %d refs, want 1", len(refs))
			}
		} else {
			var sd struct {
				Text json.RawMessage `json:"text"`
			}
			if err := json.Unmarshal(env.Data, &sd); err != nil {
				t.Fatalf("parse stripped data: %v", err)
			}
			assertRefAt(t, sd.Text, sourcePath, "part")
			if len(refs) != 1 {
				t.Errorf("text part: %d refs, want 1", len(refs))
			}
		}
		for _, ref := range refs {
			assertRefIntegrity(t, ref, sourcePath, "part", data.Text)
		}
	case "tool":
		var sd struct {
			State json.RawMessage `json:"state"`
		}
		if err := json.Unmarshal(env.Data, &sd); err != nil {
			t.Fatalf("parse stripped tool data: %v", err)
		}
		var origState map[string]json.RawMessage
		if err := json.Unmarshal(data.State, &origState); err != nil {
			t.Fatalf("parse orig state: %v", err)
		}
		var strippedState map[string]json.RawMessage
		if err := json.Unmarshal(sd.State, &strippedState); err != nil {
			t.Fatalf("parse stripped state: %v", err)
		}
		for _, field := range []string{"input", "output", "error"} {
			_, ok := origState[field]
			if !ok {
				continue
			}
			if mode == config.PrivacyMetadataOnly {
				assertRefAt(t, strippedState[field], sourcePath, "part")
			}
		}
		for _, ref := range refs {
			assertRefIntegrity(t, ref, sourcePath, "part", refContentForField(ref, origState))
		}
	}
}

func refContentForField(ref domain.ContentRef, origState map[string]json.RawMessage) json.RawMessage {
	for _, field := range []string{"input", "output", "error"} {
		raw, ok := origState[field]
		if !ok {
			continue
		}
		if recomputeSHA(raw) == ref.SHA256 {
			return raw
		}
	}
	return nil
}

func verifyTodoStrip(t *testing.T, mode config.PrivacyMode, sourcePath string, payload, stripped []byte, refs []domain.ContentRef) {
	t.Helper()
	var env struct {
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(payload, &env); err != nil {
		t.Fatalf("parse todo: %v", err)
	}
	if len(env.Content) == 0 || string(env.Content) == "null" {
		return
	}
	if mode == config.PrivacyContentLocal {
		if !bytes.Equal(stripped, payload) {
			t.Errorf("content-local todo not byte-identical")
		}
		if len(refs) != 1 {
			t.Errorf("todo: %d refs, want 1", len(refs))
		}
	} else {
		if bytes.Equal(stripped, payload) {
			t.Errorf("metadata-only todo should be stripped")
		}
		var s struct {
			Content json.RawMessage `json:"content"`
		}
		if err := json.Unmarshal(stripped, &s); err != nil {
			t.Fatalf("parse stripped todo: %v", err)
		}
		assertRefAt(t, s.Content, sourcePath, "todo")
	}
	for _, ref := range refs {
		assertRefIntegrity(t, ref, sourcePath, "todo", env.Content)
	}
}

func verifySessionStrip(t *testing.T, mode config.PrivacyMode, sourcePath string, payload, stripped []byte, refs []domain.ContentRef) {
	t.Helper()
	var env struct {
		SummaryDiffs json.RawMessage `json:"summary_diffs"`
	}
	if err := json.Unmarshal(payload, &env); err != nil {
		t.Fatalf("parse session: %v", err)
	}
	if len(env.SummaryDiffs) == 0 || string(env.SummaryDiffs) == "null" || string(env.SummaryDiffs) == `""` {
		return
	}
	if mode == config.PrivacyContentLocal {
		if !bytes.Equal(stripped, payload) {
			t.Errorf("content-local session not byte-identical")
		}
		if len(refs) != 1 {
			t.Errorf("session: %d refs, want 1", len(refs))
		}
	} else {
		if bytes.Equal(stripped, payload) {
			t.Errorf("metadata-only session should be stripped")
		}
		var s struct {
			SummaryDiffs json.RawMessage `json:"summary_diffs"`
		}
		if err := json.Unmarshal(stripped, &s); err != nil {
			t.Fatalf("parse stripped session: %v", err)
		}
		assertRefAt(t, s.SummaryDiffs, sourcePath, "session")
	}
	for _, ref := range refs {
		assertRefIntegrity(t, ref, sourcePath, "session", env.SummaryDiffs)
	}
}

func assertRefAt(t *testing.T, wrapped json.RawMessage, sourcePath, table string) {
	t.Helper()
	var holder map[string]domain.ContentRef
	if err := json.Unmarshal(wrapped, &holder); err != nil {
		t.Fatalf("stripped value should be a ref object, got %s: %v", wrapped, err)
	}
	ref, ok := holder[contentKey]
	if !ok {
		t.Fatalf("stripped value missing %q key: %s", contentKey, wrapped)
	}
	if ref.Path != sourcePath {
		t.Errorf("embedded ref.Path = %q, want %q", ref.Path, sourcePath)
	}
	if ref.Table != table {
		t.Errorf("embedded ref.Table = %q, want %q", ref.Table, table)
	}
	if err := ref.Validate(); err != nil {
		t.Errorf("embedded ref invalid: %v", err)
	}
}

func partEnvelope(id, data string) []byte {
	env := map[string]any{
		"id":           id,
		"message_id":   "msg",
		"session_id":   "ses",
		"time_created": 0,
		"time_updated": 0,
		"data":         json.RawMessage(data),
	}
	b, err := json.Marshal(env)
	if err != nil {
		panic(err)
	}
	return b
}

func messageEnvelope(id, data string) []byte {
	env := map[string]any{
		"id":           id,
		"session_id":   "ses",
		"time_created": 0,
		"time_updated": 0,
		"data":         json.RawMessage(data),
	}
	b, err := json.Marshal(env)
	if err != nil {
		panic(err)
	}
	return b
}

func todoEnvelope(sessionID string, position int64, content, status, priority string, timeCreated, timeUpdated int64) []byte {
	env := map[string]any{
		"session_id":   sessionID,
		"position":     position,
		"content":      content,
		"status":       status,
		"priority":     priority,
		"time_created": timeCreated,
		"time_updated": timeUpdated,
	}
	b, err := json.Marshal(env)
	if err != nil {
		panic(err)
	}
	return b
}

func sessionEnvelope(id, title string, summaryDiffs sql.NullString) []byte {
	env := map[string]any{
		"id":            id,
		"title":         title,
		"slug":          "slug",
		"directory":     "/dir",
		"summary_diffs": nullOrString(summaryDiffs),
	}
	b, err := json.Marshal(env)
	if err != nil {
		panic(err)
	}
	return b
}

func nullOrString(ns sql.NullString) any {
	if !ns.Valid {
		return nil
	}
	return ns.String
}

func TestStripContentFailClosedUnknownRecordType(t *testing.T) {
	_, _, err := StripContent(config.PrivacyMetadataOnly, "/s", "unknown", []byte(`{}`))
	if err == nil {
		t.Fatal("unknown record type should fail closed")
	}
}

func TestStripContentFailClosedUnknownPartType(t *testing.T) {
	payload := partEnvelope("prt_1", `{"type":"future-type","text":"secret"}`)
	_, _, err := StripContent(config.PrivacyMetadataOnly, "/s", "part", payload)
	if err == nil {
		t.Fatal("unknown part type should fail closed")
	}
}

func TestStripContentFailClosedMissingType(t *testing.T) {
	payload := partEnvelope("prt_1", `{"text":"secret"}`)
	_, _, err := StripContent(config.PrivacyMetadataOnly, "/s", "part", payload)
	if err == nil {
		t.Fatal("missing part type should fail closed")
	}
}

func TestStripContentRedactedExportRejected(t *testing.T) {
	_, _, err := StripContent(config.PrivacyRedactedExport, "/s", "part", partEnvelope("p", `{"type":"text","text":"x"}`))
	if err == nil {
		t.Fatal("redacted-export should be rejected")
	}
}

func TestStripContentUnknownModeRejected(t *testing.T) {
	_, _, err := StripContent(config.PrivacyMode(200), "/s", "part", partEnvelope("p", `{"type":"text","text":"x"}`))
	if err == nil {
		t.Fatal("unknown mode should be rejected")
	}
}

func TestStripContentInvalidJSONPayload(t *testing.T) {
	_, _, err := StripContent(config.PrivacyMetadataOnly, "/s", "part", []byte(`{not json`))
	if err == nil {
		t.Fatal("invalid json payload should error")
	}
}

func TestStripContentEmptyTextProducesNoRef(t *testing.T) {
	payload := partEnvelope("prt_1", `{"type":"text","text":""}`)
	stripped, refs, err := StripContent(config.PrivacyMetadataOnly, "/s", "part", payload)
	if err != nil {
		t.Fatalf("empty text: %v", err)
	}
	if len(refs) != 0 {
		t.Errorf("empty text produced %d refs, want 0", len(refs))
	}
	if !bytes.Equal(stripped, payload) {
		t.Errorf("empty text payload should be unchanged")
	}
}

func TestStripContentToolStateOrderPreserved(t *testing.T) {
	payload := partEnvelope("prt_1", `{"type":"tool","tool":"edit","callID":"c","state":{"status":"completed","input":{"x":1},"output":"hi","time":{"start":1,"end":2}}}`)
	stripped, refs, err := StripContent(config.PrivacyMetadataOnly, "/s", "part", payload)
	if err != nil {
		t.Fatalf("tool: %v", err)
	}
	if len(refs) != 2 {
		t.Fatalf("tool with input+output: %d refs, want 2", len(refs))
	}
	var sd struct {
		State struct {
			Status json.RawMessage `json:"status"`
			Input  json.RawMessage `json:"input"`
			Output json.RawMessage `json:"output"`
			Time   json.RawMessage `json:"time"`
		} `json:"state"`
	}
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(stripped, &env); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := json.Unmarshal(env.Data, &sd); err != nil {
		t.Fatalf("parse data: %v", err)
	}
	if string(sd.State.Status) != `"completed"` {
		t.Errorf("status stripped, got %s", sd.State.Status)
	}
	if string(sd.State.Time) != `{"start":1,"end":2}` {
		t.Errorf("time stripped, got %s", sd.State.Time)
	}
	if len(sd.State.Input) == 0 || len(sd.State.Output) == 0 {
		t.Errorf("input/output should be wrapped refs, got %s %s", sd.State.Input, sd.State.Output)
	}
}

func TestStripContentRefsAreValidDomainValues(t *testing.T) {
	payload := partEnvelope("prt_1", `{"type":"text","text":"hello secret"}`)
	_, refs, err := StripContent(config.PrivacyContentLocal, "/s", "part", payload)
	if err != nil {
		t.Fatalf("strip: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("refs = %d", len(refs))
	}
	if refs[0].RowID != "prt_1" {
		t.Errorf("RowID = %q, want prt_1", refs[0].RowID)
	}
	if refs[0].Kind != domain.ContentKindSQLiteRow {
		t.Errorf("Kind = %v", refs[0].Kind)
	}
}

func TestStripSessionWithSummaryDiffs(t *testing.T) {
	payload := sessionEnvelopeRaw("ses_1", "Title", "Index: file.md\n---\n+++ secret diff\n")
	stripped, refs, err := StripContent(config.PrivacyMetadataOnly, "/s", "session", payload)
	if err != nil {
		t.Fatalf("strip session: %v", err)
	}
	if bytes.Equal(stripped, payload) {
		t.Fatal("summary_diffs should be stripped")
	}
	var s struct {
		SummaryDiffs json.RawMessage `json:"summary_diffs"`
		Title        json.RawMessage `json:"title"`
	}
	if err := json.Unmarshal(stripped, &s); err != nil {
		t.Fatalf("parse: %v", err)
	}
	assertRefAt(t, s.SummaryDiffs, "/s", "session")
	if string(s.Title) != `"Title"` {
		t.Errorf("title should be preserved, got %s", s.Title)
	}
	if len(refs) != 1 {
		t.Errorf("refs = %d, want 1", len(refs))
	}
	var orig struct {
		SummaryDiffs json.RawMessage `json:"summary_diffs"`
	}
	if err := json.Unmarshal(payload, &orig); err != nil {
		t.Fatalf("parse orig: %v", err)
	}
	assertRefIntegrity(t, refs[0], "/s", "session", orig.SummaryDiffs)
}

func TestStripSessionWithSummaryDiffsContentLocal(t *testing.T) {
	payload := sessionEnvelopeRaw("ses_1", "Title", "diff text")
	stripped, refs, err := StripContent(config.PrivacyContentLocal, "/s", "session", payload)
	if err != nil {
		t.Fatalf("strip session: %v", err)
	}
	if !bytes.Equal(stripped, payload) {
		t.Error("content-local session should be byte-identical")
	}
	if len(refs) != 1 {
		t.Errorf("refs = %d, want 1", len(refs))
	}
}

func sessionEnvelopeRaw(id, title, summaryDiffs string) []byte {
	env := map[string]any{
		"id":            id,
		"title":         title,
		"slug":          "slug",
		"directory":     "/dir",
		"summary_diffs": summaryDiffs,
	}
	b, err := json.Marshal(env)
	if err != nil {
		panic(err)
	}
	return b
}

func messageEnvelopeRaw(id, data string) []byte {
	env := map[string]any{
		"id":           id,
		"session_id":   "ses",
		"time_created": 0,
		"time_updated": 0,
		"data":         json.RawMessage(data),
	}
	b, err := json.Marshal(env)
	if err != nil {
		panic(err)
	}
	return b
}

func TestStripMessageFailClosedUnknownDiffEntryKey(t *testing.T) {
	payload := messageEnvelopeRaw("msg_1", `{"role":"user","summary":{"diffs":[{"file":"a","patch":"x","extra":"leak"}]}}`)
	for _, mode := range []config.PrivacyMode{config.PrivacyMetadataOnly, config.PrivacyContentLocal} {
		if _, _, err := StripContent(mode, "/s", "message", payload); err == nil {
			t.Errorf("%s: unknown diff-entry key should fail closed", mode)
		}
	}
}

func TestStripMessageFailClosedUnknownSummaryKey(t *testing.T) {
	payload := messageEnvelopeRaw("msg_1", `{"role":"user","summary":{"diffs":[],"extra":"leak"}}`)
	for _, mode := range []config.PrivacyMode{config.PrivacyMetadataOnly, config.PrivacyContentLocal} {
		if _, _, err := StripContent(mode, "/s", "message", payload); err == nil {
			t.Errorf("%s: unknown summary key should fail closed", mode)
		}
	}
}

func TestStripMessageFailClosedUnknownErrorKey(t *testing.T) {
	payload := messageEnvelopeRaw("msg_1", `{"role":"assistant","error":{"name":"E","data":{"message":"x"},"extra":"leak"}}`)
	for _, mode := range []config.PrivacyMode{config.PrivacyMetadataOnly, config.PrivacyContentLocal} {
		if _, _, err := StripContent(mode, "/s", "message", payload); err == nil {
			t.Errorf("%s: unknown error key should fail closed", mode)
		}
	}
}

func TestStripMessageFailClosedUnknownErrorDataKey(t *testing.T) {
	payload := messageEnvelopeRaw("msg_1", `{"role":"assistant","error":{"name":"E","data":{"message":"x","extra":"leak"}}}`)
	for _, mode := range []config.PrivacyMode{config.PrivacyMetadataOnly, config.PrivacyContentLocal} {
		if _, _, err := StripContent(mode, "/s", "message", payload); err == nil {
			t.Errorf("%s: unknown error.data key should fail closed", mode)
		}
	}
}

func TestStripMessageFailClosedSummaryStringForm(t *testing.T) {
	payload := messageEnvelopeRaw("msg_1", `{"role":"user","summary":"some diff text"}`)
	for _, mode := range []config.PrivacyMode{config.PrivacyMetadataOnly, config.PrivacyContentLocal} {
		if _, _, err := StripContent(mode, "/s", "message", payload); err == nil {
			t.Errorf("%s: summary as string should fail closed", mode)
		}
	}
}

func TestStripMessageFailClosedErrorStringForm(t *testing.T) {
	payload := messageEnvelopeRaw("msg_1", `{"role":"assistant","error":"oops"}`)
	for _, mode := range []config.PrivacyMode{config.PrivacyMetadataOnly, config.PrivacyContentLocal} {
		if _, _, err := StripContent(mode, "/s", "message", payload); err == nil {
			t.Errorf("%s: error as string should fail closed", mode)
		}
	}
}

func TestStripMessageFailClosedErrorDataStringForm(t *testing.T) {
	payload := messageEnvelopeRaw("msg_1", `{"role":"assistant","error":{"name":"E","data":"oops"}}`)
	for _, mode := range []config.PrivacyMode{config.PrivacyMetadataOnly, config.PrivacyContentLocal} {
		if _, _, err := StripContent(mode, "/s", "message", payload); err == nil {
			t.Errorf("%s: error.data as string should fail closed", mode)
		}
	}
}

func TestStripMessageSummaryTruePasses(t *testing.T) {
	payload := messageEnvelopeRaw("msg_1", `{"role":"assistant","summary":true}`)
	for _, mode := range []config.PrivacyMode{config.PrivacyMetadataOnly, config.PrivacyContentLocal} {
		stripped, refs, err := StripContent(mode, "/s", "message", payload)
		if err != nil {
			t.Fatalf("%s: summary:true should pass: %v", mode, err)
		}
		if len(refs) != 0 {
			t.Errorf("%s: summary:true produced %d refs, want 0", mode, len(refs))
		}
		if !bytes.Equal(stripped, payload) {
			t.Errorf("%s: summary:true should be byte-untouched", mode)
		}
	}
}

func TestStripMessageErrorStripsMessageKeepsName(t *testing.T) {
	payload := messageEnvelopeRaw("msg_1", `{"role":"assistant","error":{"name":"MessageAbortedError","data":{"message":"Aborted"}}}`)
	stripped, refs, err := StripContent(config.PrivacyMetadataOnly, "/s", "message", payload)
	if err != nil {
		t.Fatalf("strip: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("refs = %d, want 1", len(refs))
	}
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(stripped, &env); err != nil {
		t.Fatalf("parse: %v", err)
	}
	var data struct {
		Error struct {
			Name json.RawMessage `json:"name"`
			Data struct {
				Message json.RawMessage `json:"message"`
			} `json:"data"`
		} `json:"error"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("parse data: %v", err)
	}
	if string(data.Error.Name) != `"MessageAbortedError"` {
		t.Errorf("error.name altered: %s", data.Error.Name)
	}
	assertRefAt(t, data.Error.Data.Message, "/s", "message")
	assertRefIntegrity(t, refs[0], "/s", "message", json.RawMessage(`"Aborted"`))
}

func TestStripMessageStringDiffsWholeValueRef(t *testing.T) {
	payload := messageEnvelopeRaw("msg_1", `{"role":"user","summary":{"diffs":"Index: a\n---\n+++ secret\n"}}`)
	stripped, refs, err := StripContent(config.PrivacyMetadataOnly, "/s", "message", payload)
	if err != nil {
		t.Fatalf("strip: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("refs = %d, want 1", len(refs))
	}
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(stripped, &env); err != nil {
		t.Fatalf("parse: %v", err)
	}
	var data struct {
		Summary struct {
			Diffs json.RawMessage `json:"diffs"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("parse data: %v", err)
	}
	assertRefAt(t, data.Summary.Diffs, "/s", "message")
	assertRefIntegrity(t, refs[0], "/s", "message", json.RawMessage(`"Index: a\n---\n+++ secret\n"`))
}

func TestStripMessageDiffEntryPatchWrapped(t *testing.T) {
	payload := messageEnvelopeRaw("msg_1", `{"role":"user","summary":{"diffs":[{"file":"a.go","patch":"@@ -1 +1 @@\n-old\n+new\n","additions":1,"deletions":1,"status":"modified"}]}}`)
	stripped, refs, err := StripContent(config.PrivacyMetadataOnly, "/s", "message", payload)
	if err != nil {
		t.Fatalf("strip: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("refs = %d, want 1", len(refs))
	}
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(stripped, &env); err != nil {
		t.Fatalf("parse: %v", err)
	}
	var data struct {
		Summary struct {
			Diffs []struct {
				File      json.RawMessage `json:"file"`
				Patch     json.RawMessage `json:"patch"`
				Additions json.RawMessage `json:"additions"`
				Deletions json.RawMessage `json:"deletions"`
				Status    json.RawMessage `json:"status"`
			} `json:"diffs"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("parse data: %v", err)
	}
	entry := data.Summary.Diffs[0]
	if string(entry.File) != `"a.go"` {
		t.Errorf("file altered: %s", entry.File)
	}
	if string(entry.Additions) != `1` {
		t.Errorf("additions altered: %s", entry.Additions)
	}
	if string(entry.Deletions) != `1` {
		t.Errorf("deletions altered: %s", entry.Deletions)
	}
	if string(entry.Status) != `"modified"` {
		t.Errorf("status altered: %s", entry.Status)
	}
	assertRefAt(t, entry.Patch, "/s", "message")
	assertRefIntegrity(t, refs[0], "/s", "message", json.RawMessage(`"@@ -1 +1 @@\n-old\n+new\n"`))
}
