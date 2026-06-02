package skillpacks

import (
	"AgentTest/config"
	"os"
	"path/filepath"
	"testing"
)

func TestReload_addsPack(t *testing.T) {
	root := t.TempDir()
	packsRoot := filepath.Join(root, "skill_packs")
	if err := os.MkdirAll(packsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	packDir := filepath.Join(packsRoot, "demo-pack")
	if err := os.MkdirAll(packDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skill := []byte("---\nname: Demo\ndescription: test pack\n---\n\n# Demo\n\nbody\n")
	if err := os.WriteFile(filepath.Join(packDir, "SKILL.md"), skill, 0o644); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(root, "app.yaml")
	cfgYAML := "root: " + filepath.ToSlash(root) + "\ncapabilities:\n  skill_packs:\n    enabled: true\n    roots:\n      - skill_packs\n"
	if err := os.WriteFile(cfgPath, []byte(cfgYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	loaded = nil
	mu.Unlock()

	rep, err := Reload(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Total != 1 {
		t.Fatalf("total=%d want 1", rep.Total)
	}
	if len(rep.Added) != 1 || rep.Added[0] != "demo-pack" {
		t.Fatalf("added=%v", rep.Added)
	}
	p, ok := packByID("demo-pack")
	if !ok || p.Title != "Demo" {
		t.Fatalf("pack=%+v ok=%v", p, ok)
	}
}
