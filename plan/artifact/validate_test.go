package artifact

import (
	"os"
	"path/filepath"
	"testing"

	"AgentTest/agent/runcontrol"
)

func TestValidateReportArtifacts_ShortNonEmptyContentOK(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "e2e_stdout.txt")
	if err := os.WriteFile(path, []byte("PY_E2E_OK"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep := runcontrol.StepReport{
		Status:    "ok",
		Artifacts: []string{path},
	}
	if err := ValidateReportArtifacts(rep); err != nil {
		t.Fatalf("short valid content should pass: %v", err)
	}
}

func TestValidateReportArtifacts_WhitespaceOnlyFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(path, []byte("  \n\t  "), 0o644); err != nil {
		t.Fatal(err)
	}
	rep := runcontrol.StepReport{
		Status:    "ok",
		Artifacts: []string{path},
	}
	if err := ValidateReportArtifacts(rep); err == nil {
		t.Fatal("whitespace-only artifact should fail")
	}
}

func TestValidateReportArtifacts_StatusNotOKSkips(t *testing.T) {
	rep := runcontrol.StepReport{
		Status:    "fail",
		Artifacts: []string{"missing.txt"},
	}
	if err := ValidateReportArtifacts(rep); err != nil {
		t.Fatalf("non-ok status should skip validation: %v", err)
	}
}

func TestValidateReportArtifacts_EmptyArtifactsOK(t *testing.T) {
	rep := runcontrol.StepReport{Status: "ok", Artifacts: nil}
	if err := ValidateReportArtifacts(rep); err != nil {
		t.Fatalf("empty artifacts should pass: %v", err)
	}
}

func TestValidateReportArtifacts_DirectoryOK(t *testing.T) {
	dir := t.TempDir()
	rep := runcontrol.StepReport{
		Status:    "ok",
		Artifacts: []string{dir},
	}
	if err := ValidateReportArtifacts(rep); err != nil {
		t.Fatalf("existing directory should pass: %v", err)
	}
}

func TestValidateReportArtifacts_MissingPathFails(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "no-such-file.txt")
	rep := runcontrol.StepReport{
		Status:    "ok",
		Artifacts: []string{missing},
	}
	if err := ValidateReportArtifacts(rep); err == nil {
		t.Fatal("missing artifact path should fail")
	}
}
