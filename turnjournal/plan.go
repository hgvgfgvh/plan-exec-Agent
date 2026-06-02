package turnjournal

import (
	"AgentTest/plan/todolist"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var reTodoRecord = regexp.MustCompile(`记录:\s*([^\s）)\n]+)`)

func buildPlanSection(reply string, startedAt time.Time) *PlanSection {
	path := parseTodoListPath(reply)
	if path == "" {
		path = findRecentTodoList(startedAt)
	}
	if path == "" {
		return nil
	}
	doc, err := loadTodoDocument(path)
	if err != nil || doc == nil {
		return &PlanSection{DocumentPath: path}
	}
	sec := &PlanSection{
		DocumentID:    doc.ID,
		DocumentPath:  path,
		PlanStatus:    string(doc.Status),
		Summary:       truncateRunes(doc.Summary, maxFieldRunes),
		ExecutionMode: doc.ExecutionMode,
	}
	for _, s := range doc.Steps {
		detail := s.ResultDetail
		sec.Steps = append(sec.Steps, StepRecord{
			ID:            s.ID,
			Title:         s.Title,
			Status:        string(s.Status),
			ResultSummary: truncateRunes(s.ResultSummary, maxFieldRunes),
			ResultDetail:  truncateRunes(detail, maxFieldRunes),
			ResultExcerpt: excerpt(todolist.StepUserFacingText(s), excerptRunes),
			Artifacts:     append([]string(nil), s.Artifacts...),
			ToolsCalled:   append([]string(nil), s.ToolsCalled...),
		})
	}
	return sec
}

func parseTodoListPath(reply string) string {
	m := reTodoRecord.FindStringSubmatch(reply)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

func loadTodoDocument(absPath string) (*todolist.Document, error) {
	b, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}
	var doc todolist.Document
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

func findRecentTodoList(since time.Time) string {
	dir, err := todolist.Dir()
	if err != nil {
		return ""
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var bestPath string
	var bestMod time.Time
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(strings.ToLower(ent.Name()), ".json") {
			continue
		}
		info, err := ent.Info()
		if err != nil || info.ModTime().Before(since.Add(-2*time.Second)) {
			continue
		}
		if info.ModTime().After(bestMod) {
			bestMod = info.ModTime()
			bestPath = filepath.Join(dir, ent.Name())
		}
	}
	return bestPath
}

func indexArtifacts(steps []StepRecord) []ArtifactRef {
	seen := make(map[string]bool)
	var out []ArtifactRef
	idx := 0
	for _, st := range steps {
		for _, p := range st.Artifacts {
			p = strings.TrimSpace(p)
			if p == "" || seen[p] {
				continue
			}
			seen[p] = true
			idx++
			out = append(out, ArtifactRef{
				ID:    "art-" + itoa(idx),
				Path:  p,
				Type:  artifactType(p),
				Label: filepath.Base(p),
			})
		}
	}
	return out
}

func artifactType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp":
		return "image"
	case ".md", ".txt", ".json", ".yaml", ".yml", ".docx", ".pptx", ".xlsx", ".csv", ".html":
		return "file"
	default:
		if strings.HasSuffix(path, "/") {
			return "dir"
		}
	}
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return "dir"
	}
	return "other"
}
