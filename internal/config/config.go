package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type PrivacyMode uint8

const (
	PrivacyMetadataOnly PrivacyMode = iota + 1
	PrivacyContentLocal
	PrivacyRedactedExport
)

func (m PrivacyMode) String() string {
	switch m {
	case PrivacyMetadataOnly:
		return "metadata-only"
	case PrivacyContentLocal:
		return "content-local"
	case PrivacyRedactedExport:
		return "redacted-export"
	default:
		return fmt.Sprintf("privacy_mode(%d)", uint8(m))
	}
}

func (m PrivacyMode) known() bool {
	switch m {
	case PrivacyMetadataOnly, PrivacyContentLocal, PrivacyRedactedExport:
		return true
	}
	return false
}

func (m PrivacyMode) SupportsImport() bool {
	return m == PrivacyMetadataOnly || m == PrivacyContentLocal
}

func (m PrivacyMode) MarshalJSON() ([]byte, error) {
	if !m.known() {
		return nil, fmt.Errorf("cannot marshal unknown privacy mode %d", uint8(m))
	}
	return json.Marshal(m.String())
}

func (m *PrivacyMode) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case "metadata-only":
		*m = PrivacyMetadataOnly
	case "content-local":
		*m = PrivacyContentLocal
	case "redacted-export":
		*m = PrivacyRedactedExport
	default:
		return fmt.Errorf("unknown privacy mode %q", s)
	}
	return nil
}

type Config struct {
	Version     int         `json:"version"`
	PrivacyMode PrivacyMode `json:"privacy_mode"`
}

func Default() Config {
	return Config{Version: 1, PrivacyMode: PrivacyMetadataOnly}
}

func (c Config) Validate() error {
	if c.Version != 1 {
		return fmt.Errorf("config: unsupported version %d (want 1)", c.Version)
	}
	if !c.PrivacyMode.known() {
		return fmt.Errorf("config: unknown privacy mode %d", uint8(c.PrivacyMode))
	}
	return nil
}

func DefaultPath() (string, error) {
	if p := os.Getenv("AGENTLENS_CONFIG"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".agentlens", "config.json"), nil
}

func Load(path string) (Config, error) {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return Config{}, err
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return Config{}, fmt.Errorf("config: read %s: %w", path, err)
	}
	cfg, err := parse(data)
	if err != nil {
		return Config{}, fmt.Errorf("config: %s: %w", path, err)
	}
	return cfg, nil
}

func parse(data []byte) (Config, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return Config{}, fmt.Errorf("invalid json: %w", err)
	}
	for key := range raw {
		switch key {
		case "version", "privacy_mode":
		default:
			return Config{}, fmt.Errorf("unknown key %q", key)
		}
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
