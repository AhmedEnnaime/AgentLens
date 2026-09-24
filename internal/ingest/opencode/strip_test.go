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

func assertMessageRows(t *testing.T, db *sql.DB, mode config.PrivacyMode, sourcePath, fixture string) {
	t.Helper()
	rows, err := db.Query(`SELECT id, data FROM message`)
	if err != nil {
		t.Fatalf("query message: %v", err)
	}
	defer rows.Close()
	count := 0
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
		if !bytes.Equal(stripped, payload) {
			t.Errorf("message %s payload altered under %s", id, mode)
		}
		if len(refs) != 0 {
			t.Errorf("message %s produced %d refs, want 0", id, len(refs))
		}
		count++
	}
	t.Logf("fixture %s: %d message rows", fixture, count)
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
