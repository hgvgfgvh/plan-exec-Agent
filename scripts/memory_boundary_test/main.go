// Memory MCP + plan/memoryhook 边界测试（不依赖 LLM）。
// 用法：在仓库根目录 go run ./scripts/memory_boundary_test
// 可选：-web 在本地 Web UI 已启动时追加 API 冒烟（http://127.0.0.1:8765）
package main

import (
	"AgentTest/config"
	"AgentTest/plan/memoryhook"
	"AgentTest/plan/todolist"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	seedCorrelation = "boundary-test-seed"
	seedRequirement = "列出 WorkSpace 目录下的文件和子目录，把清单保存到 WorkSpace/boundary_plan_dir_list.txt"
	factsPollMax    = 12 * time.Second
	webUser         = "admin"
	webPassword     = "ZAQ!2wsx"
)

var (
	cfgPath   string
	runWeb    bool
	hookOnly  bool
	webBase   string
	dataDir   string
	factsPath string
)

type result struct {
	ID     string
	Name   string
	OK     bool
	Detail string
}

func main() {
	flag.StringVar(&cfgPath, "config", "", "app.yaml 路径（默认 config/app.yaml 或 AGENTTEST_CONFIG）")
	flag.BoolVar(&runWeb, "web", false, "追加 Web UI API 冒烟（需主项目已启动）")
	flag.BoolVar(&hookOnly, "hook-only", false, "仅跑 memoryhook 路由用例（跳过 MCP 直连）")
	flag.StringVar(&webBase, "base", "http://127.0.0.1:8765", "Web UI base URL")
	flag.Parse()

	if cfgPath == "" {
		cfgPath = os.Getenv("AGENTTEST_CONFIG")
		if cfgPath == "" {
			cfgPath = "config/app.yaml"
		}
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}
	config.SetGlobal(cfg)
	dataDir = cfg.PlanMemoryHook.MCPEnv["MEMORY_MCP_DATA_DIR"]
	if dataDir == "" {
		dataDir = filepath.Join(filepath.Dir(cfg.PlanMemoryHook.MCPCommand), "data")
	}
	factsPath = filepath.Join(dataDir, "facts", "facts.jsonl")

	fmt.Println("=== Memory MCP 边界测试 ===")
	fmt.Printf("config=%s engine=%s data=%s\n\n", cfgPath, cfg.PlanMemoryHook.MCPEngine, dataDir)

	var results []result
	results = append(results, runConfigChecks(cfg)...)
	if !hookOnly {
		results = append(results, runMCPDirectTests(cfg)...)
	}
	results = append(results, runHookRouteTests(cfg)...)

	if runWeb {
		results = append(results, runWebSmoke()...)
	}

	passed, total := 0, len(results)
	for _, r := range results {
		st := "FAIL"
		if r.OK {
			st = "PASS"
			passed++
		}
		fmt.Printf("[%s] %s %s\n    %s\n", st, r.ID, r.Name, r.Detail)
	}
	fmt.Printf("\n=== 完成 %d/%d ===\n", passed, total)
	if passed < total {
		os.Exit(1)
	}
}

func runConfigChecks(cfg *config.App) []result {
	var out []result
	storeOn := cfg.PlanMemoryHook.StoreEnabled == nil || *cfg.PlanMemoryHook.StoreEnabled
	ok := cfg.Executor.ExecSimpleEnabled && cfg.PlanMemoryHook.Enabled && storeOn &&
		strings.TrimSpace(cfg.PlanMemoryHook.MCPCommand) != "" &&
		strings.EqualFold(strings.TrimSpace(cfg.PlanMemoryHook.MCPEngine), "factworld")
	out = append(out, result{"C01", "配置开关与 factworld", ok,
		fmt.Sprintf("exec_simple=%v hook=%v engine=%q", cfg.Executor.ExecSimpleEnabled,
			cfg.PlanMemoryHook.Enabled, cfg.PlanMemoryHook.MCPEngine)})
	exe := strings.TrimSpace(cfg.PlanMemoryHook.MCPCommand)
	st, err := os.Stat(exe)
	out = append(out, result{"C02", "memory-mcp.exe 存在", err == nil && !st.IsDir(),
		fmt.Sprintf("path=%s err=%v", exe, err)})
	return out
}

func runMCPDirectTests(cfg *config.App) []result {
	var out []result
	sess, closeFn, err := connectMCP(cfg)
	if err != nil {
		out = append(out, result{"M01", "MCP stdio 连接", false, err.Error()})
		return out
	}
	defer closeFn()

	ctx := context.Background()
	tools, err := sess.ListTools(ctx, nil)
	if err != nil {
		out = append(out, result{"M01", "MCP stdio 连接", false, err.Error()})
		return out
	}
	names := map[string]bool{}
	for _, t := range tools.Tools {
		names[t.Name] = true
	}
	out = append(out, result{"M01", "MCP stdio 连接", true, fmt.Sprintf("tools=%d", len(tools.Tools))})
	out = append(out, result{"M02", "工具仅 store/retrieve",
		names["memory_store"] && names["memory_retrieve"] && len(names) == 2,
		fmt.Sprintf("names=%v", mapKeys(names))})

	before := countFactsLines()
	seed := buildSeedEpisode()
	storeRaw, err := callTool(ctx, sess, "memory_store", map[string]any{
		"content":        seed,
		"source":         "boundary-test",
		"kind":           "episode",
		"correlation_id": seedCorrelation,
	})
	if err != nil {
		out = append(out, result{"M03", "memory_store 接受", false, err.Error()})
		return out
	}
	accepted := strings.Contains(strings.ToLower(storeRaw), `"accepted":"true"`) ||
		strings.Contains(strings.ToLower(storeRaw), `"accepted": "true"`)
	jobID := parseJSONField(storeRaw, "job_id")
	out = append(out, result{"M03", "memory_store 接受", accepted, trim(storeRaw, 200)})

	processed := waitStoreProcessed(jobID, before, factsPollMax)
	out = append(out, result{"M04", "异步抽取完成", processed,
		fmt.Sprintf("job=%s facts_lines=%d->%d", jobID, before, countFactsLines())})

	retRaw, err := callTool(ctx, sess, "memory_retrieve", map[string]any{
		"context":    "用户诉求: " + seedRequirement + "\n",
		"query_hint": seedRequirement,
	})
	if err != nil {
		out = append(out, result{"M05", "memory_retrieve 命中", false, err.Error()})
		return out
	}
	hints := extractHints(retRaw)
	hasRoute := strings.Contains(hints, "---memory-route---")
	matchYes := strings.Contains(hints, `"exec_simple_match":"yes"`) ||
		strings.Contains(hints, `"exec_simple_match": "yes"`) ||
		strings.Contains(hints, "[exec_simple_match=yes")
	out = append(out, result{"M05", "memory_retrieve 命中", hasRoute && matchYes, trim(hints, 280)})

	skippedStore, _ := callTool(ctx, sess, "memory_store", map[string]any{"content": "你好"})
	out = append(out, result{"M06", "寒暄 store 被过滤",
		strings.Contains(strings.ToLower(skippedStore), `"skipped":"true"`) ||
			strings.Contains(strings.ToLower(skippedStore), `"skipped": "true"`),
		trim(skippedStore, 120)})

	emptyRet, _ := callTool(ctx, sess, "memory_retrieve", map[string]any{"context": "  ", "query_hint": ""})
	eh := extractHints(emptyRet)
	out = append(out, result{"M07", "空 context retrieve 跳过",
		strings.Contains(strings.ToLower(emptyRet), `"skipped":"true"`) || strings.TrimSpace(eh) == "",
		trim(emptyRet, 120)})

	return out
}

func runHookRouteTests(cfg *config.App) []result {
	var out []result
	if err := memoryhook.InitFromConfig(cfg); err != nil {
		out = append(out, result{"H01", "memoryhook InitFromConfig", false, err.Error()})
		return out
	}
	out = append(out, result{"H01", "memoryhook InitFromConfig", true,
		"provider=" + memoryhook.Default().ProviderName()})

	doc := seedDocument(2)
	d := memoryhook.Default().DecideRoute(context.Background(), memoryhook.RouteInput{
		Document: doc, SimpleExecutorReady: true,
	})
	out = append(out, result{"H02", "DecideRoute 命中 Exec-Simple", d.UseSimple,
		fmt.Sprintf("skip=%q matched=%v conf=%.2f", d.SkipReason, d.Experience.Matched, d.Experience.Confidence)})

	high := seedDocument(3)
	d2 := memoryhook.Default().DecideRoute(context.Background(), memoryhook.RouteInput{
		Document: high, SimpleExecutorReady: true,
	})
	out = append(out, result{"H03", "tier>max 拒绝快路径", !d2.UseSimple && d2.SkipReason == "tier_too_high",
		fmt.Sprintf("skip=%q max_tier=%d", d2.SkipReason, cfg.Executor.ExecSimpleMaxTier)})

	d3 := memoryhook.Default().DecideRoute(context.Background(), memoryhook.RouteInput{
		Document: seedDocument(1), SimpleExecutorReady: false,
	})
	out = append(out, result{"H04", "simple 未就绪跳过", !d3.UseSimple,
		fmt.Sprintf("skip=%q", d3.SkipReason)})

	return out
}

func runWebSmoke() []result {
	var out []result
	client := &http.Client{Timeout: 0}
	client.Jar = &simpleJar{}
	if err := webLogin(client); err != nil {
		out = append(out, result{"W01", "Web UI 可达", false, err.Error()})
		return out
	}
	out = append(out, result{"W01", "Web UI 可达", true, webBase})

	msg := "列出 WorkSpace 根目录前 5 个条目名称即可，简要回复。"
	text, err := webChat(client, msg, 300*time.Second)
	if err != nil {
		out = append(out, result{"W02", "Plan 门户一轮回复", false, err.Error()})
		return out
	}
	lower := strings.ToLower(text)
	hit := (strings.Contains(lower, "workspace") || strings.Contains(text, "WorkSpace") ||
		strings.Contains(text, "目录")) && strings.Contains(text, "计划编排")
	out = append(out, result{"W02", "Plan 门户一轮回复", hit,
		fmt.Sprintf("runes=%d", len([]rune(text)))})
	return out
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
	transport := &mcpsdk.CommandTransport{Command: execCmd}
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "memory-boundary-test", Version: "0.1"}, nil)
	ctx := context.Background()
	sess, err := client.Connect(ctx, transport, nil)
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
	return mcpText(res), nil
}

func mcpText(res *mcpsdk.CallToolResult) string {
	if res == nil {
		return ""
	}
	var parts []string
	for _, c := range res.Content {
		if t, ok := c.(*mcpsdk.TextContent); ok {
			parts = append(parts, t.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func extractHints(raw string) string {
	raw = strings.TrimSpace(raw)
	var p struct {
		Hints string `json:"hints"`
	}
	if json.Unmarshal([]byte(raw), &p) == nil && strings.TrimSpace(p.Hints) != "" {
		return p.Hints
	}
	return raw
}

func buildSeedEpisode() string {
	return `[source=agenttest-plan turn=boundary-seed plan=boundary-plan]

## 用户诉求
` + seedRequirement + `

## 门户回复
已完成 WorkSpace 目录列举，清单已写入 boundary_plan_dir_list.txt。

## 计划终态 (TodoList)
id: boundary-plan
status: completed
execution_mode: exec-simple
summary: 列出工作区目录结构并写入清单文件
step1: [1] 列出 WorkSpace 目录 tier=2 status=completed
  result_summary: 已写入 boundary_plan_dir_list.txt
  artifacts: boundary_plan_dir_list.txt
  tools_called: filesystem
`
}

func seedDocument(tier int) *todolist.Document {
	return &todolist.Document{
		ID:              todolist.NewID(seedRequirement),
		UserRequirement: seedRequirement,
		Summary:         "列出工作区目录结构并写入清单文件",
		Status:          todolist.PlanActive,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		Steps: []todolist.Step{{
			ID:              "1",
			Title:           "列出 WorkSpace 目录",
			Instruction:     "filesystem 列出并写入 boundary_plan_dir_list.txt",
			CapabilityHints: []string{"filesystem"},
			Tier:            tier,
			Status:          todolist.StepPending,
			UpdatedAt:       time.Now(),
		}},
	}
}

func countFactsLines() int {
	b, err := os.ReadFile(factsPath)
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

func parseJSONField(raw, key string) string {
	var m map[string]string
	if json.Unmarshal([]byte(strings.TrimSpace(raw)), &m) == nil {
		return strings.TrimSpace(m[key])
	}
	return ""
}

// waitStoreProcessed 等待 job 进入 done，或 facts 行数增加，或 facts 已含 seed 关联内容（ReplaceByCorrelation 不增行）。
func waitStoreProcessed(jobID string, beforeLines int, maxWait time.Duration) bool {
	deadline := time.Now().Add(maxWait)
	donePath := ""
	if jobID != "" {
		donePath = filepath.Join(dataDir, "jobs", "done", jobID+".json")
	}
	for time.Now().Before(deadline) {
		if donePath != "" {
			if st, err := os.Stat(donePath); err == nil && !st.IsDir() {
				return true
			}
		}
		if countFactsLines() > beforeLines {
			return true
		}
		if factsContainSeed() {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	if donePath != "" {
		if st, err := os.Stat(donePath); err == nil && !st.IsDir() {
			return true
		}
	}
	return countFactsLines() > beforeLines || factsContainSeed()
}

func factsContainSeed() bool {
	b, err := os.ReadFile(factsPath)
	if err != nil {
		return false
	}
	s := string(b)
	return strings.Contains(s, seedCorrelation) || strings.Contains(s, "boundary_plan_dir_list")
}

func mapKeys(m map[string]bool) []string {
	var k []string
	for x := range m {
		k = append(k, x)
	}
	return k
}

func trim(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

// --- Web UI helpers ---

type sseEntry struct {
	Source string `json:"source"`
	Text   string `json:"text"`
}

type simpleJar struct {
	cookies []*http.Cookie
}

func (j *simpleJar) SetCookies(u *url.URL, cookies []*http.Cookie) { j.cookies = cookies }
func (j *simpleJar) Cookies(u *url.URL) []*http.Cookie             { return j.cookies }

func webLogin(c *http.Client) error {
	body, _ := json.Marshal(map[string]string{"username": webUser, "password": webPassword})
	resp, err := c.Post(webBase+"/api/login", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("login status %d: %s", resp.StatusCode, b)
	}
	return nil
}

func webChat(c *http.Client, message string, timeout time.Duration) (string, error) {
	events := make(chan sseEntry, 64)
	go func() {
		req, _ := http.NewRequest(http.MethodGet, webBase+"/api/events", nil)
		resp, err := c.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			line := sc.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			var e sseEntry
			if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &e) == nil {
				select {
				case events <- e:
				default:
				}
			}
		}
	}()
	time.Sleep(400 * time.Millisecond)
	chatBody, _ := json.Marshal(map[string]string{"message": message})
	resp, err := c.Post(webBase+"/api/chat", "application/json", bytes.NewReader(chatBody))
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("chat status %d", resp.StatusCode)
	}
	var collect strings.Builder
	deadline := time.After(timeout)
	for {
		select {
		case <-deadline:
			if collect.Len() == 0 {
				return "", fmt.Errorf("timeout")
			}
			return collect.String(), nil
		case e := <-events:
			if strings.TrimSpace(e.Source) == "user" || e.Text == "" {
				continue
			}
			collect.WriteString(fmt.Sprintf("[%s] %s\n", e.Source, e.Text))
			if e.Source == "计划编排" && strings.TrimSpace(e.Text) != "" {
				time.Sleep(2 * time.Second)
				return collect.String(), nil
			}
		}
	}
}
