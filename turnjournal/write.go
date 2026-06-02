package turnjournal

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

func writeBundle(b *Bundle) error {
	if b == nil || b.TurnID == "" {
		return nil
	}
	path, err := FilePath(b.TurnID)
	if err != nil {
		log.Printf("[turnjournal] path: %v", err)
		return err
	}
	b.LogPath = path
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		log.Printf("[turnjournal] marshal: %v", err)
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		log.Printf("[turnjournal] write %s: %v", path, err)
		return err
	}
	fmt.Printf("[turnjournal] 已写入回合日志: %s\n", path)
	return nil
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return s
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

func excerpt(s string, max int) string {
	s = stringsTrimSpace(s)
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

func stringsTrimSpace(s string) string {
	i, j := 0, len(s)
	for i < j && (s[i] == ' ' || s[i] == '\n' || s[i] == '\t' || s[i] == '\r') {
		i++
	}
	for j > i && (s[j-1] == ' ' || s[j-1] == '\n' || s[j-1] == '\t' || s[j-1] == '\r') {
		j--
	}
	return s[i:j]
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
