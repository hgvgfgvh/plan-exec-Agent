// L2 语义冲突专项冒烟：独立 data 目录，覆盖候选检测 / C 降权 / A supersede / L2 关闭 / 同 correlation 不走 L2。
// 用法：在 AgentTest 根目录 go run ./scripts/l2_conflict_smoke
package main

import (
	"AgentTest/config"
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type factRow struct {
	ID            string  `json:"id"`
	CorrelationID string  `json:"correlation_id"`
	Outcome       string  `json:"outcome"`
	Weight        float64 `json:"weight"`
	Confidence    float64 `json:"confidence"`
	Superseded    bool    `json:"superseded"`
	IsPitfall     bool    `json:"is_pitfall"`
}

const tool = "filesystem__list_directory"

type caseResult struct {
	ID     string
	Name   string
	OK     bool
	Detail string
}

func main() {
	cfgPath = os.Getenv("AGENTTEST_CONFIG")
	if cfgPath == "" {
		cfgPath = "config/app.yaml"
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		exitErr("load config: %v", err)
	}

	dataDir := filepath.Join(os.TempDir(), fmt.Sprintf("l2-smoke-%d", time.Now().Unix()))
	_ = os.RemoveAll(dataDir)
	env := map[string]string{
		"MEMORY_MCP_DATA_DIR":            dataDir,
		"MEMORY_MCP_LLM_EXTRACT":         "0",
		"MEMORY_MCP_L2_CONFLICT":         "1",
		"MEMORY_MCP_ENTITY_MERGE_COSINE": "0.99",
		"MEMORY_MCP_ENTITY_ALIGN_ASYNC":  "0",
		"MEMORY_MCP_ENTITY_FUZZY_LOW":    "0.85",
	}
	for k, v := range cfg.PlanMemoryHook.MCPEnv {
		if _, ok := env[k]; !ok {
			env[k] = v
		}
	}
	for k, v := range env {
		cfg.PlanMemoryHook.MCPEnv[k] = v
	}

	fmt.Println("=== L2 语义冲突专项测试 ===")
	fmt.Printf("data_dir=%s\n\n", dataDir)

	var results []caseResult

	// --- 单元：MemoryMCP 包内 L2 单测 ---
	results = append(results, runUnitGoTest()...)

	// --- 集成：独立 MCP 数据目录 ---
	sess, closeFn, err := connectMCP(cfg)
	if err != nil {
		exitErr("mcp connect: %v", err)
	}
	defer closeFn()
	ctx := context.Background()

	// L2-I01：success 后 failed → 应触发 L2（无 LLM 时 C：新 fact 降权）
	results = append(results, runL2I01(ctx, sess, dataDir)...)

	// L2-I02：关闭 L2 → 两条均活跃、旧 fact 未 superseded
	env["MEMORY_MCP_L2_CONFLICT"] = "0"
	for k, v := range env {
		cfg.PlanMemoryHook.MCPEnv[k] = v
	}
	_ = sess.Close()
	sess2, close2, err := connectMCP(cfg)
	if err != nil {
		results = append(results, caseResult{"L2-I02", "L2 关闭", false, err.Error()})
	} else {
		results = append(results, runL2I02(ctx, sess2, dataDir)...)
		close2()
	}

	// L2-I03：同 correlation 替换，不应因 L2 堆叠两条同主题
	env["MEMORY_MCP_L2_CONFLICT"] = "1"
	env["MEMORY_MCP_DATA_DIR"] = filepath.Join(os.TempDir(), fmt.Sprintf("l2-smoke-corr-%d", time.Now().Unix()))
	dataDir3 := env["MEMORY_MCP_DATA_DIR"]
	for k, v := range env {
		cfg.PlanMemoryHook.MCPEnv[k] = v
	}
	sess3, close3, err := connectMCP(cfg)
	if err != nil {
		results = append(results, caseResult{"L2-I03", "同 correlation", false, err.Error()})
	} else {
		results = append(results, runL2I03(ctx, sess3, dataDir3)...)
		close3()
	}

	// L2-I04：不同 tool → 无冲突，旧 fact 保持 weight
	env["MEMORY_MCP_DATA_DIR"] = filepath.Join(os.TempDir(), fmt.Sprintf("l2-smoke-iso-%d", time.Now().Unix()))
	dataDir4 := env["MEMORY_MCP_DATA_DIR"]
	for k, v := range env {
		cfg.PlanMemoryHook.MCPEnv[k] = v
	}
	sess4, close4, err := connectMCP(cfg)
	if err != nil {
		results = append(results, caseResult{"L2-I04", "无冲突", false, err.Error()})
	} else {
		results = append(results, runL2I04(ctx, sess4, dataDir4)...)
		close4()
	}

	// L2-I05：启用 LLM 抽取 + 冲突（若 API 可用）
	cfg.PlanMemoryHook.MCPEnv["MEMORY_MCP_DATA_DIR"] = filepath.Join(os.TempDir(), fmt.Sprintf("l2-smoke-llm-%d", time.Now().Unix()))
	cfg.PlanMemoryHook.MCPEnv["MEMORY_MCP_LLM_EXTRACT"] = "1"
	cfg.PlanMemoryHook.MCPEnv["MEMORY_MCP_L2_CONFLICT"] = "1"
	sess5, close5, err := connectMCP(cfg)
	if err != nil {
		results = append(results, caseResult{"L2-I05", "LLM+L2", false, err.Error()})
	} else {
		results = append(results, runL2I05(ctx, sess5, cfg.PlanMemoryHook.MCPEnv["MEMORY_MCP_DATA_DIR"])...)
		close5()
	}

	pass, fail := 0, 0
	for _, r := range results {
		if r.OK {
			pass++
			fmt.Printf("[PASS] %s %s\n    %s\n", r.ID, r.Name, r.Detail)
		} else {
			fail++
			fmt.Printf("[FAIL] %s %s\n    %s\n", r.ID, r.Name, r.Detail)
		}
	}
	fmt.Printf("\n=== 完成 %d/%d ===\n", pass, pass+fail)
	if fail > 0 {
		os.Exit(1)
	}
}

func runUnitGoTest() []caseResult {
	mcpRoot := filepath.Clean(filepath.Join("..", "AgentTestMemoryMCP"))
	if st, err := os.Stat(mcpRoot); err != nil || !st.IsDir() {
		mcpRoot = `C:\DATA\GODATA\AgentTestMemoryMCP`
	}
	cmd := exec.Command("go", "test", "-count=1", "./internal/memoryagent/", "-run", "TestDetectConflict|TestApplyL2|TestL2Conflict|TestApplyL2Conflict")
	cmd.Dir = mcpRoot
	out, err := cmd.CombinedOutput()
	ok := err == nil
	detail := strings.TrimSpace(string(out))
	if len(detail) > 400 {
		detail = detail[len(detail)-400:]
	}
	return []caseResult{{"L2-U00", "memoryagent 单测", ok, detail}}
}

var cfgPath string

func runL2I01(ctx context.Context, sess *mcpsdk.ClientSession, dataDir string) []caseResult {
	tag := "l2-i01"
	store(ctx, sess, episodeSuccess(tag+"-a", "corr-a"), "corr-a")
	waitJob(dataDir, 8*time.Second)
	store(ctx, sess, episodeFail(tag+"-b", "corr-b"), "corr-b")
	waitJob(dataDir, 8*time.Second)

	all := loadFacts(dataDir)
	var oldF, newF *factRow
	for i := range all {
		if all[i].CorrelationID == "corr-a" {
			oldF = &all[i]
		}
		if all[i].CorrelationID == "corr-b" {
			newF = &all[i]
		}
	}
	if oldF == nil || newF == nil {
		return []caseResult{{"L2-I01", "规则+L2 降权 C", false, "missing facts"}}
	}
	// 无 LLM：默认 C → 新 fact confidence/weight 下降；旧 fact 通常不 superseded
	ok := newF.Confidence < 0.88 || newF.Weight < 0.98
	detail := fmt.Sprintf("old superseded=%v w=%.2f | new conf=%.2f w=%.2f", oldF.Superseded, oldF.Weight, newF.Confidence, newF.Weight)
	if !ok && oldF.Superseded {
		ok = true
		detail += " (LLM 可能判 A)"
	}
	return []caseResult{{"L2-I01", "规则+L2 冲突处理", ok, detail}}
}

func runL2I02(ctx context.Context, sess *mcpsdk.ClientSession, dataDir string) []caseResult {
	tag := "l2-i02"
	store(ctx, sess, episodeSuccess(tag+"-a", "c2-a"), "c2-a")
	waitJob(dataDir, 8*time.Second)
	store(ctx, sess, episodeFail(tag+"-b", "c2-b"), "c2-b")
	waitJob(dataDir, 8*time.Second)

	all := loadFacts(dataDir)
	var oldF, newF *factRow
	for i := range all {
		if all[i].CorrelationID == "c2-a" {
			oldF = &all[i]
		}
		if all[i].CorrelationID == "c2-b" {
			newF = &all[i]
		}
	}
	if oldF == nil || newF == nil {
		return []caseResult{{"L2-I02", "L2 关闭", false, "missing facts"}}
	}
	// 新 fact 为 failed/pitfall 时 rules 会给 weight≈0.3，仅断言旧 fact 未被 L2/supersede 降权
	ok := !oldF.Superseded && oldF.Weight >= 0.95 && newF.Outcome == "fail"
	return []caseResult{{"L2-I02", "L2 关闭旧 fact 不降权", ok,
		fmt.Sprintf("old sup=%v w=%.2f new outcome=%s w=%.2f", oldF.Superseded, oldF.Weight, newF.Outcome, newF.Weight)}}
}

func runL2I03(ctx context.Context, sess *mcpsdk.ClientSession, dataDir string) []caseResult {
	corr := "l2-corr-one"
	store(ctx, sess, episodeSuccess("v1", corr), corr)
	waitJob(dataDir, 8*time.Second)
	store(ctx, sess, episodeFail("v2", corr), corr)
	waitJob(dataDir, 8*time.Second)

	all := loadFacts(dataDir)
	n := 0
	var last factRow
	for _, f := range all {
		if f.CorrelationID == corr {
			n++
			last = f
		}
	}
	ok := n == 1 && last.Outcome == "fail"
	return []caseResult{{"L2-I03", "同 correlation 仅一条", ok, fmt.Sprintf("count=%d outcome=%s", n, last.Outcome)}}
}

func runL2I04(ctx context.Context, sess *mcpsdk.ClientSession, dataDir string) []caseResult {
	store(ctx, sess, episodeWithTool("ok-a", "corr-4a", tool), "corr-4a")
	waitJob(dataDir, 8*time.Second)
	store(ctx, sess, episodeWithTool("ok-b", "corr-4b", "sqlite__read_query"), "corr-4b")
	waitJob(dataDir, 8*time.Second)

	all := loadFacts(dataDir)
	var a, b *factRow
	for i := range all {
		if all[i].CorrelationID == "corr-4a" {
			a = &all[i]
		}
		if all[i].CorrelationID == "corr-4b" {
			b = &all[i]
		}
	}
	if a == nil || b == nil {
		return []caseResult{{"L2-I04", "无冲突", false, "missing"}}
	}
	ok := !a.Superseded && !b.Superseded && a.Weight >= 0.9 && b.Weight >= 0.9
	return []caseResult{{"L2-I04", "不同 tool 不冲突", ok,
		fmt.Sprintf("a w=%.2f b w=%.2f", a.Weight, b.Weight)}}
}

func runL2I05(ctx context.Context, sess *mcpsdk.ClientSession, dataDir string) []caseResult {
	tag := "l2-i05"
	store(ctx, sess, episodeSuccess(tag+"-old", "c5-old"), "c5-old")
	waitJob(dataDir, 12*time.Second)
	beforeAtoms := countLines(filepath.Join(dataDir, "atoms", "atoms.jsonl"))
	store(ctx, sess, episodeFail(tag+"-new", "c5-new"), "c5-new")
	waitJob(dataDir, 15*time.Second)

	all := loadFacts(dataDir)
	var oldF, newF *factRow
	for i := range all {
		if all[i].CorrelationID == "c5-old" {
			oldF = &all[i]
		}
		if all[i].CorrelationID == "c5-new" {
			newF = &all[i]
		}
	}
	afterAtoms := countLines(filepath.Join(dataDir, "atoms", "atoms.jsonl"))
	if newF == nil {
		return []caseResult{{"L2-I05", "LLM 抽取+L2", false, "no new fact (L2 B?)"}}
	}
	ok := afterAtoms > beforeAtoms
	detail := fmt.Sprintf("atoms %d->%d old_sup=%v new_conf=%.2f", beforeAtoms, afterAtoms, oldF != nil && oldF.Superseded, newF.Confidence)
	if oldF != nil && (oldF.Superseded || oldF.Weight < 0.5) {
		detail += " supersede/A"
	}
	return []caseResult{{"L2-I05", "LLM 抽取+L2", ok, detail}}
}

func episodeSuccess(turn, corr string) string {
	return fmt.Sprintf(`[source=agenttest-plan turn=%s plan=l2test]
## 用户诉求
用 %s 列出 WorkSpace 目录并保存清单（L2测试成功 %s）
## 门户回复
已完成
## 计划终态 (TodoList)
status: completed
tools_called: %s
artifacts: WorkSpace/l2_%s_ok.txt
`, turn, tool, turn, tool, turn)
}

func episodeFail(turn, corr string) string {
	return fmt.Sprintf(`[source=agenttest-plan turn=%s plan=l2test]
## 用户诉求
用 %s 列出 WorkSpace 目录（L2测试失败 %s）
## 门户回复
执行失败 blocked
## 计划终态 (TodoList)
status: failed
tools_called: %s
`, turn, tool, turn, tool)
}

func episodeWithTool(turn, corr, t string) string {
	return fmt.Sprintf(`[source=agenttest-plan turn=%s]
## 用户诉求
任务 %s
## 门户回复
ok
## 计划终态 (TodoList)
status: completed
tools_called: %s
`, turn, turn, t)
}

func store(ctx context.Context, sess *mcpsdk.ClientSession, content, corr string) {
	_, _ = callTool(ctx, sess, "memory_store", map[string]any{
		"content": content, "source": "agenttest-plan", "kind": "episode",
		"correlation_id": corr,
	})
}

func waitJob(dataDir string, d time.Duration) {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		time.Sleep(400 * time.Millisecond)
		if n, _ := countPending(dataDir); n == 0 {
			time.Sleep(300 * time.Millisecond)
			return
		}
	}
}

func countPending(dataDir string) (int, error) {
	ents, err := os.ReadDir(filepath.Join(dataDir, "jobs", "pending"))
	if err != nil {
		return 0, err
	}
	return len(ents), nil
}

func connectMCP(cfg *config.App) (*mcpsdk.ClientSession, func(), error) {
	cmd := strings.TrimSpace(cfg.PlanMemoryHook.MCPCommand)
	engine := strings.TrimSpace(cfg.PlanMemoryHook.MCPEngine)
	if engine == "" {
		engine = "factworld"
	}
	execCmd := exec.Command(cmd, "-engine", engine)
	execCmd.Env = os.Environ()
	for k, v := range cfg.PlanMemoryHook.MCPEnv {
		execCmd.Env = append(execCmd.Env, k+"="+v)
	}
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "l2-conflict-smoke", Version: "0.1"}, nil)
	sess, err := client.Connect(context.Background(), &mcpsdk.CommandTransport{Command: execCmd}, nil)
	if err != nil {
		return nil, nil, err
	}
	return sess, func() { _ = sess.Close() }, nil
}

func callTool(ctx context.Context, sess *mcpsdk.ClientSession, name string, args map[string]any) (string, error) {
	res, err := sess.CallTool(ctx, &mcpsdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return "", err
	}
	var parts []string
	for _, c := range res.Content {
		if t, ok := c.(*mcpsdk.TextContent); ok {
			parts = append(parts, t.Text)
		}
	}
	return strings.Join(parts, "\n"), nil
}

func loadFacts(dataDir string) []factRow {
	path := filepath.Join(dataDir, "facts", "facts.jsonl")
	fh, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer fh.Close()
	var out []factRow
	sc := bufio.NewScanner(fh)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var f factRow
		if json.Unmarshal([]byte(line), &f) == nil {
			out = append(out, f)
		}
	}
	return out
}

func countLines(path string) int {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	n := 0
	for _, line := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(line) != "" {
			n++
		}
	}
	return n
}

func exitErr(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
