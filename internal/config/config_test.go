package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func TestDefaultIsMetadataOnly(t *testing.T) {
	c := Default()
	if c.Version != 1 {
		t.Errorf("version = %d, want 1", c.Version)
	}
	if c.PrivacyMode != PrivacyMetadataOnly {
		t.Errorf("privacy mode = %s, want metadata-only", c.PrivacyMode)
	}
	if err := c.Validate(); err != nil {
		t.Errorf("default config should validate: %v", err)
	}
}

func TestPrivacyModeString(t *testing.T) {
	cases := []struct {
		m    PrivacyMode
		want string
	}{
		{PrivacyMetadataOnly, "metadata-only"},
		{PrivacyContentLocal, "content-local"},
		{PrivacyRedactedExport, "redacted-export"},
	}
	for _, c := range cases {
		if got := c.m.String(); got != c.want {
			t.Errorf("PrivacyMode(%d).String() = %q, want %q", uint8(c.m), got, c.want)
		}
	}
}

func TestPrivacyModeJSONRoundTrip(t *testing.T) {
	for _, m := range []PrivacyMode{PrivacyMetadataOnly, PrivacyContentLocal, PrivacyRedactedExport} {
		data, err := json.Marshal(m)
		if err != nil {
			t.Fatalf("marshal %s: %v", m, err)
		}
		var back PrivacyMode
		if err := json.Unmarshal(data, &back); err != nil {
			t.Fatalf("unmarshal %s: %v", m, err)
		}
		if back != m {
			t.Errorf("round-trip %s = %s", m, back)
		}
	}
}

func TestPrivacyModeUnknownJSONErrors(t *testing.T) {
	var m PrivacyMode
	if err := json.Unmarshal([]byte(`"bogus"`), &m); err == nil {
		t.Error("unknown mode should error")
	}
	if _, err := PrivacyMode(200).MarshalJSON(); err == nil {
		t.Error("marshal unknown mode should error")
	}
}

func TestSupportsImportMatrix(t *testing.T) {
	cases := []struct {
		m    PrivacyMode
		want bool
	}{
		{PrivacyMetadataOnly, true},
		{PrivacyContentLocal, true},
		{PrivacyRedactedExport, false},
		{PrivacyMode(200), false},
	}
	for _, c := range cases {
		if got := c.m.SupportsImport(); got != c.want {
			t.Errorf("SupportsImport(%s) = %v, want %v", c.m, got, c.want)
		}
	}
}

func TestLoadAbsentFileDefaults(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(filepath.Join(dir, "missing.json"))
	if err != nil {
		t.Fatalf("absent file should default without error: %v", err)
	}
	if cfg != Default() {
		t.Errorf("absent file = %+v, want default %+v", cfg, Default())
	}
}

func TestLoadValidContentLocal(t *testing.T) {
	path := writeTemp(t, `{"version":1,"privacy_mode":"content-local"}`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.PrivacyMode != PrivacyContentLocal {
		t.Errorf("privacy mode = %s, want content-local", cfg.PrivacyMode)
	}
}

func TestLoadFailLoudMatrix(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{"bad version", `{"version":2,"privacy_mode":"metadata-only"}`, "version"},
		{"zero version", `{"version":0,"privacy_mode":"metadata-only"}`, "version"},
		{"unknown mode", `{"version":1,"privacy_mode":"bogus"}`, "privacy mode"},
		{"unknown key", `{"version":1,"privacy_mode":"metadata-only","extra":true}`, "unknown key"},
		{"invalid json", `{not json`, "json"},
		{"missing version", `{"privacy_mode":"metadata-only"}`, "version"},
		{"missing mode", `{"version":1}`, "privacy mode"},
		{"mode wrong type", `{"version":1,"privacy_mode":42}`, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := writeTemp(t, c.content)
			_, err := Load(path)
			if err == nil {
				t.Fatal("expected error")
			}
			if c.want != "" && !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %q should contain %q", err, c.want)
			}
		})
	}
}

func TestLoadRedactedExportParsesButRejectsImport(t *testing.T) {
	path := writeTemp(t, `{"version":1,"privacy_mode":"redacted-export"}`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("redacted-export should parse: %v", err)
	}
	if cfg.PrivacyMode != PrivacyRedactedExport {
		t.Errorf("privacy mode = %s, want redacted-export", cfg.PrivacyMode)
	}
	if cfg.PrivacyMode.SupportsImport() {
		t.Error("redacted-export must not support import until #14")
	}
}

func TestLoadEnvOverride(t *testing.T) {
	path := writeTemp(t, `{"version":1,"privacy_mode":"content-local"}`)
	t.Setenv("AGENTLENS_CONFIG", path)
	got, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	if got != path {
		t.Errorf("DefaultPath = %q, want %q", got, path)
	}
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load empty path should honor env override: %v", err)
	}
	if cfg.PrivacyMode != PrivacyContentLocal {
		t.Errorf("privacy mode = %s, want content-local", cfg.PrivacyMode)
	}
}
