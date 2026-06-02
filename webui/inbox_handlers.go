package webui

import (
	"AgentTest/userupload"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"
)

const maxMultipartMem = 32 << 20

func handleUpload(w http.ResponseWriter, r *http.Request) {
	if !sessionValid(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if err := r.ParseMultipartForm(maxMultipartMem); err != nil {
		http.Error(w, "bad multipart", http.StatusBadRequest)
		return
	}
	stagingID := strings.TrimSpace(r.FormValue("staging_id"))
	if stagingID == "" {
		stagingID = userupload.NewStagingID()
	}
	var saved []userupload.Entry
	var skipped []string
	for _, headers := range r.MultipartForm.File {
		for _, fh := range headers {
			e, sk, err := saveUploadPart(stagingID, fh)
			if sk != "" {
				skipped = append(skipped, sk)
				continue
			}
			if err != nil {
				skipped = append(skipped, fmt.Sprintf("%s: %v", fh.Filename, err))
				continue
			}
			saved = append(saved, e)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":         true,
		"staging_id": stagingID,
		"files":      saved,
		"skipped":    skipped,
	})
}

func saveUploadPart(stagingID string, fh *multipart.FileHeader) (userupload.Entry, string, error) {
	name := fh.Filename
	if userupload.KindForName(name) == "file" && isAudioUpload(name) {
		return userupload.Entry{}, name + " (音频暂不处理)", nil
	}
	f, err := fh.Open()
	if err != nil {
		return userupload.Entry{}, "", err
	}
	defer f.Close()
	e, err := userupload.SaveStagingFile(stagingID, name, f, fh.Size)
	if err != nil {
		if strings.Contains(err.Error(), "音频暂不处理") {
			return userupload.Entry{}, name + " (音频暂不处理)", nil
		}
		return userupload.Entry{}, "", err
	}
	return e, "", nil
}

func isAudioUpload(name string) bool {
	i := strings.LastIndex(name, ".")
	if i < 0 {
		return false
	}
	ext := strings.ToLower(name[i:])
	switch ext {
	case ".mp3", ".wav", ".ogg", ".webm", ".m4a", ".aac", ".flac":
		return true
	}
	return false
}

func userDisplayMessage(msg, stagingID string) string {
	msg = strings.TrimSpace(msg)
	if stagingID == "" {
		return msg
	}
	entries, err := userupload.ListStaging(stagingID)
	if err != nil || len(entries) == 0 {
		return msg
	}
	block := userupload.FormatAttachmentsBlock(entries)
	if block == "" {
		return msg
	}
	if msg == "" {
		return block
	}
	return msg + "\n\n" + block
}
