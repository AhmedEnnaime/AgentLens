package domain

import "time"

type SourceMetadata struct {
	RootSessionID    string    `json:"root_session_id"`
	AgentVersion     string    `json:"agent_version"`
	ProjectDirectory string    `json:"project_directory"`
	ImportedAt       time.Time `json:"imported_at"`
	AgentLensVersion string    `json:"agentlens_version"`
}
