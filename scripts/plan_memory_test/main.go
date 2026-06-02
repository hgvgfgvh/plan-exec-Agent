// 通过 Web UI API 测试 PlanAgent 跨轮会话记忆（外挂 SKILL 列表 → 「他们的详细信息」）。
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
	"strings"
	"time"
)

const (
	baseURL  = "http://127.0.0.1:8765"
	user     = "admin"
	password = "ZAQ!2wsx"
)

type sseEntry struct {
	Source string `json:"source"`
	Text   string `json:"text"`
}

type turn struct {
	Name    string
	Message string
	Timeout time.Duration
	// MustNot 若输出含任一子串则判失败（澄清追问）
	MustNot []string
	// MustAny 至少命中其一
	MustAny []string
	// MinRunes 计划编排输出最少字符数（0=不检查）
	MinRunes int
}

func main() {
	client := &http.Client{Timeout: 0}
	client.Jar = &simpleJar{}
	if err := login(client); err != nil {
		fmt.Fprintf(os.Stderr, "login: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("=== Plan 会话记忆 API 测试 ===")
	fmt.Printf("target: %s\n\n", baseURL)

	turns := []turn{
		{
			Name:    "列举外挂SKILL",
			Message: "你有那些外挂SKILL",
			Timeout: 180 * time.Second,
			MustAny: []string{"bug-pattern", "complex-bug", "demo_external", "外挂"},
		},
		{
			Name:    "指代详情（应延续上一轮）",
			Message: "他们的详细信息 都介绍一下",
			Timeout: 240 * time.Second,
			MustNot: []string{"请帮我明确", "具体是指哪些人", "澄清具体对象", "哪些人/事"},
			MustAny: []string{"bug-pattern", "complex-bug", "demo_external", "get_capability", "SKILL", "详情", "介绍"},
			// 详情步应输出较长正文，而非仅一句「已输出给用户」
			MinRunes: 200,
		},
	}

	allOK := true
	for i, tc := range turns {
		fmt.Printf("--- 轮次 %d: %s ---\n", i+1, tc.Name)
		out, err := runTurn(client, tc)
		if err != nil {
			fmt.Printf("[FAIL] %v\n\n", err)
			allOK = false
			continue
		}
		lower := strings.ToLower(out)
		for _, bad := range tc.MustNot {
			if strings.Contains(lower, strings.ToLower(bad)) {
				fmt.Printf("[FAIL] 不应出现澄清类话术，命中: %q\n", bad)
				fmt.Printf("输出片段:\n%s\n\n", trim(out, 1500))
				allOK = false
				goto nextTurn
			}
		}
		if len(tc.MustAny) > 0 {
			hit := false
			var matched string
			for _, w := range tc.MustAny {
				if strings.Contains(lower, strings.ToLower(w)) {
					hit = true
					matched = w
					break
				}
			}
			if !hit {
				fmt.Printf("[FAIL] 未命中期望关键词 %v\n", tc.MustAny)
				fmt.Printf("输出片段:\n%s\n\n", trim(out, 1500))
				allOK = false
				goto nextTurn
			}
			fmt.Printf("[PASS] matched=%q\n", matched)
		} else {
			fmt.Println("[PASS]")
		}
		if tc.MinRunes > 0 {
			n := len([]rune(out))
			if n < tc.MinRunes {
				fmt.Printf("[FAIL] 输出过短 runes=%d want>=%d\n", n, tc.MinRunes)
				fmt.Printf("输出片段:\n%s\n\n", trim(out, 1500))
				allOK = false
				goto nextTurn
			}
			fmt.Printf("[PASS] length runes=%d\n", n)
		}
		fmt.Printf("输出片段:\n%s\n\n", trim(out, 800))
	nextTurn:
	}
	if allOK {
		fmt.Println("=== 全部通过 ===")
		return
	}
	fmt.Println("=== 存在失败 ===")
	os.Exit(1)
}

func login(c *http.Client) error {
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

func runTurn(c *http.Client, tc turn) (string, error) {
	events := make(chan sseEntry, 128)
	go func() {
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
		return "", err
	}
	chatResp.Body.Close()
	if chatResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("chat status %d", chatResp.StatusCode)
	}

	var collect strings.Builder
	deadline := time.After(tc.Timeout)
	for {
		select {
		case <-deadline:
			if collect.Len() == 0 {
				return "", fmt.Errorf("超时无输出")
			}
			return collect.String(), nil
		case e := <-events:
			src := strings.TrimSpace(e.Source)
			if src == "user" || e.Text == "" {
				continue
			}
			collect.WriteString(fmt.Sprintf("[%s] %s\n", src, e.Text))
			if src == "计划编排" && strings.TrimSpace(e.Text) != "" {
				time.Sleep(2 * time.Second)
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

type simpleJar struct {
	cookies []*http.Cookie
}

func (j *simpleJar) SetCookies(u *url.URL, cookies []*http.Cookie) { j.cookies = cookies }
func (j *simpleJar) Cookies(u *url.URL) []*http.Cookie             { return j.cookies }
