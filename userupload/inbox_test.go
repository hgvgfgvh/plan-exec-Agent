package userupload

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAllocUniqueRelName(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "README.md")
	if err := os.WriteFile(first, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := allocUniqueRelName(dir, "README.md")
	if err != nil {
		t.Fatal(err)
	}
	if got != "README (2).md" {
		t.Fatalf("got %q want README (2).md", got)
	}
	if err := os.WriteFile(filepath.Join(dir, got), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	got3, err := allocUniqueRelName(dir, "README.md")
	if err != nil {
		t.Fatal(err)
	}
	if got3 != "README (3).md" {
		t.Fatalf("got %q want README (3).md", got3)
	}
}

func TestAllocUniqueRelNameKeepsFirst(t *testing.T) {
	dir := t.TempDir()
	got, err := allocUniqueRelName(dir, "notes.md")
	if err != nil {
		t.Fatal(err)
	}
	if got != "notes.md" {
		t.Fatalf("got %q want notes.md", got)
	}
}

func TestAllocUniqueRelNameNestedPath(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "pkg")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "README.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := allocUniqueRelName(dir, "pkg/README.md")
	if err != nil {
		t.Fatal(err)
	}
	if got != "pkg/README (2).md" {
		t.Fatalf("got %q want pkg/README (2).md", got)
	}
}

func TestNumberedFileName(t *testing.T) {
	if got := numberedFileName("README.md", 2); got != "README (2).md" {
		t.Fatalf("got %q", got)
	}
	if got := numberedFileName("README", 3); got != "README (3)" {
		t.Fatalf("got %q", got)
	}
	if !strings.Contains(numberedFileName(".env", 2), "(2)") {
		t.Fatalf("unexpected %q", numberedFileName(".env", 2))
	}
}
