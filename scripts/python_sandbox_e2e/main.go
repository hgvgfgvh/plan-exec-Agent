// 经主项目 Web UI（POST /api/chat）触发 Plan→Behavior，验证 python-sandbox MCP 联通与边界。
// 用法（仓库根、主项目已启动且 web.enabled）：
//
//	go run ./scripts/python_sandbox_e2e
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	webBase     = "http://127.0.0.1:8765"
	webUser     = "admin"
	webPassword = "ZAQ!2wsx"
	chatTimeout = 8 * time.Minute
)

type result struct {
	ID     string
	Name   string
	OK     bool
	Detail string
}

type sseEntry struct {
	Source string `json:"source"`
	Text   string `json:"text"`
}

type todoDoc struct {
	Steps []struct {
		Status      string   `json:"status"`
		ToolsCalled []string `json:"tools_called"`
		Result      struct {
			Summary string `json:"summary"`
		} `json:"result"`
		Feedback []struct {
			Phase   string `json:"phase"`
			Summary string `json:"summary"`
		} `json:"feedback"`
	} `json:"steps"`
}

func main() {
	fmt.Println("=== python-sandbox 主项目前端 E2E ===")
	fmt.Printf("web=%s\n\n", webBase)

	client := &http.Client{Timeout: 0}
	client.Jar = &simpleJar{}

	var results []result
	if err := webLogin(client); err != nil {
		results = append(results, result{"W00", "Web 登录", false, err.Error()})
		printReport(results)
		os.Exit(1)
	}
	results = append(results, result{"W00", "Web 登录", true, webBase})

	// 能力：Python 计算并落盘
	results = append(results, runChatCase(client, "E01", "Python 计算落盘",
		"请用 Python 在 WorkSpace/python_out 目录下计算 sum(range(1,101)) 的值，把结果写入 WorkSpace/python_out/e2e_sum.txt（仅写入数字一行）。完成后 report 说明你调用了哪些 MCP 工具。",
		func(collected string, todo *todoDoc) (bool, string) {
			data, err := os.ReadFile(filepath.Join("WorkSpace", "python_out", "e2e_sum.txt"))
			if err != nil {
				return false, "e2e_sum.txt 未生成: " + err.Error()
			}
			s := strings.TrimSpace(string(data))
			if s != "5050" {
				return false, "sum 应为 5050，得到: " + s
			}
			if !toolsMentionPython(todo, collected) {
				return false, "未见 python-sandbox 工具痕迹: " + toolsSummary(todo)
			}
			return true, "e2e_sum.txt=5050, tools=" + toolsSummary(todo)
		}))

	// 能力：execute 打印
	results = append(results, runChatCase(client, "E02", "Python execute 打印",
		"请用 Python 执行脚本：print('PY_E2E_OK')，并把标准输出原样写入 WorkSpace/python_out/e2e_stdout.txt。",
		func(collected string, todo *todoDoc) (bool, string) {
			data, err := os.ReadFile(filepath.Join("WorkSpace", "python_out", "e2e_stdout.txt"))
			if err != nil {
				return false, err.Error()
			}
			if !strings.Contains(string(data), "PY_E2E_OK") {
				return false, "stdout 内容: " + string(data)
			}
			if !toolsMentionPython(todo, collected) {
				return false, toolsSummary(todo)
			}
			return true, "stdout ok, " + toolsSummary(todo)
		}))

	// 边界：工作区外路径 / 系统文件
	results = append(results, runChatCase(client, "B01", "边界-读系统路径",
		"请用 Python 脚本尝试 open(r'C:\\Windows\\win.ini') 读取前 50 个字符并打印。若失败请如实报告错误信息。",
		func(collected string, todo *todoDoc) (bool, string) {
			lower := strings.ToLower(collected)
			blocked := strings.Contains(lower, "permission") || strings.Contains(lower, "blocked") ||
				strings.Contains(collected, "权限") || strings.Contains(collected, "拦截") ||
				strings.Contains(lower, "denied") || strings.Contains(lower, "无法") ||
				strings.Contains(lower, "fail")
			if !blocked && !strings.Contains(lower, "error") {
				return false, "预期被拦截或报错，回复: " + trim(collected, 400)
			}
			return true, "边界表现符合（拦截/失败）: " + trim(collected, 200)
		}))

	// 边界：subprocess
	results = append(results, runChatCase(client, "B02", "边界-subprocess",
		"请用 Python 执行：import subprocess; subprocess.run(['echo','hi'])。把运行结果简要说明。",
		func(collected string, todo *todoDoc) (bool, string) {
			lower := strings.ToLower(collected)
			ok := strings.Contains(lower, "blocked") || strings.Contains(lower, "importerror") ||
				strings.Contains(lower, "禁止") || strings.Contains(lower, "拦截") ||
				strings.Contains(lower, "fail") || strings.Contains(lower, "error")
			if !ok {
				return false, trim(collected, 500)
			}
			return true, trim(collected, 200)
		}))

	printReport(results)
	fail := 0
	for _, r := range results {
		if !r.OK {
			fail++
		}
	}
	if fail > 0 {
		os.Exit(1)
	}
}

func runChatCase(client *http.Client, id, name, prompt string, verify func(string, *todoDoc) (bool, string)) result {
	fmt.Printf("\n--- %s: %s ---\n", id, name)
	text, err := webChat(client, prompt, chatTimeout)
	if err != nil {
		return result{id, name, false, "chat: " + err.Error()}
	}
	todo := loadLatestTodo()
	ok, detail := verify(text, todo)
	return result{id, name, ok, detail}
}

func toolsMentionPython(todo *todoDoc, collected string) bool {
	if strings.Contains(strings.ToLower(collected), "python-sandbox") {
		return true
	}
	if todo == nil {
		return false
	}
	for _, s := range todo.Steps {
		for _, t := range s.ToolsCalled {
			if strings.Contains(strings.ToLower(t), "python-sandbox") || strings.Contains(strings.ToLower(t), "execute") {
				return true
			}
		}
	}
	return false
}

func toolsSummary(todo *todoDoc) string {
	if todo == nil {
		return "(no todo)"
	}
	var all []string
	for _, s := range todo.Steps {
		all = append(all, s.ToolsCalled...)
	}
	if len(all) == 0 {
		return "(empty tools_called)"
	}
	return strings.Join(all, ", ")
}

func loadLatestTodo() *todoDoc {
	dir := filepath.Join("WorkSpace", "ToDoList")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	if len(files) == 0 {
		return nil
	}
	sort.Slice(files, func(i, j int) bool {
		fi, _ := os.Stat(files[i])
		fj, _ := os.Stat(files[j])
		if fi == nil || fj == nil {
			return files[i] > files[j]
		}
		return fi.ModTime().After(fj.ModTime())
	})
	b, err := os.ReadFile(files[0])
	if err != nil {
		return nil
	}
	var doc todoDoc
	if json.Unmarshal(b, &doc) != nil {
		return nil
	}
	return &doc
}

func printReport(results []result) {
	fmt.Println("\n========== 报告 ==========")
	pass, fail := 0, 0
	for _, r := range results {
		mark := "PASS"
		if !r.OK {
			mark = "FAIL"
			fail++
		} else {
			pass++
		}
		fmt.Printf("[%s] %s — %s\n    %s\n", mark, r.ID, r.Name, r.Detail)
	}
	fmt.Printf("\n合计 PASS=%d FAIL=%d\n", pass, fail)
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
		return fmt.Errorf("login %d: %s", resp.StatusCode, b)
	}
	return nil
}

func webChat(c *http.Client, message string, timeout time.Duration) (string, error) {
	events := make(chan sseEntry, 128)
	done := make(chan struct{})
	go func() {
		defer close(done)
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
	time.Sleep(500 * time.Millisecond)
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
	gotPlan := false
	for {
		select {
		case <-deadline:
			if collect.Len() == 0 {
				return "", fmt.Errorf("timeout no events")
			}
			return collect.String(), nil
		case <-done:
			if collect.Len() > 0 {
				return collect.String(), nil
			}
			return "", fmt.Errorf("sse closed")
		case e := <-events:
			if strings.TrimSpace(e.Source) == "user" || e.Text == "" {
				continue
			}
			collect.WriteString(fmt.Sprintf("[%s] %s\n", e.Source, e.Text))
			if e.Source == "计划编排" && strings.TrimSpace(e.Text) != "" {
				gotPlan = true
			}
			if gotPlan && (strings.Contains(e.Text, "计划编排") || strings.Contains(e.Source, "行为")) {
				// 计划编排终稿后稍等收尾
				time.Sleep(3 * time.Second)
				return collect.String(), nil
			}
		}
	}
}

func trim(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
