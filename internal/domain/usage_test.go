package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestUsageValidate(t *testing.T) {
	valid := &Usage{
		InputTokens:      Observed(int64(10), "r"),
		OutputTokens:     Observed(int64(5), "r"),
		ReasoningTokens:  Unavailable[int64](),
		CacheReadTokens:  Unavailable[int64](),
		CacheWriteTokens: Unavailable[int64](),
	}
	if err := valid.Validate(); err != nil {
		t.Errorf("valid usage: %v", err)
	}
}

func TestUsageValidateBadField(t *testing.T) {
	bad := &Usage{
		InputTokens: Observed(int64(10), ""),
	}
	err := bad.Validate()
	if err == nil {
		t.Fatal("usage with observed empty ref should error")
	}
	if !strings.Contains(err.Error(), "usage.input_tokens") {
		t.Errorf("error %q should mention the field", err)
	}
}

func TestUsageNilValidate(t *testing.T) {
	var u *Usage
	if err := u.Validate(); err != nil {
		t.Errorf("nil usage should be valid: %v", err)
	}
}

func TestModelIdentityValidate(t *testing.T) {
	m := &ModelIdentity{
		ProviderID: "opencode",
		ModelID:    "glm-5.3",
		Variant:    Observed("max", "r"),
	}
	if err := m.Validate(); err != nil {
		t.Errorf("valid model identity: %v", err)
	}
	bad := &ModelIdentity{Variant: Observed("max", "")}
	if err := bad.Validate(); err == nil {
		t.Error("model identity with observed empty-ref variant should error")
	}
	var nilM *ModelIdentity
	if err := nilM.Validate(); err != nil {
		t.Errorf("nil model identity should be valid: %v", err)
	}
}

func TestCostValidate(t *testing.T) {
	c := &Cost{Amount: Unavailable[float64](), Currency: "USD"}
	if err := c.Validate(); err != nil {
		t.Errorf("valid cost: %v", err)
	}
	bad := &Cost{Amount: Estimated(1.0, "")}
	if err := bad.Validate(); err == nil {
		t.Error("cost with estimated empty-ref amount should error")
	}
	var nilC *Cost
	if err := nilC.Validate(); err != nil {
		t.Errorf("nil cost should be valid: %v", err)
	}
}

func TestCapabilitiesValidate(t *testing.T) {
	valid := Capabilities{PerCallUsage: true, CacheTokenUsage: true, ReasoningTokenUsage: true}
	if err := valid.Validate(); err != nil {
		t.Errorf("capabilities with per-call usage should be valid: %v", err)
	}
	noPerCall := Capabilities{CacheTokenUsage: true}
	if err := noPerCall.Validate(); err == nil {
		t.Error("cache token usage without per-call usage should error")
	}
	reasonOnly := Capabilities{ReasoningTokenUsage: true}
	if err := reasonOnly.Validate(); err == nil {
		t.Error("reasoning token usage without per-call usage should error")
	}
	empty := Capabilities{}
	if err := empty.Validate(); err != nil {
		t.Errorf("empty capabilities should be valid: %v", err)
	}
}

func TestModelIdentityJSON(t *testing.T) {
	m := ModelIdentity{ProviderID: "opencode", ModelID: "glm-5.3", Variant: Unavailable[string]()}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m2 map[string]any
	if err := json.Unmarshal(data, &m2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m2["provider_id"] != "opencode" || m2["model_id"] != "glm-5.3" {
		t.Errorf("model identity json = %s", data)
	}
}
