// Package lintcheck 含静态规则测试：禁止 Plan 路径用用户原文关键词/正则绕过 LLM 分流。
package lintcheck

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// 禁止 reintroduce 的标识与模式（用户输入 → 固定链路）。
var forbiddenSubstrings = []string{
	"func Classify(",
	"TierTrivial",
	"TierLight",
	"TrivialStep(",
	"LightStep(",
	"IsSimpleMathQuery",
	"toolCueWords",
	"reGreeting",
	"reSimpleMath",
	"寒暄单步（规则）",
	"轻量单步（规则）",
}

var forbiddenRegexInPlan = regexp.MustCompile(`regexp\.MustCompile\([^)]*(?:你好|您好|寒暄|greeting|toolCue)`)

func TestNoUserInputRuleRoutingInPlanPath(t *testing.T) {
	root := findModuleRoot(t)
	scanDirs := []string{
		filepath.Join(root, "plan"),
		filepath.Join(root, "agent", "agent"),
	}
	var violations []string
	for _, dir := range scanDirs {
		_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			if filepath.Base(path) == "no_rule_routing_test.go" {
				return nil
			}
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			content := string(b)
			for _, forbidden := range forbiddenSubstrings {
				if strings.Contains(content, forbidden) {
					violations = append(violations, path+": contains forbidden "+forbidden)
				}
			}
			if forbiddenRegexInPlan.MatchString(content) {
				violations = append(violations, path+": forbidden greeting/tool regexp.MustCompile")
			}
			if strings.Contains(path, string(filepath.Join("plan", "intent"))+string(filepath.Separator)) ||
				strings.Contains(path, "plan/intent/") {
				if err := checkIntentPackageOnlyConstants(path, content); err != nil {
					violations = append(violations, err.Error())
				}
			}
			return nil
		})
	}
	if len(violations) > 0 {
		t.Fatalf("user-input rule routing lint failed:\n%s", strings.Join(violations, "\n"))
	}
}

func checkIntentPackageOnlyConstants(path, content string) error {
	if strings.Contains(content, "func Classify") ||
		strings.Contains(content, "regexp.") ||
		strings.Contains(content, "switch ") && strings.Contains(content, "Tier") {
		return &lintErr{path + ": plan/intent must not contain Classify/switch/regexp routing"}
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, content, parser.ParseComments)
	if err != nil {
		return err
	}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		return &lintErr{path + ": plan/intent must not declare functions with bodies (only consts allowed)"}
	}
	return nil
}

type lintErr struct{ msg string }

func (e *lintErr) Error() string { return e.msg }

func findModuleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
