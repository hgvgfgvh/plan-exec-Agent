// Package userupload 管理 Web UI 用户上传到 WorkSpace 的暂存与回合目录。
package userupload

import (
	"AgentTest/config"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	StagingSubdir = "inbox/staging"
	InboxSubdir   = "inbox"
	maxFileBytes  = 25 << 20 // 25 MiB
	maxFiles      = 30
)

// Entry 用户附件元数据（路径相对仓库根，便于 Agent / MCP 使用）。
type Entry struct {
	RelPath string `json:"rel_path"`
	Kind    string `json:"kind"` // image | file | folder
	Name    string `json:"name"`
}

// WorkspaceRelPrefix 返回配置中的工作区目录名（如 WorkSpace）。
func WorkspaceRelPrefix() string {
	cfg := config.TryGet()
	if cfg == nil || cfg.Paths.Workspace == "" {
		return "WorkSpace"
	}
	return strings.Trim(filepath.ToSlash(cfg.Paths.Workspace), "/")
}

func workspaceAbs() (string, string, error) {
	cfg := config.TryGet()
	if cfg == nil {
		return "", "", fmt.Errorf("config 未加载")
	}
	ws := cfg.ResolvedPaths().Workspace
	if ws == "" {
		return "", "", fmt.Errorf("workspace 未配置")
	}
	return ws, WorkspaceRelPrefix(), nil
}

// NewStagingID 生成新的暂存批次 ID。
func NewStagingID() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("s-%d-%s", time.Now().UnixNano(), hex.EncodeToString(b[:]))
}

// StagingAbs 返回暂存目录绝对路径。
func StagingAbs(stagingID string) (string, error) {
	ws, _, err := workspaceAbs()
	if err != nil {
		return "", err
	}
	stagingID = sanitizeID(stagingID)
	if stagingID == "" {
		return "", fmt.Errorf("invalid staging id")
	}
	return filepath.Join(ws, StagingSubdir, stagingID), nil
}

// TurnInboxAbs 返回某回合 inbox 目录绝对路径。
func TurnInboxAbs(turnID string) (string, error) {
	ws, _, err := workspaceAbs()
	if err != nil {
		return "", err
	}
	turnID = sanitizeID(turnID)
	if turnID == "" {
		return "", fmt.Errorf("invalid turn id")
	}
	return filepath.Join(ws, InboxSubdir, turnID), nil
}

func sanitizeID(id string) string {
	id = strings.TrimSpace(id)
	id = strings.ReplaceAll(id, "..", "")
	id = strings.ReplaceAll(id, "/", "")
	id = strings.ReplaceAll(id, "\\", "")
	return id
}

// SaveStagingFile 将上传文件写入暂存目录（保留 webkitdirectory 相对路径）。
func SaveStagingFile(stagingID, relName string, r io.Reader, size int64) (Entry, error) {
	if size > maxFileBytes {
		return Entry{}, fmt.Errorf("文件过大（上限 %d MiB）: %s", maxFileBytes>>20, relName)
	}
	relName, err := sanitizeRelName(relName)
	if err != nil {
		return Entry{}, err
	}
	if isAudioName(relName) {
		return Entry{}, fmt.Errorf("音频暂不处理: %s", relName)
	}
	dir, err := StagingAbs(stagingID)
	if err != nil {
		return Entry{}, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Entry{}, err
	}
	n, _ := countFiles(dir)
	if n >= maxFiles {
		return Entry{}, fmt.Errorf("单批附件数量已达上限 %d", maxFiles)
	}
	relName, err = allocUniqueRelName(dir, relName)
	if err != nil {
		return Entry{}, err
	}
	dest := filepath.Join(dir, filepath.FromSlash(relName))
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return Entry{}, err
	}
	if !ensureUnder(dir, dest) {
		return Entry{}, fmt.Errorf("非法路径: %s", relName)
	}
	f, err := os.Create(dest)
	if err != nil {
		return Entry{}, err
	}
	defer f.Close()
	written, err := io.Copy(f, io.LimitReader(r, maxFileBytes+1))
	if err != nil {
		return Entry{}, err
	}
	if written > maxFileBytes {
		_ = os.Remove(dest)
		return Entry{}, fmt.Errorf("文件过大: %s", relName)
	}
	prefix := WorkspaceRelPrefix()
	rel := filepath.ToSlash(filepath.Join(prefix, StagingSubdir, sanitizeID(stagingID), relName))
	return Entry{
		RelPath: rel,
		Kind:    KindForName(relName),
		Name:    filepath.Base(relName),
	}, nil
}

const maxUniqueNameTries = 100

// allocUniqueRelName 若暂存目录中已有同名文件则自动重命名为 "name (2).ext"（避免静默覆盖）。
func allocUniqueRelName(dir, relName string) (string, error) {
	relSlash := filepath.ToSlash(relName)
	parent := filepath.ToSlash(filepath.Dir(filepath.FromSlash(relSlash)))
	base := filepath.Base(filepath.FromSlash(relSlash))
	for n := 0; n < maxUniqueNameTries; n++ {
		candidateBase := base
		if n > 0 {
			candidateBase = numberedFileName(base, n+1)
		}
		candidateRel := candidateBase
		if parent != "" && parent != "." {
			candidateRel = parent + "/" + candidateBase
		}
		dest := filepath.Join(dir, filepath.FromSlash(candidateRel))
		if !fileExistsAtDest(dest) {
			return candidateRel, nil
		}
	}
	return "", fmt.Errorf("无法为 %q 分配唯一文件名", base)
}

func numberedFileName(base string, n int) string {
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	if stem == "" {
		return fmt.Sprintf("%s (%d)", base, n)
	}
	return fmt.Sprintf("%s (%d)%s", stem, n, ext)
}

func fileExistsAtDest(dest string) bool {
	_, err := os.Stat(dest)
	return err == nil
}

func sanitizeRelName(name string) (string, error) {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "\\", "/")
	name = filepath.Clean(name)
	name = strings.TrimPrefix(name, "/")
	if name == "" || name == "." {
		return "", fmt.Errorf("空文件名")
	}
	if strings.Contains(name, "..") {
		return "", fmt.Errorf("非法文件名")
	}
	return name, nil
}

func ensureUnder(base, target string) bool {
	base = filepath.Clean(base)
	target = filepath.Clean(target)
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func isAudioName(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".mp3", ".wav", ".ogg", ".webm", ".m4a", ".aac", ".flac":
		return true
	}
	return strings.HasPrefix(strings.ToLower(mime.TypeByExtension(ext)), "audio/")
}

// KindForName 根据扩展名推断附件类型。
func KindForName(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp":
		return "image"
	default:
		return "file"
	}
}

func countFiles(dir string) (int, error) {
	var n int
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		n++
		return nil
	})
	return n, nil
}

// ListStaging 列出暂存目录中的附件（不移动）。
func ListStaging(stagingID string) ([]Entry, error) {
	dir, err := StagingAbs(stagingID)
	if err != nil {
		return nil, err
	}
	return listDirEntries(dir, filepath.Join(WorkspaceRelPrefix(), StagingSubdir, sanitizeID(stagingID)))
}

func listDirEntries(absDir, relPrefix string) ([]Entry, error) {
	if _, err := os.Stat(absDir); os.IsNotExist(err) {
		return nil, nil
	}
	var out []Entry
	seenDirs := make(map[string]struct{})
	err := filepath.WalkDir(absDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path == absDir {
				return nil
			}
			rel, _ := filepath.Rel(absDir, path)
			rel = filepath.ToSlash(rel)
			dirRel := filepath.ToSlash(filepath.Join(relPrefix, rel))
			if _, ok := seenDirs[dirRel]; !ok {
				seenDirs[dirRel] = struct{}{}
				out = append(out, Entry{
					RelPath: dirRel + "/",
					Kind:    "folder",
					Name:    filepath.Base(rel),
				})
			}
			return nil
		}
		rel, _ := filepath.Rel(absDir, path)
		rel = filepath.ToSlash(rel)
		out = append(out, Entry{
			RelPath: filepath.ToSlash(filepath.Join(relPrefix, rel)),
			Kind:    KindForName(rel),
			Name:    filepath.Base(rel),
		})
		return nil
	})
	return out, err
}

// FinalizeStaging 将暂存目录整体移动到 inbox/{turnID}，返回最终条目。
func FinalizeStaging(stagingID, turnID string) ([]Entry, error) {
	stagingID = sanitizeID(stagingID)
	turnID = sanitizeID(turnID)
	if stagingID == "" {
		return nil, nil
	}
	src, err := StagingAbs(stagingID)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return nil, nil
	}
	dst, err := TurnInboxAbs(turnID)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return nil, err
	}
	_ = os.RemoveAll(dst)
	if err := os.Rename(src, dst); err != nil {
		if err := copyTree(src, dst); err != nil {
			return nil, err
		}
		_ = os.RemoveAll(src)
	}
	prefix := filepath.ToSlash(filepath.Join(WorkspaceRelPrefix(), InboxSubdir, turnID))
	return listDirEntries(dst, prefix)
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// FormatAttachmentsBlock 生成注入 Plan/Behavior 的附件说明块。
func FormatAttachmentsBlock(entries []Entry) string {
	if len(entries) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("【用户附件】（已落在工作区，请用 filesystem MCP 读取文件/目录，或用内置技能 AnalyzeWorkspaceImage 分析图片；勿编造未读取的内容）\n")
	for _, e := range entries {
		kind := e.Kind
		if kind == "" {
			kind = "file"
		}
		b.WriteString(fmt.Sprintf("- [%s] %s", kind, e.RelPath))
		if e.Name != "" && e.Name != filepath.Base(e.RelPath) {
			b.WriteString(fmt.Sprintf(" （%s）", e.Name))
		}
		b.WriteString("\n")
	}
	b.WriteString("图片分析请使用内置技能 AnalyzeWorkspaceImage，传入上述 image 路径；勿用 SeeAndDenseCaption/SeeCameraAndDescribe（截屏/摄像头）。")
	return b.String()
}

// ResolveUnderWorkspace 将 Agent 传入的路径解析为绝对路径并校验在工作区内。
func ResolveUnderWorkspace(displayPath string) (string, error) {
	displayPath = strings.TrimSpace(displayPath)
	if displayPath == "" {
		return "", fmt.Errorf("路径为空")
	}
	cfg := config.TryGet()
	if cfg == nil {
		return "", fmt.Errorf("config 未加载")
	}
	root := cfg.AbsRoot()
	ws := cfg.ResolvedPaths().Workspace
	var abs string
	if filepath.IsAbs(displayPath) {
		abs = filepath.Clean(displayPath)
	} else {
		p := filepath.ToSlash(displayPath)
		if strings.HasPrefix(strings.ToLower(p), strings.ToLower(WorkspaceRelPrefix())) {
			abs = filepath.Clean(filepath.Join(root, filepath.FromSlash(p)))
		} else {
			abs = filepath.Clean(filepath.Join(ws, filepath.FromSlash(p)))
		}
	}
	wsClean := filepath.Clean(ws)
	if abs != wsClean && !strings.HasPrefix(abs, wsClean+string(os.PathSeparator)) {
		return "", fmt.Errorf("路径须位于工作区内: %s", displayPath)
	}
	if _, err := os.Stat(abs); err != nil {
		return "", fmt.Errorf("路径不可访问: %s (%v)", displayPath, err)
	}
	return abs, nil
}
