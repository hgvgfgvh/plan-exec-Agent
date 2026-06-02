package runview

import (
	"AgentTest/turnjournal"
	"encoding/json"
	"os"
)

// Manifest 供 HTML 内链与 /api/run-view/file 校验。
type Manifest struct {
	TurnID    string                    `json:"turn_id"`
	Artifacts []turnjournal.ArtifactRef `json:"artifacts"`
}

func writeManifest(path string, m *Manifest) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func readManifest(path string) (*Manifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return &m, nil
}
