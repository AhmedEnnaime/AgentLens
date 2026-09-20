package domain

import "errors"

type ModelIdentity struct {
	ProviderID string        `json:"provider_id"`
	ModelID    string        `json:"model_id"`
	Variant    Value[string] `json:"variant"`
}

func (m *ModelIdentity) Validate() error {
	if m == nil {
		return nil
	}
	return m.Variant.Validate()
}

type Cost struct {
	Amount   Value[float64] `json:"amount"`
	Currency string         `json:"currency"`
}

func (c *Cost) Validate() error {
	if c == nil {
		return nil
	}
	return c.Amount.Validate()
}

type Capabilities struct {
	PerCallUsage            bool `json:"per_call_usage"`
	CacheTokenUsage         bool `json:"cache_token_usage"`
	ReasoningTokenUsage     bool `json:"reasoning_token_usage"`
	ToolCallDuration        bool `json:"tool_call_duration"`
	FileEditEvents          bool `json:"file_edit_events"`
	CompactionEvents        bool `json:"compaction_events"`
	TaskMarkers             bool `json:"task_markers"`
	SubagentSessions        bool `json:"subagent_sessions"`
	SessionUsageRollups     bool `json:"session_usage_rollups"`
	AgentIdentityPerMessage bool `json:"agent_identity_per_message"`
	ProviderCost            bool `json:"provider_cost"`
	ModelVariant            bool `json:"model_variant"`
}

func (c Capabilities) Validate() error {
	var errs []error
	if (c.CacheTokenUsage || c.ReasoningTokenUsage) && !c.PerCallUsage {
		errs = append(errs, errors.New("capabilities: cache/reasoning token usage requires per-call usage"))
	}
	return errors.Join(errs...)
}
