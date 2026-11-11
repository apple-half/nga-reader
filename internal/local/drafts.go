package local

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"ngareader/internal/nga"
)

const MaxDrafts = 32
const MaxDraftBytes = 4 << 20

type Draft struct {
	UID       int64          `json:"uid"`
	Target    nga.PostTarget `json:"target"`
	Subject   string         `json:"subject"`
	Content   string         `json:"content"`
	Category  string         `json:"category,omitempty"`
	Label     string         `json:"label,omitempty"`
	Uncertain bool           `json:"uncertain,omitempty"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

func (d Draft) Key() string {
	if d.Target.Action == "new" {
		return fmt.Sprintf("%d/new/%d", d.UID, d.Target.FID)
	}
	return fmt.Sprintf("%d/%s/%d/%d", d.UID, d.Target.Action, d.Target.TID, d.Target.PID)
}

func (d Draft) PostSubject() string {
	subject := strings.TrimSpace(d.Subject)
	category := strings.TrimSpace(d.Category)
	if d.Target.Action == "new" && category != "" && !strings.HasPrefix(subject, category) {
		return category + " " + subject
	}
	return subject
}

func (d *Draft) BeginSend(uid int64) error {
	if uid <= 0 || uid != d.UID {
		return fmt.Errorf("发帖账号已变化，请重新打开编辑器")
	}
	if d.Uncertain {
		return fmt.Errorf("上次发送结果待确认，请先检查原帖或板块")
	}
	if err := nga.ValidatePostText(d.Target, d.PostSubject(), d.Content); err != nil {
		return err
	}
	d.Uncertain = true
	return nil
}

// Returns true only when the server explicitly confirmed completion, allowing
// this draft to be removed. Unknown failures remain pending across restarts.
func (d *Draft) EndSend(err error) bool {
	if err == nil {
		return true
	}
	var failure *nga.PublishError
	d.Uncertain = !errors.As(err, &failure) || failure.Uncertain
	return false
}

func (d *Draft) AcknowledgeUncertain() { d.Uncertain = false }

func validateDrafts(drafts []Draft) error {
	if len(drafts) > MaxDrafts {
		return fmt.Errorf("最多保留%d份草稿，请先清理旧草稿", MaxDrafts)
	}
	seen := map[string]bool{}
	for _, d := range drafts {
		if d.UID <= 0 || d.Target.Validate() != nil || seen[d.Key()] || utf8.RuneCountInString(d.Subject) > nga.MaxPostSubjectRunes || utf8.RuneCountInString(d.Content) > nga.MaxPostContentRunes || utf8.RuneCountInString(d.Category) > 64 || utf8.RuneCountInString(d.Label) > 256 || !utf8.ValidString(d.Subject+d.Content+d.Category+d.Label) || strings.ContainsRune(d.Subject+d.Content+d.Category+d.Label, 0) {
			return fmt.Errorf("草稿内容、账号或目标无效")
		}
		seen[d.Key()] = true
	}
	return nil
}

func LoadDrafts(dir string) ([]Draft, error) {
	file, err := os.Open(filepath.Join(dir, "drafts.json"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取草稿失败：%w", err)
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, MaxDraftBytes+1))
	if err != nil {
		return nil, fmt.Errorf("读取草稿失败：%w", err)
	}
	if len(raw) > MaxDraftBytes {
		return nil, fmt.Errorf("草稿文件过大，已保留原文件")
	}
	var drafts []Draft
	if err = json.Unmarshal(raw, &drafts); err != nil {
		return nil, fmt.Errorf("草稿文件损坏，已保留原文件")
	}
	if err = validateDrafts(drafts); err != nil {
		return nil, err
	}
	return drafts, nil
}

func SaveDrafts(dir string, drafts []Draft) error {
	if err := validateDrafts(drafts); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(drafts, "", "  ")
	if err != nil {
		return err
	}
	if len(raw) > MaxDraftBytes {
		return fmt.Errorf("草稿文件超过本地保存上限")
	}
	return atomicWrite(dir, "drafts.json", raw)
}
