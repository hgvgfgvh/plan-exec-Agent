// 多步骤 Plan + Memory MCP 复杂流程测试（需主项目 Web UI 已启动）。
// 用法：go run ./scripts/memory_complex_test
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
	workspaceRoot   = `C:\DATA\GODATA\AgentTest\WorkSpace`
	seedCorrelation = "boundary-complex-3step"
	complexReq      = "请完成三步：1) 用 filesystem 列出 WorkSpace 根目录前 6 个条目名称；2) 把名称列表写入 WorkSpace/boundary_memory_complex_3step.txt；3) 回复文件路径与条目数。"
	outFile         = "boundary_memory_complex_3step.txt"
	webUser         = "admin"
	webPassword     = "ZAQ!2wsx"
)

var webBase string

func main() {
	flag.StringVar(&webBase, "base", "http://127.0.0.1:8765", "Web UI base URL")
	flag.Parse()

	cfgPath := os.Getenv("AGENTTEST_CONFIG")
	if cfgPath == "" {
		cfgPath = "config/app.yaml"
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		exitErr("load config: %v", err)
	}
	config.SetGlobal(cfg)

	fmt.Println("=== Memory 多步骤复杂流程测试 ===")
	fmt.Printf("target=%s outfile=%s\n\n", webBase, outFile)

	passed := 0
	total := 0
	check := func(name string, ok bool, detail string) {
		total++
		st := "FAIL"
		if ok {
			st = "PASS"
			passed++
		}
		fmt.Printf("[%s] %s\n    %s\n\n", st, name, detail)
	}

	// 1) Seed factworld：模拟曾成功完成的三步 filesystem 任务
	if err := seedComplexEpisode(cfg); err != nil {
		check("Seed 三步 episode 到 factworld", false, err.Error())
	} else {
		check("Seed 三步 episode 到 factworld", true, "correlation="+seedCorrelation)
	}

	// 2) Hook 路由：多步 tier=2 文档应仍可 Exec-Simple
	if err := memoryhook.InitFromConfig(cfg); err != nil {
		check("DecideRoute 多步 tier≤2", false, err.Error())
	} else {
		doc := complexDocument(3)
		d := memoryhook.Default().DecideRoute(context.Background(), memoryhook.RouteInput{
			Document: doc, SimpleExecutorReady: true,
		})
		ok := d.UseSimple && d.Experience.Matched && d.Experience.Confidence >= cfg.Executor.ExecSimpleMinConfidence
		check("DecideRoute 多步 tier≤2", ok,
			fmt.Sprintf("use_simple=%v skip=%q conf=%.2f steps=%d max_tier=%d",
				d.UseSimple, d.SkipReason, d.Experience.Confidence, len(doc.Steps), maxTier(doc)))
	}

	// 3) tier=3 应拒绝快路径（设计护栏）
	docHigh := complexDocument(3)
	for i := range docHigh.Steps {
		docHigh.Steps[i].Tier = 3
	}
	if err := memoryhook.InitFromConfig(cfg); err == nil {
		d := memoryhook.Default().DecideRoute(context.Background(), memoryhook.RouteInput{
			Document: docHigh, SimpleExecutorReady: true,
		})
		check("DecideRoute tier=3 走逐步 Exec", !d.UseSimple && d.SkipReason == "tier_too_high",
			fmt.Sprintf("skip=%q", d.SkipReason))
	}

	// 4) Web UI 端到端
	client := &http.Client{Timeout: 0}
	client.Jar = &simpleJar{}
	if err := webLogin(client); err != nil {
		check("Web UI 登录", false, err.Error())
		fmt.Printf("=== 完成 %d/%d（Web 未跑）===\n", passed, total)
		os.Exit(1)
	}
	check("Web UI 登录", true, webBase)

	text, err := webChat(client, complexReq, 12*time.Minute)
	if err != nil {
		check("Plan 三步任务门户回复", false, err.Error())
	} else {
		lower := strings.ToLower(text)
		hasPlan := strings.Contains(text, "计划编排")
		hasFile := strings.Contains(lower, "boundary_memory_complex_3step") ||
			strings.Contains(text, outFile)
		hasList := strings.Contains(text, "WorkSpace") || strings.Contains(text, "条目") ||
			strings.Contains(text, "filesystem") || strings.Contains(text, ".gitkeep")
		check("Plan 三步任务门户回复", hasPlan && (hasFile || hasList),
			fmt.Sprintf("plan=%v file=%v list=%v runes=%d", hasPlan, hasFile, hasList, len([]rune(text))))
	}

	outPath := filepath.Join(workspaceRoot, outFile)
	if st, err := os.Stat(outPath); err != nil {
		check("产物文件非空", false, err.Error())
	} else {
		check("产物文件非空", !st.IsDir() && st.Size() > 0,
			fmt.Sprintf("%s size=%d", outPath, st.Size()))
	}

	// 5) 最近 TodoList 应 completed 且含多步或 simple
	todoOK, todoDetail := verifyLatestTodoList()
	check("TodoList 终态 completed", todoOK, todoDetail)

	fmt.Printf("=== 完成 %d/%d ===\n", passed, total)
	if passed < total {
		os.Exit(1)
	}
}

func seedComplexEpisode(cfg *config.App) error {
	sess, closeFn, err := connectMCP(cfg)
	if err != nil {
		return err
	}
	defer closeFn()
	content := `[source=agenttest-plan turn=complex-seed plan=complex-plan]

## 用户诉求
` + complexReq + `

## 门户回复
已完成三步：列出 WorkSpace 根目录前 6 项并写入 boundary_memory_complex_3step.txt。

## 计划终态 (TodoList)
id: complex-plan
status: completed
execution_mode: simple
summary: 三步列出目录、写文件、汇报路径与条目数
step1: [1] 列出 WorkSpace 前6项 tier=2 status=completed
  tools_called: filesystem__list_directory
step2: [2] 写入清单文件 tier=2 status=completed
  artifacts: boundary_memory_complex_3step.txt
  tools_called: filesystem__write_file
step3: [3] 汇报路径与条目数 tier=1 status=completed
  result_summary: 已写入 WorkSpace/boundary_memory_complex_3step.txt 共6条
`
	_, err = callTool(context.Background(), sess, "memory_store", map[string]any{
		"content": content, "source": "complex-test", "kind": "episode",
		"correlation_id": seedCorrelation,
	})
	return err
}

func complexDocument(steps int) *todolist.Document {
	if steps < 1 {
		steps = 3
	}
	var st []todolist.Step
	for i := 1; i <= steps; i++ {
		tier := 2
		if i == steps {
			tier = 1
		}
		st = append(st, todolist.Step{
			ID: fmt.Sprintf("%d", i), Title: fmt.Sprintf("步骤%d", i),
			Instruction: "filesystem 相关操作", Tier: tier,
			Status: todolist.StepPending, UpdatedAt: time.Now(),
		})
	}
	return &todolist.Document{
		ID:              todolist.NewID(complexReq),
		UserRequirement: complexReq,
		Summary:         "三步：列目录、写文件、汇报",
		Status:          todolist.PlanActive,
		CreatedAt:       time.Now(), UpdatedAt: time.Now(),
		Steps: st,
	}
}

func maxTier(doc *todolist.Document) int {
	m := 1
	for _, s := range doc.Steps {
		if s.Tier > m {
			m = s.Tier
		}
	}
	return m
}

func verifyLatestTodoList() (bool, string) {
	dir := filepath.Join(workspaceRoot, "ToDoList")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err.Error()
	}
	var latest string
	var latestMod time.Time
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		st, err := os.Stat(p)
		if err != nil {
			continue
		}
		if st.ModTime().After(latestMod) {
			latestMod = st.ModTime()
			latest = p
		}
	}
	if latest == "" {
		return false, "无 TodoList json"
	}
	b, err := os.ReadFile(latest)
	if err != nil {
		return false, err.Error()
	}
	var doc struct {
		Status        string `json:"status"`
		ExecutionMode string `json:"execution_mode"`
		Steps         []struct {
			Status string `json:"status"`
			Tier   int    `json:"tier"`
		} `json:"steps"`
	}
	if json.Unmarshal(b, &doc) != nil {
		return false, "parse " + latest
	}
	n := len(doc.Steps)
	ok := doc.Status == "completed" && n >= 1
	return ok, fmt.Sprintf("file=%s status=%s mode=%s steps=%d", filepath.Base(latest), doc.Status, doc.ExecutionMode, n)
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
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "memory-complex-test", Version: "0.1"}, nil)
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
		return fmt.Errorf("status %d: %s", resp.StatusCode, b)
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
				return "", fmt.Errorf("timeout %s", timeout)
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

func exitErr(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
