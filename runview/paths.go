package runview

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func htmlPath(outputDir, turnID string) string {
	safe := safeTurnID(turnID)
	return filepath.Join(outputDir, safe+".html")
}

func manifestPath(outputDir, turnID string) string {
	safe := safeTurnID(turnID)
	return filepath.Join(outputDir, safe+".manifest.json")
}

func safeTurnID(turnID string) string {
	return strings.ReplaceAll(strings.TrimSpace(turnID), string(os.PathSeparator), "_")
}

func ensureOutputDir(dir string) error {
	return os.MkdirAll(dir, 0o755)
}

func turnIDFromLogFilename(name string) (string, bool) {
	if !strings.HasSuffix(strings.ToLower(name), ".json") {
		return "", false
	}
	base := strings.TrimSuffix(name, filepath.Ext(name))
	if base == "" {
		return "", false
	}
	return base, true
}

func validateTurnID(turnID string) error {
	if turnID == "" {
		return fmt.Errorf("empty turn_id")
	}
	if strings.ContainsAny(turnID, `/\..`) {
		return fmt.Errorf("invalid turn_id")
	}
	return nil
}
