// 通过 Web UI API（/api/login、/api/chat、/api/events）对本地 Agent 做边界测试。
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultBase   = "http://127.0.0.1:8765"
	user          = "admin"
	password      = "ZAQ!2wsx"
	workspaceRoot = `C:\DATA\GODATA\AgentTest\WorkSpace`
)

type testCase struct {
	ID           int
	Name         string
	Message      string
	Timeout      time.Duration
	WantSubs     []string // 期望在输出里至少出现其一（空=仅检查有回复）
	RequireFiles []string // 硬验收：文件必须存在且非空（相对 workspaceRoot 或绝对路径）
}

type sseEntry struct {
	Source string `json:"source"`
	Text   string `json:"text"`
}

func main() {
	baseURL := defaultBase
	fromID := 0
	toID := 0
	flag.StringVar(&baseURL, "base", defaultBase, "Web UI base URL")
	flag.IntVar(&fromID, "from", 0, "仅运行 ID>=from 的用例（0=不限）")
	flag.IntVar(&toID, "to", 0, "仅运行 ID<=to 的用例（0=不限）")
	flag.Parse()

	client := &http.Client{Timeout: 0}
	jar := &simpleJar{}
	client.Jar = jar

	if err := login(client, baseURL); err != nil {
		fmt.Fprintf(os.Stderr, "login failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("=== AgentTest API 边界测试（15 项，#11-15 高复杂 / PlanAgent）===")
	fmt.Printf("target: %s\n\n", baseURL)

	tests := []testCase{
		// --- 基础 1-10 ---
		{1, "问候", "你好", 120 * time.Second, []string{"你好", "计划", "编排", "助手"}, nil},
		{2, "简单算术", "1+1等于多少？只回答数字。", 120 * time.Second, []string{"2"}, nil},
		{3, "MCP 列举", "你有那些MCP？只列服务名和工具数量。", 180 * time.Second, []string{"sqlite", "filesystem", "resend"}, nil},
		{4, "SQLite 表", "用 MCP 列出当前 SQLite 数据库里有哪些表，把结果简要告诉我。", 300 * time.Second, []string{"sqlite", "表", "table", "list"}, nil},
		{5, "Filesystem 列目录", "用 filesystem MCP 列出工作区 C:\\DATA\\GODATA\\AgentTest\\WorkSpace 根目录下的条目名称（前 10 个即可）。", 300 * time.Second, []string{"WorkSpace", "filesystem", "目录", "file", "ToDoList"}, nil},
		{6, "能力 Schema", "调用 get_capability_details，builtin_skills 填 SeeCameraAndDescribe，把参数说明前 200 字告诉我。", 300 * time.Second, []string{"SeeCameraAndDescribe", "builtin", "能力", "参数"}, nil},
		{7, "摄像头技能", "请用 SetExecutorStep 调用内置技能 SeeCameraAndDescribe（args 为空数组），把技能返回的完整描述发给我。", 420 * time.Second, []string{"摄像", "画面", "SeeCamera", "技能", "执行", "相机"}, nil},
		{8, "能力目录工具", "请调用 list_agent_capabilities（无参），在回复里说明返回里是否包含 sqlite 和 resend 两段。", 300 * time.Second, []string{"sqlite", "resend", "AGENTS", "能力"}, nil},
		{9, "摄像头+写文件", "先调用 SeeCameraAndDescribe，再把真实返回的摘要写入工作区文件 C:\\DATA\\GODATA\\AgentTest\\WorkSpace\\boundary_camera_summary.txt（用 filesystem MCP 写），最后告诉我文件是否写入成功。", 600 * time.Second, []string{"boundary_camera", "写入", "filesystem", "摄像", "成功", "文件"}, []string{"boundary_camera_summary.txt"}},
		{10, "Resend 只读", "用 resend MCP 调用 resend__list_domains，把返回中的域名数量或前 2 条记录告诉我（不要发邮件）。", 300 * time.Second, []string{"resend", "domain", "域名", "list"}, nil},

		// --- 高复杂 11-15（PlanAgent 多步编排）---
		{11, "Plan-列目录写清单", "请完成：1) 用 filesystem 列出 WorkSpace 根目录前 8 个条目；2) 把名称列表写入 WorkSpace/boundary_plan_dir_list.txt；3) 告诉我文件路径与条目数。", 720 * time.Second, []string{"boundary_plan_dir_list", "WorkSpace", "计划", "TodoList", "步骤", "写入", "文件"}, []string{"boundary_plan_dir_list.txt"}},
		{12, "Plan-SQLite两步", "请完成：先用 sqlite MCP 列出所有表名；再对第一个表执行 SELECT * LIMIT 3；把表名与 3 行样例数据汇总回复我。", 720 * time.Second, []string{"sqlite", "表", "SELECT", "计划", "步骤", "查询"}, nil},
		{13, "Plan-摄像头+摘要文件", "请完成：1) SetExecutorStep 执行 SeeCameraAndDescribe；2) 将真实返回摘要写入 WorkSpace/boundary_plan_camera.txt；3) 确认文件存在并复述摘要前 100 字。", 900 * time.Second, []string{"boundary_plan_camera", "摄像", "SeeCamera", "文件", "计划", "写入"}, []string{"boundary_plan_camera.txt"}},
		{14, "Plan-读回校验", "请读取 WorkSpace/boundary_plan_camera.txt：若存在则输出前 300 字并说明文件大小；若不存在则创建空文件并说明。", 600 * time.Second, []string{"boundary_plan_camera", "文件", "读取", "不存在", "字节", "计划"}, []string{"boundary_plan_camera.txt"}},
		{15, "Plan-三步汇总", "请完成并汇总：A) sqlite 列出表并报告表数量；B) 将表数量写入 WorkSpace/boundary_plan_final.txt；C) 调用 resend__list_domains 只读报告域名条数。不要发邮件。", 900 * time.Second, []string{"boundary_plan_final", "sqlite", "resend", "domain", "计划", "表", "域名"}, []string{"boundary_plan_final.txt"}},
	}

	passed := 0
	run := 0
	for _, tc := range tests {
		if fromID > 0 && tc.ID < fromID {
			continue
		}
		if toID > 0 && tc.ID > toID {
			continue
		}
		run++
		fmt.Printf("--- 开始 #%d %s (timeout %s) ---\n", tc.ID, tc.Name, tc.Timeout)
		ok, detail := runOne(client, baseURL, tc)
		status := "FAIL"
		if ok {
			status = "PASS"
			passed++
		}
		fmt.Printf("[%s] #%d %s\n    %s\n\n", status, tc.ID, tc.Name, detail)
	}
	fmt.Printf("=== 完成: %d/%d 通过（共运行 %d 项）===\n", passed, run, run)
	if passed < run {
		os.Exit(1)
	}
}

func login(c *http.Client, baseURL string) error {
	body, _ := json.Marshal(map[string]string{"username": user, "password": password})
	resp, err := c.Post(baseURL+"/api/login", "application/json", bytes.NewReader(body))
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

func runOne(c *http.Client, baseURL string, tc testCase) (bool, string) {
	events := make(chan sseEntry, 128)
	done := make(chan struct{})
	var collect strings.Builder

	go func() {
		defer close(done)
		req, _ := http.NewRequest(http.MethodGet, baseURL+"/api/events", nil)
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
			if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &e) != nil {
				continue
			}
			select {
			case events <- e:
			default:
			}
		}
	}()

	time.Sleep(400 * time.Millisecond)

	chatBody, _ := json.Marshal(map[string]string{"message": tc.Message})
	chatResp, err := c.Post(baseURL+"/api/chat", "application/json", bytes.NewReader(chatBody))
	if err != nil {
		return false, "POST /api/chat: " + err.Error()
	}
	chatResp.Body.Close()
	if chatResp.StatusCode != http.StatusOK {
		return false, fmt.Sprintf("chat status %d", chatResp.StatusCode)
	}

	deadline := time.After(tc.Timeout)
	var gotReply bool
	sources := map[string]int{}

	// PlanAgent：优先在收到「计划编排」后结束等待（复杂任务可能很久才汇总）
	planDone := false

	for {
		select {
		case <-deadline:
			goto finish
		case e := <-events:
			src := strings.TrimSpace(e.Source)
			if src == "user" {
				continue
			}
			sources[src]++
			collect.WriteString(fmt.Sprintf("[%s] %s\n", src, trimRunes(e.Text, 600)))
			if isAssistantSource(src) {
				gotReply = true
			}
			if src == "计划编排" && strings.TrimSpace(e.Text) != "" {
				planDone = true
				// 再等 2s 收齐可能的「反馈」
				select {
				case e2 := <-events:
					src2 := strings.TrimSpace(e2.Source)
					if src2 != "user" {
						sources[src2]++
						collect.WriteString(fmt.Sprintf("[%s] %s\n", src2, trimRunes(e2.Text, 400)))
					}
				case <-time.After(2 * time.Second):
				}
				goto finish
			}
		}
	}

finish:
	if planDone {
		gotReply = true
	}
	detail := fmt.Sprintf("sources=%v | gotReply=%v | planDone=%v", sources, gotReply, planDone)
	out := collect.String()
	if !gotReply {
		return false, detail + " | 超时无助手/计划输出"
	}
	if len(tc.WantSubs) == 0 && len(tc.RequireFiles) == 0 {
		return true, detail
	}
	lower := strings.ToLower(out)
	matched := len(tc.WantSubs) == 0
	for _, w := range tc.WantSubs {
		if strings.Contains(lower, strings.ToLower(w)) {
			matched = true
			detail += " | matched=" + w
			break
		}
	}
	if !matched {
		return false, detail + " | 缺少期望关键词; 输出片段:\n" + trimRunes(out, 1200)
	}
	if len(tc.RequireFiles) > 0 {
		if ok, why := verifyRequiredFiles(tc.RequireFiles); !ok {
			return false, detail + " | 文件验收失败: " + why
		}
		detail += " | files_ok"
	}
	return true, detail
}

func verifyRequiredFiles(paths []string) (bool, string) {
	for _, f := range paths {
		p := f
		if !filepath.IsAbs(p) {
			p = filepath.Join(workspaceRoot, f)
		}
		st, err := os.Stat(p)
		if err != nil {
			return false, p + ": " + err.Error()
		}
		if st.IsDir() {
			return false, p + ": 是目录而非文件"
		}
		if st.Size() == 0 {
			return false, p + ": 文件为空"
		}
	}
	return true, ""
}

func isAssistantSource(src string) bool {
	switch src {
	case "反馈", "行为编排", "计划编排", "Affective", "Router", "系统异常":
		return true
	default:
		return strings.Contains(src, "编排") || strings.Contains(src, "反馈") || strings.Contains(src, "计划")
	}
}

func trimRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

type simpleJar struct {
	cookies []*http.Cookie
}

func (j *simpleJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	j.cookies = cookies
}
func (j *simpleJar) Cookies(u *url.URL) []*http.Cookie {
	return j.cookies
}
