// Soul MCP + plan/soulhook 边界测试（不依赖 Plan LLM）。
// 用法：在仓库根目录 go run ./scripts/soul_boundary_test
package main

import (
	"AgentTest/config"
	"AgentTest/plan/soulhook"
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
	testUserInput   = "叫我老王。我们继续讨论项目 Alpha 和那篇架构论文的结论"
	testAssistant   = "好的，论文的核心结论是分层解耦与可观测性优先。"
	testTurnID      = "soul-boundary-turn-1"
	dialoguePollMax = 5 * time.Second
	webUser         = "admin"
	webPassword     = "ZAQ!2wsx"
)

type result struct {
	ID     string
	Name   string
	OK     bool
	Detail string
}

func main() {
	cfgPath := flag.String("config", "", "app.yaml")
	runWeb := flag.Bool("web", false, "对已启动 Web UI 做一轮门户冒烟")
	webBase := flag.String("base", "http://127.0.0.1:8765", "Web UI base URL")
	flag.Parse()
	if *cfgPath == "" {
		*cfgPath = os.Getenv("AGENTTEST_CONFIG")
		if *cfgPath == "" {
			*cfgPath = "config/app.yaml"
		}
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}
	config.SetGlobal(cfg)

	dataDir := cfg.PlanSoulHook.MCPEnv["SOUL_MCP_DATA_DIR"]
	if dataDir == "" {
		dataDir = filepath.Join(filepath.Dir(cfg.PlanSoulHook.MCPCommand), "data")
	}
	historyDir := filepath.Join(dataDir, "storage", "history")
	personPath := filepath.Join(dataDir, "person.md")
	if _, err := os.Stat(personPath); err != nil {
		personPath = filepath.Join(dataDir, "person.yaml")
	}

	fmt.Println("=== Soul MCP 边界测试 ===")
	fmt.Printf("config=%s enabled=%v provider=%s data=%s\n\n",
		*cfgPath, cfg.PlanSoulHook.Enabled, cfg.PlanSoulHook.Provider, dataDir)

	var results []result
	results = append(results, runConfigChecks(cfg)...)
	results = append(results, runMCPDirect(cfg)...)
	results = append(results, runSoulHook(cfg, historyDir, personPath)...)
	if *runWeb {
		results = append(results, runWebSmoke(cfg, historyDir, personPath, *webBase)...)
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
	ok := cfg.PlanSoulHook.Enabled &&
		strings.EqualFold(strings.TrimSpace(cfg.PlanSoulHook.Provider), "mcp")
	storeOn := cfg.PlanSoulHook.StoreEnabled == nil || *cfg.PlanSoulHook.StoreEnabled
	out = append(out, result{"C01", "plan_soul_hook 已启用", ok,
		fmt.Sprintf("enabled=%v provider=%q store=%v", cfg.PlanSoulHook.Enabled, cfg.PlanSoulHook.Provider, storeOn)})
	exe := strings.TrimSpace(cfg.PlanSoulHook.MCPCommand)
	_, err := os.Stat(exe)
	out = append(out, result{"C02", "soul-mcp.exe 存在", err == nil, fmt.Sprintf("path=%s err=%v", exe, err)})
	return out
}

func runMCPDirect(cfg *config.App) []result {
	var out []result
	sess, closeFn, err := connectSoulMCP(cfg)
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
	out = append(out, result{"M01", "MCP stdio 连接", true, fmt.Sprintf("tools=%v", mapKeys(names))})
	onlySoul := names["soul_store"] && names["soul_retrieve"] && len(names) == 2
	out = append(out, result{"M02", "工具仅 soul_store/retrieve", onlySoul, fmt.Sprintf("%v", names)})

	beforeH := countDailyFacts(historyDir(cfg))
	beforeP := fileSize(personPath(cfg))
	content := soulhook.BuildWebUIDialogueContent(soulhook.WebUITurnInput{
		TurnID: testTurnID, UserInput: testUserInput, AssistantReply: testAssistant,
	})
	storeRaw, err := callTool(ctx, sess, "soul_store", map[string]any{
		"content": content, "source": "agenttest-webui", "kind": "dialogue", "correlation_id": testTurnID,
	})
	if err != nil {
		out = append(out, result{"M03", "soul_store", false, err.Error()})
		return out
	}
	accepted := strings.Contains(strings.ToLower(storeRaw), `"accepted":"true"`) ||
		strings.Contains(strings.ToLower(storeRaw), `"accepted": "true"`)
	phaseOK := strings.Contains(storeRaw, "4-async-pipeline") || strings.Contains(storeRaw, "2-llm-triad")
	out = append(out, result{"M03", "soul_store 接受", accepted && phaseOK, trim(storeRaw, 200)})

	afterH, afterP := waitSoulDataV4(cfg, beforeH, beforeP, dialoguePollMax)
	out = append(out, result{"M04", "异步写入 daily/person", afterH > beforeH || afterP > beforeP,
		fmt.Sprintf("daily_facts %d->%d person_bytes %d->%d", beforeH, afterH, beforeP, afterP)})

	retRaw, err := callTool(ctx, sess, "soul_retrieve", map[string]any{
		"context": "用户输入: " + testUserInput + "\n", "query_hint": testUserInput,
	})
	if err != nil {
		out = append(out, result{"M05", "soul_retrieve", false, err.Error()})
		return out
	}
	hints := extractHints(retRaw)
	noMemoryRoute := !strings.Contains(hints, "exec_simple_match") && !strings.Contains(hints, "memory-route")
	hasSoulMarker := strings.Contains(hints, "Soul") || strings.Contains(hints, "协作人格")
	out = append(out, result{"M05", "soul_retrieve hints", strings.TrimSpace(hints) != "" && noMemoryRoute && hasSoulMarker,
		fmt.Sprintf("no_route=%v soul_marker=%v len=%d", noMemoryRoute, hasSoulMarker, len(hints))})

	chit := "[source=agenttest-webui]\n\n## 用户（WebUI）\n你好\n\n## 助手（WebUI）\n您好\n"
	skipped, _ := callTool(ctx, sess, "soul_store", map[string]any{
		"content": chit, "source": "agenttest-webui", "kind": "dialogue",
	})
	sk := strings.Contains(strings.ToLower(skipped), `"skipped":"true"`) ||
		strings.Contains(strings.ToLower(skipped), `"skipped": "true"`)
	out = append(out, result{"M06", "寒暄 store 过滤", sk, trim(skipped, 120)})

	return out
}

func runSoulHook(cfg *config.App, historyDir, personPath string) []result {
	var out []result
	if err := soulhook.InitFromConfig(cfg); err != nil {
		out = append(out, result{"H01", "soulhook InitFromConfig", false, err.Error()})
		return out
	}
	out = append(out, result{"H01", "soulhook InitFromConfig", true, "provider=" + soulhook.Default().ProviderName()})

	ctx := context.Background()
	userOnly := "昨天那篇架构论文的结论是什么"
	hints := soulhook.Default().RetrieveTurnBeforeProcess(ctx, userOnly)
	out = append(out, result{"H02", "Retrieve 仅用用户输入", strings.TrimSpace(hints) != "",
		fmt.Sprintf("hints_len=%d contains_assistant=%v", len(hints), strings.Contains(hints, testAssistant))})

	combined := soulhook.CombineTurnHints(userOnly, "SOUL_BLOCK", "MEMORY_BLOCK")
	orderOK := strings.Index(combined, "SOUL_BLOCK") < strings.Index(combined, "MEMORY_BLOCK") &&
		strings.Index(combined, "MEMORY_BLOCK") < strings.Index(combined, userOnly)
	out = append(out, result{"H03", "CombineTurnHints 顺序", orderOK, trim(combined, 180)})

	beforeH := countDailyFacts(historyDir)
	beforeP := fileSize(personPath)
	soulhook.Default().StoreTurnAfterProcess(ctx, soulhook.WebUITurnInput{
		TurnID: testTurnID + "-hook", UserInput: testUserInput, AssistantReply: testAssistant,
	})
	afterH, afterP := waitSoulDataV4(cfg, beforeH, beforeP, 15*time.Second)
	dataOK := dailyContains(historyDir, "Alpha") || dailyContains(historyDir, "架构") ||
		fileContains(personPath, "老王") || afterP > beforeP
	out = append(out, result{"H04", "StoreTurn 用户+助手入 MCP", afterH > beforeH || dataOK,
		fmt.Sprintf("daily_facts %d->%d person %d->%d", beforeH, afterH, beforeP, afterP)})

	chitH, chitP := countDailyFacts(historyDir), fileSize(personPath)
	soulhook.Default().StoreTurnAfterProcess(ctx, soulhook.WebUITurnInput{
		TurnID: "chit", UserInput: "你好", AssistantReply: "您好",
	})
	time.Sleep(400 * time.Millisecond)
	out = append(out, result{"H05", "寒暄不 store",
		countDailyFacts(historyDir) == chitH && fileSize(personPath) == chitP,
		"history/person unchanged"})

	return out
}

func runWebSmoke(cfg *config.App, historyDir, personPath, webBase string) []result {
	var out []result
	before := countDailyFacts(historyDir)
	client := &http.Client{Timeout: 0}
	client.Jar = &simpleJar{}
	if err := webLogin(client, webBase); err != nil {
		out = append(out, result{"W01", "Web UI 登录", false, err.Error()})
		return out
	}
	out = append(out, result{"W01", "Web UI 登录", true, webBase})

	msg := "请用一句话确认你已收到：我们在讨论 Soul MCP 人格记忆对接测试。"
	text, err := webChat(client, webBase, msg, 180*time.Second)
	if err != nil {
		out = append(out, result{"W02", "门户一轮对话", false, err.Error()})
		return out
	}
	gotReply := strings.Contains(text, "计划编排") || strings.Contains(text, "行为编排")
	out = append(out, result{"W02", "门户一轮对话", gotReply, fmt.Sprintf("runes=%d", len([]rune(text)))})

	deadline := time.Now().Add(8 * time.Second)
	after := before
	for time.Now().Before(deadline) {
		after = countDailyFacts(historyDir)
		if after > before {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	out = append(out, result{"W03", "Web 回合后 soul_store", after > before,
		fmt.Sprintf("storage/history daily_facts %d -> %d", before, after)})
	return out
}

type sseEntry struct {
	Source string `json:"source"`
	Text   string `json:"text"`
}

type simpleJar struct {
	cookies []*http.Cookie
}

func (j *simpleJar) SetCookies(u *url.URL, cookies []*http.Cookie) { j.cookies = cookies }
func (j *simpleJar) Cookies(u *url.URL) []*http.Cookie             { return j.cookies }

func webLogin(c *http.Client, webBase string) error {
	body, _ := json.Marshal(map[string]string{"username": webUser, "password": webPassword})
	resp, err := c.Post(webBase+"/api/login", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("login %d: %s", resp.StatusCode, b)
	}
	return nil
}

func webChat(c *http.Client, webBase, message string, timeout time.Duration) (string, error) {
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
				return "", fmt.Errorf("timeout waiting portal reply")
			}
			return collect.String(), nil
		case e := <-events:
			if strings.TrimSpace(e.Source) == "user" || e.Text == "" {
				continue
			}
			collect.WriteString(fmt.Sprintf("[%s] %s\n", e.Source, e.Text))
			if (e.Source == "计划编排" || e.Source == "行为编排") && strings.TrimSpace(e.Text) != "" {
				time.Sleep(2 * time.Second)
				return collect.String(), nil
			}
		}
	}
}

func connectSoulMCP(cfg *config.App) (*mcpsdk.ClientSession, func(), error) {
	cmd := strings.TrimSpace(cfg.PlanSoulHook.MCPCommand)
	execCmd := exec.Command(cmd)
	execCmd.Env = os.Environ()
	for k, v := range cfg.PlanSoulHook.MCPEnv {
		execCmd.Env = append(execCmd.Env, k+"="+v)
	}
	transport := &mcpsdk.CommandTransport{Command: execCmd}
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "soul-boundary-test", Version: "0.1"}, nil)
	sess, err := client.Connect(context.Background(), transport, nil)
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

func soulDataDir(cfg *config.App) string {
	d := cfg.PlanSoulHook.MCPEnv["SOUL_MCP_DATA_DIR"]
	if d == "" {
		d = filepath.Join(filepath.Dir(cfg.PlanSoulHook.MCPCommand), "data")
	}
	return d
}

func historyDir(cfg *config.App) string {
	return filepath.Join(soulDataDir(cfg), "storage", "history")
}

func personPath(cfg *config.App) string {
	p := filepath.Join(soulDataDir(cfg), "person.md")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return filepath.Join(soulDataDir(cfg), "person.yaml")
}

func waitSoulDataV4(cfg *config.App, beforeH, beforeP int, maxWait time.Duration) (int, int) {
	deadline := time.Now().Add(maxWait)
	dir := historyDir(cfg)
	pp := personPath(cfg)
	for time.Now().Before(deadline) {
		ah := countDailyFacts(dir)
		ap := fileSize(pp)
		if ah > beforeH || ap > beforeP {
			return ah, ap
		}
		time.Sleep(100 * time.Millisecond)
	}
	return countDailyFacts(dir), fileSize(pp)
}

func countDailyFacts(historyDir string) int {
	entries, err := os.ReadDir(historyDir)
	if err != nil {
		return countLines(filepath.Join(soulDataDirFromHist(historyDir), "history.facts.jsonl"))
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		n += countLines(filepath.Join(historyDir, e.Name()))
	}
	return n
}

func soulDataDirFromHist(historyDir string) string {
	return filepath.Dir(filepath.Dir(historyDir))
}

func dailyContains(historyDir, sub string) bool {
	entries, err := os.ReadDir(historyDir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		if fileContains(filepath.Join(historyDir, e.Name()), sub) {
			return true
		}
	}
	return false
}

func fileSize(path string) int {
	st, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return int(st.Size())
}

func fileContains(path, sub string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.Contains(string(b), sub)
}

func countLines(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	n := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) != "" {
			n++
		}
	}
	return n
}

func mapKeys(m map[string]bool) []string {
	var k []string
	for x := range m {
		k = append(k, x)
	}
	return k
}

func trim(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
