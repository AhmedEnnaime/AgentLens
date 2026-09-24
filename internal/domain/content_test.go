package domain

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func testRef() ContentRef {
	return ContentRef{
		Kind:   ContentKindSQLiteRow,
		Path:   "/source/opencode.db",
		Table:  "part",
		RowID:  "prt_1",
		SHA256: strings.Repeat("a", 64),
		Bytes:  3,
	}
}

func TestContentRefValidateValid(t *testing.T) {
	if err := testRef().Validate(); err != nil {
		t.Errorf("valid ref should validate, got %v", err)
	}
}

func TestContentRefValidateMatrix(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*ContentRef)
		want   string
	}{
		{"unknown kind", func(r *ContentRef) { r.Kind = ContentKind(200) }, "kind"},
		{"zero kind", func(r *ContentRef) { r.Kind = 0 }, "kind"},
		{"empty path", func(r *ContentRef) { r.Path = "" }, "path"},
		{"empty table", func(r *ContentRef) { r.Table = "" }, "table"},
		{"empty row id", func(r *ContentRef) { r.RowID = "" }, "row id"},
		{"empty sha", func(r *ContentRef) { r.SHA256 = "" }, "sha256"},
		{"short sha", func(r *ContentRef) { r.SHA256 = "abc" }, "sha256"},
		{"uppercase sha", func(r *ContentRef) { r.SHA256 = strings.Repeat("A", 64) }, "sha256"},
		{"non-hex sha", func(r *ContentRef) { r.SHA256 = strings.Repeat("g", 64) }, "sha256"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ref := testRef()
			c.mutate(&ref)
			err := ref.Validate()
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %q should contain %q", err, c.want)
			}
		})
	}
}

func TestContentRefJSONRoundTrip(t *testing.T) {
	ref := testRef()
	data, err := json.Marshal(ref)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back ContentRef
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if err := back.Validate(); err != nil {
		t.Fatalf("round-tripped ref should validate: %v", err)
	}
	again, err := json.Marshal(back)
	if err != nil {
		t.Fatalf("second marshal: %v", err)
	}
	if string(data) != string(again) {
		t.Errorf("round-trip not byte-identical:\n%s\n%s", data, again)
	}
}

func TestContentRefJSONUnknownKindErrors(t *testing.T) {
	var ref ContentRef
	if err := json.Unmarshal([]byte(`{"kind":"bogus","path":"p","table":"t","row_id":"r","sha256":"`+strings.Repeat("a", 64)+`","bytes":1}`), &ref); err == nil {
		t.Error("unknown kind should error on unmarshal")
	}
}

func TestContentKindMarshalUnknownErrors(t *testing.T) {
	if _, err := ContentKind(200).MarshalJSON(); err == nil {
		t.Error("marshal of unknown kind should error")
	}
}

func TestAsContentRefDecode(t *testing.T) {
	m := map[string]any{
		"kind":   "sqlite-row",
		"path":   "/source/opencode.db",
		"table":  "part",
		"row_id": "prt_1",
		"sha256": strings.Repeat("b", 64),
		"bytes":  float64(5),
	}
	ref, err := AsContentRef(m)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if ref.Kind != ContentKindSQLiteRow || ref.Table != "part" || ref.RowID != "prt_1" || ref.Bytes != 5 {
		t.Errorf("decoded ref mismatch: %+v", ref)
	}
}

func TestAsContentRefMalformed(t *testing.T) {
	cases := []struct {
		name string
		in   any
	}{
		{"missing fields", map[string]any{"kind": "sqlite-row"}},
		{"unknown kind", map[string]any{"kind": "nope", "path": "p", "table": "t", "row_id": "r", "sha256": strings.Repeat("a", 64)}},
		{"bad sha", map[string]any{"kind": "sqlite-row", "path": "p", "table": "t", "row_id": "r", "sha256": "zzz"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := AsContentRef(c.in); err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestContentStatusString(t *testing.T) {
	cases := []struct {
		s    ContentStatus
		want string
	}{
		{ContentResolved, "resolved"},
		{ContentUnavailable, "unavailable"},
		{ContentChanged, "changed"},
	}
	for _, c := range cases {
		if got := c.s.String(); got != c.want {
			t.Errorf("ContentStatus(%d).String() = %q, want %q", uint8(c.s), got, c.want)
		}
	}
}

func TestContentStatusJSONRoundTrip(t *testing.T) {
	for _, s := range []ContentStatus{ContentResolved, ContentUnavailable, ContentChanged} {
		data, err := json.Marshal(s)
		if err != nil {
			t.Fatalf("marshal %s: %v", s, err)
		}
		var back ContentStatus
		if err := json.Unmarshal(data, &back); err != nil {
			t.Fatalf("unmarshal %s: %v", s, err)
		}
		if back != s {
			t.Errorf("round-trip %s = %s", s, back)
		}
	}
}

func TestContentStatusUnknownJSONErrors(t *testing.T) {
	var s ContentStatus
	if err := json.Unmarshal([]byte(`"bogus"`), &s); err == nil {
		t.Error("unknown status should error")
	}
	if _, err := ContentStatus(200).MarshalJSON(); err == nil {
		t.Error("marshal unknown status should error")
	}
}

type stubResolver struct {
	resolved    ResolvedContent
	unavailable ResolvedContent
	changed     ResolvedContent
	failErr     error
}

func (s stubResolver) ResolveContent(_ context.Context, ref ContentRef) (ResolvedContent, error) {
	if err := ref.Validate(); err != nil {
		return ResolvedContent{Status: ContentUnavailable}, nil
	}
	switch ref.RowID {
	case "resolved":
		return s.resolved, nil
	case "unavailable":
		return s.unavailable, nil
	case "changed":
		return s.changed, nil
	case "fail":
		return ResolvedContent{}, s.failErr
	default:
		return ResolvedContent{Status: ContentUnavailable}, nil
	}
}

func TestRunContentResolverContract(t *testing.T) {
	resolver := stubResolver{
		resolved:    ResolvedContent{Status: ContentResolved, Text: "hello"},
		unavailable: ResolvedContent{Status: ContentUnavailable},
		changed:     ResolvedContent{Status: ContentChanged},
		failErr:     errors.New("boom"),
	}
	c := ContentResolverContract{
		Resolved:    testRefWithRow("resolved"),
		Unavailable: testRefWithRow("unavailable"),
		Changed:     testRefWithRow("changed"),
		Failing:     testRefWithRow("fail"),
	}
	if err := RunContentResolverContract(resolver, c); err != nil {
		t.Errorf("contract suite should pass against compliant stub: %v", err)
	}
}

func TestRunContentResolverContractDetectsViolations(t *testing.T) {
	resolver := stubResolver{
		resolved:    ResolvedContent{Status: ContentUnavailable},
		unavailable: ResolvedContent{Status: ContentResolved, Text: "leak"},
		changed:     ResolvedContent{Status: ContentResolved, Text: "leak"},
		failErr:     nil,
	}
	c := ContentResolverContract{
		Resolved:    testRefWithRow("resolved"),
		Unavailable: testRefWithRow("unavailable"),
		Changed:     testRefWithRow("changed"),
		Failing:     testRefWithRow("fail"),
	}
	err := RunContentResolverContract(resolver, c)
	if err == nil {
		t.Fatal("non-compliant resolver should fail the contract suite")
	}
	msg := err.Error()
	for _, want := range []string{"resolved", "unavailable", "changed", "operational"} {
		if !strings.Contains(msg, want) {
			t.Errorf("contract error %q should mention %q", msg, want)
		}
	}
}

func testRefWithRow(row string) ContentRef {
	ref := testRef()
	ref.RowID = row
	return ref
}

func TestContentSHA256Stable(t *testing.T) {
	raw := json.RawMessage(`"hello world"`)
	got := ContentSHA256(raw)
	want := ContentSHA256(json.RawMessage(`"hello world"`))
	if got != want {
		t.Errorf("hash not stable: %s vs %s", got, want)
	}
	if len(got) != 64 {
		t.Errorf("hash length = %d, want 64", len(got))
	}
}

func TestSourceMetadataPrivacyModeWireRoundTrip(t *testing.T) {
	withMode := SourceMetadata{
		RootSessionID:    "s",
		AgentVersion:     "1.18.30",
		ProjectDirectory: "/p",
		AgentLensVersion: "v0.1.0",
		PrivacyMode:      "metadata-only",
	}
	data, err := json.Marshal(withMode)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), `"privacy_mode":"metadata-only"`) {
		t.Errorf("privacy_mode missing from wire: %s", data)
	}
	var back SourceMetadata
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.PrivacyMode != "metadata-only" {
		t.Errorf("privacy_mode round-trip = %q", back.PrivacyMode)
	}

	withoutMode := SourceMetadata{
		RootSessionID:    "s",
		AgentVersion:     "1.18.30",
		ProjectDirectory: "/p",
		AgentLensVersion: "v0.1.0",
	}
	data2, err := json.Marshal(withoutMode)
	if err != nil {
		t.Fatalf("marshal empty-mode: %v", err)
	}
	if strings.Contains(string(data2), "privacy_mode") {
		t.Errorf("empty privacy_mode should be omitted: %s", data2)
	}
	var back2 SourceMetadata
	if err := json.Unmarshal(data2, &back2); err != nil {
		t.Fatalf("unmarshal empty-mode: %v", err)
	}
	if back2.PrivacyMode != "" {
		t.Errorf("empty privacy_mode should decode as empty string, got %q", back2.PrivacyMode)
	}
}
